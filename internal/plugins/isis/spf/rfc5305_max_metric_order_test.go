// Design: docs/architecture/isis/isis-9-spf-rib.md -- route arbitration over the SPF results.
//
// VALIDATES: a prefix advertised with a metric LARGER than MAX_PATH_METRIC
// (0xFE000000) takes no part in route selection, IPv4 (RFC 5305 sec 4) and IPv6
// (RFC 5308 sec 2): a finite advertisement of the same prefix from another
// router is installed alone, at its own cost, with only its own next hop. And
// the RFC 5308 sec 5 four-step order of preference between paths holds for
// every adjacent pair, whatever the metrics.
// PREVENTS: an over-max advertisement being clamped into a usable path, joining
// an ECMP set, or displacing a finite route; and any pair of the four classes
// being ranked by metric instead of by class.

package spf

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/plugins/isis/types"
)

// maxMetricFixture returns one level-1 graph and result in which nodes 2 and 3
// are both reachable at distance 10 from root 1, each through itself.
func maxMetricFixture() (*Graph, *Result, *Node, *Node) {
	root := sysID(1)
	g := NewGraph()
	g.node(types.NewSourceID(root, 0))
	over := g.node(srcID(2))
	finite := g.node(srcID(3))
	res := &Result{
		Root:  root,
		Level: Level1,
		Nodes: map[types.SourceID]*NodeResult{
			srcID(2): {ID: srcID(2), Metric: 10, FirstHops: []types.SystemID{sysID(2)}},
			srcID(3): {ID: srcID(3), Metric: 10, FirstHops: []types.SystemID{sysID(3)}},
		},
	}
	return g, res, over, finite
}

// onlyRoute returns the single route for want, failing when it is absent or duplicated.
func onlyRoute(t *testing.T, routes []RouteEntry, want netip.Prefix) (RouteEntry, bool) {
	t.Helper()
	var found []RouteEntry
	for _, r := range routes {
		if r.Prefix == want {
			found = append(found, r)
		}
	}
	if len(found) > 1 {
		t.Fatalf("%s installed %d times", want, len(found))
	}
	if len(found) == 0 {
		return RouteEntry{}, false
	}
	return found[0], true
}

// assertFiniteRouteAlone checks that shared is installed from node 3 alone at
// 10 + 50 and that lone, advertised only above the ceiling, is not installed.
func assertFiniteRouteAlone(t *testing.T, routes []RouteEntry, shared, lone netip.Prefix) {
	t.Helper()
	r, ok := onlyRoute(t, routes, shared)
	if !ok {
		t.Fatalf("%s, advertised at metric 50 by node 3, was not installed", shared)
	}
	if r.Metric != 60 {
		t.Fatalf("%s installed at metric %d, want 60 (the finite advertisement alone)", shared, r.Metric)
	}
	if len(r.NextHops) != 1 {
		t.Fatalf("%s installed with %d next hops, want 1: the over-max advertisement joined the set", shared, len(r.NextHops))
	}
	if _, ok := onlyRoute(t, routes, lone); ok {
		t.Fatalf("%s, advertised only above MAX_PATH_METRIC, was installed", lone)
	}
}

// RFC requirement: RFC5305-4-1 positive -- a TLV 135 prefix advertised at a finite
// metric (50) by node 3 is installed at 10 + 50 through node 3.
// RFC requirement: RFC5305-4-1 negative -- the same prefix advertised by node 2 at
// 0xFE000001, and a second prefix advertised only at 0xFFFFFFFF, both larger than
// MAX_PATH_METRIC, take no part: the shared prefix keeps one next hop and metric 60,
// and the lone prefix is not installed.
func TestRFC5305PrefixAboveMaxPathMetricNotConsidered(t *testing.T) {
	shared := netip.MustParsePrefix("10.9.0.0/24")
	lone := netip.MustParsePrefix("10.8.0.0/24")
	g, res, over, finite := maxMetricFixture()
	over.Prefixes = append(over.Prefixes,
		Prefix{Prefix: shared, Metric: uint32(MaxPathMetric + 1)},
		Prefix{Prefix: lone, Metric: 0xFFFFFFFF},
	)
	finite.Prefixes = append(finite.Prefixes, Prefix{Prefix: shared, Metric: 50})

	routes := BuildRoutes([]*Result{res}, map[Level]*Graph{Level1: g}, stubResolver{})
	assertFiniteRouteAlone(t, routes, shared, lone)
}

