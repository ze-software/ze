// VALIDATES: RFC 2328 Section 16.2: the backbone-only restriction on summary-LSAs binds
// only a router attached to multiple areas; a router attached to one non-backbone area
// examines that area's summary-LSAs.
// PREVENTS: a producer that examines only backbone summaries for every router; the
// positive TestOSPFInterAreaRoute uses a backbone-only router and cannot see it.
package spf

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// RFC requirement: RFC2328-16.2-1 positive -- a router attached only to area 0.0.0.1 (not an area border router) computes the inter-area route from that area's summary-LSA: 10.40.0.0/24 advertised at 1 by ABR 3.3.3.3, reached at 1, is installed at 2 with origin 3.3.3.3 (ComputeInterAreaWith, interarea.go).
func TestRFC2328NonABRUsesItsAreaSummaries(t *testing.T) {
	// Goal: the "if the router is attached to multiple areas" condition.
	// Method: ComputeInterArea with one non-backbone area in Areas.
	root := testRID(t, "1.1.1.1")
	area1 := areaID(t, "0.0.0.1")
	abr1 := testRID(t, "3.3.3.3")
	results := map[types.AreaID]*Result{
		area1: resultWithRouter(area1, root, abr1, 1, netip.MustParseAddr("10.1.0.3"), 0),
	}
	src := testSource(t, area1, summaryNetworkLSA(t, "10.40.0.0", "3.3.3.3", 1))
	routes, _ := ComputeInterArea(InterAreaInput{Source: src, Root: root, Areas: []types.AreaID{area1}, Results: results, MaxPaths: 8})
	if len(routes) != 1 {
		t.Fatalf("routes = %+v, want the one inter-area route from area 0.0.0.1's summary", routes)
	}
	if routes[0].Metric != 2 || routes[0].Origin != abr1 || routes[0].Type != RouteInterArea {
		t.Fatalf("route = %+v, want inter-area metric 2 via 3.3.3.3", routes[0])
	}
}
