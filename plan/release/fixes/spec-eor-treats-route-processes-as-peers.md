# Spec: eor-treats-route-processes-as-peers

| Field | Value |
|-------|-------|
| Status | in-progress |
| Priority | P1 |
| Release reason | Required correctness of shipped configuration, routing or forwarding behavior; existing partial implementations and remaining proof are retained. |
| Scope | bgp |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-14 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Hand-back record (both items cleared 2026-11-14)

Cleared, verified by the second closure agent against the commits:

| # | How it was cleared | Evidence |
|---|--------------------|----------|
| 1 | Commit 9609d00703 adds the scenario `test/interop/scenarios/bgp-manual-eor-requested-frr/` and its checker (`checkManualEORRequested`, `internal/le/interoplab/bgp/check_manual_eor.go`; predicates in `check_manual_eor_predicate.go`). The process acts from inside its peer-UP handler, which `deliverPeerState` awaits before the sync starts. Session 1 asks for nothing: FRR must log the route and no End-of-RIB through a 10 s hold. After `clear bgp`, session 2 announces a route then asks for the marker: FRR must log the route, then exactly one End-of-RIB. FRR's behaviour was read at `frr-10.7.1` (the lab's `FRRImage`): `bgp_update_receive_eor` (`bgpd/bgp_packet.c`) logs every marker before it tests `PEER_STATUS_EOR_RECEIVED`, and `bgp_update` logs each decoded prefix under `debug bgp updates in`. The design reads FRR's log instead of negotiating Graceful Restart, because `gracefulRestartInfo.endOfRibRecv` is one flag and can show neither a count nor an order (`frr.conf` header) | Main thread, Docker lab, Ze image rebuilt for each run: GREEN on HEAD (1 passed, `sha256:4da4a47314f2...`); RED with `if once {` made unconditional in `announceEOR` (assertion 7, `sha256:883e8bd4bf3b...`); RED with `sendEOR := true` in `sendInitialRoutes` (assertion 3, `sha256:fe2dd16c5cfc...`); both breaks restored. Not covered by the scenario: a replay's marker on a `manual-eor` peer, which `TestManualEORPeerWithholdsAReplayEndOfRIBAfterTheSync` covers |
| 2 | Commit a64cf431a1 rewrites the four strings (`rib_replay_test.go`, `api_sync_test.go`), text only, judged by an independent test judge | The second closure re-ran the search for `waitForAPISync`, `apiSyncTimeout`, `apiSyncReady`, "holds its End-of-RIB" and "End-of-RIB waits for" over `docs/`, `internal/`, `pkg/` and `test/`. It found one more stale sentence, `docs/architecture/api/architecture.md` step 6 ("The last expected report closes `apiSyncReady`"), corrected in this closure. The other hits are true: `docs/plugin-development/protocol.md` says Ze never holds its marker for a report, and `peer_initial_sync_test.go` names the deleted channel as history |

The stress pass over the sixteen plugin tests ran 80/80 clean, and `./le rfc
check` was clean for `rfc4724` (main thread, 2026-11-14, state file).

The original hand-back follows, unchanged.

Set by the closure agent on 2026-11-14. The closure's review read the eight
commits of this work (a2302b8968, 2cd7ac9f09, 0461489d41, 4aa1f84ff6,
036e5184e9, 49b01f67ac, 923628582e, 8d8a506457) as an independent reader and
found one BLOCKER and one ISSUE. The spec goes back to `/ze-implement`; the
main thread sets it to `in-progress` when both are cleared, and the closure
then runs again.

| # | Severity | What blocks | Producer | What clears it |
|---|----------|-------------|----------|----------------|
| 1 | BLOCKER | No interop scenario proves D-1 and D-2 against another implementation. Both change which End-of-RIB markers reach a peer: a marker a process asks for during the initial sync now reaches the wire where it was dropped, and a `manual-eor` peer no longer receives a replay's marker. `ai/rules/interop-and-goal-validation.md` exempts only a change with no wire-visible effect, a config-only feature or tooling, and `ai/doors/bgp-engine.md`, "Gates", says any wire-visible change owes `./le test integration interop`. No scenario under `test/interop/scenarios/` sets `manual-eor` or asks for a marker (searched 2026-11-14), and every functional test of this work runs against Ze's own test peer | `reactorAPIAdapter.announceEOR` (`internal/component/bgp/reactor/reactor_api_forward.go`); the `sendEORFamilies(sendEOR, ...)` call in `Peer.sendInitialRoutes` (`peer_initial_sync.go`) | A named scenario (for instance `bgp-manual-eor-requested-frr`): Ze peers with FRR under `behavior { manual-eor true }`, with graceful restart negotiated so FRR records the marker; a process, or an ExaBGP script through the bridge as `bgp-exabgp-bridge-vpnv4-frr` does, announces a route and asks for the marker. The check asserts FRR holds the route and has recorded Ze's End-of-RIB for the family only after the request, and none before it. Before the check is written, read FRR's source at the lab's version for what it records and prints on a received End-of-RIB (`bgp_update_receive_eor`, and the vty or JSON field that shows it) and name the function (`ai/rules/interop-and-goal-validation.md`). Red walk: rebuild the Ze image with the old suppression branch restored, see red, restore, see green, and record it in this spec's Goal Validation |
| 2 | ISSUE | AC-8 is not met: three test comments and one failure message still say the End-of-RIB waits for the session-ready report | `internal/component/bgp/plugins/rib/rib_replay_test.go`: the doc comment of `TestPeerUpEmptyRibOutSignalsReady` ("The reactor arms waitForAPISync(2s) ... and sends its EOR only after that wait returns"), the message of its first `require.Equal` ("the reactor waits the full waitForAPISync(2s) and the EOR is 2.5s late"), the PREVENTS line of `TestPeerUpEmptyRibOutSignalsReadyOnceOnly` ("inflating apiSyncCount ... a multi-plugin peer's wait"); `internal/component/bgp/reactor/api_sync_test.go`, the section comment above `warnRecorder` ("waitForAPISync ... is released") | Rewrite each to say what the report does now: it is credited, and a fence owner's report lowers the replay fence (`Peer.creditAPIReady`, `Peer.lowerReplayFence`). The VALIDATES and PREVENTS lines 4aa1f84ff6 already rewrote in the first test are the model. Text only, no assertion changes; the main thread runs the two packages' tests after |

Judged and not blocking:

| Subject | Verdict | Evidence |
|---------|---------|----------|
| Interop for D-3 | Not owed | No peer can make the sync panic, so no other implementation can reach the path; its wire output is a Cease with subcode 0, the NOTIFICATION every existing scenario's teardown already exchanges. The proof is the unwind tests' wire assertions (`peer_initial_sync_unwind_test.go`) |
| `Peer.endSessionAfterSyncPanic` checks `syncSessionGoneLocked` under the read lock, then tears down unlocked | NOTE | Same check-then-act shape as `runQueuedTeardown`, its declared model; the session it names is the one the recovering goroutine owns |
| `Peer.syncOwed` reads the forward queue counts | NOTE | `request quiesce` now waits while a peer's forward queue holds items, so a quiesce under sustained forwarding can time out; its error names the peer and the reason (`peerSyncOwedError`) |
| A panic in the route-server fast path's write | NOTE | `forward_rs_direct.go` releases the write path with a plain unlock, so `discardInterruptedWrite` does not run there; owned by the open row 2026-11-14 in `plan/journal/silent-fall-through.md` (owner `spec-bgp-one-off-defects`) |

Still owed by the main thread before the closure runs again: the stress pass
over the sixteen plugin tests, which could not build at its last attempt, and
`./le rfc check` for `rfc4724`.

## Task

**A process that creates routes is a PEER as far as the End-of-RIB marker is
concerned, and ze does not wait for a peer's marker before sending its own
(owner ruling, 2026-09-18).**

Ze sends its initial table to a new session and then the End-of-RIB marker.
`sendInitialRoutes` (`internal/component/bgp/reactor/peer_initial_sync.go`)
already omits the wait for external route-pushing processes. It drains and
closes its queue gate, calls `wakeForwardOverflow`, then takes the write hold
and sends the eligible family markers. The earlier implementation waited for
process readiness or the API-sync timeout after draining, under the
2026-08-30 ruling that placed plugin-injected routes in the initial update.

This ruling supersedes that one for the marker's TIMING. A route-creating
process stands where a peer stands: ze never holds its own marker waiting for a
peer's, so it does not hold it waiting for a process either. The routes such a
process pushes are ordinary updates when they arrive, exactly as a peer's are.

The no-wait change removes that timeout window, but it does not by itself prove
the forwarding order required by AC-2. The current producer still reopens the
forwarding rail before taking the marker's write hold, and `forwardOrderHold`
reads the initial-sync flag rather than `initialSyncEOROwed`. Research must
establish whether a forwarded UPDATE can overtake the marker through either
rail and preserve AC-2 when correcting that order.

**What it also settles.** Four functional tests assert routes-before-marker and
lose that race under load: `redistribute-export-reject`,
`ipv4-announce-withdraw`, `ipv6-announce-withdraw` and
`show-bgp-bare-runs-summary` (`plan/journal/false-synchronization-claim.md`,
2026-09-18). Their recorded failures motivated the ruling. Each consumer still
needs assessment against the current producer; an ordering the product no
longer promises must not remain in a test, and no assertion may be discarded
without accounting for the behaviour it was intended to prove.

**What this spec must NOT assume.** The applied no-wait change is only part of
the scope. `initialSyncEOROwed` still gates EOR suppression on the API rail
(`reactor_api_forward.go`). The remaining consumers of `apiSyncExpected` and
`waitForAPISync` need assessment, and `forwardOrderHold` narrows `shouldQueue` twice for
reasons its own comment calls load-bearing. Each has to be read before anything
is removed, and the 2026-08-30 ruling has to be re-read in full: it may still
govern WHICH routes belong to the initial update even where it no longer governs
WHEN the marker goes out.

### Research corrections to this Task (2026-11-14)

The Task above is kept as written. Research found four of its statements wrong
or stale, and the design starts from the corrected facts.

| Task statement | What research found | Evidence |
|----------------|---------------------|----------|
| Four tests "assert routes-before-marker" | None does today. `ipv4-announce-withdraw` and `ipv6-announce-withdraw` lost their marker assertion in commit 9c93f82bb4 and their headers say so. `show-bgp-bare-runs-summary` carries no route. `redistribute-export-reject` asserts the OPPOSITE order: the marker at `seq=1`, then a route forwarded from another peer at `seq=2` | the four files under `test/plugin/` |
| Their failures are in `plan/journal/false-synchronization-claim.md`, 2026-09-18 | The one row of that date is about `observeConfigured` and `DrainPeerSync` and names none of the four. `ipv6-announce-withdraw` is in the 2026-09-17 row; `ipv4-announce-withdraw` and `redistribute-export-reject` are reported in rows dated 2026-09-19 and 2026-09-24 (agent report, the last two not re-read) | the journal file |
| `initialSyncEOROwed` gates EOR suppression in `reactor_api_forward.go` beside `shouldQueue`, and the forwarding rail reads it | `announceEOR` reads it through `Peer.initialSyncMarkerOwed`. The forwarding rails read only `Peer.forwardOrderHold`, which does not read it | `reactor_api_forward.go`, `peer.go` |
| "The remaining consumers of `apiSyncExpected` and `waitForAPISync` need assessment" | `waitForAPISync` was removed by 9c93f82bb4. `apiSyncExpected` is live: it is the population the replay fence is drawn from. Only the readiness channel beside it has no product reader | Existing Mechanisms |

### Owner decision at the research gate (2026-11-14)

