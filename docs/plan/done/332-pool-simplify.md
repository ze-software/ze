# Spec: pool-simplify

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/pool-architecture.md` - pool design
4. `internal/attrpool/handle.go` - handle bit layout
5. `internal/plugins/bgp-rib/rib.go` - plugin entry point

## Task

Two problems in the pool subsystem:

**Problem A — Compaction not wired in:** The pool has compaction infrastructure (Scheduler, StartCompaction/MigrateBatch, double-buffer) but nothing ever starts it. In a long-running daemon with route churn, dead bytes accumulate in pool buffers permanently. Slot reuse works (freeSlots), but the underlying buffer is append-only — dead data gaps are never reclaimed.

**Problem B — Handle carries dead NLRI bits:** The Handle bit layout reserves 2 flag bits (HasPathID for ADD-PATH). These were designed for NLRI pooling which never happened (NLRI stored as map keys, not pooled). The flags are always zero in production (`Intern()` at pool.go:244 always passes flags=0).

**Goal:** Remove the unused flag bits from the handle, and wire the existing compaction scheduler into the RIB plugin lifecycle so it actually runs.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/pool-architecture.md` - pool design, handle layout, compaction model
  → Constraint: Pool lives in API program (RIB plugin), not engine
  → Decision: Per-attribute-type pools (13 instances, idx 2-14) for fine-grained dedup
  → Constraint: Scheduler exists and is tested — needs wiring, not rewriting

### RFC Summaries (MUST for protocol work)
N/A — this is infrastructure, not protocol work.

