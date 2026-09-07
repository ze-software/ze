// VALIDATES: the engine re-stamps a forked plugin's administrative distance from
// the declaration, taking the wire value as the fallback (AC-17, A-5); and the
// route-install entry carries the forwarding action and the equal-cost next-hop
// set, each with its device and share (A-8).
// PREVENTS: `rib { distance { <protocol> N } }` staying inert for every FORKED
// producer, because the seam is process-global and never reaches a subprocess;
// and a forked plugin's blackhole or multipath arriving as a plain single-next-hop
// route because the RPC entry had nowhere to carry either.

package server

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	ribdistance "github.com/ze-software/ze/internal/core/rib/distance"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/internal/core/rib/routetype"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func init() {
	redistevents.RegisterProtocol("test-restamp")
}

// declareDistance publishes one protocol's distance on the seam for the duration
// of the test, the way sysrib's publishDistances does at configure time.
func declareDistance(t *testing.T, protocol string, d uint8) {
	t.Helper()
	ribdistance.Set(func(p string) (uint8, bool) {
		if p == protocol {
			return d, true
		}
		return 0, false
	})
	t.Cleanup(func() { ribdistance.Set(nil) })
}

func installOne(t *testing.T, rib *locrib.RIB, entry rpc.RouteInstallEntry) locrib.Path {
	t.Helper()
	if _, err := applyRouteInstall(rib, rpc.RouteInstallInput{Routes: []rpc.RouteInstallEntry{entry}}); err != nil {
		t.Fatalf("applyRouteInstall: %v", err)
	}
	group, ok := rib.Lookup(v4u(), mustPfx(t, entry.Prefix))
	if !ok || group.Best < 0 {
		t.Fatalf("prefix %s is not in the engine Loc-RIB", entry.Prefix)
	}
	return group.Paths[group.Best]
}

// TestForkedProducerDistanceIsRestampedByTheEngine is AC-17. A forked producer
// never sees the declaration, so it ships its own bootstrap value; the engine is
// where the declaration lives and where the Path is rebuilt, so that is where the
// operator's number is applied.
func TestForkedProducerDistanceIsRestampedByTheEngine(t *testing.T) {
	declareDistance(t, "test-restamp", 5)
	path := installOne(t, locrib.NewRIB(), rpc.RouteInstallEntry{
		Protocol: "test-restamp", AFI: uint16(family.AFIIPv4), SAFI: uint8(family.SAFIUnicast),
		Prefix: "10.90.0.0/24", NextHop: "192.0.2.1",
		// The bootstrap value the forked producer stamped, having no declaration.
		AdminDistance: 110,
	})
	if path.AdminDistance != 5 {
		t.Errorf("AdminDistance = %d, want the declared 5, not the producer's bootstrap 110", path.AdminDistance)
	}
}

// TestForkedProducerKeepsItsOwnDistanceWhenUndeclared is R-8: a protocol the
// declaration does not name keeps what its producer chose, so the re-stamp never
// overwrites a value nobody declared.
func TestForkedProducerKeepsItsOwnDistanceWhenUndeclared(t *testing.T) {
	declareDistance(t, "some-other-protocol", 5)
	path := installOne(t, locrib.NewRIB(), rpc.RouteInstallEntry{
		Protocol: "test-restamp", AFI: uint16(family.AFIIPv4), SAFI: uint8(family.SAFIUnicast),
		Prefix: "10.91.0.0/24", NextHop: "192.0.2.1", AdminDistance: 110,
	})
	if path.AdminDistance != 110 {
		t.Errorf("AdminDistance = %d, want the wire value 110 for a protocol the declaration does not name", path.AdminDistance)
	}
}

// TestRouteInstallEntryCarriesRouteTypeAndECMP is A-8: a forked plugin's discard
// route and its multipath both survive the process boundary.
func TestRouteInstallEntryCarriesRouteTypeAndECMP(t *testing.T) {
	path := installOne(t, locrib.NewRIB(), rpc.RouteInstallEntry{
		Protocol: "test-restamp", AFI: uint16(family.AFIIPv4), SAFI: uint8(family.SAFIUnicast),
		Prefix: "10.92.0.0/24", NextHop: "192.0.2.1", Interface: "tun100", Weight: 3,
		RouteType: uint8(routetype.Blackhole), AdminDistance: 10,
		ECMP: []rpc.RouteNextHop{
			{NextHop: "192.0.2.2", Weight: 1},
			{Interface: "tun101", Weight: 2},
		},
	})

	if path.RouteType != routetype.Blackhole {
		t.Errorf("RouteType = %v, want blackhole", path.RouteType)
	}
	if path.Interface != "tun100" || path.Weight != 3 {
		t.Errorf("primary device/share = %q/%d, want tun100/3", path.Interface, path.Weight)
	}
	if len(path.ECMP) != 2 {
		t.Fatalf("ECMP = %+v, want two members", path.ECMP)
	}
	if path.ECMP[0].Addr != netip.MustParseAddr("192.0.2.2") || path.ECMP[0].Weight != 1 {
		t.Errorf("ECMP[0] = %+v, want 192.0.2.2 at weight 1", path.ECMP[0])
	}
	if path.ECMP[1].Interface != "tun101" || path.ECMP[1].Weight != 2 {
		t.Errorf("ECMP[1] = %+v, want tun101 at weight 2", path.ECMP[1])
	}
}

// TestRouteInstallRejectsAnEmptyECMPMember is the fail-closed half: a member that
// names neither a gateway nor a device is malformed input and fails the batch,
// rather than entering the Loc-RIB as a next-hop to nowhere.
func TestRouteInstallRejectsAnEmptyECMPMember(t *testing.T) {
	rib := locrib.NewRIB()
	_, err := applyRouteInstall(rib, rpc.RouteInstallInput{Routes: []rpc.RouteInstallEntry{{
		Protocol: "test-restamp", AFI: uint16(family.AFIIPv4), SAFI: uint8(family.SAFIUnicast),
		Prefix: "10.93.0.0/24", NextHop: "192.0.2.1", AdminDistance: 10,
		ECMP: []rpc.RouteNextHop{{Weight: 1}},
	}}})
	if err == nil {
		t.Fatal("an ecmp member naming neither a next-hop nor an interface must be refused")
	}
	if _, ok := rib.Lookup(v4u(), mustPfx(t, "10.93.0.0/24")); ok {
		t.Error("a refused batch must apply nothing")
	}
}
