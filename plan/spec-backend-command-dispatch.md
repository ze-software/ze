# Spec: Backend-Dispatched Health Checks

| Field | Value |
|-------|-------|
| Status | complete |
| Depends | - |
| Phase | 4/4 |
| Updated | 2026-05-25 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/component/cmd/show/health_checks.go` - backend-specific checks in wrong package
4. `internal/component/cmd/show/show.go` lines 47-63 - health.Register calls in init()
5. `internal/core/health/registry.go` - health.Register API
6. `internal/plugins/iface/vpp/cmd_show.go` - example of completed backend dispatch (RPC handlers)
7. `internal/plugins/firewall/nft/cmd_show.go` - example of completed backend dispatch (RPC handlers)
8. `internal/component/ike/engine/health.go` - prior art: component registering its own health check
9. `internal/component/pki/health.go` - prior art: component registering its own health check

## Task

Move backend-specific health checks from `cmd/show/health_checks.go` to the backend plugins that own them. Today, `cmd/show/` imports `firewall` for `checkFirewallHealth()` and hardcodes `/run/vpp/api.sock` for `checkVPPHealth()`. These backend-specific checks should register from their backend packages using the existing `health.Register()` API, the same pattern already applied to RPC handlers.

**Prior art:** VPP trace RPCs and firewall ruleset RPCs were previously moved from `cmd/show/` to `plugins/iface/vpp/cmd_show.go` and `plugins/firewall/nft/cmd_show.go` using `init()` + `pluginserver.RegisterRPCs()`. IKE and PKI already register their own health checks from `ike/engine/health.go` and `pki/health.go`. This spec applies the same pattern to the firewall and VPP health checks.

**Mixed-backend consideration:** This work enables a future where VPP and netlink coexist (mixed backends). When health checks register from their own backend package, each backend reports its own health independently. A future orchestrator can aggregate without `cmd/show/` needing to know which backends are loaded.

## Problem

`cmd/show/health_checks.go` contains six health check functions. Two are backend-specific but live in a central package:

| Check | Imports | Backend-specific? | Move? |
|-------|---------|-------------------|-------|
| `checkFirewallHealth` | `firewall` (calls `firewall.AuditTables()`) | Yes (nft only) | Yes, to `plugins/firewall/nft/` |
| `checkVPPHealth` | none (hardcodes `/run/vpp/api.sock`) | Yes (VPP only) | Yes, to `plugins/iface/vpp/` |
| `checkIfaceHealth` | `iface` (calls `iface.CheckAllInterfaceErrors()`) | No (both backends) | No |
| `checkBGPHealth` | `report` only | No | No |
| `checkFIBHealth` | `report` only | No | No |
| `checkPluginHealth` | `report` only | No | No |

After moving `checkFirewallHealth` and `checkVPPHealth`, `health_checks.go` loses its `firewall` import and VPP socket constant. The remaining checks (`checkBGPHealth`, `checkFIBHealth`, `checkPluginHealth`, `checkIfaceHealth`) stay because they are not backend-specific.

The `health.Register()` calls in `show.go` init() (lines 60, 63) for "firewall" and "vpp" also move to the respective backend packages.

## Design Direction

**Health check registration from backend packages:**

Each backend plugin registers its own health check via `init()` + `health.Register()`. The check function lives next to the code it calls. This is the same pattern IKE (`ike/engine/health.go`) and PKI (`pki/health.go`) already use.

Current (`cmd/show/show.go` init + `cmd/show/health_checks.go`):
```
cmd/show/show.go:        init() { health.Register("firewall", checkFirewallHealth) }
cmd/show/health_checks.go:  func checkFirewallHealth() { firewall.AuditTables() ... }
                             import "component/firewall"
```

Proposed:
```
plugins/firewall/nft/health.go:  init() { health.Register("firewall", checkFirewallHealth) }
                                  func checkFirewallHealth() { firewall.AuditTables() ... }
                                  import "component/firewall"  (already imported in this package)
```

Same pattern for VPP:
```
plugins/iface/vpp/health.go:  init() { health.Register("vpp", checkVPPHealth) }
                               func checkVPPHealth() { ... dial /run/vpp/api.sock ... }
                               import "component/vpp"  (already imported in this package)
