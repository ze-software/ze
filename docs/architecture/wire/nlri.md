# NLRI Wire Format

## TL;DR (Read This First)

| Concept | Description |
|---------|-------------|
| **Pattern** | Packed-bytes-first: store wire format in `packed` field |
| **ADD-PATH** | 4-byte Path ID prepended when negotiated |
| **Key Types** | `NLRI` interface, `INET`, `Label`, `IPVPN`, `EVPN`, `Flow` |
| **Zero-Copy** | Return `packed` directly when ADD-PATH matches |
| **Index** | Family bytes + packed bytes for RIB deduplication |

**When to read full doc:** NLRI parsing, new NLRI types, ADD-PATH handling.

---

**Source:** ExaBGP `bgp/message/update/nlri/`
**Purpose:** Document wire format for all NLRI types

---

## Overview

NLRI (Network Layer Reachability Information) represents route prefixes in BGP UPDATE messages.

### AFI/SAFI Registry

Ze uses the canonical `afi/safi` names from the family registry:

| AFI | SAFI | Family | Wire Location |
|-----|------|--------|---------------|
| 1 (ipv4) | 1 (unicast) | `ipv4/unicast` | UPDATE NLRI field |
| 1 (ipv4) | 2 (multicast) | `ipv4/multicast` | MP_REACH_NLRI |
| 1 (ipv4) | 4 (mpls-label) | `ipv4/mpls-label` | MP_REACH_NLRI |
| 1 (ipv4) | 128 (mpls-vpn) | `ipv4/mpls-vpn` | MP_REACH_NLRI |
| 1 (ipv4) | 133 (flow) | `ipv4/flow` | MP_REACH_NLRI |
| 2 (ipv6) | 1 (unicast) | `ipv6/unicast` | MP_REACH_NLRI |
| 2 (ipv6) | 128 (mpls-vpn) | `ipv6/mpls-vpn` | MP_REACH_NLRI |
| 1 (ipv4) | 73 (sr-policy) | `ipv4/sr-policy` | MP_REACH_NLRI |
| 2 (ipv6) | 73 (sr-policy) | `ipv6/sr-policy` | MP_REACH_NLRI |
| 25 (l2vpn) | 70 (evpn) | `l2vpn/evpn` | MP_REACH_NLRI |
| 16388 (bgp-ls) | 71 (bgp-ls) | `bgp-ls/bgp-ls` | MP_REACH_NLRI |

<!-- source: internal/core/family/family.go -- AFI, SAFI, Family -->
<!-- source: internal/core/family/registry.go -- RegisterFamily, base family registrations -->
<!-- source: internal/component/bgp/plugins/nlri/labeled/types.go -- labeled-unicast family registrations -->
<!-- source: internal/component/bgp/plugins/nlri/vpn/types.go -- VPN family registrations -->
<!-- source: internal/component/bgp/plugins/nlri/flowspec/types.go -- FlowSpec family registrations -->
<!-- source: internal/component/bgp/plugins/nlri/srpolicy/register.go -- SR Policy family registrations -->
<!-- source: internal/component/bgp/plugins/nlri/evpn/types.go -- EVPN family registration -->
<!-- source: internal/component/bgp/plugins/nlri/ls/types.go -- BGP-LS family registrations -->

---

## Class Hierarchy

### Ze Type Hierarchy (with Embedding)

