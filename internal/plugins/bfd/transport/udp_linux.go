//go:build linux

// Design: rfc/short/rfc5881.md -- single-hop GTSM (TTL=255)
// Related: udp.go -- UDP transport that invokes applySocketOptions and parseReceivedTTL
// Related: udp_other.go -- non-Linux stub with matching function signatures
//
// Linux-specific socket options and control-message parsing for the BFD
// UDP transport. Splits from udp.go because IP_RECVTTL, IP_TTL, and
// SO_BINDTODEVICE all require either the golang.org/x/sys/unix package
// or Linux-only syscall constants.
package transport

import (
	"encoding/binary"
	"fmt"
	"syscall"

	"golang.org/x/sys/unix"
)

// applySocketOptions configures the BFD UDP socket for production use:
//
//   - IP_RECVTTL: kernel delivers the received packet's IP TTL as an
//     IP_TTL control message on every recvmsg. The engine then enforces
//     RFC 5881 Section 5 (single-hop TTL=255) and RFC 5883 Section 5
//     (multi-hop min-TTL).
//   - IP_TTL=255: every transmitted packet leaves with the maximum TTL
//     so RFC-conformant peers accept it.
//   - SO_BINDTODEVICE <device>: when device is non-empty, the socket is
//     bound to the named network device (a real interface for single-hop,
//     or a VRF device for multi-VRF deployments). Requires CAP_NET_RAW on
//     Linux.
//
// On any setsockopt failure the function returns a wrapped error; the
// caller (UDP.Start) closes the connection and propagates.
func applySocketOptions(c syscall.RawConn, device string) error {
	var innerErr error
	controlErr := c.Control(func(fd uintptr) {
		ifd := int(fd)
		// RFC 5881 Section 5: "The TTL or Hop Limit of the received
		// packet MUST be 255." Enable IP_RECVTTL so recvmsg delivers
		// the kernel's observed TTL as a control message.
		if err := unix.SetsockoptInt(ifd, unix.IPPROTO_IP, unix.IP_RECVTTL, 1); err != nil {
			innerErr = fmt.Errorf("setsockopt IP_RECVTTL: %w", err)
			return
		}
		// RFC 5881 Section 5: "The TTL or Hop Limit of the transmitted
		// packet MUST be 255."
		if err := unix.SetsockoptInt(ifd, unix.IPPROTO_IP, unix.IP_TTL, 255); err != nil {
			innerErr = fmt.Errorf("setsockopt IP_TTL=255: %w", err)
			return
		}
		if device != "" {
			if err := unix.SetsockoptString(ifd, unix.SOL_SOCKET, unix.SO_BINDTODEVICE, device); err != nil {
				innerErr = fmt.Errorf("setsockopt SO_BINDTODEVICE %q: %w", device, err)
				return
			}
		}
	})
	if controlErr != nil {
		return fmt.Errorf("rawconn Control: %w", controlErr)
	}
	return innerErr
}

// parseReceivedTTL parses a recvmsg control-message blob and returns the
// IP TTL value extracted from an IP_TTL cmsg, or zero when no TTL cmsg is
// present. Linux delivers the TTL as a 32-bit integer in native byte
// order inside an IP_TTL control message; we decode the full 32-bit
// value with binary.NativeEndian and return the low byte.
func parseReceivedTTL(oob []byte) uint8 {
	if len(oob) == 0 {
		return 0
	}
	msgs, err := unix.ParseSocketControlMessage(oob)
	if err != nil {
		return 0
	}
	for _, m := range msgs {
		if m.Header.Level != unix.IPPROTO_IP || m.Header.Type != unix.IP_TTL {
			continue
		}
		if len(m.Data) >= 4 {
			return uint8(binary.NativeEndian.Uint32(m.Data[:4]))
		}
		if len(m.Data) >= 1 {
			return m.Data[0]
		}
	}
	return 0
}

// oobBufLen is the size of a per-slot control-message backing buffer.
// One IP_TTL cmsg plus the cmsghdr fits in ~24 bytes on Linux; 64 gives
// headroom for future additions (IP_PKTINFO, timestamp) without forcing
// another allocation.
const oobBufLen = 64
