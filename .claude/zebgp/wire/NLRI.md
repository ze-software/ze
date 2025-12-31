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

ExaBGP supports 42 AFI/SAFI combinations. Key ones:

| AFI | SAFI | Family | Wire Location |
|-----|------|--------|---------------|
| 1 (IPv4) | 1 (unicast) | inet | UPDATE NLRI field |
| 1 (IPv4) | 2 (multicast) | inet-multicast | MP_REACH_NLRI |
| 1 (IPv4) | 4 (nlri_mpls) | inet-labeled | MP_REACH_NLRI |
| 1 (IPv4) | 128 (mpls_vpn) | vpnv4 | MP_REACH_NLRI |
| 1 (IPv4) | 133 (flow_ip) | flowspec4 | MP_REACH_NLRI |
| 2 (IPv6) | 1 (unicast) | inet6 | MP_REACH_NLRI |
| 2 (IPv6) | 128 (mpls_vpn) | vpnv6 | MP_REACH_NLRI |
| 25 (L2VPN) | 70 (evpn) | evpn | MP_REACH_NLRI |
| 16388 (BGP-LS) | 71 (bgp_ls) | bgp-ls | MP_REACH_NLRI |

---

## Class Hierarchy

```
NLRI (base)
├── INET (IPv4/IPv6 unicast/multicast)
│   └── Label (MPLS labeled routes, RFC 3107)
│       └── IPVPN (VPNv4/VPNv6, RFC 4364)
├── EVPN (L2VPN EVPN, RFC 7432)
│   ├── EthernetAD (Type 1)
│   ├── MAC (Type 2)
│   ├── Multicast (Type 3)
│   ├── Segment (Type 4)
│   └── Prefix (Type 5)
├── Flow (FlowSpec, RFC 5575)
├── BGPLS (BGP-LS, RFC 7752)
│   ├── Node
│   ├── Link
│   ├── PrefixV4
│   ├── PrefixV6
│   └── SRv6SID
├── VPLS (RFC 4761)
├── RTC (Route Target Constraint, RFC 4684)
└── MUP (Mobile User Plane)
```

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

---

## EVPN NLRI (RFC 7432)

See `NLRI_EVPN.md` for detailed documentation.

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

---

## FlowSpec NLRI (RFC 5575)

See `NLRI_FLOWSPEC.md` for detailed documentation.

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

---

## BGP-LS NLRI (RFC 7752)

See `NLRI_BGPLS.md` for detailed documentation.

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

### ZeBGP Implementation

```go
// pkg/bgp/nlri/labeled.go
type LabeledUnicast struct {
    family  Family       // AFI + SAFI (SAFI always SAFIMPLSLabel)
    prefix  netip.Prefix
    labels  []uint32     // Label stack (BOS on last)
    pathID  uint32
    hasPath bool
}

// Implements nlri.NLRI interface
func (l *LabeledUnicast) Pack(ctx *PackContext) []byte
func (l *LabeledUnicast) Bytes() []byte
func (l *LabeledUnicast) Family() Family
func (l *LabeledUnicast) PathID() uint32
func (l *LabeledUnicast) HasPathID() bool
```

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

## ZeBGP Implementation Notes

### Packed-Bytes-First Pattern

All NLRI types store wire format in `_packed`:

```go
type NLRI interface {
    PackNLRI(negotiated *Negotiated) []byte
    Index() []byte  // For RIB deduplication
}

// INET stores complete wire format
type INET struct {
    packed     []byte  // [addpath:4?][mask:1][prefix:var]
    hasAddpath bool
}

// Properties extract data lazily
func (i *INET) CIDR() CIDR {
    offset := 0
    if i.hasAddpath { offset = 4 }
    return ParseCIDR(i.packed[offset:])
}
```

### Index Generation

For RIB deduplication, index includes family + packed bytes:

```go
func (i *INET) Index() []byte {
    return append(i.Family().Index(), i.packed...)
}
```

### ADD-PATH Handling

```go
func (i *INET) PackNLRI(neg *Negotiated) []byte {
    sendAddpath := neg.AddPath.Send(i.AFI, i.SAFI)

    if sendAddpath {
        if i.hasAddpath {
            return i.packed  // Zero-copy
        }
        // Prepend NOPATH (4 zero bytes)
        return append([]byte{0,0,0,0}, i.packed...)
    } else {
        if i.hasAddpath {
            return i.packed[4:]  // Strip path ID
        }
        return i.packed  // Zero-copy
    }
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

**Last Updated:** 2025-12-19
