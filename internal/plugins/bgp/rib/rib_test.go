package rib

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/nlri"
)

const testPeerID = "192.168.1.1"

// TestIncomingRIBInsert verifies that routes can be inserted into Adj-RIB-In.
//
// VALIDATES: Basic route insertion indexed by peer and NLRI.
//
// PREVENTS: Route loss on insert, incorrect indexing causing lookup failures.
func TestIncomingRIBInsert(t *testing.T) {
	rib := NewIncomingRIB()

	prefix := netip.MustParsePrefix("10.0.0.0/24")
	inet := nlri.NewINET(nlri.IPv4Unicast, prefix, 0)
	nextHop := netip.MustParseAddr("192.168.1.1")
	route := NewRoute(inet, nextHop, nil)

	// Insert route
	rib.Insert(testPeerID, route)

	// Lookup route
	found := rib.Get(testPeerID, route.Index())
	require.NotNil(t, found, "route must be found after insert")
	require.Equal(t, route.NLRI(), found.NLRI(), "NLRI must match")
}

// TestIncomingRIBReplace verifies that routes can be replaced (update).
//
// VALIDATES: Route updates replace existing routes for same NLRI.
//
// PREVENTS: Stale route data persisting after update, memory leaks from
// unreleased old routes.
func TestIncomingRIBReplace(t *testing.T) {
	rib := NewIncomingRIB()

	prefix := netip.MustParsePrefix("10.0.0.0/24")
	inet := nlri.NewINET(nlri.IPv4Unicast, prefix, 0)
	nextHop1 := netip.MustParseAddr("192.168.1.1")
	nextHop2 := netip.MustParseAddr("192.168.1.2")

	route1 := NewRoute(inet, nextHop1, nil)
	route2 := NewRoute(inet, nextHop2, nil)

	// Insert first route
	rib.Insert(testPeerID, route1)

	// Replace with second route
	old := rib.Insert(testPeerID, route2)
	require.NotNil(t, old, "old route must be returned on replace")
	require.Equal(t, nextHop1, old.NextHop(), "old route next-hop must match")

	// Verify new route is stored
	found := rib.Get(testPeerID, route2.Index())
	require.Equal(t, nextHop2, found.NextHop(), "new route next-hop must match")
}

// TestIncomingRIBRemove verifies that routes can be removed (withdrawal).
//
// VALIDATES: Route withdrawal removes route from RIB.
//
// PREVENTS: Withdrawn routes persisting in RIB, memory leaks.
func TestIncomingRIBRemove(t *testing.T) {
	rib := NewIncomingRIB()

	prefix := netip.MustParsePrefix("10.0.0.0/24")
	inet := nlri.NewINET(nlri.IPv4Unicast, prefix, 0)
	nextHop := netip.MustParseAddr("192.168.1.1")
	route := NewRoute(inet, nextHop, nil)

	rib.Insert(testPeerID, route)
	removed := rib.Remove(testPeerID, route.Index())

	require.NotNil(t, removed, "removed route must be returned")
	require.Nil(t, rib.Get(testPeerID, route.Index()), "route must be gone after remove")
}

// TestIncomingRIBClearPeer verifies that all routes from a peer can be cleared.
//
// VALIDATES: Peer session teardown clears all routes from that peer.
//
// PREVENTS: Stale routes persisting after peer disconnect, memory leaks.
func TestIncomingRIBClearPeer(t *testing.T) {
	rib := NewIncomingRIB()

	prefix1 := netip.MustParsePrefix("10.0.0.0/24")
	prefix2 := netip.MustParsePrefix("10.0.1.0/24")
	inet1 := nlri.NewINET(nlri.IPv4Unicast, prefix1, 0)
	inet2 := nlri.NewINET(nlri.IPv4Unicast, prefix2, 0)
	nextHop := netip.MustParseAddr("192.168.1.1")

	route1 := NewRoute(inet1, nextHop, nil)
	route2 := NewRoute(inet2, nextHop, nil)

	rib.Insert(testPeerID, route1)
	rib.Insert(testPeerID, route2)

	// Clear all routes from peer
	routes := rib.ClearPeer(testPeerID)
	require.Len(t, routes, 2, "must return 2 cleared routes")

	require.Nil(t, rib.Get(testPeerID, route1.Index()), "route1 must be gone")
	require.Nil(t, rib.Get(testPeerID, route2.Index()), "route2 must be gone")
}

