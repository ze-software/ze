// Design: docs/architecture/core-design.md -- authorization component
// Related: auth.go -- user/token authentication feeding into authz decisions

// Package authz provides profile-based command authorization.
// Profiles contain ordered allow/deny entries matched against command paths.
// Each profile has two sections: run (operational) and edit (configuration).
package authz

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"sync"

	"github.com/ze-software/ze/internal/component/aaa"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/slogutil"
)

var errProfileNameCannotBeEmpty = errors.New("profile name cannot be empty")

// authzLogger records authorization decisions so an operator can tell "denied by
// profile" from "denied because no profile applied", and can see when a
// break-glass recovery or trusted-internal grant was used (evidence.md:
// the layer that knows the reason is the one that must say so).
var authzLogger = slogutil.Logger("authz")

// Action represents an authorization decision.
type Action int

const (
	// Deny means the command is not authorized.
	Deny Action = iota
	// Allow means the command is authorized.
	Allow
)

// String returns the string representation of an Action.
func (a Action) String() string {
	if a == Allow {
		return "allow"
	}
	return "deny"
}

// Entry is a single authorization rule within a section.
type Entry struct {
	Number uint32 // Auto-assigned sequence number for ordering
	Action Action // Allow or Deny
	Match  string // Command path prefix (or regex if Regex is true)
	Regex  bool   // If true, Match is a regular expression

	compiled *regexp.Regexp // cached compiled regex (set by Validate at config load)
}

// Validate checks that the entry is well-formed.
// Must be called at config load time to eagerly compile regex entries.
// After Validate(), entries are read-only and safe for concurrent use.
func (e *Entry) Validate() error {
	if e.Match == "" {
		return fmt.Errorf("entry %d: match string cannot be empty", e.Number)
	}
	if e.Regex {
		r, err := regexp.Compile(e.Match)
		if err != nil {
			return fmt.Errorf("entry %d: invalid regex %q: %w", e.Number, e.Match, err)
		}
		e.compiled = r
	}
	return nil
}

// matches checks if a command matches this entry.
// For prefix matching: the entry's match string must be a prefix of the command
// at a word boundary (exact match or followed by a space).
// For regex matching: uses the compiled regex (set by Validate).
func (e *Entry) matches(command string) bool {
	if e.Regex {
		if e.compiled == nil {
			// Safety net for entries not validated (test code only).
			// Production entries are always validated at config load.
			r, err := regexp.Compile(e.Match)
			if err != nil {
				return false
			}
			e.compiled = r
		}
		return e.compiled.MatchString(command)
	}

	lowerMatch := strings.ToLower(e.Match)
	lowerCmd := strings.ToLower(command)

	if !strings.HasPrefix(lowerCmd, lowerMatch) {
		return false
	}
	// Word boundary check: exact match or followed by a space
	return len(lowerCmd) == len(lowerMatch) || lowerCmd[len(lowerMatch)] == ' '
}

// Section holds an ordered list of entries and a default action.
type Section struct {
	Default Action  // Action when no entry matches
	Entries []Entry // Ordered by Number (ascending)
}

const numberStep = uint32(10)

// Append adds a new entry at the end with auto-assigned number.
func (s *Section) Append(action Action, match string, regex bool) {
	var next uint32
	if len(s.Entries) == 0 {
		next = numberStep
	} else {
		next = s.Entries[len(s.Entries)-1].Number + numberStep
	}
	s.Entries = append(s.Entries, Entry{
		Number: next,
		Action: action,
		Match:  match,
		Regex:  regex,
	})
}

