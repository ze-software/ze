# Spec: cmd-7 -- Route Attribute Modifier Plugin

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | - |
| Updated | 2026-04-11 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `plan/spec-cmd-0-umbrella.md` -- umbrella context
3. `internal/component/bgp/plugins/filter_community/` -- existing filter plugin pattern
4. `internal/component/bgp/reactor/filter_chain.go` -- filter chain dispatch
5. `internal/component/bgp/reactor/forward_build.go` -- buildModifiedPayload()

## Task

Create `bgp-filter-modify` plugin. Named modifier definitions under `bgp/policy` with
`ze:filter`. Each definition sets attributes: local-preference (uint32), med (uint32),
origin (igp/egp/incomplete), next-hop (ip-address), as-path-prepend (uint8 count).
Referenced as `modify:NAME`. This is the "set" half of route-maps. Ze separates match
from modify for composability.

**Config syntax (editor):**

| Command | Purpose |
|---------|---------|
| `set bgp policy modify PREFER-LOCAL set local-preference 200` | Set local-preference on matched routes |
| `set bgp policy modify LOWER-MED set med 50` | Set MED on matched routes |
| `set bgp policy modify SET-ORIGIN set origin igp` | Set origin on matched routes |
| `set bgp policy modify SET-NH set next-hop 10.0.0.1` | Set next-hop on matched routes |
| `set bgp policy modify PREPEND set as-path-prepend 3` | Prepend local AS 3 times on matched routes |
| `set bgp peer X filter import modify:PREFER-LOCAL` | Apply modifier on import |
| `set bgp peer X filter import prefix-list:X modify:PREFER-LOCAL` | Modify after match (composable) |

**YANG location:** `bgp/policy` container, new `modify` list with `ze:filter` extension.

| Leaf | Type | Default | Notes |
|------|------|---------|-------|
| `local-preference` | uint32 | (unset) | Set LOCAL_PREF attribute |
| `med` | uint32 | (unset) | Set MULTI_EXIT_DISC attribute |
| `origin` | enum {igp, egp, incomplete} | (unset) | Set ORIGIN attribute |
| `next-hop` | inet:ip-address | (unset) | Set NEXT_HOP attribute |
| `as-path-prepend` | uint8 | (unset) | Number of times to prepend local AS to AS_PATH |

**Modifier rules:**
- Only declared attributes are modified; undeclared attributes are preserved unchanged
- Multiple attributes can be set in a single modifier definition
- Modifier is the "set" half -- it applies unconditionally to routes that reach it in the chain
- For conditional modification: compose with match filters (prefix-list, as-path-list, community-match)
- Modification requires copy-on-modify: original wire bytes preserved, modified copy written to outgoing pool

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` -- zero-copy, copy-on-modify pattern
  -> Constraint: modification triggers copy to Outgoing Peer Pool; original preserved
- [ ] `.claude/patterns/plugin.md` -- how to create a filter plugin
  -> Constraint: filter plugins augment bgp/policy, use ze:filter extension
- [ ] `rules/buffer-first.md` -- wire encoding pattern
  -> Constraint: modifications use WriteTo(buf, off) int, not append or make

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc4271.md` -- BGP-4 base: UPDATE attributes
  -> Constraint: LOCAL_PREF (type 5), MULTI_EXIT_DISC (type 4), ORIGIN (type 1), NEXT_HOP (type 3), AS_PATH (type 2)

