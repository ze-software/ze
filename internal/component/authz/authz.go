// Design: (none — new authorization component)

// Package authz provides profile-based command authorization.
// Profiles contain ordered allow/deny entries matched against command paths.
// Each profile has two sections: run (operational) and edit (configuration).
package authz

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
)

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
		return fmt.Errorf("profile name cannot be empty")
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

// HasUserAssignments returns true if any user-to-profile assignments exist.
func (s *Store) HasUserAssignments() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.assignments) > 0
}

// Authorize checks if a user is allowed to execute a command.
// Empty username (no auth) always returns Allow.
// Users with no profile assignment get the built-in admin profile (allow all).
func (s *Store) Authorize(username, command string, isReadOnly bool) Action {
	// No authentication configured = allow all
	if username == "" {
		return Allow
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	profileNames, hasAssignment := s.assignments[username]
	if !hasAssignment || len(profileNames) == 0 {
		// No profile assigned -> built-in admin (allow all)
		admin := BuiltinAdminProfile()
		return admin.Authorize(command, isReadOnly)
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

	// All referenced profiles were missing -> admin default
	admin := BuiltinAdminProfile()
	return admin.Authorize(command, isReadOnly)
}
