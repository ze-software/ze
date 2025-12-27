package reactor

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// WatchdogManager Tests

// TestWatchdogManagerAddRouteCreatesPool verifies pool is auto-created on first route.
//
// VALIDATES: AddRoute creates pool if it doesn't exist.
//
// PREVENTS: Nil pointer panic when adding to non-existent pool.
func TestWatchdogManagerAddRouteCreatesPool(t *testing.T) {
	wm := NewWatchdogManager()

	route := StaticRoute{
		Prefix:  netip.MustParsePrefix("10.0.0.0/24"),
		NextHop: netip.MustParseAddr("1.2.3.4"),
	}

	pr, err := wm.AddRoute("health", route)
	require.NoError(t, err)
	require.NotNil(t, pr, "AddRoute should return PoolRoute")
	assert.Equal(t, "10.0.0.0/24#0", pr.RouteKey(), "route key should match")

	// Pool should exist
	pool := wm.GetPool("health")
	require.NotNil(t, pool, "pool should be created")
	assert.Len(t, pool.Routes(), 1, "pool should have 1 route")
}

// TestWatchdogManagerAddRouteToExistingPool verifies routes are added to existing pools.
//
// VALIDATES: AddRoute appends to existing pool correctly.
//
// PREVENTS: Routes overwriting each other in same pool.
func TestWatchdogManagerAddRouteToExistingPool(t *testing.T) {
	wm := NewWatchdogManager()

	route1 := StaticRoute{
		Prefix:  netip.MustParsePrefix("10.0.0.0/24"),
		NextHop: netip.MustParseAddr("1.2.3.4"),
	}
	route2 := StaticRoute{
		Prefix:  netip.MustParsePrefix("10.0.1.0/24"),
		NextHop: netip.MustParseAddr("1.2.3.4"),
	}

	_, err := wm.AddRoute("health", route1)
	require.NoError(t, err)
	_, err = wm.AddRoute("health", route2)
	require.NoError(t, err)

	pool := wm.GetPool("health")
	require.NotNil(t, pool)
	assert.Len(t, pool.Routes(), 2, "pool should have 2 routes")
}

// TestWatchdogManagerRemoveRoute verifies route removal from pool.
//
// VALIDATES: RemoveRoute removes the correct route.
//
// PREVENTS: Wrong route being removed or pool corruption.
func TestWatchdogManagerRemoveRoute(t *testing.T) {
	wm := NewWatchdogManager()

	route1 := StaticRoute{
		Prefix:  netip.MustParsePrefix("10.0.0.0/24"),
		NextHop: netip.MustParseAddr("1.2.3.4"),
	}
	route2 := StaticRoute{
		Prefix:  netip.MustParsePrefix("10.0.1.0/24"),
		NextHop: netip.MustParseAddr("1.2.3.4"),
	}

	_, _ = wm.AddRoute("health", route1)
	_, _ = wm.AddRoute("health", route2)

	// Remove first route
	removed := wm.RemoveRoute("health", "10.0.0.0/24#0")
	assert.True(t, removed, "should return true for successful removal")

	pool := wm.GetPool("health")
	assert.Len(t, pool.Routes(), 1, "pool should have 1 route after removal")

	// Remaining route should be route2
	routes := pool.Routes()
	assert.Equal(t, "10.0.1.0/24#0", routes[0].RouteKey())
}

// TestWatchdogManagerRemoveRouteMissingPool verifies removing from non-existent pool.
//
// VALIDATES: RemoveRoute handles missing pool gracefully.
//
// PREVENTS: Panic when removing from non-existent pool.
func TestWatchdogManagerRemoveRouteMissingPool(t *testing.T) {
	wm := NewWatchdogManager()

	removed := wm.RemoveRoute("nonexistent", "10.0.0.0/24#0")
	assert.False(t, removed, "should return false for missing pool")
}

// TestWatchdogManagerRemoveRouteMissingRoute verifies removing non-existent route.
//
// VALIDATES: RemoveRoute handles missing route gracefully.
//
// PREVENTS: Panic or corruption when removing non-existent route.
func TestWatchdogManagerRemoveRouteMissingRoute(t *testing.T) {
	wm := NewWatchdogManager()

	route := StaticRoute{
		Prefix:  netip.MustParsePrefix("10.0.0.0/24"),
		NextHop: netip.MustParseAddr("1.2.3.4"),
	}
	_, _ = wm.AddRoute("health", route)

	removed := wm.RemoveRoute("health", "192.168.0.0/24#0")
	assert.False(t, removed, "should return false for missing route")

	// Original route should still exist
	pool := wm.GetPool("health")
	assert.Len(t, pool.Routes(), 1)
}

