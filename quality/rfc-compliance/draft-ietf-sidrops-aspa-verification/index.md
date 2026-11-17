# DRAFT-IETF-SIDROPS-ASPA-VERIFICATION - BGP AS_PATH Verification Based on Autonomous System Provider Authorization (ASPA) Objects

ASPA path verification. Every requirement listed for DRAFT-IETF-SIDROPS-ASPA-VERIFICATION, the tests linked to it, and their recorded reviews. This summary is included in the automatic compliance check, ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 9 | of 10 mandatory requirements | a positive test checks that Ze does what the requirement demands and a negative one checks that it refuses what the requirement forbids |
| One polarity plus reason | 0 | of 10 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason supplies the complete test coverage required |
| One polarity, unexcused | 0 | of 10 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial test coverage | 0 | of 10 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unverified, with zero whole-requirement credit |
| Tests shown to detect a fault | 21 | 21 of 23 tests linked to requirements | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests verifies a requirement only after both tests have been shown to detect a fault |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Declared gap or no test | 1 | of 10 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 10 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 0 | of 10 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 0 | of 10 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze verified: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 10 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 10 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no test verification credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level) | 10 | of 20 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements included in the automatic checks: the compliance check (./le rfc check) holds Ze to each of them. This is the total being assessed; the counts above show the recorded status of those requirements. |

The 9 counts marked as a part above are the whole of the 10 mandatory requirements: they add up to 10. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got. An outlined card with no colored edge, under Totals, is a total the other cards are counted against, not a measure.

| Card | Tone here | Why that color |
|---|---|---|
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the count under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: one test direction is missing without a recorded reason |
| Partial test coverage | ok | green at zero, RED above it: testing one clause does not verify the whole requirement |
| Declared gap or no test | bad | green at zero, RED above it: a binding obligation that nothing shows Ze meets is owed, whether or not a reason is stated |
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
| Public row | ASPA path verification |
| Automatic checks | Included |
| Requirements | 20 |
| Mandatory requirements (MUST level) | 10 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 1 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 23 |
| Tests linked to requirements | 23 |
| Recorded audit verdicts | 5 |
| Fault test records | 21 |
| Summary | `rfc/short/draft-ietf-sidrops-aspa-verification.md` |
| Requirement table | `rfc/requirements/draft-ietf-sidrops-aspa-verification.md` |
| RFC text | `rfc/drafts/draft-ietf-sidrops-aspa-verification.txt` |

## Automatic checks

Included in automatic checks: Router-verification requirements follow the cached revision 28. ASPA registration and AS-migration duties remain separate from router verification.

## Published support status

**Area:** ASPA path verification

**Test verification:** 9 of 10 MUST proven

**Evidence:** unit 9

**Unmet:** DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.1-2


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 9 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 1 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 0 | one part of the mandatory requirements |
| Partial test coverage (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level)** | **10** | every mandatory requirement appears in exactly one group above |

