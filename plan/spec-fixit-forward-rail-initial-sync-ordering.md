# Spec: fixit-forward-rail-initial-sync-ordering

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 2/2 |
| Updated | 2026-08-11 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. The masked-verdict-and-RFC-exemption record (retired with the learned corpus) - the session that found this
3. Source files under Current Behavior

## Freshness check (2026-07-27)

Re-verified against the tree at `5b6d64c35`; the defect is **still live** and the
premise holds. Line numbers below have drifted since 2026-07-22, so use these:

| Spec said | Now | What it is |
|-----------|-----|------------|
| `peer.go` | `peer.go` | `ShouldQueue` definition |
| `reactor_api_batch.go` | `reactor_api_batch.go` | batch announce gate |
| `reactor_api_batch.go` | `reactor_api_batch.go` | batch withdraw gate |
| `reactor_api_forward.go` | `reactor_api_forward.go` | `AnnounceEOR` gate |

Still exactly three non-test callers, all on the route-INJECTION rail. A full
`grep -rn ShouldQueue internal/component/bgp/reactor/` shows no call in
`forward_rs.go` and none in `forwardUpdateCore`, so the forwarding rail remains
ungated.

Not started: this is a hot-path ordering change in the reactor, so it needs
`make ze-race-reactor` (`ai/rules/testing.md` makes that mandatory for reactor
concurrency edits) and an independent review pass before it can close.

## Design analysis (2026-07-27)

Not implemented. This records the shape the fix must take and, more usefully,
which two cheap-looking fixes are WRONG.

**D-1. Dropping the forwarded update is NOT safe: the fix must defer, not skip.**
The tempting one-liner is "if `ShouldQueue()`, skip this destination", mirroring
the existing drop for a not-established peer. It loses data. Initial sync reads
from the RIB, so for a withdraw of prefix P arriving mid-sync:

| Sync position | Effect of dropping the forwarded withdraw | Correct? |
|---------------|-------------------------------------------|----------|
| has not yet reached P | RIB already has P withdrawn, so sync never announces it | yes, by luck |
| already sent P | nothing else will ever withdraw P | **no -- peer holds P forever** |

The second row is the reported bug reproduced by its own proposed fix, so
`ShouldQueue` cannot gate a discard here. The not-established drop is safe only
because that peer gets a full RIB replay on establishment; a mid-sync peer does
not get a second replay.

**D-2. It cannot go on `opQueue` as-is.** `PeerOp` (`peer.go`, verified)
carries `Route *rib.Route`, `NLRI nlri.NLRI`, `Subcode`, `Message` -- structured
operations only, no wire-body member. A forwarded UPDATE here is wire bytes
deliberately: not re-deriving structure per destination is the entire purpose of
the forward rail, so converting it to a `PeerOp` would undo the change this spec
descends from.

**D-3. Strict order forbids a second queue.** Two queues drained independently
lose the relative order of a queued announce and a forwarded withdraw, which is
the exact invariant being restored. One FIFO must carry both, so `PeerOp` grows
a wire variant (`PeerOpForward` holding a pooled buffer handle) rather than the
forward rail growing a parallel path.

**D-4. SUPERSEDED 2026-08-10. Its premise is false: the budget and the overflow
policy are already shipped.** The paragraph below is kept for history. It asked
which byte budget and which overflow behaviour to invent. Both exist, and every
claim here was read from its producer.

~~D-4. The open question is buffer lifetime, and it is a memory-architecture
question, not a reactor one. A queued wire body must outlive the source peer's
read buffer, so deferring means pinning a pooled buffer for the whole
initial-sync window. `opQueueMax` defaults to `DefaultOpQueueSize` = 10000
(`peer.go`) and scales with prefix-maximum, so a COUNT-bounded queue of wire
bodies is a BYTE-unbounded pin on the forward pools. Per
`ai/rules/performance.md` the queue has to be byte-budgeted like the
global shared pool, and the overflow behaviour chosen deliberately: pool
exhaustion is the backpressure signal, and the honest options are tearing the
destination session down (it re-syncs) or ending the sync early -- never a silent
drop, which is D-1 again by another route.~~

**D-4a. The forward pool already has two levels, and neither drops a route.**
Tier 1 is the per-destination pool `newPeerPool` builds, `peerPoolSize` = 64
fixed buffers in one contiguous allocation. Tier 2 is the shared `MixedBufMux`
wired by `setOverflowMux`: 64K blocks, each whole or subdivided into 16 slices of
4K. `setByteBudget` converts bytes to blocks, auto-sized from peer prefix maximums
and overridable by `ze.fwd.pool.size` and `ze.fwd.pool.maxbytes`. Tier 1
exhaustion falls back to the `modBufPool` sync.Pool. Tier 2 denial proceeds
without a handle, and `DispatchOverflow` says so in its own comment: routes are
never dropped.

**D-4b. The tier 2 handle WAS an accounting token, not storage.** Nothing wrote
into the handle's buffer. The queue's storage is the unbounded `w.overflow`
slice, and the bytes stayed in the source peer's read buffer, aliased by
`rawBodies`. That aliasing is the D-5 hazard, and Phase 1 closed it: the handle
now carries the item's copied bodies, so deferring costs a budget token, a slice
entry, and one copy.

**D-4c. The overflow behaviour was already chosen, and it is the one D-4 asks
for.** `forward_pool_congestion.go` sets `congestionDenialThreshold` at 0.80 and
`congestionTeardownThreshold` at 0.95, and `CheckTeardown` tears the destination
session down so it re-syncs. Graduated backpressure above a byte budget, ending
in the teardown D-4 lists as an honest option. Nothing here is owed.

**D-4d. So the design reduces to two edits and one predicate.** Gate the two
dispatch sites on `ShouldQueue()` and route to `DispatchOverflow` instead of
`TryDispatch`; add a hold predicate at the top of `drainOverflow` so held items
stay in `w.overflow` until sync ends. `overflowPending` already stops a later
forward overtaking a held one, `fwdKey` is the destination peer so the FIFO is
already per-destination, and `sendInitialRoutes` already has the wake point where
it clears `sendingInitialRoutes`.

**D-5. The genuine open question, which D-4 never identified: the recent-update
cache safety valve can reclaim the bytes under a long-deferred item.**
`isGapEvictable` refuses eviction only while `totalConsumers()` is above zero, so
an entry held solely by `retainCount` is force-evictable once a later entry is
fully acked and `retainedAt` passes the valve. `runGapScan` then calls
`evictLocked`, which returns the read buffer through `ReturnReadBuffer`. A held
forward item whose `rawBodies` alias that buffer would be written from recycled
memory.

The default valve is 5 minutes, but it drops to 30 seconds under read-pool
pressure. Deferral extends the hold to the whole initial-sync window, which on a
full-table drain is exactly the regime where the 30-second valve bites. This
hazard is pre-existing and reachable today by any long overflow episode, so
deferral does not invent it; deferral makes it routine. No test covers valve
eviction against a retained forward item.

**D-5 is LIVE TODAY, not a consequence of deferral.** Every claim below was read
from its producer on 2026-08-10.

`evictLocked` returns the entry's buffers to the multiplexer: `ReturnReadBuffer`
on `poolBuf`, on both EBGP patched slots, and `returnFwdHandles` on every adopted
per-forward wire variant. A queued `fwdItem`'s `rawBodies` alias
`peerWire.Payload()` (`buildFwdBody`), which is memory inside those same buffers.
`isGapEvictable` is DESIGNED to evict an entry that still has consumers: it
returns false when `totalConsumers() <= 0` precisely because a consumer-free entry
is already evicted by the normal path. `retainCount` is counted in
`totalConsumers`, so the retain a forward item holds does not protect it.

So an overflow item whose entry is passed over, with a later entry fully acked and
the valve elapsed, has its bytes reclaimed and reissued to another session while it
is still queued. The worker then writes recycled memory to a peer. That is the
wrong-wire-bytes class again, by a different route, and it needs no deferral to
happen: 30 seconds in overflow under read-pool pressure is enough.

