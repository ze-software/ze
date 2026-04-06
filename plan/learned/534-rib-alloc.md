# 534 -- RIB Allocation Optimization

## Context

The RIB storage layer allocated two heap objects per route: a `string(nlriBytes)` map key and a `*RouteEntry` pointer. With 1M routes, this meant 2M allocations that the GC had to track. BIRD avoids this with fixed-size trie nodes and slab allocators. Ze needed a Go-idiomatic equivalent.

## Decisions

- Chose `NLRIKey` struct (`len uint8` + `data [24]byte`) over bare `[24]byte` array because iteration needs exact-length NLRI bytes for `wireToPrefix`/`formatNLRIAsPrefix` -- `Bytes()` returns `data[:len]` without trailing zeros.
- Chose value-type `RouteEntry` in `map[NLRIKey]RouteEntry` over `sync.Pool` because RouteEntry is long-lived (stored until withdrawal), making `sync.Pool` ineffective. At 56 bytes with alignment, it fits under Go's 128-byte map-value inline threshold.
- Chose get-modify-put pattern for StaleLevel mutation over keeping pointer types because eliminating pointer indirection on every lookup is worth the O(1) extra map write on the rare stale mutation path.
- Added `ModifyEntry`/`ModifyAll`/`ModifyFamilyEntry`/`ModifyFamilyAll` methods for cases requiring entry mutation (attach-community, mark-stale), over changing `attachCommunity` to a standalone pattern, because the map owns the lifecycle and mutation should go through it.
- Chose two-pass `splitNLRIs` (count then allocate) over single-pass with `append` because the counting pass is identical O(n) work and eliminates all slice growth allocations.
- Did not change attrpool initial capacity -- pools are already pre-sized generously (AS_PATH at 256KB, Communities at 64KB).

## Consequences

- Per-route heap allocations eliminated: ~2M fewer allocs per 1M routes (NLRIKey + RouteEntry).
- `FamilyRIB.LookupEntry` now returns `(RouteEntry, bool)` value copy instead of `(*RouteEntry, bool)`. All callers receive read-only copies. Mutation requires `ModifyEntry`/`ModifyAll`.
- `PipelineRecord.InEntry` is now `storage.RouteEntry` (value) with a `HasInEntry bool` sentinel replacing `!= nil` checks.
- `ParseAttributes` returns `RouteEntry` value. attrInterners closures still use `*RouteEntry` via pointer to local variable during parsing.
- `splitNLRIs` result slice has exact capacity (no over-allocation), sub-slices are still zero-copy into the original data.

## Gotchas

- `attachCommunity` mutates entry.Communities handle -- needed `ModifyFamilyAll` wrapper to write back into value map. Iteration with mutation requires the modify helpers, not direct iterate.
- `IterateEntry` callback signature changed from `func([]byte, *RouteEntry) bool` to `func([]byte, RouteEntry) bool` -- every callback across 8 files needed updating.
- `item.InEntry != nil` checks (4 occurrences across pipeline files) had to become `item.HasInEntry` -- value types cannot be compared to nil.
- The fuzz test `FuzzParseAttributes` had `entry == nil` check that became invalid with value return type.
- Go's 128-byte map inline threshold is gc-compiler-specific, not a language guarantee.

## Files

- `internal/component/bgp/plugins/rib/storage/nlrikey.go` -- new NLRIKey type
- `internal/component/bgp/plugins/rib/storage/nlrikey_test.go` -- NLRIKey tests
- `internal/component/bgp/plugins/rib/storage/familyrib.go` -- map[NLRIKey]RouteEntry, ModifyEntry/ModifyAll
- `internal/component/bgp/plugins/rib/storage/routeentry.go` -- NewRouteEntry returns value
- `internal/component/bgp/plugins/rib/storage/attrparse.go` -- ParseAttributes returns value
- `internal/component/bgp/plugins/rib/storage/peerrib.go` -- value-type API, ModifyFamilyEntry/ModifyFamilyAll
- `internal/component/bgp/plugins/rib/rib_nlri.go` -- splitNLRIs pre-count
- `internal/component/bgp/plugins/rib/rib_pipeline.go` -- HasInEntry sentinel, value callbacks
- `internal/component/bgp/plugins/rib/rib_pipeline_best.go` -- value callbacks
- `internal/component/bgp/plugins/rib/rib_commands.go` -- value params, ModifyFamilyAll for attach
- `internal/component/bgp/plugins/rib/rib_bestchange.go` -- value params
- `internal/component/bgp/plugins/rib/rib_attr_format.go` -- value params
- `docs/architecture/plugin/rib-storage-design.md` -- updated FamilyRIB description
