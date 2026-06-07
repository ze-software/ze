# MRT Architecture

MRT (Multi-Threaded Routing Toolkit) support in Ze covers three areas:
wire format encoding/decoding, daemon-side dump generation, and offline
analysis tooling.

## Packages

| Package | Purpose |
|---------|---------|
| `internal/mrt` | Wire format library: types, encode, decode per RFC 6396/6397/8050 |
| `internal/component/mrt` | Daemon component: bus subscription, periodic RIB dumps, update/state streams |
| `internal/analyze` | Offline tools: parse, filter, statistics, inject, convert |

## Wire Format Library (`internal/mrt`)

Shared by both daemon (write) and analysis (read) paths.

### Encoding (write side)

Follows buffer-first rules. All encoders use `WriteTo(buf []byte, off int) int`.
The daemon component provides pooled buffers; encoders never allocate.

Key encoder functions:
- `WriteHeader` / `WriteExtendedHeader` -- common/ET header
- `WritePeerIndexTable` -- PEER_INDEX_TABLE subtype
- `WriteRIBEntry` / `WriteRIBEntryAddPath` -- RIB entries with/without Path ID
- `WriteBGP4MPMessage` / `WriteBGP4MPStateChange` -- BGP4MP records
- `WriteTableDumpV2Header` -- sequence + prefix for AFI-specific subtypes
- `WriteRIBGenericHeader` -- sequence + AFI/SAFI + NLRI

### Decoding (read side)

Used by `internal/analyze` for offline parsing. Allocates freely (offline tool).
Returns structured types from byte slices.

## Daemon Component (`internal/component/mrt`)

Registers as a Ze component. Subscribes to:
- BGP update events (for BGP4MP update stream)
- BGP state change events (for BGP4MP_STATE_CHANGE records)
- Periodic timer (for TABLE_DUMP_V2 RIB snapshots)

### Dump Streams

Three independent streams (following FRR model):
1. **Updates** -- BGP4MP records for UPDATE messages only
2. **All** -- BGP4MP records for all BGP messages + state changes
3. **Routes** -- periodic TABLE_DUMP_V2 RIB snapshots

Each stream has its own file path (with strftime patterns), interval, and
enable/disable state.

### Features

- strftime filename rotation with `%N` table name substitution (BIRD)
- Per-peer filtering (OpenBGPD)
- Direction filtering: in/out (OpenBGPD)
- Extended timestamps: BGP4MP_ET (FRR, OpenBGPD)
- Add-path aware: auto-selects ADDPATH subtypes (all implementations)
- On-demand CLI dump (BIRD)
- Buffered writes with configurable flush

## Analysis Tooling (`internal/analyze`)

Extends existing MRT parser with:
- Add-path subtypes (8-12)
- STATE_CHANGE subtypes (0, 5)
- TABLE_DUMP v1 (type 12)
- GEO_PEER_TABLE (subtype 7)

New subcommands:
- `statistics` -- per-type/subtype counts, AFI breakdown, peer summary
- `filter` -- select by prefix, peer, ASN, AS-path regex, community regex, timestamp, type
- `inject` -- open BGP session to remote peer, send TABLE_DUMP_V2/BGP4MP UPDATEs
- `replay` -- replay BGP4MP messages over BGP session preserving timing
- `convert pcap` -- MRT BGP4MP to pcap (IPv4 only, IPv6 skipped)
- `convert json` -- MRT record headers as JSON
- `export bmp` -- send BGP4MP records as BMP Route Monitoring to a collector
- `record bmp` -- accept incoming BMP connections, write as MRT BGP4MP
- `show` -- human-readable record dump (like bgpdump)
- `routes` -- extract prefix table as JSON (prefix, next-hop, AS path, communities)
- `serve` -- passive BGP server serving MRT file contents to connecting peers

HTTP/HTTPS URL input is supported anywhere a file path is accepted.
Compression (gz/bz2) is auto-detected from the URL suffix.

## RFCs

- RFC 6396: MRT base format (types, TABLE_DUMP, TABLE_DUMP_V2, BGP4MP)
- RFC 6397: GEO_PEER_TABLE extension
- RFC 8050: Add-path extensions (ADDPATH subtypes)

## References

- `rfc/short/rfc6396.md` -- wire format summary with all diagrams
- `rfc/short/rfc6397.md` -- geo extension summary
- `rfc/short/rfc8050.md` -- add-path extension summary
- `docs/research/mrt-implementation-comparison.md` -- feature comparison across implementations
