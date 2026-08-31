# Spec: fixit-peer-pending-sync-settles-too-early

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Depends | - |
| Phase | 1/1 |
| Deferral shard | `plan/deferrals/ad-hoc-2026-08-08-ci-31225029268.md` |
| Updated | 2026-08-30 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task RE-CUT 2026-08-28, read this before the history below

**The Task below is kept for its measurements and MUST NOT be implemented as
written.** Its stated root cause is now contradicted by the producer. Every
statement here was read at the producing function on 2026-08-28.

### The half this spec was named for is FIXED

`(*Peer).setState` (`internal/component/bgp/reactor/peer.go`) stores
`sendingInitialRoutes = 1` in the same call that publishes Established, so
`pendingSync` is true the instant any observer can see the peer up. A CONNECTING
peer with a momentarily empty queue no longer reports settled, and `pendingSync`'s
own doc comment now records that as a deliberate decision rather than an
accident. Do not carry the old root cause forward.

### The half that is LIVE is a wire-visible RFC 4724 violation

It is the `apiSyncExpected` hold in `sendInitialRoutes`
(`internal/component/bgp/reactor/peer_initial_sync.go`), and the code already
carries the analysis under a `KNOWN DEFECT` heading. `RFC4724-4-1` is the
obligation: *"The End-of-RIB marker MUST be sent by a BGP speaker to its peer
once it completes the initial routing update for an address family"*
(Section 4). Two failures, in opposite directions:

| Direction | Producer | What the peer sees |
|-----------|----------|--------------------|
| OVERCOUNT | `resetAPISync` counts every binding carrying `send [ update ]`, but only `bgp-rib` ever emits `request peer <addr> plugin session ready` (`(*Reactor).SignalPeerAPIReady`) | Any other permitted plugin leaves the count unreachable, so `waitForAPISync` runs the full `apiSyncTimeout` of 2s. The marker is sent 2s AFTER the initial update completed, which is not "once it completes". Worse, an event-driven announce raised inside that window drains BEFORE the marker, so the marker then claims a route that was never part of the initial routing update |
| UNDERCOUNT | `ze-bgp:peer-raw` is gated on ATTACHMENT alone (`rawOrigin`), because it carries a message of the caller's choosing | A hand-built UPDATE injected from a bare `attach process X { }` binding lands on the peer from a binding `MaySend(SendUpdate)` reports false for, so the count skips it and it sits outside the barrier entirely |

### Why the obvious fix is banned, and what the shape must be

Widening the condition to "any process binding" is NOT the fix. The hold keeps
`sendingInitialRoutes` non-zero, so it also widens the window in which
`shouldQueue()` is true, and the forwarding rail does not consult `shouldQueue`.
Measured 2026-08-08 with a 500ms hold:
`test/plugin/role-otc-rs-withdraw-eor.ci` delivers the same relayed route to the
destination peer TWICE.

The shape the code names, and the one to design against: separate **"initial
sync running"**, which gates queueing, from **"End-of-RIB not yet sent"**, which
gates the marker. They are one flag today and they are two facts.

### Two owner decisions, both open

| # | Decision | Why it is his |
|---|----------|---------------|
| 1 | Does a plugin-INJECTED route belong to this speaker's initial dump? | The ruling quoted in `test/plugin/plugin-nexthop.ci` (Thomas, 2026-07-27) is VOID by default under `ai/rules/rfc-compliance.md` and must be re-raised, not cited. It decides whether mup4, ipv4 and ipv6 keep their seq-2 marker rule |
| 2 | Should the send vocabulary gain a word for `raw`? | The `KNOWN DEFECT` comment states outright that this is an owner decision. Without it the undercount cannot be closed by counting alone |

### Both decisions ANSWERED, Thomas, 2026-08-30

Put to him as two "which way" questions naming the cost of each side. Both
answers choose the fuller reading of RFC 4724 Section 4. This replaces the void
2026-07-27 ruling quoted in `test/plugin/plugin-nexthop.ci`, which
`ai/rules/rfc-compliance.md` forbids citing.

| # | Ruling | What it obliges |
|---|--------|-----------------|
| 1 | **A plugin-injected route DOES belong to this speaker's initial routing update.** The marker waits for it | The barrier covers every route-pushing binding, not only those carrying `send [ update ]`. The flag split below is therefore mandatory rather than optional |
| 2 | **The send vocabulary GAINS a word for `raw`.** A binding that may put a hand-built UPDATE on the wire declares it | `ze-bgp:peer-raw` stops being gated on attachment alone, so a raw-capable binding becomes countable and the undercount closes by counting, the way every other send permission already works |

**The flag split is the shape, and it is not negotiable under ruling 1.**
`sendingInitialRoutes` carries two facts today: whether the initial sync is
running, which gates `shouldQueue()`, and whether the End-of-RIB marker is still
owed, which gates the marker. Widening the single flag was measured on
2026-08-08 to deliver the same relayed route TWICE in
`test/plugin/role-otc-rs-withdraw-eor.ci`, because the forwarding rail does not
consult `shouldQueue`. Split the two facts; do not widen the one flag.

**The seq-2 marker rule in the mup4, ipv4 and ipv6 tests changes under ruling 1.**
Those fixtures encode the old answer, that an injected route follows the marker.
They now expect the marker AFTER the injected route. That is a fixture
correction, not a weakening: the tests were asserting the behaviour the ruling
overturns.

**CORRECTED 2026-08-30 at the drivers: no wire assertion changes in those three.**
The paragraph above was written from the `.ci` files alone and the drivers
refuse it. `announceWithdraw09`
(`internal/test/fixture/plugin_fixture_09.go`) announces without waiting, and
`ipv4-announce-withdraw.ci` and `ipv6-announce-withdraw.ci` already expect
announce, then marker, then withdraw, which IS ruling 1's order.
`fixture10MUPIPv4` (`internal/test/fixture/plugin_fixture_10.go`) calls
`fixture10WaitEOR` BEFORE it injects, so mup4's marker-first order is the
driver's own sequencing and says nothing about what the daemon owes. What the
two announce-withdraw fixtures did carry was a false ANNOTATION, a
`cmd=api ... eor` line naming a command no driver ever issues; that line is
deleted and every `expect=` survives.

## Task (2026-08-08, superseded above)

`test/plugin/mup-ipv4-announce.ci` and `test/plugin/ipv6-announce-withdraw.ci` fail under CPU
oversubscription because a plugin's `quiesce()` returns before the peer's
initial sync is owed, so a withdraw joins the same initial dump and overtakes
the End-of-RIB marker.

**Measured, not inferred.** Under load the received frames were announce,
withdraw, and NO marker at all, failing as `Expected UPDATE (len=23), Received
UPDATE (len=27) WITHDRAWN`. An earlier reading of this failure had the marker
overtaking the announce; that reading is wrong and should not be carried
forward.

