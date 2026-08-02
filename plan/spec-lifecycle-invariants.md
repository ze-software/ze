# Spec: lifecycle-invariants

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Deferral shard | `plan/deferrals/lifecycle-invariants.md` |
| Updated | 2026-08-02 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**One invariant, violated in four places: a fact or a step that belongs to the
FIRST activation of a thing must not be recomputed, re-sent, or skipped when a
LATER event touches the same thing.**

Found on 2026-08-02 by taking the three bug classes fixed in
veesix-networks/osvbng during July 2026 and hunting each one in ze. Every defect
was verified by reading the producing function.

### Root cause 1: the subscriber event namespace has one consumer

`plan/learned/760-subscriber-session-model.md` built a transport-generic
`subscriber` event namespace so PPPoE and L2TP sessions would drive the same
downstream plugins. It states the problem it existed to solve: *"PPPoE did not
emit EventBus events. Auth, pool, shaping, accounting, and CoA were wired only to
L2TP."*

Half of it landed. PPPoE emits `subevents.SessionUp`, `SessionIPAssigned` and
`SessionDown` (`pppoe/subsystem.go`, `Subsystem.eventConsumer`), and nothing
under `pppoe/` emits `l2tpevents.*`. But across `internal/`, the only non-test
subscriber to any `subevents.*` topic is `subevents.SessionAuthResult` in
`subscriber_bridge.go`. Every lifecycle consumer still subscribes to
`l2tpevents`, which PPPoE never emits.

The handler REGISTRIES were delegated (`l2tp.RegisterPoolHandler` forwards to
`subscriber.RegisterPoolHandler`), which is why PPPoE address ALLOCATION works.
Release is not a handler, it is an event subscription, so it does not.

### Root cause 2: PPP renegotiation defeats the re-entry guard

`TestHandleLCPPacketOpenedRXRDoesNotReenterOpened` (`ppp/echo_test.go`) records
in its own docstring: *"PREVENTS: regression of the known re-entry bug where
every Echo packet in Opened triggered setMRU, SetMTU, auth, NCP, and
EventSessionUp again."* That bug was fixed with a `cur != LCPStateOpened` guard.

Renegotiation defeats that exact guard. `LCPDoTransition` (`ppp/ppp_fsm.go`) maps
`Opened` plus a peer Configure-Request to `AckSent`, then `AckSent` plus
Configure-Ack back to `Opened`, so `cur != Opened` holds on the way back in and
every side effect the Echo test enumerates runs again, **including auth**. RFC
1661 Section 4.3 requires an implementation to be ready for this at any time, so
it is normal traffic, not an error path.

### Root cause 3: a replacement object stamped with a new birth time

Two instances, one of them a disagreement between sibling functions in one file.

### Root cause 4: two namespaces sharing one keyspace

Found during research for this spec.

## Verified defects

| # | Producing function | Defect |
|---|--------------------|--------|
| D-1 | `poolPlugin.onSessionDown` (`l2tp/plugins/pool/register.go`) | reached only from `l2tpevents.SessionDown.Subscribe`, so a PPPoE session never releases its pool address. The pool drains monotonically to exhaustion; recovery needs a daemon restart |
| D-2 | `radiusAcct.SubscribeEventBus` (`l2tp/plugins/authradius/acct.go`) | binds `l2tpevents` only: no Accounting-Start, no interim, no Stop for any PPPoE session |
| D-3 | `coaListener.handleCoA` (`l2tp/plugins/authradius/coa.go`) | emits `subevents.SessionRateChange`, which has no subscriber, then returns `CodeCoAACK`. RFC 5176 Section 3.1 reserves ACK for a change that was applied. Its CoS branch emits `l2tpevents.SessionCoSChange` keyed on `subSess.TunnelID`/`SessionID`, which are zero for a PPPoE session |
| D-4 | `shaper.handleSubscriberSessionUp` (`l2tp/plugins/shaper/shaper.go`), `newCosHandler` (`plugins/cos/handler.go`) | the shaper's PPPoE entry calls `applyTC` and returns without writing `s.sessions`, so `onSessionRateChange` warns "unknown session", `onSessionDown` returns before `RestoreOriginal`, and `showSessions` omits it. It also skips `LoadSessionMetadata`, so PPPoE always gets the default rate. CoS subscribes to `l2tpevents` only |
| D-5 | `subscriberBridge.onSessionUp` (`l2tp/subscriber_bridge.go`), `Subsystem.eventConsumer` (`pppoe/subsystem.go`) | both construct a fresh `subscriber.Session` with `ActivatedAt: time.Now()`; `Registry.Add` overwrites unconditionally. A re-fired SessionUp resets the activation time and blanks `Username`. The sibling handlers `onAuthResult` and `onSessionIPAssigned` correctly read-modify-write |
| D-6 | `radiusAcct.onSessionIPAssigned` (`l2tp/plugins/authradius/acct.go`) | overwrites `a.sessions[key]` with no existence check, drops the previous `cancel`, mints a new `acctSessID` and `startTime`, and unconditionally calls `sendAcctStart`. A renegotiating subscriber produces a second RFC 2866 Accounting-Start under a new Acct-Session-Id, restarts Acct-Session-Time (Section 5.7), and orphans the previous `interimLoop` |
| D-7 | `L2TPReactor.startSessionTimeouts` (`l2tp/session_timeout.go`) | assigns `sess.sessionTimeoutCancel` and `sess.idleTimeoutCancel` over the previous value without cancelling it, then starts a second `runSessionTimeout` and `runIdleTimeout` goroutine. RFC 2865 Section 5.27 Session-Timeout restarts, so a subscriber extends service past the RADIUS limit by renegotiating. The file's own header says *"teardown cancels them"*, which now reaches only the last-written cancel |
| D-8 | `applyIKERekeyResponse` and `respondIKERekey` (`ike/engine/rekey.go`) | the two rekey paths disagree, and RFC 7296 says **both are wrong**. The initiator path sets `CreatedAt` and `EstablishedAt` both to the current time; the responder path sets both from `oldSA.EstablishedAt`. Section 2.18 calls the result "the new IKE SA" with new SPIs and new keys, so `CreatedAt` should advance. Section 2.8.3 says rekeying *"does not authenticate the parties again"*, and reserves a from-scratch IKE_SA_INIT/IKE_AUTH for reauthentication, so `EstablishedAt` (the authenticated relationship, which is what `uptime-seconds` measures in `saToMap`) should carry forward. Today `show ipsec` uptime zeroes on a locally-initiated rekey, and `created-at` misreports the new SA's age on a peer-initiated one |
| D-9 | `newExporter` (`plugins/flowexport/exporter.go`) | stamps `startTime` from the current time, and the `configure` closure in `register.go` is explicitly shared by boot and reload. RFC 3954 Section 5.1 defines sysUpTime as time since the device booted. `FIRST_SWITCHED` (22) and `LAST_SWITCHED` (21) are emitted as sysUpTime-relative milliseconds from the same value, so a reload corrupts every flow record's timestamps, not only the header |
| D-10 | `StoreSessionMetadata` (`l2tp/session_metadata.go`) | one package-level `sessionMeta` map keyed on a pair of uint16s, shared by both transports. L2TP stores under (tunnel id, session id); PPPoE stores under (ifindex, PPPoE session id). The namespaces collide, so whichever session authenticates second overwrites the other's Framed-Pool, Session-Timeout, Idle-Timeout, Filter-Id rate and CoS profile |