**Positive and negative tests (9):** [`DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-4-1`](#draft-ietf-sidrops-aspa-verification-4-1), [`DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.1-1`](#draft-ietf-sidrops-aspa-verification-5.1-1), [`DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.4-1`](#draft-ietf-sidrops-aspa-verification-5.4-1), [`DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.4-2`](#draft-ietf-sidrops-aspa-verification-5.4-2), [`DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.5-1`](#draft-ietf-sidrops-aspa-verification-5.5-1), [`DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.6-5`](#draft-ietf-sidrops-aspa-verification-5.6-5), [`DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.6-1`](#draft-ietf-sidrops-aspa-verification-5.6-1), [`DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-6.2-1`](#draft-ietf-sidrops-aspa-verification-6.2-1), [`DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-6.2-2`](#draft-ietf-sidrops-aspa-verification-6.2-2)

**Annotated (including scoped evidence) (1):** [`DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.1-2`](#draft-ietf-sidrops-aspa-verification-5.1-2)

## Figures

- **MUST bar**: Ze verified 9, Not built 1
- **Overview bar**: Ze MUST verified 9, Not assessed 10, Not built 1

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | not complete |
| MUST rows | 10 |
| MUST rows kept (the MUST bar) | 10 |
| Ze verified | 9 |
| OS verified | 0 |
| Tests awaiting verification | 0 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 1 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 10 |
| SHOULD verified | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD tests awaiting verification | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 10 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze verified means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Tests awaiting verification counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

Source: [data/rfc/draft-ietf-sidrops-aspa-verification.json](../../../data/rfc/draft-ietf-sidrops-aspa-verification.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-4-1` | Normally, a SPAS (see Section 3) is not expected to contain both an AS 0 and other Provider ASes, but an unexpected presence of AS 0 has no influence on the AS_PATH verification procedures (see Section 5.3, Section 5). (§4) | MUST | 4 | **positive:** `unit/verify` [`TestASPAZeroBesideProvidersIsNoWildcard`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/draft_ietf_sidrops_aspa_verification_aspa_verify_zero_test.go#L28). **positive:** `unit/verify` [`TestASPAZeroDoesNotAuthorizeOrInvalidate`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/draft_ietf_sidrops_aspa_verification_aspa_verify_test.go#L212). **negative:** `unit/verify` [`TestASPAZeroBesideProvidersIsNoWildcard`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/draft_ietf_sidrops_aspa_verification_aspa_verify_zero_test.go#L34). **negative:** `unit/verify` [`TestASPAZeroDoesNotAuthorizeOrInvalidate`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/draft_ietf_sidrops_aspa_verification_aspa_verify_test.go#L213) |
| `DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.1-2` | If the aforementioned AS_PATH checks and error handling are implemented, they MUST be applied prior to ASPA verification. (Section 5.1) | MUST | 5.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** entrypoint coverage for this newly explicit ordering obligation is unverified; Session.processMessage applies existing checks before semantic UPDATE delivery, but the prior neighbor-AS tests have not been established as ordering proof |
| `DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.1-1` | Let the sequence COMPRESSED_AS_PATH {AS(N), AS(N-1),..., AS(2), AS(1)} represent the AS_PATH after removing consecutive duplicate ASNs, where AS(1) is the origin AS, and AS(N) is the most recently added neighbor AS of the receiving/verifying AS. (§5.2) | MUST | 5.2 | **positive:** `unit/verify` [`TestASPACompressedUpdatePrependsValid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/draft_ietf_sidrops_aspa_verification_compressed_aspa_test.go#L97). **negative:** `unit/verify` [`TestASPACompressedUpdatePreservesHops`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/draft_ietf_sidrops_aspa_verification_compressed_aspa_test.go#L149) |
| `DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.4-1` | The upstream verification algorithm described here is applied when a route is received from a Customer or Peer, or is received by an RS from an RS-client, or is received by an RS-client from an RS. (§5.5) | MUST | 5.5 | **positive:** `unit/verify` [`TestASPAUpstreamAppliesToCustomerPeerAndRSRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/draft_ietf_sidrops_aspa_verification_aspa_role_mode_test.go#L32). **negative:** `unit/verify` [`TestASPAUpstreamAppliesToCustomerPeerAndRSRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/draft_ietf_sidrops_aspa_verification_aspa_role_mode_test.go#L33) |
| `DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.4-2` | If the AS_PATH has an AS_SET, then the procedure halts with the outcome "Invalid" (Section 5.5; Section 5.6) | MUST | 5.5 | **positive:** `unit/verify` [`TestASPAVerifyASSet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/draft_ietf_sidrops_aspa_verification_aspa_verify_test.go#L56). **negative:** `unit/verify` [`TestASPAStateForPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/draft_ietf_sidrops_aspa_verification_aspa_verify_test.go#L180) |
| `DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.5-1` | If the receiving AS is not an RS-client and the most recently added AS in the AS_PATH does not match the neighbor AS, then the procedure halts with the outcome "Invalid". (§5.5) | MUST | 5.5 | **positive:** `unit/verify` [`TestASPAASTransNeighborUnconfirmed`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/draft_ietf_sidrops_aspa_verification_aspa_as_trans_neighbor_test.go#L116). **negative:** `unit/verify` [`TestASPAASTransNeighborFirstASIsASTrans`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/draft_ietf_sidrops_aspa_verification_aspa_as_trans_neighbor_test.go#L200). **negative:** `unit/verify` [`TestASPAASTransNeighborRSClientExempt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/draft_ietf_sidrops_aspa_verification_aspa_as_trans_neighbor_test.go#L242) |
| `DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.6-5` | If the most recently added AS in the AS_PATH does not match the neighbor AS, then the procedure halts with the outcome "Invalid". (§5.6) | MUST | 5.6 | **positive:** `unit/verify` [`TestASPAASTransNeighborDownstreamUnconfirmed`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/draft_ietf_sidrops_aspa_verification_aspa_as_trans_neighbor_test.go#L261). **negative:** `unit/verify` [`TestASPAASTransNeighborDownstreamConfirmed`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/draft_ietf_sidrops_aspa_verification_aspa_as_trans_neighbor_test.go#L286) |
| `DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.6-1` | If the AS_PATH is determined to be Invalid, then the route SHOULD be considered ineligible for route selection (see Section 3) and MUST be kept in the Adj-RIB-In for potential future re-evaluation (see [RFC9324]). (§5.7) | MUST | 5.7 | **positive:** `unit/verify` [`TestASPARetainedPathReplayRecovery`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/adj_rib_in/draft_ietf_sidrops_aspa_verification_rib_aspa_retention_test.go#L65). **positive:** `unit/verify` [`TestASPARetentionReceiveRecovery`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/draft_ietf_sidrops_aspa_verification_rib_aspa_retention_test.go#L210). **negative:** `unit/verify` [`TestASPARetainedPathReplayRecovery`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/adj_rib_in/draft_ietf_sidrops_aspa_verification_rib_aspa_retention_test.go#L68). **negative:** `unit/verify` [`TestASPARetentionReplacementWithdrawal`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/draft_ietf_sidrops_aspa_verification_rib_aspa_retention_test.go#L257) |
| `DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-6.2-1` | The verification procedures described in this document MUST be applied to BGP routes with {AFI, SAFI} combinations {AFI 1 (IPv4), SAFI 1} and {AFI 2 (IPv6), SAFI 1} [IANA-AF] [IANA-SAF]. (Section 6.2) | MUST | 6.2 - The two uppercase MUST sites retain their AFI/SAFI IDs | **positive:** `unit/verify` [`TestASPAAppliesToIPv6Unicast`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/draft_ietf_sidrops_aspa_verification_aspa_family_scope_test.go#L110). **negative:** `unit/verify` [`TestASPAAppliesToIPv4Unicast`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/draft_ietf_sidrops_aspa_verification_aspa_family_scope_test.go#L132) |
| `DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-6.2-2` | The procedures MUST NOT be applied to other address families by default (Section 6.2) | MUST NOT | 6.2 - The two uppercase MUST sites retain their AFI/SAFI IDs | **positive:** `unit/verify` [`TestASPAAppliesToIPv6Unicast`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/draft_ietf_sidrops_aspa_verification_aspa_family_scope_test.go#L111). **negative:** `unit/verify` [`TestASPANotAppliedToOtherFamilies`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/draft_ietf_sidrops_aspa_verification_aspa_family_scope_test.go#L165) |
| `DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.6-2` | If the AS_PATH is determined to be Invalid, then the route SHOULD be considered ineligible for route selection (Section 5.7) | SHOULD | 5.7 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.6-3` | When a route is evaluated as Unknown (using ASPA- based AS_PATH verification), it SHOULD be treated at the same preference level as a route evaluated as Valid. (§5.7) | SHOULD | 5.7 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.6-4` | The specific configuration of a mitigation policy based on AS_PATH verification using ASPA is at the discretion of the network operator. However, the following mitigation policy is RECOMMENDED. (§5.7) | RECOMMENDED | 5.7 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-6.2-3` | However, the procedures are NOT RECOMMENDED for use on internal BGP (iBGP) sessions or eBGP sessions internal to an AS Confederation. (§6.2) | NOT RECOMMENDED | 6.2 - The two uppercase MUST sites retain their AFI/SAFI IDs | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-6.3-1` | The BGP Role configuration parameter and its cross-check in BGP OPEN message as specified in [RFC9234] are RECOMMENDED. (§6.3) | RECOMMENDED | 6.3 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-6.3-2` | The configured BGP Roles SHOULD be used to automate the use of the above-described AS_PATH verification procedures helping to distinguish whether upstream or downstream procedures should be applied. (§6.3) | SHOULD | 6.3 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-6.4-1` | If multiple eBGP sessions can segregate the Complex peering relationship into eBGP sessions with normal peering relationships, the receiving/verifying AS SHOULD select the algorithm (per Section 5.5 or Section 5.6) for each of the normal sessions based on its peering relation type (Section 6.4) | SHOULD | 6.4 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-6.6-1` | For any route with an Invalid AS_PATH, the cause of the Invalid state SHOULD be logged for monitoring and diagnostic purposes (Section 6.6) | SHOULD | 6.6 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-8.4-1` | The implementation of the procedures utilizing the OTC Attribute is RECOMMENDED to complement the ASPA-based AS_PATH verification (Section 8.4) | RECOMMENDED | 8.4 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-6.4-2` | If a Complex peering relation cannot be segregated (i.e., when a Complex BGP relationship occurs within one single BGP session), an operator may want to achieve an equivalent outcome by applying an appropriate algorithm (Section 5.5 or Section 5.6) on a per-prefix basis corresponding to the peering relation for the prefix. If this option is not feasible, then an operator MAY apply the algorithm for downstream paths (Section 5.6) to avoid false positive outcomes. (§6.4) | MAY | 6.4 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.1-2`](#draft-ietf-sidrops-aspa-verification-5.1-2) If the aforementioned AS_PATH checks and error handling are implemented, they MUST be applied prior to ASPA verification. (Section 5.1) | {gap}, no test | entrypoint coverage for this newly explicit ordering obligation is unverified; Session.processMessage applies existing checks before semantic UPDATE delivery, but the prior neighbor-AS tests have not been established as ordering proof |

## Test status

A test reads not verified when no record shows it failing after a deliberate fault. Until then, the test has not been shown to detect a fault in the required behavior.

### [`DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-4-1`](#draft-ietf-sidrops-aspa-verification-4-1)

Normally, a SPAS (see Section 3) is not expected to contain both an AS 0 and other Provider ASes, but an unexpected presence of AS 0 has no influence on the AS_PATH verification procedures (see Section 5.3, Section 5). (§4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: AS 0 beside other providers changing the verification outcome. TestASPAZeroDoesNotAuthorizeOrInvalidate: {0,200} Valid, {0} alone Invalid. TestASPAZeroBesideProvidersIsNoWildcard: {0,200} = {200} = Valid (AS 0 does not invalidate) and {0,999} = {999} = Invalid for a path whose provider 200 is unlisted (AS 0 is no wildcard).

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestASPAZeroDoesNotAuthorizeOrInvalidate`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/draft_ietf_sidrops_aspa_verification_aspa_verify_test.go#L213) | unit/verify | revert, verified |
| negative | [`TestASPAZeroBesideProvidersIsNoWildcard`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/draft_ietf_sidrops_aspa_verification_aspa_verify_zero_test.go#L34) | unit/verify | revert, verified |
| positive | [`TestASPAZeroDoesNotAuthorizeOrInvalidate`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/draft_ietf_sidrops_aspa_verification_aspa_verify_test.go#L212) | unit/verify | revert, verified |
| positive | [`TestASPAZeroBesideProvidersIsNoWildcard`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/draft_ietf_sidrops_aspa_verification_aspa_verify_zero_test.go#L28) | unit/verify | revert, verified |

### [`DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.1-2`](#draft-ietf-sidrops-aspa-verification-5.1-2)

If the aforementioned AS_PATH checks and error handling are implemented, they MUST be applied prior to ASPA verification. (Section 5.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.1-2, so there is no linked test.

### [`DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.1-1`](#draft-ietf-sidrops-aspa-verification-5.1-1)

Let the sequence COMPRESSED_AS_PATH {AS(N), AS(N-1),..., AS(2), AS(1)} represent the AS_PATH after removing consecutive duplicate ASNs, where AS(1) is the origin AS, and AS(N) is the most recently added neighbor AS of the receiving/verifying AS. (§5.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a COMPRESSED_AS_PATH that keeps consecutive duplicates, removes non-consecutive ones, or reverses origin and neighbor. TestASPACompressedUpdatePrependsValid asserts state valid and accepted for prepends within a segment, across a segment boundary and on a single AS, red if a duplicate survives as an unauthorized self-hop; with 100 as neighbor and 300 as origin, a reversed order also turns it Invalid. TestASPACompressedUpdatePreservesHops asserts invalid and rejected for the separated repeat [100 200 300 200], red if non-consecutive ASNs are dropped.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestASPACompressedUpdatePreservesHops`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/draft_ietf_sidrops_aspa_verification_compressed_aspa_test.go#L149) | unit/verify | revert, verified |
| positive | [`TestASPACompressedUpdatePrependsValid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/draft_ietf_sidrops_aspa_verification_compressed_aspa_test.go#L97) | unit/verify | revert, verified |

### [`DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.4-1`](#draft-ietf-sidrops-aspa-verification-5.4-1)

The upstream verification algorithm described here is applied when a route is received from a Customer or Peer, or is received by an RS from an RS-client, or is received by an RS-client from an RS. (§5.5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a route from a Customer or Peer, by an RS from an RS-client, or by an RS-client from an RS verified other than by the upstream algorithm. TestASPAUpstreamAppliesToCustomerPeerAndRSRoutes asserts configuredASPAMode == aspaUpstream for local roles provider, peer, rs and rs-client (one per listed case) and that the down-ramp path is Invalid under each, red if any maps to downstream; local role customer maps to aspaDownstream and verifies the same path Valid.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestASPAUpstreamAppliesToCustomerPeerAndRSRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/draft_ietf_sidrops_aspa_verification_aspa_role_mode_test.go#L33) | unit/verify | revert, verified |
| positive | [`TestASPAUpstreamAppliesToCustomerPeerAndRSRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/draft_ietf_sidrops_aspa_verification_aspa_role_mode_test.go#L32) | unit/verify | revert, verified |

### [`DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.4-2`](#draft-ietf-sidrops-aspa-verification-5.4-2)

If the AS_PATH has an AS_SET, then the procedure halts with the outcome "Invalid" (Section 5.5; Section 5.6)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestASPAStateForPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/draft_ietf_sidrops_aspa_verification_aspa_verify_test.go#L180) | unit/verify | revert, verified |
| positive | [`TestASPAVerifyASSet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/draft_ietf_sidrops_aspa_verification_aspa_verify_test.go#L56) | unit/verify | revert, verified |

### [`DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.5-1`](#draft-ietf-sidrops-aspa-verification-5.5-1)

If the receiving AS is not an RS-client and the most recently added AS in the AS_PATH does not match the neighbor AS, then the procedure halts with the outcome "Invalid". (§5.5)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestASPAASTransNeighborFirstASIsASTrans`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/draft_ietf_sidrops_aspa_verification_aspa_as_trans_neighbor_test.go#L200) | unit/verify | revert, verified |
| negative | [`TestASPAASTransNeighborRSClientExempt`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/draft_ietf_sidrops_aspa_verification_aspa_as_trans_neighbor_test.go#L242) | unit/verify | revert, verified |
| positive | [`TestASPAASTransNeighborUnconfirmed`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/draft_ietf_sidrops_aspa_verification_aspa_as_trans_neighbor_test.go#L116) | unit/verify | revert, verified |

### [`DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.6-5`](#draft-ietf-sidrops-aspa-verification-5.6-5)

If the most recently added AS in the AS_PATH does not match the neighbor AS, then the procedure halts with the outcome "Invalid". (§5.6)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestASPAASTransNeighborDownstreamConfirmed`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/draft_ietf_sidrops_aspa_verification_aspa_as_trans_neighbor_test.go#L286) | unit/verify | revert, verified |
| positive | [`TestASPAASTransNeighborDownstreamUnconfirmed`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/draft_ietf_sidrops_aspa_verification_aspa_as_trans_neighbor_test.go#L261) | unit/verify | revert, verified |

### [`DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.6-1`](#draft-ietf-sidrops-aspa-verification-5.6-1)

If the AS_PATH is determined to be Invalid, then the route SHOULD be considered ineligible for route selection (see Section 3) and MUST be kept in the Adj-RIB-In for potential future re-evaluation (see [RFC9324]). (§5.7)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/bgp/plugins/adj_rib_in/draft_ietf_sidrops_aspa_verification_rib_aspa_retention_test.go::TestASPARetainedPathReplayRecovery, internal/component/bgp/plugins/adj_rib_in/draft_ietf_sidrops_aspa_verification_rib_aspa_retention_test.go::TestASPARetainedPathReplayRecovery#2 moved. Independent rejudgment 2026-11-15 after only the two selecting-RIB retention fixture topology repairs. The canonical ASPA verification draft Section 5.7 says: 'If the AS_PATH is determined to be Invalid, then the route SHOULD be considered ineligible for route selection (see Section 3) and MUST be kept in the Adj-RIB-In for potential future re-evaluation (see [RFC9324]).' The row's stable identifier remains 5.6-1 although the current draft section is 5.7. RFC 4271 Section 9.1.2 independently excludes an unresolvable NEXT_HOP before selection. TestASPARetentionReceiveRecovery now supplies only connected 192.0.2.9/32; all six typed/text by early/pending/installed cases still use the real registered Adj-RIB-In bridge and selecting RIB, retain exact attributes, NLRI and next-hop bytes after rejection, require absence from both Loc-RIB and best-path replay while Invalid, and recover the same received route on a later accepting decision without another UPDATE. TestASPARetentionReplacementWithdrawal supplies only connected 192.0.2.9/32 and 192.0.2.10/32, making both unchanged old and replacement next hops feasible independently of validation. It still rejects the newer message generation, removes the old selection, retains the replacement's new bytes, refuses a stale acceptance, and forbids resurrection after actual withdrawal, session removal and origin rejection. The topology cannot itself accept a validation-refused path; every generation and exclusion assertion remains. The unchanged Adj-RIB-In TestASPARetainedPathReplayRecovery supplies both tagged polarities: a retained Invalid ADD-PATH sibling survives expiry with complete bytes, stays out of ordinary replay, recovers after a cursor without another UPDATE, and its withdrawal preserves the eligible sibling while targeted replay reports an explicit withdrawal. No wire bytes, actions, message IDs, validation phases, assertions, negative controls, shared constructors or producers changed in this repair. Main's selected repaired RIB roots passed with race in 4d7e8db1 and no skips. I inspected both new normal-return semantic proof logs and native records against the current selecting-RIB bodies. In e8c69275 the clean TestASPARetentionReceiveRecovery baseline passed all six cases. The applyToInstalled mutant removed an installed route on an Ineligible decision: false/installed and true/installed then failed the exact retained-byte assertion with an empty stored list, while all four early/pending cases still passed. This observation proves discrimination for the installed-to-Invalid retention branch, not an observed mutant failure of every receive order. In 517807c4 the clean TestASPARetentionReplacementWithdrawal baseline passed all three branches. Changing the old-decision guard from d.MsgID < route.MsgID to d.MsgID != route.MsgID wrongly ignored the newer Invalid decision; withdraw, session and origin-reject cases then each failed at Loc-RIB installed=true, want false, before their later removal assertions. These two native mutant records bind the current positive and negative claim bodies. Neither observation used a panic, skip or build failure, and no broader mutant coverage is inferred. Other tagged units, requirement quotes, levels and discrimination entries are unchanged. This pending file does not itself stamp or publish an audit.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestASPARetainedPathReplayRecovery`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/adj_rib_in/draft_ietf_sidrops_aspa_verification_rib_aspa_retention_test.go#L68) | unit/verify | not verified |
| negative | [`TestASPARetentionReplacementWithdrawal`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/draft_ietf_sidrops_aspa_verification_rib_aspa_retention_test.go#L257) | unit/verify | mutant, verified |
| positive | [`TestASPARetainedPathReplayRecovery`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/adj_rib_in/draft_ietf_sidrops_aspa_verification_rib_aspa_retention_test.go#L65) | unit/verify | not verified |
| positive | [`TestASPARetentionReceiveRecovery`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/draft_ietf_sidrops_aspa_verification_rib_aspa_retention_test.go#L210) | unit/verify | mutant, verified |

### [`DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-6.2-1`](#draft-ietf-sidrops-aspa-verification-6.2-1)

The verification procedures described in this document MUST be applied to BGP routes with {AFI, SAFI} combinations {AFI 1 (IPv4), SAFI 1} and {AFI 2 (IPv6), SAFI 1} [IANA-AF] [IANA-SAF]. (Section 6.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: an IPv4 unicast or IPv6 unicast route left unverified. TestASPAAppliesToIPv6Unicast asserts family ipv6/unicast reaches the decision path with aspaState ASPAInvalid and is tracked, red if MP_REACH IPv6 unicast is skipped. TestASPAAppliesToIPv4Unicast asserts ipv4/unicast carries ASPAInvalid, red if the plain-NLRI branch is skipped.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestASPAAppliesToIPv4Unicast`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/draft_ietf_sidrops_aspa_verification_aspa_family_scope_test.go#L132) | unit/verify | revert, verified |
| positive | [`TestASPAAppliesToIPv6Unicast`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/draft_ietf_sidrops_aspa_verification_aspa_family_scope_test.go#L110) | unit/verify | revert, verified |

### [`DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-6.2-2`](#draft-ietf-sidrops-aspa-verification-6.2-2)

The procedures MUST NOT be applied to other address families by default (Section 6.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestASPANotAppliedToOtherFamilies`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/draft_ietf_sidrops_aspa_verification_aspa_family_scope_test.go#L165) | unit/verify | revert, verified |
| positive | [`TestASPAAppliesToIPv6Unicast`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rpki/draft_ietf_sidrops_aspa_verification_aspa_family_scope_test.go#L111) | unit/verify | revert, verified |

## Requirement list review

| Field | Value |
|---|---|
| Reviewer | Main integration; independent source-mapping review: SourceIdentityReview |
| Signed off | 2026-09-22 |
| Register | rfc2119 |
| Source | rfc/drafts/draft-ietf-sidrops-aspa-verification.txt |
| Source fingerprint | b90c021fcc40be45 |
| Record | rfc/extraction/draft-ietf-sidrops-aspa-verification.json |
| Mapped sentences | 4 |
| Declined as scope | 6 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | skipped (front-matter) | Abstract, draft status, copyright and table of contents; no router-verification obligation. |
| `1` | not stated | 0 | reviewed | Introduction describes threats and the purpose of the procedures; it adds no distinct checklist obligation. |
| `2` | not stated | 0 | reviewed | BCP 14 keyword interpretation boilerplate; no operational requirement. |
| `3` | not stated | 0 | reviewed | Terminology defines route ineligibility, CAS, PAS, SPAS and local peering relationships used by the later algorithms. |
| `4` | not stated | 4 | reviewed | The indicative verification rule retained under 4-1 is: "Normally, a SPAS (see Section 3) is not expected to contain both an AS 0 and other Provider ASes, but an unexpected presence of AS 0 has no influence on the AS_PATH verification procedures (see Section 5.3, Section 5)." It carries no uppercase MUST, so it is declared here rather than mapped to a registration site. The uppercase registration sites bind the separate registrar. The SHOULDs to register an ASPA, use a single object, enforce that practice in CA hosting software and keep registries identical, plus the RECOMMENDED advance inclusion of standby providers, likewise address registration/CA operations. Revision 28 omits revision 27's AS0-registration MUST, so old site 4:5 has no current counterpart. |
| `5` | not stated | 0 | reviewed | The procedures apply to four-octet-compatible speakers and use the reconstructed AS_PATH when AS_PATH and AS4_PATH arrive together. This input definition is retained in the summary's prerequisite discussion. Former section-5 neighbor-check SHOULD and mismatch SHALL sentences are absent; their allocated IDs are source-level conflicts documented in aspa-metadata-migration.json, not assigned to unrelated current sites. |
| `5.1` | not stated | 2 | reviewed | The AS_SET treat-as-withdraw MUST is attributed to RFC 9774/RFC 7606. The draft's own MUST is: "If the aforementioned AS_PATH checks and error handling are implemented, they MUST be applied prior to ASPA verification." This is DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.1-2. The neighbor-AS check is described with the transparent route-server exception and attributed to RFC 4271/RFC 7606. The following sentence describes ASPA Invalid if the prerequisites are not enforced; it does not restate the former unconditional SHALL. |
| `5.2` | not stated | 0 | reviewed | The compression definition retains 5.1-1: "Let the sequence COMPRESSED_AS_PATH {AS(N), AS(N-1),..., AS(2), AS(1)} represent the AS_PATH after removing consecutive duplicate ASNs, where AS(1) is the origin AS, and AS(N) is the most recently added neighbor AS of the receiving/verifying AS." The definition is indicative prose. The section also defines the ramp topology and apex separation used by the formal algorithms. |
| `5.3` | not stated | 0 | reviewed | Defines the union of cryptographically valid ASPA provider sets and the Provider+, Not Provider+ and No Attestation results. The summary retains the function table and the restriction that No Attestation means no retrieved or cryptographically valid ASPA. |
| `5.4` | not stated | 0 | reviewed | Defines maximum/minimum up-ramp and down-ramp lengths and their Invalid/Unknown/Valid bounds; the summary's ramp table and algorithm steps retain these indicative rules. This is not the current source of the permanently allocated 5.4-1 and 5.4-2 IDs. |
| `5.5` | not stated | 0 | reviewed | The role obligation retaining 5.4-1 reads: "The upstream verification algorithm described here is applied when a route is received from a Customer or Peer, or is received by an RS from an RS-client, or is received by an RS-client from an RS." Step 3 retains 5.4-2: "If the AS_PATH has an AS_SET, then the procedure halts with the outcome "Invalid"." Both are indicative algorithm rules. Empty path, neighbor mismatch with the RS-client exception, ramp denial, missing attestation and the Valid fallback are retained in the summary's upstream algorithm. |
| `5.6` | not stated | 0 | reviewed | Downstream applies to a route from a Provider and uses both ramps. Step 3 repeats the AS_SET Invalid obligation already recorded as DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.4-2 under section 5.5; no duplicate requirement is allocated. Step 2, the neighbor-AS mismatch with no RS-client exception, is the indicative algorithm rule DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.6-5; the upstream step 2 with its RS-client exception is 5.5-1. The summary retains all downstream steps. This is not the source of the old mitigation IDs 5.6-1 to 5.6-4. |
| `5.7` | not stated | 1 | reviewed | The MUST-bearing Invalid sentence maps to retained ID 5.6-1, and also says the route "SHOULD be considered ineligible for route selection" (5.6-2). The same paragraph is one inventory site, so the SHOULD has no separate mapping. The other advisory rows come from: "When a route is evaluated as Unknown (using ASPA-based AS_PATH verification), it SHOULD be treated at the same preference level as a route evaluated as Valid." (5.6-3), and "However, the following mitigation policy is RECOMMENDED." (5.6-4), preceded by operator discretion over the specific policy configuration. |
| `6` | Introduces deployment recommendations | 0 | reviewed | Introduces deployment recommendations. |
| `6.1` | not stated | 0 | reviewed | Links illustrative verification examples; no additional normative requirement. |
| `6.2` | The two uppercase MUST sites retain their AFI/SAFI IDs | 2 | reviewed | The two uppercase MUST sites retain their AFI/SAFI IDs. The advisory row reads: "However, the procedures are NOT RECOMMENDED for use on internal BGP (iBGP) sessions or eBGP sessions internal to an AS Confederation." The ingress placement and external Confederation boundary context are retained. |
| `6.3` | not stated | 0 | reviewed | 6.3-1: "The BGP Role configuration parameter and its cross-check in BGP OPEN message as specified in [RFC9234] are RECOMMENDED." 6.3-2: "The configured BGP Roles SHOULD be used to automate the use of the above-described AS_PATH verification procedures helping to distinguish whether upstream or downstream procedures should be applied." Both are advisory and outside the uppercase MUST inventory. |
| `6.4` | not stated | 0 | reviewed | 6.4-1: "If multiple eBGP sessions can segregate the Complex peering relationship into eBGP sessions with normal peering relationships, the receiving/verifying AS SHOULD select the algorithm (per Section 5.5 or Section 5.6) for each of the normal sessions based on its peering relation type." For a relation that cannot be segregated and where the described per-prefix option is infeasible, 6.4-2 reads: "If this option is not feasible, then an operator MAY apply the algorithm for downstream paths (Section 5.6) to avoid false positive outcomes." |
| `6.5` | not stated | 1 | reviewed | The migration notification MUST binds the AS operator's customer-notification process, as classified at 6.5:1. |
| `6.6` | not stated | 0 | reviewed | The advisory requirement reads: "For any route with an Invalid AS_PATH, the cause of the Invalid state SHOULD be logged for monitoring and diagnostic purposes." The following prose suggests listing Not Provider+ hops and warns that the logging router cannot necessarily identify the leak's author. |
| `7` | not stated | 0 | reviewed | Security considerations heading; substantive text follows in subsections. |
| `7.1` | not stated | 0 | reviewed | Explains the cross-family permissiveness caused by a single union of IPv4 and IPv6 providers; retained in the summary. |
| `7.2` | not stated | 0 | reviewed | Operational advice to maintain correct ASPAs and monitor customer SPAS membership; explains reduced detection or false Invalid caused by erroneous provider sets. The lowercase modals add no uppercase requirement site. |
| `7.3` | not stated | 0 | reviewed | Describes provider-origin/path manipulation that ASPA may not detect and its operational consequences; no new verification rule. |
| `7.4` | not stated | 0 | reviewed | ASPA cannot detect added or removed prepends, but that change alone does not affect route-leak detection. It does not authorise removing non-consecutive repeats. |
| `8` | Relation-to-other-technologies heading | 0 | reviewed | Relation-to-other-technologies heading. |
| `8.1` | Explains complementary origin verification with ROAs | 0 | reviewed | Explains complementary origin verification with ROAs. |
| `8.2` | not stated | 0 | reviewed | Explains complementary cryptographic path protection with BGPsec; no ASPA requirement added. |
| `8.3` | not stated | 0 | reviewed | Compares ASPA with Peerlock; descriptive text adds no distinct algorithm obligation. |
| `8.4` | not stated | 0 | reviewed | The advisory requirement reads: "The implementation of the procedures utilizing the OTC Attribute is RECOMMENDED to complement the ASPA-based AS_PATH verification." It follows the limitations on preventing locally initiated leaks and detecting leaks through Complex relationships. |
| `9` | not stated | 0 | skipped (iana) | The section states that the document includes no request to IANA. |
| `10` | not stated | 0 | reviewed | Describes third-party implementation reports under RFC 7942 and expressly disclaims independent verification. These reports supply no Ze execution evidence. |
| `11` | Reference-list heading | 0 | skipped (references) | Reference-list heading. |
| `11.1` | not stated | 0 | skipped (references) | Normative bibliography; titles and dates are not router requirements. |
| `11.2` | Informative bibliography | 0 | skipped (references) | Informative bibliography. |
| `A` | Acknowledges contributors | 0 | skipped (acknowledgements) | Acknowledges contributors. |
| `B` | not stated | 0 | reviewed | Explains route-leak, forged-origin and forged-segment detection properties and assumptions during partial deployment; introduces no new procedure. Authors' addresses follow without a numbered heading. |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `4:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Section 4 says: "An AS MUST list in its SPAS the union of all its Provider AS(es) and non-transparent RS AS(es) at which it is an RS-client." This action belongs to the resource holder's ASPA registration service. internal/component/bgp/plugins/rpki/aspa_cache.go::ApplyDelta stores customer/provider records received through RTR; internal/component/bgp/plugins/rpki/aspa_cache.go::checkPair and internal/component/bgp/plugins/rpki/rpki.go::handleStructuredUpdate consume them for verification. These functions neither choose the resource holder's published SPAS nor register signed objects. The registrar acts for the resource holder independently of this router's verification role; a cache supplying verified records does not register the router operator's ASPA on its behalf. | An AS MUST list in its SPAS the union of all its Provider AS(es) and non-transparent RS AS(es) at which it is an RS-client. |
| `4:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Section 4 says: "An AS MUST include a Provider AS in its SPAS regardless of whether it provides connectivity for only IPv4 or only IPv6 or both." The resource holder's ASPA registration service chooses that SPAS. internal/component/bgp/plugins/rpki/aspa_cache.go::ApplyDelta receives a provider set with no address-family field, and internal/component/bgp/plugins/rpki/aspa_cache.go::checkPair reads it. Receiving the union does not perform the independent resource holder's duty to publish it, and no layer used by these router-verification producers selects or registers that holder's providers. | An AS MUST include a Provider AS in its SPAS regardless of whether it provides connectivity for only IPv4 or only IPv6 or both. |
| `4:3` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Section 4 says: "In the Complex relationship case (Section 3 and [RFC9234]), an AS MUST include the neighbor AS in its SPAS if the neighbor plays the Provider role for all or a subset of received or sent prefixes." The resource holder's ASPA registration service publishes those provider relationships. internal/component/bgp/plugins/rpki/aspa_cache.go::ApplyDelta stores received sets and internal/component/bgp/plugins/rpki/rpki_config.go::configuredASPAMode selects the router's verification algorithm; neither publishes ASPAs. A Complex verification relationship does not turn those consumers, or the cache supplying them, into the resource holder's registrar. | In the Complex relationship case (Section 3 and [RFC9234]), an AS MUST include the neighbor AS in its SPAS if the neighbor plays the Provider role for all or a subset of received or sent prefixes. |
| `4:4` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Section 4 says: "The ASes on the boundary of an AS Confederation MUST register ASPAs using the Confederation's global AS number (ASN) as the CAS." The Confederation resource holder's ASPA registration service chooses the CAS of the published object. internal/component/bgp/plugins/rpki/aspa_cache.go::ApplyDelta indexes the CAS supplied through RTR and does not register it. Operating a boundary router does not make its verification consumer or its supplying cache the registrant for that resource holder. | The ASes on the boundary of an AS Confederation MUST register ASPAs using the Confederation's global AS number (ASN) as the CAS. |
| `5.1:1` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | The sentence explicitly attributes the obligation elsewhere: "[RFC9774] specifies that "treat-as-withdraw" error handling [RFC7606] MUST be applied to any route containing an AS_SET in the AS_PATH." This attribution does not exclude ASPA's own AS_SET Invalid result: DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-5.4-2 retains that distinct algorithm obligation from sections 5.5 and 5.6. The conditional requirement that implemented checks run before ASPA is separately mapped at 5.1:2. | [RFC9774] specifies that "treat-as-withdraw" error handling [RFC7606] MUST be applied to any route containing an AS_SET in the AS_PATH. |
| `6.5:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Section 6.5 says: "The AS operator MUST notify its customer ASes and advise them to update ASPA records to include both the globally configured ASN and the legacy ASN in their SPAS." The migrating AS operator's out-of-band customer notification process performs this communication. internal/component/bgp/plugins/rpki/aspa_cache.go::ApplyDelta receives provider sets and internal/component/bgp/plugins/rpki/rpki.go::handleStructuredUpdate verifies received routes; neither contacts the operator's customers or changes their registrations. A cache delivering verified ASPAs does not fulfil this human operational notification on the router's behalf. | The AS operator MUST notify its customer ASes and advise them to update ASPA records to include both the globally configured ASN and the legacy ASN in their SPAS. |

## Superseded

No document obsoletes DRAFT-IETF-SIDROPS-ASPA-VERIFICATION, so its obligations are stated where they were written.
