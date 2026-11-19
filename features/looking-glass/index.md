# Looking Glass

Ze includes a built-in looking glass that exposes BGP session state and route information via both an HTMX web UI and a birdwatcher-compatible REST API. The looking glass runs as a separate HTTP server on its own port (default 8443). It is read-only and open by default, because a looking glass is a public surface. It serves TLS by default, with a self-signed certificate unless the `certificate` leaf names an entry in the PKI store. An optional bearer `token` gates every route when you set one.

| Feature | Description |
|---------|-------------|
| Peer dashboard | Live peer table with state, ASN, route counts, SSE updates |
| Session history | The peer table shows when each session last changed state and how often it dropped, and the header of a peer's page adds why it last closed. The values are the ones `show bgp` answers, the live stream carries them, and a value the engine did not send is an empty cell, never a zero. The last error is public where the looking glass is public |
| Named next hops | `next-hop-range <prefix> { name }` names a range of next-hop addresses, such as an exchange LAN. The route tables and the next-hop graph show the name of the longest range that holds a next hop, and a next hop in no range shows its address alone: nothing is derived. The next-hop graph and the trace draw a router of the AS (an internal session) by its address and a neighbour outside it (an external session) by its AS number only, one node per AS. Two entries for one range are refused at commit, and a reload replaces the list on the running server |
| Peer description and group | The peers table shows the peer's own `description`, empty when the peer has none, and the group apart, in the cell's tooltip and in the peer header. The description is public where the looking glass is public |
| Route age | Every route table has an Age column and the route detail an Age entry: how long ago `bgp-rib` last stored a change to the route, written as `3m 12s` with the exact seconds in the cell's `data-seconds` attribute. An equal re-announcement keeps the age. A route with no reading is an empty cell, never a zero. The API answers the same seconds in `age`, with `age_available` saying whether the value is a reading. Ages start again when the daemon restarts |
| Route lookup | A prefix with its length lists the routes of exactly that prefix, its host bits cleared, sent to `bgp-rib` as `exact`. A bare address lists every route whose prefix contains it, the most specific first for each peer, sent as `covering`. Rows carry the full attributes |
| AS path search | AS numbers separated by spaces, matched as a run anywhere in the path; a leading `^` anchors the run at the start, and a `$` directly after the last AS number anchors it at the end. `65001$` lists the routes AS 65001 originates, and `^65010 65001$` the routes whose path is exactly those two AS numbers. Any other pattern character is refused with the form the field takes |
| Community search | Standard and large community filtering |
| AS path topology graph | Server-side SVG visualization of AS path DAGs |
| Trace graph | `/lg/graph?prefix=X&mode=trace`, the **Trace** button of a search result: from each router of the AS, through the routers its BGP next hops name, to the exit router, then the AS path to the origin AS. An exit toward an address outside the AS links straight to the AS the traffic is handed to: the address is in the walk's line, never a node. Every walk states its end under the graph (an exit, an origin inside the AS, a loop, a router with no route). A router is matched by its session address, and the IGP's hops between two next hops are not drawn |
| Birdwatcher REST API | Alice-LG compatible JSON endpoints under `/api/looking-glass/`. Specified in RFC 2119 terms in [Birdwatcher compatibility](https://github.com/ze-software/ze/blob/main/docs/architecture/api/birdwatcher-compat.md) |
| htmx web UI | Server-rendered HTML pages under `/lg/` with fragment updates. htmx 4.0.0 is embedded, with `hx-sse.min.js` on the peers page |
| templ rendering | Every page and fragment is written in a `.templ` source and compiled to Go, and each view takes a typed struct. The three SVG graph builders keep their markup in Go, because every attribute they write is a coordinate the layout pass computed |
| TLS certificate | `certificate` names a `pki { certificate <name> }` entry. The listener serves that leaf and every intermediate the store holds, so a visitor's browser accepts the chain. A named certificate needs no blob storage, so it serves on a deployment that never ran `ze init` |
| Certificate fails closed | A name the PKI store does not hold stops the daemon at start, and a reload that introduces one is rejected as a whole. Ze never falls back to the self-signed certificate for a configured name. `ze doctor` reports a broken reference, and an expiry less than 30 days away, before the operator commits |
| Certificate rotation | A commit that changes the name installs the new chain on the running server. The listener keeps its socket, and the next handshake serves the new chain. A looking glass serving plaintext takes no rotation, and a restart is what puts a certificate on the wire |
| YANG configuration | `environment/looking-glass` block with enabled, server (ip, port), tls (default true), certificate, token, and the `next-hop-range` list |

<!-- source: internal/component/lg/server.go -- LGServer, HTTP lifecycle -->
<!-- source: internal/component/lg/handler_api.go -- Birdwatcher REST API handlers -->
<!-- source: internal/component/lg/handler_ui.go -- HTMX UI handlers -->
<!-- source: internal/component/lg/prefix_lookup.go -- parsePrefixLookup: a prefix becomes the exact lookup, a bare address the covering lookup -->
<!-- source: internal/component/lg/aspath_pattern.go -- parseASPathPattern: the AS path in the engine's comma grammar -->
<!-- source: internal/component/lg/view.go -- typed view models: layoutView, searchView, peerRow -->
<!-- source: internal/component/lg/markup_check_test.go -- lgMarkupExempt: the two exempt drawings -->
<!-- source: internal/component/lg/yang/ze-lg-conf.yang -- leaf certificate -->
<!-- source: cmd/ze/hub/service_tls.go -- listenerTLSMaterial, the named-certificate precedence -->
<!-- source: internal/component/lg/server.go -- UpdateTLSCertificate, getCertificate -->
<!-- source: internal/component/lg/doctor.go -- the looking-glass certificate reference check -->
<!-- source: internal/component/lg/handler_graph.go -- AS path graph handler -->
<!-- source: internal/component/lg/handler_graph.go -- handleTraceGraph -->
<!-- source: internal/component/lg/graph_trace.go -- buildTraceGraph, traceWalk.statement -->
<!-- source: internal/component/lg/graph.go -- Graph data model -->
<!-- source: internal/component/lg/layout.go -- Layout algorithm and SVG rendering -->

See [Looking Glass Guide](../../guides/looking-glass/index.md) for configuration and usage.

### AS Path Topology Graph

The looking glass includes a server-side SVG graph that visualizes AS path topology for any prefix. When looking up a route, clicking "Show topology" renders a directed acyclic graph where nodes represent autonomous systems and edges represent peering links.

| Feature | Description |
|---------|-------------|
| Server-side SVG | Rendered entirely in Go, no external dependencies (no GraphViz, no WASM, no JS graph library) |
| Layered layout | Sugiyama-inspired left-to-right layout with source ASes on the left, origin on the right |
| AS prepending | Consecutive duplicate ASNs collapsed to a single node |
| Multi-path | Multiple AS paths to the same prefix shown as a branching DAG |
| ASN labels | Each node shows `AS` and the AS number. The organization name, when Team Cymru gives one, shows on mouse-over only, as it does over the AS number of the peer tables |
| Node cap | Graphs limited to 100 nodes to prevent resource exhaustion |
| HTMX integration | Loaded as an inline SVG fragment via `GET /lg/graph?prefix=X` |

<!-- source: internal/component/lg/graph.go -- buildGraph, extractASPath -->
<!-- source: internal/graph/graph.go -- BuildGraphFromPaths, DeduplicateASPath -->
<!-- source: internal/component/lg/layout.go -- computeLayout, renderGraphSVG -->
<!-- source: internal/component/lg/handler_graph.go -- handleGraph endpoint -->
