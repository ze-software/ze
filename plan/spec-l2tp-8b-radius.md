# Spec: l2tp-8b -- RADIUS Client and L2TP Auth/Acct Plugin

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | spec-l2tp-8a-auth-pool |
| Phase | - |
| Updated | 2026-04-21 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `internal/component/l2tp/handler.go` -- AuthHandler, AuthResult types
4. `internal/component/l2tp/drain.go` -- startAuthDrain, Handled sentinel
5. `internal/component/ppp/auth_events.go` -- EventAuthRequest, AuthMethod enum
6. `internal/component/ppp/manager.go` -- Driver.AuthResponse threading
7. `internal/component/aaa/aaa.go` -- AAA backend interfaces
8. `internal/component/tacacs/client.go` -- TACACS+ client precedent
9. `internal/component/l2tp/events/events.go` -- SessionDown event handle

## Task

Implement a RADIUS client library and an L2TP auth/accounting plugin for ze.

The RADIUS client library (`internal/component/radius/`) implements
RFC 2865 (authentication) and RFC 2866 (accounting) wire format: packet
encoding/decoding, UDP transport with retransmit and exponential
backoff, server failover, authenticator computation, and a standard
attribute dictionary. It follows ze's buffer-first encoding discipline.

The L2TP auth plugin (`internal/plugins/l2tpauthradius/`) bridges
RADIUS to the PPP auth handler registered via `l2tp.RegisterAuthHandler`.
It maps PAP/CHAP-MD5/MS-CHAPv2 credentials from `EventAuthRequest` to
RADIUS Access-Request packets, processes Access-Accept/Reject responses,
and extracts session attributes (Framed-IP-Address, Framed-Pool,
Session-Timeout, Filter-Id). For BNG scale, the handler spawns a
goroutine per request and calls `Driver.AuthResponse` directly, using
the `AuthResult.Handled` sentinel to tell the drain goroutine to skip
its own response.

RADIUS accounting (RFC 2866) sends Start/Interim-Update/Stop records on
session lifecycle events via EventBus subscription. Interim-Update
is periodic (configurable interval, default 300s).

RADIUS for SSH/web login (AAA backend registration in
`internal/component/aaa/`) is included: the same client serves both
L2TP PPP auth and AAA `Authenticator` for SSH/web sessions.

**Out of scope (deferred to spec-l2tp-8b2-coa):**
- CoA/DM listener (RFC 5176): server-side UDP 3799 for live session changes
- RADIUS-directed pool selection: pool plugin changes for Framed-Pool attribute routing

### Design Decisions (agreed with user)

| Decision | Detail |
|----------|--------|
| Goroutine per RADIUS request | Handler spawns goroutine, calls Driver.AuthResponse directly. AuthResult.Handled sentinel skips drain's response. Scales to thousands of concurrent sessions. |
| Custom RADIUS client, no third-party dep | RFC 2865/2866 is simple UDP + TLV. Custom implementation follows buffer-first encoding. Avoids dependency on layeh.com/radius. |
| AAA backend included | Same RADIUS client serves both L2TP PPP auth and SSH/web login. Register via `aaa.Default.Register()`. |
| Accounting in same spec | Trivial once client exists. Start/Interim/Stop via EventBus. Interim default 300s. |
| CoA/DM deferred | Architecturally different (server, not client). Separate spec. |
| Auth + Acct share one UDP socket per server | Match TACACS+ precedent. Per-server mutex for I/O. Server failover in order. |
| 8a AuthResult.Handled field | Minor addition to 8a: `Handled bool` in AuthResult. Drain skips AuthResponse when true. Backwards compatible (zero value = false). |
| Single auth handler, no chain | AuthHandler registry is single-handler. When RADIUS is configured, it replaces local auth. When RADIUS is absent (no config), local auth remains active. No fallback chain: RADIUS unreachable = session rejected. Operators who want fallback configure both RADIUS and local auth on the RADIUS server itself. |
| SessionIPAssigned EventBus payload | Carries TunnelID, SessionID, Family, PeerAddr, Username. Username available from sess.username in reactor (line 961 of reactor.go). Emitted right after routeObserver.OnSessionIPUp. |
| Minimal attribute dictionary | Only attributes needed for PPP auth + accounting. Not a general-purpose RADIUS library. Extensible via type codes, not a full dictionary file. |
| Mock RADIUS server in tests | Lightweight UDP responder in test helper (internal to test package). Receives Access-Request, verifies authenticator, returns canned Access-Accept/Reject. Runs on localhost ephemeral port. |

