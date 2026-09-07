// VALIDATES: a MAIN-table static route becomes one locrib.Path carrying the
// declared administrative distance, the operator's next-hop set with its weights
// and devices, and the forwarding action; a NAMED-table route reaches the data
// plane directly and never enters the Loc-RIB.
// PREVENTS: `rib { distance { static N } }` staying inert because the producer
// stamps a constant; a weighted or device-named next-hop losing its weight or its
// device on the way to the FIB; and a named-table route colliding in a Loc-RIB
// that has no table dimension.

package static

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	ribdistance "github.com/ze-software/ze/internal/core/rib/distance"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/internal/core/rib/nexthop"
	"github.com/ze-software/ze/internal/core/rib/routetype"
	staticevents "github.com/ze-software/ze/internal/plugins/static/events"
)

// declareStaticDistance publishes d as the operator's `rib { distance { static } }`
// for the duration of the test, the way sysrib's publishDistances does at
// configure time, and clears the seam afterwards so the next test starts unset.
func declareStaticDistance(t *testing.T, d uint8) {
	t.Helper()
	ribdistance.Set(func(protocol string) (uint8, bool) {
		if protocol == "static" {
			return d, true
		}
		return 0, false
	})
	t.Cleanup(func() { ribdistance.Set(nil) })
}

func TestStaticApplyInsertsAPathPerRoute(t *testing.T) {
	mb := &mockStaticBackend{}
	rm := newTestRouteManager(mb)

	if err := rm.applyRoutes([]staticRoute{
		fwd("10.0.0.0/8", "192.0.2.1"),
		{Prefix: netip.MustParsePrefix("198.51.100.0/24"), Action: actionBlackhole},
	}); err != nil {
		t.Fatalf("applyRoutes: %v", err)
	}

	if len(mb.paths) != 2 {
		t.Fatalf("inserted %d Paths, want one per route", len(mb.paths))
	}
	if len(mb.applied) != 2 {
		t.Fatalf("installed %d routes, want 2", len(mb.applied))
	}
}

func TestStaticPathCarriesTheRegisteredSource(t *testing.T) {
	path, err := staticPath(fwd("10.0.0.0/8", "192.0.2.1"))
	if err != nil {
		t.Fatalf("staticPath: %v", err)
	}
	if path.Source != staticevents.ProtocolID {
		t.Errorf("Source = %d, want the registered static protocol id %d", path.Source, staticevents.ProtocolID)
	}
	if got := redistevents.ProtocolName(path.Source); got != "static" {
		t.Errorf("the Path's source resolves to %q, want \"static\"", got)
	}
}

// TestStaticStampsTheDeclaredDistance is the producer half of AC-8 and AC-9: the
// number the operator writes is the number selectBest ranks on, and it is read at
// insert so a reload takes effect on the next apply.
func TestStaticStampsTheDeclaredDistance(t *testing.T) {
	for _, declared := range []uint8{5, 250} {
		declareStaticDistance(t, declared)
		path, err := staticPath(fwd("10.0.0.0/8", "192.0.2.1"))
		if err != nil {
			t.Fatalf("staticPath: %v", err)
		}
		if path.AdminDistance != declared {
			t.Errorf("AdminDistance = %d, want the declared %d", path.AdminDistance, declared)
		}
	}
}

// TestStaticBootstrapDistanceAppliesBeforeTheDeclaration pins the other half of
// the seam contract: with nothing published, the producer uses its own constant
// rather than a zero, which would be the best distance any route can hold.
func TestStaticBootstrapDistanceAppliesBeforeTheDeclaration(t *testing.T) {
	ribdistance.Set(nil)
	path, err := staticPath(fwd("10.0.0.0/8", "192.0.2.1"))
	if err != nil {
		t.Fatalf("staticPath: %v", err)
	}
	if path.AdminDistance != DefaultAdminDistance {
		t.Errorf("AdminDistance = %d, want the bootstrap %d", path.AdminDistance, DefaultAdminDistance)
	}
}

// TestStaticLosesToEBGPAtARaisedDistance drives the arbitration itself: with the
// static distance raised above eBGP's, the eBGP path is the one the Loc-RIB hands
// the FIB. This is AC-8 at the point the decision is made.
func TestStaticLosesToEBGPAtARaisedDistance(t *testing.T) {
	declareStaticDistance(t, 250)
	best := bestOfStaticAndEBGP(t, 20)
	if redistevents.ProtocolName(best.Source) != "bgp" {
		t.Errorf("best source = %q, want bgp: static at 250 must lose to eBGP at 20",
			redistevents.ProtocolName(best.Source))
	}
}

