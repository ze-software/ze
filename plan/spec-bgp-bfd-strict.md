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
- [ ] `rfc/short/rfc5880.md` - the BFD base protocol. Section 6.8.6 is the one this work touches: a packet whose Your Discriminator is zero is matched to a session "based on some combination of other fields", and the combination is the application's to choose.
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
| `bfd-first-packet-pktinfo` | `test/bfd/bfd-first-packet-pktinfo.ci` (`option=needs-linux`) | a Control packet with Your Discriminator 0 selects the session its destination address belongs to, proving the kernel's IP_PKTINFO reaches `firstPacketKey` | PASS 2026-09-11 in the QEMU guest (Alpine 3.21 aarch64, HVF); RED there with `readLoop` stamping the bind address again |
| `bfd-first-packet-pktinfo-v6` | `test/bfd/bfd-first-packet-pktinfo-v6.ci` (`option=needs-linux`) | the same over IPv6, where IPV6_PKTINFO puts the 16-byte address before the ifindex rather than after it | PASS 2026-09-11 in the QEMU guest; RED there with `IPV6_RECVPKTINFO` not enabled |

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
| A-4: FRR 10.3.1 implements the draft well enough to negotiate code 74 | it does not advertise capability 74 at all | running the scenario against the FRR image | the FRR scenario proves the NEGATIVE half only; the POSITIVE half needed the lab's own speaker extended to advertise 74 and answer BFD (`internal/le/interoplab/bgp/speaker_bfd.go`), which is one of the four scenarios this spec added |
| `Subscribe` reports the BFD session's state | it reported only TRANSITIONS, so a client joining a session another client had already brought Up read Down for ever | round 2 | `api.StateChange.Initial` and `Loop.snapshot`; the defect had reached the `bgp-bfd-strict-reload-speaker` scenario, where the peer never established |
| The first-packet lookup needed one field repaired | it needed a RULE. Four repairs, each correct in the half it examined | rounds 8, 10, 11, 12 | the rule is now stated once above `firstPacketKey` and derived from `optionalKeyFields`; `plan/learned/019-state-the-rule-not-the-field.md` |

### Kinds
| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-4: FRR would negotiate the draft | FRR 10.3.1 advertises no capability 74 | the scenario ran | the lab speaker carries the positive half; the FRR scenario keeps the negative half, which is worth having on its own |
| approach | the strict gate was first put on the OPEN rail, and the Section 10 hold-down with it | Section 10 gates the transition to ESTABLISHED, so gating the OPEN let the common case skip the interval entirely | round 2 | `bfdHoldDownPending` measured from the BFD session's own entry time, on the transition the draft names |
| escalation | one lookup repaired four times, field by field | the four were one sentence applied to four fields | round 12's reviewer predicted the fifth, and Thomas chose the general form | the rule, its derived relaxation walk, and the lesson routed to `plan/learned/019-state-the-rule-not-the-field.md` plus two journal classes |

## Design Insights
<!-- LIVE -->

### Deviations from Plan

- The wire half lives in a new `internal/component/bgp/reactor/session_bfd_strict.go`
  rather than being spread across `session_handlers.go` and
  `session_connection.go` as planned. Both OPEN rails call `advanceAfterOpen`, so
  the draft's clause table reads as a table in one file and no third rail can be
  added that bypasses the gate.
- `hold-down <ms>` was added to the config surface, which the plan did not carry.
  Draft Section 10 asks for it and nothing else in ze reads it, so a reload that
  changed only the damping interval would otherwise restart the peer with no
  Event 35 and no NOTIFICATION saying why.
- The transport (`internal/component/bfd/transport/udp*.go`) was changed, which
  the plan did not anticipate. The strict gate was the first caller to depend on
  a first packet reaching its session, and it walked into a defect where every
  received packet carried the wildcard bind address as its local address.

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
| First-packet demux | `(*UDP).readLoop` takes the destination address and the ingress interface from `IP_PKTINFO` rather than from the wildcard bind address, so the RFC 5880 Section 6.8.6 index can match a real packet (`internal/component/bfd/transport/udp.go`, `udp_linux.go`) |
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

The ambiguous case is refused rather than guessed, and a refusal gives the
under-specified request its own session. `Canonical` can therefore SEPARATE two
sessions that should be one; it cannot MERGE two that should be separate,
because `Mode` stays in the key and two requests converge only on the same
(peer, local, VRF), which is the pair RFC 5883 Section 5 demultiplexes on.

So the Section 4.4 MUST is met except in these configurations. The consequence
is NOT uniform, and the single-hop rows are worse than a sharing gap: a
single-hop session whose key carries no interface is still selected for a first
packet, by the interface-less lookup in `Loop.handleInbound`, but the two
clients hold two sessions where the RFC asks for one. Before round 12 those
sessions could not be selected at all and NEITHER came up, which is how long a
silent mismatch survives when only one half of it is ever looked at.

| Configuration | Why it refuses |
|---------------|----------------|
| Single-hop, an IPv6 link-local peer, and one client names no interface | Every link carries `fe80::/64`, so several links match and only the client knows which it meant |
| Single-hop, two links onto the same subnet, and one client names neither interface nor local address | Several links match |
| Single-hop, the peer on no connected prefix, and one client names no local address | No link matches, so nothing can be derived |
| Multi-hop, no route to the peer, and one client names no local address | `ifcomp.RouteLookup` cannot answer |
| Multi-hop, the egress interface carrying no address of the peer's family, or more than one | Nothing to take, or nothing that says which |
| Multi-hop in a non-default VRF where one client names no local address | `ifcomp.RouteLookup` reads the default routing table, so `topologyFor` does not call it there |
| Any mode, on a daemon with no interface backend loaded | The link table is empty, so every key stays as its client wrote it. This is NOT "the behavior that existed before": before the transport surfaced the ingress interface, such a key matched every packet by accident, and it is the interface-less lookup that deliberately restores it |

