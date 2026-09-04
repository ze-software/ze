# Looking Glass

Ze includes a built-in looking glass that exposes BGP session state and route information via both an HTMX web UI and a birdwatcher-compatible REST API. The looking glass runs as a separate HTTP server on its own port (default 8443). It is read-only and open by default, because a looking glass is a public surface. It serves TLS by default, with a self-signed certificate unless the `certificate` leaf names an entry in the PKI store. An optional bearer `token` gates every route when you set one.

| Feature | Description |
|---------|-------------|
| Peer dashboard | Live peer table with state, ASN, route counts, SSE updates |
| Route lookup | Prefix and IP containment search with full attribute display |
| AS path search | Pattern-based AS path filtering |
| Community search | Standard and large community filtering |
| AS path topology graph | Server-side SVG visualization of AS path DAGs |
| Birdwatcher REST API | Alice-LG compatible JSON endpoints under `/api/looking-glass/`. Specified in RFC 2119 terms in [Birdwatcher compatibility](../architecture/api/birdwatcher-compat.md) |
| htmx web UI | Server-rendered HTML pages under `/lg/` with fragment updates. htmx 4.0.0 is embedded, with `hx-sse.min.js` on the peers page |
| templ rendering | Every page and fragment is written in a `.templ` source and compiled to Go, and each view takes a typed struct. The two SVG graph builders keep their markup in Go, because every attribute they write is a coordinate the layout pass computed |
| TLS certificate | `certificate` names a `pki { certificate <name> }` entry. The listener serves that leaf and every intermediate the store holds, so a visitor's browser accepts the chain. A named certificate needs no blob storage, so it serves on a deployment that never ran `ze init` |
| Certificate fails closed | A name the PKI store does not hold stops the daemon at start, and a reload that introduces one is rejected as a whole. Ze never falls back to the self-signed certificate for a configured name. `ze doctor` reports a broken reference, and an expiry less than 30 days away, before the operator commits |
| Certificate rotation | A commit that changes the name installs the new chain on the running server. The listener keeps its socket, and the next handshake serves the new chain. A looking glass serving plaintext takes no rotation, and a restart is what puts a certificate on the wire |
| YANG configuration | `environment/looking-glass` block with enabled, server (ip, port), tls (default true), certificate, and token settings |

<!-- source: internal/component/lg/server.go -- LGServer, HTTP lifecycle -->
<!-- source: internal/component/lg/handler_api.go -- Birdwatcher REST API handlers -->
<!-- source: internal/component/lg/handler_ui.go -- HTMX UI handlers -->
<!-- source: internal/component/lg/view.go -- typed view models: layoutView, searchView, peerRow -->
<!-- source: internal/component/lg/markup_check_test.go -- lgMarkupExempt: the two exempt drawings -->
<!-- source: internal/component/lg/yang/ze-lg-conf.yang -- leaf certificate -->
<!-- source: cmd/ze/hub/service_tls.go -- listenerTLSMaterial, the named-certificate precedence -->
<!-- source: internal/component/lg/server.go -- UpdateTLSCertificate, getCertificate -->
<!-- source: internal/component/lg/doctor.go -- the looking-glass certificate reference check -->
<!-- source: internal/component/lg/handler_graph.go -- AS path graph handler -->
<!-- source: internal/component/lg/graph.go -- Graph data model -->
<!-- source: internal/component/lg/layout.go -- Layout algorithm and SVG rendering -->

See [Looking Glass Guide](../guide/looking-glass.md) for configuration and usage.

### AS Path Topology Graph

The looking glass includes a server-side SVG graph that visualizes AS path topology for any prefix. When looking up a route, clicking "Show topology" renders a directed acyclic graph where nodes represent autonomous systems and edges represent peering links.

| Feature | Description |
|---------|-------------|
| Server-side SVG | Rendered entirely in Go, no external dependencies (no GraphViz, no WASM, no JS graph library) |
| Layered layout | Sugiyama-inspired left-to-right layout with source ASes on the left, origin on the right |
| AS prepending | Consecutive duplicate ASNs collapsed to a single node |
| Multi-path | Multiple AS paths to the same prefix shown as a branching DAG |
| ASN labels | Each node shows AS number and organization name (when decorator is available) |
| Node cap | Graphs limited to 100 nodes to prevent resource exhaustion |
| HTMX integration | Loaded as an inline SVG fragment via `GET /lg/graph?prefix=X` |

<!-- source: internal/component/lg/graph.go -- buildGraph, extractASPath -->
<!-- source: internal/graph/graph.go -- BuildGraphFromPaths, DeduplicateASPath -->
<!-- source: internal/component/lg/layout.go -- computeLayout, renderGraphSVG -->
<!-- source: internal/component/lg/handler_graph.go -- handleGraph endpoint -->