// InsertBefore inserts a new entry before the entry with the given number.
// The new entry gets a number midway between the previous entry and the target.
// If the gap is too small (< 2), all entries are renumbered.
func (s *Section) InsertBefore(beforeNum uint32, action Action, match string, regex bool) {
	idx := -1
	for i, e := range s.Entries {
		if e.Number == beforeNum {
			idx = i
			break
		}
	}
	if idx < 0 {
		s.Append(action, match, regex)
		return
	}

	var prevNum uint32
	if idx > 0 {
		prevNum = s.Entries[idx-1].Number
	}

	gap := beforeNum - prevNum
	if gap < 2 {
		// Insert at position, then renumber
		newEntry := Entry{Action: action, Match: match, Regex: regex}
		s.Entries = append(s.Entries[:idx], append([]Entry{newEntry}, s.Entries[idx:]...)...)
		s.renumber()
		return
	}

	newNum := prevNum + gap/2
	newEntry := Entry{Number: newNum, Action: action, Match: match, Regex: regex}
	s.Entries = append(s.Entries[:idx], append([]Entry{newEntry}, s.Entries[idx:]...)...)
}

// InsertAfter inserts a new entry after the entry with the given number.
// The new entry gets a number midway between the target and the next entry.
// If there is no next entry, uses target + step.
// If the gap is too small (< 2), all entries are renumbered.
func (s *Section) InsertAfter(afterNum uint32, action Action, match string, regex bool) {
	idx := -1
	for i, e := range s.Entries {
		if e.Number == afterNum {
			idx = i
			break
		}
	}
	if idx < 0 {
		s.Append(action, match, regex)
		return
	}

	insertPos := idx + 1

	var nextNum uint32
	if insertPos < len(s.Entries) {
		nextNum = s.Entries[insertPos].Number
	} else {
		nextNum = afterNum + numberStep*2
	}

	gap := nextNum - afterNum
	if gap < 2 {
		newEntry := Entry{Action: action, Match: match, Regex: regex}
		s.Entries = append(s.Entries[:insertPos], append([]Entry{newEntry}, s.Entries[insertPos:]...)...)
		s.renumber()
		return
	}

	newNum := afterNum + gap/2
	newEntry := Entry{Number: newNum, Action: action, Match: match, Regex: regex}
	s.Entries = append(s.Entries[:insertPos], append([]Entry{newEntry}, s.Entries[insertPos:]...)...)
}

// renumber reassigns all entry numbers to 10, 20, 30, ...
func (s *Section) renumber() {
	for i := range s.Entries {
		s.Entries[i].Number = uint32(i+1) * numberStep
	}
}

// evaluate walks entries in order and returns the action of the first match.
// Returns the section default if no entry matches.
func (s *Section) evaluate(command string) Action {
	for i := range s.Entries {
		if s.Entries[i].matches(command) {
			return s.Entries[i].Action
		}
	}
	return s.Default
}

// tokensContainInOrder reports whether every lowercased token in target appears
// in tokens in the same relative order.
func tokensContainInOrder(tokens, target []string) bool {
	if len(target) == 0 {
		return false
	}
	if len(tokens) < len(target) {
		return false
	}
	ti := 0
	for _, tok := range tokens {
		if tok == target[ti] {
			ti++
			if ti == len(target) {
				return true
			}
		}
	}
	return false
}

// matchedCanonicalIndices locates the index in typedLower of each token in
// canonicalTokens in order. Returns (nil, false) when typedLower does not
// contain every canonical token in order.
func matchedCanonicalIndices(typedLower, canonicalTokens []string) ([]int, bool) {
	if len(canonicalTokens) == 0 {
		return nil, false
	}
	if len(typedLower) < len(canonicalTokens) {
		return nil, false
	}
	indices := make([]int, 0, len(canonicalTokens))
	ci := 0
	for i, tok := range typedLower {
		if tok == canonicalTokens[ci] {
			indices = append(indices, i)
			ci++
			if ci == len(canonicalTokens) {
				return indices, true
			}
		}
	}
	return nil, false
}

func (e *Entry) compiledRegexp() (*regexp.Regexp, bool) {
	if e.compiled != nil {
		return e.compiled, true
	}
	r, err := regexp.Compile(e.Match)
	if err != nil {
		return nil, false
	}
	return r, true
}