**D-5 DECISION (2026-08-10, owner-ordered: fix D-5 before the ordering fix).**
Make tier 2 hold the bytes it is already sized for. `DispatchOverflow` acquires a
`MixedBufMux` handle today and never writes into it, so tier 2 is an accounting
token over aliased memory. On the overflow path, COPY `rawBodies` into that handle
and re-point `rawBodies` at the copy. The item then owns its bytes, eviction
becomes harmless, and the aliasing that creates the hazard is gone rather than
coordinated around.

| Rejected alternative | Why |
|----------------------|-----|
| Make the valve refuse to evict while `retainCount > 0` | Defeats the valve. It exists to bound exactly the stalled-consumer case, and a leaked retain would then pin a buffer forever |
| Have eviction call back into the forward pool to flush that update's items | More coordination across two lock domains, and it converts a memory bug into a lock-ordering bug |

**Handle denial honestly.** When the mux denies, or the item's bodies exceed what
one handle carries, allocate the copy on the heap for that item. An allocation on
the exhausted-pool path is the correct trade: `ai/rules/rule-precedence.md` puts
correctness above the buffer-first rule, and the fast path is untouched because
only the overflow path copies.

**The fast path stays zero-copy.** `TryDispatch` items reach the channel and drain
in microseconds, and the valve cannot fire against them: it requires a later entry
fully acked plus the valve duration. Do not copy there.

Phase 1 of this spec is D-5. Phase 2 is the ordering fix (D-4d).

**D-6. One latency consequence to state in the closure.** Holding `drainOverflow`
while `ShouldQueue()` is true makes `Barrier` and `BarrierPeer` block until
initial sync completes, because the barrier sentinel already routes through
overflow when overflow is non-empty. That is correct behaviour and a real change
to how long `request peer * flush` takes.

**D-7 (implementation, 2026-08-10). The hold needs a WAKE, and it is the wake
`DispatchOverflow` already sends.** `runWorker` re-evaluates the hold only after
something reaches the worker's channel, and a held worker's channel is empty by
construction: the gates send every item for that destination to overflow, and
`TryDispatch` refuses while `overflowPending > 0`. Clearing
`sendingInitialRoutes` therefore releases nothing on its own. Measured with the
wake removed: the forwarded withdraw never reached the peer within 5s
(`TestForwardedWithdrawWaitsForQueuedAnnounceCoreRail`), and the idle timer is a
5-second re-check that refuses to exit rather than a drain. `wakeOverflow` sends
the same non-blocking nil-peer sentinel `DispatchOverflow` already sends for the
same wedge, and `Peer.wakeForwardOverflow` calls it beside every store that
clears the flag.

**The wake takes NO peer lock, and that is a correctness requirement rather than
an optimisation.** One of its five call sites is the recover defer in
`sendInitialRoutes`, and the drain loop that defer guards holds `p.mu.Lock`
across `buildRIBRouteUpdate`. A panic there reaches the defer write-locked, so an
`RLock` inside the wake deadlocks the peer's goroutine for good. `p.reactor` is
read without the lock, like every other reader of it (`peer_run.go`), and
`p.settings.PeerKey()` reads fields the `Peer.Settings` contract fixes at
construction.

**D-8 (implementation, 2026-08-10, REVISED after review). Gate and hold are ONE
predicate, `Peer.forwardOrderHold()` = `Established && initialSyncInProgress()`.**
The first version gated on `ShouldQueue()` and held on the narrower condition,
which is two bugs in one mismatch: a gate wider than its hold parks items nothing
will release, and a hold wider than its gate releases items the gate never
parked. Both narrowings the one predicate makes are load-bearing.

| Excluded | Why |
|----------|-----|
| Not Established | That peer will never drain an opQueue, so holding for it parks the items until its REPLACEMENT session finishes its own sync and then delivers a dead session's UPDATE after a full RIB dump -- the duplicate delivery `test/plugin/role-otc-rs-withdraw-eor.ci` refuses. Not gating lets `fwdBatchHandler` discard them, which is what the forwarding rail already does for a destination that is not Established |
| Established, sync flag clear, `opQueue` non-empty | `setState` stores the flag in the same call that publishes Established and BEFORE it, so from establishment until `sendInitialRoutes` clears the flag every queued operation is covered by the flag alone. An `opQueue` still non-empty with the flag CLEAR is one NOTHING will drain: `sendInitialRoutes` is its only drainer and it has returned (the `nc == nil` abort, the recover defer, or the check-then-append race between `ShouldQueue()` and `QueueAnnounce` in `reactor_api_batch.go`). Ordering a forwarded UPDATE behind it would park that UPDATE for the life of the session |

**D-9 (implementation, 2026-08-10). The batch handler must not sort a batch, and
`fwdReorderWithdrawalsFirst` is deleted.** It hoisted every withdrawal ahead of
every announcement, and its stated safety condition was false. The condition it
claimed (`fwdSupersedeKey` superseding means no prefix is in both groups) merges
only BYTE-IDENTICAL bodies, and an announce and a withdraw of one prefix never
are. Measured: two forwarded operations on 192.0.2.0/24 parked behind one
destination's sync arrive as withdraw-then-announce, so the peer ends holding a
prefix that was withdrawn -- AC-1's own words.

It came from RustBGPd (`docs/architecture/congestion-industry.md`), where it is
safe because `PendingTx` deduplicates by PREFIX. That page's own paragraph states
the consequence without per-prefix dedup: "reordering can invert an
announce/withdraw sequence for the same prefix, causing permanent stale routes at
the remote peer."

Deleting it costs nothing it was bought for. `fwdBatchHandler` writes every item
of a batch under one `writeMu` and calls `flushWrites` ONCE, so the partition
moved bytes inside a single write rather than sending a withdrawal any sooner.
`fwdItem.withdrawal`, `fwdIsWithdrawal` and the attribute scan that fed it are
deleted with it: they had no other reader, and the classifier parsed every
forwarded body on the hot path.

**D-10 (implementation, 2026-08-10). `fwdBucketMerge` reordered by the same
mechanism, one function later, and is fixed rather than left half-done.** It
grouped items by attribute hash ACROSS the batch and appended each merged body at
the END, so `[announce P, withdraw P, announce Q]` (P and Q sharing attributes)
came out as `[withdraw P, announce P+Q]`. Same inversion, same blackhole,
reproduced by `TestForwardBucketKeepsOrderAcrossAWithdraw` before the fix.

Merging now runs over ADJACENT items only: a run of consecutive mergeable items
carrying byte-identical attributes, packed into bodies emitted where the run was.
Anything that cannot join a run is a barrier. That is order-preserving by
construction, and a run's members are equivalent to merge among themselves (same
attributes, no withdrawals). The cost is a lower merge rate on a batch whose
attributes alternate. The same rewrite also stops a run whose members carry no
legacy NLRI (MP_REACH/MP_UNREACH) being stripped with nothing emitted in its
place, which dropped those UPDATEs whenever another group in the batch merged.

**D-11 (implementation, 2026-08-11, round 6 ISSUE-B). The route-server rail's
pending-overflow clause takes no pool lock: the destination peer holds the
count.** Round 4 put `hasOverflowPending` on that rail, and it took
`fp.mu.RLock` plus a `map[fwdKey]*fwdWorker` lookup per destination per UPDATE,
whenever `forwardOrderHold()` was false. That is the steady state, and the rail
exists to bypass the pool.

