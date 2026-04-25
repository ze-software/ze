# Web Interface

Ze provides an HTTPS web interface for browsing configuration, editing values, and running commands through a browser. The web UI uses the same YANG schemas, user database, and command grammar as the SSH CLI.
<!-- source: internal/component/web/handler.go -- URL routing, three-tier scheme -->
<!-- source: internal/component/web/auth.go -- AuthMiddleware uses ssh.AuthenticateUser -->

## Starting the Web Server

### Command Line

```bash
ze start --web 8443                              # Start daemon + web on port 8443
ze start --web 8443 --insecure-web               # No authentication (forces 127.0.0.1)
```

When no certificate is configured, ze generates an ECDSA P-256 self-signed certificate automatically. The certificate includes SANs for localhost, 127.0.0.1, ::1, and the listen address.
<!-- source: cmd/ze/main.go -- cmdStart, webPort/insecureWeb flags -->
<!-- source: internal/component/web/server.go -- GenerateWebCertWithAddr -->

| Flag | Description |
|------|-------------|
| `--web <port>` | Start web interface on `0.0.0.0:<port>` |
| `--insecure-web` | Disable authentication (forces `127.0.0.1`, requires `--web`) |

### V2 workbench (experiment)

Ze ships an experimental RouterOS-style workbench UI alongside the established Finder UI. It is opt-in via the `ze.web.ui` env var:

```bash
ZE_WEB_UI=workbench ze start --web 8443
```

The workbench keeps the same authentication, commit flow, and CLI bar; only the chrome and table contract differ. BGP peer rows ship with related operator tools (peer detail, capabilities, statistics, flush, teardown) that run the same dispatched commands as the SSH CLI. Confirmation prompts gate destructive tools.

To return to the default Finder UI:

```bash
ZE_WEB_UI=finder ze start --web 8443     # explicit rollback
ze start --web 8443                       # also Finder while the default has not flipped
```

The workbench is gated on Promotion Criteria browser tests. Until those pass, the default stays at `finder`. See `plan/spec-web-2-operator-workbench.md` for the experiment status.

### Configuration

The web server listen address can also be set in the ze configuration file:

```
environment {
    web {
        enabled true;
        server main {
            ip 0.0.0.0;
            port 8443;
        }
    }
}
```
<!-- source: internal/component/web/schema/ze-web-conf.yang -- web container, server list -->

## Authentication

The web interface uses the same user database as the SSH server. Users log in through a browser login page or authenticate API requests with HTTP Basic Auth.
<!-- source: internal/component/web/auth.go -- AuthMiddleware, LoginHandler, parseBasicAuth -->

### Browser Sessions

1. Navigate to `https://<host>:8443/`. Unauthenticated requests receive a login page.
2. Enter username and password. On success, a `ze-session` cookie is set.
3. The cookie is `Secure`, `HttpOnly`, and `SameSite=Strict`.
4. Each user can have one active session. Logging in again invalidates the previous session.

### JSON API

API clients that send `Accept: application/json` (or append `?format=json` to the URL) can authenticate with HTTP Basic Auth instead of session cookies. No session is created for Basic Auth requests.

```bash
curl -k -u admin:password https://localhost:8443/show/bgp/?format=json
```

### Security Headers

All authenticated responses include security headers: HSTS (`max-age=63072000`), Content-Security-Policy (`default-src 'self'`), X-Frame-Options `DENY`, X-Content-Type-Options `nosniff`, and `Cache-Control: no-store`.
<!-- source: internal/component/web/auth.go -- addSecurityHeaders -->

## Navigation

### URL Scheme

URLs follow a verb-first three-tier pattern:
<!-- source: internal/component/web/handler.go -- ParseURL, knownPrefixes, configVerbs -->

| Tier | URL Pattern | Method | Description |
|------|-------------|--------|-------------|
| View | `/show/<yang-path>` | GET | Read-only config tree view |
| View | `/monitor/<yang-path>` | GET | View with auto-polling |
| Config | `/config/edit/<path>` | GET | Editable config tree view |
| Config | `/config/set/<path>` | POST | Set a leaf value |
| Config | `/config/add/<path>` | POST | Create a list entry (with optional field values) |
| Config | `/config/add-form/<path>` | GET | Fetch add-entry overlay form |
| Config | `/config/rename/<path>` | POST | Rename a keyed list entry |
| Config | `/config/delete/<path>` | POST | Delete a leaf value |
| Config | `/config/commit/` | GET/POST | View diff and commit changes |
| Config | `/config/discard/` | POST | Discard pending changes |
| Config | `/config/changes` | GET | Commit bar state (pending change count) |
| Config | `/config/compare/` | GET | Compare pending vs committed |
| Admin | `/admin/<yang-path>` | GET/POST | Administrative commands |
| Auth | `/login` | POST | Login (no auth required) |
| Static | `/assets/` | GET | CSS, JS, images (no auth required) |

