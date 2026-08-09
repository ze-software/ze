// Design: docs/architecture/l2tp/bng-5-pppoe.md -- shared /dev/ppp setup for L2TP and PPPoE

//go:build linux

package ppp

import (
	"fmt"
	"os"
	"unsafe"

	"golang.org/x/sys/unix"
)

const (
	pppiocGChan   = 0x80047437 // PPPIOCGCHAN: _IOR('t', 55, int)
	pppiocAttChan = 0x40047438 // PPPIOCATTCHAN: _IOW('t', 56, int)
	pppiocNewUnit = 0xc004743e // PPPIOCNEWUNIT: allocate PPP unit (creates pppN)
)

// DevPPPSetup opens /dev/ppp, attaches the PPP channel from the PPPoX
// socket (L2TP or PPPoE), allocates a PPP unit (creating the pppN
// interface), and returns the channel fd, unit fd, and unit number.
//
// The ioctl sequence is identical for PPPoL2TP and PPPoE:
//  1. PPPIOCGCHAN on pppoxFD -> channel index
//  2. open /dev/ppp, PPPIOCATTCHAN -> channel fd
//  3. open /dev/ppp, PPPIOCNEWUNIT -> unit fd + pppN interface
//
// PPPIOCCONNECT (step 4) is deferred to after LCP completes.
func DevPPPSetup(pppoxFD int) (chanFD, unitFD, unitNum int, err error) {
	chanIdx, err := ioctlGetInt(pppoxFD, pppiocGChan)
	if err != nil {
		return -1, -1, -1, fmt.Errorf("ppp: PPPIOCGCHAN: %w", err)
	}

	chanFD, err = openDevPPP()
	if err != nil {
		return -1, -1, -1, err
	}
	if err := ioctlSetInt(chanFD, pppiocAttChan, chanIdx); err != nil {
		unix.Close(chanFD) //nolint:errcheck // rollback
		return -1, -1, -1, fmt.Errorf("ppp: PPPIOCATTCHAN: %w", err)
	}

	unitFD, err = openDevPPP()
	if err != nil {
		unix.Close(chanFD) //nolint:errcheck // rollback
		return -1, -1, -1, err
	}
	unitNum = -1
	unitNum, err = ioctlGetSetInt(unitFD, pppiocNewUnit, unitNum)
	if err != nil {
		unix.Close(unitFD) //nolint:errcheck // rollback
		unix.Close(chanFD) //nolint:errcheck // rollback
		return -1, -1, -1, fmt.Errorf("ppp: PPPIOCNEWUNIT: %w", err)
	}

	return chanFD, unitFD, unitNum, nil
}

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

func ioctlGetInt(fd int, req uint) (int, error) {
	var val int32
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), uintptr(req), uintptr(unsafe.Pointer(&val))) //nolint:gosec // ioctl pointer arg
	if errno != 0 {
		return 0, errno
	}
	return int(val), nil
}

func ioctlSetInt(fd int, req uint, val int) error {
	v := int32(val)
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), uintptr(req), uintptr(unsafe.Pointer(&v))) //nolint:gosec // ioctl pointer arg
	if errno != 0 {
		return errno
	}
	return nil
}

func ioctlGetSetInt(fd int, req uint, val int) (int, error) {
	v := int32(val)
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), uintptr(req), uintptr(unsafe.Pointer(&v))) //nolint:gosec // ioctl pointer arg
	if errno != 0 {
		return 0, errno
	}
	return int(v), nil
}
