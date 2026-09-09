# Spec: bgp-bfd-strict

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 1/4 |
| Updated | 2026-09-08 |

Design rewrite (2026-09-08, owner decision): the 2026-07-03 design made BFD
strict a LOCAL establishment policy and stated "No capability is negotiated with
the peer." That is void. Ze implements `draft-ietf-idr-bgp-bfd-strict-mode-19`
in full: the BFD Strict-Mode Capability (code 74) and the FSM changes the draft
makes to RFC 4271 Section 8.2.2. The draft text is at
`rfc/drafts/draft-ietf-idr-bgp-bfd-strict-mode.txt`.

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md`
3. `rfc/drafts/draft-ietf-idr-bgp-bfd-strict-mode.txt` - Sections 3 to 10
4. `internal/component/bgp/fsm/fsm.go` - the RFC 4271 FSM the draft revises
5. `internal/component/bgp/reactor/peer_bfd.go` - current BFD/BGP integration

## Task

Today Ze's BFD-for-BGP is a **failure detector only**: the BFD session opens
*after* the BGP session reaches Established, and BFD Up transitions are ignored.
A peer whose control plane is reachable but whose forwarding path is broken can
therefore establish BGP and blackhole traffic until the BGP hold timer fires.

Implement `draft-ietf-idr-bgp-bfd-strict-mode-19`. Two halves:

1. **The capability.** Section 5 defines capability code 74, length 0 octets.
   Section 6 makes it a MUST for a speaker with strict-mode enabled, and sets
   the `BfdStrictNegotiated` session attribute TRUE when both speakers send it.
2. **The FSM.** Sections 4 and 8 add six events and the sub-states that hold a
   BGP session in OpenSent until the BFD session reaches Up. Section 7 says when
   the BFD session starts and stops. Section 9 says every session this feature
   closes carries Cease (6) / BFD Down (10).

Config surface, under the existing peer `bfd` container:

- `strict` (boolean): advertise capability 74 and run the strict-mode procedures.
- `hold-time <sec>`: the `BfdHoldTime` attribute (Section 3, item 18), default 30.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/wire/capabilities.md` - the capability TLV, the code table, the negotiation rules
  → Constraint: a new capability is a `Capability` implementation plus one `parseCapability` arm; the code table on that page is a declared fact and the page is edited in this work.
- [ ] `docs/architecture/behavior/fsm.md` - the RFC 4271 FSM, its states and its events
  → Constraint: the draft revises Section 8.2.2, so the sub-states and the six events land in `internal/component/bgp/fsm`, not in the reactor.
- [ ] `docs/architecture/core-design.md` - reactor/FSM and plugin service discovery
  → Constraint: the BGP reactor reaches the BFD plugin via `api.GetService()` (in-process), never by importing the BFD package.
- [ ] `docs/architecture/bfd.md` - the BFD engine, its sessions and its state channel
- [ ] `docs/guide/bfd.md` - the operator-facing BFD configuration
- [ ] `docs/architecture/testing/interop.md` - the suites, the scenario naming rule and the revert-to-RED discrimination walk

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/drafts/draft-ietf-idr-bgp-bfd-strict-mode.txt` - the normative source. Section 3 session attributes, Section 4 events 30-35, Section 5 capability 74, Section 6 negotiation, Section 7 session start and stop, Section 8 FSM changes, Section 9 the close subcode, Section 10 stability.
  → Constraint: Section 6 is a MUST. A speaker with strict-mode enabled advertises the capability in its OPEN.
  → Constraint: Section 8.1 gates every sub-state on "BFD is enabled, and BFD strict-mode is enabled and negotiated". A peer that does not advertise 74 negotiates nothing, and the session runs the unmodified RFC 4271 FSM.
- [ ] `rfc/short/rfc4271.md` - the FSM the draft updates (Section 8.2.2)
- [ ] `rfc/short/rfc5492.md` - capability advertisement, the negotiation intersection
- [ ] `rfc/short/rfc5882.md` - how a BFD client requests, reacts to and coexists with BFD sessions
  → Constraint: Section 4.2 says a client SHOULD NOT take control protocol action when the session transitions to AdminDown. Draft Section 8.7.1 states the same rule as an FSM clause.
- [ ] `rfc/short/rfc6286.md` - the BGP Identifier rules the OPEN rail already enforces
- [ ] `rfc/short/rfc2918.md`, `rfc/short/rfc7313.md` - the zero-length capabilities whose shape capability 74 copies

**Key insights:**
- BFD strict is NOT local policy. It is negotiated, and the sub-states are only entered when both speakers advertised code 74.
- Ze does not implement the RFC 4271 DelayOpenTimer (`fsm.go` header, permitted by RFC 4271 Section 8.2.1.3). Draft Sections 8.3.5 and 8.4.5 revise Event 20, which only fires while that timer runs, so `ConnectDelayOpenBfdUpPending` and `ActiveDelayOpenBfdUpPending` are unreachable in Ze. That is an optional feature out of scope, not a gap.
- Ze always arms a hold timer in OpenSent (the `ze.bgp.openwait` bound, `session_connection.go`). The `BfdHoldTimer` is the draft's answer for a NEGOTIATED hold time of zero, which is a different timer at a different moment.
- Every state handler in `fsm.go` ends in a default arm that returns `ErrFSMError` and drops to Idle, so each of the six new events needs an explicit arm in each of the six states.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/reactor/peer_bfd.go` - `startBFDClient` is called from the FSM callback on `StateEstablished`; `runBFDSubscriber` acts only on Down and AdminDown and says "BFD is a failure detector, not a session driver". Down and AdminDown both call `teardownAutomatic` with `NotifyCeaseBFDDown`.
- [ ] `internal/component/bgp/reactor/peer_settings.go` - `BFDSettings` has `Enabled`, `MultiHop`, `Profile`, `MinTTL`, `Interface`. No `Strict`, no `HoldTime`.
- [ ] `internal/component/bgp/reactor/config.go` - `parseBFDSettings` decodes the `bfd` container; `parsePeerFromTree` reaches it through `connection > bfd`.
- [ ] `internal/component/bgp/reactor/config_capabilities.go` - `parseCapabilitiesFromTree` appends each advertised capability to `PeerSettings.Capabilities`.
- [ ] `internal/component/bgp/yang/ze-bgp-conf.yang` - the peer `bfd` container carries `enabled`, `mode`, `profile`, `min-ttl`, `interface`.
- [ ] `internal/core/bgp/capability/capability.go` - `Code`, the `Capability` interface, `parseCapability`, `parseZeroLengthCapability`, `RouteRefresh` as the zero-length shape.
- [ ] `internal/core/bgp/capability/negotiated.go` - `Negotiate` builds the local and remote sets, intersects them, records `Mismatch` rows and fills `peerCodes`.
- [ ] `internal/component/bgp/fsm/state.go` - six states, eighteen events, no sub-state.
- [ ] `internal/component/bgp/fsm/fsm.go` - `handleOpenSent` sends Event 19 straight to OpenConfirm; `handleOpenConfirm` sends Event 26 to Established.
- [ ] `internal/component/bgp/fsm/timer.go` - `Timers` owns the hold, keepalive and connect-retry timers, each with a generation guard.
- [ ] `internal/component/bgp/reactor/session_handlers.go` - `handleOpen` fires `EventBGPOpen` then `sendKeepalive`; `handleKeepalive` starts the keepalive and send-hold timers in OpenConfirm.
- [ ] `internal/component/bgp/reactor/session_connection.go` - `processOpen` is the collision-winner rail and repeats the same Event 19 and KEEPALIVE pair.
- [ ] `internal/component/bgp/reactor/peer_run.go` - the FSM callback calls `startBFDClient` on entry to Established and `stopBFDClient` on exit.

