# Spec: forward-pool

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/encoding-context.md` - zero-copy forwarding rule
4. `internal/plugins/bgp/reactor/reactor.go:3178-3314` - ForwardUpdate (the bottleneck)
5. `internal/plugins/bgp/reactor/recent_cache.go` - Retain/Release API
6. `internal/plugins/bgp-rr/worker.go` - existing per-source-peer pool (pattern to follow)

## Task

`ForwardUpdate` in the reactor iterates destination peers sequentially. Each `SendRawUpdateBody` or `SendUpdate` does a TCP write that blocks if the kernel send buffer is full (slow destination peer). One slow peer blocks forwarding to ALL other peers in that batch.

Add a per-destination-peer forward pool to the reactor so that TCP sends happen in parallel across destination peers, using long-lived worker goroutines (not per-event `go func()`).

## Required Reading

### Architecture Docs
- [x] `docs/architecture/core-design.md` - engine/plugin boundary, cache system
  → Decision: Engine handles BGP protocol; plugins decide WHAT to forward
  → Constraint: CacheConsumer plugins must forward or release every cached UPDATE
- [x] `docs/architecture/encoding-context.md` - zero-copy forwarding
  → Decision: If sourceCtxID == destCtxID, return cached wire bytes directly (zero-copy)
  → Constraint: Zero-copy must be preserved
- [x] `.claude/rules/goroutine-lifecycle.md` - goroutine patterns
  → Constraint: Long-lived workers reading from channels. Never per-event goroutines in hot path.
- [x] `.claude/rules/plugin-design.md` - SDK callback pattern
  → Constraint: ForwardUpdate is called via RPC from plugin — must return promptly

### RFC Summaries (MUST for protocol work)
- [x] `rfc/short/rfc8654.md` - Extended message support
  → Constraint: UPDATE splitting when message exceeds peer's max size (4096 or 65535)

**Key insights:**
- ForwardUpdate is called via plugin SDK RPC — making it async means the RPC returns faster
- Cache has `Retain()` / `Release()` reference counting (`retainCount` in `cacheEntry`) — keeps buffer alive for async sends without copying
- The bgp-rr `workerPool` in `worker.go` is the exact pattern to follow (per-key workers, lazy creation, idle timeout, blocking send, backpressure)
- TCP write errors trigger FSM state changes (peer disconnect) — the plugin does not need synchronous error return
- `parsedUpdate` (`message.Update`) has `[]byte` sub-slices referencing the cache buffer — Retain keeps them valid

## Current Behavior (MANDATORY)

**Source files read:**
- [x] `internal/plugins/bgp/reactor/reactor.go:3178-3314` - `ForwardUpdate`: iterates matching peers, three forwarding paths (wire split, zero-copy, re-encode), collects errors, returns joined errors
- [x] `internal/plugins/bgp/reactor/peer.go:1273-1350` - `SendUpdate()` and `SendRawUpdateBody()` delegate to session (TCP write)
- [x] `internal/plugins/bgp/reactor/recent_cache.go` - `RecentUpdateCache`: `Get()` for read, `Ack()` for plugin tracking, `Retain()`/`Release()` for API-level refcount
- [x] `internal/plugins/bgp-rr/worker.go` - `workerPool`: per-source-peer goroutines, lazy creation, idle timeout, blocking send with pending counter, backpressure detection
- [x] `internal/plugins/bgp-rr/server.go:397-410` - `forwardUpdate()`: selects targets, issues single `cache N forward peer1,peer2,...` RPC

**Behavior to preserve:**
- Three forwarding paths in ForwardUpdate (wire split, zero-copy, re-encode)
- Selector matching and source-peer exclusion
- CacheConsumer protocol: every UPDATE must be forwarded or released
- Zero-copy when source and destination encoding contexts match
- FIFO ordering per destination peer (per-peer channel guarantees this)
- Cache Ack semantics for plugin FIFO tracking
- RFC 8654 UPDATE splitting for oversized messages

**Behavior to change:**
- ForwardUpdate sends to peers in parallel (via per-peer channels) instead of sequentially
- ForwardUpdate returns immediately after dispatching (no synchronous error collection from TCP writes)
- Send errors logged by workers instead of returned to caller

## Data Flow (MANDATORY)

### Entry Point
- Plugin issues `cache N forward peer1,peer2,...` RPC
- Reaches `ForwardUpdate(sel, updateID, pluginName)` in reactor

### Transformation Path

**Current (synchronous):**
1. Cache Get(updateID) + defer Ack
2. Select matching established peers (RLock on reactor.peers)
3. For each peer (sequential): compute send operations (split/zero-copy/re-encode), call Send* (TCP write blocks)
4. Collect errors, return joined errors

**Proposed (async per peer):**
1. Cache Get(updateID) + defer Ack (unchanged)
2. Select matching established peers (unchanged)
3. For each peer: compute send operations (unchanged — CPU work, fast)
4. For each peer: Retain(updateID), dispatch pre-computed ops to per-peer forward worker
5. Return immediately (no TCP writes in ForwardUpdate)
6. Worker: execute Send* calls sequentially for this peer, then Release(updateID)
7. Worker: log errors (TCP failures trigger FSM disconnect independently)

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| Plugin → Engine | SDK RPC `cache forward` — unchanged | [x] |
| ForwardUpdate → forward worker | Per-peer buffered channel (new) | [x] |
| Cache → forward worker | Retain/Release keeps buffer alive | [x] |

### Integration Points
- `reactorAPIAdapter.ForwardUpdate()` — refactored to dispatch instead of send
- `Reactor.fwdPool` — new field, created in `New()`, stopped with reactor
- `RecentUpdateCache.Retain/Release` — existing API, used for buffer lifetime
- `Peer.SendRawUpdateBody()` / `Peer.SendUpdate()` — called by workers instead of ForwardUpdate

### Architectural Verification
- [x] No bypassed layers (ForwardUpdate still computes paths; workers only do TCP writes)
- [x] No unintended coupling (pool is a reactor-internal component)
- [x] No duplicated functionality (reuses same forwarding path logic, just async delivery)
- [x] Zero-copy preserved (Retain keeps cache buffer alive; no payload copying)

## Design Decisions

### FP-1: Per-destination-peer long-lived workers

One long-lived goroutine per destination peer address. Workers created lazily on first dispatch, exit after idle timeout. Same pattern as `bgp-rr/worker.go` but keyed by destination peer (not source peer).

| Event | Action |
|-------|--------|
| First dispatch to a peer | Create worker goroutine + buffered channel |
| Subsequent dispatches | Enqueue to existing channel (block if full) |
| No dispatches for idle timeout | Worker exits; removed from pool |
| Pool shutdown | Close all channels; workers drain and exit |

### FP-2: Retain/Release for buffer lifetime

ForwardUpdate calls `Retain(updateID)` once per destination peer before dispatching. Each worker calls `Release(updateID)` after completing all send operations for that batch. This keeps the cache buffer alive without copying payload data.

| State | Buffer Alive? | Why |
|-------|---------------|-----|
| ForwardUpdate running | Yes | Cache entry exists, Ack deferred |
| After ForwardUpdate returns (Ack fires) | Yes | Ack decrements pendingConsumers, but retainCount > 0 keeps entry alive |
| Worker completes sends | Depends | Release decrements retainCount; buffer freed when totalConsumers() reaches 0 |
| All workers done | No | totalConsumers() = (pendingConsumers after Ack) + 0 retains = 0, entry evicted |

Note: `defer Ack` fires when ForwardUpdate returns — before workers finish sending. This is correct because `totalConsumers() = pendingConsumers + retainCount`. The Ack decrements pendingConsumers while Retain keeps retainCount > 0, preventing premature eviction. The entry is only evicted when the last worker calls Release and totalConsumers() reaches 0.

### FP-3: Pre-computed send operations

ForwardUpdate computes all split/re-encode decisions synchronously (fast CPU work). Workers receive pre-computed operations and only do TCP writes.

| Operation type | What worker does |
|----------------|-----------------|
| Raw body (zero-copy or split) | `peer.SendRawUpdateBody(body)` |
| Parsed update (re-encode) | `peer.SendUpdate(update)` |

This keeps the complex context-dependent logic in one place (ForwardUpdate) and makes workers simple.

### FP-4: Fire-and-forget error semantics

ForwardUpdate returns nil (or error only for cache miss / no matching peers). Per-peer TCP write errors are logged by workers but not propagated to the caller.

| Error type | Current behavior | New behavior |
|-----------|------------------|--------------|
| Cache miss | Return ErrUpdateExpired | Unchanged |
| No matching peers | Return error | Unchanged |
| TCP write failure | Return joined errors | Worker logs warning; FSM handles disconnect |
| Split failure | Return error for that peer | Worker logs warning; skip remaining ops for that peer |

Rationale: By the time a TCP write fails, the RPC caller (plugin) has already moved to the next work item. TCP failures trigger FSM state transitions (peer disconnect) independently — the plugin learns about it via a state event, not a send error.

### FP-5: Blocking send with stopCh escape

When a worker's channel is full (destination peer is slow), Dispatch blocks. This applies backpressure to the ForwardUpdate caller — but only for that one peer. Other peers in the same batch are dispatched immediately.

A `stopCh` channel prevents deadlock during shutdown: if Stop() is called while Dispatch is blocked on a full channel, the stopCh select case unblocks it and Release is called.

### FP-6: parsedUpdate sharing across workers

When ForwardUpdate lazily parses an UPDATE for the re-encode path, the same `*message.Update` may be dispatched to multiple workers. This is safe because `SendUpdate` only reads from the Update's byte slices (via `WriteTo`). No mutation occurs.

The Update's internal byte slices reference the cache buffer, which remains valid due to Retain (FP-2).

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | ForwardUpdate with 3 matching peers, 1 slow | Slow peer's worker blocks independently; other 2 peers receive UPDATE without waiting |
| AC-2 | ForwardUpdate zero-copy path | Raw payload dispatched to worker; worker calls SendRawUpdateBody; no payload copy |
| AC-3 | ForwardUpdate re-encode path | parsedUpdate dispatched to worker; worker calls SendUpdate |
| AC-4 | ForwardUpdate wire-split path | Split pieces dispatched as separate raw ops; worker sends each piece |
| AC-5 | Cache buffer lifetime | Buffer remains valid while workers are sending (Retain/Release) |
| AC-6 | Worker idle timeout | Worker exits after idle period; removed from pool; no goroutine leak |
| AC-7 | Pool shutdown | All workers drain remaining items and exit; no goroutine leak |
| AC-8 | Dispatch to stopped pool | Returns false; Release called to free cache ref |
| AC-9 | FIFO ordering per peer | Multiple dispatches to same peer delivered in order |
| AC-10 | Backpressure on full channel | Dispatch blocks until worker drains; does not drop items |
| AC-11 | TCP write error in worker | Error logged; remaining ops for that peer skipped; Release still called |
| AC-12 | All existing tests pass | No regression in unit or functional tests |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestFwdPool_LazyCreation` | `reactor/forward_pool_test.go` | Worker created on first Dispatch; WorkerCount reflects (AC-1) | |
| `TestFwdPool_IdleTimeout` | `reactor/forward_pool_test.go` | Worker exits after idle; WorkerCount decrements (AC-6) | |
| `TestFwdPool_Stop` | `reactor/forward_pool_test.go` | All workers drain and exit on Stop (AC-7) | |
| `TestFwdPool_DispatchAfterStop` | `reactor/forward_pool_test.go` | Returns false; done callback still called (AC-8) | |
| `TestFwdPool_FIFOPerPeer` | `reactor/forward_pool_test.go` | Items for same peer processed in dispatch order (AC-9) | |
| `TestFwdPool_ParallelPeers` | `reactor/forward_pool_test.go` | Slow peer does not block fast peer (AC-1) | |
| `TestFwdPool_BackpressureBehavior` | `reactor/forward_pool_test.go` | Full channel blocks Dispatch; does not drop (AC-10) | |
| `TestFwdPool_HandlerError` | `reactor/forward_pool_test.go` | Error in handler does not kill worker; Release still called (AC-11) | |
| `TestFwdPool_StopUnblocksDispatch` | `reactor/forward_pool_test.go` | Stop closes stopCh; blocked Dispatch returns false | |
| `TestForwardUpdate_DispatchesToPool` | `reactor/forward_update_test.go` | ForwardUpdate dispatches to pool instead of calling Send directly | |
| `TestForwardUpdate_RetainRelease` | `reactor/forward_update_test.go` | Retain called per peer; Release called after worker completes (AC-5) | |

