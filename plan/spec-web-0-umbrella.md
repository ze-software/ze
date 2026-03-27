# Spec: web-0 -- Web Interface (Umbrella)

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-03-27 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `internal/component/cli/editor.go` -- Editor struct, schema walking, draft/commit
4. `internal/component/config/schema.go` -- Node types (ContainerNode, ListNode, LeafNode)
5. `internal/component/config/tree.go` -- Runtime config tree
6. `internal/component/cli/editor_walk.go` -- schemaGetter, walkOrCreateIn, walkPath
7. `internal/component/ssh/auth.go` -- AuthenticateUser, CheckPassword, UserConfig
8. `internal/component/cli/editor_session.go` -- EditSession, per-user change files
9. `internal/chaos/web/` -- existing HTMX+SSE web patterns (chaos dashboard)
10. Child specs: `spec-web-1-*` through `spec-web-6-*`

## Task

Add a web interface to Ze that is generated from YANG schemas, mirroring the CLI experience over HTTPS. The web UI enables config viewing, editing, and operational command execution through a browser. It uses Go `html/template` for rendering and HTMX for dynamic partial updates.

The web interface is the HTTP equivalent of the SSH CLI: same YANG schemas, same editor infrastructure, same config tree, same command grammar. From the engine's perspective, a web session is indistinguishable from a CLI session.

### Vision

The web UI maps the YANG schema tree to a navigable HTML interface:

- **Breadcrumb navigation** at the top mirrors the CLI's `contextPath` -- each path segment is a clickable link
- **List nodes** render with a key list on the left panel and selected entry detail on the right panel
- **Container nodes** render full-width with leaves as form fields and sub-containers as navigable links
- **Verb-first URLs** (`/config/set/...`, `/config/delete/...`, `/config/commit`, `/config/discard`) place the verb as the first segment after the prefix, creating a positionally unambiguous mapping to CLI commands
- **Persistent CLI input bar** at the bottom accepts the same command grammar as the SSH CLI
- **Two CLI modes**: integrated (commands drive the GUI) and terminal (full text CLI over HTTPS)

### Design Decisions (agreed with user)

#### D-1: Placement

| Aspect | Decision |
|--------|----------|
| Component code | `internal/component/web/` |
| CLI entry point | `cmd/ze/web/main.go` |
| Rationale | Cross-cutting like CLI. The "delete the folder" test rules out `plugins/` -- the web UI serves all config, not just BGP. Parallel to how `internal/component/cli/` is placed |

#### D-2: Read-Write Editor

| Aspect | Decision |
|--------|----------|
| Mode | Full editor: set, delete, commit, discard |
| Rationale | User decided. The web UI is not a read-only dashboard -- it is a config editor |

#### D-3: Scope

| Aspect | Decision |
|--------|----------|
| Config YANG | Yes -- `ze-*-conf.yang` modules (all config schemas) |
| API/command YANG | Yes -- `ze-*-api.yang` and `ze-*-cmd.yang` modules (operational commands) |
| Rationale | User decided. Both config editing and operational command execution are in scope |

#### D-4: Config View Model

| Aspect | Decision |
|--------|----------|
| Configured values | Shown in form fields with current values |
| Available options | Shown as empty fields or "add" actions for unconfigured leaves |
| Default values | Shown with visual distinction (e.g., placeholder text) so users can override |
| Rationale | User decided. The UI shows what IS configured and presents options for what COULD be configured |

#### D-5: Authentication

| Aspect | Decision |
|--------|----------|
| Login | POST credentials to login endpoint. Server validates against zefs bcrypt hash. On success, generates session token, stores server-side keyed by username (one active token per user). Returns cookie: `ze-session=<token>; Secure; HttpOnly; SameSite=Strict` |
| Subsequent requests | Cookie sent automatically by browser. No Basic Auth header needed after login |
| New login | Invalidates previous session for same username. Old session's page stays visible (read-only ghost). Any action triggers login overlay (dismissible). User can read stale content before re-authenticating |
| Editor ownership | One Editor per user, held by active session. New session takes ownership of same Editor (same change file) |
| JSON API consumers | Still use Basic Auth (stateless). Server accepts either valid cookie OR valid Basic Auth. Cookie checked first |
| Cookie properties | Secure, HttpOnly, SameSite=Strict |
| Login overlay | Dismissible modal. User can read stale page. Any mutation re-shows overlay. Successful login restores session without page reload |
| Rationale | Session-based auth for browsers (no credentials on every request). Basic Auth for programmatic access. Cookie checked first, then Basic Auth fallback |

#### D-6: Runtime Model

| Aspect | Decision |
|--------|----------|
| Model | Embedded component reusing CLI editor infrastructure |
| Engine perspective | Web sessions look like CLI sessions (same IPC, same commands, same draft/commit) |
| Direct access | Web handler calls the same `Editor`, `Completer`, `Schema`, `Tree` code the CLI uses |
| Not a plugin | Does not go through plugin infrastructure. Direct in-process access to editor stack |
| Rationale | Mirrors CLI. Direct access to Editor/Schema/Tree. Engine doesn't know the difference between a web user and a CLI user |

#### D-7: URL Scheme

Three tiers with verb-first paths. Verb is the first segment after the prefix, positionally unambiguous. HTTP method matches semantics (GET=read, POST=mutation, DELETE=deletion).

| Tier | Pattern | Methods |
|------|---------|---------|
| View (read-only, both modes) | `/show/<yang-path>` | GET |
| View (monitoring) | `/monitor/<yang-path>` | GET |
| Config editing | `/config/<verb>/<yang-path>` | Verb-dependent (see below) |
| Admin (operational mutations) | `/admin/<yang-path>` | POST |

| Config verb | HTTP method | Purpose |
|-------------|-------------|---------|
| `edit` | GET | Navigate to edit view for a node |
| `set` | POST | Set a leaf value (form body: leaf + value) |
| `delete` | DELETE | Delete a leaf or sub-container |
| `commit` | POST | Commit draft changes |
| `discard` | POST | Discard draft changes |
| `compare` | GET | View diff of uncommitted changes |

