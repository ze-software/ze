# Active Probes: Ping, Traceroute and Route Lookup

An operator, or an agent through MCP, validates a forwarding path from the
router itself. A gokrazy appliance carries no `ping` and no `traceroute`
binary, so these are raw-socket implementations inside the daemon.

<!-- source: internal/component/ping/cmd/ping.go -- ICMP echo probe -->
<!-- source: internal/component/traceroute/cmd/traceroute.go -- per-hop TTL probe -->
<!-- source: internal/component/traceroute/cmd/probe_round.go -- batch probe round -->
<!-- source: internal/component/iface/cmd/show_route_lookup.go -- kernel longest-prefix lookup -->

## No shelling out

Ping opens `net.ListenPacket("ip4:icmp", "")` and builds the Echo Request
itself. Running the system `ping` binary would carry a command-injection
surface, would return text instead of structured output, and would not exist on
gokrazy at all.

Traceroute reuses that ICMP code (`buildICMPEcho`, `icmpChecksum`) instead of
taking a dependency. The whole per-hop logic is about 200 lines, and a
third-party library for that would have to be evaluated and maintained.

Per-hop TTL control uses the `golang.org/x/net/ipv4` and `ipv6` PacketConn
wrappers, not raw syscalls. `x/net` was already a dependency, and it sets
TTL and hop limit across platforms. A `ttlSetter` interface hides
`SetTTL` against `SetHopLimit`, because the probe loop body is identical for
both families. IPv6 works through the same path, selected by `dest.Is6()`.

## Reply matching

A reply is accepted only when the identifier (derived from the PID) and the
sequence number match, and the read loop skips everything else. On a shared
host, other processes are pinging at the same time on the same socket family.

Time Exceeded verification is not done. Reaching the original ICMP header
inside a Time Exceeded message means parsing the embedded IP header with its
variable IHL. In a diagnostic context a false positive hop is acceptable, so
the parsing is skipped on purpose.

## Bounds and privileges

Ping count is capped at 100 and the timeout at 30 seconds, so one CLI call
cannot hold resources indefinitely. Both commands need `CAP_NET_RAW`. ze runs
as root on gokrazy, so this is always available in production. Without the
capability the error names `CAP_NET_RAW`.

## Route lookup

`show ip route lookup <dest>` returns the kernel's longest-prefix answer:
matching prefix, next hop, interface, protocol and metric. It calls
`RouteGet` from `vishvananda/netlink` on Linux and returns "not available"
through a build-tag stub elsewhere. The existing `show ip route`, which filters
on an exact prefix, is unchanged.

## The offline wrapper was deleted, not kept

`ze traceroute` used to wrap the OS binary. It was removed when `show
traceroute` landed. On gokrazy there is no OS traceroute to wrap, and on a
normal Linux host the operator can run `traceroute` directly. Keeping both
paths would mean two behaviors under one name (`ai/rules/no-layering.md`).
