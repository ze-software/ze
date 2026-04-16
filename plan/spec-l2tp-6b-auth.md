# Spec: l2tp-6b -- PPP Authentication

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | spec-l2tp-6a-lcp-base |
| Phase | 4/9 |
| Updated | 2026-04-16 |

## Scope Changes (2026-04-16)

**Proxy-authentication handling dropped.** Survey of every extant RFC 2661 LNS
implementation shows it is universally unimplemented:

| Implementation | Code path | Behavior |
|----------------|-----------|----------|
| accel-ppp | `accel-pppd/ctrl/l2tp/l2tp.c:3590-3594` | silent-accept `break` in ICCN switch |
| mpd5 (FreeBSD) | `src/l2tp_avp.c:714-737` | decoded into struct, only reads are cleanup + debug print |
| xl2tpd | `avp.c:54-58` + `avp.c:383-388` | `ignore_avp`; comment labels proxy auth "a ridiculous security threat" |

Shipping a `trust-proxy=true` knob nobody asks for adds a credential-forwarding
surface with no offsetting benefit. ze matches the accel-ppp stance: always
re-authenticate over PPP. `L2TPSession.proxyAuthen*` fields remain populated
(parsing is already cheap, they are useful for logs), but PPP consumes none of
them. Deferral recorded in `plan/deferrals.md` with Destination
`user-approved-drop`.

**Phase count restructured** from 13 to 9. Sections below referencing proxy auth
(scope table, AC-11/12, wiring-test row, TDD tests, phase 7, Files to Modify
`proxy.go` extension, Security Review `trust-proxy` row) are superseded by this
note; they will be struck through as each phase is implemented.

**Phase 3 scope (2026-04-16).** Phase 3 ships env-var registration and reactor
plumbing only. `ze.l2tp.auth.timeout` is registered in
`internal/component/config/environment.go` (duration, default 30s) and the
L2TP reactor now reads it in `handleKernelSuccess` and fills
`ppp.StartSession.AuthTimeout` on every dispatch. The matching YANG leaf and
the other two env vars (`ze.l2tp.auth.reauth-interval`, `ze.l2tp.auth.methods`)
are deferred per YAGNI -- they have no consumer until Phase 7 (auth state
machine) and spec-l2tp-7-subsystem (YANG wiring), respectively. The Integration
Checklist row "auth knobs come via env vars; YANG wiring in
spec-l2tp-7-subsystem" stands.

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `plan/spec-l2tp-6a-lcp-base.md` -- LCP foundation, auth-phase hook stub
3. `plan/spec-l2tp-0-umbrella.md` -- umbrella context
4. `rfc/short/rfc1334.md` -- PAP (create if missing)
5. `rfc/short/rfc1994.md` -- CHAP-MD5
6. `rfc/short/rfc2759.md` -- MS-CHAPv2 (create if missing)
7. `rfc/short/rfc2661.md` Section 18 -- proxy authentication

## Task

Replace the 6a stub `AuthHook` with a real PPP authentication layer that
runs PAP (RFC 1334), CHAP-MD5 (RFC 1994), and MS-CHAPv2 (RFC 2759), plus
proxy authentication (RFC 2661 Section 18). Authentication decisions are
delegated to subscribers of `EventAuthRequest` on the manager events
channel; ze handles only wire format and protocol state. The actual
RADIUS query lives in spec-l2tp-8-plugins (l2tp-auth plugin).

| Capability | In Scope |
|------------|----------|
| PAP wire format | yes -- Authenticate-Request, Authenticate-Ack, Authenticate-Nak |
| CHAP-MD5 wire format | yes -- Challenge, Response, Success, Failure |
| MS-CHAPv2 wire format | yes -- Challenge, Response (peer-challenge + NT-response), Success, Failure |
| Proxy authentication | yes -- RFC 2661 Section 18 short-circuit when ICCN AVPs populated |
| `EventAuthRequest` / `EventAuthResponse` flow | yes -- channel-based request/reply between PPP and external auth handler (l2tp-auth plugin in Phase 8; in-test stub in 6b) |
| Auth method advertised in LCP CONFREQ | yes -- update LCP options to advertise PAP / CHAP-MD5 / MS-CHAPv2 based on config |
| Auth method retry / fallback | yes -- if peer NAKs the auth method, fall back per RFC 1661 negotiation |
| Periodic re-auth (CHAP) | yes -- optional, configurable interval |
| RADIUS roundtrip | NO -- spec-l2tp-8-plugins (l2tp-auth) |
| MPPE key derivation | NO -- CCP is umbrella out-of-scope |
| EAP | NO -- not in umbrella scope |

