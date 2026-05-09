# Spec: appliance-4-device-config

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | appliance-1-builder, appliance-2-remote |
| Phase | - |
| Updated | 2026-05-09 |
| Split | Split from appliance-1-builder. Device-side runtime behavior for config loading and revert. |

## Task

Device-side config management for ze appliances: config loading priority (pushed > seed), validation of pushed configs before applying, automatic revert on runtime failure, and last-known-good hash verification at boot.

Split from `spec-appliance-1-builder` (design session 2026-05-09). The bastion side of config-push (SSH upload) is in `spec-appliance-2-remote`. This spec covers what the device does when it receives a pushed config.

## Required Reading

To be filled during design phase. Expected:
- [ ] `internal/component/config/` - config loading and startup path
- [ ] `internal/plugins/bgp/reactor/` - BGP session monitoring (for auto-revert triggers)
- [ ] `pkg/zefs/` - ZeFS key reading at runtime
- [ ] `spec-appliance-1-builder.md` Design section (Last-Known-Good Config, Config Push)

## Current Behavior (MANDATORY)

To be filled during design phase. Expected source files:
- [ ] Config startup path (how ze currently loads config from ZeFS at boot)
- [ ] BGP session state change hooks (for health-check-based auto-revert)

## Data Flow (MANDATORY)

To be filled during design phase.

### Entry Point
- Device boot / config reload

### Transformation Path
1. Read ZeFS seed config and compute SHA-256
2. Verify hash matches `meta/config/last-known-good`
3. Check for `/perm/ze/config-pushed.conf`
4. If pushed config exists and is valid, use it; otherwise use seed config
5. Write effective config hash to `/perm/ze/config-active-hash`

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| ZeFS -> runtime config | zefs.ReadFile at boot | [ ] |
| Pushed config file -> runtime config | os.ReadFile + validate | [ ] |

### Integration Points
- `internal/component/config/` - config loading
- `pkg/zefs/` - ZeFS reading
- BGP reactor - session flap detection for auto-revert

### Architectural Verification
- [ ] No bypassed layers
- [ ] No unintended coupling
- [ ] No duplicated functionality
- [ ] Zero-copy preserved where applicable

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Device boot with seed config only | -> | config loader | `TestBootWithSeedConfigOnly` |
| Device boot with valid pushed config | -> | config loader | `TestBootWithValidPushedConfig` |
| Device boot with invalid pushed config | -> | config loader | `TestBootWithInvalidPushedConfigFallsBack` |
| Config apply triggers revert on failure | -> | health monitor | `TestAutoRevertOnRuntimeFailure` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-70 | `ze appliance build lab` produces last-known-good hash | ZeFS contains `meta/config/last-known-good` with SHA-256 of validated seed config |
| AC-71 | `ze appliance config-push lab` with config that passes validation but causes runtime failure | Device detects failure (health check timeout), reverts to last-known-good config from ZeFS seed, prints revert reason to device log |
| AC-72 | `ze appliance config-push lab` updates last-known-good | After device confirms applied config is healthy, device updates /perm/ze/last-known-good with new config hash |
| AC-73 | Device boots with no config-pushed.conf | Uses ZeFS seed config (last-known-good baseline); normal boot path unchanged |
| AC-74 | Device boots with config-pushed.conf that fails validation | Ignores pushed config, uses ZeFS seed config, logs warning |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestBootWithSeedConfigOnly` | TBD | AC-73: normal boot with seed config | |
| `TestBootWithValidPushedConfig` | TBD | Pushed config preferred over seed | |
| `TestBootWithInvalidPushedConfigFallsBack` | TBD | AC-74: invalid pushed config ignored | |
| `TestLastKnownGoodHashVerification` | TBD | Hash in ZeFS matches seed config SHA-256 | |
| `TestAutoRevertOnRuntimeFailure` | TBD | AC-71: health check failure triggers revert | |
| `TestConfigActiveHashWritten` | TBD | /perm/ze/config-active-hash updated after load | |
| `TestBuildWritesLastKnownGood` | `cmd/ze/appliance/cmd_build_test.go` | AC-70: ZeFS contains meta/config/last-known-good | |
| `TestAssembleWritesLastKnownGood` | `cmd/ze/appliance/cmd_assemble_test.go` | ZeFS contains meta/config/last-known-good after assemble | |
| `TestLastKnownGoodHashMatchesSeedConfig` | `cmd/ze/appliance/cmd_build_test.go` | Hash matches SHA-256 of file/template/ze.conf content | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| To be defined during design phase | | | |

## Files to Modify

To be determined during design phase. Expected:
- Config startup/loading path in `internal/component/config/`

## Files to Create

To be determined during design phase.

## Implementation Steps

To be defined during design phase.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Config loading priority | Pushed config preferred when valid; seed config used as fallback |
| Validation before apply | Invalid pushed config never activates |
| Auto-revert triggers | Health check timeout and BGP flap within 30s both trigger revert |
| Two-tier revert chain | Previous pushed -> seed config fallback order |
| Last-known-good integrity | Hash is SHA-256 of validated seed config |
| No boot regression | Normal boot (no pushed config) unchanged |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| Config loading priority logic | Boot test with/without pushed config |
| Auto-revert on failure | Health check failure test |
| Last-known-good hash in ZeFS | `bin/ze data cat --path <db> meta/config/last-known-good` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| **Last-known-good integrity** | Hash in meta/config/last-known-good is SHA-256 of the validated seed config; device trusts this as the fallback |
| **Config push validation** | Device validates pushed config (parse + semantic check) before applying; invalid config never activates |
| **Config push revert safety** | Device saves previous config before applying new; auto-reverts on validation failure; no config loss |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Design

### Scope

See `spec-appliance-1-builder.md` Design section for full details:
- **Last-Known-Good Config** subsection: hash format, boot behavior, two-tier revert chain
- **Config Push** subsection: device-side loading priority, auto-revert triggers, config-previous.conf

### In scope (device-side runtime)
- Config loading priority: `/perm/ze/config-pushed.conf` (if valid) > ZeFS `file/template/ze.conf`
- Validation of pushed config before applying (parse + semantic check)
- Auto-revert on validation failure: delete invalid pushed config, use seed config
- Auto-revert on runtime failure: health check timeout, BGP session flap within 30s of apply
- Two-tier revert chain: previous pushed config -> ZeFS seed config
- Last-known-good hash verification at boot
- `/perm/ze/config-active-hash` for fleet drift detection
- Build-time: writing `meta/config/last-known-good` SHA-256 hash into ZeFS (AC-70)

### Out of scope
- Bastion-side SSH upload (appliance-2-remote)
- Config drift detection and mandatory resync (ze fleet spec)
- Staged rollout coordination (ze fleet spec)

## Resolved Questions

None yet.

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
- [List actual changes made]

### Bugs Found/Fixed
- [Any bugs discovered]

### Documentation Updates
- [Docs updated, or "None"]

### Deviations from Plan
- [Differences from original plan and why]

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
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied

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
- [ ] AC-70..AC-74 all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean (Review Gate section filled)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end

### Quality Gates (SHOULD pass -- defer with user approval)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed
