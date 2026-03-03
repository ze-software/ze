# 070 — WireUpdate Buffer Lifecycle

## Objective

Replace the confusing "swap" pattern for session read buffers (persistent `readBuf` field + get-next-from-pool) with a clean get-from-pool / process / return-to-pool lifecycle with explicit ownership transfer.

## Decisions

- Two separate pools by size: `readBufPool4K` (4096, pre-OPEN and standard sessions) and `readBufPool64K` (65535, after Extended Message negotiated) — mixed-size pool would either waste memory or add complexity
- `returnReadBuffer` uses `cap(buf)` not `len(buf)` to determine which pool — cap is fixed at allocation, len varies per read
- `extendedMessage bool` on Session is thread-safe: only accessed from the session's single read goroutine, no mutex needed
- Cache uses `Take()` (removes entry) not `Get()` (leaves entry) — prevents race where two concurrent calls both claim the same buffer
- Ownership is exclusive: either session or cache owns the buffer, never both; `kept` return value from process() signals transfer

## Patterns

- Callback fires BEFORE cache.Add() — prevents use-after-free when cache is full (session can't return a buffer the cache has already rejected)
- OPEN messages are always ≤4096 bytes (RFC 4271), so 4K pool is correct pre-negotiation; 64K pool activates only after `negotiate()` sets `extendedMessage = true`

## Gotchas

- Prior swap pattern: `oldBuf := s.readBuf; s.readBuf = pool.Get(); process(oldBuf); pool.Put(oldBuf)` — confusing because pool acted as "next buffer" source rather than a recycler
- `ReceivedUpdate.Announces/Withdraws` were never populated in this implementation; moving to `spec-wireupdate-split.md`

## Files

- `internal/reactor/session.go` — removed `readBuf` field, added `extendedMessage`, `getReadBuffer()`, `returnReadBuffer()`
- `internal/reactor/reactor.go` — `notifyMessageReceiver` returns `kept`, callback before cache
- `internal/reactor/received_update.go` — `poolBuf []byte`, `Release()`
- `internal/reactor/recent_cache.go` — `Get()` → `Take()`, `Contains()`
