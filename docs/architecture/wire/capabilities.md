# BGP Capabilities Wire Format

## TL;DR (Read This First)

| Concept | Description |
|---------|-------------|
| **Format** | TLV: Type (1) + Length (1) + Value (variable) |
| **Key Codes** | 1=MP, 2=RouteRefresh, 65=ASN4, 69=ADD-PATH, 6=ExtMsg |
| **ADD-PATH Flags** | 1=receive, 2=send, 3=both |
| **Negotiation** | Capability-specific rules; Extended Message uses independent local and peer advertisements |
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

<!-- source: internal/core/bgp/capability/capability.go -- Capability interface, writeCapabilityTo -->

---

## Capability Codes

| Code | Hex | Name | RFC | Length | Ze handling |
|------|-----|------|-----|--------|-------------|
| 1 | 0x01 | Multiprotocol Extensions | RFC 4760 | 4 per family | Core parser |
| 2 | 0x02 | Route Refresh | RFC 2918 | 0 | Core parser |
| 3 | 0x03 | Outbound Route Filtering | RFC 5291 | Variable | Preserved as unknown |
| 4 | 0x04 | Multiple Routes to Destination | RFC 3107 | 0 | Preserved as unknown |
| 5 | 0x05 | Extended Next Hop Encoding | RFC 8950 | 6 per entry | Core parser |
| 6 | 0x06 | Extended Message | RFC 8654 | 0 | Core parser |
| 9 | 0x09 | Role | RFC 9234 | 1 | Role plugin |
| 64 | 0x40 | Graceful Restart | RFC 4724 | 2 + 4*n | Core parser |
| 65 | 0x41 | 4-Byte AS Number | RFC 6793 | 4 | Core parser |
| 68 | 0x44 | Multisession | draft-ietf-idr-bgp-multisession | 1 + n | Preserved as unknown |
| 69 | 0x45 | ADD-PATH | RFC 7911 | 4 per family | Core parser |
| 70 | 0x46 | Enhanced Route Refresh | RFC 7313 | 0 | Core parser |
| 73 | 0x49 | FQDN | draft-walton-bgp-hostname-capability | Variable | Core parser |
| 74 | 0x4A | BFD Strict-Mode | draft-ietf-idr-bgp-bfd-strict-mode | 0 | Core parser |
| 75 | 0x4B | Software Version | draft-abraitis-bgp-version-capability | Variable | Preserved as unknown |
| 76 | 0x4C | PATHS-LIMIT | draft-abraitis-idr-addpath-paths-limit-04 | 0 or 5 per family | Core parser |
| 77 | 0x4D | Link-Local Next Hop | draft-ietf-idr-linklocal-capability | 0 | Core parser, llnh plugin declares |
| 128 | 0x80 | Route Refresh (Cisco) | Vendor | 0 | Preserved as unknown |
| 131 | 0x83 | Multisession (Cisco) | Vendor | Variable | Preserved as unknown |
| 185 | 0xB9 | Operational Message (ExaBGP) | Vendor | 0 | Preserved as unknown |

The core parser preserves every unrecognized capability as `Unknown`. The BGP
Role plugin handles code 9 through its capability declaration and OPEN callback.

Ze declares neither multisession code, and no operational code, so it never
reciprocates any of them. Two consequences follow for an ExaBGP peer:

- Ze ignoring a received code 68 is what RFC 5492 Section 3 asks of a speaker
  that does not support a capability.
- An ExaBGP configured with `capability { multi-session; }` still drops the
  session, because its own rule is that the capability binds both speakers or
  neither. draft-ietf-idr-bgp-multisession-07 Section 11 states it: "If a BGP
  speaker receives OPEN message that doesn't include Multisession Capability
  and local BGP speaker is required to use multisession (e.g. through
  configuration by operator), the local BGP speaker MUST drop the session".
  `ze exabgp migrate` therefore WARNS, naming the peer and the keyword: the
  migrated peer does not require multi-session, so its session comes up where
  the ExaBGP one would have dropped. That is a behaviour change the operator has
  to see, which is what the warning is for.

Code 185 is not an IANA assignment. ExaBGP picked it for itself and marks it
"ExaBGP only", because draft-ietf-idr-operational-message-00 Section 9 requests
a capability code and a BGP message type from IANA and neither was allocated
before the draft expired. Ze does not put a private codepoint on the wire.

