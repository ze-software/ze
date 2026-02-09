# draft-ietf-bess-mup-safi - BGP Extensions for the Mobile User Plane (MUP) SAFI

## Meta

| Field | Value |
|-------|-------|
| Draft | draft-ietf-bess-mup-safi-00 |
| Previous | draft-mpmz-bess-mup-safi-06 |
| Title | BGP Extensions for the Mobile User Plane (MUP) SAFI |
| Status | Internet-Draft (Standards Track) |
| Date | November 2025 |
| Working Group | BESS |
| Obsoletes | - |
| Updates | - |

**Purpose:** Defines a BGP MUP SAFI to carry mobile user plane session information as BGP routes, enabling conversion of 3GPP session state into IP forwarding information for SRv6 MUP networks.

**Scope:** AFI 1 (IPv4) / AFI 2 (IPv6), SAFI 85 (MUP)

## Wire Formats

### BGP-MUP NLRI Envelope

Section 3.1

```
         +-----------------------------------+
         |    Architecture Type (1 octet)    |
         +-----------------------------------+
         |       Route Type (2 octets)       |
         +-----------------------------------+
         |         Length (1 octet)          |
         +-----------------------------------+
         |  Route Type specific (variable)   |
         +-----------------------------------+
```

| Field | Offset | Size | Type | Constraints |
|-------|--------|------|------|-------------|
| Architecture Type | 0 | 1 | uint8 | 1 = 3gpp-5g |
| Route Type | 1 | 2 | uint16 | 1-4 defined |
| Length | 3 | 1 | uint8 | Length of Route Type specific data in octets |
| Route Type Specific | 4 | variable | - | Encoding depends on Architecture Type + Route Type |

**Architecture Types:**

| Value | Name |
|-------|------|
| 1 | 3gpp-5g |

**Route Types:**

| Value | Name | Abbreviation |
|-------|------|--------------|
| 1 | Interwork Segment Discovery route | ISD |
| 2 | Direct Segment Discovery route | DSD |
| 3 | Type 1 Session Transformed route | T1ST |
| 4 | Type 2 Session Transformed route | T2ST |

### Route Type 1: Interwork Segment Discovery (ISD)

Section 3.1.1

```
         +-----------------------------------+
         |           RD  (8 octets)          |
         +-----------------------------------+
         |       Prefix Length (1 octet)     |
         +-----------------------------------+
         |        Prefix (variable)          |
         +-----------------------------------+
```

| Field | Offset | Size | Type | Constraints |
|-------|--------|------|------|-------------|
| RD | 0 | 8 | Route Distinguisher | Encoded per RFC 4364 |
| Prefix Length | 8 | 1 | uint8 | Max 32 (AFI 1) or 128 (AFI 2); exceeding = malformed |
| Prefix | 9 | variable | IP prefix | Byte length = ceil(Prefix Length / 8) |

**Route Key:** RD + Prefix Length + Prefix

**Length calculation:** 8 + 1 + ceil(prefix_length_bits / 8)

### Route Type 2: Direct Segment Discovery (DSD)

Section 3.1.2

```
         +-----------------------------------+
         |           RD  (8 octets)          |
         +-----------------------------------+
         |        Address (4 or 16 octets)   |
         +-----------------------------------+
```

| Field | Offset | Size | Type | Constraints |
|-------|--------|------|------|-------------|
| RD | 0 | 8 | Route Distinguisher | Encoded per RFC 4364 |
| Address | 8 | 4 or 16 | IP address | 4 octets for AFI 1 (IPv4), 16 octets for AFI 2 (IPv6); other sizes = malformed |

**Route Key:** RD + Address

**Length calculation:** 8 + address_length (12 for IPv4, 24 for IPv6)

### Route Type 3: Type 1 Session Transformed (T1ST)

Section 3.1.3

```
         +-----------------------------------+
         |           RD  (8 octets)          |
         +-----------------------------------+
         |      Prefix Length (1 octet)      |
         +-----------------------------------+
         |         Prefix (variable)         |
         +-----------------------------------+
         | Architecture specific (variable)  |
         +-----------------------------------+
```

