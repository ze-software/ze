# Looking Glass

Ze includes a built-in looking glass that provides public, read-only access to BGP session state and route information. It runs as a separate HTTP server from the authenticated web UI.

<!-- source: internal/component/lg/server.go -- LGServer -->

## Configuration

Add the `looking-glass` block under `environment` in your Ze config:

```
environment {
    looking-glass {
        enabled true
        server main {
            ip 0.0.0.0
            port 8443
        }
    }
}
```

<!-- source: internal/component/lg/schema/ze-lg-conf.yang -- YANG schema -->

| Setting | Default | Description |
|---------|---------|-------------|
| `enabled` | `false` | Enable the looking glass server. |
| `server <name> { ip }` | `0.0.0.0` | Listen address. Set to `127.0.0.1` to restrict to local access. |
| `server <name> { port }` | `8443` | Listen port. Must differ from the web UI port. |
| `tls` | `false` | Enable TLS. Requires blob storage (`ze init`). |

Environment variable overrides: `ze.looking-glass.ip`, `ze.looking-glass.port`, `ze.looking-glass.tls`.

When the `looking-glass` block is absent, no HTTP server is started and no resources are consumed.

<!-- source: cmd/ze/hub/main.go -- startLGServer -->

## Web UI

The HTMX web UI is available at `http://<host>:<port>/lg/`. No authentication is required.

| Tab / View | URL | Description |
|------------|-----|-------------|
| Peers tab | `/lg/peers` | All peers with state, ASN names (Team Cymru), route counts. Live SSE updates. |
| Lookup tab | `/lg/lookup` | Prefix/IP lookup with inline all-peers AS path topology graph (SVG). |
| Search tab | `/lg/search` | Unified search: prefix, AS path pattern, or community. Type selector. |
| Per-peer routes | `/lg/peer/{address}` | Routes received from a specific peer (inline below peers table). |
| Route detail | `/lg/route/detail` | Expanded route attributes (HTMX fragment, click-to-expand). |
| AS path graph | `/lg/graph?prefix=X` | Server-side SVG topology from all peers (auto-loaded on lookup). |

Navigation uses a single-page tab layout with HTMX fragment swapping. The real htmx.min.js (v2.0.4) and SSE extension are embedded.

<!-- source: internal/component/lg/handler_ui.go -- UI handlers -->
<!-- source: internal/component/lg/handler_graph.go -- Graph handler -->

## Birdwatcher REST API

The looking glass exposes a birdwatcher-compatible JSON API for integration with tools like Alice-LG.

| Endpoint | Description |
|----------|-------------|
| `GET /api/looking-glass/status` | Router ID, version, uptime. |
| `GET /api/looking-glass/protocols/bgp` | Peer list with state and route counts. |
| `GET /api/looking-glass/routes/protocol/{name}` | Routes from a named peer. |
| `GET /api/looking-glass/routes/table/{family}` | Best routes by address family (URL-encode the `/` in family, e.g., `ipv4%2Funicast`). |
| `GET /api/looking-glass/routes/filtered/{name}` | Filtered routes per peer. |
| `GET /api/looking-glass/routes/search?prefix=X` | Prefix lookup across all peers. |

All API responses use `Content-Type: application/json` with birdwatcher-convention `snake_case` field names (not Ze's standard `kebab-case`).

<!-- source: internal/component/lg/handler_api.go -- API handlers and birdwatcher transform -->

## Alice-LG Integration

To use Ze as a data source for [Alice-LG](https://github.com/alice-lg/alice-lg), point Alice-LG's birdwatcher source configuration at the looking glass API:

```yaml
sources:
  - name: "Ze Router"
    type: birdwatcher
    birdwatcher:
      api: "http://ze-host:8443/api/looking-glass"
```

## Security

The looking glass is designed for public IXP deployment. It is read-only and unauthenticated. Security measures include:

- Strict input validation on all query parameters (character allowlists, length limits).
- `Content-Security-Policy`, `X-Frame-Options: DENY`, `X-Content-Type-Options: nosniff` headers.
- SSE connection limit (100 concurrent clients).
- All HTML output rendered via Go `html/template` (auto-escaped).
- No direct RIB or plugin imports; all data accessed via command dispatcher.

When TLS is enabled, the server uses TLS 1.2 minimum with the same certificate infrastructure as the web UI.
