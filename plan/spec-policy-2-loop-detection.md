# Spec: policy-2-loop-detection

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | spec-policy-1-framework |
| Phase | - |
| Updated | 2026-04-08 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `plan/spec-policy-0-umbrella.md` - parent umbrella spec, design decisions
4. `internal/component/bgp/reactor/filter/loop.go` - LoopIngress function
5. `internal/component/bgp/reactor/peersettings.go` - PeerSettings struct
6. `internal/component/plugin/registry/registry_bgp_filter.go` - PeerFilterInfo struct

## Task

Implement the loop-detection filter type as a child of the policy framework (spec-policy-0-umbrella). This filter is a facade over the existing in-process `LoopIngress` wire-bytes filter. It provides named configuration under `bgp { policy { loop-detection <name> { ... } } }` with two settings: `allow-own-as` (how many occurrences of the local ASN in AS_PATH before rejecting) and `cluster-id` (override router-id for CLUSTER_LIST loop check). The in-process LoopIngress filter reads these settings from PeerFilterInfo at execution time. A default instance auto-populates in each peer's filter chain if the user does not define one.

This spec resolves three deferred items:
- spec-redistribution-filter AC-18: default named filter active by default
- spec-route-loop-detection: allow-own-as N configuration
- spec-route-loop-detection: explicit cluster-id configuration

Key constraint from umbrella Decision 5: the named filter is config only, NOT a replacement of LoopIngress. Zero-copy preserved. Settings flow via PeerFilterInfo.

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] -- checkboxes are template markers, not progress trackers. -->
<!-- Capture insights as -> Decision: / -> Constraint: annotations -- these survive compaction. -->
<!-- Track reading progress in session-state.md, not here. -->
- [ ] `docs/architecture/core-design.md` - filter pipeline architecture
  -> Decision: TBD
  -> Constraint: TBD
