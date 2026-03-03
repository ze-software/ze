# 303 — seqmap

## Objective

Create a generic sequence-indexed map library and integrate it into adj-rib-in to make delta replays O(log N + K) instead of O(N), where N = total routes and K = matching entries.

## Decisions

- seqmap uses an append-only log (sorted by sequence number) plus a regular map for O(1) key lookup — binary search on the log gives O(log N) start point for `Since()`
- Auto-compaction threshold: dead entries > len/2 AND len > 256 — balances memory vs CPU; below 256 entries compaction cost exceeds benefit
- `SeqIndex` removed from `RawRoute` struct — sequence numbers live only in seqmap, eliminating data duplication
- Per-peer seqmaps with a global seq counter: enables cross-peer range queries with a single `fromIndex` without needing to merge per-peer logs

## Patterns

- seqmap API: `Put(key, seq, val)`, `Get(key)`, `Delete(key)`, `Since(fromSeq, fn)`, `Range(fn)`, `Clear()`, `Len()` — mirrors standard map operations but adds sequence-based iteration
- `Since()` accepts a callback returning bool; returning false stops iteration (early exit for ack cumulative loops)
- Per-peer seqmaps are created fresh on peer-up and dropped entirely on peer-down (`delete(r.ribIn, peer)`)

## Gotchas

- None.

## Files

- `internal/seqmap/seqmap.go` — generic Map[K,V] with append-only log, binary search Since, auto-compaction
- `internal/seqmap/seqmap_test.go` — 20 TDD tests
- `internal/plugins/bgp-adj-rib-in/rib.go` — ribIn type changed, SeqIndex removed, handlers updated
- `internal/plugins/bgp-adj-rib-in/rib_commands.go` — statusJSON uses Len(), showJSON uses Range()