Two questions were put to Thomas: which order Ze promises between its marker
and a route forwarded from another peer, and whether the 2026-09-18 ruling
covers the in-tree replay (the `rib`, `rs` and `rr` plugins).

| Question | His words | What it decides |
|----------|-----------|-----------------|
| Marker against a forwarded route | "end of rib is a trick to not to have to recalculate the RIB/FIB with every update when the router starts, it has no bearing on what we forward" | Ze promises no order between its marker and a route that is not of its own table. Forwarding is never held for the marker. AC-2 as first written is withdrawn |
| Does the ruling cover the replay | "That rule is right but should be considered in view of what i replied in the the previous question" | The ruling stands and covers the replay: the marker waits for nobody. Main thread's reading, restated to him in the same turn |

The decision is written into `docs/architecture/behavior/peer-lifecycle.md`,
"What the End-of-RIB marker orders, and what it does not". It describes the
code as it already is, so this spec changes no ordering in the product. What
remains is: the tests that assert an order the product does not promise, the
API End-of-RIB that is dropped while answered as sent, the readiness channel
with no reader, the stale pages and comments, and the two owned journal rows.

## Required Reading

### Architecture Docs
- [ ] The initial-sync and forwarding rails, page named by `ai/CODE-TO-DOCS.md` for `peer_initial_sync.go`
  → Decision: the pages are `docs/architecture/behavior/peer-lifecycle.md` and `docs/architecture/bgp/structural-forwarding.md`. Neither states an order between a forwarded UPDATE and the initial-sync marker, so whichever order the design chooses is written into `peer-lifecycle.md` in the same work.
  → Constraint: `Peer.forwardOrderHold` runs under the forward worker's overflow lock, so it reads atomics only and never takes `p.mu`; a session fact it needs is read through `Peer.establishedSession`.
  → Constraint: one predicate serves both the dispatch gate and the overflow release (`takeOverflowReleased`, `forward_pool.go`). The release is decided per item by that predicate; `wakeForwardOverflow` only wakes an idle worker. Any store that makes the predicate false owes a wake after it.
  → Constraint: once the queue gate is open, `QueueAnnounce` and `QueueWithdraw` refuse with `errQueueGateOpen` and the caller writes directly, so a process's own route sent after the gate opens is not parked by any fence.
  → Constraint: the marker's write hold cannot be widened to cover the queue drain: the drain sends through the same non-reentrant session write lock, and that lock also gates KEEPALIVE.
  → Constraint: a replay item is refused once neither `initialSyncEOROwed` nor `initialUpdateOwed` is set (`initialReplayToken`, `session_ownership.go`); a design that changes when either clears keeps that refusal correct.
- [ ] `rfc/full/rfc4724.txt` Section 2 - what the marker claims about everything before it
  → Constraint: Section 4: "The End-of-RIB marker MUST be sent by a BGP speaker to its peer once it completes the initial routing update (including the case when there is no update to send) for an address family after the BGP session is established." This is the only timing rule. The RFC uses "initial routing update" seven times and never defines it, and no sentence says what may or may not precede the marker.
  → Decision: the RFC is silent on a forwarded UPDATE preceding the marker, so interoperability with FRR and BIRD decides (`ai/META-DECISIONS.md`).
- [ ] FRR 10.7.1, `bgpd/bgp_packet.c`, `bgp_generate_updgrp_packets`, and `bgpd/bgp_updgrp.h`, `advertise_list_is_empty`
  → Decision: FRR sends its marker only when the peer's outbound announce and withdraw queues are both empty and the coalesce timer is idle. It keeps no separate initial-table queue, so a route learned from another peer while the first send is in progress goes out BEFORE the marker.
- [ ] BIRD 3.4.0, `proto/bgp/packets.c`, `bgp_fire_tx`, and `proto/bgp/bgp.c`, `bgp_export_fed`
  → Decision: BIRD builds its end mark only when no update is left to build after the feed reports done. A route that changes during the feed is written BEFORE the marker.
- [ ] `docs/architecture/forward-congestion-pool.md`, "The replay fence" - the design page of `forward_eor.go`
  → Decision: the page already states that an End-of-RIB "claims the family and queues the marker at the tail of the peer's forward queue (`queueEndOfRIB`), always". The suppression branch in `announceEOR` sits in front of that and is not on the page. D-1 makes the code match the page for the explicit form, and the page gains the replay form's stand-down.
- [ ] `docs/architecture/api/architecture.md` - design page of the reactor files; searched for the marker and the session-ready report
  → Constraint: near its send-permission text it still speaks of "the barrier that holds a peer's End-of-RIB" when explaining why a raw-capable binding must be countable. The counting stands (it is the reporter population); the sentence is corrected in phase 4 to name the replay fence.
- [ ] `docs/architecture/bgp/replay-cursor.md` - design page of the replay files
  → Constraint: a replay item needs its `initial-replay` token "while the initial EOR is owed or the peer-up replay fence remains open". D-1 and D-3 change neither condition; D-3 ends the session, which retires the token with it.
- [ ] `docs/architecture/plugin/rib-storage-design.md` - design page of the RIB plugin files
  → Decision: it already describes the replay as able to complete "after Ze's own EOR", which is the order the owner decision confirms. No change owed beyond the comment in `rib_replay.go`.
- [ ] The two owner rulings
  → Decision: 2026-08-30 (recorded in the deleted `plan/spec-fixit-peer-pending-sync-settles-too-early.md`, readable at commit `1a0463d7dc^`): "A plugin-injected route DOES belong to this speaker's initial routing update. The marker waits for it", and the send vocabulary gains `raw`.
  → Decision: 2026-09-18 (verbatim in the `RFC-approved:` trailers of commit 9c93f82bb4): "we should treat process creating routes like peers when looking at the EOR marker. We will not wait for peers to have sent us EOR for sending ours." It removes "The marker waits for it" and nothing else: the `raw` send word, the split of the queue gate from the owed-marker flag, and the reporter population (now the replay fence) all stand.