**The count itself did NOT move, and that is the whole of why the five ordering
properties survive.** It is the same `atomic.Int64`, mutated in the same four
places under the same `w.overflowMu`: `DispatchOverflow` adds one per appended
item, `runWorker` re-derives it from `len(w.overflow)` when a batch completes
with an empty channel, the stopped branch of `drainOverflow` re-derives it the
same way, and `Stop` zeroes it. `TryDispatch` still reads it directly. What
changed is WHERE the route-server rail reads it: `fwdWorker.overflowPending` is
now a `*atomic.Int64`, and `DispatchOverflow` publishes that pointer on
`item.peer` before it can raise the count, so the rail reads it through the
destination it already holds. The gate is `dst.forwardOrderHold() ||
dst.forwardOverflowPending()`: four atomic loads, no lock and no map hash.

**A count kept ON the peer was the reviewer's shape, and it cannot be kept
exact.** Three producers say so. `runWorker` re-derives the count as
`len(w.overflow)`, and that slice also holds nil-peer barrier sentinels
(`forward_pool_barrier.go` dispatches one through `DispatchOverflow` whenever
overflow is non-empty), which belong to no peer. Zeroing when the slice empties
needs the worker to remember which peer it was counting, which is state the
worker does not carry. `Stop` zeroes the worker's count in one store, while a
per-peer count would leak the contribution of every item already on the channel
or in the batch, because those items are not in the slice it can walk. An
approximate correctness gate is not shippable, so the count stays where it is
proven and only the READ moves.

**The published pointer names the counter, not the worker, and the difference is
memory.** An interior handle into `fwdWorker` would pin the whole worker after
its goroutine exits, `batchBuf` included, which is up to `batchLimit` `fwdItem`
values for every peer that ever overflowed. Eight bytes on their own pin eight
bytes.

**One divergence from the map lookup, and it is harmless.** A nil handle means
no real item was ever parked for that peer, so nothing is owed. A barrier
sentinel can raise the count while naming no peer; the map lookup would then
refuse the direct write and this reader does not. A sentinel carries no bytes,
so a destination owed nothing but sentinels is owed nothing.

**Measured on the gate itself**, 64 workers in the pool, AMD EPYC 7351:
38.16 ns/op through the pool lookup against 0.61 ns/op through the peer handle,
and 63.77 ns/op against 0.097 ns/op with 29 goroutines calling it. The pool
lookup gets slower under concurrency because every reader does a
read-modify-write on one `RWMutex` word; the peer handle is two loads and scales.
No test distinguishes the two implementations: both ordering tests pass on
either, which is expected, because they test the gate and not its implementation.

## Task

The BGP forwarding rail never consults `Peer.ShouldQueue()`, so a forwarded UPDATE
can overtake a route already queued for the same peer and leave that peer holding
a stale route.

`ShouldQueue()` (`internal/component/bgp/reactor/peer.go`) returns true
while a peer is running initial route sync or still has a non-empty `opQueue`. It
exists to preserve strict insertion order of route operations. It is called from
exactly three non-test sites, all on the route-INJECTION rail:
`reactor_api_batch.go` (batch announce), `:235` (batch withdraw), and
`reactor_api_forward.go` (`AnnounceEOR`).

The FORWARDING rail consults it nowhere. Neither `reactorForwardRS`
(`internal/component/bgp/reactor/forward_rs.go`) nor `forwardUpdateCore`
(`internal/component/bgp/reactor/reactor_api_forward.go`) gates on peer
readiness: their per-destination loops filter on `forwardFacts() != nil`, export
filters, community policy and RR rules only.

`forwardFacts()` is not a readiness gate. It is a plain atomic load
(`peer_forward_facts.go`) whose snapshot is stored by `setEncodingContexts`
(`peer.go`) BEFORE `sendingInitialRoutes` is set and before the sync
goroutine starts, so facts are non-nil for the whole initial-sync window.

Consequence: an announce for prefix P sits in a peer's `opQueue` while a withdraw
for P arrives from another peer and is forwarded directly. The withdraw reaches
the wire first, the queued announce drains after it, and the peer ends up
believing P is live when it has been withdrawn.

**This is NOT about End-of-RIB ordering.** RFC 4724 orders the EOR only against
the speaker's own initial dump, never against routes learned later
(`rfc/short/rfc4724.md`). Tests asserting EOR-vs-forwarded-route order were
asserting something Ze never owed and were corrected separately; see
the masked-verdict-and-RFC-exemption record.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - reactor and forwarding overview
  → Constraint: (fill during design)

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc4271.md` - UPDATE semantics and route replacement
  → Constraint: (fill during design) -- a withdraw and a later announce of the
    same prefix are order-sensitive; the receiver applies them as delivered.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/reactor/peer.go` - `ShouldQueue`/`PendingSync`
  → Constraint: `ShouldQueue` is true while `sendingInitialRoutes != 0` or the
    `opQueue` is non-empty; it also gates on state being Established.
- [ ] `internal/component/bgp/reactor/forward_rs.go` - RS fast path
  → Constraint: `tryDirectWriteNoFlush` gates on session non-nil, FSM
    Established, and `writeMu.TryLock()`. No readiness check.
- [ ] `internal/component/bgp/reactor/reactor_api_forward.go` - `forwardUpdateCore`
  → Constraint: per-destination gates are facts-nil, community policy, RR rules,
    export filters and wire-rewrite failure. No readiness check.
- [ ] `internal/component/bgp/reactor/peer_forward_facts.go` - facts load
  → Constraint: non-nil from `setEncodingContexts` until teardown, therefore
    non-nil throughout initial sync.
- [ ] `internal/component/bgp/reactor/peer.go` - `PeerOp`
  → Constraint: `opQueue` holds STRUCTURED ops (`Route`, `NLRI`), not wire
    bodies, so a forwarded wire UPDATE cannot simply be queued there.

**Behavior to preserve:**
- The fast path must not block: it runs on the SOURCE peer's read goroutine, so
  waiting there stalls forwarding to every other destination of that source.
- `HoldWrites` must stay short: `writeMu` also gates KEEPALIVE
  (`session_write.go` `writeMessage`), so a long hold risks the hold timer.
- Forwarded updates to a genuinely not-established peer are DROPPED today, not
  deferred; that is deliberate (the peer gets a RIB replay on establishment).

**Behavior to change:**
- Only the ordering hazard above.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
A peer is Established but still draining its initial sync when an UPDATE arrives
from a different peer for a prefix already queued to it.

### Transformation Path
See "Design analysis" under the Freshness check above: D-1 to D-4 establish that
the forwarded update must be deferred on the SAME FIFO as `opQueue`, and that
buffer lifetime is the open question.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| source read goroutine ↔ destination session | `tryDirectWriteNoFlush` writes directly under `writeMu.TryLock()` | [ ] |
| forward pool worker ↔ destination session | `fwdBatchHandler` takes a blocking `writeMu.Lock()` | [ ] |
| initial-sync goroutine ↔ destination session | `opQueue` drain under `p.mu` | [ ] |

