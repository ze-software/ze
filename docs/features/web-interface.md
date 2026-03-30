# Web Interface

Ze includes an HTTPS web interface for configuration viewing, editing, and runtime command execution through a browser.

| Feature | Description |
|---------|-------------|
| YANG-driven UI | Config tree navigation generated from YANG schemas |
| Finder navigation | macOS-style column browser; named containers above unnamed with separator |
| List table view | Lists with YANG `unique` constraints shown as interactive tables with inline editing |
| Config viewing | Browse the config tree with breadcrumb navigation |
| Config editing | Set and delete leaf values with per-user draft sessions |
| Inline diff | Review pending changes before committing |
| Session authentication | Login page with session cookies; same user database as SSH |
| JSON API | Content negotiation via `Accept` header or `?format=json` query parameter; Basic Auth for API clients |
| CLI bar | Integrated command bar with the same grammar as the SSH CLI (edit, set, delete, show, commit, discard) |
| Terminal mode | Full terminal mode in the browser with scrollback and prompt |
| Tab completion | Autocomplete candidates served via JSON endpoint |
| Live updates | SSE notifications when another user commits config changes |
| HTTPS only | TLS 1.2 minimum; auto-generated ECDSA P-256 self-signed certificate when no cert is provided |
| Security headers | HSTS, CSP, X-Frame-Options DENY, no-store cache on all authenticated responses |
| YANG decorators | Leaves with `ze:decorate` extension show enriched display text (e.g., ASN numbers annotated with organization name via Team Cymru DNS) |

<!-- source: internal/component/web/server.go -- WebServer, TLS config, cert generation -->
<!-- source: internal/component/web/decorator.go -- Decorator registry and interface -->
<!-- source: internal/component/web/decorator_asn.go -- ASN name decorator via Team Cymru DNS -->
<!-- source: internal/component/web/auth.go -- SessionStore, AuthMiddleware, LoginHandler -->
<!-- source: internal/component/web/handler.go -- URL routing, content negotiation, three-tier scheme -->
<!-- source: internal/component/web/handler_config.go -- Config view and edit handlers -->
<!-- source: internal/component/web/handler_admin.go -- Admin command handlers -->
<!-- source: internal/component/web/cli.go -- CLI bar and terminal mode -->
<!-- source: internal/component/web/sse.go -- EventBroker SSE broadcast -->
<!-- source: internal/component/web/editor.go -- EditorManager per-user sessions -->

See [Web Interface Guide](guide/web-interface.md) for usage instructions.
