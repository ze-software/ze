---
spec: fleet-3-audit-trail
topic: Fleet
feature: fleet-management
---

## Before

A Ze hub recorded no centralized history of fleet operations across managed devices. Operators investigating which device connected or disconnected, when a configuration was pushed, fetched, applied, rejected, or held, who resolved a divergence, or when a disconnected device left and rejoined the fleet had to inspect individual devices manually and had no hub-wide audit view.

## After

Ze hubs now maintain a centralized, append-only fleet audit log in their live state store. Every significant fleet event is recorded with a hub-assigned monotonic sequence number, UTC event and receipt timestamps, event type (`device.connect`, `device.disconnect`, `config.fetch`, `config.push`, `config.ack`, `config.reject`, `config.held`, `divergence.adopt`, `divergence.revert`, `fleet.disable`, `fleet.enable`, and `fleet.receipt.gap`), device name, authenticated actor and surface, outcome, version references, and safe bounded metadata without ever storing raw configuration text or secrets.

Entries are written asynchronously through a bounded non-blocking queue into immutable pages with atomic manifest publication, so storage errors or bursts never stall configuration delivery and retained history survives hub restarts in monotonic sequence order. Operators can configure retention under `bus hub audit max-entries` (100 to 100,000 entries, default 10,000), updated live on reload, while direct live mutation of fleet audit and receipt keys is refused while the daemon runs.

Devices that run `request fleet disable` or `request fleet enable` while disconnected record each mode transition in a bounded local receipt outbox guarded by a recoverable write intent and replay batches to the hub on the next authenticated reconnect with automatic deduplication across lost acknowledgements. Operators query the log across CLI, SSH, REST, gRPC, MCP, and the web command line with `show fleet audit`, narrowing rows with `device`, `type`, `since`, and `until` pipes under per-device authorization, and monitor persistence failures, queue drops, corrupt audit keys, or device receipt gaps in `show doctor` (`doctor-managed-audit-write-failed`, `doctor-managed-audit-dropped`, `doctor-managed-audit-corrupt`, and `doctor-managed-audit-receipt-gap`).

## Corrects

none

## Exposure

none

## Upstream

none

## Asked by

none
