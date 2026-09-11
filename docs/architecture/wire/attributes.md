# BGP Path Attributes Wire Format

## TL;DR (Read This First)

| Concept | Description |
|---------|-------------|
| **Header** | Flags (1) + Type (1) + Length (1-2 bytes) |
| **Flags** | 0x80=Optional, 0x40=Transitive, 0x20=Partial, 0x10=ExtLen |
| **Key Codes** | 1=ORIGIN, 2=AS_PATH, 3=NEXT_HOP, 4=MED, 5=LOCAL_PREF |
| **ASN4 Impact** | AS_PATH uses 2-byte ASNs by default, 4-byte with ASN4 cap |
| **Key Types** | `Attribute` interface, `AttributeCode`, `AttributeFlags` |

**When to read full doc:** Attribute parsing, new attribute types, ASN4 encoding.
<!-- source: internal/core/bgp/attribute/attribute.go -- AttributeCode, AttributeFlags, Attribute interface -->

---

**Source:** RFC 4271, various RFCs, ExaBGP `bgp/message/update/attribute/`
**Purpose:** Document wire format for all BGP path attributes

---

## Attribute Header Format

All path attributes share a common header:

```
 0                   1                   2
 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|  Attr. Flags  |  Attr. Type   |    Length     |  (1 byte length)
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
         or
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|  Attr. Flags  |  Attr. Type   |         Length (2 bytes)      |  (extended length)
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
```
<!-- source: internal/core/bgp/attribute/attribute.go -- ParseHeader, WriteHeaderTo -->

### Attribute Flags

```
 0 1 2 3 4 5 6 7
+-+-+-+-+-+-+-+-+
|O|T|P|E|  Rsv  |
+-+-+-+-+-+-+-+-+
```

| Bit | Name | Value | Description |
|-----|------|-------|-------------|
| 0 | Optional | 0x80 | 1=Optional, 0=Well-known |
| 1 | Transitive | 0x40 | 1=Transitive, 0=Non-transitive |
| 2 | Partial | 0x20 | 1=Partial (set by non-originating AS) |
| 3 | Extended Length | 0x10 | 1=2-byte length, 0=1-byte length |
| 4-7 | Reserved | | Must be 0 |
<!-- source: internal/core/bgp/attribute/attribute.go -- FlagOptional, FlagTransitive, FlagPartial, FlagExtLength -->

### Flag Combinations

| Attribute Type | Flags |
|----------------|-------|
| Well-known Mandatory | 0x40 (Transitive) |
| Well-known Discretionary | 0x40 (Transitive) |
| Optional Transitive | 0xC0 (Optional + Transitive) |
| Optional Non-transitive | 0x80 (Optional only) |

---

## Attribute Type Codes

<!-- source: internal/core/bgp/attribute/attribute.go -- AttributeCode constants -->