| Example URLs | HTTP Method | Purpose |
|--------------|-------------|---------|
| `/show/bgp/peer/192.168.1.1/` | GET | View peer details (read-only) |
| `/monitor/bgp/peer/192.168.1.1/` | GET | Auto-refreshing peer status |
| `/config/edit/bgp/peer/192.168.1.1/` | GET | Edit view for peer |
| `/config/set/bgp/peer/192.168.1.1/` | POST | Set a leaf value |
| `/config/delete/bgp/peer/192.168.1.1/` | DELETE | Delete a leaf or sub-container |
| `/config/commit` | POST | Commit with diff review |
| `/config/discard` | POST | Discard draft changes |
| `/config/compare` | GET | View uncommitted changes |
| `/admin/peer/192.168.1.1/teardown` | POST | Tear down peer session |
| `/admin/bgp/summary` | POST | Execute BGP summary command |
| `/admin/rib/clear` | POST | Clear RIB |
| `/admin/daemon/shutdown` | POST | Shutdown daemon |

| Rationale | Verb-first is positionally unambiguous. HTTP method matches semantics. No collision between YANG node names and verbs possible. Admin tier covers operational mutations (peer teardown, rib clear, daemon shutdown, future fleet) |

#### D-8: Template Structure

| Aspect | Decision |
|--------|----------|
| Strategy | One Go `html/template` per YANG node kind |
| Embedding | Templates in `internal/component/web/templates/`, embedded via `go:embed` |

| Template | YANG Node Kind | Renders |
|----------|---------------|---------|
| `layout.html` | Page wrapper | HTML head, auth state, HTMX global headers, wraps content area + notification + CLI bar |
| `login.html` | Auth | Login form (username + password fields) |
| `container.html` | ContainerNode | Full-width: leaves as form fields, sub-containers and lists as navigable links |
| `list.html` | ListNode | Left panel: flat key list. Right panel: selected entry detail |
| `leaf_input.html` | LeafNode | Input field typed by ValueType (IP, prefix, uint32, bool toggle, enum dropdown, duration) |
| `flex.html` | FlexNode | Flag/value/block rendering |
| `freeform.html` | FreeformNode | Terminal node, not navigable -- renders as list of entries |
| `inline_list.html` | InlineListNode | Renders like list with key panel |
| `breadcrumb.html` | Shared partial | Back button + path segments as clickable links |
| `notification.html` | Shared partial | Persistent status bar (change count, errors, feedback) |
| `cli_bar.html` | Shared partial | CLI input bar with context prompt and mode toggle |
| `commit.html` | Commit page | Full diff view (all changes, color-coded) with confirm button |
| `command.html` | API command | Titled card: command name in header, output below |

| Rationale | Single concern per template per `rules/file-modularity.md` |

#### D-9: Schema Source

| Aspect | Decision |
|--------|----------|
| Tree walking | `config.Node` types via `schemaGetter` interface (same as CLI editor) |
| Metadata | `goyang.Entry` for descriptions, help text, type information (same as CLI completer) |
| Rationale | Exact same split the CLI uses. `editor_walk.go` uses `config.Node`, `completer.go` uses `goyang.Entry` |

#### D-10: Live Updates

Three distinct behaviors:

| Behavior | Mechanism | Detail |
|----------|-----------|--------|
| Monitor elements | HTMX `hx-trigger="every Ns"` polling | Operational data (peer status, counters) auto-refresh at configurable intervals |
| Config change notifications | SSE push | When another session commits, a non-intrusive banner appears with the reason and a "refresh" button. Does NOT auto-refresh or steal focus |
| Navigation and form actions | Fresh fetch on every action | HTMX naturally fetches fresh HTML on every click/submit. Always current |

| Rationale | Monitor = auto-poll. Config commit = notify-don't-force. Navigation = always fresh |

#### D-11: Content Negotiation

| Aspect | Decision |
|--------|----------|
| Primary | `Accept: text/html` returns HTML (default for browsers and HTMX) |
| JSON | `Accept: application/json` returns JSON |
| Override | `?format=json` query parameter overrides the `Accept` header |
| Conflict resolution | URL parameter wins over `Accept` header |
| Default | HTML when neither header nor parameter is set |

| Consumer | How |
|----------|-----|
| Browser (HTMX) | Gets HTML automatically (default) |
| Curl / scripts | `curl -H "Accept: application/json"` or `curl "...?format=json"` |
| Shared URL | Append `?format=json` to any URL to get JSON -- easy to share |

| Rationale | Standard content negotiation with convenience override. URL wins on conflict |

#### D-12: Draft Model

| Aspect | Decision |
|--------|----------|
| Model | One draft per authenticated user, keyed by username from session token lookup |
| Storage | Per-user change files: `config.conf.change.<username>` (same as CLI) |
| Multiple tabs | Same user in multiple browser tabs sees the same draft (consistent, same session cookie) |
| Multiple users | Different users get independent drafts (no conflicts) |
| Session lookup | Username resolved from session token (cookie) or Basic Auth header (JSON API) |
| Conflict detection | Same mechanism as CLI: scans all `config.conf.change.*` files |
| `who` command | Shows both CLI and web users (web users have `Origin: "web"`) |

| Rationale | Session cookie or Basic Auth provides username on every request. Per-user change files work identically to CLI sessions |

#### D-13: User Management

| Aspect | Decision |
|--------|----------|
| Credential storage | zefs `meta/` namespace (bcrypt hashes), same as CLI/SSH |
| Authentication function | Reuses `AuthenticateUser()` from `internal/component/ssh/auth.go` |
| Validation mode | Login endpoint receives plaintext credentials via POST, server validates against zefs bcrypt hash. JSON API consumers use Basic Auth header |
| Session identity | `EditSession` with `Origin: "web"` and `User` from session token lookup (or Basic Auth for JSON API) |
| User creation | Same mechanism as CLI users (created via `ze init` or config) |

| Rationale | No separate auth system. Same users, same credentials, same audit trail |

#### D-14: Web Server Config and TLS

| Aspect | Decision |
|--------|----------|
| Config schema | YANG `web {}` block in `ze-web-conf.yang` (listen address, port) |
| TLS default | Self-signed certificate auto-generated into zefs `meta/web/` on first run (like SSH key generation in `ze init`) |
| TLS override | `ze web --cert /path/to/cert.pem --key /path/to/key.pem` CLI flags |
| Always TLS | HTTPS only -- no plaintext HTTP option (credentials in every request) |

