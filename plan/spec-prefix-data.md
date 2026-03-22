# Spec: prefix-data

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | - |
| Updated | 2026-03-22 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `plan/learned/413-prefix-limit.md` -- context from predecessor spec

## Task

Data infrastructure for prefix limit management: zefs storage, embedded
routing data, CLI commands, autocompletion, source tracking, advisory
system, and config commit warnings.

This spec receives deferred items from `spec-prefix-limit` (413-prefix-limit).

## Deferred Items from spec-prefix-limit

| Item | AC | Description |
|------|-----|-------------|
| zefs storage | AC-20, AC-22 | Store routing-data.json in `meta/prefix/limit` |
| Embedded data | AC-22, AC-24 | Embed routing-data.json in binary, write to zefs on first run |
| JSON format | AC-24 | Documented per-ASN per-family prefix count format |
| source-url config | AC-23 | Global BGP setting for data fetch URL |
| source field (manual/ze) | AC-6 | Per-family tracking of how maximum was set |
| CLI: ze data prefix update | AC-20 | Fetch latest data from source-url into zefs |
| CLI: ze data prefix show | - | Show current zefs data metadata |
| CLI: ze data prefix lookup | - | Lookup per-family counts for an ASN |
| CLI: ze data prefix import | - | Import operator-provided JSON into zefs |
| CLI: ze bgp peer * prefix maximum update | AC-18, AC-19 | Update peer configs from zefs data |
| Autocompletion | AC-6, AC-7 | Suggest prefix maximums from local data |
| Advisory: staleness warning | AC-12 | Log warning at startup if data > 6 months old |
| Advisory: suggestion field | - | Config field that warns on commit |
| Enforcement .ci test | AC-3 | Blocked on ze-peer race condition for close-time NOTIFICATION capture |

## Prerequisite

Requires routing-data.json to exist. This file is produced by a build
pipeline that analyzes PeeringDB + routing table snapshots. The pipeline
is outside ze's codebase.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` -- zefs integration points

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc4486.md` -- predecessor spec context

**Key insights:** (to be filled during design)

## Current Behavior (MANDATORY)

**Source files read:** (to be filled during design)

**Behavior to preserve:**
- Existing prefix limit enforcement from spec-prefix-limit

**Behavior to change:**
- (to be filled during design)

## Data Flow (MANDATORY)

### Entry Point
- (to be filled during design)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|

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

## Files to Modify

## Files to Create

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|

### Implementation Phases

### Critical Review Checklist (/implement stage 5)

| Check | What to verify for this spec |
|-------|------------------------------|

### Deliverables Checklist (/implement stage 9)

| Deliverable | Verification method |
|-------------|---------------------|

### Security Review Checklist (/implement stage 10)

| Check | What to look for |
|-------|-----------------|

### Failure Routing

| Failure | Route To |
|---------|----------|

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
- [ ] AC-1..AC-N all demonstrated
- [ ] Wiring Test table complete
- [ ] `make ze-test` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

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
