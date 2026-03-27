# Spec: prometheus-plugin-health

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | spec-prometheus-deep (Phase 1 histogram interface) |
| Phase | - |
| Updated | 2026-03-26 |

> **NOTE:** All design decisions below are INFORMATIVE and must be reviewed before implementation begins. They reflect initial research only.

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/api/process-protocol.md` - plugin lifecycle protocol
4. `internal/component/plugin/process/process.go` - Process struct
5. `internal/component/plugin/process/manager.go` - ProcessManager and respawn logic

## Task

Add Prometheus metrics for plugin health visibility: current status per plugin, crash restart counts, and event delivery counts. These were listed in `spec-prometheus-deep.md` Phase 4 but deferred because the plugin infrastructure (`process/`, `server/`) has no access to the metrics registry today.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/process-protocol.md` - plugin 5-stage startup protocol
  → Decision: (to be reviewed)
  → Constraint: (to be reviewed)
- [ ] `docs/architecture/core-design.md` - system component boundaries
  → Constraint: (to be reviewed)

### Source Files (INFORMATIVE -- must re-verify before implementation)

- [ ] `internal/component/plugin/registration.go` - PluginStage type (7 stages: StageInit through StageRunning)
  → Constraint: (to be reviewed) Stage values 0-6 are iota constants. StageInit=0, StageRunning=6.
- [ ] `internal/component/plugin/process/process.go` - Process struct with `running` (atomic.Bool) and `stage` (atomic.Int32)
  → Constraint: (to be reviewed) Process has no metrics access today. Stage is already atomic -- reading is zero-cost.
- [ ] `internal/component/plugin/process/manager.go` - ProcessManager with `totalRespawns` map and `Respawn()` method
  → Constraint: (to be reviewed) RespawnLimit=5 per 60s window, MaxTotalRespawns=20. `totalRespawns` already tracked per plugin.
- [ ] `internal/component/plugin/process/delivery.go` - EventDelivery and `Deliver()` returning bool
  → Constraint: (to be reviewed) `Deliver()` returns false if process stopping. True = successfully enqueued.
- [ ] `internal/component/bgp/server/events.go` - `onMessageReceived()` and `onMessageBatchReceived()` event dispatch
  → Constraint: (to be reviewed) These are the two call sites where events reach plugins via `proc.Deliver()`.
- [ ] `internal/component/plugin/server/server.go` - Server struct, no metrics field
  → Constraint: (to be reviewed) Server holds `procManager` atomic.Pointer but no metrics registry.
- [ ] `internal/component/plugin/server/config.go` - ServerConfig, no metrics field
  → Constraint: (to be reviewed) Must add MetricsRegistry field to thread metrics through.
- [ ] `internal/component/bgp/reactor/reactor.go` lines 709-732 - where ServerConfig is constructed and Server created
  → Constraint: (to be reviewed) Reactor has `metricsRegistry` -- this is where it would be passed to ServerConfig.

**Key insights:** (INFORMATIVE -- to be verified)
- Plugin infrastructure (`process/`, `server/`) currently has zero metrics access
- Metrics registry must be threaded: Reactor -> ServerConfig -> Server -> ProcessManager
- Process already tracks stage (atomic) and running (atomic) -- no new state needed for status
- ProcessManager already counts respawns per plugin -- no new state needed for restarts
- Event delivery already returns success/failure -- counter increment is the only addition
- Plugin count is config-bounded (typically 5-10) so label cardinality is not a concern

## Current Behavior (MANDATORY)

**Source files read:** (INFORMATIVE -- must re-read before implementation)
- [ ] `internal/component/plugin/process/process.go` - manages individual plugin subprocess lifecycle
- [ ] `internal/component/plugin/process/manager.go` - supervises all plugin processes, handles respawn
- [ ] `internal/component/plugin/process/delivery.go` - event delivery to plugin processes
- [ ] `internal/component/bgp/server/events.go` - BGP event dispatch to subscribed plugins
- [ ] `internal/component/plugin/server/server.go` - plugin server, manages process lifecycle
- [ ] `internal/component/plugin/server/config.go` - server configuration struct

**Behavior to preserve:**
- Plugin startup 5-stage protocol unchanged
- Respawn limits (5/60s window, 20 cumulative) unchanged
- Event delivery semantics unchanged (enqueue, not synchronous)
- Process supervision logic unchanged

