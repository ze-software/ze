# Spec: healthcheck-5-external

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | spec-healthcheck-4-ip-hooks-cli |
| Phase | - |
| Updated | 2026-04-03 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `plan/spec-healthcheck-0-umbrella.md` - umbrella design (internal vs external mode table)
4. `internal/component/bgp/plugins/healthcheck/` - all files from Phase 2-4
5. `internal/component/bgp/plugins/healthcheck/config.go` - config parsing
6. `internal/component/bgp/plugins/healthcheck/register.go` - CLIHandler (external mode entry)
7. `docs/architecture/api/process-protocol.md` - 5-stage protocol, Stage 2 configure

## Task

Enable the healthcheck plugin to run in external plugin mode (fork + TLS connect-back) in addition to the existing internal (goroutine) mode. The only behavioral difference: `ip-setup` is rejected at configuration time for external mode because external processes cannot call iface.AddAddress/RemoveAddress (requires in-process netlink access). All other healthcheck features (FSM, probes, hooks, MED mode, debounce, config reload, CLI) work identically in both modes.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/process-protocol.md` - 5-stage plugin protocol
  -> Constraint: external plugins use TLS connect-back via sdk.NewFromTLSEnv. Stage 2 delivers config.
- [ ] `.claude/patterns/plugin.md` - plugin structural template
  -> Constraint: CLIHandler is the external mode entry point. BaseConfig + RunPlugin pattern.
- [ ] `plan/spec-healthcheck-0-umbrella.md` - umbrella design
  -> Constraint: internal vs external mode table. ip-setup rejected for external at Stage 2 configure / config-verify.

### RFC Summaries (MUST for protocol work)
N/A -- operational tooling.

**Key insights:**
- Internal mode: `RunHealthcheckPlugin(conn net.Conn)` called by engine with net.Pipe
- External mode: `CLIHandler(args []string)` called as subprocess, uses sdk.NewFromTLSEnv
- Both modes use the same RunHealthcheckPlugin logic -- only the SDK connection differs
- ip-setup rejection must happen at config time (OnConfigure / config-verify), not at runtime
- External mode detection: sdk provides method or env var to distinguish modes

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/healthcheck/register.go` - CLIHandler registered with cli.BaseConfig + cli.RunPlugin. RunEngine set to RunHealthcheckPlugin.
  -> Constraint: CLIHandler is already wired for external mode. No new registration needed.
- [ ] `internal/component/bgp/plugins/healthcheck/config.go` - Parses probe config including ip-setup container.
  -> Decision: add validation in OnConfigure to reject ip-setup when running in external mode.
- [ ] `internal/component/bgp/plugins/healthcheck/healthcheck.go` - RunHealthcheckPlugin uses sdk.NewWithConn for internal mode.
  -> Decision: detect mode (internal vs external) and pass mode flag to config validation.

**Behavior to preserve:**
- All Phase 2-4 behavior unchanged for internal mode
- External mode gets identical FSM, probe, hook, MED, debounce, config reload behavior
- CLI commands (show/reset) work identically in external mode

**Behavior to change:**
- ip-setup block rejected at OnConfigure when plugin runs in external mode
- Clear error message explaining why ip-setup requires internal mode

## Data Flow (MANDATORY)

### Entry Point
- External mode: `ze plugin bgp-healthcheck` fork + TLS connect-back
- Config delivery: Stage 2 configure callback

### Transformation Path
1. Plugin starts in external mode (sdk.NewFromTLSEnv)
2. OnConfigure receives BGP config with healthcheck subtree
3. Config parser checks if ip-setup is present AND mode is external
4. If both: return error "ip-setup requires internal plugin mode (ip management needs in-process netlink access)"
5. If no ip-setup or internal mode: proceed normally

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config -> Healthcheck (external) | Stage 2 configure via TLS socket | [ ] |
| Validation error -> Engine | OnConfigure returns error, engine rejects config | [ ] |

### Integration Points
- `sdk.NewFromTLSEnv` - external mode SDK initialization
- `OnConfigure` callback - validates ip-setup not present in external mode
- `config-verify` callback - validates ip-setup not present in external mode (config reload)