// RFC requirement: RFC5308-2-2 positive -- a TLV 236 prefix advertised at a finite
// metric (50) by node 3 is installed at 10 + 50 through node 3.
// RFC requirement: RFC5308-2-2 negative -- the same prefix advertised by node 2 at
// 0xFE000001, and a second prefix advertised only at 0xFFFFFFFF, both larger than
// MAX_V6_PATH_METRIC, take no part: the shared prefix keeps one next hop and metric
// 60, and the lone prefix is not installed.
func TestRFC5308PrefixAboveMaxV6PathMetricNotConsidered(t *testing.T) {
	shared := netip.MustParsePrefix("2001:db8:9::/64")
	lone := netip.MustParsePrefix("2001:db8:8::/64")
	g, res, over, finite := maxMetricFixture()
	over.PrefixesV6 = append(over.PrefixesV6,
		Prefix{Prefix: shared, Metric: uint32(MaxV6PathMetric + 1)},
		Prefix{Prefix: lone, Metric: 0xFFFFFFFF},
	)
	finite.PrefixesV6 = append(finite.PrefixesV6, Prefix{Prefix: shared, Metric: 50})

	routes := BuildRoutesV6([]*Result{res}, map[Level]*Graph{Level1: g}, stubResolverV6{})
	assertFiniteRouteAlone(t, routes, shared, lone)
}

// preferenceClass is one of the RFC 5308 sec 5 path classes, advertised by its
// own node at a metric that INVERTS the class order, so a metric comparison
// would pick the opposite winner.
type preferenceClass struct {
	name   string
	level  Level
	upDown bool
	origin byte
	metric uint32
}

// preferenceOrder lists the four classes best first (RFC 5308 sec 5).
var preferenceOrder = []preferenceClass{
	{name: "L1 up", level: Level1, upDown: false, origin: 2, metric: 400},
	{name: "L2 up", level: Level2, upDown: false, origin: 3, metric: 300},
	{name: "L2 down", level: Level2, upDown: true, origin: 4, metric: 200},
	{name: "L1 down", level: Level1, upDown: true, origin: 5, metric: 100},
}

// classRoutes builds the L1 and L2 graphs and results holding the given classes,
// each class advertised by its own node at distance 10, and returns the IPv6 routes.
func classRoutes(prefix netip.Prefix, classes []preferenceClass) []RouteEntry {
	graphs := map[Level]*Graph{Level1: NewGraph(), Level2: NewGraph()}
	results := map[Level]*Result{}
	for _, lvl := range []Level{Level1, Level2} {
		graphs[lvl].node(types.NewSourceID(sysID(1), 0))
		results[lvl] = &Result{Root: sysID(1), Level: lvl, Nodes: map[types.SourceID]*NodeResult{}}
	}
	for _, c := range classes {
		n := graphs[c.level].node(srcID(c.origin))
		n.PrefixesV6 = append(n.PrefixesV6, Prefix{Prefix: prefix, Metric: c.metric, UpDown: c.upDown})
		results[c.level].Nodes[srcID(c.origin)] = &NodeResult{
			ID: srcID(c.origin), Metric: 10, FirstHops: []types.SystemID{sysID(c.origin)},
		}
	}
	return BuildRoutesV6([]*Result{results[Level1], results[Level2]}, graphs, stubResolverV6{})
}

// RFC requirement: RFC5308-5-1 positive -- with the four classes present, and then
// with each best class removed in turn, the installed path is always the most
// preferred class left: L1 up, then L2 up, then L2 down, then L1 down.
// RFC requirement: RFC5308-5-1 negative -- every class is advertised at a LOWER
// metric than the class above it, so each of the adjacent pairs (L1 up over L2 up,
// L2 up over L2 down, L2 down over L1 down) and L1 up over L2 down would be
// decided the other way by metric; none is.
func TestRFC5308FourStepPathPreference(t *testing.T) {
	prefix := netip.MustParsePrefix("2001:db8:5::/64")
	for first := range preferenceOrder {
		want := preferenceOrder[first]
		r, ok := onlyRoute(t, classRoutes(prefix, preferenceOrder[first:]), prefix)
		if !ok {
			t.Fatalf("with %s the best class present, no route was installed", want.name)
		}
		if len(r.NextHops) != 1 || r.NextHops[0].Addr.As16()[15] != want.origin {
			t.Fatalf("with %s the best class present, the route goes via %v, want node %d", want.name, r.NextHops, want.origin)
		}
		if r.Level != want.level || r.UpDown != want.upDown {
			t.Fatalf("with %s the best class present, the winner is level %d up/down %v", want.name, r.Level, r.UpDown)
		}
	}

	// L1 up against L2 down alone: the non-adjacent pair the four-step list also orders.
	r, ok := onlyRoute(t, classRoutes(prefix, []preferenceClass{preferenceOrder[0], preferenceOrder[2]}), prefix)
	if !ok || r.Level != Level1 || r.UpDown {
		t.Fatalf("L1 up against L2 down: winner level %d up/down %v (installed=%v), want L1 up", r.Level, r.UpDown, ok)
	}
}