**Behavior to change:**
- Add metrics registry field to ServerConfig and Server
- Add metrics counter increments at 3 sites (status, respawn, delivery)

## Data Flow (MANDATORY)

### Entry Point
- Metrics registry created by reactor at startup
- Passed to ServerConfig when plugin server is constructed

### Transformation Path
1. Reactor passes `metrics.Registry` to `ServerConfig`
2. `Server.NewServer()` stores registry, passes to `ProcessManager`
3. `ProcessManager` creates plugin metrics (3 metrics) from registry
4. On stage change: `Process.SetStage()` triggers status gauge update
5. On respawn: `ProcessManager.Respawn()` increments restart counter
6. On event delivery: `events.go` increments delivered counter after `Deliver()` returns true

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Reactor -> Server | MetricsRegistry field in ServerConfig | [ ] |
| Server -> ProcessManager | Constructor parameter or setter | [ ] |
| ProcessManager -> metrics | Direct counter increment | [ ] |

### Integration Points
- `reactor.go:709` ServerConfig construction -- add MetricsRegistry field
- `server.go` NewServer -- store and pass registry
- `process/manager.go` -- create and increment metrics

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)

## Metrics (INFORMATIVE -- names and types to be reviewed)

| Metric | Type | Labels | Increment Site |
|--------|------|--------|----------------|
| `ze_plugin_status` | GaugeVec | plugin | `Process.SetStage()` or periodic poll |
| `ze_plugin_restarts_total` | CounterVec | plugin | `ProcessManager.Respawn()` |
| `ze_plugin_events_delivered_total` | CounterVec | plugin | `events.go` after `Deliver()` returns true |

## Design Decisions (INFORMATIVE -- all require review)

### Decision: How to update plugin status gauge

**Option A: Event-driven (on stage change)**
- Increment at `Process.SetStage()` -- immediate, no polling goroutine
- Requires adding a callback or metrics pointer to Process
- Pro: Real-time. Con: Process gains metrics dependency.

**Option B: Periodic poll (piggyback on reactor metrics loop)**
- Reactor reads `Process.Stage()` and `Running()` every 10s
- Pro: No changes to Process. Con: Crosses layer boundary, 10s staleness.

**Option C: Periodic poll in ProcessManager**
- ProcessManager runs its own metrics loop
- Pro: Clean layer ownership. Con: Adds a goroutine.

> To be decided before implementation. Leaning toward A (matches how reactor peer stats work).

### Decision: Thread registry through chain vs callbacks

**Option A: Thread metrics.Registry**
- Add field to ServerConfig, Server, ProcessManager
- Each component registers and increments its own metrics
- Follows existing SetMetricsRegistry pattern (RIB, GR)

**Option B: Callbacks to reactor**
- ProcessManager fires callbacks, reactor increments counters
- No metrics import in process/ package

> To be decided. Option A is more consistent with existing patterns.

### Decision: Disabled plugin status value

- PluginStage is 0-6 (iota). A disabled plugin (respawn limit exceeded) has no stage value.
- Option: Use -1 for disabled, or a separate `ze_plugin_disabled` gauge.

> To be decided.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Reactor startup with metrics registry | -> | Plugin metrics registered | `TestPluginMetricsRegistered` |
| Plugin reaches StageRunning | -> | Status gauge set to 6 | `TestPluginStatusMetric` |
| Plugin crash + respawn | -> | Restart counter increments | `TestPluginRestartMetric` |
| Event dispatched to plugin | -> | Delivery counter increments | `TestPluginEventDeliveryMetric` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Plugin reaches StageRunning | `ze_plugin_status{plugin=X}` reflects running state |
| AC-2 | Plugin crashes and respawns | `ze_plugin_restarts_total{plugin=X}` increments by 1 |
| AC-3 | Event delivered to plugin via Deliver() | `ze_plugin_events_delivered_total{plugin=X}` increments |
| AC-4 | Plugin stopped normally | `ze_plugin_status{plugin=X}` reflects stopped state |
| AC-5 | Plugin disabled (respawn limit exceeded) | `ze_plugin_status{plugin=X}` reflects disabled state |
| AC-6 | Metrics registry not set (nil) | No panic, counters silently not incremented |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPluginMetricsRegistered` | `process/manager_test.go` | All 3 metrics created from registry | |
| `TestPluginStatusMetric` | `process/process_test.go` | Status gauge updates on stage change | |
| `TestPluginRestartMetric` | `process/manager_test.go` | Restart counter increments in Respawn() | |
| `TestPluginEventDeliveryMetric` | `bgp/server/events_test.go` | Delivery counter increments on successful Deliver() | |
| `TestPluginMetricsNilRegistry` | `process/manager_test.go` | No panic when registry is nil | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| PluginStage | 0-6 | 6 (StageRunning) | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-plugin-metrics` | `test/plugin/*.ci` | Start ze with metrics + plugin, scrape /metrics, verify plugin counters | |

