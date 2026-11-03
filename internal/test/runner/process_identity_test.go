// Design: docs/architecture/testing/runner-architecture.md -- retired process identity.

package runner

import (
	"errors"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
)

// TestOwnedCleanupAfterReapNeverSignals substitutes a foreign-group sentinel at
// the syscall boundary after a real child's cached Wait. No reused PID is killed.
func TestOwnedCleanupAfterReapNeverSignals(t *testing.T) {
	p := startOwnedTestProcess(t, exec.CommandContext(t.Context(), "sh", "-c", "exit 0"))
	if err := p.Wait(); err != nil {
		t.Fatal(err)
	}
	var foreignSignals atomic.Int32
	p.groupSignal = func(pid int, sig syscall.Signal) error {
		foreignSignals.Add(1)
		return nil
	}
	p.cleanup()
	p.kill()
	if err := p.cmd.Cancel(); !errors.Is(err, os.ErrProcessDone) {
		t.Errorf("late context cancellation = %v, want ErrProcessDone", err)
	}
	if got := foreignSignals.Load(); got != 0 {
		t.Fatalf("retired identity signaled foreign-group sentinel %d times", got)
	}
}

// TestOwnedLostChildReportsObservationError simulates an outside reaper taking
// this test's child. The owner must report its lost observation, never signal
// the retired numeric identity. No other process number is used.
func TestOwnedLostChildReportsObservationError(t *testing.T) {
	cmd := exec.CommandContext(t.Context(), "sh", "-c", "exit 0")
	p := boundProcessDeadline(cmd)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	var status syscall.WaitStatus
	if _, err := syscall.Wait4(cmd.Process.Pid, &status, 0, nil); err != nil {
		t.Fatal(err)
	}
	var foreignSignals atomic.Int32
	p.groupSignal = func(int, syscall.Signal) error { foreignSignals.Add(1); return nil }
	go p.reap()
	<-p.done // The injected loss must be observed before testing retired cleanup.
	p.cleanup()
	if err := p.Wait(); !errors.Is(err, syscall.ECHILD) {
		t.Errorf("lost child result=%v, want ECHILD", err)
	}
	if err := p.lifecycleError(); err == nil || !strings.Contains(err.Error(), "observe child exit before reap") {
		t.Errorf("lost observation error missing: %v", err)
	}
	if got := foreignSignals.Load(); got != 0 {
		t.Errorf("lost identity signaled foreign-group sentinel %d times", got)
	}
}
