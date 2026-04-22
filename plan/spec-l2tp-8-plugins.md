# Spec: l2tp-8 -- L2TP Plugins

| Field | Value |
|-------|-------|
| Status | done |
| Depends | spec-l2tp-7-subsystem |
| Phase | - |
| Updated | 2026-04-22 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `plan/spec-l2tp-0-umbrella.md` -- umbrella context
3. `docs/research/l2tpv2-ze-integration.md section 6`

## Task

Implement four L2TP plugins using ze's plugin registration pattern:

l2tp-auth: RADIUS authentication for PPP sessions. Subscribe to
(l2tp, session-auth-request). Query RADIUS Access-Request. Return
accept/reject + attributes. RADIUS accounting (start/interim/stop).
CoA/DM handling for live session changes.

l2tp-pool: IP address pool management. Subscribe to
(l2tp, session-ip-request). Allocate IPv4/IPv6 addresses from configured
pools. RADIUS-directed pool selection via Framed-Pool attribute. Release
on session-down.

l2tp-shaper: Traffic shaping via TC (traffic control). Subscribe to
(l2tp, session-up). Apply TBF/HTB rules on pppN interfaces. Handle
rate changes from RADIUS CoA. Clean up on session-down.

l2tp-stats: Session statistics and Prometheus metrics. Subscribe to
session lifecycle events. Periodic stats collection via PPPIOCGL2TPSTATS.
Per-session byte/packet counters.

All plugins use correct registry.Registration fields: RunEngine
(func(conn net.Conn) int), CLIHandler, YANG, ConfigureEngineLogger,
ConfigureMetrics, ConfigureEventBus.

Reference: docs/research/l2tpv2-ze-integration.md section 6.

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
