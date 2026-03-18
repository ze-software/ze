package watchdog

import (
	"errors"
	"strconv"
	"sync"
	"testing"
)

// addRoute is a test helper that fatals on error.
func addRoute(t *testing.T, ps *PoolSet, poolName string, entry *PoolEntry) {
	t.Helper()
	if err := ps.AddRoute(poolName, entry); err != nil {
		t.Fatalf("AddRoute(%s, %s): %v", poolName, entry.Key, err)
	}
}

// VALIDATES: Pool CRUD operations (add, remove, get, list)
// PREVENTS: Routes lost, duplicate routes accepted, empty pools lingering

func TestPoolSetAddRemove(t *testing.T) {
	tests := []struct {
		name       string
		poolName   string
		entries    []*PoolEntry
		removeKey  string
		wantPools  int
		wantRoutes int
		wantRemove bool
	}{
		{
			name:     "add single route",
			poolName: "dns",
			entries: []*PoolEntry{
				NewPoolEntry("10.0.0.0/24#0", "update text origin set igp nlri ipv4/unicast add 10.0.0.0/24", "update text nlri ipv4/unicast del 10.0.0.0/24"),
			},
			wantPools:  1,
			wantRoutes: 1,
		},
		{
			name:     "add multiple routes to same pool",
			poolName: "dns",
			entries: []*PoolEntry{
				NewPoolEntry("10.0.0.0/24#0", "announce-a", "withdraw-a"),
				NewPoolEntry("10.0.1.0/24#0", "announce-b", "withdraw-b"),
			},
			wantPools:  1,
			wantRoutes: 2,
		},
		{
			name:     "remove existing route",
			poolName: "dns",
			entries: []*PoolEntry{
				NewPoolEntry("10.0.0.0/24#0", "announce-a", "withdraw-a"),
				NewPoolEntry("10.0.1.0/24#0", "announce-b", "withdraw-b"),
			},
			removeKey:  "10.0.0.0/24#0",
			wantPools:  1,
			wantRoutes: 1,
			wantRemove: true,
		},
		{
			name:     "remove last route cleans up pool",
			poolName: "dns",
			entries: []*PoolEntry{
				NewPoolEntry("10.0.0.0/24#0", "announce-a", "withdraw-a"),
			},
			removeKey:  "10.0.0.0/24#0",
			wantPools:  0,
			wantRoutes: 0,
			wantRemove: true,
		},
		{
			name:     "remove nonexistent route",
			poolName: "dns",
			entries: []*PoolEntry{
				NewPoolEntry("10.0.0.0/24#0", "announce-a", "withdraw-a"),
			},
			removeKey:  "10.0.99.0/24#0",
			wantPools:  1,
			wantRoutes: 1,
			wantRemove: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ps := NewPoolSet()

			for _, e := range tt.entries {
				addRoute(t, ps, tt.poolName, e)
			}

			if tt.removeKey != "" {
				_, removed := ps.RemoveRoute(tt.poolName, tt.removeKey)
				if removed != tt.wantRemove {
					t.Errorf("RemoveRoute(%s) = %v, want %v", tt.removeKey, removed, tt.wantRemove)
				}
			}

			names := ps.PoolNames()
			if len(names) != tt.wantPools {
				t.Errorf("pool count = %d, want %d", len(names), tt.wantPools)
			}

			if tt.wantPools > 0 {
				pool := ps.GetPool(tt.poolName)
				if pool == nil {
					t.Fatal("GetPool returned nil")
					return
				}
				routes := pool.Routes()
				if len(routes) != tt.wantRoutes {
					t.Errorf("route count = %d, want %d", len(routes), tt.wantRoutes)
				}
			}
		})
	}
}

// VALIDATES: Duplicate route key rejected
// PREVENTS: Silent overwrite of existing route data

func TestPoolSetDuplicateRoute(t *testing.T) {
	ps := NewPoolSet()

	e1 := NewPoolEntry("10.0.0.0/24#0", "announce-1", "withdraw-1")
	addRoute(t, ps, "dns", e1)

	e2 := NewPoolEntry("10.0.0.0/24#0", "announce-2", "withdraw-2")
	err := ps.AddRoute("dns", e2)
	if !errors.Is(err, ErrRouteExists) {
		t.Errorf("second AddRoute = %v, want ErrRouteExists", err)
	}
}

// VALIDATES: Per-peer announce/withdraw state transitions
// PREVENTS: Routes stuck in wrong state, double-announce/withdraw

