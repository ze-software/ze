# Spec: hub-lockfree-dispatch

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-03-20 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/component/plugin/server/hub.go` - RouteCommand dispatch
4. `internal/component/plugin/server/schema.go` - SchemaRegistry with RWMutex
5. `internal/component/plugin/server/subsystem.go` - SubsystemHandler/Manager with RWMutex

## Task

Make Hub command routing lock-free after startup by freezing the SchemaRegistry and SubsystemManager into immutable snapshots. Both registries are populated during the 5-stage startup protocol and never mutated during normal operation. The current RWMutex read-locks have near-zero contention (no writers after startup), but the locks obscure the immutability invariant. Freeze-after-init makes this invariant explicit, auditable, and removes the locks from the hot path entirely.

### Problem Statement

SchemaRegistry and SubsystemManager protect their internal maps with `sync.RWMutex`. Every RouteCommand call acquires three read-locks (FindHandler, Get, Handle). After the 5-stage startup protocol completes, no writer ever acquires these locks -- all mutations (Register, Unregister) happen during startup or shutdown. The read-locks are technically uncontended but:

1. They obscure the "immutable after startup" invariant -- a reader cannot know no writer will appear
2. They add ~4ns per lock/unlock pair (3 pairs = ~12ns per RouteCommand) of unnecessary overhead
3. They prevent future concurrent dispatch optimizations that assume lock-free routing

### Goals

| Goal | Description |
|------|-------------|
| Lock-free routing | SchemaRegistry.FindHandler and SubsystemManager.Get require no locks after startup |
| Explicit immutability | Freeze() makes the "no writers after startup" invariant a compile-time-auditable property |
| Post-freeze safety | Unregister (plugin crash) publishes a new frozen snapshot atomically |
| Zero behavioral change | RouteCommand, ProcessConfig, RouteCommit, RouteRollback semantics unchanged |

### Non-Goals

| Non-Goal | Reason |
|----------|--------|
| Worker pool / concurrent dispatch | Separate spec (`spec-dispatch-pool.md`) |
| Plugin-side concurrency | Separate concern, enabled by dispatch pool |
| Read/write command classification | Plugin decides its own concurrency model |
| Hub-level concurrency limiting | Current callers already run on independent goroutines |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/hub-architecture.md` - Hub routing, schema registry, 5-stage protocol
  -> Constraint: SchemaRegistry populated during Stage 1 declarations, barrier before any dispatch
  -> Decision: Handler routing uses longest-prefix match on dot-separated paths
- [ ] `docs/architecture/core-design.md` - System architecture overview
  -> Constraint: MuxConn already supports concurrent CallRPC. Hub routing is not the bottleneck.

