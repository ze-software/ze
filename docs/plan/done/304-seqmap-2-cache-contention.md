# Spec: seqmap-2-cache-contention — Reduce RecentUpdateCache Lock Contention

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/plugins/bgp/reactor/recent_cache.go` — current cache implementation
4. `internal/seqmap/seqmap.go` — seqmap library (prerequisite: spec-seqmap.md)
5. `internal/plugins/bgp/reactor/reactor.go:584-709` — notifyMessageReceiver (Add + Activate call sites)
6. `internal/plugins/bgp/reactor/reactor_api_forward.go:360-377,987-1004` — Get + Ack call sites

## Task

Reduce lock contention in `RecentUpdateCache` via two changes:
1. Extract the gap-based safety valve scan from `Add()` into a background goroutine
2. Replace `map[uint64]*cacheEntry` with `seqmap.Map[uint64, *cacheEntry]` so the FIFO cumulative ack loop uses `Since()` instead of probing every integer ID

**Motivation:** Flame graph shows `RecentUpdateCache.Add` as a significant cost on the UPDATE receive path. Multiple peer goroutines compete for the single `sync.RWMutex` write lock. Two factors extend lock hold time: (a) the gap scan iterates ALL entries every 30s, (b) `Ack()` cumulative loop probes every integer ID between `lastAck` and the target — most miss because non-UPDATE messages (KEEPALIVE, OPEN) also consume IDs from the global atomic counter.

**Prerequisite:** `spec-seqmap.md` — the seqmap library must exist before this spec can be implemented.

**Sibling spec:** `spec-seqmap.md` (library + adj-rib-in integration). This spec is the second consumer of seqmap.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` — reactor event loop, cache role
  → Constraint: RecentUpdateCache is on the critical UPDATE receive path — called per received UPDATE from every peer goroutine
  → Decision: single cache shared across all peers, protected by RWMutex
- [ ] `.claude/rules/goroutine-lifecycle.md` — background goroutine patterns
  → Constraint: background goroutines must be long-lived workers with clean shutdown, not per-event

### Source Files (prerequisite — read BEFORE implementing)
- [ ] `internal/plugins/bgp/reactor/recent_cache.go` (~524L) — RecentUpdateCache with sync.RWMutex, gap scan in Add(), cumulative ack loop in Ack()
  → Constraint: gap scan at Add:186-200 iterates ALL entries under write lock every 30s
  → Constraint: cumulative ack at Ack:323-327 probes every integer ID from lastAck+1 to target
  → Constraint: evictLocked returns pool buffers (poolBuf, ebgpPoolBuf4, ebgpPoolBuf2)
- [ ] `internal/plugins/bgp/reactor/received_update.go` (lines 16-23) — msgIDCounter atomic, gap-free increments by 1
  → Constraint: message IDs are global across ALL message types (OPEN, KEEPALIVE, UPDATE, NOTIFICATION) — most IDs in the gap between lastAck and target are not in the cache
- [ ] `internal/seqmap/seqmap.go` — generic Map with Put/Get/Delete/Since/Range/Clear
  → Decision: Since(fromSeq, fn) uses binary search + live-entry iteration — O(log n + k) vs O(gap)
  → Constraint: not safe for concurrent use — caller must synchronize externally
  → Constraint: auto-compaction when dead > len/2 and len > 256

### RFC Summaries
- N/A — internal performance optimization, no protocol impact.

