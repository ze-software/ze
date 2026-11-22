//go:build integration && linux

// Design: docs/research/l2tpv2-ze-integration.md -- raw /dev/ppp fixture descriptors
// Related: session_release_integration_linux_test.go -- native release pins a raw unit
// Related: session_family_integration_linux_test.go -- native family modes on a raw unit
// Production setup uses CreatePPPChannel and UnitCreator, which retain their
// original file. These raw descriptors exist only for native fixtures.

package kernel

import (
	"fmt"
	"os"
	"unsafe"

	"golang.org/x/sys/unix"
)

const pppiocNewUnit = 0xc004743e // PPPIOCNEWUNIT: _IOWR('t', 62, int)

// openDevPPP opens /dev/ppp as a raw descriptor the caller MUST close.
func openDevPPP() (int, error) {
	f, err := os.OpenFile("/dev/ppp", os.O_RDWR, 0)
	if err != nil {
		return -1, fmt.Errorf("ppp: open /dev/ppp: %w", err)
	}
	rawFD, err := dupFD(f)
	f.Close() //nolint:errcheck // source replaced by rawFD
	if err != nil {
		return -1, fmt.Errorf("ppp: dup /dev/ppp fd: %w", err)
	}
	return rawFD, nil
}

func dupFD(f *os.File) (int, error) {
	raw, err := f.SyscallConn()
	if err != nil {
		return -1, err
	}
	var fd int
	var opErr error
	if err := raw.Control(func(fdp uintptr) {
		fd, opErr = unix.Dup(int(fdp))
	}); err != nil {
		return -1, err
	}
	return fd, opErr
}

func ioctlGetSetInt(fd int, req uint, val int) (int, error) {
	v := int32(val)
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), uintptr(req), uintptr(unsafe.Pointer(&v))) //nolint:gosec // ioctl pointer arg
	if errno != 0 {
		return 0, errno
	}
	return int(v), nil
}
