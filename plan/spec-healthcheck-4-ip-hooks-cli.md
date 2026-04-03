# Spec: healthcheck-4-ip-hooks-cli

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | spec-healthcheck-3-modes |
| Phase | - |
| Updated | 2026-04-03 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `plan/spec-healthcheck-0-umbrella.md` - umbrella design (IP management, hooks, CLI commands)
4. `internal/component/bgp/plugins/healthcheck/` - all files from Phase 2-3
5. `internal/component/iface/manage_linux.go` - AddAddress/RemoveAddress signatures
6. `internal/component/bgp/plugins/healthcheck/fsm.go` - state transitions
7. `internal/component/bgp/plugins/healthcheck/healthcheck.go` - probe manager

## Task

Add IP management (loopback VIP add/remove), state-transition hooks (on-up/on-down/on-disabled/on-change with 30s timeout + process group kill), and CLI commands (show/reset) to the healthcheck plugin. After this phase, healthcheck is feature-complete for internal (goroutine) mode.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - component architecture
  -> Constraint: components register at startup via init()
- [ ] `plan/spec-healthcheck-0-umbrella.md` - umbrella design
  -> Constraint: IP management section, hook execution section, CLI commands section
- [ ] `internal/component/iface/manage_linux.go` - AddAddress/RemoveAddress
  -> Constraint: standalone functions, no receiver. Signature: func AddAddress(ifaceName, cidr string) error

### RFC Summaries (MUST for protocol work)
N/A -- operational tooling.

