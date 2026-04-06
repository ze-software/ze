# 534 -- RIB Allocation Optimization

## Context

The RIB storage layer had two performance problems: (1) per-route heap allocations from `string(nlriBytes)` map key and `*RouteEntry` pointer, and (2) a catastrophic throughput cliff at 1M+ routes caused by Go map rehashing -- working set exceeds L3 cache during rehash, every lookup becomes a main-memory round trip. BIRD avoids this with Patricia tries (incremental growth, no rehash). Benchmarks showed ze matching BIRD at 500k (242k/s vs 245k/s) but collapsing to 22k/s at 1M vs BIRD's 631k/s.

## Decisions

- Chose BART (gaissmai/bart, popcount-compressed multibit trie) over classic Patricia trie because BART branches 8 bits at a time (max depth 4 for IPv4 vs 32 for Patricia), uses cache-friendly bitmask operations, and has a production Go implementation with `netip.Prefix` API.
- Chose BART for non-ADD-PATH only, map fallback for ADD-PATH. BART keys on `netip.Prefix` which has no path-ID concept. ADD-PATH families use `map[NLRIKey]RouteEntry` (the full table case that hits 1M routes is always non-ADD-PATH).
- Chose value-type `RouteEntry` in both BART and map over pointer types. At 56 bytes with alignment, it fits under Go's 128-byte map-value inline threshold and BART stores values by copy.
- Added `NLRIToPrefix`/`PrefixToNLRI` conversion functions over modifying the wire format. The conversion is a few nanoseconds per call and keeps the wire layer unchanged.
- Added `SetAddPath` call in `handleReceivedPool` before first insert when event has ADD-PATH. Without this, the FamilyRIB was created in trie mode and two ADD-PATH NLRIs with different path-IDs but same prefix would collapse to one entry.

## Consequences

- No rehash cliff: BART grows incrementally (one node per insert), no O(n) rehash at any table size. The 1M route round should match the 500k round.
- New dependency: `github.com/gaissmai/bart` v0.26.1 (MIT, pure Go, requires Go 1.23+).
- `PrefixToNLRI` allocates a small `[]byte` per iteration call (wire bytes for callers). This is in the show/display path, not the insert hot path.
- `PurgeStale` on BART collects stale prefixes into a slice before deleting (can't delete during BART iteration). Small transient allocation proportional to stale count.
- ADD-PATH families still use Go maps. If ADD-PATH + 1M routes becomes a real scenario, the map would need pre-sizing or a different structure.

## Gotchas

- BART `Get()` is exact-match (not LPM), which is what the RIB needs. `Lookup()` is LPM and takes `netip.Addr`, not `netip.Prefix` -- easy to confuse.
- BART iteration via `All()` returns `iter.Seq2` (Go 1.23 range-over-func). Can't delete during iteration.
- The `SetAddPath` bug was latent in the old code -- string keys accidentally included path-ID bytes, making it work by coincidence. BART exposed the real bug.
- `go mod vendor` is required after `go get` -- the lint hook runs with `-mod=vendor`.

## Files

- `internal/component/bgp/plugins/rib/storage/nlrikey.go` -- NLRIKey, NLRIToPrefix, PrefixToNLRI
- `internal/component/bgp/plugins/rib/storage/familyrib.go` -- BART trie + map dual-mode storage
- `internal/component/bgp/plugins/rib/storage/routeentry.go` -- value-type NewRouteEntry
- `internal/component/bgp/plugins/rib/storage/attrparse.go` -- value-type ParseAttributes
- `internal/component/bgp/plugins/rib/storage/peerrib.go` -- value-type API, ModifyFamilyEntry/ModifyFamilyAll
- `internal/component/bgp/plugins/rib/rib.go` -- SetAddPath before insert
- `internal/component/bgp/plugins/rib/rib_nlri.go` -- splitNLRIs pre-count
- `internal/component/bgp/plugins/rib/rib_pipeline.go` -- HasInEntry sentinel
- 5 other rib files -- value-type callback/param adaptation
