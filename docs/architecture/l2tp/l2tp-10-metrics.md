# L2TP and RADIUS Prometheus metrics

Session metrics for L2TP and server metrics for RADIUS. A 30-second poller reads
the kernel counters. Per-session series carry the full label set and are removed
asynchronously after teardown.

<!-- source: internal/component/l2tp/metrics.go -- bindL2TPMetrics, registerLateL2TPMetrics, l2tpStatsPoller, deleteSessionSeries -->
<!-- source: internal/component/l2tp/subsystem.go -- Subsystem.Start, Subsystem.onLateMetricsRegistry, Subsystem.startStatsPollerLocked -->
<!-- source: internal/component/l2tp/observer_metrics.go -- retainCQMSeries, observeCQMBucket -->
<!-- source: internal/component/l2tp/plugins/authradius/metrics.go -- RADIUS server metrics -->

## Decisions

**Names follow the `ze_<component>_*` convention.** The metric families are
`ze_l2tp_*` and `ze_radius_*`. The legacy `nas_*` naming was dropped.

**Per-session labels are `sid`, `ifname`, `username`, `ip` and `caller_id`.**
The calling station id is always empty, because the session struct does not
track it. The label is present so that adding the field later does not change
the series shape.

**Series cleanup is asynchronous.** A per-session series is deleted at the next
poll tick, up to 30 seconds after the interface disappears from the snapshot,
rather than synchronously on session down. The observer does not hold the label
set that immediate deletion would need.

**The bucket observation feeds the histogram.** When the observer finalizes a
100-second bucket with echoes, its average round-trip time is one histogram
observation. The loss ratio and bucket state gauges are updated from that
finalized bucket. The key remains the username.

Expected and received echo counts retain their complete 64-bit values through
the loss calculation. A short positive sampling interval does not narrow the
expected count before computing the ratio.

**Quality series belong to their original physical registration.** When the
metrics registry already exists at `Start`, startup binds metrics before
constructing the observer and starts the stats poller immediately. When the
registry is created after `Start` (for example on an L2TP-only daemon without
BGP), `Subsystem.Start` defers binding through `registry.InjectPluginMetrics`
(after releasing `Subsystem.mu` so synchronous callbacks cannot deadlock). When
the registry arrives, `onLateMetricsRegistry` binds `ze_l2tp_*` and subscriber
metrics, attaches the binding to the live observer (retaining series leases for
any already-tracked logins without replacing an existing binding), and starts
the stats poller; `Stop` and `unwindLocked` detach the pending handle so a
stopped subsystem never starts a late poller. Each bounded login slot retains
the three original vectors before publishing measured values. A failed or
partial retention releases only its new interests and preserves useful old
history. Eviction and final Stop release those exact leases; the last holder
removes the original series. An older observer cannot delete a current holder's
series or redirect measurements into a replacement registry. Retention alone
publishes no sample.

<!-- source: internal/component/l2tp/observer_metrics.go -- cqmSeriesLease, retainCQMSeries, observeCQMBucket -->
<!-- source: internal/component/l2tp/observer.go -- attachMetrics, touchLogin, evictLRU, Stop -->

<!-- source: internal/component/l2tp/cqm.go -- CQMBucket, BucketInterval -->

**Polling reads a snapshot, it does not follow lifecycle events.** The reactor
snapshot plus the interface statistics call decouples collection from session
events, so there is no race to reason about.

## Traps this code exists to avoid

**Interface counters can decrease.** The poller compares unsigned readings.
When a reading is lower than its predecessor, it adds the current reading as
a new baseline instead of producing an unsigned underflow.

<!-- source: internal/component/l2tp/metrics.go -- addCounterDelta -->

**A registered metric that nothing sets reads as zero, not as absent.** The echo
loss ratio was registered and never written. Zero is a plausible value for a
healthy link, so nothing looked wrong.

**A metric with a label needs the label threaded to every call site.** The
RADIUS metrics were first called with an empty server label. The server address
has to reach each call site.
