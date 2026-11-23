# RFC 7611 - BGP ACCEPT_OWN Community Attribute

ACCEPT_OWN community. Every requirement listed for RFC 7611, the tests linked to it, and their recorded reviews. This summary is included in the automatic compliance check, ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 5 | of 5 mandatory requirements | a positive test checks that Ze does what the requirement demands and a negative one checks that it refuses what the requirement forbids |
| One polarity plus reason | 0 | of 5 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason supplies the complete test coverage required |
| One polarity, unexcused | 0 | of 5 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial test coverage | 0 | of 5 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unverified, with zero whole-requirement credit |
| Declared gap or no test | 0 | of 5 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |
| Tests shown to detect a fault | 10 | 10 of 12 tests linked to requirements | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests verifies a requirement only after both tests have been shown to detect a fault |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 5 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 0 | of 5 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 0 | of 5 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze verified: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 5 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 5 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no test verification credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level) | 5 | of 8 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements included in the automatic checks: the compliance check (./le rfc check) holds Ze to each of them. This is the total being assessed; the counts above show the recorded status of those requirements. |

The 9 counts marked as a part above are the whole of the 5 mandatory requirements: they add up to 5. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

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
| Public row | ACCEPT_OWN community |
| Automatic checks | Included |
| Requirements | 8 |
| Mandatory requirements (MUST level) | 5 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 12 |
| Tests linked to requirements | 12 |
| Recorded audit verdicts | 4 |
| Fault test records | 10 |
| Summary | `rfc/short/rfc7611.md` |
| Requirement table | `rfc/requirements/rfc7611.md` |
| RFC text | `rfc/full/rfc7611.txt` |

## Automatic checks

Included in automatic checks: Ze recognizes ACCEPT_OWN, implements the Section 2.2 non-RD receive discard, exposes the `bgp accept-own` boolean configuration leaf defaulting to disabled (false) per Section 2.3, keeps Section 2.1 re-import disabled by default (normal loop checks remain active), and implements the Section 3 decision-process preference step strictly gated to RFC 4364 VPN-IP routes when configured.

## Published support status

**Area:** ACCEPT_OWN community

**Test verification:** 5 of 5 MUST proven

**Evidence:** unit 5

**Unmet:** -


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 5 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 0 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 0 | one part of the mandatory requirements |
| Partial test coverage (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level)** | **5** | every mandatory requirement appears in exactly one group above |