| Rationale | Zero-config TLS out of the box. YANG-configured like every other component. User replaces cert when ready for production |

#### D-15: Layout -- List Views

| Aspect | Decision |
|--------|----------|
| Left panel | Flat key list (single column of key names), click to select |
| Right panel | Selected entry's children: leaves as form fields, sub-containers as links |
| Draft changes | Inline color coding on modified fields. Hover shows old value |
| No separate compare view | Diff is always visible inline -- modified fields are visually distinct |

Visual model (list view):

| Area | Content |
|------|---------|
| Breadcrumb | `[back] / > bgp > peer` |
| Left panel header | `Peer` |
| Left panel body | `192.168.1.1` (selected, highlighted), `10.0.0.1`, `172.16.0.1`, `[+ Add]` |
| Right panel | Form fields for selected peer: `remote-as [65002]` (colored if modified, hover for old value), `enabled [checkbox]`, `local >` (link), `timer >` (link) |
| Notification | `2 uncommitted changes` |
| CLI bar | `ze[bgp peer 192.168.1.1]# _` with `[CLI mode toggle]` |

#### D-16: Navigation Behavior

| Behavior | Detail |
|----------|--------|
| Back button | Always present at start of breadcrumb. Goes up one level (like CLI `up`) |
| Set action | After setting a value, auto-navigates back one level (ready for next edit) |
| Discard action | After discarding, auto-navigates back one level |
| Notification area | Persistent, always visible. Shows feedback messages (field updated, errors, change count). Like CLI status line |
| Commit | Dedicated page at `/config/compare` (GET for diff view) and `/config/commit` (POST to confirm). Shows full diff of all changes (color-coded). Requires explicit confirmation. Not a modal, not inline |

#### D-17: Container Views

| Aspect | Decision |
|--------|----------|
| Layout | Full-width (no left sidebar). Only list nodes get the split layout |
| Leaves | Rendered as form fields (typed by ValueType) |
| Sub-containers | Rendered as navigable links |
| Sub-lists | Rendered as navigable links |

Visual model (container view):

| Area | Content |
|------|---------|
| Breadcrumb | `[back] / > bgp > peer > 192.168.1.1` |
| Content | `enabled [checkbox]`, `description [text field]`, `remote-as [65002]` (colored if modified), `local` (link), `timer` (link), `capability` (link) |
| Notification | `2 uncommitted changes` |
| CLI bar | `ze[bgp peer 192.168.1.1]# _` with `[CLI mode toggle]` |

#### D-18: Persistent CLI Input Bar

| Aspect | Decision |
|--------|----------|
| Position | Fixed at bottom of page, always visible |
| Input | Accepts CLI grammar (`set`, `delete`, `show`, `edit`, `top`, `up`, etc.) |
| Prompt | Shows current context: `ze[bgp peer 192.168.1.1]# ` (synced with breadcrumb) |
| Autocomplete | Same completions as CLI (YANG-driven via Completer) |
| Context sync | Navigating in GUI updates CLI prompt. Typing `edit timer` in CLI updates breadcrumb |

#### D-19: HTMX Partial Updates

| Action | URL | HTMX target | Swap behavior |
|--------|-----|-------------|---------------|
| Breadcrumb click / back | `GET /show/<yang-path>` | `#content` | Replace content area |
| Click list key | `GET /show/<yang-path>/<key>/` | `#detail` | Replace right panel only |
| Set a value | `POST /config/set/<yang-path>` | `#content` + `#notification` (OOB) | Content refreshes, notification updates |
| Delete a value | `DELETE /config/delete/<yang-path>` | `#content` + `#notification` (OOB) | Content refreshes, notification updates |
| CLI bar command (`edit`) | `GET /config/edit/<yang-path>` | `#content` + `#breadcrumb` (OOB) | Content and breadcrumb both update |
| CLI bar command (`set`) | `POST /config/set/<yang-path>` | `#content` + `#notification` (OOB) | Content refreshes, notification updates |
| Compare page | `GET /config/compare` | `#content` | Full content swap to diff view |
| Commit | `POST /config/commit` | `#content` + `#notification` (OOB) | Content refreshes, notification updates |
| Config change notification (SSE) | `GET /events` | `#notification` (OOB) | Pre-rendered banner appears in notification area |
| Monitor auto-poll | `GET /monitor/<yang-path>` | Element-specific | Each monitor element refreshes itself |
| Toggle CLI mode | - | `#content` | Content area becomes terminal or returns to GUI |

Page shell (breadcrumb + notification bar + CLI bar) is the stable frame. Content area is the primary swap target.

#### D-20: CLI Modes

| Mode | Behavior |
|------|----------|
| **Integrated** (default) | CLI bar at bottom of page. Commands drive the GUI. `edit` changes breadcrumb and content. `set` updates form fields and notification. `show` renders output in content area. GUI and CLI stay in sync |
| **Terminal** | Entire content area becomes a text terminal. Same experience as SSH CLI but over HTTPS. Full scrollback, same text output. Only the notification bar remains from the GUI frame. Toggle button switches back to integrated mode |

| Rationale | Two audiences: GUI users who want CLI shortcuts (integrated), and CLI users who want CLI over HTTPS (terminal) |

#### D-21: Operational Command Rendering

| Aspect | Decision |
|--------|----------|
| Navigation | YANG command tree under `/admin/` renders identically to config tree (breadcrumb + container links) |
| Execution | Commands with parameters render as a form. Submit button executes |
| Results | Titled card: command name in header bar, output rendered below |
| Multiple commands | Result cards stack (most recent on top) |
| Content negotiation | Same as config: `Accept: application/json` or `?format=json` returns JSON |

Visual model (command result):

| Area | Content |
|------|---------|
| Breadcrumb | `[back] / > admin > bgp` |
| Content | Command links: `summary`, `peer`, `route` |
| Result card header | `bgp summary` |
| Result card body | Table: Peer, AS, State, Received, Sent (formatted output) |
| Notification | `ready` |
| CLI bar | `ze> _` |

