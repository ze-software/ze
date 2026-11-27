---
spec: fleet-ui-5-alarm-list
topic: Fleet
feature: fleet-management
---

## Before

A fleet hub tracked device sessions, configuration state, hardware inventory, component health, and metric history separately across individual commands, but it kept no unified alarm list and no durable history of when conditions opened or cleared. Operators had to inspect devices one by one (`show fleet devices`, `show fleet device health`, `show fleet sample latest`, `show fleet divergence`, and `show doctor`) to learn what needed attention across the estate, and transient conditions that opened and cleared between manual checks left no record.

## After

The hub now derives open fleet alarms continuously from declared devices, authenticated sessions, component health reports, held and rejected configuration state, metric samples, inventory software versions, and metric history capacity, exposing eight built-in kinds (`device-offline`, `component-down`, `component-degraded`, `config-diverged`, `config-rejected`, `resource-high`, `software-behind`, and `history-full`) alongside an open kind registration seam for future conditions. Alarm delays, CPU/memory/storage utilization thresholds and sustained duration, version lag threshold, and closed-episode retention caps (`retention-days`, `max-episodes-per-device`, `max-episodes-hub`) are configured under `bus { hub { alarms { ... } } }` and apply live on reload without restarting the hub.

Two read-only commands expose the alarm model identically across CLI, SSH, MCP, REST, gRPC, and the web command line, with caller-scoped local device visibility and snapshot-bound continuation cursors: `show fleet alarm` (supporting `severity`, `kind`, `device`, `cursor`, and `limit` pipe filters, returning up to 1,024 rows per page plus complete authorized totals and severity/visibility counts across all pages) and `show fleet alarm history device <name>` (supporting `cursor` and `limit` pipe filters, returning up to 512 events per page with `retained-since` and `truncated` indicators). Each occurrence carries a stable monotonic identifier and opening timestamp preserved across repeat reports, reads, disconnects, and hub restarts; never-connected declared devices count `offline-delay` from declaration across reloads and restarts; open `resource-high` occurrences remain open while offline, close on reconnect, and require fresh sustained post-reconnect samples spanning `resource-duration` to reopen; and shard persistence or startup decode failures keep live alarms available while reporting degraded durability and raising `doctor-managed-alarm-write-failed` or `doctor-managed-alarm-corrupt` in `show doctor`.

## Corrects

none

## Exposure

none

## Upstream

none

## Asked by

none
