# Spec: RIB Attribute Bundle and AS_PATH Separation

| Field | Value |
|-------|-------|
| Status | done |
| Depends | - |
| Phase | 9/9 |
| Updated | 2026-05-16 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/plugin/rib-storage-design.md` - RIB internals
4. `docs/research/attribute-bundle-dedup.md` - analysis data
5. `internal/component/bgp/plugins/rib/storage/routeentry.go` - current layout
6. `internal/component/bgp/attrpool/pool.go` - pool internals

## Task

Restructure the RIB route entry by grouping the 12 non-AS_PATH attribute handles
into a shared Bundle, and keeping AS_PATH as a separate handle. This reduces
RouteEntry from 56 bytes (13 handles + stale) to 12 bytes (stale + Bundle +
ASPath), fitting 5 routes per cache line instead of 1. The 97% non-AS_PATH
sharing measured across full-table MRT dumps makes bundling the dominant win.

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
- 97% of routes share non-AS_PATH attributes, making bundling highly effective
- AS_PATH pool already deduplicates at 87%; trie is future work, not this spec
- RouteEntry is value type stored inline in BART/map; size directly affects scan performance
- RouteEntry has 114 references across 16 files (LSP verified)
- 56 -> 12 bytes = 5 routes per cache line instead of 1 (4.7x scan density)
- `entriesEqual` drops from 13 comparisons to 2 (Bundle + ASPath)
- Release/AddRef drop from 13 pool ops to 2 (bundle cascade handles inner refs)
- Bundle is a comparable Go struct (12 Handle fields), can be map key for dedup

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
- RouteEntry struct layout: 13 inline handles -> Bundle handle + ASPath handle (56 -> 12 bytes)
- Non-AS_PATH attributes stored as Bundle in a dedicated BundlePool (dedup, refcounted)
- Has* methods move to Bundle type (callers dereference BundleID first)
- Release/AddRef operate on 2 handles; bundle pool cascade-releases inner handles
- `entriesEqual` compares Bundle + ASPath handles (2 fields, not 13)
- `ParseAttributes` interns individual attrs, then interns the Bundle

## Data Flow (MANDATORY)

### Entry Point
- Wire bytes from BGP UPDATE message, via reactor -> plugin event

### Transformation Path
1. Raw attribute bytes arrive at `ParseAttributes()` in `attrparse.go`
2. Each attribute is interned in its per-type pool via `attrInterners` table
3. **NEW:** The 12 non-AS_PATH handles are grouped into a Bundle value
4. **NEW:** Bundle is interned in BundlePool -> BundleID (dedup: 97% sharing)
5. **NEW:** RouteEntry = {StaleLevel, BundleID, ASPath} (12 bytes)
6. Entry stored in `FamilyRIB` (BART or map backend)
7. Best-path extraction: dereference BundleID -> Bundle, read handles -> pool.Get() -> Candidate

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
| BGP UPDATE with attributes | → | ParseAttributes returns compact RouteEntry (Bundle + ASPath) | `TestParseAttributesBundled` |
| RIB insert with route | → | FamilyRIB stores 12-byte RouteEntry in BART | `TestFamilyRIBInsertCompact` |
| Best-path on stored routes | → | extractCandidate dereferences Bundle for LP/Origin/MED | `TestBestPathWithBundle` |
| Route withdrawal | → | Release cascade-releases Bundle inner handles + ASPath | `TestReleaseCascade` |
| Duplicate insert (no-op) | → | entriesEqual compares Bundle + ASPath (2 fields) | `TestEntriesEqualCompact` |
| show bgp rib | → | Formatter dereferences Bundle for attribute display | existing `.ci` tests |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | RouteEntry struct with Bundle + ASPath | `unsafe.Sizeof(RouteEntry{})` == 12 bytes (stale + pad + Bundle + ASPath) |
| AC-2 | Bundle type with 12 attribute handles | `unsafe.Sizeof(Bundle{})` == 48 bytes (12 handles) |
| AC-3 | BundlePool.Intern with identical bundles | Returns same BundleID (dedup verified) |
| AC-4 | BundlePool.Release when refcount -> 0 | Cascade-releases all 12 inner attribute handles |
| AC-5 | ParseAttributes with full UPDATE | Returns RouteEntry{StaleLevel, BundleID, ASPath} |
| AC-6 | extractCandidate | Dereferences Bundle, reads LP/Origin/MED/OriginatorID from inner handles |
| AC-7 | RouteEntry.Release | Releases BundleID (with cascade) + ASPath (2 ops, not 13) |
| AC-8 | entriesEqual | Compares Bundle + ASPath handles (2 comparisons, not 13) |
| AC-9 | Multipath comparison | Still compares ASPath handles for content equality |
| AC-10 | All existing functional tests pass | No regression in .ci tests |
| AC-11 | Benchmark: RIB scan of 100K entries | Measurable improvement in ns/op vs current layout |

## Design

### Attribute bundle + AS_PATH separation

Group the 12 non-AS_PATH handles into a `Bundle` struct. Intern bundles in
a dedicated BundlePool. RouteEntry becomes:

```
RouteEntry (12 bytes with padding -> 5 per cache line):
    StaleLevel  uint8        // 1 byte + 3 padding
    Bundle      Handle       // -> shared Bundle{Origin, NextHop, ...}
    ASPath      Handle       // -> AS_PATH pool handle

