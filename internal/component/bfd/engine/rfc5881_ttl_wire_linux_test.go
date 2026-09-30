//go:build linux

// VALIDATES: RFC 5881 Section 5 from the side of a sender that breaks it. A
// peer sends single-hop Control packets with TTL 254 (IPv4) or Hop Limit 254
// (IPv6), without and with authentication, over the wire to a Loop running on
// Ze's real single-hop UDP transport. Ze reads the received TTL off the IP
// header (IP_RECVTTL, IPV6_RECVHOPLIMIT) and its receive path discards the
// packets: the session installs no bfd.RemoteDiscr and stays Down. The same
// packet sent with 255 moves the session to Init, so the discard is the TTL's
// doing and not a packet lost on the way.
// PREVENTS: a TTL gate that holds against a hand-built transport.Inbound while
// the TTL the kernel reports never reaches it.
//
// Each test binds port 3784, so it runs in a user and network namespace of its
// own (userns.Enter), where neither a ze on the host nor a parallel run holds
// the port.
package engine

import (
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/bfd/api"
	"github.com/ze-software/ze/internal/component/bfd/auth"
	"github.com/ze-software/ze/internal/component/bfd/packet"
	"github.com/ze-software/ze/internal/component/bfd/transport"
	"github.com/ze-software/ze/internal/core/clock"
	"github.com/ze-software/ze/internal/test/userns"
)

// The addresses of the wire TTL tests. Ze and the peer take one address each,
// so Ze's own transmissions, addressed to the peer at 3784, never come back to
// Ze's socket. The IPv6 pair is added to lo by rfc5881WireAddrs.
var (
	ttlWireZe4   = netip.MustParseAddr("127.0.0.1")
	ttlWirePeer4 = netip.MustParseAddr("127.0.0.2")
	ttlWireZe6   = netip.MustParseAddr("fd00:5881::1")
	ttlWirePeer6 = netip.MustParseAddr("fd00:5881::2")
)

// ttlWirePeerPort is the peer's source port. The session key holds no port.
const ttlWirePeerPort = 49152

// rfc5881WireAddrs puts the IPv6 pair on lo with duplicate address detection
// off, so both addresses can be bound at once.
func rfc5881WireAddrs(t *testing.T) {
	t.Helper()
	lo, err := netlink.LinkByName("lo")
	if err != nil {
		t.Fatalf("lo lookup: %v", err)
	}
	for _, a := range []netip.Addr{ttlWireZe6, ttlWirePeer6} {
		addr := &netlink.Addr{IPNet: &net.IPNet{IP: a.AsSlice(), Mask: net.CIDRMask(128, 128)}, Flags: unix.IFA_F_NODAD}
		if err := netlink.AddrAdd(lo, addr); err != nil {
			t.Fatalf("add %s to lo: %v", a, err)
		}
	}
}

// rfc5881WireLoop starts a Loop over a single-hop UDP transport bound to
// local at 3784, holding one session for peer on lo. A nil settings leaves
// the session unauthenticated.
func rfc5881WireLoop(t *testing.T, peer, local netip.Addr, settings *api.AuthSettings) (*Loop, api.Key) {
	t.Helper()
	tr := &transport.UDP{Bind: netip.AddrPortFrom(local, transport.UDPPortSingleHopControl), Mode: api.SingleHop}
	l := NewLoop(tr, clock.RealClock{})
	req := api.SessionRequest{
		Peer:                  peer,
		Local:                 local,
		Interface:             "lo",
		Mode:                  api.SingleHop,
		DesiredMinTxInterval:  300_000,
		RequiredMinRxInterval: 300_000,
		DetectMult:            3,
		Auth:                  settings,
	}
	if _, err := l.EnsureSession(req); err != nil {
		t.Fatalf("EnsureSession: %v", err)
	}
	if err := l.Start(); err != nil {
		t.Fatalf("start loop on %s: %v", tr.Bind, err)
	}
	t.Cleanup(func() { l.Stop() }) //nolint:errcheck // Test cleanup; the loop is discarded.
	return l, req.Key()
}

// rfc5881PeerSocket opens the peer's socket on peer at ttlWirePeerPort.
func rfc5881PeerSocket(t *testing.T, peer netip.Addr) *net.UDPConn {
	t.Helper()
	network := "udp4"
	if peer.Is6() {
		network = "udp6"
	}
	conn, err := net.ListenUDP(network, net.UDPAddrFromAddrPort(netip.AddrPortFrom(peer, ttlWirePeerPort)))
	if err != nil {
		t.Fatalf("listen %s: %v", peer, err)
	}
	t.Cleanup(func() { conn.Close() }) //nolint:errcheck // Test cleanup; the socket is discarded.
	return conn
}

// rfc5881PeerHops sets the TTL (IPv4) or unicast Hop Limit (IPv6) the peer's
// datagrams leave with.
func rfc5881PeerHops(t *testing.T, conn *net.UDPConn, v6 bool, hops int) {
	t.Helper()
	raw, err := conn.SyscallConn()
	if err != nil {
		t.Fatalf("SyscallConn: %v", err)
	}
	var optErr error
	err = raw.Control(func(fd uintptr) {
		if v6 {
			optErr = unix.SetsockoptInt(int(fd), unix.IPPROTO_IPV6, unix.IPV6_UNICAST_HOPS, hops)
			return
		}
		optErr = unix.SetsockoptInt(int(fd), unix.IPPROTO_IP, unix.IP_TTL, hops)
	})
	if err != nil {
		t.Fatalf("Control: %v", err)
	}
	if optErr != nil {
		t.Fatalf("set peer hops %d: %v", hops, optErr)
	}
}