### Boundary Tests (MANDATORY for numeric inputs)

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| chanSize | 1+ | 1 | 0 (defaults to 64) | N/A |
| idleTimeout | 1ns+ | 1ns | 0 (defaults to 5s) | N/A |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| Existing `make ze-functional-test` | `test/` | All 246+ functional tests still pass | |

### Future (if deferring any tests)
- Benchmark comparing sequential vs pooled ForwardUpdate under slow-peer conditions — deferred until functional correctness proven

## Files to Modify

- `internal/plugins/bgp/reactor/reactor.go` - ForwardUpdate refactored to dispatch ops to pool; Reactor struct gets `fwdPool` field; `New()` creates pool; `cleanup()` stops pool in Phase 1 (before peer wait)
- ~~`internal/plugins/bgp-rr/worker.go` - Backpressure rate-limiting fix (`LoadOrStore` instead of `Store`)~~ Already applied in dd3c15ac

### Integration Checklist

| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | |
| RPC count in architecture docs | No | |
| CLI commands/flags | No | |
| CLI usage/help text | No | |
| API commands doc | No | `cache forward` unchanged |
| Plugin SDK docs | No | No SDK changes |
| Editor autocomplete | No | |
| Functional test for new RPC/API | No | No new RPCs |

## Files to Create

