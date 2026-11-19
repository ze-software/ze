// Design: docs/architecture/web-interface.md -- Trace graph: the routers of the AS a prefix crosses, then its AS path
// Related: graph_nexthop.go -- Next-hop forwarding graph (the inside half, one hop per row)
// Related: graph.go -- AS path topology graph (the outside half)
// Related: handler_graph.go -- Graph HTTP handler and the node cap

package lg

import (
	"cmp"
	"errors"
	"fmt"
	"net/netip"
	"slices"

	"github.com/ze-software/ze/internal/core/bgp/asn"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// traceNote is what the trace graph page and its help say about the
// limit of the trace. It is stated here once, and both render this constant.
const traceNote = "The trace follows each router's BGP next hop, from one router of the AS to the next. " +
	"The IGP's hops between two consecutive next hops lie outside what it draws."

// The keys of the peer summary the router list is read from
// (bgp/plugins/cmd/peer/summary.go, handleBgpSummary), and the keys of a RIB
// row the walk reads (bgp/plugins/rib/rib_pipeline.go, serializeRouteItem).
const (
	fieldPeerType = "peer-type"
	fieldFamily   = "family"
	fieldPrefix   = "prefix"
	fieldNextHop  = "next-hop"
	fieldASPath   = "as-path"
)

// The two values the engine writes under `peer-type`
// (bgp/reactor/reactor_api.go, the peerType of a PeerInfo).
const (
	peerTypeInternal = "internal"
	peerTypeExternal = "external"
)

var errTraceSummary = errors.New("lg trace graph: the peer summary does not say which peers are routers of the AS")

// traceRouters is the set of routers of the AS: the peers whose session with
// Ze is internal, each known by its session address with Ze. Only
// traceRoutersFrom builds one from a summary. The zero value holds no router.
type traceRouters struct {
	addresses map[netip.Addr]struct{}
	// peerCount is how many peers the summary held, routers or not. The
	// message of a graph with no walk states it beside the router count.
	peerCount int
}

// traceRoutersFrom reads the routers of the AS out of the `show bgp` answer
// the looking glass already reads for its peers page. A peer is a router of
// the AS when the engine says its session is internal: its row's `peer-type`.
//
// The engine decides that from the local AS of the peer's own session, and
// the looking glass MUST NOT decide it again from the summary's `local-as`:
// that member is the global local AS, which is 0 in a configuration that
// states the local AS on each peer, and comparing with it finds no router.
//
// A summary that cannot say so is refused: with a peer whose type or address
// cannot be read, the walk would take a router for an external neighbor and
// state a wrong exit.
func traceRoutersFrom(summary map[string]any) (traceRouters, error) {
	peers := summaryPeers(summary)
	addresses := make(map[netip.Addr]struct{})
	for _, entry := range peers {
		peer, isRow := entry.(map[string]any)
		if !isRow {
			return traceRouters{}, fmt.Errorf("%w: a peer entry is not a row", errTraceSummary)
		}

		// The set is open: the value arrives from the engine's answer.
		switch peerType := getStr(peer, fieldPeerType); peerType {
		case peerTypeExternal:
			continue
		case peerTypeInternal:
		default:
			return traceRouters{}, fmt.Errorf("%w: peer %q carries %s %q, which is neither %s nor %s",
				errTraceSummary, peerAddress(peer), fieldPeerType, peerType,
				peerTypeInternal, peerTypeExternal)
		}

		address, err := netip.ParseAddr(peerAddress(peer))
		if err != nil {
			return traceRouters{}, fmt.Errorf("%w: peer address %q is not an IP address",
				errTraceSummary, peerAddress(peer))
		}
		addresses[address.Unmap()] = struct{}{}
	}
	return traceRouters{addresses: addresses, peerCount: len(peers)}, nil
}

// has reports whether address is the session address of a router of the AS.
func (r traceRouters) has(address netip.Addr) bool {
	_, isRouter := r.addresses[address]
	return isRouter
}

// traceEnd says how one walk ends. Every walk has exactly one, and the text
// form and the page state it.
type traceEnd uint8

const (
	// traceEndUnspecified is the zero value and never the end of a walk.
	traceEndUnspecified traceEnd = iota
	// traceEndExitOwnAddress: the last router's next hop is its own address,
	// the next-hop-self case. It is the exit, and the AS path follows.
	traceEndExitOwnAddress
	// traceEndExitExternalAddress: the last router's next hop is the address
	// of no router of the AS. The walk leaves the AS there toward that
	// address, and the AS path follows.
	traceEndExitExternalAddress
	// traceEndOriginInside: the last router's route leaves the walk (its next
	// hop is its own address or no router's) and its AS path is empty. The
	// prefix originates inside the AS at that router.
	traceEndOriginInside
	// traceEndLoop: the last router's next hop is a router the walk already
	// crossed.
	traceEndLoop
	// traceEndNoRoute: the last router was named as next hop and sent Ze no
	// route for the prefix.
	traceEndNoRoute
	// traceEndSeveralRoutes: Ze holds several different routes from the last
	// router for the prefix, and no row says which one the router chose.
	traceEndSeveralRoutes
	// traceEndRouteUnreadable: the row Ze holds from the last router carries
	// a next hop or an AS path that the walk cannot read.
	traceEndRouteUnreadable
)

// String names the end as the text form writes it after `end=`. The first
// three names are those of the simulated topology's TraceEnd values
// (internal/chaos/scenario/topology.go), so a test compares the two directly.
func (e traceEnd) String() string {
	switch e {
	case traceEndUnspecified:
		return "unspecified"
	case traceEndExitOwnAddress:
		return "exit-own-address"
	case traceEndExitExternalAddress:
		return "exit-external-address"
	case traceEndOriginInside:
		return "origin-inside"
	case traceEndLoop:
		return "loop"
	case traceEndNoRoute:
		return "no-route"
	case traceEndSeveralRoutes:
		return "several-routes"
	case traceEndRouteUnreadable:
		return "route-unreadable"
	}
	return "unspecified"
}

// tracePrefix identifies the routes of one walk: the family and the prefix as
// the engine wrote them on the row. It stays the engine's text because it is
// only ever compared with another row's, never computed on.
type tracePrefix struct {
	family string
	prefix string
}

// traceWalk is one walk: from a router of the AS, through each router its
// BGP next hops name in turn, to a stated end.
type traceWalk struct {
	Prefix tracePrefix
	// Routers holds the routers the walk reached, the starting router first.
	// End is a statement about the last one.
	Routers []netip.Addr
	End     traceEnd
	// NextHop is the next hop the last router's route carries. It is set for
	// the two exits, for traceEndOriginInside and for traceEndLoop, where it
	// is the router already crossed. The three other ends read no next hop.
	NextHop netip.Addr
	// ASPath is the AS path of the exit route, as the row holds it. It is set
	// for the two exits only.
	ASPath []uint32
}

// last answers the router the walk ended at.
func (t traceWalk) last() netip.Addr {
	return t.Routers[len(t.Routers)-1]
}

// statement is the sentence the page shows for the end of the walk.
func (t traceWalk) statement() string {
	var sentence textbuf.Buffer
	switch t.End {
	case traceEndUnspecified:
		panic("BUG: a trace was built without an end")
	case traceEndExitOwnAddress:
		sentence.Str("leaves the AS at ").Addr(t.last()).Str(", which announces its own address as next hop")
	case traceEndExitExternalAddress:
		sentence.Str("leaves the AS at ").Addr(t.last()).Str(" toward ").Addr(t.NextHop).
			Str(", the address of no router of the AS")
	case traceEndOriginInside:
		sentence.Str("ends at ").Addr(t.last()).
			Str(": its route has an empty AS path, so the prefix originates inside the AS at that router")
	case traceEndLoop:
		sentence.Str("ends at ").Addr(t.last()).Str(": its next hop is ").Addr(t.NextHop).
			Str(", a router this walk already crossed, so the walk would loop")
	case traceEndNoRoute:
		sentence.Str("ends at ").Addr(t.last()).Str(": Ze holds no route from it for the prefix")
	case traceEndSeveralRoutes:
		sentence.Str("ends at ").Addr(t.last()).
			Str(": Ze holds several routes from it for the prefix and no row says which one it chose")
	case traceEndRouteUnreadable:
		sentence.Str("ends at ").Addr(t.last()).
			Str(": Ze holds a route from it, and its next hop or its AS path cannot be read")
	}
	return sentence.String()
}

// traceRouter is a router of the AS that at least one walk reached.
type traceRouter struct {
	Address netip.Addr
	Exit    bool // At least one walk leaves the AS at this router.
	Origin  bool // At least one prefix originates inside the AS at this router.
}

// traceHopEdge is one BGP next hop a walk followed: from a router of the AS
// to another router, or to the external address a walk leaves the AS toward.
type traceHopEdge struct {
	From netip.Addr
	To   netip.Addr
}

// traceEntryEdge joins the two halves: from where a walk leaves the AS to the
// first AS of the exit route's path. From is the exit router when its next hop
// is its own address, and the external next hop otherwise.
type traceEntryEdge struct {
	From netip.Addr
	AS   uint32
}

// traceASEdge is one step of an exit route's AS path, toward the origin.
type traceASEdge struct {
	From uint32
	To   uint32
}

// traceGraph is the trace of one prefix query. Every slice is sorted,
// so two builds from the same rows are equal whatever order the rows came in.
type traceGraph struct {
	Routers    []traceRouter    // By address.
	Neighbors  []netip.Addr     // External next hops walks leave the AS toward, by address.
	ASes       []uint32         // Every AS of an exit route's path, by number.
	HopEdges   []traceHopEdge   // By From, then To.
	EntryEdges []traceEntryEdge // By From, then AS.
	ASEdges    []traceASEdge    // By From, then To.
	Walks      []traceWalk      // By family, prefix, then starting router.

	counts traceCounts // What the graph was built from, for the message of a graph with no walk.
}

// nodeCount is what the node cap counts: routers, external next hops and ASes.
func (g *traceGraph) nodeCount() int {
	return len(g.Routers) + len(g.Neighbors) + len(g.ASes)
}

// traceWalksListedMax bounds the walks the text form and the drawing list, one
// line each. The node cap does not bound them: a lookup by address answers
// every covering prefix, up to 129 of them, and each is walked from every
// router that sent it, so a graph under the node cap can hold thousands of
// walks. Every walk still gives its nodes and edges to the graph.
const traceWalksListedMax = 100

// listedWalks answers the walks that get a line, the first
// traceWalksListedMax in the graph's own order, and how many got none.
func (g *traceGraph) listedWalks() (listed []traceWalk, unlistedCount int) {
	if len(g.Walks) <= traceWalksListedMax {
		return g.Walks, 0
	}
	return g.Walks[:traceWalksListedMax], len(g.Walks) - traceWalksListedMax
}

// traceUnlistedSentence says how many walks got no line and why. count MUST
// be above zero: a graph that lists every walk writes no such sentence.
func traceUnlistedSentence(count int) string {
	var sentence textbuf.Buffer
	return sentence.Int(int64(count)).Str(" more traces are not listed: the list stops at ").
		Int(traceWalksListedMax).Str(" lines. Ask for one prefix with its length to see its traces.").String()
}

// traceCounts is what a graph was built from: how many peers the summary
// held, how many of them are routers of the AS, and how many routes the query
// answered. The message of a graph with no walk states the three, so its
// reader sees which of them left nothing to draw.
type traceCounts struct {
	peerCount   int
	routerCount int
	routeCount  int
}

// undrawn answers the message the handler shows in place of the graph, and
// whether there is one: no walk started, or the graph is past maxGraphNodes,
// the cap the two other graphs apply (handler_graph.go).
func (g *traceGraph) undrawn() (message string, isUndrawn bool) {
	if len(g.Walks) == 0 {
		return g.counts.noTrace(), true
	}
	if g.nodeCount() > maxGraphNodes {
		return textbuf.StrIntStr("Too many nodes (", int64(g.nodeCount()), ") for graph"), true
	}
	return "", false
}

// noTrace is the message of a graph with no walk. A walk starts at a router of
// the AS that sent Ze a route, so there are three ways to have none, and each
// has its own sentence: the summary names no router, the query answered no
// route, or no answered route came from a router. Every form ends with the
// three counts it was decided on. It names no command: the page is public, and
// its reader cannot run one. It is one line, because the drawing shows it as
// one.
func (a traceCounts) noTrace() string {
	var message textbuf.Buffer
	message.Str("No trace: ")
	if a.routerCount == 0 {
		message.Str("this looking glass has a session with no router of the AS (a peer with an internal session)")
		return a.writeCounts(&message)
	}
	if a.routeCount == 0 {
		message.Str("the query answered no route for this prefix")
		return a.writeCounts(&message)
	}
	message.Str("no route the query answered came from a router of the AS (a peer with an internal session)")
	return a.writeCounts(&message)
}

// writeCounts ends a noTrace message with the three counts and answers it.
func (a traceCounts) writeCounts(message *textbuf.Buffer) string {
	return message.Str(". Peers: ").Int(int64(a.peerCount)).
		Str(", routers of the AS: ").Int(int64(a.routerCount)).
		Str(", routes answered: ").Int(int64(a.routeCount)).
		Byte('.').String()
}

// traceRoute is what the walk reads of one RIB row.
type traceRoute struct {
	nextHop netip.Addr
	asPath  []uint32
}

// traceRouterRows says what Ze holds from one router for one prefix.
type traceRouterRows uint8

const (
	// traceRouterRowsUnspecified is the zero value and never stored.
	traceRouterRowsUnspecified traceRouterRows = iota
	// traceRouterRowsOneRoute: one route, the router's chosen one.
	traceRouterRowsOneRoute
	// traceRouterRowsSeveralRoutes: rows that differ, so the route the router chose is unknown.
	traceRouterRowsSeveralRoutes
	// traceRouterRowsUnreadable: a row the walk cannot read.
	traceRouterRowsUnreadable
)

// traceRouterRoute is what the walk reads of one router for one prefix: what
// its rows are, and the route when they are traceRouterRowsOneRoute.
type traceRouterRoute struct {
	rows  traceRouterRows
	route traceRoute
}

// same reports whether two rows of one router for one prefix say the same.
func (c traceRouterRoute) same(other traceRouterRoute) bool {
	if c.rows != other.rows {
		return false
	}
	if c.route.nextHop != other.route.nextHop {
		return false
	}
	return slices.Equal(c.route.asPath, other.route.asPath)
}

// traceRouterRouteFrom reads one RIB row. An absent `as-path` is the empty path:
// bgp-rib writes no `as-path` member for a route whose path is empty
// (rib_attr_format.go). A next hop that is no address, or a path member that
// is no AS number, makes the row unreadable: skipping it would state that the
// router sent no route.
func traceRouterRouteFrom(row map[string]any) traceRouterRoute {
	nextHop, err := netip.ParseAddr(getStr(row, fieldNextHop))
	if err != nil {
		return traceRouterRoute{rows: traceRouterRowsUnreadable}
	}

	var asPath []uint32
	if raw, present := row[fieldASPath]; present {
		members, isList := raw.([]any)
		if !isList {
			return traceRouterRoute{rows: traceRouterRowsUnreadable}
		}
		asPath = make([]uint32, 0, len(members))
		for _, member := range members {
			number, ok := asn.FromJSON(member)
			if !ok {
				return traceRouterRoute{rows: traceRouterRowsUnreadable}
			}
			asPath = append(asPath, number)
		}
	}
	return traceRouterRoute{
		rows:  traceRouterRowsOneRoute,
		route: traceRoute{nextHop: nextHop.Unmap(), asPath: asPath},
	}
}

// traceRoutesByRouter groups the rows by prefix, then by the router that sent them.
// A row from a peer that is no router of the AS starts no walk and is left
// out: an external neighbor is reached by a router's next hop, never walked
// from.
//
// Each router sends the collector its one best path, so a router's row for a
// prefix is its chosen route. Rows of one router and one prefix that differ
// (ADD-PATH) leave the chosen route unknown, because a received row carries no
// mark of the router's own best path: the rows become
// traceRouterRowsSeveralRoutes, and the walk says so, never picking one.
func traceRoutesByRouter(routes []any, routers traceRouters) map[tracePrefix]map[netip.Addr]traceRouterRoute {
	routesByPrefix := make(map[tracePrefix]map[netip.Addr]traceRouterRoute)
	for _, r := range routes {
		row, isRow := r.(map[string]any)
		if !isRow {
			continue
		}
		peer, err := netip.ParseAddr(getStr(row, "peer-address"))
		if err != nil {
			continue
		}
		router := peer.Unmap()
		if !routers.has(router) {
			continue
		}

		prefix := tracePrefix{family: getStr(row, fieldFamily), prefix: getStr(row, fieldPrefix)}
		routerRoutes := routesByPrefix[prefix]
		if routerRoutes == nil {
			routerRoutes = make(map[netip.Addr]traceRouterRoute)
			routesByPrefix[prefix] = routerRoutes
		}

		routerRoute := traceRouterRouteFrom(row)
		previous, seen := routerRoutes[router]
		if !seen {
			routerRoutes[router] = routerRoute
			continue
		}
		if previous.same(routerRoute) {
			continue
		}
		routerRoutes[router] = traceRouterRoute{rows: traceRouterRowsSeveralRoutes}
	}
	return routesByPrefix
}

// traceWalkFrom follows the chosen routes of one prefix from start. start MUST be
// a router of the AS.
//
// The loop is bounded by the number of routers of the AS: each pass reaches a
// router the walk has not crossed, or returns.
func traceWalkFrom(
	prefix tracePrefix, start netip.Addr, routers traceRouters, routerRoutes map[netip.Addr]traceRouterRoute,
) traceWalk {
	crossed := make(map[netip.Addr]struct{}, len(routers.addresses))
	walk := traceWalk{Prefix: prefix}
	router := start

	for range len(routers.addresses) {
		crossed[router] = struct{}{}
		walk.Routers = append(walk.Routers, router)

		routerRoute, hasRows := routerRoutes[router]
		if !hasRows {
			walk.End = traceEndNoRoute
			return walk
		}
		switch routerRoute.rows {
		case traceRouterRowsUnspecified:
			panic("BUG: a router route was stored without saying what rows it was read from")
		case traceRouterRowsSeveralRoutes:
			walk.End = traceEndSeveralRoutes
			return walk
		case traceRouterRowsUnreadable:
			walk.End = traceEndRouteUnreadable
			return walk
		case traceRouterRowsOneRoute:
		}

		// The next hop goes on the trace only at an end that states it. A
		// hop the walk follows is already the next entry of Routers, and an
		// end that reads no next hop MUST NOT carry the one before it.
		nextHop := routerRoute.route.nextHop
		if nextHop == router {
			walk.NextHop = nextHop
			return walk.leave(traceEndExitOwnAddress, routerRoute.route.asPath)
		}
		if !routers.has(nextHop) {
			walk.NextHop = nextHop
			return walk.leave(traceEndExitExternalAddress, routerRoute.route.asPath)
		}
		if _, visited := crossed[nextHop]; visited {
			walk.NextHop = nextHop
			walk.End = traceEndLoop
			return walk
		}
		router = nextHop
	}
	panic("BUG: a trace walk crossed more routers than the AS has")
}

// leave ends the walk where it leaves the routers of the AS. An empty AS path
// there means the prefix originates inside the AS, whichever exit it was.
func (t traceWalk) leave(exit traceEnd, asPath []uint32) traceWalk {
	if len(asPath) == 0 {
		t.End = traceEndOriginInside
		return t
	}
	t.End = exit
	t.ASPath = asPath
	return t
}

// buildTraceGraph builds the trace from the rows of one prefix query
// and the routers of the AS. routes MUST be the answer of the `received`
// scope, as extractRoutes returns it (prefixLookup.receivedCommand): a row of
// the `sent` scope names the peer Ze sent the route to, which would read as
// that router's chosen route.
//
// One walk starts at each router that sent a row, for each prefix the answer
// holds: a lookup by address answers every covering prefix, and each is
// walked on its own rows.
func buildTraceGraph(routes []any, routers traceRouters) *traceGraph {
	routesByPrefix := traceRoutesByRouter(routes, routers)

	prefixes := make([]tracePrefix, 0, len(routesByPrefix))
	for prefix := range routesByPrefix {
		prefixes = append(prefixes, prefix)
	}
	slices.SortFunc(prefixes, func(a, b tracePrefix) int {
		if c := cmp.Compare(a.family, b.family); c != 0 {
			return c
		}
		return cmp.Compare(a.prefix, b.prefix)
	})

	graph := &traceGraph{counts: traceCounts{
		peerCount:   routers.peerCount,
		routerCount: len(routers.addresses),
		routeCount:  len(routes),
	}}
	for _, prefix := range prefixes {
		routerRoutes := routesByPrefix[prefix]
		starts := make([]netip.Addr, 0, len(routerRoutes))
		for router := range routerRoutes {
			starts = append(starts, router)
		}
		slices.SortFunc(starts, netip.Addr.Compare)
		for _, start := range starts {
			graph.Walks = append(graph.Walks, traceWalkFrom(prefix, start, routers, routerRoutes))
		}
	}
	graph.draw()
	return graph
}

// draw derives the nodes and edges from the traces.
func (g *traceGraph) draw() {
	routers := make(map[netip.Addr]traceRouter)
	neighbors := make(map[netip.Addr]struct{})
	ases := make(map[uint32]struct{})
	hops := make(map[traceHopEdge]struct{})
	entries := make(map[traceEntryEdge]struct{})
	steps := make(map[traceASEdge]struct{})

	for _, walk := range g.Walks {
		for i, address := range walk.Routers {
			node := routers[address]
			node.Address = address
			routers[address] = node
			if i > 0 {
				hops[traceHopEdge{From: walk.Routers[i-1], To: address}] = struct{}{}
			}
		}

		last := routers[walk.last()]
		switch walk.End {
		case traceEndExitOwnAddress:
			last.Exit = true
			entries[traceEntryEdge{From: last.Address, AS: walk.ASPath[0]}] = struct{}{}
			traceASChain(walk.ASPath, ases, steps)
		case traceEndExitExternalAddress:
			last.Exit = true
			neighbors[walk.NextHop] = struct{}{}
			hops[traceHopEdge{From: last.Address, To: walk.NextHop}] = struct{}{}
			entries[traceEntryEdge{From: walk.NextHop, AS: walk.ASPath[0]}] = struct{}{}
			traceASChain(walk.ASPath, ases, steps)
		case traceEndOriginInside:
			last.Origin = true
			// The prefix may originate at a router whose route points at an
			// address outside the routers of the AS: the hop is drawn, and
			// no AS follows it.
			if walk.NextHop != last.Address {
				neighbors[walk.NextHop] = struct{}{}
				hops[traceHopEdge{From: last.Address, To: walk.NextHop}] = struct{}{}
			}
		case traceEndLoop:
			hops[traceHopEdge{From: last.Address, To: walk.NextHop}] = struct{}{}
		case traceEndUnspecified, traceEndNoRoute, traceEndSeveralRoutes, traceEndRouteUnreadable:
			// Nothing beyond the routers the walk reached.
		}
		routers[last.Address] = last
	}

	g.Routers = make([]traceRouter, 0, len(routers))
	for _, node := range routers {
		g.Routers = append(g.Routers, node)
	}
	slices.SortFunc(g.Routers, func(a, b traceRouter) int { return a.Address.Compare(b.Address) })

	g.Neighbors = make([]netip.Addr, 0, len(neighbors))
	for address := range neighbors {
		g.Neighbors = append(g.Neighbors, address)
	}
	slices.SortFunc(g.Neighbors, netip.Addr.Compare)

	g.ASes = make([]uint32, 0, len(ases))
	for number := range ases {
		g.ASes = append(g.ASes, number)
	}
	slices.Sort(g.ASes)

	g.HopEdges = make([]traceHopEdge, 0, len(hops))
	for edge := range hops {
		g.HopEdges = append(g.HopEdges, edge)
	}
	slices.SortFunc(g.HopEdges, func(a, b traceHopEdge) int {
		if c := a.From.Compare(b.From); c != 0 {
			return c
		}
		return a.To.Compare(b.To)
	})

	g.EntryEdges = make([]traceEntryEdge, 0, len(entries))
	for edge := range entries {
		g.EntryEdges = append(g.EntryEdges, edge)
	}
	slices.SortFunc(g.EntryEdges, func(a, b traceEntryEdge) int {
		if c := a.From.Compare(b.From); c != 0 {
			return c
		}
		return cmp.Compare(a.AS, b.AS)
	})

	g.ASEdges = make([]traceASEdge, 0, len(steps))
	for edge := range steps {
		g.ASEdges = append(g.ASEdges, edge)
	}
	slices.SortFunc(g.ASEdges, func(a, b traceASEdge) int {
		if c := cmp.Compare(a.From, b.From); c != 0 {
			return c
		}
		return cmp.Compare(a.To, b.To)
	})
}

// traceASChain adds the ASes of one exit route's path and the steps between
// them. A prepended AS is one node and no step, as in the AS path graph.
func traceASChain(asPath []uint32, ases map[uint32]struct{}, steps map[traceASEdge]struct{}) {
	for i, number := range asPath {
		ases[number] = struct{}{}
		if i == 0 {
			continue
		}
		if asPath[i-1] == number {
			continue
		}
		steps[traceASEdge{From: asPath[i-1], To: number}] = struct{}{}
	}
}

// traceNames holds the two name lookups the text form and the drawing label
// nodes with. A nil lookup, and a lookup that answers "", name nothing.
type traceNames struct {
	// address answers the operator's name for a router or next-hop address
	// (nexthop_ranges.go).
	address func(netip.Addr) string
	// as answers the name of an AS number (LGServer.resolveASN).
	as func(uint32) string
}

func (n traceNames) ofAddress(address netip.Addr) string {
	if n.address == nil {
		return ""
	}
	return n.address(address)
}

func (n traceNames) ofAS(number uint32) string {
	if n.as == nil {
		return ""
	}
	return n.as(number)
}

// renderTraceGraphText returns the deterministic text form: the nodes, the
// edges, then one `trace` line per walk, each group in the graph's own order.
//
// A trace line reads
//
//	trace <family> <prefix> <router> routers=<a>,<b> end=<end> [next-hop=<a>] [as-path=<n>,<n>] says: <sentence>
//
// where <router> is the starting router, `next-hop=` is written for the ends
// that read one and `as-path=` for the two exits. The sentence runs to the end
// of the line. AS numbers are written in the configured notation, as the AS
// path graph writes them.
func renderTraceGraphText(g *traceGraph, names traceNames) string {
	var text textbuf.Buffer
	text.Str("mode trace\n")

	for _, router := range g.Routers {
		text.Str("node router ").Addr(router.Address)
		if router.Exit {
			text.Str(" exit")
		}
		if router.Origin {
			text.Str(" origin")
		}
		traceWriteName(&text, names.ofAddress(router.Address))
	}
	for _, neighbor := range g.Neighbors {
		text.Str("node neighbor ").Addr(neighbor)
		traceWriteName(&text, names.ofAddress(neighbor))
	}
	for _, number := range g.ASes {
		text.Str("node AS").Str(asn.Text(number, asn.Configured()))
		traceWriteName(&text, names.ofAS(number))
	}

	for _, edge := range g.HopEdges {
		text.Str("edge ").Addr(edge.From).Str(" -> ").Addr(edge.To).Byte('\n')
	}
	for _, edge := range g.EntryEdges {
		text.Str("edge ").Addr(edge.From).Str(" -> AS").Str(asn.Text(edge.AS, asn.Configured())).Byte('\n')
	}
	for _, edge := range g.ASEdges {
		text.Str("edge AS").Str(asn.Text(edge.From, asn.Configured())).
			Str(" -> AS").Str(asn.Text(edge.To, asn.Configured())).Byte('\n')
	}

	listed, unlistedCount := g.listedWalks()
	for _, walk := range listed {
		traceWriteWalk(&text, walk)
	}
	if unlistedCount > 0 {
		text.Str("unlisted traces=").Int(int64(unlistedCount)).
			Str(" says: ").Str(traceUnlistedSentence(unlistedCount)).Byte('\n')
	}
	return text.String()
}

// traceWriteName ends a node line, with the name when there is one. The name
// is the last field, so a name that holds a space still reads to the line end.
func traceWriteName(text *textbuf.Buffer, name string) {
	if name != "" {
		text.Str(" name=").Str(name)
	}
	text.Byte('\n')
}

// traceWriteWalk writes the `trace` line of one walk.
func traceWriteWalk(text *textbuf.Buffer, walk traceWalk) {
	text.Str("trace ").Str(walk.Prefix.family).Byte(' ').Str(walk.Prefix.prefix).
		Byte(' ').Addr(walk.Routers[0]).Str(" routers=")
	for i, router := range walk.Routers {
		if i > 0 {
			text.Byte(',')
		}
		text.Addr(router)
	}
	text.Str(" end=").Str(walk.End.String())

	switch walk.End {
	case traceEndExitOwnAddress, traceEndExitExternalAddress:
		text.Str(" next-hop=").Addr(walk.NextHop).Str(" as-path=")
		for i, number := range walk.ASPath {
			if i > 0 {
				text.Byte(',')
			}
			text.Str(asn.Text(number, asn.Configured()))
		}
	case traceEndOriginInside, traceEndLoop:
		text.Str(" next-hop=").Addr(walk.NextHop)
	case traceEndUnspecified, traceEndNoRoute, traceEndSeveralRoutes, traceEndRouteUnreadable:
		// These ends read no next hop.
	}
	text.Str(" says: ").Str(walk.statement()).Byte('\n')
}
