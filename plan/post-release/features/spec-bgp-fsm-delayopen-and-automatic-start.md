# Spec: bgp-fsm-delayopen-and-automatic-start -- DelayOpen and Event 3 in the BGP FSM

| Field | Value |
|-------|-------|
| Status | in-progress |
| Priority | P3 |
| Release reason | Optional DelayOpen and automatic-start controls are owner-selected follow-up capabilities, not required behavior of the shipped FSM. |
| Scope | protocol |
| Depends | - |
| Phase | 5/5 — selected-scope closure review complete; native publication owed |
| Handoff | `tmp/session/2026-11-15-193834/state/session-state-bgp-fsm-delayopen-and-automatic-start-01a1e0ac-0f28-7471-8c00-742a752c8144.md` |
| Updated | 2026-11-17 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The BGP child of the rfc-verdict pass (spec-rfc-verdict-fix-bgp; its
"Gap accounting" section survives in git history) left requirement rows that no spec owned.
One spec took them (`spec-bgp-unowned-rfc-rows`). On 2026-10-11 the
owner put DelayOpen and Event 3 IN and split the work: "Split into three". This
spec owns the two RFC 4271 FSM rows. The defects stay in the immediate spec; the
EPE rows are `plan/post-release/features/spec-bgp-epe-peer-segments-and-srv6-peernode.md`.

The same-Session capture work also repairs the removal boundary it exercises:
an old Session factory must not register a capture after its Peer is removed,
so it cannot suppress a replacement Peer's configured trace at that address.

The behavioral reassessment also corrects excess incoming admission refusal:
capacity exhaustion before OPEN uses Cease / Connection Rejected (6/5), while
actual OPEN-based collision losers retain Cease / Connection Collision (6/7).
It preserves the admitted exchange and the healthy incumbent.

Closure review also repairs provisional OPEN validation: a competing incoming
connection must not change the incumbent's learned plugin state. Selection
publishes its retained OPEN pair once, after incumbent cleanup. The extracted
frame reader must retain pooled storage without allocating a wrapper for each
message.

Reload also checks independently frozen incoming offers across construction,
handoff and selected validation. It retires an offer whose changed capability
is in use, while preserving unused or unchanged offers and the immutable OPEN
already sent. The existing peer-change estimate continues to count peer
changes, rather than private connection retirement.

The shared-pool stress proof also repairs two ownership defects. An incoming
connection waits for read storage before and after OPEN, then resumes on its
original socket and reader. Its original deadline and cancellation still end
the wait. Failed receive publication releases adopted receipts and queued
buffers, while an existing retained consumer keeps its original receipt and
forwarding buffers until its final release. Neither repair raises pool limits
or resets shared pools to hide retained ownership.

Passive startup also preserves an incoming connection that arrives before the
selected Session can process TCP success. Initialization may delay its
handshake within the existing lifecycle bounds; it must not lose the
connection event, emit a duplicate OPEN or make the peer reconnect. The same
connection and its owning Session complete the ordinary exchange once startup
can proceed.

The final review also covers received UPDATE admission before establishment.
A connection waiting for OPEN or strict BFD must reject an UPDATE before it
can publish routes through a selected peer's delivery path. An independent
candidate's refusal preserves the incumbent and its useful routes; established
connections still process valid UPDATEs.

Verification also repairs the live-router test fixture's teardown: after its
reactor, plugin engines, workers and server handlers join, it restores the
initially absent validation provider for the next test's fictional daemon.

Row numbers are the numbers the rows had in the parent spec.

| # | Row id | Document, section | Class | What Ze does today |
|---|--------|-------------------|-------|--------------------|
| 6 | `RFC4271-8.2.2-13` | RFC 4271 §8.2.2 (Active) | implemented; native clause proof renewed and independently audited | `Session.handleConnectionCloseLocked` retains the actual retry deadline before `StopAll`. `Peer.run` uses its remaining interval. Held Active and strict BFD-pending retry units passed under race, with an ordinary OpenSent control. All three native records are current and the independent audit is enforced; the obsolete retry gap is removed. Both functional carriers and pinned FRR qualification passed. The final ledger check has no finding for this row; broader active-task findings remain. |
| 7 | `RFC4271-8.2.2-20` | RFC 4271 §8.2.2 (Idle) | implemented; native clause proofs renewed and independently audited | `Peer.runOnce` selects Event 3 after an Established cycle and Event 6 after a failed attempt. Its four affected real-Peer records were renewed against the final producer, and the independent audit is enforced. The final native inventory reports all eight row-20 records current, with no stale or unproven binding. The final ledger check has no finding for this row; broader active-task findings remain. |
| 6s | draft-ietf-idr-bgp-bfd-strict-mode §8.3.5, §8.4.5 (and the sub-state branches of §8.3.1, §8.4.1) | draft, Connect and Active | implemented; current-source carrier qualified | Both DelayOpen BFD-pending sub-states landed in `c987777bb8`. The current-source strict carrier passed clean–broken–clean, all 80 loaded draft invocations and its promoted Linux selection on Ze's matching kernel/image, with one test executed and zero skips. Candidate BFD delivery also passed clean–broken–clean and twenty incoming race repetitions. These results do not diagnose the historical promoted TIME30. |

Row 6s exists because row 6 makes the two strict-mode sub-states reachable: with
`timer delay-open` set on a peer whose `connection bfd { strict true }` is enabled,
an OPEN received while the DelayOpenTimer runs (Event 20) enters the draft's
revised Event 20 clause. On 2026-10-11 the owner, asked whether to implement them or
refuse `delay-open` together with `bfd strict`, answered: "Implement them". The
draft's Section 8 carries no RFC 2119 keyword (`rfc/short/draft-ietf-idr-bgp-bfd-strict-mode.md`,
Notes), so 6s adds behavior and tests but no MUST-level requirement row; AC-9
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
| incoming FSM (2026-11-15 recheck) | FRR `frr-10.7.1`: `bgp_accept` (`bgpd/bgp_network.c`) starts the doppelganger in Active with ordinary or delayed TCP-open; `bgp_collision_detect` (`bgpd/bgp_packet.c`) compares OpenSent identifiers; `bgp_establish` / `peer_xfer_conn` retain live transport state on promotion | BIRD `v3.4.0`: `bgp_incoming_connection_single` (`proto/bgp/bgp.c`) creates its incoming connection and sends OPEN; `bgp_rx_open` (`proto/bgp/packets.c`) continues beside another OpenSent, compares OpenConfirm and refuses against Established; `bgp_rx_keepalive` establishes. Ze follows this OpenSent policy because it has no remote identifier learned outside the protocol, as required for Section 6.8's optional OpenSent arbitration. Official sources: [FRR](https://github.com/FRRouting/frr/tree/frr-10.7.1/bgpd), [BIRD](https://github.com/CZ-NIC/bird/tree/v3.4.0/proto/bgp). This is source research, not an external interop run. |
| 6s | FRR has no BFD strict mode (`bgp-bfd-strict-frr` README: FRR 10.3.1 implements neither) | No BFD strict mode |

## Required Reading

### Architecture Docs

The constraints below record the approved design. The current FSM pages and
strict-mode summary describe the implemented DelayOpen paths and their current
source anchors. The bounded implementation and RFC evidence is recorded below.

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
  → Constraint: main-thread rulings R47(a) and R53(a) (`plan/records/rfc-verdict-test-fix-pass-rulings.md`) hold 8.2.2-20 as a counted gap because Ze has an automatic start, and 8.2.2-13 as a `{gap}` stamped unimplemented because the gate refused `{feature-declined}` beside existing tags. Both are now implemented (owner: IN), so both rows lose `{gap}` and owe tags in both polarities; R50 found the HEAD-tagged 8.2.2-13 units drive a synthetic Event 18 no real Peer reached, so those tags are re-pointed at a test that reaches Active with DelayOpen through a real Peer

**Key insights:**
- Row 7's Event 3 and the existing Event 6 are chosen from one explicit flag the session sets on entering Established, not from a second timer and not from the loop's error arm, which cannot tell a session that ran (round-2 B1).
- Row 6 is the only wire-visible change: the moment Ze sends its OPEN.

## Current Behavior (MANDATORY)

**2026-11-16 source checkpoint:** DelayOpen configuration, Connect/Active events,
strict-mode sub-states and Event 3 selection are implemented. Transport failure
retains the restarted retry deadline across Session cleanup. Admitted incoming connections
have independent Sessions and retain their original validated OPEN, timers,
reader and socket through selection. Each OPEN has one immutable proposal.
Capture retains the selected connection's original input and proposal identity,
and removal excludes late capture registration by an old Peer.

The later stress run exposed failed receive-publication ownership and incoming
pool-pressure resets. The targeted RED in `job-unit-pkg-b9083d6e.log` retained
one real standard read slot on nil receiver, one on receiver panic, three on
queued receiver panic and one on pre-handoff panic. Retained-consumer controls
also left lookup/retain admission open and failed to release their last cache
entry. Both incoming read rails returned EOF under pressure before the same
connection could resume. AC-1q/r own these corrections. Their four roots later
passed twenty race repetitions in `job-incoming-pool-controls20-4b9a9784.log`:
99 selected roots produced 1980 passes, no failure or race report, in 530.088
seconds. That checkpoint precedes the continuation correction below.
The follow-up `job-incoming-follow-panic-red-3c7a18c7.log` also confirmed that
a later heap-backed policy continuation lost the original buffer's earlier
transfer result: standard and coalesced readers held only 4096 of the required
8192 retained bytes, then reported a double return. The earlier successful
publication and its retained buffers remain permitted under AC-1r.
The passes and native-currentness counts below are earlier checkpoints, not
final verification of these corrections. Final producer proof renewal and
independent closure remain owed.

The role daemon stress also exposed an initial-destination EOF. Its observer
had assumed `OnAllPluginsReady` followed peer startup; the documented contract
permits the opposite order. A judged fixture precondition now waits for both
configured passive peers to report ACTIVE before starting its role scenario.
That precondition is not a product fix or proof of the EOF's exact cause.
Separately, `Peer.runOnce` publishes a fresh Idle Session before its start;
an early `Session.connectionEstablished` can send OPEN while TCP-confirmed
is ignored, then the later start leaves remote OPEN unable to complete.
`TestPassiveIncomingAcrossSessionStart` proved this race in
`job-incoming-passive-start-red-09d2d0cd.log`: the gated startup received EOF
while waiting for KEEPALIVE, while the ordinary passive-start control passed.
The package failed in 3.389 seconds without a race report. Starting before
publication repaired that handshake but left its public OpenSent transition
unobserved while the initial Peer-state callback was paused. The strengthened
actual-IPC observer isolated this in
`job-incoming-start-complete-observer-red-09d2d0cd.log`: the gated case
received connected, OpenConfirm and Established, but no OpenSent event;
ordinary passive startup passed. The package failed in 10.084 seconds.
AC-1s owns both behaviors; source verification remains pending.

Targeted race runs passed for retry, ingress refusal, hold-deadline admission,
selected identity and capture ownership. Twelve ownership boundaries also passed
twenty repetitions each (240 named passes), and the retry-budget timer passed
twenty repetitions. After the collision panic repair, fifteen affected incoming
roots each passed twenty repetitions (300 named passes), with no failure, skip
or race report. The untagged panic regression observed the old lock leak, then
passed with the scoped `Peer.mu` release; the original collision discrimination
proof now completes its expected clean PASS and broken FAIL without hanging.
All six retry-budget and ordinary-backoff roots also passed twenty race
repetitions each (120 named passes), retaining the original clock deadline.
The four final expiry, adoption-lock, BFD-delivery and connected-metadata cuts
failed their intended assertions, and all four restored roots passed under race.
Scoped reactor lint analyzed the normal, Linux integration and capability
flavors with no owned finding. The later shared reactor analysis is clear;
the remaining interop lint findings belong to other active tasks.
Both functional carriers passed clean–broken–clean, eighty loaded draft runs
and their promoted live selections. The FRR scenario passed clean–broken–clean
against pinned FRR 10.7.1 with complete wire captures and image identities.

The full FSM race run passed. The full reactor race run reported 121 failing
roots and timed out; the owning state holds the single known-red list and its
owner, baseline and correction evidence. Three admission-fixture failures now
pass their targeted race rerun, and live-router fixture teardown no longer
leaves its validation owner in the next test's baseline. Incoming behavior is
assessed through OPEN exchange and failure isolation; bounded admission is a
capacity policy, not an FSM object-count gap. Core rows 13 and 15 have current
proofs and independent audits, as does row 20 after its final four producer
renewals. Native inventories across fifteen stems report all 85 owned proof
bindings current, with no owned stale or missing binding. The ledger update
completed; its check reported broader active-task violations. Four additional
owned-row audit file hashes were mechanically refreshed without changing any
unit, requirement, producer, note or verdict. The final post-refresh check
reports 339 broader active-task violations, with no finding for the 55 selected
audits or core rows 13, 15 and 20. It does not establish a green whole ledger.
The historical baseline below does not state the current implementation or
diagnose an earlier TIME30. Independent closure remains a separate step.