// TestStaticBeatsEBGPAtALoweredDistance is the reverse of the row above (AC-9).
func TestStaticBeatsEBGPAtALoweredDistance(t *testing.T) {
	declareStaticDistance(t, 5)
	best := bestOfStaticAndEBGP(t, 20)
	if redistevents.ProtocolName(best.Source) != "static" {
		t.Errorf("best source = %q, want static: static at 5 must beat eBGP at 20",
			redistevents.ProtocolName(best.Source))
	}
}

// bestOfStaticAndEBGP inserts a static and an eBGP path for one prefix into a
// fresh Loc-RIB and returns the winner.
func bestOfStaticAndEBGP(t *testing.T, ebgpDistance uint8) locrib.Path {
	t.Helper()
	pfx := netip.MustParsePrefix("10.0.0.0/8")
	rib := locrib.NewRIB()

	path, err := staticPath(fwd(pfx.String(), "192.0.2.1"))
	if err != nil {
		t.Fatalf("staticPath: %v", err)
	}
	rib.InsertForward(family.IPv4Unicast, pfx, path, nil)
	rib.InsertForward(family.IPv4Unicast, pfx, locrib.Path{
		Source:        redistevents.RegisterProtocol("bgp"),
		NextHop:       netip.MustParseAddr("198.51.100.1"),
		AdminDistance: ebgpDistance,
		IsEBGP:        true,
	}, nil)

	group, found := rib.Lookup(family.IPv4Unicast, pfx)
	if !found || group.Best < 0 {
		t.Fatal("the Loc-RIB holds no best path for the prefix")
	}
	return group.Paths[group.Best]
}

// TestNamedTableStaticRouteNeverReachesTheLocRIB guards the main-table boundary
// (AC-10, R-5). The Loc-RIB is keyed by (family, prefix) with no table, so a
// named-table route inserted there would collide with the main-table route for
// the same prefix and one of the two would be lost.
func TestNamedTableStaticRouteNeverReachesTheLocRIB(t *testing.T) {
	mb := &mockStaticBackend{}
	rm := newTestRouteManager(mb)

	named := fwd("10.0.0.0/8", "192.0.2.1")
	named.Table = 100
	if err := rm.applyRoutes([]staticRoute{named}); err != nil {
		t.Fatalf("applyRoutes: %v", err)
	}

	if len(mb.paths) != 0 {
		t.Errorf("a named-table route produced %d Loc-RIB Paths, want none", len(mb.paths))
	}
	if len(mb.applied) != 1 {
		t.Fatalf("the named-table route reached the data plane %d times, want 1", len(mb.applied))
	}
}

func TestStaticBlackholeCarriesTheRouteType(t *testing.T) {
	for _, tc := range []struct {
		action actionType
		want   routetype.Type
	}{
		{actionBlackhole, routetype.Blackhole},
		{actionReject, routetype.Unreachable},
		{actionForward, routetype.Unicast},
	} {
		route := fwd("10.0.0.0/8", "192.0.2.1")
		route.Action = tc.action
		path, err := staticPath(route)
		if err != nil {
			t.Fatalf("staticPath for %s: %v", tc.action, err)
		}
		if path.RouteType != tc.want {
			t.Errorf("%s: RouteType = %v, want %v", tc.action, path.RouteType, tc.want)
		}
		if tc.want.Discards() && path.NextHop.IsValid() {
			t.Errorf("%s: a discard route must name no next-hop, got %v", tc.action, path.NextHop)
		}
	}
}

// TestStaticPathCarriesWeightedNextHops is AC-12 at the producer: the route's own
// next-hop takes the first member's weight and the rest become the equal-cost
// group, each with its own.
func TestStaticPathCarriesWeightedNextHops(t *testing.T) {
	route := staticRoute{
		Prefix: netip.MustParsePrefix("10.0.0.0/8"),
		Action: actionForward,
		NextHops: []nextHop{
			{Address: netip.MustParseAddr("192.0.2.1"), Weight: 3},
			{Address: netip.MustParseAddr("192.0.2.2"), Weight: 1},
		},
	}
	path, err := staticPath(route)
	if err != nil {
		t.Fatalf("staticPath: %v", err)
	}
	if path.NextHop != netip.MustParseAddr("192.0.2.1") || path.Weight != 3 {
		t.Errorf("primary = %v weight %d, want 192.0.2.1 weight 3", path.NextHop, path.Weight)
	}
	want := []nexthop.NextHop{{Addr: netip.MustParseAddr("192.0.2.2"), Weight: 1}}
	if len(path.ECMP) != 1 || path.ECMP[0] != want[0] {
		t.Errorf("ECMP = %+v, want %+v", path.ECMP, want)
	}
}