**Key insights:**
- SchemaRegistry.Register() called only during Stage 1. FindHandler is the only hot-path method.
- SubsystemManager.Register() called only during startup. Get() is the only hot-path method.
- After the startup barrier, no goroutine ever calls Register, RegisterRPCs, RegisterCLICommand, or Unregister during normal operation. The RWMutex write-lock path is dead code on the hot path.
- Unregister can be called during shutdown or plugin crash recovery. This is a lifecycle event, not a hot-path concern. The frozen snapshot must handle it (atomic.Store of new snapshot).

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/plugin/server/hub.go` - RouteCommand: FindHandler (RLock) -> Get (RLock) -> Handle (RLock + RPC). ProcessConfig: sequential commands + sorted commit.
  -> Constraint: ProcessConfig must remain sequential for transactional ordering
- [ ] `internal/component/plugin/server/schema.go` - SchemaRegistry: single RWMutex for all maps. FindHandler does longest-prefix match. Register/RegisterRPCs/RegisterCLICommand/RegisterNotifications are write operations.
  -> Constraint: All write operations happen during startup only. 343 lines, single concern (schema storage + lookup).
- [ ] `internal/component/plugin/server/subsystem.go` - SubsystemManager: RWMutex for handler map. Get() is hot-path read. Register/Unregister/StartAll/StopAll are lifecycle.
  -> Constraint: SubsystemHandler.Handle() already minimal: RLock to snapshot proc, RUnlock, then RPC. The handler-level lock protects proc pointer, not the dispatch path.

**Behavior to preserve:**
- RouteCommand semantics: FindHandler -> Get -> Handle -> response. Synchronous from caller perspective.
- ProcessConfig sequential dispatch and sorted commit ordering
- RouteCommit/RouteRollback semantics
- FindHandler longest-prefix match behavior
- All SchemaRegistry query methods (ListRPCs, FindRPC, FindRPCByCommand, etc.)
- SubsystemManager lifecycle methods (StartAll, StopAll, Unregister)

**Behavior to change:**
- FindHandler uses atomic.Load on frozen snapshot instead of RLock on mutable map
- SubsystemManager.Get uses atomic.Load on frozen snapshot instead of RLock on mutable map
- Both registries gain a Freeze() method called once after startup barrier
- Unregister publishes a new frozen snapshot via atomic.Store

## Data Flow (MANDATORY)

### Entry Point
- CLI command or API request enters Hub.RouteCommand()
- Format: ConfigBlock with Handler path, Action, Data

### Transformation Path (Current)

1. **Hub.RouteCommand()** -- caller goroutine
2. **SchemaRegistry.FindHandler()** -- RLock, strip predicates, longest-prefix match on handlers map, RUnlock
3. **SubsystemManager.Get()** -- RLock, map lookup by plugin name, RUnlock
4. **SubsystemHandler.Handle()** -- RLock to snapshot proc, RUnlock, MuxConn.CallRPC
5. Caller blocks on MuxConn response

### Transformation Path (Target)

1. **Hub.RouteCommand()** -- caller goroutine (unchanged)
2. **SchemaRegistry.FindHandler()** -- atomic.Load frozen snapshot, strip predicates, longest-prefix match
3. **SubsystemManager.Get()** -- atomic.Load frozen snapshot, map lookup by plugin name
4. **SubsystemHandler.Handle()** -- unchanged (proc snapshot lock is handler-level, not dispatch-level)
5. Caller blocks on MuxConn response (unchanged)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Hub -> SchemaRegistry | atomic.Load frozen map (no lock) | [ ] |
| Hub -> SubsystemManager | atomic.Load frozen map (no lock) | [ ] |
| Hub -> Plugin | MuxConn.CallRPC (unchanged) | [ ] |

### Integration Points
- `Hub.RouteCommand()` -- calls FindHandler and Get (now lock-free)
- `Hub.ProcessConfig()` -- calls RouteCommand (inherits lock-free lookups)
- Startup sequence -- must call Freeze() after Stage 1 barrier, before any dispatch
- Shutdown / plugin crash -- Unregister must publish new frozen snapshot

### Architectural Verification
- [ ] No bypassed layers (freeze wraps existing lookup, does not change routing logic)
- [ ] No unintended coupling (frozen snapshot is internal to each registry)
- [ ] No duplicated functionality (frozen path replaces locked path, not alongside it)
- [ ] Snapshot shares value pointers (*Schema, *SubsystemHandler) -- map containers are shallow-copied, values are not duplicated

## Design

### 1. Frozen Snapshot for SchemaRegistry

**Frozen type:**

| Field | Type | Purpose |
|-------|------|---------|
| handlers | map[string]string | Handler path -> module name (same as SchemaRegistry.handlers) |
| modules | map[string]*Schema | Module name -> Schema (same as SchemaRegistry.modules) |

SchemaRegistry gains:

| Addition | Purpose |
|----------|---------|
| `frozen atomic.Pointer[frozenSchema]` field | Stores immutable snapshot |
| `Freeze()` method | Builds snapshot from current maps, stores via atomic.Store |
| `FindHandler()` updated | If frozen != nil, use snapshot. Otherwise, fall back to RLock (pre-freeze startup calls). |

**FindHandler with frozen snapshot (prose):**

1. Load frozen snapshot via atomic.Load
2. If nil (pre-freeze): fall back to existing RLock path (unchanged behavior)
3. Strip predicates from path
4. Try exact match in frozen handlers map
5. Try progressively shorter prefixes (same algorithm as current)
6. Return Schema and matched handler path

The fallback is precautionary. RouteCommand is only called after startup, so FindHandler on the hot path always hits the frozen snapshot. But during startup, internal calls (e.g., schema validation) may call FindHandler before Freeze(). The fallback ensures correctness in both cases without requiring callers to know the freeze state.

**Other read methods** (FindRPC, FindRPCByCommand, ListRPCs, GetByModule, GetByHandler, ListModules, ListHandlers, Count, ListNotifications) -- these are used by CLI commands and schema queries, not the hot dispatch path. They continue to use RLock. Only FindHandler (the hot path in RouteCommand) uses the frozen snapshot.

### 2. Frozen Snapshot for SubsystemManager

**Frozen type:**

| Field | Type | Purpose |
|-------|------|---------|
| handlers | map[string]*SubsystemHandler | Plugin name -> handler (same as SubsystemManager.handlers) |

SubsystemManager gains:

| Addition | Purpose |
|----------|---------|
| `frozen atomic.Pointer[frozenSubsystems]` field | Stores immutable snapshot |
| `Freeze()` method | Builds snapshot from current map, stores via atomic.Store |
| `Get()` updated | If frozen != nil, use snapshot. Otherwise, fall back to RLock. |

**Get with frozen snapshot (prose):**

1. Load frozen snapshot via atomic.Load
2. If nil (pre-freeze): fall back to existing RLock path
3. Direct map lookup by plugin name
4. Return SubsystemHandler or nil

### 3. Post-Freeze Mutations (Unregister)

Unregister is called during plugin crash recovery or shutdown. After freeze, it must:

1. Acquire write lock on mutable map (for lifecycle safety)
2. Remove handler from mutable map
3. Build new frozen snapshot from updated mutable map
4. Publish via atomic.Store

This is safe because:
- Unregister is a lifecycle event (rare, not hot path)
- atomic.Store is a single-writer operation (lifecycle management is single-threaded)
- Concurrent readers see either the old or new snapshot (both valid)
- The write lock only protects the mutable map, not the frozen path

### 4. Freeze Timing

Freeze() is called once, after all plugins complete Stage 1 (declaration barrier), before any command dispatch begins. The startup sequence provides a natural happens-before:

1. StartAll() launches plugins, completes 5-stage protocol per plugin
2. RegisterSchemas() registers all schemas in SchemaRegistry
3. **Freeze()** -- builds frozen snapshots for both registries
4. Hub begins accepting commands (RouteCommand, ProcessConfig)

Step 3 happens-before step 4 because they're sequential in the same goroutine. No additional synchronization needed.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Hub.RouteCommand after Freeze | -> | SchemaRegistry.FindHandler via frozen snapshot | `TestHubRouteCommandFrozen` |
| Hub.RouteCommand before Freeze | -> | SchemaRegistry.FindHandler via RLock fallback | `TestHubRouteCommandPreFreeze` |
| SubsystemManager.Get after Freeze | -> | Frozen snapshot lookup | `TestSubsystemManagerGetFrozen` |
| Unregister after Freeze | -> | New snapshot published, Get reflects removal | `TestUnregisterPublishesNewSnapshot` |
| Concurrent FindHandler after Freeze | -> | No lock contention, race-safe | `TestConcurrentFindHandlerFrozen` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | SchemaRegistry.Freeze() called | Subsequent FindHandler uses atomic.Load, not RLock |
| AC-2 | SubsystemManager.Freeze() called | Subsequent Get uses atomic.Load, not RLock |
| AC-3 | FindHandler called before Freeze | Falls back to RLock path, same behavior as today |
| AC-4 | Unregister called after Freeze | Mutable map updated AND new frozen snapshot published |
| AC-5 | Get after Unregister+Freeze | Returns nil for unregistered plugin, valid handler for others |
| AC-6 | Concurrent FindHandler from 100 goroutines after Freeze | Race detector clean, all return correct results |
| AC-7 | Hub.RouteCommand after Freeze | End-to-end: frozen lookup -> Handle -> correct response |
| AC-8 | Hub.ProcessConfig after Freeze | Sequential commands unchanged, uses frozen lookups |
| AC-9 | Frozen snapshot content | Matches pre-freeze mutable map exactly (same keys, same values) |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestSchemaRegistryFreeze` | `internal/component/plugin/server/schema_test.go` | Freeze creates snapshot, FindHandler works via atomic.Load | |
| `TestSchemaRegistryFreezeConsistency` | `internal/component/plugin/server/schema_test.go` | Frozen snapshot matches mutable map exactly | |
| `TestSchemaRegistryPreFreezeFallback` | `internal/component/plugin/server/schema_test.go` | FindHandler works before Freeze (RLock path) | |
| `TestSchemaRegistryFreezeIdempotent` | `internal/component/plugin/server/schema_test.go` | Calling Freeze twice is safe (second overwrites first) | |
| `TestSchemaRegistryConcurrentFindHandler` | `internal/component/plugin/server/schema_test.go` | 100 concurrent FindHandler calls, race-safe | |
| `TestSubsystemManagerFreeze` | `internal/component/plugin/server/subsystem_test.go` | Freeze creates snapshot, Get works via atomic.Load | |
| `TestSubsystemManagerPreFreezeFallback` | `internal/component/plugin/server/subsystem_test.go` | Get works before Freeze (RLock path) | |
| `TestSubsystemManagerUnregisterAfterFreeze` | `internal/component/plugin/server/subsystem_test.go` | Unregister updates mutable map + publishes new snapshot | |
| `TestSubsystemManagerConcurrentGet` | `internal/component/plugin/server/subsystem_test.go` | 100 concurrent Get calls, race-safe | |
| `TestHubRouteCommandFrozen` | `internal/component/plugin/server/hub_test.go` | RouteCommand uses frozen lookups, returns correct response | |