// TestWatchdogManagerGetPoolMissing verifies GetPool for non-existent pool.
//
// VALIDATES: GetPool returns nil for missing pool.
//
// PREVENTS: Incorrect pool being returned.
func TestWatchdogManagerGetPoolMissing(t *testing.T) {
	wm := NewWatchdogManager()

	pool := wm.GetPool("nonexistent")
	assert.Nil(t, pool, "should return nil for missing pool")
}

// WatchdogPool Tests

// TestWatchdogPoolPerPeerState verifies per-peer announced state tracking.
//
// VALIDATES: PoolRoute tracks announced state per peer independently.
//
// PREVENTS: One peer's state affecting another peer's state.
func TestWatchdogPoolPerPeerState(t *testing.T) {
	wm := NewWatchdogManager()

	route := StaticRoute{
		Prefix:  netip.MustParsePrefix("10.0.0.0/24"),
		NextHop: netip.MustParseAddr("1.2.3.4"),
	}
	pr, err := wm.AddRoute("health", route)
	require.NoError(t, err)

	// Default state should be false (not announced)
	assert.False(t, pr.IsAnnounced("192.168.1.1"), "should default to not announced")
	assert.False(t, pr.IsAnnounced("192.168.1.2"), "should default to not announced")

	// Mark announced for peer1
	pr.SetAnnounced("192.168.1.1", true)
	assert.True(t, pr.IsAnnounced("192.168.1.1"), "peer1 should be announced")
	assert.False(t, pr.IsAnnounced("192.168.1.2"), "peer2 should still be not announced")

	// Mark announced for peer2
	pr.SetAnnounced("192.168.1.2", true)
	assert.True(t, pr.IsAnnounced("192.168.1.1"))
	assert.True(t, pr.IsAnnounced("192.168.1.2"))

	// Withdraw from peer1 only
	pr.SetAnnounced("192.168.1.1", false)
	assert.False(t, pr.IsAnnounced("192.168.1.1"), "peer1 should be withdrawn")
	assert.True(t, pr.IsAnnounced("192.168.1.2"), "peer2 should still be announced")
}

// TestWatchdogManagerAnnouncePool verifies bulk announce returns correct routes.
//
// VALIDATES: AnnouncePool returns routes that are currently withdrawn for peer.
//
// PREVENTS: Re-announcing already announced routes.
func TestWatchdogManagerAnnouncePool(t *testing.T) {
	wm := NewWatchdogManager()

	route1 := StaticRoute{
		Prefix:  netip.MustParsePrefix("10.0.0.0/24"),
		NextHop: netip.MustParseAddr("1.2.3.4"),
	}
	route2 := StaticRoute{
		Prefix:  netip.MustParsePrefix("10.0.1.0/24"),
		NextHop: netip.MustParseAddr("1.2.3.4"),
	}

	pr1, _ := wm.AddRoute("health", route1)
	_, _ = wm.AddRoute("health", route2)

	// Mark route1 as already announced for peer1
	pr1.SetAnnounced("192.168.1.1", true)

	// AnnouncePool should return only route2 (not yet announced for peer1)
	routes := wm.AnnouncePool("health", "192.168.1.1")
	assert.Len(t, routes, 1, "should return 1 route to announce")
	assert.Equal(t, "10.0.1.0/24#0", routes[0].RouteKey())

	// After AnnouncePool, route2 should be marked announced
	pool := wm.GetPool("health")
	for _, pr := range pool.Routes() {
		assert.True(t, pr.IsAnnounced("192.168.1.1"), "all routes should be announced for peer1")
	}
}

// TestWatchdogManagerWithdrawPool verifies bulk withdraw returns correct routes.
//
// VALIDATES: WithdrawPool returns routes that are currently announced for peer.
//
// PREVENTS: Re-withdrawing already withdrawn routes.
func TestWatchdogManagerWithdrawPool(t *testing.T) {
	wm := NewWatchdogManager()

	route1 := StaticRoute{
		Prefix:  netip.MustParsePrefix("10.0.0.0/24"),
		NextHop: netip.MustParseAddr("1.2.3.4"),
	}
	route2 := StaticRoute{
		Prefix:  netip.MustParsePrefix("10.0.1.0/24"),
		NextHop: netip.MustParseAddr("1.2.3.4"),
	}

	pr1, _ := wm.AddRoute("health", route1)
	pr2, _ := wm.AddRoute("health", route2)

	// Mark both as announced for peer1
	pr1.SetAnnounced("192.168.1.1", true)
	pr2.SetAnnounced("192.168.1.1", true)

	// WithdrawPool should return both routes
	routes := wm.WithdrawPool("health", "192.168.1.1")
	assert.Len(t, routes, 2, "should return 2 routes to withdraw")

	// After WithdrawPool, both should be marked withdrawn
	pool := wm.GetPool("health")
	for _, pr := range pool.Routes() {
		assert.False(t, pr.IsAnnounced("192.168.1.1"), "all routes should be withdrawn for peer1")
	}
}

