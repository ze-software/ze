package authz

import (
	"testing"

	"github.com/ze-software/ze/internal/component/aaa"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
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
		{"regex no match different", "show bgp rib", Deny},
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
	// spec-fixit-authz-admin-fallthrough (Q-2, deny always): a named, authenticated
	// user who resolves NO applicable profile is DENIED, not granted admin. A
	// non-nil Store means authorization is in use, so "nothing applied" fails
	// closed. The "no authorization configured" case is a NIL store handled one
	// layer up (StoreAuthorizer), never an empty Store here.
	//
	// VALIDATES: authenticated user with no resolvable profile is denied (fail closed)
	// PREVENTS: the privilege escalation where an unassigned user became admin
	s := NewStore()
	if got := s.Authorize("someuser", "restart", true); got != Deny {
		t.Errorf("no profiles: expected Deny (fail closed), got %v", got)
	}
	if got := s.Authorize("someuser", "config set", false); got != Deny {
		t.Errorf("no profiles (edit): expected Deny (fail closed), got %v", got)
	}
}

func TestStoreAuthorizeNoAuth(t *testing.T) {
	// spec-fixit-authz-admin-fallthrough (S-1, Q-4/O-4): an EMPTY username reaching
	// a non-nil Store fails closed. An empty identity is a bug (an RPC path that
	// forgot to inject one) or an attack; the RPC boundary now injects an explicit
	// reserved internal identity, so a legitimate internal caller never arrives
	// empty. The no-auth-configured box is a NIL store (allowed one layer up).
	//
	// VALIDATES: empty username is denied by a non-nil store (fail closed)
	// PREVENTS: the S-1 hole where an empty username was granted allow-all
	s := NewStore()
	if got := s.Authorize("", "restart", true); got != Deny {
		t.Errorf("empty user: expected Deny (fail closed), got %v", got)
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
	// spec-fixit-authz-admin-fallthrough (S-3, AC-4): an assignment naming only
	// undefined profiles fails closed, not admin allow-all. Unreachable from config
	// (ValidateAuthzConfig rejects undefined user and tacacs-profile references),
	// so this only guards the direct Store API, but it must still deny.
	//
	// VALIDATES: assignment to a missing profile is denied (fail closed)
	// PREVENTS: a dangling assignment granting admin
	s := NewStore()
	s.AssignProfiles("user1", []string{"nonexistent"})

	if got := s.Authorize("user1", "restart", true); got != Deny {
		t.Errorf("expected Deny (fail closed for missing profile), got %v", got)
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
	if s.hasProfile("test") {
		t.Error("empty store should not have profile 'test'")
	}
	s.AddProfile(Profile{Name: "test", Run: Section{Default: Allow}, Edit: Section{Default: Allow}})
	if !s.hasProfile("test") {
		t.Error("store should have profile 'test' after adding it")
	}
	if s.hasProfile("other") {
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
	if s.hasUserAssignments() {
		t.Error("empty store should report no user assignments")
	}
	s.AssignProfiles("user1", []string{"admin"})
	if !s.hasUserAssignments() {
		t.Error("store with assignment should report has user assignments")
	}
}

func TestBuiltinAdminProfile(t *testing.T) {
	// VALIDATES: built-in admin profile allows everything
	admin := builtinAdminProfile()
	if got := admin.Authorize("restart", true); got != Allow {
		t.Errorf("admin run: expected Allow, got %v", got)
	}
	if got := admin.Authorize("config set", false); got != Allow {
		t.Errorf("admin edit: expected Allow, got %v", got)
	}
}

func TestBuiltinReadOnlyProfile(t *testing.T) {
	// VALIDATES: built-in read-only profile denies dangerous run commands and all edit
	ro := builtinReadOnlyProfile()
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
		{"clear l2tp denied", "clear l2tp session id 42", true, Deny},
		{"show l2tp allowed", "show l2tp sessions", true, Allow},
		{"debug denied", "debug", true, Deny},
		{"debug ip ospf inject denied", "debug ip ospf inject opaque scope area id 1", true, Deny},
		{"debug ipv6 ospf inject denied", "debug ipv6 ospf inject lsa scope area type 0x2009 id 1", true, Deny},
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

// TestInjectDeniedReadOnly / TestV3InjectDeniedReadOnly: spec-ospf-ext-14 AC-16, R-1 -- the
// read-only profile denies the OSPF debug LSA-injection commands (both families) before the
// engine is reached.
//
// Fidelity: the original assertion used Authorize(cmd, isReadOnly=true), which exercises the
// Run section's `deny "debug"` rule. But production dispatches inject with isReadOnly=FALSE
// (IsReadOnlyPath cuts on the first word and "debug" is not a read-only verb), so the denial
// that actually fires is the Edit section default (Deny), NOT the Run rule. These tests pin
// IsReadOnlyPath(inject)==false and assert the isReadOnly=false Store path also denies, so a
// future Edit.Default change cannot silently open inject while the true-path assertion stays green.
func TestInjectDeniedReadOnly(t *testing.T) {
	const (
		v4 = "debug ip ospf inject opaque scope area id 1 hex 00"
		v6 = "debug ipv6 ospf inject lsa scope area type 0x2009 id 1 hex 00"
	)
	ro := builtinReadOnlyProfile()
	// Original assertion: the Run-section `deny "debug"` rule (isReadOnly=true path).
	if got := ro.Authorize(v4, true); got != Deny {
		t.Fatalf("IPv4 inject Authorize(readOnly=true) = %v, want Deny", got)
	}

	// Pin the premise: inject dispatches with isReadOnly=false, so the true path is the Edit section.
	if pluginserver.IsReadOnlyPath(v4) {
		t.Fatalf("IsReadOnlyPath(%q) = true, want false (inject dispatches with isReadOnly=false)", v4)
	}
	if pluginserver.IsReadOnlyPath(v6) {
		t.Fatalf("IsReadOnlyPath(%q) = true, want false", v6)
	}

	// Guard the ACTUAL production path: a user under the read-only profile is denied inject on
	// the isReadOnly=false (Edit) path for both address families.
	s := NewStore()
	s.AddProfile(builtinReadOnlyProfile())
	s.AssignProfiles("ro-user", []string{"read-only"})
	if got := s.Authorize("ro-user", v4, false); got != Deny {
		t.Fatalf("Store.Authorize(v4 inject, isReadOnly=false) = %v, want Deny", got)
	}
	if got := s.Authorize("ro-user", v6, false); got != Deny {
		t.Fatalf("Store.Authorize(v6 inject, isReadOnly=false) = %v, want Deny", got)
	}
}

func TestV3InjectDeniedReadOnly(t *testing.T) {
	const v6 = "debug ipv6 ospf inject lsa scope area type 0x2009 id 1 hex 00"
	ro := builtinReadOnlyProfile()
	// Original assertion: the Run-section `deny "debug"` rule (isReadOnly=true path).
	if got := ro.Authorize(v6, true); got != Deny {
		t.Fatalf("IPv6 inject Authorize(readOnly=true) = %v, want Deny", got)
	}

	// Pin the premise and guard the true (isReadOnly=false / Edit) production path.
	if pluginserver.IsReadOnlyPath(v6) {
		t.Fatalf("IsReadOnlyPath(%q) = true, want false (inject dispatches with isReadOnly=false)", v6)
	}
	s := NewStore()
	s.AddProfile(builtinReadOnlyProfile())
	s.AssignProfiles("ro-user", []string{"read-only"})
	if got := s.Authorize("ro-user", v6, false); got != Deny {
		t.Fatalf("Store.Authorize(v6 inject, isReadOnly=false) = %v, want Deny", got)
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

// TestStoreAuthorizeUsesLoginResolvedProfiles verifies that a user whose profiles
// were resolved at authentication is authorized against those profiles.
//
// A TACACS+ user has no system.authentication.user block, so the store holds no
// assignment for their username. Their profiles come from the server's priv-lvl
// reply through the tacacs-profile mapping, bound to the authentication result.
//
// VALIDATES: tacacs-profile priv-lvl mapping governs command authorization.
// PREVENTS: regression to every TACACS+-authenticated user being authorized as
//
//	admin because the store found no config assignment and fell through to
//	the built-in admin profile.
func TestStoreAuthorizeUsesLoginResolvedProfiles(t *testing.T) {
	s := NewStore()
	s.AddProfile(Profile{
		Name: "read-only",
		Run:  Section{Default: Allow},
		Edit: Section{Default: Deny},
	})

	if got := s.AuthorizeWithProfiles("tacacs-noc", []string{"read-only"}, "show bgp", true); got != Allow {
		t.Errorf("run section defaults to allow: expected Allow, got %v", got)
	}
	if got := s.AuthorizeWithProfiles("tacacs-noc", []string{"read-only"}, "request quiesce", false); got != Deny {
		t.Errorf("edit section defaults to deny: expected Deny, got %v (admin fallthrough?)", got)
	}
}

// TestStoreAuthorizeLoginBindingWinsOverConfigAssignment verifies that a remote
// authentication result takes precedence over a same-name local assignment.
//
// VALIDATES: one authenticated session uses the profiles resolved for that
// credential and not mutable state keyed only by username.
// PREVENTS: a same-name local assignment widening a remote session's authority.
func TestStoreAuthorizeLoginBindingWinsOverConfigAssignment(t *testing.T) {
	s := NewStore()
	s.AddProfile(Profile{Name: "read-only", Run: Section{Default: Allow}, Edit: Section{Default: Deny}})
	s.AddProfile(Profile{Name: "admin-like", Run: Section{Default: Allow}, Edit: Section{Default: Allow}})
	s.AssignProfiles("dual", []string{"read-only"})

	if got := s.AuthorizeWithProfiles("dual", []string{"admin-like"}, "request quiesce", false); got != Allow {
		t.Errorf("login-bound profile must win: expected Allow, got %v", got)
	}
}

// TestStoreAuthorizeProfilesDoNotLeakAcrossUsers verifies that one request's
// profiles leave another user's decision untouched.
//
// VALIDATES: login-resolved profiles are request-scoped.
// PREVENTS: one user's login granting or restricting a different account.
func TestStoreAuthorizeProfilesDoNotLeakAcrossUsers(t *testing.T) {
	s := NewStore()
	s.AddProfile(Profile{Name: "read-only", Run: Section{Default: Allow}, Edit: Section{Default: Deny}})
	s.AssignProfiles("known", []string{"read-only"})

	// "other" has no assignment and no login-bound profiles. A non-nil store
	// means authorization is in use, so it fails closed.
	if got := s.Authorize("other", "show bgp", true); got != Deny {
		t.Errorf("unassigned user with no resolved profile must fail closed: got %v", got)
	}
}

// TestStoreAuthorizeIgnoresUnresolvableLoginProfiles verifies that login-resolved
// profile names the store does not define are ignored rather than treated as an
// assignment.
//
// ValidateAuthzConfig checks user[*].profile references but not tacacs-profile
// ones, so `tacacs-profile 1 { profile [ typo ]; }` loads fine. If such a name
// counted as an assignment, the profile loop would skip every unknown name, leave
// firstDefault nil, and fall through to the admin default.
//
// VALIDATES: an unresolvable mapping fails closed when assignments exist.
// PREVENTS: a typo in tacacs-profile authorizing that user as admin -- strictly
//
//	worse than the pre-fallback behavior, which denied them.
func TestStoreAuthorizeIgnoresUnresolvableLoginProfiles(t *testing.T) {
	s := NewStore()
	s.AddProfile(Profile{Name: "read-only", Run: Section{Default: Allow}, Edit: Section{Default: Deny}})
	// A local user is assigned only to demonstrate coexistence; the decision below
	// no longer depends on it -- a non-nil store fails closed for any name that
	// resolves no profile.
	s.AssignProfiles("local-admin", []string{"read-only"})

	if got := s.AuthorizeWithProfiles("tacacs-typo", []string{"does-not-exist"}, "request quiesce", false); got != Deny {
		t.Errorf("unresolvable profile name must not authorize as admin: got %v", got)
	}
	if got := s.AuthorizeWithProfiles("tacacs-typo", []string{"does-not-exist"}, "show bgp", true); got != Deny {
		t.Errorf("unresolvable profile name must not authorize as admin: got %v", got)
	}
}

// TestStoreAuthorizeKeepsResolvableLoginProfiles verifies that a partially valid
// mapping is honored through the names that do resolve.
//
// VALIDATES: one bad name in a leaf-list does not discard the good ones.
// PREVENTS: over-correcting the fail-open into a lockout.
func TestStoreAuthorizeKeepsResolvableLoginProfiles(t *testing.T) {
	s := NewStore()
	s.AddProfile(Profile{Name: "read-only", Run: Section{Default: Allow}, Edit: Section{Default: Deny}})

	if got := s.AuthorizeWithProfiles("mixed", []string{"does-not-exist", "read-only"}, "show bgp", true); got != Allow {
		t.Errorf("resolvable name must still apply: got %v", got)
	}
	if got := s.AuthorizeWithProfiles("mixed", []string{"does-not-exist", "read-only"}, "request quiesce", false); got != Deny {
		t.Errorf("resolvable name's edit deny must apply: got %v", got)
	}
}

// --- spec-fixit-authz-admin-fallthrough: the privilege-escalation fix ---

// TestStoreAuthorizeProfilesNoAssignmentsDeniesNotAdmin is the core vulnerability
// test. A TACACS/RADIUS-only box has system.authorization profiles configured but
// NO system.authentication.user assignments (assignments come only from local
// users). Before the fix, hasUsers was false and any authenticated user whose
// profiles did not resolve fell through to the built-in admin profile
// (allow-all). The store existing IS the "authorization is in use" signal,
// so this must deny.
//
// VALIDATES: AC-1 -- authenticated user, profiles configured, no assignment, no
//
//	login profile -> Deny, NOT admin.
//
// PREVENTS: the privilege escalation where a TACACS/RADIUS-only box authorized an
//
//	arbitrary authenticated user as admin.
func TestStoreAuthorizeProfilesNoAssignmentsDeniesNotAdmin(t *testing.T) {
	s := NewStore()
	// A profiles-with-no-assignments store is exactly what extractAuthzConfig
	// builds for a TACACS/RADIUS-only box.
	s.AddProfile(Profile{Name: "read-only", Run: Section{Default: Allow}, Edit: Section{Default: Deny}})

	// No assignment or login-bound profiles: the arbitrary authenticated user
	// resolves nothing.
	if got := s.Authorize("tacacs-user", "restart", true); got != Deny {
		t.Errorf("run: expected Deny (fail closed), got %v (admin fallthrough?)", got)
	}
	if got := s.Authorize("tacacs-user", "router bgp", false); got != Deny {
		t.Errorf("edit: expected Deny (fail closed), got %v (admin fallthrough?)", got)
	}
}

// TestStoreAuthorizeTacacsUnresolvedLoginDeniesNotAdmin covers the same box where
// the authenticated user DID resolve login profiles, but none names a profile the
// store defines (e.g. a tacacs-profile mapping to a name that does not exist).
// Before the fix this fell through to admin; it must deny.
//
// VALIDATES: AC-1 -- login profiles that do not resolve deny, not admin.
func TestStoreAuthorizeTacacsUnresolvedLoginDeniesNotAdmin(t *testing.T) {
	s := NewStore()
	s.AddProfile(Profile{Name: "read-only", Run: Section{Default: Allow}, Edit: Section{Default: Deny}})

	if got := s.AuthorizeWithProfiles("tacacs-priv15", []string{"does-not-exist"}, "restart", true); got != Deny {
		t.Errorf("expected Deny (fail closed), got %v (admin fallthrough?)", got)
	}
}

// TestStoreAuthorizeEmptyUsernameWithProfiles pins S-1 on a store that has
// profiles but no assignments (the shape where the old empty-username branch
// returned Allow because hasUsers was false).
//
// VALIDATES: AC-3 -- an empty username fails closed even when hasUsers would be false.
func TestStoreAuthorizeEmptyUsernameWithProfiles(t *testing.T) {
	s := NewStore()
	s.AddProfile(Profile{Name: "read-only", Run: Section{Default: Allow}, Edit: Section{Default: Deny}})

	if got := s.Authorize("", "restart", true); got != Deny {
		t.Errorf("empty username with profiles: expected Deny, got %v", got)
	}
}

// TestStoreAuthorizeReservedInternalIdentity pins O-4: a trusted in-process caller
// whose username bears aaa.ReservedInternalPrefix (injected at the RPC boundary) is
// allowed even on a strict store, so internal plugin dispatch keeps working. The
// descriptor after the prefix (here a plugin name) is preserved for audit and
// does not change the decision.
//
// VALIDATES: AC-3/AC-10 -- reserved internal identity authorizes; plugin dispatch works.
func TestStoreAuthorizeReservedInternalIdentity(t *testing.T) {
	s := NewStore()
	s.AddProfile(Profile{Name: "read-only", Run: Section{Default: Allow}, Edit: Section{Default: Deny}})
	s.AssignProfiles("operator", []string{"read-only"})

	rpc := aaa.ReservedInternalPrefix + "rpc"
	plug := aaa.ReservedInternalPrefix + "plugin:ospf"
	for _, id := range []string{rpc, plug} {
		if got := s.Authorize(id, "restart", true); got != Allow {
			t.Errorf("reserved internal identity %q run: expected Allow, got %v", id, got)
		}
		if got := s.Authorize(id, "router bgp", false); got != Allow {
			t.Errorf("reserved internal identity %q edit: expected Allow, got %v", id, got)
		}
	}
}

// VALIDATES: AC-7 and AC-8, the server-injected shared API identity crosses a
// strict authorization store for both read and write commands.
// PREVENTS: token and loopback no-auth callers being denied as an unassigned
// printable user after authorization profiles are configured.
func TestStoreAuthorizeReservedSharedAPIIdentity(t *testing.T) {
	s := NewStore()
	s.AddProfile(Profile{
		Name: "unrelated",
		Run:  Section{Default: Deny},
		Edit: Section{Default: Deny},
	})

	if got := s.Authorize(aaa.ReservedSharedAPIUsername, "show version", true); got != Allow {
		t.Errorf("shared API read: expected Allow, got %v", got)
	}
	if got := s.Authorize(aaa.ReservedSharedAPIUsername, "request reload", false); got != Allow {
		t.Errorf("shared API write: expected Allow, got %v", got)
	}
	for _, username := range []string{
		aaa.ReservedSharedAPIUsername + ":other",
		"shared-api",
		"api",
		"ordinary-config-user",
		"",
	} {
		if got := s.Authorize(username, "request reload", false); got != Deny {
			t.Errorf("non-shared identity %q: expected Deny, got %v", username, got)
		}
	}
}

// TestStoreAuthorizeRecoveryProfile pins O-3': the ze init bootstrap admin reaches
// a strict box via the reserved recovery profile, delivered through login-resolved
// profiles (never a config assignment). It is honored regardless of what the store
// defines, so a strict default cannot brick a profiles-but-no-config-admin box.
//
// VALIDATES: AC-2 -- break-glass recovery admin reaches a misconfigured box.
func TestStoreAuthorizeRecoveryProfile(t *testing.T) {
	s := NewStore()
	// A store with an unrelated profile and NO assignment for the admin: the
	// exposed profiles-but-no-config-admin shape.
	s.AddProfile(Profile{Name: "read-only", Run: Section{Default: Allow}, Edit: Section{Default: Deny}})

	aaa.SetAcceptedLocalProfileGeneration(1)
	t.Cleanup(func() { aaa.SetAcceptedLocalProfileGeneration(0) })
	authorizer := aaa.AuthorizerForResult(StoreAuthorizer{Store: s}, aaa.AuthResult{
		Source:          aaa.SourceLocal,
		Profiles:        []string{aaa.ReservedRecoveryProfile},
		LocalGeneration: 1,
	})

	if !authorizer.Authorize("admin", "", "restart", true) {
		t.Error("recovery run: expected allow")
	}
	if !authorizer.Authorize("admin", "", "router bgp", false) {
		t.Error("recovery edit: expected allow")
	}
}

// TestStoreAuthorizeConfigAssignmentWinsOverRecoveryName proves the recovery grant
// only fires through the login-resolved route. A config assignment (the operator's
// stated intent) is evaluated first, so even the reserved name in an assignment
// does not short-circuit to allow-all -- and the reserved name resolves to no
// defined profile, so it fails closed.
//
// VALIDATES: recovery is a login-route-only escape hatch, not a config back door.
func TestStoreAuthorizeConfigAssignmentWinsOverRecoveryName(t *testing.T) {
	s := NewStore()
	s.AddProfile(Profile{Name: "read-only", Run: Section{Default: Allow}, Edit: Section{Default: Deny}})
	s.AssignProfiles("evil", []string{aaa.ReservedRecoveryProfile})

	// The assignment names only the reserved recovery profile, which the store
	// does not define -> fails closed, does not become admin.
	if got := s.Authorize("evil", "restart", true); got != Deny {
		t.Errorf("reserved name as config assignment must fail closed: got %v", got)
	}
}

// TestIsReservedAuthzName guards the un-typeable namespace helper used by
// ValidateAuthzConfig to reject reserved names in configuration.
func TestIsReservedAuthzName(t *testing.T) {
	if !aaa.IsReservedName(aaa.ReservedRecoveryProfile) {
		t.Errorf("recovery profile must be reserved")
	}
	if !aaa.IsReservedName(aaa.ReservedInternalPrefix + "plugin:x") {
		t.Errorf("internal identity must be reserved")
	}
	if !aaa.IsReservedName(aaa.ReservedSharedAPIUsername) {
		t.Errorf("shared API identity must be reserved")
	}
	for _, name := range []string{"admin", "read-only", "operator", "plugin:ospf", ""} {
		if aaa.IsReservedName(name) {
			t.Errorf("%q must not be reserved (config-typeable names)", name)
		}
	}
}

// VALIDATES: one username can hold concurrent session-bound profile views.
// PREVENTS: a later login replacing the authorization of an established session.
func TestStoreAuthorizerBoundProfilesDoNotCrossSessions(t *testing.T) {
	s := NewStore()
	s.AddProfile(Profile{
		Name: "read-only",
		Run:  Section{Default: Allow},
		Edit: Section{Default: Deny},
	})
	s.AddProfile(Profile{
		Name: "operator",
		Run:  Section{Default: Deny},
		Edit: Section{Default: Allow},
	})
	base := StoreAuthorizer{Store: s}
	readSession := base.BindProfiles([]string{"read-only"})
	writeSession := base.BindProfiles([]string{"operator"})

	if !readSession.Authorize("same-user", "", "show version", true) {
		t.Fatal("the first session lost its read profile after a later login")
	}
	if readSession.Authorize("same-user", "", "set system host-name router", false) {
		t.Fatal("the first session gained the later login's write profile")
	}
	if !writeSession.Authorize("same-user", "", "set system host-name router", false) {
		t.Fatal("the second session did not receive its own write profile")
	}
}