#### D-22: Editor Concurrency

| Aspect | Decision |
|--------|----------|
| Editor per user | One Editor per user, keyed by username |
| Concurrency | Per-user `sync.Mutex` wrapping all Editor method calls in web handler's `editor.go` |
| New session | New session for same user takes ownership of existing Editor |
| Capacity | Editor map has max size with LRU eviction for idle users |
| Server restart | Detect existing change files in zefs, initialize Editor from them on first authenticated request |

| Rationale | Same user can have multiple tabs or reconnect after session invalidation. Per-user mutex prevents concurrent mutations to the same draft. LRU eviction bounds memory. Restart recovery preserves uncommitted work |

#### D-23: URL Verb Disambiguation

| Aspect | Decision |
|--------|----------|
| Verb position | Positionally fixed as first segment after prefix -- never mixed with schema segments |
| Semantics | HTTP method determines semantics. No collision between YANG node names and verbs possible |
| Read | GET always reads |
| Mutate | POST always mutates |
| Delete | DELETE always deletes |

| Rationale | Positional disambiguation eliminates ambiguity between verbs and YANG path segments. HTTP method alignment is idiomatic REST |

#### D-24: SSE Event Data Rendering

| Aspect | Decision |
|--------|----------|
| Rendering | Server renders notification banner via Go `html/template` (auto-escapes reason text) |
| Broadcast | Pre-rendered HTML broadcast as SSE `data:` payload |
| Client insertion | HTMX inserts via innerHTML safely since content is already escaped |
| Pipeline | Same rendering pipeline as all other pages |

| Rationale | Server-side rendering for SSE payloads avoids client-side template logic and ensures consistent escaping with the rest of the UI |

### Scope

**In Scope:**

| Area | Description |
|------|-------------|
| HTTP server | TLS, embedded in main process, YANG `web {}` config |
| Authentication | Session-based auth for browsers (cookie), Basic Auth for JSON API consumers, custom login page with dismissible overlay |
| YANG-to-HTML | Template rendering for all node kinds (container, list, leaf, flex) |
| Config navigation | Breadcrumb, container view, list view with key panel |
| Config editing | Set, delete, draft per user, inline diff, commit page with confirmation |
| Admin commands | Operational command tree under `/admin/`, execution, result cards |
| Content negotiation | HTML (primary) + JSON, via Accept header or `?format=json` |
| CLI integration | Persistent CLI bar, integrated mode, terminal mode |
| Live updates | SSE config change notification, HTMX auto-poll for monitors |
| Per-user drafts | Same change file mechanism as CLI |
| Self-signed TLS | Auto-generated on init, CLI override for user certs |

**Out of Scope:**

| Area | Reason |
|------|--------|
| WebSocket | SSE is sufficient for server-to-client push. HTMX handles client-to-server |
| Role-based access control | All authenticated users have full access. RBAC is a future feature |
| Multi-language / i18n | English only |
| Mobile-specific layout | Desktop-first. Responsive CSS is a refinement, not a requirement |
| Config file upload/download | Can be added later. Not core to the YANG-driven navigation model |
| Plugin web extensions | Plugins cannot contribute custom web pages. Future feature |

### Child Specs

| Phase | Spec | Scope | Depends |
|-------|------|-------|---------|
| 1 | `spec-web-1-foundation.md` | HTTP server, TLS (self-signed + override), YANG `web {}` config, `ze web` CLI command, session-based auth with cookie (browsers) + Basic Auth fallback (JSON API), custom login page with dismissible overlay, content negotiation, page layout frame (breadcrumb area + content area + notification area + CLI bar), embedded assets (HTMX, CSS) | - |
| 2 | `spec-web-2-config-view.md` | YANG-to-HTML templates for all node kinds, schema walking via `schemaGetter`, container full-width view, list view with key panel, leaf input fields typed by ValueType, breadcrumb navigation, back button. Read-only config display (no editing yet) | web-1 |
| 3 | `spec-web-3-config-edit.md` | Set and delete via verb-first URLs (`/config/set/...`, `/config/delete/...`), per-user draft via `EditSession` (origin "web"), inline diff (color + hover for old value), commit page with full diff and confirmation, discard, set/discard auto-navigate back one level, conflict detection, per-user Editor concurrency (sync.Mutex) | web-2 |
| 4 | `spec-web-4-api-commands.md` | Admin YANG command tree navigation under `/admin/`, command parameter forms, command execution, titled result cards, card stacking | web-2 |
| 5 | `spec-web-5-cli-modes.md` | Persistent CLI input bar with autocomplete (reuses Completer), integrated mode (commands drive GUI, context sync), terminal mode (full text CLI over HTTPS in content area), mode toggle | web-2 |
| 6 | `spec-web-6-live-updates.md` | SSE endpoint for config change notifications (commit by other users), notification banner with reason + refresh button (non-intrusive), HTMX auto-poll for monitor elements | web-3 |

Phases 2-5 depend on foundation (phase 1). Phases 4 and 5 can proceed in parallel with phase 3 since they all depend on phase 2 (config view) but not on each other. Phase 6 depends on phase 3 (needs the editing infrastructure to detect commits).

### Dependency Graph

| Spec | After |
|------|-------|
| web-1 (foundation) | - |
| web-2 (config view) | web-1 |
| web-3 (config edit) | web-2 |
| web-4 (api commands) | web-2 |
| web-5 (cli modes) | web-2 |
| web-6 (live updates) | web-3 |

### Key Integration Points

