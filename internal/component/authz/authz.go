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

// BuiltinAdminProfile returns the built-in admin profile (allow all).
func BuiltinAdminProfile() Profile {
	return Profile{
		Name: "admin",
		Run:  Section{Default: Allow},
		Edit: Section{Default: Allow},
	}
}

// BuiltinReadOnlyProfile returns the built-in read-only profile.
func BuiltinReadOnlyProfile() Profile {
	return Profile{
		Name: "read-only",
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

// HasUserAssignments returns true if any user-to-profile assignments exist.
func (s *Store) HasUserAssignments() bool {
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
// Two reserved identities keep the strict default from bricking or breaking a box:
//   - A trusted in-process caller, whose username bears ReservedInternalPrefix
//     (injected at the plugin RPC boundary), is allowed so internal dispatch
//     keeps working. Its prefix is un-typeable, so no authenticated identity can
//     spoof it.
//   - The break-glass recovery admin, whose LOGIN-RESOLVED profiles include
//     ReservedRecoveryProfile (delivered to the `ze init` bootstrap admin, never
//     as a config assignment), is allowed so an operator can always reach a
//     misconfigured box.
//
// Every audit-worthy decision (a trusted-internal grant, a recovery grant, and
// each deny reason) is logged so an operator can distinguish "denied by profile"
// from "denied because no profile applied".
func (s *Store) Authorize(username, command string, isReadOnly bool) Action {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Trusted in-process caller injected at the RPC boundary. Not a config
	// subject: the prefix is un-typeable, so this cannot be reached by an
	// authenticated user or a config assignment.
	if strings.HasPrefix(username, aaa.ReservedInternalPrefix) {
		authzLogger.Debug("authorized: trusted internal caller",
			"identity", username, "command", command)
		return Allow
	}

	// Fail closed: an empty identity is a bug (an RPC path that forgot to inject
	// an identity) or an attack, never a legitimate caller. Was: allow-all when
	// no local user was assigned a profile.
	if username == "" {
		authzLogger.Warn("denied: empty identity", "command", command)
		return Deny
	}

	profileNames, hasAssignment := s.assignments[username]
	if !hasAssignment || len(profileNames) == 0 {
		// Fall back to the profiles the user's authentication resolved. A local
		// user's profiles reach us as a config assignment above, but a TACACS+
		// user's come from the server's priv-lvl reply via the tacacs-profile
		// mapping and appear nowhere in config keyed by username. Without this the
		// mapping is logged at login and then ignored: an unassigned user falls
		// through to the admin default below (or to Deny once any local user
		// exists), so priv-lvl 1 mapped to read-only would authorize as admin.
		//
		// Config assignments win when both exist: an explicit local assignment is
		// the operator's stated intent for that name.
		//
		// Only names this store actually defines count. ValidateAuthzConfig checks
		// user[*].profile references but NOT tacacs-profile ones, so a mapping may
		// name a profile that does not exist. Accepting such a name as an
		// assignment would be worse than ignoring it: the loop below skips every
		// unknown name, leaves firstDefault nil, and falls through to the admin
		// default -- turning a typo in tacacs-profile into allow-all. Dropping
		// them here leaves the decision to the fail-closed branch below.
		if loginNames, ok := aaa.LoginProfiles(username); ok {
			// The break-glass recovery admin is delivered here, never as a config
			// assignment (which would flip the operator's RBAC posture). It is
			// honored regardless of what the store defines, so a strict default
			// cannot lock an operator out of a box whose authorization config is
			// wrong or partial.
			if slices.Contains(loginNames, aaa.ReservedRecoveryProfile) {
				authzLogger.Info("authorized: break-glass recovery admin",
					"username", username, "command", command)
				return Allow
			}
			known := make([]string, 0, len(loginNames))
			for _, name := range loginNames {
				if s.profiles[name] != nil {
					known = append(known, name)
				}
			}
			if len(known) > 0 {
				profileNames = known
				hasAssignment = true
			}
		}
	}
	if !hasAssignment || len(profileNames) == 0 {
		// Fail closed: authenticated but no applicable profile resolved. Was:
		// BuiltinAdminProfile allow-all whenever no local user was assigned
		// (hasUsers == false) -- the privilege escalation this store's existence
		// now closes. The store existing IS the "authorization is in use" signal.
		authzLogger.Warn("denied: no applicable profile",
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
