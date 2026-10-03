package spf

import (
	"net/netip"
	"testing"
)

// RFC requirement: RFC2328-16.4-1 positive -- an intra-area path is preferred over an AS-external path regardless of metric: the intra-area route at cost 100 wins over the external at cost 1 (routeTypeRank/routeBetter, route.go:52-65, 273-279).
func TestOSPFRouteTablePreference(t *testing.T) {
	pfx := netip.MustParsePrefix("10.10.0.0/16")
	intra := RouteEntry{AreaID: testArea(), Prefix: pfx, Metric: 100, Type: RouteIntraArea, Origin: testRID(t, "2.2.2.2"), NextHops: []NextHop{{Addr: netip.MustParseAddr("10.0.0.2")}}}
	external := RouteEntry{AreaID: testArea(), Prefix: pfx, Metric: 1, Type: RouteExternalType2, Origin: testRID(t, "3.3.3.3"), NextHops: []NextHop{{Addr: netip.MustParseAddr("10.0.0.3")}}}
	got := selectBestRoutes([]RouteEntry{external, intra}, 8)
	if len(got) != 1 || got[0].Type != RouteIntraArea || got[0].Metric != 100 {
		t.Fatalf("selected = %+v, want intra-area despite higher metric", got)
	}
}

func TestOSPFRouteDiff(t *testing.T) {
	pfxA := netip.MustParsePrefix("10.0.0.0/24")
	pfxB := netip.MustParsePrefix("10.0.1.0/24")
	pfxC := netip.MustParsePrefix("10.0.2.0/24")
	prev := map[netip.Prefix]RouteEntry{
		pfxA: {Prefix: pfxA, Metric: 10, Type: RouteIntraArea, Origin: testRID(t, "2.2.2.2"), NextHops: []NextHop{{Addr: netip.MustParseAddr("10.0.0.2")}}},
		pfxB: {Prefix: pfxB, Metric: 20, Type: RouteIntraArea, Origin: testRID(t, "2.2.2.2"), NextHops: []NextHop{{Addr: netip.MustParseAddr("10.0.0.2")}}},
	}
	cur := map[netip.Prefix]RouteEntry{
		pfxB: {Prefix: pfxB, Metric: 21, Type: RouteIntraArea, Origin: testRID(t, "2.2.2.2"), NextHops: []NextHop{{Addr: netip.MustParseAddr("10.0.0.2")}}},
		pfxC: {Prefix: pfxC, Metric: 5, Type: RouteIntraArea, Origin: testRID(t, "3.3.3.3"), NextHops: []NextHop{{Addr: netip.MustParseAddr("10.0.0.3")}}},
	}
	d := DiffRoutes(prev, cur)
	if len(d.Added) != 1 || d.Added[0].Prefix != pfxC {
		t.Fatalf("Added = %+v, want %s", d.Added, pfxC)
	}
	if len(d.Changed) != 1 || d.Changed[0].Prefix != pfxB {
		t.Fatalf("Changed = %+v, want %s", d.Changed, pfxB)
	}
	if len(d.Removed) != 1 || d.Removed[0] != pfxA {
		t.Fatalf("Removed = %+v, want %s", d.Removed, pfxA)
	}
}

func TestOSPFRouteECMPCap(t *testing.T) {
	pfx := netip.MustParsePrefix("10.20.0.0/16")
	var candidates []RouteEntry
	for i := 1; i <= 9; i++ {
		addr := netip.MustParseAddr("10.0.0." + string(rune('0'+i)))
		candidates = append(candidates, RouteEntry{Prefix: pfx, Metric: 10, Type: RouteIntraArea, Origin: testRID(t, "2.2.2.2"), NextHops: []NextHop{{Addr: addr}}})
	}
	got := selectBestRoutes(candidates, 8)
	if len(got) != 1 || len(got[0].NextHops) != 8 {
		t.Fatalf("ECMP cap selected %+v, want 8 next-hops", got)
	}
}
