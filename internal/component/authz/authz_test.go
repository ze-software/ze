package authz

import (
	"testing"
)

// VALIDATES: Profile evaluation returns correct action for matching entries.
// PREVENTS: Authorization bypass due to incorrect entry matching.

func TestProfileEvaluateAllow(t *testing.T) {
	p := Profile{
		Name: "test",
		Run:  Section{Default: Allow, Entries: []Entry{{Number: 10, Action: Allow, Match: "peer show"}}},
		Edit: Section{Default: Deny},
	}
	if got := p.Authorize("peer show routes", true); got != Allow {
		t.Errorf("expected Allow, got %v", got)
	}
}

func TestProfileEvaluateDeny(t *testing.T) {
	p := Profile{
		Name: "test",
		Run:  Section{Default: Allow, Entries: []Entry{{Number: 10, Action: Deny, Match: "restart"}}},
		Edit: Section{Default: Deny},
	}
	if got := p.Authorize("restart", true); got != Deny {
		t.Errorf("expected Deny, got %v", got)
	}
}

func TestProfileEvaluateDefault(t *testing.T) {
	p := Profile{
		Name: "test",
		Run:  Section{Default: Allow},
		Edit: Section{Default: Deny},
	}
	// No entries, run section defaults to allow
	if got := p.Authorize("anything", true); got != Allow {
		t.Errorf("run: expected Allow, got %v", got)
	}
	// No entries, edit section defaults to deny
	if got := p.Authorize("anything", false); got != Deny {
		t.Errorf("edit: expected Deny, got %v", got)
	}
}

func TestProfileFirstMatchWins(t *testing.T) {
	p := Profile{
		Name: "test",
		Run: Section{Default: Deny, Entries: []Entry{
			{Number: 10, Action: Deny, Match: "peer show secret"},
			{Number: 20, Action: Allow, Match: "peer show"},
		}},
		Edit: Section{Default: Deny},
	}
	// "peer show secret" matches entry 10 (deny) first
	if got := p.Authorize("peer show secret", true); got != Deny {
		t.Errorf("expected Deny for 'peer show secret', got %v", got)
	}
	// "peer show routes" matches entry 20 (allow)
	if got := p.Authorize("peer show routes", true); got != Allow {
		t.Errorf("expected Allow for 'peer show routes', got %v", got)
	}
}

func TestProfilePrefixMatch(t *testing.T) {
	p := Profile{
		Name: "test",
		Run: Section{Default: Deny, Entries: []Entry{
			{Number: 10, Action: Allow, Match: "peer show"},
		}},
		Edit: Section{Default: Deny},
	}
	tests := []struct {
		name    string
		command string
		want    Action
	}{
		{"exact match", "peer show", Allow},
		{"prefix match", "peer show routes", Allow},
		{"prefix match deeper", "peer show routes detail", Allow},
		{"no match different command", "peer list", Deny},
		{"no match partial word", "peer shower", Deny},
		{"no match empty", "", Deny},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := p.Authorize(tt.command, true); got != tt.want {
				t.Errorf("Authorize(%q) = %v, want %v", tt.command, got, tt.want)
			}
		})
	}
}

func TestProfileRegexMatch(t *testing.T) {
	p := Profile{
		Name: "test",
		Run: Section{Default: Deny, Entries: []Entry{
			{Number: 10, Action: Allow, Match: "peer .* show", Regex: true},
		}},
		Edit: Section{Default: Deny},
	}
	tests := []struct {
		name    string
		command string
		want    Action
	}{
		{"regex matches", "peer 10.0.0.1 show", Allow},
		{"regex matches wildcard", "peer * show", Allow},
		{"regex no match", "peer 10.0.0.1 list", Deny},
		{"regex no match different", "bgp rib show", Deny},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := p.Authorize(tt.command, true); got != tt.want {
				t.Errorf("Authorize(%q) = %v, want %v", tt.command, got, tt.want)
			}
		})
	}
}

