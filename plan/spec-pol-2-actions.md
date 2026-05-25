# Spec: Policy Action Macros

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 1/3 |
| Updated | 2026-05-25 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md`
3. `internal/component/bgp/plugins/filter_modify/` - existing modify plugin
4. `internal/component/bgp/reactor/filter_delta.go` - text delta to wire ops
5. `internal/component/bgp/plugins/filter_community/handler.go` - community AttrModHandlers

## Task

Extend Ze's policy action vocabulary with three features:
1. **as-path-length filter** -- accept/reject based on AS_PATH hop count
2. **modify inc/dec** -- increment/decrement integer attributes (local-pref, med, aigp)
3. **community add/remove in modify** -- add/remove individual community values (standard, large, extended) via the policy chain

Parent: `spec-pol-0-umbrella.md`

Note: remove-private-as was already implemented (filter_remove_private_as/ plugin + ExtractRemovePrivateASOps engine support).

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - policy filter chain
  -> Constraint: filters piped, text format, delta modify
- [ ] `ai/patterns/registration.md` - plugin init/registry/blank-import
  -> Constraint: register via init() in register.go with FilterTypes

### Source Files
- [ ] `internal/component/bgp/plugins/filter_modify/` - existing modify plugin
  -> Constraint: pre-builds delta at config time, returns it unchanged at runtime
  -> Constraint: handleFilterUpdate ignores incoming update text (just returns pre-built delta)
- [ ] `internal/component/bgp/reactor/filter_delta.go` - textDeltaToModOps
  -> Constraint: only emits AttrModSet; community-add/remove needs AttrModAdd/AttrModRemove
  -> Constraint: skips AS_PATH and NLRI in delta processing
- [ ] `internal/component/bgp/plugins/filter_community/handler.go` - genericCommunityHandler
  -> Constraint: already handles AttrModAdd, AttrModRemove, AttrModSet for all 3 community types
  -> Constraint: remove-before-add ordering within handler
- [ ] `internal/component/bgp/attribute/aspath.go:209` - PathLength() method exists
  -> Constraint: counts AS_SEQUENCE + AS_SET (AS_SET = 1), skips CONFED segments per RFC

**Key insights:**
- The modify plugin pre-builds deltas at config time. Inc/dec requires reading the current value from the update text at runtime, so handleFilterUpdate needs to become dynamic.
- Community add/remove needs new text directives (community-add, community-remove, etc.) recognized by isPolicyAttrName, parseFilterAttrs, and textDeltaToModOps.
- The community handlers already support Add/Remove ops. Only the text-delta-to-wire path is missing.
- attribute.ASPath.PathLength() already exists and handles confederation segments correctly.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/filter_modify/filter_modify.go` (101 lines) - RunFilterModify, handleFilterUpdate returns pre-built delta
  -> Constraint: handleFilterUpdate ignores in.Update entirely
- [ ] `internal/component/bgp/plugins/filter_modify/modify.go` (88 lines) - buildDelta constructs text from set block leaves
  -> Constraint: supports local-preference, med, origin, next-hop, as-path-prepend
- [ ] `internal/component/bgp/plugins/filter_modify/config.go` (59 lines) - parseModifyDefs walks bgp/policy/modify
  -> Constraint: requires 'set' container, rejects empty set
- [ ] `internal/component/bgp/plugins/filter_modify/schema/ze-filter-modify.yang` - YANG with set container
  -> Constraint: set has leaves: local-preference, med, origin, next-hop, as-path-prepend
- [ ] `internal/component/bgp/reactor/filter_chain.go` - isPolicyAttrName, parseFilterAttrs, formatFilterAttrs
  -> Constraint: 13 known attr names + as-path-prepend + remove-private
- [ ] `internal/component/bgp/reactor/filter_delta.go` - textDeltaToModOps
  -> Constraint: iterates modAttrs, skips nlri/as-path/as-path-prepend/remove-private, emits AttrModSet
- [ ] `internal/component/bgp/plugins/filter_community/handler.go` (131 lines) - genericCommunityHandler
  -> Constraint: processes ops in order: Remove first, Add second, Set last
- [ ] `internal/component/bgp/attribute/aspath.go` - ParseASPath, ASPath.PathLength()
  -> Constraint: PathLength counts AS_SEQUENCE entries + 1 per AS_SET, skips CONFED

**Behavior to preserve:**
- Existing modify filter config syntax and behavior unchanged
- Existing filter chain text format unchanged for current attributes
- Community handler Remove-Add-Set ordering preserved
- as-path-prepend via modify continues working
- remove-private via modify continues working

