# Traffic Usage

## Meta

| Field | Value |
|-------|-------|
| Name | Traffic Usage |
| Page | docs/guide/traffic-usage.md |
| Kind | daemon |
| Scope | complete |
| Level | experimental |
| Components | internal/plugins/trafficusage |
| Real-path tests | test/plugin/traffic-usage-config.ci, test/plugin/trafficusage-external-refuses.ci, test/parse/traffic-usage.ci, test/parse/traffic-usage-invalid.ci |
| Docs | docs/guide/traffic-usage.md |
| Doc review | 2026-10-07: every source anchor resolves; the five ze_traffic_usage_* metrics and doctor-traffic-usage-ebpf exist in internal/plugins/trafficusage |
| Defect review | 2026-10-07: journal green-that-could-not-have-been-red.md and helper-bypassed-by-an-open-coded-copy.md rows name trafficusage; journal rows naming a Component, not each re-verified here: green-that-could-not-have-been-red.md:178, helper-bypassed-by-an-open-coded-copy.md:17 |
| Extra criteria | supported: each journal row naming a Component re-verified as fixed or not a defect = none yet |

## Description

eBPF TCX per-(port, protocol) and opt-in per-IP byte accounting on operator-selected interfaces, exported as Prometheus metrics and viewed via `show traffic usage [name <interface>]`. IPv4 only, monitoring only (never drops or modifies traffic), Linux >= 6.6 (no-op elsewhere; needs CAP_BPF + CAP_NET_ADMIN). The eBPF programs are assembled in pure Go (cilium/ebpf asm.Instructions) and loaded from memory: no C source, no committed .o, no clang/LLVM; hand-written assembly validated by BPF_PROG_TEST_RUN tests. Per-port accounting (ingress by dst_port, egress by src_port) is always on; `track-ip` adds per source/destination IPv4 (off by default to bound cardinality). Configurable poll `interval`, `stale-timeout` (delete unseen series to bound /metrics cardinality), and per-map LRU `max-entries` (top-talker eviction). Prometheus metrics: `ze_traffic_usage_ingress_port_bytes_total`, `ze_traffic_usage_egress_port_bytes_total`, `ze_traffic_usage_ingress_bytes_total` and `ze_traffic_usage_egress_bytes_total` (track-ip only), `ze_traffic_usage_map_entries`. A `ze doctor` check (`doctor-traffic-usage-ebpf`) warns when enabled but eBPF/TCX is unavailable. <!-- source: internal/plugins/trafficusage/monitor.go -- reconcile, lifecycle, poller --> <!-- source: internal/plugins/trafficusage/program_linux.go -- pure-Go asm.Instructions program assembly --> <!-- source: internal/plugins/trafficusage/yang/ze-traffic-usage-conf.yang -- traffic-usage config --> <!-- source: internal/plugins/trafficusage/show.go -- ze-show:traffic-usage RPC --> <!-- source: internal/plugins/trafficusage/metrics.go -- ze_traffic_usage_* Prometheus metrics --> Compile-out-able with the `ze_trafficusage` build tag: default-on in `ZE_FEATURES`, dropped from `ze-stripped` / bare `ze_core` builds, with its config block rejected as unknown. <!-- source: feature-gates.txt -- ze_trafficusage --> <!-- source: internal/le/test/deployment/actions.go -- Answer -->
