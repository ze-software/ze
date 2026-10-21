// VALIDATES: the first-red keyword reaches the engine from both entry points,
// is separate work in the job registry, is refused with a cut, and leaves a
// trace (stage times, the stop, the unreached stages) every reader can find.
// PREVENTS: a shortened run reading as a pass, attaching to a whole run, or
// leaving no time behind for the next run's order.
package verify

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/job"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	verifyengine "github.com/ze-software/ze/internal/le/verify/engine"
)

// historyLines answers the lines of the stage-time history under root.
func historyLines(t *testing.T, root string) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(verifyengine.StageTimesPath)))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
}

// TestFirstRedReachesTheEngineFromBothEntryPoints walks the keyword from each
// typed line to the engine. `le verify current first-red` hands the worktree
// run FirstRed, `le verify current fast first-red` hands the in-place run
// firstRed, `le verify worktree first-red` parses into Options, and both runs
// then stop at the first red with the stop named on their own surface.
func TestFirstRedReachesTheEngineFromBothEntryPoints(t *testing.T) {
	var worktreeOptions Options
	var inPlaceFirstRed bool
	runs := currentRuns{
		worktree: func(given Options) (any, int) { worktreeOptions = given; return nil, 0 },
		inPlace:  func(_ string, firstRed bool) (any, int) { inPlaceFirstRed = firstRed; return nil, 0 },
	}
	captureStderr(t, func() {
		routeCurrent(leaction.Arguments{keywordFirstRed: {""}}, runs)
		routeCurrent(leaction.Arguments{keywordFast: {""}, keywordFirstRed: {""}}, runs)
	})
	if !worktreeOptions.FirstRed {
		t.Fatalf("le verify current first-red handed the worktree run %#v", worktreeOptions)
	}
	if !inPlaceFirstRed {
		t.Fatal("le verify current fast first-red did not hand the in-place run first-red")
	}
	options, err := optionsFrom(leaction.Arguments{keywordFirstRed: {""}}, func(string) string { return "" })
	if err != nil || !options.FirstRed {
		t.Fatalf("le verify worktree first-red parsed to %#v (%v)", options, err)
	}

	repo := newFixtureRepo(t)
	worktree := Run(context.Background(), repo.root, options, failingRunner())
	if worktree.Verify == nil || worktree.Verify.StoppedAt == "" {
		t.Fatalf("the worktree run did not stop: %#v", worktree.Verify)
	}
	if len(worktree.Verify.Stages) != 1 {
		t.Fatalf("the worktree run started %d stages after a red first stage, want 1", len(worktree.Verify.Stages))
	}
	stopLine := "verify-worktree: full stopped at " + worktree.Verify.StoppedAt
	if !strings.Contains(worktree.Text(), stopLine) ||
		!strings.Contains(worktree.Text(), "runs them all") {
		t.Fatalf("the lifecycle does not print the stop and the command that runs every stage:\n%s", worktree.Text())
	}

	inPlace := runFast(context.Background(), repo.root, true, failingRunner())
	if inPlace.StoppedAt == "" || len(inPlace.Stages) != 1 {
		t.Fatalf("the in-place run did not stop at its first red: stopped-at %q, %d stages",
			inPlace.StoppedAt, len(inPlace.Stages))
	}
}

// TestFirstRedIsSeparateWorkFromAWholeRun proves AC-13 at the fingerprint the
// registry attaches on: a first-red run and a plain run of one commit, and of
// the fast population, are four distinct pieces of work.
func TestFirstRedIsSeparateWorkFromAWholeRun(t *testing.T) {
	const sha = "0123456789abcdef0123456789abcdef01234567"
	whole := worktreeArgv(sha, verifyengine.Uncut(), false)
	firstRed := worktreeArgv(sha, verifyengine.Uncut(), true)
	if slices.Equal(whole, firstRed) {
		t.Fatalf("a first-red worktree run fingerprints as the whole run: %q", whole)
	}
	if !slices.Contains(firstRed, keywordFirstRed) || !slices.Contains(firstRed, sha) {
		t.Fatalf("first-red fingerprint %q does not name the commit and the keyword", firstRed)
	}
	if slices.Equal(fastArgv(false), fastArgv(true)) {
		t.Fatalf("a first-red fast run fingerprints as the plain one: %q", fastArgv(false))
	}
}

// TestFirstRedWithAPartIsRefused proves AC-14: the line is refused before any
// work, and the refusal says why and names the two spellings that work.
func TestFirstRedWithAPartIsRefused(t *testing.T) {
	_, err := optionsFrom(leaction.Arguments{
		keywordFirstRed: {""}, keywordPart: {"1"}, keywordOf: {"2"},
	}, func(string) string { return "" })
	if err == nil {
		t.Fatal("first-red with part 1 of 2 was accepted")
	}
	for _, want := range []string{"debt clearing", "le verify worktree first-red", "le verify worktree part <i> of <n>"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal %q does not say %q", err, want)
		}
	}
	options, err := optionsFrom(leaction.Arguments{keywordPart: {"1"}, keywordOf: {"2"}}, func(string) string { return "" })
	if err != nil || options.Part != 1 || options.Parts != 2 {
		t.Fatalf("part 1 of 2 parsed to %#v (%v)", options, err)
	}
	if _, err := optionsFrom(leaction.Arguments{keywordPart: {"x"}, keywordOf: {"2"}}, func(string) string { return "" }); err == nil {
		t.Fatal("part x of 2 was accepted")
	}
}