| Code | Hex | Name | Flags | RFC | Status |
|------|-----|------|-------|-----|--------|
| 1 | 0x01 | ORIGIN | 0x40 (WK-M) | RFC 4271 | implemented |
| 2 | 0x02 | AS_PATH | 0x40 (WK-M) | RFC 4271 | implemented |
| 3 | 0x03 | NEXT_HOP | 0x40 (WK-M) | RFC 4271 | implemented |
| 4 | 0x04 | MULTI_EXIT_DISC | 0x80 (O-NT) | RFC 4271 | implemented |
| 5 | 0x05 | LOCAL_PREF | 0x40 (WK-D) | RFC 4271 | implemented |
| 6 | 0x06 | ATOMIC_AGGREGATE | 0x40 (WK-D) | RFC 4271 | implemented |
| 7 | 0x07 | AGGREGATOR | 0xC0 (O-T) | RFC 4271 | implemented |
| 8 | 0x08 | COMMUNITY | 0xC0 (O-T) | RFC 1997 | implemented |
| 9 | 0x09 | ORIGINATOR_ID | 0x80 (O-NT) | RFC 4456 | implemented |
| 10 | 0x0A | CLUSTER_LIST | 0x80 (O-NT) | RFC 4456 | implemented |
| 14 | 0x0E | MP_REACH_NLRI | 0x80 (O-NT) | RFC 4760 | implemented |
| 15 | 0x0F | MP_UNREACH_NLRI | 0x80 (O-NT) | RFC 4760 | implemented |
| 16 | 0x10 | EXTENDED_COMMUNITY | 0xC0 (O-T) | RFC 4360 | implemented |
| 17 | 0x11 | AS4_PATH | 0xC0 (O-T) | RFC 6793 | implemented |
| 18 | 0x12 | AS4_AGGREGATOR | 0xC0 (O-T) | RFC 6793 | implemented |
| 22 | 0x16 | PMSI_TUNNEL | 0xC0 (O-T) | RFC 6514 | not implemented |
| 23 | 0x17 | TUNNEL_ENCAP | 0xC0 (O-T) | RFC 9012 | parsed (on-demand sub-TLVs) |
| 25 | 0x19 | IPV6_EXT_COMMUNITY | 0xC0 (O-T) | RFC 5701 | implemented |
| 26 | 0x1A | AIGP | 0x80 (O-NT) | RFC 7311 | implemented |
| 29 | 0x1D | BGP_LS | 0x80 (O-NT) | RFC 7752 | not implemented |
| 32 | 0x20 | LARGE_COMMUNITY | 0xC0 (O-T) | RFC 8092 | implemented |
| 40 | 0x28 | BGP_PREFIX_SID | 0xC0 (O-T) | RFC 8669 | not implemented |
| 252 | 0xFC | ATTR_TOMBSTONE | 0x80/0xC0 (O, T mirrors discarded attr) | draft-mangin-idr-attr-tombstone-00 | marker implemented, Section 5.3 egress clear not implemented (provisional code point) |

Legend: WK=Well-known, O=Optional, M=Mandatory, D=Discretionary, T=Transitive, NT=Non-transitive.
Unimplemented attributes are parsed as opaque (raw bytes preserved for forwarding).

---

## 1. ORIGIN (Code 1)

RFC 4271 - Origin of the path information.

```
+-+-+-+-+-+-+-+-+
|    Origin     |
+-+-+-+-+-+-+-+-+
```

| Value | Name | Description |
|-------|------|-------------|
| 0 | IGP | Originated in IGP |
| 1 | EGP | Originated in EGP |
| 2 | INCOMPLETE | Unknown origin |

**Length:** 1 byte
**Flags:** 0x40 (Well-known Mandatory)
<!-- source: internal/core/bgp/attribute/attribute.go -- AttrOrigin -->

---

## 2. AS_PATH (Code 2)

RFC 4271 - Sequence of ASNs traversed.

```
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
| Segment Type  | Segment Length|
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|     AS Number (2 or 4 bytes)  |  (repeated Segment Length times)
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
```

### Segment Types

| Type | Name | Notation |
|------|------|----------|
| 1 | AS_SET | [ ] |
| 2 | AS_SEQUENCE | ( ) |
| 3 | AS_CONFED_SEQUENCE | {( )} |
| 4 | AS_CONFED_SET | {[ ]} |

**AS Size:** 2 bytes without ASN4 capability, 4 bytes with ASN4
**Max Segment Length:** 255 ASNs per segment
<!-- source: internal/core/bgp/attribute/aspath.go -- ASPathSegmentType, ASSet, ASSequence, ASConfedSequence -->

### Example

AS_PATH: (65001 65002 65003) [65004 65005]
```
02 03 00FDE9 00FDEA 00FDEB  // SEQUENCE: 65001, 65002, 65003
01 02 00FDEC 00FDED          // SET: 65004, 65005
```

---

## 3. NEXT_HOP (Code 3)

RFC 4271 - Next hop IP address (IPv4 only in traditional UPDATE).

```
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                      IPv4 Address                             |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
```

**Length:** 4 bytes (IPv4)
**Flags:** 0x40 (Well-known Mandatory)