| Field | Offset | Size | Type | Constraints |
|-------|--------|------|------|-------------|
| RD | 0 | 8 | Route Distinguisher | Encoded per RFC 4364 |
| Prefix Length | 8 | 1 | uint8 | Max 32 (AFI 1) or 128 (AFI 2) |
| Prefix | 9 | variable | IP prefix | UE address/prefix in 3GPP 5G case |
| Architecture Specific | varies | variable | - | See 3gpp-5g T1ST below |

**Route Key:** RD + Prefix Length + Prefix

#### 3gpp-5g Specific T1ST (Section 3.1.3.1)

Architecture-specific fields following the Prefix:

```
         +-----------------------------------+
         |          TEID (4 octets)          |
         +-----------------------------------+
         |          QFI (1 octet)            |
         +-----------------------------------+
         | Endpoint Address Length (1 octet) |
         +-----------------------------------+
         |    Endpoint Address (variable)    |
         +-----------------------------------+
         |  Source Address Length (1 octet)  |
         +-----------------------------------+
         |     Source Address (variable)     |
         +-----------------------------------+
```

| Field | Offset | Size | Type | Constraints |
|-------|--------|------|------|-------------|
| TEID | 0 | 4 | uint32 | 0 = malformed |
| QFI | 4 | 1 | uint8 | QoS Flow Identifier |
| Endpoint Address Length | 5 | 1 | uint8 | 32 (IPv4) or 128 (IPv6); other = malformed |
| Endpoint Address | 6 | 4 or 16 | IP address | GTP tunnel endpoint (gNodeB) |
| Source Address Length | varies | 1 | uint8 | 0 (absent), 32 (IPv4), or 128 (IPv6); other = malformed |
| Source Address | varies | 0, 4, or 16 | IP address | Optional; 0-length = not present |

**Length calculation:** 8 + 1 + ceil(prefix_bits/8) + 4 + 1 + 1 + endpoint_bytes + 1 + source_bytes

### Route Type 4: Type 2 Session Transformed (T2ST)

Section 3.1.4

```
         +-----------------------------------+
         |           RD  (8 octets)          |
         +-----------------------------------+
         |      Endpoint Length (1 octet)    |
         +-----------------------------------+
         |      Endpoint Address (variable)  |
         +-----------------------------------+
         | Architecture specific Endpoint    |
         |         Identifier (variable)     |
         +-----------------------------------+
```

| Field | Offset | Size | Type | Constraints |
|-------|--------|------|------|-------------|
| RD | 0 | 8 | Route Distinguisher | Encoded per RFC 4364 |
| Endpoint Length | 8 | 1 | uint8 | **Combined** bit length of Endpoint Address + Architecture-specific Endpoint Identifier |
| Endpoint Address | 9 | 4 or 16 | IP address | UPF N3 interface address |
| Arch-specific Endpoint ID | varies | variable | - | See 3gpp-5g T2ST below |

**Route Key:** RD + Endpoint Address + Architecture-specific Endpoint Identifier

**Critical encoding detail:** The Endpoint Length field is the **combined** bit length of the Endpoint Address plus the Architecture-specific Endpoint Identifier (TEID). For AFI 1 (IPv4): max 64 bits (32 IP + 32 TEID). For AFI 2 (IPv6): max 160 bits (128 IP + 128 TEID). Endpoint Length > IP bits means the architecture-specific field is present.

#### 3gpp-5g Specific T2ST (Section 3.1.4.1)

Architecture-specific Endpoint Identifier:

```
         +-----------------------------------+
         |          TEID (0-4 octets)        |
         +-----------------------------------+
```

| Field | Offset | Size | Type | Constraints |
|-------|--------|------|------|-------------|
| TEID | 0 | 0-4 | uint32 (partial) | Size = ceil((Endpoint Length - IP bits) / 8); max 4 octets; value 0 = malformed |

**TEID presence and size:** Derived from Endpoint Length:
- Endpoint Length = IP bits (32 or 128): TEID absent (0 bytes)
- Endpoint Length > IP bits: TEID present, size = ceil((Endpoint Length - IP bits) / 8)
- TEID bits = Endpoint Length - IP bits

**Examples (IPv4, IP bits = 32):**