// matchesExplicitAllow reports whether an Allow entry explicitly grants the
// command described by (canonicalPath, canonicalTokens, typedCommand,
// typedRawTokens, canonicalIndices) without relying on a broad prefix or
// wildcard regex.
func (e *Entry) matchesExplicitAllow(
	canonicalPath string,
	canonicalTokens []string,
	typedCommand string,
	typedRawTokens []string,
	canonicalIndices []int,
) bool {
	if e.Regex {
		return e.matchesExplicitAllowRegex(canonicalPath, canonicalTokens, typedCommand, typedRawTokens, canonicalIndices)
	}
	matchTokens := strings.Fields(strings.ToLower(e.Match))
	if len(matchTokens) == 0 {
		return false
	}
	if slices.Equal(matchTokens, canonicalTokens) {
		return true
	}
	return e.matches(typedCommand) && tokensContainInOrder(matchTokens, canonicalTokens)
}

func (e *Entry) matchesExplicitAllowRegex(
	canonicalPath string,
	canonicalTokens []string,
	typedCommand string,
	typedRawTokens []string,
	canonicalIndices []int,
) bool {
	r, ok := e.compiledRegexp()
	if !ok {
		return false
	}
	if !r.MatchString(typedCommand) && !r.MatchString(canonicalPath) {
		return false
	}
	// Reject wildcard or overly broad regexes that match the empty string or
	// any proper prefix of the canonical command path.
	if r.MatchString("") {
		return false
	}
	for i := 1; i < len(canonicalTokens); i++ {
		if r.MatchString(strings.Join(canonicalTokens[:i], " ")) {
			return false
		}
	}
	// Reject regexes that match any proper prefix of the typed command before
	// the final canonical command token appears.
	lastCanonicalIdx := canonicalIndices[len(canonicalIndices)-1]
	for i := 1; i <= lastCanonicalIdx; i++ {
		if r.MatchString(strings.Join(typedRawTokens[:i], " ")) {
			return false
		}
	}
	// Reject regexes that still match when any single canonical command token
	// is removed (prevents unanchored or wildcard-middle regexes from granting
	// explicit commands via sibling-command patterns).
	if len(canonicalTokens) == 1 {
		probe := "unrelated"
		if canonicalTokens[0] == probe {
			probe = "other"
		}
		return !r.MatchString(probe)
	}
	omittedTyped := make([]string, 0, len(typedRawTokens)-1)
	for _, skipIdx := range canonicalIndices {
		omittedTyped = omittedTyped[:0]
		omittedTyped = append(omittedTyped, typedRawTokens[:skipIdx]...)
		omittedTyped = append(omittedTyped, typedRawTokens[skipIdx+1:]...)
		if r.MatchString(strings.Join(omittedTyped, " ")) {
			return false
		}
	}
	omittedCanonical := make([]string, 0, len(canonicalTokens)-1)
	for skipIdx := range canonicalTokens {
		omittedCanonical = omittedCanonical[:0]
		omittedCanonical = append(omittedCanonical, canonicalTokens[:skipIdx]...)
		omittedCanonical = append(omittedCanonical, canonicalTokens[skipIdx+1:]...)
		if r.MatchString(strings.Join(omittedCanonical, " ")) {
			return false
		}
	}
	return true
}

// evaluateExplicit walks entries in order for a default-deny explicit-grant
// command. It never falls back to s.Default.
func (s *Section) evaluateExplicit(
	canonicalPath string,
	canonicalTokens []string,
	typedCommand string,
	typedRawTokens []string,
	canonicalIndices []int,
) (Action, bool) {
	for i := range s.Entries {
		entry := &s.Entries[i]
		if entry.Action == Deny {
			if entry.matches(typedCommand) {
				return Deny, true
			}
			if entry.matches(canonicalPath) {
				return Deny, true
			}
			continue
		}
		if entry.matchesExplicitAllow(canonicalPath, canonicalTokens, typedCommand, typedRawTokens, canonicalIndices) {
			return Allow, true
		}
	}
	return Deny, false
}

// Profile defines authorization rules for a set of commands.
type Profile struct {
	Name string  // Profile identifier
	Run  Section // Operational commands (ReadOnly=true)
	Edit Section // Configuration commands (ReadOnly=false)
}

