// Design: docs/architecture/testing/runner-architecture.md -- signal/reap exclusion.

package runner

import (
	"errors"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// TestOwnedCleanupPinsIdentityThroughSignal parks the numeric syscall, then
// lets the real leader exit. Reaping must not pass that in-flight syscall.
// The seam models reuse as a foreign-group sentinel, never a real reused PID.
func TestOwnedCleanupPinsIdentityThroughSignal(t *testing.T) {
	input, release, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	defer release.Close()
	cmd := exec.CommandContext(t.Context(), "sh", "-c", "read line; exit 0")
	cmd.Stdin = input
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	entered, resume := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	var foreignSignals atomic.Int32
	p := &ownedProcess{cmd: cmd, done: make(chan struct{})}
	p.groupSignal = func(int, syscall.Signal) error {
		if calls.Add(1) == 1 {
			close(entered)
			<-resume
		}
		select {
		case <-p.done:
			foreignSignals.Add(1)
		default:
		}
		return nil
	}
	go p.reap()
	cleaned := make(chan struct{})
	go func() { p.cleanup(); close(cleaned) }()
	<-entered
	if err := release.Close(); err != nil {
		t.Fatal(err)
	}
	var info unix.Siginfo
	err = unix.Waitid(unix.P_PID, cmd.Process.Pid, &info, unix.WEXITED|unix.WNOWAIT, nil)
	if errors.Is(err, unix.ECHILD) {
		t.Error("leader reaped while its numeric group signal is in flight")
	} else if err != nil {
		t.Error(err)
	}
	select {
	case <-p.done:
		t.Error("Wait completed before the numeric signal returned")
	case <-time.After(100 * time.Millisecond):
		// Negative control: resume is the only way out of the parked syscall.
	}
	close(resume)
	<-cleaned
	if got := foreignSignals.Load(); got != 0 {
		t.Errorf("in-flight signal reached foreign-group sentinel %d times", got)
	}
	if err := p.Wait(); err != nil {
		t.Error(err)
	}
}

// TestOwnedNormalExitCleansDescendantsBeforeReap proves the final group kill
// sees an exited, unreaped leader and closes a real descendant's inherited pipe.
func TestOwnedNormalExitCleansDescendantsBeforeReap(t *testing.T) {
	output := newSyncWriterPattern("child=")
	cmd := exec.CommandContext(t.Context(), "sh", "-c", "sleep 60 & echo child=$!; exit 23")
	cmd.Stdout = output
	p := boundProcessDeadline(cmd)
	var signals atomic.Int32
	p.groupSignal = func(pid int, sig syscall.Signal) error {
		var info unix.Siginfo
		if err := unix.Waitid(unix.P_PID, -pid, &info, unix.WEXITED|unix.WNOWAIT|unix.WNOHANG, nil); err != nil {
			t.Errorf("final signal lost leader identity: %v", err)
		} else if info.Signo != int32(unix.SIGCHLD) {
			t.Errorf("final signal preceded leader exit: signo=%d", info.Signo)
		}
		signals.Add(1)
		return syscall.Kill(pid, sig)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	// Read the descendant identity before starting the owner, so the test can
	// clean up its own child even under the no-final-signal discrimination cut.
	if !output.waitFor(t.Context()) {
		t.Fatal("no descendant identity")
	}
	pid, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(output.String(), "child=")))
	if err != nil {
		t.Fatal(err)
	}
	descendant, err := os.FindProcess(pid)
	if err != nil {
		t.Fatal(err)
	}
	defer descendant.Release()
	defer descendant.Kill() // pidfd-backed on Linux; never kill a recycled number.
	go p.reap()
	defer p.cleanup()
	select {
	case <-p.done:
	case <-time.After(2 * time.Second):
		t.Error("normal exit left the descendant holding the output pipe")
		_ = descendant.Kill()
		<-p.done
	}
	var exit *exec.ExitError
	if !errors.As(p.Wait(), &exit) || exit.ExitCode() != 23 {
		t.Errorf("normal leader status = %v, want exit 23", p.Wait())
	}
	if got := signals.Load(); got != 1 {
		t.Errorf("pre-reap descendant signals = %d, want 1", got)
	}
}

// TestOwnedExitObservationFailures proves interrupted observations retry and
// failed observations retire numeric signaling, without leaving a live waiter.
func TestOwnedExitObservationFailures(t *testing.T) {
	for _, failure := range []error{unix.ECHILD, unix.ENOSYS} {
		t.Run(failure.Error(), func(t *testing.T) {
			cmd := exec.CommandContext(t.Context(), "sh", "-c", "exit 0")
			p := boundProcessDeadline(cmd)
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			var signals int
			p.groupSignal = func(int, syscall.Signal) error { signals++; return nil }
			calls := 0
			err := p.observeExit(func() error {
				calls++
				if calls == 1 {
					return unix.EINTR
				}
				return failure
			})
			if !errors.Is(err, failure) {
				t.Errorf("observation = %v, want %v", err, failure)
			}
			if calls != 2 {
				t.Errorf("wait calls = %d, want interrupted call and retry", calls)
			}
			wantSignals := 1
			if errors.Is(failure, unix.ECHILD) {
				wantSignals = 0
			}
			if signals != wantSignals {
				t.Errorf("failure signals=%d, want %d", signals, wantSignals)
			}
			p.kill()
			if signals != wantSignals {
				t.Error("observation failure did not retire identity")
			}
			// Only this test owns the process; its real normal exit is still
			// observable even though the injected observation failed.
			go p.reap()
			p.cleanup()
			if err := p.Wait(); err != nil {
				t.Error(err)
			}
		})
	}
}
