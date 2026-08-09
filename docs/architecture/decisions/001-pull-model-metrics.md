# Decision: Defer Pull-Model Metrics Collector

| Field | Value |
|-------|-------|
| Status | deferred |
| Updated | 2026-05-24 |
| Scope | Decision record. Not an implementation spec. |
| Supersedes | `plan/decision-pull-model-metrics.md` |

## The Question

Should Ze switch from its current push-model metrics to a pull-model `prometheus.Collector` pattern?

- **Push model:** each component or plugin receives a `metrics.Registry`, registers counters and gauges, and updates metric handles from the code path that owns the state.
- **Pull model:** each component or plugin exposes a `Snapshot() *Metrics` method returning a plain Go struct. A separate adapter imports the Prometheus client library and converts snapshots to Prometheus metrics on scrape.

<!-- source: internal/core/metrics/metrics.go - Registry interface -->
<!-- source: internal/core/metrics/prometheus.go - PrometheusRegistry -->

Three sub-questions were asked before any refactor:

1. Is it worth doing now, or should it wait until the metrics surface is larger?
2. What would the cost be?
3. How would external-plugin metrics work in the pull model, since external plugins run over RPC?

## Current Ze State

`internal/core/metrics/metrics.go` defines an abstract `Registry` interface with `Counter`, `Gauge`, `CounterVec`, `GaugeVec`, `Histogram`, and `HistogramVec`. The method names and shapes deliberately match Prometheus `client_golang` types where practical, but the interface itself does not import Prometheus.

<!-- source: internal/core/metrics/metrics.go - Registry interface -->

`internal/core/metrics/prometheus.go` implements that interface with a private per-instance Prometheus registry, and `internal/core/metrics/nop.go` provides a no-op backend for tests and disabled telemetry.

<!-- source: internal/core/metrics/prometheus.go - PrometheusRegistry -->
<!-- source: internal/core/metrics/nop.go - NopRegistry -->

Direct Prometheus client imports in Ze-owned code are restricted to the Prometheus backend and chaos report metrics. OS collectors may import `prometheus/procfs`, but plugins and most components use the abstract registry rather than Prometheus metric types.

<!-- source: internal/core/metrics/prometheus.go - Prometheus backend imports -->
<!-- source: internal/chaos/report/metrics.go - chaos report Prometheus imports -->
<!-- source: internal/component/telemetry/collector/collector.go - collector Registry usage -->

`ConfigureMetrics func(reg any)` remains on `plugin.Registration`. The internal plugin runner reads the global metrics registry and calls the callback before `RunEngine`, so internal plugins can register their own metrics without the core importing them directly.

<!-- source: internal/component/plugin/registry/registry.go - Registration.ConfigureMetrics -->
<!-- source: internal/component/plugin/inprocess.go - GetInternalPluginRunner metrics callback -->

The metrics surface is now larger than when this decision was first written. Ze uses the registry model for BGP reactor metrics, BGP plugins, sysrib, FIB backends, BFD, subscriber and L2TP state, IPsec, host inventory, interface rates, OS collectors, and plugin-process health.

<!-- source: internal/component/bgp/reactor/reactor_metrics.go - reactorMetrics -->
<!-- source: internal/component/bgp/plugins/rib/rib.go - SetMetricsRegistry -->
<!-- source: internal/component/sysrib/sysrib.go - SetMetricsRegistry -->
<!-- source: internal/plugins/fib/kernel/fibkernel.go - SetMetricsRegistry -->
<!-- source: internal/component/bfd/metrics.go - bindMetricsRegistry -->
<!-- source: internal/component/l2tp/subscriber/metrics.go - BindMetrics -->
<!-- source: internal/component/ike/engine/metrics.go - RegisterMetrics -->
<!-- source: internal/component/host/metrics.go - RegisterMetrics -->
<!-- source: internal/component/iface/rate.go - bindMetricsRegistry -->
<!-- source: internal/component/telemetry/collector/collector.go - Collector interface -->
<!-- source: internal/component/plugin/process/manager.go - pluginMetrics -->

External subprocess plugins still cannot publish their own arbitrary metrics through the public SDK or RPC protocol. Ze does publish process-health metrics for them from the parent process: `ze_plugin_status`, `ze_plugin_restarts_total`, and `ze_plugin_events_delivered_total`.

<!-- source: pkg/plugin/sdk/sdk.go - Plugin public SDK fields and startup path -->
<!-- source: internal/component/plugin/process/manager.go - pluginMetrics and SetMetricsRegistry -->