- `internal/plugins/bgp/reactor/forward_pool.go` - `fwdPool` type: per-destination-peer workers, lazy creation, idle timeout, blocking send, Retain/Release integration
- `internal/plugins/bgp/reactor/forward_pool_test.go` - Pool lifecycle tests, FIFO, backpressure, parallel peers, error handling
- `internal/plugins/bgp/reactor/forward_update_test.go` - Integration tests for ForwardUpdate dispatching to pool

## Implementation Steps

Each step ends with a **Self-Critical Review**. Fix issues before proceeding.

### Phase 1: Forward pool infrastructure (testable in isolation)

1. **Write pool tests** — lazy creation, idle timeout, stop, FIFO ordering, parallel peers, backpressure, error handling, stop-unblocks-dispatch
   → **Review:** Tests use race detector? Idle timeout testable with short duration? Handler errors tested?

2. **Run tests** — verify FAIL (paste output)
   → **Review:** Tests fail for the right reason (missing types)?

3. **Implement `fwdPool`** in `forward_pool.go` — per-peer workers with buffered channels, lazy creation, idle timer, blocking send with stopCh escape, done-callback for Release
   → **Review:** Goroutine lifecycle compliant? No `go func()` per Dispatch? Pending counter prevents idle-exit race?

4. **Run tests** — verify PASS (paste output)
   → **Review:** Race detector clean? No goroutine leaks?

