# BGP Capabilities Wire Format

## TL;DR (Read This First)

| Concept | Description |
|---------|-------------|
| **Format** | TLV: Type (1) + Length (1) + Value (variable) |
| **Key Codes** | 1=MP, 2=RouteRefresh, 65=ASN4, 69=ADD-PATH, 6=ExtMsg |
| **ADD-PATH Flags** | 1=receive, 2=send, 3=both |
| **Negotiation** | Intersection of peer caps; unknown ignored; last wins |
| **Modes** | `enable`/`disable`/`require`/`refuse` — enforcement after negotiation |
| **Key Types** | `Capability` interface, `CapabilityCode`, `Negotiated` |

**When to read full doc:** Capability parsing, OPEN messages, new capabilities, mode enforcement.

---

**Source:** RFC 5492, various RFCs, ExaBGP `bgp/message/open/capability/`
**Purpose:** Document wire format for all BGP capabilities

---

## Capability TLV Format

All capabilities share a common TLV (Type-Length-Value) format:

```
 0                   1
 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
| Cap. Code     | Cap. Length   |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|          Capability Value (variable)          |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
```

| Field | Bytes | Description |
|-------|-------|-------------|
| Cap. Code | 1 | Capability type code |
| Cap. Length | 1 | Length of value (0-255) |
| Value | Variable | Capability-specific data |

---

## Capability Codes

| Code | Hex | Name | RFC | Length |
|------|-----|------|-----|--------|
| 1 | 0x01 | Multiprotocol Extensions | RFC 2858 | 4 per family |
| 2 | 0x02 | Route Refresh | RFC 2918 | 0 |
| 3 | 0x03 | Outbound Route Filtering | RFC 5291 | Variable |
| 4 | 0x04 | Multiple Routes to Destination | RFC 3107 | 0 |
| 5 | 0x05 | Extended Next Hop Encoding | RFC 5549 | 6 per entry |
| 6 | 0x06 | Extended Message | RFC 8654 | 0 |
| 64 | 0x40 | Graceful Restart | RFC 4724 | 2 + 4*n |
| 65 | 0x41 | 4-Byte AS Number | RFC 6793 | 4 |
| 69 | 0x45 | ADD-PATH | RFC 7911 | 4 per family |
| 70 | 0x46 | Enhanced Route Refresh | RFC 7313 | 0 |
| 73 | 0x49 | FQDN | draft-walton-bgp-hostname | Variable |
| 75 | 0x4B | Software Version | draft-abraitis-bgp-version | Variable |
| 128 | 0x80 | Route Refresh (Cisco) | Vendor | 0 |
| 131 | 0x83 | Multisession (Cisco) | Vendor | Variable |

---

## 1. Multiprotocol Extensions (Code 1)

RFC 2858 - Enables support for address families beyond IPv4 unicast.

```
 0                   1                   2                   3
 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|              AFI              |   Reserved    |     SAFI      |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
```

| Field | Bytes | Description |
|-------|-------|-------------|
| AFI | 2 | Address Family Identifier |
| Reserved | 1 | Must be 0 |
| SAFI | 1 | Subsequent Address Family Identifier |

**Note:** One capability TLV per address family. Multiple families = multiple capabilities.

### Common AFI/SAFI Combinations

| AFI | SAFI | Name |
|-----|------|------|
| 1 | 1 | IPv4 Unicast |
| 1 | 2 | IPv4 Multicast |
| 1 | 4 | IPv4 MPLS Labels |
| 1 | 128 | IPv4 MPLS VPN |
| 1 | 133 | IPv4 FlowSpec |
| 2 | 1 | IPv6 Unicast |
| 2 | 128 | IPv6 MPLS VPN |
| 25 | 65 | L2VPN VPLS |
| 25 | 70 | L2VPN EVPN |
| 16388 | 71 | BGP-LS |

---

## 2. Route Refresh (Code 2)

RFC 2918 - Ability to request route refresh from peer.

```
[Empty - Length = 0]
```

No value field. Presence of capability indicates support.

---

## 3. Extended Next Hop Encoding (Code 5)

RFC 5549 - Advertise IPv6 next hop for IPv4 NLRI.

```
 0                   1                   2                   3
 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|       NLRI AFI                |      NLRI SAFI                |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|      Nexthop AFI              |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
```

| Field | Bytes | Description |
|-------|-------|-------------|
| NLRI AFI | 2 | AFI of NLRI (e.g., 1 for IPv4) |
| NLRI SAFI | 2 | SAFI of NLRI (e.g., 1 for unicast) |
| Nexthop AFI | 2 | AFI of nexthop (e.g., 2 for IPv6) |

Multiple entries concatenated.

---

## 4. Extended Message (Code 6)

RFC 8654 - Support for BGP messages > 4096 bytes.

```
[Empty - Length = 0]
```

No value field. When negotiated, max message size increases to 65535 bytes.

---

## 5. Graceful Restart (Code 64)

RFC 4724 - Graceful restart support and state preservation.

