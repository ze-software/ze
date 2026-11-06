// Design: docs/architecture/testing/runner-architecture.md -- identity before reap.
// Related: process_owner.go -- observation, locked retirement and the sole reap.

package runner

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"syscall"

	"golang.org/x/sys/unix"
)

// beforeReap uses NOTE_EXIT without consuming the child's wait status. OpenBSD's
// filt_procattach rejects an already-exiting child with ESRCH. The process table
// includes zombies, so that race is distinguished from a lost child without a
// reaping wait. proc_finish_wait, called only by our later Cmd.Wait, frees the PID.
func (p *ownedProcess) beforeReap() error {
	return p.observeExit(func() error {
		syscall.ForkLock.RLock()
		fd, err := unix.Kqueue()
		if err == nil {
			unix.CloseOnExec(fd)
		}
		syscall.ForkLock.RUnlock()
		if err != nil {
			return err
		}
		defer unix.Close(fd) //nolint:errcheck // private observation descriptor
		return waitOpenBSDExit(fd, p.cmd.Process.Pid, unix.Kevent, openBSDChild)
	})
}

func waitOpenBSDExit(fd, pid int,
	kevent func(int, []unix.Kevent_t, []unix.Kevent_t, *unix.Timespec) (int, error),
	child func(int) error,
) error {
	if err := child(pid); err != nil {
		return err
	}
	var change unix.Kevent_t
	unix.SetKevent(&change, pid, unix.EVFILT_PROC, unix.EV_ADD|unix.EV_ONESHOT)
	change.Fflags = unix.NOTE_EXIT
	// No event buffer: registration failures are returned as syscall errors.
	_, err := kevent(fd, []unix.Kevent_t{change}, nil, nil)
	if errors.Is(err, unix.ESRCH) {
		// Only exiting/missing processes produce ESRCH for a valid child PID.
		// A still-owned child cannot be reused: this observer never reaps it.
		return child(pid)
	}
	if err != nil {
		return err
	}
	var events [1]unix.Kevent_t
	n, err := kevent(fd, nil, events[:], nil)
	if err != nil {
		return err
	}
	if n != 1 {
		return fmt.Errorf("exit observation returned %d events", n)
	}
	event := events[0]
	if event.Flags&unix.EV_ERROR != 0 {
		return unix.Errno(event.Data)
	}
	if event.Ident != uint64(pid) {
		return fmt.Errorf("exit observation returned child %d", event.Ident)
	}
	if event.Fflags&unix.NOTE_EXIT == 0 {
		return fmt.Errorf("exit observation returned flags %d", event.Fflags)
	}
	return nil
}

// openBSDChild reads the fixed-width kinfo_proc prefix through p_ppid (sysctl.h).
// sysctl_doproc permits a short prefix and scans both allprocess and zombprocess.
// An absent or foreign child grants no numeric signaling permission. Other
// observation errors follow observeExit's stop-before-retirement error path.
func openBSDChild(pid int) error {
	const prefixSize = 116 // Twelve uint64 fields, then five int32 fields.
	const kernProcPID = 1
	info, err := unix.SysctlRaw("kern.proc", kernProcPID, pid, prefixSize, 1)
	if err != nil {
		return err
	}
	if len(info) == 0 {
		return unix.ECHILD
	}
	if len(info) != prefixSize {
		return fmt.Errorf("child identity prefix has %d bytes, want %d", len(info), prefixSize)
	}
	if int32(binary.NativeEndian.Uint32(info[108:112])) != int32(pid) {
		return unix.ECHILD
	}
	if int32(binary.NativeEndian.Uint32(info[112:116])) != int32(os.Getpid()) {
		return unix.ECHILD
	}
	return nil
}
