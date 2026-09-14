# draft-ietf-idr-linklocal-capability - Link-Local Next Hop Capability for BGP

## Meta

| Field | Value |
|-------|-------|
| Draft | draft-ietf-idr-linklocal-capability-06 |
| Title | Link-Local Next Hop Capability for BGP |
| Status | Internet Draft (Standards Track, expires 2026-12-05) |
| Date | 2026-06-03 |
| Updates | RFC 2545 (if approved) |
| Depends | RFC 4760 (MP-BGP), RFC 2545 (IPv6 next hops), RFC 5492 (capabilities), RFC 8950, RFC 7606 |
| Enrolment | enrolled |
| Enrolment reason | Link-Local Next Hop capability for BGP (code 77): twelve MUST-level requirements, all conditioned by Section 2 on the capability being negotiated. Ze advertises the capability (extractLLNHCapabilities, internal/component/bgp/plugins/llnh/llnh.go), parses it back as a typed capability (parseCapability, internal/core/bgp/capability/capability.go) and records what it negotiated to (Negotiate, internal/core/bgp/capability/negotiated.go), which is the Section 2 condition every other procedure reads. 3-1 and 5-1 are met on the send side: Peer.linkLocalOnlyNextHopPermitted (internal/component/bgp/reactor/peer.go) admits a link-local next hop only where the session may carry the 16-octet form, and buildMPReach (internal/component/bgp/message/update_build.go) writes that address alone in 16 octets, or the global-then-link-local pair in 32. 4-5 and 4-6 are met on the reflection path: egressNextHopIsLinkLocalOnly (internal/component/bgp/reactor/forward_next_hop.go) classifies the field about to be written and sameLinkLayerSegment (internal/component/bgp/reactor/link_scope.go) answers the segment half, so forwardUpdateCore either sends the rewrite applyFactsNextHop recorded or withholds the announcement. 3-2 is met by the RFC 2545 32-octet path; 4-2 and 4-8 are met because a link-local is appended only when the peer shares a connected subnet. 4-1, 4-4, 4-7 and 4-9 are met because a route whose next hop has no wire form is refused rather than encoded, and 4-3 because a route reachable through the speaker carries the speaker's own link-local toward a one-hop peer. One MUST-level requirement stays outstanding and carries a `{gap}` on its checklist line. |
| Support | drafts 30 |
| Support area | Link-local next-hop capability code 77 |
| Support status | Partial |
| Support coverage | Capability 77 declared, negotiated and acted on, for the send side and for the reflection path. Declaration: `extractLLNHCapabilities` (`internal/component/bgp/plugins/llnh/llnh.go`) advertises the empty code 77 capability for a peer or group whose config carries a `link-local-nexthop` key that is not `disable`. Negotiation: `parseCapability` (`internal/core/bgp/capability/capability.go`) types it and `Negotiate` (`internal/core/bgp/capability/negotiated.go`) sets `LinkLocalNextHop` only when both OPENs carried it, which `Peer.linkLocalOnlyNextHopPermitted` (`internal/component/bgp/reactor/peer.go`) reads beside the RFC 8950 Extended Next Hop Encoding state Section 5 asks for. Send: `resolveNextHop` (same file) refuses a link-local next hop on a session that may not carry it, so the route is left out rather than encoded in a form RFC 2545 Section 3 forbids, and `buildMPReach` (`internal/component/bgp/message/update_build.go`) writes the 16-octet Link-Local-only field or the 32-octet pair, for IPv4 NLRI as well as IPv6. Reflection: `egressNextHopIsLinkLocalOnly` (`internal/component/bgp/reactor/forward_next_hop.go`) and `sameLinkLayerSegment` (`internal/component/bgp/reactor/link_scope.go`) decide, per client, between the rewrite and the ineligibility Section 4 allows. Receive: `parseNextHops` (`internal/core/bgp/attribute/mpnlri.go`) still performs no `fe80::/10` test, so a received 16-octet link-local-only Next Hop is read as a Global IPv6 next hop; the Section 3 receive sentence is indicative and carries no checklist row (see Notes). One MUST-level requirement carries a `{gap}`: 6-1, because a malformed Next Hop field is answered with the RFC 7606 Section 7.11 session reset rather than the treat-as-withdraw of Section 7.3. |
| Support remaining | - |

