# RFC 8950 - Advertising IPv4 Network Layer Reachability Information (NLRI) with an IPv6 Next Hop

Extended Next Hop. Every requirement listed for RFC 8950, the tests linked to it, and their recorded reviews. This summary is included in the automatic compliance check, ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 3 | of 6 mandatory requirements | a positive test checks that Ze does what the requirement demands and a negative one checks that it refuses what the requirement forbids |
| One polarity plus reason | 3 | of 6 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason supplies the complete test coverage required |
| One polarity, unexcused | 0 | of 6 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial test coverage | 0 | of 6 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unverified, with zero whole-requirement credit |
| Declared gap or no test | 0 | of 6 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |
| Tests shown to detect a fault | 20 | 20 of 28 tests linked to requirements | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests verifies a requirement only after both tests have been shown to detect a fault |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 6 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 0 | of 6 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 0 | of 6 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze verified: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 6 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 6 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no test verification credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level) | 6 | of 8 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements included in the automatic checks: the compliance check (./le rfc check) holds Ze to each of them. This is the total being assessed; the counts above show the recorded status of those requirements. |

The 9 counts marked as a part above are the whole of the 6 mandatory requirements: they add up to 6. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got. An outlined card with no colored edge, under Totals, is a total the other cards are counted against, not a measure.

| Card | Tone here | Why that color |
|---|---|---|
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the count under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: one test direction is missing without a recorded reason |
| Partial test coverage | ok | green at zero, RED above it: testing one clause does not verify the whole requirement |
| Declared gap or no test | ok | green at zero, RED above it: a binding obligation that nothing shows Ze meets is owed, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green counts above are what says how much Ze's own tests verify |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Linux cannot do this | neutral | neutral at every value: an evidenced platform limit is neither test verification nor a scope exemption |
| Tests shown to detect a fault | ok | green at every value: these tests have been shown to detect a deliberate fault. This card counts individual tests separately, so its total cannot be added to the requirement counts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a mandatory requirement is unreviewed, green when every one is judged sound and current |
| Mandatory requirements (MUST level) | outlined | outlined, with no colored edge: it is the total the other cards are counted against, not a measure, and a larger one is neither good news nor bad |

## At a glance

| Field | Value |
|---|---|
| Public row | Extended Next Hop |
| Automatic checks | Included |
| Requirements | 8 |
| Mandatory requirements (MUST level) | 6 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 28 |
| Tests linked to requirements | 28 |
| Recorded audit verdicts | 4 |
| Fault test records | 20 |
| Summary | `rfc/short/rfc8950.md` |
| Requirement table | `rfc/requirements/rfc8950.md` |
| RFC text | `rfc/full/rfc8950.txt` |

## Automatic checks

Included in automatic checks: Advertising IPv4 NLRI with an IPv6 Next Hop (extended next-hop encoding, capability code 5): six MUST-level requirements, all met. 4-1 (cross-family IPv6 next-hop honored only when the extended-next-hop capability is negotiated), 4-2 (a tuple is negotiated only if both peers advertise the same NLRI-AFI/SAFI/next-hop-AFI), and 3-1 (a 16- or 32-octet next-hop is decoded as IPv6 regardless of the NLRI AFI) carry positive+negative tags. 4-3 (capability code 5), 3-2 (VPN next-hop carries an 8-octet RD set to zero), and 5-1 (a reflected next-hop encoding is not rewritten) are {single-polarity: positive}, bound to code-constant, RD-zeroing, and reflection-byte-identical tests in internal/core/bgp/attribute and internal/component/bgp/reactor.

## Published support status

**Area:** Extended Next Hop

**Test verification:** 6 of 6 MUST proven

**Evidence:** unit 6

**Unmet:** -


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 3 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 3 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 0 | one part of the mandatory requirements |
| Partial test coverage (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level)** | **6** | every mandatory requirement appears in exactly one group above |