- [ ] `docs/architecture/config/syntax.md` - config parsing, YANG schema
  -> Decision: TBD
  -> Constraint: TBD

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc4271.md` - AS loop detection (Section 9)
  -> Constraint: TBD
- [ ] `rfc/short/rfc4456.md` - route reflector loop detection (Section 8)
  -> Constraint: TBD

**Key insights:** TBD

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE moving to design)
<!-- Same rule: never tick [ ] to [x]. Write -> Constraint: annotations instead. -->
- [ ] `internal/component/bgp/reactor/filter/loop.go` - LoopIngress filter, current AS loop and cluster-id loop detection
  -> Constraint: TBD
- [ ] `internal/component/bgp/reactor/filter/loop_test.go` - existing unit tests for loop detection
  -> Constraint: TBD
- [ ] `internal/component/bgp/reactor/filter/register.go` - loop filter registration as IngressFilterFunc
  -> Constraint: TBD
- [ ] `internal/component/bgp/reactor/peersettings.go` - PeerSettings struct, current fields
  -> Constraint: TBD
- [ ] `internal/component/plugin/registry/registry_bgp_filter.go` - PeerFilterInfo struct, fields available to filters
  -> Constraint: TBD
- [ ] `internal/component/bgp/config/peers.go` - PeersFromConfigTree, filter chain assembly
  -> Constraint: TBD

**Behavior to preserve:**
- LoopIngress stays as a wire-bytes IngressFilterFunc (zero-copy)
- Existing loop detection behavior when allow-own-as is 0 (default)
- Existing cluster-id loop detection using router-id when cluster-id not configured
- In-process filter execution path unchanged

**Behavior to change:**
- LoopIngress reads allow-own-as count from PeerFilterInfo instead of rejecting on first occurrence
- LoopIngress reads cluster-id from PeerFilterInfo when set, instead of always using router-id
- Loop detection settings configurable per-peer via named filter instances
- Default filter instance auto-populates in peer filter chains

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point
- Config: `bgp { policy { loop-detection <name> { allow-own-as N; cluster-id X; } } }` defines filter instances
- Config: per-peer filter chain references the named instance (or default auto-populates)

### Transformation Path
1. YANG validation -- loop-detection list parsed, settings validated (allow-own-as 0-10, cluster-id ipv4)
2. Config tree -- named instances stored in policy section
3. `PeersFromConfigTree` -- default instance created if none defined, settings resolved to PeerSettings
4. Reactor startup -- PeerFilterInfo populated with allow-own-as and cluster-id from PeerSettings
5. Wire processing -- LoopIngress reads PeerFilterInfo settings at filter execution time

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config -> PeerSettings | allow-own-as and cluster-id extracted during config parse | [ ] |
| PeerSettings -> PeerFilterInfo | Fields copied into PeerFilterInfo at peer setup | [ ] |
| PeerFilterInfo -> LoopIngress | Filter reads per-peer settings from PeerFilterInfo | [ ] |

### Integration Points
- `PeersFromConfigTree` -- assembles filter chain, auto-populates default loop-detection
- `PeerSettings` -- carries loop-detection config fields
- `PeerFilterInfo` -- delivers settings to in-process filters
- `LoopIngress` -- reads allow-own-as count and cluster-id from PeerFilterInfo

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (LoopIngress stays on wire bytes)

## Wiring Test (MANDATORY -- NOT deferrable)

<!-- BLOCKING: Proves the feature is reachable from its intended entry point. -->
| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Config with default loop-detection (no user config) | -> | Auto-populated filter in chain, LoopIngress runs | `test/plugin/filter-loop-detection-default.ci` |
| Config with `allow-own-as 2` | -> | LoopIngress allows 2 occurrences of local ASN | `test/plugin/filter-loop-detection-allow-own-as.ci` |

## Acceptance Criteria

<!-- Define BEFORE implementation. Each row is a testable assertion. -->
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | YANG list `loop-detection` under `bgp/policy` with `ze:filter` extension | YANG validates, autocomplete works for `loop-detection` and its leaves |
| AC-2 | `allow-own-as` leaf with type uint8, range 0-10, default 0 | Values outside 0-10 rejected at parse time |
| AC-3 | `cluster-id` leaf with type ipv4-address, optional | Valid IPv4 accepted, invalid rejected at parse time |
| AC-4 | No user-defined loop-detection instance | Default instance (e.g., `no-self-as`) auto-created in each peer's filter chain |
| AC-5 | Default instance in filter chain | Visible in `show config` output |
| AC-6 | `allow-own-as 0` (default) | LoopIngress rejects UPDATE containing local ASN in AS_PATH (first occurrence) |
| AC-7 | `allow-own-as 2` configured | LoopIngress allows up to 2 occurrences of local ASN in AS_PATH, rejects on 3rd |
| AC-8 | No `cluster-id` configured | LoopIngress uses router-id for CLUSTER_LIST loop check (current behavior) |
| AC-9 | `cluster-id 10.0.0.1` configured | LoopIngress uses 10.0.0.1 instead of router-id for CLUSTER_LIST loop check |
| AC-10 | PeerFilterInfo populated with allow-own-as and cluster-id | LoopIngress reads per-peer settings, not global defaults |
| AC-11 | Config with loop-detection settings | Settings flow through PeerSettings into PeerFilterInfo correctly |
| AC-12 | `delete` on default loop-detection filter | Sets `inactive:` prefix, loop detection skipped for that peer (umbrella Decision 4) |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestLoopIngressAllowOwnAsZero` | `internal/component/bgp/reactor/filter/loop_test.go` | Rejects on first local ASN occurrence (default) | |
| `TestLoopIngressAllowOwnAsN` | `internal/component/bgp/reactor/filter/loop_test.go` | Allows N occurrences, rejects on N+1 | |
| `TestLoopIngressClusterIdOverride` | `internal/component/bgp/reactor/filter/loop_test.go` | Uses configured cluster-id instead of router-id | |
| `TestLoopIngressClusterIdDefault` | `internal/component/bgp/reactor/filter/loop_test.go` | Uses router-id when cluster-id not set | |
| `TestLoopDetectionConfigExtract` | `internal/component/bgp/reactor/filter/loop_config_test.go` | Extracts allow-own-as and cluster-id from config tree | |
| `TestLoopDetectionDefaultAutoPopulate` | `internal/component/bgp/config/peers_test.go` | Default instance created when none defined | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `allow-own-as` | 0-10 | 10 | N/A (uint8, 0 is valid) | 11 |

