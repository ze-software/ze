package bgp_rr

import (
	"fmt"
	"sync"
	"testing"
)

// TestRIB_ConcurrentDifferentPeers verifies concurrent updates to different peers.
//
// VALIDATES: Multiple goroutines inserting/removing routes for different peers
// don't corrupt each other's data (AC-16).
// PREVENTS: Data race or deadlock under concurrent per-peer RIB access.
func TestRIB_ConcurrentDifferentPeers(t *testing.T) {
	rib := NewRIB()
	const numPeers = 10
	const routesPerPeer = 100

	var wg sync.WaitGroup
	wg.Add(numPeers)

	for p := range numPeers {
		go func() {
			defer wg.Done()
			peerID := fmt.Sprintf("10.0.0.%d", p+1)

			// Insert routes.
			for r := range routesPerPeer {
				rib.Insert(peerID, &Route{
					MsgID:  uint64(p*1000 + r),
					Family: "ipv4/unicast",
					Prefix: fmt.Sprintf("10.%d.%d.0/24", p, r),
				})
			}

			// Read routes.
			routes := rib.GetPeerRoutes(peerID)
			if len(routes) != routesPerPeer {
				t.Errorf("peer %s: expected %d routes, got %d", peerID, routesPerPeer, len(routes))
			}

			// Remove half.
			for r := range routesPerPeer / 2 {
				rib.Remove(peerID, "ipv4/unicast", fmt.Sprintf("10.%d.%d.0/24", p, r))
			}
		}()
	}

	wg.Wait()

	// Verify each peer has exactly half its routes remaining.
	for p := range numPeers {
		peerID := fmt.Sprintf("10.0.0.%d", p+1)
		routes := rib.GetPeerRoutes(peerID)
		expected := routesPerPeer / 2
		if len(routes) != expected {
			t.Errorf("peer %s: expected %d routes after removal, got %d", peerID, expected, len(routes))
		}
	}
}

// TestRIB_ConcurrentInsertAndClear verifies ClearPeer during concurrent inserts.
//
// VALIDATES: ClearPeer is safe during concurrent Insert from other goroutines.
// PREVENTS: Race between Insert and ClearPeer for the same peer.
func TestRIB_ConcurrentInsertAndClear(t *testing.T) {
	rib := NewRIB()

	var wg sync.WaitGroup
	wg.Add(2)

	// Goroutine 1: continuously inserts routes for peer 1.
	go func() {
		defer wg.Done()
		for i := range 200 {
			rib.Insert("10.0.0.1", &Route{
				MsgID:  uint64(i),
				Family: "ipv4/unicast",
				Prefix: fmt.Sprintf("10.0.%d.0/24", i%50),
			})
		}
	}()

	// Goroutine 2: inserts for peer 2 and periodically clears peer 1.
	go func() {
		defer wg.Done()
		for i := range 200 {
			rib.Insert("10.0.0.2", &Route{
				MsgID:  uint64(i),
				Family: "ipv4/unicast",
				Prefix: fmt.Sprintf("10.1.%d.0/24", i%50),
			})
			if i%50 == 0 {
				rib.ClearPeer("10.0.0.1")
			}
		}
	}()

	wg.Wait()

	// Peer 2 should have routes (not cleared).
	routes := rib.GetPeerRoutes("10.0.0.2")
	if len(routes) == 0 {
		t.Error("peer 2 should have routes")
	}
}

// TestRIB_Insert verifies route insertion and retrieval.
//
// VALIDATES: Routes are stored by peer and indexed by family+prefix.
// PREVENTS: Lost routes, incorrect peer isolation.
func TestRIB_Insert(t *testing.T) {
	rib := NewRIB()

	route := &Route{
		MsgID:  100,
		Family: "ipv4/unicast",
		Prefix: "10.0.0.0/24",
	}

	old := rib.Insert("10.0.0.1", route)
	if old != nil {
		t.Error("expected nil for new route")
	}

	routes := rib.GetPeerRoutes("10.0.0.1")
	if len(routes) != 1 {
		t.Fatalf("expected 1 route, got %d", len(routes))
	}
	if routes[0].MsgID != 100 {
		t.Errorf("expected MsgID 100, got %d", routes[0].MsgID)
	}
}

// TestRIB_InsertReplace verifies route replacement returns old route.
//
// VALIDATES: Same prefix replaces existing, returns old for cleanup.
// PREVENTS: Memory leaks, stale route references.
func TestRIB_InsertReplace(t *testing.T) {
	rib := NewRIB()

	route1 := &Route{MsgID: 100, Family: "ipv4/unicast", Prefix: "10.0.0.0/24"}
	route2 := &Route{MsgID: 200, Family: "ipv4/unicast", Prefix: "10.0.0.0/24"}

	rib.Insert("10.0.0.1", route1)
	old := rib.Insert("10.0.0.1", route2)

	if old == nil {
		t.Fatal("expected old route on replace")
		return
	}
	if old.MsgID != 100 {
		t.Errorf("expected old MsgID 100, got %d", old.MsgID)
	}

	routes := rib.GetPeerRoutes("10.0.0.1")
	if len(routes) != 1 {
		t.Fatalf("expected 1 route, got %d", len(routes))
	}
	if routes[0].MsgID != 200 {
		t.Errorf("expected new MsgID 200, got %d", routes[0].MsgID)
	}
}

