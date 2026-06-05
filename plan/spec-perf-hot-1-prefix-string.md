# Spec: perf-hot-1-prefix-string

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
3. `ai/rules/memory-architecture.md` - allocation rules
4. `ai/rules/no-sprintf-alloc.md` - hot path rules
5. `internal/component/bgp/plugins/rib/rib_structured.go` - storeSentEntries, removeSentNLRIs, wirePrefixToString
6. `internal/component/bgp/plugins/rib/ribout_entry.go` - ribOutEntry, outRouteKey, parseOutRouteKey, setRibOutSource, collectRibOutRoutes
7. `internal/component/bgp/plugins/rib/rib_replay.go` - collectGroupedRibOutRoutesFiltered
8. `internal/component/bgp/plugins/rib/rib_pipeline.go` - outboundSource iteration
9. `internal/component/bgp/plugins/rib/event.go` - outRouteKey

## Task

Replace string-keyed ribOut maps with value-type `ribOutKey` (netip.Prefix + uint32 pathID)
in the rib plugin's sent-UPDATE path. Currently, `storeSentEntries` and `removeSentNLRIs`
call `wirePrefixToString()` per NLRI to build a string map key via `outRouteKey()`. This
allocates a heap string per prefix. Profiling shows `Prefix.String()` accounts for 5.3M
object allocations (80 MB) across the benchmark, and `rib.storeSentEntries` holds 8.22 MB
in-use at snapshot time.

The fix introduces a `ribOutKey` struct `{Prefix netip.Prefix; PathID uint32}` (20 bytes,
comparable value type, zero allocation) and changes ribOut from
`map[string]map[family.Family]map[string]ribOutEntry` to
`map[string]map[family.Family]map[ribOutKey]ribOutEntry`. The `ribOutSource` map gets the
same key change. All consumers that iterate or look up ribOut entries are updated to use
`ribOutKey` instead of string keys. The `outRouteKey` and `parseOutRouteKey` functions
become unused on the structured path and can be removed if the JSON event path (`handleSent`)
is also migrated; otherwise they remain for backward compatibility.

Part of spec set `perf-hot` (umbrella: `spec-perf-hot-0-umbrella.md`).

## Required Reading

### Architecture Docs
- [ ] `ai/rules/memory-architecture.md` - allocation avoidance on hot paths
  -> Constraint: no heap allocation per message on hot paths; use value types or pools
- [ ] `ai/rules/no-sprintf-alloc.md` - hot path definition and alternatives
  -> Constraint: storeSentEntries and removeSentNLRIs are hot path (called per sent UPDATE)
- [ ] `ai/rules/enum-over-string.md` - numeric keys over string keys in maps
  -> Decision: use struct key (value type) instead of string key for map dispatch

### RFC Summaries (MUST for protocol work)
- Not applicable. This is an internal optimization with no wire format changes.

