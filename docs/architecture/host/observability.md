# Host observability: cache, metrics, hardware-change events

Three pieces sit on top of the inventory: a cache so a scrape does not re-read
sysfs, a Prometheus gauge set, and a diff engine that names hardware changes.

<!-- source: internal/component/host/cached.go -- CachedDetector, Detect, Invalidate -->
<!-- source: internal/component/host/metrics.go -- RegisterMetrics, CollectOnce, StartRefresh, Stop -->
<!-- source: internal/component/host/diff.go -- DiffInventory, DiffEvent -->

## Decisions

- **The cache refreshes lazily, on the next call after the TTL expires.** No
  background goroutine, so there are no timers to mock in a test. A zero TTL
  disables caching.
- **The diff engine returns values, it does not publish.** `DiffInventory`
  returns events and the caller turns them into report-bus issues. The engine
  stays pure and testable with no bus.
- **Three change categories, chosen for a network OS**: carrier change, ECC
  error, and throttle. A carrier flip affects convergence, an ECC error signals
  memory degradation, and a rising throttle count signals thermal pressure on
  forwarding.
- The metrics type does run a refresh goroutine, because Prometheus gauges must
  stay fresh between scrapes even though the cache itself is lazy.

## Traps

- **ECC counters are exported as gauges, not Prometheus counters.** The value
  comes from EDAC hardware counters that survive a process restart. A Prometheus
  counter would lose the absolute value every time Ze restarts.
- **The NIC carrier gauge is 1.0 or 0.0.** Prometheus has no boolean type.
- **The first snapshot returns nil, not an empty slice.** That is what
  distinguishes "no changes" from "no baseline yet". A caller that treats them
  alike alerts on every start.

## Related

- `inventory.md` for the detection library underneath
