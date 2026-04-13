# 582 -- iface-route-priority

## Context

Link-state failover (spec-gokrazy-4) toggled DHCP default routes between metric 0 and a
hardcoded 1024 on carrier change. Multi-uplink setups (e.g., gokrazy with eth and wlan)
could not express "prefer eth over wlan when both are up" because both used metric 0. The
goal was a per-unit `route-priority` YANG leaf so operators set a base metric per interface,
with deprioritization relative to that base (configured + 1024).

## Decisions

- Placed `route-priority` on the interface unit (not the interface itself) because DHCP is
  per-unit and routes are installed per-unit.
- Upper bound 4294966271 (2^32 - 1 - 1024) ensures configured + 1024 never overflows uint32.
- Metric flows through the existing `dhcpClientFactory` signature (added `routeMetric int`
  parameter) over restructuring the factory to accept a struct. The factory signature already
  had 10 parameters; one more was simpler than a breaking refactor.
- `dhcpParams.routePriority` triggers DHCP client restart on reload when only the metric
  changes, because the metric is part of the comparison key.

## Consequences

- gokrazy appliances can now set eth=1, wlan=5 to prefer wired uplink.
- The factory signature now has 11 parameters. If another field is needed, consider
  restructuring to pass DHCPConfig directly instead of individual parameters.
- IPv6 default routes are unaffected (DHCPv6 does not install default routes in ze).

## Gotchas

- The `dhcpClientFactory` signature had to change despite the handoff claiming otherwise.
  Individual parameters are passed through the factory, not a struct, so there was no way
  to add the metric without a new parameter.
- `baseMetric` was initially a separate field on `dhcpEntry` but was redundant with
  `params.routePriority`. Removed during review; handlers use `entry.params.routePriority`.
- Link failover tests (handleLinkDown/handleLinkUp) don't need Linux. They use a
  route-tracking `fakeBackend` registered via `RegisterBackend`/`LoadBackend`, which
  records AddRoute/RemoveRoute calls for assertion. No netlink required.
- Pre-existing lint failures on macOS (loggerPtr unused in ifacedhcp.go because consumers
  are in _linux.go files) and pre-existing parse test failures (dhcp tests use ze:os "linux"
  leaves) are unrelated to this spec.

## Files

- `internal/component/iface/schema/ze-iface-conf.yang` -- route-priority leaf
- `internal/component/iface/config.go` -- RoutePriority field + parsing
- `internal/component/iface/config_test.go` -- 5 unit tests (2 parse, 3 link failover)
- `internal/component/iface/register.go` -- dhcpParams, dhcpEntry, factory, failover
- `internal/plugins/ifacedhcp/ifacedhcp.go` -- DHCPConfig.RouteMetric
- `internal/plugins/ifacedhcp/dhcp_v4_linux.go` -- AddRoute/RemoveRoute with metric
- `internal/plugins/ifacedhcp/register.go` -- factory parameter
- `test/parse/route-priority.ci` -- functional test
- `docs/features/interfaces.md` -- capability table
- `docs/guide/configuration.md` -- Route Priority section
