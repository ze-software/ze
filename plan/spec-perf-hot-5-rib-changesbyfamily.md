# Spec: perf-hot-5-rib-changesbyfamily

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | - |
| Updated | 2026-06-05 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/component/bgp/plugins/rib/rib_structured.go` - handleReceivedStructured
4. `internal/component/bgp/plugins/rib/rib_bestchange.go` - bestChangeEntry, publishBestChanges

## Task

Replace the per-UPDATE `changesByFamily` map allocation and the per-UPDATE `affected` slice
allocation in `rib.handleReceivedStructured` with stack-friendly alternatives. The map is
created on every UPDATE at line 233 but in practice nearly all UPDATEs carry a single address
family (ipv4/unicast), making the map overhead unnecessary. The `affected` slice at line 88
allocates 128 entries (about 5 KB) per UPDATE.

Part of spec set `perf-hot` (umbrella: `spec-perf-hot-0-umbrella.md`).
Profiling evidence in the umbrella spec.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/plugin/rib-storage-design.md` - RIB plugin structured delivery
  -> Constraint: RIB processes one UPDATE at a time per source peer; multi-family UPDATEs are rare but valid
- [ ] `internal/component/bgp/plugins/rib/rib_bestchange.go` lines 25-33 - bestChangeEntry is an alias to ribevents.BestChangeEntry
  -> Decision: changesByFamily groups entries for publishBestChanges which needs one batch per family

### RFC Summaries (MUST for protocol work)
- Not applicable (internal allocation optimization, no wire behavior change)

**Key insights:**
- `changesByFamily` exists because `publishBestChanges` emits one `BestChangeBatch` per family
- Nearly all UPDATEs carry a single family; multi-family UPDATEs happen only when legacy IPv4 NLRI coexists with MP_REACH for another family in the same message
- The `affected` slice is local to the function, append-only, and not retained
- `bestChangeEntry` is about 80 bytes (netip.Prefix, netip.Addr, slices for Labels and ASPath)

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/rib/rib_structured.go` lines 51-250 - handleReceivedStructured
  -> Constraint: Phase 3 (lines 224-249) iterates `affected` slice, calls `checkBestPathChange` per prefix, groups results into `changesByFamily` map, then publishes one batch per family
- [ ] `internal/component/bgp/plugins/rib/rib_structured.go` lines 40-45 - affectedPrefix struct (fam + nlriBytes + addPath)
- [ ] `internal/component/bgp/plugins/rib/rib_bestchange.go` lines 1172-1186 - publishBestChanges takes a slice of bestChangeEntry and a family

**Behavior to preserve:**
- `publishBestChanges` receives one batch per family, never mixed families
- All affected prefixes from a single UPDATE are processed before any batch is published
- Order of families in publication does not matter
- `checkBestPathChange` acquires its own locks (peerMu.RLock, shard.mu.Lock); no external lock dependency from the caller

**Behavior to change:**
- Replace `make(map[family.Family][]bestChangeEntry)` at line 233 with a single-family fast path
- Pool the `affected` slice to amortize the per-UPDATE 5 KB allocation

## Data Flow (MANDATORY)

### Entry Point
- Wire UPDATE bytes arrive via `StructuredEvent` from `DirectBridge`

### Transformation Path
1. `dispatchStructured` routes to `handleReceivedStructured` for received UPDATEs
2. Phase 1: peer slot creation under `peerMu.Lock` (brief)
3. Phase 2: NLRI parsing and PeerRIB insert/remove; `affected` slice accumulates entries
4. Phase 3: iterate `affected`, call `checkBestPathChange` per prefix, group results by family, publish batches

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| RIB plugin -> EventBus | `publishBestChanges` emits `BestChangeBatch` via typed handle | [ ] |

### Integration Points
- `checkBestPathChange` (rib_bestchange.go:694) - returns `(bestChangeEntry, bool)` per prefix
- `publishBestChanges` (rib_bestchange.go:1172) - emits batch to EventBus per family

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Received UPDATE with single family | -> | single-family fast path publishes correctly | `TestHandleReceivedSingleFamilyFastPath` |
| Received UPDATE with two families (legacy IPv4 + MP_REACH) | -> | multi-family spill publishes both batches | `TestHandleReceivedMultiFamilySpill` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Single-family UPDATE (IPv4 unicast, 100 NLRIs) | No map allocation; changes collected in a single slice and published as one batch |
| AC-2 | Multi-family UPDATE (legacy IPv4 + MP_REACH IPv6) | Falls back to map grouping; publishes one batch per family |
| AC-3 | UPDATE with zero best-path changes (all same-best short-circuit) | No batch published; no map or slice allocated beyond the fast-path variable |
| AC-4 | `affected` slice pooled via sync.Pool | Pool Get/Put on every call; no 5 KB allocation per UPDATE after warmup |
| AC-5 | Benchmark allocation reduction | `go test -bench -memprofile` shows zero map allocs for single-family UPDATEs |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestHandleReceivedSingleFamilyFastPath` | `internal/component/bgp/plugins/rib/rib_structured_test.go` | AC-1: single family uses fast path, not map | |
| `TestHandleReceivedMultiFamilySpill` | `internal/component/bgp/plugins/rib/rib_structured_test.go` | AC-2: two families spill to map, both batches published | |
| `TestHandleReceivedZeroChanges` | `internal/component/bgp/plugins/rib/rib_structured_test.go` | AC-3: no changes means no allocation beyond stack vars | |
| `TestAffectedSlicePoolReuse` | `internal/component/bgp/plugins/rib/rib_structured_test.go` | AC-4: pool returns and reuses slice | |
| `BenchmarkHandleReceivedStructured` | `internal/component/bgp/plugins/rib/rib_structured_test.go` | AC-5: zero map allocs per op for single family | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Number of families per UPDATE | 1-4 | 4 | N/A (min 1 NLRI section) | N/A (protocol max) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| Existing functional tests | `test/plugin/*.ci` | Best-path change events still emitted correctly | |

