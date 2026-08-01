# Learned: RIB Attribute Bundle (spec-perf-1)

**Spec:** spec-perf-1-rib-cache-layout.md
**Date:** 2026-05-16

## What

Replaced 13 inline per-attribute handles on RouteEntry (56 bytes) with a
Bundle handle + ASPath handle (12 bytes). The 12 non-ASPath attributes are
stored in a shared Bundle type deduped by BundlePool.

## Key decisions

- **Bundle over AS_PATH-separation-only.** AS_PATH separation alone (56 -> 52 bytes)
  doesn't cross the 1-route-per-cache-line threshold. Bundling (56 -> 12 bytes) gets
  5 per cache line. The change surface is similar either way (114 RouteEntry refs).

- **BundlePool uses map[Bundle]uint32, not attrpool.Pool.** Bundle is a fixed-size
  comparable struct (12 Handle fields = 48 bytes). Using it as a map key gives
  zero-serialization dedup. The existing byte-slice pool doesn't fit fixed-size structs.

- **Cascade release on refcount zero.** BundlePool.Release at refcount 0 releases all
  12 inner attribute handles. The release happens outside the BundlePool mutex to
  avoid lock ordering issues with per-attribute pools.

- **attachCommunity uses AddRefInnerHandles.** Modifying one attribute in a shared
  bundle requires creating a new bundle with fresh refs for all 12 handles. The old
  bundle's refs are independent.

## Performance

| Metric | Before | After |
|--------|--------|-------|
| RIB scan 100K (ns/op) | 1,929,914 | 1,586,062 |
| Scan memory (B/op) | 10,853 | 2,281 |
| Insert no-op 100K (ns/op) | 56,517,392 | 48,481,889 |

## Mistakes

- Global pool state contamination in tests: cascade release test used `[]byte{0x01}`
  which collides with wireOriginEGP in attrparse_test.go across -count=N runs.
  Fix: use unique byte patterns for test data that exercises pool release.

## Patterns to reuse

- Comparable struct as map key for fixed-size dedup (no serialization needed).
- Cascade release: pool owns inner refs, releases them atomically when outer refcount
  hits zero. Mutex released before inner releases to prevent lock ordering issues.
- RWMutex for read-heavy pool access (Get is the hot path, Intern/Release are write paths).

## Files

None recorded.
