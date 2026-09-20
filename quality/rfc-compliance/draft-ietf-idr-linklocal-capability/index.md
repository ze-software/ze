# DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY - Link-Local Next Hop Capability for BGP

Partial. Every requirement this repository extracted from DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 92.3% | 12 of 13 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 13 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 13 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 100.0% | 24 of 24 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 13 | of 25 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 13 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 13 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 13 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 13 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 7.7% | 1 of 13 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

The 7 shares marked as a part above are the whole of the 13 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| No test at all | bad | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green shares above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Partial |
| Enrolment | Enrolled |
| Requirements | 25 |
| Gated MUST-level | 13 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 1 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 24 |
| Tagged units | 24 |
| Recorded audit verdicts | 0 |
| Discrimination records | 24 |
| Summary | `rfc/short/draft-ietf-idr-linklocal-capability.md` |
| Requirement shard | `rfc/requirements/draft-ietf-idr-linklocal-capability.md` |
| RFC text | `rfc/drafts/draft-ietf-idr-linklocal-capability.txt` |

## Enrolment

Enrolled: Link-Local Next Hop capability for BGP (code 77): twelve MUST-level requirements, all conditioned by Section 2 on the capability being negotiated. Ze advertises the capability (extractLLNHCapabilities, internal/component/bgp/plugins/llnh/llnh.go), parses it back as a typed capability (parseCapability, internal/core/bgp/capability/capability.go) and records what it negotiated to (Negotiate, internal/core/bgp/capability/negotiated.go), which is the Section 2 condition every other procedure reads. 3-1 and 5-1 are met on the send side: Peer.linkLocalOnlyNextHopPermitted (internal/component/bgp/reactor/peer.go) admits a link-local next hop only where the session may carry the 16-octet form, and buildMPReach (internal/component/bgp/message/update_build.go) writes that address alone in 16 octets, or the global-then-link-local pair in 32. 4-5 and 4-6 are met on the reflection path: egressNextHopIsLinkLocalOnly (internal/component/bgp/reactor/forward_next_hop.go) classifies the field about to be written and sameLinkLayerSegment (internal/component/bgp/reactor/link_scope.go) answers the segment half, so forwardUpdateCore either sends the rewrite applyFactsNextHop recorded or withholds the announcement. 3-2 is met by the RFC 2545 32-octet path; 4-2 and 4-8 are met because a link-local is appended only when the peer shares a connected subnet. 4-1, 4-4, 4-7 and 4-9 are met because a route whose next hop has no wire form is refused rather than encoded, and 4-3 because a route reachable through the speaker carries the speaker's own link-local toward a one-hop peer. One MUST-level requirement stays outstanding and carries a `{gap}` on its checklist line.

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

