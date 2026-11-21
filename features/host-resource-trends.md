# Host Resource Trends

## Meta

| Field | Value |
|-------|-------|
| Name | Host Resource Trends |
| Kind | daemon |
| Scope | partial |
| Scope gaps | file descriptor, conntrack and disk rules, the kernel slab cache names, the fill pattern, history kept across a daemon restart, the host-trend health row, ze_host_trend metrics |
| Level | experimental |
| Components | internal/component/host/trend, internal/core/stats, internal/plugins/host-cmd, internal/plugins/host, cmd/ze/hub |
| Real-path tests | test/plugin/host-trend-runs-without-telemetry.ci, test/plugin/host-trend-show.ci, test/plugin/host-trend-doctor.ci |
| Docs | docs/guide/command-reference.md, docs/architecture/host/observability.md |
| Doc review | 2026-11-21: written with the first version; the source anchors in the Description name the files that hold the monitor, the rule, the command and its offline fallback |
| Defect review | 2026-11-21: no plan/journal/ row names the host trend monitor or the kernel-memory rule; the owed work is in plan/release/validation/spec-host-resource-trend-alerts.md |

## Description

The daemon keeps a short history of host resources and warns when a built-in rule sees one heading for exhaustion, so a slow leak is reported before the box fails. The trend monitor starts with the daemon whether or not telemetry or BGP is configured, samples each rule on its own period into hourly buckets, and judges whether the hourly floor rose for 6 consecutive hours, by at least 1% of total memory, on a line that reaches the limit within 7 days. A rule confirms after 3 such judgments and raises one warning on the report bus, with the fix `show host trend`, and the warning clears after 3 quiet judgments. The first version has one rule, `kernel-memory`: the kernel memory that cannot be reclaimed (`SUnreclaim` in `/proc/meminfo`), against a limit of that level plus `MemAvailable`. The operator meets it in `show warnings`, in `show doctor` (the `host-trend-monitor` check, which reports a monitor that is not running or a rule that cannot read its resource, and the `host-trend-kernel-memory` check, which reports the rule's standing warning), and in `show host trend`, which lists every rule with its state, level, limit, observed duration, growth, projected time to exhaustion and warning, each figure absent rather than zero when the rule has none. With no monitor running `show host trend` refuses and names `ze signal restart`; with no daemon reachable it says the daemon is needed. <!-- source: internal/component/host/trend/monitor.go -- Start, Stop, sample, fold --> <!-- source: internal/component/host/trend/register_kernel_memory.go -- readKernelMemory, describeKernelMemory --> <!-- source: internal/component/host/trend/register.go -- checkMonitor, ruleCheck --> <!-- source: cmd/ze/hub/main_system.go -- startTrendMonitor, stopTrendMonitor --> <!-- source: internal/plugins/host-cmd/cmd/show_trend.go -- handleShowTrend, trendRuleOf --> <!-- source: internal/plugins/host/trend_offline.go -- runShowTrendOffline -->