// TestWatchdogManagerAnnouncePoolMissing verifies AnnouncePool for non-existent pool.
//
// VALIDATES: AnnouncePool returns nil for missing pool.
//
// PREVENTS: Panic when announcing non-existent pool.
func TestWatchdogManagerAnnouncePoolMissing(t *testing.T) {
	wm := NewWatchdogManager()

	routes := wm.AnnouncePool("nonexistent", "192.168.1.1")
	assert.Nil(t, routes, "should return nil for missing pool")
}

// TestWatchdogManagerWithdrawPoolMissing verifies WithdrawPool for non-existent pool.
//
// VALIDATES: WithdrawPool returns nil for missing pool.
//
// PREVENTS: Panic when withdrawing non-existent pool.
func TestWatchdogManagerWithdrawPoolMissing(t *testing.T) {
	wm := NewWatchdogManager()

	routes := wm.WithdrawPool("nonexistent", "192.168.1.1")
	assert.Nil(t, routes, "should return nil for missing pool")
}

// TestWatchdogManagerPoolNames verifies listing all pool names.
//
// VALIDATES: PoolNames returns all created pool names.
//
// PREVENTS: Missing pools in list.
func TestWatchdogManagerPoolNames(t *testing.T) {
	wm := NewWatchdogManager()

	// Empty manager
	names := wm.PoolNames()
	assert.Empty(t, names, "empty manager should have no pools")

	// Add routes to different pools
	route := StaticRoute{
		Prefix:  netip.MustParsePrefix("10.0.0.0/24"),
		NextHop: netip.MustParseAddr("1.2.3.4"),
	}
	_, _ = wm.AddRoute("health", route)
	_, _ = wm.AddRoute("backup", route)
	_, _ = wm.AddRoute("primary", route)

	names = wm.PoolNames()
	assert.Len(t, names, 3, "should have 3 pools")
	assert.Contains(t, names, "health")
	assert.Contains(t, names, "backup")
	assert.Contains(t, names, "primary")
}

