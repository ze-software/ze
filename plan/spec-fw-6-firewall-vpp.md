# Spec: fw-6-firewall-vpp — VPP Firewall Backend

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | spec-fw-1-data-model, VPP Phase 0 (ze-vpp-plan.md) |
| Phase | - |
| Updated | 2026-04-13 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `plan/spec-fw-0-umbrella.md` — design decisions, VPP compatibility table
3. `plan/ze-vpp-plan.md` — VPP Phase 0 (lifecycle), Phase 4 (ACL/policer)
4. `internal/component/firewall/backend.go` — Backend interface (from fw-1)

## Task

Implement the firewallvpp plugin using GoVPP. The plugin receives `[]Table` via `Apply`,
translates ze Expression types to VPP ACL rules, classifier sessions, and policers.

Depends on VPP Phase 0 (lifecycle management, GoVPP connection) being implemented first.
This spec subsumes VPP plan Phase 4 ACL feature.

## Required Reading

### Architecture Docs
- [ ] `plan/spec-fw-0-umbrella.md` — VPP compatibility mapping table
  → Decision: same data model, different backend translation
- [ ] `plan/ze-vpp-plan.md` — Phase 0 (VPPConn), Phase 4 (ACL)
  → Constraint: depends on VPPConn for GoVPP connection management
  → Constraint: ACL via AclAddReplace, AclInterfaceSetAclList

**Key insights:**
- VPP ACL model is flat rules (src/dst prefix, ports, proto, permit/deny)
- Abstract Match/Action types map directly to VPP ACL fields (no register-chain reverse-engineering)
- MatchSourceAddress → AclRule.SrcPrefix, MatchProtocol → AclRule.Proto, Accept → IsPermit=1
- Complex actions (rate limit, log, counter) map to VPP policer and tracing, not ACL
- Some types have no VPP equivalent (FlowOffload: VPP IS the fast path, silently ignored)

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `plan/ze-vpp-plan.md` — Phase 4 ACL section: AclAddReplace, AclInterfaceSetAclList, PolicerAddDel APIs
  → Constraint: AclAddReplace, AclInterfaceSetAclList, PolicerAddDel APIs
- [ ] `internal/component/firewall/backend.go` — Backend interface (from fw-1, to be created)
  → Constraint: same Apply([]Table) interface as firewallnft
- [ ] `internal/component/firewall/model.go` — Expression types (from fw-1, to be created)
  → Constraint: translate ze Expression types to VPP ACL rules

**Behavior to preserve:**
- Firewall component and firewallnft plugin unaffected
- Same YANG config works with both backends

**Behavior to change:**
- Add firewallvpp plugin as alternative backend

## Data Flow (MANDATORY)

### Entry Point
- Component calls `backend.Apply(desired []Table)` (same as firewallnft)

### Transformation Path
1. Backend receives `[]Table` (ze data model, same as nft backend)
2. For each table/chain/term: translate abstract Match types directly to AclRule fields
3. MatchSourceAddress → SrcPrefix, MatchProtocol → Proto, Accept → IsPermit=1
4. Complex actions (Limit, Log, Counter) → VPP policer or trace
5. Apply ACLs to interfaces via AclInterfaceSetAclList
6. Apply policers via PolicerAddDel

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Component → Plugin | Apply([]Table) | [ ] |
| Plugin → VPP | GoVPP binary API via unix socket | [ ] |

### Integration Points
- `internal/component/firewall/model.go` (fw-1) — same Table/Expression types
- `internal/component/firewall/backend.go` (fw-1) — same Backend interface
- `internal/component/vpp/conn.go` (VPP Phase 0) — GoVPP connection

