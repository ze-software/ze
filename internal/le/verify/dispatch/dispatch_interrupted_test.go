package verifydispatch

// VALIDATES: dispatch refuses a stage as interrupted only when the cancel cut
// the stage process short, and returns the exit of a stage that answered on
// its own even when the run's context is done by the time dispatch reads it.
// PREVENTS: a Ctrl-C landing just after a stage exited 1 turning that red into
// an interrupted stage with no verdict, which the failure index then writes as
// never judged and the commit gate does not charge.

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"sync/atomic"
	"testing"
	"time"

	verifyengine "github.com/ze-software/ze/internal/le/verify/engine"
)

// lateCancel is a context that reports done only after its first Err call,
// and whose Done channel never closes: it stands for an interrupt that lands
// after the stage process has exited, because exec never cancels the process
// and dispatch's entry check passes, while every later Err answers canceled.
type lateCancel struct {
	context.Context
	calls atomic.Int32
}

func (c *lateCancel) Err() error {
	if c.calls.Add(1) == 1 {
		return nil
	}
	return context.Canceled
}

func (c *lateCancel) Deadline() (time.Time, bool) { return time.Time{}, false }

// TestCancelOfAnExitedStageDoesNotMarkItCutShort holds that the stage's
// Cancel records a cut-short stage only when its signal reached a running
// process. exec's context watcher can call Cancel after Wait has reaped the
// process, and the signal then answers os.ErrProcessDone: the stage exited on
// its own, so its exit is a verdict and MUST NOT be refused as interrupted.
// Method: run this test binary to completion, then call the Cancel function
// on it.
func TestCancelOfAnExitedStageDoesNotMarkItCutShort(t *testing.T) {
	cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^$") //nolint:gosec // the test binary itself, with no test selected
	if err := cmd.Run(); err != nil {
		t.Fatalf("run the exiting process: %v", err)
	}
	var cutShort atomic.Bool
	err := cancelStage(cmd, &cutShort)()
	if !errors.Is(err, os.ErrProcessDone) {
		t.Fatalf("Cancel on an exited process = %v, want os.ErrProcessDone", err)
	}
	if cutShort.Load() {
		t.Fatal("Cancel on an exited process marked the stage cut short, discarding its exit")
	}
}

// TestDispatchKeepsTheAnswerOfAStageThatExitedBeforeTheCancel holds that a
// stage which exited on its own keeps its exit status and is not refused as
// interrupted. Method: the probe answers 37 under a context that only reads
// as canceled after the stage has run.
func TestDispatchKeepsTheAnswerOfAStageThatExitedBeforeTheCancel(t *testing.T) {
	ctx := &lateCancel{Context: context.Background()}
	result := runProbe(ctx, t, t.TempDir(),
		verifyengine.Identity{Name: "late", Command: probeRoot, Args: []string{"alpha"}})
	if !result.Completed || result.Code != 37 || result.Failure != nil {
		t.Fatalf("a stage that answered before the cancel = %#v, want its exit 37", result)
	}
}