**Purpose:** Defines BGP capability code 77, which signals that a speaker is
willing to send and receive an MP_REACH_NLRI Next Hop field holding an IPv6
Link-Local address ALONE, in 16 octets. RFC 2545 has no encoding for that case:
it carries a link-local address only as the second of two addresses in a
32-octet field.

**Scope:** The capability itself, plus next-hop encoding, next-hop selection for
internal and external peers, the interaction with RFC 8950, and error handling.
Section 2 bounds all of it: "In this document, all procedures described are
applicable only when the capability described herein has been successfully
advertised by both BGP speakers; i.e., negotiated. When the capability has not
been negotiated, the procedures in this document do not apply."

## Wire Format

Capability Code: 77 (0x4D). Capability Length: 0. No value.

Next Hop field forms this document governs:

| Length | Content | Defined by |
|--------|---------|-----------|
| 16 | one Global IPv6 address | RFC 2545 |
| 16 | one Link-Local IPv6 address, in `fe80::/10` | this draft, §3 |
| 32 | Global IPv6 address then Link-Local IPv6 address | RFC 2545 §3 |

A receiver distinguishes the two 16-octet forms by testing the address against
`fe80::/10` (§3).

## Ze Implementation

- Capability declaration: `extractLLNHCapabilities`
  (`internal/component/bgp/plugins/llnh/llnh.go`) appends an empty-payload
  `sdk.CapabilityDecl` for code 77 for every peer or group whose config carries
  a `link-local-nexthop` capability key that is not `disable`. OPEN injection is
  `Peer.getPluginCapabilities` (`internal/component/bgp/reactor/peer.go`), wired
  at `internal/component/bgp/reactor/peer_run.go` through
  `session.SetPluginCapabilityGetter`.
- Negotiation: `parseCapability`
  (`internal/core/bgp/capability/capability.go`) reads code 77 into
  `capability.LinkLocalNextHop`, a zero-length capability. Both OPENs are parsed
  back through it (`handleOpen`,
  `internal/component/bgp/reactor/session_handlers.go`), so `Negotiate`
  (`internal/core/bgp/capability/negotiated.go`) sees the same type on both
  sides and sets `Negotiated.LinkLocalNextHop` only when both carried it. The
  reactor holds it on `NegotiatedCapabilities`
  (`internal/component/bgp/reactor/negotiated.go`), and
  `Peer.linkLocalOnlyNextHopPermitted`
  (`internal/component/bgp/reactor/peer.go`) is the one reader.
- Which form may be sent: `attribute.LinkLocalOnlyNextHopPermitted`
  (`internal/core/bgp/attribute/nexthop_form.go`) states Section 2's condition
  and Section 5's combination once. IPv6 NLRI needs capability 77; IPv4 NLRI
  needs it and RFC 8950 Extended Next Hop Encoding for that family.
- Next-hop send: `Peer.resolveNextHop`
  (`internal/component/bgp/reactor/peer.go`) refuses a link-local next hop on a
  session that may not carry the 16-octet form, and every origination rail skips
  the route and logs it. `buildMPReach`
  (`internal/component/bgp/message/update_build.go`) writes that address alone in
  16 octets, or the global-then-link-local pair in 32; the second address rides
  whenever the NEXT HOP is IPv6, so IPv4 NLRI carried under RFC 8950 gets the
  32-octet form Section 5 requires outside the combination.
  `linkScope.linkLocalNextHop` and `applyLinkLocalNextHop`
  (`internal/component/bgp/reactor/link_scope.go`) still decide the RFC 2545
  Section 3 second address, and `attribute.ValidateGlobalNextHop`
  (`internal/core/bgp/attribute/nexthop_form.go`) still refuses a link-local
  address in the FIRST slot of the 32-octet pair.