No new functional tests required. The change is internal allocation strategy; observable behavior (best-change events) is unchanged. Existing functional tests in `test/plugin/` validate best-path event emission.

### Interop Tests (MANDATORY for protocol features)
Not applicable. No wire protocol change.

### Future (if deferring any tests)
- None

## Files to Modify
- `internal/component/bgp/plugins/rib/rib_structured.go` - replace `changesByFamily` map with single-family fast path; pool `affected` slice

## Files to Create
- `internal/component/bgp/plugins/rib/rib_structured_test.go` - if not already present, add test file for structured handler allocation tests

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | No | |
| YANG validation constraints | No | |
| YANG custom validators | No | |
| CLI commands/flags | No | |
| CLI grammar (action before identifier) | No | |
| Editor autocomplete | No | |
| Functional test for new RPC/API | No | |
| Pipe completeness | No | |
| Env var registration | No | |
| Doctor check for runtime dependencies | No | |
| Prometheus counters/metrics | No | |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | No | |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented? | No | |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | No | |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | |
| 15 | Registered plugin, event type, send type, command, capability, or runtime inventory changed? | No | |
| 16 | Any changed source file is referenced by existing doc source anchors? | No | |
| 17 | Existing docs show config/CLI/API examples for this area? | No | |

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 7. Critical review | Critical Review Checklist below |
| 8. Fix issues | Fix every issue from critical review |
| 9. Re-verify | Re-run stage 6 |
| 10. Repeat 7-9 | Until clean |
| 11. Deliverables review | Deliverables Checklist below |
| 12. Security review | Security Review Checklist below |
| 13. Re-verify | Re-run stage 6 |
| 14. Present summary | Executive Summary Report |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring (MANDATORY FIRST)** - pool the affected slice
   - Tests: `TestAffectedSlicePoolReuse`
   - Files: `internal/component/bgp/plugins/rib/rib_structured.go`
   - Verify: pool Get/Put replaces make; test confirms reuse

