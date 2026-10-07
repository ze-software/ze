# Health Registry

## Meta

| Field | Value |
|-------|-------|
| Name | Health Registry |
| Kind | daemon |
| Scope | partial |
| Scope gaps | l2tp health check does not probe the kernel data plane |
| Level | experimental |
| Components | internal/core/health, internal/component/l2tp/health.go, internal/plugins/flowexport/health.go |
| Real-path tests | test/plugin/health-components-show.ci, test/plugin/l2tp-health-show.ci |
| Docs | docs/guide/health-checks.md |
| Doc review | 2026-10-07: registered list read against every health.Register caller (bgp, iface, ike, l2tp, pki, plugin process, report bus, fib kernel, firewall nft, flowexport, vpp); 503 on StatusDown and panic recovery without timeout read in internal/core/health/registry.go invokeCheck |
| Defect review | 2026-10-07: plan/journal/green-that-could-not-have-been-red.md (2026-08-31) l2tp checkHealth healthy with a dead kernel data plane, unfixed, now stated in the row and carried as a Scope gap |
| Extra criteria | supported: /health answers 503 when a component is down, driven through the telemetry listener = test/plugin/health-endpoint-503.ci |

## Description

Aggregated component health via `show health` and `/health` HTTP endpoint (503 when any component is down). Registered components: l2tp, report-bus, ipsec, pki, bgp (session-stuck/flap/EOR), fib (sync-failure/orphan/lag), firewall (stale-table/drift audit), iface (error counters), plugins (crash/disabled), vpp (API socket probe), flow-export. The l2tp check reports healthy whenever the subsystem is running and does not probe the kernel data plane. Each check is expected to return within 1 second by contract; the registry recovers panics but does not enforce a timeout. <!-- source: internal/component/l2tp/health.go -- checkHealth --> <!-- source: internal/plugins/flowexport/health.go -- checkFlowExportHealth -->