## Required Reading

### Architecture Docs
- [ ] `docs/research/l2tpv2-ze-integration.md` -- section 6: plugin design
  -> Decision: auth/pool are plugins, not hardcoded; EventBus for cross-component notification
  -> Constraint: plugins register via init(); ConfigureEventBus for EventBus access
- [ ] `ai/patterns/plugin.md` -- plugin file structure
  -> Constraint: register.go with init(), atomic logger, RunXxxPlugin(conn), CLIHandler closure
  -> Constraint: plugin name hyphen-form (l2tp-auth-radius); log/env dot-form (l2tp.auth.radius)
- [ ] `ai/rules/buffer-first.md` -- wire encoding patterns
  -> Constraint: no append(), no make() in encoding helpers
- [ ] `ai/rules/goroutine-lifecycle.md` -- concurrency patterns
  -> Constraint: goroutines must have clear ownership and shutdown path
- [ ] `docs/architecture/core-design.md` -- subsystem and plugin patterns
  -> Constraint: subsystem implements ze.Subsystem; plugins discovered via registry

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc2865.md` -- RADIUS Authentication (CREATE if missing)
  -> Constraint: authenticator = MD5(Code+ID+Length+RequestAuth+Attributes+Secret)
  -> Constraint: User-Password encoding = XOR with MD5(secret+request_authenticator) chains
  -> Constraint: retransmit: same ID + same authenticator = same request
- [ ] `rfc/short/rfc2866.md` -- RADIUS Accounting (CREATE if missing)
  -> Constraint: Acct-Status-Type: Start(1), Stop(2), Interim-Update(3)
  -> Constraint: Acct-Session-Id must be unique per session
- [ ] `rfc/short/rfc2759.md` -- MS-CHAPv2
  -> Constraint: RADIUS carries MS-CHAP2-Response (vendor-specific attribute 311:25)

**Key insights:**
- RADIUS is a simple request-response UDP protocol with 16-byte authenticator, TLV attributes, and MD5-based security. Packet max 4096 bytes.
- The l2tp-auth-local handler is the reference implementation; RADIUS follows the same registration pattern but spawns goroutines for async I/O.
- The AAA registry (`internal/component/aaa/`) already supports backend chaining with priority. RADIUS at priority 50 (before TACACS+ at 100, before local at 200).
- TACACS+ client (`internal/component/tacacs/client.go`) is the closest precedent: per-server connection, mutex-serialized I/O, buffer pool, failover.
- Driver.AuthResponse is thread-safe and non-blocking (buffered(1) channel). Can be called from RADIUS goroutine safely.
- PPP auth-timeout (default 30s via ze.l2tp.auth.timeout) bounds the RADIUS query. If RADIUS is slower, the session tears down.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/l2tp/handler.go` -- AuthHandler/AuthResult/PoolHandler types
  -> Constraint: AuthHandler is synchronous func(EventAuthRequest) AuthResult; need Handled sentinel for async
- [ ] `internal/component/l2tp/handler_registry.go` -- RegisterAuthHandler/GetAuthHandler
  -> Constraint: single global handler; RADIUS replaces local auth (or chains somehow)
  -> Decision: RADIUS takes priority; if RADIUS rejects, session fails (no fallback to local)
- [ ] `internal/component/l2tp/drain.go` -- startAuthDrain/callAuthHandler
  -> Constraint: drain calls AuthResponse after handler returns; Handled sentinel skips this
- [ ] `internal/component/ppp/auth_events.go` -- EventAuthRequest struct fields
  -> Constraint: Method (PAP/CHAP-MD5/MSCHAPv2), Identifier, Username, Challenge, Response
  -> Constraint: PAP: Response is cleartext; CHAP-MD5: Challenge 16B + Response 16B; MSCHAPv2: Challenge 16B + Response 40B
- [ ] `internal/component/ppp/manager.go` -- AuthResponse threading
  -> Constraint: thread-safe, non-blocking, buffered(1); ErrAuthResponsePending on duplicate; ErrSessionNotFound on teardown race
- [ ] `internal/component/aaa/aaa.go` -- Authenticator/Authorizer/Accountant interfaces
  -> Constraint: Authenticator.Authenticate(username, password string) (AuthResult, error)
  -> Constraint: Backend.Build(BuildParams) (Contribution, error); BackendRegistry freezes after first Build
