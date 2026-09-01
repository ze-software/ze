# DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY - Link-Local Next Hop Capability for BGP

Partial. Every requirement this repository extracted from DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0.0% | 0 of 13 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 13 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 13 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 0.0% | 0 of 0 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 13 | of 25 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 13 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 100.0% | 13 of 13 binding obligations | no test carries the requirement id, whether or not a gap states why |

The 4 shares marked as a part above are the whole of the 13 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| No test at all | bad | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Partial |
| Enrolment | Enrolled |
| Requirements | 25 |
| Gated MUST-level | 13 |
| Obligations that bind Ze | 13 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Gated with no test | 13 |
| Nightly-only evidence | 0 |
| Test tags | 0 |
| Tagged units | 0 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/draft-ietf-idr-linklocal-capability.md` |
| Requirement shard | `rfc/requirements/draft-ietf-idr-linklocal-capability.md` |
| RFC text | `rfc/drafts/draft-ietf-idr-linklocal-capability.txt` |

## Enrolment

Enrolled: Link-Local Next Hop capability for BGP (code 77): twelve MUST-level requirements, all conditioned by Section 2 on the capability being negotiated. Ze advertises the capability (extractLLNHCapabilities, internal/component/bgp/plugins/llnh/llnh.go) and implements none of the 16-octet Link-Local-only next-hop procedures behind it: linkScope.linkLocalNextHop and applyLinkLocalNextHop (internal/component/bgp/reactor/link_scope.go) emit only the RFC 2545 forms, and parseNextHops (internal/core/bgp/attribute/mpnlri.go) runs no fe80::/10 classification on a 16-octet next hop. 3-2 is met by the RFC 2545 32-octet path; 4-2 and 4-8 are met because a link-local is appended only when the peer shares a connected subnet. The rest are outstanding and named in the coverage rollup.

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

Capability declaration only. `extractLLNHCapabilities` ([`internal/component/bgp/plugins/llnh/llnh.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/llnh/llnh.go)) advertises the empty code 77 capability for a peer or group whose config carries a `link-local-nexthop` key that is not `disable`. The draft's own procedures are the 16-octet Link-Local-ONLY Next Hop form, and ze produces neither side of it. Send: `linkScope.linkLocalNextHop` and `applyLinkLocalNextHop` ([`internal/component/bgp/reactor/link_scope.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/link_scope.go)) emit the RFC 2545 forms only, a 16-octet GLOBAL address or the 32-octet global-then-link-local pair, and `attribute.ValidateGlobalNextHop` ([`internal/core/bgp/attribute/nexthop_form.go`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/nexthop_form.go)) refuses a link-local address in the first slot. Receive: `parseNextHops` ([`internal/core/bgp/attribute/mpnlri.go`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/mpnlri.go)) performs no `fe80::/10` test, so a 16-octet link-local-only Next Hop is read as a Global IPv6 next hop. `parseCapability` ([`internal/core/bgp/capability/capability.go`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability.go)) has no case for code 77, so ze records no negotiated state and no ze path is conditioned on the capability. The requirements are listed per line in [`rfc/short/draft-ietf-idr-linklocal-capability.md`](https://github.com/ze-software/ze/blob/main/rfc/short/draft-ietf-idr-linklocal-capability.md) and the walk is bounded by [`rfc/extraction/draft-ietf-idr-linklocal-capability.json`](https://github.com/ze-software/ze/blob/main/rfc/extraction/draft-ietf-idr-linklocal-capability.json).

**What the ledger says remains:**

-

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 0 | one part of the gated population |
| Annotated instead of tested | 0 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 13 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **13** | every gated MUST falls in exactly one bucket above |