| Endpoint Length | TEID bits | TEID bytes | Meaning |
|-----------------|-----------|------------|---------|
| 32 | 0 | 0 | No TEID (wildcard) |
| 55 | 23 | 3 | Partial TEID (23 most-significant bits) |
| 64 | 32 | 4 | Full TEID |

**Length calculation:** 8 + 1 + ip_bytes + ceil(teid_bits / 8)

### BGP MUP Extended Community

Section 3.2

```
         +-----------------------------------+
         |     Type (1 octet)               |
         +-----------------------------------+
         |     Sub-Type (1 octet)           |
         +-----------------------------------+
         |     Value (6 octets)             |
         +-----------------------------------+
```

| Field | Size | Description |
|-------|------|-------------|
| Type | 1 | MUP type (transitive, IANA-assigned) |
| Sub-Type | 1 | Direct-Type Segment Identifier (IANA-assigned) |
| Value | 6 | Configurable segment identifier value |

Transitive across AS boundaries per RFC 4360.

## Encoding Rules

- All multi-byte fields use network byte order (big-endian)
- RD encoded per RFC 4364
- Prefix fields use minimal byte encoding: ceil(prefix_length / 8)
- The AFI in MP_REACH_NLRI/MP_UNREACH_NLRI determines IPv4 (AFI 1) vs IPv6 (AFI 2) for all address fields within the NLRI
- T2ST Endpoint Length is the **combined** bit length, not just the address bits

## Decoding Rules

1. Parse Architecture Type (1 byte) → determines encoding of rest
2. Parse Route Type (2 bytes) → determines Route Type specific format
3. Parse Length (1 byte) → bounds for Route Type specific data
4. Parse Route Type specific fields based on Architecture Type + Route Type
5. For T2ST: derive TEID presence from Endpoint Length minus IP address bits
6. Unknown Route Types for supported Architecture Types: MUST silently ignore (Section 3.1)

## Validation

| Check | Valid | Invalid Action |
|-------|-------|----------------|
| ISD Prefix Length (AFI 1) | 0-32 | Treat-as-withdraw (RFC 7606) |
| ISD Prefix Length (AFI 2) | 0-128 | Treat-as-withdraw (RFC 7606) |
| DSD Address size (AFI 1) | 4 octets | Treat-as-withdraw (RFC 7606) |
| DSD Address size (AFI 2) | 16 octets | Treat-as-withdraw (RFC 7606) |
| T1ST Prefix Length (AFI 1) | 0-32 | Treat-as-withdraw (RFC 7606) |
| T1ST Prefix Length (AFI 2) | 0-128 | Treat-as-withdraw (RFC 7606) |
| T1ST TEID value | Non-zero | Treat-as-withdraw (RFC 7606) |
| T1ST Endpoint Address Length | 32 or 128 | Treat-as-withdraw (RFC 7606) |
| T1ST Source Address Length | 0, 32, or 128 | Treat-as-withdraw (RFC 7606) |
| T2ST Endpoint Length (AFI 1) | 32-64 | Treat-as-withdraw (RFC 7606) |
| T2ST Endpoint Length (AFI 2) | 128-160 | Treat-as-withdraw (RFC 7606) |
| T2ST TEID value | Non-zero | Treat-as-withdraw (RFC 7606) |
| Unknown Route Types | - | Silently ignore; MAY log |

## MUST Requirements

### Tx (Sender)

