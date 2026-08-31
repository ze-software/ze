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

### Run 2 (closure, 2026-08-30)

Scope: the whole diff `fc9c8bcaa~1..01d0474da`, read from git rather than the
working tree, with two lenses: the guard lens (does a new guard fail open) and
the wiring lens (does the barrier's population reach the code that releases it).

| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 9 | BLOCKER | The barrier's release is FUNGIBLE. `apiSyncCount` is an anonymous counter, so any `request peer <addr> plugin session ready` satisfies one unit of it whoever sent it, and this spec's `signalSessionReady` makes `bgp-adj-rib-in` report on EVERY peer-up, including a peer that attaches it for events alone and does not count it. `test/plugin/role-otc-rs-withdraw-eor.ci` is such a config: it counts `rs` and `test-rs-eor` and the uncounted `bgp-adj-rib-in` answers for one of them, so the marker can precede a counted process's routes. That is the RFC 4724 Section 4 obligation this spec exists to close, defeated by the spec's own change | `internal/component/bgp/reactor/peer.go` `(*Peer).SignalAPIReady`, `internal/component/bgp/plugins/adj_rib_in/rib.go` `(*AdjRIBInManager).signalSessionReady` | fix WRITTEN, not landed: `resetAPISync` takes the counted process NAMES, `SignalAPIReady` takes the `plugin.Sender` and credits the report to it, and a report from outside the set or a repeat from inside it is refused. Proven RED with the guard removed (the marker reaches the wire while `pusher-two` still owes routes) by `TestInitialSyncBarrierCreditsOnlyTheProcessesItCounts` |
| 10 | BLOCKER | Finding 9's fix cannot land alone. An honest barrier waits `apiSyncTimeout` for every counted process that never reports, and several do not: `(*routeServer).replayForPeer` returns at its `stale` branch, and for a peer that is not a route-server client, without reaching `sendEOR`; a `.ci` test fixture carrying `send [ update ]` never reports at all. Measured: `./le functional plugin` goes from 5 failures to 9, and the four added are route-server observers reading `EOR peers=1/3` | `internal/component/bgp/plugins/rs/server_handlers.go` `replayForPeer` | OPEN, and it is the OVERCOUNT this spec put out of scope. Its home is `plan/spec-bgp-session-ready-contract.md`, whose Task is this exact sentence: "the set that is COUNTED is not the set that SIGNALS" |

This spec does not close on these. Finding 9 says the goal is not met; finding
10 says the repair reaches into a spec that is still `skeleton`. Which way the
two land together is the owner's decision, so the spec stays `in-progress` and
the fix stays uncommitted in the working tree.

### Final status
- [ ] Re-review shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

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