### Integration Points
- `internal/component/bgp/reactor/forward_rs.go`
- `internal/component/bgp/reactor/reactor_api_forward.go`

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| announce queued for a peer in initial sync, then a withdraw for the same prefix forwarded | -> | forwarding rail honours insertion order | `TestForwardedWithdrawWaitsForQueuedAnnounceCoreRail` and `...RSRail` (`internal/component/bgp/reactor/forward_initial_sync_order_test.go`) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Peer P is Established and mid-initial-sync with an announce for prefix X queued; a withdraw for X is forwarded from another peer | P receives announce then withdraw, in that order; P does not end holding X |
| AC-2 | A forward item is queued in a worker's overflow buffer and the recent-update cache entry it came from is force-evicted by the safety valve | the item still carries the bytes it was queued with; no handle is returned twice and no route is dropped |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestOverflowItemSurvivesSafetyValveEviction` | `internal/component/bgp/reactor/forward_overflow_own_bytes_test.go` | AC-2 | passing; red without the fix |
| `TestOverflowItemOwnsBytesWithoutAHandle` | same | AC-2 denial path | passing; red without the fix |
| `TestOverflowItemOwnsBytesLargerThanOneHandle` | same | AC-2 oversized item | passing; red without the fix |
| `TestOverflowItemOwnsUpdateSections` | same | AC-2 re-encode rail | passing; red without the fix |
| `TestForwardedWithdrawWaitsForQueuedAnnounceCoreRail` | `internal/component/bgp/reactor/forward_initial_sync_order_test.go` | AC-1, plugin rail | passing; red without the fix |
| `TestForwardedWithdrawWaitsForQueuedAnnounceRSRail` | same | AC-1, route-server fast path | passing; red without the fix |
| `TestForwardedAnnounceThenWithdrawKeepOrder` | same | AC-1, two FORWARDED operations on one prefix | passing; red before D-9 (announce at wire index 2, withdraw at 1) |
| `TestFwdBatchKeepsQueuedOrder` | `internal/component/bgp/reactor/forward_pool_supersede_test.go` | AC-1, `safeBatchHandle` preserves queue order | passing; replaces the five AC-25 tests of the deleted reorder |
| `TestForwardBucketKeepsOrderAcrossAWithdraw` | `internal/component/bgp/reactor/forward_bucket_test.go` | AC-1, D-10 merge does not move an announce past a same-prefix withdraw | passing; red before D-10 (announce at index 3, withdraw at 1) |
| `TestForwardBucketKeepsMPOnlyBodies` | same | AC-1, D-10 defect 3: a run that packs no body consumes nothing | passing; red with the empty-run guard reverted (`got 0 of 2` for MP_REACH and for MP_UNREACH) |
| `TestForwardedUpdateWaitsForPendingOverflowRSRail` | `internal/component/bgp/reactor/forward_initial_sync_order_test.go` | AC-1, the pending-overflow count driven from `reactorForwardRS` | passing; red with the `forwardOverflowPending()` clause reverted (announce at 1, withdraw at 0) |
| `TestForwardedUpdateWaitsForInFlightOverflowRSRail` | same | AC-1, the enqueue-to-`writeMu` window: an item released from overflow is on the channel, not on the wire | passing; red with the `forwardOverflowPending()` clause STILL in place and only the count's meaning reverted (announce at 1, withdraw at 0) |

Both AC-1 tests assert on the BYTES the destination's connection received, and
each of the three pieces was measured to be load-bearing on its own: with the
dispatch gates removed the withdraw arrives FIRST (announce at index 1, withdraw
at index 0, both rails); with only the `drainOverflow` hold removed the withdraw
reaches the wire before the sync runs; with only the wake removed it never
arrives.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `forward-overflow-two-tier` | `test/plugin/forward-overflow-two-tier.ci` | 50 UPDATEs burst through a 2-deep channel still arrive in order, no handle leak | passing (regression, not written for this spec) |
| `role-otc-rs-withdraw-eor` | `test/plugin/role-otc-rs-withdraw-eor.ci` | a relayed route is delivered ONCE, not twice, across the initial-sync window | passing (regression: the test that refused the earlier barrier attempt) |

A `forward-order-during-initial-sync.ci` was NOT written. The window it would
have to hit is the destination's initial sync, which is sub-millisecond for a
peer with no static routes, and the only way to widen it from config is the
`apiSyncExpected` 500ms hold -- the same widening that made
`role-otc-rs-withdraw-eor.ci` deliver a relayed route twice on 2026-08-08
(`peer_initial_sync.go`). The two unit tests above hold that window open
deterministically and assert on the same wire bytes a `.ci` would read.

## Files to Modify

This list is this spec's half of the ONE combined commit (see the closing
section). Fifteen files carry this spec's work, and the review artifact
`tmp/review/fixit-forward-rail-initial-sync-ordering-640fa955-f03a-45e8-a58f-4b367f5859e6.md`
hash-pins every one of them.

Phase 1 (D-5), done:
- `internal/component/bgp/reactor/forward_body.go` - `ownOverflowBodies`
- `internal/component/bgp/reactor/forward_pool.go` - `DispatchOverflow` calls it
- `internal/component/bgp/reactor/forward_overflow_own_bytes_test.go` (new) - the four AC-2 tests

Phase 2 (D-4d), done:
- `internal/component/bgp/reactor/reactor_api_forward.go` - dispatch gate in `forwardUpdateCore`
- `internal/component/bgp/reactor/forward_rs.go` - dispatch gate in `reactorForwardRS`, ahead of `tryDirectWriteNoFlush`, and the pending-overflow refusal beside it
- `internal/component/bgp/reactor/forward_pool.go` - `overflowHeld`, the hold at the top of `drainOverflow`, `wakeOverflow`, `fwdWorker.overflowPending`
- `internal/component/bgp/reactor/peer.go` - `forwardOrderHold`, `forwardOverflowPending`, `wakeForwardOverflow`
- `internal/component/bgp/reactor/peer_initial_sync.go` - wake beside each of the four stores that clear `sendingInitialRoutes`
- `internal/component/bgp/reactor/peer_run.go` - wake beside the session-teardown store
- `internal/component/bgp/reactor/forward_initial_sync_order_test.go` (new) - the three AC-1 tests

Phase 2 review round 3 (D-8 revision, D-9, D-10), done:
- `internal/component/bgp/reactor/forward_pool.go` - `fwdReorderWithdrawalsFirst`, `fwdIsWithdrawal`, `fwdAttrsHaveReach`, `fwdAttrsHaveUnreach`, `fwdAttrsScanCode` and `fwdItem.withdrawal` deleted
- `internal/component/bgp/reactor/forward_body.go` - `fwdBodyResult.withdrawal` deleted
- `internal/component/bgp/reactor/forward_bucket.go` - adjacency-only merge
- `internal/component/bgp/reactor/forward_bucket_test.go`, `forward_pool_supersede_test.go`, `forward_pool_property_test.go`, `forward_update_test.go` - the ordering tests that replace the deleted reorder's
- `docs/architecture/forward-congestion-pool.md`, `docs/architecture/congestion-industry.md`

Closure, done:
- `test/relax-ceiling.txt` - 748 -> 752 with a `raised-for:` line naming which four tests lost what. It MUST ride in the same commit as the four `test-relax:` tokens, which is what `scripts/dev/relax-census.py` refuses a raise without
- `plan/known-failures/bgp-plugin-rs-forward-duplicate-and-order.md` - two rows of its withdrawals-first table named `fwdReorderWithdrawalsFirst` and `fwdItem.withdrawal`, both deleted by D-9; they now name `safeBatchHandle` and the adjacency-only `fwdBucketMerge`

Not this spec's product change, and it rides anyway:
- `internal/component/bgp/reactor/session_rfc7606_diagnostics_test.go` - Thomas's `rfc-test-change-approved:` marker and the `syncBuffer` race fix. Leaving it out reds `make ze-race-reactor` at HEAD, and `rfc/audit/rfc7606.json` (the shutdown spec's file) now pins its content, so `check_audit_freshness` fails on a fresh clone without it

## Known Limitations

