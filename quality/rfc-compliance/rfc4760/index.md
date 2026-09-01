# RFC 4760 - Multiprotocol Extensions for BGP-4

Supported. Every requirement this repository extracted from RFC 4760, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 66.7% | 4 of 6 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 33.3% | 2 of 6 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 6 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 0.0% | 0 of 6 binding obligations | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 0.0% | 0 of 22 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 6 | of 17 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 6 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

The 4 shares marked as a part above are the whole of the 6 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| No test at all | ok | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Supported |
| Enrolment | Enrolled |
| Requirements | 17 |
| Gated MUST-level | 6 |
| Obligations that bind Ze | 6 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 22 |
| Tagged units | 22 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc4760.md` |
| Requirement shard | `rfc/requirements/rfc4760.md` |
| RFC text | `rfc/full/rfc4760.txt` |

## Enrolment

Enrolled: Multiprotocol Extensions for BGP-4: six MUST-level requirements. Five are met: 3-2 (the next-hop length determines the next-hop protocol) carries positive+negative tags; 3-3 (an UPDATE with MP_REACH_NLRI also carries ORIGIN and AS_PATH) and 3-4 (an iBGP UPDATE carrying MP_REACH includes LOCAL_PREF) carry positive+negative tags on new internal/component/bgp/message tests; 3-1 (the MP_REACH Reserved octet is 0) and 8-1 (advertise the Multiprotocol capability) are {single-polarity: positive}. 7-1 (Section 7 bulk per-AFI/SAFI route deletion) is {not-applicable}: ze supersedes it with RFC 7606 revised error handling (treat-as-withdraw per NLRI or session reset).

## What the public ledger says

**Status:** Supported

**What the ledger says is covered:**

AFI/SAFI capability negotiation, MP_REACH_NLRI, MP_UNREACH_NLRI, family-specific UPDATE handling.

**What the ledger says remains:**

RFC 7606 MP attribute ordering tradeoff is tracked under RFC 7606.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 4 | one part of the gated population |
| Annotated instead of tested | 2 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **6** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (4):** [`RFC4760-3-2`](#rfc4760-3-2), [`RFC4760-3-3`](#rfc4760-3-3), [`RFC4760-3-4`](#rfc4760-3-4), [`RFC4760-7-1`](#rfc4760-7-1)

**Annotated instead of tested (2):** [`RFC4760-3-1`](#rfc4760-3-1), [`RFC4760-8-1`](#rfc4760-8-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC4760-3-1` | Reserved field in MP_REACH_NLRI MUST be set to 0 (Section 3) | MUST | 3 - Multiprotocol Reachable NLRI - MP_REACH_NLRI (Type Code 14) | **positive:** `unit/verify` [`TestMPReachNLRI_WriteTo`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/mpnlri_test.go#L12). **positive:** `unit/verify` [`TestRFC4760ReservedIsWrittenNotInherited`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc4760_reserved_test.go#L37). **positive:** `unit/verify` [`TestRFC4760ReservedSurvivesBufferReuse`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc4760_reserved_test.go#L122). **negative:** no negative test. **{single-polarity}:** MPReachNLRI.WriteTo writes the Reserved octet as 0 unconditionally, so there is no non-zero form to reject (internal/core/bgp/attribute/mpnlri.go:182) |
| `RFC4760-3-2` | If Next Hop is allowed to be from more than one Network Layer protocol, encoding of the Next Hop MUST provide a way to determine its Network Layer protocol (Section 3) | MUST | 3 - Multiprotocol Reachable NLRI - MP_REACH_NLRI (Type Code 14) | **positive:** `unit/verify` [`TestCommitVPNAnnounceCarriesTheRFC4364NextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/rib/commit_nexthop_test.go#L155). **positive:** `unit/verify` [`TestMPReachNLRI_WriteTo`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/mpnlri_test.go#L16). **positive:** `unit/verify` [`TestMPReachNextHopLengthCountsTheOctetsWritten`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/mpnlri_nexthop_wire_test.go#L39). **positive:** `unit/verify` [`TestParseMPReachNLRI`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/mpnlri_test.go#L128). **negative:** `unit/verify` [`TestBuildRIBRouteUpdate_RefusesANextHopWithNoWireForm`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_rib_routes_nexthop_test.go#L50). **negative:** `unit/verify` [`TestCommitRefusesAnAnnounceWhoseNextHopHasNoWireForm`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/rib/commit_nexthop_test.go#L69). **negative:** `unit/verify` [`TestMPReachValidateNextHopsRefusesAnAddressWithNoWireForm`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/mpnlri_nexthop_wire_test.go#L124). **negative:** `unit/verify` [`TestParseMPReachNLRI_InvalidNextHopLength`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/mpnlri_test.go#L498) |
| `RFC4760-3-3` | An UPDATE message carrying MP_REACH_NLRI MUST also carry the ORIGIN and AS_PATH attributes (Section 3) | MUST | 3 - Multiprotocol Reachable NLRI - MP_REACH_NLRI (Type Code 14) | **positive:** `unit/verify` [`TestRFC4760MPReachRequiresOriginAndASPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4760_mp_reach_test.go#L85). **negative:** `unit/verify` [`TestRFC4760MPReachRequiresOriginAndASPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4760_mp_reach_test.go#L88) |
| `RFC4760-3-4` | In IBGP exchanges, an UPDATE with MP_REACH_NLRI MUST also carry the LOCAL_PREF attribute (Section 3) | MUST | 3 - Multiprotocol Reachable NLRI - MP_REACH_NLRI (Type Code 14) | **positive:** `unit/verify` [`TestRFC4760IBGPMPReachCarriesLocalPref`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4760_mp_reach_test.go#L149). **negative:** `unit/verify` [`TestRFC4760IBGPMPReachCarriesLocalPref`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4760_mp_reach_test.go#L153) |
| `RFC4760-7-1` | On incorrect MP_REACH_NLRI or MP_UNREACH_NLRI, the speaker MUST delete all BGP routes from that neighbor for that AFI/SAFI (Section 7). RFC 7606 does not retire this: Section 3 clause (j) says that when the MP attribute cannot be parsed, "the procedures of [RFC4271] and/or [RFC4760] continue to apply, meaning that the 'session reset' approach (or the 'AFI/SAFI disable' approach) MUST be followed". Ze follows session reset, the stronger of the two, which drops every route from that neighbor and so a superset of that AFI/SAFI's routes. ValidateUpdateRFC7606 (internal/component/bgp/message/rfc7606.go) returns RFC7606ActionSessionReset for an unparseable MP_REACH_NLRI or MP_UNREACH_NLRI, and rfc7606SessionReset (internal/component/bgp/reactor/session_validation.go) sends the NOTIFICATION and closes the connection. | MUST | 7 - Error Handling | **positive:** `unit/verify` [`TestHandleState_PeerDown`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_test.go#L298). **positive:** `unit/verify` [`TestRFC4760IncorrectMPAttributeResetsTheSession`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4760_mp_reach_test.go#L240). **positive:** `unit/verify` [`TestRFC4760IncorrectMPReachDeletesTheNeighborsRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4760_section7_test.go#L102). **positive:** `unit/verify` [`TestRFC4760IncorrectMPUnreachDeletesTheNeighborsRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4760_section7_test.go#L139). **negative:** `unit/verify` [`TestRFC4760CorrectMPReachKeepsTheNeighborsRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4760_section7_test.go#L171). **negative:** `unit/verify` [`TestRFC4760IncorrectMPAttributeResetsTheSession`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4760_mp_reach_test.go#L244) |
| `RFC4760-8-1` | For bi-directional exchange of routing information for a particular AFI/SAFI, each speaker MUST advertise the capability to support that AFI/SAFI via Capability Advertisement (Section 8) | MUST | 8 - Use of BGP Capability Advertisement | **positive:** `unit/verify` [`TestParseCapabilities`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L65). **negative:** no negative test. **{single-polarity}:** the obligation is to advertise a well-formed Multiprotocol capability per AFI/SAFI, enforced by the fixed 4-octet AFI/Reserved/SAFI wire form (internal/core/bgp/capability/capability.go:299-318); there is no malformed-advertisement counter-case distinct from RFC 5492 TLV framing |
| `RFC4760-3-5` | Reserved field in MP_REACH_NLRI SHOULD be ignored upon receipt (Section 3) | SHOULD | 3 - Multiprotocol Reachable NLRI - MP_REACH_NLRI (Type Code 14) | **positive:** no positive test. **negative:** no negative test |
| `RFC4760-3-6` | The next hop in MP_REACH_NLRI SHOULD be used as the next hop to listed destinations (Section 3) | SHOULD | 3 - Multiprotocol Reachable NLRI - MP_REACH_NLRI (Type Code 14) | **positive:** no positive test. **negative:** no negative test |
| `RFC4760-3-7` | An UPDATE carrying no NLRI other than MP_REACH_NLRI SHOULD NOT carry the NEXT_HOP attribute (Section 3) | SHOULD NOT | 3 - Multiprotocol Reachable NLRI - MP_REACH_NLRI (Type Code 14) | **positive:** no positive test. **negative:** no negative test |
| `RFC4760-3-8` | If such a message contains NEXT_HOP, the receiver SHOULD ignore this attribute (Section 3) | SHOULD | 3 - Multiprotocol Reachable NLRI - MP_REACH_NLRI (Type Code 14) | **positive:** no positive test. **negative:** no negative test |
| `RFC4760-3-9` | An UPDATE SHOULD NOT include the same address prefix in more than one of WITHDRAWN ROUTES, NLRI, MP_REACH_NLRI, and MP_UNREACH_NLRI fields (Section 3) | SHOULD NOT | 3 - Multiprotocol Reachable NLRI - MP_REACH_NLRI (Type Code 14) | **positive:** no positive test. **negative:** no negative test |
| `RFC4760-7-2` | After deleting routes for an incorrect attribute, speaker SHOULD ignore all subsequent routes with that AFI/SAFI for the session (Section 7) | SHOULD | 7 - Error Handling | **positive:** no positive test. **negative:** no negative test |
| `RFC4760-7-3` | Session SHOULD be terminated with UPDATE Message Error / Optional Attribute Error on incorrect attribute (Section 7) | SHOULD | 7 - Error Handling | **positive:** no positive test. **negative:** no negative test |
| `RFC4760-8-2` | A BGP speaker using Multiprotocol Extensions SHOULD use Capability Advertisement to determine support with a peer (Section 8) | SHOULD | 8 - Use of BGP Capability Advertisement | **positive:** no positive test. **negative:** no negative test |
| `RFC4760-8-3` | Reserved field in Multiprotocol Capability SHOULD be set to 0 by sender and ignored by receiver (Section 8) | SHOULD | 8 - Use of BGP Capability Advertisement | **positive:** no positive test. **negative:** no negative test |
| `RFC4760-7-4` | The speaker MAY terminate the BGP session on incorrect MP_REACH_NLRI or MP_UNREACH_NLRI (Section 7) | MAY | 7 - Error Handling | **positive:** no positive test. **negative:** no negative test |
| `RFC4760-6-1` | An implementation MAY support all, some, or none of the SAFI values defined in this document (Section 6) | MAY | 6 - Subsequent Address Family Identifier | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

RFC 4760 declares no gap, and every gated MUST it carries has a test bound to it.

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC4760-3-1`](#rfc4760-3-1)

Reserved field in MP_REACH_NLRI MUST be set to 0 (Section 3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestMPReachNLRI_WriteTo`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/mpnlri_test.go#L12) | unit/verify | unproven |
| positive | [`TestRFC4760ReservedIsWrittenNotInherited`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc4760_reserved_test.go#L37) | unit/verify | unproven |
| positive | [`TestRFC4760ReservedSurvivesBufferReuse`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc4760_reserved_test.go#L122) | unit/verify | unproven |

### [`RFC4760-3-2`](#rfc4760-3-2)

If Next Hop is allowed to be from more than one Network Layer protocol, encoding of the Next Hop MUST provide a way to determine its Network Layer protocol (Section 3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestBuildRIBRouteUpdate_RefusesANextHopWithNoWireForm`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_rib_routes_nexthop_test.go#L50) | unit/verify | unproven |
| negative | [`TestCommitRefusesAnAnnounceWhoseNextHopHasNoWireForm`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/rib/commit_nexthop_test.go#L69) | unit/verify | unproven |
| negative | [`TestMPReachValidateNextHopsRefusesAnAddressWithNoWireForm`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/mpnlri_nexthop_wire_test.go#L124) | unit/verify | unproven |
| negative | [`TestParseMPReachNLRI_InvalidNextHopLength`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/mpnlri_test.go#L498) | unit/verify | unproven |
| positive | [`TestCommitVPNAnnounceCarriesTheRFC4364NextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/rib/commit_nexthop_test.go#L155) | unit/verify | unproven |
| positive | [`TestMPReachNextHopLengthCountsTheOctetsWritten`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/mpnlri_nexthop_wire_test.go#L39) | unit/verify | unproven |
| positive | [`TestMPReachNLRI_WriteTo`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/mpnlri_test.go#L16) | unit/verify | unproven |
| positive | [`TestParseMPReachNLRI`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/mpnlri_test.go#L128) | unit/verify | unproven |

### [`RFC4760-3-3`](#rfc4760-3-3)

An UPDATE message carrying MP_REACH_NLRI MUST also carry the ORIGIN and AS_PATH attributes (Section 3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4760MPReachRequiresOriginAndASPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4760_mp_reach_test.go#L88) | unit/verify | unproven |
| positive | [`TestRFC4760MPReachRequiresOriginAndASPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4760_mp_reach_test.go#L85) | unit/verify | unproven |

### [`RFC4760-3-4`](#rfc4760-3-4)

In IBGP exchanges, an UPDATE with MP_REACH_NLRI MUST also carry the LOCAL_PREF attribute (Section 3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4760IBGPMPReachCarriesLocalPref`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4760_mp_reach_test.go#L153) | unit/verify | unproven |
| positive | [`TestRFC4760IBGPMPReachCarriesLocalPref`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4760_mp_reach_test.go#L149) | unit/verify | unproven |

### [`RFC4760-7-1`](#rfc4760-7-1)

On incorrect MP_REACH_NLRI or MP_UNREACH_NLRI, the speaker MUST delete all BGP routes from that neighbor for that AFI/SAFI (Section 7). RFC 7606 does not retire this: Section 3 clause (j) says that when the MP attribute cannot be parsed, "the procedures of [RFC4271] and/or [RFC4760] continue to apply, meaning that the 'session reset' approach (or the 'AFI/SAFI disable' approach) MUST be followed". Ze follows session reset, the stronger of the two, which drops every route from that neighbor and so a superset of that AFI/SAFI's routes. ValidateUpdateRFC7606 (internal/component/bgp/message/rfc7606.go) returns RFC7606ActionSessionReset for an unparseable MP_REACH_NLRI or MP_UNREACH_NLRI, and rfc7606SessionReset (internal/component/bgp/reactor/session_validation.go) sends the NOTIFICATION and closes the connection.

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4760IncorrectMPAttributeResetsTheSession`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4760_mp_reach_test.go#L244) | unit/verify | unproven |
| negative | [`TestRFC4760CorrectMPReachKeepsTheNeighborsRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4760_section7_test.go#L171) | unit/verify | unproven |
| positive | [`TestRFC4760IncorrectMPAttributeResetsTheSession`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc4760_mp_reach_test.go#L240) | unit/verify | unproven |
| positive | [`TestHandleState_PeerDown`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_test.go#L298) | unit/verify | unproven |
| positive | [`TestRFC4760IncorrectMPReachDeletesTheNeighborsRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4760_section7_test.go#L102) | unit/verify | unproven |
| positive | [`TestRFC4760IncorrectMPUnreachDeletesTheNeighborsRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4760_section7_test.go#L139) | unit/verify | unproven |

### [`RFC4760-8-1`](#rfc4760-8-1)

For bi-directional exchange of routing information for a particular AFI/SAFI, each speaker MUST advertise the capability to support that AFI/SAFI via Capability Advertisement (Section 8)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestParseCapabilities`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L65) | unit/verify | unproven |

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
