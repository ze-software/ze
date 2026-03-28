# MRT Analysis

Ze includes `ze-analyse`, a standalone tool for analysing real-world BGP data
from public route collectors (RIPE RIS, RouteViews). It processes MRT dump files
to extract statistics that inform ze's internal buffer sizing, caching strategies,
and congestion handling.

<!-- source: cmd/ze-analyse/main.go -- ze-analyse CLI entry point -->

## Building

```
make analyse
```

This produces `bin/ze-analyse`.

## Quick Start

Download BGP data from public collectors and run an analysis:

```
bin/ze-analyse download                                    # fetch latest data
bin/ze-analyse density test/internet/ripe-updates.*.gz     # UPDATE density + burst patterns
bin/ze-analyse attributes test/internet/latest-bview.gz    # attribute repetition analysis
```

## Data Sources

`ze-analyse download` fetches MRT files from two public BGP collectors:

| Source | Type | Interval | Size |
|--------|------|----------|------|
| RIPE RIS rrc00 (Amsterdam) | BGP4MP updates | 5 min | ~5 MB per file |
| RIPE RIS rrc00 | TABLE_DUMP_V2 RIB | Latest | ~400 MB |
| RouteViews route-views2 | BGP4MP updates | 15 min | ~2 MB per file |
| RouteViews route-views2 | TABLE_DUMP_V2 RIB | 2-hour intervals | ~100 MB |

<!-- source: cmd/ze-analyse/download.go -- download URLs and conversion -->

Files are saved to `test/internet/` (gitignored). RouteViews bz2 files are
converted to gzip on download for Go stdlib compatibility.

```
bin/ze-analyse download                     # today's data at 00:00 UTC
bin/ze-analyse download 20260324            # specific date
bin/ze-analyse download 20260324 1200       # specific date and time
bin/ze-analyse download -o /tmp/mrt         # custom output directory
```

## Commands

### density

Measures how many NLRIs each UPDATE carries and how many UPDATEs arrive per
second. Separates traffic into setup (table dumps, convergence) and maintenance
(steady-state churn) using per-source-peer burst detection.

<!-- source: cmd/ze-analyse/density.go -- NLRI counting and burst detection -->

```
bin/ze-analyse density test/internet/ripe-updates.*.gz
```

**Output sections:**
- NLRIs per UPDATE distribution (announced, withdrawn, total)
- UPDATEs per active second distribution
- Setup vs maintenance classification per source peer
- Per-peer maintenance rate distribution
- Channel sizing recommendation with empirical P50/P95/P99

**Used for:** per-peer forward pool channel sizing. Results documented in
[Update Density Analysis](../architecture/update-density-analysis.md).

### attributes

Analyses attribute repetition across routes to guide caching decisions. Measures
per-attribute cache hit rates, bundle deduplication effectiveness, and temporal
locality (consecutive identical bundles).

<!-- source: cmd/ze-analyse/attributes.go -- bundle hashing and community extraction -->

```
bin/ze-analyse attributes test/internet/latest-bview.gz 2>/dev/null | jq .   # JSON
bin/ze-analyse attributes test/internet/latest-bview.gz >/dev/null           # summary
```

**Output:** JSON to stdout, human summary to stderr.

**Used for:** attribute pool sizing, cache strategy decisions. Results documented
in [mrt-attribute-caching.md](../research/mrt-attribute-caching.md).

### communities

Identifies per-ASN community defaults: communities that appear in nearly every
route from a given ASN. These defaults can be assumed present in a cache,
encoding only exceptions (absent defaults) to save wire bytes.

<!-- source: cmd/ze-analyse/communities.go -- per-ASN frequency analysis -->

```
bin/ze-analyse communities test/internet/latest-bview.gz
bin/ze-analyse communities --threshold 0.90 --format json test/internet/latest-bview.gz
bin/ze-analyse communities --post-policy test/internet/latest-bview.gz
```

**Options:**
- `--threshold` (default 0.95): minimum frequency to be considered a default
- `--min-routes` (default 1000): minimum routes from an ASN to generate defaults
- `--format` (yaml or json)
- `--post-policy`: strip action communities (simulates route server post-policy view)

### count-attrs

Counts how many path attributes each route carries. Produces a distribution
table showing the typical attribute set size.

```
bin/ze-analyse count-attrs test/internet/latest-bview.gz
```

### mrt-dump

Dumps MRT records as BGP UPDATE hex, one per line. Useful for piping into
`ze bgp decode` or other tools.

```
bin/ze-analyse mrt-dump test/internet/ripe-updates.*.gz | head -5
bin/ze-analyse mrt-dump test/internet/latest-bview.gz | bin/ze bgp decode -
```

## MRT File Formats

The tool handles two MRT record types (RFC 6396):

| Type | Records | Used By |
|------|---------|---------|
| TABLE_DUMP_V2 | RIB snapshots (one entry per route per peer) | attributes, communities, count-attrs, mrt-dump |
| BGP4MP | Live UPDATE messages with timestamps | density, attributes, communities, mrt-dump |

Both `.gz` and `.bz2` compressed files are supported.

## Related

- [Update Density Analysis](../architecture/update-density-analysis.md) -- empirical
  findings that inform forward pool channel sizing
- [Forward Congestion Pool](../architecture/forward-congestion-pool.md) -- the design
  that consumes these measurements
- [Congestion Industry Survey](../architecture/congestion-industry.md) -- how other
  BGP implementations handle similar problems