## Why the gates are green

Six RFCs here are enrolled and gated by `make ze-rfc-check`: 1661, 2865, 2866,
3954, 5176, 7296. The gate is green because the obligations these defects break
were never extracted.

| Obligation | State | Consequence |
|---|---|---|
| RFC1661-4.3-1, "must be prepared to immediately renegotiate Configuration Options on RCR+/RCR- in Opened" | extracted, gated, proven | its tagged proof `TestRFC1661RenegotiateOnConfigureRequestInOpened` asserts only that the pure function `LCPDoTransition` returns the right state and action tuple. It never constructs a session, so D-5, D-6 and D-7 sit underneath it |
| RFC 2866 Section 5.7, Acct-Session-Time semantics | not extracted | D-6 breaks it and no row exists to go red |
| RFC 2865 Section 5.27, Session-Timeout semantics | not extracted | D-7 breaks it and no row exists to go red |
| RFC 5176 Section 3.1, ACK means applied | not extracted | D-3 breaks it and no row exists to go red |
| RFC 3954 Section 5.1, sysUpTime is time since boot | not extracted (indicative prose, no RFC 2119 keyword) | D-9 breaks it and no row exists to go red |

Per `ai/rules/rfc-compliance.md`, an unextracted obligation is still an
obligation, and adding the row plus its proof is doing MORE, so it proceeds
without asking.

`docs/features/rfc-status.md` currently claims RFC 2866 and RFC 5176 are
"Supported for subscriber access" while PPPoE, a subscriber access type, has no
accounting at all and takes a CoA-ACK with no effect. Landing this spec makes
those two rows true rather than changing them.

## Improvements in scope (operator-selected 2026-08-02)

| # | Improvement |
|---|-------------|
| I-1 | A build-time guard that fails when an event topic is emitted and never subscribed |
| I-2 | A vacuity-trap row for concurrency tests that never contend, and a fresh-versus-restore trigger row in the Sibling Call-Site Audit |
| I-3 | Sub-millisecond `poll-sleep` granularity, a pinned VPP evidence image, and a `checkVPPVersion` that checks what its comment claims |
| I-4 | PPPoE parity `plan/learned/760` deferred: Disconnect-Request teardown, and RADIUS Framed-Route |

## Required Reading

### Architecture Docs
- [ ] `plan/learned/760-subscriber-session-model.md` - the spec that created the subscriber namespace and the bridge
  → Decision: the bridge re-emits one-directionally, `l2tpevents` to `subevents`, for L2TP only. A consumer migrated to `subevents` therefore keeps working for L2TP and gains PPPoE, so consumers migrate one at a time with no flag day.
  → Constraint: handler registries delegate (`l2tp.Register*` forwards to `subscriber.Register*`); event subscriptions do not. Never assume a working handler implies a working subscription.
- [ ] `plan/learned/885-cos-dynamic.md` - the CoS session handler this spec migrates
  → Constraint: `cos/register.go` passes a nil `resolveStatic`, so `onSessionDown` pushes nil maps and wipes the VLAN QoS map rather than restoring the configured profile. Fix while migrating, do not preserve.
- [ ] `ai/rules/data-flow-tracing.md` - required before a spec whose data crosses a boundary
  → Constraint: trace the producer of every value, not the consumer; the boundary table must name how each crossing is verified.
- [ ] `ai/rules/rfc-compliance.md` - six enrolled RFCs are in scope
  → Decision: full compliance plus a tagged test is the answer and needs no permission; only doing LESS requires asking.
  → Constraint: `check_coverage_ratchet` and `check_evidence_ratchet` mean a requirement may not lose a polarity or an evidence kind. Adding a functional-tier proof beside an existing unit proof is allowed; replacing one is not.
- [ ] `ai/rules/goroutine-lifecycle.md` - D-7 leaks two goroutines per renegotiation
  → Constraint: a goroutine per session lifecycle is permitted; leaking the previous one when the lifecycle event repeats is not.
