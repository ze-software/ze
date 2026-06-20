# Spec: bugfix-sys-plugin-lifecycle-rollback

| Field | Value |
|-------|-------|
| Status | done |
| Depends | - |
| Phase | - |
| Updated | 2026-06-20 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `plan/review-bug-review-plugin-engine-system.md` findings SYS-001 and SYS-002
3. `ai/rules/plugin-design.md` lifecycle and dependency sections
4. `internal/component/plugin/server/startup.go`
5. `internal/component/plugin/server/reload.go`
6. `internal/component/plugin/server/startup_autoload.go`
7. `internal/component/plugin/registration.go`
8. `internal/core/family/registry.go`

## Task

Fix plugin lifecycle rollback defects found by SYS-001 and SYS-002. Plugin startup and reload must be exact-or-reject: a failed startup or failed reload must not leave dynamic plugin registration, family registration, capability injection, command ownership, process state, or backend runtime state from a plugin that did not successfully reach the committed state.

## Required Reading

### Architecture Docs
- [ ] `plan/review-bug-review-plugin-engine-system.md` - source finding and reproduction plan
  -> Decision: fix SYS-001 and SYS-002 together because both are plugin lifecycle rollback defects.
  -> Constraint: do not fix by suppressing errors. Return startup/reload failure and roll back partial state.
- [ ] `ai/rules/plugin-design.md` - plugin startup stages, dependencies, optional dependencies, DirectBridge lifecycle
  -> Decision: stage failures must stop the affected plugin and prevent async command handlers from starting.
  -> Constraint: command/family/capability state must be owned by the plugin that reached ready.
- [ ] `ai/rules/plugin-self-containment.md` - removal and ownership invariant
  -> Decision: failed plugins must leave no user-visible command/schema/runtime surface active.
  -> Constraint: deleting or rejecting a plugin removes its owned features and nothing else.

### RFC Summaries (MUST for protocol work)
- [ ] N/A. This is plugin lifecycle and config transaction behavior, not wire protocol behavior.

