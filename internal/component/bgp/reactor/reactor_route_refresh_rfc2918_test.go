// RFC 2918 Section 4 sender obligation: a BGP speaker MUST NOT send a
// ROUTE-REFRESH to a peer unless that peer advertised the Route Refresh
// capability. This is enforced in two independent producers, both exercised here
// against real Peer/Session state (not a mock):
//   - reactorAPIAdapter.sendRouteRefresh (reactor_api_forward.go) -- the low-level
//     send used by explicit route-refresh requests.
//   - reactorAPIAdapter.SoftClearPeer   (reactor_api.go) -- the operator soft-reset
//     entry that emits a ROUTE-REFRESH for every negotiated family.
//
// VALIDATES: the capability gate (negotiated.RouteRefresh) actually decides whether
// a ROUTE-REFRESH reaches the wire, observed on a recordingConn and via the peer's
// RefreshSent counter.
// PREVENTS: regressing the guard so a ROUTE-REFRESH is emitted to a peer that never
// advertised the Route Refresh capability (RFC 2918 Section 4 violation).

package reactor

import (
	"bufio"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/msgtype"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"
)

// rfc-test-change-approved: 2026-07-22 Thomas approved the msgtype/routeaction
// package rename (spec-feature-gate-10-bgp). MessageType/Type* moved to
// internal/core/bgp/msgtype and the route-action enum to
// internal/core/bgp/routeaction so MRT, sysrib and the FIB backends keep
// compiling when the BGP engine is compiled out (//go:build ze_bgp). Every hunk
// in this file is a package-qualifier requalification: no assertion was added,
// removed, reworded, weakened or re-tagged, verified by normalising the diff
// under the renaming and confirming the add/delete multisets cancel.

// newRefreshPeer builds a single Established peer wired to a Session backed by a
// recordingConn (defined in reactor_api_forward_test.go), so a test can observe
// exactly what, if anything, is flushed to the wire. The peer negotiates IPv4
// unicast; routeRefresh controls whether the Route Refresh capability was
// negotiated. The returned adapter has just this peer, so selector.All() matches it.
func newRefreshPeer(t *testing.T, routeRefresh bool) (*reactorAPIAdapter, *Peer, *recordingConn) {
	t.Helper()
	settings := &PeerSettings{
		Connection: ConnectionBoth,
		Address:    netip.MustParseAddr("10.0.0.2"),
		LocalAS:    65000,
		PeerAS:     65001,
		RouterID:   0x01020301,
	}
	peer := NewPeer(settings)
	peer.state.Store(int32(PeerStateEstablished))
	peer.negotiated.Store(&NegotiatedCapabilities{
		families:     map[family.Family]bool{{AFI: family.AFIIPv4, SAFI: family.SAFIUnicast}: true},
		RouteRefresh: routeRefresh,
	})

	// Drive a Session to Established, then back it with the recording conn.
	session := NewSession(settings)
	require.NoError(t, session.fsm.Event(fsm.EventManualStart))
	require.NoError(t, session.fsm.Event(fsm.EventTCPConnectionConfirmed))
	require.NoError(t, session.fsm.Event(fsm.EventBGPOpen))
	require.NoError(t, session.fsm.Event(fsm.EventKeepaliveMsg))
	require.Equal(t, fsm.StateEstablished, session.fsm.State())

	conn := &recordingConn{}
	session.mu.Lock()
	session.conn = conn
	session.bufWriter = bufio.NewWriterSize(conn, 4096)
	session.mu.Unlock()

	peer.mu.Lock()
	peer.session = session
	peer.mu.Unlock()

	adapter := &reactorAPIAdapter{r: &Reactor{
		config: &Config{LocalAS: 65000},
		peers:  map[netip.AddrPort]*Peer{settings.PeerKey(): peer},
	}}
	return adapter, peer, conn
}

// assertRouteRefreshOnWire checks that written is exactly one ROUTE-REFRESH
// message (type 5) carrying the given family.
func assertRouteRefreshOnWire(t *testing.T, written []byte, afi family.AFI, safi family.SAFI) {
	t.Helper()
	require.Len(t, written, message.HeaderLen+4, "a ROUTE-REFRESH must reach the wire")
	h, err := message.ParseHeader(written)
	require.NoError(t, err)
	require.Equal(t, msgtype.TypeROUTEREFRESH, h.Type)
	rr, err := message.UnpackRouteRefresh(written[message.HeaderLen:])
	require.NoError(t, err)
	require.Equal(t, afi, rr.AFI)
	require.Equal(t, safi, rr.SAFI)
}

