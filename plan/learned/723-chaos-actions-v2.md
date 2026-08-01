# 723 -- chaos-actions-v2

## Context

ze-chaos had 9 built-in chaos action types (TCPDisconnect, NotificationCease, etc.) but all were non-parameterized one-shot actions. Richer testing scenarios required actions with configurable parameters: timing drift, route volume control, backpressure simulation. The design doc had specified 6 new parameterized actions but none had been implemented.

## Decisions

- Added Params map[string]string to ChaosAction over a typed union, because params flow through JSON serialization (NDJSON log, replay) and string maps round-trip cleanly without custom marshaling.
- Made v2 actions opt-in via --chaos-actions flag and EnabledActions in SchedulerConfig, over always-on, to preserve backwards compatibility for existing chaos runs.
- Per-instance weight table in Scheduler over global var, because EnabledActions varies per run and the old global init() could not support that.
- Parameter validation uses clamping (cap at max) over rejection (return error) for count/cycles/duration fields, because chaos testing should be resilient and partial execution is better than no execution. ClockDrift drift is the exception: drift >= holdTime is rejected because it would definitely cause a hold-timer expiry (a different action type).
- SlowPeer uses TCP write buffer reduction over message-level delay, because actual TCP backpressure tests Ze's congestion handling more realistically than application-level sleep.
- ZeroWindow uses SetReadBuffer(1) over SetReadBuffer(0), because 0 is often interpreted as "use default" by the OS.

## Consequences

- The web dashboard now shows 15 chaos action buttons (9 legacy + 6 v2). The ManualTrigger.Params was already collected by the web UI but was being discarded by handleManualTrigger; now it flows through to ChaosAction.Params.
- NDJSON event log has a new optional "chaos-params" field. Old logs without this field replay fine (omitempty). New logs with params replay fine too since replay just reads the field.
- Any new v2 action added in the future only needs: ActionType constant, Name constant, maps entries, parse function, executeChaos case, guard case, web control entries, v2Weights entry.

## Gotchas

- The exhaustive linter catches every switch on ActionType across the entire codebase. Adding 6 new constants required updating guard/guard.go (not obvious from the spec's file list).
- The Edit tool cannot match strings containing certain Unicode emoji characters in Go source comments. Had to use Python for the chaosActionIcon function edits.
- The spec referenced files under cmd/ze-chaos/chaos/ and cmd/ze-chaos/peer/ but the actual code lives under internal/chaos/engine/ and internal/chaos/peer/. Spec file paths were completely wrong and needed full correction before implementation.
- TestScenario_FullRouterSetup in internal/component/web was pre-existing failure unrelated to this work.

## Files

- `internal/chaos/engine/action.go` -- 6 new ActionType constants, Name constants, map entries, Params field on ChaosAction
- `internal/chaos/engine/action_params.go` -- NEW: parameter parsing and validation for all 6 v2 actions
- `internal/chaos/engine/action_params_test.go` -- NEW: parameter validation tests
- `internal/chaos/engine/action_test.go` -- updated for 15 action types
- `internal/chaos/engine/scheduler.go` -- per-instance weights, EnabledActions, v2Weights
- `internal/chaos/engine/scheduler_test.go` -- new: TestSchedulerNewActionsDisabledByDefault, TestSchedulerChaosActionsFilter
- `internal/chaos/peer/event.go` -- ChaosParams field on Event
- `internal/chaos/peer/simulator.go` -- pass action.Params through EventChaosExecuted
- `internal/chaos/peer/simulator_actions.go` -- 6 new cases in executeChaos switch
- `internal/chaos/peer/simulator_actions_v2.go` -- NEW: execute functions for 6 v2 actions
- `internal/chaos/report/jsonlog.go` -- ChaosParams field on jsonEvent
- `internal/chaos/report/jsonlog_test.go` -- new: TestJSONLogChaosParams, TestJSONLogChaosParamsBackwardsCompat
- `internal/chaos/replay/replay.go` -- ChaosParams field on logEvent
- `internal/chaos/guard/guard.go` -- 6 new cases in exhaustive switch
- `internal/chaos/web/control.go` -- 6 new entries in chaosActionTypes, icon, label, impact
- `internal/chaos/web/dashboard_test.go` -- updated button count 9->15
- `cmd/ze/ze_chaos_run.go` -- --chaos-actions flag, parseChaosActions, usage string, engine import
- `internal/chaos/orchestrator/types.go` -- EnabledActions in ChaosConfig, engine import
- `internal/chaos/orchestrator/scheduler.go` -- pass EnabledActions to NewScheduler, pass Params in handleManualTrigger
