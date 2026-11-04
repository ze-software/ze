package verifydispatch

// VALIDATES: dispatch refuses a stage as interrupted only when the cancel cut
// the stage process short, and returns the exit of a stage that answered on
// its own even when the run's context is done by the time dispatch reads it.
// PREVENTS: a Ctrl-C landing just after a stage exited 1 turning that red into
// an interrupted stage with no verdict, which the failure index then writes as
// never judged and the commit gate does not charge.

import (
	"context"
	"os"
	"os/exec"
	"sync/atomic"
	"testing"
	"time"

	verifyengine "github.com/ze-software/ze/internal/le/verify/engine"
	"github.com/ze-software/ze/internal/test/runner"
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

// TestCancelOfAnExitedStageDoesNotMarkItCutShort checks the real owner after
// joining a successful child, then canceling its context. Close and repeated
// Wait MUST preserve the child's actual zero exit verdict.
func TestCancelOfAnExitedStageDoesNotMarkItCutShort(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^$") //nolint:gosec // The test binary itself, with no test selected.
	process, err := runner.StartServerProcess(cmd)
	if err != nil {
		t.Fatalf("start the exiting process: %v", err)
	}
	defer func() {
		if err := process.Close(); err != nil {
			t.Errorf("close the exited process: %v", err)
		}
	}()
	if err := process.Wait(); err != nil {
		t.Fatalf("join the exiting process: %v", err)
	}
	cancel()
	if err := process.Close(); err != nil {
		t.Fatalf("close after late cancellation: %v", err)
	}
	if !cmd.ProcessState.Exited() || cmd.ProcessState.ExitCode() != 0 {
		t.Fatalf("late cancellation changed the natural exit: %v", cmd.ProcessState)
	}
	if err := process.Wait(); err != nil {
		t.Fatalf("repeated Wait changed the successful result: %v", err)
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
