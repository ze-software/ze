# 696 -- host-1-observability

## Context

Host hardware inventory data was available via `ze host show` but not exposed through the observability stack. Operators needed Prometheus metrics for scraping dashboards, hardware-change events for alerting, and a caching layer to avoid re-reading sysfs on every scrape. This was deferred from spec-host-0-inventory (items 1-3).

## Decisions

- **CachedDetector with lazy refresh** over background goroutines. The cache is populated on first access and refreshed when TTL expires on the next call. This avoids a long-lived goroutine and makes the component easy to test (no timers to mock). Zero TTL disables caching.
- **DiffEvent as a value type** rather than emitting directly to the report bus. DiffInventory returns `[]DiffEvent` which the caller translates into report bus issues. This keeps the diff engine pure (no report bus dependency) and testable without mocking the bus.
- **Three change categories: carrier-change, ecc-error, throttle.** These are the hardware events that matter for network OS operations. Carrier flip affects routing convergence. ECC errors signal memory degradation. Throttle count increase signals thermal issues affecting forwarding performance.
- **StartRefresh goroutine on HostMetrics** for periodic collection. Although CachedDetector is lazy, the metrics system needs periodic pushes to keep gauges fresh between Prometheus scrapes.

## Consequences

- Prometheus scrapes return `ze_host_*` gauges covering memory, CPU, NIC, storage, thermal, ECC, and uptime.
- DiffInventory enables hardware-change alerting without requiring external monitoring agents.
- CachedDetector bounds sysfs reads to at most once per TTL period regardless of scrape frequency.

## Gotchas

- ECC metrics are gauges of external counters (not Prometheus counters) because the value comes from edac hardware counters that persist across process restarts. Using a Prometheus counter would lose the absolute value on ze restart.
- NIC carrier gauge uses 1.0/0.0 float convention (not bool) because Prometheus has no boolean type.
- DiffInventory returns nil (not empty slice) for first-ever snapshot to distinguish "no changes" from "no baseline."

## Files

- `internal/component/host/cached.go` -- CachedDetector with TTL and Invalidate
- `internal/component/host/cached_test.go` -- TTL, invalidate, concurrent, zero-TTL tests
- `internal/component/host/metrics.go` -- RegisterMetrics, CollectOnce, collectFrom, StartRefresh, Stop
- `internal/component/host/metrics_test.go` -- NoPanic, partial inventory, full inventory, nil inventory
- `internal/component/host/diff.go` -- DiffInventory, DiffEvent, carrier/ecc/throttle diff
- `internal/component/host/diff_test.go` -- carrier change, ecc error, throttle, no-change, nil cases
- `docs/guide/monitoring.md` -- host inventory metrics table added
