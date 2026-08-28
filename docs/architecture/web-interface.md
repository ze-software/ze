# Web Interface Architecture

The ze web interface is an HTTPS server that renders YANG-driven configuration views using HTMX components. All UI is server-rendered, from typed Go components the templ generator compiles. HTMX handles navigation, auto-save, and error display via out-of-band swaps. The only JavaScript is `cli.js` for Tab/? autocomplete in the CLI bar.

For the component design, template filesystem, and interaction flows, see [web-components.md](web-components.md).

All source files in `internal/component/web/` reference this document via `// Design:` comments.

## Source Files

| File | Responsibility |
|------|---------------|
| `server.go` | HTTPS server, TLS config, self-signed cert generation, cert persistence |
| `auth.go` | Session store, auth middleware, login handler, Basic Auth, `GetUsernameFromRequest` |
| `handler.go` | URL parsing, content negotiation, route registration |
| `fragment.go` | HTMX fragment handler, `FragmentData`, `FieldMeta`, sidebar builder |
| `error_fragment.go` | What a refused request is answered with: `WriteOOBError`, which writes the error fragment for the request's target and then the out-of-band error item. The fragment itself, and the middleware that renders one from the plain-text body `http.Error` writes, are shared: `errorfragment.Render` and `errorfragment.Middleware` (`internal/core/errorfragment`). The middleware converts a 4xx or 5xx answer only when the request carries `HX-Request` and the body is text/plain, so a JSON client, a plain browser navigation and a handler's own markup each reach the client untouched. `ServerHandler` (`auth.go`) is the one place it wraps the mux |
| `handler_config.go` | Config set/delete/commit/discard handlers, `ConfigViewData`, `HandleConfigView` |
| `handler_config_walk.go` | Schema + tree walking, `buildConfigViewData`, `populateContainerView` |
| `handler_config_leaf.go` | `buildLeafField`, `leafInputType`, `configViewComponent`, breadcrumbs |
| `handler_admin.go` | Admin command tree navigation and execution; YANG-derived tree via `AdminTreeFromYANG`. When the YANG loader fails at hub startup the admin nav is empty (and the failure is logged to stderr) rather than falling back to a stale static map; an empty admin nav is operator-visible feedback that the hub did not load its command modules. |
| `cli.go` | CLI bar (integrated + terminal modes), tab completion |
| `editor.go` | Per-user `EditorManager`, working tree isolation, change tracking |
| `render.go` | `Renderer`: embedded assets, decorators, and the entry points the hub calls (`RenderLayout`, `RenderLogin`, `RenderWorkbench`, `RenderField`, `RenderDiffModal`) |
| `sse.go` | `EventBroker`, SSE client management, config change broadcast |
| `ui_mode.go` | `UIMode` selector for the workbench experiment (Phase 4 default flip pending) |
| `handler_workbench.go` | workbench shell handler; reuses fragment data path with workbench chrome |
| `workbench_sections.go` | Left-nav section taxonomy (Dashboard/Routing/Logs/...) |
| `workbench_enrich.go` | Promotes any named list to a workbench table; attaches per-row tools and pending markers |
| `handler_tools.go` | `POST /tools/related/run`: resolves `ze:related` descriptors, dispatches via `CommandDispatcher`, renders the overlay |
| `related_resolver.go` | Placeholder substitution for `ze:related` command templates against the user's working tree |

<!-- source: internal/component/web/server.go -- WebServer struct -->
<!-- source: internal/component/web/auth.go -- SessionStore, authMiddleware -->
<!-- source: internal/component/web/fragment.go -- HandleFragment, FragmentData -->
<!-- source: internal/component/web/editor.go -- EditorManager -->

## Component Structure

Every page, fragment and editor is a templ component. A component is a Go
function, so the markup and the view model are checked against each other at
build time. The sources sit in the package directory, named by visual concern:

```
internal/component/web/
  page_*.templ        layout, login, workbench
  component_*.templ   breadcrumb, path_bar, sidebar, detail, finder,
                      list_table, log_table, log_live, add_form_overlay,
                      command_form, command_result, cli_bar, commit_bar,
                      error_panel, diff_modal, notification_error,
                      dashboard_*, oob_*, workbench_*, tool_*
  config_*.templ      breadcrumb, command, command_form, commit, container,
                      list, inline_list, freeform, flex, leaf_input,
                      notification
  input_*.templ       wrapper, bool, enum, number, text
  l2tp_*.templ        list, detail
  notification_banner.templ, terminal.templ
```