```
NLRI (interface)
├── PrefixNLRI [embedded: family, prefix, pathID]
│   ├── INET (IPv4/IPv6 unicast/multicast)
│   └── LabeledUnicast (SAFI 4) [+labels]
├── MUP (Mobile User Plane, draft-ietf-bess-mup-safi) [standalone - the route type
│                    specific field as it arrived, plus the afi, archType, routeType,
│                    rd, prefix, address, endpoint, source, teid and qfi read from it]
├── MVPN (RFC 6514) [standalone - packed wire octets, route type and length included,
│                    plus the rd, source, group and source-as read from them]
├── IPVPN (VPNv4/VPNv6) [standalone - field order: family, rd, labels, prefix, pathID]
├── EVPN (L2VPN EVPN, RFC 7432)
│   ├── EVPNType1 (Ethernet Auto-Discovery)
│   ├── EVPNType2 (MAC/IP Advertisement)
│   ├── EVPNType3 (Inclusive Multicast)
│   ├── EVPNType4 (Ethernet Segment)
│   └── EVPNType5 (IP Prefix)
├── FlowSpec (RFC 5575)
├── FlowSpecVPN (RFC 8955)
├── BGPLS (BGP-LS, RFC 7752)
│   ├── BGPLSNode
│   ├── BGPLSLink
│   ├── BGPLSPrefix
│   └── BGPLSSRv6SID
├── SRPolicy (SR-Policy, RFC 9830) [standalone: distinguisher, color, endpoint]
├── VPLS (RFC 4761)
└── RTC (Route Target Constraint, RFC 4684)
```

### Embedding Details

**PrefixNLRI** (base.go) - Shared by prefix-based types:
- `family` - AFI/SAFI address family
- `prefix` - IP prefix (netip.Prefix)
- `pathID` - RFC 7911 ADD-PATH identifier
- `addPath` - true when the wire carried a Path Identifier, read through `HasAddPath()`

RFC 7911 Section 3 reserves no value for the Path Identifier, so `pathID` of
zero identifies a path and says nothing about whether one arrived. `addPath`
answers that, and only the parser can set it, because the negotiation is what
told the parser how to read the octets. A locally built NLRI reports false: it
has no wire layout yet, and `WriteNLRI` takes the layout from the session's
EncodingContext.

**RDNLRIBase** (base.go) - Shared by RD-based types:
- `rd` - Route Distinguisher (8 bytes)
- `data` - Route-type specific data after RD (zero-copy slice)
- `cached` - Wire format cache (zero-copy slice from parsing)
- `cacheOnce` - sync.Once for thread-safe lazy initialization

**Zero-copy design:** Both `cached` and `data` are slices of the original wire buffer during parsing. No copies are made.

**Thread safety:** `Bytes()` uses `sync.Once` for thread-safe cache initialization when called on constructed (not parsed) NLRIs.

**Note:** IPVPN stays standalone because its field order differs (rd before prefix).

<!-- source: internal/core/bgp/nlri/base.go -- PrefixNLRI, RDNLRIBase -->
<!-- source: internal/core/bgp/nlri/inet.go -- INET struct -->
<!-- source: internal/component/bgp/plugins/nlri/labeled/types.go -- LabeledUnicast struct -->
<!-- source: internal/component/bgp/plugins/nlri/vpn/types.go -- VPN struct -->
<!-- source: internal/component/bgp/plugins/nlri/evpn/types.go -- EVPNRouteType constants -->
<!-- source: internal/component/bgp/plugins/nlri/flowspec/types.go -- FlowComponentType constants -->
<!-- source: internal/component/bgp/plugins/nlri/ls/types.go -- BGPLSNLRIType constants -->

---

## INET NLRI (RFC 4271)

### Wire Format

```
+---------------------------+
|   Length (1 octet)        |  Prefix length in BITS (0-32 IPv4, 0-128 IPv6)
+---------------------------+
|   Prefix (variable)       |  ceiling(Length/8) bytes, truncated IP
+---------------------------+
```

### With ADD-PATH (RFC 7911)

```
+---------------------------+
|   Path ID (4 octets)      |  Only if ADD-PATH negotiated
+---------------------------+
|   Length (1 octet)        |
+---------------------------+
|   Prefix (variable)       |
+---------------------------+
```

### Examples

| Prefix | Wire Bytes | Explanation |
|--------|------------|-------------|
| 0.0.0.0/0 | `00` | mask=0, no prefix bytes |
| 10.0.0.0/8 | `08 0A` | mask=8, 1 byte (10) |
| 192.168.1.0/24 | `18 C0 A8 01` | mask=24, 3 bytes |
| 10.0.0.1/32 | `20 0A 00 00 01` | mask=32, 4 bytes (full IP) |