- Section 3.3.1: "When advertising the Interwork Segment Discovery route, a PE MUST attach the export BGP Route Target Extended Community of the associated routing instance."
- Section 3.3.1: "When advertising the Interwork Segment Discovery route, a PE MUST use the IPv6 address of the PE as the nexthop address in the MP_REACH_NLRI attribute."
- Section 3.3.1: "The Interwork Segment Discovery route update MUST have a prefix SID attribute"
- Section 3.3.2: "When withdrawing the Interwork Segment Discovery route, a PE MUST attach the export BGP Route Target Extended Community of the associated routing instance."
- Section 3.3.4: "The address in the BGP-MUP NLRI MUST be a unique PE identifier."
- Section 3.3.4: "When announcing the Direct Segment Discovery route, a PE MUST attach a BGP MUP Extended community of the associated routing instance."
- Section 3.3.4: "When advertising the Direct Segment Discovery route, a PE MUST use the IPv6 address of the PE as the nexthop address in the MP_REACH_NLRI attribute."
- Section 3.3.4: "The Direct Segment Discovery route update MUST have a prefix SID attribute"
- Section 3.3.5: "a BGP speaker MUST attach a BGP MUP Extended community of the associated routing instance."
- Section 3.3.7: "The MUP Controller MUST set the nexthop of the route to the address of the controller."
- Section 3.3.7: "The controller MUST announce this route using a AFI of the route and the SAFI of BGP-MUP to all other BGP speakers within the SRv6 domain."
- Section 3.3.10: "The controller MUST also attach a Route Target Extended community of the routing instances in the PE"
- Section 3.3.10: "The controller MUST set the nexthop of the route to the address of the MUP Controller."

### Rx (Receiver)

- Section 3.1: "Any other Route Types MUST be silently ignored upon a receipt if a BGP speaker supports only 3gpp-5G architecture type."
- Section 3.3.3: "the receiving BGP speaker MUST ensure that the value of Address field in the NLRI is an address of the originator of the locator value in the prefix SID attribute."
- Section 3.3.3: "When a BGP speaker receives an MP_UNREACH_NLRI attribute update message it MUST delete the withdrawn Interwork Segment Discovery route from the routing instance table"
- Section 3.3.6: "the receiving BGP speaker MUST ensure that the received nexthop value in the MP_REACH_NLRI attribute is identical to the originator of the locator value in the prefix SID attribute."
- Section 3.3.6: "When a BGP speaker receives an MP_UNREACH_NLRI attribute update message it MUST delete the withdrawn Direct Segment Discovery route from the routing instance table"
- Section 3.3.9: "The PE receiving Type 1 ST routes in MP_UNREACH_NLRI attribute MUST delete all the routes from the associated routing instance."
- Section 3.3.12: "The PE MUST handle such a malformed NLRI as a 'Treat-as-withdraw' [RFC7606]." (T2ST without MUP Extended Community)

### Validation

- Section 3.1.1: "A BGP speaker MUST handle such a malformed NLRI as a 'Treat-as-withdraw' [RFC7606]. A BGP speaker MUST skip such NLRIs and continue processing of rest of the Update message." (ISD prefix length)
- Section 3.1.2: Same treatment for DSD address length violations
- Section 3.1.3: Same treatment for T1ST prefix length violations
- Section 3.1.3.1: Same treatment for T1ST TEID=0, Endpoint Address Length invalid, Source Address Length invalid
- Section 3.1.4: Same treatment for T2ST endpoint length violations
- Section 3.1.4.1: Same treatment for T2ST TEID=0

### Errors

- Section 3.3.3: "When a BGP speaker receives the Interwork Segment Discovery routes with a MP_REACH_NLRI attribute without a prefix SID attribute, then it MUST be treated as if it contained a malformed prefix SID attribute and the 'Treat-as-withdraw' procedure"
- Section 3.3.6: Same for Direct Segment Discovery routes without prefix SID

## SHOULD/MAY

- [SHOULD] Section 3.3.3: "The BGP speaker receiving the Interwork Segment Discovery routes SHOULD ignore the nexthop in the MP_REACH_NLRI attribute." - Use prefix SID locator instead
- [SHOULD] Section 3.3.7: "the controller SHOULD attach a Route Target Extended community which the PEs are importing" - For routing instance import
- [SHOULD] Section 3.3.9: "the PE SHOULD use the received Tunnel Endpoint Address in this NLRI as a key to lookup the associated Interwork Segment Discovery route" - To extract locator and function from prefix SID
- [SHOULD] Section 3.3.12: "The BGP speaker receiving the Type 2 ST routes SHOULD ignore the received nexthop in the MP_REACH_NLRI attribute."
- [MAY] Section 3.1: "An implementation MAY log an error when such Route Types are ignored." - For unknown route types
- [MAY] Section 3.1.3.1: "A BGP speaker MAY have a local configuration for using a Source address." - When Source Address Length is 0

