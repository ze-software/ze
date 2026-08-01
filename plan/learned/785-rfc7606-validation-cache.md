# 785: RFC 7606 Validation Cache Research

**Spec:** research-rfc7606-validation-cache.md
**Status:** Closed (do not implement)
**Date:** 2026-05-25

## What

Investigated whether caching RFC 7606 validation results provides
measurable benefit for route reflector scenarios.

## Findings

Cache hit rate is excellent (77% global, 96.7% per-peer) because BGP
path attributes are highly shared: NEXT_HOP is set at the eBGP border
(1,632 unique across 79.5M routes), MED/LOCAL_PREF/communities are
policy-set at boundaries, AS_PATH is identical for routes from the same
external source.

However, `ValidateUpdateRFC7606()` is too cheap to benefit from caching:
67-138 ns/op for 3-7 attributes, roughly 2-3% of per-UPDATE CPU. The
cache lookup itself (FNV hash + LRU map) would cost 50-80 ns, consuming
half the time it saves. Net throughput improvement would be ~1%.

The attribute pool already handles the expensive dedup (InternExisting:
130 ns, InternNew: 560 ns). Validation is the cheap step before the
expensive steps.

## Key Data

All from existing MRT analysis (93.5M routes, 200K live updates):
- NEXT_HOP: 1,632 unique values, 99.997% cache hit
- Full attribute bundle: 77% dedup rate (with AS_PATH)
- Per-peer LRU: 96.7% hit rate
- Validation: 67-138 ns/op vs pool Intern: 130-560 ns/op

## Decision

Do not implement. Cache hit rate passes (>30%) but CPU fraction fails
(<5%). Adding LRU complexity for ~1% throughput gain is not justified.

## Artifacts

- Research results: `docs/research/rfc7606-validation-cache.md`
- Benchmarks: `internal/component/bgp/message/rfc7606_bench_test.go`
- Domain facts added: `ai/rules/design-context.md` (BGP attribute reuse)

## Side Finding

Happy-path allocation: `ValidateUpdateRFC7606` allocates a 64B result
struct on every call including valid UPDATEs. A singleton return for
action=none would eliminate ~100M allocs/sec under load. Independent of
the cache question.

## Files

None recorded.
