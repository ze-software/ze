# Spec: policy-1-framework

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | - |
| Updated | 2026-04-08 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `plan/spec-policy-0-umbrella.md` - parent spec with design decisions
4. `internal/component/config/yang/modules/ze-extensions.yang` - existing extensions
5. `internal/component/bgp/schema/ze-bgp-conf.yang` - BGP config YANG
6. `internal/component/bgp/config/peers.go` - PeersFromConfigTree
7. `internal/component/bgp/config/redistribution.go` - current filter validation
8. `internal/component/config/serialize.go` - serializer

## Task

Build the framework foundations for the route policy system. This is the first child spec of `spec-policy-0-umbrella.md`. It establishes the YANG extensions, config container, filter name registry, parse-time name validation, and serializer enforcement that all subsequent policy specs depend on.

Scope:
1. `ze:filter` YANG extension -- marks YANG lists as filter type lists for mechanical discovery
2. `bgp { policy { } }` container -- empty container in `ze-bgp-conf.yang` that filter type plugins augment into
3. Filter name registry -- collects all filter instance names from `bgp/policy` in config tree, enforces global uniqueness across filter types
4. Name validation in `PeersFromConfigTree` -- validates every name in `filter { import/export [ ... ] }` exists in the registry at parse time
5. `ze:hidden` serializer enforcement -- serializer skips nodes marked `ze:hidden` (hidden from display, still saved to file)
6. `ze:ephemeral` extension -- schema-present for validation/autocomplete but values not persisted to config file

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - overall component architecture
  -> Decision: TBD
  -> Constraint: TBD
- [ ] `docs/architecture/config/syntax.md` - config parsing pipeline, YANG schema handling
  -> Decision: TBD
  -> Constraint: TBD

### RFC Summaries (MUST for protocol work)
- Not applicable -- this spec is config infrastructure, not protocol implementation.

**Key insights:** (to be filled during RESEARCH phase)
- Umbrella decision 1: plugins augment `bgp/policy` with `ze:filter` marked lists
- Umbrella decision 2: name resolution at parse-time in `PeersFromConfigTree`
- Umbrella decision 8: `ze:filter` marks filter type lists in YANG, follows `ze:listener`/`ze:validate` pattern
- Umbrella decision 12: `ze:hidden` hides from display, still saves -- extension exists but not enforced
- Umbrella decision 13: `ze:ephemeral` is new -- not saved to config file, runtime only

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
- [ ] `internal/component/config/yang/modules/ze-extensions.yang` - existing YANG extensions (`ze:listener`, `ze:validate`, `ze:hidden`, etc.)
  -> Constraint: TBD
- [ ] `internal/component/bgp/schema/ze-bgp-conf.yang` - BGP config YANG with `redistribution` container, peer/group structure
  -> Constraint: TBD
- [ ] `internal/component/bgp/config/peers.go` - `PeersFromConfigTree` function, current peer extraction pipeline
  -> Constraint: TBD
- [ ] `internal/component/bgp/config/redistribution.go` - `validateFilterRefs` with `<plugin>:<filter>` format, `DefaultImportFilters`, `applyOverrides`
  -> Constraint: TBD
- [ ] `internal/component/config/serialize.go` - serializer for config output, handles inlining and `inactive:` but does not enforce `ze:hidden`
  -> Constraint: TBD

**Behavior to preserve:**
- TBD -- document during RESEARCH phase

**Behavior to change:**
- TBD -- document during RESEARCH phase

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point
- Config file with `bgp { policy { <type> <name> { ... } } }` defining filter instances
- Config file with `peer { filter { import [ <name> ... ] } }` referencing filters by name

### Transformation Path
1. YANG parsing -- `ze:filter` extension detected on lists under `bgp/policy`
2. Config tree -- filter instances stored under `bgp/policy/<type>/<name>`
3. Filter name registry -- built by scanning `bgp/policy` subtree, collecting all instance names across filter types, enforcing uniqueness
4. `PeersFromConfigTree` -- peer filter chain names resolved against registry, unknown names fail at parse time
5. Serializer -- nodes marked `ze:hidden` skipped during display serialization; nodes marked `ze:ephemeral` skipped during file serialization

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| YANG schema -> config parser | `ze:filter` extension on list nodes | [ ] |
| Config tree -> filter registry | Scan `bgp/policy` subtree for filter instances | [ ] |
| Filter registry -> `PeersFromConfigTree` | Registry lookup for name validation | [ ] |
| Schema -> serializer | `ze:hidden` / `ze:ephemeral` extensions control serialization | [ ] |