## Required Reading

### Architecture Docs

- [ ] `plan/spec-l2tp-6a-lcp-base.md` -- the `AuthHook` interface point
  -> Constraint: 6b implements `AuthHook`; the 6a stub is replaced, not extended; `rules/no-layering.md`
- [ ] `internal/component/ppp/manager.go` (after 6a) -- events channel shape
  -> Constraint: `EventAuthRequest` carries auth-method, identifier, payload bytes; `EventAuthResponse` written back via `Manager.AuthResponse(sessionID, accept, message)`
- [ ] `.claude/rules/buffer-first.md`
  -> Constraint: PAP/CHAP/MS-CHAPv2 packet encoding via offset writes
- [ ] `docs/research/l2tpv2-implementation-guide.md` Section 18 (Proxy LCP and Proxy Authentication)
  -> Constraint: when `Proxy-Authen-Type` (AVP 29), `-Name` (30), `-Challenge` (31), `-ID` (32), `-Response` (33) are present in ICCN, LNS may skip auth and use proxied credentials as already-validated; OR re-issue auth (config choice)

### RFC Summaries (MUST for protocol work)

- [ ] `rfc/short/rfc1334.md` -- PAP (CREATE if missing)
  -> Constraint: 3 packet types (codes 1/2/3); username + password sent CLEARTEXT; identifier echoes between request and ack/nak
- [ ] `rfc/short/rfc1994.md` -- CHAP
  -> Constraint: 4 packet types (codes 1/2/3/4); challenge value is opaque to PPP; response = MD5(identifier || secret || challenge); identifier MUST change between successive challenges to prevent replay
- [ ] `rfc/short/rfc2759.md` -- MS-CHAPv2 (CREATE if missing)
  -> Constraint: response carries Peer-Challenge (16) + Reserved (8) + NT-Response (24) + Flags (1); Success packet contains Authenticator-Response derived from NT-hash; ze SHOULD pass Authenticator-Response from RADIUS verbatim
- [ ] `rfc/short/rfc2661.md` Section 18 -- proxy authentication
  -> Constraint: proxy auth is informational; LNS decides whether to trust based on config (`l2tp { auth { trust-proxy true; } }`)

**Key insights:**
- ze never sees plaintext passwords for CHAP/MS-CHAPv2; ze passes challenge + response to RADIUS, RADIUS validates
- For PAP, ze receives username + password plaintext from the peer and forwards to RADIUS via the auth handler -- the wire is cleartext but the link should be IPsec-protected (umbrella note)
- Proxy auth is OPT-IN per the umbrella -- default is to re-auth even when proxy AVPs present, because the LAC might be untrusted

## Current Behavior (MANDATORY)

**Source files read:**

- [ ] `internal/component/ppp/auth_hook.go` (after 6a) -- stub interface
  -> Constraint: stub is single method `Authenticate(ctx, req) Response` returning success; this spec REPLACES the stub with a real channel-based dispatcher
- [ ] `internal/component/ppp/lcp_options.go` (after 6a) -- Auth-Protocol option negotiation
  -> Constraint: 6a advertises Auth-Protocol; 6b makes the advertised value config-driven and adds NAK-fallback logic
- [ ] `internal/component/l2tp/session.go` -- proxy auth fields
  -> Constraint: `proxyAuthenType`, `proxyAuthenName`, `proxyAuthenChallenge`, `proxyAuthenID`, `proxyAuthenResponse` already populated in Phase 4; pass through to PPP `StartSession` payload

**Behavior to preserve:**
- All Phase 6a behavior unchanged
- LCP FSM and Echo unchanged
- Proxy LCP unchanged

**Behavior to change:**
- `AuthHook` interface replaced by event-channel dispatch on `Manager`
- LCP CONFREQ Auth-Protocol value driven by config (not hardcoded in 6a)
- `StartSession` payload extended with proxy auth bytes + `trust-proxy bool` config
- `EventSessionUp` is now emitted ONLY after successful authentication (in 6a it fired right after stub success; same emission point, but now the stub is gone)