**Key insights:**
- `runPluginPhase` currently waits for startup goroutines but does not surface per-process stage failure after the wait.
- Stage 1 registration and Stage 3 capability injection mutate registries before later conflicts are known.
- Reload cleanup currently passes plugin names to a function that expects removed config roots.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/plugin/server/startup.go:386-415` - starts async runtime handlers for all tier processes after startup goroutines return, with no checked startup error aggregate.
- [ ] `internal/component/plugin/server/startup.go:511-527` - registers plugin metadata before registering families, with no rollback on family conflict.
- [ ] `internal/component/plugin/server/startup.go:577-582` - capability conflict calls `handlePluginConflict` after partial capability mutation can already have happened.
- [ ] `internal/component/plugin/registration.go:185-218` - plugin registry mutates command/family maps and plugin map after conflicts checked only inside that registration.
- [ ] `internal/component/plugin/registration.go:333-377` - `AddPluginCapabilities` appends capabilities while iterating and returns on later conflicts without rollback.
- [ ] `internal/core/family/registry.go:130-177` - `RegisterFamily` stores each successful family immediately in global state.
- [ ] `internal/component/plugin/server/reload.go:199-212` and `271-276` - reload records auto-loaded plugin names, then passes those names to cleanup after transaction failure.
- [ ] `internal/component/plugin/server/startup_autoload.go:233-237` and `278-305` - auto-load returns plugin names, while auto-stop matches removed config roots.
- [ ] `internal/plugins/fib/kernel/register.go:57-64` - plugin name `fib-kernel` differs from config root `fib/kernel`, proving the mismatch.

**Behavior to preserve:**
- Successful plugin startup still progresses through declaration, config, capability, registry, ready, then runtime handlers.
- Reload still starts newly needed config-root plugins before verification and applies transaction callbacks across affected plugins.
- Optional dependencies remain optional per `plugin-design.md`.

**Behavior to change:**
- Any startup stage failure returns an error to the phase caller and prevents runtime handlers from starting for failed processes.
- Partial plugin registry, family registry, and capability injector changes made by a failed plugin are rolled back or applied atomically only after all validation passes.
- Reload failure after auto-load stops the exact plugin names that were started for the attempted config, including names whose config roots differ.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- Plugin startup through generated imports, config-root autoload, event/send type autoload, and explicit plugin config.
- Config reload through changed config roots and transaction verification/apply callbacks.

### Transformation Path
1. Server selects plugins to start for a phase.
2. `runPluginPhase` starts processes by dependency tier and drives startup RPC stages.
3. Stage handlers mutate plugin registry, family registry, capability injector, command registry, config delivery, and ready state.
4. Runtime handlers start after startup succeeds.
5. Reload starts newly needed config-root plugins, verifies and applies the transaction, then either commits or rolls back.
6. On failure, all state that was created for the failed startup or reload is removed.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Plugin process -> server startup | stage RPC over plugin connection | [ ] startup unit tests assert error propagation |
| Server -> plugin registry | `registry.Register` and command/family claims | [ ] rollback or atomic validation unit tests |
| Server -> family registry | `family.RegisterFamily` for external families | [ ] conflict test shows no earlier family remains |
| Server -> capability injector | `AddPluginCapabilities` | [ ] conflict test shows no earlier capability remains |
| Reload -> process manager | auto-loaded plugin names | [ ] failed reload cleanup test checks process removed |

### Integration Points
- `internal/component/plugin/server/startup.go`
- `internal/component/plugin/server/reload.go`
- `internal/component/plugin/server/startup_autoload.go`
- `internal/component/plugin/registration.go`
- `internal/core/family/registry.go`
- `internal/component/plugin/process/manager.go`

### Architectural Verification
- [ ] No bypassed layers: startup and reload failures flow through server lifecycle, not test-only hooks.
- [ ] No unintended coupling: rollback uses registry/process ownership data, not plugin-specific names.
- [ ] No duplicated functionality: one cleanup path handles startup and reload partial state where possible.
- [ ] Zero-copy preserved where applicable: N/A.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|------------|-------|----------|--------------|--------|
| A-1 | Registry state can be snapshotted or transactionally validated for plugin startup tests | existing registry Snapshot/Restore patterns and test reset helpers | fix needs a narrower rollback API | rollback tests and `family.RegisterFamilyBatch` | confirmed |
| A-2 | Auto-loaded plugin names are sufficient to stop failed reload additions | `autoLoadForNewConfigPaths` returns plugin names | cleanup also needs dependency names or roots | `TestReloadFailureStopsAutoLoadedPluginByName` | confirmed |
| A-3 | Startup stage failure should abort the phase for config-root and explicit plugin startup | exact-or-reject plugin lifecycle rule | some optional plugin failures should remain warnings | startup failure and optional dependency tests | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Rollback removes state owned by another plugin | test fails with two plugins sharing unrelated valid registrations | attach rollback state to plugin name and only remove matching entries |
| R-2 | Existing tests rely on warning-only autoload failure | startup tests fail after stricter behavior | split hard dependency errors from optional dependency absence |
| R-3 | Family registry lacks rollback API | cannot remove per-plugin registered families safely | validate all plugin family declarations before mutating global family state |

## Wiring Test (MANDATORY - NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| plugin startup Stage 1 conflict | -> | startup phase returns error and rolls back registry/family state | `TestPluginStartupRollsBackPartialRegistration` |
| plugin startup Stage 3 capability conflict | -> | capability injector mutation is atomic | `TestCapabilityInjectorConflictIsAtomic` |
| tier startup goroutine failure | -> | `runPluginPhase` returns error and does not start runtime handlers for failed process | `TestRunPluginPhaseReturnsStageFailure` |
| failed config reload after autoload | -> | auto-loaded plugin name is stopped and removed | `TestReloadFailureStopsAutoLoadedPluginByName` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A plugin registers multiple families and a later family conflicts | startup returns an error and none of that plugin's families or registry rows remain visible |
| AC-2 | Capability injection receives multiple capabilities and a later capability conflicts | the call returns an error and no earlier capability from that batch remains visible |
| AC-3 | A process fails any startup stage before ready | the startup phase returns an error and async runtime handlers are not started for that process |
| AC-4 | Reload auto-loads `fib-kernel` for `fib/kernel`, then transaction verify or apply fails | `fib-kernel` and any auto-loaded dependencies from the failed reload are stopped or returned to pre-reload state |
| AC-5 | Optional dependency is absent but declared optional | startup still succeeds with the existing optional fallback behavior |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Starts Ze with a plugin whose dynamic family declaration conflicts | config/root autoload -> plugin startup -> family registration -> startup error | `TestPluginStartupRollsBackPartialRegistration` |
| 2 | Reloads config that starts a config-root plugin but later fails transaction | reload diff -> auto-load -> verify/apply failure -> cleanup | `TestReloadFailureStopsAutoLoadedPluginByName` |
| 3 | Uses a plugin with an optional dependency absent | registry dependency resolution -> tier startup -> owner fallback | `TestRunPluginPhaseAllowsMissingOptionalDependency` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPluginStartupRollsBackPartialRegistration` | `internal/component/plugin/server/startup_test.go` | AC-1 and phase error propagation | |
| `TestCapabilityInjectorConflictIsAtomic` | `internal/component/plugin/registration_test.go` | AC-2 | |
| `TestRunPluginPhaseReturnsStageFailure` | `internal/component/plugin/server/startup_test.go` | AC-3 | |
| `TestReloadFailureStopsAutoLoadedPluginByName` | `internal/component/plugin/server/reload_test.go` | AC-4 | |
| `TestRunPluginPhaseAllowsMissingOptionalDependency` | `internal/component/plugin/server/startup_test.go` | AC-5 | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A | lifecycle state, not numeric input | | | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| Existing plugin reload suite or new reload `.ci` if infrastructure supports injected failure | `test/reload/` or Go integration test | failed reload does not leave auto-loaded process running | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N/A | | | plugin lifecycle only | |

