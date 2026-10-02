// VALIDATES: RFC 2328 Section 16.4 path-type preference: intra-area and inter-area paths
// beat AS-external paths whatever the metric, and among type 2 external paths the
// smallest advertised type 2 metric wins even over a closer ASBR.
// PREVENTS: an inter-area route losing to a cheaper external, and a type 2 choice made
// on the distance to the ASBR instead of the advertised metric.
package spf

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

func pathPreferenceCandidates(t *testing.T) (RouteEntry, RouteEntry, RouteEntry) {
	t.Helper()
	pfx := netip.MustParsePrefix("10.10.0.0/16")
	inter := RouteEntry{AreaID: testArea(), Prefix: pfx, Metric: 100, Type: RouteInterArea, Origin: testRID(t, "2.2.2.2"), NextHops: []NextHop{{Addr: netip.MustParseAddr("10.0.0.2")}}}
	e1 := RouteEntry{AreaID: testArea(), Prefix: pfx, Metric: 1, Type: RouteExternalType1, Origin: testRID(t, "3.3.3.3"), NextHops: []NextHop{{Addr: netip.MustParseAddr("10.0.0.3")}}}
	e2 := RouteEntry{AreaID: testArea(), Prefix: pfx, Metric: 1, Type: RouteExternalType2, Origin: testRID(t, "4.4.4.4"), NextHops: []NextHop{{Addr: netip.MustParseAddr("10.0.0.4")}}}
	return inter, e1, e2
}

// e2ByMetric computes 10.98.0.0 from two type 2 externals: 2.2.2.2 advertises 7 and is
// 50 away, 3.3.3.3 advertises 9 and is 1 away.
func e2ByMetric(t *testing.T) []RouteEntry {
	t.Helper()
	src := testSource(t, types.BackboneArea,
		externalLSA(t, "10.98.0.0", "2.2.2.2", true, 7, "0.0.0.0"),
		externalLSA(t, "10.98.0.0", "3.3.3.3", true, 9, "0.0.0.0"),
	)
	border := []BorderRouterEntry{asbrBorder(t, "2.2.2.2", 50, "10.0.0.2"), asbrBorder(t, "3.3.3.3", 1, "10.0.0.3")}
	return ComputeExternal(ExternalInput{Source: src, Root: testRID(t, "1.1.1.1"), BorderRouters: border, MaxPaths: 8})
}

// RFC requirement: RFC2328-16.4-1 positive -- the preferred path is selected: an inter-area path at cost 100 is chosen over a type 1 external at cost 1 and over a type 2 external at cost 1 (selectBestRoutes, route.go), and among type 2 externals the one with the smallest advertised type 2 metric (7, via 2.2.2.2 at distance 50) is chosen over metric 9 via an ASBR at distance 1 (ComputeExternal, external.go).
func TestRFC2328PreferredPathTypeSelected(t *testing.T) {
	inter, e1, e2 := pathPreferenceCandidates(t)
	for _, ext := range []RouteEntry{e1, e2} {
		got := selectBestRoutes([]RouteEntry{ext, inter}, 8)
		if len(got) != 1 || got[0].Type != RouteInterArea || got[0].Metric != 100 {
			t.Fatalf("inter-area vs %v: selected %+v, want the inter-area path at 100", ext.Type, got)
		}
	}
	out := e2ByMetric(t)
	if len(out) != 1 || out[0].Type != RouteExternalType2 || out[0].Metric != 7 {
		t.Fatalf("type 2 externals: %+v, want one route at advertised metric 7", out)
	}
}

// RFC requirement: RFC2328-16.4-1 negative -- the cheaper path of a lower preference is refused: the type 1 and type 2 externals at cost 1 never appear beside or instead of the inter-area path, and the type 2 path with the larger advertised metric contributes no next hop even though its ASBR is 49 closer (selectBestRoutes, route.go; ComputeExternal, external.go).
func TestRFC2328LowerPreferencePathRefused(t *testing.T) {
	inter, e1, e2 := pathPreferenceCandidates(t)
	for _, ext := range []RouteEntry{e1, e2} {
		for _, r := range selectBestRoutes([]RouteEntry{inter, ext}, 8) {
			for _, nh := range r.NextHops {
				if nh.Addr == ext.NextHops[0].Addr {
					t.Fatalf("the %v path at cost 1 was selected against the inter-area path: %+v", ext.Type, r)
				}
			}
		}
	}
	for _, r := range e2ByMetric(t) {
		for _, nh := range r.NextHops {
			if nh.Addr != netip.MustParseAddr("10.0.0.2") {
				t.Fatalf("type 2 next hop %v, want only 10.0.0.2: the larger advertised metric 9 must lose", nh.Addr)
			}
		}
	}
}