A name ending in `_*` above is a family: every file carrying that prefix
belongs to the line it sits on. Each other name is one file.

`./le repository generate` compiles each `.templ` into a `*_templ.go` beside it, and
`./le doc-check templ-output` refuses a source whose generated file is stale.

Each input type is one file. `fieldInputFor` (`field_input.go`) reads the `fieldInputs` registry, which maps a YANG field type onto the component that edits it. A type nobody registered reaches the text editor by a named rule. No if/else chain in the markup.

<!-- source: internal/component/web/render.go -- NewRenderer, Renderer -->
<!-- source: internal/component/web/field_input.go -- fieldInputs, fieldInputFor -->

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
/tools/related/run          workbench: execute a related operator tool (POST)
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

<!-- source: internal/component/web/auth.go -- authMiddleware, parseBasicAuth -->

Session tokens: 32 bytes from `crypto/rand`, hex-encoded. Cookie: `Secure`, `HttpOnly`, `SameSite=Strict`. One session per user, 24h TTL.

A session also ends when the running configuration stops declaring its user. The session records which backend authenticated it (`AuthResult.Source`), and `validateToken` re-checks a session the LOCAL backend granted against the credentials the running configuration declares right now, on every request. An operator removed by a reload loses an open browser tab at once, with no restart and no wait for the TTL. A session a remote backend granted (RADIUS, TACACS+) is not checked against the local list, because that list never granted it.

<!-- source: internal/component/web/auth.go -- SessionStore.validateToken, webSession -->
<!-- source: cmd/ze/hub/main_servers.go -- liveLocalUsers, liveConfigUsers -->

An SSE stream that is already open survives the removal until the client disconnects: it authenticates at connect and then blocks for the life of the connection, so no later request exists to refuse. Every mutation route is a fresh request and is refused.

## Per-User Editor

The `EditorManager` creates independent `cli.Editor` instances per authenticated user.

<!-- source: internal/component/web/editor.go -- EditorManager, GetOrCreate -->

Each session has an isolated working tree, change tracking, and serialized access via per-user mutex. Operations: `SetValue`, `DeleteValue`, `Commit`, `Discard`, `Diff`, `ChangeCount`, `Tree`.

`Commit` detects conflicts when two users modify the same leaf and returns `CommitResult` with conflict details. Limits: 50 concurrent sessions, 1 hour idle timeout.

Editors with a reload hook commit transactionally: `CommitSessionCandidate` stages a candidate version, the hook (`reloadAfterCommit`) reloads the daemons and promotes the candidate. Editors without a hook write `config.conf` directly via `CommitSession`. The web editor manager and the SSH session factory both wire the hook, so a commit from either surface reaches the running daemons ("commit = apply + propagate").
<!-- source: cmd/ze/hub/session_factory.go -- newSessionEditor reload notifier wiring -->
<!-- source: cmd/ze/hub/main.go -- sessionReloadHolder late binding -->

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

<!-- source: internal/core/selfcert/selfcert.go -- GenerateWebCertWithAddr, LoadOrGenerateCert, addInterfaceIPs, CertStore -->

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
| Env vars | `ze.web.listen=ip:port`, `ze.web.enabled=true`, `ze.web.insecure=true`, `ze.web.ui-mode=finder` (rollback to legacy Finder) |

Configured and web-only startup call `startWebServer` in
`cmd/ze/hub/service_web.go`. Web-only mode starts the server without a BGP
config for initial setup.
<!-- source: cmd/ze/hub/service_web.go -- startWebServer -->

## Workbench UI (default)

The RouterOS-style operator workbench is the default UI. Set `ze.web.ui-mode=finder` to roll back to the legacy Finder shell.

| Region | Source |
|--------|--------|
| Top bar (identity, breadcrumb, actions) | `component_workbench_topbar.templ` |
| Left nav (Dashboard / Routing / Logs / ...) | `component_workbench_nav.templ` driven by `workbench_sections.go` |
| Workspace (table + detail) | reuses the existing `detail` fragment so list tables and fields render unchanged inside the new chrome |
| Tool overlay container `#tool-overlays` | `component_tool_overlay.templ`. HTMX `hx-swap="beforeend"` appends each instance as a sibling, so several overlays can pin |
| Commit bar / diff modal / error panel | reused unchanged from Finder; CLI bar removed (available as `/cli` tab) |

