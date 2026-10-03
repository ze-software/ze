// Design: docs/architecture/isis/isis-9-spf-rib.md -- RFC 1195 section 3.10 and
// Annex C.1 on IP reachability entries in the Dijkstra calculation.
//
// Goal: prove that SPF computes one route for each distinct IP reachability
// entry a reachable node advertises, at the node distance plus the entry's
// default metric, and none for an entry no path reaches; and that a reachability
// entry is a leaf keyed by its prefix, never confused with a router. Method:
// build small graphs through the stub source, run Compute and BuildRoutes, and
// compare the route set.

package spf

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/plugins/isis/types"
)

// routeByPrefix indexes routes so a test can name the one it asserts on.
func routeByPrefix(routes []RouteEntry) map[netip.Prefix]RouteEntry {
	out := make(map[netip.Prefix]RouteEntry, len(routes))
	for _, r := range routes {
		out[r.Prefix] = r
	}
	return out
}

// RFC requirement: RFC1195-3.10-1 positive -- three distinct reachability entries
// advertised by two reachable nodes each get a route, and an entry advertised by
// both nodes gets one route at the lower total.
// RFC requirement: RFC1195-3.10-3 positive -- each route's metric is the node
// distance plus the entry's default metric, the one metric every entry carries.
func TestRFC1195RouteForEachReachabilityEntry(t *testing.T) {
	src := newStubSource()
	a, b, c := srcID(1), srcID(2), srcID(3)
	src.bidir(a, b, 10)
	src.bidir(b, c, 5)

	g := BuildGraph(src, Level1)
	p1 := netip.MustParsePrefix("10.1.0.0/24")
	p2 := netip.MustParsePrefix("10.2.0.0/24")
	p3 := netip.MustParsePrefix("10.3.0.0/24")
	g.Nodes[b].Prefixes = []Prefix{{Prefix: p1, Metric: 1}, {Prefix: p2, Metric: 2}}
	g.Nodes[c].Prefixes = []Prefix{{Prefix: p3, Metric: 3}, {Prefix: p1, Metric: 100}}

	res := Compute(g, sysID(1), Level1)
	routes := routeByPrefix(BuildRoutes([]*Result{res}, map[Level]*Graph{Level1: g}, stubResolver{}))
	if len(routes) != 3 {
		t.Fatalf("got %d routes, want 3 (one per distinct entry)", len(routes))
	}
	want := map[netip.Prefix]uint64{p1: 11, p2: 12, p3: 18}
	for pfx, metric := range want {
		r, ok := routes[pfx]
		if !ok {
			t.Fatalf("no route for entry %s", pfx)
		}
		if r.Metric != metric {
			t.Fatalf("route %s metric = %d, want %d (node distance plus default metric)", pfx, r.Metric, metric)
		}
	}
}

// RFC requirement: RFC1195-3.10-1 negative -- a reachability entry advertised by
// a node with no path from the root yields no route, so the route set is exactly
// the entries the Dijkstra calculation reached.
func TestRFC1195NoRouteForUnreachedReachabilityEntry(t *testing.T) {
	src := newStubSource()
	a, b, d := srcID(1), srcID(2), srcID(4)
	src.bidir(a, b, 10)
	// D advertises a link toward A, but nothing links toward D: D is unreached.
	src.edge(d, isEdge{a, 10})

	g := BuildGraph(src, Level1)
	reached := netip.MustParsePrefix("10.1.0.0/24")
	unreached := netip.MustParsePrefix("10.9.0.0/24")
	g.Nodes[b].Prefixes = []Prefix{{Prefix: reached, Metric: 1}}
	g.Nodes[d].Prefixes = []Prefix{{Prefix: unreached, Metric: 1}}

	res := Compute(g, sysID(1), Level1)
	if res.Nodes[d] != nil {
		t.Fatalf("node D settled at metric %d, want unreached", res.Nodes[d].Metric)
	}
	routes := routeByPrefix(BuildRoutes([]*Result{res}, map[Level]*Graph{Level1: g}, stubResolver{}))
	if _, ok := routes[unreached]; ok {
		t.Fatalf("a route exists for %s, advertised only by the unreached node D", unreached)
	}
	if _, ok := routes[reached]; !ok {
		t.Fatalf("no route for %s, advertised by the reached node B", reached)
	}
}

