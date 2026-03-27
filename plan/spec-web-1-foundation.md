# Spec: web-1 -- Web Interface Foundation

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 1/9 |
| Updated | 2026-03-27 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `plan/spec-web-0-umbrella.md` -- umbrella spec with all design decisions
4. `internal/component/ssh/auth.go` -- AuthenticateUser, CheckPassword, UserConfig
5. `internal/chaos/web/` -- existing HTMX+SSE web patterns (chaos dashboard)
6. `cmd/ze/main.go` -- CLI subcommand dispatch

## Task

Implement Phase 1 (Foundation) of the Ze web interface. This phase establishes the HTTP server infrastructure, TLS, authentication, content negotiation, and the page layout frame. All subsequent web phases build on this foundation.

The foundation provides:
- An HTTPS server started by `ze web` with self-signed TLS by default
- YANG-configured listen address (`web {}` config block)
- Session-based auth middleware reusing `AuthenticateUser()` from the SSH component
- A custom login page that POSTs credentials, receives a session cookie, and redirects to `/config/edit/`
- JSON API consumers use Basic Auth (no session cookie)
- Content negotiation: `Accept` header as primary signal, `?format=json` as URL override, URL wins on conflict. Unknown `?format` values are ignored, falls back to Accept header or HTML default
- A page layout frame with four areas: breadcrumb, content, notification, and CLI bar
- Embedded static assets: htmx.min.js, sse.js, style.css

URL scheme uses a verb-first three-tier layout: `/show/<path>` and `/monitor/<path>` for views (GET), `/config/<verb>/<path>` for config editing, and `/admin/<path>` for operational mutations (POST).

Parent spec: `spec-web-0-umbrella.md` (design decisions D-1, D-5, D-7, D-8, D-11, D-14).

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] -- checkboxes are template markers, not progress trackers. -->
<!-- Capture insights as -> Decision: / -> Constraint: annotations -- these survive compaction. -->
<!-- Track reading progress in session-state.md, not here. -->
- [ ] `docs/architecture/core-design.md` -- overall architecture, component placement
  -> Constraint: web UI is a component (`internal/component/web/`), not a plugin
- [ ] `docs/architecture/zefs-format.md` -- zefs storage, credential format, meta namespace
  -> Constraint: credentials are bcrypt hashes in `meta/` namespace; self-signed cert goes in `meta/web/`
- [ ] `docs/architecture/chaos-web-dashboard.md` -- existing HTMX embedding patterns
  -> Decision: reuse HTMX asset embedding pattern from chaos dashboard
