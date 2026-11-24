# A registry read outruns its lazy creation

A component reads `registry.GetMetricsRegistry()` directly at `Start`
instead of deferring through `registry.InjectPluginMetrics`. On a daemon
whose only enabled block starts before `startStandaloneTelemetry` creates
the registry, the read answers nil, and the metric it would have bound
stays absent for the process lifetime with no line saying so.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-09-09 | spec-subscriber-reader-loops-retry-a-failing-socket-without-backoff | `internal/component/l2tp/subsystem.go`, `Subsystem.Start` binding `l2tpMetrics` | Read `registry.GetMetricsRegistry()` directly at Start, so on an L2TP-only daemon every `ze_l2tp_*` series stayed unregistered | fixed: deferred L2TP metrics binding via `registry.InjectPluginMetrics` (`registerLateL2TPMetrics`, `onLateMetricsRegistry`, `Observer.attachMetrics`) with Stop/unwind detachment; `metrics_test.go` |
| 2026-09-10 | - | No-BGP telemetry startup, `runYANGConfig` and OSPF `wireV4Engine` | OSPF constructs metric handles before the registry exists. Its deferred callback later stores the registry but never replaces those handles. The live FRR cost-reload check finds no NSM counter despite enabled Prometheus | fixed: Move standalone telemetry initialization before engine and plugin startup. Keep the registry available when plugins construct their handles; preserve the live counter assertion |
