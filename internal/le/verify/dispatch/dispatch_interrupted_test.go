package verifydispatch

// VALIDATES: dispatch refuses a stage as interrupted only when the cancel cut
// the stage process short, and returns the exit of a stage that answered on
// its own even when the run's context is done by the time dispatch reads it.
// PREVENTS: a Ctrl-C landing just after a stage exited 1 turning that red into
// an interrupted stage with no verdict, which the failure index then writes as
// never judged and the commit gate does not charge.

import (
	"context"
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
