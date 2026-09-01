# Web Interface

Ze provides an HTTPS web interface for browsing configuration, editing values, and running commands through a browser. The web UI uses the same YANG schemas, user database, and command grammar as the SSH CLI.
<!-- source: internal/component/web/handler.go -- URL routing, three-tier scheme -->
<!-- source: internal/component/web/auth.go -- authMiddleware, AuthMiddlewareWithAudit -->
<!-- source: cmd/ze/hub/service_web.go -- buildWebService -->
<!-- source: cmd/ze/hub/aaa_lifecycle.go -- liveAAABundleAuthenticator, liveAAABundleAuthenticator.Authenticate -->
<!-- source: internal/component/authz/auth.go -- LocalAuthenticator.Authenticate, authenticateUser -->

## Starting the Web Server

### Command Line

```bash
ze start --web 8443                              # Start daemon + web on port 8443
ze start --web 8443 --insecure-web               # No authentication (forces 127.0.0.1)
```

When no certificate is configured, ze generates an ECDSA P-256 self-signed certificate automatically. The certificate includes SANs for localhost, 127.0.0.1, ::1, and the listen address.
<!-- source: cmd/ze/ze_core_start.go -- cmdStart, flagStartWeb, flagStartInsecureWeb -->
<!-- source: internal/core/selfcert/selfcert.go -- GenerateWebCertWithAddr, GenerateWebCertWithNames -->

| Flag | Description |
|------|-------------|
| `--web <port>` | Start web interface on `0.0.0.0:<port>` (requires config) |
| `--web-only` | Start web UI only, no daemon (config editing only, default port 3443) |
| `--insecure-web` | Disable authentication (forces `127.0.0.1`, requires `--web` or `--web-only`) |

### Workbench UI

Ze defaults to a RouterOS-style operator workbench UI. The workbench keeps the same authentication and commit flow; the CLI is available as a separate `/cli` tab instead of a bottom bar. BGP peer rows ship with related operator tools (peer detail, capabilities, statistics, flush, teardown) that run the same dispatched commands as the SSH CLI. Confirmation prompts gate destructive tools.

To roll back to the legacy Finder UI:

```bash
ZE_WEB_UI=finder ze start --web 8443
```

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
<!-- source: internal/component/web/yang/ze-web-conf.yang -- web container, server list -->

## Authentication

The web interface uses the same user database as the SSH server. Users log in through a browser login page or authenticate API requests with HTTP Basic Auth.
<!-- source: internal/component/web/auth.go -- authMiddleware, loginHandler, parseBasicAuth -->

### Browser Sessions

1. Navigate to `https://<host>:8443/`. Unauthenticated requests receive a login page.
2. Enter username and password. On success, a `ze-session` cookie is set.
3. The cookie is `Secure`, `HttpOnly`, and `SameSite=Strict`.
4. Each user can have one active session. Logging in again invalidates the previous session.
5. A session lasts 24 hours, and ends earlier if the configuration stops declaring the user. Remove a user and reload, and their open browser tab is refused on its next request. No daemon restart is necessary.

A user authenticated by RADIUS or TACACS+ keeps their session: the local user list did not grant it and does not end it.

### JSON API

API clients that send `Accept: application/json` (or append `?format=json` to the URL) can authenticate with HTTP Basic Auth instead of session cookies. No session is created for Basic Auth requests.

```bash
curl -k -u admin:password https://localhost:8443/show/bgp/?format=json
```

### Security Headers

Every response carries `X-Frame-Options: DENY`, `X-Content-Type-Options: nosniff`, `Content-Security-Policy: default-src 'self'; script-src 'self'; style-src 'self'`, and HSTS (`max-age=63072000; includeSubDomains`). An authenticated response adds `Cache-Control: no-store` and `X-Ze-Version`.
<!-- source: internal/component/web/auth.go -- setSecurityHeaders, addSecurityHeaders -->