### Phase 2: Integrate into ForwardUpdate

5. **Write ForwardUpdate integration tests** — verify dispatch to pool, Retain/Release lifecycle, cache miss handling
   → **Review:** Tests mock pool and cache? Verify Retain count matches peer count?

6. **Refactor ForwardUpdate** — compute ops synchronously, Retain per peer, dispatch to pool, return immediately for send errors
   → **Review:** Same three forwarding paths preserved? Ack still in defer? Release in worker done-callback?

7. **Add fwdPool to Reactor** — create in `New()`, stop in `cleanup()` Phase 1 before peer Stop calls
   → **Review:** `fwdPool.Stop()` added to `cleanup()` (reactor.go:4738) before `peer.Stop()` loop? This avoids log noise from workers hitting ErrInvalidState on already-stopped sessions.

8. **Run tests** — verify PASS
   → **Review:** All existing reactor tests still pass?

### Phase 3: Verification

9. **Run full verification** — `make ze-verify`
   → **Review:** Zero lint issues? No regressions?

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Step 3 or 6 (fix types) |
| Test fails wrong reason | Step 1 or 5 (fix test) |
| Race detector failure | Step 3 or 6 (fix synchronization) |
| Existing test regression | Step 6 (ForwardUpdate refactor broke something) |
| Lint failure | Fix inline |