| Component | How the Web UI Integrates |
|-----------|--------------------------|
| `config.Schema` / `config.Node` | Template rendering walks schema via `schemaGetter` to determine what to show |
| `config.Tree` | Handler reads Tree at the URL path to get current values for form fields |
| `cli.Editor` | Web handler creates/reuses Editor per user for draft/commit/discard |
| `cli.Completer` | CLI bar autocomplete calls Completer with current context path |
| `cli.EditSession` | Web sessions use `Origin: "web"`, same per-user change files |
| `ssh.AuthenticateUser` | Auth middleware calls same function SSH uses, against same zefs credentials |
| `config/yang.Loader` | Templates pull descriptions and help text from loaded YANG entries |
| `config/storage.Storage` | Editor reads/writes config through Storage (zefs-backed) |
| `confModules` | Needs updating to include `ze-web-conf` YANG module for web server config parsing |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` -- overall architecture, reactor, plugin model
  -> Constraint: web UI is a component, not a plugin
- [ ] `docs/architecture/zefs-format.md` -- zefs storage format, credential storage
  -> Constraint: credentials stored as bcrypt hashes in meta/ namespace
- [ ] `docs/architecture/chaos-web-dashboard.md` -- existing HTMX/SSE patterns
  -> Decision: reuse HTMX asset embedding pattern, SSE broker pattern

### Source Files
- [ ] `internal/component/config/schema.go` -- Node types, schemaGetter interface
  -> Constraint: Node.Kind() determines template selection
- [ ] `internal/component/config/tree.go` -- Runtime config tree
  -> Constraint: Tree.GetList(), Tree.GetContainer(), Tree.Get() for data access
- [ ] `internal/component/cli/editor.go` -- Editor struct, commands
  -> Decision: web handler creates Editor per user, reuses all methods
- [ ] `internal/component/cli/editor_walk.go` -- walkPath, walkOrCreateIn
  -> Constraint: list keys consume 2 path segments (name + key)
- [ ] `internal/component/cli/editor_session.go` -- EditSession, per-user change files
  -> Decision: web sessions use Origin "web", same change file pattern
- [ ] `internal/component/cli/completer.go` -- YANG-driven completions
  -> Decision: CLI bar reuses Completer for autocomplete
- [ ] `internal/component/ssh/auth.go` -- AuthenticateUser, CheckPassword
  -> Decision: web auth calls same function
- [ ] `internal/component/ssh/ssh.go` -- SSH server setup pattern
  -> Constraint: follow same TLS and listener patterns
- [ ] `internal/chaos/web/dashboard.go` -- existing web dashboard setup
  -> Decision: follow route registration and asset embedding patterns
- [ ] `internal/chaos/web/render.go` -- existing HTML rendering
  -> Decision: REPLACE inline string rendering with html/template for web UI

**Key insights:**
- CLI contextPath is `[]string`, maps directly to URL path segments
- schemaGetter interface enables polymorphic tree walking (Container, List, Flex all satisfy it)
- ListNode navigation consumes 2 path elements (list name + key value)
- Per-user change files keyed by sanitized username
- Chaos dashboard proves HTMX + SSE + go:embed pattern works
- Editor has full draft/commit/discard/compare semantics ready to reuse

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/config/schema.go` -- defines Node, ContainerNode, ListNode, LeafNode, FlexNode, InlineListNode, ValueType
- [ ] `internal/component/config/tree.go` -- Tree with values, containers, lists, ordered access
- [ ] `internal/component/cli/editor.go` -- Editor with schema walking, set/delete/commit/discard
- [ ] `internal/component/cli/editor_walk.go` -- walkPath, walkOrCreateIn, schemaGetter
- [ ] `internal/component/cli/editor_session.go` -- EditSession with User, Origin, ID
- [ ] `internal/component/cli/completer.go` -- Completer with Complete(), GhostText()
- [ ] `internal/component/ssh/auth.go` -- AuthenticateUser(), CheckPassword(), UserConfig
- [ ] `internal/chaos/web/dashboard.go` -- Dashboard, Config, registerRoutes
- [ ] `internal/chaos/web/render.go` -- htmlWriter inline rendering pattern

**Behavior to preserve:**
- CLI editor semantics (draft/commit/discard/compare) unchanged
- Per-user change file naming and conflict detection unchanged
- zefs credential storage format unchanged
- AuthenticateUser dual-mode (hash-as-token + plaintext) unchanged
- YANG schema loading and registration unchanged
- Config tree structure and access patterns unchanged

**Behavior to change:**
- None -- all existing behavior preserved. Web UI adds a new access method to existing infrastructure

## Data Flow (MANDATORY)

### Entry Points

| Entry | Format | Web UI impact |
|-------|--------|---------------|
| Browser GET `/show/...` or `/monitor/...` | HTTP URL with verb-first path | Read-only view or auto-refreshing monitor of YANG path |
| Browser GET `/config/edit/...` or `/config/compare` | HTTP URL with config verb | Navigate to edit view or view uncommitted diff |
| Browser POST `/config/set/...` or DELETE `/config/delete/...` | HTTP URL with config verb + form body | Mutation dispatched to Editor (`SetValue()`, `DeleteValue()`, `DeleteContainer()`) |
| Browser POST `/config/commit` or `/config/discard` | HTTP URL with config verb | Commit or discard draft via Editor (`CommitSession()`, `Discard()`) |
| Browser POST `/admin/...` | HTTP URL with admin path | Operational mutation (peer teardown, rib clear, command execution) |
| Browser POST `/login` | Credentials in form body | Session token generated, returned as `ze-session` cookie |
| SSE connection | HTTP GET `/events` | Long-lived connection for config change notifications |
| JSON API request | HTTP with Accept: application/json or ?format=json | Same handlers, JSON response instead of HTML |

### Transformation Path

1. HTTP request received by Go `net/http` handler
2. Auth middleware checks for valid `ze-session` cookie first, falls back to Basic Auth header. Either must validate against zefs bcrypt hash via `AuthenticateUser()`
3. Tier determined from URL prefix: `/show/` and `/monitor/` (view), `/config/<verb>/` (config), `/admin/` (admin)
4. Verb extracted from first segment after prefix (positionally fixed)
5. Remaining path segments split into `[]string` (same as CLI `contextPath`)
6. Path validated against YANG schema via `schemaGetter.Get()` / `walkPath()`
7. For GET (show/monitor/edit/compare): schema node kind determines template selection, Tree provides data
8. For POST/DELETE (set/delete/commit/discard/admin): verb dispatches to Editor or command handler
9. Content negotiation: `?format=json` or `Accept` header determines response format
10. Response: HTML template rendered or JSON marshaled

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| Browser <-> Web server | HTTPS + session cookie (browsers) or Basic Auth (JSON API) + HTMX | [ ] |
| Web handler <-> Editor | Direct in-process function call (same as CLI). Per-user `sync.Mutex` serializes access | [ ] |
| Web handler <-> Schema | Direct in-process via schemaGetter interface | [ ] |
| Web handler <-> Tree | Direct in-process via Tree accessor methods | [ ] |
| Web handler <-> Completer | Direct in-process for CLI bar autocomplete | [ ] |
| Web handler <-> zefs | Via Storage interface for credential validation and config persistence | [ ] |
| SSE broker <-> Browser | Server-Sent Events over HTTPS. Pre-rendered HTML payloads via `html/template` | [ ] |

