---
spec: bus-observer-delivery-decoupling
topic: Plugins
---

## Before

Each bus endpoint delivered events through a fixed 64-slot queue and sent at
most 64 events per RPC batch. During a full-table BGP burst, slow or numerous
external observer endpoints (such as `ze exabgp plugin` scripts subscribing to
`bgp:update`) filled their 64-slot queues almost immediately, stalling the
shared event delivery pipeline and throttling live BGP route forwarding to
other plugins. There was no configuration setting to adjust endpoint queue
headroom, and no Prometheus metric or `ze doctor` warning showed when a slow
endpoint forced producers to wait at its queue ceiling.

## After

Each bus endpoint now uses a bounded, dynamically growable, lossless FIFO
delivery queue that starts at 64 slots, grows on demand up to 65,536 events
for ordinary endpoints (256 events for cache-consumer forwarding plugins such
as `bgp-rs` and `bgp-rr`), drains up to 512 events per RPC batch, and shrinks
back to 64 slots once drained empty. Events are never dropped; producers wait
only if an endpoint reaches its queue ceiling. Operators can tune the ceiling
per endpoint with `bus { endpoint <name> { delivery-queue-max <64..1048576>; } }`
(requiring a restart on change), monitor live backlog and ceiling stalls via
the `ze_plugin_delivery_queue_depth` and
`ze_plugin_delivery_backpressure_waits_total` Prometheus metrics, and see
recent queue-ceiling backpressure waits reported by `ze doctor`.

## Corrects

none

## Exposure

none

## Upstream

none

## Asked by

none
