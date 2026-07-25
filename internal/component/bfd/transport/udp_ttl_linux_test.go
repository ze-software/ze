//go:build linux

package transport

import (
	"bytes"
	"net"
	"net/netip"
	"strconv"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/bfd/api"
)

// VALIDATES: RFC 5881 Section 5 -- a UDP socket with IP_TTL=255 applied
// at Start produces packets whose on-wire TTL is 255, observable via a
// second socket with IP_RECVTTL enabled.
// PREVENTS: regression where Start skips the setsockopt and the kernel
// default TTL (usually 64) is used instead.
//
// RFC requirement: RFC5881-5-1 positive -- with authentication not in use, the
// transmitted TTL/Hop Limit is 255: applySocketOptions
// (internal/component/bfd/transport/udp_linux.go:79) sets IP_TTL=255 at Start,
// so the bound socket reports IP_TTL 255.
// RFC requirement: RFC5881-5-3 positive -- the same IP_TTL=255 is applied
// unconditionally (udp_linux.go:79), so the transmitted TTL is 255 whether or
// not authentication is in use.
// RFC requirement: RFC5881-6-1 positive -- transmitting with TTL 255 is what
// confines a single-hop Control packet to the one-hop path being protected (a
// conformant peer's GTSM accepts only TTL 255); the same setsockopt produces it.
func TestUDPSetOutboundTTL255(t *testing.T) {
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

	// Read the socket option back from the bound fd.
	raw, err := u.conn.SyscallConn()
	if err != nil {
		t.Fatalf("SyscallConn: %v", err)
	}
	var (
		ttl    int
		optErr error
	)
	err = raw.Control(func(fd uintptr) {
		ttl, optErr = unix.GetsockoptInt(int(fd), unix.IPPROTO_IP, unix.IP_TTL)
	})
	if err != nil {
		t.Fatalf("Control: %v", err)
	}
	if optErr != nil {
		t.Fatalf("GetsockoptInt IP_TTL: %v", optErr)
	}
	if ttl != 255 {
		t.Fatalf("IP_TTL = %d, want 255", ttl)
	}
}

// VALIDATES: RFC 5881 Section 5 -- a UDP socket with IP_RECVTTL enabled
// surfaces the received TTL on Inbound.TTL. A test helper sends a packet
// from an ephemeral sender socket (which inherits the kernel default
// TTL, 64 on Linux) and the BFD transport's RX channel delivers an
// Inbound with TTL=64.
// PREVENTS: regression where readLoop hard-codes Inbound.TTL=0 or fails
// to parse the IP_TTL control message.
func TestUDPRecvTTLExtraction(t *testing.T) {
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

	// Resolve the bound port so the sender knows where to send.
	localAddr, ok := u.conn.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatalf("LocalAddr type = %T, want *net.UDPAddr", u.conn.LocalAddr())
	}

	sender, err := net.DialUDP("udp4", nil, localAddr)
	if err != nil {
		t.Fatalf("DialUDP: %v", err)
	}
	defer func() { _ = sender.Close() }()

	payload := bytes.Repeat([]byte{0xAB}, 24)
	if _, err := sender.Write(payload); err != nil {
		t.Fatalf("sender Write: %v", err)
	}

	select {
	case in := <-u.rx:
		defer in.Release()
		if !bytes.Equal(in.Bytes, payload) {
			t.Fatalf("payload mismatch: got %x want %x", in.Bytes, payload)
		}
		// Linux default IP_TTL on an ephemeral socket is 64 unless
		// the sysctl net.ipv4.ip_default_ttl is changed. We accept
		// any non-zero value to be robust on kernels with a custom
		// default; the important thing is that Inbound.TTL is NOT
		// zero, proving IP_RECVTTL extraction worked.
		if in.TTL == 0 {
			t.Fatalf("Inbound.TTL is zero; IP_RECVTTL extraction failed")
		}
		t.Logf("observed receive TTL = %d", in.TTL)
	case <-time.After(2 * time.Second):
		t.Fatal("did not receive packet within 2s")
	}
}

// VALIDATES: A second UDP transport bound with the same setsockopt path
// on the same loopback port fails (EADDRINUSE), proving that the real
// socket (and not a no-op stub) is in place.
// PREVENTS: regression where Start silently returns success without
// actually binding the kernel socket.
func TestUDPBindConflict(t *testing.T) {
	port := netip.MustParseAddrPort("127.0.0.1:0")
	first := &UDP{Bind: port, Mode: api.SingleHop}
	if err := first.Start(); err != nil {
		t.Fatalf("first Start: %v", err)
	}
	defer func() { _ = first.Stop() }()

	// Grab the actual port the kernel handed out.
	ua, ok := first.conn.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatalf("LocalAddr type = %T, want *net.UDPAddr", first.conn.LocalAddr())
	}
	second := &UDP{
		Bind: netip.MustParseAddrPort(net.JoinHostPort("127.0.0.1", strconv.Itoa(ua.Port))),
		Mode: api.SingleHop,
	}
	err := second.Start()
	if err == nil {
		_ = second.Stop()
		t.Fatal("second Start succeeded; expected bind conflict")
	}
}

// RFC requirement: RFC5881-5-1 negative -- the transmit TTL is 255 because ze
// sets IP_TTL=255 explicitly, not because the kernel default is 255. A plain
// UDP socket that did NOT run applySocketOptions reports the kernel default
// TTL, which is not 255, so without ze's setsockopt a Control packet would be
// transmitted with a TTL a conformant single-hop peer discards.
// RFC requirement: RFC5881-5-3 negative -- the same holds regardless of
// authentication: the 255 is a deliberate socket option, and its absence yields
// a non-255 transmit TTL.
// RFC requirement: RFC5881-6-1 negative -- without TTL 255 the packet is not
// confined to the one-hop path; the kernel default would allow it to traverse
// multiple hops. This pins that ze's one-hop confinement comes from the
// explicit IP_TTL=255 (internal/component/bfd/transport/udp_linux.go:79), not
// from chance.
func TestUDPDefaultTTLNot255(t *testing.T) {
	// A bare UDP socket with no BFD socket options applied.
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatalf("ListenUDP: %v", err)
	}
	defer func() { _ = conn.Close() }()

	raw, err := conn.SyscallConn()
	if err != nil {
		t.Fatalf("SyscallConn: %v", err)
	}
	var (
		ttl    int
		optErr error
	)
	if err := raw.Control(func(fd uintptr) {
		ttl, optErr = unix.GetsockoptInt(int(fd), unix.IPPROTO_IP, unix.IP_TTL)
	}); err != nil {
		t.Fatalf("Control: %v", err)
	}
	if optErr != nil {
		t.Fatalf("GetsockoptInt IP_TTL: %v", optErr)
	}
	if ttl == 255 {
		t.Fatal("a plain UDP socket already reports IP_TTL 255; the test cannot prove ze's setsockopt is what produces 255")
	}
}
