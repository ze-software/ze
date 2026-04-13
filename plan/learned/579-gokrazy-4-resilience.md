# 579 -- gokrazy-4: Network Resilience

## Context

After ze replaced gokrazy's DHCP and NTP, several resilience features that
gokrazy provided were missing: link-state route failover (deprioritize routes
when carrier drops), a clock readiness signal (components need to know when
the clock is trustworthy), and configurable resolv.conf path (gokrazy uses
/tmp because rootfs is read-only, standard Linux uses /etc).

## Decisions

- Link-state failover with hardcoded metrics (0 normal, 1024 deprioritized) over
  configurable route priority -- configurable metrics are not gokrazy-specific
  and were moved to `spec-iface-route-priority.md`.
- `NamespaceSystem` with `EventClockSynced` over a plugin-specific namespace --
  system-level events (clock sync, future disk/network readiness) belong in a
  shared namespace.
- Event emission (pub/sub) over queryable state -- follows ze's EventBus pattern.
  Components subscribe if they care; no blocking gate mechanism needed yet.
- `resolv-conf-path` at interface container level over per-unit DHCP config --
  the path is system-wide, not per-interface. Alongside `dhcp-auto` and `backend`.
- Empty string disables resolv.conf writing over a separate boolean -- single
  leaf, less config surface. No DNS conflict detection needed when the operator
  explicitly controls the path.
- Path validation (absolute, no traversal) matching NTP's `validatePersistPath` --
  defense in depth for a file write path.

## Consequences

- Components can subscribe to `(system, clock-synced)` to know when the clock
  is set from NTP. Currently informational; no consumer blocks on it.
- Operators can set `interface { resolv-conf-path "/etc/resolv.conf" }` for
  standard Linux or empty string to disable DNS writes entirely.
- The hardcoded `/tmp/resolv.conf` constant and `writeResolvConf` wrapper were
  removed. All DNS writes go through `writeResolvConfTo` with the config path.
- Link-state failover stores the DHCP gateway per interface and adjusts route
  metrics on carrier change via a dedicated link worker goroutine.

## Gotchas

- The v6 resolv.conf write initially used an early `return` when the path was
  empty, which would have skipped prefix delegation processing below it. Fixed
  to use a conditional block instead.
- The `TestAllPluginsRegistered` and `TestAvailablePlugins` tests had stale
  expected lists missing `bgp-rr`, `ntp`, and `sysctl`, and assumed `iface-dhcp`
  registers on all platforms (it has `//go:build linux`). Changed to
  platform-aware bidirectional checks.

## Files

- `internal/component/plugin/events.go` -- NamespaceSystem, EventClockSynced
- `internal/plugins/ntp/ntp.go` -- syncWorker eventBus, synced flag, emit
- `internal/plugins/ntp/register.go` -- pass EventBus to syncWorker
- `internal/component/iface/schema/ze-iface-conf.yang` -- resolv-conf-path leaf
- `internal/component/iface/config.go` -- ResolvConfPath field, parsing, validation
- `internal/component/iface/register.go` -- factory signature, threading
- `internal/plugins/ifacedhcp/ifacedhcp.go` -- ResolvConfPath in DHCPConfig
- `internal/plugins/ifacedhcp/resolv_linux.go` -- removed constant and wrapper
- `internal/plugins/ifacedhcp/dhcp_v4_linux.go` -- configurable path
- `internal/plugins/ifacedhcp/dhcp_v6_linux.go` -- configurable path
