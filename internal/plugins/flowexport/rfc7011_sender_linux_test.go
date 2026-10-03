// RFC 7011 conformance test for the UDP checksum on the socket Ze dials.
// Linux computes the checksum of every datagram unless the socket opts out
// through SO_NO_CHECK (IPv4) or UDP_NO_CHECK6_TX (IPv6), so the value Ze
// installs is the absence of either option.

//go:build integration && linux

package flowexport

import (
	"context"
	"net"
	"testing"

	"golang.org/x/sys/unix"
)

// sockoptInt reads one integer socket option off the Sender's UDP socket.
func sockoptInt(t *testing.T, s *Sender, level, opt int) int {
	t.Helper()
	raw, err := s.conn.SyscallConn()
	if err != nil {
		t.Fatal(err)
	}
	var value int
	var optErr error
	if err := raw.Control(func(fd uintptr) {
		value, optErr = unix.GetsockoptInt(int(fd), level, opt)
	}); err != nil {
		t.Fatal(err)
	}
	if optErr != nil {
		t.Fatal(optErr)
	}
	return value
}

// RFC requirement: RFC7011-10.3.2-2 positive -- the IPv4 socket NewSender
// dials has SO_NO_CHECK clear and the IPv6 socket has UDP_NO_CHECK6_TX
// clear, so Linux writes a valid UDP checksum into every datagram Send puts
// on the wire.
func TestRFC7011UDPChecksumEnabledOnSenderSocket(t *testing.T) {
	var lc net.ListenConfig
	pc4, err := lc.ListenPacket(context.Background(), "udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = pc4.Close() }()
	addr4, ok := pc4.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatal("unexpected address type")
	}
	s4, err := NewSender("127.0.0.1", addr4.Port, "", DatagramSizeDefault)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s4.Close() }()
	if got := sockoptInt(t, s4, unix.SOL_SOCKET, unix.SO_NO_CHECK); got != 0 {
		t.Errorf("SO_NO_CHECK = %d on the IPv4 socket, want 0 (checksum computed)", got)
	}

	pc6, err := lc.ListenPacket(context.Background(), "udp6", "[::1]:0")
	if err != nil {
		t.Skipf("no IPv6 loopback: %v", err)
	}
	defer func() { _ = pc6.Close() }()
	addr6, ok := pc6.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatal("unexpected address type")
	}
	s6, err := NewSender("::1", addr6.Port, "", DatagramSizeDefault)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s6.Close() }()
	if got := sockoptInt(t, s6, unix.SOL_UDP, unix.UDP_NO_CHECK6_TX); got != 0 {
		t.Errorf("UDP_NO_CHECK6_TX = %d on the IPv6 socket, want 0 (checksum computed)", got)
	}
}
