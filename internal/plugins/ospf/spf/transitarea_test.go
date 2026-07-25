// VALIDATES: spec-ospf-ext-7 -- the shared transit-area side of OSPF virtual links: reading
// a virtual neighbor's reachability/cost/next hop from the transit area's intra-area SPF
// result (RFC 2328 sec 16.1), RFC 2328 sec 16.3 TransitCapability + the improve-only /
// resolve-or-discard-virtual-next-hop pass, RFC 5340 sec 3.5 backbone attachment, and the
// no-flap resolution cache.
package spf

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// fakeTransitStrategy embeds v4Strategy but returns canned inter-area summaries so the RFC
// 2328 sec 16.3 pass can be exercised without hand-building Summary-LSAs.
type fakeTransitStrategy struct {
	v4Strategy
	summaries map[types.AreaID][]InterAreaSummary
}

func (s fakeTransitStrategy) SummaryReader(_ Source) SummaryReader {
	return func(area types.AreaID) []InterAreaSummary { return s.summaries[area] }
}

func TestVirtualNeighborResolvedFromTransitSPF(t *testing.T) {
	transit := areaID(t, "0.0.0.1")
	root := testRID(t, "1.1.1.1")
	neighbor := testRID(t, "9.9.9.9")
	nh := netip.MustParseAddr("10.1.0.2")
	res := resultWithRouter(transit, root, neighbor, 30, nh, 0)
	vr := resolveVirtualNeighbor(res, neighbor)
	if !vr.Reachable || vr.Cost != 30 {
		t.Fatalf("resolve = %+v, want reachable cost 30", vr)
	}
	if len(vr.NextHops) != 1 || vr.NextHops[0].Addr != nh {
		t.Fatalf("next hops = %+v, want [%s]", vr.NextHops, nh)
	}
}

func TestVirtualLinkNeighborUnreachableStaysDown(t *testing.T) {
	transit := areaID(t, "0.0.0.1")
	root := testRID(t, "1.1.1.1")
	neighbor := testRID(t, "9.9.9.9")
	g := NewGraph(transit)
	g.Routers[root] = &RouterVertex{ID: root}
	res := &Result{Area: transit, Root: root, Graph: g, Nodes: map[VertexID]*NodeResult{routerVertex(root): {ID: routerVertex(root)}}}
	if resolveVirtualNeighbor(res, neighbor).Reachable {
		t.Fatalf("neighbor absent from the transit tree must be down")
	}
	if resolveVirtualNeighbor(nil, neighbor).Reachable {
		t.Fatalf("nil transit result must be down")
	}
}

func TestTransitCapabilitySetByVBit(t *testing.T) {
	area := areaID(t, "0.0.0.1")
	root := testRID(t, "1.1.1.1")
	endpoint := testRID(t, "2.2.2.2")
	nh := netip.MustParseAddr("10.1.0.2")
	if !TransitCapability(resultWithRouter(area, root, endpoint, 10, nh, packet.RouterFlagV)) {
		t.Fatalf("TransitCapability false with a V-bit Router-LSA present")
	}
	if TransitCapability(resultWithRouter(area, root, endpoint, 10, nh, 0)) {
		t.Fatalf("TransitCapability true with no V-bit Router-LSA")
	}
}

func TestVirtualNextHopResolvedOrDiscarded(t *testing.T) {
	root := testRID(t, "1.1.1.1")
	neighbor := testRID(t, "9.9.9.9")
	virtualAddr := netip.MustParseAddr("172.16.0.2") // the unresolved backbone next hop
	transitAddr := netip.MustParseAddr("10.1.0.2")   // the real transit next hop
	dest := netip.MustParsePrefix("10.5.0.0/24")
	backbone := resultWithRouter(types.BackboneArea, root, neighbor, 20, virtualAddr, 0)
	results := map[types.AreaID]*Result{types.BackboneArea: backbone}
	cands := []RouteEntry{{AreaID: types.BackboneArea, Prefix: dest, Metric: 25, Type: RouteInterArea, Origin: neighbor, NextHops: []NextHop{{Addr: virtualAddr}}}}

	c := NewComputer(Config{})
	resolved := []VirtualNeighborResult{{TransitArea: areaID(t, "0.0.0.1"), Neighbor: neighbor, Reachable: true, Cost: 20, NextHops: []NextHop{{Addr: transitAddr, Interface: "eth1"}}}}
	out := c.transitAreaPass(results, cands, resolved, 8)
	if len(out) != 1 || len(out[0].NextHops) != 1 || out[0].NextHops[0].Addr != transitAddr {
		t.Fatalf("virtual next hop not rewritten to the transit next hop: %+v", out)
	}

	// When the neighbor is unreachable in the transit area, the route is discarded rather
	// than installed with the unroutable virtual next hop.
	down := []VirtualNeighborResult{{TransitArea: areaID(t, "0.0.0.1"), Neighbor: neighbor, Reachable: false}}
	if out := c.transitAreaPass(results, cands, down, 8); len(out) != 0 {
		t.Fatalf("route with an unresolvable virtual next hop was not discarded: %+v", out)
	}
}

