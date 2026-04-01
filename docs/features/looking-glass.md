# Looking Glass

Ze includes a built-in looking glass that exposes BGP session state and route information via both an HTMX web UI and a birdwatcher-compatible REST API. The looking glass runs as a separate HTTP server on its own port (default 8443), with no authentication (public, read-only). TLS is optional.

| Feature | Description |
|---------|-------------|
| Peer dashboard | Live peer table with state, ASN, route counts, SSE updates |
| Route lookup | Prefix and IP containment search with full attribute display |
| AS path search | Pattern-based AS path filtering |
| Community search | Standard and large community filtering |
| AS path topology graph | Server-side SVG visualization of AS path DAGs |
| Birdwatcher REST API | Alice-LG compatible JSON endpoints under `/api/looking-glass/` |
| HTMX web UI | Server-rendered HTML pages under `/lg/` with fragment updates |
| YANG configuration | `environment/looking-glass` block with enabled, server (ip, port), tls settings |

<!-- source: internal/component/lg/server.go -- LGServer, HTTP lifecycle -->
<!-- source: internal/component/lg/handler_api.go -- Birdwatcher REST API handlers -->
<!-- source: internal/component/lg/handler_ui.go -- HTMX UI handlers -->
<!-- source: internal/component/lg/handler_graph.go -- AS path graph handler -->
<!-- source: internal/component/lg/graph.go -- Graph data model -->
<!-- source: internal/component/lg/layout.go -- Layout algorithm and SVG rendering -->

See [Looking Glass Guide](guide/looking-glass.md) for configuration and usage.

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

<!-- source: internal/component/lg/graph.go -- buildGraph, deduplicateASPath -->
<!-- source: internal/component/lg/layout.go -- computeLayout, renderGraphSVG -->
<!-- source: internal/component/lg/handler_graph.go -- handleGraph endpoint -->