## Data Flow (MANDATORY)

### Entry Point

- LCP reaches Opened (6a)
- PPP per-session goroutine enters Authentication phase
- If proxy auth AVPs present AND `trust-proxy=true`: emit `EventAuthSuccess` with proxied identity, skip wire exchange
- Otherwise: send PAP/CHAP/MS-CHAPv2 packets per negotiated Auth-Protocol

### Transformation Path

1. After LCP-Opened, PPP goroutine reads negotiated Auth-Protocol from session state
2. CHAP/MS-CHAPv2: PPP generates challenge (crypto/rand), sends Challenge packet to peer, waits for Response
3. PAP: PPP waits for Authenticate-Request packet from peer
4. On receiving Response/Authenticate-Request: PPP builds `EventAuthRequest` (method, identifier, username, challenge, response) and writes to `Manager.EventsOut()`
5. External handler (in test: stub; in production: l2tp-auth plugin) processes, calls `Manager.AuthResponse(sessionID, accept, message, mppeKeys)` (mppeKeys nil for now; placeholder for future CCP)
6. Manager routes `AuthResponse` back to the per-session goroutine via per-session channel
7. PPP sends Success/Ack or Failure/Nak packet to peer
8. On accept: emit `EventSessionUp`; on reject: emit `EventSessionDown`, send LCP Terminate-Request, exit

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| LCP -> Auth phase | internal state transition in per-session goroutine | [ ] |
| PPP -> external handler | `Manager.EventsOut() <-chan Event` carrying `EventAuthRequest` | [ ] |
| External handler -> PPP | `Manager.AuthResponse(sessionID, ...)` method | [ ] |
| Auth -> NCP phase | internal state transition (handed off to spec-l2tp-6c-ncp) | [ ] |