`script-src 'self'` refuses an inline script and refuses `Function()`. No page therefore carries an inline event handler, and no htmx attribute uses a bracketed trigger filter, because htmx compiles such a filter into source and calls `Function()` on it. A test refuses both in any `.templ` source.
<!-- source: internal/component/web/markup_contract_test.go -- TestNoTriggerFilterNeedsEval, TestSelfReplacingControlsCarryAStableID -->

### Secret Masking

No render path prints a value the schema marks `ze:sensitive` or `ze:bcrypt`. One
predicate, `config.LeafHoldsSecret`, answers whether a leaf holds a secret. The
display mask and the write guard both read it, so the two halves cannot drift
apart.

| Surface | Behavior |
|---------|----------|
| The config tree, the diff, the compare view and the download | A secret leaf that holds a value reads as the placeholder. An unset secret stays empty, so the field still reads as unconfigured |
| The commit diff | A rotated secret is named as a changed path. Neither the old value nor the new one is printed |
| The web CLI bar and the terminal | `show` masks the same leaves. The verb needs no config authorization, so any authenticated session used to reach them |
| Commit, load and upload | A tree carrying the placeholder in a secret leaf is refused. Restore the real value from the edit-authorized raw download, or set it through `plaintext-<name>` or `ze passwd` |

<!-- source: internal/component/config/mask.go -- LeafHoldsSecret, MaskSecrets, ChangedSecretPaths, RejectMaskedSecretLeaves -->

## Rendering

