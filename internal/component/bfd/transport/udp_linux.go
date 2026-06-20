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

	"codeberg.org/thomas-mangin/ze/internal/core/env"
)

// bfdTestParallelEnv enables SO_REUSEPORT on the BFD UDP socket so
// multiple ze processes can bind the fixed RFC 5881 / RFC 5883 ports
// concurrently. Intended ONLY for the functional test runner, which
// launches ~20 ze processes in parallel and otherwise fails with
// `bind: address already in use` on the second process to touch a
// given port. Production ze never sets this env var, so the default
// fail-fast behavior (one ze per host owning the BFD ports) is
// preserved.
var _ = env.MustRegister(env.EnvEntry{
	Key:         "ze.bfd.test-parallel",
	Type:        "bool",
	Description: "Test-only: enable SO_REUSEPORT on the BFD UDP socket so parallel .ci tests can co-bind the RFC ports.",
	Private:     true,
})

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
	testParallel := env.GetBool("ze.bfd.test-parallel", false)
	var innerErr error
	controlErr := c.Control(func(fd uintptr) {
		ifd := int(fd)
		// Test-only: SO_REUSEPORT allows multiple ze processes to
		// bind the same (IP, UDP port) tuple concurrently so the
		// functional test runner can launch parallel BFD tests
		// without EADDRINUSE. Kernel distributes inbound packets
		// among the bound sockets via a flow hash; production ze
		// runs one instance per host so the default is off.
		if testParallel {
			if err := unix.SetsockoptInt(ifd, unix.SOL_SOCKET, unix.SO_REUSEPORT, 1); err != nil {
				innerErr = fmt.Errorf("setsockopt SO_REUSEPORT: %w", err)
				return
			}
		}
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

// applySocketOptionsV6 is the IPv6 counterpart of applySocketOptions.
// It enables IPV6_RECVHOPLIMIT so the receiver sees the inbound
// hop limit as a cmsg, sets IPV6_UNICAST_HOPS=255 so outbound
// packets satisfy RFC 5881 Section 5 on the peer's receive check,
// and binds the socket to the given device when non-empty (VRF or
// single-hop interface).
//
// Stage 2b uses this alongside applySocketOptions on a parallel
// IPv6 socket inside transport.Dual.
func applySocketOptionsV6(c syscall.RawConn, device string) error {
	var innerErr error
	controlErr := c.Control(func(fd uintptr) {
		ifd := int(fd)
		if err := unix.SetsockoptInt(ifd, unix.IPPROTO_IPV6, unix.IPV6_RECVHOPLIMIT, 1); err != nil {
			innerErr = fmt.Errorf("setsockopt IPV6_RECVHOPLIMIT: %w", err)
			return
		}
		if err := unix.SetsockoptInt(ifd, unix.IPPROTO_IPV6, unix.IPV6_UNICAST_HOPS, 255); err != nil {
			innerErr = fmt.Errorf("setsockopt IPV6_UNICAST_HOPS=255: %w", err)
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

// parseReceivedTTL parses a recvmsg control-message blob and returns
// the hop count extracted from an IP_TTL or IPV6_HOPLIMIT cmsg, or
// zero when no TTL cmsg is present. Linux delivers the value as a
// 32-bit integer in native byte order; we decode the full 32-bit
// value with binary.NativeEndian and return the low byte. Stage 2b
// extends the parser to also recognize the IPv6 cmsg so a Dual
// transport's v6 readLoop can populate Inbound.TTL via the same
// helper.
func parseReceivedTTL(oob []byte) uint8 {
	if len(oob) == 0 {
		return 0
	}
	msgs, err := unix.ParseSocketControlMessage(oob)
	if err != nil {
		return 0
	}
	for _, m := range msgs {
		isV4 := m.Header.Level == unix.IPPROTO_IP && m.Header.Type == unix.IP_TTL
		isV6 := m.Header.Level == unix.IPPROTO_IPV6 && m.Header.Type == unix.IPV6_HOPLIMIT
		if !isV4 && !isV6 {
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
