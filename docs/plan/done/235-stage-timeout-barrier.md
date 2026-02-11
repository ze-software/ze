# Spec: stage-timeout-barrier

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md` - workflow rules
3. `internal/plugin/server.go` - stageTransition, progressThroughStages
4. `internal/plugin/startup_coordinator.go` - WaitForStage, advanceStage

## Task

Fix flaky plugin functional tests caused by incorrect stage barrier timeout. The per-plugin 5-second timeout in `stageTransition` includes time waiting for OTHER plugins at the barrier, causing fast plugins to timeout when slow plugins (Python subprocesses) take longer to start under load.

Additionally, add an environment variable `ze.plugin.stage.timeout` to override the default stage timeout without modifying config files (useful for test environments and slow CI machines).

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/capability-contract.md` - startup protocol stages

### Source Files
- [ ] `internal/plugin/server.go` - stageTransition (line 74), defaultStageTimeout (line 29)
- [ ] `internal/plugin/startup_coordinator.go` - WaitForStage, advanceStage, lastAdvance tracking
- [ ] `internal/plugin/startup_test.go` - existing coordinator tests
- [ ] `internal/plugin/types.go` - PluginConfig.StageTimeout (line 552)

**Key insights:**
- `stageTransition` creates a fresh 5s timeout per call, but `WaitForStage` blocks until ALL plugins complete the current stage
- Under load (20 concurrent tests × ~4 processes), subprocess fork+startup can exceed 5s
- The failing plugin varies per run (rib, refresh, rib-reconnect, attributes) — whichever draws the short straw
- Always fails at `waiting_for=Config` — the first barrier after Registration
- Tests pass 100% in isolation, fail ~30% under full suite load
- Per-plugin config `timeout` already exists (spec 109) but test configs don't set it

## Current Behavior

**Source files read:**
- [ ] `internal/plugin/server.go` - `stageTransition` at line 74 creates `context.WithTimeout(s.ctx, timeout)` where timeout is per-plugin config or 5s default. This timeout starts from when THIS plugin calls `WaitForStage`, NOT from when the stage began.
- [ ] `internal/plugin/startup_coordinator.go` - `advanceStage` at line 207 advances `currentStage` and closes the channel to unblock waiters. No timestamp recorded.

**Behavior to preserve:**
- Per-plugin config `timeout` overrides default (spec 109)
- Coordinator barrier semantics: ALL plugins must complete stage N before any proceed to N+1
- `PluginFailed` propagation: one plugin's failure aborts all others
- 5-stage protocol sequence: Registration → Config → Capability → Registry → Ready

**Behavior to change:**
- Barrier timeout measured from stage start time, not from when individual plugin reaches barrier
- New env var `ze.plugin.stage.timeout` to override default without config changes

## Data Flow

### Entry Point
- `runPluginPhase` creates coordinator and starts all processes
- Each process goroutine calls `handleProcessStartupRPC`

### Transformation Path
1. `ProcessManager.StartWithContext` forks all plugin processes
2. Per-plugin goroutine: `handleProcessStartupRPC` reads first RPC from plugin (no timeout, uses `s.ctx`)
3. After receiving registration: `progressThroughStages` calls `stageTransition`
4. `stageTransition`: marks stage complete → creates timeout context → `WaitForStage` (barrier)
5. Barrier blocks until coordinator advances (all plugins complete current stage)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Server → Coordinator | `StageComplete()` / `WaitForStage()` | [ ] |
| Coordinator internal | `advanceStage()` closes channel | [ ] |

### Integration Points
- `stageTransition` in `server.go` — timeout calculation changes
- `StartupCoordinator` in `startup_coordinator.go` — tracks stage start time
- `defaultStageTimeout` const in `server.go` — env var override

### Architectural Verification
- [ ] No bypassed layers
- [ ] No unintended coupling
- [ ] No duplicated functionality
- [ ] Zero-copy not applicable

## Design Decisions

### Stage Start Time

The coordinator records a timestamp when each stage begins:
- Construction time for the initial stage (Registration)
- `advanceStage` time for subsequent stages