### Integration Points
- `PeersFromConfigTree` in `peers.go` -- name validation inserted after template resolution
- `serialize.go` -- hidden/ephemeral check added to serialization path
- `ze-extensions.yang` -- new extensions follow existing declaration pattern
- `ze-bgp-conf.yang` -- new `policy` container at same level as existing `redistribution`

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Config with `bgp { policy { } }` | -> | YANG parse accepts empty policy container | `test/parse/filter-name-resolution.ci` |
| Config with unknown filter name in `filter { import [ bad ] }` | -> | Name validation error in `PeersFromConfigTree` | `test/parse/filter-name-resolution.ci` |
| Config with `ze:hidden` marked node | -> | Serializer skips node in display output | TBD (unit test in serialize_test.go) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `ze:filter` extension declared in `ze-extensions.yang` | YANG module parses without error |
| AC-2 | `ze:ephemeral` extension declared in `ze-extensions.yang` | YANG module parses without error |
| AC-3 | `bgp { policy { } }` container in `ze-bgp-conf.yang` | Config with empty policy container parses successfully |
| AC-4 | Two filter instances with the same name under different types in `bgp/policy` | Registry construction fails with duplicate name error |
| AC-5 | Filter name in `filter { import [ name ] }` that exists in registry | Name resolves successfully, no error |
| AC-6 | Filter name in `filter { import [ bad ] }` that does not exist in registry | Parse-time error with clear message naming the unknown filter |
| AC-7 | Node marked with `ze:hidden` | Serializer omits node from display output |
| AC-8 | Node marked with `ze:hidden` | Node is still saved to config file |
| AC-9 | Node marked with `ze:ephemeral` | Node is present in schema for validation and autocomplete |
| AC-10 | Node marked with `ze:ephemeral` | Node value is not persisted to config file |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestFilterRegistryBuild` | `internal/component/bgp/config/filter_registry_test.go` | Registry collects names from config tree | |
| `TestFilterRegistryDuplicateName` | `internal/component/bgp/config/filter_registry_test.go` | Duplicate name across types rejected | |
| `TestFilterRegistryLookup` | `internal/component/bgp/config/filter_registry_test.go` | Lookup by name returns filter type | |
| `TestFilterNameValidation` | `internal/component/bgp/config/filter_registry_test.go` | Unknown name in peer filter chain fails | |
| `TestSerializeHiddenSkipped` | `internal/component/config/serialize_test.go` | Hidden nodes omitted from display output | |
| `TestSerializeHiddenSaved` | `internal/component/config/serialize_test.go` | Hidden nodes still written to file | |
| `TestSerializeEphemeralNotSaved` | `internal/component/config/serialize_test.go` | Ephemeral nodes omitted from file output | |
| `TestSerializeEphemeralInSchema` | `internal/component/config/serialize_test.go` | Ephemeral nodes present in schema | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Filter name length | 1+ chars | 1 char | 0 (empty string) | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `filter-name-resolution` | `test/parse/filter-name-resolution.ci` | Config with filter names resolves or fails at parse time | |

### Future (if deferring any tests)
- None anticipated -- all tests are in scope.

## Files to Modify

- `internal/component/config/yang/modules/ze-extensions.yang` - add `ze:filter` and `ze:ephemeral` extension declarations
- `internal/component/bgp/schema/ze-bgp-conf.yang` - add empty `policy` container under `bgp`
- `internal/component/bgp/config/peers.go` - integrate filter name validation after template resolution
- `internal/component/bgp/config/redistribution.go` - replace `validateFilterRefs` with registry-based validation
- `internal/component/config/serialize.go` - enforce `ze:hidden` (skip in display), enforce `ze:ephemeral` (skip in file save)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new extensions) | [x] | `internal/component/config/yang/modules/ze-extensions.yang` |
| YANG schema (policy container) | [x] | `internal/component/bgp/schema/ze-bgp-conf.yang` |
| CLI commands/flags | [ ] | N/A for this spec |
| Editor autocomplete | [ ] | YANG-driven (automatic if YANG updated) |
| Functional test for parse-time validation | [x] | `test/parse/filter-name-resolution.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] | Not yet user-facing -- framework only |
| 2 | Config syntax changed? | [ ] | No new user syntax in this spec |
| 3 | CLI command added/changed? | [ ] | N/A |
| 4 | API/RPC added/changed? | [ ] | N/A |
| 5 | Plugin added/changed? | [ ] | N/A |
| 6 | Has a user guide page? | [ ] | N/A |
| 7 | Wire format changed? | [ ] | N/A |
| 8 | Plugin SDK/protocol changed? | [ ] | N/A |
| 9 | RFC behavior implemented? | [ ] | N/A |
| 10 | Test infrastructure changed? | [ ] | N/A |
| 11 | Affects daemon comparison? | [ ] | N/A |
| 12 | Internal architecture changed? | [ ] | TBD -- may need `docs/architecture/config/syntax.md` update for new extensions |

