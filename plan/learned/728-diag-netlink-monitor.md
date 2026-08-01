# 728 -- diag-netlink-monitor

## Context

Gokrazy appliances lack `ip monitor` for observing kernel route, link, and address changes in real time. Ze needed a streaming diagnostic command to fill this gap, wired into the existing streaming infrastructure (StreamingHandler, BubbleTea monitor mode, MCP task streaming).

## Decisions

- Used `vishvananda/netlink` subscribe APIs over raw AF_NETLINK syscalls: the library is already vendored for routewatch, provides typed updates, and handles netlink message parsing.
- Unified output channel for all netlink groups over per-goroutine json.Encoder: multiple goroutines sharing a json.Encoder is a data race. Forwarder goroutines send to a single channel; one writer goroutine encodes.
- Placed handler code in `internal/component/cmd/show/` (where system diagnostics live) with streaming registration in `register_netlink_monitor.go`, over creating a new `cmd/monitor/` directory: the handler only needs `RegisterStreamingHandler`, not a full verb package.
- YANG entry goes in `ze-monitor-cmd.yang` (monitor verb tree) over `ze-cli-show-cmd.yang`: the spec originally named the show YANG file, but the monitor verb has its own module.
- Added both StreamingHandler and RPC handler: the RPC handler enables dispatch-command testing via the plugin API; the streaming handler does the actual monitoring.

## Consequences

- `monitor system netlink` is the pattern for future system-level streaming monitors (e.g., conntrack, iptables counters).
- The register_*.go pattern bypasses the block-init-register hook for non-RegisterRPCs registrations. The hook should be updated to also allow RegisterStreamingHandler.

## Gotchas

- The spec named `ze-cli-show-cmd.yang` for the YANG update. The correct file is `ze-monitor-cmd.yang`. Always check the YANG module for the command's verb, not the show module.
- goconst flags string literals across build-tagged file pairs. Constants must be in a platform-independent file visible to both linux and other builds.
- The block-init-register hook allows RegisterRPCs but not RegisterStreamingHandler. Workaround: use a `register_*.go` filename which is exempt.

## Files

- `internal/component/iface/cmd/monitor_netlink.go` (created)
- `internal/component/iface/cmd/monitor_netlink_linux.go` (created)
- `internal/component/iface/cmd/monitor_netlink_other.go` (created)
- `internal/component/iface/cmd/monitor_netlink_test.go` (created)
- `internal/component/iface/cmd/monitor_netlink_linux_test.go` (created)
- `internal/component/bgp/plugins/cmd/monitor/yang/ze-monitor-cmd.yang` (modified)
- `test/plugin/monitor-system-netlink.ci` (created)
- `docs/architecture/api/commands.md` (modified)
- `docs/features.md` (modified)
- `docs/guide/command-reference.md` (modified)
- `docs/guide/production-diagnostics.md` (modified)
