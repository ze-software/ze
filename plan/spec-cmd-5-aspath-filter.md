# Spec: cmd-5 -- AS-Path Filter Plugin

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
5. `internal/component/bgp/reactor/filter/` -- in-process loop-detection filter

## Task

Create `bgp-filter-aspath` plugin. Named AS-path regex lists under `bgp/policy` with
`ze:filter` extension. Each entry has `regex` (string) and `action` (accept/reject,
default accept). Referenced from filter chains as `as-path-list:NAME`.

**Config syntax (editor):**

| Command | Purpose |
|---------|---------|
| `set bgp policy as-path-list PEERS-ONLY regex "^[0-9]+$"` | Match single-AS paths |
| `set bgp policy as-path-list TRANSIT regex "^[0-9]+ [0-9]+"` | Match paths with transit |
| `set bgp peer X filter import as-path-list:PEERS-ONLY` | Apply AS-path filter on import |

**YANG location:** `bgp/policy` container, new `as-path-list` list with `ze:filter` extension.

| Leaf | Type | Default | Notes |
|------|------|---------|-------|
| `regex` | string | (required) | Regex matched against space-separated AS-path representation |
| `action` | enum {accept, reject} | accept | What to do when regex matches |

**AS-path matching rules:**
- AS-path converted to space-separated decimal string representation (e.g., "65001 65002 65003")
- Regex matched against the full string representation
- Entries evaluated in order; first match wins
- No match = implicit deny (reject)
- Empty AS-path represented as empty string ""

**Security:** Regex complexity must be limited to prevent ReDoS attacks. Compile-time validation
with timeout or complexity limit.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` -- plugin model, filter chain
  -> Constraint: filter plugins are independent, composable in chains
- [ ] `.claude/patterns/plugin.md` -- how to create a filter plugin
  -> Constraint: filter plugins augment bgp/policy, use ze:filter extension

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc4271.md` -- BGP-4 base: AS-PATH attribute format
  -> Constraint: AS-PATH is a sequence of AS numbers, each 4 bytes (or 2 in old format)

**Key insights:**
- AS-path string representation is space-separated decimal ASNs
- Empty AS-path (locally originated) must be matchable
- Regex must be compiled once at config load, not per-UPDATE
- ReDoS protection is essential since regex comes from user config

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/filter_community/` -- existing filter plugin pattern
- [ ] `internal/component/bgp/plugins/filter_community/register.go` -- plugin registration
- [ ] `internal/component/bgp/reactor/filter_chain.go` -- filter chain dispatch
- [ ] `internal/component/bgp/reactor/filter/` -- in-process loop-detection filter
- [ ] `internal/component/bgp/schema/ze-bgp-conf.yang` -- bgp/policy container

**Behavior to preserve:**
- Existing filter chain dispatch order
- Existing community filter and loop-detection filter unchanged
- All existing config files parse and work identically

**Behavior to change:**
- New as-path-list list under bgp/policy in YANG
- New bgp-filter-aspath plugin registered
- Filter chain recognizes `as-path-list:NAME` references
- UPDATEs with non-matching AS-paths rejected by filter

## Data Flow (MANDATORY)

### Entry Point
- Config: `bgp { policy { as-path-list PEERS-ONLY { ... } } }` parsed from YANG
- Wire: UPDATE received, AS-PATH attribute checked against regex entries

### Transformation Path
1. Config parse: YANG as-path-list entries extracted by ResolveBGPTree()
2. Regex compilation: each regex compiled and validated at config load time
3. Plugin registration: bgp-filter-aspath plugin registers with filter registry
4. Filter chain setup: `filter import as-path-list:PEERS-ONLY` wires into peer's import chain
5. UPDATE receive: wire bytes pass through import filter chain
6. AS-PATH extraction: AS-PATH attribute extracted from UPDATE, converted to string
7. Regex matching: string matched against entries in order; first match wins
8. Action: matching entry's action applied; no match = reject

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| Config -> Plugin | ResolveBGPTree() extracts as-path-list config, passed to plugin at init | [ ] |
| Reactor -> Plugin | Filter chain dispatches filter-update RPC with wire bytes | [ ] |

### Integration Points
- `ResolveBGPTree()` -- extract as-path-list configuration
- `filter_chain.go` -- dispatch to as-path-list plugin for matching
- Plugin registration -- bgp-filter-aspath registers as filter type

### Architectural Verification
- [ ] No bypassed layers (config -> resolver -> filter chain -> plugin)
- [ ] No unintended coupling (as-path-list is independent, composable)
- [ ] No duplicated functionality (new filter type, follows existing pattern)
- [ ] Zero-copy preserved (plugin receives wire bytes, extracts AS-PATH for matching)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Config with `policy as-path-list` + `filter import as-path-list:X` | → | UPDATE with matching AS-path accepted | `test/plugin/aspath-filter.ci` |
| Config with `policy as-path-list` + `filter import as-path-list:X` | → | UPDATE with non-matching AS-path rejected | `test/plugin/aspath-filter.ci` |
| Config parse with as-path-list entries | → | YANG validates as-path-list syntax | `test/parse/aspath-list-config.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Regex matches AS-path string | Accepted by filter (action=accept) |
| AC-2 | Regex does not match AS-path string | Not matched by this entry; next entry evaluated |
| AC-3 | Accept action on match | Route accepted, passes to next filter in chain |
| AC-4 | Reject action on match | Route rejected, not forwarded |
| AC-5 | Multiple entries, first match wins | First matching entry's action applied |
| AC-6 | No entry matches | Implicit deny (reject) |
| AC-7 | Empty AS-path matches `^$` | Locally originated routes matchable |
| AC-8 | Composable in chain with other filters | `filter import as-path-list:X prefix-list:Y` works |
| AC-9 | Invalid regex in config | Rejected at config load time with clear error |
| AC-10 | Regex complexity limit | Overly complex regex rejected to prevent ReDoS |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestAsPathRegexMatch` | `filter_aspath_test.go` | Regex matches AS-path string | |
| `TestAsPathRegexNoMatch` | `filter_aspath_test.go` | Regex does not match | |
| `TestAsPathRejectAction` | `filter_aspath_test.go` | Reject action applied | |
| `TestAsPathFirstMatchWins` | `filter_aspath_test.go` | Multiple entries, first match | |
| `TestAsPathImplicitDeny` | `filter_aspath_test.go` | No match = reject | |
| `TestAsPathEmptyPath` | `filter_aspath_test.go` | Empty AS-path matches ^$ | |
| `TestAsPathInvalidRegex` | `filter_aspath_test.go` | Invalid regex rejected | |
| `TestAsPathRegexComplexity` | `filter_aspath_test.go` | Complex regex rejected | |

### Boundary Tests (MANDATORY for numeric inputs)

No numeric inputs in this spec (regex string + enum action).

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A | N/A | N/A | N/A | N/A |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `aspath-filter` | `test/plugin/aspath-filter.ci` | Config with AS-path filter, verify accept/reject | |
| `aspath-list-config` | `test/parse/aspath-list-config.ci` | Config with as-path-list entries parses correctly | |

## Files to Modify

- `internal/component/bgp/schema/ze-bgp-conf.yang` -- add as-path-list under bgp/policy
- `internal/component/bgp/config/resolve.go` -- extract as-path-list config
- `internal/component/bgp/reactor/filter_chain.go` -- dispatch to as-path-list filter

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new list) | [x] | `internal/component/bgp/schema/ze-bgp-conf.yang` |
| Plugin registration | [x] | `internal/component/bgp/plugins/filter_aspath/register.go` |
| Functional test | [x] | `test/plugin/aspath-filter.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [x] | `docs/features.md` -- add AS-path filtering |
| 2 | Config syntax changed? | [x] | `docs/guide/configuration.md` -- as-path-list config examples |
| 3 | CLI command added/changed? | [ ] | N/A |
| 4 | API/RPC added/changed? | [ ] | N/A |
| 5 | Plugin added/changed? | [x] | `docs/guide/plugins.md` -- bgp-filter-aspath plugin |
| 6 | Has a user guide page? | [ ] | N/A |
| 7 | Wire format changed? | [ ] | N/A |
| 8 | Plugin SDK/protocol changed? | [ ] | N/A |
| 9 | RFC behavior implemented? | [ ] | N/A |
| 10 | Test infrastructure changed? | [ ] | N/A |
| 11 | Affects daemon comparison? | [x] | `docs/comparison.md` -- AS-path filtering now supported |
| 12 | Internal architecture changed? | [ ] | N/A |