// RFC requirement: RFC2918-4-1 positive -- a BGP speaker sends a ROUTE-REFRESH to
// a peer that advertised the Route Refresh capability (negotiated.RouteRefresh ==
// true): sendRouteRefresh writes the message to the wire and records the send via
// IncrRefreshSent (reactor_api_forward.go sendRouteRefresh, the neg.RouteRefresh
// gate and the SendRawMessage/IncrRefreshSent path).
func TestRFC2918SendRouteRefreshToCapablePeer(t *testing.T) {
	adapter, peer, conn := newRefreshPeer(t, true)

	err := adapter.sendRouteRefresh(
		selector.All(),
		uint16(family.AFIIPv4),
		uint8(family.SAFIUnicast),
		message.RouteRefreshNormal,
	)
	require.NoError(t, err)

	assertRouteRefreshOnWire(t, conn.written(), family.AFIIPv4, family.SAFIUnicast)
	require.Equal(t, uint32(1), peer.Stats().RefreshSent, "IncrRefreshSent must record the send")
}

// RFC requirement: RFC2918-4-1 negative -- a BGP speaker MUST NOT send a
// ROUTE-REFRESH to a peer that did not advertise the Route Refresh capability. The
// gate skips the peer both when the capability was explicitly not negotiated
// (RouteRefresh == false) and when no capabilities were negotiated at all
// (negotiated == nil): nothing reaches the wire and RefreshSent stays 0
// (reactor_api_forward.go sendRouteRefresh: `if neg == nil || !neg.RouteRefresh`).
func TestRFC2918SendRouteRefreshSkipsPeerWithoutCapability(t *testing.T) {
	t.Run("capability_not_negotiated", func(t *testing.T) {
		adapter, peer, conn := newRefreshPeer(t, false)

		err := adapter.sendRouteRefresh(
			selector.All(),
			uint16(family.AFIIPv4),
			uint8(family.SAFIUnicast),
			message.RouteRefreshNormal,
		)
		require.NoError(t, err)

		require.Empty(t, conn.written(),
			"no ROUTE-REFRESH may reach a peer that never advertised the capability")
		require.Equal(t, uint32(0), peer.Stats().RefreshSent)
	})

	t.Run("no_negotiated_capabilities", func(t *testing.T) {
		adapter, peer, conn := newRefreshPeer(t, false)
		peer.negotiated.Store(nil)

		err := adapter.sendRouteRefresh(
			selector.All(),
			uint16(family.AFIIPv4),
			uint8(family.SAFIUnicast),
			message.RouteRefreshNormal,
		)
		require.NoError(t, err)

		require.Empty(t, conn.written(),
			"no ROUTE-REFRESH may reach a peer with no negotiated capabilities")
		require.Equal(t, uint32(0), peer.Stats().RefreshSent)
	})
}

// RFC requirement: RFC2918-4-1 positive -- SoftClearPeer (the operator soft-reset
// entry) sends a ROUTE-REFRESH for each negotiated family only to a peer that
// advertised the Route Refresh capability, and reports the family it cleared
// (reactor_api.go SoftClearPeer, past the neg.RouteRefresh gate).
func TestRFC2918SoftClearPeerSendsRefreshToCapablePeer(t *testing.T) {
	adapter, _, conn := newRefreshPeer(t, true)

	families, err := adapter.SoftClearPeer(selector.All())
	require.NoError(t, err)
	require.Contains(t, families,
		family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIUnicast}.String())

	assertRouteRefreshOnWire(t, conn.written(), family.AFIIPv4, family.SAFIUnicast)
}

// RFC requirement: RFC2918-4-1 negative -- SoftClearPeer skips a peer that did not
// advertise the Route Refresh capability: no ROUTE-REFRESH is sent and no family
// is reported cleared (reactor_api.go SoftClearPeer: `if !neg.RouteRefresh`).
func TestRFC2918SoftClearPeerSkipsPeerWithoutCapability(t *testing.T) {
	adapter, _, conn := newRefreshPeer(t, false)

	families, err := adapter.SoftClearPeer(selector.All())
	require.NoError(t, err)
	require.Empty(t, families, "an incapable peer contributes no cleared families")
	require.Empty(t, conn.written(),
		"SoftClearPeer must not send ROUTE-REFRESH to an incapable peer")
}
