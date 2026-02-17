# Spec: chaos-web-controls

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `docs/architecture/chaos-web-dashboard.md` - "Control Architecture", "Parameterized Manual Trigger UI", "Replay Constraint"
3. `cmd/ze-bgp-chaos/web/` - foundation package
4. `cmd/ze-bgp-chaos/chaos/scheduler.go` - scheduler (needs Pause/Resume/SetRate)
5. `cmd/ze-bgp-chaos/orchestrator.go` - main event loop (needs control channel)

## Task

Add interactive controls to the chaos web dashboard: **control channel** from web server to orchestrator, **scheduler extensions** (pause/resume/setRate), **control panel UI** in sidebar, **parameterized trigger form** with per-action-type parameters, and **multi-select peers** from the table for targeted chaos.

All manual triggers flow through the normal event pipeline and are recorded in the NDJSON log for replay. Control actions (pause/resume/rate change) are logged as informational "control" records.

**Parent spec:** `docs/plan/spec-chaos-web-dashboard.md`
**Design doc:** `docs/architecture/chaos-web-dashboard.md`
**Depends on:** `spec-chaos-web-foundation` (web package, SSE, HTTP handlers), `spec-chaos-actions-v2` (parameterized action types)

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/chaos-web-dashboard.md` - Control Architecture, Parameterized Trigger UI, Replay Constraint
  -> Decision: Buffered control channel (capacity 16), non-blocking send, HTTP 503 if full
  -> Decision: Per-action parameter forms loaded via HTMX on dropdown change
  -> Constraint: Manual triggers must produce standard EventChaosExecuted (replayable)
  -> Decision: Control actions (pause/resume) logged as "control" record type (informational)
- [ ] `cmd/ze-bgp-chaos/chaos/scheduler.go` - Current scheduler (fire-and-forget)
  -> Decision: Add Pause(), Resume(), SetRate(), IsPaused() — mutex-protected
- [ ] `cmd/ze-bgp-chaos/orchestrator.go` - Event loop processes events sequentially
  -> Decision: Add select on both event channel and control channel
- [ ] `cmd/ze-bgp-chaos/main.go` - runOrchestrator, runScheduler goroutines
  -> Constraint: Scheduler runs in separate goroutine, needs mutex for new methods

**Key insights:**
- Control channel processed in same select as event channel — no priority issues
- Manual trigger: POST handler -> control channel -> orchestrator -> peer chaos channel -> event pipeline -> all consumers (including JSONLog)
- Scheduler pause is a simple boolean flag checked in Tick()
- RestartRun is the most complex: requires stopping all goroutines and re-entering runOrchestrator

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `cmd/ze-bgp-chaos/chaos/scheduler.go` - Tick() generates actions, no pause/resume
- [ ] `cmd/ze-bgp-chaos/orchestrator.go` - Event loop with `for ev := range events`
- [ ] `cmd/ze-bgp-chaos/main.go` - runScheduler goroutine, chaos channels
- [ ] `cmd/ze-bgp-chaos/report/jsonlog.go` - NDJSON event format

**Behavior to preserve:**
- Automatic scheduler behavior unchanged when not paused
- Event processing order unchanged
- NDJSON format backwards compatible

**Behavior to change:**
- Scheduler gets pause/resume/setRate methods
- Orchestrator event loop becomes a select on event + control channels
- NDJSON extended with "control" record type for non-chaos UI actions
- New POST endpoints for control actions

## Data Flow (MANDATORY)

### Entry Point
- Browser POST requests to /control/* endpoints (pause, resume, rate, trigger, stop, restart)
- HTTP handler validates input and creates a control command

### Transformation Path
1. Browser submits POST with form data (action type, peer selection, parameters)
2. HTTP handler validates parameters against action-type constraints
3. Handler creates typed command struct (PauseChaos, TriggerChaos, SetRate, etc.)
4. Handler sends command to buffered control channel (non-blocking, 503 if full)
5. Orchestrator reads command from control channel in select loop
6. Orchestrator dispatches: scheduler methods (pause/resume/rate) or peer chaos channel (trigger)
7. For triggers: peer executes action, emits EventChaosExecuted through normal pipeline
8. For control actions: orchestrator logs "control" record type to NDJSON

### Control Path (new)
1. Browser clicks "Pause" -> POST /control/chaos/pause
2. HTTP handler creates PauseChaos command, sends to control channel
3. Orchestrator reads control channel in select loop
4. Orchestrator calls scheduler.Pause()
5. Scheduler's next Tick() returns empty (paused)
6. HTTP handler returns updated control panel fragment

### Manual Trigger Path (new)
1. Browser submits trigger form -> POST /control/chaos/trigger (action, peers, params)
2. HTTP handler validates params, creates TriggerChaos command
3. Orchestrator reads command, sends ChaosAction to peer's chaos channel
4. Peer executes action, emits EventChaosExecuted
5. Event flows through Reporter to all consumers (including JSONLog -> NDJSON record)
6. SSE broadcasts event to browser dashboard

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| HTTP handler -> Orchestrator | Control channel (buffered, non-blocking) | [ ] |
| Orchestrator -> Scheduler | Method calls (Pause/Resume/SetRate) | [ ] |
| Orchestrator -> Peer | ChaosAction on peer's chaos channel | [ ] |
| Reporter -> JSONLog | EventChaosExecuted with chaos-params | [ ] |

### Integration Points
- `chaos/scheduler.go` — New Pause(), Resume(), SetRate(), IsPaused() methods (mutex-protected)
- `orchestrator.go` event loop — Extended with select on control channel alongside event channel
- `main.go` — Control channel created, passed to both WebDashboard and orchestrator
- `web/handlers.go` — POST handlers for /control/* endpoints
- `web/dashboard.go` — Accepts control channel, exposes to handlers
- `report/jsonlog.go` — "control" record type for non-chaos UI actions

### Architectural Verification
- [ ] No bypassed layers (manual triggers flow through normal event pipeline)
- [ ] No unintended coupling (control channel is the only link between web and orchestrator)
- [ ] No duplicated functionality (extends scheduler and orchestrator, doesn't recreate)

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | "Pause Chaos" clicked | Scheduler stops firing; button changes to "Resume" |
| AC-2 | "Resume" clicked | Scheduler resumes; button changes to "Pause" |
| AC-3 | Rate slider set to 0.5 | Scheduler uses 0.5 on next tick |
| AC-4 | Rate slider set to 0.0 | Equivalent to pause (no chaos events) |
| AC-5 | "Stop" clicked | Run stops gracefully, status = COMPLETED/FAILED |
| AC-6 | "New Seed" submitted with 12345 | Run stops, restarts with seed 12345 |
| AC-7 | Trigger dropdown changed to "RouteBurst" | Parameter form shows count + family inputs |
| AC-8 | Trigger dropdown changed to "TCPDisconnect" | No parameter inputs shown |
| AC-9 | "RouteBurst" triggered on peer 3 with count=500 | Peer 3 announces 500 routes, event in feed |
| AC-10 | Invalid params submitted (ClockDrift > hold time) | Error fragment returned, no action |
| AC-11 | Peers 0, 3, 7 selected via checkboxes, trigger executed | Action sent to exactly those 3 peers |
| AC-12 | Trigger with --event-log | EventChaosExecuted in NDJSON with chaos-params |
| AC-13 | Pause/resume with --event-log | "control" record in NDJSON (informational) |
| AC-14 | Property badge shows FAIL, clicked | Violation details expand inline |
| AC-15 | Control channel full (16 commands queued) | POST returns HTTP 503 "busy" |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| TestSchedulerPause | `chaos/scheduler_test.go` | Paused Tick() returns empty | |
| TestSchedulerResume | `chaos/scheduler_test.go` | Resumed Tick() generates actions again | |
| TestSchedulerSetRate | `chaos/scheduler_test.go` | New rate used on next Tick() | |
| TestSchedulerIsPaused | `chaos/scheduler_test.go` | IsPaused reflects current state | |
| TestControlChannelPause | `web/control_test.go` | PauseChaos command sent and received | |
| TestControlChannelTrigger | `web/control_test.go` | TriggerChaos command with peer + action + params | |
| TestControlChannelFull | `web/control_test.go` | Non-blocking send returns busy when full | |
| TestHandlerPauseChaos | `web/handlers_test.go` | POST /control/chaos/pause sends command | |
| TestHandlerTriggerChaos | `web/handlers_test.go` | POST /control/chaos/trigger validates and sends | |
| TestHandlerSetRate | `web/handlers_test.go` | POST /control/chaos/rate validates 0.0-1.0 | |
| TestHandlerTriggerParams | `web/handlers_test.go` | GET /control/trigger-params?action=X returns form | |
| TestHandlerTriggerValidation | `web/handlers_test.go` | Invalid params rejected with error fragment | |
| TestJSONLogControlRecord | `report/jsonlog_test.go` | Control actions logged as "control" record type | |

### Boundary Tests

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Chaos rate (slider) | 0.0-1.0 | 1.0 | N/A (0.0 valid) | 1.1 (clamp) |
| Peer index (trigger) | 0 to N-1 | N-1 | -1 | N |
| Control channel capacity | 16 | 16 queued | N/A | 17th = 503 |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| test-web-control-pause | `test/chaos/web-control.ci` | POST pause, verify scheduler stops | |
| test-web-trigger-replay | `test/chaos/web-trigger-replay.ci` | Manual trigger logged and replayable | |

## Files to Modify

- `cmd/ze-bgp-chaos/chaos/scheduler.go` - Add Pause(), Resume(), SetRate(), IsPaused()
- `cmd/ze-bgp-chaos/orchestrator.go` - Add control channel, select loop
- `cmd/ze-bgp-chaos/main.go` - Create control channel, pass to web + orchestrator
- `cmd/ze-bgp-chaos/report/jsonlog.go` - Add "control" record type
- `cmd/ze-bgp-chaos/web/handlers.go` - Add POST control handlers, trigger-params handler
- `cmd/ze-bgp-chaos/web/dashboard.go` - Accept control channel, expose to handlers
- `cmd/ze-bgp-chaos/web/sse.go` - Property status change SSE event

## Files to Create

- `cmd/ze-bgp-chaos/web/control.go` - Control command types, channel wrapper
- `cmd/ze-bgp-chaos/web/control_test.go` - Control channel tests
- `cmd/ze-bgp-chaos/web/templates/controls.html` - Control panel (pause, slider, trigger)
- `cmd/ze-bgp-chaos/web/templates/trigger_params.html` - Per-action parameter forms (16 variants)
- `cmd/ze-bgp-chaos/web/templates/property_detail.html` - Expandable violation details
- `test/chaos/web-control.ci` - Functional test
- `test/chaos/web-trigger-replay.ci` - Functional test

## Implementation Steps

1. **Extend scheduler (TDD)** - Pause/Resume/SetRate/IsPaused with mutex
2. **Define control commands (TDD)** - Command types, channel wrapper, non-blocking send
3. **Modify orchestrator event loop** - select on event + control channels
4. **Control panel template** - Pause button, rate slider, stop button, seed input
5. **POST control handlers (TDD)** - /control/chaos/pause, resume, rate
6. **Trigger param forms** - Template per action type, loaded via HTMX
7. **POST trigger handler (TDD)** - Validate params, send TriggerChaos command
8. **Multi-select peers** - Checkbox column in table, target selection in trigger form
9. **NDJSON control records** - Log pause/resume/rate as "control" type
10. **Property detail expansion** - Click badge to expand violations
11. **Functional tests**

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Scheduler pause/resume | | | |
| Scheduler setRate | | | |
| Control channel | | | |
| Control panel UI | | | |
| Parameterized trigger form | | | |
| Multi-select peers | | | |
| Manual triggers in NDJSON | | | |
| Control actions in NDJSON | | | |
| Property violation details | | | |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | | | |
| AC-2 | | | |
| AC-3 | | | |
| AC-4 | | | |
| AC-5 | | | |
| AC-6 | | | |
| AC-7 | | | |
| AC-8 | | | |
| AC-9 | | | |
| AC-10 | | | |
| AC-11 | | | |
| AC-12 | | | |
| AC-13 | | | |
| AC-14 | | | |
| AC-15 | | | |

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:**
- **Skipped:**

## Checklist

### Goal Gates
- [ ] AC-1..AC-15 demonstrated
- [ ] `make ze-unit-test` passes
- [ ] `make ze-functional-test` passes
- [ ] `make ze-lint` passes
- [ ] Controls work in browser during live chaos run

### TDD
- [ ] Tests written
- [ ] Tests FAIL
- [ ] Implementation complete
- [ ] Tests PASS