### Architectural Verification
- [ ] No bypassed layers
- [ ] No unintended coupling
- [ ] No duplicated functionality
- [ ] Zero-copy not applicable

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Firewall config + VPP backend | → | firewallvpp Apply → ACLs in VPP | `test/firewall/010-vpp-boot-apply.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Apply with src/dst address + port + verdict rules | VPP ACL created with matching rules |
| AC-2 | Apply with rate limit expression | VPP policer created |
| AC-3 | Apply with connection state expression | VPP reflexive ACL (stateful) |
| AC-4 | Apply with mark expression | VPP classifier session |
| AC-5 | Apply with NAT expression | VPP NAT44 configured |
| AC-6 | Apply with flow offload expression | Silently ignored (VPP IS the fast path) |
| AC-7 | Backend registered as "vpp" | LoadBackend("vpp") succeeds |
| AC-8 | Same YANG config as nft backend | Produces equivalent filtering behavior |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestTranslateToACLRule` | `internal/plugins/firewallvpp/translate_test.go` | ze expressions → VPP ACL rule |
| `TestTranslateToPolicer` | `internal/plugins/firewallvpp/translate_test.go` | ze limit → VPP policer |
| `TestTranslateToClassifier` | `internal/plugins/firewallvpp/translate_test.go` | ze mark → VPP classifier |
| `TestUnsupportedExpressionHandling` | `internal/plugins/firewallvpp/translate_test.go` | Expressions with no VPP equivalent |
| `TestBackendRegistration` | `internal/plugins/firewallvpp/register_test.go` | RegisterBackend("vpp") |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| ACL rule count | 0-unlimited | VPP limit | N/A | VPP returns error |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| VPP boot apply | `test/firewall/010-vpp-boot-apply.ci` | Config with VPP backend, ACLs created | |

### Future (if deferring any tests)
- Full VPP integration tests deferred until VPP Phase 0 is implemented

## Files to Modify

No existing files modified.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | No | Same as fw-4, backend selected by config leaf |
| CLI commands | No | Same as fw-5 |
| Functional test | Yes | `test/firewall/010-vpp-boot-apply.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` — VPP firewall backend |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` — firewallvpp |
| 2-4,6-12 | Other | No | - |

## Files to Create

- `internal/plugins/firewallvpp/firewallvpp.go` — package doc, logger
- `internal/plugins/firewallvpp/backend_linux.go` — Apply implementation via GoVPP
- `internal/plugins/firewallvpp/backend_other.go` — stub
- `internal/plugins/firewallvpp/translate.go` — ze expressions → VPP ACL/policer/classifier
- `internal/plugins/firewallvpp/translate_test.go` — translation tests
- `internal/plugins/firewallvpp/register.go` — init() RegisterBackend("vpp", factory)
- `test/firewall/010-vpp-boot-apply.ci` — functional test

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file + fw-0 + fw-1 + ze-vpp-plan |
| 2. Audit | Files to Create |
| 3. Implement | Phases below |
| 4-12 | Standard flow |

### Implementation Phases

1. **Phase: Translation layer** — ze expressions → VPP ACL rules + policers + classifiers
   - Tests: TestTranslateToACLRule, TestTranslateToPolicer, TestTranslateToClassifier
   - Files: translate.go

2. **Phase: Backend** — Apply implementation using GoVPP
   - Tests: TestBackendRegistration, functional .ci
   - Files: backend_linux.go, register.go, backend_other.go

3. **Full verification** → `make ze-verify`

### Critical Review Checklist (/implement stage 5)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every translatable expression type handled |
| Correctness | ACL rules produce equivalent filtering to nftables |
| Naming | Backend name is "vpp" |
| Data flow | Same config → same filtering behavior regardless of backend |
| Unsupported expressions | Clearly logged, not silently dropped |

### Deliverables Checklist (/implement stage 9)
| Deliverable | Verification method |
|-------------|---------------------|
| firewallvpp plugin compiles | `go build ./internal/plugins/firewallvpp/...` |
| Translation covers core expressions | unit test output |
| RegisterBackend("vpp") works | unit test |

### Security Review Checklist (/implement stage 10)
| Check | What to look for |
|-------|-----------------|
| ACL rule injection | All rule fields validated before GoVPP call |
| VPP connection | Authenticated via VPPConn (Phase 0) |

### Failure Routing
| Failure | Route To |
|---------|----------|
| GoVPP API mismatch | Check VPP version bindings |
| Expression untranslatable | Document limitation, log warning |
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
- [ ] AC-1..AC-8 all demonstrated
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
- [ ] Write learned summary to `plan/learned/NNN-fw-6-firewall-vpp.md`
- [ ] Summary included in commit
