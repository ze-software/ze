# 787 -- In-Process Chaos and Route Dynamics

## Context

The in-process chaos runner (`ze-chaos --in-process`) could only test steady-state BGP behavior. The chaos and route dynamics schedulers existed in the external orchestrator but were not wired to in-process mode. This meant in-process tests could not verify fault tolerance, reconnection, or route churn, forcing those tests to use real TCP and wall-clock time.

## Decisions

- Feed virtual time (`vc.Now()`) to existing `Tick(time.Time, []bool)` schedulers over scheduling from scratch, because they are already clock-source agnostic.
- Use a tick channel (buffered 1, non-blocking send) to drive scheduler goroutines from the advance loop, over calling Tick inline in the loop, because schedulers may take time and should not block the advance.
- Use an inline `Dialer` interface on `SimulatorConfig` over importing `network.Dialer`, because the `peer` package has no reason to depend on `internal/core/network` for a single method.
- Use a `reconnectDialer` factory (creates pairs on demand) over pre-registering connections, because chaos disconnects are stochastic and the count is unknown in advance.
- Wrap simulators in a reconnection loop only when `ChaosRate > 0`, over always wrapping, to preserve the exact single-shot behavior of existing tests.

## Consequences

- `ze-chaos --in-process --chaos-rate 0.1 --route-rate 0.05 --duration 60s` now works, enabling fast deterministic chaos tests without real network I/O.
- The same virtual-time advancement that drives keepalive timers also drives chaos scheduling, so all timing relationships are preserved.
- `ActionConfigReload` is a no-op in-process (no Ze PID to SIGHUP), which is expected and documented in the spec.
- `ActionSlowPeer` and `ActionZeroWindow` will emit errors in-process because mock connections are `net.Pipe` (not `*net.TCPConn`). These are non-fatal and do not break the simulation.

## Gotchas

- `TestInProcessChaosReconnect` is timing-sensitive: with certain seeds, `ActionHoldTimerExpiry` fires instead of `ActionTCPDisconnect`. HoldTimerExpiry only stops keepalives; the actual disconnect happens when the hold timer expires (HoldTime seconds later). Use short HoldTime (20s) and long Duration (60s) to ensure disconnect happens within the window.
- The spec's file paths were stale (`cmd/ze-chaos/inprocess/` vs actual `internal/chaos/inprocess/`). Always verify paths with filesystem lookup.
- `BuildOpen` panics on zero-value `RouterID` (As4 on zero IP). Test fixtures must always set RouterID.

## Files

- `internal/chaos/inprocess/runner.go` -- Added RunConfig fields, scheduler wiring, reconnection loop, established state tracking
- `internal/chaos/inprocess/chaos.go` -- New: establishedState, reconnectDialer, chaosSchedulerLoop, routeSchedulerLoop
- `internal/chaos/inprocess/runner_test.go` -- New tests: ChaosEvents, RouteEvents, ChaosReconnect, NoChaosDefault
- `internal/chaos/peer/simulator.go` -- Added Dialer field to SimulatorConfig
- `internal/chaos/peer/simulator_actions.go` -- Modified storm/collision to use pluggable dialer
- `internal/chaos/peer/simulator_actions_test.go` -- New test: SimulatorDialerField
- `cmd/ze/ze_chaos_run.go` -- Wire chaos/route flags to RunConfig in --in-process branch
- `docs/guide/chaos-testing.md` -- Added in-process chaos example
