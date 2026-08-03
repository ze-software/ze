//go:build linux

// Design: plan/learned/744-ipsec-9-ikev2-eap-nat.md -- UDP encapsulation of ESP
// RFC: rfc/short/rfc7296.md -- receive UDP-encapsulated ESP at any time (Section 2.23)
// RFC: rfc/short/rfc3948.md -- UDP encapsulation of ESP packets
// Related: encap_other.go -- the platforms that carry no IPsec dataplane
// Related: udp.go -- the socket this option is applied to

package transport

import (
	"errors"
	"net"

	"golang.org/x/sys/unix"
)

// ErrNoESPInUDP reports that this platform cannot decapsulate ESP inside UDP.
// Linux can, so nothing here returns it. It exists so the symbol is available on
// every platform and a caller needs no build tag of its own.
var ErrNoESPInUDP = errors.New("transport: this platform cannot decapsulate ESP in UDP")

// EnableESPInUDP asks the kernel to decapsulate ESP that arrives inside UDP on this
// socket. RFC 3948 Section 2.1 gives the encapsulation. RFC 7296 Section 2.23 makes
// the receive side a MUST for an implementation that supports NAT traversal.
//
// Without the option the kernel delivers each encapsulated ESP datagram to user
// space. The IKE dispatch then reads four leading octets that hold an ESP SPI, not
// the non-ESP marker of RFC 3948 Section 2.2, and discards the datagram. The ESP
// state the engine installed never matches a packet.
//
// Holding port 4500 without the option is therefore worse than not holding it.
//
// It fails closed. Every failure returns an error and sets nothing
// (ai/rules/evidence.md).
func EnableESPInUDP(c *net.UDPConn) error {
	if c == nil {
		return errors.New("transport: udp encap needs a socket")
	}
	rc, err := c.SyscallConn()
	if err != nil {
		return err
	}
	var setErr error
	ctlErr := rc.Control(func(fd uintptr) {
		setErr = unix.SetsockoptInt(int(fd), unix.IPPROTO_UDP, unix.UDP_ENCAP, unix.UDP_ENCAP_ESPINUDP)
	})
	if ctlErr != nil {
		return ctlErr
	}
	return setErr
}

// ESPInUDPEnabled reads the option back off a socket.
//
// A test asserts on the value the kernel holds rather than on the absence of an
// error. The doctor check reads it for the same reason.
func ESPInUDPEnabled(c *net.UDPConn) (bool, error) {
	if c == nil {
		return false, errors.New("transport: udp encap needs a socket")
	}
	rc, err := c.SyscallConn()
	if err != nil {
		return false, err
	}
	var (
		value  int
		getErr error
	)
	ctlErr := rc.Control(func(fd uintptr) {
		value, getErr = unix.GetsockoptInt(int(fd), unix.IPPROTO_UDP, unix.UDP_ENCAP)
	})
	if ctlErr != nil {
		return false, ctlErr
	}
	if getErr != nil {
		return false, getErr
	}
	return value == unix.UDP_ENCAP_ESPINUDP, nil
}
