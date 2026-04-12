//go:build !linux

// Design: rfc/short/rfc5881.md -- single-hop GTSM (TTL=255)
// Related: udp.go -- UDP transport that invokes applySocketOptions and parseReceivedTTL
// Related: udp_linux.go -- Linux implementation with real socket options
//
// Non-Linux stub for the BFD transport socket options. Ze's BFD
// production deployment target is Linux; other platforms compile this
// file so developers on macOS, FreeBSD, OpenBSD, and NetBSD can still
// build and run the daemon for development. On these platforms the
// stub still applies IP_TTL=255 via the stdlib syscall package so the
// outbound packets remain RFC 5881 Section 5 compliant -- missing that
// setsockopt would cause every peer to silently drop ze's transmits.
// SO_BINDTODEVICE is Linux-specific and any device-binding request is
// rejected outright. Receive-side TTL extraction via IP_RECVTTL is also
// Linux-specific; the stub returns zero, which makes the engine fail
// closed on single-hop sessions (TTL=0 != 255) as intended.
package transport

import (
	"errors"
	"fmt"
	"syscall"
)

// errBindToDeviceUnsupported is returned by applySocketOptions on non-Linux
// builds when the caller asks for a device binding.
var errBindToDeviceUnsupported = errors.New("bfd: SO_BINDTODEVICE is Linux-only")

// applySocketOptions applies IP_TTL=255 on the outbound path so non-Linux
// developer builds still produce RFC 5881 Section 5 compliant traffic.
// SO_BINDTODEVICE is rejected -- the underlying Linux primitive does not
// exist on BSD/macOS. IP_RECVTTL is not attempted: BSD reuses the name
// with different semantics and the engine fails closed on missing TTL,
// which is the intended behaviour on platforms ze does not officially
// target.
func applySocketOptions(c syscall.RawConn, device string) error {
	if device != "" {
		return errBindToDeviceUnsupported
	}
	var innerErr error
	controlErr := c.Control(func(fd uintptr) {
		// RFC 5881 Section 5: "The TTL or Hop Limit of the transmitted
		// packet MUST be 255."
		if err := syscall.SetsockoptInt(int(fd), syscall.IPPROTO_IP, syscall.IP_TTL, 255); err != nil {
			innerErr = fmt.Errorf("setsockopt IP_TTL=255: %w", err)
		}
	})
	if controlErr != nil {
		return fmt.Errorf("rawconn Control: %w", controlErr)
	}
	return innerErr
}

// applySocketOptionsV6 mirrors applySocketOptions on the IPv6 side
// for non-Linux builds. It rejects device binding for the same
// reason (SO_BINDTODEVICE is Linux-only) and does not attempt to
// set IPV6_UNICAST_HOPS via a portable path because the BSD
// semantics differ per platform. The engine's TTL gate will fail
// closed on receive until a production operator runs on Linux.
func applySocketOptionsV6(c syscall.RawConn, device string) error {
	if device != "" {
		return errBindToDeviceUnsupported
	}
	var innerErr error
	controlErr := c.Control(func(fd uintptr) {
		if err := syscall.SetsockoptInt(int(fd), syscall.IPPROTO_IPV6, syscall.IPV6_UNICAST_HOPS, 255); err != nil {
			innerErr = fmt.Errorf("setsockopt IPV6_UNICAST_HOPS=255: %w", err)
		}
	})
	if controlErr != nil {
		return fmt.Errorf("rawconn Control: %w", controlErr)
	}
	return innerErr
}

// parseReceivedTTL returns zero on non-Linux builds: the engine then
// fails closed on single-hop sessions (TTL=0 != 255) and multi-hop
// sessions drop packets whose MinTTL is not 0 (never, since the default
// is 254).
func parseReceivedTTL(_ []byte) uint8 { return 0 }

// oobBufLen sizes the per-slot oob backing buffer; kept as a constant so
// the portable readLoop code allocates the same slice shape on every
// platform. Non-Linux builds never write into it.
const oobBufLen = 64
