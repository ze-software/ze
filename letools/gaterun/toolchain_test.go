// Related: gaterun.go -- the environment handoff these tests drive
//
// VALIDATES: Stream's doc comment states, "a child inherits nothing this does
// not hand it". This behavior makes letools/gotoolchain the ONE statement of
// what a gate runs under.
// PREVENTS: These tests prevent a gate from using the developer's ambient
// environment. A GOCACHE outside the checkout causes Unix-socket tests to fail
// because paths are too long. An unpinned GOTOOLCHAIN makes golangci-lint print
// "0 issues" and exit non-zero. Both failures can appear to be successful runs.

package gaterun

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// probe runs a shell fragment under environ and answers what it printed.
func probe(t *testing.T, fragment string, environ []string) string {
	t.Helper()
	dir := t.TempDir()
	out := filepath.Join(dir, "seen")

	// The child writes to a file instead of stdout. Stream gives the child THIS
	// process's stdout. Therefore, a test must not read the terminal.
	script := fragment + " > " + out //nolint:gocritic // a fixture script, not a hot path
	if code := Stream([]string{"sh", "-c", script}, dir, environ); code != 0 {
		t.Fatalf("the probe exited %d", code)
	}
	seen, err := os.ReadFile(out) //nolint:gosec // a path this test created
	if err != nil {
		t.Fatalf("read what the probe saw: %v", err)
	}
	return strings.TrimSpace(string(seen))
}

// TestStreamHandsTheChildExactlyTheEnvironmentItWasGiven pins the handoff.
func TestStreamHandsTheChildExactlyTheEnvironmentItWasGiven(t *testing.T) {
	if got := probe(t, "printf %s \"$ZE_PROBE\"", []string{"ZE_PROBE=carried", "PATH=" + os.Getenv("PATH")}); got != "carried" {
		t.Errorf("the child saw ZE_PROBE=%q, want the value it was handed", got)
	}
}

// TestStreamGivesTheChildNothingItWasNotHanded verifies that unprovided
// variables are absent. An inherited value that the caller did not select can
// cause a gate to use an environment that no component derived.
func TestStreamGivesTheChildNothingItWasNotHanded(t *testing.T) {
	t.Setenv("ZE_PROBE_AMBIENT", "leaked")
	if got := probe(t, "printf %s \"$ZE_PROBE_AMBIENT\"", []string{"PATH=" + os.Getenv("PATH")}); got != "" {
		t.Errorf("the child saw ZE_PROBE_AMBIENT=%q, want nothing: it was not handed one", got)
	}
}

// TestALaterEntryWinsOverAnEarlierOne verifies the override rule that every
// caller uses. A caller appends an override to an inherited environment. The
// isolated binary set uses this rule to add ZE_TEST_NO_BUILD to the environment
// that gotoolchain derived.
func TestALaterEntryWinsOverAnEarlierOne(t *testing.T) {
	environ := []string{"PATH=" + os.Getenv("PATH"), "ZE_PROBE=first", "ZE_PROBE=second"}
	if got := probe(t, "printf %s \"$ZE_PROBE\"", environ); got != "second" {
		t.Errorf("the child saw ZE_PROBE=%q, want the last entry to win", got)
	}
}
