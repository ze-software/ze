# Spec: rib-alloc

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | - |
| Updated | 2026-04-06 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/plugin/rib-storage-design.md` - RIB pool architecture
4. `docs/architecture/pool-architecture.md` - pool system design
5. `internal/component/bgp/plugins/rib/storage/familyrib.go` - FamilyRIB (map + key)
6. `internal/component/bgp/plugins/rib/storage/routeentry.go` - RouteEntry struct
7. `internal/component/bgp/plugins/rib/storage/attrparse.go` - ParseAttributes
8. `internal/component/bgp/plugins/rib/rib_nlri.go` - splitNLRIs

## Task

Eliminate per-route heap allocations in the RIB storage layer. Four targeted optimizations:

1. **Value-type RouteEntry** -- store `RouteEntry` by value in map instead of pointer (eliminates 1 heap alloc per route)
2. **Fixed-size NLRI key** -- replace `string(nlriBytes)` map key with `NLRIKey` struct (`len uint8` + `[24]byte` data) (eliminates 1 string alloc per route on every Insert/Remove/Lookup). Only simple prefix families are stored in the RIB (`isSimplePrefixFamily` guard), so 24 bytes covers all cases (max: ADD-PATH IPv6 /128 = 21 bytes).
3. **Pre-count splitNLRIs** -- two-pass split: count first, then `make([][]byte, 0, count)` (eliminates slice growth allocs)
4. **Pre-size attrpool initial capacity** -- larger initial buffer and slot capacity for pools (reduces append-triggered regrowth)

Combined: eliminates ~2M allocations per 1M routes received.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/plugin/rib-storage-design.md` - RIB pool storage design
  -> Constraint: RIB stores NLRI -> attr handles, not WireUpdate. Pool dedup is per-attribute-type.
- [ ] `docs/architecture/pool-architecture.md` - pool system design
  -> Constraint: attrpool.Handle is uint32 (value type). Pool.Intern() returns handle, not pointer.
- [ ] `.claude/rules/design-principles.md` - lazy over eager, zero-copy
  -> Decision: value types preferred over pointer indirection. No identity wrappers.

