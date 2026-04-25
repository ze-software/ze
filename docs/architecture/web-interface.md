# Web Interface Architecture

The ze web interface is an HTTPS server that renders YANG-driven configuration views using HTMX components. All UI is server-rendered Go templates. HTMX handles navigation, auto-save, and error display via out-of-band swaps. The only JavaScript is `cli.js` for Tab/? autocomplete in the CLI bar.

For the component design, template filesystem, and interaction flows, see [web-components.md](web-components.md).

All source files in `internal/component/web/` reference this document via `// Design:` comments.

## Source Files

| File | Responsibility |
|------|---------------|
| `server.go` | HTTPS server, TLS config, self-signed cert generation, cert persistence |
| `auth.go` | Session store, auth middleware, login handler, Basic Auth, `GetUsernameFromRequest` |
| `handler.go` | URL parsing, content negotiation, route registration |
| `fragment.go` | HTMX fragment handler, `FragmentData`, `FieldMeta`, sidebar builder, OOB error writer |
| `handler_config.go` | Config set/delete/commit/discard handlers, `ConfigViewData`, `HandleConfigView` |
| `handler_config_walk.go` | Schema + tree walking, `buildConfigViewData`, `populateContainerView` |
| `handler_config_leaf.go` | `buildLeafField`, `leafInputType`, `nodeKindToTemplate`, breadcrumbs |
| `handler_admin.go` | Admin command tree navigation and execution; YANG-derived tree via `AdminTreeFromYANG`. When the YANG loader fails at hub startup the admin nav is empty (and the failure is logged to stderr) rather than falling back to a stale static map; an empty admin nav is operator-visible feedback that the hub did not load its command modules. |
| `cli.go` | CLI bar (integrated + terminal modes), tab completion |
| `editor.go` | Per-user `EditorManager`, working tree isolation, change tracking |
| `render.go` | Template loading (embedded), `RenderFragment`, `fieldFor` dispatch |
| `sse.go` | `EventBroker`, SSE client management, config change broadcast |
| `ui_mode.go` | `UIMode` selector for the V2 workbench experiment (Phase 4 default flip pending) |
| `handler_workbench.go` | V2 workbench shell handler; reuses fragment data path with workbench chrome |
| `workbench_sections.go` | Left-nav section taxonomy (Dashboard/Routing/Logs/...) |
| `workbench_enrich.go` | Promotes any named list to a workbench table; attaches per-row tools and pending markers |
| `handler_tools.go` | `POST /tools/related/run`: resolves `ze:related` descriptors, dispatches via `CommandDispatcher`, renders the overlay |
| `related_resolver.go` | Placeholder substitution for `ze:related` command templates against the user's working tree |

<!-- source: internal/component/web/server.go -- WebServer struct -->
<!-- source: internal/component/web/auth.go -- SessionStore, AuthMiddleware -->
<!-- source: internal/component/web/fragment.go -- HandleFragment, FragmentData -->
<!-- source: internal/component/web/editor.go -- EditorManager -->

## Template Structure

Templates are organized by visual concern:

```
templates/
  page/        layout.html, login.html
  component/   breadcrumb, sidebar, detail, cli_bar, commit_bar,
               error_panel, diff_modal, oob_response, oob_save, oob_error
  input/       wrapper, bool, enum, number, text
```

Each input type is one file. The `fieldFor()` template function dispatches to `input_<type>` at render time based on the YANG `ValueType`. No if/else chain in templates.

<!-- source: internal/component/web/render.go -- NewRenderer, fieldFor func -->

## URL Scheme

```
/show/<yang-path>           Full page or HTMX fragment (GET)
/fragment/detail?path=X     HTMX partial: detail + OOB sidebar/breadcrumb (GET)
/config/set/<path>          Save field value (POST, returns OOB commit bar or error)
/config/diff                Diff modal with changes (GET, returns open modal HTML)
/config/diff-close          Close diff modal (GET, returns closed modal HTML)
/config/commit              Apply pending changes (POST)
/config/discard             Revert pending changes (POST)
/cli                        CLI command execution (POST)
/cli/complete?input=X       Tab/? autocomplete (GET, returns JSON)
/cli/terminal               Terminal mode command (POST, returns plain text)
/cli/mode                   Toggle CLI/GUI mode (POST)
/admin/<yang-path>          Admin commands (GET browse, POST execute)
/tools/related/run          V2 workbench: execute a related operator tool (POST)
/login                      Authentication (POST)
/assets/                    Static files (CSS, JS)
/                           Redirects to /show/
```

<!-- source: internal/component/web/handler.go -- ParseURL, knownPrefixes -->

## Authentication