func TestPoolSetAnnounceWithdraw(t *testing.T) {
	ps := NewPoolSet()

	addRoute(t, ps, "dns", NewPoolEntry("10.0.0.0/24#0", "announce-a", "withdraw-a"))
	addRoute(t, ps, "dns", NewPoolEntry("10.0.1.0/24#0", "announce-b", "withdraw-b"))

	// Announce all for peer1
	announced := ps.AnnouncePool("dns", "192.168.1.1")
	if len(announced) != 2 {
		t.Errorf("first announce count = %d, want 2", len(announced))
	}

	// Second announce should return nothing (already announced)
	announced2 := ps.AnnouncePool("dns", "192.168.1.1")
	if len(announced2) != 0 {
		t.Errorf("second announce count = %d, want 0", len(announced2))
	}

	// Different peer should still get announced
	announced3 := ps.AnnouncePool("dns", "192.168.1.2")
	if len(announced3) != 2 {
		t.Errorf("peer2 announce count = %d, want 2", len(announced3))
	}

	// Withdraw for peer1
	withdrawn := ps.WithdrawPool("dns", "192.168.1.1")
	if len(withdrawn) != 2 {
		t.Errorf("withdraw count = %d, want 2", len(withdrawn))
	}

	// Second withdraw should return nothing
	withdrawn2 := ps.WithdrawPool("dns", "192.168.1.1")
	if len(withdrawn2) != 0 {
		t.Errorf("second withdraw count = %d, want 0", len(withdrawn2))
	}

	// Peer2 should still be announced
	peer2Announced := ps.AnnouncedForPeer("dns", "192.168.1.2")
	if len(peer2Announced) != 2 {
		t.Errorf("peer2 still announced = %d, want 2", len(peer2Announced))
	}
}

// VALIDATES: Nonexistent pool returns nil
// PREVENTS: Panic on missing pool

func TestPoolSetNonexistentPool(t *testing.T) {
	ps := NewPoolSet()

	if pool := ps.GetPool("missing"); pool != nil {
		t.Error("GetPool(missing) should return nil")
	}
	if announced := ps.AnnouncePool("missing", "peer1"); announced != nil {
		t.Error("AnnouncePool(missing) should return nil")
	}
	if withdrawn := ps.WithdrawPool("missing", "peer1"); withdrawn != nil {
		t.Error("WithdrawPool(missing) should return nil")
	}
	if entries := ps.AnnouncedForPeer("missing", "peer1"); entries != nil {
		t.Error("AnnouncedForPeer(missing) should return nil")
	}
}

// VALIDATES: PoolEntry methods (IsAnnounced, AnnouncedPeers)
// PREVENTS: State query returning wrong results

func TestPoolEntryState(t *testing.T) {
	ps := NewPoolSet()
	addRoute(t, ps, "dns", NewPoolEntry("10.0.0.0/24#0", "announce", "withdraw"))

	pool := ps.GetPool("dns")
	routes := pool.Routes()
	if len(routes) != 1 {
		t.Fatalf("route count = %d, want 1", len(routes))
	}
	entry := routes[0]

	// Initially not announced
	if entry.IsAnnounced("peer1") {
		t.Error("should not be announced initially")
	}

	// Announce for peer1
	ps.AnnouncePool("dns", "peer1")

	if !entry.IsAnnounced("peer1") {
		t.Error("should be announced after AnnouncePool")
	}
	if entry.IsAnnounced("peer2") {
		t.Error("peer2 should not be announced")
	}

	// Check AnnouncedPeers
	peers := entry.AnnouncedPeers()
	if len(peers) != 1 || peers[0] != "peer1" {
		t.Errorf("AnnouncedPeers = %v, want [peer1]", peers)
	}

	// Announce for peer2
	ps.AnnouncePool("dns", "peer2")

	peers = entry.AnnouncedPeers()
	if len(peers) != 2 {
		t.Errorf("AnnouncedPeers count = %d, want 2", len(peers))
	}
}

// VALIDATES: Empty pool auto-cleaned after all routes removed
// PREVENTS: Pool map growing indefinitely

func TestPoolSetAutoCleanup(t *testing.T) {
	ps := NewPoolSet()

	addRoute(t, ps, "dns", NewPoolEntry("10.0.0.0/24#0", "a", "w"))
	addRoute(t, ps, "dns", NewPoolEntry("10.0.1.0/24#0", "a", "w"))

	if len(ps.PoolNames()) != 1 {
		t.Fatal("expected 1 pool")
	}

	ps.RemoveRoute("dns", "10.0.0.0/24#0")
	if len(ps.PoolNames()) != 1 {
		t.Error("pool should still exist with 1 route")
	}

	ps.RemoveRoute("dns", "10.0.1.0/24#0")
	if len(ps.PoolNames()) != 0 {
		t.Error("pool should be auto-cleaned after last route removed")
	}
}

// VALIDATES: Thread-safety under concurrent access
// PREVENTS: Race conditions, map corruption