`Peer.PendingSync` (`internal/component/bgp/reactor/peer.go`) reports settled on
`sendingInitialRoutes` being zero and an empty operation queue, and
`DrainPeerSync` (`internal/component/bgp/reactor/reactor_api.go`) calls the peer
settled on that. A peer that is still CONNECTING, with a momentarily empty
queue, is neither down nor idle yet reports settled.

**A second, related defect is BLOCKED and must not be fixed here.**
`sendInitialRoutes` (`internal/component/bgp/reactor/peer_initial_sync.go`)
gates its hold on `apiSyncExpected`, which counts only process bindings
carrying `send [ update ]`. Nothing on the injection path reads that leaf-list,
so the bare `process X { }` idiom used by mup4, ipv4, ipv6, announce and
nexthop injects routes with no hold at all. That is a guard whose condition is
unrelated to its hazard. RFC 4724 Section 4 requires the marker to be sent once
the speaker completes its initial routing update, and a route handed to ze
before establishment belongs to that update, so a route-pushing plugin must sit
inside the barrier.

That fix was implemented, proven RED to GREEN at the owning layer, and then
REVERTED: any hold inside `sendInitialRoutes` keeps `sendingInitialRoutes`
non-zero, which widens the window where `ShouldQueue` is true, and the
forwarding rail did not consult `ShouldQueue` when this was written
(`spec-fixit-forward-rail-initial-sync-ordering`, closed 2026-08-11: both
forwarding rails now hold on `Peer.forwardOrderHold`, and the pool worker holds
its overflow with `overflowHeld`). Measured A/B on
identical builds differing only in that gate:
`test/plugin/role-otc-rs-withdraw-eor.ci` passes with the gate off and fails
with it on, delivering the same relayed route twice. Separating "initial sync
running" from "End-of-RIB not yet sent" is the shape that unblocks it.

**A ruling must be adjudicated before either half lands.**
`test/plugin/plugin-nexthop.ci` quotes Thomas, 2026-07-27: the marker is ordered only
against this speaker's own initial dump, never against routes learned
afterwards. That was applied to a PLUGIN-INJECTED route by analogy with
`role-otc-unicast-scope.ci`, where the route is FORWARDED from another peer.
The masked-verdict-and-RFC-exemption record says of that same shape
that the pattern alone is not the defect, the route's provenance is. Ze's own
design treats plugin-injected initial routes as part of the dump. Under
`ai/rules/rfc-compliance.md` that ruling is void by default and must be
re-raised rather than cited. It decides whether mup4, ipv4 and ipv6 keep their
seq-2 marker rule at all.

**Already fixed and NOT part of this spec:** the test matcher that hid this
failure. `ExpectedOrKeepalive` (`internal/test/peer/checker.go`) silently
consumed a marker that any later expectation still matched, so the failure was
reported against the wrong pair of frames. That is landed and proven.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/config/syntax.md` - the send permission and what a truthful one moves
  -> Decision: the send list is the barrier's population, so `raw` becoming a word makes the
     raw rail countable without any new declaration mechanism.
- [ ] `docs/architecture/api/architecture.md` - the ten rails and the one filter they share
  -> Constraint: `filterPermittedPeers` is the ONE place the permission is applied, so raw
     changes its origin and nothing else.

**Key insights:** (minimal context to resume after compaction)
- The barrier's count and the barrier's SIGNAL are two populations. Counting is config-derived
  (`ProcessBinding.MayPushRoutes`); signalling is a plugin behaviour
  (`request peer <addr> plugin session ready`). A counted plugin that never signals costs the
  peer the whole `apiSyncTimeout`, which is the OVERCOUNT.
- `bgp-rib` and `bgp-watchdog` signalled; `bgp-rs` and `bgp-adj-rib-in` did not.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/reactor/peer.go` - pendingSync, shouldQueue, forwardOrderHold, setState
- [ ] `internal/component/bgp/reactor/reactor_api.go` - DrainPeerSync, SendRawMessage
- [ ] `internal/component/bgp/reactor/peer_initial_sync.go` - sendInitialRoutes and the apiSyncExpected gate
- [ ] `internal/component/bgp/reactor/send_permission.go` - sendOrigin, maySend, filterPermittedPeers
- [ ] `internal/component/bgp/plugins/rs/server_handlers.go` - replayForPeer, sendEOR
- [ ] `internal/component/bgp/plugins/adj_rib_in/rib.go` - handleState, handleStructuredState

**Behavior to preserve:**
- The landed matcher fix in `internal/test/peer/checker.go`. This spec does not revisit it.
- `test/plugin/role-otc-rs-withdraw-eor.ci` must keep passing; it is the regression that reverted the barrier fix.
- `request quiesce` keeps meaning "this peer's initial update AND its marker are on the wire".

**Behavior to change:**
- `sendingInitialRoutes` answers ONE question (is the sync goroutine still writing what it owns).
  `initialSyncEOROwed` answers the other (is the marker still owed).
- The barrier's population is every route-pushing binding, by either rail.
- `ze-bgp:peer-raw` is gated on `send [ raw ]`.
- Every ze plugin the barrier can count signals when its peer-up work is done.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
A BGP session reaching Established. `(*Peer).setState` closes the queueing gate and marks the
End-of-RIB owed; the FSM callback in `peer_run.go` counts the route-pushing bindings and spawns
`sendInitialRoutes`.

### Transformation Path
`setState(Established)` -> `resetAPISync(count of MayPushRoutes bindings)` -> `sendInitialRoutes`:
config static routes, default-originate, peer-up barrier, opQueue drain, family routes ->
`drainAndCloseQueueGate` (queueing gate shuts) -> `waitForAPISync` (marker still owed) ->
End-of-RIB per negotiated family -> `initialSyncEOROwed` cleared.

### Boundaries Crossed

| From | To | What crosses |
|------|----|--------------|
| peer config (`attach process`) | reactor | the send list, read as `MayPushRoutes` |
| plugin (bgp-rib, bgp-watchdog, bgp-rs, bgp-adj-rib-in) | reactor | `request peer <addr> plugin session ready` |
| reactor | peer socket | the routes, then the End-of-RIB marker |