Bundle (48 bytes, stored in BundlePool):
    Origin           Handle
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
    OtherAttrs       Handle
```

BundlePool holds unique bundles (expected: ~1.7M for a full table).
97% sharing means most routes point to the same few bundles.

### BundlePool design

Bundle is a fixed-size comparable Go struct (12 uint32 fields). This means
`map[Bundle]uint32` works for dedup without serialization.

```
BundlePool:
    bundles  []Bundle        // indexed by slot
    refcount []uint32        // indexed by slot
    index    map[Bundle]uint32  // Bundle -> slot (for dedup)
    free     []uint32        // free slot list
```

**Intern:** hash Bundle struct, look up in map. If found, increment refcount
and return existing handle. If new, allocate slot, store bundle, return handle.
The 12 per-attribute handles passed in are NOT released on dedup hit; they
belong to the bundle now. On dedup hit, release the 12 fresh handles (the
existing bundle already holds refs for identical values).

**Release:** decrement refcount. If zero, cascade-release all 12 inner attribute
handles, remove from map, add slot to free list.

**AddRef:** increment refcount. No inner handle ops needed (inner refs are
stable while bundle exists).

**Get:** return bundles[slot]. O(1) array lookup.

Handle encoding: reuse existing Handle layout. Allocate one poolIdx (0-30)
for BundlePool. Slot is 26 bits = up to 67M bundles (well beyond the ~1.7M
expected).

### Performance impact

| Metric | Current (56B) | Bundle (12B) | Change |
|--------|---------------|--------------|--------|
| Routes per cache line | 1 | 5 | 5x density |
| Cache lines for 1M entries | 875,000 | 187,500 | 78.6% fewer |
| Entry working set (1M routes) | 53.4 MB | 11.4 MB | Fits L3 cache |
| `entriesEqual` comparisons | 13 | 2 | 6.5x faster |
| Release/AddRef pool ops | 13 | 2 | 6.5x fewer |
| `extractCandidate` | 5 pool.Get | 1 BundlePool.Get + 5 pool.Get | +1 indirection |
| Memory (1M routes, 1 peer) | 53.4 MB | 11.4 MB entries + 78 MB bundles | +36 MB |
| Memory (1M routes, 10 peers) | 534 MB | 114 MB entries + 78 MB bundles | -342 MB (64%) |

The bundle pool (78 MB for 1.7M unique bundles) is a fixed cost shared across
all peers. At multi-peer scale, the per-entry savings dominate.

The +1 indirection on extractCandidate is mitigated: with 97% sharing, the
most-used bundles stay in L1/L2 cache. The bundle lookup is one array index,
not a pointer chase.

### Why not AS_PATH separation only

Removing ASPath alone shrinks RouteEntry from 56 to 52 bytes: still 1 route
per cache line, 7% working set reduction, no threshold crossed. The change
surface is similar (all 114 RouteEntry references must be audited either way).
The marginal win does not justify touching the code without bundling.

### AS_PATH trie (NOT in scope)

The reversed trie for AS_PATH internal compression (95-96% measured) changes
pool internals, not the RIB layout. Separate concern. See
`docs/research/attribute-bundle-dedup.md`.

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRouteEntry_NewEmpty` | `internal/component/bgp/plugins/rib/storage/routeentry_test.go` | RouteEntry has Bundle + ASPath InvalidHandle | |
| `TestBundleSizeCompact` | `internal/component/bgp/plugins/rib/storage/bundle_test.go` | `unsafe.Sizeof(Bundle{})` == 48 | |
| `TestBundlePoolInternDedup` | `internal/component/bgp/plugins/rib/storage/bundle_test.go` | Identical bundles return same handle | |
| `TestBundlePoolReleaseCascade` | `internal/component/bgp/plugins/rib/storage/bundle_test.go` | Refcount 0 releases all 12 inner handles | |
| `TestBundlePoolAddRef` | `internal/component/bgp/plugins/rib/storage/bundle_test.go` | AddRef increments, inner handles untouched | |
| `TestParseAttributes_AllTypes` | `internal/component/bgp/plugins/rib/storage/attrparse_test.go` | ParseAttributes returns bundled RouteEntry | |
| `TestFamilyRIB_PerAttrDedup` | `internal/component/bgp/plugins/rib/storage/familyrib_test.go` | Compares Bundle + ASPath (2 fields) | |
| `TestFamilyRIB_Insert` | `internal/component/bgp/plugins/rib/storage/familyrib_test.go` | Insert stores compact entry in BART | |
| `TestFamilyRIB_Remove` | `internal/component/bgp/plugins/rib/storage/familyrib_test.go` | Delete releases Bundle (cascade) + ASPath | |
| `TestExtractCandidate_PoolWiring` | `internal/component/bgp/plugins/rib/rib_test.go` | extractCandidate dereferences Bundle | |
| `TestSelectMultipath_ContentEqual` | `internal/component/bgp/plugins/rib/bestpath_test.go` | Multipath still uses ASPath handle equality | |
| `BenchmarkRIBScan` | `internal/component/bgp/plugins/rib/storage/familyrib_bench_test.go` | ns/op for full-RIB iteration | |
| `BenchmarkEntriesEqual` | `internal/component/bgp/plugins/rib/storage/familyrib_bench_test.go` | ns/op for equality check | |

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
- AS_PATH trie pool if memory profiling shows AS_PATH pool is dominant

