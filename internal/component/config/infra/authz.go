// Design: docs/architecture/config/syntax.md -- daemon-startup authorization extraction
// Related: ssh.go -- SSH server config extraction from the same tree

package infra

import (
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/component/aaa"
	"github.com/ze-software/ze/internal/component/authz"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// infraLogger is the daemon-startup config-extraction logger (lazy init).
var infraLogger = slogutil.LazyLogger("config.infra")

// ExtractAuthUsers returns the local user credentials the `system` subtree
// describes. The subtree is in resolved map form, which is what
// config.Tree.ToMap() produces and what config.Provider.Get("system") returns.
//
// A nil or shapeless subtree yields no users. Every caller treats that as "this
// configuration authenticates nobody", which denies, so the empty result is the
// closed answer rather than a permissive one.
//
// Users and their public keys come back sorted by name. The map form has no
// order of its own, and a caller that merges or logs the result needs one.
func ExtractAuthUsers(system map[string]any) []authz.UserConfig {
	auth, ok := system["authentication"].(map[string]any)
	if !ok {
		return nil
	}
	entries, ok := auth["user"].(map[string]any)
	if !ok {
		return nil
	}
	users := make([]authz.UserConfig, 0, len(entries))
	for name, raw := range entries {
		entry, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		uc := authz.UserConfig{Name: name}
		if pw, ok := entry["password"].(string); ok {
			uc.Hash = pw
		}
		uc.Profiles = leafListValues(entry["profile"])
		uc.PublicKeys = extractPublicKeys(entry["public-keys"])
		if ls, ok := entry["login-service"].(string); ok {
			uc.LoginSet = ls
		}
		users = append(users, uc)
	}
	slices.SortFunc(users, func(a, b authz.UserConfig) int { return strings.Compare(a.Name, b.Name) })
	return users
}

// extractPublicKeys reads the `public-keys` keyed list of one user entry.
func extractPublicKeys(raw any) []authz.SSHPublicKey {
	entries, ok := raw.(map[string]any)
	if !ok {
		return nil
	}
	keys := make([]authz.SSHPublicKey, 0, len(entries))
	for name, rawKey := range entries {
		entry, ok := rawKey.(map[string]any)
		if !ok {
			continue
		}
		pk := authz.SSHPublicKey{Name: name}
		if t, ok := entry["type"].(string); ok {
			pk.Type = t
		}
		if k, ok := entry["key"].(string); ok {
			pk.Key = k
		}
		keys = append(keys, pk)
	}
	if len(keys) == 0 {
		return nil
	}
	slices.SortFunc(keys, func(a, b authz.SSHPublicKey) int { return strings.Compare(a.Name, b.Name) })
	return keys
}

// leafListValues reads a YANG leaf-list from resolved map form. Tree.ToMap
// collapses a one-member leaf-list to a plain string and emits []string for
// more, while the same value arrives as []any after a JSON round trip, so all
// three shapes are accepted.
func leafListValues(raw any) []string {
	switch v := raw.(type) {
	case string:
		if v == "" {
			return nil
		}
		return []string{v}
	case []string:
		return slices.Clone(v)
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		if len(out) == 0 {
			return nil
		}
		return out
	}
	return nil
}

// ValidateAuthzConfig validates authorization config in the parsed tree.
// Checks: profile entry regex syntax (hard error), and that no profile,
// login-service set, user, tacacs-profile, or radius entry names a reserved
// identifier. A name that no profile or login-service defines is refused by the
// YANG leafref on each referring leaf (Validator.ValidateReferences), which
// names the entries that exist. Exported so ze config validate can also call it.
func ValidateAuthzConfig(tree *config.Tree) error {
	sys := tree.GetContainer("system")
	if sys == nil {
		return nil
	}

	if authzContainer := sys.GetContainer("authorization"); authzContainer != nil {
		// Validate each profile's entries (regex syntax, empty match).
		for name, profileTree := range authzContainer.GetList("profile") {
			// Fail closed: reserved names live outside the config namespace (the
			// break-glass recovery profile and the trusted internal identity). They are
			// un-typeable by construction, so this only fires on a hand-crafted tree,
			// but rejecting it here keeps an operator from ever defining a profile that
			// collides with a reserved allow-all name (spec R-8).
			if aaa.IsReservedName(name) {
				return fmt.Errorf("authorization profile %q uses a reserved name", name)
			}
			p := authz.Profile{Name: name}
			if runContainer := profileTree.GetContainer("run"); runContainer != nil {
				p.Run = extractAuthzSection(runContainer)
			}
			if editContainer := profileTree.GetContainer("edit"); editContainer != nil {
				p.Edit = extractAuthzSection(editContainer)
			}
			if err := p.Validate(); err != nil {
				return fmt.Errorf("authorization profile: %w", err)
			}
		}

		for name := range authzContainer.GetList("login-service") {
			if aaa.IsReservedName(name) {
				return fmt.Errorf("authorization login-service %q uses a reserved name", name)
			}
		}
	}

	// Check user, tacacs-profile, and radius references for reserved names. A
	// name no profile or login-service defines is refused by its YANG leafref.
	auth := sys.GetContainer("authentication")
	if auth == nil {
		return nil
	}

	for username, userTree := range auth.GetList("user") {
		if aaa.IsReservedName(username) {
			return fmt.Errorf("user %q uses a reserved name", username)
		}
		for _, pn := range userTree.GetSlice("profile") {
			if aaa.IsReservedName(pn) {
				return fmt.Errorf("user %q references reserved profile %q", username, pn)
			}
		}
		if ls, ok := userTree.Get("login-service"); ok {
			if aaa.IsReservedName(ls) {
				return fmt.Errorf("user %q references reserved login-service %q", username, ls)
			}
		}
	}

	// Check tacacs-profile priv-lvl -> profile and login-service references for
	// reserved names, on the same footing as the user references above.
	for level, entry := range auth.GetList("tacacs-profile") {
		for _, pn := range entry.GetSlice("profile") {
			if aaa.IsReservedName(pn) {
				return fmt.Errorf("tacacs-profile %q references reserved profile %q", level, pn)
			}
		}
		if ls, ok := entry.Get("login-service"); ok {
			if aaa.IsReservedName(ls) {
				return fmt.Errorf("tacacs-profile %q references reserved login-service %q", level, ls)
			}
		}
	}

	if radius := auth.GetContainer("radius"); radius != nil {
		if ls, ok := radius.Get("login-service"); ok {
			if aaa.IsReservedName(ls) {
				return fmt.Errorf("radius references reserved login-service %q", ls)
			}
		}
	}

	return nil
}

// ExtractLoginServices returns the named login-service sets the `system`
// subtree describes, in resolved map form (as config.Tree.ToMap() produces).
// Returns an empty map when no system/authorization/login-service section
// exists. For every defined login-service entry, the returned map holds a
// non-nil slice ([]string{} when no service is listed) so an empty set is
// always distinct from an absent set.
func ExtractLoginServices(system map[string]any) map[string][]string {
	sets := make(map[string][]string)
	authzMap, ok := system["authorization"].(map[string]any)
	if !ok {
		// refuse-by-default: guards aaa.SetAcceptedLoginSets -- absent
		// authorization section yields an empty set map so any restricted
		// login-service reference fails closed.
		return sets
	}
	entries, ok := authzMap["login-service"].(map[string]any)
	if !ok {
		// refuse-by-default: guards aaa.SetAcceptedLoginSets -- absent
		// login-service section yields an empty set map so any restricted
		// login-service reference fails closed.
		return sets
	}
	for name, raw := range entries {
		entry, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		services := leafListValues(entry["service"])
		if services == nil {
			services = []string{}
		}
		sets[name] = services
	}
	return sets
}

// ExtractAuthzStore extracts authorization profiles and user assignments from a
// parsed config tree. Returns nil when no system.authorization profiles exist.
func ExtractAuthzStore(tree *config.Tree) (*authz.Store, error) {
	return extractAuthzConfig(tree)
}

// Returns a populated Store if system.authorization is present with profiles, nil otherwise.
// User-to-profile assignments come from system.authentication.user[*].profile (leaf-list).
// The error is the YANG loader's, when the command set the match entries are
// checked against cannot be loaded.
func extractAuthzConfig(tree *config.Tree) (*authz.Store, error) {
	sys := tree.GetContainer("system")
	if sys == nil {
		return nil, nil //nolint:nilnil // no system block: no authz store, and nothing failed
	}

	authzContainer := sys.GetContainer("authorization")
	if authzContainer == nil {
		return nil, nil //nolint:nilnil // no authorization block: no authz store, and nothing failed
	}

	profiles := authzContainer.GetList("profile")
	if len(profiles) == 0 {
		return nil, nil //nolint:nilnil // no profile: no authz store, and nothing failed
	}

	store := authz.NewStore()

	for name, profileTree := range profiles {
		p := authz.Profile{Name: name}

		if runContainer := profileTree.GetContainer("run"); runContainer != nil {
			p.Run = extractAuthzSection(runContainer)
		}

		if editContainer := profileTree.GetContainer("edit"); editContainer != nil {
			p.Edit = extractAuthzSection(editContainer)
		}

		// ValidateAuthzConfig already rejected invalid profiles (regex, empty match).
		store.AddProfile(p)
	}

	// Extract user → profile assignments from authentication block
	if auth := sys.GetContainer("authentication"); auth != nil {
		for username, userTree := range auth.GetList("user") {
			profileNames := userTree.GetSlice("profile")
			if len(profileNames) > 0 {
				store.AssignProfiles(username, profileNames)
			}
		}
	}

	// Warn about match entries that match neither a known builtin command nor a
	// config permission string (AC-9). Warning only — plugins may register
	// commands dynamically at runtime.
	if err := validateMatchEntries(store); err != nil {
		return nil, err
	}

	if !store.HasProfiles() {
		return nil, nil //nolint:nilnil // no profile: no authz store, and nothing failed
	}

	return store, nil
}

// validateMatchEntries warns about profile match entries that match neither a
// known builtin command prefix nor a config permission string. This is a
// best-effort check because plugins register commands dynamically at runtime.
// A YANG loader failure is returned, not treated as an empty command set: an
// empty set would warn on every entry and hide the cause.
func validateMatchEntries(store *authz.Store) error {
	unknown, err := unknownMatchEntries(store)
	if err != nil {
		return err
	}
	for _, u := range unknown {
		infraLogger().Warn("authz match entry does not match any known command",
			"profile", u.profile, "section", u.section, "match", u.match)
	}
	return nil
}

// unknownMatchEntry is one profile match entry that matches nothing Ze is
// known to authorize.
type unknownMatchEntry struct {
	profile string
	section string
	match   string
}

// unknownMatchEntries returns the prefix entries of the store that match
// neither a builtin command nor a config permission string, in walk order.
// Regex entries are never returned: they cannot be prefix-checked.
func unknownMatchEntries(store *authz.Store) ([]unknownMatchEntry, error) {
	schema, err := yang.DefaultLoader()
	if err != nil {
		return nil, fmt.Errorf("authorization match entries: YANG schema: %w", err)
	}
	wireToPaths := yang.WireMethodToPaths(schema)

	var cmds []string
	for _, paths := range wireToPaths {
		for _, path := range paths {
			cmds = append(cmds, strings.ToLower(path))
		}
	}

	var unknown []unknownMatchEntry
	store.WalkEntries(func(profileName, section string, e authz.Entry) {
		if e.Regex {
			return // regex entries can't be prefix-checked
		}
		match := strings.ToLower(e.Match)
		for _, cmd := range cmds {
			if strings.HasPrefix(cmd, match) || strings.HasPrefix(match, cmd) {
				return // match is a prefix of (or matches) a known command
			}
		}
		if matchesConfigPermission(match) {
			return
		}
		unknown = append(unknown, unknownMatchEntry{profile: profileName, section: section, match: e.Match})
	})
	return unknown, nil
}

// matchesConfigPermission reports whether a lowercased prefix entry reaches a
// config permission string (plugin.ConfigAuthStrings). These are not
// dispatcher commands, so the command registry does not know them, yet every
// surface that changes configuration asks the authorizer with them. The entry
// reaches one when it is a word-boundary prefix of the string (`config`,
// `config commit`), which is how the authorizer matches it, or when the string
// is a word-boundary prefix of the entry (`config commit client-edge.conf`),
// because a session on a named stored configuration asks the string followed
// by one space and the configuration's name.
func matchesConfigPermission(match string) bool {
	for _, permission := range plugin.ConfigAuthStrings() {
		if isWordPrefix(match, permission) {
			return true
		}
		if isWordPrefix(permission, match) {
			return true
		}
	}
	return false
}

// isWordPrefix reports whether prefix is s, or the start of s followed by a
// space: the word-boundary rule the authorizer applies to a prefix entry.
func isWordPrefix(prefix, s string) bool {
	if !strings.HasPrefix(s, prefix) {
		return false
	}
	if len(s) == len(prefix) {
		return true
	}
	return s[len(prefix)] == ' '
}

// extractAuthzSection extracts a run or edit authorization section from the config tree.
func extractAuthzSection(container *config.Tree) authz.Section {
	var s authz.Section

	if v, ok := container.Get("default-action"); ok {
		if v == "allow" {
			s.Default = authz.Allow
		}
	}

	for numStr, entryTree := range container.GetList("entry") {
		num, err := strconv.ParseUint(numStr, 10, 32)
		if err != nil {
			continue
		}

		e := authz.Entry{Number: uint32(num)}

		if v, ok := entryTree.Get("action"); ok {
			if v == "allow" {
				e.Action = authz.Allow
			}
		}

		if v, ok := entryTree.Get("match"); ok {
			e.Match = v
		}

		if v, ok := entryTree.Get("regex"); ok {
			e.Regex = v == "true"
		}

		s.Entries = append(s.Entries, e)
	}

	// Sort entries by number (ascending) for deterministic evaluation order
	sort.Slice(s.Entries, func(i, j int) bool {
		return s.Entries[i].Number < s.Entries[j].Number
	})

	return s
}