### Integration Points
- `Manager.AuthResponse` is the new method added to the public API
- `Event` sealed sum extended with `EventAuthRequest`, `EventAuthSuccess`, `EventAuthFailure`
- `StartSession` payload extended with `AuthMethod` config (PAP / CHAP-MD5 / MS-CHAPv2 / any) and proxy auth bytes
- Test stub: `internal/component/ppp/helpers_test.go` (extending 6a's helpers)

### Architectural Verification
- [ ] No bypassed layers (PPP still does not import l2tp; auth handler still goes through events channel)
- [ ] No unintended coupling (auth method negotiation lives entirely in `lcp_options.go` extension; auth wire codec lives in new files)
- [ ] No duplicated functionality (reuses 6a frame/LCP I/O; only the auth phase is added)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| LCP reaches Opened | -> | PPP enters auth phase, sends Challenge or waits for PAP request | `TestAuthPhaseEntered` (ppp/auth_test.go) |
| PAP Authenticate-Request from peer | -> | PPP emits `EventAuthRequest` with username + password | `TestPAPRequestEmitsEvent` (ppp/pap_test.go) |
| CHAP-MD5 Response from peer | -> | PPP emits `EventAuthRequest` with challenge + response | `TestCHAPResponseEmitsEvent` (ppp/chap_test.go) |
| MS-CHAPv2 Response from peer | -> | PPP emits `EventAuthRequest` with peer-challenge + NT-response | `TestMSCHAPv2ResponseEmitsEvent` (ppp/mschapv2_test.go) |
| `Manager.AuthResponse(id, accept=true, ...)` called | -> | PPP sends Success/Ack to peer; emits `EventSessionUp` | `TestAuthAcceptEmitsSessionUp` (ppp/manager_test.go) |
| `Manager.AuthResponse(id, accept=false, ...)` called | -> | PPP sends Failure/Nak; sends LCP Terminate-Request; emits `EventSessionDown` | `TestAuthRejectTearsDown` (ppp/manager_test.go) |
| Proxy auth AVPs present + `trust-proxy=true` | -> | PPP skips wire exchange, emits `EventAuthSuccess` | `TestProxyAuthShortCircuit` (ppp/proxy_test.go) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | LCP-Opened with negotiated Auth-Protocol = PAP | PPP waits for peer's Authenticate-Request |
| AC-2 | LCP-Opened with negotiated Auth-Protocol = CHAP-MD5 | PPP sends Challenge with random 16-byte value, identifier from monotonic counter |
| AC-3 | LCP-Opened with negotiated Auth-Protocol = MS-CHAPv2 | PPP sends Challenge with random 16-byte value |
| AC-4 | Peer sends valid PAP Authenticate-Request | PPP emits `EventAuthRequest{method=PAP, username, password}` |
| AC-5 | Peer sends CHAP Response with our challenge identifier | PPP emits `EventAuthRequest{method=CHAP, identifier, challenge, response, peer-name}` |
| AC-6 | Peer sends MS-CHAPv2 Response | PPP emits `EventAuthRequest{method=MSCHAPv2, peer-challenge, nt-response, peer-name}` |
| AC-7 | Handler calls `AuthResponse(id, accept=true, message)` after PAP | PPP sends Authenticate-Ack with message field; emits `EventSessionUp` |
| AC-8 | Handler calls `AuthResponse(id, accept=false, ...)` after CHAP | PPP sends Failure with message field; sends LCP Terminate-Request; emits `EventSessionDown` |
| AC-9 | Handler calls `AuthResponse` for MS-CHAPv2 with `authenticatorResponse` blob | PPP sends MS-CHAPv2 Success carrying that blob |
| AC-10 | No AuthResponse within `auth-timeout` (default 30s) | PPP emits `EventAuthFailure{Reason:"timeout"}`; `s.fail` emits `EventSessionDown`; session goroutine exits. No wire-level Nak is sent on timeout -- the LCP Terminate-Request that follows the session-down signal is the single authoritative tear-down. (Matches the Phase 1 `runAuthPhase` timeout contract; `runPAPAuthPhase` preserves it.) |
| AC-11 | Proxy auth AVPs present, config `trust-proxy=true` | PPP skips wire exchange; emits `EventAuthSuccess`; transitions to NCP phase |
| AC-12 | Proxy auth AVPs present, config `trust-proxy=false` (default) | PPP ignores proxy bytes; runs full auth exchange |
| AC-13 | Peer NAKs the proposed Auth-Protocol with alternative | PPP fallbacks per `auth-fallback-order` config (default: prefer CHAP > MS-CHAPv2 > PAP) |
| AC-14 | Periodic re-auth enabled (CHAP only), interval elapses | PPP sends new Challenge with fresh identifier; on Failure tears down session |
| AC-15 | Auth-Request received before LCP-Opened | PPP discards packet; logs warning |
| AC-16 | Identifier mismatch in CHAP Response (peer used wrong identifier) | PPP discards; auth eventually times out |
| AC-17 | MS-CHAPv2 Response with wrong Reserved bytes (non-zero) | PPP rejects with Failure; emits `EventSessionDown` |

## TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPAPParseRequest` | `internal/component/ppp/pap_test.go` | PAP Authenticate-Request decode (username + password length-prefixed) | |
| `TestPAPWriteAck` | `internal/component/ppp/pap_test.go` | PAP Ack encode with message field | |
| `TestPAPWriteNak` | `internal/component/ppp/pap_test.go` | PAP Nak encode | |
| `TestCHAPParseResponse` | `internal/component/ppp/chap_test.go` | CHAP Response decode (value-size + value + name) | |
| `TestCHAPWriteChallenge` | `internal/component/ppp/chap_test.go` | CHAP Challenge encode with random value | |
| `TestCHAPWriteSuccess` | `internal/component/ppp/chap_test.go` | CHAP Success encode | |
| `TestCHAPWriteFailure` | `internal/component/ppp/chap_test.go` | CHAP Failure encode | |
| `TestMSCHAPv2ParseResponse` | `internal/component/ppp/mschapv2_test.go` | MS-CHAPv2 Response decode (Peer-Challenge 16 + Reserved 8 + NT-Response 24 + Flags 1 + Name) | |
| `TestMSCHAPv2WriteSuccess` | `internal/component/ppp/mschapv2_test.go` | MS-CHAPv2 Success encode with authenticator-response | |
| `TestMSCHAPv2RejectsNonZeroReserved` | `internal/component/ppp/mschapv2_test.go` | Response with Reserved != 0 rejected | |
| `TestProxyAuthDecode` | `internal/component/ppp/proxy_test.go` | Decode Proxy-Authen-Type + ID + Challenge + Name + Response into auth state | |
| `TestProxyAuthShortCircuit` | `internal/component/ppp/proxy_test.go` | trust-proxy=true skips wire exchange, emits EventAuthSuccess | |
| `TestProxyAuthDistrust` | `internal/component/ppp/proxy_test.go` | trust-proxy=false ignores proxy bytes; runs full exchange | |
| `TestAuthDispatcherEventEmission` | `internal/component/ppp/auth_test.go` | EventAuthRequest emitted with correct method tag and payload | |
| `TestAuthResponseRoutedToSession` | `internal/component/ppp/auth_test.go` | Manager.AuthResponse routes to correct per-session channel | |
| `TestAuthTimeout` | `internal/component/ppp/auth_test.go` | No AuthResponse within auth-timeout triggers Failure + Terminate | |
| `TestAuthFallbackOnNak` | `internal/component/ppp/lcp_options_test.go` | LCP Auth-Protocol Nak triggers fallback per config order | |
| `TestPeriodicReauthCHAP` | `internal/component/ppp/auth_test.go` | After re-auth interval, new Challenge sent; Failure tears down |
| `TestAuthAcceptEmitsSessionUp` | `internal/component/ppp/manager_test.go` | accept=true -> EventSessionUp |
| `TestAuthRejectTearsDown` | `internal/component/ppp/manager_test.go` | accept=false -> Terminate + EventSessionDown |

### Boundary Tests (MANDATORY for numeric inputs)

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| PAP username length | 0-255 | 255 | N/A | N/A (uint8) |
| PAP password length | 0-255 | 255 | N/A | N/A (uint8) |
| CHAP value-size | 1-255 | 255 | 0 | N/A (uint8) |
| CHAP-MD5 challenge length | 16 (recommended), 1-255 accepted | 255 | 0 | N/A |
| MS-CHAPv2 fixed Response length | 49 bytes (16+8+24+1) | 49 | 48 | 50 |
| Auth identifier (echo from peer) | 0-255 | 255 | N/A | N/A (uint8) |
| auth-timeout seconds | 1-300 | 300 | 0 | 301 |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `auth-pap-net-pipe` | `internal/component/ppp/manager_test.go::TestAuthPAPNetPipe` | Full PAP exchange with scripted peer over net.Pipe; in-test handler accepts | |
| `auth-chap-net-pipe` | `internal/component/ppp/manager_test.go::TestAuthCHAPNetPipe` | Full CHAP-MD5 exchange | |
| `auth-mschapv2-net-pipe` | `internal/component/ppp/manager_test.go::TestAuthMSCHAPv2NetPipe` | Full MS-CHAPv2 exchange | |

### Future (if deferring any tests)

- `.ci` test against accel-ppp peer with real RADIUS -- deferred to spec-l2tp-7-subsystem + spec-l2tp-8-plugins (l2tp-auth)

## Files to Modify

- `internal/component/ppp/auth_hook.go` -- DELETE stub interface (replaced by event-channel dispatch). Per `rules/no-layering.md` -- delete first, then implement.
- `internal/component/ppp/manager.go` -- add `AuthResponse(sessionID uint32, accept bool, message string, authenticatorResponse []byte) error`; route to per-session channel; extend lifecycle docs
- `internal/component/ppp/session.go` -- add per-session auth-response channel; auth state fields (negotiated method, identifier counter, pending-request flag)
- `internal/component/ppp/start_session.go` -- add `AuthMethod`, `AuthFallbackOrder`, `TrustProxyAuth`, `AuthTimeout`, `ReauthInterval` fields; add proxy auth byte fields
- `internal/component/ppp/events.go` -- add `EventAuthRequest`, `EventAuthSuccess`, `EventAuthFailure`
- `internal/component/ppp/lcp_options.go` -- make Auth-Protocol option config-driven; add NAK-fallback logic
- `internal/component/ppp/proxy.go` -- extend with proxy-auth handling (currently 6a only handles proxy-LCP)
- `internal/component/ppp/lcp_handlers.go` -- on LCP-Opened, transition to Auth phase (was: stub call returning success)

### Integration Checklist

| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | [ ] | N/A in 6b; auth knobs (`auth-method`, `trust-proxy`, `auth-timeout`, `reauth-interval`) come via env vars; YANG wiring in spec-l2tp-7-subsystem |
| CLI commands/flags | [ ] | N/A |
| Editor autocomplete | [ ] | N/A |
| Functional test for new RPC/API | [x] | `internal/component/ppp/manager_test.go` (`.ci` deferred to Phase 7) |

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] | N/A in 6b |
| 2 | Config syntax changed? | [ ] | N/A in 6b (config knobs via env only until Phase 7 wires YANG) |
| 3 | CLI command added/changed? | [ ] | N/A |
| 4 | API/RPC added/changed? | [ ] | N/A |
| 5 | Plugin added/changed? | [ ] | N/A in 6b; l2tp-auth plugin is Phase 8 |
| 6 | Has a user guide page? | [ ] | N/A |
| 7 | Wire format changed? | [ ] | N/A (PPP wire format scope unchanged) |
| 8 | Plugin SDK/protocol changed? | [ ] | N/A |
| 9 | RFC behavior implemented? | [x] | `rfc/short/rfc1334.md` (CREATE), `rfc/short/rfc1994.md` (extend), `rfc/short/rfc2759.md` (CREATE), `rfc/short/rfc2661.md` (Section 18 proxy-auth note) |
| 10 | Test infrastructure changed? | [ ] | N/A |
| 11 | Affects daemon comparison? | [ ] | N/A in 6b |
| 12 | Internal architecture changed? | [ ] | N/A (boundary unchanged from 6a) |

