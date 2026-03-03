# 261 — BGP Chaos Properties

## Objective

Replace the monolithic validation model with composable named RFC property assertions that can be checked independently and reported by name in the exit summary.

## Decisions

- Properties are **additive** alongside the existing EventProcessor, not a replacement — dual-consumer fan-out where both the original EventProcessor and the new PropertyEngine receive every event. Avoids refactoring the working validation model.
- Properties use `ProcessEvent()` (stateful) rather than a stateless `Check(state, action)` — each property maintains its own internal state and reports violations at query time via `Violations()`.
- 5 of 10 planned properties implemented — withdrawal-propagation, keepalive-interval, family-filtering, collision-resolution, and eor-per-family were deferred because they require observing Ze's responses (e.g., detecting session teardown, KEEPALIVE intervals) which is not observable from the external chaos tool alone.
- No `pgregory.net/rapid` dependency added — stateful rapid testing was a skeleton-phase idea; the core RFC properties were the real deliverable.

## Patterns

- `AllProperties()` / `SelectProperties()` / `ListProperties()` — three helpers for the `--properties` CLI flag covering all/subset/enumerate modes.
- PropertyEngine is a thin dispatcher: register properties, fan out ProcessEvent(), collect violations via Results().

## Gotchas

- Properties that check Ze's internal behavior (FSM teardown timing, KEEPALIVE scheduling, family negotiation enforcement) cannot be verified from the outside — the chaos tool can only observe what it sends and receives, not Ze's internal state transitions. Five properties are therefore fundamentally blocked until in-process mode (Phase 9) or Ze gains structured event logging.

## Files

- `cmd/ze-bgp-chaos/validation/property.go` — Property interface, PropertyEngine, helper functions
- `cmd/ze-bgp-chaos/validation/props_route.go` — route-consistency
- `cmd/ze-bgp-chaos/validation/props_convergence.go` — convergence-deadline
- `cmd/ze-bgp-chaos/validation/props_duplicate.go` — no-duplicate-routes
- `cmd/ze-bgp-chaos/validation/props_holdtimer.go` — hold-timer-enforcement
- `cmd/ze-bgp-chaos/validation/props_ordering.go` — message-ordering