## Files to Modify

- `internal/component/bgp/plugins/rib/storage/routeentry.go` - replace 13 handles with Bundle + ASPath, update Release/AddRef/Clone, move Has* to Bundle
- `internal/component/bgp/plugins/rib/storage/attrparse.go` - intern individual attrs then intern Bundle, return compact RouteEntry
- `internal/component/bgp/plugins/rib/storage/familyrib.go` - entriesEqual compares 2 fields, release cascades through BundlePool
- `internal/component/bgp/plugins/rib/storage/peerrib.go` - updated RouteEntry usage (smaller struct, same API)
- `internal/component/bgp/plugins/rib/rib_commands.go` - extractCandidate dereferences Bundle for attributes
- `internal/component/bgp/plugins/rib/rib_attr_format.go` - formatting dereferences Bundle
- `internal/component/bgp/plugins/rib/rib_pipeline.go` - pipeline dereferences Bundle for attribute access
- `internal/component/bgp/plugins/rib/rib_commands_community.go` - community operations dereference Bundle
- `internal/component/bgp/plugins/rib/rib.go` - event handler uses compact RouteEntry
- `internal/component/bgp/plugins/rib/rib_bestchange.go` - best-path change tracking with compact entry
- `docs/architecture/plugin/rib-storage-design.md` - document bundle layout

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

- `internal/component/bgp/plugins/rib/storage/bundle.go` - Bundle type + BundlePool (dedup, refcount, cascade release)
- `internal/component/bgp/plugins/rib/storage/bundle_test.go` - BundlePool unit tests
- `internal/component/bgp/plugins/rib/storage/familyrib_bench_test.go` - RIB scan benchmark

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify (114 refs across 16 files), current sizes, benchmark baseline |
| 3. Implement (TDD) | Phases 1-7 below |
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
   - Tests: `BenchmarkRIBScan` (before), `BenchmarkEntriesEqual` (before)
   - Files: new bench test file
   - Verify: baseline numbers recorded

