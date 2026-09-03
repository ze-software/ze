// VALIDATES: a stage that could not judge its subject keeps that outcome
// through the run, a stage that judged the tree and found it wrong outranks it,
// and a write failure separates a full device from a run that broke.
// PREVENTS: a run that cleared nothing answering with the exit code a red
// answers, which is the false verdict plan/journal/full-disk-false-red.md
// records seven times.
package verifyengine

import (
	"context"
	"errors"
	"fmt"
	"io"
	"syscall"
	"testing"
	"time"
)

// twoStageRunner answers first for stage 0, second for stage 1, and zero for
// every stage after them.
func twoStageRunner(first, second int) ActionRunner {
	call := 0
	return func(_ context.Context, _ string, identity Identity) ActionResult {
		result := ActionResult{Identity: identity, Registered: true, Completed: true}
		switch call {
		case 0:
			result.Code = first
		case 1:
			result.Code = second
		}
		call++
		return result
	}
}

func TestAStageThatCouldNotJudgeIsNotFlattenedToFailed(t *testing.T) {
	for _, test := range []struct {
		name  string
		stage int
		want  int
	}{
		{name: "every stage green", stage: 0, want: 0},
		{name: "a stage could not judge", stage: stageUnjudged, want: Unjudged},
		{name: "a stage judged and failed", stage: 1, want: 1},
		{name: "any other stage status", stage: 37, want: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			report := run(context.Background(), t.TempDir(), "abc",
				twoStageRunner(0, test.stage), time.Now)
			if report.Code != test.want {
				t.Fatalf("run status = %d, want %d for a stage that exited %d",
					report.Code, test.want, test.stage)
			}
			if report.Stages[1].Code != test.stage {
				t.Fatalf("stage status = %d, want the %d the action answered",
					report.Stages[1].Code, test.stage)
			}
		})
	}
}

// TestAStageThatJudgedTheTreeOutranksOneThatCouldNot drives BOTH orders. The
// red-first order is what holds the precedence guard in runCode: without it a
// later stage that could not judge would demote a red the run had already
// reached, which is unjudged outranking a verdict.
func TestAStageThatJudgedTheTreeOutranksOneThatCouldNot(t *testing.T) {
	for _, test := range []struct {
		name          string
		first, second int
	}{
		{name: "could not judge, then judged and failed", first: stageUnjudged, second: 9},
		{name: "judged and failed, then could not judge", first: 9, second: stageUnjudged},
	} {
		t.Run(test.name, func(t *testing.T) {
			report := run(context.Background(), t.TempDir(), "abc",
				twoStageRunner(test.first, test.second), time.Now)
			if report.Code != 1 {
				t.Fatalf("run status = %d, want 1: a stage that found the tree wrong judged it", report.Code)
			}
		})
	}
}

func TestAFullDeviceIsUnjudgedAndAnyOtherWriteFailureIsABrokenRun(t *testing.T) {
	for _, test := range []struct {
		name string
		err  error
		want int
	}{
		{name: "a full device", err: syscall.ENOSPC, want: Unjudged},
		{name: "a full device under a wrapper", err: fmt.Errorf("copy stage logs: %w", syscall.ENOSPC), want: Unjudged},
		{name: "a permission refusal", err: syscall.EACCES, want: 2},
		{name: "a short write", err: io.ErrShortWrite, want: 2},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := brokenCode(test.err); got != test.want {
				t.Fatalf("brokenCode(%v) = %d, want %d", test.err, got, test.want)
			}
			if Defeated(test.err) != (test.want == Unjudged) {
				t.Fatalf("Defeated(%v) = %v", test.err, Defeated(test.err))
			}
		})
	}
	if Defeated(nil) || Defeated(errors.New("no space left on device")) {
		t.Fatal("a full device is recognized by its typed error, never by its text")
	}
}
