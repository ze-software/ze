package spec

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/env"
)

const readySpecFixture = `# Spec

| Field | Value |
|-------|-------|
| Status | ready |
| Updated | 2026-08-01 |

## Goal
`

func TestConcurrentClaimsCannotExceedTheWIPCap(t *testing.T) {
	// VALIDATES: the WIP count, ready transition, and marker publication share
	// one repository lock.
	// PREVENTS: two sessions both observing one free slot and opening two specs.
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "plan", "post-release", "engineering"), 0o750); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"spec-a.md", "spec-b.md"} {
		if err := os.WriteFile(filepath.Join(root, "plan", "post-release", "engineering", name), []byte(readySpecFixture), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	owners := []specOwner{
		{Root: root, SessionID: "session-a", WIPCap: 1, Now: func() time.Time { return time.Date(2026, 8, 27, 0, 0, 0, 0, time.UTC) }},
		{Root: root, SessionID: "session-b", WIPCap: 1, Now: func() time.Time { return time.Date(2026, 8, 27, 0, 0, 0, 0, time.UTC) }},
	}
	results := make([]ClaimReport, 2)
	errs := make([]error, 2)
	var wait sync.WaitGroup
	for index := range owners {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			results[index], errs[index] = owners[index].Claim([]string{"spec-a.md", "spec-b.md"}[index])
		}(index)
	}
	wait.Wait()
	refused := 0
	transitioned := 0
	for index := range results {
		if errs[index] != nil {
			t.Fatalf("claim %d: %v", index, errs[index])
		}
		if results[index].Refused {
			refused++
		}
		if results[index].Transitioned {
			transitioned++
		}
	}
	if refused != 1 || transitioned != 1 {
		t.Fatalf("results = %#v, want one transition and one refusal", results)
	}
	wip, err := wip(root, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(wip.Specs) != 1 {
		t.Fatalf("wip = %#v, want exactly one spec", wip.Specs)
	}
}

func TestConcurrentSessionsKeepIndependentMarkers(t *testing.T) {
	// VALIDATES: two sessions may own the same in-progress spec without
	// overwriting each other's claim.
	// PREVENTS: a shared marker turning the second claim into ownership theft.
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "plan", "post-release", "engineering"), 0o750); err != nil {
		t.Fatal(err)
	}
	body := []byte("| Field | Value |\n|-------|-------|\n| Status | in-progress |\n| Updated | 2026-08-27 |\n")
	if err := os.WriteFile(filepath.Join(root, "plan", "post-release", "engineering", "spec-a.md"), body, 0o600); err != nil {
		t.Fatal(err)
	}
	owners := []specOwner{{Root: root, SessionID: "one"}, {Root: root, SessionID: "two"}}
	var wait sync.WaitGroup
	for index := range owners {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			if _, err := owners[index].Claim("spec-a.md"); err != nil {
				t.Errorf("claim: %v", err)
			}
		}(index)
	}
	wait.Wait()
	for _, owner := range owners {
		got, err := owner.currentSpec()
		if err != nil || got != "spec-a.md" {
			t.Errorf("currentSpec(%s) = %q, %v", owner.SessionID, got, err)
		}
	}
}

func TestMalformedClaimMarkerFailsClosed(t *testing.T) {
	// VALIDATES: a marker containing a path rather than a spec basename is
	// reported as malformed.
	// PREVENTS: state-path construction escaping the per-session state directory.
	root := t.TempDir()
	marker := filepath.Join(root, "tmp", "session", ".session-bad")
	if err := os.MkdirAll(filepath.Dir(marker), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(marker, []byte("../../spec-other.md\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := (specOwner{Root: root, SessionID: "bad"}).currentSpec(); err == nil {
		t.Fatal("currentSpec accepted a path-bearing marker")
	}
}

func TestSpecSessionCommandMapsEveryOwnershipAction(t *testing.T) {
	// VALIDATES: every member of the spec namespace (current, claim, wip,
	// state, model, review hash, release) answers through its own handler.
	// PREVENTS: final hook routing reaching an API that has no command grammar.
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "plan", "post-release", "engineering"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "plan", "post-release", "engineering", "spec-a.md"), []byte(readySpecFixture), 0o600); err != nil {
		t.Fatal(err)
	}
	setAnswerRoot(t, root)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "command-session")

	payload, code := Current(nil)
	if current, ok := payload.(currentReport); code != 0 || !ok || current.Spec != "" {
		t.Fatalf("current before claim = (%#v, %d)", payload, code)
	}
	payload, code = Claim([]string{"plan/post-release/engineering/spec-a.md"})
	if claim, ok := payload.(ClaimReport); code != 0 || !ok || !claim.Transitioned {
		t.Fatalf("claim = (%#v, %d)", payload, code)
	}
	payload, code = Current(nil)
	if current, ok := payload.(currentReport); code != 0 || !ok || current.Spec != "spec-a.md" {
		t.Fatalf("current = (%#v, %d)", payload, code)
	}
	if payload, code = WIP(nil); code != 0 {
		t.Fatalf("wip = (%#v, %d)", payload, code)
	}
	if payload, code = State([]string{"current"}); code != 0 {
		t.Fatalf("state current = (%#v, %d)", payload, code)
	}
	state, ok := payload.(statePathReport)
	if !ok || state.Path == "" {
		t.Fatalf("state current payload = %#v", payload)
	}
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(state.Path)), []byte("state\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if payload, code = State([]string{"latest", "spec", "a"}); code != 0 {
		t.Fatalf("state latest = (%#v, %d)", payload, code)
	}
	latest, ok := payload.(statePathReport)
	if !ok || latest.Path != state.Path {
		t.Fatalf("state latest payload = %#v, want %s", payload, state.Path)
	}
	if payload, code = Model([]string{"current"}); code != 1 {
		t.Fatalf("unreadable model = (%#v, %d), want advisory code 1", payload, code)
	}
	if payload, code = Review([]string{"hash", "file", "plan/post-release/engineering/spec-a.md"}); code != 0 {
		t.Fatalf("review hash = (%#v, %d)", payload, code)
	}
	if payload, code = Release(nil); code != 0 {
		t.Fatalf("release = (%#v, %d)", payload, code)
	}
	// `spec` before the file is the retired verb (D-P4-areaword): the
	// verbless table reads it as the spec and refuses the second value.
	if _, code = Claim([]string{"spec", "spec-a.md"}); code != 2 {
		t.Fatalf("claim accepted the retired spec selector before a value, code %d", code)
	}
	t.Setenv("CLAUDE_CODE_SESSION_ID", "../unreadable")
	if payload, code = Model([]string{"current"}); code != 1 {
		t.Fatalf("model fail-speak = (%#v, %d)", payload, code)
	}
	if payload, code = WIP(nil); code != 0 {
		t.Fatalf("wip depended on session ownership = (%#v, %d)", payload, code)
	}
	if spec, err := (specOwner{Root: root, SessionID: "command-session"}).currentSpec(); err != nil || spec != "" {
		t.Fatalf("released currentSpec = %q, %v", spec, err)
	}
}