**Key insights:**
- Add() is called per received UPDATE from each peer's TCP-read goroutine — many concurrent writers
- Ack() is called from plugin worker goroutines (bgp-rs per-source-peer workers, bgp-rr per-family workers)
- Get() is called from ForwardUpdate path — readers blocked by any writer
- The gap scan is O(N) on all entries but runs only every 30s — when it runs, it holds write lock for potentially milliseconds
- The cumulative ack loop probes IDs 1-by-1 through a map where most IDs are absent (non-UPDATE messages consume IDs)
- seqmap.Since() eliminates the gap-probing problem entirely — binary search to start, iterate only live entries

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/bgp/reactor/recent_cache.go` — Add() acquires write lock, calls clock.Now(), runs gap scan every 30s (iterates ALL entries to find stalled ones), checks soft limit, inserts into map, updates highestAddedID. Ack() acquires write lock, FIFO cumulative loop probes each integer ID from lastAck+1 to target, acking any found entries.
- [ ] `internal/plugins/bgp/reactor/reactor.go:676-706` — notifyMessageReceiver calls recentUpdates.Add() then enqueues to delivery channel (or synchronous Activate)
- [ ] `internal/plugins/bgp/reactor/reactor_api_forward.go:362,372,994` — ForwardUpdate calls Get() then defers Ack(); ReleaseUpdate calls Ack()

**Behavior to preserve:**
- All cache semantics: Add/Get/Ack/Activate/Retain/Release/Decrement/Delete
- Consumer tracking: pluginLastAck, nonFIFOConsumers, highestFullyAcked
- Safety valve: stalled entries evicted after safetyValveDuration when passed over
- Soft limit: warns but never rejects
- Pool buffer lifecycle: evictLocked returns poolBuf + ebgpPoolBuf4 + ebgpPoolBuf2
- All existing test assertions

**Behavior to change:**
- Gap scan moved from inline in Add() to background goroutine on ticker
- Internal storage: `map[uint64]*cacheEntry` replaced by `seqmap.Map[uint64, *cacheEntry]`
- Cumulative ack loop: probing each integer ID replaced by `seqmap.Since(lastAck+1, fn)`
- UnregisterConsumer FIFO path: uses `Since(lastAck+1)` instead of walking all entries
- New lifecycle: `Start()` launches background goroutine, `Stop()` shuts it down

**Method-by-method seqmap migration:**

| Cache method | Old map operation | New seqmap operation |
|--------------|-------------------|----------------------|
| `Add()` | `c.entries[msgID] = &cacheEntry{...}` | `c.entries.Put(msgID, msgID, entry)` — msgID as both key and seq |
| `Get()` | `entry, ok := c.entries[id]` | `entry, ok := c.entries.Get(id)` |
| `Contains()` | `_, ok := c.entries[id]` | `_, ok := c.entries.Get(id)` |
| `evictLocked()` | `delete(c.entries, id)` | `c.entries.Delete(id)` — marks entry dead in seqmap log |
| `Ack()` cumulative | `for id := lastAck+1; id < target; id++ { if ie, exists := c.entries[id] ... }` | `c.entries.Since(lastAck+1, fn)` with early stop at target |
| `UnregisterConsumer()` FIFO | `for id, e := range c.entries { if id <= lastAck { continue } ... }` | `c.entries.Since(lastAck+1, fn)` |
| `UnregisterConsumer()` unordered | `for id, e := range c.entries { ... }` | `c.entries.Range(fn)` |
| `Len()` | `len(c.entries)` | `c.entries.Len()` |
| `List()` | `for id := range c.entries` | `c.entries.Range(fn)` collecting IDs |
| Gap scan (background) | `for id, e := range c.entries` | `c.entries.Range(fn)` |
| `Delete()` | `if e, ok := c.entries[id]; ok { ... delete(c.entries, id) }` | `e, ok := c.entries.Get(id)` then `c.entries.Delete(id)` |

## Data Flow (MANDATORY)

### Entry Point
- UPDATEs arrive via TCP → session read goroutine → notifyMessageReceiver → cache.Add()
- Plugin acks arrive via RPC → ForwardUpdate/ReleaseUpdate → cache.Ack()
- Plugin reads arrive via RPC → ForwardUpdate → cache.Get()

### Transformation Path
1. Session goroutine receives UPDATE → calls `recentUpdates.Add(receivedUpdate)` (write lock)
2. After dispatch, delivery goroutine calls `recentUpdates.Activate(id, count)` (write lock)
3. Plugin goroutine calls `recentUpdates.Get(id)` to read cached update (read lock)
4. Plugin goroutine calls `recentUpdates.Ack(id, name)` after forwarding (write lock)
5. Background goroutine runs gap scan every 30s (write lock only if evicting)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Session goroutine → cache | Direct call to Add() | [ ] |
| Delivery goroutine → cache | Direct call to Activate() | [ ] |
| Plugin goroutine → cache | RPC → ForwardUpdate/ReleaseUpdate → Get()/Ack() | [ ] |
| Background goroutine → cache | Ticker → gap scan under write lock | [ ] |

### Integration Points
- `reactor.notifyMessageReceiver()` → `cache.Add()` — unchanged call
- `peer.runOnce()` delivery goroutine → `cache.Activate()` — unchanged call
- `reactorAPIAdapter.ForwardUpdate()` → `cache.Get()` + `cache.Ack()` — unchanged calls
- `reactorAPIAdapter.ReleaseUpdate()` → `cache.Ack()` — unchanged call
- Reactor startup → `cache.Start()` — NEW (launches background goroutine)
- Reactor shutdown → `cache.Stop()` — NEW (stops background goroutine)

### Architectural Verification
- [ ] No bypassed layers — seqmap is a pure data structure, cache semantics unchanged
- [ ] No unintended coupling — seqmap has zero external imports
- [ ] No duplicated functionality — replaces map with seqmap, not adding parallel storage
- [ ] Zero-copy preserved — seqmap stores pointers to cacheEntry, no copies

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `NewRecentUpdateCache` + `Start()` | → | Background goroutine runs gap scan | `TestGapScanRunsInBackground` |
| `Add()` after Start() | → | No inline gap scan, fast map insert only | `TestAddDoesNotRunGapScanInline` |
| `Ack(highID)` FIFO with gap | → | `seqmap.Since()` iterates only cached entries | `TestAckCumulativeSkipsNonCachedIDs` |
| `UnregisterConsumer` FIFO | → | `seqmap.Since(lastAck+1)` walks only relevant entries | `TestUnregisterConsumerUsesSince` |
| `Stop()` | → | Background goroutine exits, no leak | `TestStopCleansUpGoroutine` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `Add()` called after `Start()` with entries older than gapScanInterval | No gap scan runs inside Add — write lock held only for seqmap.Put + metadata update |
| AC-2 | Background goroutine ticks with stalled entry (passed over, expired safety valve) | Entry evicted, pool buffers returned, highestFullyAcked updated |
| AC-3 | `Stop()` called | Background goroutine exits cleanly within one tick interval |
| AC-4 | FIFO `Ack(id=100)` with lastAck=1, only IDs 10,50,100 cached | Cumulative ack visits exactly entries 10,50,100 — not 2,3,4,...,99 |
| AC-5 | Unordered `Ack(id)` | Per-entry ack only, no cumulative loop (unchanged semantics) |
| AC-6 | `UnregisterConsumer` for FIFO consumer with lastAck=50 | Only entries with seq > 50 are decremented (uses Since, not full walk) |
| AC-7 | All existing `recent_cache_test.go` tests | Pass with `defer cache.Stop()` added — no behavioral regression |
| AC-8 | Race detector with concurrent Add/Get/Ack/Stop | No races detected |

## 🧪 TDD Test Plan

### Unit Tests — RecentUpdateCache changes
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestGapScanRunsInBackground` | `reactor/recent_cache_test.go` | FakeClock + Start(): gap scan evicts stalled entry after tick | |
| `TestAddDoesNotRunGapScanInline` | same | Add() with stale entries does NOT evict inline (gap scan is background only) | |
| `TestAckCumulativeSkipsNonCachedIDs` | same | Ack with large ID gap visits only cached entries (count intermediate ack calls) | |
| `TestUnregisterConsumerUsesSince` | same | FIFO unregister only decrements entries above lastAck | |
| `TestStopCleansUpGoroutine` | same | After Stop(), background goroutine has exited | |
| `TestStopIdempotent` | same | Calling Stop() twice does not panic | |
| `TestConcurrentAddAckWithBackground` | same | Race detector clean with Add + Ack + gap scan all running | |

