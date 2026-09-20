// Related: cmd_exabgp_process.go — startExaProcess and stopExaProcess, the producers under test

package cli

import (
	"context"
	"os"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// forkAndReportChild runs a shell that starts one background process and prints
// its pid, then waits. The background process stands for what a case's daemon
// starts under itself: ze forks a plugin, the mock forks the bridge's scripts.
// A stop that reaches only the direct child leaves it running.
const forkAndReportChild = "sleep 120 & echo CHILD $!; wait"

func startForkingProcess(t *testing.T, ctx context.Context) (*exaProcess, int) {
	t.Helper()
	proc, err := startExaProcess(ctx, "fixture", "/bin/sh", []string{"-c", forkAndReportChild}, os.Environ(), nil)
	if err != nil {
		t.Fatalf("start fixture process: %v", err)
	}
	return proc, awaitChildPID(t, proc)
}

// awaitChildPID reads the pid the shell printed. The shell writes it before it
// waits, so the line arrives without a timer to wait on.
func awaitChildPID(t *testing.T, proc *exaProcess) int {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		for line := range strings.SplitSeq(proc.stdout.String(), "\n") {
			value, found := strings.CutPrefix(line, "CHILD ")
			if !found {
				continue
			}
			pid, err := strconv.Atoi(strings.TrimSpace(value))
			if err != nil {
				t.Fatalf("shell reported an unreadable pid %q: %v", value, err)
			}
			return pid
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("shell never reported its child pid; stdout was %q", proc.stdout.String())
	return 0
}

// inProcessTable answers whether a pid still names an entry. Signal 0 delivers
// nothing and only reports whether the process could be signaled, so it
// answers true for a zombie as well as for a running process.
//
// Both are failures here, and the stricter reading is the wanted one: a
// process the runner killed but never waited on is a reap left half done, and
// the same unfinished Wait is what leaves the runner blocked on its own pipes.
func inProcessTable(pid int) bool {
	return syscall.Kill(pid, 0) == nil
}

func requireReaped(t *testing.T, pid int, what string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if !inProcessTable(pid) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	// Leave nothing behind whatever the verdict: this test's own leak is the
	// defect it exists to catch.
	_ = syscall.Kill(pid, syscall.SIGKILL)
	t.Errorf("%s (pid %d) is still in the process table, running or unreaped", what, pid)
}

// TestStopExaProcessReapsTheWholeGroup pins the stop a case's branch arms call.
//
// VALIDATES: stopExaProcess ends the process it names and everything that
// process started.
// PREVENTS: a stop aimed at the direct child. The suite starts a shell, and ze
// starts plugins under itself, so the pid the runner holds is never the whole
// tree.
//
// A stop that missed the group would HANG this case rather than fail it fast:
// stopExaProcess waits for its own pipes to reach EOF, and the survivor holds
// the write end. That is the producer's shape, not the test's, and it is also
// why the runner must never stop a group it does not own.
func TestStopExaProcessReapsTheWholeGroup(t *testing.T) {
	proc, childPID := startForkingProcess(t, t.Context())

	stopExaProcess(proc)

	requireReaped(t, proc.cmd.Process.Pid, "the shell")
	requireReaped(t, childPID, "the process the shell started")
}

// TestCanceledContextReapsTheWholeGroup pins the OTHER way a case's processes
// end: the per-case context expiring, and the suite context being canceled
// when the operator interrupts the runner.
//
// VALIDATES: cancellation reaps the whole group, without stopExaProcess being
// called at all.
// PREVENTS: the shape this file carried until the group kill came from
// plugin.KillGroupOnCancel. exec.CommandContext installs its own Cancel, which
// kills the direct child alone, so a canceled suite left every grandchild
// reparented to init, and the direct child stayed a zombie behind it: its
// pipes never reached EOF, so the runner's own Wait never ran.
//
// Measured against that shape, this case fails in 10s with both pids still in
// the table. It is the one route by which the suite was SHOWN to leak; the
// five daemons that started the search are still unexplained
// (plan/journal/parallel-copies-collide-on-a-deterministic-port.md).
func TestCanceledContextReapsTheWholeGroup(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	proc, childPID := startForkingProcess(t, ctx)

	cancel()

	// Nothing here waits on proc.done. That channel closes when both pipes
	// reach EOF, and a surviving grandchild holds the write end, so waiting on
	// it turns "the reap failed" into "the test is slow" and the case goes
	// green the moment the fixture's own sleep expires. The reap is observed
	// directly instead, inside a window the sleep outlives.
	requireReaped(t, proc.cmd.Process.Pid, "the shell")
	requireReaped(t, childPID, "the process the shell started")
}
