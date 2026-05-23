# Spec: Backend-Dispatched Command Handlers

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | - |
| Updated | 2026-05-22 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/component/cmd/show/vpp_trace.go` - cross-dependency example
4. `internal/component/plugin/server/rpc_register.go` - current RegisterRPCs pattern
5. `internal/component/iface/backend.go` - Backend interface pattern

## Task

Decouple CLI command handlers from backend-specific packages by allowing backend plugins to register their own handler implementations for commands they support. Today, `cmd/show/` imports `vpp`, `iface`, `firewall`, `traffic`, `l2tp`, `host`, `ike/engine`, and `config/system` directly, making it a dependency magnet. Backend-specific handlers (e.g., VPP trace, firewall ruleset) should be registered by the backend that provides them, not by a central show package.

This complements `spec-backend-aware-completion.md`: completion filtering hides commands whose backend is absent; this spec ensures the handler itself only exists when the backend is loaded.

## Problem

`cmd/show/` currently has 12+ cross-component imports:

| Handler | Imports | Backend-specific? |
|---------|---------|-------------------|
| `vpp_trace.go` | `vpp` | Yes (VPP only) |
| `firewall.go` | `firewall` | Yes (nft only for ruleset) |
| `ip.go` | `iface` | No (both backends implement ListKernelRoutes) |
| `neighbors.go` | `iface` | No (both backends implement ListNeighbors) |
| `show.go` | `iface`, `host`, `l2tp`, `traffic` | Mixed |
| `interface_rate.go` | `iface` | No (dispatches through Backend interface) |
| `conntrack.go` | `config/system` | Partially (nft/conntrack specific) |
| `ipsec.go` | `ike/engine` | No (IKE is not backend-dispatched) |

**Categories:**
1. **Backend-dispatched via interface** (ip.go, neighbors.go, interface_rate.go): Already clean. Call `iface.ListKernelRoutes()` which dispatches through the Backend interface. Both netlink and VPP implement it. No change needed.
2. **Backend-specific, hardcoded** (vpp_trace.go, firewall.go): Import the backend package directly. Should move to the backend plugin's package and register from there.
3. **Component-specific, not backend** (ipsec.go, host.go): Import a component that has no backend abstraction. These are a different problem (component coupling, not backend coupling).

This spec addresses category 2: backend-specific handlers that should register from their backend plugin.

## Design Direction

**Handler registration from backend packages:**

Each backend plugin registers RPC handlers for commands it exclusively supports. The command YANG still defines the command shape. The handler lives in the backend's plugin package.

Current:
```
cmd/show/vpp_trace.go:  init() { RegisterRPCs("ze-show:vpp-trace-start", handler) }
                        import "component/vpp"
```

Proposed:
```
plugins/iface/vpp/cmd_trace.go:  init() { RegisterRPCs("ze-show:vpp-trace-start", handler) }
                                  import "component/vpp"  (same package or sibling)
```

The handler moves from `cmd/show/` to the VPP plugin package. The import stays local. When VPP is not compiled in, the handler does not exist.

**Shared argument parsing:** Commands like `show ip route` have argument parsing (--limit, CIDR validation) that both backends reuse. This parsing logic stays in a shared location (either `cmd/show/` as unexported helpers, or a new `cmd/show/args/` package). Backend handlers import the shared parsing, not the other way around.

## Required Reading

### Source Files
- [ ] `internal/component/cmd/show/vpp_trace.go` - VPP-specific handlers in wrong package
- [ ] `internal/component/cmd/show/firewall.go` - firewall-specific handlers
- [ ] `internal/component/cmd/show/show.go` - central show with many imports
- [ ] `internal/component/cmd/show/ip.go` - correctly dispatched through Backend interface
- [ ] `internal/component/plugin/server/rpc_register.go` - RegisterRPCs pattern
- [ ] `internal/component/plugin/server/handler.go` - RPCRegistration type
- [ ] `internal/plugins/iface/vpp/register.go` - VPP backend registration (where VPP handlers should live)
- [ ] `internal/plugins/firewall/nft/register.go` - nft backend registration

**Key insights:**
- (to be filled during research)

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/cmd/show/vpp_trace.go`
  -> Constraint: Registers 4 VPP RPCs (trace-start, trace-show, trace-clear, runtime). Imports `component/vpp` directly. Handlers call `vpp.TraceStart()`, `vpp.ShowRuntime()` etc.
- [ ] `internal/component/cmd/show/firewall.go`
  -> Constraint: Registers firewall-ruleset and firewall-group RPCs. Imports `component/firewall`. Handler calls `firewall.ActiveBackendName()` and rejects if not "nft".

**Behavior to preserve:**
- All RPC wire methods unchanged (same ze:command annotations in YANG)
- Handler function signatures unchanged (pluginserver.Handler)
- Argument parsing behavior unchanged
- Error messages unchanged

**Behavior to change:**
- VPP-specific handlers move from `cmd/show/` to `plugins/iface/vpp/`
- nft-specific handlers move from `cmd/show/` to `plugins/firewall/nft/`
- `cmd/show/` loses its imports of `vpp` and `firewall`

## Data Flow (MANDATORY)

### Entry Point
- RPC dispatch: WireMethod string -> pluginserver handler registry -> handler function

### Transformation Path
1. Command arrives at pluginserver.Dispatcher with WireMethod "ze-show:vpp-trace-start"
2. Dispatcher looks up handler in registeredRPCs map
3. Handler executes (today: in cmd/show/ importing vpp; proposed: in plugins/iface/vpp/ using local access)
4. Response returned

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| pluginserver -> handler | WireMethod dispatch | [ ] |
| handler -> backend | Direct call (vpp.TraceStart) or Backend interface (iface.ListKernelRoutes) | [ ] |

### Integration Points
- `pluginserver.RegisterRPCs` - same API, different call site
- `command.Node` in YANG command tree - unchanged

### Architectural Verification
- [ ] No bypassed layers
- [ ] No unintended coupling (goal: reduce coupling)
- [ ] No duplicated functionality
- [ ] Zero-copy preserved where applicable

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|

### Boundary Tests
N/A - no numeric inputs.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|

### Interop Tests
N/A - internal restructuring, no protocol changes.

### Future
- (to be filled during design)

## Files to Modify
- (to be filled during design)

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
| 12 | Internal architecture changed? | Yes | `docs/architecture/api/commands.md` - document backend-dispatched handler pattern |

## Files to Create
- (to be filled during design)

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

1. (to be filled during design)

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|

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

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

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

## Goal Validation (BLOCKING)
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|

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
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/753-backend-command-dispatch.md`
- [ ] Summary included in commit
