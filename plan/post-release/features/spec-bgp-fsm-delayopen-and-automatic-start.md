# Spec: bgp-fsm-delayopen-and-automatic-start -- DelayOpen and Event 3 in the BGP FSM

| Field | Value |
|-------|-------|
| Status | in-progress |
| Priority | P3 |
| Release reason | Optional DelayOpen and automatic-start controls are owner-selected follow-up capabilities, not required behavior of the shipped FSM. |
| Scope | protocol |
| Depends | - |
| Phase | 3a/5 — strict-mode DelayOpen sub-states (row 6s) |
| Handoff | `tmp/session/2026-10-22-644a4e78-0122-4eeb-8642-eb50638a1641/state/session-state-bgp-fsm-delayopen-and-automatic-start-644a4e78-0122-4eeb-8642-eb50638a1641.md` |
| Updated | 2026-10-27 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The BGP child of the rfc-verdict pass (spec-rfc-verdict-fix-bgp; its
"Gap accounting" section survives in git history) left requirement rows that no spec owned.
One spec took them (`plan/release/fixes/spec-bgp-unowned-rfc-rows.md`). On 2026-10-11 the
owner put DelayOpen and Event 3 IN and split the work: "Split into three". This
spec owns the two RFC 4271 FSM rows. The defects stay in the immediate spec; the
EPE rows are `plan/post-release/features/spec-bgp-epe-peer-segments-and-srv6-peernode.md`.

Row numbers are the numbers the rows had in the parent spec.

| # | Row id | Document, section | Class | What Ze does today |
|---|--------|-------------------|-------|--------------------|
| 6 | `RFC4271-8.2.2-13` | RFC 4271 §8.2.2 (Active) | gap: OPTIONAL feature absent, owner IN | No DelayOpen, so Active never holds a connection and the Event 18 action list is reached by no layer |
| 7 | `RFC4271-8.2.2-20` | RFC 4271 §8.2.2 (Idle) | gap: optional event absent, owner IN | No Event 3; automatic restart is Event 6, which keeps the ConnectRetryCounter |
| 6s | draft-ietf-idr-bgp-bfd-strict-mode §8.3.5, §8.4.5 (and the sub-state branches of §8.3.1, §8.4.1) | draft, Connect and Active | gap: reachable only with DelayOpen, owner IN | `ConnectDelayOpenBfdUpPending` and `ActiveDelayOpenBfdUpPending` are undeclared, and four comments plus two pages call them unreachable because Ze has no DelayOpenTimer |

Row 6s exists because row 6 makes the two strict-mode sub-states reachable: with
`timer delay-open` set on a peer whose `connection bfd { strict true }` is enabled,
an OPEN received while the DelayOpenTimer runs (Event 20) enters the draft's
revised Event 20 clause. On 2026-10-11 the owner, asked whether to implement them or
refuse `delay-open` together with `bfd strict`, answered: "Implement them". The
draft's Section 8 carries no RFC 2119 keyword (`rfc/short/draft-ietf-idr-bgp-bfd-strict-mode.md`,
Notes), so 6s adds behaviour and tests but no MUST-level requirement row; AC-9
says what the ledger gains instead.

The class column follows `ai/rules/rfc-compliance.md`: "a capability Ze does not
provide is an implementation gap". The owner scoped both in, so both are
implemented here.

### Bucket

`plan/`. Both rows are OPTIONAL FSM behavior: the release goes out without
DelayOpen and with Event 6 as its automatic start, and no operator meets either
as a bug. `plan/README.md` puts that at this level. The owner's split put them here.

### The RFC text, quoted from the source

| # | Quote | Read at |
|---|-------|---------|
| 6 | "If the local system receives a TcpConnectionFails event (Event 18), the local system: - restarts the ConnectRetryTimer (with the initial value), - stops and clears the DelayOpenTimer (sets the value to zero), - releases all BGP resource, - increments the ConnectRetryCounter by 1," and §8.2.1.3: "If the flag indicating support for an optional timer (DelayOpen or DampPeerOscillations) cannot be set to TRUE, the timers and events supporting that option do not have to be supported." | `rfc/full/rfc4271.txt` §8.2.2 Active, §8.2.1.3 |
| 7 | "In response to a ManualStart event (Event 1) or an AutomaticStart event (Event 3), the local system: - initializes all BGP resources for the peer connection, - sets ConnectRetryCounter to zero," and §8.1.2 Event 3 "Status: Optional, depending on local system" | `rfc/full/rfc4271.txt` §8.2.2 Idle, §8.1.2 |
| 6 (Connect, Event 16/17) | "If the DelayOpen attribute is set to TRUE, the local system: - stops the ConnectRetryTimer (if running) and sets the ConnectRetryTimer to zero, - sets the DelayOpenTimer to the initial value, and - stays in the Connect state." | `rfc/full/rfc4271.txt` §8.2.2 Connect, lines 3070-3086 |
| 6 (Connect, Event 12) | "If the DelayOpenTimer_Expires event (Event 12) occurs in the Connect state, the local system: - sends an OPEN message to its peer, - sets the HoldTimer to a large value, and - changes its state to OpenSent." | §8.2.2 Connect, lines 3054-3060 |
| 6 (Connect, Event 18) | "If the DelayOpenTimer is running, the local system: - restarts the ConnectRetryTimer with the initial value, - stops the DelayOpenTimer and resets its value to zero, - continues to listen for a connection that may be initiated by the remote BGP peer, and - changes its state to Active." | §8.2.2 Connect, lines 3104-3115 |
| 6 (Connect and Active, Event 20) | "If an OPEN message is received while the DelayOpenTimer is running (Event 20), the local system: - stops the ConnectRetryTimer (if running) and sets the ConnectRetryTimer to zero, - completes the BGP initialization, - stops and clears the DelayOpenTimer (sets the value to zero), - sends an OPEN message, - sends a KEEPALIVE message, ... - and changes its state to OpenConfirm." (Active words it "If an OPEN message is received and the DelayOpenTimer is running") | §8.2.2 Connect lines 3127-3150, Active lines 3378-3395 |
| 6 (Connect and Active, Event 24) | "If the DelayOpenTimer is running, the local system: - stops the ConnectRetryTimer (if running) and sets the ConnectRetryTimer to zero, - stops and resets the DelayOpenTimer (sets to zero), - releases all BGP resources, - drops the TCP connection, and - changes its state to Idle." | §8.2.2 Connect lines 3188-3207, Active lines 3440-3452 |
| 6 (Active, Event 16/17) | "If the DelayOpen attribute is set to TRUE, the local system: - stops the ConnectRetryTimer and sets the ConnectRetryTimer to zero, - sets the DelayOpenTimer to the initial value (DelayOpenTime), and - stays in the Active state." | §8.2.2 Active, lines 3326-3338 |
| 6 (Active, Event 12) | "If the local system receives a DelayOpenTimer_Expires event (Event 12), the local system: - sets the ConnectRetryTimer to zero, - stops and clears the DelayOpenTimer (set to zero), - completes the BGP initialization, - sends the OPEN message to its remote peer, - sets its hold timer to a large value, and - changes its state to OpenSent." | §8.2.2 Active, lines 3292-3314 (verbatim recheck 2026-10-11, round-2 I2) |
| 6s (§8.3.5 new text) | "If the FSM is NOT in the ConnectDelayOpenBfdUpPending, the local system: ... - sends an OPEN message, - If BfdEnabled is TRUE, and BfdStrictNegotiated is TRUE, and bfd.SessionState is neither Up nor AdminDown, - DOES NOT send a KEEPALIVE message, ... - stays in the Connect state (enters ConnectDelayOpenBfdUpPending)." and "If the FSM is in the ConnectDelayOpenBfdUpPending sub-state, the reception of a second OPEN message is a FSM error." | `rfc/drafts/draft-ietf-idr-bgp-bfd-strict-mode.txt` §8.3.5, lines 548-610; §8.4.5 is the Active copy, lines 720-830 |
| 6s (§8.3.1 sub-state branch) | "If the FSM is in the ConnectDelayOpenBfdUpPending sub-state, the local system: - sends a KEEPALIVE message, - if the HoldTimer initial value is non-zero, - starts the KeepaliveTimer with the initial value and - resets the BfdHoldTimer value to zero, - and changes its state to OpenConfirm (leaves ConnectDelayOpenBfdUpPending)." | draft §8.3.1, lines 424-441; §8.4.1 is the Active copy, lines 642-659 |

