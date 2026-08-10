package reactor

import (
	"bufio"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
)

// VALIDATES: Peer Send methods return ErrNotConnected when no session is active.
// PREVENTS: Nil pointer panics on send to disconnected peer.

func newTestPeer() *Peer {
	settings := NewPeerSettings(mustParseAddr("192.0.2.1"), 65000, 65001, 0x01010101)
	return NewPeer(settings)
}

// TestPeerSendUpdate_NoSession verifies SendUpdate returns ErrNotConnected.
func TestPeerSendUpdate_NoSession(t *testing.T) {
	peer := newTestPeer()
	err := peer.SendUpdate(nil)
	require.ErrorIs(t, err, ErrNotConnected)
}

// TestPeerSendAnnounce_NoSession verifies SendAnnounce returns ErrNotConnected.
func TestPeerSendAnnounce_NoSession(t *testing.T) {
	peer := newTestPeer()
	route := bgptypes.RouteSpec{
		Prefix:  netip.MustParsePrefix("10.0.0.0/24"),
		NextHop: bgptypes.NewNextHopExplicit(netip.MustParseAddr("1.1.1.1")),
	}
	err := peer.SendAnnounce(route, 65000)
	require.ErrorIs(t, err, ErrNotConnected)
}

// TestPeerSendWithdraw_NoSession verifies SendWithdraw returns ErrNotConnected.
func TestPeerSendWithdraw_NoSession(t *testing.T) {
	peer := newTestPeer()
	err := peer.sendWithdraw(netip.MustParsePrefix("10.0.0.0/24"))
	require.ErrorIs(t, err, ErrNotConnected)
}

// TestPeerSendRawUpdateBody_NoSession verifies SendRawUpdateBody returns ErrNotConnected.
func TestPeerSendRawUpdateBody_NoSession(t *testing.T) {
	peer := newTestPeer()
	err := peer.sendRawUpdateBody([]byte{0x00, 0x00, 0x00, 0x00})
	require.ErrorIs(t, err, ErrNotConnected)
}

// TestPeerSendRawMessage_NoSession verifies SendRawMessage returns ErrNotConnected.
func TestPeerSendRawMessage_NoSession(t *testing.T) {
	peer := newTestPeer()
	err := peer.SendRawMessage(2, []byte{0x00})
	require.ErrorIs(t, err, ErrNotConnected)
}

// TestPeerSendAnnounce_IPv6NoSession verifies IPv6 path also returns ErrNotConnected.
func TestPeerSendAnnounce_IPv6NoSession(t *testing.T) {
	peer := newTestPeer()
	route := bgptypes.RouteSpec{
		Prefix:  netip.MustParsePrefix("2001:db8::/32"),
		NextHop: bgptypes.NewNextHopExplicit(netip.MustParseAddr("::1")),
	}
	err := peer.SendAnnounce(route, 65000)
	require.ErrorIs(t, err, ErrNotConnected)
}

// TestPeerSendWithdraw_IPv6NoSession verifies IPv6 withdrawal also returns ErrNotConnected.
func TestPeerSendWithdraw_IPv6NoSession(t *testing.T) {
	peer := newTestPeer()
	err := peer.sendWithdraw(netip.MustParsePrefix("2001:db8::/32"))
	require.ErrorIs(t, err, ErrNotConnected)
}

// TestPeerAcceptConnection_NoSession verifies AcceptConnection returns ErrNotConnected.
func TestPeerAcceptConnection_NoSession(t *testing.T) {
	peer := newTestPeer()
	err := peer.AcceptConnection(nil)
	require.ErrorIs(t, err, ErrNotConnected)
}

// TestPeerSessionState_NoSession verifies SessionState returns StateIdle without session.
func TestPeerSessionState_NoSession(t *testing.T) {
	peer := newTestPeer()
	assert.Equal(t, fsm.StateIdle, peer.SessionState())
}

