# Spec: doctor-reports-failed-plugin-startup

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | plugin |
| Priority | P1 |
| Release reason | A configured plugin whose startup failed can leave ze running without it while `ze doctor` reports nothing, and a failed explicit-plugin phase leaves ze running with the later startup phases undone. |
| RFC gaps | - |
| Depends | - |
| Phase | Defects found and traced at the producer (read, not run); design owed |
| Handoff | - |
| Updated | 2026-11-10 |

## Task

No configured plugin that failed startup leaves ze running while `ze doctor`
is silent: either ze stops, or doctor reports the plugin with its cause and the
command or config change that fixes it.

**Doctor is silent about a failed plugin.** A plugin fails Stage 2 when its
`OnConfigure` refuses (`deliverConfigRPC`,
`internal/component/plugin/server/startup.go`). The engine calls
`coord.PluginFailed` and `proc.Stop`, and sets `startupErr` only when
`configRefusalIsFatal` holds, which needs `FatalOnConfigError` on the
registration. `runPluginPhaseWithin` then calls `rollbackStartupProcess`; for a
plugin below `StageRunning` that is `stopRemovedProcess`
(`internal/component/plugin/server/startup_removal.go`), which removes it from
the process manager. `stopDaemonOnStartupFailure`
(`internal/component/plugin/server/failure_policy.go`) stops ze only for a
`rpc.FailureFatal` plugin. `DoctorCheckPlugins`
(`internal/component/plugin/server/server.go`) walks `pm.AllProcesses()`, so
the failed plugin's checks are no longer listed and `show doctor` is clean. The
only record is the ERROR log line. Instance: an external ddos-detect refuses
with "ddos-detect: event bus not configured"
(`spec-plugin-event-bus-across-the-process-boundary`), and doctor then shows
neither `ddos-detect-rate-feed` nor any failure. `ai/META-DECISIONS.md`: a
problem class doctor does not report is owed a doctor check.

**A failed explicit phase leaves ze half-started.** In
`Server.runPluginStartup` (`startup.go`), a failed phase 1 (config-path
plugins) sets `startupErr`. A failed phase 2 (explicit `plugin { external ... }`
plugins) only logs "explicit plugin startup failed", signals startup complete
and returns: phases 3 (auto-load for unclaimed families), 4 (custom event
types) and 5 (custom send types) never run, and ze keeps running.

**Owner ruling that bounds the design.** `plan/learned/DESIGN-HISTORY.md`,
Plugin system: "ANY plugin failure aborts the whole startup tier, and that is
CORRECT" (2026-08-11): a router that starts with half its features is worse
than one that does not start, so a config-rejection failure is never made
survivable. Running phases 3 to 5 after a phase-2 failure is therefore not the
fix. The design chooses between stopping ze as phase 1 does, and reporting the
failed plugin and the phases not run through doctor, against that ruling and
the failure policy in `failure_policy.go`.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Coordinator failure record | `coord.PluginFailed` in `deliverConfigRPC` and the stage-timeout path | Records the first failure and aborts the tier | Candidate source of the doctor finding: to read whether it outlives startup |
| Plugin failure policy | `failure_policy.go` `pluginFailurePolicy`, `stopDaemonOnStartupFailure`; `registry.Registration.FatalOnConfigError` | Stops ze for a fatal plugin; a non-fatal one is dropped | Kept or extended: to decide against the owner ruling |
| Phase-1 refusal | `runPluginStartup` sets `startupErr` on a config-path failure | ze does not run half-started | Candidate for phase 2 |
| Plugin doctor checks | `DoctorCheckPlugins` (`server.go`), `internal/component/doctor` | Lists the checks of running plugins | Extends, or a daemon doctor check beside it: to design |
| Doctor coverage audit | `plan/release/validation/spec-doctor-coverage-1-audit-and-gate.md` | Inventories problem classes doctor misses | This class is one of them; the code lands here |

## Required Reading

- [ ] `ai/doors/system.md`, `ai/doors/plugin-infra.md` - the plugin server, the "One plugin failure stops the startup tier" trap
- [ ] `docs/architecture/plugin/plugin-system.md` - startup stages, failure policy
- [ ] `plan/learned/DESIGN-HISTORY.md` - Plugin system, the 2026-08-11 fail-closed ruling

## Current Behavior

**Source files read:**
- [ ] `internal/component/plugin/server/startup.go` - `runPluginStartup`, `runPluginPhaseWithin`, `deliverConfigRPC`, `configRefusalIsFatal`
- [ ] `internal/component/plugin/server/startup_removal.go` - `rollbackStartupProcess`, `stopRemovedProcess`
- [ ] `internal/component/plugin/server/failure_policy.go` - `stopDaemonOnStartupFailure`
- [ ] `internal/component/plugin/server/server.go` - `DoctorCheckPlugins`

**Behavior to change:**
- A configured plugin whose startup failed is invisible to `ze doctor` while ze runs.
- A failed explicit-plugin phase leaves ze running with phases 3 to 5 undone and nothing reported.

## Data Flow

### Entry Point
- A configured plugin whose startup fails at any stage, on boot or reload.

### Transformation Path
1. To design.

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| Engine ↔ doctor | To design | No |

### Integration Points
- The coordinator's failure record, `startupErr`, and the doctor's plugin walk.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| plugin refusing Stage 2 | → | ze stops, or the doctor finding | To design: a `.ci` with a plugin fixture that refuses `OnConfigure` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | A configured non-fatal plugin fails startup and ze keeps running | `ze doctor` reports a warning naming the plugin, the stage, the cause and the fix | None |
| AC-2 | The explicit-plugin phase fails | ze stops as for a phase-1 failure, or `ze doctor` names the phases not run; which one is the design decision above | None |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| To design | | AC-1, AC-2 | |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| To design | `test/plugin/` | AC-1 | |

## Files to Modify
- To design: `internal/component/plugin/server/startup.go`, `internal/component/plugin/server/server.go`

## Implementation Steps
1. To design.

## Design Work Owed

- Read whether the coordinator's failure record survives startup and reload,
  and whether `ze show plugin list` shows a failed plugin.
- Decide phase 2 against the fail-closed ruling: stop ze, or report.
- Choose the doctor surface: a daemon doctor check over the failure record, or
  a phase of the plugin walk.
- Set the diagnostic code in `internal/core/diagnostic/codes.go`.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `failed-start-reads-as-absent` | 2026-11-10 | A non-fatal plugin that fails startup is removed, so `show doctor` is clean |
| `one-failure-skips-independent-startup-work` | 2026-11-10 | A failed explicit-plugin phase skips phases 3 to 5 and ze runs half-started |

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

### Goal Gates
- [ ] Design approved
- [ ] A failed plugin either stops ze or is reported by `show doctor`, proven by a `.ci`
- [ ] `./le test functional plugin` covering the new `.ci`, and `go test -race ./internal/component/plugin/server` under `./le job run`, pass
