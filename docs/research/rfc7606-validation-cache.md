# RFC 7606 Validation Cache: Research Results

**Date:** 2026-05-25
**Verdict:** Do not implement.
**Spec:** plan/research-rfc7606-validation-cache.md (closed)

## Hypothesis

Caching `ValidateUpdateRFC7606()` results with an LRU keyed on
`(pathAttrs bytes, hasNLRI, isIBGP, asn4)` could eliminate redundant
validation in route reflector scenarios where many UPDATEs share
identical path attributes.

## Data Sources

- MRT analysis: 93.5M RIB routes + 200K live updates across LINX,
  RIPE RIS, RouteViews (`docs/research/mrt-attribute-caching.md`)
- Bundle dedup analysis: 55M routes (`docs/research/attribute-bundle-dedup.md`)
- UPDATE density: 5-minute BGP4MP stream (`docs/architecture/update-density-analysis.md`)
- Benchmarks: `internal/component/bgp/message/rfc7606_bench_test.go` +
  `internal/component/bgp/attrpool/benchmark_test.go`

## Results

### Q1: Cache hit rate (threshold: >30%)

**Pass. 77-97% depending on strategy.**

From existing MRT research, attribute bundle dedup rates on real data:

| Strategy | Unique bundles | Hit rate |
|----------|---------------|----------|
| Full bundle (incl. AS_PATH) | 9M / 55M | 77% |
| Exclude AS_PATH | 1.7M / 55M | 97% |
| Per-peer LRU | - | 96.7% |

The RFC 7606 cache key is the full pathAttrs bytes, corresponding to the
"full bundle" row: 77% hit rate globally, 96.7% per-peer.

Why attribute reuse is high: NEXT_HOP is set at the eBGP border router
(only 1,632 unique values across 79.5M routes). MED, LOCAL_PREF, and
communities are policy-set at network boundaries and shared across most
routes from the same origin. AS_PATH is identical for all routes learned
from the same external source (IBGP does not prepend).

### Q2: CPU fraction (threshold: >5%)

**Fail. Validation costs 67-138 ns/op, roughly 2-3% of per-UPDATE processing.**

Benchmark results (Apple M4 Max, arm64):

| Case | ns/op | allocs | Notes |
|------|-------|--------|-------|
| 3 attrs | 67 ns | 1 (64B) | 23% of real routes |
| 5 attrs | 100 ns | 1 (64B) | 31% of real routes |
| 7 attrs IBGP | 138 ns | 1 (64B) | RR scenario |
| Empty (withdrawals) | 18 ns | 1 (64B) | No path attrs |

Downstream operations on the same UPDATE for comparison:

| Operation | ns/op | Source |
|-----------|-------|--------|
| attrpool InternExisting | 130 ns | attrpool benchmark |
| attrpool InternNew | 560 ns | attrpool benchmark |
| attrpool Deduplication | 187 ns | attrpool benchmark |

A typical UPDATE triggers 4-5 pool Intern calls (~520-650 ns) plus cache
operations, RIB insertion, and forwarding. Validation at 67-138 ns is
roughly 2-3% of total per-UPDATE CPU.

### Q3: Memory cost (threshold: <10 MB)

**Pass (moot). ~2.5 MB for 10K entries.**

Typical pathAttrs bytes are 30-80 bytes (3-5 attributes cover 89% of
routes per the attribute count analysis). A 10K-entry LRU would cost
~2.5 MB. Within budget, but irrelevant given Q2 failure.

## Why Caching Does Not Help

The cache lookup (FNV hash of pathAttrs bytes + LRU map lookup) would
cost ~50-80 ns. The validation itself costs 67-138 ns. At 77% hit rate,
the net saving per UPDATE is roughly 50-70 ns, or about 1% of total
processing time. The cache adds complexity (LRU structure, concurrency,
memory management) for negligible throughput improvement.

The existing architecture already handles the expensive dedup at the
right layer: the attribute pool interns identical attribute bytes with
FNV hashing and refcounting, which is where the real cost concentrates.
Validation is the cheap linear scan that runs before the expensive pool
operations.

## Go/No-Go Summary

| Criterion | Threshold | Measured | Verdict |
|-----------|-----------|----------|---------|
| Cache hit rate | >30% | 77-97% | Pass |
| CPU fraction | >5% | ~2-3% | **Fail** |
| Memory budget | <10 MB | ~2.5 MB | Pass |

**Decision: Do not implement.** Cache hit rate is excellent but
validation is too cheap relative to total UPDATE cost. The cache lookup
itself consumes half the time it would save.

## Separate Finding: Happy-Path Allocation

`ValidateUpdateRFC7606` allocates a 64-byte `RFC7606ValidationResult`
on every call, including the happy path (action=none). At full-table
ingestion rates this produces ~100M unnecessary allocs/sec. Returning a
package-level `validResult` singleton for the happy path would eliminate
this. This is a code change, not a cache, and is independent of this
research question.

## Related

- `docs/research/mrt-attribute-caching.md` (attribute reuse data)
- `docs/research/attribute-bundle-dedup.md` (bundle dedup rates)
- `docs/architecture/update-density-analysis.md` (NLRI density)
- `internal/component/bgp/message/rfc7606_bench_test.go` (benchmarks)
