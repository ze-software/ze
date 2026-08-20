# Performance Round 3: Hot-Path Allocation and Lock Reduction

<!-- source: internal/component/bgp/plugins/rib/rib_attr_format.go -- communityList, communityByteList -->
<!-- source: internal/component/bgp/reactor/filter_chain.go -- filterAttrs, parseFilterAttrs -->

Third optimization round. The first campaign took the convergence benchmark from
91ms to 71ms, and the second took it from 71ms to 62ms.

This round targeted three paths identified through source audit and arithmetic,
not speculative profiling. Each change preserves wire bytes, JSON output, and
CLI behavior byte-for-byte. The proof for each is its own Go benchmark, not
the end-to-end ze-perf convergence number (none of these paths are exercised
by the single-DUT 100K-route benchmark).

## 1. Lock-Free EBGP Variant Cache Hits (deleted 2026-08-17)

The reactor cached the EBGP variant of a received UPDATE (local ASN prepended
to AS_PATH per RFC 4271 Section 9.1.2), one entry per AS width. On a route
server that fans one UPDATE to N eBGP peers, the wire was built once and read N
times. Every read took a mutex.

At 100K UPDATE/s with 150 peers, that is about 15 million lock and unlock pairs
per second to read two immutable pointers.

This round replaced the four cache fields with two atomic slots. Each slot
bundled the wire pointer and its backing pool buffer handle in one struct,
published atomically. A `BufHandle` is a three-field struct and cannot be stored
atomically on its own, which is why the bundling mattered. A cache hit became a
single atomic pointer load. Generation stayed single-flight under the mutex with
double-checked locking, the idiom `Peer.negotiated` and `Peer.sendCtx` use.

Eviction loaded each slot atomically and returned the handle. The slots were
fire-once (written at most once, never mutated after publication), so eviction
could not race a reader: the Go memory model guarantees that an atomic load
which observes the stored pointer sees the fully initialized struct.

```
16 goroutines, Apple M4 Max, 2026-07:
  Before (mutex hit):   ~128 ns/op    0 allocs/op
  After (atomic hit):   ~0.36 ns/op   0 allocs/op

Re-measured 2026-08-05, linux/amd64 EPYC 7351, GOMAXPROCS=32, -benchtime=2s:
  mutex hit path        73.6 ns/op    0 allocs/op
  atomic hit path        0.26 ns/op   0 allocs/op
```

`b.RunParallel` divides wall time by total operations, so both figures scale
with GOMAXPROCS. Compare the two numbers on one host. Never compare a recorded
ns/op across machines.

**The cache no longer exists.** The AS-path fold (`e2037e598`) moved eBGP
prepending onto the edit-set path on 2026-08-01 and left the cache with no
non-test caller. `plan/spec-wire-edit-3-deferred-ac9-dead-code.md` deleted the
cache, its two benchmarks and its allocation ceiling. The optimization above was
correct and measured. The traffic it was written for takes another route, so the
numbers are a record and are not re-runnable.

Files: `received_update.go`, `recent_cache.go`.

## 2. Lazy JSON Marshaling for Community Display

Route enrichment for `show bgp rib | json` built, for every displayed route,
fresh `[]string` slices with one `String()` allocation per community element.
On a full-table query with hundreds of thousands of routes carrying 2-4
communities each, that is millions of short-lived string allocations per
request.

The `Community` type already had a zero-allocation `appendCommunityText`
helper, but the display path ignored it. A new exported `Community.AppendText`
method delegates to it. Three wrapper types (`communityList`,
`largeCommunityList`, `extCommunityList`) over the typed community slices
implement `json.Marshaler`, building the JSON array directly via AppendText
with no intermediate strings.

For the pool-backed path (`enrichRouteMapFromEntry`, Adj-RIB-In), a fourth
wrapper (`communityByteList`) operates on raw bytes. It copies the bytes at
enrichment time because `attrpool.Pool.Get` returns a reference into
shard-internal storage, and pool compaction can run concurrently before
`json.Marshal` executes. One byte-slice copy replacing N string allocations
is still a large win.

`formatCommunities` is kept unchanged for the community match filter
(`rib_pipeline.go`) and looking-glass template (`lg/render.go`),
which need string semantics and are lower-volume.

```
BenchmarkEnrichRouteCommunities (100 communities, enrich + json.Marshal):
  Before:  ~110 allocs/op
  After:   19 allocs/op    (~83% reduction)
```

Files: `text_append.go`, `rib_attr_format.go`.

## 3. Struct-Based Filter Attribute Parsing

When an external policy filter modifies an UPDATE, the reactor parses the
filter's text output into a `map[string]string` via `parseFilterAttrs`.
This fires twice per modified UPDATE (original and modified text), and on
the export path it fires per destination peer. Each parse allocated the map
buckets plus a `strings.Fields` slice.

The attribute name set is closed: 22 names defined by `isPolicyAttrName`.
A fixed `filterAttrs` struct with a `[22]string` array and a `uint32`
presence bitset replaces the map. Field access is by a `filterAttrID` enum.
The init-time `filterAttrNameToID` map (built once from the enum) handles
the parse-time name lookup. `parseFilterAttrs` returns `*filterAttrs`
(one heap allocation replacing two map allocations per call).

All consumers (`textDeltaToModOps`, `ExtractASPathPrependOps`,
`ExtractRemovePrivateASOps`, `computeChangedAttrs`, `computeWireChanges`,
`formatFilterAttrs`, `validateModifyDelta`, `applyFilterDelta`) were updated
to use the struct. `textDeltaToModOps` switches from three map-range loops
to a single enum-order iteration. The op multiset is identical (verified by
`TestFilterDeltaParseOnceEquivalence`).

Unknown attribute names at key position are recorded in `unknownName` so
that `validateModifyDelta` can still reject misbehaving plugins, matching
the old map-based behavior.

```
BenchmarkFilterModifyEgress:
  Before:  24 allocs/op   1420 ns/op
  After:   22 allocs/op   1335 ns/op
```

The remaining 22 allocations come from the 14 encoder `make()` sites in
`filter_delta.go`. A pooled scratch buffer per modify block (Phase B) would
reduce this further to roughly 12 allocs/op. The struct foundation is in
place for that work.

Files: `filter_chain.go`, `filter_delta.go`, `policy_dryrun.go`.

## What Was Not Done

An audit investigated seven additional candidates and rejected all of them.
The evidence is recorded in `plan/spec-perf-next-0-umbrella.md` (Negative
Findings table) so future sessions do not re-investigate them:

- Engine event dispatch slice copy: BGP events never reach engine subscribers.
  Dispatch rate is approximately 0.1/s operational.
- UPDATE builder pooling: already done in commit 233ff1726.
- `forward_build.go` pool-fallback `make()`: deliberate tiered escalation.
- RFC 7606 validation cache: stale proposal, never measured.
- `prefixToWire` allocations: CLI inject/withdraw one-shots only.
- seqmap compaction: sound design, infrequent (O(n log n) only when dead > len/2).
- Looking-glass error-path JSON: error responses only.

---

**Last Updated:** 2026-06-15