**Key insights:**
- RouteEntry is 56 bytes with Go alignment (1 byte StaleLevel + 3 padding + 13 x 4 byte handles). No pointers, slices, or heap references. Ideal for value storage.
- Go maps (gc compiler) store values inline for types under 128 bytes. RouteEntry at 56 bytes qualifies. This is gc-specific, not a language guarantee.
- The RIB only stores simple prefix families (IPv4/IPv6 unicast/multicast) -- non-unicast is skipped by `isSimplePrefixFamily()`. Max NLRI for these: IPv6 /128 with ADD-PATH = 4 (path-id) + 1 (prefix-len) + 16 (addr) = 21 bytes. A `NLRIKey` struct with `len uint8` + `[24]byte` data covers all stored families.
- splitNLRIs uses open-ended `append` which triggers slice growth. Counting prefixes first is O(n) same as parsing them.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/rib/storage/familyrib.go` - FamilyRIB with `map[string]*RouteEntry`
  -> Constraint: routes map uses string keys from `string(nlriBytes)`. RouteEntry stored as pointer.
- [ ] `internal/component/bgp/plugins/rib/storage/routeentry.go` - RouteEntry struct (13 handles + StaleLevel)
  -> Constraint: NewRouteEntry() returns *RouteEntry. Release() mutates in place. Clone() returns *RouteEntry.
- [ ] `internal/component/bgp/plugins/rib/storage/attrparse.go` - ParseAttributes returns *RouteEntry
  -> Constraint: attrInterners table uses func(*RouteEntry) closures for get/set.
- [ ] `internal/component/bgp/plugins/rib/storage/peerrib.go` - PeerRIB wraps FamilyRIB per peer
  -> Constraint: Lookup returns (*RouteEntry, bool). Iterate callbacks take *RouteEntry.
- [ ] `internal/component/bgp/plugins/rib/rib_nlri.go` - splitNLRIs returns [][]byte
  -> Constraint: called from rib.go and rib_structured.go (6 call sites).
- [ ] `internal/component/bgp/plugins/rib/rib_pipeline.go` - PipelineRecord.InEntry is *RouteEntry
- [ ] `internal/component/bgp/plugins/rib/rib_bestchange.go` - extractMPNextHop takes *RouteEntry
- [ ] `internal/component/bgp/plugins/rib/rib_commands.go` - multiple funcs take *RouteEntry
- [ ] `internal/component/bgp/plugins/rib/storage/familyrib_test.go` - tests for FamilyRIB
- [ ] `internal/component/bgp/plugins/rib/storage/peerrib_test.go` - tests for PeerRIB
- [ ] `internal/component/bgp/attrpool/pool.go` - Pool with initial capacity of 64

**Behavior to preserve:**
- All existing FamilyRIB/PeerRIB API semantics (Insert, Remove, Lookup, Iterate, MarkStale, PurgeStale)
- Attribute dedup via attrpool (per-attribute-type Intern/Release lifecycle)
- RouteEntry.Release() frees all pool handles
- entriesEqual() for no-op detection
- splitNLRIs returns [][]byte slices into the original data (zero-copy sub-slices)
- Thread safety: PeerRIB mutex protects FamilyRIB access

**Behavior to change:**
- `map[string]*RouteEntry` becomes `map[NLRIKey]RouteEntry` (value type, struct key with len + [24]byte data)
- `*RouteEntry` return types become `RouteEntry` (or `*RouteEntry` via map-get-modify-put where mutation needed)
- `ParseAttributes` returns `RouteEntry` (value) instead of `*RouteEntry`
- splitNLRIs pre-counts before allocating result slice
- Pool initial capacity increased from 64 to larger defaults

## Data Flow (MANDATORY)

### Entry Point
- Wire bytes arrive as `WireUpdate` in reactor. RIB plugin receives UPDATE event with raw attribute + NLRI bytes.

### Transformation Path
1. `rib.go:handleUpdate()` receives raw attr bytes + NLRI bytes from UPDATE event
2. `splitNLRIs(data, addPath)` splits concatenated NLRI wire bytes into individual NLRIs (**optimization 3: pre-count**)
3. For each NLRI: `FamilyRIB.Insert(attrBytes, nlriBytes)` called
4. Inside Insert: `nlriKey := string(nlriBytes)` creates map key (**optimization 2: fixed-size key**)
5. Inside Insert: `ParseAttributes(attrBytes)` creates RouteEntry with pool handles (**optimization 1: value type**)
6. Entry stored in `routes` map (**optimization 1: value storage**)
7. attrpool.Intern() on miss path appends to buffer (**optimization 4: pre-size**)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Wire -> RIB | NLRI bytes + attr bytes from WireUpdate | [ ] |
| RIB -> attrpool | Intern(value) returns Handle | [ ] |
| PeerRIB -> FamilyRIB | Direct call under mutex | [ ] |

### Integration Points
- `PipelineRecord.InEntry *storage.RouteEntry` -- pipeline holds pointer to RIB entry for show commands
- `extractMPNextHop(*storage.RouteEntry)` -- best-change reads OtherAttrs
- `rib_commands.go` -- multiple functions read RouteEntry fields for JSON output
- `rib_pipeline_best.go` -- best-path selection reads RouteEntry fields

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (splitNLRIs still returns sub-slices)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| UPDATE event with routes | -> | FamilyRIB.Insert with NLRIKey | `TestFamilyRIB_InsertLookupRemove` (updated for value type) |
| UPDATE event with routes | -> | splitNLRIs pre-count | `TestSplitNLRIs_PreCount` |
| RIB plugin receive | -> | Full insert/withdraw pipeline | `test/plugin/plugin-rib-features.ci` |
| RIB plugin show | -> | Pipeline reads value-type entries | `test/plugin/api-rib-show-in.ci` |
| RIB plugin stale/purge | -> | GR stale marking on value entries | `test/plugin/graceful-restart-rib.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Insert 1M routes into FamilyRIB | Zero per-route heap allocations for RouteEntry (value stored in map) |
| AC-2 | Insert route with IPv4 /24 NLRI | NLRIKey.Bytes() returns exact original bytes, lookup matches |
| AC-3 | Insert route with IPv6 /48 NLRI | NLRIKey.Bytes() returns exact original bytes, lookup matches |
| AC-4 | Insert route with ADD-PATH path-id | NLRIKey includes path-id, Bytes() returns exact original, lookup matches |
| AC-5 | Insert duplicate NLRI (implicit withdraw) | Old entry's Release() called, new entry stored, no leak |
| AC-6 | Insert identical attributes (no-op) | entriesEqual returns true, new entry released, old preserved |
| AC-7 | splitNLRIs on 100 concatenated NLRIs | Result slice has exact capacity (no over-allocation) |
| AC-8 | splitNLRIs returns sub-slices of input | Zero-copy preserved (sub-slices point into original data) |
| AC-9 | ParseAttributes returns value type | No heap allocation for RouteEntry itself |
| AC-10 | Pool initial capacity accommodates 10K routes without regrowth | Buffer and slot pre-sizing verified via Metrics |
| AC-11 | MarkStale/PurgeStale work with value-type entries | Stale routes correctly marked and purged |
| AC-12 | LookupEntry returns usable entry for read-only callers | Pipeline, commands, best-path can read entry fields |
| AC-13 | All existing RIB tests pass unchanged (or with minimal signature updates) | No regression |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestNLRIKey_IPv4` | `storage/nlrikey_test.go` | AC-2: IPv4 prefix round-trips through NLRIKey, Bytes() matches input |  |
| `TestNLRIKey_IPv6` | `storage/nlrikey_test.go` | AC-3: IPv6 prefix round-trips through NLRIKey |  |
| `TestNLRIKey_AddPath` | `storage/nlrikey_test.go` | AC-4: ADD-PATH prefix with path-id round-trips |  |
| `TestNLRIKey_MaxLength` | `storage/nlrikey_test.go` | AC-4: max NLRI length (21 bytes) fits, Bytes() exact |  |
| `TestNLRIKey_Equality` | `storage/nlrikey_test.go` | AC-2: same input produces equal keys, different input produces unequal |  |
| `TestFamilyRIB_ValueType` | `storage/familyrib_test.go` | AC-5, AC-6: implicit withdraw + no-op with value entries |  |
| `TestFamilyRIB_MarkStale` | `storage/familyrib_test.go` | AC-11: stale marking on value entries |  |
| `TestFamilyRIB_PurgeStale` | `storage/familyrib_test.go` | AC-11: purge on value entries |  |
| `TestParseAttributes_Value` | `storage/attrparse_test.go` | AC-9: returns value type, handles correct |  |
| `TestSplitNLRIs_PreCount` | `rib_nlri_test.go` | AC-7: result cap equals count |  |
| `TestSplitNLRIs_ZeroCopy` | `rib_nlri_test.go` | AC-8: sub-slices share backing array |  |
| `TestPoolPreSize` | `attrpool/pool_test.go` | AC-10: no regrowth for first 10K interns |  |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| NLRI length | 0-21 (with ADD-PATH) | 21 bytes | N/A (0 = empty) | 22+ (should never happen for unicast) |
| Prefix len IPv4 | 0-32 | 32 | N/A | 33 (handled by splitNLRIs validation) |
| Prefix len IPv6 | 0-128 | 128 | N/A | 129 (handled by splitNLRIs validation) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| RIB features | `test/plugin/plugin-rib-features.ci` | Insert/withdraw/lookup pipeline | must pass unchanged |
| Adj-RIB-In show | `test/plugin/api-rib-show-in.ci` | Show routes reads value entries | must pass unchanged |
| GR stale | `test/plugin/graceful-restart-rib.ci` | Stale marking/purge on value entries | must pass unchanged |
| LLGR stale | `test/plugin/llgr-rib-stale.ci` | LLGR stale levels on value entries | must pass unchanged |
| RIB withdrawal | `test/plugin/rib-withdrawal.ci` | Withdraw releases value entry | must pass unchanged |
| Best selection | `test/plugin/rib-best-selection.ci` | Best-path reads value entries | must pass unchanged |

### Future (if deferring any tests)
- None

## Files to Modify
- `internal/component/bgp/plugins/rib/storage/familyrib.go` - NLRIKey type, value-type map, API changes
- `internal/component/bgp/plugins/rib/storage/routeentry.go` - NewRouteEntry returns value, adjust Release/Clone
- `internal/component/bgp/plugins/rib/storage/attrparse.go` - ParseAttributes returns value, attrInterners use value
- `internal/component/bgp/plugins/rib/storage/peerrib.go` - Adapt to value-type RouteEntry API
- `internal/component/bgp/plugins/rib/rib_nlri.go` - splitNLRIs pre-count
- `internal/component/bgp/plugins/rib/rib.go` - Adapt to value-type API
- `internal/component/bgp/plugins/rib/rib_structured.go` - Adapt to value-type API
- `internal/component/bgp/plugins/rib/rib_commands.go` - Adapt to value-type API
- `internal/component/bgp/plugins/rib/rib_pipeline.go` - PipelineRecord uses value or pointer-into-map
- `internal/component/bgp/plugins/rib/rib_pipeline_best.go` - Adapt to value-type API
- `internal/component/bgp/plugins/rib/rib_bestchange.go` - Adapt to value-type API
- `internal/component/bgp/plugins/rib/rib_attr_format.go` - Adapt to value-type API
- `internal/component/bgp/plugins/rib/storage/familyrib_test.go` - Update for new types
- `internal/component/bgp/plugins/rib/storage/peerrib_test.go` - Update for new types
- `internal/component/bgp/plugins/rib/storage/routeentry_test.go` - Update for value type
- `internal/component/bgp/plugins/rib/storage/stale_test.go` - Update for value-type RouteEntry stale tests
- `internal/component/bgp/plugins/rib/storage/attrparse_test.go` - Add TestParseAttributes_Value
- `internal/component/bgp/plugins/rib/rib_test.go` - Update for value type
- `internal/component/bgp/attrpool/pool.go` - Increase default initial capacity
- `internal/component/bgp/plugins/rib/pool/attributes.go` - Increase initial capacity for per-attribute pools

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | N/A |
| CLI commands/flags | No | N/A |
| Editor autocomplete | No | N/A |
| Functional test for new RPC/API | No | Existing .ci tests cover |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | - |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | No | - |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented? | No | - |
| 10 | Test infrastructure changed? | No | - |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | Yes | `docs/architecture/plugin/rib-storage-design.md` -- update storage section to describe value-type RouteEntry and NLRIKey |

## Files to Create
- `internal/component/bgp/plugins/rib/storage/nlrikey.go` - NLRIKey type and conversion functions
- `internal/component/bgp/plugins/rib/storage/nlrikey_test.go` - NLRIKey unit tests

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

1. **Phase: NLRIKey type** -- introduce struct key type (`len uint8` + `data [24]byte`) with conversion functions and `Bytes()` method
   - Tests: `TestNLRIKey_IPv4`, `TestNLRIKey_IPv6`, `TestNLRIKey_AddPath`, `TestNLRIKey_MaxLength`
   - Files: `storage/nlrikey.go`, `storage/nlrikey_test.go`
   - Verify: tests fail -> implement -> tests pass

2. **Phase: Value-type RouteEntry** -- change RouteEntry from pointer to value throughout storage package
   - Tests: `TestFamilyRIB_ValueType`, `TestParseAttributes_Value`, `TestFamilyRIB_MarkStale`, `TestFamilyRIB_PurgeStale`
   - Files: `storage/routeentry.go`, `storage/attrparse.go`, `storage/familyrib.go`, `storage/peerrib.go`
   - Key design: `map[NLRIKey]RouteEntry`. Insert does get-modify-put for StaleLevel mutation. LookupEntry returns `(RouteEntry, bool)` -- callers get a copy (read-only). IterateEntry callbacks receive `RouteEntry` value.
   - attrInterners closures change from `func(*RouteEntry)` to direct field access on value
   - Verify: all storage tests pass

3. **Phase: Adapt RIB plugin callers** -- update all code outside storage/ that uses *RouteEntry
   - Tests: existing rib_test.go tests must pass
   - Files: `rib.go`, `rib_structured.go`, `rib_commands.go`, `rib_pipeline.go`, `rib_pipeline_best.go`, `rib_bestchange.go`, `rib_attr_format.go`
   - Key design: PipelineRecord.InEntry changes to `storage.RouteEntry` (value). Read-only functions change to value params: `enrichRouteMapFromEntry(RouteEntry)`, `extractMPNextHop(RouteEntry)`, `extractCandidate(RouteEntry)`, `attachCommunity(RouteEntry)`, `matchInEntry(RouteEntry)`.
   - IterateEntry callbacks: receive `NLRIKey` or use `key.Bytes()` to reconstruct exact-length NLRI bytes for callers that need wire format.
   - Verify: all rib tests pass

4. **Phase: splitNLRIs pre-count** -- two-pass implementation
   - Tests: `TestSplitNLRIs_PreCount`, `TestSplitNLRIs_ZeroCopy`
   - Files: `rib_nlri.go`
   - Verify: tests fail -> implement -> tests pass

5. **Phase: Pool pre-sizing** -- increase initial capacity for attrpool instances
   - Tests: `TestPoolPreSize` (verify Metrics shows no regrowth for first 10K interns)
   - Files: `attrpool/pool.go`, `pool/attributes.go`
   - Verify: tests pass

6. **Functional tests** -- run existing .ci tests to verify no regression
7. **Full verification** -- `make ze-verify`
8. **Complete spec** -- fill audit tables, write learned summary

### Critical Review Checklist (/implement stage 5)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | NLRIKey encoding is deterministic -- same NLRI bytes always produce same key. Value-type Release() still frees all pool handles. |
| Zero-copy | splitNLRIs still returns sub-slices into original data, not copies |
| No leak | Implicit withdraw (Insert over existing) calls Release() on old entry before overwrite |
| Map semantics | Value-type map: read-modify-write pattern correct for StaleLevel mutation |
| API compat | All callers adapted -- no dangling *RouteEntry references |
| Rule: no-layering | Old string-key code fully removed, not kept alongside NLRIKey |

### Deliverables Checklist (/implement stage 9)

| Deliverable | Verification method |
|-------------|---------------------|
| NLRIKey type and tests | `ls internal/component/bgp/plugins/rib/storage/nlrikey.go` |
| Value-type RouteEntry compiles | `go build ./internal/component/bgp/plugins/rib/...` |
| splitNLRIs pre-count | `grep "count" internal/component/bgp/plugins/rib/rib_nlri.go` |
| All tests pass | `make ze-verify` |

### Security Review Checklist (/implement stage 10)

| Check | What to look for |
|-------|-----------------|
| Input validation | NLRIKey must handle zero-length and oversized NLRI bytes safely (pad/truncate) |
| Panic safety | No index-out-of-bounds when NLRI bytes exceed 24 (should not happen for unicast, but defend) |
| Pool handle leak | Value-type copy semantics: ensure Release() is called exactly once per entry lifecycle |

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

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

- RouteEntry at 56 bytes (with alignment) is well under Go's 128-byte inline map value threshold (gc compiler specific).
- **NLRIKey is a struct, not a bare array.** `NLRIKey` has `len uint8` + `data [24]byte` = 28 bytes with padding. This solves the iteration problem: `IterateEntry` callbacks reconstruct exact-length NLRI bytes via `key.Data[:key.Len]`, so callers like `wireToPrefix` and `formatNLRIAsPrefix` receive correctly-sized bytes without trailing zeros.
- **Non-unicast families are not stored in FamilyRIB.** Both `rib.go` and `rib_structured.go` guard with `isSimplePrefixFamily()` / `isSimplePrefixFamilyNLRI()`. Only IPv4/IPv6 unicast/multicast enter the map, so 24 bytes is sufficient.
- The attrInterners table in attrparse.go uses closures over *RouteEntry -- these must change to work with value types. Options: (a) pass pointer to local var during parsing, (b) switch to direct field assignment without closure table. Option (a) preserves the table-driven pattern. ParseAttributes creates a local RouteEntry value, passes `&entry` to attrInterner closures, then returns the value.
- **Release() keeps pointer receiver.** Release() mutates the receiver (sets handles to InvalidHandle) as a defensive measure, but its real work is the pool.Release() side effects. With value-type map storage, callers do: get copy from map, call Release() on `&copy`, delete from map. The InvalidHandle mutation on the copy is harmless. No receiver change needed.
- **entriesEqual changes to value params.** Currently takes `(*RouteEntry, *RouteEntry)`, becomes `(RouteEntry, RouteEntry)`. This is a hot-path function (called on every Insert), but it only compares 13 uint32 fields -- the value copy is cheaper than the pointer dereference chain.
- **matchInEntry (rib_pipeline.go:510) takes *storage.RouteEntry.** Must change to value param. It reads fields for filter matching -- no mutation. Same for extractMPNextHop.
- PipelineRecord holds InEntry for read-only display. Value copy is fine -- the pipeline operates within PeerRIB's read lock, so the entry won't change.
- Clone() currently returns *RouteEntry with AddRef. With value types, Clone returns RouteEntry (value) with AddRef. Semantics identical.
- The map get-modify-put pattern for StaleLevel: get copy from map, modify StaleLevel, put back. This is O(1) extra map write but eliminates pointer indirection on every lookup.

## RFC Documentation

N/A -- this is an internal optimization, no RFC behavior changes.

## Implementation Summary

### What Was Implemented
- [pending]

### Bugs Found/Fixed
- [pending]

### Documentation Updates
- [pending]

### Deviations from Plan
- [pending]

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
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

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
- [ ] AC-1..AC-13 all demonstrated
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes (all 6 checks in `rules/quality.md` -- no failures)

### Quality Gates (SHOULD pass -- defer with user approval)
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
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes -- all 6 checks in `rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `plan/learned/NNN-rib-alloc.md`
- [ ] **Summary included in commit** -- NEVER commit implementation without the completed summary. One commit = code + tests + summary.