```
 0                   1                   2                   3
 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|R|Rsv|  Restart Time           |      AFI                      |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|     SAFI      |    Flags      |  (repeat AFI/SAFI/Flags)      |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
```

### Header (2 bytes)

| Bits | Field | Description |
|------|-------|-------------|
| 0 | R (Restart State) | 1 = Speaker is restarting |
| 1-3 | Reserved | Must be 0 |
| 4-15 | Restart Time | Seconds (0-4095) |

### Per-Family Entry (4 bytes each)

| Field | Bytes | Description |
|-------|-------|-------------|
| AFI | 2 | Address Family |
| SAFI | 1 | Sub Address Family |
| Flags | 1 | Bit 7: Forwarding State preserved |

---

## 6. 4-Byte AS Number (Code 65)

RFC 6793 - Support for 4-byte AS numbers.

```
 0                   1                   2                   3
 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                    4-Byte AS Number                           |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
```

| Field | Bytes | Description |
|-------|-------|-------------|
| AS Number | 4 | Speaker's 4-byte AS number |

When this capability is negotiated:
- AS_PATH uses 4-byte ASNs
- My AS in OPEN can use AS_TRANS (23456) if > 65535

---

## 7. ADD-PATH (Code 69)

RFC 7911 - Advertise multiple paths per prefix.

```
 0                   1                   2                   3
 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|              AFI              |     SAFI      | Send/Receive  |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
```

| Field | Bytes | Description |
|-------|-------|-------------|
| AFI | 2 | Address Family |
| SAFI | 1 | Sub Address Family |
| Send/Receive | 1 | 1=Receive, 2=Send, 3=Both |

Multiple entries concatenated (4 bytes each).

### Send/Receive Values

| Value | Meaning |
|-------|---------|
| 0 | Disabled |
| 1 | Can receive ADD-PATH |
| 2 | Can send ADD-PATH |
| 3 | Can send and receive |

When ADD-PATH is enabled, NLRI includes a 4-byte Path ID before each prefix.

---

## 8. Enhanced Route Refresh (Code 70)

RFC 7313 - Beginning/End of Route Refresh markers.

```
[Empty - Length = 0]
```

Enables BoRR (1) and EoRR (2) in ROUTE-REFRESH reserved field.

---

## 9. FQDN / Hostname (Code 73)

draft-walton-bgp-hostname-capability

```
 0                   1                   2   ...
 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
| Hostname Len  |  Hostname (variable)              |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
| Domain Len    |  Domain Name (variable)           |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
```

| Field | Bytes | Description |
|-------|-------|-------------|
| Hostname Len | 1 | Length of hostname |
| Hostname | Variable | UTF-8 hostname |
| Domain Len | 1 | Length of domain name |
| Domain Name | Variable | UTF-8 domain name |

---

## 10. Software Version (Code 75)

draft-abraitis-bgp-version-capability

```
 0                   1                   2   ...
 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
| Version Len   |  Version String (variable)        |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
```

| Field | Bytes | Description |
|-------|-------|-------------|
| Version Len | 1 | Length of version string |
| Version String | Variable | UTF-8 software version |

---

## Capability Negotiation

### Rules

1. **Intersection:** Negotiated capabilities = capabilities both peers advertise
2. **Duplicates:** If same capability appears multiple times, use last one (RFC 5492)
3. **Unknown:** Unknown capabilities are ignored (not an error)
4. **Required:** Session fails if required capability not negotiated
5. **Refused:** Session fails if refused capability is present in peer's OPEN

### Negotiated State

Defined in `internal/plugins/bgp/capability/negotiated.go`:

```go
type Negotiated struct {
    // Sub-components (composite pattern)
    Identity *PeerIdentity // ASNs, Router IDs
    Encoding *EncodingCaps // ASN4, families, ADD-PATH
    Session  *SessionCaps  // ExtendedMessage, GR

    // Backward-compat fields (delegates to sub-components)
    LocalASN             uint32
    PeerASN              uint32
    ASN4                 bool
    ExtendedMessage      bool
    RouteRefresh         bool
    EnhancedRouteRefresh bool
    HoldTime             uint16
    GracefulRestart      *GracefulRestart

    // Internal maps
    families        map[Family]bool
    addPath         map[Family]AddPathMode
    extendedNextHop map[Family]AFI
    peerCodes       map[Code]bool  // Raw capability codes from peer's OPEN
}
```

---

## Capability Mode Enforcement

### Mode Vocabulary

All capabilities support a four-mode vocabulary controlling advertisement and enforcement:

| Mode | Advertise? | Enforcement | Use Case |
|------|------------|-------------|----------|
| `enable` | Yes | None — proceed if peer lacks it | Normal operation (default for ASN4) |
| `disable` | No | None — proceed either way | Explicitly turn off a capability |
| `require` | Yes | Reject session if peer **lacks** it | Mandate 4-byte ASN, route-refresh, etc. |
| `refuse` | No | Reject session if peer **has** it | Block unwanted capabilities |

Backwards-compatible aliases: `true` = `enable`, `false` = `disable`.

### Enforcement Flow

