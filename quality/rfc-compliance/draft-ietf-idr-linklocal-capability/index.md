# DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY - Link-Local Next Hop Capability for BGP

Partial. Every requirement this repository extracted from DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 80.0% | 12 of 15 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 15 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 15 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 15 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Proven by a recorded break | 43.2% | 19 of 44 tagged units, 0 escaped and 25 lapsed | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 15 | of 27 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 1 | of 15 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 6.7% | 1 of 15 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 15 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 15 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 13.3% | 2 of 15 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Audit verdicts | 15 | of 15 gated MUSTs judged | 2 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

The 8 shares marked as a part above are the whole of the 15 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| Partial proof; remaining gap | ok | green at zero, RED above it: a tested clause cannot prove the whole requirement |
| No test at all | bad | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green shares above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | bad | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Partial |
| Enrolment | Enrolled |
| Requirements | 27 |
| Gated MUST-level | 15 |
| Not applicable, so out of scope | 1 |
| Declared gaps | 2 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 44 |
| Tagged units | 44 |
| Recorded audit verdicts | 15 |
| Discrimination records | 44 |
| Summary | `rfc/short/draft-ietf-idr-linklocal-capability.md` |
| Requirement shard | `rfc/requirements/draft-ietf-idr-linklocal-capability.md` |
| RFC text | `rfc/drafts/draft-ietf-idr-linklocal-capability.txt` |

## Enrolment

Enrolled: Link-Local Next Hop capability for BGP (code 77): fifteen MUST-level requirements; those of Sections 3 to 6 are conditioned by Section 2 on the capability being negotiated. Ze advertises the capability (extractLLNHCapabilities, internal/component/bgp/plugins/llnh/llnh.go), parses it back as a typed capability (parseCapability, internal/core/bgp/capability/capability.go) and records what it negotiated to (Negotiate, internal/core/bgp/capability/negotiated.go), which is the Section 2 condition every other procedure reads. 3-1 and 5-1 are met on the send side: Peer.linkLocalOnlyNextHopPermitted (internal/component/bgp/reactor/peer.go) admits a link-local next hop only where the session may carry the 16-octet form, and buildMPReach (internal/component/bgp/message/update_build.go) writes that address alone in 16 octets, or the global-then-link-local pair in 32. 4-5 and 4-6 are met on the reflection path: egressNextHopIsLinkLocalOnly (internal/component/bgp/reactor/forward_next_hop.go) classifies the field about to be written and sameLinkLayerSegment (internal/component/bgp/reactor/link_scope.go) answers the segment half, so forwardUpdateCore either sends the rewrite applyFactsNextHop recorded or withholds the announcement. 3-2 is met by the RFC 2545 32-octet path; 4-2 and 4-8 are met because a link-local is appended only when the peer shares a connected subnet. 4-1, 4-4, 4-7 and 4-9 are met because a route whose next hop has no wire form is refused rather than encoded, and 4-3 because a route reachable through the speaker carries the speaker's own link-local toward a one-hop peer. Two MUST-level requirements stay outstanding and carry a `{gap}` on their checklist lines: 6-1, and 1-2, the Section 1 interface association (row added 2026-10-01, rfc/corrections/draft-ietf-idr-linklocal-capability.md). 4-15, the second condition of the 4-3 sentence (split 2026-10-01, rfc/corrections/draft-ietf-idr-linklocal-capability.md), is `{not-applicable}`: RFC 4271 Section 5.1.3 forbids the route it would apply to.

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