func TestPoolSetConcurrency(t *testing.T) {
	ps := NewPoolSet()

	// Pre-populate
	for i := range 10 {
		key := "10.0." + strconv.Itoa(i) + ".0/24#0"
		addRoute(t, ps, "dns", NewPoolEntry(key, "announce-"+key, "withdraw-"+key))
	}

	var wg sync.WaitGroup

	// Concurrent announce/withdraw from multiple peers
	for p := range 5 {
		peer := "192.168.1." + strconv.Itoa(p)
		wg.Go(func() {
			ps.AnnouncePool("dns", peer)
			ps.WithdrawPool("dns", peer)
			ps.AnnouncePool("dns", peer)
		})
	}

	// Concurrent reads
	for r := range 5 {
		peer := "192.168.1." + strconv.Itoa(r)
		wg.Go(func() {
			ps.PoolNames()
			ps.GetPool("dns")
			ps.AnnouncedForPeer("dns", peer)
		})
	}

	wg.Wait()

	// Verify no corruption
	pool := ps.GetPool("dns")
	if pool == nil {
		t.Fatal("pool should exist after concurrent operations")
		return
	}
	routes := pool.Routes()
	if len(routes) != 10 {
		t.Errorf("route count = %d, want 10 after concurrent operations", len(routes))
	}
}

// VALIDATES: AnnounceInitial only marks initiallyAnnounced entries
// PREVENTS: Withdrawn routes promoted to announced on session establishment

func TestAnnounceInitialPool(t *testing.T) {
	ps := NewPoolSet()

	// Route A: initiallyAnnounced
	routeA := NewPoolEntry("10.0.0.0/24#0", "announce-a", "withdraw-a")
	routeA.initiallyAnnounced = true
	addRoute(t, ps, "dns", routeA)

	// Route B: initially withdrawn (default)
	routeB := NewPoolEntry("10.0.1.0/24#0", "announce-b", "withdraw-b")
	addRoute(t, ps, "dns", routeB)

	// Route C: another initiallyAnnounced
	routeC := NewPoolEntry("10.0.2.0/24#0", "announce-c", "withdraw-c")
	routeC.initiallyAnnounced = true
	addRoute(t, ps, "dns", routeC)

	// AnnounceInitial should only mark A and C
	announced := ps.AnnounceInitial("dns", "peer1")
	if len(announced) != 2 {
		t.Fatalf("AnnounceInitial count = %d, want 2", len(announced))
	}

	// Verify route B is NOT announced
	if routeB.IsAnnounced("peer1") {
		t.Error("route B should NOT be announced (initiallyAnnounced=false)")
	}

	// Verify routes A and C ARE announced
	if !routeA.IsAnnounced("peer1") {
		t.Error("route A should be announced")
	}
	if !routeC.IsAnnounced("peer1") {
		t.Error("route C should be announced")
	}

	// Second call should return empty (already announced)
	announced2 := ps.AnnounceInitial("dns", "peer1")
	if len(announced2) != 0 {
		t.Errorf("second AnnounceInitial count = %d, want 0", len(announced2))
	}

	// Different peer should still get the initial routes
	announced3 := ps.AnnounceInitial("dns", "peer2")
	if len(announced3) != 2 {
		t.Errorf("peer2 AnnounceInitial count = %d, want 2", len(announced3))
	}

	// Nonexistent pool returns nil
	if result := ps.AnnounceInitial("missing", "peer1"); result != nil {
		t.Error("AnnounceInitial on missing pool should return nil")
	}
}

// VALIDATES: RemoveRoute returns removed entry data
// PREVENTS: Lost route data needed for withdrawal commands

func TestPoolSetRemoveReturnsEntry(t *testing.T) {
	ps := NewPoolSet()
	addRoute(t, ps, "dns", NewPoolEntry("10.0.0.0/24#0", "announce-cmd", "withdraw-cmd"))

	// Announce first so we can verify state is preserved
	ps.AnnouncePool("dns", "peer1")

	removed, ok := ps.RemoveRoute("dns", "10.0.0.0/24#0")
	if !ok {
		t.Fatal("RemoveRoute should return true")
	}
	if removed.Key != "10.0.0.0/24#0" {
		t.Errorf("Key = %s, want 10.0.0.0/24#0", removed.Key)
	}
	if removed.AnnounceCmd != "announce-cmd" {
		t.Errorf("AnnounceCmd = %s, want announce-cmd", removed.AnnounceCmd)
	}
	if removed.WithdrawCmd != "withdraw-cmd" {
		t.Errorf("WithdrawCmd = %s, want withdraw-cmd", removed.WithdrawCmd)
	}

	// Remove from nonexistent pool
	_, ok = ps.RemoveRoute("missing", "key")
	if ok {
		t.Error("RemoveRoute from missing pool should return false")
	}
}
