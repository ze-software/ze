// Design: docs/architecture/testing/runner-architecture.md -- owned process teardown.

package runner

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
)

// startOwnedTestProcess uses the runner's start transition and registers the
// same mandatory cleanup, including when a test assertion aborts the caller.
func startOwnedTestProcess(t *testing.T, cmd *exec.Cmd) *ownedProcess {
	t.Helper()
	owned, err := startWithETXTBSYRetry(t.Context(), cmd.Path, cmd.Args[1:], cmd, nil, forkThreadSpawner)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		owned.cleanup()
		if err := owned.lifecycleError(); err != nil {
			t.Errorf("owned child lifecycle: %v", err)
		}
	})
	return owned
}

// TestRunnerEarlyReturnReapsBackground starts a real helper, waits for its ready
// line, then reaches a missing stop target. The recorded PID must already have
// been reaped when runTest releases its work directory and port lease.
func TestRunnerEarlyReturnReapsBackground(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "owned.pid")
	r, rec := lastHelperRecord(t, "owned-background", "10s", &RunCommand{
		Mode: modeBackground, Seq: 1, Name: "owned", Ready: "owned-ready",
		Exec: `sh -c 'echo $$ > "$OWNED_PID_FILE"; echo owned-ready; exec sleep 60'`,
	})
	rec.EnvVars = append(rec.EnvVars, "OWNED_PID_FILE="+pidFile)
	rec.RunCommands = append(rec.RunCommands, RunCommand{Mode: modeStop, Seq: 2, Name: "missing"})
	if r.runTest(t.Context(), rec, &RunOptions{}) {
		t.Fatal("unknown stop target passed")
	}
	if rec.Error == nil || !strings.Contains(rec.Error.Error(), "unknown background process") {
		t.Fatalf("failure = %v, want unknown stop target", rec.Error)
	}
	data, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		t.Fatal(err)
	}
	// This test owns the child and uses its explicit PID, never process discovery.
	var status syscall.WaitStatus
	reaped, waitErr := syscall.Wait4(pid, &status, syscall.WNOHANG, nil)
	if !errors.Is(waitErr, syscall.ECHILD) {
		t.Errorf("runner returned before sole reap: wait4(%d) = %d, %v", pid, reaped, waitErr)
		if reaped == 0 {
			if err := syscall.Kill(pid, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
				t.Error(err)
			}
			if _, err := syscall.Wait4(pid, &status, 0, nil); err != nil {
				t.Error(err)
			}
		}
	}
	if _, err := os.Stat(rec.WorkDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("work directory remains: %v", err)
	}
}