**Key insights:** (minimal context to resume after compaction)
- The ruling is about WHEN the marker goes out, not about which routes belong to the initial update.
- A route-creating process is a peer for this purpose; ze waits for no peer's marker before sending its own.
- AC-1 is done and pinned (`TestInitialSyncMarkerWaitsForNoProcess`, commit 9c93f82bb4).
- AC-2 is not met and is contested: between the gate opening and the marker's write, every rail can write, and who goes first is a race for the session write lock. FRR and BIRD both let a forwarded route precede their marker; the RFC is silent. About 77 functional tests assert the marker BEFORE a forwarded route and are load-dependent on that race. No test goes red deterministically either way.
- The session-ready expectation set stays: it arms the replay fence. Only `Peer.apiSyncReady` is dead.
- `announceEOR` counts a marker as sent in the branch that drops it; on a `manual-eor` peer no producer then sends one.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/component/bgp/reactor/peer_initial_sync.go` - `sendInitialRoutes` drains the queue, calls `wakeForwardOverflow`, then takes the write hold and sends eligible markers without an external route-process wait; it retains `waitPeerUpBarrier` earlier in the send and clears `initialSyncEOROwed` at the end
- [ ] `internal/component/bgp/reactor/peer.go` - `forwardOrderHold` is `Established && sendingInitialRoutes != 0` and does not read `initialSyncEOROwed`
- [ ] `internal/component/bgp/reactor/reactor_api_forward.go` - the EOR suppression path DOES read `initialSyncEOROwed`, beside `shouldQueue`
  → Constraint: read 2026-11-14. In `announceEOR` the branch taken when `shouldQueue()` or `initialSyncMarkerOwed()` holds logs the suppression, counts the marker as sent and moves on, before any claim. The caller is answered success. With `manual-eor` set the initial sync withholds its own marker, so in that window neither producer sends one and the caller has no reason to retry. This is the owned journal row of 2026-11-14.
  → Constraint: the relay rail's dispatch loop in the same file parks an item only on `forwardOrderHold`; it does not read the owed-marker flag.
- [ ] `internal/component/bgp/reactor/peer.go`, `Peer.creditAPIReady` and `Peer.resetAPISync` - read by the research agent, wiring confirmed by the main thread
  → Constraint: the expected set is keyed by process name, each named process is credited once, and an unnamed report is refused. The defect in the owned journal row of 2026-08-30 (an anonymous counter any report satisfied) is no longer present; the row is still marked open and cites this spec at its old path under `plan/pre-release/`.
- [ ] `internal/component/bgp/plugins/rs/server_handlers.go`, `routeServer.sendEOR`
  → Constraint: the route server sends its replay marker and then reports session-ready, on its own schedule ("End-of-RIB is independent", its own comment). Nothing orders that report against the initial sync's marker, so the replay fence can be down before the sync has finished its own table.

**What reaches the wire between the gate opening and the marker (2026-11-14):**

| Writer | Can it write in that window? | What stops it, when something does |
|--------|------------------------------|------------------------------------|
| Relay rail, live forward | Yes | Only the replay fence, while a fence owner has not reported |
| Forward pool worker draining overflow | Yes: the predicate releases the item once the gate is open, then it competes for the write lock | Same |
| Route-server fast path | Yes when the fence is down | Hands back while the replay owns the update; parks while the fence or overflow is pending |
| A process's own announce | Yes: the queue refuses it and the caller writes directly | Nothing |
| A process's own withdraw | Yes unless forwards are pending for the peer | Joins the forward queue when they are |
| Another producer's End-of-RIB | No | `announceEOR` defers while the marker is owed, and the claim allows one per family |

**Behavior to preserve:** (unless the user explicitly said to change it)
- One End-of-RIB per family per session (RFC 4724 Section 2), and the claim protocol that enforces it
- A forwarded UPDATE never overtakes route operations still queued for that peer
- Corrected 2026-11-14: the first line's rule is Ze's claim protocol, not RFC text. RFC 4724 Section 2 has no sentence limiting the marker to one per family per session, and the rule covers the AUTOMATIC marker only (the sync's and a replay's). An explicit request is written each time, as it is today after the sync and as ExaBGP does.
- Ze's own table for the session precedes its End-of-RIB (RFC 4724 Section 4: "once it completes the initial routing update")
- The replay fence and the refusal of a replay item once neither the owed marker nor the fence is set

**Behavior to change:** (only what the user asked for)
- Preserve and prove the applied no-wait timing change; finish the forwarding-order and consumer obligations in AC-2 and AC-3 before considering closure

## Existing Mechanisms (MANDATORY)

Rows written at the start of research (2026-11-14) from a read of the three
producers named in Current Behavior. The "This spec" cells state the direction
research starts from; the design gate confirms or corrects each in place.

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| The forwarding order fence | `internal/component/bgp/reactor/peer.go`, `Peer.forwardOrderHold` | Answers whether a forwarded UPDATE for a destination is parked in that peer's forward overflow instead of written. True while the session's queueing gate is closed (`initialSyncInProgress`), and for a live change while a replay is owed (`initialUpdateOwed`) or a policy replay holds. It does not read the owed-marker flag, so it is false between the gate opening and the marker's write | extends: AC-2 is an ordering this fence exists to give, so the gap between gate and marker is closed in this predicate or by moving the release, never by a second fence beside it |
| The overflow release | `internal/component/bgp/reactor/peer.go`, `Peer.wakeForwardOverflow` | Wakes the destination's forward worker so it re-evaluates the fence and drains parked UPDATEs. `sendInitialRoutes` calls it after `drainAndCloseQueueGate` and before the marker's write hold | extends: where the release sits relative to the marker's write is the order AC-2 judges |
| The End-of-RIB owed flag and claim protocol | `internal/component/bgp/reactor/session.go`, `Session.initialSyncEOROwed`; `Peer.claimInitialSyncEOR` and `Peer.releaseInitialSyncEOR` | The flag says the session still owes its initial-sync marker: `pendingSync` reads it so `request quiesce` does not report a peer settled, and `initialSyncMarkerOwed` reads it so the API End-of-RIB command defers to the sync. The claim gives one marker per family per session across the two producers | extends: the one-marker guarantee stays on the claim, and the owned journal row on a `manual-eor` peer's dropped marker is a defect in what the flag's reader answers |
| The marker's write hold | `internal/component/bgp/reactor/peer_initial_sync.go`, the second `holdWrites` phase of `sendInitialRoutes` | Holds the session write lock across the family markers. The forward pool worker blocks on that lock and the route-server fast path gives up and falls back to the pool, so no forwarded write interleaves between two family markers | extends: it already orders the markers against each other; it starts after the release, which is the gap |
| The peer-up delivery barrier | `internal/component/bgp/reactor/peer.go`, `Peer.waitPeerUpBarrier` | Holds the sync, before the queue drain, until every plugin that registers this peer on the peer-up event has taken delivery, bounded by a timeout | does not apply: it waits for delivery of an event, not for a process's routes, so the 2026-09-18 ruling does not remove it; research confirms no route-creating process is waited for through it |
| The session-ready expectation set | `internal/component/bgp/reactor/peer.go`, `Peer.apiSyncExpected`, `Peer.resetAPISync`, `Peer.creditAPIReady`; filled from `Peer.initialUpdateReporters` (`peer_run.go`) | Names, per session, the processes that owe a session-ready report, and credits each named process once; an unnamed or repeated report is refused. Nothing waits on it for the marker since commit 9c93f82bb4. It is live for a second job: it is the population the replay fence is drawn from | extends: it stays, and its name and comments stop saying the End-of-RIB waits for it. Only the readiness channel beside it (`Peer.apiSyncReady`, `apiSyncReadyOnce`) has no product reader left; five tests read it, so its removal carries those tests |
| The replay fence | `internal/component/bgp/reactor/peer.go`, `Peer.raiseReplayFence`, `Peer.lowerReplayFence`, `Peer.initialUpdateOwed`; owners from `Peer.replayFenceOwners` (`peer_run.go`) through `registry.FencesLiveForwards` (declared by the `rib`, `rs` and `rr` plugins) | From Established until every fence owner has reported, a LIVE forwarded UPDATE for this peer is parked while an item of the replay passes, so a replayed announce is not overtaken by a live change of the same prefix | extends: it is the part of the 2026-08-30 ruling the 2026-09-18 one leaves standing (which routes are ordered inside the initial update); this spec does not change what it holds |

## Design (2026-11-14, approved by the owner the same day)

Approved at the design gate, each as recommended: D-1 approach A ("Join the
forward queue"), D-2 ("Withhold all automatic"), D-3 ("End the session"), and
the test work of D-5 stays in this spec.

The owner decision leaves no ordering to build. The design is five repairs,
each inside a mechanism that exists.

### D-1. An End-of-RIB a process asks for is never dropped

Facts read at the producers. `announceEOR` has a branch in front of everything
else: while the peer queues operations or the session still owes its marker,
the caller's marker is logged, counted as sent and not written. Past that
branch, a marker is claimed and placed at the tail of the peer's forward queue
(`queueEndOfRIB`, `forward_eor.go`), marked as part of the initial update. The
overflow release holds every item of that queue while the session's queueing
gate is closed (`Peer.forwardOrderHold`), and `sendInitialRoutes` opens the
gate only after its drain has emptied the peer's operation queue.
`docs/architecture/forward-congestion-pool.md`, "The replay fence", already
says the marker joins that queue "always". ExaBGP 4.2 and 6.0 hold a requested
marker until no routes are pending and then send it; they never drop it and
never deduplicate it.

| Approach | How it works | Gains | Costs |
|----------|--------------|-------|-------|
| A. Stop suppressing the explicit form (recommended) | The branch applies to the replay form only. An explicit request always takes the path the design page describes: claim when unclaimed, join the forward queue. The fence parks it until the sync has drained the peer's queued operations | No new state; the marker follows the caller's own queued routes, as ExaBGP's does; the answer "sent" becomes true; a `manual-eor` peer receives the marker it was promised | On a normal peer a marker asked for during the sync can be written as well as, or instead of, the sync's own (whichever claims the family first sends the automatic one; an explicit one is written each time, as it already is after the sync). Three unit tests pin "nothing written" and change through the test judge |
| B. Record the request on the session, sent by the sync's marker phase | A per-session set of requested families; the marker phase sends them and re-checks until none is left before clearing the owed flag | The marker comes from the producer that knows the table is done | New session state and a second drain-until-empty loop, beside a queue that already orders the marker |
| C. Queue the marker as an operation in the peer's queue, like a route-refresh marker | `PeerOpRefreshMarker` precedent | Keeps its place among queued routes | In the window after the gate opens nothing drains the queue, so the marker strands it and every later route with it; the first drain runs before Ze's family routes, putting the marker ahead of Ze's own table |
| D. Refuse the command with an error | The caller retries | Simple | ExaBGP scripts send the marker once and do not retry; a route server's replay would see an error during every sync |

The replay form keeps standing down while the sync owes its marker, and is
then answered as handled: on a normal peer the sync's marker is the one
automatic marker for that family, which is what the claim exists to give.

### D-2. `manual-eor` withholds every automatic marker, not only the sync's

Today the sync honours `manual-eor` and a replay does not: a replay that ends
after the sync claims the family and sends a marker the operator asked Ze not
to send, and one that ends during the sync is suppressed. The setting's own
description is that something else decides when this peer's table is complete.
The replay form stands down on a `manual-eor` peer at every point. An explicit
request is the "something else" and is always written (D-1).

This is a behaviour change nobody asked for by name. It is in the design
because D-1 makes the two producers' treatment of `manual-eor` visible side by
side; it is put to the owner at the gate as its own question.

### D-3. A sync that recovers from a panic ends its session

The recovery opens the queueing gate and clears the owed marker without
draining the queue, and the peer stays Established. With operations left in
the queue, the peer queues every later route into a queue nothing drains and
suppresses every later marker as redundant. The recovery ends the session
instead (the peer's existing teardown path), so the next session's sync drains
what is queued. A session whose initial update stopped at an unknown point has
no table the peer can trust.

How, from the producers read 2026-11-14:

| Step | What the recovery does | Why |
|------|------------------------|-----|
| 1 | Clears the session's queueing gate and owed marker and wakes the forward overflow, as it does today | the sync goroutine is ending and must leave nothing parked on its own facts |
| 2 | Moves the peer to Connecting, then ends its own session with the direct session teardown `runQueuedTeardown` uses (`sessionTeardown`), as an automatic stop | `Peer.Teardown` and `teardownOf` QUEUE a teardown while the sync is in flight, into the very queue nothing will drain. The state moves first for the reason `runQueuedTeardown` gives: a reconnect must not be refused as a collision |
| 3 | Leaves the operation queue as it is | unlike a queued teardown, which discards routes the operator's stop makes moot, these operations are still owed. An operation the interrupted drain had already sent is sent again by the next session, which is harmless for an announce or a withdraw |
| 4 | Sends no End-of-RIB before the NOTIFICATION | the initial update did not complete, so no marker is true |

As implemented (2cd7ac9f09, 2026-11-14), steps 1 and 2 run in the other
order: the state change and the teardown come first, and the gate and the
owed marker are cleared after, as the queued teardown does
(`endSessionAfterSyncPanic`). A route that arrives in between then joins the
kept queue for the next session rather than going to the session being ended.

The NOTIFICATION is the one the run loop's own panic recovery sends for an
internal fault (`docs/architecture/behavior/peer-lifecycle.md`, "`safeRunOnce`
and panic recovery" and "Panic recovery summary"); the implementation reads
that path and RFC 4486 before choosing a subcode, and does not invent one.
Settled 2026-11-14: it is Cease with subcode 0, Unspecific. RFC 4486 Section 3
lists eight Cease subcodes and none is for an internal fault, and RFC 4271
Section 4.5 says: "If no appropriate Error Subcode is defined, then a zero
(Unspecific) value is used for the Error Subcode field."

Found 2026-11-14 while the change was judged: a panic inside the socket write
left the interrupted message staged in the session's write buffer, and the
next flush sent it ahead of the NOTIFICATION. The first test judge refused the
count-only test change for this reason. The write-path release now drops what
an interrupted socket write left staged (`Session.releaseWrites`,
`discardInterruptedWrite`), and the unwind tests assert the wire content. Both
are in 2cd7ac9f09.

### D-4. The session-ready expectation set says what it is for

`Peer.apiSyncExpected`, `resetAPISync` and `creditAPIReady` stay: they arm and
lower the replay fence. Their names stay too (a rename reaches the plugin
server and several plugins for no behaviour). Every comment and page sentence
that says the End-of-RIB waits for the report is corrected to say the report
lowers the replay fence. `Peer.apiSyncReady` and `apiSyncReadyOnce` are
deleted; the five tests that read the channel observe `initialUpdateOwed`
going false, which is the fact they meant.

### D-5. Tests assert the one order Ze promises

From a per-file read of the 71 functional tests that place the marker at a
lower `seq` than a route on the same connection:

| Class | Count | Treatment |
|-------|-------|-----------|
| Held: a fixture or the scenario makes the route follow the marker (the fixture waits for the `eor-sent` counter, the source dials later, the peer acts after its own marker) | 43 | none |
| Usually held: the RPKI relay fixture waits for the counters to stay unchanged for one second and never requires a marker | 5 | the fixture waits for the marker counter to reach one, then for quiet (`plugin15DestinationSettled`, `internal/test/fixture/plugin_fixture_15_rpki.go`) |
| Race: a source sends at session up and nothing but timing puts the destination's marker first | 8 | the marker and the first racing route share one `seq` group: both stay required, neither is required first |
| Unsure: one fact each still undecided | 15 | read at implementation, then treated as its class |

Race files: `test/plugin/redistribute-export-reject.ci`,
`test/plugin/med-ibgp-post-selection-removal.ci`,
`test/plugin/rfc7999-blackhole-readvertise-agreement.ci`,
`test/plugin/rfc7999-blackhole-readvertise-agreement-forked.ci`,
`test/plugin/mcp-announce.ci`,
`test/reload/bgp-filter-community-reload.ci` (its `destb` block),
`test/reload/bgp-filter-export-reload-next-update-only.ci`,
`test/reload/rfc7999-blackhole-agreement-removed-by-reload.ci`. None uses the
marker as a fence: where a `reject=` needs one, a fence route does that job.

Unsure files, with the fact that decides each:
`test/plugin/healthcheck-cycle.ci` and `test/plugin/as112-healthcheck-withdraw.ci`
(whether the down probe can fire before the marker);
`test/reload/bgp-healthcheck-probe-reload.ci`,
`test/reload/bgp-healthcheck-probe-and-pool-reload.ci` and
`test/reload/bgp-watchdog-pool-reload.ci` (how the live-stages fixture orders
its script); `test/reload/bgp-role-export-reload.ci` (when its second UPDATE is
sent); `test/reload/bgp-policy-final-reload.ci`,
`test/plugin/policy-final-disposition.ci` and the seven
`test/reload/rfc8212-*.ci` files `explicit-policy-reload`, `import-inherited`,
`import-invalidate`, `import-invalidate-refresh`, `import-no-refresh`,
`import-owner-failure`, `import-rollback` (whether their fixtures wait for the
marker counter before the seed route).

The per-file table the research agent wrote is
`tmp/session/2026-11-14-111143/scratch/eor-order-census.md`; it is a session
file, so the lists above are the durable record.

**The fifteen unsure files, settled 2026-11-14.** A second read decided each,
and re-read the destination side of the two race files that had been
classified from their source side alone. This table replaces the "Unsure" row
and list above; the two confirmed race files keep their place in the race list.

| File | Class | What decides it | Change owed |
|------|-------|-----------------|-------------|
| `test/reload/bgp-policy-final-reload.ci`, `test/plugin/policy-final-disposition.ci` | Held | their fixtures wait for every peer's marker before the scenario sends anything | none |
| the seven `test/reload/rfc8212-*.ci` files | Held | `rfc8212ReloadDriver` (`internal/test/fixture/rfc8212_reload.go`) calls `waitPeersEOR03` for the three sessions before its scenario; read by the main thread | none |
| `test/reload/bgp-healthcheck-probe-reload.ci`, `test/reload/bgp-healthcheck-probe-and-pool-reload.ci`, `test/reload/bgp-watchdog-pool-reload.ci` | Held | the stages script polls the peer's sent counter, which counts the marker, before it reloads; the marker and the announce already share one `seq` group | none |
| `test/reload/bgp-role-export-reload.ci` | Usually held | the second UPDATE follows a reload, and only the reload's duration puts it after the marker | its stages script waits for the marker counter of both peers before the reload, which makes it held |
| `test/plugin/healthcheck-cycle.ci`, `test/plugin/as112-healthcheck-withdraw.ci` | Race, and one group cannot express it | the probe waits only for the plugins to be ready; the marker can land before the announce, between it and the withdraw, or after both, and the withdraw must follow the announce | the marker expectation is dropped from these two files, through the test judge: `bgp-healthcheck-probe-reload.ci` keeps asserting the marker beside a probe's announce, and the checker tolerates a marker nobody expects |
| `test/reload/bgp-filter-export-reload-next-update-only.ci` | Race, confirmed | the source sends at session up; the destination's marker races that first UPDATE | the destination's marker and first UPDATE share one group and the later expectations are renumbered behind them; its stages script waits for the source's marker counter before its first route |
| `test/reload/rfc7999-blackhole-agreement-removed-by-reload.ci` | Race, confirmed | same on its `destb` block; on `destc` nothing waits for that session | `destb` as above; its stages script waits for the marker counter of the source and of `destc` before its first route. `destc`'s `reject=` rests on its fence route, not on the marker |

The five RPKI relay tests whose fixture never requires the marker are
`test/plugin/bgp-rr-rpki-pending-withdraw-relay.ci`,
`test/plugin/bgp-rs-rpki-pending-withdraw-relay.ci`,
`test/plugin/bgp-rr-rpki-revalidate-relay.ci`,
`test/plugin/bgp-rs-rpki-revalidate-relay.ci` and
`test/plugin/bgp-rs-rpki-revalidate-relay-forked.ci`. Their fixture can pass
its settle wait before any marker is sent and then tear the first session
down, so the first connection's marker may never be written.

So the test work is exactly this: six `.ci` files gain a shared `seq` group
(the five race files under `test/plugin/` and
`bgp-filter-community-reload.ci`'s `destb` block), two reload-staged race files
gain a shared group and a marker wait, one usually-held file gains a marker
wait, two healthcheck files drop a marker expectation, and one fixture
function waits for the marker. The marker-counter wait in a stages script
assumes a single-family peer, which all the edited files are.

One thing the read did not settle: the 43 held files that push a route from a
stages script to a peer that may still be in its initial sync. Such a route
is queued and drains before the sync's marker (`sendInitialRoutes`, read by
the main thread), so a test expecting the marker first there would be wrong
the other way. Settled 2026-11-14 in phase 5: none of the 43 has that shape.
Each orders its route behind the marker by a fixture wait on the marker
counter, by the marker seen in the peer's output, or by a peer action that
fires only after it matched the marker (A-3).

The two announce-withdraw tests do not regain a marker expectation: their
withdraw can legitimately reach the wire before the marker, and no `seq` form
says "required, anywhere across two groups". A new test carries what they
lost (AC-1's functional proof): a process pushes one route and nothing else,
and the peer expects the route and the marker in one `seq` group.

### Triple challenge

| Challenge | Answer |
|-----------|--------|
| Simplicity | D-1 removes a branch for one form and adds no state. D-3 replaces three stores with the existing teardown. D-4 deletes a channel. The alternatives that add state (B) or a second marker path (C) are set aside for that reason |
| Uniformity | Every marker a caller asks for takes the one path the forward-pool page already describes. The replay form keeps the claim protocol it has |
| Performance | No hot path changes: `announceEOR` runs once per End-of-RIB command per peer, and the fence predicate is not touched. No allocation is added |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `Peer.sendInitialRoutes` (`internal/component/bgp/reactor/peer_initial_sync.go`): every session that reaches Established.

### Transformation Path
1. Current producer: drain the queue and clear its gate, wake forwarding overflow, then take the marker write hold and send the eligible family markers. Design still owes AC-2's ordering across that release/reacquire boundary and the treatment of process routes arriving concurrently.
2. Settled 2026-11-14: that order stays exactly as it is. Nothing is held for the marker, so a route that is not of Ze's own table may be written on either side of it (owner decision). The sync's sequence is unchanged by this spec except on a recovered panic, where it ends the session (D-3).

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Reactor ↔ plugin rail | the API sync expectation set, if it survives at all | No |
| Process → reactor | the text End-of-RIB command, on DirectBridge and over RPC alike; the replay form is marked by route metadata the caller sets | Yes, read 2026-11-14 (`announceEOR`, `routeServer.sendEOR`) |
| Process → reactor | the session-ready report, which lowers the replay fence and nothing else | Yes, read 2026-11-14 (`Peer.creditAPIReady`, `Peer.lowerReplayFence`) |
| Reactor → forward pool | the marker as a forward-queue item of the initial update | Yes, read 2026-11-14 (`queueEndOfRIB`, `settleEndOfRIB`) |

### Integration Points
- The four tests named in the Task, and whatever each asserts once the ordering is the product's

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes after D-1 | Today the suppression branch bypasses the forward queue the design page says every marker joins. D-1 removes the bypass for the explicit form |
| No engine code names a plugin or a feature | Yes | The replay form is told apart by the meta key the caller sets (`ReplayEORMeta`), and `manual-eor` is the peer's own setting; no plugin name is read |
| Works when the plugin runs in another process | Yes | A process asks for the marker through the text command on both transports; nothing in D-1 to D-4 adds an in-process call |

### Path of an End-of-RIB a process asks for, after D-1 and D-2

1. The process sends the text command (`update text nlri eor` or `mp <family> eor`); the ExaBGP bridge writes the same command for `announce eor`.
2. The command handler picks the replay form when the caller set the replay meta, and the explicit form otherwise.
3. `announceEOR`, per selected peer: a peer that is not Established or did not negotiate the family is skipped.
4. Replay form: it stands down, answered as handled, when the peer has `manual-eor`, when the session still owes its sync marker, or when the family is already claimed. Otherwise it claims the family.
5. Explicit form: it claims the family when unclaimed and is never stood down.
6. The marker joins the tail of the peer's forward queue as an item of the initial update.
7. The forward worker holds it while the session's queueing gate is closed, then writes it after everything queued ahead of it, and counts it as sent once the write has flushed. A failed write hands back a claim the item owns.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | A process's routes arriving AFTER the marker is correct, because a peer's do | owner ruling, 2026-09-18 | the 2026-08-30 ruling still governs and only the window closes, by widening `forwardOrderHold` instead | owner confirmation at the gate | confirmed 2026-11-14: the ruling is right, and the marker has no bearing on what Ze forwards |
| A-2 | AC-2 (a forwarded route never precedes the marker) is the order the owner wants | the skeleton's AC table, written 2026-09-18; no owner sentence states it | FRR and BIRD both send a forwarded route before their marker and RFC 4724 is silent, so AC-2 makes Ze stricter than both with no RFC sentence behind it; the header of `test/plugin/ipv4-announce-withdraw.ci` already reads the ruling as "A peer's routes have no defined order against our marker either" | owner decision at the research gate | broken 2026-11-14: Thomas ruled the marker has no bearing on what Ze forwards, so no order is promised and AC-2 is withdrawn |
| A-3 | The roughly 77 functional tests that place the marker before a forwarded route are load-dependent on the race rather than proofs of an order | census by the research agent over `test/` (652 files with a marker expectation); `redistribute-export-reject` read by the main thread, the rest classified by pattern | some of them hold the order by another mechanism (a source that sends only after the destination's marker), and they are unaffected by the decision | reading each affected test at design, before any is changed | confirmed in part 2026-11-14: a per-file read found 71 such tests, of which 55 hold the order by a real wait, 10 rest on timing (8 race, 2 healthcheck), 1 is usually held and 5 share a fixture that never requires the marker. The 43 first classed as held are confirmed again in phase 5 for the stages-script shape D-5 names. Confirmed 2026-11-14 in phase 5: all 43 files re-read, all 43 held, and none pushes a route from a stages script into a sync still running. The orderers are a fixture wait on the marker counter, the marker seen in the peer's output, or a peer action that fires only once it has matched the marker. One condition: a counter wait asks for one marker, so it holds only where the peer negotiates one family or the expected marker is the first family sent |
| A-4 | A route sent after the marker costs a graceful-restart helper a flush and relearn only when the helper held it as stale | derived by the research agent from FRR `bgp_update_receive_eor` and BIRD `bgp_rx_end_mark`; not re-read by the main thread | the cost is larger or absent, which changes the weight of the choice but not what the RFC permits | reading both receiver functions at design | confirmed 2026-11-14, refined, by a source-check agent that quoted the deciding lines (the main thread did not open the upstream files itself). FRR 10.7.1 `bgp_update_receive_eor` calls `bgp_clear_stale_route` only under `nsf`, and that removes only paths still flagged stale. BIRD 3.4.0 `bgp_rx_end_mark` calls `bgp_graceful_restart_done` only when `gr_active`, and `rt_refresh_end` prunes only routes of an older stale cycle. So the withdraw and re-announce cost falls only on a route the helper held as stale and that was not re-announced before the marker. Any other route after the marker is an ordinary UPDATE. During the receiver's own startup, its deferral or update-delay may already have ended, so such a route misses the first batch |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A consumer depends on plugin routes preceding the marker | the four tests, and any `.ci` asserting it | name every one during research and put the list to the owner. Research found none: no functional test places a process-pushed route strictly before the marker, and the fixtures that push routes wait for the marker themselves |
| R-2 | Stale text tells a third-party plugin author that the End-of-RIB waits for the session-ready report | `pkg/plugin/rpc/types.go` (`SignalsSessionReady`), `docs/architecture/api/process-protocol.md` "Session-Ready Declaration (Stage 1)", `docs/architecture/plugin/plugin-system.md` | correct each sentence in this work; the field's comment is part of the public SDK contract (`ai/doors/system.md`), and `ai/doors/bgp-plugins.md` already states the rule the pages must match |
| R-3 | Removing the dead readiness channel breaks the five tests that read it | `peer_initial_sync_test.go`, `peer_up_barrier_test.go`, `rs_receipt_readiness_test.go`, `api_sync_test.go` | each test is re-pointed at the fact it meant to observe (the fence lowering) through the test-judge route, or the channel stays with a reader named |
| R-5 | An explicit marker parked in the forward overflow during the sync is never released | the peer of `api-eor-during-initial-sync` never receives its marker | the gate's opening is already followed by the overflow wake in `sendInitialRoutes`; the unit test asserts the write with no further traffic |
| R-6 | An explicit marker that did not take the claim hands back a claim it does not own when its write fails (`settleEndOfRIB` releases by family, whoever claimed) | a second automatic marker for one family after a failed explicit write | read at implementation; if real, the forward item records whether it owns the claim. Confirmed real at the producer 2026-11-14 and fixed in a2302b8968: the queued item records whether it took the claim (`fwdEndOfRIB.ownsClaim`), and a failed write hands back only an owned claim (`settleEndOfRIB`) |
| R-7 | A test reactor without a forward pool writes an explicit marker at once, ahead of queued routes | a unit test on that path sees the marker first | production always builds the pool (`reactor.go`); the new unit tests run with one, and the no-pool arm keeps the suppression for both forms. Holds as written 2026-11-14: the reactor's constructor always builds the forward pool, and `announceEOR`'s no-pool arm stands down both forms while the peer queues or the sync owes its marker |
| R-10 | `request quiesce` does not see a requested marker still in the forward queue: its settle predicate (`Peer.pendingSync`) reads the operation queue, the queueing gate and the owed flag only. A replay's marker already had the same gap, so this is not a regression | a fixture that quiesces right after it asks for a marker tears the session down before the marker is written | fixtures wait for the peer's marker counter before they quiesce, as `fixture10EORDuringInitialSync` does. Resolved 2026-11-14: quiesce now waits while the peer's replay fence is up, while a policy re-send holds it, and while its forward queue holds items (`Peer.syncOwed`, `peer_sync_owed.go`), so a requested marker in the queue is waited for |
| R-8 | Ending the session on a recovered panic loops if the panic is deterministic for that peer's table | reconnect, panic, reconnect | the existing reconnect backoff bounds it, and the panic is logged with its stack each time; a looping peer is visible where a silently stuck one was not |
| R-9 | A `manual-eor` operator relies on today's replay marker reaching the peer | a route-server client configured with `manual-eor` stops receiving any marker until a process asks | the gate question on D-2; if the owner keeps today's behaviour, AC-6's survivor clause changes and D-2 is dropped. Answered 2026-11-14: withhold all automatic markers. The change is stated in `docs/guide/configuration.md` and in the news fragment at closure, so such an operator reads it |
| R-4 | A hold added before the marker parks forwards that nothing wakes | a forwarded UPDATE delivered only when the next one arrives | every store that ends a hold is followed by the overflow wake, and a unit test asserts delivery with no further traffic |

### Implementation notes (2026-11-14)

The spec was implemented without a claim: the WIP cap refused it (15 specs in
progress, cap 12), and the owner had ordered the work. The Status therefore
stayed `ready` during implementation. The closure claims it one above the
in-progress count only to close it. The work landed as a2302b8968 (D-1, D-2,
R-6), 2cd7ac9f09 (D-3), 0461489d41 (D-5 and the two new tests) and 4aa1f84ff6
(D-4).

The stress run of the changed tests exposed an existing race in a file this
spec regrouped: `test/plugin/rfc7999-blackhole-readvertise-agreement-forked.ci`
failed when `request quiesce` reported a peer settled while a route server's
peer-up replay to it was still in flight, so the fixture stopped the daemon
mid-replay. `Peer.pendingSync` read the operation queue, the queueing gate, the
owed End-of-RIB and a live handshake, never the replay fence nor the peer's
forward queue. It was fixed here and not parked, for three reasons. It is the
same surface: what `request quiesce` reports about the initial update. The page
that documents the barrier (`docs/architecture/api/commands.md`) did not list
the replay as excluded. The fix reuses the existing fence and forward counters
(`Peer.syncOwed`, `peer_sync_owed.go`). One consequence: a fence owner that
never reports its replay done now makes a quiesce on that peer time out, with
an error that names the peer and the process. Before, the same owner silently
stranded that peer's live forwards.

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A graceful-restart helper folds fewer routes into its deferred batch, and a plugin's routes land after the marker instead of inside it |
| How is it reverted? | One commit |
| Who else touches this path? | the two forwarding rails, the route-server replay, the API `eor` command |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| a session established with a route-pushing plugin configured | → | `sendInitialRoutes` writes the marker without waiting | `test/plugin/initial-sync-marker-beside-process-route.ci` (new) |
| `update text nlri eor` from a process during the initial sync | → | `announceEOR` queues the marker behind the forward queue | `test/plugin/api-eor-during-initial-sync.ci` (new) |
| the same command on a `manual-eor` peer, through the ExaBGP bridge | → | `announceEOR`, the sync's withheld marker | `test/exabgp-compat/api/api-manual-eor.ci` (existing, its one-second sleep replaced by a wait for the session to be up). Corrected 2026-11-14: removing the sleep does not make the request land inside the sync. The bridge acknowledges the peer-up event once it has written the line to the script (`Bridge.handleDeliverEvent`), so an external script cannot be made to ask inside the sync. This test proves a requested marker reaches a `manual-eor` peer whenever the request lands; the during-sync case is row 2's test and `TestExplicitEndOfRIBDuringSyncFollowsQueuedRoutes` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | a session establishes while a process that pushes routes is configured | ze's End-of-RIB follows its own table with no wait for that process |
| AC-2 | a route forwarded from another peer arrives during session start | WITHDRAWN 2026-11-14 by owner decision (first written: "it does not reach the wire before the marker"). No order is promised between the marker and a route that is not of Ze's own table. The design phase restates the criteria that replace it |
| AC-3 | the four tests named in the Task | each asserts what the product now promises, and none asserts an ordering it does not |

Criteria restated after the owner decision (2026-11-14). AC-1 and AC-3 above
stand; AC-3 is widened by AC-4. Each row names what is still permitted and why.

| AC ID | Input / Condition | Expected Behavior | Permitted survivors, and why |
|-------|-------------------|-------------------|------------------------------|
| AC-1 | a session establishes while a process that pushes one route is attached | the peer receives Ze's End-of-RIB for the family and the process's route, in either order | either order: the marker waits for no process and has no bearing on what Ze forwards (owner, 2026-09-18 and 2026-11-14) |
| AC-4 | each functional test that expects the marker before a route that is not of Ze's own table, with nothing but timing ordering them | the test requires both the marker and the route and requires neither first; what the test exists to prove is still proven, by the same lines | a test whose fixture or scenario orders the route after the marker keeps its order: the order is the test's doing, not a product promise |
| AC-5 | a process asks for an End-of-RIB while the peer's initial sync is still running, with routes queued for that peer | the peer receives the marker after those queued routes, and the command's answer counts a marker that was written | the replay form is still answered as handled without a write while the sync owes its marker on a peer without `manual-eor`: the sync's marker is that family's one automatic marker |
| AC-6 | the same request on a peer with `behavior { manual-eor true }` | the peer receives exactly the marker the process asked for, once per request | no automatic marker reaches that peer from the sync or from a replay: the operator set `manual-eor` to decide the marker themselves |
| AC-7 | the initial sync panics with operations still queued for the peer | the session ends, and the next session's sync sends the queued operations | none: no Established peer is left holding a queue nothing drains |
| AC-8 | a reader, including a third-party plugin author, looks up what the session-ready report does | every page and comment says it lowers the replay fence, and none says the End-of-RIB waits for it | none |
| AC-9 | the readiness channel no product code waits on | it is gone, and the tests that read it observe the replay fence lowering | none |
| AC-10 | the two journal rows this spec owns | each is closed with the commit that fixed it | none |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | brings up a session on a router carrying a route-pushing plugin | Ze's own table, then the marker; the plugin's route before or after it | `test/plugin/initial-sync-marker-beside-process-route.ci` |
| 2 | runs an ExaBGP script that announces its routes and then `announce eor` against a `manual-eor` peer, with no delay | bridge, text command, `announceEOR`, forward queue, wire | `test/exabgp-compat/api/api-manual-eor.ci`, which proves the marker arrives whenever the request lands; a request made during the sync is proven by `test/plugin/api-eor-during-initial-sync.ci` and `TestExplicitEndOfRIBDuringSyncFollowsQueuedRoutes` (corrected 2026-11-14: a bridge script cannot be made to ask inside the sync) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestInitialSyncMarkerWaitsForNoProcess` | `internal/component/bgp/reactor/peer_initial_sync_test.go` | AC-1 | existing, green |
| `TestExplicitEndOfRIBDuringSyncFollowsQueuedRoutes` | `internal/component/bgp/reactor/reactor_api_forward_test.go` | AC-5: a marker asked for with the gate closed is written after the queued announce, and the answer counts it | new |
| `TestReplayEndOfRIBStandsDownWhileSyncOwesItsMarker` | same file | AC-5's permitted survivor: the replay form writes nothing and is answered handled | new, or the existing suppression tests re-pointed at the replay form |
| `TestManualEORPeerReceivesOnlyTheRequestedMarker` | same file | AC-6: one write per explicit request, none from the sync, none from a replay at any point | new |
| `TestInitialSyncPanicEndsTheSession` | `internal/component/bgp/reactor/peer_initial_sync_unwind_test.go` | AC-7: after a recovered panic with a queued operation the session is ended and the queue is kept for the next one. AC-7's "the next session's sync sends the queued operations" rests on the existing queue drain of `sendInitialRoutes` and has no two-session test of its own (2026-11-14) | new |
| the wire-content assertions of the three unwind tests | `internal/component/bgp/reactor/peer_initial_sync_unwind_test.go` | D-3: after a recovered panic the wire holds what the socket took before the fault, then only the Cease / Unspecific NOTIFICATION, with no End-of-RIB (2cd7ac9f09) | changed through the test judge |
| the five readers of `apiSyncReady` | `peer_initial_sync_test.go`, `peer_up_barrier_test.go`, `rs_receipt_readiness_test.go`, `api_sync_test.go` | AC-9: each observes `initialUpdateOwed` going false | changed through the test judge |
| `TestAnnounceEOR_SkipsPeerInInitialRouteSync`, `TestAnnounceEORDefersToCurrentSessionMarker`, `TestInitialSyncSuppressesAnotherProducersEORWhileItsOwnIsOwed` | `reactor_api_forward_test.go`, `peer_initial_sync_test.go` | they pin "an explicit marker is not written while the sync owes its own", which D-1 replaces | changed through the test judge: the old expectation is the defect in the owned journal row |
| `TestPendingSyncWaitsForTheReplayFence` | `internal/component/bgp/reactor/peer_sync_owed_test.go` | R-10: an Established peer whose replay fence is up is not settled | new (2026-11-14) |
| `TestPendingSyncWaitsForAPolicyReplay` | same file | R-10: a peer held by a policy re-send is not settled | new (2026-11-14) |
| `TestPendingSyncWaitsForAForwardOnTheChannel` | same file | R-10: a forward item on the worker's channel keeps the peer unsettled | new (2026-11-14) |
| `TestPendingSyncWaitsForAForwardInOverflow` | same file | R-10: a forward item in overflow keeps the peer unsettled | new (2026-11-14) |
| `TestDrainPeerSyncErrorNamesWhatEachPeerOwes` | same file | the drain's timeout error names each owed peer, what it owes, and the process a replay waits for | new (2026-11-14) |
| `TestQuiesceErrorCarriesWhatTheSubsystemOwes` | `internal/component/plugin/server/quiesce_test.go` | `quiesceAll` carries each failed subsystem's own error in its answer | new (2026-11-14) |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| the sync timeout, if it survives at all | none | N-A | N-A | N-A |