## What The Pull Model Would Buy

The pull-model benefits are still real:

1. Metric names can be centralized in one adapter package.
2. Snapshot-on-scrape can remove some metric updates from hot paths.
3. Tests can read a struct field directly instead of scraping or spying on a registry.
4. A second exporter for the same metric surface, such as OpenTelemetry, can be added as another adapter.

## Decision

Defer the pull-model refactor.

The Prometheus implementation should continue to use the existing push-model `metrics.Registry` path. Ze already has type-level Prometheus isolation, and the current registry model fits the component and plugin registration pattern.

<!-- source: internal/core/metrics/metrics.go - Registry interface -->
<!-- source: internal/component/plugin/registry/registry.go - ConfigureMetrics -->

Do not introduce a half pull-model beside the registry model. A future pull-model refactor should replace the registry update path for the affected metric surface in one sitting, after the external-plugin metrics shape is known.

## Why Not Now

The original motivator, Prometheus leaking into business logic, does not apply strongly to Ze. Prometheus is already isolated behind `metrics.Registry` for normal component and plugin metrics.

<!-- source: internal/core/metrics/metrics.go - Registry interface -->
<!-- source: internal/core/metrics/prometheus.go - Prometheus backend imports -->

The hard unresolved piece is external-plugin metrics. A pull adapter needs to know where snapshots come from. In-process plugins can expose a Go method, but subprocess plugins need a protocol shape. Ze currently has parent-owned process-health metrics, but no SDK/RPC hook for plugin-authored metric snapshots.

<!-- source: internal/component/plugin/process/manager.go - pluginMetrics -->
<!-- source: pkg/plugin/sdk/sdk_callbacks.go - public callback surface -->

## External-Plugin Metrics Options

If the pull model is adopted later, external-plugin metrics need one of these choices:

| Option | Shape | Tradeoff |
|--------|-------|----------|
| Snapshot RPC | Add a plugin RPC returning structured metrics on scrape | Uniform, but scrape fans out over subprocess RPC and every external plugin needs the method |
| Streaming metrics | Plugins periodically push snapshots to the parent, parent caches them | No scrape fan-out, but exported values are stale by up to the push interval |
| In-process only | Pull model for internal components only | Simple, but external plugin-authored metrics remain absent |

None of these is clearly right today. The answer should be decided by the plugin protocol, not by a Prometheus refactor.

## Flow Export Relationship

Flow export does not reopen this decision. sFlow, NetFlow v9, and IPFIX are protocol export paths for interface counters, packet samples, and flow records. They are not alternate exporters for the same Ze Prometheus metric surface.

<!-- source: docs/architecture/flowexport/flow-export-0-umbrella.md - Task and Data Flow -->
<!-- source: docs/architecture/flowexport/flow-export-1-counter-export.md - Data Flow -->

The flow-export component should use raw interface snapshots from `iface.rateTracker` or another explicit data path, and it should use `metrics.Registry` only for its own health counters such as datagrams, bytes, and errors.

<!-- source: internal/component/iface/rate.go - rateTracker.collect -->
<!-- source: docs/architecture/flowexport/flow-export-1-counter-export.md - Acceptance Criteria AC-11 -->

## Revisit Triggers

Reopen this decision when one of these happens:

| Trigger | Why it matters |
|---------|----------------|
| External-plugin metrics need to expose plugin-authored values | The protocol decision determines whether snapshots are local calls, RPC calls, or cached pushes |
| A second exporter for the same Ze metric surface is required | An adapter split pays for itself when Prometheus is no longer the only consumer |
| Hot-path profiles show metric updates as significant | Snapshot-on-scrape could remove that cost from UPDATE or packet paths |
| Metric-name drift becomes operationally painful | Centralized names become more valuable as dashboards and alerts grow |

## Consequences

New Ze Prometheus metrics should follow the registry-backed push model unless this decision is explicitly reopened.

<!-- source: docs/plugin-development/metrics.md - Implementation Pattern -->

Metric names remain local to the owning component or plugin for now. New names should follow the existing `ze_<subsystem>_<thing>_<unit>` convention and the plugin metrics naming policy.

<!-- source: docs/plugin-development/metrics.md - Naming Policy -->

Tests may use a Prometheus registry, `NopRegistry`, or a spy registry depending on what they need to prove. A test-only snapshot struct should not be treated as the start of a production pull-model adapter.

<!-- source: internal/core/metrics/nop.go - NopRegistry -->
<!-- source: internal/core/metrics/prometheus_test.go - PrometheusRegistry tests -->