- Capability 77 declared, negotiated and acted on, for the send side and for the reflection path. Declaration: `extractLLNHCapabilities` ([`internal/component/bgp/plugins/llnh/llnh.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/llnh/llnh.go)) advertises the empty code 77 capability for a peer or group whose config carries a `link-local-nexthop` key that is not `disable`. Negotiation: `parseCapability` ([`internal/core/bgp/capability/capability.go`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability.go)) types it and `Negotiate` ([`internal/core/bgp/capability/negotiated.go`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/negotiated.go)) sets `LinkLocalNextHop` only when both OPENs carried it, which `Peer.linkLocalOnlyNextHopPermitted` ([`internal/component/bgp/reactor/peer.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer.go)) reads beside the RFC 8950 Extended Next Hop Encoding state Section 5 asks for. Send: `resolveNextHop` (same file) refuses a link-local next hop on a session that may not carry it, so the route is left out rather than encoded in a form RFC 2545 Section 3 forbids, and `buildMPReach` ([`internal/component/bgp/message/update_build.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/update_build.go)) writes the 16-octet Link-Local-only field or the 32-octet pair, for IPv4 NLRI as well as IPv6. Reflection: `egressNextHopIsLinkLocalOnly` ([`internal/component/bgp/reactor/forward_next_hop.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_next_hop.go)) and `sameLinkLayerSegment` ([`internal/component/bgp/reactor/link_scope.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/link_scope.go)) decide, per client, between the rewrite and the ineligibility Section 4 allows. Receive: `parseNextHops` ([`internal/core/bgp/attribute/mpnlri.go`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/mpnlri.go)) still performs no `fe80::/10` test, so a received 16-octet link-local-only Next Hop is read as a Global IPv6 next hop
- the Section 3 receive sentence is indicative and carries no checklist row (see Notes). One MUST-level requirement carries a `{gap}`: 6-1, because a malformed Next Hop field is answered with the RFC 7606 Section 7.11 session reset rather than the treat-as-withdraw of Section 7.3.


**What the ledger says remains:**

-

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 12 | one part of the gated population |
| Annotated instead of tested | 1 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **13** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (12):** [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-3-1`](#draft-ietf-idr-linklocal-capability-3-1), [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-3-2`](#draft-ietf-idr-linklocal-capability-3-2), [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-1`](#draft-ietf-idr-linklocal-capability-4-1), [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-2`](#draft-ietf-idr-linklocal-capability-4-2), [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-3`](#draft-ietf-idr-linklocal-capability-4-3), [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-4`](#draft-ietf-idr-linklocal-capability-4-4), [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-5`](#draft-ietf-idr-linklocal-capability-4-5), [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-6`](#draft-ietf-idr-linklocal-capability-4-6), [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-7`](#draft-ietf-idr-linklocal-capability-4-7), [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-8`](#draft-ietf-idr-linklocal-capability-4-8), [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-9`](#draft-ietf-idr-linklocal-capability-4-9), [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-5-1`](#draft-ietf-idr-linklocal-capability-5-1)

**Annotated instead of tested (1):** [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-6-1`](#draft-ietf-idr-linklocal-capability-6-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-1-1` | "BGP speakers SHOULD NOT advertise a route whose Next Hop is a Link-Local address that is in the tentative state (Section 5.4 of [RFC4862]); this applies both to a first-party Next Hop (the speaker's own Link-Local address) and to a third-party Next Hop re-advertised from another peer" (§1) | SHOULD NOT | 1 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-2-1` | "A BGP speaker that is willing to use (send and receive) IPv6 Link-Local-only next hops SHOULD advertise the Link-Local Next Hop Capability to its peers only when: 1. It is capable of sending IPv6 Link-Local-only next hops for a route. 2. IPv6 Link-Local neighbors are associated with interfaces as part of their configuration to assist in determining the interface scope of received IPv6 Link-Local-only next hops" (§2) | SHOULD | 2 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-3-1` | "If an implementation intends to send a single IPv6 Link-Local forwarding address in the Next Hop field of the MP_REACH_NLRI, it MUST set the length of the Next Hop field to 16 and include only the IPv6 Link-Local address in the Next Hop field" (§3) | MUST | 3 | **positive:** `unit/verify` [`TestLinkLocalOnlyNextHopFieldIsSixteenOctets`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc_draft_linklocal_send_test.go#L87). **negative:** `unit/verify` [`TestLinkLocalNextHopWithAGlobalIsNotSentAlone`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc_draft_linklocal_send_test.go#L118) |
| `DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-3-2` | "If an implementation intends to send both a IPv6 Global and Link-Local forwarding address in the Next Hop field of the MP_REACH_NLRI, it MUST set the length of the Next Hop field to 32 and include both the IPv6 Global and Link-Local addresses in the Next Hop field" (§3) | MUST | 3 | **positive:** `unit/verify` [`TestLinkLocalBothAddressesUseThirtyTwoOctets`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc_draft_linklocal_test.go#L48). **negative:** `unit/verify` [`TestLinkLocalSingleAddressKeepsSixteenOctets`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc_draft_linklocal_test.go#L61) |
| `DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-1` | "If, after completing these procedures, there are no IPv6 next hop addresses included in the next hop, the BGP route MUST not be advertised to its peer" (§4) | MUST | 4 | **positive:** `unit/verify` [`TestLinkLocalRouteWithNoNextHopIsNotAdvertised`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc_draft_linklocal_advertise_test.go#L70). **negative:** `unit/verify` [`TestLinkLocalRouteWithANextHopIsAdvertised`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc_draft_linklocal_advertise_test.go#L83) |
| `DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-2` | "If the internal peer is more than one IP hop away, the BGP speaker MUST NOT include a Link-Local IPv6 next hop" (§4) | MUST NOT | 4 | **positive:** `unit/verify` [`TestLinkLocalNotIncludedForPeerMoreThanOneHopAway`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc_draft_linklocal_test.go#L73). **negative:** `unit/verify` [`TestLinkLocalIncludedForPeerOneHopAway`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc_draft_linklocal_test.go#L84) |
| `DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-3` | "If the route is directly connected to the speaker, or if the interface address of the router through which the announced network is reachable for the speaker is the internal peer's address, the next hop MUST include its own Link-Local IPv6 address" (§4) | MUST | 4 | **positive:** `unit/verify` [`TestLinkLocalOwnAddressIncludedForRouteReachableThroughTheSpeaker`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc_draft_linklocal_test.go#L135). **negative:** `unit/verify` [`TestLinkLocalOwnAddressNotInsertedWhenAnotherRouterIsTheNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc_draft_linklocal_test.go#L150) |
| `DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-4` | "If, after evaluating the above procedures, there are no IPv6 next hops included with the route, the route MUST NOT be announced to the remote BGP speaker" (§4) | MUST NOT | 4 | **positive:** `unit/verify` [`TestLinkLocalRouteWithNoNextHopIsNotAnnouncedToAnInternalPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc_draft_linklocal_advertise_test.go#L96). **negative:** `unit/verify` [`TestLinkLocalRouteWithANextHopIsAnnouncedToAnInternalPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc_draft_linklocal_advertise_test.go#L105) |
| `DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-5` | "A Route Reflector (RR) reflecting a route with a link-local-only next hop MUST NOT advertise that route to a client unless the client shares the same link-layer segment as the original advertiser" (§4) | MUST NOT | 4 | **positive:** `unit/verify` [`TestReflectedLinkLocalOnlyRouteIsWithheldFromAClientOffTheSegment`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc_draft_linklocal_reflect_test.go#L192). **negative:** `unit/verify` [`TestReflectedLinkLocalOnlyRouteIsWithheldFromAClientOffTheSegment`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc_draft_linklocal_reflect_test.go#L201) |
| `DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-6` | "For all other clients, the RR MUST either rewrite the next hop to its own address (next-hop-self) or consider the route ineligible for advertisement to that specific peer" (§4) | MUST | 4 | **positive:** `unit/verify` [`TestReflectedLinkLocalOnlyRouteIsRewrittenOrIneligibleForOtherClients`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc_draft_linklocal_reflect_test.go#L228). **negative:** `unit/verify` [`TestReflectedGlobalNextHopRouteIsNeitherRewrittenNorWithheld`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc_draft_linklocal_reflect_test.go#L258) |
| `DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-7` | "If no next hops are included, the route MUST NOT be announced (treat-as-withdraw)" (§4) | MUST NOT | 4 | **positive:** `unit/verify` [`TestLinkLocalRouteWithNoNextHopIsNotAnnouncedToAnExternalPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc_draft_linklocal_advertise_test.go#L117). **negative:** `unit/verify` [`TestLinkLocalRouteWithANextHopIsAnnouncedToAnExternalPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc_draft_linklocal_advertise_test.go#L126) |
| `DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-8` | "Link-Local IPv6 next hops MUST NOT be included" for an external peer that "is multiple IP hops away from the speaker (aka \\"multihop EBGP\\")" (§4) | MUST NOT | 4 | **positive:** `unit/verify` [`TestLinkLocalNotIncludedForMultihopExternalPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc_draft_linklocal_test.go#L95). **negative:** `unit/verify` [`TestLinkLocalIncludedForDirectlyAttachedExternalPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc_draft_linklocal_test.go#L108) |
| `DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-9` | "If a Global IPv6 next hop is not included, the route MUST NOT be advertised to the external peer (treat-as-withdraw)" (§4) | MUST NOT | 4 | **positive:** `unit/verify` [`TestLinkLocalRouteWithNoGlobalNextHopIsNotAdvertisedToAMultihopExternalPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc_draft_linklocal_advertise_test.go#L137). **negative:** `unit/verify` [`TestLinkLocalRouteWithAGlobalNextHopIsAdvertisedToAMultihopExternalPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc_draft_linklocal_advertise_test.go#L151) |
| `DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-10` | "When sending a message to an internal peer, if the route is not locally-originated, the BGP speaker SHOULD NOT modify the Global IPv6 next hop, if one is present, unless it has been explicitly configured to announce its own IP address as the next hop" (§4) | SHOULD NOT | 4 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-11` | "implementations SHOULD log this suppression, or otherwise expose it through operator notification (e.g., via BMP or YANG telemetry), so that unexpected reachability gaps can be detected" (§4) | SHOULD | 4 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-12` | "If the external peer is one IP hop away, the announcing BGP speaker SHOULD include a Link-Local IPv6 next hop" (§4) | SHOULD | 4 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-13` | "If a BGP speaker receives a route with a link-local-only next hop, the route SHOULD be considered unusable for forwarding, consistent with the next-hop resolvability requirements described in [RFC4271]" (§4) | SHOULD | 4 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-14` | "By default, the BGP speaker SHOULD use the Global IPv6 address of the interface that the speaker uses in the next hop to establish the BGP connection to peer X" (§4) | SHOULD | 4 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-5-1` | "When this combination has not been negotiated, a sender MUST follow the rules in Section 3 of [RFC8950] and encode the Next Hop as 32 octets" (§5) | MUST | 5 | **positive:** `unit/verify` [`TestIPv4NLRINextHopIsThirtyTwoOctetsWithoutTheCombination`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc_draft_linklocal_send_test.go#L141). **negative:** `unit/verify` [`TestIPv4NLRILinkLocalOnlyNextHopNeedsTheCombination`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc_draft_linklocal_send_test.go#L173) |
| `DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-6-1` | "If the Next Hop field is malformed, the implementation MUST handle the malformed UPDATE message using the approach of \\"treat-as-withdraw\\", as described in section 7.3 of [RFC7606]" (§6) | MUST | 6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** validateMPReachNextHop (internal/component/bgp/message/rfc7606.go) answers a next-hop length outside attribute.ValidNextHopLens with RFC7606ActionSessionReset, which is RFC 7606 Section 7.11's approach and not the treat-as-withdraw of Section 7.3 this requirement names, and no producer inspects the CONTENT of a field whose length is admitted |
| `DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-6-2` | "Receivers SHOULD use the second Link-Local IPv6 address for forwarding, because the second slot is the position that carries the Link-Local address in the conforming Global-then-Link-Local layout defined by [RFC2545], and thus is the value the sender most likely intended as the Link-Local next hop" (§6) | SHOULD | 6 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-6-3` | "If the Next Hop field is properly formed, but the IPv6 Link-Local next hop is not reachable (as determined by an examination of the IPv6 neighbor table), the route SHOULD be considered unusable for forwarding purposes, in accordance with the next hop resolvability conditions described in [RFC4271]" (§6) | SHOULD | 6 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-7-1` | "Implementations SHOULD support BGP Add-Path [RFC7911] and Extended Next-Hop Encoding [RFC8950] to ensure full path utilization in IPv4-over-IPv6 underlays" (§7) | SHOULD | 7 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-7-2` | "Implementations SHOULD provide specific telemetry via the BGP Monitoring Protocol (BMP) [RFC7854] or a BGP YANG model (e.g., [I-D.ietf-idr-bgp-model]) to expose the state of link-local capability negotiation" (§7) | SHOULD | 7 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-7-3` | "implementations SHOULD treat a change in the local Link-Local address as a session reset rather than as a graceful restart event" (§7) | SHOULD | 7 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-6-1`](#draft-ietf-idr-linklocal-capability-6-1) "If the Next Hop field is malformed, the implementation MUST handle the malformed UPDATE message using the approach of \\"treat-as-withdraw\\", as described in section 7.3 of [RFC7606]" (§6) | {gap}, no test | validateMPReachNextHop (internal/component/bgp/message/rfc7606.go) answers a next-hop length outside attribute.ValidNextHopLens with RFC7606ActionSessionReset, which is RFC 7606 Section 7.11's approach and not the treat-as-withdraw of Section 7.3 this requirement names, and no producer inspects the CONTENT of a field whose length is admitted |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-3-1`](#draft-ietf-idr-linklocal-capability-3-1)

"If an implementation intends to send a single IPv6 Link-Local forwarding address in the Next Hop field of the MP_REACH_NLRI, it MUST set the length of the Next Hop field to 16 and include only the IPv6 Link-Local address in the Next Hop field" (§3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLinkLocalNextHopWithAGlobalIsNotSentAlone`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc_draft_linklocal_send_test.go#L118) | unit/verify | revert, verified |
| positive | [`TestLinkLocalOnlyNextHopFieldIsSixteenOctets`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc_draft_linklocal_send_test.go#L87) | unit/verify | revert, verified |

### [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-3-2`](#draft-ietf-idr-linklocal-capability-3-2)

"If an implementation intends to send both a IPv6 Global and Link-Local forwarding address in the Next Hop field of the MP_REACH_NLRI, it MUST set the length of the Next Hop field to 32 and include both the IPv6 Global and Link-Local addresses in the Next Hop field" (§3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLinkLocalSingleAddressKeepsSixteenOctets`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc_draft_linklocal_test.go#L61) | unit/verify | revert, verified |
| positive | [`TestLinkLocalBothAddressesUseThirtyTwoOctets`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc_draft_linklocal_test.go#L48) | unit/verify | revert, verified |

### [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-1`](#draft-ietf-idr-linklocal-capability-4-1)

"If, after completing these procedures, there are no IPv6 next hop addresses included in the next hop, the BGP route MUST not be advertised to its peer" (§4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLinkLocalRouteWithANextHopIsAdvertised`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc_draft_linklocal_advertise_test.go#L83) | unit/verify | revert, verified |
| positive | [`TestLinkLocalRouteWithNoNextHopIsNotAdvertised`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc_draft_linklocal_advertise_test.go#L70) | unit/verify | revert, verified |

### [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-2`](#draft-ietf-idr-linklocal-capability-4-2)

"If the internal peer is more than one IP hop away, the BGP speaker MUST NOT include a Link-Local IPv6 next hop" (§4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLinkLocalIncludedForPeerOneHopAway`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc_draft_linklocal_test.go#L84) | unit/verify | revert, verified |
| positive | [`TestLinkLocalNotIncludedForPeerMoreThanOneHopAway`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc_draft_linklocal_test.go#L73) | unit/verify | revert, verified |

### [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-3`](#draft-ietf-idr-linklocal-capability-4-3)

"If the route is directly connected to the speaker, or if the interface address of the router through which the announced network is reachable for the speaker is the internal peer's address, the next hop MUST include its own Link-Local IPv6 address" (§4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLinkLocalOwnAddressNotInsertedWhenAnotherRouterIsTheNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc_draft_linklocal_test.go#L150) | unit/verify | revert, verified |
| positive | [`TestLinkLocalOwnAddressIncludedForRouteReachableThroughTheSpeaker`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc_draft_linklocal_test.go#L135) | unit/verify | revert, verified |

### [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-4`](#draft-ietf-idr-linklocal-capability-4-4)

"If, after evaluating the above procedures, there are no IPv6 next hops included with the route, the route MUST NOT be announced to the remote BGP speaker" (§4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLinkLocalRouteWithANextHopIsAnnouncedToAnInternalPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc_draft_linklocal_advertise_test.go#L105) | unit/verify | revert, verified |
| positive | [`TestLinkLocalRouteWithNoNextHopIsNotAnnouncedToAnInternalPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc_draft_linklocal_advertise_test.go#L96) | unit/verify | revert, verified |

### [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-5`](#draft-ietf-idr-linklocal-capability-4-5)

"A Route Reflector (RR) reflecting a route with a link-local-only next hop MUST NOT advertise that route to a client unless the client shares the same link-layer segment as the original advertiser" (§4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestReflectedLinkLocalOnlyRouteIsWithheldFromAClientOffTheSegment`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc_draft_linklocal_reflect_test.go#L201) | unit/verify | revert, verified |
| positive | [`TestReflectedLinkLocalOnlyRouteIsWithheldFromAClientOffTheSegment`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc_draft_linklocal_reflect_test.go#L192) | unit/verify | revert, verified |

### [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-6`](#draft-ietf-idr-linklocal-capability-4-6)

"For all other clients, the RR MUST either rewrite the next hop to its own address (next-hop-self) or consider the route ineligible for advertisement to that specific peer" (§4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestReflectedGlobalNextHopRouteIsNeitherRewrittenNorWithheld`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc_draft_linklocal_reflect_test.go#L258) | unit/verify | revert, verified |
| positive | [`TestReflectedLinkLocalOnlyRouteIsRewrittenOrIneligibleForOtherClients`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc_draft_linklocal_reflect_test.go#L228) | unit/verify | revert, verified |

### [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-7`](#draft-ietf-idr-linklocal-capability-4-7)

"If no next hops are included, the route MUST NOT be announced (treat-as-withdraw)" (§4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLinkLocalRouteWithANextHopIsAnnouncedToAnExternalPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc_draft_linklocal_advertise_test.go#L126) | unit/verify | revert, verified |
| positive | [`TestLinkLocalRouteWithNoNextHopIsNotAnnouncedToAnExternalPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc_draft_linklocal_advertise_test.go#L117) | unit/verify | revert, verified |

### [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-8`](#draft-ietf-idr-linklocal-capability-4-8)

"Link-Local IPv6 next hops MUST NOT be included" for an external peer that "is multiple IP hops away from the speaker (aka \"multihop EBGP\")" (§4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLinkLocalIncludedForDirectlyAttachedExternalPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc_draft_linklocal_test.go#L108) | unit/verify | revert, verified |
| positive | [`TestLinkLocalNotIncludedForMultihopExternalPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc_draft_linklocal_test.go#L95) | unit/verify | revert, verified |

### [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-9`](#draft-ietf-idr-linklocal-capability-4-9)

"If a Global IPv6 next hop is not included, the route MUST NOT be advertised to the external peer (treat-as-withdraw)" (§4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLinkLocalRouteWithAGlobalNextHopIsAdvertisedToAMultihopExternalPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc_draft_linklocal_advertise_test.go#L151) | unit/verify | revert, verified |
| positive | [`TestLinkLocalRouteWithNoGlobalNextHopIsNotAdvertisedToAMultihopExternalPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc_draft_linklocal_advertise_test.go#L137) | unit/verify | revert, verified |

### [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-5-1`](#draft-ietf-idr-linklocal-capability-5-1)

"When this combination has not been negotiated, a sender MUST follow the rules in Section 3 of [RFC8950] and encode the Next Hop as 32 octets" (§5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestIPv4NLRILinkLocalOnlyNextHopNeedsTheCombination`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc_draft_linklocal_send_test.go#L173) | unit/verify | revert, verified |
| positive | [`TestIPv4NLRINextHopIsThirtyTwoOctetsWithoutTheCombination`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc_draft_linklocal_send_test.go#L141) | unit/verify | revert, verified |

### [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-6-1`](#draft-ietf-idr-linklocal-capability-6-1)

"If the Next Hop field is malformed, the implementation MUST handle the malformed UPDATE message using the approach of \"treat-as-withdraw\", as described in section 7.3 of [RFC7606]" (§6)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-6-1, so no unit is bound to it.

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | ze-work agent, spec-rfcgate-6 draft walk, draft-ietf-idr-linklocal-capability |
| Signed off | 2026-09-01 |
| Register | prose |
| Source | rfc/drafts/draft-ietf-idr-linklocal-capability.txt |
| Source fingerprint | f6a372b9b6ab4db5 |
| Record | rfc/extraction/draft-ietf-idr-linklocal-capability.json |
| Mapped sentences | 13 |
| Declined as scope | 0 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | skipped (front-matter) | Title block, Abstract, Status of This Memo, Copyright Notice and Table of Contents. The Abstract says the document updates RFC 2545 to clarify the next-hop encoding when only an IPv6 Link-Local address is available, and defines a capability signalling support for it. It directs no speaker. Its one site is the Copyright Notice, excluded below. |
| `1` | not stated | 0 | walked | not stated |
| `2` | not stated | 0 | walked | not stated |
| `3` | not stated | 2 | walked | not stated |
| `4` | not stated | 9 | walked | not stated |
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

The walk over DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY declined no sentence: every site it found is mapped to a requirement.

## Superseded

No document obsoletes DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY, so its obligations are stated where they were written.
