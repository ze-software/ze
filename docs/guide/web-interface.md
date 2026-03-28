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

### Configuration

The web server listen address can also be set in the ze configuration file:

```
environment {
    web {
        host 0.0.0.0;
        port 8443;
    }
}
```
<!-- source: internal/component/web/schema/ze-web-conf.yang -- web container, host/port leaves -->

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
| Config | `/config/delete/<path>` | POST | Delete a leaf value |
| Config | `/config/commit/` | GET/POST | View diff and commit changes |
| Config | `/config/discard/` | POST | Discard pending changes |
| Config | `/config/compare/` | GET | Compare pending vs committed |
| Admin | `/admin/<yang-path>` | GET/POST | Administrative commands |
| Auth | `/login` | POST | Login (no auth required) |
| Static | `/assets/` | GET | CSS, JS, images (no auth required) |

The root URL `/` redirects to `/show/`.

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

1. **Browse:** Navigate to `/config/edit/<path>/` to see containers, lists, and leaf values.
2. **Set:** Change a leaf value through the form or the CLI bar (`set <leaf> <value>`).
3. **Delete:** Remove a configured value through the form or CLI bar (`delete <leaf>`).
4. **Review:** Visit `/config/commit/` to see a diff of pending changes.
5. **Commit:** POST to `/config/commit/` to apply changes. Conflicts with other users are detected and reported.
6. **Discard:** POST to `/config/discard/` to abandon all pending changes.

### Conflict Detection

When two users edit the same leaf concurrently, the commit reports which paths conflict, showing both the local and other user's values. The user must resolve conflicts before committing.
<!-- source: internal/component/web/handler_config.go -- handleCommitPost, result.Conflicts -->

### Session Limits

The editor manager allows up to 50 concurrent user sessions. Idle sessions (no activity for 1 hour) are evicted when capacity is reached.
<!-- source: internal/component/web/editor.go -- NewEditorManager, maxSessions, idleTimeout -->

## CLI Bar

The web interface includes a CLI bar at the bottom of the page that accepts the same command grammar as the SSH CLI.
<!-- source: internal/component/web/cli.go -- HandleCLICommand, knownCLIVerbs -->

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

The CLI bar provides tab completion via a JSON endpoint at `/cli/complete`. Completions are context-aware and respect the current path.
<!-- source: internal/component/web/cli.go -- HandleCLIComplete -->

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