- [ ] `ai/rules/interop-and-goal-validation.md` - the vacuity-trap table I-2 extends
  → Constraint: a test is evidence only if it fails when the behaviour is reverted. The existing RFC1661-4.3-1 proof is the worked example of a proof narrower than its requirement.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc1661.md` - PPP LCP, Section 4.3 renegotiation
  → Constraint: RFC1661-4.3-1 is gated. Its Remaining cell in `docs/features/rfc-status.md` spells "Twenty-five MUST gaps", which `check_gap_count_agreement` ties to the `{gap}` count in the summary. Changing that count means changing the spelled word.
- [ ] `rfc/short/rfc2865.md` - RADIUS, Section 5.27 Session-Timeout
  → Constraint: Session-Timeout is the maximum seconds of service before termination, measured from service start, so a restart on renegotiation extends service past the operator's limit.
- [ ] `rfc/short/rfc2866.md` - RADIUS accounting, Sections 5.5 and 5.7
  → Constraint: RFC2866-5.5-1 (Acct-Session-Id unique across the NAS) is already gated and stays satisfied by D-6, because each duplicate id is itself unique. Uniqueness is not identity: one session owes one id.
- [ ] `rfc/short/rfc3954.md` - NetFlow v9, Section 5.1
  → Constraint: sysUpTime is time since the device booted, and FIRST_SWITCHED / LAST_SWITCHED are expressed relative to it.
- [ ] `rfc/short/rfc5176.md` - RADIUS CoA, Sections 3.1 and 3.3
  → Constraint: a CoA-ACK asserts the change was applied. When it cannot be applied the answer is CoA-NAK, not ACK.
- [ ] `rfc/short/rfc7296.md` - IKEv2, Section 2.18
  → Constraint: RFC7296-2.18-2 (new IKE SA resets message counters to 0) is gated and both rekey paths satisfy it. Section 2.18 says nothing about uptime continuity, so D-8 is an internal consistency defect and must not be presented as a conformance fix.

**Key insights:** (minimal context to resume after compaction)
- The bridge's one-directional re-emit is what makes an incremental consumer migration safe.
- The Echo re-entry fix and this spec's root cause 2 are the same bug; the guard that fixed Echo is the guard renegotiation walks through.
- The metadata keyspace collision (D-10) blocks the cheap migration, which is why it was pulled into scope.
- `Emit` returns a count of PLUGIN-PROCESS subscribers only and explicitly does not count in-process ones, so it cannot serve as the I-1 guard.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/component/l2tp/subscriber_bridge.go` - subscribes to three `l2tpevents` topics and one `subevents` topic, re-emits three `subevents` topics for L2TP only
- [ ] `internal/component/l2tp/pppoe/subsystem.go` - `eventConsumer` builds a `subscriber.Session` setting `PPPoESID` but never `TunnelID`/`SessionID`, and emits `subevents` only
- [ ] `internal/component/l2tp/subscriber/events/events.go` - registers five topics; there is no `SessionCoSChange`
- [ ] `internal/component/l2tp/subscriber/registry.go` - `Add` overwrites the map entry unconditionally
- [ ] `internal/component/l2tp/session_metadata.go` - one package-level map keyed on a uint16 pair, shared by both transports
- [ ] `internal/component/l2tp/session_timeout.go` - `startSessionTimeouts` overwrites both cancel funcs and starts two goroutines
- [ ] `internal/component/l2tp/reactor_kernel.go` - `handleSessionUp` emits `l2tpevents.SessionUp` then calls `startSessionTimeouts`
- [ ] `internal/component/l2tp/plugins/authradius/acct.go` - `SubscribeEventBus` binds `l2tpevents`; `onSessionIPAssigned` rebuilds the accounting session with no existence check
- [ ] `internal/component/l2tp/plugins/authradius/coa.go` - `handleCoA` emits an unsubscribed subscriber topic and ACKs; `handleDisconnect` NAKs PPPoE
- [ ] `internal/component/l2tp/plugins/authradius/handler.go` - stores session metadata under the request's uint16 pair for both transports
- [ ] `internal/component/l2tp/plugins/pool/register.go` - `handle` is transport-generic, `onSessionDown` is L2TP-only
- [ ] `internal/component/l2tp/plugins/shaper/shaper.go` - `handleSubscriberSessionUp` applies TC without recording session state
- [ ] `internal/component/l2tp/route_observer.go` - `emitFramedRoutes` is reachable only through the L2TP reactor
- [ ] `internal/component/l2tp/ppp/ppp_fsm.go` - the `Opened` case returns `AckSent` on a peer Configure-Request
- [ ] `internal/component/l2tp/ppp/ncp.go` - `onNCPOpened` runs when the new state is Opened and the previous was not
- [ ] `internal/component/l2tp/ppp/session_run.go` - `afterLCPOpen` carries the same guard and drives auth, NCP and the session-up event
- [ ] `internal/plugins/cos/handler.go` - subscribes to `l2tpevents`; `onSessionDown` pushes nil maps
- [ ] `internal/plugins/cos/register.go` - passes a nil `resolveStatic`
- [ ] `internal/component/ike/engine/rekey.go` - the two rekey paths disagree on the replacement SA's timestamps
- [ ] `internal/plugins/flowexport/exporter.go` - `newExporter` stamps `startTime`
- [ ] `internal/plugins/flowexport/register.go` - `configure` is shared by boot and reload
- [ ] `internal/plugins/flowexport/netflow9/flow_data.go` - emits FIRST_SWITCHED and LAST_SWITCHED as sysUpTime-relative values
- [ ] `internal/component/vpp/config.go` - `parsePollSleepMs` accepts whole milliseconds only
- [ ] `internal/component/doctor/checks_linux.go` - `checkVPPVersion` checks for a literal substring, not a version range
- [ ] `pkg/ze/eventbus.go` - the `Emit` contract: the returned count excludes in-process subscribers
- [ ] `scripts/dev/verify_wiring_docs.py` - `check_wiring` is the existing symbol-versus-reference checker I-1 extends
- [ ] `internal/component/l2tp/ppp/echo_test.go` - the negative control that documents the re-entry side effects
- [ ] `internal/component/l2tp/plugins/authradius/acct_test.go` - `startAcctServer` and `acctCapture` provide a real fake accounting server

**Behavior to preserve:**
- L2TP lifecycle behaviour end to end. Every existing L2TP `.ci`, interop and unit test must pass unchanged; the migration must be invisible to L2TP.
- `LCPDoTransition`'s transition table. The FSM is correct and RFC1661-4.3-1's existing unit proof must keep passing, not be replaced.
- `show l2tp shaper`, `show subscriber summary` and `show ipsec` output shapes.
- The transport-generic handler registries (`RegisterPoolHandler`, `RegisterAuthHandler`, `RegisterShaperHandler`) and their delegation from `l2tp.Register*`.
- Dataplane reprogramming on renegotiation: `onNCPOpened` must keep calling `AddAddressP2P` every time, so a changed IPCP address still lands.
- `poll-sleep` configurations already written in whole milliseconds must keep parsing and producing the same startup.conf.

**Behavior to change:**
- `SessionUp` fires once per session; renegotiation emits a new `SessionParamsChanged` topic instead.
- Six lifecycle consumers move from `l2tpevents` to `subevents`.
- Session metadata is keyed by the subscriber session ID string.
- The IKE initiator rekey path carries the old SA's establishment time forward.
- The flow exporter's sysUpTime epoch survives a config reload.
- CoS teardown restores the configured static profile instead of wiping the map.
- A CoA that cannot be applied answers NAK.

### Consumer migration map

What each consumer needs before it can leave `l2tpevents`. Every row was read
during research; none is inferred from a caller. All six use the uint16 pair as a
map key today, which is why the keyspace phase precedes the migration phase.

| Consumer | Topics it holds | Blocker to migrating | Resolved by |
|----------|-----------------|----------------------|-------------|
| `poolPlugin.setEventBus` | Down | keys `sessionAddrs` on the uint16 pair | keyspace phase; the release key becomes the session ID |
| `radiusAcct.SubscribeEventBus` | IPAssigned, Down | needs the assigned peer address, which the bridge's subscriber payload never carries: `subscriberBridge.onSessionIPAssigned` sets only `Username` and `PppInterface` and leaves `IPv4Addr` zero, while `pppoe/subsystem.go` does set it | carry the address through the bridge; then keyspace phase for metadata and the accounting id |
| `shaper.setEventBus` | Up, Down, RateChange | keys `s.sessions` on the pair, reads metadata for the Filter-Id rate, and `showSessions` emits the pair in its JSON | keyspace phase; `showSessions` reports the session ID, which is the identifier every other `show` already uses |
| `newCosHandler` | Up, Down, CoSChange | `subevents` has no `SessionCoSChange` topic at all | wiring phase adds the topic |
| `Subsystem.wireObserverSubscriptions` | Up, Down, IPAssigned, EchoRTT | `ReleaseSession` is typed on a uint16 session id, and `EchoRTT` has no subscriber-namespace equivalent | keyspace phase re-types the release; EchoRTT stays on `l2tpevents` because it is transport-level, not lifecycle |
| `subscriberBridge.subscribe` | Up, Down, IPAssigned | none; it is the producer | unchanged |

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
Three independent entry points converge on the same downstream consumers.
1. A peer PPP Configure-Request arriving on an established session, read by `handleFrame` in `ppp/session_run.go`. This is the renegotiation trigger.
2. A PPPoE session reaching Opened, surfaced by the PPP driver as `ppp.EventSessionUp` and consumed by `Subsystem.eventConsumer` in `pppoe/subsystem.go`.
3. An L2TP session reaching Opened, surfaced to `L2TPReactor.handleSessionUp` in `reactor_kernel.go`.