- [ ] `internal/component/aaa/all/all.go` -- current backends: authz, tacacs
  -> Decision: add radius blank import here
- [ ] `internal/component/tacacs/client.go` -- TACACS+ client architecture
  -> Constraint: per-server TCP, mutex I/O, buffer pool, server failover in order
  -> Decision: RADIUS follows same pattern but over UDP (connectionless, but still per-server I/O mutex for retransmit state)
- [ ] `internal/component/l2tp/events/events.go` -- SessionDown handle
  -> Constraint: SessionDown emitted by reactor on PPP session teardown; used by pool for IP release
  -> Decision: RADIUS acct subscribes to SessionDown for Accounting-Stop
- [ ] `internal/plugins/l2tpauthlocal/register.go` -- local auth plugin structure
  -> Constraint: init() calls RegisterAuthHandler + registry.Register; RunPlugin with config lifecycle

**Behavior to preserve:**
- Existing l2tp-auth-local plugin continues to work if no RADIUS is configured
- AuthHandler registry remains single-handler (one active at a time)
- PPP Driver channel semantics unchanged
- All existing L2TP and PPP tests pass unchanged
- AAA backend chain order (authz, tacacs) unchanged when RADIUS is absent

**Behavior to change:**
- Add `Handled bool` to `AuthResult` (8a change); drain skips AuthResponse when Handled=true
- RADIUS handler replaces local auth handler when configured (init() registration order)
- Add `SessionIPAssigned` typed event handle to `l2tp/events/events.go`
- Emit `(l2tp, session-ip-assigned)` from reactor in `handleSessionIPAssigned` (same callsite as routeObserver.OnSessionIPUp)
- RADIUS acct subscribes to `(l2tp, session-ip-assigned)` for Accounting-Start
- RADIUS acct subscribes to `(l2tp, session-down)` for Accounting-Stop
- Add RADIUS to AAA backend chain (priority 50, before TACACS+ at 100)

## Data Flow (MANDATORY)

### Entry Point
- PPP session goroutine emits `EventAuthRequest` on `ppp.Driver.authEventsOut` after LCP completes
- YANG config tree delivers RADIUS server list, shared secrets, timeouts during plugin configure phase
- EventBus emits `(l2tp, session-down)` when session tears down (accounting stop trigger)

### Transformation Path

#### Authentication flow:
1. PPP session completes LCP, enters auth phase
2. Session emits `EventAuthRequest{Method, Username, Challenge, Response}` on `authEventsOut`
3. Auth drain goroutine reads event, calls `callAuthHandler(logger, handler, req)`
4. RADIUS handler spawns goroutine: build Access-Request, send to RADIUS server
5. RADIUS client encodes packet (Code=1, ID, Authenticator, Attributes), sends UDP
6. Client waits for response with retransmit (exponential backoff, max 3 retries)
7. On Access-Accept: extract attributes (Framed-IP, Framed-Pool, Session-Timeout, Filter-Id)
8. Goroutine calls `driver.AuthResponse(tid, sid, true, "", mschapv2Blob)`
9. Handler returns `AuthResult{Handled: true}` to drain; drain skips its own AuthResponse call
10. PPP session proceeds to NCP phase

#### Accounting flow:
1. PPP NCP completes, emits `EventSessionIPAssigned` on lifecycle channel
2. Reactor handles it: calls routeObserver.OnSessionIPUp AND emits `(l2tp, session-ip-assigned)` on EventBus
3. RADIUS acct plugin receives `(l2tp, session-ip-assigned)` via EventBus subscription
4. Plugin sends Accounting-Request (Status-Type=Start) with session attributes (IP, username, NAS-IP)
5. Plugin starts per-session interim timer (configurable interval, default 300s)
6. Timer fires: sends Accounting-Request (Status-Type=Interim-Update) with counters
7. On `(l2tp, session-down)`: cancel interim timer, send Accounting-Request (Status-Type=Stop) with final duration and counters

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| PPP session -> auth drain | authEventsOut channel (buffered 64) | [ ] |
| Auth drain -> RADIUS handler | Direct function call (registered at init) | [ ] |
| RADIUS handler -> RADIUS goroutine | go func() with captured driver reference | [ ] |
| RADIUS goroutine -> RADIUS server | UDP packet (RFC 2865 wire format) | [ ] |
| RADIUS server -> RADIUS goroutine | UDP response packet | [ ] |
| RADIUS goroutine -> PPP session | Driver.AuthResponse -> authRespCh (buffered 1) | [ ] |
| EventBus -> RADIUS acct | (l2tp, session-down) subscription | [ ] |
| RADIUS acct -> RADIUS server | UDP Accounting-Request | [ ] |

