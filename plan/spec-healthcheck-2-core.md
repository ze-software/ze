# Spec: healthcheck-2-core

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | spec-healthcheck-1-watchdog-med |
| Phase | - |
| Updated | 2026-04-03 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `plan/spec-healthcheck-0-umbrella.md` - umbrella design (FSM, state actions, data flow)
4. `.claude/patterns/plugin.md` - plugin structural template
5. `internal/component/bgp/plugins/watchdog/register.go` - registration pattern reference
6. `internal/component/bgp/plugins/watchdog/server.go` - watchdog command syntax
7. `pkg/plugin/sdk/sdk_engine.go` - DispatchCommand signature
8. `internal/component/bgp/route.go` - Route struct

## Task

Create the `bgp-healthcheck` plugin with core end-to-end functionality: plugin registration, YANG schema, config parsing, 8-state FSM, probe execution (shell command with timeout + process group kill), and watchdog command dispatch. This phase delivers a minimal but complete healthcheck: config defines a probe, probe runs a shell command, FSM tracks state, and watchdog announce/withdraw commands are dispatched. Only `withdraw-on-down true` mode (simpler path: announce on UP, withdraw on DOWN). MED mode, debounce, fast-interval, ip-setup, hooks, and CLI are deferred to later phases.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - component architecture
  -> Constraint: components register at startup via init(), communicate via bus/RPC
- [ ] `docs/architecture/api/process-protocol.md` - 5-stage plugin protocol
  -> Constraint: Stage 2 delivers config via OnConfigure. Dependencies field orders startup.
- [ ] `.claude/patterns/plugin.md` - plugin structural template
  -> Constraint: atomic logger, register.go with init(), schema/ subdir for YANG, RunXxxPlugin entry point
- [ ] `docs/architecture/api/update-syntax.md` - route command syntax
  -> Constraint: text commands use "update text ..." format. format.go is source of truth (doc is stale for `set` keyword).

### RFC Summaries (MUST for protocol work)
N/A -- healthcheck is operational tooling, not a BGP protocol feature.

