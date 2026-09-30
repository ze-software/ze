package reactor

import (
	"bytes"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"
)

// refreshMarkerWire is the ROUTE-REFRESH ze writes for one Enhanced Route
// Refresh marker on IPv4 unicast.
func refreshMarkerWire(subtype message.RouteRefreshSubtype) []byte {
	return message.PackTo(&message.RouteRefresh{AFI: family.AFIIPv4, SAFI: family.SAFIUnicast, Subtype: subtype}, nil)
}

// refreshWireLabels names each message on the wire: "borr", "eorr", "eor" (the
// IPv4 unicast End-of-RIB), "route" (any other UPDATE), or "other".
func refreshWireLabels(t *testing.T, wire []byte) []string {
	t.Helper()
	borr := refreshMarkerWire(message.RouteRefreshBoRR)
	eorr := refreshMarkerWire(message.RouteRefreshEoRR)
	eor := eorWire(family.IPv4Unicast)
	var labels []string
	for _, msg := range splitWireMessages(t, wire) {
		switch {
		case bytes.Equal(msg, borr):
			labels = append(labels, "borr")
		case bytes.Equal(msg, eorr):
			labels = append(labels, "eorr")
		case bytes.Equal(msg, eor):
			labels = append(labels, "eor")
		case msg[18] == 2: // BGP header octet 18 is the type; 2 is UPDATE.
			labels = append(labels, "route")
		default:
			labels = append(labels, "other")
		}
	}
	return labels
}

// answerRefresh drives the three calls the rib makes to answer a peer's
// ROUTE-REFRESH (RIBManager.handleRefresh): the BoRR, the Adj-RIB-Out's one
// route, then the EoRR, each through the reactor API entry point it uses.
func answerRefresh(t *testing.T, api *reactorAPIAdapter) {
	t.Helper()
	all := selector.All()
	operator := plugin.OperatorSender()
	afi, safi := uint16(family.AFIIPv4), uint8(family.SAFIUnicast)
	route := bgptypes.NLRIBatch{
		Family:  family.IPv4Unicast,
		NLRIs:   []nlri.NLRI{nlri.NewINET(family.IPv4Unicast, netip.MustParsePrefix("192.0.2.0/24"), 0)},
		NextHop: bgptypes.NewNextHopExplicit(netip.MustParseAddr("10.0.0.1")),
	}
	require.NoError(t, api.SendBoRR(all, afi, safi, operator))
	require.NoError(t, api.AnnounceNLRIBatch(t.Context(), all, route, operator))
	require.NoError(t, api.SendEoRR(all, afi, safi, operator))
}

// TestRFC7313EoRRFollowsTheRoutesItCloses answers a peer's route refresh through
// the reactor API on a Peer whose session writes to a recording socket, and
// reads the order of the messages that reached it.
//
// VALIDATES: outside the initial sync the wire holds BoRR, the route, EoRR, in
// that order. During the initial sync, where the route waits in the operation
// queue, nothing leaves until the queue drains, and then the wire holds BoRR, the
// route, EoRR, and the sync's End-of-RIB; each marker is counted once it is sent.
// PREVENTS: the EoRR leaving at once while the route it closes still waits in the
// queue, which makes the peer purge that route as stale (RFC 7313 Section 4).
//
// RFC requirement: RFC7313-4-2 positive -- with no initial sync running, the EoRR is written
// after the refresh's route, and the BoRR before it.
// RFC requirement: RFC7313-4-2 negative -- a refresh answered while the initial sync holds its
// route in the queue: no EoRR is written before that route, and it follows the route.
func TestRFC7313EoRRFollowsTheRoutesItCloses(t *testing.T) {
	enhanced := func(t *testing.T) (*Peer, *recordingConn) {
		t.Helper()
		peer, conn := newInitialSyncPeer(t, true, family.IPv4Unicast)
		nc := peer.negotiated.Load()
		nc.RouteRefresh = true
		nc.EnhancedRouteRefresh = true
		return peer, conn
	}

	t.Run("outside the initial sync", func(t *testing.T) {
		peer, conn := enhanced(t)
		peer.sendInitialRoutes()
		require.False(t, peer.shouldQueue(), "the sync is over, so routes go straight out")

		answerRefresh(t, newSendPermissionReactor(peer))
		assert.Equal(t, []string{"eor", "borr", "route", "eorr"}, refreshWireLabels(t, conn.written()))
		assert.Equal(t, uint32(2), peer.Stats().RefreshSent, "both markers reached the socket")
	})

	t.Run("during the initial sync", func(t *testing.T) {
		peer, conn := enhanced(t)
		require.True(t, peer.shouldQueue(), "the sync holds the queueing gate")

		answerRefresh(t, newSendPermissionReactor(peer))
		assert.Empty(t, refreshWireLabels(t, conn.written()),
			"no marker may leave ahead of the route the queue still holds")
		assert.Equal(t, uint32(0), peer.Stats().RefreshSent, "a queued marker has not been sent")

		peer.sendInitialRoutes()
		assert.Equal(t, []string{"borr", "route", "eorr", "eor"}, refreshWireLabels(t, conn.written()),
			"the queue drains in the order the refresh was answered")
		assert.Equal(t, uint32(2), peer.Stats().RefreshSent, "both markers reached the socket")
	})
}