**Key insights:**
- Route modification is the "set" half of route-maps; Ze separates match from modify
- Modification triggers copy-on-modify: buffer allocated from Outgoing Peer Pool
- Only declared attributes modified; the rest pass through unchanged
- AS-path prepend adds N copies of local AS at the beginning of AS_PATH
- buildModifiedPayload() in forward_build.go is the existing modification path

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/filter_community/` -- existing filter plugin pattern
- [ ] `internal/component/bgp/reactor/filter_chain.go` -- filter chain dispatch
- [ ] `internal/component/bgp/reactor/forward_build.go` -- buildModifiedPayload()
- [ ] `internal/component/bgp/reactor/reactor_api_forward.go` -- UPDATE forwarding
- [ ] `internal/component/bgp/schema/ze-bgp-conf.yang` -- bgp/policy container

**Behavior to preserve:**
- Existing filter chain dispatch order
- Existing community tag/strip modification functionality
- Existing copy-on-modify pattern in forward_build.go
- Zero-copy path for unmodified routes
- All existing config files parse and work identically

**Behavior to change:**
- New modify list under bgp/policy in YANG
- New bgp-filter-modify plugin registered
- Filter chain recognizes `modify:NAME` references
- UPDATEs passing through modify filter have specified attributes rewritten

## Data Flow (MANDATORY)

### Entry Point
- Config: `bgp { policy { modify PREFER-LOCAL { set { local-preference 200; } } } }` parsed from YANG
- Wire: UPDATE passes through filter chain, modifier applies attribute changes

### Transformation Path
1. Config parse: YANG modify entries extracted by ResolveBGPTree()
2. Plugin registration: bgp-filter-modify plugin registers with filter registry
3. Filter chain setup: `filter import modify:PREFER-LOCAL` wires modifier into peer's import chain
4. UPDATE receive: wire bytes pass through import filter chain
5. Match filters: preceding filters (prefix-list, as-path-list) accept or reject
6. Modifier application: for accepted routes reaching the modifier, copy-on-modify triggered
7. Attribute rewriting: specified attributes written to new buffer; unspecified preserved
8. Forwarding: modified buffer sent to destination peer

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| Config -> Plugin | ResolveBGPTree() extracts modify config, passed to plugin at init | [ ] |
| Reactor -> Plugin | Filter chain dispatches filter-update RPC with wire bytes | [ ] |
| Plugin -> Wire | Modified attributes written via buffer-first pattern to Outgoing Peer Pool | [ ] |

### Integration Points
- `ResolveBGPTree()` -- extract modify configuration
- `filter_chain.go` -- dispatch to modify plugin
- `forward_build.go` -- buildModifiedPayload() integration for attribute rewriting
- spec-apply-mods infrastructure -- wire-level attribute rewriting support

### Architectural Verification
- [ ] No bypassed layers (config -> resolver -> filter chain -> plugin -> wire)
- [ ] No unintended coupling (modify is independent, composable with match filters)
- [ ] No duplicated functionality (extends existing modification path)
- [ ] Zero-copy preserved (copy only when modification needed, via Outgoing Peer Pool)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Config with `policy modify` + `filter import modify:X` | → | UPDATE has local-preference set | `test/plugin/route-modify.ci` |
| Config with `filter import prefix-list:X modify:Y` | → | Only matched routes modified | `test/plugin/route-modify.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `set local-preference 200` on import | Ingress routes have LOCAL_PREF set to 200 |
| AC-2 | `set med 50` on export | Egress routes have MED set to 50 |
| AC-3 | `set origin igp` | Route ORIGIN attribute set to IGP |
| AC-4 | `set next-hop 10.0.0.1` | Route NEXT_HOP set to 10.0.0.1 |
| AC-5 | `set as-path-prepend 3` | Local AS prepended 3 times to AS_PATH |
| AC-6 | Composable: `prefix-list:X modify:Y` | Modifier applied only to routes accepted by prefix-list |
| AC-7 | Only declared attributes modified | Undeclared attributes preserved unchanged |
| AC-8 | Multiple attributes in one modifier | All specified attributes set in single pass |
| AC-9 | No modify config (existing deployments) | Behavior identical to current Ze |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestModifyLocalPreference` | `filter_modify_test.go` | LOCAL_PREF attribute set | |
| `TestModifyMED` | `filter_modify_test.go` | MED attribute set | |
| `TestModifyOrigin` | `filter_modify_test.go` | ORIGIN attribute set | |
| `TestModifyNextHop` | `filter_modify_test.go` | NEXT_HOP attribute set | |
| `TestModifyASPathPrepend` | `filter_modify_test.go` | AS_PATH prepended N times | |
| `TestModifyMultipleAttributes` | `filter_modify_test.go` | Multiple attributes set in single pass | |
| `TestModifyPreservesUndeclared` | `filter_modify_test.go` | Undeclared attributes unchanged | |
| `TestModifyComposable` | `filter_modify_test.go` | Works after match filter in chain | |

### Boundary Tests (MANDATORY for numeric inputs)

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| local-preference | 0-4294967295 | 4294967295 | N/A (uint32) | N/A |
| med | 0-4294967295 | 4294967295 | N/A (uint32) | N/A |
| as-path-prepend | 0-255 | 255 | N/A (uint8) | 256 |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `route-modify` | `test/plugin/route-modify.ci` | Config with modifier, verify attribute changes in wire output | |

## Files to Modify

- `internal/component/bgp/schema/ze-bgp-conf.yang` -- add modify list under bgp/policy
- `internal/component/bgp/config/resolve.go` -- extract modify config
- `internal/component/bgp/reactor/filter_chain.go` -- dispatch to modify plugin
- `internal/component/bgp/reactor/forward_build.go` -- integrate modification into buildModifiedPayload()

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new list) | [x] | `internal/component/bgp/schema/ze-bgp-conf.yang` |
| Plugin registration | [x] | `internal/component/bgp/plugins/filter_modify/register.go` |
| Functional test | [x] | `test/plugin/route-modify.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [x] | `docs/features.md` -- add route attribute modification |
| 2 | Config syntax changed? | [x] | `docs/guide/configuration.md` -- modify filter config examples |
| 3 | CLI command added/changed? | [ ] | N/A |
| 4 | API/RPC added/changed? | [ ] | N/A |
| 5 | Plugin added/changed? | [x] | `docs/guide/plugins.md` -- bgp-filter-modify plugin |
| 6 | Has a user guide page? | [ ] | N/A |
| 7 | Wire format changed? | [ ] | N/A |
| 8 | Plugin SDK/protocol changed? | [ ] | N/A |
| 9 | RFC behavior implemented? | [x] | `rfc/short/rfc4271.md` -- attribute types |
| 10 | Test infrastructure changed? | [ ] | N/A |
| 11 | Affects daemon comparison? | [x] | `docs/comparison.md` -- route attribute modification now supported |
| 12 | Internal architecture changed? | [ ] | N/A |