Reuses SSH user database (`[]ssh.UserConfig`). Two mechanisms:

| Mechanism | When Used | Session Created |
|-----------|-----------|-----------------|
| Session cookie (`ze-session`) | Browser access | Yes (on login) |
| HTTP Basic Auth | JSON API requests | No |

<!-- source: internal/component/web/auth.go -- AuthMiddleware, parseBasicAuth -->

Session tokens: 32 bytes from `crypto/rand`, hex-encoded. Cookie: `Secure`, `HttpOnly`, `SameSite=Strict`. One session per user, 24h TTL.

## Per-User Editor

The `EditorManager` creates independent `cli.Editor` instances per authenticated user.

<!-- source: internal/component/web/editor.go -- EditorManager, GetOrCreate -->

Each session has an isolated working tree, change tracking, and serialized access via per-user mutex. Operations: `SetValue`, `DeleteValue`, `Commit`, `Discard`, `Diff`, `ChangeCount`, `Tree`.

`Commit` detects conflicts when two users modify the same leaf and returns `CommitResult` with conflict details. Limits: 50 concurrent sessions, 1 hour idle timeout.

## YANG Schema Integration

The YANG schema drives the entire UI. No hardcoded field lists.

| Schema element | UI rendering |
|---------------|-------------|
| `ContainerNode` | Sidebar heading (clickable, navigable) |
| `ListNode` | Sidebar heading + entry list + add form |
| `LeafNode` type `TypeBool` | Toggle button (on/off) |
| `LeafNode` type `TypeUint16/32` | Number input with min/max |
| `LeafNode` type `TypeIP/IPv4/IPv6` | Text input with pattern validation |
| `LeafNode` type `TypeString` with `Enums` | Select dropdown |
| `LeafNode` type `TypeString` | Text input |
| `LeafNode.Description` | (i) tooltip on hover (field label and sidebar heading) |
| `ContainerNode.Description` | (i) tooltip on sidebar heading |
| `ListNode.Description` | (i) tooltip on sidebar heading |

<!-- source: internal/component/config/schema.go -- LeafNode, ContainerNode, ListNode -->
<!-- source: internal/component/web/fragment.go -- buildFieldMeta, nodeDescription -->

## TLS

Self-signed ECDSA P-256 certificate, valid 365 days. When listening on `0.0.0.0`, all non-loopback interface IPs are added as SANs so the cert is valid regardless of which IP the client connects to.

Certificates are persisted in zefs (`meta/web/cert`, `meta/web/key`) via the `CertStore` interface. On restart, the existing cert is loaded instead of regenerated, so browsers don't need to re-accept.

TLS handshake errors from browsers rejecting self-signed certs are suppressed in the server error log.

<!-- source: internal/component/web/server.go -- GenerateWebCertWithAddr, LoadOrGenerateCert, addInterfaceIPs -->

## Security Headers

```
Content-Security-Policy: default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'
X-Frame-Options: DENY
X-Content-Type-Options: nosniff
Strict-Transport-Security: max-age=63072000; includeSubDomains
Cache-Control: no-store
```

No `unsafe-eval`. All scripts are external files. No inline `<script>` blocks.

<!-- source: internal/component/web/auth.go -- addSecurityHeaders -->

## Starting the Web Server

| Method | Command |
|--------|---------|
| CLI flag | `ze start --web <port>` |
| Config | `environment { web { enabled true; server main { ip 0.0.0.0; port 3443; } } }` |
| Env vars | `ze.web.listen=ip:port`, `ze.web.enabled=true`, `ze.web.insecure=true`, `ze.web.ui=workbench` (V2 experiment) |

Both paths call `startWebServer()` in `cmd/ze/hub/main.go`. Web-only mode (no BGP config) starts the web server standalone for initial setup.

## V2 Workbench (experiment)

`spec-web-2-operator-workbench.md` introduces a RouterOS-style operator workbench as a V2 experiment behind `ze.web.ui=workbench`. The default stays at `finder` until `bin/ze-test web -p workbench-bgp-change-verify` passes every Promotion Criteria threshold; flipping the default is a one-line change in `internal/component/config/environment.go`.

| Region | Source |
|--------|--------|
| Top bar (identity, breadcrumb) | `templates/component/workbench_topbar.html` |
| Left nav (Dashboard / Routing / Logs / ...) | `templates/component/workbench_nav.html` driven by `workbench_sections.go` |
| Workspace (table + detail) | reuses the existing `detail` fragment so list tables and fields render unchanged inside the new chrome |
| Tool overlay container `#tool-overlays` | `templates/component/tool_overlay.html`; HTMX `hx-swap="beforeend"` appends each instance as a sibling so multiple overlays can pin |
| Commit bar / CLI bar / diff modal / error panel | reused unchanged from Finder |

