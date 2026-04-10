# Spec: cmd-4 -- Prefix-List Filter Plugin

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | policy framework |
| Phase | - |
| Updated | 2026-04-10 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `plan/spec-cmd-0-umbrella.md` -- umbrella context
3. `internal/component/bgp/plugins/filter_community/` -- existing filter plugin pattern
4. `internal/component/bgp/reactor/filter_chain.go` -- filter chain dispatch
5. `internal/component/bgp/reactor/filter/` -- in-process loop-detection filter

## Task

Create `bgp-filter-prefix` plugin. Named prefix-lists under `bgp/policy` with `ze:filter`
extension. Each entry has `prefix` (CIDR), `ge` (uint8), `le` (uint8), `action`
(accept/reject, default accept). Referenced from filter chains as `prefix-list:NAME`.

**Config syntax (editor):**

| Command | Purpose |
|---------|---------|
| `set bgp policy prefix-list CUSTOMERS prefix 10.0.0.0/8 ge 16 le 24` | Match prefixes in range |
| `set bgp policy prefix-list CUSTOMERS prefix 192.168.0.0/16 action reject` | Reject exact match |
| `set bgp peer X filter import prefix-list:CUSTOMERS` | Apply prefix-list on import |

**YANG location:** `bgp/policy` container, new `prefix-list` list with `ze:filter` extension.

| Leaf | Type | Default | Notes |
|------|------|---------|-------|
| `prefix` | inet:ip-prefix (CIDR) | (required) | IPv4 or IPv6 prefix |
| `ge` | uint8 | prefix-length | Greater-than-or-equal prefix length |
| `le` | uint8 | 32 (IPv4) or 128 (IPv6) | Less-than-or-equal prefix length |
| `action` | enum {accept, reject} | accept | What to do when prefix matches |

**Prefix matching rules:**
- A route prefix P/L matches entry E/EL with ge/le if: P is a subnet of E/EL AND L >= ge AND L <= le
- ge defaults to the prefix length of the entry
- le defaults to the maximum prefix length for the address family
- Entries evaluated in order; first match wins
- No match = implicit deny (reject)

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` -- plugin model, filter chain
  -> Constraint: filter plugins are independent, composable in chains
- [ ] `.claude/patterns/plugin.md` -- how to create a filter plugin
  -> Constraint: filter plugins augment bgp/policy, use ze:filter extension
- [ ] `.claude/patterns/config-option.md` -- YANG leaf addition pattern
  -> Constraint: YANG + resolver + wiring + .ci test

**Key insights:**
- Filter plugins follow the community filter pattern (tag/strip)
- Filter chain dispatch is in filter_chain.go
- Each filter plugin is a separate binary under bgp/plugins/
- ze:filter extension marks YANG containers as filter-eligible

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/filter_community/` -- existing filter plugin pattern
- [ ] `internal/component/bgp/plugins/filter_community/register.go` -- plugin registration
- [ ] `internal/component/bgp/reactor/filter_chain.go` -- filter chain dispatch
- [ ] `internal/component/bgp/reactor/filter/` -- in-process loop-detection filter
- [ ] `internal/component/bgp/schema/ze-bgp-conf.yang` -- bgp/policy container

**Behavior to preserve:**
- Existing filter chain dispatch order (in-process filters first, then plugin filters)
- Existing community filter functionality unchanged
- Existing loop-detection filter unchanged
- All existing config files parse and work identically

**Behavior to change:**
- New prefix-list list under bgp/policy in YANG
- New bgp-filter-prefix plugin registered
- Filter chain recognizes `prefix-list:NAME` references
- UPDATEs with non-matching prefixes rejected by filter

## Data Flow (MANDATORY)

### Entry Point
- Config: `bgp { policy { prefix-list CUSTOMERS { ... } } }` parsed from YANG
- Wire: UPDATE received, NLRI prefixes checked against prefix-list entries