| Limitation | Detail |
|------------|--------|
| `request peer <sel> flush` blocks for the whole initial sync of any targeted peer | D-6, confirmed by reading `handleBgpPeerFlush` (`internal/component/bgp/plugins/cmd/peer/peer.go`): it wraps `FlushForwardPool` in a 30s context and reports `StatusError` on expiry. The barrier sentinel routes through overflow whenever overflow is non-empty (`forward_pool_barrier.go`), so it now sits behind the held items. No `.ci` depends on a prompt flush DURING a sync: the two that call `request peer * flush` (`forward-overflow-two-tier.ci`, `forward-two-tier-under-load.ci`) call it after establishment and after their burst, and both pass |
| The forwarding rail reads four atomics per destination per UPDATE, and takes no lock | `forwardOrderHold()` needs no peer lock: state and the sync flag are both atomics, and dropping `len(opQueue)` from the predicate (D-8) dropped the `p.mu.RLock` the first version took. The RS rail's pending-overflow clause adds two more atomic loads and no lock (D-11): the destination `*Peer` is in hand at the call site and holds the counter the pool keeps for it. Nothing allocates: `BenchmarkFwdPoolTryDispatch` is at 0 allocs/op, `BenchmarkReactorForwardRS` at 28, `BenchmarkForwardBucketDrain` at 8, all three unchanged by D-11 |
| A destination held for its whole sync keeps its tier-2 handles for that window | The designed backpressure still applies and still escalates: every dispatch to a held peer wakes its worker (`DispatchOverflow`'s sentinel), and `runWorker` calls `congestion.CheckTeardown(key.peerAddr)` on each such wake. Denial at 0.80 and teardown at 0.95 are therefore evaluated for the held peer by the traffic that is filling its queue |
| `overflowHeld` reads the peer of the FIRST real item, not of `fwdKey` | Correct while one `*Peer` owns the key, which is the invariant `fwdKey` already assumes everywhere else in the pool. It goes stale if a dynamic peer is removed and re-added under the same `AddrPort` while items of the previous `*Peer` are still parked |
| The teardown wake races a replacement session | `peer_run.go` wakes the worker after the session-teardown store, and a replacement session could establish before the drain runs. `fwdBatchHandler` discards on session-nil or not-Established, so the window costs a reconnect delay against a microsecond drain |
| The merge rate drops on a batch whose attributes alternate | D-10: merging is adjacency-only now, so `[A(x), A(y), A(x)]` merges nothing where hash grouping merged the two `x` items. Correctness of same-prefix order is not tradeable against message count (`ai/rules/rule-precedence.md` rung 2) |

## Implementation Steps

### Implementation Phases
1. Reproduce AC-1 as a failing test.
2. Design the readiness gate (note: `opQueue` cannot hold wire bodies, and
   blocking the fast path is not an option -- see Behavior to preserve).
3. Implement, then `make ze-race-reactor` (reactor concurrency change).

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] `make ze-test` passes

## State at 2026-08-10 (session paused, NOT closed)

All three phases are implemented and independently reviewed. The spec stays open
on two ISSUEs, both missing tests, both named by the round-3 review.

**Phase 1 (D-5), landed.** `ownOverflowBodies` (`forward_body.go`) copies the
item's payload into the tier 2 handle it already holds, or onto the heap, and
re-points the item through fresh header slices. Called once from
`DispatchOverflow`. Scope was extended on evidence: `item.updates` carried the
same defect on the cross-context rail, aliasing either the entry's `poolBuf` or
the transcode handle, both of which `evictLocked` returns.

**Phase 2, landed.** `ShouldQueue()` gates both dispatch sites, the RS one placed
ahead of `tryDirectWriteNoFlush`. `overflowHeld` holds `drainOverflow`. The wake
was NOT in the design and was found by measurement: without it the forwarded
withdraw never arrived, because a held worker's channel is empty by construction.

**Phase 3, landed, and it is the largest finding.** `fwdReorderWithdrawalsFirst`
is DELETED. `docs/architecture/congestion-industry.md` states the precondition it
needs, per-prefix dedup, and Ze dedups by BYTES, so the precondition never held.
Chasing it found the same inversion in `fwdBucketMerge`, and a third defect where
a run packing no body was stripped with nothing emitted, dropping MP_REACH and
MP_UNREACH updates whenever another group merged.

**What blocks closure**

| # | Owed | State |
|---|------|-------|
| 1 | Defect 3 (dropped MP-only UPDATEs) has no regression test. Proven real and fixed by an overlay probe; the case belongs in `forward_bucket_test.go` | DONE 2026-08-11, `TestForwardBucketKeepsMPOnlyBodies` |
| 2 | `hasOverflowPending` has no test driving it from `reactorForwardRS` | DONE 2026-08-11, `TestForwardedUpdateWaitsForPendingOverflowRSRail` |
| 3 | One clause in `docs/architecture/congestion-industry.md`: the "no sooner" justification is not absolute, because `bufWriter` is a 16K `bufio.Writer` and `batchLimit` defaults to 1024, so a large batch does auto-flush mid-batch. The verdict is unchanged; the published absolute is wrong | DONE 2026-08-11, the paragraph now states the exception and keeps the verdict |
| 4 | No `## Review Gate` section; the artifact carries `verdict=findings`, rounds 4 | OPEN, `/ze-close` |
| 5 | Round 5 ISSUE-1: the ordering gate closed only half the window. `drainOverflow` decremented `overflowPending` as each item entered `w.ch`, and the worker takes `session.writeMu` only on its next loop iteration; a direct write from `reactorForwardRS` that won that race overtook the items the hold had just released | DONE 2026-08-11. The count is no longer decremented per enqueue: `runWorker` re-derives it from `len(w.overflow)` once a batch completes with an empty channel, which is the first moment every released item is provably written. `TestForwardedUpdateWaitsForInFlightOverflowRSRail` |
| 6 | Round 5 ISSUE-3: this spec's 4 `test-relax:` tokens take the HEAD census to 755 against a ceiling of 751 | BLOCKED, not this spec's file to commit. See "The relax ceiling is another session's file" below |
| 7 | Round 6 ISSUE-B: the ordering gate put a pool lock and a map hash on the rail that exists to bypass the pool | DONE 2026-08-11, D-11. The count did not move; only the read did |

**The two tests owed above were written on 2026-08-11 and each was measured
against the fix it targets.** Reverting the empty-run guard in `fwdBucketMerge`
drops all four MP-only UPDATEs (`got 0 of 2` twice), and reverting the
`hasOverflowPending` clause in `reactorForwardRS` inverts the pair on the wire
("1" is not less than "0"). Both were restored and the four ordering tests pass
five times over under `-race`.

**Item 3 read its two figures at their producers**, not from the spec text:
`session_connection.go` sizes `bufWriter` with
`max(env.GetInt("ze.buf.write.size", 16384), 4096)`, and `reactor.go` sets
`batchLimit` from `max(env.GetInt("ze.fwd.batch.limit", 1024), 0)`.

**The relax ceiling is RESOLVED (2026-08-11).** It was blocked while the gate was
another session's untracked work, and that premise is dead: `test/relax-ceiling.txt`
and `scripts/dev/relax-census.py` are tracked at HEAD, landed by
`spec-relax-token-gate-is-per-file-not-per-change`. `python3 scripts/dev/relax-census.py`
now reads 748 tokens at HEAD and 752 in the working tree, the +4 being this spec's.

The ceiling is raised to **752** with a `raised-for:` line naming which four tests
lost what, in `test/relax-ceiling.txt`, and the census is green. It MUST ride in
the same commit as the four tokens, which is what the census refuses a raise
without.

**The earlier figure of 755 in this spec was wrong**, derived from a working-tree
count taken while three sessions were editing. The file says so itself: a
working-tree count moves under whoever reads it, and the gate counts what git
HOLDS. Read the census output, never a remembered number.

**Do not re-attribute `reload-import-policy-no-bounce` to this spec.** A reviewer
built a `ze` with `forwardOrderHold()` hard-wired false and the test failed
identically. The `.ci` configures one peer and receives no UPDATE, so neither
rail executes. The cause is a config transaction rollback from another session.

**Commit scoping.** `peer.go` and `peer_run.go` carry the shutdown-Cease spec's
hunks too, and `session_connection.go` deletes `Session.Close` whose only caller
is the `cleanup()` body `peer_run.go` rewrites.

