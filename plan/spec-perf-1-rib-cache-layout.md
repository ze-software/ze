# Spec: RIB Attribute Bundle and AS_PATH Separation

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | performance |
| Updated | 2026-05-15 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/plugin/rib-storage-design.md` - RIB internals
4. `docs/research/attribute-bundle-dedup.md` - analysis data
5. `internal/component/bgp/plugins/rib/storage/routeentry.go` - current layout
6. `internal/component/bgp/attrpool/pool.go` - pool internals

## Task

Restructure the RIB route entry to separate AS_PATH from non-AS_PATH attributes,
improving CPU cache utilisation during RIB scans (best-path, filter evaluation, show
commands). Optionally introduce an attribute bundle that groups the non-AS_PATH
handles into a single shared reference.

### Motivation

Analysis of full-table MRT dumps (RIPE RIS rrc00, LINX rrc01, RouteViews) shows:

| Metric | Value |
|---|---|
| Routes sharing identical non-AS_PATH attributes | 97% (1.7M unique / 55M) |
| Routes sharing identical full AS_PATH | 87% (7M unique / 55M) |
| AS_PATH trie suffix compression | 95.7-96.4% across all vantage points |

The current `RouteEntry` is 53 bytes (13 handles + stale). It barely fits one
cache line. Best-path selection never reads the AS_PATH handle (ASPathLen and
FirstAS are pre-extracted into Candidate), yet every RIB scan drags it through
cache. Separating AS_PATH and bundling the shared non-AS_PATH attributes
reduces per-route size and increases cache density.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/plugin/rib-storage-design.md` - RIB storage internals
  → Constraint: FamilyRIB picks backend from (CIDR, ADD-PATH) pair
  → Constraint: BART store for CIDR families, map for non-CIDR
- [ ] `docs/architecture/pool-architecture.md` - pool design
  → Constraint: Handle is 32-bit, pool index 5 bits, slot 26 bits
  → Constraint: Pools are refcounted, support compaction
- [ ] `docs/architecture/core-design.md` Section 4 - pool rationale
  → Decision: per-attribute-type pools for fine-grained dedup
- [ ] `docs/research/attribute-bundle-dedup.md` - analysis results
  → Decision: 97% non-AS_PATH sharing justifies bundling
  → Decision: AS_PATH trie compression 95-96% (future work, not this spec)

### RFC Summaries
- [ ] `rfc/short/rfc4271.md` - best-path selection
  → Constraint: best-path uses LOCAL_PREF, AS_PATH length, ORIGIN, MED, router-id
  → Constraint: AS_PATH length comparison, not AS_PATH bytes, is on the hot path