Every page, panel, fragment and out-of-band swap is written in a `.templ` source
and compiled to Go by [templ](https://templ.guide). No Go file in the package
builds markup: the exemption table is empty and a test holds it at zero. A
renamed view-model field is therefore a compile error, where `html/template`
rendered a blank panel and answered 200.

| Command | What it does |
|---------|--------------|
| `./le repository generate` | Regenerates every `*_templ.go` from its `.templ` source. Run it after any `.templ` edit |
| `./le doc check templ-output` | Refuses stale generated output, orphaned `*_templ.go` files, and `.templ` sources outside the generator walk |

The generator runs from `vendor/`, so `./le repository generate` needs no network and
nothing on `PATH`. Run `./le repository generate`, not a bare `templ generate`: the walk
root is written into the generated Go, so a bare run rewrites every file and
reds the check.

The browser assets are vendored in `third_party/web/` and copied to each
consumer by the same `./le repository generate` run. htmx 4.0.0-beta6 and its
`hx-sse.min.js` extension serve the web interface, the looking glass and the
chaos dashboard. `internal/le/webassets.Write` derives each page's asset set
from its component graph, so a page loads only what it reaches.

<!-- source: internal/component/web/markup_check_test.go -- webMarkupExempt, TestNoGoFileBuildsMarkup -->
<!-- source: third_party/web/MANIFEST.md -- vendored versions, consumers, sync targets -->

## Navigation

### URL Scheme

URLs follow a verb-first three-tier pattern:
<!-- source: internal/component/web/handler.go -- ParseURL, knownPrefixes, configVerbs -->
<!-- source: internal/component/web/handler_config_form.go -- HandleConfigDeleteWithAuthorizer -->

| Tier | URL Pattern | Method | Description |
|------|-------------|--------|-------------|
| View | `/show/<yang-path>` | GET | Read-only config tree view |
| View | `/monitor/<yang-path>` | GET | View with auto-polling |
| Config | `/config/edit/<path>` | GET | Editable config tree view |
| Config | `/config/set/<path>` | POST | Set a leaf value |
| Config | `/config/add/<path>` | POST | Create a list entry (with optional field values) |
| Config | `/config/add-form/<path>` | GET | Fetch add-entry overlay form |
| Config | `/config/rename/<path>` | POST | Rename a keyed list entry |
| Config | `/config/delete/<path>` | POST | Delete the node named by the `leaf` form field: a leaf, a leaf-list member, a container, a whole list, or one list entry (the delete button on a list row) |
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
<!-- source: internal/component/web/component_list_table.templ -->

| Column | Behavior |
|--------|----------|
| Rename button | Opens a modal, normalizes the new key, and renames the entry without losing its subtree |
| Key column (e.g., name) | Clickable link, navigates into the entry's config subtree |
| Unique field columns (e.g., remote/ip) | Editable inline, saves on blur/Enter/auto-save (1s debounce) |
| Delete button | Removes the entry after confirmation |

The `+ new` button below the table opens a server-rendered form (via HTMX) with inputs for the entry name and all unique fields. Field values are validated against YANG types before the entry is created.
<!-- source: internal/component/web/handler_config_entry.go -- HandleConfigAddWithAuthorizer, HandleConfigAddForm -->
<!-- source: internal/component/web/component_add_form_overlay.templ -->

### Breadcrumb Navigation

Every page displays a breadcrumb trail from root to the current YANG path. Clicking any breadcrumb segment navigates to that level.
<!-- source: internal/component/web/handler_config_leaf.go -- buildBreadcrumbs -->

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
<!-- source: internal/component/web/component_notification_error.templ -->

A refused action raises a toast carrying the status and the message the daemon wrote, and the same message lands in the field or panel the action came from. A request that gets no answer at all raises a toast that says so instead: the daemon is unreachable, and nothing was changed.
<!-- source: internal/component/web/assets/notification.js -- handleResponseError and handleRequestError -->

### Input Auto-Save

Text and number fields auto-save 1 second after the user stops typing, in addition to saving on blur and Enter. This prevents data loss when navigating away before a field loses focus. Enter commits inside the debounce, so three keystrokes and an Enter send one POST, not four.

Enter arrives as `ze-enter`, a named event a delegated listener in `assets/cli.js` dispatches. The listener reads the element's own `hx-trigger`, lives in a file rather than in an attribute, and survives every swap. An inline editor replaces itself, so each input also carries a stable id derived from its leaf path: without one, htmx restored focus by looking up the empty string and the caret was lost on every save.
<!-- source: internal/component/web/input_text.templ -- hx-trigger="blur changed, ze-enter, input changed delay:1s" -->
<!-- source: internal/component/web/assets/cli.js -- initEnterSubmit -->
<!-- source: internal/component/web/view.go -- fieldInputID, fieldInputIDEscaper -->

### Conflict Detection

When two users edit the same leaf concurrently, the commit reports which paths conflict, showing both the local and other user's values. The user must resolve conflicts before committing.
<!-- source: internal/component/web/handler_config_commit.go -- handleCommitPost -->
<!-- source: internal/component/cli/contract/contract.go -- CommitResult.Conflicts, Conflict -->

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
<!-- source: internal/component/web/cli_terminal.go -- HandleCLITerminalWithDispatchAuthorizerAndAudit, executeTerminalCommand -->

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

The form also shows the command's help, read from the same command model every other surface reads. The declared one-line summary is the lede above the form, and the declared long explanation is the paragraph under it. A command that declares no explanation shows the summary alone. A path the command tree does not hold shows neither text, and never borrows its parent's.
<!-- source: internal/component/web/handler_admin.go -- HandleAdminView, HandleAdminExecute, buildAdminFragmentData -->
<!-- source: internal/component/web/component_command_form.templ -- commandForm -->

Admin command results are displayed as titled cards showing the command name, output text, and success/error styling.

## Resilience

**Corrupt change files:** If a per-user change file in the blob store is unparseable (e.g., from a previous bug), it is automatically discarded with a warning log. The user can continue editing without manual intervention.
<!-- source: internal/component/cli/editor_draft.go -- readChangeFile -->

**Asset caching:** Static assets (`/assets/`) are served with `Cache-Control: no-cache, must-revalidate` so browsers always pick up changes after binary updates without requiring a hard refresh.
<!-- source: internal/component/web/render.go -- AssetHandler -->
