# Netdata-compatible OS Telemetry

## Meta

| Field | Value |
|-------|-------|
| Name | Netdata-compatible OS Telemetry |
| Page | docs/guide/monitoring.md#os-metrics-netdata-compatible |
| Kind | daemon |
| Scope | complete |
| Level | experimental |
| Components | internal/component/telemetry/collector, internal/component/telemetry/exporter |
| Real-path tests | test/parse/telemetry-prometheus-valid.ci, test/parse/telemetry-multi-listener.ci |
| Docs | docs/guide/monitoring.md |
| Doc review | 2026-10-07: every source anchor resolves; the netdata prefix default is in internal/component/telemetry/exporter/yang/ze-telemetry-conf.yang; the row states no test compares names or count against a running Netdata, which is true |
| Defect review | 2026-10-07: audit found no open immediate spec against the collectors; journal rows naming a Component, not each re-verified here: gate-excludes-part-of-its-population.md:140, published-value-drifts-from-the-behavior-it-describes.md:21, secret-echoed-to-the-client.md:4, secret-echoed-to-the-client.md:9 |
| Extra criteria | supported: a scrape of /metrics asserted against the Netdata names = test/plugin/telemetry-netdata-scrape.ci; supported: each journal row naming a Component re-verified as fixed or not a defect = none yet |

## Description

Prometheus metrics from /proc and /sys (CPU, memory, network, disk, IPv4/IPv6 protocols, conntrack, PSI, cpuidle, cpufreq, ZFS, btrfs, mdstat, SCTP, IPVS, wireless, etc) named and labeled after Netdata's Prometheus exporter, so dashboards built on Netdata's metric names can read them. No test compares the names or the count against a running Netdata. Per-collector enable/disable, interval override, and prefix are scoped under `telemetry.prometheus.netdata` so Ze-native metrics keep their `ze_*` names. The Prometheus HTTP service defaults to loopback and can require HTTP Basic Auth. The Prometheus HTTP exporter (the `/metrics` + `/health` listener, telemetry config extraction, basic-auth, and the Netdata OS collectors) is compile-out-able with the `ze_telemetry` build tag: normal and appliance builds include it, while `ze-stripped` and bare `ze_core` builds drop `internal/component/telemetry/exporter` and `internal/component/telemetry/collector` and reject the `telemetry {}` config block as unknown. Metric COLLECTION (the always-on `internal/core/metrics` registry used by ~60 packages, plus its no-op dummy) stays linked in every build, so a no-telemetry binary still records every `ze_*` counter; it just cannot expose them over HTTP. <!-- source: internal/component/telemetry/collector/ -- Netdata-compatible OS collectors --> <!-- source: feature-gates.txt -- ze_telemetry --> <!-- source: cmd/ze/hub/register_telemetry.go -- ze_telemetry seam wiring --> <!-- source: internal/component/telemetry/exporter/server.go -- gated Prometheus exporter --> <!-- source: internal/core/metrics/exporter_hook.go -- StartExporter seam -->