**Key insights:**
- ribOut maps are keyed by string today; changing to value-type struct key eliminates per-prefix string allocation
- `netip.Prefix` is a 20-byte comparable value type in Go stdlib, no allocation on map insert/lookup
- Consumers of ribOut (replay, pipeline, CLI show) reconstruct full Route structs; they need prefix+pathID which the new key provides directly without parsing

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/rib/rib_structured.go` - storeSentEntries (line 343): walks NLRI bytes via NLRIIterator, calls wirePrefixToString per prefix, builds string key via outRouteKey, stores ribOutEntry. removeSentNLRIs (line 378): same pattern for deletions. wirePrefixToString (line 507): parses wire bytes into netip.Prefix, calls .String() to return heap-allocated string.
  -> Constraint: wirePrefixToString already builds a netip.Prefix internally before calling .String(); returning the Prefix directly eliminates the allocation
- [ ] `internal/component/bgp/plugins/rib/ribout_entry.go` - ribOutEntry (line 31): compact 16-byte struct. outRouteKey (event.go line 39): builds "prefix:pathID" string. parseOutRouteKey (line 113): reverses outRouteKey by parsing the string back. setRibOutSource (line 216): uses string key in ribOutSource map. releaseRibOutSource (line 233): same. ribOutSourcePeer (line 254): same. collectRibOutRoutes (line 262): iterates ribOut, calls reconstructRoute with string key.
  -> Constraint: reconstructRoute takes the string key and calls parseOutRouteKey to extract prefix+pathID; with ribOutKey, it receives these fields directly
- [ ] `internal/component/bgp/plugins/rib/rib_replay.go` - collectGroupedRibOutRoutesFiltered (line 51): iterates ribOut, calls parseOutRouteKey to extract prefix+pathID for groupKey and prefixes list. The prefixes list is []string used downstream for replay command formatting.
  -> Constraint: replay needs prefix as string for command formatting; convert from netip.Prefix at replay time (cold path) instead of storing as string
- [ ] `internal/component/bgp/plugins/rib/rib_pipeline.go` - outboundSource (line 242): iterates ribOut, calls reconstructRoute with the string key. Sets RouteItem.Prefix from rt.Prefix (which was parsed from the key string).
  -> Constraint: pipeline is a cold path (CLI show); allocating prefix string there is acceptable
- [ ] `internal/component/bgp/plugins/rib/rib.go` - handleSent (line 726): the JSON event path also stores into ribOut using string outRouteKey. This function processes JSON events from external plugins.
  -> Constraint: handleSent must also use ribOutKey; it already calls parseNLRIValue which returns (string, uint32); the string must be parsed to netip.Prefix
- [ ] `internal/component/bgp/plugins/rib/event.go` - outRouteKey (line 39): builds string key. Currently called from storeSentEntries, removeSentNLRIs, handleSent, and one test.

**Behavior to preserve:**
- RIB replay produces identical UPDATE commands after the change
- `show bgp rib` CLI output shows identical prefix strings
- Route refresh triggers identical re-advertisement
- ribOut reference counting (ribOutSource) tracks identical route sets
- JSON event path (handleSent) stores into ribOut correctly

**Behavior to change:**
- Internal map key type changes from string to ribOutKey struct
- wirePrefixToString replaced by wirePrefixToKey (returns netip.Prefix, not string)
- outRouteKey/parseOutRouteKey no longer used on the structured (hot) path

## Data Flow (MANDATORY)

### Entry Point
- Sent UPDATE wire bytes arrive at handleSentStructured via DirectBridge
- Wire bytes contain NLRI data that must be stored in ribOut for replay

### Transformation Path
1. handleSentStructured receives StructuredEvent with WireUpdate
2. NLRIIterator walks wire bytes, yielding (wirePrefix []byte, pathID uint32) per NLRI
3. **Today:** wirePrefixToString converts wirePrefix to string; outRouteKey builds string key
4. **After:** wirePrefixToKey converts wirePrefix to netip.Prefix; ribOutKey{Prefix, PathID} is the map key
5. ribOutEntry stored/deleted in ribOut map
6. ribOutSource tracks source peer using the same key type

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Wire bytes to map key | wirePrefixToKey parses wire NLRI to netip.Prefix | [ ] |
| Map key to display string | Prefix.String() called at display time (replay, CLI) | [ ] |

### Integration Points
- `nlri.NewNLRIIterator` - provides (wirePrefix, pathID) tuples; unchanged
- `reconstructRoute` - called on cold paths; signature changes to accept ribOutKey instead of string
- `collectGroupedRibOutRoutesFiltered` - iterates ribOut with new key type; extracts prefix string for replay at iteration time
- `handleSent` (JSON path) - must parse prefix string to netip.Prefix for ribOutKey construction

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Sent UPDATE via DirectBridge | -> | storeSentEntries with ribOutKey | `TestStoreSentEntriesRibOutKey` |
| Withdrawal via DirectBridge | -> | removeSentNLRIs with ribOutKey | `TestRemoveSentNLRIsRibOutKey` |
| Route refresh request | -> | collectRibOutRoutes with ribOutKey | `TestRibOutReplayWithValueKey` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | storeSentEntries called with IPv4 NLRI wire bytes | ribOut entry stored under ribOutKey{Prefix, PathID}; no string allocation for the key |
| AC-2 | removeSentNLRIs called for same NLRI | ribOut entry removed using ribOutKey lookup |
| AC-3 | collectRibOutRoutes called after store | returns Route with correct prefix string (converted from ribOutKey.Prefix at read time) |
| AC-4 | collectGroupedRibOutRoutesFiltered called | groups routes correctly; replay produces valid UPDATE commands |
| AC-5 | handleSent (JSON event path) stores route | ribOut entry stored under ribOutKey parsed from prefix string |
| AC-6 | setRibOutSource/releaseRibOutSource track references | source peer tracking uses ribOutKey; reference counting works identically |
| AC-7 | Benchmark allocation profile | wirePrefixToString no longer appears in pprof alloc profile for the hot path |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRibOutKeyEquality` | `internal/component/bgp/plugins/rib/ribout_entry_test.go` | ribOutKey struct is comparable; same prefix+pathID equals, different does not | |
| `TestWirePrefixToKey` | `internal/component/bgp/plugins/rib/rib_structured_test.go` | wirePrefixToKey returns correct netip.Prefix for IPv4 and IPv6 wire bytes | |
| `TestStoreSentEntriesRibOutKey` | `internal/component/bgp/plugins/rib/rib_test.go` | storeSentEntries stores entry under ribOutKey, retrievable by collectRibOutRoutes | |
| `TestRemoveSentNLRIsRibOutKey` | `internal/component/bgp/plugins/rib/rib_test.go` | removeSentNLRIs deletes the correct entry | |
| `TestRibOutReplayWithValueKey` | `internal/component/bgp/plugins/rib/rib_replay_test.go` | collectGroupedRibOutRoutesFiltered groups correctly with ribOutKey | |
| `TestRibOutSourceRefWithValueKey` | `internal/component/bgp/plugins/rib/ribout_entry_test.go` | setRibOutSource/releaseRibOutSource work with ribOutKey | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Prefix length IPv4 | 0-32 | /32 | N/A | /33 (wirePrefixToKey returns invalid) |
| Prefix length IPv6 | 0-128 | /128 | N/A | /129 (wirePrefixToKey returns invalid) |
| PathID | 0-4294967295 | max uint32 | N/A | N/A (uint32 type) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| Existing `show bgp rib` tests | `test/plugin/*.ci` | CLI output shows correct prefixes after ribOut key change | |

