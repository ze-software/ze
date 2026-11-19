package lg

import (
	"errors"
	"net/netip"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// The fixtures below are JSON text in the two shapes the engine answers,
// decoded by the looking glass's own readers, so a test never feeds the
// builder a shape the engine cannot produce (numbers stay numbers):
//
//   - the peer summary of handleBgpSummary
//     (internal/component/bgp/plugins/cmd/peer/summary.go): `local-as` and the
//     rows as siblings, each row with `address`, `remote-as` and `peer-type`.
//     The top-level `local-as` is the GLOBAL local AS, which is 0 when the
//     configuration states the local AS on each peer, as the simulated
//     network's does (internal/chaos/scenario/config.go, writeFullPeerBlock);
//   - the rows of `show bgp rib received`, one serializeRouteItem each
//     (internal/component/bgp/plugins/rib/rib_pipeline.go): `peer`,
//     `direction`, `family`, `prefix`, a bare `next-hop`, and `as-path`
//     wrapped with its attribute flags, absent when the path is empty.

const (
	traceTestLocalAS = 65000
	traceTestPrefix  = "198.18.0.0/24"
)

// traceTestPeer is one session of the summary fixture.
type traceTestPeer struct {
	address  string
	remoteAS uint32
}

// traceTestWrite appends the parts of a JSON fixture in order.
func traceTestWrite(sb *strings.Builder, parts ...string) {
	for _, part := range parts {
		sb.WriteString(part)
	}
}

// traceTestRouter is a session with a router of the AS.
func traceTestRouter(address string) traceTestPeer {
	return traceTestPeer{address: address, remoteAS: traceTestLocalAS}
}

// traceTestSummaryText writes a `show bgp` answer holding the given sessions,
// of a configuration that states the local AS on each peer: the global
// `local-as` is 0, and `peer-type` says which sessions are internal, as the
// engine derives it from the session's own local AS.
func traceTestSummaryText(peers ...traceTestPeer) string {
	var sb strings.Builder
	traceTestWrite(&sb, `{"router-id":"10.0.0.1","local-as":0,"peers":[`)
	for i, peer := range peers {
		if i > 0 {
			sb.WriteByte(',')
		}
		peerType := "external"
		if peer.remoteAS == traceTestLocalAS {
			peerType = "internal"
		}
		traceTestWrite(&sb, `{"address":"`, peer.address, `","name":"p`, strconv.Itoa(i),
			`","description":"","group":"","remote-as":`, strconv.FormatUint(uint64(peer.remoteAS), 10),
			`,"peer-type":"`, peerType, `","state":"established","connections-dropped":0}`)
	}
	sb.WriteString(`]}`)
	return sb.String()
}

// traceTestSummary decodes a `show bgp` answer holding the given sessions.
func traceTestSummary(t *testing.T, peers ...traceTestPeer) traceRouters {
	t.Helper()
	routers, err := traceRoutersFrom(parseJSON(traceTestSummaryText(peers...)))
	if err != nil {
		t.Fatalf("traceRoutersFrom: %v", err)
	}
	return routers
}

// traceTestRow is one received row of traceTestPrefix: the peer that sent
// it, its next hop and its AS path. An empty path writes no `as-path` member.
func traceTestRow(peer, nextHop string, asPath ...uint32) string {
	return traceTestRowOf(traceTestPrefix, peer, nextHop, asPath...)
}

func traceTestRowOf(prefix, peer, nextHop string, asPath ...uint32) string {
	var sb strings.Builder
	traceTestWrite(&sb, `{"peer":"`, peer, `","direction":"received","family":"ipv4/unicast","prefix":"`,
		prefix, `","next-hop":"`, nextHop, `",`,
		`"origin":{"value":"igp","optional":false,"transitive":true,"partial":false}`)
	if len(asPath) > 0 {
		sb.WriteString(`,"as-path":{"value":[`)
		for i, number := range asPath {
			if i > 0 {
				sb.WriteByte(',')
			}
			sb.WriteString(strconv.FormatUint(uint64(number), 10))
		}
		sb.WriteString(`],"optional":false,"transitive":true,"partial":false}`)
	}
	sb.WriteString(`,"local-preference":{"value":100,"optional":false,"transitive":true,"partial":false}}`)
	return sb.String()
}

// traceTestRoutes decodes a `show bgp rib received` answer holding the rows.
func traceTestRoutes(t *testing.T, rows ...string) []any {
	t.Helper()
	routes := extractRoutes(parseJSON(`{"routes":[` + strings.Join(rows, ",") + `]}`))
	if len(routes) != len(rows) {
		t.Fatalf("the fixture decoded to %d rows, want %d", len(routes), len(rows))
	}
	return routes
}

func traceTestAddrs(addresses ...string) []netip.Addr {
	out := make([]netip.Addr, 0, len(addresses))
	for _, address := range addresses {
		out = append(out, netip.MustParseAddr(address))
	}
	return out
}

// traceTestWalkOf answers the one walk that starts at router.
func traceTestWalkOf(t *testing.T, g *traceGraph, router string) traceWalk {
	t.Helper()
	start := netip.MustParseAddr(router)
	var found []traceWalk
	for _, trace := range g.Walks {
		if trace.Routers[0] == start {
			found = append(found, trace)
		}
	}
	if len(found) != 1 {
		t.Fatalf("%d walks start at %s, want 1", len(found), router)
	}
	return found[0]
}

// traceTestExpectWalk compares one walk with its expected routers, end,
// next hop ("" for an end that reads none) and AS path.
func traceTestExpectWalk(
	t *testing.T, trace traceWalk, routers []netip.Addr, end traceEnd, nextHop string, asPath []uint32,
) {
	t.Helper()
	if !slices.Equal(trace.Routers, routers) {
		t.Errorf("routers = %v, want %v", trace.Routers, routers)
	}
	if trace.End != end {
		t.Errorf("end = %s, want %s", trace.End, end)
	}
	want := netip.Addr{}
	if nextHop != "" {
		want = netip.MustParseAddr(nextHop)
	}
	if trace.NextHop != want {
		t.Errorf("next hop = %s, want %s", trace.NextHop, want)
	}
	if !slices.Equal(trace.ASPath, asPath) {
		t.Errorf("AS path = %v, want %v", trace.ASPath, asPath)
	}
}

// VALIDATES: a walk follows each router's next hop to the router whose session
// address it is, across two intermediate routers, to the exit router.
// PREVENTS: a trace that stops after one hop, as the next-hop graph's rows do,
// or that walks from an external neighbor's row.
func TestTraceWalkGoesRouterToRouter(t *testing.T) {
	routers := traceTestSummary(t,
		traceTestRouter("198.51.100.1"), traceTestRouter("198.51.100.3"),
		traceTestRouter("198.51.100.4"), traceTestRouter("198.51.100.5"),
		traceTestPeer{address: "203.0.113.1", remoteAS: 65101},
	)
	routes := traceTestRoutes(t,
		traceTestRow("198.51.100.5", "198.51.100.4", 65101, 64500),
		traceTestRow("198.51.100.4", "198.51.100.3", 65101, 64500),
		traceTestRow("198.51.100.3", "198.51.100.1", 65101, 64500),
		traceTestRow("198.51.100.1", "198.51.100.1", 65101, 64500),
		traceTestRow("203.0.113.1", "203.0.113.1", 65101, 64500),
	)

	g := buildTraceGraph(routes, routers)

	if len(g.Walks) != 4 {
		t.Fatalf("walks = %d, want 4: one per router, none from the external neighbor", len(g.Walks))
	}
	path := []uint32{65101, 64500}
	traceTestExpectWalk(t, traceTestWalkOf(t, g, "198.51.100.5"),
		traceTestAddrs("198.51.100.5", "198.51.100.4", "198.51.100.3", "198.51.100.1"),
		traceEndExitOwnAddress, "198.51.100.1", path)
	traceTestExpectWalk(t, traceTestWalkOf(t, g, "198.51.100.4"),
		traceTestAddrs("198.51.100.4", "198.51.100.3", "198.51.100.1"),
		traceEndExitOwnAddress, "198.51.100.1", path)
	traceTestExpectWalk(t, traceTestWalkOf(t, g, "198.51.100.1"),
		traceTestAddrs("198.51.100.1"), traceEndExitOwnAddress, "198.51.100.1", path)

	wantHops := []traceHopEdge{
		{From: netip.MustParseAddr("198.51.100.3"), To: netip.MustParseAddr("198.51.100.1")},
		{From: netip.MustParseAddr("198.51.100.4"), To: netip.MustParseAddr("198.51.100.3")},
		{From: netip.MustParseAddr("198.51.100.5"), To: netip.MustParseAddr("198.51.100.4")},
	}
	if !slices.Equal(g.HopEdges, wantHops) {
		t.Errorf("hop edges = %v, want %v", g.HopEdges, wantHops)
	}
}

// VALIDATES: AC-21, AC-22 -- the exit router is joined to the first AS of the
// exit route's path, and the AS chain runs to the origin, a prepended AS
// drawn once.
// PREVENTS: two graphs side by side with no edge between the halves.
func TestTraceGraphLinksExitToFirstAS(t *testing.T) {
	routers := traceTestSummary(t, traceTestRouter("198.51.100.1"), traceTestRouter("198.51.100.4"))
	routes := traceTestRoutes(t,
		traceTestRow("198.51.100.4", "198.51.100.1", 65101, 65101, 64496, 64500),
		traceTestRow("198.51.100.1", "198.51.100.1", 65101, 65101, 64496, 64500),
	)

	g := buildTraceGraph(routes, routers)

	exit := netip.MustParseAddr("198.51.100.1")
	wantRouters := []traceRouter{{Address: exit, Exit: true}, {Address: netip.MustParseAddr("198.51.100.4")}}
	if !slices.Equal(g.Routers, wantRouters) {
		t.Errorf("routers = %v, want %v: one exit, every chain ending at it", g.Routers, wantRouters)
	}
	if want := []traceEntryEdge{{From: exit, AS: 65101}}; !slices.Equal(g.EntryEdges, want) {
		t.Errorf("entry edges = %v, want %v", g.EntryEdges, want)
	}
	if want := []uint32{64496, 64500, 65101}; !slices.Equal(g.ASes, want) {
		t.Errorf("ASes = %v, want %v", g.ASes, want)
	}
	if want := []traceASEdge{{From: 64496, To: 64500}, {From: 65101, To: 64496}}; !slices.Equal(g.ASEdges, want) {
		t.Errorf("AS edges = %v, want %v", g.ASEdges, want)
	}
	if len(g.Neighbors) != 0 {
		t.Errorf("neighbors = %v, want none: the exit announces its own address", g.Neighbors)
	}
}

// VALIDATES: AC-26 -- a next hop equal to the router's own address makes it the
// exit, and the AS path follows.
// PREVENTS: next-hop-self read as a router visited twice, which is a loop.
func TestTraceWalkExitsOnOwnAddress(t *testing.T) {
	routers := traceTestSummary(t, traceTestRouter("198.51.100.1"))
	routes := traceTestRoutes(t, traceTestRow("198.51.100.1", "198.51.100.1", 65101))

	g := buildTraceGraph(routes, routers)

	trace := traceTestWalkOf(t, g, "198.51.100.1")
	traceTestExpectWalk(t, trace, traceTestAddrs("198.51.100.1"),
		traceEndExitOwnAddress, "198.51.100.1", []uint32{65101})
	if len(g.HopEdges) != 0 {
		t.Errorf("hop edges = %v, want none: the router does not cross itself", g.HopEdges)
	}
	if got := trace.statement(); strings.Contains(got, "loop") || !strings.Contains(got, "leaves the AS at 198.51.100.1") {
		t.Errorf("statement = %q, want the exit at 198.51.100.1 and no loop", got)
	}
}

// VALIDATES: AC-27, A-10 -- a next hop that is no router's address leaves the
// AS at that router toward that address, whether or not the neighbor at that
// address holds a session with Ze. The address is outside the AS, so it is no
// node: the exit links straight to the first AS of the exit route's path, and
// the walk's statement and next hop keep the address.
// PREVENTS: a walk that needs a session, or a row, for the external neighbor,
// and an outside address drawn as if it were a router of the AS.
func TestTraceWalkLeavesAtExternalNextHop(t *testing.T) {
	border := netip.MustParseAddr("198.51.100.2")

	sessions := map[string][]traceTestPeer{
		"the neighbor holds no session": {traceTestRouter("198.51.100.2")},
		"the neighbor holds a session": {
			traceTestRouter("198.51.100.2"), {address: "203.0.113.9", remoteAS: 65102},
		},
	}
	for name, peers := range sessions {
		t.Run(name, func(t *testing.T) {
			routers := traceTestSummary(t, peers...)
			rows := []string{traceTestRow("198.51.100.2", "203.0.113.9", 65102, 64500)}
			if len(peers) > 1 {
				rows = append(rows, traceTestRow("203.0.113.9", "203.0.113.9", 65102, 64500))
			}

			g := buildTraceGraph(traceTestRoutes(t, rows...), routers)

			if len(g.Walks) != 1 {
				t.Fatalf("walks = %d, want 1", len(g.Walks))
			}
			trace := traceTestWalkOf(t, g, "198.51.100.2")
			traceTestExpectWalk(t, trace, traceTestAddrs("198.51.100.2"),
				traceEndExitExternalAddress, "203.0.113.9", []uint32{65102, 64500})
			if len(g.Neighbors) != 0 {
				t.Errorf("neighbors = %v, want none: an address outside the AS is drawn as its AS", g.Neighbors)
			}
			if len(g.HopEdges) != 0 {
				t.Errorf("hop edges = %v, want none: the walk crossed one router", g.HopEdges)
			}
			if want := []traceEntryEdge{{From: border, AS: 65102}}; !slices.Equal(g.EntryEdges, want) {
				t.Errorf("entry edges = %v, want %v", g.EntryEdges, want)
			}
			if want := []uint32{64500, 65102}; !slices.Equal(g.ASes, want) {
				t.Errorf("ASes = %v, want %v", g.ASes, want)
			}
			if got := trace.statement(); !strings.Contains(got, "toward 203.0.113.9") {
				t.Errorf("statement = %q, want the address the walk leaves toward", got)
			}
		})
	}
}

// VALIDATES: AC-25 -- two routers naming each other end each walk with the
// loop, stating the router already crossed.
// PREVENTS: a walk that never ends, or a loop drawn as a complete chain.
func TestTraceWalkStopsOnRevisitedRouter(t *testing.T) {
	routers := traceTestSummary(t, traceTestRouter("198.51.100.1"), traceTestRouter("198.51.100.2"))
	routes := traceTestRoutes(t,
		traceTestRow("198.51.100.1", "198.51.100.2", 65101),
		traceTestRow("198.51.100.2", "198.51.100.1", 65101),
	)

	g := buildTraceGraph(routes, routers)

	first := traceTestWalkOf(t, g, "198.51.100.1")
	traceTestExpectWalk(t, first, traceTestAddrs("198.51.100.1", "198.51.100.2"),
		traceEndLoop, "198.51.100.1", nil)
	second := traceTestWalkOf(t, g, "198.51.100.2")
	traceTestExpectWalk(t, second, traceTestAddrs("198.51.100.2", "198.51.100.1"),
		traceEndLoop, "198.51.100.2", nil)

	got := first.statement()
	if !strings.Contains(got, "its next hop is 198.51.100.1") || !strings.Contains(got, "would loop") {
		t.Errorf("statement = %q, want the revisited router 198.51.100.1 and that the walk would loop", got)
	}
	if len(g.ASes) != 0 || len(g.EntryEdges) != 0 {
		t.Errorf("ASes = %v, entry edges = %v, want none: no walk left the AS", g.ASes, g.EntryEdges)
	}
	for _, router := range g.Routers {
		if router.Exit {
			t.Errorf("router %s is marked exit, and no walk left the AS", router.Address)
		}
	}
}

// VALIDATES: AC-28 -- a next hop naming a router of the AS that sent no route
// for the prefix ends the walk at that router, with its statement.
// PREVENTS: the named router read as an external neighbor, or the row before
// it drawn as the exit.
func TestTraceWalkStopsOnRouterWithoutRoute(t *testing.T) {
	routers := traceTestSummary(t, traceTestRouter("198.51.100.3"), traceTestRouter("198.51.100.4"))
	routes := traceTestRoutes(t,
		traceTestRow("198.51.100.4", "198.51.100.3", 65101),
		// The named router holds a route for another prefix only.
		traceTestRowOf("198.18.9.0/24", "198.51.100.3", "198.51.100.3", 65101),
	)

	g := buildTraceGraph(routes, routers)

	var walk traceWalk
	for _, trace := range g.Walks {
		if trace.Prefix.prefix == traceTestPrefix {
			if walk.End != traceEndUnspecified {
				t.Fatalf("two walks for %s, want one: the router without a route starts none", traceTestPrefix)
			}
			walk = trace
		}
	}
	traceTestExpectWalk(t, walk, traceTestAddrs("198.51.100.4", "198.51.100.3"), traceEndNoRoute, "", nil)
	if got := walk.statement(); !strings.Contains(got, "ends at 198.51.100.3: Ze holds no route from it") {
		t.Errorf("statement = %q, want the router and that Ze holds no route from it", got)
	}
	if len(g.Neighbors) != 0 {
		t.Errorf("neighbors = %v, want none: 198.51.100.3 is a router of the AS", g.Neighbors)
	}
}

// VALIDATES: AC-30 -- an empty AS path at the exit ends the walk there and
// states that the prefix originates inside the AS at that router.
// PREVENTS: an edge to an AS that does not exist, or an index past an empty
// path.
func TestTraceWalkOriginInsideAS(t *testing.T) {
	routers := traceTestSummary(t, traceTestRouter("198.51.100.1"), traceTestRouter("198.51.100.4"))
	routes := traceTestRoutes(t,
		traceTestRow("198.51.100.4", "198.51.100.1"),
		traceTestRow("198.51.100.1", "198.51.100.1"),
	)

	g := buildTraceGraph(routes, routers)

	trace := traceTestWalkOf(t, g, "198.51.100.4")
	traceTestExpectWalk(t, trace, traceTestAddrs("198.51.100.4", "198.51.100.1"),
		traceEndOriginInside, "198.51.100.1", nil)
	if got := trace.statement(); !strings.Contains(got, "originates inside the AS at that router") ||
		!strings.Contains(got, "ends at 198.51.100.1") {
		t.Errorf("statement = %q, want the origin inside the AS at 198.51.100.1", got)
	}
	if len(g.ASes) != 0 || len(g.EntryEdges) != 0 || len(g.ASEdges) != 0 {
		t.Errorf("ASes = %v, entry edges = %v, AS edges = %v, want none", g.ASes, g.EntryEdges, g.ASEdges)
	}
	origin := traceRouter{Address: netip.MustParseAddr("198.51.100.1"), Origin: true}
	if !slices.Contains(g.Routers, origin) {
		t.Errorf("routers = %v, want %v among them", g.Routers, origin)
	}
}

// VALIDATES: AC-23 -- the text form lists nodes, edges and walks in a fixed
// order whatever order the rows came in, with each walk's stated end and a
// name on a node that has one.
// PREVENTS: map iteration order reaching the text, so two requests differ.
func TestTraceGraphTextIsOrdered(t *testing.T) {
	routers := traceTestSummary(t,
		traceTestRouter("198.51.100.1"), traceTestRouter("198.51.100.2"),
		traceTestRouter("198.51.100.3"), traceTestRouter("198.51.100.4"),
	)
	rows := []string{
		traceTestRow("198.51.100.4", "198.51.100.3", 65101, 64500),
		traceTestRow("198.51.100.3", "198.51.100.1", 65101, 64500),
		traceTestRow("198.51.100.1", "198.51.100.1", 65101, 64500),
		traceTestRow("198.51.100.2", "203.0.113.9", 65102, 64500),
	}
	names := traceNames{
		address: func(address netip.Addr) string {
			if address == netip.MustParseAddr("203.0.113.9") {
				return "ixp-lan-a"
			}
			return ""
		},
		as: func(number uint32) string {
			if number == 65101 {
				return "Transit A"
			}
			return ""
		},
	}

	want := "mode trace\n" +
		"node router 198.51.100.1 exit\n" +
		"node router 198.51.100.2 exit\n" +
		"node router 198.51.100.3\n" +
		"node router 198.51.100.4\n" +
		"node AS64500\n" +
		"node AS65101 name=Transit A\n" +
		"node AS65102\n" +
		"edge 198.51.100.3 -> 198.51.100.1\n" +
		"edge 198.51.100.4 -> 198.51.100.3\n" +
		"edge 198.51.100.1 -> AS65101\n" +
		"edge 198.51.100.2 -> AS65102\n" +
		"edge AS65101 -> AS64500\n" +
		"edge AS65102 -> AS64500\n" +
		"trace ipv4/unicast 198.18.0.0/24 198.51.100.1 routers=198.51.100.1 end=exit-own-address" +
		" next-hop=198.51.100.1 as-path=65101,64500" +
		" says: leaves the AS at 198.51.100.1, which announces its own address as next hop\n" +
		"trace ipv4/unicast 198.18.0.0/24 198.51.100.2 routers=198.51.100.2 end=exit-external-address" +
		" next-hop=203.0.113.9 as-path=65102,64500" +
		" says: leaves the AS at 198.51.100.2 toward 203.0.113.9, the address of no router of the AS\n" +
		"trace ipv4/unicast 198.18.0.0/24 198.51.100.3 routers=198.51.100.3,198.51.100.1 end=exit-own-address" +
		" next-hop=198.51.100.1 as-path=65101,64500" +
		" says: leaves the AS at 198.51.100.1, which announces its own address as next hop\n" +
		"trace ipv4/unicast 198.18.0.0/24 198.51.100.4 routers=198.51.100.4,198.51.100.3,198.51.100.1" +
		" end=exit-own-address next-hop=198.51.100.1 as-path=65101,64500" +
		" says: leaves the AS at 198.51.100.1, which announces its own address as next hop\n"

	forward := renderTraceGraphText(buildTraceGraph(traceTestRoutes(t, rows...), routers), names)
	if forward != want {
		t.Errorf("text =\n%s\nwant\n%s", forward, want)
	}

	slices.Reverse(rows)
	backward := renderTraceGraphText(buildTraceGraph(traceTestRoutes(t, rows...), routers), names)
	if backward != forward {
		t.Errorf("the text changed with the order of the rows:\n%s\nagainst\n%s", backward, forward)
	}
}

// VALIDATES: R-12 -- the trace graph applies the 100-node cap of the other
// graphs, with its message, and draws a graph of exactly 100 nodes.
// PREVENTS: a full path set drawn as an unreadable picture.
func TestTraceGraphRespectsNodeCap(t *testing.T) {
	routers := traceTestSummary(t, traceTestRouter("198.51.100.1"))
	// One router and a path of n distinct ASes: n+1 nodes.
	graphOf := func(pathLength int) *traceGraph {
		asPath := make([]uint32, 0, pathLength)
		for i := range pathLength {
			asPath = append(asPath, uint32(64600+i))
		}
		return buildTraceGraph(traceTestRoutes(t, traceTestRow("198.51.100.1", "198.51.100.1", asPath...)), routers)
	}

	atCap := graphOf(maxGraphNodes - 1)
	if atCap.nodeCount() != maxGraphNodes {
		t.Fatalf("nodes = %d, want %d", atCap.nodeCount(), maxGraphNodes)
	}
	if message, isUndrawn := atCap.undrawn(); isUndrawn {
		t.Errorf("a graph of %d nodes is refused with %q, want it drawn", maxGraphNodes, message)
	}

	pastCap := graphOf(maxGraphNodes)
	message, isUndrawn := pastCap.undrawn()
	if !isUndrawn || message != "Too many nodes (101) for graph" {
		t.Errorf("undrawn = %q, %v, want the cap message for 101 nodes", message, isUndrawn)
	}

	empty := buildTraceGraph(traceTestRoutes(t), routers)
	message, isUndrawn = empty.undrawn()
	const noRoute = "No trace: the query answered no route for this prefix. " +
		"Peers: 1, routers of the AS: 1, routes answered: 0."
	if !isUndrawn || message != noRoute {
		t.Errorf("undrawn = %q, %v, want %q", message, isUndrawn, noRoute)
	}
}

// VALIDATES: rows of one router and one prefix that differ (ADD-PATH) end the
// walk there, stating that no row says which route the router chose; equal
// rows are one route.
// PREVENTS: the walk following whichever row came first.
func TestTraceWalkStopsOnSeveralRoutes(t *testing.T) {
	routers := traceTestSummary(t, traceTestRouter("198.51.100.1"), traceTestRouter("198.51.100.4"))
	routes := traceTestRoutes(t,
		traceTestRow("198.51.100.4", "198.51.100.1", 65101),
		traceTestRow("198.51.100.4", "198.51.100.1", 65101),
		traceTestRow("198.51.100.1", "198.51.100.1", 65101),
		traceTestRow("198.51.100.1", "203.0.113.9", 65102),
	)

	g := buildTraceGraph(routes, routers)

	trace := traceTestWalkOf(t, g, "198.51.100.4")
	traceTestExpectWalk(t, trace, traceTestAddrs("198.51.100.4", "198.51.100.1"),
		traceEndSeveralRoutes, "", nil)
	if got := trace.statement(); !strings.Contains(got, "Ze holds several routes from it") {
		t.Errorf("statement = %q, want that Ze holds several routes", got)
	}
	if len(g.ASes) != 0 || len(g.Neighbors) != 0 {
		t.Errorf("ASes = %v, neighbors = %v, want none: no route was chosen", g.ASes, g.Neighbors)
	}
}

// VALIDATES: a router's row with no next hop the walk can read ends the walk
// there with its own statement.
// PREVENTS: the row skipped, which would state that the router sent no route.
func TestTraceWalkStopsOnUnreadableRoute(t *testing.T) {
	routers := traceTestSummary(t, traceTestRouter("198.51.100.1"))
	routes := traceTestRoutes(t, traceTestRow("198.51.100.1", "", 65101))

	g := buildTraceGraph(routes, routers)

	trace := traceTestWalkOf(t, g, "198.51.100.1")
	traceTestExpectWalk(t, trace, traceTestAddrs("198.51.100.1"), traceEndRouteUnreadable, "", nil)
	if got := trace.statement(); !strings.Contains(got, "its next hop or its AS path cannot be read") {
		t.Errorf("statement = %q, want that the next hop or the AS path cannot be read", got)
	}
}

// VALIDATES: a graph under the node cap that holds more walks than
// traceWalksListedMax lists that many, in the text form and under the drawing,
// and states how many it left unlisted; a graph at the bound lists them all.
// PREVENTS: a lookup by address, which answers every covering prefix, writing
// one line per prefix and router without a bound.
func TestTraceGraphBoundsTheWalksItLists(t *testing.T) {
	routers := traceTestSummary(t, traceTestRouter("198.51.100.1"))
	// One router, one row per prefix: as many walks as prefixes, and two
	// nodes whatever their number.
	graphOf := func(prefixCount int) *traceGraph {
		rows := make([]string, 0, prefixCount)
		for i := range prefixCount {
			prefix := netip.PrefixFrom(netip.AddrFrom4([4]byte{10, byte(i >> 8), byte(i), 0}), 24)
			rows = append(rows, traceTestRowOf(prefix.String(), "198.51.100.1", "198.51.100.1", 65101))
		}
		return buildTraceGraph(traceTestRoutes(t, rows...), routers)
	}

	atBound := graphOf(traceWalksListedMax)
	if _, isUndrawn := atBound.undrawn(); isUndrawn {
		t.Fatal("a graph of two nodes is not drawn")
	}
	text := renderTraceGraphText(atBound, traceNames{})
	if got := strings.Count(text, "\ntrace "); got != traceWalksListedMax {
		t.Errorf("trace lines at the bound = %d, want %d", got, traceWalksListedMax)
	}
	if strings.Contains(text, "unlisted") {
		t.Errorf("a graph at the bound states unlisted walks:\n%s", text)
	}
	if got := len(traceEndLines(atBound)); got != traceWalksListedMax {
		t.Errorf("lines under the drawing at the bound = %d, want %d", got, traceWalksListedMax)
	}

	pastBound := graphOf(traceWalksListedMax + 3)
	text = renderTraceGraphText(pastBound, traceNames{})
	if got := strings.Count(text, "\ntrace "); got != traceWalksListedMax {
		t.Errorf("trace lines past the bound = %d, want %d", got, traceWalksListedMax)
	}
	const sentence = "3 more traces are not listed: the list stops at 100 lines. " +
		"Ask for one prefix with its length to see its traces."
	if !strings.HasSuffix(text, "\nunlisted traces=3 says: "+sentence+"\n") {
		t.Errorf("the text form does not end with the unlisted line:\n%s", text)
	}
	lines := traceEndLines(pastBound)
	if len(lines) != traceWalksListedMax+1 || lines[len(lines)-1] != sentence {
		t.Errorf("lines under the drawing = %d, last %q, want %d and %q",
			len(lines), lines[len(lines)-1], traceWalksListedMax+1, sentence)
	}
}

// VALIDATES: a summary that cannot say which peers are routers of the AS is
// refused, and the routers are the peers the engine calls internal.
// PREVENTS: a router read as an external neighbor because its `peer-type`
// was missing or unknown, which would state a wrong exit.
func TestTraceRoutersRefusesUnreadableSummary(t *testing.T) {
	refused := map[string]string{
		"no peer-type":         `{"local-as":0,"peers":[{"address":"198.51.100.1","remote-as":65000}]}`,
		"unknown peer-type":    `{"local-as":0,"peers":[{"address":"198.51.100.1","remote-as":65000,"peer-type":"confederation"}]}`,
		"address no IP":        `{"local-as":0,"peers":[{"address":"router-1","remote-as":65000,"peer-type":"internal"}]}`,
		"peer entry not a row": `{"local-as":0,"peers":["198.51.100.1"]}`,
	}
	for name, summary := range refused {
		t.Run(name, func(t *testing.T) {
			_, err := traceRoutersFrom(parseJSON(summary))
			if !errors.Is(err, errTraceSummary) {
				t.Errorf("error = %v, want errTraceSummary", err)
			}
		})
	}

	routers := traceTestSummary(t,
		traceTestRouter("198.51.100.1"), traceTestPeer{address: "203.0.113.1", remoteAS: 65101})
	if !routers.has(netip.MustParseAddr("198.51.100.1")) {
		t.Error("198.51.100.1 has an internal session and is not a router")
	}
	if routers.has(netip.MustParseAddr("203.0.113.1")) {
		t.Error("203.0.113.1 has an external session and is a router")
	}
}

// VALIDATES: the routers of the AS are read from each peer's `peer-type`, in
// the two shapes handleBgpSummary answers: a global `local-as` of 0 (the local
// AS stated on each peer), and a global `local-as` a peer overrides.
// PREVENTS: the router list recomputed from the global `local-as`, which found
// no router on the simulated network (eleven sessions, each with its own
// `session asn local`) and answered every trace graph with no trace.
func TestTraceRoutersAreTheInternalPeers(t *testing.T) {
	summaries := map[string]string{
		"local AS on each peer": `{"router-id":"192.0.2.254","local-as":0,"peers":[` +
			`{"address":"198.51.100.1","remote-as":65000,"peer-type":"internal"},` +
			`{"address":"203.0.113.1","remote-as":65101,"peer-type":"external"}]}`,
		"peer overrides the global local AS": `{"router-id":"192.0.2.254","local-as":64512,"peers":[` +
			`{"address":"198.51.100.1","remote-as":65000,"peer-type":"internal"},` +
			`{"address":"203.0.113.1","remote-as":64512,"peer-type":"external"}]}`,
	}
	for name, summary := range summaries {
		t.Run(name, func(t *testing.T) {
			routers, err := traceRoutersFrom(parseJSON(summary))
			if err != nil {
				t.Fatalf("traceRoutersFrom: %v", err)
			}
			if !routers.has(netip.MustParseAddr("198.51.100.1")) {
				t.Error("198.51.100.1 is internal and is not a router")
			}
			if routers.has(netip.MustParseAddr("203.0.113.1")) {
				t.Error("203.0.113.1 is external and is a router")
			}
			if routers.peerCount != 2 {
				t.Errorf("peerCount = %d, want 2", routers.peerCount)
			}
		})
	}
}

// VALIDATES: the message of a graph with no walk states which of the three
// facts left nothing to draw, with the three counts.
// PREVENTS: "No route found" for a summary that names no router, which sent
// the reader to the routes when the sessions were the cause.
func TestTraceGraphSaysWhyItHasNoTrace(t *testing.T) {
	noRouter := traceTestSummary(t, traceTestPeer{address: "203.0.113.1", remoteAS: 65101})
	message, isUndrawn := buildTraceGraph(
		traceTestRoutes(t, traceTestRow("203.0.113.1", "203.0.113.1", 65101)), noRouter).undrawn()
	want := "No trace: this looking glass has a session with no router of the AS (a peer with an internal session). " +
		"Peers: 1, routers of the AS: 0, routes answered: 1."
	if !isUndrawn || message != want {
		t.Errorf("no router: undrawn = %q, %v, want %q", message, isUndrawn, want)
	}

	oneRouter := traceTestSummary(t,
		traceTestRouter("198.51.100.1"), traceTestPeer{address: "203.0.113.1", remoteAS: 65101})
	message, isUndrawn = buildTraceGraph(
		traceTestRoutes(t, traceTestRow("203.0.113.1", "203.0.113.1", 65101)), oneRouter).undrawn()
	want = "No trace: no route the query answered came from a router of the AS " +
		"(a peer with an internal session). " +
		"Peers: 2, routers of the AS: 1, routes answered: 1."
	if !isUndrawn || message != want {
		t.Errorf("no route from a router: undrawn = %q, %v, want %q", message, isUndrawn, want)
	}
}
