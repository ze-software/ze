//go:build linux

// VALIDATES: the UDP ports BFD packets are actually sent to and from, observed
// on loopback rather than read off the transport's configuration: RFC 5881
// Section 4 (Control to 3784, Echo to 3785, one source port per session) and
// RFC 5883 Section 5 (multihop Control to 4784), in IPv4 and IPv6.
// PREVENTS: a transport whose Send addresses a port other than the one the RFC
// names while its bound port still looks right.
//
// Every test binds a BFD well-known port, so each runs in a user and network
// namespace of its own (userns.Enter): a ze on the host holding 0.0.0.0:3784,
// or a parallel run of this package, can never take the port from it.
package bfd

import (
	"errors"
	"net"
	"net/netip"
	"os"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bfd/api"
	"github.com/ze-software/ze/internal/component/bfd/transport"
	"github.com/ze-software/ze/internal/test/userns"
)

// Loopback addresses of the wire-port tests. Linux answers the whole of
// 127.0.0.0/8 on lo, so the sender and the listener take two addresses of
// their own and the listener never receives the sender's own datagram.
var (
	wireSender   = netip.MustParseAddr("127.88.81.1")
	wireListener = netip.MustParseAddr("127.88.81.2")
)

// wireListen binds a UDP listener on wireListener at port.
func wireListen(t *testing.T, port uint16) *net.UDPConn {
	t.Helper()
	conn, err := net.ListenUDP("udp4", net.UDPAddrFromAddrPort(netip.AddrPortFrom(wireListener, port)))
	if err != nil {
		t.Fatalf("listen %s:%d: %v", wireListener, port, err)
	}
	t.Cleanup(func() { conn.Close() }) //nolint:errcheck // Test cleanup; the socket is discarded.
	return conn
}

// wireStart moves tr onto wireSender, keeping the port its constructor chose,
// and starts it.
func wireStart(t *testing.T, tr *transport.UDP) {
	t.Helper()
	tr.Bind = netip.AddrPortFrom(wireSender, tr.Bind.Port())
	if err := tr.Start(); err != nil {
		t.Fatalf("start transport on %s: %v", tr.Bind, err)
	}
	t.Cleanup(func() { tr.Stop() }) //nolint:errcheck // Test cleanup; the transport is discarded.
}

// wireSend sends count datagrams from tr to the wire listener address.
func wireSend(t *testing.T, tr *transport.UDP, count int) {
	t.Helper()
	for i := range count {
		out := transport.Outbound{To: wireListener, Mode: tr.Mode, Bytes: []byte{'b', 'f', 'd', byte(i)}}
		if err := tr.Send(out); err != nil {
			t.Fatalf("send %d: %v", i, err)
		}
	}
}

// wireSources reads count datagrams from conn and returns the source address
// and port of each. A datagram that never arrives fails the test.
func wireSources(t *testing.T, conn *net.UDPConn, count int) []netip.AddrPort {
	t.Helper()
	sources := make([]netip.AddrPort, 0, count)
	buf := make([]byte, 64)
	for i := range count {
		if err := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
			t.Fatalf("set deadline: %v", err)
		}
		_, from, err := conn.ReadFromUDPAddrPort(buf)
		if err != nil {
			t.Fatalf("datagram %d of %d never reached %s: %v", i+1, count, conn.LocalAddr(), err)
		}
		sources = append(sources, from)
	}
	return sources
}

// wireSilent fails the test when any datagram reaches conn within 200 ms.
func wireSilent(t *testing.T, conn *net.UDPConn, why string) {
	t.Helper()
	if err := conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond)); err != nil {
		t.Fatalf("set deadline: %v", err)
	}
	buf := make([]byte, 64)
	_, from, err := conn.ReadFromUDPAddrPort(buf)
	if err == nil {
		t.Fatalf("a datagram from %s reached %s: %s", from, conn.LocalAddr(), why)
	}
	if !errors.Is(err, os.ErrDeadlineExceeded) {
		t.Fatalf("read %s: %v", conn.LocalAddr(), err)
	}
}

