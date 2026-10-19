// Related: process.go -- systemProcessRunner.Run, whose group kill and
// WaitDelay these tests pin.

package interoplab

import (
	"context"
	"errors"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// grandchildPid runs script, which prints the pid of the child it puts in the
// background and then waits for it, under a context canceled 300 ms after the
// start, and answers that pid, how long Run took, and Run's error.
func grandchildPid(t *testing.T, script string) (int, time.Duration, error) {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	timer := time.AfterFunc(300*time.Millisecond, cancel)
	defer timer.Stop()

	start := time.Now()
	result, err := systemProcessRunner{}.Run(ctx, ProcessCommand{Arguments: []string{"sh", "-c", script}, Timeout: time.Minute})
	took := time.Since(start)
	pid, parseErr := strconv.Atoi(strings.TrimSpace(result.Stdout))
	if parseErr != nil {
		t.Fatalf("the script printed no grandchild pid: %q (%v)", result.Stdout, err)
	}
	return pid, took, err
}

// pidGone reports whether no process holds pid within two seconds: a killed
// grandchild is reparented and then reaped by init, on init's schedule.
func pidGone(pid int) bool {
	tick := time.NewTicker(50 * time.Millisecond)
	defer tick.Stop()
	for range 40 {
		if errors.Is(syscall.Kill(pid, 0), syscall.ESRCH) {
			return true
		}
		<-tick.C
	}
	return false
}

// VALIDATES: G-2: a canceled command takes its whole process group with it,
// so a grandchild of the docker client, or a compiler under a Go build, dies
// on a Ctrl-C too.
// METHOD: `sh -c 'sleep 30 & echo $!; wait'` under a context canceled after
// 300 ms. Run answers the cancel and the backgrounded sleep is gone.
// exec.CommandContext's own cancel kills sh alone and leaves the sleep.
// PREVENTS: cmd.Cancel losing the group kill, which no other test notices.
func TestCanceledCommandKillsItsGrandchildren(t *testing.T) {
	pid, took, err := grandchildPid(t, "sleep 30 & echo $!; wait")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Run answered %v, want context.Canceled", err)
	}
	if took > 10*time.Second {
		t.Fatalf("Run took %v after a 300ms cancel", took)
	}
	if !pidGone(pid) {
		syscall.Kill(pid, syscall.SIGKILL) //nolint:errcheck,gosec // cleanup of the leaked grandchild the failure reports
		t.Fatalf("grandchild %d outlived the canceled command", pid)
	}
}

// VALIDATES: a descendant that left the process group and holds the stdout
// pipe keeps Run waiting no longer than processWaitDelay after a cancel.
// METHOD: `setsid sleep 20` escapes the group kill and inherits stdout; Run
// still returns within processWaitDelay plus a margin. Without WaitDelay it
// waits for the sleep, 20 seconds.
// PREVENTS: a Ctrl-C that cannot end a command whose escaped descendant holds
// its output open.
func TestCanceledCommandDoesNotWaitOnAnEscapedPipeHolder(t *testing.T) {
	if _, err := exec.LookPath("setsid"); err != nil {
		t.Skip("setsid is not on PATH (macOS has none), so the escape this test needs cannot be made")
	}
	pid, took, err := grandchildPid(t, "setsid sleep 20 & echo $!; wait")
	defer syscall.Kill(pid, syscall.SIGKILL) //nolint:errcheck // the escaped sleep is outside the group kill by design
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Run answered %v, want context.Canceled", err)
	}
	if took > processWaitDelay+3*time.Second {
		t.Fatalf("Run took %v after a 300ms cancel, want at most processWaitDelay (%v) plus a margin", took, processWaitDelay)
	}
}