### Transformation Path
1. Config parse: YANG prefix-list entries extracted by ResolveBGPTree()
2. Plugin registration: bgp-filter-prefix plugin registers with filter registry
3. Filter chain setup: `filter import prefix-list:CUSTOMERS` wires prefix-list into peer's import chain
4. UPDATE receive: wire bytes pass through import filter chain
5. Prefix extraction: NLRI prefixes extracted from UPDATE
6. Prefix matching: each prefix checked against prefix-list entries in order
7. Action: first matching entry's action (accept/reject) applied; no match = reject

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| Config -> Plugin | ResolveBGPTree() extracts prefix-list config, passed to plugin at init | [ ] |
| Reactor -> Plugin | Filter chain dispatches filter-update RPC with wire bytes | [ ] |

### Integration Points
- `ResolveBGPTree()` -- extract prefix-list configuration
- `filter_chain.go` -- dispatch to prefix-list plugin for matching
- Plugin registration -- bgp-filter-prefix registers as filter type

### Architectural Verification
- [ ] No bypassed layers (config -> resolver -> filter chain -> plugin)
- [ ] No unintended coupling (prefix-list is independent, composable)
- [ ] No duplicated functionality (new filter type, follows existing pattern)
- [ ] Zero-copy preserved (plugin receives wire bytes, extracts NLRI for matching)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Config with `policy prefix-list` + `filter import prefix-list:X` | → | UPDATE with matching prefix accepted | `test/plugin/prefix-filter.ci` |
| Config with `policy prefix-list` + `filter import prefix-list:X` | → | UPDATE with non-matching prefix rejected | `test/plugin/prefix-filter.ci` |
| Config parse with prefix-list entries | → | YANG validates prefix-list syntax | `test/parse/prefix-list-config.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Prefix within ge/le range | Accepted by filter |
| AC-2 | Prefix outside ge/le range (too specific) | Rejected by filter |
| AC-3 | Prefix outside ge/le range (too general) | Rejected by filter |
| AC-4 | Exact match (ge=le=prefix-length) | Accepted when action=accept |
| AC-5 | Exact match with action=reject | Rejected by filter |
| AC-6 | Default action (no explicit action) | accept |
| AC-7 | Multiple entries, first match wins | First matching entry's action applied |
| AC-8 | No entry matches | Implicit deny (reject) |
| AC-9 | IPv4 prefix support | Correct matching for IPv4 CIDR |
| AC-10 | IPv6 prefix support | Correct matching for IPv6 CIDR |
| AC-11 | Composable in chain with other filters | `filter import prefix-list:X as-path-list:Y` works |
| AC-12 | ge boundary: 0 (minimum) | Valid |
| AC-13 | le boundary: 128 (maximum for IPv6) | Valid |
| AC-14 | ge > le (invalid) | Rejected by YANG validation |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPrefixMatchInRange` | `filter_prefix_test.go` | Prefix within ge/le range matches | |
| `TestPrefixMatchOutOfRange` | `filter_prefix_test.go` | Prefix outside range does not match | |
| `TestPrefixMatchExact` | `filter_prefix_test.go` | Exact prefix match | |
| `TestPrefixMatchReject` | `filter_prefix_test.go` | Reject action applied | |
| `TestPrefixMatchFirstWins` | `filter_prefix_test.go` | Multiple entries, first match wins | |
| `TestPrefixMatchImplicitDeny` | `filter_prefix_test.go` | No match = reject | |
| `TestPrefixMatchIPv4` | `filter_prefix_test.go` | IPv4 prefix matching | |
| `TestPrefixMatchIPv6` | `filter_prefix_test.go` | IPv6 prefix matching | |
| `TestPrefixListGeLeBoundary` | `filter_prefix_test.go` | ge=0, le=128 boundaries | |

### Boundary Tests (MANDATORY for numeric inputs)

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| ge | 0-128 | 128 | N/A (uint8) | 129 |
| le | 0-128 | 128 | N/A (uint8) | 129 |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `prefix-filter` | `test/plugin/prefix-filter.ci` | Config with prefix-list filter, verify accept/reject | |
| `prefix-list-config` | `test/parse/prefix-list-config.ci` | Config with prefix-list entries parses correctly | |

## Files to Modify