### Integration Points
`ze-bgp:peer-raw` (`plugins/cmd/raw`), `filterPermittedPeers` (the one send guard), the YANG
`send` leaf-list validator, and `AnnounceEOR`'s deferral to the initial sync.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| `peer <addr> raw hex ...` from a `send [ raw ]` binding | -> | `rawOrigin`, `MayPushRoutes`, `waitForAPISync` | `test/plugin/initial-sync-barrier-raw.ci` |
| `peer <addr> raw hex ...` from a `send [ update ]` binding | -> | `Peer.maySend` | `TestPeerRawEntryPointNeedsTheRawSendWord` |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates |
|------|------|-----------|
| `TestInitialSyncClosesTheQueueGateBeforeItWaitsForRoutePushingPlugins` | `peer_initial_sync_test.go` | inside the barrier: marker owed, queueing gate shut, forward rails free |
| `TestInitialSyncSuppressesAnotherProducersEORWhileItsOwnIsOwed` | `peer_initial_sync_test.go` | AnnounceEOR does not overtake the marker the sync owes |
| `TestRoutePushingBindingsCountBothRails` | `peer_initial_sync_test.go` | the barrier population reads `update`, `raw` and the wildcard |
| `TestPeerRawEntryPointNeedsTheRawSendWord` | `send_permission_rails_test.go` | attachment alone no longer permits a raw injection |
| `TestPeersSynced_NotSyncedForJustEstablishedPeer` | `peer_established_gate_test.go` | quiesce still waits for the marker across the split |

### Functional Tests
- [ ] `test/plugin/initial-sync-barrier-raw.ci` -- the marker waits for a raw-injected route
- [ ] `test/plugin/role-otc-rs-withdraw-eor.ci` must not regress
- [ ] `test/plugin/mup-ipv4-announce.ci`, `ipv4-announce-withdraw.ci`, `ipv6-announce-withdraw.ci` unchanged

## Files to Modify

| File | Change |
|------|--------|
| `internal/core/bgp/events/events.go` | `SendRaw`, in `BaseSendTypes` |
| `internal/component/bgp/reactor/send_permission.go` | `rawOrigin` gated on `send [ raw ]`; `attachOnly` and `sendTypeRaw` deleted |
| `internal/component/bgp/reactor/peer_settings.go` | `ProcessBinding.MayPushRoutes` |
| `internal/component/bgp/reactor/peer_run.go` | the barrier count reads `MayPushRoutes`; teardown clears both facts |
| `internal/component/bgp/reactor/peer.go` | `initialSyncEOROwed`; `setState`, `pendingSync` |
| `internal/component/bgp/reactor/peer_initial_sync.go` | the barrier moves after `drainAndCloseQueueGate` |
| `internal/component/bgp/reactor/reactor_api_forward.go` | `AnnounceEOR` defers on the marker fact |
| `internal/component/bgp/plugins/rs/server_handlers.go` | signal readiness when the replay terminates |
| `internal/component/bgp/plugins/adj_rib_in/rib.go` | signal readiness on every peer-up |
| `internal/component/bgp/yang/ze-bgp-conf.yang` | the `send` leaf-list description |
| `internal/component/bgp/config/peers.go` | `validatePeerProcessCaps` reads `MayPushRoutes`, so a `send [ raw ]`-only process satisfies route-refresh and graceful-restart |
| `internal/component/bgp/config/loader_test.go` | `TestRouteRefreshAcceptsARawOnlyProcess` drives that validator from `LoadReactor` |
| `test/plugin/ipv4-announce-withdraw.ci`, `ipv6-announce-withdraw.ci` | the false `cmd=api ... eor` annotation is deleted; every `expect=` survives |
| `test/plugin/plugin-nexthop.ci` | the void 2026-07-27 quote is replaced by the 2026-08-30 ruling |
| `docs/architecture/config/syntax.md` | the send-type enum table and the "two BASE types" sentence carry `raw` |
| `docs/architecture/api/architecture.md` | the API Sync Protocol section: `MayPushRoutes`, 2s, the drain before the wait, the `initialSyncEOROwed` guard |

## Implementation Steps

1. Reproduce, then name the root cause at its producing function.
2. Fix at the owning layer, never at the symptom.
3. Prove the fix discriminates: red before, green after.

## Goal Validation (BLOCKING)

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| The End-of-RIB waits for every route-pushing binding, by either rail (RFC 4724 Section 4, ruling 1) | functional test with a red phase | `test/plugin/initial-sync-barrier-raw.ci`: a `send [ raw ]`-only binding injects, dispatches `plugin session ready`, and the marker follows. Red proven by deleting the `SendRaw` arm of `MayPushRoutes` |
| Holding the marker does NOT widen the queueing window | unit test at the entry point | `TestInitialSyncClosesTheQueueGateBeforeItWaitsForRoutePushingPlugins`: `shouldQueue()` is false INSIDE the wait, which one flag made impossible |
| No regression on the measurement that reverted the first attempt | functional test | `test/plugin/role-otc-rs-withdraw-eor.ci` PASS 4/4; it delivered the relayed route twice under the 2026-08-08 single-flag hold |
| Another producer's marker does not overtake the one the sync owes | unit test | `TestInitialSyncSuppressesAnotherProducersEORWhileItsOwnIsOwed`; `AnnounceEOR` defers on `initialSyncEOROwed` (`reactor_api_forward.go`) |
| The send vocabulary gains `raw`, and attachment alone no longer permits an injection (ruling 2) | unit test at the entry point | `TestPeerRawEntryPointNeedsTheRawSendWord` drives `peer <addr> raw`, and the `send [ update ]` binding is REFUSED |
| `request quiesce` still means "this peer's initial update AND its marker are on the wire" | unit test | `TestPeersSynced_NotSyncedForJustEstablishedPeer`; `pendingSync` reads all three facts (`peer.go`) |
| A config the new word makes writable is accepted | unit test with a red phase | `TestRouteRefreshAcceptsARawOnlyProcess`; RED against `MaySend(SendUpdate)`, GREEN against `MayPushRoutes` |