## Files to Create

- `internal/component/ppp/pap.go` -- PAP packet codec
- `internal/component/ppp/chap.go` -- CHAP-MD5 packet codec
- `internal/component/ppp/mschapv2.go` -- MS-CHAPv2 packet codec
- `internal/component/ppp/auth.go` -- auth phase state machine, dispatcher, timeout, re-auth
- `internal/component/ppp/auth_test.go`
- `internal/component/ppp/pap_test.go`
- `internal/component/ppp/chap_test.go`
- `internal/component/ppp/mschapv2_test.go`
- `rfc/short/rfc1334.md` -- PAP summary (if not already created)
- `rfc/short/rfc2759.md` -- MS-CHAPv2 summary (if not already created)

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file + 6a + umbrella |
| 2. Audit | Files to Modify, Files to Create |
| 3. Implement (TDD) | Implementation phases below |
| 4. Full verification | `make ze-verify-fast` |
| 5-12 | Standard flow |

### Implementation Phases

1. **Delete 6a stub** -- remove `auth_hook.go`. Code that referenced it now fails to compile -- guides where the new dispatcher must hook in.
2. **Auth events + AuthResponse method** -- extend `events.go`, add `Manager.AuthResponse`, add per-session response channel. Test: `TestAuthResponseRoutedToSession`.
3. **PAP codec + handler** -- `pap.go`. Tests: `TestPAPParseRequest`, `TestPAPWriteAck`, `TestPAPWriteNak`, `TestPAPRequestEmitsEvent`.
4. **CHAP-MD5 codec + handler** -- `chap.go`. Tests: `TestCHAPParseResponse`, `TestCHAPWriteChallenge`, `TestCHAPWriteSuccess`, `TestCHAPWriteFailure`, `TestCHAPResponseEmitsEvent`.
5. **MS-CHAPv2 codec + handler** -- `mschapv2.go`. Tests: `TestMSCHAPv2ParseResponse`, `TestMSCHAPv2WriteSuccess`, `TestMSCHAPv2RejectsNonZeroReserved`, `TestMSCHAPv2ResponseEmitsEvent`.
6. **Auth phase state machine** -- `auth.go`. Drives the LCP-Opened -> auth -> NCP transition. Handles timeout. Tests: `TestAuthPhaseEntered`, `TestAuthTimeout`, `TestAuthAcceptEmitsSessionUp`, `TestAuthRejectTearsDown`.
7. **Proxy auth** -- extend `proxy.go`. Tests: `TestProxyAuthDecode`, `TestProxyAuthShortCircuit`, `TestProxyAuthDistrust`.
8. **LCP NAK fallback** -- extend `lcp_options.go`. Test: `TestAuthFallbackOnNak`.
9. **Periodic re-auth** -- extend `auth.go`. Test: `TestPeriodicReauthCHAP`.
10. **Net.Pipe end-to-end** -- extend `manager_test.go` with full PAP/CHAP/MS-CHAPv2 exchanges. Tests: `TestAuthPAPNetPipe`, `TestAuthCHAPNetPipe`, `TestAuthMSCHAPv2NetPipe`.
11. **RFC summaries** -- create `rfc/short/rfc1334.md`, `rfc/short/rfc2759.md`; extend `rfc1994.md` and `rfc2661.md` Section 18.
12. **Functional verification** -- `make ze-verify-fast`.