// TestStaticWeightIsCappedAtWhatTheDataPlaneCanCarry pins the boundary: the YANG
// range runs to 65535, and both data planes carry a share in one octet.
func TestStaticWeightIsCappedAtWhatTheDataPlaneCanCarry(t *testing.T) {
	route := fwd("10.0.0.0/8", "192.0.2.1")
	route.NextHops[0].Weight = 65535
	path, err := staticPath(route)
	if err != nil {
		t.Fatalf("staticPath: %v", err)
	}
	if path.Weight != 255 {
		t.Errorf("Weight = %d, want 255: a larger share than a data plane can express is capped, not wrapped", path.Weight)
	}
}

// TestStaticRefusesAnUnresolvableNextHopBeforeInsert is AC-15 and A-6: a route
// the plugin cannot resolve is refused where the operator can still be told,
// before any Path exists.
func TestStaticRefusesAnUnresolvableNextHopBeforeInsert(t *testing.T) {
	if _, err := staticPath(ifaceNextHop("10.0.0.0/8")); err == nil {
		t.Fatal("a next-hop naming a device no backend can resolve must be refused")
	}
	route := fwd("10.0.0.0/8", "192.0.2.1")
	route.NextHops = []nextHop{{}}
	if _, err := staticPath(route); err == nil {
		t.Error("a next-hop naming neither an address nor a device must be refused")
	}
}

// TestStaticBFDDownReinsertsTheSurvivingNextHop is AC-18: a next-hop BFD reports
// down leaves the group, and the route is re-inserted with the survivor rather
// than withdrawn.
func TestStaticBFDDownReinsertsTheSurvivingNextHop(t *testing.T) {
	mb := &mockStaticBackend{}
	rm := newTestRouteManager(mb)

	route := staticRoute{
		Prefix: netip.MustParsePrefix("10.0.0.0/8"),
		Action: actionForward,
		NextHops: []nextHop{
			{Address: netip.MustParseAddr("192.0.2.1"), Weight: 1},
			{Address: netip.MustParseAddr("192.0.2.2"), Weight: 1},
		},
	}
	if err := rm.applyRoutes([]staticRoute{route}); err != nil {
		t.Fatalf("applyRoutes: %v", err)
	}
	if len(mb.paths) != 1 || len(mb.paths[0].ECMP) != 1 {
		t.Fatalf("setup: want one Path with one equal-cost sibling, got %+v", mb.paths)
	}

	// The first next-hop goes down, the way watchBFD marks it.
	rm.mu.Lock()
	rs := rm.routes[routeKey{prefix: route.Prefix}]
	rs.nhStates[0].active = false
	err := rm.programRouteLocked(rs)
	rm.mu.Unlock()
	if err != nil {
		t.Fatalf("reprogram after BFD down: %v", err)
	}

	if len(mb.paths) != 2 {
		t.Fatalf("BFD down produced %d Paths in total, want a second one", len(mb.paths))
	}
	survivor := mb.paths[1]
	if survivor.NextHop != netip.MustParseAddr("192.0.2.2") {
		t.Errorf("re-inserted next-hop = %v, want the surviving 192.0.2.2", survivor.NextHop)
	}
	if len(survivor.ECMP) != 0 {
		t.Errorf("re-inserted ECMP = %+v, want none: only one next-hop survives", survivor.ECMP)
	}
}

// TestStaticWithNoInstallDestinationRefuses is the fail-closed half of the sink
// wiring: with neither a shared Loc-RIB nor a route-install channel the route has
// nowhere to go, and the manager says so instead of reporting a route it never
// installed.
func TestStaticWithNoInstallDestinationRefuses(t *testing.T) {
	rm := newRouteManager(&mockStaticBackend{})
	if err := rm.applyProgrammed(fwd("10.0.0.0/8", "192.0.2.1")); err == nil {
		t.Error("a main-table route with no install destination must be refused")
	}
}