// wireSelfV6 starts tr on [::1] at the port its constructor chose, sends one
// datagram to [::1], and requires the transport's own socket to receive it.
// Only one socket can hold [::1] at that port, so the transport is its own
// listener: the datagram arrives only when it was addressed to that port.
func wireSelfV6(t *testing.T, tr *transport.UDP) {
	t.Helper()
	loop6 := netip.IPv6Loopback()
	tr.Bind = netip.AddrPortFrom(loop6, tr.Bind.Port())
	if err := tr.Start(); err != nil {
		t.Fatalf("start transport on %s: %v", tr.Bind, err)
	}
	t.Cleanup(func() { tr.Stop() }) //nolint:errcheck // Test cleanup; the transport is discarded.
	if err := tr.Send(transport.Outbound{To: loop6, Mode: tr.Mode, Bytes: []byte("bfd6")}); err != nil {
		t.Fatalf("send over IPv6: %v", err)
	}
	select {
	case in := <-tr.RX():
		if in.From != loop6 {
			t.Fatalf("IPv6 datagram from %s, want %s", in.From, loop6)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("no IPv6 datagram reached [::1]:%d", tr.Bind.Port())
	}
}

// RFC requirement: RFC5881-4-1 positive -- a Control packet sent by the
// single-hop transport newUDPTransport builds is observed on the wire: in IPv4
// it reaches a listener on the peer address at port 3784, and in IPv6 it
// reaches [::1] at port 3784 (the newUDPTransport6 socket's own port).
func TestRFC5881ControlSentToPort3784OnTheWire(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	listener := wireListen(t, 3784)
	tr := newUDPTransport(api.SingleHop, "", "")
	wireStart(t, tr)
	wireSend(t, tr, 1)
	wireSources(t, listener, 1)

	wireSelfV6(t, newUDPTransport6(api.SingleHop, "", ""))
}

// RFC requirement: RFC5881-4-1 negative -- port 3784 is where Control packets
// go, not where every BFD packet goes: a datagram the Echo transport sends
// reaches the peer at 3785, and nothing reaches a listener on the peer at
// 3784.
func TestRFC5881EchoTransportNeverAddressesControlPort(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	control := wireListen(t, 3784)
	echo := wireListen(t, 3785)
	tr := newEchoTransport("", "")
	wireStart(t, tr)
	wireSend(t, tr, 1)
	wireSources(t, echo, 1)
	wireSilent(t, control, "an Echo datagram was addressed to the Control port 3784")
}

// RFC requirement: RFC5881-4-5 positive -- an Echo packet sent by the Echo
// transport newEchoTransport builds reaches a listener on the peer address at
// port 3785, in IPv4.
func TestRFC5881EchoSentToPort3785OnTheWire(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	listener := wireListen(t, 3785)
	tr := newEchoTransport("", "")
	wireStart(t, tr)
	wireSend(t, tr, 1)
	wireSources(t, listener, 1)
}

// RFC requirement: RFC5881-4-5 negative -- port 3785 is where Echo packets go,
// not where every BFD packet goes: a datagram the single-hop Control transport
// sends reaches the peer at 3784, and nothing reaches a listener on the peer at
// 3785.
func TestRFC5881ControlTransportNeverAddressesEchoPort(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	control := wireListen(t, 3784)
	echo := wireListen(t, 3785)
	tr := newUDPTransport(api.SingleHop, "", "")
	wireStart(t, tr)
	wireSend(t, tr, 1)
	wireSources(t, control, 1)
	wireSilent(t, echo, "a Control datagram was addressed to the Echo port 3785")
}

// RFC requirement: RFC5881-4-3 positive -- five Control packets sent through
// the single-hop transport of one session are observed at the peer, and all
// five carry the same UDP source port (and the same source address).
func TestRFC5881ControlSourcePortFixedOnTheWire(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	listener := wireListen(t, 3784)
	tr := newUDPTransport(api.SingleHop, "", "")
	wireStart(t, tr)
	wireSend(t, tr, 5)
	sources := wireSources(t, listener, 5)
	for i, from := range sources[1:] {
		if from != sources[0] {
			t.Fatalf("Control packet %d left from %s, packet 1 from %s: one session, one source port", i+2, from, sources[0])
		}
	}
}

// RFC requirement: RFC5883-5-1 positive -- a Control packet sent by the
// multihop transport newUDPTransport builds is observed on the wire: in IPv4
// it reaches a listener on the peer address at port 4784, and in IPv6 it
// reaches [::1] at port 4784 (the newUDPTransport6 socket's own port).
func TestRFC5883MultiHopControlSentToPort4784OnTheWire(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	listener := wireListen(t, 4784)
	tr := newUDPTransport(api.MultiHop, "", "")
	wireStart(t, tr)
	wireSend(t, tr, 1)
	wireSources(t, listener, 1)

	wireSelfV6(t, newUDPTransport6(api.MultiHop, "", ""))
}

// RFC requirement: RFC5883-5-1 negative -- the single-hop Control transport
// does not address the multihop port: its datagram reaches the peer at 3784,
// and nothing reaches a listener on the peer at 4784.
func TestRFC5883SingleHopControlNeverAddressesPort4784(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	single := wireListen(t, 3784)
	multi := wireListen(t, 4784)
	tr := newUDPTransport(api.SingleHop, "", "")
	wireStart(t, tr)
	wireSend(t, tr, 1)
	wireSources(t, single, 1)
	wireSilent(t, multi, "a single-hop Control datagram was addressed to the multihop port 4784")
}