### Integration Points
- `Editor` -- web handler creates/reuses per user, calls `SetValue()`, `DeleteValue()`, `DeleteContainer()`, `CommitSession()`, `Discard()`. Per-user `sync.Mutex` in web handler's `editor.go`
- `Completer` -- CLI bar autocomplete calls Complete() with URL-derived context path
- `AuthenticateUser()` -- auth middleware validates session cookie or Basic Auth header
- `EditSession` -- web sessions created with Origin "web"
- `config.Schema` -- template selection based on Node.Kind()
- `config.Tree` -- data access for form field values
- `yang.Loader` -- YANG entry metadata for descriptions, help text

### Architectural Verification
- [ ] No bypassed layers (web handler uses Editor, not raw Tree modification)
- [ ] No unintended coupling (web component depends on cli/editor, config/schema, ssh/auth -- all stable interfaces)
- [ ] No duplicated functionality (reuses Editor, Completer, AuthenticateUser -- does not reimplement)
- [ ] Zero-copy preserved (web layer adds HTML rendering on top, does not change underlying data access)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `ze web` CLI command | -> | HTTP server starts, serves login page | `test/plugin/web-startup.ci` |
| Browser GET `/show/bgp/` with session cookie | -> | Auth validates, schema walked, container template rendered | `test/plugin/web-config-view.ci` |
| Browser POST `/config/set/bgp/peer/192.168.1.1/` | -> | Editor.SetValue() called, change file written | `test/plugin/web-config-edit.ci` |
| Browser POST `/config/commit` | -> | Editor.CommitSession() called, config persisted | `test/plugin/web-config-commit.ci` |
| Browser POST `/admin/bgp/summary` | -> | Command dispatched, result card rendered | `test/plugin/web-api-command.ci` |
| Browser GET `/show/bgp/?format=json` | -> | JSON response with config data | `test/plugin/web-json-response.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `ze web` started with config containing `web { listen 0.0.0.0:8080; }` | HTTPS server listens on port 8080 with self-signed TLS cert |
| AC-2 | Browser navigates to `/` without credentials | 401 response with custom login page HTML |
| AC-3 | Browser sends valid session cookie to `/show/` | Root config view rendered with all top-level YANG containers |
| AC-4 | Browser sends invalid credentials | 401 response with login page |
| AC-5 | Navigate to `/show/bgp/peer/` (a list node) | Left panel shows peer key names, right panel empty or instructions |
| AC-6 | Click a peer key in the left panel | Right panel shows peer's leaves as form fields, containers as links |
| AC-7 | Navigate to `/show/bgp/` (a container node) | Full-width view with leaves as fields, sub-containers/lists as links |
| AC-8 | POST to `/config/set/bgp/peer/192.168.1.1/` with leaf=remote-as, value=65002 | Value set in user's draft, auto-navigate back one level |
| AC-9 | POST to `/config/discard` | User's draft discarded via Editor.Discard(), navigate back one level |
| AC-10 | GET `/config/compare` | Diff page showing all uncommitted changes, color-coded |
| AC-11 | POST `/config/commit` after review | Config committed, notification "committed" |
| AC-12 | Modified field in container view | Field visually distinct (color), hover shows old value |
| AC-13 | GET `/show/bgp/?format=json` | JSON response with config data, not HTML |
| AC-14 | GET `/show/bgp/` with `Accept: application/json` | JSON response |
| AC-15 | Both `Accept: text/html` and `?format=json` present | JSON returned (URL wins) |
| AC-16 | Two users edit simultaneously | Independent drafts, `who` shows both |
| AC-17 | User A commits while User B is viewing | User B sees notification banner with reason and "refresh" button |
| AC-18 | Breadcrumb back button at `/show/bgp/peer/192.168.1.1/` | Navigates to `/show/bgp/peer/` |
| AC-19 | CLI bar at bottom with `edit timer` typed | Content and breadcrumb update to timer context |
| AC-20 | Toggle to terminal CLI mode | Content area becomes full text CLI, same as SSH experience |
| AC-21 | POST `/admin/bgp/summary` | Titled card with "bgp summary" header and tabular output below |
| AC-22 | `ze web --cert /path --key /path` | User-provided TLS cert used instead of self-signed |
| AC-23 | Notification area always visible | Shows change count, feedback messages, errors at all times |
| AC-24 | URL path contains `..`, null bytes, or non-YANG characters | 400 Bad Request response. Path traversal rejected before schema walking |
| AC-25 | TLS certificate file missing or unreadable at startup | Server exits with exit code 1 and error message to stderr |
| AC-26 | Two users POST `/config/commit` simultaneously with conflicting changes | First commit succeeds. Second re-renders diff page with error explaining conflict |
| AC-27 | Login returns `Set-Cookie` header | Cookie has `ze-session=<token>; Secure; HttpOnly; SameSite=Strict` properties |
| AC-28 | Session invalidated (new login from same user) | Old session's page stays visible (read-only). Any action shows dismissible login overlay. Stale content remains readable. Successful re-login restores session without page reload |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestAuthMiddleware` | `internal/component/web/auth_test.go` | Session cookie and Basic Auth validation, 401 on missing/invalid credentials, login overlay on expired session | Detailed in web-1 |
| `TestURLToPath` | `internal/component/web/handler_test.go` | URL path splitting into contextPath | Detailed in web-1 |
| `TestContentNegotiation` | `internal/component/web/handler_test.go` | Accept header vs ?format=json, URL wins on conflict | Detailed in web-1 |
| `TestNodeKindTemplate` | `internal/component/web/render_test.go` | Correct template selected for each Node kind | Detailed in web-2 |
| `TestContainerRendering` | `internal/component/web/render_test.go` | Container renders leaves as fields, sub-containers as links | Detailed in web-2 |
| `TestListRendering` | `internal/component/web/render_test.go` | List renders key panel (left) and detail panel (right) | Detailed in web-2 |
| `TestLeafInputTypes` | `internal/component/web/render_test.go` | ValueType maps to correct HTML input type | Detailed in web-2 |
| `TestBreadcrumbFromPath` | `internal/component/web/render_test.go` | URL path renders as clickable breadcrumb with back button | Detailed in web-2 |
| `TestSetVerb` | `internal/component/web/handler_test.go` | POST /set dispatches to Editor set command | Detailed in web-3 |
| `TestDeleteVerb` | `internal/component/web/handler_test.go` | POST /delete dispatches to Editor delete command | Detailed in web-3 |
| `TestDraftPerUser` | `internal/component/web/handler_test.go` | Different usernames get independent drafts | Detailed in web-3 |
| `TestInlineDiff` | `internal/component/web/render_test.go` | Modified fields have diff class, hover data attribute for old value | Detailed in web-3 |
| `TestCommandExecution` | `internal/component/web/handler_test.go` | POST /admin/ dispatches command and returns result card | Detailed in web-4 |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `web-startup` | `test/plugin/web-startup.ci` | `ze web` starts, responds to HTTPS request | Detailed in web-1 |
| `web-config-view` | `test/plugin/web-config-view.ci` | Browser navigates config tree, sees correct HTML | Detailed in web-2 |
| `web-config-edit` | `test/plugin/web-config-edit.ci` | User sets a value via POST, change persisted in draft | Detailed in web-3 |
| `web-config-commit` | `test/plugin/web-config-commit.ci` | User commits, config applied | Detailed in web-3 |
| `web-json-response` | `test/plugin/web-json-response.ci` | ?format=json returns JSON config data | Detailed in web-1 |
| `web-api-command` | `test/plugin/web-api-command.ci` | Execute operational command, get result card | Detailed in web-4 |