Interop: no new wire format and no new message. The change moves WHEN an existing
End-of-RIB marker is written, and `RFC4724-4-1`'s interop row is already served by
`checkNoFamilyEndOfRIB` (`internal/le/interoplab/bgp/check_rfc.go`), which a
conforming FRR receiver decodes.

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/fixit-peer-pending-sync-settles-too-early-5286b836-8702-47bc-a370-6034e573c3d9.md` |
| `./le spec session review check` | clean |
| Rounds | 2. Run 1 by the implementing session's reviewer over `fc9c8bcaa` and `267fb88e9`; Run 2 by the closure context over the committed diff of `fc9c8bcaa`, `267fb88e9` and `01d0474da`, plus the fixes Run 1 produced |
| Reviewer lenses used | wiring and registration, removed-behavior audit, guard audit, logic and concurrency, documentation drift, RFC 4724 conformance, ze-go-style pass, simplicity |

### Run 1 (initial)

Independent review, separate context, over HEAD after `fc9c8bcaa` and `267fb88e9`.

| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | ISSUE | `validatePeerProcessCaps` still requires `MaySend(SendUpdate)`, so a `send [ raw ]`-only process is refused on a peer with graceful-restart or route-refresh. The `raw` word made that config writable, so the change reaches this guard | `internal/component/bgp/config/peers.go` `validatePeerProcessCaps` | fixed: reads `MayPushRoutes`, with `TestRouteRefreshAcceptsARawOnlyProcess` proven red first |
| 2 | ISSUE | The send-type enum table says there are two base types where `BaseSendTypes` now returns three | `docs/architecture/config/syntax.md` | fixed |
| 3 | ISSUE | The API Sync Protocol section carries three wrong claims: it counts `SendUpdate`, waits 500ms, and guards `AnnounceEOR` on `ShouldQueue()` alone. The sample code shows a sleep the reactor does not do | `docs/architecture/api/architecture.md` | fixed |
| 4 | ISSUE | `test/plugin/initial-sync-barrier-raw.ci` had never executed, so its discrimination was unproven | `test/plugin/initial-sync-barrier-raw.ci` | fixed: run, and its red phase forced and recorded |
| 5 | ISSUE | The void 2026-07-27 ruling is still quoted as the reason this fixture asserts no marker position | `test/plugin/plugin-nexthop.ci` | fixed: replaced by the 2026-08-30 ruling and by the reason the file owns |
| 6 | NOTE | Two fixtures carry a `cmd=api ... eor` annotation naming a command their driver never issues | `test/plugin/ipv4-announce-withdraw.ci`, `test/plugin/ipv6-announce-withdraw.ci` | fixed: annotation deleted, every `expect=` kept |
| 7 | NOTE | `TestRoutePushingBindingsCountBothRails` drives `MayPushRoutes` as a helper, not from the FSM entry point. Nothing in Go proves the count loop reads it | `peer_initial_sync_test.go` | acknowledged: `initial-sync-barrier-raw.ci` is the entry-point proof, and its red phase is now on record |
| 8 | NOTE | The re-cut said the mup4, ipv4 and ipv6 fixtures owed a wire correction. The drivers refuse it | this spec, re-cut section | fixed: the spec records the correction and names the drivers |

### Fixes applied

- `validatePeerProcessCaps` counts both rails, its refusal names both words, and a test drives it from `LoadReactor`.
- The two stale documentation blocks agree with `BaseSendTypes`, `apiSyncTimeout`, `peer_run.go` and `reactor_api_forward.go`.
- `initial-sync-barrier-raw.ci` executed, its red phase forced by deleting the `SendRaw` arm.
- The three `.ci` header comments state what their drivers do.

### Run 2 (closure, over the committed diff)

Independent of the context that wrote the code: `/ze-implement` produced the
diff and ended, and Run 1 ran in the implementing session. This run read the
diff of `fc9c8bcaa`, `267fb88e9` and `01d0474da` from git, and the current
producers from source. `./le repository check` passed. `./le commit audit`
reported clean at base HEAD.

| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 9 | ISSUE | Dead control flow in the extracted drain. `if !isRouteScopedSendError(sendErr) { break }` sits inside a `switch` inside the `for`, so the `break` leaves the SWITCH and lands where `continue` lands. Both arms are the same, and the comment added with the extraction explains the `break` rather than deleting it. Present in the announce case and the withdraw case | `internal/component/bgp/reactor/peer_initial_sync.go` `drainAndCloseQueueGate` | fixed: both conditionals deleted, one comment states why this drain attempts every remaining operation and names the main drain loop that does not |
| 10 | ISSUE | `docs/architecture/api/capability-contract.md` quotes both `validatePeerProcessCaps` refusals verbatim, and `01d0474da` rewrote both. The page also says `send [ update ]` "is the permission", which is now either rail | `docs/architecture/api/capability-contract.md`, Config Validation | fixed: both strings match the producer, the either-rail sentence names `MayPushRoutes`, and the page gains the two source anchors it never carried |
| 11 | ISSUE | The change newly proves RFC 4724 Section 4 for a plugin-injected route, and the public status row does not say so | `docs/features/rfc-status.md`, the RFC 4724 row | written, not committed here: the row names `setState`, `sendInitialRoutes`, `ProcessBinding.MayPushRoutes` and `test/plugin/initial-sync-barrier-raw.ci` with its red phase, and the edit sits in the working tree. A concurrent RFC session holds a second, unrelated hunk in that file, so committing it would carry their work under this message and split their change across three files |
| 12 | ISSUE | The spec's Goal Validation cited `checkNoFamilyEndOfRIB` as this change's interop evidence. That checker cannot pass: it calls `checkScenario` with its own scenario name, and `scenarioOperations` (`internal/le/interoplab/bgp/checkers.go`) has no such key, which `checkScenario` answers with `scenario ... has no typed assertions`. The same shape holds for 14 other RFC checkers, and `bgp_test.go:53` forbids the entry that would fix it | `internal/le/interoplab/bgp/check_rfc.go` `checkNoFamilyEndOfRIB`, `check_engine.go` `checkScenario` | recorded, not fixed here: it is a defect in the interop harness that a 2026-08-27 tooling migration introduced, it spans 15 checkers across four protocols, and this spec's goal does not depend on it. Row in `plan/journal/gate-red-where-nothing-blocks-on-it.md`. The Goal Validation note below states what the citation is worth today |
| 13 | NOTE | `AdjRIBInManager.signalSessionReady` returns silently when `r.plugin` is nil, and no other call site in the file guards it | `internal/component/bgp/plugins/adj_rib_in/rib.go` | fixed: the guard states that production always sets the handle and that the sibling replay path needs no guard because it returns early with no routes |
| 14 | NOTE | `sendInitialRoutes` keeps the `session` pointer across `waitForAPISync` and calls `HoldWrites` on it afterwards. A session torn down inside the wait makes `SendUpdateHeld` return `ErrInvalidState`, which the marker loop already handles and logs | `internal/component/bgp/reactor/peer_initial_sync.go` | acknowledged, no change: the pre-split code held the same pointer across the same wait, and the failure is handled rather than silent |
| 15 | NOTE | Nothing joins the `sendInitialRoutes` goroutine, so a goroutine belonging to a torn-down session can clear `initialSyncEOROwed` after the successor session set it. The same shape already existed on `sendingInitialRoutes`, which the old code cleared at the same point | `internal/component/bgp/reactor/peer_initial_sync.go`, `peer_run.go` `runOnce` | recorded, not fixed: pre-existing in shape, needs a session generation the flags do not carry, and it does not block this spec's goal. Row in `plan/journal/late-write-lands-on-the-successor.md` |

### Fixes applied (Run 2)

- `drainAndCloseQueueGate` has one path through a send error, and says why.
- `docs/architecture/api/capability-contract.md` agrees with
  `validatePeerProcessCaps` and carries source anchors.
- `docs/features/rfc-status.md` records the barrier under RFC 4724.
- `internal/test/fixture/register_initial_sync_barrier.go` has its `// Design:`
  header repointed from this spec, which commit B removes, to
  `docs/architecture/api/architecture.md`, which is where the barrier is now
  documented.
