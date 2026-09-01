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
| Enrolment | enrolled |
| Enrolment reason | BGP Extensions for Mobile User Plane (MUP) SAFI |
| Support | drafts 60 |
| Support area | BGP Mobile User Plane SAFI 85 |
| Support status | Partial |
| Support coverage | The BGP-MUP NLRI codec only: ipv4/mup and ipv6/mup family registration, ISD/DSD/T1ST/T2ST encoding from config and route commands, header plus RD decoding, the RFC 7606 Section 5.4 ruling that discards a route whose Architecture Type is not 1 or whose Route Type is outside 1..4 at ingress (internal/component/bgp/plugins/nlri/mup/rfc7606.go, RecognizeNLRI), MUP extended-community config syntax, and family-generic MP_REACH announcement (internal/component/bgp/plugins/nlri/mup). Thirty-seven MUST gaps are annotated per line in `rfc/short/draft-ietf-bess-mup-safi.md`. Withdrawal: no MUP NLRI reaches the family-generic MP_UNREACH encoder (internal/component/bgp/reactor/peer_rib_routes.go) -- its callers read the PeerOpWithdraw queue, and neither withdrawal entry point parses SAFI 85 (internal/component/bgp/plugins/cmd/update/update_text_nlri.go, internal/component/bgp/plugins/cmd/announce/announce.go) -- so a Type 1 ST or Type 2 ST withdrawal cannot be emitted (3.3.8-1, 3.3.11-1). Receive side: nlrisplit registers SplitMUP for SAFI 85 since 2026-08-04 (internal/core/bgp/nlri/nlrisplit/register.go), so a received MUP route is stored as an opaque Adj-RIB-In entry (insertPoolNLRIs) and a withdrawal deletes exactly the NLRI it names (removePoolNLRIs, internal/component/bgp/plugins/rib/rib.go). Four routing-instance obligations stay open because ze models no MUP routing instance and no route-type-aware wildcard delete exists (DRAFT-IETF-BESS-MUP-SAFI-3.3.3-2, 3.3.6-2, 3.3.9-1, 3.3.9-2). ParseMUP keeps the route-type body opaque (internal/component/bgp/plugins/nlri/mup/types.go), so no RFC 7606 treat-as-withdraw fires on an out-of-range prefix length, wrong-size address, zero TEID, invalid endpoint or source length, over-long T2ST endpoint length, or non-3gpp-5g architecture type (3.1.1-1, 3.1.1-2, 3.1.2-1, 3.1.3-1, 3.1.3.1-1, 3.1.3.1-2, 3.1.3.1-3, 3.1.3.1-4, 3.1.4-1, 3.1.4.1-1, 3.1.4.1-2), nor on a missing Prefix-SID, a nexthop/locator mismatch, or a Type 2 ST route without the BGP MUP Extended Community (3.3.3-1, 3.3.3-3, 3.3.3-4, 3.3.6-1, 3.3.12-1). Send side: ze runs no MUP PE or MUP Controller function, so route targets, the BGP MUP Extended Community, the Prefix-SID, the GTP4.E/GTP6.E function, the required T1ST TEID and Endpoint Address, and the PE or controller IPv6 nexthop are whatever the operator configures rather than derived (3.3.1-1, 3.3.1-2, 3.3.1-3, 3.3.1-4, 3.3.2-1, 3.3.4-1, 3.3.4-2, 3.3.4-3, 3.3.4-4, 3.3.5-1, 3.3.5-2, 3.3.7-1, 3.3.7-3, 3.3.10-1, 3.3.10-2). |
| Support remaining | - |

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

## Compliance Checklist

