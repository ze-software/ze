# Spec: host-3-smart

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
3. `internal/component/host/inventory.go` - StorageInfo, StorageDevice structs
4. `internal/component/host/storage_linux.go` - current block device detection

## Task

Add SMART health monitoring for storage devices via `smartctl --json`. Extends
the existing StorageDevice struct with health status, temperature, power-on hours,
and error counts. Requires `smartctl` binary (smartmontools package).

**Origin:** deferred from spec-host-0-inventory (item 6 in Deferrals table, dated 2026-04-18).
Reason for deferral: adds external tool dependency; out of scope for sysfs-only inventory.

## Required Reading

### Architecture Docs
- [ ] `internal/component/host/inventory.go` - StorageInfo, StorageDevice
- [ ] `internal/component/host/storage_linux.go` - block device enumeration

**Key insights:**
- Skeleton: to be filled during design phase

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
- [ ] `internal/component/host/storage_linux.go` - enumerates /sys/block/*, reads size/model/serial
- [ ] `internal/component/host/inventory.go` - StorageDevice struct

**Behavior to preserve:**
- Block device enumeration unchanged
- StorageDevice fields unchanged (additive only)
- Non-Linux returns ErrUnsupported

**Behavior to change:**
- Add Smart field to StorageDevice
- Add smartctl detection after block device enumeration

## Data Flow (MANDATORY)

### Entry Point
- `Detector.DetectStorage()` called (already exists)

### Transformation Path
1. Block devices enumerated from /sys/block/* (existing)
2. For each device: exec `smartctl --json=c --all /dev/<name>`
3. Parse JSON output into SmartInfo struct
4. Attach SmartInfo to StorageDevice

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Go process ↔ smartctl binary | exec.Command | [ ] |
| JSON output ↔ SmartInfo struct | json.Unmarshal | [ ] |

### Integration Points
- `Detector.DetectStorage()` - call SMART detection per device

### Architectural Verification
- [ ] No bypassed layers
- [ ] No unintended coupling
- [ ] No duplicated functionality
- [ ] Zero-copy preserved where applicable

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ze host show storage` | → | Detector.DetectStorage → detectSMART | `test/host/host-smart-storage.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Linux host with smartctl and SMART-capable disk | StorageDevice.Smart populated with health, temperature, power-on hours |
| AC-2 | Linux host without smartctl | StorageDevice.Smart nil, no error propagated |
| AC-3 | Device without SMART support | StorageDevice.Smart has Unavailable reason |
| AC-4 | `show host storage` output | SMART data included in JSON when available |
| AC-5 | Non-Linux platform | No SMART detection attempted |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestDetectSMARTSuccess` | `internal/component/host/smart_linux_test.go` | AC-1 | skeleton |
| `TestDetectSMARTNoSmartctl` | `internal/component/host/smart_linux_test.go` | AC-2 | skeleton |
| `TestDetectSMARTUnsupported` | `internal/component/host/smart_linux_test.go` | AC-3 | skeleton |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A | - | - | - | - |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `host-smart-storage` | `test/host/host-smart-storage.ci` | `ze host show storage` includes SMART data | skeleton |

### Future (if deferring any tests)
- None planned

## Files to Create
- `internal/component/host/smart_linux.go` - smartctl parsing
- `internal/component/host/smart_other.go` - stub
- `internal/component/host/smart_linux_test.go` - tests with canned smartctl JSON
- `test/host/host-smart-storage.ci` - functional test

## Files to Modify
- `internal/component/host/inventory.go` - add SmartInfo struct, extend StorageDevice
- `internal/component/host/storage_linux.go` - call SMART detection after enumeration

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | - |
| CLI commands/flags | No | existing `ze host show storage` |
| Editor autocomplete | No | - |
| Functional test for new RPC/API | Yes | `test/host/host-smart-storage.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/guide/monitoring.md` |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | No | - |
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

1. **Phase: SmartInfo type** -- struct and JSON tags
   - Tests: none (type only)
   - Files: `inventory.go`
   - Verify: compiles

2. **Phase: smartctl parser** -- exec + JSON parsing
   - Tests: `TestDetectSMARTSuccess`, `TestDetectSMARTNoSmartctl`, `TestDetectSMARTUnsupported`
   - Files: `smart_linux.go`, `smart_other.go`, `smart_linux_test.go`
   - Verify: tests fail → implement → tests pass

3. **Phase: Storage integration** -- call SMART per device
   - Tests: functional test
   - Files: `storage_linux.go`
   - Verify: `ze host show storage` includes SMART data

4. **Full verification** → `make ze-verify`

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | JSON parsing handles all smartctl output variants |
| Security | Device names come from sysfs only, never user input |
| Data flow | Detection → exec → parse → attach |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| smart_linux.go exists | `ls internal/component/host/smart_linux.go` |
| SmartInfo in inventory.go | `grep SmartInfo internal/component/host/inventory.go` |
| All tests pass | `go test ./internal/component/host/...` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | Device names from sysfs only, never from user/network input |
| Command injection | No shell interpolation; exec.Command with explicit args |
| Privilege | smartctl needs root/CAP_SYS_RAWIO; document requirement |

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
