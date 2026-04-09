# Spec: policy-4-redistribute

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
3. `docs/architecture/config/syntax.md` - ze:hidden documentation
4. `internal/component/config/validators.go` - existing custom validators
5. `internal/component/config/validators_register.go` - validator registration
6. `internal/component/bgp/schema/ze-bgp-conf.yang` - BGP YANG schema

## Task

Add route redistribution support to the policy framework. This spec covers the `redistribute { import/export }` YANG container at peer level, a Go-level redistribute source registry where protocols register route source names, the `ze:validate "redistribute-source"` custom validator for autocomplete and validation, and BGP's registration of `ibgp`/`ebgp` as redistribution sources.

The redistribute container uses `ze:hidden` (implemented in spec-policy-1-framework) so it is schema-present for validation and autocomplete but hidden from config display. It becomes visible when redistribution is fully implemented.

This is a child spec of `spec-policy-0-umbrella.md`. See umbrella Design Decisions 9-11 for resolved design choices.

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] -- checkboxes are template markers, not progress trackers. -->
<!-- Capture insights as -> Decision: / -> Constraint: annotations -- these survive compaction. -->
- [ ] `docs/architecture/config/syntax.md` - ze:hidden extension behavior, config display rules
  -> Decision: TBD
  -> Constraint: TBD
- [ ] `docs/architecture/core-design.md` - registration pattern, plugin architecture
  -> Decision: TBD
  -> Constraint: TBD

### Source Files
- [ ] `internal/component/config/validators.go` - existing custom validators (registered-families pattern)
  -> Decision: TBD
  -> Constraint: TBD
- [ ] `internal/component/config/validators_register.go` - validator registration via `RegisterValidators`
  -> Decision: TBD
  -> Constraint: TBD
- [ ] `internal/component/bgp/schema/ze-bgp-conf.yang` - BGP YANG schema, peer-level containers
  -> Decision: TBD
  -> Constraint: TBD
- [ ] `internal/component/config/yang/modules/ze-extensions.yang` - ze:hidden extension definition
  -> Decision: TBD
  -> Constraint: TBD

**Key insights:** TBD after reading.

## Current Behavior (MANDATORY)

**Source files read:** TBD -- must read before implementation.
- [ ] `internal/component/config/validators.go` - custom validator functions
- [ ] `internal/component/config/validators_register.go` - registration into ValidatorRegistry
- [ ] `internal/component/bgp/schema/ze-bgp-conf.yang` - current peer-level YANG containers
- [ ] `internal/component/config/yang/modules/ze-extensions.yang` - ze:hidden extension

**Behavior to preserve:**
- TBD

**Behavior to change:**
- TBD

## Data Flow (MANDATORY)

### Entry Point
- Config file: `bgp { peer <name> { redistribute { import [ ibgp ]; export [ ebgp ]; } } }`
- YANG schema validates leaf-list values via `ze:validate "redistribute-source"`

### Transformation Path
1. YANG parse -- redistribute container parsed, `ze:hidden` suppresses display
2. Custom validator -- leaf-list values checked against redistribute source registry
3. Autocomplete -- `CompleteFn` queries registry `Names()` for available sources

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| BGP plugin -> Source registry | `init()` registration of `ibgp`/`ebgp` | [ ] |
| Config parse -> Validator | `ze:validate "redistribute-source"` triggers `ValidateFn` | [ ] |
| Editor -> Autocomplete | `CompleteFn` returns registered source names | [ ] |

### Integration Points
- `yang.ValidatorRegistry` - existing registry for custom validators
- `RegisterValidators()` - existing function where new validator is registered
- `ze:hidden` - implemented in spec-policy-1-framework, used on redistribute container

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Config with `redistribute { import [ ibgp ] }` | -> | YANG validation + source registry lookup | `test/parse/redistribute-source-validation.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `redistribute { import [ ibgp ] }` in peer config | Parses successfully, validated against source registry |
| AC-2 | `redistribute { import [ unknown-source ] }` in peer config | Parse-time validation error naming the invalid source |
| AC-3 | `redistribute { export [ ebgp ] }` in peer config | Parses successfully |
| AC-4 | Tab completion on redistribute import value | Shows `ibgp`, `ebgp` as completions |
| AC-5 | `show config` with redistribute configured | Redistribute container hidden from display (ze:hidden) |
| AC-6 | Config file saved with redistribute values | Values persisted to file despite being hidden from display |
| AC-7 | BGP source registry at startup | Contains exactly `ibgp` and `ebgp` |
| AC-8 | Source registry `Names()` call | Returns sorted list of registered source names |
| AC-9 | Source registry `Lookup()` with valid name | Returns true |
| AC-10 | Source registry `Lookup()` with invalid name | Returns false |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRegistryRegister` | `internal/component/bgp/redistribute/registry_test.go` | Source registration and lookup | |
| `TestRegistryNames` | `internal/component/bgp/redistribute/registry_test.go` | Sorted name listing | |
| `TestRegistryLookup` | `internal/component/bgp/redistribute/registry_test.go` | Valid and invalid lookups | |
| `TestRegistryDuplicateRegister` | `internal/component/bgp/redistribute/registry_test.go` | Duplicate registration behavior | |
| `TestRedistributeSourceValidator` | `internal/component/config/validators_test.go` | Validator accepts registered sources, rejects unknown | |
| `TestRedistributeSourceComplete` | `internal/component/config/validators_test.go` | CompleteFn returns registered names | |