Enforcement happens after OPEN exchange, in both active and passive session paths:

```
1. Exchange OPENs (local + remote)
2. Negotiate capabilities (intersection)
3. Check address family requirements (existing)
4. Check required capability codes → NOTIFICATION if any missing
5. Check refused capability codes → NOTIFICATION if any present in peer's OPEN
6. If all checks pass → proceed to ESTABLISHED
```

### Why peerCodes Exists

The `peerCodes` map tracks capability codes from the peer's raw OPEN, separate from the negotiated intersection. This is essential for `refuse` enforcement:

- If we refuse ASN4, we don't advertise it
- The peer may still advertise ASN4 in its OPEN
- The negotiated intersection won't contain ASN4 (we didn't advertise it)
- But we need to detect that the **peer** has it and reject the session

Without `peerCodes`, refused capabilities would be invisible after negotiation.

### NOTIFICATION on Rejection

When a capability mode violation is detected, the session sends a NOTIFICATION per RFC 5492:

| Field | Value |
|-------|-------|
| Error Code | 2 (OPEN Message Error) |
| Error Subcode | 7 (Unsupported Capability) |
| Data | Capability TLVs for each violating code |

**Data format** — each violating capability is encoded as a 2-byte TLV:

```
 0                   1
 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
| Cap. Code     | Cap. Length=0 |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
```

For simple capabilities (ASN4, route-refresh, extended-message), the length is 0 because the NOTIFICATION data signals which capability was problematic, not its value. Multiple violations are concatenated.

**Example:** `asn4 require;` with peer lacking ASN4 →
NOTIFICATION hex: `FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF 0017 03 02 07 41 00`

| Part | Hex | Meaning |
|------|-----|---------|
| Marker | `FFFF...` (16 bytes) | BGP marker |
| Length | `0017` | 23 bytes |
| Type | `03` | NOTIFICATION |
| Error Code | `02` | OPEN Message Error |
| Error Subcode | `07` | Unsupported Capability |
| Data: Cap Code | `41` | Code 65 (ASN4) |
| Data: Cap Length | `00` | Length 0 |

### Implementation

| Component | File | Purpose |
|-----------|------|---------|
| `CheckRequiredCodes(codes)` | `capability/negotiated.go` | Returns missing required codes |
| `CheckRefusedCodes(codes)` | `capability/negotiated.go` | Returns peer codes that match refused list |
| `buildUnsupportedCapabilityDataCodes(codes)` | `reactor/session.go` | Builds NOTIFICATION data for non-family codes |
| Enforcement in `processOpen()` | `reactor/session.go` | Active session path (we initiated) |
| Enforcement in `handleOpen()` | `reactor/session.go` | Passive session path (peer initiated) |

### Config to Enforcement Data Flow

```
Config: "asn4 require;"
    ↓ parseCapabilitiesFromTree()
PeerSettings.RequiredCapabilities = [CodeASN4]
    ↓ session constructor
Session stores RequiredCapabilities, RefusedCapabilities
    ↓ OPEN exchange
Negotiate() populates peerCodes map
    ↓ post-negotiation validation
CheckRequiredCodes() / CheckRefusedCodes()
    ↓ if violations
sendNotification(OpenMessageError, UnsupportedCapability, data)
```

### Capability Defaults

| Capability | Default Mode | Notes |
|------------|-------------|-------|
| ASN4 (code 65) | `enable` | RFC 6793 — on unless explicitly disabled |
| Extended Message (code 6) | absent (opt-in) | Only active if configured |
| Route Refresh (code 2) | absent (opt-in) | Only active if configured |
| Graceful Restart (code 64) | absent (opt-in) | Needs restart-time config |
| ADD-PATH (code 69) | absent (opt-in) | Needs send/receive config |
| Extended Next Hop (code 5) | absent (opt-in) | Needs family mapping |
| Software Version (code 75) | absent (opt-in) | Only active if configured |

"Absent" means the capability is not advertised and has no enforcement — it's as if it doesn't exist. Setting it to `enable`, `require`, or `refuse` activates it.

---

## Go Implementation Notes

### Capability Interface

Defined in `internal/plugins/bgp/capability/capability.go`:

```go
type Capability interface {
    Code() Code
    Pack() []byte
}

type Code uint8

const (
    CodeMultiprotocol        Code = 1  // RFC 4760
    CodeRouteRefresh         Code = 2  // RFC 2918
    CodeExtendedNextHop      Code = 5  // RFC 8950
    CodeExtendedMessage      Code = 6  // RFC 8654
    CodeGracefulRestart      Code = 64 // RFC 4724
    CodeASN4                 Code = 65 // RFC 6793
    CodeAddPath              Code = 69 // RFC 7911
    CodeEnhancedRouteRefresh Code = 70 // RFC 7313
    CodeFQDN                 Code = 73 // RFC 8516
    CodeSoftwareVersion      Code = 75 // draft
)
```

### Parsing Capabilities

Parsing is done via `Parse()` in `parse.go`, returning `[]Capability`.

---

**Created:** 2025-12-19
**Last Updated:** 2026-02-21
