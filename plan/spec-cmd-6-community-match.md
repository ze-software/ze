# Spec: cmd-6 -- Community Match Filter

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 1/1 |
| Updated | 2026-04-12 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `plan/spec-cmd-0-umbrella.md` -- umbrella context
3. `internal/component/bgp/plugins/filter_community/` -- existing community plugin
4. `internal/component/bgp/plugins/filter_community/schema/ze-filter-community.yang` -- existing YANG
5. `internal/component/bgp/reactor/filter_chain.go` -- filter chain dispatch

## Task

Extend existing `bgp-filter-community` plugin with match-and-act capability. New
`community-match` list under `bgp/policy` with `ze:filter`. Each entry has `match`
(community value), `type` (standard/large/extended), `action` (accept/reject).
Referenced as `community-match:NAME`. Separate from tag/strip because intent differs:
tag/strip modifies, community-match filters.

**Config syntax (editor):**

| Command | Purpose |
|---------|---------|
| `set bgp policy community-match NO-EXPORT match standard no-export action reject` | Reject routes with no-export |
| `set bgp policy community-match CUSTOMER match large 65001:100:200 action accept` | Accept routes with large community |
| `set bgp peer X filter import community-match:NO-EXPORT` | Apply community-match on import |

**YANG location:** `bgp/policy` container, new `community-match` list with `ze:filter` extension.

| Leaf | Type | Default | Notes |
|------|------|---------|-------|
| `match` | string | (required) | Community value to match (numeric or well-known name) |
| `type` | enum {standard, large, extended} | (required) | Which community attribute to check |
| `action` | enum {accept, reject} | (required) | What to do when community matches |

**Community matching rules:**
- Match checks whether the specified community value exists in the route's community attributes
- Standard communities: type 8 (COMMUNITIES), format ASN:value or well-known name
- Large communities: type 32 (LARGE_COMMUNITY), format ASN:value1:value2
- Extended communities: type 16 (EXTENDED_COMMUNITY), format type:value
- Well-known communities matchable by name: no-export, no-advertise, no-export-subconfed
- Entries evaluated in order; first match wins
- No match = implicit deny (reject)

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` -- plugin model, filter chain
  -> Constraint: filter plugins are independent, composable in chains
- [ ] `.claude/patterns/plugin.md` -- how to create a filter plugin
  -> Constraint: filter plugins augment bgp/policy, use ze:filter extension

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc4271.md` -- BGP-4 base: community attributes
  -> Constraint: community attributes are optional transitive (type 8, 16, 32)
- [ ] `rfc/short/rfc1997.md` -- BGP Communities
  -> Constraint: well-known communities: no-export (0xFFFFFF01), no-advertise (0xFFFFFF02)