No numeric input is added or changed. The per-peer sync timeout was removed by
commit 9c93f82bb4 and nothing replaces it; the peer-up barrier's two-second
bound is not touched.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `initial-sync-marker-beside-process-route` | `test/plugin/initial-sync-marker-beside-process-route.ci` | AC-1: a process pushes one route; the peer requires the route and the marker in one `seq` group | new |
| `api-eor-during-initial-sync` | `test/plugin/api-eor-during-initial-sync.ci` | AC-5 and AC-6: a process announces a route and asks for the marker before the session is up, on a `manual-eor` peer; the peer requires the route, then exactly one marker. Corrected 2026-11-14: the process asks from inside the peer-up delivery (`fixture10EORDuringInitialSync`), not before the session is up (`announceEOR` skips a peer that is not Established). The engine completes that delivery before it starts the sync, so the queueing gate is closed on every run. Its first version asked after the sync and stayed green against the old drop; the committed version (0461489d41) went red against the old drop ("ze never sent its End-of-RIB to peer1") and green on a2302b8968 | new |
| `api-manual-eor` | `test/exabgp-compat/api/api-manual-eor.ci` | AC-6 through the bridge, with the script's one-second sleep removed so the request lands inside the sync. Corrected 2026-11-14: the sleep is replaced by a wait for the session to be up, and the request still need not land inside the sync, because the bridge acknowledges the peer-up event once it has written the line to the script. The test proves a requested marker reaches a `manual-eor` peer whenever the request lands; the during-sync case is `api-eor-during-initial-sync` and `TestExplicitEndOfRIBDuringSyncFollowsQueuedRoutes` | existing, changed |
| the eight race files of D-5 | `test/plugin/`, `test/reload/` | AC-4 | changed through the test judge |
| the five RPKI relay tests | fixture `internal/test/fixture/plugin_fixture_15_rpki.go` | AC-4: the fixture waits for the marker counter | fixture changed |
| the fifteen unsure files of D-5 | `test/plugin/`, `test/reload/` | AC-4, after the deciding fact is read | assessed at implementation |
| settled 2026-11-14, replacing the row above: `bgp-role-export-reload` | `test/reload/bgp-role-export-reload.ci` | AC-4: its stages script waits for both peers' marker counter before the reload | existing, changed |
| `healthcheck-cycle`, `as112-healthcheck-withdraw` | `test/plugin/healthcheck-cycle.ci`, `test/plugin/as112-healthcheck-withdraw.ci` | AC-4: the marker expectation, which no `seq` form can place, is dropped; the announce and the withdraw are still required in order | existing, changed through the test judge |
| the twelve held files of the settled table | `test/reload/`, `test/plugin/` | AC-4's permitted survivor | existing, unchanged |
| `redistribute-export-reject` | `test/plugin/redistribute-export-reject.ci` | AC-3: asserts what the product promises about the marker and the routes around it | existing, owes assessment |
| `ipv4-announce-withdraw` | `test/plugin/ipv4-announce-withdraw.ci` | AC-3 | existing, owes assessment |
| `ipv6-announce-withdraw` | `test/plugin/ipv6-announce-withdraw.ci` | AC-3 | existing, owes assessment |
| `show-bgp-bare-runs-summary` | `test/plugin/show-bgp-bare-runs-summary.ci` | AC-3 | existing, owes assessment |