### How FRR and BIRD handle it

| # | FRR (`/Users/thomas/Code/github.com/FRRouting/frr`) | BIRD (`/Users/thomas/Code/gitlab.nic.cz/labs/bird`) |
|---|------|------|
| 6 | DelayOpen implemented (`bgpd/bgp_fsm.c`, `bgp_delayopen_timer`); configured per neighbor as `neighbor X timers delayopen (1-240)` (`bgpd/bgp_vty.c` line 8204) | No DelayOpen; restart pacing is `error_delay_time_min/max` backoff (`proto/bgp/bgp.c` around line 1018), the same shape as Ze's Event 6 |
| 7 | No RFC ConnectRetryCounter tracked; FRR counts `peer->dropped` | No RFC ConnectRetryCounter |
| 6 (Event 18 in Active) | The FSM table maps TCP_connection_open_failed in Active to `bgp_ignore`, so FRR performs none of the four Event 18 actions; Receive_OPEN_message in Connect and Active goes to `bgp_fsm_open` and OpenConfirm, and DelayOpen_timer_expired to `bgp_fsm_delayopen_timer_expire` and OpenSent (`bgpd/bgp_fsm.c` around lines 2655-2690) | N-A |
| 6s | FRR has no BFD strict mode (`bgp-bfd-strict-frr` README: FRR 10.3.1 implements neither) | No BFD strict mode |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/behavior/fsm.md` - names DelayOpen and the timers Ze does not implement
  → Constraint: rows 6 and 7 are IN (owner, 2026-10-11), so this page changes with the FSM phase: DelayOpen, the DelayOpenTimer and Event 3 move from "not implemented" to implemented, in the same commit as the FSM code
- [ ] `docs/architecture/testing/interop.md` - scenario naming and the revert-and-red walk
  → Constraint: `bgp-delayopen-frr` is named (no numeric prefix) and owes a recorded red with the fix reverted and the artifact rebuilt
- [ ] `docs/architecture/core-design.md` - the design page the reactor files this spec edits declare (peer run loop, session lifecycle, config parse)
  → Constraint: if it describes the reconnect loop's start events or the OPEN timing, it changes with rows 6 and 7
- [ ] `docs/architecture/behavior/fsm-active.md` - the Active event table
  → Constraint: line 47 says `EventTCPConnectionFails` is "not generated in production: Active holds no connection, because without DelayOpen ..." and that "RFC4271-8.2.2-13 carries `{feature-declined}`"; the second half is already stale (R53(a) made the row `{gap}`) and the first goes false with row 6. Rewritten with the DelayOpen phase
- [ ] `docs/architecture/behavior/fsm-connect.md` - the Connect event table
  → Constraint: gains the DelayOpen arms of Events 12, 16/17, 18, 20 and 24 with the DelayOpen phase
- [ ] `docs/architecture/behavior/fsm.md` "The two BFD strict-mode sub-states" (lines 217-223)
  → Constraint: says the two DelayOpen sub-states are entered only by Event 20 "and ze implements no DelayOpenTimer"; rewritten with row 6s to declare all four sub-states

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/draft-ietf-idr-bgp-bfd-strict-mode.md` - enrolment reason, support coverage, Notes
  → Constraint (row 6s): `rfc/short/draft-ietf-idr-bgp-bfd-strict-mode.md` lines 14 and 20 state that `ConnectDelayOpenBfdUpPending` and `ActiveDelayOpenBfdUpPending` "are unreachable and undeclared" because Ze implements no DelayOpenTimer; its Notes (line 99) record that Section 8 "carries no RFC 2119 keyword at all", so the two sections owe no requirement row and the enrolment reason and support coverage prose change instead
- [ ] `rfc/drafts/draft-ietf-idr-bgp-bfd-strict-mode.txt` - §8.3.1, §8.3.3, §8.3.4, §8.3.5, §8.4.1, §8.4.3, §8.4.4, §8.4.5
  → Constraint: in either DelayOpen sub-state the BfdHoldTimerExpires (§8.3.3) and BfdStrictConfigChanged (§8.3.4) arms Ze already runs for Connect and Active apply unchanged, because the state is still Connect or Active; only §8.3.1/§8.4.1 (sub-state branch) and §8.3.5/§8.4.5 (Event 20) carry sub-state-specific actions
- [ ] `rfc/short/rfc4271.md` - rows 8.2.2-13 (line 811), 8.2.2-20 (line 805)
  → Constraint: main-thread rulings R47(a) and R53(a) (`plan/handover/rfc-verdict-test-fix-pass/RULINGS.md`) hold 8.2.2-20 as a counted gap because Ze has an automatic start, and 8.2.2-13 as a `{gap}` stamped unimplemented because the gate refused `{feature-declined}` beside existing tags. Both are now implemented (owner: IN), so both rows lose `{gap}` and owe tags in both polarities; R50 found the HEAD-tagged 8.2.2-13 units drive a synthetic Event 18 no real Peer reached, so those tags are re-pointed at a test that reaches Active with DelayOpen through a real Peer

**Key insights:**
- Row 7's Event 3 and the existing Event 6 are chosen from one explicit flag the session sets on entering Established, not from a second timer and not from the loop's error arm, which cannot tell a session that ran (round-2 B1).
- Row 6 is the only wire-visible change: the moment Ze sends its OPEN.

## Current Behavior (MANDATORY)

**2026-10-26 phase-2 reconciliation:** the following checklist records the
design-time baseline, not today's source status. Event 3 and
`Peer.cycleReachedEstablished` were already present when phase 2 began; this
phase does not implement or certify phase 3. The base DelayOpen leaf, timer,
Connect/Active events, real Session wiring, and bounded pending collision
handoff are now authored, with verification assigned to the supervising main
agent. `fsm.handleDelayOpen`, `Session.delayOpenExpired`, `advanceAfterOpen`,
and `Peer.finishCollisionHandoff` are the enforcing entry points. Strict-mode
DelayOpen sub-states remain phase 3a, not part of this implementation.