### Boundary Tests (MANDATORY for numeric inputs)

No numeric inputs in this spec. Freeze is a boolean transition (unfrozen -> frozen).

### Benchmark Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `BenchmarkFindHandlerFrozen` | `internal/component/plugin/server/schema_test.go` | Frozen path faster than RLock path (~4ns vs ~8ns) | |

### Functional Tests

No new functional test needed. Existing functional tests already exercise post-startup routing. Freeze is an internal implementation detail with zero observable behavior change -- a `.ci` test cannot distinguish frozen vs RLock path.

### Future (if deferring any tests)
- None

## Files to Modify

- `internal/component/plugin/server/schema.go` - Add frozen type, Freeze(), update FindHandler
- `internal/component/plugin/server/subsystem.go` - Add frozen type, Freeze(), update Get, update Unregister
- `internal/component/plugin/server/hub.go` - Call Freeze() at appropriate startup point (if Hub owns lifecycle)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | [ ] No | N/A |
| CLI commands/flags | [ ] No | N/A |
| API commands doc | [ ] No | N/A |
| Plugin SDK docs | [ ] No | N/A |
| Functional test | [ ] No -- freeze is internal, existing tests cover post-startup routing | N/A |

## Files to Create

No new files. Frozen types and Freeze() methods are added to existing files.

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan -- check what exists |
| 3. Implement (TDD) | Implementation phases below |
| 4. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 5. Critical review | Critical Review Checklist below |
| 6. Fix issues | Fix every issue from critical review |
| 7. Re-verify | Re-run stage 4 |
| 8. Repeat 5-7 | Max 2 review passes |
| 9. Deliverables review | Deliverables Checklist below |
| 10. Security review | Security Review Checklist below |
| 11. Re-verify | Re-run stage 4 |
| 12. Present summary | Executive Summary Report per `rules/planning.md` |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: SchemaRegistry freeze** -- Add frozen snapshot type, Freeze(), update FindHandler
   - Tests: `TestSchemaRegistryFreeze`, `TestSchemaRegistryFreezeConsistency`, `TestSchemaRegistryPreFreezeFallback`, `TestSchemaRegistryFreezeIdempotent`, `TestSchemaRegistryConcurrentFindHandler`
   - Files: `internal/component/plugin/server/schema.go`
   - Verify: tests fail -> implement -> tests pass

