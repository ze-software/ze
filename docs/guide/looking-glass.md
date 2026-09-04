---
title: Looking Glass (Guide)
---
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

<!-- source: internal/component/lg/yang/ze-lg-conf.yang -- YANG schema -->

| Setting | Default | Description |
|---------|---------|-------------|
| `enabled` | `false` | Enable the looking glass server. |
| `server <name> { ip }` | `0.0.0.0` | Listen address. Set to `127.0.0.1` to restrict to local access. |
| `server <name> { port }` | `8443` | Listen port. Must differ from the web UI port. |
| `tls` | `true` | Serve HTTPS. Set `false` to serve plaintext. |
| `certificate` | (empty) | Name of a `pki { certificate <name> }` entry to serve. Empty means a self-signed certificate from blob storage (`ze init`). |
| `token` | (empty) | Bearer token. When set, every `/api/` and `/lg/` route needs `Authorization: Bearer <token>`. Empty leaves the looking glass open. |

Environment variable overrides: `ze.looking-glass.listen=ip:port`, `ze.looking-glass.enabled=true`, `ze.looking-glass.tls=false`, `ze.looking-glass.token=<token>`, `ze.looking-glass.certificate=<name>`.

### TLS is on by default

The looking glass binds `0.0.0.0` and publishes route data and session state, so
it serves HTTPS unless you turn TLS off. Two rules apply:

- Write `tls false` (or set `ze.looking-glass.tls=false`) to serve plaintext,
  for example behind a proxy that terminates TLS.
- With no `certificate` and no blob storage there is no self-signed certificate
  to serve. If you wrote `tls true`, Ze reports the error and does not start the
  looking glass. If you wrote nothing and took the default, Ze serves plaintext
  and prints a warning that names `ze init` as the remedy.

### Serve your own certificate

A visitor to a public looking glass has not installed your root, so a
self-signed certificate gives them a browser warning. Set `certificate` to the
name of a `pki { certificate <name> }` entry and the listener serves that leaf
and every intermediate the entry holds:

```
environment {
    looking-glass {
        enabled     true
        certificate lan
    }
}
```

| Rule | Detail |
|------|--------|
| Default | No `certificate` leaf means Ze generates and serves a self-signed certificate, which is the behavior of every release before this leaf existed. |
| Fail closed | A name the store does not hold, or an entry with no `private { key }`, refuses the start: Ze exits and names the missing certificate. A reload that names one is rejected as a whole, and the running looking glass keeps the chain it is serving. Ze never falls back to a self-signed certificate for a name you configured. |
| Rotation | Load new material under the same name and reload: the listener serves the new chain from the next handshake, with no rebind, so a viewer's open connection survives. |
| TLS off | `tls false` serves no certificate, so the leaf is inert. Neither the start nor a reload reads the name. |
| Plaintext by downgrade | You took the `tls` default and have no blob store, so Ze dropped the looking glass to plaintext at start. A `certificate` name you add later changes nothing there. The reload is accepted and rotates nothing. Restart Ze to serve the named chain over TLS. |
| No blob storage needed | The material comes from the `pki {}` container, so a named certificate serves on a deployment that never ran `ze init`. The blob store holds the self-signed certificate only. |
| Name | 1 to 255 characters, `A-Z a-z 0-9 . _ -`. It is a store key, never a file path. |
| Own leaf | The looking glass and the web UI have separate `certificate` leaves, so each listener serves the certificate that matches its own hostname. |
| Env override | `ze.looking-glass.certificate` takes precedence over the config file. |

<!-- source: internal/component/lg/yang/ze-lg-conf.yang -- leaf certificate -->
<!-- source: cmd/ze/hub/service_tls.go -- listenerTLSMaterial (fail-closed selection) -->
<!-- source: cmd/ze/hub/main.go -- runYANGConfig, the looking-glass certificate startup gate -->
<!-- source: cmd/ze/hub/main_reload.go -- lgCertificateName and the reload refusal -->
<!-- source: cmd/ze/hub/listener_migrate.go -- updateLGCertificate (rotation without rebind) -->

