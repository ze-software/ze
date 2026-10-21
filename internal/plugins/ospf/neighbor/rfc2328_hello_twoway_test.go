// Design: docs/architecture/ospf/README.md -- neighbor state machine
// Related: table.go -- hello and twoWayReceivedLocked, the RFC 2328 Section 10.5 dispatch.
//
// VALIDATES: RFC 2328 Section 10.5 through Table.Hello: a Hello that lists this router
// runs the neighbor state machine with 2-WayReceived, and a Hello that does not runs it
// with 1-WayReceived.
// PREVENTS: a neighbor reaching 2-Way, or starting an adjacency, before it has heard this
// router, and a neighbor staying in 2-Way after it stops listing this router.
package neighbor

import (
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// RFC requirement: RFC2328-10.5-2 positive -- a Hello that lists this router runs the neighbor state machine with 2-WayReceived: on a broadcast interface where this router is neither DR nor BDR, a neighbor in Init moves to 2-way, and on a point-to-point interface a neighbor heard for the first time moves through 2-Way into ExStart because an adjacency is wanted (hello and twoWayReceivedLocked, table.go).
func TestRFC2328HelloListingRouterRunsTwoWayReceived(t *testing.T) {
	tbl, cfg := testTable(t, types.NetworkBroadcast)
	peer := rid(t, "10.0.0.2")
	if reason := tbl.Hello(hello(cfg, peer, false, time.Unix(1, 0))); reason != "" {
		t.Fatalf("Hello not listing this router: %s", reason)
	}
	if reason := tbl.Hello(hello(cfg, peer, true, time.Unix(2, 0))); reason != "" {
		t.Fatalf("Hello listing this router: %s", reason)
	}
	snap, ok := tbl.Lookup(cfg.Name, peer)
	if !ok || snap.State != "2-way" {
		t.Fatalf("broadcast neighbor = %+v ok=%v, want 2-way after a Hello listing this router", snap, ok)
	}

	p2p, p2pCfg := testTable(t, types.NetworkPointToPoint)
	if reason := p2p.Hello(hello(p2pCfg, peer, true, time.Unix(1, 0))); reason != "" {
		t.Fatalf("point-to-point Hello listing this router: %s", reason)
	}
	snap, ok = p2p.Lookup(p2pCfg.Name, peer)
	if !ok || snap.State != "exstart" {
		t.Fatalf("point-to-point neighbor = %+v ok=%v, want exstart after a Hello listing this router", snap, ok)
	}
}

// RFC requirement: RFC2328-10.5-2 negative -- a Hello that does not list this router never runs 2-WayReceived: repeated Hellos leave a new neighbor in Init on a point-to-point interface, where 2-WayReceived would start an adjacency, and a neighbor already in 2-way that stops listing this router is taken back to Init by 1-WayReceived (hello, table.go).
func TestRFC2328HelloNotListingRouterRunsOneWayReceived(t *testing.T) {
	tbl, cfg := testTable(t, types.NetworkPointToPoint)
	peer := rid(t, "10.0.0.2")
	for i := range 3 {
		if reason := tbl.Hello(hello(cfg, peer, false, time.Unix(int64(i+1), 0))); reason != "" {
			t.Fatalf("Hello %d: %s", i, reason)
		}
		snap, ok := tbl.Lookup(cfg.Name, peer)
		if !ok || snap.State != "init" {
			t.Fatalf("after Hello %d not listing this router: neighbor = %+v ok=%v, want init", i, snap, ok)
		}
	}

	bcast, bcastCfg := testTable(t, types.NetworkBroadcast)
	if reason := bcast.Hello(hello(bcastCfg, peer, true, time.Unix(1, 0))); reason != "" {
		t.Fatalf("Hello listing this router: %s", reason)
	}
	if snap, _ := bcast.Lookup(bcastCfg.Name, peer); snap.State != "2-way" {
		t.Fatalf("setup: neighbor state = %s, want 2-way", snap.State)
	}
	if reason := bcast.Hello(hello(bcastCfg, peer, false, time.Unix(2, 0))); reason != "" {
		t.Fatalf("Hello no longer listing this router: %s", reason)
	}
	snap, ok := bcast.Lookup(bcastCfg.Name, peer)
	if !ok || snap.State != "init" {
		t.Fatalf("neighbor = %+v ok=%v, want init after a Hello that no longer lists this router", snap, ok)
	}
}