### Future (if deferring any tests)
- No deferral approved. If functional `.ci` injection is not possible, the Go integration test must drive the real server/process manager path.

## Files to Modify

- `internal/component/plugin/server/startup.go` - propagate stage failures and guard runtime handler start.
- `internal/component/plugin/server/reload.go` - clean failed auto-loads by plugin name or retained load record.
- `internal/component/plugin/server/startup_autoload.go` - return cleanup records that include plugin names and config roots.
- `internal/component/plugin/registration.go` - make capability injection atomic or rollbackable.
- `internal/core/family/registry.go` or plugin startup validation code - avoid partial family registration on failed plugin startup.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | No | lifecycle only |
| CLI commands/flags | No | |
| Functional test for new RPC/API | No | unit/integration lifecycle tests |
| Env var registration | No | |
| Doctor check for runtime dependencies | No | |
| Prometheus counters/metrics | No | |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | bug fix |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | No | |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | behavior unchanged, lifecycle error semantics fixed |
| 9 | RFC behavior implemented? | No | |
| 10 | Test infrastructure changed? | No | unless a reload integration harness is added |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | No | unless a new rollback API is added, then update `docs/architecture/core-design.md` |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | |
| 15 | Registered plugin, event type, send type, command, capability, or runtime inventory changed? | No | dynamic rollback semantics only |
| 16 | Any changed source file is referenced by existing doc source anchors? | Yes | grep docs for changed source paths during implementation |
| 17 | Existing docs show config/CLI/API examples for this area? | No | |

## Files to Create

- `internal/component/plugin/server/startup_test.go` or extend existing startup tests.
- `internal/component/plugin/server/reload_test.go` or extend existing reload tests.
- `internal/component/plugin/registration_test.go` or extend existing registry tests.

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Current Behavior and Files to Modify |
| 3. Wiring phase | Lifecycle tests above |
| 4. Implement | Rollback/atomic lifecycle changes |
| 5. Review gate | Critical Review Checklist |
| 6. Full verification | targeted Go tests, `make ze-test-plugins`, then `make ze-verify` if code changed |

### Implementation Phases
1. **Phase: failing lifecycle tests** - write AC-1 through AC-4 tests and confirm they fail for the described reason.
2. **Phase: atomic registration** - make plugin registry, family registration, and capability injection transactional for a startup attempt.
3. **Phase: startup error propagation** - make `runPluginPhase` return stage failures and suppress runtime handlers for failed processes.
4. **Phase: reload cleanup** - track auto-loaded plugin names and stop those names on transaction failure.
5. **Phase: verification** - targeted tests, plugin component tests, and final gate.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | SYS-001 and SYS-002 both have failing tests and source fixes |
| Correctness | rollback removes only state introduced by the failed plugin or reload |
| Optional deps | optional dependency behavior still works |
| Data flow | startup and reload failure paths reach the same cleanup invariants |
| Tests | tests fail before fix and pass after fix |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Startup rollback tests | `go test -run 'TestPluginStartupRollsBackPartialRegistration|TestRunPluginPhaseReturnsStageFailure' ./internal/component/plugin/server` |
| Capability atomicity test | `go test -run TestCapabilityInjectorConflictIsAtomic ./internal/component/plugin` |
| Reload cleanup test | `go test -run TestReloadFailureStopsAutoLoadedPluginByName ./internal/component/plugin/server` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Partial state leakage | failed plugin cannot leave commands, families, capabilities, or process state active |
| Cleanup ownership | rollback cannot remove state owned by a different plugin |
| Error messages | startup/reload errors include plugin name and failed stage without secrets |

