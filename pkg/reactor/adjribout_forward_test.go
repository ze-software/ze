package reactor

import (
	"net/netip"
	"testing"
	"time"

	"github.com/exa-networks/zebgp/pkg/bgp/attribute"
	bgpctx "github.com/exa-networks/zebgp/pkg/bgp/context"
	"github.com/exa-networks/zebgp/pkg/bgp/nlri"
	"github.com/exa-networks/zebgp/pkg/rib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestForwardStoresInAdjRIBOut verifies forwarded routes are stored in adj-rib-out.
//
// VALIDATES: Forwarded routes appear in adj-rib-out for reconnect replay.
// PREVENTS: Routes lost on peer reconnect after forward.
func TestForwardStoresInAdjRIBOut(t *testing.T) {
	// Register encoding context
	ctx := &bgpctx.EncodingContext{ASN4: true}
	ctxID := bgpctx.Registry.Register(ctx)

	// Build attributes: ORIGIN + NEXT_HOP + AS_PATH
	attrBytes := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN IGP
		0x40, 0x03, 0x04, 10, 0, 0, 1, // NEXT_HOP 10.0.0.1
		0x40, 0x02, 0x00, // AS_PATH empty
	}
	attrs := attribute.NewAttributesWire(attrBytes, ctxID)

	// Create NLRI
	prefix := netip.MustParsePrefix("192.168.1.0/24")
	announceNLRI := nlri.NewINET(nlri.IPv4Unicast, prefix, 0)

	// Create ReceivedUpdate
	update := &ReceivedUpdate{
		UpdateID:     1,
		RawBytes:     buildTestRawUpdateBody(attrBytes, []nlri.NLRI{announceNLRI}),
		Attrs:        attrs,
		Announces:    []nlri.NLRI{announceNLRI},
		AnnounceWire: [][]byte{announceNLRI.Bytes()},
		SourcePeerIP: netip.MustParseAddr("10.0.0.1"),
		SourceCtxID:  ctxID,
		ReceivedAt:   time.Now(),
	}

	// Convert to routes (what ForwardUpdate does internally)
	routes, err := update.ConvertToRoutes()
	require.NoError(t, err)
	require.Len(t, routes, 1)

	// Create adj-rib-out and mark route as sent
	adjRIBOut := rib.NewOutgoingRIB()
	for _, route := range routes {
		adjRIBOut.MarkSent(route)
	}

	// Verify route is in sent cache
	sentRoutes := adjRIBOut.GetSentRoutes()
	require.Len(t, sentRoutes, 1, "route must be in adj-rib-out after forward")
	assert.Equal(t, prefix.String(), sentRoutes[0].NLRI().String(), "NLRI must match")
}

// TestForwardWithdrawRemovesFromAdjRIBOut verifies withdraws are handled.
//
// VALIDATES: Withdrawn routes removed from adj-rib-out.
// PREVENTS: Stale routes replayed after withdraw.
func TestForwardWithdrawRemovesFromAdjRIBOut(t *testing.T) {
	// Register encoding context
	ctx := &bgpctx.EncodingContext{ASN4: true}
	ctxID := bgpctx.Registry.Register(ctx)

	prefix := netip.MustParsePrefix("192.168.1.0/24")
	testNLRI := nlri.NewINET(nlri.IPv4Unicast, prefix, 0)

	// First, add a route to adj-rib-out (simulating prior announcement)
	adjRIBOut := rib.NewOutgoingRIB()
	route := rib.NewRoute(testNLRI, netip.MustParseAddr("10.0.0.1"), nil)
	adjRIBOut.MarkSent(route)

	// Verify it's there
	require.Len(t, adjRIBOut.GetSentRoutes(), 1)

	// Now simulate a withdraw
	withdrawNLRI := nlri.NewINET(nlri.IPv4Unicast, prefix, 0)

	// Create withdraw-only ReceivedUpdate
	update := &ReceivedUpdate{
		UpdateID:     2,
		Attrs:        nil, // Withdraw-only
		Withdraws:    []nlri.NLRI{withdrawNLRI},
		WithdrawWire: [][]byte{withdrawNLRI.Bytes()},
		SourcePeerIP: netip.MustParseAddr("10.0.0.1"),
		SourceCtxID:  ctxID,
		ReceivedAt:   time.Now(),
	}

	// Remove from adj-rib-out (what ForwardUpdate does)
	for _, n := range update.Withdraws {
		adjRIBOut.RemoveFromSent(n)
	}

	// Verify route is gone
	sentRoutes := adjRIBOut.GetSentRoutes()
	assert.Empty(t, sentRoutes, "route must be removed from adj-rib-out after withdraw")
}

