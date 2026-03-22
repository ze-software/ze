# Spec: rpki-test-isolation

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | - |
| Updated | 2026-03-22 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. Test harness source and rpki-decorator .ci files
4. Plugin startup and registration code

## Task

Investigate and fix intermittent rpki-decorator functional test failures (tests 132-135) when run as part of the full plugin suite. Tests pass in isolation but fail with "registration conflict: command already registered by adj-rib-in" when run sequentially.

Symptoms:
- `rpki-decorator-timeout` (135) fails most often, `rpki-decorator-autoload` (132) and `rpki-decorator-merge` (133) fail intermittently
- Error: `stage 1 (declare-registration): rpc error: registration conflict: command conflict: "adj-rib-in status" already registered by adj-rib-in`
- All tests pass when run individually via `bin/ze-test bgp plugin <N>`
- Failure rate varies: 1-3 tests per full suite run

Hypothesis: external plugin processes from a prior rpki-decorator test may not have fully exited before the next test starts, causing stale processes to connect to the new test's daemon.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/testing/ci-format.md` - functional test format
  → Decision: TBD
  → Constraint: TBD
- [ ] `docs/architecture/api/process-protocol.md` - plugin 5-stage startup
  → Decision: TBD
  → Constraint: TBD

### RFC Summaries (MUST for protocol work)
Not applicable -- test infrastructure work.

**Key insights:**
- TBD during research phase

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `test/plugin/rpki-decorator-timeout.ci` - launches ze with rpki-decorator, adj-rib-in, and a test plugin
- [ ] `test/plugin/rpki-decorator-register.ci` - preceding test, also launches adj-rib-in
- [ ] Test harness port allocation code - TBD
- [ ] `internal/component/plugin/registration.go` - command conflict detection at line 141

**Behavior to preserve:**
- Registration conflict detection (it correctly prevents duplicate commands)
- All rpki-decorator tests pass in isolation
- Test harness port allocation for other test suites

**Behavior to change:**
- Test isolation: ensure each test's plugin processes are fully cleaned up before next test starts

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point
- Test harness starts ze daemon with external plugins on allocated ports
- External plugins connect back to daemon hub listener

### Transformation Path
1. Test N finishes, ze daemon shuts down
2. External plugin processes may still be alive (shutdown race)
3. Test N+1 starts new ze daemon on possibly overlapping port
4. Stale plugin from test N connects to test N+1's daemon
5. Plugin registers commands that are already registered by test N+1's own plugin instance

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Test harness → ze daemon | Process exec + port assignment | [ ] |
| ze daemon → external plugins | Hub TLS listener on ephemeral port | [ ] |
| Test N cleanup → Test N+1 startup | Process termination + port reuse | [ ] |

### Integration Points
- Test harness process management and cleanup
- Hub listener port allocation
- Plugin process lifecycle

### Architectural Verification
- [ ] No bypassed layers
- [ ] No unintended coupling
- [ ] No duplicated functionality
- [ ] Zero-copy preserved where applicable

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `make ze-plugin-test` full suite | -> | Test isolation fix | rpki-decorator tests 132-135 pass consistently in full suite |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Run `make ze-plugin-test` 5 times | All rpki-decorator tests pass every time |
| AC-2 | Run tests 132-135 sequentially via `bin/ze-test bgp plugin 132-135` | All pass |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| TBD during research | TBD | TBD | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A | N/A | N/A | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `rpki-decorator-timeout` | `test/plugin/rpki-decorator-timeout.ci` | Existing test passes reliably | |
| `rpki-decorator-autoload` | `test/plugin/rpki-decorator-autoload.ci` | Existing test passes reliably | |
| `rpki-decorator-merge` | `test/plugin/rpki-decorator-merge.ci` | Existing test passes reliably | |

### Future (if deferring any tests)
- None

## Files to Modify
- TBD during research (test harness, possibly plugin process cleanup)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | N/A |
| CLI commands/flags | No | N/A |
| Editor autocomplete | No | N/A |
| Functional test for new RPC/API | No | N/A |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | No | |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented? | No | |
| 10 | Test infrastructure changed? | Likely | `docs/functional-tests.md` |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | No | |

## Files to Create
- TBD during research

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
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

1. **Phase: Investigation** -- trace the root cause
   - Tests: N/A (research)
   - Files: test harness, plugin startup
   - Verify: root cause identified and documented

2. **Phase: Fix** -- implement test isolation
   - Tests: AC-1 and AC-2
   - Files: TBD
   - Verify: rpki-decorator tests pass reliably in full suite

3. **Full verification** -- `make ze-verify`

4. **Complete spec** -- Fill audit tables, write learned summary

### Critical Review Checklist (/implement stage 5)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | rpki-decorator tests pass reliably |
| Correctness | Fix addresses root cause, not symptom |
| No regression | Other functional tests still pass |

### Deliverables Checklist (/implement stage 9)
| Deliverable | Verification method |
|-------------|---------------------|
| rpki-decorator tests pass in suite | `make ze-plugin-test` 5 consecutive passes |

### Security Review Checklist (/implement stage 10)
| Check | What to look for |
|-------|-----------------|
| Process cleanup | No zombie processes left after test suite |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior |
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
N/A

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
- [ ] AC-1..AC-2 all demonstrated
- [ ] Wiring Test table complete
- [ ] `make ze-test` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes

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
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-rpki-test-isolation.md`
- [ ] **Summary included in commit**