```

**Shared helpers:** `checkWarningCodes()` stays in `cmd/show/health_checks.go` as it is used by the remaining non-backend checks. `checkFirewallHealth` calls `checkWarningCodes()` after the audit; the moved version should inline the warning-code check (two codes: `firewall-stale-table`, `firewall-drift`) since exporting a helper for two string lookups is not worth the coupling. `checkVPPHealth` does not use `checkWarningCodes()` at all.

## Required Reading

### Source Files
- [ ] `internal/component/cmd/show/health_checks.go` - backend-specific health checks in wrong package
- [ ] `internal/component/cmd/show/health_checks_test.go` - existing tests for health checks
- [ ] `internal/component/cmd/show/show.go` lines 47-63 - health.Register calls in init()
- [ ] `internal/core/health/registry.go` - health.Register API (DefaultRegistry, CheckFunc type)
- [ ] `internal/plugins/iface/vpp/cmd_show.go` - prior art: VPP RPC handlers registered from backend
- [ ] `internal/plugins/firewall/nft/cmd_show.go` - prior art: nft RPC handlers registered from backend
- [ ] `internal/component/ike/engine/health.go` - prior art: IKE health check registered from component
- [ ] `internal/component/pki/health.go` - prior art: PKI health check registered from component

**Key insights:**
- `health.Register()` is safe to call from any package's `init()`, same as `pluginserver.RegisterRPCs()`
- IKE and PKI already register health checks from their own packages (confirmed via LSP findReferences on `health.Register`)
- `checkFirewallHealth` calls `firewall.AuditTables()` with a 1s timeout goroutine, then checks warning codes via `checkWarningCodes()`
- `checkVPPHealth` probes `/run/vpp/api.sock` with a 1s dial timeout; returns healthy if socket doesn't exist
- No backends (plugins/) currently register their own health checks; IKE and PKI are components, not plugins

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/cmd/show/health_checks.go`
  -> Constraint: 6 check functions. `checkFirewallHealth` imports `firewall`, calls `AuditTables()` with goroutine+timeout, then checks warning codes. `checkVPPHealth` hardcodes `/run/vpp/api.sock`, returns healthy if socket missing, dials with 1s timeout otherwise. `checkIfaceHealth` imports `iface`, calls `CheckAllInterfaceErrors()` with goroutine+timeout. `checkBGPHealth`, `checkFIBHealth`, `checkPluginHealth` use `report.Warnings()` only.
- [ ] `internal/component/cmd/show/health_checks_test.go`
  -> Constraint: Tests `checkBGPHealth`, `checkPluginHealth`, and registry aggregation. Tests call check functions directly. `checkFirewallHealth` is registered in test but only exercises the warning-code path (does not call `firewall.AuditTables()`).
- [ ] `internal/component/cmd/show/show.go` lines 47-63
  -> Constraint: init() registers 8 health checks. "firewall" and "vpp" are the backend-specific ones to move.
- [ ] `internal/core/health/registry.go`
  -> Constraint: `health.Register(name, CheckFunc)` appends to `DefaultRegistry`. `CheckFunc = func() (Status, string)`. Thread-safe. Called from init() is fine.

**Behavior to preserve:**
- Health check names unchanged ("firewall", "vpp")
- Check logic unchanged (same timeout, same socket path, same warning codes)
- `health.Check()` aggregation unchanged (returns same Report)
- `/health` HTTP endpoint behavior unchanged
- Existing tests continue to pass

**Behavior to change:**
- `checkFirewallHealth` moves from `cmd/show/health_checks.go` to `plugins/firewall/nft/health.go`
- `checkVPPHealth` moves from `cmd/show/health_checks.go` to `plugins/iface/vpp/health.go`
- `health.Register("firewall", ...)` moves from `cmd/show/show.go` init() to `plugins/firewall/nft/health.go` init()
- `health.Register("vpp", ...)` moves from `cmd/show/show.go` init() to `plugins/iface/vpp/health.go` init()
- `cmd/show/health_checks.go` loses its `firewall` import
- `cmd/show/health_checks.go` loses the `vppSocketPath` constant

## Data Flow (MANDATORY)

### Entry Point
- `health.Check()` iterates all registered checks and aggregates results
- `/health` HTTP handler calls `health.Check()`

### Transformation Path
1. Backend package init() calls `health.Register("firewall", checkFirewallHealth)`
2. `health.Check()` is called (by HTTP handler or programmatically)
3. Registry iterates checks, calls each CheckFunc
4. Backend-specific check executes (calls `firewall.AuditTables()` or dials VPP socket)
5. Results aggregated into `health.Report`

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| backend plugin -> health registry | `health.Register()` in `init()` | [ ] |
| health registry -> check function | `CheckFunc()` call during `Check()` | [ ] |
| check function -> backend component | Direct call (`firewall.AuditTables()`) or socket dial | [ ] |