**Key insights:**
- Pool is used only for BGP attribute dedup, never for NLRI
- Compaction exists but is never started — buffer memory leaks under route churn
- Handle flags (2 bits) are always zero in production — designed for NLRI that isn't pooled
- RIB plugin has `OnStarted(ctx)` callback — safe place to start scheduler goroutine
- Plugin shutdown propagates via context cancellation + socket close
- Existing Scheduler uses MigrateBatch (incremental, non-blocking) — already tested and correct

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/attrpool/handle.go` (115L) — Handle uint32 with BufferBit(1) + PoolIdx(5) + Flags(2) + Slot(24)
  → Constraint: InvalidHandle = 0xFFFFFFFF (poolIdx=31)
  → Constraint: IsValid() checks poolIdx < 31
  → Constraint: Flags always passed as 0 — HasPathID never set
- [ ] `internal/attrpool/pool.go` (746L) — Pool struct with double-buffer, Intern/Get/Release, Compact(), StartCompaction/MigrateBatch
  → Constraint: Intern() creates handles with flags=0 (line 244: `NewHandleWithBuffer(bufIdx, p.idx, 0, slotIdx)`)
  → Constraint: MigrateBatch also hardcodes flags=0 (line 512: `NewHandleWithBuffer(newBit, p.idx, 0, p.compactCursor)`)
- [ ] `internal/attrpool/scheduler.go` (153L) — Round-robin scheduler, uses MigrateBatch for incremental compaction
  → Constraint: Respects QuietPeriod via Touch()/IsIdle()
  → Constraint: Only one pool compacts at a time — already correct design
- [ ] `internal/plugins/bgp-rib/pool/attributes.go` (68L) — 13 global pool instances, idx 2-14
- [ ] `internal/plugins/bgp-rib/rib.go` — RIBManager, RunRIBPlugin entry, OnStarted/OnBye callbacks available
  → Constraint: OnStarted(ctx) runs after 5-stage startup, before event loop — safe for goroutine launch

**Behavior to preserve:**
- All pool operations: Intern/Get/Release/AddRef/Compact/StartCompaction/MigrateBatch
- Scheduler logic (round-robin, quiet period, dead ratio threshold)
- Double-buffer compaction model (BufferBit in handle stays — it's used by the compaction system)
- Per-attribute dedup (13 pools)
- All existing tests
- `InvalidHandle` sentinel (0xFFFFFFFF)
- `Handle.PoolIdx()` for cross-pool validation
- `Handle.Slot()` for slot lookup

**Behavior to change:**
- Handle layout: remove Flags (2 bits), giving BufferBit(1) + PoolIdx(5) + Slot(26)
- Remove Flags-related methods: `Flags()`, `HasPathID()`, `WithFlags()`
- Remove `flags` parameter from `NewHandle()` and `NewHandleWithBuffer()`
- Wire existing Scheduler into RIB plugin lifecycle via OnStarted callback
- Export pool list from `pool/attributes.go` for scheduler construction

## Data Flow (MANDATORY)

### Entry Point
- Route UPDATE arrives at RIB plugin as JSON event with base64 wire bytes
- `ParseAttributes(attrBytes)` iterates attributes, calls `pool.X.Intern(value)` per type
- Returns RouteEntry with 13 Handle fields

### Transformation Path
1. JSON event decoded → raw attribute wire bytes
2. `ParseAttributes()` iterates wire bytes → per-attribute `Intern()` calls
3. Pool dedup check: existing → refCount++; new → append to buffer, allocate slot
4. RouteEntry stored in FamilyRIB.routes map keyed by NLRI bytes
5. On withdraw: `Release()` on all handles → refCount--, dead if 0
6. **NEW:** Scheduler goroutine periodically checks dead ratio → StartCompaction/MigrateBatch → reclaims dead buffer gaps

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine → RIB plugin | JSON events over unix socket | [ ] |
| RIB plugin → Pool | Direct function calls (in-process) | [ ] |
| Pool → Buffer | Append-only write, offset-based read | [ ] |

### Integration Points
- `RIBManager` in `rib.go` — starts scheduler in OnStarted, stops via context cancellation
- `pool/attributes.go` — 13 global pool instances, new `AllPools()` function for scheduler
- `attrpool.NewScheduler()` — existing scheduler, already tested
- `sdk.Plugin.OnStarted(ctx)` — safe startup point for background goroutine

### Architectural Verification
- [ ] No bypassed layers (scheduler calls pool methods, RIB plugin starts scheduler)
- [ ] No unintended coupling (scheduler is attrpool-internal, RIB just constructs and runs it)
- [ ] No duplicated functionality (reuses existing Scheduler, does not rewrite)
- [ ] Zero-copy preserved (Get() still returns slice into pool buffer)

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| RIB plugin startup (OnStarted) | → | Scheduler.Run starts | `TestCompactionSchedulerStartsOnPluginStartup` |
| Route churn (Intern + Release) | → | Scheduler triggers compaction, dead bytes reclaimed | `TestSchedulerCompactsAfterChurn` |
| Plugin shutdown (context cancel) | → | Scheduler.Run exits | `TestCompactionSchedulerStopsOnShutdown` |
| Handle without Flags field | → | Intern/Get/Release work with new layout | `TestHandleWithoutFlags` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | NewHandle(poolIdx, slot) with no flags param | PoolIdx and Slot extractable, handle valid |
| AC-2 | NewHandleWithBuffer(bufferBit, poolIdx, slot) with no flags param | BufferBit, PoolIdx, Slot extractable |
| AC-3 | RIB plugin starts | Scheduler goroutine running, tied to plugin context |
| AC-4 | Plugin shuts down (context cancel) | Scheduler exits cleanly, no goroutine leak |
| AC-5 | Route churn: intern entries, release some, wait for scheduler tick | Dead ratio drops, buffer bytes reclaimed |
| AC-6 | Existing Intern/Get/Release/Compact tests pass | No regression |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestHandleWithoutFlags` | `internal/attrpool/handle_test.go` | AC-1, AC-2: Handle layout encodes/decodes without flags | |
| `TestHandleWithoutFlagsInvalidHandle` | `internal/attrpool/handle_test.go` | AC-1: InvalidHandle sentinel still works | |
| `TestHandleWithoutFlagsMaxSlot` | `internal/attrpool/handle_test.go` | AC-1: 26-bit slot max value (67,108,863) | |
| `TestCompactionSchedulerStartsOnPluginStartup` | `internal/plugins/bgp-rib/compaction_test.go` | AC-3: Scheduler starts in OnStarted | |
| `TestCompactionSchedulerStopsOnShutdown` | `internal/plugins/bgp-rib/compaction_test.go` | AC-4: Scheduler exits on context cancel | |
| `TestSchedulerCompactsAfterChurn` | `internal/plugins/bgp-rib/compaction_test.go` | AC-5: Dead bytes reclaimed after churn | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| PoolIdx | 0-30 | 30 | N/A (uint8) | 31 (reserved for InvalidHandle) |
| Slot | 0-0x3FFFFFF | 0x3FFFFFF (67,108,863) | N/A (uint32) | 0x4000000 (truncated to 26 bits) |

### Functional Tests
No new functional tests — internal infrastructure. Existing functional tests exercise pool indirectly through RIB operations and validate no regression.

