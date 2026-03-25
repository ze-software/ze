# 424 -- Forward Backpressure

## Context

The forward pool dispatched UPDATEs to per-destination-peer workers via blocking `Dispatch()`. A single slow peer blocked forwarding to all other peers. TCP writes had no deadline, so a stuck connection held the worker goroutine indefinitely. bgp-rs used a single `forwardLoop` goroutine, so one blocked forward RPC stalled the entire pipeline. The spec defined 6 phases to eliminate head-of-line blocking.

## Decisions

- Chose **TryDispatch + DispatchOverflow fallback** over replacing Dispatch. Non-blocking attempt first, overflow buffer on failure. Original blocking `Dispatch` retained for callers that need guaranteed delivery.
- Chose **unbounded overflow with token-based soft bound** over hard dropping (spec AC-5 said "drop oldest"). Routes are critical data — dropping causes silent routing inconsistency with no automatic recovery. Memory growth from a slow peer is preferable to missing routes.
- Chose **hysteresis congestion detection** (mark at full, clear at 25% capacity) over threshold-only. Prevents callback storms when channel oscillates near capacity.
- Chose **N forward senders** (default 4, `ze.rs.fwd.senders`) in bgp-rs over a single `forwardLoop`. Shared `forwardCh` channel with WaitGroup shutdown.
- All 6 phases were implemented through `spec-forward-congestion` Phases 1-2, which absorbed this spec's scope and extended it with industry research, metrics, read throttling, and a 4-layer congestion design.

## Consequences

- Forward path is fully non-blocking: TryDispatch in ForwardUpdate, overflow buffer as fallback, write deadline on TCP, multiple RS senders.
- Congestion events (`peer-congested`/`peer-resumed`) are delivered to subscribed plugins via `onPeerCongestionChange`.
- Read throttle (`ReadThrottle`) was added beyond original scope — proportional sleep based on pool fill and per-source overflow ratio.
- Prometheus metrics (overflow depth, pool ratio, source ratio) make congestion observable.
- Remaining congestion work (pool multiplexer, weighted sizing, buffer denial, GR-aware teardown) continues in `spec-forward-congestion` Phases 3-5.

## Gotchas

- AC-5 (drop oldest) was deliberately changed to unbounded fallback. The industry consensus (BIRD, GoBGP, RustBGPd) is unanimous: never drop routes silently.
- Forward barrier (`learned/414`) was a separate spec but tightly coupled — sentinel `fwdItem` dispatched through the same pool for deterministic drain.
- `drainOverflow` had a pre-existing race (send on closing channel) exposed during deep-review. Fixed with two-select pattern checking `stopCh` before each send.

## Files

- `internal/component/bgp/reactor/forward_pool.go` — TryDispatch, DispatchOverflow, fwdOverflowPool, congestion callbacks, write deadline
- `internal/component/bgp/reactor/forward_pool_throttle.go` — ReadThrottle (beyond original scope)
- `internal/component/bgp/reactor/reactor_api_forward.go` — TryDispatch+overflow in ForwardUpdate, source stats
- `internal/component/bgp/reactor/reactor_metrics.go` — overflow/pool/source metrics
- `internal/component/bgp/reactor/reactor.go` — env var registration, congestion callback wiring
- `internal/component/bgp/plugins/rs/server.go` — multiple forward senders
- `internal/component/bgp/server/events.go` — onPeerCongestionChange delivery
