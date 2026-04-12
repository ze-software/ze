# Spec: gokrazy-4 -- Network Resilience

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | - |
| Updated | 2026-04-12 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `spec-gokrazy-0-umbrella.md` -- parent spec with research

## Task

Stretch goal: add network resilience features that gokrazy provides and ze
currently lacks. These are polish items that improve operational reliability
but are not required for the gokrazy build change.

**Features:**
- Link-state failover: deprioritize routes when interface carrier drops
- Readiness gate: components can block until clock is sane
- Route priority: configurable metric for multi-uplink setups
- DNS conflict detection: don't overwrite resolv.conf if another process manages it

## Required Reading

### Architecture Docs
- [ ] `docs/features/interfaces.md` -- interface monitoring
  -> Constraint: monitor already detects link up/down events
- [ ] `internal/component/iface/iface.go` -- TopicUp, TopicDown events
  -> Constraint: link state events already published on event bus

### RFC Summaries (MUST for protocol work)
- N/A

**Key insights:**
- Link up/down events already exist on the event bus (TopicUp, TopicDown)
- Route priority could be a per-interface config option in YANG
- Readiness gate could use a flag file or event bus subscription

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/iface/iface.go` -- TopicUp/TopicDown constants
- [ ] `internal/component/iface/backend.go` -- no route priority support

**Behavior to preserve:**
- Link monitoring and event publishing
- All existing interface operations

**Behavior to change:**
- On link down: deprioritize routes on that interface
- On link up: restore route priority
- New YANG leaf for route priority per interface
- Readiness gate mechanism

## Data Flow (MANDATORY)

### Entry Point
- Link state change detected by netlink monitor
- TopicUp/TopicDown event on event bus

### Transformation Path
1. Monitor detects carrier loss on interface
2. Publishes TopicDown event
3. Subscriber (new) adjusts route metrics for that interface
4. On carrier restore: TopicUp, metrics restored

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| Netlink -> monitor | rtnetlink carrier events | [ ] |
| Monitor -> event bus | TopicDown/TopicUp | [ ] |
| Event bus -> route manager | Subscribe to link events | [ ] |

### Integration Points
- iface monitor (existing)
- Backend route methods (from spec-gokrazy-1)
- YANG schema for route-priority leaf

### Architectural Verification
- [ ] No bypassed layers
- [ ] No unintended coupling
- [ ] No duplicated functionality
- [ ] Zero-copy preserved where applicable (N/A)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Link carrier drop | -> | Route metric increased | To be designed |
| Link carrier restore | -> | Route metric restored | To be designed |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Interface carrier drops | Routes via that interface get deprioritized (metric 1024) |
| AC-2 | Interface carrier restores | Routes restored to configured priority |
| AC-3 | Route priority configured in YANG | Applied as route metric |
| AC-4 | No route priority configured | Default metric (0) used |
| AC-5 | Clock readiness gate | Components can query clock status |
| AC-6 | DNS conflict detection | resolv.conf not overwritten if externally managed |

## TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| To be designed | | | |

### Boundary Tests (MANDATORY for numeric inputs)

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Route priority | 0-1024 | 1024 | N/A | 1025 |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| To be designed | | | |

### Future
- Full multi-uplink failover test

## Files to Modify

- To be determined during design phase

### Integration Checklist

| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | [x] | route-priority leaf |
| Functional test | [x] | TBD |

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [x] | `docs/features.md` -- link-state failover |
| 2 | Config syntax changed? | [x] | route-priority leaf |
| 3-12 | | [ ] | TBD during design |

## Files to Create

- To be determined during design phase

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2-12 | To be detailed during design |

### Implementation Phases

To be designed.

### Critical Review Checklist (/implement stage 5)

| Check | What to verify for this spec |
|-------|------------------------------|
| To be designed | |

### Deliverables Checklist (/implement stage 9)

| Deliverable | Verification method |
|-------------|---------------------|
| To be designed | |

### Security Review Checklist (/implement stage 10)

| Check | What to look for |
|-------|-----------------|
| To be designed | |

### Failure Routing

| Failure | Route To |
|---------|----------|
| 3 fix attempts fail | STOP. Ask user. |

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

(to be filled during design)

## RFC Documentation

N/A

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
- [ ] AC-1..AC-6 all demonstrated
- [ ] `make ze-verify` passes
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)
- [ ] Implementation Audit complete

### Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL
- [ ] Tests PASS

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-gokrazy-4-resilience.md`
- [ ] Summary included in commit