### Future (if deferring any tests)
None — all tests listed above are required.

## Files to Modify

- `internal/attrpool/handle.go` — Remove Flags field: BufferBit(1) + PoolIdx(5) + Slot(26). Remove Flags(), HasPathID(), WithFlags(). Update NewHandle/NewHandleWithBuffer signatures (drop flags param).
- `internal/attrpool/pool.go` — Update calls to NewHandle/NewHandleWithBuffer (remove flags=0 arg). No other changes — double-buffer and all methods preserved.
- `internal/attrpool/handle_test.go` — Update tests for new layout, remove flags-related tests.
- `internal/plugins/bgp-rib/rib.go` — Wire scheduler: construct with AllPools() in OnStarted, run in goroutine with plugin context.
- `internal/plugins/bgp-rib/pool/attributes.go` — Add `AllPools() []*attrpool.Pool` returning all 13 pools.
- `docs/architecture/pool-architecture.md` — Update handle layout diagram (remove Flags bits), add scheduler wiring section.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | N/A |
| RPC count in architecture docs | No | N/A |
| CLI commands/flags | No | N/A |
| CLI usage/help text | No | N/A |
| API commands doc | No | N/A |
| Plugin SDK docs | No | N/A |
| Editor autocomplete | No | N/A |
| Functional test for new RPC/API | No | N/A |

## Files to Create

- `internal/plugins/bgp-rib/compaction.go` — Thin wiring: constructs Scheduler with AllPools() and default config, runs it.
- `internal/plugins/bgp-rib/compaction_test.go` — Wiring tests: scheduler starts, stops, compacts under churn.

## Implementation Steps

### Phase 1: Remove Handle Flags (Problem B)

1. **Write handle tests** for new layout: BufferBit(1) + PoolIdx(5) + Slot(26), boundary at 0x3FFFFFF, InvalidHandle preserved
2. **Run tests** → verify FAIL
3. **Modify `handle.go`**: remove Flags field (2 bits), remove Flags(), HasPathID(), WithFlags(); update NewHandle to `NewHandle(poolIdx uint8, slot uint32)`; update NewHandleWithBuffer to `NewHandleWithBuffer(bufferBit uint32, poolIdx uint8, slot uint32)`; shift slot mask from 24 to 26 bits
4. **Update `pool.go`**: remove flags=0 argument from NewHandle/NewHandleWithBuffer calls (lines 244, 512, 618)
5. **Update existing handle tests** that reference Flags
6. **Run tests** → verify PASS

### Phase 2: Wire Compaction into RIB Plugin (Problem A)

1. **Write wiring tests**: scheduler lifecycle (start/stop with context), triggers compaction under churn
2. **Run tests** → verify FAIL
3. **Add `AllPools()` to `pool/attributes.go`** — returns slice of all 13 pools
4. **Create `compaction.go`** in `internal/plugins/bgp-rib/`: thin function that constructs `attrpool.NewScheduler(AllPools(), config)` and calls `scheduler.Run(ctx)`
5. **Wire into `rib.go`**: register `OnStarted()` callback that launches compaction goroutine with the plugin context
6. **Run tests** → verify PASS

### Phase 3: Update Documentation

1. **Update `docs/architecture/pool-architecture.md`**: new handle layout diagram, add scheduler wiring section describing RIB plugin lifecycle integration
2. **Run `make test-all`** → verify everything passes

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Phase that introduced it (fix syntax/types) |
| Test fails wrong reason | Fix test |
| Test fails behavior mismatch | Re-read source from Current Behavior |
| Lint failure | Fix inline |
| Existing tests break | Likely missed a caller of changed API — grep and update |

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

- Existing Scheduler and MigrateBatch are already tested and correct — the problem is purely wiring, not logic
- OnStarted(ctx) is the right lifecycle hook: runs after 5-stage startup (Socket A free), context tied to plugin lifetime
- AllPools() export is the minimal coupling needed — scheduler receives pool slice, doesn't import RIB internals

## RFC Documentation

N/A — infrastructure change, no protocol impact.

## Implementation Summary

