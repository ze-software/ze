# 439 -- Web Interface Foundation

## Context

Ze needed a web interface for config viewing and editing, accessible via HTTPS. Not all users want to use the CLI over SSH. The web UI is generated from YANG schemas, mirroring the CLI experience in a browser. This phase established the HTTP server infrastructure with TLS, session-based authentication, content negotiation, and the page layout frame.

## Decisions

- Placed as component (`internal/component/web/`) over plugin -- the "delete the folder" test: web serves all config, not just BGP.
- Session-based auth (HttpOnly cookie) over stateless Basic Auth on every request -- solves SSE EventSource auth (cookies sent automatically), prevents XSS credential theft.
- Self-signed ECDSA P-256 TLS cert auto-generated over RSA -- smaller keys, faster handshakes.
- Content negotiation via Accept header with `?format=json` override (URL wins) over URL suffix -- standard HTTP, no URL pollution.
- Go `html/template` over inline string rendering (as used in chaos dashboard) -- auto-escaping prevents XSS.

## Consequences

- All subsequent web phases build on this foundation (server, auth, templates, assets).
- Session cookie means server has state per user (unlike stateless Basic Auth).
- Self-signed cert means browser warnings until user provides a real cert.
- Same zefs credentials as SSH -- no separate user management.

## Gotchas

- `EventSource` (SSE) does not support custom headers -- this drove the switch from Basic Auth to session cookies.
- Go's `vendor/` directory is reserved for `go mod vendor` -- vendored HTMX assets moved to `third_party/web/`.
- The `confModules` list in `completer.go` is hardcoded -- new YANG modules must be added manually.
- `go:embed` only works with paths relative to the package directory -- assets must be copied to each consumer.

## Files

- `internal/component/web/server.go`, `auth.go`, `handler.go`, `render.go`
- `internal/component/web/schema/ze-web-conf.yang`, `register.go`, `embed.go`
- `internal/component/web/templates/layout.html`, `login.html`
- `internal/component/web/assets/htmx.min.js`, `sse.js`, `style.css`
- `cmd/ze/web/main.go`, `cmd/ze/main.go` (dispatch)
