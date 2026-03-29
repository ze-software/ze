# Spec: redistribution-filter-phase2

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | - |
| Updated | 2026-03-29 |

## Task

Complete the remaining redistribution filter infrastructure: wire-level dirty tracking (re-encode only modified attributes using ModAccumulator), attribute validation (reject modify of undeclared attributes), raw mode (deliver hex wire bytes), export modify functional test, and deferred unit tests.

### Deferred Items Received

| Source | Item |
|--------|------|
| spec-redistribution-filter AC-10 | Export filter modify dedicated functional test |
| spec-redistribution-filter AC-13 | Reject modify of undeclared attribute |
| spec-redistribution-filter AC-15 | raw=true delivers hex wire bytes |
| spec-redistribution-filter | Wire-level dirty tracking (ModAccumulator integration) |
| spec-redistribution-filter | TestAttributeAccumulation, TestDirtyTracking, TestFilterModifyOnlyDeclared |

## Required Reading

- [ ] `internal/component/bgp/reactor/filter_chain.go` -- PolicyFilterChain, applyFilterDelta
- [ ] `internal/component/bgp/reactor/forward_build.go` -- buildModifiedPayload, ModAccumulator
- [ ] `internal/component/bgp/reactor/reactor_api_forward.go` -- export filter wiring

## Current Behavior (MANDATORY)

**Source files read:** TBD during design phase.

## Data Flow (MANDATORY)

TBD during design phase.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| TBD | -> | TBD | TBD |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| TBD | TBD | TBD | [ ] |

## Files to Modify

TBD during design phase.

## Implementation Steps

TBD during design phase.

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

### Completion (BLOCKING -- before ANY commit)
- [ ] `make ze-test` passes
- [ ] Implementation Audit filled
- [ ] Write learned summary
