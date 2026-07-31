//go:build !linux

// Design: plan/learned/744-ipsec-9-ikev2-eap-nat.md -- UDP encapsulation of ESP
// RFC: rfc/short/rfc7296.md -- receive UDP-encapsulated ESP at any time (Section 2.23)
// Related: encap_linux.go -- the platform that can decapsulate ESP in UDP

package transport

import (
	"errors"
	"net"
)

// ErrNoESPInUDP reports that this platform cannot decapsulate ESP inside UDP.
//
// RFC 7296 Section 2.23 MUST:
// "all devices MUST be able to receive and process both UDP-encapsulated ESP and non-UDP-encapsulated ESP packets at any time".
// Ze meets that MUST on Linux, through the XFRM dataplane and the UDP_ENCAP socket
// option. No other platform Ze builds for carries an IPsec dataplane at all.
//
// The named error states the limitation. It does not let a caller read success from
// a nil error (ai/rules/fail-closed-guards.md).
var ErrNoESPInUDP = errors.New("transport: this platform cannot decapsulate ESP in UDP")

// EnableESPInUDP reports ErrNoESPInUDP on every platform that is not Linux.
// The caller reports the limitation through a doctor check.
func EnableESPInUDP(_ *net.UDPConn) error {
	return ErrNoESPInUDP
}

// ESPInUDPEnabled reports false on every platform that is not Linux.
func ESPInUDPEnabled(_ *net.UDPConn) (bool, error) {
	return false, ErrNoESPInUDP
}
