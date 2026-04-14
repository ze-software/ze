# Spec: cmd-3 -- Multipath / ECMP

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 3/3 |
| Updated | 2026-04-14 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `plan/spec-cmd-0-umbrella.md` -- umbrella context
3. `internal/component/bgp/schema/ze-bgp-conf.yang` -- bgp config structure
4. `internal/component/bgp/plugins/rib/bestpath.go` -- best-path selection
5. `internal/component/bgp/plugins/rib/rib.go` -- RIB storage

## Task

Add `bgp/multipath` container with `maximum-paths` (uint16, 1-256, default 1) and
`relax-as-path` (boolean, default false) to YANG config. Extend RIB plugin best-path
selection to track N paths per prefix. Global config, not per-peer.

**Config syntax (editor):**

| Command | Purpose |
|---------|---------|
| `set bgp multipath maximum-paths 8` | Allow up to 8 equal-cost paths per prefix |
| `set bgp multipath relax-as-path` | Allow different AS-paths of same length as equal-cost |

**YANG location:** `bgp/multipath` container (new, top-level under bgp).

| Leaf | Type | Default | Notes |
|------|------|---------|-------|
| `maximum-paths` | uint16 | 1 | Range 1-256. Default 1 = current single best-path behavior |
| `relax-as-path` | boolean | false | When true, paths with different AS-paths but same length are equal-cost |

**Multipath selection rules:**
- After best-path selection, collect up to N paths that are equal-cost to the best
- Equal-cost: same local-preference, same AS-path length (or any length if relax), same origin, same MED
- maximum-paths=1 is identical to current single best-path behavior
- Multipath paths visible in `rib best` output

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` -- reactor, RIB plugin interaction
  -> Constraint: RIB is a plugin, not part of reactor core
- [ ] `.claude/patterns/config-option.md` -- YANG leaf + resolver + reactor wiring
  -> Constraint: follow the pattern for adding config knobs

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc4271.md` -- BGP-4 base: best-path decision process (Section 9.1.2)
  -> Constraint: multipath extends best-path, does not replace it

