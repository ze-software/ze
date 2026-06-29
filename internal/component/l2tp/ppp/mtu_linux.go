// Design: docs/research/l2tpv2-implementation-guide.md -- /dev/ppp PPPIOCSMRU, PPPIOCCONNECT
// Related: ops.go -- pppOps struct referencing realSetMRU, realConnect

//go:build linux

package ppp

import (
	"syscall"
	"unsafe"
)

// pppiocSMRU is the ioctl number for PPPIOCSMRU on Linux.
//
// RFC: docs/research/l2tpv2-implementation-guide.md §26.7 cites
// 0x40047452 for x86_64 and notes the value is architecture-
// dependent. The value here matches `_IOW('t', 82, int)` per
// linux/ppp-ioctl.h, which is stable across the Linux kernel
// architectures Go's GOOS=linux supports for PPP (amd64, arm64,
// riscv64, ppc64le, s390x, 386, arm).
const pppiocSMRU = 0x40047452

// realSetMRU performs the PPPIOCSMRU ioctl on a /dev/ppp unit fd.
// mru is the negotiated MRU from LCP; it is passed as a 32-bit
// signed integer per the kernel API (the high bits must be zero
// since MRU is a uint16).
func realSetMRU(unitFD int, mru uint16) error {
	val := int32(mru)
	//nolint:gosec // PPPIOCSMRU's user pointer is the kernel's documented contract; no Go-friendly wrapper exists in golang.org/x/sys/unix.
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL,
		uintptr(unitFD),
		uintptr(pppiocSMRU),
		uintptr(unsafe.Pointer(&val)),
	)
	if errno != 0 {
		return errno
	}
	return nil
}

// pppiocConnect is PPPIOCCONNECT: _IOW('t', 58, int) = 0x4004743a.
const pppiocConnect = 0x4004743a

func realConnect(chanFD, unitNum int) error {
	val := int32(unitNum) //nolint:gosec // unitNum is small
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL,
		uintptr(chanFD),
		uintptr(pppiocConnect),
		uintptr(unsafe.Pointer(&val)),
	)
	if errno != 0 {
		return errno
	}
	return nil
}