**Source files in the design-time baseline:**
- [ ] `internal/component/bgp/fsm/fsm.go`, `internal/component/bgp/fsm/state.go` - Event 6 implemented as the reactor's exponential backoff (state.go line 162); Events 3 and DelayOpen absent by recorded design (fsm.go header lines 16-18, 77)
- [ ] `internal/component/bgp/reactor/session.go` - line 1006-1011 fires Event 6 for an automatic restart
- [ ] `internal/component/bgp/yang/ze-bgp-conf.yang` - grouping `peer-fields`, container `timer` (`receive-hold-time`, `send-hold-time`, `keepalive`, `connect-retry`); no DelayOpen leaf
- [ ] `internal/component/bgp/reactor/peer_run.go` - the reconnect loop: `delay` is reset to `reconnectMin` on `ErrTeardown` and on `inboundNotify`, and doubled after any other error. The `else` arm that resets `delay` and `prefixTeardownCount` when `safeRunOnce` returns nil is reached only when the peer is stopping: `runOnce` returns `session.Run`'s error, `Session.Run` returns non-nil on every exit (`waitForResume` error, `*closeReason`, or the read error), and `runOnce`'s only `return nil` is its `stopping` check (round-2 B1; the missed resets are a journal row in `plan/journal/`, not this spec's fix). So the loop keeps no "session ran" outcome today; around line 379 the first cycle after the operator's start fires Event 1 (`session.Start`) and every later cycle Event 6 (`session.startDamped`), chosen by the `operatorStarted` swap
- [ ] `internal/component/bgp/fsm/fsm.go` lines 380-388 - Events 1 and 4 are conflated: one arm resets the counter and goes to Active when `passive` is set, Connect otherwise
- [ ] `internal/component/bgp/reactor/config.go` around line 244 (timer container parse), `internal/component/bgp/reactor/peer_settings.go` around line 69 (timer defaults, YANG as source of truth)
- [ ] `internal/component/bgp/reactor/session_connection.go` `connectionEstablished` (line 328) fires Event 17 at line 413 and the OPEN follows through `session_negotiate.go` `sendOpen` (line 130): the OPEN is sent immediately after the TCP connection today
- [ ] `internal/component/bgp/reactor/session_bfd_strict.go` - `advanceAfterOpen`, `handleBFDEvent`, `raiseBFDHoldTimerExpired`: the wire half of the strict-mode OpenSent sub-states, which row 6s extends to the Connect and Active sub-states
- [ ] Comments that call the DelayOpen sub-states unreachable, all false after row 6s: `fsm/fsm.go` header lines 11 and 77, `EnterBfdUpPending` comment lines 232-240, the Connect BFD arm lines 503-512, the BfdHoldTimerExpires arm lines 531-532, the Event 24 arm lines 557-558 and 656, the Active sub-state arm lines 664-669; `fsm/state.go` lines 274-277; `fsm/timer.go` line 61; test comments `fsm/bfd_strict_test.go` line 274, `fsm/rfc4271_connect_any_other_event_test.go` line 48, `fsm/rfc4271_connect_retry_test.go` lines 320, 367, 396

**Behavior to preserve:**
- Event 6 backoff semantics and `TestRFC4271ConnectRetryCounterSurvivesDampedStart`.
- With `timer delay-open` unset (0), the OPEN timing and every existing FSM test unchanged.

**Behavior to change:** rows 6 and 7, as the ACs state.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- FSM: the peer run loop's automatic restart and TCP events (rows 6, 7).
- Config: `bgp peer <address> timer delay-open` (row 6).

### Transformation Path
1. Config parse of `timer delay-open` into the peer settings.
2. The reactor's run loop (`peer_run.go`) picks the start event for each cycle (row 7). The predicate is an explicit flag, because the loop keeps no "session ran" outcome (Current Behavior, round-2 B1): a peer-owned `cycleReachedEstablished` (an `atomic.Bool` on `Peer`) is cleared by `runOnce` before it starts the session and set by the FSM state callback in `peer_run.go` in the same branch that publishes `PeerStateEstablished` (`to == fsm.StateEstablished`). `run()` reads it after `safeRunOnce` returns, whatever `err` is, and carries the answer into the next cycle's start-event choice. Per exit: `nil` (peer stopping) ends the loop, no event; `ErrTeardown` after Established, an error after Established (NOTIFICATION received, HoldTimer expiry, read error, a recovered panic), and a backoff cut short by `inboundNotify` after such a cycle each start the next cycle with Event 3; the same exits from a cycle that never reached Established start it with Event 6. The backoff arm the error takes is unchanged by this spec: the flag decides the event, never the delay. Event 1 stays the operator's start.
3. The FSM runs the DelayOpenTimer in Connect and Active: on Event 16/17 with DelayOpen TRUE the reactor holds the connection without calling `sendOpen` until Event 20 (peer OPEN) or Event 12 (timer expiry) (row 6).
4. On Event 20 with `connection bfd { strict true }` negotiated and BFD neither Up nor AdminDown, the reactor sends the OPEN, withholds the KEEPALIVE and the FSM records the Connect or Active DelayOpen sub-state; a BfdUp, BfdAdminDown or BfdDisabled event then sends the KEEPALIVE and moves to OpenConfirm (row 6s). The same `advanceAfterOpen` decision that holds OpenSent today makes the call, so the strict-mode test is declared once.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config to reactor | peer settings carry the DelayOpen value | No |
| Reactor to FSM | Event 3 / Event 6 and the DelayOpen events | No |

### Integration Points
- `ze-bgp-conf.yang` grouping `peer-fields` - the new leaf reaches peer, group and bgp levels.
- `peer_run.go` start-event choice (around line 379) - the Event 3 / Event 6 choice; `session.go` `Start` / `startDamped` gain the Event 3 entry.
- `session_bfd_strict.go` `advanceAfterOpen` - the KEEPALIVE-withholding decision, now also called from the Event 20 path (row 6s).

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | to be shown at implementation |
| No unintended coupling (components stay isolated) | No | to be shown at implementation |
| No duplicated functionality (extends existing, does not recreate) | No | Event 3 reads one flag set at the FSM's Established transition, no second timer |
| Zero-copy preserved where applicable (refs, not copies) | N-A | no wire encoding change |
| Registration over hardcoding, outbound | N-A | no registry involved |
| Registration over hardcoding, inbound | N-A | no registry involved |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | FRR's `neighbor X timers delayopen` lets an interop scenario run DelayOpen on both sides | `bgpd/bgp_vty.c` line 8204, read in the parent spec | `bgp-delayopen-frr` covers Ze-side DelayOpen only, and the scenario README says so | build the FRR scenario with both sides set | holds by source reading (2026-10-11) |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Row 7 changes restart pacing: zeroing the counter on Event 3 hides oscillation the counter used to show | a flapping peer restarts faster after the change | Event 3 only on the cycle after a session that reached Established, read from the `cycleReachedEstablished` flag the session sets, never from the loop's error arm (which cannot tell); every cycle after a failed attempt keeps Event 6 and its counter (AC-2) |
| R-3 | Row 6s holds a session in Connect or Active with an OPEN sent and no KEEPALIVE | a session stuck before OpenConfirm with BFD Down | The HoldTimer (reset to the negotiated value, AC-7h) or the BfdHoldTimer (hold time zero, AC-7) bounds the wait |
| R-2 | Row 6 delays Ze's OPEN; a peer that also waits never establishes | a session stuck in Active or Connect | The DelayOpenTimer always expires and Ze then sends its OPEN; the interop scenario sets DelayOpen on both sides |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Row 6: session establishment timing with `timer delay-open` set (OPEN held, Event 18 in Active). Row 7: restart pacing and the ConnectRetryCounter |
| How is it reverted? | Each row lands as its own commit, revertible alone |
| Who else touches this path? | the rfc-verdict pass `plan/release/validation/spec-rfc-verdict-test-fix-pass.md` (ledger rows, the R50 re-pointed tags its BGP child spec-rfc-verdict-fix-bgp landed), `spec-bgp-open-session-rfc-defects` (closed 2026-10-21; OPEN and session handling), `spec-bgp-live-receive-binding-and-bfd-keep-session` (closed 2026-10-13; BFD strict-mode and session work in flight on 2026-10-11; row 6s edits `session_bfd_strict.go` and the strict-mode FSM arms it may also touch, so this spec's row 6s phase starts after that spec lands or coordinates file by file) |
| Row 6s | A peer with `bfd strict` and `delay-open` both set: wrong sub-state handling holds the session in Connect or Active until the BfdHoldTimer or the HoldTimer ends it |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Config `bgp peer <address> timer delay-open 5`, peer connects | → | FSM DelayOpenTimer, OPEN held, Event 18 in Active | `test/plugin/bgp-delay-open.ci` |
| An established peer session ends (peer NOTIFICATION, hold expiry, or `peer <address> clear` / session reset) and the reconnect loop restarts it | → | FSM Event 3 zeroes ConnectRetryCounter | `TestRFC4271AutomaticStartEvent3ZeroesCounter` (`internal/component/bgp/reactor/`, through a real `Peer`; Event 3 is not wire-visible, so a Go test through the run loop is the entry point a user reaches by resetting an established session) |
| Config `timer delay-open 5` plus `connection bfd { strict true }` on a peer whose BFD session is Down, peer sends its OPEN | → | Connect/Active DelayOpen BFD sub-state, KEEPALIVE withheld until BfdUp | `test/plugin/bgp-delay-open-bfd-strict.ci` (lab BFD speaker, the shape `bgp-bfd-strict-speaker` uses) |

## Acceptance Criteria

AC numbers changed in the split: AC-1 and AC-2 were AC-6 and AC-7 in the parent
spec; AC-3 is the parent's ledger AC-11 for the `rfc4271` stem.

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 (row 6, config) | `bgp peer <address> timer delay-open <seconds>` | 1..240 sets the DelayOpen attribute TRUE and DelayOpenTime to the value; `0`, the default, leaves DelayOpen FALSE and every existing FSM test and the OPEN timing unchanged. A value outside the range is refused at commit with the leaf, the range and the fix |
| AC-1a (row 6, Event 16/17) | DelayOpen TRUE, TCP connection succeeds in Connect (Ze dialled) or Active (Ze accepted), through a real `Peer` | The ConnectRetryTimer is stopped and zero, the DelayOpenTimer runs with DelayOpenTime, no OPEN is written to the connection, the state stays Connect or Active (the two quoted clauses) |
| AC-1b (row 6, Event 12) | DelayOpen TRUE, the DelayOpenTimer expires with no peer OPEN, in Connect and in Active | Ze sends its OPEN, sets the HoldTimer to a large value, and moves to OpenSent; in Active it also zeroes the ConnectRetryTimer and clears the DelayOpenTimer (the two quoted clauses) |
| AC-1c (row 6, Event 20) | DelayOpen TRUE, the peer's OPEN arrives while the DelayOpenTimer runs, in Connect and in Active, BFD strict not negotiated | ConnectRetryTimer zero, DelayOpenTimer cleared, Ze sends its OPEN then a KEEPALIVE, the KeepaliveTimer and HoldTimer follow the negotiated hold time (or both reset when it is zero), and the state is OpenConfirm with no stop in OpenSent |
| AC-1d (row 6, Event 18 in Connect) | DelayOpen TRUE, the held connection fails in Connect while the DelayOpenTimer runs | The ConnectRetryTimer restarts with its initial value, the DelayOpenTimer is cleared, Ze keeps listening, and the state is Active |
| AC-1e (row 6, Event 18 in Active, the ledger row) | DelayOpen TRUE, the held connection fails in Active while the DelayOpenTimer runs, reached through a real `Peer` | The four quoted actions: the ConnectRetryTimer restarts with its initial value, the DelayOpenTimer is cleared, the connection's resources are released, the ConnectRetryCounter rises by 1; the state is Idle. Negative polarity: with DelayOpen FALSE no Event 18 reaches Active (today's path) |
| AC-1f (row 6, Event 24) | DelayOpen TRUE, a NOTIFICATION with a version error arrives while the DelayOpenTimer runs, in Connect and in Active | ConnectRetryTimer zero, DelayOpenTimer cleared, resources released, connection dropped, state Idle, and the ConnectRetryCounter is NOT incremented (the DelayOpenTimer-running clause carries no increment; the not-running clause Ze runs today keeps its increment) |
| AC-1g (row 6, the other-events arm) | DelayOpen TRUE, any other event of Connect's or Active's catch-all list while the DelayOpenTimer runs | The DelayOpenTimer is stopped and reset in the same arm that already stops the ConnectRetryTimer, then today's actions |
| AC-1h (row 6, pre-buffered OPEN, round-2 I1) | DelayOpen TRUE, an inbound connection whose peer OPEN Ze has already read reaches the session through `acceptWithOpen` (the collision hand-off) | The accept is Event 17 and the buffered OPEN is Event 20, in that order, in the state the start left (round-3 I1): `runOnce` fires the start event before it takes the inbound slot and runs `acceptWithOpen` (`peer_run.go`), and `session.go` sets `SetPassive(!settings.Connection.IsActive())`, so for a peer that also dials (`connection both`, the default) the start goes to Connect and both events arrive in Connect, and for a passive-only peer (`connection passive`) the start goes to Active and both arrive in Active. An active-only peer never reaches this path: `acceptOrReject` closes its inbound. In both states: the DelayOpenTimer is started and then cleared, the ConnectRetryTimer is zero, Ze writes no OPEN before it processes the buffered one (`connectionEstablished` holds it, AC-1a), then sends its OPEN and a KEEPALIVE and moves to OpenConfirm without passing OpenSent (or to `ActiveDelayOpenBfdUpPending` under row 6s, AC-4). With DelayOpen FALSE the sequence is today's (OPEN sent, OpenSent, then OpenConfirm) |
| AC-1i (row 6, second connection, round-2 I1) | DelayOpen TRUE on Ze and on the peer: Ze holds its outbound connection in Connect with the DelayOpenTimer running and no OPEN either way, and the peer's own outbound connection arrives (both speakers dial, both hold) | RFC 4271 §6.8 runs "Upon receipt of an OPEN message" and compares BGP Identifiers, which neither OPEN-less connection carries, so no collision is decided while both are OPEN-less. The inbound reaches the pending slot only through a changed gate (round-3 B1): today `acceptOrReject` (`reactor_connection.go`) sends an inbound to `setPendingConnection` and `handlePendingCollision` only in OpenSent, OpenConfirm or Established, and any other state goes to `peer.acceptConnection`, where `Session.Accept` returns `ErrAlreadyConnected` and the inbound is parked for the next cycle or closed. The gate gains Connect and Active while the DelayOpenTimer runs, so the inbound takes the pending slot, `handlePendingCollision` reads its OPEN under the `ReceiveHoldTime` deadline, and the held outbound keeps its DelayOpenTimer. Then, by which OPEN arrives first: (a) the peer's OPEN on the held outbound is Event 20, Ze sends OPEN and KEEPALIVE and moves to OpenConfirm, and the pending inbound's later OPEN is resolved against that OpenConfirm connection by §6.8 through `resolvePendingCollision` (the lower BGP Identifier's side closes its existing connection; both orders tested); (b) the peer's OPEN on the pending inbound while the outbound is still OPEN-less (owner decision, 2026-10-11: close at once): `Session.detectCollision` today answers accept-new and keep-existing for Idle, Connect, Active and OpenSent (`session.go`), which `resolvePendingCollision` turns into rejecting the pending inbound. Its Connect and Active arm changes: with the DelayOpenTimer running (a held connection that sent no OPEN) it answers accept-new and close-existing; without it, and in Idle and OpenSent, today's answer stays. The held outbound is then closed by a new `Session.closeHeldForCollision`, not by `CloseWithNotification`: it fires Event 23 (OpenCollisionDump) into the FSM, whose Connect and Active arm for "any other events (Events 8, 10-11, 13, 19, 23, 25-28)" stops and zeroes the DelayOpenTimer and the ConnectRetryTimer (Connect's list names both; Active's names only "sets the ConnectRetryTimer to zero", so the Active arm's comment quotes Active's own text and clearing the DelayOpenTimer there is Ze's housekeeping on the way to Idle, round-4 N1), drops the TCP connection, increments the ConnectRetryCounter by one, and moves to Idle (RFC 4271 §8.2.2, Connect and Active). That arm sends no NOTIFICATION, so none is sent: no Cease and no subcode on a connection that never carried an OPEN, and no NOTIFICATION counter moves. It sets the close reason `ErrCollisionDump`, so `Session.Run` returns it and `run()` takes the ordinary error arm; `resolvePendingCollision` has already stored the inbound and its OPEN in the inbound slot (`storeInboundLocked`), whose `inboundNotify` wake-up ends that arm's backoff wait at once with `delay` reset to `reconnectMin` and no doubling. If the FSM has left Connect and Active before the close runs (the peer's OPEN reached the held outbound first), `closeHeldForCollision` closes nothing and reports it, and `resolvePendingCollision` asks `detectCollision` again, which answers by §6.8 as in order (a). A close-existing answer keeps the stored inbound and the reservation and closes the existing session with `CloseWithNotification`, today's path. A keep-existing answer (Ze's BGP Identifier is higher) takes the slot back (round-4 I1): under `p.mu`, only while `p.inbound.conn` is still this inbound, it clears the slot, clears `collisionHandoff`, and drains a pending `inboundNotify` wake-up, if any, with a non-blocking receive (a `select` with a `default` arm), because a blocking receive under `p.mu` hangs when the run loop consumed the token first; after unlocking it closes the inbound socket. The token drained is not always the one this store posted: `runOnce` takes the slot without consuming the channel, so a stale token from an earlier store can fill the one-slot buffer and make this store's post a no-op. Draining it is harmless, because a stale token only cuts a later backoff short (round-5 N1, N2). A slot `runOnce` has already taken (the session ended for another reason) now belongs to `runOnce`, so the take-back leaves it alone. If the existing session ends between the store and the re-ask, the re-ask runs against a session now in Idle, which answers reject, and the take-back closes an inbound that would have been the right one to keep; the peer re-dials, so this costs one connection cycle and never correctness, and no guard is added for it (round-5 N3). Decision: store first and take back on reject, not close before storing, because the store-before-close order is the invariant `resolvePendingCollision` already documents ("Publishing before CloseWithNotification prevents runOnce from passing its inbound check without the winner", `peer_connection.go`): closing first opens a window where `run()` reaches the passive `takeInboundConnection` in `runOnce` with the slot still empty and dials instead, and closing that window needs a timing argument, while the take-back is three assignments under the lock already held. The next cycle starts with Event 6, because the closed cycle never reached Established and `cycleReachedEstablished` is false (AC-2), takes the inbound slot, and runs `acceptWithOpen`, which is AC-1h; (c) Ze's DelayOpenTimer expires first: Event 12 sends Ze's OPEN on the outbound (OpenSent) and (a) or §6.8 against OpenSent follows as today. Expiry of the pending inbound's read deadline (`ReceiveHoldTime`, 90 s by default, shorter than a peer DelayOpenTime of up to 240 s) closes only the pending connection, never the held one. In every order exactly one session reaches Established. An active-only peer (`connection active`) never meets AC-1i: `acceptOrReject` closes every inbound before the state gate (`!settings.Connection.IsPassive()`), so only Ze's outbound exists and the peer's own dial is refused |
| AC-2 (row 7) | A peer session that reached Established ends, by a received NOTIFICATION, by HoldTimer expiry (both error returns from `Session.Run`), or by `ErrTeardown`, and the reconnect loop starts the next cycle, after the backoff timer or after `inboundNotify` cuts it short | The FSM receives Event 3 (AutomaticStart) and the ConnectRetryCounter is zero, for each of the three exits. Negative polarity: the cycle after a failed attempt (no Established), including the cycle `inboundNotify` restarts, fires Event 6 and keeps the counter (`TestRFC4271ConnectRetryCounterSurvivesDampedStart` stays green). The operator's first start stays Event 1. The predicate is the `cycleReachedEstablished` flag the FSM state callback sets on entering Established and `runOnce` clears before each session, read by `run()` whatever `safeRunOnce` returned (Data Flow step 2), never a second timer and never the loop's error arm. The prefix-limit arm of `run()` is one more exit after Established: `PrefixReconnectTimer` waits its own delay and continues, and the next cycle takes Event 3 like the others; `PrefixReconnectNever` returns from `run()`, so no next cycle starts and no event fires (round-3 N3) |
| AC-2p (row 7, passive) | The same, on a peer with `connection passive` | Ze fires Event 3 and the FSM goes to Active, following the existing Event 1/Event 4 conflation in `fsm.go` (one arm, `passive` chooses Connect or Active), extended to Event 3 in the same arm; the arm's comment states the conflation. Event 5 is not added, because nothing in Ze fires it. The counter is zero |
| AC-3 (ledger) | Each row once its AC is met | The row in `rfc/short/rfc4271.md` loses `{gap}`, carries tags in both polarities, the HEAD-tagged 8.2.2-13 units are re-pointed at the real-`Peer` test (R50), and each carries a discrimination record (`./le rfc record`); `./le rfc check` exits clean for `rfc4271` |
| AC-4 (row 6s, enter) | DelayOpen TRUE, `connection bfd { strict true }` negotiated, BFD neither Up nor AdminDown, the peer's OPEN arrives while the DelayOpenTimer runs, in Connect and in Active | Ze sends its OPEN and NO KEEPALIVE; with a non-zero HoldTimer initial value the KeepaliveTimer is not started and the HoldTimer is reset to the negotiated value; with zero the KeepaliveTimer and HoldTimer reset and the BfdHoldTimer starts with BfdHoldTime; the state stays Connect or Active and the FSM reports `ConnectDelayOpenBfdUpPending` or `ActiveDelayOpenBfdUpPending` (§8.3.5, §8.4.5) |
| AC-5 (row 6s, leave) | In either DelayOpen sub-state, BfdUp, BfdAdminDown or BfdDisabled arrives | Ze sends a KEEPALIVE, with a non-zero HoldTimer starts the KeepaliveTimer and zeroes the BfdHoldTimer, and moves to OpenConfirm, leaving the sub-state (§8.3.1, §8.4.1 first branch). Outside the sub-state the same events keep today's "stays in the Connect/Active state" |
| AC-6 (row 6s, second OPEN) | In either DelayOpen sub-state a second OPEN arrives | Ze sends a NOTIFICATION with Finite State Machine Error, zeroes the ConnectRetryTimer, releases resources, drops the connection, increments the ConnectRetryCounter and goes to Idle (§8.3.5, §8.4.5 new text) |
| AC-7 (row 6s, timers and config change) | In either DelayOpen sub-state the BfdHoldTimer expires, or `bfd strict` changes | Today's Connect and Active arms run unchanged: Cease / BFD Down and Idle (§8.3.3, §8.4.3), or drop and Idle with the counter zero (§8.3.4, §8.4.4); the sub-state is cleared on every transition out |
| AC-7h (row 6s, HoldTimer bound, round-2 I3) | In either DelayOpen sub-state with a non-zero negotiated hold time, BFD stays Down and no KEEPALIVE is sent | The HoldTimer runs while the state is still Connect or Active (AC-4 resets it to the negotiated value on entering the sub-state); on expiry, HoldTimer_Expires (Event 10) takes the Connect or Active catch-all arm: the ConnectRetryTimer is zeroed, resources released, the connection dropped, the ConnectRetryCounter incremented by one, the sub-state cleared, and the state is Idle. The session never stays in the sub-state past the negotiated hold time |
| AC-8 (row 6s, strict-mode path refusal) | `EnterBfdUpPending` or its Connect/Active sibling is called in a state the draft does not allow | It returns the error it returns today for an OpenSent sub-state outside OpenSent; the Connect/Active sub-states are accepted only in Connect and Active with the DelayOpenTimer just cleared by Event 20 |
| AC-9 (row 6s, ledger and comments) | Row 6s landed | `rfc/short/draft-ietf-idr-bgp-bfd-strict-mode.md` lines 14 and 20 no longer call the sub-states unreachable and name the producing functions; no requirement row is added because §8 carries no RFC 2119 keyword (its Notes); every comment and page listed in Current Behavior and the Documentation checklist that calls the sub-states unreachable is rewritten in the same commit; `./le rfc check` exits clean for the draft stem |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Sets `timer delay-open` on a peer facing FRR | config -> FSM DelayOpenTimer -> OPEN held -> session up | `bgp-delayopen-frr` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRFC4271DelayOpenHoldsOpen` | `internal/component/bgp/fsm/` | AC-1: OPEN held until peer OPEN or timer expiry; 0 leaves DelayOpen off | |
| `TestRFC4271ActiveEvent18WithDelayOpen` | `internal/component/bgp/reactor/` | AC-1: Event 18 in Active through a real `Peer`, four actions, both polarities | |
| `TestRFC4271AutomaticStartEvent3ZeroesCounter` | `internal/component/bgp/reactor/` | AC-2 both polarities through a real `Peer`: an Established session ended by a peer NOTIFICATION, by HoldTimer expiry and by `ErrTeardown` each zero the counter through Event 3 (the first two are error returns, so the test fails against a predicate tied to `runOnce` returning nil); after a failed attempt, and after `inboundNotify` cuts the backoff of a failed attempt, the counter is kept through Event 6; AC-2p passive goes to Active | |
| `TestRFC4271DelayOpenEventsConnect`, `TestRFC4271DelayOpenEventsActive` | `internal/component/bgp/fsm/` | AC-1a, AC-1b, AC-1c, AC-1d, AC-1f, AC-1g: one subtest per event, the quoted actions as assertions | |
| `TestRFC4271DelayOpenHeldConnectionWritesNoOpen` | `internal/component/bgp/reactor/` | AC-1a: no OPEN bytes on the wire while the DelayOpenTimer runs, through a real `Peer` | |
| `TestRFC4271DelayOpenPreBufferedOpen` | `internal/component/bgp/reactor/` | AC-1h, once per connection mode: through `acceptWithOpen` with DelayOpen TRUE, the wire carries no OPEN before the buffered OPEN is processed, then OPEN and KEEPALIVE, and the state goes Connect to OpenConfirm for `connection both` and Active to OpenConfirm for `connection passive`, with no OpenSent in either; with DelayOpen FALSE the old sequence in both modes | |
| `TestRFC4271DelayOpenBothSidesHold` | `internal/component/bgp/reactor/` | AC-1i, through a real `Peer` and two connections: the second inbound takes the pending slot while the DelayOpenTimer runs, in Connect and in Active; orders (a) with both BGP Identifier orders, (b) and (c); in (b) the held outbound is closed with no NOTIFICATION on its wire, the FSM logs Event 23 and Idle, the ConnectRetryCounter rises by one, the next cycle starts at once with Event 6 and runs `acceptWithOpen`; in (b) with the held outbound already in OpenConfirm, §6.8 decides instead, run in both BGP Identifier orders (round-4 I1): peer higher, the existing session is closed with Cease and the stored inbound runs `acceptWithOpen`; Ze higher, the inbound slot is empty, `collisionHandoff` is false, no `inboundNotify` wake-up is pending, the inbound socket is closed, and the existing session stays in OpenConfirm and reaches Established; the pending deadline closes only the pending connection; exactly one Established session in each; an active-only peer closes the inbound | |
| `TestSessionBFDStrictDelayOpenBoundedWait` | `internal/component/bgp/reactor/bfd_delay_open_test.go` | AC-6, AC-7, AC-7h: real timer callbacks and Session.Run retire socket/timers, counter and pending state; second OPEN refuses renegotiation; config close is silent | passed in scoped race selection; boundary mutations discriminate |
| `TestDraftBFDStrictDelayOpenSubStates`, `TestDraftBFDStrictDelayOpenFailures`, `TestEnterDelayOpenBfdUpPendingRefusesInvalidEntry` | `internal/component/bgp/fsm/bfd_delay_open_test.go` | AC-4, AC-5, AC-6, AC-7, AC-8 in Connect and Active, hold zero/nonzero and all three releases | passed in scoped race selection; boundary mutations discriminate |
| `TestSessionBFDStrictDelayOpenRelease`, `TestSessionBFDStrictDelayOpenNonwaitingControls`, `TestPeerBFDStrictDelayOpenRelease` | `internal/component/bgp/reactor/bfd_delay_open_test.go` | AC-4, AC-5 wire half: both OPEN processing rails, all releases, nonwaiting controls and actual Peer Connect/Active cycles | passed in scoped race selection |
| `TestSessionBFDStrictDelayOpenImmediatePeerKeepalive` | `internal/component/bgp/reactor/bfd_delay_open_release_race_test.go` | AC-5: immediate peer reply cannot race BFD release into the Connect/Active catchall | observed red before `handleKeepalive` serialization; repaired race selection passed |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `timer delay-open` (seconds) | 0 (off) or 1..240, FRR's range | 240 | N/A (0 is valid: off) | 241 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `bgp-delay-open` | `test/draft/plugin/bgp-delay-open.ci`; not promoted | AC-1, AC-1a, AC-1b, AC-1c through the daemon listener and operational FSM state | original promoted TIME30 unresolved; later diagnostic passes do not clear it |
| `bgp-delay-open-bfd-strict` | `test/plugin/bgp-delay-open-bfd-strict.ci` | AC-4, AC-5: daemon OPEN exchange, public pending sub-state, withheld KEEPALIVE, real BFD Init datagram causes Up and establishment | draft GREEN–RED–GREEN and stress80 passed after runner repair; promoted live selector bg_508 FAILED TIME30 case124, acceptance not cleared |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `bgp-delayopen-frr` | `test/interop/scenarios/` | FRR (`neighbor X timers delayopen (1-240)`) | With `timer delay-open` set on Ze, the capture shows Ze's OPEN only after FRR's OPEN (or after the DelayOpenTimer), and the session establishes; with FRR's own delayopen set as well, both hold and the session still establishes | |

Row 7 is not wire-visible; its proof is the reactor unit test through a real `Peer`. The scenario owes the revert, rebuild, red, restore, green walk with the red recorded.

## Files to Modify
- `internal/component/bgp/fsm/fsm.go`, `internal/component/bgp/fsm/state.go`, `internal/component/bgp/fsm/timer.go`, `internal/component/bgp/reactor/session.go` - rows 6, 7, 6s (the fsm.go header comment that records DelayOpen and Event 3 as absent changes with the code; every comment listed in Current Behavior that calls the DelayOpen sub-states unreachable is rewritten; the DelayOpenTimer joins the timers in timer.go; state.go declares the two Connect/Active sub-states)
- `internal/component/bgp/reactor/peer_run.go`, `internal/component/bgp/reactor/peer.go` - row 7: `Peer` gains `cycleReachedEstablished`; `runOnce` clears it before the session starts; the FSM state callback sets it beside `PeerStateEstablished`; `run()` reads it after `safeRunOnce` whatever `err` is, and the start-event choice around line 379 fires Event 3 or Event 6 from it
- `internal/component/bgp/reactor/session_connection.go` (`acceptWithOpen`, `processOpen`, and the new `closeHeldForCollision`), `internal/component/bgp/reactor/peer_connection.go` (`resolvePendingCollision`: calls `closeHeldForCollision` for a held existing connection, re-asks `detectCollision` when it closed nothing, and on a keep-existing answer takes the slot back: clears it, clears `collisionHandoff`, drains a pending wake-up with a non-blocking receive, closes the inbound socket; round-4 I1, round-5 N1), `closeHeldForCollision` and `CloseWithNotification` built on one helper so the Event 23 and `ErrCollisionDump` handling is declared once, the NOTIFICATION write the only difference (round-4 N2), `internal/component/bgp/reactor/reactor_connection.go` (`acceptOrReject`'s state gate: Connect or Active with the DelayOpenTimer running also takes the pending path, read through a new `Session` accessor for "DelayOpenTimer running" taken under the same lock as the state, which `detectCollision`'s new arm reads too (round-4 N3); `handlePendingCollision`), `internal/component/bgp/reactor/session.go` (`detectCollision`'s Connect and Active arm: with the DelayOpenTimer running it answers accept-new and close-existing; Idle, OpenSent, and Connect or Active without it keep today's answer) - row 6, round-2 I1, round-3 B1: a pre-buffered OPEN under DelayOpen takes the Event 20 arm; a second connection while the DelayOpenTimer runs (AC-1h, AC-1i)
- `internal/component/bgp/reactor/config.go` (timer container parse, around line 244), `internal/component/bgp/reactor/peer_settings.go` (timer setting and default, around line 69) - row 6
- `internal/component/bgp/reactor/session_connection.go`, `internal/component/bgp/reactor/session_negotiate.go` - row 6: `connectionEstablished` holds the OPEN when DelayOpen is TRUE; `sendOpen` runs on Event 20 or Event 12
- `internal/component/bgp/reactor/session_bfd_strict.go` - row 6s: `advanceAfterOpen` and `handleBFDEvent` extended to the Connect and Active DelayOpen sub-states
- Test comments listed in Current Behavior (`fsm/bfd_strict_test.go`, `fsm/rfc4271_connect_any_other_event_test.go`, `fsm/rfc4271_connect_retry_test.go`) - their "no DelayOpenTimer" sentences become false
- `internal/le/interoplab/bgp/check_special.go` - map entry for `bgp-delayopen-frr`
- `rfc/short/draft-ietf-idr-bgp-bfd-strict-mode.md` - AC-9
- `internal/component/bgp/yang/ze-bgp-conf.yang` - row 6: grouping `peer-fields`, container `timer`, new leaf `delay-open` (uint16 seconds, range `0 | 1..240`, default 0 = DelayOpen FALSE), so `bgp peer <address> timer delay-open <seconds>` and the group and bgp levels through the grouping
- `docs/architecture/behavior/fsm.md` - rows 6, 7
- `rfc/short/rfc4271.md` - AC-3

## Files to Create
- `test/plugin/bgp-delay-open.ci`
- `test/interop/scenarios/bgp-delayopen-frr/`
- `test/plugin/bgp-delay-open-bfd-strict.ci`
- `internal/le/interoplab/bgp/check_delayopen.go` (and its `_test.go`) - the `bgp-delayopen-frr` checker: parses the capture and asserts Ze's OPEN follows FRR's OPEN, or follows the TCP handshake by at least DelayOpenTime when FRR holds too

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `ze-bgp-conf.yang` leaf `timer delay-open` |
| YANG validation constraints | Yes | `delay-open` range 0 or 1..240 |
| YANG custom validators | N-A | the range is a YANG constraint |
| CLI commands/flags | N-A | no command changes |
| CLI grammar (keyword before value) | N-A | no command changes |
| Editor autocomplete | Yes | the new leaf comes from YANG |
| Functional test for new RPC/API | N-A | no new RPC; `bgp-delay-open.ci` covers the behavior |
| Pipe completeness | N-A | no command output |
| Env var registration | N-A | none |
| Doctor check for runtime dependencies | N-A | no runtime dependency |
| Prometheus counters/metrics | No | the FSM state metrics already exist; confirm at implementation |
| BGP family surface (new SAFI / capability / attribute) | N-A | none |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | `features/bgp-protocol.md` if its evidence names these rows (check `./le feature report`) |
| 2 | Config syntax changed? | Yes | `docs/config-reference.md` and `docs/guide/configuration.md`, which list the `timer` leaves (`connect-retry`) |
| 3 | CLI command added/changed? | No | none |
| 4 | API/RPC added/changed? | No | none |
| 5 | Plugin added/changed? | No | none |
| 6 | Has a user guide page? | Yes | `docs/guide/bgp-peering.md` if it describes session timers |
| 7 | Wire format changed? | No | the OPEN is unchanged; only its timing |
| 8 | Plugin SDK/protocol changed? | No | none |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc4271.md`, `rfc/short/draft-ietf-idr-bgp-bfd-strict-mode.md` (row 6s), and `docs/features/rfc-status.md` (generated by `./le rfc update`) |
| 10 | Test infrastructure changed? | No | none |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` if it lists DelayOpen |
| 12 | Internal architecture changed? | Yes | `docs/architecture/behavior/fsm.md` (rows 6, 7 implemented; "The two BFD strict-mode sub-states" becomes all four, row 6s), `docs/architecture/behavior/fsm-active.md` (line 47 Event 18 row and the Event 12, 16/17, 20, 24 DelayOpen arms), `docs/architecture/behavior/fsm-connect.md` (the Connect DelayOpen arms), each edited in the phase whose code makes it wrong |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | No | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | none |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `docs/architecture/behavior/fsm.md` (affected). Re-derive at implementation with `./le spec citation anchors spec plan/post-release/features/spec-bgp-fsm-delayopen-and-automatic-start.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | the `timer` examples in the pages of item 2 |

## Implementation Steps

1. **Phase: Wiring** -- `bgp-delay-open.ci` and the Event 3 reactor test written and red.
2. **Phase: DelayOpen (row 6)** -- YANG leaf, config, DelayOpenTimer in Connect and Active, Event 18 in Active; `fsm.md` edited. Own commit.
3. **Phase: Event 3 (row 7)** -- the automatic restart picks Event 3 or Event 6 from the Established flag of the cycle that ended; passive peers through the Event 1/4 arm. Own commit.
3a. **Phase: strict-mode DelayOpen sub-states (row 6s)** -- after the DelayOpen phase: declare the two sub-states, extend `advanceAfterOpen` and `handleBFDEvent`, rewrite every comment and page listed in Current Behavior, edit the strict-mode summary prose (AC-9). Own commit.

Phase 3a implementation and evidence (2026-10-27): `EnterDelayOpenBfdUpPending`
atomically consumes strict Event 20 under the FSM lock and leaves the main
state in Connect/Active. Both OPEN rails share `advanceAfterOpen`'s strict
decision. BFD release, ordinary hold expiry and pre-established KEEPALIVE
receive serialize with OPEN entry under `openMu`; silent Connect/Active config
teardown shares the session lifecycle with notified teardown.

The supervising phase reports 34 named FSM/reactor race units passing and
accepted independent code reviews. Seven isolated boundary mutations failed
their assertions and passed after restoration. Native draft proof completed
GREEN–RED–GREEN. The initial stress attempt failed on invocation 7; after the
separately committed cross-process runner exclusion repair `16245cda84`, the
authorized unchanged stress80 command passed all 80 invocations. Evidence is
retained in `tmp/strict-delayopen-phase3-native/session-state.md` (latest section
supersedes its initial stress verdict) and
`tmp/strict-delayopen-claim-proof/expanded-final-receipt.json`.

The RFC receipt accounts for 45 native records across 23 files (two approved
claim-comment records plus 43 attributable refreshes), with no owned stale
record in its final scoped reports. Thirteen foreign stale records remain;
the unscoped `./le rfc check` was not run and a clean ledger is not claimed.
Section 8 adds no normative requirement row. Promotion changes no tagged unit,
producer or RFC record identity: the untagged strict carrier is copied
unchanged to `test/plugin/bgp-delay-open-bfd-strict.ci`. After matching SHA256
identities, the untracked original was moved out of draft discovery to this
session's `scratch/strict-draft-before-promotion.ci`; no tracked removal is owed.

Main's promoted live selector `bg_508` failed TIME30, case124: expected/received
0, with only the exec step passing. The failed workdir is
`/home/thomas/.cache/tmp/ze-work-3685380489`. Draft GREEN–RED–GREEN and stress80
remain valid historical evidence, but do not clear this live failure. Main
owns evidence preservation, diagnosis and runtime capture; no rerun to confirm,
carrier relocation or RFC-record change follows from this result.

The strict implementation and its owned proof records landed as `c987777bb8`,
with this live failure named in the commit. The runner had already removed
the failed workdir. Its exec step is recorded before process start, so that
step alone does not establish that the daemon ran. One timing trace and ten
untraced deadline-capture attempts subsequently passed without producing a
stall or failure stacks. These diagnostic passes do not clear TIME30.
The session retains `scratch/strict-delayopen-functional/exec-events-20261027-135508/`
and `scratch/strict-delayopen-functional/untraced-20261027-140225/`.
No deadline, assertion or product behavior changed for these diagnostics;
no privileged command is required.

Main also owns applicable BGP/plugin integration gates, source-anchor index
maintenance and committed `./le verify changed`.
This spec stays in progress: base `bgp-delay-open`'s original promoted TIME30
is unresolved and its carrier is not promoted. Event 3 reconciliation, FRR
interop and whole-spec ledger/closure obligations remain independent; none is
certified by the strict-mode evidence above.

4. **Phase: interop** -- `bgp-delayopen-frr` with the recorded red.
5. **Phase: ledger** -- AC-3, discrimination records, `./le rfc check` clean for `rfc4271`.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | `delay-open 0` leaves every existing FSM test green |
| Correctness | A damped restart still keeps the ConnectRetryCounter |
| Rule: rfc-compliance | Each new implementing function quotes its RFC sentence; each caller names the section |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| No `{gap}` left on rows 6, 7 | `grep -n` each id in `rfc/short/rfc4271.md` |
| Both polarities tagged | `grep -rn "RFC requirement: <id>" internal/ test/` |
| Ledger clean | `./le rfc check` exit status for `rfc4271` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | `delay-open` range enforced at commit |
| Resource exhaustion | A held connection is bounded by the DelayOpenTimer and the hold timer |
| Error leakage | none: no peer-supplied bytes parsed differently |

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

## Owner decisions (2026-10-11)

| Question | Owner's words | What it means here |
|----------|---------------|--------------------|
| Row 6: scope DelayOpen in, or decline it? | IN | AC-1: a `timer delay-open` leaf, the DelayOpenTimer, and `bgp-delayopen-frr` |
| Row 7: add Event 3, or record Event 6 as Ze's automatic start by design? | IN | AC-2: Event 3 starts every cycle that follows a cycle which reached Established, whichever backoff arm that cycle's exit takes; Event 6 starts every other cycle |
| DelayOpen with `bfd strict`: implement the two strict-mode DelayOpen sub-states (draft §8.3.5, §8.4.5), or refuse `delay-open` together with `bfd strict` at commit? (raised by the independent design review, B1) | "Implement them" | Row 6s and AC-4 to AC-9: `ConnectDelayOpenBfdUpPending` and `ActiveDelayOpenBfdUpPending` are declared and implemented, and every comment and page that calls them unreachable is corrected |
| With DelayOpen on both sides, when the peer's OPEN arrives first on the inbound connection while Ze holds its outbound with no OPEN sent: close Ze's held outbound at once, or keep both until one reaches Established as FRR does? (round-3 design review, Q1; Thomas, 2026-10-11) | "ok" to the recommendation, after comparing FRR and BIRD | AC-1i order (b): Ze closes its held outbound at once and hands the inbound to the session. Key Design Decisions carries the rationale |
| Approve the design after the round-5 design review (0 BLOCKER, 0 ISSUE, 3 NOTE, all folded)? (Thomas, 2026-10-11) | "Approve" | Status moves from `design` to `ready`, so `/ze-implement` may claim it |
| One spec or several? | "Split into three" | This spec owns rows 6 and 7; the defects stay in `plan/release/fixes/spec-bgp-unowned-rfc-rows.md`, EPE is `plan/post-release/features/spec-bgp-epe-peer-segments-and-srv6-peernode.md` |

## Open Questions (owner)

None open.

## Design Insights

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| `delay-open` range 0 or 1..240, 0 meaning off | A separate enable leaf | FRR's range; one leaf carries both the attribute and the timer |
| Event 3 vs Event 6 read from the cycle's Established flag | A second restart timer | A second timer would be a second declaration of the same fact; the flag records the one fact Event 3 needs, that the cycle reached Established |
| Two OPEN-less held connections are not a collision; the first OPEN decides (AC-1i, round-2 I1) | Close the inbound at once as a collision; keep both and compare in OpenSent | RFC 4271 §6.8 compares BGP Identifiers "Upon receipt of an OPEN message", so it has nothing to compare before one arrives; the existing pending slot already waits for the inbound OPEN, and closing an OPEN-less held connection loses no state |
| When the peer's OPEN arrives first on the inbound, Ze closes its OPEN-less held outbound at once (AC-1i order b; owner, 2026-10-11) | Keep both connections until one reaches Established, as FRR does | RFC 4271 §6.8 governs only "connections that are in the OpenConfirm state" and, optionally, OpenSent, so dropping an OPEN-less Connect or Active connection is outside it and not forbidden by it. BIRD's `bgp_rx_open` (`proto/bgp/packets.c`) stops the other connection when it is still in Connect or Active ("Stop outgoing connection attempts"). FRR keeps both through its doppelganger peer: `bgp_collision_detect` (`bgpd/bgp_packet.c`) finds no collision unless the other connection is OpenSent or OpenConfirm, and the loser is discarded only at `bgp_establish` through `peer_xfer_conn` (`bgpd/bgp_fsm.c`). Ze holds one connection per session (`Session.conn`), which is BIRD's shape; keeping two OPEN-less connections would need a second connection tracked under one session. Ze therefore diverges from FRR here, by choice |
| Event 3 predicate: an explicit `cycleReachedEstablished` flag the session sets on entering Established, read by `run()` whatever the error (round-2 B1) | "The peer has not been oscillating", which Ze keeps as no state; the backoff delay value; the loop's `runOnce`-returned-nil arm | The nil arm runs only when the peer is stopping, because `Session.Run` returns non-nil on every exit, so a predicate tied to it never fires; `inboundNotify` resets the delay without proving a session ran, so the delay value would misfire Event 3. The flag is set where the fact happens, the FSM's Established transition, and read where the decision is made |
| Passive peers take Event 3 through the existing Event 1/4 conflation | A distinct Event 5 path | `fsm.go` already serves Events 1 and 4 from one arm with `passive` choosing the next state; adding Event 3 to that arm keeps one declaration, and the arm's comment names the conflation. Event 5 is not added: nothing in Ze fires it (round-3 N2) |
| Event 18 in Active follows the RFC's four actions | FRR's `bgp_ignore` for TCP_connection_open_failed in Active | The row is RFC4271-8.2.2-13; FRR is laxer than the RFC here and the RFC outranks the peer implementation |
| Row 6s implemented, not refused (owner, 2026-10-11) | Refuse `delay-open` with `bfd strict` at commit | Owner decision; the strict-mode decision (`advanceAfterOpen`) is reused so the "BFD neither Up nor AdminDown" test lives in one place |

## Known Limitations
- None beyond the FRR/BIRD comparison: BIRD has no DelayOpen, so the interop proof is against FRR only.

## RFC Documentation (Scope: protocol)

Add `// RFC 4271 Section 8.2.2: "<quoted requirement>"` above enforcing code, using
the quotes in the Task section.

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
- [ ] AC-1, AC-1a..AC-1g, AC-2, AC-2p, AC-3..AC-9 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/post-release/features/spec-bgp-fsm-delayopen-and-automatic-start.md` only, in the same `./le commit create` script