## Files to Modify
- `internal/component/bgp/reactor/peer_initial_sync.go` - assess the remaining marker/forwarding ordering; the external route-process wait is already removed. Design doc: `docs/architecture/core-design.md`.

- `internal/component/bgp/reactor/reactor_api_forward.go` - `announceEOR`: the suppression branch covers the replay form only; the replay form stands down on a `manual-eor` peer; `logEORSuppressed` loses the branches that can no longer be reached and says only true things. The comment citing RFC 4724 Section 2 for "one End-of-RIB per family per session" is corrected: the RFC has no such sentence, the rule is the claim protocol's. Design doc: `docs/architecture/forward-congestion-pool.md`.
- `internal/component/bgp/reactor/peer.go` - delete `apiSyncReady` and `apiSyncReadyOnce`; correct the comments on `apiSyncExpected` and `SignalAPIReady`, including the spec path they cite.
- `internal/component/bgp/reactor/peer_run.go` - the comment on `initialUpdateReporters` ("its End-of-RIB waits for each of them to report").
- `internal/component/bgp/reactor/source_policy_reload.go`, `internal/component/bgp/reactor/session_write.go` - stale comments in `enterEstablished` (a barrier and a timeout diagnostic that no longer exist) and on `holdWrites` (the forward pool blocks; the route-server fast path is the one that tries the lock).
- `internal/component/bgp/plugins/adj_rib_in/rib.go` - `sessionReadyTimeout` cites `apiSyncTimeout`, a constant removed by 9c93f82bb4.
- `internal/component/bgp/plugins/rib/rib_replay.go` - "the signal the reactor waits on".
- `internal/component/plugin/registry/registry.go`, `pkg/plugin/rpc/types.go` - the `SignalsSessionReady` comments ("holds its End-of-RIB for that report"). The second is the public SDK contract. The `PeerUpBarrier` comment beside it is correct and stays.
- `internal/test/fixture/plugin_fixture_15_rpki.go` - `plugin15DestinationSettled` waits for the marker counter.
- `docs/architecture/behavior/peer-lifecycle.md` (step 4 of the Established transition), `docs/architecture/plugin/plugin-system.md`, `docs/architecture/api/process-protocol.md`, `docs/architecture/api/update-syntax.md`, `docs/architecture/forward-congestion-pool.md`, `docs/plugin-development/` (the session-ready report, which it does not mention today), `docs/guide/configuration.md` (`manual-eor`, absent today). The sentences in the first four come from the research agent's report and are confirmed line by line when each page is edited.
- `test/plugin/plugin-nexthop.ci` - its header still quotes the 2026-08-30 ruling as current.
- `plan/journal/grace-bound-hides-a-missing-signal.md`, `plan/journal/guard-addition-drops-what-it-refuses.md` - the two owned rows.
- The eight race files and the changed existing tests named in the test plan.

