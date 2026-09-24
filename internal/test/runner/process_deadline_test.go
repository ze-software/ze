package runner

import (
	"bytes"
	"context"
	"os/exec"
	"testing"
	"time"
)

// VALIDATES: the test deadline ends a started process AND the descendants that
// inherited its output pipes, so Wait returns at the deadline.
// PREVENTS: display-fill-completion.ci, declared at 90s, running 588s: the
// context killed the fixture alone, its children kept the runner's stdout pipe
// open, and Wait blocked until they exited on their own.
func TestStartedProcessDeadlineEndsTheTree(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 300*time.Millisecond)
	defer cancel()

	// The grandchild sleep holds stdout long after sh would have been killed.
	args := []string{"-c", "sleep 60 & wait"}
	proc := exec.CommandContext(ctx, "sh", args...)
	var out bytes.Buffer
	proc.Stdout = &out
	proc.Stderr = &out
	proc, err := startWithETXTBSYRetry(ctx, "sh", args, proc)
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	start := time.Now()
	_ = proc.Wait() //nolint:errcheck // the process is killed by design; only the return time matters
	elapsed := time.Since(start)

	// Well under processWaitDelay: the group kill closed the pipe, not the
	// WaitDelay backstop.
	if elapsed >= processWaitDelay/2 {
		t.Fatalf("Wait returned after %v; the deadline did not end the process tree", elapsed)
	}
}