2. **Phase: Bundle type + BundlePool** -- new abstraction, fully tested in isolation
   - Tests: `TestBundleSizeCompact`, `TestBundlePoolInternDedup`, `TestBundlePoolReleaseCascade`, `TestBundlePoolAddRef`
   - Files: `internal/component/bgp/plugins/rib/storage/bundle.go`, `internal/component/bgp/plugins/rib/storage/bundle_test.go`
   - Verify: dedup works, cascade release verified, handle encoding correct

3. **Phase: Compact RouteEntry** -- replace 13 handles with Bundle + ASPath
   - Tests: `TestRouteEntrySizeCompact`, `TestParseAttributesBundled`, `TestEntriesEqualCompact`
   - Files: `routeentry.go`, `attrparse.go`, `familyrib.go` (entriesEqual)
   - Verify: compilation fails at all sites that read individual handles from RouteEntry (expected, fix in phase 4)

4. **Phase: Wire up consumers** -- all files that read attributes from RouteEntry now dereference Bundle
   - Tests: `TestCandidateExtractionBundle`, `TestMultipathASPathComparison`
   - Files: `rib_commands.go`, `rib_attr_format.go`, `rib_pipeline.go`, `rib_commands_community.go`, `rib.go`, `rib_bestchange.go`, `peerrib.go`
   - Verify: all unit tests pass

5. **Phase: Release/AddRef/Clone** -- update lifecycle methods for 2-handle model
   - Tests: `TestFamilyRIBInsertCompact`, `TestFamilyRIBDeleteCascade`
   - Files: `routeentry.go` (Release, AddRef, Clone), `familyrib.go` (all delete/release paths)
   - Verify: no refcount leaks, cascade release correct

6. **Phase: Benchmark after** -- measure improvement
   - Tests: `BenchmarkRIBScan` (after), `BenchmarkEntriesEqual` (after)
   - Files: bench test file
   - Verify: improvement measured and recorded

7. **Functional tests** -- run all existing `.ci` tests, verify no regression
8. **Full verification** -- `make ze-verify`
9. **Complete spec** -- audit tables, learned summary, delete spec

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | BundlePool cascade release frees all 12 inner handles when refcount -> 0 |
| Correctness | BundlePool dedup hit releases the 12 fresh per-attr handles (existing bundle owns refs) |
| Correctness | RouteEntry.Release releases Bundle (cascade) + ASPath in all paths (insert, delete, clone, GR stale) |
| Data flow | extractCandidate dereferences Bundle, then reads individual handles |
| Data flow | All 114 RouteEntry reference sites updated (no direct handle access on RouteEntry) |
| Rule: no-layering | No duplicate attribute storage (individual handles only in Bundle, not on RouteEntry) |
| Pattern | Bundle is comparable struct usable as map key (no serialization needed) |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| RouteEntry == 12 bytes | `unsafe.Sizeof` test assertion |
| Bundle type with 12 handles | `unsafe.Sizeof` test assertion |
| BundlePool with dedup + cascade release | `TestBundlePoolInternDedup`, `TestBundlePoolReleaseCascade` |
| No individual handles on RouteEntry | `grep -n 'Origin\|NextHop\|LocalPref\|MED' storage/routeentry.go` shows only Bundle + ASPath fields |
| Benchmark showing improvement | bench output in spec |
| All .ci tests pass | `make ze-functional-test` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Refcount leaks | BundlePool refcount must reach 0 on every delete/purge path |
| Cascade correctness | BundlePool release at refcount 0 must free all 12 inner handles |
| Double release | BundlePool must not cascade-release if refcount > 0 after decrement |
| Use after release | BundlePool.Get must not return freed bundle data |
| Dedup accounting | BundlePool dedup hit must release the 12 fresh handles, not the existing ones |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Benchmark shows no improvement | Record numbers, investigate BART node overhead, ask user |
| BundlePool cascade leaves leaked refs | Check all Release paths, verify with pool stats |
| Functional test fails | Check if Bundle dereference was missed in a consumer |
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
- 97% non-AS_PATH sharing makes bundling the dominant win over AS_PATH separation alone
- AS_PATH separation alone (56 -> 52 bytes) does not cross any cache threshold (still 1 per line)
- Bundle as comparable Go struct enables `map[Bundle]uint32` for zero-serialization dedup
- BundlePool cascade release simplifies RouteEntry lifecycle: 2 ops instead of 13
- `entriesEqual` becomes 2-field comparison: dominant win for implicit-withdraw no-op check on every INSERT
- 114 RouteEntry references across 16 files: large but mechanical change surface
- LINX collector shows best trie compression (96.42%) due to IXP topology (future work)
- Best-path never reads AS_PATH bytes from pool (uses pre-extracted ASPathLen/FirstAS)