### Related-tool execution

`POST /tools/related/run` accepts only `tool_id` and `context_path` (plus an optional `confirm=true` for destructive tools). Raw command text is never trusted from the form. The handler:

1. Walks the schema to the context node and looks up the descriptor by id.
2. Returns a confirmation overlay if the descriptor declares `confirm=...` and the operator has not confirmed.
3. Resolves placeholder substitutions against the user's working tree via `related_resolver.go` (rejects unsafe values, caps depth at 16 segments, caps resolved command at 4096 chars).
4. Dispatches via the standard `CommandDispatcher(command, username, remoteAddr)` so authz, accounting, and peer-selector extraction live in one place.
5. ANSI-strips and 4-MiB-truncates output, splits the first 128 KiB inline and the rest into a `<details>` "Show full output" disclosure, HTML-escapes every leg.

See `spec-web-2-operator-workbench.md` (Argument Wire Format, Resolved-Value Validation, Day-One BGP Related Tools) for the full descriptor grammar and the BGP YANG annotations that ship with the experiment.


## Looking Glass

The looking glass is a separate HTTP server (`internal/component/lg/`) that provides public, read-only access to BGP state. It runs on its own port (default 8443), independent from the authenticated web UI.

All source files in `internal/component/lg/` reference this document via `// Design:` comments.

### LG Source Files

| File | Responsibility |
|------|---------------|
| `server.go` | HTTP server lifecycle, mux setup, TLS support, CommandDispatcher. `NewLGServer` builds the one handler chain: security headers over `errorfragment.Middleware` over the bearer gate over the mux |
| `handler_api.go` | Birdwatcher-compatible REST API (JSON, snake_case), input validation |
| `handler_ui.go` | HTMX pages (peers, lookup, search), SSE events, asset serving |
| `handler_graph.go` | AS path topology SVG endpoint |
| `graph.go` | Graph data model (nodes/edges from AS paths), prepending dedup |
| `layout.go` | Layered layout algorithm, SVG rendering |
| `render.go` | templ component rendering, page vs fragment detection |
| `view.go` | One named view-model struct per page, which the components take |
| `*.templ` | The markup, compiled into `*_templ.go` by `./le repository generate` |
| `embed.go` | Embedded assets (CSS, HTMX, SSE) via go:embed |
| `auth.go` | Optional bearer-token gate over the whole mux |

<!-- source: internal/component/lg/server.go -- LGServer, NewLGServer -->
<!-- source: internal/component/lg/handler_api.go -- API handlers, birdwatcher transform -->
<!-- source: internal/component/lg/handler_ui.go -- UI handlers, SSE -->
<!-- source: internal/component/lg/graph.go -- buildGraph, extractASPath -->
<!-- source: internal/component/lg/layout.go -- computeLayout, renderGraphSVG -->

### LG Refused Requests

A handler refuses with `http.Error`, which writes a bare status line. The chain
in `NewLGServer` carries `errorfragment.Middleware`
(`internal/core/errorfragment`), so a request carrying `HX-Request` receives the
same error fragment the web UI answers with, and htmx can swap it into the
element the request named. Everything else is untouched: the JSON API refuses
through `writeJSONError`, and a plain browser or a script still reads the status
line. `.error-fragment` in `assets/style.css` is what an operator sees.

### LG Bearer Token

The looking glass is open by default, because it is a public read-only surface.
When `token` is set, `bearerAuth` wraps the mux before the security headers, so
a route added later is gated by construction. A request with no
`Authorization` header, a different scheme, or a wrong token gets `401` with
`WWW-Authenticate: Bearer realm="looking glass"`. The compare is
`subtle.ConstantTimeCompare` over SHA-256 digests, so neither the token length
nor its content leaks through timing. RFC 7235 Section 2.1 makes the scheme
case-insensitive, so `bearer <token>` is accepted. The token itself stays
case-sensitive.

<!-- source: internal/component/lg/auth.go -- bearerAuth, bearerTokenMatches -->

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
| Env vars | `ze.looking-glass.listen=ip:port`, `ze.looking-glass.enabled=true`, `ze.looking-glass.tls=false` (TLS is on by default), `ze.looking-glass.token=<token>` |

The hub builds the looking-glass service with `buildLGService` and starts its
server with `serveLG` in `cmd/ze/hub/service_lg.go`.

<!-- source: cmd/ze/hub/service_lg.go -- buildLGService, serveLG -->