### Critical Review Checklist (/implement stage 5)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has a Go test naming a file:line; assertion verifies AC behavior |
| Correctness | PAP/CHAP/MS-CHAPv2 wire format matches RFC byte-for-byte; identifier echo correct |
| Naming | Types: `PAPPacket`, `CHAPPacket`, `MSCHAPv2Packet`; events: `EventAuthRequest` |
| Data flow | ze never sees plaintext password except for PAP (where the protocol forces it); CHAP/MS-CHAPv2 challenge+response opaque to ze |
| Rule: no-layering | 6a stub deleted, not extended |
| Rule: buffer-first | All packet encoding via offset writes |
| Security | Proxy-auth defaults to NOT trusted (`trust-proxy=false`); auth timeout enforced |

### Deliverables Checklist (/implement stage 9)

| Deliverable | Verification method |
|-------------|---------------------|
| 6a `auth_hook.go` is deleted | `ls internal/component/ppp/auth_hook.go` -> not exist |
| Three auth methods compile + parse + serialize | All `*_test.go` files for pap/chap/mschapv2 pass |
| AuthResponse method exists on Manager | `go doc codeberg.org/thomas-mangin/ze/internal/component/ppp.Manager.AuthResponse` |
| Auth timeout fires | `TestAuthTimeout` passes with measurable timing |
| Proxy auth opt-in works | `TestProxyAuthShortCircuit` and `TestProxyAuthDistrust` both pass |
| RFC summaries exist | `ls rfc/short/rfc1334.md rfc/short/rfc2759.md` -> both exist |