### Architectural Verification
- [ ] No bypassed layers -- validation at config delivery time
- [ ] No unintended coupling -- only adds mode check to existing config validation
- [ ] No duplicated functionality -- reuses all Phase 2-4 code
- [ ] Zero-copy preserved -- N/A

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| External mode config with `ip-setup` | -> | OnConfigure validation error | `test/parse/healthcheck-ip-external.ci` |
| External mode config without `ip-setup` | -> | Normal operation | `test/plugin/healthcheck-external.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | External plugin mode with `ip-setup` block in config | OnConfigure returns validation error with clear message |
| AC-2 | External plugin mode without `ip-setup` | Plugin starts normally, probes run, watchdog commands dispatched |
| AC-3 | Internal plugin mode with `ip-setup` | Accepted (no change from Phase 4) |
| AC-4 | External mode: probe success/failure | FSM transitions and watchdog dispatch identical to internal mode |
| AC-5 | External mode: config reload adds ip-setup | config-verify rejects with error |
| AC-6 | External mode: show/reset commands | Work identically to internal mode |
| AC-7 | Error message for ip-setup rejection | Mentions "internal plugin mode" and "netlink" or "in-process" |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestExternalModeRejectsIPSetup` | `config_test.go` | AC-1: ip-setup + external mode = error |  |
| `TestExternalModeAcceptsNoIPSetup` | `config_test.go` | AC-2: no ip-setup + external mode = ok |  |
| `TestInternalModeAcceptsIPSetup` | `config_test.go` | AC-3: ip-setup + internal mode = ok |  |
| `TestExternalModeErrorMessage` | `config_test.go` | AC-7: error message is clear |  |

### Boundary Tests (MANDATORY for numeric inputs)
N/A -- no new numeric inputs.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `healthcheck-ip-external` | `test/parse/healthcheck-ip-external.ci` | External mode config with ip-setup, verify error |  |
| `healthcheck-external` | `test/plugin/healthcheck-external.ci` | External mode config without ip-setup, probe runs, watchdog commands dispatched |  |

### Future
- None -- this completes the healthcheck feature.

## Files to Modify
- `internal/component/bgp/plugins/healthcheck/config.go` - Add mode parameter to validation. Reject ip-setup in external mode.
- `internal/component/bgp/plugins/healthcheck/healthcheck.go` - Detect internal vs external mode, pass to config validation.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | No | - |
| CLI commands/flags | No | - |
| Editor autocomplete | No | - |
| Functional test | Yes | Two .ci tests above |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | - |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` -- note internal/external mode difference for healthcheck |
| 6 | Has a user guide page? | Yes | `docs/guide/healthcheck.md` -- add external mode section with ip-setup limitation |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented? | No | - |
| 10 | Test infrastructure changed? | No | - |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` -- note external mode support |
| 12 | Internal architecture changed? | No | - |

## Files to Create
- `test/parse/healthcheck-ip-external.ci`
- `test/plugin/healthcheck-external.ci`

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

1. **Phase: Mode detection + config validation** -- Detect internal vs external mode in RunHealthcheckPlugin. Pass mode to config validation. Reject ip-setup in external mode with clear error.
   - Tests: `TestExternalModeRejectsIPSetup`, `TestExternalModeAcceptsNoIPSetup`, `TestInternalModeAcceptsIPSetup`, `TestExternalModeErrorMessage`
   - Files: config.go, healthcheck.go, config_test.go
   - Verify: tests fail -> implement -> tests pass

2. **Phase: Functional tests** -- Create .ci tests for external mode with and without ip-setup.
   - Files: test/parse/healthcheck-ip-external.ci, test/plugin/healthcheck-external.ci
   - Verify: functional tests pass

3. **Full verification** -- `make ze-verify`
4. **Complete spec** -- audit, learned summary. Also write umbrella learned summary since this completes the feature.

### Critical Review Checklist (/implement stage 5)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1 through AC-7 has implementation with file:line |
| Correctness | External mode rejects ip-setup. Internal mode unchanged. Error message clear. |
| Mode detection | Reliable way to distinguish internal vs external (not a heuristic) |
| Config reload | External mode config reload also validates ip-setup rejection |
| Backward compat | All Phase 2-4 tests pass without modification |

### Deliverables Checklist (/implement stage 9)
| Deliverable | Verification method |
|-------------|---------------------|
| External mode rejects ip-setup | `test/parse/healthcheck-ip-external.ci` passes |
| External mode works without ip-setup | `test/plugin/healthcheck-external.ci` passes |
| All previous tests pass | `go test ./internal/component/bgp/plugins/healthcheck/...` |
| Error message is clear | `TestExternalModeErrorMessage` passes |

### Security Review Checklist (/implement stage 10)
| Check | What to look for |
|-------|-----------------|
| Mode detection spoofing | Ensure external process cannot claim to be internal to bypass ip-setup check |
| Validation completeness | Both OnConfigure (startup) and config-verify (reload) check ip-setup in external mode |

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
- [ ] AC-1..AC-7 all demonstrated
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
- [ ] Functional tests

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-healthcheck-5-external.md`
- [ ] Write umbrella learned summary to `plan/learned/NNN-healthcheck-0-umbrella.md`
- [ ] Summary included in commit
