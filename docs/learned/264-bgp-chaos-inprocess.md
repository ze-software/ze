# 264 — BGP Chaos In-Process Mode

## Objective

Add an in-process execution mode where the chaos tool and Ze's reactor run in the same Go process with a virtual clock and mock network, making 30-second scenarios complete in under 2 seconds.

## Decisions

- VirtualClock (new) is separate from FakeClock (existing) — FakeClock has inert timers (never fire) suited for unit tests; VirtualClock maintains a min-heap and fires timers in deadline order when `Advance()` is called, suited for simulation.
- TCP loopback pairs (`net.Listen("tcp", "127.0.0.1:0")`) instead of `net.Pipe()` for mock connections — BGP requires simultaneous bidirectional writes (both peers send OPENs at the same time); `net.Pipe()` is single-buffered and deadlocks under this access pattern.
- `SimulatorConfig.Conn` field (optional `net.Conn`) bypasses TCP dialing when set — zero behavior change when nil; in-process mode sets it with the loopback pair's peer-end.
- `reactor.SetClock()` propagation bug fixed — the setter only set the reactor-level and recentUpdates clocks; peers created during `LoadReactorWithPlugins()` retained the real clock. Fixed by iterating `r.peers` in `SetClock()`.

## Patterns

- VirtualClock uses buffered channel sends (size 1) when firing channel timers — avoids deadlock when multiple timers fire at the same virtual instant (Advance fires all in one call).
- Same-deadline timers fire in insertion order (FIFO) — provides determinism when Go's real timer resolution would be ambiguous.
- 500ms real-time sleep after connection creation is necessary — BGP handshake runs in real goroutines even when timers are virtual; the TCP I/O exchange is real and needs actual wall-clock time to complete.

## Gotchas

- `net.Pipe()` deadlock: both BGP peers write OPENs simultaneously; pipe has no buffer so each write blocks waiting for the other side to read. Symptom: first integration test hung indefinitely.
- `reactor.SetClock()` not propagating to already-created peers: reconnect backoff timers used the real clock, causing long gaps in virtual-time tests. Symptom: `TestInProcessDisconnectReconnect/long_gap` never completed.
- `disconnected` events from simulators only fire on clean context cancellation, not on connection errors — error events cover the disconnect detection path in most scenarios.

## Files

- `internal/sim/virtualclock.go` — VirtualClock: timer min-heap, Advance, AfterFunc, NewTimer, Sleep
- `cmd/ze-bgp-chaos/inprocess/mocknet.go` — ConnPairManager, MockDialer, MockListener (TCP loopback)
- `cmd/ze-bgp-chaos/inprocess/runner.go` — In-process execution engine
- `cmd/ze-bgp-chaos/peer/simulator.go` — Added optional Conn + Clock fields to SimulatorConfig
- `internal/component/bgp/reactor/reactor.go` — SetClock now propagates to all existing peers
