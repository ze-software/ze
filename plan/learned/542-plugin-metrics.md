# 542 -- Plugin-Owned Prometheus Metrics

## Context

Ze's metrics infrastructure was 90% there: `ConfigureMetrics` callback existed in `Registration`, `GetInternalPluginRunner` called it, and 2 of 34 plugins used it (bgp-rib, bgp-gr). But 32 plugins had no metrics, and there was no naming convention or documentation. The goal was to extend the existing pattern to more plugins and document a consistent naming policy.

## Decisions

- Followed the existing bgp-rib pattern (atomic.Pointer + SetMetricsRegistry + nil-check) over creating new abstractions, because it's proven and simple.
- Chose `ze_{scope}_{subject}_{detail}` naming taxonomy over ad-hoc names: counters always end in `_total`, gauges never do, histograms use unit suffix (`_seconds`). Scope is plugin name with hyphens stripped.
- Used `systemrib` as the metric scope for the sysrib plugin (registration name `rib`) over `sysrib`, because `ze_rib_` is already taken by bgp-rib and `systemrib` reads more naturally.
- Skipped AC-7 (prefix metrics migration from reactor to rib) because prefix limit metrics are reactor Session state, not RIB state -- they're updated during UPDATE processing in `session_prefix.go` before routes reach the RIB plugin.

## Consequences

- 7 of 34 plugins now have metrics (bgp-rib, bgp-gr, fib-kernel, sysrib, bgp-watchdog, bgp-rpki, bgp-persist). Remaining plugins with observable state (adj-rib-in, healthcheck) can follow the same pattern.
- The naming policy in `docs/plugin-development/metrics.md` constrains future metric names. New metrics must follow the taxonomy.
- Prefix limit metrics remain in `reactor_metrics.go`. Any future move would require an interface between the reactor Session and RIB plugin for prefix counting.

## Gotchas

- Pre-existing `ze_rib_routes_in_total` uses `_total` suffix on a gauge (Prometheus convention says `_total` = counter only). Not fixed in this spec to avoid churn, but the naming policy documents the correct pattern for new metrics.
- Functional `.ci` tests using Python plugins must always call `daemon shutdown` before `sys.exit()` -- if the script exits without shutting down the daemon, the test runner times out waiting for the daemon to exit.

## Files

- `internal/plugins/fibkernel/register.go`, `fibkernel.go` -- fib-kernel metrics
- `internal/plugins/sysrib/register.go`, `sysrib.go` -- sysrib metrics
- `internal/component/bgp/plugins/watchdog/register.go`, `watchdog.go`, `server.go` -- watchdog metrics
- `internal/component/bgp/plugins/rpki/register.go`, `rpki.go`, `rtr_session.go` -- rpki metrics
- `internal/component/bgp/plugins/persist/register.go`, `server.go` -- persist metrics
- `docs/plugin-development/metrics.md` -- naming policy and pattern documentation
- `test/plugin/plugin-metrics-owned.ci` -- functional test