Each of them is closed by the operator naming the local address on both sides,
which is what the interop scenarios do. The first four would need a
configuration answer rather than a derivation, and the VRF one needs a
VRF-scoped route lookup the interface component does not offer.

The seven rows above are also published outside this spec, because this file is
removed at closure: `rfc/short/rfc5882.md`, under "Multiple Control Protocols
(Section 4.4)", carries the same table beside the requirement it bounds, and
`docs/architecture/bfd.md` points there.

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/bgp-bfd-strict-828bf96b-ed79-41e5-bc2d-b82d08d4124a.md` (31 files, verdict=clean) |
| `./le spec session review check` | OK, hashes match |
| Rounds | 15. Rounds 1 to 5 under the standard cap; rounds 6 to 14 each authorised by Thomas individually, each authorisation recorded in its own scope block below; round 15 is this closure's own review, run in an independent context. The product defect that earned the extra rounds is round 11's BLOCKER: a single-hop BFD session whose `api.Key` carries no interface could never be selected for a first packet by `Loop.handleInbound`, so under strict mode the BGP peer sat in OpenSent for the life of the daemon |
| Reviewer lenses used | closure ran two itself: (1) producers and fail-closed behavior over the uncommitted diff, reading `advanceAfterOpen`, `handleBFDEvent`, `bfdTeardown`, `writeMessageWithin`, `Canonical`, `parseReceivedPktinfo` and `buildKeyRelaxations`; (2) test discrimination, by breaking two producers and observing which tests red. Rounds 1 to 14 carried their own lenses in their own contexts |

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

### What each round found, and what fixed it

Fourteen rounds ran, each in its own context, each over the scope block above
it. Per-finding severity was recorded only where a round raised a BLOCKER; every
other round's findings are enumerated by the NEXT round's scope block, which
lists the fixes it was commissioned to review. This table is that enumeration
read forward.

| Round | Severity high-water | What it found | What fixed it |
|-------|---------------------|---------------|---------------|
| 1 | ISSUE | Event 34 reached no wire action, `handleOpen` gated too late, the §10 hold-down was measured on the wrong rail, no operator visibility of the sub-state, no doctor check for strict-without-engine | `raiseBFDHoldTimerExpired` with its `OnBfdHoldTimerExpires` registration, the sub-state gate at the top of `handleOpen`, the hold-down moved onto the KEEPALIVE rail, `PeerInfo.BFDSubState`, `bfd_strict_doctor.go` |
| 2 | ISSUE | a strict peer joining a BFD session another client had already brought Up read Down for ever, because `Subscribe` delivered only transitions | `Loop.snapshot` and `api.StateChange.Initial`, consumed by `startBFDClient`; `validateBFDHoldDown`; `bfdHoldDownPending` on the injected clock |
| 3 | ISSUE | `Loop.subscribe` registered outside the lock it snapshots under, and `StateChange.IsTransition` was a second way to ask one question | the two locks held across snapshot and append with `TestSubscribeHoldsBothLocksAcrossTheRegistration`; the contract moved onto `api.SessionHandle.Subscribe` |
| 4 | ISSUE | `IsTransition` removed, the scenario claims overstated what the Subscribe snapshot discriminates | the three callers read the field; the claims rewritten to what was MEASURED; `TestSubscribeReportsAStateAnotherClientAlreadyReached` as the snapshot's real proof |
| 5 | ISSUE | OSPF and a strict BGP peer to one neighbor built two keys, so RFC 5882 Section 4.4 was unmet | Thomas chose "make the two builders agree on the key" (2026-09-09): `api.SessionRequest.Canonical`, reached by `EnsureSession` and `applyPinned` |
| 6 | ISSUE | `loopDeviceFor` locked the SO_BINDTODEVICE name for every later single-hop session in a VRF, a regression this spec's own 4.4 fix introduced | `loopDeviceFor(req, normalized)` taking the device from `req.Interface`; `canonicalMultiHop`; `api.Link.VRF` and `vrfMembership` |
| 7 | ISSUE | the RFC aggregates understated the draft while the reactor half was parked | `config_bfd_strict_test.go` updated to `Canonical(api.Topology{...})`; `./le rfc index-update` |
| 8 | ISSUE | a received packet's LOCAL address came from the wildcard bind, so no first-packet key could ever match | `IP_PKTINFO` / `IPV6_RECVPKTINFO` and `parseReceivedPktinfo`; `test/bfd/bfd-first-packet-pktinfo.ci` |
| 9 | ISSUE | `oobBufLen` sat exactly at the IPv6 requirement with no margin; a runtime client could bind the shared socket; the interface-name cache never expired | `oobBufLen` 128 with the CMSG_SPACE arithmetic stated; `TestNoRuntimeClientBindsTheSharedSocket`; `ifNameTTL` |
| 10 | ISSUE | the ingress interface was stamped on multi-hop packets, which `Inbound.Interface`'s own contract forbids | `interfaceName` became `ingressInterface`, single-hop only, with `TestIngressInterfaceIsSingleHopOnly` |
| 11 | **BLOCKER** | a single-hop session whose key carries no interface could NEVER be selected for a first packet, so under strict mode the BGP peer sat in OpenSent for the life of the daemon | `Loop.handleInbound` retries with `iface` cleared; `TestFirstPacketSelectsASessionKeyedWithoutAnInterface` and `TestFirstPacketPrefersTheSessionThatNamedTheLink` |
| 12 | **BLOCKER** | that fix was the fourth field-by-field repair to one lookup, and the reviewer predicted a fifth dimension | Thomas chose the SHAPE (2026-09-11): state the match rule once as a property. `optionalKeyFields`, `keyRelaxations`, and the rule declared above `firstPacketKey` |
| 13 | ISSUE | the relaxed-match COST was unstated, and the popcount tie-break was left implicit in the `iota` | `TestFirstPacketRelaxedMatchCosts`; the tie-break stated above `keyRelaxations`; `UDP` taking an injected `Clock` |
| 14 | ISSUE (0 BLOCKER, 2 ISSUE, 4 NOTE) | the parity pair enforced field NAMES and not their VALUES, and the stated tie-break had no test | `TestFirstPacketIndexCarriesEveryFieldValue`; `TestFirstPacketTieBreakPrefersTheLink`; the four NOTEs taken in place |
| closure | ISSUE | `p.updateDelayPeerDown()` and its comment duplicated in `peer_run.go`, a bad merge against another session's update-delay work; `docs/architecture/bfd.md` and `rfc/short/rfc5882.md` both cited a spec commit B deletes | the duplicate removed; the seven-row table moved into `rfc/short/rfc5882.md` and both citations repointed at it |

Round 14's reviewer recommended no round 15: both its ISSUEs were additive
assertions in one test file and no product code moved. The closure round's two
findings are recorded above and fixed in this same commit.


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


### Round 14 scope (owner-authorised, fixed before the round ran, 2026-09-11)

Thomas authorised this round on 2026-09-11, SCOPED TO THE PARITY TEST ALONE, on
the round-13 reviewer's explicit recommendation. Everything else that round
raised is either a NOTE taken in place or the out-of-scope find he routed to a
journal row and its own spec.

In scope, and nothing else:

1. `internal/component/bfd/engine/first_packet_parity_test.go`.
   `TestFirstPacketKeyMirrorsEveryKeyField` reds when an `api.Key` field is added
   and not classified, when a classification names a field on neither struct, and
   when the index holds a field no key field maps to.
   `TestRelaxationsCoverEveryRelaxableField` DERIVES the relaxation set from the
   code by zeroing a populated key with `optionalKeyFields` and reading back
   which fields moved, so a relaxable claim with no bit, or a bit on a field
   never unset, reds.
2. The three round-13 notes taken in place: the popcount tie-break now stated
   rather than left implicit in the `iota`, the removal of the unreachable
   `drop &^ optionalKeyFields` guard with the comment that replaced it, and the
   corrected round attribution naming the rounds that FIXED rather than found.

Not in scope: the journal row at `plan/journal/declared-format-contradicts-payload.md`
and the spec at `plan/immediate/spec-bfd-link-local-peer-zone.md`, which are the
IPv6 link-local zone find, deliberately not fixed here. Nor anything rounds 1 to
12 cleared.

### Round 13 scope (owner-authorised, fixed before the round ran, 2026-09-11)

Thomas authorised this round on 2026-09-11 and chose the SHAPE of the fix it
reviews: state the match rule once rather than patch a third field. Round 12's
reviewer predicted that a field-by-field repair would leave a fourth dimension
for this round to find, and that a general rule would leave nothing.

In scope:

1. **The rule and its implementation as a property.** Declared above
   `firstPacketKey` (`internal/component/bfd/engine/engine.go`): a key field the
   session left UNSET does not participate in the match. `relaxLocal` and
   `relaxIface` name the optional fields, `optionalKeyFields` ORs them,
   `keyRelaxations` is every subset ordered most-specific first, and
   `Loop.handleInbound` walks that order.
2. **Its stated cost**: a session that left a field unset is matched by a packet
   carrying any value in that field; one such session alone takes the first
   packet from every link and every local address in its remaining tuple, and a
   more specific session wins only for the values it names.
3. `TestFirstPacketRelaxedMatchCosts`, a table varying what the SESSION leaves
   unset against a packet that always carries real values.
4. The `eth1` arm added to `TestFirstPacketPrefersTheSessionThatNamedTheLink`.
5. The corrected `pluginService` doc, both cost comments, and `UDP` taking an
   injected `Clock` with the TTL test driving a stepped clock.

Plus the eight always-in-scope classes, which apply anywhere they are found.

Not in scope: anything rounds 1 to 12 cleared that these fixes did not touch,
including `oobBufLen`, the kernel struct offsets, the fallback's inability to
cross VRF, mode or peer, and the rewritten multi-hop test arm.

### Round 12 scope (owner-authorised, fixed before the round ran, 2026-09-11)

Thomas authorised this round on 2026-09-11, SCOPED TO THE BLOCKER'S FIX, on the
round-11 reviewer's advice: this code path produced a defect in three
consecutive repairs, always in the half the repair was not examining, and this
fix carries a design choice with its own blind half. The other five findings'
fixes ride along; the reviewer judged they need no round of their own.

In scope:

1. **The blocker's fix and its design choice.** `Loop.handleInbound` retries the
   first-packet index with `iface` cleared after an exact match misses, so a
   single-hop session whose key carries no interface can be selected. The author
   chose the second lookup over refusing to create an unkeyable session, and
   states the cost: where two sessions share `(peer, local, vrf, mode)` and only
   one names a link, a packet from any other link reaches the one that named
   none rather than being dropped. The exact match runs first, so a session that
   named the link always wins for packets on it. Pinned by
   `TestFirstPacketSelectsASessionKeyedWithoutAnInterface` and
   `TestFirstPacketPrefersTheSessionThatNamedTheLink`.
2. **A test whose meaning changed.** `TestFirstPacketMultiHopIgnoresTheIngressInterface`'s
   second arm asserted a stamped multi-hop packet is DROPPED; the engine now
   finds the session, so the arm was rewritten to assert selection rather than
   deleted.
3. The corrected spec rows and `rfc/short/rfc5882.md`: the four single-hop rows
   used to match by accident, neither session could be selected at all before
   this fix, and the interface-less lookup deliberately restores it.
4. The rebind warning firing only when a device was actually requested.
5. `oobBufLen` raised to 128 with the corrected `CMSG_SPACE` arithmetic.
6. `ifNameTTL` expiry of cached interface names, with
   `TestIngressInterfaceReresolvesAStaleName`.
7. The IPv6 `.ci` using `ip -6 addr replace` and removing both addresses, proven
   by running both PKTINFO cases twice in one guest boot.

Plus the eight always-in-scope classes, which apply anywhere they are found.

Not in scope: anything rounds 1 to 11 cleared that these fixes did not touch.

### Round 11 scope (owner-authorised, fixed before the round ran, 2026-09-11)

Thomas authorised this round on 2026-09-11. It covers ONLY the round-10 fixes.

1. **The blocker's repair.** `interfaceName` became `ingressInterface` and
   returns empty unless `u.Mode == api.SingleHop`, re-establishing the invariant
   `Inbound.Interface`'s own doc had always claimed and round 8 silently removed.
   With `TestIngressInterfaceIsSingleHopOnly` at the transport, the corrected
   `TestFirstPacketMatchesWhatTheTransportSurfaces` now varying the interface
   independently of the key, and the new
   `TestFirstPacketMultiHopIgnoresTheIngressInterface`.
2. `ingressInterface` no longer writing a failed `net.InterfaceByIndex` into
   `ifNames`, with `TestIngressInterfaceDoesNotCacheAFailure`.
3. `runtimeState` recording each live loop's device and `loopFor` warning when an
   existing loop's binding differs from what this apply wants, naming both
   devices, the effect and the action. Plus `loopDeviceFor`'s corrected comment
   and its removed unused parameter.
4. `test/bfd/bfd-first-packet-pktinfo-v6.ci` and the shared fixture
   parameterised over both families, including the two test-shape findings the
   guest runs produced: `::2` cannot be bound, and `::1`-to-`::1` is not
   selected, so the case adds `fd00:5882::1/128` and `::2/128` to `lo`.
5. The corrected `oobBufLen` comments carrying the measured numbers, the docs
   table's multi-hop row, `Inbound.Interface`'s doc, and `rfc/short/rfc5882.md`
   now saying 4.4-1 is met where the clients reach one key and partially met
   otherwise, matching the `Partial` row `docs/features/rfc-status.md` publishes.

Plus the eight always-in-scope classes, which apply anywhere they are found.

Not in scope: anything rounds 1 to 9 cleared that these fixes did not touch,
including the seven-row refusal table and the kernel struct-offset reading.

### Round 9 scope (owner-authorised, fixed before the round ran, 2026-09-11)

Thomas authorised this round on 2026-09-11. It covers the round-8 fixes, which
are wider than the round that prompted them.

1. **The transport change, the widest-blast-radius edit in this spec.**
   `applySocketOptions` enabling `IP_PKTINFO` and `IPV6_RECVPKTINFO`,
   `parseReceivedPktinfo` reading the destination address and ingress ifindex out
   of the control message with the kernel struct offsets documented, and
   `(*UDP).readLoop` filling `Inbound.Local` and `Interface` and resolving the
   index through a cached `net.InterfaceByIndex`. This changes what every BFD
   session sees on every packet.
2. The non-Linux path returning an invalid address rather than the wildcard, with
   `udp_pktinfo_other_test.go` pinning it as a fail-closed guard.
3. `loopDeviceFor` returning the VRF or nothing, so no runtime client binds the
   shared socket; device binding stays with the pinned path where
   `resolveLoopDevices` sees every sharer first. Includes the REPLACEMENT of
   `TestLoopForKeepsTheFirstCallersDevice`'s round-6 meaning by
   `TestNoRuntimeClientBindsTheSharedSocket`, because the original was pinning
   the defect.
4. `test/bfd/bfd-first-packet-pktinfo.ci` with `internal/test/fixture/bfd_fixture_pktinfo.go`
   and its `register_bfd.go` entry, plus `TestFirstPacketMatchesWhatTheTransportSurfaces`
   and its `RFC5880-6.8.6-18` tag and discrimination record.
5. The reachable `soleAddressOn` VRF filter and its test, and the `vrfMembership`
   cycle and broken-chain coverage.
6. `docs/architecture/bfd.md`'s per-mode table and the corrected first-packet-key
   row, `connectedLinks` and `topologyFor`'s comments, and the spec's seven-row
   table of configurations where a refusal leaves two sessions and RFC 5882
   Section 4.4 is consequently unmet.

Plus the eight always-in-scope classes, which apply anywhere they are found.

Not in scope: anything rounds 1 to 7 cleared that these fixes did not touch.

### Round 7 scope (owner-authorised, fixed before the round ran, 2026-09-11)

Thomas authorised this round on 2026-09-11. It covers the three round-6 fixes,
which were made while this spec's reactor half was PARKED, plus what the restore
needed.

1. `loopDeviceFor(req, normalized)` in `internal/component/bfd/bfd.go`, taking
   the SO_BINDTODEVICE name from `req.Interface` and never the derived one. This
   repairs a regression THIS spec's own RFC 5882 fix introduced, where the first
   caller's loop locked the device for every later single-hop session in a VRF.
2. `canonicalMultiHop` in `internal/component/bfd/api/session_identity.go`,
   implementing RFC 5882 Section 4.4 for multi-hop: it clears the interface and
   derives `Local` from the interface the route to the peer leaves by, refusing
   on no route, no address, several addresses, or a non-default VRF. Thomas chose
   "make the two builders agree on the key" on 2026-09-09, and the ledger row was
   NOT lowered.
3. `api.Link` gaining `VRF`, the `l.VRF != r.VRF` guard in `deriveLink`, and
   `vrfMembership` in `internal/component/bfd/session_identity.go` computing
   membership by walking `InterfaceInfo.MasterIndex` to a `Type == "vrf"` master.
4. The restore's one edit: `config_bfd_strict_test.go` was parked before
   `Canonical` took an `api.Topology`, so it called the old signature. Updated to
   `Canonical(api.Topology{Links: links})` with `VRF: api.DefaultVRF`.
5. The regenerated RFC aggregates: `ai/RFC-REQUIREMENTS.md`,
   `docs/features/rfc-status.md`, `rfc/enrolled.txt`, and the multi-hop test row
   in `rfc/requirements/rfc5882.md`. They understated this draft while the tests
   were parked.

Plus the eight always-in-scope classes, which apply anywhere they are found.

Not in scope: anything rounds 1 to 6 cleared that these fixes did not touch.

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

Every round from 2 on IS a re-run: each one reviewed only the fixes the round
before it produced, and each scope block above says so. The table above carries
them. The last run over product code is round 14, which reported 0 BLOCKER,
2 ISSUE and 4 NOTE, and whose reviewer recommended no round 15 because both
ISSUEs were additive assertions in one test file.

The closure round is the pass after that, run in this context over the whole
uncommitted diff. Its two findings are the last row of the table above. Both are
fixed here, and neither moved BFD or FSM behavior: one deleted a duplicated call
another session's merge left in `peer_run.go`, and one repointed two citations of
a spec that commit B removes.

| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | ISSUE | `p.updateDelayPeerDown()` and its four-line comment appear twice in the same `else if from == fsm.StateEstablished` block | `internal/component/bgp/reactor/peer_run.go` | fixed: the duplicate removed. `updateDelayHold.peerDown` is two map deletes, so the second call changed nothing, which is why no test saw it |
| 2 | ISSUE | `docs/architecture/bfd.md` and the `rfc/short/rfc5882.md` enrolment reason both resolved the seven-row refusal table to `plan/spec-bgp-bfd-strict.md`, which commit B deletes | `docs/architecture/bfd.md`, `rfc/short/rfc5882.md` | fixed: the table now lives in `rfc/short/rfc5882.md` under "Multiple Control Protocols (Section 4.4)" and both citations point there |
| 3 | NOTE | the parity test gained two lint fixes after round 14 closed: a checked `reflect.TypeAssert[api.Key]` in place of an unchecked assertion, and `Type.Fields()` for the two name-only loops | `internal/component/bfd/engine/first_packet_parity_test.go` | re-verified at closure, not merely read: the mutation below was re-run against the edited file |
| 4 | ISSUE | `Session.SetBFDStateReader` is exported and takes the UNEXPORTED type `bfdStateReader`, so no package outside `reactor` can call it. `./le repository check` names it, and its only caller is `peer_run.go` in the same package | `internal/component/bgp/reactor/session_bfd_strict.go` | fixed: renamed to `setBFDStateReader`, which is the local shape for a same-package setter (`setConfigCapabilityGetter`, `peer_settings_negotiation.go`). Eleven sibling `Session` setters carry the same finding; they take exported types, so they are a different question and were left alone |
| 5 | ISSUE | three British spellings in files this spec adds; `misspell` names one of them in the full verify | `internal/test/fixture/bfd_fixture_pktinfo.go`, `test/bfd/bfd-first-packet-pktinfo.ci`, `...-v6.ci` | fixed: `synthesised` and `synthesise` to the US forms, which is the project language (`ai/rules/writing.md`) |
| 6 | ISSUE | `./le doc wiring` reported 9 pages whose claims name a symbol this diff changed. Read one by one against the code, five carried a statement the change had made WRONG or incomplete, and four did not | five pages | fixed, each with the fact rather than a token edit: `fsm.md`'s hold-down timer said a BFD Up arms it, which is true on ONE of the two release rails and false on the other; `fsm-active.md` said a pre-buffered OPEN drives Active to OpenConfirm in one sequence, which strict mode stops at OpenSent; `fsm-established.md`'s entry section named one rail into Established and there are now two, one of them delayed by the hold-down; `peer-lifecycle.md`'s numbered `runOnce` sequence had no step for the early BFD start; `docs/guide/bfd.md` described the Established-scoped BFD lifetime unqualified, still said AdminDown tears the session down, and enumerated the socket options without `IP_PKTINFO` |
| 7 | NOTE | four more drifted pages carry claims this change did NOT falsify: `fsm-connect.md` (the passive/active decision), `bgp-fsm.md` (the FSM goroutine), `command-reference.md` (peer row ordering), `configuration.md` (AS_PATH and migration), `monitoring.md` (the connect-retry counter's CLI field) and `update-building.md` (the read-buffer lifecycle) | those pages | not edited. The drift is at SYMBOL granularity: the function moved, the sentence about it did not become false. Editing them to clear the gate would be the token edit the documentation rule exists to prevent |

### Independent discrimination, run in this context

Round 14's two fixes had no review round of their own, so closure broke the
producers itself rather than trusting the record:

| Break applied | What went red | What stayed green |
|---------------|---------------|-------------------|
| `iface` dropped from `firstPacketIndex` (`engine.go`) | `TestFirstPacketIndexCarriesEveryFieldValue` naming the field and the consequence, plus `TestFirstPacketMatchesWhatTheTransportSurfaces`, `TestFirstPacketPrefersTheSessionThatNamedTheLink`, `TestFirstPacketTieBreakPrefersTheLink` | the rest of the package |
| `relaxLocal` and `relaxIface` swapped in the `iota` run | `TestFirstPacketTieBreakPrefersTheLink` ALONE | every other test in `internal/component/bfd/engine`, which is the measurement that says the comment had been the only guard |

Both breaks were reverted and the package re-run green
(`go test ./internal/component/bfd/...`, 8 packages ok).

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE -- MET: round 14 reported 0 BLOCKER; its 2 ISSUEs and the closure round's 2 are fixed above
- [ ] All NOTEs recorded above -- MET

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

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| The capability: code 74, length 0, advertised when strict is enabled (Sections 5, 6) | Done | `capability.BFDStrictMode`, `CodeBFDStrictMode` (`internal/core/bgp/capability/capability.go`); `parsePeerFromTree` (`internal/component/bgp/reactor/config.go`) | `Negotiated.BFDStrictMode` set only when both sides advertised |
| The FSM: events 30 to 35 and the two sub-states (Sections 4, 8) | Done | `fsm.EventBfdAdminDown`..`EventBfdStrictConfigChanged`, `BfdSubState` (`internal/component/bgp/fsm/state.go`); arms in all six handlers (`fsm.go`) | `ConnectDelayOpenBfdUpPending` and `ActiveDelayOpenBfdUpPending` are unreachable: ze implements no DelayOpenTimer, recorded as feature-out-of-scope |
| Session start and stop (Section 7) | Done | `Peer.runOnce` opens before the start event, `Peer.cleanup` releases (`internal/component/bgp/reactor/peer_run.go`) | non-strict peers keep the Established-scoped lifetime |
| Cease (6) / BFD Down (10) on every session this feature closes (Section 9) | Done | `Session.bfdTeardown` (`internal/component/bgp/reactor/session_bfd_strict.go`) | the config-change clause carries Other Configuration Change (6), which is the draft's own exception |
| Config surface: `strict`, `hold-time`, `hold-down` | Done | `internal/component/bgp/yang/ze-bgp-conf.yang`; `parseBFDSettings`, `validateBFDHoldDown` (`reactor/config.go`) | `hold-down` was added during the work for Section 10 |
| RFC 5882 Section 4.4, one session per neighbor | Done, partially met by construction | `api.SessionRequest.Canonical` (`internal/component/bfd/api/session_identity.go`); `connectedLinks`, `topologyFor` (`internal/component/bfd/session_identity.go`) | seven configurations refuse the derivation and leave two sessions; the table moved to `rfc/short/rfc5882.md` and the ledger publishes `Partial` |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestBFDStrictModeCapabilityEncoding`, `TestBFDSettingsStrictParse`, `test/plugin/bgp-bfd-strict.ci` | |
| AC-2 | Done | `TestNegotiateBFDStrictMode`, `TestSessionBFDStrictEstablishesWhenPeerDoesNotAdvertise`, interop `bgp-bfd-strict-frr` | the interop half proves it against FRR, which advertises no 74 |
| AC-3 | Done | `TestSessionBFDStrictWithholdsKeepalive`, `TestBFDStrictOpenSentHoldsUntilUp` | |
| AC-4 | Done | `TestSessionBFDStrictSendsKeepaliveOnBFDUp`, `TestBFDStrictOpenSentHoldsUntilUp` | |
| AC-5 | Done | `TestBFDStrictKeepaliveBeforeUp`, `TestBFDStrictKeepaliveInOpenSentStaysAnError` | the second pins that the relaxation is the sub-state's alone |
| AC-6 | Done | `TestSessionBFDStrictConfirmedPathReachesEstablished` | |
| AC-7 | Done | `TestSessionBFDStrictArmsBfdHoldTimerOnZeroHoldTime`, `TestSessionBFDStrictBfdHoldTimerTearsDownAZeroHoldTimeSession`, `TestBfdHoldTimerFiresAtBfdHoldTime`, `TestBFDStrictHoldTimerExpires` | |
| AC-8 | Done | `TestBFDStrictDownConnectRetryCounter`, `TestSessionBFDStrictDownClosesWithBFDDownSubcode` | counter zeroed in OpenSent and OpenConfirm |
| AC-9 | Done | `TestBFDStrictDownConnectRetryCounter` (Established arm), `TestBFDClient_TeardownOnDown` | counter incremented; unchanged from before this spec |
| AC-10 | Done | `TestBFDStrictEstablishedIgnoresTheThreeUpEvents`, `TestBFDClientAdminDownDoesNotTeardown` | the AdminDown correction is a behavior CHANGE, recorded in the Implementation Summary |
| AC-11 | Done | `TestBFDStrictDownIgnoredWhereTheDraftSaysSo` | |
| AC-12 | Done | `TestBFDStrictConnectAndActiveStayPut`, `TestBFDStrictOpenConfirmIgnoresTheThreeUpEvents`, `TestBFDStrictIdleIgnoresEverything` | |
| AC-13 | Done | `TestBFDStrictConfigChangedResetsToIdle`, `TestSessionBFDStrictConfigChangedUsesConfigSubcode`, `TestSessionBFDStrictConfigChangedWithBFDDisabled` | the last pins Section 4's MUST NOT: with BfdEnabled false the producer raises BfdAdminDown instead |
| AC-14 | Done | `Peer.runOnce` and `Peer.cleanup` (`peer_run.go`); interop `bgp-bfd-strict-preup-speaker` and `bgp-bfd-strict-reload-speaker` | the reload scenario is the end-to-end proof: `show bfd sessions` reports ONE session at refcount 2 |
| AC-15 | Done | doctor check `bgp-bfd-strict-engine` with `TestCheckBFDStrictHasEngineRaisesTheCode`, `TestStrictPeersWithoutEngineNamesThePeer`; `startBFDClient` logs at error and the session gate holds | three surfaces: config warning, doctor code, runtime error |
| AC-16 | Done | `TestBFDSettingsStrictAbsentAdvertisesNothing`, `TestBFDSettingsStrictDisabledAdvertisesNothing` | |
| AC-17 | Done | `TestSessionBFDStrictHoldDownDelaysEstablishment`, `TestSessionBFDStrictHoldDownCountsTimeAlreadyServed`, `TestSessionBFDStrictHoldDownAbsentEstablishesOnFirstUp`, `TestSessionBFDStrictHoldDownRestartsOnAFlapInsideTheInterval`, `TestBFDHoldDownStarvationIsRefused` | the leaf is `hold-down <ms>`; the AC's text says `<ms>` and the spec's Task section said `hold-time`, which is a different leaf |
| AC-18 | Done | `TestSessionBFDStrictHoldIsBoundedByTheBGPHoldTimer` | proves the pending sub-state is never a hang when the negotiated hold time is non-zero |
| AC-19 | Done | `TestStrictPeerRequestReachesTheSharedKey`, `TestPinnedSessionReachesTheSharedKey`, `TestCanonicalCollapsesEveryClientShapeOntoOneKey`, `TestCanonicalRefusesToGuessAnAmbiguousLink`, interop `bgp-bfd-strict-preup-speaker` and `bgp-bfd-strict-reload-speaker` | the refusals are the seven rows now in `rfc/short/rfc5882.md` |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestBFDStrictModeCapabilityEncoding`, `...Parse`, `TestNegotiateBFDStrictMode` | Done | `internal/core/bgp/capability/bfd_strict_test.go` | plus `...RoundTrip`, `...RequiredCode`, `...CodeString` |
| The eight FSM tests | Done, exceeded | `internal/component/bgp/fsm/bfd_strict_test.go` | 17 tests |
| `TestSessionBFDStrictWithholdsKeepalive` | Done, exceeded | `internal/component/bgp/reactor/session_bfd_strict_test.go` | 21 tests |
| `TestBFDSettingsStrictParse` | Done, exceeded | `internal/component/bgp/reactor/config_bfd_strict_test.go` | 7 tests; the file was planned as `config_test.go` and split out |
| `TestBFDClientAdminDownDoesNotTeardown` | Done | `internal/component/bgp/reactor/peer_bfd_test.go` | |
| `bgp-bfd-strict`, `bgp-bfd-strict-pinned` | Done | `test/plugin/` | both PASS, re-run at closure |
| `bfd-first-packet-pktinfo`, `...-v6` | Done, added during the work | `test/bfd/` | `option=needs-linux`; PASS and forced RED in the QEMU guest |
| Four interop scenarios | Done, three added during the work | `test/interop/scenarios/bgp-bfd-strict-*` | |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| Every file in "Files to Modify" and "Files to Create" | Done | verified by `ls` and `git status`; the Pre-Commit Verification table below carries the evidence |
| `internal/component/bgp/reactor/session_bfd_strict.go` | Added, not planned | the plan put the wire half in `session_handlers.go` and `session_connection.go`; both rails now call into this one file, which is what keeps the draft's clause table in one place |
| `internal/component/bfd/transport/udp*.go` | Added, not planned | the first-packet demux defect (round 8) was reached through this spec's own gate |

### Audit Summary
- **Total items:** 6 requirements, 19 acceptance criteria, 8 test groups
- **Done:** all of them
- **Partial:** none. RFC 5882 Section 4.4 is met wherever the clients reach one key and refuses rather than guesses elsewhere; that is a stated design decision with its seven cases published, not an unfinished item
- **Skipped:** none
- **Changed:** three, all in Deviations below

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| A peer whose forwarding path is broken must not establish BGP | interop | `bgp-bfd-strict-speaker` PASS 2026-09-09: with strict negotiated on both sides ze sends no KEEPALIVE while BFD is down, and one once it is Up. Reddens when the hold is disabled |
| A peer that does not speak the draft is unaffected | interop | `bgp-bfd-strict-frr` PASS 2026-09-08 against FRR 10.3.1 bgpd with no BFD: ze establishes normally. Reddens when the negotiation half of the condition is dropped |
| The capability is on the wire and negotiated both ways | functional | `test/plugin/bgp-bfd-strict.ci` PASS at closure (992ms, plugin suite test 102): the two YANG leaves parse and code 74 reaches the OPEN |
| One BFD session per neighbor, whatever asks for it (RFC 5882 Section 4.4) | interop | `bgp-bfd-strict-reload-speaker` PASS 2026-09-09: a SIGHUP reload adds the strict peer to a session already Up and `show bfd sessions` reports ONE session at refcount 2. RED with the Subscribe snapshot removed: the peer never establishes |
| A first packet reaches the session it belongs to, on a real kernel | functional, in a Linux guest | `bfd-first-packet-pktinfo` and `bfd-first-packet-pktinfo-v6` PASS 2026-09-11 in the QEMU guest (`qemu-r13b.log`). Forced RED there, separately: `qemu-red2.log` with `readLoop` stamping the bind address again, and `qemu-v6red.log` with `IPV6_RECVPKTINFO` not enabled. Both reds name the mechanism: "the kernel's PKTINFO did not reach firstPacketKey" |
| A key field nobody has added yet cannot silently break the lookup | structural test, discrimination re-run at closure | `TestFirstPacketKeyMirrorsEveryKeyField`, `TestRelaxationsCoverEveryRelaxableField`, `TestFirstPacketIndexCarriesEveryFieldValue`. Deleting `iface` from `firstPacketIndex` reds the third by name; swapping the two relaxation constants reds `TestFirstPacketTieBreakPrefersTheLink` and nothing else |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| A BFD session to an IPv6 LINK-LOCAL peer still cannot be selected for a packet carrying no discriminator: `in.From` holds `fe80::1%eth0` and the key holds `fe80::1`, and `Peer` is compared exactly | a DIFFERENT failure class from the five this spec repaired. Those were unsetness, and the rule they produced is general over that class. This one is two FORMS of one value, and its repair is a reconciliation with its own design question: the zone's real job is telling two sessions to the same link-local address on two links apart. Thomas decided on 2026-09-11 that it is not fixed here | `plan/immediate/spec-bfd-link-local-peer-zone.md`, with the mechanism at `plan/journal/declared-format-contradicts-payload.md` |
| The seven configurations where `Canonical` refuses and two BFD sessions remain | each needs a configuration answer rather than a derivation, and the VRF row needs a VRF-scoped route lookup the interface component does not offer | not owned by a spec: they are published as the standing limit of RFC5882-4.4-1 in `rfc/short/rfc5882.md`, and the ledger row reads `Partial` rather than `Supported` |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/bgp/reactor/session_bfd_strict.go` | yes | `ls -l` 455 lines |
| `internal/component/bgp/reactor/session_bfd_strict_test.go` | yes | `ls -l` 913 lines |
| `internal/component/bgp/reactor/config_bfd_strict_test.go` | yes | `ls -l` 243 lines |
| `internal/component/bfd/engine/first_packet_parity_test.go` | yes | `ls -l` 232 lines |
| `internal/component/bfd/api/session_identity.go`, `internal/component/bfd/session_identity.go` | yes | tracked, modified in this diff |
| `test/plugin/bgp-bfd-strict.ci`, `bgp-bfd-strict-pinned.ci` | yes | `ls -l test/plugin/bgp-bfd-strict*.ci` |
| `test/bfd/bfd-first-packet-pktinfo.ci`, `...-v6.ci` | yes | `ls -l test/bfd/bfd-first-packet-pktinfo*.ci` |
| `test/interop/scenarios/bgp-bfd-strict-{frr,speaker,preup-speaker,reload-speaker}/` | yes | `ls -d test/interop/scenarios/bgp-bfd-strict-*` returns four directories |
| `rfc/short/draft-ietf-idr-bgp-bfd-strict-mode.md`, `rfc/discrimination/rfc5880.json`, `rfc/discrimination/rfc5882.json` | yes | the first two enrolled, the third carrying the four `RFC5882-4.4-1` records |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1, AC-2 | capability 74 encodes, parses and negotiates | `go test ./internal/core/bgp/capability` ok |
| AC-3 to AC-13, AC-17, AC-18 | the FSM and session rails | `go test ./internal/component/bgp/fsm ./internal/component/bgp/reactor` ok for the BFD strict tests; 17 + 21 tests |
| AC-14, AC-19 | one session per neighbor, opened before the FSM | `go test ./internal/component/bfd/...` ok, 8 packages |
| AC-15 | strict without an engine is surfaced | `go test ./internal/component/bgp/config -run BFDStrict` ok |
| AC-16 | no `strict` leaf changes nothing | `TestBFDSettingsStrictAbsentAdvertisesNothing` ok |
| end to end | a user reaches it through config | `./le functional plugin` at closure: test 102 `bgp-bfd-strict` PASS in 787ms, test 101 `bgp-bfd-strict-pinned` PASS in 992ms |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `set protocols bgp neighbor X connection bfd strict true` to capability 74 in the OPEN | `test/plugin/bgp-bfd-strict.ci` | yes: the file drives `ze-test fixture plugin/bgp-bfd-strict` and asserts the leaves parse and the capability is advertised. PASS at closure |
| a strict peer joining a session a pinned entry already created | `test/plugin/bgp-bfd-strict-pinned.ci` | yes: read, and it asserts the peer reads the CURRENT state from the Subscribe snapshot. PASS at closure |
| a Control packet with Your Discriminator 0 selecting its session on a real kernel | `test/bfd/bfd-first-packet-pktinfo.ci`, `...-v6.ci` | yes: both run in the QEMU guest, and both were forced RED there |
| BFD Up driving the FSM | `internal/component/bgp/fsm/bfd_strict_test.go`, `session_bfd_strict_test.go` | yes |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `EnsureSession` is not gated on BGP state; `Peer.runOnce` opens the session one statement before the start event and `Peer.cleanup` releases it |
| A-2 | confirmed, then made irrelevant | both rails did call `sendKeepalive` after Event 19, and both now call `advanceAfterOpen` instead, so the gate cannot be bypassed by a third rail being added |
| A-3 | confirmed | ze implements no DelayOpenTimer, so Event 20 never fires and the two DelayOpen sub-states are unreachable. Recorded as an optional feature out of scope, not a gap |
| A-4 | broken | FRR 10.3.1 does not advertise capability 74, so the FRR scenario proves the NEGATIVE half only. The POSITIVE half needed the lab's own speaker, extended to advertise 74 and answer BFD (`internal/le/interoplab/bgp/speaker_bfd.go`). Mistake Log row below |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| `docs/architecture/wire/capabilities.md` code table carries 74 | `capability.CodeBFDStrictMode` | yes, edited in the implementation commits |
| `docs/architecture/behavior/fsm-open-sent.md`, `fsm-open-confirm.md` carry the sub-states and the six events | `fsm.BfdSubState`, the arms in `fsm.go` | yes, edited in this diff |
| `docs/architecture/bfd.md` one-session-per-neighbor section | `api.SessionRequest.Canonical`, `Loop.handleInbound` | yes, edited in this diff, and its pointer to the seven-row table repointed at `rfc/short/rfc5882.md` at closure |
| `docs/guide/bfd.md` carries `strict`, `hold-time` and `hold-down` | the YANG leaves in `ze-bgp-conf.yang` | yes, edited in the implementation commits: the leaves, what each one does, and the OpenConfirm / hold-down / hold-timer relation |
| `docs/features.md` BFD row and `docs/comparison.md` | `capability.BFDStrictMode`, `session_bfd_strict.go` | yes, both name the draft and capability 74, with source anchors |
| `docs/guide/configuration.md` -- the spec's checklist named it | `grep -n bfd docs/guide/configuration.md` returns NOTHING | no update owed: that page documents no BFD configuration at all, and `docs/guide/bfd.md` is the page that owns the block. The checklist row was written before anyone looked |
| `docs/features/rfc-status.md` and the three other generated RFC indexes | `./le rfc index-update` re-run at closure after the `rfc/short/rfc5882.md` edit | yes; the draft's row and RFC 5882's `Partial` are published in this commit, in the same commit as the producing code |
| `./le doc check links` | run at closure | 41 broken references tree-wide, NONE of them from a file this spec edits; the three that would have been created by removing the spec were repaired first |

## Core Insight

A defect repaired five times is a rule that has not been written down. Each of
the first four repairs to the first-packet lookup was correct in the dimension it
examined and blind in the next, and the sentence that covers all of them was
available at the first one: a key field the session left UNSET does not
participate in the match. The general form cost one constant and one loop, where
the field-by-field repairs cost four review rounds. The full lesson, including
what a structural parity test does and does not hold, is
`plan/learned/019-state-the-rule-not-the-field.md`.
