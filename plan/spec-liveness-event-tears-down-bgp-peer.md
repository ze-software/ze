# Spec: liveness-event-tears-down-bgp-peer

| Field | Value |
|-------|-------|
| Status | design |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-07 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

A router runs several protocols to the same neighbor address. When one of them
proves the address is dead, the others keep their sessions open until their own
timers expire, and the RIB keeps advertising routes learned over a peer that
cannot answer. LDP is the first detector: its keepalive expires after the
negotiated hold time and the session ends, while the BGP session to the same
address stays Established until the BGP hold timer runs out.

The goal is a bus event any liveness detector can publish and the BGP engine can
act on. A detector that finds an address dead publishes one `liveness`
`peer-down` event carrying the address, the detector name, the cause, and the
protocols the operator asked that detector to monitor. The BGP reactor
subscribes, maps the address to its peers, and tears each one down with a
NOTIFICATION, so the RIB withdraws the routes learned over it.

The same work closes half of a recorded RFC 5036 MUST gap. On keepalive expiry
LDP MUST transmit a Shutdown Notification before it closes the transport, and Ze
transmits nothing, because `wire.go` has no Notification or Status TLV encoder.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/ldp/mpls-ldp.md` - LDP plugin structure, config parse, session lifecycle
  → Constraint: a config leaf-list reaches the plugin as a scalar OR an array depending on how many values the operator wrote, so `Monitors` MUST be read with `configvalue.LeafList` and never with a direct `[]any` assertion
  → Decision: LDP config is parsed once in `parseLDPConfig`; the parsed struct is what `runSession` can see, so the monitor list is read there and carried, not looked up at emit time
- [ ] `docs/architecture/bgp/interface-event-reactions.md` - the existing precedent for the reactor reacting to a bus event
  → Constraint: a handler runs synchronously inside the EventBus delivery path and MUST NOT hold `reactor.mu` across a bus operation or a peer teardown (the recorded deadlock); it copies the peer set under `RLock` and releases the lock first
  → Decision: this page carries a defect of its own (see Known Limitations); its claim about cease subcode 6 is not a precedent this spec follows
- [ ] `docs/architecture/api/process-protocol.md` - event bus namespaces and payload encoding
  → Constraint: payload JSON keys are kebab-case, and an event payload is a self-contained value type, not a pointer into another package's state
- [ ] `ai/rules/principles.md` - registration over central enumeration
  → Decision: ONE `liveness` namespace for every detector. A namespace per detector would make the reactor hold a list of detectors to subscribe to, which is the central enumeration this rule bans
- [ ] `ai/rules/no-layering.md` - replacing versus adding
  → Decision: this adds a path. BFD keeps its per-session handle and no existing path is replaced, so nothing is deleted first

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc5036.md` - LDP session FSM, KeepAlive, Notification, Status TLV
  → Constraint: `RFC5036-2.5.3-2` is a recorded `{gap}` citing the keepalive-expiry return in `internal/plugins/ldp/session.go`, which is the code this spec edits. The session state machine table in `rfc/full/rfc5036.txt` gives the OPERATIONAL state plus Timeout event the action "Transmit Shutdown msg and close transport connection", and Section 3.5.1.2.3 names the status code
  → Decision: this spec closes the keepalive half of that gap only. The decode-failure half stays open
- [ ] `rfc/short/rfc4486.md` - BGP Cease NOTIFICATION subcodes
  → Constraint: subcode 6 is "a configuration change other than the ones described above", and nothing was configured here, so it is the wrong code. Section 4 recommends `DampPeerOscillations` after subcodes 2, 3, 5 and 8 and NOT after 4, so subcode 4 leaves a transient failure's retry schedule alone
  → Decision: subcode 4, Administrative Reset
- [ ] `rfc/short/rfc9003.md` - Shutdown Communication in a Cease NOTIFICATION
  → Constraint: only subcodes 2 and 4 may carry shutdown communication text (`ShutdownMessage`, `internal/component/bgp/message/notification.go`), which is a second reason subcode 4 is the one that fits
- [ ] `rfc/short/rfc9384.md` - BGP Cease subcode 10, BFD Down
  → Constraint: Section 3 of `rfc/full/rfc9384.txt` makes subcode 10 assert "a BFD session going into the Down state". An LDP keepalive expiry is not that, so subcode 10 would be a false statement on the wire

