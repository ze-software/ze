# Spec: RFC 7606 Validation Cache (Research)

## Task

Investigate whether caching RFC 7606 validation results provides measurable benefit. This is a research task — implementation only proceeds if measurements justify it.

**Priority:** Low — optimization, not required for correctness
**Prerequisite:** RFC 7606 validation must be implemented first

## Hypothesis

`ValidateUpdateRFC7606()` is called for every UPDATE message. In route reflector scenarios, many UPDATEs share identical path attributes, causing repeated validation of the same bytes. An LRU cache keyed on attribute bytes + session context could eliminate redundant work.

RFC 7606 validation is a pure function (same input = same output), making it inherently cacheable.

## Research Questions

1. What fraction of validation calls see repeated attribute bytes? (target: >30% hit rate)
2. What fraction of CPU time does validation consume? (target: >5%)
3. Does the memory cost of caching (~2.5 MB for 10k entries) justify the savings?

## Go/No-Go Criteria

| Criterion | Threshold | If not met |
|-----------|-----------|------------|
| Cache hit rate | > 30% under realistic traffic | Do not implement |
| CPU fraction | > 5% of UPDATE processing time | Do not implement |
| Memory budget | < 10 MB for useful cache sizes | Reduce cache size or do not implement |

## Measurement Plan

1. Write `rfc7606_bench_test.go` benchmarking `ValidateUpdateRFC7606()`
2. Profile CPU under simulated route reflector traffic (many peers, shared prefixes)
3. Measure attribute byte uniqueness across UPDATE streams
4. Estimate memory per cache entry from actual attribute sizes

## Memory Estimates

| Cache Size | Memory |
|------------|--------|
| 1,000 | ~250 KB |
| 10,000 | ~2.5 MB |
| 100,000 | ~25 MB |

## Implementation Sketch (only if research justifies)

- LRU cache keyed on hash of attribute bytes + relevant session flags
- Disabled by default, enabled via config
- Cache metrics (hits, misses, evictions) for observability
- No external dependencies preferred; evaluate `hashicorp/golang-lru/v2` vs stdlib map + linked list

## Files Likely Involved

- `internal/bgp/message/rfc7606_bench_test.go` — benchmarks (research output)
- `internal/bgp/message/validation_cache.go` — cache implementation (if justified)
- `internal/bgp/message/validation_cache_test.go` — cache tests (if justified)
