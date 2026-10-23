// VALIDATES: `le verify worktree all of <m>` runs every piece of one cut over
// one resolved commit, skips the pieces already proven, carries on past a red
// piece, and refuses the whole sweep before any piece when the host cannot run
// one (moved from the four debt-clear sweep tests, assertions kept).
// PREVENTS: a sweep that pays twice for a proven piece, understates what a red
// sweep still owes, or runs hours of pieces before finding the host unfit.
package verify

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	verifyengine "github.com/ze-software/ze/internal/le/verify/engine"
)

// anyHost is a host preflight that passes, for the tests whose question is what
// the sweep does with a verdict rather than whether the host can run it.
func anyHost(Options) error { return nil }

// everyStageRed answers exit 1 for every stage, so a sweep meets a red piece
// wherever it looks. The question is what the SWEEP does with one, never
// whether the stage was right to be red.
func everyStageRed(_ context.Context, _ string, identity verifyengine.Identity) verifyengine.ActionResult {
	return verifyengine.ActionResult{Identity: identity, Registered: true, Completed: true, Code: 1}
}

// TestWorktreeSweepProvesEveryPiece: one invocation runs every piece in order
// and writes the green record when the last one lands.
//
// MUTATION: run only the first piece and this fails: one piece proves one piece.
func TestWorktreeSweepProvesEveryPiece(t *testing.T) {
	repo := newFixtureRepo(t)
	head := repo.git(t, "rev-parse", "HEAD")
	report := Sweep(context.Background(), repo.root, Options{Parts: 3}, passingRunner, anyHost)
	if report.Code != 0 {
		t.Fatalf("a green sweep answered %d: %v", report.Code, report.Diagnostics)
	}
	if !slices.Equal(report.Ran, []int{1, 2, 3}) || !slices.Equal(report.Proven, []int{1, 2, 3}) {
		t.Errorf("the sweep ran %v and proved %v, want [1 2 3] both", report.Ran, report.Proven)
	}
	if !report.Green || report.Commit != head {
		t.Errorf("the sweep reports green %v over %s, want true over %s", report.Green, report.Commit, head)
	}
	record, err := ReadGreenRecord(repo.root)
	if err != nil || record.Pieces() != 3 || record.Commit() != head {
		t.Errorf("after a whole sweep the green record is (%+v, %v), want %s cut 3", record, err, head)
	}
}

// TestWorktreeSweepSkipsProvenPiece: a sweep resumed after a kill does not run
// again what an earlier run proved at the same commit and cut, and a sweep over
// a commit the green record already covers runs nothing.
//
// MUTATION: drop the parts-record consultation and this fails: the sweep runs
// the proven piece a second time.
func TestWorktreeSweepSkipsProvenPiece(t *testing.T) {
	repo := newFixtureRepo(t)
	if report := Run(context.Background(), repo.root, Options{Part: 1, Parts: 3}, passingRunner); report.Code != 0 {
		t.Fatalf("the first piece answered %d", report.Code)
	}
	report := Sweep(context.Background(), repo.root, Options{Parts: 3}, passingRunner, anyHost)
	if report.Code != 0 {
		t.Fatalf("the sweep answered %d: %v", report.Code, report.Diagnostics)
	}
	if !slices.Equal(report.Skipped, []int{1}) || !slices.Equal(report.Ran, []int{2, 3}) {
		t.Errorf("the sweep skipped %v and ran %v, want [1] and [2 3]", report.Skipped, report.Ran)
	}
	if !slices.Equal(report.Proven, []int{1, 2, 3}) || !report.Green {
		t.Errorf("the sweep proved %v green %v, want [1 2 3] true", report.Proven, report.Green)
	}

	again := Sweep(context.Background(), repo.root, Options{Parts: 3}, passingRunner, anyHost)
	if again.Code != 0 || len(again.Ran) != 0 || !slices.Equal(again.Skipped, []int{1, 2, 3}) {
		t.Errorf("a second sweep over a green commit: code %d ran %v skipped %v, want 0, none, [1 2 3]",
			again.Code, again.Ran, again.Skipped)
	}
}