`ParseINET` takes the ADD-PATH flag the session negotiated, consumes the 4-octet
Path Identifier when it is set, and records the flag on the NLRI it returns. A
reader of a parsed INET therefore asks `HasAddPath()` for the layout and reads
`PathID()` for the value, including when that value is zero.

<!-- source: internal/core/bgp/nlri/inet.go -- ParseINET, INET.Len, INET.WriteTo -->

### ExaBGP Implementation

```python
# INET stores: [addpath:4?][mask:1][prefix:var]
class INET(NLRI):
    _packed: bytes      # Complete wire format
    _has_addpath: bool  # Whether path ID is included

    @property
    def cidr(self) -> CIDR:
        # Extract CIDR from _packed[offset:]
        offset = 4 if self._has_addpath else 0
        return CIDR.from_ipv4(self._packed[offset:])
```

---

## Label NLRI (RFC 3107)

### Wire Format

```
+---------------------------+
|   Length (1 octet)        |  Total bits: label_bits + prefix_bits
+---------------------------+
|   Label 1 (3 octets)      |  20-bit label + 3 exp + 1 BoS
+---------------------------+
|   Label N (3 octets)      |  Last label has BoS=1
+---------------------------+
|   Prefix (variable)       |  IP prefix bytes
+---------------------------+
```

### Label Encoding (3 bytes)

```
 0                   1                   2
 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|          Label Value (20 bits)        |Exp|S|
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
```

- **Label Value:** 20 bits (0-1048575)
- **Exp:** 3 bits (experimental/TC)
- **S (BoS):** 1 bit - Bottom of Stack (1 = last label)

<!-- source: internal/core/bgp/nlri/helpers.go -- WriteLabelStack -->

### Special Label Values

| Raw Value | Label | Meaning |
|-----------|-------|---------|
| 0x800000 | 524288 | Withdrawal label |
| 0x000000 | 0 | Next-hop label |

### Example

```
Label 100, prefix 10.0.0.0/8:
Length = 24 (label) + 8 (prefix) = 32 bits = 0x20
Wire: 20 00 06 41 0A
      |  |______| |
      |     |     +-- Prefix byte (10)
      |     +-------- Label: (100 << 4) | 1 = 0x000641
      +-------------- Length: 32 bits
```

---

## IPVPN NLRI (RFC 4364)

### Wire Format

```
+---------------------------+
|   Length (1 octet)        |  Total bits: labels + RD + prefix
+---------------------------+
|   Label(s) (3+ octets)    |  MPLS label stack
+---------------------------+
|   RD (8 octets)           |  Route Distinguisher
+---------------------------+
|   Prefix (variable)       |  IP prefix bytes
+---------------------------+
```

### Route Distinguisher Types

**Type 0 (ASN2:NN):**
```
+---------------------------+
|   Type (2 octets) = 0     |
+---------------------------+
|   ASN (2 octets)          |  2-byte AS number
+---------------------------+
|   Assigned (4 octets)     |  Admin-assigned value
+---------------------------+
```

**Type 1 (IP:NN):**
```
+---------------------------+
|   Type (2 octets) = 1     |
+---------------------------+
|   IP (4 octets)           |  IPv4 address
+---------------------------+
|   Assigned (2 octets)     |  Admin-assigned value
+---------------------------+
```

**Type 2 (ASN4:NN):**
```
+---------------------------+
|   Type (2 octets) = 2     |
+---------------------------+
|   ASN (4 octets)          |  4-byte AS number
+---------------------------+
|   Assigned (2 octets)     |  Admin-assigned value
+---------------------------+
```

### Example

```
VPNv4: 65000:100 10.0.0.0/8 label 1000

Length = 24 (label) + 64 (RD) + 8 (prefix) = 96 bits = 0x60
Label = (1000 << 4) | 1 = 0x003E81

Wire: 60 00 3E 81 00 00 FD E8 00 00 00 64 0A
      |  |______| |___________________| |
      |     |              |            +-- Prefix (10)
      |     |              +--------------- RD: Type=0, ASN=65000, Assigned=100
      |     +------------------------------ Label: 1000 with BoS
      +------------------------------------ Length: 96 bits
```

