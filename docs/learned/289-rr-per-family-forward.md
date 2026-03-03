# 289 — Route Reflector Per-Family Forwarding (Worker Pool)

## Objective

Replace bgp-rr's synchronous single-goroutine OnEvent dispatcher with a per-source-peer worker pool, enabling cross-peer parallelism while preserving FIFO ordering within each peer's UPDATE stream.

## Decisions

- Per-source-peer granularity (not per-destination-peer, not per-(source,family)) — the source peer determines the UPDATE stream; FIFO ordering within a peer is mandatory for correctness (announce then withdraw for the same prefix must stay ordered).
- Blocking send with `stopCh` escape: every cached UPDATE must be forwarded or released (CacheConsumer protocol); non-blocking drop silently violates the cache contract and leaks entries; `stopCh` prevents deadlock during shutdown.
- `sync.Map` keyed by msgID for `forwardCtx` instead of a struct with families/release fields — lint blocked unused struct fields during incremental development; `sync.Map` is cleaner for this pattern.
- Lightweight family-only events, engine-side decoder goroutine, and on-demand decode all deferred — not needed for correctness; plugin-side parallelism was the priority.
- Three `go func()` at state-down/up/refresh kept as-is — these are per-lifecycle goroutines (one per session event), not per-event. Compliant with goroutine-lifecycle rule.
- Per-family splitting for partial-match multi-family UPDATEs documented as a known limitation — the engine's `ForwardUpdate` does not filter by family; full UPDATE is forwarded to any peer supporting at least one family.

## Patterns

- `quickParseEvent` in the dispatcher for lightweight envelope extraction (type, msgID, peerAddr only); full parse deferred to worker goroutine.
- Idle cooldown with race prevention: worker exits after 5s idle, checks `pending == 0` before self-removal to avoid racing with a new dispatch.
- `PeerDown` closes the worker channel and waits for drain — ensures all deferred RIB operations complete before peer-down withdrawal generation.

## Gotchas

- Incremental edits with unused struct fields → lint hook blocks every edit with compile errors. Solution: write the complete file in one pass with the Write tool when adding a new struct with fields that are referenced later.
- `ForwardUpdate` in the engine does NOT do per-family wire splitting — it sends the entire cached UPDATE to all listed peers. A peer receiving an unnegotiated family NLRI may NOTIFICATION (RFC 4271) or silently discard (RFC 7606). This is the known multi-family limitation.
- MP_REACH and MP_UNREACH can carry different address families in the same UPDATE (RFC 4760 allows it) — a single UPDATE can contain up to 3 distinct families.

## Files

- `internal/plugins/bgp-rr/worker.go` — lazy per-source-peer goroutines, backpressure, peer-down drain
- `internal/plugins/bgp-rr/rib.go` — `peerRIB` type with per-peer `sync.Mutex`
- `internal/plugins/bgp-rr/server.go` — thin `dispatch()` dispatcher, `processForward` in workers
