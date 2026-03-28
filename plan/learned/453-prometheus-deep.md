# 453 -- Prometheus Deep Instrumentation

## Context

Ze started with 11 basic Prometheus metrics covering peer state and RIB route counts. Operators needed session lifecycle visibility (flaps, transitions, notifications), forward pool backpressure metrics, RIB churn tracking, wire layer byte/error counters, GR state, and config reload monitoring. The Histogram type was also missing from the metrics interface, preventing timing distributions. rustbgpd served as a reference implementation for what operators expect.

## Decisions

- Histogram/HistogramVec types placed in existing prometheus.go and nop.go, over creating a separate histogram.go file, because the types fit naturally alongside Counter/Gauge wrappers.
- Forward pool metrics diverged from the original plan: `overflow_drops`, `dispatch_total`, `batch_size`, `channel_utilization` replaced by `pool_used_ratio`, `overflow_items`, `overflow_ratio`, because the forward pool architecture uses bounded overflow buffers (items buffered, not dropped).
- Pool stats (intern_total, dedup_hits, slots_used) implemented as GaugeVec (polled snapshots from AllPools()) over CounterVec, because pool counters are read via atomic snapshots in a metrics loop rather than incremented directly.
- `ze_rib_eor_received_total` per peer+family dropped because EOR is already counted at reactor level as `type=eor` on `ze_peer_messages_received_total`; a per-family counter would require the RIB to subscribe to EOR events, which is a design change beyond metrics wiring.
- Plugin health metrics (status, restarts, events_delivered) split into dedicated `spec-prometheus-plugin-health.md` because they require changes to `internal/plugin/` infrastructure, not just reactor/RIB metric registration.

## Consequences

- Ze now exposes 47 Prometheus metrics (up from 11), covering session lifecycle, message types, forward pool backpressure, RIB churn, attribute pool efficiency, config operations, wire layer, GR state, and prefix limits.
- The Histogram type is available for future timing distributions (UPDATE parse time, batch write time, RPC latency) but no histogram metrics are registered yet -- hot-path overhead needs benchmarking first.
- Adding `type` label to existing `ze_peer_messages_*_total` is a breaking change for dashboards that assumed unlabeled counters.
- Plugin health metrics remain as the only unimplemented category from the original plan.

## Gotchas

- .ci test plugins that call `sys.exit(1)` on assertion failure without first dispatching `daemon shutdown` cause the test to timeout (15s) rather than report the failure. Always shut down before exit.
- `metrics list` returns metric family names. Vec metrics only appear in `metrics values` after their first label combination is used (e.g., after a session establishes).
- Notification code labels use human-readable names ("header", "open", "cease") with "other" as a catch-all, to prevent unbounded cardinality from unknown/future codes.

## Files

- `internal/core/metrics/metrics.go` -- Histogram/HistogramVec interfaces
- `internal/core/metrics/prometheus.go` -- PrometheusRegistry Histogram methods + promHistogramVec wrapper
- `internal/core/metrics/nop.go` -- nopHistogram, nopHistogramVec
- `internal/component/bgp/reactor/reactor_metrics.go` -- 30+ metric fields, initReactorMetrics, metricsUpdateLoop
- `internal/component/bgp/reactor/peer_stats.go` -- Message type labels, notification code/subcode, session lifecycle
- `internal/component/bgp/plugins/rib/rib.go` -- Route insert/withdrawal counters, pool metrics
- `internal/component/bgp/plugins/gr/gr.go` -- GR state metrics
- `test/plugin/cli-metrics-show-deep.ci` -- Functional test for metrics values
- `test/plugin/cli-metrics-list-deep.ci` -- Functional test for metrics list
- `test/plugin/metrics-session-lifecycle.ci` -- Session lifecycle metric verification
