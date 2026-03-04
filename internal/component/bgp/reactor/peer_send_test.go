package reactor

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/fsm"
	bgptypes "codeberg.org/thomas-mangin/ze/internal/component/bgp/types"
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
	err := peer.SendWithdraw(netip.MustParsePrefix("10.0.0.0/24"))
	require.ErrorIs(t, err, ErrNotConnected)
}

// TestPeerSendRawUpdateBody_NoSession verifies SendRawUpdateBody returns ErrNotConnected.
func TestPeerSendRawUpdateBody_NoSession(t *testing.T) {
	peer := newTestPeer()
	err := peer.SendRawUpdateBody([]byte{0x00, 0x00, 0x00, 0x00})
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
	err := peer.SendWithdraw(netip.MustParsePrefix("2001:db8::/32"))
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
