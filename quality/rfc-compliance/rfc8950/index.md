# RFC 8950 - Advertising IPv4 Network Layer Reachability Information (NLRI) with an IPv6 Next Hop

Supported. Every requirement this repository extracted from RFC 8950, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 50.0% | 3 of 6 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 50.0% | 3 of 6 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 6 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 0.0% | 0 of 6 binding obligations | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 0.0% | 0 of 13 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 6 | of 8 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
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
| Requirements | 8 |
| Gated MUST-level | 6 |
| Obligations that bind Ze | 6 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 13 |
| Tagged units | 13 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc8950.md` |
| Requirement shard | `rfc/requirements/rfc8950.md` |
| RFC text | `rfc/full/rfc8950.txt` |

## Enrolment

Enrolled: Advertising IPv4 NLRI with an IPv6 Next Hop (extended next-hop encoding, capability code 5): six MUST-level requirements, all met. 4-1 (cross-family IPv6 next-hop honored only when the extended-next-hop capability is negotiated), 4-2 (a tuple is negotiated only if both peers advertise the same NLRI-AFI/SAFI/next-hop-AFI), and 3-1 (a 16- or 32-octet next-hop is decoded as IPv6 regardless of the NLRI AFI) carry positive+negative tags. 4-3 (capability code 5), 3-2 (VPN next-hop carries an 8-octet RD set to zero), and 5-1 (a reflected next-hop encoding is not rewritten) are {single-polarity: positive}, bound to code-constant, RD-zeroing, and reflection-byte-identical tests in internal/core/bgp/attribute and internal/component/bgp/reactor.

## What the public ledger says

**Status:** Supported

**What the ledger says is covered:**

IPv6 next-hop for IPv4 NLRI and negotiated extended next-hop lookup.

**What the ledger says remains:**

No tracked gap in current source anchors.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 3 | one part of the gated population |
| Annotated instead of tested | 3 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **6** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (3):** [`RFC8950-4-1`](#rfc8950-4-1), [`RFC8950-4-2`](#rfc8950-4-2), [`RFC8950-3-1`](#rfc8950-3-1)

**Annotated instead of tested (3):** [`RFC8950-4-3`](#rfc8950-4-3), [`RFC8950-5-1`](#rfc8950-5-1), [`RFC8950-3-2`](#rfc8950-3-2)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC8950-4-1` | A BGP speaker MUST only advertise IPv4 or VPN-IPv4 NLRI with an IPv6 next hop to a peer if it has ascertained via BGP Capability Advertisement that the peer supports the Extended Next Hop Encoding capability for the relevant AFI/SAFI pair (§4) | MUST | 4 - Use of BGP Capability Advertisement | **positive:** `unit/verify` [`TestCanUseNextHopFor_ExtendedNH`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_test.go#L1143). **negative:** `unit/verify` [`TestCanUseNextHopFor_CrossFamilyNoCap`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_test.go#L1174). **negative:** `unit/verify` [`TestCanUseNextHopFor_NilSendCtx`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_test.go#L1198) |
| `RFC8950-4-2` | MUST use the Capability Advertisement procedures defined in RFC 5492 with the Extended Next Hop Encoding capability (§4) | MUST | 4 - Use of BGP Capability Advertisement | **positive:** `unit/verify` [`TestNegotiateExtendedNextHop`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/negotiated_test.go#L283). **negative:** `unit/verify` [`TestNegotiateExtendedNextHopMismatch`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/negotiated_test.go#L322) |
| `RFC8950-4-3` | The Capability Code field MUST be set to 5 (§4) | MUST | 4 - Use of BGP Capability Advertisement | **positive:** `unit/verify` [`TestExtendedNextHopCapability`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L502). **positive:** `unit/verify` [`TestExtendedNextHopRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L535). **negative:** no negative test. **{single-polarity}:** ze binds the Extended Next Hop Encoding capability to the constant CodeExtendedNextHop = 5 (internal/core/bgp/capability/capability.go:70); Code() returns it (capability.go:640) and WriteTo emits it (capability.go:644-646). The code is a fixed constant with no alternate-value code path, so there is no wrong-code case to reject as a negative |
| `RFC8950-3-1` | The BGP speaker receiving the advertisement MUST use the Length of Next Hop Address field to determine which network-layer protocol the next-hop address belongs to (§3) | MUST | 3 | **positive:** `unit/verify` [`TestParseMPReachNLRI_ExtendedNextHop`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/mpnlri_test.go#L338). **positive:** `unit/verify` [`TestParseMPReachNLRI_ExtendedNextHop_DualStack`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/mpnlri_test.go#L442). **negative:** `unit/verify` [`TestParseMPReachNLRI_InvalidNextHopLength`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/mpnlri_test.go#L488) |
| `RFC8950-5-1` | When a next-hop address needs to be passed along unchanged (e.g., as a Route Reflector), its encoding MUST NOT be changed (§5) | MUST NOT | 5 - Operations | **positive:** `unit/verify` [`TestReactorForwardRRPreservesExtendedNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_rr_test.go#L230). **negative:** no negative test. **{single-polarity}:** on the reflection path ze rewrites the next-hop only under an explicit next-hop-self/explicit override (nhMode != nhModeNone); the default nhModeNone leaves the next-hop untouched (internal/component/bgp/reactor/peer_forward_facts.go:226-229) and the MP re-encode changes an attribute only when the NLRI framing differs between encoding contexts (internal/component/bgp/reactor/forward_body.go:217), so a reflected next-hop is carried verbatim and there is no ze code path that rewrites an unchanged-passthrough next-hop to assert as a negative. The positive is proven byte-identical in TestReactorForwardRRPreservesExtendedNextHop |
| `RFC8950-3-2` | For VPN-IPv4 NLRI with IPv6 next hop, the Route Distinguisher in the next hop MUST be set to zero (8 zero bytes) (§3) | MUST | 3 | **positive:** `unit/verify` [`TestMPReachNLRI_RoundTrip_VPN`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/mpnlri_test.go#L674). **positive:** `unit/verify` [`TestParseMPReachNLRI_VPNWithIPv6NextHop`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/mpnlri_test.go#L576). **negative:** no negative test. **{single-polarity}:** on encode ze always writes an all-zero 8-byte Route Distinguisher before a VPN next-hop (internal/core/bgp/attribute/mpnlri.go:170-176) and on decode it skips the 8 RD bytes without validating their value (mpnlri.go:438-443), so ze never emits a nonzero RD and never rejects one -- there is no negative case |
| `RFC8950-5-2` | If a BGP session is running over IPvx and the speaker is setting itself as next hop, the next-hop address SHOULD be specified as an IPvx address (§5) | SHOULD | 5 - Operations | **positive:** no positive test. **negative:** no negative test |
| `RFC8950-5-3` | Default next-hop address family selection may be overridden by policy (§5) | MAY | 5 - Operations | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

RFC 8950 declares no gap, and every gated MUST it carries has a test bound to it.

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC8950-4-1`](#rfc8950-4-1)

A BGP speaker MUST only advertise IPv4 or VPN-IPv4 NLRI with an IPv6 next hop to a peer if it has ascertained via BGP Capability Advertisement that the peer supports the Extended Next Hop Encoding capability for the relevant AFI/SAFI pair (§4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestCanUseNextHopFor_CrossFamilyNoCap`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_test.go#L1174) | unit/verify | unproven |
| negative | [`TestCanUseNextHopFor_NilSendCtx`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_test.go#L1198) | unit/verify | unproven |
| positive | [`TestCanUseNextHopFor_ExtendedNH`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_test.go#L1143) | unit/verify | unproven |

### [`RFC8950-4-2`](#rfc8950-4-2)

MUST use the Capability Advertisement procedures defined in RFC 5492 with the Extended Next Hop Encoding capability (§4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestNegotiateExtendedNextHopMismatch`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/negotiated_test.go#L322) | unit/verify | unproven |
| positive | [`TestNegotiateExtendedNextHop`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/negotiated_test.go#L283) | unit/verify | unproven |

### [`RFC8950-4-3`](#rfc8950-4-3)

The Capability Code field MUST be set to 5 (§4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestExtendedNextHopCapability`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L502) | unit/verify | unproven |
| positive | [`TestExtendedNextHopRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L535) | unit/verify | unproven |

### [`RFC8950-3-1`](#rfc8950-3-1)

The BGP speaker receiving the advertisement MUST use the Length of Next Hop Address field to determine which network-layer protocol the next-hop address belongs to (§3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestParseMPReachNLRI_InvalidNextHopLength`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/mpnlri_test.go#L488) | unit/verify | unproven |
| positive | [`TestParseMPReachNLRI_ExtendedNextHop`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/mpnlri_test.go#L338) | unit/verify | unproven |
| positive | [`TestParseMPReachNLRI_ExtendedNextHop_DualStack`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/mpnlri_test.go#L442) | unit/verify | unproven |

### [`RFC8950-5-1`](#rfc8950-5-1)

When a next-hop address needs to be passed along unchanged (e.g., as a Route Reflector), its encoding MUST NOT be changed (§5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestReactorForwardRRPreservesExtendedNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_rr_test.go#L230) | unit/verify | unproven |

### [`RFC8950-3-2`](#rfc8950-3-2)

For VPN-IPv4 NLRI with IPv6 next hop, the Route Distinguisher in the next hop MUST be set to zero (8 zero bytes) (§3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestMPReachNLRI_RoundTrip_VPN`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/mpnlri_test.go#L674) | unit/verify | unproven |
| positive | [`TestParseMPReachNLRI_VPNWithIPv6NextHop`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/mpnlri_test.go#L576) | unit/verify | unproven |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | ze-work agent, spec-rfcgate-6 phase 5, rfc8950 |
| Signed off | 2026-08-31 |
| Register | rfc2119 |
| Source | rfc/full/rfc8950.txt |
| Source fingerprint | b1b27252ca8688aa |
| Record | rfc/extraction/rfc8950.json |
| Mapped sentences | 5 |
| Declined as scope | 1 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | skipped (front-matter) | Title block, Abstract, Status of This Memo, Copyright Notice and Table of Contents. The Abstract restates the Introduction and states no obligation. |
| `1` | Introduction | 0 | walked | Introduction. Indicative prose: which AFI/SAFI pairs already let the next hop belong to another address family, that RFC 4798 and RFC 4659 solve the IPv6 NLRI with an IPv4 next hop direction, that no solution existed for the reverse direction, and what this document adds. The last sentence records that the document obsoletes RFC 5549. No sentence directs a speaker. |
| `1.1` | Requirements Language | 0 | walked | Requirements Language. The BCP 14 key-words paragraph of RFC 2119 and RFC 8174, which binds the key words only when they appear in all capitals. It tells a reader how to read the other sections and binds no speaker, which is why the derivation excludes it from the site inventory. |
| `2` | Changes Compared to RFC 5549 | 0 | walked | Changes Compared to RFC 5549. Two changes, both stated indicatively. The next hop for AFI/SAFI <1/128> is now encoded as a VPN-IPv6 address of 24 or 48 octets, in place of the 16 or 32 octets of RFC 5549, which answers Erratum ID 5253. AFI/SAFI <1/129> can now use an IPv6 underlay with the same encoding and procedures as <1/128>. Both sentences describe what sections 3, 6.2 and 6.3 define and direct no speaker. The lengths are carried by the MP_REACH_NLRI Next Hop Encoding tables of rfc/short/rfc8950.md. |
| `3` | not stated | 1 | walked | Extension of AFI/SAFI Definitions for the IPv4 Address Family. One capitalised MUST-level site, mapped below to RFC8950-3-1. The rest of the section is value assignment written as bullet lists: AFI 1 with SAFI 1, 2 or 4 takes a Length of Next Hop Address of 16 or 32 and the address is then of type IPv6, AFI 1 with SAFI 128 or 129 takes 24 or 48 and the address is then of type VPN-IPv6. Those bullets carry no RFC 2119 level, and the Wire Formats and Decoding Rules tables of rfc/short/rfc8950.md hold the values. One obligation inside them the site scan cannot see is the unsourced id below: the 8-octet Route Distinguisher of a VPN-IPv6 next hop is set to zero. A second bullet, "This field is to be constructed as per Section 3 of [RFC2545]", delegates the global and link-local construction to RFC 2545, which carries its own summary and its own sign-off. The closing note that RFC 4684 and RFC 6074 use the same length method is a comparison, not a directive. |
| `4` | Use of BGP Capability Advertisement | 4 | walked | Use of BGP Capability Advertisement. Four capitalised MUST-level sites: the RFC 5492 procedures (RFC8950-4-2), the lead-in to the field list (excluded as a duplicate below), Capability Code 5 (RFC8950-4-3), and the precondition that a speaker advertises an IPv6 next hop for IPv4 or VPN-IPv4 NLRI only after it has ascertained peer support for the relevant AFI/SAFI pair (RFC8950-4-1). The Capability Value format, its <NLRI AFI, NLRI SAFI, Nexthop AFI> triples and the values this document allows (NLRI AFI 1, NLRI SAFI 1, 2, 4, 128 or 129, Nexthop AFI 2) are value assignment, held by the Wire Formats and Constants tables of rfc/short/rfc8950.md. Three sentences the site scan cannot see are scoping rather than obligations: that this document does not specify the capability for any other combination, that new AFIs/SAFIs are not expected to use it, and that the capability says how a next hop is encoded while the Multiprotocol Extensions capability of RFC 4760 decides whether the AFI/SAFI is allowed at all. Each is indicative, so section 1.1 gives it no normative level. |
| `5` | Operations | 1 | walked | Operations. One capitalised MUST-level site, mapped below to RFC8950-5-1. The two remaining directives are the SHOULD to give the next hop as an IPvx address when the session runs over IPvx and the speaker puts its own address in, and the MAY that policy overrides that default. Both are the unsourced ids below. The closing sentences state a consequence rather than an obligation: an RR client that cannot handle an encoding, as determined by the BGP Capability Advertisement, does not get the NLRI, which is what RFC8950-4-1 already requires of the sender, and sound routing in some designs needs every RR client to handle whatever encodings any of them generate. |
| `6` | Usage Examples | 0 | walked | Usage Examples. A heading with no text of its own. Sections 6.1, 6.2 and 6.3 carry the three examples. |
| `6.1` | IPv4 over IPv6 Core | 0 | walked | IPv4 over IPv6 Core. An illustrative encoding for the interconnection of IPv4 islands over an IPv6 backbone as described in RFC 5565: AFI 1, SAFI 1, Length of Next Hop Address field 16 or 32, an IPv6 next hop, and the capability triple <NLRI AFI 1, NLRI SAFI 1, Nexthop AFI 2>. Written as "may be used" and "would include", so it directs no speaker; the values repeat sections 3 and 4. |
| `6.2` | IPv4 VPN Unicast over IPv6 Core | 0 | walked | IPv4 VPN Unicast over IPv6 Core. The same illustrative shape for SAFI 128: Length of Next Hop Address field 24 or 48, a VPN-IPv6 next hop whose Route Distinguisher is zero, and the capability triple <NLRI AFI 1, NLRI SAFI 128, Nexthop AFI 2>. This is the example section 2 points at for the encoding change against RFC 5549. No directive. |
| `6.3` | IPv4 VPN Multicast over IPv6 Core | 0 | walked | IPv4 VPN Multicast over IPv6 Core. The same illustrative shape for SAFI 129, which this document adds: Length of Next Hop Address field 24 or 48, a VPN-IPv6 next hop whose Route Distinguisher is zero, and the capability triple <NLRI AFI 1, NLRI SAFI 129, Nexthop AFI 2>. No directive. |
| `7` | IANA Considerations | 0 | skipped (iana) | IANA Considerations. Records that this document defines no new code point, and that IANA updated the "Extended Next Hop Encoding" entry of the Capability Codes registry, value 5, to refer to this document in place of RFC 5549. Binds IANA, not a speaker. |
| `8` | Security Considerations | 0 | walked | Security Considerations. States that the document raises no security issue beyond BGP-4 and the Multiprotocol Extensions, that the ability to advertise an IPv6 next hop widens the reach of the traffic diversion attacks RFC 4272 describes, and that the IPv6 next hop can be an IPv4-mapped IPv6 address of RFC 4291. The last point addresses the network operator who configures next-hop security checks, states no capitalised keyword, and directs no speaker. |
| `9` | References | 0 | skipped (references) | References. A heading whose two lists are sections 9.1 and 9.2. |
| `9.1` | not stated | 0 | skipped (references) | Normative References: RFC 2119, RFC 2545, RFC 4291, RFC 4364, RFC 4760, RFC 5492, RFC 8174 and RFC 8277. |
| `9.2` | not stated | 0 | skipped (references) | Informative References: Erratum ID 5253, the IANA Address Family Numbers, Capability Codes and SAFI Parameters registries, RFC 4272, RFC 4659, RFC 4684, RFC 4798, RFC 4925, RFC 5549, RFC 5565, RFC 6074, RFC 6513 and RFC 6514. The unnumbered Acknowledgments and Authors' Addresses that close the document fall into this section, because the derivation opens a section only on a numbered heading at column 0. Neither states an obligation. |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `4:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The lead-in to the field list of the Capabilities Optional Parameter. Of the three bullets it introduces, only the Capability Code bullet carries an RFC 2119 keyword, and site 4:3 maps it to RFC8950-4-3; the Capability Length bullet and the Capability Value format bullet are written indicatively. The sentence therefore restates the obligation site 4:3 already carries and adds no separate one. | The fields in the Capabilities Optional Parameter MUST be set as follows: |

## Superseded

No document obsoletes RFC 8950, so its obligations are stated where they were written.
