# Looking Glass

## Meta

| Field | Value |
|-------|-------|
| Name | Looking Glass |
| Page | docs/features/looking-glass.md |
| Kind | daemon |
| Scope | complete |
| Level | experimental |
| Components | internal/component/lg |
| Real-path tests | test/ui/lg-birdwatcher-counts-unavailable.ci, test/ui/lg-peer-table-flat-payload.ci, test/reload/lg-pki-reference-reload.ci, test/plugin/lg-covering-lookup.ci, test/plugin/lg-aspath-search.ci |
| Docs | docs/features/looking-glass.md |
| Doc review | 2026-10-07: ze-lg-conf.yang leaf tls carries default true, and token and certificate leaves exist; matches the row |
| Defect review | 2026-10-07: no open spec or journal row found naming the looking glass |

## Description

Public BGP looking glass with birdwatcher API, AS path graphs, and BMP-monitored route display. Runs on its own configurable port, separate from the web UI. `tls` defaults to **true**. The `certificate` leaf names a `pki { certificate <name> }` entry, and the listener then serves that leaf with every intermediate the store holds. An unset leaf keeps the self-signed certificate. An optional `token` gates every route and is compared in constant time. An unset token leaves the looking glass open, which is the default. Pages render through templ with typed view models, not `html/template`. <!-- source: internal/component/lg/yang/ze-lg-conf.yang -- leaf tls default true, leaf token, leaf certificate -->

The route search, the graph and the birdwatcher prefix endpoints read a prefix with its length as a lookup of exactly that prefix, its host bits cleared, and a bare address as a lookup of every route whose prefix contains it, the most specific first for each peer. <!-- source: internal/component/lg/prefix_lookup.go -- parsePrefixLookup -->

A third graph mode, the trace (`/lg/graph?prefix=X&mode=trace`), follows a prefix from each router of the AS, through the routers its BGP next hops name, to the exit router, and then along the AS path to the origin AS. It is built from the one RIB query for the prefix and the peer summary, states the end of every walk under the graph, matches a router by its session address, and does not draw the IGP's hops between two next hops. <!-- source: internal/component/lg/handler_graph.go -- handleTraceGraph --> <!-- source: internal/component/lg/graph_trace.go -- buildTraceGraph, traceNote -->

The route search reads an AS path as AS numbers separated by spaces, matched as a run anywhere in the path, with an optional leading `^` that anchors the run at the start and an optional `$` directly after the last AS number that anchors it at the end, so `65001$` lists the routes AS 65001 originates; any other pattern character is refused. <!-- source: internal/component/lg/aspath_pattern.go -- parseASPathPattern -->

The peers page shows, for each session, the time of its last state change and how often it dropped, and the header of a peer's page shows why it last closed. The three values are read from `show bgp`, follow the live stream, and render empty, never as a zero, when the engine sent none. <!-- source: internal/component/lg/handler_ui.go -- extractPeers, peerInfoFrom, connectionsDroppedFrom -->

Every route table has an Age column: how long ago `bgp-rib` last stored a change to the route, with the exact seconds in the cell. A route with no reading renders an empty cell, never a zero, and the birdwatcher API answers `age` with `age_available` beside it. Ages start again when the daemon restarts. <!-- source: internal/component/lg/handler_ui.go -- routeAgeFrom, routeRows --> <!-- source: internal/component/lg/handler_api.go -- transformRoutes -->
