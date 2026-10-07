# RFC 4760 - Multiprotocol Extensions for BGP-4

Supported. Every requirement this repository extracted from RFC 4760, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 83.3% | 5 of 6 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 16.7% | 1 of 6 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 6 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 6 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| No test at all | 0.0% | 0 of 6 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 39.4% | 13 of 33 tagged units, 0 escaped and 1 lapsed | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |
| Audit verdicts | 7 | of 6 gated MUSTs judged | 0 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 6 | of 17 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 6 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 6 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 6 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 6 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

The 8 shares marked as a part above are the whole of the 6 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| Partial proof; remaining gap | ok | green at zero, RED above it: a tested clause cannot prove the whole requirement |
| No test at all | ok | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green shares above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | ok | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Supported |
| Enrolment | Enrolled |
| Requirements | 17 |
| Gated MUST-level | 6 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 33 |
| Tagged units | 33 |
| Recorded audit verdicts | 7 |
| Discrimination records | 14 |
| Summary | `rfc/short/rfc4760.md` |
| Requirement shard | `rfc/requirements/rfc4760.md` |
| RFC text | `rfc/full/rfc4760.txt` |

## Enrolment

Enrolled: Multiprotocol Extensions for BGP-4: six MUST-level requirements, all six met: 3-2 (the next-hop length determines the next-hop protocol) carries positive+negative tags; 3-3 (an UPDATE with MP_REACH_NLRI also carries ORIGIN and AS_PATH) and 3-4 (an iBGP UPDATE carrying MP_REACH includes LOCAL_PREF) carry positive+negative tags on new internal/component/bgp/message tests; 3-1 (the MP_REACH Reserved octet is 0) and 8-1 (advertise the Multiprotocol capability) are {single-polarity: positive}. 7-1 (Section 7 bulk per-AFI/SAFI route deletion) carries positive+negative tags on internal/component/bgp/reactor/rfc4760_section7_test.go. The non-applicability annotation that stood here, claiming RFC 7606 superseded the behavior, was voided by the owner on 2026-08-31: RFC 7606 Section 3 clause (j) keeps the obligation, and Ze meets it by session reset, which drops every route from that neighbor and so a superset of that AFI/SAFI's routes.

## What the public ledger says

**Status:** Supported

**What the ledger says is covered:**

AFI/SAFI capability negotiation, MP_REACH_NLRI, MP_UNREACH_NLRI, family-specific UPDATE handling.

**What the ledger says remains:**

