# MRT Analysis

Ze includes `ze-analyze`, a standalone tool for analysing real-world BGP data
from public route collectors (RIPE RIS, RouteViews). It processes MRT dump files
to extract statistics that inform ze's internal buffer sizing, caching strategies,
and congestion handling.

<!-- source: internal/analyze/register.go -- ze-analyze CLI entry point -->

## Building

```
make ze-analyze-build
```

This produces `bin/ze-analyze`.

## Quick Start

Download BGP data from public collectors and run an analysis:

```
bin/ze-analyze download                                    # fetch latest data
bin/ze-analyze density test/internet/ripe-updates.*.gz     # UPDATE density + burst patterns
bin/ze-analyze attributes test/internet/latest-bview.gz    # attribute repetition analysis
```

## Data Sources

`ze-analyze download` fetches MRT files from two public BGP collectors:

| Source | Type | Interval | Size |
|--------|------|----------|------|
| RIPE RIS rrc00 (Amsterdam) | BGP4MP updates | 5 min | ~5 MB per file |
| RIPE RIS rrc00 | TABLE_DUMP_V2 RIB | Latest | ~400 MB |
| RouteViews route-views2 | BGP4MP updates | 15 min | ~2 MB per file |
| RouteViews route-views2 | TABLE_DUMP_V2 RIB | 2-hour intervals | ~100 MB |

<!-- source: internal/analyze/download.go -- download URLs and conversion -->

Files are saved to `test/internet/` (gitignored). RouteViews bz2 files are
converted to gzip on download for Go stdlib compatibility.

```
bin/ze-analyze download                     # today's data at 00:00 UTC
bin/ze-analyze download 20260324            # specific date
bin/ze-analyze download 20260324 1200       # specific date and time
bin/ze-analyze download -o /tmp/mrt         # custom output directory
```

## Commands

### density

Measures how many NLRIs each UPDATE carries and how many UPDATEs arrive per
second. Separates traffic into setup (table dumps, convergence) and maintenance
(steady-state churn) using per-source-peer burst detection.

<!-- source: internal/analyze/density.go -- NLRI counting and burst detection -->

```
bin/ze-analyze density test/internet/ripe-updates.*.gz
```

**Output sections:**
- NLRIs per UPDATE distribution (announced, withdrawn, total)
- UPDATEs per active second distribution
- Setup vs maintenance classification per source peer
- Per-peer maintenance rate distribution
- Channel sizing recommendation with empirical P50/P95/P99