The root URL `/` redirects to `/show/`.

### Finder Navigation

The left panel uses a Finder-style column browser (similar to macOS Finder). It shows up to 3 columns, scrolling horizontally as you navigate deeper.
<!-- source: internal/component/web/fragment.go -- buildFinderColumns, buildColumnAt -->

**Named vs unnamed containers:** Named containers (lists with YANG keys, like `peer`, `group`) appear above unnamed containers (global settings like `local`, `timer`), separated by a horizontal rule. This makes keyed sections easy to find.

**Simple lists:** Lists without unique constraints show as a flat column of clickable entries with a `+ new` button.

### Context Heading

When inside a list entry, the detail panel shows a context heading at the top with the list name and entry key (e.g., `PEER london`). This provides immediate context without checking the breadcrumb.
<!-- source: internal/component/web/fragment.go -- buildContextHeading, ContextEntry -->

### List Table View

Lists that have YANG `unique` constraints (e.g., `peer` with `unique "remote/ip"`) display as an interactive table in the detail panel. The table shows the list key and all unique fields as columns.
<!-- source: internal/component/web/fragment.go -- buildListTable, collectUniqueFields -->
<!-- source: internal/component/web/templates/component/list_table.html -->

| Column | Behavior |
|--------|----------|
| Rename button | Opens a modal, normalizes the new key, and renames the entry without losing its subtree |
| Key column (e.g., name) | Clickable link, navigates into the entry's config subtree |
| Unique field columns (e.g., remote/ip) | Editable inline, saves on blur/Enter/auto-save (1s debounce) |
| Delete button | Removes the entry after confirmation |

The `+ new` button below the table opens a server-rendered form (via HTMX) with inputs for the entry name and all unique fields. Field values are validated against YANG types before the entry is created.
<!-- source: internal/component/web/handler_config.go -- HandleConfigAdd, HandleConfigAddForm -->
<!-- source: internal/component/web/templates/component/add_form_overlay.html -->

### Breadcrumb Navigation

Every page displays a breadcrumb trail from root to the current YANG path. Clicking any breadcrumb segment navigates to that level.
<!-- source: internal/component/web/handler_config.go -- buildBreadcrumbs -->

### Content Negotiation

The response format is determined by:

1. `?format=json` query parameter (takes precedence)
2. `Accept: application/json` header (when `text/html` is not also present)
3. Default: HTML

## Config Editing

Each authenticated user gets an independent editor session with its own working tree. Changes are tracked per-user and do not affect other users until committed.
<!-- source: internal/component/web/editor.go -- EditorManager, userSession -->

### Workflow

1. **Browse:** Navigate to a list (e.g., `/show/bgp/peer/`) to see entries in a table.
2. **Add:** Click `+ new` to create an entry. Fill in the name and unique fields. Values are validated against YANG types (e.g., IP addresses must be valid).
3. **Rename:** In table views, click the rename button to change an entry key. The new key is trimmed and lowercased, and the existing subtree stays attached to the renamed entry.
4. **Edit:** Click an entry name to see its full config. Edit leaf values through inline fields.
5. **Review:** The commit bar at the bottom shows pending change count. Click "Review & Commit" to see a diff.
6. **Commit:** Apply changes. Conflicts with other users are detected and reported.
7. **Discard:** Click "Discard" to abandon all pending changes.

### Validation

Field values are validated server-side against YANG types before being accepted:
<!-- source: internal/component/config/schema.go -- ValidateValue -->

| Type | Validation |
|------|-----------|
| IP address | Must be a valid IPv4 or IPv6 address |
| IPv4 | Must be a valid IPv4 address |
| IPv6 | Must be a valid IPv6 address |
| Prefix | Must be a valid CIDR prefix |
| Uint16/Uint32 | Must be a valid unsigned integer in range |
| Boolean | Normalized to `true`/`false` |

YANG `unique` constraints are enforced: duplicate values are rejected with an error naming the conflicting entry.
<!-- source: internal/component/web/handler_config_walk.go -- checkUniqueConstraint, validateUniqueOnSet -->

Entry key names are automatically lowercased and trimmed for both add and rename operations.

Duplicate entry keys are rejected. Validation runs before the entry is created, so invalid input never produces a partial entry.