Note: For IPv6 and other families, next hop is in MP_REACH_NLRI.
<!-- source: internal/core/bgp/attribute/attribute.go -- AttrNextHop -->

---

## 4. MULTI_EXIT_DISC (MED) (Code 4)

RFC 4271 - Multi-exit discriminator for external links.

```
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                           MED Value                           |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
```

**Length:** 4 bytes
**Flags:** 0x80 (Optional Non-transitive)

Lower MED is preferred.
<!-- source: internal/core/bgp/attribute/attribute.go -- AttrMED -->

---

## 5. LOCAL_PREF (Code 5)

RFC 4271 - Local preference (IBGP only).

```
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                       Local Preference                        |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
```

**Length:** 4 bytes
**Flags:** 0x40 (Well-known Discretionary)

Higher LOCAL_PREF is preferred. Default: 100.
<!-- source: internal/core/bgp/attribute/attribute.go -- AttrLocalPref -->

---

## 6. ATOMIC_AGGREGATE (Code 6)

RFC 4271 - Indicates route is an aggregate.

```
[Empty - Length = 0]
```

**Length:** 0 bytes
**Flags:** 0x40 (Well-known Discretionary)

Presence indicates the route is less specific than component routes.
<!-- source: internal/core/bgp/attribute/attribute.go -- AttrAtomicAggregate -->

---

## 7. AGGREGATOR (Code 7)

RFC 4271 - AS and router ID that performed aggregation.

```
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|          AS Number            |       BGP Identifier          |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|     (BGP Identifier cont.)    |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
```

**Length:** 6 bytes (2-byte AS) or 8 bytes (4-byte AS)
**Flags:** 0xC0 (Optional Transitive)
<!-- source: internal/core/bgp/attribute/attribute.go -- AttrAggregator -->

---

## 8. COMMUNITY (Code 8)

RFC 1997 - Community values for policy.

```
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                        Community Value                        |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
```

**Length:** 4 bytes per community (variable total)
**Flags:** 0xC0 (Optional Transitive)

### Well-Known Communities

| Value | Name |
|-------|------|
| 0xFFFF0000 | GRACEFUL_SHUTDOWN |
| 0xFFFFFF01 | NO_EXPORT |
| 0xFFFFFF02 | NO_ADVERTISE |
| 0xFFFFFF03 | NO_EXPORT_SUBCONFED |

### Format

Communities displayed as AS:Value (e.g., 65001:100)
<!-- source: internal/core/bgp/attribute/community.go -- CommunityNoExport, CommunityNoAdvertise, CommunityNoExportSubconfed -->

---

## 9. ORIGINATOR_ID (Code 9)

RFC 4456 - Router ID of route reflector client origin.

```
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                       Originator ID                           |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
```

**Length:** 4 bytes
**Flags:** 0x80 (Optional Non-transitive)
<!-- source: internal/core/bgp/attribute/attribute.go -- AttrOriginatorID -->

---

## 10. CLUSTER_LIST (Code 10)

RFC 4456 - List of route reflector cluster IDs.

```
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                        Cluster ID                             |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                         (repeated)                            |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
```

**Length:** 4 bytes per cluster ID (variable total)
**Flags:** 0x80 (Optional Non-transitive)
<!-- source: internal/core/bgp/attribute/attribute.go -- AttrClusterList -->

---

## 14. MP_REACH_NLRI (Code 14)

RFC 4760 - Multiprotocol reachable NLRI.

```
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|      AFI      |     SAFI      |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
| Next Hop Len  |   Network Address of Next Hop |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
| Reserved      |   NLRI (variable)             |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
```

| Field | Bytes | Description |
|-------|-------|-------------|
| AFI | 2 | Address Family |
| SAFI | 1 | Sub-Address Family |
| Next Hop Len | 1 | Length of next hop address |
| Next Hop | Variable | Next hop address(es) |
| Reserved | 1 | Must be 0 |
| NLRI | Variable | Network reachability info |

**Flags:** 0x80 (Optional Non-transitive)

### Next Hop Lengths