### Integration Points
- `health.Register()` - same API, different call site
- `health.Check()` / `health.Report` - unchanged
- `/health` HTTP endpoint - unchanged

### Architectural Verification
- [ ] No bypassed layers
- [ ] No unintended coupling (goal: reduce coupling)
- [ ] No duplicated functionality
- [ ] Zero-copy preserved where applicable

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `/health` endpoint | -> | `health.Check()` -> `checkFirewallHealth` in `plugins/firewall/nft/` | `health_checks_test.go` (existing) + new test in `plugins/firewall/nft/` |
| `/health` endpoint | -> | `health.Check()` -> `checkVPPHealth` in `plugins/iface/vpp/` | new test in `plugins/iface/vpp/` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `checkFirewallHealth` in `plugins/firewall/nft/health.go` | Function exists, registers via `init()` + `health.Register("firewall", ...)`, calls `firewall.AuditTables()` with same timeout logic |
| AC-2 | `checkVPPHealth` in `plugins/iface/vpp/health.go` | Function exists, registers via `init()` + `health.Register("vpp", ...)`, probes `/run/vpp/api.sock` with same logic |
| AC-3 | `cmd/show/health_checks.go` no longer imports `firewall` | `grep 'component/firewall' health_checks.go` returns nothing |
| AC-4 | `cmd/show/health_checks.go` no longer contains `vppSocketPath` | `grep 'vppSocketPath' health_checks.go` returns nothing |
| AC-5 | `cmd/show/show.go` init() no longer registers "firewall" or "vpp" health checks | `grep 'health.Register.*firewall\|health.Register.*vpp' show.go` returns nothing |
| AC-6 | `health.Check()` still returns "firewall" and "vpp" components | Existing aggregation test passes; both names appear in report |
| AC-7 | All existing health check tests pass | `go test ./internal/component/cmd/show/ -run TestHealth` passes |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| TestFirewallHealthCheckWarningCodes | `plugins/firewall/nft/health_test.go` | checkFirewallHealth returns degraded on firewall-stale-table/firewall-drift warnings | [ ] |
| TestFirewallHealthCheckTimeout | `plugins/firewall/nft/health_test.go` | checkFirewallHealth returns degraded when AuditTables blocks past 1s | [ ] |
| TestVPPHealthCheckSocketMissing | `plugins/iface/vpp/health_test.go` | checkVPPHealth returns healthy when socket does not exist | [ ] |
| TestVPPHealthCheckSocketUnreachable | `plugins/iface/vpp/health_test.go` | checkVPPHealth returns down when socket exists but dial fails | [ ] |

### Boundary Tests
N/A - no numeric inputs.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| Existing health endpoint test | existing `.ci` if any | `/health` returns all components including "firewall" and "vpp" | [ ] |

### Interop Tests
N/A - internal restructuring, no protocol changes.

### Future
- When mixed backends land, test that both VPP and netlink health checks coexist in the same report

## Files to Modify

| File | Change |
|------|--------|
| `internal/component/cmd/show/health_checks.go` | Remove `checkFirewallHealth`, `checkVPPHealth`, `vppSocketPath` constant, `firewall` import |
| `internal/component/cmd/show/show.go` | Remove `health.Register("firewall", ...)` and `health.Register("vpp", ...)` from init() |
| `internal/component/cmd/show/health_checks_test.go` | Remove `checkFirewallHealth` from `TestHealthRegistryNewComponents` (it moved); adjust component count |

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | |
| CLI commands/flags | No | |
| CLI grammar (action before identifier) | No | |
| Editor autocomplete | No | |
| Functional test for new RPC/API | No | |
| Doctor check for runtime dependencies | No | |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | No | |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented? | No | |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | No | |

## Files to Create

| File | Purpose |
|------|---------|
| `internal/plugins/firewall/nft/health.go` | `checkFirewallHealth` + `init()` with `health.Register("firewall", ...)` |
| `internal/plugins/firewall/nft/health_test.go` | Tests for moved firewall health check |
| `internal/plugins/iface/vpp/health.go` | `checkVPPHealth` + `init()` with `health.Register("vpp", ...)` |
| `internal/plugins/iface/vpp/health_test.go` | Tests for moved VPP health check |

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | make ze-lint && make ze-unit-test && make ze-functional-test |
| 7. Critical review | Critical Review Checklist below |
| 8. Fix issues | Fix every issue from critical review |
| 9. Re-verify | Re-run stage 6 |
| 10. Repeat 7-9 | Until clean |
| 11. Deliverables review | Deliverables Checklist below |
| 12. Security review | Security Review Checklist below |
| 13. Re-verify | Re-run stage 6 |
| 14. Present summary | Executive Summary Report |

