# 386 — Prometheus Metrics

## Objective

Add Prometheus metrics export to ze: core metrics interfaces, map-based Prometheus backend, HTTP server, YANG config schema, and per-peer reactor instrumentation.

## Decisions

- Zero-wrapping for scalar metrics: `prometheus.Counter`/`Gauge` already satisfy `metrics.Counter`/`Gauge` interfaces — no wrapper structs needed
- Map-based idempotent registry: `PrometheusRegistry` stores metrics in `map[string]` per type. `Counter(name, help)` is get-or-create — any component or plugin can register metrics dynamically by name without coordination
- Ownership split: loader (`bgpconfig`) creates the registry + HTTP server from telemetry config; reactor receives the registry via `SetMetricsRegistry` and only registers/increments metrics. Server lifecycle follows the pprof pattern (fire-and-forget)
- Per-instance `prometheus.Registry` (not global default) so tests don't interfere
- Implicit enable: setting any of `address`/`port`/`path` in config enables Prometheus without explicit `enabled true`, but explicit `enabled false` always overrides
- `Delete()` on Vec types to clean up stale peer labels when peers are removed
- `peerMsgRecv`/`peerMsgSent` aggregate all BGP message types (updates, keepalives, EOR) into a single per-peer counter

## Patterns

- Interfaces designed for compatibility with existing libraries eliminates wrapping overhead — check if the library type already satisfies your interface before writing adapters
- Setter injection pattern (`SetMetricsRegistry`) matches existing `SetClock`/`SetDialer`/`SetListenerFactory` — caller creates, reactor uses
- Port validation with boundary tests: last valid (65535), first invalid below (0), first invalid above (65536), negative (-1)
- Nil-guard pattern for optional metrics: every `Incr*` method checks `p.reactor != nil && p.reactor.rmetrics != nil` before touching Prometheus

## Gotchas

- `http.Server.Close()` is idempotent in Go stdlib despite docs not explicitly saying so — confirmed by test, but the nil guard on `Server.httpServer` is still needed for the "Close without Start" case
- Counter refactoring conflict: main branch split generic `messagesReceived`/`messagesSent` into `updatesReceived`/`keepalivesReceived`/`eorReceived` etc., making the route-level Prometheus counters dead code — NLRI-level counting belongs in the RIB plugin
- Storing a registry field "for later" that nothing reads is easy to miss — the local variable passed to init functions was sufficient
- First attempt had reactor owning registry + server + config parsing — wrong ownership. Metrics server is cross-cutting infrastructure, not BGP-subsystem-specific

## Files

- `internal/core/metrics/` — metrics.go (interfaces), prometheus.go (map-based backend), nop.go (no-op), server.go (HTTP + config extraction)
- `internal/component/bgp/config/loader.go` — creates registry + server, injects registry into reactor
- `internal/component/bgp/reactor/reactor_metrics.go` — reactor-level Prometheus metrics struct and update loop
- `internal/component/bgp/reactor/peer_stats.go` — per-peer atomic counters with Prometheus integration
- `internal/component/bgp/reactor/reactor_peers.go` — AddPeer/RemovePeer with label cleanup
- `internal/component/telemetry/schema/` — YANG module `ze-telemetry-conf`
- `test/parse/telemetry-prometheus-*.ci` — functional tests for config parsing
