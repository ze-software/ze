// VALIDATES: `le verify worktree all of <m>` runs every piece of one cut over
// one resolved commit, skips the pieces already proven, carries on past a red
// piece, and refuses the whole sweep before any piece when the host cannot run
// one (moved from the four debt-clear sweep tests, assertions kept).
// PREVENTS: a sweep that pays twice for a proven piece, understates what a red
// sweep still owes, or runs hours of pieces before finding the host unfit.
package verify

import (
	"context"
	"errors"
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