### Optional bearer token

A looking glass is normally public and read-only, so `token` is off by default.
Set it to require a bearer token on every route. Ze compares the token
constant-time over SHA-256 digests. A request with no token, a wrong token, or
an `Authorization` header that is not `Bearer <token>` gets `401`.

When the `looking-glass` block is absent, no HTTP server is started and no resources are consumed.

<!-- source: cmd/ze/hub/service_lg.go -- buildLGService -->

## Web UI

The HTMX web UI is available at `https://<host>:<port>/lg/` (`http://` when you
set `tls false`). No authentication is required unless you set `token`.

| Tab / View | URL | Description |
|------------|-----|-------------|
| Peers tab | `/lg/peers` | All peers with state, ASN names (Team Cymru), route counts. Live SSE updates. |
| Lookup tab | `/lg/lookup` | Prefix/IP lookup with inline all-peers AS path topology graph (SVG). |
| Search tab | `/lg/search` | Unified search: prefix, AS path pattern, or community. Type selector. |
| Per-peer routes | `/lg/peer/{address}` | Routes received from a specific peer (inline below peers table). |
| Route detail | `/lg/route/detail` | Expanded route attributes (HTMX fragment, click-to-expand). |
| AS path graph | `/lg/graph?prefix=X` | Server-side SVG topology from all peers (auto-loaded on lookup). |

Navigation uses a single-page tab layout with HTMX fragment swapping. The real htmx.min.js (v4.0.0) is embedded, and the peers page also loads hx-sse.min.js, htmx 4's SSE extension.

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

The per-peer route counts on `/protocols/bgp` come from the `bgp-rib` plugin's
Adj-RIB-In and Adj-RIB-Out sizes, merged into `show bgp`:
`routes_received` and `routes_imported` are both the Adj-RIB-In size (Ze retains
only accepted routes, so there is no distinct pre-policy received count here),
and `routes_exported` is the Adj-RIB-Out size. `routes_filtered` is always `0`:
Ze does not retain import-filtered routes (unlike BIRD's "import keep filtered"),
so the `/routes/filtered/{name}` endpoint also returns an empty list. Route
counts are only present when the `bgp-rib` plugin is loaded.

<!-- source: internal/component/bgp/plugins/cmd/peer/summary.go -- fetchRibRouteCounts, mergeRibRouteCounts -->
<!-- source: internal/component/bgp/plugins/rib/rib_commands.go -- status per-peer route-counts -->
<!-- source: internal/component/lg/handler_api.go -- API handlers and birdwatcher transform -->

## Alice-LG Integration

To use Ze as a data source for [Alice-LG](https://github.com/alice-lg/alice-lg), point Alice-LG's birdwatcher source configuration at the looking glass API:

```yaml
sources:
  - name: "Ze Router"
    type: birdwatcher
    birdwatcher:
      api: "https://ze-host:8443/api/looking-glass"
```

## Security

The looking glass is designed for public IXP deployment. It is read-only, and open unless you set `token`. Security measures include:

- Strict input validation on all query parameters (character allowlists, length limits).
- `Content-Security-Policy`, `X-Frame-Options: DENY`, `X-Content-Type-Options: nosniff` headers.
- SSE connection limit (100 concurrent clients).
- All HTML output rendered through templ, which escapes every interpolated value. The two SVG graph builders stay in Go and escape their labels with `template.HTMLEscapeString`.
- The `Content-Security-Policy` is `default-src 'self'`, so no page carries an inline script or an inline event handler. A test refuses one in any `.templ` source of the package.
- No direct RIB or plugin imports; all data accessed via command dispatcher.
<!-- source: internal/component/lg/markup_check_test.go -- TestTemplatesAvoidInlineScriptAndStyle, lgMarkupExempt -->
<!-- source: internal/component/lg/layout.go -- renderGraphSVG label escaping -->

When TLS is enabled, the server uses TLS 1.2 minimum. It reads its own
`certificate` leaf, resolved against the same PKI store the web UI reads, and
serves a self-signed certificate when that leaf is unset.
