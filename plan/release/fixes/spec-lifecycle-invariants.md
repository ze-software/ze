# Spec: lifecycle-invariants

| Field | Value |
|-------|-------|
| Status | ready |
| Priority | P1 |
| Release reason | Subscriber activation, accounting, timers and applied CoA results must preserve their lifecycle invariants. |
| Scope | protocol |
| Depends | - |
| Phase | 1/9 |
| Updated | 2026-11-26 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**One invariant, violated in four places: a fact or a step that belongs to the
FIRST activation of a thing must not be recomputed, re-sent, or skipped when a
LATER event touches the same thing.**

Found on 2026-08-02 by taking the three bug classes fixed in
veesix-networks/osvbng during July 2026 and hunting each one in ze. Every defect
was verified against HEAD by reading the producing function. Several defects
originally catalogued in this spec have since landed at HEAD (D-1 code, D-10
metadata re-key, D-14 PPPoE subscriber routes and `pppoe` redistribute source,
I-3 pinned VPP deployment image, I-4 Framed-Route, and partial pieces of
D-3/D-4/D-5/D-6/D-7/D-13); those are retired into `## Existing Mechanisms` and
`## Implementation progress` below so remaining phases implement only the open
work and missing functional/unit proofs.

### Root cause 1: the subscriber event namespace still leaves core lifecycle consumers on `l2tpevents`

The unified subscriber session model
(`docs/architecture/l2tp/subscriber-session-model.md`) built a transport-generic
`subscriber` event namespace so PPPoE and L2TP sessions would drive the same
downstream plugins. Pool release (D-1) and subscriber route redistribution
(D-14) now consume `subevents`, and handler registries delegate
(`l2tp.Register*` forwards to `subscriber.Register*`). However, RADIUS
accounting (`authradius/acct.go`), dynamic CoS (`cos/handler.go`), and the rate
shaper's session table and CoA event path (`shaper/shaper.go`,
`shaper/rate_transaction.go`, `cos/session_transaction.go`) still subscribe only
to `l2tpevents` or key session state on L2TP-only coordinates (`L2TPBirth` /
`TunnelID`/`SessionID`), leaving PPPoE sessions unbilled, unrecorded in
`show l2tp shaper`, unable to receive CoA rate/CoS changes, and unable to
restore qdiscs/CoS on teardown.

### Root cause 2: PPP renegotiation defeats the re-entry guard and tears down the network phase

`TestHandleLCPPacketOpenedRXRDoesNotReenterOpened` (`ppp/echo_test.go`) records
in its own docstring: *"PREVENTS: regression of the known re-entry bug where
every Echo packet in Opened triggered setMRU, SetMTU, auth, NCP, and
EventSessionUp again."* That bug was fixed with a `cur != LCPStateOpened` guard.

Renegotiation defeats that exact guard. `LCPDoTransition` (`ppp/ppp_fsm.go`)
maps `Opened` plus a peer Configure-Request to `AckSent` with `LCPActTLD`, then
`AckSent` plus Configure-Ack back to `Opened`. Today `LCPActTLD` emits
`EventLCPDown{NetworkPhase: true}`, causing `reactor_kernel.go`
(`handleLCPDown`) and `pppoe/subsystem.go` (`withdrawSubscriber`) to tear down
the subscriber lifecycle (clearing metadata, cancelling timeouts, and ending
accounting), and then `afterLCPOpen()` re-runs `runAuthPhase()` (a second
RADIUS Access-Request, breaking AC-17) and `runNCPPhase()` (a fresh
`SessionIPAssigned`, producing a second Accounting-Start under a new
`Acct-Session-Id` and restarting `Session-Timeout`, breaking AC-6, AC-7, AC-8).
RFC 1661 Section 4.3 requires an implementation to be ready for renegotiation
at any time, so it is normal traffic, not an error path.

### Root cause 3: a replacement object stamped with a new birth time

Two instances, one of them a disagreement between sibling functions in one file
(`ike/engine/rekey.go`), plus `flowexport` re-stamping `startTime` on config
reload (`flowexport/exporter.go`, `flowexport/register.go`).

### Root cause 4: two namespaces sharing one keyspace (partially landed; `discoveryIndexMax` cap, remaining consumer re-keys, and proofs open)

`l2tp/session_metadata.go`, `ppp/manager.go`, `pool/register.go`, and
`route_observer.go` have already moved to canonical `SubscriberID` keys at HEAD.
The remaining open work under this root cause is removing the leftover 16-bit
`discoveryIndexMax` refusal in `pppoe/subsystem_access.go:63-70` (which still
blocks any PPPoE access interface with `ifindex > 65535`), migrating the
leftover `L2TPBirth` / `(TunnelID, SessionID)` session maps in
`authradius/acct.go`, `shaper/shaper.go`, `shaper/rate_transaction.go`,
`cos/session_state.go`, and `cos/session_transaction.go`, plus landing the
cross-transport collision and ifindex-above-16-bit unit proofs (AC-11, AC-22)
and closing journal row D-13.

## Verified defects

| # | Producing function | Status at HEAD | Defect / remaining work |
|---|--------------------|----------------|-------------------------|
| D-1 | `poolPlugin.onSessionDown` (`l2tp/plugins/pool/register.go`) | **code landed; `.ci` open** | Code already subscribes to `subevents.SessionDown`, keys `sessionAddrs`/`sessionPrefixes` by canonical `subscriberID`, and passes unit tests. Remaining: functional test `test/pppoe/pool-release-cycle.ci` (AC-1). |
| D-2 | `radiusAcct.subscribeEventBus` (`l2tp/plugins/authradius/acct.go`) | **open** | Binds `l2tpevents.{SessionIPAssigned,SessionIPReleased,SessionDown}` only and keys `sessions` on `sessionKey{birth: subscriberid.L2TPBirth, tunnelID, sessionID}`: no Accounting-Start, interim, or Stop for any PPPoE session. |
| D-3 | `coaListener.applySubscriberCoA` (`authradius/coa.go`), `confirmCoA` (`authradius/coa_transaction.go`), `shaper/rate_transaction.go`, `cos/session_transaction.go` | **partially landed; PPPoE CoA open** | `confirmCoA` already runs synchronous 2-phase prepare/apply/commit/rollback and returns CoA-NAK on failure for L2TP. Still broken for PPPoE: (1) `coa_transaction.go:78` omits `SubscriberID` on `request l2tp shaper rate`; (2) `applySubscriberCoA` (`coa.go:557`) emits unsubscribed `subevents.SessionRateChange` and (`coa.go:578`) emits `l2tpevents.SessionCoSChange` with zero `L2TPBirth`/TID/SID; (3) `shaper/rate_transaction.go` keys `s.sessions` on `(tunnelID, sessionID)`; (4) `cos/session_transaction.go:211` rejects PPPoE because it requires `subscriberid.ParseL2TPBirth` and non-zero `SessionID`. |
| D-4 | `shaper.handleSubscriberSessionUp` (`l2tp/plugins/shaper/shaper.go`), `newCosHandler` (`plugins/cos/handler.go`) | **partially landed; remainder open** | `pppoe/subsystem.go:593` already loads `AuthMetadata.FilterID` and passes it to `GetShaperHandler()`, and `cos/shared_access.go` already captures and restores the real baseline VLAN maps on final departure. Still broken: `shaper.go` never records PPPoE in `s.sessions` (omitted from `show l2tp shaper`, no `RestoreOriginal` on `subevents.SessionDown`, `onSessionRateChange` still listens on `l2tpevents`), and `cos/handler.go` subscribes only to `l2tpevents` with `L2TPBirth`-only keys. |
| D-5 | `subscriberBridge.onSessionUp` (`l2tp/subscriber_bridge.go`), `Registry.Add` (`subscriber/registry.go`) | **partially landed; L2TP + registry open** | `pppoe/subsystem.go:581` already preserves `ActivatedAt` (`if !wasActive`) and `Username`. Still broken: `subscriber_bridge.go:85` unconditionally overwrites `sess.ActivatedAt = time.Now()` even when `wasActive` is true, and `subscriber/registry.go:21` overwrites existing entries without preserving an established `ActivatedAt`. |
| D-6 | `ppp/session_run.go` (`LCPActTLD`, `afterLCPOpen`), `reactor_kernel.go` (`handleLCPDown`), `pppoe/subsystem.go` (`withdrawSubscriber`), `authradius/acct.go` | **partially landed; renegotiation open** | `authradius/acct.go:648` already dedupes a repeated `SessionIPAssigned` on a live session. Still broken: on mid-session LCP renegotiation, `LCPActTLD` emits `EventLCPDown{NetworkPhase: true}`, tearing down metadata, timeouts, and accounting, and `afterLCPOpen` re-runs `runAuthPhase()` (second Access-Request, AC-17) and `runNCPPhase()` (new `SessionIPAssigned` -> second Accounting-Start with new `Acct-Session-Id` and reset `Acct-Session-Time`, AC-7). |
| D-7 | `l2tpReactor.startSessionTimeouts` (`l2tp/session_timeout.go`) | **partially landed; deadline restart open** | `session_timeout.go:64-76` already cancels previous `sessionTimeoutCancel`/`idleTimeoutCancel` before starting replacements (goroutine leak fixed). Still broken: calling `startSessionTimeouts` again (or cancelling on LCP renegotiation) restarts the full `SessionTimeout` duration from `time.Now()`, extending service past the RFC 2865 Section 5.27 limit. |
| D-8 | `applyIKERekeyResponse` and `respondIKERekey` (`ike/engine/rekey.go`) | **open** | The two rekey paths disagree, and RFC 7296 says both are wrong: initiator (`rekey.go:1254-1255`) sets both `CreatedAt` and `EstablishedAt` to `time.Now()`; responder (`rekey.go:1385,1404-1405`) sets both to `oldSA.EstablishedAt`. Both must advance `CreatedAt` to `time.Now()` (Section 2.18: new SA) and carry `EstablishedAt` from `oldSA.EstablishedAt` (Section 2.8.3: authentication unchanged). |
| D-9 | `newExporter` (`plugins/flowexport/exporter.go`), `configure` (`plugins/flowexport/register.go`) | **open** | `newExporter` (`exporter.go:108`) stamps `startTime: time.Now()`, and the shared `configure` closure (`register.go:206`) runs on both boot and reload. RFC 3954 Section 5.1 defines `sysUpTime` as time since device boot; resetting it on reload corrupts header `sysUpTime` and relative `FIRST_SWITCHED`/`LAST_SWITCHED` timestamps. |
| D-10 | `StoreSessionMetadata` (`l2tp/session_metadata.go`), `lookupSessionUsername` (`pool/register.go`) | **code landed; collision proofs open** | `session_metadata.go` already keys on canonical `subscriberID string` validated by `subscriber.RequireID`, and all callers (`authradius`, `pool`, `shaper`, `cos`, `session_timeout`, `reactor_kernel`, `subscriber_bridge`, `pppoe/subsystem`) pass canonical `SubscriberID`. Remaining: cross-transport collision unit tests (`TestSessionMetadataNoCrossTransportCollision`, `TestSubscriberIDBuildersNeverCollide`) for AC-11 and A-2. |
| D-11 | `l2tpReactor.startSessionTimeouts` (`l2tp/session_timeout.go`) | **open** | Only `reactor_kernel.go:858` calls `startSessionTimeouts`; PPPoE sessions never enforce RADIUS `Session-Timeout` (RFC 2865 Section 5.27) or `Idle-Timeout` (Section 5.28). Open journal row in `invariant-enforced-by-an-absent-call-site.md` (2026-10-28). |
| D-12 | `readIfaceRXBytes` (`l2tp/iface_stats_linux.go`, `iface_stats_other.go`), `runIdleTimeout` (`session_timeout.go`) | **open** | Failed sysfs read/parse (and the non-Linux stub) returns `0`, which `runIdleTimeout` treats as zero traffic and disconnects the session with Idle-Timeout; stale comment in `session_timeout.go:108` still mentions `rtnetlink`. Open journal row in `zero-value-as-valid-answer.md` (2026-10-28). |
| D-13 | PPPoE PPP driver key (`ppp/manager.go`, `pppoe/subsystem_access.go:63-70`, `pppoe/server.go`, `subscriber/session.go`) | **partially landed; `discoveryIndexMax` cap + unit proof + journal close open** | `ppp/manager.go:85,415,581` already keys `Driver.sessions` by canonical `start.Attempt.SubscriberID()` (full `int` ifindex); `pool` and `session_metadata` use canonical `SubscriberID`; `pppoe/server.go:444` sets `StartSession.TunnelID: uint16(s.ifIndex)` only as a diagnostic label. **Still broken:** `pppoe/subsystem_access.go:63-70` still enforces `const discoveryIndexMax = 1<<16 - 1`, refusing any PPPoE access interface with `ifindex > 65535`. Remaining: remove the `discoveryIndexMax` cap in `subsystem_access.go`, add unit test `TestPPPoEIfIndexAbove16BitsKeepsDistinctSessions` (AC-22), update `docs/architecture/l2tp/bng-5-pppoe.md`, and close journal row `key-omits-a-fact-the-builder-uses.md` (2026-11-15). |
| D-14 | `StartSubscriberRouteObserver` (`l2tp/route_observer.go`), `pppoe/redistribute.go`, `pppoe/subsystem.go` | **landed and verified (2026-11-19)** | Owner approved D-14 scope on 2026-11-19. Implemented and verified at HEAD: `StartSubscriberRouteObserver` subscribes to `subevents.SessionIPAssigned` and `subevents.SessionDown`, `pppoe/redistribute.go` registers the `pppoe` redistribute source, PPPoE binds Framed-Routes to the exact auth round (`807a87e415`), `test/pppoe/redistribute-subscriber-route.ci` passed 80 guest iterations, and the `invariant-enforced-by-an-absent-call-site.md` (2026-11-15) journal row is already marked `fixed:`. Retired to Existing Mechanisms. |

## Why the gates are green

Six RFCs here are enrolled and gated by `./le rfc check`: 1661, 2865, 2866,
3954, 5176, 7296. The gate is green because the obligations these defects break
were never extracted.

| Obligation | State | Consequence |
|---|---|---|
| RFC1661-4.3-1, "must be prepared to immediately renegotiate Configuration Options on RCR+/RCR- in Opened" | extracted, gated, proven | its tagged proof `TestRFC1661RenegotiateOnConfigureRequestInOpened` asserts only that the pure function `LCPDoTransition` returns the right state and action tuple. It never constructs a session, so D-5, D-6 and D-7 sit underneath it |
| RFC 2866 Section 5.7, Acct-Session-Time semantics | not extracted | D-6 breaks it and no row exists to go red |
| RFC 2865 Section 5.27, Session-Timeout semantics | not extracted | D-7 and D-11 break it and no row exists to go red |
| RFC 5176 Section 3.1, ACK means applied | not extracted | D-3 breaks PPPoE CoA and no row exists to go red |
| RFC 3954 Section 5.1, sysUpTime is time since boot | not extracted (indicative prose, no RFC 2119 keyword) | D-9 breaks it and no row exists to go red |

Per `ai/rules/rfc-compliance.md`, an unextracted obligation is still an
obligation, and adding the row plus its proof is doing MORE, so it proceeds
without asking.

`docs/features/rfc-status.md` currently claims RFC 2866 and RFC 5176 are
"Supported for subscriber access" while PPPoE, a subscriber access type, has no
accounting at all and cannot complete CoA or Disconnect-Request. Landing this
spec makes those two rows true rather than changing them.

## Implementation progress

| Deliverable | State at HEAD |
|---|---|
| D-1 pool release for PPPoE | **code done** (2026-08-29; re-keyed to canonical `subscriberID`). Functional test `test/pppoe/pool-release-cycle.ci` still open (AC-1). |
| D-2 PPPoE RADIUS accounting | **open** (`authradius/acct.go` still on `l2tpevents` + `L2TPBirth`) |
| D-3 CoA ACK without effect / PPPoE CoA | **partial** (`confirmCoA` 2-phase transaction landed for L2TP; PPPoE shaper/CoS transaction and `subevents.SessionRateChange`/`SessionCoSChange` delivery still open) |
| D-4 shaper and CoS PPPoE state | **partial** (PPPoE `FilterID` metadata load and CoS `shared_access.go` baseline restore landed; shaper `s.sessions` keying and CoS `subevents` migration still open) |
| D-5 replacement session resets ActivatedAt | **partial** (`pppoe/subsystem.go` preserves `ActivatedAt`; `subscriber_bridge.go` and `subscriber/registry.go` still overwrite it) |
| D-6 duplicate Accounting-Start / re-auth on renegotiation | **partial** (`acct.go` dedupes repeated `SessionIPAssigned`; `ppp/session_run.go` `LCPActTLD` / `afterLCPOpen` still tears down network phase and re-runs auth + NCP) |
| D-7 timeout restart on renegotiation | **partial** (old cancel funcs are cancelled so goroutine leak is gone; original deadline is still restarted on renegotiation) |
| D-8 IKE rekey timestamps | **open** |
| D-9 flow-export epoch | **open** |
| D-10 metadata keyspace collision | **code done** (`session_metadata.go` and all callers use canonical `SubscriberID`). Cross-transport unit proofs open (AC-11, A-2). |
| D-11 PPPoE Session-Timeout and Idle-Timeout | **open** |
| D-12 failed counter read treated as idle | **open** |
| D-13 PPPoE ifindex truncated to 16 bits | **partial** (`ppp/manager.go`, `pool`, `session_metadata` use canonical `SubscriberID` with full `int` ifindex, but `pppoe/subsystem_access.go:63-70` still enforces `discoveryIndexMax = 1<<16 - 1` refusing `ifindex > 65535`). Removing `discoveryIndexMax`, unit proof `TestPPPoEIfIndexAbove16BitsKeepsDistinctSessions`, and journal row closure remain open (AC-22). |
| D-14 PPPoE subscriber routes reach `redistribute` | **done and verified** (approved 2026-11-19; `route_observer.go`, `pppoe/redistribute.go`, `test/pppoe/redistribute-subscriber-route.ci` 80/80 guest iterations, journal row marked `fixed:`). |
| I-1 emit-without-subscribe guard | **open** (Go checker in `internal/le/doc/wiring/wiring.go` + `allowlist.go`) |
| I-2 vacuity-trap and sibling-audit rows | **open** (`docs/architecture/testing/interop.md` + `ai/rules/points/principles/directives/your-blast-radius-is-what-you-can-reach.md`) |
| I-3 poll-sleep, VPP evidence image, checkVPPVersion | **partial** (VPP deployment image `ze-vpp-deployment` is already pinned to VPP v26.06 in `internal/le/build/vpp/deployment.go` and `internal/le/test/deployment/`; sub-ms `poll-sleep` and `checkVPPVersion` in `internal/plugins/iface/vpp/doctor_api.go` remain open) |
| I-4 PPPoE Disconnect-Request and Framed-Route | **partial** (Framed-Route landed and verified with D-14; Disconnect-Request `DisconnectHandler` still open for AC-15) |

The CoA boundary is shared with
`plan/release/fixes/spec-rfc5176-remaining-defect-classes.md`: this spec owns the
PPPoE delivery and applied-result path in AC-3/AC-4. The RFC 5176 residual spec
owns atomic rollback of a multi-change request and conjunctive subscriber
matching with Error-Cause 508. Neither spec's completion proves the other's
criteria.

## Improvements in scope (operator-selected 2026-08-02)