| AFI/SAFI | Next Hop Len | Description |
|----------|--------------|-------------|
| IPv4 Unicast | 4 | Single IPv4 |
| IPv6 Unicast | 16 | Single IPv6 |
| IPv6 Unicast | 32 | IPv6 + link-local IPv6 |
| VPNv4 | 12 | RD (8) + IPv4 (4) |
| VPNv6 | 24 | RD (8) + IPv6 (16) |
<!-- source: internal/core/bgp/attribute/attribute.go -- AttrMPReachNLRI -->

---

## 15. MP_UNREACH_NLRI (Code 15)

RFC 4760 - Multiprotocol unreachable NLRI.

```
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|      AFI      |     SAFI      |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|   Withdrawn Routes (variable)                 |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
```

**Flags:** 0x80 (Optional Non-transitive)

No next hop - only withdrawn routes.
<!-- source: internal/core/bgp/attribute/attribute.go -- AttrMPUnreachNLRI -->

---

## 16. EXTENDED_COMMUNITY (Code 16)

RFC 4360 - Extended community values.

```
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|  Type (high)  |  Type (low)   |         Value                 |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                          Value (cont.)                        |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
```

**Length:** 8 bytes per community
**Flags:** 0xC0 (Optional Transitive)

### Type Codes (High Octet)

| High | Name |
|------|------|
| 0x00 | Two-Octet AS Specific |
| 0x01 | IPv4 Address Specific |
| 0x02 | Four-Octet AS Specific |
| 0x03 | Opaque |
| 0x06 | EVPN |
| 0x80 | Flow Spec (redirect) |

### Common Sub-Types

| Type:Sub | Name |
|----------|------|
| 0x00:0x02 | Route Target |
| 0x00:0x03 | Route Origin |
| 0x01:0x02 | Route Target (IPv4) |
| 0x06:0x00 | EVPN MAC Mobility |
| 0x06:0x01 | EVPN ESI Label |
<!-- source: internal/core/bgp/attribute/community.go -- ExtendedCommunity, ExtendedCommunities -->

---

## 17. AS4_PATH (Code 17)

RFC 6793 - 4-byte AS path for non-ASN4 peers.

Same format as AS_PATH but always uses 4-byte ASNs.

**Flags:** 0xC0 (Optional Transitive)

Used when:
1. Local speaker has 4-byte ASN
2. Peer doesn't support ASN4 capability
3. AS_PATH contains ASNs > 65535
<!-- source: internal/core/bgp/attribute/attribute.go -- AttrAS4Path -->
<!-- source: internal/core/bgp/attribute/as4.go -- AS4 path processing -->

### No received AS4_PATH survives ingest

The attribute is an EGRESS one. Ze reconciles the AS-path family once, when it
reads the UPDATE, and writes the result back into the payload: AS_PATH carries
the four-octet path RFC 6793 Section 4.2.3 constructs, AGGREGATOR carries the
four-octet aggregating node, and neither AS4_PATH nor AS4_AGGREGATOR is written
out. Every consumer downstream of that point reads one width, so no forward rail
performs a Section 4.2.3 step of its own and no relay can carry either attribute
to a peer that negotiated four octets, which Section 4.1 forbids.

An UPDATE that arrives on a four-octet session and carries an AS4 attribute
anyway is the Section 6 case: the attribute is discarded, the AS_PATH beside it
is left alone, and processing continues.

FRR and BIRD both place the reconciliation at ingest too, and neither carries
the pair past it.
<!-- source: internal/component/bgp/wireu/aspath_collapse.go -- CollapseAS4Family -->
<!-- source: internal/component/bgp/reactor/session_read.go -- collapseASPathFamily -->
<!-- source: internal/core/bgp/attribute/as4.go -- ReconcileASPathFamily -->

### An ORIGINATED route carries it too

The rule is one function, `attribute.AS4PathFor`, and both kinds of sender ask
it. `wireu` asks for a route ze RELAYS. The `UpdateBuilder` asks for a route ze
ORIGINATES: a static route, a route an API client announces, a route a family
plugin encodes. `message` cannot import `wireu` (the import runs the other way),
so the answer lives in `attribute`, which both import.