- `internal/component/bgp/schema/ze-bgp-conf.yang` -- add prefix-list under bgp/policy
- `internal/component/bgp/config/resolve.go` -- extract prefix-list config
- `internal/component/bgp/reactor/filter_chain.go` -- dispatch to prefix-list filter

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new list) | [x] | `internal/component/bgp/schema/ze-bgp-conf.yang` |
| Plugin registration | [x] | `internal/component/bgp/plugins/filter_prefix/register.go` |
| Functional test | [x] | `test/plugin/prefix-filter.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [x] | `docs/features.md` -- add prefix-list filtering |
| 2 | Config syntax changed? | [x] | `docs/guide/configuration.md` -- prefix-list config examples |
| 3 | CLI command added/changed? | [ ] | N/A |
| 4 | API/RPC added/changed? | [ ] | N/A |
| 5 | Plugin added/changed? | [x] | `docs/guide/plugins.md` -- bgp-filter-prefix plugin |
| 6 | Has a user guide page? | [ ] | N/A |
| 7 | Wire format changed? | [ ] | N/A |
| 8 | Plugin SDK/protocol changed? | [ ] | N/A |
| 9 | RFC behavior implemented? | [ ] | N/A |
| 10 | Test infrastructure changed? | [ ] | N/A |
| 11 | Affects daemon comparison? | [x] | `docs/comparison.md` -- prefix-list filtering now supported |
| 12 | Internal architecture changed? | [ ] | N/A |

## Files to Create

- `internal/component/bgp/plugins/filter_prefix/` -- new plugin directory
- `internal/component/bgp/plugins/filter_prefix/register.go` -- plugin registration
- `internal/component/bgp/plugins/filter_prefix/filter.go` -- prefix matching logic
- `internal/component/bgp/plugins/filter_prefix/filter_test.go` -- unit tests
- `test/plugin/prefix-filter.ci` -- functional test
- `test/parse/prefix-list-config.ci` -- config parse test

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

1. **Phase: YANG + Config** -- Add prefix-list to ze-bgp-conf.yang, extract in ResolveBGPTree()
   - Tests: config parse test
   - Files: ze-bgp-conf.yang, resolve.go
2. **Phase: Plugin Skeleton** -- Create bgp-filter-prefix plugin following community filter pattern
   - Tests: `TestPrefixMatch*`
   - Files: filter_prefix/register.go, filter_prefix/filter.go
3. **Phase: Prefix Matching** -- Implement ge/le/action matching logic
   - Tests: `TestPrefixMatchInRange`, `TestPrefixMatchOutOfRange`, `TestPrefixMatchExact`, `TestPrefixListGeLeBoundary`
   - Files: filter_prefix/filter.go
4. **Phase: Filter Chain Integration** -- Wire into filter_chain.go dispatch
   - Tests: verify filter chain calls prefix-list plugin
   - Files: filter_chain.go
5. **Functional tests** -- .ci tests proving end-to-end behavior
6. **Full verification** -- `make ze-verify`

### Critical Review Checklist (/implement stage 5)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | All 14 ACs demonstrated |
| Pattern compliance | Follows community filter plugin pattern |
| IPv4 and IPv6 | Both address families tested |
| Composability | Works in chain with other filters |
| ge/le validation | ge <= le enforced |

### Deliverables Checklist (/implement stage 9)

| Deliverable | Verification method |
|-------------|---------------------|
| YANG prefix-list in ze-bgp-conf.yang | `grep prefix-list internal/component/bgp/schema/ze-bgp-conf.yang` |
| Plugin directory | `ls internal/component/bgp/plugins/filter_prefix/` |
| .ci functional tests | `ls test/plugin/prefix-filter.ci test/parse/prefix-list-config.ci` |

### Security Review Checklist (/implement stage 10)

| Check | What to look for |
|-------|-----------------|
| Input validation | CIDR validated, ge/le ranges validated, ge <= le enforced |
| Resource exhaustion | Limit prefix-list entry count if needed |

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
- [ ] AC-1..AC-14 all demonstrated
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
- [ ] Write learned summary to `plan/learned/NNN-cmd-4-prefix-filter.md`
- [ ] Summary included in commit