**Key insights:**
- Plugin registers via init() -> registry.Register() with Name, Description, ConfigRoots, Dependencies, RunEngine
- ConfigRoots: ["bgp"] means OnConfigure receives the full BGP config tree
- Dependencies: ["bgp-watchdog"] ensures watchdog starts before healthcheck
- DispatchCommand(ctx, "watchdog announce <group> med <N>") dispatches to watchdog
- YANG schema goes in schema/ subdir with register.go + embed.go + .yang file
- After creating plugin, run `make generate` to update all.go

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/watchdog/register.go` - Registration pattern: Name, Description, ConfigRoots, RunEngine, InProcessDecoder, ConfigureEngineLogger, CLIHandler. No Dependencies field (watchdog has none).
  -> Constraint: healthcheck uses same pattern but adds Dependencies: ["bgp-watchdog"]
- [ ] `internal/component/bgp/plugins/watchdog/server.go` - handleCommand dispatches "watchdog announce" and "watchdog withdraw". After Phase 1, supports optional `med <N>`.
  -> Decision: healthcheck dispatches `watchdog announce <group> med <up-metric>` for UP, `watchdog withdraw <group>` for DOWN.
- [ ] `pkg/plugin/sdk/sdk_engine.go` - DispatchCommand(ctx, command) returns (status, data, error). Used for inter-plugin communication.
  -> Decision: healthcheck uses DispatchCommand for all watchdog interactions.
- [ ] `internal/component/bgp/plugins/role/schema/ze-role.yang` - YANG augment pattern: module with namespace, import ze-bgp-conf, augment /bgp:bgp paths.
  -> Constraint: healthcheck YANG augments /bgp:bgp with a healthcheck container.

**Behavior to preserve:**
- All existing BGP plugin behavior unchanged
- Watchdog command semantics unchanged (Phase 1 adds MED support, this phase uses it)

**Behavior to change:**
- New `bgp-healthcheck` plugin registered
- New `ze-healthcheck-conf.yang` YANG module
- New `bgp { healthcheck { probe <name> { ... } } }` config
- Probe goroutines run shell commands and dispatch watchdog commands

## Data Flow (MANDATORY)

### Entry Point
- Config: YANG `bgp { healthcheck { probe <name> { ... } } }` delivered via OnConfigure
- Timer: interval-based tick triggers probe execution
- Shell command: exit code determines success/failure

### Transformation Path
1. OnConfigure receives BGP config tree, extracts `healthcheck` subtree
2. Config parser builds ProbeConfig structs from YANG leaves
3. Probe manager starts goroutines per probe
4. Timer fires -> probe runs shell command via exec.CommandContext with timeout
5. Exit code -> FSM transition (INIT/RISING/UP/FALLING/DOWN/DISABLED/EXIT/END)
6. State change -> determine action: UP = `watchdog announce <group> med <up-metric>`, DOWN = `watchdog withdraw <group>`
7. DispatchCommand sends watchdog command

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config -> Healthcheck | OnConfigure callback with bgp.healthcheck tree | [ ] |
| Healthcheck -> Watchdog | DispatchCommand("watchdog announce/withdraw ...") | [ ] |

### Integration Points
- `registry.Register()` in init() - plugin discovered by engine
- `sdk.NewWithConn` - in-process plugin lifecycle
- `p.OnConfigure` - receives BGP config with healthcheck subtree
- `p.DispatchCommand` - sends watchdog commands
- `yang.RegisterModule` - schema registered for editor/validation
- `make generate` - updates all.go blank import

### Architectural Verification
- [ ] No bypassed layers -- healthcheck dispatches to watchdog via DispatchCommand
- [ ] No unintended coupling -- healthcheck only knows watchdog command syntax
- [ ] No duplicated functionality -- reuses watchdog for route management
- [ ] Zero-copy preserved where applicable -- N/A (config/command path)

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| YANG config `bgp { healthcheck { probe dns { command "true"; group hc-dns; withdraw-on-down true; } } }` | -> | config.go parses probe, FSM starts | `test/plugin/healthcheck-announce.ci` |
| Probe check success (rise met) | -> | FSM transitions to UP, dispatches `watchdog announce hc-dns med 100` | `test/plugin/healthcheck-announce.ci` |
| Probe check failure (fall met) | -> | FSM transitions to DOWN, dispatches `watchdog withdraw hc-dns` | `test/plugin/healthcheck-withdraw.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Plugin registered with Name="bgp-healthcheck" | `ze plugin bgp-healthcheck --features` exits 0, shows "yang" |
| AC-2 | YANG config with `bgp { healthcheck { probe dns { command "true"; group hc-dns } } }` | Config parses without error |
| AC-3 | Probe command exits 0, rise consecutive successes met (default rise=3) | FSM transitions to UP, `watchdog announce <group> med <up-metric>` dispatched |
| AC-4 | Probe command exits non-zero, fall consecutive failures met, `withdraw-on-down true` | FSM transitions to DOWN, `watchdog withdraw <group>` dispatched |
| AC-5 | Probe command times out (exceeds timeout seconds) | Treated as failure, process group killed (SIGKILL to -pid) |
| AC-6 | FSM in RISING, check fails | Counter resets, transitions to FALLING (fall > 1) or DOWN (fall <= 1) |
| AC-7 | FSM in FALLING, check succeeds | Counter resets, transitions to RISING (rise > 1) or UP (rise <= 1) |
| AC-8 | `interval 5` configured | Probe runs every 5 seconds in UP/DOWN/INIT states |
| AC-9 | `rise 1` configured | Single success transitions INIT -> UP (no intermediate RISING) |
| AC-10 | `fall 1` configured | Single failure transitions INIT -> DOWN (no intermediate FALLING) |
| AC-11 | Probe in UP, check fails | Transitions to FALLING (fall > 1) or DOWN (fall <= 1) |
| AC-12 | Probe in DOWN, check succeeds | Transitions to RISING (rise > 1) or UP (rise <= 1) |
| AC-13 | `command` leaf missing from config | YANG validation error (mandatory leaf) |
| AC-14 | `group` leaf missing from config | YANG validation error (mandatory leaf) |
| AC-15 | Plugin starts before watchdog (no dependency) | Does not happen -- Dependencies: ["bgp-watchdog"] enforces ordering |
| AC-16 | Probe stdout/stderr on failure | Combined output logged at warning level |
| AC-17 | Probe stdout/stderr on success | Logged at debug level |
| AC-18 | Graceful shutdown (context cancelled) | Probe goroutine exits, no commands dispatched after shutdown |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestFSMInitSuccess` | `fsm_test.go` | AC-3: INIT + success -> RISING/UP depending on rise |  |
| `TestFSMInitFailure` | `fsm_test.go` | AC-4: INIT + failure -> FALLING/DOWN depending on fall |  |
| `TestFSMRisingSuccess` | `fsm_test.go` | RISING + success -> UP when count >= rise |  |
| `TestFSMRisingFailure` | `fsm_test.go` | AC-6: RISING + failure -> FALLING/DOWN |  |
| `TestFSMFallingFailure` | `fsm_test.go` | FALLING + failure -> DOWN when count >= fall |  |
| `TestFSMFallingSuccess` | `fsm_test.go` | AC-7: FALLING + success -> RISING/UP |  |
| `TestFSMUpFailure` | `fsm_test.go` | AC-11: UP + failure -> FALLING/DOWN |  |
| `TestFSMDownSuccess` | `fsm_test.go` | AC-12: DOWN + success -> RISING/UP |  |
| `TestFSMShortcutRise1` | `fsm_test.go` | AC-9: rise=1 skips RISING entirely |  |
| `TestFSMShortcutFall1` | `fsm_test.go` | AC-10: fall=1 skips FALLING entirely |  |
| `TestProbeTimeout` | `probe_test.go` | AC-5: command timeout kills process group, returns failure |  |
| `TestProbeSuccess` | `probe_test.go` | Exit 0 = success |  |
| `TestProbeFailure` | `probe_test.go` | Exit non-zero = failure |  |
| `TestProbeOutputCapture` | `probe_test.go` | AC-16, AC-17: stdout/stderr captured |  |
| `TestConfigParseMandatory` | `config_test.go` | AC-13, AC-14: command and group required |  |
| `TestConfigParseDefaults` | `config_test.go` | Default values: interval=5, timeout=5, rise=3, fall=3, up-metric=100 |  |
| `TestAnnounceDispatch` | `announce_test.go` | AC-3: UP state dispatches watchdog announce med <up-metric> |  |
| `TestWithdrawDispatch` | `announce_test.go` | AC-4: DOWN state dispatches watchdog withdraw |  |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| interval | 0 - 86400 | 86400 | N/A (0 = single check) | 86401 |
| timeout | 1 - 3600 | 3600 | 0 | 3601 |
| rise | 1 - 1000 | 1000 | 0 | 1001 |
| fall | 1 - 1000 | 1000 | 0 | 1001 |
| up-metric | 0 - 4294967295 | 4294967295 | N/A (0 is valid) | 4294967296 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `healthcheck-announce` | `test/plugin/healthcheck-announce.ci` | Config with probe that runs `true`, withdraw-on-down true. After rise checks, watchdog announce is dispatched, peer receives UPDATE. |  |
| `healthcheck-withdraw` | `test/plugin/healthcheck-withdraw.ci` | Config with probe that runs `false`, withdraw-on-down true. After fall checks, watchdog withdraw is dispatched, peer receives WITHDRAW. |  |
| `healthcheck-basic-parse` | `test/parse/healthcheck-basic.ci` | YANG config with healthcheck probe parses successfully. |  |

### Future
- MED mode (withdraw-on-down false) -- Phase 3
- Debounce, fast-interval -- Phase 3
- IP management, hooks, CLI -- Phase 4
- External plugin mode -- Phase 5

## Files to Modify
- None -- this phase creates new files only. Watchdog changes are in Phase 1.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | Yes | `internal/component/bgp/plugins/healthcheck/schema/ze-healthcheck-conf.yang` |
| CLI commands/flags | No (Phase 4) | - |
| Editor autocomplete | Yes (YANG-driven, automatic) | - |
| Functional test for new RPC/API | Yes | `test/plugin/healthcheck-announce.ci`, `test/plugin/healthcheck-withdraw.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` -- add healthcheck section (basic) |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md` -- add `bgp { healthcheck {} }` syntax |
| 3 | CLI command added/changed? | No (Phase 4) | - |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` -- add bgp-healthcheck plugin |
| 6 | Has a user guide page? | Yes | `docs/guide/healthcheck.md` -- new page (basic, expanded in later phases) |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented? | No | - |
| 10 | Test infrastructure changed? | No | - |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` -- healthcheck feature |
| 12 | Internal architecture changed? | No | - |

