package rr

import (
	"testing"
)

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