// RFC requirement: RFC1195-7-3 positive -- a router whose System ID octets equal
// the octets of a reachability entry (prefix plus length) is still one router
// vertex and the entry is still one prefix route: reachability entries are keyed
// by prefix and never share the router identifier space.
// RFC requirement: RFC1195-7-5 positive -- a reachability entry is a leaf: the
// settled vertex set holds routers only, the entry reaches the route set through
// its advertising router's first hop, and a router behind that router is still
// reached through it.
func TestRFC1195ReachabilityEntryIsALeafKeyedByPrefix(t *testing.T) {
	src := newStubSource()
	pfx := netip.MustParsePrefix("10.4.0.0/24")
	// B's System ID spells the entry: 0a 04 00 00 followed by the length 0x18.
	lookalike := types.NewSourceID(types.SystemID{0x0a, 0x04, 0x00, 0x00, 0x18, 0x00}, 0)
	a, c := srcID(1), srcID(3)
	src.bidir(a, lookalike, 10)
	src.bidir(lookalike, c, 5)

	g := BuildGraph(src, Level1)
	g.Nodes[lookalike].Prefixes = []Prefix{{Prefix: pfx, Metric: 1}}

	res := Compute(g, sysID(1), Level1)
	if len(res.Nodes) != 3 {
		t.Fatalf("settled %d vertices, want 3 routers (A, B, C): a reachability entry is never a vertex", len(res.Nodes))
	}
	b := res.Nodes[lookalike]
	if b == nil || b.Metric != 10 {
		t.Fatalf("router B result = %+v, want metric 10", b)
	}
	if cr := res.Nodes[c]; cr == nil || cr.Metric != 15 || len(cr.FirstHops) != 1 || cr.FirstHops[0] != lookalike.SystemID() {
		t.Fatalf("router C result = %+v, want metric 15 through first hop B", cr)
	}

	routes := routeByPrefix(BuildRoutes([]*Result{res}, map[Level]*Graph{Level1: g}, stubResolver{}))
	if len(routes) != 1 {
		t.Fatalf("got %d routes, want 1", len(routes))
	}
	r, ok := routes[pfx]
	if !ok {
		t.Fatalf("no route for entry %s", pfx)
	}
	if r.Metric != 11 {
		t.Fatalf("route %s metric = %d, want 11 (B at 10 plus default metric 1)", pfx, r.Metric)
	}
	wantHop := netip.AddrFrom4([4]byte{10, 0, 0, 0x00})
	if len(r.NextHops) != 1 || r.NextHops[0].Addr != wantHop {
		t.Fatalf("route %s next hops = %v, want the first hop toward B (%s)", pfx, r.NextHops, wantHop)
	}
}

// RFC requirement: RFC1195-7-3 negative -- a reachable prefix whose bytes resemble a disconnected router cannot make that router's remote prefix reachable.
// RFC requirement: RFC1195-7-5 negative -- prefix leaves cannot act as transit vertices connecting otherwise disconnected router components.
func TestRFC1195PrefixCannotBridgeDisconnectedRouters(t *testing.T) {
	src := newStubSource()
	pfx := netip.MustParsePrefix("10.4.0.0/24")
	lookalike := types.NewSourceID(types.SystemID{0x0a, 0x04, 0x00, 0x00, 0x18, 0x00}, 0)
	a, b, c := srcID(1), srcID(2), srcID(3)
	src.bidir(a, b, 10)
	src.bidir(lookalike, c, 5)
	remote := netip.MustParsePrefix("198.51.100.0/24")
	for i := range src.byLevel[Level1] {
		rec := &src.byLevel[Level1][i]
		switch rec.Source {
		case b:
			rec.LSP.TLVs = append(rec.LSP.TLVs, tlv135(pfx, 1, false))
		case c:
			rec.LSP.TLVs = append(rec.LSP.TLVs, tlv135(remote, 1, false))
		}
	}
	g := BuildGraph(src, Level1)
	res := Compute(g, sysID(1), Level1)
	routes := routeByPrefix(BuildRoutes([]*Result{res}, map[Level]*Graph{Level1: g}, stubResolver{}))
	if _, ok := routes[remote]; ok {
		t.Fatalf("prefix %s bridged the disconnected router component: %+v", pfx, routes[remote])
	}
	if route, ok := routes[pfx]; !ok || route.Metric != 11 || len(route.NextHops) != 1 || route.NextHops[0].Addr.String() != "10.0.0.2" {
		t.Fatalf("reachable prefix was confused with a router: %+v", route)
	}
}