### Related-tool execution

`POST /tools/related/run` accepts only `tool_id` and `context_path` (plus an optional `confirm=true` for destructive tools). Raw command text is never trusted from the form. The handler:

1. Walks the schema to the context node and looks up the descriptor by id.
2. Returns a confirmation overlay if the descriptor declares `confirm=...` and the operator has not confirmed.
3. Resolves placeholder substitutions against the user's working tree via `related_resolver.go` (rejects unsafe values, caps depth at 16 segments, caps resolved command at 4096 chars).
4. Dispatches via the standard `CommandDispatcher(command, username, remoteAddr)` so authz, accounting, and peer-selector extraction live in one place.
5. ANSI-strips and 4-MiB-truncates output, splits the first 128 KiB inline and the rest into a `<details>` "Show full output" disclosure, HTML-escapes every leg.

See `spec-web-2-operator-workbench.md` (Argument Wire Format, Resolved-Value Validation, Day-One BGP Related Tools) for the full descriptor grammar and the BGP YANG annotations that ship with the experiment.

<!-- source: cmd/ze/hub/main.go -- startWebServer, RunWebOnly -->

## Looking Glass

The looking glass is a separate HTTP server (`internal/component/lg/`) that provides public, read-only access to BGP state. It runs on its own port (default 8443), independent from the authenticated web UI.

All source files in `internal/component/lg/` reference this document via `// Design:` comments.

### LG Source Files

| File | Responsibility |
|------|---------------|
| `server.go` | HTTP server lifecycle, mux setup, TLS support, CommandDispatcher |
| `handler_api.go` | Birdwatcher-compatible REST API (JSON, snake_case), input validation |
| `handler_ui.go` | HTMX pages (peers, lookup, search), SSE events, asset serving |
| `handler_graph.go` | AS path topology SVG endpoint |
| `graph.go` | Graph data model (nodes/edges from AS paths), prepending dedup |
| `layout.go` | Layered layout algorithm, SVG rendering |
| `render.go` | Go html/template rendering, page vs fragment detection |
| `embed.go` | Embedded assets (CSS, HTMX, SSE) and templates via go:embed |

<!-- source: internal/component/lg/server.go -- LGServer, NewLGServer -->
<!-- source: internal/component/lg/handler_api.go -- API handlers, birdwatcher transform -->
<!-- source: internal/component/lg/handler_ui.go -- UI handlers, SSE -->
<!-- source: internal/component/lg/graph.go -- buildGraph, extractASPath -->
<!-- source: internal/component/lg/layout.go -- computeLayout, renderGraphSVG -->

### LG URL Scheme

```
/api/looking-glass/status              Router status (JSON, birdwatcher format)
/api/looking-glass/protocols/bgp       Peer list (JSON)
/api/looking-glass/routes/protocol/X   Routes from peer X (JSON)
/api/looking-glass/routes/table/X      Best routes by family (JSON)
/api/looking-glass/routes/filtered/X   Filtered routes per peer (JSON)
/api/looking-glass/routes/search?prefix=X  Prefix lookup (JSON)
/lg/peers                              Peer dashboard (HTML)
/lg/lookup                             Route lookup form (HTML)
/lg/search                             Unified search: prefix, AS path, community (HTML)
/lg/peer/{address}                     Per-peer routes (HTML)
/lg/route/detail                       Route detail fragment (HTMX)
/lg/graph?prefix=X                     AS path topology (SVG)
/lg/events                             SSE peer state stream
/lg/assets/                            Static CSS/JS
```

### LG Data Access

All BGP data is queried via `CommandDispatcher` (same `func(string) (string, error)` as the web UI's admin handlers). The LG never imports RIB or peer plugin packages. The dispatcher routes commands to the engine, preserving plugin isolation.

### LG JSON Convention Exception

The birdwatcher API uses `snake_case` JSON keys (`router_id`, `neighbor_address`, `routes_received`) instead of Ze's standard `kebab-case`. This is intentional for compatibility with Alice-LG and other birdwatcher consumers.

### Starting the Looking Glass

| Method | Config |
|--------|--------|
| Config | `environment { looking-glass { enabled true; server main { ip 0.0.0.0; port 8443; } } }` |
| Env vars | `ze.looking-glass.listen=ip:port`, `ze.looking-glass.enabled=true`, `ze.looking-glass.tls=true` |

Started by `startLGServer()` in `cmd/ze/hub/main.go` alongside the web server, after engine startup.

<!-- source: cmd/ze/hub/main.go -- startLGServer, serveLG -->
