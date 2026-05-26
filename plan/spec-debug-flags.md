# Spec: debug-flags

| Field | Value |
|-------|-------|
| Status | complete |
| Depends | - |
| Phase | 9/9 |
| Updated | 2026-05-26 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `pkg/zefs/keys.go` - existing key registrations
4. `pkg/zefs/registry.go` - KeyEntry pattern
5. `internal/core/slogutil/slogutil.go` - logging infrastructure
6. `internal/component/cmd/log/log.go` - existing log RPC commands

## Task

Runtime debug flags stored in zefs, toggled via CLI. Inspired by VyOS filesystem debug flags
(`touch /tmp/vyos.ifconfig.debug`) but using zefs keys instead of files.

Three-tier resolution: **global override > per-subsystem key > per-subsystem default**.

- Per-subsystem default: off (hardcoded)
- Per-subsystem zefs key: `state/debug/{subsystem}` with value `on`/`off`
- Global zefs key: `state/debug/all` with value `on`/`off`; when `on`, all subsystems
  are forced to debug level regardless of their individual keys

CLI commands (action before identifier per `ai/rules/cli-grammar.md`):
- `debug enable <subsystem>` -- set per-subsystem debug on
- `debug disable <subsystem>` -- set per-subsystem debug off
- `debug enable all` -- set global override on
- `debug disable all` -- set global override off
- `debug show` -- list all subsystems with columns: name, effective-state (on/off), source (global/explicit/default)

The debug flag writes to zefs (persists across restarts) AND applies the level change
to the running daemon immediately via `slogutil.SetLevel()`.

