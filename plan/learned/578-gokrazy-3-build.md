# 578 -- gokrazy-3: Build Config

## Context

The gokrazy appliance image included gokrazy's built-in DHCP and NTP packages by
default. After ze gained its own DHCP wiring (spec-gokrazy-1) and NTP plugin
(spec-gokrazy-2), the appliance needed to exclude those packages so ze owns all
network configuration and time synchronization. Without this change, two DHCP
clients and two NTP clients would fight for the same resources.

## Decisions

- Explicit `GokrazyPackages` with only `randomd` and `heartbeat` over relying on
  gokrazy defaults -- makes the exclusion visible and intentional.
- Removed `WaitForClock: true` from ze's package config -- ze owns the clock via
  its NTP plugin, so blocking on gokrazy's clock is circular.
- Seed config uses `dhcp-auto true` over explicit interface naming -- gokrazy
  images run on unknown hardware, interface names vary.
- `environment ntp enabled false` in seed config -- NTP is disabled by default in
  the YANG schema, the seed config makes this explicit for documentation.

## Consequences

- gokrazy images built with `make ze-gokrazy` no longer contain `cmd/dhcp` or
  `cmd/ntp` from gokrazy.
- Ze acquires IP via DHCP and syncs clock via NTP on boot, single control plane.
- `docs/guide/appliance.md` updated to reflect ze-owned DHCP/NTP.
- TLS certificate caching added per hostname across rebuilds (`gokrazy/ze/builddir/`).

## Gotchas

- The Makefile comment at line 519 still mentioned gokrazy providing DHCP/NTP
  at the time of review. The code was correct but the comment was stale.
- `gokrazy/ze/ze.conf` seed config is written by the Makefile during build, not
  read from ExtraFileContents in config.json.

## Files

- `gokrazy/ze/config.json` -- GokrazyPackages, removed WaitForClock
- `gokrazy/ze/ze.conf` -- seed config with dhcp-auto and ntp disabled
- `docs/guide/appliance.md` -- updated for ze-owned DHCP/NTP