### Future (if deferring any tests)
- None planned

## Files to Modify
- `internal/component/plugin/server/config.go` - add MetricsRegistry field
- `internal/component/plugin/server/server.go` - store and pass registry
- `internal/component/plugin/process/manager.go` - create metrics, increment restart counter
- `internal/component/plugin/process/process.go` - increment status gauge on stage change
- `internal/component/bgp/server/events.go` - increment delivery counter
- `internal/component/bgp/reactor/reactor.go` - pass metricsRegistry to ServerConfig

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | - |
| CLI commands/flags | No | - |
| Editor autocomplete | No | - |
| Functional test for new RPC/API | No | - |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` - add plugin health metrics |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | No | - |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented? | No | - |
| 10 | Test infrastructure changed? | No | - |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` - metrics threading |

## Files to Create
- None (all changes in existing files)
- `test/plugin/NNN-plugin-metrics.ci` - functional test

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

1. **Phase: Registry threading** -- Add MetricsRegistry to ServerConfig, Server, ProcessManager
   - Tests: `TestPluginMetricsRegistered`, `TestPluginMetricsNilRegistry`
   - Files: `config.go`, `server.go`, `manager.go`, `reactor.go`
   - Verify: tests fail -> implement -> tests pass

2. **Phase: Status gauge** -- Increment plugin status gauge on stage transitions
   - Tests: `TestPluginStatusMetric`
   - Files: `process.go`
   - Verify: tests fail -> implement -> tests pass

3. **Phase: Restart counter** -- Increment restart counter in Respawn()
   - Tests: `TestPluginRestartMetric`
   - Files: `manager.go`
   - Verify: tests fail -> implement -> tests pass

4. **Phase: Delivery counter** -- Increment events delivered counter after Deliver() returns true
   - Tests: `TestPluginEventDeliveryMetric`
   - Files: `events.go`
   - Verify: tests fail -> implement -> tests pass

5. **Functional tests** -- Create after feature works. Cover user-visible behavior.
6. **Full verification** -- `make ze-verify`
7. **Complete spec** -- Fill audit tables, write learned summary

### Critical Review Checklist (/implement stage 5)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Gauge values match PluginStage constants (0-6, or -1 for disabled) |
| Nil safety | All metrics increments guarded by nil check |
| Layer boundaries | ProcessManager owns plugin metrics, events.go increments delivery counter |
| No hot-path overhead | Counter increments are O(1) atomics, no allocations |

### Deliverables Checklist (/implement stage 9)
| Deliverable | Verification method |
|-------------|---------------------|
| 3 new Prometheus metrics registered | `grep ze_plugin internal/component/plugin/process/manager.go` |
| ServerConfig has MetricsRegistry field | `grep MetricsRegistry internal/component/plugin/server/config.go` |
| Reactor passes registry | `grep metricsRegistry internal/component/bgp/reactor/reactor.go` |
| Functional test exists | `ls test/plugin/*plugin-metrics*` |

### Security Review Checklist (/implement stage 10)
| Check | What to look for |
|-------|-----------------|
| Label cardinality | Plugin names from config only, not user input -- bounded |
| Resource exhaustion | Counter increment is O(1), no unbounded growth |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior -> RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural -> DESIGN phase |
| Functional test fails | Check AC; if AC wrong -> DESIGN; if AC correct -> IMPLEMENT |
| Audit finds missing AC | Back to relevant phase and implement |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
