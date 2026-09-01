# RFC 7999 - BLACKHOLE Community

Partial. Every requirement this repository extracted from RFC 7999, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 100.0% | 4 of 4 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 4 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 4 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 0.0% | 0 of 4 binding obligations | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 0.0% | 0 of 26 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 4 | of 16 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 4 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

The 4 shares marked as a part above are the whole of the 4 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Public status | Partial |
| Enrolment | Enrolled |
| Requirements | 16 |
| Gated MUST-level | 4 |
| Obligations that bind Ze | 4 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 26 |
| Tagged units | 26 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc7999.md` |
| Requirement shard | `rfc/requirements/rfc7999.md` |
| RFC text | `rfc/full/rfc7999.txt` |

## Enrolment

Enrolled: BLACKHOLE Community (65535:666, 0xFFFF029A): four MUST-level requirements, every one of them proven in both polarities. RFC7999-3.3-1 and RFC7999-3.3-2 are the two bullet conditions of the single MUST sentence of section 3.3. Each carries unit evidence over the RIB honoring path (internal/component/bgp/plugins/rib/rib_blackhole_wiring_test.go) and interop evidence at nightly tier. That interop scenario, test/interop/scenarios/bgp-rfc7999-blackhole-frr, reads `ip route show` inside the ze container, so its assertion is a Linux FIB discard route rather than a Ze table. Its three outcomes differ in configuration alone. FRR announces a covered prefix on an agreed session and the kernel holds `blackhole 10.100.0.1`. FRR announces an uncovered prefix on the same session and the kernel forwards it, which is the 3.3-1 negative. BIRD announces a covered prefix on a session that agreed NO community and the kernel forwards it, which is the 3.3-2 negative: the authorization IS present on that peer, so the negative isolates the session agreement instead of an absent config block. RFC7999-3.3-4 is the operator obligation that origin validation must not block a legitimate blackhole, proven over the RPKI decision path (internal/component/bgp/plugins/rpki/blackhole_decision_test.go). RFC7999-3.1-2 binds the SEND side and carried a {not-applicable} annotation until 2026-08-13, on the reading that the obligation binds "the two networks" and no daemon can verify an out-of-band agreement. The owner replaced that reading the same day: configuring the community on a peer IS Ze's half of the agreement, so the obligation has a machine-checkable predicate after all and the annotation is withdrawn. Ze originates BLACKHOLE-tagged announcements through announce blackhole and through announce unicast community 65535:666, and both now narrow the fan-out to the sessions that named the community (agreedSelector, internal/component/bgp/plugins/cmd/announce/blackhole_agreement.go). A peer that agreed is announced to; a peer that did not is left out entirely rather than being sent the prefix untagged, because an ordinary announcement of a host route under attack attracts the traffic the operator asked to have discarded. Both polarities are proven at unit tier over the command handler. The other 12 rows are 6 SHOULD, 1 SHOULD NOT, 1 RECOMMENDED and 4 MAY. The section-by-section walk is recorded in rfc/extraction/rfc7999.json at register prose, with 1 of 4 sites excluded. Enrolled 2026-08-13.

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

- The BLACKHOLE community (65535:666) is recognized on receive, and Ze honors it per BGP session. Section 3.3 states two conditions in one MUST sentence, and Ze enforces both. The peer must have agreed on that session by naming the community it honors (`blackhole communities`), which accepts the well-known value under either spelling and an operator's own value, because operators run RTBH on their own community far more often than on 65535:666. The announced prefix must be covered by an equal or shorter prefix that peer is authorized to advertise (`blackhole prefixes`). Coverage is not prefix-list membership: a /24 authorization covers the /32 inside it (`coveredByAuthorized`, [`internal/component/bgp/plugins/rib/rib_blackhole.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_blackhole.go)). A honored route becomes a discard route in the Linux FIB (`RTN_BLACKHOLE`) and a drop path in VPP. Section 4's default holds: a `blackhole` container with neither leaf-list, and a peer with no container, discard nothing. Stating `prefixes` alone is itself the explicit configuration directive Section 4 asks for, so it resolves the community to the well-known value
- a stated `communities` list is taken exactly and the well-known value is never added to it. Section 3.1's send-side obligation is met on the origination path. `announce blackhole`, and `announce unicast community 65535:666`, advertise the community only to the sessions that named it, and a peer that named none is left out of the fan-out rather than being sent the prefix untagged (`agreedSelector`, [`internal/component/bgp/plugins/cmd/announce/blackhole_agreement.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/cmd/announce/blackhole_agreement.go)). Section 3.3's origin-validation obligation is met by `blackhole-exempt` under an RPKI peer. It keeps a BLACKHOLE-tagged route that RFC 6811 makes Invalid on prefix length alone, and only when a covering VRP names the origin AS and the session named the community the route carries ([`internal/component/bgp/plugins/rpki/blackhole.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/blackhole.go)). All three consumers read one per-peer answer from one place (internal/component/bgp/blackholecfg). Section 3.2's receiver obligations and the own-Global-Administrator scrub are tracked under RFC 1997 and RFC 7454. Tests bound per requirement in [`rfc/requirements/rfc7999.md`](https://github.com/ze-software/ze/blob/main/rfc/requirements/rfc7999.md), and the FRR and BIRD interop scenario `test/interop/scenarios/bgp-rfc7999-blackhole-frr` asserts the discard route in a real kernel. Both Section 3.3 conditions carry interop evidence as well as unit evidence. That scenario tags [`RFC7999-3.3-1`](#rfc7999-3.3-1) and [`RFC7999-3.3-2`](#rfc7999-3.3-2) in both polarities. Neither can afterwards fall back to a unit test alone. Enrolled on 2026-08-13, so the ratchets now hold every requirement here. The walk behind that claim is recorded in [`rfc/extraction/rfc7999.json`](https://github.com/ze-software/ze/blob/main/rfc/extraction/rfc7999.json), at register `prose`, with 1 of 4 sites excluded.


**What the ledger says remains**

No tracked gap. Every MUST-level row is proven in both polarities. [`RFC7999-3.1-2`](#rfc7999-3.1-2) carried a `{not-applicable}` annotation until 2026-08-13, on the reading that Section 3.1 binds "the two networks" where Section 3.3 binds "BGP speakers", and that no daemon can verify an out-of-band agreement. The owner replaced that reading the same day: configuring the community on a peer is Ze's half of the agreement, the peer sending it is the other half, so the obligation is machine-checkable on the send side. The annotation is withdrawn and the requirement is tested. Four advisory rows carry no test. [`RFC7999-3.1-3`](#rfc7999-3.1-3) and [`RFC7999-3.1-5`](#rfc7999-3.1-5) bind the sender. [`RFC7999-3.3-3`](#rfc7999-3.3-3) asks a multilateral speaker to apply both Section 3.3 conditions. [`RFC7999-6-1`](#rfc7999-6-1) asks for strict filtering. None is gated, so none is ratcheted. Two are worth stating plainly. The authorization Section 3.3 and Section 6 turn on is operator-supplied. `blackhole prefixes` is a configured leaf-list, not a view derived from IRR or RPKI. The honoring machinery is bilateral, so a route server does not apply the two conditions to its clients.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 4 | one part of the gated population |
| Annotated instead of tested | 0 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **4** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (4):** [`RFC7999-3.1-2`](#rfc7999-3.1-2), [`RFC7999-3.3-1`](#rfc7999-3.3-1), [`RFC7999-3.3-2`](#rfc7999-3.3-2), [`RFC7999-3.3-4`](#rfc7999-3.3-4)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC7999-3.1-2` | "In a bilateral peering relationship, use of the BLACKHOLE community MUST be agreed upon by the two networks before advertising it" (§3.1) | MUST | 3.1 - IP Prefix Announcements with BLACKHOLE Community Attached | **positive:** `unit/verify` [`TestAnnounceBlackholeReachesAMemberOfADynamicGroup`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/cmd/announce/blackhole_agreement_test.go#L272). **positive:** `unit/verify` [`TestAnnounceBlackholeReachesAnAgreedPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/cmd/announce/blackhole_agreement_test.go#L96). **negative:** `unit/verify` [`TestAnnounceBlackholeIsWithheldFromAPeerThatDidNotAgree`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/cmd/announce/blackhole_agreement_test.go#L110). **negative:** `unit/verify` [`TestAnnounceBlackholeIsWithheldFromASessionOutsideTheGroup`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/cmd/announce/blackhole_agreement_test.go#L300) |
| `RFC7999-3.3-1` | A BGP speaker in a bilateral peering relationship using BLACKHOLE MUST only accept and honor an announcement carrying BLACKHOLE when the announced prefix is covered by an equal or shorter prefix that the neighboring network is authorized to advertise (§3.3) | MUST | 3.3 - Accepting Blackholed IP Prefixes | **positive:** `unit/verify` [`TestBlackholeGroupIdentityArrivesOnAStructuredEvent`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_blackhole_dynamic_test.go#L31). **positive:** `unit/verify` [`TestBlackholeRouteTypeStampedOnBestPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_blackhole_wiring_test.go#L104). **positive:** `unit/verify` [`TestBlackholeRuleForDynamicGroup`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_blackhole_wiring_test.go#L326). **negative:** `unit/verify` [`TestBlackholeNotStampedOutsideAuthorization`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_blackhole_wiring_test.go#L186). **positive:** `interop/nightly` [`checkRFC7999Blackhole`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L762). **negative:** `interop/nightly` [`checkRFC7999Blackhole`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L763) |
| `RFC7999-3.3-2` | A BGP speaker in a bilateral peering relationship using BLACKHOLE MUST only accept and honor an announcement carrying BLACKHOLE when the receiving party agreed to honor the BLACKHOLE community on that particular BGP session (§3.3) | MUST | 3.3 - Accepting Blackholed IP Prefixes | **positive:** `unit/verify` [`TestBlackholeCommunityRewritesNextHopWhenAgreed`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/filter_modify/match_test.go#L170). **positive:** `unit/verify` [`TestBlackholeGroupIdentityArrivesOnAStructuredEvent`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_blackhole_dynamic_test.go#L34). **positive:** `unit/verify` [`TestBlackholeRouteTypeStampedOnBestPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_blackhole_wiring_test.go#L107). **positive:** `unit/verify` [`TestBlackholeRuleForDynamicGroup`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_blackhole_wiring_test.go#L329). **negative:** `unit/verify` [`TestBlackholeCommunityLeavesNextHopAloneWithoutAgreement`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/filter_modify/match_test.go#L198). **negative:** `unit/verify` [`TestBlackholeNotStampedWhenNoCommunityAgreedButAuthorizationExists`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_blackhole_wiring_test.go#L158). **negative:** `unit/verify` [`TestBlackholeRuleForDynamicGroup`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_blackhole_wiring_test.go#L359). **positive:** `interop/nightly` [`checkRFC7999Blackhole`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L764). **negative:** `interop/nightly` [`checkRFC7999Blackhole`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L765) |
| `RFC7999-3.3-4` | "An operator MUST ensure that origin validation techniques (such as the one described in [RFC6811]) do not inadvertently block legitimate announcements carrying the BLACKHOLE community" (§3.3) | MUST | 3.3 - Accepting Blackholed IP Prefixes | **positive:** `unit/verify` [`TestBlackholeSurvivesLengthOnlyInvalid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/blackhole_decision_test.go#L60). **negative:** `unit/verify` [`TestBlackholeDoesNotSurviveAWrongOrigin`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/blackhole_decision_test.go#L80) |
| `RFC7999-3.1-3` | "The community SHOULD be ignored, if it is received by a network that it [sic] not using it" (§3.1) | SHOULD | 3.1 - IP Prefix Announcements with BLACKHOLE Community Attached | **positive:** no positive test. **negative:** no negative test |
| `RFC7999-3.1-5` | A network that announces a prefix covering the victim addresses under DDoS duress SHOULD attach the BLACKHOLE community (§3.1) | SHOULD | 3.1 - IP Prefix Announcements with BLACKHOLE Community Attached | **positive:** no positive test. **negative:** no negative test |
| `RFC7999-3.2-1` | "A BGP speaker receiving an announcement tagged with the BLACKHOLE community SHOULD add the NO_ADVERTISE or NO_EXPORT community as defined in [RFC1997], or a similar community, to prevent propagation of the prefix outside the local AS" (§3.2) | SHOULD | 3.2 - Local Scope of Blackholes | **positive:** `unit/verify` [`TestRFC7999BlackholeFieldHasReader`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/filter_community/blackhole_test.go#L42). **negative:** no negative test. **positive:** `functional/verify` [`community-blackhole-noexport.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/community-blackhole-noexport.ci#L6) |
| `RFC7999-3.2-2` | "The community to prevent propagation SHOULD be chosen according to the operator's routing policy" (§3.2) | SHOULD | 3.2 - Local Scope of Blackholes | **positive:** `unit/verify` [`TestBlackholeGuardNoAdvertise`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/filter_community/blackhole_test.go#L63). **negative:** no negative test |
| `RFC7999-3.3-3` | In topologies with a route server or other multilateral peering relationships, BGP speakers SHOULD accept and honor BGP announcements under the same two conditions that bind a bilateral speaker (§3.3) | SHOULD | 3.3 - Accepting Blackholed IP Prefixes | **positive:** no positive test. **negative:** no negative test |
| `RFC7999-4-1` | "Without an explicit configuration directive set by the operator, network elements SHOULD NOT discard traffic destined towards IP prefixes that are tagged with the BLACKHOLE community" (§4) | SHOULD NOT | 4 - Vendor Implementation Recommendations | **positive:** `unit/verify` [`TestBlackholeNotStampedWhenNoCommunityAgreedButAuthorizationExists`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_blackhole_wiring_test.go#L161). **positive:** `unit/verify` [`TestBlackholeNotStampedWithoutAgreement`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_blackhole_wiring_test.go#L135). **negative:** no negative test |
| `RFC7999-6-1` | The receiving BGP speaker SHOULD verify by applying strict filtering (RFC 7454 Section 6.2.1.1.2) that the peer announcing the prefix is authorized to do so (§6) | SHOULD | 6 - Security Considerations | **positive:** no positive test. **negative:** no negative test |
| `RFC7999-6-2` | "It is RECOMMENDED that operators use best common practices to protect their BGP sessions, such as the ones in [RFC7454]" (§6) | RECOMMENDED | 6 - Security Considerations | **positive:** no positive test. **negative:** no negative test |
| `RFC7999-3.1-1` | "This community MAY be used in all bilateral and multilateral BGP deployment scenarios" (§3.1) | MAY | 3.1 - IP Prefix Announcements with BLACKHOLE Community Attached | **positive:** no positive test. **negative:** no negative test |
| `RFC7999-3.1-4` | A network under DDoS duress MAY announce an IP prefix covering the victim's IP addresses, to signal to neighboring networks that traffic destined for those addresses is to be discarded (§3.1) | MAY | 3.1 - IP Prefix Announcements with BLACKHOLE Community Attached | **positive:** no positive test. **negative:** no negative test |
| `RFC7999-3.1-6` | "The BLACKHOLE community MAY also be used as one of the trigger communities in a destination-based Remote Triggered Blackhole (RTBH) [RFC5635] configuration" (§3.1) | MAY | 3.1 - IP Prefix Announcements with BLACKHOLE Community Attached | **positive:** no positive test. **negative:** no negative test |
| `RFC7999-4-2` | Vendors MAY provide a shorthand keyword in their configuration language for the well-known BLACKHOLE community value. The suggested string is "blackhole" (§4) | MAY | 4 - Vendor Implementation Recommendations | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

RFC 7999 declares no gap, and every gated MUST it carries has a test bound to it.

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC7999-3.1-2`](#rfc7999-3.1-2)

"In a bilateral peering relationship, use of the BLACKHOLE community MUST be agreed upon by the two networks before advertising it" (§3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestAnnounceBlackholeIsWithheldFromAPeerThatDidNotAgree`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/cmd/announce/blackhole_agreement_test.go#L110) | unit/verify | unproven |
| negative | [`TestAnnounceBlackholeIsWithheldFromASessionOutsideTheGroup`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/cmd/announce/blackhole_agreement_test.go#L300) | unit/verify | unproven |
| positive | [`TestAnnounceBlackholeReachesAMemberOfADynamicGroup`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/cmd/announce/blackhole_agreement_test.go#L272) | unit/verify | unproven |
| positive | [`TestAnnounceBlackholeReachesAnAgreedPeer`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/cmd/announce/blackhole_agreement_test.go#L96) | unit/verify | unproven |

### [`RFC7999-3.3-1`](#rfc7999-3.3-1)

A BGP speaker in a bilateral peering relationship using BLACKHOLE MUST only accept and honor an announcement carrying BLACKHOLE when the announced prefix is covered by an equal or shorter prefix that the neighboring network is authorized to advertise (§3.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestBlackholeNotStampedOutsideAuthorization`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_blackhole_wiring_test.go#L186) | unit/verify | unproven |
| negative | [`checkRFC7999Blackhole`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L763) | interop/nightly | unproven |
| positive | [`TestBlackholeGroupIdentityArrivesOnAStructuredEvent`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_blackhole_dynamic_test.go#L31) | unit/verify | unproven |
| positive | [`TestBlackholeRouteTypeStampedOnBestPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_blackhole_wiring_test.go#L104) | unit/verify | unproven |
| positive | [`TestBlackholeRuleForDynamicGroup`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_blackhole_wiring_test.go#L326) | unit/verify | unproven |
| positive | [`checkRFC7999Blackhole`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L762) | interop/nightly | unproven |

### [`RFC7999-3.3-2`](#rfc7999-3.3-2)

A BGP speaker in a bilateral peering relationship using BLACKHOLE MUST only accept and honor an announcement carrying BLACKHOLE when the receiving party agreed to honor the BLACKHOLE community on that particular BGP session (§3.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestBlackholeCommunityLeavesNextHopAloneWithoutAgreement`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/filter_modify/match_test.go#L198) | unit/verify | unproven |
| negative | [`TestBlackholeNotStampedWhenNoCommunityAgreedButAuthorizationExists`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_blackhole_wiring_test.go#L158) | unit/verify | unproven |
| negative | [`TestBlackholeRuleForDynamicGroup`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_blackhole_wiring_test.go#L359) | unit/verify | unproven |
| negative | [`checkRFC7999Blackhole`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L765) | interop/nightly | unproven |
| positive | [`TestBlackholeCommunityRewritesNextHopWhenAgreed`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/filter_modify/match_test.go#L170) | unit/verify | unproven |
| positive | [`TestBlackholeGroupIdentityArrivesOnAStructuredEvent`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_blackhole_dynamic_test.go#L34) | unit/verify | unproven |
| positive | [`TestBlackholeRouteTypeStampedOnBestPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_blackhole_wiring_test.go#L107) | unit/verify | unproven |
| positive | [`TestBlackholeRuleForDynamicGroup`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_blackhole_wiring_test.go#L329) | unit/verify | unproven |
| positive | [`checkRFC7999Blackhole`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L764) | interop/nightly | unproven |

### [`RFC7999-3.3-4`](#rfc7999-3.3-4)

"An operator MUST ensure that origin validation techniques (such as the one described in [RFC6811]) do not inadvertently block legitimate announcements carrying the BLACKHOLE community" (§3.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestBlackholeDoesNotSurviveAWrongOrigin`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/blackhole_decision_test.go#L80) | unit/verify | unproven |
| positive | [`TestBlackholeSurvivesLengthOnlyInvalid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/blackhole_decision_test.go#L60) | unit/verify | unproven |

### [`RFC7999-3.2-1`](#rfc7999-3.2-1)

"A BGP speaker receiving an announcement tagged with the BLACKHOLE community SHOULD add the NO_ADVERTISE or NO_EXPORT community as defined in [RFC1997], or a similar community, to prevent propagation of the prefix outside the local AS" (§3.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC7999BlackholeFieldHasReader`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/filter_community/blackhole_test.go#L42) | unit/verify | unproven |
| positive | [`community-blackhole-noexport.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/community-blackhole-noexport.ci#L6) | functional/verify | unproven |

### [`RFC7999-3.2-2`](#rfc7999-3.2-2)

"The community to prevent propagation SHOULD be chosen according to the operator's routing policy" (§3.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestBlackholeGuardNoAdvertise`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/filter_community/blackhole_test.go#L63) | unit/verify | unproven |

### [`RFC7999-4-1`](#rfc7999-4-1)

"Without an explicit configuration directive set by the operator, network elements SHOULD NOT discard traffic destined towards IP prefixes that are tagged with the BLACKHOLE community" (§4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestBlackholeNotStampedWhenNoCommunityAgreedButAuthorizationExists`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_blackhole_wiring_test.go#L161) | unit/verify | unproven |
| positive | [`TestBlackholeNotStampedWithoutAgreement`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_blackhole_wiring_test.go#L135) | unit/verify | unproven |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | ze-implement agent, spec-bcp194-6-blackhole enrolment step |
| Signed off | 2026-08-13 |
| Register | prose |
| Source | rfc/full/rfc7999.txt |
| Source fingerprint | f9768bb1d175830a |
| Record | rfc/extraction/rfc7999.json |
| Mapped sentences | 3 |
| Declined as scope | 1 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 1 | walked | Title block, Abstract, Status of This Memo, Copyright Notice and Table of Contents. Walked because the site scan attributes one site here. That site is the IETF Trust Legal Provisions boilerplate and is excluded below. |
| `1` | Introduction | 0 | walked | Introduction. States the operational problem: each network triggers blackholing by a different mechanism, so an operator must learn the mechanism of every neighbor. Gives the reason for one well-known community. No sentence directs a speaker. |
| `1.1` | Requirements Language | 0 | walked | Requirements Language. Restricts the RFC 2119 key words to their upper-case form and states that the lower-case forms are English words without normative meaning. It binds how the document is read and states no obligation of its own. The summary records the restriction, because sections 3.2, 3.3 and 6 each carry a lower-case 'should' that must not be read as a requirement. |
| `2` | BLACKHOLE Community | 0 | walked | BLACKHOLE Community. Defines the community as well-known, transitive and advisory, and states its semantics: the presence of the community asks the receiver to drop traffic sent toward the prefix. Every sentence is declarative. The registered value is stated in section 5. |
| `3` | Operational Recommendations | 0 | walked | Operational Recommendations. Section heading only. The three subsections carry all of the text. |
| `3.1` | IP Prefix Announcements with BLACKHOLE Community Attached | 1 | walked | IP Prefix Announcements with BLACKHOLE Community Attached. One MUST-level site, mapped below. The section also states that accepting and honoring the community or ignoring it is each operator's choice, and carries one MAY on deployment scenarios, one SHOULD on ignoring the community, one MAY and one SHOULD on announcing under DDoS duress, and one MAY on RTBH trigger communities. Captured as RFC7999-3.1-1 through RFC7999-3.1-6. The multilateral sentence states that the decision follows the operator's routing policy and is recorded as guidance without a keyword. |
| `3.2` | Local Scope of Blackholes | 0 | walked | Local Scope of Blackholes. No MUST-level site: both obligations are SHOULD. The receiver adds NO_ADVERTISE, NO_EXPORT or a similar community to prevent propagation outside the local AS, and chooses which one by routing policy. Captured as RFC7999-3.2-1 and RFC7999-3.2-2. The closing caution about propagating a tagged prefix outside the local routing domain uses a lower-case 'should' and is recorded as guidance without a keyword. |
| `3.3` | Accepting Blackholed IP Prefixes | 2 | walked | Accepting Blackholed IP Prefixes. Two MUST-level sites, both mapped below. The first states one obligation with two bullet conditions, and the summary captures each bullet as its own requirement because each is independently violable: a speaker can honor an uncovered prefix while the session agreement holds, and can honor a covered prefix on a session that agreed to nothing. The site scan sees the lead-in sentence alone, so the second bullet has no site of its own and RFC7999-3.3-2 is recorded as unsourced here. The section also carries one SHOULD extending both conditions to multilateral topologies (RFC7999-3.3-3), the RFC 7606 pointer that makes a malformed community treat-as-withdraw, and four lower-case statements recorded as guidance without a keyword. |
| `4` | Vendor Implementation Recommendations | 0 | walked | Vendor Implementation Recommendations. No MUST-level site. One SHOULD NOT states that a network element does not discard traffic toward a tagged prefix without an explicit configuration directive, and one MAY offers a configuration shorthand whose suggested string is 'blackhole'. Captured as RFC7999-4-1 and RFC7999-4-2. |
| `5` | IANA Considerations | 0 | skipped (iana) | IANA Considerations. Records that IANA has registered BLACKHOLE as 0xFFFF029A in the 'BGP Well-known Communities' registry, and that the low-order two octets are 666 in decimal. The sentence reports a completed registry action and binds IANA. The value it records is carried in the summary's Constants and Encoding tables, which state a codepoint rather than an obligation. |
| `6` | Security Considerations | 0 | walked | Security Considerations. No MUST-level site. One SHOULD asks the receiving speaker to verify by strict filtering that the announcing peer is authorized for the prefix, and one RECOMMENDED asks operators to protect their BGP sessions. Captured as RFC7999-6-1 and RFC7999-6-2. The rest is threat analysis: a forwarding agent can alter, add or remove communities, recipients cannot detect the change, and BGPsec does not resolve it. Two lower-case sentences, on filtering an unauthorized announcement and on the validation method following routing policy, are recorded as guidance without a keyword. |
| `7` | References heading | 0 | skipped (references) | References heading. |
| `7.1` | not stated | 0 | skipped (references) | Normative References: RFC 1997, RFC 2119, RFC 4271 and RFC 7606. |
| `7.2` | not stated | 0 | skipped (references) | Informative References: the BGPsec protocol draft, RFC 3882, RFC 5575, RFC 5635, RFC 6811 and RFC 7454. |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `front:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | The IETF Trust Legal Provisions boilerplate of the Copyright Notice. Its 'must' is lower case, which section 1.1 puts outside the normative set, and it binds a party that extracts Code Components from the document rather than a BGP speaker. RFC 7999 contains no code component: it defines one codepoint and no syntax. | Code Components extracted from this document must include Simplified BSD License text as described in Section 4.e of the Trust Legal Provisions and are provided without warranty as described in the Simplified BSD License. |

## Superseded

No document obsoletes RFC 7999, so its obligations are stated where they were written.