func TestTransitAreaPassOnlyImprovesReachable(t *testing.T) {
	transit := areaID(t, "0.0.0.1")
	root := testRID(t, "1.1.1.1")
	abr := testRID(t, "2.2.2.2")
	abrNH := netip.MustParseAddr("10.1.0.2")
	res := resultWithRouter(transit, root, abr, 5, abrNH, packet.RouterFlagV)
	results := map[types.AreaID]*Result{transit: res}

	reachablePfx := netip.MustParsePrefix("10.0.0.0/24")
	newPfx := netip.MustParsePrefix("10.9.9.0/24")
	summaries := map[types.AreaID][]InterAreaSummary{transit: {
		{AdvertisingRouter: abr, Metric: 3, Prefix: reachablePfx}, // already reachable: 5+3=8 improves 100
		{AdvertisingRouter: abr, Metric: 1, Prefix: newPfx},       // not already reachable: must NOT be added
	}}
	c := NewComputer(Config{Strategy: fakeTransitStrategy{summaries: summaries}})
	cands := []RouteEntry{{AreaID: types.BackboneArea, Prefix: reachablePfx, Metric: 100, Type: RouteInterArea, Origin: abr, NextHops: []NextHop{{Addr: netip.MustParseAddr("10.0.0.254")}}}}
	out := c.transitAreaPass(results, cands, nil, 8)

	var improved, added bool
	for _, r := range out {
		if r.Prefix == reachablePfx && r.Metric == 8 {
			improved = true
		}
		if r.Prefix == newPfx {
			added = true
		}
	}
	if !improved {
		t.Fatalf("already-reachable prefix not improved to cost 8: %+v", out)
	}
	if added {
		t.Fatalf("transit pass added an unreachable destination (violates improve-only)")
	}
}

func TestVirtualLinkCostUpdateNoFlap(t *testing.T) {
	c := NewComputer(Config{})
	transit := areaID(t, "0.0.0.1")
	neighbor := testRID(t, "9.9.9.9")
	vr := VirtualNeighborResult{TransitArea: transit, Neighbor: neighbor, Reachable: true, Cost: 10, NextHops: []NextHop{{Addr: netip.MustParseAddr("10.1.0.2")}}}
	if _, changed := c.updateVirtualLocked([]VirtualNeighborResult{vr}); !changed {
		t.Fatalf("first resolution should register as changed")
	}
	if _, changed := c.updateVirtualLocked([]VirtualNeighborResult{vr}); changed {
		t.Fatalf("an unchanged resolution flapped the virtual link")
	}
}

func TestVirtualLinkCostTracksTransitTopology(t *testing.T) {
	c := NewComputer(Config{})
	transit := areaID(t, "0.0.0.1")
	neighbor := testRID(t, "9.9.9.9")
	nh := []NextHop{{Addr: netip.MustParseAddr("10.1.0.2")}}
	c.updateVirtualLocked([]VirtualNeighborResult{{TransitArea: transit, Neighbor: neighbor, Reachable: true, Cost: 10, NextHops: nh}})
	if _, changed := c.updateVirtualLocked([]VirtualNeighborResult{{TransitArea: transit, Neighbor: neighbor, Reachable: true, Cost: 20, NextHops: nh}}); !changed {
		t.Fatalf("a transit cost change must update the virtual link")
	}
}

func TestVirtualLinkBackboneAttachment(t *testing.T) {
	root := testRID(t, "1.1.1.1")
	neighbor := testRID(t, "9.9.9.9")
	g := NewGraph(types.BackboneArea)
	g.Routers[root] = &RouterVertex{ID: root, Links: []packet.RouterLink{{Type: packet.RouterLinkTypeVirtual, LinkID: types.LinkStateID(neighbor)}}}
	results := map[types.AreaID]*Result{types.BackboneArea: {Area: types.BackboneArea, Root: root, Graph: g, Nodes: map[VertexID]*NodeResult{routerVertex(root): {ID: routerVertex(root)}}}}
	if !rootVirtualBackboneAttached(results, root) {
		t.Fatalf("root with a backbone virtual link record must be backbone-attached (RFC 5340 sec 3.5)")
	}
	// A not-Full link withdraws the record, so the endpoint is no longer backbone-attached.
	g.Routers[root].Links = nil
	if rootVirtualBackboneAttached(results, root) {
		t.Fatalf("root with no virtual link record must not be backbone-attached")
	}
}
