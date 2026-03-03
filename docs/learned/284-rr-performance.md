# 284 — Route Reflector Forwarding Performance

## Objective

Eliminate pause/resume oscillation and reduce per-UPDATE syscall cost in `bgp-rr` by widening backpressure thresholds, batching forward RPCs, making them fire-and-forget, and splitting JSON parsing into two levels.

## Decisions

- Channel capacity 1024→4096 and thresholds 75%/25%→90%/10%: the narrow 50-point band caused rapid oscillation; 80-point band combined with 4× buffer dramatically reduces pause/resume cycles.
- Batch accumulation flush triggers: full (50 items), timeout (1ms), selector change, worker idle — chosen to keep latency low while amortizing RPC overhead.
- Fire-and-forget via buffered channel (capacity 16 batches) not goroutine per call — backpressure is preserved: channel full → worker blocks, which is correct flow control.
- Two-level JSON parsing: `parseUpdateFamilies` extracts only map keys for target selection; full NLRI parse deferred to RIB path. Chose `json.RawMessage` values to avoid second unmarshal allocation until needed.
- Deferred RIB after forward is safe because `PeerDown()` drains the worker queue before `ClearPeer` — all deferred inserts complete before peer-down withdrawal generation.
- Two functional tests skipped (with approval): batch accumulation and async forward are internal optimizations invisible on the BGP wire; 15 unit tests cover the behavior adequately.

## Patterns

- Batch command parsing: detect comma in the ID argument (`strings.SplitSeq`) → loop calling existing single-ID handler per ID. No new handler needed.
- Async release uses same channel + background goroutine pattern as async forward — idempotent operation, eventual consistency acceptable.
- `envelopeTypeBGP` constant extracted to satisfy `goconst` linter — recurring pattern: extract string literals used 3+ times.

## Gotchas

- `buildTestUpdate` always used the same prefix "10.0.0.0/24" — RIB deduplicates to 1 entry. Tests for deferred RIB consistency required unique prefixes per item.
- `flushWorkers` in tests needed to drain the forward loop (`stopForwardLoop` + `startForwardLoop`) after `workers.Stop()`, otherwise async forward RPCs were not yet processed when assertions ran.
- `nilerr` linter: `parseUpdateFamilies` was returning nil error on failed unmarshal. Always propagate wrapped errors from `json.Unmarshal`.

## Files

- `internal/plugins/bgp-rr/worker.go` — capacity, thresholds
- `internal/plugins/bgp-rr/server.go` — pre-parsed payload, deferred RIB, async release, batch accumulation, async forward, two-level parse
- `internal/plugins/bgp/handler/cache.go` — comma-separated batch command parsing
- `docs/architecture/api/commands.md` — batch forward/release syntax documented
