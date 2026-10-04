# RFC 4684 - Constrained Route Distribution for Border Gateway Protocol/MultiProtocol Label Switching (BGP/MPLS) Internet Protocol (IP) Virtual Private Networks (VPNs)

Partial. Every requirement this repository extracted from RFC 4684, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 33.3% | 2 of 6 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 6 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 6 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 6 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Proven by a recorded break | 100.0% | 6 of 6 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 6 | of 10 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 6 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 6 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 6 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 6 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 66.7% | 4 of 6 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

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
| Requirements | 10 |
| Gated MUST-level | 6 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 4 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 8 |
| Tagged units | 6 |
| Recorded audit verdicts | 2 |
| Discrimination records | 6 |
| Summary | `rfc/short/rfc4684.md` |
| Requirement shard | `rfc/requirements/rfc4684.md` |
| RFC text | `rfc/full/rfc4684.txt` |

## Enrolment

Enrolled: Constrained Route Distribution for BGP/MPLS VPNs (Route Target Constraint): six MUST-level requirements after the 2026-09-21 extraction walk. Four are {gap}, and the two the walk added are tested: RFC4684-4-1 (the MP_REACH_NLRI Next Hop is read as IPv4 at 4 octets and IPv6 at 16) and RFC4684-5-2 (a speaker that exchanges Route Target membership uses the Multiprotocol Extensions capability to advertise the AFI/SAFI pair). Ze implements RTC as a DECODE-ONLY NLRI codec for display/analysis (internal/component/bgp/plugins/nlri/rtc/rtc.go DecodeNLRIHex/RunDecode) with no encode/origination path and no RT-membership distribution. RFC4684-3.2-1 (Originator/Next-hop when advertising RT membership NLRI) and RFC4684-3.2-2 (best-path/client-path advertisement selection): Ze never advertises RT membership NLRI. RFC4684-3.2-3 (consider all iBGP paths for the outbound route filter): Ze builds no ORF from RT membership. RFC4684-6-1 (bound the End-of-RIB delay for VPN route advertisement): Ze gates no VPN route advertisement on RT-membership state. Disclosed in the docs/features/rfc-status.md RFC 4684 row (Partial). The 6-2/6-3/8-1 SHOULDs and 5-1 MAY are not gated.

## What the public ledger says

**Status:** Partial

**What the ledger says is covered:**