### Interop Tests (MANDATORY for protocol features)
- N/A: purely internal optimization, no wire format change

### Future (if deferring any tests)
- None

## Files to Modify
- `internal/component/bgp/plugins/rib/ribout_entry.go` - add ribOutKey type; change setRibOutSource, releaseRibOutSource, ribOutSourcePeer, collectRibOutRoutes, reconstructRoute to use ribOutKey
- `internal/component/bgp/plugins/rib/rib_structured.go` - replace wirePrefixToString with wirePrefixToKey in storeSentEntries and removeSentNLRIs; change ribOut map type
- `internal/component/bgp/plugins/rib/rib.go` - change ribOut field type; update handleSent to use ribOutKey; update ribOut initialization
- `internal/component/bgp/plugins/rib/rib_replay.go` - update collectGroupedRibOutRoutesFiltered to iterate ribOutKey maps
- `internal/component/bgp/plugins/rib/rib_pipeline.go` - update outboundSource to iterate ribOutKey maps
- `internal/component/bgp/plugins/rib/event.go` - keep outRouteKey if handleSent still needs it; otherwise remove
- `internal/component/bgp/plugins/rib/rib_test.go` - update tests using ribOut maps
- `internal/component/bgp/plugins/rib/ribout_entry_test.go` - update tests for new key type

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
| 12 | Internal architecture changed? | No | Internal data structure change only |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | |
| 15 | Registered plugin, event type, send type, command, capability, or runtime inventory changed? | No | |
| 16 | Any changed source file is referenced by existing doc source anchors? | [ ] | Grep docs/ for source anchors pointing at changed files |
| 17 | Existing docs show config/CLI/API examples for this area? | No | |

## Files to Create
- None (all changes are to existing files)

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, TDD Test Plan -- check what exists |
| 3. Wiring phase | Wiring Test table -- write failing tests for ribOutKey usage |
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

1. **Phase: Wiring (MANDATORY FIRST)** -- define ribOutKey type and wirePrefixToKey, write failing tests
   - Tests: `TestRibOutKeyEquality`, `TestWirePrefixToKey`
   - Files: `ribout_entry.go` (add type), `rib_structured.go` (add wirePrefixToKey)
   - Verify: type exists and compiles; tests fail because storeSentEntries still uses string keys

2. **Phase: ribOut map migration** -- change ribOut and ribOutSource field types from string key to ribOutKey
   - Tests: `TestStoreSentEntriesRibOutKey`, `TestRemoveSentNLRIsRibOutKey`
   - Files: `rib.go` (field type + init), `rib_structured.go` (storeSentEntries, removeSentNLRIs), `ribout_entry.go` (setRibOutSource, releaseRibOutSource, ribOutSourcePeer)
   - Verify: tests pass; storeSentEntries and removeSentNLRIs no longer call wirePrefixToString