**Key insights:**
- Best-path never reads AS_PATH bytes from pool (uses pre-extracted ASPathLen/FirstAS)
- 97% of routes share non-AS_PATH attributes, making bundling effective
- AS_PATH pool already deduplicates at 87%; trie is future work, not this spec
- RouteEntry is value type stored inline in BART/map; size directly affects scan performance
- `ASPath` field on RouteEntry has 25 references across 8 files (LSP verified)
- FamilyRIB already has a parallel BART for labels (`labels *store.Store[attrpool.Handle]`)

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/rib/storage/routeentry.go` - 13 per-attr handles + stale byte
  → Constraint: RouteEntry is a value type (not pointer), stored inline in BART/map
  → Constraint: Release/AddRef/Clone manage refcounts for all 13 handles
- [ ] `internal/component/bgp/plugins/rib/storage/attrparse.go` - ParseAttributes populates RouteEntry from wire
  → Constraint: attrInterners table maps type code to pool+field binding
  → Constraint: AS_PATH interned via `pool.ASPath` at type code 2
- [ ] `internal/component/bgp/plugins/rib/storage/familyrib.go` - BART store typed on RouteEntry
  → Constraint: `direct *store.Store[RouteEntry]` for CIDR, `opaque map[string]RouteEntry` for non-CIDR
  → Constraint: `labels *store.Store[attrpool.Handle]` is existing parallel BART pattern
- [ ] `internal/component/bgp/plugins/rib/storage/peerrib.go` - per-peer RIB, delegates to FamilyRIB
- [ ] `internal/component/bgp/plugins/rib/bestpath.go` - Candidate struct with pre-extracted fields
  → Constraint: Candidate.ASPathHandle used only for multipath content equality check
  → Constraint: ASPathLen and FirstAS are ints, not pool reads, during ComparePair
- [ ] `internal/component/bgp/plugins/rib/pool/attributes.go` - 13 per-type pool instances
  → Constraint: ASPath pool initial capacity 1<<18 (262144)
- [ ] `internal/component/bgp/attrpool/handle.go` - Handle is uint32

**ASPath references (25 across 8 files, LSP-verified):**

| File | Lines | Usage |
|------|-------|-------|
| `storage/routeentry.go` | 39,65,84,126-128,204,258 | field def, init, Has, Release, AddRef, Clone |
| `storage/attrparse.go` | 37-38 | interner registration |
| `storage/familyrib.go` | 589,648 | extractCandidate, route access |
| `rib_attr_format.go` | 36 | formatting for display |
| `rib_commands.go` | 961-962 | extractCandidate |
| `rib_pipeline.go` | 401,624 | pipeline processing |

**Behavior to preserve:**
- Handle type and pool interface (Intern/Get/Release/AddRef)
- Best-path selection logic (Candidate extraction, ComparePair)
- ParseAttributes API (raw bytes in, structured route out)
- FamilyRIB backend selection (BART for CIDR, map for non-CIDR)
- Refcounting semantics (AddRef/Release on all handles)
- All existing per-type pools remain (Origin, NextHop, etc.)
- Multipath comparison using AS_PATH handle equality

**Behavior to change:**
- RouteEntry struct layout (remove ASPath field)
- Where AS_PATH handle lives (parallel storage, not inline in entry)
- Optionally: group non-AS_PATH handles into a bundle

## Data Flow (MANDATORY)

### Entry Point
- Wire bytes from BGP UPDATE message, via reactor -> plugin event

### Transformation Path
1. Raw attribute bytes arrive at `ParseAttributes()` in `attrparse.go`
2. Each attribute is interned in its per-type pool via `attrInterners` table
3. `RouteEntry` is populated with handles
4. **NEW:** AS_PATH handle returned separately from entry; stored in parallel BART
5. Entry stored in `FamilyRIB` (BART or map backend)
6. Best-path extraction reads handles -> pool.Get() -> Candidate fields

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Wire -> Storage | `ParseAttributes()` + pool.Intern() | [ ] |
| Storage -> Best-path | `extractCandidate()` reads handles | [ ] |
| Storage -> Wire (export) | handle -> pool.Get() -> wire encoding | [ ] |
| Storage -> Filters | handle -> pool.Get() -> filter evaluation | [ ] |

### Integration Points
- `extractCandidate()` in `rib_commands.go` (lines 961-962) - builds Candidate from RouteEntry
- `FamilyRIB.Insert/Lookup/Delete` - stores/retrieves RouteEntry
- `RouteEntry.Release/AddRef/Clone` - refcount management
- `rib_attr_format.go` (line 36) - formats AS_PATH for display
- `rib_pipeline.go` (lines 401, 624) - pipeline reads AS_PATH
- `forward_handle.go` / `forward_observer.go` - forwarding reads attributes

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing labels pattern)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| BGP UPDATE with attributes | → | ParseAttributes returns entry + AS_PATH handle | `TestParseAttributesSeparateASPath` |
| RIB insert with route | → | FamilyRIB stores entry + AS_PATH in parallel BART | `TestFamilyRIBInsertWithASPath` |
| Best-path on stored routes | → | extractCandidate reads ASPathLen from parallel store | `TestBestPathWithSeparateASPath` |
| Route withdrawal | → | Release decrements both entry handles and AS_PATH handle | `TestReleaseCompactEntry` |
| show bgp rib | → | Formatter retrieves AS_PATH from parallel store | existing `.ci` tests |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | RouteEntry struct without ASPath field | `unsafe.Sizeof(RouteEntry{})` <= 52 bytes (12 handles + stale + padding) |
| AC-2 | ParseAttributes with full UPDATE | Returns (entry, aspathHandle) pair |
| AC-3 | FamilyRIB.Insert | Stores AS_PATH handle in parallel BART (CIDR) or parallel map (non-CIDR) |
| AC-4 | FamilyRIB.Lookup | Returns both entry and AS_PATH handle |
| AC-5 | extractCandidate | Reads ASPathLen/FirstAS from AS_PATH pool via parallel handle |
| AC-6 | RouteEntry.Release + parallel AS_PATH release | Both decremented on every delete path |
| AC-7 | Multipath comparison | Still compares AS_PATH handles for content equality |
| AC-8 | All existing functional tests pass | No regression in .ci tests |
| AC-9 | Benchmark: RIB scan of 100K entries | Measurable improvement in ns/op vs current layout |

## Design

### Option A: AS_PATH separation only (recommended first step)

Remove `ASPath` from `RouteEntry`. Store it in a parallel data structure
with the same key (prefix + peer for CIDR, wire key for non-CIDR).

```
RouteEntry (49 bytes with padding):
    StaleLevel       uint8
    Origin           Handle   // 4 bytes
    NextHop          Handle
    LocalPref        Handle
    MED              Handle
    AtomicAggregate  Handle
    Aggregator       Handle
    Communities      Handle
    LargeCommunities Handle
    ExtCommunities   Handle
    ClusterList      Handle
    OriginatorID     Handle
    OtherAttrs       Handle   // 12 handles x 4 = 48 + 1 stale = 49

