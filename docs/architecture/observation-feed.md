# Observation Feed

<!-- source: internal/core/observation/observation.go -- Feed, Observation, Publish, Subscribe -->

The observation feed is an in-process, typed, multi-subscriber bus for traffic
observations. It replaces the single-callback `RegisterCollectNotify` hook in the
iface rate tracker with two complementary mechanisms:

1. **Iface tick fan-out** (`iface/rate.go`): `SubscribeCollectNotify` /
   `UnsubscribeCollectNotify` deliver the per-second `[]InterfaceInfo` snapshot
   to N subscribers (was 1). Existing consumers (flowexport, trafficusage,
   ddos-detect) receive every tick with the same payload as before.

2. **Observation feed** (`internal/core/observation`): a normalized `Observation`
   contract + typed `Feed` that collectors publish to and consumers subscribe to.
   Per-source-IP byte observations from trafficusage and per-flow observations
   from flowexport conntrack are published here.

## Observation Contract

<!-- source: internal/core/observation/observation.go -- Observation struct -->

```
Observation {
    Kind:    Interface | SourceIP | DestIP | Flow
    Iface:   string
    Flow:    FlowKey{Src, Dst netip.Addr; SrcPort, DstPort uint16; Proto uint8}
    Feature: RxBytes | RxPackets | FlowBytes | FlowPackets | NewFlowCount
    Value:   float64 (collector-native)
    At:      time.Time
    SrcAS:   uint32 (origin AS of Flow.Src, 0 = unknown)
}
```

`Kind` tells consumers which `Flow` fields are meaningful. `Observation` is a value
type (embedded `FlowKey`, no pointers) for zero-allocation publish.

`SrcAS` is an optional label the publisher stamps. The flowexport conntrack
producer copies the origin AS it already resolved from the BGP RIB onto each flow
observation, so a consumer reads the AS off the feed and never calls into the
producer's plugin. AS 0 is reserved (RFC 7607), so `SrcAS == 0` is the
unambiguous "not attributed" value: it is what a consumer sees when the producer
has no enricher or the RIB holds no matching prefix, and the consumer falls back
to the address or its prefix.

## Why this shape

- **A subscriber list, not a single callback.** `RegisterCollectNotify` stored
  one callback in an atomic pointer, so the last registration won and every
  earlier plugin lost its tick-driven behavior whenever two of them were
  enabled together. The copy-on-write subscriber list is the fix, and
  `RegisterCollectNotify` now wraps the new API.
- **A typed in-process feed, not the EventBus.** The EventBus appends every
  event to a 1024-entry diagnostic ring. Per-source and per-flow volume would
  thrash that ring, so this feed uses its own buffered channels and
  copy-on-write dispatch.
- **Publish never blocks, by contract.** Both publishers call it while holding
  their own lock: trafficusage from `publishLocked` under `m.mu`, and
  flowexport from `ExportFlows` under `e.mu`. A blocking publish would stall a
  collector behind a slow subscriber.
- **Conntrack flow values are deltas.** The cumulative reading applies to the
  eBPF and rate-tracker byte counters. `FlowBytes` from conntrack is the delta
  since the last export, which follows the `NewFlowCount` precedent.
- **The iface tick payload did not change.** Existing consumers still receive
  the same `[]InterfaceInfo`, which is what made the migration a no-op for
  them.

## Delivery

<!-- source: internal/core/observation/observation.go -- Publish, subscriber.drain -->

Each subscriber owns a bounded buffered channel (cap 1024). `Publish` does a
non-blocking send; on full buffer, the observation is dropped and
`ze_observation_dropped_total` increments. Subscribers drain on their own
goroutine, so handler code never runs on the publisher's goroutine.

The subscriber set is an atomic-pointer copy-on-write snapshot: dispatch iterates
a preallocated slice with zero per-publish allocation.

## Publishers

<!-- source: internal/plugins/trafficusage/monitor.go -- publishLocked -->
<!-- source: internal/plugins/flowexport/exporter.go -- exporter.exportFlows -->

| Collector | Publish point | Observation kind |
|-----------|--------------|------------------|
| trafficusage eBPF | `monitor.go:publishLocked` | `SourceIP` / `RxBytes` |
| flowexport conntrack | `exporter.exportFlows` | `Flow` / `FlowBytes` |

## Metrics

<!-- source: internal/core/observation/observation.go -- BindMetrics -->

| Counter | Meaning |
|---------|---------|
| `ze_observation_published_total` | Total observations published |
| `ze_observation_dropped_total` | Observations dropped (full subscriber buffer) |
| `ze_observation_subscribers` | Current subscriber count |