### Implementation Phases
Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Move `checkFirewallHealth`** to `plugins/firewall/nft/health.go` with `init()` registration. Inline the `checkWarningCodes` call (two codes). Write tests. Remove from `cmd/show/health_checks.go` and `show.go` init(). Verify `cmd/show/` no longer imports `firewall`.
2. **Move `checkVPPHealth`** to `plugins/iface/vpp/health.go` with `init()` registration. Write tests. Remove from `cmd/show/health_checks.go` and `show.go` init(). Verify `vppSocketPath` is gone.
3. **Update `health_checks_test.go`** to remove references to moved functions. Verify remaining tests pass.
4. **Verify integration**: `health.Check()` still returns both "firewall" and "vpp" components when both backends are loaded.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Import removed | `cmd/show/health_checks.go` does not import `component/firewall` |
| Constant removed | `vppSocketPath` gone from `cmd/show/` |
| Registration moved | `show.go` init() has no "firewall" or "vpp" health.Register calls |
| Logic preserved | Moved check functions are byte-for-byte identical logic |
| Tests pass | All existing health tests pass, new tests cover moved functions |
| No duplicate registration | "firewall" and "vpp" registered exactly once each |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| `plugins/firewall/nft/health.go` exists | `ls` |
| `plugins/iface/vpp/health.go` exists | `ls` |
| `cmd/show/` firewall import removed | `grep` |
| All tests pass | `make ze-unit-test` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Socket path not user-controlled | `vppSocketPath` is still a hardcoded constant in the new location |
| No new imports in cmd/show/ | Moving code out should reduce imports, not add any |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior |
| Lint failure | Fix inline; if architectural, back to DESIGN |
| Functional test fails | Check AC; if AC wrong, DESIGN; if correct, IMPLEMENT |
| Audit finds missing AC | Back to relevant phase and implement |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| Original spec assumed `vpp_trace.go` and `firewall.go` existed in `cmd/show/` | RPC handlers were already moved to backend plugins | `ls` showed files missing | Spec was rewritten to target remaining work (health checks) |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

- The `health.Register()` API is identical in shape to `pluginserver.RegisterRPCs()`: both accept registrations from any package's `init()`. No new infrastructure needed.
- IKE and PKI already use this exact pattern (`ike/engine/health.go`, `pki/health.go`), confirming it works.
- `checkFirewallHealth` calls `checkWarningCodes()` after the audit. The moved version should inline the two warning codes (`firewall-stale-table`, `firewall-drift`) since exporting a helper for two string lookups adds coupling for no benefit.
- `checkVPPHealth` has no dependency on any helper in `cmd/show/` and moves cleanly.
- The `firewall` package is already imported by `plugins/firewall/nft/cmd_show.go`, so the new `health.go` in the same package adds no new import edges.
- This pattern naturally supports mixed backends: each backend registers its own health check independently, and `health.Check()` aggregates all of them.

## RFC Documentation
N/A - no protocol work.

## Implementation Summary

### What Was Implemented
- (to be filled during implementation)

### Bugs Found/Fixed
- (to be filled during implementation)

### Documentation Updates
- (to be filled during implementation)

### Deviations from Plan
- (to be filled during implementation)

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Move checkFirewallHealth to nft plugin | [ ] | `plugins/firewall/nft/health.go` | |
| Move checkVPPHealth to vpp plugin | [ ] | `plugins/iface/vpp/health.go` | |
| Remove firewall import from cmd/show | [ ] | `cmd/show/health_checks.go` | |
| Remove vppSocketPath from cmd/show | [ ] | `cmd/show/health_checks.go` | |
| Remove health.Register calls from show.go | [ ] | `cmd/show/show.go` | |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | [ ] | | |
| AC-2 | [ ] | | |
| AC-3 | [ ] | | |
| AC-4 | [ ] | | |
| AC-5 | [ ] | | |
| AC-6 | [ ] | | |
| AC-7 | [ ] | | |

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

## Goal Validation (BLOCKING)
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| `cmd/show/` no longer imports `firewall` | grep | |
| `cmd/show/` no longer hardcodes VPP socket path | grep | |
| Health checks still work from backend packages | test | |
| Mixed-backend future unblocked | design | Backend-owned registration means each backend reports independently |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied
-

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
- [ ] AC-1..AC-7 all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end

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
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/753-backend-command-dispatch.md`
- [ ] Summary included in commit
