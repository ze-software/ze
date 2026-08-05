# 1347 -- A Speedup Whose Baseline The Same Commit Deleted

## Context

`ReceivedUpdate.EBGPWire` cached the AS-prepended eBGP variant of an UPDATE. It
took `ebgpMu` on every call, cache hits included. On a route server fanning one
UPDATE to 100+ eBGP peers that was a mutex pair per read of two immutable
pointers.

The work replaced four cache fields with two `atomic.Pointer` slots bundling the
wire and its pool `BufHandle`. A hit became one atomic load. Generation stayed
single-flight under the mutex. It landed in `b5ad2cabe` on 2026-06-15.

By the closure on 2026-08-05 the AS-path fold (`e2037e598`, 2026-08-01) had
moved eBGP prepending onto the edit-set path. `EBGPWire` had no non-test caller
left.

## Decisions

- Bundled the wire pointer and its `BufHandle` in one `ebgpWireSlot` published by a single atomic store, over two separate atomics. `BufHandle` is a multi-field struct, and separate storage opens a window where eviction sees the wire but not the handle.
- Kept `ebgpMu` for generation with a re-check after the lock, over a lock-free CAS race. Two racing generators each borrow a pool buffer, and the loser's must be given back for no gain.
- Closed the spec recording the dead-path fact, rather than deleting the cache inside the closing commit. The deletion touches read-pool buffer lifetime, wants its own soak, and is already homed at `plan/spec-wire-edit-3-deferred-ac9-dead-code.md`. Folding it in would cost the closing commit its focus (`ai/rules/rule-precedence.md`).
- Added `BenchmarkEBGPWireCacheHitParallelMutexBaseline`, a comparator reproducing the pre-change mutex hit path, over citing a number from a deleted working tree.

## Consequences

- The before number is re-runnable by anyone. Run both benchmarks in `internal/component/bgp/reactor` and compare. Measured 2026-08-05 on a 32-thread EPYC 7351: 73.6 ns/op against 0.26 ns/op, 0 allocs/op on both.
- `internal/perf/allocgate.go` registers the hit benchmark at 0 allocs. A regression in the slot path fails `make ze-verify` even while the method has no production caller.
- Anyone deleting the cache must delete four things together. `EBGPWire`. `ebgpWireSlot` and the two slots. The two release branches in `recent_cache.go`. Both benchmarks with the alloc-gate entry.
- `evictLocked` and `Delete` return the slot handle but leave the slot pointer set. That is safe only because the entry leaves the cache in the same call. Anyone keeping a `*ReceivedUpdate` alive past eviction must `Store(nil)` first.

## Gotchas

- **An honestly measured baseline can still be unreproducible.** The 128 ns/op before number was real, and `docs/architecture/perf-round-3.md` names its host (Apple M4 Max). It was still unusable. The benchmark file and the optimization landed in the same commit, so the pre-change tree ceased to exist the instant the number was recorded. No gate can re-derive it. A baseline needs a producer that survives the change: a committed comparator benchmark, or a stored `benchstat` file. This is the second occurrence in this family. `spec-perf-next-3-rib-show-alloc.md` claims an allocation cut against a baseline that was never measured at all.
- **An optimization can be superseded between implementation and closure.** The code landed 2026-06-15. The AS-path fold removed its last caller on 2026-08-01, seven weeks before anyone came back to close the spec. Assumption A-1 ("multiple goroutines call `EBGPWire` concurrently") went from true to false with nobody editing the spec. Closure grepped for callers instead of trusting the spec's own caller trace, which is the only reason it was caught. Re-derive a perf spec's caller trace at closure. The trace is a measurement with an expiry date, not a fact.
- **Comments outlive reachability.** Four places still called this a live RS fan-out hot path after the caller was gone. A benchmark header, the alloc-gate registration comment, a `RewriteASPath` comment in `wireu`, and an architecture doc section. A grep for the symbol name finds callers. It does not find prose that assumes them.
- **`b.RunParallel` divides wall time by total operations, so a shared read scales with GOMAXPROCS.** 0.26 ns/op is not a sub-nanosecond atomic load. It is about 8 ns of load spread over 32 threads. Compare the two benchmarks on one host. Never compare a recorded ns/op across machines.
- **Closing a spec can break `make ze-doc-test` from a distance.** `scripts/dev/learned_staleness.py` is a shrink-only ratchet. A learned summary citing `plan/spec-<name>.md` becomes a dead reference the moment closure removes that spec. Two summaries from an earlier closure had pushed the count two over baseline. A learned summary must not cite the spec path that its own closure deletes.

## Files

- `internal/component/bgp/reactor/received_update.go` -- `ebgpWireSlot`, the two atomic slots, `EBGPWire`, `ebgpSlot`
- `internal/component/bgp/reactor/recent_cache.go` -- `evictLocked` and `Delete` load the slots atomically
- `internal/component/bgp/reactor/received_update_test.go` -- eviction and error-path tests
- `internal/component/bgp/reactor/received_update_bench_test.go` -- hit-path benchmark
- `internal/component/bgp/reactor/received_update_bench_baseline_test.go` -- mutex comparator, new at closure
- `internal/perf/allocgate.go` -- alloc ceiling and its comment
- `internal/component/bgp/wireu/aspath_rewrite.go` -- stale cache comment removed
- `docs/architecture/buffer-architecture.md` -- EBGP Variant Cache section
- `docs/architecture/perf-round-3.md` -- re-measured numbers, the RunParallel caveat, the reachability note
- `plan/learned/1336-withdraw-only-relay-shape.md`, `plan/learned/1346-rfc7606-5-1-2-relay-shape.md` -- dead spec citations removed to clear the staleness ratchet