Upstream (exa-networks/exabgp):
`src/exabgp/bgp/message/open/capability/capability.py`, `MULTISESSION` 0x44 and
`OPERATIONAL` 0xB9.

<!-- source: internal/exabgp/migration/migrate_unimplemented.go -- untranslatedCapabilityWarning -->
<!-- source: internal/core/bgp/capability/capability.go -- Code constants, parseCapability, Unknown -->
<!-- source: internal/component/bgp/plugins/role/config.go -- extractRoleCapabilities -->
<!-- source: internal/component/bgp/plugins/role/validate.go -- extractRolesFromCaps -->

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

<!-- source: internal/core/bgp/capability/capability.go -- Multiprotocol struct, parseMultiprotocol -->

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

<!-- source: internal/core/bgp/capability/capability.go -- AFI*, SAFI* constants -->

---

## 2. Route Refresh (Code 2)

RFC 2918 - Ability to request route refresh from peer.

```
[Empty - Length = 0]
```

No value field. Presence of capability indicates support.

<!-- source: internal/core/bgp/capability/capability.go -- RouteRefresh struct -->

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

<!-- source: internal/core/bgp/capability/capability.go -- ExtendedNextHop struct, ExtendedNextHopFamily -->

An IPv6 next hop for IPv4 or VPN-IPv4 NLRI is sent only when the peer
negotiated the entry for that exact AFI/SAFI pair (RFC 8950 Section 4). The
check covers the five families RFC 8950 Section 3 extends: AFI 1 with SAFI 1,
2, 4, 128 or 129 (`rfc8950Family`). Another AFI 1 family is outside it. SR
Policy (SAFI 73) takes an IPv4 or an IPv6 next hop for either AFI, with no
Extended Next Hop pair (RFC 9830 Section 2.1). The check applies to `next-hop self` and to an explicit
next hop alike, on every announce rail. A route that fails it is not sent to
that peer, and the announce returns `ErrNextHopIncompatible` when no peer took
it. A queued announcement is checked against the session that drains the queue,
because only that session holds the negotiated pairs.

A licensed IPv4 unicast announce with an IPv6 next hop is sent as MP_REACH_NLRI
for AFI 1 / SAFI 1, with no NEXT_HOP attribute and an empty NLRI field
(RFC 8950 Section 3). An IPv4 next hop keeps the inline NLRI and NEXT_HOP.

The forward rails (general and route server) apply the same check to a received
route. The next hop checked is the one about to be written: the received one
when it passes along unchanged, or the one a next-hop mode or a filter sets.
When a destination lacks the pair, the announcement is withheld from it and the
withdrawals in the same UPDATE are still sent. A warning names the peer and the
family.

<!-- source: internal/component/bgp/reactor/peer.go -- resolveNextHop, canUseNextHopFor, rfc8950Family -->
<!-- source: internal/component/bgp/reactor/peer_initial_sync.go -- queuedNextHopPolicy -->
<!-- source: internal/component/bgp/reactor/reactor_api_batch.go -- inlineIPv4Unicast -->
<!-- source: internal/component/bgp/reactor/forward_next_hop.go -- egressNextHopLacksExtendedNextHop -->

---

## 4. Extended Message (Code 6)

RFC 8654 - Support for BGP messages > 4096 bytes.

```
[Empty - Length = 0]
```

No value field. The local advertisement permits receiving up to 65,535 octets;
the peer advertisement permits sending up to 65,535 octets. OPEN remains limited
to 4,096 octets and KEEPALIVE remains exactly 19 octets. See
[Extended Message](../edge-cases/extended-message.md) for asymmetric combinations.
<!-- source: internal/core/bgp/capability/negotiated.go -- Negotiate -->
<!-- source: internal/component/bgp/message/header.go -- ValidateLengthWithMax -->

<!-- source: internal/core/bgp/capability/capability.go -- ExtendedMessage struct -->

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

<!-- source: internal/core/bgp/capability/capability.go -- GracefulRestart struct, GracefulRestartFamily -->

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

<!-- source: internal/core/bgp/capability/capability.go -- ASN4 struct, parseASN4 -->

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

<!-- source: internal/core/bgp/capability/capability.go -- AddPath struct, AddPathMode, AddPathFamily -->

---

## 9b. BFD Strict-Mode (Code 74)

draft-ietf-idr-bgp-bfd-strict-mode Section 5.