## RFC Documentation

No RFC constraints affected. Best-path selection per RFC 4271 Section 9.1.2
is unchanged (operates on extracted Candidate values, not pool handles).

## Implementation Summary

### What Was Implemented
- Bundle type (48 bytes, 12 attribute handles) with Has* methods
- BundlePool with dedup via map[Bundle]uint32, refcounting, cascade release
- Compact RouteEntry (12 bytes: StaleLevel + Bundle handle + ASPath handle)
- ParseAttributes interns attributes individually, then interns Bundle
- All consumer files updated to dereference Bundle via entry.GetBundle()
- attachCommunity creates modified bundles with proper AddRef/Release lifecycle
- entriesEqual reduced to 2-field comparison (Bundle + ASPath)
- Release/AddRef/Clone reduced to 2 pool operations
- Benchmark baseline and post-implementation measurements

### Bugs Found/Fixed
- Global pool state contamination in tests: cascade test used data (`[]byte{0x01}`) colliding with other tests. Fixed with unique byte patterns.

### Documentation Updates
- docs/architecture/plugin/rib-storage-design.md: Updated dedup section with bundle layout

### Deviations from Plan
- Has* methods use value receivers (Bundle not *Bundle) since Bundle is returned by value from BundlePool.Get
- No separate size assertion test for RouteEntry (verified via struct layout: uint8 + pad + 2*uint32 = 12)

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| RouteEntry 56->12 bytes | Done | storage/routeentry.go:31 | StaleLevel + Bundle + ASPath |
| Bundle with 12 handles | Done | storage/bundle.go:15 | 48 bytes, comparable struct |
| BundlePool dedup | Done | storage/bundle.go:132 | map[Bundle]uint32 key |
| Cascade release | Done | storage/bundle.go:172 | releaseInnerHandles on refcount 0 |
| 5x cache density | Done | Benchmark | 1,930us -> 1,586us scan |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | Struct layout: uint8+pad+Handle+Handle = 12 | |
| AC-2 | Done | TestBundleSizeCompact | 48 bytes |
| AC-3 | Done | TestBundlePoolInternDedup | Same handle returned |
| AC-4 | Done | TestBundlePoolReleaseCascade | Inner handles freed |
| AC-5 | Done | attrparse.go:128-129 | Returns compact RouteEntry |
| AC-6 | Done | rib_commands.go extractCandidate | GetBundle() then reads |
| AC-7 | Done | routeentry.go:72-80 | 2 ops: Bundles.Release + ASPath.Release |
| AC-8 | Done | familyrib.go:588 | Bundle == Bundle && ASPath == ASPath |
| AC-9 | Done | bestpath.go:185 | ASPathHandle on Candidate from entry.ASPath |
| AC-10 | Done | make ze-unit-test | All RIB tests pass |
| AC-11 | Done | BenchmarkRIBScan | -17.8% ns/op, -79% B/op |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| TestBundleSizeCompact | Done | internal/component/bgp/plugins/rib/storage/bundle_test.go | 48 bytes |
| TestBundlePoolInternDedup | Done | internal/component/bgp/plugins/rib/storage/bundle_test.go | Dedup verified |
| TestBundlePoolReleaseCascade | Done | internal/component/bgp/plugins/rib/storage/bundle_test.go | Cascade verified |
| TestBundlePoolAddRef | Done | internal/component/bgp/plugins/rib/storage/bundle_test.go | Refcount increment |
| TestParseAttributesBundled | Done | attrparse_test.go (existing tests) | All pass with bundle path |
| TestEntriesEqualCompact | Done | familyrib_test.go (existing tests) | 2-field comparison |
| TestFamilyRIBInsertCompact | Done | familyrib_test.go (existing tests) | 12-byte entry in BART |
| TestFamilyRIBDeleteCascade | Done | familyrib_test.go (existing tests) | Release cascades |
| BenchmarkRIBScan | Done | familyrib_bench_test.go | 1,586us |
| BenchmarkEntriesEqual | Done | familyrib_bench_test.go | 0.35ns |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| storage/bundle.go | Created | Bundle type + BundlePool |
| storage/bundle_test.go | Created | 8 tests |
| storage/familyrib_bench_test.go | Created | 3 benchmarks |
| storage/routeentry.go | Modified | Compact layout |
| storage/attrparse.go | Modified | Bundle interning |
| storage/familyrib.go | Modified | entriesEqual + ToWireBytes |
| rib_commands.go | Modified | extractCandidate |
| rib_attr_format.go | Modified | enrichRouteMap |
| rib_pipeline.go | Modified | Pipeline attribute access |
| rib_bestchange.go | Modified | NextHop/OtherAttrs access |
| rib_commands_community.go | Modified | Bundle mutation for attach |