// newAnnouncePeer returns a peer with an Established session recording what
// reaches the wire, for the Peer.SendAnnounce rail.
//
// peerAddr decides the peer half of RFC 2545 Section 3's condition.
func newAnnouncePeer(t *testing.T, peerAddr string) (*Peer, *recordingConn) {
	t.Helper()
	settings := &PeerSettings{
		Connection:   ConnectionBoth,
		Address:      netip.MustParseAddr(peerAddr),
		LocalAddress: netip.MustParseAddr("::1"),
		LinkLocal:    netip.MustParseAddr("fe80::1"),
		LocalAS:      65000,
		PeerAS:       65001,
		RouterID:     0x01020301,
	}
	peer := NewPeer(settings)
	peer.state.Store(int32(PeerStateEstablished))

	session := NewSession(settings)
	require.NoError(t, session.fsm.Event(fsm.EventManualStart))
	require.NoError(t, session.fsm.Event(fsm.EventTCPConnectionConfirmed))
	require.NoError(t, session.fsm.Event(fsm.EventBGPOpen))
	require.NoError(t, session.fsm.Event(fsm.EventKeepaliveMsg))

	conn := &recordingConn{}
	session.mu.Lock()
	session.conn = conn
	session.bufWriter = bufio.NewWriterSize(conn, 4096)
	session.mu.Unlock()

	peer.mu.Lock()
	peer.session = session
	peer.mu.Unlock()

	// setEncodingContexts (peer.go) refreshes the link scope at establishment.
	peer.refreshLinkScope()
	return peer, conn
}

// TestSendAnnounceAppendsLinkLocalWhenSection3Holds drives RFC 2545 Section 3
// through the single-route announce rail (Peer.SendAnnounce -> Session.SendAnnounce
// -> WriteAnnounceUpdate), from the entry point to the socket.
//
// RFC requirement: RFC2545-3-1 positive -- the Next Hop field carries the global
// IPv6 address of the next hop followed by the link-local IPv6 address of the next
// hop.
//
// RFC requirement: RFC2545-3-2 positive -- the Length of Next Hop Network Address
// octet is 32 (0x20) because a link-local address is also included.
//
// RFC requirement: RFC2545-3-3 positive -- both halves of the condition hold: the
// speaker shares the loopback subnet with the entity named by the global next hop
// (::1) and with the peer the route is advertised to (::1).
//
// VALIDATES: this rail emits the 32-octet form. Before this it hardcoded a next-hop
// length of 16 and could not encode the second address at all.
// PREVENTS: an announce leaving with the 16-octet form in a case Section 3 requires
// the link-local address.
func TestSendAnnounceAppendsLinkLocalWhenSection3Holds(t *testing.T) {
	peer, conn := newAnnouncePeer(t, "::1")
	route := bgptypes.RouteSpec{
		Prefix:  netip.MustParsePrefix("2001:db8:1::/64"),
		NextHop: bgptypes.NewNextHopExplicit(netip.MustParseAddr("::1")),
	}

	require.NoError(t, peer.SendAnnounce(route, 65000))

	nlriBytes := []byte{0x40, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x01, 0x00, 0x00}
	assert.Contains(t, string(conn.written()), string(mpReachIPv6Attr(t, nlriBytes, "::1", "fe80::1")),
		"RFC 2545 Section 3: global address first, link-local second, length octet 0x20")
}

// TestSendAnnounceOmitsLinkLocalWhenPeerOffLink is the other polarity.
//
// RFC requirement: RFC2545-3-3 negative -- the peer half of the condition fails
// (2001:db8:dead:beef::2 sits on no locally connected subnet), so the link-local
// address is NOT included even though the leaf names one.
//
// RFC requirement: RFC2545-3-4 positive -- "in all other cases" the speaker
// advertises only the global IPv6 address of the next hop and sets the length
// octet to 16.
//
// VALIDATES: the same rail emits the 16-octet form when Section 3 excludes the
// second address. Without this row an encoder that always appended would pass the
// positive test above.
func TestSendAnnounceOmitsLinkLocalWhenPeerOffLink(t *testing.T) {
	peer, conn := newAnnouncePeer(t, "2001:db8:dead:beef::2")
	route := bgptypes.RouteSpec{
		Prefix:  netip.MustParsePrefix("2001:db8:1::/64"),
		NextHop: bgptypes.NewNextHopExplicit(netip.MustParseAddr("::1")),
	}

	require.NoError(t, peer.SendAnnounce(route, 65000))

	nlriBytes := []byte{0x40, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x01, 0x00, 0x00}
	written := string(conn.written())
	assert.Contains(t, written, string(mpReachIPv6Attr(t, nlriBytes, "::1")),
		"RFC 2545 Section 3: the global address alone, length octet 0x10")
	assert.NotContains(t, written, string(mpReachIPv6Attr(t, nlriBytes, "::1", "fe80::1")),
		"no link-local may be appended when the peer shares no subnet with the speaker")
}
