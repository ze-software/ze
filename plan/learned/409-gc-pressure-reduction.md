# 401 -- GC Pressure Reduction on Event Dispatch Hot Path

## Objective

Reduce per-UPDATE heap allocations on the event dispatch hot path to lower GC pressure,
targeting the three highest-frequency allocation sites in `events.go`.

## Decisions

- **StructuredUpdate pool:** Used `sync.Pool` for `*rpc.StructuredUpdate` objects that escape
  to heap via the `any` interface field in `EventDelivery`. Pooled objects are returned only
  after all result channels have been drained -- returning earlier would race with consumer
  goroutines still reading the pointer through the delivery channel.
- **Stack-based format cache:** Replaced `make(map[string]string, 2)` with a fixed [4]-slot
  struct (`formatCache`). The key space is 1-2 entries in practice (format+encoding combinations).
  Linear scan over 4 slots is faster than map hash+bucket for this size. The struct lives on the
  goroutine stack and never touches the heap.
- **Precomputed format cache key:** Added `FormatCacheKey()` to `Process`, recomputed by
  `SetFormat()` and `SetEncoding()`. Eliminates per-event `proc.Format() + "+" + proc.Encoding()`
  string concatenation. Format and encoding are set once during the 5-stage startup and rarely
  change, so recomputation cost is negligible.

## Patterns

- **Pool return timing matters.** When a pooled pointer is stored in a channel-sent struct,
  it must not be returned to the pool until all consumers have finished. The safe point is
  after draining the results channel (all deliveries complete). A fixed-size `[4]*T` array on
  the stack tracks pooled pointers without additional allocation.
- **Stack arrays beat sync.Pool for function-scoped caches.** If the cached data is created and
  consumed within a single function call (no cross-goroutine sharing), a stack array is strictly
  cheaper than sync.Pool. Pool's Get/Put involve atomic operations and GC interaction; a stack
  array is free.
- **Precompute derived strings at set-time, not at use-time.** When two values are set rarely
  but their concatenation is read on every event, store the concatenation as a third field
  and update it in the setters. `atomic.Value` makes this safe for concurrent readers.
- **Go maps allocate even with size hints.** `make(map[K]V, 2)` still allocates bucket memory
  on the heap. For tiny key spaces (1-4 entries), a fixed-size array with linear scan is
  both faster and allocation-free.

## Gotchas

- **Linter requires checked type assertions on sync.Pool.Get().** `pool.Get().(*T)` triggers
  errcheck. Must use `v, ok := pool.Get().(*T)` with fallback.
- **Pool.Put must clear pointer fields.** Without clearing `Event` and `PeerAddress`, the pool
  would keep references alive, defeating GC for the pointed-to objects. Always nil out pointer
  and string fields before Put.
- **formatCache.set silently drops if full.** With 4 slots and typical usage of 1-2, overflow
  should never happen. If it does, the caller falls through to format on the spot -- correctness
  is preserved, only the caching benefit is lost. No error, no panic.

## Files

- `internal/component/bgp/server/events.go` -- formatCache type, StructuredUpdate pool,
  all 7 event dispatch functions updated
- `internal/component/plugin/process/process.go` -- formatCacheKey field, FormatCacheKey(),
  recomputeFormatCacheKey(), updated SetFormat/SetEncoding