Every builder appends its AS_PATH through `UpdateBuilder.appendASPath`, which
adds the AS4_PATH when the peer is an OLD speaker and the path holds a
non-mappable AS number. A builder that took no configured AS_PATH still needs
it: ze prepends its own AS to a self-originated eBGP route, so a four-octet
LOCAL AS makes the path non-mappable with no operator input at all.

Until 2026-09-11 no originating builder wrote the attribute, and such a route
reached an OLD speaker as AS_TRANS with the real AS number carried nowhere.
<!-- source: internal/core/bgp/attribute/as4.go -- AS4PathFor -->
<!-- source: internal/component/bgp/message/update_build.go -- UpdateBuilder.appendASPath -->

### The policy prepend obeys the same two rules

The `as-path-prepend` filter action does not go through the per-destination
AS-path rail. It records an operation that the AS_PATH handler splices in front
of the value already in the payload, so the segment it builds is encoded at the
width of THAT payload, and never at four octets by default.

The width comes from the call site, and each one derives it from wherever the
payload came from. The import chain and the forwarded export chain read the
source encoding context, because those bytes are still in the sending peer's
encoding. `exportFilterForBody` passes the destination's send width, because the
session write path has already encoded that body in the destination's send
context.

At two octets, a local AS above 65535 is written as AS_TRANS by the same encoder
every other AS_PATH goes through, and the real value is then carried in AS4_PATH.
A MAPPABLE local AS records no AS4_PATH operation: RFC 6793 Section 4.2.3
reconstructs by taking AS numbers from the leading part of AS_PATH, which is
where a prepend lands, so the receiver recovers them without one.

The AS4_PATH ze emits is DERIVED from the outgoing path, and it holds the WHOLE
path. It is never the received one with the local AS numbers on top, because no
received AS4_PATH survives ingest (see above). Section 4.2.3 makes the receiver
take as many AS numbers from the leading part of AS_PATH as make the two counts
equal, so an AS4_PATH shorter than the AS_PATH would have the receiver read the
AS_TRANS placeholders ze just wrote in place of the real local AS.

`AS4PathForRewrite` still holds a merging branch for the case where the payload
it is handed already carries an AS4_PATH ze itself wrote. `exportFilterForBody`
is that caller: it hands the extractor a body already encoded in a two-octet
destination's send context, so the AS4_PATH beside it is ze's own derivation
rather than a peer's attribute. `attribute.MergeAS4Path` is ze's declaration of
the receiver's rule, and running the path through it there emits what the
receiver must arrive at.
<!-- source: internal/component/bgp/reactor/filter_delta.go -- ExtractASPathPrependOps -->
<!-- source: internal/component/bgp/wireu/aspath_as4.go -- AS4PathForRewrite -->

---

## 18. AS4_AGGREGATOR (Code 18)

RFC 6793 - 4-byte aggregator for non-ASN4 peers.

```
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                     4-Byte AS Number                          |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                      BGP Identifier                           |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
```

**Length:** 8 bytes
**Flags:** 0xC0 (Optional Transitive)

Like AS4_PATH, it is an EGRESS attribute. The ingest reconciliation chooses
between the AGGREGATOR and the AS4_AGGREGATOR once and writes one four-octet
AGGREGATOR back into the payload, so no received AS4_AGGREGATOR reaches a
forward rail. A length other than 8 is malformed (RFC 6793 Section 6): the
attribute is discarded there, the AGGREGATOR beside it is left as the peer sent
it, and the UPDATE continues. `appendAS4AggregatorFor` synthesizes the companion
again for a destination that negotiated two octets.

An ORIGINATED route gets the same pair. `UpdateBuilder.appendAggregator` writes
the AGGREGATOR with AS_TRANS in its AS field and the companion beside it, and
`attribute.AS4AggregatorFor` is the one function that answers whether one is
owed, for the originating builder and the forwarding rail alike.
<!-- source: internal/core/bgp/attribute/attribute.go -- AttrAS4Aggregator -->
<!-- source: internal/core/bgp/attribute/as4.go -- selectAggregator -->
<!-- source: internal/core/bgp/attribute/as4.go -- AS4AggregatorFor -->
<!-- source: internal/component/bgp/message/update_build.go -- UpdateBuilder.appendAggregator -->
<!-- source: internal/component/bgp/rib/commit.go -- appendAS4AggregatorFor -->

