---
spec: fleet-8-metric-history
topic: Fleet
feature: fleet-management
---

## Before

Managed devices could push one-shot hardware inventory and component health snapshots to their hub, but the hub kept no time-series metric history for its fleet. Checking CPU, memory, storage, load, uptime, interface counts and traffic rates, BGP peer and route counts, or subscriber session totals over time — or ranking devices and aggregating readings across label groups — required connecting to each device individually or querying an external Prometheus deployment.

## After

Managed devices now push a fixed set of 15 operational figures (`cpu-used`, `memory-used`, `memory-total`, `storage-used`, `storage-total`, `load-1m`, `uptime`, `interfaces-up`, `interfaces-configured`, `traffic-in`, `traffic-out`, `bgp-peers-established`, `bgp-peers-configured`, `routes-held`, and `subscriber-sessions`) over their existing authenticated TLS hub session using `sample-report`. The hub advertises its configured `bus hub history interval` (default 60 seconds, `0` to disable) in `config-fetch` replies, admits at most one report per device per UTC interval across reconnects, and retains both a 1-minute ring for 1 day and a 15-minute ring for up to 7 days (`bus hub history retention-days`) with per-minute aligned peaks in a bounded, zlib-compressed persistent store (`device-budget-bytes` default 1 MiB, `hub-budget-bytes` default 2 GiB, checkpointed every 5 minutes and flushed cleanly on shutdown).

Three read-only commands expose the history identically across CLI, SSH, MCP, REST, gRPC, and the web command line, with caller-scoped local device visibility (never issuing per-device remote TACACS+ lookups): `show fleet sample latest` (with `device`, `label`, `figure`, `reduce`, and `rank` pipe filters), `show fleet sample series` (with `device`, `label`, `figure`, `from`, `until`, `reduce`, and `points` pipe filters), and `show fleet sample storage`. Devices that have not reported appear explicitly as `no-reports`, optional compiled-out or unconfigured producers are omitted rather than zero-filled, offline intervals retain the last accepted reading while leaving genuine gaps in series, budget pressure shortens retention oldest-first while raising `doctor-managed-history-capacity-pressure`, checkpoint failures raise `doctor-managed-history-checkpoint-failed`, corrupt shards raise `doctor-managed-history-corrupt`, undeclaring a device preserves its history, and deleting `meta/fleet/device/<name>/record` fences checkpoint writers and erases its metric history durably across hub restart. Standalone devices start no sampler and incur zero sampling overhead; clients talking to an older hub that does not advertise an interval or answers `unknown method: sample-report` leave sampling disabled for that session while continuing configuration sync.

## Corrects

none

## Exposure

none

## Upstream

none

## Asked by

none
