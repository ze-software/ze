# Spec: redistribution-producer-exit-withdrawal

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | plugin |
| Priority | P1 |
| Release reason | An unexpected route-producer exit can leave its redistributed routes installed at consumers after the producer is gone. |
| RFC gaps | - |
| Depends | - |
| Phase | Source-confirmed defect capture; lifecycle design and runtime reproduction owed |
| Handoff | - |
| Updated | 2026-11-13 |

## Task

Withdraw the routes a redistribution consumer holds for a producer generation
when that generation exits unexpectedly. Preserve routes from other sources
and any successor generation. This is a release correctness fix discovered
during deployment-user-journeys validation; recording it
does not satisfy that spec's startup, route, traffic or graceful-stop proof.

This skeleton captures the problem. It approves no transport, attachment RPC,
lease or new lifecycle state. Design must respect
`plan/release/fixes/spec-single-owner-lifecycle-1-plugin-process.md`: the process
pointer identifies the generation and the existing supervisor and run-once
cleanup own its end. No new owner object, generation counter or source gate.
Coordinate restart ordering with
`plan/release/fixes/spec-plugin-restart-bounded-and-configuration-true.md`.

## Current Behavior

Source read on 2026-11-13; no crash reproduction or test run in this capture.

`internal/component/plugin/server/dispatch.go` `cleanupProcess` runs after
dispatch drains. It clears commands and subscriptions, calls
`dispatch_route.go` `withdrawPluginRoutes`, and invokes registered generation
and name cleanup hooks. The route cleanup removes only paths recorded through
the route-install RPC in `installedByPlugin`.

`internal/plugins/redistribute/redistribute.go` `handleBatch` records accepted
Adds in `consumerHeld`. Explicit Removes release those entries. `held.go`
`forgetConsumer` handles a consumer registering again; it does not retire a
producer. A replay that omits a formerly announced prefix does not withdraw it.
The current cleanup-hook registrations do not connect process loss to this
held set. Protocol identity alone does not identify an authenticated process
generation; plugin names cannot supply that association.

`internal/plugins/connected/lifecycle.go` `retire`, wired through SDK `OnBye`
in `eventbus.go`, emits explicit removals during graceful retirement. An
unexpected exit cannot execute that callback. Its deployment repair therefore
does not establish producer-crash cleanup.

## Existing Mechanisms

| Mechanism | Where | What it does today | Design work owed |
|-----------|-------|--------------------|------------------|
| Run-once process cleanup | `internal/component/plugin/server/dispatch.go` `cleanupProcess` | Releases one process generation after admitted dispatch drains | Reuse its existing ownership; determine the smallest registered connection to redistribution |
| Generation cleanup registration | `internal/component/plugin/server/register_engine_op.go` `RegisterProcessGenerationCleanup` | Calls registered owners with the exact process pointer | Assess whether it can carry the required association and ordering without new lifecycle ownership |
| RPC route cleanup | `internal/component/plugin/server/dispatch_route.go` `withdrawPluginRoutes` | Removes paths tracked in `installedByPlugin` | Preserve it; it does not release redistribution-held entries |
| Held-route reconciliation | `internal/plugins/redistribute/held.go` `heldRoutes` | Tracks routes by consumer, source, family and prefix; releases explicit removals and rejected routes | Extend existing route ownership after generation association and replacement ordering are settled |
| Connected graceful retirement | `internal/plugins/connected/lifecycle.go` `retire` | Publishes removals before a successful Bye | Preserve; cannot handle unexpected process loss |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A running producer publishes a useful Add accepted and installed by a consumer, then exits without Bye or Remove | The consumer withdraws that producer's held route through the real loss path |
| AC-2 | The same prefix is held from another source when one producer exits | Cleanup withdraws only the exited producer's contribution; the other source remains useful |
| AC-3 | An old generation's cleanup overlaps a replacement publishing routes | Old cleanup cannot remove successor routes or accept a late old-generation publication as current |
| AC-4 | The producer runs over DirectBridge and over the socket transport | Both paths prove positive installation before forced loss and exact owned withdrawal afterward |
| AC-5 | A real daemon loses an active redistribution producer after routes reach its destination | A functional regression proves useful routing before loss, withdrawal afterward, and unrelated-source and successor survival without weakening existing traffic assertions |

## Design Work Owed

1. Read process-exit, dispatch-drain and replacement-start ordering at the
   producers. Establish an authenticated process-to-protocol association using
   existing registration seams; no candidate is selected here.
2. Specify which held routes belong to the ending generation, including a
   successor announcing the same source and prefix before late cleanup.
3. Reproduce forced loss through actual runners on both transports before
   implementation. Obtain daemon proof of downstream withdrawal and survivors.
4. Define the operator diagnosis for retained routes or failed withdrawal and
   name its doctor check. Complete integration and documentation checklists
   before this skeleton advances to design.

The externally hosted orchestrator's missing EventBus is separately owned by
`plan/release/fixes/spec-plugin-event-bus-across-the-process-boundary.md` AC-2.
It is not repaired or closed by this record.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `retained-entry-still-answers-lookups` | 2026-11-13 | redistribution `consumerHeld` after unexpected producer exit |