**Behavior to change:**
- modify handleFilterUpdate becomes dynamic (reads current values from update text for inc/dec)
- New text directives: community-add, community-remove, large-community-add, large-community-remove, extended-community-add, extended-community-remove
- textDeltaToModOps recognizes new directives and emits AttrModAdd/AttrModRemove
- New as-path-length filter plugin

## Data Flow (MANDATORY)

### Entry Point
- Config: YANG `bgp/policy/modify` (extended) and `bgp/policy/as-path-length` (new)
- Runtime: UPDATE text arrives via filter-update RPC

### Transformation Path

**Inc/dec flow:**
1. Modify plugin receives filter-update with update text containing current attribute values
2. Plugin extracts current value (e.g., "local-preference 100")
3. Plugin computes new value (100 + 50 = 150, saturating at uint32 bounds)
4. Plugin returns delta "local-preference 150" (absolute value)
5. Engine processes via existing textDeltaToModOps -> AttrModSet path (no engine changes)

**Community add/remove flow:**
1. Modify plugin returns delta containing new directives (e.g., "community-add 65000:200")
2. applyFilterDelta merges: community-add is a NEW key, doesn't conflict with existing community
3. textDeltaToModOps sees community-add, encodes value, emits AttrModAdd op
4. buildModifiedPayload calls communityAttrModHandler which already handles AttrModAdd

**as-path-length flow:**
1. Plugin receives filter-update with update text
2. Plugin extracts as-path field, counts hops
3. Plugin compares count against configured max/min
4. Plugin returns accept or reject (no modify)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config -> modify plugin | YANG tree via OnConfigure | [ ] |
| Config -> as-path-length plugin | YANG tree via OnConfigure | [ ] |
| Engine -> plugin (text) | filter-update RPC with text format update | [ ] |
| Plugin -> engine (delta) | FilterUpdateOutput with delta text | [ ] |
| Delta -> wire | textDeltaToModOps emits AttrModAdd/Remove on ModAccumulator | [ ] |
| Wire -> handler | communityAttrModHandler processes Add/Remove ops | [ ] |

### Integration Points
- `filter_chain.go:isPolicyAttrName` - add 6 new community directive names
- `filter_chain.go:parseFilterAttrs` - add new names to singleToken/multiToken maps
- `filter_chain.go:formatFilterAttrs` - add new names to output order
- `filter_delta.go:textDeltaToModOps` - handle new directives with AttrModAdd/Remove
- `filter_delta.go:encodeAttrValue` - reuse existing encode functions for community values

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| modify with increment config | -> | handleFilterUpdate reads current value | TestModifyIncrement |
| modify with community-add config | -> | handleFilterUpdate emits community-add directive | TestModifyCommunityAdd |
| as-path-length config in policy | -> | as-path-length handleFilterUpdate | TestAsPathLengthWiring |
| community-add in delta text | -> | textDeltaToModOps emits AttrModAdd | TestCommunityAddDeltaOp |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | modify with `increment { local-preference 50; }` on route with LP 100 | Delta contains local-preference 150 |
| AC-2 | modify with `decrement { med 30; }` on route with MED 20 | Delta contains med 0 (floor, not underflow) |
| AC-3 | modify with `increment { local-preference 50; }` on route with LP 4294967280 | Delta contains local-preference 4294967295 (saturate, not overflow) |
| AC-4 | modify with `community-add` leaf-list containing 65000:200 | Delta contains community-add 65000:200 |
| AC-5 | modify with `community-remove` leaf-list containing 65000:100 | Delta contains community-remove 65000:100 |
| AC-6 | modify with `large-community-add` containing 65000:100:200 | Delta contains large-community-add 65000:100:200 |
| AC-7 | textDeltaToModOps with community-add directive | ModAccumulator contains AttrModAdd op for COMMUNITY code |
| AC-8 | textDeltaToModOps with community-remove directive | ModAccumulator contains AttrModRemove op for COMMUNITY code |
| AC-9 | as-path-length filter with max 30, route has 35-hop path | FilterReject returned |
| AC-10 | as-path-length filter with max 30, route has 25-hop path | FilterAccept returned |
| AC-11 | as-path-length filter with min 2, route has 1-hop path | FilterReject returned |
| AC-12 | as-path-length filter with min 2, route has 3-hop path | FilterAccept returned |
| AC-13 | modify with both set and increment for same attr | Config parse error (mutually exclusive) |
| AC-14 | modify with community-add idempotent (value already present) | AttrModAdd op emitted; handler appends (dedup is handler responsibility, not plugin) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| TestBuildDynamicDelta | filter_modify/modify_test.go | inc/dec computes correct absolute value from current | |
| TestBuildDeltaCommunityAdd | filter_modify/modify_test.go | community-add directive in delta text | |
| TestBuildDeltaCommunityRemove | filter_modify/modify_test.go | community-remove directive in delta text | |
| TestBuildDeltaLargeCommunityAdd | filter_modify/modify_test.go | large-community-add directive in delta | |
| TestParseModifyDefsIncDec | filter_modify/modify_test.go | config parsing for increment/decrement | |
| TestParseModifyDefsCommAdd | filter_modify/modify_test.go | config parsing for community-add leaf-list | |
| TestHandleFilterUpdateIncrement | filter_modify/modify_test.go | runtime delta with incremented value | |
| TestHandleFilterUpdateCommunityAdd | filter_modify/modify_test.go | runtime delta with community-add | |
| TestCommunityAddDeltaToModOps | filter_delta_test.go | textDeltaToModOps emits AttrModAdd for community-add | |
| TestCommunityRemoveDeltaToModOps | filter_delta_test.go | textDeltaToModOps emits AttrModRemove for community-remove | |
| TestLargeCommunityAddDeltaToModOps | filter_delta_test.go | textDeltaToModOps emits AttrModAdd for large-community-add | |
| TestAsPathLengthAccept | filter_aspath_length/aspath_length_test.go | accept when path length within bounds | |
| TestAsPathLengthRejectMax | filter_aspath_length/aspath_length_test.go | reject when path exceeds max | |
| TestAsPathLengthRejectMin | filter_aspath_length/aspath_length_test.go | reject when path below min | |
| TestAsPathLengthConfig | filter_aspath_length/aspath_length_test.go | config parsing for max/min | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| increment local-preference | 1-4294967295 | 4294967295 | 0 | N/A (uint32) |
| decrement local-preference | 1-4294967295 | 4294967295 | 0 | N/A (uint32) |
| as-path-length max | 1-65535 | 65535 | 0 | 65536 |
| as-path-length min | 0-65535 | 65535 | N/A | 65536 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| modify-increment | test/plugin/modify-increment.ci | Peer sends route with LP 100, modify increments by 50, verify LP 150 in event | |
| modify-community-add | test/plugin/modify-community-add.ci | Peer sends route, modify adds community, verify community in event | |
| aspath-length-reject | test/plugin/aspath-length-reject.ci | Peer sends route with 35-hop path, as-path-length max=30, verify reject | |

