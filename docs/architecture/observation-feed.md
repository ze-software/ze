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
}
```

`Kind` tells consumers which `Flow` fields are meaningful. `Observation` is a value
type (embedded `FlowKey`, no pointers) for zero-allocation publish.

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