## Mistake Log

### Wrong Assumptions

| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches

| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|
| Per-event `go func()` with Retain/Release | Violates goroutine-lifecycle rule — per-event goroutines in hot path | Per-destination-peer long-lived workers (this spec) |

### Escalation Candidates

| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

- **Retain/Release eliminates copying:** The cache's API-level reference counting (`retainCount`) was designed for the `bgp cache N retain` API command, but it's equally useful for keeping buffers alive during async forwarding. No need to copy 4KB payloads.
- **ForwardUpdate's CPU work is fast; TCP writes are slow:** Split computation, context comparison, and lazy parsing are microsecond operations. The bottleneck is `net.Conn.Write()` which can block for milliseconds when the kernel send buffer is full. Separating compute from I/O is the right decomposition.
- **Error semantics change is safe:** The plugin (bgp-rr) does not act on per-peer send errors from ForwardUpdate. TCP failures trigger FSM state transitions that the plugin observes via separate state events. Dropping synchronous error return simplifies the async model without losing functionality.

## RFC Documentation

No new RFC constraints. Existing RFC 8654 splitting logic is unchanged — just called from workers instead of ForwardUpdate.

## Implementation Summary

### What Was Implemented
- `fwdPool` type in `forward_pool.go`: per-destination-peer long-lived workers with lazy creation, idle timeout, blocking send with `stopCh` escape, `dispatchWG` for concurrent Dispatch/Stop safety, `safeHandle` with panic recovery + guaranteed done callback
- `fwdHandler` in `forward_pool.go`: executes pre-computed `rawBodies` (SendRawUpdateBody) and `updates` (SendUpdate), logs errors, skips remaining ops on first error
- `ForwardUpdate` refactored to pre-compute send operations, Retain per peer, dispatch to `fwdPool`, Release in done callback
- `fwdPool` field added to `Reactor` struct, created in `New()`, stopped in `cleanup()` Phase 1
- `SetClock` propagation from Reactor to fwdPool (sim.Clock pattern)
- 10 unit tests covering pool lifecycle, FIFO, parallelism, backpressure, error handling, shutdown

### Bugs Found/Fixed
- DATA RACE between `w.ch <- item` (Dispatch) and `close(w.ch)` (Stop) — fixed with `dispatchWG sync.WaitGroup` tracking in-flight Dispatches; Stop waits for all to exit before closing channels
- `sim.Clock` audit test failure — `time.NewTimer()` is forbidden in reactor code; replaced with `fp.clock.NewTimer()` using sim.Timer interface

### Documentation Updates
- None (no new RPCs, CLI, or SDK changes)

