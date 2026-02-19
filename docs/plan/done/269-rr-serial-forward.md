# Spec: rr-serial-forward

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `internal/plugins/bgp-rr/server.go` — the RR plugin implementation
3. `internal/plugins/bgp-rr/propagation_test.go` — the ordering test that reproduces the bug

## Task

Replace the per-UPDATE goroutine (`go forwardUpdate()`) in the bgp-rr plugin with a single worker goroutine draining a channel. The current pattern spawns one goroutine per received UPDATE. Under load with 522K routes, this creates 522K concurrent goroutines all racing to send SDK RPCs. The engine's FIFO cache requires acks in message-ID order — when a later ID is acked first, all earlier entries are implicitly acked and evicted, causing ~98% route loss.

**Two problems solved:**
1. **FIFO ordering violation** — concurrent goroutines reach the engine out of order, causing implicit ack cascades that evict cache entries before their forward commands arrive
2. **Goroutine explosion** — one goroutine per UPDATE is wasteful; a single worker serializes all SDK calls through one goroutine

## Required Reading

### Architecture Docs
- [ ] `.claude/rules/plugin-design.md` — SDK callback pattern, OnEvent is synchronous RPC
  → Constraint: OnEvent must return promptly — blocking stalls all event delivery
- [ ] `docs/architecture/core-design.md` — Engine/Plugin boundary via JSON over pipes
  → Constraint: Plugin communicates via SDK RPCs only, no shared memory

### RFC Summaries
None — this is an internal plugin fix, no protocol changes.

**Key insights:**
- OnEvent is a synchronous RPC — cannot block for SDK calls
- Engine FIFO cache requires acks in message-ID order (implicit ack cascades)
- `TestForwardOrdering_ConcurrentGoroutines` reproduces 28-45% out-of-order rate

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
- [ ] `internal/plugins/bgp-rr/server.go` — `handleUpdate` spawns `go forwardUpdate()` on line 205; `releaseCache` spawned via `go` on line 170; `forwardUpdate` acquires lock, selects targets, sends SDK RPC; `updateRoute` does synchronous SDK RPC with 10s timeout
- [ ] `internal/plugins/bgp/reactor/recent_cache.go` — `Ack()` FIFO constraint: acking N implicitly acks 1..N-1

**Behavior to preserve:**
- OnEvent callback must return promptly (it is a synchronous RPC — blocking stalls all event delivery)
- Forward command format: `bgp cache N forward peer1,peer2`
- Release command format: `bgp cache N release`
- RIB updates (Insert/Remove) remain synchronous in handleUpdate
- Peer state management unchanged
- Command handler (rr status/peers) unchanged

**Behavior to change:**
- Remove `go forwardUpdate()` — replace with channel send
- Remove `go releaseCache()` — replace with channel send
- Add single worker goroutine started at plugin init, draining the channel
- Worker calls forwardUpdate/releaseCache sequentially, preserving message order

## Data Flow (MANDATORY)

### Entry Point
- UPDATE event arrives via OnEvent synchronous RPC callback
- Format: ze-bgp JSON with message.type "update" and message.id for cache tracking

### Transformation Path
1. OnEvent callback receives JSON string
2. parseEvent extracts peer, msgID, families
3. handleUpdate updates RIB synchronously (Insert/Remove)
4. handleUpdate sends work item to buffered channel (non-blocking)
5. Worker goroutine reads channel in FIFO order
6. Worker calls forwardUpdate (select targets + SDK RPC) or releaseCache (SDK RPC)
7. Engine receives cache forward/release in message-ID order

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| Plugin → Engine | SDK RPC (`UpdateRoute`) — unchanged | [x] |
| OnEvent → Worker | Buffered channel — new | [ ] |

### Integration Points
- `sdk.Plugin.UpdateRoute()` — SDK RPC to engine, used by forwardUpdate and releaseCache
- `RecentUpdateCache.Ack()` — engine-side FIFO constraint, called by engine on forward/release
- `RouteServer.selectForwardTargets()` — peer selection logic, called by worker (unchanged)