// TestWatchdogManagerConcurrency verifies thread-safe operations.
//
// VALIDATES: Concurrent access to WatchdogManager is safe.
//
// PREVENTS: Race conditions and data corruption.
func TestWatchdogManagerConcurrency(t *testing.T) {
	wm := NewWatchdogManager()

	// Run concurrent operations
	done := make(chan bool)

	for i := 0; i < 10; i++ {
		go func() {
			route := StaticRoute{
				Prefix:  netip.MustParsePrefix("10.0.0.0/24"),
				NextHop: netip.MustParseAddr("1.2.3.4"),
			}
			// Ignore error - duplicates are expected in concurrent test
			_, _ = wm.AddRoute("health", route)
			wm.GetPool("health")
			wm.AnnouncePool("health", "192.168.1.1")
			wm.WithdrawPool("health", "192.168.1.1")
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	// Should not panic or deadlock
	pool := wm.GetPool("health")
	assert.NotNil(t, pool)
}

// TestWatchdogPoolRouteWithPathID verifies PathID is part of route key.
//
// VALIDATES: Different PathIDs create different route entries.
//
// PREVENTS: PathID being ignored in route identification.
func TestWatchdogPoolRouteWithPathID(t *testing.T) {
	wm := NewWatchdogManager()

	route1 := StaticRoute{
		Prefix:  netip.MustParsePrefix("10.0.0.0/24"),
		NextHop: netip.MustParseAddr("1.2.3.4"),
		PathID:  1,
	}
	route2 := StaticRoute{
		Prefix:  netip.MustParsePrefix("10.0.0.0/24"),
		NextHop: netip.MustParseAddr("1.2.3.4"),
		PathID:  2,
	}

	_, err := wm.AddRoute("health", route1)
	require.NoError(t, err)
	_, err = wm.AddRoute("health", route2)
	require.NoError(t, err)

	pool := wm.GetPool("health")
	assert.Len(t, pool.Routes(), 2, "should have 2 routes with different PathIDs")

	// Remove only PathID=1
	removed := wm.RemoveRoute("health", "10.0.0.0/24#1")
	assert.True(t, removed)

	// PathID=2 should still exist
	pool = wm.GetPool("health")
	assert.Len(t, pool.Routes(), 1)
	assert.Equal(t, "10.0.0.0/24#2", pool.Routes()[0].RouteKey())
}

// Reactor Integration Tests

// TestReactorHasWatchdogManager verifies reactor initializes WatchdogManager.
//
// VALIDATES: New reactor has initialized WatchdogManager.
//
// PREVENTS: Nil pointer panic when using watchdog features.
func TestReactorHasWatchdogManager(t *testing.T) {
	r := New(&Config{})
	require.NotNil(t, r.WatchdogManager(), "reactor should have WatchdogManager")
}

// TestReactorAddWatchdogRoute verifies adding routes to global pools.
//
// VALIDATES: AddWatchdogRoute creates pool and adds route.
//
// PREVENTS: Routes not being stored in global pool.
func TestReactorAddWatchdogRoute(t *testing.T) {
	r := New(&Config{})

	route := StaticRoute{
		Prefix:  netip.MustParsePrefix("10.0.0.0/24"),
		NextHop: netip.MustParseAddr("1.2.3.4"),
	}

	err := r.AddWatchdogRoute(route, "health")
	require.NoError(t, err)

	pool := r.WatchdogManager().GetPool("health")
	require.NotNil(t, pool)
	assert.Len(t, pool.Routes(), 1)
}

// TestReactorRemoveWatchdogRoute verifies removing routes from global pools.
//
// VALIDATES: RemoveWatchdogRoute removes the correct route.
//
// PREVENTS: Route remaining in pool after removal.
func TestReactorRemoveWatchdogRoute(t *testing.T) {
	r := New(&Config{})

	route1 := StaticRoute{
		Prefix:  netip.MustParsePrefix("10.0.0.0/24"),
		NextHop: netip.MustParseAddr("1.2.3.4"),
	}
	route2 := StaticRoute{
		Prefix:  netip.MustParsePrefix("10.0.1.0/24"),
		NextHop: netip.MustParseAddr("1.2.3.4"),
	}

	_ = r.AddWatchdogRoute(route1, "health")
	_ = r.AddWatchdogRoute(route2, "health")

	err := r.RemoveWatchdogRoute("10.0.0.0/24#0", "health")
	require.NoError(t, err)

	pool := r.WatchdogManager().GetPool("health")
	require.NotNil(t, pool)
	assert.Len(t, pool.Routes(), 1, "should have 1 route remaining")
	assert.Equal(t, "10.0.1.0/24#0", pool.Routes()[0].RouteKey())

	// Remove last route - pool should be cleaned up
	err = r.RemoveWatchdogRoute("10.0.1.0/24#0", "health")
	require.NoError(t, err)

	pool = r.WatchdogManager().GetPool("health")
	assert.Nil(t, pool, "empty pool should be cleaned up")
}

// TestReactorRemoveWatchdogRouteMissingPool verifies error for missing pool.
//
// VALIDATES: RemoveWatchdogRoute returns error for missing pool.
//
// PREVENTS: Silent failure when removing from non-existent pool.
func TestReactorRemoveWatchdogRouteMissingPool(t *testing.T) {
	r := New(&Config{})

	err := r.RemoveWatchdogRoute("10.0.0.0/24#0", "nonexistent")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrWatchdogNotFound)
}

// TestReactorRemoveWatchdogRouteMissingRoute verifies error for missing route.
//
// VALIDATES: RemoveWatchdogRoute returns error for missing route.
//
// PREVENTS: Silent failure when removing non-existent route.
func TestReactorRemoveWatchdogRouteMissingRoute(t *testing.T) {
	r := New(&Config{})

	route := StaticRoute{
		Prefix:  netip.MustParsePrefix("10.0.0.0/24"),
		NextHop: netip.MustParseAddr("1.2.3.4"),
	}
	_ = r.AddWatchdogRoute(route, "health")

	err := r.RemoveWatchdogRoute("192.168.0.0/24#0", "health")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrWatchdogRouteNotFound)
}

