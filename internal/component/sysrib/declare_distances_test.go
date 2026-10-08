// Design: docs/architecture/core-design.md -- System RIB plugin, administrative distance.
package sysrib

import (
	"maps"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	ribdistance "github.com/ze-software/ze/internal/core/rib/distance"
	"github.com/ze-software/ze/internal/core/rib/locrib"
)

// Every distance table sysrib takes on re-ranks the paths already installed.
//
// Method: a static route and an eBGP route hold one prefix in the Loc-RIB under
// `static 5`. declareDistances, the one path the startup seed, stage-2
// configure, apply and rollback all take, then declares `static 250`: the
// installed eBGP route must win with no producer re-sending anything. Before
// stage-2 configure went through it, that site published without re-ranking, so
// a path inserted before it stayed ranked at the schema default.
func TestDeclareDistancesReranksInstalledPaths(t *testing.T) {
	loc := locrib.NewRIB()
	SetLocRIB(loc)
	t.Cleanup(func() { SetLocRIB(nil) })
	t.Cleanup(func() { ribdistance.Set(nil) })
	s := newSysRIB()

	declared, err := parseAdminDistanceConfig("{}")
	if err != nil {
		t.Fatalf("schema defaults: %v", err)
	}
	low := maps.Clone(declared)
	low["static"] = 5
	s.declareDistances(low)

	prefix := netip.MustParsePrefix("10.0.0.0/8")
	static := redistevents.RegisterProtocol("static")
	bgp := redistevents.RegisterProtocol("bgp")
	loc.Insert(family.IPv4Unicast, prefix, locrib.Path{Source: static, NextHop: netip.MustParseAddr("192.0.2.1")})
	loc.Insert(family.IPv4Unicast, prefix, locrib.Path{Source: bgp, NextHop: netip.MustParseAddr("198.51.100.1"), IsBGP: true, IsEBGP: true})
	if best, _ := loc.Best(family.IPv4Unicast, prefix); best.Source != static {
		t.Fatalf("under static 5 the winner is %s, want static", redistevents.ProtocolName(best.Source))
	}

	high := maps.Clone(declared)
	high["static"] = 250
	s.declareDistances(high)
	best, ok := loc.Best(family.IPv4Unicast, prefix)
	if !ok {
		t.Fatal("the prefix has no best path")
	}
	if best.Source != bgp {
		t.Fatalf("after declaring static 250 the winner is %s, want bgp", redistevents.ProtocolName(best.Source))
	}
	if best.AdminDistance != 20 {
		t.Fatalf("the eBGP winner ranks at %d, want the declared 20", best.AdminDistance)
	}
}