---

## 32. LARGE_COMMUNITY (Code 32)

RFC 8092 - Large community values (12 bytes each).

```
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                   Global Administrator                        |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                       Local Data Part 1                       |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                       Local Data Part 2                       |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
```

**Length:** 12 bytes per community
**Flags:** 0xC0 (Optional Transitive)

Format: GlobalAdmin:LocalData1:LocalData2 (e.g., 4294967295:100:200)
<!-- source: internal/core/bgp/attribute/community.go -- LargeCommunity, LargeCommunities -->

---

## Go Implementation Notes

### Attribute Interface

Defined in `internal/core/bgp/attribute/attribute.go`:

```go
type Attribute interface {
    Code() AttributeCode
    Flags() AttributeFlags
    Len() int
    WriteTo(buf []byte, off int) int
    WriteToWithContext(buf []byte, off int, srcCtx, dstCtx *bgpctx.EncodingContext) int
}

type AttributeCode uint8
type AttributeFlags uint8

const (
    FlagOptional   AttributeFlags = 0x80
    FlagTransitive AttributeFlags = 0x40
    FlagPartial    AttributeFlags = 0x20
    FlagExtLength  AttributeFlags = 0x10
)
```

**Note:** Context-dependent attributes (AS_PATH, Aggregator) use `PackWithContext`/`WriteToWithContext` for ASN4 encoding decisions. Most attributes ignore the context parameters.
<!-- source: internal/core/bgp/attribute/attribute.go -- Attribute interface, AttributeCode, AttributeFlags -->

### WireWriter Interface

Defined in `internal/core/bgp/context/context.go` - used by messages, not directly embedded by Attribute:

```go
type WireWriter interface {
    Len(ctx *EncodingContext) int
    WriteTo(buf []byte, off int, ctx *EncodingContext) int
}
```
<!-- source: internal/core/bgp/context/context.go -- WireWriter interface -->

### Attribute Parsing

Parsing uses `AttributesWire` (lazy parsing, `internal/core/bgp/attribute/wire.go`) or `ParseAttributes` (`internal/component/bgp/plugins/rib/storage/attrparse.go`).

Simplified parsing logic (pseudocode):

```
for each attribute in packed bytes:
    flags = byte[0]
    code = byte[1]
    if flags & 0x10 (ExtLength):
        length = bytes[2:4] as uint16, skip 4 bytes
    else:
        length = byte[2], skip 3 bytes
    parse attribute value from next 'length' bytes
```

Actual implementation uses `ParseHeader()` function in `attribute.go`.
<!-- source: internal/core/bgp/attribute/attribute.go -- ParseHeader -->
<!-- source: internal/core/bgp/attribute/wire.go -- AttributesWire -->

### The Attribute Span Index

`NewAttributesWire` walks the section once, at construction, and records one
`Span` per attribute: the value offset, the value length, the type code, and the
header size class. It also sets one presence bit per type code. Value parsing
stays lazy; only the layout is eager.

Offsets are relative to the start of the attribute section, never to the UPDATE
payload. A span is therefore valid against any byte array holding the same
section contents, which is what lets `WireUpdate.Snapshot` carry the index across
its copy instead of rebuilding it.
<!-- source: internal/core/bgp/attribute/span.go -- Span, SpanIndex, BuildSpanIndex -->

The index is built once and never written afterwards. That is what makes the base
shared-immutable, so `Has`, `GetRaw`, `Packed`, `Count` and `Spilled` take no
lock: many destination goroutines read one received UPDATE concurrently and do
not contend. Only `Get`, `All` and `ForEach` take a lock, and it guards the
parsed-value side table alone.
<!-- source: internal/core/bgp/attribute/wire.go -- AttributesWire.Has, GetRaw, parseAtLocked -->