// TestPeerGetsGlobalWatchdog verifies peer gets global watchdog when added to reactor.
//
// VALIDATES: AddPeer sets global watchdog on peer.
//
// PREVENTS: Peer missing global watchdog for reconnect handling.
func TestPeerGetsGlobalWatchdog(t *testing.T) {
	r := New(&Config{})

	settings := &PeerSettings{
		Address: netip.MustParseAddr("192.168.1.1"),
		LocalAS: 65001,
		PeerAS:  65002,
	}

	err := r.AddPeer(settings)
	require.NoError(t, err)

	// Get the peer and verify it has global watchdog set
	r.mu.RLock()
	peer := r.peers["192.168.1.1"]
	r.mu.RUnlock()

	require.NotNil(t, peer)

	peer.mu.RLock()
	hasGlobalWatchdog := peer.globalWatchdog != nil
	peer.mu.RUnlock()

	assert.True(t, hasGlobalWatchdog, "peer should have global watchdog set")
}

// TestGlobalPoolAnnouncedStateTracked verifies per-peer announced state is tracked.
//
// VALIDATES: AnnouncePool marks routes as announced for specific peer.
//
// PREVENTS: Routes not being re-sent on peer reconnect.
func TestGlobalPoolAnnouncedStateTracked(t *testing.T) {
	wm := NewWatchdogManager()

	route := StaticRoute{
		Prefix:  netip.MustParsePrefix("10.0.0.0/24"),
		NextHop: netip.MustParseAddr("1.2.3.4"),
	}
	_, _ = wm.AddRoute("health", route)

	// Announce to peer1
	routes := wm.AnnouncePool("health", "192.168.1.1")
	assert.Len(t, routes, 1, "should return 1 route to announce")

	// Verify state is tracked
	pool := wm.GetPool("health")
	for _, pr := range pool.Routes() {
		assert.True(t, pr.IsAnnounced("192.168.1.1"), "should be announced for peer1")
		assert.False(t, pr.IsAnnounced("192.168.1.2"), "should NOT be announced for peer2")
	}

	// Announce again - should return no routes (already announced)
	routes = wm.AnnouncePool("health", "192.168.1.1")
	assert.Len(t, routes, 0, "should return 0 routes (already announced)")

	// Now announce to peer2
	routes = wm.AnnouncePool("health", "192.168.1.2")
	assert.Len(t, routes, 1, "should return 1 route for peer2")
}

// TestNextHopSelfStoredInPool verifies NextHopSelf flag is preserved in pool routes.
//
// VALIDATES: Routes with NextHopSelf=true are stored correctly.
//
// PREVENTS: NextHopSelf being lost when adding to pool.
func TestNextHopSelfStoredInPool(t *testing.T) {
	wm := NewWatchdogManager()

	route := StaticRoute{
		Prefix:      netip.MustParsePrefix("10.0.0.0/24"),
		NextHopSelf: true, // No NextHop set, will be resolved per-peer
	}
	pr, err := wm.AddRoute("health", route)
	require.NoError(t, err)

	assert.True(t, pr.NextHopSelf, "NextHopSelf should be preserved")
	assert.False(t, pr.NextHop.IsValid(), "NextHop should be invalid (will be resolved)")
}

// TestWatchdogManagerAddRouteDuplicate verifies duplicate route key returns error.
//
// VALIDATES: AddRoute returns ErrRouteExists for duplicate key.
//
// PREVENTS: Silent overwrite of existing routes.
func TestWatchdogManagerAddRouteDuplicate(t *testing.T) {
	wm := NewWatchdogManager()

	route := StaticRoute{
		Prefix:  netip.MustParsePrefix("10.0.0.0/24"),
		NextHop: netip.MustParseAddr("1.2.3.4"),
	}

	// First add should succeed
	_, err := wm.AddRoute("health", route)
	require.NoError(t, err)

	// Second add with same key should fail
	_, err = wm.AddRoute("health", route)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrRouteExists)

	// Pool should still have only 1 route
	pool := wm.GetPool("health")
	assert.Len(t, pool.Routes(), 1)
}

// TestWatchdogManagerEmptyPoolCleanup verifies empty pools are removed.
//
// VALIDATES: RemoveRoute deletes pool when last route is removed.
//
// PREVENTS: Memory leak from empty pools.
func TestWatchdogManagerEmptyPoolCleanup(t *testing.T) {
	wm := NewWatchdogManager()

	route := StaticRoute{
		Prefix:  netip.MustParsePrefix("10.0.0.0/24"),
		NextHop: netip.MustParseAddr("1.2.3.4"),
	}

	_, _ = wm.AddRoute("health", route)
	assert.NotNil(t, wm.GetPool("health"), "pool should exist")

	// Remove the only route
	removed := wm.RemoveRoute("health", "10.0.0.0/24#0")
	assert.True(t, removed)

	// Pool should be cleaned up
	assert.Nil(t, wm.GetPool("health"), "empty pool should be removed")
}