// TestWorktreeSweepCarriesOnPastRedPiece: the pieces are independent, so one red
// says nothing about the ones behind it. Every piece runs, none is proven, no
// green record is written, and the exit is non-zero.
//
// MUTATION: break out of the loop on a red piece and this fails.
func TestWorktreeSweepCarriesOnPastRedPiece(t *testing.T) {
	repo := newFixtureRepo(t)
	report := Sweep(context.Background(), repo.root, Options{Parts: 3}, everyStageRed, anyHost)
	if report.Code == 0 {
		t.Fatalf("a red sweep answered 0: %v", report.Diagnostics)
	}
	if !slices.Equal(report.Failed, []int{1, 2, 3}) {
		t.Errorf("the sweep reports %v red, want [1 2 3]: a red piece must not end the sweep", report.Failed)
	}
	if report.Green || len(report.Proven) != 0 {
		t.Errorf("a red sweep reports green %v proven %v", report.Green, report.Proven)
	}
	mustHaveNoGreenRecord(t, repo.root, "a red sweep")
}

// TestWorktreeSweepWithoutGreenRecordExitsNonZero: every piece passes, but the
// green record is not written (here ZE_SKIP_SUITES is set, so no run can prove
// a push), and the sweep exits non-zero with the reason on its verdict line.
// A zero exit would read as a covered push when nothing records one.
//
// MUTATION: drop the not-green check after the loop and this fails: the sweep
// answers 0.
func TestWorktreeSweepWithoutGreenRecordExitsNonZero(t *testing.T) {
	repo := newFixtureRepo(t)
	t.Setenv("ZE_SKIP_SUITES", "functional")
	report := Sweep(context.Background(), repo.root, Options{Parts: 3}, passingRunner, anyHost)
	if len(report.Failed) != 0 || !slices.Equal(report.Ran, []int{1, 2, 3}) {
		t.Fatalf("fixture: ran %v red %v, want every piece run and none red: %v", report.Ran, report.Failed, report.Diagnostics)
	}
	if report.Code == 0 {
		t.Fatalf("a sweep that wrote no green record answered 0: %v", report.Diagnostics)
	}
	if report.Green {
		t.Fatal("a sweep with suites skipped reports green")
	}
	verdict := report.Diagnostics[len(report.Diagnostics)-1]
	if !strings.Contains(verdict, "no green record was written") || !strings.Contains(verdict, "all of 3") {
		t.Errorf("the verdict line is %q, want it to say no green record was written and the command", verdict)
	}
	if !strings.Contains(verdict, "ZE_SKIP_SUITES=functional") || strings.Contains(verdict, "diagnostics above") {
		t.Errorf("the verdict line is %q, want it to name the skipped suites itself", verdict)
	}
	mustHaveNoGreenRecord(t, repo.root, "a sweep with suites skipped")
}

// TestWorktreeSweepOnUnreadyHostStartsNoPiece: a host that cannot run one piece
// refuses the whole sweep before the first piece starts. The method refuses only
// piece 3 of 3 and counts the stages the runner was handed.
func TestWorktreeSweepOnUnreadyHostStartsNoPiece(t *testing.T) {
	repo := newFixtureRepo(t)
	missing := errors.New("this host is missing loopback addresses")
	checked := []int{}
	host := func(options Options) error {
		checked = append(checked, options.Part)
		if options.Part == 3 {
			return missing
		}
		return nil
	}
	stages := 0
	runner := func(ctx context.Context, root string, identity verifyengine.Identity) verifyengine.ActionResult {
		stages++
		return passingRunner(ctx, root, identity)
	}
	report := Sweep(context.Background(), repo.root, Options{Parts: 3}, runner, host)
	if report.Code != 2 {
		t.Errorf("a refused sweep answered %d, want 2", report.Code)
	}
	if stages != 0 || len(report.Ran) != 0 {
		t.Errorf("the runner was handed %d stages (%v pieces) after the host was refused", stages, report.Ran)
	}
	if !slices.Equal(checked, []int{1, 2, 3}) {
		t.Errorf("the preflight checked pieces %v, want [1 2 3]", checked)
	}
	if !strings.Contains(report.Text(), missing.Error()) {
		t.Errorf("the refusal does not name the host: %s", report.Text())
	}
}

// writeCompleteParts leaves the state a piece leaves when its recordPart
// completes the cut and the green write after it fails: the parts record holds
// every piece of a cut of of over commit, and no green record names commit.
func writeCompleteParts(t *testing.T, root, commit string, of int) {
	t.Helper()
	passed := make([]int, 0, of)
	for index := 1; index <= of; index++ {
		passed = append(passed, index)
	}
	rendered, err := json.Marshal(cutParts{Commit: commit, Of: of, Passed: passed, Updated: "2026-10-23T00:00:00Z"})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, filepath.FromSlash(partsPath))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, rendered, 0o600); err != nil {
		t.Fatal(err)
	}
}