Corrected 2026-11-14, after implementation. The `holdWrites` comment in
`session_write.go` was already corrected by the session-write-wedge work
(178cffd97d), so this spec did not edit it; it added only the discard call at
the top of `releaseWrites` (2cd7ac9f09). The implementation also touched these
files, which the list above lacks:

- `internal/component/bgp/reactor/forward_eor.go` - the queued marker records whether it took the claim (`fwdEndOfRIB.ownsClaim`), and `settleEndOfRIB` hands back only an owned claim (R-6).
- `internal/component/bgp/reactor/session_write_interrupted.go` (new) - `discardInterruptedWrite` drops what a socket write a panic unwound through left staged (D-3).
- `internal/component/bgp/reactor/session_wire.go` - the socket boundary marks a write that never returned (`observedBGPWriter.writeOpen`), which the discard reads (D-3). Design doc: `docs/architecture/mrt.md`.
- `internal/component/bgp/reactor/peer_settings.go` - the `ManualEOR` comment: every automatic marker is withheld, and a requested one is always written (D-2).
- `internal/component/bgp/types/types.go` - the `ReplayEORMeta` comment: when the engine treats a replay's marker as an automatic one (D-1, D-2).
- `internal/component/bgp/reactor/peer_derived_bindings.go`, `internal/component/bgp/reactor/api_sync.go` - comments on the session-ready expectation set and the replay fence it arms (D-4, 4aa1f84ff6).
- `internal/component/plugin/server/events.go`, `pkg/plugin/rpc/enums.go` - the same correction in `Server.declaresSessionReady` and the `ReasonAttached` comment (D-4, 4aa1f84ff6).
- `internal/test/fixture/plugin_fixture_10.go` - `fixture10EORDuringInitialSync`, the fixture of `api-eor-during-initial-sync`, which asks from inside the peer-up delivery.
- `internal/test/fixture/register_initial_sync_marker_beside_process_route.go` (new) - the fixture of `initial-sync-marker-beside-process-route`.
- `test/exabgp-compat/etc/api-manual-eor.conf`, `test/exabgp-compat/etc/run/api-manual-eor.run` - the script waits for the session to be up in place of the sleep, and its header says what the test can and cannot prove.
- `docs/exabgp/exabgp-differences.md` - a requested marker is held behind pending routes and never dropped, as in ExaBGP (Documentation checklist row 6).

Added 2026-11-14, for the quiesce race in the implementation notes (R-10):

- `internal/component/bgp/reactor/peer_sync_owed.go` (new) - `Peer.syncOwed` names what a peer still owes, the replay fence, a policy re-send and the forward queue included; `peerSyncOwedError` turns it into the drain's timeout error.
- `internal/component/bgp/reactor/peer.go` - `Peer.pendingSync` reads `Peer.syncOwed`.
- `internal/component/bgp/reactor/reactor_api.go` - `DrainPeerSync` returns `peerSyncOwedError` when its context ends first.
- `internal/component/plugin/server/quiesce.go` - `quiesceAll` answers with an operator error that carries each failed subsystem's own error.
- `internal/component/bgp/reactor/peer_sync_owed_test.go` (new), `internal/component/plugin/server/quiesce_test.go` - the six tests in the unit test table.
- `docs/architecture/api/commands.md` - "What `bgp-peer-sync` waits for".

## Files to Create
- `test/plugin/initial-sync-marker-beside-process-route.ci`
- `test/plugin/api-eor-during-initial-sync.ci`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | no leaf |
| CLI commands/flags | N-A | none |
| Functional test for new RPC/API | Yes | corrected 2026-11-14 (AC-2 is withdrawn): `test/plugin/api-eor-during-initial-sync.ci` and `test/plugin/initial-sync-marker-beside-process-route.ci` |
| Interop scenario (wire-visible change) | Yes | `test/interop/scenarios/bgp-manual-eor-requested-frr/` (9609d00703): a requested End-of-RIB on a `manual-eor` peer, against FRR 10.7.1; green, and red under both breaks (Hand-back record, row 1) |
| Prometheus counters/metrics | No | `eorSent` keeps its contract, markers that reached the socket |
| BGP family surface (new SAFI / capability / attribute) | N-A | no family change |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | none |
| 7 | Wire format changed? | No | the marker is unchanged; its timing is not the format |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | RFC 4724 Section 2, and the `rfc/short/` summary carrying the marker's requirements |
| 12 | Internal architecture changed? | Yes | `docs/architecture/behavior/peer-lifecycle.md` (the decision, landed 2026-11-14; step 4 and the panic recovery table in phases 3 and 4), `docs/architecture/forward-congestion-pool.md` (phase 2) |
| 2 | Config option changed? | Yes | `docs/guide/configuration.md`: `behavior { manual-eor }` has no entry today, and D-2 changes what it withholds (phase 2) |
| 4 | API or plugin contract changed? | Yes | `docs/architecture/api/process-protocol.md`, `docs/architecture/plugin/plugin-system.md`, `docs/plugin-development/` and the `pkg/plugin/rpc/types.go` comment: what the session-ready report does (phase 4); `docs/architecture/api/update-syntax.md`: what the `eor` keyword's answer counts (phase 2) |
| 6 | ExaBGP behaviour difference? | Yes | `docs/exabgp/exabgp-differences.md` says nothing on `eor`; after D-1 the bridge matches ExaBGP (a requested marker is held behind pending routes and never dropped), which the page states in phase 2 |

## Implementation Steps

1. **Phase: Research** -- read the three producers, re-read the 2026-08-30 ruling in full, and name every consumer that depends on plugin routes preceding the marker
   - Verify: the list of dependents, and the owner's answer on A-1
   - Done 2026-11-14: see "Research corrections" and "Owner decision at the research gate".
2. **Phase: the explicit marker (D-1, D-2)** -- write the three new unit tests and `test/plugin/api-eor-during-initial-sync.ci` first and see them red; change `announceEOR` and `logEORSuppressed`; take the three existing suppression tests through the test judge; remove the sleep from the ExaBGP compatibility script. Pages in the same phase: `forward-congestion-pool.md`, `update-syntax.md`, `docs/guide/configuration.md`.
   - Verify: the new tests green, `api-manual-eor` green with no sleep, the journal row of 2026-11-14 closed.