2. **Phase: SubsystemManager freeze** -- Add frozen snapshot type, Freeze(), update Get, update Unregister
   - Tests: `TestSubsystemManagerFreeze`, `TestSubsystemManagerPreFreezeFallback`, `TestSubsystemManagerUnregisterAfterFreeze`, `TestSubsystemManagerConcurrentGet`
   - Files: `internal/component/plugin/server/subsystem.go`
   - Verify: tests fail -> implement -> tests pass

3. **Phase: Hub integration** -- Call Freeze() at startup, verify RouteCommand uses frozen path
   - Tests: `TestHubRouteCommandFrozen`
   - Files: `internal/component/plugin/server/hub.go`
   - Verify: tests fail -> implement -> tests pass

4. **Functional tests** -> Create after feature works.
5. **Full verification** -> `make ze-verify`
6. **Complete spec** -> Fill audit tables, write learned summary, delete spec.

### Critical Review Checklist (/implement stage 5)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Frozen snapshot matches mutable map. Race detector clean. Unregister publishes new snapshot. |
| Naming | `frozenSchema`/`frozenSubsystems` types. `Freeze()` method. Clear and consistent. |
| Data flow | RouteCommand: atomic.Load -> prefix match -> Get -> Handle. No RLock on hot path. |
| Rule: no-layering | FindHandler has pre-freeze fallback (different lifecycle phase, not old+new layering). Both paths return same results. If no pre-freeze callers exist, remove fallback. |
| Rule: goroutine-lifecycle | No new goroutines. Freeze is a one-time synchronous call. |

