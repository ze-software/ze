// Related: setstate.go -- DeriveSets, the set view and the session-start lines
// Related: specset.go -- GroupSets, the sets whose state is derived
//
// The goal is the derived state of a spec set (AC-8, AC-9 of
// spec-agent-economy-4-supervision): a set with a closed or claimed child and
// an open one is live while a live session or running agent holds it, stalled
// when nobody does, an idea when nothing was ever claimed or closed, and
// "children closed" when only its umbrella is left. Every doubt (claims not
// read, a holder the reaper cannot judge, a record not computed) answers
// cannot tell, never stalled or idea. The method: a population of Spec rows
// and a record of closed specs built by hand, or, for the two entry points,
// an isolated repository whose history removes one child.
package specstatus

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
	lesession "github.com/ze-software/ze/internal/le/session"
)

// setSpec answers a spec body whose metadata table declares status and
// depends, the two fields the set state reads.
func setSpec(status, depends string) string {
	return "# Spec: fixture\n\n| Field | Value |\n|-------|-------|\n| Status | " + status +
		" |\n| Priority | P2 |\n| Release reason | Release proof |\n| Depends | " + depends +
		" |\n| Phase | - |\n| Updated | 2026-11-20 |\n\n## Task\n\nFixture prose.\n"
}

// setRow answers one population row as Parse fills it from a fixture name.
func setRow(name, status, depends string) Spec {
	return Spec{Name: strings.TrimSuffix(strings.TrimPrefix(name, "spec-"), ".md"), Path: fixturePath(name), Status: status, Depends: depends}
}

// knownClosed answers a computed record holding stems.
func knownClosed(stems ...string) ClosedSpecs {
	return ClosedSpecs{commit: "0123456789abcdef0123456789abcdef01234567", stems: stems, known: true}
}

// econ is the population most cases share: an umbrella, child 1 closed (so
// absent here), child 2 in progress after it, child 3 ready with no
// dependency.
func econ() []Spec {
	return []Spec{
		setRow("spec-econ-0-umbrella.md", "skeleton", "-"),
		setRow("spec-econ-2-second.md", "in-progress", "spec-econ-1-first.md"),
		setRow("spec-econ-3-third.md", "ready", "-"),
	}
}

// viewOf answers the view of the set named prefix, and fails when the report
// holds none.
func viewOf(t *testing.T, report SetReport, prefix string) SetView {
	t.Helper()
	for index := range report.Sets {
		if report.Sets[index].Prefix == prefix {
			return report.Sets[index]
		}
	}
	t.Fatalf("the report holds no set %q: %+v", prefix, report.Sets)
	return SetView{}
}

// claimOn answers a claim on spec-econ-2-second.md, the fixture's in-progress
// child, by session, with verdict.
func claimOn(session string, verdict lesession.Liveness) lesession.Claim {
	return lesession.Claim{Marker: "tmp/session/" + session + "-spec", Spec: "spec-econ-2-second.md", Session: session, Liveness: verdict, Reason: "fixture verdict " + verdict.String()}
}