**Key insights:**
- iface.AddAddress/RemoveAddress are standalone package-level functions (no interface type, no receiver)
- Plugin defines local IPManager interface for test injection (two methods)
- IP startup: all IPs added at probe startup (before first check), regardless of dynamic setting
- Dynamic mode: IPs removed on DOWN/DISABLED, restored on UP. Non-dynamic: IPs stay except EXIT.
- Hooks fire on state changes only, not count increments (RISING->RISING does NOT fire)
- Hook order: state-specific first, then on-change. Within each, config order.
- Hooks don't block FSM (run in goroutines). 30s timeout + process group kill.
- CLI: show bgp healthcheck (all probes), show bgp healthcheck <name> (detail), reset bgp healthcheck <name>
- Reset: withdraw current route, reset FSM to INIT, immediate re-check. Error if DISABLED.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/iface/manage_linux.go` - AddAddress(ifaceName, cidr string) error and RemoveAddress(ifaceName, cidr string) error. Package-level functions using netlink AddrAdd/AddrDel. Validate interface name (1-15 chars). Accept IPv4/IPv6 CIDR.
  -> Constraint: direct import from healthcheck plugin. No abstraction layer.
  -> Decision: local IPManager interface in healthcheck for test injection.
- [ ] `internal/component/bgp/plugins/healthcheck/fsm.go` - FSM from Phase 2-3 with all 8 states wired.
  -> Decision: hooks fire via callback after trigger() returns a state change.
- [ ] `internal/component/bgp/plugins/healthcheck/healthcheck.go` - Probe manager from Phase 2-3.
  -> Decision: add IP management and hook dispatch to probe goroutine lifecycle.

**Behavior to preserve:**
- All Phase 2-3 behavior: FSM, probe execution, MED mode, debounce, fast-interval, config reload, disable
- iface.AddAddress/RemoveAddress behavior unchanged

**Behavior to change:**
- New `ip-setup` container in YANG with interface, dynamic, ip leaf-list
- New hook leaf-lists: on-up, on-down, on-disabled, on-change
- Probe goroutine manages IPs at startup and on state transitions
- Hooks execute asynchronously on state changes
- CLI commands registered for show/reset

## Data Flow (MANDATORY)

### Entry Point
- FSM state transition -> hook dispatch + IP management
- CLI command -> probe status query / FSM reset

### Transformation Path
1. FSM transition fires -> determine if state changed (not just count increment)
2. If state changed and ip-setup configured: add/remove IPs per state actions table
3. If state changed and hooks configured: execute hooks in goroutines
4. CLI show -> read probe state -> return JSON
5. CLI reset -> withdraw current route, reset FSM, trigger immediate check

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Healthcheck -> Iface | iface.AddAddress/RemoveAddress (direct import, internal mode only) | [ ] |
| CLI -> Healthcheck | Command dispatch (show/reset RPC) | [ ] |
| Healthcheck -> Shell | exec.CommandContext for hooks (30s timeout) | [ ] |

### Integration Points
- `iface.AddAddress` / `iface.RemoveAddress` - standalone functions, direct import
- Command registration in DeclareRegistrationInput for show/reset
- Hook execution via exec.CommandContext with process group + timeout

### Architectural Verification
- [ ] No bypassed layers -- IP management via iface package functions
- [ ] No unintended coupling -- local IPManager interface for test injection
- [ ] No duplicated functionality -- reuses iface package
- [ ] Zero-copy preserved -- N/A

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `show bgp healthcheck` CLI command | -> | Returns probe status table | `test/plugin/healthcheck-show.ci` |
| `reset bgp healthcheck <name>` while UP | -> | Withdraws, resets to INIT | `test/plugin/healthcheck-show.ci` |
| `reset bgp healthcheck <name>` while DISABLED | -> | Returns error | `test/plugin/healthcheck-show.ci` |
| Config with ip-setup, probe starts | -> | IPs added to interface | Unit test (requires CAP_NET_ADMIN for functional) |
| on-up hook, state transitions to UP | -> | Hook command executed with STATE=UP | Unit test (hook execution) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `ip-setup { interface lo; ip 10.0.0.1/32; }`, probe starts | 10.0.0.1/32 added to lo at startup (before first check) |
| AC-2 | `ip-setup { dynamic true; }`, state DOWN | IPs removed from interface |
| AC-3 | `ip-setup { dynamic true; }`, state UP (after DOWN) | IPs restored on interface |
| AC-4 | `ip-setup { dynamic false; }` (default), state DOWN | IPs remain on interface |
| AC-5 | State transitions to EXIT | All managed IPs removed from interface |
| AC-6 | `ip-setup` with multiple IPs in leaf-list | All IPs added/removed as a group |
| AC-7 | `on-up "echo up"`, state transitions to UP | Hook executed with env STATE=UP |
| AC-8 | `on-change "echo change"`, any state transition | Hook executed with env STATE=<new-state> |
| AC-9 | State-specific + on-change hooks defined | State-specific hooks run first, then on-change |
| AC-10 | `on-up` has multiple entries (leaf-list) | All hooks execute in config order |
| AC-11 | Hook command hangs > 30 seconds | Process group killed, warning logged, FSM not blocked |
| AC-12 | Hook stdout/stderr | Discarded (not captured) |
| AC-13 | RISING -> RISING (count increment, same state) | on-change does NOT fire |
| AC-14 | `show bgp healthcheck` | Returns JSON with all probe names, states, groups, check counts |
| AC-15 | `show bgp healthcheck <name>` | Returns JSON with FSM state, count, thresholds, IPs, metrics |
| AC-16 | `reset bgp healthcheck <name>` while UP | Withdraws current route, resets FSM to INIT, immediate re-check |
| AC-17 | `reset bgp healthcheck <name>` while DISABLED | Returns error, probe stays DISABLED |
| AC-18 | `ip-setup { dynamic true; }`, DISABLED state | IPs removed from interface |
| AC-19 | `interval 0`, transition to END | No hooks fire |
| AC-20 | Hook failure (non-zero exit) | Warning logged, FSM state unaffected |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestIPSetupStartup` | `ip_test.go` | AC-1: IPs added at probe startup |  |
| `TestIPDynamicRemoveOnDown` | `ip_test.go` | AC-2: dynamic=true removes IPs on DOWN |  |
| `TestIPDynamicRestoreOnUp` | `ip_test.go` | AC-3: dynamic=true restores IPs on UP |  |
| `TestIPStaticKeepOnDown` | `ip_test.go` | AC-4: dynamic=false keeps IPs on DOWN |  |
| `TestIPRemoveOnExit` | `ip_test.go` | AC-5: all IPs removed on EXIT |  |
| `TestIPMultiple` | `ip_test.go` | AC-6: multiple IPs handled as group |  |
| `TestHookOnUp` | `hooks_test.go` | AC-7: on-up hook executed with STATE=UP |  |
| `TestHookOnChange` | `hooks_test.go` | AC-8: on-change fires on any transition |  |
| `TestHookOrder` | `hooks_test.go` | AC-9: state-specific before on-change |  |
| `TestHookMultiple` | `hooks_test.go` | AC-10: leaf-list hooks execute in order |  |
| `TestHookTimeout` | `hooks_test.go` | AC-11: 30s timeout kills process group |  |
| `TestHookNoOutput` | `hooks_test.go` | AC-12: stdout/stderr discarded |  |
| `TestHookNoFireOnCountIncrement` | `hooks_test.go` | AC-13: RISING->RISING no hook |  |
| `TestShowAll` | `healthcheck_test.go` | AC-14: show bgp healthcheck returns all probes |  |
| `TestShowSingle` | `healthcheck_test.go` | AC-15: show bgp healthcheck <name> returns detail |  |
| `TestResetUp` | `healthcheck_test.go` | AC-16: reset withdraws, resets to INIT |  |
| `TestResetDisabled` | `healthcheck_test.go` | AC-17: reset while DISABLED returns error |  |
| `TestIPDynamicRemoveOnDisabled` | `ip_test.go` | AC-18: dynamic=true removes IPs on DISABLED |  |
| `TestHookNoFireOnEnd` | `hooks_test.go` | AC-19: END state does not fire hooks |  |
| `TestHookFailureNoFSMEffect` | `hooks_test.go` | AC-20: hook failure only logs warning |  |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Hook timeout | 30s (fixed) | N/A | N/A | N/A |
| ip leaf-list | 1+ entries | N/A | 0 (empty) | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `healthcheck-show` | `test/plugin/healthcheck-show.ci` | Start healthcheck, query `show bgp healthcheck`, verify JSON response includes probe state. Test `reset bgp healthcheck <name>`. |  |