### Deliverables Checklist (/implement stage 9)

| Deliverable | Verification method |
|-------------|---------------------|
| SchemaRegistry.Freeze() | `grep 'func.*SchemaRegistry.*Freeze' internal/component/plugin/server/schema.go` |
| frozenSchema type | `grep 'frozenSchema' internal/component/plugin/server/schema.go` |
| SubsystemManager.Freeze() | `grep 'func.*SubsystemManager.*Freeze' internal/component/plugin/server/subsystem.go` |
| frozenSubsystems type | `grep 'frozenSubsystems' internal/component/plugin/server/subsystem.go` |
| Unregister publishes snapshot | `grep 'atomic.Store\|frozen.Store' internal/component/plugin/server/subsystem.go` |
| All tests pass | `make ze-verify` |

### Security Review Checklist (/implement stage 10)

| Check | What to look for |
|-------|-----------------|
| Race conditions | Concurrent FindHandler/Get after Freeze -- race detector must pass |
| Snapshot consistency | Frozen map containers must be shallow-copied (new map, same value pointers) so mutable map mutations (add/delete keys) do not corrupt snapshot |
| Unregister safety | atomic.Store of new snapshot concurrent with atomic.Load from readers -- safe by Go memory model |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior -> RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural -> DESIGN phase |
| Functional test fails | Check AC; if AC wrong -> DESIGN; if AC correct -> IMPLEMENT |
| Audit finds missing AC | Back to relevant phase and implement |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| Hub RLocks are a performance bottleneck | Near-zero contention (no writers after startup) | Critical review: RWMutex allows unlimited concurrent readers | Reframed as correctness/clarity improvement, not performance |
| Hub needs a worker pool for concurrent dispatch | Callers already run on independent goroutines, MuxConn already concurrent | Critical review: pool adds goroutines + latency without improving concurrency | Removed pool from this spec, separate spec for pool infrastructure |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|
| Plugin-scoped request IDs | Per-connection IDs already sufficient | Unchanged per-connection atomic IDs |
| Hub-level per-plugin RWMutex | Creates new contention (slow reads block writes) | No new Hub-level locks |
| Per-key FIFO dispatch pool in Hub | Serializes same-plugin commands, worse than today | Pool extracted to separate spec, Hub uses freeze-after-init only |
| Hub wrapping RouteCommand in pool Submit | Adds goroutines + latency for zero concurrency gain | Hub stays synchronous on caller goroutine |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |

## Design Insights

- Freeze-after-init is primarily a correctness/clarity improvement, not a performance optimization. The ~12ns saved per RouteCommand is negligible, but the explicit immutability invariant prevents future bugs where someone adds a write path during operation.
- The pre-freeze fallback (RLock path) is precautionary. RouteCommand is only called after startup, so the hot path always uses the frozen snapshot. The fallback exists for potential internal startup calls. Verify during implementation whether any pre-freeze FindHandler calls exist -- if not, the fallback is defensive dead code (harmless, 2 lines).
- Only FindHandler (dispatch hot path) needs the frozen path. Other SchemaRegistry read methods (ListRPCs, FindRPC, etc.) are CLI/query operations and can stay on the RLock path without concern.

## Implementation Summary

### What Was Implemented
- [List actual changes made]

### Bugs Found/Fixed
- [Any bugs discovered -- add test for each]

### Documentation Updates
- [Docs updated, or "None"]

### Deviations from Plan
- [Differences from original plan and why]

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |

### Tests from TDD Plan
| Test | Status | Location | Notes |

### Files from Plan
| File | Status | Notes |

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated (N/A if no behavior change -- freeze is internal)
- [ ] Critical Review passes (all 6 checks in `rules/quality.md` -- no failures)

### Quality Gates (SHOULD pass -- defer with user approval)
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

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes -- all 6 checks in `rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Summary included in commit** -- NEVER commit implementation without the completed summary. One commit = code + tests + summary.