### Transformation Path
1. The PPP FSM transitions and `afterLCPOpen` runs its post-Opened side effects.
2. `afterLCPOpen` drives the auth phase and the NCP phase, then emits the driver-level session event.
3. The transport subsystem consumes the driver event: PPPoE builds a `subscriber.Session` and emits `subevents`; L2TP emits `l2tpevents`, which `subscriberBridge` re-emits as `subevents`.
4. Lifecycle consumers act: pool release, accounting, timeouts, shaping, CoS, framed routes.
5. Consumers that need RADIUS profile data call `LoadSessionMetadata` with the session key.
6. Accounting and CoA reach the wire as RADIUS packets; shaping and CoS reach the dataplane through the iface backend.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| PPP FSM ↔ session side effects | the `cur != Opened` guard in `afterLCPOpen` and `onNCPOpened` | No |
| Transport subsystem ↔ lifecycle consumers | EventBus topics in two namespaces | No |
| Consumer ↔ RADIUS profile data | `LoadSessionMetadata` on a shared keyspace | No |
| Ze ↔ RADIUS server | Accounting-Request and CoA-ACK/NAK on the wire | No |
| Ze ↔ NetFlow collector | sysUpTime-relative flow timestamps | No |
| Ze ↔ IKE peer | the replacement SA after a rekey | No |