## Files to Create

- `internal/component/bgp/plugins/filter_modify/` -- new plugin directory
- `internal/component/bgp/plugins/filter_modify/register.go` -- plugin registration
- `internal/component/bgp/plugins/filter_modify/modify.go` -- attribute modification logic
- `internal/component/bgp/plugins/filter_modify/modify_test.go` -- unit tests
- `test/plugin/route-modify.ci` -- functional test

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

1. **Phase: YANG + Config** -- Add modify list to ze-bgp-conf.yang, extract in ResolveBGPTree()
   - Tests: config parse verification
   - Files: ze-bgp-conf.yang, resolve.go
2. **Phase: Plugin Skeleton** -- Create bgp-filter-modify plugin following community filter pattern
   - Tests: `TestModifyLocalPreference`
   - Files: filter_modify/register.go, filter_modify/modify.go
3. **Phase: Attribute Modification** -- Implement each attribute setter using buffer-first pattern
   - Tests: `TestModify*` for each attribute type
   - Files: filter_modify/modify.go, forward_build.go
4. **Phase: Composability** -- Ensure modifier works after match filters in chain
   - Tests: `TestModifyComposable`
   - Files: filter_chain.go
5. **Functional tests** -- .ci tests proving end-to-end behavior
6. **Full verification** -- `make ze-verify`

### Critical Review Checklist (/implement stage 5)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | All 9 ACs demonstrated |
| Copy-on-modify | Original wire bytes preserved; modification uses Outgoing Peer Pool |
| Buffer-first | All attribute writes use WriteTo(buf, off) int |
| Composability | Works in chain with prefix-list, as-path-list, community-match |
| Backward compat | No modify config = identical behavior |

### Deliverables Checklist (/implement stage 9)

| Deliverable | Verification method |
|-------------|---------------------|
| YANG modify list in ze-bgp-conf.yang | `grep 'modify' internal/component/bgp/schema/ze-bgp-conf.yang` |
| Plugin directory | `ls internal/component/bgp/plugins/filter_modify/` |
| .ci functional test | `ls test/plugin/route-modify.ci` |

### Security Review Checklist (/implement stage 10)

| Check | What to look for |
|-------|-----------------|
| Input validation | Attribute values within RFC-defined ranges; next-hop must be valid IP |
| Buffer safety | WriteTo respects buffer bounds; no overflow possible |

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
### Bugs Found/Fixed
### Documentation Updates
### Deviations from Plan

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
- [ ] AC-1..AC-9 all demonstrated
- [ ] Wiring Test table complete
- [ ] `make ze-test` passes
- [ ] Feature code integrated
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)
- [ ] Buffer-first compliance verified
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
- [ ] Write learned summary to `plan/learned/NNN-cmd-7-route-modify.md`
- [ ] Summary included in commit
