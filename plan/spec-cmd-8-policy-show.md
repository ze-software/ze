# Spec: cmd-8 -- Policy Introspection Commands

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | filter plugins exist |
| Phase | - |
| Updated | 2026-04-10 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `plan/spec-cmd-0-umbrella.md` -- umbrella context
3. `internal/component/bgp/reactor/filter_chain.go` -- filter chain dispatch
4. `internal/component/bgp/config/resolve.go` -- config resolution
5. `internal/component/cmd/show/` -- existing show command pattern

## Task

Add `show policy` command tree for runtime policy introspection. Four subcommands:
- `show policy list` -- all filters by type
- `show policy detail <type:name>` -- one filter's config
- `show policy chain <peer-sel> [import|export]` -- effective chain after inheritance
- `show policy test <peer-sel> <import|export> prefix <prefix> [attrs]` -- dry-run: would this route pass?

Policy dry-run testing is unique to Ze -- no vendor has this built-in.

**Config syntax (editor):**

| Command | Purpose |
|---------|---------|
| `show policy list` | List all registered filter types and instances |
| `show policy detail prefix-list:CUSTOMERS` | Show one filter's full configuration |
| `show policy chain peer 10.0.0.1 import` | Show effective import chain for a peer |
| `show policy test peer 10.0.0.1 import prefix 10.0.0.0/24` | Dry-run: test if prefix would be accepted |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` -- CLI command dispatch, show command pattern
  -> Constraint: show commands are operational, not config; dispatched via CLI handler registry
- [ ] `.claude/patterns/cli-command.md` -- how to add CLI commands
  -> Constraint: command registration, YANG-modeled, handler function

**Key insights:**
- show commands are operational queries, read-only, no state mutation
- Filter registry knows all registered filter types and instances
- Peer config includes resolved filter chain after group inheritance
- Dry-run testing simulates filter chain execution without actual route processing

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/cmd/show/` -- existing show command implementations
- [ ] `internal/component/bgp/reactor/filter_chain.go` -- filter chain dispatch
- [ ] `internal/component/bgp/config/resolve.go` -- config resolution with inheritance
- [ ] `internal/component/bgp/config/filter_registry.go` -- filter type registration
- [ ] `internal/component/cli/model.go` -- CLI command constants

**Behavior to preserve:**
- Existing show commands unchanged
- Filter chain execution order unchanged
- Config inheritance (bgp > group > peer) unchanged
- All existing config files parse and work identically

**Behavior to change:**
- New `show policy` command tree with four subcommands
- Filter registry queryable for introspection
- Peer's effective filter chain queryable after inheritance resolution
- Dry-run testing simulates filter execution on synthetic route

## Data Flow (MANDATORY)

### Entry Point
- CLI: `show policy list` typed in CLI session or SSH
- CLI: `show policy test peer 10.0.0.1 import prefix 10.0.0.0/24` for dry-run

### Transformation Path
1. Command parse: CLI dispatcher routes `show policy` to policy handler
2. Subcommand dispatch: `list`, `detail`, `chain`, or `test` handler invoked
3. For `list`: query filter registry for all registered types and instances
4. For `detail`: query filter registry for specific filter's config
5. For `chain`: resolve peer's effective filter chain after group inheritance
6. For `test`: construct synthetic route from prefix + optional attrs, execute filter chain, report result
7. Output: formatted text response returned to CLI session

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| CLI -> Reactor | Show command dispatched to reactor for filter/peer state queries | [ ] |
| Reactor -> Filter Registry | Filter registry queried for type/instance information | [ ] |
| Reactor -> Config | Peer config queried for effective chain after inheritance | [ ] |

### Integration Points
- CLI command dispatcher -- register `show policy` commands
- Filter registry -- query interface for list/detail
- Peer config resolution -- effective chain after inheritance
- Filter chain execution -- dry-run mode for testing

### Architectural Verification
- [ ] No bypassed layers (CLI -> dispatcher -> handler -> registry/config)
- [ ] No unintended coupling (read-only queries, no state mutation)
- [ ] No duplicated functionality (new command tree, no overlap with existing)
- [ ] Zero-copy not applicable (read-only query, text output)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| CLI `show policy list` | → | Policy handler queries filter registry, returns list | `test/plugin/policy-show.ci` |
| CLI `show policy chain peer X import` | → | Policy handler resolves peer's effective chain | `test/plugin/policy-show.ci` |
| CLI `show policy test peer X import prefix P` | → | Policy handler dry-runs filter chain | `test/plugin/policy-test.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `show policy list` | All registered filter types and instances listed |
| AC-2 | `show policy detail prefix-list:CUSTOMERS` | Full config of the named prefix-list shown |
| AC-3 | `show policy detail` with nonexistent filter | Error: filter not found |
| AC-4 | `show policy chain peer 10.0.0.1 import` | Effective import chain shown after group inheritance |
| AC-5 | `show policy chain` for peer with no filters | Empty chain shown (no filters configured) |
| AC-6 | `show policy test peer X import prefix 10.0.0.0/24` | Returns accept/reject and which filter decided |
| AC-7 | `show policy test` with attributes | Returns accept/reject and shows modify result |
| AC-8 | `show policy test` on peer with no filters | Route accepted (empty chain = accept all) |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPolicyList` | `policy_show_test.go` | List returns all registered filters | |
| `TestPolicyDetail` | `policy_show_test.go` | Detail returns single filter config | |
| `TestPolicyDetailNotFound` | `policy_show_test.go` | Nonexistent filter returns error | |
| `TestPolicyChain` | `policy_show_test.go` | Chain shows effective filters after inheritance | |
| `TestPolicyChainEmpty` | `policy_show_test.go` | Empty chain for peer with no filters | |
| `TestPolicyTestAccept` | `policy_test_test.go` | Dry-run returns accept with deciding filter | |
| `TestPolicyTestReject` | `policy_test_test.go` | Dry-run returns reject with deciding filter | |
| `TestPolicyTestModify` | `policy_test_test.go` | Dry-run shows attribute modifications | |
| `TestPolicyTestEmptyChain` | `policy_test_test.go` | Empty chain accepts all | |