At daemon startup, read debug keys from zefs and apply before first log message.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/zefs-format.md` - zefs key/value format and naming conventions
  -> Constraint: keys use `meta/` for metadata, `file/` for config files, `state/` for runtime state
- [ ] `docs/architecture/config/environment.md` - logging env var design
  -> Constraint: env vars take priority over config file; debug flags should sit alongside, not replace
- [ ] `ai/rules/cli-grammar.md` - action before identifier
  -> Constraint: `debug enable bgp` not `debug bgp enable`
- [ ] `ai/rules/registration.md` - registration pattern
  -> Constraint: use init() + MustRegister/RegisterLocal for CLI commands and zefs keys
- [ ] `ai/rules/derive-not-hardcode.md` - subsystem list from registry
  -> Constraint: `debug show` must derive subsystem list from slogutil.Subsystems(), not hardcode

**Key insights:**
- zefs keys are hierarchical with `/` separators, registered via `MustRegister`
- slogutil already has `SetLevel()` (5 callers across 3 files), `ListLevels()`, `Subsystems()` for runtime level control
- existing `log set` RPC (internal/component/cmd/log/log.go:115) provides runtime level control but is ephemeral
- debug flags add persistence layer (zefs) on top of existing level control

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `pkg/zefs/keys.go` - 31 registered keys under `meta/` and `file/` namespaces; no `state/` namespace yet
- [ ] `pkg/zefs/registry.go` - KeyEntry with Pattern, Description, Private; MustRegister, Key(params...), Prefix()
- [ ] `internal/core/slogutil/slogutil.go` - Logger() creates per-subsystem loggers; SetLevel() changes level at runtime via levelRegistry (*slog.LevelVar); ListLevels() returns current state; Subsystems() returns all registered names with descriptions
- [ ] `internal/component/cmd/log/log.go` - RPC handlers: `ze-bgp:log-levels`, `ze-bgp:log-set`, `ze-bgp:log-recent`; registered via pluginserver.RegisterRPCs in init()
- [ ] `cmd/ze/data/register.go` - `ze data` root command (offline, SectionConfiguration); `show data ls/cat/registered` subcommands

**Behavior to preserve:**
- Existing env var hierarchy (`ze.log.*`) continues to set initial levels
- `log set` RPC continues to work for ephemeral runtime level changes
- `log levels` RPC continues to show current levels
- Config file log settings continue to work via ApplyLogConfig()

**Behavior to change:**
- Add `state/debug/` namespace in zefs for persistent debug flags
- Add `debug` CLI command namespace for toggling
- At daemon startup, apply debug flags from zefs after env var / config initialization

## Data Flow (MANDATORY)

### Entry Point
- CLI: `ze debug enable bgp` (offline command, writes to zefs directly)
- CLI session: `debug enable bgp` (online command via RPC, daemon writes to zefs + applies to running loggers)
- Startup: daemon reads `state/debug/*` from zefs and applies before loggers are used

### Transformation Path
1. CLI parses `debug enable <subsystem>` or `debug enable all`
2. Validate subsystem name against `slogutil.Subsystems()` (or literal "all")
3. Write `on`/`off` to `state/debug/<subsystem>` or `state/debug/all` in zefs
4. Apply to running daemon: resolve effective state per subsystem, call `slogutil.SetLevel()`
5. `debug show` reads all `state/debug/*` keys + `slogutil.Subsystems()`, computes effective state

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| CLI -> zefs | Direct WriteFile via resolved storage (offline) | [ ] |
| CLI -> daemon | RPC for online mode (like log set pattern) | [ ] |
| zefs -> slogutil | Read keys, call SetLevel() at startup | [ ] |

### Integration Points
- `slogutil.SetLevel(subsystem, level)` - applies level change to running logger (slogutil.go:494)
- `slogutil.Subsystems()` - discovers all registered subsystem names (slogutil.go:107)
- `slogutil.ListLevels()` - shows current effective levels (slogutil.go:475)
- `zefs.BlobStore.WriteFile()` / `ReadFile()` - persists debug state (store.go:171, 94)
- `cmdregistry.RegisterRoot()` / `MustRegisterLocal()` - CLI registration

### Architectural Verification
- [ ] No bypassed layers (uses existing slogutil SetLevel, not direct LevelVar mutation)
- [ ] No unintended coupling (debug package only imports slogutil + zefs, not specific components)
- [ ] No duplicated functionality (extends existing log level control with persistence, doesn't recreate)
- [ ] Zero-copy preserved where applicable (debug values are tiny strings, not performance-critical)

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `ze debug enable bgp` | -> | debug.Run() writes zefs key + applies level | `TestDebugEnableSubsystem` |
| `ze debug show` | -> | debug.Run() reads zefs + slogutil state | `TestDebugShow` |
| daemon startup | -> | ApplyDebugFlags() reads zefs, calls SetLevel | `TestApplyDebugFlagsFromZefs` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `ze debug enable bgp` | Writes `state/debug/bgp` = `on` to zefs; bgp.* subsystems set to debug level |
| AC-2 | `ze debug disable bgp` | Writes `state/debug/bgp` = `off` to zefs; bgp.* subsystems restored to configured level |
| AC-3 | `ze debug enable all` | Writes `state/debug/all` = `on` to zefs; ALL subsystems set to debug level |
| AC-4 | `ze debug disable all` | Writes `state/debug/all` = `off`; each subsystem falls back to its per-subsystem key or default |
| AC-5 | `ze debug show` | Lists all subsystems with columns: name, effective-state (on/off), source (global/explicit/default) |
| AC-6 | Per-subsystem key `on` + global `off` | Only that subsystem at debug; global does not override |
| AC-7 | Per-subsystem key `off` + global `on` | Subsystem at debug (global overrides) |
| AC-8 | Daemon restart with debug keys in zefs | Debug levels restored from zefs at startup |
| AC-9 | Invalid subsystem name | Error message listing valid subsystems |
| AC-10 | `debug enable bgp` in CLI session (online) | Same as AC-1 but via RPC to running daemon |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestResolveDebugState` | `internal/core/slogutil/debug_test.go` | Three-tier resolution: global > explicit > default | |
| `TestResolveDebugStateHierarchical` | `internal/core/slogutil/debug_test.go` | `debug enable bgp` covers bgp.reactor, bgp.fsm etc. | |
| `TestDebugEnableSubsystem` | `cmd/ze/debug/debug_test.go` | Writes correct zefs key, calls SetLevel | |
| `TestDebugDisableSubsystem` | `cmd/ze/debug/debug_test.go` | Writes off, restores previous level via getLogEnv | |
| `TestDebugEnableAll` | `cmd/ze/debug/debug_test.go` | Global override sets all to debug | |
| `TestDebugDisableAll` | `cmd/ze/debug/debug_test.go` | Fallback to per-subsystem keys | |
| `TestDebugShow` | `cmd/ze/debug/debug_test.go` | Output includes name, state, source columns | |
| `TestDebugInvalidSubsystem` | `cmd/ze/debug/debug_test.go` | Error with valid subsystem list | |
| `TestApplyDebugFlagsFromZefs` | `internal/core/slogutil/debug_test.go` | Startup reads zefs keys, applies levels | |
| `TestZefsKeyRegistration` | `cmd/ze/debug/debug_test.go` | Keys registered in zefs registry | |

### Boundary Tests (MANDATORY for numeric inputs)
N/A - no numeric inputs (on/off boolean flags only)

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `debug-enable-show` | `test/ui/debug-enable-show.ci` | Enable bgp debug, show reflects state, disable, show reflects off | |
| `debug-global-override` | `test/ui/debug-global-override.ci` | Enable all, show all on, disable all, show all off | |

### Interop Tests
N/A - not a protocol feature.

## Files to Modify
- `pkg/zefs/keys.go` - add `state/debug/all` and `state/debug/{subsystem}` key registrations
- `internal/core/slogutil/slogutil.go` - export `getLogEnv()` as `GetLogEnv()` for restore-on-disable
- `cmd/ze/main.go` - call `slogutil.ApplyDebugFlags()` after ApplyLogConfig

## Files to Create
- `cmd/ze/debug/register.go` - CLI command registration (init + RegisterRoot + RegisterLocal)
- `cmd/ze/debug/debug.go` - Run() handler: enable/disable/show logic
- `cmd/ze/debug/debug_test.go` - unit tests
- `internal/core/slogutil/debug.go` - ApplyDebugFlags(), ResolveDebugState(), ReadDebugKeys()
- `internal/core/slogutil/debug_test.go` - unit tests for resolution logic
- `test/ui/debug-enable-show.ci` - functional test
- `test/ui/debug-global-override.ci` - functional test

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | [x] | RPC registration for online `debug enable/disable/show` |
| CLI commands/flags | [x] | `cmd/ze/debug/register.go` |
| CLI grammar (action before identifier) | [x] | `debug enable <subsystem>`, `debug disable <subsystem>`, `debug show` |
| Editor autocomplete | [ ] | N/A |
| Functional test for new RPC/API | [x] | `test/ui/debug-enable-show.ci` |
| Doctor check for runtime dependencies | [ ] | No new runtime dependencies |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [x] | `docs/features.md` |
| 2 | Config syntax changed? | [ ] | N/A |
| 3 | CLI command added/changed? | [x] | `docs/guide/command-reference.md` |
| 4 | API/RPC added/changed? | [x] | `docs/architecture/api/commands.md` |
| 5 | Plugin added/changed? | [ ] | N/A |
| 6 | Has a user guide page? | [x] | `docs/guide/debugging.md` (new or add section) |
| 7 | Wire format changed? | [ ] | N/A |
| 8 | Plugin SDK/protocol changed? | [ ] | N/A |
| 9 | RFC behavior implemented? | [ ] | N/A |
| 10 | Test infrastructure changed? | [ ] | N/A |
| 11 | Affects daemon comparison? | [ ] | N/A |
| 12 | Internal architecture changed? | [ ] | N/A |

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
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

1. **Phase: Wiring (MANDATORY FIRST)** -- register zefs keys, CLI commands, stub handlers
   - Tests: `TestZefsKeyRegistration`, `TestDebugEnableSubsystem` (fails: stub)
   - Files: `pkg/zefs/keys.go`, `cmd/ze/debug/register.go`, `cmd/ze/debug/debug.go`
   - Verify: `ze debug show` runs (returns empty/stub); wiring test fails because logic is stub

2. **Phase: Resolution logic** -- three-tier debug state resolution
   - Tests: `TestResolveDebugState`, `TestResolveDebugStateHierarchical`
   - Files: `internal/core/slogutil/debug.go`, `internal/core/slogutil/debug_test.go`
   - Verify: resolution returns correct state for all combinations of global/explicit/default

3. **Phase: Enable/disable** -- write zefs keys + apply to running loggers
   - Tests: `TestDebugEnableSubsystem`, `TestDebugDisableSubsystem`, `TestDebugEnableAll`, `TestDebugDisableAll`
   - Files: `cmd/ze/debug/debug.go`, `cmd/ze/debug/debug_test.go`
   - Verify: zefs keys written, slogutil levels changed

4. **Phase: Show** -- read state and format output
   - Tests: `TestDebugShow`
   - Files: `cmd/ze/debug/debug.go`
   - Verify: output shows name, effective state, source for all subsystems

5. **Phase: Startup apply** -- read zefs at daemon start
   - Tests: `TestApplyDebugFlagsFromZefs`
   - Files: `internal/core/slogutil/debug.go`, `cmd/ze/main.go`
   - Verify: debug levels restored from zefs keys at startup

6. **Phase: Online RPC** -- `debug enable/disable/show` via CLI session
   - Tests: functional test for online mode
   - Files: RPC handler registration following internal/component/cmd/log/log.go pattern
   - Verify: CLI session `debug enable bgp` works

7. **Functional tests** -- create .ci tests for end-user scenarios
8. **Full verification** -- `make ze-verify`
9. **Complete spec** -- audit, learned summary, delete spec

### Critical Review Checklist (/implement stage 6)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1 through AC-10 has implementation with file:line |
| Correctness | Resolution order: global on overrides all; per-subsystem overrides default |
| Naming | zefs keys use `state/debug/` prefix; CLI grammar: action before identifier |
| Data flow | CLI -> zefs write -> slogutil.SetLevel(); startup: zefs read -> SetLevel() |
| CLI grammar | `debug enable <subsystem>`, not `debug <subsystem> enable` |
| Subsystem discovery | `debug show` and validation use slogutil.Subsystems(), not hardcoded list |
| Persistence | Keys survive daemon restart (written to zefs blob store) |
| Hierarchical matching | `debug enable bgp` covers all bgp.* children |

### Deliverables Checklist (/implement stage 10)

| Deliverable | Verification method |
|-------------|---------------------|
| zefs keys registered | `grep "state/debug" pkg/zefs/keys.go` |
| CLI commands wired | `grep "RegisterLocal.*debug" cmd/ze/debug/register.go` |
| Resolution logic tested | `go test ./internal/core/slogutil/ -run TestResolveDebug` |
| Enable/disable tested | `go test ./cmd/ze/debug/ -run TestDebug` |
| Startup apply tested | `go test ./internal/core/slogutil/ -run TestApplyDebugFlags` |
| Functional tests exist | `ls test/ui/debug-enable-show.ci test/ui/debug-global-override.ci` |
| Docs updated | `grep -l debug docs/guide/command-reference.md` |

### Security Review Checklist (/implement stage 11)

| Check | What to look for |
|-------|-----------------|
| Input validation | Subsystem names validated against registry; reject unknown names |
| Resource exhaustion | Fixed set of subsystem keys (no user-created arbitrary keys); zefs write is bounded |
| Information disclosure | Debug output shows subsystem names (already public via `log levels`) |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior |
| Lint failure | Fix inline |
| Functional test fails | Check AC; if AC wrong -> DESIGN; if AC correct -> IMPLEMENT |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |

### Failed Approaches
| Approach | Why abandoned | Replacement |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |

## Design Insights

### Key Design Decisions

1. **Hierarchical subsystem matching.** `debug enable bgp` enables all `bgp.*` subsystems
   (bgp.reactor, bgp.fsm, bgp.routes, etc.). This matches the existing env var hierarchy
   where `ze.log.bgp=debug` covers all bgp.* children. The zefs key `state/debug/bgp`
   means "all subsystems with prefix bgp.". The resolution logic walks the subsystem
   list and checks if the subsystem name starts with the debug key name.

2. **Restore level on disable.** When disabling debug, restore to the env-var/config-determined
   level by re-running `getLogEnv()`. This is the source of truth for non-debug levels.
   Requires exporting `getLogEnv` (or extracting the restore logic into a dedicated function).

3. **Offline vs online distinction.** `ze debug enable bgp` as an offline command writes
   to zefs but cannot notify the running daemon. The daemon picks up the change on next
   restart. For immediate effect, the user uses the CLI session (`debug enable bgp`
   inside `ze cli`), which triggers the RPC path. This matches how `ze data` (offline
   zefs) works separately from `log set` (online RPC).

## RFC Documentation

N/A - not a protocol feature.

## Implementation Summary

### What Was Implemented
- [pending]

### Bugs Found/Fixed
- [pending]

### Documentation Updates
- [pending]

### Deviations from Plan
- [pending]

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
| Debug flags persist across restart | functional test | `test/ui/debug-enable-show.ci` |
| Per-subsystem + global override | unit test | `TestResolveDebugState` |
| CLI toggle works | functional test | `test/ui/debug-enable-show.ci` |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied
- [pending]

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
- [ ] AC-1..AC-10 all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes
- [ ] Feature code integrated (`cmd/ze/debug/`, `internal/core/slogutil/debug.go`)
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
- [ ] Functional tests for end-to-end behavior
- [ ] Goal Validation table filled

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/790-debug-flags.md`
- [ ] Summary included in commit
