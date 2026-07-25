// VALIDATES: RFC 5286 Section 6.3 OSPF multi-area LFA suppression (AC-14, A-10,
// A-14): intra-area routes are always eligible; inter-area routes are suppressed
// with more than one alternate ABR; external routes with more than one ASBR; the
// backbone with virtual links suppresses inter-area/external LFAs.
// PREVENTS: an inter-area/external micro-loop where the real path leaves the area.
package spf

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

func TestLFASuppressedMultiAreaLeakage(t *testing.T) {
	area1, err := types.ParseAreaID("0.0.0.1")
	if err != nil {
		t.Fatalf("ParseAreaID: %v", err)
	}
	backbone := types.BackboneArea
	cases := []struct {
		name string
		rt   RouteType
		area types.AreaID
		abr  int
		asbr int
		vl   bool
		want bool
	}{
		{"intra-area always safe", RouteIntraArea, area1, 5, 5, true, false},
		{"inter-area single ABR eligible", RouteInterArea, area1, 1, 0, false, false},
		{"inter-area multi ABR suppressed", RouteInterArea, area1, 2, 0, false, true},
		{"external single ASBR eligible", RouteExternalType1, area1, 1, 1, false, false},
		{"external multi ASBR suppressed", RouteExternalType2, area1, 1, 2, false, true},
		{"backbone virtual-link suppresses inter-area", RouteInterArea, backbone, 1, 0, true, true},
		{"backbone virtual-link suppresses external", RouteExternalType1, backbone, 1, 1, true, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := suppressLFA(c.rt, c.area, c.abr, c.asbr, c.vl); got != c.want {
				t.Fatalf("suppressLFA(%v, abr=%d, asbr=%d, vl=%v) = %v, want %v", c.rt, c.abr, c.asbr, c.vl, got, c.want)
			}
		})
	}
}

func TestLFAEligibilityByRouteType(t *testing.T) {
	// A-14: RouteType gates eligibility end-to-end. On the triangle topology the
	// intra-area stub gets a backup; an inter-area route advertised by two ABRs in
	// the same area is suppressed (no backup).
	area := testArea()
	g := BuildGraph(triangleAltSource(t), area)
	root := testRID(t, "1.1.1.1")
	res := Compute(g, root, 8)
	routes := BuildRoutes(res, 8, nil) // the intra-area stub 192.0.2.0/24
	// An inter-area route whose advertising ABR (Origin) is a real intra-area
	// router (E), so routeVertex resolves, but two ABRs exist in the area.
	interAreaR := RouteEntry{
		AreaID:   area,
		Prefix:   netip.MustParsePrefix("198.51.100.0/24"),
		Metric:   30,
		Type:     RouteInterArea,
		Origin:   testRID(t, "2.2.2.2"),
		NextHops: []NextHop{{Addr: netip.MustParseAddr("10.0.12.2"), Router: testRID(t, "2.2.2.2")}},
	}
	routes = append(routes, interAreaR)
	border := []BorderRouterEntry{
		{RouterID: testRID(t, "2.2.2.2"), AreaID: area, Kind: BorderRouterABR},
		{RouterID: testRID(t, "3.3.3.3"), AreaID: area, Kind: BorderRouterABR},
	}
	attachAllBackups(routes, fastRerouteInput{
		root: root, maxPaths: 8, nh: v4NextHop{},
		results: map[types.AreaID]*Result{area: res},
		graphs:  map[types.AreaID]*Graph{area: g},
		border:  border,
		cfg:     FastRerouteConfig{Enabled: true, NodeProtection: true},
	})

	intra, _ := backupFor(routes, "192.0.2.0/24")
	if len(intra.Backups) == 0 || !intra.Backups[0].Valid() {
		t.Fatalf("intra-area prefix should be protected, got %+v", intra.Backups)
	}
	inter, _ := backupFor(routes, "198.51.100.0/24")
	if len(inter.Backups) != 0 {
		t.Fatalf("inter-area prefix with two ABRs should be suppressed, got %+v", inter.Backups)
	}
}
