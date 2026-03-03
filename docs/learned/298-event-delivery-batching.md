# 298 — Event Delivery Batching

## Objective

Reduce debug log noise and improve event delivery performance by consolidating redundant log lines and batching received UPDATEs at the peer delivery goroutine level so subscription lookup, JSON formatting, and result collection happen once per batch.

## Decisions

- Batching at peer level (not server level) — all items in a batch share the same source peer, therefore same subscription matches; drain pattern proven by `process.drainBatch`
- `OnMessageBatchReceived` returns `[]int` (per-message cacheCount) — `Activate(msgID, cacheCount)` is per-message; a batch-level count would break the "ack before activate" correction where early acks are tracked per-message
- `OnMessageBatchReceived` uses `[]any` at the interface boundary (not `[]bgptypes.RawMessage`) — matches existing `any` convention in `MessageReceiver` and `BGPHooks`; type assertion happens in hooks.go closure
- `TestSendUpdateSingleDebugLog` not implemented — testing slog output requires capturing LazyLogger output which is fragile; AC verified by code review instead
- Sent-direction events unaffected — come from different peer goroutines, naturally can't batch

## Patterns

- Pre-format dedup: format map built once per batch (same format modes apply to all messages from same peer)
- Drain pattern from deliverChan mirrors `process.drainBatch` exactly

## Gotchas

- None recorded.

## Files

- `internal/plugins/bgp/reactor/delivery.go` — `drainDeliveryBatch`
- `internal/plugins/bgp/reactor/peer.go` — updated delivery goroutine to batch-drain
- `internal/plugins/bgp/reactor/reactor.go` — `OnMessageBatchReceived` added to `MessageReceiver` interface
- `internal/plugin/server_events.go` — `Server.OnMessageBatchReceived`
- `internal/plugin/types.go` — `OnMessageBatchReceived` in `BGPHooks`
- `internal/plugins/bgp/server/events.go` — `onMessageBatchReceived`, removed per-process "writing" debug logs
- `internal/plugin/subscribe.go` — removed `GetMatching` debug log
- `internal/plugins/bgp/reactor/session.go` — merged two SendUpdate debug lines into one