`stageTransition` uses `stageStartTime + timeout` as the deadline instead of `time.Now() + timeout`.

### Env Var Convention

`ze.plugin.stage.timeout` with underscore fallback `ze_plugin_stage_timeout`, following the existing `ze.log.*` pattern. Parsed once at init time via `os.Getenv`. Must be a valid `time.Duration` string (e.g., `10s`, `15s`, `1m`).

### Timeout Priority

| Priority | Source | Example |
|----------|--------|---------|
| 1 (highest) | Per-plugin config `timeout` | `timeout 30s;` in config |
| 2 | Env var `ze.plugin.stage.timeout` | `ze.plugin.stage.timeout=15s` |
| 3 (lowest) | Code default | `defaultStageTimeout = 5s` |

### Coordinator SetStartTime

After `ProcessManager.StartWithContext` returns (all processes forked), the coordinator's start time is set. This ensures the Registration stage timeout includes fork time, not time before processes exist.

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestBarrierTimeoutFromStageStart` | `internal/plugin/startup_test.go` | Slow plugin doesn't cause fast plugin to timeout when within stageStart+timeout | |
| `TestBarrierTimeoutExpired` | `internal/plugin/startup_test.go` | Plugin that exceeds stageStart+timeout still fails | |
| `TestStageStartTimeAdvances` | `internal/plugin/startup_test.go` | Each advanceStage updates the start time | |
| `TestEnvVarStageTimeout` | `internal/plugin/server_test.go` | `ze.plugin.stage.timeout` env var overrides default | |
| `TestEnvVarStageTimeoutUnderscore` | `internal/plugin/server_test.go` | `ze_plugin_stage_timeout` underscore form works | |
| `TestEnvVarStageTimeoutInvalid` | `internal/plugin/server_test.go` | Invalid env var value falls back to default | |
| `TestTimeoutPriorityConfigOverEnv` | `internal/plugin/server_test.go` | Per-plugin config timeout beats env var | |

### Boundary Tests
N/A - timeout values are durations, not bounded numeric fields.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| Existing plugin suite | `test/plugin/*.ci` | All 45 tests pass under full load (3 consecutive runs) | |

## Files to Modify

- `internal/plugin/startup_coordinator.go` - Add `stageStartTime` field, update in `advanceStage`, expose via `StageStartTime()` method
- `internal/plugin/server.go` - Change `stageTransition` to use stage start time for deadline; add `stageTimeoutFromEnv()` helper; call `coordinator.SetStartTime()` after process spawn
- `internal/plugin/startup_test.go` - Add barrier timeout tests

## Files to Create

None.

## Implementation Steps

1. **Write unit tests** - Tests for barrier timeout from stage start, env var override
   → **Review:** Do tests verify the timing is from stage start, not from WaitForStage call?

2. **Run tests** - Verify FAIL (paste output)
   → **Review:** Tests fail because stageStartTime doesn't exist yet?

3. **Add stageStartTime to coordinator** - Record timestamp in constructor and advanceStage
   → **Review:** Thread-safe? Uses existing mutex?

4. **Change stageTransition deadline** - Use `stageStartTime + timeout` instead of `now + timeout`
   → **Review:** Handles case where stageStartTime + timeout is already past (returns immediately)?

5. **Add env var support** - `stageTimeoutFromEnv()` reads `ze.plugin.stage.timeout` / `ze_plugin_stage_timeout`
   → **Review:** Parsed once? Invalid values fall back to default?

6. **Add SetStartTime** - Called after ProcessManager.StartWithContext returns
   → **Review:** Called before any plugin can complete Registration?

7. **Run tests** - Verify PASS (paste output)
   → **Review:** All existing coordinator tests still pass?

8. **Stress test** - Run `ze-test bgp plugin --all` 5 times
   → **Review:** Zero timeouts across all runs?

9. **Verify all** - `make lint && make test && make functional` (paste output)

## Checklist

### 🧪 TDD
- [ ] Tests written
- [ ] Tests FAIL
- [ ] Implementation complete
- [ ] Tests PASS
- [ ] Feature code integrated into codebase (`internal/*`)
- [ ] Functional tests verify end-user behavior (stress test)

### Verification
- [ ] `make lint` passes
- [ ] `make test` passes
- [ ] `make functional` passes

### Documentation
- [ ] Required docs read
- [ ] Go env var pattern follows existing `ze.*` convention

## Implementation Summary

### What Was Implemented
- `stageStartTime` field in `StartupCoordinator`, set at construction and updated in `advanceStage`
- `StageStartTime()` and `SetStartTime()` methods on coordinator
- `stageTransition` changed from `context.WithTimeout(now+timeout)` to `context.WithDeadline(stageStart+timeout)`
- `stageTimeoutFromEnv()` helper with dot/underscore env var fallback
- `SetStartTime()` called after `ProcessManager.StartWithContext` in `runPluginPhase`
- Default timeout kept at 5s (production-appropriate); test runner sets `ze_plugin_stage_timeout=10s`
- Test runner env setup in `internal/test/runner/runner.go` at both non-orchestrated and orchestrated paths
- 8 new tests (3 coordinator + 5 env var)

### Bugs Found/Fixed
- Default 5s timeout was too aggressive for concurrent plugin startups under load
- Fix: correct measurement point (stage start, not barrier wait) + env var override (10s) in test runner

### Design Insights
- `WithTimeout` vs `WithDeadline`: WithTimeout measures from now (each caller gets different deadline); WithDeadline uses absolute time (all callers share same deadline from stage start)
- Two-part fix needed: correct measurement point AND adequate timeout for test environments
- Production keeps conservative 5s default; test runner opts into 10s via env var

### Deviations from Plan
- Added `TestEnvVarStageTimeoutDotPrecedence` test (not in spec, validates dot-over-underscore priority)
- Default kept at 5s (not changed to 10s); test runner sets env var instead (per user direction)
- Test runner (`internal/test/runner/runner.go`) modified to set `ze_plugin_stage_timeout=10s` (not in original plan)
- Test names use subtests under `TestStageSynchronization` rather than standalone functions (matches existing test structure)

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Barrier timeout from stage start | ✅ Done | `server.go:112` | `WithDeadline(stageStartTime + timeout)` |
| Env var `ze.plugin.stage.timeout` | ✅ Done | `server.go:34-46` | `stageTimeoutFromEnv()` |
| Underscore form `ze_plugin_stage_timeout` | ✅ Done | `server.go:35` | Checked as fallback |
| Priority: config > env > default | ✅ Done | `server.go:101-106` | Config checked first, then env |
| Flaky tests eliminated | ✅ Done | 4/4 consecutive `make functional` passes | Was ~70% under load, now 100% |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestBarrierTimeoutFromStageStart` | ✅ Done | `startup_test.go:64` | Subtest of TestStageSynchronization |
| `TestBarrierTimeoutExpired` | ✅ Done | `startup_test.go:114` | Subtest of TestStageSynchronization |
| `TestStageStartTimeAdvances` | ✅ Done | `startup_test.go:93` | Subtest of TestStageSynchronization |
| `TestEnvVarStageTimeout` | ✅ Done | `server_test.go:525` | |
| `TestEnvVarStageTimeoutUnderscore` | ✅ Done | `server_test.go:533` | |
| `TestEnvVarStageTimeoutInvalid` | ✅ Done | `server_test.go:541` | |
| `TestTimeoutPriorityConfigOverEnv` | ✅ Done | `server_test.go:549` | |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/plugin/startup_coordinator.go` | ✅ Modified | stageStartTime field + 3 methods |
| `internal/plugin/server.go` | ✅ Modified | stageTransition + stageTimeoutFromEnv + SetStartTime call |
| `internal/plugin/startup_test.go` | ✅ Modified | 3 new subtests |
| `internal/plugin/server_test.go` | ✅ Modified | 5 new tests (not in original plan, needed for env var) |
| `internal/test/runner/runner.go` | ✅ Modified | `ze_plugin_stage_timeout=10s` in both env paths |

### Audit Summary
- **Total items:** 17
- **Done:** 17
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 2 (default kept at 5s + test runner env var; extra test added)
