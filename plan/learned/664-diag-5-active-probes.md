# 664 -- Diag 5: Active Network Probes

## Context

`spec-diag-5-active-probes` added two active diagnostic capabilities:
ICMP ping and kernel route lookup (longest-prefix-match). These let an
operator (or Claude via MCP) validate forwarding paths from the router's
perspective without leaving the CLI.

## Decisions

- Use raw ICMP sockets via `net.ListenPacket("ip4:icmp", "")` rather than
  shelling out to the system `ping` command. This avoids command injection
  risk, provides structured JSON output, and works on gokrazy where the
  system ping binary may not exist.
- Use vishvananda/netlink `RouteGet` on Linux for the LPM lookup. Non-linux
  platforms return "not available" via build-tag stub.
- Both commands register as online RPCs (`ze-show:ping`, `ze-show:route-lookup`)
  in the show verb package. They appear in MCP tools/list automatically via
  the YANG RPC auto-generation.
- Ping count capped at 100, timeout at 30s to prevent resource exhaustion
  from a single CLI call.
- The ICMP implementation manually builds Echo Request packets and verifies
  Reply packets by matching PID-based identifier and sequence number, which
  filters out replies to other processes' pings on shared hosts.

## Consequences

- `ping <dest> [count N] [timeout Ns]` works on any platform with
  CAP_NET_RAW. Returns JSON with per-packet RTT, loss percentage, and
  min/avg/max summary.
- `show ip route lookup <dest>` returns the kernel's LPM answer: matching
  prefix, next-hop, interface, protocol, metric. Linux only.
- Existing `show ip route` (exact prefix filter) is unchanged.

## Gotchas

- Requires CAP_NET_RAW. Ze runs as root on gokrazy so this is always
  available in production. In test environments without raw socket
  permissions, ping will fail with a clear error mentioning CAP_NET_RAW.
- IPv6 uses different ICMP type codes (128/129 instead of 8/0) and a
  different network string ("ip6:ipv6-icmp").
- The ReadFrom loop skips non-matching replies (wrong type, wrong ID,
  wrong sequence, wrong source) to handle shared-socket scenarios.

## Files

- `internal/component/cmd/show/{ping.go,ping_test.go}`
- `internal/component/cmd/show/route_lookup.go`
- `internal/component/iface/{route_lookup_linux.go,route_lookup_other.go}`
- `internal/component/cmd/show/schema/ze-cli-show-cmd.yang` (ping + route-lookup containers)
