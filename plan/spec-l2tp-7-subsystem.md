# Spec: l2tp-7 -- L2TP Subsystem Wiring

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | spec-l2tp-6c-ncp |
| Phase | - |
| Updated | 2026-04-13 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `plan/spec-l2tp-0-umbrella.md` -- umbrella context
3. `docs/research/l2tpv2-ze-integration.md sections 3-9, 14-16`

## Task

Wire the L2TP implementation into ze's subsystem infrastructure:

Subsystem: implement ze.Subsystem interface (Start/Stop/Reload). Register
with engine via engine.RegisterSubsystem().

Events: add NamespaceL2TP and ValidL2TPEvents to events.go. Register all
event types. Emit events at tunnel/session lifecycle points.

Config: YANG schema (ze-l2tp-conf.yang) with listen address, shared secret,
timers, limits. Env var registration. Config transaction participation
(verify/apply/rollback).

CLI: show l2tp tunnels, show l2tp sessions, clear l2tp tunnel/session
commands via YANG ze:command declarations.

Redistribute: register "l2tp" source. Inject subscriber /32 routes into
protocol RIB on session-ip-assigned. Withdraw on session-down.

Metrics: Prometheus counters/gauges for tunnels, sessions, messages,
retransmissions, auth failures.

Main binary: wiring in engine startup path + blank imports in all.go.

Reference: docs/research/l2tpv2-ze-integration.md sections 3-9, 14-16.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` -- subsystem and plugin patterns
- [ ] `docs/research/l2tpv2-implementation-guide.md` -- protocol spec
- [ ] `docs/research/l2tpv2-ze-integration.md` -- ze integration design

### RFC Summaries (MUST for protocol work)
- [ ] RFC 2661 -- L2TP

**Key insights:** (to be filled during RESEARCH phase)

## Current Behavior (MANDATORY)

**Source files read:** (to be filled during RESEARCH phase)

**Behavior to preserve:** (to be filled during RESEARCH phase)

**Behavior to change:** (to be filled during RESEARCH phase)

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point
- (to be filled during DESIGN phase)

### Transformation Path
1. (to be filled during DESIGN phase)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|

### Integration Points
- (to be filled during DESIGN phase)

### Architectural Verification
- [ ] No bypassed layers
- [ ] No unintended coupling
- [ ] No duplicated functionality
- [ ] Zero-copy preserved where applicable

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| (to be filled during DESIGN phase) | -> | | |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| (to be filled during DESIGN phase) | | |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|

### Future (if deferring any tests)
- None planned

## Files to Modify
- (to be filled during DESIGN phase)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] | |
| 2 | Config syntax changed? | [ ] | |
| 3 | CLI command added/changed? | [ ] | |
| 4 | API/RPC added/changed? | [ ] | |
| 5 | Plugin added/changed? | [ ] | |
| 6 | Has a user guide page? | [ ] | |
| 7 | Wire format changed? | [ ] | |
| 8 | Plugin SDK/protocol changed? | [ ] | |
| 9 | RFC behavior implemented? | [ ] | |
| 10 | Test infrastructure changed? | [ ] | |
| 11 | Affects daemon comparison? | [ ] | |
| 12 | Internal architecture changed? | [ ] | |

## Files to Create
- (to be filled during DESIGN phase)

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file + umbrella |
| 2. Audit | Files to Modify, Files to Create |
| 3. Implement (TDD) | Implementation phases below |
| 4. Full verification | `make ze-verify` |
| 5-12 | Standard flow |

### Implementation Phases

1. (to be filled during DESIGN phase)

### Critical Review Checklist (/implement stage 5)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation |
| Correctness | (to be filled) |

### Deliverables Checklist (/implement stage 9)
| Deliverable | Verification method |
|-------------|---------------------|

### Security Review Checklist (/implement stage 10)
| Check | What to look for |
|-------|-----------------|
| Input validation | (to be filled) |

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

Add `// RFC 2661 Section X.Y` above enforcing code.

## Implementation Summary

### What Was Implemented
- (to be filled)

### Bugs Found/Fixed
- (to be filled)

### Documentation Updates
- (to be filled)

### Deviations from Plan
- (to be filled)

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
- [ ] AC-N all demonstrated
- [ ] Wiring Test table complete
- [ ] `make ze-verify` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete

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
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary
- [ ] Summary included in commit