<!-- source: internal/component/bgp/plugins/nlri/vpn/types.go -- VPN struct, ParseVPN -->
<!-- source: internal/core/bgp/nlri/rd.go -- RouteDistinguisher, RDType constants -->

---

## EVPN NLRI (RFC 7432)

See `nlri-evpn.md` for detailed documentation.

### Wire Format

```
+---------------------------+
|   Route Type (1 octet)    |  1-5 for standard types
+---------------------------+
|   Length (1 octet)        |  Payload length
+---------------------------+
|   Route Data (variable)   |  Type-specific
+---------------------------+
```

### Route Types

| Type | Name | Key Components |
|------|------|----------------|
| 1 | Ethernet Auto-Discovery | RD, ESI, ETag, Label |
| 2 | MAC/IP Advertisement | RD, ESI, ETag, MAC, IP, Label |
| 3 | Inclusive Multicast | RD, ETag, IP |
| 4 | Ethernet Segment | RD, ESI, IP |
| 5 | IP Prefix | RD, ESI, ETag, IP-Prefix, GW-IP, Label |

<!-- source: internal/component/bgp/plugins/nlri/evpn/types.go -- EVPNRouteType1..5 -->

---

## FlowSpec NLRI (RFC 5575)

See `nlri-flowspec.md` for detailed documentation.

### Wire Format

```
+---------------------------+
|   Length (1-2 octets)     |  < 240: 1 byte, >= 240: 2 bytes
+---------------------------+
|   RD (8 octets)           |  Only for flow_vpn SAFI
+---------------------------+
|   Components (variable)   |  Ordered filter components
+---------------------------+
```

### Length Encoding

- If length < 240: Single byte
- If length >= 240: `0xF0 | (length >> 8)` + `length & 0xFF`

### Component Types

| ID | Name | Type |
|----|------|------|
| 1 | Destination Prefix | Prefix |
| 2 | Source Prefix | Prefix |
| 3 | IP Protocol / Next Header | Numeric |
| 4 | Port (any) | Numeric |
| 5 | Destination Port | Numeric |
| 6 | Source Port | Numeric |
| 7 | ICMP Type | Numeric |
| 8 | ICMP Code | Numeric |
| 9 | TCP Flags | Binary |
| 10 | Packet Length | Numeric |
| 11 | DSCP / Traffic Class | Numeric |
| 12 | Fragment | Binary |
| 13 | Flow Label (IPv6) | Numeric |

<!-- source: internal/component/bgp/plugins/nlri/flowspec/types.go -- FlowComponentType constants -->

---

## BGP-LS NLRI (RFC 7752)

See `nlri-bgpls.md` for detailed documentation.

### Wire Format

```
+---------------------------+
|   NLRI Type (2 octets)    |  1=Node, 2=Link, 3=PrefixV4, 4=PrefixV6
+---------------------------+
|   Total Length (2 octets) |
+---------------------------+
|   RD (8 octets)           |  Only for bgp_ls_vpn SAFI
+---------------------------+
|   Protocol ID (1 octet)   |  1=ISIS-L1, 2=ISIS-L2, 3=OSPFv2, etc.
+---------------------------+
|   Identifier (8 octets)   |  Instance identifier
+---------------------------+
|   Descriptors (variable)  |  Node/Link/Prefix descriptors (TLVs)
+---------------------------+
```

### Protocol IDs

| ID | Protocol |
|----|----------|
| 1 | IS-IS Level 1 |
| 2 | IS-IS Level 2 |
| 3 | OSPFv2 |
| 4 | Direct |
| 5 | Static |
| 6 | OSPFv3 |

<!-- source: internal/component/bgp/plugins/nlri/ls/types.go -- BGPLSNLRIType, BGPLSProtocolID -->

---

## LabeledUnicast NLRI (RFC 8277)

### Wire Format (SAFI 4)

Same as Label NLRI (RFC 3107), but specifically for labeled unicast routes.