### What Was Implemented
- Removed 2-bit Flags field from Handle: layout now BufferBit(1) + PoolIdx(5) + Slot(26)
- Removed Flags(), HasPathID(), WithFlags() methods from Handle
- Updated NewHandle/NewHandleWithBuffer signatures (dropped flags parameter)
- Updated 3 call sites in pool.go (Intern, MigrateBatch, Compact)
- Updated pool_test.go (WithFlags→WithBufferBit in TestPoolExtractsSlot)
- Rewrote handle_test.go with new layout tests, boundary tests, fuzz tests
- Created compaction.go: thin wiring function runCompaction(ctx, pools)
- Created compaction_test.go: 3 wiring tests (start, stop, churn)
- Added AllPools() to pool/attributes.go returning all 13 attribute pools
- Wired scheduler into rib.go via OnStarted callback
- Updated pool-architecture.md: handle layout, API summary, scheduler wiring section

### Bugs Found/Fixed
- pool_test.go:367 referenced removed WithFlags() — replaced with PoolIdx assertion

### Documentation Updates
- pool-architecture.md: handle diagram (3 fields not 4), bit widths, API summary, scheduler wiring section, slot bit reference (24→26)

### Deviations from Plan
- None

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Remove handle flags bits | ✅ Done | handle.go:39-56 | BufferBit(1)+PoolIdx(5)+Slot(26) |
| Wire compaction scheduler into RIB plugin | ✅ Done | rib.go:108-112, compaction.go | OnStarted → go runCompaction |
| Preserve all existing pool operations | ✅ Done | pool.go, scheduler.go | All preserved, only flags param removed |
| Update architecture docs | ✅ Done | pool-architecture.md | Handle layout, API, wiring section |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | ✅ | TestHandleWithoutFlags, TestHandleBoundary | NewHandle(poolIdx, slot) round-trips |
| AC-2 | ✅ | TestHandleWithoutFlags, TestHandleWithBufferBit | NewHandleWithBuffer(bufferBit, poolIdx, slot) round-trips |
| AC-3 | ✅ | TestCompactionSchedulerStartsOnPluginStartup | Scheduler runs and compacts dead entries |
| AC-4 | ✅ | TestCompactionSchedulerStopsOnShutdown | Context cancel stops goroutine cleanly |
| AC-5 | ✅ | TestSchedulerCompactsAfterChurn | Dead bytes reclaimed, live data accessible |
| AC-6 | ✅ | make test-all | All existing tests pass |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestHandleWithoutFlags` | ✅ | handle_test.go:72 | 7 subtests: zero, max, mid, boundary values |
| `TestHandleWithoutFlagsInvalidHandle` | ✅ | handle_test.go:102 | Verifies poolIdx=31 sentinel |
| `TestHandleWithoutFlagsMaxSlot` | ✅ | handle_test.go:146 | 26-bit max (0x3FFFFFF) |
| `TestCompactionSchedulerStartsOnPluginStartup` | ✅ | compaction_test.go:21 | AC-3 wiring test |
| `TestCompactionSchedulerStopsOnShutdown` | ✅ | compaction_test.go:53 | AC-4 goroutine lifecycle |
| `TestSchedulerCompactsAfterChurn` | ✅ | compaction_test.go:87 | AC-5 dead byte reclamation |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/attrpool/handle.go` | ✅ Done | Removed Flags field, updated layout |
| `internal/attrpool/pool.go` | ✅ Done | Updated 3 NewHandleWithBuffer calls |
| `internal/attrpool/handle_test.go` | ✅ Done | Rewritten for new layout + boundary + fuzz |
| `internal/plugins/bgp-rib/rib.go` | ✅ Done | OnStarted wiring + pool import + cross-ref |
| `internal/plugins/bgp-rib/compaction.go` | ✅ Created | runCompaction thin wiring |
| `internal/plugins/bgp-rib/compaction_test.go` | ✅ Created | 3 wiring tests |
| `internal/plugins/bgp-rib/pool/attributes.go` | ✅ Done | Added AllPools() |
| `docs/architecture/pool-architecture.md` | ✅ Done | Handle layout, API, wiring section |

### Audit Summary
- **Total items:** 22 (4 requirements, 6 ACs, 6 tests, 8 files, minus 2 overlap)
- **Done:** 22
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 0

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-6 all demonstrated
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `make test-all` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes (all 6 checks in `rules/quality.md` — no failures)

### Quality Gates (SHOULD pass — defer with user approval)
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
- [ ] Critical Review passes — all 6 checks in `rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Spec moved to `docs/plan/done/NNN-pool-simplify.md`
- [ ] **Spec included in commit** — NEVER commit implementation without the completed spec. One commit = code + tests + spec.
