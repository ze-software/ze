# 765 — GC Pressure Reduction on BGP Hot Paths

## Context

Codebase review of every `make()` call across the BGP component to identify
unnecessary heap allocations on per-UPDATE, per-forward, and per-route paths.
Goal: reduce GC pressure at 1M+ UPDATEs/sec by eliminating allocations that
the caller could avoid.

## Key Decisions

### [256]bool replaces map[uint8]bool for attribute code sets

Attribute type codes are uint8 (0-255). Three sites used `make(map[uint8]bool)`
on every inbound UPDATE: RFC 7606 validation, attribute discard, wire index
building. A `[256]bool` stack array is zero-alloc, faster lookup, and
semantically identical.

### Inline FNV-1a replaces hash/fnv interface allocation

`fnv.New64a()` allocates a `digest64a` on the heap via the `hash.Hash64`
interface. Four sites in the forwarding path (bucket merge, supersede key)
and four in the store (HashBytes, HashUint32, HashString, CombineHashes)
allocated a hasher per call. Replaced with inline FNV-1a computation using
named constants. Zero allocation.

### NextHopAddrs inline struct replaces []netip.Addr slice

`MPReachNLRI.NextHops` was `[]netip.Addr`, allocating a slice on every
MP_REACH_NLRI construction. RFC 2545 bounds next-hops to 2 (global +
link-local). Changed to `NextHopAddrs` struct with `[2]netip.Addr` inline
storage and `uint8` count. `Slice()` returns a view with no allocation.

### LargeCommunities dedup fast path

`Len()` and `WriteTo()` both called `deduplicate()`, which allocated a
`map[LargeCommunity]struct{}` and a new slice. Since `ParseLargeCommunities`
already deduplicates at parse time, the common case has no duplicates.
Added `unique()` fast path: O(n^2) scan for n<=16 (returns original slice
if no dups found), falls back to map-based dedup for larger slices.

### clear() reuses map hash tables

`outgoing.go`, `incoming.go`, `seqmap.go` replaced `map = make(...)` with
`clear(map)` in flush/reset paths. `clear()` removes all entries but
preserves the allocated hash table for reuse.

### Stack-backed append replaces bytes.Buffer in grouping

`buildGroupKey` and `hashASPathString` used `bytes.Buffer` (heap-allocated).
Replaced with `var stack [256]byte; key = stack[:0]` plus `append`. The key
is converted to string at the end (one alloc), but the buffer itself stays
on the stack because `string(key)` copies without retaining the backing array.

## What Did NOT Work: Stack Arrays That Escape

Several attempts to use stack-allocated arrays were reverted because Go's
escape analysis moved them to heap:

| Pattern | Why it escapes |
|---------|---------------|
| `var nhBuf [2]netip.Addr` stored in returned struct field | Struct is heap-allocated, field reference escapes |
| `var nh [32]byte` passed to `mods.Op()` as `nh[:]` | ModAccumulator stores the slice |
| `var sortBuf [16]Attribute` captured by `sort.Slice` closure | Closure captures the slice backed by the array |
| `var attrScratch [64]byte` appended into `key` | `key` escapes via `string(key)` |
| `var stack [64]byte` passed to `store.HashBytes(buf)` | Interface method can't prove non-retention |

**Lesson:** stack arrays only help when the backing array never leaves the
function through any path: no return, no closure capture, no interface method
parameter, no append into an escaping slice. Verify with `go test -gcflags='-m'`
before claiming a stack optimization works.

### Effective stack patterns

- `var stack [N]byte; key = stack[:0]; key = append(key, ...); return string(key)` — the `string()` conversion copies, so `stack` does not escape.
- `var seen [256]bool` — value type, never referenced externally.
- `[256][]T` fixed array — value type returned by value (large but stack-allocated in caller).

### Alternatives for escaped buffers

When stack arrays escape, use:
- `sync.Pool` for reusable scratch buffers
- Inline computation (FNV-1a) to avoid buffers entirely
- Structural change (inline fields like `NextHopAddrs`) to avoid slices at the source
- Streaming hash to avoid materializing bytes

## Mechanics

- 26 files changed across attribute, message, reactor, rib, store, seqmap, perf
- All existing tests pass unchanged (except test struct literals updated for NextHopAddrs)
- `go vet ./...` clean
- Review found zero BLOCKERs after fixes

## Files

None recorded.
