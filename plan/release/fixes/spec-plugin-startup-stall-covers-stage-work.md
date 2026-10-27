# Spec: plugin-startup-stall-covers-stage-work

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | plugin |
| Priority | P1 |
| Release reason | The configured startup stall bound does not cover all work in the handshake it promises to bound. |
| RFC gaps | - |
| Depends | - |
| Phase | Research recorded; runtime reproduction and implementation design owed |
| Handoff | - |
| Updated | 2026-10-27 |

## Task

Enforce the existing dependency-tier no-progress timeout while plugins perform
stage work, not only while a plugin waits at a barrier. Preserve progress-based
renewal, configuration precedence, startup rollback and normal completion.

This is a separate lifecycle defect found while investigating the base DelayOpen
TIME30 in `plan/post-release/features/spec-bgp-fsm-delayopen-and-automatic-start.md`.
It is not established as that timeout's cause. Repairing it does not clear that
failure without evidence from the failed path.

## Required Reading

- [ ] `docs/architecture/api/process-protocol.md`, startup timeout.
  → Constraint: the bound covers a dependency tier's period without progress;
  duplicate completions do not renew it.
- [ ] `docs/guide/plugins.md`, startup timeout.
  → Constraint: a wedged tier must stop within one configured timeout after
  its last progress, rather than wait for another plugin to reach a barrier.
- [ ] `internal/component/plugin/yang/ze-plugin-conf.yang`, external `timeout`.
  → Constraint: preserve the existing leaf, duration validation and environment
  fallback. The description explicitly includes reaching the first stage.

## Current Behavior

| Producer | Observed source behavior |
|----------|--------------------------|
| `server/startup.go`, `stageTransition` | Calls `WaitForStageProgress` after stage work completes. |
| `server/startup.go`, `runPluginPhase` | Starts tier workers and waits for all of them; it starts no independent stall watcher. |
| `server/startup.go`, `handleProcessStartupRPC` | Supplies the server lifetime context to `runStartupHandshake`. |
| `server/startup_driver.go`, `runStartupHandshake` | Reads plugin requests and awaits configure/registry delivery under that context. |
| `internal/component/plugin/startup_coordinator.go`, `NewStartupCoordinator` | Initializes progress state and channels; it starts no independent timeout worker. |
| `cmd/ze/hub/main.go`, startup-completion wait | Has a separate 30-second deadline, then stops the server and engine. Normal daemon startup is not wholly unbounded. |

The server paths above are under `internal/component/plugin/`. In a singleton
tier, each barrier can complete immediately. If its plugin then withholds a
request or callback response, no barrier waiter enforces the configured stall
window. The same condition can arise when every member of a larger tier is
inside unfinished work.

This finding is source-demonstrated. No runtime fault injection has reproduced
it yet, and no existing native selector was found that withholds those messages.
A socket-write deadline is not a deadline for receiving a request or response.

## Data Flow

External plugin configuration selects the existing timeout. The server groups
plugins into dependency tiers, creates the coordinator, starts each handshake
worker and waits for completion. Stage completions update the coordinator's
progress. The repair must make a stalled tier terminate through the existing
startup-failure and rollback path even when no worker reaches a barrier.

## Acceptance Criteria

| ID | Condition | Required result |
|----|-----------|-----------------|
| AC-1 | A connected singleton withholds its first startup request, with a configured stall window shorter than the remaining outer startup budget | The configured tier stall window expires and startup reports the responsible stage; it does not wait for the daemon's outer 30-second fallback. |
| AC-2 | A singleton withholds a configure or registry response, or a later plugin-initiated request | The same stall policy applies during stage work. |
| AC-3 | Several tier members are all inside unfinished stage work | Lack of a barrier waiter does not disable stall enforcement. |
| AC-4 | Distinct stage completions continue within the configured window | Progress renews the window. Duplicate completion does not. Healthy work is not given a flat total-stage deadline. |
| AC-5 | Per-plugin timeout, environment fallback or default is selected | Preserve existing precedence and the effective policy when members have different values; determine it from current callers before choosing the watcher design. |
| AC-6 | Startup completes, is canceled, or fails | No timer can abort an already completed tier; timeout work is joined and the existing process/registry rollback finishes. |
| AC-7 | The new functional fault is removed | The same native daemon/plugin path completes normally, without changing assertions or extending deadlines. |

## Reproduction and Design Work

1. Add a registered native fixture that connects through the real SDK/IPC path
   and deliberately withholds a selected startup operation. Do not substitute
   an isolated coordinator test for the missing server integration.
2. Run the singleton case against the existing producer and retain the red
   result. Distinguish the configured stall failure from outer daemon or runner
   termination.
3. Trace all timeout-policy callers, including mixed-value tiers and the final
   ready acknowledgement, before selecting the lifetime of the enforcement
   worker. Reuse the coordinator's progress accounting rather than copy it.
4. Define cancellation, completion and rollback ordering; review that design
   independently before implementation.
5. Keep server regression tests for the missing bound and progress/completion
   races. Exercise the registered native fixture red, repaired green, and with
   the repair disabled. Run only the affected packages and named cases.

## Risks and Boundaries

| Risk | Required control |
|------|------------------|
| A flat timeout replaces progress renewal | Prove continued legitimate progress survives beyond one stall window. |
| The last ready acknowledgement races timeout cleanup | Cover both successful commit and failed-startup rollback at that boundary. |
| A second timeout policy drifts from barrier behavior | Reuse existing configuration and progress semantics; add no new option. |
| The outer daemon deadline is mistaken for the configured bound | Keep the two verdicts distinct in the native proof. |
| This repair is claimed to fix historical DelayOpen TIME30 | Require evidence identifying that failure's blocked path separately. |

No BGP wire behavior, SDK message shape, timeout default, configuration syntax
or new privileged operation belongs in this fix. The separate daemon startup
fallback remains distinct from the tier stall policy.

## Documentation Work

Update the startup timeout explanation in `docs/architecture/api/process-protocol.md`
and `docs/guide/plugins.md` with the implementation: explain how enforcement
covers unfinished work and distinguish the daemon-wide fallback. Preserve the
existing operator guarantee rather than weaken it to match the defect.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `plugin-startup-barrier-deadlock.md` | 2026-10-27 | Configured tier stall timeout is inactive during unfinished stage work |