func TestProfileRegexNoMatch(t *testing.T) {
	p := Profile{
		Name: "test",
		Run: Section{Default: Allow, Entries: []Entry{
			{Number: 10, Action: Deny, Match: "^restart$", Regex: true},
		}},
		Edit: Section{Default: Deny},
	}
	// "restart" matches exactly
	if got := p.Authorize("restart", true); got != Deny {
		t.Errorf("expected Deny for exact 'restart', got %v", got)
	}
	// "restart now" does not match ^restart$
	if got := p.Authorize("restart now", true); got != Allow {
		t.Errorf("expected Allow for 'restart now' (no regex match), got %v", got)
	}
}

func TestProfileSectionSelection(t *testing.T) {
	p := Profile{
		Name: "test",
		Run:  Section{Default: Allow},
		Edit: Section{Default: Deny},
	}
	// isReadOnly=true -> run section
	if got := p.Authorize("anything", true); got != Allow {
		t.Errorf("run section: expected Allow, got %v", got)
	}
	// isReadOnly=false -> edit section
	if got := p.Authorize("anything", false); got != Deny {
		t.Errorf("edit section: expected Deny, got %v", got)
	}
}

func TestProfileCaseInsensitive(t *testing.T) {
	p := Profile{
		Name: "test",
		Run: Section{Default: Deny, Entries: []Entry{
			{Number: 10, Action: Allow, Match: "peer show"},
		}},
		Edit: Section{Default: Deny},
	}
	tests := []struct {
		command string
		want    Action
	}{
		{"peer show", Allow},
		{"PEER SHOW", Allow},
		{"Peer Show", Allow},
		{"PEER show routes", Allow},
	}
	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			if got := p.Authorize(tt.command, true); got != tt.want {
				t.Errorf("Authorize(%q) = %v, want %v", tt.command, got, tt.want)
			}
		})
	}
}

// --- Store tests ---

func TestStoreAuthorizeNoProfiles(t *testing.T) {
	// VALIDATES: user with no profile gets admin (allow all)
	// PREVENTS: users locked out when no profile assigned
	s := NewStore()
	if got := s.Authorize("someuser", "restart", true); got != Allow {
		t.Errorf("no profiles: expected Allow (admin default), got %v", got)
	}
	if got := s.Authorize("someuser", "config set", false); got != Allow {
		t.Errorf("no profiles (edit): expected Allow (admin default), got %v", got)
	}
}

func TestStoreAuthorizeNoAuth(t *testing.T) {
	// VALIDATES: empty username (no auth configured) allows all
	// PREVENTS: breaking backwards compatibility when no auth
	s := NewStore()
	if got := s.Authorize("", "restart", true); got != Allow {
		t.Errorf("empty user: expected Allow, got %v", got)
	}
}

func TestStoreAuthorizeWithProfile(t *testing.T) {
	s := NewStore()
	s.AddProfile(Profile{
		Name: "noc",
		Run:  Section{Default: Allow, Entries: []Entry{{Number: 10, Action: Deny, Match: "restart"}}},
		Edit: Section{Default: Deny, Entries: []Entry{{Number: 10, Action: Allow, Match: "router bgp"}}},
	})
	s.AssignProfiles("noc-user", []string{"noc"})

	tests := []struct {
		name     string
		command  string
		readOnly bool
		want     Action
	}{
		{"run allowed", "peer show", true, Allow},
		{"run denied", "restart", true, Deny},
		{"edit allowed", "router bgp", false, Allow},
		{"edit denied", "router ospf", false, Deny},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := s.Authorize("noc-user", tt.command, tt.readOnly); got != tt.want {
				t.Errorf("Authorize(%q, %q, %v) = %v, want %v",
					"noc-user", tt.command, tt.readOnly, got, tt.want)
			}
		})
	}
}

func TestStoreMultiProfile(t *testing.T) {
	// VALIDATES: first profile with matching entry wins
	// PREVENTS: incorrect multi-profile evaluation order
	s := NewStore()
	s.AddProfile(Profile{
		Name: "restricted",
		Run: Section{Default: Deny, Entries: []Entry{
			{Number: 10, Action: Allow, Match: "peer show"},
		}},
		Edit: Section{Default: Deny},
	})
	s.AddProfile(Profile{
		Name: "ops",
		Run: Section{Default: Allow, Entries: []Entry{
			{Number: 10, Action: Deny, Match: "kill"},
		}},
		Edit: Section{Default: Deny},
	})
	// User has restricted first, then ops
	s.AssignProfiles("user1", []string{"restricted", "ops"})

	tests := []struct {
		name    string
		command string
		want    Action
	}{
		{"restricted allows peer show", "peer show", Allow},
		{"ops denies kill", "kill", Deny},
		{"restricted denies (default) unknown", "summary", Deny},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := s.Authorize("user1", tt.command, true); got != tt.want {
				t.Errorf("Authorize(%q) = %v, want %v", tt.command, got, tt.want)
			}
		})
	}
}

