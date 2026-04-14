# Spec: fw-7-traffic-vpp — VPP Traffic Control Backend

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | spec-fw-1-data-model, spec-vpp-1-lifecycle |
| Phase | - |
| Updated | 2026-04-13 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `plan/spec-fw-0-umbrella.md` — design decisions, VPP tc mapping table
3. `plan/spec-vpp-0-umbrella.md` — VPP architecture, design decisions
4. `plan/spec-vpp-1-lifecycle.md` — VPP lifecycle component (GoVPP connection)
5. `internal/component/traffic/backend.go` — Backend interface (from fw-1)

## Task

Implement the trafficvpp plugin using GoVPP. The plugin receives `map[string]InterfaceQoS`
via `Apply`, translates ze traffic control types to VPP policers, QoS egress maps, and
classifier sessions.

Depends on spec-vpp-1-lifecycle (VPP component, GoVPP connection) being implemented first.
trafficvpp gets the GoVPP connection via direct import of `internal/component/vpp/`
(`vpp.Channel()`), same pattern as fibvpp/firewallvpp.

## Required Reading

### Architecture Docs
- [ ] `plan/spec-fw-0-umbrella.md` — VPP tc compatibility mapping table
  → Decision: HTB → PolicerAddDel, priority → QosEgressMapUpdate, mark → ClassifyAddDelSession
- [ ] `plan/spec-vpp-1-lifecycle.md` — VPP lifecycle component
  → Constraint: depends on vpp.Channel() for GoVPP connection (direct import)
  → Constraint: Policer via PolicerAddDel, QoS via QosEgressMapUpdate/QosMarkEnableDisable

**Key insights:**
- VPP policer model is per-interface token bucket (CIR/EIR), not hierarchical like HTB
- Translation from HTB classes to VPP policers is lossy (no class hierarchy in VPP policer)
- DSCP-based QoS maps natively to VPP's QoS egress map
- Mark-based classification maps to VPP classifier tables

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `plan/spec-vpp-0-umbrella.md` — VPP architecture decisions (connection sharing, YANG ownership)
  → Constraint: GoVPP connection via direct import of internal/component/vpp/
- [ ] `internal/component/traffic/backend.go` — Backend interface (from fw-1, to be created)
  → Constraint: same Apply(map[string]InterfaceQoS) interface as trafficnetlink
- [ ] `internal/component/traffic/model.go` — InterfaceQoS types (from fw-1, to be created)
  → Constraint: translate ze Qdisc/Class/Filter types to VPP policer/classifier

**Behavior to preserve:**
- Traffic component and trafficnetlink plugin unaffected
- Same YANG config works with both backends

**Behavior to change:**
- Add trafficvpp plugin as alternative backend

## Data Flow (MANDATORY)

### Entry Point
- Component calls `backend.Apply(desired map[string]InterfaceQoS)` (same as trafficnetlink)

### Transformation Path
1. Backend receives `map[string]InterfaceQoS` (ze data model, same as netlink backend)
2. For each interface: resolve VPP interface index via SwInterfaceDump
3. Translate qdisc type to VPP policer parameters (CIR from rate, EIR from ceil)
4. Translate classes to individual policers or QoS map entries
5. Translate mark-based filters to classifier sessions
6. Apply via GoVPP: PolicerAddDel, QosEgressMapUpdate, ClassifyAddDelSession

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Component → Plugin | Apply(map[string]InterfaceQoS) | [ ] |
| Plugin → VPP | GoVPP binary API via unix socket | [ ] |

### Integration Points
- `internal/component/traffic/model.go` (fw-1) — same InterfaceQoS types
- `internal/component/traffic/backend.go` (fw-1) — same Backend interface
- `internal/component/vpp/` (spec-vpp-1-lifecycle) — GoVPP connection via vpp.Channel()

