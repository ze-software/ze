# 580 -- gokrazy-0: Own DHCP and NTP (Umbrella)

## Context

Ze runs as a gokrazy appliance but relied on gokrazy's built-in DHCP and NTP
packages for network configuration and clock synchronization. This meant two
control planes for the same resources, no config-driven DHCP behavior, and no
way to use DHCP-discovered NTP servers. The goal was a single control plane
where ze owns DHCP, DNS resolver config, NTP, and the gokrazy image excludes
the default packages.

## Decisions

- Four child specs over a monolithic implementation -- DHCP wiring (1), NTP
  plugin (2), build config (3), and resilience (4) are independent concerns.
- ifacedhcp stays as a separate plugin from the interface component -- it has
  its own lifecycle (DORA/SARR workers) and depends on the interface plugin
  via the factory callback pattern.
- NTP as an in-process plugin with `beevik/ntp` library over a managed external
  process -- ze owns the clock directly, no IPC overhead.
- Configurable route priority moved to a standalone iface spec (`spec-iface-route-priority`)
  -- not gokrazy-specific, useful for any multi-uplink setup.
- DNS conflict detection replaced by configurable `resolv-conf-path` -- simpler,
  operator chooses the path or disables writes entirely.

## Consequences

- Ze is now a self-contained gokrazy appliance: DHCP, DNS, NTP, and BGP all
  flow through ze's config pipeline.
- The `system` event namespace exists for cross-cutting system events. Currently
  only `clock-synced`; ready for future events (disk readiness, etc.).
- Backend interface has `AddRoute`/`RemoveRoute` with metric support -- any
  future route management uses these.
- Three new dependencies: `beevik/ntp` (NTP queries), `insomniacslk/dhcp`
  (already used), no new third-party imports beyond what spec-gokrazy-2 added.

## Gotchas

- The umbrella spec and child spec 3 were already implemented in code but never
  closed with learned summaries. This created confusion about what was "done."
- The `TestAllPluginsRegistered` test broke because `ntp` and `bgp-rr` were
  registered but not in the expected list, and `iface-dhcp` was in the list but
  doesn't register on Darwin. Platform-aware tests were needed.
- gokrazy's `WaitForClock: true` had to be removed from config.json since ze
  owns the clock. Leaving it would cause a boot hang (ze waits for clock that
  only ze can set).

## Files

- See child learned summaries: 576 (DHCP wiring), 577 (NTP), 578 (build), 579 (resilience)
- `plan/spec-iface-route-priority.md` -- deferred configurable route priority