### Functional Tests
<!-- REQUIRED: Verify feature works from end-user perspective -->
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `filter-loop-detection-default` | `test/plugin/filter-loop-detection-default.ci` | Default loop detection active, rejects own-AS route | |
| `filter-loop-detection-allow-own-as` | `test/plugin/filter-loop-detection-allow-own-as.ci` | allow-own-as 2 permits route with 2 occurrences of local ASN | |

### Future (if deferring any tests)
- None planned

## Files to Modify
<!-- MUST include feature code (internal/*, cmd/*), not only test files -->
- `internal/component/bgp/reactor/filter/loop.go` - add allow-own-as count logic, cluster-id override logic
- `internal/component/bgp/reactor/filter/register.go` - register loop-detection as a named filter type
- `internal/component/bgp/reactor/peersettings.go` - add AllowOwnAS and ClusterID fields
- `internal/component/plugin/registry/registry_bgp_filter.go` - add AllowOwnAS and ClusterID to PeerFilterInfo
- `internal/component/bgp/reactor/reactor_notify.go` - populate new PeerFilterInfo fields from PeerSettings
- `internal/component/bgp/config/peers.go` - extract loop-detection config, auto-populate default instance

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new filter type) | [ ] | `internal/component/bgp/reactor/filter/schema/ze-loop-detection.yang` |
| CLI commands/flags | [ ] | YANG-driven (automatic if YANG updated) |
| Editor autocomplete | [ ] | YANG-driven (automatic if YANG updated) |
| Functional test for filter behavior | [ ] | `test/plugin/filter-loop-detection-default.ci`, `test/plugin/filter-loop-detection-allow-own-as.ci` |

### Documentation Update Checklist (BLOCKING)
<!-- Every row MUST be answered Yes/No during the Completion Checklist (planning.md step 1). -->
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] | TBD |
| 2 | Config syntax changed? | [ ] | TBD |
| 3 | CLI command added/changed? | [ ] | N/A |
| 4 | API/RPC added/changed? | [ ] | N/A |
| 5 | Plugin added/changed? | [ ] | TBD |
| 6 | Has a user guide page? | [ ] | TBD |
| 7 | Wire format changed? | [ ] | N/A |
| 8 | Plugin SDK/protocol changed? | [ ] | N/A |
| 9 | RFC behavior implemented? | [ ] | TBD |
| 10 | Test infrastructure changed? | [ ] | N/A |
| 11 | Affects daemon comparison? | [ ] | TBD |
| 12 | Internal architecture changed? | [ ] | N/A |

## Files to Create
- `internal/component/bgp/reactor/filter/schema/ze-loop-detection.yang` - YANG list with `ze:filter` extension, `allow-own-as` and `cluster-id` leaves
- `internal/component/bgp/reactor/filter/schema/embed.go` - go:embed for YANG file
- `internal/component/bgp/reactor/filter/schema/register.go` - yang.RegisterModule in init()
- `internal/component/bgp/reactor/filter/loop_config.go` - config extraction for loop-detection settings
- `test/plugin/filter-loop-detection-default.ci` - functional test: default loop detection active
- `test/plugin/filter-loop-detection-allow-own-as.ci` - functional test: allow-own-as behavior

## Implementation Steps

<!-- Steps must map to /implement stages. -->

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan -- check what exists |
| 3. Implement (TDD) | Implementation phases below (write-test-fail-implement-pass per phase) |
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

1. **Phase: YANG schema** -- define loop-detection filter type in YANG
   - Tests: YANG validation tests
   - Files: `ze-loop-detection.yang`, `embed.go`, `register.go`
   - Verify: tests fail -> implement -> tests pass
2. **Phase: PeerSettings and PeerFilterInfo** -- add allow-own-as and cluster-id fields
   - Tests: `TestLoopDetectionConfigExtract`
   - Files: `peersettings.go`, `registry_bgp_filter.go`, `reactor_notify.go`
   - Verify: tests fail -> implement -> tests pass
3. **Phase: LoopIngress enhancement** -- implement allow-own-as count and cluster-id override
   - Tests: `TestLoopIngressAllowOwnAsZero`, `TestLoopIngressAllowOwnAsN`, `TestLoopIngressClusterIdOverride`, `TestLoopIngressClusterIdDefault`
   - Files: `loop.go`
   - Verify: tests fail -> implement -> tests pass
4. **Phase: Config parsing and auto-population** -- extract settings, auto-populate default
   - Tests: `TestLoopDetectionDefaultAutoPopulate`
   - Files: `peers.go`, `loop_config.go`
   - Verify: tests fail -> implement -> tests pass
5. **Functional tests** -- create after feature works, cover user-visible behavior
6. **RFC refs** -- add `// RFC 4271 Section 9` and `// RFC 4456 Section 8` comments
7. **Full verification** -- `make ze-verify`
8. **Complete spec** -- fill audit tables, write learned summary to `plan/learned/NNN-policy-2-loop-detection.md`, delete spec from `plan/`

### Critical Review Checklist (/implement stage 5)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1 through AC-12 has implementation with file:line |
| Correctness | allow-own-as count logic correct (N occurrences allowed, N+1 rejected), cluster-id override only when set |
| Naming | YANG uses kebab-case (`allow-own-as`, `cluster-id`), Go uses CamelCase (`AllowOwnAS`, `ClusterID`) |
| Data flow | Settings flow Config -> PeerSettings -> PeerFilterInfo -> LoopIngress, no shortcuts |
| Rule: no-layering | No parallel loop detection path created, facade only |
| Rule: zero-copy | LoopIngress still operates on wire bytes, no new allocations in hot path |

### Deliverables Checklist (/implement stage 9)

| Deliverable | Verification method |
|-------------|---------------------|
| YANG file `ze-loop-detection.yang` exists | `ls internal/component/bgp/reactor/filter/schema/ze-loop-detection.yang` |
| YANG registered via init() | `grep RegisterModule internal/component/bgp/reactor/filter/schema/register.go` |
| PeerSettings has AllowOwnAS and ClusterID fields | `grep AllowOwnAS internal/component/bgp/reactor/peersettings.go` |
| PeerFilterInfo has matching fields | `grep AllowOwnAS internal/component/plugin/registry/registry_bgp_filter.go` |
| LoopIngress reads allow-own-as from PeerFilterInfo | `grep AllowOwnAS internal/component/bgp/reactor/filter/loop.go` |
| LoopIngress reads cluster-id from PeerFilterInfo | `grep ClusterID internal/component/bgp/reactor/filter/loop.go` |
| Default auto-population in config | `grep -r "no-self-as\|auto.popul\|default.*loop" internal/component/bgp/config/peers.go` |
| Functional test: default | `ls test/plugin/filter-loop-detection-default.ci` |
| Functional test: allow-own-as | `ls test/plugin/filter-loop-detection-allow-own-as.ci` |

### Security Review Checklist (/implement stage 10)

| Check | What to look for |
|-------|-----------------|
| Input validation | allow-own-as bounded to 0-10 in YANG, verify Go code also checks (defense in depth) |
| Input validation | cluster-id must be valid IPv4 address, validated by YANG type |
| Resource exhaustion | allow-own-as max 10, no unbounded iteration risk |

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
<!-- LIVE -- write IMMEDIATELY when you learn something -->

## RFC Documentation

Add `// RFC 4271 Section 9` above AS_PATH loop detection code.
Add `// RFC 4456 Section 8` above CLUSTER_LIST loop detection code.
MUST document: validation rules, error conditions.

## Implementation Summary

### What Was Implemented
- TBD

### Bugs Found/Fixed
- TBD

### Documentation Updates
- TBD

### Deviations from Plan
- TBD

## Implementation Audit

<!-- BLOCKING: Complete BEFORE writing learned summary. See rules/implementation-audit.md -->

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

<!-- BLOCKING: Do NOT trust the audit above. Re-verify everything independently. -->

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
- [ ] AC-1 through AC-12 all demonstrated
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`)
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
- [ ] Write learned summary to `plan/learned/NNN-policy-2-loop-detection.md`
- [ ] **Summary included in commit** -- NEVER commit implementation without the completed summary. One commit = code + tests + summary.
