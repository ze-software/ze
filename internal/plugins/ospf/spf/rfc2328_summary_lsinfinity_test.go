package spf

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// VALIDATES: RFC 2328 Section 16.2 (1), a summary-LSA whose advertised cost is LSInfinity is
// not used by the inter-area calculation.
// PREVENTS: an unreachable destination announced with cost LSInfinity installed as a route.

// TestRFC2328SummaryAdvertisingLSInfinitySkipped proves the inter-area calculation skips a
// summary-LSA whose OWN cost field is LSInfinity, with a cheap path to its ABR. RFC 2328
// Section 16.2: "If the cost specified by the LSA is LSInfinity, or if the LSA's LS age is
// equal to MaxAge, then examine the the next LSA."
//
// Goal: the LSInfinity clause on the advertised cost, which the composed-cost test
// (TestOSPFInterAreaLSInfinityDropped) reaches only by saturating the sum.
// Method: one ABR 2.2.2.2 at cost 10; the same summary-LSA shape with cost 20 installs a
// route, and with cost LSInfinity installs none.
func TestRFC2328SummaryAdvertisingLSInfinitySkipped(t *testing.T) {
	root := testRID(t, "1.1.1.1")
	abr := testRID(t, "2.2.2.2")
	area := types.BackboneArea
	compute := func(cost uint32) int {
		res := resultWithRouter(area, root, abr, 10, netip.MustParseAddr("10.0.0.2"), 0)
		src := testSource(t, area, summaryNetworkLSA(t, "10.20.0.0", "2.2.2.2", cost))
		routes, _ := ComputeInterArea(InterAreaInput{Source: src, Root: root, Areas: []types.AreaID{area},
			Results: map[types.AreaID]*Result{area: res}, MaxPaths: 8})
		return len(routes)
	}

	// RFC requirement: RFC2328-16.2-2 positive -- a summary-LSA advertising cost 20 through
	// an ABR at cost 10 is used: one inter-area route is installed (ComputeInterAreaWith).
	if n := compute(20); n != 1 {
		t.Fatalf("summary-LSA with cost 20 installed %d routes, want 1", n)
	}
	// RFC requirement: RFC2328-16.2-2 negative -- the same summary-LSA advertising cost
	// LSInfinity is skipped although its ABR is 10 away: no inter-area route is installed.
	if n := compute(uint32(LSInfinity)); n != 0 {
		t.Fatalf("summary-LSA advertising LSInfinity installed %d routes, want none", n)
	}
}