## Files to Create
- `internal/component/bgp/plugins/healthcheck/register.go` - init() -> registry.Register()
- `internal/component/bgp/plugins/healthcheck/healthcheck.go` - Package doc, atomic logger, RunHealthcheckPlugin()
- `internal/component/bgp/plugins/healthcheck/config.go` - Parse YANG config tree into ProbeConfig structs
- `internal/component/bgp/plugins/healthcheck/fsm.go` - 8-state FSM with trigger() shortcut logic
- `internal/component/bgp/plugins/healthcheck/probe.go` - Shell command execution (process group, timeout)
- `internal/component/bgp/plugins/healthcheck/announce.go` - Watchdog command dispatch via DispatchCommand
- `internal/component/bgp/plugins/healthcheck/schema/register.go` - yang.RegisterModule
- `internal/component/bgp/plugins/healthcheck/schema/embed.go` - //go:embed ze-healthcheck-conf.yang
- `internal/component/bgp/plugins/healthcheck/schema/ze-healthcheck-conf.yang` - YANG schema
- `internal/component/bgp/plugins/healthcheck/fsm_test.go`
- `internal/component/bgp/plugins/healthcheck/probe_test.go`
- `internal/component/bgp/plugins/healthcheck/config_test.go`
- `internal/component/bgp/plugins/healthcheck/announce_test.go`
- `test/plugin/healthcheck-announce.ci`
- `test/plugin/healthcheck-withdraw.ci`
- `test/parse/healthcheck-basic.ci`

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan -- check what exists |
| 3. Implement (TDD) | Implementation phases below |
| 4. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 5. Critical review | Critical Review Checklist below |
| 6. Fix issues | Fix every issue from critical review |
| 7. Re-verify | Re-run stage 4 |
| 8. Repeat 5-7 | Max 2 review passes |
| 9. Deliverables review | Deliverables Checklist below |
| 10. Security review | Security Review Checklist below |
| 11. Re-verify | Re-run stage 4 |
| 12. Present summary | Executive Summary Report per `rules/planning.md` |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Plugin skeleton + YANG** -- Create register.go, healthcheck.go (logger, RunHealthcheckPlugin stub), schema/ (YANG with probe list, mandatory command/group leaves, interval/timeout/rise/fall/up-metric/withdraw-on-down leaves with defaults). Run `make generate`.
   - Tests: `TestConfigParseMandatory`, `TestConfigParseDefaults`, parse .ci
   - Files: register.go, healthcheck.go, schema/*, config.go, config_test.go
   - Verify: plugin registers, YANG validates, config parses

2. **Phase: FSM** -- Implement 8-state FSM with trigger() shortcut. States: INIT, RISING, UP, FALLING, DOWN, DISABLED, EXIT, END. Only INIT/RISING/UP/FALLING/DOWN transitions in this phase (DISABLED/EXIT/END wired in later phases).
   - Tests: all FSM unit tests
   - Files: fsm.go, fsm_test.go
   - Verify: all FSM transition tests pass

3. **Phase: Probe execution** -- Shell command via exec.CommandContext with timeout. Process group isolation (Setpgid). SIGKILL on timeout. Exit code -> success/failure. Output capture.
   - Tests: `TestProbeTimeout`, `TestProbeSuccess`, `TestProbeFailure`, `TestProbeOutputCapture`
   - Files: probe.go, probe_test.go
   - Verify: probe tests pass

4. **Phase: Announce/withdraw dispatch** -- DispatchCommand for watchdog announce/withdraw. UP dispatches `watchdog announce <group> med <up-metric>`. DOWN dispatches `watchdog withdraw <group>`.
   - Tests: `TestAnnounceDispatch`, `TestWithdrawDispatch`
   - Files: announce.go, announce_test.go
   - Verify: dispatch tests pass

5. **Phase: Wire it together** -- RunHealthcheckPlugin: OnConfigure parses config, starts probe goroutines. Each goroutine: timer -> probe -> FSM -> dispatch. Graceful shutdown via context cancellation.
   - Tests: functional tests (healthcheck-announce.ci, healthcheck-withdraw.ci)
   - Files: healthcheck.go
   - Verify: functional tests pass

6. **Full verification** -- `make ze-verify`
7. **Complete spec** -- Fill audit tables, write learned summary.

### Critical Review Checklist (/implement stage 5)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1 through AC-18 has implementation with file:line |
| Correctness | FSM transition table matches spec exactly. trigger() shortcut handles rise<=1 and fall<=1. |
| Naming | Plugin name "bgp-healthcheck". YANG module "ze-healthcheck-conf". Package "healthcheck". |
| Data flow | Config -> ProbeConfig -> goroutine -> probe -> FSM -> DispatchCommand -> watchdog |
| Wiring | Can a user reach healthcheck through config? Functional test proves it. |
| Process isolation | Probe uses Setpgid + SIGKILL(-pid) on timeout. No zombie processes. |
| Rule: plugin pattern | Follows .claude/patterns/plugin.md: atomic logger, register.go, schema/ subdir |

### Deliverables Checklist (/implement stage 9)
| Deliverable | Verification method |
|-------------|---------------------|
| Plugin registers | `ze plugin bgp-healthcheck --features` exits 0 |
| YANG schema loads | `ze schema list` includes ze-healthcheck-conf |
| Config parses | `test/parse/healthcheck-basic.ci` passes |
| FSM transitions correct | All fsm_test.go tests pass |
| Probe timeout works | `TestProbeTimeout` passes |
| Announce dispatched on UP | `test/plugin/healthcheck-announce.ci` passes |
| Withdraw dispatched on DOWN | `test/plugin/healthcheck-withdraw.ci` passes |
| all.go updated | `grep healthcheck internal/component/plugin/all/all.go` |
| No import of watchdog package | `grep -r 'watchdog' internal/component/bgp/plugins/healthcheck/` shows only command strings |

### Security Review Checklist (/implement stage 10)
| Check | What to look for |
|-------|-----------------|
| Shell injection via command | Config value passed to `/bin/sh -c <command>`. Admin-controlled (config file). Same threat model as ExaBGP. Process group isolation + timeout. |
| Process group kill | Verify SIGKILL sent to -pid (negative = process group), not just pid |
| Zombie processes | Verify cmd.Wait() called after kill to reap child |
| Resource exhaustion (goroutines) | One goroutine per probe. Bounded by config. Context cancellation stops goroutines. |
| Timeout bypass | Verify exec.CommandContext uses context.WithTimeout, not manual timer |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior |
| Lint failure | Fix inline |
| Functional test fails | Check AC; if AC wrong -> DESIGN; if AC correct -> IMPLEMENT |
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

## Implementation Summary

### What Was Implemented
- [List actual changes made]

### Bugs Found/Fixed
- [Any bugs discovered -- add test for each]

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
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-healthcheck-2-core.md`
- [ ] Summary included in commit
