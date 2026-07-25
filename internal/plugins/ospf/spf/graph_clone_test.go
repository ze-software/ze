// VALIDATES: graph.go Clone / excludeLink / excludeRouter -- the TI-LFA
// post-convergence graph transforms (RFC 5286). A Clone is a deep copy: mutating
// the clone (excludeLink for link protection, excludeRouter for node protection)
// drops exactly the protected edge/vertex in the clone while leaving the live
// graph the Computer retains byte-for-byte intact.
// PREVENTS: a repair SPF corrupting the delivered SPT by sharing per-vertex Links
// / AttachedRouters backing arrays with the live graph.
package spf

import (
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// twoRouterLANGraph builds Y(1.1.1.1) <-p2p-> X(2.2.2.2), Y also owning a stub,
// and a transit network 10.0.0.254 with both routers attached.
func twoRouterLANGraph(t *testing.T) (*Graph, types.RouterID, types.RouterID, types.LinkStateID) {
	t.Helper()
	y := testRID(t, "1.1.1.1")
	x := testRID(t, "2.2.2.2")
	nw := testLSID(t, "10.0.0.254")
	g := NewGraph(testArea())
	g.Routers[y] = &RouterVertex{ID: y, Links: []packet.RouterLink{
		p2pLink(t, "2.2.2.2", "10.0.0.1", 10),
		stubLink(t, "198.51.100.0", 7),
	}}
	g.Routers[x] = &RouterVertex{ID: x, Links: []packet.RouterLink{
		p2pLink(t, "1.1.1.1", "10.0.0.2", 10),
	}}
	g.Networks[nw] = &NetworkVertex{
		ID:              nw,
		AdvertisingDR:   x,
		NetworkMask:     testIP(t, "255.255.255.0"),
		AttachedRouters: []types.RouterID{y, x},
	}
	return g, y, x, nw
}

func hasP2PTo(v *RouterVertex, to types.RouterID) bool {
	if v == nil {
		return false
	}
	want := linkStateIDFromRouterID(to)
	for _, l := range v.Links {
		if l.Type == packet.RouterLinkTypeP2P && l.LinkID == want {
			return true
		}
	}
	return false
}

func attachedContains(n *NetworkVertex, r types.RouterID) bool {
	if n == nil {
		return false
	}
	return slices.Contains(n.AttachedRouters, r)
}

func TestGraphCloneIsolatesExcludeRouter(t *testing.T) {
	g, y, x, nw := twoRouterLANGraph(t)

	clone := g.Clone()
	clone.excludeRouter(x)

	// The clone dropped X entirely: the vertex, Y's p2p link to X, and X's LAN
	// membership.
	if _, ok := clone.Routers[x]; ok {
		t.Fatalf("clone still has excluded router X")
	}
	if hasP2PTo(clone.Routers[y], x) {
		t.Fatalf("clone still has Y's p2p link to excluded X")
	}
	if attachedContains(clone.Networks[nw], x) {
		t.Fatalf("clone network still lists excluded X as attached")
	}
	// Y itself and its stub survive; only the link to X went.
	if cy := clone.Routers[y]; cy == nil || len(cy.Links) != 1 || cy.Links[0].Type != packet.RouterLinkTypeStub {
		t.Fatalf("clone Y links = %+v, want the single stub link", clone.Routers[y])
	}
	if !attachedContains(clone.Networks[nw], y) {
		t.Fatalf("clone network dropped Y, which was not excluded")
	}

	// The live graph is untouched: X present, Y's link to X present, LAN membership intact.
	if _, ok := g.Routers[x]; !ok {
		t.Fatalf("excludeRouter on the clone deleted X from the live graph")
	}
	if !hasP2PTo(g.Routers[y], x) {
		t.Fatalf("excludeRouter on the clone dropped Y->X from the live graph")
	}
	if gy := g.Routers[y]; gy == nil || len(gy.Links) != 2 {
		t.Fatalf("live Y links = %+v, want 2 (p2p + stub)", g.Routers[y])
	}
	if !attachedContains(g.Networks[nw], x) {
		t.Fatalf("excludeRouter on the clone dropped X from the live LAN membership")
	}
}

func TestGraphCloneIsolatesExcludeLink(t *testing.T) {
	g, y, x, _ := twoRouterLANGraph(t)

	clone := g.Clone()
	clone.excludeLink(y, x)

	// Both directions of the Y<->X adjacency are gone in the clone.
	if hasP2PTo(clone.Routers[y], x) {
		t.Fatalf("clone kept Y->X after excludeLink")
	}
	if hasP2PTo(clone.Routers[x], y) {
		t.Fatalf("clone kept X->Y after excludeLink")
	}
	// The live graph keeps both directions.
	if !hasP2PTo(g.Routers[y], x) || !hasP2PTo(g.Routers[x], y) {
		t.Fatalf("excludeLink on the clone mutated the live graph adjacency")
	}
}

// TestExcludeRouterSkipsNilVertices covers the defensive nil-vertex guards in
// excludeRouter: a nil router/network map entry is skipped, never dereferenced,
// while the target router is still removed everywhere it is real.
func TestExcludeRouterSkipsNilVertices(t *testing.T) {
	g, y, x, nw := twoRouterLANGraph(t)
	g.Routers[testRID(t, "7.7.7.7")] = nil
	g.Networks[testLSID(t, "10.7.7.254")] = nil

	g.excludeRouter(x) // must not panic on the nil entries

	if _, ok := g.Routers[x]; ok {
		t.Fatalf("excludeRouter left X in the router table")
	}
	if hasP2PTo(g.Routers[y], x) {
		t.Fatalf("excludeRouter left Y's p2p link to X")
	}
	if attachedContains(g.Networks[nw], x) {
		t.Fatalf("excludeRouter left X attached to the LAN")
	}
}

// TestGraphCloneSkipsNilVertices covers the defensive nil-vertex guards in Clone:
// a nil map entry is skipped, never copied and never nil-dereferenced.
func TestGraphCloneSkipsNilVertices(t *testing.T) {
	g := NewGraph(testArea())
	live := testRID(t, "1.1.1.1")
	g.Routers[live] = &RouterVertex{ID: live, Links: []packet.RouterLink{stubLink(t, "203.0.113.0", 3)}}
	g.Routers[testRID(t, "9.9.9.9")] = nil
	g.Networks[testLSID(t, "10.9.9.254")] = nil

	clone := g.Clone()
	if len(clone.Routers) != 1 {
		t.Fatalf("clone routers = %d, want 1 (nil entry skipped)", len(clone.Routers))
	}
	if _, ok := clone.Routers[testRID(t, "9.9.9.9")]; ok {
		t.Fatalf("clone copied a nil router vertex")
	}
	if len(clone.Networks) != 0 {
		t.Fatalf("clone networks = %d, want 0 (nil entry skipped)", len(clone.Networks))
	}
	if cr := clone.Routers[live]; cr == nil || len(cr.Links) != 1 {
		t.Fatalf("clone dropped the real router vertex: %+v", clone.Routers[live])
	}
}