```
Without ADD-PATH:
+---------------------------+
|   Length (1 octet)        |  = 24*N + prefix_bits (N = labels)
+---------------------------+
|   Label 1 (3 octets)      |  S=0 (more labels follow)
+---------------------------+
|   Label N (3 octets)      |  S=1 (Bottom of Stack)
+---------------------------+
|   Prefix (variable)       |  ceiling(prefix_bits/8) bytes
+---------------------------+

With ADD-PATH (RFC 7911):
+---------------------------+
|   Path ID (4 octets)      |  Always present when negotiated
+---------------------------+
|   Length (1 octet)        |
+---------------------------+
|   Labels (3*N octets)     |
+---------------------------+
|   Prefix (variable)       |
+---------------------------+
```

A withdrawal (RFC 8277 Section 2.4) carries one Compatibility field where the
announcement carried its label stack:

```
+---------------------------+
|   Path ID (4 octets)      |  ADD-PATH only
+---------------------------+
|   Length (1 octet)        |  = 24 + prefix_bits
+---------------------------+
|   Compatibility (3 oct.)  |  SHOULD be 0x800000 on send, ignored on receipt
+---------------------------+
|   Prefix (variable)       |
+---------------------------+
```

The RECOMMENDED 0x800000 has its S bit clear, so the announcement's walk to the
bottom of the stack would read past the NLRI. A withdrawal is therefore framed
by its Length alone (`nlrisplit.GetWithdraw`, `nlrisplit.SplitWithdrawn`), and
the three octets are skipped whatever they hold (`keyLabeled` with `withdraw`).
<!-- source: internal/core/bgp/nlri/nlrisplit/nlrisplit.go -- SplitWithdrawn -->
<!-- source: internal/core/bgp/nlri/nlrisplit/prefix_key.go -- GetWithdraw, keyLabeled -->

### Ze Implementation

```go
// internal/core/bgp/nlri/labeled.go
type LabeledUnicast struct {
    PrefixNLRI           // Embeds family, prefix, pathID (Family(), Prefix(), PathID() inherited)
    labels  []uint32     // Label stack (BOS on last)
}

// NLRI interface methods (payload-only, no path ID)
func (l *LabeledUnicast) Len() int                       // Payload length only
func (l *LabeledUnicast) WriteTo(buf []byte, off int) int  // Write payload only
func (l *LabeledUnicast) Bytes() []byte                  // Payload bytes only
// Family(), Prefix(), PathID() inherited from PrefixNLRI

// For ADD-PATH aware encoding, use package functions:
nlri.LenWithContext(n, ctx)      // Adds 4 bytes when ctx.AddPath=true
nlri.WriteNLRI(n, buf, off, ctx) // Prepends path ID when ctx.AddPath=true
```

<!-- source: internal/component/bgp/plugins/nlri/labeled/types.go -- LabeledUnicast struct -->

### Label Encoding

Per RFC 3032 (3 bytes in BGP, no TTL):

```
 0                   1                   2
 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|          Label Value (20 bits)        |TC |S|
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+

Label Value: 20 bits (0-1048575)
TC: 3 bits (Traffic Class, set to 0)
S: 1 bit (Stack bit: 0=more labels, 1=bottom of stack)
```

### Example

```
Label 100, prefix 10.0.0.0/8, no ADD-PATH:
Length = 24 (label) + 8 (prefix) = 32 bits
Label = (100 >> 12) = 0x00, (100 >> 4) = 0x06, (100 << 4 | 1) = 0x41

Wire: [32, 0x00, 0x06, 0x41, 10]
       |   |_____________|   |
       |         |           +-- Prefix byte
       |         +-------------- Label 100 with BOS=1
       +------------------------ Length in bits
```

---

## Ze Implementation Notes

### NLRI Interface (Payload-Only)

After ADD-PATH simplification (Phase 3), NLRI methods return **payload only**:

```go
type NLRI interface {
    Family() Family
    Len() int                           // Payload length (no path ID)
    Bytes() []byte                      // Payload bytes (no path ID)
    WriteTo(buf []byte, off int) int    // Write payload only (no path ID)
    PathID() uint32                     // Stored path ID (0 if unset)
    SupportsAddPath() bool              // True if ADD-PATH supported
    String() string
}

// PrefixNLRI base type (base.go) - embedded by INET and LabeledUnicast
type PrefixNLRI struct {
    family Family        // AFI/SAFI
    prefix netip.Prefix  // IP prefix
    pathID uint32        // RFC 7911: 0 means no path ID
}

// INET embeds PrefixNLRI
type INET struct {
    PrefixNLRI  // Family(), Prefix(), PathID() inherited
}

// LabeledUnicast embeds PrefixNLRI + adds labels
type LabeledUnicast struct {
    PrefixNLRI           // Family(), Prefix(), PathID() inherited
    labels []uint32      // Label stack (BOS on last)
}
```

<!-- source: internal/core/bgp/nlri/nlri.go -- NLRI interface -->
<!-- source: internal/core/bgp/nlri/base.go -- PrefixNLRI struct -->
<!-- source: internal/core/bgp/nlri/inet.go -- INET struct -->
<!-- source: internal/component/bgp/plugins/nlri/labeled/types.go -- LabeledUnicast struct -->

### ADD-PATH Encoding (RFC 7911)

Path ID is handled **separately** from NLRI payload encoding:

```go
// Calculate wire length with ADD-PATH
size := nlri.LenWithContext(n, ctx)  // +4 when ctx.AddPath=true

// Write NLRI with ADD-PATH handling
buf := make([]byte, size)
nlri.WriteNLRI(n, buf, 0, ctx)  // Prepends path ID when ctx.AddPath=true
```

**WriteNLRI behavior:**
- `ctx.AddPath=true`: writes `[4-byte pathID][payload]`
- `ctx.AddPath=false` or `ctx=nil`: writes `[payload]` only

<!-- source: internal/core/bgp/nlri/nlri.go -- LenWithContext, WriteNLRI -->

### ADD-PATH Decoding for Plugin Families (RFC 7911)

A family with no dedicated in-process parser (mpls-vpn, evpn, flowspec, mup,
vpls, rtc, sr-policy, labeled, bgp-ls) reaches `ParseNLRIs` through its default
arm, which frames the section with the family's registered splitter
(`nlrisplit.Split`), wraps each NLRI in one opaque `*nlri.WireNLRI`, and hands
the detailed decode to the plugin registered for the family.

A withdrawal goes through `ParseWithdrawnNLRIs` instead (MP_UNREACH_NLRI and the
IPv4 Withdrawn Routes field). It frames the section with the family's
withdrawal splitter (`nlrisplit.SplitWithdrawn`), and `wrapNLRI` turns each
NLRI of a family that names its routes by a CIDR (`nlrisplit.RouteCIDR`, the
labeled families today) into an `*nlri.INET` of that prefix and its Path
Identifier. A labeled withdrawal carries the Compatibility field where its
announcement carried a label stack, and RFC 8277 Section 2.4 says "Upon
reception, the value of the Compatibility field MUST be ignored": framed as an
announcement, 0x800000 reads as a label entry with the S bit clear and the
reader runs past the NLRI. Any other family's withdrawal stays an opaque
`WireNLRI`.

Inventory and wire commands use that same registered framing. The route server
retains each already-framed `WireNLRI` as one native hex identity, including
its negotiated Path Identifier; it refuses malformed, unsupported, or
multi-route carriers rather than reinterpreting them as CIDR bytes. Wire
commands accept concatenated routes and select the registered announcement or
withdrawal walk from `add` or `del`. MUP therefore keeps its complete
four-octet envelope (`architecture:1`, `route-type:2`, `length:1`) and body
through inventory and peer-down command parsing.

<!-- source: internal/component/bgp/plugins/cmd/update/update_wire.go -- splitWireNLRIs -->

