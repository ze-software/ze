# Runtime Diagnostics

## Meta

| Field | Value |
|-------|-------|
| Name | Runtime Diagnostics |
| Kind | daemon |
| Scope | complete |
| Level | experimental |
| Components | internal/component/l2tp/cmd, internal/plugins/traffic-cmd, internal/component/bgp/plugins/cmd/rib/yang/ze-rib-poolstats-cmd.yang |
| Real-path tests | test/plugin/subsystem-list.ci |
| Docs | docs/guide/command-reference.md |
| Doc review | 2026-10-07: commands read against YANG: observer, cqm, echo, reliable containers in ze-l2tp-cmd.yang, show traffic control in ze-traffic-cmd.yang, show metrics pool in ze-rib-poolstats-cmd.yang |
| Defect review | 2026-10-07: no journal row or immediate spec names these packages |
| Extra criteria | supported: show l2tp observer, cqm, echo and reliable answer on a live session = test/l2tp/l2tp-runtime-diagnostics.ci; supported: show traffic control answers on a live qdisc = test/traffic/traffic-control-show.ci; supported: show metrics pool answers on a live daemon = test/plugin/metrics-pool-show.ci |

## Description

Production debugging via CLI and MCP: `show l2tp observer` (per-session event ring), `show l2tp cqm` (per-login echo RTT/loss buckets), `show l2tp echo` (current echo state), `show l2tp reliable` (reliable transport Ns/Nr/cwnd), `show traffic control` (TC qdisc/class state), `show metrics pool` (BGP attribute pool occupancy and dedup rates), enhanced `subsystem-list` (real plugin state). All auto-exposed as MCP tools for AI-assisted troubleshooting.
