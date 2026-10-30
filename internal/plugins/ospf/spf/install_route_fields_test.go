// Design: docs/architecture/rib/unified-locrib.md -- the fields a Loc-RIB import filter reads.
//
// VALIDATES: an OSPFv2 route reaches the Loc-RIB carrying its path type
// (intra-area, inter-area, external-1, external-2, nssa-1, nssa-2) and, for an
// AS-external or NSSA route, the external route tag its LSA carried, a zero tag
// included (ComputeExternal, v4ExternalReader, ribRouteType, Installer.insert).
// PREVENTS: a Loc-RIB `match { route-type [ nssa-2 ]; }` or `match { tag [ 7 ]; }`
// filter misjudging OSPF routes because the producer dropped or confused a field.

package spf

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/rib/locrib"
	ribpolicy "github.com/ze-software/ze/internal/core/rib/policy"
	ospflsdb "github.com/ze-software/ze/internal/plugins/ospf/lsdb"
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// TestOSPFInstallCarriesRouteTypeAndTag computes the external routes of an
// LSDB holding two Type 5 and two Type 7 LSAs, installs them beside one
// intra-area and one inter-area route, and reads every path back. Method: each
// route has its own prefix, type and tag, so a producer that swapped two types,
// lost the NSSA origin, or read the tag from the wrong LSA fails a named row.
func TestOSPFInstallCarriesRouteTypeAndTag(t *testing.T) {
	root := testRID(t, "1.1.1.1")
	nssa := areaID(t, "0.0.0.5")

	e1 := externalLSA(t, "10.50.0.0", "2.2.2.2", false, 5, "0.0.0.0")
	e1.External.ExternalRouteTag = 7
	e2 := externalLSA(t, "10.51.0.0", "2.2.2.2", true, 5, "0.0.0.0")
	n2 := type7LSA(t, "10.52.0.0", "255.255.255.0", "3.3.3.3", 5, true)
	n2.External.ExternalRouteTag = 9
	n1 := type7LSA(t, "10.53.0.0", "255.255.255.0", "3.3.3.3", 5, true)
	n1.External.ExternalType2 = false
	n1.External.ExternalRouteTag = 11

	db := ospflsdb.New(nil)
	for _, lsa := range []struct {
		name string
		area types.AreaID
		lsa  packet.LSA
	}{
		{"e1", types.BackboneArea, e1},
		{"e2", types.BackboneArea, e2},
		{"n2", nssa, n2},
		{"n1", nssa, n1},
	} {
		if !db.Install(lsa.area, lsa.lsa) {
			t.Fatalf("LSDB refused LSA %s", lsa.name)
		}
	}

	routesToFA := []RouteEntry{
		{AreaID: nssa, Prefix: netip.MustParsePrefix("192.168.0.0/24"), Metric: 3, Type: RouteIntraArea, NextHops: []NextHop{{Addr: netip.MustParseAddr("10.0.0.8")}}},
	}
	border := []BorderRouterEntry{asbrBorder(t, "2.2.2.2", 3, "10.0.0.9"), nssaASBR(t, nssa, "3.3.3.3")}
	externals := ComputeExternal(ExternalInput{Source: db, Root: root, BorderRouters: border, Routes: routesToFA, NSSAAreas: []types.AreaID{nssa}})
	if len(externals) != 4 {
		t.Fatalf("external routes = %+v, want 4 (two Type 5, two Type 7)", externals)
	}

	routes := append([]RouteEntry{
		{AreaID: testArea(), Prefix: netip.MustParsePrefix("10.54.0.0/24"), Metric: 2, Type: RouteIntraArea, NextHops: []NextHop{{Addr: netip.MustParseAddr("10.0.0.2")}}},
		{AreaID: testArea(), Prefix: netip.MustParsePrefix("10.55.0.0/24"), Metric: 2, Type: RouteInterArea, NextHops: []NextHop{{Addr: netip.MustParseAddr("10.0.0.2")}}},
	}, externals...)
	loc := locrib.NewRIB()
	NewInstaller(loc).Apply(routes)

	for _, tc := range []struct {
		prefix string
		kind   ribpolicy.OSPFRouteType
		tag    uint32
		hasTag bool
	}{
		{"10.50.0.0/24", ribpolicy.OSPFExternal1, 7, true},
		// RFC 2328 Section A.4.5: every AS-external-LSA carries the tag field, so
		// a zero tag is a tag.
		{"10.51.0.0/24", ribpolicy.OSPFExternal2, 0, true},
		{"10.52.0.0/24", ribpolicy.OSPFNSSA2, 9, true},
		{"10.53.0.0/24", ribpolicy.OSPFNSSA1, 11, true},
		{"10.54.0.0/24", ribpolicy.OSPFIntraArea, 0, false},
		{"10.55.0.0/24", ribpolicy.OSPFInterArea, 0, false},
	} {
		paths := lookupPaths(t, loc, netip.MustParsePrefix(tc.prefix))
		if len(paths) != 1 {
			t.Fatalf("%s: %d paths, want 1", tc.prefix, len(paths))
		}
		p := paths[0]
		if p.OSPFRouteType != tc.kind {
			t.Errorf("%s: route type = %d, want %d", tc.prefix, p.OSPFRouteType, tc.kind)
		}
		if p.HasTag != tc.hasTag {
			t.Errorf("%s: has tag = %v, want %v", tc.prefix, p.HasTag, tc.hasTag)
		}
		if p.Tag != tc.tag {
			t.Errorf("%s: tag = %d, want %d", tc.prefix, p.Tag, tc.tag)
		}
	}
}