- `AdjRIBInManager.signalSessionReady` states why its nil guard exists.

### Final status
- [ ] Re-review shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

Run 2 found 0 BLOCKER. It found four ISSUEs and three NOTEs. Every ISSUE is
fixed except finding 12, which is a defect in the interop harness rather than in
this change, is homed in a journal row, and is answered in the Goal Validation
note below. No finding survived a fix, so no third round was owed.

### Goal Validation, interop row, corrected at closure

The Interop paragraph above says RFC4724-4-1's interop row "is already served by
`checkNoFamilyEndOfRIB`". That checker has been unable to pass since the
2026-08-27 tooling migration (finding 12), so read the sentence as: the scenario
`test/interop/scenarios/no-family-peer-eor-frr` exists, its FRR config still
enables the per-UPDATE decode the assertion reads, and the marker it was written
against is the one `sendInitialRoutes` still emits. What this change alters is
WHEN that marker is written, and no new message and no new encoding is
introduced. The ordering itself is asserted byte for byte, against a separate
BGP peer process, by `test/plugin/initial-sync-barrier-raw.ci`, whose red phase
is on record.

### Out of scope, and where it lives

The OVERCOUNT residual is NOT closed here. An external plugin that is counted and
never dispatches `request peer <addr> plugin session ready` still costs the peer
the whole `apiSyncTimeout`. `bgp-rib`, `bgp-watchdog`, `bgp-rs` and
`bgp-adj-rib-in` all signal; the test fixtures do not. The signalling contract is
`plan/spec-bgp-session-ready-contract.md`.

## Checklist

### Goal Gates (MUST pass)
- [ ] A peer that has not established is never reported settled by `quiesce()`
- [ ] The RFC 4724 ruling on plugin-injected routes is re-raised with Thomas and recorded
- [ ] No fixture was edited to match current behaviour

### TDD
- [ ] Tests written before the fix
- [ ] Tests FAIL without the fix
- [ ] Tests PASS with the fix
- [ ] `./le verify worktree` green before commit

### Deliverables Checklist

<!-- The spec was written in the abbreviated fixit shape and carried no such
     table. Filled at closure from plan/TEMPLATE.md, on the owner's instruction
     of 2026-08-31, rather than stopping on its absence. -->

| Deliverable | Verification method | Result |
|-------------|---------------------|--------|
| `send [ raw ]` is a base send type the YANG validator accepts | `grep -n SendRaw internal/core/bgp/events/events.go`, and `SendMessageValidator` (`internal/component/config/validators.go`) reads `bgpevents.BaseSendTypes()` | `BaseSendTypes` returns `update, refresh, raw`; the validator derives from it, so no second list was written |
| `ze-bgp:peer-raw` is gated on that word | `rawOrigin` (`internal/component/bgp/reactor/send_permission.go`) | returns `sendOrigin{sendType: bgpevents.SendRaw}`; `attachOnly` and `sendTypeRaw` are deleted |
| The barrier counts both rails | `ProcessBinding.MayPushRoutes` (`internal/component/bgp/reactor/peer_settings.go`), read at `peer_run.go` `runOnce` | `apiSendCount` counts `MayPushRoutes()`, not `MaySend(SendUpdate)` |
| The two facts are split | `Peer.initialSyncEOROwed` (`internal/component/bgp/reactor/peer.go`) | set by `setState`, cleared by `sendInitialRoutes`, `runOnce` teardown, the panic path and the no-caps path |
| The barrier sits AFTER the queueing gate closes | `sendInitialRoutes` (`internal/component/bgp/reactor/peer_initial_sync.go`) | `drainAndCloseQueueGate`, then `wakeForwardOverflow`, then `waitForAPISync`, then the marker loop |
| Every ze plugin the barrier counts signals readiness | `signalSessionReady` in `plugins/rs/server_handlers.go` and `plugins/adj_rib_in/rib.go` | both added; `bgp-rib` and `bgp-watchdog` already signalled |
| The functional proof exists and discriminates | `ls test/plugin/initial-sync-barrier-raw.ci` and its recorded red phase | the file exists; deleting the `SendRaw` arm of `MayPushRoutes` makes seq=2 fail with `out-of-order marker accepted in silence` |
| The capability validator accepts a raw-only process | `go test -run TestRouteRefreshAcceptsARawOnlyProcess ./internal/component/bgp/config/` | `ok ... 7.858s`, and RED against `MaySend(SendUpdate)` |

### Security Review Checklist

