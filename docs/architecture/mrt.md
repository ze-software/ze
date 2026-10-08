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

`WritePeerIndexTable` rejects a non-UTF-8 or overlength view name before writing
any bytes and returns `(count, error)`. The decoder rejects non-UTF-8 names too.
The daemon's `writePeerIndexTable` deliberately supplies no view name and emits
an exact zero View Name Length; it does not invent a configured name.
<!-- source: internal/mrt/encode.go — WritePeerIndexTable -->
<!-- source: internal/mrt/decode.go — DecodePeerIndexTable -->
<!-- source: internal/plugins/mrt/dump.go — writePeerIndexTable -->

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
| `ParseBGPMessage` | Exactly one complete BGP message including the 19-byte header; trailing or missing bytes are errors |
| `UpdateAttributeBytes` | The raw path-attribute section of an UPDATE body |
| `ParseAttributes` | A packed path-attribute section into typed attributes, with an error for a truncated header, length or value |
| `ParseASPath` | AS_PATH at an explicitly supplied AS width |
| `ParsePrefixesAFI` | Packed NLRI for a given address family |
| `ParseMPReach` / `ParseMPUnreach` | Full RFC 4760 MP_REACH / MP_UNREACH |
| `ParseMPReachRIBEntry` | The abbreviated MP_REACH inside a RIB entry |
<!-- source: internal/mrt/bgp_attribute.go — MPReach, MPUnreach, AttrAggregator, AttrCommunity -->

Two RFC 6396 constraints shape this API and are easy to get wrong:

**AS width is a property of the record, not of the bytes.** A 2-byte and a
4-byte AS_PATH can occupy the same number of octets, so the width can never be
inferred from the attribute. `ParseASPath` therefore takes it as a parameter and
callers derive it with `ASPathIsFourByte(mrtType, subtype)`: TABLE_DUMP is
2-byte (Section 4.2), TABLE_DUMP_V2 is 4-byte (Section 4.3.4), BGP4MP_MESSAGE is
2-byte (Section 4.4.2) and BGP4MP_MESSAGE_AS4 is 4-byte (Section 4.4.3).
<!-- source: internal/mrt/bgp_attribute.go — ASPathIsFourByte -->

**ADD-PATH is negotiated per family and direction.** `MessageRecord.BGPMessage`
borrows the complete original bytes and carries the subtype hint plus immutable
context derived from both actual directional OPENs. Each reader invocation owns
its own bounded session evidence; teardown, a new OPEN epoch and identity reuse
invalidate it. The recorder's post-handshake Idle-to-Established notification
does not discard the handshake. Capture starting mid-session, updates-only
streams and independently opened rotated files cannot invent missing OPENs.
Ordinary subtypes and single-family ADD-PATH records remain unambiguous without
OPENs. A multiple-family ADD-PATH UPDATE without both OPENs returns an explicit
unavailable/ambiguous-context error, even if its bytes happen to decode under
one mode. No Path Identifier rewriting, message splitting or byte guessing is
performed. `ParseBGPMessage(record.BGPMessage)` preserves the context;
`ParsedUpdate.AddPathFor` supplies the mode for each MP attribute. Prefix arrays
have corresponding Path Identifier arrays when ADD-PATH applies. Callers MUST
copy borrowed bytes if retaining them beyond their owner's lifetime.
<!-- source: internal/mrt/decode.go — DecodeBGP4MPMessage -->
<!-- source: internal/mrt/bgp.go — BGPMessage, ParseBGPMessage -->
<!-- source: internal/mrt/context.go — SessionContexts, BGPMessage.AddPathFor -->

The no-OPEN decoder regression matrix reads each record in a fresh invocation:
all eight ordinary/ADD-PATH message subtypes under BGP4MP and BGP4MP_ET, classic
announcements and withdrawals, and isolated IPv4/IPv6 MP_REACH and MP_UNREACH.
It checks two exact prefixes per location, ADD-PATH identifiers `0` and
`0x01020304`, no identifiers for ordinary NLRI, and unchanged complete bytes.
Matching-OPEN mixed-family tests remain separate; they cannot establish the
subtype-only branch's behavior.
<!-- source: internal/mrt/rfc8050_no_open_test.go — TestRFC8050NoOPENSubtypeDecodesExactNLRI -->

An empty MP_REACH or MP_UNREACH field still names a family and can cause the
recorder to select an ADD-PATH subtype. The reader includes that family when
checking ambiguity, even though the field carries no prefixes. For example,
ordinary classic IPv4 NLRI alongside an empty IPv6 MP_UNREACH requires both
OPENs when recorded under an ADD-PATH subtype. A standalone empty EOR remains
decodable and replayable.

Content filters retain OPENs and session-control records that pass their
record/peer constraints, so selected mixed-family UPDATEs keep their real
decoding evidence. Time filters that omit a handshake cannot manufacture it.
<!-- source: internal/analyze/filter.go — runFilter -->

RFC 8050 NLRI layout, relative to each NLRI entry:

```text
Offset   ADD-PATH                    Base
0..3     Path Identifier (MSB first) Prefix Length starts at 0
4        Prefix Length
5..      Prefix octets              Prefix octets start at 1
```

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
| `ParseAttributes` | Returns complete preceding attributes together with an error for structural damage. UPDATE parsing validates the whole attribute section before resolving family modes, including when both OPENs are available. |
| `ParseBGPMessage` | Propagates it; `./le mrt show` identifies the damaged record and returns nonzero. Missing mixed-family negotiation context is an explicit error, not a partial successful decode. |
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

The RIB snapshot bridge reconstructs path attributes from pooled values. It
retains the Extended Length flag and emits the corresponding two-octet length
even for a value shorter than 256 octets; otherwise a reader would consume the
first value octet as part of the length and lose following attributes.
<!-- source: internal/component/bgp/plugins/rib/rib_mrt.go -- appendWireAttr, appendOtherAttrsWire -->

### Features

- strftime filename rotation with `%N` table name substitution (BIRD)
- Per-peer filtering (OpenBGPD)
- Direction filtering: in/out (OpenBGPD)
- Extended timestamps: BGP4MP_ET (FRR, OpenBGPD)
- Add-path aware: UPDATE subtypes follow negotiated ADD-PATH for the actual
  AFI/SAFI and direction, not another family's negotiation. The `add-path`
  setting affects RIB snapshots only; it cannot relabel captured BGP bytes.
  The raw observer receives complete original BGP messages before receive-side
  semantic validation and after successful outbound transport acceptance.
  Synthetic treat-as-withdraw messages stay on the semantic callback and never
  masquerade as received packets. Sent OPENs and other messages use LOCAL
  subtypes. The immutable directional `PeerInfo.MessageContextID` selects the
  subtype without constructing a per-message capability map; MRT copies the
  complete original header and body once into its pooled record.
  Local-AS metadata comes from the OPEN built for that connection, including
  RFC 7705 migration fallback. Both directions and a retained outbound writer
  keep that epoch's identity when configuration or a replacement session changes.
  Dispatch borrows that immutable connection identity by pointer and initializes
  only the observer fields in a call-local record. Concurrent send and receive
  callbacks cannot overwrite each other's directional context or endpoints.
<!-- source: internal/component/bgp/reactor/session_wire.go — observeReceivedWire, observedBGPWriter -->
<!-- source: internal/plugins/mrt/component.go — OnBGPMessage -->
- Collision resolution transfers the winning socket and its already-read
  original OPEN into the Peer's bounded inbound slot before tearing down the
  loser. A Peer-owned reservation prevents a third inbound connection from
  occupying the replacement session, including after the winner leaves the
  slot but before its socket is installed. The next normal session epoch
  accepts that same socket before any outgoing dial and observes the retained
  OPEN bytes unchanged. Waiting for the old Session's `Done` and then consulting
  its Peer cannot establish ownership of the replacement epoch.
<!-- source: internal/component/bgp/reactor/peer_connection.go — resolvePendingCollision, inboundConnection -->
<!-- source: internal/component/bgp/reactor/peer_run.go — runOnce -->
- On-demand CLI dump (BIRD)
- Buffered writes with configurable flush

The live collision regression also sends isolated classic and MP announcements
and withdrawals in both directions. Its OPENs negotiate IPv4 ADD-PATH only
outbound and IPv6 ADD-PATH only inbound. Each isolated packet therefore changes
between ordinary and ADD-PATH subtype if the observer uses the opposite
direction's context. Mixed-family packets alone cannot make that distinction:
the recorder's family OR can select ADD-PATH under either context.
<!-- source: internal/component/bgp/reactor/rfc8050_collision_epoch_test.go — TestMRTWinningCollisionPreservesOPEN, mrtCollisionIsolatedUpdates -->

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
- `convert pcap` -- MRT BGP4MP to pcap, IPv4 and IPv6, under `LINKTYPE_RAW` (101)
- `convert json` -- MRT record headers as JSON
- `export bmp` -- send BGP4MP records as BMP Route Monitoring to a collector
- `record bmp` -- accept incoming BMP connections, write as MRT BGP4MP
- `show` -- human-readable record dump (like bgpdump)
- `routes` -- extract prefix table as JSON (prefix, next-hop, AS path, communities)

Semantic consumers must report unavailable decoding, not silently filter it out
or present partial route counts as complete. Raw/header-only transformations may
preserve undecodable messages verbatim. `inject`, `replay` and `serve` do not
negotiate ADD-PATH: they refuse Path-ID-bearing or ambiguous UPDATEs before
transmitting them. Ordinary UPDATEs and empty EOR messages remain supported.
The BGP4MP replay guard requires the declared BGP length to equal the entire
captured message length and checks every attribute's framing before writing.
An empty UPDATE followed by another packet in the same record is refused,
as is an attribute whose declared length exceeds its section; negotiation
context cannot bypass these checks.

`convert pcap` frames each record through `internal/core/pcap`, the same writer
the diagnostic captures use. Link type 101 takes the IP family from the version
nibble, so an IPv6 record is converted like any other; it used to be counted and
dropped, because link type 228 carries IPv4 alone. Only a record whose peer or
local address is missing is now skipped, and the count is reported. The TCP
ports are fabricated as 179 at both ends, because an MRT record holds none.
<!-- source: internal/analyze/convert.go -- runConvertPcap, convertFlow -->
<!-- source: internal/core/pcap/frame.go -- Framer.WriteMessage -->

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
