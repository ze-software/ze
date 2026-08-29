package specsession

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
	if err := os.MkdirAll(filepath.Join(root, "plan"), 0o750); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"spec-a.md", "spec-b.md"} {
		if err := os.WriteFile(filepath.Join(root, "plan", name), []byte(readySpecFixture), 0o600); err != nil {
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
	if err := os.MkdirAll(filepath.Join(root, "plan"), 0o750); err != nil {
		t.Fatal(err)
	}
	body := []byte("| Field | Value |\n|-------|-------|\n| Status | in-progress |\n| Updated | 2026-08-27 |\n")
	if err := os.WriteFile(filepath.Join(root, "plan", "spec-a.md"), body, 0o600); err != nil {
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
	// VALIDATES: the grouped native command maps the old default/current,
	// claim, wip, state, model, review-hash, and release interfaces.
	// PREVENTS: final hook routing reaching an API that has no command grammar.
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "plan"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "plan", "spec-a.md"), []byte(readySpecFixture), 0o600); err != nil {
		t.Fatal(err)
	}
	setAnswerRoot(t, root)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "command-session")

	payload, code := Answer(nil)
	if current, ok := payload.(currentReport); code != 0 || !ok || current.Spec != "" {
		t.Fatalf("bare current = (%#v, %d)", payload, code)
	}
	payload, code = Answer([]string{"claim", "spec", "plan/spec-a.md"})
	if claim, ok := payload.(ClaimReport); code != 0 || !ok || !claim.Transitioned {
		t.Fatalf("claim = (%#v, %d)", payload, code)
	}
	payload, code = Answer([]string{"current"})
	if current, ok := payload.(currentReport); code != 0 || !ok || current.Spec != "spec-a.md" {
		t.Fatalf("current = (%#v, %d)", payload, code)
	}
	if payload, code = Answer([]string{"wip"}); code != 0 {
		t.Fatalf("wip = (%#v, %d)", payload, code)
	}
	if payload, code = Answer([]string{"state", "current"}); code != 0 {
		t.Fatalf("state current = (%#v, %d)", payload, code)
	}
	state, ok := payload.(statePathReport)
	if !ok || state.Path == "" {
		t.Fatalf("state current payload = %#v", payload)
	}
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(state.Path)), []byte("state\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if payload, code = Answer([]string{"state", "latest", "spec", "a"}); code != 0 {
		t.Fatalf("state latest = (%#v, %d)", payload, code)
	}
	latest, ok := payload.(statePathReport)
	if !ok || latest.Path != state.Path {
		t.Fatalf("state latest payload = %#v, want %s", payload, state.Path)
	}
	if payload, code = Answer([]string{"model", "current"}); code != 1 {
		t.Fatalf("unreadable model = (%#v, %d), want advisory code 1", payload, code)
	}
	if payload, code = Answer([]string{"review", "hash", "file", "plan/spec-a.md"}); code != 0 {
		t.Fatalf("review hash = (%#v, %d)", payload, code)
	}
	if payload, code = Answer([]string{"release"}); code != 0 {
		t.Fatalf("release = (%#v, %d)", payload, code)
	}
	if _, code = Answer([]string{"claim", "spec-a.md"}); code != 2 {
		t.Fatalf("claim accepted a value without the spec selector, code %d", code)
	}
	t.Setenv("CLAUDE_CODE_SESSION_ID", "../unreadable")
	if payload, code = Answer([]string{"model", "current"}); code != 1 {
		t.Fatalf("model fail-speak = (%#v, %d)", payload, code)
	}
	if payload, code = Answer([]string{"wip"}); code != 0 {
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
