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
| Enrolment reason | Link-Local Next Hop capability for BGP (code 77): twelve MUST-level requirements, all conditioned by Section 2 on the capability being negotiated. Ze advertises the capability (extractLLNHCapabilities, internal/component/bgp/plugins/llnh/llnh.go) and implements none of the 16-octet Link-Local-only next-hop procedures behind it: linkScope.linkLocalNextHop and applyLinkLocalNextHop (internal/component/bgp/reactor/link_scope.go) emit only the RFC 2545 forms, and parseNextHops (internal/core/bgp/attribute/mpnlri.go) runs no fe80::/10 classification on a 16-octet next hop. 3-2 is met by the RFC 2545 32-octet path; 4-2 and 4-8 are met because a link-local is appended only when the peer shares a connected subnet. The rest are outstanding and named in the coverage rollup. |
| Support | drafts 30 |
| Support area | Link-local next-hop capability code 77 |
| Support status | Partial |
| Support coverage | Capability declaration only. `extractLLNHCapabilities` (`internal/component/bgp/plugins/llnh/llnh.go`) advertises the empty code 77 capability for a peer or group whose config carries a `link-local-nexthop` key that is not `disable`. The draft's own procedures are the 16-octet Link-Local-ONLY Next Hop form, and ze produces neither side of it. Send: `linkScope.linkLocalNextHop` and `applyLinkLocalNextHop` (`internal/component/bgp/reactor/link_scope.go`) emit the RFC 2545 forms only, a 16-octet GLOBAL address or the 32-octet global-then-link-local pair, and `attribute.ValidateGlobalNextHop` (`internal/core/bgp/attribute/nexthop_form.go`) refuses a link-local address in the first slot. Receive: `parseNextHops` (`internal/core/bgp/attribute/mpnlri.go`) performs no `fe80::/10` test, so a 16-octet link-local-only Next Hop is read as a Global IPv6 next hop. `parseCapability` (`internal/core/bgp/capability/capability.go`) has no case for code 77, so ze records no negotiated state and no ze path is conditioned on the capability. The requirements are listed per line in `rfc/short/draft-ietf-idr-linklocal-capability.md` and the walk is bounded by `rfc/extraction/draft-ietf-idr-linklocal-capability.json`. |
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
- Receive of the capability: `parseCapability`
  (`internal/core/bgp/capability/capability.go`) has no case for code 77, so its
  default branch returns an `Unknown`. Nothing in the reactor reads it, so Ze
  records no negotiated state for this capability and no Ze code path is
  conditioned on it.
- Next-hop send: `linkScope.linkLocalNextHop` and `applyLinkLocalNextHop`
  (`internal/component/bgp/reactor/link_scope.go`) produce the RFC 2545 forms
  only: a 16-octet GLOBAL address, or the 32-octet global-then-link-local pair
  when the speaker shares a connected subnet with both the peer and the global
  next hop. `attribute.ValidateGlobalNextHop`
  (`internal/core/bgp/attribute/nexthop_form.go`) refuses a link-local address
  in the first slot, and `linkScope.linkLocalNextHop` refuses it independently.
  No Ze path emits a 16-octet link-local-only Next Hop field.
- Next-hop receive: `parseNextHops` (`internal/core/bgp/attribute/mpnlri.go`)
  accepts lengths 16 and 32 for an IPv6 next hop and refuses every other length
  with `ErrInvalidNextHopLen`. It performs no `fe80::/10` test, so a 16-octet
  link-local-only Next Hop is read as a Global IPv6 next hop.
- Config: `session > capability > link-local-nexthop`
  (`internal/component/bgp/plugins/llnh/yang/ze-link-local-nexthop.yang`).

## Compliance Checklist

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
- [ ] [DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-6-1] [MUST] "If the Next Hop field is malformed, the implementation MUST handle the malformed UPDATE message using the approach of \"treat-as-withdraw\", as described in section 7.3 of [RFC7606]" (§6)
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