### Existing Tests (must still pass — add `defer cache.Stop()`)
| Test | File | Validates | Status |
|------|------|-----------|--------|
| All `Test*` in `recent_cache_test.go` | `reactor/recent_cache_test.go` | Behavioral preservation | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| gapScanInterval (30s) | time.Duration | 30s (tick fires) | N/A (internal const) | N/A |
| seqmap compaction threshold | 256 entries, 50% dead | 256 live + 257 dead | N/A | N/A |

### Functional Tests
- N/A — internal performance optimization with no user-facing surface. Existing functional tests exercise the cache through the full UPDATE receive/forward path.

## Files to Modify
- `internal/plugins/bgp/reactor/recent_cache.go` — replace map with seqmap, extract gap scan to background goroutine, add Start/Stop, rewrite Ack cumulative loop and UnregisterConsumer to use Since
- `internal/plugins/bgp/reactor/recent_cache_test.go` — add `defer cache.Stop()` to all tests, add new tests for background gap scan, cumulative ack via Since, Stop lifecycle
- `internal/plugins/bgp/reactor/reactor.go` — call `cache.Start()` at initialization, `cache.Stop()` at shutdown

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | No | N/A — internal optimization |
| RPC count in docs | No | N/A — no new RPCs |
| CLI commands | No | N/A |
| API commands doc | No | N/A |
| Plugin SDK docs | No | N/A |
| Functional test | No | Existing tests cover path |