## Files to Modify

- `cmd/ze/main.go` -- add `web` subcommand dispatch
- `internal/component/ssh/auth.go` -- possibly export AuthenticateUser for web use (verify accessibility)
- `internal/component/cli/editor_session.go` -- verify "web" is a valid Origin value

## Files to Create

- `cmd/ze/web/main.go` -- `ze web` CLI entry point
- `internal/component/web/server.go` -- HTTP server setup, TLS, route registration
- `internal/component/web/auth.go` -- Session cookie + Basic Auth middleware, login endpoint, session store
- `internal/component/web/editor.go` -- Per-user Editor map with sync.Mutex, LRU eviction, restart recovery from existing change files
- `internal/component/web/handler.go` -- URL path parsing, schema walking, content negotiation dispatch
- `internal/component/web/handler_config.go` -- config tree GET/POST handlers
- `internal/component/web/handler_api.go` -- API command handlers
- `internal/component/web/render.go` -- template loading, node-kind dispatch, template data preparation
- `internal/component/web/sse.go` -- SSE broker for config change notifications
- `internal/component/web/cli.go` -- CLI bar endpoint, autocomplete, integrated/terminal mode
- `internal/component/web/schema/register.go` -- YANG module registration for `ze-web-conf.yang`
- `internal/component/web/schema/ze-web-conf.yang` -- web server config schema
- `internal/component/web/templates/layout.html` -- page frame template
- `internal/component/web/templates/login.html` -- login page template
- `internal/component/web/templates/container.html` -- container node template
- `internal/component/web/templates/list.html` -- list node template (split layout)
- `internal/component/web/templates/leaf_input.html` -- typed input field partial
- `internal/component/web/templates/flex.html` -- flex node template
- `internal/component/web/templates/freeform.html` -- freeform node template (terminal node, list of entries)
- `internal/component/web/templates/inline_list.html` -- inline list node template (like list with key panel)
- `internal/component/web/templates/breadcrumb.html` -- breadcrumb partial
- `internal/component/web/templates/notification.html` -- notification bar partial
- `internal/component/web/templates/cli_bar.html` -- CLI input bar partial
- `internal/component/web/templates/commit.html` -- commit diff page template
- `internal/component/web/templates/command.html` -- command result card template
- `internal/component/web/assets/htmx.min.js` -- HTMX library (vendored)
- `internal/component/web/assets/sse.js` -- SSE extension for HTMX
- `internal/component/web/assets/style.css` -- web UI stylesheet
- `test/plugin/web-startup.ci` -- functional test: server starts
- `test/plugin/web-config-view.ci` -- functional test: config navigation
- `test/plugin/web-config-edit.ci` -- functional test: set/delete
- `test/plugin/web-config-commit.ci` -- functional test: commit
- `test/plugin/web-json-response.ci` -- functional test: JSON content negotiation
- `test/plugin/web-api-command.ci` -- functional test: operational command execution

### Integration Checklist

| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (web config) | Yes | `internal/component/web/schema/ze-web-conf.yang` |
| CLI commands/flags | Yes | `cmd/ze/web/main.go` |
| Editor autocomplete | Yes | YANG-driven (automatic if YANG updated) |
| Functional test for web features | Yes | `test/plugin/web-*.ci` |

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` -- add web interface |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md` -- add `web {}` block |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` -- add `ze web` |
| 4 | API/RPC added/changed? | No | N/A |
| 5 | Plugin added/changed? | No | N/A (web is a component, not a plugin) |
| 6 | Has a user guide page? | Yes | `docs/guide/web-interface.md` -- new guide page |
| 7 | Wire format changed? | No | N/A |
| 8 | Plugin SDK/protocol changed? | No | N/A |
| 9 | RFC behavior implemented? | No | N/A |
| 10 | Test infrastructure changed? | No | N/A |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` -- web UI is a differentiator |
| 12 | Internal architecture changed? | Yes | `docs/architecture/web-interface.md` -- new architecture doc |

## Implementation Steps

### Implementation Phases

Each phase corresponds to a child spec. Phases are ordered by dependency.

