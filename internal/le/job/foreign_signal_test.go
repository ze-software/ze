// Design: docs/contributing/running-commands.md -- observational admission waiters.
// Related: contention_test.go -- real processes and isolated registry fixtures.

package job

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// foreignSignals lists every signal a shell can trap that a waiter could send
// a holder without killing it. SIGKILL cannot be trapped, and
// TestAStalledHolderIsBrokenOnItsSilence detects it by the holder's absence.
// SIGSTOP cannot be trapped either, so the stopped state is read separately.
var foreignSignals = []string{"HUP", "INT", "QUIT", "USR1", "USR2", "TERM", "CONT", "ALRM", "PIPE", "WINCH"}

// TestAForeignQuietHolderIsNeverSignalled verifies that a waiter which measures
// a foreign holder's silence leaves the holder untouched by any signal, not
// only by a kill. The holder belongs to another session, so a SIGTERM, a
// SIGUSR1 or a SIGSTOP from a waiter interferes with work it does not own.
//
// Method: the holder is a shell that traps every catchable signal and records
// each one it receives in a file. The waiter observes it past the quiet window
// twice. The test then reads the holder's state through ps, to catch a SIGSTOP,
// and sends the holder one SIGUSR2 of its own as a barrier: once that line is
// recorded, every earlier signal has been recorded too, so the file must hold
// exactly the barrier.
func TestAForeignQuietHolderIsNeverSignalled(t *testing.T) {
	root := fixtureRepo(t)
	ran := filepath.Join(root, "tmp", "ran")
	received := filepath.Join(root, "tmp", "holder-signals")
	ready := filepath.Join(root, "tmp", "holder-ready")

	holder := trappingHolder(t, received, ready)
	entry, logPath := heldEntry(t, root, "quiet-holder", holder.Process.Pid)
	age(t, logPath)
	unchanged := preserveRegistryFiles(t, entry, logPath)

	out := observeQueue(t, root, "has not grown for", "sh", "-c", record(ran, "admitted"))
	if !strings.Contains(out, "holder kept occupied") {
		t.Fatalf("the waiter never measured the holder past its quiet window: %s", out)
	}
	if running, err := alive(t.Context(), holder.Process.Pid); err != nil || !running {
		t.Fatalf("the foreign quiet holder was killed: running=%v err=%v", running, err)
	}
	state := processState(t, holder.Process.Pid)
	if strings.HasPrefix(state, "T") || strings.HasPrefix(state, "t") {
		t.Fatalf("the foreign quiet holder was stopped: ps state %q", state)
	}

	if err := holder.Process.Signal(syscall.SIGUSR2); err != nil {
		t.Fatalf("send the holder the test's barrier: %v", err)
	}
	waitForBarrier(t, received)
	if got := read(t, received); got != "USR2\n" {
		t.Errorf("the waiter signalled the foreign quiet holder; it recorded %q, want only the test's barrier %q", got, "USR2\n")
	}
	unchanged()
	stop(t, holder)
}

// trappingHolder starts a shell that records every trappable signal it
// receives in received, one name per line, and writes ready once its traps are
// set. The `sleep 1 & wait` loop lets a trap run as soon as its signal arrives:
// a shell defers a trap until a foreground child exits.
func trappingHolder(t *testing.T, received, ready string) *exec.Cmd {
	t.Helper()
	var script strings.Builder
	for _, name := range foreignSignals {
		script.WriteString("trap 'echo " + name + " >> \"$1\"' " + name + "; ")
	}
	script.WriteString(": > \"$2\"; while :; do sleep 1 & wait $!; done")
	cmd := exec.CommandContext(context.Background(), "sh", "-c", script.String(), "sh", received, ready)
	if err := cmd.Start(); err != nil {
		t.Fatalf("start a trapping stand-in holder: %v", err)
	}
	t.Cleanup(func() {
		if cmd.ProcessState == nil && cmd.Process != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	})
	ctx, cancel := context.WithTimeout(t.Context(), runContextBound)
	defer cancel()
	ticker := time.NewTicker(helperPoll)
	defer ticker.Stop()
	for {
		if _, err := os.Stat(ready); err == nil {
			return cmd
		}
		select {
		case <-ctx.Done():
			t.Fatalf("the trapping holder never set its traps: no %s", ready)
		case <-ticker.C:
		}
	}
}

// processState answers the ps state letters of pid, failing the test when the
// process cannot be queried.
func processState(t *testing.T, pid int) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), runContextBound)
	defer cancel()
	out, err := exec.CommandContext(ctx, "ps", "-o", "state=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		t.Fatalf("query the holder's state: %v", err)
	}
	return strings.TrimSpace(string(out))
}

// waitForBarrier waits until the holder recorded the test's own SIGUSR2 as its
// last line, bounded by runContextBound.
func waitForBarrier(t *testing.T, received string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), runContextBound)
	defer cancel()
	ticker := time.NewTicker(helperPoll)
	defer ticker.Stop()
	for !strings.HasSuffix(read(t, received), "USR2\n") {
		select {
		case <-ctx.Done():
			t.Fatalf("the holder never recorded the test's barrier: %q", read(t, received))
		case <-ticker.C:
		}
	}
}
