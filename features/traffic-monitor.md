# Traffic Monitor

## Meta

| Field | Value |
|-------|-------|
| Name | Traffic Monitor |
| Kind | daemon |
| Scope | complete |
| Level | experimental |
| Components | internal/component/trafficstat, internal/core/portname |
| Real-path tests | test/plugin/traffic-monitor.ci |
| Docs | docs/guide/command-reference.md, docs/architecture/traffic/traffic-analysis-layers.md |
| Doc review | 2026-10-07: every source anchor resolves; the amplification overlay holds 7 ports (amplificationPorts in internal/core/portname/portname.go) |
| Defect review | 2026-10-07: audit found no open immediate spec against trafficstat; journal rows naming a Component, not each re-verified here: unwired-feature.md:39 |
| Extra criteria | supported: each journal row naming a Component re-verified as fixed or not a defect = none yet |

## Description

Lazy consumer-refcounted aggregation service (`internal/component/trafficstat`) that subscribes to the observation feed and maintains a time-windowed ranked usage view (per-interface rates, top-N source/dest IPs, top ports with service names, protocol mix, 60s history). The aggregator is rebuilt on the shared `internal/core/stats` rolling-window primitive; severity is now a display-only CLI computation from the history facts (the neutral layer holds no verdict). Runs only while consumers are attached. Consumed by `show traffic stat` (one-shot JSON snapshot), `monitor traffic stat` (full-screen alt-screen TUI via the generic MonitorProvider registry), and `ddos/detect` (Depth-1: pre-computed per-interface rates instead of raw counter diffing). Includes `internal/core/portname` hardcoded port-to-service-name table with amplification-vector overlay for 7 known reflection ports. <!-- source: internal/component/trafficstat/service.go -- lazy refcounted service --> <!-- source: internal/component/trafficstat/window.go -- per-key rolling window and rate derivation --> <!-- source: internal/component/trafficstat/cmd/traffic.go -- show/monitor handlers, displaySeverity --> <!-- source: internal/component/trafficstat/cmd/render.go -- full-screen TUI renderer --> <!-- source: internal/core/portname/portname.go -- port-to-service-name table --> <!-- source: internal/core/stats/window.go -- shared rolling-window primitive -->