- Reflection: `egressNextHopIsLinkLocalOnly`
  (`internal/component/bgp/reactor/forward_next_hop.go`) classifies the field
  about to be written to one client, and `sameLinkLayerSegment`
  (`internal/component/bgp/reactor/link_scope.go`) answers whether that client
  shares a connected subnet with the original advertiser. `forwardUpdateCore`
  (`internal/component/bgp/reactor/reactor_api_forward.go`) then sends the route
  with the rewrite `applyFactsNextHop` recorded, or withholds the announcement
  and logs the suppression. A link-local prefix is never read as evidence of a
  shared segment, because the interface behind it is not in the table ze reads.
- Next-hop receive: `parseNextHops` (`internal/core/bgp/attribute/mpnlri.go`)
  accepts lengths 16 and 32 for an IPv6 next hop and refuses every other length
  with `ErrInvalidNextHopLen`. It performs no `fe80::/10` test, so a 16-octet
  link-local-only Next Hop is read as a Global IPv6 next hop.
  `ValidNextHopLens` (same file) admits 32 octets for IPv4 unicast and
  multicast and 48 for VPN-IPv4, which is what RFC 8950 Section 3 defines for
  IPv4 NLRI behind an IPv6 next hop.
- Config: `session > capability > link-local-nexthop`
  (`internal/component/bgp/plugins/llnh/yang/ze-link-local-nexthop.yang`).

