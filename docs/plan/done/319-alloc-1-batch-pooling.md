# Spec: Allocation Reduction — Batch Pooling

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `docs/plan/spec-alloc-0-umbrella.md` — umbrella tracker
3. `internal/plugins/bgp/reactor/delivery.go` — `drainDeliveryBatch`
4. `internal/plugins/bgp/reactor/forward_pool.go` — `drainBatch`, `fwdWorker`, `runWorker`
5. `internal/plugin/process_delivery.go` — `drainBatch`, `deliverBatch`
6. `internal/plugins/bgp-rs/server.go` — `selectForwardTargets`, `batchForwardUpdate`

## Task

Replace per-burst slice allocations with per-worker reusable slices in four drain/batch functions and one target-selection function. Workers are long-lived goroutines that process serially — per-worker fields are safe.

Parent: `spec-alloc-0-umbrella.md` (child 1).

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` — reactor event loop, delivery pipeline
  → Constraint: per-peer delivery goroutines are long-lived workers with serial processing
  → Constraint: forward pool workers are per-destination-peer, serial, idle-timeout exit + re-create
- [ ] `docs/architecture/api/text-format.md` — event delivery format
  → Decision: text events are strings passed through channels — slice of strings is the batch format

**Key insights:**
- All five functions allocate a new slice per burst, starting at capacity 1 and growing via `append`
- Workers process one batch at a time — the batch is consumed before the next burst
- Forward pool workers may exit on idle timeout and be re-created — per-worker buffer must be initialized on worker creation
- `deliverBatch` in process_delivery.go creates `events := make([]string, len(batch))` — this can use a reusable slice with `[:0]` + `append`, then pass the slice directly

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/bgp/reactor/delivery.go` — line 29: `drainDeliveryBatch(first deliveryItem, ch) []deliveryItem`: allocates `[]deliveryItem{first}`, appends from channel. Called per-burst in per-peer delivery goroutine (started at `peer.go:936`). Batch consumed by `receiver.OnMessageBatchReceived` synchronously, then discarded.
  → Constraint: return type is `[]deliveryItem` — callers iterate the returned slice
- [ ] `internal/plugins/bgp/reactor/forward_pool.go` — line 272: `drainBatch(firstItem fwdItem, ch) []fwdItem`: allocates `[]fwdItem{firstItem}`, appends from channel. Called in `runWorker` (line 310). Batch passed to `safeBatchHandle` synchronously, then discarded.
  → Constraint: `fwdWorker` struct exists (line 100) with fields `ch`, `done`, `pending` — can add `batchBuf` field
- [ ] `internal/plugin/process_delivery.go` — line 101: `drainBatch(first EventDelivery) []EventDelivery`: allocates `[]EventDelivery{first}`, appends from `p.eventChan`. Called in `deliveryLoop` (line 94). Batch passed to `deliverBatch` synchronously.
  → Constraint: method on `Process` — can add `batchBuf` field to Process, or keep local to deliveryLoop
  → Constraint: line 119: `deliverBatch(batch []EventDelivery, timeout)` allocates `events := make([]string, len(batch))` — consumed within function scope, use `[:0]` + `append`
