# Handover: MRT Deep Decode (BGP Message Parsing Inside MRT Records)

## Goal

Parse BGP messages inside MRT records the same way Ze parses live BGP wire
messages. Today `internal/mrt/decode.go` returns `MessageRecord.BGPMessage []byte`
as an opaque blob. The goal is to bridge that to Ze's BGP message parsers
(`internal/component/bgp/message/`) so MRT analysis tools can extract NLRI,
path attributes, communities, AS_PATH, next-hop, etc.

## Current State

### What exists

**MRT side (internal/mrt/):**
- `decode.go:DecodeBGP4MPMessage` returns `*MessageRecord` with `BGPMessage []byte` (complete BGP message including 16-byte marker + 2-byte length + 1-byte type)
- `decode.go:DecodeRIBRecord` returns `*RIBRecord` with `[]RIBEntry`, each containing `Attributes []byte` (path attributes only, AS_PATH in 4-byte encoding)
- RIB entries have a special MP_REACH_NLRI encoding: only Next Hop Length + Next Hop Address (AFI/SAFI/NLRI/Reserved omitted per RFC 6396 Section 4.3.4)
- `reader.go:Handler` dispatches to `OnMessage` / `OnRIB` / `OnTableDump` callbacks

**BGP message parsers (internal/component/bgp/message/):**
- `ParseHeader(data []byte) (Header, error)` -- validates marker, extracts length + type
- `Header.Type` is `MessageType` (1=OPEN, 2=UPDATE, 3=NOTIFICATION, 4=KEEPALIVE, 5=ROUTE_REFRESH)
- `Splitter.Split()` can chunk UPDATEs but is a builder tool, not a parser
- No `ParseUpdate(body []byte)` function exists -- UPDATE parsing is done inline in `reactor_wire.go` via `WireUpdate` lazy parsing
- `rfc7606.go` has attribute validation logic

**Analyze side (internal/analyze/mrt.go) -- hand-rolled parsers:**
- `extractUpdateAttrs(update []byte)` -- returns path attributes from UPDATE body (skips withdrawn + withdrawn_len + attr_len)
- `iterateAttrs(attrs, fn)` -- iterates individual attributes by type code
- `countAttrs(attrs)` -- counts attributes
- `forEachRIBEntry(data, subtype, fn)` -- iterates RIB entries (predates internal/mrt/decode.go)

### The gap

The MRT decoder produces structured records with raw BGP bytes. The BGP message
package has parsing for headers but not a standalone UPDATE parser. The analyze
package has hand-rolled attribute iteration that duplicates what the message
package should provide. There is no unified "parse this BGP message from an MRT
record" function.

## What Needs Building

### 1. BGP message parser for offline use

A function like:
```
// ParseBGPMessage parses a complete BGP message (with header) into structured form.
func ParseBGPMessage(data []byte) (*ParsedMessage, error)
```

Where `ParsedMessage` contains:
- Message type (OPEN/UPDATE/NOTIFICATION/KEEPALIVE/ROUTE_REFRESH)
- For UPDATE: withdrawn prefixes, path attributes (parsed), announced NLRI
- For OPEN: version, ASN, hold time, BGP ID, capabilities
- For NOTIFICATION: error code, subcode, data

This could live in `internal/mrt/` (MRT-specific) or better in
`internal/component/bgp/message/` as a general-purpose BGP message parser
that both the daemon and offline tools can use.

### 2. Attribute parser for offline use

The existing `iterateAttrs` in analyze/mrt.go needs to be generalized or
replaced. The attribute package (`internal/component/bgp/attribute/`) has
type-specific parsers but they're coupled to the daemon's pool/dedup
infrastructure. A lightweight offline parser would:
- Extract ORIGIN, AS_PATH, NEXT_HOP, MED, LOCAL_PREF, COMMUNITIES, etc.
- Handle both 2-byte and 4-byte AS_PATH (MRT TABLE_DUMP uses 2-byte, TABLE_DUMP_V2 uses 4-byte)
- Handle the truncated MP_REACH_NLRI in TABLE_DUMP_V2 RIB entries

### 3. Integration with ze-analyse subcommands

The existing subcommands (statistics, filter, aspath, communities, attributes,
density) could use the structured parser instead of hand-rolling attribute
extraction. New subcommands enabled by deep parsing:
- `ze-analyse show <file.mrt>` -- human-readable dump of every record (like bgpdump)
- `ze-analyse routes <file.mrt>` -- extract prefix table (prefix, next-hop, AS_PATH, communities)
- Enhanced `filter` with AS_PATH regex, community match, next-hop filter

## Key Constraints

| Constraint | Source |
|------------|--------|
| RIB entry MP_REACH_NLRI is truncated (Next Hop only) | RFC 6396 Section 4.3.4 |
| TABLE_DUMP v1 AS_PATH uses 2-byte AS only | RFC 6396 Section 4.2 |
| TABLE_DUMP_V2 AS_PATH uses 4-byte AS only | RFC 6396 Section 4.3.4 |
| BGP4MP_MESSAGE AS_PATH uses 2-byte AS | RFC 6396 Section 4.4.2 |
| BGP4MP_MESSAGE_AS4 AS_PATH uses 4-byte AS | RFC 6396 Section 4.4.3 |
| Offline tool, allocations are fine | docs/architecture/mrt.md |
| Daemon attribute parsers are pool-coupled | internal/component/bgp/attribute/ |

## Architecture Decision Needed

**Option A: Extend internal/component/bgp/message/ with ParseUpdate/ParseOpen.**
Pro: single source of truth for BGP parsing. Con: those parsers are daemon-oriented
(WireUpdate, pool buffers); adding offline-friendly APIs may conflict.

**Option B: Build standalone parsers in internal/mrt/bgp.go.**
Pro: clean separation, offline-friendly, no daemon dependencies. Con: some
duplication with the daemon's attribute code.

**Option C: Build in internal/analyze/ (extend existing hand-rolled parsers).**
Pro: minimal change, already works. Con: keeps the duplication, doesn't benefit
the daemon or future MRT tools.

Recommended: **Option B** -- `internal/mrt/bgp.go` with standalone parsers that
operate on `[]byte` and return structured types. The daemon's parsers are
optimized for zero-copy streaming; the offline parsers optimize for clarity and
completeness. The overlap is small (attribute type dispatch) and worth paying
for clean separation.

## Files to Read Before Starting

| File | Why |
|------|-----|
| `internal/mrt/decode.go` | Current MRT decoder output types |
| `internal/analyze/mrt.go:225-290` | Existing hand-rolled attribute parsers |
| `internal/component/bgp/message/header.go` | BGP header parsing (reusable) |
| `internal/component/bgp/message/rfc7606.go` | Attribute validation (reference) |
| `internal/component/bgp/attribute/` | Attribute type definitions |
| `rfc/short/rfc6396.md` | MP_REACH_NLRI truncation rule |
| `rfc/short/rfc4271.md` (if exists) | UPDATE message format |

## Estimated Scope

- `internal/mrt/bgp.go` -- ~300-400 lines: ParseBGPMessage, ParseUpdate, ParseOpen, ParseNotification, ParseAttributes, attribute extractors
- `internal/mrt/bgp_test.go` -- ~200 lines: round-trip tests with real BGP messages
- `internal/analyze/` -- refactor existing subcommands to use new parsers (aspath.go, communities.go, attributes.go, dump.go)
- New `internal/analyze/show.go` -- human-readable MRT dump (like bgpdump)
- New `internal/analyze/routes.go` -- prefix table extraction
