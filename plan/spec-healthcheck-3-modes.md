# Spec: healthcheck-3-modes

| Field | Value |
|-------|-------|
| Status | design |
| Depends | spec-healthcheck-2-core |
| Phase | - |
| Updated | 2026-04-03 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `plan/spec-healthcheck-0-umbrella.md` - umbrella design (FSM state actions, debounce semantics, config reload)
4. `internal/component/bgp/plugins/healthcheck/` - all files from Phase 2
5. `internal/component/bgp/plugins/healthcheck/fsm.go` - FSM transitions
6. `internal/component/bgp/plugins/healthcheck/config.go` - probe config parsing
7. `internal/component/bgp/plugins/healthcheck/healthcheck.go` - probe manager lifecycle

## Task

Add MED mode (default behavior), debounce, fast-interval, exclusive group validation, config reload lifecycle, and admin disable toggle to the healthcheck plugin. After this phase, healthcheck supports both withdraw-on-down and MED mode, probes can be reconfigured at runtime, and the admin can disable/re-enable probes via config.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - component architecture
  -> Constraint: config reload via two-phase verify/apply protocol
- [ ] `docs/architecture/api/process-protocol.md` - config reload protocol
  -> Constraint: config-verify (Phase 1) for validation, config-apply (Phase 2) for activation
- [ ] `plan/spec-healthcheck-0-umbrella.md` - umbrella design
  -> Constraint: FSM state actions table, debounce+MED interaction, config reload lifecycle table

### RFC Summaries (MUST for protocol work)
N/A -- operational tooling.

