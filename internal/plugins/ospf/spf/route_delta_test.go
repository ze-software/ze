// VALIDATES: route.go RouteDelta.Empty, routeEqual next-hop sensitivity, the RFC
// 5286 Section 3.6 Backup.Class rendering for every protection class, and the
// RouteType name/rank tables (RFC 2328 Section 16.4 preference order).
// PREVENTS: a next-hop change that fails to re-install, a mislabelled protection
// class in `show`, and an out-of-range RouteType rendering as a real type.
package spf

import (
	"net/netip"
	"testing"
)

func TestRouteDeltaEmpty(t *testing.T) {
	if !(RouteDelta{}).Empty() {
		t.Fatalf("zero RouteDelta is not Empty()")
	}
	pfx := netip.MustParsePrefix("10.0.0.0/24")
	r := RouteEntry{Prefix: pfx, Metric: 10, Type: RouteIntraArea}
	if (RouteDelta{Added: []RouteEntry{r}}).Empty() {
		t.Fatalf("delta with an Added route reported Empty()")
	}
	if (RouteDelta{Changed: []RouteEntry{r}}).Empty() {
		t.Fatalf("delta with a Changed route reported Empty()")
	}
	if (RouteDelta{Removed: []netip.Prefix{pfx}}).Empty() {
		t.Fatalf("delta with a Removed prefix reported Empty()")
	}
}

func TestRouteEqualNextHopSensitivity(t *testing.T) {
	pfx := netip.MustParsePrefix("10.0.0.0/24")
	base := RouteEntry{
		AreaID: testArea(), Prefix: pfx, Metric: 10, Type: RouteIntraArea, Origin: testRID(t, "2.2.2.2"),
		NextHops: []NextHop{{Addr: netip.MustParseAddr("10.0.0.2")}},
	}

	// Identical routes are equal.
	if !routeEqual(base, base) {
		t.Fatalf("routeEqual said two identical routes differ")
	}

	// A different next-hop count re-installs.
	twoHops := base
	twoHops.NextHops = []NextHop{{Addr: netip.MustParseAddr("10.0.0.2")}, {Addr: netip.MustParseAddr("10.0.0.3")}}
	if routeEqual(base, twoHops) {
		t.Fatalf("routeEqual ignored a next-hop COUNT change")
	}

	// A same-count but different next-hop ADDRESS re-installs (traffic must follow the moved path).
	moved := base
	moved.NextHops = []NextHop{{Addr: netip.MustParseAddr("10.0.0.9")}}
	if routeEqual(base, moved) {
		t.Fatalf("routeEqual ignored a next-hop ADDRESS change")
	}

	// DiffRoutes surfaces the address change as a Changed route.
	d := DiffRoutes(IndexByPrefix([]RouteEntry{base}), IndexByPrefix([]RouteEntry{moved}))
	if len(d.Changed) != 1 || d.Changed[0].NextHops[0].Addr != netip.MustParseAddr("10.0.0.9") {
		t.Fatalf("next-hop change not surfaced as Changed: %+v", d)
	}
}

func TestBackupClassRendering(t *testing.T) {
	nh := netip.MustParseAddr("10.0.0.3")
	cases := []struct {
		name string
		b    Backup
		want string
	}{
		{"node+link", Backup{Kind: BackupLFA, NextHop: nh, NodeProtect: true, LinkProtect: true}, "node+link"},
		{"node", Backup{Kind: BackupLFA, NextHop: nh, NodeProtect: true}, "node"},
		{"link", Backup{Kind: BackupLFA, NextHop: nh, LinkProtect: true}, "link"},
		{"downstream", Backup{Kind: BackupLFA, NextHop: nh, Downstream: true}, "downstream"},
		{"loop-free", Backup{Kind: BackupLFA, NextHop: nh}, "loop-free"},
		{"invalid-none", Backup{Kind: BackupNone, NextHop: nh}, ""},
		{"invalid-noaddr", Backup{Kind: BackupLFA}, ""},
	}
	for _, tc := range cases {
		if got := tc.b.Class(); got != tc.want {
			t.Fatalf("%s: Class() = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestRouteTypeNamesAndRanks(t *testing.T) {
	// String and routeTypeRank must agree across all four real types plus the
	// out-of-range fallback (which must never masquerade as a real type).
	cases := []struct {
		rt   RouteType
		name string
		rank int
	}{
		{RouteIntraArea, "intra-area", 0},
		{RouteInterArea, "inter-area", 1},
		{RouteExternalType1, "external-type-1", 2},
		{RouteExternalType2, "external-type-2", 3},
		{RouteType(0), "unknown", 4},
		{RouteType(99), "unknown", 4},
	}
	for _, tc := range cases {
		if got := tc.rt.String(); got != tc.name {
			t.Fatalf("RouteType(%d).String() = %q, want %q", tc.rt, got, tc.name)
		}
		if got := routeTypeRank(tc.rt); got != tc.rank {
			t.Fatalf("routeTypeRank(%d) = %d, want %d", tc.rt, got, tc.rank)
		}
	}

	// The rank order encodes RFC 2328 Section 16.4 preference: intra < inter < ext1 < ext2.
	ranks := []int{
		routeTypeRank(RouteIntraArea),
		routeTypeRank(RouteInterArea),
		routeTypeRank(RouteExternalType1),
		routeTypeRank(RouteExternalType2),
	}
	for i := 1; i < len(ranks); i++ {
		if ranks[i-1] >= ranks[i] {
			t.Fatalf("route type ranks not strictly increasing by RFC 2328 preference: %v", ranks)
		}
	}
}