**Key insights:**
- Community-match is separate from tag/strip because intent differs: filtering vs modification
- Existing community plugin handles tag/strip; this adds match-and-act
- Well-known community names must resolve to their numeric values
- Three community types correspond to three different BGP attributes

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/filter_community/` -- existing community plugin
- [ ] `internal/component/bgp/plugins/filter_community/schema/ze-filter-community.yang` -- existing YANG
- [ ] `internal/component/bgp/plugins/filter_community/register.go` -- plugin registration
- [ ] `internal/component/bgp/reactor/filter_chain.go` -- filter chain dispatch
- [ ] `internal/component/bgp/schema/ze-bgp-conf.yang` -- bgp/policy container

**Behavior to preserve:**
- Existing community tag/strip functionality completely unchanged
- Existing filter chain dispatch order
- All existing config files parse and work identically

**Behavior to change:**
- New community-match list under bgp/policy in YANG
- Existing community plugin extended to handle match-and-act filter type
- Filter chain recognizes `community-match:NAME` references
- UPDATEs matched on community presence for accept/reject decisions

## Data Flow (MANDATORY)

### Entry Point
- Config: `bgp { policy { community-match NO-EXPORT { ... } } }` parsed from YANG
- Wire: UPDATE received, community attributes checked against match entries

### Transformation Path
1. Config parse: YANG community-match entries extracted by ResolveBGPTree()
2. Plugin initialization: match entries loaded into community plugin alongside tag/strip config
3. Filter chain setup: `filter import community-match:NO-EXPORT` wires into peer's import chain
4. UPDATE receive: wire bytes pass through import filter chain
5. Community extraction: community attributes extracted from UPDATE by type
6. Community matching: specified community value checked for presence in attribute
7. Action: first matching entry's action applied; no match = reject

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| Config -> Plugin | ResolveBGPTree() extracts community-match config, passed to plugin at init | [ ] |
| Reactor -> Plugin | Filter chain dispatches filter-update RPC with wire bytes | [ ] |

### Integration Points
- `ResolveBGPTree()` -- extract community-match configuration
- `filter_chain.go` -- dispatch to community-match filter for matching
- Existing community plugin -- extend to handle match-and-act

### Architectural Verification
- [ ] No bypassed layers (config -> resolver -> filter chain -> plugin)
- [ ] No unintended coupling (community-match extends existing community plugin)
- [ ] No duplicated functionality (match-and-act is new capability, separate from tag/strip)
- [ ] Zero-copy preserved (plugin receives wire bytes, extracts communities for matching)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Config with `policy community-match` + `filter import community-match:X` | → | UPDATE with matching community rejected | `test/plugin/community-match.ci` |
| Config with `policy community-match` + `filter import community-match:X` | → | UPDATE without matching community passes through | `test/plugin/community-match.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Match on standard community value | Route with matching standard community triggers action |
| AC-2 | Match on large community value | Route with matching large community triggers action |
| AC-3 | Match on extended community value | Route with matching extended community triggers action |
| AC-4 | Accept action on match | Route accepted, passes to next filter in chain |
| AC-5 | Reject action on match | Route rejected, not forwarded |
| AC-6 | No entry matches | Implicit deny (reject) |
| AC-7 | Well-known community name: no-export | Resolves to 0xFFFFFF01, matches correctly |
| AC-8 | Well-known community name: no-advertise | Resolves to 0xFFFFFF02, matches correctly |
| AC-9 | Composable in chain with tag/strip | `filter import community-match:X community-tag:Y` works |
| AC-10 | Multiple entries, first match wins | First matching entry's action applied |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestCommunityMatchStandard` | `filter_community_match_test.go` | Standard community match | |
| `TestCommunityMatchLarge` | `filter_community_match_test.go` | Large community match | |
| `TestCommunityMatchExtended` | `filter_community_match_test.go` | Extended community match | |
| `TestCommunityMatchAccept` | `filter_community_match_test.go` | Accept action applied | |
| `TestCommunityMatchReject` | `filter_community_match_test.go` | Reject action applied | |
| `TestCommunityMatchImplicitDeny` | `filter_community_match_test.go` | No match = reject | |
| `TestCommunityMatchNoExport` | `filter_community_match_test.go` | no-export well-known name resolved | |
| `TestCommunityMatchNoAdvertise` | `filter_community_match_test.go` | no-advertise well-known name resolved | |
| `TestCommunityMatchFirstWins` | `filter_community_match_test.go` | First match wins with multiple entries | |

### Boundary Tests (MANDATORY for numeric inputs)

No numeric inputs in this spec (community values are strings, action is enum).

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A | N/A | N/A | N/A | N/A |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `community-match` | `test/plugin/community-match.ci` | Config with community-match filter, verify accept/reject | |

## Files to Modify