2. **Phase: Single-family fast path** - replace changesByFamily map
   - Tests: `TestHandleReceivedSingleFamilyFastPath`, `TestHandleReceivedMultiFamilySpill`, `TestHandleReceivedZeroChanges`
   - Files: `internal/component/bgp/plugins/rib/rib_structured.go`
   - Verify: single-family UPDATE avoids map; multi-family UPDATE still works

3. **Phase: Benchmark** - confirm zero map allocs
   - Tests: `BenchmarkHandleReceivedStructured`
   - Files: `internal/component/bgp/plugins/rib/rib_structured_test.go`
   - Verify: `go test -bench -memprofile` shows improvement

4. **Functional tests** - run existing functional tests to confirm no regression
5. **Full verification** - `make ze-verify`
6. **Complete spec** - fill audit tables, write learned summary

### Critical Review Checklist (/implement stage 6)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | AC-1 through AC-5 all have implementation with file:line |
| Correctness | Multi-family spill produces correct batches; pool does not leak stale data |
| Data flow | `publishBestChanges` still receives per-family slices, never mixed |
| Rule: no-layering | Old map code fully replaced, not wrapped |

### Deliverables Checklist (/implement stage 10)

| Deliverable | Verification method |
|-------------|---------------------|
| Single-family fast path in handleReceivedStructured | `grep -n 'singleFam\|singleChanges' rib_structured.go` |
| sync.Pool for affected slice | `grep -n 'affectedPool\|sync.Pool' rib_structured.go` |
| Unit tests for fast path and spill | `go test -run TestHandleReceived -v` |
| Benchmark showing zero map allocs | `go test -bench BenchmarkHandleReceivedStructured -benchmem` |

### Security Review Checklist (/implement stage 11)

| Check | What to look for |
|-------|-----------------|
| Pool poisoning | Verify pooled slice is reset to length 0 before reuse; no stale data leaks |
| Unbounded growth | Verify pooled slice cap does not grow unboundedly across calls |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior |
| Lint failure | Fix inline |
| Functional test fails | Check AC; if AC wrong, redesign; if AC correct, fix implementation |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

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

## Core Insight

The `changesByFamily` map is the last remaining per-UPDATE map allocation on the
RIB hot path. Since BGP UPDATEs carry a single family in >99% of real traffic,
replacing the map with a stack variable for the common case eliminates one
heap allocation per UPDATE while preserving correct multi-family behavior via
a spill path.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Single-family fast path with spill to map | Fixed-size array [4]familyChanges; always use map with pre-alloc | Fast path covers >99% of traffic with zero alloc; spill path reuses existing map logic for correctness; fixed-size array is harder to reason about edge cases |
| sync.Pool for affected slice | Grow-only field on RIBManager; stack array with spill | Pool amortizes across concurrent goroutines; field on RIBManager would serialize; stack array limited to 128 entries |

## Known Limitations
- Pool cap growth: if a single UPDATE carries many NLRIs (e.g., full table), the pooled slice grows and stays large. Acceptable because pool entries are eventually GC'd.
- This is the lowest-impact optimization in the perf-hot set (about 2 MB of the 1.09 GB total). Impact is primarily in reducing GC scan work, not raw allocation volume.

## RFC Documentation

Not applicable. No protocol behavior change.

## Implementation Summary

### What Was Implemented
- [To be filled after implementation]

### Bugs Found/Fixed
- [To be filled after implementation]

### Documentation Updates
- [To be filled after implementation]

### Deviations from Plan
- [To be filled after implementation]

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|

### Files from Plan
| File | Status | Notes |
|------|--------|-------|

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:**
- **Skipped:**
- **Changed:**

## Goal Validation (BLOCKING)

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Zero map allocs for single-family UPDATEs | benchmark | BenchmarkHandleReceivedStructured shows 0 allocs/op for map |
| Pooled affected slice reduces per-UPDATE allocation | benchmark | BenchmarkHandleReceivedStructured shows reduced allocs/op |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied
-

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-5 all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean (Review Gate section filled)
- [ ] `make ze-test` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Architecture docs and guides updated where changed behavior is documented
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)
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
- [ ] Interop tests for protocol features (or N/A with justification)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/<spec>` only