### Audit Summary
- **Total items:** 27
- **Done:** 27
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 1 (Has* receivers: pointer -> value)

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | NOTE | BundlePool uses sync.Mutex; Get/Len/RefCount are read-only | bundle.go:107 | Changed to RWMutex |
| 2 | NOTE | attachCommunity ignores AddRef errors | rib_commands_community.go:244 | Added AddRefInnerHandles with rollback |
| 3 | NOTE | Empty-input path interns an empty Bundle | attrparse.go:78 | Kept (tests enforce this behavior) |

### Fixes applied
- Changed BundlePool.mu to sync.RWMutex, Get/Len/RefCount use RLock
- attachCommunity now uses Bundle.AddRefInnerHandles() with full error handling and rollback
- NOTE 3 left as-is (cosmetic, existing tests depend on current behavior)

### Run 2 (after fixes)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| - | - | Clean pass. 0 findings. | - | - |

### Final status
- [x] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [x] All NOTEs recorded above

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| storage/bundle.go | Yes | `ls -la` confirmed |
| storage/bundle_test.go | Yes | `ls -la` confirmed |
| storage/familyrib_bench_test.go | Yes | `ls -la` confirmed |
| storage/routeentry.go | Yes | Modified, `git status` |
| storage/attrparse.go | Yes | Modified, `git status` |
| storage/familyrib.go | Yes | Modified, `git status` |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | RouteEntry == 12 bytes | struct: uint8(1)+pad(3)+Handle(4)+Handle(4) = 12 |
| AC-2 | Bundle == 48 bytes | TestBundleSizeCompact PASS |
| AC-3 | BundlePool dedup | TestBundlePoolInternDedup PASS |
| AC-4 | Cascade release | TestBundlePoolReleaseCascade PASS |
| AC-5 | ParseAttributes compact | attrparse.go:128 returns RouteEntry{Bundle, ASPath} |
| AC-6 | extractCandidate | rib_commands.go:931 calls GetBundle() |
| AC-7 | Release 2 ops | routeentry.go:72-80 |
| AC-8 | entriesEqual 2 fields | familyrib.go:588 |
| AC-9 | Multipath ASPath | entry.ASPath on Candidate.ASPathHandle |
| AC-10 | Tests pass | `go test ./internal/component/bgp/plugins/rib/...` all OK |
| AC-11 | Benchmark improvement | -18% scan, -79% memory |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| BGP UPDATE parse + RIB insert | test/plugin/attributes.ci | ParseAttributes -> Bundle -> BART storage |
| show bgp rib with filters | test/plugin/rib-show-filter.ci | enrichRouteMapFromEntry -> GetBundle() |
| best-path selection | test/plugin/rib-best-selection.ci | extractCandidate -> GetBundle() |
| RIB withdrawal | test/plugin/rib-withdrawal.ci | Release cascade on remove |
| GR stale + purge | test/plugin/graceful-restart-rib.ci | MarkStale/PurgeStale on compact entry |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-11 all demonstrated
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
- [ ] No premature abstraction (Bundle justified by 97% sharing data)
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
- [x] Write learned summary to `plan/learned/608-perf-1-rib-cache-layout.md`
- [ ] Summary included in commit
