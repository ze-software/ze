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
<!-- source: internal/component/bgp/attribute/attribute.go -- AttributeCode, AttributeFlags, Attribute interface -->

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
<!-- source: internal/component/bgp/attribute/attribute.go -- ParseHeader, WriteHeaderTo -->

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
<!-- source: internal/component/bgp/attribute/attribute.go -- FlagOptional, FlagTransitive, FlagPartial, FlagExtLength -->

### Flag Combinations

| Attribute Type | Flags |
|----------------|-------|
| Well-known Mandatory | 0x40 (Transitive) |
| Well-known Discretionary | 0x40 (Transitive) |
| Optional Transitive | 0xC0 (Optional + Transitive) |
| Optional Non-transitive | 0x80 (Optional only) |

---

## Attribute Type Codes

<!-- source: internal/component/bgp/attribute/attribute.go -- AttributeCode constants -->

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
| 23 | 0x17 | TUNNEL_ENCAP | 0xC0 (O-T) | RFC 5512 | not implemented |
| 25 | 0x19 | IPV6_EXT_COMMUNITY | 0xC0 (O-T) | RFC 5701 | implemented |
| 26 | 0x1A | AIGP | 0x80 (O-NT) | RFC 7311 | not implemented |
| 29 | 0x1D | BGP_LS | 0x80 (O-NT) | RFC 7752 | not implemented |
| 32 | 0x20 | LARGE_COMMUNITY | 0xC0 (O-T) | RFC 8092 | implemented |
| 40 | 0x28 | BGP_PREFIX_SID | 0xC0 (O-T) | RFC 8669 | not implemented |

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
<!-- source: internal/component/bgp/attribute/attribute.go -- AttrOrigin -->

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
<!-- source: internal/component/bgp/attribute/aspath.go -- ASPathSegmentType, ASSet, ASSequence, ASConfedSequence -->

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
<!-- source: internal/component/bgp/attribute/attribute.go -- AttrNextHop -->

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
<!-- source: internal/component/bgp/attribute/attribute.go -- AttrMED -->

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
<!-- source: internal/component/bgp/attribute/attribute.go -- AttrLocalPref -->

---

## 6. ATOMIC_AGGREGATE (Code 6)

RFC 4271 - Indicates route is an aggregate.

```
[Empty - Length = 0]
```

**Length:** 0 bytes
**Flags:** 0x40 (Well-known Discretionary)

Presence indicates the route is less specific than component routes.
<!-- source: internal/component/bgp/attribute/attribute.go -- AttrAtomicAggregate -->

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
<!-- source: internal/component/bgp/attribute/attribute.go -- AttrAggregator -->

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
<!-- source: internal/component/bgp/attribute/community.go -- CommunityNoExport, CommunityNoAdvertise, CommunityNoExportSubconfed -->

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
<!-- source: internal/component/bgp/attribute/attribute.go -- AttrOriginatorID -->

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
<!-- source: internal/component/bgp/attribute/attribute.go -- AttrClusterList -->

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
<!-- source: internal/component/bgp/attribute/attribute.go -- AttrMPReachNLRI -->

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
<!-- source: internal/component/bgp/attribute/attribute.go -- AttrMPUnreachNLRI -->

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
<!-- source: internal/component/bgp/attribute/community.go -- ExtendedCommunity, ExtendedCommunities -->

---

## 17. AS4_PATH (Code 17)

RFC 6793 - 4-byte AS path for non-ASN4 peers.

Same format as AS_PATH but always uses 4-byte ASNs.

**Flags:** 0xC0 (Optional Transitive)

Used when:
1. Local speaker has 4-byte ASN
2. Peer doesn't support ASN4 capability
3. AS_PATH contains ASNs > 65535
<!-- source: internal/component/bgp/attribute/attribute.go -- AttrAS4Path -->
<!-- source: internal/component/bgp/attribute/as4.go -- AS4 path processing -->

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
<!-- source: internal/component/bgp/attribute/attribute.go -- AttrAS4Aggregator -->

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
<!-- source: internal/component/bgp/attribute/community.go -- LargeCommunity, LargeCommunities -->

---

## Go Implementation Notes

### Attribute Interface

Defined in `internal/component/bgp/attribute/attribute.go`:

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
<!-- source: internal/component/bgp/attribute/attribute.go -- Attribute interface, AttributeCode, AttributeFlags -->

### WireWriter Interface

Defined in `internal/component/bgp/context/context.go` - used by messages, not directly embedded by Attribute:

```go
type WireWriter interface {
    Len(ctx *EncodingContext) int
    WriteTo(buf []byte, off int, ctx *EncodingContext) int
}
```
<!-- source: internal/component/bgp/context/context.go -- WireWriter interface -->

### Attribute Parsing

Parsing uses `AttributesWire` (lazy parsing, `internal/component/bgp/attribute/wire.go`) or `ParseAttributes` (`internal/component/plugin/rib/storage/attrparse.go`).

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
<!-- source: internal/component/bgp/attribute/attribute.go -- ParseHeader -->
<!-- source: internal/component/bgp/attribute/wire.go -- AttributesWire -->

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

`AttributesWire` uses initial slice capacity of 8:

```go
index := make([]attrIndex, 0, 8)  // 99.9% of routes fit without reallocation
```

| Capacity | Coverage | Memory |
|----------|----------|--------|
| 6 | 96% | 144 bytes |
| 8 | 99.9% | 192 bytes |
| 10 | 100% | 240 bytes |
<!-- source: internal/component/bgp/attribute/wire.go -- attrIndex slice capacity -->

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

**Created:** 2025-12-19
**Last Updated:** 2026-03-21
