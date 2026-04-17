# 609 -- spec-l2tp-6b-auth (PPP Authentication)

## Context

Phase 6a left a stub `AuthHook` interface that every session ran through
to reach the NCP phase. No real PPP authentication existed: the LAC's
choice of Auth-Protocol had no effect on ze's behaviour, ze never
emitted credentials for a handler to validate, and there was no way to
re-authenticate a long-lived session. The goal of 6b was to replace
the stub with a real channel-based auth dispatcher (PAP / CHAP-MD5 /
MS-CHAPv2 wire format + `EventAuthRequest` / `AuthResponse` plumbing),
wire it into the LCP Auth-Protocol negotiation, fall back correctly on
peer Configure-Nak / Configure-Reject of Auth-Protocol, and add
periodic re-authentication. RADIUS validation itself stays in
spec-l2tp-8-plugins (`l2tp-auth`).

## Decisions

- Channel-based auth dispatch over a synchronous `AuthHook` interface.
  `Manager.AuthEventsOut()` + `Manager.AuthResponse(tunnelID, sessionID,
  accept, message, blob)` decouples ze's PPP code from the RADIUS/AAA
  policy and matches the existing plugin IPC shape.
- **Proxy-auth (RFC 2661 §18 `trust-proxy`) dropped** after surveying
  accel-ppp, mpd5, and xl2tpd: all three silently accept the AVPs and
  then re-authenticate over PPP (xl2tpd's source comment calls it "a
  ridiculous security threat"). Shipping a knob no one asks for would
  have introduced a credential-forwarding surface with no offsetting
  benefit. The decoded `proxyAuthen*` fields remain available for logs.
- AC-16 Identifier-mismatch semantics implemented as **silent-discard
  with shared auth-timeout** rather than the Phase 5 fail-immediately
  placeholder. The loop lives in a single generic helper
  `waitCHAPLike[T]` consumed by both CHAP-MD5 and MS-CHAPv2. Malformed
  frames, wrong protocol, and structural parse errors (e.g. MS-CHAPv2
  Reserved != 0) still tear down immediately.
- Periodic re-authentication implemented as a **third ticker in the
  session's main select loop**, not a background goroutine. On tick the
  handler inline-invokes `runAuthPhase`, reusing the same
  `runCHAPAuthPhase` / `runMSCHAPv2AuthPhase` helpers as initial auth.
  Keeps single-goroutine-per-session invariant intact; trades brief
  echo-ticker pause during the reauth round-trip for code simplicity.
- `waitCHAPLike` routes **non-CHAP frames through `s.handleFrame`**
  instead of failing on them. Critical for periodic re-auth: a benign
  non-CHAP frame (e.g. IPCP or NCP control during a long-lived session)
  arriving during the reauth round would otherwise tear the session
  down every `ReauthInterval`. LCP handling during auth is bounded by
  a pre-existing FSM re-entrance in `handleLCPPacket`'s Opened branch
  (out-of-scope for Phase 9); the regression test uses IPCP 0x8021 to
  exercise the "don't fail on stray frames" half of the fix without
  triggering that separate pre-existing issue.
- **Safety floor on `ReauthInterval`** (5 s) applied by
  `clampReauthInterval` in the L2TP reactor. Values below the floor are
  clamped up with a WARN rather than rejected; zero / malformed / empty
  disables re-auth entirely. Programmatic callers constructing
  `ppp.StartSession` directly bypass the clamp (tests use 150 ms to
  prove behaviour inside a second-scale test window).
- `ReauthInterval` surfaced via `ze.l2tp.auth.reauth-interval` env var
  (duration, default `0s` = disabled). Same inline-parse pattern as
  `ze.l2tp.auth.timeout`: operator typos log a WARN instead of silently
  falling back.
- LCP Configure-Nak / Configure-Reject fallback (AC-13) chose
  **membership-only lookup over position downgrade**: if the peer's Nak
  suggestion is in `AuthFallbackOrder`, use it verbatim; no match ->
  `AuthMethodNone`. Position in the order list is advisory to the
  operator about preference ranking, not a force-downgrade.
- Default `AuthFallbackOrder` is `[CHAP-MD5, MS-CHAPv2, PAP]`. PAP is
  last because it sends cleartext passwords on the wire (the same
  guidance AC-13 text captures).

## Consequences

- `Manager.AuthResponse` is now part of the public PPP package API.
  Production consumers (the future l2tp-auth plugin) will implement an
  event subscriber that emits `AuthResponse` per request; tests use
  `autoAcceptAuth` or direct `d.AuthResponse(...)` calls.
- `pppSession` now carries four mu-free goroutine-owned auth fields:
  `configuredAuthMethod`, `authFallbackOrder`, `authTimeout`,
  `reauthInterval`. Writers outside the session goroutine are only
  `manager.spawnSession` (initial set) and LCP Nak/Reject handling.
- `session_run.go` main select now drives three tickers (negoTimer ->
  echoTicker -> reauthTicker). Reauth and echo can coexist because
  `time.Ticker` buffers at most one pending tick; a reauth round that
  outlasts one echo interval shows up as one buffered echo-tick, not a
  lost one.
- The generic `waitCHAPLike[T]` helper is a precedent for any future
  "read a frame, check identifier, loop or dispatch" pattern in the
  package; it is test-proven with two concrete parse callbacks.
- Spec-l2tp-7-subsystem inherits two env vars to wire to YANG leaves:
  `ze.l2tp.auth.timeout` (already Phase 3) and
  `ze.l2tp.auth.reauth-interval` (added Phase 9). The
  `auth-method` / `trust-proxy` leaves the original Phase 3 note
  mentioned can be dropped entirely (trust-proxy removed; auth-method
  drives from `StartSession.AuthMethod` via the L2TP reactor, not a
  per-session YANG leaf).
- Phase 8/9 together close the Phase 5 deviation that documented
  AC-16 as fail-immediately pending a retry loop; AC-16 now behaves as
  the spec text always described.

## Gotchas

- **net.Pipe synchronous writes deadlock auth-phase tests** that don't
  drain Success/Failure frames. Every reauth / accept test must read
  the reply frame off `peerEnd` before checking the handler's return
  value, or the write blocks forever. The `chap_reauth_test.go`
  helpers include explicit `readCHAPReplyCode` steps for this reason.
- **`scriptedPeer` idles on peerEnd** after the LCP handshake, which
  consumes any subsequent frames the driver writes. For normal-path
  dispatch tests that need to observe post-Opened CHAP Challenge
  frames, use the new `scriptedPeerLCPOnly` helper that exits once
  ze's CA is consumed.
- **Formatter/linter race with unused imports.** The `strconv` import
  had to be added in the same Edit that uses it, otherwise
  `goimports` (run post-write) removes the import before the function
  body references land. This forced a single `Write` of `auth.go`
  rather than sequential `Edit`s.
- **`dupl` linter flagged parallel waitCHAPResponse / waitMSCHAPv2Response
  as duplicates.** Resolved by extracting `waitCHAPLike[T]` (Go 1.18+
  generics) with a typed parse callback; also a cleaner design than
  two copy-pasted functions.
- **Phase 9 tests use a 150 ms `ReauthInterval`** deliberately far
  below the default `EchoInterval` (10 s) so the echo ticker never
  fires during a reauth round-trip in the test window. In production
  the intervals are independent and the reauth pause is bounded by
  `AuthTimeout`.
- **3 pre-existing suite failures surfaced during `ze-verify-fast`**
  (BGP encode addpath, fib-vpp-* plugin tests, exabgp conf-addpath)
  are unrelated to Phase 9 (no PPP / L2TP / auth files touched).
  Logged to `plan/known-failures.md` under the 2026-04-17 entry.

## Files

- `internal/component/ppp/auth.go` -- `adjustAuthOnNakOrReject`,
  `selectAuthFallback`, `waitCHAPLike[T]`, `waitCHAPResponse`,
  `waitMSCHAPv2Response`, `authMethodToLCPOptions`,
  `authMethodFromAuthProto`, `defaultAuthFallbackOrder`,
  `awaitAuthDecision`.
- `internal/component/ppp/pap.go`, `chap.go`, `mschapv2.go` -- per-
  method wire codecs + `runXAuthPhase` handlers.
- `internal/component/ppp/session.go` -- `pppSession` fields
  (`authTimeout`, `authFallbackOrder`, `reauthInterval`,
  `configuredAuthMethod`, `negotiatedAuthMethod`, `chapIdentifier`,
  `authRespCh`, `framesIn`, `sessStop`).
- `internal/component/ppp/start_session.go` -- `AuthMethod`,
  `AuthFallbackOrder`, `AuthTimeout`, `ReauthInterval`.
- `internal/component/ppp/session_run.go` -- main select with
  echo + reauth tickers; `runAuthPhase` dispatcher.
- `internal/component/ppp/manager.go` -- `AuthResponse`,
  `AuthEventsOut`, per-session routing.
- `internal/component/ppp/auth_events.go` -- `EventAuthRequest`,
  `EventAuthSuccess`, `EventAuthFailure`, `AuthMethod` enum.
- `internal/component/ppp/auth_dispatch_test.go` (Phase 8) --
  Nak/Reject fallback integration tests.
- `internal/component/ppp/auth_normal_dispatch_test.go` (Phase 9
  Decision E) -- normal-path PAP/CHAP/MS-CHAPv2 dispatch tests +
  post-fallback dispatch.
- `internal/component/ppp/chap_reauth_test.go` (Phase 9) -- AC-16
  silent-discard + AC-14 periodic reauth tear-down.
- `internal/component/config/environment.go` -- env registrations for
  `ze.l2tp.auth.timeout` and `ze.l2tp.auth.reauth-interval`.
- `internal/component/l2tp/reactor.go` -- parse env vars and populate
  `StartSession{AuthTimeout, ReauthInterval}`.
- `rfc/short/rfc1334.md` (Phase 4), `rfc/short/rfc2759.md` (Phase 6).