**Key insights:**
- Default behavior is MED mode (withdraw-on-down false): UP announces with up-metric, DOWN re-announces with down-metric, DISABLED re-announces with disabled-metric
- Withdraw-on-down true: UP announces with up-metric, DOWN/DISABLED withdraw
- Debounce false (default): dispatch every interval even if state unchanged. MED path bypasses watchdog dedup, so repeated announce with same MED is harmless.
- Debounce true: dispatch only on state changes
- Fast-interval: used during RISING/FALLING states instead of normal interval
- Exclusive group: YANG `unique` constraint on group leaf prevents two probes sharing a group
- Config reload: compare new config struct against running probes. Deconfigure removed, reconfigure changed, start new.
- Disable toggle: `disable true` -> immediate DISABLED (no deconfigure). `disable false` -> INIT.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/healthcheck/fsm.go` - 8-state FSM from Phase 2. States INIT/RISING/UP/FALLING/DOWN with trigger() shortcut. DISABLED/EXIT/END states exist but only EXIT is wired (shutdown).
  -> Decision: wire DISABLED state transitions. Wire END state for interval=0.
- [ ] `internal/component/bgp/plugins/healthcheck/config.go` - Parses probe config from YANG tree. Fields: command, group, interval, timeout, rise, fall, up-metric, withdraw-on-down.
  -> Decision: add down-metric, disabled-metric, debounce, fast-interval, disable leaves.
- [ ] `internal/component/bgp/plugins/healthcheck/announce.go` - Dispatches watchdog announce/withdraw. Currently only withdraw-on-down path.
  -> Decision: add MED mode dispatch (announce with down-metric/disabled-metric).
- [ ] `internal/component/bgp/plugins/healthcheck/healthcheck.go` - RunHealthcheckPlugin with OnConfigure. Starts probe goroutines.
  -> Decision: add config reload handler (compare running probes, deconfigure/reconfigure/start).

**Behavior to preserve:**
- All Phase 2 behavior: probe execution, FSM transitions, withdraw-on-down mode
- Plugin registration unchanged
- YANG schema extended (not replaced)

**Behavior to change:**
- MED mode dispatch: DOWN dispatches `watchdog announce <group> med <down-metric>`, DISABLED dispatches `watchdog announce <group> med <disabled-metric>`
- Debounce: when true, skip dispatch if state unchanged since last dispatch
- Fast-interval: RISING/FALLING states use fast-interval timer, others use interval
- DISABLED state: `disable true` in config -> immediate DISABLED, check command NOT executed
- DISABLED -> INIT: `disable false` in config reload -> resume from INIT
- Config reload: deconfigure removed probes (EXIT), reconfigure changed probes (EXIT + new INIT), start new probes
- Exclusive group: YANG `unique` constraint on group leaf + Go validation rejects "med" as group name
- END state: interval=0 -> one check, announce/withdraw, dormant (no hooks, no further checks)

## Data Flow (MANDATORY)

### Entry Point
- Config reload: new healthcheck config via config-verify/config-apply
- Timer tick: interval or fast-interval depending on state
- Disable toggle: `disable` leaf change in config reload

### Transformation Path
1. Config reload -> OnConfigure with new BGP tree -> extract healthcheck subtree
2. Compare new ProbeConfig set against running probes (struct equality)
3. Deconfigure removed probes: cancel context -> probe transitions to EXIT -> withdraw routes
4. Reconfigure changed probes: deconfigure old + start new from INIT
5. Start new probes from INIT
6. Disable toggle: probe receives updated config -> immediate DISABLED or INIT transition
7. MED mode: DOWN state -> `watchdog announce <group> med <down-metric>` (re-announce, not withdraw)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config reload -> Healthcheck | config-verify/config-apply callbacks | [ ] |
| Healthcheck -> Watchdog | DispatchCommand("watchdog announce <group> med <down-metric>") | [ ] |

### Integration Points
- `p.OnConfigVerify` - validates exclusive group constraint, rejects "med" group name
- `p.OnConfigApply` - triggers probe lifecycle (deconfigure/reconfigure/start)
- FSM DISABLED state - check command not executed, probe sleeps on interval

### Architectural Verification
- [ ] No bypassed layers -- config reload through SDK callbacks
- [ ] No unintended coupling -- healthcheck manages its own probe lifecycle
- [ ] No duplicated functionality -- reuses Phase 2 FSM and dispatch
- [ ] Zero-copy preserved -- N/A

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Config with `withdraw-on-down false`, probe fails | -> | MED mode: `watchdog announce <group> med <down-metric>` | `test/plugin/healthcheck-med-mode.ci` |
| Config reload removes a probe | -> | Probe deconfigured, routes withdrawn | `test/plugin/healthcheck-deconfigure.ci` |
| Config reload with `disable true` | -> | Probe transitions to DISABLED | `test/plugin/healthcheck-deconfigure.ci` |
| Config reload with `disable false` | -> | Probe transitions DISABLED -> INIT | `test/plugin/healthcheck-deconfigure.ci` |
| Two probes with same group value | -> | Config validation error | `test/parse/healthcheck-duplicate-group.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `withdraw-on-down false` (default), probe fails, fall met | `watchdog announce <group> med <down-metric>` dispatched (not withdraw) |
| AC-2 | `withdraw-on-down false`, config reload with `disable true` | `watchdog announce <group> med <disabled-metric>` dispatched |
| AC-3 | `withdraw-on-down true`, config reload with `disable true` | `watchdog withdraw <group>` dispatched |
| AC-4 | `debounce true`, state unchanged between checks | No watchdog command dispatched |
| AC-5 | `debounce false` (default), state unchanged (UP) | `watchdog announce <group> med <up-metric>` re-dispatched every interval |
| AC-6 | FSM in RISING or FALLING state | Timer uses fast-interval (default 1s), not interval |
| AC-7 | FSM in UP or DOWN state | Timer uses interval (default 5s) |
| AC-8 | Two probes with same `group` value | YANG validation rejects config |
| AC-9 | `group` value is literal `med` | Go-level config validation error with clear message |
| AC-10 | Config reload removes a probe | Probe transitions to EXIT, routes withdrawn, goroutine stopped |
| AC-11 | Config reload changes probe `command` | Old probe deconfigured, new probe starts from INIT |
| AC-12 | Config reload with `disable false` (was `disable true`) | Probe transitions from DISABLED to INIT, resumes checking |
| AC-13 | Probe in DISABLED state | Check command NOT executed. Probe sleeps on interval timer only. |
| AC-14 | `interval 0` (single check mode) | One check, announce/withdraw, transition to END. No further checks. |
| AC-15 | `interval 0`, transition to END | No hooks fire (END early return) |
| AC-16 | Config reload: probe config unchanged | No action (probe keeps running) |
| AC-17 | `debounce false`, `withdraw-on-down true`, state unchanged (UP) | `watchdog announce <group> med <up-metric>` re-dispatched (MED path bypasses dedup) |
| AC-18 | Probe starts with `disable true` in initial config | Probe enters DISABLED directly, skips INIT |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestMEDModeDown` | `announce_test.go` | AC-1: DOWN with withdraw-on-down false dispatches announce with down-metric |  |
| `TestMEDModeDisabled` | `announce_test.go` | AC-2: DISABLED with withdraw-on-down false dispatches announce with disabled-metric |  |
| `TestWithdrawModeDisabled` | `announce_test.go` | AC-3: DISABLED with withdraw-on-down true dispatches withdraw |  |
| `TestDebounceTrue` | `healthcheck_test.go` | AC-4: debounce=true skips dispatch on unchanged state |  |
| `TestDebounceFalse` | `healthcheck_test.go` | AC-5: debounce=false re-dispatches every interval |  |
| `TestFastInterval` | `healthcheck_test.go` | AC-6, AC-7: RISING/FALLING use fast-interval, others use interval |  |
| `TestExclusiveGroup` | `config_test.go` | AC-8: duplicate group rejected |  |
| `TestGroupNameMed` | `config_test.go` | AC-9: "med" as group name rejected |  |
| `TestLifecycleDeconfigure` | `lifecycle_test.go` | AC-10: removed probe transitions to EXIT |  |
| `TestLifecycleReconfigure` | `lifecycle_test.go` | AC-11: changed probe deconfigured + restarted |  |
| `TestLifecycleDisableToggle` | `lifecycle_test.go` | AC-12, AC-18: disable true -> DISABLED, false -> INIT |  |
| `TestFSMDisabledNoCheck` | `fsm_test.go` | AC-13: DISABLED state does not execute check command |  |
| `TestFSMEndState` | `fsm_test.go` | AC-14, AC-15: interval=0 -> END, no hooks |  |
| `TestLifecycleUnchanged` | `lifecycle_test.go` | AC-16: unchanged config -> no action |  |
| `TestDebounceWithdrawModeUp` | `healthcheck_test.go` | AC-17: debounce false + withdraw-on-down true + UP re-dispatches |  |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| down-metric | 0 - 4294967295 | 4294967295 | N/A | 4294967296 |
| disabled-metric | 0 - 4294967295 | 4294967295 | N/A | 4294967296 |
| fast-interval | 1 - 3600 | 3600 | 0 | 3601 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `healthcheck-med-mode` | `test/plugin/healthcheck-med-mode.ci` | Probe fails with withdraw-on-down false, peer receives UPDATE with down-metric MED |  |
| `healthcheck-deconfigure` | `test/plugin/healthcheck-deconfigure.ci` | Config reload removes probe, peer receives WITHDRAW. Config reload re-enables, probe restarts. |  |
| `healthcheck-duplicate-group` | `test/parse/healthcheck-duplicate-group.ci` | Two probes with same group rejected at parse time |  |

### Future
- IP management -- Phase 4
- Hooks -- Phase 4
- CLI commands -- Phase 4
- External plugin mode -- Phase 5

## Files to Modify
- `internal/component/bgp/plugins/healthcheck/config.go` - Add down-metric, disabled-metric, debounce, fast-interval, disable leaves. Add group uniqueness validation. Add "med" group name rejection.
- `internal/component/bgp/plugins/healthcheck/fsm.go` - Wire DISABLED state transitions. Wire END state for interval=0.
- `internal/component/bgp/plugins/healthcheck/announce.go` - Add MED mode dispatch (announce with down-metric/disabled-metric).
- `internal/component/bgp/plugins/healthcheck/healthcheck.go` - Add config reload handler, debounce logic, fast-interval timer switching.
- `internal/component/bgp/plugins/healthcheck/schema/ze-healthcheck-conf.yang` - Add down-metric, disabled-metric, debounce, fast-interval, disable leaves.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new leaves) | Yes | `ze-healthcheck-conf.yang` |
| CLI commands/flags | No (Phase 4) | - |
| Editor autocomplete | Yes (YANG-driven) | - |
| Functional test for new behavior | Yes | Three .ci tests above |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No (extends Phase 2) | - |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md` -- add MED mode, debounce, fast-interval, disable leaves |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No (extends Phase 2) | - |
| 6 | Has a user guide page? | Yes | `docs/guide/healthcheck.md` -- add MED mode, debounce, config reload sections |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented? | No | - |
| 10 | Test infrastructure changed? | No | - |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | No | - |

