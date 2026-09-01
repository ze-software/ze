# RFC 5549 - Advertising IPv4 Network Layer Reachability Information with an IPv6 Next Hop

Supported. Every requirement this repository extracted from RFC 5549, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 50.0% | 3 of 6 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 50.0% | 3 of 6 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 6 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 0.0% | 0 of 6 binding obligations | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 0.0% | 0 of 17 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 6 | of 9 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
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
| Requirements | 9 |
| Gated MUST-level | 6 |
| Obligations that bind Ze | 6 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 17 |
| Tagged units | 17 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc5549.md` |
| Requirement shard | `rfc/requirements/rfc5549.md` |
| RFC text | `rfc/full/rfc5549.txt` |

## Enrolment

Enrolled: Advertising IPv4 NLRI with an IPv6 Next Hop (legacy encoding, obsoleted by RFC 8950 which ze implements): six MUST-level requirements, all met by the shared extended-next-hop implementation. 4-1 (cross-family IPv6 next-hop honored only when the extended-next-hop capability is negotiated), 3-1 (a 16- or 32-octet next-hop is decoded as IPv6), and 4-4 (do not use a cross-family next-hop without the negotiated capability) carry positive+negative tags. 4-2 (use the RFC 5492 capability mechanism), 4-3 (capability code 5), and 5-1 (do not rewrite a reflected next-hop) are {single-polarity: positive}. Tags added alongside the sibling RFC 8950 tags on internal/core/bgp/capability, internal/core/bgp/attribute, and internal/component/bgp/reactor tests.

## What the public ledger says

**Status:** Supported

**What the ledger says is covered:**

Backward-compatible parser for the older format superseded by RFC 8950.

**What the ledger says remains:**

Main public claim uses RFC 8950.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 3 | one part of the gated population |
| Annotated instead of tested | 3 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **6** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (3):** [`RFC5549-4-1`](#rfc5549-4-1), [`RFC5549-3-1`](#rfc5549-3-1), [`RFC5549-4-4`](#rfc5549-4-4)

**Annotated instead of tested (3):** [`RFC5549-4-2`](#rfc5549-4-2), [`RFC5549-4-3`](#rfc5549-4-3), [`RFC5549-5-1`](#rfc5549-5-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC5549-4-1` | A BGP speaker MUST only advertise IPv4 or VPN-IPv4 NLRI with an IPv6 Next Hop if it has ascertained via Capability Advertisement that the peer supports Extended Next Hop Encoding for the relevant AFI/SAFI pair (§4) | MUST | 4 - Use of BGP Capability Advertisement | **positive:** `unit/verify` [`TestCanUseNextHopFor_ExtendedNH`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_test.go#L1147). **positive:** `unit/verify` [`TestNegotiateExtendedNextHop`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/negotiated_test.go#L287). **negative:** `unit/verify` [`TestCanUseNextHopFor_CrossFamilyNoCap`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_test.go#L1178). **negative:** `unit/verify` [`TestNegotiateExtendedNextHopMismatch`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/negotiated_test.go#L326) |
| `RFC5549-4-2` | A BGP speaker that wishes to advertise an IPv6 Next Hop for IPv4 or VPN-IPv4 NLRI MUST use the Capability Advertisement procedures defined in RFC 5492 (§4) | MUST | 4 - Use of BGP Capability Advertisement | **positive:** `unit/verify` [`TestExtendedNextHopCapability`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L506). **positive:** `unit/verify` [`TestExtendedNextHopRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L539). **negative:** no negative test. **{single-polarity}:** ze advertises and parses the Extended Next Hop Encoding capability exclusively through the RFC 5492 capability TLV framework (internal/core/bgp/capability/capability.go:644 WriteTo, :667 parseExtendedNextHop). There is no non-RFC-5492 signalling path in ze, so no wrong-procedure case exists to assert as a negative; the peer-support-not-ascertained negative is covered by RFC5549-4-1 |
| `RFC5549-4-3` | The Capability Code field MUST be set to 5 (Extended Next Hop Encoding) (§4) | MUST | 4 - Use of BGP Capability Advertisement | **positive:** `unit/verify` [`TestCapabilityCodeConstants`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L17). **positive:** `unit/verify` [`TestExtendedNextHopCapability`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L509). **positive:** `unit/verify` [`TestExtendedNextHopRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L542). **negative:** no negative test. **{single-polarity}:** the Extended Next Hop Encoding capability code is the fixed constant CodeExtendedNextHop = 5 (internal/core/bgp/capability/capability.go:70); Code() returns it (capability.go:640) and WriteTo emits it (capability.go:646). The code has no alternate-value code path, so there is no wrong-code case to reject as a negative |
| `RFC5549-3-1` | The BGP speaker receiving the advertisement MUST use the Length of Next Hop Address field to determine which network-layer protocol the next hop address belongs to (§3) | MUST | 3 | **positive:** `unit/verify` [`TestParseMPReachNLRI_ExtendedNextHop`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/mpnlri_test.go#L342). **positive:** `unit/verify` [`TestParseMPReachNLRI_ExtendedNextHop_DualStack`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/mpnlri_test.go#L446). **positive:** `unit/verify` [`TestParseMPReachNLRI_ExtendedNextHop_VPN`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/mpnlri_test.go#L400). **negative:** `unit/verify` [`TestParseMPReachNLRI_InvalidNextHopLength`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/mpnlri_test.go#L493) |
| `RFC5549-5-1` | When a next hop address needs to be passed along unchanged (e.g., Route Reflector), its encoding MUST NOT be changed (§5) | MUST NOT | 5 - Operations | **positive:** `unit/verify` [`TestReactorForwardRRPreservesExtendedNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_rr_test.go#L237). **negative:** no negative test. **{single-polarity}:** on the reflection path ze rewrites the next-hop only under an explicit next-hop-self/explicit override (nhMode != nhModeNone); the default nhModeNone leaves the next-hop untouched (internal/component/bgp/reactor/peer_forward_facts.go:226) and the MP re-encode changes an attribute only when the NLRI framing differs between encoding contexts (internal/component/bgp/reactor/forward_body.go:217), so a reflected next-hop is carried verbatim and there is no ze code path that rewrites an unchanged-passthrough next-hop to assert as a negative. The positive is proven byte-identical in TestReactorForwardRRPreservesExtendedNextHop |
| `RFC5549-4-4` | MUST NOT send IPv6 Next Hop for IPv4 NLRI to peers that have not advertised Extended Next Hop Encoding capability (§4, Compatibility) | MUST NOT | 4 - Use of BGP Capability Advertisement | **positive:** `unit/verify` [`TestCanUseNextHopFor_ExtendedNH`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_test.go#L1150). **negative:** `unit/verify` [`TestCanUseNextHopFor_CrossFamilyNoCap`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_test.go#L1181). **negative:** `unit/verify` [`TestCanUseNextHopFor_NilSendCtx`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_test.go#L1202) |
| `RFC5549-5-2` | By default, if a BGP session is running over IPvx, the next hop address SHOULD be specified as an IPvx address (§5) | SHOULD | 5 - Operations | **positive:** no positive test. **negative:** no negative test |
| `RFC5549-4-5` | The Extended Next Hop Encoding capability MAY be dynamically updated through the Dynamic Capability capability (§4) | MAY | 4 - Use of BGP Capability Advertisement | **positive:** no positive test. **negative:** no negative test |
| `RFC5549-5-3` | The default next-hop-address-family behavior may be overridden by policy (§5) | MAY | 5 - Operations | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

RFC 5549 declares no gap, and every gated MUST it carries has a test bound to it.

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC5549-4-1`](#rfc5549-4-1)

A BGP speaker MUST only advertise IPv4 or VPN-IPv4 NLRI with an IPv6 Next Hop if it has ascertained via Capability Advertisement that the peer supports Extended Next Hop Encoding for the relevant AFI/SAFI pair (§4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestCanUseNextHopFor_CrossFamilyNoCap`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_test.go#L1178) | unit/verify | unproven |
| negative | [`TestNegotiateExtendedNextHopMismatch`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/negotiated_test.go#L326) | unit/verify | unproven |
| positive | [`TestCanUseNextHopFor_ExtendedNH`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_test.go#L1147) | unit/verify | unproven |
| positive | [`TestNegotiateExtendedNextHop`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/negotiated_test.go#L287) | unit/verify | unproven |

### [`RFC5549-4-2`](#rfc5549-4-2)

A BGP speaker that wishes to advertise an IPv6 Next Hop for IPv4 or VPN-IPv4 NLRI MUST use the Capability Advertisement procedures defined in RFC 5492 (§4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestExtendedNextHopCapability`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L506) | unit/verify | unproven |
| positive | [`TestExtendedNextHopRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L539) | unit/verify | unproven |

### [`RFC5549-4-3`](#rfc5549-4-3)

The Capability Code field MUST be set to 5 (Extended Next Hop Encoding) (§4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestCapabilityCodeConstants`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L17) | unit/verify | unproven |
| positive | [`TestExtendedNextHopCapability`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L509) | unit/verify | unproven |
| positive | [`TestExtendedNextHopRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L542) | unit/verify | unproven |

### [`RFC5549-3-1`](#rfc5549-3-1)

The BGP speaker receiving the advertisement MUST use the Length of Next Hop Address field to determine which network-layer protocol the next hop address belongs to (§3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestParseMPReachNLRI_InvalidNextHopLength`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/mpnlri_test.go#L493) | unit/verify | unproven |
| positive | [`TestParseMPReachNLRI_ExtendedNextHop`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/mpnlri_test.go#L342) | unit/verify | unproven |
| positive | [`TestParseMPReachNLRI_ExtendedNextHop_DualStack`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/mpnlri_test.go#L446) | unit/verify | unproven |
| positive | [`TestParseMPReachNLRI_ExtendedNextHop_VPN`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/mpnlri_test.go#L400) | unit/verify | unproven |

### [`RFC5549-5-1`](#rfc5549-5-1)

When a next hop address needs to be passed along unchanged (e.g., Route Reflector), its encoding MUST NOT be changed (§5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestReactorForwardRRPreservesExtendedNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_rr_test.go#L237) | unit/verify | unproven |

### [`RFC5549-4-4`](#rfc5549-4-4)

MUST NOT send IPv6 Next Hop for IPv4 NLRI to peers that have not advertised Extended Next Hop Encoding capability (§4, Compatibility)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestCanUseNextHopFor_CrossFamilyNoCap`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_test.go#L1181) | unit/verify | unproven |
| negative | [`TestCanUseNextHopFor_NilSendCtx`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_test.go#L1202) | unit/verify | unproven |
| positive | [`TestCanUseNextHopFor_ExtendedNH`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_test.go#L1150) | unit/verify | unproven |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | ze-work agent, spec-rfcgate-6 phase 5, rfc5549 |
| Signed off | 2026-08-31 |
| Register | rfc2119 |
| Source | rfc/full/rfc5549.txt |
| Source fingerprint | a4896c55c7192c94 |
| Record | rfc/extraction/rfc5549.json |
| Mapped sentences | 5 |
| Declined as scope | 1 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | skipped (front-matter) | Title block, Status of This Memo, Copyright Notice, Abstract and Table of Contents. The Abstract restates the Introduction: MP-BGP lets the AFI/SAFI decide the network-layer protocol of the Next Hop, the IPv4 AFI/SAFI definitions provide only for an IPv4 Next Hop, and this document adds the IPv6 case. It states no obligation. |
| `1` | Introduction | 0 | walked | Introduction. Indicative prose: which AFI/SAFI pairs already let the Next Hop belong to another address family (<25/65> of L2VPN-SIG, <1/132> of RFC 4684), that RFC 4684 already decides the family from the Length of Next Hop Address field, that RFC 4798 and RFC 4659 solve IPv6 NLRI with an IPv4 Next Hop, and that no solution existed for IPv4 NLRI with an IPv6 Next Hop. The last paragraph states what this document adds. No sentence directs a speaker. |
| `2` | Requirements Language | 0 | walked | Requirements Language. The RFC 2119 key-words paragraph. It tells a reader how to read the other sections and binds no speaker, which is why the derivation excludes it from the site inventory. |
| `3` | not stated | 1 | walked | Extension of AFI/SAFI Definitions for the IPv4 Address Family. One capitalised MUST-level site, mapped below to RFC5549-3-1. The rest is value assignment written as a bullet list: AFI 1, SAFI 1, 2, 4 or 128, a Length of Next Hop Address of 16 or 32, and a Next Hop Address that is an IPv6 address optionally followed by the link-local IPv6 address. Those bullets carry no RFC 2119 level, and the Wire Formats and Decoding Rules tables of rfc/short/rfc5549.md hold the values. One bullet delegates rather than obliges: 'This field is to be constructed as per Section 3 of [RFC2545]', which is RFC 2545's obligation and carries its own summary. The closing note that RFC 4684 and L2VPN-SIG use the same length method is a comparison. RFC 8950 Section 3 replaces the 16-or-32 length for SAFI 128 with 24 or 48 and a zero Route Distinguisher; that successor obligation belongs to RFC8950-3-2 and is not stated here, so nothing of it is unsourced against this document. |
| `4` | Use of BGP Capability Advertisement | 4 | walked | Use of BGP Capability Advertisement. Four capitalised MUST-level sites: the RFC 5492 procedures (RFC5549-4-2), the lead-in to the field list (excluded as a duplicate below), Capability Code 5 (RFC5549-4-3), and the precondition that a speaker advertises an IPv6 Next Hop for IPv4 or VPN-IPv4 NLRI only after the Capability Advertisement has shown peer support (RFC5549-4-1). The Capability Value format, its <NLRI AFI, NLRI SAFI, Nexthop AFI> triples and the values this document allows (NLRI AFI 1, NLRI SAFI 1, 2, 4 or 128, Nexthop AFI 2) are value assignment, held by the Wire Formats and Constants tables of rfc/short/rfc5549.md. Three sentences the site scan cannot see are scoping rather than obligations: that this document does not propose the capability for any other combination, that new AFI/SAFIs are not expected to use it, and that the capability says how a Next Hop is encoded while the Multiprotocol Extensions capability of RFC 4760 decides whether the AFI/SAFI is allowed at all. Each is indicative, so section 2 gives it no normative level. Two ids the site scan cannot reach are listed below. |
| `5` | Operations | 1 | walked | Operations. One capitalised MUST-level site, mapped below to RFC5549-5-1. The two remaining directives are the SHOULD to give the next hop as an IPvx address when the session runs over IPvx and the speaker puts its own address in, and the MAY that policy overrides that default. Both are the unsourced ids below. The closing sentences state a consequence rather than an obligation: an RR client that cannot handle an encoding, as determined by the BGP Capability Advertisement, does not get the NLRI, and sound routing in some designs needs every RR client to handle whatever encodings any of them generate. |
| `6` | Usage Examples | 0 | walked | Usage Examples. A heading with no text of its own. Sections 6.1 and 6.2 carry the two examples. |
| `6.1` | IPv4 over IPv6 Core | 0 | walked | IPv4 over IPv6 Core. An illustrative encoding for the interconnection of IPv4 islands over an IPv6 backbone as described in MESH-FMWK: AFI 1, SAFI 1, Length of Next Hop Network Address 16 or 32, an IPv6 Next Hop, IPv4 routes as NLRI, and the capability triple <NLRI AFI 1, NLRI SAFI 1, Nexthop AFI 2>. Written as 'may be used' and 'would include', so it directs no speaker; the values repeat sections 3 and 4. |
| `6.2` | IPv4 VPN over IPv6 Core | 0 | walked | IPv4 VPN over IPv6 Core. The same illustrative shape for SAFI 128: Length of Next Hop Network Address 16 or 32, an IPv6 Next Hop, IPv4-VPN routes as NLRI, and the capability triple <NLRI AFI 1, NLRI SAFI 128, Nexthop AFI 2>. No directive. This is the section Erratum ID 5253 reports, because a VPN-IPv4 Next Hop carries an 8-octet Route Distinguisher and the lengths given here leave no room for one; RFC 8950 Section 2 answers the erratum with 24 or 48. The Errata section of rfc/short/rfc5549.md records both readings, and parseVPNNextHops in internal/core/bgp/attribute/mpnlri.go accepts 16 and 32 as the legacy form beside the 24 and 48 of RFC 8950. |
| `7` | IANA Considerations | 0 | skipped (iana) | IANA Considerations. Records the allocation of Capability Code 5 for the Extended Next Hop Encoding capability from the IETF Review range of RFC 5226. Binds IANA, not a speaker. The value itself is an obligation on a speaker only through RFC5549-4-3, which site 4:3 maps. |
| `8` | Security Considerations | 0 | walked | Security Considerations. States that the document raises no security issue beyond BGP-4 and the Multiprotocol Extensions, and that the IPv6 Next Hop Address can be an IPv4-mapped IPv6 address of RFC 4291. The last sentence addresses the network operator who configures next-hop security checks, states no capitalised keyword, and directs no speaker. |
| `9` | Acknowledgments | 0 | skipped (acknowledgements) | Acknowledgments. Thanks Yakov Rekhter, Pranav Mehta and John Scudder. No obligation. |
| `10` | References | 0 | skipped (references) | References. A heading whose two lists are sections 10.1 and 10.2. |
| `10.1` | not stated | 0 | skipped (references) | Normative References: RFC 2119, RFC 2545, RFC 3107, RFC 4291, RFC 4364, RFC 4760 and RFC 5492. |
| `10.2` | not stated | 0 | skipped (references) | Informative References: DYN-CAP, L2VPN-SIG, MESH-FMWK, RFC 4659, RFC 4684, RFC 4798, RFC 4925 and RFC 5226. The unnumbered Authors' Addresses that closes the document falls into this section, because the derivation opens a section only on a numbered heading at column 0. Neither states an obligation. |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `4:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The lead-in to the field list of the Capabilities Optional Parameter. Of the three bullets it introduces, only the Capability Code bullet carries an RFC 2119 keyword, and site 4:3 maps it to RFC5549-4-3; the Capability Length bullet and the Capability Value format bullet are written indicatively. The sentence therefore restates the obligation site 4:3 already carries and adds no separate one. | The fields in the Capabilities Optional Parameter MUST be set as follows: |

## Superseded

RFC 5549 is obsoleted by RFC 8950.

| Requirement | Disposition | Now stated at | Reason |
|---|---|---|---|
| [`RFC5549-4-1`](#rfc5549-4-1) A BGP speaker MUST only advertise IPv4 or VPN-IPv4 NLRI with an IPv6 Next Hop if it has ascertained via Capability Advertisement that the peer supports Extended Next Hop Encoding for the relevant AFI/SAFI pair (§4) | restated | RFC8950-4-1 | RFC 8950 Section 4 states the same obligation in the same words, that a speaker MUST only advertise IPv4 or VPN-IPv4 NLRI with an IPv6 next hop to a peer that has advertised the Extended Next Hop Encoding capability for the relevant AFI/SAFI pair |
| [`RFC5549-4-2`](#rfc5549-4-2) A BGP speaker that wishes to advertise an IPv6 Next Hop for IPv4 or VPN-IPv4 NLRI MUST use the Capability Advertisement procedures defined in RFC 5492 (§4) | restated | RFC8950-4-2 | RFC 8950 Section 4 keeps the RFC 5492 Capability Advertisement procedures unchanged as the way to determine peer support |
| [`RFC5549-4-3`](#rfc5549-4-3) The Capability Code field MUST be set to 5 (Extended Next Hop Encoding) (§4) | restated | RFC8950-4-3 | RFC 8950 Section 4 keeps Capability Code 5, and its Section 7 records that IANA moved the registration of that code point to RFC 8950 |
| [`RFC5549-3-1`](#rfc5549-3-1) The BGP speaker receiving the advertisement MUST use the Length of Next Hop Address field to determine which network-layer protocol the next hop address belongs to (§3) | restated | RFC8950-3-1 | RFC 8950 Section 3 keeps the Length of Next Hop Address field as the field that tells a receiver which network-layer protocol the next-hop address belongs to |
| [`RFC5549-5-1`](#rfc5549-5-1) When a next hop address needs to be passed along unchanged (e.g., Route Reflector), its encoding MUST NOT be changed (§5) | restated | RFC8950-5-1 | RFC 8950 Section 5 keeps the sentence unchanged, that an encoding passed along unchanged MUST NOT be changed |
| [`RFC5549-4-4`](#rfc5549-4-4) MUST NOT send IPv6 Next Hop for IPv4 NLRI to peers that have not advertised Extended Next Hop Encoding capability (§4, Compatibility) | restated | RFC8950-4-1 | this is the negative spelling of the Section 4 sentence RFC5549-4-1 states positively, and RFC 8950 states it once, positively, as RFC8950-4-1. RFC 5549 has no section named Compatibility, so the cite on this line names a section the document does not contain |
| [`RFC5549-5-2`](#rfc5549-5-2) By default, if a BGP session is running over IPvx, the next hop address SHOULD be specified as an IPvx address (§5) | restated | RFC8950-5-2 | RFC 8950 Section 5 keeps the default that a speaker putting its own address in as the next hop over an IPvx session SHOULD use an IPvx address |
| [`RFC5549-4-5`](#rfc5549-4-5) The Extended Next Hop Encoding capability MAY be dynamically updated through the Dynamic Capability capability (§4) | dropped | not stated | RFC 8950 states no dynamic-capability permission. RFC 5549 Section 4 ends with a MAY to update the Extended Next Hop Encoding capability through the Dynamic Capability capability of the expired draft-ietf-idr-dynamic-cap, and RFC 8950 Section 4 ends instead with the paragraph that the capability does not influence whether an AFI/SAFI is allowed. The permission is gone, so no successor obligation exists to point at |
| [`RFC5549-5-3`](#rfc5549-5-3) The default next-hop-address-family behavior may be overridden by policy (§5) | restated | RFC8950-5-3 | RFC 8950 Section 5 keeps the sentence that the default next-hop address family behavior may be overridden by policy |