**Positive and negative tests (3):** [`RFC8950-4-1`](#rfc8950-4-1), [`RFC8950-4-2`](#rfc8950-4-2), [`RFC8950-3-1`](#rfc8950-3-1)

**Annotated (including scoped evidence) (3):** [`RFC8950-4-3`](#rfc8950-4-3), [`RFC8950-5-1`](#rfc8950-5-1), [`RFC8950-3-2`](#rfc8950-3-2)

## Figures

- **MUST bar**: Ze verified 6
- **Overview bar**: Ze MUST verified 6, Not assessed 2

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | complete |
| MUST rows | 6 |
| MUST rows kept (the MUST bar) | 6 |
| Ze verified | 6 |
| OS verified | 0 |
| Tests awaiting verification | 0 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 0 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 2 |
| SHOULD verified | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD tests awaiting verification | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 2 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze verified means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Tests awaiting verification counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

Source: [data/rfc/rfc8950.json](../../../data/rfc/rfc8950.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC8950-4-1` | A BGP speaker MUST only advertise the IPv4 or VPN-IPv4 NLRI with an IPv6 next hop to a BGP peer if the BGP speaker has first ascertained via the BGP Capability Advertisement that the BGP peer supports the Extended Next Hop Encoding capability for the relevant AFI/SAFI pair. (§4) | MUST | 4 - Use of BGP Capability Advertisement | **positive:** `unit/verify` [`TestCanUseNextHopFor_ExtendedNH`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_test.go#L1280). **positive:** `unit/verify` [`TestDefaultNextHopIPv6RequiresExactExtendedPair`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/default_next_hop_controls_test.go#L126). **positive:** `unit/verify` [`TestFamilyNextHopOriginationAdmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/next_hop_family_admission_test.go#L37). **positive:** `unit/verify` [`TestRFC8950BatchRailIPv4UnicastIPv6NextHopUsesMPReach`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/reactor_a_rfc8950_explicit_nexthop_test.go#L137). **positive:** `unit/verify` [`TestRFC8950ExplicitIPv6NextHopFollowsTheNegotiatedPair`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/reactor_a_rfc8950_explicit_nexthop_test.go#L42). **positive:** `unit/verify` [`TestRFC8950ForwardWithholdsIPv6NextHopFromPeerLackingThePair`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8950_reactor_a2_forward_test.go#L211). **positive:** `unit/verify` [`TestRFC8950IPv6NextHopFollowsTheNegotiatedPair`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8950_nexthop_pair_test.go#L36). **positive:** `unit/verify` [`TestStaticGroupedResolvedNextHopWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_static_group_next_hop_test.go#L30). **negative:** `unit/verify` [`TestCanUseNextHopFor_CrossFamilyNoCap`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_test.go#L1311). **negative:** `unit/verify` [`TestCanUseNextHopFor_NilSendCtx`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_test.go#L1335). **negative:** `unit/verify` [`TestDefaultNextHopIPv6RequiresExactExtendedPair`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/default_next_hop_controls_test.go#L129). **negative:** `unit/verify` [`TestDefaultNextHopLinkLocalCandidateUsesSessionPolicy`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8950_default_link_local_test.go#L18). **negative:** `unit/verify` [`TestFamilyNextHopOriginationAdmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/next_hop_family_admission_test.go#L39). **negative:** `unit/verify` [`TestRFC8950ExplicitIPv6NextHopFollowsTheNegotiatedPair`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/reactor_a_rfc8950_explicit_nexthop_test.go#L43). **negative:** `unit/verify` [`TestRFC8950ForwardWithholdsIPv6NextHopFromPeerLackingThePair`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8950_reactor_a2_forward_test.go#L212). **negative:** `unit/verify` [`TestRFC8950IPv6NextHopFollowsTheNegotiatedPair`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8950_nexthop_pair_test.go#L37) |
| `RFC8950-4-2` | A BGP speaker that wishes to advertise an IPv6 next hop for IPv4 NLRI or for VPN-IPv4 NLRI to a BGP peer as per this specification MUST use the Capability Advertisement procedures defined in [RFC5492] with the Extended Next Hop Encoding capability to determine whether its peer supports this for the NLRI AFI/SAFI pair(s) of interest. (§4) | MUST | 4 - Use of BGP Capability Advertisement | **positive:** `unit/verify` [`TestNegotiateExtendedNextHop`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/negotiated_test.go#L324). **positive:** `unit/verify` [`TestRFC8950PeerSupportDecidedPerPair`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/rfc8950_extnh_negotiation_test.go#L61). **negative:** `unit/verify` [`TestNegotiateExtendedNextHopMismatch`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/negotiated_test.go#L363). **negative:** `unit/verify` [`TestRFC8950PeerWithoutThePairGetsNoIPv6NextHop`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/rfc8950_extnh_negotiation_test.go#L75) |
| `RFC8950-4-3` | The Capability Code field MUST be set to 5 (§4) | MUST | 4 - Use of BGP Capability Advertisement | **positive:** `unit/verify` [`TestExtendedNextHopCapability`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L558). **positive:** `unit/verify` [`TestExtendedNextHopRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L591). **negative:** no negative test. **{single-polarity}:** ze binds the Extended Next Hop Encoding capability to the constant CodeExtendedNextHop = 5 (internal/core/bgp/capability/capability.go:70); Code() returns it (capability.go:640) and WriteTo emits it (capability.go:644-646). The code is a fixed constant with no alternate-value code path, so there is no wrong-code case to reject as a negative |
| `RFC8950-3-1` | The BGP speaker receiving the advertisement MUST use the Length of Next Hop Address field to determine which network-layer protocol the next-hop address belongs to (§3) | MUST | 3 | **positive:** `unit/verify` [`TestParseMPReachNLRI_ExtendedNextHop`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/mpnlri_test.go#L338). **positive:** `unit/verify` [`TestParseMPReachNLRI_ExtendedNextHop_DualStack`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/mpnlri_test.go#L442). **negative:** `unit/verify` [`TestParseMPReachNLRI_InvalidNextHopLength`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/mpnlri_test.go#L488) |
| `RFC8950-5-1` | When a next-hop address needs to be passed along unchanged (e.g., as a Route Reflector (RR) would do), its encoding MUST NOT be changed. (§5) | MUST NOT | 5 - Operations | **positive:** `unit/verify` [`TestReactorForwardRRPreservesExtendedNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_rr_test.go#L252). **negative:** no negative test. **{single-polarity}:** on the reflection path ze rewrites the next-hop only under an explicit next-hop-self/explicit override (nhMode != nhModeNone); the default nhModeNone leaves the next-hop untouched (internal/component/bgp/reactor/peer_forward_facts.go:226-229) and the MP re-encode changes an attribute only when the NLRI framing differs between encoding contexts (internal/component/bgp/reactor/forward_body.go:217), so a reflected next-hop is carried verbatim and there is no ze code path that rewrites an unchanged-passthrough next-hop to assert as a negative. The positive is proven byte-identical in TestReactorForwardRRPreservesExtendedNextHop |
| `RFC8950-3-2` | Next Hop Address = VPN-IPv6 address of a next hop with an 8-octet RD set to zero (potentially followed by the link-local VPN-IPv6 address of the next hop with an 8-octet RD set to zero). (§3) | MUST | 3 | **positive:** `unit/verify` [`TestParseMPReachNLRI_VPNWithIPv6NextHop`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/mpnlri_test.go#L576). **positive:** `unit/verify` [`TestRFC8950VPNIPv6NextHopCarriesZeroRD`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc8950_vpn_nexthop_test.go#L27). **negative:** no negative test. **{single-polarity}:** on encode ze always writes an all-zero 8-byte Route Distinguisher before each VPN next-hop address, the link-local one included (internal/core/bgp/attribute/mpnlri.go, writeNextHops), and takes no RD as input; on decode it skips the 8 RD bytes without validating their value (mpnlri.go, parseVPNNextHops), which the sentence, a sender-side encoding, does not ask it to check; so ze never emits a nonzero RD and never rejects one -- there is no negative case |
| `RFC8950-5-2` | By default, if a particular BGP session is running over IPvx (where IPvx is IPv4 or IPv6) and if the BGP speaker sending an update is putting its own address in as the next hop, then the next-hop address SHOULD be specified as an IPvx address, using the encoding rules specified in the AFI/SAFI definition of the NLRI being updated. (§5) | SHOULD | 5 - Operations | **positive:** no positive test. **negative:** no negative test |
| `RFC8950-5-3` | This default behavior may be overridden by policy. (§5) | MAY | 5 - Operations | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

RFC 8950 lists no missing behavior, and every mandatory requirement included in automatic checks has a linked test.

## Test status

A test reads not verified when no record shows it failing after a deliberate fault. Until then, the test has not been shown to detect a fault in the required behavior.

### [`RFC8950-4-1`](#rfc8950-4-1)

A BGP speaker MUST only advertise the IPv4 or VPN-IPv4 NLRI with an IPv6 next hop to a BGP peer if the BGP speaker has first ascertained via the BGP Capability Advertisement that the BGP peer supports the Extended Next Hop Encoding capability for the relevant AFI/SAFI pair. (§4)

Audit verdict: enforced (the tests do what the requirement demands), stale-unit: internal/component/bgp/reactor/default_next_hop_controls_test.go::TestDefaultNextHopIPv6RequiresExactExtendedPair, internal/component/bgp/reactor/default_next_hop_controls_test.go::TestDefaultNextHopIPv6RequiresExactExtendedPair#2, internal/component/bgp/reactor/next_hop_family_admission_test.go::TestFamilyNextHopOriginationAdmission, internal/component/bgp/reactor/next_hop_family_admission_test.go::TestFamilyNextHopOriginationAdmission#2, internal/component/bgp/reactor/peer_static_group_next_hop_test.go::TestStaticGroupedResolvedNextHopWire, internal/component/bgp/reactor/rfc8950_default_link_local_test.go::TestDefaultNextHopLinkLocalCandidateUsesSessionPolicy, internal/component/bgp/reactor/rfc8950_reactor_a2_forward_test.go::TestRFC8950ForwardWithholdsIPv6NextHopFromPeerLackingThePair, internal/component/bgp/reactor/rfc8950_reactor_a2_forward_test.go::TestRFC8950ForwardWithholdsIPv6NextHopFromPeerLackingThePair#2 moved. Re-judged 2026-11-14 by an independent auditor who authored neither 08fe903803 nor the uncommitted phase 2 of spec-single-owner-lifecycle-2-bgp-session. RFC 8950 Section 4 read: "A BGP speaker MUST only advertise the IPv4 or VPN-IPv4 NLRI with an IPv6 next hop to a BGP peer if the BGP speaker has first ascertained via the BGP Capability Advertisement that the BGP peer supports the Extended Next Hop Encoding capability for the relevant AFI/SAFI pair." Units read: TestFamilyNextHopOriginationAdmission (both tags), TestStaticGroupedResolvedNextHopWire; only their sync calls and flag writes changed. 08fe903803 moved the initial-sync gate (sendingInitialRoutes) and the End-of-RIB owed mark (initialSyncEOROwed) from Peer onto Session. In the tagged units named stale it replaced peer.sendInitialRoutes() with peer.runInitialSyncNow(), which is Peer.sendInitialRoutes(p.testInitialSync()) (initial_sync_helpers_test.go): the same production sync body, run on the session the fixture installed (or one built with NewSession(p.settings) and published through establishedSession as enterEstablished does); it replaced peer.<flag>.Store(v) with peer.testInitialSync().<flag>.Store(v) or setTestInitialSync, and drainAndCloseQueueGate now takes that session. git diff 08fe903803^ against the working tree shows no assertion, expected byte, driven input or tag prose changed in any of them, each under the RFC-approved line 08fe903803 carries. Questions 1, 3 and 4 answer exactly as the earlier judgement below; question 2 still rests on the same producers, whose discrimination records are retaken by ./le rfc record after this stamp. Verdict unchanged: enforced. Earlier judgement follows. Re-judged 2026-11-13 against the current complete tagged population, adding TestDefaultNextHopIPv6RequiresExactExtendedPair positive/negative and TestDefaultNextHopLinkLocalCandidateUsesSessionPolicy negative. RFC 8950 Section 4: "A BGP speaker MUST only advertise the IPv4 or VPN-IPv4 NLRI with an IPv6 next hop to a BGP peer if the BGP speaker has first ascertained via the BGP Capability Advertisement that the BGP peer supports the Extended Next Hop Encoding capability for the relevant AFI/SAFI pair." Q1: yes, current tags assert the relevant pair rather than any capability or any IPv4-family permission. New direct and both actual-drain cases require exact IPv4/unicast MP_REACH next-hop/prefix bytes under the exact tuple, and no such announcement under no tuple or VPN-only, while an explicit IPv4 survivor remains. Internal Default gateway, external Default session self and explicit API under configured self use the same observed output contract. Existing self/explicit matrices retain unicast-versus-VPN resolver checks; existing API, queued, commit, post-policy and static-group tests observe actual output, and general/RS controls withhold unlicensed IPv6 reachability while retaining withdrawals. Q2: yes, exact wire and count assertions reject over-admission or blanket refusal. Current new mutant records and ROOT's retained native outputs show clean GREEN then queued announcements=1,want2 when Negotiate's exact tuple is removed, and announcements=2,want1 in no-pair/VPN-only cases when an unadvertised tuple is injected. The link-local record likewise shows actual queued extra announcements beside the IPv4 survivor after its explicitly compound tuple-plus-capability-77 state forgery. Direct error-class failures alone are not used as normative emitted-wire proof. Existing whole-producer revert records retain their producer-reach scope. Q3: the global-unicast exact-pair cases isolate this rule: valid address roles, MP family and useful IPv4 survivor leave no capability-77 or link-local gate to hide tuple mistakes. The new link-local carrier is supplementary: absence of both permissions has two independent refusals, so its compound record does not prove the capability-5 guard alone, capability-77 negotiation or draft conformance. Existing TestFamilyNextHopOriginationAdmission separately checks capability 77 without required capability 5, and valid negotiated-pair output. Q4: the collective tagged population continues to enforce the exact-pair advertisement condition on the implemented origination and forwarding boundaries reviewed. No new positive VPN socket/forwarding proof, actual OPEN-exchange proof or capability-77 proof is claimed; the VPN positive remains the existing explicit/self resolver scope. RFC 8950 Section 4 also states that allowed AFI/SAFI is determined separately by the Multiprotocol Extensions capability, and the new negotiation fixture includes IPv4/unicast. Producer review: Negotiate intersects complete triples before publishing composite encoding facts; resolveNextHop/canUseNextHopFor check the selected address against that family; writeUpdateGated rechecks final post-policy MP wire against session.negotiated; egressNextHopLacksExtendedNextHop checks the final forwarding address under the same family scope. SR Policy and independently defined mapped forms remain separate contracts. Verdict unchanged. This auditor authored the original untagged default controls; this is a requirement rejudgment, not an independent weakening approval. ROOT owns audit-stamp and the final check.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestDefaultNextHopIPv6RequiresExactExtendedPair`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/default_next_hop_controls_test.go#L129) | unit/verify | mutant, verified |
| negative | [`TestFamilyNextHopOriginationAdmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/next_hop_family_admission_test.go#L39) | unit/verify | revert, verified |
| negative | [`TestCanUseNextHopFor_CrossFamilyNoCap`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_test.go#L1311) | unit/verify | not verified |
| negative | [`TestCanUseNextHopFor_NilSendCtx`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_test.go#L1335) | unit/verify | not verified |
| negative | [`TestRFC8950ExplicitIPv6NextHopFollowsTheNegotiatedPair`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/reactor_a_rfc8950_explicit_nexthop_test.go#L43) | unit/verify | revert, verified |
| negative | [`TestDefaultNextHopLinkLocalCandidateUsesSessionPolicy`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8950_default_link_local_test.go#L18) | unit/verify | mutant, verified |
| negative | [`TestRFC8950IPv6NextHopFollowsTheNegotiatedPair`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8950_nexthop_pair_test.go#L37) | unit/verify | revert, verified |
| negative | [`TestRFC8950ForwardWithholdsIPv6NextHopFromPeerLackingThePair`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8950_reactor_a2_forward_test.go#L212) | unit/verify | revert, verified |
| positive | [`TestDefaultNextHopIPv6RequiresExactExtendedPair`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/default_next_hop_controls_test.go#L126) | unit/verify | mutant, verified |
| positive | [`TestFamilyNextHopOriginationAdmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/next_hop_family_admission_test.go#L37) | unit/verify | revert, verified |
| positive | [`TestStaticGroupedResolvedNextHopWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_static_group_next_hop_test.go#L30) | unit/verify | revert, verified |
| positive | [`TestCanUseNextHopFor_ExtendedNH`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_test.go#L1280) | unit/verify | not verified |
| positive | [`TestRFC8950BatchRailIPv4UnicastIPv6NextHopUsesMPReach`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/reactor_a_rfc8950_explicit_nexthop_test.go#L137) | unit/verify | revert, verified |
| positive | [`TestRFC8950ExplicitIPv6NextHopFollowsTheNegotiatedPair`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/reactor_a_rfc8950_explicit_nexthop_test.go#L42) | unit/verify | revert, verified |
| positive | [`TestRFC8950IPv6NextHopFollowsTheNegotiatedPair`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8950_nexthop_pair_test.go#L36) | unit/verify | revert, verified |
| positive | [`TestRFC8950ForwardWithholdsIPv6NextHopFromPeerLackingThePair`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8950_reactor_a2_forward_test.go#L211) | unit/verify | revert, verified |

### [`RFC8950-4-2`](#rfc8950-4-2)

A BGP speaker that wishes to advertise an IPv6 next hop for IPv4 NLRI or for VPN-IPv4 NLRI to a BGP peer as per this specification MUST use the Capability Advertisement procedures defined in [RFC5492] with the Extended Next Hop Encoding capability to determine whether its peer supports this for the NLRI AFI/SAFI pair(s) of interest. (§4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRFC8950PeerSupportDecidedPerPair / ...PeerWithoutThePairGetsNoIPv6NextHop drive Negotiate with both sides' Extended Next Hop capabilities and read the per-pair answer through Negotiated and EncodingCaps alike: both list the pair -> AFI 2; peer lacks the pair, lists another NH AFI, or sends no capability -> 0. Judge breaks: intersection ignoring the NH AFI, and intersection enabling every local pair, each turned the negative red.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestNegotiateExtendedNextHopMismatch`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/negotiated_test.go#L363) | unit/verify | not verified |
| negative | [`TestRFC8950PeerWithoutThePairGetsNoIPv6NextHop`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/rfc8950_extnh_negotiation_test.go#L75) | unit/verify | revert, verified |
| positive | [`TestNegotiateExtendedNextHop`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/negotiated_test.go#L324) | unit/verify | not verified |
| positive | [`TestRFC8950PeerSupportDecidedPerPair`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/rfc8950_extnh_negotiation_test.go#L61) | unit/verify | revert, verified |

### [`RFC8950-4-3`](#rfc8950-4-3)

The Capability Code field MUST be set to 5 (§4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestExtendedNextHopCapability`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L558) | unit/verify | not verified |
| positive | [`TestExtendedNextHopRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/capability_test.go#L591) | unit/verify | revert, verified |

### [`RFC8950-3-1`](#rfc8950-3-1)

The BGP speaker receiving the advertisement MUST use the Length of Next Hop Address field to determine which network-layer protocol the next-hop address belongs to (§3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestParseMPReachNLRI_InvalidNextHopLength`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/mpnlri_test.go#L488) | unit/verify | revert, verified |
| positive | [`TestParseMPReachNLRI_ExtendedNextHop`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/mpnlri_test.go#L338) | unit/verify | revert, verified |
| positive | [`TestParseMPReachNLRI_ExtendedNextHop_DualStack`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/mpnlri_test.go#L442) | unit/verify | not verified |

### [`RFC8950-5-1`](#rfc8950-5-1)

When a next-hop address needs to be passed along unchanged (e.g., as a Route Reflector (RR) would do), its encoding MUST NOT be changed. (§5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. MUST NOT change the encoding of a next hop passed along unchanged. The non-compliant behaviour is the RR re-encoding the 16-byte IPv6 next hop. TestReactorForwardRRPreservesExtendedNextHop (reactor/forward_rr_test.go) asserts require.Equal(mpReach, attrs[14]) and the next-hop bytes 4:20, so any re-encoding goes red. Single-polarity positive per the row's marker.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestReactorForwardRRPreservesExtendedNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_rr_test.go#L252) | unit/verify | revert, verified |

### [`RFC8950-3-2`](#rfc8950-3-2)

Next Hop Address = VPN-IPv6 address of a next hop with an 8-octet RD set to zero (potentially followed by the link-local VPN-IPv6 address of the next hop with an 8-octet RD set to zero). (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-30. TestRFC8950VPNIPv6NextHopCarriesZeroRD writes AFI 1 SAFI 128 with an IPv6 next hop into a 0xFF-filled buffer: Length 24 = zero RD + global, Length 48 = zero RD + global + zero RD + link-local, and parses back to the same addresses; observed red on a break of mpnlri.go::writeNextHops. {single-polarity: positive} valid: a sender-side encoding, the encoder takes no RD input, so no non-zero RD can be produced or refused. TestParseMPReachNLRI_VPNWithIPv6NextHop is the decode-side neighbour. The RFC8950-3-2 tag on TestMPReachNLRI_RoundTrip_VPN was dropped by the judge (D-15): it encodes an IPv4 VPN next hop (RFC 4364) on a zeroed buffer and asserts no clause of this row.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestParseMPReachNLRI_VPNWithIPv6NextHop`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/mpnlri_test.go#L576) | unit/verify | not verified |
| positive | [`TestRFC8950VPNIPv6NextHopCarriesZeroRD`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc8950_vpn_nexthop_test.go#L27) | unit/verify | revert, verified |

## Requirement list review

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
| `1` | Introduction | 0 | reviewed | Introduction. Indicative prose: which AFI/SAFI pairs already let the next hop belong to another address family, that RFC 4798 and RFC 4659 solve the IPv6 NLRI with an IPv4 next hop direction, that no solution existed for the reverse direction, and what this document adds. The last sentence records that the document obsoletes RFC 5549. No sentence directs a speaker. |
| `1.1` | Requirements Language | 0 | reviewed | Requirements Language. The BCP 14 key-words paragraph of RFC 2119 and RFC 8174, which binds the key words only when they appear in all capitals. It tells a reader how to read the other sections and binds no speaker, which is why the derivation excludes it from the site inventory. |
| `2` | Changes Compared to RFC 5549 | 0 | reviewed | Changes Compared to RFC 5549. Two changes, both stated indicatively. The next hop for AFI/SAFI <1/128> is now encoded as a VPN-IPv6 address of 24 or 48 octets, in place of the 16 or 32 octets of RFC 5549, which answers Erratum ID 5253. AFI/SAFI <1/129> can now use an IPv6 underlay with the same encoding and procedures as <1/128>. Both sentences describe what sections 3, 6.2 and 6.3 define and direct no speaker. The lengths are carried by the MP_REACH_NLRI Next Hop Encoding tables of rfc/short/rfc8950.md. |
| `3` | not stated | 1 | reviewed | Extension of AFI/SAFI Definitions for the IPv4 Address Family. One capitalised MUST-level site, mapped below to RFC8950-3-1. The rest of the section is value assignment written as bullet lists: AFI 1 with SAFI 1, 2 or 4 takes a Length of Next Hop Address of 16 or 32 and the address is then of type IPv6, AFI 1 with SAFI 128 or 129 takes 24 or 48 and the address is then of type VPN-IPv6. Those bullets carry no RFC 2119 level, and the Wire Formats and Decoding Rules tables of rfc/short/rfc8950.md hold the values. One obligation inside them the site scan cannot see is the unsourced id below: the 8-octet Route Distinguisher of a VPN-IPv6 next hop is set to zero. A second bullet, "This field is to be constructed as per Section 3 of [RFC2545]", delegates the global and link-local construction to RFC 2545, which carries its own summary and its own sign-off. The closing note that RFC 4684 and RFC 6074 use the same length method is a comparison, not a directive. |
| `4` | Use of BGP Capability Advertisement | 4 | reviewed | Use of BGP Capability Advertisement. Four capitalised MUST-level sites: the RFC 5492 procedures (RFC8950-4-2), the lead-in to the field list (excluded as a duplicate below), Capability Code 5 (RFC8950-4-3), and the precondition that a speaker advertises an IPv6 next hop for IPv4 or VPN-IPv4 NLRI only after it has ascertained peer support for the relevant AFI/SAFI pair (RFC8950-4-1). The Capability Value format, its <NLRI AFI, NLRI SAFI, Nexthop AFI> triples and the values this document allows (NLRI AFI 1, NLRI SAFI 1, 2, 4, 128 or 129, Nexthop AFI 2) are value assignment, held by the Wire Formats and Constants tables of rfc/short/rfc8950.md. Three sentences the site scan cannot see are scoping rather than obligations: that this document does not specify the capability for any other combination, that new AFIs/SAFIs are not expected to use it, and that the capability says how a next hop is encoded while the Multiprotocol Extensions capability of RFC 4760 decides whether the AFI/SAFI is allowed at all. Each is indicative, so section 1.1 gives it no normative level. |
| `5` | Operations | 1 | reviewed | Operations. One capitalised MUST-level site, mapped below to RFC8950-5-1. The two remaining directives are the SHOULD to give the next hop as an IPvx address when the session runs over IPvx and the speaker puts its own address in, and the MAY that policy overrides that default. Both are the unsourced ids below. The closing sentences state a consequence rather than an obligation: an RR client that cannot handle an encoding, as determined by the BGP Capability Advertisement, does not get the NLRI, which is what RFC8950-4-1 already requires of the sender, and sound routing in some designs needs every RR client to handle whatever encodings any of them generate. |
| `6` | Usage Examples | 0 | reviewed | Usage Examples. A heading with no text of its own. Sections 6.1, 6.2 and 6.3 carry the three examples. |
| `6.1` | IPv4 over IPv6 Core | 0 | reviewed | IPv4 over IPv6 Core. An illustrative encoding for the interconnection of IPv4 islands over an IPv6 backbone as described in RFC 5565: AFI 1, SAFI 1, Length of Next Hop Address field 16 or 32, an IPv6 next hop, and the capability triple <NLRI AFI 1, NLRI SAFI 1, Nexthop AFI 2>. Written as "may be used" and "would include", so it directs no speaker; the values repeat sections 3 and 4. |
| `6.2` | IPv4 VPN Unicast over IPv6 Core | 0 | reviewed | IPv4 VPN Unicast over IPv6 Core. The same illustrative shape for SAFI 128: Length of Next Hop Address field 24 or 48, a VPN-IPv6 next hop whose Route Distinguisher is zero, and the capability triple <NLRI AFI 1, NLRI SAFI 128, Nexthop AFI 2>. This is the example section 2 points at for the encoding change against RFC 5549. No directive. |
| `6.3` | IPv4 VPN Multicast over IPv6 Core | 0 | reviewed | IPv4 VPN Multicast over IPv6 Core. The same illustrative shape for SAFI 129, which this document adds: Length of Next Hop Address field 24 or 48, a VPN-IPv6 next hop whose Route Distinguisher is zero, and the capability triple <NLRI AFI 1, NLRI SAFI 129, Nexthop AFI 2>. No directive. |
| `7` | IANA Considerations | 0 | skipped (iana) | IANA Considerations. Records that this document defines no new code point, and that IANA updated the "Extended Next Hop Encoding" entry of the Capability Codes registry, value 5, to refer to this document in place of RFC 5549. Binds IANA, not a speaker. |
| `8` | Security Considerations | 0 | reviewed | Security Considerations. States that the document raises no security issue beyond BGP-4 and the Multiprotocol Extensions, that the ability to advertise an IPv6 next hop widens the reach of the traffic diversion attacks RFC 4272 describes, and that the IPv6 next hop can be an IPv4-mapped IPv6 address of RFC 4291. The last point addresses the network operator who configures next-hop security checks, states no capitalised keyword, and directs no speaker. |
| `9` | References | 0 | skipped (references) | References. A heading whose two lists are sections 9.1 and 9.2. |
| `9.1` | not stated | 0 | skipped (references) | Normative References: RFC 2119, RFC 2545, RFC 4291, RFC 4364, RFC 4760, RFC 5492, RFC 8174 and RFC 8277. |
| `9.2` | not stated | 0 | skipped (references) | Informative References: Erratum ID 5253, the IANA Address Family Numbers, Capability Codes and SAFI Parameters registries, RFC 4272, RFC 4659, RFC 4684, RFC 4798, RFC 4925, RFC 5549, RFC 5565, RFC 6074, RFC 6513 and RFC 6514. The unnumbered Acknowledgments and Authors' Addresses that close the document fall into this section, because the derivation opens a section only on a numbered heading at column 0. Neither states an obligation. |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `4:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The lead-in to the field list of the Capabilities Optional Parameter. Of the three bullets it introduces, only the Capability Code bullet carries an RFC 2119 keyword, and site 4:3 maps it to RFC8950-4-3; the Capability Length bullet and the Capability Value format bullet are written indicatively. The sentence therefore restates the obligation site 4:3 already carries and adds no separate one. | The fields in the Capabilities Optional Parameter MUST be set as follows: |

## Superseded

No document obsoletes RFC 8950, so its obligations are stated where they were written.