**No test and no annotation (13):** [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-3-1`](#draft-ietf-idr-linklocal-capability-3-1), [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-3-2`](#draft-ietf-idr-linklocal-capability-3-2), [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-1`](#draft-ietf-idr-linklocal-capability-4-1), [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-2`](#draft-ietf-idr-linklocal-capability-4-2), [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-3`](#draft-ietf-idr-linklocal-capability-4-3), [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-4`](#draft-ietf-idr-linklocal-capability-4-4), [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-5`](#draft-ietf-idr-linklocal-capability-4-5), [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-6`](#draft-ietf-idr-linklocal-capability-4-6), [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-7`](#draft-ietf-idr-linklocal-capability-4-7), [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-8`](#draft-ietf-idr-linklocal-capability-4-8), [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-9`](#draft-ietf-idr-linklocal-capability-4-9), [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-5-1`](#draft-ietf-idr-linklocal-capability-5-1), [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-6-1`](#draft-ietf-idr-linklocal-capability-6-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-1-1` | "BGP speakers SHOULD NOT advertise a route whose Next Hop is a Link-Local address that is in the tentative state (Section 5.4 of [RFC4862]); this applies both to a first-party Next Hop (the speaker's own Link-Local address) and to a third-party Next Hop re-advertised from another peer" (§1) | SHOULD NOT | 1 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-2-1` | "A BGP speaker that is willing to use (send and receive) IPv6 Link-Local-only next hops SHOULD advertise the Link-Local Next Hop Capability to its peers only when: 1. It is capable of sending IPv6 Link-Local-only next hops for a route. 2. IPv6 Link-Local neighbors are associated with interfaces as part of their configuration to assist in determining the interface scope of received IPv6 Link-Local-only next hops" (§2) | SHOULD | 2 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-3-1` | "If an implementation intends to send a single IPv6 Link-Local forwarding address in the Next Hop field of the MP_REACH_NLRI, it MUST set the length of the Next Hop field to 16 and include only the IPv6 Link-Local address in the Next Hop field" (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-3-2` | "If an implementation intends to send both a IPv6 Global and Link-Local forwarding address in the Next Hop field of the MP_REACH_NLRI, it MUST set the length of the Next Hop field to 32 and include both the IPv6 Global and Link-Local addresses in the Next Hop field" (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-1` | "If, after completing these procedures, there are no IPv6 next hop addresses included in the next hop, the BGP route MUST not be advertised to its peer" (§4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-2` | "If the internal peer is more than one IP hop away, the BGP speaker MUST NOT include a Link-Local IPv6 next hop" (§4) | MUST NOT | 4 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-3` | "If the route is directly connected to the speaker, or if the interface address of the router through which the announced network is reachable for the speaker is the internal peer's address, the next hop MUST include its own Link-Local IPv6 address" (§4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-4` | "If, after evaluating the above procedures, there are no IPv6 next hops included with the route, the route MUST NOT be announced to the remote BGP speaker" (§4) | MUST NOT | 4 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-5` | "A Route Reflector (RR) reflecting a route with a link-local-only next hop MUST NOT advertise that route to a client unless the client shares the same link-layer segment as the original advertiser" (§4) | MUST NOT | 4 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-6` | "For all other clients, the RR MUST either rewrite the next hop to its own address (next-hop-self) or consider the route ineligible for advertisement to that specific peer" (§4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-7` | "If no next hops are included, the route MUST NOT be announced (treat-as-withdraw)" (§4) | MUST NOT | 4 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-8` | "Link-Local IPv6 next hops MUST NOT be included" for an external peer that "is multiple IP hops away from the speaker (aka \\"multihop EBGP\\")" (§4) | MUST NOT | 4 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-9` | "If a Global IPv6 next hop is not included, the route MUST NOT be advertised to the external peer (treat-as-withdraw)" (§4) | MUST NOT | 4 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-10` | "When sending a message to an internal peer, if the route is not locally-originated, the BGP speaker SHOULD NOT modify the Global IPv6 next hop, if one is present, unless it has been explicitly configured to announce its own IP address as the next hop" (§4) | SHOULD NOT | 4 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-11` | "implementations SHOULD log this suppression, or otherwise expose it through operator notification (e.g., via BMP or YANG telemetry), so that unexpected reachability gaps can be detected" (§4) | SHOULD | 4 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-12` | "If the external peer is one IP hop away, the announcing BGP speaker SHOULD include a Link-Local IPv6 next hop" (§4) | SHOULD | 4 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-13` | "If a BGP speaker receives a route with a link-local-only next hop, the route SHOULD be considered unusable for forwarding, consistent with the next-hop resolvability requirements described in [RFC4271]" (§4) | SHOULD | 4 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-14` | "By default, the BGP speaker SHOULD use the Global IPv6 address of the interface that the speaker uses in the next hop to establish the BGP connection to peer X" (§4) | SHOULD | 4 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-5-1` | "When this combination has not been negotiated, a sender MUST follow the rules in Section 3 of [RFC8950] and encode the Next Hop as 32 octets" (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-6-1` | "If the Next Hop field is malformed, the implementation MUST handle the malformed UPDATE message using the approach of \\"treat-as-withdraw\\", as described in section 7.3 of [RFC7606]" (§6) | MUST | 6 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-6-2` | "Receivers SHOULD use the second Link-Local IPv6 address for forwarding, because the second slot is the position that carries the Link-Local address in the conforming Global-then-Link-Local layout defined by [RFC2545], and thus is the value the sender most likely intended as the Link-Local next hop" (§6) | SHOULD | 6 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-6-3` | "If the Next Hop field is properly formed, but the IPv6 Link-Local next hop is not reachable (as determined by an examination of the IPv6 neighbor table), the route SHOULD be considered unusable for forwarding purposes, in accordance with the next hop resolvability conditions described in [RFC4271]" (§6) | SHOULD | 6 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-7-1` | "Implementations SHOULD support BGP Add-Path [RFC7911] and Extended Next-Hop Encoding [RFC8950] to ensure full path utilization in IPv4-over-IPv6 underlays" (§7) | SHOULD | 7 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-7-2` | "Implementations SHOULD provide specific telemetry via the BGP Monitoring Protocol (BMP) [RFC7854] or a BGP YANG model (e.g., [I-D.ietf-idr-bgp-model]) to expose the state of link-local capability negotiation" (§7) | SHOULD | 7 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-7-3` | "implementations SHOULD treat a change in the local Link-Local address as a session reset rather than as a graceful restart event" (§7) | SHOULD | 7 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-3-1`](#draft-ietf-idr-linklocal-capability-3-1) "If an implementation intends to send a single IPv6 Link-Local forwarding address in the Next Hop field of the MP_REACH_NLRI, it MUST set the length of the Next Hop field to 16 and include only the IPv6 Link-Local address in the Next Hop field" (§3) | no test | no test carries this requirement id |
| [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-3-2`](#draft-ietf-idr-linklocal-capability-3-2) "If an implementation intends to send both a IPv6 Global and Link-Local forwarding address in the Next Hop field of the MP_REACH_NLRI, it MUST set the length of the Next Hop field to 32 and include both the IPv6 Global and Link-Local addresses in the Next Hop field" (§3) | no test | no test carries this requirement id |
| [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-1`](#draft-ietf-idr-linklocal-capability-4-1) "If, after completing these procedures, there are no IPv6 next hop addresses included in the next hop, the BGP route MUST not be advertised to its peer" (§4) | no test | no test carries this requirement id |
| [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-2`](#draft-ietf-idr-linklocal-capability-4-2) "If the internal peer is more than one IP hop away, the BGP speaker MUST NOT include a Link-Local IPv6 next hop" (§4) | no test | no test carries this requirement id |
| [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-3`](#draft-ietf-idr-linklocal-capability-4-3) "If the route is directly connected to the speaker, or if the interface address of the router through which the announced network is reachable for the speaker is the internal peer's address, the next hop MUST include its own Link-Local IPv6 address" (§4) | no test | no test carries this requirement id |
| [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-4`](#draft-ietf-idr-linklocal-capability-4-4) "If, after evaluating the above procedures, there are no IPv6 next hops included with the route, the route MUST NOT be announced to the remote BGP speaker" (§4) | no test | no test carries this requirement id |
| [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-5`](#draft-ietf-idr-linklocal-capability-4-5) "A Route Reflector (RR) reflecting a route with a link-local-only next hop MUST NOT advertise that route to a client unless the client shares the same link-layer segment as the original advertiser" (§4) | no test | no test carries this requirement id |
| [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-6`](#draft-ietf-idr-linklocal-capability-4-6) "For all other clients, the RR MUST either rewrite the next hop to its own address (next-hop-self) or consider the route ineligible for advertisement to that specific peer" (§4) | no test | no test carries this requirement id |
| [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-7`](#draft-ietf-idr-linklocal-capability-4-7) "If no next hops are included, the route MUST NOT be announced (treat-as-withdraw)" (§4) | no test | no test carries this requirement id |
| [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-8`](#draft-ietf-idr-linklocal-capability-4-8) "Link-Local IPv6 next hops MUST NOT be included" for an external peer that "is multiple IP hops away from the speaker (aka \\"multihop EBGP\\")" (§4) | no test | no test carries this requirement id |
| [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-9`](#draft-ietf-idr-linklocal-capability-4-9) "If a Global IPv6 next hop is not included, the route MUST NOT be advertised to the external peer (treat-as-withdraw)" (§4) | no test | no test carries this requirement id |
| [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-5-1`](#draft-ietf-idr-linklocal-capability-5-1) "When this combination has not been negotiated, a sender MUST follow the rules in Section 3 of [RFC8950] and encode the Next Hop as 32 octets" (§5) | no test | no test carries this requirement id |
| [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-6-1`](#draft-ietf-idr-linklocal-capability-6-1) "If the Next Hop field is malformed, the implementation MUST handle the malformed UPDATE message using the approach of \\"treat-as-withdraw\\", as described in section 7.3 of [RFC7606]" (§6) | no test | no test carries this requirement id |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-3-1`](#draft-ietf-idr-linklocal-capability-3-1)

"If an implementation intends to send a single IPv6 Link-Local forwarding address in the Next Hop field of the MP_REACH_NLRI, it MUST set the length of the Next Hop field to 16 and include only the IPv6 Link-Local address in the Next Hop field" (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-3-1, so no unit is bound to it.

### [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-3-2`](#draft-ietf-idr-linklocal-capability-3-2)

"If an implementation intends to send both a IPv6 Global and Link-Local forwarding address in the Next Hop field of the MP_REACH_NLRI, it MUST set the length of the Next Hop field to 32 and include both the IPv6 Global and Link-Local addresses in the Next Hop field" (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-3-2, so no unit is bound to it.

### [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-1`](#draft-ietf-idr-linklocal-capability-4-1)

"If, after completing these procedures, there are no IPv6 next hop addresses included in the next hop, the BGP route MUST not be advertised to its peer" (§4)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-1, so no unit is bound to it.

### [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-2`](#draft-ietf-idr-linklocal-capability-4-2)

"If the internal peer is more than one IP hop away, the BGP speaker MUST NOT include a Link-Local IPv6 next hop" (§4)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-2, so no unit is bound to it.

### [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-3`](#draft-ietf-idr-linklocal-capability-4-3)

"If the route is directly connected to the speaker, or if the interface address of the router through which the announced network is reachable for the speaker is the internal peer's address, the next hop MUST include its own Link-Local IPv6 address" (§4)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-3, so no unit is bound to it.

### [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-4`](#draft-ietf-idr-linklocal-capability-4-4)

"If, after evaluating the above procedures, there are no IPv6 next hops included with the route, the route MUST NOT be announced to the remote BGP speaker" (§4)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-4, so no unit is bound to it.

### [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-5`](#draft-ietf-idr-linklocal-capability-4-5)

"A Route Reflector (RR) reflecting a route with a link-local-only next hop MUST NOT advertise that route to a client unless the client shares the same link-layer segment as the original advertiser" (§4)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-5, so no unit is bound to it.

### [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-6`](#draft-ietf-idr-linklocal-capability-4-6)

"For all other clients, the RR MUST either rewrite the next hop to its own address (next-hop-self) or consider the route ineligible for advertisement to that specific peer" (§4)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-6, so no unit is bound to it.

### [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-7`](#draft-ietf-idr-linklocal-capability-4-7)

"If no next hops are included, the route MUST NOT be announced (treat-as-withdraw)" (§4)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-7, so no unit is bound to it.

### [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-8`](#draft-ietf-idr-linklocal-capability-4-8)

"Link-Local IPv6 next hops MUST NOT be included" for an external peer that "is multiple IP hops away from the speaker (aka \"multihop EBGP\")" (§4)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-8, so no unit is bound to it.

### [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-9`](#draft-ietf-idr-linklocal-capability-4-9)

"If a Global IPv6 next hop is not included, the route MUST NOT be advertised to the external peer (treat-as-withdraw)" (§4)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-9, so no unit is bound to it.

### [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-5-1`](#draft-ietf-idr-linklocal-capability-5-1)

"When this combination has not been negotiated, a sender MUST follow the rules in Section 3 of [RFC8950] and encode the Next Hop as 32 octets" (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-5-1, so no unit is bound to it.

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
