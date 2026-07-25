// VALIDATES: computer.go RouterReachable (RFC 5250 Section 5 Type-11 opaque
// reachability gate: root, reachable border router, or origin of an installed
// route), the BorderRouterSnapshot method render, spf.go compareVertexID ordering,
// and Trigger arming a throttled backbone SPF run.
// PREVENTS: honoring an unreachable originator's opaque LSAs, a mis-rendered
// border-router row, an unstable vertex tie-break, and a Trigger that never runs.
package spf

import (
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

func TestRouterReachable(t *testing.T) {
	root := testRID(t, "1.1.1.1")
	reachableABR := testRID(t, "2.2.2.2")
	costedOutABR := testRID(t, "3.3.3.3")
	noHopABR := testRID(t, "4.4.4.4")
	routeOrigin := testRID(t, "5.5.5.5")
	danglingOrigin := testRID(t, "6.6.6.6")
	unknown := testRID(t, "8.8.8.8")
	hop := []NextHop{{Addr: netip.MustParseAddr("10.0.0.2")}}

	c := &Computer{
		root: root,
		lastBorder: []BorderRouterEntry{
			{RouterID: reachableABR, AreaID: testArea(), Kind: BorderRouterABR, Metric: 10, NextHops: hop},
			{RouterID: costedOutABR, AreaID: testArea(), Kind: BorderRouterABR, Metric: LSInfinity, NextHops: hop},
			{RouterID: noHopABR, AreaID: testArea(), Kind: BorderRouterABR, Metric: 10, NextHops: nil},
		},
		last: []RouteEntry{
			{Prefix: netip.MustParsePrefix("192.0.2.0/24"), Origin: routeOrigin, NextHops: hop},
			{Prefix: netip.MustParsePrefix("198.51.100.0/24"), Origin: danglingOrigin, NextHops: nil},
		},
	}

	cases := []struct {
		name string
		id   types.RouterID
		want bool
	}{
		{"zero-router-id", types.RouterID{}, false},
		{"local-root-always-reachable", root, true},
		{"border-finite-metric-with-hops", reachableABR, true},
		{"border-costed-out-LSInfinity", costedOutABR, false},
		{"border-no-next-hops", noHopABR, false},
		{"origin-of-installed-route", routeOrigin, true},
		{"origin-with-no-next-hops", danglingOrigin, false},
		{"unknown-router", unknown, false},
	}
	for _, tc := range cases {
		if got := c.RouterReachable(tc.id); got != tc.want {
			t.Fatalf("%s: RouterReachable(%s) = %v, want %v", tc.name, tc.id, got, tc.want)
		}
	}
}

func TestComputerBorderRouterSnapshot(t *testing.T) {
	c := &Computer{lastBorder: []BorderRouterEntry{
		{
			RouterID: testRID(t, "2.2.2.2"), AreaID: testArea(), Kind: BorderRouterABR, Metric: 20,
			NextHops: []NextHop{{Addr: netip.MustParseAddr("10.0.0.2"), Interface: "eth0"}},
		},
	}}
	snap := c.BorderRouterSnapshot()
	if len(snap) != 1 {
		t.Fatalf("border-router snapshot rows = %d, want 1", len(snap))
	}
	row := snap[0]
	if row.RouterID != "2.2.2.2" || row.Kind != "abr" || row.Metric != 20 {
		t.Fatalf("border-router row = %+v", row)
	}
	if len(row.NextHops) != 1 || row.NextHops[0].NextHop != "10.0.0.2" || row.NextHops[0].Interface != "eth0" {
		t.Fatalf("border-router next-hops = %+v", row.NextHops)
	}
}

func TestCompareVertexID(t *testing.T) {
	rLow := routerVertex(testRID(t, "1.1.1.1"))
	rHigh := routerVertex(testRID(t, "2.2.2.2"))
	nLow := networkVertex(testLSID(t, "10.0.0.1"))
	nHigh := networkVertex(testLSID(t, "10.0.0.2"))

	// A router vertex sorts before a network vertex (Kind ordering), symmetrically.
	if compareVertexID(rLow, nLow) != -1 {
		t.Fatalf("router vs network = %d, want -1", compareVertexID(rLow, nLow))
	}
	if compareVertexID(nLow, rLow) != 1 {
		t.Fatalf("network vs router = %d, want 1", compareVertexID(nLow, rLow))
	}
	// Same kind, router: compare by Router ID.
	if compareVertexID(rLow, rHigh) != -1 || compareVertexID(rHigh, rLow) != 1 || compareVertexID(rLow, rLow) != 0 {
		t.Fatalf("router-vs-router ordering wrong: %d %d %d",
			compareVertexID(rLow, rHigh), compareVertexID(rHigh, rLow), compareVertexID(rLow, rLow))
	}
	// Same kind, network: compare by Network Link State ID.
	if compareVertexID(nLow, nHigh) != -1 || compareVertexID(nHigh, nLow) != 1 || compareVertexID(nHigh, nHigh) != 0 {
		t.Fatalf("network-vs-network ordering wrong: %d %d %d",
			compareVertexID(nLow, nHigh), compareVertexID(nHigh, nLow), compareVertexID(nHigh, nHigh))
	}
}

func TestSetMaxPathsClamp(t *testing.T) {
	c := NewComputer(Config{Areas: []types.AreaID{testArea()}})
	c.SetMaxPaths(3)
	if c.maxPaths != 3 {
		t.Fatalf("SetMaxPaths(3) => %d, want 3", c.maxPaths)
	}
	// A non-positive cap falls back to the committed ECMP default, never 0.
	c.SetMaxPaths(0)
	if c.maxPaths != DefaultMaxPaths {
		t.Fatalf("SetMaxPaths(0) => %d, want DefaultMaxPaths %d", c.maxPaths, DefaultMaxPaths)
	}
	c.SetMaxPaths(-5)
	if c.maxPaths != DefaultMaxPaths {
		t.Fatalf("SetMaxPaths(-5) => %d, want DefaultMaxPaths %d", c.maxPaths, DefaultMaxPaths)
	}
}

func TestSetTimersNormalisation(t *testing.T) {
	c := NewComputer(Config{Areas: []types.AreaID{testArea()}})

	// Zero everywhere resolves to the SPF throttle defaults.
	c.SetTimers(0, 0, 0)
	if c.delay != DefaultSPFDelay || c.hold != DefaultSPFHold || c.maxHold != DefaultSPFMaxHold {
		t.Fatalf("SetTimers(0,0,0) => delay=%v hold=%v maxHold=%v", c.delay, c.hold, c.maxHold)
	}

	// hold < delay is raised to delay, and maxHold < hold is raised to hold, so the
	// throttle window is always monotonic delay <= hold <= maxHold.
	c.SetTimers(time.Second, 100*time.Millisecond, 50*time.Millisecond)
	if c.delay != time.Second || c.hold != time.Second || c.maxHold != time.Second {
		t.Fatalf("SetTimers(1s,100ms,50ms) => delay=%v hold=%v maxHold=%v, want all 1s", c.delay, c.hold, c.maxHold)
	}
}

type fakeTimer struct{}

func (fakeTimer) Stop() bool { return true }

func TestTriggerArmsBackboneRun(t *testing.T) {
	area := testArea()
	loc := locrib.NewRIB()
	db := baseP2PSource(t, area)
	c := NewComputer(Config{Source: db, Root: testRID(t, "1.1.1.1"), Areas: []types.AreaID{area}, Installer: NewInstaller(loc)})

	var captured func()
	c.afterFunc = func(_ time.Duration, f func()) timerHandle {
		captured = f
		return fakeTimer{}
	}

	if c.runCount() != 0 {
		t.Fatalf("fresh Computer already ran SPF: runCount=%d", c.runCount())
	}
	c.Trigger()

	if _, ok := c.dirty[types.BackboneArea]; !ok {
		t.Fatalf("Trigger did not mark the backbone area dirty")
	}
	if !c.pending {
		t.Fatalf("Trigger did not arm a pending run")
	}
	if captured == nil {
		t.Fatalf("Trigger scheduled no throttled run function")
	}

	// Fire the throttle timer: the armed run computes SPF over the P2P+stub topology.
	captured()
	if c.runCount() != 1 {
		t.Fatalf("armed run did not compute SPF: runCount=%d, want 1", c.runCount())
	}
	if len(c.Routes()) == 0 {
		t.Fatalf("armed run installed no routes over the P2P+stub topology")
	}
}
