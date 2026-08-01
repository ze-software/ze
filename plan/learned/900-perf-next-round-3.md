# 900 -- Third Performance Optimization Round (perf-next)

## Context

After two prior performance campaigns (771, 859) reduced ze's convergence from 91ms to 62ms at 100K routes, an evidence-based audit identified three remaining hot-path improvements. Each was designed from source code audit and arithmetic (15M lock ops/s, 24 allocs/op x fan-out, millions of string allocs per full-table show), not from speculative profiling.

## Decisions

- Chose three independent children over one mega-spec, matching the campaign 859 pattern; independent files, independent bisection
- Chose `atomic.Pointer[ebgpWireSlot]` bundling (wire, BufHandle) over separate atomics; BufHandle is multi-field, separate storage creates a publish window during eviction
- Chose double-checked locking over CAS-race generation; CAS would let two generators each take a pool buffer, leaking the loser's
- Chose lazy `json.Marshaler` wrappers for community display over AppendTo into reused `[]string` (still allocates per element) or cached strings on Route (memory bloat + invalidation)
- Chose `filterAttrs` fixed struct with bitset presence tracking over map[string]string; closed 22-name key set, eliminates map bucket allocations and all key hashing
- Recorded negative findings in the umbrella so future sessions do not re-investigate (engine event dispatch, update builder pooling, prefixToWire, pool-fallback make, RFC 7606 cache, seqmap compaction, LG error-path JSON)
- Docker perf infrastructure (Dockerfile.ze) is stale: references `cmd/ze-test` and `cmd/ze-perf` as separate directories, but these are now build-tagged from `cmd/ze/`; per-child Go benchmarks are the proof per R-1

## Consequences

- EBGPWire cache-hit path is now lock-free; any future change to the variant cache must maintain the atomic publication contract (fire-once, immutable after store)
- Community JSON rendering no longer allocates per-element strings; new community types or display-format changes must implement `MarshalJSON` on the wrapper or add `AppendText`
- Filter-delta parse output is a fixed struct, not a map; adding new filter directive names requires adding a `filterAttrID` enum value (compile-time visible) rather than a silent map key
- `formatCommunities` still exists for filter match and looking-glass template consumers (string-based); these paths are lower-volume and keep their current cost
- Phase B (scratch pool for filter-delta encoders) is deferred: 14 encoder sites each do their own `make([]byte, ...)`, reducible to one pooled scratch per modify block. The struct foundation from Phase A is in place

## Gotchas

- The `prealloc` linter crashes on Go's `range N` syntax (range-over-int); had to use traditional `for` loops with `//nolint:modernize` directives
- Pool.Get returns a reference slice into shard-internal storage, not a copy; the `communityByteList` wrapper copies bytes at enrichment time because pool compaction can run concurrently before `json.Marshal`
- The `ze-perf-bench` single-DUT benchmark does not exercise any of the three children's paths (no RS fan-out for child 1, no policy filters for child 2, no full-table show for child 3); per-child Go benchmarks are the only meaningful proof

## Files

- `internal/component/bgp/reactor/received_update.go` (child 1: ebgpWireSlot, atomic slots, lock-free EBGPWire)
- `internal/component/bgp/reactor/recent_cache.go` (child 1: evictLocked/Delete use atomic loads)
- `internal/component/bgp/reactor/received_update_test.go` (child 1: 2 new tests)
- `internal/component/bgp/reactor/received_update_bench_test.go` (child 1: parallel cache-hit benchmark)
- `internal/component/bgp/reactor/filter_chain.go` (child 2: filterAttrs struct, parseFilterAttrs, formatFilterAttrs)
- `internal/component/bgp/reactor/filter_delta.go` (child 2: textDeltaToModOps, Extract* updated)
- `internal/component/bgp/reactor/policy_dryrun.go` (child 2: computeChangedAttrs, computeWireChanges updated)
- `internal/core/bgp/attribute/text_append.go` (child 3: Community.AppendText)
- `internal/component/bgp/plugins/rib/rib_attr_format.go` (child 3: MarshalJSON wrappers, enrichment switch)
- `internal/component/bgp/plugins/rib/rib_attr_format_test.go` (child 3: golden byte-identity test)
- `internal/component/bgp/plugins/rib/rib_attr_format_bench_test.go` (child 3: enrichment benchmark)
- `docs/architecture/buffer-architecture.md` (child 1: EBGP variant cache section)