**Key insights:**
- Best-path selection is in the RIB plugin, not the reactor
- Multipath is a post-selection step: pick best, then find N-1 more equal-cost paths
- maximum-paths=1 must produce identical behavior to current code (no regression)
- relax-as-path only relaxes AS-path content comparison, not length

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/schema/ze-bgp-conf.yang` -- bgp top-level containers
- [ ] `internal/component/bgp/plugins/rib/bestpath.go` -- best-path selection algorithm
- [ ] `internal/component/bgp/plugins/rib/rib.go` -- RIB storage, route tracking
- [ ] `internal/component/bgp/config/resolve.go` -- ResolveBGPTree() config extraction
- [ ] `internal/component/bgp/plugins/rib/rib_pipeline.go` -- rib show pipeline

**Behavior to preserve:**
- Current single best-path selection logic (RFC 4271 Section 9.1.2 decision steps)
- RIB storage and retrieval patterns
- `rib best` output format for single best-path
- All existing config files parse and work identically (no multipath = maximum-paths 1)

**Behavior to change:**
- New YANG container `bgp/multipath` with `maximum-paths` and `relax-as-path` leaves
- Best-path selection extended to track N equal-cost paths when maximum-paths > 1
- `rib best` output shows multiple paths when multipath is active
- RIB storage tracks multipath set per prefix

## Data Flow (MANDATORY)

### Entry Point
- Config: `bgp { multipath { maximum-paths 8; relax-as-path; } }` parsed from YANG
- RIB: best-path selection invoked when routes change for a prefix

### Transformation Path
1. Config parse: YANG leaves extracted by `ResolveBGPTree()`
2. RIB plugin initialization: multipath config passed to RIB plugin at startup
3. Route insertion: new route added to RIB for a prefix
4. Best-path selection: standard RFC 4271 Section 9.1.2 decision process picks single best
5. Multipath extension: if maximum-paths > 1, scan remaining paths for equal-cost matches
6. Multipath set: up to N paths stored as the multipath set for the prefix
7. Query: `rib best` pipeline returns multipath set when available

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| Config -> RIB Plugin | ResolveBGPTree() extracts multipath config, passed to RIB at init | [ ] |
| RIB Plugin -> CLI | rib best pipeline yields multipath set | [ ] |

### Integration Points
- `ResolveBGPTree()` -- extract multipath container leaves
- `bestpath.go` -- extend selection to collect N equal-cost paths
- `rib.go` -- store multipath set per prefix
- `rib_pipeline.go` -- `rib best` terminal shows multipath paths

### Architectural Verification
- [ ] No bypassed layers (config -> resolver -> RIB plugin)
- [ ] No unintended coupling (multipath logic stays in RIB plugin, not reactor)
- [ ] No duplicated functionality (extends existing best-path, does not replace)
- [ ] Zero-copy preserved where applicable

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Config with `bgp multipath maximum-paths 4` | → | RIB selects up to 4 equal-cost paths | `test/plugin/multipath-basic.ci` |
| Config with `bgp multipath relax-as-path` | → | Paths with different AS-paths of same length selected | `test/plugin/multipath-basic.ci` |
| Default config (no multipath) | → | Single best-path selected (current behavior) | `test/plugin/multipath-basic.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | maximum-paths=1 (default, no config) | Single best-path selected, identical to current behavior |
| AC-2 | maximum-paths=4, 4 equal-cost paths available | All 4 paths selected in multipath set |
| AC-3 | maximum-paths=4, only 2 equal-cost paths available | 2 paths selected (multipath set size = available, not configured max) |
| AC-4 | relax-as-path=false, paths with different AS-paths | Only paths with identical AS-path content are equal-cost |
| AC-5 | relax-as-path=true, paths with different AS-paths of same length | All paths with same AS-path length are equal-cost |
| AC-6 | `rib best` with multipath active | Shows all paths in multipath set with indication of best + multipath |
| AC-7 | No multipath config in existing deployments | Behavior identical to current Ze |
| AC-8 | maximum-paths boundary: value 1 (minimum valid) | Accepted, single best-path behavior |
| AC-9 | maximum-paths boundary: value 256 (maximum valid) | Accepted |
| AC-10 | maximum-paths boundary: value 0 (invalid below) | Rejected by YANG validation |
| AC-11 | maximum-paths boundary: value 257 (invalid above) | Rejected by YANG validation |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestMultipathDefault` | `bestpath_test.go` | maximum-paths=1 selects single best | |
| `TestMultipathFourPaths` | `bestpath_test.go` | maximum-paths=4 selects 4 equal-cost paths | |
| `TestMultipathFewerAvailable` | `bestpath_test.go` | maximum-paths=4 but only 2 equal-cost available | |
| `TestMultipathRelaxDisabled` | `bestpath_test.go` | Different AS-path content not equal-cost | |
| `TestMultipathRelaxEnabled` | `bestpath_test.go` | Same AS-path length counts as equal-cost | |
| `TestMultipathEqualCostCriteria` | `bestpath_test.go` | Same LP, AS-path len, origin, MED required | |

### Boundary Tests (MANDATORY for numeric inputs)

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| maximum-paths | 1-256 | 256 | 0 | 257 |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `multipath-basic` | `test/plugin/multipath-basic.ci` | Config with multipath, verify multiple paths selected in rib best | |

## Files to Modify

- `internal/component/bgp/schema/ze-bgp-conf.yang` -- add bgp/multipath container with maximum-paths and relax-as-path
- `internal/component/bgp/config/resolve.go` -- extract multipath config from tree
- `internal/component/bgp/plugins/rib/bestpath.go` -- extend best-path to collect N equal-cost paths
- `internal/component/bgp/plugins/rib/rib.go` -- store multipath set per prefix
- `internal/component/bgp/plugins/rib/rib_pipeline.go` -- rib best terminal shows multipath

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new container) | [x] | `internal/component/bgp/schema/ze-bgp-conf.yang` |
| CLI commands/flags | [ ] | YANG-driven (automatic) |
| Functional test for new feature | [x] | `test/plugin/multipath-basic.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [x] | `docs/features.md` -- add multipath/ECMP |
| 2 | Config syntax changed? | [x] | `docs/guide/configuration.md` -- multipath config examples |
| 3 | CLI command added/changed? | [ ] | N/A |
| 4 | API/RPC added/changed? | [ ] | N/A |
| 5 | Plugin added/changed? | [ ] | N/A (extends existing RIB plugin) |
| 6 | Has a user guide page? | [ ] | N/A |
| 7 | Wire format changed? | [ ] | N/A |
| 8 | Plugin SDK/protocol changed? | [ ] | N/A |
| 9 | RFC behavior implemented? | [x] | `rfc/short/rfc4271.md` -- multipath extends Section 9.1.2 |
| 10 | Test infrastructure changed? | [ ] | N/A |
| 11 | Affects daemon comparison? | [x] | `docs/comparison.md` -- multipath/ECMP now supported |
| 12 | Internal architecture changed? | [ ] | N/A |