**The index freezes which attributes an UPDATE has.** Code that rewrites the
attribute bytes must run before the first `Attrs()` call, or wrap its result in a
new `WireUpdate`. On the receive path that is why `enforceRFC7606` publishes the
base last, after its RFC 7606 Section 3.g duplicate strip and its in-place
attribute-discard branch have both run.
<!-- source: internal/component/bgp/reactor/session_validation.go -- publishBase -->

---

## Real-World Attribute Count Distribution

Analysis of 112M routes from MRT dumps (RIPE RIS, LINX, RouteViews):

| Attrs | % | Cumulative | Typical Composition |
|-------|---|------------|---------------------|
| 3 | 23% | 23% | ORIGIN, AS_PATH, NEXT_HOP |
| 4 | 35% | 58% | + LOCAL_PREF or MED |
| 5 | 31% | **89%** | + COMMUNITY |
| 6 | 7% | **96%** | + LARGE_COMMUNITY or EXT_COMMUNITY |
| 7 | 3% | **99.6%** | + AGGREGATOR |
| 8 | 0.3% | **99.9%** | + ORIGINATOR_ID, CLUSTER_LIST |
| 9-10 | <0.1% | 100% | All attributes |

**Maximum observed:** 10 attributes

### Implementation Notes

`SpanIndex` holds 8 spans inline, so 99.9% of routes are indexed with no heap
allocation at all. A 9th attribute spills the remainder to a heap slice and the
receive path counts the event as `ze_bgp_update_span_spill_total`.

One span is 6 bytes, so the whole inline array is 48 bytes and rides inside the
`AttributesWire` allocation.

| Capacity | Coverage | Inline memory |
|----------|----------|---------------|
| 6 | 96% | 36 bytes |
| 8 | 99.9% | 48 bytes |
| 10 | 100% | 60 bytes |
<!-- source: internal/core/bgp/attribute/span.go -- SpanInline, SpanIndex.add -->

The table above is the measurement behind the constant: the maximum observed
attribute count is 10, and 8 covers 99.9% of the corpus.

---

## BGP-LS Attribute (Type 29)

Code 29, Optional Non-Transitive. Defined by RFC 7752 (BGP Link-State).

The BGP-LS attribute carries node, link, and prefix properties as a sequence of TLVs (Type-Length-Value). Ze decodes 40 TLV sub-types organized into categories:

| Category | TLV codes | Examples |
|----------|-----------|---------|
| Node | 263-267, 1024-1029 | ISIS area-id, router-id, SR capabilities, node name |
| Link | 1028-1036, 1088-1092, 1099-1105, 1114 | admin-group, TE metric, bandwidth, SID label |
| Prefix | 1152-1159 | prefix metric, OSPF forwarding, IGP flags, SRv6 locator |
| SRv6 | 1250-1252 | SRv6 SID structure, endpoint behavior, SID information |

Each TLV: 2-byte type + 2-byte length + value. Decoded via offset-based iterators (no allocation). See `docs/architecture/wire/bgpls-attribute-naming.md` for the full naming convention and JSON key mapping.

Source: `internal/component/bgp/plugins/nlri/ls/`.
<!-- source: internal/component/bgp/plugins/nlri/ls/register.go -- RegisterName(29, "BGP_LS") -->
<!-- source: internal/component/bgp/plugins/nlri/ls/types.go -- BGP-LS TLV decoding -->

---

## ATTR_TOMBSTONE (Code 252)

Code 252 (`0xFC`), constant `attribute.AttrTombstone`. Optional; its Transitive
bit mirrors the discarded attribute at generation time. Defined by
draft-mangin-idr-attr-tombstone-00; the code point is provisional (Section 8, IANA
allocation pending).

When a speaker applies "attribute discard" per RFC 7606, it overwrites the
malformed or policy-discarded attribute's header and first two value bytes with an
ATTR_TOMBSTONE marker in place, preserving the wire layout for zero-copy
forwarding. The length field is never modified.

