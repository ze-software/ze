---
spec: dropped-error-5-forwarding
topic: Diagnostics
---

## Before

Across interface management, DHCPv4/DHCPv6 clients, IPv6 Router Advertisements,
the Linux netlink and VPP interface monitors, and the sysctl plugin, 32 runtime
failures were logged only at debug level and never raised on the report bus, so
an operator at the default log level did not see them:

- a leased DHCPv4 or DHCPv6 address or DHCPv4 default route that failed to be
  removed on expiry, decline, or shutdown could remain in the kernel silently,
  while already-absent kernel objects were logged the same way as real failures;
- DHCPv6 leased-address ownership registration failures and shutdown Release
  build or socket-open failures were hidden at debug;
- sysctl EventBus notifications after applying a kernel tunable or answering a
  query, and unreadable original `/proc/sys` values before first write (which
  prevent restoring the tunable on stop), were dropped at debug;
- configured addresses refused during deferred interface reconciliation,
  per-interface sysctl or profile publish failures, `interface:rollback`
  publish failures, and failed listings of pre-existing IPv6 default routes
  before RA default-route suppression were hidden at debug;
- DHCP lease publish failures, Linux netlink and VPP interface monitor EventBus
  publish failures, and ICMPv6 Router Advertisement send failures were dropped
  at debug.

## After

Each of those 32 failures now logs a WARN message naming the interface, object,
or tunable, why it matters, and the exact command to inspect or recover, under
its own registered doctor code, and raises an error event on the daemon's report
bus so `ze show errors` and the `recent-events` check of `ze show doctor` list
it. Sysctl raises reach the daemon over the plugin SDK report-issue RPC whether
`sysctl` runs in-process or as a forked plugin.

The 13 new doctor event codes are:

- Interfaces: `doctor-iface-address-add`, `doctor-iface-sysctl-emit`,
  `doctor-iface-rollback-emit`, `doctor-iface-stale-route-cleanup`;
- Sysctl: `doctor-sysctl-event-emit`, `doctor-sysctl-original-unread`;
- DHCP client: `doctor-iface-dhcp-event-emit`, `doctor-iface-dhcp-remove-failed`,
  `doctor-iface-dhcp-lease-register`, `doctor-iface-dhcp-release-failed`;
- Monitors and Router Advertisements: `doctor-iface-netlink-monitor-emit`,
  `doctor-vpp-monitor-emit`, `doctor-iface-ra-send`.

Already-absent kernel objects (`ENOENT`, `ESRCH`, `EADDRNOTAVAIL`) during DHCP
address or route removal, benign `EEXIST` or backend-not-ready errors during
deferred address reconciliation, and closed-socket RA sends during shutdown
complete quietly without false warnings. Repeating failure paths (DHCP event
publish, netlink and VPP monitor emit, and Router Advertisement sends) suppress
duplicate warnings and report-bus events until recovery or the next rate-limit
window so link flaps do not flood logs.

## Corrects

none

## Exposure

none

## Upstream

none

## Asked by

none