## Compliance
- [ ] [DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-1-1] [SHOULD NOT] "BGP speakers SHOULD NOT advertise a route whose Next Hop is a Link-Local address that is in the tentative state (Section 5.4 of [RFC4862]); this applies both to a first-party Next Hop (the speaker's own Link-Local address) and to a third-party Next Hop re-advertised from another peer" (§1)
- [ ] [DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-2-1] [SHOULD] "A BGP speaker that is willing to use (send and receive) IPv6 Link-Local-only next hops SHOULD advertise the Link-Local Next Hop Capability to its peers only when: 1. It is capable of sending IPv6 Link-Local-only next hops for a route. 2. IPv6 Link-Local neighbors are associated with interfaces as part of their configuration to assist in determining the interface scope of received IPv6 Link-Local-only next hops" (§2)
- [ ] [DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-3-1] [MUST] "If an implementation intends to send a single IPv6 Link-Local forwarding address in the Next Hop field of the MP_REACH_NLRI, it MUST set the length of the Next Hop field to 16 and include only the IPv6 Link-Local address in the Next Hop field" (§3)
- [ ] [DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-3-2] [MUST] "If an implementation intends to send both a IPv6 Global and Link-Local forwarding address in the Next Hop field of the MP_REACH_NLRI, it MUST set the length of the Next Hop field to 32 and include both the IPv6 Global and Link-Local addresses in the Next Hop field" (§3)
- [ ] [DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-1] [MUST] "If, after completing these procedures, there are no IPv6 next hop addresses included in the next hop, the BGP route MUST not be advertised to its peer" (§4)
- [ ] [DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-2] [MUST NOT] "If the internal peer is more than one IP hop away, the BGP speaker MUST NOT include a Link-Local IPv6 next hop" (§4)
- [ ] [DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-3] [MUST] "If the route is directly connected to the speaker, or if the interface address of the router through which the announced network is reachable for the speaker is the internal peer's address, the next hop MUST include its own Link-Local IPv6 address" (§4)
- [ ] [DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-4] [MUST NOT] "If, after evaluating the above procedures, there are no IPv6 next hops included with the route, the route MUST NOT be announced to the remote BGP speaker" (§4)
- [ ] [DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-5] [MUST NOT] "A Route Reflector (RR) reflecting a route with a link-local-only next hop MUST NOT advertise that route to a client unless the client shares the same link-layer segment as the original advertiser" (§4)
- [ ] [DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-6] [MUST] "For all other clients, the RR MUST either rewrite the next hop to its own address (next-hop-self) or consider the route ineligible for advertisement to that specific peer" (§4)
- [ ] [DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-7] [MUST NOT] "If no next hops are included, the route MUST NOT be announced (treat-as-withdraw)" (§4)
- [ ] [DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-8] [MUST NOT] "Link-Local IPv6 next hops MUST NOT be included" for an external peer that "is multiple IP hops away from the speaker (aka \"multihop EBGP\")" (§4)
- [ ] [DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-9] [MUST NOT] "If a Global IPv6 next hop is not included, the route MUST NOT be advertised to the external peer (treat-as-withdraw)" (§4)
- [ ] [DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-10] [SHOULD NOT] "When sending a message to an internal peer, if the route is not locally-originated, the BGP speaker SHOULD NOT modify the Global IPv6 next hop, if one is present, unless it has been explicitly configured to announce its own IP address as the next hop" (§4)
- [ ] [DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-11] [SHOULD] "implementations SHOULD log this suppression, or otherwise expose it through operator notification (e.g., via BMP or YANG telemetry), so that unexpected reachability gaps can be detected" (§4)
- [ ] [DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-12] [SHOULD] "If the external peer is one IP hop away, the announcing BGP speaker SHOULD include a Link-Local IPv6 next hop" (§4)
- [ ] [DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-13] [SHOULD] "If a BGP speaker receives a route with a link-local-only next hop, the route SHOULD be considered unusable for forwarding, consistent with the next-hop resolvability requirements described in [RFC4271]" (§4)
- [ ] [DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-14] [SHOULD] "By default, the BGP speaker SHOULD use the Global IPv6 address of the interface that the speaker uses in the next hop to establish the BGP connection to peer X" (§4)
- [ ] [DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-5-1] [MUST] "When this combination has not been negotiated, a sender MUST follow the rules in Section 3 of [RFC8950] and encode the Next Hop as 32 octets" (§5)
- [ ] [DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-6-1] [MUST] "If the Next Hop field is malformed, the implementation MUST handle the malformed UPDATE message using the approach of \"treat-as-withdraw\", as described in section 7.3 of [RFC7606]" (§6) {gap: validateMPReachNextHop (internal/component/bgp/message/rfc7606.go) answers a next-hop length outside attribute.ValidNextHopLens with RFC7606ActionSessionReset, which is RFC 7606 Section 7.11's approach and not the treat-as-withdraw of Section 7.3 this requirement names, and no producer inspects the CONTENT of a field whose length is admitted}
- [ ] [DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-6-2] [SHOULD] "Receivers SHOULD use the second Link-Local IPv6 address for forwarding, because the second slot is the position that carries the Link-Local address in the conforming Global-then-Link-Local layout defined by [RFC2545], and thus is the value the sender most likely intended as the Link-Local next hop" (§6)
- [ ] [DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-6-3] [SHOULD] "If the Next Hop field is properly formed, but the IPv6 Link-Local next hop is not reachable (as determined by an examination of the IPv6 neighbor table), the route SHOULD be considered unusable for forwarding purposes, in accordance with the next hop resolvability conditions described in [RFC4271]" (§6)
- [ ] [DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-7-1] [SHOULD] "Implementations SHOULD support BGP Add-Path [RFC7911] and Extended Next-Hop Encoding [RFC8950] to ensure full path utilization in IPv4-over-IPv6 underlays" (§7)
- [ ] [DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-7-2] [SHOULD] "Implementations SHOULD provide specific telemetry via the BGP Monitoring Protocol (BMP) [RFC7854] or a BGP YANG model (e.g., [I-D.ietf-idr-bgp-model]) to expose the state of link-local capability negotiation" (§7)
- [ ] [DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-7-3] [SHOULD] "implementations SHOULD treat a change in the local Link-Local address as a session reset rather than as a graceful restart event" (§7)

## Notes

The Section 3 receive rule is stated in the indicative, not with an RFC 2119 keyword:
"A BGP speaker receiving an MP_REACH_NLRI with the Next Hop field length set to
16 classifies the address as follows. If the address is in fe80::/10, the Next
Hop is a Link-Local-only Next Hop as defined in this document." It is therefore
carried by the Wire Format table above rather than by a checklist row, because a
row would have to invent a level the document does not state. `parseNextHops`
(`internal/core/bgp/attribute/mpnlri.go`) performs no such classification.

Sections 3 to 6 all sit under the Section 2 scope sentence: they bind a speaker
that has negotiated capability 77. Ze advertises capability 77 when a peer's
config asks for it, so the scope sentence's condition is reachable on a Ze
session, and every requirement above binds Ze on such a session.