## Error Handling

| Condition | Detect How | Response | Code/Subcode |
|-----------|------------|----------|--------------|
| ISD prefix length > max for AFI | Check prefix length vs 32 (AFI 1) or 128 (AFI 2) | Treat-as-withdraw (RFC 7606) | N/A (NLRI-level) |
| DSD address wrong size for AFI | Check remaining bytes vs 4 (AFI 1) or 16 (AFI 2) | Treat-as-withdraw (RFC 7606) | N/A |
| T1ST prefix length > max for AFI | Check prefix length vs 32/128 | Treat-as-withdraw (RFC 7606) | N/A |
| T1ST TEID = 0 | Check 4-byte TEID field | Treat-as-withdraw (RFC 7606) | N/A |
| T1ST Endpoint Address Length invalid | Check != 32 and != 128 | Treat-as-withdraw (RFC 7606) | N/A |
| T1ST Source Address Length invalid | Check != 0 and != 32 and != 128 | Treat-as-withdraw (RFC 7606) | N/A |
| T2ST Endpoint Length > max for AFI | Check > 64 (AFI 1) or > 160 (AFI 2) | Treat-as-withdraw (RFC 7606) | N/A |
| T2ST TEID = 0 | Check TEID bytes are all zero | Treat-as-withdraw (RFC 7606) | N/A |
| ISD/DSD without Prefix SID attribute | Absence of attribute | Treat-as-withdraw (RFC 7606) | N/A |
| T2ST without MUP Extended Community | Absence of community | Treat-as-withdraw (RFC 7606) | N/A |
| Unknown Route Type | Route Type not in 1-4 | Silently ignore, MAY log | N/A |
| Nexthop/locator mismatch (ISD, DSD) | Compare nexthop to prefix SID locator originator | Treat-as-withdraw (RFC 7606) | N/A |

All malformed NLRIs: "A BGP speaker MUST skip such NLRIs and continue processing of rest of the Update message."

## Constants

| Name | Value | Usage |
|------|-------|-------|
| SAFI MUP | 85 | IANA-assigned SAFI for BGP-MUP |
| Architecture Type 3gpp-5g | 1 | Only defined architecture type |
| Route Type ISD | 1 | Interwork Segment Discovery |
| Route Type DSD | 2 | Direct Segment Discovery |
| Route Type T1ST | 3 | Type 1 Session Transformed |
| Route Type T2ST | 4 | Type 2 Session Transformed |
| MUP Extended Community Type | 0x0c | Transitive extended community type |
| MUP Extended Community Sub-Type | 0x00 | Direct-Type Segment Identifier |

## Pitfalls

- **T2ST Endpoint Length is combined:** The Endpoint Length field includes both the IP address bits AND the TEID bits. It is NOT just the IP address length. For IPv4 with full TEID: 32+32=64. For IPv6 with full TEID: 128+32=160.
- **T2ST TEID is variable-length:** Unlike T1ST where TEID is always 4 bytes, T2ST TEID length is derived from Endpoint Length minus IP bits, enabling TEID prefix aggregation.
- **T1ST TEID=0 vs T2ST TEID=0:** Both are malformed, but T2ST can have Endpoint Length equal to IP bits (no TEID present) which is different from TEID=0.
- **Source Address in T1ST is optional:** Source Address Length=0 means no source address. PE MAY use locally configured address.
- **AFI determines all address sizes:** The AFI in MP_REACH_NLRI/MP_UNREACH_NLRI determines whether ALL addresses in the NLRI are IPv4 or IPv6.
- **ISD/DSD require Prefix SID:** Discovery routes without prefix SID attribute are malformed.
- **T2ST requires MUP Extended Community:** Type 2 ST routes without MUP Extended Community SHOULD be treated as malformed.

## Compatibility

- BGP capability negotiation via RFC 4760 multiprotocol (capability code 1) with AFI 1/2 and SAFI 85
- Unknown route types silently ignored (forward-compatible for future architecture types)
- Route-REFRESH (RFC 2918) MAY be used to re-request discarded unknown route types after implementation upgrade
- All error handling uses Treat-as-withdraw (RFC 7606) for graceful degradation