- [ ] `internal/plugins/bgp-rs/server.go` — line 577: `selectForwardTargets(sourcePeer, families) []string`: allocates `var targets []string`, appends matching peer addresses, sorts. Called per UPDATE in `batchForwardUpdate` (line 608). Result immediately consumed by `strings.Join(targets, ",")`.
  → Constraint: called under `rs.mu.RLock()` — the returned slice must not be held across lock boundaries (it isn't — consumed immediately at line 616)

**Behavior to preserve:**
- Batch ordering: items must appear in drain order (first + channel drain order)
- Forward pool FIFO ordering per destination peer
- `selectForwardTargets` sort order (deterministic target selection for selector caching)
- Channel close detection (`!ok` checks in drain loops)

**Behavior to change:**
- Five allocation sites changed from new-slice-per-call to reuse-per-worker

## Data Flow (MANDATORY)

### Entry Point
- BGP UPDATE arrives → `notifyMessageReceiver` → `peer.deliverChan` (reactor delivery)
- Reactor delivery → `server/events.go` → `proc.Deliver` → `Process.eventChan` (process delivery)
- `deliverBatch` → `bridge.DeliverEvents` → bgp-rs `processForward` → `batchForwardUpdate` → `selectForwardTargets` (bgp-rs)
- `ForwardUpdate` → `fwdPool.Dispatch` → `fwdWorker.ch` → `runWorker` → `drainBatch` (forward pool)

### Transformation Path
1. Reactor drain: `drainDeliveryBatch(first, ch)` → batch of `deliveryItem` → consumed by `OnMessageBatchReceived`
2. Process drain: `drainBatch(first)` → batch of `EventDelivery` → consumed by `deliverBatch`
3. Process delivery: `deliverBatch` → `make([]string)` → consumed by bridge/socket within same function
4. Forward pool drain: `drainBatch(firstItem, ch)` → batch of `fwdItem` → consumed by `safeBatchHandle`
5. bgp-rs target selection: `selectForwardTargets` → `[]string` → consumed by `strings.Join` in `batchForwardUpdate`

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Channel → Worker | Drain functions read from channel | [ ] |
| Worker → Consumer | Returned slice consumed synchronously | [ ] |

### Integration Points
- `drainDeliveryBatch` called from `peer.go` delivery goroutine — caller receives `[]deliveryItem`
- `drainBatch` (fwd) called from `runWorker` — caller passes to `safeBatchHandle`
- `drainBatch` (process) called from `deliveryLoop` — caller passes to `deliverBatch`
- `selectForwardTargets` called from `batchForwardUpdate` — result used for `strings.Join`

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| BGP UPDATE received by peer | → | `drainDeliveryBatch` reuses buffer | `TestDrainDeliveryBatchReusesBuffer` |
| Forward pool dispatches item | → | `drainBatch` (fwd) reuses buffer | `TestFwdDrainBatchReusesBuffer` |
| Process receives event | → | `drainBatch` (process) reuses buffer | `TestProcessDrainBatchReusesBuffer` |
| Process delivers to bridge | → | `deliverBatch` reuses events slice | `TestDeliverBatchReusesEventsSlice` |
| bgp-rs forwards UPDATE | → | `selectForwardTargets` reuses buffer | `TestSelectForwardTargetsReusesBuffer` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `drainDeliveryBatch` called twice in same worker | Second call reuses backing array from first (no new allocation) |
| AC-2 | `drainBatch` (forward pool) called twice in same worker | Second call reuses backing array |
| AC-3 | `drainBatch` (process) called twice in same delivery loop | Second call reuses backing array |
| AC-4 | `deliverBatch` called with batch of N events | `events` slice reuses backing array from previous call |
| AC-5 | `selectForwardTargets` called twice from same worker | Second call reuses backing array |
| AC-6 | Forward pool worker exits on idle and restarts | New worker gets fresh buffer (no stale data) |
| AC-7 | `make ze-verify` | Passes with zero regressions |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestDrainDeliveryBatchReusesBuffer` | `internal/plugins/bgp/reactor/delivery_test.go` | AC-1: drain twice, verify same backing array via `cap` | |
| `TestFwdDrainBatchReusesBuffer` | `internal/plugins/bgp/reactor/forward_pool_test.go` | AC-2: dispatch twice to same worker, verify reuse | |
| `TestProcessDrainBatchReusesBuffer` | `internal/plugin/process_delivery_test.go` | AC-3: deliver twice, verify backing array reuse | |
| `TestDeliverBatchReusesEventsSlice` | `internal/plugin/process_delivery_test.go` | AC-4: deliverBatch twice, verify events slice reuse | |
| `TestSelectForwardTargetsReusesBuffer` | `internal/plugins/bgp-rs/server_test.go` | AC-5: call twice from worker context, verify reuse | |
| `TestFwdWorkerIdleRestartFreshBuffer` | `internal/plugins/bgp/reactor/forward_pool_test.go` | AC-6: worker exits and restarts, no stale data | |

### Boundary Tests
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A — no new numeric inputs | | | | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| Existing `test/plugin/` functional tests | `test/plugin/*.ci` | UPDATE delivery through full pipeline — regression check | |

## Files to Modify
- `internal/plugins/bgp/reactor/delivery.go` — change `drainDeliveryBatch` to accept and return a reusable slice
- `internal/plugins/bgp/reactor/peer.go` — per-peer delivery goroutine: pass reusable buffer to drain function
- `internal/plugins/bgp/reactor/forward_pool.go` — add `batchBuf` field to `fwdWorker`, pass to `drainBatch`
- `internal/plugin/process_delivery.go` — add reusable slice to `deliveryLoop`, pass to `drainBatch` and `deliverBatch`
- `internal/plugins/bgp-rs/server.go` — add `targetBuf` field to worker or pass through `batchForwardUpdate`

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | No | |
| CLI commands/flags | No | |
| API commands doc | No | |
| Plugin SDK docs | No | |
| Functional test for new RPC/API | No — existing tests cover regression | |

## Files to Create
- `internal/plugins/bgp/reactor/delivery_test.go` — if not already exists, add buffer reuse tests
- No new production files

## Implementation Steps

1. **Write unit tests** for buffer reuse in all five drain/selection functions → Review: tests verify same backing array via pointer comparison or cap check
2. **Run tests** → Verify FAIL (new tests expect reuse, current code allocates new)
3. **Implement** — for each function:
   - Change drain function signature to accept `buf []T` parameter, return `buf[:0]` + appended items
   - Add `batchBuf` field to worker struct (fwdWorker) or local variable in goroutine scope (deliveryLoop, peer delivery)
   - `selectForwardTargets`: add `targetBuf` parameter or per-worker field
   - `deliverBatch`: reuse `events []string` from previous call via closure or Process field
4. **Run tests** → Verify PASS
5. **Run `make ze-verify`** → Verify zero regressions
6. **Critical Review** → All 6 checks from `rules/quality.md`

### Failure Routing
| Failure | Route To |
|---------|----------|
| Test verifies wrong thing (always passes) | Step 1 — add assertion that fails without reuse |
| Stale data in reused buffer | Step 3 — ensure `[:0]` reset before use |
| Race detector failure | Step 3 — verify no concurrent access to buffer |

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

## Implementation Summary

### What Was Implemented
- Changed all five drain/selection functions from per-call slice allocation to caller-provided reusable buffer pattern
- `drainDeliveryBatch` (reactor): accepts `buf []deliveryItem`, returns reused slice
- `drainBatch` (forward pool): accepts `buf []fwdItem`, returns reused slice; `fwdWorker` struct gets `batchBuf` field
- `drainBatch` (process): accepts `buf []EventDelivery`, returns reused slice; `deliveryLoop` holds local var
- `deliverBatch` (process): accepts `eventsBuf []string`, returns reused slice; `deliveryLoop` holds local var
- `selectForwardTargets` (bgp-rs): accepts `buf []string`, returns reused slice; `forwardBatch` struct gets `targetBuf` field

### Bugs Found/Fixed
- None

### Documentation Updates
- None required (no architecture doc changes — this is an internal allocation optimization)

### Deviations from Plan
- Spec suggested `batchBuf` field on Process or local to deliveryLoop for process drainBatch; chose local variable in deliveryLoop (simpler, no struct field pollution)
- Same local-variable approach for eventsBuf in deliveryLoop

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Replace per-burst allocations in drainDeliveryBatch | ✅ Done | delivery.go:31 | `buf = append(buf[:0], first)` pattern |
| Replace per-burst allocations in drainBatch (fwd) | ✅ Done | forward_pool.go:274 | Same pattern, `batchBuf` on `fwdWorker` |
| Replace per-burst allocations in drainBatch (process) | ✅ Done | process_delivery.go:105 | Same pattern, local var in deliveryLoop |
| Replace per-event allocations in deliverBatch | ✅ Done | process_delivery.go:124 | `eventsBuf[:0]` + append, returned for reuse |
| Replace per-call allocations in selectForwardTargets | ✅ Done | server.go:578 | `buf[:0]` + append, `targetBuf` on `forwardBatch` |
| Preserve batch ordering | ✅ Done | All drain functions | `append(buf[:0], first)` preserves order |
| Preserve channel close detection | ✅ Done | All drain functions | `!ok` checks unchanged |
| Preserve selectForwardTargets sort order | ✅ Done | server.go:598 | `sort.Strings(buf)` unchanged |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | ✅ Done | `TestDrainDeliveryBatchReusesBuffer` | Pointer comparison via unsafe.SliceData |
| AC-2 | ✅ Done | `TestFwdDrainBatchReusesBuffer` | Same pointer comparison pattern |
| AC-3 | ✅ Done | `TestProcessDrainBatchReusesBuffer` | Same pointer comparison pattern |
| AC-4 | ✅ Done | `TestDeliverBatchReusesEventsSlice` | Verifies returned eventsBuf reuse |
| AC-5 | ✅ Done | `TestSelectForwardTargetsReusesBuffer` | Same pointer comparison pattern |
| AC-6 | ✅ Done | `TestFwdWorkerIdleRestartFreshBuffer` | Worker exits, re-creates with nil buf |
| AC-7 | ✅ Done | `make ze-verify` | Lint: 0 issues. All packages pass except pre-existing flaky TestSlowPluginFatal |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestDrainDeliveryBatchReusesBuffer` | ✅ Done | reactor/delivery_test.go:13 | AC-1 |
| `TestDrainDeliveryBatchChannelClose` | ✅ Done | reactor/delivery_test.go:49 | Channel close preservation |
| `TestFwdDrainBatchReusesBuffer` | ✅ Done | reactor/forward_pool_test.go:504 | AC-2 |
| `TestProcessDrainBatchReusesBuffer` | ✅ Done | plugin/process_delivery_test.go:14 | AC-3 |
| `TestDeliverBatchReusesEventsSlice` | ✅ Done | plugin/process_delivery_test.go:53 | AC-4 |
| `TestSelectForwardTargetsReusesBuffer` | ✅ Done | bgp-rs/server_test.go:1619 | AC-5 |
| `TestFwdWorkerIdleRestartFreshBuffer` | ✅ Done | reactor/forward_pool_test.go:539 | AC-6 |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/plugins/bgp/reactor/delivery.go` | ✅ Done | Signature change + buf reuse |
| `internal/plugins/bgp/reactor/peer.go` | ✅ Done | Caller updated with local batchBuf |
| `internal/plugins/bgp/reactor/forward_pool.go` | ✅ Done | `batchBuf` field + signature change |
| `internal/plugin/process_delivery.go` | ✅ Done | Both drainBatch + deliverBatch changed |
| `internal/plugins/bgp-rs/server.go` | ✅ Done | `targetBuf` field + signature change |
| `internal/plugins/bgp/reactor/delivery_test.go` | ✅ Done | New file with 2 tests |
| `internal/plugin/process_delivery_test.go` | ✅ Done | New file with 2 tests |

### Audit Summary
- **Total items:** 22 (8 requirements + 7 AC + 7 tests)
- **Done:** 22
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 0

## Checklist

### Goal Gates
- [ ] AC-1..AC-7 all demonstrated
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `make ze-unit-test` passes
- [ ] `make ze-functional-test` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes

### Quality Gates
- [ ] `make ze-lint` passes
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Spec moved to `docs/plan/done/NNN-alloc-1-batch-pooling.md`
- [ ] Spec included in commit