### Deviations from Plan
- `forward_update_test.go` created with 3 integration tests (DispatchesToPool, RetainRelease, DispatchToStoppedPool) — covers AC-2, AC-5, AC-8 end-to-end through ForwardUpdate
- `fwdPool` uses `dispatchWG` for concurrent safety — not in original design but required because Dispatch is called from multiple RPC goroutines (unlike bgp-rr's single-goroutine constraint)
- Split failures in ForwardUpdate logged as warnings and peer skipped (instead of collecting errors) — consistent with FP-4 fire-and-forget semantics

## Implementation Audit

### Requirements from Task

| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Per-destination-peer forward pool | ✅ Done | `forward_pool.go:70-95` | fwdPool type with per-key workers |
| Long-lived worker goroutines | ✅ Done | `forward_pool.go:226-267` | runWorker with idle timeout |
| ForwardUpdate dispatches to pool | ✅ Done | `reactor.go:3228-3319` | Pre-compute ops, Retain, Dispatch |
| Retain/Release for buffer lifetime | ✅ Done | `reactor.go:3309-3310` | Retain before dispatch, Release in done callback |
| Fire-and-forget error semantics | ✅ Done | `forward_pool.go:36-54` | fwdHandler logs errors, does not propagate |
| Pool shutdown in cleanup Phase 1 | ✅ Done | `reactor.go:4758-4760` | Before peer Stop loop |

### Acceptance Criteria

| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | ✅ Done | `TestFwdPool_ParallelPeers`, `TestFwdPool_LazyCreation` | Slow peer independent; per-peer workers |
| AC-2 | ✅ Done | `reactor.go:3267` | Zero-copy: rawBodies gets payload directly |
| AC-3 | ✅ Done | `reactor.go:3300` | Re-encode: updates gets parsedUpdate |
| AC-4 | ✅ Done | `reactor.go:3256-3258` | Split: rawBodies gets split pieces |
| AC-5 | ✅ Done | `TestFwdPool_DoneCalledOnSuccess`, `reactor.go:3309-3310` | Retain per peer, Release in done callback |
| AC-6 | ✅ Done | `TestFwdPool_IdleTimeout` | Worker exits after idle, count decrements |
| AC-7 | ✅ Done | `TestFwdPool_Stop`, `TestFwdPool_StopUnblocksDispatch` | Drain + exit; stopCh unblocks |
| AC-8 | ✅ Done | `TestFwdPool_DispatchAfterStop` | Returns false |
| AC-9 | ✅ Done | `TestFwdPool_FIFOPerPeer` | Sequential counter verification |
| AC-10 | ✅ Done | `TestFwdPool_BackpressureBehavior` | Blocks on full channel, unblocks on drain |
| AC-11 | ✅ Done | `TestFwdPool_HandlerError` | Panic recovery, done callback still called |
| AC-12 | ✅ Done | `make ze-verify` | 0 lint, all unit + 246 functional pass |

### Tests from TDD Plan

| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestFwdPool_LazyCreation` | ✅ Done | `forward_pool_test.go:17` | AC-1 |
| `TestFwdPool_IdleTimeout` | ✅ Done | `forward_pool_test.go:50` | AC-6 |
| `TestFwdPool_Stop` | ✅ Done | `forward_pool_test.go:68` | AC-7 |
| `TestFwdPool_DispatchAfterStop` | ✅ Done | `forward_pool_test.go:89` | AC-8 |
| `TestFwdPool_FIFOPerPeer` | ✅ Done | `forward_pool_test.go:103` | AC-9 |
| `TestFwdPool_ParallelPeers` | ✅ Done | `forward_pool_test.go:131` | AC-1 |
| `TestFwdPool_BackpressureBehavior` | ✅ Done | `forward_pool_test.go:166` | AC-10 |
| `TestFwdPool_HandlerError` | ✅ Done | `forward_pool_test.go:210` | AC-11 |
| `TestFwdPool_StopUnblocksDispatch` | ✅ Done | `forward_pool_test.go:246` | AC-7 |
| `TestFwdPool_DoneCalledOnSuccess` | ✅ Done | `forward_pool_test.go:283` | AC-5 |
| `TestForwardUpdate_DispatchesToPool` | ✅ Done | `forward_update_test.go:27` | AC-2: zero-copy dispatch verified |
| `TestForwardUpdate_RetainRelease` | ✅ Done | `forward_update_test.go:105` | AC-5: Retain per peer, Release after worker, eviction verified |

### Files from Plan

| File | Status | Notes |
|------|--------|-------|
| `reactor/forward_pool.go` (create) | ✅ Done | fwdPool, fwdHandler, fwdItem, fwdKey |
| `reactor/forward_pool_test.go` (create) | ✅ Done | 10 tests |
| `reactor/forward_update_test.go` (create) | ✅ Done | 3 integration tests |
| `reactor/reactor.go` (modify) | ✅ Done | Struct field, New(), cleanup(), ForwardUpdate, SetClock |
| ~~`bgp-rr/worker.go`~~ | N/A | Already fixed in dd3c15ac |

### Audit Summary
- **Total items:** 30
- **Done:** 30
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 1 (dispatchWG addition — improvement, documented)

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-12 all demonstrated
- [ ] `make ze-unit-test` passes
- [ ] `make ze-functional-test` passes
- [ ] Feature code integrated (`internal/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] `make ze-lint` passes
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (3+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (verified before implementation)
- [ ] Implementation complete
- [ ] Tests PASS (verified after implementation)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Spec moved to `docs/plan/done/NNN-<name>.md`
- [ ] All files committed together