RFC 7606 MP attribute ordering tradeoff is tracked under RFC 7606.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 5 | one part of the gated population |
| Annotated (including scoped evidence) | 1 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **6** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (5):** [`RFC4760-3-2`](#rfc4760-3-2), [`RFC4760-3-3`](#rfc4760-3-3), [`RFC4760-3-4`](#rfc4760-3-4), [`RFC4760-7-1`](#rfc4760-7-1), [`RFC4760-8-1`](#rfc4760-8-1)

**Annotated (including scoped evidence) (1):** [`RFC4760-3-1`](#rfc4760-3-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC4760-3-1` | A 1 octet field that MUST be set to 0 (Section 3) | MUST | 3 - Multiprotocol Reachable NLRI - MP_REACH_NLRI (Type Code 14) | **positive:** `unit/verify` [`TestMPReachNLRI_WriteTo`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/mpnlri_test.go#L12). **positive:** `unit/verify` [`TestRFC4760ReservedIsWrittenNotInherited`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc4760_reserved_test.go#L37). **positive:** `unit/verify` [`TestRFC4760ReservedSurvivesBufferReuse`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc4760_reserved_test.go#L122). **negative:** no negative test. **{single-polarity}:** MPReachNLRI.WriteTo writes the Reserved octet as 0 unconditionally, so there is no non-zero form to reject (internal/core/bgp/attribute/mpnlri.go:182) |
| `RFC4760-3-2` | If the Next Hop is allowed to be from more than one Network Layer protocol, the encoding of the Next Hop MUST provide a way to determine its Network Layer protocol. (Section 3) | MUST | 3 - Multiprotocol Reachable NLRI - MP_REACH_NLRI (Type Code 14) | **positive:** `unit/verify` [`TestCommitVPNAnnounceCarriesTheRFC4364NextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/rib/rfc4760_commit_nexthop_test.go#L155). **positive:** `unit/verify` [`TestMPReachNLRI_WriteTo`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/mpnlri_test.go#L16). **positive:** `unit/verify` [`TestMPReachNextHopLengthCountsTheOctetsWritten`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc4760_mpnlri_nexthop_wire_test.go#L39). **positive:** `unit/verify` [`TestParseMPReachNLRI`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/mpnlri_test.go#L128). **negative:** `unit/verify` [`TestBuildRIBRouteUpdate_RefusesANextHopWithNoWireForm`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4760_peer_rib_routes_nexthop_test.go#L50). **negative:** `unit/verify` [`TestCommitRefusesAnAnnounceWhoseNextHopHasNoWireForm`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/rib/rfc4760_commit_nexthop_test.go#L69). **negative:** `unit/verify` [`TestMPReachValidateNextHopsRefusesAnAddressWithNoWireForm`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc4760_mpnlri_nexthop_wire_test.go#L124). **negative:** `unit/verify` [`TestParseMPReachNLRI_InvalidNextHopLength`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/mpnlri_test.go#L498) |
| `RFC4760-3-3` | An UPDATE message that carries the MP_REACH_NLRI MUST also carry the ORIGIN and the AS_PATH attributes (both in EBGP and in IBGP exchanges). (Section 3) | MUST | 3 - Multiprotocol Reachable NLRI - MP_REACH_NLRI (Type Code 14) | **positive:** `unit/verify` [`TestRFC4760MPReachRequiresOriginAndASPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4760_mp_reach_test.go#L85). **positive:** `unit/verify` [`TestRFC4760MPReachUpdateCarriesOriginASPathAndIBGPLocalPref`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4760_mp_reach_mandatory_send_test.go#L22). **negative:** `unit/verify` [`TestRFC4760MPReachRequiresOriginAndASPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4760_mp_reach_test.go#L88). **negative:** `unit/verify` [`TestRFC4760MPReachUpdateCarriesOriginASPathAndIBGPLocalPref`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4760_mp_reach_mandatory_send_test.go#L24) |
| `RFC4760-3-4` | Moreover, in IBGP exchanges such a message MUST also carry the LOCAL_PREF attribute. (Section 3) | MUST | 3 - Multiprotocol Reachable NLRI - MP_REACH_NLRI (Type Code 14) | **positive:** `unit/verify` [`TestRFC4760IBGPMPReachCarriesLocalPref`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4760_mp_reach_test.go#L149). **positive:** `unit/verify` [`TestRFC4760MPReachUpdateCarriesOriginASPathAndIBGPLocalPref`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4760_mp_reach_mandatory_send_test.go#L26). **negative:** `unit/verify` [`TestRFC4760IBGPMPReachCarriesLocalPref`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4760_mp_reach_test.go#L153). **negative:** `unit/verify` [`TestRFC4760MPReachUpdateCarriesOriginASPathAndIBGPLocalPref`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4760_mp_reach_mandatory_send_test.go#L28) |
| `RFC4760-7-1` | If a BGP speaker receives from a neighbor an UPDATE message that contains the MP_REACH_NLRI or MP_UNREACH_NLRI attribute, and if the speaker determines that the attribute is incorrect, the speaker MUST delete all the BGP routes received from that neighbor whose AFI/SAFI is the same as the one carried in the incorrect MP_REACH_NLRI or MP_UNREACH_NLRI attribute. (Section 7) | MUST | 7 - Error Handling | **positive:** `unit/verify` [`TestHandleState_PeerDown`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_test.go#L311). **positive:** `unit/verify` [`TestRFC4760IncorrectMPAttributeResetsTheSession`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4760_mp_reach_test.go#L240). **positive:** `unit/verify` [`TestRFC4760IncorrectMPReachDeletesTheNeighborsRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4760_section7_test.go#L102). **positive:** `unit/verify` [`TestRFC4760IncorrectMPReachRaisesThePeerDownThatClearsTheRIB`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4760_peer_down_link_test.go#L214). **positive:** `unit/verify` [`TestRFC4760IncorrectMPUnreachDeletesTheNeighborsRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4760_section7_test.go#L139). **positive:** `unit/verify` [`TestRFC4760PeerDownDeletesOnlyThatNeighborsRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4760_peer_down_test.go#L48). **negative:** `unit/verify` [`TestRFC4760CorrectMPReachKeepsTheNeighborsRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4760_section7_test.go#L171). **negative:** `unit/verify` [`TestRFC4760CorrectMPReachRaisesNoPeerDown`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4760_peer_down_link_test.go#L256). **negative:** `unit/verify` [`TestRFC4760IncorrectMPAttributeResetsTheSession`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4760_mp_reach_test.go#L244) |
| `RFC4760-8-1` | To have a bi-directional exchange of routing information for a particular <AFI, SAFI> between a pair of BGP speakers, each such speaker MUST advertise to the other (via the Capability Advertisement mechanism) the capability to support that particular <AFI, SAFI> route. (Section 8) | MUST | 8 - Use of BGP Capability Advertisement | **positive:** `unit/verify` [`TestParseCapabilities`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L66). **positive:** `unit/verify` [`TestRFC4760OpenAdvertisesEveryConfiguredFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc_open_family_capability_test.go#L88). **negative:** `unit/verify` [`TestRFC4760FamilyNotAdvertisedIsNotExchanged`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc_open_family_capability_test.go#L111) |
| `RFC4760-3-5` | A 1 octet field that MUST be set to 0, and SHOULD be ignored upon receipt. (Section 3) | SHOULD | 3 - Multiprotocol Reachable NLRI - MP_REACH_NLRI (Type Code 14) | **positive:** no positive test. **negative:** no negative test |
| `RFC4760-3-6` | The next hop information carried in the MP_REACH_NLRI path attribute defines the Network Layer address of the router that SHOULD be used as the next hop to the destinations listed in the MP_NLRI attribute in the UPDATE message. (Section 3) | SHOULD | 3 - Multiprotocol Reachable NLRI - MP_REACH_NLRI (Type Code 14) | **positive:** no positive test. **negative:** no negative test |
| `RFC4760-3-7` | An UPDATE message that carries no NLRI, other than the one encoded in the MP_REACH_NLRI attribute, SHOULD NOT carry the NEXT_HOP attribute. (Section 3) | SHOULD NOT | 3 - Multiprotocol Reachable NLRI - MP_REACH_NLRI (Type Code 14) | **positive:** no positive test. **negative:** no negative test |
| `RFC4760-3-8` | An UPDATE message that carries no NLRI, other than the one encoded in the MP_REACH_NLRI attribute, SHOULD NOT carry the NEXT_HOP attribute. If such a message contains the NEXT_HOP attribute, the BGP speaker that receives the message SHOULD ignore this attribute. (Section 3) | SHOULD | 3 - Multiprotocol Reachable NLRI - MP_REACH_NLRI (Type Code 14) | **positive:** no positive test. **negative:** no negative test |
| `RFC4760-3-9` | An UPDATE message SHOULD NOT include the same address prefix (of the same <AFI, SAFI>) in more than one of the following fields: WITHDRAWN ROUTES field, Network Reachability Information fields, MP_REACH_NLRI field, and MP_UNREACH_NLRI field. (Section 3) | SHOULD NOT | 3 - Multiprotocol Reachable NLRI - MP_REACH_NLRI (Type Code 14) | **positive:** no positive test. **negative:** no negative test |
| `RFC4760-7-2` | For the duration of the BGP session over which the UPDATE message was received, the speaker then SHOULD ignore all the subsequent routes with that AFI/SAFI received over that session. (Section 7) | SHOULD | 7 - Error Handling | **positive:** no positive test. **negative:** no negative test |
| `RFC4760-7-3` | The session SHOULD be terminated with the Notification message code/subcode indicating "UPDATE Message Error"/"Optional Attribute Error". (Section 7) | SHOULD | 7 - Error Handling | **positive:** no positive test. **negative:** no negative test |
| `RFC4760-8-2` | A BGP speaker that uses Multiprotocol Extensions SHOULD use the Capability Advertisement procedures [BGP-CAP] to determine whether the speaker could use Multiprotocol Extensions with a particular peer. (Section 8) | SHOULD | 8 - Use of BGP Capability Advertisement | **positive:** `unit/verify` [`TestMUPSessionNegotiatesBothAFIs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/rfc4760_mup_session_test.go#L38). **negative:** `unit/verify` [`TestMUPSessionMissingAFIIsNotNegotiated`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/rfc4760_mup_session_test.go#L69) |
| `RFC4760-8-3` | Reserved (8 bit) field. SHOULD be set to 0 by the sender and ignored by the receiver. (Section 8) | SHOULD | 8 - Use of BGP Capability Advertisement | **positive:** no positive test. **negative:** no negative test |
| `RFC4760-7-4` | In addition, the speaker MAY terminate the BGP session over which the UPDATE message was received. (Section 7) | MAY | 7 - Error Handling | **positive:** no positive test. **negative:** no negative test |
| `RFC4760-6-1` | An implementation MAY support all, some, or none of the Subsequent Address Family Identifier values defined in this document. (Section 6) | MAY | 6 - Subsequent Address Family Identifier | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

RFC 4760 declares no gap, and every gated MUST it carries has a test bound to it.

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC4760-3-1`](#rfc4760-3-1)

A 1 octet field that MUST be set to 0 (Section 3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Clause "MUST be set to 0": (a) forbidden: an MP_REACH_NLRI whose Reserved octet is non-zero on send, e.g. inherited from a pooled buffer. (b) rfc4760_reserved_test.go TestRFC4760ReservedIsWrittenNotInherited asserts buf[reserved]==0 over a 0xAA-poisoned buffer at a non-zero offset for six next-hop forms, and TestRFC4760ReservedSurvivesBufferReuse asserts 0 where the previous encode left 0x11; both go red if MPReachNLRI.WriteTo stops writing the octet. Single-polarity marker on the row: there is no non-zero form Ze could emit to reject.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestMPReachNLRI_WriteTo`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/mpnlri_test.go#L12) | unit/verify | unproven |
| positive | [`TestRFC4760ReservedIsWrittenNotInherited`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc4760_reserved_test.go#L37) | unit/verify | unproven |
| positive | [`TestRFC4760ReservedSurvivesBufferReuse`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc4760_reserved_test.go#L122) | unit/verify | unproven |

### [`RFC4760-3-2`](#rfc4760-3-2)

If the Next Hop is allowed to be from more than one Network Layer protocol, the encoding of the Next Hop MUST provide a way to determine its Network Layer protocol. (Section 3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. The one multi-protocol case Ze encodes is IPv4 NLRI over an IPv4 or an IPv6 next hop (RFC 5549/8950), plus RD-prefixed VPN hops. (a) forbidden: an encoding whose Length of Next Hop does not tell which protocol the hop is, e.g. an IPv6 hop under AFI 1 written with length 4, or a zero-length field. (b) TestMPReachNextHopLengthCountsTheOctetsWritten asserts buf[3]==16 for "ipv4 unicast over an ipv6 next hop" and 4 for the IPv4 hop, and that the written span equals Len(); negatives: TestMPReachValidateNextHopsRefusesAnAddressWithNoWireForm (ErrUnencodableNextHop, nothing written), TestCommitRefusesAnAnnounceWhoseNextHopHasNoWireForm and TestBuildRIBRouteUpdate_RefusesANextHopWithNoWireForm (no UPDATE on the commit and RIB-replay rails), TestParseMPReachNLRI_InvalidNextHopLength (length 5 refused). Tag-prose note: TestParseMPReachNLRI claims the parser derives the family from the length, but its cases are AFI 2 only and assert counts, not the family.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestBuildRIBRouteUpdate_RefusesANextHopWithNoWireForm`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4760_peer_rib_routes_nexthop_test.go#L50) | unit/verify | revert, verified |
| negative | [`TestCommitRefusesAnAnnounceWhoseNextHopHasNoWireForm`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/rib/rfc4760_commit_nexthop_test.go#L69) | unit/verify | revert, verified |
| negative | [`TestParseMPReachNLRI_InvalidNextHopLength`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/mpnlri_test.go#L498) | unit/verify | unproven |
| negative | [`TestMPReachValidateNextHopsRefusesAnAddressWithNoWireForm`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc4760_mpnlri_nexthop_wire_test.go#L124) | unit/verify | unproven |
| positive | [`TestCommitVPNAnnounceCarriesTheRFC4364NextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/rib/rfc4760_commit_nexthop_test.go#L155) | unit/verify | revert, verified |
| positive | [`TestMPReachNLRI_WriteTo`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/mpnlri_test.go#L16) | unit/verify | unproven |
| positive | [`TestParseMPReachNLRI`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/mpnlri_test.go#L128) | unit/verify | unproven |
| positive | [`TestMPReachNextHopLengthCountsTheOctetsWritten`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc4760_mpnlri_nexthop_wire_test.go#L39) | unit/verify | unproven |

### [`RFC4760-3-3`](#rfc4760-3-3)

An UPDATE message that carries the MP_REACH_NLRI MUST also carry the ORIGIN and the AS_PATH attributes (both in EBGP and in IBGP exchanges). (Section 3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-30 after the R43 signature change (assertions unchanged, re-recorded red). TestRFC4760MPReachUpdateCarriesOriginASPathAndIBGPLocalPref: BuildUnicast of an IPv6 route stating ORIGIN, and a bare route stating neither ORIGIN nor AS_PATH, on eBGP and iBGP, always carries MP_REACH_NLRI with ORIGIN and AS_PATH (genuine negative, R1b). BuildUnicast rail; CommitService/RIB-replay rails not driven.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4760MPReachUpdateCarriesOriginASPathAndIBGPLocalPref`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4760_mp_reach_mandatory_send_test.go#L24) | unit/verify | revert, verified |
| negative | [`TestRFC4760MPReachRequiresOriginAndASPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4760_mp_reach_test.go#L88) | unit/verify | unproven |
| positive | [`TestRFC4760MPReachUpdateCarriesOriginASPathAndIBGPLocalPref`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4760_mp_reach_mandatory_send_test.go#L22) | unit/verify | revert, verified |
| positive | [`TestRFC4760MPReachRequiresOriginAndASPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4760_mp_reach_test.go#L85) | unit/verify | unproven |

### [`RFC4760-3-4`](#rfc4760-3-4)

Moreover, in IBGP exchanges such a message MUST also carry the LOCAL_PREF attribute. (Section 3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-30 after the R43 signature change (assertions unchanged, re-recorded red). Same unit: every iBGP MP_REACH UPDATE carries LOCAL_PREF, including the bare route with no LOCAL_PREF value (genuine negative); the HEAD eBGP-omits unit stays as a neighbour. BuildUnicast rail.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4760MPReachUpdateCarriesOriginASPathAndIBGPLocalPref`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4760_mp_reach_mandatory_send_test.go#L28) | unit/verify | revert, verified |
| negative | [`TestRFC4760IBGPMPReachCarriesLocalPref`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4760_mp_reach_test.go#L153) | unit/verify | unproven |
| positive | [`TestRFC4760MPReachUpdateCarriesOriginASPathAndIBGPLocalPref`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4760_mp_reach_mandatory_send_test.go#L26) | unit/verify | revert, verified |
| positive | [`TestRFC4760IBGPMPReachCarriesLocalPref`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4760_mp_reach_test.go#L149) | unit/verify | unproven |

### [`RFC4760-7-1`](#rfc4760-7-1)

If a BGP speaker receives from a neighbor an UPDATE message that contains the MP_REACH_NLRI or MP_UNREACH_NLRI attribute, and if the speaker determines that the attribute is incorrect, the speaker MUST delete all the BGP routes received from that neighbor whose AFI/SAFI is the same as the one carried in the incorrect MP_REACH_NLRI or MP_UNREACH_NLRI attribute. (Section 7)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-read 2026-10-01 (c8 judge): the RIB unit changed only by two added RFC4271-8.2.2-12/-14 tag comments; body and assertions unchanged. The link the earlier weak verdict named is now asserted end to end. Reactor: TestRFC4760IncorrectMPReachRaisesThePeerDownThatClearsTheRIB runs a whole Peer over TCP, sends an MP_REACH_NLRI whose NLRI overruns the attribute, and asserts the run loop reports THAT peer closed to its lifecycle observers (the apiStateObserver feed that becomes the SessionStateDown event), NOTIFICATION code 3 on the wire, and no consumer dispatch; observed red with notifyPeerClosed broken. Negative TestRFC4760CorrectMPReachRaisesNoPeerDown: the well-formed attribute is dispatched, no peer-down, still Established (red with enforceRFC7606 broken). RIB: TestRFC4760PeerDownDeletesOnlyThatNeighborsRoutes drives handleStructuredState down and asserts the offender's Adj-RIB-In is gone while a bystander's IPv4 unicast route stays (red with handleStructuredState broken); TestHandleState_PeerDown covers the JSON rail. Session and message units pin the SessionReset trigger for bad MP_REACH next-hop length and short MP_UNREACH. Ze deletes all AFI/SAFIs of the neighbor via session reset (RFC 7606 3(j)), a superset of the MUST. Not asserted: the plugin dispatcher's translation of OnPeerStateChange into the RIB's structured state event, which is the shared peer-state path every plugin uses.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4760IncorrectMPAttributeResetsTheSession`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4760_mp_reach_test.go#L244) | unit/verify | unproven |
| negative | [`TestRFC4760CorrectMPReachRaisesNoPeerDown`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4760_peer_down_link_test.go#L256) | unit/verify | revert, verified |
| negative | [`TestRFC4760CorrectMPReachKeepsTheNeighborsRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4760_section7_test.go#L171) | unit/verify | unproven |
| positive | [`TestRFC4760IncorrectMPAttributeResetsTheSession`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4760_mp_reach_test.go#L240) | unit/verify | unproven |
| positive | [`TestRFC4760PeerDownDeletesOnlyThatNeighborsRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4760_peer_down_test.go#L48) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| positive | [`TestHandleState_PeerDown`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_test.go#L311) | unit/verify | unproven |
| positive | [`TestRFC4760IncorrectMPReachRaisesThePeerDownThatClearsTheRIB`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4760_peer_down_link_test.go#L214) | unit/verify | revert, verified |
| positive | [`TestRFC4760IncorrectMPReachDeletesTheNeighborsRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4760_section7_test.go#L102) | unit/verify | unproven |
| positive | [`TestRFC4760IncorrectMPUnreachDeletesTheNeighborsRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4760_section7_test.go#L139) | unit/verify | unproven |

### [`RFC4760-8-1`](#rfc4760-8-1)

To have a bi-directional exchange of routing information for a particular <AFI, SAFI> between a pair of BGP speakers, each such speaker MUST advertise to the other (via the Capability Advertisement mechanism) the capability to support that particular <AFI, SAFI> route. (Section 8)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Now reads Ze's own built OPEN (parsePeerFromTree -> buildOpen -> ParseFromOptionalParams). Positive: three configured families each get one Multiprotocol capability, all negotiated against a peer offering them. Negative: ipv6/unicast mode disable -> no MP capability for it, and Negotiate against a peer offering it leaves it unsupported, so no exchange without Ze's advertisement. Judge break: the disabled-family skip in parseFamiliesFromTree removed turned the negative red. Old parse-level TestParseCapabilities tag is supplementary. Single-polarity marker removal is correct.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4760FamilyNotAdvertisedIsNotExchanged`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc_open_family_capability_test.go#L111) | unit/verify | revert, verified |
| positive | [`TestRFC4760OpenAdvertisesEveryConfiguredFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc_open_family_capability_test.go#L88) | unit/verify | revert, verified |
| positive | [`TestParseCapabilities`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L66) | unit/verify | unproven |

### [`RFC4760-8-2`](#rfc4760-8-2)

A BGP speaker that uses Multiprotocol Extensions SHOULD use the Capability Advertisement procedures [BGP-CAP] to determine whether the speaker could use Multiprotocol Extensions with a particular peer. (Section 8)

Audit verdict: enforced (the tests do what the requirement demands), fresh. capability.Negotiate is the determination the sentence names: TestMUPSessionNegotiatesBothAFIs shows both families usable when both speakers advertise MP(1,85) and MP(2,85); TestMUPSessionMissingAFIIsNotNegotiated shows a family the peer did not advertise is not usable (v4-only peer drops ipv6/mup, no-MUP peer drops both). Exercised for SAFI 85 only; the gating of sends and receives on SupportsFamily is outside these units. Both records red on Negotiate

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestMUPSessionMissingAFIIsNotNegotiated`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/rfc4760_mup_session_test.go#L69) | unit/verify | revert, verified |
| positive | [`TestMUPSessionNegotiatesBothAFIs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/rfc4760_mup_session_test.go#L38) | unit/verify | revert, verified |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | ze-work agent, spec-fixit-rfc-drain-quota-never-armed WP-1 |
| Signed off | 2026-08-31 |
| Register | rfc2119 |
| Source | rfc/full/rfc4760.txt |
| Source fingerprint | 7b28975d269770a5 |
| Record | rfc/extraction/rfc4760.json |
| Mapped sentences | 6 |
| Declined as scope | 3 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | skipped (front-matter) | Title block, Status of This Memo, Copyright Notice and Abstract. The Abstract states that the document extends BGP-4 to carry routing information for multiple Network Layer protocols and that the extensions are backward compatible. It binds no speaker. |
| `1` | Introduction | 0 | walked | Introduction. Indicative prose naming the three IPv4-specific pieces of BGP-4 (NEXT_HOP, AGGREGATOR and NLRI) and the two attributes this document adds, both optional and non-transitive so a speaker without the extension ignores them. Its only modal is the lowercase 'should' of 'the advertisement of reachable destinations should be grouped with ... the next hop', which describes the design rationale for MP_REACH_NLRI rather than directing a speaker. Every obligation it foreshadows is stated normatively in sections 3, 7 and 8. |
| `2` | not stated | 0 | walked | Specification of Requirements: the RFC 2119 key-words paragraph. It tells a reader how to read the other sections and binds no speaker, which is why the derivation excludes it from the site inventory. |
| `3` | Multiprotocol Reachable NLRI - MP_REACH_NLRI (Type Code 14) | 5 | walked | Multiprotocol Reachable NLRI - MP_REACH_NLRI (Type Code 14). The wire format, the per-field semantics, and the attribute-companion rules. Five derived sites: 3:1 and 3:2 are the same Next Hop encoding sentence repeated under the AFI and SAFI field headings, 3:3 is the Reserved octet, and 3:4 and 3:5 are the ORIGIN/AS_PATH and LOCAL_PREF companions. The section's five advisory rows are listed below: each is a SHOULD or SHOULD NOT sentence the MUST-level site inventory cannot see, and RFC4760-3-5 shares its sentence with site 3:3. |
| `4` | not stated | 2 | walked | Multiprotocol Unreachable NLRI - MP_UNREACH_NLRI (Type Code 15). The attribute carries AFI, SAFI and Withdrawn Routes only. Both derived sites are the AFI and SAFI field paragraphs copied from section 3, Next Hop sentence included, which is the copy verified errata 1573 reports: MP_UNREACH_NLRI has no Next Hop field. The section adds one sentence of its own, 'An UPDATE message that contains the MP_UNREACH_NLRI is not required to carry any other path attributes', which relaxes section 3 rather than obliging anyone. The summary declares no id from this section. |
| `5` | NLRI Encoding | 0 | walked | NLRI Encoding. Defines the <length, prefix> 2-tuple, that a zero length matches all addresses of the family, and that the value of the trailing bits is irrelevant. Descriptive throughout, with no modal of any case, so the summary reads no requirement from it. |
| `6` | Subsequent Address Family Identifier | 0 | walked | Subsequent Address Family Identifier. Assigns SAFI 1 to unicast forwarding and SAFI 2 to multicast forwarding, then states one MAY. A value assignment carries no obligation, and the MAY is advisory so the MUST-level inventory cannot see it. |
| `7` | Error Handling | 1 | walked | Error Handling. Its one MUST is site 7:1. The other three sentences are the SHOULD to ignore subsequent routes of that AFI/SAFI, the MAY to terminate the session, and the SHOULD that fixes the Notification code/subcode when it is terminated. All three are advisory and are listed below. |
| `8` | Use of BGP Capability Advertisement | 1 | walked | Use of BGP Capability Advertisement. Its one MUST is site 8:1. The opening SHOULD to use Capability Advertisement and the Res. field's SHOULD are advisory and are listed below. The Capability Code 1 and Capability Length 4 sentences are value assignments, and 'A speaker that supports multiple <AFI, SAFI> tuples includes them as multiple Capabilities' is indicative prose the summary captures as wire format rather than as a requirement row. |
| `9` | IANA Considerations | 0 | skipped (iana) | IANA Considerations. Defines the SAFI name space: 1 and 2 assigned, 3 deprecated, the allocation policy for 5-63 and 67-127, and the reserved and private-use ranges. Binds IANA, not a speaker. |
| `10` | Comparison with RFC 2858 | 0 | walked | Comparison with RFC 2858. A change log against the obsoleted document: next hop use made consistent with NEXT_HOP, SAFI 3 deprecated, the SAFI partitioning changed, and Number of SNPAs renamed to Reserved. Its one lowercase 'should be considered reserved' restates the IANA action of section 9 and binds IANA. |
| `11` | Comparison with RFC 2283 | 0 | walked | Comparison with RFC 2283. A change log: one instance per attribute, the no-NLRI clarification, the error-handling clarification, and the addition of Capability Advertisement. Every item it names is stated normatively in sections 3, 7 or 8 and is captured there. |
| `12` | Security Considerations | 0 | walked | Security Considerations. One sentence: the extension does not change the security issues inherent in existing BGP. No countermeasure is directed at a speaker. |
| `13` | Acknowledgements: the IDR Working Group | 0 | skipped (acknowledgements) | Acknowledgements: the IDR Working Group. |
| `14` | not stated | 0 | skipped (references) | Normative References: RFC 3392, RFC 4271, IANA Address Family Numbers, RFC 2119, RFC 2434, RFC 4020. The section also absorbs the Authors' Addresses block, the Full Copyright Statement and the Intellectual Property notice, none of which binds a speaker. |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `3:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The identical sentence, repeated word for word under the Subsequent Address Family Identifier field heading because AFI and SAFI are described as a pair. Site 3:1 maps RFC4760-3-2; this is the same obligation stated once more, not a second one. | If the Next Hop is allowed to be from more than one Network Layer protocol, the encoding of the Next Hop MUST provide a way to determine its Network Layer protocol. |
| `4:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The section 3 AFI paragraph copied into MP_UNREACH_NLRI, Next Hop sentence included. Verified errata 1573 records that this text was copied from MP_REACH_NLRI without adjustment and that MP_UNREACH_NLRI carries no Next Hop field, so the sentence states no obligation here beyond the one site 3:1 maps as RFC4760-3-2. | If the Next Hop is allowed to be from more than one Network Layer protocol, the encoding of the Next Hop MUST provide a way to determine its Network Layer protocol. |
| `4:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The same copy again, under the SAFI field heading of MP_UNREACH_NLRI. Fourth occurrence of one sentence; site 3:1 maps it as RFC4760-3-2. | If the Next Hop is allowed to be from more than one Network Layer protocol, the encoding of the Next Hop MUST provide a way to determine its Network Layer protocol. |

## Superseded

No document obsoletes RFC 4760, so its obligations are stated where they were written.
