// Design: docs/architecture/ospf/ospf-ext-7-virtual-links.md -- the virtual link cost is
// the transit area's intra-area path cost.
// Related: transitarea.go -- resolveVirtualNeighbor.
//
// VALIDATES: RFC 2328 Section 15: "It is defined to be the cost of the intra-area path
// between the two defining area border routers. This cost appears in the virtual link's
// corresponding routing table entry."
// PREVENTS: a virtual link cost that does not follow the transit path.
package spf

import (
	"testing"
)

// RFC requirement: RFC2328-15-2 positive -- the virtual link's cost is the cost of the
// intra-area path through the Transit area to the other endpoint, the cost of that
// endpoint's router entry in the transit area's SPF result: a two-hop path costing 10+20
// resolves to cost 30, and raising the second hop to 45 resolves to 55 with no configured
// input (resolveVirtualNeighbor).
// Goal: the cost follows the transit path, so a change in the path changes the cost.
// Method: compute the transit SPF twice over router-LSAs that differ in one link cost.
func TestRFC2328VirtualCostIsTransitPathCost(t *testing.T) {
	transit := areaID(t, "0.0.0.1")
	root := testRID(t, "1.1.1.1")
	far := testRID(t, "9.9.9.9")
	for _, tc := range []struct {
		name string
		hop  uint16
		want uint64
	}{
		{name: "hop-20", hop: 20, want: 30},
		{name: "hop-45", hop: 45, want: 55},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src := testSource(t, transit,
				routerLSA(t, "1.1.1.1", p2pLink(t, "2.2.2.2", "10.0.0.1", 10)),
				routerLSA(t, "2.2.2.2", p2pLink(t, "1.1.1.1", "10.0.0.2", 10), p2pLink(t, "9.9.9.9", "10.0.1.2", tc.hop)),
				routerLSA(t, "9.9.9.9", p2pLink(t, "2.2.2.2", "10.0.1.9", tc.hop)),
			)
			res := Compute(BuildGraph(src, transit), root, DefaultMaxPaths)
			entry := res.Nodes[routerVertex(far)]
			if entry == nil || entry.Metric != tc.want {
				t.Fatalf("transit router entry for %s = %+v, want metric %d", far, entry, tc.want)
			}
			vr := resolveVirtualNeighbor(res, far)
			if !vr.Reachable || vr.Cost != entry.Metric {
				t.Fatalf("virtual neighbor = %+v, want reachable at the router entry's cost %d", vr, entry.Metric)
			}
		})
	}
}