| Check | What to look for | Finding |
|-------|------------------|---------|
| Does the change widen who may write to a peer's socket? | `rawOrigin` and `maySend` (`internal/component/bgp/reactor/send_permission.go`) | It NARROWS it. Raw was gated on attachment alone, so any attached process could put a whole BGP message of its choosing on the wire, a forged NOTIFICATION or OPEN included. It now needs `send [ raw ]`. `TestPeerRawEntryPointNeedsTheRawSendWord` drives the refusal from the registered command |
| Does the wildcard grant more than it should? | `MaySend` (`peer_settings.go`) returns `b.SendAll \|\| b.Send[sendType]` | `send [ * ]` granted raw before this change too, because attachment alone did. No widening |
| Can a plugin hold a peer's session open? | `waitForAPISync` (`internal/component/bgp/reactor/peer.go`), `apiSyncTimeout` (`api_sync.go`) | Bounded at 2s. A plugin that never signals delays the marker by that bound and nothing more. The wait runs with the queueing gate shut, so it parks no route operation and no forwarding rail |
| Can a plugin's readiness signal be forged for another peer? | `SignalPeerAPIReady` (`internal/component/bgp/reactor/api_sync.go`) | The command names a peer address and the reactor looks it up; a miss is logged loudly rather than silently counted. The signal only RELEASES a wait, so a forged one costs ordering, never a write permission |
| Unbounded allocation on the new path? | `drainAndCloseQueueGate` (`peer_initial_sync.go`) | It drains an existing bounded `opQueue` (`opQueueMax`) and allocates through `getBuildBuf` and `putBuildBuf`. No `make` was added |
| Error leakage | `signalSessionReady` in both plugins | Logs the peer address and the transport error at Warn. Neither carries operator secrets |

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File updated |
|---|----------|----------|--------------|
| 1 | New user-facing feature? | Yes | `docs/architecture/config/syntax.md`: the send-type table gains `raw`, and "two BASE types" becomes three |
| 2 | Config syntax changed? | Yes | `internal/component/bgp/yang/ze-bgp-conf.yang` `send` description, and `docs/architecture/config/syntax.md`. `docs/config-reference.md` needed no edit: it states the auto-load rule and names no base type |
| 3 | CLI command added/changed? | No | `peer <addr> raw` is unchanged in grammar. `docs/architecture/api/update-syntax.md` already states "The peer must attach the program with `send [ raw ]`" |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/architecture.md`, the API Sync Protocol section: the count, the 2s bound, the drain before the wait, the `initialSyncEOROwed` guard, with six source anchors |
| 5 | Plugin added/changed? | Yes | `bgp-rs` and `bgp-adj-rib-in` now signal readiness. `docs/guide/plugins.md` already carries `raw` in its send-type prose, and its per-plugin table lists permissions rather than signals |
| 6 | Has a user guide page? | No | `grep -rn "plugin session ready" docs/guide/` names no page that describes the barrier |
| 7 | Wire format changed? | No | No new message and no changed encoding. Only WHEN an existing End-of-RIB marker is written |
| 8 | Plugin SDK/protocol changed? | No | `request peer <addr> plugin session ready` already existed and is unchanged. What changed is which plugins send it |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes, written, NOT in this commit | `docs/features/rfc-status.md`, the RFC 4724 row: the barrier, the predicate, and `test/plugin/initial-sync-barrier-raw.ci` with its red phase. The edit is in the working tree and this closure does NOT commit the file, because a concurrent RFC session holds an unrelated hunk in it (the RFC 1997 status word) alongside its edits to `docs/comparison.md` and `website/data/features.json`. Carrying one file of that change would split it. `rfc/requirements/rfc4724.md` already lists the two new tagged tests under `RFC4724-4-1`, regenerated at `15f5811dc` |
| 10 | Test infrastructure changed? | Yes | One new fixture, `internal/test/fixture/register_initial_sync_barrier.go`. `docs/functional-tests.md` describes the harness rather than the scenario list, so no edit is owed |
| 11 | Affects daemon comparison? | No | `grep -n "End-of-RIB" docs/comparison.md` is empty |
| 12 | Internal architecture changed? | Yes | `docs/architecture/api/architecture.md`, as row 4 |
| 13 | Route metadata keys added/changed? | No | `grep -rn "MayPushRoutes\|SendRaw" docs/architecture/meta/` is empty |
| 14 | A page the change made WRONG | Yes | `docs/architecture/api/capability-contract.md` quoted both `validatePeerProcessCaps` error strings verbatim, and both changed. Found at closure, fixed with the two source anchors it never carried |

## Implementation Summary

### What Was Implemented

- `Peer.sendingInitialRoutes` answers ONE question again (is the sync goroutine
  still writing what it owns), and `Peer.initialSyncEOROwed` answers the other
  (is the End-of-RIB still owed). `setState` sets both when it publishes
  Established. `pendingSync` reads both, so `request quiesce` still means "this
  peer's initial update AND its marker are on the wire".
- `sendInitialRoutes` closes the queueing gate through the new
  `drainAndCloseQueueGate` BEFORE it waits for the route-pushing plugins, and
  releases the forwarding rails with it. That is what makes the barrier
  widenable: the 2026-08-08 measurement that reverted the first attempt was a
  hold taken while one flag carried both facts.
- The barrier's population is `ProcessBinding.MayPushRoutes`, which reads
  `send [ update ]` and the new `send [ raw ]`.
- `raw` is a base send type. `rawOrigin` is gated on it, `attachOnly` and
  `sendTypeRaw` are deleted, and `validatePeerProcessCaps` reads the same
  predicate, so a raw-only process satisfies route-refresh and graceful-restart.
- `bgp-rs` and `bgp-adj-rib-in` dispatch `request peer <addr> plugin session
  ready`. `bgp-rs` also stopped returning early for a peer with no recorded
  family, which was one whole peer paying the timeout.
- `AnnounceEOR` defers on `initialSyncEOROwed` as well as `shouldQueue`, and
  `logEORSuppressed` names the fourth reason.

### Bugs Found/Fixed

- `validatePeerProcessCaps` refused a peer whose only route-pushing process
  declared `send [ raw ]`. Found in review Run 1, fixed, covered by
  `TestRouteRefreshAcceptsARawOnlyProcess`, proven RED against the old predicate.
- `routeServer.sendEOR` returned before signalling for a peer with no recorded
  family, so that peer waited out the whole `apiSyncTimeout`.
- The `initial-sync-barrier-raw` observer line claimed a wire order it never
  observed. It printed OK in the red run too. It now states what it saw.
- Closure Run 2 found dead control flow in `drainAndCloseQueueGate`: an
  `if !isRouteScopedSendError(sendErr)` whose `break` left the SWITCH, so both
  arms reached the same place. Deleted, with the reason the drain continues
  stated once.
- Closure Run 2 found `docs/architecture/api/capability-contract.md` quoting two
  error strings this change rewrote.

### Documentation Updates

- `docs/architecture/api/architecture.md` -- API Sync Protocol, eleven steps and
  the two-facts paragraph, six source anchors.
- `docs/architecture/config/syntax.md` -- the send-type table and the base-type
  sentence.
- `docs/architecture/api/capability-contract.md` -- both quoted refusals, the
  either-rail sentence, and two new source anchors.
- `docs/features/rfc-status.md` -- the RFC 4724 row names the barrier, the
  predicate and the functional proof. Written, and left uncommitted here: a
  concurrent session holds an unrelated hunk in that file.
- `internal/component/bgp/yang/ze-bgp-conf.yang` -- the `send` leaf-list
  description names `raw`.
- `./le doc check verify` exits 1 on 27 stale source anchors and a set of CLI
  description whitespace findings. None names a file this spec touched, and the
  two anchors this closure ADDED both resolve.

### Deviations from Plan

- The re-cut said the mup4, ipv4 and ipv6 fixtures owed a wire correction under
  ruling 1. Read at the drivers, they did not: `announceWithdraw09` announces
  without waiting, and the two announce-withdraw scenarios already expected
  announce, marker, withdraw. What they carried was a false `cmd=api ... eor`
  annotation, which is deleted. `test/weakened.md` carries the row.
- Files to Modify named `peer.go` for `initialSyncEOROwed` and `peer_run.go` for
  the count, and both landed. It did not name
  `internal/component/bgp/reactor/api_sync.go`, which took a comment correction
  (500ms became `apiSyncTimeout`), nor
  `internal/test/fixture/register_initial_sync_barrier.go`, which is the new
  fixture.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | The re-cut assumed the mup4, ipv4 and ipv6 `.ci` fixtures asserted marker-before-injected-route, so ruling 1 would force a wire correction in three files | The drivers decide that order. `fixture10MUPIPv4` waits for the marker before it injects, and `announceWithdraw09` does not wait at all, so two of the three already expected ruling 1's order and the third says nothing about what the daemon owes | Read at `internal/test/fixture/plugin_fixture_09.go` and `plugin_fixture_10.go` on 2026-08-30, after the `.ci` files alone had been read | The spec records the correction and names both drivers. No `expect=` line was changed |
| approach | The first attempt at the barrier widened `sendingInitialRoutes` itself | The flag also gates queueing and the forwarding rails, so the hold delivered the same relayed route twice in `role-otc-rs-withdraw-eor.ci` | Measured A/B on 2026-08-08 with a 500ms hold | Reverted, and the re-cut demanded the flag split instead. That split is what landed |

## Implementation Audit

### Requirements from Task

| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Ruling 1: a plugin-injected route belongs to this speaker's initial routing update | Done | `peer_run.go` `runOnce`, `ProcessBinding.MayPushRoutes` | The barrier covers every route-pushing binding |
| Ruling 2: the send vocabulary gains a word for `raw` | Done | `internal/core/bgp/events/events.go` `BaseSendTypes`, `send_permission.go` `rawOrigin` | The undercount closes by counting |
| The flag split, not a widened single flag | Done | `peer.go` `Peer.initialSyncEOROwed`, `peer_initial_sync.go` `drainAndCloseQueueGate` | `shouldQueue()` is false inside the wait |
| The OVERCOUNT half of the KNOWN DEFECT | Partial, by design | `plugins/rs/server_handlers.go`, `plugins/adj_rib_in/rib.go` | The four ze plugins signal. An EXTERNAL plugin that is counted and never signals still costs `apiSyncTimeout`. Homed at `spec-bgp-session-ready-contract`, and stated in this spec's "Out of scope" section before implementation started |

### Acceptance Criteria

<!-- The spec carries Goal Gates rather than AC-N rows. Each is audited here. -->

| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| Goal Gate 1: a peer that has not established is never reported settled by `quiesce()` | Done | `TestPeersSynced_NotSyncedForJustEstablishedPeer` (`peer_established_gate_test.go`) | It now asserts the middle state too: gate shut, marker owed, NOT settled |
| Goal Gate 2: the RFC 4724 ruling is re-raised with Thomas and recorded | Done | The "Both decisions ANSWERED, Thomas, 2026-08-30" table above, and `test/plugin/plugin-nexthop.ci` where the void quote was replaced | The 2026-07-27 ruling is no longer cited as authority anywhere |
| Goal Gate 3: no fixture was edited to match current behavior | Done | `./le commit audit` clean at base HEAD; `test/weakened.md` carries the one rename | No `expect=` line was weakened; the deleted `cmd=api` lines are annotations, not assertions |

### Tests from TDD Plan

| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestInitialSyncClosesTheQueueGateBeforeItWaitsForRoutePushingPlugins` | Done | `peer_initial_sync_test.go` | Asserts from INSIDE the wait, on `triggerClock.waiting` |
| `TestInitialSyncSuppressesAnotherProducersEORWhileItsOwnIsOwed` | Done | `peer_initial_sync_test.go` | `AnnounceEOR` writes nothing and reports handled |
| `TestRoutePushingBindingsCountBothRails` | Done | `peer_initial_sync_test.go` | Five cases: update, raw, wildcard, refresh-only, attached-only |
| `TestPeerRawEntryPointNeedsTheRawSendWord` | Done | `send_permission_rails_test.go` | Renamed from the unattached-process spelling; two negatives and one positive |
| `TestPeersSynced_NotSyncedForJustEstablishedPeer` | Done | `peer_established_gate_test.go` | Extended across the split |
| `TestRouteRefreshAcceptsARawOnlyProcess` | Done, added in review | `internal/component/bgp/config/loader_test.go` | Not in the TDD plan; Run 1 finding 1 required it |
| `test/plugin/initial-sync-barrier-raw.ci` | Done | `test/plugin/` | Run and red-phase-forced on 2026-08-30 |

