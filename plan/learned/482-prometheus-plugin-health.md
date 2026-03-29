# 482 -- Prometheus Plugin Health Metrics

## Context

Ze had 47 Prometheus metrics covering BGP sessions, wire layer, RIB, and config, but zero visibility into plugin infrastructure health. Operators could not see plugin stage, restart counts, or event delivery rates. The plugin infrastructure (`process/`, `server/`) had no access to the metrics registry. This was deferred from spec-prometheus-deep Phase 4 because it required threading the registry through Server and ProcessManager.

## Decisions

- Event-driven status gauge via `SetStage` callback, over periodic polling, because stage transitions are infrequent and polling adds goroutines plus staleness.
- Thread `metrics.Registry` via ServerConfig chain (Reactor -> ServerConfig -> Server -> Manager -> ProcessManager), over using the global `registry.GetMetricsRegistry()`, because explicit threading is more testable and keeps `process/` independent of `plugin/registry/`.
- Delete only the status gauge label on disable (not counters), over deleting all labels, because Prometheus counters must not be deleted mid-lifetime (breaks `rate()` and `increase()` queries). Counters are preserved for post-mortem.
- Function callbacks (`onStageChange`, `deliveryInc`) on Process, over adding metrics fields directly, because it avoids importing `metrics` in the process package and keeps the coupling minimal.
- Manager stores registry as `any` to avoid import cycle, with type assertion in `spawnProcesses` and a logged warning on assertion failure.

## Consequences

- Ze now exposes 50 Prometheus metrics (up from 47): `ze_plugin_status`, `ze_plugin_restarts_total`, `ze_plugin_events_delivered_total`.
- All plugin health categories from the original spec-prometheus-deep plan are now implemented. No remaining gaps.
- The `MetricsRegistry` field on `ServerConfig` is available for future use by other server-side components needing metrics access.
- Callbacks on Process are set before start and cleared on disable/respawn. The pattern requires "set before start" discipline; documented but not enforced at compile time.

## Gotchas

- `onStageChange` callback must be called outside `stageMu` lock to avoid holding the lock during Prometheus internal lock acquisition. If called inside, a panic in the callback would skip `stageCh` close, hanging `WaitForStage` callers.
- Old process callbacks must be nil'd before deleting metric labels in the disable path, otherwise the dying process can re-create deleted labels via `SetStage` during shutdown.
- The Manager's `SetMetricsRegistry(any)` silently no-ops if the type assertion to `metrics.Registry` fails. A warning log was added, but the `any` indirection is inherently fragile.
- Internal test plugins ("crash", "cycle") registered in `process_test.go` may not resolve correctly via `ResolvePlugin` for config names that differ from registered names. Tests that need `Respawn` to succeed must use registered names directly.

## Files

- `internal/component/plugin/process/manager.go` -- `pluginMetrics` struct, `SetMetricsRegistry`, `wireMetrics`, `clearProcessCallbacks`, `deletePluginStatusLabel`
- `internal/component/plugin/process/process.go` -- `onStageChange`/`deliveryInc` callback fields, `SetStage` calls callback outside lock
- `internal/component/plugin/process/delivery.go` -- `deliveryInc` call in `Deliver`
- `internal/component/plugin/server/config.go` -- `MetricsRegistry` field on `ServerConfig`
- `internal/component/plugin/server/server.go` -- `SetProcessSpawner` forwards registry via interface assertion
- `internal/component/plugin/manager/manager.go` -- `metricsRegistry` field, `SetMetricsRegistry`, `spawnProcesses` threading
- `internal/component/bgp/reactor/reactor.go` -- passes `metricsRegistry` to `ServerConfig`
- `internal/component/plugin/process/manager_metrics_test.go` -- 6 unit tests covering all ACs
- `test/plugin/cli-metrics-plugin-health.ci` -- functional test
- `docs/features.md` -- Plugin Health Metrics section
