# Flow Export

## Meta

| Field | Value |
|-------|-------|
| Name | Flow Export |
| Page | docs/guide/flow-export.md |
| Kind | protocol |
| Scope | complete |
| Level | experimental |
| Components | internal/plugins/flowexport |
| Real-path tests | test/flow-export/sflow-export.ci, test/flow-export/netflow9-export.ci, test/flow-export/ipfix-export.ci, test/flow-export/multi-collector-export.ci, test/flow-export/collector-reload.ci, test/flow-export/conntrack-config.ci, test/flow-export/sampling-config.ci, test/flow-export/flow-export-show.ci |
| RFCs | rfc7011, rfc3954 |
| Docs | docs/guide/flow-export.md |
| Doc review | 2026-10-07: every source anchor resolves; ze_flowexport_datagrams_total, ze_flowexport_flows_active and ze_flowexport_recent_ring_drops are registered in internal/plugins/flowexport |
| Defect review | 2026-10-07: open: spec-bmp-sflow-export-rfc-defects names the sFlow exporter; journal rows naming a Component, not each re-verified here: bound-too-small-for-its-own-burst.md:18, guard-added-to-one-half-of-a-pair.md:40, mtime-granularity-stamp.md:4, mtime-granularity-stamp.md:5, stale-spec-claims-done.md:18, unwired-feature.md:44 |
| Extra criteria | supported: collector interop against nfdump = internal/plugins/flowexport/interop_integration_linux_test.go::TestFlowExportNFDumpInterop; supported: collector interop against sflowtool = internal/plugins/flowexport/interop_integration_linux_test.go::TestFlowExportSFlowToolInterop; supported: each journal row naming a Component re-verified as fixed or not a defect = none yet |

## Description

Interface counter and per-flow record export over UDP via sFlow v5, NetFlow v9 (RFC 3954), and IPFIX (RFC 7011). Per-collector polling interval and template refresh. Packet sampling (tc sample + psample) exported as sFlow flow samples with configurable 1-in-N rate, header truncation, and psample group. Conntrack-based per-flow records (periodic table dumps) for NetFlow v9 and IPFIX. Optional BGP next-hop enrichment from the RIB best-change event. `show flow export [<collector>]` reports per-collector datagrams-sent, bytes-sent, errors, sequence, and last-export-time. `show flow recent [dst <prefix>]` returns recent conntrack flow records (5-tuple + TCP state) from a bounded drop-oldest ring (`recent-flow-ring`, default 4096 records, allocated only when conntrack export is on) that feeds on-box DDoS characterization. Prometheus metrics: `ze_flowexport_datagrams_total`, `ze_flowexport_bytes_total`, `ze_flowexport_errors_total`, `ze_flowexport_samples_total`, `ze_flowexport_flows_total`, `ze_flowexport_flows_active`, `ze_flowexport_recent_ring_drops`. Per-flow records cover IPv4 and IPv6 (separate templates); BGP enrichment currently fills next-hop only; sampling needs Linux with CAP_NET_ADMIN and kernel psample. <!-- source: internal/plugins/flowexport/exporter.go -- newExporter, exporter.status, per-collector senders --> <!-- source: internal/plugins/flowexport/yang/ze-flowexport-conf.yang -- collector, sampling, conntrack, enrichment config --> <!-- source: internal/plugins/flowexport/cmd_show.go -- ze-flowexport:show-flow-export and ze-flowexport:show-flow-recent RPCs --> <!-- source: internal/plugins/flowexport/recent.go -- bounded recent-flow ring --> <!-- source: internal/plugins/flowexport/metrics.go -- ze_flowexport_* Prometheus metrics --> Compile-out-able with the `ze_flowexport` build tag: default-on in `ZE_FEATURES`, dropped from `ze-stripped` / bare `ze_core` builds, with its config block rejected as unknown. <!-- source: feature-gates.txt -- ze_flowexport -->
