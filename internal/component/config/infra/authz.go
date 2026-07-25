// Design: docs/architecture/config/syntax.md -- daemon-startup authorization extraction
// Related: ssh.go -- SSH server config extraction from the same tree

package infra

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/component/aaa"
	"github.com/ze-software/ze/internal/component/authz"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// infraLogger is the daemon-startup config-extraction logger (lazy init).
var infraLogger = slogutil.LazyLogger("config.infra")

// ValidateAuthzConfig validates authorization config in the parsed tree.
// Checks: profile entry regex syntax (hard error), user→profile references (AC-8).
// Exported so ze config validate can also call it.
func ValidateAuthzConfig(tree *config.Tree) error {
	sys := tree.GetContainer("system")
	if sys == nil {
		return nil
	}

	authzContainer := sys.GetContainer("authorization")
	if authzContainer == nil {
		return nil
	}

	profiles := authzContainer.GetList("profile")

	// Validate each profile's entries (regex syntax, empty match).
	for name, profileTree := range profiles {
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

	// Check user→profile references (AC-8).
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
			if _, ok := profiles[pn]; !ok {
				return fmt.Errorf("user %q references undefined profile %q", username, pn)
			}
		}
	}

	// Check tacacs-profile priv-lvl -> profile references, on the same footing as
	// the user references above. These names decide what a TACACS+-authenticated
	// session may run (the authenticator resolves them at login and authorization
	// applies them), so an undefined one is the same operator error as an
	// undefined user profile -- it just arrives through a different door.
	//
	// Catching it here matters because the runtime cannot report it: authorization
	// receives profile names, not the mapping, so it can only ignore a name it
	// cannot resolve. A typo would otherwise load quietly and surface as a session
	// whose profile silently does not apply.
	for level, entry := range auth.GetList("tacacs-profile") {
		for _, pn := range entry.GetSlice("profile") {
			if aaa.IsReservedName(pn) {
				return fmt.Errorf("tacacs-profile %q references reserved profile %q", level, pn)
			}
			if _, ok := profiles[pn]; !ok {
				return fmt.Errorf("tacacs-profile %q references undefined profile %q", level, pn)
			}
		}
	}

	return nil
}

// ExtractAuthzStore extracts authorization profiles and user assignments from a
// parsed config tree. Returns nil when no system.authorization profiles exist.
func ExtractAuthzStore(tree *config.Tree) *authz.Store {
	return extractAuthzConfig(tree)
}

// Returns a populated Store if system.authorization is present with profiles, nil otherwise.
// User-to-profile assignments come from system.authentication.user[*].profile (leaf-list).
func extractAuthzConfig(tree *config.Tree) *authz.Store {
	sys := tree.GetContainer("system")
	if sys == nil {
		return nil
	}

	authzContainer := sys.GetContainer("authorization")
	if authzContainer == nil {
		return nil
	}

	profiles := authzContainer.GetList("profile")
	if len(profiles) == 0 {
		return nil
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

	// Warn about match entries that don't match any known builtin command (AC-9).
	// Warning only — plugins may register commands dynamically at runtime.
	validateMatchEntries(store)

	if !store.HasProfiles() {
		return nil
	}

	return store
}

// validateMatchEntries warns about profile match entries that don't match
// any known builtin command prefix. This is a best-effort check because
// plugins register commands dynamically at runtime.
func validateMatchEntries(store *authz.Store) {
	loader, _ := yang.DefaultLoader()
	wireToPaths := yang.WireMethodToPaths(loader)

	var cmds []string
	for _, paths := range wireToPaths {
		for _, path := range paths {
			cmds = append(cmds, strings.ToLower(path))
		}
	}

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
		infraLogger().Warn("authz match entry does not match any known command",
			"profile", profileName, "section", section, "match", e.Match)
	})
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