1. **Phase: Foundation (web-1)** -- HTTP server, TLS, auth, content negotiation, page layout frame, `ze web` CLI
   - Tests: `TestAuthMiddleware`, `TestURLToPath`, `TestContentNegotiation`, `web-startup.ci`, `web-json-response.ci`
   - Files: `cmd/ze/web/`, `internal/component/web/server.go`, `auth.go`, `handler.go`, `render.go`, templates (`layout.html`, `login.html`), assets, YANG schema
   - Verify: `ze web` starts, login page served, auth works, content negotiation works

2. **Phase: Config View (web-2)** -- YANG-to-HTML templates, schema walking, container/list/leaf rendering, breadcrumb
   - Tests: `TestNodeKindTemplate`, `TestContainerRendering`, `TestListRendering`, `TestLeafInputTypes`, `TestBreadcrumbFromPath`, `web-config-view.ci`
   - Files: templates (`container.html`, `list.html`, `leaf_input.html`, `flex.html`, `breadcrumb.html`), `handler_config.go`
   - Verify: navigate config tree in browser, see correct rendering per node kind

3. **Phase: Config Edit (web-3)** -- Set/delete, per-user drafts, inline diff, commit page
   - Tests: `TestSetVerb`, `TestDeleteVerb`, `TestDraftPerUser`, `TestInlineDiff`, `web-config-edit.ci`, `web-config-commit.ci`
   - Files: `handler_config.go` (POST handlers), `render.go` (diff rendering), templates (`commit.html`, `notification.html`)
   - Verify: set a value, see inline diff, commit with confirmation

4. **Phase: API Commands (web-4)** -- Command tree navigation, execution, result cards
   - Tests: `TestCommandExecution`, `web-api-command.ci`
   - Files: `handler_api.go`, templates (`command.html`)
   - Verify: execute operational command, see titled result card

5. **Phase: CLI Modes (web-5)** -- CLI bar, integrated mode, terminal mode
   - Tests: CLI bar autocomplete, mode toggle
   - Files: `cli.go`, templates (`cli_bar.html`)
   - Verify: type CLI command in bar, see GUI update; toggle to terminal mode

6. **Phase: Live Updates (web-6)** -- SSE notifications, auto-poll for monitors
   - Tests: SSE notification on commit, auto-poll refresh
   - Files: `sse.go`, templates (`notification.html` update)
   - Verify: commit from CLI, web session sees notification banner

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation in a child spec |
| Correctness | Auth validates session cookie or Basic Auth on every request, no bypass paths |
| Naming | URL paths use kebab-case consistent with YANG. JSON keys use kebab-case per `rules/json-format.md` |
| Data flow | All mutations go through Editor, never raw Tree writes |
| Rule: no-layering | No duplicate auth system -- reuses ssh/auth |
| Rule: plugin-design | Web is a component not a plugin -- verify no registry.Register() call |
| Security | TLS always on, session cookie or Basic Auth validated every request, session tokens stored server-side with one-per-user limit |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| `ze web` command starts HTTPS server | `ze web` + curl to verify response |
| Login page on 401 | curl without auth, verify HTML login form |
| Config tree navigation | curl with session cookie to `/show/bgp/`, verify HTML |
| Set/delete/commit | curl POST to set endpoint, verify change file |
| JSON response | curl with `?format=json`, verify JSON |
| CLI bar functional | Browser test with CLI command |
| SSE notifications | Commit from CLI while web open, verify banner |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Input validation | URL path segments validated against YANG schema (no arbitrary path traversal) |
| Auth bypass | Every handler goes through auth middleware, no exceptions except login page |
| Credential handling | Passwords never logged, never stored in server memory beyond request scope. Session tokens stored server-side, keyed by username |
| TLS enforcement | No plaintext HTTP listener, redirect or refuse |
| XSS prevention | `html/template` auto-escapes by default. Verify no `template.HTML` bypasses. SSE payloads pre-rendered server-side with same escaping |
| CSRF | Session cookie has SameSite=Strict. POST requests require valid session cookie or Basic Auth |
| Session security | Cookie: Secure, HttpOnly, SameSite=Strict. One active session per user. New login invalidates previous |
| Path traversal | URL path validated against schema before any file/tree access |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Auth bypass found | Security fix, block release |
| Template rendering wrong | Check schema walking, verify node kind detection |
| Content negotiation wrong | Check ?format=json priority over Accept header |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user |

## Mistake Log

### Wrong Assumptions

| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches

| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates

| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

- The CLI `contextPath []string` maps perfectly to URL path segments. No translation layer needed.
- `schemaGetter` interface already supports polymorphic tree walking -- Container, List, Flex, InlineList all implement `Get(name) Node` and `Children() []string`. The web handler can walk any schema node identically.
- The Editor already supports multiple concurrent users via per-user change files. Web sessions are just another origin.
- The chaos dashboard proves the HTMX + SSE + `go:embed` stack works in Ze. The web UI follows the same infrastructure patterns but uses `html/template` instead of inline strings.
- Content negotiation is a thin layer: check `?format=json` first (URL wins), then `Accept` header, default to HTML. The same handler function prepares the data, then either renders a template or marshals JSON.
- Editor actual method names: `SetValue()`, `DeleteValue()`, `DeleteContainer()`, `CommitSession()`, `Discard()` -- not the shortened forms `Set()`, `Delete()`, `Commit()`. Web handlers must call the real method names.

## Implementation Summary

### What Was Implemented
- Umbrella spec only -- implementation in child specs

### Documentation Updates
- None yet

### Deviations from Plan
- None

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Web interface from YANG | | Child specs web-1 through web-6 | Umbrella defines scope |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 through AC-28 | | Child specs | Distributed across phases |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| All tests | | Child specs | Distributed across phases |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| All files | | Created in child specs |

### Audit Summary
- **Total items:** 28 ACs, 13 unit tests, 6 functional tests, 30+ files
- **Done:** 0 (umbrella spec -- implementation in child specs)
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 0

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| Umbrella only -- child specs create files | | |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| Umbrella only -- child specs verify ACs | | |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| Umbrella only -- child specs verify wiring | | |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-23 all demonstrated
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes (all 6 checks in `rules/quality.md` -- no failures)

### Quality Gates (SHOULD pass -- defer with user approval)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (3+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-web-0-umbrella.md`
- [ ] Summary included in commit