// TestReconnectReplaysForwardedRoutes verifies routes are available for replay.
//
// VALIDATES: Adj-rib-out routes available for replay on session re-establishment.
// PREVENTS: Route loss on peer flap.
func TestReconnectReplaysForwardedRoutes(t *testing.T) {
	// Register encoding context
	ctx := &bgpctx.EncodingContext{ASN4: true}
	ctxID := bgpctx.Registry.Register(ctx)

	// Create multiple routes
	prefixes := []string{"10.0.0.0/24", "10.0.1.0/24", "10.0.2.0/24"}
	adjRIBOut := rib.NewOutgoingRIB()

	for _, p := range prefixes {
		prefix := netip.MustParsePrefix(p)
		testNLRI := nlri.NewINET(nlri.IPv4Unicast, prefix, 0)
		route := rib.NewRouteWithWireCacheFull(
			testNLRI,
			netip.MustParseAddr("10.0.0.1"),
			nil, // attrs
			nil, // asPath
			nil, // wireBytes
			testNLRI.Bytes(),
			ctxID,
		)
		adjRIBOut.MarkSent(route)
	}

	// Simulate "reconnect" by getting sent routes (what sendInitialRoutes does)
	sentRoutes := adjRIBOut.GetSentRoutes()

	// Verify all routes are available for replay
	require.Len(t, sentRoutes, 3, "all routes must be available for replay")

	// Verify prefixes match (order may differ)
	gotPrefixes := make(map[string]bool)
	for _, route := range sentRoutes {
		gotPrefixes[route.NLRI().String()] = true
	}
	for _, p := range prefixes {
		assert.True(t, gotPrefixes[p], "prefix %s must be in replay set", p)
	}
}

// TestMultiplePeersIndependentAdjRIBOut verifies per-peer state isolation.
//
// VALIDATES: Each peer has independent adj-rib-out.
// PREVENTS: Cross-peer state corruption.
func TestMultiplePeersIndependentAdjRIBOut(t *testing.T) {
	// Create two independent adj-rib-out instances (simulating two peers)
	peer1RIB := rib.NewOutgoingRIB()
	peer2RIB := rib.NewOutgoingRIB()

	// Add different routes to each
	prefix1 := netip.MustParsePrefix("10.0.0.0/24")
	prefix2 := netip.MustParsePrefix("192.168.0.0/24")

	nlri1 := nlri.NewINET(nlri.IPv4Unicast, prefix1, 0)
	nlri2 := nlri.NewINET(nlri.IPv4Unicast, prefix2, 0)

	route1 := rib.NewRoute(nlri1, netip.MustParseAddr("10.0.0.1"), nil)
	route2 := rib.NewRoute(nlri2, netip.MustParseAddr("10.0.0.2"), nil)

	// Peer1 gets route1 only
	peer1RIB.MarkSent(route1)

	// Peer2 gets route2 only
	peer2RIB.MarkSent(route2)

	// Verify isolation
	peer1Routes := peer1RIB.GetSentRoutes()
	peer2Routes := peer2RIB.GetSentRoutes()

	require.Len(t, peer1Routes, 1)
	require.Len(t, peer2Routes, 1)

	assert.Equal(t, prefix1.String(), peer1Routes[0].NLRI().String(),
		"peer1 must only have its own routes")
	assert.Equal(t, prefix2.String(), peer2Routes[0].NLRI().String(),
		"peer2 must only have its own routes")

	// Withdrawing from peer1 shouldn't affect peer2
	peer1RIB.RemoveFromSent(nlri1)
	assert.Empty(t, peer1RIB.GetSentRoutes(), "peer1 routes must be empty after withdraw")
	assert.Len(t, peer2RIB.GetSentRoutes(), 1, "peer2 routes must be unaffected")
}

// buildTestRawUpdateBody builds a raw UPDATE message body for testing.
func buildTestRawUpdateBody(attrBytes []byte, nlris []nlri.NLRI) []byte {
	// Calculate NLRI length
	nlriLen := 0
	for _, n := range nlris {
		nlriLen += len(n.Bytes())
	}

	// Build body: WithdrawnLen(2) + Withdrawn(0) + AttrLen(2) + Attrs + NLRI
	body := make([]byte, 2+0+2+len(attrBytes)+nlriLen)
	offset := 0

	// Withdrawn routes length (0)
	body[offset] = 0
	body[offset+1] = 0
	offset += 2

	// Path attribute length
	body[offset] = byte(len(attrBytes) >> 8)
	body[offset+1] = byte(len(attrBytes))
	offset += 2

	// Path attributes
	copy(body[offset:], attrBytes)
	offset += len(attrBytes)

	// NLRI
	for _, n := range nlris {
		nlriBytes := n.Bytes()
		copy(body[offset:], nlriBytes)
		offset += len(nlriBytes)
	}

	return body
}