// Authorize checks if a command is allowed under this profile.
// isReadOnly determines which section to evaluate (run or edit).
func (p *Profile) Authorize(command string, isReadOnly bool) Action {
	if isReadOnly {
		return p.Run.evaluate(command)
	}
	return p.Edit.evaluate(command)
}

// Validate checks that the profile is well-formed.
func (p *Profile) Validate() error {
	if p.Name == "" {
		return errProfileNameCannotBeEmpty
	}
	for i := range p.Run.Entries {
		if err := p.Run.Entries[i].Validate(); err != nil {
			return fmt.Errorf("profile %q run: %w", p.Name, err)
		}
	}
	for i := range p.Edit.Entries {
		if err := p.Edit.Entries[i].Validate(); err != nil {
			return fmt.Errorf("profile %q edit: %w", p.Name, err)
		}
	}
	return nil
}

// builtinAdminProfile returns the built-in admin profile (allow all).
func builtinAdminProfile() Profile {
	return Profile{
		Name: "admin",
		Run:  Section{Default: Allow},
		Edit: Section{Default: Allow},
	}
}

// builtinReadOnlyProfile returns the built-in read-only profile.
func builtinReadOnlyProfile() Profile {
	return Profile{
		Name: "read-only",
		// enumeration: exempt (these are the command prefixes the read-only profile denies, a policy list, not the CLI verb vocabulary)
		Run: Section{Default: Allow, Entries: []Entry{
			{Number: 10, Action: Deny, Match: "restart"},
			{Number: 20, Action: Deny, Match: "kill"},
			{Number: 30, Action: Deny, Match: "clear"},
			// spec-ospf-ext-14 AC-16: deny every `debug` command (both OSPF address
			// families' inject paths start with `debug`). This is the first of two
			// independent gates; the engine debug-enablement is the second.
			{Number: 40, Action: Deny, Match: "debug"},
		}},
		Edit: Section{Default: Deny},
	}
}

// Store holds profiles and user-to-profile assignments.
// It is safe for concurrent use.
type Store struct {
	mu          sync.RWMutex
	profiles    map[string]*Profile // name -> profile
	assignments map[string][]string // username -> profile names
}

// NewStore creates an empty authorization store.
func NewStore() *Store {
	return &Store{
		profiles:    make(map[string]*Profile),
		assignments: make(map[string][]string),
	}
}

// AddProfile adds or replaces a profile in the store.
func (s *Store) AddProfile(p Profile) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.profiles[p.Name] = &p
}

// AssignProfiles sets the profile list for a user.
func (s *Store) AssignProfiles(username string, profileNames []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.assignments[username] = profileNames
}

// HasProfiles returns true if any profiles are defined.
func (s *Store) HasProfiles() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.profiles) > 0
}

// hasProfile returns true if a profile with the given name exists.
func (s *Store) hasProfile(name string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.profiles[name]
	return ok
}

// WalkEntries calls fn for each entry in every profile's run and edit sections.
func (s *Store) WalkEntries(fn func(profileName, section string, e Entry)) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, p := range s.profiles {
		for _, e := range p.Run.Entries {
			fn(p.Name, "run", e)
		}
		for _, e := range p.Edit.Entries {
			fn(p.Name, "edit", e)
		}
	}
}

// hasUserAssignments returns true if any user-to-profile assignments exist.
func (s *Store) hasUserAssignments() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.assignments) > 0
}