```
[Empty - Length = 0]
```

No value field. Advertising it says this speaker runs the strict-mode
procedures of Section 8: the BGP session does not send its KEEPALIVE and does
not advance to OpenConfirm until the BFD session to that neighbour is Up.

Negotiation is the plain RFC 5492 intersection. Both speakers advertising it
sets the draft's `BfdStrictNegotiated` session attribute, which is
`Negotiated.BFDStrictMode`. A peer that does not advertise it leaves the
attribute false and the session establishes on the unmodified RFC 4271 path,
which is what Section 1 asks for: "always using 'strict-mode' would preclude BGP
operation in an environment where not all routers support BFD strict-mode".

Ze advertises the capability from the peer's `connection bfd { strict true; }`
leaf rather than from the `capability` block, because it is not an independent
choice. It commits the speaker to procedures that only exist when BFD is enabled
for that peer, so a `bfd` block with `enabled false` advertises nothing.

The FSM half is `docs/architecture/behavior/fsm.md`. The operator-facing half is
`docs/guide/bfd.md`.

<!-- source: internal/core/bgp/capability/capability.go -- BFDStrictMode, CodeBFDStrictMode -->
<!-- source: internal/core/bgp/capability/negotiated.go -- Negotiate, BFDStrictMode -->
<!-- source: internal/component/bgp/reactor/config.go -- parsePeerFromTree, the bfd block that advertises it -->

---

## 9c. Link-Local Next Hop (Code 77)

draft-ietf-idr-linklocal-capability Section 2.

```
[Empty - Length = 0]
```

No value field. Advertising it says this speaker is willing to send and to
receive an MP_REACH_NLRI Next Hop field holding one IPv6 Link-Local address in
16 octets, which RFC 2545 Section 3 has no form for: there a link-local address
is only ever the second of two.

Negotiation is the plain RFC 5492 intersection, and it is load-bearing rather
than informational. Section 2 scopes every procedure of Sections 3 to 6 to a
session that negotiated it, so `Negotiated.LinkLocalNextHop` is what
`Peer.linkLocalOnlyNextHopPermitted` reads before ze sends the 16-octet form.
For IPv4 NLRI carried behind an IPv6 next hop, Section 5 asks for the
COMBINATION with Extended Next Hop Encoding (code 5): without both, the field is
encoded as 32 octets.

The llnh plugin declares the capability, from `session capability
link-local-nexthop`, and the core parses and negotiates it. The plugin refuses a
peer that enables the capability with no `session link-local` address, on the
peer or its group, both at commit (config-verify) and at startup, where the
refusal stops ze. That leaf is Ze's only source for its own Link-Local, and
Section 4 makes including it a MUST for a directly connected route, so a
session negotiating the capability without it would promise a next hop Ze
cannot send. Section 2 asks a speaker to advertise the capability only when "It
is capable of sending IPv6 Link-Local-only next hops for a route". A route whose next
hop is link-local is left out of the announcement on a session that did not
negotiate what it needs, rather than encoded in a form RFC 2545 Section 3
forbids.

The reflection half is `docs/architecture/core-design.md`: a route with a
link-local-only next hop is withheld from a route-reflector client that shares no
link-layer segment with the original advertiser, unless the operator configured
next-hop-self for that client.

<!-- source: internal/component/bgp/plugins/llnh/llnh.go -- refuseLinkLocalCapabilityWithoutAddress -->
<!-- source: internal/core/bgp/capability/capability.go -- LinkLocalNextHop, CodeLinkLocalNextHop -->
<!-- source: internal/core/bgp/capability/negotiated.go -- Negotiate, LinkLocalNextHop -->
<!-- source: internal/core/bgp/attribute/nexthop_form.go -- LinkLocalOnlyNextHopPermitted -->
<!-- source: internal/component/bgp/reactor/peer.go -- linkLocalOnlyNextHopPermitted -->

---

## 7b. PATHS-LIMIT (Code 76)

draft-abraitis-idr-addpath-paths-limit-04: receiver-requested path count limit for ADD-PATH.

```
+---------------------------------------------+
| AFI (2 bytes) | SAFI (1) | Max Paths (2)    |
+---------------------------------------------+
```

The value contains zero or more 5-byte entries. An empty capability encodes as `4C 00` and requests no limit.