// writeTree writes files under root, as the working tree beside the fixture
// history.
func writeTree(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for name, body := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

// econRepository answers a repository whose history removes child 1 of set
// econ, with the working tree and the index at its tip and a tracked
// handover named for the set, and the tip commit.
func econRepository(t *testing.T) (root, tip string) {
	t.Helper()
	root = closedFixtureRepository(t)
	files := map[string]string{
		closedFixes + "spec-econ-0-umbrella.md": setSpec("skeleton", "-"),
		closedFixes + "spec-econ-1-first.md":    setSpec("in-progress", "-"),
		closedFixes + "spec-econ-2-second.md":   setSpec("ready", "spec-econ-1-first.md"),
		closedFixes + "spec-econ-3-third.md":    setSpec("ready", "spec-econ-2-second.md"),
		"plan/handover/2026-11-14-econ.md":      "where the econ set stands\n",
	}
	closedFixtureCommit(t, root, files)
	delete(files, closedFixes+"spec-econ-1-first.md")
	tip = closedFixtureCommit(t, root, files)
	writeTree(t, root, files)
	closedFixtureGit(t, root, "", "read-tree", "main")
	return root, tip
}

// noClaims is a claim reader that finds no claim marker.
func noClaims(string) ([]lesession.Claim, error) { return nil, nil }

// TestStatusReportsAStartedSetNobodyHoldsAsStalled drives the set view's
// entry point over a repository whose history closed child 1.
//
// VALIDATES: AC-8 and AC-9: the closed child comes from the record the view
// computes from git history, the set is stalled because no claim holds it,
// and the view names the next child, the command that claims it and the
// tracked handover.
// PREVENTS: a started set nobody holds reading as an idea, or the view
// printing no way to pick it up.
func TestStatusReportsAStartedSetNobodyHoldsAsStalled(t *testing.T) {
	root, tip := econRepository(t)
	report, err := setList(t.Context(), root, noClaims, func(string) {})
	if err != nil {
		t.Fatal(err)
	}
	if report.ClosedCommit != tip {
		t.Fatalf("the record was computed at %q, want HEAD %q (unknown: %q)", report.ClosedCommit, tip, report.ClosedUnknown)
	}
	view := viewOf(t, report, "econ")
	if view.State != SetStalled {
		t.Fatalf("state %s (%s), want stalled", view.State, view.Why)
	}
	if !reflect.DeepEqual(view.Closed, []string{"econ-1-first"}) {
		t.Fatalf("closed children %v, want [econ-1-first]", view.Closed)
	}
	if len(view.Open) != 2 || view.Open[0].Number != 2 || view.Open[1].Number != 3 {
		t.Fatalf("open children %+v, want 2 and 3", view.Open)
	}
	if view.Next != "spec-econ-2-second.md" {
		t.Fatalf("next %q (waits on %q, %q), want spec-econ-2-second.md: its dependency closed", view.Next, view.WaitsOn, view.NoNext)
	}
	if !reflect.DeepEqual(view.Handovers, []string{"plan/handover/2026-11-14-econ.md"}) {
		t.Fatalf("handovers %v, want the tracked econ handover", view.Handovers)
	}
	text := report.Text()
	for _, want := range []string{"econ: stalled", "econ-1-first", "./le spec claim spec-econ-2-second.md", "plan/handover/2026-11-14-econ.md"} {
		if !strings.Contains(text, want) {
			t.Fatalf("the view lacks %q:\n%s", want, text)
		}
	}
}

// TestALiveClaimMakesAStartedSetLiveWhateverItsUmbrellaDeclares holds child 2
// under a live session while the umbrella still declares skeleton.
//
// VALIDATES: AC-8: the set is live, lists its closed and open children, and
// names as next the lowest open child no live claim runs whose dependencies
// are met.
// PREVENTS: the umbrella's own Status deciding the set's state.
func TestALiveClaimMakesAStartedSetLiveWhateverItsUmbrellaDeclares(t *testing.T) {
	claims := ClaimsRead([]lesession.Claim{claimOn("s1", lesession.LivenessLive)})
	view := viewOf(t, DeriveSets(econ(), knownClosed("econ-1-first"), claims, nil), "econ")
	if view.State != SetLive {
		t.Fatalf("state %s (%s), want live", view.State, view.Why)
	}
	if !reflect.DeepEqual(view.Closed, []string{"econ-1-first"}) {
		t.Fatalf("closed %v", view.Closed)
	}
	if view.Next != "spec-econ-3-third.md" {
		t.Fatalf("next %q, want spec-econ-3-third.md: child 2 is being run", view.Next)
	}
	if len(view.Holders) != 1 || view.Holders[0].Spec != "spec-econ-2-second.md" || view.Holders[0].Liveness != "live" {
		t.Fatalf("holders %+v, want the live claim on child 2", view.Holders)
	}
}

// TestADeadHolderLeavesTheSetStalled holds child 2 under a session the reaper
// proved dead.
//
// VALIDATES: AC-9: a claim whose holder is dead holds nothing, so the set is
// stalled, and the holder is still listed.
// PREVENTS: a leaked marker of a dead session keeping a set reported live.
func TestADeadHolderLeavesTheSetStalled(t *testing.T) {
	claims := ClaimsRead([]lesession.Claim{claimOn("gone", lesession.LivenessDead)})
	view := viewOf(t, DeriveSets(econ(), knownClosed("econ-1-first"), claims, nil), "econ")
	if view.State != SetStalled {
		t.Fatalf("state %s (%s), want stalled", view.State, view.Why)
	}
	if view.Next != "spec-econ-2-second.md" {
		t.Fatalf("next %q, want the half-done child 2", view.Next)
	}
	if len(view.Holders) != 1 || view.Holders[0].Liveness != "dead" {
		t.Fatalf("holders %+v, want the dead claim listed", view.Holders)
	}
}

// TestASetNeverClaimedOrClosedStaysAnIdea is AC-8's survivor.
//
// VALIDATES: a set with no closed child, no claim and no child past ready is
// an idea.
// PREVENTS: every numbered family of skeletons being reported as stalled.
func TestASetNeverClaimedOrClosedStaysAnIdea(t *testing.T) {
	specs := []Spec{
		setRow("spec-idea-0-umbrella.md", "skeleton", "-"),
		setRow("spec-idea-1-first.md", "ready", "-"),
		setRow("spec-idea-2-second.md", "skeleton", "-"),
	}
	view := viewOf(t, DeriveSets(specs, knownClosed(), ClaimsRead(nil), nil), "idea")
	if view.State != SetIdea {
		t.Fatalf("state %s (%s), want idea", view.State, view.Why)
	}
	if view.Started {
		t.Fatal("a set nothing claimed or closed reads as started")
	}
}

// TestASetWhoseOpenChildrenAllWaitOutsideNamesThatSpec is AC-9's survivor.
//
// VALIDATES: when every open child waits on an open spec outside the set, the
// view and the session-start line name that spec and no claim command.
// PREVENTS: a session being told to claim a child that cannot start.
func TestASetWhoseOpenChildrenAllWaitOutsideNamesThatSpec(t *testing.T) {
	specs := []Spec{
		setRow("spec-wait-0-umbrella.md", "skeleton", "-"),
		setRow("spec-wait-2-second.md", "ready", "`spec-outside-work.md` first"),
		setRow("spec-wait-3-third.md", "ready", "spec-wait-2-second.md"),
		setRow("spec-outside-work.md", "in-progress", "-"),
	}
	report := DeriveSets(specs, knownClosed("wait-1-first"), ClaimsRead(nil), nil)
	view := viewOf(t, report, "wait")
	if view.State != SetStalled {
		t.Fatalf("state %s (%s), want stalled", view.State, view.Why)
	}
	if view.Next != "" {
		t.Fatalf("next %q, want none: every open child waits", view.Next)
	}
	if view.WaitsOn != fixturePath("spec-outside-work.md") {
		t.Fatalf("waits on %q, want the outside spec", view.WaitsOn)
	}
	lines := strings.Join(report.sessionStartLines(), "\n")
	if !strings.Contains(lines, fixturePath("spec-outside-work.md")) {
		t.Fatalf("the session-start lines do not name the outside spec:\n%s", lines)
	}
	if strings.Contains(lines, "spec claim") {
		t.Fatalf("the session-start lines offer a claim of a child that cannot start:\n%s", lines)
	}
}

// TestADependencyOnTheSetsOwnUmbrellaIsMet names the open umbrella in a
// child's Depends.
//
// VALIDATES: the umbrella waits on its children, never the reverse, so a
// child that names it can still run next.
// PREVENTS: every set whose children cite their umbrella reading as blocked.
func TestADependencyOnTheSetsOwnUmbrellaIsMet(t *testing.T) {
	specs := []Spec{
		setRow("spec-own-0-umbrella.md", "design", "-"),
		setRow("spec-own-1-first.md", "in-progress", "spec-own-0-umbrella"),
		setRow("spec-own-2-second.md", "ready", "spec-own-0-umbrella"),
	}
	view := viewOf(t, DeriveSets(specs, knownClosed(), ClaimsRead(nil), nil), "own")
	if view.Next != "spec-own-1-first.md" {
		t.Fatalf("next %q (waits on %q, %q), want child 1", view.Next, view.WaitsOn, view.NoNext)
	}
}

// TestADoubtIsNeverCalledStalled covers every input the derivation cannot
// judge.
//
// VALIDATES: a holder the reaper cannot judge, claims not read, and a claim
// marker that could not be read each answer cannot tell with the reason.
// PREVENTS: a set an agent still runs being offered to another session.
func TestADoubtIsNeverCalledStalled(t *testing.T) {
	cases := map[string]SetClaims{
		"holder cannot be told": ClaimsRead([]lesession.Claim{claimOn("s1", lesession.LivenessCannotTell)}),
		"claims not read":       ClaimsUnread("the process table could not be read", "fixture fix"),
		"the zero claims":       {},
		"unreadable marker":     ClaimsRead([]lesession.Claim{{Marker: "tmp/session/x-spec", Unread: "permission denied", Session: "x"}}),
	}
	fixes := map[string]string{
		"holder cannot be told": "wait for the holder",
		"claims not read":       "fixture fix",
		"the zero claims":       "`./le spec status set list`, which reads every claim",
		"unreadable marker":     "`./le session clean`",
	}
	for name, claims := range cases {
		t.Run(name, func(t *testing.T) {
			view := viewOf(t, DeriveSets(econ(), knownClosed("econ-1-first"), claims, nil), "econ")
			if view.State != SetCannotTell {
				t.Fatalf("state %s, want cannot tell", view.State)
			}
			if view.Why == "" {
				t.Fatal("cannot tell carries no reason")
			}
			if !strings.Contains(view.Fix, fixes[name]) {
				t.Fatalf("fix %q, want the fix for this doubt, holding %q", view.Fix, fixes[name])
			}
		})
	}
}

// TestTheSessionStartLinesNameUnreadClaimsOnceWithTheCommandThatReadsThem
// passes the claims as not read, as the session-start hook does when its claim
// read misses its deadline.
//
// VALIDATES: AC-9 within the session-start budget: every started set with an
// open child is named on one line with the reason and the command that reads
// the claims, and no set is called stalled or offered for a claim.
// PREVENTS: claims skipped for time being read as nobody holding the set.
func TestTheSessionStartLinesNameUnreadClaimsOnceWithTheCommandThatReadsThem(t *testing.T) {
	specs := append(econ(),
		setRow("spec-more-0-umbrella.md", "skeleton", "-"),
		setRow("spec-more-1-first.md", "in-progress", "-"),
	)
	claims := ClaimsUnread("the claims are not read at session start", "")
	lines := DeriveSets(specs, knownClosed("econ-1-first"), claims, nil).sessionStartLines()
	text := strings.Join(lines, "\n")
	matching := 0
	for _, line := range lines {
		if strings.Contains(line, "econ, more") {
			matching++
		}
	}
	if matching != 1 {
		t.Fatalf("want one line naming econ and more, found %d:\n%s", matching, text)
	}
	for _, want := range []string{"not judged live or stalled here", "the claims are not read at session start", "`./le spec status set list`"} {
		if !strings.Contains(text, want) {
			t.Fatalf("the lines lack %q:\n%s", want, text)
		}
	}
	for _, refused := range []string{"Stalled", "spec claim"} {
		if strings.Contains(text, refused) {
			t.Fatalf("claims not read still produced %q:\n%s", refused, text)
		}
	}
}

// TestTheSetActionListsTheSetsThroughItsEntryPoint types `spec status set
// list` through Answer over a fixture checkout, with a claims reader holding a
// live claim on child 2.
//
// VALIDATES: AC-8 through the real entry point: the answer is the set report,
// code 0, with the closed child computed from history and the set live
// because the reader Answer uses found the live claim.
// PREVENTS: the action reaching a different reader or a different root than
// the one the line names.
func TestTheSetActionListsTheSetsThroughItsEntryPoint(t *testing.T) {
	root, tip := econRepository(t)
	t.Setenv("ZE_REPO_ROOT", root)
	env.ResetCache()
	t.Cleanup(env.ResetCache)
	saved := listSetClaims
	t.Cleanup(func() { listSetClaims = saved })
	listSetClaims = func(checkout string) ([]lesession.Claim, error) {
		if checkout != root {
			t.Errorf("claims read under %q, want the checkout %q", checkout, root)
		}
		return []lesession.Claim{claimOn("s1", lesession.LivenessLive)}, nil
	}
	answer, code := Answer([]string{verbSet, keywordList})
	if code != 0 {
		t.Fatalf("`spec status set list` answered %d, want 0", code)
	}
	report, ok := answer.(SetReport)
	if !ok {
		t.Fatalf("the answer is %T, want SetReport", answer)
	}
	if report.ClosedCommit != tip {
		t.Fatalf("the record was computed at %q, want HEAD %q (unknown: %q)", report.ClosedCommit, tip, report.ClosedUnknown)
	}
	view := viewOf(t, report, "econ")
	if view.State != SetLive {
		t.Fatalf("state %s (%s), want live from the injected claim", view.State, view.Why)
	}
	if !reflect.DeepEqual(view.Closed, []string{"econ-1-first"}) {
		t.Fatalf("closed children %v, want [econ-1-first]", view.Closed)
	}
}

// TestARecordNotComputedNeverCallsAQuietSetAnIdea leaves the record of
// closed specs unread.
//
// VALIDATES: a set with no started child and no claim cannot be told from an
// idea while the record is unknown, and a set started by its files is still
// judged stalled.
// PREVENTS: the zero record reading as "no child closed".
func TestARecordNotComputedNeverCallsAQuietSetAnIdea(t *testing.T) {
	quiet := []Spec{
		setRow("spec-quiet-0-umbrella.md", "skeleton", "-"),
		setRow("spec-quiet-2-second.md", "ready", "-"),
	}
	view := viewOf(t, DeriveSets(quiet, ClosedSpecs{}, ClaimsRead(nil), nil), "quiet")
	if view.State != SetCannotTell {
		t.Fatalf("state %s, want cannot tell", view.State)
	}
	if !strings.Contains(view.Why, notComputed) {
		t.Fatalf("why %q does not say the record is not computed", view.Why)
	}
	started := viewOf(t, DeriveSets(econ(), ClosedSpecs{}, ClaimsRead(nil), nil), "econ")
	if started.State != SetStalled {
		t.Fatalf("a set its files show started reads %s, want stalled", started.State)
	}
}

// TestAnUmbrellaWhoseChildrenAllClosedReportsChildrenClosed leaves only the
// umbrella file.
//
// VALIDATES: the state is children closed, and the session-start line says
// the umbrella owes its own criteria and its closure.
// PREVENTS: a finished set reading as an idea because no child file is left.
func TestAnUmbrellaWhoseChildrenAllClosedReportsChildrenClosed(t *testing.T) {
	specs := []Spec{setRow("spec-done-0-umbrella.md", "skeleton", "-")}
	report := DeriveSets(specs, knownClosed("done-1-first", "done-2-second"), ClaimsRead(nil), nil)
	view := viewOf(t, report, "done")
	if view.State != SetChildrenClosed {
		t.Fatalf("state %s (%s), want children closed", view.State, view.Why)
	}
	lines := strings.Join(report.sessionStartLines(), "\n")
	if !strings.Contains(lines, fixturePath("spec-done-0-umbrella.md")) {
		t.Fatalf("the session-start lines do not name the umbrella:\n%s", lines)
	}
}

// TestTheSessionStartSummaryReadsTheRecordAndRunsNoGit plants a record at a
// commit the fixture history does not hold.
//
// VALIDATES: AC-9: the session-start lines report the stalled set from the
// planted record, name the commit it holds, and leave the record as it was.
// LoadClosedSpecs would find that commit no ancestor of HEAD and rewrite the
// record from the history, which removed nothing it names. A git first on
// PATH that fails and leaves a mark makes any git process the call starts
// fail the test.
// PREVENTS: the session-start path walking git history.
func TestTheSessionStartSummaryReadsTheRecordAndRunsNoGit(t *testing.T) {
	root, _ := econRepository(t)
	planted := "fedcba9876543210fedcba9876543210fedcba98"
	plantClosedRecord(t, root, planted, "econ-1-first", "econ-9-planted")
	poison := t.TempDir()
	called := filepath.Join(poison, "git-was-called")
	script := "#!/bin/sh\necho \"$@\" >> '" + called + "'\nexit 97\n"
	if err := os.WriteFile(filepath.Join(poison, "git"), []byte(script), 0o700); err != nil { //nolint:gosec // an executable fixture in the test's own temp dir
		t.Fatal(err)
	}
	t.Setenv("PATH", poison+string(os.PathListSeparator)+os.Getenv("PATH"))
	lines, err := SessionStartSets(root, ClaimsRead(nil))
	if ran, readErr := os.ReadFile(called); readErr == nil { //nolint:gosec // the mark the fixture git writes
		t.Fatalf("the session-start path ran git: %s", ran)
	}
	if err != nil {
		t.Fatal(err)
	}
	text := strings.Join(lines, "\n")
	for _, want := range []string{planted[:12], "econ", "./le spec claim spec-econ-2-second.md", "plan/handover/2026-11-14-econ.md"} {
		if !strings.Contains(text, want) {
			t.Fatalf("the session-start lines lack %q:\n%s", want, text)
		}
	}
	if after := ReadClosedSpecs(root); after.Commit() != planted {
		t.Fatalf("the record now holds commit %q: the session-start path rewrote it", after.Commit())
	}
}

// TestTheSessionStartSummarySaysNotComputedBesideTheCommand runs with no
// record.
//
// VALIDATES: the line says "not computed" and names the command that
// computes the record.
// PREVENTS: an absent record printing as no closed children.
func TestTheSessionStartSummarySaysNotComputedBesideTheCommand(t *testing.T) {
	root, _ := econRepository(t)
	lines, err := SessionStartSets(root, ClaimsRead(nil))
	if err != nil {
		t.Fatal(err)
	}
	text := strings.Join(lines, "\n")
	if !strings.Contains(text, notComputed) {
		t.Fatalf("the lines do not say not computed:\n%s", text)
	}
	if !strings.Contains(text, "`./le spec status set list`") {
		t.Fatalf("the lines do not name the command that computes the record:\n%s", text)
	}
}

// TestTheSetActionRefusesALineWithoutList types `spec status set` bare.
//
// VALIDATES: the action enumerates sets only under its `list` keyword, and
// refuses the bare verb with the usage code.
// PREVENTS: a second spelling of the listing.
func TestTheSetActionRefusesALineWithoutList(t *testing.T) {
	if _, code := Answer([]string{verbSet}); code != 2 {
		t.Fatalf("`spec status set` answered %d, want the refusal 2", code)
	}
}