## Files to Create
- `internal/component/bgp/plugins/healthcheck/lifecycle.go` - Config reload: deconfigure/reconfigure/start/disable toggle
- `internal/component/bgp/plugins/healthcheck/lifecycle_test.go`
- `test/plugin/healthcheck-med-mode.ci`
- `test/plugin/healthcheck-deconfigure.ci`
- `test/parse/healthcheck-duplicate-group.ci`

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
| 8. Repeat 5-7 | Max 2 review passes |
| 9. Deliverables review | Deliverables Checklist below |
| 10. Security review | Security Review Checklist below |
| 11. Re-verify | Re-run stage 4 |
| 12. Present summary | Executive Summary Report |

### Implementation Phases

1. **Phase: YANG + config extension** -- Add new leaves to YANG. Extend config parser. Add group uniqueness + "med" name validation.
   - Tests: `TestExclusiveGroup`, `TestGroupNameMed`, config parse tests
   - Files: ze-healthcheck-conf.yang, config.go, config_test.go
   - Verify: tests fail -> implement -> tests pass

2. **Phase: MED mode dispatch** -- Extend announce logic for MED mode (down-metric, disabled-metric).
   - Tests: `TestMEDModeDown`, `TestMEDModeDisabled`, `TestWithdrawModeDisabled`
   - Files: announce.go, announce_test.go
   - Verify: tests pass

