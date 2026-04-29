# Spec: host-2-tuning

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | spec-host-0-inventory (done) |
| Phase | - |
| Updated | 2026-04-29 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/component/host/inventory.go` - Inventory struct, Detector
4. `internal/component/config/` - YANG-modeled config tree

## Task

Add runtime hardware tuning capabilities: CPU governor writes, IRQ affinity
assignment, NIC ethtool coalesce/ring configuration. Provide a YANG config
surface so operators can declare desired tuning in the config tree and have
ze apply it at startup and on config commit.

**Origin:** deferred from spec-host-0-inventory (items 4-5 in Deferrals table, dated 2026-04-18).

### Sub-tasks

1. **Runtime tuning engine** - write-side operations (governor, IRQ affinity, ethtool).
   Each operation must be idempotent and verify the write took effect.

2. **YANG config surface** - model under `system { tuning { ... } }`.

3. **Apply-on-commit** - integrate with config commit pipeline.

4. **Safety** - dry-run mode, rollback on failure, platform guards
   (Linux-only, requires CAP_SYS_ADMIN or root).

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - config commit pipeline
- [ ] `internal/component/host/inventory.go` - current read-only detection
- [ ] `internal/component/host/cpu_linux.go` - reads scaling governor
- [ ] `internal/component/host/nic_linux.go` - reads NIC info
- [ ] `internal/component/host/ethtool_linux.go` - reads ethtool via ioctl
- [ ] `internal/component/config/` - YANG schema, tree, commit

**Key insights:**
- Skeleton: to be filled during design phase

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
- [ ] `internal/component/host/cpu_linux.go` - reads scaling governor, no writes
- [ ] `internal/component/host/nic_linux.go` - reads NIC info, no writes
- [ ] `internal/component/host/ethtool_linux.go` - reads ethtool via ioctl, no writes

**Behavior to preserve:**
- Read-only detection unchanged
- Inventory struct unchanged (detection separate from tuning)
- Non-Linux platforms return ErrUnsupported

**Behavior to change:**
- Add write operations alongside existing read operations
- Add YANG schema for tuning config
- Add commit-time application

## Data Flow (MANDATORY)

### Entry Point
- Config commit with tuning section present
- Startup with tuning config in running config

### Transformation Path
1. Config tree parsed, tuning section extracted
2. Tuning engine compares desired vs current state
3. Write operations applied (governor, IRQ, ethtool)
4. Verification reads confirm writes took effect
5. Failures reported to report bus

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config tree ↔ Tuning engine | Commit callback | [ ] |
| Tuning engine ↔ sysfs | File writes / ioctl | [ ] |
| Tuning engine ↔ Report bus | Error/success events | [ ] |

### Integration Points
- Config commit pipeline - tuning applied on commit
- Host detection - verify writes via existing read paths

### Architectural Verification
- [ ] No bypassed layers
- [ ] No unintended coupling
- [ ] No duplicated functionality
- [ ] Zero-copy preserved where applicable

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Config commit with `system { tuning { cpu { governor performance } } }` | → | tuning.ApplyCPUGovernor | `test/host/host-tuning-governor.ci` |
| Config commit with `system { tuning { ethtool { ... } } }` | → | tuning.ApplyEthtool | `test/host/host-tuning-ethtool.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Config `system { tuning { cpu { governor performance } } }` committed on Linux | scaling_governor set to "performance" |
| AC-2 | Config `system { tuning { irq-affinity { interface eth0 { cpus 0,2 } } } }` | smp_affinity_list set for eth0's IRQs |
| AC-3 | Config `system { tuning { ethtool { interface eth0 { ring { rx 512 } } } } }` | Ring buffer applied |
| AC-4 | Tuning config on non-Linux | Config accepted, apply returns ErrUnsupported warning |
| AC-5 | Invalid governor value in config | YANG validation rejects at parse time |
| AC-6 | Write fails (permission denied) | Error surfaced via report bus |
| AC-7 | Config commit with changed tuning | Only changed parameters re-applied |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestApplyCPUGovernor` | `internal/component/host/tuning_test.go` | AC-1 | skeleton |
| `TestApplyIRQAffinity` | `internal/component/host/tuning_test.go` | AC-2 | skeleton |
| `TestApplyEthtoolRing` | `internal/component/host/tuning_test.go` | AC-3 | skeleton |
| `TestTuningUnsupported` | `internal/component/host/tuning_test.go` | AC-4 | skeleton |
| `TestTuningIdempotent` | `internal/component/host/tuning_test.go` | AC-7 | skeleton |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| rx ring | 1-65535 | 65535 | 0 | 65536 |
| cpu list | 0-N | N (num CPUs - 1) | -1 | N+1 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `host-tuning-governor` | `test/host/host-tuning-governor.ci` | Config commit applies CPU governor | skeleton |
| `host-tuning-ethtool` | `test/host/host-tuning-ethtool.ci` | Config commit applies ethtool ring | skeleton |

### Future (if deferring any tests)
- None planned

## Files to Create
- `internal/component/host/tuning.go` - tuning engine interface
- `internal/component/host/tuning_linux.go` - Linux implementation
- `internal/component/host/tuning_other.go` - ErrUnsupported stub
- `internal/component/host/tuning_test.go` - unit tests
- YANG schema for `system { tuning { ... } }`
- `test/host/host-tuning-governor.ci` - functional test
- `test/host/host-tuning-ethtool.ci` - functional test

## Files to Modify
- Config commit pipeline (wire tuning application)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | Yes | `internal/component/host/schema/` |
| CLI commands/flags | No | - |
| Editor autocomplete | Yes | YANG-driven |
| Functional test for new RPC/API | Yes | `test/host/*.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/guide/tuning.md` (new) |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md` |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | Yes | `docs/guide/tuning.md` |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented? | No | - |
| 10 | Test infrastructure changed? | No | - |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | No | - |

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Implement (TDD) | Implementation phases below |
| 4-13. | Standard flow |

### Implementation Phases

1. **Phase: Tuning Engine** -- write operations for governor, IRQ, ethtool
   - Tests: `TestApplyCPUGovernor`, `TestApplyIRQAffinity`, `TestApplyEthtoolRing`
   - Files: `tuning.go`, `tuning_linux.go`, `tuning_other.go`
   - Verify: tests fail → implement → tests pass

2. **Phase: YANG Schema** -- config surface for tuning
   - Tests: YANG validation tests
   - Files: YANG schema files
   - Verify: parse + validate

3. **Phase: Commit Integration** -- apply on config commit
   - Tests: `TestTuningIdempotent`
   - Files: commit pipeline wiring
   - Verify: end-to-end functional test

4. **Full verification** → `make ze-verify`

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Writes verified via read-back |
| Safety | Permission checks, rollback on failure |
| Data flow | Config commit → tuning engine → sysfs write → verify |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| tuning_linux.go exists | `ls internal/component/host/tuning_linux.go` |
| YANG schema exists | `ls internal/component/host/schema/` |
| All tests pass | `go test ./internal/component/host/...` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | CPU list, ring sizes validated before sysfs write |
| Privilege escalation | Tuning requires root/CAP_SYS_ADMIN |
| Injection | No shell commands; direct sysfs/ioctl only |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
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

## RFC Documentation

N/A

## Implementation Summary

### What Was Implemented
- Skeleton

### Bugs Found/Fixed
- None

### Documentation Updates
- None

### Deviations from Plan
- None

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
- **Total items:** -
- **Done:** -
- **Partial:** -
- **Skipped:** -
- **Changed:** -

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied
- Skeleton

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

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
- [ ] `/ze-review` gate clean
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
- [ ] Write learned summary
- [ ] Summary included in commit