- The first tuple for an AFI/SAFI wins, even when its limit is zero.
- A zero limit is ignored. A later duplicate cannot replace it with a nonzero limit.
- The first received PATHS-LIMIT capability instance wins, including an empty instance.
- The remote limit constrains Ze's send only for a family with negotiated ADD-PATH send.
- The local limit requests a bound on the peer's send only for a family with negotiated ADD-PATH receive.

A local limit is a receiver request, not a guarantee that the peer obeys it.

The session writer enforces the remote limit per AFI/SAFI, prefix, and distinct path ID across messages and batches. This includes route-server fast-path forwarding. The first admitted paths keep their slots, and replacements with the same path ID remain permitted at the limit. A withdrawal frees its slot, and a new destination connection starts with no admitted paths.

Ze does not queue suppressed announcements. A later re-announcement can retry after capacity becomes available. Normal send and forwarding paths share enforcement, but deliberate raw-message injection remains an exact-byte diagnostic command.

<!-- source: internal/core/bgp/capability/capability.go -- PathsLimit struct, PathsLimitEntry -->
<!-- source: internal/core/bgp/capability/negotiated.go -- negotiatePathsLimit, negotiatePathsLimitDirection -->
<!-- source: internal/component/bgp/reactor/session_paths_limit.go -- initPathsLimit, pathsLimitSection, filterPathsLimit -->

---

## 8. Enhanced Route Refresh (Code 70)

RFC 7313 - Beginning/End of Route Refresh markers.

```
[Empty - Length = 0]
```

Enables BoRR (1) and EoRR (2) in ROUTE-REFRESH reserved field.

<!-- source: internal/core/bgp/capability/capability.go -- EnhancedRouteRefresh struct -->

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

<!-- source: internal/core/bgp/capability/capability.go -- FQDN struct, parseFQDN -->

---

## 10. Software Version (Code 75)

draft-abraitis-bgp-version-capability, revision 18

```
 0                   1                   2   ...
 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|  Version String (variable, Capability Length octets) |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
```

| Field | Bytes | Description |
|-------|-------|-------------|
| Version String | Capability Length | UTF-8 software version, not null-terminated |

The draft's Capability Value is the string itself: "The Capability Value field
is the software version encoded as a UTF-8 [RFC3629] string" (Section 3). It
carries no length octet of its own, because the Capability Length already
states its size.

FRRouting and ExaBGP both put a one-octet length in front of the string, which
the draft does not define:

```
+--------+----------------------------------------+
| Length |  Version String (Length octets)        |
+--------+----------------------------------------+
  1 octet   Capability Length - 1 octets
```

FRRouting 10.3.1 reads the first octet as that length
(`bgp_capability_software_version`, `bgpd/bgp_open.c`) and, when the length
runs past the capability, answers with an OPEN Message Error NOTIFICATION. The
draft form of `Ze/0.1.0` starts with `Z`, 0x5A or 90, against 7 octets that
follow, so FRRouting closes the session.

Ze therefore sends either form, chosen per peer by the `encoding` leaf of
`session > capability > software-version`. `draft`, the default, sends the
bare string. `legacy` sends the length octet first, and a peer running FRR or
ExaBGP needs it. `legacy` is an owner-approved deviation from the draft
(`rfc/short/draft-abraitis-bgp-version-capability.md`, Notes) and is not
counted as conformant. `encodeValue`
(`internal/component/bgp/plugins/softver/softver.go`) writes the form, and
`ze exabgp migrate` writes `encoding legacy` for every migrated peer that
enables software-version (`keepExaBGPSoftwareVersionFraming`, `internal/exabgp/migration/migrate.go`),
because that is what ExaBGP sent.

`decodeSoftwareVersion` reads both forms. It reads the legacy form when the
first octet equals the number of octets after it and those octets are valid
UTF-8, and the bare string otherwise. The two overlap in one place: a bare
version whose first octet equals its own length minus one reads as legacy and
loses that octet. Every printable first octet is 0x20 or more, so this needs a
bare version of 33 octets or more starting with the character that encodes its
remaining length, for example a 34-octet version starting with `!`.

Section 3 of the draft gives a receiver one answer for a Capability Value it
cannot read: "A value of zero SHALL be treated as an encoding error and the
Capability MUST be ignored", and "A receiving BGP speaker MUST NOT interpret
invalid UTF-8 sequences". `decodeSoftwareVersion` reports an encoding error for
three shapes, and ze then shows no version at all rather than an empty one: a
Capability Value of zero octets, a legacy value whose length octet is zero,
and a Version String that is not valid UTF-8.

