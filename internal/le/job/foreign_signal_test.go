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
// a holder without killing it, including URG and CHLD, whose default action is
// to ignore them. SIGKILL cannot be trapped, and
// TestAStalledHolderIsBrokenOnItsSilence detects it by the holder's absence.
// SIGSTOP cannot be trapped either, so the stopped state is read separately.
var foreignSignals = []string{
	"HUP", "INT", "QUIT", "USR1", "USR2", "TERM", "CONT", "ALRM", "PIPE", "WINCH", "URG", "CHLD",
}

// holderBarrier is the signal the test itself sends the holder once the waiter
// has exited, and the line the holder records for it.
const holderBarrier = "USR2"

// holderSettle is how long the record must stay unchanged after the barrier
// before the test reads it. A shell runs the traps pending at one moment in
// signal-number order, not arrival order, so a signal the waiter sent before
// the barrier can be recorded after it, within the same pass over the pending
// set. That pass takes microseconds; the settle is five polls.
const holderSettle = 5 * helperPoll

// TestAForeignQuietHolderIsNeverSignalled verifies that a waiter which measures
// a foreign holder's silence leaves the holder untouched by any signal, not
// only by a kill. The holder belongs to another session, so a SIGTERM, a
// SIGUSR1 or a SIGSTOP from a waiter interferes with work it does not own.
//
// Method: the holder is a shell that traps every catchable signal and records
// each one it receives in a file. The waiter observes it past the quiet window
// twice. The test then reads the holder's state through ps, to catch a SIGSTOP,
// and sends the holder one SIGUSR2 of its own as a barrier. A signal is pending
// at its target once kill returns, so every signal the exited waiter sent is
// pending or recorded by then. The shell runs pending traps in signal-number
// order, so a pending TERM or URG is recorded after the barrier: the test waits
// for the barrier line, then for the record to stay unchanged for holderSettle,
// and the file must then hold exactly the barrier, whatever the order.
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
	waitForSettled(t, received)
	if got := read(t, received); got != "USR2\n" {
		t.Errorf("the waiter signalled the foreign quiet holder; it recorded %q, want only the test's barrier %q", got, "USR2\n")
	}
	if foreign := recordedBesidesBarrier(read(t, received)); len(foreign) != 0 {
		t.Errorf("the waiter sent the foreign quiet holder %v", foreign)
	}
	unchanged()
	stopTrappingHolder(t, holder)
}

// trappingHolder starts a shell that records every trappable signal it
// receives in received, one name per line, and writes ready once its traps are
// set. The shell waits on one background child with `wait`, which a trapped
// signal interrupts, so a trap runs as soon as its signal arrives: a shell
// defers a trap until a foreground child exits. The child is one long sleep,
// started once, because a child that exits sends the holder its own SIGCHLD,
// which the CHLD trap would record as the waiter's. The sleep outlives every
// bound the test waits under, so any CHLD the holder records came from outside.
// The holder leads its own process group, so stopTrappingHolder ends the sleep
// with it.
func trappingHolder(t *testing.T, received, ready string) *exec.Cmd {
	t.Helper()
	var script strings.Builder
	for _, name := range foreignSignals {
		script.WriteString("trap 'echo " + name + " >> \"$1\"' " + name + "; ")
	}
	script.WriteString(": > \"$2\"; sleep 3600 & child=$!; while :; do wait $child; done")
	cmd := exec.CommandContext(context.Background(), "sh", "-c", script.String(), "sh", received, ready)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatalf("start a trapping stand-in holder: %v", err)
	}
	t.Cleanup(func() {
		if cmd.ProcessState == nil && cmd.Process != nil {
			stopTrappingHolder(t, cmd)
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

// stopTrappingHolder kills the holder's whole process group, its long sleep
// included, then reaps the holder. The group is killed before the reap, so
// the holder's PID, which names the group, cannot have been reused.
func stopTrappingHolder(t *testing.T, cmd *exec.Cmd) {
	t.Helper()
	if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL); err != nil {
		t.Logf("kill the trapping holder's process group: %v", err)
	}
	_ = cmd.Wait()
}

// recordedBesidesBarrier answers every line of the holder's record other than
// one barrier line, in the order recorded.
func recordedBesidesBarrier(record string) []string {
	var foreign []string
	barrierSeen := false
	for line := range strings.Lines(record) {
		name := strings.TrimSuffix(line, "\n")
		if name == holderBarrier && !barrierSeen {
			barrierSeen = true
			continue
		}
		foreign = append(foreign, name)
	}
	return foreign
}

// waitForSettled waits until the holder's record has not changed for
// holderSettle, bounded by runContextBound, so a trap the shell ran after the
// barrier's is recorded before the test reads the file.
func waitForSettled(t *testing.T, received string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), runContextBound)
	defer cancel()
	ticker := time.NewTicker(helperPoll)
	defer ticker.Stop()
	last := read(t, received)
	stableSince := time.Now()
	for time.Since(stableSince) < holderSettle {
		select {
		case <-ctx.Done():
			t.Fatalf("the holder's record never settled: %q", read(t, received))
		case <-ticker.C:
		}
		if current := read(t, received); current != last {
			last = current
			stableSince = time.Now()
		}
	}
}

// waitForBarrier waits until the holder recorded the test's own SIGUSR2 on any
// line, bounded by runContextBound. It need not be the last line: a pending
// signal numbered above SIGUSR2 is recorded after it.
func waitForBarrier(t *testing.T, received string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), runContextBound)
	defer cancel()
	ticker := time.NewTicker(helperPoll)
	defer ticker.Stop()
	for !strings.Contains("\n"+read(t, received), "\n"+holderBarrier+"\n") {
		select {
		case <-ctx.Done():
			t.Fatalf("the holder never recorded the test's barrier: %q", read(t, received))
		case <-ticker.C:
		}
	}
}
