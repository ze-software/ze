# Spec: plugin-event-bus-across-the-process-boundary

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | plugin |
| Priority | P1 |
| Release reason | A plugin that takes its EventBus from `Registration.ConfigureEventBus` holds no bus when it runs as an external process: ddos-detect refuses its config and stops the startup tier, and others drop their events with no log line. |
| RFC gaps | - |
| Depends | - |
| Phase | Defect found and traced at the producer; design owed |
| Handoff | - |
| Updated | 2026-11-10 |

## Task

Every plugin that reaches the EventBus works when it runs as an external
process, or refuses to start external with a message that says what is wrong,
why, and the config that fixes it.

`Registration.ConfigureEventBus` (`internal/component/plugin/registry/registry.go`)
is called only by the in-process spawn (`internal/component/plugin/inprocess.go`).
No forked path calls it, and `le mock plugin-external`
(`internal/test/cli/cmd_plugin_external.go` `CmdPluginExternal`) does not
either. A forked plugin therefore holds a nil bus:

- ddos-detect `OnConfigure` calls `loadBus` (`internal/plugins/ddos/detect/register.go`),
  which refuses with "ddos-detect: event bus not configured", so plugin startup
  fails at stage Config.
- The other plugins that fill a package pointer from `ConfigureEventBus`
  (ddos-*, anomaly-*, kernel, connected, redistribute, fib-*, ldp, rsvpte,
  isis, mrt, vrrp, flowexport, cos) each need their nil branch read: refuse,
  skip silently, or panic.

`test/plugin/ddos-detect-external-warns.ci` masks the ddos-detect failure: it
tears down after the start warning `warnIfExternal` logs, before Config.
`test/draft/plugin/ddos-detect-external-blind.ci`
(spec-ddos-blind-detector-diagnostic, AC-9) goes green when this spec lands.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| `Registration.ConfigureEventBus` | `internal/component/plugin/registry/registry.go`, `internal/component/plugin/inprocess.go` | Hands the daemon's bus to an in-process plugin before `RunEngine` | To decide: replaced for every transport, or kept as the in-process shortcut of an SDK route |
| SDK event emit and subscribe | `pkg/plugin/sdk/sdk_engine.go` `EmitEvent`, `SubscribeEvents` | Emits and subscribes over the plugin protocol, on both transports | Candidate carrier: the adj-rib-in and bgp-rs fixes in `plan/journal/process-global-read-across-the-plugin-boundary.md` moved to it |
| Forked metrics publish | `ze-plugin-engine:metrics-publish` (15979b0b53) | Replaced `ConfigureMetrics` for a forked plugin | Precedent for the same seam shape |

## Required Reading

- [ ] `ai/doors/system.md`, `ai/doors/plugin-infra.md`, `ai/doors/system-plugins.md` - the process boundary
- [ ] `docs/architecture/plugin/plugin-system.md` - "The process boundary", "Communication patterns"
- [ ] `docs/architecture/api/process-protocol.md` - event delivery to a forked plugin

## Current Behavior

**Source files read:**
- [ ] `internal/component/plugin/inprocess.go` - the only `ConfigureEventBus` caller, and only when `registry.GetEventBus` answers
- [ ] `internal/test/cli/cmd_plugin_external.go` - forked launcher, no bus
- [ ] `internal/plugins/ddos/detect/register.go` - `loadBus` refuses Config

**Behavior to change:**
- A forked plugin holds no bus.

Deployment source inspection on 2026-11-13 confirms the redistribute instance
of AC-2. `internal/plugins/redistribute/redistribute.go` `run` warns and returns
when `getEventBus` is nil, before `subscribe` installs producer route-change
subscriptions. `register.go` supplies that bus through `ConfigureEventBus`,
but its SDK startup subscriptions cover only BGP state. The in-process spawn
in `internal/component/plugin/inprocess.go` invokes `ConfigureEventBus`; the
external process cannot receive that package pointer. No external-orchestrator
runtime proof was run for this capture. The existing 2026-11-10 journal row
already covers this consumer. Deployment's connected SDK repair is a subset
of this owner's work and does not establish that all consumers are repaired.

## Data Flow

### Entry Point
- `plugin { external <name> { run ...; } }` starting a plugin that uses the EventBus.

### Transformation Path
1. To design.

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| Engine ↔ forked plugin | To design: the plugin protocol (`EmitEvent`, `SubscribeEvents`) | No |

### Integration Points
- `registry.Registration.ConfigureEventBus` and each consumer's package bus pointer.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| external ddos-detect | → | bus on the forked path | `test/plugin/ddos-detect-external-blind.ci` (promoted from the draft) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | ddos-detect runs external | Config is accepted, the detector arms, `show doctor` reports `doctor-ddos-detect-blind` with the external cause | None |
| AC-2 | Every other `ConfigureEventBus` consumer runs external | It emits and receives its events, or refuses to start with an operator message | None |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| To design | | AC-2 | |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ddos-detect-external-blind` | `test/draft/plugin/ddos-detect-external-blind.ci` | AC-1 | draft, red until this spec lands |

## Files to Modify
- To design: `internal/component/plugin/registry/registry.go`, the consumers listed in Task

## Implementation Steps
1. To design.

## Design Work Owed

- Read every `ConfigureEventBus` consumer and classify its nil branch.
- Choose the route (SDK-backed `ze.EventBus` for a forked plugin, or per-plugin
  refusal) against `ai/doors/system.md` "A plugin can run in another process"
  and `ai/doors/plugin-infra.md` "Every feature works over both transports".
- Extend `./le plugin boundary check` so a new nil-bus read is refused.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `process-global-read-across-the-plugin-boundary` | 2026-11-10 | ddos-detect `loadBus` refuses Config when forked: `ConfigureEventBus` runs only in-process |

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

### Goal Gates
- [ ] `./le test functional plugin` covering the promoted `ddos-detect-external-blind.ci` and `ddos-detect-external-warns.ci` pass
- [ ] Design approved
- [ ] Every consumer's nil branch fixed or refusing with an operator message
- [ ] `test/draft/plugin/ddos-detect-external-blind.ci` promoted and green
- [ ] `test/plugin/ddos-detect-external-warns.ci` asserts past Config
