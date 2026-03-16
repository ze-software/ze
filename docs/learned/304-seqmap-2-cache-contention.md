# 304 — seqmap-2-cache-contention

## Objective

Reduce lock contention in `RecentUpdateCache` by moving the gap-based safety valve scan to a background goroutine and replacing the integer-probing cumulative ack loop with `seqmap.Since()`.

## Decisions

- Gap scan moved from inline in `Add()` to `gapScanLoop` background goroutine on a ticker — decouples fault detection (slow, infrequent; never fires in normal operation) from the hot path (fast, per-UPDATE)
- `SetGapScanInterval()` added for test configurability — avoids sleeping in tests, uses 1ms ticker via FakeClock
- `Stop()` uses `sync.Once` for idempotent shutdown — `defer cache.Stop()` is safe even without a preceding `Start()`
- Cumulative ack uses collect-then-ack pattern with `Since()`: collect IDs to ack under lock, then ack them — avoids holding lock during iteration while also acking
- `lastGapScan` field removed — no longer needed with ticker-based background goroutine

## Patterns

- Message IDs are global across ALL message types (OPEN, KEEPALIVE, UPDATE) but only UPDATEs are cached — the integer-probing cumulative ack loop was wasteful by design; `seqmap.Since()` eliminates this structurally
- Cache lifecycle: `NewRecentUpdateCache()` + `Start()` + deferred `Stop()` — all existing tests needed `defer cache.Stop()` added
- Gap-scan-dependent existing tests call `cache.runGapScan()` directly instead of relying on ticker

## Gotchas

- `recent_cache.go` is 615 lines (above 600 monitor threshold) but is a single concern — acceptable per file-modularity rules

## Files

- `internal/component/bgp/reactor/recent_cache.go` — seqmap entries, background goroutine, Start/Stop, Since-based ack
- `internal/component/bgp/reactor/recent_cache_test.go` — 8 new tests, defer Stop to all existing tests
- `internal/component/bgp/reactor/reactor.go` — cache.Start() in StartWithContext, cache.Stop() in cleanup