### Integration Points
- `l2tp.RegisterAuthHandler()` -- RADIUS auth handler registration
- `ppp.Driver.AuthResponse()` -- async response from RADIUS goroutine
- `aaa.Default.Register()` -- RADIUS as AAA backend
- `l2tpevents.SessionDown` -- accounting stop trigger
- `registry.Register()` -- standard plugin registration
- `yang.RegisterModule()` -- YANG schema registration
- `env.MustRegister()` -- env var registration (ze.log.l2tp.auth.radius)

### Architectural Verification
- [ ] No bypassed layers (RADIUS goes through handler registry, not direct import)
- [ ] No unintended coupling (plugin uses handler registry and EventBus, not l2tp internals)
- [ ] No duplicated functionality (reuses existing drain, handler, EventBus infrastructure)
- [ ] Zero-copy preserved where applicable (buffer-first RADIUS encoding)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| PPP emits EventAuthRequest (PAP) | -> | RADIUS handler queries mock server, returns accept | `TestRADIUSAuthPAPAccept` |
| PPP emits EventAuthRequest (CHAP-MD5) | -> | RADIUS handler queries mock server, returns accept | `TestRADIUSAuthCHAPAccept` |
| RADIUS Access-Reject received | -> | Handler calls AuthResponse(reject) | `TestRADIUSAuthReject` |
| RADIUS server unreachable (all retries fail) | -> | Handler calls AuthResponse(reject) after timeout | `TestRADIUSAuthTimeout` |
| Config with RADIUS server | -> | Plugin parses config, creates RADIUS client | `test/l2tp/auth-radius-basic.ci` |
| Session down after RADIUS auth | -> | Accounting-Stop sent to RADIUS server | `TestRADIUSAcctStop` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | PAP auth with valid credentials, RADIUS Access-Accept | Session accepted; Framed-IP-Address extracted if present |
| AC-2 | CHAP-MD5 auth, RADIUS Access-Accept | Session accepted; CHAP-Password attribute sent correctly |
| AC-3 | MS-CHAPv2 auth, RADIUS Access-Accept | Session accepted; MS-CHAP2-Response vendor attribute sent; Authenticator-Response extracted for mutual auth |
| AC-4 | RADIUS Access-Reject | Session rejected with reason from Reply-Message |
| AC-5 | All RADIUS servers unreachable (retransmit exhausted) | Session rejected after timeout; error logged |
| AC-6 | RADIUS server failover | First server timeout, second server responds; session accepted |
| AC-7 | RADIUS Access-Accept with Framed-Pool attribute | Pool name extracted and stored in session attributes |
| AC-8 | RADIUS Access-Accept with Session-Timeout | Session timeout value stored in session attributes |
| AC-9 | Accounting-Start sent on session establishment | RADIUS Accounting-Request with Status-Type=Start, correct session attributes |
| AC-10 | Accounting-Stop sent on session teardown | RADIUS Accounting-Request with Status-Type=Stop, session duration and byte counters |
| AC-11 | Accounting-Interim-Update sent periodically | Timer fires at configured interval; Accounting-Request with Status-Type=Interim-Update |
| AC-12 | Plugin appears in `make ze-inventory` | l2tp-auth-radius listed with correct name |
| AC-13 | AAA backend registered | RADIUS backend appears in aaa.Default backends list |
| AC-14 | RADIUS shared secret incorrect | Authenticator verification fails; response discarded; retransmit until timeout |
| AC-15 | Concurrent auth requests (10+) | All handled in parallel goroutines; no blocking of drain channel |
| AC-16 | AuthResult.Handled sentinel | Drain goroutine skips AuthResponse when handler returns Handled=true |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestEncodeAccessRequest` | `internal/component/radius/packet_test.go` | Access-Request encoding with authenticator | |
| `TestDecodeAccessAccept` | `internal/component/radius/packet_test.go` | Access-Accept decoding and authenticator verify | |
| `TestDecodeAccessReject` | `internal/component/radius/packet_test.go` | Access-Reject decoding | |
| `TestEncodeUserPassword` | `internal/component/radius/attr_test.go` | User-Password XOR encoding (RFC 2865 S5.2) | |
| `TestEncodeCHAPPassword` | `internal/component/radius/attr_test.go` | CHAP-Password attribute encoding | |
| `TestEncodeMSCHAPv2Response` | `internal/component/radius/attr_test.go` | MS-CHAP2-Response vendor attribute (26/311:25) | |
| `TestPacketRoundTrip` | `internal/component/radius/packet_test.go` | Encode then decode preserves all fields | |
| `TestClientRetransmit` | `internal/component/radius/client_test.go` | Retransmit after timeout, same ID and authenticator | |
| `TestClientFailover` | `internal/component/radius/client_test.go` | First server timeout, try second server | |
| `TestClientAuthenticatorVerify` | `internal/component/radius/client_test.go` | Bad response authenticator discarded | |
| `TestRADIUSAuthPAPAccept` | `internal/plugins/l2tpauthradius/handler_test.go` | PAP -> Access-Request -> Accept -> AuthResponse(true) | |
| `TestRADIUSAuthPAPReject` | `internal/plugins/l2tpauthradius/handler_test.go` | PAP -> Access-Request -> Reject -> AuthResponse(false) | |
| `TestRADIUSAuthCHAPAccept` | `internal/plugins/l2tpauthradius/handler_test.go` | CHAP-MD5 -> Access-Request with CHAP attrs -> Accept | |
| `TestRADIUSAuthMSCHAPv2Accept` | `internal/plugins/l2tpauthradius/handler_test.go` | MS-CHAPv2 -> vendor attrs -> Accept with auth blob | |
| `TestRADIUSAuthTimeout` | `internal/plugins/l2tpauthradius/handler_test.go` | No response -> reject after retransmit exhaustion | |
| `TestRADIUSHandledSentinel` | `internal/component/l2tp/drain_test.go` | Handler returns Handled=true; drain does not call AuthResponse | |
| `TestRADIUSAcctStart` | `internal/plugins/l2tpauthradius/acct_test.go` | Session-up -> Accounting-Start with correct attributes | |
| `TestRADIUSAcctStop` | `internal/plugins/l2tpauthradius/acct_test.go` | Session-down -> Accounting-Stop with duration and counters | |
| `TestRADIUSAcctInterim` | `internal/plugins/l2tpauthradius/acct_test.go` | Timer -> Accounting-Interim-Update | |
| `TestRADIUSAAABackend` | `internal/component/radius/aaa_test.go` | AAA Authenticator wraps RADIUS client correctly | |
| `TestRADIUSConfigParse` | `internal/plugins/l2tpauthradius/config_test.go` | YANG config -> server list, secrets, timeouts | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| RADIUS server port | 1-65535 | 65535 | 0 | 65536 |
| RADIUS retransmit count | 1-10 | 10 | 0 | 11 |
| RADIUS timeout (seconds) | 1-30 | 30 | 0 | 31 |
| Acct interim interval (seconds) | 60-3600 | 3600 | 59 | 3601 |
| Packet length | 20-4096 | 4096 | 19 | 4097 |
| Attribute length | 3-255 | 255 | 2 | N/A (uint8) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `auth-radius-basic` | `test/l2tp/auth-radius-basic.ci` | L2TP session with PAP auth against mock RADIUS | |
| `auth-radius-chap` | `test/l2tp/auth-radius-chap.ci` | L2TP session with CHAP-MD5 auth against mock RADIUS | |
| `auth-radius-reject` | `test/l2tp/auth-radius-reject.ci` | RADIUS rejects credentials; session torn down | |
| `auth-radius-failover` | `test/l2tp/auth-radius-failover.ci` | First RADIUS server down; failover to second | |

### Future (if deferring any tests)
- MS-CHAPv2 functional test: deferred because fakel2tp peer does not implement MS-CHAPv2 wire format; unit tests cover the attribute encoding
- CoA/DM tests: deferred to spec-l2tp-8b2-coa
- RADIUS-directed pool selection test: deferred until pool plugin wires Framed-Pool extraction

## Files to Modify

- `internal/component/l2tp/handler.go` -- add `Handled bool` to AuthResult
- `internal/component/l2tp/drain.go` -- skip AuthResponse when Handled=true in startAuthDrain
- `internal/component/l2tp/drain_test.go` -- test Handled sentinel behavior
- `internal/component/l2tp/events/events.go` -- add `SessionIPAssigned` typed event handle + payload struct
- `internal/component/l2tp/reactor.go` -- emit `(l2tp, session-ip-assigned)` in `handleSessionIPAssigned`
- `internal/component/aaa/all/all.go` -- add blank import for radius backend
- `internal/component/plugin/registry/registry_test.go` -- update expected plugin count (+1)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new) | [x] | `internal/plugins/l2tpauthradius/schema/ze-l2tp-auth-radius-conf.yang` |
| CLI commands | [ ] | None for 8b (future: `show radius statistics`) |
| Editor autocomplete | [x] | YANG-driven (automatic) |
| Functional test for new RPC/API | [x] | `test/l2tp/auth-radius-*.ci` |
| Plugin blank imports | [x] | `internal/component/plugin/all/all.go` (via `make generate`) |
| AAA backend blank import | [x] | `internal/component/aaa/all/all.go` |
| Env var registration | [x] | `ze.log.l2tp.auth.radius` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [x] | `docs/features.md` -- add RADIUS authentication |
| 2 | Config syntax changed? | [x] | `docs/guide/configuration.md` -- add RADIUS server config examples |
| 3 | CLI command added/changed? | [ ] | |
| 4 | API/RPC added/changed? | [ ] | |
| 5 | Plugin added/changed? | [x] | `docs/guide/plugins.md` -- add l2tp-auth-radius |
| 6 | Has a user guide page? | [ ] | Covered in existing `docs/guide/l2tp.md` |
| 7 | Wire format changed? | [ ] | |
| 8 | Plugin SDK/protocol changed? | [ ] | |
| 9 | RFC behavior implemented? | [x] | `rfc/short/rfc2865.md`, `rfc/short/rfc2866.md` (CREATE) |
| 10 | Test infrastructure changed? | [ ] | |
| 11 | Affects daemon comparison? | [x] | `docs/comparison.md` -- RADIUS column |
| 12 | Internal architecture changed? | [ ] | |

## Files to Create

### RADIUS client library
- `internal/component/radius/doc.go` -- package doc
- `internal/component/radius/packet.go` -- RADIUS packet encode/decode (buffer-first)
- `internal/component/radius/packet_test.go` -- packet round-trip and boundary tests
- `internal/component/radius/attr.go` -- attribute types, encode/decode, dictionary
- `internal/component/radius/attr_test.go` -- attribute encoding tests
- `internal/component/radius/client.go` -- UDP client with retransmit, failover, authenticator
- `internal/component/radius/client_test.go` -- retransmit, failover, authenticator verify tests
- `internal/component/radius/dict.go` -- standard attribute dictionary (type codes, names)
- `internal/component/radius/pool.go` -- buffer pool for RADIUS packets (sync.Pool, 4096 bytes)
- `internal/component/radius/aaa.go` -- AAA backend implementation (Authenticator for SSH/web)
- `internal/component/radius/aaa_test.go` -- AAA backend tests
- `internal/component/radius/register.go` -- aaa.Default.Register() in init()

### L2TP auth RADIUS plugin
- `internal/plugins/l2tpauthradius/l2tpauthradius.go` -- atomic logger, Name constant
- `internal/plugins/l2tpauthradius/register.go` -- init(), plugin registration, config lifecycle
- `internal/plugins/l2tpauthradius/handler.go` -- RADIUS AuthHandler (goroutine-per-request)
- `internal/plugins/l2tpauthradius/handler_test.go` -- handler tests with mock RADIUS server
- `internal/plugins/l2tpauthradius/acct.go` -- accounting Start/Interim/Stop via EventBus
- `internal/plugins/l2tpauthradius/acct_test.go` -- accounting tests
- `internal/plugins/l2tpauthradius/config.go` -- YANG config parsing (server list, secrets)
- `internal/plugins/l2tpauthradius/config_test.go` -- config parsing tests
- `internal/plugins/l2tpauthradius/schema/embed.go` -- go:embed
- `internal/plugins/l2tpauthradius/schema/register.go` -- yang.RegisterModule
- `internal/plugins/l2tpauthradius/schema/ze-l2tp-auth-radius-conf.yang` -- YANG schema

### RFC summaries
- `rfc/short/rfc2865.md` -- RADIUS Authentication summary
- `rfc/short/rfc2866.md` -- RADIUS Accounting summary

### Functional tests
- `test/l2tp/auth-radius-basic.ci` -- PAP auth via RADIUS
- `test/l2tp/auth-radius-chap.ci` -- CHAP-MD5 auth via RADIUS
- `test/l2tp/auth-radius-reject.ci` -- RADIUS reject
- `test/l2tp/auth-radius-failover.ci` -- RADIUS server failover

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file + umbrella |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Implement (TDD) | Implementation phases below |
| 4. /ze-review gate | Review Gate section |
| 5. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 6. Critical review | Critical Review Checklist below |
| 7. Fix issues | Fix every issue from critical review |
| 8. Re-verify | Re-run stage 5 |
| 9. Repeat 6-8 | Until clean |
| 10. Deliverables review | Deliverables Checklist below |
| 11. Security review | Security Review Checklist below |
| 12. Re-verify | Re-run stage 5 |
| 13. Present summary | Executive Summary Report |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: 8a AuthResult.Handled + SessionIPAssigned event** -- add Handled field to AuthResult; update drain to skip AuthResponse when true; add SessionIPAssigned typed event to events.go; emit from reactor in handleSessionIPAssigned
   - Tests: `TestRADIUSHandledSentinel`, `TestSessionIPAssignedEvent`
   - Files: `handler.go`, `drain.go`, `drain_test.go`, `events/events.go`, `reactor.go`
   - Verify: tests fail -> implement -> tests pass

2. **Phase: RADIUS wire format** -- packet encode/decode, attribute encode/decode (User-Password, CHAP-Password, vendor attrs), buffer pool
   - Tests: `TestEncodeAccessRequest`, `TestDecodeAccessAccept`, `TestDecodeAccessReject`, `TestEncodeUserPassword`, `TestEncodeCHAPPassword`, `TestEncodeMSCHAPv2Response`, `TestPacketRoundTrip`
   - Files: `packet.go`, `attr.go`, `dict.go`, `pool.go`, `*_test.go`
   - Verify: tests fail -> implement -> tests pass

3. **Phase: RADIUS client** -- UDP transport, retransmit, failover, authenticator verification
   - Tests: `TestClientRetransmit`, `TestClientFailover`, `TestClientAuthenticatorVerify`
   - Files: `client.go`, `client_test.go`
   - Verify: tests fail -> implement -> tests pass

4. **Phase: l2tp-auth-radius handler** -- goroutine-per-request handler, PAP/CHAP/MS-CHAPv2 attribute mapping, session attribute extraction
   - Tests: `TestRADIUSAuthPAPAccept`, `TestRADIUSAuthPAPReject`, `TestRADIUSAuthCHAPAccept`, `TestRADIUSAuthMSCHAPv2Accept`, `TestRADIUSAuthTimeout`
   - Files: `handler.go`, `handler_test.go`
   - Verify: tests fail -> implement -> tests pass

5. **Phase: RADIUS accounting** -- EventBus subscription, Start/Interim/Stop lifecycle
   - Tests: `TestRADIUSAcctStart`, `TestRADIUSAcctStop`, `TestRADIUSAcctInterim`
   - Files: `acct.go`, `acct_test.go`
   - Verify: tests fail -> implement -> tests pass

6. **Phase: Plugin wiring** -- register.go, YANG schema, config parsing, AAA backend, blank imports
   - Tests: `TestRADIUSConfigParse`, `TestRADIUSAAABackend`
   - Files: `register.go`, `config.go`, `schema/*`, `aaa.go`, `aaa_test.go`, `all/all.go`
   - Verify: `make generate` updates all.go; tests pass

7. **Phase: RFC summaries and functional tests** -- write rfc/short/rfc2865.md and rfc2866.md; create .ci tests
   - Tests: `auth-radius-basic.ci`, `auth-radius-chap.ci`, `auth-radius-reject.ci`, `auth-radius-failover.ci`
   - Files: `rfc/short/rfc2865.md`, `rfc/short/rfc2866.md`, `test/l2tp/auth-radius-*.ci`
   - Verify: functional tests pass

8. **Phase: Full verification and review** -- `make ze-verify`, critical review, deliverables, security
   - Verify: `make ze-verify` passes

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1 through AC-16 has implementation with file:line |
| Correctness | User-Password XOR encoding matches RFC 2865 S5.2 byte-for-byte; CHAP-Password = CHAP-Ident + CHAP-Response; MS-CHAP2-Response in vendor attr 311:25 |
| Naming | Plugin name `l2tp-auth-radius`; env `l2tp.auth.radius`; RADIUS client package `radius` |
| Data flow | Auth: EventAuthRequest -> handler -> goroutine -> RADIUS UDP -> Accept/Reject -> Driver.AuthResponse; no shortcut |
| Rule: buffer-first | RADIUS packet encoding uses pooled 4096-byte buffers, offset-based writes, no append |
| Rule: goroutine-lifecycle | RADIUS goroutines bounded by auth-timeout; accounting goroutine has clean shutdown via context |
| Panic safety | Handler goroutine recover() from panics; reject on panic |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| RADIUS client library exists | `ls internal/component/radius/*.go` |
| RADIUS packet encode/decode | `go test ./internal/component/radius/ -run TestPacketRoundTrip` |
| RADIUS retransmit works | `go test ./internal/component/radius/ -run TestClientRetransmit` |
| Plugin registered | `grep l2tp-auth-radius internal/plugins/l2tpauthradius/register.go` |
| Plugin in all.go | `grep l2tpauthradius internal/component/plugin/all/all.go` |
| AAA backend in all.go | `grep radius internal/component/aaa/all/all.go` |
| YANG schema exists | `ls internal/plugins/l2tpauthradius/schema/*.yang` |
| AuthResult.Handled works | `go test ./internal/component/l2tp/ -run TestHandledSentinel` |
| Accounting works | `go test ./internal/plugins/l2tpauthradius/ -run TestRADIUSAcct` |
| Functional tests exist | `ls test/l2tp/auth-radius-*.ci` |
| RFC summaries exist | `ls rfc/short/rfc2865.md rfc/short/rfc2866.md` |
| `make ze-verify` passes | Run and check exit code |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Shared secret handling | RADIUS shared secret not logged; YANG leaf marked ze:sensitive; env var uses env.Secret |
| User-Password encoding | XOR chain uses MD5(secret+authenticator); no plaintext password in logs or on wire |
| Response authenticator | Verify response authenticator before trusting Access-Accept; reject packets with bad authenticator |
| Retransmit ID reuse | Same Request Authenticator for retransmits of the same request (RFC 2865 S2.5) |
| Buffer bounds | All packet parsing validates length before reading; reject packets > 4096 bytes |
| UDP source validation | Only accept responses from the server IP:port we sent to |
| Timing attacks | Use subtle.ConstantTimeCompare for authenticator verification |
| Resource exhaustion | Bound concurrent RADIUS goroutines (e.g., semaphore matching channel buffer size) |
| Accounting reliability | Accounting failures logged but do not tear down sessions |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read RFC 2865/2866 and source |
| Lint failure | Fix inline |
| Functional test fails | Check AC; if AC wrong -> DESIGN; if AC correct -> IMPLEMENT |
| RADIUS encoding mismatch | Re-read RFC 2865 byte-for-byte; compare with pcap |
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

- RADIUS is architecturally simple (UDP request-response with TLV encoding) but has subtle security requirements: authenticator computation chains, User-Password XOR encoding, retransmit ID semantics. Getting the crypto right is the single biggest implementation risk.
- The goroutine-per-request model for RADIUS auth means the drain goroutine never blocks on network I/O. The Handled sentinel in AuthResult bridges the sync/async boundary cleanly.
- RADIUS accounting is fire-and-forget from the session's perspective: accounting failures must not tear down sessions. This is explicitly different from auth (where failures reject the session).
- The AAA backend integration means RADIUS serves two masters: L2TP PPP auth (binary credentials via EventAuthRequest) and SSH/web login auth (username+password via aaa.Authenticator). The client library is shared; the credential mapping is different.

## RFC Documentation

Add `// RFC 2865 Section X.Y: "<quoted requirement>"` above enforcing code.
Add `// RFC 2866 Section X.Y: "<quoted requirement>"` above accounting code.
MUST document: authenticator computation, User-Password encoding, CHAP-Password encoding, retransmit semantics, Acct-Status-Type values.

## Implementation Summary

### What Was Implemented
- (to be filled)

### Bugs Found/Fixed
- (to be filled)

### Documentation Updates
- (to be filled)

### Deviations from Plan
- (to be filled)

## Implementation Audit

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
- **Partial:**
- **Skipped:**
- **Changed:**

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied
- (to be filled)

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-16 all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)
- [ ] RFC constraint comments added (RFC 2865, RFC 2866)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL
- [ ] Tests PASS
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/`
- [ ] Summary included in commit