### Interop Tests (MANDATORY for protocol features)
N/A -- these are policy actions, not wire protocol changes. The wire encoding is unchanged (uses existing AttrMod ops and handlers).

### Future (if deferring any tests)
- extended-community-add/remove functional tests (same mechanism as standard, lower priority)

## Files to Modify

- `internal/component/bgp/plugins/filter_modify/schema/ze-filter-modify.yang` - add increment, decrement containers and community leaf-lists
- `internal/component/bgp/plugins/filter_modify/modify.go` - extend buildDelta for dynamic computation and community directives
- `internal/component/bgp/plugins/filter_modify/config.go` - parse increment/decrement/community-add/remove config
- `internal/component/bgp/plugins/filter_modify/filter_modify.go` - handleFilterUpdate becomes dynamic (reads update text)
- `internal/component/bgp/plugins/filter_modify/modify_test.go` - tests for new features
- `internal/component/bgp/reactor/filter_chain.go` - add 6 community directive names to isPolicyAttrName, parseFilterAttrs, formatFilterAttrs
- `internal/component/bgp/reactor/filter_delta.go` - handle community-add/remove in textDeltaToModOps
- `internal/component/bgp/reactor/filter_delta_test.go` - tests for new delta ops

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | [x] | ze-filter-modify.yang (extend), ze-filter-aspath-length.yang (new) |
| CLI commands/flags | [ ] | N/A |
| CLI grammar (action before identifier) | [ ] | N/A |
| Editor autocomplete | [x] | YANG-driven (automatic) |
| Functional test for new RPC/API | [x] | test/plugin/*.ci |
| Doctor check for runtime dependencies | [ ] | N/A |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [x] | `docs/features.md` - policy action macros |
| 2 | Config syntax changed? | [x] | `docs/guide/configuration.md` - modify extensions |
| 3 | CLI command added/changed? | [ ] | N/A |
| 4 | API/RPC added/changed? | [ ] | N/A |
| 5 | Plugin added/changed? | [x] | `docs/guide/plugins.md` - modify extensions, as-path-length |
| 6 | Has a user guide page? | [ ] | Covered by plugins.md |
| 7 | Wire format changed? | [ ] | N/A |
| 8 | Plugin SDK/protocol changed? | [ ] | N/A |
| 9 | RFC behavior implemented? | [ ] | N/A |
| 10 | Test infrastructure changed? | [ ] | N/A |
| 11 | Affects daemon comparison? | [x] | `docs/comparison.md` - inc/dec, community add/remove, as-path-length |
| 12 | Internal architecture changed? | [ ] | N/A (extends existing pattern) |

## Files to Create

- `internal/component/bgp/plugins/filter_aspath_length/register.go` - plugin registration
- `internal/component/bgp/plugins/filter_aspath_length/filter_aspath_length.go` - plugin entry point
- `internal/component/bgp/plugins/filter_aspath_length/aspath_length.go` - path length evaluation
- `internal/component/bgp/plugins/filter_aspath_length/config.go` - config parsing
- `internal/component/bgp/plugins/filter_aspath_length/aspath_length_test.go` - unit tests
- `internal/component/bgp/plugins/filter_aspath_length/schema/ze-filter-aspath-length.yang` - YANG
- `internal/component/bgp/plugins/filter_aspath_length/schema/embed.go` - YANG embed
- `test/plugin/modify-increment.ci` - functional test
- `test/plugin/modify-community-add.ci` - functional test
- `test/plugin/aspath-length-reject.ci` - functional test

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Phases below |
| 5. /ze-review gate | Review Gate |
| 6. Full verification | make ze-lint, make ze-unit-test |
| 7. Critical review | Critical Review Checklist |

### Implementation Phases

1. **Phase: as-path-length plugin (MANDATORY FIRST -- standalone)** -- new filter plugin
   - Tests: TestAsPathLengthAccept, TestAsPathLengthRejectMax, TestAsPathLengthRejectMin, TestAsPathLengthConfig
   - Files: filter_aspath_length/ (all new files)
   - Verify: unit tests pass; plugin registers with FilterTypes ["as-path-length"]

2. **Phase: modify inc/dec** -- extend modify plugin with dynamic delta
   - Tests: TestBuildDynamicDelta, TestParseModifyDefsIncDec, TestHandleFilterUpdateIncrement
   - Files: modify.go, config.go, filter_modify.go, ze-filter-modify.yang, modify_test.go
   - Verify: unit tests pass; modify with increment produces correct absolute value

3. **Phase: community add/remove** -- extend modify plugin + engine text delta
   - Tests: TestBuildDeltaCommunityAdd, TestCommunityAddDeltaToModOps, TestCommunityRemoveDeltaToModOps
   - Files: modify.go, config.go, ze-filter-modify.yang, filter_chain.go, filter_delta.go, filter_delta_test.go
   - Verify: unit tests pass; community-add directive produces AttrModAdd op

### Critical Review Checklist (/implement stage 6)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Inc/dec saturates at uint32 bounds; community encode reuses existing functions |
| Naming | New YANG leaves follow existing kebab-case pattern in ze-filter-modify |
| Data flow | Inc/dec reads current from update text, returns absolute value in delta |
| Rule: no-sprintf | No fmt.Sprintf in delta building; use textbuf helpers |
| Rule: buffer-first | Wire encoding unchanged (uses existing handlers) |
| Mutual exclusion | set and increment for same attr rejected at parse time |

### Deliverables Checklist (/implement stage 10)

| Deliverable | Verification method |
|-------------|---------------------|
| as-path-length plugin registers | grep "as-path-length" in register.go |
| modify inc/dec produces correct values | go test -run TestBuildDynamicDelta |
| community-add emits AttrModAdd | go test -run TestCommunityAddDeltaToModOps |
| YANG validates | make ze-lint |
| All unit tests pass | make ze-unit-test |

### Security Review Checklist (/implement stage 11)

| Check | What to look for |
|-------|-----------------|
| Integer overflow | Inc/dec saturates at uint32 boundaries (0 floor, max ceil) |
| Input validation | as-path-length max/min validated in YANG range; community values validated by existing parsers |
| Resource exhaustion | community-add leaf-list unbounded; no risk (config-time, not per-packet) |
| Regex DoS | N/A (no regex in this spec) |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior |
| Lint failure | Fix inline |
| 3 fix attempts fail | STOP. Report. Ask user. |

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

## RFC Documentation

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.

## Implementation Summary

### What Was Implemented
- [Pending]

### Bugs Found/Fixed
- [Pending]

### Documentation Updates
- [Pending]

### Deviations from Plan
- [Pending]

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
| Inc/dec for integer attrs | unit test | TestBuildDynamicDelta, TestHandleFilterUpdateIncrement |
| Community add/remove in chain | unit test | TestCommunityAddDeltaToModOps |
| AS-path length filtering | unit test | TestAsPathLengthRejectMax |

## Review Gate

### Run 1 (initial)
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
- [ ] AC-1..AC-14 all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes
- [ ] Feature code integrated
- [ ] Critical Review passes

### Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL
- [ ] Tests PASS
- [ ] Boundary tests for numeric inputs
- [ ] Goal Validation table filled

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary
- [ ] Summary included in commit