// TestRIB_Remove verifies route removal.
//
// VALIDATES: Routes can be removed by peer+family+prefix.
// PREVENTS: Stale routes after withdrawal.
func TestRIB_Remove(t *testing.T) {
	rib := NewRIB()

	route := &Route{MsgID: 100, Family: "ipv4/unicast", Prefix: "10.0.0.0/24"}
	rib.Insert("10.0.0.1", route)

	removed := rib.Remove("10.0.0.1", "ipv4/unicast", "10.0.0.0/24")
	if removed == nil {
		t.Fatal("expected removed route")
		return
	}
	if removed.MsgID != 100 {
		t.Errorf("expected MsgID 100, got %d", removed.MsgID)
	}

	routes := rib.GetPeerRoutes("10.0.0.1")
	if len(routes) != 0 {
		t.Errorf("expected 0 routes after removal, got %d", len(routes))
	}
}

// TestRIB_ClearPeer verifies all routes from a peer are cleared.
//
// VALIDATES: Session teardown clears all peer routes.
// PREVENTS: Stale routes after peer down.
func TestRIB_ClearPeer(t *testing.T) {
	rib := NewRIB()

	rib.Insert("10.0.0.1", &Route{MsgID: 100, Family: "ipv4/unicast", Prefix: "10.0.0.0/24"})
	rib.Insert("10.0.0.1", &Route{MsgID: 101, Family: "ipv4/unicast", Prefix: "10.0.1.0/24"})
	rib.Insert("10.0.0.2", &Route{MsgID: 200, Family: "ipv4/unicast", Prefix: "10.0.0.0/24"})

	cleared := rib.ClearPeer("10.0.0.1")
	if len(cleared) != 2 {
		t.Errorf("expected 2 cleared routes, got %d", len(cleared))
	}

	// Peer 1 should be empty
	if len(rib.GetPeerRoutes("10.0.0.1")) != 0 {
		t.Error("peer 10.0.0.1 should have no routes")
	}

	// Peer 2 should be unaffected
	if len(rib.GetPeerRoutes("10.0.0.2")) != 1 {
		t.Error("peer 10.0.0.2 should still have 1 route")
	}
}

// TestRIB_GetAllPeers verifies iteration over all peers.
//
// VALIDATES: Replay on peer up can iterate all other peers' routes.
// PREVENTS: Missing routes during replay.
func TestRIB_GetAllPeers(t *testing.T) {
	rib := NewRIB()

	rib.Insert("10.0.0.1", &Route{MsgID: 100, Family: "ipv4/unicast", Prefix: "10.0.0.0/24"})
	rib.Insert("10.0.0.2", &Route{MsgID: 200, Family: "ipv4/unicast", Prefix: "10.0.1.0/24"})
	rib.Insert("10.0.0.2", &Route{MsgID: 201, Family: "ipv4/unicast", Prefix: "10.0.2.0/24"})

	all := rib.GetAllPeers()
	if len(all) != 2 {
		t.Errorf("expected 2 peers, got %d", len(all))
	}

	if len(all["10.0.0.1"]) != 1 {
		t.Errorf("expected 1 route from peer 1, got %d", len(all["10.0.0.1"]))
	}
	if len(all["10.0.0.2"]) != 2 {
		t.Errorf("expected 2 routes from peer 2, got %d", len(all["10.0.0.2"]))
	}
}

// TestRIB_PeerIsolation verifies routes are isolated per peer.
//
// VALIDATES: Same prefix from different peers stored separately.
// PREVENTS: Route collision between peers.
func TestRIB_PeerIsolation(t *testing.T) {
	rib := NewRIB()

	// Same prefix from two peers
	rib.Insert("10.0.0.1", &Route{MsgID: 100, Family: "ipv4/unicast", Prefix: "10.0.0.0/24"})
	rib.Insert("10.0.0.2", &Route{MsgID: 200, Family: "ipv4/unicast", Prefix: "10.0.0.0/24"})

	routes1 := rib.GetPeerRoutes("10.0.0.1")
	routes2 := rib.GetPeerRoutes("10.0.0.2")

	if len(routes1) != 1 || routes1[0].MsgID != 100 {
		t.Error("peer 1 route corrupted")
	}
	if len(routes2) != 1 || routes2[0].MsgID != 200 {
		t.Error("peer 2 route corrupted")
	}
}

// TestRIB_FamilyIsolation verifies different families stored separately.
//
// VALIDATES: Same prefix in different families are distinct routes.
// PREVENTS: IPv4/IPv6 collision.
func TestRIB_FamilyIsolation(t *testing.T) {
	rib := NewRIB()

	// Same prefix string, different families (contrived but valid)
	rib.Insert("10.0.0.1", &Route{MsgID: 100, Family: "ipv4/unicast", Prefix: "10.0.0.0/24"})
	rib.Insert("10.0.0.1", &Route{MsgID: 200, Family: "ipv6/unicast", Prefix: "10.0.0.0/24"})

	routes := rib.GetPeerRoutes("10.0.0.1")
	if len(routes) != 2 {
		t.Errorf("expected 2 routes (different families), got %d", len(routes))
	}
}