// rfc5881PeerSend sends the peer's first Control packet (state Down, Your
// Discriminator 0) to Ze at 3784, signed with sequence number seq when signer
// is not nil.
func rfc5881PeerSend(t *testing.T, conn *net.UDPConn, ze netip.Addr, signer auth.Signer, seq uint32) {
	t.Helper()
	c := packet.Control{
		Version:               packet.Version,
		State:                 packet.StateDown,
		DetectMult:            3,
		Length:                packet.MandatoryLen,
		MyDiscriminator:       peerMyDiscr,
		DesiredMinTxInterval:  300_000,
		RequiredMinRxInterval: 300_000,
	}
	if signer != nil {
		c.Auth = true
		c.Length = uint8(packet.MandatoryLen + signer.BodyLen())
	}
	buf := make([]byte, int(c.Length))
	c.WriteTo(buf, 0)
	if signer != nil {
		signer.Sign(buf, packet.MandatoryLen, seq)
	}
	to := net.UDPAddrFromAddrPort(netip.AddrPortFrom(ze, transport.UDPPortSingleHopControl))
	if _, err := conn.WriteToUDP(buf, to); err != nil {
		t.Fatalf("peer send to %s: %v", to, err)
	}
}

// rfc5881Observed reads bfd.RemoteDiscr and bfd.SessionState under the loop
// lock, which the loop's receive goroutine holds while it delivers a packet.
func rfc5881Observed(t *testing.T, l *Loop, key api.Key) (uint32, packet.State) {
	t.Helper()
	l.mu.Lock()
	defer l.mu.Unlock()
	e := l.sessions[key]
	if e == nil {
		t.Fatalf("no session registered for key %+v", key)
	}
	return e.machine.RemoteDiscriminator(), e.machine.State()
}

// rfc5881WireDiscard runs one family and one authentication setting: three
// packets at 254 leave the session Down with no bfd.RemoteDiscr, then one at
// 255 installs it and moves the session to Init.
func rfc5881WireDiscard(t *testing.T, peer, ze netip.Addr, authType uint8) {
	t.Helper()
	var (
		settings *api.AuthSettings
		signer   auth.Signer
	)
	if authType != 0 {
		settings = &api.AuthSettings{Type: authType, KeyID: 5, Secret: rfc5880Secret}
		var err error
		signer, err = auth.NewSigner(auth.Settings{Type: authType, KeyID: 5, Secret: rfc5880Secret})
		if err != nil {
			t.Fatalf("NewSigner: %v", err)
		}
	}
	l, key := rfc5881WireLoop(t, peer, ze, settings)
	conn := rfc5881PeerSocket(t, peer)

	rfc5881PeerHops(t, conn, peer.Is6(), 254)
	for seq := range uint32(3) {
		rfc5881PeerSend(t, conn, ze, signer, seq+1)
	}
	time.Sleep(300 * time.Millisecond)
	remote, state := rfc5881Observed(t, l, key)
	if remote != 0 {
		t.Fatalf("%s, auth type %d: a packet sent with 254 installed bfd.RemoteDiscr %d; it must be discarded", peer, authType, remote)
	}
	if state != packet.StateDown {
		t.Fatalf("%s, auth type %d: a packet sent with 254 moved the session to %s; it must be discarded", peer, authType, state)
	}

	rfc5881PeerHops(t, conn, peer.Is6(), 255)
	rfc5881PeerSend(t, conn, ze, signer, 4)
	deadline := time.Now().Add(2 * time.Second)
	for {
		remote, state = rfc5881Observed(t, l, key)
		if remote == peerMyDiscr {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s, auth type %d: the same packet sent with 255 never reached the session (RemoteDiscr %d, %s); the discard proves nothing", peer, authType, remote, state)
		}
		time.Sleep(5 * time.Millisecond)
	}
	if state != packet.StateInit {
		t.Fatalf("%s, auth type %d: state after a packet sent with 255 = %s, want Init", peer, authType, state)
	}
}

// RFC requirement: RFC5881-5-1 negative -- a Control packet without
// authentication that a peer sends in breach of this rule, with TTL 254 in
// IPv4 or Hop Limit 254 in IPv6, crosses the wire to Ze's single-hop UDP
// transport and is discarded: the session installs no bfd.RemoteDiscr and stays
// Down, where the same packet sent with 255 moves it to Init.
func TestRFC5881UnauthenticatedControlBelow255IsDiscardedOffTheWire(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	rfc5881WireAddrs(t)
	rfc5881WireDiscard(t, ttlWirePeer4, ttlWireZe4, 0)
	rfc5881WireDiscard(t, ttlWirePeer6, ttlWireZe6, 0)
}

// RFC requirement: RFC5881-5-3 negative -- a Control packet with the A bit set
// and a valid Keyed SHA1 section that a peer sends in breach of this rule, with
// TTL 254 in IPv4 or Hop Limit 254 in IPv6, crosses the wire to Ze's
// single-hop UDP transport for an authenticated session and is discarded: the
// session installs no bfd.RemoteDiscr and stays Down, where the same signed
// packet sent with 255 moves it to Init.
func TestRFC5881AuthenticatedControlBelow255IsDiscardedOffTheWire(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	rfc5881WireAddrs(t)
	rfc5881WireDiscard(t, ttlWirePeer4, ttlWireZe4, packet.AuthTypeKeyedSHA1)
	rfc5881WireDiscard(t, ttlWirePeer6, ttlWireZe6, packet.AuthTypeKeyedSHA1)
}
