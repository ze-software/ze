# Week of 2026-01-12

A week focused on Graceful Restart, ExaBGP migration tooling, and logging.

## 🛰️ Graceful Restart

A dedicated Graceful Restart plugin now handles per-peer restart-time configuration and registers the GR capability with peers during session startup, building on the plugin system's new per-peer capability support.

## 🔀 ExaBGP migration

For anyone running ExaBGP today, Ze picked up a migration path:

- `zebgp exabgp migrate` converts ExaBGP configs to Ze's format (neighbor -> peer syntax, capability normalization, automatic wiring of the RIB plugin when graceful-restart or route-refresh is in use)
- `zebgp exabgp plugin` runs existing ExaBGP process plugins under Ze via a bridge that translates JSON and commands both ways
- The bridge forwards negotiated capabilities and next-hop information to those plugins, including RFC 8950 extended next-hop handling

## 📊 Logging

Per-subsystem logging landed, with `SLOG_LEVEL` to control verbosity and debug logging available where needed.

## 🧩 Config & API

- The text-based update API now covers end-of-RIB markers, VPLS, and EVPN routes
- EVPN Type 5 routes support the RFC 9136 gateway keyword for overlay index resolution
- Plugin startup timeouts are now configurable per plugin
- Config validation now catches route-refresh or graceful-restart enabled without a process to resend routes, instead of failing silently later
