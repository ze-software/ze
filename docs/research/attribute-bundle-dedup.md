# Attribute Bundle Dedup Analysis

## Overview

`ze-analyse attributes <mrt-file.gz>` measures how much route attribute data
can be deduplicated by hashing attribute bundles. The analysis processes
RIPE RIS and RouteViews MRT dumps (TABLE_DUMP_V2 RIB snapshots and BGP4MP
live update streams), fetched via `ze-analyse download`.

## What is a "bundle"?

A bundle is an FNV-64a hash of concatenated `(type_code, raw_wire_value)` pairs
from a route's path attributes. Hashing operates on raw wire bytes as they appear
in the MRT dump, not parsed or reconstructed data.

Four bundle variants are computed per route simultaneously
(`attrAnalyzeRoute` in `cmd/ze-analyse/attributes.go`):

| Bundle | Includes | Excludes |
|---|---|---|
| Without AS_PATH | All path attributes except AS_PATH (2), AS4_PATH (17), MP_REACH (14), MP_UNREACH (15) | AS_PATH, AS4_PATH, MP_REACH/UNREACH |
| With AS_PATH | All path attributes except MP_REACH (14), MP_UNREACH (15) | MP_REACH/UNREACH only |
| No communities | Like "Without AS_PATH" but also excludes COMMUNITY (8), LARGE_COMMUNITY (32), EXT_COMMUNITY (16) | AS_PATH + all community types |
| Minimal | Only ORIGIN (1) + NEXT_HOP (3) + LOCAL_PREF (5) | Everything else |

MP_REACH/MP_UNREACH are always excluded because they carry per-prefix NLRI data
that is inherently unique per route.

## How uniqueness is counted

Each hash is used as a key in `map[uint64]uint64` (hash to occurrence count).
Unique bundles = `len(map)`. Dedup rate = `(total - unique) / total`.

```go
func jsonBundle(bs *bundleStats) *attrJSONBundle {
    u := uint64(len(bs.Values))
    hr := 0.0
    if bs.Total > 0 {
        hr = float64(bs.Total-u) / float64(bs.Total)
    }
    return &attrJSONBundle{Total: bs.Total, Unique: u, CacheHitRate: hr}
}
```

## Results (55M routes, full table)

| AS_PATH | Unique bundles | Dedup rate |
|---------|---------------|------------|
| With | 9M / 55M | 84% |
| Without | 1.7M / 55M | 97% |

### Interpretation

- **With AS_PATH (84%):** when AS_PATH is part of the bundle hash, 9M unique
  attribute combinations exist across 55M routes. 84% of routes share their
  full attribute set (including AS_PATH) with at least one other route.

- **Without AS_PATH (97%):** dropping AS_PATH from the hash collapses uniqueness
  to 1.7M. This shows AS_PATH is the primary differentiator between routes that
  otherwise share identical attributes. 97% of routes share their non-AS_PATH
  attributes with other routes.

The gap between 84% and 97% quantifies the cost of AS_PATH diversity. All other
attributes (NEXT_HOP, COMMUNITY, ORIGIN, LOCAL_PREF, etc.) are highly shared.

## Consecutive hit rate (temporal locality)

The code also tracks whether consecutive routes in the MRT stream share the same
bundle hash. This measures whether a single-entry "last bundle" cache would hit,
which is relevant for UPDATE streams where consecutive routes from the same peer
often share attributes.

## AS_PATH suffix sharing analysis

`ze-analyse aspath <mrt-file.gz>` builds a reversed trie over all AS_PATHs
to measure suffix sharing. Each AS_PATH is inserted origin-first (reversed),
so paths sharing the same origin and transit chain share trie nodes.

### Results (three vantage points, 2026-03-24)

| Metric | RIPE RIS rrc00 | LINX rrc01 | RouteViews |
|---|---|---|---|
| Collector type | Amsterdam multi-hop | LINX IXP | Oregon multi-hop |
| Total AS_PATHs | 54,680,391 | 44,848,097 | 17,820,553 |
| Unique AS_PATHs | 7,001,820 (87.2%) | 5,610,294 (87.5%) | 2,217,557 (87.6%) |
| Naive storage | 260M slots (~994 MB) | 197M slots (~752 MB) | 77M slots (~296 MB) |
| Reversed trie nodes | 11,284,707 (~43 MB) | 7,064,085 (~27 MB) | 3,487,550 (~13 MB) |
| **Trie compression** | **95.67%** | **96.42%** | **95.50%** |
| Chain nodes | 69.2% | 52.6% | ~68% |

Compression is consistently above 95% across all vantage points. The LINX
collector shows the best compression (96.42%), which makes sense: IXP peers
produce more path diversity at the edge but stronger convergence through
transit, maximizing suffix sharing.

### Path length distribution

75% of paths are 3-5 ASNs long across all three datasets. Peak at length 4
(32%). LINX has more length-2 paths (6% vs 3%) from direct IXP peering.

| Length | RIS rrc00 | LINX rrc01 | RouteViews |
|---|---|---|---|
| 1-2 | 2.9% | 6.0% | 5.4% |
| 3 | 18.2% | 27.7% | 29.4% |
| 4 | 32.1% | 32.4% | 32.5% |
| 5 | 24.0% | 16.0% | 15.9% |
| 6 | 10.9% | 7.4% | 7.2% |
| 7+ | 11.9% | 10.5% | 9.6% |

### Trie shape

- ~86K origin ASNs at trie root (depth 0)
- Branching peaks at depth 2-4 (transit layer)
- LINX has fewer chain nodes (52.6%) than transit collectors (~69%),
  meaning more branching: path compression helps less, but raw
  compression is already higher
- Top origin ASNs by path count: AS16509 (Amazon), AS9808 (China Mobile),
  AS8151 (Uninet), AS12479 (Orange Espana), AS13335 (Cloudflare)

### Implications for Ze's architecture

Per-attribute-type pooling (dedup NEXT_HOP, COMMUNITY, etc. independently)
captures most of the memory savings (97% without AS_PATH).

For AS_PATH itself, a reversed trie with path-compressed chains is the
recommended approach:

- **23x memory reduction** vs storing full paths per route (994 MB to 43 MB
  for raw ASN data, before pointers and bookkeeping)
- Append-only during RIB loading (insert leaf for each new path)
- Prepend-on-export adds one new leaf node (cheap)
- AS_PATH length comparison for best-path requires walking to root (trade-off)
- Path compression on the 69% single-child chains would reduce further

The alternative (whole-path interning, hash to pool index) gets 87% dedup
but stores each of the 7M unique paths fully. That is simpler but uses ~7x
more memory for AS_PATH storage than the trie approach.

## Related

- `docs/research/mrt-attribute-caching.md` -- earlier caching analysis
- `docs/research/performance-analysis.md` -- broader performance findings
- `cmd/ze-analyse/attributes.go` -- bundle dedup implementation
- `cmd/ze-analyse/aspath.go` -- suffix sharing trie implementation
