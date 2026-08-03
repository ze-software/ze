# 1016: Shared Traffic-Observation Feed

Spec: `plan/spec-observation-feed.md`

## Context

Three plugins (flowexport, trafficusage, ddos-detect) all registered on a
single-callback hook (`iface.RegisterCollectNotify`) for the 1 Hz interface tick.
Last registration won; with multiple plugins enabled, all but the last-registered
lost their tick-driven behavior. This was a latent defect.

## Decisions

1. **Multi-subscriber iface fan-out.** Replaced the single `collectNotifyPtr`
   atomic pointer with a copy-on-write subscriber list (`collectSubsPtr`).
   `SubscribeCollectNotify` / `UnsubscribeCollectNotify` replace the old API.
   The deprecated `RegisterCollectNotify` wraps the new API for compatibility.

2. **In-process typed Feed, not EventBus.** The EventBus appends to a 1024-entry
   diagnostic ring per event (`dispatch.go`); per-source/per-flow volume would
   thrash it. A dedicated `internal/core/observation.Feed` with buffered channels
   and atomic-pointer CoW avoids this.

3. **Observation is a value type.** `FlowKey` is embedded (not a pointer) so
   `Observation` is heap-alloc-free on the publish path. Benchmark: 0 allocs/op.

4. **Non-blocking publish with bounded channels.** Each subscriber gets a cap-1024
   buffered channel. Publisher does a non-blocking send; drops on full with a
   counter. Subscribers drain on their own goroutine.

5. **Conntrack flow values are deltas, not cumulative.** The spec's "cumulative"
   semantics apply to eBPF and rate-tracker byte counters. Conntrack `FlowBytes`
   are deltas since last export, matching the `NewFlowCount` precedent.

## Consequences

- All three tick consumers now fire on every tick when co-enabled.
- New consumers (the behavioural detector from spec-cp-survival-5-detect-6-behavioural)
  can subscribe to the observation feed and receive per-source/per-flow data from
  all collectors via a single subscription.
- The iface rate tracker payload (`[]InterfaceInfo`) is unchanged for existing
  consumers, making migration zero-regression.

## Gotchas

- `legacySubID` (the `RegisterCollectNotify` compat wrapper) must be accessed
  under `collectSubsMu` to avoid data races. Initially missed; fixed during review.
- The `publishLocked` call in trafficusage runs under `m.mu`, so `Feed.Publish`
  must be non-blocking (it is by design). Same for `ExportFlows` under `e.mu`.

## Files

None recorded.