An `INET` is never handed to a plugin family's decoder (`appendNLRIJSONValue`).
Its `Bytes` are the CIDR, not the family's wire form, so a labeled decoder would
read the prefix octets as a label stack. `WireNLRI.String` names a route of a
CIDR-keyed family by its prefix, the way `INET.String` does. For labeled unicast,
the reflector pairs the announcement's `WireNLRI` with the withdrawal's `INET`
by family and prefix. Under ADD-PATH, both keys also carry the negotiated Path
Identifier, including zero; the reflector retains the announcement's native
hex and ADD-PATH framing for peer-down commands. Plain labeled inventory keeps
its existing CIDR form.
The route server keys both arms by the prefix and Path Identifier
(`appendOpaqueRecords`, `appendParsedRecords`) and keeps the announcement's
hex for its peer-down withdrawal. Inventory extraction keeps CIDR scratch in
the same pooled holder as its records and reuses it across the UPDATE.
Families without CIDR keys do not call the CIDR decoder.

<!-- source: internal/component/bgp/wireu/mpwire.go -- ParseWithdrawnNLRIs, wrapNLRI -->
<!-- source: internal/core/bgp/nlri/wire.go -- WireNLRI.String -->
<!-- source: internal/component/bgp/plugins/rs/server_inventory.go -- appendOpaqueRecords -->
<!-- source: internal/component/bgp/plugins/rr/withdrawal.go -- walkNLRIsAllocating -->

### Native VPN inventory identity

VPNv4 and VPNv6 stay opaque, but their route identity is not their full wire
encoding or `WireNLRI.String`'s size summary. The route server and reflector
use the registered `GetPrefixKey` operation after `SplitPathID`: `keyVPN`
keeps the RD and prefix and omits the announced label or withdrawn
Compatibility field. The source peer, family, ADD-PATH presence and Path
Identifier distinguish inventory entries. Identifier zero remains a valid path.
<!-- source: internal/core/bgp/nlri/nlrisplit/prefix_key.go -- GetPrefixKey, keyVPN -->
<!-- source: internal/component/bgp/plugins/rs/server_inventory.go -- appendOpaqueRecords, recordKey -->
<!-- source: internal/component/bgp/plugins/rr/withdrawal.go -- walkVPNNLRIs -->

Both inventories retain the announcement's native bytes separately. A
peer-down command uses `update hex` and the stored ADD-PATH framing, and names
only routes not already withdrawn by that source. This does not select a
replacement route from another source.
<!-- source: internal/component/bgp/plugins/rs/server_handlers.go -- sendBatchedWithdrawals -->
<!-- source: internal/component/bgp/plugins/rr/rr.go -- handleStateDown -->

### Native VPN withdrawal decoding

The enclosing MP_UNREACH operation also travels to the registered decoder.
`appendNLRIJSONValue`, the codec RPC handlers, and the full-UPDATE CLI decoder
pass `withdraw` alongside `addPath`; neither fact can be recovered from the
NLRI octets alone. `rpc.DecodeNLRIInput` carries the same boolean as
`"withdraw"`, with an absent or false value selecting announcement decoding.
<!-- source: internal/component/bgp/format/text_json.go -- appendNLRIJSONValue -->
<!-- source: internal/component/bgp/server/codec.go -- handleDecodeMPUnreach, handleDecodeNLRI -->
<!-- source: internal/component/bgp/cli/decode_mp.go -- parseNLRIByFamily -->
<!-- source: pkg/plugin/rpc/types.go -- DecodeNLRIInput -->

For a VPN withdrawal, the decoder consumes exactly three Compatibility
octets after the length, without reading their label or bottom-of-stack bits.
It then reads the RD and prefix, and publishes those identities and any
negotiated Path Identifier, including zero. There is no `labels` field in
that withdrawal JSON. Announcements still decode their complete label stack;
`ParseVPN` remains the announcement parser. Decoding does not rewrite the
native bytes retained for forwarding.
<!-- source: internal/component/bgp/plugins/nlri/vpn/types.go -- ParseVPN, parseVPN -->
<!-- source: internal/component/bgp/plugins/nlri/vpn/vpn.go -- DecodeNLRIHex, vpnToJSON -->