### Files from Plan

| File | Status | Notes |
|------|--------|-------|
| `internal/core/bgp/events/events.go` | Done | `SendRaw` in `BaseSendTypes` |
| `internal/component/bgp/reactor/send_permission.go` | Done | `attachOnly` and `sendTypeRaw` deleted |
| `internal/component/bgp/reactor/peer_settings.go` | Done | `MayPushRoutes` |
| `internal/component/bgp/reactor/peer_run.go` | Done | The count and the teardown clear |
| `internal/component/bgp/reactor/peer.go` | Done | `initialSyncEOROwed`, `setState`, `pendingSync`, `resetAPISync` comment |
| `internal/component/bgp/reactor/peer_initial_sync.go` | Done | Barrier moved; `drainAndCloseQueueGate` extracted |
| `internal/component/bgp/reactor/reactor_api_forward.go` | Done | `AnnounceEOR` and `logEORSuppressed` |
| `internal/component/bgp/plugins/rs/server_handlers.go` | Done | `signalSessionReady`, and the family-less early return removed |
| `internal/component/bgp/plugins/adj_rib_in/rib.go` | Done | `signalSessionReady` on every peer-up |
| `internal/component/bgp/yang/ze-bgp-conf.yang` | Done | The `send` description |
| `internal/component/bgp/config/peers.go` | Done | `validatePeerProcessCaps` |
| `internal/component/bgp/config/loader_test.go` | Done | `TestRouteRefreshAcceptsARawOnlyProcess` |
| `test/plugin/ipv4-announce-withdraw.ci`, `ipv6-announce-withdraw.ci` | Done | Annotation deleted, every `expect=` kept |
| `test/plugin/plugin-nexthop.ci` | Done | Void quote replaced |
| `docs/architecture/config/syntax.md` | Done | Send-type table |
| `docs/architecture/api/architecture.md` | Done | API Sync Protocol |
| `internal/component/bgp/reactor/api_sync.go` | Done, not planned | Comment: 2.5s became `apiSyncTimeout` |
| `internal/test/fixture/register_initial_sync_barrier.go` | Done, not planned | The new fixture |
| `docs/architecture/api/capability-contract.md` | Done, added at closure | Two quoted refusals the change rewrote |
| `docs/features/rfc-status.md` | Written, not committed | The RFC 4724 row; the file is held by a concurrent session |

### Audit Summary

- **Total items:** 34 (4 requirements, 3 goal gates, 7 tests, 20 files)
- **Done:** 33
- **Partial:** 1 -- the OVERCOUNT residual for an EXTERNAL plugin, declared out
  of scope before implementation started and homed at
  `spec-bgp-session-ready-contract`
- **Skipped:** 0
- **Changed:** 4 -- three files not in the plan, and the fixture correction
  recorded under Deviations

## Deferrals Resolved

