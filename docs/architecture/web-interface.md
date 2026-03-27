# Web Interface Architecture

The ze web interface is an HTTPS server that renders YANG-driven configuration views, handles config editing with per-user draft sessions, and provides an integrated CLI bar and SSE-based live updates.

All source files in `internal/component/web/` reference this document via `// Design:` comments.

## Component Placement

| File | Responsibility |
|------|---------------|
| `internal/component/web/server.go` | HTTPS server, TLS config, self-signed cert generation |
| `internal/component/web/auth.go` | Session store, auth middleware, login handler, Basic Auth |
| `internal/component/web/handler.go` | URL parsing, content negotiation, route registration |
| `internal/component/web/handler_config.go` | Config tree view, set/delete/commit/discard handlers |
| `internal/component/web/handler_admin.go` | Admin command tree navigation and execution |
| `internal/component/web/cli.go` | CLI bar (integrated + terminal modes), tab completion |
| `internal/component/web/editor.go` | Per-user EditorManager, working tree isolation |
| `internal/component/web/render.go` | Template loading (embedded), layout/login/config rendering |
| `internal/component/web/sse.go` | EventBroker, SSE client management, config change broadcast |
| `internal/component/web/schema/` | YANG schema (`ze-web-conf.yang`) and registration |
| `internal/component/web/templates/` | HTML templates (layout, login, config views, CLI, admin) |
| `internal/component/web/assets/` | Static CSS, JS, images |
| `cmd/ze/web/main.go` | CLI entry point (`ze web` command) |

<!-- source: internal/component/web/server.go -- WebServer struct -->
<!-- source: internal/component/web/auth.go -- SessionStore, AuthMiddleware -->
<!-- source: internal/component/web/handler.go -- ParseURL, RegisterRoutes -->
<!-- source: internal/component/web/editor.go -- EditorManager -->

## URL Scheme

URLs follow a verb-first three-tier pattern. Each tier represents a different authorization level.

```
/show/<yang-path>           View tier (GET, read-only)
/monitor/<yang-path>        View tier (GET, auto-poll)
/config/<verb>/<path>       Config tier (edit/set/delete/commit/discard/compare)
/admin/<yang-path>          Admin tier (GET browse, POST execute)
/login                      Authentication (POST, no auth required)
/assets/                    Static files (GET, no auth required)
/                           Redirects to /show/
```

<!-- source: internal/component/web/handler.go -- Tier constants, knownPrefixes, configVerbs -->

The `ParseURL` function decomposes each request into a `ParsedURL` struct containing the tier, verb, YANG path segments, and negotiated format (HTML or JSON). Path segments are validated against YANG identifier characters `[a-zA-Z0-9._:-]` with explicit rejection of path traversal (`..`), empty segments, and null bytes.

## Authentication Model

Authentication reuses the SSH user database (`[]ssh.UserConfig`). Two mechanisms are supported:

| Mechanism | When Used | Session Created |
|-----------|-----------|-----------------|
| Session cookie (`ze-session`) | Browser access | Yes (on login) |
| HTTP Basic Auth | JSON API requests | No |

<!-- source: internal/component/web/auth.go -- AuthMiddleware, parseBasicAuth -->

The `SessionStore` maps tokens to `WebSession` objects and enforces one session per user. Tokens are 32 bytes from `crypto/rand`, hex-encoded to 64 characters. The session cookie is `Secure`, `HttpOnly`, and `SameSite=Strict`.

The `AuthMiddleware` wraps protected routes. It checks the session cookie first, then falls back to Basic Auth. HTMX requests with expired sessions receive a login overlay instead of a full-page redirect, enabling in-place session recovery.

## YANG-to-HTML Template Rendering

The `Renderer` loads HTML templates from `embed.FS` at startup. Templates are organized by config node kind:

| Template | Renders |
|----------|---------|
| `layout.html` | Page wrapper (breadcrumb, CLI bar, content area, notification bar) |
| `login.html` | Login form |
| `container.html` | Container nodes (leaf fields + child links) |
| `list.html` | List nodes (key enumeration + entry detail) |
| `flex.html` | Flex nodes |
| `freeform.html` | Freeform nodes (raw value lists) |
| `inline_list.html` | Inline list nodes |
| `leaf_input.html` | Partial: leaf input field (text, checkbox, number, select) |
| `command.html` | Admin command result card |
| `command_form.html` | Admin command parameter form |