## Files to Create

- `internal/component/bgp/config/filter_registry.go` - filter name registry: build from config tree, enforce uniqueness, provide lookup
- `internal/component/bgp/config/filter_registry_test.go` - unit tests for filter registry
- `test/parse/filter-name-resolution.ci` - functional test for filter name resolution at parse time

## Implementation Steps

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

1. **Phase: YANG extensions** -- add `ze:filter` and `ze:ephemeral` to `ze-extensions.yang`
   - Tests: YANG module parse validation
   - Files: `ze-extensions.yang`
   - Verify: YANG parse succeeds with new extensions

2. **Phase: Policy container** -- add empty `bgp { policy { } }` container to `ze-bgp-conf.yang`
   - Tests: Config with empty policy container parses
   - Files: `ze-bgp-conf.yang`
   - Verify: tests fail -> implement -> tests pass

3. **Phase: Filter registry** -- implement `filter_registry.go` with build, uniqueness, lookup
   - Tests: `TestFilterRegistryBuild`, `TestFilterRegistryDuplicateName`, `TestFilterRegistryLookup`
   - Files: `filter_registry.go`, `filter_registry_test.go`
   - Verify: tests fail -> implement -> tests pass

4. **Phase: Name validation** -- integrate registry into `PeersFromConfigTree`, replace `validateFilterRefs`
   - Tests: `TestFilterNameValidation`, `filter-name-resolution.ci`
   - Files: `peers.go`, `redistribution.go`
   - Verify: tests fail -> implement -> tests pass

5. **Phase: Serializer enforcement** -- `ze:hidden` display skip, `ze:ephemeral` file save skip
   - Tests: `TestSerializeHiddenSkipped`, `TestSerializeHiddenSaved`, `TestSerializeEphemeralNotSaved`, `TestSerializeEphemeralInSchema`
   - Files: `serialize.go`
   - Verify: tests fail -> implement -> tests pass

6. **Functional tests** -- create after feature works. Cover user-visible behavior.
7. **Full verification** -- `make ze-verify` (lint + all ze tests except fuzz)
8. **Complete spec** -- fill audit tables, write learned summary to `plan/learned/NNN-policy-1-framework.md`, delete spec from `plan/`. BLOCKING: summary is part of the commit, not a follow-up.

### Critical Review Checklist (/implement stage 5)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1 through AC-10 has implementation with file:line |
| Correctness | Filter names are truly globally unique across all filter types |
| Naming | Extensions use kebab-case (`ze:filter`, `ze:ephemeral`), YANG containers use kebab-case |
| Data flow | Registry built before name validation, serializer checks happen at serialize time |
| Rule: no-layering | `validateFilterRefs` colon-format code fully deleted, not kept alongside registry validation |
| Rule: config-design | Unknown filter names fail at parse time with clear error message |

### Deliverables Checklist (/implement stage 9)

| Deliverable | Verification method |
|-------------|---------------------|
| `ze:filter` extension in `ze-extensions.yang` | `grep "extension filter" ze-extensions.yang` |
| `ze:ephemeral` extension in `ze-extensions.yang` | `grep "extension ephemeral" ze-extensions.yang` |
| `policy` container in `ze-bgp-conf.yang` | `grep "container policy" ze-bgp-conf.yang` |
| `filter_registry.go` exists | `ls internal/component/bgp/config/filter_registry.go` |
| `filter_registry_test.go` exists | `ls internal/component/bgp/config/filter_registry_test.go` |
| `filter-name-resolution.ci` exists | `ls test/parse/filter-name-resolution.ci` |
| `ze:hidden` enforced in serializer | `grep -n "hidden" internal/component/config/serialize.go` |
| Name validation in `PeersFromConfigTree` | `grep -n "registry\|filter.*name" internal/component/bgp/config/peers.go` |

### Security Review Checklist (/implement stage 10)

| Check | What to look for |
|-------|-----------------|
| Input validation | Filter names from config must be validated for non-empty, no embedded whitespace |
| Name collision | Duplicate names across filter types must be detected and rejected |
| Error messages | Error messages must not leak internal paths or sensitive config data |

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

## RFC Documentation

Not applicable -- this spec is config infrastructure, not protocol implementation.

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
- [ ] Write learned summary to `plan/learned/NNN-policy-1-framework.md`
- [ ] **Summary included in commit** -- NEVER commit implementation without the completed summary. One commit = code + tests + summary.