## Files to Create
- None — all changes are modifications to existing files.

## Implementation Steps

Each step ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Write new unit tests** (background gap scan, cumulative ack via Since, Stop lifecycle) → Review: edge cases? Race coverage?
2. **Run tests** → Verify FAIL (new tests fail because cache still uses old implementation)
3. **Add Start/Stop lifecycle** → Background goroutine with ticker for gap scan. Move gap scan logic from Add() to background callback. Add `defer cache.Stop()` to all existing tests.
4. **Run tests** → Old tests pass (gap scan still works, just moved). New Start/Stop tests pass.
5. **Replace map with seqmap** → Change `entries map[uint64]*cacheEntry` to `*seqmap.Map[uint64, *cacheEntry]`. Update Add (Put), Get (Get), Delete (Delete), evictLocked (Delete), Len (Len), List (Range), Contains (Get).
6. **Rewrite Ack cumulative loop** → Replace integer-probing for-loop with `seqmap.Since(lastAck+1, fn)`.
7. **Rewrite UnregisterConsumer** → FIFO path uses `Since(lastAck+1, fn)` instead of walking all entries.
8. **Run tests** → All tests pass (old + new). Verify PASS.
9. **Verify all** → `go test -race ./internal/plugins/bgp/reactor/... -v -count=1`
10. **Critical Review** → All 6 checks from `rules/quality.md`. Document pass/fail. Fix before continuing.
11. **Complete spec** → Fill audit tables, move to done.

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error after seqmap swap | Step 5 (fix type mismatches — seqmap API vs map API) |
| Test fails: gap scan not evicting | Step 3 (verify FakeClock advances past gapScanInterval + safetyValve) |
| Test fails: cumulative ack wrong count | Step 6 (verify Since iterates correct range, check seq vs key) |
| Race detected | Step 3-7 (verify all seqmap access under cache.mu) |
| Existing test fails | Step 5 (check seqmap Put/Get/Delete match old map semantics) |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights
- Message IDs are global and gap-free across ALL message types, but only UPDATEs are cached — making the cumulative ack loop wasteful by design. seqmap.Since() addresses this structurally.
- Moving the gap scan to a background goroutine decouples fault detection (slow, infrequent) from the hot path (fast, per-message). The gap scan only matters for crashed-plugin detection — it never fires during normal operation.
- seqmap's auto-compaction under the caller's lock is acceptable here: with typical cache sizes (hundreds to low thousands of entries), compaction is sub-millisecond.

## RFC Documentation

N/A — internal performance optimization, no RFC references.

## Implementation Summary

### What Was Implemented
- Replaced `map[uint64]*cacheEntry` with `*seqmap.Map[uint64, *cacheEntry]` in RecentUpdateCache
- Extracted gap-based safety valve scan from inline in `Add()` to background goroutine (`gapScanLoop`)
- Added `Start()`/`Stop()` lifecycle methods with idempotent shutdown via `sync.Once`
- Added `SetGapScanInterval()` for test configurability of ticker interval
- Rewrote `Ack()` cumulative loop to use `seqmap.Since()` with collect-then-ack pattern
- Rewrote `UnregisterConsumer()` FIFO path to use `seqmap.Since(lastAck+1)`, unordered path to use `seqmap.Range()`
- Updated all map operations: Add to Put, Get to Get, Delete to Delete, len to Len, range to Range
- Wired `cache.Start()` into `reactor.StartWithContext()` and `cache.Stop()` into `reactor.cleanup()`
- Added 8 new tests, updated 4 gap-scan-dependent tests, added `defer cache.Stop()` to all existing tests