// TestIncomingRIBMultiplePeers verifies route isolation between peers.
//
// VALIDATES: Routes from different peers are stored independently.
//
// PREVENTS: Route collision across peers, peer A's routes affecting peer B.
func TestIncomingRIBMultiplePeers(t *testing.T) {
	rib := NewIncomingRIB()

	prefix := netip.MustParsePrefix("10.0.0.0/24")
	inet := nlri.NewINET(nlri.IPv4Unicast, prefix, 0)

	nextHop1 := netip.MustParseAddr("192.168.1.1")
	nextHop2 := netip.MustParseAddr("192.168.1.2")

	route1 := NewRoute(inet, nextHop1, nil)
	route2 := NewRoute(inet, nextHop2, nil)

	peer1 := "192.168.1.1"
	peer2 := "192.168.1.2"

	// Insert same prefix from different peers
	rib.Insert(peer1, route1)
	rib.Insert(peer2, route2)

	// Each peer has its own route
	found1 := rib.Get(peer1, route1.Index())
	found2 := rib.Get(peer2, route2.Index())

	require.Equal(t, nextHop1, found1.NextHop(), "peer1 route must have its next-hop")
	require.Equal(t, nextHop2, found2.NextHop(), "peer2 route must have its next-hop")
}

// TestOutgoingRIBQueue verifies that routes can be queued for announcement.
//
// VALIDATES: Routes can be queued for outbound UPDATE messages.
//
// PREVENTS: Lost announcements, routes not being sent to peers.
func TestOutgoingRIBQueue(t *testing.T) {
	rib := NewOutgoingRIB()

	prefix := netip.MustParsePrefix("10.0.0.0/24")
	inet := nlri.NewINET(nlri.IPv4Unicast, prefix, 0)
	nextHop := netip.MustParseAddr("192.168.1.1")
	route := NewRoute(inet, nextHop, nil)

	// Queue route for announcement
	rib.QueueAnnounce(route)

	// Get pending routes
	pending := rib.GetPending(nlri.IPv4Unicast)
	require.Len(t, pending, 1, "must have 1 pending route")
	require.Equal(t, route.NLRI(), pending[0].NLRI(), "pending route NLRI must match")
}

// TestOutgoingRIBQueueWithdraw verifies withdrawal queuing.
//
// VALIDATES: Withdrawals can be queued and override pending announcements.
//
// PREVENTS: Announcing routes that should be withdrawn, stale routes.
func TestOutgoingRIBQueueWithdraw(t *testing.T) {
	rib := NewOutgoingRIB()

	prefix := netip.MustParsePrefix("10.0.0.0/24")
	inet := nlri.NewINET(nlri.IPv4Unicast, prefix, 0)
	nextHop := netip.MustParseAddr("192.168.1.1")
	route := NewRoute(inet, nextHop, nil)

	// Queue announce then withdraw
	rib.QueueAnnounce(route)
	rib.QueueWithdraw(inet)

	// Pending announcements should be empty (withdraw cancels announce)
	pending := rib.GetPending(nlri.IPv4Unicast)
	require.Len(t, pending, 0, "announce must be canceled by withdraw")

	// Withdrawals should be queued
	withdrawals := rib.GetWithdrawals(nlri.IPv4Unicast)
	require.Len(t, withdrawals, 1, "must have 1 pending withdrawal")
}

// TestOutgoingRIBFlush verifies pending route flushing.
//
// VALIDATES: Pending routes can be flushed after sending UPDATE.
//
// PREVENTS: Routes being sent multiple times, memory leaks in pending queue.
func TestOutgoingRIBFlush(t *testing.T) {
	rib := NewOutgoingRIB()

	prefix := netip.MustParsePrefix("10.0.0.0/24")
	inet := nlri.NewINET(nlri.IPv4Unicast, prefix, 0)
	nextHop := netip.MustParseAddr("192.168.1.1")
	route := NewRoute(inet, nextHop, nil)

	rib.QueueAnnounce(route)

	// Get pending (returns and clears)
	pending := rib.FlushPending(nlri.IPv4Unicast)
	require.Len(t, pending, 1, "must return pending routes")

	// Queue should be empty after flush
	pending2 := rib.GetPending(nlri.IPv4Unicast)
	require.Len(t, pending2, 0, "pending must be empty after flush")
}

// TestOutgoingRIBStats verifies route counting.
//
// VALIDATES: RIB provides accurate route counts for monitoring.
//
// PREVENTS: Incorrect stats leading to operational issues.
func TestOutgoingRIBStats(t *testing.T) {
	rib := NewOutgoingRIB()

	prefix1 := netip.MustParsePrefix("10.0.0.0/24")
	prefix2 := netip.MustParsePrefix("10.0.1.0/24")
	inet1 := nlri.NewINET(nlri.IPv4Unicast, prefix1, 0)
	inet2 := nlri.NewINET(nlri.IPv4Unicast, prefix2, 0)
	nextHop := netip.MustParseAddr("192.168.1.1")

	rib.QueueAnnounce(NewRoute(inet1, nextHop, nil))
	rib.QueueAnnounce(NewRoute(inet2, nextHop, nil))

	stats := rib.Stats()
	require.Equal(t, 2, stats.PendingAnnouncements, "must have 2 pending announcements")
}