## Files to Create

- `internal/component/bgp/plugins/filter_aspath/` -- new plugin directory
- `internal/component/bgp/plugins/filter_aspath/register.go` -- plugin registration
- `internal/component/bgp/plugins/filter_aspath/filter.go` -- AS-path regex matching logic
- `internal/component/bgp/plugins/filter_aspath/filter_test.go` -- unit tests
- `test/plugin/aspath-filter.ci` -- functional test
- `test/parse/aspath-list-config.ci` -- config parse test

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

1. **Phase: YANG + Config** -- Add as-path-list to ze-bgp-conf.yang, extract in ResolveBGPTree()
   - Tests: config parse test
   - Files: ze-bgp-conf.yang, resolve.go
2. **Phase: Plugin Skeleton** -- Create bgp-filter-aspath plugin following community filter pattern
   - Tests: `TestAsPathRegexMatch`
   - Files: filter_aspath/register.go, filter_aspath/filter.go
3. **Phase: Regex Matching** -- Implement regex matching with ReDoS protection
   - Tests: `TestAsPathRegex*`, `TestAsPathEmptyPath`, `TestAsPathInvalidRegex`, `TestAsPathRegexComplexity`
   - Files: filter_aspath/filter.go
4. **Phase: Filter Chain Integration** -- Wire into filter_chain.go dispatch
   - Tests: verify filter chain calls as-path-list plugin
   - Files: filter_chain.go
5. **Functional tests** -- .ci tests proving end-to-end behavior
6. **Full verification** -- `make ze-verify`

### Critical Review Checklist (/implement stage 5)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | All 10 ACs demonstrated |
| Pattern compliance | Follows community filter plugin pattern |
| ReDoS protection | Regex complexity limited |
| Empty path | Empty AS-path matchable with ^$ |
| Composability | Works in chain with other filters |

### Deliverables Checklist (/implement stage 9)

| Deliverable | Verification method |
|-------------|---------------------|
| YANG as-path-list in ze-bgp-conf.yang | `grep as-path-list internal/component/bgp/schema/ze-bgp-conf.yang` |
| Plugin directory | `ls internal/component/bgp/plugins/filter_aspath/` |
| .ci functional tests | `ls test/plugin/aspath-filter.ci test/parse/aspath-list-config.ci` |

### Security Review Checklist (/implement stage 10)

| Check | What to look for |
|-------|-----------------|
| ReDoS | Regex compilation with complexity limit or timeout |
| Input validation | Regex syntax validated at config load, not at match time |

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
- [ ] AC-1..AC-10 all demonstrated
- [ ] Wiring Test table complete
- [ ] `make ze-test` passes
- [ ] Feature code integrated
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)
- [ ] Pattern compliance with existing filter plugins
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
- [ ] Write learned summary to `plan/learned/NNN-cmd-5-aspath-filter.md`
- [ ] Summary included in commit