### Boundary Tests (MANDATORY for numeric inputs)
No numeric inputs in this spec.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `redistribute-source-validation` | `test/parse/redistribute-source-validation.ci` | Config with valid redistribute sources parses successfully | |

### Future (if deferring any tests)
- None planned

## Files to Modify

- `internal/component/bgp/schema/ze-bgp-conf.yang` - add `redistribute` container with `ze:hidden` at peer level, `import` and `export` `ordered-by user` leaf-lists with `ze:validate "redistribute-source"`
- `internal/component/config/validators.go` - add `RedistributeSourceValidator()` function following `registered-families` pattern
- `internal/component/config/validators_register.go` - register `"redistribute-source"` validator

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | [ ] | `internal/component/bgp/schema/ze-bgp-conf.yang` |
| CLI commands/flags | [ ] | N/A -- no new CLI commands |
| Editor autocomplete | [ ] | YANG-driven via `ze:validate` CompleteFn |
| Functional test for new RPC/API | [ ] | `test/parse/redistribute-source-validation.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] | TBD -- redistribute is hidden for now |
| 2 | Config syntax changed? | [ ] | `docs/architecture/config/syntax.md` -- ze:hidden usage on redistribute |
| 3 | CLI command added/changed? | [ ] | N/A |
| 4 | API/RPC added/changed? | [ ] | N/A |
| 5 | Plugin added/changed? | [ ] | N/A |
| 6 | Has a user guide page? | [ ] | TBD |
| 7 | Wire format changed? | [ ] | N/A |
| 8 | Plugin SDK/protocol changed? | [ ] | N/A |
| 9 | RFC behavior implemented? | [ ] | N/A |
| 10 | Test infrastructure changed? | [ ] | N/A |
| 11 | Affects daemon comparison? | [ ] | N/A |
| 12 | Internal architecture changed? | [ ] | N/A |

## Files to Create

- `internal/component/bgp/redistribute/registry.go` - source registry with `Register()`, `Names()`, `Lookup()` functions
- `internal/component/bgp/redistribute/registry_test.go` - registry unit tests
- `test/parse/redistribute-source-validation.ci` - parse test for validation

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

1. **Phase: Source registry** -- implement the redistribute source registry package
   - Tests: `TestRegistryRegister`, `TestRegistryNames`, `TestRegistryLookup`, `TestRegistryDuplicateRegister`
   - Files: `internal/component/bgp/redistribute/registry.go`, `registry_test.go`
   - Verify: tests fail -> implement -> tests pass
2. **Phase: BGP source registration** -- register `ibgp` and `ebgp` at init time
   - Tests: `TestRegistryNames` verifies both present after init
   - Files: BGP registration file (location TBD during research)
   - Verify: tests fail -> implement -> tests pass
3. **Phase: Custom validator** -- add `RedistributeSourceValidator()` and register it
   - Tests: `TestRedistributeSourceValidator`, `TestRedistributeSourceComplete`
   - Files: `internal/component/config/validators.go`, `validators_register.go`
   - Verify: tests fail -> implement -> tests pass
4. **Phase: YANG schema** -- add `redistribute` container with `ze:hidden` to BGP peer YANG
   - Tests: functional test validates config parsing
   - Files: `internal/component/bgp/schema/ze-bgp-conf.yang`
   - Verify: YANG compiles, config parses
5. **Functional tests** -- create after feature works, cover user-visible behavior
6. **Full verification** -- `make ze-verify`
7. **Complete spec** -- fill audit tables, write learned summary

### Critical Review Checklist (/implement stage 5)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1 through AC-10 has implementation with file:line |
| Correctness | Registry returns sorted names, validator error messages name the invalid source |
| Naming | Registry functions follow Go conventions, YANG uses kebab-case |
| Data flow | Validator queries registry at call time (not cached stale list) |
| Rule: no-layering | No remnants of old `<plugin>:<filter>` redistribution format |
| Rule: registered-families | Validator follows same pattern as `AddressFamilyValidator` |

### Deliverables Checklist (/implement stage 9)

| Deliverable | Verification method |
|-------------|---------------------|
| Source registry package | `ls internal/component/bgp/redistribute/registry.go` |
| Registry unit tests | `go test -run TestRegistry ./internal/component/bgp/redistribute/...` |
| Custom validator function | `grep RedistributeSourceValidator internal/component/config/validators.go` |
| Validator registration | `grep redistribute-source internal/component/config/validators_register.go` |
| YANG redistribute container | `grep redistribute internal/component/bgp/schema/ze-bgp-conf.yang` |
| Functional parse test | `ls test/parse/redistribute-source-validation.ci` |

### Security Review Checklist (/implement stage 10)

| Check | What to look for |
|-------|-----------------|
| Input validation | Validator rejects all non-registered source names |
| Registry safety | Registry is safe for concurrent reads after init-time registration |

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

N/A -- redistribute is a ze-internal config mechanism, not an RFC-defined protocol feature.

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
- [ ] AC-1..AC-10 all demonstrated
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes (all 6 checks in `rules/quality.md` -- no failures)

### Quality Gates (SHOULD pass -- defer with user approval)
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
- [ ] Write learned summary to `plan/learned/NNN-policy-4-redistribute.md`
- [ ] **Summary included in commit** -- NEVER commit implementation without the completed summary. One commit = code + tests + summary.
