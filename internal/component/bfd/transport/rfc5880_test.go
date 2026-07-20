//go:build linux

// VALIDATES: RFC 5880 Section 9 -- a single-hop BFD socket transmits with the
// IP TTL / IPv6 Hop Limit set to the maximum value of 255, which is the
// sending half of the GTSM protection that the receive gate relies on.
// PREVENTS: a socket that leaves the kernel default TTL in place, which a
// conformant peer would discard and which would let an off-link forgery look
// indistinguishable from a legitimate packet.
package transport

import (
	"net/netip"
	"testing"

	"golang.org/x/sys/unix"

	"codeberg.org/thomas-mangin/ze/internal/component/bfd/api"
)

// rfc5880SockoptInt reads one integer socket option off the transport's bound
// file descriptor.
func rfc5880SockoptInt(t *testing.T, u *UDP, level, opt int) int {
	t.Helper()
	raw, err := u.conn.SyscallConn()
	if err != nil {
		t.Fatalf("SyscallConn: %v", err)
	}
	var (
		value  int
		optErr error
	)
	if err := raw.Control(func(fd uintptr) {
		value, optErr = unix.GetsockoptInt(int(fd), level, opt)
	}); err != nil {
		t.Fatalf("Control: %v", err)
	}
	if optErr != nil {
		t.Fatalf("GetsockoptInt(%d, %d): %v", level, opt, optErr)
	}
	return value
}

// RFC requirement: RFC5880-9-2 positive -- for a single-hop session the TTL is
// set to the maximum on transmit. applySocketOptions
// (internal/component/bfd/transport/udp_linux.go:79) sets IP_TTL to 255 on the
// IPv4 socket at Start, so every Control packet the engine hands to this
// transport leaves with the maximum TTL. The receive half of the same
// requirement -- checking the TTL equals the maximum -- is passesTTLGate
// (internal/component/bfd/engine/loop.go:158-170).
func TestRFC5880SingleHopTransmitTTLIsMaximum(t *testing.T) {
	u := &UDP{
		Bind: netip.MustParseAddrPort("127.0.0.1:0"),
		Mode: api.SingleHop,
	}
	if err := u.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() {
		if err := u.Stop(); err != nil {
			t.Errorf("Stop: %v", err)
		}
	}()

	if got := rfc5880SockoptInt(t, u, unix.IPPROTO_IP, unix.IP_TTL); got != 255 {
		t.Fatalf("IP_TTL = %d, want the maximum 255", got)
	}
}