// TestIncomingRIBClearAll verifies that all routes from all peers can be cleared.
//
// VALIDATES: Admin clear operation removes all incoming routes.
//
// PREVENTS: Stale routes persisting after RIB clear, memory leaks.
func TestIncomingRIBClearAll(t *testing.T) {
	rib := NewIncomingRIB()

	prefix1 := netip.MustParsePrefix("10.0.0.0/24")
	prefix2 := netip.MustParsePrefix("10.0.1.0/24")
	inet1 := nlri.NewINET(nlri.IPv4Unicast, prefix1, 0)
	inet2 := nlri.NewINET(nlri.IPv4Unicast, prefix2, 0)
	nextHop := netip.MustParseAddr("192.168.1.1")

	route1 := NewRoute(inet1, nextHop, nil)
	route2 := NewRoute(inet2, nextHop, nil)

	peer1 := "192.168.1.1"
	peer2 := "192.168.1.2"

	// Insert routes from different peers
	rib.Insert(peer1, route1)
	rib.Insert(peer2, route2)

	// Verify routes exist
	stats := rib.Stats()
	require.Equal(t, 2, stats.PeerCount, "must have 2 peers")
	require.Equal(t, 2, stats.RouteCount, "must have 2 routes")

	// Clear all
	count := rib.ClearAll()
	require.Equal(t, 2, count, "must report 2 routes cleared")

	// Verify all cleared
	stats = rib.Stats()
	require.Equal(t, 0, stats.PeerCount, "must have 0 peers after clear")
	require.Equal(t, 0, stats.RouteCount, "must have 0 routes after clear")
}

// TestOutgoingRIBClearSent verifies that clearing sent routes queues withdrawals.
//
// VALIDATES: ClearSent queues withdrawals and clears sent cache.
//
// PREVENTS: Orphaned routes in peers after admin clear.
func TestOutgoingRIBClearSent(t *testing.T) {
	rib := NewOutgoingRIB()

	prefix1 := netip.MustParsePrefix("10.0.0.0/24")
	prefix2 := netip.MustParsePrefix("10.0.1.0/24")
	inet1 := nlri.NewINET(nlri.IPv4Unicast, prefix1, 0)
	inet2 := nlri.NewINET(nlri.IPv4Unicast, prefix2, 0)
	nextHop := netip.MustParseAddr("192.168.1.1")

	route1 := NewRoute(inet1, nextHop, nil)
	route2 := NewRoute(inet2, nextHop, nil)

	// Mark routes as sent (simulating they were already advertised)
	rib.MarkSent(route1)
	rib.MarkSent(route2)

	stats := rib.Stats()
	require.Equal(t, 2, stats.SentRoutes, "must have 2 sent routes")

	// Clear sent - should queue withdrawals
	count := rib.ClearSent()
	require.Equal(t, 2, count, "must report 2 routes withdrawn")

	// Verify sent is empty and withdrawals are queued
	stats = rib.Stats()
	require.Equal(t, 0, stats.SentRoutes, "sent must be empty after clear")
	require.Equal(t, 2, stats.PendingWithdrawals, "must have 2 pending withdrawals")
}

// TestOutgoingRIBFlushSent verifies that flushing re-queues routes for announcement.
//
// VALIDATES: FlushSent queues sent routes for re-announcement.
//
// PREVENTS: Route sync failures after peer reconnection.
func TestOutgoingRIBFlushSent(t *testing.T) {
	rib := NewOutgoingRIB()

	prefix1 := netip.MustParsePrefix("10.0.0.0/24")
	prefix2 := netip.MustParsePrefix("10.0.1.0/24")
	inet1 := nlri.NewINET(nlri.IPv4Unicast, prefix1, 0)
	inet2 := nlri.NewINET(nlri.IPv4Unicast, prefix2, 0)
	nextHop := netip.MustParseAddr("192.168.1.1")

	route1 := NewRoute(inet1, nextHop, nil)
	route2 := NewRoute(inet2, nextHop, nil)

	// Mark routes as sent
	rib.MarkSent(route1)
	rib.MarkSent(route2)

	stats := rib.Stats()
	require.Equal(t, 2, stats.SentRoutes, "must have 2 sent routes")
	require.Equal(t, 0, stats.PendingAnnouncements, "must have 0 pending")

	// Flush - should re-queue for announcement
	count := rib.FlushSent()
	require.Equal(t, 2, count, "must report 2 routes flushed")

	// Verify routes are now pending (sent should remain for tracking)
	stats = rib.Stats()
	require.Equal(t, 2, stats.SentRoutes, "sent should still have routes")
	require.Equal(t, 2, stats.PendingAnnouncements, "must have 2 pending announcements")
}