// Authorize checks if a user is allowed to execute a command.
//
// It FAILS CLOSED. A non-nil Store means the operator configured
// system.authorization, so authorization IS in use: an identity that resolves to
// no applicable profile is DENIED, never granted an implicit admin default. The
// "no authorization configured" case is carried by a NIL store one layer up
// (StoreAuthorizer.Authorize in register.go, and Dispatcher.isAuthorized), which
// never reaches this function; there is deliberately no permissive fall-through
// here. Whether any local user is assigned a profile is NOT an input: a
// TACACS/RADIUS-only box has profiles but no assignments, and its authenticated
// users must still resolve a profile or be denied.
//
// Four reserved cases govern daemon-injected and break-glass identities:
//   - The sole trusted in-process RPC caller, whose username exactly equals
//     ReservedInternalRPCIdentity (naming no bus participant), is allowed so
//     internal RPC dispatch keeps working. It is un-typeable, so no
//     authenticated identity can spoof it.
//   - A plugin identity carrying ReservedInternalPluginPrefix is NEVER granted
//     by prefix: it is evaluated against the endpoint's compiled bus command
//     profile (aaa.LookupPluginCommandProfile) through authz's own Entry.matches
//     prefix matcher and fails closed when no profile allows the command,
//     independent of whether operator profiles are configured.
//   - A shared-token or no-auth API caller whose username exactly equals
//     ReservedSharedAPIUsername is allowed after REST or gRPC classification.
//     The transport's read-only gate denies no-auth mutations first.
//   - The break-glass recovery admin, whose LOGIN-RESOLVED profiles include
//     ReservedRecoveryProfile (delivered to the `ze init` bootstrap admin, never
//     as a config assignment), is allowed so an operator can always reach a
//     misconfigured box.
//
// Every audit-worthy decision (a trusted-internal grant, a plugin profile
// grant or deny, a shared-API grant, a recovery grant, and each deny reason) is
// logged so an operator can distinguish "denied by profile" from "denied
// because no profile applied".
func (s *Store) Authorize(username, command string, isReadOnly bool) Action {
	return s.authorize(username, nil, false, command, isReadOnly)
}

// AuthorizeWithProfiles evaluates one authenticated session's resolved profile
// names. It does not consult the mutable username profile view.
func (s *Store) AuthorizeWithProfiles(username string, profiles []string, command string, isReadOnly bool) Action {
	return s.authorize(username, profiles, true, command, isReadOnly)
}

// AuthorizeExplicit evaluates whether username holds an explicit RBAC grant for
// a default-deny command. Unlike Authorize, it never falls back to a section's
// Default action, never allows via ReservedInternalRPCIdentity,
// ReservedSharedAPIUsername, or ReservedRecoveryProfile, and rejects broad
// prefixes or wildcard regexes.
func (s *Store) AuthorizeExplicit(username, canonicalPath, typedCommand string, isReadOnly bool) Action {
	return s.authorizeExplicit(username, nil, false, canonicalPath, typedCommand, isReadOnly)
}

// authorizeExplicitWithProfiles evaluates one authenticated session's resolved
// profile names for an explicit default-deny command.
func (s *Store) authorizeExplicitWithProfiles(
	username string,
	profiles []string,
	canonicalPath, typedCommand string,
	isReadOnly bool,
) Action {
	return s.authorizeExplicit(username, profiles, true, canonicalPath, typedCommand, isReadOnly)
}