The session receive path never reaches that decoder. `parseCapability`
(`internal/core/bgp/capability/capability.go`) has no case for code 75, so a
received Software Version Capability becomes an `Unknown` that no ze decision
reads. `ze bgp decode capability 75 <hex>` is the operator path into it.

<!-- source: internal/component/bgp/plugins/softver/softver.go -- encodeValue, parseValueEncoding, decodeSoftwareVersion, the encoding errors and the ignore -->

---

## Capability Negotiation

### Rules

1. **Intersection:** Negotiated capabilities = capabilities both peers advertise
2. **Implicit IPv4 unicast:** the address-family intersection has one exception.
   A side that advertises no Multiprotocol capability counts as advertising
   `ipv4/unicast`. Each side is judged on its own, before the intersection runs.
   RFC 4271 carries that family in the UPDATE message itself. A speaker that
   declares no capability still exchanges it, and the session owes it the
   End-of-RIB marker of RFC 4724 Section 4.
   The default is one family and never a wildcard. A side that advertises only
   `ipv6/unicast`, against a silent side, still intersects to nothing. The two
   have no family in common.
   The OPEN bytes are unchanged. Ze advertises no Multiprotocol capability for
   such a peer.
3. **Duplicates:** If same capability appears multiple times, use last one (RFC 5492)
4. **Unknown:** Unknown capabilities are ignored (not an error)
5. **Required:** Session fails if required capability not negotiated
6. **Refused:** Session fails if refused capability is present in peer's OPEN

<!-- source: internal/core/bgp/capability/negotiated.go -- Negotiate, the per-side implicit ipv4/unicast default before the family intersection -->
<!-- source: internal/component/bgp/reactor/peer_initial_sync.go -- sendInitialRoutes, one End-of-RIB marker per negotiated family, sent without waiting for an attached process -->

The automatic initial-sync marker follows Ze's own routes, without waiting for
an attached process to supply routes. A separate bounded peer-up acknowledgement
barrier still runs before the operation-queue drain. Initial sync captures a
Session for config-static sends, serializes the static set with reload under
`staticMu`, and passes the captured Session through queued announcement splitting.
`manual-eor` suppresses the automatic marker only: the write hold and pending
marker state are released even when no marker is sent. A failed marker send
returns its family claim and does not increment the sent counter.
<!-- source: internal/component/bgp/reactor/peer_initial_sync.go -- sendInitialRoutes -->

### Negotiated State

Defined in `internal/core/bgp/capability/negotiated.go`:

```go
type Negotiated struct {
    // Sub-components (composite pattern)
    Identity *PeerIdentity // ASNs, Router IDs
    Encoding *EncodingCaps // ASN4, families, ADD-PATH
    Session  *SessionCaps  // Route Refresh, GR

    // Backward-compat fields (delegates to sub-components)
    LocalASN             uint32
    PeerASN              uint32
    ASN4                 bool
    ExtendedMessageRecv  bool // Local advertisement
    ExtendedMessageSend  bool // Peer advertisement
    RouteRefresh         bool
    EnhancedRouteRefresh bool
    BFDStrictMode        bool
    LinkLocalNextHop     bool
    HoldTime             uint16
    GracefulRestart      *GracefulRestart

    // Internal maps
    families        map[Family]bool
    addPath         map[Family]AddPathMode
    extendedNextHop map[Family]AFI
    peerCodes       map[Code]bool  // Raw capability codes from peer's OPEN
}
```

<!-- source: internal/core/bgp/capability/negotiated.go -- Negotiated struct -->

`Identity` is an INPUT to negotiation, not a result of it. `Negotiate` takes a
`PeerIdentity` from its caller and copies it unchanged, because neither half is
readable from the OPEN alone. A four-octet speaker sends AS_TRANS (23456) in My
Autonomous System (RFC 6793 Section 3), so the peer's real AS is in the capability
value or nowhere. RFC 7705 Section 4.2 lets a renumbering router hold an internal
session under its second AS, so equal AS numbers are the ordinary internal case and
not the whole rule. The reactor answers both before it negotiates: `sessionPeerAS`
takes the configured AS before the advertised one, `PeerSettings.isIBGPWith` decides
`Internal`, and `PeerIdentity.IsIBGP` returns that verdict unchanged.