// TestStageTimesReachTheCheckoutOnGreenAndRed proves AC-7 end to end: a
// worktree run, red or green, appends one line per reached stage to the
// checkout's history, not the worktree's, and so does an in-place run.
func TestStageTimesReachTheCheckoutOnGreenAndRed(t *testing.T) {
	repo := newFixtureRepo(t)
	full := len(verifyengine.StagesForMode(verifyengine.Mode))

	red := Run(context.Background(), repo.root, Options{}, failingRunner())
	if red.Code != 1 {
		t.Fatalf("red run = %#v", red)
	}
	if got := len(historyLines(t, repo.root)); got != full {
		t.Fatalf("after a red worktree run the checkout's history holds %d lines, want %d", got, full)
	}
	green := Run(context.Background(), repo.root, Options{}, passingRunner)
	if green.Code != 0 {
		t.Fatalf("green run = %#v", green)
	}
	lines := historyLines(t, repo.root)
	if len(lines) != 2*full {
		t.Fatalf("after a green worktree run the checkout's history holds %d lines, want %d", len(lines), 2*full)
	}
	if !strings.Contains(lines[0], "\t9\t"+verifyengine.Mode+"\t"+red.Commit+"\t") {
		t.Fatalf("the red run's first line %q does not carry its exit, mode and commit", lines[0])
	}

	fast := runFast(context.Background(), repo.root, false, passingRunner)
	if fast.Code != 0 {
		t.Fatalf("fast run = %#v", fast)
	}
	if got := len(historyLines(t, repo.root)); got != 2*full+len(fast.Stages) {
		t.Fatalf("an in-place run appended %d lines, want %d", got-2*full, len(fast.Stages))
	}
}

// TestVerifyListKeepsTheDeclaredOrder proves `le verify list` ignores the
// history: CI deals its shards from this list, so a history that reverses the
// population changes nothing it prints.
//
// Method: point the checkout root at a fixture, give it a history whose
// recorded seconds fall strictly from the first declared stage to the last, so
// a cost order would print the population reversed, and read the list through
// the action `le verify list` dispatches to. A history of equal times, which a
// passing fixture run writes, cannot tell the two orders apart: ties keep
// declared order.
func TestVerifyListKeepsTheDeclaredOrder(t *testing.T) {
	repo := newFixtureRepo(t)
	redsRootedAt(t, repo.root)
	declaredFull := verifyengine.StagesForMode(verifyengine.Mode)
	lines := make([]string, 0, len(declaredFull))
	for position, stage := range declaredFull {
		lines = append(lines, stage.Identity.Name+"\t"+strconv.Itoa(len(declaredFull)-position)+
			"\t0\tfull\tabc\t2026-10-21T03:00:00Z")
	}
	redsFixtureWrite(t, repo.root, verifyengine.StageTimesPath, strings.Join(lines, "\n")+"\n")
	for _, words := range [][]string{{"list"}, {"list", "fast"}} {
		mode := verifyengine.Mode
		if len(words) == 2 {
			mode = verifyengine.FastMode
		}
		declared := verifyengine.StagesForMode(mode)
		answer, code := Answer(words)
		list, isList := answer.(stageList)
		if code != 0 || !isList {
			t.Fatalf("le verify %s answered %d with %#v", strings.Join(words, " "), code, answer)
		}
		for index := range declared {
			if list.Stages[index].Name != declared[index].Identity.Name {
				t.Fatalf("%s list position %d is %s, want declared %s",
					mode, index, list.Stages[index].Name, declared[index].Identity.Name)
			}
		}
	}
}

// TestRedsNamesAStoppedRun proves AC-15: over a run that stopped at its first
// red, the answer stays undetermined and says where the run stopped and that
// its pending stages will never report.
func TestRedsNamesAStoppedRun(t *testing.T) {
	repo := newFixtureRepo(t)
	report := verifyengine.RunMode(context.Background(), repo.root, job.Head(repo.root), verifyengine.Mode,
		verifyengine.Plan{FirstRed: true}, failingRunner(), verifyengine.Slot{})
	if report.StoppedAt == "" {
		t.Fatalf("the run did not stop: %#v", report)
	}
	answer := readReds(repo.root, "internal/mine/a.go")
	if answer.Verdict != VerdictUndetermined {
		t.Fatalf("verdict %q over a stopped run, want %q", answer.Verdict, VerdictUndetermined)
	}
	if answer.StoppedAt != report.StoppedAt {
		t.Fatalf("stopped-at %q, want %q", answer.StoppedAt, report.StoppedAt)
	}
	if !strings.Contains(answer.Text(), "stopped at its first red, "+report.StoppedAt) ||
		!strings.Contains(answer.Text(), "will never report") {
		t.Fatalf("the answer does not name the stop:\n%s", answer.Text())
	}
}
