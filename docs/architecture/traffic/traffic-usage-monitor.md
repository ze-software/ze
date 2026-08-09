# Traffic Stat: the Aggregation Layer

Ze's live traffic data (per-interface rates, per-IP talker statistics, per-port
breakdowns) was spread across collectors: `trafficusage`, `flowexport` and
`iface/rate`. The only aggregation of it lived inside `ddos/detect`, so a viewer
could not see traffic usage without starting the detection logic.

"Aggregate and time-window traffic data" is a layer between collection and
action. `internal/component/trafficstat` is that layer. The later split of that
layer into math primitives, aggregation and neutral features is
[traffic analysis layers](traffic-analysis-layers.md).

## Placement

<!-- source: internal/component/trafficstat/service.go -- Service, Snapshot, Attach, Detach -->
<!-- source: internal/core/portname/portname.go -- Lookup, Info -->

- The service is a COMPONENT, not a core leaf, because it imports
  `internal/component/iface` for per-interface rates. A core package cannot
  import a component, and `make ze-tier-check` enforces that.
- The port-to-service-name table is a CORE LEAF (`internal/core/portname`),
  because the ddos classifier needs the same table and the table is pure data
  with no component import.
- The table is GENERATED from `/etc/services`, 9,839 keyed entries, proto-aware.
  A hand-curated list of about 150 entries cannot disambiguate by protocol, and
  port 512 is `exec` on TCP and `comsat` on UDP.
- `portname.Lookup` returns `Info{Name, Amplification}` keyed on (port, proto)
  with a port-only fallback, so a future amplification detector reuses the
  overlay.

## The substrate is the observation feed

<!-- source: internal/core/observation/observation.go -- Feed, Publish, Subscribe -->

`observation.Feed` was built for this. Aggregating on the service side is
reusable across consumers, where re-collecting with libpcap or composing in the
CLI is not.

Interface rates are read from `iface.ListRates` rather than from a
`KindInterface` observation. `KindInterface` is defined and not yet published,
and waiting for it would have blocked this layer on feed coordination.

## Lazy lifecycle by consumer refcount

<!-- source: internal/component/trafficstat/service.go -- Attach, Detach, attachIDs -->

`Attach` and `Detach` refcount consumers, so the service idles when nothing
consumes it. A streaming handler's `ctx.Done()` drives the detach naturally.

`Detach` tracks consumer IDs in an `attachIDs` map, so a duplicate call is a
no-op. A plain counter double-decremented. `UnsubscribeRates` has the same
protection: it decrements only when the subscriber was actually removed, or a
double unsubscribe drives the count negative.

## The monitor provider registry

<!-- source: internal/component/plugin/server/handler.go -- RegisterMonitorProvider -->

The CLI model already carried per-feature fields for the dashboard, traceroute
and ping. Adding `Model.trafficMonitor` and `SetTrafficFactory` is the
instinctive next step and it is the anti-pattern: it hardcodes what registration
should carry. `RegisterMonitorProvider` is the generic registry, and it is the
template for every future full-screen TUI view. The existing hardcoded views
should migrate to it.

## Wire method naming

The wire methods are `ze-show:traffic-stat` and `ze-monitor:traffic-stat`.
`ze-show:traffic` was already taken by the QoS traffic-control component, and the
collision was found at compile time.

## Consequences

<!-- source: internal/plugins/ddos/detect/register.go -- SubscribeRates, SubscribeCollectNotify fallback -->

- The CLI and the analytics consumers (the ddos classifier, anomaly scoring) are
  peers reading one `Snapshot` substrate. A new consumer is an `Attach` and a
  read.
- `ddos/detect` has two rate sources: `trafficstat.SubscribeRates` when it is
  available and `iface.SubscribeCollectNotify` as a fallback. The fallback can be
  removed once trafficstat is always present.
- Per-source-IP data needs either `traffic-usage track-ip` (IPv4, cumulative) or
  `flow-export` conntrack (v4 and v6, delta, full 5-tuple). Without either, the
  view degrades to interface rates.
- The view is statistical. The feed drops on a full subscriber buffer, so it is
  not suitable for lossless accounting.

## Traps

<!-- source: internal/component/trafficstat/service.go -- ingest -->

- **Not every feed value is cumulative.** `FeatureFlowBytes` observations are
  per-publish DELTAS. The first implementation diffed them like counters, and
  flow rates grew without bound. `ingest()` branches on `Feature`, sums deltas
  per window, and resets on snapshot.
- **A flow observation feeds two maps.** It contributes to `sources` by
  `Flow.Src` and to `dests` by `Flow.Dst`. Folding it into `sources` alone left
  `TopDestIPs` empty.
- **Metrics registration across an import cycle.** `BindMetrics` was repeatedly
  deleted as dead code, because the `iface` and `trafficstat` cycle prevented
  direct registration. It is wired through `registry.GetMetricsRegistry()`, a
  neutral package, inside `EnsureGlobal()`. That indirection is the pattern for
  any component that needs a metrics registry it cannot import.