Section 5.1 gives a second procedure for a speaker that REBUILDS the path
attribute section: the discarded attribute is removed and one marker is inserted
whose value is exactly the (code, reason) pairs, two octets for each discarded
attribute. Ze uses each procedure where it applies, and the two rails agree on
the decision rather than on the bytes.

| Rail | Procedure | Marker value |
|------|-----------|--------------|
| `wireu.TranscodeASPath`, which narrows one payload for an OLD-speaker destination | in place | the pair, then the discarded attribute's remaining octets zeroed |
| `wireu.ASPathEdit`, which records per-destination operations the forward path rebuilds from | rebuild | the pair alone, 2 octets |

Both mark the same attribute: an AGGREGATOR whose length is neither 6 nor 8, which
RFC 7606 Section 7.7 makes malformed and subject to attribute discard. An
AGGREGATOR whose value is under 2 octets is forwarded unchanged on both rails,
because the in-place procedure cannot fit the pair in it, and the two rails answer
one attribute the same way (`plan/journal/silent-fall-through.md`).

The rebuild rail reaches the wire through one handler registered for code 252,
`reactor.tombstoneHandler`. It exists because the flags of every other attribute
follow that attribute's own type code, and this one's do not: Section 4.2 derives
the Transitive bit from the DISCARDED attributes, so the handler reads the codes
out of the marker's own value. A marker the source already carries is forwarded
unchanged and the local pairs are dropped, because the Section 5.1 merge of an
upstream marker with a local discard is not implemented.

| Field | Value | Reference |
|-------|-------|-----------|
| Flags | `0x80 \| (original_flags & 0x50)`: Optional set, Transitive and Extended Length mirror the discarded attribute, Partial cleared | Section 4.2 |
| Type | 252 | Section 8 |
| Value[0] | original attribute type code | Section 5.1 |
| Value[1] | reason code (0 unspecified, 1 EBGP-invalid, 2 invalid-length, 3 malformed-value, 4 local-policy) | Section 4.4 |
| Value[2..] | zeroed | Section 5.1 |

**Egress flag handling: the Section 5.3 clear is NOT implemented.** The
generation flags are stamped at receive time, where the destination is not yet
known, so a transitive discarded attribute yields a transitive marker (`0xC0`)
and keeps it on every rail. Section 5.3 asks a recognizing EBGP speaker to clear
the Transitive bit before forwarding the marker to an EBGP peer, so an EBGP peer
of ze can propagate the marker further.

Ze once did this, inside the whole-payload AS_PATH rewrite. That rewrite was
replaced by the per-destination edit set (`wireu.ASPathEdit`), which resolves the
AS-path family and nothing else, so the clear had no successor and stopped
running. Thomas retired the support on 2026-09-09 rather than rebuild it, and the
two helpers that performed it were deleted with the rewrite
(`plan/journal/unwired-feature.md`, 2026-09-09).

What ze still does is the marker itself: it is written at receive time and
forwarded with the layout and the flags it was written with.

Source: `internal/component/bgp/message/attr_discard.go` (receive-time stamp),
`internal/component/bgp/wireu/tombstone.go` (`WriteTombstone`),
`internal/component/bgp/wireu/aspath_transcode.go` (the marker written in place
over an AGGREGATOR ze cannot re-encode),
`internal/component/bgp/wireu/aspath_slot.go` (`recordAggregator`, the same
decision recorded as operations for the rebuild),
`internal/component/bgp/reactor/filter_delta_handlers.go` (`tombstoneHandler`,
which emits the recorded marker).
<!-- source: internal/core/bgp/attribute/attribute.go -- AttrTombstone = 252 -->
<!-- source: internal/component/bgp/wireu/tombstone.go -- WriteTombstone -->
<!-- source: internal/component/bgp/wireu/aspath_slot.go -- recordAggregator -->
<!-- source: internal/component/bgp/reactor/filter_delta_handlers.go -- tombstoneHandler -->

---

**Created:** 2025-12-19
**Last Updated:** 2026-07-21
