//go:build darwin

// Design: docs/architecture/testing/qemu-integration.md -- a VM never outlives its le

package testqemu

import (
	"errors"
	"fmt"
	"syscall"

	"golang.org/x/sys/unix"
)

// processStartTime answers when pid started, in microseconds since the epoch,
// from the kernel's process table, or errProcessGone when no process holds it.
func processStartTime(pid int) (int64, error) {
	if err := syscall.Kill(pid, 0); errors.Is(err, syscall.ESRCH) {
		return 0, errProcessGone
	}
	info, err := unix.SysctlKinfoProc("kern.proc.pid", pid)
	if err != nil {
		return 0, fmt.Errorf("read kern.proc.pid.%d: %w", pid, err)
	}
	if int(info.Proc.P_pid) != pid {
		return 0, errProcessGone // it exited between the two reads
	}
	start := info.Proc.P_starttime
	return start.Sec*1_000_000 + int64(start.Usec), nil
}
