// Design: docs/architecture/testing/runner-architecture.md -- identity before reap.
// Related: process_exit_linux.go -- the same WNOWAIT ownership on Linux.

package runner

import (
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

// beforeReap uses Darwin's waitid, which x/sys/unix does not wrap. XNU's
// bsd/kern/kern_exit.c waitid_nocancel leaves the child waitable with WNOWAIT.
// bsd/sys/wait.h defines P_PID as 1; bsd/sys/signal.h defines the 64-bit
// siginfo_t as six 32-bit fields followed by ten 64-bit fields (104 bytes).
// Both supported Darwin architectures use that layout. The result is opaque:
// only success matters, and Cmd.Wait still collects the actual exit status.
func (p *ownedProcess) beforeReap() error {
	var info [13]uint64
	return p.observeExit(func() error {
		_, _, errno := syscall.Syscall6(unix.SYS_WAITID, 1, uintptr(p.cmd.Process.Pid),
			uintptr(unsafe.Pointer(&info[0])), unix.WEXITED|unix.WNOWAIT, 0, 0)
		if errno != 0 {
			return errno
		}
		return nil
	})
}
