# 771: Performance Optimization Campaign

## Context

Systematic performance work to close the gap with BIRD on 100K-route propagation.
Starting point: Ze at 91ms convergence (1.1M r/s), BIRD at 50ms (2.0M r/s), ratio 1.82x.
End point: Ze at 71ms convergence (1.4M r/s), BIRD at 44ms (2.3M r/s), ratio 1.61x.

## Approach

Profile-driven: two analysis reports (PERFORMANCE_OPTIMIZATION_REPORT.md and
PRECOMPUTATION_OPTIMIZATION_REPORT.md) identified hotspots from Go benchmarks,
allocation profiles, and escape analysis. Each proposal was validated against
actual source code before implementation. Three of seven precomputation proposals
were rejected after code review showed the "repeated derivations" were trivially
cheap (field reads, stack arrays, integer comparisons).

## Optimizations by Layer

### RIB Storage (dominant allocation site: 57.7% of RIB alloc space)

| Change | Commit | Impact |
|--------|--------|--------|
| Bundle RIB attributes for 5x cache density | 472fcaa81 | Reduced per-route memory, improved cache locality |
| Eliminate ParseAttributes bundle heap escape | 86a7c2a83 | `bundle` stays on stack by replacing function-table indirection with direct switch |
| Parse attributes once per UPDATE instead of per NLRI | 82c9e5011 | N NLRIs sharing one attr blob parse once, clone/AddRef for each route |

### Forwarding Path

| Change | Commit | Impact |
|--------|--------|--------|
| Reactor-native RS forwarding, bypass plugin dispatch | 0d1a6a9aa | Route-server path avoids JSON serialization entirely |
| Direct TCP write from read goroutine | 3b2c7c3ba | Eliminates forward-pool channel hop for RS |
| Deferred flush batching, no per-UPDATE deadline/map | 496155e98 | Amortizes syscalls across batch window |
| Zero-alloc AS-PATH prepend for same-encoding EBGP | 4bbaab0e1 | In-place single-prepend avoids copy for ASN4-to-ASN4 |
| Co-locate WireUpdate in ReceivedUpdate, O(1) selector match | c08c5344d | Eliminates pointer indirection and linear selector scan |
| Precompute per-peer forwarding facts at session boundaries | 56c87ce9c | Cached EBGP/IBGP, sendCtxID (mutex), cluster-id, next-hop bytes; refreshed on session/config change |
| Resolve batch destinations once per ForwardUpdatesDirect | 5cd7222de | Peer-map walk and source-info lookup hoisted out of per-ID loop |

### Event Delivery

| Change | Commit | Impact |
|--------|--------|--------|
| Lazy monitor delivery | ce5dccced | JSON formatting skipped entirely when no CLI monitor matches |
| Atomic monitor count and typed delivery | f3c5c93cc | atomic.Int64 replaces RLock for zero-monitor fast path |
| Skip hex encode/decode for structured event raw bytes | 66e115bea | Structured path passes bytes directly, no hex round-trip |

### Formatting

| Change | Commit | Impact |
|--------|--------|--------|
| Eliminate fmt.Sprintf on hot paths | a4f20132f | Replaced with strconv.Append*, append-based builders |
| appendJSONSafeString for safe peer strings | 841e50a37 | Skips per-byte escape check for pre-validated strings |
| Zero-filter parsed JSON fast path | f3c9c7fff | Bypasses filter machinery (map alloc, slice alloc) when no filter is set |
| Stream raw sections from WireUpdate, drop ExtractRawComponents | 181487a58 | Writes hex directly from wire sections, no intermediate map |
| NextHop case in appendAttributeJSON, 0 allocs | 666c8f325 | Parsed JSON path achieves zero allocations on warm AttrsWire |
| Array-indexed enum String(), typed direction | 3524ff8a9 | Branch-free lookup, eliminates per-sent-UPDATE []byte allocation |

### Infrastructure

| Change | Commit | Impact |
|--------|--------|--------|
| textbuf: freeze-after-extract, sync.Pool, Grow, Write | 33cb72936 | Pooled text buffers for diagnostic and formatting paths |
| Reduce GC pressure on hot paths | 3dffa31dd | Sentinel errors, pooled buffers, stack-friendly patterns |
| Zero-alloc send hold timer reset | 899bd8385 | Timer.Reset without channel drain allocation |

## Design Principles That Emerged

1. **Profile before optimizing.** Three proposals from the precomputation report were
   rejected after source review showed operations were stack-allocated or trivially cheap.
   Without validation, they would have added lifecycle complexity for zero gain.

2. **Resolve decisions at lifecycle boundaries.** The pattern that separates BIRD-speed
   code from correct-but-slow code: compute EBGP/IBGP, next-hop mode, send-community mask,
   cluster-id bytes at session setup, not per-UPDATE. C implementations do this naturally.

3. **Sum of small decisions.** At 200 peers and 100K UPDATEs/sec, 20ns per peer per UPDATE
   is 400M ns/sec = 400ms of CPU. Individual items look trivial; in aggregate they are the
   gap with BIRD.

4. **Structured delivery is the strongest single win.** Structured plugin delivery (226 ns,
   6 allocs) vs JSON (23,722 ns, 145 allocs) is a 105x difference. Every path that avoids
   JSON formatting is a path that stays competitive.

5. **The allocation that matters is the one in the inner loop.** A single `make([]byte, 32)`
   per peer per UPDATE at 200 peers is 200 heap allocations per UPDATE. The same allocation
   at session setup is once per peer lifetime.

## Benchmark Results

100K IPv4/unicast routes, Docker/Colima, Apple M4 Max:

| DUT | Before (Apr) | After (May) | Improvement |
|-----|-------------|-------------|-------------|
| Ze convergence | 91ms | 71ms | 22% faster |
| Ze throughput | 1,098,901 r/s | 1,408,450 r/s | 28% higher |
| Ze vs BIRD ratio | 1.82x | 1.61x | Gap closed by 23% |

## Remaining Gap

BIRD converges at 44ms (2.3M r/s). Ze at 71ms (1.4M r/s). The remaining 1.61x gap
is likely in: wire parsing (BIRD parses in-place in a single pass), memory allocation
model (BIRD uses slab allocators with no GC), and TCP write batching (BIRD coalesces
at the socket layer). These are architectural differences, not optimization targets.

## Files

None recorded.