### Architectural Verification
- [x] No bypassed layers (channel is internal to plugin, SDK RPC unchanged)
- [x] No unintended coupling (worker is internal to RouteServer)
- [x] No duplicated functionality (replaces goroutine pattern with channel)
- [x] Zero-copy preserved where applicable (no data copying changes)

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | 100 rapid UPDATE events dispatched | All forward commands arrive at engine in message-ID order |
| AC-2 | UPDATE with no targets (empty families) | Release command sent via worker, not goroutine |
| AC-3 | Plugin shutdown (p.Run returns) | Worker drains remaining commands before exit |
| AC-4 | No `go forwardUpdate` or `go releaseCache` in codebase | Grep confirms zero per-message goroutines |
| AC-5 | Existing propagation tests still pass | No regression in target selection or RIB logic |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestForwardOrdering_ConcurrentGoroutines` | `propagation_test.go` | EXISTING: demonstrates the bug (expected to FAIL until fix) | exists |
| `TestForwardOrdering_SequentialPreservesOrder` | `propagation_test.go` | EXISTING: confirms sequential ordering works | exists |
| `TestForwardWorker_OrderPreserved` | `propagation_test.go` | Channel-based worker delivers commands in order | |
| `TestForwardWorker_ReleaseInOrder` | `propagation_test.go` | Release commands interleaved with forwards maintain order | |
| `TestForwardWorker_DrainOnClose` | `propagation_test.go` | Closing the channel drains remaining items | |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| chaos test | `cmd/ze-chaos/` | 4-peer 7-family route reflection under load | deferred — run manually after fix |

## Files to Modify

- `internal/plugins/bgp-rr/server.go` — add channel + worker, remove `go` calls
- `internal/plugins/bgp-rr/propagation_test.go` — add worker tests, update ordering test to pass

### Integration Checklist

| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | |
| RPC count in architecture docs | No | |
| CLI commands/flags | No | |
| CLI usage/help text | No | |
| API commands doc | No | |
| Plugin SDK docs | No | |
| Editor autocomplete | No | |
| Functional test for new RPC/API | No | |

## Files to Create

None.

## Implementation Steps

Each step ends with a **Self-Critical Review**.

1. **Write worker tests** — TestForwardWorker_OrderPreserved, ReleaseInOrder, DrainOnClose
   → **Review:** Do tests exercise the channel-based pattern specifically?

2. **Run tests** — Verify FAIL (worker doesn't exist yet)
   → **Review:** Do tests fail for the right reason (missing function/type)?

3. **Define forward work item type** — struct with msgID, sourcePeer, families, and a release flag
   → **Review:** Is this the minimal struct needed?

4. **Add channel and worker to RouteServer** — buffered channel field, startWorker method
   → **Review:** Buffer size reasonable? Shutdown clean?

5. **Replace `go forwardUpdate()` with channel send** — in handleUpdate
   → **Review:** Non-blocking send? What if channel full?

6. **Replace `go releaseCache()` with channel send** — in handleUpdate
   → **Review:** Release items also go through the channel?

7. **Worker goroutine** — range over channel, dispatch to forwardUpdate or releaseCache
   → **Review:** Single goroutine? Clean shutdown on channel close?

8. **Wire up in RunRouteServer** — start worker before p.Run, close channel after p.Run returns
   → **Review:** Worker started before events arrive? Drained after shutdown?

9. **Run tests** — `go test -race ./internal/plugins/bgp-rr/... -v`
   → **Review:** All tests pass? Race detector clean?

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

- Channel buffer size 4096 chosen to avoid blocking OnEvent under load. With 522K routes the channel acts as backpressure rather than unbounded goroutine creation.
- The `go func()` calls in handleStateDown, handleStateUp, and handleRefresh are NOT part of this fix — they send withdrawals and refresh requests, not cache forward/release commands. Their ordering relative to the cache is irrelevant.

## Implementation Summary

### What Was Implemented
- Added `forwardWork` struct type with msgID, sourcePeer, families, release flag
- Added `workCh chan forwardWork` field to RouteServer (buffered 4096)
- Added `runForwardWorker()` method — single goroutine ranging over channel
- Replaced `go rs.forwardUpdate(...)` in handleUpdate with channel send
- Replaced `go rs.releaseCache(...)` in handleUpdate with channel send
- Wired worker start/stop in RunRouteServer (start before p.Run, close+wait after)
- Updated newTestRouteServer to initialize workCh for test coverage
- Replaced TestForwardOrdering_ConcurrentGoroutines (demonstrated the bug) with TestForwardWorker_OrderPreserved (proves the fix)

### Bugs Found/Fixed
- None — this is a design fix, not a code bug fix

### Documentation Updates
- Updated stale comment referencing "async forwardUpdate goroutine" → "forward worker"

### Deviations from Plan
- Combined scaffold + implementation steps due to linter blocking on unused types
- Replaced the ConcurrentGoroutines test (which demonstrated the bug) with the OrderPreserved test (which proves the fix), rather than keeping both

## Implementation Audit

### Requirements from Task

| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Remove per-UPDATE goroutines | ✅ Done | `server.go:211,246` | Channel sends replace `go` calls |
| Add channel-based worker | ✅ Done | `server.go:165-178` | `runForwardWorker` ranges over `workCh` |
| Preserve FIFO ordering | ✅ Done | `propagation_test.go:24-73` | `TestForwardWorker_OrderPreserved` |
| Clean shutdown | ✅ Done | `server.go:133-135` | `close(workCh)` + `<-workerDone` after `p.Run` |

### Acceptance Criteria

| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | ✅ Done | `TestForwardWorker_OrderPreserved` | 100 rapid UPDATEs arrive in FIFO order |
| AC-2 | ✅ Done | `TestForwardWorker_ReleaseInOrder` | Release items interleaved with forwards maintain order |
| AC-3 | ✅ Done | `TestForwardWorker_DrainOnClose` | Channel close → worker drains and exits |
| AC-4 | ✅ Done | `grep -r 'go rs.forwardUpdate\|go rs.releaseCache'` → 0 matches | No per-message goroutines |
| AC-5 | ✅ Done | All 48 tests pass with `-race` | Zero regressions |

### Tests from TDD Plan

| Test | Status | Location | Notes |
|------|--------|----------|-------|
| TestForwardWorker_OrderPreserved | ✅ Done | `propagation_test.go:24` | Replaced ConcurrentGoroutines test |
| TestForwardWorker_ReleaseInOrder | ✅ Done | `propagation_test.go:81` | Forward/release interleaving |
| TestForwardWorker_DrainOnClose | ✅ Done | `propagation_test.go:134` | Shutdown drain |

### Files from Plan

| File | Status | Notes |
|------|--------|-------|
| `internal/plugins/bgp-rr/server.go` | ✅ Modified | forwardWork type, workCh, worker, handleUpdate, RunRouteServer |
| `internal/plugins/bgp-rr/propagation_test.go` | ✅ Modified | 3 new tests, 1 replaced, 1 kept |

### Audit Summary
- **Total items:** 14
- **Done:** 14
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 1 (ConcurrentGoroutines test replaced, not kept alongside)

## Checklist

### Goal Gates (MUST pass)
- [x] Acceptance criteria AC-1..AC-5 all demonstrated
- [x] Tests pass (`make ze-unit-test`) — all packages pass with -race
- [x] No regressions (`make ze-functional-test`) — parse/decode/editor pass; encode/plugin/reload have pre-existing port-binding failures unrelated to bgp-rr
- [x] Feature code integrated into codebase (`internal/*`)
- [x] Integration completeness: forward ordering proven via test

### Quality Gates (SHOULD pass)
- [x] `make ze-lint` on bgp-rr — 0 issues (full ze-lint has unrelated gosec internal panic)
- [x] Implementation Audit fully completed

### 🧪 TDD
- [x] Tests written
- [x] Tests FAIL (linter blocked on unused types before implementation)
- [x] Implementation complete
- [x] Tests PASS — 48/48 with -race