func TestStoreMultiProfileFirstMatchWins(t *testing.T) {
	// VALIDATES: when both profiles have matching entries, first profile's match wins
	s := NewStore()
	s.AddProfile(Profile{
		Name: "profile-a",
		Run: Section{Default: Deny, Entries: []Entry{
			{Number: 10, Action: Deny, Match: "restart"},
		}},
		Edit: Section{Default: Deny},
	})
	s.AddProfile(Profile{
		Name: "profile-b",
		Run: Section{Default: Allow, Entries: []Entry{
			{Number: 10, Action: Allow, Match: "restart"},
		}},
		Edit: Section{Default: Deny},
	})
	s.AssignProfiles("user1", []string{"profile-a", "profile-b"})

	// profile-a has a matching entry for "restart" (deny) -> wins over profile-b's allow
	if got := s.Authorize("user1", "restart", true); got != Deny {
		t.Errorf("expected Deny (first profile match wins), got %v", got)
	}
}

func TestStoreMultiProfileDefaultFallback(t *testing.T) {
	// VALIDATES: when no profile has matching entry, first profile's default applies
	s := NewStore()
	s.AddProfile(Profile{
		Name: "p1",
		Run:  Section{Default: Allow},
		Edit: Section{Default: Deny},
	})
	s.AddProfile(Profile{
		Name: "p2",
		Run:  Section{Default: Deny},
		Edit: Section{Default: Allow},
	})
	s.AssignProfiles("user1", []string{"p1", "p2"})

	// No entries in either profile -> first profile's default (Allow for run)
	if got := s.Authorize("user1", "anything", true); got != Allow {
		t.Errorf("expected Allow (first profile default), got %v", got)
	}
}

func TestStoreProfileNotFound(t *testing.T) {
	// VALIDATES: user assigned a non-existent profile gets admin default
	s := NewStore()
	s.AssignProfiles("user1", []string{"nonexistent"})

	// Profile doesn't exist -> admin default (allow all)
	if got := s.Authorize("user1", "restart", true); got != Allow {
		t.Errorf("expected Allow (admin default for missing profile), got %v", got)
	}
}

func TestStoreOverrideBuiltinProfile(t *testing.T) {
	// VALIDATES: config-defined profile overrides built-in
	s := NewStore()
	// Override "admin" with a restricted version
	s.AddProfile(Profile{
		Name: "admin",
		Run:  Section{Default: Allow, Entries: []Entry{{Number: 10, Action: Deny, Match: "kill"}}},
		Edit: Section{Default: Allow},
	})
	s.AssignProfiles("user1", []string{"admin"})

	if got := s.Authorize("user1", "kill", true); got != Deny {
		t.Errorf("expected Deny (overridden admin), got %v", got)
	}
	if got := s.Authorize("user1", "peer show", true); got != Allow {
		t.Errorf("expected Allow (overridden admin allows rest), got %v", got)
	}
}

// --- Entry auto-numbering tests ---

func TestAutoNumber(t *testing.T) {
	// VALIDATES: entries are auto-numbered 10, 20, 30
	s := Section{Default: Deny}
	s.Append(Allow, "peer show", false)
	s.Append(Deny, "restart", false)
	s.Append(Allow, "summary", false)

	if len(s.Entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(s.Entries))
	}
	expected := []uint32{10, 20, 30}
	for i, e := range s.Entries {
		if e.Number != expected[i] {
			t.Errorf("entry %d: expected number %d, got %d", i, expected[i], e.Number)
		}
	}
}

