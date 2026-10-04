# Netlink Monitor Command

`monitor system netlink` streams kernel route, link and address changes. A
gokrazy appliance carries no `ip monitor`, so this command is the only way to
watch those events on the box.

<!-- source: internal/component/iface/cmd/monitor_netlink.go -- handler and streaming registration -->
<!-- source: internal/component/iface/cmd/monitor_netlink_linux.go -- netlink subscription and forwarding -->
<!-- source: internal/component/iface/cmd/monitor_netlink_other.go -- non-Linux stub -->

## Decisions

- The `vishvananda/netlink` subscribe API is used, not raw AF_NETLINK
  syscalls. The library is already vendored for routewatch, returns typed
  updates, and parses the netlink messages.
- One output channel carries every netlink group. Each group has its own
  forwarder goroutine, and one writer goroutine encodes. Sharing a
  `json.Encoder` between goroutines is a data race.
- The handler registers both a streaming handler and an RPC handler. The
  streaming handler does the monitoring. The RPC handler is what makes the
  command reachable from a dispatch-command test.
- The command is a pattern for later system-level streaming monitors, for
  example conntrack or firewall counters.

## Two placement traps

The YANG entry belongs to the verb's own module. This command is under the
`monitor` verb, so it goes in `ze-monitor-cmd.yang`, not in the show module.
Check which verb owns the command before editing a YANG file.

<!-- source: internal/component/bgp/plugins/cmd/monitor/yang/ze-monitor-cmd.yang -- monitor verb tree -->

`goconst` reports a repeated string literal across a build-tagged file pair. A
constant shared by the Linux file and the stub belongs in the file with no
build tag, where both builds see it.

## Functional test lifecycle

`monitor-system-netlink.ci` checks the dispatch-command contract for the default
group, the route group, and an invalid group. Its BGP peer also requires the
exact initial End-of-RIB frame. The observer MUST wait for that peer's
`eor-sent` counter before a successful return can trigger daemon shutdown;
otherwise the peer can receive a Cease notification where EOR was expected.
The peer MUST linger after its wire expectation so the API checks finish
before teardown.

The wait runs in `Observe`'s scenario worker, not in a startup callback that
must return before peers start. Plugin readiness permits command dispatch;
it does not prove that a peer's initial routing update has reached the wire.
This fixture checks the RPC response, not delivery of kernel events from the
separate streaming handler.
<!-- source: internal/test/fixture/plugin_fixture_10.go -- fixture10MonitorSystemNetlink, fixture10WaitEOR -->
<!-- source: internal/test/fixture/fixture.go -- observeConfigured -->