**Source files in the design-time baseline:**
- [ ] `internal/component/bgp/fsm/fsm.go`, `internal/component/bgp/fsm/state.go` - Event 6 implemented as the reactor's exponential backoff (state.go line 162); Events 3 and DelayOpen absent by recorded design (fsm.go header lines 16-18, 77)
- [ ] `internal/component/bgp/reactor/session.go` - line 1006-1011 fires Event 6 for an automatic restart
- [ ] `internal/component/bgp/yang/ze-bgp-conf.yang` - grouping `peer-fields`, container `timer` (`receive-hold-time`, `send-hold-time`, `keepalive`, `connect-retry`); no DelayOpen leaf
- [ ] `internal/component/bgp/reactor/peer_run.go` - the reconnect loop: `delay` is reset to `reconnectMin` on `ErrTeardown` and on `inboundNotify`, and doubled after any other error. The `else` arm that resets `delay` and `prefixTeardownCount` when `safeRunOnce` returns nil is reached only when the peer is stopping: `runOnce` returns `session.Run`'s error, `Session.Run` returns non-nil on every exit (`waitForResume` error, `*closeReason`, or the read error), and `runOnce`'s only `return nil` is its `stopping` check (round-2 B1; the missed resets are a journal row in `plan/journal/`, not this spec's fix). So the loop keeps no "session ran" outcome today; around line 379 the first cycle after the operator's start fires Event 1 (`session.Start`) and every later cycle Event 6 (`session.startDamped`), chosen by the `operatorStarted` swap
- [ ] `internal/component/bgp/fsm/fsm.go` lines 380-388 - Events 1 and 4 are conflated: one arm resets the counter and goes to Active when `passive` is set, Connect otherwise
- [ ] `internal/component/bgp/reactor/config.go` around line 244 (timer container parse), `internal/component/bgp/reactor/peer_settings.go` around line 69 (timer defaults, YANG as source of truth)
- [ ] `internal/component/bgp/reactor/session_connection.go` `connectionEstablished` (line 328) fires Event 17 at line 413 and the OPEN follows through `session_negotiate.go` `sendOpen` (line 130): the OPEN is sent immediately after the TCP connection today
- [ ] `internal/component/bgp/reactor/session_bfd_strict.go` - `advanceAfterOpen`, `handleBFDEvent`, `raiseBFDHoldTimerExpired`: the wire half of the strict-mode OpenSent sub-states, which row 6s extends to the Connect and Active sub-states
- [ ] Comments that call the DelayOpen sub-states unreachable, all false after row 6s: `internal/component/bgp/fsm/fsm.go` header lines 11 and 77, `EnterBfdUpPending` comment lines 232-240, the Connect BFD arm lines 503-512, the BfdHoldTimerExpires arm lines 531-532, the Event 24 arm lines 557-558 and 656, the Active sub-state arm lines 664-669; `fsm/state.go` lines 274-277; `fsm/timer.go` line 61; test comments `fsm/bfd_strict_test.go` line 274, `fsm/rfc4271_connect_any_other_event_test.go` line 48, `fsm/rfc4271_connect_retry_test.go` lines 320, 367, 396

**Behavior to preserve:**
- Event 6 backoff semantics and `TestRFC4271ConnectRetryCounterSurvivesDampedStart`.
- With `timer delay-open` unset (0), the OPEN timing and every existing FSM test unchanged.

**Behavior to change:** rows 6 and 7, as the ACs state.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec |
|-----------|-----------------------------------------|--------------------|-----------|
| DelayOpen FSM | `internal/component/bgp/fsm/delay_open.go`: `FSM.handleDelayOpen`; `internal/component/bgp/fsm/fsm.go`: `FSM.handleActive` | Holds the connection and handles the DelayOpen events; Active Event 18 restarts retry | Reuses these transitions and completes their live retry obligation; no parallel FSM |
| Timer ownership | `fsm/timer.go`: `Timers.StartConnectRetryTimer`, `Timers.ConnectRetryBudget`, `Timers.StopAll` | Retains the actual retry deadline and its clock before timer cleanup | AC-1e uses that remaining interval after connection-resource release |
| Real connection lifecycle | `reactor/session_read.go`: `Session.handleConnectionCloseLocked` | Delivers the TCP failure event, retains its retry budget, then retires timers | Uses the existing lifecycle for held Connect and every Active failure |
| Peer reconnect loop | `reactor/peer_run.go`: `Peer.run` | Uses a transferred retry deadline when present and ordinary backoff otherwise | Preserves the required initial-value retry without changing ordinary OpenSent backoff |
| Read-pool backpressure | `reactor/session.go`: `Session.Run`; `reactor/session_read.go`: `Session.awaitReadBuffer` | Selected reads wait for available storage and retry without consuming wire bytes | AC-1q extends the same pressure classification to both incoming read rails and keeps their deadline/cancellation owners |
| Received UPDATE ownership | `reactor/recent_cache.go`: `RecentUpdateCache.Add`, `Activate`, `Retain`, `Release`; `reactor/peer_run.go`: delivery worker | Cache adoption holds pooled storage through consumer publication and explicit retains | AC-1r closes failed publication admission and releases abandoned ownership without returning buffers still borrowed by a consumer |
| Passive connection startup | `reactor/peer_run.go`: `Peer.runOnce`; `reactor/peer_connection.go`: `Peer.acceptConnection`; `reactor/session_connection.go`: `Session.connectionEstablished` | The Peer owns the selected Session and accepted TCP connection across its start and handshake | AC-1s preserves a valid early exchange as well as ordinary passive acceptance; the implementation chooses how initialization and acceptance are ordered |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- FSM: the peer run loop's automatic restart and TCP events (rows 6, 7).
- Config: `bgp peer <address> timer delay-open` (row 6).

### Transformation Path
1. Config parse of `timer delay-open` into the peer settings.
2. The reactor's `runOnce` (`peer_run.go`) picks the start event for each cycle (row 7). A peer-owned `cycleReachedEstablished` (`atomic.Bool`) is set by the FSM callback beside publication of `PeerStateEstablished`. The next `runOnce` consumes and clears the previous cycle's answer with `Swap(false)` before choosing its start event, whatever error ended that cycle. `run()` chooses the retry wake-up, not the start event. `ErrTeardown` after Established, an error after Established (NOTIFICATION received, HoldTimer expiry, read error or recovered panic), and an `inboundNotify` wake-up after such a cycle each select Event 3. A cycle that never reached Established selects Event 6. The existing backoff policy is unchanged: the flag decides the event, never the delay. Event 1 stays the operator's first start; stopping the peer starts no next cycle.
3. The FSM runs the DelayOpenTimer in Connect and Active: on Event 16/17 with DelayOpen TRUE the reactor holds the connection without calling `sendOpen` until Event 20 (peer OPEN) or Event 12 (timer expiry) (row 6).
4. On Event 20 with `connection bfd { strict true }` negotiated and BFD neither Up nor AdminDown, the reactor sends the OPEN, withholds the KEEPALIVE and the FSM records the Connect or Active DelayOpen sub-state; a BfdUp, BfdAdminDown or BfdDisabled event then sends the KEEPALIVE and moves to OpenConfirm (row 6s). The same `advanceAfterOpen` decision that holds OpenSent today makes the call, so the strict-mode test is declared once.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config to reactor | peer settings carry the DelayOpen value | Source wired through the YANG leaf, `parsePeerFromTree` and `PeerSettings.DelayOpen`. Both daemon carriers and the pinned FRR clean–broken–clean qualification passed. |
| Reactor to FSM | Event 3 / Event 6 and the DelayOpen events | Source wired through `Peer.runOnce`, `Session.connectionEstablished`, `Session.delayOpenExpired` and `Session.advanceAfterOpen`. Scoped race populations, ownership and retry repetitions, final boundary cuts and row-20 proof renewals passed. |

### Integration Points
- `ze-bgp-conf.yang` grouping `peer-fields` - the new leaf reaches peer, group and bgp levels.
- `peer_run.go` start-event choice (around line 379) - the Event 3 / Event 6 choice; `session.go` `Start` / `startDamped` gain the Event 3 entry.
- `session_bfd_strict.go` `advanceAfterOpen` - the KEEPALIVE-withholding decision, now also called from the Event 20 path (row 6s).

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes, by source | YANG settings reach the ordinary Session/FSM paths. Incoming candidates use the shared Session frame reader and semantic OPEN admission. Daemon and FRR qualification passed, as did the 15 affected incoming roots over twenty race repetitions each; the remaining bounded proof work is listed above. |
| No unintended coupling (components stay isolated) | Yes, by source | Changes remain in BGP config, Session/FSM, capture and existing plugin event interfaces. Final scoped reactor lint analyzed all three flavors with no owned finding. Its foreign next-hop conversion finding and the full reactor run's known reds remain recorded in the owning state. |
| No duplicated functionality (extends existing, does not recreate) | Yes, by source | Event 3 reads the existing Established flag. The legacy duplicate collision resolver is removed. Candidate ingress and compatibility `processOpen` share semantic admission. |
| Zero-copy preserved where applicable (refs, not copies) | N-A | no wire encoding change |
| Registration over hardcoding, outbound | N-A | no registry involved |
| Registration over hardcoding, inbound | N-A | no registry involved |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | FRR's `neighbor X timers delayopen` lets an interop scenario run DelayOpen on both sides | `bgpd/bgp_vty.c` line 8204, read in the parent spec | `bgp-delayopen-frr` covers Ze-side DelayOpen only, and the scenario README says so | build the FRR scenario with both sides set | confirmed by native guest clean–broken–clean against pinned FRR 10.7.1, with loaded FRR DelayOpen configuration and complete captures (2026-11-16) |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Row 7 changes restart pacing: zeroing the counter on Event 3 hides oscillation the counter used to show | a flapping peer restarts faster after the change | Event 3 only on the cycle after a session that reached Established, read from the `cycleReachedEstablished` flag the session sets, never from the loop's error arm (which cannot tell); every cycle after a failed attempt keeps Event 6 and its counter (AC-2) |
| R-3 | Row 6s holds a session in Connect or Active with an OPEN sent and no KEEPALIVE | a session stuck before OpenConfirm with BFD Down | The HoldTimer (reset to the negotiated value, AC-7h) or the BfdHoldTimer (hold time zero, AC-7) bounds the wait |
| R-2 | Row 6 delays Ze's OPEN; a peer that also waits never establishes | a session stuck in Active or Connect | The DelayOpenTimer always expires and Ze then sends its OPEN; the interop scenario sets DelayOpen on both sides |
| R-4 | Concurrent TCP attempts share BGP4MP endpoint fields, which omit TCP ports; an overlapping collision NOTIFICATION cannot identify which OPEN pair survives | A captured mixed-family ADD-PATH UPDATE reports unavailable or ambiguous context | Retain every original frame and report ambiguity rather than infer a winner. The live OPEN-less incumbent control proves automatic file decoding when the winning pair is unambiguous; the overlap fixture separately checks its actual selected TCP connection's unchanged OPEN context and exact prefixes/Path Identifiers. |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Row 6: session establishment timing with `timer delay-open` set (OPEN held, Event 18 in Active). Row 7: restart pacing and the ConnectRetryCounter |
| How is it reverted? | Each row lands as its own commit, revertible alone |
| Who else touches this path? | the rfc-verdict pass `spec-rfc-verdict-test-fix-pass` (closed 2026-11-14, `plan/learned/051-rfc-verdict-test-fix-pass.md`) (ledger rows, the R50 re-pointed tags its BGP child spec-rfc-verdict-fix-bgp landed), `spec-bgp-open-session-rfc-defects` (closed 2026-10-21; OPEN and session handling), `spec-bgp-live-receive-binding-and-bfd-keep-session` (closed 2026-10-13; BFD strict-mode and session work in flight on 2026-10-11; row 6s edits `session_bfd_strict.go` and the strict-mode FSM arms it may also touch, so this spec's row 6s phase starts after that spec lands or coordinates file by file) |
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
| AC-1m (capture removal ownership) | A Session factory is still constructing its capture while API removal, reload, dynamic removal or rollback deletes its exact Peer, followed by a replacement at the address | Registration and removal share the reactor membership cut. Every registered old input is joined, and the late old factory cannot register afterward. The replacement still captures its complete original wire input and terminates its file; ordinary current-member factory capture remains permitted. Shutdown seals new registration as well. |
| AC-1 (row 6, config) | `bgp peer <address> timer delay-open <seconds>` | 1..240 sets the DelayOpen attribute TRUE and DelayOpenTime to the value; `0`, the default, leaves DelayOpen FALSE and every existing FSM test and the OPEN timing unchanged. A value outside the range is refused at commit with the leaf, the range and the fix |
| AC-1a (row 6, Event 16/17) | DelayOpen TRUE, TCP connection succeeds in Connect (Ze dialled) or Active (Ze accepted), through a real `Peer` | The ConnectRetryTimer is stopped and zero, the DelayOpenTimer runs with DelayOpenTime, no OPEN is written to the connection, the state stays Connect or Active (the two quoted clauses) |
| AC-1b (row 6, Event 12) | DelayOpen TRUE, the DelayOpenTimer expires with no peer OPEN, in Connect and in Active | Ze sends its OPEN, sets the HoldTimer to a large value, and moves to OpenSent; in Active it also zeroes the ConnectRetryTimer and clears the DelayOpenTimer (the two quoted clauses) |
| AC-1c (row 6, Event 20) | DelayOpen TRUE, the peer's OPEN arrives while the DelayOpenTimer runs, in Connect and in Active, BFD strict not negotiated | ConnectRetryTimer zero, DelayOpenTimer cleared, Ze sends its OPEN then a KEEPALIVE, the KeepaliveTimer and HoldTimer follow the negotiated hold time (or both reset when it is zero), and the state is OpenConfirm with no stop in OpenSent |
| AC-1d (row 6, Event 18 in Connect) | DelayOpen TRUE, the held connection fails in Connect while the DelayOpenTimer runs | The ConnectRetryTimer restarts with its initial value, the DelayOpenTimer is cleared, Ze keeps listening, and the state is Active |
| AC-1e (row 6, Event 18 in Active, the ledger row) | A real `Peer` loses its TCP connection in Active, while DelayOpen runs or after strict Event 20 cleared it and left the session BFD pending | The four quoted actions: the ConnectRetryTimer restarts with its initial value, the DelayOpenTimer is cleared, the connection's resources are released, the ConnectRetryCounter rises by 1; the state is Idle. The Peer preserves the actual restarted timer's remaining interval across cleanup. Negative polarity: a TCP failure in OpenSent retains ordinary backoff rather than receiving Active's initial retry interval. |
| AC-1f (row 6, Event 24) | DelayOpen TRUE, a NOTIFICATION with a version error arrives while the DelayOpenTimer runs, in Connect and in Active | ConnectRetryTimer zero, DelayOpenTimer cleared, resources released, connection dropped, state Idle, and the ConnectRetryCounter is NOT incremented (the DelayOpenTimer-running clause carries no increment; the not-running clause Ze runs today keeps its increment) |
| AC-1g (row 6, the other-events arm; ledger row `RFC4271-8.2.2-15`) | DelayOpen TRUE, any other event of Connect's or Active's catch-all list while the DelayOpenTimer runs. RFC 4271 Section 8.2.2 (Connect): "In response to any other events (Events 8, 10-11, 13, 19, 23, 25-28), the local system:", whose second action is "if the DelayOpenTimer is running, stops and resets the DelayOpenTimer (sets to zero)," | The DelayOpenTimer is stopped and reset in the same arm that already stops the ConnectRetryTimer, then today's actions. `TestRFC4271ConnectCatchAllStopsRunningDelayOpen` exercises that path, with `TestRFC4271ConnectDuplicateStartPreservesRunningDelayOpen` as the useful ignored-start control. The catch-all unit asserts timer stopped, connection dropped, counter incremented and Idle, and fails with only the Idle timer stop removed. Native records are current, the independent verdict is `enforced`, and the final ledger check has no row-15 finding. |
| AC-1h (row 6, pre-buffered OPEN, round-2 I1) | DelayOpen TRUE, an externally pre-buffered OPEN reaches the compatibility `acceptWithOpen` entry; actual listener candidate transfers follow AC-1j/k | The accept is Event 17 and the buffered OPEN is Event 20, in that order, in the state the start left (round-3 I1): `runOnce` fires the start event before it takes the inbound slot and runs `acceptWithOpen` (`peer_run.go`), and `session.go` sets `SetPassive(!settings.Connection.IsActive())`, so for a peer that also dials (`connection both`, the default) the start goes to Connect and both events arrive in Connect, and for a passive-only peer (`connection passive`) the start goes to Active and both arrive in Active. An active-only peer never reaches this path: `acceptOrReject` closes its inbound. In both states: the DelayOpenTimer is started and then cleared, the ConnectRetryTimer is zero, Ze writes no OPEN before it processes the buffered one (`connectionEstablished` holds it, AC-1a), then sends its OPEN and a KEEPALIVE and moves to OpenConfirm without passing OpenSent (or to the corresponding Connect/Active DelayOpen BFD-pending sub-state under row 6s, AC-4). With DelayOpen FALSE the sequence is today's (OPEN sent, OpenSent, then OpenConfirm) |
| AC-1i (row 6, second connection) | A second incoming TCP connection arrives while the outbound is held by DelayOpen | The admitted incoming connection gets its own Session/FSM and DelayOpen lifecycle. Neither OPEN-less connection wins merely by arriving. If the outbound receives OPEN first, its Event 20 handshake proceeds and Section 6.8 compares the later incoming OPEN against OpenConfirm. If the incoming receives OPEN first while the outbound is still OPEN-less, Ze retains the owner's immediate-close policy: Event 23 retires the held outbound without a NOTIFICATION, and the exact incoming Session is adopted after old-epoch cleanup. If the outbound DelayOpen expires first and enters OpenSent, AC-1j applies. A pending read deadline closes only that candidate. Active-only peers continue to refuse inbound connections. In every permitted order one useful connection can establish. |
| AC-1j (independent incoming FSM) | An admitted incoming TCP connection arrives beside an OpenSent incumbent whose remote identifier is not known outside the protocol | With zero DelayOpen, the incoming connection receives its own local OPEN before it sends a remote OPEN. With DelayOpen configured, its own timer or received OPEN drives that same Session. Section 6.8 does not reject it merely because the incumbent is OpenSent; the independent handshake can establish, then retire a still-OpenSent incumbent. OpenConfirm arbitration retains both identifier orders and Established retains the useful incumbent. An incoming failure or malformed OPEN closes only that connection; the original can still complete its handshake. |
| AC-1k (same Session adoption and shutdown) | A candidate wins, expires during handoff, meets a publication failure, receives BFD events, or the peer stops | Promotion joins its reader, handoff and cancellation bridge and adopts its original socket, buffered reader, proposal, FSM and timer deadlines. Local OPEN and KEEPALIVE are not sent twice. All timer/writer callbacks remain immutable; selected publication begins only after delivery and its down finalizer exist. Expired candidates cannot restart or announce a peer-up. Failure updates peer-owned retry history. Every socket, timer, capture worker and incoming worker has a retiring owner. A reserved third arrival closes silently before OPEN. Ordinary and strict-BFD useful establishment remain permitted. |
| AC-1n (capacity refusal diagnosis) | An excess TCP connection arrives while an admitted incoming exchange or its joined retirement owns capacity | Before receiving OPEN, the excess socket receives exact Cease / Connection Rejected (6/5), then EOF. The existing incumbent and admitted candidate remain owned. The same admitted candidate can complete its OPEN exchange and, when the incumbent is Established, receives true collision 6/7 and EOF. The incumbent retains its original connection and processes a subsequent KEEPALIVE. Reserved-handoff and shutdown silent closure remain separate existing outcomes. |
| AC-1o (provisional plugin validation) | An incoming OPEN is validated beside an existing connection, or its candidate is selected after incumbent cleanup | Provisional validation checks the OPEN without changing peer-scoped plugin state. Rejected or losing candidates preserve the incumbent's learned role and useful OTC export. Promotion publishes the exact retained OPEN pair once on internal and JSON RPC transports. Selected-validation failure or expiry of the original HoldBudget refuses the candidate without peer-up or deadline renewal. Ordinary selected OPEN validation remains permitted. |
| AC-1p (pooled frame transfer) | A complete BGP frame is read through the shared reader | A warmed standard or extended pooled frame read and return allocates no per-message wrapper. Both selected and incoming readers retain original-wire validation, capture and buffer-return ownership. Valid messages remain available for semantic dispatch. |
| AC-1q (incoming read-pool pressure) | The standard read pool is exhausted before the candidate's remote OPEN, or its negotiated extended pool is exhausted before the following read | Pressure pauses that incoming read without EOF, replacing the candidate, consuming frame bytes or retiring the useful OpenSent incumbent. Returning storage lets the same candidate, TCP socket and buffered reader complete OPEN/KEEPALIVE and establish, with exactly two local OPENs across incumbent and candidate and one received OPEN. Waiting does not renew any timer or deadline. With storage still exhausted, the original first-OPEN deadline or peer cancellation retires the candidate and releases admission; a candidate-only deadline preserves the incumbent. The first-OPEN zero-value fallback stays 90 seconds. |
| AC-1r (failed receive-publication ownership) | A real pooled UPDATE is adopted before a nil receiver, receiver panic, queued receiver panic or pre-handoff panic, with ordinary and coalesced reads | After failure and owner joins, abandoned current and queued receipts leave no cache entry or pool slot. Failed publication refuses new lookup and retain admission. An already-retained consumer remains permitted to read its byte-identical original receipt and forwarding buffer until it releases its retain; those buffers stay owned during that borrow and return exactly once after its last release. Earlier UPDATEs already handed to a live delivery worker still complete in order when only a later read-task handoff fails. Worker failure retires only work it cannot publish. Successful publication and its existing consumers remain permitted. Pool capacity, safety-valve timing and process isolation are not increased to mask a leak. |
| AC-1s (incoming connection during passive startup) | A valid incoming TCP connection and remote OPEN reach a passive peer during selected-Session initialization, with ordinary acceptance after startup as the control | The early connection may wait for initialization within existing lifecycle/handshake bounds. Once startup proceeds, that same TCP connection and its owning Session complete OPEN/KEEPALIVE and reach Established without a reconnect, a lost connection event or a replayed handshake. The documented public `connected` event occurs once before the TCP-driven `fsm` transition into OpenSent, and that transition is delivered once even when the initial Peer-state callback is paused. With DelayOpen disabled, exactly one local OPEN and one remote OPEN produce one establishment; configured DelayOpen timing remains governed by AC-1a through AC-1c. Ordinary passive acceptance remains permitted. Stop/cancellation still ends pending ownership without late publication. No particular buffering or publication architecture is required. |
| AC-1t (UPDATE admission before establishment) | A valid withdrawal arrives on an ordinary selected connection or independent incoming connection still waiting for OPEN or strict BFD, on either read path | The UPDATE cannot reach route or received-UPDATE publication before that connection is Established. Existing protocol rejection and cleanup remain appropriate to the receiving state; the candidate's failure preserves the incumbent connection and its useful routes. OpenSent/OpenConfirm rejection remains intact, and a valid Established withdrawal remains permitted. The shared admission rule must cover pre-OPEN and post-OPEN strict waits without changing unrelated ROUTE-REFRESH semantics. |
| AC-1l (TCP connected event) | TCP succeeds through dial or accept, including an independent incoming candidate, with zero or configured DelayOpen | Exactly one connected event precedes OPEN processing and the connection's first published FSM handshake transition. It reports this Session's actual local endpoint and omits a dynamic remote AS before negotiation. A DelayOpen wait still emits it immediately. The same useful connection subsequently establishes; adoption does not repeat connected. |
| AC-2 (row 7) | A peer session that reached Established ends, by a received NOTIFICATION, by HoldTimer expiry (both error returns from `Session.Run`), or by `ErrTeardown`, and the reconnect loop starts the next cycle, after the backoff timer or after `inboundNotify` cuts it short | The FSM receives Event 3 (AutomaticStart) and the ConnectRetryCounter is zero, for each of the three exits. Negative polarity: the cycle after a failed attempt (no Established), including the cycle `inboundNotify` restarts, fires Event 6 and keeps the counter (`TestRFC4271ConnectRetryCounterSurvivesDampedStart` stays green). The operator's first start stays Event 1. The predicate is the `cycleReachedEstablished` flag the FSM state callback sets on entering Established and `runOnce` clears before each session, consumed by the next `runOnce` whatever `safeRunOnce` returned (Data Flow step 2), never a second timer and never the loop's error arm. The prefix-limit arm of `run()` is one more exit after Established: `PrefixReconnectTimer` waits its own delay and continues, and the next cycle takes Event 3 like the others; `PrefixReconnectNever` returns from `run()`, so no next cycle starts and no event fires (round-3 N3) |
| AC-2p (row 7, passive) | The same, on a peer with `connection passive` | Ze fires Event 3 and the FSM goes to Active, following the existing Event 1/Event 4 conflation in `fsm.go` (one arm, `passive` chooses Connect or Active), extended to Event 3 in the same arm; the arm's comment states the conflation. Event 5 is not added, because nothing in Ze fires it. The counter is zero |
| AC-3 (ledger) | Each row once its AC is met | The row in `rfc/short/rfc4271.md` loses `{gap}`, carries tags in both polarities, the HEAD-tagged 8.2.2-13 units are re-pointed at the real-`Peer` test (R50), and each carries a discrimination record (`./le rfc record`); `./le rfc check` exits clean for `rfc4271` |
| AC-4 (row 6s, enter) | DelayOpen TRUE, `connection bfd { strict true }` negotiated, BFD neither Up nor AdminDown, the peer's OPEN arrives while the DelayOpenTimer runs, in Connect and in Active | Ze sends its OPEN and NO KEEPALIVE; with a non-zero HoldTimer initial value the KeepaliveTimer is not started and the HoldTimer is reset to the negotiated value; with zero the KeepaliveTimer and HoldTimer reset and the BfdHoldTimer starts with BfdHoldTime; the state stays Connect or Active and the FSM reports `ConnectDelayOpenBfdUpPending` or `ActiveDelayOpenBfdUpPending` (§8.3.5, §8.4.5) |
| AC-5 (row 6s, leave) | In either DelayOpen sub-state, BfdUp, BfdAdminDown or BfdDisabled arrives | Ze sends a KEEPALIVE, with a non-zero HoldTimer starts the KeepaliveTimer and zeroes the BfdHoldTimer, and moves to OpenConfirm, leaving the sub-state (§8.3.1, §8.4.1 first branch). Outside the sub-state the same events keep today's "stays in the Connect/Active state" |
| AC-6 (row 6s, second OPEN) | In either DelayOpen sub-state a second OPEN arrives | Ze sends a NOTIFICATION with Finite State Machine Error, zeroes the ConnectRetryTimer, releases resources, drops the connection, increments the ConnectRetryCounter and goes to Idle (§8.3.5, §8.4.5 new text) |
| AC-7 (row 6s, timers and config change) | In either DelayOpen sub-state the BfdHoldTimer expires, or `bfd strict` changes | Today's Connect and Active arms run unchanged: Cease / BFD Down and Idle (§8.3.3, §8.4.3), or drop and Idle with the counter zero (§8.3.4, §8.4.4); the sub-state is cleared on every transition out |
| AC-7h (row 6s, HoldTimer bound, round-2 I3) | In either DelayOpen sub-state with a non-zero negotiated hold time, BFD stays Down and no KEEPALIVE is sent | The HoldTimer runs while the state is still Connect or Active (AC-4 resets it to the negotiated value on entering the sub-state); on expiry, HoldTimer_Expires (Event 10) takes the Connect or Active catch-all arm: the ConnectRetryTimer is zeroed, resources released, the connection dropped, the ConnectRetryCounter incremented by one, the sub-state cleared, and the state is Idle. The session never stays in the sub-state past the negotiated hold time |
| AC-8 (row 6s, strict-mode path refusal) | `EnterBfdUpPending` or its Connect/Active sibling is called in a state the draft does not allow | It returns the error it returns today for an OpenSent sub-state outside OpenSent; the Connect/Active sub-states are accepted only in Connect and Active with the DelayOpenTimer just cleared by Event 20 |
| AC-9 (row 6s, ledger and comments) | Row 6s landed | `rfc/short/draft-ietf-idr-bgp-bfd-strict-mode.md` lines 14 and 20 no longer call the sub-states unreachable and name the producing functions; no requirement row is added because §8 carries no RFC 2119 keyword (its Notes); every comment and page listed in Current Behavior and the Documentation checklist that calls the sub-states unreachable is rewritten in the same commit; `./le rfc check` exits clean for the draft stem |
| AC-10 (connection diagnostics) | Either OPEN validation rail refuses an unacceptable AS or Identifier, or the BGP HoldTimer expires during a strict-mode BFD wait | The bounded report bus retains the connection's cause under a registered doctor code, including failed NOTIFICATION delivery. A refused unselected connection cannot overwrite the healthy incumbent's notification history or report that peer down. Valid OPEN and ordinary hold expiry are controls. |
| AC-11 (OPEN preparation during reload) | A plugin declaration changes during selected OPEN preparation or an independent incoming constructor, handoff or selected validation | A busy selected constructor cannot prove its captured offer current and restarts. A genuinely unprepared session can read the current declaration; a published proposal remains immutable. Incoming offers are judged against their admitted remote OPEN: a changed capability in use is refused before takeover when publication preceded that decision; unused or unchanged offers remain permitted. Reload snapshots exact current, pending and typed inbound Session identities for all retained configured, dynamic and runtime-created peers, then checks and retires stale owners outside Peer locks. A hidden constructor must pass the later fresh check; selected validation checks freshness again after its blocking callback. A collision decision before declaration publication may legitimately retire its incumbent, while the later census or selected guard retires the newly stale epoch. No OPEN-owner callback runs under reload caller locks. Failed construction releases its capture owner so the same Session can retry. The peer-change estimate retains its existing peer-based meaning. |
| AC-12 (live fixture isolation) | A `lowLiveRouter` fixture enables validation, then finishes | Validation remains enabled while its daemon is alive. After all reactor, plugin and server owners join, the fixture restores its initially absent provider. A failed join keeps the refusing gate. `TestLowLiveRouterReleasesValidationAfterCleanup` and the existing RFC 7705 live callers prove the lifecycle without changing production shutdown policy. |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Sets `timer delay-open` on a peer facing FRR | config -> FSM DelayOpenTimer -> OPEN held -> session up | `bgp-delayopen-frr` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestIncomingBeforeOpenUpdateCannotPublishIncumbentWithdrawal`, `TestActiveDelayOpenUpdateNeverPublishes`, `TestIncomingStrictPendingUpdateNeverPublishes` | `internal/component/bgp/reactor/rfc4271_incoming_preestablished_update_test.go` | AC-1t: actual candidate listener, ordinary read rails, no publication before establishment and useful incumbent/Established controls | All seven intended failing leaves were observed before the shared guard correction. The corrected twelve-root race selection completed twenty repetitions per root, 240 passes without FAIL or DATA RACE. The actual daemon discriminator also accepted clean PASS and broken FAIL at the original socket's withdrawal/FIFO assertion. Final13 unique live controls passed with zero skips; selected-source RFC currentness is complete. |
| `TestPassiveIncomingAcrossSessionStart` | `internal/component/bgp/reactor/peer_passive_start_admission_test.go` | AC-1s: actual listener, controlled initial Active/start interval and ordinary passive control; original TCP/Session, exactly one local/remote OPEN and one establishment; actual IPC observes connected, OpenSent, OpenConfirm and Established once in order | Intended RED observed in `job-incoming-passive-start-red-09d2d0cd.log`: gated startup reached EOF while waiting for KEEPALIVE; ordinary passive startup passed. The strengthened `job-incoming-start-complete-observer-red-09d2d0cd.log` isolates missing OpenSent after the handshake correction, with ordinary control passing. Final callback/publication source ordering independently read; eight startup/raw roots passed under race, and fifteen startup/reload/retry/BFD controls each passed twenty repetitions (300 passes) in job-incoming-start-controls20-10155989.log. |
| `TestIncomingCandidateReadPoolBackpressure`, `TestIncomingCandidateReadPoolWaitEnds` | `internal/component/bgp/reactor/incoming_read_buffer_wait_test.go` | AC-1q: actual listener, exhausted standard/extended pools, same-socket establishment after release, original deadline and cancellation while storage remains held | Main observed RED in `job-unit-pkg-b9083d6e.log`: both before/after-OPEN resumption paths and both bounded-wait controls received premature EOF. Each restored root passed twenty race repetitions in the 99-root/1980-pass pool-controls checkpoint. Selected-source behavior and currentness checks are complete; the main thread owns the two publication commits. |
| `TestReceivePublicationAbortReleasesReadOwnership`, `TestReceivePublicationAbortProtectsRetainedReadOwnership` | `internal/component/bgp/reactor/receive_publication_abort_test.go` | AC-1r: real read-slot ownership at worker/read-task failures, three queued receipts, closed lookup/retain admission and byte-identical retained receipt/forwarding buffers through final release, in both read modes | Main observed RED in `job-unit-pkg-b9083d6e.log`: nil/panic/queued/pre-handoff paths retained 1/1/3/1 adopted entries; retained-consumer cases still accepted lookup/retain and kept their last entries. Each expanded ordinary/coalesced root passed twenty race repetitions in the 99-root/1980-pass checkpoint. The later continuation correction requires separate verification below. |
| `TestReceivePublicationFollowPanicPreservesPrimaryOwnership` | `internal/component/bgp/reactor/receive_publication_follow_panic_test.go` | AC-1r: a real two-family policy withdrawal publishes its primary with retained pooled receipt/variant, then its heap-backed continuation panics; preserve the primary, both buffers and original panic cause through final release | Main observed intended RED in `job-incoming-follow-panic-red-3c7a18c7.log` on standard and coalesced readers: 4096 held bytes instead of 8192 and actual double-return reports. The additive fixture reaches the real continuation producer; corrected-carrier clean–broken–clean and the final ten-root race selection passed, each root twenty times (200 passes), in job-incoming-ownership-final20-daefe27c.log. |
| `TestAbortedPublicationRetainSurvivesLateLifecycle` | `internal/component/bgp/reactor/recent_cache_abort_lifecycle_test.go` | AC-1r: late activation, explicit expiry and pressured gap scan cannot reopen admission or return the retained original/variant; final Release restores the pool baseline | Added guard coverage after the original retained-consumer RED; the final ten-root race selection passed each root twenty times (200 passes), including this guard, in job-incoming-ownership-final20-daefe27c.log. |
| `TestOpenConnectionRefusalDiagnostic`, `TestOpenConnectionRefusalDiagnosticSurvivesUnsentNotification`, `TestBFDWaitHoldExpiryDiagnostic`, `TestSessionProblemDiagnosticRegistration` | `internal/component/bgp/reactor/session_problem_diagnostic_test.go` | AC-10, both OPEN rails and selection states, failed delivery, strict waits, ordinary control, registered metadata and exact command paths | Actual missing-history RED observed; valid controls and registration PASS. Reviewed production reporting is applied; all four roots PASS under race. |
| `TestPluginCapabilitiesKeptBeforeOpen`, `TestPluginCapabilitiesKeptDuringPreparation`, `TestPrepareOpenFailureReleasesProposalCapture` | `internal/component/bgp/reactor/session_plugin_preparation_test.go` | AC-11, exact frozen capability bytes, actual input-getter boundary and failed-build retry | During-preparation regression RED observed. Independently reviewed capture ownership is applied; all three roots and collision/reload controls PASS under race. |
| `TestRemovedPeerFactoryCannotCaptureReplacement` | `internal/component/bgp/reactor/capture_removal_test.go` | AC-1m: gate the real old factory before capture registration, finish Remove/Add, refuse its late input, then retain the replacement's real TCP OPEN in its terminated capture | Named race RED/GREEN observed; the capture-removal root passed twenty ownership race repetitions. |
| `TestIncomingExpiredOpenCannotRetireIncumbent`, `TestIncomingValidatorRefusalUsesOpenErrorEvent` | `internal/component/bgp/reactor/incoming_ingress_corrections_test.go` | AC-1j/k: original HoldTimer deadline still governs admission and publication. Coded plugin refusal delivers Event22 once and gets no transport retry budget. | Named race RED/GREEN observed; both roots passed twenty affected incoming race repetitions. Removing only the final expiry guard under the peer owner failed the expired-candidate assertion; the restored race run passed both expiry barriers. |
| `TestCandidateOpenPromotionPublishesRemoteRouterID` | `internal/component/bgp/reactor/candidate_selected_router_id_test.go` | AC-1k: the original candidate Session/socket establishes and publishes its validated remote Identifier after incumbent cleanup | Named race RED/GREEN observed; selected-identifier promotion passed twenty ownership race repetitions. |
| `TestCaptureIdentityFollowsPreparedDynamicProposal`, `TestSelectedUnpreparedCaptureClosesWithoutFile` | `internal/component/bgp/reactor/candidate_capture_identity_test.go` | AC-1k: capture identity follows the exact local OPEN proposal across dynamic-AS publication. A selected unprepared Session closes without a file. | Named race GREEN observed after intended behavioral REDs. Both named roots also passed in the merged reactor race run; that run's unrelated failures and timeout remain separately recorded. |
| `TestPreparedCaptureReceiptBeforeCloseDrainsOriginalInput` | `internal/component/bgp/reactor/candidate_capture_receipt_test.go` | AC-1k: selection and the prepared proposal received before Close preserve the original queued input in all 20 rounds | The named root passed twenty ownership race repetitions, each retaining its twenty internal selection/close rounds. |
| `TestCandidateValidationPreservesIncumbentRouterID`, `TestIncumbentCleanupPreservesCandidateRouterID`, `TestCandidateOpenIdentitySurvivesDynamicPublication`, `TestCandidatePreparedAfterPublicationUsesCurrentAS`, `TestRouterIDRevalidationRetainsSiblingClaim` | `internal/component/bgp/reactor/candidate_router_id_test.go`, `candidate_router_id_registry_test.go` | AC-1j/k: unselected validation preserves incumbent publication. Session-owned claims and immutable proposals survive independent connection cleanup and dynamic publication. | All five named roots passed in the merged reactor race run; that run's unrelated failures and timeout remain recorded. Registry revalidation also passed twenty ownership race repetitions. |
| `TestRFC4271DelayOpenHoldsOpen` | `internal/component/bgp/fsm/` | AC-1: OPEN held until peer OPEN or timer expiry; 0 leaves DelayOpen off | |
| `TestRFC4271ActiveEvent18WithDelayOpen` | `internal/component/bgp/reactor/` | AC-1e: real-Peer Active EOF state, counter, socket and DelayOpen cleanup, with the zero-delay OpenSent control; actual retry scheduling is covered below | |
| `TestRFC4271ActiveDelayOpenFailureRetriesAtInitialValue`, `TestRFC4271ActiveStrictPendingFailureRetriesAtInitialValue`, `TestRFC4271OpenSentFailureKeepsOrdinaryBackoff` | `internal/component/bgp/reactor/rfc4271_delay_open_retry_test.go`, `rfc4271_delay_open_strict_retry_test.go` | AC-1e: actual Peer retry interval after held Active and strict BFD-pending EOF, plus the ordinary OpenSent control; each replacement establishes on a distinct Session | All three passed twenty race repetitions, alongside three budget/retirement controls. Native clean-pass/broken-failure records are current and independently audited; the final ledger check has no row-13 finding. |
| `TestRFC4271AutomaticStartEvent3ZeroesCounter` | `internal/component/bgp/reactor/rfc4271_automatic_start_peer_test.go` | AC-2: real Established sessions ended by received NOTIFICATION and negotiated HoldTimer expiry reset the counter on reconnect | existing tagged coverage; unchanged |
| `TestRFC4271FailedCycleRestartKeepsCounter`, `TestRFC4271FailedCycleAfterEstablishedKeepsCounter`, `TestRFC4271FailedCycleInboundShortenedBackoffKeepsCounter` | same file | AC-2 negative: ordinary failed retry, a failed cycle after an earlier Established cycle, and inbound-shortened failed backoff retain the counter | new inbound case passed; its wrong-zeroing branch mutation was recorded red |
| `TestRFC4271AutomaticStopTeardownRestartZeroesCounter`, `TestRFC4271InboundShortenedBackoffZeroesCounter` | same file | AC-2: automatic stop and inbound-shortened backoff after an Established session select the zeroing restart | existing coverage; unchanged |
| `TestRFC4271HoldExpiryInboundShortenedBackoffZeroesCounter` | same file | AC-2: real HoldTimer Expired NOTIFICATION, counter 1, then inbound wake-up and counter 0 without releasing the held backoff | scoped race passed; wrong-damped-start mutation recorded red |
| `TestRFC4271PassiveAutomaticStartReconnectZeroesCounter` | same file | AC-2p: passive reconnect after NOTIFICATION, real hold expiry and `ErrTeardown`; exit counter 1, distinct next session Active, counter 0 | scoped race passed; all three exits failed under the recorded wrong-damped-start mutation |
| `TestRFC4271DelayOpenEventsConnect`, `TestRFC4271DelayOpenEventsActive` | `internal/component/bgp/fsm/` | AC-1a, AC-1b, AC-1c, AC-1d, AC-1f, AC-1g: one subtest per event, the quoted actions as assertions | |
| `TestRFC4271DelayOpenHeldConnectionWritesNoOpen` | `internal/component/bgp/reactor/` | AC-1a: no OPEN bytes on the wire while the DelayOpenTimer runs, through a real `Peer` | |
| `TestRFC4271DelayOpenPreBufferedOpen` | `internal/component/bgp/reactor/` | AC-1h, once per connection mode: through `acceptWithOpen` with DelayOpen TRUE, the wire carries no OPEN before the buffered OPEN is processed, then OPEN and KEEPALIVE, and the state goes Connect to OpenConfirm for `connection both` and Active to OpenConfirm for `connection passive`, with no OpenSent in either; with DelayOpen FALSE the old sequence in both modes | |
| `TestRFC4271DelayOpenBothSidesHold` | `internal/component/bgp/reactor/` | AC-1i, through a real `Peer` and two connections: the second inbound takes the pending slot while the DelayOpenTimer runs, in Connect and in Active; orders (a) with both BGP Identifier orders, (b) and (c); in (b) the held outbound is closed with no NOTIFICATION on its wire, the FSM logs Event 23 and Idle, the ConnectRetryCounter rises by one, the next cycle starts at once with Event 6 and runs `acceptWithOpen`; in (b) with the held outbound already in OpenConfirm, §6.8 decides instead, run in both BGP Identifier orders (round-4 I1): peer higher, the existing session is closed with Cease and the stored inbound runs `acceptWithOpen`; Ze higher, the inbound slot is empty, `collisionHandoff` is false, no `inboundNotify` wake-up is pending, the inbound socket is closed, and the existing session stays in OpenConfirm and reaches Established; the pending deadline closes only the pending connection; exactly one Established session in each; an active-only peer closes the inbound | |
| `TestRFC4271ListenerIncomingDuringOutgoingOpenSentSendsOpen`, `TestIncomingCandidateDelayOpenExpires` | `internal/component/bgp/reactor/` | AC-1j: actual listener gives the second connection an independent default OPEN or DelayOpen timer, then keeps useful handshake ownership | |
| `TestIncomingAdoptionPanicReleasesOpenOwner` | `internal/component/bgp/reactor/` | AC-1k: completed private TCP handshake, controlled publication panic, bounded timer retirement after the scoped adoption lock releases | The named root passed twenty affected incoming race repetitions. Removing the scoped unlock failed the stranded-lock assertion; the restored race run passed. |
| `TestIncomingCollisionPanicReleasesPeerOwner` | `internal/component/bgp/reactor/incoming_collision_panic_test.go` | AC-1k: a panic during collision arbitration releases `Peer.mu`; the admitted candidate retires its socket, timers and admission while the same incumbent remains useful | Untagged regression: the old source failed only the stranded-lock assertion; the fix passed twenty affected incoming race repetitions. The original native collision proof also completed clean PASS and expected broken FAIL with bounded cleanup. |
| `TestIncomingCandidateFailureKeepsPeerRetryHistory` | `internal/component/bgp/reactor/` | AC-1k: independent candidate Active-held EOF increments shared retry history; original still establishes | Corrected Active-held input passed its targeted checkpoint and the named root passed in the merged reactor race run; the run's unrelated failures and timeout remain recorded. |
| `TestIncomingCandidateBFDReleaseKeepsOriginalSession` | `internal/component/bgp/reactor/` | AC-1k: actual listener's strict pending candidate gets Peer BFD release and promotes the same Session/reader with no duplicate OPEN | The named listener root passed twenty affected incoming race repetitions. Native clean–broken–clean qualification detected omission of candidate BFD delivery at the bounded socket assertion. |
| `TestDelayOpenConnectedEventPrecedesOpen`, `TestImmediateConnectedEventPrecedesOpenConfirm`, `TestIncomingConnectedMetadataUsesCandidateSocket` | `internal/component/bgp/reactor/` | AC-1l: real IPC connected ordering, exactly once, candidate-specific TCP endpoint and unknown pre-OPEN dynamic AS, followed by useful establishment | Both connected-order roots passed the scoped lint-fix race selection; candidate metadata also passed twenty affected incoming race repetitions. Native clean–broken–clean qualification detected borrowing the incumbent's endpoint and previously published dynamic AS. |
| `TestSessionBFDStrictDelayOpenBoundedWait` | `internal/component/bgp/reactor/bfd_delay_open_test.go` | AC-6, AC-7, AC-7h: real timer callbacks and Session.Run retire socket/timers, counter and pending state; second OPEN refuses renegotiation; config close is silent | passed in scoped race selection; boundary mutations discriminate |
| `TestDraftBFDStrictDelayOpenSubStates`, `TestDraftBFDStrictDelayOpenFailures`, `TestEnterDelayOpenBfdUpPendingRefusesInvalidEntry` | `internal/component/bgp/fsm/bfd_delay_open_test.go` | AC-4, AC-5, AC-6, AC-7, AC-8 in Connect and Active, hold zero/nonzero and all three releases | passed in scoped race selection; boundary mutations discriminate |
| `TestSessionBFDStrictDelayOpenRelease`, `TestSessionBFDStrictDelayOpenNonwaitingControls`, `TestPeerBFDStrictDelayOpenRelease` | `internal/component/bgp/reactor/bfd_delay_open_test.go` | AC-4, AC-5 wire half: both OPEN processing rails, all releases, nonwaiting controls and actual Peer Connect/Active cycles | passed in scoped race selection |
| `TestSessionBFDStrictDelayOpenImmediatePeerKeepalive` | `internal/component/bgp/reactor/bfd_delay_open_release_race_test.go` | AC-5: immediate peer reply cannot race BFD release into the Connect/Active catchall | observed red before `handleKeepalive` serialization; repaired race selection passed |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `timer delay-open` (seconds) | 0 (off) or 1..240, FRR's range | 240 | N/A (0 is valid: off) | 241 |

### Controlled lifecycle seams

These cases use the controlled-Go exception in `ai/rules/testing.md`. The
daemon role-isolation carriers remain required for observable learned-state
and OTC/filter wiring. In-process JSON RPC is not a forked-process result.

| Scenario | Controlled proof | Why the exact interleaving uses Go |
|----------|------------------|-----------------------------------|
| A constructor captured the previous declaration before publishing its OPEN proposal | `TestIncomingPluginReloadPreservesUsefulIncumbent` | The capture has no wire-visible receipt yet. A public reload cannot pause that private getter boundary. The actual TCP exchange then distinguishes refused in-use capability from permitted unused/unchanged offers and preserves the useful incumbent. |
| Reload crosses a typed incoming handoff | `TestIncomingPluginReloadRetiresStaleHandoff` | The pending/inbound/current membership and joined-cleanup barrier are private. Operator state exposes the selected peer, not an API that holds this exact receipt transition. The test also establishes a fresh current offer on the same Peer after retirement. |
| The plugin store changes during selected validation, before BGP reconciliation seals the Session | `TestIncomingPluginChangeDuringSelectedValidation` | Blocking a callback is public; pausing between atomic store publication and BGP reconciliation is not. A complete operator reload also runs the census and cannot isolate the post-RPC freshness check. The Go test drives actual TCP plus bridge and JSON RPC transports; neither mode claims a child-process run. |
| Read storage is unavailable at the first or following incoming read | `TestIncomingCandidateReadPoolBackpressure`, `TestIncomingCandidateReadPoolWaitEnds` | The private pool probes fence the exact standard/extended Get while its handles remain owned. An operator cannot select that handle-acquisition boundary. Actual listener/TCP behavior still proves same-socket resumption and original deadline/cancellation, with fixture-owned pool balance checked after all owners join. |
| Delivery fails after adoption or partway through a policy continuation | `TestReceivePublicationAbortReleasesReadOwnership`, `TestReceivePublicationAbortProtectsRetainedReadOwnership`, `TestReceivePublicationFollowPanicPreservesPrimaryOwnership` | A nil receiver or panic inside Ze's worker/read handoff is an internal fault, not a public daemon operation. Controlled hooks exercise those exact faults with real pooled receipts, ordinary/coalesced read owners and retained-buffer survivors. An external plugin exit would exercise a different failure boundary. |
| Late activation, expiry or pressure scanning meets an aborted retained entry | `TestAbortedPublicationRetainSurvivesLateLifecycle` | The cache lifecycle and fake-clock scan are internal resource boundaries. This cheap guard test supplements the real receive-ownership paths; it does not replace their entry-driven proof. |
| An incoming TCP exchange crosses selected-Session initialization | `TestPassiveIncomingAcrossSessionStart` | An external peer can connect early, but a daemon script cannot hold the exact internal start interval reliably. The controlled callback fences that interval while the actual listener exchanges bytes; the ordinary passive control proves useful acceptance without the gate. The role daemon carriers separately require public ACTIVE as their scenario precondition and do not substitute for this startup race proof. |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `bgp-delay-open` | `test/plugin/bgp-delay-open.ci` | AC-1, AC-1a, AC-1b, AC-1c through the daemon listener and operational FSM state | Current-source clean–broken–clean passed; the premature-OPEN mutation failed at the observer. All 80 loaded draft invocations executed and passed without skips. Promoted live selection passed all five assertions. Historical TIME30 remains undiagnosed. |
| `bgp-delay-open-bfd-strict` | `test/plugin/bgp-delay-open-bfd-strict.ci` | AC-4, AC-5: daemon OPEN exchange, public pending sub-state, withheld KEEPALIVE, real BFD Init datagram causes Up and establishment | Current-source clean–broken–clean passed on Ze's matching kernel/image; the bypass mutation failed at the pending-state assertion. All 80 loaded draft invocations executed and passed without skips. The promoted live Linux selection passed, with one test executed and zero skips. Historical bg_508 TIME30 remains undiagnosed. |
| `role-incoming-open-isolation` | `test/plugin/role-incoming-open-isolation.ci` | AC-1o: actual daemon/internal bgp-role, original Customer socket, mismatched competing Provider refusal2/11, permitted roleless/Customer policy controls that lose collision6/7, and fresh OTC65000-stamped export after each outcome | Final rebuilt baseline passed; all eighty loaded invocations executed and passed. Promoted unchanged at 17:07 UTC. Native clean/broken qualification detected loss of the original Customer export after mismatched OPEN; promoted live selection passed in job-role-final-live-controls-b6a803dc.log. The surrounding ten-carrier run passed eight and failed the two separately diagnosed stamp fixtures; it is not an all-green suite claim. |
| `role-incoming-open-isolation-forked` | `test/plugin/role-incoming-open-isolation-forked.ci` | The same AC-1o wire/export assertions with the actual bgp-role subprocess over TLS/RPC | Explicit external bgp-role configuration passed its final rebuilt baseline and all eighty loaded invocations. Promoted unchanged at 17:07 UTC. Native clean/broken qualification detected loss of the original Customer export after mismatched OPEN; promoted live selection passed in job-role-final-live-controls-b6a803dc.log. The surrounding ten-carrier run passed eight and failed the two separately diagnosed stamp fixtures; it is not an all-green suite claim. |
| `incoming-preopen-update-isolation` | `test/plugin/incoming-preopen-update-isolation.ci` | AC-1t: a private pre-OPEN withdrawal cannot alter actual Adj-RIB-In or original-socket export; an Established source can still withdraw its route | Final-source baseline and all eighty loaded draft invocations passed, then the carrier was promoted. The native discriminator accepted clean PASS in 2.0 seconds and broken FAIL in 2.0 seconds: the old predicate exposed an unexpected withdrawal at the original-source FIFO fence. Final unique live selection passed13/13 with zero skips in7.6 seconds. |
| `incoming-open-learned-isolation` | `test/plugin/incoming-open-learned-isolation.ci` | AC-1o: a losing IPv6-only OPEN cannot replace the internal route server's selected IPv4 family/ASN or interrupt fresh export to the original socket | Final-source baseline and all eighty loaded invocations passed, then the carrier was promoted. Native clean PASS in 2.7 seconds and broken FAIL in 16.4 seconds were accepted. After collision6/7, the broken build logged IPv4 family mismatch and failed fresh original-socket export; later operational-state checks were not reached on that broken run. Final unique live selection passed13/13 with zero skips in7.6 seconds. |
| `incoming-open-learned-isolation-forked` | `test/plugin/incoming-open-learned-isolation-forked.ci` | The same learned-family and useful-forwarding boundary with actual external bgp-rs over TLS/RPC | Final-source baseline and all eighty loaded invocations passed, then the carrier was promoted. Native clean PASS in 1.5 seconds and broken FAIL in 16.5 seconds were accepted. After collision6/7 and EOF, forked bgp-rs logged IPv4 family mismatch and the original socket missed the fresh route. Later broken-state assertions are not credited. Final unique live selection passed13/13 with zero skips in7.6 seconds. |
| `role-otc-egress-stamp`, `role-otc-egress-stamp-forked` | `test/plugin/role-otc-egress-stamp.ci`, `test/plugin/role-otc-egress-stamp-forked.ci` | Existing exact OTC65000, AS_PATH, next-hop and destination-NLRI wire assertions on both process transports | Fixture maintenance only: independently approved destination `next-hop unchanged` makes the existing 1.1.1.1 expectation explicit. Both corrected drafts passed their baseline and all eighty loaded invocations, then were promoted. Every original assertion remains. Both passed in the final unique13 live selection, zero skips; native approval bindings remain separately recorded. |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `bgp-delayopen-frr` | `test/interop/scenarios/` | Pinned FRR 10.7.1 | With `timer delay-open` set on Ze, the capture shows Ze's OPEN only after FRR's OPEN or DelayOpen expiry, and both speakers establish in each configured delay ordering | Native Ze Docker guest clean–broken–clean passed. The eager-OPEN build failed with captured premature OPENs on both peer paths. The restored complete capture has one live OPEN/KEEPALIVE winner per peer and zero drops; its native suite report retains both image identities. |

Row 7 is not wire-visible; its proof is the reactor unit test through a real `Peer`. The scenario owes the revert, rebuild, red, restore, green walk with the red recorded.

## Files to Modify

This approved design list names the original edit targets. The current ownership
paths below supersede the old pending-resolver details.

- `internal/component/bgp/fsm/fsm.go`, `internal/component/bgp/fsm/state.go`, `internal/component/bgp/fsm/timer.go`, `internal/component/bgp/reactor/session.go` - rows 6, 7, 6s (the fsm.go header comment that records DelayOpen and Event 3 as absent changes with the code; every comment listed in Current Behavior that calls the DelayOpen sub-states unreachable is rewritten; the DelayOpenTimer joins the timers in timer.go; state.go declares the two Connect/Active sub-states)
- `internal/component/bgp/reactor/peer_run.go`, `internal/component/bgp/reactor/peer.go` - row 7: `Peer` gains `cycleReachedEstablished`; `runOnce` clears it before the session starts; the FSM state callback sets it beside `PeerStateEstablished`; the next `runOnce` consumes it whatever `err` is and fires Event 3 or Event 6
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
- `internal/component/bgp/reactor/incoming_candidate.go`, `internal/component/bgp/reactor/session_read.go` - AC-1q: incoming pressure waits reuse selected-session backpressure while preserving the original first-OPEN deadline and later hold/cancellation owners
- `internal/component/bgp/reactor/recent_cache.go`, `internal/component/bgp/reactor/peer_run.go`, `internal/component/bgp/reactor/reactor_notify.go` - AC-1r: failed publication releases abandoned adopted receipts and queued work, closes new lookup/retain admission, and preserves existing retained receipt and forwarding buffers
- `internal/component/bgp/reactor/peer_run.go`, `internal/component/bgp/reactor/peer_connection.go`, `internal/component/bgp/reactor/session_connection.go` - AC-1s: preserve an admitted passive connection across selected-Session startup without constraining the internal ordering mechanism

## Files to Create
- `test/plugin/bgp-delay-open.ci`
- `test/interop/scenarios/bgp-delayopen-frr/`
- `test/plugin/bgp-delay-open-bfd-strict.ci`
- `test/plugin/role-incoming-open-isolation.ci`, `test/plugin/role-incoming-open-isolation-forked.ci` - AC-1o, after draft qualification and promotion
- `internal/test/fixture/role_incoming_open.go` - actual daemon role/export scenario with competing incoming TCP exchanges and useful routes on the original socket
- `internal/le/interoplab/bgp/check_delayopen.go` (and its `_test.go`) - the `bgp-delayopen-frr` checker: parses the capture and asserts Ze's OPEN follows FRR's OPEN, or follows the TCP handshake by at least DelayOpenTime when FRR holds too
- `internal/component/bgp/reactor/incoming_read_buffer_wait_test.go` - AC-1q: pressure resumption and bounded cancellation on the actual listener
- `internal/component/bgp/reactor/receive_publication_abort_test.go` - AC-1r: abandoned publication reclamation and retained-consumer survivors with real pool handles
- `internal/component/bgp/reactor/receive_publication_follow_panic_test.go` - AC-1r: preserve an earlier successful publication and its original pooled receipt across a later policy continuation panic
- `internal/component/bgp/reactor/recent_cache_abort_lifecycle_test.go` - AC-1r: retained ownership survives late activation, explicit expiry and pressured gap scanning
- `internal/component/bgp/reactor/peer_passive_start_admission_test.go` - AC-1s: early and ordinary passive acceptance through the actual listener

### Current incoming-connection ownership files

| Concern | Current producers |
|---------|-------------------|
| Admission, semantic OPEN receipt and same-Session selection | `incoming_candidate.go`: `handlePendingCollision`, `resolveIncoming`. `incoming_candidate.go`: `adoptIncoming`. `session_open_admission.go`: OPEN admission and advancement. |
| Immutable OPEN identity and Session-owned identifier claims | `session_open_proposal.go`: `preparedOpen`. `session_negotiate.go`: `prepareOpen`. `routerid_unique.go`: `routerIDClaims.claim`, `releaseSession`. |
| Capture input, proposal identity and membership exclusion | `capture_pending.go`: `prepareSessionCapture`, `pendingSessionCapture`. `peer_removal.go`: `removalLocked`. |
| Original hold deadline and coded OPEN refusal | `session_hold_expiry.go`: `holdTimerExpiredLocked`. `session_open_validation.go`: `runOpenValidator`. |

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
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/process-protocol.md`, OPEN Validation: the provisional flag and selected replay match ValidateOpenInput and selectNegotiationEpoch |
| 5 | Plugin added/changed? | Yes | `docs/architecture/meta/role.md`, Learned role lifecycle: provisional validation preserves incumbent learned state |
| 6 | Has a user guide page? | Yes | `docs/guide/bgp-peering.md` if it describes session timers |
| 7 | Wire format changed? | No | the OPEN is unchanged; only its timing |
| 8 | Plugin SDK/protocol changed? | Yes | `docs/plugin-development/protocol.md`, validate-open callback: the same provisional semantics on both transports |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc4271.md`, `rfc/short/draft-ietf-idr-bgp-bfd-strict-mode.md` (row 6s), and `docs/features/rfc-status.md` (generated by `./le rfc update`) |
| 10 | Test infrastructure changed? | Yes | `internal/test/fixture/role_incoming_open.go` registers the daemon observer; the controlled lifecycle seams above explain internal-fault coverage |
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

Main also owns scoped FSM/reactor tests and lint, the BGP component and functional
suites, and source-anchor maintenance. Agents run no `./le verify` command.
At the 2026-10-27 checkpoint, base `bgp-delay-open`'s original promoted TIME30
was unresolved and its carrier was not promoted; FRR interop also remained owed.
Those historical TIME30 results remain undiagnosed. The current 2026-11-16
qualification is separate: both canonical carriers passed clean–broken–clean,
all eighty loaded invocations each and their promoted live selections; pinned
FRR 10.7.1 also passed clean–broken–clean with complete capture and image
identities. The tables above record those results. This spec stays in progress
for the remaining RFC, retry and boundary proofs and the independent closure
phase; the historical strict-mode evidence alone certifies none of those.

**2026-10-27 Event 3 reconciliation:** no production implementation was needed.
Three new real-Peer units fill the inbound-negative, hold-expiry/inbound and
passive reconnect gaps. Initial failures exposed test setup errors: observation
began before the initial EOR, and the teardown peer did not finish its TCP
close. The repaired setup validates EOR and Cease, closes the remote endpoint,
and retains the original counter, state and expiry assertions. The repaired
positive race selection passed in 7.515s; independent review found no issue.

Main recorded three hand-written clause mutations through native `rfc record`.
Zeroing a failed inbound retry changed the required 1 to 0; damping an
Established retry changed the required 0 to 1, including all three passive
exit cases. `rfc/discrimination/rfc4271/RFC4271-8.2.2-20.json` carries these
records. The focused `rfc discriminate id RFC4271-8.2.2-20` inventory reports
eight records and no stale or unproven entry. Evidence is retained in the
session's `scratch/job-event3-*-record-*.log`,
`scratch/job-event3-record-inventory-51ab73b0.log` and
`scratch/job-delayopen-event3-positive-repaired-1a9acda3.log`.
This does not clear either promoted TIME30 or certify the whole RFC ledger.

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
| Incoming pressure preserves useful connections | Named AC-1q listener roots pass under race, including same-socket resumption and deadline/cancel controls; repeated scoped controls restore their pool baseline |
| Failed publication ends only abandoned ownership | Named AC-1r roots pass under race in both read modes; no abandoned cache entries/slots remain after joins, retained buffers stay intact until last release |
| Startup preserves a useful passive connection | Named AC-1s root passes under race: early and ordinary controls establish the same admitted socket/Session with one OPEN exchange and one establishment |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | `delay-open` range enforced at commit |
| Resource exhaustion | A held connection is bounded by its original timers and read deadline. Incoming read pressure waits without resetting TCP; failed publication returns abandoned buffers while preserving active retained borrows. No pool-limit increase or global reset substitutes for ownership cleanup. |
| Error leakage | none: no peer-supplied bytes parsed differently |

Closure source review found no additional security issue in the selected
corrections. YANG and `parsePeerSettings` both bound DelayOpen to 0..240 seconds.
`readFrame` validates the original header before body reads; OPEN observations
own at most two bounded bodies and never retain a read-pool handle. Candidate
admission stays bounded, original deadlines survive blocking callbacks, and
expiry or cancellation retires the same owner. Replay invokes observers
outside the epoch mutex, with the OPEN owner and joined private reader
preserving its ordering. Failed UPDATE publication closes new cache admission
while existing retains keep their bytes until release. The changes add no
shell, SQL, filesystem-path parser, credential, cryptographic primitive or
privilege grant. Selected-snapshot dependency review is complete. Scoped
lint ran with no selected whole/hunk finding; its global failures remain
disclosed below. The native artifact records the independent final verdict.

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
| One spec or several? | "Split into three" | This spec owns rows 6 and 7; the defects stay in `spec-bgp-unowned-rfc-rows`, EPE is `plan/post-release/features/spec-bgp-epe-peer-segments-and-srv6-peernode.md` |

## Open Questions (owner)

None open.

## Design Insights

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| `delay-open` range 0 or 1..240, 0 meaning off | A separate enable leaf | FRR's range; one leaf carries both the attribute and the timer |
| Event 3 vs Event 6 read from the cycle's Established flag | A second restart timer | A second timer would be a second declaration of the same fact; the flag records the one fact Event 3 needs, that the cycle reached Established |
| Two OPEN-less held connections are not a collision; the first OPEN decides (AC-1i, round-2 I1) | Close the inbound at once as a collision; keep both and compare in OpenSent | RFC 4271 §6.8 compares BGP Identifiers "Upon receipt of an OPEN message", so it has nothing to compare before one arrives; the existing pending slot already waits for the inbound OPEN, and closing an OPEN-less held connection loses no state |
| When the peer's OPEN arrives first on the inbound, Ze closes its OPEN-less held outbound at once (AC-1i order b; owner, 2026-10-11) | Keep both connections until one reaches Established, as FRR does | RFC 4271 §6.8 governs only "connections that are in the OpenConfirm state" and, optionally, OpenSent, so dropping an OPEN-less Connect or Active connection is outside it and not forbidden by it. BIRD's `bgp_rx_open` (`proto/bgp/packets.c`) stops the other connection when it is still in Connect or Active ("Stop outgoing connection attempts"). FRR keeps both through its doppelganger peer: `bgp_collision_detect` (`bgpd/bgp_packet.c`) finds no collision unless the other connection is OpenSent or OpenConfirm, and the loser is discarded only at `bgp_establish` through `peer_xfer_conn` (`bgpd/bgp_fsm.c`). Ze now gives each admitted TCP connection its own Session. The owner-selected immediate retirement policy remains: an incoming first OPEN retires an OPEN-less held incumbent |
| Event 3 predicate: an explicit `cycleReachedEstablished` flag the session sets on entering Established, consumed by the next `runOnce` whatever the error (round-2 B1) | "The peer has not been oscillating", which Ze keeps as no state; the backoff delay value; the loop's `runOnce`-returned-nil arm | The nil arm runs only when the peer is stopping, because `Session.Run` returns non-nil on every exit, so a predicate tied to it never fires; `inboundNotify` resets the delay without proving a session ran, so the delay value would misfire Event 3. The flag is set where the fact happens, the FSM's Established transition, and read where the decision is made |
| Passive peers take Event 3 through the existing Event 1/4 conflation | A distinct Event 5 path | `fsm.go` already serves Events 1 and 4 from one arm with `passive` choosing the next state; adding Event 3 to that arm keeps one declaration, and the arm's comment names the conflation. Event 5 is not added: nothing in Ze fires it (round-3 N2) |
| Event 18 in Active follows the RFC's four actions | FRR's `bgp_ignore` for TCP_connection_open_failed in Active | The row is RFC4271-8.2.2-13; FRR is laxer than the RFC here and the RFC outranks the peer implementation |
| Row 6s implemented, not refused (owner, 2026-10-11) | Refuse `delay-open` with `bfd strict` at commit | Owner decision; the strict-mode decision (`advanceAfterOpen`) is reused so the "BFD neither Up nor AdminDown" test lives in one place |

## Known Limitations
- Incoming admission remains bounded to one pending exchange per peer, including joined refusal and reserved handoff. This is a capacity policy, not an RFC4271-8.2.1-3 conformance gap: Section 8 explicitly permits different internal structures with the same externally visible behavior. AC-1j/k preserve an admitted connection's OPEN exchange and failure isolation; the two existing tagged bodies remain intact. Additional sockets need no otherwise unused Session object merely to be refused. Capacity refusal before OPEN uses RFC 4486 Section 4 Connection Rejected (6/5); a real OPEN-based collision loser retains Connection Collision Resolution (6/7). Main observed the exact 6/7 RED before correction, then three-root GREEN and nine roots repeated twenty times under the race detector (180 passes). Both tagged claims have renewed native clean-PASS/broken-FAIL records; the independent one-row rejudgment records enforced with its structural-error correction in the upgrade reason. The owning state retains exact commands and receipts. Separate closure findings on provisional plugin publication and pooled frame transfer remain in progress under AC-1o/p.
- BIRD has no DelayOpen, so the DelayOpen interop proof is against FRR only.

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
- [ ] All 34 AC rows, including AC-1a..AC-1t, AC-2p, AC-7h and AC-10..AC-12, demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] Scoped package and relevant functional/interop tests pass; GitHub CI supplies whole-tree verification after an authorized push. Agents do not run any `./le verify` command or test a copy of the tree
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

## Observed incoming-listener tests: 2026-11-15

This is the original RED observation. The independent incoming Session now
supplies that connection's OPEN and retains its lifecycle through adoption;
the current listener and ownership evidence is recorded above and in the
owning state. The historical shared log remains evidence for its other owners.

The four new `TestRFC4271ListenerIncoming...` probes compiled and ran in
`tmp/session/2026-11-14-210034/scratch/job-core-bgp-lifecycle-advertisement-bdcd2163.log`.
Initial passive/default OPEN, configured DelayOpen expiry, and remote OPEN
before expiry each complete their handshake and pass. The second incoming
connection while an outgoing connection is OpenSent fails waiting for its
own local OPEN.

The source cause is `acceptOrReject` handing that socket to
`handlePendingCollision`, which reads the remote OPEN without starting the
connection's own OPEN/DelayOpen lifecycle. RFC 4271 Section 8.2.1 states:
"For each incoming connection, a state machine MUST be instantiated."
The test reaches the actual Reactor listener; it creates no substitute FSM.
FRR and BIRD source research supports an incoming connection alongside an
outgoing candidate, but no versioned external interop run is claimed here.

This records the completed investigation for later implementation review,
under the owner's test-first campaign instruction. Status, existing approval
gates and historical handoff metadata remain unchanged. Review the release
classification of this demonstrated connection-lifecycle defect separately
from optional timer enhancements; placement in this feature spec does not
establish that the defect is optional.

Retain the shared log until all owning specs that cite it have superseding
evidence. No duplicate report is created.

### Pending failure preserves useful established traffic

The test-first campaign adds the untagged
`TestPendingFailurePreservesEstablishedSession` in
`internal/component/bgp/reactor/pending_established_survival_test.go`.
The older `TestSessionRFC4271EstablishedCollisionWaitsForOpen` proves route
delivery while a pending OPEN is incomplete, followed by collision rejection
and an Established state. This addition requires a fresh, distinct route to
reach the original session's semantic UPDATE callback after pending cleanup.
It also requires the original Session and connection to remain installed.

The four inputs are a complete frame with a truncated optional-parameter
envelope (exact OPEN Error 2/0), a partial OPEN followed by transport EOF,
an expired pending read deadline, and a valid OPEN control (exact collision
Cease 6/7). EOF and expiry require silent cleanup. The deadline case uses
the existing offset fake-clock technique from
`TestDelayOpenPendingDeadlineOnlyDropsPending`: production still sets and
enforces the actual socket deadline. A delegating socket observer records
that deadline, write attempts and the producer's close; it supplies no read
result and leaves all socket operations real.

RFC 4271 Sections 6.2, 6.8 and the Established arm of 8.2.2 govern the
distinction between malformed OPEN refusal, valid collision resolution and
the established connection. Every subcase first requires a useful accepted
route, then pending cleanup, then another useful announcement with no
synthesized withdrawal. The valid control prevents wholesale rejection of
pending input from satisfying the malformed-OPEN expectation. The test is
additive and does not change the earlier tagged proof or the spec's approval
and closure gates. Independent source review found no remaining issue. The
main-thread race-enabled run passed all four named subcases in
`tmp/session/2026-11-14-210034/scratch/job-pending-established-survival-ac5b4f3e.log`.
Each observed the expected pending cleanup and useful traffic afterward.
This direct admission and session probe does not prove the listener accept
loop or every Established-state event, and earns no new RFC-row credit.
Retain the log through the campaign's final evidence audit; the committed
test and this result then supersede the temporary console output.

The separate `TestSessionRFC4271EstablishedCollisionWaitsForOpen` directly
checks the lower bound in RFC4271-8.2.2-19: the second connection stays
tracked while its OPEN is incomplete. Its positive native observation passed
cleanly, then failed at the partial-OPEN write when the private producer
overlay closed that pending socket immediately. The named assertion reported
`io: read/write on closed pipe`, not a build failure or a test timeout.
`rfc/discrimination/rfc4271/RFC4271-8.2.2-19.json` retains the native record;
`tmp/session/2026-11-14-210034/scratch/job-pending-until-open-native-proof-c5344cfa.log`
retains the clean and broken console output through the campaign audit.
The completed-OPEN control checks collision Cease
6/7 and pending cleanup, which Section 6.8 governs; neither a wrong
notification code nor a stale internal pending slot independently violates
the tracking-until-OPEN sentence. No negative discrimination record or
whole-row enforcement is inferred from those controls. RFC6608's existing
same-session unexpected-message proofs remain separate evidence.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `callback-reenters-held-lock.md` | 2026-10-30 | strict BFD release holds `openMu` across the negotiated publication, so the read loop's close waits on it |
| `claim-outlives-the-evidence-it-cites.md` | 2026-10-31 | RFC4271-8.2.2-15 WEAK: no tagged unit runs a DelayOpenTimer in Connect |
| `lifetime-ends-before-its-last-reader.md` | 2026-11-04 | AC-1e / RFC4271-8.2.2-13: Session `StopAll` cancels Active Event 18's restarted initial-value retry; Peer uses independent backoff |


## Implementation Summary

### What Was Implemented
- Configured DelayOpen and the strict-BFD pending paths, with the original
  retry deadline preserved through cleanup. `Peer.runOnce` selects AutomaticStart
  after an Established cycle and a damped restart after a failed one.
- Independent incoming handshakes retain their original Session, socket,
  proposal, reader and deadlines through collision selection and cleanup.
  Capacity refusal uses Connection Rejected; genuine collision uses Connection
  Collision Resolution. This is a bounded admission policy, not an object-count
  conformance claim.
- Provisional plugin validation preserves incumbent learned state; selection
  publishes the retained OPEN pair once. Reload checks current, pending and
  inbound owners, including constructors and blocking selected validation.
- Frame reads transfer pooled handles by value. Incoming reads wait for
  storage without reconnecting; publication failure ends abandoned ownership
  while preserving active retained buffers and earlier successful publications.
- A fresh selected Session is started and its publication callback installed
  before TCP acceptance can see it. Live-router fixtures join all owners before
  restoring their validation provider.
- Shared UPDATE admission now withholds route publication until that
  connection is Established. AC-1t has actual listener race and daemon
  clean/broken evidence; final unique13 live controls passed; selected-source native RFC currentness is complete.

### Bugs Found/Fixed
- Candidate role publication: `TestProvisionalOpenPreservesSelectedRole`
  and actual daemon role-isolation carriers preserve useful OTC export.
- Per-frame wrapper allocation: `TestReadFrameUsesPooledStorage` measured one
  allocation before the value transfer and zero afterward on both pools.
- Stale incoming plugin offers: constructor, typed-handoff and selected-RPC
  reload roots reject changed in-use offers while retaining useful controls.
- Delivery-channel publication race: the existing Peer lock now owns its
  installation, snapshots and clear; the selected validation race root passed.
- Failed receive publication and later continuation panic: the real pooled
  receipt/variant tests preserve active ownership and restore the pool baseline.
- Pool pressure: `TestIncomingCandidateReadPoolBackpressure` resumes the same
  incoming socket; `TestIncomingCandidateReadPoolWaitEnds` preserves its bounds.
- Passive startup and observer loss: `TestPassiveIncomingAcrossSessionStart`
  retains the handshake and exactly-once ordered public event delivery.
- Pre-established UPDATE escape: seven intended failing leaves preceded the
  shared admission correction; twelve related roots then passed twenty race
  repetitions each. The real-daemon old-predicate break also failed at the
  useful incumbent's original-socket route assertion.
- Decoded private OPEN escape: the generic bounded publication epoch retains
  both actual OPEN directions until selection. Both daemon transports have
  accepted clean/broken useful-export discrimination. The exact borrowed-body
  alias break failed at overwritten bytes, then all six canonical ownership,
  decoded and SDK roots passed twenty race repetitions each. Final live
  controls passed13/13 without skips. Lint completed with findings outside the
  selected whole files/hunks; selected-source native RFC currentness is complete.

### Risks and observations retained

AutomaticStart intentionally resets pacing history after an Established cycle;
failed-cycle controls preserve damped restart history. A strict BFD wait can
remain in Connect/Active, but its original negotiated HoldTimer or zero-hold
BfdHoldTimer bounds it. Concurrent collision captures still cannot identify a
winning TCP connection from portless BGP4MP endpoint fields alone; retained
original frames and explicit ambiguity remain the documented answer. No new
full RFC or external strict-BFD interoperability claim follows from these tests.

### Documentation Updates
- FSM/configuration guides describe DelayOpen, automatic restart and all four
  strict-BFD pending states; the implemented RFC rows name their actual proof.
- `peer-lifecycle.md` anchors incoming ownership, reload census, provisional
  validation, original retry budgets and complete startup publication to the
  corresponding reactor producers. `process-protocol.md` and the SDK protocol
  page describe the provisional RPC contract on both transports.
- `buffer-architecture.md` and `memory/lifetime-contracts.md` anchor frame value
  transfer, aborted cache admission and retained-buffer/continuation ownership.
- `features/bgp-protocol.md` names the four daemon carriers and pinned FRR
  scenario; its overall scope remains partial and its level experimental.
- The two owned open journal Fix cells now name their implemented corrections;
  sibling spec citations point to durable lifecycle prose or retain historical
  provenance as a bare stem. A news fragment records operator-visible behavior.
- Final light documentation check ran and reported catalog, command-mapping,
  summary and source-anchor findings. No clean global documentation result is claimed.

### Deviations from Plan
- Real listener review replaced the old pending-OPEN resolver with one owned
  incoming Session and typed transfer receipts. Compatibility buffered OPENs
  remain a separately tested entry. The retained single admission slot is a
  resource bound, not a scope reduction.
- The owner's behavioral RFC interpretation is recorded in
  `ai/META-DECISIONS.md`; review judges unmet behavior, not literal FSM structure.
- Retry, capture/removal, plugin publication, reload, receive ownership and
  startup defects encountered on the required paths were repaired in this
  spec. AC-1t carries the received-UPDATE admission correction. Its final
  closure checks remain in progress; no behavior is intentionally deferred.

## Mistake Log

<!-- One table, one place. Ship the `none` row and either replace it or leave it
     deliberately: three separate empty tables produced three separate 67-82%
     untouched rates, because an empty table asks nothing.
     Kind: assumption (a broken A-N) | approach (a route abandoned) | escalation
     (a mistake frequent enough to deserve a rule). -->
| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | Original pending handling read a remote OPEN without owning that connection's local handshake | The admitted TCP connection needs its own bounded handshake and transfer owner | Actual listener regressions and producing code | Retain one independent Session and its immutable inputs through selection |
| approach | Original review rationale treated missing extra FSM objects as a conformance gap | The owner requires applicable externally visible behavior, with RFC updates and errata | Owner clarification and primary RFC review | Record the policy in META-DECISIONS and reassess actual incoming behavior |
| approach | Provisional policy callbacks could overwrite the incumbent's learned role | Validation and selected publication have different ownership | Actual SDK/TCP and daemon OTC export regressions | Carry the generic provisional flag and replay the selected OPEN pair once |
| approach | Receiver failure and a later policy continuation obscured transferred read-buffer ownership | Admission closure must preserve existing retains; a later failure cannot revoke a successful primary | Real-slot REDs and continuation double return | Abort only abandoned receipts; carry exact ownership through panic unwind |
| approach | Starting a published Idle Session lost TCP success; starting earlier alone lost its public OpenSent observer | Readiness includes the publication callback before external acceptance | Actual listener and IPC observer REDs | Start with no callback, install it through its noninvoking setter, then publish under the existing admission lock |
| approach | The shared unexpected-UPDATE guard covered only OpenSent/OpenConfirm | Connect/Active DelayOpen paths can receive UPDATE before route publication is permitted | Formal Round1 source trace, actual listener failures and real-daemon withdrawal | Repaired shared admission under AC-1t, preserving ROUTE-REFRESH semantics and useful Established traffic; final selected-source currentness is verified |
| approach | Adding the decoded-OPEN tests overwrote an existing untracked test file without first reading it | The existing file already held three SDK validation roots and its selected-callback gate | Main compared the write with retained literal tool inputs | Reconstructed the exact original 301 lines, kept the two new roots in their own file with unique helper names, and reran all three original SDK roots successfully |

## Implementation Audit

<!-- BLOCKING before closure. See ai/rules/completion.md.
     Status: Done (with file:line) | Partial | Skipped | Changed.
     Partial and Skipped both require explicit user approval. -->

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Configured DelayOpen, including real retry timing | Done | `Session.connectionEstablished`, `delayOpenExpired`, `handleConnectionCloseLocked`; `Peer.run` | Base daemon and pinned FRR proof; real-Peer held/strict retry controls retain the original timer interval. |
| Automatic start after an Established cycle | Done | `Peer.runOnce`, `FSM.handleIdle` | Real-Peer active/passive restart and failed-cycle controls; four current-source runtime records renewed in this closure batch. |
| DelayOpen strict-BFD enter, release and bounded failure | Done | `Session.advanceAfterOpen`, `handleBFDEvent`; `FSM.EnterDelayOpenBfdUpPending` | Strict daemon, candidate BFD and ordinary/zero-hold/nonwaiting controls. |
| Independent incoming ownership, collision and capture removal | Done | `incomingCandidate`, `Peer.resolveIncoming`, `Session.adoptIncoming`, `prepareSessionCapture`, `Reactor.removalLocked` | Actual listener, same-Session adoption, collision/refusal, capture and shutdown controls. |
| Provisional validation, selected learned publication and reload freshness | Done | `Session.selectNegotiationEpoch`, `privateNegotiationEpoch.messageCallback`, `peerOpenSnapshot.reconcilePluginOffers` | SDK transports, exact-body ownership unit and both real RS process modes discriminate useful selected behavior. |
| Pooled-frame value transfer, pressure and failed-publication ownership | Done | `receivedFrame`, `incomingCandidate.readFrame`, `RecentUpdateCache.abortPublication`, `finishReadBuffer` | Allocation and real-pool fault controls retain useful borrowers and reclaim abandoned ownership. |
| Passive startup, diagnostics and fixture lifecycle | Done | `Peer.runOnce`, `Session.runOpenValidator`, `lowLiveRouter` cleanup | Startup observer ordering, coded refusal/hold diagnostics and joined provider cleanup controls. |
| UPDATE admission before establishment | Done | `updateIsUnexpected`, both read rails | Intended listener and daemon failures, restored race controls, useful Established withdrawals and one bounded positive RFC record. |
| Current RFC records, audit verdicts and final closure checks | Done | RFC discrimination/audit records; Review Gate below | All41 native identities, five published-Peer.runOnce retakes and16 independent audit rows/8stems are accepted. Final update/tree/citation passed with no generated diff. The full-WT RFC check exited1 with425 global findings; exact owned-ID/name readback found only RFC4724 SHIFTED from excluded surrounding edits. Native reseal against HEAD f1d64ff442 confirmed fresh/0shifted/0refused; the selected interop blob is74ea9188c288c2103510392e66c2aaf06b799ed7. Four-package lint exited1 (HOST16/LINUX16/CAP8 on13 paths), with no selected whole/hunk finding; the Session.go finding is absent from its pinned HEAD body. Doc/repo/weakening global findings remain disclosed, including38 weakening findings with no selected-path intersection. |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestDelayOpenConfigBoundary`, `TestDelayOpenInheritanceToPeerSettings`; YANG `delay-open-timer` | Zero/off and upper-bound refusal; runtime parse range agrees with schema. |
| AC-1a | Done | `TestRFC4271DelayOpenHeldConnectionWritesNoOpen`, Connect/Active event tests | Actual held socket sends no early OPEN. |
| AC-1b | Done | `TestRFC4271DelayOpenTimerExpiry`, Connect/Active event tests | Timer expiry sends the one local OPEN and advances. |
| AC-1c | Done | Connect/Active event tests, `bgp-delay-open` | Remote-first OPEN clears delay/retry and performs the permitted handshake. |
| AC-1d | Done | `TestRFC4271DelayOpenConnectFailureClearsTimer`, `TestDelayOpenConnectFailureRetriesAtInitialValue` | Connect failure preserves listening and original retry ownership. |
| AC-1e | Done | Three real-Peer held/strict/ordinary retry roots | Active Event18 retains its actual retry budget; OpenSent retains ordinary backoff. |
| AC-1f | Done | `TestRFC4271DelayOpenVersionError` | Both held states preserve the version-error counter distinction. |
| AC-1g | Done | Connect catch-all and duplicate-start control | Delay timer clears only for the rejecting event, not an ignored start. |
| AC-1h | Done | `TestRFC4271DelayOpenPreBufferedOpen` | Both connection modes retain Event17-before-Event20 behavior. |
| AC-1i | Done | `TestRFC4271DelayOpenBothSidesHold`, actual listener collision roots | Both identifier orders and OPEN-less incumbent policy remain useful. |
| AC-1j | Done | Actual incoming-listener default/delayed OPEN roots | Independent wire exchange beside OpenSent; original remains useful on failure. |
| AC-1k | Done | Adoption/collision panic, terminal selection, BFD, capture and identity roots | Original Session/reader/deadlines survive selection; joined retirement covers failure. |
| AC-1l | Done | Connected-event ordering and candidate metadata roots | Exactly-once connection-specific public event precedes handshake publication. |
| AC-1m | Done | `TestRemovedPeerFactoryCannotCaptureReplacement` | Registry membership cut prevents late old capture registration. |
| AC-1n | Done | `TestIncomingCapacityRefusalPreservesConnections` | Exact capacity6/5 and true collision6/7 preserve the useful exchange. |
| AC-1o | Done | Six final OPEN race roots; role and internal/forked RS daemon carriers | Both decoded directions, exact copied bodies, selected policy and useful export. |
| AC-1p | Done | `TestReadFrameUsesPooledStorage` | Warmed standard/extended reads allocate no separate frame wrapper. |
| AC-1q | Done | Read-pool backpressure and bounded-wait roots | Same socket resumes; original deadline/cancellation remain effective. |
| AC-1r | Done | Abort/retained/follow-panic/lifecycle ownership roots | Abandoned receipts end; earlier publications and existing borrowed buffers survive. |
| AC-1s | Done | `TestPassiveIncomingAcrossSessionStart` and real IPC observer | Same early TCP exchange establishes with complete ordered public transitions. |
| AC-1t | Done | Twelve-root race selection, pre-OPEN daemon and bounded tagged listener proof | No route publication before Established; useful incumbent and valid withdrawal remain. |
| AC-2 | Done | Active automatic-start and failed-cycle real-Peer roots | Established-cycle Event3 zeroes; failed-cycle damped start preserves counter. |
| AC-2p | Done | `TestRFC4271PassiveAutomaticStartReconnectZeroesCounter` | Passive restart covers notification, hold expiry and teardown. |
| AC-3 | Done | Rows13/20 have no gap and both polarities are tagged | All native identities/audits accepted; final exact owned RFC readback clear at the selected snapshot, with the RFC4724 surrounding-source exception confirmed fresh against HEAD. The425 global findings are not called green. |
| AC-4 | Done | FSM and Session strict DelayOpen enter tests; strict daemon | Both states and negotiated zero/nonzero hold values. |
| AC-5 | Done | All strict release/nonwaiting/actual-Peer roots | Up, AdminDown and Disabled release once; immediate peer KEEPALIVE is serialized. |
| AC-6 | Done | Strict failure tests and Session bounded-wait root | Second OPEN refuses renegotiation and retires only its owner. |
| AC-7 | Done | Strict failure tests and Session bounded-wait root | BFD hold expiry and configuration change retain their existing outcomes. |
| AC-7h | Done | `TestSessionBFDStrictDelayOpenBoundedWait` | Original negotiated HoldTimer bounds the pending state. |
| AC-8 | Done | `TestEnterDelayOpenBfdUpPendingRefusesInvalidEntry` | Invalid state entry is refused. |
| AC-9 | Done | Draft notes and FSM guides describe all four sub-states | Strict-BFD draft has no final native finding; update/tree passed. No invented RFC2119 row or whole-RFC support claim. |
| AC-10 | Done | Four coded diagnostic roots | Both OPEN rails, failed notification, strict expiry and valid controls. |
| AC-11 | Done | Preparation and all incoming reload/selected-validation roots | Exact immutable offer and post-callback freshness; no callback under reload caller locks. |
| AC-12 | Done | `TestLowLiveRouterReleasesValidationAfterCleanup` | Restore the absent provider only after all fixture owners join. |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| DelayOpen FSM events, wire timing, config bounds and buffered OPEN | Done | `fsm/delay_open_test.go`, `reactor/delay_open_test.go`, `delay_open_collision_test.go` | All named rows in the unit inventory above retain their original questions and controls. |
| Active retry and automatic-start plan rows | Done | Real-Peer retry and automatic-start files above | Current positive behavior and failed-cycle controls are retained; all corresponding native records and published-source retakes are accepted. |
| Strict-BFD FSM, Session and Peer rows | Done | Both `bfd_delay_open_test.go` files and immediate-reply race file | Hold-zero/nonzero, all releases, failures and invalid-entry coverage. |
| Incoming listener, identity, capture, removal, terminal and panic rows | Done | Named incoming/candidate/capture files above | Typed ownership supersedes the original pending-resolver design; no planned behavior dropped. |
| OPEN preparation, reload, diagnostics, startup and fixture cleanup rows | Done | Named preparation/reload/startup/diagnostic files above | Controlled internal seams retain actual TCP observations and useful controls. |
| Pool pressure and publication ownership rows | Done | Named read-pool/cache/abort/follow/lifecycle files above | Restored real-slot ownership and retained-borrow assertions; later continuation correction included. |
| Final decoded OPEN and pre-established UPDATE regressions | Done | Decoded/lifetime/pre-established files | Intended REDs, restored race proof and actual consumer daemon observations. |
| Eight functional-table rows, including both stamp companion carriers | Done | `test/plugin/` paths in Functional Tests | Final unique live selection passed13 incoming/role/stamp controls, zero skips. Base and strict DelayOpen retain their separate qualified Linux evidence; every qualified draft had eighty actual invocations. |
| Pinned FRR DelayOpen scenario and capture checker | Done | `test/interop/scenarios/bgp-delayopen-frr`, `check_delayopen.go` and its tests | Existing retained clean/broken/restored guest evidence; no new external interop run is inferred. |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| FSM state, event and timer source/tests | Done | Existing DelayOpen files carry the original event work; no extra literal FSM-object requirement. |
| Session/Peer connection, startup, retry and strict-BFD source/tests | Done | Original named files and their extracted owners are present. |
| Pending incoming collision implementation | Changed | `incoming_candidate.go`, typed admission and immutable proposal/capture owners replace the old resolver; behavior is covered by actual listener tests. |
| YANG/config/settings and configuration guides | Done | Existing registered grouping reaches peer/global inheritance and runtime parsing. |
| Two original daemon carriers and FRR scenario/checker | Done | All paths present; retained functional and interop evidence above. |
| Incoming role/UPDATE/learned-OPEN fixture and live carriers | Done | New retained fixtures plus promoted internal/forked carriers are in the candidate population. |
| Pool/ownership/passive-start test files | Done | All planned creation paths exist and have bounded evidence above. |
| RFC rows, record files and draft notes | Done | All41 runtime identities, five final-source retakes and16 audit judgments accepted; exact selected-source freshness readback complete, including the native HEAD RFC4724 confirmation. |
| Documentation, fragment, owned journal rows and live citers | Done | Selected-page patches omit unselected producer claims; documentation/source review, final update/tree/citation and global check attribution are complete. |

### Audit Summary
- **Total items:** 9 task groups,34 AC rows,36 current unit-plan rows,8 functional rows,1 interop scenario and9 source-file groups.
- **Done:** All9 task groups,34 AC rows,36 current unit-plan rows,8 functional rows,1 interop scenario and9 source-file groups are complete on the selected scope, with bounded implementation/runtime evidence.
- **Partial:** 0; no scope reduction requested or taken.
- **Skipped:** 0.
- **Changed:** The pending resolver became a typed same-Session incoming owner, as recorded in Deviations.
- **Open:** The main thread's two closure commits; native review artifact/check are complete. Runtime records/audits and selected-source readback are complete; global check findings remain recorded.

## Goal Validation (BLOCKING)

<!-- Maps each goal from the Task section to proof it was achieved. "Tests pass"
     is not evidence for a goal; a named test with its output is.
     See ai/rules/interop-and-goal-validation.md for the required evidence per
     goal type, and for the vacuity traps: a test that would still pass with the
     behavior reverted proves nothing. -->
| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Delay the local OPEN without stranding useful sessions | Functional and interop | `bgp-delay-open`, strict companion and pinned FRR10.7.1 guest capture qualification distinguish an early OPEN from the permitted exchange; timer expiry and remote-first cases both establish. |
| Reset retry history only after an Established cycle | Actual Peer and native discrimination | Active/passive automatic-start and opposite failed-cycle roots exercise actual reconnects; current four automatic-start records accepted, with failed branches observed under native breaks. |
| Preserve incoming connection ownership through selection and cleanup | Actual TCP and controlled lifecycle | Listener, typed-adoption, collision/refusal, capture-removal and BFD/terminal/panic controls retain the original socket/Session and useful incumbent. |
| Keep provisional OPENs from changing selected learned state | Functional and negative discrimination | Actual internal/forked RS losing IPv6-only OPENs retain selected IPv4 useful export; the wrapper bypass fails that export after true collision. Role carriers separately prove policy on both transports. |
| Keep pre-established UPDATEs out of useful route state | Functional and negative discrimination | Pre-OPEN withdrawal daemon retains actual Adj-RIB-In and original-socket output; old predicate fails its FIFO assertion. The original Established source still removes its own route. |
| Preserve bounded pooled ownership under pressure and callback failure | Allocation and fault injection | Zero-wrapper frame test, actual depleted-pool TCP continuation, abort/retained/follow-panic controls and exact borrowed-body alias RED followed by120 restored race passes. |
| Preserve useful passive startup and complete public event ordering | Actual listener and IPC | Startup gate/control retains one TCP/OPEN exchange; strengthened observer detects the missing OpenSent and restored startup controls pass300 repetitions. |
| Maintain diagnostics, reload and test-fixture lifecycle | Controlled lifecycle and affected controls | Coded refusal/hold roots, constructor/handoff/selected-RPC reload controls, joined provider cleanup and18 affected positive RIB controls passed without race. |

## Work Not Done

<!-- Every in-scope item this spec did not do. Each one is a spec of its own by
     now, in the bucket that item belongs to (plan/README.md), and this table
     names it. There is no shard and no deferral row: a row nobody can count is a
     row nobody schedules, which is why plan/deferrals/ was deleted on 2026-09-05
     holding 103 live rows, 29 of which named no destination at all.
     Scope reduction is the owner's decision, never the author's
     (ai/rules/completion.md). -->
| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| No in-scope behavior was dropped or transferred | All planned behavior and selected-scope verification are complete; the two native closure commits remain the main thread's final publication action | No destination spec or deferral |

## Review Gate

<!-- BLOCKING (ai/rules/planning.md). The review is INDEPENDENT: reviewer
     subagents or a fresh session over the actual diff, never your own inline
     reasoning about code you just wrote.

     Record the machine-checked artifact with:
     ./le spec review record spec <spec> verdict CLEAN rounds <N> file <path> ...
     Then check the reviewed population with:
     ./le spec review check spec <spec> file <path> ...
     rounds is the pass count and is required; more than five needs
     rounds-reason naming the PRODUCT defect a later round found, AND
     owner-authorised carrying Thomas's word, because more than five passes
     is his decision (owner ruling 2026-08-17). At the cap you stop and ask him;
     you never set that keyword on your own initiative. A false statement in
     this record is a NOTE, never a reason for another round (ai/rules/planning.md).
     ./le commit create checks the closure commit for a fresh, hash-pinned,
     CLEAN artifact. Closure is the spec removal in commit B; a journal row
     never makes commit A a closure. This table preserves the findings and
     fixes in the committed spec; it does not replace the artifact or require
     a separate learned summary. -->

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/bgp-fsm-delayopen-and-automatic-start-01a1e0ac-0f28-7471-8c00-742a752c8144.md`; SHA256 `ca34f1da59d1b41d79bd8aa42261a4f720fed8be55b0de3a92d8c1d4ed0811c9`; native clean, rounds2,154 exact selections:35 whole code files, three patches and116 HEAD blobs/modes. |
| `./le spec review check` | Exit0:35 selected whole code files clean and hashes match the artifact. The native commit gate checks the full prospective hunk/HEAD selection against the same artifact. |
| Rounds | 2; final selected-scope review found0 BLOCKER/0 ISSUE; prose corrections add no round |
| Reviewer lenses used | logic and wiring; security and edge cases; lifetime, allocation and validity/style |

### Round 1 scope and open finding

The review covers five exact Spec-work commits (216 touched paths, 104 source
code paths) plus all current corrections and daemon carriers. The two approved
partial-file selections are reviewed as HEAD plus their exact patches; excluded
dirty listener producers are reviewed at their committed blobs. No CLEAN
artifact exists while the final source correction and its evidence are owed.

BLOCKER: `incoming_candidate.go::readOpen` dispatches a pre-OPEN UPDATE while
DelayOpen holds Active. `session_handlers.go::updateIsUnexpected` omits
Connect/Active, so `session_read.go::processMessage` reaches route validation
and `deliverReceivedMessage` before Event27 performs the correct rejection.
The factory callback resolves the incumbent Peer and can publish through its
delivery queue. A valid withdrawal requires no OPEN-derived AS attributes.
Post-OPEN strict DelayOpen waits have the same gap. AC-1t owns the shared
admission repair, actual-listener and real-daemon proof, and useful controls.

The private Established path was separately traced: after KEEPALIVE the private
reader resolves or waits for progress instead of reading another frame. Transfer
returns out of that reader; selection joins it and installs delivery/down and
selected publication before Session.Run resumes. No additional product gap was
found on that boundary.

BLOCKER: decoded private OPEN events bypass the selected learned-state cut.
`incoming_candidate.go::admitFrame` publishes the remote OPEN before semantic
admission. The Peer-keyed event reaches `rs/server_handlers.go::handleOpen` and
`rr/rr.go::handleStructuredOpen`, which replace an existing Up peer's learned
ASN/capabilities/families. RS forwarding uses that family set. GR's received
OPEN handlers replace or clear the maps its selected Down handler reads for
retention; its sent OPEN observer also replaces the local LLGR exchange set.
The provisional RPC protects role validation, but does not distinguish these
decoded message events. AC-1o owns this remaining learned-publication gap.
The source fix must preserve actual wire observation, ordinary selected OPENs,
both transports and the selected exact-pair/event/counter contracts. Runtime
proof and correction remain owed; no CLEAN artifact is recorded.

### Findings fixed
<!-- Only BLOCKER and ISSUE. NOTEs do not block: record them and proceed.
     Every fix is new code that needs a fresh pass, so re-run until clean. -->
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | BLOCKER | A pre-established UPDATE reached consumers before the FSM rejected it | `session_handlers.go::updateIsUnexpected` and both read rails | Shared Established-only admission. Seven intended failing leaves, 240 related race passes and accepted real-daemon withdrawal discrimination; the bounded RFC positive record is written. Final unique13 live controls passed; all owned native proof currentness is verified on the selected snapshot. |
| 2 | BLOCKER | A private decoded OPEN could replace the incumbent's learned state | `session_open_publication.go::privateNegotiationEpoch.messageCallback`, `Session.selectNegotiationEpoch` | Bounded owned observations and selected replay. Internal and forked daemon discriminators accepted; deliberate borrowed-body alias failed, followed by 120 canonical race passes across six roots. Final unique13 live controls passed; selected-source ledger checks are complete. |

### Round 2 scope

This pass covers the two Round 1 corrections and their sibling paths: shared
pre-established UPDATE admission and unchanged ROUTE-REFRESH handling; private
decoded OPEN retention, selected replay and its factory/validation/adoption
wiring; RS, RR and GR received/sent OPEN consumers; the affected fixture setup,
daemon carriers, counters, capture and documentation. Logic and wiring,
security and edge cases, and lifetime, allocation and validity/style are all
included. Round 1's complete historical scope and findings remain recorded
above. No CLEAN verdict is recorded before the final scoped runs, daemon
qualification and RFC records are verified.

Companion review covers the Established refresh-handler fixtures, the shared
fresh-Session UPDATE/panic prerequisite and the borrowed OPEN-body ownership
unit. The existing notification, teardown and panic assertions remain intact;
the two handler tags now claim only their observed no-error result. The RIB
fixture proposal supplies independently resolvable next hops through the real
Loc-RIB and preserves the ordinary reconnect settlement wait. Only the test
that deliberately holds replay waits for full settlement after it releases
that hold. The reviewed proposal is applied with all three exact source
hashes checked. Its formatter and affected controls completed, including18 affected RIB
positive controls,13 unique live carriers and all41 native proof identities.
The earlier advertiser compilation hole was repaired before those observations.
Final owned RFC-check readback is complete:425 global findings, no selected-source owned defect, and exact HEAD RFC4724 confirmation fresh/0shifted/0refused.
These are continuation checks within Round 2, not another review round.

Current prospective scope reconciles all six Spec-work commits (221 touched
paths) with152 code inputs:35 whole files, one code patch and116 explicit
HEAD blob/mode baselines. Two additional documentation patches bring the
commit to82 whole files and three patches. At HEAD
`f1d64ff442a1125557c45517c4dfc3a11469bc75`, every baseline object/mode matches
and all three patches match their HEAD contexts. The complete source review
retains the style/security pass, including peer-reachable panic paths,
bounded OPEN-body ownership and callback/lock ordering. No new product
BLOCKER or ISSUE was found in the SourceSession foundation interaction:
`sourceMessageCallback` stamps UPDATEs only, OPEN selection retains its private
epoch, and `Peer.runOnce` captures source authority before UP and drains
selected delivery before closing it. Native artifact binding below records
the final selected-scope verdict; the425 global RFC findings and other global
check failures remain disclosed.

### Final Round 2 result

0 BLOCKER,0 ISSUE on the exact selected scope. Logic, wiring, generic security,
edge cases, lifetime, allocation and the complete changed-Go style pass were
included; no peer-reachable new panic or unbounded input ownership was found.
All34 AC rows and goal mappings are complete. Native record and freshness
check both exited0; global checkout checks remain red as disclosed above.
This is the second round, not an extra round for corrected bookkeeping.

## Pre-Commit Verification

<!-- BLOCKING. Do NOT trust the audit above: re-verify independently and paste
     the evidence. For each row get the evidence now: a reader (ls, grep) is the
     closing agent's own, and a test run (go test -run) is an owed command the
     main thread runs.

     EVERY sub-table needs at least one data row: pre_commit_verification_gaps
     in internal/le/commit/prepare.go checks them one by one and names the empty
     ones. A row in Files Exist is not evidence for AC Verified.
     Not acceptable: "already checked", "should work", a pointer to the audit. -->

### Files Exist (ls)
<!-- Every file in "Files to Create", and every .ci named in Wiring Test and
     Functional Tests. Paste the ls output. -->
| File | Exists | Evidence |
|------|--------|----------|
| Original DelayOpen and strict-BFD live carriers | Yes | Direct `ls -d` read found `test/plugin/bgp-delay-open.ci` and `test/plugin/bgp-delay-open-bfd-strict.ci`; both were read for listener/BFD wiring. |
| FRR scenario and native checker pair | Yes | Direct `ls -d` read found `test/interop/scenarios/bgp-delayopen-frr`, `internal/le/interoplab/bgp/check_delayopen.go` and `check_delayopen_test.go`. |
| Role fixture and internal/forked role-isolation carriers | Yes | `internal/test/fixture/role_incoming_open.go` is present and read; both promoted carrier bodies were read. |
| Pressure/abort/follow/lifecycle/passive-start creation paths | Yes | Direct `ls -d` read found all five named reactor test files, including both receive-publication files and the cache lifecycle file. |
| Added UPDATE and learned-OPEN daemon fixtures/carriers | Yes | Retained fixture sources were read; direct path enumeration found all three promoted live `.ci` files. Final unique live selection executed each. |

### AC Verified (grep/test)
<!-- Every AC-N, re-checked. Acceptable: test name + pass output, grep showing
     the call, ls showing the file. -->
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1, AC-1a, AC-1b, AC-1c, AC-1d, AC-1e, AC-1f, AC-1g | Configured delay and the receiving state's timer/wire/retry actions | YANG range/default and runtime parse guard read; current `connectionEstablished`, `delayOpenExpired` and timer-retirement producers read. Named FSM events, wire-delay, retry-budget and catch-all/control roots carry the retained scoped evidence in the unit inventory. Base/strict daemon and pinned FRR proofs retain their exact useful wire boundary. |
| AC-1h, AC-1i, AC-1j, AC-1k, AC-1l, AC-1m, AC-1n | Same-Session incoming, buffered compatibility, collision, metadata and capture ownership | Actual listener/default/delayed OPEN, typed-adoption/terminal/panic, both identifier orders, original connected metadata and capture removal controls are mapped above. Current `adoptIncoming`, `resolveIncoming`, `prepareSessionCapture` and membership-cut code read; exact capacity6/5 and collision6/7 remain distinct. |
| AC-1p, AC-1q, AC-1r, AC-1s | Value frames, pressure, failed publication and startup ownership | Allocation and real depleted-pool assertions; abort/retained/follow-panic/lifecycle final200-pass race selection; complete public startup observer/control final300-pass selection. Their source-bound inventories above retain scope and original intended failures. |
| AC-2, AC-2p | Established-cycle automatic restart and opposite failed-cycle behavior | Actual active/passive Peer roots retain the counter and session observations. Four current automatic-start runtime records are accepted in the final queue; their exact current producer is `Peer.runOnce`. |
| AC-4, AC-5, AC-6, AC-7, AC-7h, AC-8 | Strict DelayOpen pending states, permitted releases and bounded refusals | `advanceAfterOpen` and BFD release ordering read; both FSM state families and real Session/Peer wire controls cover zero/nonzero hold, all releases, second OPEN, configuration close, original hold expiry and invalid entry. Strict daemon qualification remains the retained Linux entry-point proof. |
| AC-10, AC-11, AC-12 | Coded diagnostics, immutable/reconciled offers and joined fixture cleanup | Named four diagnostic roots and preparation/reload/handoff/selected-validation controls retain actual refusal/liveness observations. `reconcilePluginOffers` and refusal producer read; provider cleanup and affected18 positive RIB controls pass without race. |
| AC-3, AC-9 | Complete current RFC records/audits and final draft notes check | Both row13/20 polarities and all41 native identities, five published-runOnce retakes and16 independent audit rows/8stems are accepted. Strong replacement retarget failed only after the useful B-then-better-A baselines at the intended post-withdrawal oracle. Final RFC update/tree/citation passed. Full-WT RFC check exited1 with425 findings; exact owned readback found only RFC4724 surrounding-file SHIFTED from excluded check_rfc.go edits. Native source HEAD confirmation at f1d64ff442 is fresh/0shifted/0refused and the74ea9188 interop blob matches the explicit review pin. No selected whole/hunk lint finding; global lint/doc/repo/weakening reds remain disclosed. |
| AC-1t | Shared admission permits UPDATE publication only after Established | Seven intended failing leaves preceded the fix; twelve roots then passed twenty race repetitions each. Native daemon clean PASS 2.0 seconds/broken FAIL 2.0 seconds was accepted at the useful incumbent's withdrawal/FIFO fence. The new RFC4271-8.2.1-3 positive record passed both read modes and failed the old predicate at real callbacks/delivery with explicitly modeled route presence; it claims no actual-RIB observation. Final unique13 live controls passed; selected-source native RFC currentness is complete. |
| AC-1o | A losing decoded OPEN preserves selected learned families and useful export | Internal daemon clean PASS 2.7 seconds/broken FAIL 16.4 seconds; actual forked bgp-rs clean PASS 1.5 seconds/broken FAIL 16.5 seconds. Each break retained collision6/7 and EOF, then exposed IPv4 family mismatch and missing fresh original-socket output. Both carriers passed eighty loaded invocations and were promoted. Final unique13 live controls passed with zero skips. |
| AC-1o | Deferred decoded OPEN bodies survive reuse of both borrowed inputs | The native generic one-file alias overlay failed `TestPrivateOpenPublicationOwnsBorrowedBodies` at all-zero local bytes versus the exact original OPEN, with no compiler/setup failure. With overlay absent, all six canonical ownership/decoded/SDK roots ran twenty times: 120 RUN, 120 PASS, zero FAIL or DATA RACE, package 69.325 seconds. This is restored final GREEN after the intended alias RED. |
| Companion fixture correction | Refresh handlers reach their declared Established conditions; UPDATE-triggered panic and policy callbacks remain reachable | Actual native `incoming-final-handler-and-owned-open` receipt: 13 top-level roots run, 13 pass, zero failures or DATA RACE; package PASS in 9.525 seconds. This includes all nine companion roots, peer panic isolation, both decoded OPEN roots and the borrowed-body unit. The later alias RED/restored race GREEN is recorded above; selected-source native RFC currentness is complete. |

### Wiring Verified (end-to-end)
<!-- Every Wiring Test row: does the .ci exist AND exercise the claimed path?
     Read the file; do not infer it from its name. -->
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| Registered DelayOpen configuration reaches actual daemon TCP/FSM | `test/plugin/bgp-delay-open.ci` | Read carrier and fixture wiring; retained qualified live observer proves held OPEN and eventual establishment. |
| DelayOpen plus strict BFD reaches actual pending state and release | `test/plugin/bgp-delay-open-bfd-strict.ci` | Read carrier; real BFD datagram and useful handshake are required, not a synthetic state-only result. |
| Established-session failure starts the next actual Peer cycle | Actual-Peer automatic-start roots | Event3 is internal counter behavior, so the spec's named controlled-Go entry exercises real session teardown/reconnect instead of claiming an unrelated `.ci` result. |
| Competing OPEN/UPDATE cannot change useful selected behavior | Three new incoming carriers and internal/forked role carriers | Final unique live selection passed13/13, zero skips,7.6 seconds; each new carrier has its own accepted native clean/broken discriminator. |

### Assumptions Resolved
<!-- Every A-N. `unvalidated` is not a valid final status. A broken assumption
     needs a Mistake Log row and a Deviations entry. -->
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | Pinned FRR10.7.1 guest clean/broken/restored qualification loaded both DelayOpen configurations and retained complete captures; the eager-OPEN break was observed. This is retained interop evidence, not a new external run. |

### Documentation Verified
<!-- Every Yes in the Documentation checklist: verify the edited claim against
     source. Every No: paste the grep that proves no update was needed. -->
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Feature declaration and real-path inventory | `features/bgp-protocol.md` names promoted carriers and the pinned scenario; overall level remains experimental and scope partial | Source/fixture paths and declared scope read; documentation check completed with separately disclosed global findings. |
| Configuration, examples and user guide | `docs/config-reference.md`, `docs/guide/configuration.md`, `docs/guide/bgp-peering.md` agree with registered `delay-open-timer` and runtime parse bounds | Direct range/default/example text read; no new CLI grammar or standalone RPC command. |
| FSM and lifecycle architecture | FSM Connect/Active pages and `peer-lifecycle.md` name actual delay, automatic-start, BFD, incoming, reload and startup producers | Producer and current claim reads; selected snapshot retains the qualified contract. |
| API, plugin and SDK OPEN contract | Published process-protocol prose plus the conditional SDK page and selected role prose describe provisional checks and selected exact-pair/decoded publication | Generic internal/JSON boundary read; no claim that a private observation is selected learned state. |
| Wire format, metadata, metrics and inventory categories that need no new surface | Actual OPEN-body tests preserve bytes; onOpenSent/onOpenRecv and capture stay at physical read/write sites; no new route metadata key, counter, plugin registration or event kind in this fix | Source/callback census and exact-body/counter controls; semantic publication timing is documented separately. |
| Buffer and test-infrastructure claims | Published buffer prose describes value frames, bounded OPEN copies and retained-borrow ownership; controlled lifecycle seams justify their internal fault hooks | Current selected producers and real-pool tests read. Unselected advertiser/PPP/raw-originated/restart claims are omitted only from the prospective pages. |
| RFC status and comparison | Original rows13/20 and draft notes reflect implemented behavior; comparison has no DelayOpen entry to change | Row/tag census read; RFC update/tree passed with no generated diff and16 native audit rows are stamped. All five foundation producer records are renewed. Final full-WT check has425 global findings; exact owned readback is clear on selected source. Native RFC4724 HEAD confirmation is fresh/0shifted/0refused; only excluded surrounding WT source produces its SHIFTED report. |
| Runtime dependencies and doctor coverage | The correction reuses TCP, capture and pool owners; registered refusal/hold diagnostic codes are exercised by named roots | No new binary/socket/path/kernel/certificate dependency introduced; no extra doctor probe required. |
| Final native documentation gate | `./le doc check all` | Ran exit1: wiki catalog drift,3 YANG mappings,3 command summary rules and3 source anchors remain recorded. No global clean result is claimed. |

## Core Insight
<!-- Optional: the single most important design revelation from this work.
     Not every spec has one. Delete the section if nothing qualifies.
     Route it to the governing surface under ai/rules/planning.md. -->
