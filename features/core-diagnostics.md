# Core Diagnostics

## Meta

| Field | Value |
|-------|-------|
| Name | Core Diagnostics |
| Kind | daemon |
| Scope | partial |
| Scope gaps | BFD raw capture cannot be selected alone |
| Level | experimental |
| Components | internal/plugins/diag, internal/core/privilege, internal/component/iface/cmd/monitor_netlink_linux.go |
| Real-path tests | test/plugin/system-sockets-show.ci, test/plugin/system-kernel-log-show.ci, test/plugin/system-goroutines-show.ci, test/plugin/tcp-check-show.ci, test/plugin/traceroute-show.ci, test/plugin/monitor-traceroute.ci, test/plugin/capture-interface-show.ci, test/plugin/system-profile-show.ci, test/plugin/system-fd-show.ci, test/plugin/runtime-memory-show.ci, test/plugin/monitor-system-netlink.ci, test/ui/monitor-ping-pipe-resolve-log.ci |
| Docs | docs/guide/production-diagnostics.md |
| Doc review | 2026-10-07: protocol leaf of ze-diag-cmd.yang holds only l2tp and bgp, as the row says; privilege.CheckPrivileges results are printed as warnings in cmd/ze/hub/main.go and startup continues, as the row says |
| Defect review | 2026-10-07: journal 2026-09-14 diag capture protocol leaf versus dispatcher arm, unfixed and carried as a Scope gap |
| Extra criteria | supported: show capture raw selects BFD alone = test/plugin/capture-raw-bfd.ci |

## Description

Built-in diagnostic commands replacing ss, dmesg, lsof, dig, nc, traceroute, mtr, ping, tcpdump, and pprof on gokrazy appliances: `show system sockets` (TCP/UDP state), `show system kernel-log` (dmesg), `show system goroutines` (dump with singleflight dedup), `show tcp-check` (port probe), `show traceroute` (ICMP path trace with per-hop RTT, IPv4/IPv6), `monitor traceroute` (live mtr-style continuous trace with `\| log \| resolve` and `\| log \| origin` enrichment), `monitor ping` (continuous ICMP ping with live stats), `show capture interface` (AF_PACKET live capture with BPF filters, pcap or text output), `show system file-descriptors` (FD counts and limits), `show dns lookup/cache` (DNS resolution, cache listing, selective delete, flush, stats reset), `show system profile` (cpu/heap pprof), `show system memory` (/proc/self/status OS view). `monitor system netlink` streams kernel route/link/address change events as JSON (replaces `ip monitor`). A BFD raw capture ring is started and dumped with the other protocols when `show capture raw` names no protocol; its `protocol` leaf offers only `l2tp` and `bgp`, so BFD cannot be selected alone. At startup Ze warns when it runs without root or without a required capability, and keeps running. See [Production Diagnostics Guide](guide/production-diagnostics.md). <!-- source: internal/component/iface/cmd/monitor_netlink_linux.go -- streamNetlinkMonitor --> <!-- source: internal/plugins/diag/cmd/capture_raw.go -- captureRawStart, captureRawDump --> <!-- source: internal/core/privilege/check_linux.go -- CheckPrivileges -->