3. **Phase: the panic recovery (D-3)** -- `TestInitialSyncPanicEndsTheSession` red first; the recovery ends the session; `peer-lifecycle.md`'s panic recovery table in the same phase.
   - Verify: the test green, `TestInitialSyncPanicReleasesWriteHolds` still green.
4. **Phase: the expectation set (D-4)** -- delete the channel, re-point its five tests through the test judge, correct every comment and page in Files to Modify, add the session-ready report to `docs/plugin-development/`.
   - Verify: no sentence in the tree says the End-of-RIB waits for a session-ready report; the journal row of 2026-08-30 closed after its producer is re-read.
5. **Phase: the tests (D-5)** -- add `initial-sync-marker-beside-process-route.ci`; read the deciding fact of each unsure file; change the race files and the RPKI fixture, each through the test judge with its header saying what the change keeps.
   - Verify: each changed test run under load with `./le test stress-repro`, and the recorded break of each still goes red.

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Correctness | the marker still goes out once per family per session, and no forwarded UPDATE precedes it |
| Correctness, restated after the owner decision | Ze's own table precedes its marker; one AUTOMATIC marker per family per session; an explicit marker is written once per request and after the peer's queued routes; nothing is held back for the marker |
| No vacuous test | each new test is seen red against the unchanged code first; each changed race test still goes red under its recorded break |
| No silent answer | no branch of `announceEOR` counts a marker as sent that was not written or claimed for the sync's own marker |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| The marker no longer waits for a route-creating process | AC-1's test |
| A requested marker is written, after the queued routes, and answered truthfully | `test/plugin/api-eor-during-initial-sync.ci`, `test/exabgp-compat/api/api-manual-eor.ci` with no sleep |
| No automatic marker on a `manual-eor` peer | `TestManualEORPeerReceivesOnlyTheRequestedMarker` |
| No Established peer with an undrainable queue | `TestInitialSyncPanicEndsTheSession` |
| No sentence says the End-of-RIB waits for a session-ready report | a search of `docs/`, `internal/`, `pkg/` and `test/` for the phrases listed in Files to Modify returns nothing |
| The race tests assert no unpromised order | each of the eight files carries the marker and its first racing route in one `seq` group, with the test judge's approval on its commit |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Input validation | none: this adds no external input |
| Resource exhaustion | a process that asks for a marker repeatedly during the sync adds one forward-queue item per request; the forward queue's existing bound and overflow accounting apply, and the command is already limited to processes the peer grants a send word |
| Denial of service through D-3 | a peer or a process cannot trigger the sync's panic recovery by input this spec adds; a deterministic panic becomes a visible reconnect loop under the existing backoff instead of a silent stall |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| grace-bound-hides-a-missing-signal | 2026-08-30 | `(*Peer).SignalAPIReady` anonymous sync counter masks a missing route-process signal |
| guard-addition-drops-what-it-refuses | 2026-11-14 | `announceEOR` drops a `manual-eor` peer's End-of-RIB during the sync or a queued drain and answers it sent |

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] No code snippets
- [ ] Current Behavior and Data Flow sections completed
- [ ] Every assumption has a Basis and a validation method

### Goal Gates (MUST pass)
- [ ] AC-1..AC-3 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] The scoped evidence passes, run by the main thread: the reactor package under `ze_bgp`, the named unit tests, and each functional test in the test plan. No agent runs any `./le verify` command; GitHub CI verifies the pushed commit (`ai/META-DECISIONS.md`)
- [ ] Feature code integrated (`internal/*`), not library-only
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional `.ci` tests for end-to-end behavior

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)

## Implementation Summary

### What Was Implemented
- D-1 and D-2 (a2302b8968): `reactorAPIAdapter.announceEOR` stands down only the replay form (`once`) while the peer queues or its sync owes its marker, and stands it down on every `manual-eor` peer (`logEORWithheldByManualEOR`). An explicit request claims the family when unclaimed and joins the forward queue (`queueEndOfRIB`), which `Peer.forwardOrderHold` holds while the queueing gate is closed. R-6: the queued item records whether it took the claim (`fwdEndOfRIB.ownsClaim`) and `settleEndOfRIB` hands back only an owned claim.
- D-3 (2cd7ac9f09): the recovery of `Peer.sendInitialRoutes` calls `Peer.endSessionAfterSyncPanic`, which moves the peer to Connecting, ends the session with a Cease / Unspecific through `sessionTeardown`, keeps the opQueue, then clears the gate and the owed marker and wakes the forward overflow. `Session.releaseWrites` first calls `Session.discardInterruptedWrite`, which drops what a socket write a panic unwound through left staged (`observedBGPWriter.writeOpen`).
- D-4 (4aa1f84ff6): `Peer.apiSyncReady` and `apiSyncReadyOnce` deleted; every comment and page says the session-ready report is credited and lowers the replay fence, and that the End-of-RIB waits for no report. a64cf431a1 corrected the last four test strings.
- D-5 (0461489d41, 8d8a506457): the race files share one `seq` group for the marker and the first racing route; reload stages scripts wait for the marker counter; the RPKI fixture waits for the marker, then for quiet; two new functional tests; the drivers of three tests stop the daemon themselves so a lingering peer cannot race the stop.
- R-10 (49b01f67ac): `Peer.pendingSync` reads `Peer.syncOwed`, which also waits for the replay fence, a policy re-send and the forward queue; `DrainPeerSync` answers `peerSyncOwedError` on timeout and `quiesceAll` answers an operator error carrying each subsystem's error.
- Interop (9609d00703): scenario `bgp-manual-eor-requested-frr` against FRR 10.7.1.

### Bugs Found/Fixed
- A requested End-of-RIB dropped during the sync and answered as sent: `test/plugin/api-eor-during-initial-sync.ci` (red against the old drop, green on the fix) and the interop scenario's assertion 7.
- A failed requested marker released a claim the sync held: `TestFailedEndOfRIBWriteHandsBackOnlyAnOwnedClaim`.
- A recovered sync panic left an Established peer with an undrainable queue: `TestInitialSyncPanicEndsTheSession`.
- The interrupted frame of a panicking socket write rode out ahead of the Cease: the wire assertions of the three unwind tests (`peer_initial_sync_unwind_test.go`).
- `request quiesce` settled while a replay was still passing: `TestPendingSyncWaitsForTheReplayFence` and the three sibling tests; the stress failure of `rfc7999-blackhole-readvertise-agreement-forked.ci` did not recur in 80 runs.

### Documentation Updates
- a2302b8968: `docs/architecture/api/update-syntax.md`, `docs/architecture/forward-congestion-pool.md`, `docs/exabgp/exabgp-differences.md` ("An `announce eor` sent while the session starts", anchor `reactor_api_forward.go -- AnnounceEOR, announceEOR`), `docs/guide/configuration.md` (`behavior { manual-eor; }` row).
- 2cd7ac9f09 and 4aa1f84ff6: `docs/architecture/behavior/peer-lifecycle.md` (panic recovery, step 4), `docs/architecture/plugin/plugin-system.md`, `docs/architecture/api/process-protocol.md`, `docs/architecture/api/architecture.md`, `docs/plugin-development/protocol.md` ("The session-ready report").
- 49b01f67ac: `docs/architecture/api/commands.md`, "What `bgp-peer-sync` waits for". 8d8a506457: `docs/architecture/testing/ci-format.md`. 9609d00703: `docs/architecture/testing/interop.md`.
- This closure: `docs/architecture/api/architecture.md` step 6 no longer names the deleted `apiSyncReady` (it names `Peer.creditAPIReady` and `Peer.lowerReplayFence` inline); `docs/architecture/testing/interop.md` inventory row 69 records the green run and the two red walks in place of "Not yet run".
- `./le doc check all`, run by the second closure agent after its edits: one failure, not this work's. The live command catalog and the wiki catalog disagree on `show data check` (`internal/le/cli/catalog/catalog.go`, column `vanished-entries`), which the data-store session is changing. Every source anchor resolves, the digest anchors included.

### Deviations from Plan
- AC-2 withdrawn by the owner at the research gate; AC-4 to AC-10 replace it (A-2 broken).
- D-3's steps 1 and 2 run in the other order (state and teardown first, facts cleared last), for the reason `runQueuedTeardown` gives; recorded under D-3.
- `api-manual-eor` through the bridge cannot place its request inside the sync; the during-sync case is proven by `api-eor-during-initial-sync` and the interop scenario (Wiring Test row 3).
- R-10 was fixed here rather than recorded: same surface (what `request quiesce` reports about the initial update), found by this work's stress run.
- The interop scenario reads FRR's log rather than negotiating Graceful Restart, as the hand-back proposed: the GR flag can show neither a count nor an order.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-2: AC-2 (a forwarded route never precedes the marker) was taken as the owner's order | The owner ruled the marker has no bearing on what Ze forwards; FRR and BIRD let a forwarded route precede their marker | research gate | AC-2 withdrawn, AC-4 to AC-10 written |
| approach | The first `api-eor-during-initial-sync` asked for the marker after the sync, so it stayed green against the old drop | A request lands inside the sync only from inside the peer-UP handler the engine awaits | red walk against the old drop | rewritten (0461489d41); lesson in `plan/learned/052-eor-treats-route-processes-as-peers.md` |
| approach | Phase 3's first test change counted writes only | The interrupted frame was flushed ahead of the Cease | an independent test judge refused it | `discardInterruptedWrite`, wire assertions (2cd7ac9f09) |
| approach | Phase 4 corrected every page that said the marker waits for the report, but left `docs/architecture/api/architecture.md` step 6 naming the deleted `apiSyncReady` | The channel was gone in the same commit | second closure's AC-8 search | corrected in this closure |
| approach | The first closure found no interop scenario for a wire-visible change | `ai/doors/bgp-engine.md`, "Gates", owes `./le test integration interop` for any wire-visible change | first closure review | scenario added (9609d00703) |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| The marker waits for no route-creating process | Done | `Peer.sendInitialRoutes` (`peer_initial_sync.go`) | removed by 9c93f82bb4, pinned by `TestInitialSyncMarkerWaitsForNoProcess` |
| The four named tests assert only what the product promises | Done | `test/plugin/` | none asserted an unpromised order (Research corrections) |
| `initialSyncEOROwed` suppression assessed | Changed | `reactorAPIAdapter.announceEOR` | it now stands down only the replay form (D-1) |
| `apiSyncExpected` and `waitForAPISync` consumers assessed | Done | `Peer.resetAPISync`, `Peer.creditAPIReady` | the set stays and arms the fence; `apiSyncReady` deleted (D-4) |
| The 2026-08-30 ruling re-read | Done | Required Reading | only its "The marker waits for it" is superseded |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestInitialSyncMarkerWaitsForNoProcess`; `test/plugin/initial-sync-marker-beside-process-route.ci` | stress 80/80 |
| AC-2 | Changed | withdrawn by the owner | A-2 |
| AC-3 | Done | the four files read; none asserts an unpromised order | Research corrections |
| AC-4 | Done | the eight race files, the reload stages waits, the RPKI fixture, the two healthcheck files (0461489d41) | judged; stress 80/80 plugin and reload |
| AC-5 | Done | `TestExplicitEndOfRIBDuringSyncFollowsQueuedRoutes`, `TestReplayEndOfRIBStandsDownWhileSyncOwesItsMarker`, `api-eor-during-initial-sync.ci`, interop assertion 7 | |
| AC-6 | Done | `TestManualEORPeerReceivesOnlyTheRequestedMarker`, `TestManualEORPeerWithholdsAReplayEndOfRIBAfterTheSync`, interop assertions 3 and 8 | |
| AC-7 | Done | `TestInitialSyncPanicEndsTheSession` and the unwind wire assertions | the next session's drain is the existing `sendInitialRoutes` opQueue drain, with no two-session test (NOTE N-3) |
| AC-8 | Done | the search in the Hand-back record, row 2; `docs/plugin-development/protocol.md`, `pkg/plugin/rpc/types.go` | one page sentence corrected at closure |
| AC-9 | Done | `apiSyncReady` absent from `internal/` and `pkg/` product code; the five tests observe `initialUpdateOwed` | |
| AC-10 | Done | `plan/journal/guard-addition-drops-what-it-refuses.md` 2026-11-14 and `plan/journal/grace-bound-hides-a-missing-signal.md` 2026-08-30, both `fixed:` with their commits | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestInitialSyncMarkerWaitsForNoProcess` | Done | `peer_initial_sync_test.go` | RFC4724-4-1 re-judged with a record (923628582e) |
| `TestExplicitEndOfRIBDuringSyncFollowsQueuedRoutes`, `TestReplayEndOfRIBStandsDownWhileSyncOwesItsMarker`, `TestManualEORPeerReceivesOnlyTheRequestedMarker`, `TestManualEORPeerWithholdsAReplayEndOfRIBAfterTheSync`, `TestFailedEndOfRIBWriteHandsBackOnlyAnOwnedClaim` | Done | `reactor_api_forward_test.go` | red under breaks (a2302b8968 body) |
| `TestInitialSyncPanicEndsTheSession` and the unwind wire assertions | Done | `peer_initial_sync_unwind_test.go` | judged |
| the five readers of `apiSyncReady` | Done | four test files | judged |
| the three suppression tests | Done | `reactor_api_forward_test.go`, `peer_initial_sync_test.go` | judged |
| the five `peer_sync_owed_test.go` tests, `TestQuiesceErrorCarriesWhatTheSubsystemOwes` | Done | `peer_sync_owed_test.go`, `quiesce_test.go` | |
| functional tests of the plan | Done | `test/plugin/`, `test/reload/`, `test/exabgp-compat/` | stress 80/80 each set |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| every file in Files to Modify and its corrections | Done | the eleven commits named in the Hand-back record and the Implementation Summary |
| `test/plugin/initial-sync-marker-beside-process-route.ci`, `test/plugin/api-eor-during-initial-sync.ci` | Done | 0461489d41 |
| the `holdWrites` comment in `session_write.go` | Changed | already corrected by 178cffd97d; this spec added only the discard call |

