# MRT Architecture

MRT (Multi-Threaded Routing Toolkit) support in Ze covers three areas:
wire format encoding/decoding, daemon-side dump generation, and offline
analysis tooling.

## Packages

| Package | Purpose |
|---------|---------|
| `internal/mrt` | Wire format library: types, encode, decode per RFC 6396/6397/8050 |
| `internal/plugins/mrt` | Daemon component: bus subscription, periodic RIB dumps, update/state streams |
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

#### BGP message decoding

The BGP messages carried inside MRT records are decoded by standalone parsers in
`internal/mrt/bgp.go` and `internal/mrt/bgp_attribute.go`, separate from the
daemon's codec in `internal/component/bgp/message`. The separation is
structural, not stylistic: `internal/component/bgp/message` is compiled out by
the `ze_bgp` feature gate, while `internal/mrt` is always-on (the MRT recorder
depends on it), so the offline parsers cannot import it.
<!-- source: feature-gates.txt — ze_bgp gates internal/component/bgp/message -->
<!-- source: internal/mrt/bgp.go — ParseBGPMessage, UpdateAttributeBytes, ParsePrefixesAFI -->

| Entry point | Decodes |
|-------------|---------|
| `ParseBGPMessage` | A complete BGP message including the 19-byte header |
| `UpdateAttributeBytes` | The raw path-attribute section of an UPDATE body |
| `ParseAttributes` | A packed path-attribute section into typed attributes |
| `ParseASPath` | AS_PATH at an explicitly supplied AS width |
| `ParsePrefixesAFI` | Packed NLRI for a given address family |
| `ParseMPReach` / `ParseMPUnreach` | Full RFC 4760 MP_REACH / MP_UNREACH |
| `ParseMPReachRIBEntry` | The abbreviated MP_REACH inside a RIB entry |
<!-- source: internal/mrt/bgp_attribute.go — MP_REACH, aggregator and community decoders -->

Two RFC 6396 constraints shape this API and are easy to get wrong:

**AS width is a property of the record, not of the bytes.** A 2-byte and a
4-byte AS_PATH can occupy the same number of octets, so the width can never be
inferred from the attribute. `ParseASPath` therefore takes it as a parameter and
callers derive it with `ASPathIsFourByte(mrtType, subtype)`: TABLE_DUMP is
2-byte (Section 4.2), TABLE_DUMP_V2 is 4-byte (Section 4.3.4), BGP4MP_MESSAGE is
2-byte (Section 4.4.2) and BGP4MP_MESSAGE_AS4 is 4-byte (Section 4.4.3).
<!-- source: internal/mrt/bgp_attribute.go — ASPathIsFourByte -->

**MP_REACH_NLRI is truncated inside RIB entries.** RFC 6396 Section 4.3.4 keeps
only the Next Hop Length and Next Hop Address; AFI, SAFI, Reserved and NLRI are
omitted because they already appear in the RIB record header. Decoding that with
the full-form parser reads the length from the wrong offset, so RIB entries use
the dedicated `ParseMPReachRIBEntry` / `ExtractNextHopRIB` entry points and BGP
UPDATE messages use `ParseMPReach` / `ExtractNextHop`.
<!-- source: internal/mrt/bgp_attribute.go — ParseMPReachRIBEntry, ExtractNextHopRIB -->

**Damage is reported, never silently swallowed.** An analysis tool that quietly
returns fewer routes than the file contains makes "this record is damaged"
indistinguishable from "this record is small", so malformed input always
produces a signal:

| Layer | Contract on malformed input |
|-------|------------------------------|
| `ParsePrefixesAFI` | Returns the prefixes decoded *before* the damage **and** an error naming the offset and offending value. The caller can salvage the good entries and still report the record as damaged. |
| `ParseMPReach` / `ParseMPUnreach` | Propagate that error, wrapped with the AFI/SAFI. |
| `ParseBGPMessage` | Propagates it; `ze-analyze show` renders the record as `[parse error]` so one damaged record is visible without aborting the file. |
| `forEachRIBEntry` | Returns the decode error; the subcommands count damaged records and print a `warning: N malformed RIB record(s) skipped` line to stderr. |

An out-of-range prefix length is never emitted as a prefix: `netip`'s zero
`Prefix` reads downstream as a default route. An unrecognized AFI yields an
error rather than an empty result, per RFC 6396 Section 4.3.3.
<!-- source: internal/mrt/bgp.go — ParsePrefixesAFI length validation and error contract -->
<!-- source: internal/analyze/mrt.go — forEachRIBEntry error return -->

## Daemon Component (`internal/plugins/mrt`)

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

`show` and `routes` decode records through the shared parsers above, deriving the
AS width from each record's type and using the RIB-entry MP_REACH decoder, so
IPv6 RIB next-hops and 2-byte-AS BGP4MP paths are reported correctly.
<!-- source: internal/analyze/routes.go — buildRouteRecord -->
<!-- source: internal/analyze/show.go — formatASPathFromAttrs, showParsedMessage -->

The record-walking helpers in `internal/analyze/mrt.go` (`forEachRIBEntry`,
`iterateAttrs`, `countAttrs`, `extractUpdateAttrs`) are thin adapters over
`internal/mrt`; the offline tools hold no second copy of the wire format.
<!-- source: internal/analyze/mrt.go — forEachRIBEntry, iterateAttrs, countAttrs, extractUpdateAttrs -->
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
