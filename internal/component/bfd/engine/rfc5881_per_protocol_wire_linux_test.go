//go:build linux

// VALIDATES: RFC 5881 Section 2 on the wire: an IPv4 session and an IPv6
// session to the same system over one link are two sessions, and each one's
// Control packets are encapsulated in its own protocol. One Loop runs over a
// transport.Dual whose IPv4 and IPv6 sockets are real, and the peer listens on
// an IPv4 and an IPv6 socket at 3784.
// PREVENTS: a session's Control packets leaving in the other protocol's
// encapsulation, which is one session carried over both protocols.
//
// The test adds addresses to lo, so it runs in a user and network namespace
// of its own (userns.Enter).
package engine

import (
	"encoding/binary"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bfd/api"
	"github.com/ze-software/ze/internal/component/bfd/transport"
	"github.com/ze-software/ze/internal/core/clock"
	"github.com/ze-software/ze/internal/test/userns"
)

// perProtocolSeen collects, per peer socket, the My Discriminator values of
// the Control packets it received within the window.
func perProtocolSeen(t *testing.T, conn *net.UDPConn, window time.Duration) map[uint32]bool {
	t.Helper()
	seen := make(map[uint32]bool)
	buf := make([]byte, 128)
	deadline := time.Now().Add(window)
	for time.Now().Before(deadline) {
		if err := conn.SetReadDeadline(deadline); err != nil {
			t.Fatalf("read deadline: %v", err)
		}
		n, err := conn.Read(buf)
		if err != nil {
			break
		}
		if n < 24 {
			continue
		}
		seen[binary.BigEndian.Uint32(buf[4:8])] = true
	}
	return seen
}

// perProtocolPeer opens a peer socket at 3784 on addr.
func perProtocolPeer(t *testing.T, addr netip.Addr) *net.UDPConn {
	t.Helper()
	network := "udp4"
	if addr.Is6() {
		network = "udp6"
	}
	conn, err := net.ListenUDP(network, net.UDPAddrFromAddrPort(netip.AddrPortFrom(addr, transport.UDPPortSingleHopControl)))
	if err != nil {
		t.Fatalf("listen %s: %v", addr, err)
	}
	t.Cleanup(func() { conn.Close() }) //nolint:errcheck // Test cleanup; the socket is discarded.
	return conn
}

// RFC requirement: RFC5881-2-2 positive -- an IPv4 session and an IPv6
// session to the peer over lo run as two sessions with distinct
// discriminators; the peer's IPv4 socket receives the IPv4 session's Control
// packets and never the IPv6 session's, and the peer's IPv6 socket receives
// the IPv6 session's and never the IPv4 session's.
func TestRFC5881EachProtocolSessionEncapsulatedInItsProtocol(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	rfc5881WireAddrs(t)
	peer4 := perProtocolPeer(t, ttlWirePeer4)
	peer6 := perProtocolPeer(t, ttlWirePeer6)
	dual := &transport.Dual{
		V4: &transport.UDP{Bind: netip.AddrPortFrom(ttlWireZe4, transport.UDPPortSingleHopControl), Mode: api.SingleHop},
		V6: &transport.UDP{Bind: netip.AddrPortFrom(ttlWireZe6, transport.UDPPortSingleHopControl), Mode: api.SingleHop},
	}
	l := NewLoop(dual, clock.RealClock{})
	discr := make(map[string]uint32, 2)
	for _, pair := range [][2]netip.Addr{{ttlWirePeer4, ttlWireZe4}, {ttlWirePeer6, ttlWireZe6}} {
		req := api.SessionRequest{
			Peer: pair[0], Local: pair[1], Interface: "lo", Mode: api.SingleHop,
			DesiredMinTxInterval: 100_000, RequiredMinRxInterval: 100_000, DetectMult: 3,
		}
		if _, err := l.EnsureSession(req); err != nil {
			t.Fatalf("EnsureSession %s: %v", pair[0], err)
		}
		l.mu.Lock()
		discr[pair[0].String()] = l.sessions[req.Key()].machine.LocalDiscriminator()
		l.mu.Unlock()
	}
	d4, d6 := discr[ttlWirePeer4.String()], discr[ttlWirePeer6.String()]
	if d4 == d6 {
		t.Fatalf("the IPv4 and IPv6 sessions share discriminator %d", d4)
	}
	if err := l.Start(); err != nil {
		t.Fatalf("start loop: %v", err)
	}
	t.Cleanup(func() { l.Stop() }) //nolint:errcheck // Test cleanup; the loop is discarded.

	seen4 := perProtocolSeen(t, peer4, 2500*time.Millisecond)
	seen6 := perProtocolSeen(t, peer6, 500*time.Millisecond)
	if !seen4[d4] {
		t.Fatalf("the peer's IPv4 socket received no Control packet of the IPv4 session (discriminator %d); saw %v", d4, seen4)
	}
	if !seen6[d6] {
		t.Fatalf("the peer's IPv6 socket received no Control packet of the IPv6 session (discriminator %d); saw %v", d6, seen6)
	}
	if seen4[d6] {
		t.Fatalf("the IPv6 session (discriminator %d) sent a Control packet encapsulated in IPv4", d6)
	}
	if seen6[d4] {
		t.Fatalf("the IPv4 session (discriminator %d) sent a Control packet encapsulated in IPv6", d4)
	}
}