AS_PATH stored in parallel BART/map with same key, same as labels already are.
```

FamilyRIB already has a parallel BART for labels (`labels *store.Store[attrpool.Handle]`).
AS_PATH separation follows the exact same pattern. This is not a new concept in
the codebase.

**Pros:** minimal change, follows existing label pattern, no new abstractions.
**Cons:** 49 bytes still fills most of a cache line, improvement comes from not
fetching AS_PATH data during scans that don't need it.

### Option B: Attribute bundle + AS_PATH separation (larger win, follow-up spec)

Group the 12 non-AS_PATH handles into a `Bundle` struct. Intern bundles in
a dedicated bundle pool. RouteEntry becomes:

```
CompactRoute (12 bytes with padding -> 5 per cache line):
    StaleLevel  uint8
    BundleID    uint32   // -> shared Bundle{Origin, NextHop, ...}
    ASPathID    uint32   // -> AS_PATH pool handle (parallel store)
```

Bundle pool holds unique bundles (expected: ~1.7M for a full table).
97% sharing means most routes point to the same few bundles.

**Not in scope for this spec.** Implement if Option A benchmarks justify it.

### AS_PATH trie (NOT in scope)

The reversed trie for AS_PATH internal compression (95-96% measured) changes
pool internals, not the RIB layout. Separate concern. See
`docs/research/attribute-bundle-dedup.md`.

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRouteEntrySizeCompact` | `storage/routeentry_test.go` | Entry size reduced | |
| `TestParseAttributesSeparateASPath` | `storage/attrparse_test.go` | ParseAttributes returns entry + aspath | |
| `TestFamilyRIBInsertWithASPath` | `storage/familyrib_test.go` | Insert stores AS_PATH in parallel | |
| `TestFamilyRIBLookupWithASPath` | `storage/familyrib_test.go` | Lookup returns entry + aspath | |
| `TestFamilyRIBDeleteReleasesASPath` | `storage/familyrib_test.go` | Delete releases both handles | |
| `TestCandidateExtractionSeparateASPath` | `rib_commands_test.go` | extractCandidate works with parallel AS_PATH | |
| `TestMultipathASPathComparison` | `bestpath_test.go` | Multipath still uses AS_PATH handle equality | |
| `BenchmarkRIBScan` | `storage/familyrib_bench_test.go` | ns/op for full-RIB iteration | |

### Boundary Tests
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Handle slot | 0 - 0x03FFFFFF | 0x03FFFFFF | N/A (uint32) | pool rejects |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| existing RIB tests | `test/plugin/*.ci` | All existing routes still work | |
| existing best-path tests | `test/plugin/*.ci` | Best-path selection unchanged | |
| show bgp rib output | `test/plugin/*.ci` | AS_PATH displayed correctly | |

### Future
- Option B (bundle) spec if benchmarks warrant it
- AS_PATH trie pool if memory profiling shows AS_PATH pool is dominant

## Files to Modify

- `internal/component/bgp/plugins/rib/storage/routeentry.go` - remove ASPath field, update Release/AddRef/Clone
- `internal/component/bgp/plugins/rib/storage/attrparse.go` - return AS_PATH handle separately
- `internal/component/bgp/plugins/rib/storage/familyrib.go` - add parallel AS_PATH store (like labels)
- `internal/component/bgp/plugins/rib/storage/peerrib.go` - thread AS_PATH through insert/lookup
- `internal/component/bgp/plugins/rib/bestpath.go` - Candidate.ASPathHandle sourced from parallel store
- `internal/component/bgp/plugins/rib/rib_commands.go` - extractCandidate reads parallel AS_PATH
- `internal/component/bgp/plugins/rib/rib_attr_format.go` - formatting reads parallel AS_PATH
- `internal/component/bgp/plugins/rib/rib_pipeline.go` - pipeline reads parallel AS_PATH
- `internal/component/bgp/plugins/rib/forward_handle.go` - forwarding reads parallel AS_PATH
- `internal/component/bgp/plugins/rib/rib.go` - event handler threads AS_PATH
- `docs/architecture/plugin/rib-storage-design.md` - document new layout

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | - |
| CLI commands/flags | No | - |
| Editor autocomplete | No | - |
| Functional test for new RPC/API | No | existing tests cover |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | - |
| 12 | Internal architecture changed? | Yes | `docs/architecture/plugin/rib-storage-design.md` |

