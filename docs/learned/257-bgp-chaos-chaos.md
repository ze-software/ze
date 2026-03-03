# 257 — BGP Chaos Events

## Objective

Phase 3 of ze-bgp-chaos: inject chaos events (session resets, route flaps, capability changes) during BGP sessions.

## Decisions

- `chaos/` package is pure scheduling (10 action types + weighted selection); execution lives in `peer/simulator.go`, not a separate `executor.go`.
- `executeChaos()` called from the KEEPALIVE select loop in the simulator — natural integration point.
- `net.DialTimeout` replaced with `net.Dialer.DialContext` (noctx linter violation).

## Patterns

- Chaos separation: `chaos/` defines what can happen (action types, weights); `simulator.go` defines how it happens (execution in event loop).
- Weighted selection: each chaos action has a weight (total=100); selection picks randomly proportional to weight.
- Reconnect lifecycle: `runPeerLoop()` wraps simulator with backoff-based reconnect; chaos disconnect sets `Disconnected=true` to trigger it.
- Non-blocking chaos dispatch: actions sent via buffered channels; skip if peer is busy.

## Gotchas

- `net.DialTimeout` triggers noctx lint; use `net.Dialer{Timeout: t}.DialContext(ctx, ...)` instead.
- Separate executor.go was abandoned — execution fits naturally in the simulator's existing select loop.

## Files

- `cmd/ze-bgp-chaos/chaos/scheduler.go`, `action.go` — seed-based scheduler, 10 action types with weights
- `cmd/ze-bgp-chaos/peer/simulator.go` — `executeChaos()` in KEEPALIVE select loop; reconnect storm, connection collision
- `cmd/ze-bgp-chaos/peer/sender.go` — `BuildWithdrawal()`, `BuildMalformedUpdate()` added
- `cmd/ze-bgp-chaos/main.go` — `--ze-pid` flag, `runPeerLoop()` reconnect lifecycle, `runScheduler()`
- `cmd/ze-bgp-chaos/report/summary.go` — chaos stats (ChaosEvents, Reconnections, Withdrawn)