func (s *Store) authorizeExplicit(
	username string,
	loginNames []string,
	hasLoginNames bool,
	canonicalPath, typedCommand string,
	isReadOnly bool,
) Action {
	if s == nil {
		return Deny
	}
	canonicalTokens := strings.Fields(strings.ToLower(canonicalPath))
	if len(canonicalTokens) == 0 {
		return Deny
	}
	typedRawTokens := strings.Fields(typedCommand)
	if len(typedRawTokens) == 0 {
		return Deny
	}
	typedLowerTokens := strings.Fields(strings.ToLower(typedCommand))
	canonicalIndices, ok := matchedCanonicalIndices(typedLowerTokens, canonicalTokens)
	if !ok {
		return Deny
	}

	if username == aaa.ReservedInternalRPCIdentity {
		authzLogger.Info("denied explicit command: internal RPC identity",
			"identity", username, "command", canonicalPath)
		return Deny
	}
	if endpoint, isPlugin := aaa.PluginEndpointFromIdentity(username); isPlugin {
		return authorizePluginIdentityExplicit(username, endpoint, canonicalPath, typedCommand, isReadOnly)
	}
	if strings.HasPrefix(username, aaa.ReservedInternalPluginPrefix) {
		authzLogger.Warn("denied explicit command: malformed plugin identity",
			"identity", username, "command", canonicalPath,
			"code", diagnostic.CodeDoctorPluginRequestRefused)
		return Deny
	}
	if username == aaa.ReservedSharedAPIUsername {
		authzLogger.Info("denied explicit command: shared API identity",
			"command", canonicalPath)
		return Deny
	}
	if username == "" {
		authzLogger.Info("denied explicit command: empty identity",
			"command", canonicalPath)
		return Deny
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	var profileNames []string
	var hasAssignment bool
	if hasLoginNames {
		// Explicit-grant commands never short-circuit Allow for
		// ReservedRecoveryProfile; only configured store profiles count.
		profileNames = make([]string, 0, len(loginNames))
		for _, name := range loginNames {
			if name == aaa.ReservedRecoveryProfile {
				continue
			}
			if s.profiles[name] != nil {
				profileNames = append(profileNames, name)
			}
		}
		hasAssignment = len(profileNames) > 0
	} else {
		profileNames, hasAssignment = s.assignments[username]
	}
	if !hasAssignment {
		authzLogger.Info("denied explicit command: no applicable profile",
			"username", username, "command", canonicalPath)
		return Deny
	}
	if len(profileNames) == 0 {
		authzLogger.Info("denied explicit command: no applicable profile",
			"username", username, "command", canonicalPath)
		return Deny
	}

	for _, name := range profileNames {
		if name == aaa.ReservedRecoveryProfile {
			continue
		}
		p := s.profiles[name]
		if p == nil {
			continue
		}

		var section *Section
		if isReadOnly {
			section = &p.Run
		} else {
			section = &p.Edit
		}

		if action, matched := section.evaluateExplicit(canonicalPath, canonicalTokens, typedCommand, typedRawTokens, canonicalIndices); matched {
			return action
		}
	}

	authzLogger.Info("denied explicit command: no explicit grant entry matched",
		"username", username, "command", canonicalPath)
	return Deny
}

func (s *Store) authorize(username string, loginNames []string, hasLoginNames bool, command string, isReadOnly bool) Action {
	// Trusted in-process RPC caller carrying no bus participant identity.
	// Match the complete identity rather than a prefix so no plugin or other
	// reserved name bypasses authorization profiles.
	if username == aaa.ReservedInternalRPCIdentity {
		authzLogger.Debug("authorized: trusted internal RPC caller",
			"identity", username, "command", command)
		return Allow
	}

	// Plugin identity injected at the plugin RPC boundary. Evaluated against
	// the endpoint's compiled bus command profile before operator store state
	// so plugin authorization never depends on operator profiles and never
	// holds s.mu across the profile lookup callback.
	if endpoint, isPlugin := aaa.PluginEndpointFromIdentity(username); isPlugin {
		return authorizePluginIdentity(username, endpoint, command, isReadOnly)
	}

	// Shared API identity injected only after REST or gRPC has classified a
	// validated shared-token caller or a no-auth caller. Match the complete
	// identity rather than a prefix so no other reserved name gains this grant.
	// No-auth mutations are rejected by the API read-only gate before this call.
	if username == aaa.ReservedSharedAPIUsername {
		authzLogger.Debug("authorized: shared API caller", "command", command)
		return Allow
	}

	// Fail closed: an empty identity is a bug (an RPC path that forgot to inject
	// an identity) or an attack, never a legitimate caller. Was: allow-all when
	// no local user was assigned a profile.
	if username == "" {
		authzLogger.Warn("denied: empty identity", "command", command)
		return Deny
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	var profileNames []string
	var hasAssignment bool
	if hasLoginNames {
		// The break-glass recovery admin is delivered here, never as a config
		// assignment. It is honored regardless of what the store defines, so a
		// strict default cannot lock an operator out of a box whose authorization
		// config is wrong or partial.
		if slices.Contains(loginNames, aaa.ReservedRecoveryProfile) {
			authzLogger.Info("authorized: break-glass recovery admin",
				"username", username, "command", command)
			return Allow
		}

		// Only names this store defines count. ValidateAuthzConfig checks local
		// user profile references but not remote AAA profile mappings. A remote
		// mapping can therefore name a profile that does not exist.
		profileNames = make([]string, 0, len(loginNames))
		for _, name := range loginNames {
			if s.profiles[name] != nil {
				profileNames = append(profileNames, name)
			}
		}
		hasAssignment = len(profileNames) > 0
	} else {
		profileNames, hasAssignment = s.assignments[username]
	}
	if !hasAssignment {
		// Fail closed: authenticated but no applicable profile resolved. Was:
		// the built-in admin profile's allow-all default whenever no local user
		// was assigned
		// (hasUsers == false) -- the privilege escalation this store's existence
		// now closes. The store existing IS the "authorization is in use" signal.
		authzLogger.Warn("denied: no applicable profile",
			"username", username, "command", command)
		return Deny
	}
	if len(profileNames) == 0 {
		authzLogger.Info("denied: no applicable profile",
			"username", username, "command", command)
		return Deny
	}

	// Multi-profile: first profile with a matching entry wins.
	// If no entry matches in any profile, first profile's default applies.
	var firstDefault *Action
	for _, name := range profileNames {
		p := s.profiles[name]
		if p == nil {
			continue // profile not found, skip
		}

		var section *Section
		if isReadOnly {
			section = &p.Run
		} else {
			section = &p.Edit
		}

		// Check entries for a match
		for i := range section.Entries {
			if section.Entries[i].matches(command) {
				return section.Entries[i].Action
			}
		}

		// Track first profile's default
		if firstDefault == nil {
			d := section.Default
			firstDefault = &d
		}
	}

	// No entry matched in any profile -> first profile's section default
	if firstDefault != nil {
		return *firstDefault
	}

	// Fail closed: every profile the assignment named was undefined. Was: admin
	// default (allow-all). Unreachable from config -- ValidateAuthzConfig rejects
	// undefined user and tacacs-profile references -- so this denies rather than
	// grants admin only on the direct Store API (assignment to a missing profile).
	authzLogger.Warn("denied: assigned profiles all undefined",
		"username", username, "command", command)
	return Deny
}

// compilePluginProfile builds an authorization Profile whose Run and Edit
// sections both evaluate allowedPrefixes with Default: Deny using authz's own
// word-boundary prefix matcher (Entry.matches). Empty or whitespace-only
// prefixes are skipped so they can never act as a wildcard allow.
func compilePluginProfile(endpoint string, allowedPrefixes []string) Profile {
	section := Section{
		Default: Deny,
		Entries: make([]Entry, 0, len(allowedPrefixes)),
	}
	for _, raw := range allowedPrefixes {
		prefix := strings.TrimSpace(raw)
		if prefix == "" {
			continue
		}
		section.Append(Allow, prefix, false)
	}
	return Profile{
		Name: aaa.ReservedInternalPluginPrefix + endpoint,
		Run:  section,
		Edit: section,
	}
}

// authorizePluginIdentity evaluates a plugin identity against its published bus
// command profile through authz's own Profile/Entry matcher. Fails closed when
// no profile is published, AllowNone is set, or AllowedPrefixes contains no
// matching prefix.
func authorizePluginIdentity(username, endpoint, command string, isReadOnly bool) Action {
	prof, ok := aaa.LookupPluginCommandProfile(endpoint)
	if !ok {
		authzLogger.Warn("denied: plugin identity has no command profile",
			"identity", username, "endpoint", endpoint, "command", command,
			"code", diagnostic.CodeDoctorPluginRequestRefused)
		return Deny
	}
	if prof.AllowNone {
		authzLogger.Warn("denied: plugin command profile allows no commands",
			"identity", username, "endpoint", endpoint, "command", command,
			"code", diagnostic.CodeDoctorPluginRequestRefused)
		return Deny
	}
	if len(prof.AllowedPrefixes) == 0 {
		authzLogger.Warn("denied: plugin command profile has no allowed prefixes",
			"identity", username, "endpoint", endpoint, "command", command,
			"code", diagnostic.CodeDoctorPluginRequestRefused)
		return Deny
	}

	profile := compilePluginProfile(endpoint, prof.AllowedPrefixes)
	if len(profile.Run.Entries) == 0 {
		authzLogger.Warn("denied: plugin command profile has no non-empty prefixes",
			"identity", username, "endpoint", endpoint, "command", command,
			"code", diagnostic.CodeDoctorPluginRequestRefused)
		return Deny
	}

	if profile.Authorize(command, isReadOnly) == Allow {
		authzLogger.Debug("authorized: plugin identity by bus profile",
			"identity", username, "endpoint", endpoint, "command", command)
		return Allow
	}

	authzLogger.Warn("denied: plugin command not permitted by bus profile",
		"identity", username, "endpoint", endpoint, "command", command,
		"code", diagnostic.CodeDoctorPluginRequestRefused)
	return Deny
}

// authorizePluginIdentityExplicit evaluates a plugin identity against its
// compiled bus command profile under explicit default-deny entry rules. Broad
// prefixes (such as "show" or "show pki") do not satisfy explicit-grant
// commands; the compiled bus profile must explicitly grant the command path.
func authorizePluginIdentityExplicit(username, endpoint, canonicalPath, typedCommand string, isReadOnly bool) Action {
	canonicalTokens := strings.Fields(strings.ToLower(canonicalPath))
	if len(canonicalTokens) == 0 {
		return Deny
	}
	typedRawTokens := strings.Fields(typedCommand)
	if len(typedRawTokens) == 0 {
		return Deny
	}
	typedLowerTokens := strings.Fields(strings.ToLower(typedCommand))
	canonicalIndices, ok := matchedCanonicalIndices(typedLowerTokens, canonicalTokens)
	if !ok {
		return Deny
	}

	prof, ok := aaa.LookupPluginCommandProfile(endpoint)
	if !ok {
		authzLogger.Warn("denied explicit command: plugin identity has no command profile",
			"identity", username, "endpoint", endpoint, "command", canonicalPath,
			"code", diagnostic.CodeDoctorPluginRequestRefused)
		return Deny
	}
	if prof.AllowNone {
		authzLogger.Warn("denied explicit command: plugin command profile allows no commands",
			"identity", username, "endpoint", endpoint, "command", canonicalPath,
			"code", diagnostic.CodeDoctorPluginRequestRefused)
		return Deny
	}
	if len(prof.AllowedPrefixes) == 0 {
		authzLogger.Warn("denied explicit command: plugin command profile has no allowed prefixes",
			"identity", username, "endpoint", endpoint, "command", canonicalPath,
			"code", diagnostic.CodeDoctorPluginRequestRefused)
		return Deny
	}

	profile := compilePluginProfile(endpoint, prof.AllowedPrefixes)
	var section *Section
	if isReadOnly {
		section = &profile.Run
	} else {
		section = &profile.Edit
	}
	if len(section.Entries) == 0 {
		authzLogger.Warn("denied explicit command: plugin command profile has no non-empty prefixes",
			"identity", username, "endpoint", endpoint, "command", canonicalPath,
			"code", diagnostic.CodeDoctorPluginRequestRefused)
		return Deny
	}

	if action, matched := section.evaluateExplicit(canonicalPath, canonicalTokens, typedCommand, typedRawTokens, canonicalIndices); matched && action == Allow {
		authzLogger.Debug("authorized explicit command: plugin identity by bus profile",
			"identity", username, "endpoint", endpoint, "command", canonicalPath)
		return Allow
	}

	authzLogger.Warn("denied explicit command: plugin bus profile lacks explicit grant",
		"identity", username, "endpoint", endpoint, "command", canonicalPath,
		"code", diagnostic.CodeDoctorPluginRequestRefused)
	return Deny
}