3. **Phase: consumer migration** -- update all readers of ribOut maps
   - Tests: `TestRibOutReplayWithValueKey`, `TestRibOutSourceRefWithValueKey`
   - Files: `rib_replay.go`, `rib_pipeline.go`, `ribout_entry.go` (collectRibOutRoutes, reconstructRoute)
   - Verify: replay and pipeline tests pass; reconstructRoute receives ribOutKey directly

4. **Phase: JSON event path** -- update handleSent to use ribOutKey
   - Tests: existing handleSent tests
   - Files: `rib.go` (handleSent)
   - Verify: handleSent parses prefix string to netip.Prefix for ribOutKey

5. **Functional tests** -- run existing functional tests to verify no regression
6. **Full verification** -- `make ze-verify`
7. **Complete spec** -- audit tables, learned summary

### Critical Review Checklist (/implement stage 6)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | ribOutKey equality semantics match previous string key semantics (same prefix+pathID = same key) |
| No string leaks | grep for wirePrefixToString calls; must be zero on hot path (only remain if needed by unrelated code) |
| Replay fidelity | collectGroupedRibOutRoutesFiltered produces identical replay commands as before |
| Pipeline output | outboundSource produces identical RouteItem.Prefix values |
| handleSent parity | JSON event path stores entries under same logical key as structured path |
| Rule: no-layering | outRouteKey/parseOutRouteKey removed or only used by JSON path |

### Deliverables Checklist (/implement stage 10)

| Deliverable | Verification method |
|-------------|---------------------|
| ribOutKey type defined | `grep -n 'type ribOutKey' internal/component/bgp/plugins/rib/ribout_entry.go` |
| wirePrefixToKey function | `grep -n 'func wirePrefixToKey' internal/component/bgp/plugins/rib/rib_structured.go` |
| ribOut field uses ribOutKey | `grep 'ribOut.*map.*ribOutKey' internal/component/bgp/plugins/rib/rib.go` |
| No wirePrefixToString on hot path | `grep -n wirePrefixToString internal/component/bgp/plugins/rib/rib_structured.go` returns 0 or only function definition |
| Tests pass | `go test ./internal/component/bgp/plugins/rib/...` |

### Security Review Checklist (/implement stage 11)

| Check | What to look for |
|-------|-----------------|
| Input validation | wirePrefixToKey must validate wire bytes length and prefix length bounds, returning invalid Prefix for malformed input (same as wirePrefixToString returning "") |
| Map key safety | ribOutKey uses fixed-size value types; no risk of unbounded key growth |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior; RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural then DESIGN phase |
| Functional test fails | Check AC; if AC wrong then DESIGN; if AC correct then IMPLEMENT |
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

## Core Insight

The ribOut map key stores a string that was created by formatting a netip.Prefix, which
was itself created from wire bytes. The intermediate Prefix is the natural key; converting
to string and back is pure waste. By stopping at the Prefix, the hot path avoids 2
allocations per NLRI (the Prefix.String() result and the outRouteKey concatenation).

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Use `ribOutKey{Prefix, PathID}` struct | (a) Use raw wire bytes as key, (b) Use [20]byte array | netip.Prefix is comparable, self-documenting, and already constructed in wirePrefixToKey; raw bytes would need length tracking; fixed array loses prefix-length semantics |
| Keep outRouteKey for JSON event path if needed | Remove outRouteKey entirely | handleSent receives prefix as string from JSON; parsing to netip.Prefix is straightforward, so outRouteKey can be removed if handleSent is migrated |
| Convert Prefix to string only on cold paths | Intern prefix strings in a pool | Interning adds complexity and memory; the cold paths (replay, CLI show) are infrequent enough that per-call String() is fine |

## Known Limitations
- The JSON event path (handleSent) must parse prefix strings to netip.Prefix, adding a small cost to that path; this path is already not performance-critical
- Non-unicast families (VPN, EVPN) with complex NLRI formats do not pass through storeSentEntries/removeSentNLRIs in the structured path, so this optimization does not affect them

## RFC Documentation

Not applicable. No RFC behavior changed.

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
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Eliminate Prefix.String() allocations on ribOut hot path | Benchmark + pprof | wirePrefixToString absent from pprof allocs after change |
| Preserve ribOut replay fidelity | Functional test | Existing replay tests pass unchanged |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied
- [To be filled after review]

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
- [ ] AC-1..AC-7 all demonstrated
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled -- 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Architecture docs and guides updated where changed behavior is documented
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
- [ ] Interop tests for protocol features (or N/A: internal optimization, no wire change)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes -- all 6 checks in `rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec (with all edits) + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/<spec>` only (preserves edited spec in git history from commit A)
