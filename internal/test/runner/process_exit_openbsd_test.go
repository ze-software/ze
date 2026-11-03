// Design: docs/architecture/testing/runner-architecture.md -- OpenBSD exit observation.
// Related: process_identity_test.go -- real lost-child and late-cancel regressions.

package runner

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
	"testing"

	"golang.org/x/sys/unix"
)

// TestOpenBSDExitRetiresBeforeReap injects registration, event and ownership
// outcomes. A simulated foreign group replaces the child after observation;
// neither cleanup signaling nor late context cancellation may reach it.
func TestOpenBSDExitRetiresBeforeReap(t *testing.T) {
	for _, tc := range []struct {
		name       string
		register   error
		lost       bool
		lostAfter  bool
		interrupt  bool
		eventError bool
		wantError  error
		wantSignal int
	}{
		{name: "exit-note", wantSignal: 1},
		{name: "already-exiting", register: unix.ESRCH, wantSignal: 1},
		{name: "lost-before-registration", lost: true, wantError: unix.ECHILD},
		{name: "lost-during-registration", register: unix.ESRCH, lostAfter: true, wantError: unix.ECHILD},
		{name: "interrupted-observation", interrupt: true, wantSignal: 1},
		{name: "registration-error", register: unix.EPERM, wantError: unix.EPERM, wantSignal: 1},
		{name: "event-error", eventError: true, wantError: unix.EIO, wantSignal: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			const pid = 42 // Only the seam sees this number; no process is signaled.
			p := &ownedProcess{cmd: &exec.Cmd{Process: &os.Process{Pid: pid}}}
			signals, foreignSignals := 0, 0
			foreign := false
			p.groupSignal = func(gid int, sig syscall.Signal) error {
				if gid != -pid || sig != syscall.SIGKILL {
					t.Errorf("signal=(%d,%v), want owned group SIGKILL", gid, sig)
				}
				if foreign {
					foreignSignals++
				}
				signals++
				return nil
			}
			calls := 0
			kevent := func(fd int, changes, events []unix.Kevent_t, timeout *unix.Timespec) (int, error) {
				calls++
				if tc.interrupt && calls == 1 {
					return 0, unix.EINTR
				}
				if len(changes) != 0 {
					if changes[0].Fflags != unix.NOTE_EXIT {
						t.Fatal("registration did not request exit")
					}
					return 0, tc.register
				}
				unix.SetKevent(&events[0], pid, unix.EVFILT_PROC, unix.EV_EOF)
				events[0].Fflags = unix.NOTE_EXIT
				if tc.eventError {
					events[0].Flags = unix.EV_ERROR
					events[0].Data = int64(unix.EIO)
				}
				return 1, nil
			}
			childCalls := 0
			child := func(int) error {
				childCalls++
				if tc.lost {
					return unix.ECHILD
				}
				if tc.lostAfter && childCalls == 2 {
					return unix.ECHILD
				}
				return nil
			}
			err := p.observeExit(func() error { return waitOpenBSDExit(0, pid, kevent, child) })
			if !errors.Is(err, tc.wantError) {
				t.Errorf("observation=%v, want %v", err, tc.wantError)
			}
			if signals != tc.wantSignal {
				t.Errorf("pre-reap signals=%d, want %d", signals, tc.wantSignal)
			}
			foreign = true
			p.kill()
			if err := p.cancel(); !errors.Is(err, os.ErrProcessDone) {
				t.Errorf("late cancellation=%v", err)
			}
			if foreignSignals != 0 {
				t.Errorf("retired identity reached foreign sentinel %d times", foreignSignals)
			}
		})
	}
}