### Boundary Tests (MANDATORY for numeric inputs)

No numeric inputs in this spec (commands are string-based).

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A | N/A | N/A | N/A | N/A |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `policy-show` | `test/plugin/policy-show.ci` | show policy list and chain with registered filters | |
| `policy-test` | `test/plugin/policy-test.ci` | show policy test dry-run accept/reject | |

## Files to Modify

- `internal/component/cmd/show/` -- add policy show handlers
- `internal/component/bgp/config/filter_registry.go` -- add query interface for introspection
- `internal/component/bgp/reactor/filter_chain.go` -- add dry-run execution mode

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| CLI command registration | [x] | `internal/component/cmd/show/policy.go` |
| YANG additions | [x] | `ze-cli-show-cmd.yang` |
| Functional test | [x] | `test/plugin/policy-show.ci`, `test/plugin/policy-test.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [x] | `docs/features.md` -- add policy introspection |
| 2 | Config syntax changed? | [ ] | N/A (operational commands) |
| 3 | CLI command added/changed? | [x] | `docs/guide/commands.md` -- show policy commands |
| 4 | API/RPC added/changed? | [ ] | N/A |
| 5 | Plugin added/changed? | [ ] | N/A |
| 6 | Has a user guide page? | [x] | `docs/guide/policy.md` -- policy introspection guide |
| 7 | Wire format changed? | [ ] | N/A |
| 8 | Plugin SDK/protocol changed? | [ ] | N/A |
| 9 | RFC behavior implemented? | [ ] | N/A |
| 10 | Test infrastructure changed? | [ ] | N/A |
| 11 | Affects daemon comparison? | [x] | `docs/comparison.md` -- policy introspection and dry-run unique to Ze |
| 12 | Internal architecture changed? | [ ] | N/A |

## Files to Create

- `internal/component/cmd/show/policy.go` -- policy show command handlers
- `test/plugin/policy-show.ci` -- policy show functional test
- `test/plugin/policy-test.ci` -- policy dry-run functional test

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

1. **Phase: show policy list/detail** -- Query filter registry, format output
   - Tests: `TestPolicyList`, `TestPolicyDetail`, `TestPolicyDetailNotFound`
   - Files: show/policy.go, filter_registry.go
2. **Phase: show policy chain** -- Resolve peer's effective chain after inheritance
   - Tests: `TestPolicyChain`, `TestPolicyChainEmpty`
   - Files: show/policy.go
3. **Phase: show policy test** -- Dry-run filter chain execution on synthetic route
   - Tests: `TestPolicyTest*`
   - Files: show/policy.go, filter_chain.go
4. **Functional tests** -- .ci tests proving end-to-end behavior
5. **Full verification** -- `make ze-verify`

### Critical Review Checklist (/implement stage 5)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | All 8 ACs demonstrated |
| Read-only | show policy commands do not mutate state |
| Inheritance | chain shows effective filters after group inheritance |
| Dry-run accuracy | test result matches what actual filter chain would produce |

### Deliverables Checklist (/implement stage 9)

| Deliverable | Verification method |
|-------------|---------------------|
| Policy show handlers | `ls internal/component/cmd/show/policy.go` |
| CLI command registration | `grep 'policy' internal/component/cmd/show/` |
| .ci functional tests | `ls test/plugin/policy-show.ci test/plugin/policy-test.ci` |

### Security Review Checklist (/implement stage 10)

| Check | What to look for |
|-------|-----------------|
| Read-only | show commands must not mutate any state |
| Input validation | Peer selector and filter name validated |

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
- [ ] AC-1..AC-8 all demonstrated
- [ ] Wiring Test table complete
- [ ] `make ze-test` passes
- [ ] Feature code integrated
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)
- [ ] Dry-run accuracy verified against actual chain
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
- [ ] Write learned summary to `plan/learned/NNN-cmd-8-policy-show.md`
- [ ] Summary included in commit
