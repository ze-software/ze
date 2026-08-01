# 1019 -- Traffic Usage Monitor (lazy aggregation service)

## Context

Ze's live traffic data (per-interface rates, per-IP talker stats, per-port breakdowns) was scattered across collectors (`trafficusage`, `flowexport`, `iface/rate`) and the only aggregation of it lived inside `ddos/detect`, making it impossible to view traffic usage without starting the detection logic. The fix was recognizing "aggregate and time-window traffic data" as a missing architectural layer sitting between collection/dispatch and action/detection. This spec built that layer as `internal/component/trafficstat`, a lazy consumer-refcounted service consuming `observation.Feed`, and delivered CLI consumers (`show traffic-stat`, `monitor traffic-stat`) and the ddos-detect Depth-1 refactor.

## Decisions

- Placed the aggregation service as a **component** (`internal/component/trafficstat`) over a core leaf, because it imports `internal/component/iface` for per-interface rates; core packages cannot import components (tier-check enforced)
- Port-to-service-name table placed as a **core leaf** (`internal/core/portname`) over an inline map, because `ddos/detect-5` classifier will need the same table; pure data with no component imports
- Generated the port table from `/etc/services` (9,839 keyed entries, proto-aware) over a hand-curated ~150 entry list, because proto-disambiguation (port 512 = exec/TCP vs comsat/UDP) required real data
- Used `observation.Feed` as the substrate over libpcap re-collection or CLI-side composition, because the feed was just built for this purpose and service-side aggregation is reusable across consumers
- Lazy lifecycle via consumer refcount (Attach/Detach) over always-on or CLI-local, because the service should idle when no consumer exists and the streaming handler's `ctx.Done()` naturally drives detach
- Depth-1 only for ddos-detect (swap rate input) over Depth-2 (move baseline too), because baseline extraction is owned by `spec-cp-survival-5-detect-6-behavioural` and three specs editing `baseline.go` risks merge hell (R-1)
- Interface rates read from `iface.ListRates` over waiting for `KindInterface` on the feed, because `KindInterface` is defined but not yet published; avoids blocking on observation-feed coordination (R-6)
- Used generic `MonitorProvider` registry over hardcoded CLI model fields, because the `cli.Model` already had per-feature fields for dashboard/traceroute/ping and repeating that pattern violates registration-over-hardcoding
- Named wire methods `ze-show:traffic-stat`/`ze-monitor:traffic-stat` over `ze-show:traffic`, because `ze-show:traffic` was already taken by the QoS traffic control component

## Consequences

- CLI and future analytics (detect-5 classifier, anomaly scoring) are now peers consuming one `Snapshot` substrate; adding a new consumer is Attach + read
- The `MonitorProvider` registry (`handler.go:RegisterMonitorProvider`) is the template for all future full-screen TUI views; existing hardcoded dashboard/traceroute/ping should migrate to it
- `ddos/detect` now has two data source paths: `trafficstat.SubscribeRates` (preferred) and `iface.SubscribeCollectNotify` (fallback); the fallback can be removed once trafficstat is always available
- Per-source-IP data requires either `traffic-usage track-ip` (IPv4, cumulative) or `flow-export` conntrack (v4+v6, delta, full 5-tuple); without either, the view degrades to interface rates only
- The view is statistical (feed drops on full subscriber buffer); not suitable for lossless accounting
- `portname.Lookup` returns `Info{Name, Amplification}` keyed on `(port, proto)` with port-only fallback; future amplification detection can reuse the overlay

## Gotchas

- Wire method collision: `ze-show:traffic` was already registered by `internal/component/traffic/cmd` (QoS traffic control). Discovered at compile time. Renamed to `ze-show:traffic-stat` throughout.
- `BindMetrics` was repeatedly deleted instead of wired. The function existed but was never called because the import cycle (`iface` <-> `trafficstat`) prevented direct registration. Solution: wire via `registry.GetMetricsRegistry()` (neutral package) inside `EnsureGlobal()`. The pattern: when a component needs a metrics registry but importing the telemetry component would create a cycle, use the neutral plugin registry as the indirection point.
- Flow delta observations (`FeatureFlowBytes`) are per-publish deltas, not cumulative counters. The first implementation assumed all feed values were cumulative and diffed them, causing flow "rates" to grow without bound. Fix: branch on `Feature` in `ingest()`, sum deltas per window, reset on snapshot.
- `TopDestIPs` was initially missing because flow observations were only folded into the `sources` map. Fix: flow observations contribute to both `sources` (by `Flow.Src`) and `dests` (by `Flow.Dst`).
- `UnsubscribeRates` initially decremented the consumer count unconditionally, so a double-unsubscribe could drive the count negative. Fix: `found` flag, only decrement if the subscriber was actually removed.
- `Detach` initially used a simple counter with no ID tracking, so a duplicate call would double-decrement. Fix: `attachIDs` map keyed by consumer ID; unknown ID is a no-op.
- Hardcoding monitor types in the CLI model struct (`Model.trafficMonitor`, `SetTrafficFactory`) was the instinctive approach, matching the existing dashboard/ping/traceroute pattern. User correctly identified this as the anti-pattern the spec's Critical Review Checklist explicitly warned about. Led to the `MonitorProvider` generic registry.

## Files

- `internal/core/portname/portname.go`, `portname_test.go`, `services_table.go` (generated)
- `internal/component/trafficstat/service.go`, `service_test.go`, `window.go`, `window_test.go`, `register.go`
- `internal/component/trafficstat/cmd/traffic.go`, `cmd/render.go`, `internal/component/trafficstat/cmd/yang/ze-traffic-stat-cmd.yang`
- `internal/plugins/ddos/detect/detector.go`, `register.go`, `detector_test.go`
- `internal/component/cli/contract/contract.go`, `model_monitor.go`, `model_keys.go`, `model_render.go`
- `internal/component/plugin/server/handler.go`
- `docs/features.md`, `docs/guide/command-reference.md`, `docs/architecture/api/commands.md`
- `test/plugin/traffic-monitor.ci`