- [ ] `plan/spec-web-0-umbrella.md` -- all design decisions for the web interface
  -> Decision: D-5 (session-based auth, custom login page), D-11 (content negotiation), D-14 (TLS)

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc7617.md` -- HTTP Basic Auth scheme
  -> Constraint: `Authorization: Basic base64(user:pass)` header format

**Key insights:**
- Web UI is a component, not a plugin -- placed in `internal/component/web/`
- Auth reuses `AuthenticateUser()` from `internal/component/ssh/auth.go` -- same users, same credentials
- Self-signed cert auto-generated into zefs `meta/web/` on first run, CLI flags override. Uses crypto/rand for key generation, ECDSA P-256 preferred, SAN for localhost + listen address, private key 0600 permissions
- Content negotiation: Accept header primary, `?format=json` override, URL wins on conflict. Unknown `?format` values are ignored, falls back to Accept header or HTML default
- Login page uses custom HTML form (no `WWW-Authenticate` header, no browser popup). Form POSTs credentials to login endpoint, server validates against zefs, returns `Set-Cookie: ze-session=<token>; Secure; HttpOnly; SameSite=Strict`, page redirects to `/config/edit/`
- JSON API consumers use Basic Auth directly (no session cookie)
- New login invalidates previous session. Old page stays visible (read-only ghost). Login overlay (dismissible modal) appears on any action
- URL scheme: verb-first three-tier layout -- `/show/<path>` and `/monitor/<path>` for views, `/config/<verb>/<path>` for config editing, `/admin/<path>` for operational mutations
- Completer `confModules` in `completer.go` is hardcoded -- `ze-web-conf` must be added for web config to be visible in CLI

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
<!-- Same rule: never tick [ ] to [x]. Write -> Constraint: annotations instead. -->
- [ ] `internal/component/ssh/auth.go` -- `AuthenticateUser()` validates plaintext password against bcrypt hash in zefs `meta/`
  -> Constraint: function signature must not change; web auth calls it identically to SSH
- [ ] `cmd/ze/main.go` -- subcommand dispatch table; `ze web` must be added as a new case
  -> Constraint: follows existing dispatch pattern (case string match, call `Run(args)`)
- [ ] `internal/chaos/web/` -- chaos dashboard embeds HTMX via `go:embed`, serves assets from embedded FS
  -> Decision: follow same embedding pattern for web component assets
- [ ] `internal/component/config/` -- YANG schema loading and config tree walking
  -> Constraint: `web {}` config block parsed by same pipeline as all other components

**Behavior to preserve:**
- `AuthenticateUser()` function signature and behavior (SSH depends on it)
- Existing subcommand dispatch in `cmd/ze/main.go` (all current commands unchanged)
- YANG module loading pipeline (new `ze-web-conf.yang` registers like all other schemas)
- zefs `meta/` namespace conventions (self-signed cert follows same storage pattern)

**Behavior to change:**
- Add `web` case to CLI subcommand dispatch in `cmd/ze/main.go`
- Add `ze-web-conf.yang` YANG module for `web {}` config block

## Data Flow (MANDATORY -- see `rules/data-flow-tracing.md`)

### Entry Point
- User runs `ze web` CLI command or Ze engine starts with `web {}` config block
- Browser sends HTTPS request to listen address

### Transformation Path
1. `cmd/ze/web/main.go` parses flags (`--cert`, `--key`, `--listen`), loads config
2. `internal/component/web/server.go` creates TLS config (self-signed from zefs or CLI-provided cert)
3. `internal/component/web/server.go` registers routes on mux, starts HTTPS listener
4. Browser request arrives at mux
5. Login flow: browser POSTs credentials to login endpoint, `internal/component/web/auth.go` calls `AuthenticateUser()`, on success returns `Set-Cookie: ze-session=<token>; Secure; HttpOnly; SameSite=Strict` and redirects to `/config/edit/`
6. Subsequent requests: `internal/component/web/auth.go` middleware validates session cookie. JSON API requests may use Basic Auth header instead
7. Missing or invalid session (and no valid Basic Auth): return 401 with login page HTML (no `WWW-Authenticate` header)
8. Valid session or Basic Auth: pass request to `internal/component/web/handler.go`
9. `internal/component/web/handler.go` parses URL path using verb-first scheme (`/show/<path>`, `/config/<verb>/<path>`, `/admin/<path>`), determines content type (Accept header vs `?format=json`)
10. `internal/component/web/render.go` executes appropriate template (`layout.html` wrapping content)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| CLI -> Server | `cmd/ze/web/` calls `web.NewServer()` with config | [ ] |
| HTTP -> Auth | Login endpoint validates credentials via `AuthenticateUser()`, sets session cookie. Middleware validates session cookie on subsequent requests. JSON API falls back to Basic Auth | [ ] |
| Auth -> zefs | `AuthenticateUser()` reads bcrypt hash from zefs `meta/` | [ ] |
| Handler -> Template | Handler passes data struct to `html/template.Execute()` | [ ] |
| Config -> Server | YANG `web {}` block parsed into listen address for server | [ ] |

### Integration Points
- `internal/component/ssh/auth.go` -- `AuthenticateUser()` called by web auth middleware
- `cmd/ze/main.go` -- dispatch table adds `"web"` case
- `internal/component/config/` -- YANG module registration for `ze-web-conf.yang`
- zefs `meta/web/` -- self-signed TLS cert storage

### Architectural Verification
- [ ] No bypassed layers (auth middleware always runs before handlers)
- [ ] No unintended coupling (web component depends on auth function, not SSH internals)
- [ ] No duplicated functionality (reuses `AuthenticateUser()`, does not reimplement)
- [ ] Zero-copy preserved where applicable (N/A for this phase -- no wire data)

## Wiring Test (MANDATORY -- NOT deferrable)

<!-- BLOCKING: Proves the feature is reachable from its intended entry point. -->
<!-- Without this, the feature exists in isolation -- unit tests pass but nothing calls it. -->
<!-- Every row MUST have a test name. "Deferred" / "TODO" / empty = spec cannot be marked done. -->
| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `ze web` CLI command | -> | HTTP server starts, serves HTTPS | `test/plugin/web-startup.ci` |
| POST login credentials, then GET `/show/` with session cookie | -> | Auth middleware passes, layout rendered | `test/plugin/web-auth.ci` |
| GET `/show/?format=json` with valid session | -> | Content negotiation returns JSON | `test/plugin/web-json-response.ci` |

## Acceptance Criteria

<!-- Define BEFORE implementation. Each row is a testable assertion. -->
<!-- The Implementation Audit cross-references these criteria. -->
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `ze web` with config `web { listen 0.0.0.0:8080; }` | HTTPS server starts on port 8080 with self-signed TLS |
| AC-2 | Browser to `/` without session cookie | 401 response with custom login page HTML body (no `WWW-Authenticate` header) |
| AC-3 | POST valid credentials to login endpoint | Server validates against zefs, returns `Set-Cookie: ze-session=<token>; Secure; HttpOnly; SameSite=Strict`, redirects to `/config/edit/` |
| AC-4 | POST invalid credentials to login endpoint | 401 response with login page HTML body |
| AC-5 | GET `/show/?format=json` with valid session cookie | Response with `Content-Type: application/json` |
| AC-6 | GET `/show/` with `Accept: application/json` header and valid session cookie | Response with `Content-Type: application/json` |
| AC-7 | Both `Accept: text/html` header and `?format=json` query param, valid session | JSON response returned (URL parameter wins over Accept header) |
| AC-8 | `ze web --cert /path/cert.pem --key /path/key.pem` | Server uses provided TLS certificate instead of self-signed |
| AC-9 | First `ze web` run with no existing cert in zefs | Self-signed certificate auto-generated into zefs `meta/web/` directory |
| AC-10 | Session expires or new login from another browser invalidates old session | Login overlay is dismissible. User can read stale page content. Any mutation action re-shows the overlay. Successful re-login restores session without full page reload |
| AC-11 | GET `/show/` with valid session cookie | 200 response with layout HTML containing breadcrumb, content, notification, and CLI bar areas |
| AC-12 | JSON API request with valid Basic Auth header (no session cookie) | Request succeeds; JSON API consumers do not need session cookies |
| AC-13 | Any authenticated page response | Server sets `X-Frame-Options: DENY`, `X-Content-Type-Options: nosniff`, `Content-Security-Policy: default-src 'self'; script-src 'self'`, `Strict-Transport-Security: max-age=31536000`, `Cache-Control: no-store` |
| AC-14 | Self-signed cert generation fails (e.g., filesystem error) | `ze web` exits with code 1 and prints error to stderr |
| AC-15 | `--cert`/`--key` files are unreadable or contain invalid certificate data | `ze web` exits with code 2 and prints error to stderr |
| AC-16 | URL path segment containing `..`, empty string, null byte, or non-YANG identifier character | 400 Bad Request returned before schema walking |
| AC-17 | `?format=invalid` query parameter | Unknown format value is ignored, falls back to Accept header or HTML default |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestSessionCookieValidation` | `internal/component/web/auth_test.go` | Missing/invalid session cookie returns 401 with login page HTML, no `WWW-Authenticate` header. Valid session cookie passes through to wrapped handler | |
| `TestSessionCreation` | `internal/component/web/auth_test.go` | POST valid credentials to login endpoint returns `Set-Cookie` with `ze-session` token, `Secure`, `HttpOnly`, `SameSite=Strict` flags | |
| `TestSessionInvalidation` | `internal/component/web/auth_test.go` | New login invalidates previous session token. Requests with old token receive 401 | |
| `TestLoginOverlayOnExpiredSession` | `internal/component/web/auth_test.go` | Expired or invalidated session triggers login overlay response (not full 401 redirect) for HTMX requests | |
| `TestBasicAuthForJSONAPI` | `internal/component/web/auth_test.go` | JSON API request with valid Basic Auth header succeeds without session cookie | |
| `TestSecurityHeaders` | `internal/component/web/auth_test.go` | Authenticated responses include `X-Frame-Options: DENY`, `X-Content-Type-Options: nosniff`, `Content-Security-Policy`, `Strict-Transport-Security`, `Cache-Control: no-store` | |
| `TestPathTraversalRejected` | `internal/component/web/handler_test.go` | URL path segments containing `..`, empty strings, null bytes, or non-YANG identifier characters return 400 Bad Request | |
| `TestURLToPath` | `internal/component/web/handler_test.go` | `/config/edit/bgp/peer/192.168.1.1/` parses to path segments `["bgp", "peer", "192.168.1.1"]` + verb `edit` | |
| `TestURLToPathRoot` | `internal/component/web/handler_test.go` | `/show/` parses to empty path with implicit `show` verb | |
| `TestURLVerbExtraction` | `internal/component/web/handler_test.go` | `/config/set/bgp/peer/192.168.1.1` parses to path `["bgp", "peer", "192.168.1.1"]` + verb `set` | |
| `TestURLShowMonitorPaths` | `internal/component/web/handler_test.go` | `/show/bgp/peer/` and `/monitor/bgp/peer/` parse to path `["bgp", "peer"]` with correct tier | |
| `TestURLAdminPath` | `internal/component/web/handler_test.go` | `/admin/restart/` parses to path `["restart"]` under admin tier | |
| `TestContentNegotiationAcceptJSON` | `internal/component/web/handler_test.go` | `Accept: application/json` header produces JSON content type | |
| `TestContentNegotiationFormatParam` | `internal/component/web/handler_test.go` | `?format=json` query parameter produces JSON content type | |
| `TestContentNegotiationURLWins` | `internal/component/web/handler_test.go` | `Accept: text/html` header with `?format=json` param produces JSON (URL wins) | |
| `TestContentNegotiationDefault` | `internal/component/web/handler_test.go` | No Accept header and no format param produces HTML content type | |
| `TestContentNegotiationUnknownFormat` | `internal/component/web/handler_test.go` | `?format=invalid` is ignored, falls back to Accept header or HTML default | |
| `TestTemplateLoading` | `internal/component/web/render_test.go` | All embedded templates parse without error via `html/template` | |
| `TestSelfSignedCertGeneration` | `internal/component/web/server_test.go` | Generates a valid self-signed X.509 certificate with ECDSA P-256 key | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Listen port | 1-65535 | 65535 | 0 | 65536 |