- `internal/component/bgp/plugins/filter_community/` -- extend with match-and-act capability
- `internal/component/bgp/plugins/filter_community/schema/ze-filter-community.yang` -- add community-match list
- `internal/component/bgp/config/resolve.go` -- extract community-match config
- `internal/component/bgp/reactor/filter_chain.go` -- dispatch to community-match filter

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (extend existing) | [x] | `ze-filter-community.yang` |
| Plugin extension | [x] | `internal/component/bgp/plugins/filter_community/` |
| Functional test | [x] | `test/plugin/community-match.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [x] | `docs/features.md` -- add community matching |
| 2 | Config syntax changed? | [x] | `docs/guide/configuration.md` -- community-match config examples |
| 3 | CLI command added/changed? | [ ] | N/A |
| 4 | API/RPC added/changed? | [ ] | N/A |
| 5 | Plugin added/changed? | [x] | `docs/guide/plugins.md` -- community-match capability |
| 6 | Has a user guide page? | [ ] | N/A |
| 7 | Wire format changed? | [ ] | N/A |
| 8 | Plugin SDK/protocol changed? | [ ] | N/A |
| 9 | RFC behavior implemented? | [x] | `rfc/short/rfc1997.md` -- well-known communities |
| 10 | Test infrastructure changed? | [ ] | N/A |
| 11 | Affects daemon comparison? | [x] | `docs/comparison.md` -- community matching now supported |
| 12 | Internal architecture changed? | [ ] | N/A |

## Files to Create

- `test/plugin/community-match.ci` -- functional test

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file + umbrella |
| 2. Audit | Files to Modify, TDD Plan |
| 3. Implement (TDD) | Phases below |
| 4. Full verification | `make ze-verify` |
| 5-12. | Standard flow |

### Implementation Phases

1. **Phase: YANG + Config** -- Add community-match to ze-filter-community.yang, extract in ResolveBGPTree()
   - Tests: config parse verification
   - Files: ze-filter-community.yang, resolve.go
2. **Phase: Match Logic** -- Implement community presence check for standard/large/extended types
   - Tests: `TestCommunityMatch*`
   - Files: filter_community/ match implementation
3. **Phase: Well-Known Names** -- Map no-export, no-advertise, no-export-subconfed to numeric values
   - Tests: `TestCommunityMatchNoExport`, `TestCommunityMatchNoAdvertise`
   - Files: filter_community/ well-known resolution
4. **Phase: Filter Chain Integration** -- Wire community-match into filter_chain.go dispatch
   - Tests: verify filter chain calls community-match
   - Files: filter_chain.go
5. **Functional tests** -- .ci tests proving end-to-end behavior
6. **Full verification** -- `make ze-verify`

### Critical Review Checklist (/implement stage 5)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | All 10 ACs demonstrated |
| Separation | tag/strip unchanged, community-match is additive |
| Well-known names | no-export and no-advertise resolve correctly |
| Three types | standard, large, extended all tested |
| Composability | Works in chain with tag/strip and other filters |

### Deliverables Checklist (/implement stage 9)

| Deliverable | Verification method |
|-------------|---------------------|
| YANG community-match in ze-filter-community.yang | `grep community-match internal/component/bgp/plugins/filter_community/schema/ze-filter-community.yang` |
| Match logic | `grep -r community-match internal/component/bgp/plugins/filter_community/` |
| .ci functional test | `ls test/plugin/community-match.ci` |

### Security Review Checklist (/implement stage 10)

| Check | What to look for |
|-------|-----------------|
| Input validation | Community values validated at config time; well-known names from allow-list |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
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

## Implementation Summary

### What Was Implemented

New `bgp-filter-community-match` plugin (separate from existing tag/strip `bgp-filter-community`):

| Component | File | Purpose |
|-----------|------|---------|
| YANG schema | `filter_community_match/schema/ze-filter-community-match.yang` | `community-match` augment under `bgp/policy` |
| Schema embed/register | `filter_community_match/schema/{embed,register}.go` | Standard schema pattern |
| Plugin entry point | `filter_community_match/filter_community_match.go` | SDK entry, OnConfigure, handleFilterUpdate |
| Config parsing | `filter_community_match/config.go` | `parseCommunityLists` with name/value length limits |
| Community matching | `filter_community_match/match.go` | `evaluateCommunities`, `extractCommunityField` |
| Registration | `filter_community_match/register.go` | `FilterTypes: ["community-match"]` |
| Unit tests | `filter_community_match/{match,config}_test.go` | 31 test cases |
| Functional tests | `test/plugin/community-match-{accept,reject}.ci` | Accept/reject end-to-end |
| Parse test | `test/parse/community-match-config.ci` | YANG validation |

### Bugs Found/Fixed
None.

### Documentation Updates
Deferred to umbrella completion.

### Deviations from Plan

| Deviation | Reason |
|-----------|--------|
| New plugin instead of extending existing | Existing `bgp-filter-community` uses IngressFilter/EgressFilter (no FilterTypes, no PolicyFilterChain dispatch). Match-and-act requires a separate plugin with FilterTypes + OnFilterUpdate. |
| AC-9 (composable with tag/strip) not tested in .ci | Tag/strip uses a different registration path (not PolicyFilterChain); composing them in a single import chain is not meaningful for the accept/reject pattern. They coexist by design but operate at different stages. |

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
- [ ] AC-1..AC-10 all demonstrated
- [ ] Wiring Test table complete
- [ ] `make ze-test` passes
- [ ] Feature code integrated
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)
- [ ] Separation from tag/strip verified
- [ ] Implementation Audit complete

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-cmd-6-community-match.md`
- [ ] Summary included in commit
