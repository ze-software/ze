# L2TP and RADIUS Prometheus metrics

Session metrics for L2TP and server metrics for RADIUS. A 30-second poller reads
the kernel counters. Per-session series carry the full label set and are removed
asynchronously after teardown.

<!-- source: internal/component/l2tp/metrics.go -- bindL2TPMetrics, l2tpStatsPoller, observeCQMBucket, deleteSessionSeries -->
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
100-second bucket, the minimum, average and maximum round-trip times go into the
histogram, and the loss ratio and bucket state gauges are updated. The key is
the username.

<!-- source: internal/component/l2tp/cqm.go -- CQMBucket, BucketInterval -->

**Polling reads a snapshot, it does not follow lifecycle events.** The reactor
snapshot plus the interface statistics call decouples collection from session
events, so there is no race to reason about.

## Traps this code exists to avoid

**Kernel counters are 32-bit and they wrap.** The poller computes deltas between
polls and handles the wrap. A raw read reports a large negative jump.

<!-- source: internal/component/l2tp/metrics.go -- addCounterDelta -->

**A registered metric that nothing sets reads as zero, not as absent.** The echo
loss ratio was registered and never written. Zero is a plausible value for a
healthy link, so nothing looked wrong.

**A metric with a label needs the label threaded to every call site.** The
RADIUS metrics were first called with an empty server label. The server address
has to reach each call site.