<!-- source: internal/component/web/render.go -- NewRenderer, configTemplateNames -->

Config view handlers walk the YANG schema and config tree in parallel (`walkSchema`, `walkTree`) to the requested path, then assemble a `ConfigViewData` struct. Leaf nodes are mapped to HTML input types based on their YANG `ValueType` (string to text, bool to checkbox, uint16/uint32 to number with min/max, IP addresses to text with pattern validation).

<!-- source: internal/component/web/handler_config.go -- buildConfigViewData, leafInputType -->

Content negotiation determines the format: JSON requests receive the config subtree as a `map[string]any`. HTML requests render through the template pipeline. HTMX partial requests (identified by the `HX-Request` header) return content fragments without the layout wrapper.

## Per-User Editor Management

The `EditorManager` creates and manages independent `cli.Editor` instances per authenticated user. Each user session has:

- An isolated working tree (copy-on-write from the committed config)
- Change tracking (set/delete operations recorded per session)
- Serialized access via a per-user mutex

<!-- source: internal/component/web/editor.go -- EditorManager, userSession, GetOrCreate -->

Operations: `SetValue`, `DeleteValue`, `Commit`, `Discard`, `Diff`, `ChangeCount`, `Tree`, `ContentAtPath`. The `Commit` method detects conflicts when two users modify the same leaf and returns a `CommitResult` with conflict details.

Session limits: 50 concurrent sessions maximum, 1 hour idle timeout. Idle sessions are evicted on capacity overflow.

## SSE Broker Pattern

The `EventBroker` manages Server-Sent Events for live config change notifications.

<!-- source: internal/component/web/sse.go -- EventBroker, Subscribe, Broadcast -->

```
Commit handler --> BroadcastConfigChange() --> EventBroker.Broadcast()
                                                    |
                                    +---------------+---------------+
                                    |               |               |
                                client A        client B        client C
                              (chan, 16 buf)   (chan, 16 buf)  (chan, 16 buf)
```

Design choices:

| Choice | Rationale |
|--------|-----------|
| Non-blocking send to client channels | Slow clients lose events rather than blocking the broker |
| 16-event channel buffer per client | Absorbs short bursts without drops |
| 100 client maximum (configurable) | Prevents resource exhaustion |
| Pre-rendered HTML fragments as event data | Clients swap HTML directly via htmx OOB, no client-side rendering |

The `BroadcastConfigChange` function renders a notification banner template with the username and reason, then broadcasts it as a `config-change` SSE event. The banner includes "Refresh" and "Dismiss" buttons.

## CLI Bar

The CLI bar provides two modes, both using the same command grammar as the SSH CLI.

<!-- source: internal/component/web/cli.go -- HandleCLICommand, HandleCLITerminal -->

**Integrated mode** (`/cli` POST): Commands update the page via HTMX multi-target responses. Navigation commands (`edit`, `top`, `up`) swap the content area and update breadcrumbs. Mutation commands (`set`, `delete`) redirect back to the config edit view. `commit` and `discard` redirect to `/config/edit/`.

**Terminal mode** (`/cli/terminal` POST): Commands produce plain text output appended to a scrollback area. The prompt echoes `ze[<path>]#` before each command.

**Tab completion** (`/cli/complete` GET): Returns JSON array of completion candidates with text, description, and type fields. Input is capped at 1024 characters, results at 50 candidates.

**Mode toggle** (`/cli/mode` POST): Switches between integrated and terminal modes, re-rendering the content area for the target mode.

## Admin Command Tree

The `/admin/` tier provides a browsable command tree. The `HandleAdminView` handler receives a static `children` map describing the command hierarchy. Container paths display navigable links. Leaf paths display a parameter form.

<!-- source: internal/component/web/handler_admin.go -- HandleAdminView, HandleAdminExecute -->

The `HandleAdminExecute` handler reconstructs the command string from URL path segments, dispatches through a `CommandDispatcher` function, and returns a result card (HTML) or JSON object with command, output, and error fields.

## TLS

The web server enforces TLS 1.2 as the minimum version. When no certificate is provided, `GenerateWebCertWithAddr` creates an ECDSA P-256 self-signed certificate valid for 365 days with SANs for localhost, 127.0.0.1, ::1, and the configured listen address (if non-loopback). The `CertStore` interface abstracts certificate persistence for reuse across restarts.

<!-- source: internal/component/web/server.go -- NewTLSConfig, GenerateWebCertWithAddr, CertStore -->