**No ordering of two commits is executable, so the two specs land in ONE
commit.** Thomas settled this on 2026-08-11. `peer.go` carries this spec's
`Peer.wakeForwardOverflow`, which calls `fwdPool.wakeOverflow`, and
`Peer.forwardOverflowPending`, which reads `fwdWorker.overflowPending` as a
pointer; the same file carries the shutdown spec's `Peer.stopping`,
`Peer.ShutdownNotify` and `Peer.sealSession`, and `peer_run.go` carries both
specs as well. `fwdPool.wakeOverflow` and `Session.seal` are both absent at
HEAD, so following either symbol outward pulls the other spec's producer in,
and git stages whole files. Either ordering therefore leaves HEAD unable to
build for everyone (`ai/rules/git-safety.md`, "Your Working Tree Is Not What
You Committed"). The single commit carries the union of both file lists, and
Files to Modify above names this spec's half of it.

**One commit, two specs, one review artifact each.** `review_gate_problems`
(`scripts/dev/commit_helper.py`) derives ONE spec stem and checks every code
file in the commit against that stem's artifact, and neither artifact covers
the union: each reviewer refused to hash the other spec's files into a verdict
it did not reach. The closure therefore passes `--review-override` naming both
artifacts, both `verdict=clean` lines and both round counts.

---

## Implementation Summary

### What Was Implemented

- **The readiness gate on both forwarding rails.** `Peer.forwardOrderHold`
  (`internal/component/bgp/reactor/peer.go`) answers "is this destination owed
  earlier route operations", from two atomics and no lock. `forwardUpdateCore`
  (`reactor_api_forward.go`) and `reactorForwardRS` (`forward_rs.go`) consult it
  per destination, the RS one ahead of `tryDirectWriteNoFlush`.
- **The overflow hold and its wake.** `overflowHeld` holds `drainOverflow`
  (`forward_pool.go`) while a destination is held; `fwdPool.wakeOverflow` and
  `Peer.wakeForwardOverflow` release it. The wake was not in the design and was
  found by measurement: a held worker's channel is empty by construction, so
  without it the forwarded withdraw never arrived at all.
- **The pending-overflow refusal.** `fwdWorker.overflowPending` is published to
  the destination peer (`Peer.fwdOverflowPending`) inside `overflowMu` and
  ahead of its own `Add(1)` (`forward_pool.go:759-794`), so the RS rail reads it
  with two atomic loads rather than a pool lock and a map hash (D-11).
- **`fwdReorderWithdrawalsFirst` DELETED**, with `fwdIsWithdrawal`,
  `fwdAttrsHaveReach`, `fwdAttrsHaveUnreach`, `fwdAttrsScanCode`,
  `fwdItem.withdrawal` and `fwdBodyResult.withdrawal`. Its precondition,
  per-prefix dedup, never held: Ze dedups by BYTES
  (`docs/architecture/congestion-industry.md`). Hoisting withdrawals inverts an
  announce and a withdraw of ONE prefix.
- **`fwdBucketMerge` is adjacency-only** (`forward_bucket.go`), and a run that
  packs no body now consumes nothing rather than being stripped with nothing
  emitted (defect 3: dropped MP_REACH and MP_UNREACH UPDATEs).
- **`ownOverflowBodies`** (`forward_body.go`) copies an overflow item's payload
  into the tier 2 handle it already holds, or onto the heap, so a safety-valve
  eviction of the recent-update cache entry cannot take the bytes out from under
  a queued item.

### Bugs Found/Fixed

- **The batch reorder inverted same-prefix order.** Covered by
  `TestFwdBatchKeepsQueuedOrder` (`forward_pool_supersede_test.go`) and
  `TestForwardedAnnounceThenWithdrawKeepOrder`.
- **`fwdBucketMerge` did the same by hash grouping.** Covered by
  `TestForwardBucketKeepsOrderAcrossAWithdraw` (`forward_bucket_test.go`), red
  before D-10 with the announce at index 3 and the withdraw at 1.
- **A merge run that packed no body dropped MP-only UPDATEs.** Covered by
  `TestForwardBucketKeepsMPOnlyBodies`; reverting the empty-run guard reads
  `got 0 of 2` for MP_REACH and again for MP_UNREACH.
- **An overflow item aliased buffers `evictLocked` returns.** Covered by the
  four tests in `forward_overflow_own_bytes_test.go`.
- **The first ordering gate closed only half the window** (round 5): the count
  was decremented per enqueue while the worker takes `session.writeMu` only on
  its next loop iteration. `runWorker` now re-derives it from `len(w.overflow)`
  once a batch completes with an empty channel. Covered by
  `TestForwardedUpdateWaitsForInFlightOverflowRSRail`.
- **The gate took a pool lock and a map hash on the rail that exists to bypass
  the pool** (round 6). D-11 moved the read to the destination peer's own
  atomics. Measured: 38.16 ns/op -> 0.61 ns/op single-goroutine, 63.77 ns/op ->
  0.097 ns/op with 29 goroutines.

### Documentation Updates

- `docs/architecture/forward-congestion-pool.md` - the reorder it documented is
  gone; the merge is adjacency-only.
- `docs/architecture/congestion-industry.md` - states the per-prefix-dedup
  precondition the deleted reorder needed and that Ze does not meet it, and the
  "no sooner" flush justification now names its exception (`bufWriter` is a 16K
  `bufio.Writer`, `batchLimit` defaults to 1024, so a large batch auto-flushes
  mid-batch). Both figures read at their producers: `session_connection.go`
  sizes `bufWriter` with `max(env.GetInt("ze.buf.write.size", 16384), 4096)`,
  `reactor.go` sets `batchLimit` from `max(env.GetInt("ze.fwd.batch.limit", 1024), 0)`.
- `plan/known-failures/bgp-plugin-rs-forward-duplicate-and-order.md` - two rows
  of its withdrawals-first table named symbols D-9 deleted.
- `make ze-doc-test` NOT run in this phase: a QEMU run holds the tree and the
  owner directed that no suite start. The doc edits are prose in two
  architecture pages and one known-failure shard; no source anchor was added.

### Deviations from Plan

- The spec planned a defer-on-the-same-FIFO design (D-1..D-4). What shipped
  defers at DISPATCH instead, because `opQueue` holds structured ops and cannot
  carry a wire body. The hold lives in the pool worker, not in `opQueue`.
- Phase 3 was not in the original plan at all: deleting
  `fwdReorderWithdrawalsFirst` was found while implementing Phase 2.
- The two commits this spec assumed became ONE. See the closing section.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | The first gate was designed to close the window and shipped closing half of it: `overflowPending` was decremented as each item entered `w.ch`, while the worker takes `session.writeMu` only on its next loop iteration | The first moment every released item is provably written is a batch completing with an empty channel, so the count must be RE-DERIVED there, not decremented at enqueue | Round 5 review, then `TestForwardedUpdateWaitsForInFlightOverflowRSRail` | Count re-derived in `runWorker` (`forward_pool.go:1131`) and in `drainOverflow`'s stopped branch (`:1229`) |
| approach | The gate was read through the pool: `fp.mu.RLock` plus a `fwdKey` map hash per destination per UPDATE, on the rail whose whole purpose is to bypass the pool | The destination `*Peer` is already in hand at the call site and can hold the counter itself | Round 6 review, then `BenchmarkReactorForwardRS` | D-11: `Peer.fwdOverflowPending`, published inside `overflowMu` ahead of the `Add`. 38.16 -> 0.61 ns/op, 63.77 -> 0.097 ns/op at 29 goroutines |
| assumption | `fwdReorderWithdrawalsFirst` was assumed correct because it only ever moved withdrawals EARLIER | That is safe only under per-prefix dedup, and Ze dedups by bytes, so it inverts an announce and a withdraw of ONE prefix | Reading `docs/architecture/congestion-industry.md` for the precondition rather than the code for the effect | The function and its five helpers deleted; `TestFwdBatchKeepsQueuedOrder` replaces the five AC-25 tests |
| escalation | A count of 755 relax tokens was carried in this spec for a day and was wrong | 748 at HEAD. The figure had been taken from the working tree while three sessions were editing | `scripts/dev/relax-census.py`, which prints HEAD and working tree separately | The spec now says to read the census output and never a remembered number |

## Implementation Audit

### Requirements from Task

| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| The forwarding rail consults peer readiness before dispatching | Done | `Peer.forwardOrderHold` (`peer.go`), read in `forwardUpdateCore` (`reactor_api_forward.go`) and `reactorForwardRS` (`forward_rs.go`) | Four atomic loads per destination per UPDATE, no lock |
| A held destination's items are released in insertion order | Done | `overflowHeld` + the hold at the top of `drainOverflow`, `wakeOverflow` (`forward_pool.go`), `Peer.wakeForwardOverflow` (`peer.go`) | The wake is beside each of the four stores that clear `sendingInitialRoutes` (`peer_initial_sync.go`) and the session-teardown store (`peer_run.go`) |
| The fast path does not block | Done | `forwardOrderHold` takes no lock; the RS pending-overflow clause adds two atomic loads (D-11) | `BenchmarkFwdPoolTryDispatch` 0 allocs/op, unchanged by D-11 |
| No batching stage may move a route past another for the same prefix | Done | `fwdReorderWithdrawalsFirst` deleted (`forward_pool.go`); `fwdBucketMerge` adjacency-only (`forward_bucket.go`) | Correctness of same-prefix order is not tradeable against message count |
| A queued overflow item owns its bytes | Done | `ownOverflowBodies` (`forward_body.go`), called from `DispatchOverflow` (`forward_pool.go:749`) | Extended to `item.updates` on the cross-context rail |

### Acceptance Criteria

| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestForwardedWithdrawWaitsForQueuedAnnounceCoreRail`, `...RSRail`, `TestForwardedAnnounceThenWithdrawKeepOrder`, `TestForwardedUpdateWaitsForPendingOverflowRSRail`, `TestForwardedUpdateWaitsForInFlightOverflowRSRail` (`forward_initial_sync_order_test.go`), `TestFwdBatchKeepsQueuedOrder`, `TestForwardBucketKeepsOrderAcrossAWithdraw`, `TestForwardBucketKeepsMPOnlyBodies` | Each asserts the BYTES the destination's connection received. Each of the three pieces measured load-bearing on its own |
| AC-2 | Done | `TestOverflowItemSurvivesSafetyValveEviction`, `TestOverflowItemOwnsBytesWithoutAHandle`, `TestOverflowItemOwnsBytesLargerThanOneHandle`, `TestOverflowItemOwnsUpdateSections` (`forward_overflow_own_bytes_test.go`) | All four red without `ownOverflowBodies` |

### Tests from TDD Plan

| Test | Status | Location | Notes |
|------|--------|----------|-------|
| The four AC-2 tests | Done | `forward_overflow_own_bytes_test.go` | `grep -c '^func Test'` = 4 |
| The five AC-1 rail tests | Done | `forward_initial_sync_order_test.go` | `grep -c '^func Test'` = 5 |
| `TestFwdBatchKeepsQueuedOrder` | Done | `forward_pool_supersede_test.go` | Replaces the five AC-25 tests of the deleted reorder |
| `TestForwardBucketKeepsOrderAcrossAWithdraw`, `TestForwardBucketKeepsMPOnlyBodies` | Done | `forward_bucket_test.go` | Round-3 D-10 defects 1 and 3 |
| `forward-overflow-two-tier`, `role-otc-rs-withdraw-eor` | Done | `test/plugin/` | Regressions, not written for this spec; both untouched by it |
| `forward-order-during-initial-sync.ci` | Skipped | -- | Deliberate and recorded in the TDD plan: the only config lever that widens the window is the `apiSyncExpected` 500ms hold, which is the widening that made `role-otc-rs-withdraw-eor.ci` deliver a relayed route twice. The two unit tests hold the window open deterministically and read the same wire bytes |

### Files from Plan

| File | Status | Notes |
|------|--------|-------|
| Every file in Files to Modify | Done | 15 files, each hash-pinned by the review artifact |
| `internal/component/bgp/reactor/session_rfc7606_diagnostics_test.go` | Changed | Not this spec's product change. Rides because leaving it out reds `ze-race-reactor` at HEAD and `rfc/audit/rfc7606.json` pins its content |

### Audit Summary

- **Total items:** 2 acceptance criteria, 13 tests, 5 task requirements
- **Done:** all except the one Skipped below
- **Partial:** none
- **Skipped:** `forward-order-during-initial-sync.ci`, with the reason recorded in the TDD plan and the coverage carried by two deterministic unit tests
- **Changed:** 1 file (`session_rfc7606_diagnostics_test.go`, recorded in Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| A forwarded UPDATE never overtakes a route already queued for the same peer, so no peer is left holding a stale route | functional (wire-byte unit tests over both rails) | `TestForwardedWithdrawWaitsForQueuedAnnounceCoreRail` and `...RSRail` assert the octets the destination's connection received. With the dispatch gates removed the withdraw arrives FIRST on both rails (announce at index 1, withdraw at 0); with only the `drainOverflow` hold removed the withdraw reaches the wire before the sync runs; with only the wake removed it never arrives |
| No batching stage reorders two operations on one prefix | functional | `TestForwardBucketKeepsOrderAcrossAWithdraw` is red before D-10 (announce at index 3, withdraw at 1). `TestForwardedAnnounceThenWithdrawKeepOrder` is red before D-9 (announce at wire index 2, withdraw at 1) |
| The route-server fast path stays a fast path | benchmark | The D-11 hot gate: 38.16 -> 0.61 ns/op single-goroutine, 63.77 -> 0.097 ns/op with 29 goroutines. `BenchmarkFwdPoolTryDispatch` 0 allocs/op, `BenchmarkReactorForwardRS` 28, `BenchmarkForwardBucketDrain` 8, all three unchanged by D-11 |
| No forwarded route is dropped by the merge | functional | `TestForwardBucketKeepsMPOnlyBodies`: reverting the empty-run guard reads `got 0 of 2` for MP_REACH and again for MP_UNREACH |
| No data race in the reactor's forwarding path | race detector | `make ze-race-reactor` clean, `-race -count=20`, 373.201s. It covers the redefined `overflowPending` and, from the sibling spec, `Peer.stopping`, `Reactor.stopping`, `Session.seal` and `Peer.sealSession`. Every reactor file in the tree predates that run |

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| This spec has no deferral shard | n/a | No shard named for this spec stem exists under `plan/deferrals/`; nothing to remove |
| `plan/deferrals/ad-hoc-2026-07-27-dd843d81.md`, the `as112-community-choice.ci` End-of-RIB row, whose Destination cell names this spec | deferred | Not resolved here and NOT this spec's work: it is an RFC 4724 question about when a speaker may emit End-of-RIB relative to its initial announcements, and the fixture may not be changed without owner approval. The row stays live in a shard this closure does not remove |
| `plan/journal/gate-excludes-part-of-its-population.md`, this spec's row (the RS rail's gate covers overflow and not the pool CHANNEL) | deferred | The row is tracked at HEAD. It records a rail this spec did not close: `overflowPending` is raised only by `DispatchOverflow`, so an item that reached `w.ch` through `TryDispatch` leaves it at zero. Both the direct write and the `TryDispatch` fallback predate this spec's gate, and a count covering channel items needs its own tests and its own review |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/fixit-forward-rail-initial-sync-ordering-640fa955-f03a-45e8-a58f-4b367f5859e6.md` |
| `review_gate.py check` | clean -- `review_gate: OK (0 code files, clean, hashes match ...)`, `verdict=clean rounds=8`, 15 files hash-pinned |
| Rounds | 8. Round 5 found a PRODUCT defect in the shipped gate (`overflowPending` decremented at enqueue, so `reactorForwardRS`'s direct write overtook items the hold had just released). Round 6 found a second (the gate took `fp.mu.RLock` plus a `fwdKey` map hash per destination per UPDATE on the rail that exists to bypass the pool). Round 7 verified both fixes and found the stated commit plan unexecutable. Round 8 re-verified the same code under the settled one-commit plan |
| Reviewer lenses used | the count and its four mutation sites, publish-before-`Add`, `Stop`'s zero, whether the gate's read is load-bearing, relax census and ceiling, race evidence, the file list for the combined commit |

### Findings fixed

| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | BLOCKER | The ordering gate closed half the window: `drainOverflow` decremented `overflowPending` per enqueue, and the worker takes `session.writeMu` only on its next loop iteration | `internal/component/bgp/reactor/forward_pool.go` | The count re-derived from `len(w.overflow)` when a batch completes with an empty channel. `TestForwardedUpdateWaitsForInFlightOverflowRSRail` |
| 2 | ISSUE | This spec's four `test-relax:` tokens took the census over its ceiling | `test/relax-ceiling.txt` | Ceiling raised 748 -> 752 with a `raised-for:` line naming which four tests lost what. The file rides with the four tokens, which is what the census refuses a raise without |
| 3 | ISSUE | The ordering gate put a pool lock and a map hash on the route-server rail | `internal/component/bgp/reactor/forward_rs.go`, `forward_pool.go`, `peer.go` | D-11: the count published to `Peer.fwdOverflowPending` inside `overflowMu` ahead of the `Add`. 38.16 -> 0.61 ns/op, 63.77 -> 0.097 ns/op at 29 goroutines |
| 4 | ISSUE | Defect 3 (dropped MP-only UPDATEs) had no regression test | `internal/component/bgp/reactor/forward_bucket_test.go` | `TestForwardBucketKeepsMPOnlyBodies` |
| 5 | ISSUE | `hasOverflowPending` had no test driving it from `reactorForwardRS` | `internal/component/bgp/reactor/forward_initial_sync_order_test.go` | `TestForwardedUpdateWaitsForPendingOverflowRSRail` |
| 6 | ISSUE | `docs/architecture/congestion-industry.md` published an absolute "no sooner" flush justification that is false for a large batch | `docs/architecture/congestion-industry.md` | The paragraph states the exception and keeps the verdict. Both figures read at their producers |
| 7 | BLOCKER | The stated commit plan was unexecutable: neither ordering of the two specs' commits builds | this spec's closing section | One commit carrying the union, settled by Thomas 2026-08-11 |

## Pre-Commit Verification

### Files Exist (ls)

| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/bgp/reactor/forward_initial_sync_order_test.go` | yes | `ls -l` 22979 bytes, `grep -c '^func Test'` = 5 |
| `internal/component/bgp/reactor/forward_overflow_own_bytes_test.go` | yes | `ls -l` 8950 bytes, `grep -c '^func Test'` = 4 |
| `test/relax-ceiling.txt` | yes | `git diff` shows 748 -> 752 with the `raised-for:` block |
| The other 13 files in Files to Modify | yes | all tracked and modified; each hash-pinned in the review artifact and `review_gate.py check` reports `hashes match` |

### AC Verified (grep/test)

| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | Both rails hold a destination that is owed earlier operations, and every batching stage preserves same-prefix order | `grep -n '^func Test' internal/component/bgp/reactor/forward_initial_sync_order_test.go` lists all five AC-1 tests at lines 270, 292, 320, 428, 517. `make ze-race-reactor` ran them clean under `-race -count=20` in 373.201s |
| AC-2 | A queued overflow item owns its bytes | `grep -n '^func Test' internal/component/bgp/reactor/forward_overflow_own_bytes_test.go` lists all four at lines 85, 168, 197, 226; `ownOverflowBodies(&item)` is called at `forward_pool.go:749`, inside `DispatchOverflow` and ahead of the `overflowMu` hold |

### Wiring Verified (end-to-end)

| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| announce queued for a peer in initial sync, then a withdraw for the same prefix forwarded | no `.ci` -- `TestForwardedWithdrawWaitsForQueuedAnnounceCoreRail` and `...RSRail` | yes. Deliberate, and the reason is in the TDD plan: the only config lever that widens the destination's initial-sync window is the `apiSyncExpected` 500ms hold, which is the widening that made `role-otc-rs-withdraw-eor.ci` deliver a relayed route twice on 2026-08-08. Both unit tests assert on the octets the destination's connection received, which is what a `.ci` would read |
| 50 UPDATEs burst through a 2-deep channel arrive in order, no handle leak | `test/plugin/forward-overflow-two-tier.ci` | yes, read: it bursts through the tier-2 path this spec's hold sits on, and calls `request peer * flush` after establishment. Regression cover, untouched by this spec |
| a relayed route is delivered ONCE across the initial-sync window | `test/plugin/role-otc-rs-withdraw-eor.ci` | yes, read: it is the test that refused the earlier barrier attempt, so it is the guard against re-introducing one. Untouched by this spec |

### Assumptions Resolved

| ID | Final Status | Evidence |
|----|--------------|----------|
| none | n/a | This spec carries no `## Risks & Assumptions` section and no A-N rows. The design questions it did carry are D-1..D-11, each answered in the Design analysis and in Known Limitations |
| D-11 (the gate's read must not cost the fast path) | confirmed | 38.16 -> 0.61 ns/op single-goroutine, 63.77 -> 0.097 ns/op at 29 goroutines; three benchmarks unchanged in allocs/op |
| The reorder's precondition (per-prefix dedup) | **broken** | `docs/architecture/congestion-industry.md` states it, and Ze dedups by BYTES. Recorded in the Mistake Log; `fwdReorderWithdrawalsFirst` deleted |

### Documentation Verified

| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| `docs/architecture/forward-congestion-pool.md` no longer documents a reorder that does not exist | `gopls symbols internal/component/bgp/reactor/forward_pool.go` shows no `fwdReorderWithdrawalsFirst`; the doc's merge section describes the adjacency-only rule | yes |
| `docs/architecture/congestion-industry.md`'s flush justification | `session_connection.go` sizes `bufWriter` with `max(env.GetInt("ze.buf.write.size", 16384), 4096)`; `reactor.go` sets `batchLimit` from `max(env.GetInt("ze.fwd.batch.limit", 1024), 0)` | yes, both read at their producers |
| `plan/known-failures/bgp-plugin-rs-forward-duplicate-and-order.md`'s withdrawals-first table | its two rows named `fwdReorderWithdrawalsFirst` and `fwdItem.withdrawal`, both deleted by D-9; they now name `safeBatchHandle` and the adjacency-only `fwdBucketMerge` | yes |
| No user guide, CLI, config, YANG, RPC or plugin surface changed | the diff is 15 reactor files, two architecture pages, one known-failure shard and the relax ceiling; no `.yang`, no `ze:command`, no env var added | yes |

### What was NOT run, and why

- **`make ze-verify` was NOT run.** A QEMU run holds this shared checkout and the
  owner directed that no suite start. This checkout is shared by several
  sessions, so a fully green `ze-verify` is unreachable by construction
  (`ai/rules/git-safety.md`). The commit is prepared with `--unverified`.
- **No end-to-end `make ze-qemu-needs-linux-test` has completed on this
  machine.** Nothing here claims that target passes.
- **`make ze-doc-test`, `make ze-lint-changed` and `make ze-plugin-test` were
  NOT run in this phase**, for the same reason.
- **What WAS run:** `make ze-race-reactor`, clean, `-race -count=20`, 373.201s,
  with every reactor file in the tree older than that run.
  `python3 scripts/dev/relax-census.py`: 748 at HEAD, ceiling 752, working tree
  752. `python3 scripts/dev/review_gate.py check` on both specs: OK, clean,
  hashes match. `python3 scripts/dev/spec-citation-check.py`: OK.

## Core Insight

**A reordering stage is only safe under the dedup key it was written for.**
`fwdReorderWithdrawalsFirst` moved withdrawals earlier and never later, which
reads as monotone and safe, and its own comment said so. It is safe only when a
queue holds at most one item per PREFIX. Ze's forward pool dedups by BYTES, so
two operations on one prefix coexist in a batch and hoisting one past the other
leaves the peer holding a route it withdrew. The defect was found by reading the
document that stated the precondition, not by reading the function that broke
it. The same question found the same inversion one layer down in
`fwdBucketMerge`, and a third defect beside it.
