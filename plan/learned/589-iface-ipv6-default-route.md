# 589 -- IPv6 Default Route from RA with Configurable Metric

## Context

IPv4 default routes via DHCP already respected the `route-priority` config leaf, but IPv6 default routes were installed by the kernel from Router Advertisements with metric 0, outside ze's control. The sysctl `accept_ra_defrtr` (per-interface) controls whether the kernel installs these routes. Setting it to 0 suppresses the default route without disabling RA processing (SLAAC, prefix info, RDNSS all continue). Ze needed to suppress the kernel route, detect routers via NDP neighbor events, and install its own `::/0` route with the configured metric.

## Decisions

- Chose netlink NeighSubscribe + NTF_ROUTER flag over raw RA packet parsing, because the kernel already processes RAs and exposes router identity through the neighbor table.
- Router events flow through the event bus (EventInterfaceRouterDiscovered/Lost) rather than direct calls, consistent with the IPv4 DHCP pattern and enforced by the package boundary (ifacenetlink cannot import iface).
- Separate `activeRouters` map (keyed by ifaceName+routerIP) over extending the existing `activeDHCP` map, because IPv6 routers are discovered via NDP, not DHCP. Both maps share `dhcpMu`.
- Link-local gateways passed as bare IPs (no zone ID) because `AddRoute` already sets `route.LinkIndex` from `ifaceName`, and `net.ParseIP` rejects zone IDs.
- No router lifetime timer (YAGNI): NUD covers active links within ~30s; on idle links, a stale route has no operational impact.
- Stale kernel route cleanup after sysctl suppression (scan + remove existing `::/0` routes) to handle the startup race where kernel installs a route before ze suppresses.
- Crash recovery documented as known limitation rather than implementing a recovery mechanism: on restart with same config, ze re-sets to 0 (no harm).

## Consequences

- `route-priority` now works for both IPv4 (DHCP) and IPv6 (RA) address families.
- ListRoutes was added to the Backend interface for stale route enumeration, available for future use.
- The monitor now subscribes to three netlink channels (Link, Addr, Neigh).
- Two new event types (router-discovered, router-lost) are available on the bus for any future consumer.

## Gotchas

- `net.ParseIP` returns nil for addresses with zone IDs (`fe80::1%eth0`), so gateway must always be bare IP.
- If ze crashes (SIGKILL), `accept_ra_defrtr` stays at 0. Manual `sysctl -w` or reboot required if config changes while ze is dead.
- On idle links where NUD never fires, a disappeared router's stale default route persists until traffic triggers NUD. No operational impact (no traffic to misroute).

## Files

- `internal/plugins/ifacenetlink/monitor_linux.go` -- NeighSubscribe, NTF_ROUTER tracking
- `internal/component/iface/register.go` -- router tracking, failover, sysctl management
- `internal/component/plugin/events.go` -- EventInterfaceRouterDiscovered/Lost
- `internal/component/iface/backend.go` -- ListRoutes on Backend interface
- `internal/component/iface/dispatch.go` -- RouterEventPayload type
- `internal/component/iface/iface.go` -- IPv6 route helpers
- `internal/plugins/ifacenetlink/manage_linux.go` -- ListRoutes netlink implementation
- `test/parse/ipv6-route-priority.ci` -- functional test
