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
		_, _, errno := syscall.Syscall6(unix.SYS_WAITID, 1, uintptr(p.cmd.Process.Pid), //nolint:staticcheck // Vendored x/sys/unix has no Darwin waitid libSystem wrapper; WNOWAIT must preserve the owned child identity.
			uintptr(unsafe.Pointer(&info[0])), unix.WEXITED|unix.WNOWAIT, 0, 0) //nolint:gosec // Darwin waitid writes its 104-byte siginfo_t into this aligned 104-byte buffer, retained until the syscall returns.
		if errno != 0 {
			return errno
		}
		return nil
	})
}

// exitedBeforeStop MUST run under signalMu while the parent identity is held.
// The existing aligned siginfo_t binding leaves signo zero for WNOHANG while
// a live parent has no exit to report; WNOWAIT preserves the sole reaper.
func (p *ownedProcess) exitedBeforeStop() (bool, error) {
	var info [13]uint64
	_, _, errno := syscall.Syscall6(unix.SYS_WAITID, 1, uintptr(p.cmd.Process.Pid), //nolint:staticcheck // Vendored x/sys/unix has no Darwin waitid wrapper.
		uintptr(unsafe.Pointer(&info[0])), unix.WEXITED|unix.WNOWAIT|unix.WNOHANG, 0, 0) //nolint:gosec // The existing aligned 104-byte siginfo_t binding receives the nonreaping kernel result.
	if errno != 0 {
		return false, errno
	}
	return int32(info[0]) != 0, nil
}