Shard: `plan/deferrals/ad-hoc-2026-08-08-ci-31225029268.md`, four rows, all
terminal. The shard is removed by commit A. No other spec cites it
(`grep -rln ad-hoc-2026-08-08-ci-31225029268 plan/` names this spec and the
shard itself).

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| `test/plugin/rib-graph.ci` never terminates | done | `spec-fixit-rib-graph-ci-never-terminates`, closed 2026-08-10. `terminateScaffoldPeers` was the fix and no timeout was raised |
| `test/plugin/forward-overflow-two-tier.ci` failed once, unattributed | done | `spec-fixit-forward-overflow-two-tier-flake`, closed 2026-08-15. 80 invocations, every one exit 0; recorded in `plan/known-failures/bgp-plugin-forward-overflow-two-tier.md` with the attempt and the next step |
| The route-server opaque withdrawal fix had unit proof only | done | `spec-fixit-bgpls-withdrawal-functional-proof`, closed 2026-08-15. `test/plugin/rfc9552-52-rs-opaque-withdraw-peer-down.ci` is the proof, measured red against a daemon with the branch removed |
| `path_problem` measures learned-staleness against the FILESYSTEM | cancelled | Both ends of the row are gone. Its holder `plan/future/spec-fixit-learned-staleness-measurement.md` was deleted as obsolete at `cfeaa7f28`, the `scripts/dev/` producer went with the journal migration at `2cff2050a`, and no dead-reference baseline survives anywhere: `grep -rn "dead-reference\|path_problem" internal/le/ --include=*.go` is empty and `internal/le/journal/validate.go` reads no baseline. A finding about a tool that no longer exists cannot recur |

## Pre-Commit Verification

### Files Exist (ls)

| File | Exists | Evidence |
|------|--------|----------|
| `test/plugin/initial-sync-barrier-raw.ci` | Yes | 87 lines added at `267fb88e9`; read in full at closure |
| `internal/test/fixture/register_initial_sync_barrier.go` | Yes | Carries `Register("plugin/initial-sync-barrier-raw", initialSyncBarrierRaw)` |
| `internal/component/bgp/config/loader_test.go` | Yes | Carries `TestRouteRefreshAcceptsARawOnlyProcess` at the file tail |
| `plan/deferrals/ad-hoc-2026-08-08-ci-31225029268.md` | Yes | Read in full at closure; removed by commit A |

### AC Verified (grep/test)

| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| Goal Gate 1 | `quiesce()` never settles a peer whose marker is owed | `go test -race -count=3` over the reactor package, filtered to the five spec tests, at closure: `ok github.com/ze-software/ze/internal/component/bgp/reactor 9.668s` |
| Goal Gate 2 | The ruling is recorded and the void one is gone | `test/plugin/plugin-nexthop.ci` names 2026-07-27 only as the ruling that was overturned |
| Goal Gate 3 | No fixture weakened | `./le commit audit` at closure: `Test-relaxation audit: clean (no unexplained test weakening). base HEAD, range 26f112aee50c..worktree, 2 changed test file(s) examined.` |
| Ruling 2, config path | A `send [ raw ]`-only process loads | `go test -race -run TestRouteRefreshAcceptsARawOnlyProcess ./internal/component/bgp/config/` at closure: `ok ... 7.858s` |
| The closure edits compile and lint | The Run 2 fixes did not break the package | `./le verify lint run scope ./internal/component/bgp/reactor/...`: `0 issues` in both flavours. `go vet` over `./internal/component/bgp/plugins/adj_rib_in/...` and `./internal/test/fixture/...`: clean. `gofmt -l` over the three edited files: empty |

### Wiring Verified (end-to-end)

| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `peer <addr> raw hex ...` from a `send [ raw ]` binding | `test/plugin/initial-sync-barrier-raw.ci` | Read in full at closure. The attach block grants `send [ raw ]` and nothing else, the fixture waits for ESTABLISHMENT rather than for the marker, injects, dispatches `plugin session ready`, and the file asserts seq=1 route then seq=2 marker by hex. Red phase on record: deleting the `SendRaw` arm of `MayPushRoutes` puts the marker at seq=1 |
| `peer <addr> raw hex ...` from a `send [ update ]` binding | none; unit at the entry point | `TestPeerRawEntryPointNeedsTheRawSendWord` dispatches through `Server.Dispatcher().Dispatch`, the registered command, and asserts `errSendNotPermitted` with an empty socket |

### Assumptions Resolved

<!-- The spec carries no A-N table. The two load-bearing assumptions its re-cut
     states are audited here instead. -->

| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1: widening the single flag delivers a relayed route twice | confirmed | Measured 2026-08-08 with a 500ms hold on `test/plugin/role-otc-rs-withdraw-eor.ci`. The split is what avoids it, and that scenario passes 4/4 with the split in place |
| A-2: the mup4, ipv4 and ipv6 fixtures encode the pre-ruling marker order | broken | The drivers decide the order, not the fixtures. Recorded in the Mistake Log and in the CORRECTED paragraph above; no `expect=` line changed |

### Documentation Verified

| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| `docs/architecture/config/syntax.md` says there are three base send types | `BaseSendTypes()` returns update, refresh and raw | Yes |
| `docs/architecture/api/architecture.md` says the barrier counts `MayPushRoutes` and waits `apiSyncTimeout` of 2s | `peer_run.go` `runOnce`, `api_sync.go` `apiSyncTimeout` | Yes |
| `docs/architecture/api/capability-contract.md` quotes the validator's refusals | `validatePeerProcessCaps` (`internal/component/bgp/config/peers.go`) | Yes, corrected at closure; both strings now match the producer |
| `docs/features/rfc-status.md` RFC 4724 row names the barrier and its proof | `peer_initial_sync.go`, `peer_settings.go`, `test/plugin/initial-sync-barrier-raw.ci` | Yes in the working tree, and NOT in this commit; see Documentation Update Checklist row 9 |
| No page still says raw is gated on attachment alone | `grep -rn "attachment alone" docs/` names one line, `docs/architecture/config/syntax.md:773`, which states the change | Yes |
| The two anchors this closure added resolve | `./le doc check verify`, source-anchor stage: 27 stale anchors, none in a file this spec touched | Yes |

## Core Insight

One flag that answers two questions is not merely confusing: it can make a
correct change IMPOSSIBLE. `sendingInitialRoutes` gated queueing and gated the
marker, so every attempt to widen the marker's barrier also widened the queueing
window, and the widened window was measurably wrong. The 2026-08-08 session read
that measurement as "the barrier cannot be widened" and reverted. The measurement
was true and the conclusion was false. What could not be widened was the FLAG.
When a change keeps producing a regression in an area it has no business
touching, ask which second meaning the thing you are changing is carrying.