### Architectural Verification
- [ ] No bypassed layers
- [ ] No unintended coupling
- [ ] No duplicated functionality
- [ ] Zero-copy not applicable

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Traffic config + VPP backend | → | trafficvpp Apply → policers in VPP | `test/traffic/010-vpp-boot-apply.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Apply with HTB qdisc and classes | VPP policers created with CIR from rate, EIR from ceil |
| AC-2 | Apply with mark-based filter | VPP classifier session matching mark |
| AC-3 | Apply with DSCP-based classification | VPP QoS egress map configured |
| AC-4 | Apply with priority classes | Policers with relative CIR priority |
| AC-5 | Interface not found in VPP | Clear error message |
| AC-6 | Backend registered as "vpp" | LoadBackend("vpp") succeeds |
| AC-7 | Same YANG config as netlink backend | Produces equivalent QoS behavior |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestTranslateHTBToPolicer` | `internal/plugins/trafficvpp/translate_test.go` | ze HTB → VPP policer parameters |
| `TestTranslateFilterToClassifier` | `internal/plugins/trafficvpp/translate_test.go` | ze filter → VPP classifier |
| `TestTranslateDSCPToQoSMap` | `internal/plugins/trafficvpp/translate_test.go` | ze DSCP → VPP QoS egress map |
| `TestBackendRegistration` | `internal/plugins/trafficvpp/register_test.go` | RegisterBackend("vpp") |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Policer CIR bps | 1+ | 1 | 0 | VPP limit |
| Policer EIR bps | >= CIR | CIR | CIR-1 | VPP limit |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| VPP boot apply | `test/traffic/010-vpp-boot-apply.ci` | Config with VPP backend, policers created | |

### Future (if deferring any tests)
- Full VPP integration tests deferred until VPP Phase 0 is implemented

## Files to Modify

No existing files modified.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | No | Same as fw-4, backend selected by config leaf |
| CLI commands | No | Same as fw-5 |
| Functional test | Yes | `test/traffic/010-vpp-boot-apply.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` — VPP traffic control backend |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` — trafficvpp |
| 2-4,6-12 | Other | No | - |

## Files to Create

- `internal/plugins/trafficvpp/trafficvpp.go` — package doc, logger
- `internal/plugins/trafficvpp/backend_linux.go` — Apply implementation via GoVPP
- `internal/plugins/trafficvpp/backend_other.go` — stub
- `internal/plugins/trafficvpp/translate.go` — ze types → VPP policer/classifier/QoS map
- `internal/plugins/trafficvpp/translate_test.go` — translation tests
- `internal/plugins/trafficvpp/register.go` — init() RegisterBackend("vpp", factory)
- `test/traffic/010-vpp-boot-apply.ci` — functional test

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file + fw-0 + fw-1 + vpp-0-umbrella + vpp-1-lifecycle |
| 2. Audit | Files to Create |
| 3. Implement | Phases below |
| 4-12 | Standard flow |

### Implementation Phases

1. **Phase: Translation layer** — ze InterfaceQoS → VPP policer/classifier/QoS map
   - Tests: TestTranslateHTBToPolicer, TestTranslateFilterToClassifier, TestTranslateDSCPToQoSMap
   - Files: translate.go

2. **Phase: Backend** — Apply implementation using GoVPP
   - Tests: TestBackendRegistration, functional .ci
   - Files: backend_linux.go, register.go, backend_other.go

3. **Full verification** → `make ze-verify`

### Critical Review Checklist (/implement stage 5)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every qdisc type translatable or clearly documented as unsupported |
| Correctness | CIR/EIR computed correctly from rate/ceil |
| Naming | Backend name is "vpp" |
| Data flow | Same config → equivalent QoS behavior regardless of backend |
| Lossy translation | HTB hierarchy flattened to per-class policers, documented |

### Deliverables Checklist (/implement stage 9)
| Deliverable | Verification method |
|-------------|---------------------|
| trafficvpp plugin compiles | `go build ./internal/plugins/trafficvpp/...` |
| Translation covers HTB/HFSC | unit test output |
| RegisterBackend("vpp") works | unit test |

### Security Review Checklist (/implement stage 10)
| Check | What to look for |
|-------|-----------------|
| Rate values | Validated as positive before GoVPP call |
| VPP connection | Authenticated via VPPConn (Phase 0) |

### Failure Routing
| Failure | Route To |
|---------|----------|
| GoVPP API mismatch | Check VPP version bindings |
| Rate translation wrong | Re-check CIR/EIR computation |
| 3 fix attempts fail | STOP. Report. Ask user. |

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
- [ ] AC-1..AC-7 all demonstrated
- [ ] Wiring Test table complete
- [ ] `make ze-test` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)
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
- [ ] Write learned summary to `plan/learned/NNN-fw-7-traffic-vpp.md`
- [ ] Summary included in commit
