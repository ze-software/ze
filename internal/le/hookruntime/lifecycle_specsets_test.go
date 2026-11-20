// Related: lifecycle.go -- reportSpecSets and sessionStartClaims, the session-start step under test
package hookruntime

import (
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/le/derived"
	"github.com/ze-software/ze/internal/le/session"
)

// specSetsHookFixture is a committed checkout holding a started spec set (one
// child in progress, one ready) and its handover, with no record of closed
// specs.
func specSetsHookFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	// Seed every derived artifact so the hook renders nothing and the test
	// stays about the spec sets lines.
	for _, artifact := range derived.All() {
		writeHookFixture(t, root, artifact.Path, "# derived\n")
	}
	spec := func(status string) string {
		return "# Spec: fixture\n\n| Field | Value |\n|---|---|\n| Status | " + status + " |\n"
	}
	bucket := "plan/post-release/engineering/"
	writeHookFixture(t, root, bucket+"spec-econ-0-umbrella.md", spec("skeleton"))
	writeHookFixture(t, root, bucket+"spec-econ-2-second.md", spec("in-progress"))
	writeHookFixture(t, root, bucket+"spec-econ-3-third.md", spec("ready"))
	writeHookFixture(t, root, "plan/handover/2026-11-14-econ.md", "where the econ set stands\n")
	commitHookFixture(t, root)
	return root
}

// useSessionStartClaims makes the spec sets step read its claims from list
// under budget, for the rest of the test.
func useSessionStartClaims(t *testing.T, list func(string) ([]session.Claim, error), budget time.Duration) {
	t.Helper()
	savedList, savedBudget := sessionStartListClaims, sessionStartClaimsBudget
	t.Cleanup(func() { sessionStartListClaims, sessionStartClaimsBudget = savedList, savedBudget })
	sessionStartListClaims, sessionStartClaimsBudget = list, budget
}

// TestSessionStartReportsAStalledSetWhenTheClaimsAnswerInTime drives the
// session-start hook with claims that answer at once and hold nothing.
//
// VALIDATES: AC-9 at session start: the started set nobody holds prints as
// stalled, with its next child, the command that claims it and its handover,
// beside the record that is not computed.
// PREVENTS: the claims passed as unread whatever their cost, so session start
// could never report a stalled set.
func TestSessionStartReportsAStalledSetWhenTheClaimsAnswerInTime(t *testing.T) {
	useSessionStartClaims(t, func(string) ([]session.Claim, error) { return nil, nil }, time.Second)
	root := specSetsHookFixture(t)
	printed, noted := runSessionStartStreams(t, root)
	for _, want := range []string{
		"Spec sets: the record of closed specs is not computed",
		"Stalled spec set econ: no live session or running agent holds it.",
		"Next: spec-econ-2-second.md, claim it with `./le spec claim spec-econ-2-second.md`",
		"; handover plan/handover/2026-11-14-econ.md",
	} {
		if !strings.Contains(printed, want) {
			t.Fatalf("the hook output lacks %q:\nstdout:\n%s\nstderr:\n%s", want, printed, noted)
		}
	}
	if strings.Contains(printed, "not judged live or stalled here") {
		t.Fatalf("the hook called claims it read unread:\n%s", printed)
	}
}

// TestSessionStartNamesAStartedSetWhenTheClaimsMissTheirDeadline drives the
// session-start hook with a claim read that does not answer within its
// shortened deadline.
//
// VALIDATES: AC-9 inside the session-start budget: past the claims deadline
// the hook names the started set on the line that says its claims were not
// read within the session-start budget, beside `./le spec status set list`,
// which reads them, and never calls the set stalled; the hook still finishes
// its own budget.
// PREVENTS: a slow claim scan taking the session-start message with it, and a
// set nobody judged offered to the session as stalled.
func TestSessionStartNamesAStartedSetWhenTheClaimsMissTheirDeadline(t *testing.T) {
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	slow := func(string) ([]session.Claim, error) {
		<-release
		return nil, nil
	}
	useSessionStartClaims(t, slow, 50*time.Millisecond)
	root := specSetsHookFixture(t)
	started := time.Now()
	printed, noted := runSessionStartStreams(t, root)
	if elapsed := time.Since(started); elapsed >= sessionStartBudget {
		t.Fatalf("the hook took %s, past its %s budget: the slow claim read held it", elapsed, sessionStartBudget)
	}
	for _, want := range []string{
		"Spec sets: the record of closed specs is not computed",
		"Spec sets started with an open child, not judged live or stalled here: econ;",
		"whether anyone holds it cannot be told: claims not read within the session-start budget;",
		"`./le spec status set list`",
	} {
		if !strings.Contains(printed, want) {
			t.Fatalf("the hook output lacks %q:\nstdout:\n%s\nstderr:\n%s", want, printed, noted)
		}
	}
	if strings.Contains(printed, "Stalled spec set") {
		t.Fatalf("the hook called a set stalled without reading a claim:\n%s", printed)
	}
}