func TestStatePathRejectsAnUnsafeOwner(t *testing.T) {
	// VALIDATES: public lifecycle APIs reject a session ID that could escape the
	// marker or state directory.
	// PREVENTS: a malformed state owner writing outside tmp/session.
	root := t.TempDir()
	owner := specOwner{Root: root, SessionID: "../other"}
	if _, err := owner.currentSpec(); err == nil {
		t.Fatal("currentSpec accepted an unsafe session ID")
	}
	if _, err := owner.StateFile(); err == nil {
		t.Fatal("StateFile accepted an unsafe session ID")
	}
}

func setAnswerRoot(t *testing.T, root string) {
	t.Helper()
	old, present := os.LookupEnv("ZE_REPO_ROOT")
	if err := os.Setenv("ZE_REPO_ROOT", root); err != nil {
		t.Fatal(err)
	}
	env.ResetCache()
	t.Cleanup(func() {
		var err error
		if present {
			err = os.Setenv("ZE_REPO_ROOT", old)
		} else {
			err = os.Unsetenv("ZE_REPO_ROOT")
		}
		if err != nil {
			t.Errorf("restore ZE_REPO_ROOT: %v", err)
		}
		env.ResetCache()
	})
}

// VALIDATES: each agent of one session owns its claim, an agent with no claim
// of its own answers its parent's, and an agent's release removes only its own
// claim.
// PREVENTS: the shared marker, where the last sibling to claim won and one
// sibling's release cleared every other one's claim
// (concurrent-session-corruption journal, 2026-08-14 and 2026-09-05).
func TestAgentClaimsAreTheirOwnAndFallBackToTheParent(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "plan", "post-release", "engineering"), 0o750); err != nil {
		t.Fatal(err)
	}
	body := []byte("| Field | Value |\n|-------|-------|\n| Status | in-progress |\n| Updated | 2026-08-27 |\n")
	for _, name := range []string{"spec-parent.md", "spec-one.md", "spec-two.md"} {
		if err := os.WriteFile(filepath.Join(root, "plan", "post-release", "engineering", name), body, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	parent := specOwner{Root: root, SessionID: "sid"}
	one := specOwner{Root: root, SessionID: "sid", AgentID: "one"}
	two := specOwner{Root: root, SessionID: "sid", AgentID: "two"}
	for owner, spec := range map[*specOwner]string{&parent: "spec-parent.md", &one: "spec-one.md"} {
		if _, err := owner.Claim(spec); err != nil {
			t.Fatalf("claim %s: %v", spec, err)
		}
	}
	current := func(owner specOwner, want string) {
		t.Helper()
		got, err := owner.currentSpec()
		if err != nil || got != want {
			t.Errorf("currentSpec(agent %q) = %q, %v; want %q", owner.AgentID, got, err, want)
		}
	}
	current(parent, "spec-parent.md")
	current(one, "spec-one.md")
	current(two, "spec-parent.md")
	if _, err := two.Claim("spec-two.md"); err != nil {
		t.Fatal(err)
	}
	current(one, "spec-one.md")
	current(two, "spec-two.md")
	current(parent, "spec-parent.md")
	if err := one.Release(); err != nil {
		t.Fatal(err)
	}
	current(one, "spec-parent.md")
	current(two, "spec-two.md")
	current(parent, "spec-parent.md")
}

// TestClaimFindsTheMovedIdentity preserves the stable marker when the source
// path disappears and the same spec is carried in a current category.
func TestClaimFindsTheMovedIdentity(t *testing.T) {
	root := t.TempDir()
	marker := filepath.Join(root, "tmp", "session", ".session-moved")
	if err := os.MkdirAll(filepath.Dir(marker), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(marker, []byte("spec-moved.md\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "plan", "release", "fixes", "spec-moved.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(readySpecFixture), 0o600); err != nil {
		t.Fatal(err)
	}
	owner := specOwner{Root: root, SessionID: "moved", WIPCap: 12}
	report, err := owner.Claim("spec-moved.md")
	if err != nil || !report.Transitioned {
		t.Fatalf("claim moved identity: %+v, %v", report, err)
	}
	contents, err := os.ReadFile(marker)
	if err != nil || string(contents) != "spec-moved.md\n" {
		t.Fatalf("stable marker = %q, %v", contents, err)
	}
}