## Files to Create

- `internal/component/bgp/plugins/rib/storage/familyrib_bench_test.go` - RIB scan benchmark

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, current sizes, benchmark baseline |
| 3. Implement (TDD) | Phases below |
| 4. /ze-review gate | Review Gate section |
| 5. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 6. Critical review | Critical Review Checklist below |
| 7. Fix issues | Fix every issue from critical review |
| 8. Re-verify | Re-run stage 5 |
| 9. Repeat 6-8 | Max 2 review passes |
| 10. Deliverables review | Deliverables Checklist below |
| 11. Security review | Security Review Checklist below |
| 12. Re-verify | Re-run stage 5 |
| 13. Present summary | Executive Summary Report |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Benchmark baseline** -- measure current RIB scan performance
   - Tests: `BenchmarkRIBScan` (before)
   - Files: new bench test file
   - Verify: baseline numbers recorded

2. **Phase: Remove ASPath from RouteEntry** -- separate the field
   - Tests: `TestRouteEntrySizeCompact`, `TestParseAttributesSeparateASPath`
   - Files: `routeentry.go`, `attrparse.go`
   - Verify: compilation fails at all 25 ASPath access sites (expected, fix in phase 3-4)

3. **Phase: Parallel AS_PATH store in FamilyRIB** -- follow labels pattern
   - Tests: `TestFamilyRIBInsertWithASPath`, `TestFamilyRIBLookupWithASPath`, `TestFamilyRIBDeleteReleasesASPath`
   - Files: `familyrib.go`, `peerrib.go`
   - Verify: insert/lookup/delete work with parallel AS_PATH

4. **Phase: Wire up consumers** -- fix all 8 files that reference ASPath
   - Tests: `TestCandidateExtractionSeparateASPath`, `TestMultipathASPathComparison`
   - Files: `rib_commands.go`, `bestpath.go`, `rib_attr_format.go`, `rib_pipeline.go`, `forward_handle.go`, `rib.go`
   - Verify: all unit tests pass

5. **Phase: Benchmark after** -- measure improvement
   - Tests: `BenchmarkRIBScan` (after)
   - Files: bench test file
   - Verify: improvement measured and recorded

6. **Functional tests** -- run all existing `.ci` tests, verify no regression
7. **Full verification** -- `make ze-verify`
8. **Complete spec** -- audit tables, learned summary, delete spec

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | AS_PATH handle refcount correct in all paths (insert, delete, clone, GR stale) |
| Correctness | Parallel store keyed identically to main store (prefix for CIDR, wire key for non-CIDR) |
| Data flow | extractCandidate reads AS_PATH from parallel store, not entry |
| Data flow | Release/Clone/AddRef cover both entry and parallel AS_PATH |
| Rule: no-layering | No duplicate AS_PATH storage (entry field fully removed) |
| Pattern | Parallel AS_PATH follows same pattern as existing parallel labels store |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| RouteEntry without ASPath field | `grep -n 'ASPath' storage/routeentry.go` shows only Has/accessor, no field |
| Parallel AS_PATH store in FamilyRIB | `grep -n 'aspaths' storage/familyrib.go` |
| Benchmark showing change | bench output in spec |
| All .ci tests pass | `make ze-functional-test` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Refcount leaks | AS_PATH handle in parallel store must be released on every code path |
| Use after release | Parallel store must not return stale handles after delete |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Benchmark shows no improvement | Record numbers, consider Option B, ask user |
| Functional test fails | Check if AS_PATH access was missed in a consumer |
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
- FamilyRIB already has a parallel BART for labels; AS_PATH separation follows the same pattern
- Best-path never reads AS_PATH from RouteEntry (pre-extracted into Candidate)
- 97% bundle sharing makes Option B attractive but Option A is the right first step
- LINX collector shows best trie compression (96.42%) due to IXP topology
- 25 ASPath references across 8 files: contained change surface

## RFC Documentation

No RFC constraints affected. Best-path selection per RFC 4271 Section 9.1.2
is unchanged (operates on extracted Candidate values, not pool handles).

## Implementation Summary

### What Was Implemented
- [to be filled]

### Bugs Found/Fixed
- [to be filled]

### Documentation Updates
- [to be filled]

### Deviations from Plan
- [to be filled]

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

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied
- [to be filled]

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

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-9 all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (Option A only, B deferred)
- [ ] No speculative features
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL
- [ ] Tests PASS
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-rib-cache-layout.md`
- [ ] Summary included in commit
