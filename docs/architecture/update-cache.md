# UPDATE Cache Architecture

## Purpose

The UPDATE cache stores received BGP UPDATE messages by message-id, enabling route-reflector-style forwarding where plugins can re-send cached wire bytes to peers without re-encoding.
<!-- source: internal/component/bgp/reactor/recent_cache.go -- RecentUpdateCache -->

## Cache Consumer Model

Not all plugins that receive UPDATE events participate in cache lifecycle management. A monitoring plugin may count updates without needing to acknowledge them. The cache distinguishes between:

| Plugin Role | Declaration | Receives UPDATEs | Blocks Eviction | Must Ack |
|-------------|-------------|------------------|-----------------|----------|
| Cache consumer (e.g., RIB, route reflector) | `"cache-consumer": true` | Yes | Yes | Yes |
| Observer (e.g., monitor, counter, logger) | default (`false`) | Yes | No | No |
<!-- source: internal/component/plugin/process/process.go -- IsCacheConsumer/SetCacheConsumer -->
<!-- source: pkg/plugin/rpc/types.go -- CacheConsumer field on DeclareRegistrationInput -->

### Opting In

A plugin declares itself as a cache consumer during Stage 1 registration:

```json
{
  "method": "ze-plugin-engine:declare-registration",
  "params": {
    "families": [...],
    "commands": [...],
    "cache-consumer": true
  }
}
```

SDK usage:

```go
p.Run(ctx, sdk.Registration{
    CacheConsumer: true,
})
```
<!-- source: internal/component/plugin/server/server.go -- Stage 1 reads cache-consumer from registration -->

This is a permanent property set once at startup. The engine stores it on the process and uses it to filter which plugin deliveries are counted for `Activate()`.

### Delivery vs Tracking

When an UPDATE arrives, the engine delivers the event to **all** subscribed plugins. But only cache consumers are counted for the `Activate()` call. Observers receive the event as fire-and-forget.

```
UPDATE received
  |
  v
Deliver to ALL subscribed plugins (observer + consumer)
  |
  v
Count successful deliveries WHERE IsCacheConsumer() = true
  |
  v
Activate(msg-id, consumerCount)
```

## Entry Lifecycle

```
Add(update)              — insert with pending=true, pendingConsumers=0
     |
Activate(id, count)      — set pendingConsumers = count - earlyAcks, clear pending
     |
  [plugins process]      — each consumer forwards or releases via Ack(id, plugin)
     |
All consumers done       — entry evicted, buffer returned to pool
  + retainCount = 0
```
<!-- source: internal/component/bgp/reactor/recent_cache.go -- Add, Activate, Ack, evictLocked -->

### States

| State | Meaning |
|-------|---------|
| Pending | Added but not yet activated (between Add and Activate) |
| Active | Consumer count set, waiting for acks |
| Retained | API-level hold (retainCount > 0), independent of consumer count |
| Evicted | All consumers acked + retainCount = 0, buffer returned |

## Consumer Tracking

Two independent tracking layers:

| Layer | Incremented By | Decremented By | Purpose |
|-------|---------------|----------------|---------|
| `pendingConsumers` (count) | `Activate(id, count)` | `Ack(id, plugin)` from cache consumer | Mandatory ack tracking |
| `retainCount` (API-level) | `retain` command | `release` command (non-plugin context) | Explicit hold |
<!-- source: internal/component/bgp/reactor/recent_cache.go -- cacheEntry struct -->

Entry evicted when: `pendingConsumers == 0 AND retainCount <= 0 AND !pending`

### Early Ack Handling

Fast plugins may ack before `Activate()` is called. The `earlyAckCount` field tracks these:
- Pre-Activate ack increments `earlyAckCount`
- `Activate()` computes: `pendingConsumers = max(0, count - earlyAckCount)`
- Prevents entries from getting stuck when plugins are faster than the activation path

### Consumer Registration / Unregistration

- `RegisterConsumer(name)` — initializes FIFO tracking, sets last-ack to highest-added-id (avoids implicit acks on pre-existing entries)
- `UnregisterConsumer(name)` — decrements `pendingConsumers` on all un-acked entries for that plugin, evicts entries that reach zero
<!-- source: internal/component/bgp/reactor/recent_cache.go -- RegisterConsumer, UnregisterConsumer -->

## Commands

| Command | Description |
|---------|-------------|
| `bgp cache list` | List cached message IDs and count |
| `bgp cache <id> retain` | Increment retain count (prevents eviction) |
| `bgp cache <id> release` | Cache consumer: ack without forwarding. Otherwise: decrement retain count |
| `bgp cache <id> expire` | Admin override: force-remove immediately |
| `bgp cache <id> forward <sel>` | Forward wire bytes to matching peers, then record ack |

### `release` Dual Purpose

The `release` command behaves differently based on the caller:

1. **Cache consumer plugin**: Acts as acknowledgment — decrements pendingConsumers. FIFO ordering enforced.
2. **Non-consumer or external CLI**: Only decrements retain count.

Determined by checking whether the calling process has `cache-consumer: true`.

### `forward`

Forwards the cached UPDATE's wire bytes to peers matching the selector (zero-copy when encoding contexts match), then records the calling plugin as having acked this entry. FIFO ordering enforced per plugin.

## FIFO Ordering

Plugin acks must follow receive order. If a plugin receives updates 100, 101, 102:

- Acking 102 implicitly acks 100 and 101 for that plugin (cumulative, TCP-like)
- Acking 100 after already acking 102 returns `FIFO violation` error

Per-plugin FIFO tracked via `pluginLastAck map[string]uint64`.
<!-- source: internal/component/bgp/reactor/recent_cache.go -- Ack FIFO enforcement -->

## Safety Valve

Detects crashed or stuck plugins. If an entry still has un-acked consumers, but a **later** entry has been fully acked by all plugins, the older entry is considered "passed over."

| Parameter | Value |
|-----------|-------|
| Gap scan interval | 30 seconds |
| Safety valve timeout | 5 minutes |

Entries at the processing frontier (no later entry fully acked) are never timed out — the plugin may just be slow.

## Soft Max-Entries Limit

The cache has a configurable soft limit (`RecentUpdateMax`). When exceeded, a rate-limited warning is logged (every 30s) but entries are never rejected. This prevents back-pressure from affecting UPDATE processing while alerting operators to capacity issues.

Configured via: `environment { reactor { cache-max N; } }` or `ze_reactor_cache_max=N`.

## Key Files

| File | Purpose |
|------|---------|
| `internal/component/bgp/reactor/recent_cache.go` | Cache implementation (Add, Activate, Ack, eviction) |
| `internal/component/bgp/plugins/cmd/cache/` | Command dispatch (list, retain, release) |
| `internal/component/bgp/server/events.go` | Event delivery + cache consumer filtering |
| `internal/component/plugin/process/process.go` | `IsCacheConsumer()` / `SetCacheConsumer()` on Process |
| `internal/component/plugin/server/server.go` | Stage 1 reads `cache-consumer` from registration |
| `pkg/plugin/rpc/types.go` | `CacheConsumer` field on `DeclareRegistrationInput` |