### Future
- External plugin mode (ip-setup rejection) -- Phase 5

## Files to Modify
- `internal/component/bgp/plugins/healthcheck/healthcheck.go` - Add CLI command handlers (show/reset). Wire IP management and hooks into probe lifecycle.
- `internal/component/bgp/plugins/healthcheck/fsm.go` - Add hook dispatch callback. Ensure state change vs count increment distinction.
- `internal/component/bgp/plugins/healthcheck/register.go` - Register show/reset commands in DeclareRegistrationInput.
- `internal/component/bgp/plugins/healthcheck/schema/ze-healthcheck-conf.yang` - Add ip-setup container, hook leaf-lists.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new containers/leaves) | Yes | `ze-healthcheck-conf.yang` |
| CLI commands | Yes | show/reset registered in registration |
| Editor autocomplete | Yes (YANG-driven) | - |
| Functional test | Yes | `test/plugin/healthcheck-show.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No (extends existing) | - |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md` -- add ip-setup, hooks |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` -- add show/reset bgp healthcheck |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No (extends) | - |
| 6 | Has a user guide page? | Yes | `docs/guide/healthcheck.md` -- add IP management, hooks, CLI sections |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented? | No | - |
| 10 | Test infrastructure changed? | No | - |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | No | - |

## Files to Create
- `internal/component/bgp/plugins/healthcheck/ip.go` - IP management via iface, local IPManager interface
- `internal/component/bgp/plugins/healthcheck/hooks.go` - Hook execution (30s timeout, process group kill)
- `internal/component/bgp/plugins/healthcheck/ip_test.go`
- `internal/component/bgp/plugins/healthcheck/hooks_test.go`
- `test/plugin/healthcheck-show.ci`

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Implement (TDD) | Implementation phases below |
| 4. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 5. Critical review | Critical Review Checklist below |
| 6. Fix issues | Fix every issue |
| 7. Re-verify | Re-run stage 4 |
| 8. Repeat 5-7 | Max 2 passes |
| 9. Deliverables review | Deliverables Checklist below |
| 10. Security review | Security Review Checklist below |
| 11. Re-verify | Re-run stage 4 |
| 12. Present summary | Executive Summary Report |

### Implementation Phases

1. **Phase: IP management** -- Create ip.go with local IPManager interface wrapping iface.AddAddress/RemoveAddress. Track managed IPs per probe. Add/remove on state transitions per state actions table.
   - Tests: all ip_test.go tests
   - Files: ip.go, ip_test.go, ze-healthcheck-conf.yang
   - Verify: tests pass

2. **Phase: Hook execution** -- Create hooks.go. Shell command via exec.CommandContext with 30s timeout. Process group isolation. Async (goroutine). STATE env var. Fire on state changes only.
   - Tests: all hooks_test.go tests
   - Files: hooks.go, hooks_test.go, ze-healthcheck-conf.yang
   - Verify: tests pass

3. **Phase: Wire IP + hooks into probe lifecycle** -- Modify probe goroutine to call IP management at startup and on transitions. Call hook dispatch on state changes. Ensure RISING->RISING doesn't fire.
   - Tests: integration of ip + hooks with FSM
   - Files: healthcheck.go, fsm.go
   - Verify: tests pass

4. **Phase: CLI commands** -- Register show/reset commands. Implement handlers querying probe state. Reset: withdraw, reset FSM, immediate check. Error on DISABLED.
   - Tests: `TestShowAll`, `TestShowSingle`, `TestResetUp`, `TestResetDisabled`
   - Files: healthcheck.go, register.go, healthcheck_test.go
   - Verify: tests pass

5. **Phase: Functional test** -- Create healthcheck-show.ci.
   - Files: test/plugin/healthcheck-show.ci
   - Verify: functional test passes

6. **Full verification** -- `make ze-verify`
7. **Complete spec** -- audit, learned summary.

### Critical Review Checklist (/implement stage 5)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1 through AC-20 has implementation with file:line |
| IP lifecycle | IPs added at startup. Dynamic removes on DOWN/DISABLED, restores on UP. Static keeps. EXIT removes all. |
| Hook dispatch | Fires on state changes only. State-specific before on-change. Config order. Async. 30s timeout. |
| CLI wiring | show/reset commands registered, routed to healthcheck, return correct JSON |
| Reset semantics | Withdraw first, then INIT. Error if DISABLED. Immediate re-check after reset. |
| Process isolation | Hooks use Setpgid + SIGKILL(-pid). No zombies (Wait after kill). |

### Deliverables Checklist (/implement stage 9)
| Deliverable | Verification method |
|-------------|---------------------|
| IP management works | `TestIPSetupStartup`, `TestIPDynamicRemoveOnDown` pass |
| Hooks fire correctly | `TestHookOnUp`, `TestHookOrder`, `TestHookTimeout` pass |
| CLI show works | `test/plugin/healthcheck-show.ci` passes |
| CLI reset works | `TestResetUp`, `TestResetDisabled` pass |
| All Phase 2-3 tests still pass | `go test ./internal/component/bgp/plugins/healthcheck/...` |

### Security Review Checklist (/implement stage 10)
| Check | What to look for |
|-------|-----------------|
| Shell injection via hooks | on-up/on-down/on-disabled/on-change values passed to /bin/sh -c. Admin-controlled. Same threat model as probe command. Process group + 30s timeout. |
| IP management privilege | iface.AddAddress requires CAP_NET_ADMIN. Ze already has it. No escalation. |
| Hook zombie processes | Verify cmd.Wait() called after process group kill |
| Hook goroutine leak | Verify hook goroutines bounded (one per hook per transition, not accumulating) |
| Reset race condition | Verify reset holds probe lock during withdraw + FSM reset sequence |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Lint failure | Fix inline |
| Functional test fails | Check AC |
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
- [ ] AC-1..AC-20 all demonstrated
- [ ] Wiring Test table complete
- [ ] `make ze-test` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end
- [ ] Critical Review passes

### Quality Gates
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
- [ ] Boundary tests
- [ ] Functional tests

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-healthcheck-4-ip-hooks-cli.md`
- [ ] Summary included in commit