## Files to Create

- `test/plugin/multipath-basic.ci` -- multipath functional test

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

1. **Phase: YANG + Config** -- Add multipath container to ze-bgp-conf.yang, extract in ResolveBGPTree()
   - Tests: `TestMultipathDefault`
   - Files: ze-bgp-conf.yang, resolve.go
2. **Phase: Best-Path Extension** -- Extend best-path selection to collect N equal-cost paths
   - Tests: `TestMultipathFourPaths`, `TestMultipathFewerAvailable`, `TestMultipathRelax*`, `TestMultipathEqualCostCriteria`
   - Files: bestpath.go, rib.go
3. **Phase: Pipeline Output** -- Update rib best to show multipath set
   - Tests: verify rib best output format
   - Files: rib_pipeline.go
4. **Functional tests** -- .ci tests proving end-to-end behavior
5. **Full verification** -- `make ze-verify`

### Critical Review Checklist (/implement stage 5)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | All 11 ACs demonstrated |
| Backward compat | No multipath config = identical behavior to current Ze |
| Equal-cost criteria | Same LP, AS-path length, origin, MED -- all four checked |
| Boundary | maximum-paths range enforced by YANG (1-256) |

### Deliverables Checklist (/implement stage 9)

| Deliverable | Verification method |
|-------------|---------------------|
| YANG container in ze-bgp-conf.yang | `grep multipath internal/component/bgp/schema/ze-bgp-conf.yang` |
| Best-path multipath logic | `grep -r multipath internal/component/bgp/plugins/rib/` |
| .ci functional test | `ls test/plugin/multipath-basic.ci` |

### Security Review Checklist (/implement stage 10)

| Check | What to look for |
|-------|-----------------|
| Input validation | maximum-paths range enforced by YANG; no unbounded allocation based on config value |

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

## RFC Documentation

Add `// RFC 4271 Section 9.1.2: "<requirement>"` above best-path and multipath selection code.

## Implementation Summary

### What Was Implemented

**Phase 1 (YANG + Config) -- DONE:**
- `bgp/multipath` container with `maximum-paths` (uint16, range 1-256, default 1) and `relax-as-path` (boolean, default false)
- Parse test: `test/parse/multipath-config.ci`

**Phase 2 (Config Delivery + Best-Path Extension) -- DONE:**
Found already implemented during 2026-04-14 audit:
- `extractMultipathConfig()` in `rib_multipath_config.go`: extracts `bgp/multipath` from JSON, validates bounds (1-256)
- RIBManager wires it in Stage 2 `OnConfigure`: stores in `maximumPaths` and `relaxASPath` atomic fields
- `SelectMultipath()` in `bestpath.go`: post-selection equal-cost multipath (RFC 4271 extension)
- `multipathEqual()`: gates on LOCAL_PREF, AS_PATH length/content, Origin, MED (same neighbor), eBGP/iBGP
- 6 config extraction unit tests in `rib_multipath_config_test.go`
- 12 best-path selection unit tests in `bestpath_test.go` (lines 838-1064)

**Phase 3 (Pipeline Output) -- DONE:**
Found already implemented during 2026-04-14 audit:
- `rib_pipeline_best.go`: loads `multipathMax` atomic, calls `SelectMultipath()`, populates `MultipathPeers`
- `RouteItem.MultipathPeers []string` field, marshaled as `"multipath-peers"` (omitempty)
- 2 pipeline output tests in `rib_pipeline_best_test.go`
- Functional test: `test/plugin/multipath-basic.ci` (added 2026-04-14)

### What Remains

| Item | Effort | Design needed |
|------|--------|---------------|
| (none -- all phases implemented) | - | - |

### Bugs Found/Fixed
- None

### Documentation Updates
- Not yet

### Deviations from Plan
- ~~Only YANG schema implemented; RIB algorithm change deferred pending design~~ -- all phases found implemented during 2026-04-14 audit. Spec was stale.

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
- [ ] AC-1..AC-11 all demonstrated
- [ ] Wiring Test table complete
- [ ] `make ze-test` passes
- [ ] Feature code integrated
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)
- [ ] RFC constraint comments added
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
- [ ] Write learned summary to `plan/learned/NNN-cmd-3-multipath.md`
- [ ] Summary included in commit
