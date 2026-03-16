# 317 — Refcounted Cache (Two-Phase)

## Objective

Replace the boolean `retained` flag and TTL-based eviction in the BGP UPDATE cache with reference-counted entries (Phase 1) and mandatory consumer acknowledgment with FIFO ordering (Phase 2), eliminating UPDATE loss in the route reflector under slow plugin conditions.

## Decisions

- Cache insertion must happen BEFORE event dispatch — the window between dispatch and insertion is the root cause of "plugin has msg-id, cache doesn't have entry yet."
- Negative consumer counts allowed during the `Activate()` race (fast plugin calls `Decrement` before `Activate`) — avoids the need for a lock between dispatch and activation; `Activate(id, N)` yields the correct net count.
- Soft limit (always accept, warn at capacity) over hard rejection — dropping UPDATE messages is always worse than using more memory in a route reflector.
- TTL is fundamentally wrong for a route reflector: time-based eviction means "silently drop if plugin is slow." The only valid eviction trigger is explicit consumer acknowledgment.
- Out-of-order acks are silent no-ops (not rejections) — multi-peer delivery causes non-ID-ordered events; rejection would break real workloads. Changed from original plan.
- Gap-based safety valve (background goroutine) instead of wall-clock timeout — fires only when a later entry is fully acked but an earlier one isn't, distinguishing "slow but making progress" from "skipped or stuck."
- Per-entry tracking uses integer counter (`pendingConsumers int`) not a name set — the cache doesn't need to know WHO per-entry, only how many remain. Earlier attempt with per-entry name set caused a bug where acking with `""` polluted `pluginLastAck`.
- `SetConsumerUnordered()` added for bgp-rs per-source-peer workers that process entries out of global message ID order — unordered consumers get per-entry acks rather than cumulative ack via `seqmap.Since()`.
- Separate `drop` command dropped — `release` (ack without forwarding) serves the same purpose.

## Patterns

- `RegisterConsumer(name)` initializes `pluginLastAck[name] = highestAddedID` — new consumers skip pre-registration entries automatically.
- Cumulative ack mirrors TCP ACK: acking message N implicitly acks all messages up to N for that plugin.
- Immediate eviction in `Ack()` (O(1)) vs. cleanup scan only for gap detection (rare fault path).

## Gotchas

- Phase 2 required `seqmap.Since()` for cumulative ack sweep — understanding this data structure is prerequisite for understanding the unregistration path.
- `Ack()` after an entry is already evicted returns `ErrUpdateExpired` — callers must handle this gracefully.

## Files

- `internal/component/bgp/reactor/recent_cache.go` — core implementation (both phases)
- `internal/component/bgp/reactor/reactor_api_forward.go` — `ForwardUpdate` deferred `Ack()`, `ReleaseUpdate`, consumer registration hooks
- `internal/component/bgp/server/events.go` — returns cache-consumer count (not total subscriber count)
- `pkg/plugin/rpc/types.go` — `CacheConsumer` and `CacheConsumerUnordered` registration fields
- `pkg/plugin/rpc/text.go` — `cache-consumer` and `cache-consumer-unordered` text protocol keywords
