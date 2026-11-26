---
spec: fleet-4-inventory-health
topic: Fleet
feature: fleet-management
---

## Before

Each managed device could report its own hardware inventory and component health locally, but none of that data reached its management hub. Checking which devices had ECC memory errors, degraded BGP sessions, or specific hardware required connecting to each device individually, and a disconnected device's last health or hardware state was unavailable from the hub.

## After

Managed clients automatically push bounded hardware inventory summaries (`inventory-report`, on connect after the initial configuration fetch and every 24 hours; excluding raw SMART readings and block-device serials, with per-section availability flags and optional ECC counters) and component health reports (`health-report`, on connect and every 5 minutes, matching `/health`) to their hub over the existing authenticated TLS connection. The hub enforces a 64 KiB body bound and per-connection rate limits (24 hours for inventory, 1 minute for health), persists the latest accepted snapshots with UTC receipt timestamps across disconnects and restarts, and exposes `show fleet device inventory name <name>`, `show fleet device health name <name>`, and `show fleet health` on every command surface (CLI, SSH, MCP, REST, gRPC, and the web command line). Unreported devices appear explicitly as `not-reported` and are counted separately in `show fleet health`, while disconnected devices retain their last accepted snapshot alongside `connected: false`. Clients talking to an older hub that answers `unknown method` disable only that report kind for the current connection and continue configuration synchronization and heartbeats; collection or send failures raise `doctor-managed-report-failed` on the report bus and clear on recovery.

## Corrects

none

## Exposure

none

## Upstream

none

## Asked by

none