3. **Phase: FSM extensions** -- Wire DISABLED state (no check execution). Wire END state (interval=0). Fast-interval timer.
   - Tests: `TestFSMDisabledNoCheck`, `TestFSMEndState`, `TestFastInterval`
   - Files: fsm.go, fsm_test.go, healthcheck.go
   - Verify: tests pass

4. **Phase: Debounce** -- Track last-dispatched state. Skip dispatch when unchanged and debounce=true.
   - Tests: `TestDebounceTrue`, `TestDebounceFalse`, `TestDebounceWithdrawModeUp`
   - Files: healthcheck.go, healthcheck_test.go
   - Verify: tests pass

5. **Phase: Config reload lifecycle** -- Compare running probes against new config. Deconfigure/reconfigure/start. Disable toggle.
   - Tests: `TestLifecycleDeconfigure`, `TestLifecycleReconfigure`, `TestLifecycleDisableToggle`, `TestLifecycleUnchanged`
   - Files: lifecycle.go, lifecycle_test.go
   - Verify: tests pass

6. **Phase: Functional tests** -- Create .ci tests for MED mode, deconfigure, duplicate group.
   - Files: test/plugin/healthcheck-med-mode.ci, test/plugin/healthcheck-deconfigure.ci, test/parse/healthcheck-duplicate-group.ci
   - Verify: functional tests pass

7. **Full verification** -- `make ze-verify`
8. **Complete spec** -- audit, learned summary.

### Critical Review Checklist (/implement stage 5)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1 through AC-18 has implementation with file:line |
| Correctness | MED mode dispatches correct metric per state. Debounce skips correctly. Fast-interval used in RISING/FALLING only. |
| Config reload | Deconfigure kills goroutine, withdraws routes. Reconfigure restarts from INIT. Disable toggle immediate. |
| Group uniqueness | YANG unique constraint works. "med" rejected at Go level. |
| DISABLED behavior | Check command NOT executed. Probe sleeps. Re-enable transitions to INIT. |
| Backward compat | Phase 2 behavior unchanged for withdraw-on-down true configs |

### Deliverables Checklist (/implement stage 9)
| Deliverable | Verification method |
|-------------|---------------------|
| MED mode works | `test/plugin/healthcheck-med-mode.ci` passes |
| Config reload works | `test/plugin/healthcheck-deconfigure.ci` passes |
| Group uniqueness works | `test/parse/healthcheck-duplicate-group.ci` passes |
| Debounce works | `TestDebounceTrue` and `TestDebounceFalse` pass |
| Fast-interval works | `TestFastInterval` passes |
| DISABLED no-check | `TestFSMDisabledNoCheck` passes |
| All Phase 2 tests still pass | `go test ./internal/component/bgp/plugins/healthcheck/...` |

### Security Review Checklist (/implement stage 10)
| Check | What to look for |
|-------|-----------------|
| Config reload race | Verify probe manager uses mutex for probe map access during config reload |
| Goroutine leak on deconfigure | Verify context cancellation stops probe goroutine, cmd.Wait() called |
| DISABLED state check bypass | Verify DISABLED state does NOT execute shell command (no exec.CommandContext) |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read FSM/dispatch from Phase 2 |
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
- [ ] AC-1..AC-18 all demonstrated
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
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-healthcheck-3-modes.md`
- [ ] Summary included in commit