**Positive and negative tests (5):** [`RFC7611-2.1-1`](#rfc7611-2.1-1), [`RFC7611-2.2-1`](#rfc7611-2.2-1), [`RFC7611-2.3-1`](#rfc7611-2.3-1), [`RFC7611-2.3-2`](#rfc7611-2.3-2), [`RFC7611-3-1`](#rfc7611-3-1)

## Figures

- **MUST bar**: Ze verified 5
- **Overview bar**: Ze MUST verified 5, Not assessed 3

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | complete |
| MUST rows | 5 |
| MUST rows kept (the MUST bar) | 5 |
| Ze verified | 5 |
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
| SHOULD / MAY rows | 3 |
| SHOULD verified | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD tests awaiting verification | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 3 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze verified means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Tests awaiting verification counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

Source: [data/rfc/rfc7611.json](../../../data/rfc/rfc7611.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC7611-2.1-1` | A route MUST NOT ever be accepted back into its source VRF, even if it carries one or more RTs that match that VRF (§2.1) | MUST NOT | 2.1 | **positive:** `unit/verify` [`TestRFC7611OwnRouteNeverReacceptedIntoSource`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_rfc7611_ingress_test.go#L184). **negative:** `unit/verify` [`TestRFC7611OwnRouteNeverReacceptedIntoSource`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_rfc7611_ingress_test.go#L180) |
| `RFC7611-2.2-1` | Likewise, if a route carrying the ACCEPT_OWN community is received in an address family that does not allow the source VRF to be looked up, the ACCEPT_OWN community MUST be discarded. (§2.2) | MUST | 2.2 | **positive:** `unit/verify` [`TestACCEPTOwnFamilyBoundary`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7611_receive_test.go#L17). **positive:** `unit/verify` [`TestRFC7611AcceptOwnDiscardedForEveryNonRDFamilyInMPReach`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7611_reactor_b_test.go#L71). **negative:** `unit/verify` [`TestACCEPTOwnFamilyBoundary`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7611_receive_test.go#L18). **negative:** `unit/verify` [`TestRFC7611AcceptOwnDiscardedForEveryNonRDFamilyInMPReach`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7611_reactor_b_test.go#L72) |
| `RFC7611-2.2-2` | As such, it SHOULD NOT be attached to any routes that cannot be associated with a source VRF. This implies that when propagating routes into a VRF, the ACCEPT_OWN community SHOULD NOT be propagated. (§2.2) | SHOULD NOT | 2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7611-2.3-1` | ACCEPT_OWN handling SHOULD be controlled by configuration, and if controlled by configuration, it MUST default to being disabled (§2.3) | MUST | 2.3 | **positive:** `unit/verify` [`TestRFC7611AcceptOwnConfigDefaultsDisabled`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc7611_accept_own_test.go#L82). **negative:** `unit/verify` [`TestRFC7611AcceptOwnConfigDefaultsDisabled`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc7611_accept_own_test.go#L83) |
| `RFC7611-2.3-2` | When ACCEPT_OWN is disabled by configuration (either explicitly or by default), the router MUST NOT apply the special route acceptance rules detailed in Section 2.1. (§2.3) | MUST NOT | 2.3 | **positive:** `unit/verify` [`TestACCEPTOwnDoesNotEnableAcceptanceImplicitly`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7611_receive_test.go#L57). **negative:** `unit/verify` [`TestACCEPTOwnDoesNotEnableAcceptanceImplicitly`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7611_receive_test.go#L58) |
| `RFC7611-3-1` | This extra step MUST only be invoked during the best path selection process of VPN-IP routes [RFC4364] (i.e., it MUST NOT be invoked for the best path selection of imported IP routes in a VRF). (§3) | MUST | 3 | **positive:** `unit/verify` [`TestRFC7611DecisionStepOnlyForVPNIPRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc7611_accept_own_test.go#L164). **negative:** `unit/verify` [`TestRFC7611DecisionStepOnlyForVPNIPRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc7611_accept_own_test.go#L165) |
| `RFC7611-2-1` | Processing of the ACCEPT_OWN community SHOULD be controlled by configuration. The functionality SHOULD default to being disabled, as further specified in Section 2.3. (§2) | SHOULD | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7611-5-1` | This approach may also be relevant in other scenarios where a BGP speaker maintains multiple routing contexts using an approach different from that of [RFC4364], as long as the specific approach in use has the property that the BGP speaker originates and receives routes within a particular context. In such a case, "VRF" in this document should be understood to mean whatever construct provides a routing context in the specific technology under consideration. Likewise, "Route Distinguisher" should be understood to mean whatever construct allows a route's originator to associate that route with its source context, and "Route Target" should be understood to mean whatever construct allows a route to be targeted for import into a context other than its source. (§5) | SHOULD | 5 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

RFC 7611 lists no missing behavior, and every mandatory requirement included in automatic checks has a linked test.

## Test status

A test reads not verified when no record shows it failing after a deliberate fault. Until then, the test has not been shown to detect a fault in the required behavior.

### [`RFC7611-2.1-1`](#rfc7611-2.1-1)

A route MUST NOT ever be accepted back into its source VRF, even if it carries one or more RTs that match that VRF (§2.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC7611OwnRouteNeverReacceptedIntoSource`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_rfc7611_ingress_test.go#L180) | unit/verify | revert, verified |
| positive | [`TestRFC7611OwnRouteNeverReacceptedIntoSource`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc4271_rfc7611_ingress_test.go#L184) | unit/verify | revert, verified |

### [`RFC7611-2.2-1`](#rfc7611-2.2-1)

Likewise, if a route carrying the ACCEPT_OWN community is received in an address family that does not allow the source VRF to be looked up, the ACCEPT_OWN community MUST be discarded. (§2.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: keeping ACCEPT_OWN on a route in a family with no RD. TestACCEPTOwnFamilyBoundary covers IPv4 in the body NLRI; TestRFC7611AcceptOwnDiscardedForEveryNonRDFamilyInMPReach drives enforceRFC7606 with IPv6 unicast and IPv4 labeled unicast in MP_REACH_NLRI and asserts the COMMUNITY attribute shrinks to exactly 65001:100 (or disappears when ACCEPT_OWN was the only value) with MP_REACH byte-identical; negative VPN-IPv6 (RD present) keeps both copies. Red on a non-RD MP family keeping ACCEPT_OWN or an RD family losing it.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC7611AcceptOwnDiscardedForEveryNonRDFamilyInMPReach`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7611_reactor_b_test.go#L72) | unit/verify | revert, verified |
| negative | [`TestACCEPTOwnFamilyBoundary`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7611_receive_test.go#L18) | unit/verify | not verified |
| positive | [`TestRFC7611AcceptOwnDiscardedForEveryNonRDFamilyInMPReach`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7611_reactor_b_test.go#L71) | unit/verify | revert, verified |
| positive | [`TestACCEPTOwnFamilyBoundary`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7611_receive_test.go#L17) | unit/verify | not verified |

### [`RFC7611-2.3-1`](#rfc7611-2.3-1)

ACCEPT_OWN handling SHOULD be controlled by configuration, and if controlled by configuration, it MUST default to being disabled (§2.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent audit against RFC 7611 Section 2.3 (configuration control MUST default to disabled). TestRFC7611AcceptOwnConfigDefaultsDisabled covers both polarities on the runtime RIBManager/ConfigTx boundary: positive proves an unconfigured RIBManager and Stage 2 Configure omitting bgp/accept-own default acceptOwn to false and select the shorter-AS_PATH VPN-IPv4 path over an ACCEPT_OWN candidate, reload omitting accept-own restores false, and explicit string "false" disables it; negative proves Verify/Apply with accept-own:true enables acceptOwn and flips VPN-IPv4 SelectBest to prefer the ACCEPT_OWN path, Rollback restores the prior running state (note: lines 135-141 roll back true->false back to true, whereas the tag comment says 'rollback restores disabled'), and invalid non-boolean values ("maybe", 123, []) are refused at Verify without mutating running state and fail Apply with ErrConfigApplyUnverified. YANG schema default (bgp/accept-own boolean default false in ze-bgp-conf.yang) is verified separately by untagged TestSchemaAcceptOwnDefaultsDisabled; the raw config tree omits unset leaves so extractAcceptOwnConfig and RIBManager.acceptOwn enforce the runtime default. Both discrimination records in rfc/discrimination/rfc7611/RFC7611-2.3-1.json are body-panic reverts on extractAcceptOwnConfig (proving producer reachability, not normal-return semantic mutation); semantic enforcement is established by source review.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC7611AcceptOwnConfigDefaultsDisabled`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc7611_accept_own_test.go#L83) | unit/verify | revert, verified |
| positive | [`TestRFC7611AcceptOwnConfigDefaultsDisabled`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc7611_accept_own_test.go#L82) | unit/verify | revert, verified |

### [`RFC7611-2.3-2`](#rfc7611-2.3-2)

When ACCEPT_OWN is disabled by configuration (either explicitly or by default), the router MUST NOT apply the special route acceptance rules detailed in Section 2.1. (§2.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: accepting, because it carries ACCEPT_OWN, a route whose ORIGINATOR_ID or NEXT_HOP is this router, while ACCEPT_OWN is disabled (ze has no ACCEPT_OWN config, so always). TestACCEPTOwnDoesNotEnableAcceptanceImplicitly asserts LoopIngress returns accepted == false for ORIGINATOR_ID == RouterID with ACCEPT_OWN attached (red on any ACCEPT_OWN exemption) and accepted == true for another originator. NEXT_HOP half: LoopIngress holds no own-NEXT_HOP refusal, so ACCEPT_OWN has no refusal to bypass there; the only acceptance the community could change is the ORIGINATOR_ID one the test asserts.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestACCEPTOwnDoesNotEnableAcceptanceImplicitly`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7611_receive_test.go#L58) | unit/verify | revert, verified |
| positive | [`TestACCEPTOwnDoesNotEnableAcceptanceImplicitly`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7611_receive_test.go#L57) | unit/verify | revert, verified |

### [`RFC7611-3-1`](#rfc7611-3-1)

This extra step MUST only be invoked during the best path selection process of VPN-IP routes [RFC4364] (i.e., it MUST NOT be invoked for the best path selection of imported IP routes in a VRF). (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent audit against RFC 7611 Section 3 (post-LOCAL_PREF ACCEPT_OWN preference step MUST only be invoked for RFC 4364 VPN-IP routes, never for imported IP routes in a VRF). Source review confirms extractCandidate is the sole production Candidate constructor, isVPNIPFamily strictly gates on SAFI 128 (SAFIVPN) with AFI IPv4 or IPv6 only, and compareAcceptOwn sits immediately after Step 1 LOCAL_PREF and before compareAIGP/Step 2 AS_PATH across compareBeforeMED, comparePairWithReason, and multipathEqual. TestRFC7611DecisionStepOnlyForVPNIPRoutes covers both polarities with bgp/accept-own enabled: positive tests both ipv4/vpn and ipv6/vpn, proving an ACCEPT_OWN route with tied LOCAL_PREF beats a shorter-AS_PATH candidate in SelectBest, explains as BestStepAcceptOwn in SelectBestExplain, excludes the non-ACCEPT_OWN sibling in SelectMultipath, and still loses to a higher LOCAL_PREF=200 non-ACCEPT_OWN candidate (confirming placement after LOCAL_PREF); negative tests ipv4/unicast, ipv6/unicast (representing imported IP routes in a VRF), and l2vpn/evpn with ACCEPT_OWN directly in RIB storage, proving the step is not invoked (shorter AS_PATH wins in SelectBest and SelectBestExplain as BestStepASPathLen, and equal-attribute paths remain multipath siblings). Both discrimination records in rfc/discrimination/rfc7611/RFC7611-3-1.json are body-panic reverts on compareAcceptOwn (proving producer reachability, not normal-return semantic mutation); semantic enforcement is established by source review.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC7611DecisionStepOnlyForVPNIPRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc7611_accept_own_test.go#L165) | unit/verify | revert, verified |
| positive | [`TestRFC7611DecisionStepOnlyForVPNIPRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc7611_accept_own_test.go#L164) | unit/verify | revert, verified |

## Requirement list review

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | rfc2119 |
| Source | rfc/full/rfc7611.txt |
| Source fingerprint | 63f038bbea1c9b4f |
| Record | rfc/extraction/rfc7611.json |
| Mapped sentences | 5 |
| Declined as scope | 0 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | reviewed | not stated |
| `1` | not stated | 0 | reviewed | not stated |
| `1.1` | not stated | 0 | reviewed | not stated |
| `2` | not stated | 0 | reviewed | not stated |
| `2.1` | not stated | 1 | reviewed | not stated |
| `2.2` | not stated | 1 | reviewed | not stated |
| `2.3` | not stated | 2 | reviewed | not stated |
| `3` | not stated | 1 | reviewed | not stated |
| `4` | not stated | 0 | reviewed | not stated |
| `5` | not stated | 0 | reviewed | not stated |
| `6` | not stated | 0 | reviewed | not stated |
| `7` | not stated | 0 | reviewed | not stated |
| `8` | not stated | 0 | reviewed | not stated |
| `8.1` | not stated | 0 | reviewed | not stated |
| `8.2` | not stated | 0 | reviewed | not stated |
| `A` | not stated | 0 | reviewed | not stated |

### Excluded sentences

The review of RFC 7611 excluded no sentences: every sentence identified as an obligation is linked to a requirement.

## Superseded

No document obsoletes RFC 7611, so its obligations are stated where they were written.