The SAFI 4 labeled-unicast decoder also uses the caller's `withdraw` flag.
It frames the entire section with `SplitWithdrawn` and reads each prefix through
`RouteCIDR`, which ignores exactly three Compatibility octets.
Withdrawal JSON contains each prefix and negotiated Path Identifier, including
zero, with no `labels` field. Announcement JSON retains every label entry,
including traffic-class and bottom-of-stack bits. A singleton section returns
one object, and adjacent NLRIs return an array in wire order.
<!-- source: internal/component/bgp/plugins/nlri/labeled/encode.go -- DecodeNLRIHex, decodeLabeledNLRI -->

### Plugin ADD-PATH metadata

`WireNLRI.Bytes()` returns those octets as they arrived, Path Identifier
included, and nothing in the octets says whether the first four are one. So the
negotiation result travels beside them, on three surfaces:

| Surface | Carrier |
|---------|---------|
| Formatter to registry | `nlri.AddPathAware`, probed by `appendNLRIJSONValue` |
| Registry to plugin | `DecodeNLRIByFamily(family, hex, addPath, withdraw)` and `Registration.InProcessNLRIDecoder` |
| Engine to external plugin | The `add-path` field of `rpc.DecodeNLRIInput` |

Each decoder consumes the 4-octet Path Identifier for each NLRI in the section
and publishes it as `"path-id"`. `nlri.SplitPathID` performs the split and
returns `ErrPathIDTruncated` for a section shorter than the identifier the
negotiation promised, so a caller never reads a zero identifier it cannot tell
from a real one.

The plugin text command `decode nlri <family> <hex>` and the `ze bgp decode`
CLI both read a hex blob with no session behind it, so both decode with no Path
Identifier.

A core family takes the same route to the same answer. `INET` records the flag
`ParseINET` was given, so `appendNLRIJSONValue` reads one interface for every
family and renders `{"prefix": ..., "path-id": 0}` rather than a bare prefix
string when the session carried a Path Identifier of zero.

Two plugin types parse the octets themselves and record the same flag on their
own value: `VPN` (`hasPath`, set by `ParseVPN` from the negotiation it was
handed) and `LabeledUnicast` (`hasPath`, set by the caller of
`NewLabeledUnicast`). Both publish it as `HasPathID()`, which is what
`vpnToJSON` and the two `String()` renderings read. A reader comparing the
identifier against zero cannot tell a route that carried one from a route that
carried none, so no surface does.

The registry ENCODE path carries the same fact the other way. A registered
encoder answers with the NLRI payload alone -- `vpn.EncodeNLRIHex` and
`labeled.EncodeNLRIHex` each return the type's `Bytes()`, which excludes the
identifier by contract -- so `encodeViaRegistry`
(`internal/component/bgp/plugins/cmd/update/update_text_nlri.go`) prepends the
four octets itself when the operator wrote `path-information`, and the flag it
hands `NewWireNLRI` describes the bytes it built. Handing that constructor the
encoder's bytes under a true flag made `WireNLRI` read the first four octets of
the payload as the identifier, and `WriteTo` then dropped them.

<!-- source: internal/core/bgp/nlri/nlri.go -- AddPathAware, SplitPathID -->
<!-- source: internal/component/bgp/format/text_json.go -- appendNLRIJSONValue -->
<!-- source: internal/component/plugin/registry/registry.go -- DecodeNLRIByFamily -->

### Index Generation

For RIB deduplication, index includes family + path ID + prefix:

```go
func (i *INET) Index() []byte {
    // Family bytes + path ID (for uniqueness) + prefix bytes
    return append(i.Family().Index(), i.Bytes()...)
}
```

---

## JSON Output Format

### INET

```json
{ "nlri": "10.0.0.0/8" }
{ "nlri": "10.0.0.0/8", "path-information": "1.2.3.4" }
```

### Label

```json
{ "nlri": "10.0.0.0/8", "label": [ [100, 1601] ] }
```

### IPVPN

```json
{
  "nlri": "10.0.0.0/8",
  "rd": "65000:100",
  "label": [ [1000, 16001] ]
}
```

---

**Last Updated: 2026-01-30
