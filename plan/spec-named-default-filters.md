# Spec: named-default-filters

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | - |
| Updated | 2026-03-29 |

## Task

Convert existing in-process protocol filters (AS loop detection, OTC) into named default/mandatory policy filters that populate `DefaultImportFilters`/`DefaultExportFilters` in `redistribution.go`. This enables the override mechanism built in `spec-redistribution-filter` (applyOverrides) and completes AC-18/19/20/21 from that spec.

### Deferred Items Received

| Source | Item |
|--------|------|
| spec-redistribution-filter AC-18 | Default named filter `rfc:no-self-as` active by default |
| spec-redistribution-filter AC-19/20 | Override mechanism for default filters (per-peer/group) |
| spec-redistribution-filter AC-21 | Mandatory filter cannot be overridden |
| spec-redistribution-filter | redistribution-override.ci functional test |

## Required Reading

- [ ] `internal/component/bgp/config/redistribution.go` -- DefaultImportFilters, applyOverrides
- [ ] `internal/component/bgp/reactor/filter/loop.go` -- current LoopIngress filter
- [ ] `internal/component/bgp/plugins/role/otc.go` -- current OTC ingress/egress filters

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