func TestInsertBefore(t *testing.T) {
	// VALIDATES: insert before N creates entry between previous and N
	s := Section{Default: Deny}
	s.Append(Allow, "peer show", false) // 10
	s.Append(Allow, "summary", false)   // 20
	s.InsertBefore(20, Deny, "clear", false)

	if len(s.Entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(s.Entries))
	}
	// Should be: 10 (peer show), 15 (clear), 20 (summary)
	if s.Entries[1].Match != "clear" {
		t.Errorf("expected 'clear' at position 1, got %q", s.Entries[1].Match)
	}
	if s.Entries[1].Number != 15 {
		t.Errorf("expected number 15, got %d", s.Entries[1].Number)
	}
}

func TestInsertAfter(t *testing.T) {
	// VALIDATES: insert after N creates entry between N and next
	s := Section{Default: Deny}
	s.Append(Allow, "peer show", false) // 10
	s.Append(Allow, "summary", false)   // 20
	s.InsertAfter(10, Deny, "clear", false)

	if len(s.Entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(s.Entries))
	}
	// Should be: 10 (peer show), 15 (clear), 20 (summary)
	if s.Entries[1].Match != "clear" {
		t.Errorf("expected 'clear' at position 1, got %q", s.Entries[1].Match)
	}
	if s.Entries[1].Number != 15 {
		t.Errorf("expected number 15, got %d", s.Entries[1].Number)
	}
}

func TestInsertBeforeFirst(t *testing.T) {
	// VALIDATES: insert before the first entry
	s := Section{Default: Deny}
	s.Append(Allow, "peer show", false) // 10
	s.InsertBefore(10, Deny, "clear", false)

	if len(s.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(s.Entries))
	}
	// Should be: 5 (clear), 10 (peer show)
	if s.Entries[0].Match != "clear" {
		t.Errorf("expected 'clear' at position 0, got %q", s.Entries[0].Match)
	}
	if s.Entries[0].Number != 5 {
		t.Errorf("expected number 5, got %d", s.Entries[0].Number)
	}
}

func TestInsertAfterLast(t *testing.T) {
	// VALIDATES: insert after the last entry
	s := Section{Default: Deny}
	s.Append(Allow, "peer show", false) // 10
	s.InsertAfter(10, Deny, "clear", false)

	if len(s.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(s.Entries))
	}
	// Should be: 10 (peer show), 20 (clear)
	if s.Entries[1].Match != "clear" {
		t.Errorf("expected 'clear' at position 1, got %q", s.Entries[1].Match)
	}
	if s.Entries[1].Number != 20 {
		t.Errorf("expected number 20, got %d", s.Entries[1].Number)
	}
}

func TestRenumberOnTightGap(t *testing.T) {
	// VALIDATES: renumber triggered when gap < 2
	s := Section{Default: Deny}
	s.Entries = []Entry{
		{Number: 10, Action: Allow, Match: "a"},
		{Number: 11, Action: Allow, Match: "b"},
	}
	// Insert between 10 and 11 — gap is 1, should trigger renumber
	s.InsertBefore(11, Deny, "c", false)

	if len(s.Entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(s.Entries))
	}
	// After renumber: 10, 20, 30
	expected := []uint32{10, 20, 30}
	for i, e := range s.Entries {
		if e.Number != expected[i] {
			t.Errorf("entry %d: expected number %d, got %d", i, expected[i], e.Number)
		}
	}
	// Order should be: a, c, b
	expectedMatch := []string{"a", "c", "b"}
	for i, e := range s.Entries {
		if e.Match != expectedMatch[i] {
			t.Errorf("entry %d: expected match %q, got %q", i, expectedMatch[i], e.Match)
		}
	}
}

// --- Validation tests ---

func TestValidateRegexValid(t *testing.T) {
	e := Entry{Number: 10, Action: Allow, Match: "peer .* show", Regex: true}
	if err := e.Validate(); err != nil {
		t.Errorf("expected valid regex, got error: %v", err)
	}
}

func TestValidateRegexInvalid(t *testing.T) {
	e := Entry{Number: 10, Action: Allow, Match: "peer [invalid", Regex: true}
	if err := e.Validate(); err == nil {
		t.Error("expected error for invalid regex, got nil")
	}
}

func TestValidateEmptyMatch(t *testing.T) {
	e := Entry{Number: 10, Action: Allow, Match: ""}
	if err := e.Validate(); err == nil {
		t.Error("expected error for empty match, got nil")
	}
}