// TestWorktreeSweepWritesGreenFromCompleteParts: a parts record that already
// proves every piece of the cut for the resolved commit, with no green record
// naming that commit (the green write failed after recordPart), is finished by
// the sweep: it writes the green record from the parts record, runs no piece,
// forgets the parts record and exits 0. Every guard the green writer applies
// held for each piece before recordPart wrote it, so the record is the proof.
//
// MUTATION: drop the complete-parts branch and this fails: every rerun skips
// every piece, runs nothing and exits 1, forever.
func TestWorktreeSweepWritesGreenFromCompleteParts(t *testing.T) {
	repo := newFixtureRepo(t)
	head := repo.git(t, "rev-parse", "HEAD")
	writeCompleteParts(t, repo.root, head, 3)
	stages := 0
	counting := func(ctx context.Context, root string, identity verifyengine.Identity) verifyengine.ActionResult {
		stages++
		return passingRunner(ctx, root, identity)
	}
	report := Sweep(context.Background(), repo.root, Options{Parts: 3}, counting, anyHost)
	if report.Code != 0 || !report.Green {
		t.Fatalf("a sweep over complete parts answered %d green %v: %v", report.Code, report.Green, report.Diagnostics)
	}
	if stages != 0 || len(report.Ran) != 0 || !slices.Equal(report.Skipped, []int{1, 2, 3}) {
		t.Errorf("the sweep ran %d stages and pieces %v, skipped %v, want none run and [1 2 3] skipped",
			stages, report.Ran, report.Skipped)
	}
	if !slices.Equal(report.Proven, []int{1, 2, 3}) {
		t.Errorf("the sweep proved %v, want [1 2 3]", report.Proven)
	}
	record, err := ReadGreenRecord(repo.root)
	if err != nil || record.Commit() != head || record.Pieces() != 3 {
		t.Fatalf("the green record is (%+v, %v), want %s cut 3", record, err, head)
	}
	if want := repo.git(t, "rev-parse", head+"^{tree}"); record.Tree() != want {
		t.Errorf("the green record names tree %s, want %s", record.Tree(), want)
	}
	if _, err := os.Stat(filepath.Join(repo.root, filepath.FromSlash(partsPath))); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the parts record survived the green record it proved: %v", err)
	}
}

// TestWorktreeSweepGreenFromPartsFailureNamesItsLine: when the green record
// cannot be written from complete parts either, the sweep exits non-zero, prints
// the reason as its own line, and the verdict refers only to that line, never
// to diagnostics nobody printed. A non-empty directory where the record goes
// makes the write fail.
//
// MUTATION: drop the printed reason and this fails: the verdict points at a line
// that does not exist.
func TestWorktreeSweepGreenFromPartsFailureNamesItsLine(t *testing.T) {
	repo := newFixtureRepo(t)
	head := repo.git(t, "rev-parse", "HEAD")
	writeCompleteParts(t, repo.root, head, 3)
	blocker := filepath.Join(repo.root, filepath.FromSlash(GreenPath))
	if err := os.MkdirAll(filepath.Join(blocker, "occupied"), 0o750); err != nil {
		t.Fatal(err)
	}
	report := Sweep(context.Background(), repo.root, Options{Parts: 3}, passingRunner, anyHost)
	if report.Code == 0 || report.Green {
		t.Fatalf("a sweep whose green write failed answered %d green %v: %v", report.Code, report.Green, report.Diagnostics)
	}
	if len(report.Diagnostics) < 2 {
		t.Fatalf("the sweep printed %v, want the write failure and then the verdict", report.Diagnostics)
	}
	reason := report.Diagnostics[len(report.Diagnostics)-2]
	if !strings.Contains(reason, "green record not written") || !strings.Contains(reason, "write green record") {
		t.Errorf("the line before the verdict is %q, want the green write failure", reason)
	}
	verdict := report.Diagnostics[len(report.Diagnostics)-1]
	if !strings.Contains(verdict, "the line above says why") || strings.Contains(verdict, "diagnostics above") {
		t.Errorf("the verdict is %q, want it to name the one line printed above it", verdict)
	}
}