RTC NLRI decode for display/analysis (`ze bgp decode`). Tests bound per requirement in [`rfc/requirements/rfc4684.md`](https://github.com/ze-software/ze/blob/main/rfc/requirements/rfc4684.md).

**What the ledger says remains**

Decode-only: no encode/origination path and no RT-membership distribution. Four MUST gaps gated in [`rfc/short/rfc4684.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc4684.md): Ze does not advertise RT membership NLRI (so no §3.2 Originator/Next-hop or best-path/client-path selection), builds no outbound route filter from RT membership (§3.2), and gates no VPN route advertisement on RT-membership End-of-RIB (§6). Two further MUST rows the 2026-09-21 extraction walk added are tested: [`RFC4684-4-1`](#rfc4684-4-1) (Next Hop length decides IPv4 or IPv6, §4) and [`RFC4684-5-2`](#rfc4684-5-2) (Ze's OPEN advertises the (1, 132) pair with the Multiprotocol Extensions capability when `ipv4/rtc` is configured, §5).

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 2 | one part of the gated population |
| Annotated (including scoped evidence) | 4 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **6** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (2):** [`RFC4684-4-1`](#rfc4684-4-1), [`RFC4684-5-2`](#rfc4684-5-2)

**Annotated (including scoped evidence) (4):** [`RFC4684-3.2-1`](#rfc4684-3.2-1), [`RFC4684-3.2-2`](#rfc4684-3.2-2), [`RFC4684-3.2-3`](#rfc4684-3.2-3), [`RFC4684-6-1`](#rfc4684-6-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC4684-3.2-1` | When advertising RT membership NLRI to a route-reflector client, the Originator attribute shall be set to the router-id of the advertiser, and the Next-hop attribute shall be set of the local address for that session. (Section 3.2) | MUST | 3.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze implements RTC (Route Target Constraint) as a DECODE-ONLY NLRI codec for display/analysis (internal/component/bgp/plugins/nlri/rtc/rtc.go DecodeNLRIHex/RunDecode; there is no encode/origination path). It never advertises RT membership NLRI, so it implements none of the Section 3.2 advertisement Originator/Next-hop procedure. Disclosed in docs/features/rfc-status.md. |
| `RFC4684-3.2-2` | When advertising an RT membership NLRI to a non-client peer, if the best path as selected by the path selection procedure described in Section 9.1 of the base BGP specification [4] is a route received from a non-client peer, and if there is an alternative path to the same destination from a client, the attributes of the client path are advertised to the peer. (§3.2) | MUST | 3.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze does not advertise RT membership NLRI at all (its RTC support is a decode-only NLRI codec, internal/component/bgp/plugins/nlri/rtc/rtc.go, with no origination path), so it implements none of the Section 3.2 best-path/client-path advertisement selection. Disclosed in docs/features/rfc-status.md. |
| `RFC4684-3.2-3` | When processing RT membership NLRIs received from internal iBGP peers, it is necessary to consider all available iBGP paths for a given RT prefix, for building the outbound route filter, and not just the best path. (§3.2) | MUST | 3.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze builds no outbound route filter from RT membership -- its RTC support is a decode-only NLRI codec (internal/component/bgp/plugins/nlri/rtc/rtc.go) with no ORF construction or RT-membership-driven VPN route filtering. Disclosed in docs/features/rfc-status.md. |
| `RFC4684-6-1` | If a BGP speaker chooses to delay the advertisement of BGP VPN route updates until it receives this End-of-RIB marker, it MUST limit that delay to an upper bound. (Section 6) | MUST | 6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze does not gate VPN route advertisement on RT-membership state -- it has no RTC-driven VPN route distribution (the RTC codec is decode-only, internal/component/bgp/plugins/nlri/rtc/rtc.go), so there is no such End-of-RIB delay for Ze to bound. Disclosed in docs/features/rfc-status.md. |
| `RFC4684-4-1` | The Next Hop field of MP_REACH_NLRI attribute shall be interpreted as an IPv4 address whenever the length of NextHop address is 4 octets, and as a IPv6 address whenever the length of the NextHop address is 16 octets. (Section 4) | MUST | 4 | **positive:** `unit/verify` [`TestRFC4684RTCNextHopByLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc4684_nexthop_rtc_test.go#L28). **negative:** `unit/verify` [`TestRFC4684RTCNextHopByLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc4684_nexthop_rtc_test.go#L29) |
| `RFC4684-5-2` | A BGP speaker that wishes to exchange Route Target membership information must use the Multiprotocol Extensions Capability Code, as defined in RFC 2858 [5], to advertise the corresponding (AFI, SAFI) pair. (Section 5) | MUST | 5 | **positive:** `unit/verify` [`TestRFC4684RTCAdvertisedAsMultiprotocolPair`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc_open_family_capability_test.go#L129). **positive:** `unit/verify` [`TestRFC4684RTCFamilyIsAMultiprotocolCapability`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/rfc4684_rtc_test.go#L36). **negative:** `unit/verify` [`TestRFC4684RTCAdvertisedAsMultiprotocolPair`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc_open_family_capability_test.go#L130). **negative:** `unit/verify` [`TestRFC4684RTCPairUnderAnotherCodeIsNotTheFamily`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/rfc4684_rtc_test.go#L74) |
| `RFC4684-6-2` | As a hint that initial RT membership exchange is complete, implementations SHOULD generate an End-of-RIB marker, as defined in [8], for the Route Target membership (afi, safi), regardless of whether graceful-restart is enabled on the BGP session. (§6) | SHOULD | 6 | **positive:** no positive test. **negative:** no negative test |
| `RFC4684-6-3` | A BGP speaker should generate the minimum set of BGP VPN route updates (advertisements and/or withdrawls) necessary to transition between the previous and current state of the route distribution graph that is derived from Route Target membership information. (§6) | SHOULD | 6 | **positive:** no positive test. **negative:** no negative test |
| `RFC4684-8-1` | Implementations SHOULD also provide means to filter RT membership information. (§8) | SHOULD | 8 | **positive:** no positive test. **negative:** no negative test |
| `RFC4684-5-1` | A BGP speaker MAY participate in the distribution of Route Target information without using the learned information for purposes of VPN NLRI output route filtering, although this is discouraged. (§5) | MAY | 5 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC4684-3.2-1`](#rfc4684-3.2-1) When advertising RT membership NLRI to a route-reflector client, the Originator attribute shall be set to the router-id of the advertiser, and the Next-hop attribute shall be set of the local address for that session. (Section 3.2) | {gap}, no test | Ze implements RTC (Route Target Constraint) as a DECODE-ONLY NLRI codec for display/analysis (internal/component/bgp/plugins/nlri/rtc/rtc.go DecodeNLRIHex/RunDecode; there is no encode/origination path). It never advertises RT membership NLRI, so it implements none of the Section 3.2 advertisement Originator/Next-hop procedure. Disclosed in docs/features/rfc-status.md. |
| [`RFC4684-3.2-2`](#rfc4684-3.2-2) When advertising an RT membership NLRI to a non-client peer, if the best path as selected by the path selection procedure described in Section 9.1 of the base BGP specification [4] is a route received from a non-client peer, and if there is an alternative path to the same destination from a client, the attributes of the client path are advertised to the peer. (§3.2) | {gap}, no test | Ze does not advertise RT membership NLRI at all (its RTC support is a decode-only NLRI codec, internal/component/bgp/plugins/nlri/rtc/rtc.go, with no origination path), so it implements none of the Section 3.2 best-path/client-path advertisement selection. Disclosed in docs/features/rfc-status.md. |
| [`RFC4684-3.2-3`](#rfc4684-3.2-3) When processing RT membership NLRIs received from internal iBGP peers, it is necessary to consider all available iBGP paths for a given RT prefix, for building the outbound route filter, and not just the best path. (§3.2) | {gap}, no test | Ze builds no outbound route filter from RT membership -- its RTC support is a decode-only NLRI codec (internal/component/bgp/plugins/nlri/rtc/rtc.go) with no ORF construction or RT-membership-driven VPN route filtering. Disclosed in docs/features/rfc-status.md. |
| [`RFC4684-6-1`](#rfc4684-6-1) If a BGP speaker chooses to delay the advertisement of BGP VPN route updates until it receives this End-of-RIB marker, it MUST limit that delay to an upper bound. (Section 6) | {gap}, no test | Ze does not gate VPN route advertisement on RT-membership state -- it has no RTC-driven VPN route distribution (the RTC codec is decode-only, internal/component/bgp/plugins/nlri/rtc/rtc.go), so there is no such End-of-RIB delay for Ze to bound. Disclosed in docs/features/rfc-status.md. |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC4684-3.2-1`](#rfc4684-3.2-1)

When advertising RT membership NLRI to a route-reflector client, the Originator attribute shall be set to the router-id of the advertiser, and the Next-hop attribute shall be set of the local address for that session. (Section 3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4684-3.2-1, so no unit is bound to it.

### [`RFC4684-3.2-2`](#rfc4684-3.2-2)

When advertising an RT membership NLRI to a non-client peer, if the best path as selected by the path selection procedure described in Section 9.1 of the base BGP specification [4] is a route received from a non-client peer, and if there is an alternative path to the same destination from a client, the attributes of the client path are advertised to the peer. (§3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4684-3.2-2, so no unit is bound to it.

### [`RFC4684-3.2-3`](#rfc4684-3.2-3)

When processing RT membership NLRIs received from internal iBGP peers, it is necessary to consider all available iBGP paths for a given RT prefix, for building the outbound route filter, and not just the best path. (§3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4684-3.2-3, so no unit is bound to it.

### [`RFC4684-6-1`](#rfc4684-6-1)

If a BGP speaker chooses to delay the advertisement of BGP VPN route updates until it receives this End-of-RIB marker, it MUST limit that delay to an upper bound. (Section 6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4684-6-1, so no unit is bound to it.

### [`RFC4684-4-1`](#rfc4684-4-1)

The Next Hop field of MP_REACH_NLRI attribute shall be interpreted as an IPv4 address whenever the length of NextHop address is 4 octets, and as a IPv6 address whenever the length of the NextHop address is 16 octets. (Section 4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. (a) forbidden: a 4-octet RTC next hop read as anything but IPv4, or a 16-octet one read as anything but IPv6 (for instance by keying on AFI 1). (b) TestRFC4684RTCNextHopByLength builds MP_REACH_NLRI for AFI 1 SAFI 132 and t.Fatalf's unless MPReachWire.NextHop returns exactly 192.0.2.1 and not Is6 for 4 octets, and exactly 2001:db8::1 and not Is4 for 16 octets. Both clauses carry both polarities.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4684RTCNextHopByLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc4684_nexthop_rtc_test.go#L29) | unit/verify | revert, verified |
| positive | [`TestRFC4684RTCNextHopByLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/wireu/rfc4684_nexthop_rtc_test.go#L28) | unit/verify | revert, verified |

### [`RFC4684-5-2`](#rfc4684-5-2)

A BGP speaker that wishes to exchange Route Target membership information must use the Multiprotocol Extensions Capability Code, as defined in RFC 2858 [5], to advertise the corresponding (AFI, SAFI) pair. (Section 5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRFC4684RTCAdvertisedAsMultiprotocolPair reads Ze's built OPEN: ipv4/rtc configured -> MP (1,132) present and negotiated with a peer offering it; mode disable -> absent and not negotiated. Judge break: disabled-family skip removed turned it red. The codec-level tag is supplementary. Summary prose that said 5-2 carries no test is corrected.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4684RTCAdvertisedAsMultiprotocolPair`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc_open_family_capability_test.go#L130) | unit/verify | revert, verified |
| negative | [`TestRFC4684RTCPairUnderAnotherCodeIsNotTheFamily`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/rfc4684_rtc_test.go#L74) | unit/verify | revert, verified |
| positive | [`TestRFC4684RTCAdvertisedAsMultiprotocolPair`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc_open_family_capability_test.go#L129) | unit/verify | revert, verified |
| positive | [`TestRFC4684RTCFamilyIsAMultiprotocolCapability`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/rfc4684_rtc_test.go#L36) | unit/verify | revert, verified |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | prose |
| Source | rfc/full/rfc4684.txt |
| Source fingerprint | 2189a4bee8739009 |
| Record | rfc/extraction/rfc4684.json |
| Mapped sentences | 4 |
| Declined as scope | 5 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | walked | not stated |
| `1` | not stated | 0 | walked | not stated |
| `1.1` | not stated | 0 | walked | not stated |
| `2` | not stated | 0 | walked | not stated |
| `3` | not stated | 0 | walked | not stated |
| `3.1` | not stated | 1 | walked | not stated |
| `3.2` | not stated | 3 | walked | not stated |
| `4` | not stated | 1 | walked | not stated |
| `5` | not stated | 1 | walked | not stated |
| `6` | not stated | 1 | walked | not stated |
| `7` | not stated | 1 | walked | not stated |
| `8` | not stated | 0 | walked | not stated |
| `9` | not stated | 0 | walked | not stated |
| `10` | not stated | 0 | walked | not stated |
| `10.1` | not stated | 0 | walked | not stated |
| `10.2` | not stated | 1 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `3.1:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Non-normative description of what the NLRI shape buys, inside the worked example of Figure 1: the sentence says including the originator AS 'allows BGP speakers to use standard path selection rules' to prune duplicate paths. It states no obligation on any role, and the paragraph after it reasons about the same example ('In the example above, AS e needs to maintain a path to AS a'). | Using RT membership information that includes both route-target and originator AS number allows BGP speakers to use standard path selection rules concerning as-path length (and other policy mechanisms) to prune duplicate paths in the RT membership information flooding graph, while maintaining the information required to reach all autonomous systems advertising the Route Target. |
| `3.2:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The lead-in that carries the 'shall' for the two lettered rules that follow it, not a third obligation: 'a BGP speaker shall modify its procedure to calculate the BGP attributes such that the following apply:'. Rule (i) is site 3.2:2 and maps to RFC4684-3.2-1. Rule (ii) carries no keyword of its own, so section 3.2 records RFC4684-3.2-2 as unsourced. | In addition, when advertising Route Target membership information sourced by the local autonomous system to an iBGP peer, a BGP speaker shall modify its procedure to calculate the BGP attributes such that the following apply: |
| `3.2:3` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | An assumption about the operator's topology, not an obligation on an implementation. The enclosing construction is 'These procedures assume that the autonomous-system route reflection topology is configured such that IPv4 unicast routing would work correctly. For instance, route reflection clusters must be contiguous.' The 'must' states what the assumed configuration looks like; no code of ze's could satisfy or violate it. | For instance, route reflection clusters must be contiguous. |
| `7:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Deployment Considerations describing the scaling situation WITHOUT this mechanism, not an obligation: 'This mechanism reduces the scaling requirements that are imposed on route reflectors ... By default, a reflector must scale in terms of the total number of VPN routes present on the network.' The 'must' names a consequence of not deploying RT constraint, and no implementation behavior answers it. | By default, a reflector must scale in terms of the total number of VPN routes present on the network. |
| `10.2:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | IETF boilerplate from the Intellectual Property Statement, addressed to 'any interested party' about bringing copyrights and patents to the IETF's attention. It is not part of the protocol specification. | The IETF invites any interested party to bring to its attention any copyrights, patents or patent applications, or other proprietary rights that may cover technology that may be required to implement this standard. |

## Superseded

No document obsoletes RFC 4684, so its obligations are stated where they were written.