func TestValidateProfileValid(t *testing.T) {
	p := Profile{
		Name: "test",
		Run:  Section{Default: Allow},
		Edit: Section{Default: Deny, Entries: []Entry{{Number: 10, Action: Allow, Match: "router bgp"}}},
	}
	if err := p.Validate(); err != nil {
		t.Errorf("expected valid profile, got error: %v", err)
	}
}

func TestValidateProfileEmptyName(t *testing.T) {
	p := Profile{
		Name: "",
		Run:  Section{Default: Allow},
		Edit: Section{Default: Deny},
	}
	if err := p.Validate(); err == nil {
		t.Error("expected error for empty profile name, got nil")
	}
}

// --- Action string tests ---

func TestActionString(t *testing.T) {
	if Allow.String() != "allow" {
		t.Errorf("Allow.String() = %q, want %q", Allow.String(), "allow")
	}
	if Deny.String() != "deny" {
		t.Errorf("Deny.String() = %q, want %q", Deny.String(), "deny")
	}
}

// --- Edge cases ---

func TestProfileEmptyCommand(t *testing.T) {
	p := Profile{
		Name: "test",
		Run: Section{Default: Deny, Entries: []Entry{
			{Number: 10, Action: Allow, Match: "peer show"},
		}},
		Edit: Section{Default: Deny},
	}
	// Empty command should not match any entry, falls to default (deny)
	if got := p.Authorize("", true); got != Deny {
		t.Errorf("empty command: expected Deny, got %v", got)
	}
}

func TestProfileMatchBoundary(t *testing.T) {
	// VALIDATES: "peer show" does not match "peer shower" (word boundary)
	p := Profile{
		Name: "test",
		Run: Section{Default: Deny, Entries: []Entry{
			{Number: 10, Action: Allow, Match: "peer show"},
		}},
		Edit: Section{Default: Deny},
	}
	if got := p.Authorize("peer shower", true); got != Deny {
		t.Errorf("expected Deny for 'peer shower' (not word boundary), got %v", got)
	}
}

func TestProfileMatchExactCommandLength(t *testing.T) {
	// VALIDATES: exact length match works
	p := Profile{
		Name: "test",
		Run: Section{Default: Deny, Entries: []Entry{
			{Number: 10, Action: Allow, Match: "restart"},
		}},
		Edit: Section{Default: Deny},
	}
	if got := p.Authorize("restart", true); got != Allow {
		t.Errorf("expected Allow for exact 'restart', got %v", got)
	}
}

func TestStoreHasProfiles(t *testing.T) {
	s := NewStore()
	if s.HasProfiles() {
		t.Error("empty store should report no profiles")
	}
	s.AddProfile(Profile{Name: "test", Run: Section{Default: Allow}, Edit: Section{Default: Allow}})
	if !s.HasProfiles() {
		t.Error("store with profile should report has profiles")
	}
}

func TestStoreHasProfile(t *testing.T) {
	s := NewStore()
	if s.HasProfile("test") {
		t.Error("empty store should not have profile 'test'")
	}
	s.AddProfile(Profile{Name: "test", Run: Section{Default: Allow}, Edit: Section{Default: Allow}})
	if !s.HasProfile("test") {
		t.Error("store should have profile 'test' after adding it")
	}
	if s.HasProfile("other") {
		t.Error("store should not have profile 'other'")
	}
}

func TestStoreWalkEntries(t *testing.T) {
	s := NewStore()
	s.AddProfile(Profile{
		Name: "noc",
		Run: Section{Default: Deny, Entries: []Entry{
			{Number: 10, Action: Allow, Match: "peer show"},
			{Number: 20, Action: Deny, Match: "restart"},
		}},
		Edit: Section{Default: Deny, Entries: []Entry{
			{Number: 10, Action: Allow, Match: "router bgp"},
		}},
	})

	var entries []string
	s.WalkEntries(func(profileName, section string, e Entry) {
		entries = append(entries, profileName+"/"+section+"/"+e.Match)
	})

	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d: %v", len(entries), entries)
	}
}