Navigating to a non-existent list entry (e.g., `/show/bgp/peer/london/` when `london` has not been created) redirects to the root view with an error notification.
<!-- source: internal/component/web/fragment.go -- HandleFragment, isListEntryPath check -->

### Notifications

Error notifications appear as toasts in the top-right corner with a 30-second countdown. Click the countdown to pause (for screenshots). Click the close button to dismiss immediately.
<!-- source: internal/component/web/templates/component/notification_error.html -->

### Input Auto-Save

Text and number fields auto-save 1 second after the user stops typing, in addition to saving on blur and Enter. This prevents data loss when navigating away before a field loses focus.
<!-- source: internal/component/web/templates/input/text.html -- hx-trigger with input changed delay:1s -->

### Conflict Detection

When two users edit the same leaf concurrently, the commit reports which paths conflict, showing both the local and other user's values. The user must resolve conflicts before committing.
<!-- source: internal/component/web/handler_config.go -- handleCommitPost, result.Conflicts -->

### Session Limits

The editor manager allows up to 50 concurrent user sessions. Idle sessions (no activity for 1 hour) are evicted when capacity is reached.
<!-- source: internal/component/web/editor.go -- NewEditorManager, maxSessions, idleTimeout -->

## CLI Bar

The web interface includes a CLI bar at the bottom of the page that accepts the same command grammar as the SSH CLI. The CLI bar sends the current URL path as context, so `set` and `delete` commands operate relative to the current view.
<!-- source: internal/component/web/cli.go -- HandleCLICommand, knownCLIVerbs -->
<!-- source: internal/component/web/assets/cli.js -- path extraction, fetch to /cli -->

### Integrated Mode

In integrated mode, CLI commands update the page content directly:

| Command | Effect |
|---------|--------|
| `edit <path>` | Navigate to a config path |
| `set <leaf> <value>` | Set a value at the current context path |
| `delete <leaf>` | Delete a value at the current context path |
| `show [path]` | Display config text at the current or specified path |
| `top` | Navigate to root |
| `up` | Navigate one level up |
| `commit` | Commit pending changes |
| `discard` | Discard pending changes |
| `help` | List available commands |

The prompt shows the current context path: `ze[bgp peer]# `.
<!-- source: internal/component/web/cli.go -- formatCLIPrompt, dispatchCLICommand -->

### Terminal Mode

Terminal mode provides a scrollback terminal in the browser. Commands produce plain text output identical to the SSH CLI, displayed in a scrollback area with prompt echo.
<!-- source: internal/component/web/cli.go -- HandleCLITerminal, executeTerminalCommand -->

### Tab Completion

The CLI bar provides tab completion via a JSON endpoint at `/cli/complete`. Completions are context-aware: when at `/show/bgp/peer/london/`, typing `set ` + Tab suggests `remote`, `local`, `timer` (children of the peer entry), not root-level items. For YANG union types that include an enum (e.g., `local/ip` accepting an IP address or `auto`), the enum values are offered as completions.
<!-- source: internal/component/web/cli.go -- HandleCLIComplete -->
<!-- source: internal/component/cli/completer.go -- valueCompletions, Yunion handling -->

## Live Updates

The web interface uses Server-Sent Events (SSE) to notify connected browsers when configuration changes are committed by any user. A notification banner appears with the username and a "Refresh" button.
<!-- source: internal/component/web/sse.go -- EventBroker, BroadcastConfigChange -->

Connect to the SSE stream at `/events` (requires authentication). The broker supports up to 100 concurrent SSE clients. Slow clients that fall behind have events dropped rather than blocking other clients.

### Event Format

Events use the standard SSE wire format:

```
event: config-change
data: <html-fragment>
```

The HTML fragment contains a notification banner with the change description and action buttons.

## Admin Commands

The `/admin/` tier provides a browsable tree of administrative commands. Container nodes display navigable links to sub-commands. Leaf commands display a parameter form with an "Execute" button.
<!-- source: internal/component/web/handler_admin.go -- HandleAdminView, HandleAdminExecute -->

Admin command results are displayed as titled cards showing the command name, output text, and success/error styling.

## Resilience

**Corrupt change files:** If a per-user change file in the blob store is unparseable (e.g., from a previous bug), it is automatically discarded with a warning log. The user can continue editing without manual intervention.
<!-- source: internal/component/cli/editor_draft.go -- readChangeFile -->

**Asset caching:** Static assets (`/assets/`) are served with `Cache-Control: no-cache, must-revalidate` so browsers always pick up changes after binary updates without requiring a hard refresh.
<!-- source: internal/component/web/render.go -- AssetHandler -->