### Functional Tests
<!-- REQUIRED: Verify feature works from end-user perspective -->
<!-- New RPCs/APIs MUST have functional tests -- unit tests alone are NOT sufficient -->
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `web-startup` | `test/plugin/web-startup.ci` | `ze web` starts HTTPS server, responds to requests | |
| `web-auth` | `test/plugin/web-auth.ci` | Valid auth returns content, missing auth returns login page | |
| `web-json-response` | `test/plugin/web-json-response.ci` | `?format=json` and Accept header produce JSON responses | |

### Future (if deferring any tests)
- None -- all tests for this phase are in scope

## Files to Modify
<!-- MUST include feature code (internal/*, cmd/*), not only test files -->
<!-- Check // Design: annotations on each file -- if the change affects behavior
     described in the referenced architecture doc, include the doc here too -->
- `cmd/ze/main.go` -- add `"web"` case to subcommand dispatch, calling `web.Run(args)`

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | Yes | `internal/component/web/schema/ze-web-conf.yang` |
| CLI commands/flags | Yes | `cmd/ze/web/main.go` |
| Editor autocomplete | No | N/A (no new editor commands in this phase) |
| Functional test for new RPC/API | Yes | `test/plugin/web-startup.ci`, `test/plugin/web-auth.ci`, `test/plugin/web-json-response.ci` |

### Documentation Update Checklist (BLOCKING)
<!-- Every row MUST be answered Yes/No during the Completion Checklist (planning.md step 1). -->
<!-- Every Yes MUST name the file and what to add/change. -->
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` -- add web interface entry |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md` -- add `web {}` block syntax |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` -- add `ze web` command |
| 4 | API/RPC added/changed? | No | N/A |
| 5 | Plugin added/changed? | No | N/A (web is a component, not a plugin) |
| 6 | Has a user guide page? | Yes | `docs/guide/web.md` -- new page for web interface usage |
| 7 | Wire format changed? | No | N/A |
| 8 | Plugin SDK/protocol changed? | No | N/A |
| 9 | RFC behavior implemented? | No | N/A (Basic Auth is standard HTTP, no BGP RFC) |
| 10 | Test infrastructure changed? | No | N/A |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` -- add web interface to feature comparison |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` -- add web component to system diagram |

## Files to Create
- `cmd/ze/web/main.go` -- `func Run(args []string) int`, flag parsing (`--cert`, `--key`, `--listen`), config loading, server startup
- `internal/component/web/server.go` -- HTTP server creation, TLS configuration (self-signed or CLI-provided), route registration, mux setup, listen and serve
- `internal/component/web/auth.go` -- Session-based auth middleware. Login endpoint: POST credentials, validate via `AuthenticateUser()`, return `Set-Cookie: ze-session=<token>`. Middleware: validate session cookie on subsequent requests. JSON API fallback: accept Basic Auth header. Security headers applied to all authenticated responses. New login invalidates previous session
- `internal/component/web/handler.go` -- URL path parsing for verb-first three-tier scheme (`/show/<path>`, `/monitor/<path>`, `/config/<verb>/<path>`, `/admin/<path>`), path traversal rejection (`..`, null bytes, non-YANG characters), content negotiation logic (Accept header vs `?format=json`, URL wins, unknown format values ignored), dispatch to appropriate renderer
- `internal/component/web/render.go` -- template loading via `go:embed`, template execution, data struct preparation for templates
- `internal/component/web/schema/register.go` -- `init()` function registering `ze-web-conf.yang` module
- `internal/component/web/schema/ze-web-conf.yang` -- YANG module defining `web` container with `listen` leaf (type string, default "0.0.0.0:8443")
- `internal/component/web/templates/layout.html` -- page frame template with four areas: breadcrumb, content, notification, CLI bar; includes HTMX script tags and stylesheet link
- `internal/component/web/templates/login.html` -- login form page with username and password fields. Form POSTs credentials to login endpoint; on success server returns session cookie and page redirects to `/config/edit/`. Also used as dismissible login overlay for expired/invalidated sessions
- `internal/component/web/assets/htmx.min.js` -- HTMX library (vendored)
- `internal/component/web/assets/sse.js` -- HTMX SSE extension (vendored)
- `internal/component/web/assets/style.css` -- base stylesheet (dark theme, layout grid for four areas)
- `internal/component/web/auth_test.go` -- unit tests for auth middleware
- `internal/component/web/handler_test.go` -- unit tests for URL parsing and content negotiation
- `internal/component/web/render_test.go` -- unit tests for template loading
- `internal/component/web/server_test.go` -- unit tests for self-signed cert generation
- `test/plugin/web-startup.ci` -- functional test: ze web starts and serves HTTPS
- `test/plugin/web-auth.ci` -- functional test: auth validation works end-to-end
- `test/plugin/web-json-response.ci` -- functional test: content negotiation returns correct types

## Implementation Steps

<!-- Steps must map to /implement stages. Each step should be a concrete phase of work,
     not a generic process description. The review checklists below are what /implement
     stages 5, 9, and 10 check against -- they MUST be filled with feature-specific items. -->

### /implement Stage Mapping

<!-- This table maps /implement stages to spec sections. Fill during design. -->
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan -- check what exists |
| 3. Implement (TDD) | Implementation phases below (write-test-fail-implement-pass per phase) |
| 4. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 5. Critical review | Critical Review Checklist below |
| 6. Fix issues | Fix every issue from critical review |
| 7. Re-verify | Re-run stage 4 |
| 8. Repeat 5-7 | Max 2 review passes |
| 9. Deliverables review | Deliverables Checklist below |
| 10. Security review | Security Review Checklist below |
| 11. Re-verify | Re-run stage 4 |
| 12. Present summary | Executive Summary Report per `rules/planning.md` |

### Implementation Phases

<!-- List concrete phases of work. Each phase follows TDD: write test -> fail -> implement -> pass.
     Phases should be ordered by dependency (e.g., schema before resolution, resolution before CLI). -->

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: YANG schema** -- create `ze-web-conf.yang` with `web` container and `listen` leaf, register via `init()` in `register.go`
   - Tests: verify YANG module loads without error
   - Files: `internal/component/web/schema/ze-web-conf.yang`, `internal/component/web/schema/register.go`
   - Verify: tests fail -> implement -> tests pass

2. **Phase: TLS and server** -- self-signed cert generation into zefs `meta/web/`, TLS config loading (self-signed or CLI-provided), HTTP mux creation, listen and serve
   - Tests: `TestSelfSignedCertGeneration`
   - Files: `internal/component/web/server.go`, `internal/component/web/server_test.go`
   - Verify: tests fail -> implement -> tests pass

3. **Phase: Auth middleware** -- login endpoint: POST credentials, validate via `AuthenticateUser()`, create session, return `Set-Cookie`. Middleware: validate session cookie, fallback to Basic Auth for JSON API. New login invalidates previous session. Security headers on all authenticated responses. No `WWW-Authenticate` header in 401 response
   - Tests: `TestSessionCookieValidation`, `TestSessionCreation`, `TestSessionInvalidation`, `TestLoginOverlayOnExpiredSession`, `TestBasicAuthForJSONAPI`, `TestSecurityHeaders`
   - Files: `internal/component/web/auth.go`, `internal/component/web/auth_test.go`
   - Verify: tests fail -> implement -> tests pass

4. **Phase: URL parsing and content negotiation** -- parse URL path using verb-first three-tier scheme (`/show/<path>`, `/monitor/<path>`, `/config/<verb>/<path>`, `/admin/<path>`), reject path traversal (`..`, null bytes, non-YANG characters) with 400, determine response content type from Accept header and `?format=json` parameter (unknown format values ignored)
   - Tests: `TestURLToPath`, `TestURLToPathRoot`, `TestURLVerbExtraction`, `TestURLShowMonitorPaths`, `TestURLAdminPath`, `TestPathTraversalRejected`, `TestContentNegotiationAcceptJSON`, `TestContentNegotiationFormatParam`, `TestContentNegotiationURLWins`, `TestContentNegotiationDefault`, `TestContentNegotiationUnknownFormat`
   - Files: `internal/component/web/handler.go`, `internal/component/web/handler_test.go`
   - Verify: tests fail -> implement -> tests pass

5. **Phase: Templates and rendering** -- create `layout.html` and `login.html` templates, embed via `go:embed`, load and execute templates, embed static assets (htmx.min.js, sse.js, style.css)
   - Tests: `TestTemplateLoading`
   - Files: `internal/component/web/render.go`, `internal/component/web/render_test.go`, `internal/component/web/templates/layout.html`, `internal/component/web/templates/login.html`, `internal/component/web/assets/htmx.min.js`, `internal/component/web/assets/sse.js`, `internal/component/web/assets/style.css`
   - Verify: tests fail -> implement -> tests pass

6. **Phase: CLI entry point** -- `cmd/ze/web/main.go` with `Run(args)`, flag parsing for `--cert`, `--key`, `--listen`, add `"web"` case to `cmd/ze/main.go` dispatch
   - Tests: CLI help output test
   - Files: `cmd/ze/web/main.go`, `cmd/ze/main.go`
   - Verify: tests fail -> implement -> tests pass

7. **Functional tests** -- create `.ci` tests for server startup, auth validation, and content negotiation
   - Tests: `test/plugin/web-startup.ci`, `test/plugin/web-auth.ci`, `test/plugin/web-json-response.ci`
   - Files: all `.ci` files listed above
   - Verify: functional tests pass end-to-end

8. **Full verification** -- `make ze-verify` (lint + all ze tests except fuzz)

9. **Complete spec** -- fill audit tables, write learned summary to `plan/learned/NNN-web-1-foundation.md`, delete spec from `plan/`. Summary is part of the commit.

### Critical Review Checklist (/implement stage 5)

<!-- MANDATORY: Fill with feature-specific checks. /implement uses this table
     to verify the implementation. Generic checks from rules/quality.md always apply;
     this table adds what's specific to THIS feature. -->
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1 through AC-17 has implementation with file:line |
| Correctness | 401 response has no `WWW-Authenticate` header; session cookie has `Secure; HttpOnly; SameSite=Strict` flags; content negotiation URL wins over Accept; security headers present on authenticated responses; path traversal rejected before schema walking |
| Naming | YANG module uses `ze-web-conf` naming convention; JSON keys use kebab-case |
| Data flow | Auth middleware always runs before handler; no path bypasses auth except login page assets |
| Rule: no-layering | No duplicate auth implementation -- reuses `AuthenticateUser()` only |
| Rule: cli-patterns | `ze web` follows `flag.NewFlagSet` pattern, errors to stderr, returns exit codes |
| Rule: design-principles | No premature abstraction -- no interfaces until 3+ implementations need one |
| Rule: goroutine-lifecycle | Server goroutine is long-lived (listener loop), not per-request |

### Deliverables Checklist (/implement stage 9)

<!-- MANDATORY: Every deliverable with a concrete verification method.
     /implement re-reads the spec and checks each item independently. -->
| Deliverable | Verification method |
|-------------|---------------------|
| `cmd/ze/web/main.go` exists with `Run` function | `grep "func Run" cmd/ze/web/main.go` |
| `"web"` case in `cmd/ze/main.go` dispatch | `grep '"web"' cmd/ze/main.go` |
| `internal/component/web/server.go` exists | `ls internal/component/web/server.go` |
| `internal/component/web/auth.go` exists with middleware | `grep "AuthenticateUser" internal/component/web/auth.go` |
| `internal/component/web/handler.go` exists with URL parsing | `grep "URLToPath\|ContentType\|contentNegotiation" internal/component/web/handler.go` |
| `internal/component/web/render.go` exists with template loading | `grep "go:embed\|template" internal/component/web/render.go` |
| YANG module registered | `grep "ze-web-conf" internal/component/web/schema/register.go` |
| `ze-web-conf.yang` has `listen` leaf | `grep "listen" internal/component/web/schema/ze-web-conf.yang` |
| `layout.html` has four areas | `grep "breadcrumb\|content\|notification\|cli-bar" internal/component/web/templates/layout.html` |
| `login.html` has login form | `grep "login\|password\|POST" internal/component/web/templates/login.html` |
| Embedded assets exist | `ls internal/component/web/assets/htmx.min.js internal/component/web/assets/sse.js internal/component/web/assets/style.css` |
| Auth unit tests pass | `go test -run TestSession ./internal/component/web/...` |
| Handler unit tests pass | `go test -run TestURLToPath ./internal/component/web/...` |
| Content negotiation tests pass | `go test -run TestContentNegotiation ./internal/component/web/...` |
| Functional tests exist | `ls test/plugin/web-startup.ci test/plugin/web-auth.ci test/plugin/web-json-response.ci` |
| `make ze-verify` passes | Run and capture output |

### Security Review Checklist (/implement stage 10)

<!-- MANDATORY: Feature-specific security concerns. /implement checks each item.
     Think about: untrusted input, injection, resource exhaustion, error leakage. -->
| Check | What to look for |
|-------|-----------------|
| TLS always enforced | No plaintext HTTP listener; server only binds with TLS config |
| Auth on every request | Middleware validates session cookie before all handlers; JSON API fallback to Basic Auth. No routes bypass auth except login endpoint and static login assets |
| No `WWW-Authenticate` header | 401 responses return login page HTML, never trigger browser Basic Auth popup |
| Password not logged | Auth middleware never logs password values; only log success/failure |
| Session cookie security | Cookie flags: `Secure`, `HttpOnly`, `SameSite=Strict`. New login invalidates old session |
| Template auto-escaping | All templates use `html/template` (not `text/template`) to prevent XSS |
| CSRF protection | `SameSite=Strict` cookie prevents cross-origin request attachment. Mutation endpoints under `/admin/` and `/config/` use POST |
| Security headers | `X-Frame-Options: DENY`, `X-Content-Type-Options: nosniff`, `Content-Security-Policy: default-src 'self'; script-src 'self'`, `Strict-Transport-Security: max-age=31536000`, `Cache-Control: no-store` on authenticated pages |
| Self-signed cert strength | ECDSA P-256 key via crypto/rand; SAN for localhost + listen address; private key 0600 permissions; certificate validity reasonable (1 year) |
| TLS failure handling | Self-signed cert generation failure exits with code 1. Unreadable or invalid `--cert`/`--key` files exit with code 2. Errors printed to stderr |
| Listen address validation | Reject invalid listen addresses before binding |
| URL path traversal | Path segments containing `..`, empty strings, null bytes, or non-YANG identifier characters rejected with 400 before schema walking |
| Asset integrity | Embedded assets are vendored at known versions, not fetched from CDN |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior -> RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural -> DESIGN phase |
| Functional test fails | Check AC; if AC wrong -> DESIGN; if AC correct -> IMPLEMENT |
| Audit finds missing AC | Back to relevant phase and implement |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

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
<!-- LIVE -- write IMMEDIATELY when you learn something -->
<!-- Route at completion: subsystem -> arch doc, process -> rules, knowledge -> memory.md -->

## RFC Documentation

Basic Auth (RFC 7617) is used only for JSON API consumers. Session-based auth (cookie) is primary for browser users. No BGP protocol-specific RFC references needed. If RFC 7617 compliance details arise during implementation, add `// RFC 7617 Section X.Y` comments above enforcing code.

## Implementation Summary

### What Was Implemented
- (to be filled during implementation)

### Bugs Found/Fixed
- (to be filled during implementation)

### Documentation Updates
- (to be filled during implementation)

### Deviations from Plan
- (to be filled during implementation)

## Implementation Audit

<!-- BLOCKING: Complete BEFORE writing learned summary. See rules/implementation-audit.md -->

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|

### Files from Plan
| File | Status | Notes |
|------|--------|-------|

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

## Pre-Commit Verification

<!-- BLOCKING: Do NOT trust the audit above. Re-verify everything independently. -->
<!-- For each item: run a command (grep, ls, go test -run) and paste the evidence. -->
<!-- Hook pre-commit-spec-audit.sh (exit 2) checks this section exists and is filled. -->

### Files Exist (ls)
<!-- For EVERY file in "Files to Create": ls -la <path> -- paste output. -->
<!-- For EVERY .ci file in Wiring Test and Functional Tests: ls -la <path> -- paste output. -->
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
<!-- For EVERY AC-N: independently verify. Do NOT copy from audit -- re-check. -->
<!-- Acceptable evidence: test name + pass output, grep showing function call, ls showing file. -->
<!-- NOT acceptable: "already checked", "should work", reference to audit table above. -->
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
<!-- For EVERY wiring test row: does the .ci test exist AND does it exercise the full path? -->
<!-- Read the .ci file content. Does it actually test what the wiring table claims? -->
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1 through AC-17 all demonstrated
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes (all 6 checks in `rules/quality.md` -- no failures)

### Quality Gates (SHOULD pass -- defer with user approval)
- [ ] RFC constraint comments added
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
- [ ] Critical Review passes -- all 6 checks in `rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `plan/learned/NNN-web-1-foundation.md`
- [ ] **Summary included in commit** -- NEVER commit implementation without the completed summary. One commit = code + tests + summary.
