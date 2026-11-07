// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- IPv6 network-protocol mode of a PPP unit
// Related: devppp_linux.go -- SetIPv4Mode, the IPv4 sibling sharing PPPIOCSNPMODE
// Linux API: https://github.com/torvalds/linux/blob/v6.18/include/uapi/linux/ppp-ioctl.h

//go:build linux

package ppp

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/unix"
)

// pppProtocolIPv6 is the PPP protocol number of IPv6 datagrams (RFC 5072
// Section 3), the npioctl protocol PPPIOCSNPMODE gates.
const pppProtocolIPv6 = 0x57

// SetIPv6Mode controls IPv6 in both directions without detaching the PPP unit.
// The caller MUST serialize this call with descriptor closure and MUST keep the
// unit descriptor reserved for the duration of the call.
// npioctl native-endian layout: 0..3 int protocol | 4..7 enum NPmode mode.
// RFC 5072 Section 2: "Before any IPv6 packets may be communicated, PPP MUST
// reach the network-layer protocol phase, and the IPv6 Control Protocol MUST
// reach the Opened state."
func SetIPv6Mode(unitFD int, mode NPMode) error {
	value, err := mode.kernelValue()
	if err != nil {
		return err
	}
	request := [2]int32{pppProtocolIPv6, value}
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(unitFD), pppiocSNPMODE,
		uintptr(unsafe.Pointer(&request))) //nolint:gosec // ioctl requires a native npioctl pointer.
	if errno != 0 {
		return fmt.Errorf("ppp: cannot change IPv6 forwarding on the PPP unit; check PPP support with ze doctor: %w", errno)
	}
	return nil
}