**Key insights:**
- The detector declares what it monitors. There is no per-BGP-peer list of detectors to opt into, so no BGP config leaf is added by this spec.
- The cause exists in exactly one place in LDP: the error `runSession` receives. `onDone`, the closure that publishes `SessionDown`, takes no argument and cannot tell why the session ended.
- BFD is not folded in, because BGP CREATES the BFD session and control runs BGP to BFD; BGP never creates an LDP session.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/ldp/register.go` - holds `ldpConfig`, `parseLDPConfig`, the `onDone` closure that publishes `SessionDown`, and the `runSession` tail that logs the session end with the error in hand
- [ ] `internal/plugins/ldp/session.go` - declares `errKeepaliveExpiry` and returns it from `ReadLoop` when the read deadline built from the negotiated hold time expires
- [ ] `internal/plugins/ldp/events.go` - `Namespace = "ldp"`, `SessionEvent`, and the three `events.Register` calls; `SessionEvent` carries no reason field
- [ ] `internal/plugins/ldp/wire.go` - LDP PDU and message encoding; there is no Notification message encoder and no Status TLV encoder
- [ ] `internal/plugins/ldp/yang/ze-ldp-conf.yang` - the `container ldp`; it has `interfaces` and no neighbour list, so any per-neighbour association has nowhere to attach
- [ ] `internal/plugins/ospf/ldp_sync.go` - the only consumer of `ldp.SessionEvent`; it reads no reason field
- [ ] `internal/component/config/yang/modules/ze-types.yang` - the shared typedef and grouping module every other module imports
- [ ] `internal/plugins/vrrp/yang/ze-vrrp-conf.yang` - the cross-module `uses` precedent, importing `ze-iface-conf`
- [ ] `internal/core/iface/events/events.go` - the leaf-package event-constant precedent: one namespace, a const block of types, no dependencies
- [ ] `internal/component/bgp/reactor/reactor.go` - `subscribeInterfaceEvents()` is called from the reactor start path; the peer map is keyed by `netip.AddrPort`; `teardownAutomatic` is the teardown entry point
- [ ] `internal/component/bgp/reactor/reactor_iface.go` - the handler shape to copy: unmarshal, then a payload handler; the handler comment states it MUST NOT hold `reactor.mu`
- [ ] `internal/component/bgp/reactor/peer_bfd.go` - `api.Service.EnsureSession`; BGP creates the BFD session, which is why BFD keeps its own handle
- [ ] `internal/component/bgp/message/notification.go` - Cease subcode constants and `ShutdownMessage`, which restricts RFC 9003 text to subcodes 2 and 4

**Behavior to preserve:**
- `ldp.SessionEvent` keeps its exact field set and JSON keys; `internal/plugins/ospf/ldp_sync.go` reads it and must not change.
- `SessionDown` keeps being published for every session end, whatever the cause.
- A BGP peer whose LDP neighbour is healthy, and a deployment with no `monitor` configured, behave exactly as today: no new teardown, no new NOTIFICATION.
- BFD-driven teardown keeps its own path and its own Cease subcode 10.
- The LDP session still closes its transport on keepalive expiry, at the same point in the sequence.

**Behavior to change:**
- On keepalive expiry only, LDP transmits a Shutdown Notification, then publishes a `liveness` `peer-down` event when `monitor` names at least one protocol.
- The BGP reactor subscribes to `liveness` `peer-down` and tears down every peer at the named address when `monitors` contains `bgp`.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- The operator writes `monitor bgp` inside `ldp { }`. The value enters as config tree text and reaches the LDP plugin as a JSON scalar or array.
- At runtime the entry point is the expiry of the LDP read deadline: `ReadLoop` returns `errKeepaliveExpiry` to `runSession`.

### Transformation Path
1. Config parse: `parseLDPConfig` reads `monitor` with `configvalue.LeafList` into `ldpConfig.Monitors`.
2. Session death: `ReadLoop` returns `errKeepaliveExpiry`; `runSession` receives it at its tail.
3. LDP wire: the session transmits a Shutdown Notification (Status Data `0x00000014`, E-bit set) on a best-effort write, then closes the transport.
4. Emit: the `runSession` tail, gated on `errors.Is(err, errKeepaliveExpiry)` and on a non-empty `Monitors`, publishes `liveness` `peer-down` with address, detector `ldp`, cause `keepalive-expired`, the monitor list, and the discovering interface.
5. Bus delivery: the reactor's `liveness` handler unmarshals the payload, filters on `monitors` containing `bgp`, and parses and `Unmap`s the address.
6. Peer match: the handler copies, under `RLock`, the set of peers whose `netip.AddrPort` key `.Addr()` equals the event address, releases the lock, then calls `teardownAutomatic` on each.
7. Wire out: each teardown sends a NOTIFICATION Cease subcode 4 carrying RFC 9003 text naming the detector and the cause.
8. RIB: the peer's routes are withdrawn from the Loc-RIB and from the peers that received them, by the existing teardown path.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| LDP plugin → event bus | `events.Register[*PeerDown]` publish in the `liveness` namespace, kebab-case JSON payload | No |
| Event bus → BGP reactor | `eventBus.Subscribe(liveness.Namespace, liveness.EventPeerDown, ...)`, synchronous delivery | No |
| Config tree → LDP plugin | `configvalue.LeafList` over the `monitor` leaf-list, scalar-or-array tolerant | No |
| BGP reactor → wire | `teardownAutomatic` → NOTIFICATION Cease subcode 4 with RFC 9003 shutdown communication | No |
| LDP session → wire | Notification message plus Status TLV, new encoders in `wire.go` | No |

### Integration Points
- `internal/plugins/ldp/register.go`, the `runSession` tail - the only site where the death cause is in scope.
- `internal/component/bgp/reactor/reactor.go`, beside `r.subscribeInterfaceEvents()` - the reactor's existing bus subscription point.
- `teardownAutomatic` in `internal/component/bgp/reactor/` - the existing teardown entry, extended with the Cease subcode and shutdown text this path needs.
- `internal/component/config/yang/modules/ze-types.yang` - the shared `grouping liveness-monitor`, `uses`d by any future detector module.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `errKeepaliveExpiry` is the only error that means "the address stopped answering", and every other `runSession` error means something else | `internal/plugins/ldp/session.go` declares it and `ReadLoop` returns it on the read deadline alone | A decode failure or a transport reset would tear BGP down on evidence that does not prove the address is dead | Unit test asserting no event for a decode failure and for a clean shutdown | unvalidated |
| A-2 | The peer map key `netip.AddrPort` `.Addr()` is the peer's remote address, so comparing it to the `Unmap`ped event address is the correct match | the peer map declaration in `internal/component/bgp/reactor/reactor.go` | The teardown would match the wrong peer or no peer | Unit test with an IPv4-mapped IPv6 event address matching an IPv4 peer | unvalidated |
| A-3 | An LDP hold time expiring means the ADDRESS is unreachable, not that LDP alone is unhealthy | Design premise; the operator opts in by writing `monitor bgp` | A one-protocol fault tears down a healthy BGP session | The opt-in itself: no `monitor`, no teardown. Interop scenario measuring the BGP session state | unvalidated |
| A-4 | The `liveness` payload needs no per-peer scoping beyond the address, because a detector monitors an address rather than a session | Owner decision: the detector declares what it monitors | A router with two BGP sessions to one address over different VRFs or ports would tear both down | Unit test asserting every peer at the address is torn down, which is the intended behavior | unvalidated |
| A-5 | `wire.go` can encode a Notification message and one Status TLV without restructuring its PDU writer | `internal/plugins/ldp/wire.go` currently encodes Hello, Initialization, KeepAlive and Label Mapping | The RFC half of this spec grows into a wire-layer refactor and needs its own phase | Read the PDU writer before phase 4; encode a Notification and decode it back in a unit test | unvalidated |
| A-6 | A leaf-list `monitor` in `container ldp` is reachable by `parseLDPConfig` in both the single-value and the multi-value spellings | `docs/architecture/ldp/mpls-ldp.md` names the scalar-or-array trap; `configvalue.LeafList` exists for it | The single-value spelling silently yields an empty list, so the feature is inert for the commonest config | Unit test over both spellings, and a functional test that writes one value | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The reactor handler holds `reactor.mu` across a teardown and deadlocks the bus delivery goroutine | A test hangs; the `subscribeInterfaceEvents` comment records the same deadlock | Copy the peer set under `RLock`, release, then act. A race-detector test publishes the event while peers are being added |
| R-2 | A flapping LDP adjacency tears BGP down repeatedly and oscillates the RIB | Repeated NOTIFICATION Cease in the peer log | Subcode 4 is deliberately outside the RFC 4486 `DampPeerOscillations` recommendation, so the existing retry schedule is unchanged. If oscillation is observed, the fix is a detector-side hold-down, not a BGP-side one |
| R-3 | The Shutdown Notification write blocks on a dead socket and delays the teardown | The session end takes as long as the write timeout | Best effort by owner decision: the write uses the existing write deadline, and a failure is logged and ignored |
| R-4 | The event carries an address the reactor cannot parse, or an empty one | Handler returns early with a debug log | Parse failure is a silent return, matching `onInterfaceAddrRemoved`; a unit test covers the malformed case |
| R-5 | Two detectors report the same address and the reactor tears down a peer twice | The second teardown finds the peer already down | `teardownAutomatic` on a non-established peer is a no-op; the test asserts it |
| R-6 | The new `liveness` namespace collides with a per-detector namespace someone adds anyway | A second detector registers `ldp-liveness` or a similar name | The grouping and the single namespace are both in shared files, so a new detector's natural route is to `uses` the grouping and publish into `liveness` |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Live BGP sessions are dropped that should not be, and their routes are withdrawn from the Loc-RIB and from every peer that received them. That is a forwarding outage, not a cosmetic defect. It is gated behind an opt-in the operator writes, so a deployment with no `monitor` leaf is unaffected |
| How is it reverted? | Single commit revert. The event is new, the subscription is new, and no existing path changes shape. Peers reconnect on their normal retry schedule |
| Who else touches this path? | Any session working `internal/plugins/ldp/**` (the LDP rows in `plan/journal/zero-value-as-valid-answer.md`, and `plan/pre-release/spec-rfc-evidence-deferred-isis-rsvpte-ldp-tranche.md`), and any session working `internal/component/bgp/reactor/**` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `monitor bgp` in `ldp { }` config | → | `parseLDPConfig` filling `ldpConfig.Monitors` | `TestParseLDPConfigReadsMonitorLeafList` |
| LDP read deadline expiry | → | the `runSession` tail publishing `liveness` `peer-down` | `TestLDPKeepaliveExpiryPublishesLivenessPeerDown` |
| `liveness` `peer-down` on the bus | → | `Reactor.onLivenessPeerDown` | `TestReactorSubscribesToLivenessPeerDown` |
| `liveness` `peer-down` naming a peer address | → | `teardownAutomatic` on the matching peers | `TestLivenessPeerDownTearsDownMatchingPeers` |
| Operator config plus an expiring LDP neighbour | → | the whole chain to a withdrawn route | `test/ldp/liveness-tears-down-bgp-peer.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | An LDP session whose config names `monitor bgp` dies because its keepalive expired | One `liveness` `peer-down` event is published, carrying the neighbour address, detector `ldp`, cause `keepalive-expired`, `monitors` holding `bgp`, and the discovering interface |
| AC-2 | The same LDP session dies for any other reason: a decode failure, a clean shutdown, a transport reset | No `liveness` event is published. `ldp` `session-down` is published exactly as today |
| AC-3 | An LDP instance with no `monitor` leaf loses a session to keepalive expiry | No `liveness` event is published, and no BGP session changes state |
| AC-4 | A `liveness` `peer-down` event whose `monitors` holds `bgp` names an address with two Established BGP peers | Both peers leave Established, and neither survives the event |
| AC-5 | The same event names an address no BGP peer uses, or its `monitors` omits `bgp` | No BGP peer changes state, and the handler returns without error |
| AC-6 | A peer is torn down by a `liveness` event | The NOTIFICATION on the wire is Cease, subcode 4 Administrative Reset, and its RFC 9003 shutdown communication names the detector and the cause |
| AC-7 | A peer torn down by a `liveness` event had advertised routes into the Loc-RIB | Those routes leave the Loc-RIB and are withdrawn to the peers that received them |
| AC-8 | An LDP session reaches keepalive expiry | A Shutdown Notification, Status Data `0x00000014` with the E-bit set, is transmitted before the transport is closed |
| AC-9 | The Shutdown Notification write fails because the socket is already gone | The session still closes, the `liveness` event is still published, and the failure is logged once |
| AC-10 | Config names `monitor bgp` | The config validates. `monitor ospf`, or any value the enumeration does not carry, is refused at validation with a message naming the leaf |
| AC-11 | Config names one `monitor` value, and separately several | Both spellings yield the same parsed list; the single-value spelling does not yield an empty list |
| AC-12 | A `liveness` `peer-down` event is delivered while another goroutine adds a peer | No deadlock and no data race under `-race`; the handler holds no reactor lock while tearing a peer down |
| AC-13 | A `liveness` `peer-down` event carries a malformed or empty address | The handler returns without a panic and without tearing any peer down |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Configures `ldp { monitor bgp; }` and loses the LDP neighbour to a dead link | config tree → `parseLDPConfig` → `runSession` → `liveness` `peer-down` → reactor handler → `teardownAutomatic` → NOTIFICATION → RIB withdraw | `test/ldp/liveness-tears-down-bgp-peer.ci` |
| 2 | Runs Ze against FRR, kills the LDP keepalive, and watches FRR report the BGP session closed by an administrative reset | Ze LDP timer → Ze BGP NOTIFICATION Cease subcode 4 with RFC 9003 text → FRR log | interop scenario `ldp-liveness-bgp-cease-frr` |
| 3 | Runs Ze against FRR, lets the LDP keepalive expire, and watches FRR report the received Shutdown Notification | Ze LDP timer → LDP Notification message with Status Data `0x00000014` → FRR log | interop scenario `ldp-keepalive-shutdown-notification-frr` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestParseLDPConfigReadsMonitorLeafList` | `internal/plugins/ldp/register_test.go` | AC-11: single-value and multi-value spellings both parse | |
| `TestLDPKeepaliveExpiryPublishesLivenessPeerDown` | `internal/plugins/ldp/register_test.go` | AC-1: payload fields, namespace and type | |
| `TestLDPOtherSessionEndPublishesNoLivenessEvent` | `internal/plugins/ldp/register_test.go` | AC-2: decode failure and clean shutdown are silent | |
| `TestLDPNoMonitorPublishesNoLivenessEvent` | `internal/plugins/ldp/register_test.go` | AC-3 | |
| `TestLDPEncodeShutdownNotification` | `internal/plugins/ldp/wire_test.go` | AC-8: message type, Status TLV, Status Data `0x00000014`, E-bit | |
| `TestLDPShutdownNotificationWriteFailureIsBestEffort` | `internal/plugins/ldp/session_test.go` | AC-9 | |
| `TestReactorSubscribesToLivenessPeerDown` | `internal/component/bgp/reactor/reactor_liveness_test.go` | Wiring: the subscription exists on the start path | |
| `TestLivenessPeerDownTearsDownMatchingPeers` | `internal/component/bgp/reactor/reactor_liveness_test.go` | AC-4 | |
| `TestLivenessPeerDownIgnoresUnmatchedAddressAndMonitors` | `internal/component/bgp/reactor/reactor_liveness_test.go` | AC-5 | |
| `TestLivenessPeerDownUsesCeaseAdministrativeReset` | `internal/component/bgp/reactor/reactor_liveness_test.go` | AC-6: subcode 4 and the RFC 9003 text content | |
| `TestLivenessPeerDownHoldsNoLockDuringTeardown` | `internal/component/bgp/reactor/reactor_liveness_test.go` | AC-12, run under `-race` | |
| `TestLivenessPeerDownMalformedAddress` | `internal/component/bgp/reactor/reactor_liveness_test.go` | AC-13 | |
| `TestLivenessPeerDownPayloadJSONKeys` | `internal/core/liveness/events/events_test.go` | Payload keys are kebab-case and the type round-trips | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| LDP Status Data (Shutdown) | fixed `0x00000014` | `0x00000014` | N/A | N/A |
| BGP Cease subcode | 1-10 (RFC 4486, RFC 9003, RFC 9384) | 4 on this path | N/A | N/A |
| RFC 9003 shutdown communication length | 0-255 octets | 255 | N/A | 256 is refused by the encoder |
| `monitor` leaf-list length | 0-1 today, one enum value | 1 | N/A | a repeated value is refused by YANG |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `liveness-tears-down-bgp-peer` | `test/ldp/liveness-tears-down-bgp-peer.ci` | An operator with `monitor bgp` loses the LDP neighbour and sees the BGP peer leave Established and its routes leave the RIB | |
| `liveness-monitor-absent-keeps-bgp` | `test/ldp/liveness-monitor-absent-keeps-bgp.ci` | An operator with no `monitor` leaf loses the LDP neighbour and the BGP peer stays Established | |
| `ldp-monitor-config-validation` | `test/ldp/ldp-monitor-config-validation.ci` | `monitor bgp` validates, and an unknown value is refused with a message naming the leaf | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `ldp-liveness-bgp-cease-frr` | `test/interop/scenarios/ldp-liveness-bgp-cease-frr/` | FRR | FRR reports the BGP session closed by Cease Administrative Reset carrying Ze's shutdown communication, after the LDP keepalive to the same address expires | |
| `ldp-keepalive-shutdown-notification-frr` | `test/interop/scenarios/ldp-keepalive-shutdown-notification-frr/` | FRR | FRR receives and logs an LDP Shutdown Notification on keepalive expiry rather than a bare transport close | |

## Files to Modify
- `internal/plugins/ldp/register.go` - `ldpConfig` gains `Monitors`; `parseLDPConfig` reads the leaf-list; the `runSession` tail publishes the event when `errors.Is(err, errKeepaliveExpiry)` holds
- `internal/plugins/ldp/session.go` - transmit the Shutdown Notification before closing the transport on keepalive expiry
- `internal/plugins/ldp/wire.go` - Notification message encoder and Status TLV encoder
- `internal/plugins/ldp/yang/ze-ldp-conf.yang` - import `ze-types` and `uses liveness-monitor` inside `container ldp`
- `internal/component/config/yang/modules/ze-types.yang` - `grouping liveness-monitor` with `leaf-list monitor`, typed as an enumeration carrying `bgp`
- `internal/component/bgp/reactor/reactor.go` - call the new subscription beside `r.subscribeInterfaceEvents()`
- `internal/component/bgp/reactor/reactor_peers.go` - the teardown call path carries the Cease subcode and the RFC 9003 text for this cause
- `docs/architecture/ldp/mpls-ldp.md` - the monitor leaf, the emitted event, and the Shutdown Notification on keepalive expiry
- `docs/architecture/bgp/interface-event-reactions.md` - the reactor's second bus reaction, and the correction named in Known Limitations
- `docs/architecture/core-design.md` - declared by the `// Design:` header of both changed reactor files; the liveness-driven teardown joins the ways a peer leaves Established
- `rfc/short/rfc5036.md` - `RFC5036-2.5.3-2` narrows from a whole `{gap}` to the decode-failure half, with the keepalive half proven
- `docs/features/rfc-status.md` - the RFC 5036 row's counts and its `Support remaining` prose
- `docs/guide/mpls.md` - the operator-facing `monitor` leaf, if that page carries the LDP config surface

## Files to Create
- `internal/core/liveness/events/events.go` - `Namespace = "liveness"`, `EventPeerDown = "peer-down"`, the `PeerDown` payload type, and `events.Register[*PeerDown]`
- `internal/core/liveness/events/events_test.go` - payload key and round-trip test
- `internal/component/bgp/reactor/reactor_liveness.go` - the subscription and the handler
- `internal/component/bgp/reactor/reactor_liveness_test.go` - the reactor-side unit tests
- `test/ldp/liveness-tears-down-bgp-peer.ci` - the end-to-end operator scenario
- `test/ldp/liveness-monitor-absent-keeps-bgp.ci` - the opt-out scenario
- `test/ldp/ldp-monitor-config-validation.ci` - the config validation scenario
- `test/interop/scenarios/ldp-liveness-bgp-cease-frr/` - scenario, `ze.conf`, FRR config, assertions
- `test/interop/scenarios/ldp-keepalive-shutdown-notification-frr/` - scenario, `ze.conf`, FRR config, assertions

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `grouping liveness-monitor` in `internal/component/config/yang/modules/ze-types.yang`, `uses`d in `internal/plugins/ldp/yang/ze-ldp-conf.yang` |
| YANG validation constraints | Yes | `leaf-list monitor` is an enumeration carrying `bgp`, which is the maximum native constraint for a one-value vocabulary |
| YANG custom validators | N-A | The enumeration is sufficient; no cross-leaf constraint exists |
| CLI commands/flags | N-A | No new verb. The feature is configured, not commanded |
| CLI grammar (keyword before value) | N-A | No new command |
| Editor autocomplete | Yes | Automatic from the YANG enumeration; no `CompleteFn` needed |
| Functional test for new RPC/API | Yes | `test/ldp/liveness-tears-down-bgp-peer.ci` and the two beside it |
| Pipe completeness | N-A | No new command output |
| Env var registration | N-A | No leaf under `environment/` |
| Doctor check for runtime dependencies | N-A | No new file path, socket, port, kernel module, binary, or certificate. The feature uses the LDP socket that already exists and the in-process event bus |
| Prometheus counters/metrics | No | Deliberate: the teardown is already visible in the peer state and the NOTIFICATION log. A counter with no consumer is a second declaration of what the peer state holds |
| BGP family surface (new SAFI / capability / attribute) | N-A | No new family, capability, or attribute. The NOTIFICATION subcode is an existing constant |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` - one row for protocol-driven liveness teardown |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md` and `docs/config-reference.md` for the `monitor` leaf under `ldp` |
| 3 | CLI command added/changed? | N-A | No command added |
| 4 | API/RPC added/changed? | N-A | No RPC added |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` if it lists LDP's config surface; settled by grep in the phase that edits the YANG |
| 6 | Has a user guide page? | Yes | `docs/guide/mpls.md` - the LDP operator page carries the new leaf |
| 7 | Wire format changed? | Yes | `docs/architecture/ldp/mpls-ldp.md` - the LDP Notification message and Status TLV encoding |
| 8 | Plugin SDK/protocol changed? | No | The event travels the internal bus; no SDK type changes |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc5036.md` for the `RFC5036-2.5.3-2` keepalive half, the `docs/features/rfc-status.md` RFC 5036 row, and `rfc/short/rfc4486.md` if its Ze-side notes name the subcodes Ze emits |
| 10 | Test infrastructure changed? | No | Existing `.ci` and interop harnesses; two new named scenarios, no new runner |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` - written only if that page already compares liveness detection between daemons |
| 12 | Internal architecture changed? | Yes | `docs/architecture/bgp/interface-event-reactions.md` gains the `liveness` reaction beside the interface ones. `docs/architecture/core-design.md` is declared by the `// Design:` headers of both changed reactor files, `reactor.go` ("BGP reactor event loop and peer management") and `reactor_peers.go` ("peer add/remove/lookup"), so the phase that edits either one reads that page and adds the liveness-driven teardown wherever it enumerates how a peer leaves Established |
| 13 | Route metadata keys added/changed? | N-A | No route metadata |
| 14 | Prometheus counters added/changed? | N-A | None added |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | A new registered event type: the inventories in `docs/plugin-overview.md` and `docs/features/plugins.md` |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED at implementation time by `./le spec citation anchors spec plan/spec-liveness-event-tears-down-bgp-peer.md`. `internal/plugins/ldp/events.go` and `internal/plugins/ldp/wire.go` both declare a `// Design:` header naming `docs/architecture/ldp/mpls-ldp.md`, which row 7 already names |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | Every `ldp { }` example on the pages named above is re-validated against the changed YANG in the phase that changes it |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the event package, the YANG grouping, and both ends registered
   - Tests: `TestParseLDPConfigReadsMonitorLeafList`, `TestReactorSubscribesToLivenessPeerDown`, `TestLivenessPeerDownPayloadJSONKeys`
   - Files: `internal/core/liveness/events/events.go`, `internal/component/config/yang/modules/ze-types.yang`, `internal/plugins/ldp/yang/ze-ldp-conf.yang`, `internal/plugins/ldp/register.go`, `internal/component/bgp/reactor/reactor.go`, `internal/component/bgp/reactor/reactor_liveness.go`
   - Verify: the subscription exists on the reactor start path and the config leaf reaches `ldpConfig`. The teardown is a stub, so the teardown tests fail
2. **Phase: Emit** -- publish the event from the `runSession` tail
   - Tests: `TestLDPKeepaliveExpiryPublishesLivenessPeerDown`, `TestLDPOtherSessionEndPublishesNoLivenessEvent`, `TestLDPNoMonitorPublishesNoLivenessEvent`
   - Files: `internal/plugins/ldp/register.go`
   - Verify: the event is published for keepalive expiry and for nothing else
3. **Phase: Consume** -- match peers and tear them down
   - Tests: `TestLivenessPeerDownTearsDownMatchingPeers`, `TestLivenessPeerDownIgnoresUnmatchedAddressAndMonitors`, `TestLivenessPeerDownUsesCeaseAdministrativeReset`, `TestLivenessPeerDownHoldsNoLockDuringTeardown`, `TestLivenessPeerDownMalformedAddress`
   - Files: `internal/component/bgp/reactor/reactor_liveness.go`, `internal/component/bgp/reactor/reactor_peers.go`
   - Verify: the NOTIFICATION carries Cease subcode 4 and the RFC 9003 text; the race test is clean under `-race`
4. **Phase: LDP Shutdown Notification (RFC 5036)** -- encode and transmit it
   - Tests: `TestLDPEncodeShutdownNotification`, `TestLDPShutdownNotificationWriteFailureIsBestEffort`, plus the RFC-tagged unit that discharges the keepalive half of `RFC5036-2.5.3-2`
   - Files: `internal/plugins/ldp/wire.go`, `internal/plugins/ldp/session.go`, `rfc/short/rfc5036.md`
   - Verify: the encoded message decodes back to Status Data `0x00000014` with the E-bit set; `./le rfc discriminate-record` writes the record for the tagged unit
5. **Phase: End to end** -- functional and interop coverage, and the remaining pages
   - Tests: the three `.ci` scenarios and the two named interop scenarios
   - Files: `test/ldp/`, `test/interop/scenarios/ldp-liveness-bgp-cease-frr/`, `test/interop/scenarios/ldp-keepalive-shutdown-notification-frr/`, and every page named in the Documentation Update Checklist that phases 1 to 4 did not already edit
   - Verify: each interop scenario is observed RED with the change reverted and the artifact rebuilt, then GREEN, and the RED is recorded

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at a named file and symbol |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | The emit is gated on `errors.Is(err, errKeepaliveExpiry)` and reachable from no other error; the address comparison uses `Unmap`ped `netip.Addr` on both sides |
| Correctness | The Cease subcode on the wire is 4 and never 6 or 10, and the RFC 9003 text is within its length limit |
| Naming | Event JSON keys are kebab-case; namespace `liveness`, type `peer-down`, detector `ldp`, cause `keepalive-expired`; the YANG leaf-list is `monitor` |
| Data flow | The reactor learns of the detector only through the payload's `monitors` field: no package under `internal/component/bgp/` imports `internal/plugins/ldp` |
| Concurrency | No reactor lock is held across a bus operation or a teardown; the peer set is copied under `RLock` and the lock released first |
| Rule: `ai/rules/principles.md` | One `liveness` namespace, and no per-detector enumeration anywhere in the reactor |
| Rule: `ai/rules/rfc-compliance.md` | Every MUST the new LDP code enforces carries its `// RFC 5036 Section X.Y: "..."` comment; the RFC row states only what the test body checks |
| Rule: `ai/rules/no-layering.md` | BFD's per-session handle is untouched and no path is duplicated by the new one |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| The `liveness` event package exists and is registered | `gopls symbols internal/core/liveness/events/events.go` |
| The reactor subscribes on its start path | `grep -n liveness internal/component/bgp/reactor/reactor.go` |
| No BGP package imports the LDP plugin | `go list -deps ./internal/component/bgp/reactor` names no `internal/plugins/ldp` |
| The LDP Notification encoder exists and has a non-test caller | `gopls references` on the encoder |
| The RFC row is proven, not asserted | `./le rfc check`, and the record under `rfc/discrimination/` |
| The two interop scenarios exist and run | `./le integration` with each scenario name |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | The event payload arrives from the in-process bus. The address is parsed and rejected on failure; `monitors` is compared against a fixed value and never used to build a path or a command |
| Denial of service | A peer that can flap an LDP adjacency can flap the BGP session at the same address. That reach is what the operator opted into, and RFC 4486 leaves subcode 4 outside the damping recommendation, so the retry schedule is unchanged. Confirm no unbounded goroutine or allocation per event |
| Information disclosure | The RFC 9003 shutdown communication is sent to the peer. It names the detector and the cause, and MUST NOT carry an interface name, an internal address, or config text |

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

- The cause of a session death exists for one instant in one place. `onDone`, the closure that publishes `SessionDown`, takes no argument, so the only site that can name the cause is the `runSession` tail that holds the error.
- LDP's YANG has no neighbour list, only `interfaces`, so a per-neighbour association has nowhere to attach. That is what makes the association global to the LDP instance, and it is a property of the schema rather than a simplification.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| One `liveness` namespace for every detector | A namespace per detector (`ldp-liveness`, `ike-liveness`) | A per-detector namespace makes the reactor hold a list of detectors to subscribe to, which is the central enumeration `ai/rules/principles.md` bans. `internal/core/iface/events/events.go` is the precedent: one namespace, many types |
| The DETECTOR declares what it monitors | A per-BGP-peer list of detectors to trust | Owner decision: "LDP should always be associated to one or more protocols it monitors". One leaf-list on the detector, no BGP-side config, and a new detector adds no BGP schema |
| The association is declared in YANG as a shared `grouping liveness-monitor` | A leaf declared privately in each detector's module | One declaration, `uses`d by every detector, so the vocabulary cannot drift. `internal/plugins/vrrp/yang/ze-vrrp-conf.yang` is the cross-module `uses` precedent |
| Cease subcode 4, Administrative Reset, with RFC 9003 text | Subcode 10, BFD Down (RFC 9384); subcode 6, Other Configuration Change (RFC 4486) | Subcode 10 asserts a BFD session went Down, which is false here. Subcode 6 asserts a configuration change, and nothing was configured. Only subcodes 2 and 4 may carry RFC 9003 text, and RFC 4486 Section 4 leaves 4 outside its `DampPeerOscillations` recommendation, so a transient failure's retry schedule is unchanged |
| BFD keeps its per-session handle | Fold BFD into the same event | BGP CREATES the BFD session (`api.Service.EnsureSession`, `internal/component/bgp/reactor/peer_bfd.go`), so control runs BGP to BFD; BGP never creates an LDP session. Folding would also force one generic handler to emit subcode 10 for one detector and 4 for the rest |
| Best-effort Shutdown Notification | Block the teardown until the write succeeds or times out | Owner decision. The peer is already unreachable by hypothesis, so a write that fails proves the point rather than blocking the response to it |

## Known Limitations

- Only LDP produces the event. IKE DPD and BFD are the expected next producers and are not in scope here.
- Only BGP consumes it. The `monitor` enumeration carries one value, `bgp`, and grows when a second consumer exists.
- The association is global to the LDP instance, not per neighbour, because LDP's YANG has no neighbour list. A per-neighbour association needs that list first and is a spec of its own if an operator asks for it.
- The decode-failure half of `RFC5036-2.5.3-2` stays a gap. This spec closes the keepalive half only, and `RFC5036-3.5.1-1` follows the decode-failure half.
- A reason field on `ldp.SessionEvent` is deliberately NOT added and owes no spec: the new event carries `cause`, and the only consumer of `SessionEvent`, `internal/plugins/ospf/ldp_sync.go`, reads no reason.
- `docs/architecture/bgp/interface-event-reactions.md` claims that a disappearing address "drains gracefully with NOTIFICATION cease subcode 6, per RFC 4486", while `handleAddrRemovedPayload` only stops the listener and `message.NotifyCeaseOtherConfigChange` has no non-test caller in `internal/`. That defect is recorded in `plan/journal/unwired-feature.md` and is NOT this spec's to fix. This spec edits the same page, so correcting that one sentence is in scope for the page edit; wiring an address-removal teardown is not.

## Open Questions

| # | Question | Reading proceeded under | Who decides |
|---|----------|------------------------|-------------|
| Q-1 | The namespace, type and cause are spelled `liveness`, `peer-down` and `keepalive-expired`. Thomas's own phrasing for the feature was "tcp failure with IP", which would spell them `tcp-failure`. The generic naming was chosen because IKE DPD and BFD are the next producers and both run over UDP, so a `tcp-failure` name would be false for them | The generic spelling: `liveness`, `peer-down`, `keepalive-expired` | Thomas. Renaming before implementation costs three constants and one YANG grouping name; renaming after costs the payload keys and the recorded RFC evidence too |

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: validation rules, error conditions, state transitions, timer
constraints, message ordering, and every MUST/MUST NOT.

- RFC 5036 Section 2.5.3 and the session state machine table in `rfc/full/rfc5036.txt`: the OPERATIONAL state plus Timeout event action is "Transmit Shutdown msg and close transport connection". The comment sits above the transmit in `session.go`.
- RFC 5036 Section 3.5.1.2.3: the Shutdown status code and its Status Data value. The comment sits above the encoder in `wire.go`.
- RFC 4486 Section 4: subcode 4, Administrative Reset, and the absence of a `DampPeerOscillations` recommendation for it. The comment sits above the subcode choice in the reactor.
- RFC 9003: the Shutdown Communication and the subcodes allowed to carry it. The comment sits above the text construction.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-13 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes. It runs every stage against a COMMIT in a throwaway worktree, which is the pre-commit gate (`ai/rules/git-safety.md`). An in-place `./le verify current` is void the moment the tree moves under it
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)