- [ ] [DRAFT-IETF-BESS-MUP-SAFI-3.3.1-1] [MUST] PE advertising ISD route must attach export BGP Route Target Extended Community of the associated routing instance (Section 3.3.1) {gap: parseConfigRoute attaches only the extended communities the operator configures (internal/component/bgp/plugins/nlri/mup/config.go:66) and ze models no routing instance with export route targets, so no export route target is derived for an ISD advertisement}
- [ ] [DRAFT-IETF-BESS-MUP-SAFI-3.3.1-2] [MUST] PE advertising ISD route must use IPv6 address of PE as nexthop in MP_REACH_NLRI (Section 3.3.1) {gap: EncodeRoute takes the next-hop verbatim from the route command (internal/component/bgp/plugins/nlri/mup/encode.go:173-191) and accepts an IPv4 next-hop for an ISD route, so nothing requires the IPv6 address of the PE}
- [ ] [DRAFT-IETF-BESS-MUP-SAFI-3.3.1-3] [MUST] ISD route update must have a prefix SID attribute (Section 3.3.1) {gap: parseConfigRoute adds the Prefix-SID attribute only when the config supplies one (internal/component/bgp/plugins/nlri/mup/config.go:71), so an ISD route configured without a prefix SID is advertised without one}
- [ ] [DRAFT-IETF-BESS-MUP-SAFI-3.3.2-1] [MUST] PE withdrawing ISD route must attach export BGP Route Target Extended Community (Section 3.3.2) {gap: a MUP withdrawal is built as a bare MP_UNREACH_NLRI with no path attributes (internal/component/bgp/reactor/peer_rib_routes.go:182-197), so an ISD withdrawal carries no export route target}
- [ ] [DRAFT-IETF-BESS-MUP-SAFI-3.3.4-1] [MUST] DSD address in NLRI must be a unique PE identifier (Section 3.3.4) {gap: parseDSDFields accepts any parsable address as the DSD NLRI address (internal/component/bgp/plugins/nlri/mup/encode.go:301-310) and never checks that it identifies the PE uniquely}
- [ ] [DRAFT-IETF-BESS-MUP-SAFI-3.3.4-2] [MUST] PE announcing DSD route must attach a BGP MUP Extended Community (Section 3.3.4) {gap: parseConfigRoute attaches only the extended communities the operator configures (internal/component/bgp/plugins/nlri/mup/config.go:66), so a DSD advertisement carries a BGP MUP Extended Community only when the operator writes one into the route}
- [ ] [DRAFT-IETF-BESS-MUP-SAFI-3.3.4-3] [MUST] PE advertising DSD route must use IPv6 address of PE as nexthop in MP_REACH_NLRI (Section 3.3.4) {gap: EncodeRoute takes the next-hop verbatim from the route command (internal/component/bgp/plugins/nlri/mup/encode.go:173-191) and accepts an IPv4 next-hop for a DSD route, so nothing requires the IPv6 address of the PE}
- [ ] [DRAFT-IETF-BESS-MUP-SAFI-3.3.4-4] [MUST] DSD route update must have a prefix SID attribute (Section 3.3.4) {gap: parseConfigRoute adds the Prefix-SID attribute only when the config supplies one (internal/component/bgp/plugins/nlri/mup/config.go:71), so a DSD route configured without a prefix SID is advertised without one}
- [ ] [DRAFT-IETF-BESS-MUP-SAFI-3.3.5-1] [MUST] BGP speaker announcing T1ST must attach a BGP MUP Extended Community (Section 3.3.5) {gap: parseConfigRoute attaches only the extended communities the operator configures (internal/component/bgp/plugins/nlri/mup/config.go:66), so a Type 1 ST advertisement carries a BGP MUP Extended Community only when the operator writes one into the route}
- [ ] [DRAFT-IETF-BESS-MUP-SAFI-3.3.7-1] [MUST] MUP Controller must set nexthop of T1ST route to the controller address (Section 3.3.7) {gap: EncodeRoute takes the next-hop verbatim from the route command (internal/component/bgp/plugins/nlri/mup/encode.go:173-191) for a Type 1 ST route and ze runs no MUP Controller function that substitutes the controller address}
- [ ] [DRAFT-IETF-BESS-MUP-SAFI-3.3.7-2] [MUST] Controller must announce T1ST route using AFI of the route and SAFI BGP-MUP to all BGP speakers in SRv6 domain (Section 3.3.7) {single-polarity: positive; EncodeRoute emits MUP NLRI only under SAFI 85 with the AFI taken from the route family (internal/component/bgp/plugins/nlri/mup/encode.go:183-199), so no non-conformant AFI/SAFI emission exists to reject}
- [ ] [DRAFT-IETF-BESS-MUP-SAFI-3.3.10-1] [MUST] Controller must attach Route Target Extended Community of routing instances in the PE for T2ST (Section 3.3.10) {gap: parseConfigRoute attaches only the extended communities the operator configures (internal/component/bgp/plugins/nlri/mup/config.go:66) and ze models no routing instance with export route targets, so a Type 2 ST advertisement carries a route target only when the operator writes one into the route}
- [ ] [DRAFT-IETF-BESS-MUP-SAFI-3.3.10-2] [MUST] Controller must set nexthop of T2ST route to MUP Controller address (Section 3.3.10) {gap: EncodeRoute takes the next-hop verbatim from the route command (internal/component/bgp/plugins/nlri/mup/encode.go:173-191) for a Type 2 ST route and ze runs no MUP Controller function that substitutes the controller address}
- [ ] [DRAFT-IETF-BESS-MUP-SAFI-3.1-1] [MUST] Unknown Route Types for supported Architecture Types must be silently ignored (Section 3.1) {single-polarity: positive; ParseMUP accepts any route type under the 3gpp-5g architecture type and returns the bytes after the declared Length (internal/component/bgp/plugins/nlri/mup/types.go:118-152), so there is no rejection path for an unknown route type to drive negatively}
- [ ] [DRAFT-IETF-BESS-MUP-SAFI-3.3.3-1] [MUST] Receiver must ensure ISD Address field value is address of originator of locator in prefix SID attribute (Section 3.3.3) {gap: DecodeNLRIHex surfaces only route type, architecture type and RD from a received MUP NLRI (internal/component/bgp/plugins/nlri/mup/mup.go:56-73) and ze reads no prefix SID locator anywhere in internal/component/bgp, so the ISD Address field is never checked against the originator of the prefix SID locator}
- [ ] [DRAFT-IETF-BESS-MUP-SAFI-3.3.3-2] [MUST] On MP_UNREACH_NLRI, receiver must delete withdrawn ISD route from routing instance table (Section 3.3.3) {gap: nlrisplit now registers SplitMUP for SAFI 85 (internal/core/bgp/nlri/nlrisplit/register.go), so insertPoolNLRIs stores a received MUP NLRI as an opaque entry keyed on the whole NLRI and removePoolNLRIs deletes exactly the NLRI a withdrawal names (internal/component/bgp/plugins/rib/rib.go). What remains is that ze models no MUP routing instance: the entry lives in the peer's Adj-RIB-In alone, and no RFC requirement tagged test drives an ISD withdrawal through it}
- [ ] [DRAFT-IETF-BESS-MUP-SAFI-3.3.6-1] [MUST] Receiver must ensure DSD nexthop in MP_REACH_NLRI is identical to originator of locator in prefix SID attribute (Section 3.3.6) {gap: DecodeNLRIHex surfaces only route type, architecture type and RD from a received MUP NLRI (internal/component/bgp/plugins/nlri/mup/mup.go:56-73) and ze reads no prefix SID locator anywhere in internal/component/bgp, so the DSD nexthop is never compared with the originator of the prefix SID locator}
- [ ] [DRAFT-IETF-BESS-MUP-SAFI-3.3.6-2] [MUST] On MP_UNREACH_NLRI, receiver must delete withdrawn DSD route from routing instance table (Section 3.3.6) {gap: nlrisplit now registers SplitMUP for SAFI 85 (internal/core/bgp/nlri/nlrisplit/register.go), so insertPoolNLRIs stores a received MUP NLRI as an opaque entry keyed on the whole NLRI and removePoolNLRIs deletes exactly the NLRI a withdrawal names (internal/component/bgp/plugins/rib/rib.go). What remains is that ze models no MUP routing instance: the entry lives in the peer's Adj-RIB-In alone, and no RFC requirement tagged test drives a DSD withdrawal through it}
- [ ] [DRAFT-IETF-BESS-MUP-SAFI-3.3.9-1] [MUST] PE receiving T1ST routes in MP_UNREACH_NLRI must delete all routes from associated routing instance (Section 3.3.9) {gap: nlrisplit now registers SplitMUP for SAFI 85 (internal/core/bgp/nlri/nlrisplit/register.go), so insertPoolNLRIs stores a received MUP NLRI as an opaque entry keyed on the whole NLRI and removePoolNLRIs deletes exactly the NLRI a withdrawal names (internal/component/bgp/plugins/rib/rib.go). The delete is one NLRI for one NLRI. Nothing reads the Type 1 ST route type to delete every route of the associated routing instance, and ze models no such instance}
- [ ] [DRAFT-IETF-BESS-MUP-SAFI-3.3.12-1] [MUST] PE must handle T2ST without MUP Extended Community as treat-as-withdraw (Section 3.3.12) {gap: DecodeNLRIHex surfaces only route type, architecture type and RD from a received MUP NLRI (internal/component/bgp/plugins/nlri/mup/mup.go:56-73) without consulting the UPDATE extended communities, so a Type 2 ST route arriving without the BGP MUP Extended Community is not treated as withdrawn}
- [ ] [DRAFT-IETF-BESS-MUP-SAFI-3.1.1-1] [MUST] ISD with prefix length exceeding max for AFI: treat-as-withdraw per RFC 7606 (Section 3.1.1) {gap: ParseMUP keeps the route-type-specific bytes opaque after the RD (internal/component/bgp/plugins/nlri/mup/types.go:137-149) and never reads the ISD prefix length, so a prefix length above 32 for AFI 1 or 128 for AFI 2 is accepted as a valid route}
- [ ] [DRAFT-IETF-BESS-MUP-SAFI-3.1.1-2] [MUST] Speaker must skip malformed NLRIs and continue processing rest of Update message (Section 3.1.1) {gap: ParseMUP applies no semantic validation (internal/component/bgp/plugins/nlri/mup/types.go:118-152), so no NLRI is ever classed malformed and skipped, and a truncated NLRI aborts the parse with ErrMUPTruncated (internal/component/bgp/plugins/nlri/mup/types.go:127-129) instead of continuing with the rest of the Update}
- [ ] [DRAFT-IETF-BESS-MUP-SAFI-3.1.2-1] [MUST] DSD with wrong address size for AFI: treat-as-withdraw per RFC 7606 (Section 3.1.2) {gap: ParseMUP keeps the route-type-specific bytes opaque after the RD (internal/component/bgp/plugins/nlri/mup/types.go:137-149) and never measures the DSD address against the AFI, so a 4-octet address under AFI 2 or a 16-octet address under AFI 1 is accepted}
- [ ] [DRAFT-IETF-BESS-MUP-SAFI-3.1.3-1] [MUST] T1ST with prefix length exceeding max for AFI: treat-as-withdraw per RFC 7606 (Section 3.1.3) {gap: ParseMUP keeps the route-type-specific bytes opaque after the RD (internal/component/bgp/plugins/nlri/mup/types.go:137-149) and never reads the T1ST prefix length, so a prefix length above 32 for AFI 1 or 128 for AFI 2 is accepted as a valid route}
- [ ] [DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-1] [MUST] T1ST with TEID=0: treat-as-withdraw per RFC 7606 (Section 3.1.3.1) {gap: ParseMUP keeps the route-type-specific bytes opaque after the RD (internal/component/bgp/plugins/nlri/mup/types.go:137-149) and never reads the T1ST TEID, and parseTEIDWithBits maps an absent TEID to zero bits which writeT1STData then omits from the NLRI (internal/component/bgp/plugins/nlri/mup/encode.go:435-450, internal/component/bgp/plugins/nlri/mup/encode.go:396-399)}
- [ ] [DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-2] [MUST] T1ST with invalid Endpoint Address Length (not 32 or 128): treat-as-withdraw (Section 3.1.3.1) {gap: ParseMUP keeps the route-type-specific bytes opaque after the RD (internal/component/bgp/plugins/nlri/mup/types.go:137-149) and never reads the T1ST Endpoint Address Length, while writeT1STData derives it from the configured address (internal/component/bgp/plugins/nlri/mup/encode.go:404-409), so a received length other than 32 or 128 is accepted}
- [ ] [DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-3] [MUST] T1ST with invalid Source Address Length (not 0, 32, or 128): treat-as-withdraw (Section 3.1.3.1) {gap: ParseMUP keeps the route-type-specific bytes opaque after the RD (internal/component/bgp/plugins/nlri/mup/types.go:137-149) and never reads the T1ST Source Address Length, while writeT1STData derives it from the configured address (internal/component/bgp/plugins/nlri/mup/encode.go:411-416), so a received length other than 0, 32 or 128 is accepted}
- [ ] [DRAFT-IETF-BESS-MUP-SAFI-3.1.4-1] [MUST] T2ST with Endpoint Length exceeding max for AFI: treat-as-withdraw (Section 3.1.4) {gap: ParseMUP keeps the route-type-specific bytes opaque after the RD (internal/component/bgp/plugins/nlri/mup/types.go:137-149) and never reads the T2ST Endpoint Length, so a combined length above 64 for AFI 1 or 160 for AFI 2 is accepted}
- [ ] [DRAFT-IETF-BESS-MUP-SAFI-3.1.4.1-1] [MUST] T2ST with TEID=0: treat-as-withdraw per RFC 7606 (Section 3.1.4.1) {gap: ParseMUP keeps the route-type-specific bytes opaque after the RD (internal/component/bgp/plugins/nlri/mup/types.go:137-149) and never reads the T2ST TEID, while writeT2STData writes whatever parseTEIDWithBits yields, zero included (internal/component/bgp/plugins/nlri/mup/encode.go:422-431)}
- [ ] [DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-4] [MUST] T1ST NLRI architecture field MUST be encoded as specified for 3gpp-5g; otherwise treat-as-withdraw (Section 3.1.3.1) {gap: writeMUPNLRI always writes architecture type 1 (internal/component/bgp/plugins/nlri/mup/encode.go:246) but ParseMUP accepts any architecture byte and keeps parsing (internal/component/bgp/plugins/nlri/mup/types.go:123), so a T1ST NLRI encoded for another architecture is not treated as withdraw}
- [ ] [DRAFT-IETF-BESS-MUP-SAFI-3.1.4.1-2] [MUST] T2ST NLRI architecture field MUST be encoded as specified for 3gpp-5g; otherwise treat-as-withdraw (Section 3.1.4.1) {gap: writeMUPNLRI always writes architecture type 1 (internal/component/bgp/plugins/nlri/mup/encode.go:246) but ParseMUP accepts any architecture byte and keeps parsing (internal/component/bgp/plugins/nlri/mup/types.go:123), so a T2ST NLRI encoded for another architecture is not treated as withdraw}
- [ ] [DRAFT-IETF-BESS-MUP-SAFI-3.3.3-3] [MUST] ISD/DSD without prefix SID attribute: treat-as-withdraw (Section 3.3.3, 3.3.6) {gap: DecodeNLRIHex surfaces only route type, architecture type and RD from a received MUP NLRI (internal/component/bgp/plugins/nlri/mup/mup.go:56-73) without consulting the UPDATE path attributes, so an ISD or DSD route arriving without a Prefix-SID attribute is not treated as withdrawn}
- [ ] [DRAFT-IETF-BESS-MUP-SAFI-3.3.3-4] [MUST] Nexthop/locator mismatch on ISD/DSD: treat-as-withdraw (Section 3.3.3, 3.3.6) {gap: DecodeNLRIHex surfaces only route type, architecture type and RD from a received MUP NLRI (internal/component/bgp/plugins/nlri/mup/mup.go:56-73) and ze reads no prefix SID locator anywhere in internal/component/bgp, so a mismatch between the nexthop and the locator originator is never detected on ISD or DSD routes}
- [ ] [DRAFT-IETF-BESS-MUP-SAFI-3.3-1] [MUST] PE and MUP Controller MUST establish a BGP session to exchange BGP-MUP NLRIs for both IPv4 and IPv6 AFIs (Section 3.3)
- [ ] [DRAFT-IETF-BESS-MUP-SAFI-3.3.1-4] [MUST] ISD prefix SID function MUST be GTP4.E if BGP AFI is IPv4, or MUST be GTP6.E if BGP AFI is IPv6 (Section 3.3.1) {gap: parseConfigRoute adds the Prefix-SID attribute only when the config supplies one (internal/component/bgp/plugins/nlri/mup/config.go:71) and passes its bytes through unchanged, and ze decodes no SRv6 endpoint function, so the GTP4.E/GTP6.E function is never tied to the BGP AFI}
- [ ] [DRAFT-IETF-BESS-MUP-SAFI-3.3.5-2] [MUST] When withdrawing DSD route, BGP speaker MUST attach a BGP MUP Extended community of the associated routing instance (Section 3.3.5) {gap: a MUP withdrawal is built as a bare MP_UNREACH_NLRI with no path attributes (internal/component/bgp/reactor/peer_rib_routes.go:182-197), so a DSD withdrawal carries no BGP MUP Extended Community}
- [ ] [DRAFT-IETF-BESS-MUP-SAFI-3.3.8-1] [MUST] Controller MUST advertise the withdrawal of the Type 1 ST route (Section 3.3.8) {gap: no MUP NLRI can reach the family-generic MP_UNREACH encoder (internal/component/bgp/reactor/peer_rib_routes.go:170). Its only callers take the NLRI from a PeerOpWithdraw queue entry (internal/component/bgp/reactor/peer_initial_sync.go:237, :377) filled by QueueWithdraw (internal/component/bgp/reactor/peer.go:886-893), and the two withdrawal entry points that feed it parse no SAFI 85: text mode rejects the family in isSupportedFamily, whose list stops at SAFI 73 (internal/component/bgp/plugins/cmd/update/update_text_nlri.go:375-403), and the announce/withdraw registry builds only unicast and FlowSpec NLRIs (internal/component/bgp/plugins/cmd/announce/announce.go:257, :304, :415). NewMUP and NewMUPFull (internal/component/bgp/plugins/nlri/mup/types.go:93, :103) have no non-test caller, and nlrisplit registers no SAFI 85 splitter (internal/core/bgp/nlri/nlrisplit/register.go:9-24) so no received MUP route is stored to be withdrawn either}
- [ ] [DRAFT-IETF-BESS-MUP-SAFI-3.3.7-3] [MUST] Controller MUST advertise the Type 1 ST route with Destination prefix, TEID, QFI, Endpoint Address, and optionally Source Address (Section 3.3.7) {gap: parseT1STFields requires only the destination prefix (internal/component/bgp/plugins/nlri/mup/encode.go:313-316) and writeT1STData omits the TEID field when no TEID is configured and the Endpoint Address field when no endpoint is configured (internal/component/bgp/plugins/nlri/mup/encode.go:396-409), so a Type 1 ST route is advertised without them}
- [ ] [DRAFT-IETF-BESS-MUP-SAFI-3.3.9-2] [MUST] PE receiving T1ST in MP_UNREACH_NLRI without Source address MUST delete all matching T1ST routes with different Source addresses (Section 3.3.9) {gap: nlrisplit now registers SplitMUP for SAFI 85 (internal/core/bgp/nlri/nlrisplit/register.go), so insertPoolNLRIs stores a received MUP NLRI as an opaque entry keyed on the whole NLRI and removePoolNLRIs deletes exactly the NLRI a withdrawal names (internal/component/bgp/plugins/rib/rib.go). The opaque key is the whole NLRI, Source address included, so a Source-less Type 1 ST withdrawal matches no stored entry. No wildcard delete over differing Source addresses exists}
- [ ] [DRAFT-IETF-BESS-MUP-SAFI-3.3.11-1] [MUST] Controller MUST advertise the withdrawal of the Type 2 ST route (Section 3.3.11) {gap: the same missing path as DRAFT-IETF-BESS-MUP-SAFI-3.3.8-1 -- the family-generic MP_UNREACH encoder (internal/component/bgp/reactor/peer_rib_routes.go:170) is reachable only through PeerOpWithdraw (internal/component/bgp/reactor/peer.go:886-893, internal/component/bgp/reactor/peer_initial_sync.go:237, :377), and neither withdrawal entry point produces a SAFI 85 NLRI: isSupportedFamily omits it (internal/component/bgp/plugins/cmd/update/update_text_nlri.go:375-403) and the announce registry builds only unicast and FlowSpec NLRIs (internal/component/bgp/plugins/cmd/announce/announce.go:257, :304, :415)}
- [ ] [DRAFT-IETF-BESS-MUP-SAFI-3.3.3-5] [SHOULD] Receiver of ISD routes should ignore nexthop in MP_REACH_NLRI and use prefix SID locator instead (Section 3.3.3)
- [ ] [DRAFT-IETF-BESS-MUP-SAFI-3.3.7-4] [SHOULD] Controller should attach Route Target Extended Community which PEs are importing for T1ST (Section 3.3.7)
- [ ] [DRAFT-IETF-BESS-MUP-SAFI-3.3.9-3] [SHOULD] PE should use received Tunnel Endpoint Address in T1ST NLRI as key to lookup associated ISD route (Section 3.3.9)
- [ ] [DRAFT-IETF-BESS-MUP-SAFI-3.3.6-3] [SHOULD] Receiver of DSD routes should ignore the nexthop in MP_REACH_NLRI attribute (Section 3.3.6)
- [ ] [DRAFT-IETF-BESS-MUP-SAFI-3.3.9-4] [SHOULD] PE receiving T1ST routes should ignore the received nexthop in MP_REACH_NLRI (Section 3.3.9)
- [ ] [DRAFT-IETF-BESS-MUP-SAFI-3.3.9-5] [SHOULD] PE should generate forwarding SID for GTP4/6.E based on SRv6 MUP procedures (Section 3.3.9)
- [ ] [DRAFT-IETF-BESS-MUP-SAFI-3.3.9-6] [SHOULD] If PE cannot generate prefix SID, it should mark the received T1ST route as invalid (Section 3.3.9)
- [ ] [DRAFT-IETF-BESS-MUP-SAFI-3.3.12-2] [SHOULD] Receiver of T2ST routes should ignore received nexthop in MP_REACH_NLRI (Section 3.3.12)
- [ ] [DRAFT-IETF-BESS-MUP-SAFI-3.3.12-3] [SHOULD] PE receiving T2ST without BGP MUP Extended community should consider the route malformed (Section 3.3.12)
- [ ] [DRAFT-IETF-BESS-MUP-SAFI-3.3.8-2] [SHOULD] When withdrawing T1ST, controller should attach Route Target Extended community for the corresponding Direct segment (Section 3.3.8)
- [ ] [DRAFT-IETF-BESS-MUP-SAFI-3.3.10-3] [SHOULD] When advertising T2ST, controller should attach BGP MUP Extended community for the Direct segment (Section 3.3.10)
- [ ] [DRAFT-IETF-BESS-MUP-SAFI-3.3.11-2] [SHOULD] When withdrawing T2ST, controller should attach BGP MUP Extended community and Route Target Extended community (Section 3.3.11)
- [ ] [DRAFT-IETF-BESS-MUP-SAFI-4-1] [SHOULD] RFC 5925 authentication should be used where authentication of BGP control packets is needed (Section 4)
- [ ] [DRAFT-IETF-BESS-MUP-SAFI-4-2] [SHOULD NOT] PEs and MUP Controller should not establish BGP sessions with untrusted domains without explicit configuration (Section 4)
- [ ] [DRAFT-IETF-BESS-MUP-SAFI-4-3] [SHOULD] RFC 5925 procedures should be enforced at untrusted domain boundaries (Section 4)
- [ ] [DRAFT-IETF-BESS-MUP-SAFI-4-4] [SHOULD] Establishing BGP sessions over encrypted paths should be considered to protect from eavesdropping (Section 4)
- [ ] [DRAFT-IETF-BESS-MUP-SAFI-4-5] [SHOULD] PEs should impose an upper bound on number of routes stored to protect control plane load (Section 4)
- [ ] [DRAFT-IETF-BESS-MUP-SAFI-3.1-2] [MAY] Implementation may log an error when unknown Route Types are ignored (Section 3.1)
- [ ] [DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-5] [MAY] BGP speaker may have local configuration for using a Source address when Source Address Length is 0 in T1ST (Section 3.1.3.1)
- [ ] [DRAFT-IETF-BESS-MUP-SAFI-3.3.1-5] [MAY] ISD IP prefix may include a gNodeB address connecting to the PE (Section 3.3.1)
- [ ] [DRAFT-IETF-BESS-MUP-SAFI-3.3.4-5] [MAY] DSD prefix SID function may be End.DT4/6 or End.DX4/6 (Section 3.3.4)