### Integration Points
- `subscriber.DefaultRegistry` - the single registry both transports write; the migration must not change its API.
- `subscriber/handler_registry.go` - where a transport-generic disconnect handler joins the existing pool, auth and shaper handlers for I-4.
- `scripts/dev/verify_wiring_docs.py` `check_wiring` - the existing checker I-1 extends with an emit-versus-subscribe predicate, reached through the `wiring` target of `make ze-validate`.
- `internal/core/events` `AllEventTypes` - the declared-topic inventory the I-1 guard reads.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugin-self-containment.md`) | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The bridge's one-directional re-emit lets consumers migrate one at a time without an L2TP regression | `subscriber_bridge.go` `subscribe` subscribes to `l2tpevents` and emits `subevents` | a flag-day migration is required and the phase order collapses | migrate the pool consumer first and run the full L2TP suite before touching the others | unvalidated |
| A-2 | `subscriber.Session.ID` is unique across transports and stable for a session's life | `l2tpSessionID` builds `l2tp-<t>-<s>`; `pppoeSessionID` builds a distinct form | the metadata re-key reintroduces a collision under a new name | a unit test asserting the two ID builders cannot produce equal strings for any input pair | unvalidated |
| A-3 | Emitting `SessionUp` once and `SessionParamsChanged` on renegotiation leaves no consumer needing the old re-fire | the six consumers enumerated in Current Behavior each act once per session | a consumer silently stops reacting to a legitimate parameter change | per-consumer review during migration, recorded in the audit table | unvalidated |
| A-4 | RADIUS metadata is stored before the first consumer reads it, on both transports | `authradius/handler.go` stores during the auth phase, which `afterLCPOpen` completes before emitting session-up | a migrated consumer reads nil metadata and silently applies defaults | assert non-nil metadata in the PPPoE accounting functional test | unvalidated |
| A-5 | No operator config expresses `poll-sleep` in a unit other than whole milliseconds | the YANG pattern admits digits followed by `ms` only | widening the pattern breaks an existing config | parse every `poll-sleep` value in `test/` and the appliance seed before and after | unvalidated |
| A-6 | A rekeyed IKE SA is a new SA carrying an unchanged authentication, so `CreatedAt` advances and `EstablishedAt` carries forward | RFC 7296 Section 2.18 ("the new IKE SA", new SPIs, new keys) and Section 2.8.3 ("does not authenticate the parties again"; reauthentication is a from-scratch IKE_SA_INIT/IKE_AUTH) | `show ipsec` uptime means the wrong thing and operators misread tunnel stability | RFC text read 2026-08-02; AC-9 asserts both fields in both rekey directions | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Re-keying session metadata touches six call sites; one missed site reads nil and applies defaults silently | a functional test shows the default rate or no Session-Timeout where RADIUS supplied one | make the metadata accessor take a `subscriber.Session` rather than loose fields, so a missed site fails to compile |
| R-2 | Splitting the topic changes when `SessionUp` fires, which observability and `show` output depend on | `show subscriber summary` or the observer reports a session late or not at all | migrate the observer in the same phase as the split, with its existing tests as the gate |
| R-3 | The I-1 guard fires on legitimately unsubscribed topics (a topic emitted for external plugin processes only) | `make ze-validate` goes red on unrelated work | reuse the existing `WIRING_ALLOWLIST` mechanism for reviewed exceptions rather than inventing a second one |
| R-4 | Adding RFC checklist rows to five enrolled summaries makes `ze-rfc-check` red until every row has positive and negative tagged tests | the gate reports uncovered requirements mid-implementation | add each row in the same phase as its proof, never in a batch ahead of the tests |
| R-5 | Nothing can drive a mid-session Configure-Request in a functional test today, so the end-to-end proof needs new harness capability | the functional phase has no way to reach the daemon | extend the scripted Python L2TP peer in `test/l2tp/` to send an LCP Configure-Request in a data message after session-up |
| R-6 | The renegotiation path re-runs the auth phase, so a fix scoped to accounting alone leaves a duplicate RADIUS Access-Request | the fake RADIUS server sees two Access-Requests for one session | assert Access-Request count as well as Accounting-Start count in the same test |
| R-7 | Changing the RFC 1661 `{gap}` count desynchronises the spelled number in the public ledger | `check_gap_count_agreement` fails at commit | change the ledger row in the same commit as the summary |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Subscriber sessions on both transports: addresses leak from the pool, accounting records double or vanish, shaping and CoS stop applying, and RADIUS-imposed session limits stop being enforced. IKE and NetFlow changes are observability-only |
| How is it reverted? | Phase by phase. The consumer migration is revertible per consumer because the bridge keeps feeding both namespaces. The topic split is one commit. The metadata re-key is not independently revertible once consumers depend on the new key |
| Who else touches this path? | `plan/spec-finish-l2tp.md` (its L42 renegotiation-test gap is closed by this spec; its L41 row is already stale and should be corrected), `plan/spec-radius-acct-timewheel.md` (same `interimLoop`, so land this first), `plan/learned/885-cos-dynamic.md` |

## Wiring Test (MANDATORY -- NOT deferrable)

Each user-facing row names the `.ci` that drives the real entry point. The two
internal rows are tooling with no user entry point and name their Go and Python
tests instead.

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| PPPoE subscriber disconnects | → | `poolPlugin.onSessionDown` via `subevents.SessionDown` | `test/pppoe/pool-release-cycle.ci` |
| PPPoE subscriber authenticates and comes up | → | `radiusAcct.onSessionUp` via `subevents` | `test/pppoe/accounting-lifecycle.ci` |
| RADIUS sends a CoA rate-change for a PPPoE session | → | `shaper.onSessionRateChange` via `subevents.SessionRateChange` | `test/pppoe/coa-rate-change.ci` |
| RADIUS sends a Disconnect-Request for a PPPoE session | → | the transport-generic disconnect handler | `test/pppoe/coa-disconnect.ci` |
| Peer sends a Configure-Request on an established session | → | `afterLCPOpen` params-changed branch | `test/l2tp/renegotiation-preserves-session.ci` |
| Operator reloads config with flow export enabled | → | `configure` closure preserving the exporter epoch | `test/plugin/flowexport-reload-epoch.ci` |
| Locally-initiated IKE SA rekey | → | `applyIKERekeyResponse` | `test/ipsec-interop/` rekey-uptime scenario |
| `ze doctor` on a VPP host | → | `checkVPPVersion` range check | `TestCheckVPPVersionRejectsUnsupportedRange` (tooling, no user entry point) |
| An event topic emitted and never subscribed | → | `check_wiring` emit-versus-subscribe predicate | `test_emit_without_subscribe_is_reported` (tooling, no user entry point) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A PPPoE session that allocated a pool address tears down | the address returns to the pool and is reusable by the next session |
| AC-2 | A PPPoE session authenticates and reaches Opened | exactly one RFC 2866 Accounting-Start reaches the RADIUS server, followed by interim updates and one Accounting-Stop on teardown |
| AC-3 | A CoA rate-change names a PPPoE session | the new rate is applied to the dataplane and the response is CoA-ACK |
| AC-4 | A CoA names a session whose change cannot be applied | the response is CoA-NAK, never CoA-ACK |
| AC-5 | A PPPoE session is established | it appears in `show l2tp shaper`, carries its RADIUS Filter-Id rate rather than the default, and restores the original qdisc on teardown |
| AC-6 | A peer sends a Configure-Request on an established session | the session's activation time and username are unchanged afterwards |
| AC-7 | A peer renegotiates after accounting has started | no second Accounting-Start is sent, the Acct-Session-Id is unchanged, and Acct-Session-Time keeps counting from first activation |
| AC-8 | A peer renegotiates on a session carrying a RADIUS Session-Timeout | the timeout still expires at the original deadline, and no additional timeout goroutine is running |
| AC-9 | An IKE SA is rekeyed, in either direction | `show ipsec` reports `uptime-seconds` continuous across the rekey, and `created-at` advanced to the rekey time. Both directions agree |
| AC-19 | The scripted L2TP peer is asked to renegotiate an established session | it sends an LCP Configure-Request in a data message and completes the exchange, giving AC-6, AC-7, AC-8 and AC-17 a functional entry point |
| AC-10 | The daemon reloads its configuration with flow export enabled | the exported sysUpTime continues to increase and FIRST_SWITCHED / LAST_SWITCHED stay on the same epoch |
| AC-11 | An L2TP session and a PPPoE session hold numerically equal identifier pairs | each reads back its own RADIUS profile attributes |
| AC-12 | A build declares an event topic that is emitted and never subscribed | `make ze-validate` reports it and fails, unless the topic is in the reviewed allowlist |
| AC-13 | An operator sets a sub-millisecond `poll-sleep` value | the value is accepted and appears in the generated startup.conf in microseconds |
| AC-14 | `ze doctor` runs against a VPP whose version is outside the supported range | a warning names the found version and the supported range |
| AC-15 | A RADIUS Disconnect-Request names a PPPoE session | the session is torn down and the response is Disconnect-ACK |
| AC-16 | RADIUS supplies a Framed-Route for a PPPoE subscriber | the route is installed and withdrawn with the session |
| AC-17 | A renegotiation occurs on a session whose auth already completed | no second RADIUS Access-Request is sent |
| AC-18 | A PPPoE session's CoS profile is torn down | the configured static profile is restored, not an empty map |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Connects a PPPoE subscriber, then disconnects it, repeatedly | PADI/PADR → PPP auth → pool allocate → session up → teardown → pool release | `test/pppoe/pool-release-cycle.ci` |
| 2 | Bills a PPPoE subscriber for a session | session up → Accounting-Start → interim → teardown → Accounting-Stop | `test/pppoe/accounting-lifecycle.ci` |
| 3 | Changes a live subscriber's rate from the RADIUS server | CoA-Request → session lookup → shaper apply → CoA-ACK | `test/pppoe/coa-rate-change.ci` |
| 4 | Disconnects a subscriber from the RADIUS server | Disconnect-Request → PADT → session teardown → Disconnect-ACK | `test/pppoe/coa-disconnect.ci` |
| 5 | Runs a subscriber whose CPE renegotiates LCP mid-session | Configure-Request → FSM → params-changed → consumers unaffected | `test/l2tp/renegotiation-preserves-session.ci` |
| 6 | Reloads configuration while a NetFlow collector is attached | reload → configure → exporter rebuilt with preserved epoch | `test/plugin/flowexport-reload-epoch.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRenegotiationEmitsParamsChangedNotSessionUp` | `internal/component/l2tp/ppp/renegotiate_test.go` | a second entry into Opened emits the params-changed topic, not session-up | |
| `TestRenegotiationDoesNotRerunAuth` | `internal/component/l2tp/ppp/renegotiate_test.go` | AC-17: the auth phase runs once per session | |
| `TestRenegotiationStillProgramsDataplane` | `internal/component/l2tp/ppp/renegotiate_test.go` | `AddAddressP2P` runs on every NCP-Opened entry, so a changed address lands | |
| `TestAcctNoDuplicateStartOnRepeatedIPAssigned` | `internal/component/l2tp/plugins/authradius/acct_test.go` | AC-7 using the existing fake accounting server | |
| `TestAcctSessionTimeSurvivesRenegotiation` | `internal/component/l2tp/plugins/authradius/acct_test.go` | AC-7: Acct-Session-Time counts from first activation | |
| `TestStartSessionTimeoutsIsIdempotent` | `internal/component/l2tp/session_timeout_test.go` | AC-8: no second goroutine, original deadline preserved | |
| `TestSessionMetadataNoCrossTransportCollision` | `internal/component/l2tp/session_metadata_test.go` | AC-11 and A-2 | |
| `TestSubscriberIDBuildersNeverCollide` | `internal/component/l2tp/subscriber/registry_test.go` | A-2: the two ID builders cannot produce equal strings | |
| `TestRegistryAddPreservesActivatedAt` | `internal/component/l2tp/subscriber/registry_test.go` | AC-6 | |
| `TestPoolReleaseOnSubscriberSessionDown` | `internal/component/l2tp/plugins/pool/register_test.go` | AC-1 at unit level | |
| `TestShaperRecordsPPPoESession` | `internal/component/l2tp/plugins/shaper/shaper_test.go` | AC-5: `s.sessions` is populated on the PPPoE path | |
| `TestCoSSessionDownRestoresStaticProfile` | `internal/plugins/cos/handler_test.go` | AC-18 | |
| `TestCoAUnappliedAnswersNAK` | `internal/component/l2tp/plugins/authradius/coa_test.go` | AC-4 | |
| `TestIKERekeyCarriesEstablishedAtAdvancesCreatedAt` | `internal/component/ike/engine/rekey_test.go` | AC-9 on the initiator path | |
| `TestIKERekeyBothDirectionsAgreeOnTimestamps` | `internal/component/ike/engine/rekey_test.go` | AC-9: `applyIKERekeyResponse` and `respondIKERekey` produce identical field semantics | |
| `TestFlowExportReloadPreservesSysUpTime` | `internal/plugins/flowexport/exporter_test.go` | AC-10 including FIRST_SWITCHED continuity | |
| `TestParsePollSleepMicroseconds` | `internal/component/vpp/config_test.go` | AC-13 | |
| `TestCheckVPPVersionRejectsUnsupportedRange` | `internal/component/doctor/checks_linux_test.go` | AC-14 | |
| `test_emit_without_subscribe_is_reported` | `scripts/dev/verify_wiring_docs_test.py` | AC-12 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `poll-sleep` microseconds | 0us-100000us | 100000us | N/A | 100001us |
| `poll-sleep` sub-millisecond | 1us-999us | 999us | 0us is valid, not invalid | N/A |
| VPP supported major version | the range `checkVPPVersion` accepts | highest supported | one below lowest | one above highest |
| Acct-Session-Time after N renegotiations | 0-2^32-1 seconds | unchanged by N | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `pool-release-cycle` | `test/pppoe/pool-release-cycle.ci` | repeated PPPoE connect/disconnect does not exhaust the pool | |
| `accounting-lifecycle` | `test/pppoe/accounting-lifecycle.ci` | a PPPoE subscriber produces Start, interim and Stop | |
| `coa-rate-change` | `test/pppoe/coa-rate-change.ci` | a CoA rate change reaches the PPPoE dataplane | |
| `coa-disconnect` | `test/pppoe/coa-disconnect.ci` | a Disconnect-Request tears down a PPPoE session | |
| `renegotiation-preserves-session` | `test/l2tp/renegotiation-preserves-session.ci` | a mid-session Configure-Request leaves accounting and timeouts intact | |
| `flowexport-reload-epoch` | `test/plugin/flowexport-reload-epoch.ci` | sysUpTime does not regress across a reload | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `pppoe-accounting-freeradius` | `test/pppoe-interop/` | accel-ppp + FreeRADIUS | a real PPPoE peer produces exactly one accounting session | |
| `l2tp-renegotiation-xl2tpd` | `test/l2tp-interop/` | xl2tpd + pppd | a real peer's renegotiation does not restart accounting | |
| `ipsec-rekey-uptime-strongswan` | `test/ipsec-interop/` | strongSwan | uptime continuity across a locally-initiated rekey | |

## Files to Modify
- `internal/component/l2tp/ppp/session_run.go` - split the post-Opened side effects into first-activation and params-changed paths
- `internal/component/l2tp/ppp/ncp.go` - keep dataplane programming on every NCP-Opened entry
- `internal/component/l2tp/subscriber/events/events.go` - add the params-changed and CoS-change topics
- `internal/component/l2tp/subscriber_bridge.go` - carry the assigned address into the subscriber payload; stop rebuilding the session
- `internal/component/l2tp/pppoe/subsystem.go` - stop rebuilding the session on a repeat event
- `internal/component/l2tp/session_metadata.go` - re-key by subscriber session ID
- `internal/component/l2tp/session_timeout.go` - cancel before restarting
- `internal/component/l2tp/reactor_kernel.go` - drive timeouts from first activation only
- `internal/component/l2tp/route_observer.go` - reach framed routes from both transports
- `internal/component/l2tp/plugins/authradius/acct.go` - migrate to the subscriber namespace, dedupe by session
- `internal/component/l2tp/plugins/authradius/coa.go` - NAK what cannot be applied; route disconnect through a transport-generic handler
- `internal/component/l2tp/plugins/authradius/handler.go` - store metadata under the subscriber session ID
- `internal/component/l2tp/plugins/pool/register.go` - migrate release to the subscriber namespace
- `internal/component/l2tp/plugins/shaper/shaper.go` - record PPPoE sessions and read their metadata
- `internal/component/l2tp/subscriber/handler_registry.go` - add a transport-generic disconnect handler
- `internal/plugins/cos/handler.go`, `internal/plugins/cos/register.go` - migrate namespace; restore the static profile on teardown
- `internal/component/ike/engine/rekey.go` - carry the establishment time forward on the initiator path
- `internal/plugins/flowexport/register.go` - preserve the exporter epoch across a reload
- `internal/component/vpp/config.go`, `internal/component/vpp/yang/ze-vpp-conf.yang` - accept sub-millisecond poll-sleep
- `internal/component/doctor/checks_linux.go` - implement the version-range check
- `scripts/evidence/effective-vpp.py`, `scripts/evidence/effective-vpp-iface.py` - pin the VPP image
- `scripts/dev/verify_wiring_docs.py` - add the emit-versus-subscribe predicate
- `ai/rules/interop-and-goal-validation.md` - add the contention vacuity-trap row
- `ai/rules/before-writing-code.md` - add the fresh-versus-restore trigger row
- `rfc/short/rfc2865.md`, `rfc/short/rfc2866.md`, `rfc/short/rfc3954.md`, `rfc/short/rfc5176.md` - extract the missing obligations
- `docs/features/rfc-status.md` - keep the ledger in step with the summaries
- `plan/spec-finish-l2tp.md` - correct the stale L41 row and close L42

## Files to Create
- `internal/component/l2tp/ppp/renegotiate_test.go` - the behavioural renegotiation tests
- `internal/component/l2tp/session_metadata_test.go` - keyspace collision coverage
- `test/pppoe/pool-release-cycle.ci`, `test/pppoe/accounting-lifecycle.ci`, `test/pppoe/coa-rate-change.ci`, `test/pppoe/coa-disconnect.ci` - PPPoE lifecycle functional tests
- `test/l2tp/renegotiation-preserves-session.ci` - the renegotiation functional test, using an extended scripted peer
- `test/plugin/flowexport-reload-epoch.ci` - reload epoch continuity
- `plan/deferrals/lifecycle-invariants.md` - the deferral shard named in the metadata table

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/component/vpp/yang/ze-vpp-conf.yang` for the widened poll-sleep leaf |
| YANG validation constraints | Yes | the poll-sleep pattern must accept a microsecond unit while still rejecting free text |
| YANG custom validators | N-A | the pattern plus range is sufficient; no cross-leaf constraint |
| CLI commands/flags | No | no new command; `show l2tp shaper` and `show ipsec` gain correct data through existing paths |
| CLI grammar (keyword before value) | N-A | no new command |
| Editor autocomplete | N-A | the poll-sleep leaf is a pattern-typed string with no enumerated values |
| Functional test for new RPC/API | Yes | the six `.ci` files listed in Files to Create |
| Pipe completeness | N-A | no new output-producing command |
| Env var registration | No | no new environment leaf |
| Doctor check for runtime dependencies | Yes | `internal/component/doctor/checks_linux.go` `checkVPPVersion` gains its range check; the diagnostic code `doctor-vpp-version` already exists in `internal/core/diagnostic/codes.go` |
| Prometheus counters/metrics | Yes | a counter for lifecycle events delivered to zero in-process subscribers, so I-1 has a runtime companion to its build-time guard |
| BGP family surface (new SAFI / capability / attribute) | N-A | no BGP surface is touched |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` - PPPoE accounting, CoA and shaping become real |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md` for the poll-sleep unit |
| 3 | CLI command added/changed? | No | no command surface changes |
| 4 | API/RPC added/changed? | No | no RPC changes |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` for the authradius, pool, shaper and cos namespace change |
| 6 | Has a user guide page? | Yes | `docs/guide/monitoring.md` for the NetFlow epoch note |
| 7 | Wire format changed? | No | no encoding changes; only which values are placed in existing fields |
| 8 | Plugin SDK/protocol changed? | No | the EventBus contract is unchanged |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc2865.md`, `rfc2866.md`, `rfc3954.md`, `rfc5176.md` and the `docs/features/rfc-status.md` rows |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` for the scripted-peer renegotiation capability |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` - PPPoE subscriber management gains accounting parity |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` for the session lifecycle event model |
| 13 | Route metadata keys added/changed? | No | framed routes reuse the existing metadata keys |
| 14 | Prometheus counters added/changed? | Yes | `docs/plugin-development/metrics.md` for the zero-subscriber counter |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | `docs/plugin-overview.md` and `docs/features/plugins.md` for the two new event types |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | grep `docs/` for anchors naming every file in Files to Modify and update each stale claim |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | verify every poll-sleep example and every subscriber-session example against the new behaviour |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** - register the new topics and the transport-generic disconnect handler, and write the failing wiring tests
   - Tests: every row of the Wiring Test table, all failing
   - Files: `subscriber/events/events.go`, `subscriber/handler_registry.go`
   - Verify: the topics exist and are discoverable, and each wiring test fails because the feature is a stub
2. **Phase: Renegotiation harness (BEFORE any dependent phase)** - teach the scripted L2TP peer to renegotiate
   - Tests: AC-19, proven by driving a Configure-Request at a daemon running today's unfixed code and observing the duplicate Accounting-Start
   - Files: the scripted peer in `test/l2tp/`, `docs/functional-tests.md`
   - Verify: the harness reproduces D-6 before any fix exists. If it cannot be built, STOP and report: the topic-split and idempotent-consumer phases then have no functional proof, and the spec's scope must be renegotiated with the operator rather than quietly dropped to unit level
3. **Phase: Keyspace** - re-key session metadata by subscriber session ID
   - Tests: `TestSessionMetadataNoCrossTransportCollision`, `TestSubscriberIDBuildersNeverCollide`
   - Files: `session_metadata.go`, `authradius/handler.go`, and every reader
   - Verify: make the accessor take a session rather than loose identifiers so a missed call site fails to compile (R-1)
4. **Phase: Topic split** - `SessionUp` once per session, params-changed on renegotiation, dataplane programming unchanged
   - Tests: the three `renegotiate_test.go` tests, plus `TestRegistryAddPreservesActivatedAt`
   - Files: `ppp/session_run.go`, `ppp/ncp.go`, `subscriber_bridge.go`, `pppoe/subsystem.go`
   - Verify: the existing Echo negative control and the RFC1661-4.3-1 table test both still pass
5. **Phase: Idempotent lifecycle consumers** - accounting dedupe and timeout cancellation
   - Tests: `TestAcctNoDuplicateStartOnRepeatedIPAssigned`, `TestAcctSessionTimeSurvivesRenegotiation`, `TestStartSessionTimeoutsIsIdempotent`
   - Files: `authradius/acct.go`, `session_timeout.go`, `reactor_kernel.go`
   - Verify: assert Access-Request count alongside Accounting-Start count (R-6)
6. **Phase: Consumer migration** - move pool, accounting, shaper, CoS and the observer to the subscriber namespace, one consumer at a time
   - Tests: `TestPoolReleaseOnSubscriberSessionDown`, `TestShaperRecordsPPPoESession`, `TestCoSSessionDownRestoresStaticProfile`
   - Files: `pool/register.go`, `authradius/acct.go`, `shaper/shaper.go`, `cos/handler.go`, `cos/register.go`, `observer.go`
   - Verify: run the full L2TP suite after each consumer, per A-1
7. **Phase: PPPoE parity** - disconnect teardown and framed routes
   - Tests: `TestCoAUnappliedAnswersNAK`, plus the disconnect and framed-route functional tests
   - Files: `authradius/coa.go`, `pppoe/subsystem.go`, `route_observer.go`, `subscriber/handler_registry.go`
   - Verify: AC-15 and AC-16 end to end
8. **Phase: Standalone defects** - IKE rekey continuity and flow-export epoch
   - Tests: `TestIKERekeyPreservesEstablishedAt`, `TestFlowExportReloadPreservesSysUpTime`
   - Files: `ike/engine/rekey.go`, `flowexport/register.go`
   - Verify: both rekey directions produce identical field semantics, per A-6 as settled from RFC 7296 Sections 2.18 and 2.8.3
9. **Phase: VPP operational items** - poll-sleep granularity, image pin, doctor range check
   - Tests: `TestParsePollSleepMicroseconds`, `TestCheckVPPVersionRejectsUnsupportedRange`
   - Files: `vpp/config.go`, `vpp/yang/ze-vpp-conf.yang`, `doctor/checks_linux.go`, both evidence scripts
   - Verify: existing whole-millisecond configs produce byte-identical startup.conf (A-5)
10. **Phase: Guards and rules** - the emit-versus-subscribe predicate and the two rule rows
   - Tests: `test_emit_without_subscribe_is_reported`
   - Files: `scripts/dev/verify_wiring_docs.py`, `ai/rules/interop-and-goal-validation.md`, `ai/rules/before-writing-code.md`
   - Verify: the predicate reports the pre-fix state of this very spec's topics when run against the parent commit
11. **Phase: RFC extraction and ledger** - add the missing obligations with their proofs, in the same phase as the tests that prove them
   - Tests: tagged positive and negative pairs for each new requirement id
   - Files: the four summaries, `docs/features/rfc-status.md`
   - Verify: `make ze-rfc-check` green, and the RFC 1661 spelled gap count still agrees (R-7)

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file and symbol |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | Renegotiation still applies a changed MRU and a changed IPCP address; only lifecycle side effects are suppressed |
| Naming | The new topics say what they mean: one names first activation, the other names a parameter change |
| Data flow | No consumer reads `TunnelID`/`SessionID` to identify a session after the re-key |
| Rule: `ai/rules/rfc-compliance.md` | Every added requirement row carries positive and negative tagged tests; no row is added ahead of its proof |
| Rule: `ai/rules/goroutine-lifecycle.md` | No timeout goroutine outlives the session that started it |
| Rule: `ai/rules/interop-and-goal-validation.md` | Each new test was proven to go red with the fix reverted, and the result recorded |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| No lifecycle consumer subscribes to `l2tpevents` | grep the six files named in Current Behavior for `l2tpevents` and expect no lifecycle topic |
| Every subscriber topic has a subscriber | `make ze-validate` |
| PPPoE accounting reaches the wire | the accounting functional test's captured packets |
| RFC gates hold | `make ze-rfc-check` |
| Pool does not leak | the pool-release-cycle test run for more cycles than the pool holds addresses |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | A peer-driven renegotiation must not let an unauthenticated party restart auth, extend a Session-Timeout, or create unbounded goroutines or nested stack frames |
| Resource exhaustion | A subscriber renegotiating in a loop must not grow memory, goroutines, or RADIUS traffic without bound |
| Authorization fail-open | A CoA that cannot be applied must answer NAK; answering ACK tells the RADIUS server a policy was enforced when it was not |

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

- The Echo re-entry fix and root cause 2 are one bug seen twice. The guard that closed the Echo case is the guard renegotiation walks through, because renegotiation genuinely leaves the Opened state and Echo does not. A guard keyed on state transition cannot express "once per session"; only a latch on the session can.
- A test can be a faithful proof of its requirement and still miss the defect the requirement exists to prevent. `TestRFC1661RenegotiateOnConfigureRequestInOpened` proves the transition table, which is what "be prepared to renegotiate" means at the FSM layer and nothing more.
- Delegating a handler registry and delegating an event subscription look alike from the call site and behave completely differently. That asymmetry is what let `plan/learned/760` read as complete.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Split the session-up topic so it fires once, and add a params-changed topic | make each consumer idempotent; suppress the side effects at the source with one latch | every existing consumer already assumes once-per-session, so the split makes six consumers correct without touching their logic, and a consumer that wants renegotiation opts in. Suppression alone would silently drop a legitimate MRU or address change |
| Re-key session metadata by the subscriber session ID string | add a transport discriminator to the existing pair; leave it and block on a separate spec | the string key is already namespaced per transport and is the identifier every migrated consumer holds. A discriminator would keep a key whose meaning depends on transport, leaving the same trap for the next consumer |
| Extend the existing `check_wiring` checker for the I-1 guard | a new AST-walking checker; a runtime subscriber count on the bus | `check_wiring` is already a symbol-versus-reference checker with an allowlist for reviewed exceptions and is already wired into `make ze-validate`. The `Emit` return value cannot serve, because its contract excludes in-process subscribers |
| On rekey, advance `CreatedAt` and carry `EstablishedAt` forward, in both directions | make both paths stamp the current time; make both carry the old time; add a third field for tunnel-first-established | RFC 7296 draws exactly this line and ze already has both fields. Section 2.18 makes the SA new (new SPIs, new keys), Section 2.8.3 makes the authentication unchanged and reserves a reset for reauthentication. Using the existing fields keeps `show ipsec` output shape unchanged; a third field would restate a distinction the two already carry |
| Build the renegotiation harness in its own phase before the phases that need it | leave it as a risk row and decide during implementation; accept unit-level proof and record a deferral | the four renegotiation ACs are the core of the spec and unit tests are what missed this defect the first time. Discovering the harness is unreachable after the code has landed is the expensive order; discovering it first stops the spec cheaply |

## Known Limitations
- Nothing in the current functional harness can drive a mid-session Configure-Request, which is why building that capability is its own early phase with AC-19. If it cannot be built the spec stops there and the scope returns to the operator; dropping to unit-level proof is not an outcome this spec may choose for itself.
- IPv6-only PPPoE subscribers are not separately covered; the accounting and pool tests use IPv4.
- RFC 7296 Section 2.8.3 says reauthentication resets what a rekey preserves, so a from-scratch IKE_SA_INIT/IKE_AUTH should start a fresh `EstablishedAt`. Whether ze implements reauthentication at all was not established here, and AC-9 asserts rekey behaviour only.
- The IKE and flow-export defects share no code with the subscriber work. They ride along because they are the same invariant, not because they are coupled.

## Review Gate

Filled at implementation time by `/ze-review`, recorded via `scripts/dev/review_gate.py`. Do not delete.

| Run | Date | BLOCKER | ISSUE | Notes |
|-----|------|---------|-------|-------|

## RFC Documentation (Scope: protocol)

Add an RFC section reference and the quoted requirement above enforcing code.
MUST document: validation rules, error conditions, state transitions, timer
constraints, message ordering, and every MUST/MUST NOT.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-19 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-verify` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
- [ ] `make ze-rfc-check` passes with the four summaries extended
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Deferral shard resolved: no live row without a destination

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features
- [ ] Each new test proven to go red with its fix reverted, result recorded

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `scripts/dev/review_gate.py`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/spec-lifecycle-invariants.md` only