- Capability 77 declared, negotiated and acted on, for the send side and for the reflection path. Declaration: `extractLLNHCapabilities` ([`internal/component/bgp/plugins/llnh/llnh.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/llnh/llnh.go)) advertises the empty code 77 capability for a peer or group whose config carries a `link-local-nexthop` key that is not `disable`, and `refuseLinkLocalCapabilityWithoutAddress` (same file) refuses that config, at commit and at startup, when neither the peer nor its group sets `session link-local`, the only source of the own Link-Local Section 4 makes the next hop include. Gap: [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-1-2`](#draft-ietf-idr-linklocal-capability-1-2), Section 1's "implementations must ensure that they are strictly associated with a specific interface": a received Link-Local next hop is kept without its interface, and the configured `session link-local` address is not tied to the session's interface. Negotiation: `parseCapability` ([`internal/core/bgp/capability/capability.go`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability.go)) types it and `Negotiate` ([`internal/core/bgp/capability/negotiated.go`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/negotiated.go)) sets `LinkLocalNextHop` only when both OPENs carried it, which `Peer.linkLocalOnlyNextHopPermitted` ([`internal/component/bgp/reactor/peer.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer.go)) reads beside the RFC 8950 Extended Next Hop Encoding state Section 5 asks for. Send: `resolveNextHop` (same file) refuses a link-local next hop on a session that may not carry it, so the route is left out rather than encoded in a form RFC 2545 Section 3 forbids, and `buildMPReach` ([`internal/component/bgp/message/update_build.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/update_build.go)) writes the 16-octet Link-Local-only field or the 32-octet pair, for IPv4 NLRI as well as IPv6. Reflection: `egressNextHopIsLinkLocalOnly` ([`internal/component/bgp/reactor/forward_next_hop.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_next_hop.go)) and `sameLinkLayerSegment` ([`internal/component/bgp/reactor/link_scope.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/link_scope.go)) decide, per client, between the rewrite and the ineligibility Section 4 allows. Receive: `parseNextHops` ([`internal/core/bgp/attribute/mpnlri.go`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/mpnlri.go)) still performs no `fe80::/10` test, so a received 16-octet link-local-only Next Hop is read as a Global IPv6 next hop
- the Section 3 receive sentence is indicative and carries no checklist row (see Notes). One MUST-level requirement carries a `{gap}`: 6-1, because no receive producer inspects the addresses inside an admitted 32-octet Next Hop field on a capability-77 session, so a field that is not a Global followed by a Link-Local is not treated as withdrawn per RFC 7606 Section 7.3. A wrong Length of Next Hop still ends in the RFC 7606 Section 7.11 session reset, which that section requires.


**What the ledger says remains:**

-

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 12 | one part of the gated population |
| Annotated (including scoped evidence) | 3 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **15** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (12):** [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-3-1`](#draft-ietf-idr-linklocal-capability-3-1), [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-3-2`](#draft-ietf-idr-linklocal-capability-3-2), [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-1`](#draft-ietf-idr-linklocal-capability-4-1), [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-2`](#draft-ietf-idr-linklocal-capability-4-2), [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-3`](#draft-ietf-idr-linklocal-capability-4-3), [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-4`](#draft-ietf-idr-linklocal-capability-4-4), [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-5`](#draft-ietf-idr-linklocal-capability-4-5), [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-6`](#draft-ietf-idr-linklocal-capability-4-6), [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-7`](#draft-ietf-idr-linklocal-capability-4-7), [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-8`](#draft-ietf-idr-linklocal-capability-4-8), [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-9`](#draft-ietf-idr-linklocal-capability-4-9), [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-5-1`](#draft-ietf-idr-linklocal-capability-5-1)

**Annotated (including scoped evidence) (3):** [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-1-2`](#draft-ietf-idr-linklocal-capability-1-2), [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-15`](#draft-ietf-idr-linklocal-capability-4-15), [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-6-1`](#draft-ietf-idr-linklocal-capability-6-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-1-1` | BGP speakers SHOULD NOT advertise a route whose Next Hop is a Link-Local address that is in the tentative state (Section 5.4 of [RFC4862]); this applies both to a first-party Next Hop (the speaker's own Link-Local address) and to a third-party Next Hop re-advertised from another peer. (§1) | SHOULD NOT | 1 - Walked 2026-10-01 (R58(c)) | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-1-2` | Since IPv6 Link-Local addresses are not required to be globally unique, implementations must ensure that they are strictly associated with a specific interface. (§1) | MUST | 1 - Walked 2026-10-01 (R58(c)) | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze keeps no interface with a Link-Local next hop. A received next hop becomes a bare address (mpNextHopAddr, internal/component/bgp/plugins/rib/rib_bestchange.go, parses the 16 octets and attaches no zone from the session's interface, so an fe80::/10 next hop names no link), and the best path the RIB publishes carries no interface for sysrib to hand the FIB (sysrib's nextHopInterface is filled only from a producer that names one, internal/component/sysrib/sysrib.go; the BGP best-change event names none). On the send side the address Ze writes is the configured `session link-local` leaf, which nothing ties to the interface the session runs over. The strict association the sentence asks for is not implemented on either side |
| `DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-2-1` | A BGP speaker that is willing to use (send and receive) IPv6 Link- Local-only next hops SHOULD advertise the Link-Local Next Hop Capability to its peers only when: 1. It is capable of sending IPv6 Link-Local-only next hops for a route. 2. IPv6 Link-Local neighbors are associated with interfaces as part of their configuration to assist in determining the interface scope of received IPv6 Link-Local-only next hops. (§2) | SHOULD | 2 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-3-1` | If an implementation intends to send a single IPv6 Link-Local forwarding address in the Next Hop field of the MP_REACH_NLRI, it MUST set the length of the Next Hop field to 16 and include only the IPv6 Link-Local address in the Next Hop field. (§3) | MUST | 3 | **positive:** `unit/verify` [`TestLinkLocalOnlyNextHopFieldIsSixteenOctets`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_idr_linklocal_capability_send_test.go#L87). **negative:** `unit/verify` [`TestLinkLocalNextHopWithAGlobalIsNotSentAlone`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_idr_linklocal_capability_send_test.go#L118) |
| `DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-3-2` | If an implementation intends to send both a IPv6 Global and Link- Local forwarding address in the Next Hop field of the MP_REACH_NLRI, it MUST set the length of the Next Hop field to 32 and include both the IPv6 Global and Link-Local addresses in the Next Hop field. (§3) | MUST | 3 | **positive:** `unit/verify` [`TestLinkLocalBothAddressesSetTheWireLengthOctetToThirtyTwo`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_idr_linklocal_capability_announce_test.go#L157). **positive:** `unit/verify` [`TestLinkLocalBothAddressesUseThirtyTwoOctets`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_idr_linklocal_capability_test.go#L48). **negative:** `unit/verify` [`TestLinkLocalSingleAddressKeepsSixteenOctets`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_idr_linklocal_capability_test.go#L61). **negative:** `unit/verify` [`TestLinkLocalSingleAddressSetsTheWireLengthOctetToSixteen`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_idr_linklocal_capability_announce_test.go#L181) |
| `DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-1` | If, after completing these procedures, there are no IPv6 next hop addresses included in the next hop, the BGP route MUST not be advertised to its peer. (§4) | MUST | 4 | **positive:** `unit/verify` [`TestLinkLocalOnlyRouteWithdrawnFromMultihopPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_idr_linklocal_capability_withhold_withdraw_test.go#L80). **positive:** `unit/verify` [`TestLinkLocalRouteWithNoNextHopIsNotAdvertised`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_idr_linklocal_capability_advertise_test.go#L70). **negative:** `unit/verify` [`TestLinkLocalOnlyRouteWithdrawnFromMultihopPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_idr_linklocal_capability_withhold_withdraw_test.go#L84). **negative:** `unit/verify` [`TestLinkLocalRouteWithANextHopIsAdvertised`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_idr_linklocal_capability_advertise_test.go#L83) |
| `DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-2` | If the internal peer is more than one IP hop away, the BGP speaker MUST NOT include a Link-Local IPv6 next hop. (§4) | MUST NOT | 4 | **positive:** `unit/verify` [`TestLinkLocalNotIncludedForPeerMoreThanOneHopAway`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_idr_linklocal_capability_test.go#L73). **positive:** `unit/verify` [`TestLinkLocalReceivedPairStrippedForMultihopInternalPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_idr_linklocal_capability_withhold_withdraw_test.go#L155). **negative:** `unit/verify` [`TestLinkLocalIncludedForPeerOneHopAway`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_idr_linklocal_capability_test.go#L84). **negative:** `unit/verify` [`TestLinkLocalReceivedPairStrippedForMultihopInternalPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_idr_linklocal_capability_withhold_withdraw_test.go#L158) |
| `DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-3` | If the route is directly connected to the speaker, or if the interface address of the router through which the announced network is reachable for the speaker is the internal peer's address, the next hop MUST include its own Link- Local IPv6 address. (§4) | MUST | 4 | **positive:** `unit/verify` [`TestLinkLocalCapabilityAcceptedWithLinkLocalAddress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/llnh/draft_ietf_idr_linklocal_capability_llnh_refusal_test.go#L104). **positive:** `unit/verify` [`TestLinkLocalOwnAddressIncludedForDirectlyConnectedOriginatedRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_idr_linklocal_capability_announce_test.go#L85). **positive:** `unit/verify` [`TestLinkLocalOwnAddressIncludedForRouteReachableThroughTheSpeaker`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_idr_linklocal_capability_test.go#L135). **positive:** `unit/verify` [`TestLinkLocalOwnAddressIncludedUnderAutoLocalAddress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_idr_linklocal_capability_test.go#L166). **negative:** `unit/verify` [`TestLinkLocalCapabilityRefusedWithoutLinkLocalAddress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/llnh/draft_ietf_idr_linklocal_capability_llnh_refusal_test.go#L75). **negative:** `unit/verify` [`TestLinkLocalOwnAddressNotIncludedForRouteThroughAnotherRouter`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_idr_linklocal_capability_announce_test.go#L107). **negative:** `unit/verify` [`TestLinkLocalOwnAddressNotInsertedWhenAnotherRouterIsTheNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_idr_linklocal_capability_test.go#L193) |
| `DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-4` | If, after evaluating the above procedures, there are no IPv6 next hops included with the route, the route MUST NOT be announced to the remote BGP speaker. (§4) | MUST NOT | 4 | **positive:** `unit/verify` [`TestLinkLocalOnlyRouteWithdrawnFromMultihopPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_idr_linklocal_capability_withhold_withdraw_test.go#L86). **positive:** `unit/verify` [`TestLinkLocalRouteWithNoNextHopIsNotAnnouncedToAnInternalPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_idr_linklocal_capability_advertise_test.go#L96). **negative:** `unit/verify` [`TestLinkLocalRouteWithANextHopIsAnnouncedToAnInternalPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_idr_linklocal_capability_advertise_test.go#L105) |
| `DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-5` | A Route Reflector (RR) reflecting a route with a link-local-only next hop MUST NOT advertise that route to a client unless the client shares the same link-layer segment as the original advertiser. (§4) | MUST NOT | 4 | **positive:** `unit/verify` [`TestReflectedLinkLocalOnlyRouteIsWithheldFromAClientOffTheSegment`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_idr_linklocal_capability_reflect_test.go#L220). **negative:** `unit/verify` [`TestReflectedLinkLocalOnlyRouteIsWithheldFromAClientOffTheSegment`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_idr_linklocal_capability_reflect_test.go#L229) |
| `DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-6` | For all other clients, the RR MUST either rewrite the next hop to its own address (next-hop-self) or consider the route ineligible for advertisement to that specific peer. (§4) | MUST | 4 | **positive:** `unit/verify` [`TestReflectedLinkLocalOnlyRouteIsRewrittenOrIneligibleForOtherClients`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_idr_linklocal_capability_reflect_test.go#L313). **negative:** `unit/verify` [`TestReflectedGlobalNextHopRouteIsNeitherRewrittenNorWithheld`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_idr_linklocal_capability_reflect_test.go#L343) |
| `DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-7` | If no next hops are included, the route MUST NOT be announced (treat-as- withdraw). (§4) | MUST NOT | 4 | **positive:** `unit/verify` [`TestLinkLocalRouteWithNoNextHopIsNotAnnouncedToAnExternalPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_idr_linklocal_capability_advertise_test.go#L117). **positive:** `unit/verify` [`TestNextHopSelfWithheldRouteServerWithdraws`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_idr_linklocal_capability_withhold_withdraw_test.go#L193). **negative:** `unit/verify` [`TestLinkLocalRouteWithANextHopIsAnnouncedToAnExternalPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_idr_linklocal_capability_advertise_test.go#L126) |
| `DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-8` | When sending a message to an external peer X, and the peer is multiple IP hops away from the speaker (aka "multihop EBGP"): * Link-Local IPv6 next hops MUST NOT be included. (§4) | MUST NOT | 4 | **positive:** `unit/verify` [`TestLinkLocalNotIncludedForMultihopExternalPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_idr_linklocal_capability_test.go#L95). **positive:** `unit/verify` [`TestLinkLocalReceivedPairStrippedForMultihopExternalPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_idr_linklocal_capability_multihop_test.go#L98). **positive:** `unit/verify` [`TestLinkLocalRouteServerStripsReceivedPairForMultihopClient`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_idr_linklocal_capability_multihop_test.go#L210). **negative:** `unit/verify` [`TestLinkLocalIncludedForDirectlyAttachedExternalPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_idr_linklocal_capability_test.go#L108). **negative:** `unit/verify` [`TestLinkLocalReceivedPairKeptForDirectlyAttachedExternalPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_idr_linklocal_capability_multihop_test.go#L121). **negative:** `unit/verify` [`TestLinkLocalRouteServerKeepsReceivedPairForAttachedClient`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_idr_linklocal_capability_multihop_test.go#L232) |
| `DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-9` | If a Global IPv6 next hop is not included, the route MUST NOT be advertised to the external peer (treat-as-withdraw). (§4) | MUST NOT | 4 | **positive:** `unit/verify` [`TestLinkLocalOnlyRouteServerWithdrawnFromMultihopClient`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_idr_linklocal_capability_withhold_withdraw_test.go#L124). **positive:** `unit/verify` [`TestLinkLocalOnlyRouteWithdrawnFromMultihopPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_idr_linklocal_capability_withhold_withdraw_test.go#L88). **positive:** `unit/verify` [`TestLinkLocalRouteWithNoGlobalNextHopIsNotAdvertisedToAMultihopExternalPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_idr_linklocal_capability_advertise_test.go#L137). **negative:** `unit/verify` [`TestLinkLocalOnlyRouteServerWithdrawnFromMultihopClient`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_idr_linklocal_capability_withhold_withdraw_test.go#L127). **negative:** `unit/verify` [`TestLinkLocalRouteWithAGlobalNextHopIsAdvertisedToAMultihopExternalPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_idr_linklocal_capability_advertise_test.go#L151) |
| `DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-10` | When sending a message to an internal peer, if the route is not locally-originated, the BGP speaker SHOULD NOT modify the Global IPv6 next hop, if one is present, unless it has been explicitly configured to announce its own IP address as the next hop. (§4) | SHOULD NOT | 4 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-11` | To aid troubleshooting in such deployments, implementations SHOULD log this suppression, or otherwise expose it through operator notification (e.g., via BMP or YANG telemetry), so that unexpected reachability gaps can be detected. (§4) | SHOULD | 4 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-12` | If the external peer is one IP hop away, the announcing BGP speaker SHOULD include a Link-Local IPv6 next hop. (§4) | SHOULD | 4 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-13` | If a BGP speaker receives a route with a link-local-only next hop, the route SHOULD be considered unusable for forwarding, consistent with the next-hop resolvability requirements described in [RFC4271]. (§4) | SHOULD | 4 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-14` | By default, the BGP speaker SHOULD use the Global IPv6 address of the interface that the speaker uses in the next hop to establish the BGP connection to peer X. (§4) | SHOULD | 4 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-15` | if the interface address of the router through which the announced network is reachable for the speaker is the internal peer's address, the next hop MUST include its own Link- Local IPv6 address. (§4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the antecedent is a route Ze is forbidden to send. RFC 4271 Section 5.1.3: "A route originated by a BGP speaker SHALL NOT be advertised to a peer using an address of that peer as NEXT_HOP." Ze withholds a route whose next hop is the receiving peer's own address on every rail (originatedNextHopIsPeerOwn at the write boundary, egressNextHopIsPeerOwn on the relayed rail, internal/component/bgp/reactor/forward_next_hop.go; owner decision 2026-08-15), so no route carrying this condition reaches the wire. Shown by the untagged TestLinkLocalSecondConditionNeverReachesTheWire (internal/component/bgp/reactor/draft_ietf_idr_linklocal_capability_announce_test.go): a static route with explicit next hop = the internal peer's address 2001:db8:1::2 writes nothing to that peer. The directly-connected condition of the same sentence stays under DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-3 |
| `DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-5-1` | When this combination has not been negotiated, a sender MUST follow the rules in Section 3 of [RFC8950] and encode the Next Hop as 32 octets. (§5) | MUST | 5 | **positive:** `unit/verify` [`TestIPv4NLRINextHopIsThirtyTwoOctetsWithoutTheCombination`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_idr_linklocal_capability_send_test.go#L141). **negative:** `unit/verify` [`TestIPv4NLRILinkLocalOnlyNextHopNeedsTheCombination`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_idr_linklocal_capability_send_test.go#L173) |
| `DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-6-1` | If the Next Hop field is malformed, the implementation MUST handle the malformed UPDATE message using the approach of "treat-as- withdraw", as described in section 7.3 of [RFC7606]. (§6) | MUST | 6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** on a session where capability 77 is negotiated, no receive producer inspects the addresses inside an admitted 32-octet Next Hop field, so a field that is not a Global followed by a Link-Local is never treated as withdrawn. A wrong Length of Next Hop is not this gap: validateMPReachNextHop (internal/component/bgp/message/rfc7606.go) answers it with RFC7606ActionSessionReset, which RFC 7606 Section 7.11 requires because the NLRI cannot be located |
| `DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-6-2` | Receivers SHOULD use the second Link-Local IPv6 address for forwarding, because the second slot is the position that carries the Link-Local address in the conforming Global-then-Link-Local layout defined by [RFC2545], and thus is the value the sender most likely intended as the Link-Local next hop. (§6) | SHOULD | 6 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-6-3` | If the Next Hop field is properly formed, but the IPv6 Link-Local next hop is not reachable (as determined by an examination of the IPv6 neighbor table), the route SHOULD be considered unusable for forwarding purposes, in accordance with the next hop resolvability conditions described in [RFC4271]. (§6) | SHOULD | 6 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-7-1` | Implementations SHOULD support BGP Add- Path [RFC7911] and Extended Next-Hop Encoding [RFC8950] to ensure full path utilization in IPv4-over-IPv6 underlays. (§7) | SHOULD | 7 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-7-2` | Implementations SHOULD provide specific telemetry via the BGP Monitoring Protocol (BMP) [RFC7854] or a BGP YANG model (e.g., [I-D.ietf-idr-bgp-model]) to expose the state of link-local capability negotiation. (§7) | SHOULD | 7 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-7-3` | implementations SHOULD treat a change in the local Link-Local address as a session reset rather than as a graceful restart event. (§7) | SHOULD | 7 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-1-2`](#draft-ietf-idr-linklocal-capability-1-2) Since IPv6 Link-Local addresses are not required to be globally unique, implementations must ensure that they are strictly associated with a specific interface. (§1) | {gap}, no test | Ze keeps no interface with a Link-Local next hop. A received next hop becomes a bare address (mpNextHopAddr, internal/component/bgp/plugins/rib/rib_bestchange.go, parses the 16 octets and attaches no zone from the session's interface, so an fe80::/10 next hop names no link), and the best path the RIB publishes carries no interface for sysrib to hand the FIB (sysrib's nextHopInterface is filled only from a producer that names one, internal/component/sysrib/sysrib.go; the BGP best-change event names none). On the send side the address Ze writes is the configured `session link-local` leaf, which nothing ties to the interface the session runs over. The strict association the sentence asks for is not implemented on either side |
| [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-15`](#draft-ietf-idr-linklocal-capability-4-15) if the interface address of the router through which the announced network is reachable for the speaker is the internal peer's address, the next hop MUST include its own Link- Local IPv6 address. (§4) | no test | no test carries this requirement id; annotated {not-applicable}: the antecedent is a route Ze is forbidden to send. RFC 4271 Section 5.1.3: "A route originated by a BGP speaker SHALL NOT be advertised to a peer using an address of that peer as NEXT_HOP." Ze withholds a route whose next hop is the receiving peer's own address on every rail (originatedNextHopIsPeerOwn at the write boundary, egressNextHopIsPeerOwn on the relayed rail, internal/component/bgp/reactor/forward_next_hop.go; owner decision 2026-08-15), so no route carrying this condition reaches the wire. Shown by the untagged TestLinkLocalSecondConditionNeverReachesTheWire (internal/component/bgp/reactor/draft_ietf_idr_linklocal_capability_announce_test.go): a static route with explicit next hop = the internal peer's address 2001:db8:1::2 writes nothing to that peer. The directly-connected condition of the same sentence stays under DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-3 |
| [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-6-1`](#draft-ietf-idr-linklocal-capability-6-1) If the Next Hop field is malformed, the implementation MUST handle the malformed UPDATE message using the approach of "treat-as- withdraw", as described in section 7.3 of [RFC7606]. (§6) | {gap}, no test | on a session where capability 77 is negotiated, no receive producer inspects the addresses inside an admitted 32-octet Next Hop field, so a field that is not a Global followed by a Link-Local is never treated as withdrawn. A wrong Length of Next Hop is not this gap: validateMPReachNextHop (internal/component/bgp/message/rfc7606.go) answers it with RFC7606ActionSessionReset, which RFC 7606 Section 7.11 requires because the NLRI cannot be located |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-1-2`](#draft-ietf-idr-linklocal-capability-1-2)

Since IPv6 Link-Local addresses are not required to be globally unique, implementations must ensure that they are strictly associated with a specific interface. (§1)

Audit verdict: unimplemented (no code path enforces the requirement), fresh. Judged 2026-10-01 (BGP c20 judge, R58(c)). Row text is a verbatim span of draft -06 Section 1 (lowercase must, ruled to bind the implementation). Gap confirmed at the producers: mpNextHopAddr (rib_bestchange.go) turns the received Next Hop octets into a bare netip.Addr with no zone and no interface, and nothing in the BGP best-change path sets one (no WithZone on that path); on the send side applyLinkLocal (config_nexthop_form.go) parses session link-local as a plain address with no tie to the session's interface. The {gap} annotation states both halves.

No test carries DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-1-2, so no unit is bound to it.

### [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-3-1`](#draft-ietf-idr-linklocal-capability-3-1)

If an implementation intends to send a single IPv6 Link-Local forwarding address in the Next Hop field of the MP_REACH_NLRI, it MUST set the length of the Next Hop field to 16 and include only the IPv6 Link-Local address in the Next Hop field. (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a sender meaning to send one Link-Local address writes a field other than 16 octets, or adds another address. TestLinkLocalOnlyNextHopFieldIsSixteenOctets builds the UPDATE through resolveNextHop and BuildUnicast and asserts assert.Len(field, 16) and that the field equals fe80::1 alone; either defect turns it red. The negative TestLinkLocalNextHopWithAGlobalIsNotSentAlone shows the 16-octet form is keyed on one address (32 octets when a Global is also sent).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLinkLocalNextHopWithAGlobalIsNotSentAlone`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_idr_linklocal_capability_send_test.go#L118) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| positive | [`TestLinkLocalOnlyNextHopFieldIsSixteenOctets`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_idr_linklocal_capability_send_test.go#L87) | unit/verify | revert, verified |

### [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-3-2`](#draft-ietf-idr-linklocal-capability-3-2)

If an implementation intends to send both a IPv6 Global and Link- Local forwarding address in the Next Hop field of the MP_REACH_NLRI, it MUST set the length of the Next Hop field to 32 and include both the IPv6 Global and Link-Local addresses in the Next Hop field. (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-01 (BGP c21 judge). The c19 finding is closed: TestLinkLocalBothAddressesSetTheWireLengthOctetToThirtyTwo reads the MP_REACH_NLRI Length of Next Hop octet off the bytes sendStaticRoutes wrote to the one-hop internal peer's connection and asserts 0x20 followed by exactly 2001:db8:1::1 then fe80::1 (both clauses: length 32, both addresses); TestLinkLocalSingleAddressSetsTheWireLengthOctetToSixteen asserts 0x10 and the lone global for a route through another router, so the 32-octet length is tied to sending both. mpReachNextHopField slices exactly the octets the length announces and fails when none is written. Records observed red on mpnlri.go::nextHopLen. The older facts-level units stay tagged; their assert.Len over a [32]byte array is vacuous and carries no weight in this verdict.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLinkLocalSingleAddressSetsTheWireLengthOctetToSixteen`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_idr_linklocal_capability_announce_test.go#L181) | unit/verify | revert, verified |
| negative | [`TestLinkLocalSingleAddressKeepsSixteenOctets`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_idr_linklocal_capability_test.go#L61) | unit/verify | revert, verified |
| positive | [`TestLinkLocalBothAddressesSetTheWireLengthOctetToThirtyTwo`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_idr_linklocal_capability_announce_test.go#L157) | unit/verify | revert, verified |
| positive | [`TestLinkLocalBothAddressesUseThirtyTwoOctets`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_idr_linklocal_capability_test.go#L48) | unit/verify | revert, verified |

### [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-1`](#draft-ietf-idr-linklocal-capability-4-1)

If, after completing these procedures, there are no IPv6 next hop addresses included in the next hop, the BGP route MUST not be advertised to its peer. (§4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Rejudged against draft -06 Sections 2 and 4. Section 4 says: 'If, after completing these procedures, there are no IPv6 next hop addresses included in the next hop, the BGP route MUST not be advertised to its peer. Instead, treat-as-withdraw (Section 2 of [RFC7606]) is used.' TestLinkLocalOnlyRouteWithdrawnFromMultihopPeer exercises forwardUpdateCore with received fe80::1, NextHopUnchanged, internal and external destinations outside the connected subnet; it requires dispatch to each distant destination, no announced next hop, and exact MP_UNREACH for 2001:db8:7::/64. The same fan-out requires an attached destination to receive exactly fe80::1. llnhExternalPeer now sets negotiated.LinkLocalNextHop=true, making that control conforming and preventing the capability refusal from masking the off-link gate. egressNextHopWithheld calls egressNextHopLinkLocalOnlyOffLink, and forwardUpdateCore turns the refusal into SetWithdraw. The two tagged buildRIBRouteUpdate tests additionally distinguish absent next hop (nil UPDATE for internal/external) from a Global address (exact 16-octet next hop), but alone are not the treat-as-withdraw proof. This is source-level judgment of semantic assertions, not a new execution or interop-pass claim.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLinkLocalRouteWithANextHopIsAdvertised`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_idr_linklocal_capability_advertise_test.go#L83) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| negative | [`TestLinkLocalOnlyRouteWithdrawnFromMultihopPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_idr_linklocal_capability_withhold_withdraw_test.go#L84) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| positive | [`TestLinkLocalRouteWithNoNextHopIsNotAdvertised`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_idr_linklocal_capability_advertise_test.go#L70) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| positive | [`TestLinkLocalOnlyRouteWithdrawnFromMultihopPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_idr_linklocal_capability_withhold_withdraw_test.go#L80) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |

### [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-2`](#draft-ietf-idr-linklocal-capability-4-2)

If the internal peer is more than one IP hop away, the BGP speaker MUST NOT include a Link-Local IPv6 next hop. (§4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-04 after f93a30fbfc. Forbidden: including a Link-Local next hop toward an internal peer more than one hop away. The third-party case the earlier weak verdict named is now driven: TestLinkLocalReceivedPairStrippedForMultihopInternalPeer relays the received 32-octet 2001:db8:1::1 + fe80::9 pair through forwardUpdateCore under next hop unchanged and under auto to an internal (AS 65000) peer off every connected subnet and asserts the exact 16-octet Global alone; the control in the same fan-out, an internal peer on the advertiser's subnet, keeps the exact 32-octet pair, which RFC 2545 Section 3 permits without the capability. Restoring the pass-through turns the positive red. The facts-level units still cover the next-hop-self half.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLinkLocalIncludedForPeerOneHopAway`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_idr_linklocal_capability_test.go#L84) | unit/verify | revert, verified |
| negative | [`TestLinkLocalReceivedPairStrippedForMultihopInternalPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_idr_linklocal_capability_withhold_withdraw_test.go#L158) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| positive | [`TestLinkLocalNotIncludedForPeerMoreThanOneHopAway`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_idr_linklocal_capability_test.go#L73) | unit/verify | revert, verified |
| positive | [`TestLinkLocalReceivedPairStrippedForMultihopInternalPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_idr_linklocal_capability_withhold_withdraw_test.go#L155) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |

### [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-3`](#draft-ietf-idr-linklocal-capability-4-3)

If the route is directly connected to the speaker, or if the interface address of the router through which the announced network is reachable for the speaker is the internal peer's address, the next hop MUST include its own Link- Local IPv6 address. (§4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-01 (BGP c20 judge, R58(a)). The c19 hole is closed at its producer: refuseLinkLocalCapabilityWithoutAddress (llnh.go), run by both OnConfigVerify (commit) and OnConfigure (startup, FatalOnConfigError in register.go), refuses a peer, a group member or a dynamic-group template whose capability 77 is enabled (llnhEnabledFor, the same answer extractLLNHCapabilities advertises from) with no session link-local on the peer or its group; ResolveBGPTree merges the group's session leaves into the peer, so the group case matches what the reactor's applyLinkLocal reads. So every configuration that can negotiate 77 carries the address linkScope.linkLocalNextHop appends. New units, both through the real LoadConfig + ToPluginMap lowering: negative TestLinkLocalCapabilityRefusedWithoutLinkLocalAddress (standalone and group-inherited capability refused, error names peer1 and session link-local, the section entry refuses too); positive TestLinkLocalCapabilityAcceptedWithLinkLocalAddress (leaf on peer, group, member: accepted and the capability still declared). Both recorded red under the producer break. The wire half stays proven both polarities on the announce rail (directly connected originated route: [global, fe80::1]; route through another router: 16-octet global) and on the forward facts (next-hop self, local ip auto, unchanged), each with an observed-red record. Functional: test/plugin/llnh-refuses-without-link-local.ci (ze boot exits 1 with the refusal) passes. Residual note: the wire units do not negotiate capability 77 itself; linkLocalNextHop does not read the capability, so the same producer answers with and without it.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLinkLocalCapabilityRefusedWithoutLinkLocalAddress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/llnh/draft_ietf_idr_linklocal_capability_llnh_refusal_test.go#L75) | unit/verify | revert, verified |
| negative | [`TestLinkLocalOwnAddressNotIncludedForRouteThroughAnotherRouter`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_idr_linklocal_capability_announce_test.go#L107) | unit/verify | revert, verified |
| negative | [`TestLinkLocalOwnAddressNotInsertedWhenAnotherRouterIsTheNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_idr_linklocal_capability_test.go#L193) | unit/verify | revert, verified |
| positive | [`TestLinkLocalCapabilityAcceptedWithLinkLocalAddress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/llnh/draft_ietf_idr_linklocal_capability_llnh_refusal_test.go#L104) | unit/verify | revert, verified |
| positive | [`TestLinkLocalOwnAddressIncludedForDirectlyConnectedOriginatedRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_idr_linklocal_capability_announce_test.go#L85) | unit/verify | revert, verified |
| positive | [`TestLinkLocalOwnAddressIncludedForRouteReachableThroughTheSpeaker`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_idr_linklocal_capability_test.go#L135) | unit/verify | revert, verified |
| positive | [`TestLinkLocalOwnAddressIncludedUnderAutoLocalAddress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_idr_linklocal_capability_test.go#L166) | unit/verify | revert, verified |

### [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-4`](#draft-ietf-idr-linklocal-capability-4-4)

If, after evaluating the above procedures, there are no IPv6 next hops included with the route, the route MUST NOT be announced to the remote BGP speaker. (§4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-04 after f93a30fbfc. Forbidden: announcing to an internal peer a route the internal-peer procedures leave with no IPv6 next hop, rather than treat-as-withdraw. TestLinkLocalOnlyRouteWithdrawnFromMultihopPeer (internal=true branch) relays a Link-Local-only fe80::1 to an internal peer more than one hop away, where the Link-Local MUST NOT be included and nothing remains, and asserts no MP_REACH next hop and the exact MP_UNREACH_NLRI for the prefix, so 'after evaluating the above procedures' is an input and the withdrawal is asserted. Control: TestLinkLocalRouteWithANextHopIsAnnouncedToAnInternalPeer announces the same route with a Global next hop and LOCAL_PREF (RIB rail, cross-rail but conformant). The RIB-rail positive alone is still only a zero-address builder check.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLinkLocalRouteWithANextHopIsAnnouncedToAnInternalPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_idr_linklocal_capability_advertise_test.go#L105) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| positive | [`TestLinkLocalRouteWithNoNextHopIsNotAnnouncedToAnInternalPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_idr_linklocal_capability_advertise_test.go#L96) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| positive | [`TestLinkLocalOnlyRouteWithdrawnFromMultihopPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_idr_linklocal_capability_withhold_withdraw_test.go#L86) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |

### [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-5`](#draft-ietf-idr-linklocal-capability-4-5)

A Route Reflector (RR) reflecting a route with a link-local-only next hop MUST NOT advertise that route to a client unless the client shares the same link-layer segment as the original advertiser. (§4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Rejudged against draft -06 Section 4: 'A Route Reflector (RR) reflecting a route with a link-local-only next hop MUST NOT advertise that route to a client unless the client shares the same link-layer segment as the original advertiser.' llnhClient now sets negotiated.LinkLocalNextHop=true. More importantly, the tagged TestReflectedLinkLocalOnlyRouteIsWithheldFromAClientOffTheSegment now includes directly-attached-other-segment: both clients are on connected /64s, only one shares the advertiser's /64, and both use NextHopUnchanged. On forwardUpdateCore's reflected path the off-segment client must be dispatched the exact MP_UNREACH for 2001:db8:7::/64, with no MP_REACH attribute and no next hop; the same-segment client must receive MP_REACH with exactly fe80::1 and no MP_UNREACH. These are distinct conforming/refused outcomes despite sharing one tagged function. Reading egressNextHopWithheld, sameLinkLayerSegment and destOnLink establishes isolation: removing only the reflection segment refusal leaves both clients on-link and capability-authorized, so the off-segment announcement violates the explicit no-MP_REACH and exact-withdrawal assertions. The older outer off-segment case alone was confounded by the independent multihop refusal; the new attached-other-segment subtest closes that proof gap. No remaining 4-5 source/test fix is established by this focused review. This is semantic source evidence, not an observed mutation run or an interop-pass claim.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestReflectedLinkLocalOnlyRouteIsWithheldFromAClientOffTheSegment`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_idr_linklocal_capability_reflect_test.go#L229) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| positive | [`TestReflectedLinkLocalOnlyRouteIsWithheldFromAClientOffTheSegment`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_idr_linklocal_capability_reflect_test.go#L220) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |

### [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-6`](#draft-ietf-idr-linklocal-capability-4-6)

For all other clients, the RR MUST either rewrite the next hop to its own address (next-hop-self) or consider the route ineligible for advertisement to that specific peer. (§4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-04 after f93a30fbfc. Forbidden: for an off-segment client, sending the Link-Local-only route neither rewritten nor ineligible. TestReflectedLinkLocalOnlyRouteIsRewrittenOrIneligibleForOtherClients asserts the next-hop-self client receives exactly 2001:db8:1::254 and not a Link-Local-only field, and the client without rewrite is written no MP_REACH next hop (now the withdrawal under the owner's decision; assert.Empty does not distinguish withdrawal from nothing, which the sentence does not require). Negative TestReflectedGlobalNextHopRouteIsNeitherRewrittenNorWithheld: a Global next hop reaches the same off-segment client unchanged. No assertion delivers a Link-Local-only next hop, so the capability-77 fixture gap does not touch this pair.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestReflectedGlobalNextHopRouteIsNeitherRewrittenNorWithheld`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_idr_linklocal_capability_reflect_test.go#L343) | unit/verify | revert, verified |
| positive | [`TestReflectedLinkLocalOnlyRouteIsRewrittenOrIneligibleForOtherClients`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_idr_linklocal_capability_reflect_test.go#L313) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |

### [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-7`](#draft-ietf-idr-linklocal-capability-4-7)

If no next hops are included, the route MUST NOT be announced (treat-as- withdraw). (§4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-04 after f93a30fbfc. Forbidden: announcing to a one-hop external peer a route with no next hop included, rather than treat-as-withdraw. TestNextHopSelfWithheldRouteServerWithdraws requires the fixture reached nhSelfWithheld (next-hop self configured, no address of this speaker, so third-party is disabled and nothing remains) and asserts on the route-server rail no MP_REACH next hop and the exact MP_UNREACH_NLRI for the prefix; the client is on-link, so no other gate explains the refusal. Both rails share egressNextHopWithheld and each calls mods.SetWithdraw on any refusal, and the general-rail withdrawal is proven by the 4-1 unit. Control: TestLinkLocalRouteWithANextHopIsAnnouncedToAnExternalPeer (RIB rail, cross-rail, conformant). The RIB-rail positive alone is a zero-address builder check.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLinkLocalRouteWithANextHopIsAnnouncedToAnExternalPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_idr_linklocal_capability_advertise_test.go#L126) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| positive | [`TestLinkLocalRouteWithNoNextHopIsNotAnnouncedToAnExternalPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_idr_linklocal_capability_advertise_test.go#L117) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| positive | [`TestNextHopSelfWithheldRouteServerWithdraws`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_idr_linklocal_capability_withhold_withdraw_test.go#L193) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |

### [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-8`](#draft-ietf-idr-linklocal-capability-4-8)

When sending a message to an external peer X, and the peer is multiple IP hops away from the speaker (aka "multihop EBGP"): * Link-Local IPv6 next hops MUST NOT be included. (§4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-02 (BGP c22 judge). The route-server rail the c21 judge named is now driven: TestLinkLocalRouteServerStripsReceivedPairForMultihopClient relays the received 32-octet 2001:db8:1::1+fe80::9 through reactorForwardRS to an off-link external client under next hop unchanged and auto and asserts the 16-octet Global alone; TestLinkLocalRouteServerKeepsReceivedPairForAttachedClient keeps the pair for a client on the connected /64. The author's overlay disabling only the forward_rs.go call (strip && false, job-c22-nocut log, read by the judge) turns the RS positive red, so the RS call is pinned on its own. General rail unchanged from c21 (forwardUpdateCore units, both polarities). Revert records on egressNextHopGlobalHalf observed red for all four units.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLinkLocalReceivedPairKeptForDirectlyAttachedExternalPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_idr_linklocal_capability_multihop_test.go#L121) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| negative | [`TestLinkLocalRouteServerKeepsReceivedPairForAttachedClient`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_idr_linklocal_capability_multihop_test.go#L232) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| negative | [`TestLinkLocalIncludedForDirectlyAttachedExternalPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_idr_linklocal_capability_test.go#L108) | unit/verify | revert, verified |
| positive | [`TestLinkLocalReceivedPairStrippedForMultihopExternalPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_idr_linklocal_capability_multihop_test.go#L98) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| positive | [`TestLinkLocalRouteServerStripsReceivedPairForMultihopClient`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_idr_linklocal_capability_multihop_test.go#L210) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| positive | [`TestLinkLocalNotIncludedForMultihopExternalPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_idr_linklocal_capability_test.go#L95) | unit/verify | revert, verified |

### [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-9`](#draft-ietf-idr-linklocal-capability-4-9)

If a Global IPv6 next hop is not included, the route MUST NOT be advertised to the external peer (treat-as-withdraw). (§4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Rejudged against draft -06 Section 4's multihop EBGP procedure: 'If a Global IPv6 next hop is not included, the route MUST NOT be advertised to the external peer (treat-as-withdraw).' TestLinkLocalOnlyRouteWithdrawnFromMultihopPeer (external branch, general rail) and TestLinkLocalOnlyRouteServerWithdrawnFromMultihopClient (route-server rail) relay received fe80::1 unchanged to an external destination outside every connected subnet and require dispatch, no announced next hop, and exact MP_UNREACH for 2001:db8:7::/64. Their attached controls receive exactly fe80::1; llnhExternalPeer now supplies negotiated.LinkLocalNextHop=true, so those controls are conforming and capability refusal cannot explain the distant-peer result. Both production rails invoke egressNextHopWithheld and materialize a withdrawal when egressNextHopLinkLocalOnlyOffLink refuses the no-Global route. The two additional tagged advertise tests distinguish no Global (nil RIB-built UPDATE) from an exact 16-octet Global next hop, alongside multihop next-hop-self facts; they supplement rather than replace the forwarding withdrawal proof. This is source-level judgment of semantic assertions, not a new execution or interop-pass claim.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLinkLocalRouteWithAGlobalNextHopIsAdvertisedToAMultihopExternalPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_idr_linklocal_capability_advertise_test.go#L151) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| negative | [`TestLinkLocalOnlyRouteServerWithdrawnFromMultihopClient`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_idr_linklocal_capability_withhold_withdraw_test.go#L127) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| positive | [`TestLinkLocalRouteWithNoGlobalNextHopIsNotAdvertisedToAMultihopExternalPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_idr_linklocal_capability_advertise_test.go#L137) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| positive | [`TestLinkLocalOnlyRouteServerWithdrawnFromMultihopClient`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_idr_linklocal_capability_withhold_withdraw_test.go#L124) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| positive | [`TestLinkLocalOnlyRouteWithdrawnFromMultihopPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_idr_linklocal_capability_withhold_withdraw_test.go#L88) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |

### [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-15`](#draft-ietf-idr-linklocal-capability-4-15)

if the interface address of the router through which the announced network is reachable for the speaker is the internal peer's address, the next hop MUST include its own Link- Local IPv6 address. (§4)

Audit verdict: not-applicable (the requirement has no reachable code path in Ze), fresh. Judged 2026-10-01 (BGP c19 judge). Split from 4-3 under R3/R57(a); row text is a verbatim span of the Section 4 sentence's second clause. Read both producers: egressNextHopIsPeerOwn compares the (modified) next hop to f.addr, originatedNextHopIsPeerOwn reads the built body. The untagged TestLinkLocalSecondConditionNeverReachesTheWire (static route, explicit next hop = the internal peer's 2001:db8:1::2) asserts nothing is written, while the sibling ::99 route on the same fixture is written, so the empty wire is the withholding, not a dead fixture. Untagged because the gate refuses a tag beside {not-applicable}; that reading is accepted.

No test carries DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-15, so no unit is bound to it.

### [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-5-1`](#draft-ietf-idr-linklocal-capability-5-1)

When this combination has not been negotiated, a sender MUST follow the rules in Section 3 of [RFC8950] and encode the Next Hop as 32 octets. (§5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: sending the 16-octet Link-Local-only field for IPv4 NLRI when capability 77 plus RFC 8950 were not both negotiated. TestIPv4NLRINextHopIsThirtyTwoOctetsWithoutTheCombination asserts require.ErrorIs(err, ErrNextHopLinkLocalOnly) from resolveNextHop and assert.Len(field, 32) with Global then Link-Local; admitting the 16-octet form turns it red. The negative TestIPv4NLRILinkLocalOnlyNextHopNeedsTheCombination shows the 16-octet form is sent once both are negotiated.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestIPv4NLRILinkLocalOnlyNextHopNeedsTheCombination`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_idr_linklocal_capability_send_test.go#L173) | unit/verify | revert, verified |
| positive | [`TestIPv4NLRINextHopIsThirtyTwoOctetsWithoutTheCombination`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_idr_linklocal_capability_send_test.go#L141) | unit/verify | revert, verified |

### [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-6-1`](#draft-ietf-idr-linklocal-capability-6-1)

If the Next Hop field is malformed, the implementation MUST handle the malformed UPDATE message using the approach of "treat-as- withdraw", as described in section 7.3 of [RFC7606]. (§6)

Audit verdict: unimplemented (no code path enforces the requirement), fresh. Re-judged 2026-10-02 (BGP c22 judge). Gap unchanged; the {gap} annotation is now aimed at the content half as the c21 judge required: on a capability-77 session no receive producer inspects the addresses of an admitted 32-octet Next Hop field, so a field that is not Global followed by Link-Local is never treated as withdrawn. A wrong Length stays an RFC 7606 Section 7.11 session reset in validateMPReachNextHop, which is correct and named here as the producer that admits the field without inspecting it. Dated correction in rfc/corrections.

No test carries DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-6-1, so no unit is bound to it.

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude, BGP c20 independent judge (spec-rfc-verdict-test-fix-pass) |
| Signed off | 2026-10-01 |
| Register | prose |
| Source | rfc/drafts/draft-ietf-idr-linklocal-capability.txt |
| Source fingerprint | f6a372b9b6ab4db5 |
| Record | rfc/extraction/draft-ietf-idr-linklocal-capability.json |
| Mapped sentences | 14 |
| Declined as scope | 3 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 1 | walked | Walked 2026-10-01 (R58(c)): the -06 text yields one derived site here, the Copyright Notice sentence front:1, excluded below as IETF Trust boilerplate. The section holds the Title block, Abstract, Status of This Memo, Copyright Notice and Table of Contents. The Abstract says the document updates RFC 2545 to clarify the next-hop encoding when only an IPv6 Link-Local address is available, and defines a capability signalling support for it. It directs no speaker. Its one site is the Copyright Notice, excluded below. |
| `1` | Walked 2026-10-01 (R58(c)) | 3 | walked | Walked 2026-10-01 (R58(c)). Three derived sites: 1:2 binds Ze and maps to DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-1-2 (a recorded gap); 1:1 and 1:3 state what a deployment or an address does not need and direct no implementation, so both are excluded. DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-1-1 quotes an uppercase SHOULD NOT sentence the lowercase site scan does not split out, so it stays unsourced. |
| `2` | not stated | 0 | walked | not stated |
| `3` | not stated | 2 | walked | not stated |
| `4` | not stated | 9 | walked | DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-15 (2026-10-01, rfc/corrections/draft-ietf-idr-linklocal-capability.md) is split from DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-3 and quotes the second condition of site 4:3's sentence: it is annotated not-applicable, so it is listed here as unsourced rather than given a second site. |
| `5` | not stated | 1 | walked | not stated |
| `6` | not stated | 1 | walked | not stated |
| `7` | not stated | 0 | walked | not stated |
| `8` | Acknowledgements | 0 | skipped (acknowledgements) | Acknowledgements. Thanks, plus the note that the work builds on draft-kumar-idr-link-local-nexthop and draft-kato-bgp-ipv6-link-local. |
| `9` | IANA Considerations | 0 | skipped (iana) | IANA Considerations. Records that IANA has assigned capability number 77 in the BGP Capability Codes registry, with the one-row table naming it. Binds IANA, not a speaker. |
| `10` | not stated | 0 | walked | not stated |
| `11` | References | 0 | skipped (references) | References. The heading over sections 11.1 and 11.2. |
| `11.1` | Normative References | 0 | skipped (references) | Normative References. |
| `11.2` | Informative References | 0 | skipped (references) | Informative References. |
| `A` | Appendix A | 0 | skipped (appendix-non-normative) | Appendix A. Motivations for a Capability. Two sentences saying Link-Local-only next hops have been inconsistently supported and that the capability lets two conforming implementations interoperate without extra configuration. |
| `B` | Appendix B | 0 | skipped (appendix-non-normative) | Appendix B. Inconsistency Reports. The RFC 7942 running-code notes for FRRouting and Bird. |
| `C` | Appendix C | 0 | skipped (appendix-non-normative) | Appendix C. Implementation Report. The RFC 7942 running-code note for FRRouting. |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `front:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | IETF Trust boilerplate in the Copyright Notice. It binds a person who copies Code Components out of the document, not a BGP speaker, and no behavior of Ze's can meet or break it. | Code Components extracted from this document must include Revised BSD License text as described in Section 4.e of the Trust Legal Provisions and are provided without warranty as described in the Revised BSD License. |
| `1:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Indicative statement about data-center deployments on point-to-point links: a Global address is not needed there. It removes a need rather than imposing one, and directs no implementation behavior; the procedures that make a Link-Local-only next hop valid are Sections 2 to 5, gated separately. | In these situations, a Global IPv6 address is not required for the advertisement of reachability information. |
| `1:3` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | A property of IPv6 addressing restated as context (RFC 4291, RFC 4862), not an obligation on a BGP speaker. The consequence the next sentences draw (a changed Link-Local resets the TCP session, and a tentative address is not advertised) is carried by DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-1-1 and by the transport, not by this site. | IPv6 Link-Local addresses are not required to be stable across interface startup or reconfiguration. |

## Superseded

No document obsoletes DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY, so its obligations are stated where they were written.