### Security Review Checklist (/implement stage 10)

| Check | What to look for |
|-------|-----------------|
| Input validation | Every packet length validated; option lengths bounded; no silent truncation |
| Resource exhaustion | One outstanding auth-request per session; auth-timeout bounded |
| Plaintext handling | PAP password never logged at any level; CHAP/MS-CHAPv2 challenge+response can be logged at debug for diagnostics |
| Identifier replay | CHAP identifier MUST change each Challenge; verify with `TestCHAPIdentifierMonotonic` |
| Random quality | CHAP challenges via `crypto/rand`; MS-CHAPv2 authenticator challenges via `crypto/rand`; verified by `TestCHAPChallengeRandom` |
| Proxy-auth default | `trust-proxy=false` is the default; documented in spec and code |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read RFC 1334/1994/2759 |
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

- ze never validates auth credentials itself -- it shuttles wire bytes between the peer and an external handler. This is the right boundary: auth policy belongs in the l2tp-auth plugin (Phase 8), and from there in RADIUS.
- Proxy-auth opt-out by default is a security stance: an LAC that lies in ICCN should not auto-grant access; the LNS must be the source of truth.
- MS-CHAPv2 looks intimidating but on the wire it is just a 49-byte response blob -- the cryptographic machinery is in the NT-hash + RADIUS path, neither of which is ze's responsibility.

## RFC Documentation

Add `// RFC 1334 Section X.Y: "..."` above PAP packet handlers.
Add `// RFC 1994 Section X.Y: "..."` above CHAP Challenge/Response/Success/Failure code paths and identifier-change comment.
Add `// RFC 2759 Section X.Y: "..."` above MS-CHAPv2 Response parser (Reserved-must-be-zero) and Success encoder.
Add `// RFC 2661 Section 18: "..."` above proxy-auth short-circuit + distrust paths.

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
- [ ] AC-1..AC-17 all demonstrated
- [ ] Wiring Test table complete
- [ ] `make ze-verify-fast` passes
- [ ] Feature code integrated
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)
- [ ] RFC 1334, 1994, 2759, 2661 §18 constraint comments added
- [ ] RFC summaries (rfc1334, rfc2759) exist
- [ ] Implementation Audit complete

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
- [ ] Functional tests for end-to-end behavior (Go-level via net.Pipe)

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary
- [ ] Summary included in commit