| # | Improvement | Remaining scope at HEAD |
|---|-------------|-------------------------|
| I-1 | A build-time guard that fails when an event topic is emitted and never subscribed | Extend Go `internal/le/doc/wiring/wiring.go` (`./le doc wiring`) and `allowlist.go` with an emit-without-subscribe predicate and Go unit test `TestEmitWithoutSubscribeIsReported`. |
| I-2 | A vacuity-trap row for concurrency tests that never contend, and a fresh-versus-restore trigger row in the Sibling Call-Site Audit | Add the contention vacuity-trap row to `docs/architecture/testing/interop.md` and the fresh-versus-restore sibling rule to `ai/rules/points/principles/directives/your-blast-radius-is-what-you-can-reach.md` (then render via `./le ai rules update render` and `./le ai rules update condensed`). |
| I-3 | Sub-millisecond `poll-sleep` granularity, a pinned VPP evidence image, and a `checkVPPVersion` that checks what its comment claims | VPP evidence image pin is already landed (`ze-vpp-deployment`, VPP v26.06). Remaining: microsecond `poll-sleep` in `internal/component/vpp/config.go` + `ze-vpp-conf.yang`, and version-range parsing in `internal/plugins/iface/vpp/doctor_api.go` (`checkVPPVersion`). |
| I-4 | PPPoE parity the subscriber-session-model work deferred: Disconnect-Request teardown, and RADIUS Framed-Route | Framed-Route is already landed and verified with D-14. Remaining: `DisconnectHandler` in `subscriber/handler_registry.go` wired from `pppoe/subsystem.go` and called from `authradius/coa.go` (AC-15). |

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` - platform invariants, plugin process boundary, registration over hardcoding
  → Constraint: every new handler or event topic registers; no hardcoded transport switch in core packages.
- [ ] `ai/doors/access-security.md` - BNG/L2TP/PPPoE/PPP/IKE subsystem boundaries
  → Constraint: `ppp` is a leaf package with no component imports; subscriber auth, pool, shaper and disconnect reach sessions through `l2tp/subscriber` registries.
- [ ] `ai/doors/system-plugins.md` - system plugin boundaries (`cos`, `flowexport`, `iface/vpp`)
  → Constraint: plugins communicate through the SDK, EventBus, or registered commands, never by reaching into another component's private state.
- [ ] `ai/doors/forwarding.md` - VPP config and dataplane boundaries
  → Constraint: VPP `startup.conf` grammar targets VPP v26.06 (`unix.poll-sleep-usec`).
- [ ] `ai/doors/le-tooling.md` - `internal/le/` action conventions and `.ci` fixture rules
  → Constraint: `internal/le` is never imported by product code; Go wiring tests live alongside the Go package in `internal/le/doc/wiring/`.
- [ ] `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` - ESP Child SA creation and dataplane abstraction
- [ ] `docs/architecture/l2tp/bng-1-radius-attributes.md` - RADIUS subscriber-profile attributes, accounting, and idle-timeout design
  → Constraint: this spec departs from two sentences in `Decisions` (the non-Linux `0` idle-timeout return and L2TP-only `L2TPBirth` accounting keys) and updates the page in the same work.
- [ ] `docs/architecture/l2tp/bng-5-pppoe.md` - PPPoE access concentrator design and VPP lifecycle decisions
- [ ] `docs/architecture/testing/interop.md` - live session interop and the vacuity-trap table I-2 extends
  → Constraint: every new test is proven to go red with its fix reverted; new interop scenarios declare only the kernel capabilities they actually need.
- [ ] `docs/guide/l2tp.md` - native L2TPv2 LNS/LAC subsystem, CoA/DM transaction contract, and subscriber routes
- [ ] `docs/research/l2tpv2-ze-integration.md` - L2TPv2 integration design, companion to the protocol guide
- [ ] `docs/research/vpp-deployment-reference.md` - VPP deployment reference (`startup.conf` grammar and VPP lifecycle decisions)
- [ ] `docs/architecture/l2tp/subscriber-session-model.md` - design record of the subscriber namespace, bridge, and canonical `SubscriberID`
  → Decision: the bridge re-emits one-directionally, `l2tpevents` to `subevents`, for L2TP only; a lifecycle consumer subscribes to `subevents` alone, never to both topics.
  → Constraint: handler registries delegate (`l2tp.Register*` forwards to `subscriber.Register*`); event subscriptions do not.
- [ ] `docs/architecture/traffic/cos-dynamic.md` - dynamic CoS handler and `request class-of-service session` transaction
  → Constraint: `cos/shared_access.go` already captures and restores actual VLAN maps on final departure; this spec departs from the page's L2TP-only event subscription and `L2TPBirth`-only key decisions to include PPPoE via canonical `SubscriberID` and updates the page in the same work.
- [ ] `ai/rules/architecture.md` - boundary tracing and changed-wire census rules
  → Constraint: trace the producer of every value, not only the consumer; the boundary table and changed-wire census name every writer and reader.
- [ ] `ai/rules/rfc-compliance.md` - six enrolled RFCs are in scope
  → Decision: full compliance plus a tagged test proceeds without asking; only doing less requires asking.
  → Constraint: `check_coverage_ratchet` and `check_evidence_ratchet` mean a requirement may not lose a polarity or an evidence kind.
- [ ] `ai/rules/goroutine-lifecycle.md` - every timer goroutine must terminate when its session ends or is superseded
- [ ] `ai/rules/repo-maintenance.md` - generated rule files (`ai/rules/*.md`) must never be hand-edited; edit canonical sources under `ai/rules/points/` and run `./le ai rules update render` + `./le ai rules update condensed`

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc1661.md` - PPP LCP, Section 4.3 renegotiation
  → Constraint: RFC1661-4.3-1 is gated. Its Remaining cell in `docs/features/rfc-status.md` spells the `{gap}` count, which `check_gap_count_agreement` enforces.
- [ ] `rfc/short/rfc2865.md` - RADIUS, Sections 5.27 Session-Timeout and 5.28 Idle-Timeout
  → Constraint: Session-Timeout is the maximum seconds of service before termination, measured from first service activation, so renegotiation must not restart the deadline.
- [ ] `rfc/short/rfc2866.md` - RADIUS accounting, Sections 5.5 and 5.7
  → Constraint: RFC2866-5.5-1 (unique Acct-Session-Id) stays satisfied while D-6 breaks identity: one logical session owes one `Acct-Session-Id` and monotonic `Acct-Session-Time`.
- [ ] `rfc/short/rfc3954.md` - NetFlow v9, Section 5.1
  → Constraint: `sysUpTime` is milliseconds since the exporting device booted, and `FIRST_SWITCHED` / `LAST_SWITCHED` are expressed relative to it.
- [ ] `rfc/short/rfc5176.md` - RADIUS CoA/DM, Sections 3.1, 3.3, and 3.5
  → Constraint: CoA-ACK and Disconnect-ACK assert the change/teardown was applied; unapplied requests answer NAK.
- [ ] `rfc/short/rfc7296.md` - IKEv2, Sections 2.8.3 and 2.18
  → Constraint: RFC7296-2.18-2 (message counters reset to 0) is gated and already satisfied. D-8 aligns `CreatedAt` (new SA) and `EstablishedAt` (unchanged authentication) across both rekey directions.

**Key insights:** (minimal context to resume after compaction)
- The bridge's one-directional re-emit (`l2tpevents` -> `subevents`) lets each lifecycle consumer migrate onto `subevents` alone without double-firing L2TP.
- `l2tp.Observer` (`observer_events.go`) is L2TP-generation-scoped CQM (`L2TPBirth` + `EchoRTT`, and `pppoe/subsystem.go` explicitly keeps no CQM state), so it stays on `l2tpevents`.
- `session_metadata.go`, `pool/register.go`, `ppp/manager.go`, `route_observer.go`, `pppoe/redistribute.go`, `cos/shared_access.go` baseline capture, `authradius/coa_transaction.go` 2-phase confirmation, and the pinned `ze-vpp-deployment` image are already landed at HEAD.
- `Emit` returns a count of external plugin-process subscribers only and excludes in-process subscribers, so I-1 must be a static check in Go `internal/le/doc/wiring/wiring.go`.

## Current Behavior (MANDATORY)

**Source files read at HEAD:**
- [ ] `internal/component/l2tp/subscriber_bridge.go` - subscribes to `l2tpevents.{SessionUp,SessionDown,SessionIPAssigned,SessionIPReleased}` and `subevents.SessionAuthResult`; `onSessionUp` does read-modify-write for `Username` but unconditionally overwrites `sess.ActivatedAt = time.Now()`; `onSessionIPAssigned` now carries `IPv4Addr`/`IPv6Addr` into `subevents.SessionIPAssigned`
- [ ] `internal/component/l2tp/pppoe/subsystem.go` - `onSessionUp` already preserves `ActivatedAt` (`if !wasActive`) and loads `AuthMetadata.FilterID` for `GetShaperHandler()`; `Start` already attaches `StartSubscriberRouteObserver`; `withdrawSubscriber` tears down subscriber state on `EventLCPDown`; no RADIUS session/idle timeouts are started and no `DisconnectHandler` is registered
- [ ] `internal/component/l2tp/subscriber/events/events.go` - registers six topics (`SessionUp`, `SessionDown`, `SessionAuthResult`, `SessionIPAssigned`, `SessionIPReleased`, `SessionRateChange`); lacks `SessionParamsChanged` and `SessionCoSChange`
- [ ] `internal/component/l2tp/subscriber/registry.go` - `Add` overwrites `r.sessions[s.ID]` unconditionally without preserving an existing non-zero `ActivatedAt`
- [ ] `internal/component/l2tp/subscriber/handler_registry.go` - registers `AuthHandler`, `PoolHandler`, and `ShaperHandler`; lacks `DisconnectHandler`
- [ ] `internal/component/l2tp/session_metadata.go` - already keyed on canonical `subscriberID string` validated by `subscriber.RequireID`; `session_metadata_test.go` exists
- [ ] `internal/component/l2tp/session_timeout.go` - `startSessionTimeouts` cancels previous cancel funcs before starting new timers, but restarts the full `SessionTimeout` duration from `time.Now()`, is called only from `reactor_kernel.go`, and its `runIdleTimeout` treats a `0` return from `readIfaceRXBytes` as idle (with stale `rtnetlink` comment)
- [ ] `internal/component/l2tp/iface_stats_linux.go`, `internal/component/l2tp/iface_stats_other.go` - `readIfaceRXBytes` returns `uint64` (`0` on error or non-Linux) instead of `(uint64, error)`
- [ ] `internal/component/l2tp/reactor_kernel.go` - `handleLCPDown` tears down network lifetime, clears metadata, cancels timeouts, and emits `SessionDown`; `handleSessionUp` calls `startSessionTimeouts` unconditionally on every `EventSessionUp`
- [ ] `internal/component/l2tp/route_observer.go`, `internal/component/l2tp/pppoe/redistribute.go` - `StartSubscriberRouteObserver` already subscribes to `subevents.{SessionIPAssigned,SessionDown}` for both L2TP and PPPoE; `pppoe` redistribute source is already registered
- [ ] `internal/component/l2tp/ppp/manager.go`, `internal/component/l2tp/pppoe/subsystem_access.go` - `Driver.sessions` already keys by canonical `start.Attempt.SubscriberID()` (full `int` ifindex), and `pppoe/server.go:444` sets `TunnelID: uint16(s.ifIndex)` only as a diagnostic label, but `subsystem_access.go:63-70` still enforces `const discoveryIndexMax = 1<<16 - 1` refusing any access interface with `ifindex > 65535`
- [ ] `internal/component/l2tp/ppp/session_run.go`, `internal/component/l2tp/ppp/ncp.go` - on peer Configure-Request in `Opened`, `LCPActTLD` emits `EventLCPDown{NetworkPhase: true}` and `afterLCPOpen` re-runs `runAuthPhase()` and `runNCPPhase()` and re-emits `EventSessionUp`
- [ ] `internal/component/l2tp/plugins/pool/register.go` - already subscribes to `subevents.SessionDown`, keys `sessionAddrs`/`sessionPrefixes` by canonical `subscriberID`, and `lookupSessionUsername` takes `subscriberID`
- [ ] `internal/component/l2tp/plugins/authradius/acct.go` - `subscribeEventBus` binds `l2tpevents.{SessionIPAssigned,SessionIPReleased,SessionDown}` only and keys `sessions` on `sessionKey{birth: subscriberid.L2TPBirth, tunnelID, sessionID}`; `onSessionIPAssigned` already dedupes a repeated assignment on a live session
- [ ] `internal/component/l2tp/plugins/authradius/coa.go`, `coa_transaction.go` - `confirmCoA` already runs synchronous 2-phase prepare/apply/commit/rollback and returns CoA-NAK on failure, but passes only `(TunnelID, SessionID)` (no `SubscriberID`) to `request l2tp shaper rate`; `applySubscriberCoA` emits unsubscribed `subevents.SessionRateChange` and L2TP-only `l2tpevents.SessionCoSChange`; `handleDisconnect` returns Disconnect-NAK 504 for PPPoE
- [ ] `internal/component/l2tp/plugins/shaper/shaper.go`, `rate_transaction.go` - `shaper.go` subscribes to `subevents.SessionDown` only to clear `s.confirmed`/transactions, never inserts PPPoE into `s.sessions` (keyed by `sessionKey{tunnelID, sessionID}`), omits PPPoE from `show l2tp shaper`, never restores PPPoE qdisc on down, and listens for rate changes on `l2tpevents.SessionRateChange`
- [ ] `internal/plugins/cos/handler.go`, `session_state.go`, `session_transaction.go`, `shared_access.go` - `shared_access.go` already captures actual VLAN maps from the backend and restores them on final departure; `handler.go` still subscribes only to `l2tpevents.{SessionUp,SessionDown,SessionCoSChange}`, `session_state.go` keys `sessionStore` by `subscriberid.L2TPBirth`, and `session_transaction.go:211` rejects PPPoE because it requires `ParseL2TPBirth` and `SessionID != 0`
- [ ] `internal/component/l2tp/observer.go`, `observer_events.go` - L2TP-generation-scoped CQM observer keyed on `L2TPBirth` + `EchoRTT`; PPPoE explicitly records no CQM state (`pppoe/subsystem.go:472`), so this consumer intentionally stays on `l2tpevents`
- [ ] `internal/component/ike/engine/rekey.go` - `applyIKERekeyResponse` (lines 1254-1255) sets both `CreatedAt` and `EstablishedAt` to `time.Now()`; `respondIKERekey` (lines 1385, 1404-1405) sets both to `oldSA.EstablishedAt`
- [ ] `internal/plugins/flowexport/exporter.go`, `register.go` - `newExporter` (`exporter.go:108`) stamps `startTime: time.Now()`, invoked on both boot and reload by `configure` (`register.go:206`)
- [ ] `internal/component/vpp/config.go`, `yang/ze-vpp-conf.yang` - `parsePollSleepMs` (`config.go:447`) accepts whole milliseconds (`ms`) only
- [ ] `internal/plugins/iface/vpp/doctor_api.go` - `checkVPPVersion` (line 165, moved here from `internal/component/doctor/checks_linux.go`) checks only for substring `"vpp "` instead of parsing and checking the supported VPP version range
- [ ] `internal/le/test/deployment/vppevidence.go`, `vppiface.go`, `internal/le/build/vpp/deployment.go` - VPP deployment image is already pinned to `ze-vpp-deployment` (VPP v26.06 pinned source digest)
- [ ] `internal/le/doc/wiring/wiring.go`, `allowlist.go` - Go wiring check (`./le doc wiring`) checks exported symbol references with `wiringAllowlist`; has no emit-without-subscribe predicate yet
- [ ] `internal/test/fixture/tunnel_fixture_l2tp_ppp.go` - Go L2TP/PPP scripted peer fixture used by `test/l2tp/radius-acct-wire.ci`; does not yet have a mid-session LCP Configure-Request renegotiation mode

**Behavior to preserve:**
- L2TP lifecycle behaviour end to end, including tunnel order tickets, `NetworkLifetime` counter capture, and every existing L2TP `.ci`, interop and unit test.
- `LCPDoTransition`'s transition table and `TestRFC1661RenegotiateOnConfigureRequestInOpened` (RFC1661-4.3-1).
- `show l2tp shaper`, `show subscriber summary`, and `show ipsec` output shapes.
- The transport-generic handler registries (`RegisterPoolHandler`, `RegisterAuthHandler`, `RegisterShaperHandler`) and their delegation from `l2tp.Register*`.
- Dataplane reprogramming on renegotiation: `onNCPOpened` must keep calling `AddAddressP2P` and `SetMTU`/MRU updates so a changed MRU or IPCP address still lands.
- `poll-sleep` configurations already written in whole milliseconds (`ms`) must keep parsing and producing byte-identical `startup.conf` output.
- Existing `confirmCoA` 2-phase atomic prepare/apply/commit/rollback semantics (preserving RFC 5176 discrimination records) and `cos/shared_access.go` native VLAN baseline capture/restore.

**Behavior to change:**
- Mid-session LCP renegotiation on an already-established PPP session keeps the authenticated session alive (no teardown of accounting, metadata, or timeouts, and no second RADIUS Access-Request), emits `subevents.SessionParamsChanged` instead of re-emitting `SessionUp`, and still applies changed LCP/NCP dataplane parameters.
- `radiusAcct`, `shaper`, and `cos` migrate completely onto `subevents` and canonical `SubscriberID` keys so PPPoE gains full RADIUS accounting, shaper state/CoA/teardown restoration, and CoS assignment/CoA/teardown restoration.
- `subscriber_bridge.go` and `subscriber/registry.go` preserve an existing non-zero `ActivatedAt` across repeat updates.
- `session_timeout.go` preserves the original `Session-Timeout` deadline across renegotiation, runs for PPPoE sessions as well as L2TP (D-11), and treats a failed `readIfaceRXBytes` as a logged read error rather than idle (D-12).
- `authradius/coa.go` and `coa_transaction.go` pass canonical `SubscriberID` to both CoA owners, emit `subevents.SessionCoSChange` alongside `subevents.SessionRateChange`, and route PPPoE Disconnect-Request through the new `subscriber.DisconnectHandler` (I-4).
- `ike/engine/rekey.go` sets `CreatedAt: time.Now()` and `EstablishedAt: oldSA.EstablishedAt` in both `applyIKERekeyResponse` and `respondIKERekey` (D-8).
- `flowexport/register.go` and `exporter.go` preserve the boot `startTime` epoch across config reloads (D-9).
- `vpp/config.go` and `ze-vpp-conf.yang` accept microsecond `poll-sleep` values (`0us..100000us`), and `iface/vpp/doctor_api.go` validates the parsed VPP version against the supported range (I-3).
- `internal/le/doc/wiring/wiring.go` fails when an emitted EventBus topic has no subscriber unless listed in `wiringAllowlist` (I-1); `docs/architecture/testing/interop.md` and `ai/rules/points/principles/directives/your-blast-radius-is-what-you-can-reach.md` gain the I-2 rule rows.

### Consumer migration map

Status of every lifecycle consumer at HEAD:

| Consumer | Topics held at HEAD | Migration state / remaining work |
|----------|---------------------|----------------------------------|
| `poolPlugin.setEventBus` (`pool/register.go`) | `subevents.SessionDown`, `PoolAnswerUndelivered` | **Already migrated** (D-1, D-10 landed). Keyed by canonical `subscriberID`. Only `test/pppoe/pool-release-cycle.ci` remains. |
| `StartSubscriberRouteObserver` (`l2tp/route_observer.go`) | `subevents.SessionIPAssigned`, `subevents.SessionDown` | **Already migrated** (D-14 landed and verified). Serves both L2TP and PPPoE with independent `l2tp` and `pppoe` redistribute sources. |
| `radiusAcct.subscribeEventBus` (`authradius/acct.go`) | `l2tpevents.{SessionIPAssigned,SessionIPReleased,SessionDown}` | **Open (D-2):** migrate to `subevents.{SessionIPAssigned,SessionIPReleased,SessionDown}` and key `sessions` by canonical `SubscriberID` (keeping L2TP `NetworkLifetime` counter capture when `Session.L2TPBirth` is present). |
| `shaper.setEventBus` (`shaper/shaper.go`, `rate_transaction.go`) | `l2tpevents.{SessionUp,SessionDown,SessionRateChange}` + `subevents.SessionDown` (cleanup only) | **Open (D-3, D-4):** migrate completely to `subevents.{SessionUp,SessionDown,SessionRateChange}`, key `s.sessions` and `rate_transaction.go` by canonical `SubscriberID` (keeping `TunnelID`/`SessionID` as display fields), record PPPoE on `SessionUp`, restore qdisc on `SessionDown`, and expose PPPoE in `show l2tp shaper`. |
| `newCosHandler` (`cos/handler.go`, `session_state.go`, `session_transaction.go`) | `l2tpevents.{SessionUp,SessionDown,SessionCoSChange}` | **Open (D-3, D-4):** migrate to `subevents.{SessionUp,SessionDown,SessionCoSChange}` (adding `subevents.SessionCoSChange` in Phase 1) and key `sessionStore` and `session_transaction.go` by canonical `SubscriberID` instead of `L2TPBirth`. |
| `l2tp.Observer` (`l2tp/observer_events.go`) | `l2tpevents.{SessionUp,SessionDown,SessionIPAssigned,EchoRTT}` | **Stays on `l2tpevents` (by design):** L2TP-generation-scoped CQM keyed on `L2TPBirth` + `EchoRTT`; PPPoE explicitly holds no CQM state (`pppoe/subsystem.go:472`). |
| `subscriberBridge.subscribe` (`l2tp/subscriber_bridge.go`) | `l2tpevents.{SessionUp,SessionDown,SessionIPAssigned,SessionIPReleased}` -> `subevents.*` | **Producer (L2TP bridge):** fix `onSessionUp` to preserve existing non-zero `ActivatedAt` (D-5) and forward `SessionParamsChanged`. |

## Existing Mechanisms (MANDATORY)

Every fix here extends a mechanism Ze already has; none adds a second one beside it. Already-landed pieces of D-1, D-3, D-4, D-10, D-13, D-14, I-3, and I-4 are recorded here as existing baselines rather than re-implemented.

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Subscriber event namespace | `internal/component/l2tp/subscriber/events/events.go` (`SessionUp`, `SessionDown`, `SessionAuthResult`, `SessionIPAssigned`, `SessionIPReleased`, `SessionRateChange`); `docs/architecture/l2tp/subscriber-session-model.md` | Transport-generic topics emitted by PPPoE and re-emitted by `subscriberBridge` for L2TP; consumed today by `pool`, `route_observer`, and partial `shaper` cleanup | extends: adds `SessionParamsChanged` and `SessionCoSChange` and migrates `authradius/acct`, `shaper`, and `cos` onto `subevents` alone |
| L2TP to subscriber bridge | `internal/component/l2tp/subscriber_bridge.go` (`subscriberBridge.subscribe`, `onSessionUp`, `onSessionIPAssigned`, `onSessionDown`) | Re-emits `l2tpevents` session topics onto `subevents` for L2TP, carrying assigned IPv4/IPv6 addresses; `onSessionUp` still resets `ActivatedAt` | extends: preserves existing non-zero `ActivatedAt` on repeat events (D-5) and forwards `SessionParamsChanged` |
| Subscriber session registry | `internal/component/l2tp/subscriber/registry.go` (`Registry.Add`, `DefaultRegistry`) | Stores live L2TP and PPPoE `*Session` entries keyed by canonical `ID`; `Add` currently overwrites existing entries unconditionally | extends: preserves an existing entry's non-zero `ActivatedAt` across repeat updates on the same `ID` so renegotiation cannot reset activation time (D-5, AC-6) |
| PPP LCP/NCP renegotiation & teardown path | `internal/component/l2tp/ppp/session_run.go` (`LCPActTLD`, `afterLCPOpen`), `internal/component/l2tp/ppp/ncp.go` (`onNCPOpened`), `internal/component/l2tp/reactor_kernel.go` (`handleLCPDown`) | On a peer Configure-Request in `Opened`, `LCPActTLD` emits `EventLCPDown{NetworkPhase: true}`, tearing down metadata, timeouts, and accounting via `handleLCPDown`, and `afterLCPOpen` re-runs `runAuthPhase()` and `runNCPPhase()` | extends: latches first activation so mid-session LCP renegotiation preserves auth, accounting, metadata, and timeouts and emits `EventSessionParamsChanged`, while keeping `SetMTU`/`AddAddressP2P` updates and full teardown on terminal LCP Closed/Stopped (D-6, D-7, AC-17) |
| Transport-generic handler registries | `internal/component/l2tp/subscriber/handler_registry.go` (`AuthHandler`, `PoolHandler`, `ShaperHandler`) | Shared auth, pool, and shaper registries called by both L2TP and PPPoE | extends: adds `DisconnectHandler` for RADIUS Disconnect-Request teardown of PPPoE sessions (I-4, AC-15) |
| Canonical subscriber ID & PPP driver keying (D-10 landed; D-13 partial) | `internal/component/l2tp/session_metadata.go` (`StoreSessionMetadata`, `LoadSessionMetadata`, `ClearSessionMetadata`), `internal/component/l2tp/ppp/manager.go` (`Driver.sessions`), `internal/component/l2tp/pppoe/subsystem_access.go` (`prepareAccessConfiguration`), `internal/component/l2tp/subscriber/id_required.go` (`RequireID`) | `session_metadata.go` and `ppp/manager.go` already key on canonical `SubscriberID` (embedding full `int` ifindex for PPPoE and `L2TPBirth` for L2TP), but `pppoe/subsystem_access.go:63-70` still refuses `ifindex > 65535` via `discoveryIndexMax` | extends: removes the obsolete `discoveryIndexMax` cap in `subsystem_access.go` (D-13), adds cross-transport collision and >16-bit ifindex unit proofs (AC-11, AC-22), and adopts the same canonical `SubscriberID` key in `acct.go`, `shaper`, and `cos` |
| Pool release on `subevents.SessionDown` (D-1 landed) | `internal/component/l2tp/plugins/pool/register.go` (`setEventBus`, `onSessionDown`, `lookupSessionUsername`) | Already subscribes to `subevents.SessionDown`, keys `sessionAddrs`/`sessionPrefixes` by canonical `subscriberID`, and releases PPPoE and L2TP allocations once | extends: keeps implementation unchanged; adds end-to-end functional test `test/pppoe/pool-release-cycle.ci` (AC-1) |
| Subscriber route observer & PPPoE Framed-Route / `redistribute pppoe` (D-14 & I-4 Framed-Route landed) | `internal/component/l2tp/route_observer.go` (`StartSubscriberRouteObserver`), `internal/component/l2tp/pppoe/redistribute.go`, `internal/component/l2tp/pppoe/subsystem.go`, `test/pppoe/redistribute-subscriber-route.ci` | Approved 2026-11-19 and verified at HEAD (80/80 guest iterations): observes `subevents.{SessionIPAssigned,SessionDown}` for both transports, stages PPPoE Framed-Routes per auth round (`807a87e415`), and exposes independent `pppoe` and `l2tp` redistribute sources | does not apply (already landed and verified for AC-16 and AC-23); preserved unchanged across renegotiation and remaining consumer migrations |
| Synchronous 2-phase CoA confirmation (D-3 partial landed) | `internal/component/l2tp/plugins/authradius/coa_transaction.go` (`confirmCoA`), `shaper/rate_transaction.go`, `cos/session_transaction.go` | Prepares, applies, commits, or rolls back shaper and CoS changes synchronously via hidden commands before sending CoA-ACK, answering CoA-NAK on failure; currently accepts L2TP coordinates only | extends: passes canonical `SubscriberID` to `request l2tp shaper rate` and allows canonical `SubscriberID` (PPPoE and L2TP) in both `rate_transaction.go` and `session_transaction.go` (D-3, AC-3, AC-4) |
| Dynamic CoS baseline capture & restore (D-4 partial landed) | `internal/plugins/cos/shared_access.go` (`captureSharedAccess`, `departSharedSession`) | Already reads actual VLAN QoS maps from the interface backend on first admission and restores the captured baseline on final departure (the old `nil resolveStatic` bug is gone) | extends: migrating `cos/handler.go`, `session_state.go`, and `session_transaction.go` to `subevents` + canonical `SubscriberID` extends this existing restoration mechanism to PPPoE (AC-18) |
| RADIUS session & idle timers | `internal/component/l2tp/session_timeout.go` (`startSessionTimeouts`, `runSessionTimeout`, `runIdleTimeout`), `iface_stats_linux.go`, `iface_stats_other.go` | Cancels previous timer contexts before restart, but resets `Session-Timeout` deadline on renegotiation, runs for L2TP only, and treats `readIfaceRXBytes` errors (`0`) as idle | extends: preserves original deadline across renegotiation (D-7), attaches timers for PPPoE keyed by canonical `SubscriberID` (D-11), and returns `(uint64, error)` from `readIfaceRXBytes` so failed reads log and keep the session up (D-12) |
| IKEv2 SA rekey handlers | `internal/component/ike/engine/rekey.go` (`applyIKERekeyResponse`, `respondIKERekey`) | Creates the replacement IKE SA on initiator and responder rekey; currently disagrees on `CreatedAt` vs `EstablishedAt` | extends: aligns both paths to `CreatedAt: time.Now()` and `EstablishedAt: oldSA.EstablishedAt` (D-8) |
| Flow export configuration & NetFlow v9 epoch | `internal/plugins/flowexport/register.go` (`configure`), `exporter.go` (`newExporter`) | Builds exporter on boot and reload; stamps fresh `startTime` every time | extends: captures boot `startTime` once on plugin init/first start and passes it to `newExporter` across reloads (D-9) |
| VPP `startup.conf` generator, pinned deployment image (I-3 partial landed), and VPP doctor checks | `internal/component/vpp/config.go` (`parsePollSleepMs`), `ze-vpp-conf.yang`, `internal/plugins/iface/vpp/doctor_api.go` (`checkVPPVersion`), `internal/le/build/vpp/deployment.go` (`DeploymentImageName`) | VPP evidence image is already pinned to `ze-vpp-deployment` (VPP v26.06); `poll-sleep` accepts `ms` only; `checkVPPVersion` checks only substring `"vpp "` | extends: adds microsecond `poll-sleep` (`us`) support and real version-range validation in `checkVPPVersion`; keeps pinned `ze-vpp-deployment` unchanged (I-3) |
| Go wiring checker | `internal/le/doc/wiring/wiring.go`, `allowlist.go` (`./le doc wiring`) | Checks added exported symbols for non-test references using `wiringAllowlist` | extends: adds static emit-without-subscribe EventBus topic check backed by `wiringAllowlist` (I-1) |
| Scripted L2TP/PPP test peer | `internal/test/fixture/tunnel_fixture_l2tp_ppp.go` (`tunnelL2TPRadiusAccountingPeer`) | Drives real kernel PPPoL2TP, CHAP, IPCP, echo replies, and RADIUS accounting assertions | extends: adds mid-session LCP Configure-Request renegotiation mode (AC-19) |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | The core knows nothing, and everything registers | the door every change reads | "Code reaches a gated feature only through a registry or a seam that the feature fills from its own `init()`." | agrees | Every new surface (`DisconnectHandler`, `SessionParamsChanged`, `SessionCoSChange`) registers through existing registries and typed EventBus topics with no hardcoded transport switch in core packages. |
| `ai/doors/access-security.md` | Boundaries | the door covering `internal/component/l2tp/...` and `internal/component/ike/engine/rekey.go` | "Subscriber auth, pool and shaper reach a session through `l2tp/subscriber` handler registries" | agrees | Adds `DisconnectHandler` in `l2tp/subscriber/handler_registry.go` and keeps `ppp` a leaf package with no component imports. |
| `ai/doors/system-plugins.md` | Boundaries | the door covering `internal/plugins/cos/...`, `internal/plugins/flowexport/...`, and `internal/plugins/iface/vpp/doctor_api.go` | "Reach the engine through the SDK, the bus, or a command, never through a Go call into a component's state (`docs/architecture/plugin/plugin-system.md`, "The process boundary")." | agrees | `cos` consumes `subevents` and hidden command transactions (`request class-of-service session`), `flowexport` keeps its boot epoch internal to the plugin, and `iface/vpp` owns its doctor check inside its plugin folder. |
| `ai/doors/forwarding.md` | Boundaries | the door covering `internal/component/vpp/config.go` and `ze-vpp-conf.yang` | "One writer per data plane." | silent | This spec only widens `vpp` `poll-sleep` microsecond parsing and `checkVPPVersion` range checks; it does not change FIB writer registration, route installation, or data-plane selection. |
| `ai/doors/le-tooling.md` | Boundaries | the door covering `internal/le/doc/wiring/...`, `internal/test/fixture/...`, and `test/.../*.ci` | "Never import `internal/le` from product code" | agrees | The I-1 emit-without-subscribe guard and its unit tests live entirely under `internal/le/doc/wiring/` and all new `.ci` tests follow the Go fixture and `option=needs-linux` rules. |
| `docs/research/l2tpv2-ze-integration.md` | 6.3 l2tp-shaper (traffic shaping) | the `// Design:` page of `ppp/session_run.go`, `ppp/ncp.go`, `reactor_kernel.go`, `authradius/{acct,coa}.go`, `shaper/{shaper,rate_transaction}.go`, and modified by this spec | "The listener ACKs only after every requested owner confirms its apply. Post-commit rate events remain notifications." | agrees | Extends the confirmed rate-change and shaper session path to PPPoE over the unified `subevents.SessionRateChange` and `request l2tp shaper rate` transaction keyed by canonical `SubscriberID`. |
| `docs/research/l2tpv2-ze-integration.md` | 6.3 l2tp-shaper (traffic shaping) | modified by this spec (D-4) | "Subscribe to `(l2tp, session-up)` to apply initial shaping." | departs | D-4 migrates `l2tp-shaper` from `l2tpevents` (`l2tp, session-up`) onto `subevents.SessionUp` keyed by canonical `SubscriberID` so both L2TP and PPPoE share one shaper session table; this spec updates `l2tpv2-ze-integration.md` accordingly. |
| `docs/research/l2tpv2-ze-integration.md` | 6.3 l2tp-shaper (traffic shaping) | modified by this spec (D-4) | "Clean up TC rules on `(l2tp, session-down)`." | departs | D-4 migrates `l2tp-shaper` teardown cleanup onto `subevents.SessionDown` alone so both L2TP and PPPoE restore original qdiscs on disconnect; this spec updates `l2tpv2-ze-integration.md` accordingly. |
| `docs/research/l2tpv2-ze-integration.md` | What ze handles in userspace (control plane) | modified by this spec (D-6, AC-17) | "An LCP restart clears the cached decision." | departs | D-6/AC-17 preserves an already-established session's authentication across mid-session LCP renegotiation instead of clearing the decision and re-authenticating; this spec updates `l2tpv2-ze-integration.md` accordingly. |
| `docs/research/l2tpv2-ze-integration.md` | 11.3 PPP session ownership | modified by this spec (D-6, D-7, AC-17) | "Leaving LCP Opened first disconnects the PPP channel and removes its NCP resources. EventLCPDown then waits for the transport's network-lifetime cleanup acknowledgment before the accompanying LCP responses are sent. Authentication and NCP negotiate anew on the same transport." | departs | On mid-session LCP renegotiation of an already-established session, Ze keeps the authenticated session, network lifetime, metadata, accounting, and timeouts alive without disconnecting to re-authenticate (while still applying updated MRU/IPCP parameters); terminal LCP Closed/Stopped still performs full teardown. This spec updates `l2tpv2-ze-integration.md` accordingly. |
| `docs/architecture/l2tp/bng-1-radius-attributes.md` | Decisions | the `// Design:` page of `session_timeout.go`, `iface_stats_linux.go`, `iface_stats_other.go`, and anchored to `ppp/`, `authradius/`, `shaper/`, `cos/` | "In PPP, LCP opens before authentication, so the Access-Accept arrives after the link is agreed: renegotiating LCP would restart authentication." | departs | D-6/AC-17 changes mid-session LCP renegotiation on an already-authenticated session to preserve its authentication rather than restarting it (Framed-MTU still lowers the interface MTU rather than renegotiating LCP); this spec updates `bng-1-radius-attributes.md` accordingly. |
| `docs/architecture/l2tp/bng-1-radius-attributes.md` | Decisions | the `// Design:` page of `session_timeout.go`, `iface_stats_linux.go`, `iface_stats_other.go`, and modified by this spec | "The non-Linux build returns 0, so the timer fires unconditionally there." | departs | D-12 changes `readIfaceRXBytes` to return `(uint64, error)` on both Linux and non-Linux so a failed counter read logs a warning and keeps the session running instead of tearing it down as idle; this spec updates `bng-1-radius-attributes.md` accordingly. |
| `docs/architecture/l2tp/bng-1-radius-attributes.md` | Decisions | anchored to `authradius/acct.go` and modified by this spec | "Records are keyed by the session's Birth plus its wire labels." | departs | D-2 migrates accounting records from L2TP-only `L2TPBirth` keys to canonical `SubscriberID` keys (which embed `L2TPBirth` for L2TP and full `ifIndex`+SID for PPPoE) so PPPoE and L2TP share one collision-free accounting map; this spec updates `bng-1-radius-attributes.md` accordingly. |
| `docs/architecture/l2tp/cpe-1-pppoe-client.md` | Decisions | anchored to `internal/component/l2tp/ppp/session_run.go` | "One negotiator answers a peer, whichever role ze is in." | silent | This page documents the customer-premises PPPoE client dialer; this spec changes only server-side post-Opened renegotiation handling (`LCPActTLD` / `afterLCPOpen`) and leaves client option negotiation untouched. |
| `docs/architecture/l2tp/subscriber-session-model.md` | Decisions | the `// Design:` page of `subscriber/events/events.go`, `subscriber/handler_registry.go`, `subscriber/registry.go`, `subscriber_bridge.go`, and modified by this spec | "A lifecycle consumer subscribes to the subscriber topic alone, never to both topics." | agrees | Completes the migration of `authradius/acct`, `shaper`, and `cos` onto `subevents` alone, preserves `ActivatedAt` across repeat updates, and keeps `l2tp.Observer` on `l2tpevents` because CQM is L2TP-specific. |
| `docs/architecture/l2tp/subscriber-session-model.md` | The order of one L2TP session's notifications | modified by this spec (D-6, D-7, AC-17) | "Session-down that ends a network lifetime (LCP down in the network phase)" | departs | On mid-session LCP renegotiation of an already-established session, `handleLCPDown` does not invoke `notifyNetworkLifetimeDown` / `ClearSessionMetadata` to end the network lifetime and tear down accounting/timeouts; instead renegotiation keeps the session alive and emits `SessionParamsChanged` (while terminal LCP down keeps the ticketed teardown path). This spec updates `subscriber-session-model.md` accordingly. |
| `docs/architecture/l2tp/subscriber-session-model.md` | Traps this code exists to avoid | modified by this spec (D-5, D-6, AC-17) | "LCP-down clears the accepted username and the transport's network-lifetime identity, so replacement admission cannot reuse the previous subscriber." | departs | On mid-session LCP renegotiation of an already-established session, Ze preserves the accepted username and logical session rather than clearing them and running replacement admission (terminal LCP Closed/Stopped still clears everything); this spec updates `subscriber-session-model.md` accordingly. |
| `docs/architecture/l2tp/subscriber-session-model.md` | Traps this code exists to avoid | modified by this spec (D-6, D-7, AC-17) | "LCP-down ends the network lifetime without ending the transport. Each transport's route observer withdraws that lifetime's routes before the transport acknowledges LCP-down. Both transports also release the old allocation and authorization state before acknowledgment. PPP waits for it before it can start replacement authentication." | departs | Mid-session LCP renegotiation keeps authorization, pool allocation, routes, and timeouts alive across renegotiation and emits `SessionParamsChanged` instead of ending the network lifetime and running replacement authentication; this spec updates `subscriber-session-model.md` accordingly. |
| `docs/architecture/l2tp/subscriber-session-model.md` | Traps this code exists to avoid | modified by this spec (D-6, AC-7) | "LCP-down stops that session before replacement authentication." | departs | Mid-session LCP renegotiation keeps the active accounting session running with its original `Acct-Session-Id` and monotonic `Acct-Session-Time` instead of stopping it; this spec updates `subscriber-session-model.md` accordingly. |
| `docs/architecture/l2tp/subscriber-session-model.md` | Traps this code exists to avoid | modified by this spec (D-2) | "PPPoE accounting is not connected to subscriber events." | departs | D-2 connects `authradius/acct.go` to `subevents` so PPPoE sessions emit full Start, Interim-Update, and Stop accounting records; this spec updates `subscriber-session-model.md` accordingly. |
| `docs/architecture/l2tp/bng-5-pppoe.md` | Decisions | the `// Design:` page of `internal/component/l2tp/pppoe/subsystem.go` and `subsystem_access.go`, and modified by this spec (D-13) | "Every prepared access must name that namespace and a distinct Linux index within the shared PPP driver's 16-bit tunnel-key range; discovery dispatches by the index from the receive call." | departs | D-13 removes the obsolete 16-bit `discoveryIndexMax` (`1<<16 - 1`) cap in `pppoe/subsystem_access.go:63-70` because the shared PPP driver (`ppp/manager.go`), pool, and metadata store now key by canonical `SubscriberID` with full `int` ifindex, allowing distinct Linux indexes above 65535 (AC-22); this spec updates `bng-5-pppoe.md` accordingly. |
| `docs/architecture/l2tp/bng-5-pppoe.md` | Design decisions | decision of 2026-11-23 on `docs/architecture/l2tp/bng-5-pppoe.md` | "D-1, a VPP API loss with the process alive, is resolved by the VPP lifecycle rules: a live VPP that Ze cannot adopt is killed and restarted, so its exit proof always arrives and retires the retained owners" | agrees | This spec does not alter PPPoE's VPP process-lease or VPP API-loss recovery path; PPPoE session timeout and Disconnect-Request hooks act only on live PPP sessions. |
| `docs/architecture/l2tp/bng-5-pppoe.md` | Design decisions | decision of 2026-11-23 on `docs/architecture/l2tp/bng-5-pppoe.md` | "When Ze adopts the VPP, retained owners are read back by exact identity over the new connection of the same pidfd-proven process, and cleaned there (option C)" | agrees | This spec preserves PPPoE's VPP retained-owner readback and option C cleanup path unchanged. |
| `docs/architecture/l2tp/bng-5-pppoe.md` | Design decisions | decision of 2026-11-23 on `docs/architecture/l2tp/bng-5-pppoe.md` | "Adoption happens only inside the single VPP loss timer: a reconnect to the same pidfd-proven process whose full readback matches (option C) keeps the sessions. A declared loss is never adopted: Ze kills the process through the retained pidfd, waits for its exit proof, restarts and reconfigures VPP as at boot, and PPPoE tears its sessions down per OD-2 and lets subscribers redial. The provider holds the process lease, which gives the identity proof, the kill and the exit proof. This narrows the row "When Ze adopts the VPP" to the single timer, and the D-1 row's "cannot adopt" to every declared loss" | agrees | Superseded by the two-form VPP recovery row below; this spec does not touch VPP loss detection, adoption, or OD-2 teardown. |
| `docs/architecture/l2tp/bng-5-pppoe.md` | Design decisions | decision of 2026-11-23 on `docs/architecture/l2tp/bng-5-pppoe.md` | "D-1 (a VPP API loss with the process alive) resolves on the VPP lifecycle's two recovery forms. Form (1), inside the reconnect-and-verify limit that follows the failed alive test: the same pidfd-proven process answering with the data Ze configured is enough to adopt it, and the PPPoE sessions are kept. The retained owners are read back by exact identity over the new connection of that process and cleaned there (option C), as part of the background reconciliation; a mismatch it finds is repaired, never treated as a loss. Form (2), a forced restart: Ze kills the process through the retained pidfd, waits for its exit proof, restarts and reconfigures VPP as at boot, and PPPoE tears its sessions down per OD-2 and lets subscribers redial. The provider holds the process lease, which gives the identity proof, the kill and the exit proof. This supersedes the row "Adoption happens only inside the single VPP loss timer" (the single timer, adoption on a full readback match, and "a declared loss is never adopted"), and narrows the D-1 row and the row "When Ze adopts the VPP" to these two forms" | agrees | This spec preserves both VPP recovery forms and PPPoE's OD-2 teardown unchanged; RADIUS timers and Disconnect-Request hook into ordinary PPPoE session teardown. |
| `docs/architecture/traffic/cos-dynamic.md` | Subscribe to the L2TP session event, not the subscriber event | the `// Design:` page of `internal/plugins/cos/{handler,session_state,session_transaction}.go` and anchored to `authradius/coa.go` | "The lifecycle producer supplies its cached canonical subscriber ID for the `AuthMetadata.CoSProfile` lookup and its packed Birth for CoS ownership." | departs | D-3 and D-4 migrate `cos` onto `subevents.{SessionUp,SessionDown,SessionCoSChange}` and key `sessionStore` and `session_transaction.go` by canonical `SubscriberID` string (which already encodes `L2TPBirth` for L2TP and full `ifIndex`+SID for PPPoE) so PPPoE subscribers participate in CoS assignment, teardown restoration, and CoA transactions; this spec updates `cos-dynamic.md` accordingly. |
| `docs/architecture/traffic/cos-dynamic.md` | Session identity and shared access ownership are separate | modified by this spec (D-4) | "The package-level `sessionStore` is a `sync.Map` keyed by `sessionKey`, the original packed session Birth." | departs | D-4 re-keys `sessionStore` (`internal/plugins/cos/session_state.go`) from L2TP-only `L2TPBirth` to canonical `SubscriberID` string (embedding `L2TPBirth` for L2TP and full `ifIndex`+SID for PPPoE) so PPPoE sessions share the same CoS ownership and baseline restoration; this spec updates `cos-dynamic.md` accordingly. |
| `docs/architecture/traffic/cos-dynamic.md` | CoA waits for the owner to apply its maps | modified by this spec (D-3) | "The decoder parses the canonical ID once and retains its Birth as the ownership key." | departs | D-3 changes `internal/plugins/cos/session_transaction.go` to validate and retain the canonical `SubscriberID` via `subscriber.RequireID` instead of calling `ParseL2TPBirth`, admitting both L2TP and PPPoE sessions to the synchronous CoA transaction path; this spec updates `cos-dynamic.md` accordingly. |
| `docs/guide/l2tp.md` | Authentication | the `// Design:` page of `internal/component/l2tp/plugins/authradius/coa_transaction.go` and modified by this spec | "A CoA-Request prepares every requested control before changing either one. The listener then asks the shaper and class-of-service owners to apply through the command dispatcher and waits for their backend results before sending an ACK." | agrees | Preserves the synchronous 2-phase prepare/apply/commit/rollback transaction and extends its `subscriber-id` identity to `request l2tp shaper rate` and `request class-of-service session` for PPPoE as well as L2TP. |
| `docs/guide/l2tp.md` | PPP negotiation | modified by this spec (D-6, AC-17) | "Leaving LCP Opened ends the previous PAP decision and both NCP lifetimes. Ze disconnects an attached channel from its PPP unit before replacement authentication and NCP, so control frames return to the channel reader." | departs | On mid-session LCP renegotiation of an already-established session, Ze keeps the authenticated session and network lifetime intact without running replacement authentication (while still applying updated LCP/NCP dataplane parameters); this spec updates `docs/guide/l2tp.md` accordingly. |
| `docs/guide/l2tp.md` | Authentication | modified by this spec (D-2) | "PPPoE accounting remains unconnected to subscriber events." | departs | D-2 connects `authradius/acct.go` to `subevents` so PPPoE sessions emit full Start, Interim-Update, and Stop accounting records; this spec updates `docs/guide/l2tp.md` accordingly. |
| `docs/guide/l2tp.md` | Authentication | modified by this spec (I-4, AC-15) | "A matching Disconnect-Request for a PPPoE session still gets NAK 504, because PPPoE teardown from a Disconnect-Request is not connected yet." | departs | I-4 wires `subscriber.DisconnectHandler` from `pppoe/subsystem.go` so a matching PPPoE Disconnect-Request sends PADT, completes teardown with cause Admin-Reset, and answers Disconnect-ACK; this spec updates `docs/guide/l2tp.md` accordingly. |
| `docs/guide/l2tp.md` | Authentication | modified by this spec (D-2) | "The accounting owner binds its issued ID to the original Birth. A numeric tunnel/session pair cannot supply an accounting identity for a replacement." | departs | D-2 binds accounting records to the canonical `SubscriberID` string (which embeds `L2TPBirth` for L2TP and full `ifIndex`+SID for PPPoE), preserving replacement isolation for L2TP while extending accounting identity to PPPoE; this spec updates `docs/guide/l2tp.md` accordingly. |
| `docs/architecture/appliance/kernel-profiles.md` | PPP original-resource ABI | anchored to `internal/component/l2tp/plugins/shaper/shaper.go` | "The global result is for capability checking only. It never authorizes a mutation or replaces the exact live creator, channel or transport probes below." | silent | This spec changes only shaper session keying (`SubscriberID`) and `subevents` subscription; it does not touch Linux kernel profiles or the PPP original-resource ioctl ABI. |
| `docs/architecture/core-design.md` | Command engine and composition roots | the `// Design:` page of `internal/le/doc/wiring/wiring.go` and `allowlist.go` | "Keep each action callable as Go. Go callers invoke that function directly, and repository workflows invoke `./le <area> <action>`." | agrees | Extends the existing Go `docwiring` package callable via `./le doc wiring` and tested directly from Go `wiring_emit_test.go`. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 on `docs/architecture/core-design.md` | "A commit fails when it configures a feature that programs only the Linux kernel, or that reads only what the Linux kernel sees of the traffic, while the interfaces run on VPP. Two host-side uses stay accepted, so that mixed setups remain possible once VRFs are added: a `fib kernel` block beside a `fib vpp` block, which programs the Linux route table for the router's own traffic, and `sysctl`, which sets the Linux kernel's own tunables. A `fib kernel` block with no `fib vpp` block beside it is refused, for now." | agrees | This spec does not weaken or bypass any kernel-vs-VPP data-plane mismatch check. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 on `docs/architecture/core-design.md` | "A configuration that holds such a mismatch is refused at daemon start as well as at commit, reload and `ze config validate`, by one verdict that is the same at each." | agrees | Superseded at daemon start by the 2026-11-26 row below; this spec changes no data-plane verdict path. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 on `docs/architecture/core-design.md` | "The VPP firewall verifier, not a YANG annotation, decides whether `connection-state` and `mark-set` are accepted on VPP. The `ze:backend` annotation of `flow-offload` is deleted with theirs; `flow-offload` stays refused on VPP because VPP has no flowtable and the `flowtable` list keeps its `ze:backend "kernel"` annotation." | agrees | This spec does not touch firewall verifiers or `ze:backend` annotations. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 on `docs/architecture/core-design.md` | "Every data-plane selector names the plane it programs, `kernel` or `vpp`: `interface backend`, `firewall backend` and `traffic control backend` take `kernel` where they took `netlink`, `nft` and `tc`, and each defaults to `kernel`; the `fib` blocks already name `kernel` and `vpp`. The plane a registration declares and the value a `ze:backend` annotation names are the same word. A second implementation on the same plane, such as eBPF, takes a value of its own when it exists. In Thomas's words (voice-transcribed): "should tc and netfilter be renamed to kernel? so we have vpp and kernel?", then "please do it"." | agrees | This spec introduces no new data-plane selector and keeps existing `kernel`/`vpp` naming intact. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 on `docs/architecture/core-design.md` | "BGP egress next-hop rewriting belongs exclusively in the export filter chain, not under session configuration (`session { next-hop }`). Ze automatically appends `next-hop auto` at the end of the export chain when no earlier export filter has already set a next-hop outcome (`self`, `unchanged`, or an explicit address), so ordinary eBGP rewrites to self while iBGP and route-server clients preserve the received next hop. In Thomas's words (voice-transcribed): "I would think that all the rewrite should be in the prefix export and not under configuration. I believe we should automatically add to the export prefix list the auto at the end of the list if not defined with something else. So if no one, for example, used a self or unchanged, we should automatically change it on eBGP. Because otherwise we have two ways to modify packets on the way out to the peer.", then "Unify into the export chain it is the way I want it."" | agrees | This spec does not touch BGP next-hop rewriting or export filter chains. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 on `docs/architecture/core-design.md` | "Egress export chains configure next-hop rewriting through one underlying `policychain.Sets` next-hop intent exposed in two authoring forms: `policy { modify <name> { set { next-hop self \| unchanged \| auto \| <ip>; } } }` for matched/conditional rules, and built-in chain references `next-hop:self`, `next-hop:unchanged`, and `next-hop:auto` for peer-wide policy without a wrapper `modify` block. Selected by Thomas on 2026-11-23 ("Option A: Extend policy modify (self\|unchanged\|auto\|<ip>) AND provide built-in next-hop:self / next-hop:unchanged / next-hop:auto chain refs")." | agrees | This spec does not touch BGP policy modify blocks or built-in next-hop chain references. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 on `docs/architecture/core-design.md` | "Built-in `next-hop:auto`, `next-hop:self`, and `next-hop:unchanged` are BGP engine steps compiled into `policychain.Chain` (parallel to `loop-detection`), never external plugin RPCs or a `pkg/plugin/rpc.FilterInsertion` tail extension. Config loading appends `next-hop:auto` at the tail of every export chain unless an unconditional `next-hop:self`, `next-hop:unchanged`, or `next-hop:auto` is already present, leaving `Policy.Configured()` false for unconfigured chains; operators override via `next-hop:unchanged`, and `inactive:next-hop:auto` is refused. Selected by Thomas on 2026-11-23 ("Option A: Built-in BGP engine step appended at config compile time (like loop-detection; zero RPC; override via next-hop:unchanged, no inactive:next-hop:auto)")." | agrees | This spec does not touch `policychain.Chain` compilation or BGP filter insertions. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 on `docs/architecture/core-design.md` | "Within an export chain, explicit next-hop sets (`self`, `unchanged`, `auto`, or an IP address) use standard last-write-wins folding while the auto-appended trailing `next-hop:auto` acts only when next-hop is still unset. For a whole-UPDATE raw override (`wireOverride`), Ze compares each emitted next-hop field against the received UPDATE (matching `forward_med.go` provenance): trailing `auto` is skipped only when the raw override changed the next-hop bytes or an earlier chain member already selected `unchanged`/`self`/explicit. Selected by Thomas on 2026-11-23 ("Option A: Provenance check on raw overrides (like MED): skip trailing auto only when the raw override changed the next-hop bytes or an earlier member set unchanged/self")." | agrees | This spec does not touch BGP UPDATE next-hop folding or `wireOverride`. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 on `docs/architecture/core-design.md` | "Explicit next-hop addresses in `policy { modify <name> { set { next-hop <ip>; } } }` and `policychain.Sets.NextHop` support both IPv4 (attr 3) and IPv6/MP_REACH (through `NextHopProfile` and `ValidatePeerGlobalNextHop`) on both export and import chains. Egress-only next-hop modes (`self`, `unchanged`, `auto`), whether written as built-in `next-hop:*` refs or inside `policy { modify }`, are refused on import chains at config load. Selected by Thomas on 2026-11-23 ("Option A: Support IPv4 + IPv6/MP_REACH for explicit next-hop on both export and import; refuse self/unchanged/auto on import at config load")." | agrees | This spec does not touch BGP next-hop validation on import or export chains. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 on `docs/architecture/core-design.md` | "Auto-appended trailing `next-hop:auto` applies only to forwarded routes and preserves the originator-chosen next hop on locally originated routes (static config routes, ExaBGP `announce`, API announcements). Explicit export-chain next-hop actions (`next-hop:self`, `modify { set { next-hop ... } }`) apply uniformly to both forwarded and locally originated routes. Selected by Thomas on 2026-11-23 ("Option A: Trailing next-hop:auto applies to forwarded routes only (preserves static/ExaBGP/API explicit next-hop); explicit next-hop:self / modify next-hop applies to both forwarded and originated routes")." | agrees | This spec does not touch BGP route origination or next-hop rewriting. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 on `docs/architecture/core-design.md` | "Every automatic filter-chain member Ze inserts — both built-in BGP engine steps (`next-hop:auto` at the export tail, `loop-detection` at the import head) and plugin-declared `FilterInsertions` — is published through one queryable implied-filter seam carrying direction, position, reference and `implied-by` reason so `\| implied` displays every extra chain member alongside the operator's written filter list. In Thomas's words (2026-11-23): "Can you make sure that \| implied show the extra chain members when it displays the filter ?"." | agrees | This spec does not touch BGP implied-filter reporting. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-26 on `docs/architecture/core-design.md` | "Ze always starts. The configuration of one service never makes `ze start` exit or refuse to start: a service whose configuration cannot be served is not started, ze runs with its other services, and the service is reported with its fix by an error line in the log under a doctor code and by `ze doctor` under the same code. Examples are a web certificate name that does not resolve, a looking-glass `server` entry with no certificate to present, and a looking-glass address list that does not parse. Where a service has several endpoints, only the endpoint that cannot be served stays down and the others serve. A reload whose config cannot be served stays refused as a whole, and the running services keep what they serve. In Thomas's words (voice-transcribed): "we should never not start ze as if we can not bring networking we can not fix things like web cert and the like so the only good answer is to disable the service badly configured and not start it"; asked about a reload, "reload stay refused"; asked "Should only that endpoint stay down while the others serve", "yes". The reason is that a daemon that does not start brings up no networking, so the operator cannot reach the box to correct it." | agrees | This spec adds no daemon-start refusal; `checkVPPVersion` reports a `ze doctor` warning and never blocks daemon startup. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-26 on `docs/architecture/core-design.md` | "The rule of 2026-11-23 above, that a configuration which holds a feature that programs only the Linux kernel while the interfaces run on VPP "is refused at daemon start", is superseded for the start: ze starts, that feature stays off, and `ze doctor` reports the mismatch with its fix. A commit, a reload and `ze config validate` still refuse such a configuration, by the same verdict as before. Thomas was asked whether the rule that ze always starts replaces the refusal at start, "(a) yes, ze starts, that feature stays off and ze doctor reports the mismatch, while commit and reload still refuse it; (b) no", and answered "2a" (voice-transcribed). The reason is the one of the row above: a daemon that does not start brings up no networking, so the operator cannot reach the box to correct the mismatch." | agrees | This spec does not alter daemon startup or kernel-vs-VPP mismatch handling. |
| `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` | Decisions | the `// Design:` page of `internal/component/ike/engine/rekey.go` | "One 1-second ticker drives dead peer detection and lifetimes, not a timer per SA." | silent | This page documents ESP Child SA and XFRM dataplane programming; D-8 changes only `CreatedAt` and `EstablishedAt` timestamps on the rekeyed IKE SA in `rekey.go`. |
| `docs/architecture/ike/ipsec-13-rekey-wire.md` | Decisions | anchored to `internal/component/ike/engine/rekey.go` | "One owner for post-establishment state." | agrees | D-8 modifies only `CreatedAt` and `EstablishedAt` inside the existing single-owner `applyIKERekeyResponse` and `respondIKERekey` functions without changing rekey message flow or ownership. |
| `docs/architecture/ike/ipsec-14-responder.md` | Direction is three concerns, not one | anchored to `internal/component/ike/engine/rekey.go` | "A partial generalization compiles and passes same-role tests while silently corrupting the other role." | agrees | D-8 removes the initiator/responder asymmetry on rekey timestamps so both roles carry `EstablishedAt` forward and advance `CreatedAt`. |
| `docs/architecture/ike/ipsec-7-ikev2-engine.md` | Decisions | anchored to `internal/component/ike/engine/rekey.go` | "An unnamed internal state is a programming error, not an unknown peer code." | silent | This page documents IKEv2 state transitions, multiple auth, and panic guards; D-8 alters only timestamp fields on rekeyed IKE SAs. |
| `docs/architecture/ike/rfcgate-1b-rfc7296-pilot.md` | Decisions | anchored to `internal/component/ike/engine/rekey.go` | "Narrowing is an intersection against operator policy, with a floor." | silent | This page documents traffic-selector narrowing and transport-mode negotiation; D-8 does not touch selectors or proposals. |
| `docs/architecture/testing/interop.md` | Prove a scenario discriminates | anchored to `internal/component/ike/engine/rekey.go` and modified by this spec (I-2) | "Six traps make a scenario pass whatever the code does." | agrees | Extends the Vacuity Traps table with the zero-contention concurrency test trap (I-2) and requires revert-proof validation for every new test in this spec. |
| `docs/architecture/testing/interop.md` | Design decisions | decision of 2026-11-23 on `docs/architecture/testing/interop.md` | "Each Docker interop suite and scenario checks only the kernel capabilities it actually requires (`Suite.KernelCapabilities` unioned with selected `ScenarioPlan.KernelCapabilities`); suites whose selected scenarios require none run without unrelated kernel features (such as SRv6 or IPsec)" | agrees | New interop scenarios in `test/interop-pppoe/`, `test/interop-l2tp/`, and `test/interop-ipsec/` declare only the kernel capabilities each scenario actually exercises. |
| `docs/architecture/flowexport/flow-export-0-umbrella.md` | In-process component with the SDK protocol | anchored to `internal/plugins/flowexport/register.go` | "`flowexport` runs inside the daemon process, which is what makes the iface callback and the EventBus access direct calls." | agrees | Keeping the boot `startTime` epoch inside the in-process `flowexport` plugin registration across reloads fits the existing lifecycle. |
| `docs/architecture/flowexport/flow-export-1-counter-export.md` | The polling interval is a maximum | the `// Design:` page of `internal/plugins/flowexport/exporter.go` and anchored to `register.go` | "Snapshots come from the iface rate tracker once a second, and never exactly on time." | silent | This page covers counter polling intervals and sequence ownership; D-9 only preserves the exporter's boot `startTime` across reloads so NetFlow v9 `sysUpTime` and flow timestamps stay on one epoch. |
| `docs/architecture/flowexport/flow-export-2-flow-records.md` | Flow encoders are interfaces in `flowexport`, registered by factory | anchored to `internal/plugins/flowexport/register.go` and `exporter.go` | "NetFlow v9 wants milliseconds relative to sysUpTime, IPFIX wants absolute milliseconds." | agrees | Preserving `exporter.startTime` across config reload keeps NetFlow v9 `FIRST_SWITCHED` and `LAST_SWITCHED` monotonic relative to boot `sysUpTime` (RFC 3954 Section 5.1). |
| `docs/architecture/observation-feed.md` | Why this shape | anchored to `internal/plugins/flowexport/exporter.go` | "Publish never blocks, by contract." | silent | This page documents the in-process observation feed; D-9 changes only `exporter.startTime` initialization and does not alter `Observation` publishing. |
| `docs/research/vpp-deployment-reference.md` | Generated grammar and upstream consumers | the `// Design:` page of `internal/component/vpp/config.go` | "Ze targets the [VPP v26.06 startup format](https://github.com/FDio/vpp/blob/v26.06/src/vpp/conf/startup.conf)." | agrees | Widening `poll-sleep` to accept microsecond strings (`us`) emits the same VPP v26.06 `unix { poll-sleep-usec <uint> }` directive with no change to whole-millisecond output. |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 on `docs/research/vpp-deployment-reference.md` | "Ze refuses to run VPP unmanaged. When VPP is external, the operator accepts that Ze may terminate and restart it" | agrees | This spec changes only `poll-sleep` unit parsing and `checkVPPVersion` range checking; managed/external VPP lifecycle policy is untouched. |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 on `docs/research/vpp-deployment-reference.md` | "Ze knows how to manage and restart an external VPP: through a systemd unit, or by starting the binary directly, as it does with the embedded runtime" | agrees | External VPP restart mechanisms are untouched by this spec. |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 on `docs/research/vpp-deployment-reference.md` | "After a failure, when the VPP process is alive, forwarding continued, and Ze reconnects and can read the state it configured, Ze continues from where it is" | agrees | VPP reconnect and continuation logic is untouched by this spec. |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 on `docs/research/vpp-deployment-reference.md` | "When VPP could have crashed or stopped forwarding, Ze reflects it in its own model and makes sure IGP and BGP peers are updated" | agrees | VPP crash detection and routing withdrawal logic is untouched by this spec. |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 on `docs/research/vpp-deployment-reference.md` | "One timer: the alive test and the loss declaration use the same time, and a probe missed inside it is not a loss. This narrows the row "After a failure, when the VPP process is alive": Ze continues from where it is only through form (1) below, inside the timer, and a declared loss is never adopted" | agrees | Superseded by the two-limit row below; VPP liveness timers are untouched by this spec. |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 on `docs/research/vpp-deployment-reference.md` | "As soon as a loss is declared, Ze makes sure the process is terminal (kill through the retained pidfd, then wait for the exit proof), then restarts and reconfigures VPP. Ze is the only restarter: a systemd unit it drives does not restart VPP itself (Restart=no)" | agrees | Narrowed/superseded by later rows below; VPP restart supervision is untouched by this spec. |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 on `docs/research/vpp-deployment-reference.md` | ""Recovered" has exactly two forms. (1) Inside the single timer Ze reconnects to the same pidfd-proven process and a full readback of what it installed (interfaces, LCP pairs, owned routes, PPPoE sessions) matches: Ze continues, with no kill and no withdrawal. Any mismatch means form (1) failed. (2) A forced restart (kill through the retained pidfd, exit proof) followed by a correct reconfiguration, exactly as at boot. Routes are withdrawn only when neither form succeeds within the recovery deadline and the crash-loop limit" | agrees | Narrowed by the Form (1) adoption row below; VPP recovery forms are untouched by this spec. |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 on `docs/research/vpp-deployment-reference.md` | "Blackholing during the restart is accepted. Routes are withdrawn (VPP-backed interfaces down in Ze's model, so IGP and BGP react) only when neither recovery form succeeds within the recovery deadline and the crash-loop limit. This narrows the row "When VPP could have crashed or stopped forwarding": Ze updates IGP and BGP peers only then" | agrees | VPP restart withdrawal behavior is untouched by this spec. |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 on `docs/research/vpp-deployment-reference.md` | "systemd configuration: Ze cannot be sure that an OS update will not change the unit's settings, so it detects the settings and adapts. No unit policy is required: Ze reads the unit's effective settings each time it acts on VPP, and adapts its restart to them. This supersedes the second sentence of the row "As soon as a loss is declared" (Ze the only restarter, Restart=no) and its forbids clause "or leaving the restart to systemd or another supervisor"; correcting that row needs Thomas's decision-change" | agrees | VPP systemd adaptation is untouched by this spec. |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 on `docs/research/vpp-deployment-reference.md` | "Two limits. The alive test and the loss declaration use the same time: a probe missed inside it is not a loss, and a failed alive test is the loss declaration. A second, separate limit then bounds the reconnect-and-verify attempt of form (1). When that limit expires without form (1), Ze makes the process terminal (kill through the retained pidfd, then wait for the exit proof), then restarts and reconfigures VPP. This supersedes the row "One timer" (its single timer, "a declared loss is never adopted" and its forbids clause "a second window after the declaration"), and narrows the row "As soon as a loss is declared": the kill follows the second limit, not the declaration" | agrees | VPP reconnect and kill timing is untouched by this spec. |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 on `docs/research/vpp-deployment-reference.md` | "Form (1) adopts on the presence of data ("We should take the presence of data as a sign that the software is still running and reconciliation may be then happening in the background."). Inside the reconnect-and-verify limit, the same pidfd-proven process answering over the API with the data Ze configured is the sign it is alive: Ze adopts it, with no kill and no withdrawal, and reconciles in the background, as the fib-vpp replay and sweep and the iface reconcile do. A mismatch the background reconciliation finds is repaired, not treated as a loss. Form (2) stays a forced restart followed by a correct reconfiguration, exactly as at boot. This narrows the row "Recovered has exactly two forms": a full readback match is no longer required, and a mismatch no longer fails form (1)" | agrees | VPP background reconciliation and adoption are untouched by this spec. |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 on `docs/research/vpp-deployment-reference.md` | "A recovery deadline and a crash-loop limit (N restarts inside a window) bound recovery. Past either, Ze withdraws: VPP-backed interfaces go down in Ze's model, so IGP and BGP react, and ze doctor reports it. Blackholing while Ze restarts inside both limits is accepted. The default values are proposed at design review for Thomas's approval. This adopts the recovery deadline and the crash-loop limit that the rows "Recovered has exactly two forms" and "Blackholing during the restart is accepted" carried as proposals, and narrows the row "When VPP could have crashed or stopped forwarding": Ze updates IGP and BGP peers only past a limit" | agrees | VPP recovery deadline and crash-loop limits are untouched by this spec. |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 on `docs/research/vpp-deployment-reference.md` | "After a withdrawal Ze keeps retrying recovery with a growing backoff. When VPP recovers, the VPP-backed interfaces come up in Ze's model and the routes return by themselves, and ze doctor reports the state throughout. This narrows the crash-loop limit: past it Ze withdraws and keeps restarting" | agrees | VPP post-withdrawal retry backoff is untouched by this spec. |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 on `docs/research/vpp-deployment-reference.md` | "The VPP loss itself triggers no withdrawal: no VPP-backed interface goes down in Ze's model while Ze recovers. Ze's own protocol timers (BFD detection, OSPF and IS-IS dead intervals, BGP hold) keep running during recovery, as their RFCs say" | agrees | VPP recovery timer interaction is untouched by this spec. |
| `docs/research/vpp-deployment-reference.md` | Design decisions | decision of 2026-11-23 on `docs/research/vpp-deployment-reference.md` | "A process on VPP's API socket whose pidfd does not resolve to the configured systemd unit is reported, never refused or killed ("We should report but not refuse if the customers have changed the software, for example. It may mean that we have an out of date configuration. We should make sure the customer updates it but not cause a routing outage."). ze doctor names the process, the configured unit, that the configuration may be out of date, and the exact fix, and Ze keeps working with that process" | agrees | `checkVPPVersion` in `doctor_api.go` follows the same non-refusing diagnostic principle: an out-of-range VPP version raises a `ze doctor` warning naming the found version and supported range rather than refusing startup. |
| `ai/rules/repo-maintenance.md` | Canonical Sources and Sync Direction | the `// Design:` page of `internal/plugins/iface/vpp/doctor_api.go` | "A generated file MUST NOT be edited. Edit its canonical source, then run its sync command." | agrees | I-2 edits the canonical directive file under `ai/rules/points/principles/directives/` and runs `./le ai rules update render` + `./le ai rules update condensed` rather than hand-editing `ai/rules/*.md`; `checkVPPVersion` stays owned in `internal/plugins/iface/vpp/doctor_api.go`. |
| `docs/architecture/module-tiers.md` | What the gate enforces | anchored to `internal/le/doc/wiring/wiring.go` | "`./le arch tier check` enforces engine placement, the non-engine manifest, core import direction, disable-ability, build-tag drift, plugin import ownership, test-only helper imports, and the docker route." | silent | This page documents module tier placement rules; I-1 extends the separate `doc/wiring` symbol/topic checker without changing package tier placement. |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
Three independent entry points converge on the same downstream consumers.
1. A peer PPP Configure-Request arriving on an established session, read by `handleFrame` in `ppp/session_run.go`. This is the renegotiation trigger.
2. A PPPoE session reaching Opened, surfaced by the PPP driver as `ppp.EventSessionUp` and consumed by `Subsystem.handlePPPEvent` in `pppoe/subsystem.go`.
3. An L2TP session reaching Opened, surfaced to `L2TPReactor.handleSessionUp` in `reactor_kernel.go`.

### Transformation Path
1. On initial establishment, `afterLCPOpen` drives auth (`runAuthPhase`) and NCP (`runNCPPhase`), marks the session established, and emits `EventSessionUp` once. On a later LCP renegotiation of an already-established session, `LCPActTLD` keeps the logical session alive (without tearing down accounting, metadata, or timeouts) and `afterLCPOpen` skips re-authentication, updates LCP/NCP dataplane parameters (`SetMTU`, `AddAddressP2P`), and emits `EventSessionParamsChanged`.
2. The transport subsystem consumes the driver event: PPPoE updates `subscriber.DefaultRegistry` (preserving `ActivatedAt`), starts RADIUS session/idle timers on first activation, and emits `subevents`; L2TP emits `l2tpevents`, which `subscriberBridge` re-emits onto `subevents` while preserving `ActivatedAt`.
3. Lifecycle consumers (`pool`, `route_observer`, `authradius/acct`, `shaper`, `cos`) all subscribe to `subevents` alone and key per-session state on canonical `SubscriberID`.
4. Consumers needing RADIUS profile data call `LoadSessionMetadata(subscriberID)` on the canonical keyspace.
5. CoA requests run through `confirmCoA` (`authradius/coa_transaction.go`), which sends canonical `SubscriberID` to `request l2tp shaper rate` and `request class-of-service session`, waits for confirmed apply/commit (or rollback on failure), emits `subevents.SessionRateChange` / `subevents.SessionCoSChange`, and returns CoA-ACK only when applied (CoA-NAK otherwise). Disconnect-Requests for PPPoE invoke `subscriber.GetDisconnectHandler()`.
6. Standalone invariants: IKE rekey (`ike/engine/rekey.go`) preserves `EstablishedAt` and advances `CreatedAt` in both directions; `flowexport` (`register.go`, `exporter.go`) preserves its boot `startTime` across reloads; `vpp/config.go` parses `us`/`ms` into `poll-sleep-usec`; `iface/vpp/doctor_api.go` validates the parsed VPP version range.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| PPP FSM ↔ session side effects | first-activation latch in `ppp/session_run.go` (`LCPActTLD`, `afterLCPOpen`) and `ppp/ncp.go` | No |
| Transport subsystem ↔ lifecycle consumers | typed `subevents` EventBus topics (`SessionUp`, `SessionParamsChanged`, `SessionIPAssigned`, `SessionIPReleased`, `SessionDown`, `SessionRateChange`, `SessionCoSChange`) | No |
| CoA listener ↔ shaper / CoS / PPPoE teardown | `confirmCoA` hidden command JSON transactions carrying canonical `subscriber-id`, plus `subscriber.DisconnectHandler` | No |
| Consumer ↔ RADIUS profile data | `LoadSessionMetadata(subscriberID)` on canonical keyspace | No |
| Ze ↔ RADIUS server | RFC 2865 Access-Request, RFC 2866 Accounting-Request, RFC 5176 CoA/Disconnect ACK & NAK on UDP | No |
| Ze ↔ NetFlow collector | RFC 3954 `sysUpTime` header and relative `FIRST_SWITCHED`/`LAST_SWITCHED` milliseconds | No |
| Ze ↔ IKE peer / CLI | replacement IKE SA `CreatedAt` and `EstablishedAt` surfaced in `show ipsec` | No |
| Ze ↔ VPP daemon | `startup.conf` `unix { poll-sleep-usec <N> }` and `vppctl show version` parsed by `checkVPPVersion` | No |

### Changed-Wire Census

**Search evidence:** Searched `internal/component/l2tp/`, `internal/plugins/cos/`, `internal/plugins/flowexport/`, `internal/component/ike/engine/`, `internal/component/vpp/`, and `internal/plugins/iface/vpp/` for `subevents.`, `l2tpevents.`, `request l2tp shaper rate`, `request class-of-service session`, `buildAcctPacket`, `handleCoA`, `handleDisconnect`, `startTime`, `poll-sleep-usec`, and `checkVPPVersion`. Positive controls verified at HEAD: `internal/component/l2tp/plugins/pool/register.go` (`subevents.SessionDown` reader), `internal/component/l2tp/route_observer.go` (`subevents.SessionIPAssigned` / `SessionDown` reader), `internal/component/l2tp/plugins/authradius/coa_transaction.go` (`confirmCoA` writer), and `internal/component/vpp/startupconf.go` (`poll-sleep-usec` writer). External protocol wire formats (RFC 2865/2866/5176/3954/7296) keep identical packet encodings; only the internal EventBus topic set and the `request l2tp shaper rate` JSON request gain fields (`SessionParamsChanged`, `SessionCoSChange`, and `subscriber-id`), and `poll-sleep` YANG input widens to microseconds.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| Internal `subevents` EventBus JSON | writer | `internal/component/l2tp/subscriber_bridge.go` (`onSessionUp`, `onSessionDown`, `onSessionIPAssigned`, `onSessionIPReleased`), `internal/component/l2tp/pppoe/subsystem.go` (`handlePPPEvent`), `internal/component/l2tp/plugins/authradius/coa.go` (`applySubscriberCoA`) | Verified at HEAD: `subscriber/events/events.go` defines 6 topics; `coa.go:578` still writes `l2tpevents.SessionCoSChange` | Change: register `SessionParamsChanged` and `SessionCoSChange`; emit both from transport/CoA producers without changing existing payload JSON field names |
| Internal `subevents` EventBus JSON | reader | `internal/component/l2tp/plugins/pool/register.go` (`onSessionDown`), `internal/component/l2tp/route_observer.go` (`StartSubscriberRouteObserver`), `internal/component/l2tp/plugins/authradius/acct.go`, `internal/component/l2tp/plugins/shaper/shaper.go`, `internal/plugins/cos/handler.go` | Verified at HEAD: `pool` and `route_observer` already read `subevents`; `acct`, `shaper` rate/up, and `cos` still read `l2tpevents` | Change: migrate `acct`, `shaper`, and `cos` to read `subevents` alone; preserve `pool` and `route_observer` readers unchanged |
| Hidden CoA command JSON (`request l2tp shaper rate`, `request class-of-service session`) | writer | `internal/component/l2tp/plugins/authradius/coa_transaction.go` (`confirmCoA`) | Verified at HEAD: `coa_transaction.go:87` already writes `subscriber-id` for CoS; line 78 omits `subscriber-id` for shaper | Change: populate `SubscriberID: sub.ID` on `request l2tp shaper rate` while retaining diagnostic `TunnelID`/`SessionID` |
| Hidden CoA command JSON (`request l2tp shaper rate`, `request class-of-service session`) | reader | `internal/component/l2tp/plugins/shaper/rate_transaction.go` (`executeRateTransaction`), `internal/plugins/cos/session_transaction.go` (`executeSessionTransaction`) | Verified at HEAD: `rate_transaction.go` ignores `subscriber-id`; `session_transaction.go:211` calls `ParseL2TPBirth` and rejects PPPoE | Change: decode and validate canonical `subscriber-id` via `subscriber.RequireID` in both readers so L2TP and PPPoE share one transaction contract |
| RADIUS Access-Request (RFC 2865) | writer | `internal/component/l2tp/plugins/authradius/handler.go` (`handle`), driven by `internal/component/l2tp/ppp/session_run.go` (`runAuthPhase`) | Verified at HEAD: encodes standard RFC 2865 Access-Request on UDP 1812 | Preserve encoding; mid-session LCP renegotiation skips `runAuthPhase()` so established sessions send no duplicate Access-Request (AC-17) |
| RADIUS Access-Request (RFC 2865) | reader | External RADIUS authentication server (FreeRADIUS, `internal/test/fixture/tunnel_fixture_l2tp_ppp.go`, `test/l2tp/renegotiation-preserves-session.ci`) | Verified at HEAD: standard RFC 2865 auth server decoder | Preserve: wire format unchanged; test asserts total Access-Request count stays 1 across renegotiation |
| RADIUS Accounting-Request (RFC 2866) | writer | `internal/component/l2tp/plugins/authradius/acct.go` (`buildAcctPacket`, `runAccounting`) | Verified at HEAD: encodes standard RFC 2866 attributes on UDP 1813 | Preserve encoding; extend trigger to PPPoE sessions via `subevents` and preserve `Acct-Session-Id`/`Acct-Session-Time` across LCP renegotiation |
| RADIUS Accounting-Request (RFC 2866) | reader | External RADIUS accounting server (FreeRADIUS, `internal/component/l2tp/plugins/authradius/acct_test.go` `startAcctServer`, `internal/test/fixture/tunnel_fixture_l2tp_ppp.go`) | Verified at HEAD: standard RFC 2866 decoder | Preserve: wire format unchanged; proven by `accounting-lifecycle.ci`, `renegotiation-preserves-session.ci`, and `pppoe-accounting-freeradius` |
| RADIUS CoA / Disconnect (RFC 5176) | writer | `internal/component/l2tp/plugins/authradius/coa.go` (`handleCoA`, `handleDisconnect`, `answerRequest`) | Verified at HEAD: encodes RFC 5176 ACK/NAK on UDP 3799 | Preserve encoding; answer Disconnect-ACK after PPPoE teardown and CoA-ACK only after confirmed application (NAK otherwise) |
| RADIUS CoA / Disconnect (RFC 5176) | reader | External RADIUS CoA/DM originator (`radclient`, `authradius/coa_test.go`, `test/pppoe/coa-*.ci`) | Verified at HEAD: standard RFC 5176 client | Preserve: wire format unchanged; proven by `coa-rate-change.ci` and `coa-disconnect.ci` |
| NetFlow v9 datagram (RFC 3954) | writer | `internal/plugins/flowexport/exporter.go` & `internal/plugins/flowexport/netflow9/` (`flow_data.go`) | Verified at HEAD: computes header `sysUpTime` and record `FIRST_SWITCHED`/`LAST_SWITCHED` from `exporter.startTime` | Preserve encoding; keep `startTime` fixed at boot epoch across config reloads |
| NetFlow v9 datagram (RFC 3954) | reader | External NetFlow v9 collector (`test/plugin/flowexport-reload-epoch.ci`, `exporter_test.go`) | Verified at HEAD: standard RFC 3954 collector | Preserve: wire format unchanged; collector sees monotonic `sysUpTime` across reload |
| VPP `startup.conf` `unix.poll-sleep-usec` | writer | `internal/component/vpp/config.go` (`parsePollSleep`) -> `internal/component/vpp/startupconf.go` (`GenerateStartupConf`) | Verified at HEAD: emits `poll-sleep-usec <uint>` in `unix` block | Preserve output directive; widen input YANG/parser to accept `us` as well as `ms` (existing `ms` configs emit byte-identical output) |
| VPP `startup.conf` `unix.poll-sleep-usec` | reader | External VPP daemon (`src/vlib/unix/main.c` `unix_config` in VPP v26.06) | Verified against `docs/research/vpp-deployment-reference.md` v26.06 consumer table | Preserve: VPP already expects microseconds in `poll-sleep-usec` |

### Integration Points
- `subscriber.DefaultRegistry` - single registry both transports write; `Add` preserves an existing non-zero `ActivatedAt` while keeping its public API unchanged.
- `subscriber/handler_registry.go` - adds `DisconnectHandler` alongside `AuthHandler`, `PoolHandler`, and `ShaperHandler` for I-4.
- `internal/le/doc/wiring/wiring.go` and `allowlist.go` - Go wiring checker reached via `./le doc wiring`, extended with the emit-without-subscribe predicate for I-1.
- `internal/core/events` - typed EventBus topic declarations inspected by the I-1 checker.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The bridge's one-directional re-emit lets consumers migrate one at a time without an L2TP regression | `subscriber_bridge.go` subscribes to `l2tpevents` and emits `subevents`; already proven by D-1 (`pool`) and D-14 (`route_observer`) | a flag-day migration is required | migrate `authradius/acct`, `shaper`, and `cos` one consumer at a time and run the L2TP package tests after each | confirmed (D-1 & D-14) |
| A-2 | `subscriber.Session.ID` is unique across transports and stable for a session's life | L2TP uses `l2tp-<32hex>` (`L2TPBirth`); PPPoE uses `pppoe-<ifIndex>-<sid>` (`pppoeSessionID`) | a canonical key reintroduces a collision | `TestSubscriberIDBuildersNeverCollide` and `TestSessionMetadataNoCrossTransportCollision` | unvalidated |
| A-3 | Emitting `SessionUp` once and `SessionParamsChanged` on renegotiation leaves no consumer needing the old re-fire | Every migrated consumer (`pool`, `route_observer`, `acct`, `shaper`, `cos`, timers) acts once per session activation | a consumer silently misses a legitimate parameter change | per-consumer review during migration and renegotiation tests verifying MRU/IPCP address updates still apply | unvalidated |
| A-4 | RADIUS metadata is stored before the first consumer reads it, on both transports | `authradius/handler.go` stores under canonical `SubscriberID` during auth before NCP and `SessionUp` | a migrated consumer reads nil metadata and silently applies defaults | assert non-nil RADIUS profile attributes (`Filter-Id` rate, `Session-Timeout`, `Acct-Interim-Interval`) in PPPoE functional tests | unvalidated |
| A-5 | No operator config expresses `poll-sleep` in a unit other than whole milliseconds (`ms`) today | `ze-vpp-conf.yang:129` admits digits followed by `ms` only | widening the pattern breaks an existing config | parse every `poll-sleep` value in `test/` and appliance seeds before and after, asserting identical `poll-sleep-usec` | unvalidated |
| A-6 | A rekeyed IKE SA is a new SA carrying an unchanged authentication, so `CreatedAt` advances and `EstablishedAt` carries forward | RFC 7296 Section 2.18 ("the new IKE SA", new SPIs, new keys) and Section 2.8.3 ("does not authenticate the parties again"; reauthentication is a from-scratch IKE_SA_INIT/IKE_AUTH) | `show ipsec` uptime misreports tunnel stability | RFC text read 2026-08-02; AC-9 asserts both fields in both rekey directions | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Migrating `shaper` and `cos` transactions to canonical `SubscriberID` could break existing L2TP CoA discrimination records (`coa_transaction.go:69`) | `./le rfc check` or CoA tests report stale producer SHA or rejected L2TP CoA | keep `confirmCoA` loop intact, pass `SubscriberID` alongside existing diagnostic `TunnelID`/`SessionID`, and re-run `authradius`, `shaper`, and `cos` tests |
| R-2 | Keeping the network phase alive across LCP renegotiation (`LCPActTLD`) must not leave stale dataplane state if renegotiation fails or terminates before re-opening | renegotiation-abort test leaks kernel interface or pool allocation | only suppress teardown while transitioning through renegotiation; if LCP reaches Closed/Stopped or fails timeout, emit full `EventSessionDown` as before |
| R-3 | The I-1 guard fires on legitimately unsubscribed topics (topics emitted for external plugin processes only) | `./le doc wiring` goes red on unrelated topics | reuse the existing `wiringAllowlist` in `internal/le/doc/wiring/allowlist.go` with per-entry rationale |
| R-4 | Adding RFC checklist rows to four enrolled summaries makes `./le rfc check` red until every row has positive and negative tagged tests | `./le rfc check` reports uncovered requirements mid-implementation | add each row in the same phase as its positive and negative tagged proofs, never ahead of them |
| R-5 | Extending `internal/test/fixture/tunnel_fixture_l2tp_ppp.go` to send a mid-session Configure-Request must reproduce D-6 before any fix lands | harness cannot trigger renegotiation against unfixed code | build and verify the harness in Phase 2 against unfixed code first; if blocked, stop and report rather than dropping to unit-only proof |
| R-6 | Renegotiation currently re-runs `runAuthPhase()`, so fixing accounting alone would leave a duplicate RADIUS Access-Request | fake RADIUS server counts 2 Access-Requests for 1 session | suppress re-auth in `ppp/session_run.go` and assert Access-Request count == 1 alongside Accounting-Start count == 1 (AC-17) |
| R-7 | Changing the RFC 1661 `{gap}` count desynchronises the spelled number in `docs/features/rfc-status.md` | `check_gap_count_agreement` fails at commit | update `docs/features/rfc-status.md` in the same commit as any `rfc/short/` change |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Subscriber sessions on both L2TP and PPPoE: accounting records double or vanish, shaping and CoS stop applying or fail to restore, CoA NAKs valid L2TP sessions, or RADIUS-imposed session limits stop being enforced. IKE and NetFlow changes are observability-only. |
| How is it reverted? | Phase by phase. Consumer migrations (`acct`, `shaper`, `cos`) are independently revertible because `subscriberBridge` continues feeding both namespaces. The renegotiation latch and standalone fixes (`rekey.go`, `flowexport`, `vpp`) are each isolated commits. |
| Who else touches this path? | `spec-finish-l2tp` (closed 2026-11-23; its L42 renegotiation-test gap is owned here), `plan/release/fixes/spec-rfc5176-remaining-defect-classes.md` (shares CoA boundary; this spec owns PPPoE delivery/applied-result AC-3/AC-4), `plan/post-release/engineering/spec-radius-acct-timewheel.md` (shares accounting loop; land this first), `docs/architecture/traffic/cos-dynamic.md`, `docs/architecture/l2tp/bng-1-radius-attributes.md`. |

## Wiring Test (MANDATORY -- NOT deferrable)

Each user-facing row names the `.ci` or interop test that drives the real entry point. The two internal tooling rows name their Go unit tests. Already-verified D-14 (`test/pppoe/redistribute-subscriber-route.ci`) is included as a regression anchor.

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| PPPoE subscriber disconnects repeatedly | → | `poolPlugin.onSessionDown` via `subevents.SessionDown` | `test/pppoe/pool-release-cycle.ci` |
| PPPoE subscriber authenticates and comes up | → | `radiusAcct.onSessionIPAssigned` / `onSessionDown` via `subevents` | `test/pppoe/accounting-lifecycle.ci` |
| RADIUS sends a CoA rate-change for a PPPoE session | → | `confirmCoA` -> `shaper` rate transaction + `subevents.SessionRateChange` | `test/pppoe/coa-rate-change.ci` |
| RADIUS sends a Disconnect-Request for a PPPoE session | → | `coaListener.handleDisconnect` -> `subscriber.GetDisconnectHandler()` -> `pppoe.Subsystem` PADT teardown | `test/pppoe/coa-disconnect.ci` |
| PPPoE subscriber gets an address and Framed-Route under `redistribute pppoe` | → | `StartSubscriberRouteObserver` + `pppoe/redistribute.go` (already landed and verified) | `test/pppoe/redistribute-subscriber-route.ci` |
| Peer sends a Configure-Request on an established session | → | `ppp/session_run.go` renegotiation latch -> `subevents.SessionParamsChanged` | `test/l2tp/renegotiation-preserves-session.ci` |
| Operator reloads config with flow export enabled | → | `flowexport/register.go` `configure` preserving boot `startTime` | `test/plugin/flowexport-reload-epoch.ci` |
| Locally-initiated IKE SA rekey | → | `applyIKERekeyResponse` preserving `EstablishedAt`, advancing `CreatedAt` | `test/interop-ipsec/` `ipsec-rekey-uptime-strongswan` |
| `ze doctor` on a VPP host | → | `ifacevpp.checkVPPVersion` range check (`internal/plugins/iface/vpp/doctor_api.go`) | `TestCheckVPPVersionRejectsUnsupportedRange` (tooling/doctor check) |
| An event topic emitted and never subscribed | → | `docwiring` emit-without-subscribe predicate (`internal/le/doc/wiring/wiring.go`) | `TestEmitWithoutSubscribeIsReported` (tooling, no user entry point) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | A PPPoE session that allocated a pool address tears down | The address returns to the pool and is allocated to a subsequent PPPoE session across more connect/disconnect cycles than the pool holds addresses (`test/pppoe/pool-release-cycle.ci`). | None: every released PPPoE allocation must return to the pool immediately upon confirmed teardown. |
| AC-2 | A PPPoE session authenticates and reaches Opened | Exactly one RFC 2866 Accounting-Start reaches the RADIUS server, followed by scheduled Interim-Updates and one Accounting-Stop carrying the session's terminate cause on teardown. | Sessions terminated before IP assignment emit no Accounting-Start/Stop (RFC 2866 starts accounting once service begins). |
| AC-3 | A RADIUS CoA rate-change names a live PPPoE session | The shaper applies the new download/upload rates to the PPPoE dataplane interface, updates `s.sessions`, and `coaListener` returns CoA-ACK. | CoA requests with unsupported attributes or duplicate rate/CoS values remain refused with RFC 5176 NAK 401/407. |
| AC-4 | A RADIUS CoA names a session whose rate or CoS change cannot be applied by the owner | The owner rolls back any partial change, leaves prior dataplane policy intact, and `coaListener` returns CoA-NAK (error cause 503/506). | None: an unapplied CoA must never return CoA-ACK (RFC 5176 Section 3.1). |
| AC-5 | A PPPoE session is established with a RADIUS `Filter-Id` rate | The session appears in `show l2tp shaper` with its RADIUS `Filter-Id` rate applied to the dataplane, and restores the original qdisc upon session teardown. | Sessions with no `Filter-Id` rate still receive the configured default shaper policy when shaping is enabled. |
| AC-6 | A peer sends an LCP Configure-Request on an established session | The subscriber registry entry retains its original `ActivatedAt` timestamp and `Username` while applying the newly negotiated LCP parameters. | Changed dataplane parameters (such as a newly negotiated MRU or IPCP address) are applied on purpose. |
| AC-7 | A peer renegotiates LCP after RADIUS accounting has started | The active accounting session retains its original `Acct-Session-Id` and monotonic `Acct-Session-Time` measured from first activation, emitting only scheduled Interim-Updates and one final Accounting-Stop (total Accounting-Start count remains 1). | Normal periodic Interim-Updates continue on schedule while the session remains up. |
| AC-8 | A peer renegotiates LCP on a session carrying a RADIUS `Session-Timeout` | The session terminates at the original `Session-Timeout` deadline computed from first activation, and the session holds at most one active timeout goroutine pair. | Teardown before the deadline cancels the timer cleanly as before. |
| AC-9 | An IKE SA is rekeyed, either locally initiated (`applyIKERekeyResponse`) or peer initiated (`respondIKERekey`) | `show ipsec` reports `uptime-seconds` continuous across the rekey (`EstablishedAt` preserved from the old SA) and `created-at` advanced to the rekey time (`CreatedAt` stamped at rekey) in both directions. | Full IKE reauthentication (from-scratch `IKE_SA_INIT`/`IKE_AUTH`, RFC 7296 Section 2.8.3) still stamps a fresh `EstablishedAt`. |
| AC-10 | The daemon reloads its configuration with NetFlow v9 flow export enabled | Exported NetFlow v9 headers report monotonically increasing `sysUpTime` on the boot epoch, and `FIRST_SWITCHED`/`LAST_SWITCHED` stay aligned to that same epoch across the reload. | A full daemon process restart starts a new boot epoch as defined by RFC 3954 Section 5.1. |
| AC-11 | An L2TP session and a PPPoE session hold numerically equal identifier pairs | Each session reads back its own RADIUS profile metadata (`Framed-Pool`, `Filter-Id`, `Session-Timeout`, `Idle-Timeout`, `CoSProfile`) under its canonical `SubscriberID`. | Numeric `TunnelID`/`SessionID` remain permitted solely as display/diagnostic labels, never as lookup or ownership keys. |
| AC-12 | A codebase declares an EventBus topic that is emitted in production code and subscribed nowhere | `./le doc wiring` reports the unsubscribed topic with file and symbol and exits non-zero, unless explicitly exempted in `wiringAllowlist`. | Topics intentionally emitted for external plugin processes only may be exempted in `wiringAllowlist` with a documented rationale. |
| AC-13 | An operator configures a sub-millisecond `vpp { poll-sleep <N>us }` value (e.g. `500us`) | Config validation accepts `0us..100000us` (alongside existing `ms` values) and `GenerateStartupConf` emits `poll-sleep-usec <N>` in microseconds, while existing whole-millisecond configs emit byte-identical output. | Values outside `0..100000` microseconds remain rejected by YANG and parser validation. |
| AC-14 | `ze doctor` runs against a VPP daemon whose reported version is outside the supported range | `checkVPPVersion` (`internal/plugins/iface/vpp/doctor_api.go`) parses the major.minor version and emits a `doctor-vpp-version` warning naming the found version, the supported range, and the remediation. | Supported VPP versions pass cleanly; an unreachable `vppctl` retains its existing non-fatal warning rather than blocking daemon startup. |
| AC-15 | A RADIUS Disconnect-Request names a live PPPoE session | `coaListener.handleDisconnect` invokes the registered `DisconnectHandler`, `pppoe.Subsystem` sends PADT and completes session teardown with cause Admin-Reset, and Ze returns Disconnect-ACK. | Ambiguous matches (508), unknown sessions (503), and malformed requests (401/405) retain their RFC 5176 Disconnect-NAK answers. |
| AC-16 | RADIUS supplies a `Framed-Route` for a PPPoE subscriber | **Already verified at HEAD (D-14):** the framed route is bound to the exact auth round, installed on IPCP up, announced via `redistribute pppoe`, and withdrawn on session teardown (`test/pppoe/redistribute-subscriber-route.ci`). | Invalid or unparseable `Framed-Route` strings remain ignored with a warning as on L2TP. |
| AC-17 | A peer renegotiates LCP on an already-authenticated PPP session | The PPP session completes LCP renegotiation and enters the network phase under its existing authenticated identity, keeping total RADIUS `Access-Request` count at 1. | A brand-new session (new call/PADR) always runs full authentication. |
| AC-18 | A PPPoE session carrying a dynamic RADIUS CoS profile tears down | `cos` removes the session from its shared access owner and restores the previously captured baseline VLAN QoS maps on final departure (`cos/shared_access.go`). | If another compatible live subscriber still shares the same access interface, its active CoS maps remain installed until the final subscriber departs. |
| AC-19 | The Go scripted L2TP/PPP peer (`tunnel_fixture_l2tp_ppp.go`) is asked to renegotiate an established session | It transmits a valid LCP Configure-Request over the established PPPoL2TP data channel and completes Configure-Ack/IPCP convergence, enabling functional proof of AC-6, AC-7, AC-8, and AC-17. | Existing L2TP fixture modes (initial setup, echo keepalive, RADIUS accounting) remain unchanged. |
| AC-20 | A PPPoE subscriber's Access-Accept carries `Session-Timeout`, `Idle-Timeout`, or both (D-11) | `session_timeout.go` enforces each timer keyed by canonical `SubscriberID`: `Session-Timeout` ends the PPPoE session at its deadline with cause Session-Timeout, and `Idle-Timeout` ends it after the quiet interval with cause Idle-Timeout, with no collision against L2TP and no reset on renegotiation. | Sessions whose Access-Accept omits both attributes start no timer goroutines. |
| AC-21 | The idle check cannot read the session interface's RX byte counter (D-12) | `readIfaceRXBytes` returns an error; `runIdleTimeout` logs a warning naming the session and interface, keeps the session running, and retries on the next tick. | Genuine interface removal ends the session through the transport teardown path, which cancels the idle timer cleanly. |
| AC-22 | Two PPPoE access interfaces whose ifindex differ by 65536 each bring up a session with the same PPPoE session ID (D-13) | Both sessions establish and remain distinct across the PPP driver, pool, and metadata store; tearing down one session releases only its own resources and leaves the second session running (`TestPPPoEIfIndexAbove16BitsKeepsDistinctSessions`). | `StartSession.TunnelID` may carry a diagnostic 16-bit display label so long as zero maps or lookups key on it. |
| AC-23 | A PPPoE subscriber comes up with an assigned address under `redistribute pppoe` (D-14) | **Already verified at HEAD (2026-11-19):** both the assigned `/32` host route and any RADIUS `Framed-Route` are announced to routing peers while active and withdrawn on teardown (`test/pppoe/redistribute-subscriber-route.ci`, 80/80 guest iterations). | Unrelated surviving PPPoE and L2TP subscribers keep their routes announced when one session disconnects. |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Connects a PPPoE subscriber, then disconnects it, repeatedly | PADI/PADR → PPP auth → pool allocate → session up → teardown → `subevents.SessionDown` → pool release | `test/pppoe/pool-release-cycle.ci` |
| 2 | Bills a PPPoE subscriber for a session | session up → `subevents.SessionIPAssigned` → Accounting-Start → interim → teardown → Accounting-Stop | `test/pppoe/accounting-lifecycle.ci` |
| 3 | Changes a live PPPoE subscriber's rate from the RADIUS server | CoA-Request → `confirmCoA` → `request l2tp shaper rate` (canonical `subscriber-id`) → dataplane apply → CoA-ACK | `test/pppoe/coa-rate-change.ci` |
| 4 | Disconnects a live PPPoE subscriber from the RADIUS server | Disconnect-Request → `subscriber.GetDisconnectHandler()` → PADT → session teardown → Disconnect-ACK | `test/pppoe/coa-disconnect.ci` |
| 5 | Runs a subscriber whose CPE renegotiates LCP mid-session | Configure-Request → PPP FSM → renegotiation latch → `SessionParamsChanged` → accounting, auth, and timeouts preserved | `test/l2tp/renegotiation-preserves-session.ci` |
| 6 | Reloads configuration while a NetFlow v9 collector is attached | reload → `flowexport` `configure` → exporter rebuilt with preserved boot `startTime` → monotonic `sysUpTime` | `test/plugin/flowexport-reload-epoch.ci` |
| 7 | Redistributes PPPoE subscriber routes into BGP (already verified) | PPPoE auth + IPCP → `subevents.SessionIPAssigned` → `StartSubscriberRouteObserver` → `redistribute pppoe` → BGP peer | `test/pppoe/redistribute-subscriber-route.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRenegotiationEmitsParamsChangedNotSessionUp` | `internal/component/l2tp/ppp/renegotiate_test.go` | AC-6: a second entry into Opened emits `EventSessionParamsChanged`, not a second `EventSessionUp` | |
| `TestRenegotiationDoesNotRerunAuth` | `internal/component/l2tp/ppp/renegotiate_test.go` | AC-17: `runAuthPhase` runs once per logical session across LCP renegotiation | |
| `TestRenegotiationStillProgramsDataplane` | `internal/component/l2tp/ppp/renegotiate_test.go` | AC-6: `SetMTU` and `AddAddressP2P` run on renegotiation so changed MRU/IPCP address lands | |
| `TestAcctNoDuplicateStartOnRepeatedIPAssigned` | `internal/component/l2tp/plugins/authradius/acct_test.go` | AC-7 using the fake accounting server | |
| `TestAcctSessionTimeSurvivesRenegotiation` | `internal/component/l2tp/plugins/authradius/acct_test.go` | AC-7: `Acct-Session-Id` and `Acct-Session-Time` survive renegotiation | |
| `TestAcctPPPoELifecycleViaSubEvents` | `internal/component/l2tp/plugins/authradius/acct_test.go` | AC-2: PPPoE `subevents` drives Start, Interim, and Stop under canonical `SubscriberID` | |
| `TestStartSessionTimeoutsPreservesDeadline` | `internal/component/l2tp/session_timeout_test.go` | AC-8: repeated start keeps original `Session-Timeout` deadline and single goroutine pair | |
| `TestSessionMetadataNoCrossTransportCollision` | `internal/component/l2tp/session_metadata_test.go` | AC-11 and A-2: equal numeric pairs on L2TP and PPPoE stay isolated | |
| `TestSubscriberIDBuildersNeverCollide` | `internal/component/l2tp/subscriber/registry_test.go` | A-2: L2TP and PPPoE canonical ID builders cannot produce equal strings | |
| `TestRegistryAddPreservesActivatedAt` | `internal/component/l2tp/subscriber/registry_test.go` | AC-6: `Registry.Add` preserves non-zero `ActivatedAt` on repeat updates | |
| `TestPoolReleasesPPPoEAddressOnSessionDown`, `TestPoolReleasesL2TPAddressOnSessionDown`, `TestPoolSessionDownReleasesOnce` | `internal/component/l2tp/plugins/pool/register_test.go` | AC-1 at unit level, both transports, release-once | done |
| `TestShaperRecordsPPPoESession` | `internal/component/l2tp/plugins/shaper/shaper_test.go` | AC-3, AC-5: PPPoE stored in `s.sessions` by canonical `SubscriberID`, visible in `show l2tp shaper`, updated via CoA, restored on `subevents.SessionDown` | |
| `TestCoSPPPoELifecycleAndRestore` | `internal/plugins/cos/handler_test.go` | AC-3, AC-18: PPPoE CoS applies via `subevents`, accepts CoA transaction by canonical `SubscriberID`, and restores captured baseline maps on final departure | |
| `TestCoAUnappliedAnswersNAK` | `internal/component/l2tp/plugins/authradius/coa_test.go` | AC-4, AC-15: failed owner returns CoA-NAK; PPPoE Disconnect-Request invokes `DisconnectHandler` and answers Disconnect-ACK | |
| `TestIKERekeyCarriesEstablishedAtAdvancesCreatedAt` | `internal/component/ike/engine/rfc7296_rekey_collision_test.go` | AC-9 on the initiator path (`applyIKERekeyResponse`) | |
| `TestIKERekeyBothDirectionsAgreeOnTimestamps` | `internal/component/ike/engine/rfc7296_rekey_collision_test.go` | AC-9: `applyIKERekeyResponse` and `respondIKERekey` produce identical `CreatedAt`/`EstablishedAt` semantics | |
| `TestFlowExportReloadPreservesSysUpTime` | `internal/plugins/flowexport/exporter_test.go` | AC-10 including `FIRST_SWITCHED`/`LAST_SWITCHED` continuity across reload | |
| `TestParsePollSleepMicroseconds` | `internal/component/vpp/config_test.go` | AC-13: parses `0us..100000us` and preserves byte-identical `ms` output | |
| `TestCheckVPPVersionRejectsUnsupportedRange` | `internal/plugins/iface/vpp/doctor_api_test.go` | AC-14: `checkVPPVersion` warns on versions below/above supported range and passes inside range | |
| `TestEmitWithoutSubscribeIsReported` | `internal/le/doc/wiring/wiring_emit_test.go` | AC-12: reports unsubscribed emitted topics and respects `wiringAllowlist` | |
| `TestPPPoESessionTimeoutEnforced` | `internal/component/l2tp/pppoe/subsystem_test.go` | AC-20: PPPoE `Session-Timeout` and `Idle-Timeout` fire with correct causes | |
| `TestSessionTimeoutsKeyedOnSubscriberSession` | `internal/component/l2tp/session_timeout_test.go` | AC-20: equal numeric pairs on L2TP and PPPoE keep independent timers | |
| `TestIdleTimeoutReadFailureIsNotIdle` | `internal/component/l2tp/session_timeout_test.go` | AC-21: counter read error logs a warning and does not tear the session down | |
| `TestPPPoEIfIndexAbove16BitsKeepsDistinctSessions` | `internal/component/l2tp/pppoe/subsystem_test.go` | AC-22: interfaces with ifindex 2 and 65538 and equal PPPoE SID stay distinct across driver, pool, and metadata | |
| `TestSubscriberRouteEventsKeepAccessAndLifetimeApart`, `TestPPPoEFramedRoutesFollowExactAuthenticationRound`, `TestPPPoETerminalPublicationHoldsSIDUntilSubscribersReturn`, `TestSubscriberRouteStopWithdrawsLiveContributions` | `internal/component/l2tp/`, `ppp/`, `pppoe/` | AC-16, AC-23 (D-14 already landed and verified at HEAD) | done |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `poll-sleep` microseconds | `0us..100000us` | `100000us` | negative (rejected by pattern) | `100001us` |
| `poll-sleep` sub-millisecond | `1us..999us` | `999us` | `0us` is valid | N/A |
| VPP supported major.minor version | configured supported range in `doctor_api.go` | highest supported minor | one minor below lowest | one minor above highest |
| `Acct-Session-Time` after N renegotiations | `0..2^32-1` seconds | monotonic across N renegotiations | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `pool-release-cycle` | `test/pppoe/pool-release-cycle.ci` | Repeated PPPoE connect/disconnect cycles beyond pool size without exhausting the pool (AC-1) | |
| `accounting-lifecycle` | `test/pppoe/accounting-lifecycle.ci` | PPPoE subscriber produces Accounting-Start, Interim-Update, and Accounting-Stop (AC-2) | |
| `coa-rate-change` | `test/pppoe/coa-rate-change.ci` | RADIUS CoA rate change updates a live PPPoE session's dataplane rate and answers CoA-ACK (AC-3, AC-4, AC-5) | |
| `coa-disconnect` | `test/pppoe/coa-disconnect.ci` | RADIUS Disconnect-Request tears down a live PPPoE session via PADT and answers Disconnect-ACK (AC-15) | |
| `renegotiation-preserves-session` | `test/l2tp/renegotiation-preserves-session.ci` | Mid-session LCP Configure-Request preserves `ActivatedAt`, `Acct-Session-Id`, `Session-Timeout`, and keeps Access-Request count at 1 (AC-6, AC-7, AC-8, AC-17, AC-19) | |
| `flowexport-reload-epoch` | `test/plugin/flowexport-reload-epoch.ci` | NetFlow v9 `sysUpTime` and flow timestamps stay monotonic across config reload (AC-10) | |
| `redistribute-subscriber-route` | `test/pppoe/redistribute-subscriber-route.ci` | BGP receives each PPPoE subscriber's assigned host route and RADIUS `Framed-Route` and withdraws them on disconnect (AC-16, AC-23) | done (80/80 guest iterations passed) |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `pppoe-accounting-freeradius` | `test/interop-pppoe/` | accel-ppp + FreeRADIUS | A real PPPoE peer produces exactly one accounting session (Start, Interim, Stop) | |
| `l2tp-renegotiation-xl2tpd` | `test/interop-l2tp/` | xl2tpd + pppd | A real peer's mid-session LCP renegotiation does not restart accounting or re-authenticate | |
| `ipsec-rekey-uptime-strongswan` | `test/interop-ipsec/` | strongSwan | `show ipsec` `uptime-seconds` continuity and `created-at` advancement across a locally-initiated IKE rekey | |

## Files to Modify
- `internal/component/l2tp/ppp/session_run.go` - keep logical session alive across mid-session LCP renegotiation (D-6, D-7, AC-17), emit `EventSessionParamsChanged` on repeat Opened
- `internal/component/l2tp/ppp/ncp.go` - keep dataplane address programming on every NCP-Opened entry across renegotiation
- `internal/component/l2tp/subscriber/events/events.go` - register `SessionParamsChanged` and `SessionCoSChange`
- `internal/component/l2tp/subscriber/handler_registry.go` - add transport-generic `DisconnectHandler` (I-4)
- `internal/component/l2tp/subscriber/registry.go` - preserve existing non-zero `ActivatedAt` in `Registry.Add` (D-5)
- `internal/component/l2tp/subscriber_bridge.go` - preserve `ActivatedAt` in `onSessionUp` (D-5) and bridge `SessionParamsChanged`
- `internal/component/l2tp/pppoe/subsystem.go` - handle `EventSessionParamsChanged`, start RADIUS session/idle timers on first activation (D-11), and register `DisconnectHandler` for PADT teardown (I-4)
- `internal/component/l2tp/pppoe/subsystem_access.go` - remove the obsolete 16-bit `discoveryIndexMax` (`1<<16 - 1`) cap while retaining positive and distinct-index checks (D-13, AC-22)
- `internal/component/l2tp/session_timeout.go` - preserve original `Session-Timeout` deadline across renegotiation (D-7), support PPPoE keyed by canonical `SubscriberID` (D-11), and treat `readIfaceRXBytes` errors as logged non-idle failures (D-12); fix stale `rtnetlink` comment
- `internal/component/l2tp/iface_stats_linux.go`, `internal/component/l2tp/iface_stats_other.go` - change `readIfaceRXBytes` to return `(uint64, error)` (D-12)
- `internal/component/l2tp/reactor_kernel.go` - distinguish renegotiation from terminal LCP down and avoid restarting `Session-Timeout` deadlines (D-6, D-7)
- `internal/component/l2tp/session_metadata_test.go` - add `TestSessionMetadataNoCrossTransportCollision` (AC-11)
- `internal/component/l2tp/plugins/authradius/acct.go` - migrate `subscribeEventBus` to `subevents` and key `sessions` by canonical `SubscriberID` (D-2)
- `internal/component/l2tp/plugins/authradius/coa.go` - emit `subevents.SessionCoSChange`, route PPPoE Disconnect-Request through `subscriber.GetDisconnectHandler()` (I-4), and keep CoA-NAK on unapplied changes (D-3)
- `internal/component/l2tp/plugins/authradius/coa_transaction.go` - pass `SubscriberID: sub.ID` to `request l2tp shaper rate` alongside diagnostic `TunnelID`/`SessionID` (D-3)
- `internal/component/l2tp/plugins/shaper/shaper.go`, `internal/component/l2tp/plugins/shaper/rate_transaction.go` - migrate completely to `subevents`, key `s.sessions` and rate transactions by canonical `SubscriberID`, record PPPoE on `SessionUp`, restore qdisc on `SessionDown`, and display PPPoE in `show l2tp shaper` (D-3, D-4)
- `internal/plugins/cos/handler.go`, `internal/plugins/cos/session_state.go`, `internal/plugins/cos/session_transaction.go` - migrate to `subevents.{SessionUp,SessionDown,SessionCoSChange}` and key `sessionStore` + CoA transactions by canonical `SubscriberID` so PPPoE gets dynamic CoS and baseline restoration (D-3, D-4, AC-18)
- `internal/component/ike/engine/rekey.go` - set `CreatedAt: time.Now()` and `EstablishedAt: oldSA.EstablishedAt` in both `applyIKERekeyResponse` and `respondIKERekey` (D-8)
- `internal/plugins/flowexport/exporter.go`, `internal/plugins/flowexport/register.go` - preserve boot `startTime` across config reloads (D-9)
- `internal/component/vpp/config.go`, `internal/component/vpp/yang/ze-vpp-conf.yang` - accept microsecond `poll-sleep` (`0us..100000us`) alongside `ms` (I-3)
- `internal/plugins/iface/vpp/doctor_api.go` - parse VPP version string and check supported major.minor range in `checkVPPVersion` (I-3, AC-14)
- `internal/le/doc/wiring/wiring.go`, `internal/le/doc/wiring/allowlist.go` - add emit-without-subscribe EventBus topic check and reviewed exemptions (I-1)
- `internal/test/fixture/tunnel_fixture_l2tp_ppp.go` - add mid-session LCP Configure-Request renegotiation mode (AC-19)
- `docs/architecture/l2tp/bng-1-radius-attributes.md` - update `Decisions` for mid-session LCP renegotiation preserving auth (D-6), `(uint64, error)` idle counter reads (D-12), and canonical `SubscriberID` accounting keys (D-2)
- `docs/architecture/l2tp/bng-5-pppoe.md` - update `Decisions` to remove the obsolete 16-bit tunnel-key range limit on PPPoE access ifindexes (D-13, AC-22)
- `docs/architecture/l2tp/subscriber-session-model.md` - update `The order of one L2TP session's notifications` and `Traps this code exists to avoid` for mid-session LCP renegotiation preserving session/auth/accounting and for PPPoE accounting on `subevents` (D-2, D-5, D-6, D-7, AC-17)
- `docs/guide/l2tp.md` - update `PPP negotiation` and `Authentication` for mid-session LCP renegotiation, PPPoE accounting on `subevents`, canonical `SubscriberID` accounting binding, and PPPoE Disconnect-Request support (D-2, D-6, I-4)
- `docs/research/l2tpv2-ze-integration.md` - update `What ze handles in userspace (control plane)`, `6.3 l2tp-shaper (traffic shaping)`, and `11.3 PPP session ownership` for mid-session LCP renegotiation and `subevents` shaper migration (D-4, D-6, D-7)
- `docs/architecture/traffic/cos-dynamic.md` - update `Decisions` for `subevents` subscription and canonical `SubscriberID` ownership (D-3, D-4)
- `docs/architecture/testing/interop.md` - add the zero-contention concurrency vacuity-trap row (I-2)
- `ai/rules/points/principles/directives/your-blast-radius-is-what-you-can-reach.md` - add the fresh-versus-restore sibling audit requirement, then sync via `./le ai rules update render` and `./le ai rules update condensed` (I-2)
- `rfc/short/rfc2865.md`, `rfc/short/rfc2866.md`, `rfc/short/rfc3954.md`, `rfc/short/rfc5176.md` - extract the missing lifecycle obligations with positive/negative tagged proofs
- `docs/features/rfc-status.md` - keep the RFC ledger in step with the summaries

## Files to Create
- `internal/component/l2tp/ppp/renegotiate_test.go` - behavioural PPP renegotiation unit tests (AC-6, AC-17)
- `internal/le/doc/wiring/wiring_emit_test.go` - Go unit tests for the emit-without-subscribe guard (AC-12)
- `test/pppoe/pool-release-cycle.ci`, `test/pppoe/accounting-lifecycle.ci`, `test/pppoe/coa-rate-change.ci`, `test/pppoe/coa-disconnect.ci` - PPPoE lifecycle functional tests (AC-1..AC-5, AC-15)
- `test/l2tp/renegotiation-preserves-session.ci` - mid-session LCP renegotiation functional test (AC-6, AC-7, AC-8, AC-17, AC-19)
- `test/plugin/flowexport-reload-epoch.ci` - NetFlow v9 reload epoch continuity functional test (AC-10)

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/component/vpp/yang/ze-vpp-conf.yang` for the widened `poll-sleep` leaf accepting `us` and `ms` |
| YANG validation constraints | Yes | `poll-sleep` pattern accepts `0..100000us` and `0..100ms` while rejecting free text and negative values |
| YANG custom validators | N-A | Native pattern plus parser range check is sufficient; no cross-leaf constraint |
| CLI commands/flags | No | No new command; `show l2tp shaper` and `show ipsec` gain accurate PPPoE/rekey data through existing commands |
| CLI grammar (keyword before value) | N-A | No new CLI command |
| Editor autocomplete | N-A | `poll-sleep` is a pattern-typed string with no enumerated values |
| Functional test for new RPC/API | Yes | The six new `.ci` files in `Files to Create` (plus already-landed `test/pppoe/redistribute-subscriber-route.ci`) |
| Pipe completeness | N-A | No new output-producing command |
| Env var registration | No | No new environment leaf |
| Doctor check for runtime dependencies | Yes | `internal/plugins/iface/vpp/doctor_api.go` `checkVPPVersion` gains version-range validation; code `doctor-vpp-version` is already registered |
| Doctor code per problem class | Yes | `doctor-vpp-version` covers unsupported VPP versions (`checkVPPVersion`); existing `doctor-radius-coa-apply` and `doctor-radius-accounting-unacknowledged` cover CoA and accounting failures |
| Prometheus counters/metrics | No | Existing counters (`ze_l2tp_*`, `ze_pppoe_*`, `ze_radius_*`) remain sufficient; I-1 is enforced statically at build/check time by `./le doc wiring` |
| BGP family surface (new SAFI / capability / attribute) | N-A | No BGP family surface is touched |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | `features/` entries rendering `docs/features.md` (PPPoE accounting, CoA, disconnect, shaping, and timers reach parity) |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md` for sub-millisecond `poll-sleep` (`us`) syntax |
| 3 | CLI command added/changed? | No | No CLI command added or removed |
| 4 | API/RPC added/changed? | No | Internal hidden CoA transaction commands keep their existing command names and add optional `subscriber-id` |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` for `authradius`, `shaper`, and `cos` subscriber-event parity |
| 6 | Has a user guide page? | Yes | `docs/guide/l2tp.md` (CoA/Disconnect PPPoE parity) and `docs/guide/monitoring.md` (NetFlow v9 reload epoch continuity) |
| 7 | Wire format changed? | No | External RFC wire formats are unchanged; only field values and internal event topics change |
| 8 | Plugin SDK/protocol changed? | No | EventBus and SDK contracts are unchanged |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc2865.md`, `rfc/short/rfc2866.md`, `rfc/short/rfc3954.md`, `rfc/short/rfc5176.md`, and `docs/features/rfc-status.md` |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` for `tunnel_fixture_l2tp_ppp.go` renegotiation mode |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` for PPPoE accounting/CoA/timer parity |
| 12 | Internal architecture changed? | Yes | `docs/architecture/l2tp/subscriber-session-model.md`, `docs/architecture/l2tp/bng-1-radius-attributes.md`, `docs/architecture/traffic/cos-dynamic.md`, `docs/architecture/testing/interop.md` |
| 13 | Route metadata keys added/changed? | No | Framed-Routes already reuse existing metadata keys (D-14) |
| 14 | Prometheus counters added/changed? | No | No new Prometheus counter added |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | `docs/plugin-overview.md` and `docs/features/plugins.md` for `subevents.SessionParamsChanged` and `subevents.SessionCoSChange` |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Update declared and anchored docs: `docs/architecture/l2tp/bng-1-radius-attributes.md` (idle error + canonical accounting key), `docs/architecture/traffic/cos-dynamic.md` (subevents + canonical key), `docs/architecture/l2tp/subscriber-session-model.md` (remove stale "PPPoE accounting is not connected" note, document `SessionParamsChanged`/`SessionCoSChange`/`DisconnectHandler`), `docs/architecture/l2tp/bng-5-pppoe.md` & `docs/guide/pppoe.md` (PPPoE timers/Disconnect/CoA/shaper/accounting), `docs/guide/l2tp.md` (PPPoE CoA/Disconnect parity), `docs/research/l2tpv2-ze-integration.md` (PPPoE shaper/accounting parity), `docs/architecture/flowexport/flow-export-1-counter-export.md`, `flow-export-2-flow-records.md`, & `docs/guide/flow-export.md` (reload epoch continuity), `docs/research/vpp-deployment-reference.md` & `docs/guide/vpp.md` (`poll-sleep` microsecond note), `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md`, `ipsec-13-rekey-wire.md`, & `docs/guide/ipsec.md` (IKE rekey timestamp note), `docs/architecture/core-design.md` & `docs/contributing/documentation-testing.md` (emit-without-subscribe check in `doc/wiring`), `docs/labs/l2tp-interop.md` (scripted peer renegotiation mode), `ai/rules/repo-maintenance.md` (unaffected — `doctor_api.go` stays plugin-owned). Remaining anchored pages (`cpe-1-pppoe-client.md`, `kernel-profiles.md`, `ipsec-14-responder.md`, `ipsec-7-ikev2-engine.md`, `rfcgate-1b-rfc7296-pilot.md`, `flow-export-0-umbrella.md`, `observation-feed.md`, `module-tiers.md`, `docs/DESIGN.md`, `docs/guide/command-reference.md`, `docs/guide/ddos-mitigation.md`, `docs/research/l2tpv2-implementation-guide.md`) verified unaffected by this change. |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | Verify `poll-sleep` examples and subscriber-session examples against the updated syntax and behaviour |

## Design presentation

### What changes

This spec finishes the lifecycle-invariant work across subscriber sessions (L2TP and PPPoE), IKEv2 rekeys, NetFlow v9 reloads, VPP operational checks, and wiring tooling. PPPoE subscribers gain full RADIUS accounting (Start/Interim/Stop), RADIUS `Session-Timeout` and `Idle-Timeout` enforcement, dynamic rate shaping (`show l2tp shaper`, CoA rate updates, teardown qdisc restore), dynamic CoS (assignment, CoA updates, baseline VLAN map restore), and RADIUS Disconnect-Request teardown, while mid-session PPP LCP renegotiation keeps an authenticated session's identity, accounting, and `Session-Timeout` deadline intact instead of re-authenticating or restarting billing. Standalone fixes preserve `EstablishedAt` and advance `CreatedAt` identically on both IKEv2 rekey directions, preserve NetFlow v9's boot `sysUpTime` epoch across config reloads, prevent failed RX counter reads from disconnecting healthy subscribers as idle, widen `vpp poll-sleep` to microsecond granularity, validate the VPP version range in `ze doctor`, and add a static emit-without-subscribe wiring check plus two testing/audit rule rows. Already-landed pieces (D-1 pool release code, D-10 metadata re-key, D-13 PPP driver canonical keying, D-14 PPPoE subscriber route redistribution and Framed-Route, `confirmCoA` 2-phase confirmation, CoS baseline capture/restore, and the pinned `ze-vpp-deployment` image) are preserved and completed with their remaining tests.

### Before and after

1. **PPPoE RADIUS accounting (D-2):** Before: PPPoE sessions produce zero Accounting-Start, Interim-Update, or Accounting-Stop packets because `authradius/acct.go` listens only to `l2tpevents`. After: `authradius/acct.go` consumes `subevents` keyed by canonical `SubscriberID`, so PPPoE sessions emit Start, scheduled Interims, and Stop with accurate terminate causes.
2. **PPPoE CoA rate/CoS & Disconnect-Request (D-3, D-4, I-4):** Before: CoA for a PPPoE session cannot update `shaper` or `cos` (neither keys on canonical `SubscriberID` nor subscribes to PPPoE `subevents`), `show l2tp shaper` omits PPPoE, and Disconnect-Request returns NAK 504. After: `confirmCoA` passes canonical `SubscriberID` to both owners, `shaper` and `cos` consume `subevents` and restore original qdisc/VLAN maps on teardown, unapplied CoA returns NAK, and Disconnect-Request sends PADT via `DisconnectHandler` and returns Disconnect-ACK.
3. **Mid-session PPP LCP renegotiation (D-5, D-6, D-7, AC-17):** Before: a peer Configure-Request in `Opened` tears down the network phase, re-sends a second RADIUS `Access-Request`, resets `ActivatedAt`, emits a second `Accounting-Start` with a fresh `Acct-Session-Id`, and restarts `Session-Timeout`. After: renegotiation keeps the authenticated session alive, emits `subevents.SessionParamsChanged`, applies changed MRU/IPCP parameters, and preserves `ActivatedAt`, `Acct-Session-Id`, `Acct-Session-Time`, and the original `Session-Timeout` deadline.
4. **PPPoE session/idle timers & counter read errors (D-11, D-12):** Before: PPPoE ignores RADIUS `Session-Timeout` and `Idle-Timeout`, and any sysfs read failure returns `0` and disconnects L2TP sessions as idle. After: both transports enforce both timers keyed by canonical `SubscriberID`, and counter read errors log a warning without disconnecting the session.
5. **IKEv2 rekey timestamps (D-8):** Before: initiator rekey resets `uptime-seconds` (`EstablishedAt = now`) while responder rekey freezes `created-at` (`CreatedAt = oldSA.EstablishedAt`). After: both directions set `CreatedAt = now` and carry `EstablishedAt = oldSA.EstablishedAt`, matching RFC 7296 Sections 2.18 and 2.8.3.
6. **NetFlow v9 reload epoch (D-9):** Before: `ze config reload` resets `exporter.startTime` to `now`, making `sysUpTime`, `FIRST_SWITCHED`, and `LAST_SWITCHED` jump backwards. After: `flowexport` keeps its boot `startTime` across reloads so timestamps remain monotonic.
7. **VPP `poll-sleep` and `ze doctor` version check (I-3):** Before: `poll-sleep` rejects sub-millisecond values like `500us`, and `checkVPPVersion` passes any output containing `"vpp "`. After: `poll-sleep` accepts `0us..100000us` (preserving `ms` output byte-for-byte) and `checkVPPVersion` parses the version and warns when outside the supported range.
8. **Build-time wiring guard & rules (I-1, I-2):** Before: an emitted EventBus topic with zero subscribers (like `subevents.SessionRateChange`) compiles and passes CI silently. After: `./le doc wiring` flags any unsubscribed emitted topic not in `wiringAllowlist`.

### Alternatives set aside

| Alternative | Why set aside |
|-------------|---------------|
| Keep `l2tpevents` subscriptions alongside `subevents` in `acct`, `shaper`, and `cos` | `subscriberBridge` re-emits L2TP events onto `subevents`, so subscribing to both double-fires every L2TP lifecycle event and violates `subscriber-session-model.md`. |
| Suppress LCP renegotiation side effects solely in downstream consumers without fixing `ppp/session_run.go` | Downstream dedupe alone still lets `LCPActTLD` tear down metadata/timers and lets `afterLCPOpen` send a second RADIUS `Access-Request` (violating AC-17). |
| Keep `L2TPBirth` keys in `cos` and `acct` and add a second parallel PPPoE map beside them | Splits one lifecycle owner into two transport-specific paths (`ai/rules/no-layering.md`); canonical `SubscriberID` already embeds `L2TPBirth` for L2TP and full `ifIndex`+SID for PPPoE. |
| Use `EventBus.Emit`'s returned subscriber count at runtime for I-1 | `pkg/ze/eventbus.go` explicitly excludes in-process subscribers from `Emit`'s count; a static check in `internal/le/doc/wiring/wiring.go` catches dead topics before runtime. |
| Add a third timestamp field to `show ipsec` for IKE rekey continuity | RFC 7296 already separates SA creation (`CreatedAt`, Section 2.18) from authenticated relationship (`EstablishedAt`, Section 2.8.3), and Ze already exposes both fields. |

### Documents departed from

1. **`docs/architecture/l2tp/bng-1-radius-attributes.md` (`Decisions`):**
   - **Before:** `"In PPP, LCP opens before authentication, so the Access-Accept arrives after the link is agreed: renegotiating LCP would restart authentication."`, `"The non-Linux build returns 0, so the timer fires unconditionally there."`, and `"Records are keyed by the session's Birth plus its wire labels."`
   - **After:** Mid-session LCP renegotiation on an already-authenticated session preserves its authentication rather than restarting it (Framed-MTU still lowers the interface MTU rather than renegotiating LCP, D-6/AC-17); `readIfaceRXBytes` returns `(uint64, error)` on both Linux and non-Linux so a failed counter read logs a warning and does not tear down the session as idle (D-12); accounting records in `authradius/acct.go` are keyed by canonical `SubscriberID` (embedding `L2TPBirth` for L2TP and full `ifIndex`+SID for PPPoE) so PPPoE and L2TP share one collision-free accounting map (D-2). Updated in `docs/architecture/l2tp/bng-1-radius-attributes.md` in the same commit.
2. **`docs/architecture/traffic/cos-dynamic.md` (`Subscribe to the L2TP session event, not the subscriber event`, `Session identity and shared access ownership are separate`, `CoA waits for the owner to apply its maps`):**
   - **Before:** `"The lifecycle producer supplies its cached canonical subscriber ID for the AuthMetadata.CoSProfile lookup and its packed Birth for CoS ownership."`, `"The package-level sessionStore is a sync.Map keyed by sessionKey, the original packed session Birth."`, and `"The decoder parses the canonical ID once and retains its Birth as the ownership key."`
   - **After:** Dynamic CoS subscribes to `subevents.{SessionUp,SessionDown,SessionCoSChange}` and keys `sessionStore` and `session_transaction.go` by canonical `SubscriberID` validated via `subscriber.RequireID`, keeping L2TP's Birth uniqueness inside its canonical ID while admitting PPPoE sessions into dynamic CoS assignment, CoA transactions, and baseline restoration. Updated in `docs/architecture/traffic/cos-dynamic.md` in the same commit.
3. **`docs/architecture/l2tp/subscriber-session-model.md` (`The order of one L2TP session's notifications`, `Traps this code exists to avoid`):**
   - **Before:** Table row `"| Session-down that ends a network lifetime (LCP down in the network phase) | handleLCPDown | notifyNetworkLifetimeDown | ClearSessionMetadata, then PPP's acknowledgement |"`, plus `"LCP-down clears the accepted username and the transport's network-lifetime identity, so replacement admission cannot reuse the previous subscriber."`, `"LCP-down ends the network lifetime without ending the transport. Each transport's route observer withdraws that lifetime's routes before the transport acknowledges LCP-down. Both transports also release the old allocation and authorization state before acknowledgment. PPP waits for it before it can start replacement authentication."`, `"LCP-down stops that session before replacement authentication."`, and `"PPPoE accounting is not connected to subscriber events."`
   - **After:** Mid-session LCP renegotiation on an already-established session preserves the accepted username, authorization, pool allocation, routes, timers, and active accounting session (`Acct-Session-Id` and monotonic `Acct-Session-Time`), emitting `subevents.SessionParamsChanged` instead of running `handleLCPDown` -> `notifyNetworkLifetimeDown` -> `ClearSessionMetadata`, while still applying updated LCP/NCP dataplane parameters (terminal LCP Closed/Stopped still clears and withdraws everything in ticket order, D-5/D-6/D-7/AC-17); PPPoE accounting is connected to `subevents` via `authradius/acct.go` (D-2). Updated in `docs/architecture/l2tp/subscriber-session-model.md` in the same commit.
4. **`docs/guide/l2tp.md` (`PPP negotiation`, `Authentication`):**
   - **Before:** `"Leaving LCP Opened ends the previous PAP decision and both NCP lifetimes. Ze disconnects an attached channel from its PPP unit before replacement authentication and NCP, so control frames return to the channel reader."`, `"PPPoE accounting remains unconnected to subscriber events."`, `"A matching Disconnect-Request for a PPPoE session still gets NAK 504, because PPPoE teardown from a Disconnect-Request is not connected yet."`, and `"The accounting owner binds its issued ID to the original Birth. A numeric tunnel/session pair cannot supply an accounting identity for a replacement."`
   - **After:** Mid-session LCP renegotiation on an already-established session preserves the authenticated session and network lifetime without running replacement authentication (D-6, AC-17); PPPoE accounting consumes `subevents` and binds accounting records to canonical `SubscriberID` (which carries `L2TPBirth` for L2TP and full `ifIndex`+SID for PPPoE, D-2); matching PPPoE Disconnect-Requests invoke `subscriber.DisconnectHandler`, send PADT, and answer Disconnect-ACK (I-4, AC-15). Updated in `docs/guide/l2tp.md` in the same commit.
5. **`docs/research/l2tpv2-ze-integration.md` (`What ze handles in userspace (control plane)`, `6.3 l2tp-shaper (traffic shaping)`, `11.3 PPP session ownership`):**
   - **Before:** `"An LCP restart clears the cached decision."`, `"Subscribe to `(l2tp, session-up)` to apply initial shaping."`, `"Clean up TC rules on `(l2tp, session-down)`."`, and `"Leaving LCP Opened first disconnects the PPP channel and removes its NCP resources. EventLCPDown then waits for the transport's network-lifetime cleanup acknowledgment before the accompanying LCP responses are sent. Authentication and NCP negotiate anew on the same transport."`
   - **After:** Mid-session LCP renegotiation preserves an established session's authentication decision, network lifetime, metadata, accounting, and timeouts without disconnecting to re-authenticate (while applying updated MRU/IPCP parameters; terminal LCP Closed/Stopped keeps full teardown, D-6/D-7/AC-17), and `l2tp-shaper` subscribes to `subevents.SessionUp`, `subevents.SessionDown`, and `subevents.SessionRateChange` keyed by canonical `SubscriberID` for both L2TP and PPPoE (D-4). Updated in `docs/research/l2tpv2-ze-integration.md` in the same commit.
6. **`docs/architecture/l2tp/bng-5-pppoe.md` (`Decisions`):**
   - **Before:** `"Every prepared access must name that namespace and a distinct Linux index within the shared PPP driver's 16-bit tunnel-key range; discovery dispatches by the index from the receive call."`
   - **After:** Because `ppp/manager.go`, `pool`, and `session_metadata.go` now key sessions by canonical `SubscriberID` carrying the full `int` ifindex, `pppoe/subsystem_access.go` removes the obsolete 16-bit `discoveryIndexMax` (`1<<16 - 1`) cap and admits any valid distinct positive Linux interface index, including indexes above 65535 (D-13, AC-22). Updated in `docs/architecture/l2tp/bng-5-pppoe.md` in the same commit.

### Choices for Thomas

none

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** - register the new subscriber topics and transport-generic disconnect handler, and add the failing wiring tests for open items
   - Tests: open rows of the Wiring Test table (`pool-release-cycle.ci`, `accounting-lifecycle.ci`, `coa-rate-change.ci`, `coa-disconnect.ci`, `renegotiation-preserves-session.ci`, `flowexport-reload-epoch.ci`, `TestCheckVPPVersionRejectsUnsupportedRange`, `TestEmitWithoutSubscribeIsReported`)
   - Files: `internal/component/l2tp/subscriber/events/events.go`, `internal/component/l2tp/subscriber/handler_registry.go`
   - Verify: `SessionParamsChanged`, `SessionCoSChange`, and `DisconnectHandler` exist and are discoverable, and each new wiring test fails because its consumer/handler is still unimplemented
2. **Phase: Renegotiation harness (BEFORE any dependent phase)** - extend the Go scripted L2TP/PPP peer to send a mid-session LCP Configure-Request
   - Tests: AC-19, proven by driving a mid-session Configure-Request against unfixed code and observing the duplicate Access-Request / Accounting-Start
   - Files: `internal/test/fixture/tunnel_fixture_l2tp_ppp.go`, `docs/functional-tests.md`
   - Verify: the harness reproduces D-6 / AC-17 before any fix lands; if blocked, stop and report rather than dropping to unit-only proof
3. **Phase: Keyspace & >16-bit ifindex completion (D-10 / D-13)** - remove the obsolete 16-bit `discoveryIndexMax` cap in `pppoe/subsystem_access.go`, prove canonical `SubscriberID` isolation under cross-transport collisions and >16-bit ifindexes, update `bng-5-pppoe.md`, and close journal row D-13
   - Tests: `TestSessionMetadataNoCrossTransportCollision`, `TestSubscriberIDBuildersNeverCollide`, `TestPPPoEIfIndexAbove16BitsKeepsDistinctSessions` (AC-11, AC-22)
   - Files: `internal/component/l2tp/pppoe/subsystem_access.go`, `internal/component/l2tp/session_metadata_test.go`, `internal/component/l2tp/subscriber/registry_test.go`, `internal/component/l2tp/pppoe/subsystem_test.go`, `docs/architecture/l2tp/bng-5-pppoe.md`
   - Verify: `prepareAccessConfiguration` accepts distinct positive ifindexes above 65535; cross-transport equal numeric pairs and ifindex 2 vs 65538 stay completely isolated across driver, pool, and metadata
4. **Phase: Topic split & renegotiation latch (D-5, D-6, D-7, AC-17)** - emit `SessionUp` once per logical session, emit `SessionParamsChanged` on mid-session renegotiation without re-running auth or tearing down accounting/metadata/timeouts, keep dataplane updates (`SetMTU`/`AddAddressP2P`), and preserve `ActivatedAt`
   - Tests: `TestRenegotiationEmitsParamsChangedNotSessionUp`, `TestRenegotiationDoesNotRerunAuth`, `TestRenegotiationStillProgramsDataplane`, `TestRegistryAddPreservesActivatedAt`, `TestStartSessionTimeoutsPreservesDeadline`, `test/l2tp/renegotiation-preserves-session.ci`
   - Files: `internal/component/l2tp/ppp/session_run.go`, `internal/component/l2tp/ppp/ncp.go`, `internal/component/l2tp/subscriber/registry.go`, `internal/component/l2tp/subscriber_bridge.go`, `internal/component/l2tp/pppoe/subsystem.go`, `internal/component/l2tp/reactor_kernel.go`, `internal/component/l2tp/session_timeout.go`
   - Verify: `TestHandleLCPPacketOpenedRXRDoesNotReenterOpened`, `TestRFC1661RenegotiateOnConfigureRequestInOpened`, and `renegotiation-preserves-session.ci` all pass; Access-Request count == 1 and Accounting-Start count == 1
5. **Phase: Consumer migration & PPPoE parity (D-1 `.ci`, D-2, D-3, D-4, D-11, D-12, I-4 Disconnect)** - migrate `authradius/acct`, `shaper`, and `cos` one consumer at a time onto `subevents` + canonical `SubscriberID`, wire PPPoE `Session-Timeout`/`Idle-Timeout` and non-idle counter read error handling, and wire PPPoE Disconnect-Request
   - Tests: `TestAcctPPPoELifecycleViaSubEvents`, `TestAcctNoDuplicateStartOnRepeatedIPAssigned`, `TestAcctSessionTimeSurvivesRenegotiation`, `TestShaperRecordsPPPoESession`, `TestCoSPPPoELifecycleAndRestore`, `TestCoAUnappliedAnswersNAK`, `TestPPPoESessionTimeoutEnforced`, `TestSessionTimeoutsKeyedOnSubscriberSession`, `TestIdleTimeoutReadFailureIsNotIdle`, plus `test/pppoe/{pool-release-cycle,accounting-lifecycle,coa-rate-change,coa-disconnect}.ci`
   - Files: `internal/component/l2tp/plugins/authradius/{acct,coa,coa_transaction}.go`, `internal/component/l2tp/plugins/shaper/{shaper,rate_transaction}.go`, `internal/plugins/cos/{handler,session_state,session_transaction}.go`, `internal/component/l2tp/pppoe/subsystem.go`, `internal/component/l2tp/{session_timeout,iface_stats_linux,iface_stats_other}.go`, `docs/architecture/l2tp/bng-1-radius-attributes.md`, `docs/architecture/traffic/cos-dynamic.md`
   - Verify: run L2TP package tests after each consumer migration (A-1); verify `test/pppoe/redistribute-subscriber-route.ci` remains green; close journal rows D-11 and D-12
6. **Phase: Standalone defects (D-8, D-9)** - align IKEv2 rekey timestamps in both directions and preserve NetFlow v9 boot `startTime` across reloads
   - Tests: `TestIKERekeyCarriesEstablishedAtAdvancesCreatedAt`, `TestIKERekeyBothDirectionsAgreeOnTimestamps`, `TestFlowExportReloadPreservesSysUpTime`, `test/plugin/flowexport-reload-epoch.ci`, `ipsec-rekey-uptime-strongswan`
   - Files: `internal/component/ike/engine/rekey.go`, `internal/plugins/flowexport/{exporter,register}.go`
   - Verify: initiator and responder IKE rekey produce identical `CreatedAt`/`EstablishedAt` semantics (A-6); NetFlow v9 `sysUpTime` stays monotonic across reload
7. **Phase: VPP operational items (I-3)** - microsecond `poll-sleep` granularity and `checkVPPVersion` range check
   - Tests: `TestParsePollSleepMicroseconds`, `TestCheckVPPVersionRejectsUnsupportedRange`
   - Files: `internal/component/vpp/config.go`, `internal/component/vpp/yang/ze-vpp-conf.yang`, `internal/plugins/iface/vpp/doctor_api.go`
   - Verify: existing whole-millisecond configs produce byte-identical `startup.conf` (A-5); out-of-range VPP versions emit `doctor-vpp-version` warning naming found version and supported range
8. **Phase: Guards and rules (I-1, I-2)** - add the Go emit-without-subscribe predicate in `docwiring` and add the two rule rows at their canonical sources
   - Tests: `TestEmitWithoutSubscribeIsReported` (`internal/le/doc/wiring/wiring_emit_test.go`)
   - Files: `internal/le/doc/wiring/wiring.go`, `internal/le/doc/wiring/allowlist.go`, `docs/architecture/testing/interop.md`, `ai/rules/points/principles/directives/your-blast-radius-is-what-you-can-reach.md` (then run `./le ai rules update render` and `./le ai rules update condensed`)
   - Verify: `./le doc wiring` passes cleanly on HEAD and flags any synthetic emitted topic with zero subscribers not in `wiringAllowlist`
9. **Phase: RFC extraction and ledger** - add the missing RFC 2865, 2866, 3954, and 5176 requirement rows with positive and negative tagged proofs and update `docs/features/rfc-status.md`
   - Tests: tagged positive and negative Go/functional tests for each added requirement ID
   - Files: `rfc/short/rfc2865.md`, `rfc/short/rfc2866.md`, `rfc/short/rfc3954.md`, `rfc/short/rfc5176.md`, `docs/features/rfc-status.md`
   - Verify: `./le rfc check` passes with zero errors and `check_gap_count_agreement` holds (R-7)

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-23 has an implementation (or verified HEAD baseline) at file and symbol |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | Renegotiation still applies changed MRU and IPCP address; only first-activation side effects are suppressed; failed RX counter reads never tear down sessions |
| Naming | New topics (`SessionParamsChanged`, `SessionCoSChange`) and `DisconnectHandler` follow existing package conventions; `poll-sleep` microsecond suffix uses `us` |
| Data flow | No migrated lifecycle consumer (`pool`, `route_observer`, `acct`, `shaper`, `cos`, timers) keys session state on `TunnelID`/`SessionID` or `L2TPBirth`-only structs |
| Operator errors and doctor codes | `checkVPPVersion` emits `doctor-vpp-version` with found version, supported range, and fix; CoA/accounting failures use existing registered doctor codes |
| Rule: `ai/rules/rfc-compliance.md` | Every added requirement row carries positive and negative tagged tests; no row is added ahead of its proof |
| Rule: `ai/rules/goroutine-lifecycle.md` | No timeout goroutine outlives the session that started it |
| Rule: `ai/rules/interop-and-goal-validation.md` | Each new test was proven to go red with its fix reverted, and the result recorded |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| No migrated lifecycle consumer subscribes to `l2tpevents` | grep `pool/register.go`, `route_observer.go`, `authradius/acct.go`, `shaper/shaper.go`, `cos/handler.go` for `l2tpevents` and expect zero matches |
| Every emitted EventBus topic has a subscriber or `wiringAllowlist` rationale | `./le doc wiring` |
| PPPoE accounting reaches the wire | `test/pppoe/accounting-lifecycle.ci` captured RADIUS packets |
| RFC gates hold | `./le rfc check` |
| Pool does not leak on PPPoE | `test/pppoe/pool-release-cycle.ci` run for more cycles than the pool holds addresses |
| Docs updated for all six departed design pages | `docs/architecture/l2tp/bng-1-radius-attributes.md`, `docs/architecture/traffic/cos-dynamic.md`, `docs/architecture/l2tp/subscriber-session-model.md`, `docs/guide/l2tp.md`, `docs/research/l2tpv2-ze-integration.md`, and `docs/architecture/l2tp/bng-5-pppoe.md` match implementation |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | A peer-driven renegotiation must not let an unauthenticated party restart auth, extend `Session-Timeout`, or create unbounded goroutines; `subscriber-id` in hidden CoA commands is validated via `subscriber.RequireID` |
| Resource exhaustion | A subscriber renegotiating in a loop must not grow memory, goroutines, or RADIUS traffic without bound |
| Authorization fail-open | A CoA that cannot be applied must answer CoA-NAK; answering CoA-ACK tells the RADIUS server a policy was enforced when it was not |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The Echo re-entry fix and root cause 2 are one bug seen twice. The guard that closed the Echo case is the guard renegotiation walks through, because renegotiation genuinely leaves `Opened` and Echo does not. A guard keyed on state transition cannot express "once per session"; only a latch on the session can.
- A test can be a faithful proof of its requirement and still miss the defect the requirement exists to prevent. `TestRFC1661RenegotiateOnConfigureRequestInOpened` proves the transition table, which is what "be prepared to renegotiate" means at the FSM layer and nothing more.
- Delegating a handler registry and delegating an event subscription look alike from the call site and behave completely differently. That asymmetry is what let the subscriber-session-model work read as complete.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `invariant-enforced-by-an-absent-call-site.md` | 2026-10-28 | `startSessionTimeouts` never reached for PPPoE (D-11, AC-20) |
| `zero-value-as-valid-answer.md` | 2026-10-28 | `readIfaceRXBytes` failure read as idle (D-12, AC-21) |
| `key-omits-a-fact-the-builder-uses.md` | 2026-11-15 | PPPoE ifindex cut to 16 bits in the PPP driver key and `PPPKey` (D-13, AC-22; remove leftover `discoveryIndexMax` cap in `pppoe/subsystem_access.go`, add unit proof, and close row) |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Split the session-up topic so it fires once, and add `SessionParamsChanged` | make each consumer idempotent; suppress all side effects at the source with one latch | Every existing consumer assumes once-per-session activation; emitting `SessionParamsChanged` keeps six consumers correct without discarding legitimate MRU or IPCP address updates. |
| Migrate `authradius/acct`, `shaper`, and `cos` onto `subevents` alone and canonical `SubscriberID` keys (following already-landed D-1 `pool` and D-14 `route_observer`) | subscribe to both `l2tpevents` and `subevents`; maintain separate L2TP `L2TPBirth` and PPPoE maps | Subscribing to both double-fires L2TP via `subscriberBridge`; canonical `SubscriberID` already encodes `L2TPBirth` for L2TP and full `ifIndex`+SID for PPPoE, keeping one collision-free ownership path for both transports. |
| Keep `l2tp.Observer` on `l2tpevents` | migrate `l2tp.Observer` to `subevents` | `l2tp.Observer` tracks L2TP-generation-scoped CQM (`L2TPBirth` + `EchoRTT`), and PPPoE explicitly holds no CQM state (`pppoe/subsystem.go:472`). |
| Extend the existing Go `internal/le/doc/wiring/wiring.go` checker and `wiringAllowlist` for I-1 | a new standalone AST checker; runtime subscriber count from `EventBus.Emit` | `docwiring` is already wired into `./le doc wiring` with a reviewed allowlist; `Emit`'s count explicitly excludes in-process subscribers. |
| On IKEv2 rekey, advance `CreatedAt` and carry `EstablishedAt` forward in both directions | stamp `now` on both fields; carry `oldSA.EstablishedAt` on both fields; add a third field | RFC 7296 Section 2.18 defines the rekeyed SA as a new SA while Section 2.8.3 preserves the authentication relationship; Ze already exposes both fields in `show ipsec`. |
| D-14 (approved 2026-11-19 and landed at HEAD): `StartSubscriberRouteObserver` consumes `subevents.{SessionIPAssigned,SessionDown}` for both transports, `pppoe` registers its own redistribute source, and PPPoE binds `Framed-Route` to the exact auth round | PPPoE calling `RouteObserver` directly; sharing the `l2tp` redistribute source | One event-driven path avoids transport duplication while independent `l2tp` and `pppoe` redistribute sources preserve existing operator configs; verified by `test/pppoe/redistribute-subscriber-route.ci` (80/80 guest iterations) and strict `l2tp-many-sessions-kl2tpd`. |
| Build the Go renegotiation fixture (`tunnel_fixture_l2tp_ppp.go`) before the phases that depend on it | defer to unit tests only | Unit tests missed D-6/AC-17 originally; proving reproduction against unfixed code first validates the functional test harness before code lands. |

## Known Limitations
- **Two `SessionDown` topics remain while `subscriberBridge` exists.** After this spec migrates `acct`, `shaper`, and `cos` onto `subevents`, only `l2tp.Observer` (L2TP CQM) and `subscriberBridge` itself remain on `l2tpevents`. Retiring `subscriberBridge` by having the L2TP reactor emit `subevents` directly is outside this fix spec's scope.
- IPv6-only PPPoE subscribers are not separately covered by the new `.ci` accounting and pool tests (which exercise IPv4); IPv6CP and delegated prefixes remain covered by existing pool/IPv6 tests.
- RFC 7296 Section 2.8.3 says full reauthentication (from-scratch `IKE_SA_INIT`/`IKE_AUTH`) starts a fresh `EstablishedAt`; AC-9 asserts rekey behaviour only.
- The IKE and flow-export defects share no code with the subscriber work; they ride along because they violate the same first-activation lifecycle invariant.

## Review Gate

Filled at implementation time by `/ze-review`, recorded via `internal/le/spec/session/review.go`. Do not delete.

| Run | Date | BLOCKER | ISSUE | Notes |
|-----|------|---------|-------|-------|

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: validation rules, error conditions, state transitions, timer
constraints, message ordering, and every MUST/MUST NOT.

Treat the RFC as the required baseline, not a ceiling on explicit operator
control. Assess compliance on the default configuration path. Preserve explicit
options for more specific behavior and extensions, including behavior that differs
from the RFC.

| RFC / section | Default configuration and required behavior | Explicit options / extensions to preserve | Tests of default behavior | Tests preserving options / extensions |
|---------------|---------------------------------------------|-------------------------------------------|---------------------------|---------------------------------------|
| RFC 1661 Section 4.3 | Renegotiate LCP options on peer Configure-Request in `Opened` without restarting auth or tearing down logical subscriber session | Configured `lcp` timers, `mru`, and `proxy-lcp` options | `TestRFC1661RenegotiateOnConfigureRequestInOpened`, `TestRenegotiationEmitsParamsChangedNotSessionUp`, `TestRenegotiationDoesNotRerunAuth`, `test/l2tp/renegotiation-preserves-session.ci` | `TestRenegotiationStillProgramsDataplane` |
| RFC 2865 Sections 5.27 & 5.28 | Enforce `Session-Timeout` from first service activation (no restart on renegotiation) and `Idle-Timeout` on genuine inactivity for both L2TP and PPPoE | Local config without RADIUS timeouts starts no timers | `TestStartSessionTimeoutsPreservesDeadline`, `TestPPPoESessionTimeoutEnforced`, `TestIdleTimeoutReadFailureIsNotIdle` | `TestSessionTimeoutsKeyedOnSubscriberSession` |
| RFC 2866 Sections 5.5 & 5.7 | Emit one `Acct-Session-Id` and monotonic `Acct-Session-Time` per logical session across renegotiation for both L2TP and PPPoE | `attributes exclude` and local `acct-interval` override | `TestAcctPPPoELifecycleViaSubEvents`, `TestAcctNoDuplicateStartOnRepeatedIPAssigned`, `TestAcctSessionTimeSurvivesRenegotiation`, `test/pppoe/accounting-lifecycle.ci` | Existing `exclude` and `acct-interval` unit tests |
| RFC 3954 Section 5.1 | `sysUpTime` reflects milliseconds since boot and `FIRST_SWITCHED`/`LAST_SWITCHED` remain relative to that epoch across config reload | Collector `max-datagram-size`, template refresh intervals, IPFIX absolute timestamps | `TestFlowExportReloadPreservesSysUpTime`, `test/plugin/flowexport-reload-epoch.ci` | Existing IPFIX and sFlow exporter tests |
| RFC 5176 Sections 3.1, 3.3, 3.5 | Return CoA-ACK / Disconnect-ACK only when applied to the subscriber session (L2TP or PPPoE); answer NAK when unapplied | Strict `Event-Timestamp` replay check and replay cache | `TestCoAUnappliedAnswersNAK`, `TestShaperRecordsPPPoESession`, `TestCoSPPPoELifecycleAndRestore`, `test/pppoe/coa-rate-change.ci`, `test/pppoe/coa-disconnect.ci` | Existing CoA replay/Event-Timestamp tests |
| RFC 7296 Sections 2.8.3 & 2.18 | New rekeyed IKE SA advances `CreatedAt` while preserving `EstablishedAt` in both initiator and responder roles | Full reauthentication starts a fresh `EstablishedAt` | `TestIKERekeyCarriesEstablishedAtAdvancesCreatedAt`, `TestIKERekeyBothDirectionsAgreeOnTimestamps`, `ipsec-rekey-uptime-strongswan` | Existing responder reauth tests |

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable positive observable assertions and `Permitted Survivors and Why`
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "Doctor check" and "YANG schema" accurately

### Goal Gates (MUST pass)
- [ ] AC-1..AC-23 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout by the main thread: the scoped package tests, the component groups and functional suites `./le repo changed scope print both` reaches. No agent runs any `./le verify` command: GitHub CI verifies the pushed commit, and the full verification is Thomas's release check. A red CI reports gets a fix commit
- [ ] `./le rfc check` passes with the four summaries extended
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied: lint fixed rather than disabled, the focused check for the changed behavior run once with its OUTPUT PASTED, and any red named with the one-line reason it is scaffolding
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features
- [ ] Each new test proven to go red with its fix reverted, result recorded

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + journal row updates
- [ ] **Commit B:** `remove plan/release/fixes/spec-lifecycle-invariants.md` only, in the same `./le commit create` script (commit A preserves the spec in history)