func TestStoreHasUserAssignments(t *testing.T) {
	s := NewStore()
	if s.HasUserAssignments() {
		t.Error("empty store should report no user assignments")
	}
	s.AssignProfiles("user1", []string{"admin"})
	if !s.HasUserAssignments() {
		t.Error("store with assignment should report has user assignments")
	}
}

func TestBuiltinAdminProfile(t *testing.T) {
	// VALIDATES: built-in admin profile allows everything
	admin := BuiltinAdminProfile()
	if got := admin.Authorize("restart", true); got != Allow {
		t.Errorf("admin run: expected Allow, got %v", got)
	}
	if got := admin.Authorize("config set", false); got != Allow {
		t.Errorf("admin edit: expected Allow, got %v", got)
	}
}

func TestBuiltinReadOnlyProfile(t *testing.T) {
	// VALIDATES: built-in read-only profile denies dangerous run commands and all edit
	ro := BuiltinReadOnlyProfile()
	tests := []struct {
		name     string
		command  string
		readOnly bool
		want     Action
	}{
		{"show allowed", "peer show", true, Allow},
		{"restart denied", "restart", true, Deny},
		{"kill denied", "kill", true, Deny},
		{"clear denied", "clear", true, Deny},
		{"edit denied", "router bgp", false, Deny},
		{"edit any denied", "anything", false, Deny},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ro.Authorize(tt.command, tt.readOnly); got != tt.want {
				t.Errorf("Authorize(%q, %v) = %v, want %v", tt.command, tt.readOnly, got, tt.want)
			}
		})
	}
}

func TestInsertBeforeNonExistent(t *testing.T) {
	// VALIDATES: InsertBefore with non-existent target falls back to Append.
	// PREVENTS: Silent failure when target entry number not found.
	s := Section{Default: Deny}
	s.Append(Allow, "peer show", false) // 10
	s.InsertBefore(999, Deny, "clear", false)

	if len(s.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(s.Entries))
	}
	// Should append as entry 20 (after the existing 10)
	if s.Entries[1].Match != "clear" {
		t.Errorf("expected 'clear' appended at end, got %q", s.Entries[1].Match)
	}
	if s.Entries[1].Number != 20 {
		t.Errorf("expected number 20 (appended), got %d", s.Entries[1].Number)
	}
}

func TestInsertAfterNonExistent(t *testing.T) {
	// VALIDATES: InsertAfter with non-existent target falls back to Append.
	// PREVENTS: Silent failure when target entry number not found.
	s := Section{Default: Deny}
	s.Append(Allow, "peer show", false) // 10
	s.InsertAfter(999, Deny, "clear", false)

	if len(s.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(s.Entries))
	}
	if s.Entries[1].Match != "clear" {
		t.Errorf("expected 'clear' appended at end, got %q", s.Entries[1].Match)
	}
	if s.Entries[1].Number != 20 {
		t.Errorf("expected number 20 (appended), got %d", s.Entries[1].Number)
	}
}

func TestStoreConcurrentAuthorize(t *testing.T) {
	// VALIDATES: Store.Authorize is safe for concurrent use.
	// PREVENTS: Data races under concurrent SSH sessions.
	s := NewStore()
	s.AddProfile(Profile{
		Name: "noc",
		Run:  Section{Default: Allow, Entries: []Entry{{Number: 10, Action: Deny, Match: "restart"}}},
		Edit: Section{Default: Deny},
	})
	s.AssignProfiles("user1", []string{"noc"})

	done := make(chan struct{})
	for range 10 {
		go func() {
			defer func() { done <- struct{}{} }()
			for range 100 {
				got := s.Authorize("user1", "peer show", true)
				if got != Allow {
					t.Errorf("concurrent: expected Allow for 'peer show', got %v", got)
				}
				got = s.Authorize("user1", "restart", true)
				if got != Deny {
					t.Errorf("concurrent: expected Deny for 'restart', got %v", got)
				}
			}
		}()
	}
	for range 10 {
		<-done
	}
}

func TestRegexInvalidPatternInMatches(t *testing.T) {
	// VALIDATES: Invalid regex in matches() returns false (safety net).
	// PREVENTS: Panic on malformed regex when Validate() was not called.
	e := Entry{Number: 10, Action: Allow, Match: "[invalid", Regex: true}
	if e.matches("anything") {
		t.Error("expected false for invalid regex, got true")
	}
}