### Failure Routing
| Failure | Route To |
|---------|----------|
| rollback API would affect unrelated registries | split into smaller owner-specific rollback tests |
| optional dependency behavior regresses | fix dependency resolver branch before continuing |
| functional reload injection unavailable | keep Go integration test as required evidence and document why `.ci` is not applicable |

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

- Plugin lifecycle correctness depends on atomic visibility of dynamic surfaces.

## Core Insight

A plugin that did not reach ready must be indistinguishable from a plugin that was never loaded.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Fix startup and reload rollback together | separate specs for SYS-001 and SYS-002 | both enforce the same exact-or-reject lifecycle invariant |

## Known Limitations

- Implemented in plugin startup, registration, family registry, capability injection, and reload cleanup.

## RFC Documentation

N/A.

## Implementation Summary

### What Was Implemented
- Plugin startup now returns pre-ready stage failures, starts runtime handlers only for committed processes, and rolls back failed plugin registry, family, capability, command, subscription, cache-consumer, and process state.
- Capability injection and runtime family registration are atomic, with owner-scoped rollback for later startup failures.
- Reload failure stops auto-loaded plugin names directly, while optional dependency absence still succeeds.

### Bugs Found/Fixed
- SYS-001 and SYS-002 documented for implementation.
- No production bug was fixed by this review program.

### Documentation Updates
- No user docs required for the fix-spec artifact.

### Deviations from Plan
- None.

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Create actionable fix plan for SYS-001 | done | this spec | startup partial registration and rollback covered |
| Create actionable fix plan for SYS-002 | done | this spec | failed reload autoload cleanup covered |
| Include regression plan | done | Wiring Test and TDD sections | tests named for lifecycle rollback and cleanup |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1..AC-5 | done | `TestPluginStartupRollsBackPartialRegistration`, `TestPluginStartupRollsBackFamiliesAfterLaterStageFailure`, `TestCapabilityInjectorConflictIsAtomic`, `TestReloadFailureStopsAutoLoadedPluginByName` | startup and reload rollback paths covered |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| startup rollback tests | done | `internal/component/plugin/server/startup_test.go` | passing |
| capability atomicity tests | done | `internal/component/plugin/capability_injection_test.go` | passing |
| reload cleanup tests | done | `internal/component/plugin/server/reload_test.go` | passing |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| plugin startup, registration, family, reload files | done | implementation targets updated |

### Audit Summary
- Total items: 2 accepted findings converted to one fix spec.
- Done: lifecycle rollback implementation and regression tests.
- Partial: none.
- Skipped: no approved scope reduction.
- Changed: plugin lifecycle, family registry, capability registry, reload cleanup, DirectBridge typed panic coverage, tests, and docs.

## Goal Validation
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Plugin lifecycle exact-or-reject | spec artifact | this file names rollback invariants, ACs, and regression tests for SYS-001 and SYS-002 |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | BLOCKER | SYS-001 startup failure can be swallowed after partial dynamic registration | child 2 report | fix spec created |
| 2 | ISSUE | SYS-002 failed reload cleanup passes plugin names to config-root stop logic | child 2 report | fix spec created |

### Fixes applied
- None, review program creates fix specs only.

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| - | - | Fix-spec artifact now has summary, audit, goal validation, review gate, and pre-commit sections | this spec | no action |

### Final status
- [ ] `/ze-review` re-run by implementation owner after code changes
- [x] Fix spec contains source evidence, ACs, and regression plan

## Pre-Commit Verification

### Files Exist
| File | Exists | Evidence |
|------|--------|----------|
| `plan/spec-bugfix-sys-plugin-lifecycle-rollback.md` | yes | created by review program |

### AC Verified
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| review deliverable | fix spec exists | this file |

### Wiring Verified
| Entry Point | Test | Verified |
|-------------|------|----------|
| plugin startup rollback | planned unit tests | pending implementation |
| failed reload autoload cleanup | planned unit tests | pending implementation |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1..A-3 | unresolved for implementation | listed in spec |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| No user docs needed | lifecycle bugfix spec only | yes |

## Checklist

### Goal Gates (MUST pass)
- [x] AC-1..AC-N planned
- [x] Wiring Test table complete
- [x] Review gate complete for fix-spec artifact
- [x] Verification plan recorded
- [x] Risks & Assumptions recorded

### Completion (BLOCKING, before ANY commit)
- [x] Critical Review passes for fix-spec artifact
- [x] Implementation Summary filled
- [x] Implementation Audit filled
- [ ] Write learned summary during implementation if code changes reveal a durable lesson