### Audit Summary
- **Total items:** 30
- **Done:** 27
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 3 (AC-2, the `announceEOR` suppression, the `holdWrites` comment), each in Deviations or Files to Modify

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| A route-creating process is a peer for the marker: Ze's End-of-RIB waits for none | functional | `test/plugin/initial-sync-marker-beside-process-route.ci`: the peer requires the process's route and Ze's marker in one `seq` group; 80/80 under the plugin stress pass (state file, 17:05) |
| No test asserts an order the product does not promise (the four named tests and their kin) | functional | the eight race files and three reload stages scripts (0461489d41), judged by independent test judges; plugin stress 80/80 and reload stress 80/80 |
| A marker a process asks for is never dropped, and follows the routes queued for the peer | interop + functional | `bgp-manual-eor-requested-frr`: FRR 10.7.1 logs the route, then exactly one End-of-RIB; red at assertion 7 with the drop restored (image `sha256:883e8bd4bf3b...`). `api-eor-during-initial-sync.ci` red against the old drop ("ze never sent its End-of-RIB to peer1"), green on a2302b8968 |
| `manual-eor` withholds every automatic marker | interop + unit | the same scenario's session 1: no End-of-RIB through 10 s; red at assertion 3 with the sync's marker forced (image `sha256:fe2dd16c5cfc...`). The replay form: `TestManualEORPeerWithholdsAReplayEndOfRIBAfterTheSync`, red with the stand-down removed |
| No Established peer is left with a queue nothing drains after a sync panic | unit (fault injection) | `TestInitialSyncPanicEndsTheSession`: the peer leaves Established, the Cease is on the wire, the queue is kept; red before 2cd7ac9f09 |
| Nobody reads that the marker waits for a session-ready report | search | the AC-8 search in the Hand-back record, row 2; the last page sentence corrected in this closure |
| `request quiesce` does not report a peer settled while its replay is still passing | unit + functional | `TestPendingSyncWaitsForTheReplayFence` and siblings; `rfc7999-blackhole-readvertise-agreement-forked.ci`, which failed on that race, passed 80/80 |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| A forward-worker batch panic keeps the session and leaves a hole in what the peer was sent (`fwdPool.safeBatchHandle`) | walked into, a defect of another recovery; journal row 2026-11-14 in `plan/journal/silent-fall-through.md` | `plan/release/fixes/spec-bgp-one-off-defects.md` |
| A replay-fence owner that never reports keeps the fence up and now times a quiesce out (`Peer.lowerReplayFence`) | walked into; journal row 2026-11-14 in `plan/journal/wait-outlives-the-process-it-waits-on.md` | `plan/release/fixes/spec-bgp-one-off-defects.md` |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/eor-treats-route-processes-as-peers-ec7d79dc-7da3-4803-8c26-086b8ea54170.md` (5 files: the spec, `docs/architecture/testing/interop.md`, `docs/architecture/api/architecture.md`, the news fragment, the learned summary; the code landed in the eleven commits this review read) |
| `./le spec review check` | clean, run after the final spec edit |
| Rounds | 2: round 1 over the eleven commits' diff and this closure's edits; round 2 over round 1's one fix (a page sentence) |
| Reviewer lenses used | wiring + functional coverage, logic + concurrency (lock order of `syncOwed`, `endSessionAfterSyncPanic`, `discardInterruptedWrite` under `writeMu`), docs drift + RFC comments, interop validity, style pass |

Round 1, independent reader (the second closure agent, which wrote none of
the code). Pre-checks: `./le repo check` reports seven findings, none in a
file of this work (`session_write_mutex.go` is the write-wedge session's,
the rest are under `internal/le/`); `./le test weakened check base
origin/main`: clean. Style pass run over every changed Go file: no `panic()`
added; the forward queue's existing bound covers repeated requests; the new
`peerSyncOwed` zero value is unspecified and never an answer; the new
lifecycle function `endSessionAfterSyncPanic` states its MUST (p.mu not
held).

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | Step 6 of the session-ready walk says "The last expected report closes `apiSyncReady`", a channel D-4 deleted (AC-8) | `docs/architecture/api/architecture.md`, step 6 | rewritten to say the report is credited, nothing waits on the set, and the last fence owner lowers the fence (`Peer.creditAPIReady`, `Peer.lowerReplayFence`) |

NOTEs, not blocking:

| # | Note | Location |
|---|------|----------|
| N-1 | The comment above the replay marker says the engine sends it "only when this session has not already carried the family's marker"; since D-2 it is also withheld on a `manual-eor` peer and while the sync owes its marker. The sentence stays true as a necessary condition, and the declaration it relies on states the full list | `routeServer.sendEOR` (`internal/component/bgp/plugins/rs/server_handlers.go`); `ReplayEORMeta` (`internal/component/bgp/types/types.go`) |
| N-2 | The route-server fast path releases the write path with a plain unlock, so `discardInterruptedWrite` does not run there | owned by the open row in `plan/journal/silent-fall-through.md` |
| N-3 | AC-7's next-session drain has no two-session test; it rests on the existing `sendInitialRoutes` opQueue drain | `peer_initial_sync.go` |
| N-4 | `request quiesce` under sustained forwarding can time out; its error names the peer and the reason | `peerSyncOwedError` |
| N-5 | A-4's upstream lines were read by a source-check agent, not by the main thread | A-4 |

Round 2, over the fix of finding 1 only: the two symbols it names exist in
`peer.go`, and the sentence matches `Peer.creditAPIReady`, which records the
credit and lowers the fence through `lowerReplayFence`. 0 BLOCKER, 0 ISSUE.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `test/plugin/initial-sync-marker-beside-process-route.ci` | Yes | `ls`: 3.9K |
| `test/plugin/api-eor-during-initial-sync.ci` | Yes | `ls`: 3.7K |
| `test/exabgp-compat/api/api-manual-eor.ci` | Yes | `ls`: 118 bytes |
| `test/interop/scenarios/bgp-manual-eor-requested-frr/ze.conf` | Yes | `ls`: 1.8K |
| `internal/component/bgp/reactor/session_write_interrupted.go`, `peer_sync_owed.go` | Yes | `ls`: 2.4K, 6.5K |
| `internal/le/interoplab/bgp/check_manual_eor.go` | Yes | `ls`: 15K |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | the marker waits for no process | `TestInitialSyncMarkerWaitsForNoProcess` exists in `peer_initial_sync_test.go`; the `.ci` expects the marker and the route both at `seq=1` |
| AC-5 | a requested marker follows the queued route | in `announceEOR` the queue-or-owed branch stands down only `once` (and a reactor with no pool); `api-eor-during-initial-sync.ci` expects the route at `seq=1` and the marker at `seq=2` |
| AC-6 | no automatic marker on `manual-eor` | `announceEOR` tests `once && peer.settings.ManualEOR`; `sendInitialRoutes` sets `sendEOR := !p.settings.ManualEOR` |
| AC-7 | the session ends after a sync panic | `endSessionAfterSyncPanic` calls `sessionTeardown` with `ceaseNotice(message.NotifyCeaseUnspecific, "")`; `TestInitialSyncPanicEndsTheSession` exists in `peer_initial_sync_unwind_test.go` |
| AC-8, AC-9 | no text says the marker waits for the report; the channel is gone | search over `docs/ internal/ pkg/ test/`: only true hits remain (Hand-back record, row 2) |
| AC-10 | both owned rows closed | each row's Fix cell starts `fixed:`, naming a2302b8968, and 391fcee8d1 with 9c93f82bb4 |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| a session with a route-pushing process | `test/plugin/initial-sync-marker-beside-process-route.ci` | read: fixture `markerBesideProcessRoute` pushes through `UpdateRoute` with no wait; the peer expects both frames in one group |
| `update text nlri eor` during the sync | `test/plugin/api-eor-during-initial-sync.ci` | read: fixture `fixture10EORDuringInitialSync` asks from inside the peer-UP handler; the `manual-eor` peer expects the route, then the marker |
| the bridge's `announce eor` on a `manual-eor` peer | `test/exabgp-compat/api/api-manual-eor.ci` | proves the marker arrives whenever the request lands (Wiring Test row 3) |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | owner at the research gate |
| A-2 | broken | owner at the research gate; Mistake Log row 1, Deviations |
| A-3 | confirmed | 71 files read one by one; the 43 held re-read in phase 5 |
| A-4 | confirmed | FRR 10.7.1 `bgp_update_receive_eor`, BIRD 3.4.0 `bgp_rx_end_mark`, quoted by a source-check agent |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| `manual-eor` withholds every automatic marker (`docs/guide/configuration.md`) | the `ManualEOR` tests in `announceEOR` and `sendInitialRoutes` | Yes |
| A requested marker is never dropped (`docs/exabgp/exabgp-differences.md`) | `announceEOR`, the explicit form joins `queueEndOfRIB` | Yes |
| The session-ready report lowers the fence, nothing waits on it (`architecture.md` step 6, `protocol.md`, `types.go`) | `Peer.creditAPIReady`, `lowerReplayFence` | Yes |
| What `bgp-peer-sync` waits for (`commands.md`) | `Peer.syncOwed` | Yes |
| The interop inventory row records the runs (`interop.md` row 69) | state file, "CLOSURE BLOCKER CLEARED" | Yes |
| Wire format (No) | the marker's bytes are unchanged: `message.BuildEOR` is not in the diff | Yes |