<!-- source: internal/core/bgp/capability/identity.go -- PeerIdentity, IsIBGP -->
<!-- source: internal/component/bgp/reactor/session_negotiate.go -- negotiateWith, the PeerIdentity it states -->

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

When a capability mode violation is detected, the session sends a NOTIFICATION with the RFC 5492 error code and subcode. Its Data contains complete capability tuples, using the same capability encoders as OPEN.

| Field | Value |
|-------|-------|
| Error Code | 2 (OPEN Message Error) |
| Error Subcode | 7 (Unsupported Capability) |
| Data | Capability TLVs for each violating code |

**Data format** — each violating capability carries its code, length, and value:

```
 0                   1
 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
| Cap. Code     | Cap. Length   |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
| Capability Value (variable)   |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
```

RFC 5492 Section 5 says: "Each such capability is encoded in the same way as it would be encoded in the OPEN message." Missing required capabilities use the values advertised in the local OPEN; refused capabilities use the received peer values. Each selected code includes all its advertised instances. Zero length is valid for route-refresh and extended-message, but ASN4 carries its four-octet AS number and variable-length capabilities retain their values. The former code-only encoder incorrectly emitted length zero for ASN4 and other value-bearing capabilities.

Per-family ADD-PATH rejection carries code 69 with the causing family's AFI, SAFI, and original advertised Send/Receive value. Missing requirements use the local direction; refusal uses the peer direction, not the negotiated direction. Unrelated families are excluded, while all matching entries stay grouped in their original capability instance. Filtering therefore cannot inflate the capability data beyond its OPEN encoding, even when entries or instances repeat. The existing require/refuse policy is unchanged.

**Example:** local AS 65001, `asn4 require;`, peer lacking ASN4 →
NOTIFICATION hex: `FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF 001B 03 02 07 41 04 0000FDE9`

| Part | Hex | Meaning |
|------|-----|---------|
| Marker | `FFFF...` (16 bytes) | BGP marker |
| Length | `001B` | 27 bytes |
| Type | `03` | NOTIFICATION |
| Error Code | `02` | OPEN Message Error |
| Error Subcode | `07` | Unsupported Capability |
| Data: Cap Code | `41` | Code 65 (ASN4) |
| Data: Cap Length | `04` | Length 4 |
| Data: Cap Value | `0000FDE9` | Local AS 65001 |

### Implementation

| Component | File | Purpose |
|-----------|------|---------|
| `CheckRequiredCodes(codes)` | `capability/negotiated.go` | Returns missing required codes |
| `CheckRefusedCodes(codes)` | `capability/negotiated.go` | Returns peer codes that match refused list |
| `buildUnsupportedCapabilityDataCodes(codes, caps)` | `reactor/session_validation.go` | Selects causing OPEN capability objects and encodes complete tuples |
| `buildUnsupportedAddPathData(family, caps)` | `reactor/session_validation.go` | Encodes the causing ADD-PATH family and original direction |
| Enforcement in `processOpen()` | `reactor/session_connection.go` | Pre-parsed OPEN path after collision resolution |
| Enforcement in `handleOpen()` | `reactor/session_handlers.go` | Received OPEN body path |

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
| BFD Strict-Mode (code 74) | absent (opt-in) | Advertised by `connection bfd { strict true; }`, never by the `capability` block |

"Absent" means the capability is not advertised and has no enforcement — it's as if it doesn't exist. Setting it to `enable`, `require`, or `refuse` activates it.

<!-- source: internal/core/bgp/capability/negotiated.go -- CheckRequiredCodes, CheckRefusedCodes -->

---

## Go Implementation Notes

### Capability Interface

Defined in `internal/core/bgp/capability/capability.go`:

```go
type Capability interface {
    Code() Code
    Len() int
    WriteTo(buf []byte, off int) int
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
    CodeBFDStrictMode        Code = 74 // draft-ietf-idr-bgp-bfd-strict-mode
    CodeSoftwareVersion      Code = 75 // draft
)
```

### Parsing Capabilities

Parsing is done via `Parse()` in `parse.go`, returning `[]Capability`.

<!-- source: internal/core/bgp/capability/capability.go -- Code, Capability interface, Parse -->

---

**Created:** 2025-12-19
**Last Updated:** 2026-02-21
