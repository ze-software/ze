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
| PKI certificate | `environment.web.certificate` names a `pki {}` store entry to serve instead, sending the leaf and every stored intermediate. A configured name that does not resolve stops the listener; ze never falls back to self-signed for it. Rotates on reload without rebinding, so open SSE streams survive. See [TLS Certificates From the PKI Store](../guide/configuration.md#tls-certificates-from-the-pki-store) |
| Security headers | HSTS, CSP, X-Frame-Options DENY, no-store cache on all authenticated responses |
| YANG decorators | Leaves with `ze:decorate` extension show enriched display text (e.g., ASN numbers annotated with organization name via Team Cymru DNS) |
| Workbench UI (default) | RouterOS-style operator workbench (default since Phase 2); row-level related-tool buttons declared via `ze:related` YANG extension dispatch through the standard CommandDispatcher; CLI available as separate `/cli` tab |
| templ rendering | Every page, panel, fragment and out-of-band swap is written in a `.templ` source and compiled to Go. No Go file in the package builds markup, so a renamed view-model field is a compile error instead of a blank panel |
| htmx 4 | htmx 4.0.0-beta6 is embedded, with `hx-sse.min.js` for the streams. Each page loads only the assets its component graph reaches |
| Secret masking | No render path prints a value the schema marks `ze:sensitive` or `ze:bcrypt`. One predicate answers "does this leaf hold a secret", and both the display mask and the write guard read it |

## Browser Configuration Workflow

The recording below signs in to a local Ze instance, edits a YANG-backed value,
reviews the generated diff, commits the browser session's draft, and verifies
the active value. The daemon and browser run locally during generation.

<!-- terminal-demo: web-config -->

<!-- source: internal/component/web/server.go -- WebServer, TLS config, cert generation, UpdateTLSCertificate -->
<!-- source: cmd/ze/hub/service_web.go -- webTLSMaterial: PKI store or self-signed, fail closed -->
<!-- source: internal/component/web/decorator.go -- Decorator registry and interface -->
<!-- source: internal/component/web/decorator_asn.go -- ASN name decorator via Team Cymru DNS -->
<!-- source: internal/component/web/auth.go -- SessionStore, AuthMiddleware, LoginHandler -->
<!-- source: internal/component/web/handler.go -- URL routing, content negotiation, three-tier scheme -->
<!-- source: internal/component/web/handler_config.go -- Config view and edit handlers -->
<!-- source: internal/component/web/handler_admin.go -- Admin command handlers -->
<!-- source: internal/component/web/cli.go -- CLI bar and terminal mode -->
<!-- source: internal/component/web/sse.go -- EventBroker SSE broadcast -->
<!-- source: internal/component/web/editor.go -- EditorManager per-user sessions -->
<!-- source: internal/component/web/render.go -- Renderer, renderComponent, AssetHandler -->
<!-- source: internal/component/web/markup_check_test.go -- webMarkupExempt: the empty exemption table -->
<!-- source: internal/component/config/mask.go -- LeafHoldsSecret, MaskSecrets, RejectMaskedSecretLeaves -->

See [Web Interface Guide](../guide/web-interface.md) for usage instructions.
