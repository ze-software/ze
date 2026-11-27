---
spec: fleet-10-drift-hash-chain
topic: Fleet
feature: fleet-management
---

## Before

A managed device learned of hub configuration changes only when it reconnected or when a hub push notification arrived, and every update transferred the full configuration blob. If a notification was dropped, an out-of-band hub store write happened, or an acknowledgement was lost in flight, the hub and device stayed out of sync without noticing until the next reconnect or edit. Heartbeat pings carried no integrity check and no per-call timeout, and when both incremental repair and full replay failed, operators saw no hub-wide alarm in `show fleet alarm`.

## After

Both the hub and each managed device now fold every applied hub-driven configuration change into a persistent 40-hex SHA-1 hash chain and exchange and compare the chain head on every managed channel reply (`ping`, `config-fetch`, `config-ack`, `config-changed`, and `config-held`). Steady-state 30-second heartbeats carry a 10-second per-call timeout, perform zero store writes when chains agree, and immediately trigger resync on a non-holding device when the hub's chain head disagrees.

Normal hub updates and drift recoveries send a deterministic line-based diff computed from the hub's unpruned versioned configuration history first, reconstructing byte-identical target configurations so comments, `$9$` secrets, `FormatSetMeta`, and external-plugin sections stay intact, followed by full YANG validation and guarded transactional commit on the device. When historical base bytes are absent, the diff is at least as large as the target, or diff verification or validation fails, the protocol automatically falls back once to full configuration replay and re-anchors both chains. Devices holding a local emergency edit (`active` differing from `baseline`) never auto-resync and remain protected in hold until an operator runs `adopt` or `revert`.

Operators can inspect chain state in `show fleet status` (`chain-hash`, `chain-state`, and `last-resync`), track mismatches and diff or full resync outcomes via hub and client Prometheus counters (`ze_managed_chain_mismatches_total`, `ze_managed_chain_resyncs_total`, `ze_managed_client_chain_mismatches_total`, and `ze_managed_client_resyncs_total`), diagnose persistent drift or unreadable chain keys in `ze doctor` (`doctor-managed-chain-drift` and `doctor-managed-chain-unreadable`), and see non-verify double failures estate-wide in `show fleet alarm` as `config-resync-failed` (**Configuration resync failed**, Warning, hidden behind `device-offline` while the device is disconnected and cleared automatically once chains converge again).

## Corrects

none

## Exposure

none

## Upstream

none

## Asked by

none