**Behavior to preserve:**
- A peer with `bfd` but no `strict` keeps today's behaviour exactly: session opens on Established, only Down tears down, no capability 74 in the OPEN.
- Teardown on BFD Down in Established keeps Cease subcode 10 and increments the ConnectRetryCounter (draft Section 8.7.2 agrees with today's `teardownAutomatic`).
- A missing BFD plugin still lets a non-strict BFD peer run (warn and continue).

**Behavior to change:**
- `strict` makes Ze advertise capability 74 (draft Section 6 MUST).
- When strict is negotiated and BFD is not Up, the OPEN rail withholds the KEEPALIVE and the FSM stays in OpenSent.
- A BFD Up event drives the FSM; today it is logged and dropped.
- BFD AdminDown in Established no longer tears the BGP session down (draft Section 8.7.1, RFC 5882 Section 4.2).
- A strict peer's BFD session starts before the FSM and outlives a transition to Idle (draft Section 7).

## Data Flow (MANDATORY)

### Entry Point
- Config: `bfd { strict true; hold-time 30; }` under the peer's `connection` container, in `internal/component/bgp/yang/ze-bgp-conf.yang`.
- Wire: capability code 74, length 0, inside the OPEN message's optional parameters, in both directions.
- Runtime: `api.StateChange` values on the BFD subscription channel.

### Transformation Path
1. File to Tree to `ResolveBGPTree()` to `PeersFromTree()`: `parseBFDSettings` fills `BFDSettings.Strict` and `BFDSettings.HoldTime`.
2. `parsePeerFromTree` appends a `capability.BFDStrictMode` to `PeerSettings.Capabilities` when strict is set and enabled, so `sendOpen` puts code 74 on the wire.
3. `capability.Negotiate` sets `Negotiated.BFDStrictMode` when both sides advertised 74.
4. `Peer.run` opens the BFD session before the first dial for a strict peer, and holds it for the peer's lifetime.
5. `runBFDSubscriber` stores the BFD state on the peer and delivers the draft's event (30 to 34) to the live session's FSM.
6. `Session.handleOpen` and `Session.processOpen` consult `BfdEnabled && BfdStrictNegotiated && bfd.SessionState` and either send the KEEPALIVE or enter `OpenSentBfdUpPending`.
7. `Session.handleKeepalive` in a pending sub-state moves to `OpenSentConfirmedBfdUpPending` rather than raising an FSM error.
8. `EventBfdUp` in a pending sub-state sends the withheld KEEPALIVE and advances to OpenConfirm or straight to Established.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config to Reactor | YANG `strict` and `hold-time` to `BFDSettings` via `PeersFromTree` | [ ] |
| Config to Wire | `BFDSettings.Strict` to `PeerSettings.Capabilities` to the OPEN optional parameters | [ ] |
| Wire to Negotiation | the peer's code 74 to `Negotiate` to `Negotiated.BFDStrictMode` | [ ] |
| Reactor to BFD plugin | `api.GetService().EnsureSession` opened before the FSM starts | [ ] |
| BFD to FSM | `api.StateChange` to `fsm.EventBfd*` to a sub-state transition | [ ] |

### Integration Points
- `capability.BFDStrictMode`, `capability.CodeBFDStrictMode` (`internal/core/bgp/capability/capability.go`).
- `capability.Negotiated.BFDStrictMode` (`internal/core/bgp/capability/negotiated.go`).
- `fsm.EventBfdAdminDown` to `fsm.EventBfdStrictConfigChanged`, `fsm.BfdSubState` (`internal/component/bgp/fsm/state.go`).
- `fsm.FSM.handleConnect`, `handleActive`, `handleOpenSent`, `handleOpenConfirm`, `handleEstablished` (`internal/component/bgp/fsm/fsm.go`).
- `fsm.Timers` BfdHoldTimer (`internal/component/bgp/fsm/timer.go`).
- `Session.handleOpen`, `Session.handleKeepalive` (`internal/component/bgp/reactor/session_handlers.go`).
- `Session.processOpen` (`internal/component/bgp/reactor/session_connection.go`).
- `Peer.startBFDClient`, `Peer.runBFDSubscriber` (`internal/component/bgp/reactor/peer_bfd.go`).
- `BFDSettings` (`internal/component/bgp/reactor/peer_settings.go`).

### Architectural Verification
- [ ] No bypassed layers (BFD reached only via `api.GetService()`)
- [ ] No unintended coupling (BGP reactor does not import the BFD engine package)
- [ ] No duplicated functionality (one sub-state field on the FSM, no second copy in the session)
- [ ] Zero-copy preserved: the capability writes two octets through `writeCapabilityTo`, no allocation
- [ ] Registration over hardcoding: the capability is a `Capability` implementation reached through `parseCapability`, and the sub-state is reported through the existing peer status

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The BFD session can be opened before the BGP FSM starts | `EnsureSession` is not gated on BGP state (`peer_bfd.go`); draft Section 7 asks for exactly this | strict mode needs a later start and a longer wait | read `EnsureSession` and `Peer.run` | unvalidated |
| A-2 | The KEEPALIVE that moves OpenSent to OpenConfirm is sent from exactly two places | `handleOpen` and `processOpen` both call `sendKeepalive` after `EventBGPOpen` | a third rail establishes without the gate | grep the `sendKeepalive` callers | unvalidated |
| A-3 | Ze never enters the Connect or Active DelayOpen sub-states | the `fsm.go` header records DelayOpen as not implemented | the two sub-states are reachable and owed | read the FSM header and the Event 20 arms | unvalidated |
| A-4 | FRR implements the draft well enough to negotiate code 74 | draft Appendix A lists Junos, IOS-XR and Nokia; FRR carries `neighbor ... bfd check-control-plane-failure` and strict mode since 8.x | the interop scenario needs another peer daemon | run the scenario against the FRR image | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The BGP hold timer expires while the session waits for BFD Up | strict peer flaps at the hold time | draft Section 10 warns about it; Ze logs the close and the guide states the timer relation |
| R-2 | Strict peer with the BFD plugin absent never comes up | peer stuck in OpenSent | strict plus no BFD plugin is a config error and a doctor failure, not a silent continue |
| R-3 | The sub-state and the BFD state disagree after a reconnection | peer stuck in a pending sub-state with BFD Up | the sub-state lives on the per-connection FSM, which is rebuilt each cycle, and the OPEN rail reads the live BFD state rather than a cached event |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `set protocols bgp neighbor X connection bfd strict true` | → | `parseBFDSettings` sets `Strict`; `PeerSettings.Capabilities` carries code 74 | `test/plugin/bgp-bfd-strict.ci` |
| the peer's OPEN carries code 74 | → | `Negotiate` sets `Negotiated.BFDStrictMode` | `internal/core/bgp/capability/capability_test.go` |
| BFD not Up when the peer's OPEN arrives | → | `handleOpen` withholds the KEEPALIVE, the FSM enters `OpenSentBfdUpPending` | `internal/component/bgp/reactor/session_bfd_strict_test.go` |
| BFD reaches Up | → | `EventBfdUp` sends the KEEPALIVE and advances the FSM | `internal/component/bgp/fsm/bfd_strict_test.go` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | peer has `bfd { strict true; }` | the OPEN carries capability code 74 with length 0 (draft Section 5, Section 6) |
| AC-2 | the peer's OPEN does not carry code 74 | `Negotiated.BFDStrictMode` is false and the session establishes on the unmodified RFC 4271 rail |
| AC-3 | strict negotiated, BFD not Up, the peer's OPEN arrives | no KEEPALIVE is sent, the KeepaliveTimer does not start, the FSM stays in OpenSent, sub-state `OpenSentBfdUpPending` (Section 8.5.5) |
| AC-4 | in `OpenSentBfdUpPending`, BFD reaches Up | a KEEPALIVE is sent, the BfdHoldTimer is reset to zero, the FSM changes to OpenConfirm (Section 8.5.1) |
| AC-5 | in `OpenSentBfdUpPending`, the peer's KEEPALIVE arrives first | the HoldTimer is reset to the negotiated value and the sub-state becomes `OpenSentConfirmedBfdUpPending`, still OpenSent (Section 8.5.6) |
| AC-6 | in `OpenSentConfirmedBfdUpPending`, BFD reaches Up | a KEEPALIVE is sent, the HoldTimer is reset and the FSM changes to Established (Section 8.5.1) |
| AC-7 | negotiated hold time zero and a pending sub-state is entered | the BfdHoldTimer is started with `BfdHoldTime`; on expiry the session sends Cease (6) and BFD Down (10), goes to Idle and increments the ConnectRetryCounter (Section 8.5.3) |
| AC-8 | BfdDown in OpenSent or OpenConfirm with strict negotiated | Cease (6) and BFD Down (10), Idle, ConnectRetryCounter set to ZERO (Sections 8.5.2, 8.6.2) |
| AC-9 | BfdDown in Established | Cease (6) and BFD Down (10), Idle, ConnectRetryCounter incremented (Section 8.7.2), unchanged from today |
| AC-10 | BfdAdminDown, BfdUp or BfdDisabled in Established | ignored; the session stays Established (Section 8.7.1) |
| AC-11 | BfdDown in Connect or Active | ignored (Sections 8.3.2, 8.4.2) |
| AC-12 | BfdAdminDown, BfdDisabled or BfdUp in Connect, Active or OpenConfirm outside a pending sub-state | ignored; the state does not change (Sections 8.3.1, 8.4.1, 8.6.1) |
| AC-13 | BfdStrictConfigChanged before Established | the session drops to Idle; in OpenSent and OpenConfirm it first sends Cease (6) and Other Configuration Change (6). Ignored in Established (Sections 8.3.4, 8.4.4, 8.5.4, 8.6.3, 8.7.3) |
| AC-14 | a strict peer starts | the BFD session is opened before the BGP FSM starts and is not destroyed when the connection goes to Idle (Section 7) |
| AC-15 | `strict true` with the BFD plugin not loaded | config validation and the doctor check surface an error; the peer is not silently run without BFD |
| AC-16 | no `strict` leaf | today's failure-detector behaviour is unchanged and no capability 74 is advertised |
| AC-17 | `hold-down <ms>` configured, BFD reaches Up | the session is NOT released until the interval has elapsed with BFD Up (Section 10); a zero or absent leaf releases on the first Up |
| AC-18 | strict negotiated, BFD never comes Up, negotiated BGP hold time non-zero | the RFC 4271 HoldTimer ends the wait: NOTIFICATION code 4 (Hold Timer Expired) and Idle. The pending sub-state is never a hang |
| AC-19 | OSPF and a strict BGP peer name the same neighbor, or a pinned `single-hop-session` and a strict peer do | all of them reach ONE `api.Key` and share ONE BFD session, whichever of the interface and the local address each client left out (RFC 5882 Section 4.4). An under-specified request the link table cannot resolve keeps the identity its client gave it |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | enables `bfd strict` on a peer whose forwarding path is broken | config to capability 74 in OPEN to BFD never Up to BGP held in OpenSent | `test/plugin/bgp-bfd-strict.ci` |
| 2 | the forwarding path recovers | subscriber Up to `EventBfdUp` to KEEPALIVE to Established | `test/plugin/bgp-bfd-strict.ci` |
| 3 | peers a strict Ze against FRR with BFD strict mode | both OPENs carry code 74 and both hold until BFD is Up | `test/interop/scenarios/bgp-bfd-strict-frr` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestBFDStrictModeCapabilityEncoding` | `internal/core/bgp/capability/capability_test.go` | code 74, length 0, two octets on the wire (AC-1) | |
| `TestBFDStrictModeCapabilityParse` | `internal/core/bgp/capability/capability_test.go` | a non-zero length TLV is refused (AC-1) | |
| `TestNegotiateBFDStrictMode` | `internal/core/bgp/capability/negotiated_test.go` | both sides set it true, one side sets it false (AC-1, AC-2) | |
| `TestBFDStrictOpenSentHoldsUntilUp` | `internal/component/bgp/fsm/bfd_strict_test.go` | the pending sub-state holds OpenSent, BfdUp advances (AC-3, AC-4) | |
| `TestBFDStrictKeepaliveBeforeUp` | `internal/component/bgp/fsm/bfd_strict_test.go` | Event 26 in the pending sub-state confirms rather than errors (AC-5, AC-6) | |
| `TestBFDStrictHoldTimerExpires` | `internal/component/bgp/fsm/bfd_strict_test.go` | Event 34 goes to Idle and increments the counter (AC-7) | |
| `TestBFDStrictDownConnectRetryCounter` | `internal/component/bgp/fsm/bfd_strict_test.go` | Event 31 zeroes the counter in OpenSent and OpenConfirm and increments it in Established (AC-8, AC-9) | |
| `TestBFDStrictIgnoredEvents` | `internal/component/bgp/fsm/bfd_strict_test.go` | Events 30 to 35 ignored where the draft says so (AC-10, AC-11, AC-12) | |
| `TestBFDStrictConfigChangedResetsToIdle` | `internal/component/bgp/fsm/bfd_strict_test.go` | Event 35 before Established, ignored in Established (AC-13) | |
| `TestSessionBFDStrictWithholdsKeepalive` | `internal/component/bgp/reactor/session_bfd_strict_test.go` | `handleOpen` sends no KEEPALIVE while BFD is down (AC-3) | |
| `TestBFDSettingsStrictParse` | `internal/component/bgp/reactor/config_test.go` | YANG `strict` and `hold-time` reach `BFDSettings` and the capability list (AC-1, AC-16) | |
| `TestBFDClientAdminDownDoesNotTeardown` | `internal/component/bgp/reactor/peer_bfd_test.go` | AdminDown leaves the session alone (AC-10) | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `hold-time` | 1-65535 seconds, default 30 | 65535 | 0 | 65536 |
| capability 74 length | 0 octets only | 0 | N/A | 1 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `bgp-bfd-strict` | `test/plugin/bgp-bfd-strict.ci` | a strict peer advertises 74 and is held out of Established until BFD is Up | |
| `bgp-bfd-strict-pinned` | `test/plugin/bgp-bfd-strict-pinned.ci` | a strict peer joins a BFD session a pinned `single-hop-session` already created, and reads its state from the Subscribe snapshot | PASS 2026-09-09, RED with the snapshot removed |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `bgp-bfd-strict-frr` | `test/interop/scenarios/bgp-bfd-strict-frr` | FRR 10.3.1 bgpd, no BFD | the NEGATIVE half: against a peer that advertises no capability 74 and answers no BFD, Ze establishes normally. Reddens when the negotiation half of the condition is dropped | PASS 2026-09-08 |
| `bgp-bfd-strict-speaker` | `test/interop/scenarios/bgp-bfd-strict-speaker` | the lab's wire-level speaker, extended to advertise code 74 and answer BFD from RFC 5880 | the POSITIVE half: with strict negotiated on both sides Ze sends no KEEPALIVE while BFD is down and sends one once it is Up. Reddens when the hold is disabled | PASS 2026-09-08 |
| `bgp-bfd-strict-preup-speaker` | `test/interop/scenarios/bgp-bfd-strict-preup-speaker` | the same speaker | the SHARED-SESSION path: a pinned `single-hop-session` and a strict peer to one neighbor establish over ONE session. It does NOT discriminate the Subscribe snapshot, which was measured rather than assumed | PASS 2026-09-09 |
| `bgp-bfd-strict-reload-speaker` | `test/interop/scenarios/bgp-bfd-strict-reload-speaker` | the same speaker | the PRE-UP path: a SIGHUP reload adds the strict peer to a BFD session that is already Up, and `show bfd sessions` reports ONE session at refcount 2 (RFC 5882 Section 4.4). Reddens when the Subscribe snapshot is removed: the peer never establishes | PASS 2026-09-09, RED with the snapshot removed |

### Future (if deferring any tests)
- None planned.

## Files to Modify
- `internal/core/bgp/capability/capability.go` - `CodeBFDStrictMode`, the `BFDStrictMode` type, the `parseCapability` arm, the `String` arm
- `internal/core/bgp/capability/negotiated.go` - the `BFDStrictMode` negotiated field and its mismatch row
- `internal/component/bgp/fsm/state.go` - Events 30 to 35 and the `BfdSubState` type
- `internal/component/bgp/fsm/fsm.go` - the six state handlers the draft revises
- `internal/component/bgp/fsm/timer.go` - the BfdHoldTimer
- `internal/component/bgp/reactor/peer_settings.go` - `BFDSettings.Strict`, `BFDSettings.HoldTime`
- `internal/component/bgp/reactor/config.go` - parse the two leaves and advertise the capability
- `internal/component/bgp/reactor/session_handlers.go` - the Section 8.5.5 and 8.5.6 rails
- `internal/component/bgp/reactor/session_connection.go` - the same gate on the collision-winner rail
- `internal/component/bgp/reactor/session.go` - the BFD state getter the OPEN rail reads
- `internal/component/bgp/reactor/peer_bfd.go` - early start, event delivery, the AdminDown correction
- `internal/component/bgp/reactor/peer_run.go` - the strict peer's BFD lifetime
- `internal/component/bgp/yang/ze-bgp-conf.yang` - the `strict` and `hold-time` leaves
- `docs/architecture/wire/capabilities.md` - the code table and a section for code 74
- `docs/architecture/behavior/fsm.md` - the six events and the two sub-states
- `internal/component/bfd/bfd.go` - every client's request goes through `Canonical`, in `EnsureSession` and `applyPinned`
- `internal/le/interoplab/bgp/prepare.go` - a scenario carrying `ze-reload.conf` starts ze from a writable copy so a SIGHUP can change it
- `docs/architecture/bfd.md`, `docs/guide/bfd.md` - the strict-mode behaviour, its configuration, and the one-session-per-neighbor rule
- `docs/guide/configuration.md`, `docs/features.md` - the operator-facing feature

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new config) | [ ] yes | the peer `bfd` container in `internal/component/bgp/yang/ze-bgp-conf.yang` |
| YANG validation constraints | [ ] yes | `strict` boolean default false; `hold-time` uint16 range 1..65535 default 30 |
| CLI grammar | [ ] yes | derived from the YANG leaves; `ai/rules/cli.md` |
| Doctor check for runtime dependencies | [ ] yes | strict requires the BFD plugin loaded |
| Functional test for new behaviour | [ ] yes | `test/plugin/bgp-bfd-strict.ci` |
| Interop scenario | [ ] yes | `test/interop/scenarios/bgp-bfd-strict-frr` |
| RFC summary and enrolment | [ ] yes | `rfc/short/draft-ietf-idr-bgp-bfd-strict-mode.md` plus `./le rfc index-update` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] yes | `docs/features.md` |
| 2 | Config syntax changed? | [ ] yes | `docs/guide/configuration.md`, `docs/guide/bfd.md` |
| 3 | Wire format changed? | [ ] yes | `docs/architecture/wire/capabilities.md` |
| 4 | FSM behaviour changed? | [ ] yes | `docs/architecture/behavior/fsm.md` |
| 11 | Affects daemon comparison? | [ ] yes | `docs/comparison.md` |

## Files to Create
- `internal/component/bgp/fsm/bfd_strict_test.go` - the FSM unit tests
- `internal/component/bgp/reactor/session_bfd_strict_test.go` - the session gate tests
- `test/plugin/bgp-bfd-strict.ci` - the functional test
- `test/interop/scenarios/bgp-bfd-strict-frr/` - the negative interop scenario
- `test/interop/scenarios/bgp-bfd-strict-speaker/` - the positive interop scenario
- `internal/le/interoplab/bgp/speaker_bfd.go` - the speaker's BFD responder and strict-mode oracle
- `rfc/short/draft-ietf-idr-bgp-bfd-strict-mode.md` - the extracted summary
- `internal/component/bfd/api/session_identity.go` - `SessionRequest.Canonical`, the RFC 5882 Section 4.4 key reduction
- `internal/component/bfd/session_identity.go` - the link table Canonical derives from
- `test/interop/scenarios/bgp-bfd-strict-preup-speaker/` - the shared-session scenario
- `test/interop/scenarios/bgp-bfd-strict-reload-speaker/` - the pre-up scenario, reached by SIGHUP reload

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation Phases below |

### Implementation Phases
1. **Phase: Capability (MANDATORY FIRST)** - code 74, the type, the parse arm, the negotiated field, the YANG leaves, `BFDSettings.Strict` and `HoldTime`, and the advertisement.
   - Tests: `TestBFDStrictModeCapabilityEncoding`, `TestNegotiateBFDStrictMode`, `TestBFDSettingsStrictParse`
2. **Phase: FSM** - Events 30 to 35, the two sub-states, the six revised handlers, the BfdHoldTimer.
   - Tests: `internal/component/bgp/fsm/bfd_strict_test.go`
3. **Phase: Session gate** - the OPEN rails withhold the KEEPALIVE, the KEEPALIVE rail confirms, and BFD events reach the live session.
   - Tests: `TestSessionBFDStrictWithholdsKeepalive`
4. **Phase: BFD lifetime and doctor** - early start, survival across Idle, the AdminDown correction, the missing-plugin error.
5. **Functional and interop tests**, including the revert-to-RED discrimination walk.
6. **RFC summary, extraction sign-off and discrimination records.**
7. **Full verification** with `./le verify current mode full`
8. **Complete spec** with the audit, the learned summary and the two-commit closure.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N implemented, with the file and the symbol named |
| Correctness | Every draft clause quoted above the code that enforces it; the ConnectRetryCounter direction matches the draft state by state |
| Data flow | BFD reached only via `api.GetService()` |
| Doctor checks | strict plus a missing BFD plugin is flagged |
| Registration over hardcoding | one sub-state field, no second copy in the session |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| capability 74 | `go test ./internal/core/bgp/capability -run BFDStrict` |
| FSM sub-states | `go test ./internal/component/bgp/fsm -run BFDStrict` |
| session gate | `go test ./internal/component/bgp/reactor -run BFDStrict` |
| interop | `bgp-bfd-strict-frr` passes against FRR bgpd and bfdd |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | the `hold-time` range is enforced; capability 74 length must be zero |
| Denial of service | draft Section 12: a blocked BFD session now blocks BGP, so the guide states that BFD authentication is RECOMMENDED |
| Resource exhaustion | a held-down strict peer leaks no BFD session, no timer and no goroutine |

## Mistake Log
### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| BFD strict is local policy with no capability | the draft defines capability 74 and negotiates it | owner decision, 2026-09-08 | the whole design was rewritten |

## Design Insights
<!-- LIVE -->

## Implementation Summary
### What Was Implemented

| Half | Where |
|------|-------|
| Capability 74 | `capability.BFDStrictMode`, `CodeBFDStrictMode`, the `parseZeroLengthCapability` arm (`internal/core/bgp/capability/capability.go`); `Negotiated.BFDStrictMode` and its `Mismatch` row (`negotiated.go`); `SessionCaps.BFDStrictMode` (`session.go`) |
| Advertisement | `parsePeerFromTree` appends the capability when `bfd { strict true }` is enabled (`internal/component/bgp/reactor/config.go`) |
| Config | `strict` and `hold-time` leaves (`internal/component/bgp/yang/ze-bgp-conf.yang`); `BFDSettings.Strict`, `BFDSettings.HoldTime` (`peer_settings.go`); range enforced in `parseBFDSettings` |
| FSM | Events 30 to 35 and `BfdSubState` (`fsm/state.go`); arms in all six state handlers, the sub-state clear and the BfdHoldTimer stop in `change` (`fsm/fsm.go`); the BfdHoldTimer (`fsm/timer.go`) |
| Wire | `advanceAfterOpen`, `handleBFDEvent`, `bfdTeardown`, `bfdStrictHolds`, `raiseBFDStrictConfigChanged`, `applyBFDStrictNegotiation` (`reactor/session_bfd_strict.go`); both OPEN rails call `advanceAfterOpen` |
| BFD lifetime | `startBFDClient` before the start event and `stopBFDClient` from `cleanup` for a strict peer (`reactor/peer_run.go`); the subscriber maps a BFD state onto an FSM event (`reactor/peer_bfd.go`) |
| Reload | `bfdStrictConfigChanged` gates Event 35 on the restart path (`reactor/reactor_api.go`) |
| One session per neighbor | `api.SessionRequest.Canonical` (`internal/component/bfd/api/session_identity.go`) reduces the five fields of `api.Key` to one form, deriving a single-hop request's interface and local address from the link the peer is on; `connectedLinks` reads that link table from the interface component (`internal/component/bfd/session_identity.go`); `pluginService.EnsureSession` and `applyPinned` both pass through it (`internal/component/bfd/bfd.go`) |
| Conformance | `rfc/short/draft-ietf-idr-bgp-bfd-strict-mode.md` (enrolled), `rfc/extraction/` sign-off over 4 sites and 53 sections, 8 tags and 8 verified discrimination records; RFC5882-4.4-1 gains four tagged units and four records (`rfc/discrimination/rfc5882.json`) |

### Behavior corrected on the way

BFD AdminDown no longer tears down an Established BGP session. Draft Section
8.7.1 ignores it, and RFC 5882 Section 4.2 says a client "SHOULD NOT take any
control protocol action" on an AdminDown transition, because Section 3.2 makes
AdminDown say nothing about the data path. `TestBFDClientAdminDownDoesNotTeardown`
holds it, and `TestBFDClient_TeardownOnDown` was rewritten to assert the wire
rather than a queued teardown, which under Section 8.2 is now a teardown of
nothing.

### RFC 5882 Section 4.4, added on the owner's answer of 2026-09-09

The clients did not build a key the same way, so OSPF and a strict BGP peer to
one neighbor opened two BFD sessions and put two packet streams on one link.
The owner's answer was to make the builders agree, so the reduction lives in
`api.SessionRequest.Canonical`, next to `Key` where identity is declared, and
every client reaches it through `pluginService.EnsureSession` or `applyPinned`.

The ambiguous case is refused rather than guessed: no matching link, or more
than one, leaves the request as its client wrote it and gives it its own
session. An IPv6 link-local peer is the standing example, because every link
carries `fe80::/64`. A daemon with no interface backend loaded derives nothing,
so every key stays as written, which is the behavior that existed before.

## Review Gate

<!-- BLOCKING (ai/rules/planning.md Review Gate). Filled by /ze-implement's /ze-review gate: -->
<!-- the final review before closure, run AFTER the inline critical/security/doc reviews, over the complete diff. -->
<!-- Every BLOCKER and ISSUE (severity > NOTE) must be fixed, then re-run /ze-review. -->
<!-- Loop until the review returns 0 BLOCKER/0 ISSUE (only NOTEs, or nothing). Paste the final clean run. -->
<!-- NOTE-only findings do not block — record them and proceed. -->

### Round 1 scope (fixed before the round ran, 2026-09-08)

The WHOLE diff of this spec, with at least two lenses. In scope:
`internal/core/bgp/capability/` (`CodeBFDStrictMode`, `BFDStrictMode`,
`parseZeroLengthCapability`, `Negotiate`), `internal/component/bgp/fsm/`
(`fsm.go` BFD arms and sub-states, `timer.go` `BfdHoldTime`, `BfdHoldDown` and
their timers), `internal/component/bgp/reactor/session_bfd_strict.go`,
`peer_bfd.go`, the `BFDSettings` fields in `peer_settings.go`,
`parseBFDSettings` and `parsePeerFromTree` in `config.go`, the `strict`,
`hold-time` and `hold-down` leaves in `internal/component/bgp/yang/ze-bgp-conf.yang`,
`internal/le/interoplab/bgp/speaker.go` and `speaker_bfd.go`,
`test/plugin/bgp-bfd-strict.ci` with its fixture,
`test/interop/scenarios/bgp-bfd-strict-frr/` and `bgp-bfd-strict-speaker/` with
their checker registrations, `rfc/short/draft-ietf-idr-bgp-bfd-strict-mode.md`,
`rfc/discrimination/draft-ietf-idr-bgp-bfd-strict-mode.json`, and the doc pages
this work edited.

Out of scope, because they belong to other sessions in flight in this shared
checkout: the as-notation work (`internal/core/bgp/asn/`, `applyASNotation`, the
ASN renderers), the update-delay work (`update_delay.go`, `UpdateDelayStatus`),
`internal/component/config/transaction/`, `internal/component/iface/`,
`internal/component/sysrib/`, and the `plugin/yang` rename.

Always-in-scope classes apply anywhere they are found, per `ai/rules/planning.md`.

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | [what /ze-review reported] | file:line | fixed in <commit/line> / deferred (id) / acknowledged |

### Fixes applied
- [short bullet per BLOCKER/ISSUE, naming the file and change]


### Round 2 scope (fixed before the round ran, 2026-09-08)

ONLY the fixes round 1 produced, plus the sibling call sites they touched:
`Session.raiseBFDHoldTimerExpired` with its `OnBfdHoldTimerExpires`
registration in `applyBFDStrictNegotiation`, the new sub-state gate at the top
of `handleOpen` (`session_handlers.go`), the hold-down move onto the KEEPALIVE
rail (`handleKeepalive`'s OpenConfirm branch and the
`OpenSentConfirmedBfdUpPending` branch of `handleBFDEvent`) with
`bfdStateReader`'s entry time, `bfdClient.since` and
`StartBfdHoldDownTimerFor`, `Peer.bfdSubState` through `PeerInfo.BFDSubState`
to the `bfd-sub-state` field of `show bgp peer`, the §10 log line moved into
the `OnHoldTimerExpires` closure in `NewSession`, `bfd_strict_doctor.go` with
its registration and `warnStrictPeersWithoutEngine`, `hold-down` in the `.ci`
fixture and both interop `ze.conf` files, `bfdStrictConfigChanged` counting
`HoldDown`, `StopBfdHoldDownTimer` on the advancing branch, the Event 33
explanation in `state.go`, and the `starting` flag in `startBFDClient`.

Plus the eight always-in-scope classes, which apply anywhere they are found.

Not in scope: anything round 1 cleared that these fixes did not touch,
including the capability encode/decode, the discrimination records, and the
speaker responder's RFC 5880/5881 conformance.


### Round 3 scope (fixed before the round ran, 2026-09-09)

ONLY the fixes round 2 produced, plus the sibling call sites they touched:
`handle.Subscribe`'s current-state snapshot with `Loop.snapshot` and the new
`api.StateChange.Initial` field, `startBFDClient`'s consumption of that snapshot
in place of the unconditional `StateDown` seed, the three subscriber callers
updated for `Initial` (BGP `runBFDSubscriber`, OSPF, static),
`test/plugin/bgp-bfd-strict-pinned.ci` and its fixture, the `bfd-sub-state`
field now on `handleBgpPeerDetail` as well as `handleBgpPeerList` with the three
re-pointed claims, `bfdHoldDownPending` measuring on the injected clock with the
test helper's seeding, `validateBFDHoldDown`, the reordered §10 log arms, and
`TestSessionBFDStrictHoldDownRestartsOnAFlapInsideTheInterval`.

Plus the eight always-in-scope classes, which apply anywhere they are found.

Not in scope: anything rounds 1 and 2 cleared that these fixes did not touch,
including Event 34's wiring, the `handleOpen` gate placement, the `starting`
flag and the capability encode/decode.


### Round 4 scope (fixed before the round ran, 2026-09-09)

ONLY the fixes round 3 produced, plus the sibling call sites they touched:
`Loop.subscribe` holding `l.mu` then `subsMu` across the snapshot and the
append with `handle.Subscribe` delegating to it, `StateChange.IsTransition`
beside the `Initial` field and the three callers' use of the affirmative form,
the contract moved onto `api.SessionHandle.Subscribe`, `sessionEntry.lastChange`
and `lastDiag` set in `makeNotify` with the snapshot carrying both, the four
tests in `subscribe_snapshot_test.go` including
`TestSubscribeHoldsBothLocksAcrossTheRegistration`,
`TestBFDClientReStampsTheEntryTimeOnlyOnAChange`, the corrected
`docs/architecture/bfd.md` lock-order section, and the new
`bgp-bfd-strict-preup-speaker` interop scenario.

Plus the eight always-in-scope classes, which apply anywhere they are found.

Not in scope: anything rounds 1 to 3 cleared that these fixes did not touch,
including Event 34's wiring, the `handleOpen` gate, the hold-down rails and the
starvation validator.


### Round 5 scope (fixed before the round ran, 2026-09-09)

ONLY the fixes round 4 produced, plus the sibling call sites they touched:
the matching `local` leaf in `test/interop/scenarios/bgp-bfd-strict-preup-speaker/ze.conf`
and in `internal/test/fixture/plugin_fixture_03_bgp_bfd_strict.go`, the rewritten
claims in that scenario and in `test/plugin/bgp-bfd-strict-pinned.ci` recording
that neither discriminates the snapshot, `TestPinnedSessionAndStrictPeerShareOneKey`,
`TestSubscribeReportsAStateAnotherClientAlreadyReached` as the snapshot's
discriminating proof, the REMOVAL of `StateChange.IsTransition` with all three
callers reading the field and the reconciled `Subscribe` contract naming
`static` as the worked exception, `bfdChangeTime` unifying the entry-time clock,
the false-red note on the 100ms sleep, and the journal row at
`plan/journal/absent-value-is-a-distinct-identity.md`.

Plus the eight always-in-scope classes, which apply anywhere they are found.

Not in scope: anything rounds 1 to 4 cleared that these fixes did not touch,
including the lock fix and its invariant test.


### Round 6 scope (owner-authorised, fixed before the round ran, 2026-09-09)

Thomas authorised this round on 2026-09-09 and chose "make the two builders
agree on the key" for the RFC 5882 Section 4.4 gap.

ONLY the fixes round 5 produced, plus the sibling call sites they touched:
`SessionRequest.Canonical` in the new
`internal/component/bfd/api/session_identity.go` with `api.DefaultVRF` declared
once in `events.go` and the component's `defaultVRFName` removed,
`connectedLinks` in `internal/component/bfd/session_identity.go`, both funnels
(`pluginService.EnsureSession` and `applyPinned`) passing through it, the four
`RFC5882-4.4-1` tagged units with their records in `rfc/discrimination/rfc5882.json`,
the new `test/interop/scenarios/bgp-bfd-strict-reload-speaker/` with the lab's
new writable-config route in `internal/le/interoplab/bgp/prepare.go`, the
corrected `test/plugin/bgp-bfd-strict-pinned.ci` comment, the repointed and
rewritten journal row, the two replaced fabricated Section 4.4 quotes, and the
`docs/architecture/bfd.md` section.

Plus the eight always-in-scope classes, which apply anywhere they are found.

Not in scope: anything rounds 1 to 5 cleared that these fixes did not touch.

### Run 2+ (re-runs until clean)
<!-- Add a new block per re-run. Final run MUST show zero BLOCKER/ISSUE. -->
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-16 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and passing test
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `./le verify worktree` passes
- [ ] Feature code integrated (`internal/*`)
- [ ] Documentation Update Checklist answered

### Quality Gates (SHOULD pass)
- [ ] Draft constraint comments added above every enforced MUST and MUST NOT
- [ ] Implementation Audit complete

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features