**Used for:** per-peer forward pool channel sizing. Results documented in
[Update Density Analysis](https://github.com/ze-software/ze/blob/main/docs/architecture/update-density-analysis.md).

### attributes

Analyses attribute repetition across routes to guide caching decisions. Measures
per-attribute cache hit rates, bundle deduplication effectiveness, and temporal
locality (consecutive identical bundles).

<!-- source: internal/analyze/attributes.go -- bundle hashing and community extraction -->

```
bin/ze-analyze attributes test/internet/latest-bview.gz 2>/dev/null | jq .   # JSON
bin/ze-analyze attributes test/internet/latest-bview.gz >/dev/null           # summary
```

**Output:** JSON to stdout, human summary to stderr.

**Used for:** attribute pool sizing, cache strategy decisions. Results documented
in [mrt-attribute-caching.md](https://github.com/ze-software/ze/blob/main/docs/research/mrt-attribute-caching.md).

### communities

Identifies per-ASN community defaults: communities that appear in nearly every
route from a given ASN. These defaults can be assumed present in a cache,
encoding only exceptions (absent defaults) to save wire bytes.

<!-- source: internal/analyze/communities.go -- per-ASN frequency analysis -->

```
bin/ze-analyze communities test/internet/latest-bview.gz
bin/ze-analyze communities --threshold 0.90 --format json test/internet/latest-bview.gz
bin/ze-analyze communities --post-policy test/internet/latest-bview.gz
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
bin/ze-analyze count-attrs test/internet/latest-bview.gz
```

### mrt-dump

Dumps MRT records as BGP UPDATE hex, one per line. Useful for piping into
`ze bgp decode` or other tools.

```
bin/ze-analyze mrt-dump test/internet/ripe-updates.*.gz | head -5
bin/ze-analyze mrt-dump test/internet/latest-bview.gz | bin/ze bgp decode -
```

### show

Human-readable MRT dump (like bgpdump). Displays record headers, peer info, and
decoded BGP message contents including attributes, AS paths, and prefixes.

<!-- source: internal/analyze/show.go -- human-readable MRT dump -->

```
bin/ze-analyze show test/internet/ripe-updates.*.gz | head -50
bin/ze-analyze show test/internet/latest-bview.gz
```

### routes

Extracts a prefix table from TABLE_DUMP_V2 files as JSON. Each entry includes
prefix, next-hop, AS path, origin, local-pref, MED, and communities.

<!-- source: internal/analyze/routes.go -- prefix table extraction -->

```
bin/ze-analyze routes test/internet/latest-bview.gz | jq '.[] | select(.prefix == "1.0.0.0/24")'
```

### inject

Opens a BGP session to a remote peer and sends routes from an MRT file.
Supports both TABLE_DUMP_V2 (RIB entries) and BGP4MP (UPDATE messages).

<!-- source: internal/analyze/inject.go -- BGP session injection -->

```
bin/ze-analyze inject --local-as 65000 test/internet/latest-bview.gz 10.0.0.1:179
```

### replay

Replays BGP4MP messages over a BGP session preserving original inter-message
timing. Configurable speed multiplier.

<!-- source: internal/analyze/replay.go -- timed BGP4MP replay -->

```
bin/ze-analyze replay --local-as 65000 --speed 10 test/internet/ripe-updates.*.gz 10.0.0.1:179
```

### convert

Converts MRT records to other formats.

<!-- source: internal/analyze/convert.go -- format conversion -->

```
bin/ze-analyze convert pcap test/internet/ripe-updates.*.gz output.pcap      # BGP4MP to pcap (IPv4 only)
bin/ze-analyze convert json test/internet/ripe-updates.*.gz | jq .           # record headers as JSON
```

### export

Send MRT data to network targets.

<!-- source: internal/analyze/export_bmp.go -- MRT to BMP export -->

```
bin/ze-analyze export bmp --target 10.0.0.1:4321 test/internet/ripe-updates.*.gz
bin/ze-analyze export bmp --target collector:4321 --peer-ip 10.0.0.1 test/internet/ripe-updates.*.gz
```

Connects to a BMP collector and sends each BGP4MP message as a BMP Route
Monitoring message. Optional `--peer-ip` filters to a single peer.

### record

Record incoming protocol streams to MRT files.

<!-- source: internal/analyze/record_bmp.go -- BMP to MRT recording -->

```
bin/ze-analyze record bmp --listen :4321 output.mrt
```

Listens for incoming BMP (RFC 7854) connections. Received Route Monitoring
messages are written as BGP4MP_MESSAGE_AS4 MRT records. Peer Up/Down
notifications are written as BGP4MP_STATE_CHANGE_AS4. Multiple concurrent
BMP connections are supported (writes are serialized).

### serve

Passive BGP server that sends MRT file contents to any peer that connects.
Useful for IXP traffic replay testing: blast an entire routing table at a
router and observe its behavior.

<!-- source: internal/analyze/serve.go -- MRT-to-BGP server -->

```
bin/ze-analyze serve --local-as 65000 --listen :1179 test/internet/latest-bview.gz
bin/ze-analyze serve --local-as 65000 --per-peer test/internet/ripe-updates.*.gz
```

With `--per-peer`, only records matching the connecting peer's ASN are sent.
Multiple MRT files can be specified; all are sent sequentially.

### statistics

Per-type/subtype counts, AFI breakdown, peer summary, timestamp range, and
BGP message type distribution.

<!-- source: internal/analyze/statistics.go -- MRT statistics -->

```
bin/ze-analyze statistics test/internet/ripe-updates.*.gz
```

### filter

Select records by peer IP, peer ASN, prefix, AS-path regex, community regex,
MRT type, or timestamp range. Writes matching records verbatim (no re-encoding)
to a new MRT file. Multiple filters are AND-composed.

<!-- source: internal/analyze/filter.go -- MRT record filtering -->

```
bin/ze-analyze filter --peer-asn 13335 test/internet/latest-bview.gz cloudflare.mrt
bin/ze-analyze filter --prefix 1.0.0.0/24 --after 1780272000 test/internet/ripe-updates.*.gz filtered.mrt
bin/ze-analyze filter --as-path "174 .* 13335" test/internet/latest-bview.gz transit.mrt
bin/ze-analyze filter --community "13335:" test/internet/latest-bview.gz tagged.mrt
```

AS-path regex matches against space-separated ASNs (e.g. `"174 1916 52888"`).
AS_SET segments are rendered as `{asn,asn}`. Community regex matches per-community
strings: standard `high:low`, large `global:local1:local2`, extended `high:low`.

## Damaged Input

A dump from a public collector can carry a truncated or corrupt record. The tool
reports the damage instead of printing a short result as fact.

| What you get | Where |
|--------------|-------|
| `warning: N malformed MRT record(s) skipped or partially decoded; results are incomplete` | stderr, from `density`, `attributes`, `aspath`, `communities`, `count-attrs`, `mrt-dump`, `routes` and `show`. A report that scrolls prints it before the numbers it qualifies. A clean file prints nothing. |
| `HH:MM:SS <peer> [unparseable: truncated] <error>` | `show`, for a record that decoded to nothing. The tag is `truncated`, `unsupported-afi`, or `damaged`. |
| `A=3+` or `W=12+` | `show`, for a count that is partial. The `+` says "at least this many"; a count with no `+` is exact. A damaged record prints the field even at zero, so a missing count and a zero count stay distinct. |
| `mrt: truncated record 41902 (type 13 subtype 2, timestamp 1780272000): unexpected EOF` | stderr, when the file itself stops early. The record ordinal is 1-based over the stream, so the failing record can be found in a multi-gigabyte dump. |

Damage never discards what already decoded. A record with 500 good prefixes and
one truncated prefix reports the 500 and counts one damaged record.

<!-- source: internal/analyze/mrt.go -- malformedCounter, damageTag -->
<!-- source: internal/mrt/reader.go -- readRecords -->
<!-- source: internal/analyze/show.go -- mpReachCount, mpUnreachCount -->

## AS Width and Add-Path

Two properties of an MRT record decide how its BGP payload reads, and neither can
be inferred from the payload bytes:

- **AS width comes from the record type** (RFC 6396). TABLE_DUMP is 2-byte
  (Section 4.2), TABLE_DUMP_V2 is 4-byte (Section 4.3.4), BGP4MP_MESSAGE is
  2-byte (Section 4.4.2), and BGP4MP_MESSAGE_AS4 is 4-byte (Section 4.4.3). A
  2-byte and a 4-byte AS_PATH can occupy the same number of octets, so a wrong
  width yields fictitious ASNs rather than an error. Every subcommand derives the
  width from the record it is reading.
- **Add-path dumps are decoded.** The RFC 8050 TABLE_DUMP_V2 RIB subtypes 8 to 12
  and the add-path BGP4MP subtypes 8 to 11 carry a Path Identifier before each
  prefix. They are dispatched like the non-add-path subtypes, so an add-path dump
  yields its routes.

<!-- source: internal/mrt/bgp_attribute.go -- ASPathIsFourByte -->
<!-- source: internal/mrt/types.go -- IsAddPathRIBSubtype, IsAddPathBGP4MPSubtype -->

## MRT File Formats

The tool handles two MRT record types (RFC 6396):

| Type | Records | Used By |
|------|---------|---------|
| TABLE_DUMP_V2 | RIB snapshots (one entry per route per peer) | attributes, communities, count-attrs, mrt-dump, show, routes, inject, filter |
| BGP4MP | Live UPDATE messages with timestamps | density, attributes, communities, mrt-dump, show, inject, replay, convert, filter |

Both `.gz` and `.bz2` compressed files are supported. HTTP and HTTPS URLs are
accepted anywhere a file path is expected; compression is auto-detected from
the URL suffix.

```
bin/ze-analyze statistics https://data.ris.ripe.net/rrc00/2026.06/updates.20260607.0000.gz
```

## Daemon MRT Recording

Ze produces MRT dumps from live BGP sessions via the `mrt` component. Configure
under `mrt {}` in the YANG config:

<!-- source: internal/plugins/mrt/yang/ze-mrt-conf.yang -- YANG config schema -->
<!-- source: internal/plugins/mrt/component.go -- daemon MRT component -->

Three independent streams (following FRR's model):
1. **Updates** -- BGP4MP records for UPDATE messages only
2. **All** -- BGP4MP records for all BGP messages + state changes
3. **Routes** -- periodic TABLE_DUMP_V2 RIB snapshots

Features: per-peer filtering, direction filtering (received/sent), extended
timestamps (BGP4MP_ET), add-path aware, on-demand CLI dump via
`request mrt dump-rib`, strftime filename rotation, async non-blocking writes.

## Related

- [MRT Architecture](https://github.com/ze-software/ze/blob/main/docs/architecture/mrt.md) -- package structure and design
- [MRT Implementation Comparison](https://github.com/ze-software/ze/blob/main/docs/research/mrt-implementation-comparison.md) -- feature
  comparison across BGP implementations
- [Update Density Analysis](https://github.com/ze-software/ze/blob/main/docs/architecture/update-density-analysis.md) -- empirical
  findings that inform forward pool channel sizing
- [Forward Congestion Pool](https://github.com/ze-software/ze/blob/main/docs/architecture/forward-congestion-pool.md) -- the design
  that consumes these measurements
- [Congestion Industry Survey](https://github.com/ze-software/ze/blob/main/docs/architecture/congestion-industry.md) -- how other
  BGP implementations handle similar problems
