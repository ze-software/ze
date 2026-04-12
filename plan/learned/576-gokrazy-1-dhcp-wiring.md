# 576 -- gokrazy-1: DHCP Config Wiring

## Context

Ze's DHCP plugin (ifacedhcp) implemented DHCPv4 DORA and DHCPv6 SARR with lease
renewal, but it was not config-driven. The YANG schema had `dhcp { enabled, hostname,
client-id }` and `dhcpv6 { enabled, pd.length, duid }` leaves, but `config.go` did
not parse them and the interface plugin did not start DHCP clients. DHCP lease payloads
carried Router and DNS fields but nothing applied them -- no route, no resolv.conf.
This was the first step toward replacing gokrazy's built-in DHCP with ze's own.

## Decisions

- Config parsing in `config.go` over a separate config root -- DHCP is per-interface-unit,
  not a standalone plugin config. The ifacedhcp plugin has no ConfigRoots of its own.
- Factory callback pattern (`SetDHCPClientFactory`) over direct import -- the iface package
  cannot import ifacedhcp (circular), so ifacedhcp registers a factory at init() time.
  Chose factory over interface because the iface package only needs "create and start".
- `DHCPStopper` interface (just `Stop()`) over storing concrete type -- minimal coupling.
- `dhcpEntry` stores both client and params for reconcile-on-change over blindly keeping
  running clients -- hostname/client-id changes require restart.
- `/tmp/resolv.conf` over `/etc/resolv.conf` -- gokrazy rootfs is read-only SquashFS.
  Documented as gokrazy-specific; configurable path deferred.
- Last-writer-wins for resolv.conf over per-interface DNS tracking -- simpler, matches
  common DHCP client behavior. Stale DNS on lease expiry is better than no DNS.
- `RouteReplace` (idempotent) for install, `RouteDel` with ESRCH tolerance for remove --
  avoids errors on double-add or double-remove.

## Consequences

- Operators can enable DHCP from config: `interface { ethernet eth0 { unit 0 { dhcp { enabled true } } } }`
- DHCP leases now install default routes and write DNS to `/tmp/resolv.conf`
- The Backend interface gained `AddRoute`/`RemoveRoute` -- all backend implementations
  (netlink, stub, test mocks) had to be updated. Future backends must implement these.
- DHCPPayload gained `DNSAll` and `NTPServers` fields -- NTP plugin (spec-gokrazy-2)
  can subscribe to lease events and discover NTP servers via option 42.
- The `reconcileDHCP` function in register.go handles all 7 interface types and
  restarts clients when config parameters change on reload.

## Gotchas

- Config list syntax uses brackets: `address [10.0.0.1/24]` not `address 10.0.0.1/24`.
  Two functional tests failed until this was fixed.
- The `publishDHCP` wrapper had a redundant `Unit` field that shadowed the embedded
  `DHCPPayload.Unit`. Removed the wrapper entirely.
- DHCPv6 does not provide a default gateway (that comes from Router Advertisements).
  Only DHCPv4 installs the default route.
- `reconcileDHCP` initially only iterated ethernet and dummy. Review caught that veth,
  bridge, tunnel, wireguard, and loopback units were silently skipped.

## Files

- `internal/component/iface/config.go` -- dhcpUnitConfig, dhcpv6UnitConfig, parsers
- `internal/component/iface/register.go` -- reconcileDHCP, DHCPStopper, factory
- `internal/component/iface/backend.go` -- AddRoute, RemoveRoute
- `internal/component/iface/dispatch.go` -- dispatch functions
- `internal/component/iface/iface.go` -- DHCPPayload extended
- `internal/plugins/ifacedhcp/dhcp_linux.go` -- DHCPConfig, updated constructor
- `internal/plugins/ifacedhcp/dhcp_v4_linux.go` -- route/DNS/NTP/hostname/client-id
- `internal/plugins/ifacedhcp/dhcp_v6_linux.go` -- DNS writing
- `internal/plugins/ifacedhcp/resolv_linux.go` -- resolv.conf writer (new)
- `internal/plugins/ifacedhcp/register.go` -- factory bridge
- `internal/plugins/ifacenetlink/manage_linux.go` -- AddRoute/RemoveRoute
- `test/parse/dhcp-config-enabled.ci` -- functional test (new)
- `test/parse/dhcp-config-disabled.ci` -- functional test (new)
- `test/parse/dhcp-static-coexist.ci` -- functional test (new)
