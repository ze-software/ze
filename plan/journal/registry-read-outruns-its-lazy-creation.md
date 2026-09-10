# A registry read outruns its lazy creation

A component reads `registry.GetMetricsRegistry()` directly at `Start`
instead of deferring through `registry.InjectPluginMetrics`. On a daemon
whose only enabled block starts before `startStandaloneTelemetry` creates
the registry, the read answers nil, and the metric it would have bound
stays absent for the process lifetime with no line saying so.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-09-09 | spec-subscriber-reader-loops-retry-a-failing-socket-without-backoff | `internal/component/l2tp/subsystem.go`, `Subsystem.Start` binding `l2tpMetrics` | `if reg := registry.GetMetricsRegistry(); reg != nil { bindL2TPMetrics(reg); ... }` reads the registry directly, the same gap `internal/component/l2tp/pppoe/metrics.go` and `internal/component/l2tp/ppp/metrics.go` already document and close with `InjectPluginMetrics` for their own counters. On an L2TP-only daemon with no `bgp` block, every `ze_l2tp_*` session gauge and CQM series this call binds stays unregistered for the process lifetime | not fixed here: the same call also starts `l2tpStatsPoller` and sets `s.statsPoller`, so deferring it changes when that poller starts and who holds `s.mu` when it does, which is a larger change than this phase's four reader loops. `readLoop`'s own new counter (`reader_metrics.go`) is registered through a separate `InjectPluginMetrics` hook instead of this call site, so it does not inherit the gap |
| 2026-09-10 | - | No-BGP telemetry startup, `runYANGConfig` and OSPF `wireV4Engine` | OSPF constructs metric handles before the registry exists. Its deferred callback later stores the registry but never replaces those handles. The live FRR cost-reload check finds no NSM counter despite enabled Prometheus | Move standalone telemetry initialization before engine and plugin startup. Keep the registry available when plugins construct their handles; preserve the live counter assertion |