### Bugs Found/Fixed
- None

### Documentation Updates
- None — internal performance optimization with no user-facing surface

### Deviations from Plan
- Added `TestStopWithoutStart` test (not in plan) — ensures `defer cache.Stop()` is safe without `Start()`
- Gap-scan-dependent existing tests call `cache.runGapScan()` directly instead of relying on `Add()` trigger
- Removed `lastGapScan` field — no longer needed with background ticker
- `recent_cache.go` is 615 lines (above 600 monitor threshold) — single concern, acceptable

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Extract gap scan to background goroutine | ✅ Done | recent_cache.go:165-199 | gapScanLoop + runGapScan |
| Replace map with seqmap | ✅ Done | recent_cache.go:57 | seqmap.Map as entries field |
| Cumulative ack via Since() | ✅ Done | recent_cache.go:296-308 | collect-then-ack pattern |
| UnregisterConsumer via Since/Range | ✅ Done | recent_cache.go:388-413 | FIFO uses Since, unordered uses Range |
| Start/Stop lifecycle | ✅ Done | recent_cache.go:143-161 | sync.Once for idempotent Stop |
| Reactor wiring | ✅ Done | reactor.go:873,1253 | Start in StartWithContext, Stop in cleanup |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | ✅ Done | TestAddDoesNotRunGapScanInline | Add() no longer runs gap scan inline |
| AC-2 | ✅ Done | TestGapScanRunsInBackground | Background goroutine evicts stalled entries |
| AC-3 | ✅ Done | TestStopCleansUpGoroutine | Stop completes within 1 second |
| AC-4 | ✅ Done | TestAckCumulativeSkipsNonCachedIDs | Cumulative ack with IDs 10,50,100 |
| AC-5 | ✅ Done | TestCacheUnorderedConsumerNoSweep | Unordered ack unchanged (per-entry) |
| AC-6 | ✅ Done | TestUnregisterConsumerUsesSince | FIFO unregister only entries > lastAck |
| AC-7 | ✅ Done | All existing tests pass | defer cache.Stop() added, no regression |
| AC-8 | ✅ Done | TestConcurrentAddAckWithBackground | Race detector clean |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| TestGapScanRunsInBackground | ✅ Done | recent_cache_test.go | FakeClock + 1ms ticker |
| TestAddDoesNotRunGapScanInline | ✅ Done | recent_cache_test.go | Proves Add() no longer scans |
| TestAckCumulativeSkipsNonCachedIDs | ✅ Done | recent_cache_test.go | Big gaps, 2 consumers |
| TestUnregisterConsumerUsesSince | ✅ Done | recent_cache_test.go | Since-based FIFO unregister |
| TestStopCleansUpGoroutine | ✅ Done | recent_cache_test.go | Deadline-based check |
| TestStopIdempotent | ✅ Done | recent_cache_test.go | Double Stop no panic |
| TestConcurrentAddAckWithBackground | ✅ Done | recent_cache_test.go | 10 goroutines + background |
| All existing tests | ✅ Done | recent_cache_test.go | defer Stop, runGapScan |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `reactor/recent_cache.go` | ✅ Done | seqmap, background goroutine, Since-based ack |
| `reactor/recent_cache_test.go` | ✅ Done | 8 new tests, 4 modified, defer Stop to all |
| `reactor/reactor.go` | ✅ Done | cache.Start() and cache.Stop() wired in |

### Audit Summary
- **Total items:** 25
- **Done:** 25
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 4 (documented in Deviations — all improvements)

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-8 all demonstrated
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `make ze-unit-test` passes
- [ ] `make ze-functional-test` passes
- [ ] Feature code integrated (`internal/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated (or N/A confirmed)
- [ ] Critical Review passes (all 6 checks in `rules/quality.md` — no failures)

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] `make ze-lint` passes
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
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes — all 6 checks in `rules/quality.md` documented pass in spec
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Spec moved to `docs/plan/done/NNN-seqmap-2-cache-contention.md`
- [ ] **Spec included in commit** — NEVER commit implementation without the completed spec
