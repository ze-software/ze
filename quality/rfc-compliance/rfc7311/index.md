# RFC 7311 - The Accumulated IGP Metric Attribute for BGP

AIGP attribute. Every requirement listed for RFC 7311, the tests linked to it, and their recorded reviews. This summary is included in the automatic compliance check, ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 22 | of 22 mandatory requirements | a positive test checks that Ze does what the requirement demands and a negative one checks that it refuses what the requirement forbids |
| One polarity plus reason | 0 | of 22 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason supplies the complete test coverage required |
| One polarity, unexcused | 0 | of 22 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial test coverage | 0 | of 22 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unverified, with zero whole-requirement credit |
| Declared gap or no test | 0 | of 22 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |
| Tests shown to detect a fault | 27 | 27 of 81 tests linked to requirements, 0 without a fault test and 28 with outdated fault tests | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests verifies a requirement only after both tests have been shown to detect a fault |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 22 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 0 | of 22 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 0 | of 22 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze verified: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 22 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 22 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no test verification credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level) | 22 | of 26 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements included in the automatic checks: the compliance check (./le rfc check) holds Ze to each of them. This is the total being assessed; the counts above show the recorded status of those requirements. |

The 9 counts marked as a part above are the whole of the 22 mandatory requirements: they add up to 22. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

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
| Public row | AIGP attribute |
| Automatic checks | Included |
| Requirements | 26 |
| Mandatory requirements (MUST level) | 22 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 81 |
| Tests linked to requirements | 81 |
| Recorded audit verdicts | 12 |
| Fault test records | 55 |
| Summary | `rfc/short/rfc7311.md` |
| Requirement table | `rfc/requirements/rfc7311.md` |
| RFC text | `rfc/full/rfc7311.txt` |

## Automatic checks

Included in automatic checks: Ze implements AIGP validation, per-session policy, configured origination, forwarding accumulation and best-path selection. Recursive metric changes reach retained advertisements and the selecting RIB, including its subprocess transport. Requirement identities follow the mappings in `rfc/extraction/rfc7311.json`; the legacy RFC7311-3.2-1 identity maps to the origination prohibition in Section 3.4.1, not a separate propagation rule in Section 3.2.

## Published support status

**Area:** AIGP attribute

**Test verification:** 8 of 22 MUST proven

**Evidence:** unit 8

**Unmet:** -


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 22 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 0 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 0 | one part of the mandatory requirements |
| Partial test coverage (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level)** | **22** | every mandatory requirement appears in exactly one group above |

**Positive and negative tests (22):** [`RFC7311-3-1`](#rfc7311-3-1), [`RFC7311-3-2`](#rfc7311-3-2), [`RFC7311-3-3`](#rfc7311-3-3), [`RFC7311-3.2-4`](#rfc7311-3.2-4), [`RFC7311-3.2-1`](#rfc7311-3.2-1), [`RFC7311-3.2-5`](#rfc7311-3.2-5), [`RFC7311-3.2-6`](#rfc7311-3.2-6), [`RFC7311-3.3-1`](#rfc7311-3.3-1), [`RFC7311-3.3-2`](#rfc7311-3.3-2), [`RFC7311-3.3-3`](#rfc7311-3.3-3), [`RFC7311-3.3-4`](#rfc7311-3.3-4), [`RFC7311-3.4.1-1`](#rfc7311-3.4.1-1), [`RFC7311-3.4.1-2`](#rfc7311-3.4.1-2), [`RFC7311-3.4.1-3`](#rfc7311-3.4.1-3), [`RFC7311-3.4.3-1`](#rfc7311-3.4.3-1), [`RFC7311-3.4.3-2`](#rfc7311-3.4.3-2), [`RFC7311-3.4.3-3`](#rfc7311-3.4.3-3), [`RFC7311-3.4.3-4`](#rfc7311-3.4.3-4), [`RFC7311-3.4.3-5`](#rfc7311-3.4.3-5), [`RFC7311-3.4.3-6`](#rfc7311-3.4.3-6), [`RFC7311-3.4.3-7`](#rfc7311-3.4.3-7), [`RFC7311-4.1-1`](#rfc7311-4.1-1)

## Figures

- **MUST bar**: Ze verified 8, Tests awaiting verification 14
- **Overview bar**: Ze MUST verified 8, Tests awaiting verification 14, Not assessed 4

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | not complete |
| MUST rows | 22 |
| MUST rows kept (the MUST bar) | 22 |
| Ze verified | 8 |
| OS verified | 0 |
| Tests awaiting verification | 14 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 0 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 4 |
| SHOULD verified | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD tests awaiting verification | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 4 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze verified means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Tests awaiting verification counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

Source: [data/rfc/rfc7311.json](../../../data/rfc/rfc7311.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC7311-3-1` | The AIGP TLV is encoded as follows: - Type: 1 - Length: 11 - Value: Accumulated IGP Metric. The value field of the AIGP TLV is always 8 octets long (§3) | MUST | 3 | **positive:** `unit/verify` [`TestParseAIGP`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc7311_aigp_test.go#L20). **positive:** `unit/verify` [`TestRFC7311AIGPTLVLengthIsEleven`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc7311_tlv_test.go#L20). **negative:** `unit/verify` [`TestParseAIGPMalformedMetricWrongLength`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc7311_aigp_test.go#L101). **negative:** `unit/verify` [`TestRFC7311AIGPTLVLengthIsEleven`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc7311_tlv_test.go#L21) |
| `RFC7311-3-2` | The value field of the AIGP attribute is defined here to be a set of elements encoded as "Type/Length/Value" (i.e., a set of TLVs). (§3) | MUST | 3 | **positive:** `unit/verify` [`TestParseAIGPMultipleTLVs`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc7311_aigp_test.go#L52). **positive:** `unit/verify` [`TestRFC7311AIGPValueIsWholeTLVSet`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc7311_tlv_test.go#L57). **negative:** `unit/verify` [`TestParseAIGPMalformedTruncatedValue`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc7311_aigp_test.go#L89). **negative:** `unit/verify` [`TestRFC7311AIGPValueIsWholeTLVSet`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc7311_tlv_test.go#L58) |
| `RFC7311-3-3` | Any other AIGP TLVs in the AIGP attribute MUST be passed along unchanged if the AIGP attribute is passed along. (§3) | MUST | 3 | **positive:** `unit/verify` [`TestParseAIGPMultipleTLVs`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc7311_aigp_test.go#L57). **positive:** `unit/verify` [`TestRFC7311OtherAIGPTLVsPassedAlongUnchanged`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_aigp_other_tlvs_test.go#L58). **positive:** `unit/verify` [`TestRFC7311OtherAIGPTLVsSurviveTheCodec`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc7311_tlv_test.go#L85). **negative:** `unit/verify` [`TestAIGPWriteToMultipleTLVs`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc7311_aigp_test.go#L142). **negative:** `unit/verify` [`TestRFC7311OtherAIGPTLVsPassedAlongUnchanged`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_aigp_other_tlvs_test.go#L59) |
| `RFC7311-3.2-4` | If a BGP path attribute is received that has the AIGP attribute codepoint but also has the transitive bit set, the attribute MUST be considered to be a malformed AIGP attribute and MUST be discarded as specified in this section. (§3.2) | MUST | 3.2 | **positive:** `unit/verify` [`TestRFC7311AIGPNonTransitiveKeptOnReceive`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_aigp_receive_test.go#L103). **positive:** `unit/verify` [`TestRFC7606AIGPNonTransitiveIsKept`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7311_aigp_test.go#L104). **negative:** `unit/verify` [`TestRFC7311AIGPTransitiveDiscardedOnReceive`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_aigp_receive_test.go#L60). **negative:** `unit/verify` [`TestRFC7606AIGPTransitiveIsDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7311_aigp_test.go#L68) |
| `RFC7311-3.2-1` | A BGP speaker MUST NOT add the AIGP attribute to any route whose path leads outside the AIGP administrative domain to which the BGP speaker belongs (§3.4.1) | MUST NOT | 3.4.1 | **positive:** `unit/verify` [`TestAIGPConfiguredOriginationRejectsOutsideDomain`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L240). **negative:** `unit/verify` [`TestAIGPConfiguredOriginationRejectsOutsideDomain`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L241) |
| `RFC7311-3.2-5` | When receiving a BGP Update message containing a malformed AIGP attribute, the attribute MUST be treated exactly as if it were an unrecognized non-transitive attribute. That is, it "MUST be quietly ignored and not passed along to other BGP peers" (§3.2) | MUST | 3.2 | **positive:** `unit/verify` [`TestAIGPReceiveValidatesEveryTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L82). **negative:** `unit/verify` [`TestAIGPReceiveValidatesEveryTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L83) |
| `RFC7311-3.2-2` | If an AIGP attribute is received and its first AIGP TLV contains the maximum value 0xffffffffffffffff, the attribute SHOULD be considered to be malformed and SHOULD be discarded (§3.2) | SHOULD | 3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7311-3.2-6` | Note that an AIGP attribute MUST NOT be considered to be malformed because it contains more than one TLV of a given type or because it contains TLVs of unknown types. (§3.2) | MUST NOT | 3.2 | **positive:** `unit/verify` [`TestRFC7311DuplicateAndUnknownTLVsAreWellFormed`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc7311_aigp_tlv_test.go#L50). **negative:** `unit/verify` [`TestRFC7311MalformedVerdictIsForLengthOnly`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc7311_aigp_tlv_test.go#L80) |
| `RFC7311-3.3-1` | An implementation that supports the AIGP attribute MUST support a per-session configuration item, AIGP_SESSION, that indicates whether the attribute is enabled or disabled for use on that session (§3.3) | MUST | 3.3 | **positive:** `unit/verify` [`TestAIGPSessionReceiveBoundary`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L44). **negative:** `unit/verify` [`TestAIGPSessionReceiveBoundary`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L45) |
| `RFC7311-3.3-2` | - For all other External BGP (EBGP) sessions, the default value of AIGP_SESSION MUST be "disabled". (§3.3) | MUST | 3.3 | **positive:** `unit/verify` [`TestAIGPSessionReceiveBoundary`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L46). **negative:** `unit/verify` [`TestAIGPSessionReceiveBoundary`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L47) |
| `RFC7311-3.3-3` | The AIGP attribute MUST NOT be sent on any BGP session for which AIGP_SESSION is disabled (§3.3) | MUST NOT | 3.3 | **positive:** `unit/verify` [`TestAIGPFinalWriterBoundary`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L111). **negative:** `unit/verify` [`TestAIGPFinalWriterBoundary`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L112) |
| `RFC7311-3.3-4` | If an AIGP attribute is received on a BGP session for which AIGP_SESSION is disabled, the attribute MUST be treated exactly as if it were an unrecognized non-transitive attribute (§3.3) | MUST | 3.3 | **positive:** `unit/verify` [`TestAIGPSessionReceiveBoundary`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L48). **negative:** `unit/verify` [`TestAIGPSessionReceiveBoundary`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L49) |
| `RFC7311-3.4.1-1` | An implementation that supports the AIGP attribute MUST support a configuration item, AIGP_ORIGINATE, that enables or disables its creation and attachment to routes (§3.4.1) | MUST | 3.4.1 | **positive:** `unit/verify` [`TestAIGPOriginationControlsReachWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L134). **negative:** `unit/verify` [`TestAIGPOriginationControlsReachWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L135) |
| `RFC7311-3.4.1-2` | The default value of AIGP_ORIGINATE MUST be "disabled" (§3.4.1) | MUST | 3.4.1 | **positive:** `unit/verify` [`TestAIGPOriginationControlsReachWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L136). **negative:** `unit/verify` [`TestAIGPOriginationControlsReachWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L137) |
| `RFC7311-3.4.1-3` | A BGP speaker R MUST NOT add the AIGP attribute to any route for which R does not set itself as the next hop (§3.4.1) | MUST NOT | 3.4.1 | **positive:** `unit/verify` [`TestAIGPOriginationControlsReachWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L138). **negative:** `unit/verify` [`TestAIGPOriginationControlsReachWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L139) |
| `RFC7311-3.4.3-1` | If R1 does not change the next hop of the route, then R1 MUST NOT change the AIGP attribute value of the route (§3.4.3) | MUST NOT | 3.4.3 | **positive:** `unit/verify` [`TestAIGPForwardedMetricsReachFinalWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L164). **negative:** `unit/verify` [`TestAIGPForwardedMetricsReachFinalWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L165) |
| `RFC7311-3.4.3-2` | In all the computations discussed in this section, the AIGP value MUST be capped at its maximum unsigned value 0xffffffffffffffff. (§3.4.3) | MUST | 3.4.3 | **positive:** `unit/verify` [`TestAIGPForwardedMetricsReachFinalWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L166). **positive:** `unit/verify` [`TestAIGPRecursiveDistanceUsesReceivedMetrics`](https://github.com/ze-software/ze/blob/main/internal/component/sysrib/rfc7311_metric_test.go#L12). **positive:** `unit/verify` [`TestRFC7311DirectLinkCostNeverWrapsOrDisappears`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_direct_link_test.go#L21). **positive:** `unit/verify` [`TestRFC7311RecursiveAIGPChangeReachesWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_recursive_readvertise_test.go#L25). **negative:** `unit/verify` [`TestAIGPForwardedMetricsReachFinalWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L167). **negative:** `unit/verify` [`TestAIGPRecursiveDistanceUsesReceivedMetrics`](https://github.com/ze-software/ze/blob/main/internal/component/sysrib/rfc7311_metric_test.go#L13). **negative:** `unit/verify` [`TestRFC7311DirectLinkCostNeverWrapsOrDisappears`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_direct_link_test.go#L22). **negative:** `unit/verify` [`TestRFC7311RecursiveAIGPChangeReachesWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_recursive_readvertise_test.go#L26) |
| `RFC7311-3.4.3-3` | Increasing the AIGP value MUST NOT cause the value to wrap around (§3.4.3) | MUST NOT | 3.4.3 | **positive:** `unit/verify` [`TestAIGPForwardedMetricsReachFinalWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L168). **negative:** `unit/verify` [`TestAIGPForwardedMetricsReachFinalWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L169) |
| `RFC7311-3.4.3-4` | If R1's route to R2 is either (a) an IGP-learned route or (b) a static route that does not require recursive next hop resolution, then R1 MUST increase the value of the AIGP TLV by adding to A the distance from R1 to R2. (§3.4.3) | MUST | 3.4.3 | **positive:** `unit/verify` [`TestAIGPForwardedMetricsReachFinalWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L170). **positive:** `unit/verify` [`TestAIGPRecursiveDistanceUsesReceivedMetrics`](https://github.com/ze-software/ze/blob/main/internal/component/sysrib/rfc7311_metric_test.go#L14). **positive:** `unit/verify` [`TestAIGPRecursiveStaticDistance`](https://github.com/ze-software/ze/blob/main/internal/component/sysrib/rfc7311_metric_test.go#L63). **negative:** `unit/verify` [`TestAIGPForwardedMetricsReachFinalWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L171). **negative:** `unit/verify` [`TestAIGPRecursiveDistanceUsesReceivedMetrics`](https://github.com/ze-software/ze/blob/main/internal/component/sysrib/rfc7311_metric_test.go#L15). **negative:** `unit/verify` [`TestAIGPRecursiveStaticDistance`](https://github.com/ze-software/ze/blob/main/internal/component/sysrib/rfc7311_metric_test.go#L64) |
| `RFC7311-3.4.3-5` | A MUST be increased by a non-zero amount (§3.4.3) | MUST | 3.4.3 | **positive:** `unit/verify` [`TestAIGPForwardedMetricsReachFinalWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L172). **negative:** `unit/verify` [`TestAIGPForwardedMetricsReachFinalWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L173) |
| `RFC7311-3.4.3-6` | Then, when R1 changes the next hop of a route from R2 to R1, the AIGP TLV value MUST be increased by a non-zero amount. (§3.4.3) | MUST | 3.4.3 | **positive:** `unit/verify` [`TestAIGPForwardedMetricsReachFinalWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L174). **positive:** `unit/verify` [`TestRFC7311DirectLinkCostNeverWrapsOrDisappears`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_direct_link_test.go#L19). **positive:** `unit/verify` [`TestRFC7311SourceLinkCostReachesRecipient`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_source_cost_test.go#L30). **positive:** `unit/verify` [`TestRFC7311UnknownCostWithdrawsUntilMetricRecovers`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_source_cost_test.go#L143). **negative:** `unit/verify` [`TestAIGPForwardedMetricsReachFinalWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L175). **negative:** `unit/verify` [`TestRFC7311DirectLinkCostNeverWrapsOrDisappears`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_direct_link_test.go#L20). **negative:** `unit/verify` [`TestRFC7311SourceLinkCostReachesRecipient`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_source_cost_test.go#L31). **negative:** `unit/verify` [`TestRFC7311UnknownCostWithdrawsUntilMetricRecovers`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_source_cost_test.go#L142) |
| `RFC7311-3.4.3-7` | Any change due to (a) in any of these values MUST trigger a new AIGP computation for that route. (§3.4.3) | MUST | 3.4.3 | **positive:** `unit/verify` [`TestAIGPDistanceChangeReselectsRetainedRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc7311_selection_test.go#L102). **positive:** `unit/verify` [`TestAIGPReadvertisesFromReceivedGeneration`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_aigp_readvertise_test.go#L172). **positive:** `unit/verify` [`TestAIGPRecursiveDistanceUsesReceivedMetrics`](https://github.com/ze-software/ze/blob/main/internal/component/sysrib/rfc7311_metric_test.go#L16). **positive:** `unit/verify` [`TestRFC7311ExtendedNextHopCostRecovery`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_cost_boundaries_test.go#L102). **positive:** `unit/verify` [`TestRFC7311ReceivedRecursiveAIGPChangeReachesWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_recursive_readvertise_test.go#L87). **positive:** `unit/verify` [`TestRFC7311RecursiveAIGPChangeReachesWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_recursive_readvertise_test.go#L23). **positive:** `unit/verify` [`TestRFC7311RecursiveZeroCostRemainsAnnounced`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_cost_boundaries_test.go#L26). **negative:** `unit/verify` [`TestAIGPDistanceChangeReselectsRetainedRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc7311_selection_test.go#L103). **negative:** `unit/verify` [`TestAIGPReadvertisesFromReceivedGeneration`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_aigp_readvertise_test.go#L173). **negative:** `unit/verify` [`TestAIGPRecursiveDistanceUsesReceivedMetrics`](https://github.com/ze-software/ze/blob/main/internal/component/sysrib/rfc7311_metric_test.go#L17). **negative:** `unit/verify` [`TestRFC7311ExtendedNextHopCostRecovery`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_cost_boundaries_test.go#L103). **negative:** `unit/verify` [`TestRFC7311ReceivedRecursiveAIGPChangeReachesWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_recursive_readvertise_test.go#L88). **negative:** `unit/verify` [`TestRFC7311RecursiveAIGPChangeReachesWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_recursive_readvertise_test.go#L24). **negative:** `unit/verify` [`TestRFC7311RecursiveZeroCostRemainsAnnounced`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_cost_boundaries_test.go#L27) |
| `RFC7311-4.1-1` | Assuming that the BGP decision process invokes the tie-breaking procedures, the procedures in this section MUST be executed BEFORE any of the tie-breaking procedures described in [BGP], Section 9.1.2.2 are executed. (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestAIGPSelectsStoredRoutesBeforeASPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc7311_selection_test.go#L53). **negative:** `unit/verify` [`TestAIGPSelectsStoredRoutesBeforeASPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc7311_selection_test.go#L54) |
| `RFC7311-3.3-5` | For Internal BGP (IBGP) sessions, and for External BGP (EBGP) sessions between members of the same BGP Confederation [BGP-CONFED], the default value of AIGP_SESSION SHOULD be "enabled". (§3.3) | SHOULD | 3.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7311-3.1-1` | This document only considers the use of the AIGP attribute in networks where each router uses tunneling of some sort to deliver a packet to its BGP next hop. Use of the AIGP attribute in other scenarios is outside the scope of this document. (§3.1) | MAY | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7311-3.4.1-4` | The AIGP attribute may be added only to routes that satisfy one of the following conditions: - The route is a static route, not leading outside the AIGP administrative domain, that is being redistributed into BGP; - The route is an IGP route that is being redistributed into BGP; - The route is an IBGP-learned route whose AS_PATH attribute is empty; or - The route is an EBGP-learned route whose AS_PATH contains only ASes that are in the same AIGP administrative domain as the BGP speaker. (§3.4.1) | MAY | 3.4.1 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

RFC 7311 lists no missing behavior, and every mandatory requirement included in automatic checks has a linked test.

## Test status

A test reads not verified when no record shows it failing after a deliberate fault. Until then, the test has not been shown to detect a fault in the required behavior.

### [`RFC7311-3-1`](#rfc7311-3-1)

The AIGP TLV is encoded as follows: - Type: 1 - Length: 11 - Value: Accumulated IGP Metric. The value field of the AIGP TLV is always 8 octets long (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge 2026-09-30. TestRFC7311AIGPTLVLengthIsEleven: + WriteTo emits 01 000b and 8 value octets and ParseAIGP reads the metric back; - Type-1 TLVs of Length 12 and 10 are refused by ParseAIGP, the registered parser. Forbidden: accepting a Type-1 TLV whose Length is not 11. Records observed red (AIGPMetricOffset).

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestParseAIGPMalformedMetricWrongLength`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc7311_aigp_test.go#L101) | unit/verify | not verified |
| negative | [`TestRFC7311AIGPTLVLengthIsEleven`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc7311_tlv_test.go#L21) | unit/verify | revert, verified |
| positive | [`TestParseAIGP`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc7311_aigp_test.go#L20) | unit/verify | not verified |
| positive | [`TestRFC7311AIGPTLVLengthIsEleven`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc7311_tlv_test.go#L20) | unit/verify | revert, verified |

### [`RFC7311-3-2`](#rfc7311-3-2)

The value field of the AIGP attribute is defined here to be a set of elements encoded as "Type/Length/Value" (i.e., a set of TLVs). (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge 2026-09-30. TestRFC7311AIGPValueIsWholeTLVSet: + a value of two whole TLVs parses into exactly those two; - one, two or three trailing octets that do not form a TLV are refused. Forbidden: accepting a value that is not a whole set of TLVs.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestParseAIGPMalformedTruncatedValue`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc7311_aigp_test.go#L89) | unit/verify | not verified |
| negative | [`TestRFC7311AIGPValueIsWholeTLVSet`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc7311_tlv_test.go#L58) | unit/verify | revert, verified |
| positive | [`TestParseAIGPMultipleTLVs`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc7311_aigp_test.go#L52) | unit/verify | not verified |
| positive | [`TestRFC7311AIGPValueIsWholeTLVSet`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc7311_tlv_test.go#L57) | unit/verify | revert, verified |

### [`RFC7311-3-3`](#rfc7311-3-3)

Any other AIGP TLVs in the AIGP attribute MUST be passed along unchanged if the AIGP attribute is passed along. (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RFC 7311 Section 3 says: Any other AIGP TLVs in the AIGP attribute MUST be passed along unchanged if the AIGP attribute is passed along. The forwarding test still constructs a first metric of 100, a second Type-1 metric of 7 and an unknown Type-9 TLV, then applies a policy replacement that attempts to drop both later TLVs. Transparent and next-hop-self cases require the transmitted AIGP value to retain its full length and every octet after the first TLV, while the first metric is exactly 100 or 130. The fixture change pairs its deliberately built local OPEN with the same immutable settings snapshot as production; it preserves negotiated identity, current encoding contexts and the actual forward-to-socket assertions. applyFactsAIGP restores the received attribute after policy and changes only the first metric when next-hop-self requires accumulation. The codec units separately require exact unknown data and second-Type-1/unknown TLV parse-write round trips. ParseAIGP retains every TLV and WriteTo emits each unchanged value. A dropped or modified later TLV, unconditional attribute suppression, or incorrect first-metric edit fails these exact assertions.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC7311OtherAIGPTLVsPassedAlongUnchanged`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_aigp_other_tlvs_test.go#L59) | unit/verify | revert, verified |
| negative | [`TestAIGPWriteToMultipleTLVs`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc7311_aigp_test.go#L142) | unit/verify | not verified |
| positive | [`TestRFC7311OtherAIGPTLVsPassedAlongUnchanged`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_aigp_other_tlvs_test.go#L58) | unit/verify | revert, verified |
| positive | [`TestParseAIGPMultipleTLVs`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc7311_aigp_test.go#L57) | unit/verify | not verified |
| positive | [`TestRFC7311OtherAIGPTLVsSurviveTheCodec`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc7311_tlv_test.go#L85) | unit/verify | revert, verified |

### [`RFC7311-3.2-4`](#rfc7311-3.2-4)

If a BGP path attribute is received that has the AIGP attribute codepoint but also has the transitive bit set, the attribute MUST be considered to be a malformed AIGP attribute and MUST be discarded as specified in this section. (§3.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: keeping or passing along an AIGP whose transitive bit is set. TestRFC7606AIGPTransitiveIsDiscarded asserts Action==AttributeDiscard on code 26 with DiscardReasonMalformedValue and require.False(found) for AIGP after ApplyAttrDiscard, with ORIGIN, AS_PATH, NEXT_HOP intact; TestRFC7311AIGPTransitiveDiscardedOnReceive asserts the same through enforceRFC7606 with no error (discard, not reset). Positives with flags 0x80 assert RFC7606ActionNone and the AIGP bytes unchanged, so a codepoint-only check goes red.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC7606AIGPTransitiveIsDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7311_aigp_test.go#L68) | unit/verify | revert, verified |
| negative | [`TestRFC7311AIGPTransitiveDiscardedOnReceive`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_aigp_receive_test.go#L60) | unit/verify | revert, verified |
| positive | [`TestRFC7606AIGPNonTransitiveIsKept`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7311_aigp_test.go#L104) | unit/verify | revert, verified |
| positive | [`TestRFC7311AIGPNonTransitiveKeptOnReceive`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_aigp_receive_test.go#L103) | unit/verify | revert, verified |

### [`RFC7311-3.2-1`](#rfc7311-3.2-1)

A BGP speaker MUST NOT add the AIGP attribute to any route whose path leads outside the AIGP administrative domain to which the BGP speaker belongs (§3.4.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestAIGPConfiguredOriginationRejectsOutsideDomain`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L241) | unit/verify | not verified |
| positive | [`TestAIGPConfiguredOriginationRejectsOutsideDomain`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L240) | unit/verify | not verified |

### [`RFC7311-3.2-5`](#rfc7311-3.2-5)

When receiving a BGP Update message containing a malformed AIGP attribute, the attribute MUST be treated exactly as if it were an unrecognized non-transitive attribute. That is, it "MUST be quietly ignored and not passed along to other BGP peers" (§3.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: acting on, or passing along, a malformed AIGP, or escalating beyond ignoring it. TestAIGPReceiveValidatesEveryTLV (IBGP, AIGP enabled) appends a truncated trailing TLV and asserts require.Equal(!malformed, present) on the payload leaving enforceRFC7606 (the bytes passed along), require.NoError (quietly, no session reset) and the NLRI kept (route not withdrawn). The well-formed half (unknown TLV plus duplicate metric) keeps the metric, so a blanket AIGP strip goes red.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestAIGPReceiveValidatesEveryTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L83) | unit/verify | not verified |
| positive | [`TestAIGPReceiveValidatesEveryTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L82) | unit/verify | not verified |

### [`RFC7311-3.2-6`](#rfc7311-3.2-6)

Note that an AIGP attribute MUST NOT be considered to be malformed because it contains more than one TLV of a given type or because it contains TLVs of unknown types. (§3.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: judging an AIGP malformed for a repeated TLV type or an unknown TLV type. TestRFC7311DuplicateAndUnknownTLVsAreWellFormed parses two Type-1 TLVs and a Type-200 TLV with require.NoError, so rejecting either shape goes red; ParseAIGP and the receive validator validateAIGPAttr share AIGPMetricOffset. TestRFC7311MalformedVerdictIsForLengthOnly shows the malformed verdict exists (ErrMalformedValue on a Type-1 TLV of Length 8) and is lifted when only the length is repaired.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC7311MalformedVerdictIsForLengthOnly`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc7311_aigp_tlv_test.go#L80) | unit/verify | revert, verified |
| positive | [`TestRFC7311DuplicateAndUnknownTLVsAreWellFormed`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc7311_aigp_tlv_test.go#L50) | unit/verify | revert, verified |

### [`RFC7311-3.3-1`](#rfc7311-3.3-1)

An implementation that supports the AIGP attribute MUST support a per-session configuration item, AIGP_SESSION, that indicates whether the attribute is enabled or disabled for use on that session (§3.3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestAIGPSessionReceiveBoundary`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L45) | unit/verify | not verified |
| positive | [`TestAIGPSessionReceiveBoundary`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L44) | unit/verify | not verified |

### [`RFC7311-3.3-2`](#rfc7311-3.3-2)

- For all other External BGP (EBGP) sessions, the default value of AIGP_SESSION MUST be "disabled". (§3.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: an EBGP (non-confederation) session defaulting AIGP_SESSION to enabled. TestAIGPSessionReceiveBoundary case external-default (peer AS 65002, local 65001, AIGPSession nil) asserts require.Equal(false, present) for AIGP after enforceRFC7606, which goes red on an enabled default; external-enabled shows the item is honoured when set; internal-default keeps AIGP, so the check is not a blanket strip.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestAIGPSessionReceiveBoundary`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L47) | unit/verify | not verified |
| positive | [`TestAIGPSessionReceiveBoundary`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L46) | unit/verify | not verified |

### [`RFC7311-3.3-3`](#rfc7311-3.3-3)

The AIGP attribute MUST NOT be sent on any BGP session for which AIGP_SESSION is disabled (§3.3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestAIGPFinalWriterBoundary`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L112) | unit/verify | revert, verified |
| positive | [`TestAIGPFinalWriterBoundary`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L111) | unit/verify | revert, verified |

### [`RFC7311-3.3-4`](#rfc7311-3.3-4)

If an AIGP attribute is received on a BGP session for which AIGP_SESSION is disabled, the attribute MUST be treated exactly as if it were an unrecognized non-transitive attribute (§3.3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestAIGPSessionReceiveBoundary`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L49) | unit/verify | not verified |
| positive | [`TestAIGPSessionReceiveBoundary`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L48) | unit/verify | not verified |

### [`RFC7311-3.4.1-1`](#rfc7311-3.4.1-1)

An implementation that supports the AIGP attribute MUST support a configuration item, AIGP_ORIGINATE, that enables or disables its creation and attachment to routes (§3.4.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestAIGPOriginationControlsReachWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L135) | unit/verify | not verified |
| positive | [`TestAIGPOriginationControlsReachWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L134) | unit/verify | not verified |

### [`RFC7311-3.4.1-2`](#rfc7311-3.4.1-2)

The default value of AIGP_ORIGINATE MUST be "disabled" (§3.4.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestAIGPOriginationControlsReachWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L137) | unit/verify | not verified |
| positive | [`TestAIGPOriginationControlsReachWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L136) | unit/verify | not verified |

### [`RFC7311-3.4.1-3`](#rfc7311-3.4.1-3)

A BGP speaker R MUST NOT add the AIGP attribute to any route for which R does not set itself as the next hop (§3.4.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestAIGPOriginationControlsReachWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L139) | unit/verify | not verified |
| positive | [`TestAIGPOriginationControlsReachWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L138) | unit/verify | not verified |

### [`RFC7311-3.4.3-1`](#rfc7311-3.4.3-1)

If R1 does not change the next hop of the route, then R1 MUST NOT change the AIGP attribute value of the route (§3.4.3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestAIGPForwardedMetricsReachFinalWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L165) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| positive | [`TestAIGPForwardedMetricsReachFinalWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L164) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |

### [`RFC7311-3.4.3-2`](#rfc7311-3.4.3-2)

In all the computations discussed in this section, the AIGP value MUST be capped at its maximum unsigned value 0xffffffffffffffff. (§3.4.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent source rejudgment on 2026-11-11 after the withdrawal builder signature migration. Read the complete RFC 7311, its short summary and derived requirement map, the prior verdict and discrimination entries, and all four currently tagged units. RFC 7311 Section 3.4.3: "In all the computations discussed in this section, the AIGP value MUST be capped at its maximum unsigned value 0xffffffffffffffff." Q1: TestRFC7311DirectLinkCostNeverWrapsOrDisappears and TestAIGPForwardedMetricsReachFinalWire assert the actual final writer's finite and overflowing source additions, not merely a helper return. TestAIGPRecursiveDistanceUsesReceivedMetrics uses an actual RIB to assert exact recursive sums 330 and 530 and overflowing maximum. TestRFC7311RecursiveAIGPChangeReachesWire separately observes recursive-plus-terminal saturation and saturation only when the source's received 100 is added; each is followed by exact finite recovery to 507. Q2: wrapping addition, unconditional saturation, omitting the terminal cost or reusing the previous advertisement violates those exact values or the complete transition list. The producer-panic records show dependence on their named producers, not semantic mutation coverage. After this source audit, main renewed all sixteen changed-unit/polarity bindings across the six affected requirements in job-refusal-rfc7311-renew-20feecf3.log. The inspected log contains sixteen distinct recorded tuples; each follows a clean baseline PASS and the expected producer-disabled panic FAIL. Q3: valid TLVs, retained source bytes, unchanged-next-hop controls and finite recovery isolate arithmetic from parse refusal or dropped routes. The final writer requires actual frames, so absence is not success. Q4: the four units compose direct source addition, recursive received-BGP addition, terminal distance and final source addition. Add caps overflowing sums without wrapping; Resolve uses Add for every received BGP and terminal contribution, applyFactsAIGP uses Add for forwarding, and replayAIGP uses Add from the retained received metric. The two changed tests only unpack buildWithdrawalPayload's new error and assert success; their original inputs, nonzero guards, metrics, withdrawal bytes and writer assertions remain. This source auditor ran no tests, builds, proof records, check or stamp. The subsequently inspected main-thread renewals are separate execution evidence and add no live interoperability claim. The verdict is an independent judgment of the reviewed assertions and producers, with freshness and execution still owned by main.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC7311DirectLinkCostNeverWrapsOrDisappears`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_direct_link_test.go#L22) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| negative | [`TestAIGPForwardedMetricsReachFinalWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L167) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| negative | [`TestRFC7311RecursiveAIGPChangeReachesWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_recursive_readvertise_test.go#L26) | unit/verify | revert, verified |
| negative | [`TestAIGPRecursiveDistanceUsesReceivedMetrics`](https://github.com/ze-software/ze/blob/main/internal/component/sysrib/rfc7311_metric_test.go#L13) | unit/verify | revert, verified |
| positive | [`TestRFC7311DirectLinkCostNeverWrapsOrDisappears`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_direct_link_test.go#L21) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| positive | [`TestAIGPForwardedMetricsReachFinalWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L166) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| positive | [`TestRFC7311RecursiveAIGPChangeReachesWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_recursive_readvertise_test.go#L25) | unit/verify | revert, verified |
| positive | [`TestAIGPRecursiveDistanceUsesReceivedMetrics`](https://github.com/ze-software/ze/blob/main/internal/component/sysrib/rfc7311_metric_test.go#L12) | unit/verify | revert, verified |

### [`RFC7311-3.4.3-3`](#rfc7311-3.4.3-3)

Increasing the AIGP value MUST NOT cause the value to wrap around (§3.4.3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestAIGPForwardedMetricsReachFinalWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L169) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| positive | [`TestAIGPForwardedMetricsReachFinalWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L168) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |

### [`RFC7311-3.4.3-4`](#rfc7311-3.4.3-4)

If R1's route to R2 is either (a) an IGP-learned route or (b) a static route that does not require recursive next hop resolution, then R1 MUST increase the value of the AIGP TLV by adding to A the distance from R1 to R2. (§3.4.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent source rejudgment on 2026-11-11 after the withdrawal builder signature migration. Read the complete RFC 7311, summary, derived row, prior verdict, current discrimination entries, all three tagged units and their producers. RFC 7311 Section 3.4.3: "If R1's route to R2 is either (a) an IGP-learned route or (b) a static route that does not require recursive next hop resolution, then R1 MUST increase the value of the AIGP TLV by adding to A the distance from R1 to R2." This follows the next-hop-self condition and permits an IGP-computed or policy distance. Q1: TestAIGPForwardedMetricsReachFinalWire asserts final metric 130 from received 100 plus resolved 30, with announcement NLRI, while transparent forwarding retains 100 despite a policy replacement. TestAIGPRecursiveDistanceUsesReceivedMetrics proves the real resolver counts terminal 30 once and excludes BGP MED values 9000 and 8000. TestAIGPRecursiveStaticDistance separately proves recursive static distance 30, unresolved refusal and nonrecursive configured static distance 25. Q2: omitting or duplicating terminal distance, using a recursive route's preference metric, adding MED, or accumulating on an unchanged next hop fails these exact assertions. Existing producer-panic records establish dependence on applyFactsAIGP only; they are not described here as semantic mutation proofs. After this source audit, main renewed all sixteen affected unit/polarity bindings in job-refusal-rfc7311-renew-20feecf3.log, including this changed unit's pair for this requirement. The inspected log shows a clean baseline PASS followed by the expected producer-disabled panic FAIL for each distinct recorded tuple. Q3: valid announcement/TLV construction, exact final-writer route assertions, deliberately different MED/preference values and the transparent control isolate the obligation. The socket carrier's mocked resolved distance is not credited as resolver evidence; the separately tagged actual-RIB tests supply that boundary. Q4: the combined tests cover resolved interior and static distances under the conditional next-hop-self obligation. nhResolver.IGPMetric delegates to Resolve, whose non-BGP, nonrecursive route terminates with its distance counted once; aigpIncrement carries that result and applyFactsAIGP adds it to the received metric after egress edits. General and RS forwarding call this producer. The changed tagged unit only unpacks the builder error and asserts success, retaining every prior case and wire assertion. No tests, builds, proof records, check or stamp ran in this source audit. Main's subsequently inspected renewals are separate execution evidence; no additional interoperability is claimed.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestAIGPForwardedMetricsReachFinalWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L171) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| negative | [`TestAIGPRecursiveDistanceUsesReceivedMetrics`](https://github.com/ze-software/ze/blob/main/internal/component/sysrib/rfc7311_metric_test.go#L15) | unit/verify | not verified |
| negative | [`TestAIGPRecursiveStaticDistance`](https://github.com/ze-software/ze/blob/main/internal/component/sysrib/rfc7311_metric_test.go#L64) | unit/verify | not verified |
| positive | [`TestAIGPForwardedMetricsReachFinalWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L170) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| positive | [`TestAIGPRecursiveDistanceUsesReceivedMetrics`](https://github.com/ze-software/ze/blob/main/internal/component/sysrib/rfc7311_metric_test.go#L14) | unit/verify | not verified |
| positive | [`TestAIGPRecursiveStaticDistance`](https://github.com/ze-software/ze/blob/main/internal/component/sysrib/rfc7311_metric_test.go#L63) | unit/verify | not verified |

### [`RFC7311-3.4.3-5`](#rfc7311-3.4.3-5)

A MUST be increased by a non-zero amount (§3.4.3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestAIGPForwardedMetricsReachFinalWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L173) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| positive | [`TestAIGPForwardedMetricsReachFinalWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L172) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |

### [`RFC7311-3.4.3-6`](#rfc7311-3.4.3-6)

Then, when R1 changes the next hop of a route from R2 to R1, the AIGP TLV value MUST be increased by a non-zero amount. (§3.4.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent source rejudgment on 2026-11-11 after the withdrawal builder signature migration. Read the complete RFC 7311, summary, derived row, prior verdict, discrimination entries, all four tagged units and their real forwarding producers. RFC 7311 Section 3.4.3: "Then, when R1 changes the next hop of a route from R2 to R1, the AIGP TLV value MUST be increased by a non-zero amount." The preceding sentence describes a direct EBGP link on which no IGP is running. Q1: TestRFC7311DirectLinkCostNeverWrapsOrDisappears requires configured cost 7 to turn received 100 into final-wire 107 and verifies an exact route withdrawal for absent cost. TestAIGPForwardedMetricsReachFinalWire separately covers policy-link increment 7 with no IGP resolution and zero-cost refusal. TestRFC7311SourceLinkCostReachesRecipient enters general and RS cached forwarding with source costs 7 and 19, a deliberately different destination cost 43, no resolving IGP route, exact output metrics 107 and 119, unchanged-next-hop 100 controls and maximum boundaries. TestRFC7311UnknownCostWithdrawsUntilMetricRecovers checks both rails, unknown/zero costs and three advertisement histories; known distance 11 restores the original route as exactly 111 without another source UPDATE. Q2: using the destination cost, omitting or doubling the increment, dropping AIGP, or losing recovery fails exact wire values, prefix fields or frame counts. The panic/revert records prove producer dependence, not a semantic wrong-value mutation. After this source audit, main renewed all sixteen changed-unit/polarity bindings in job-refusal-rfc7311-renew-20feecf3.log, including both changed units' pairs for this requirement. The inspected log shows a clean baseline PASS followed by the expected producer-disabled panic FAIL for each distinct recorded tuple. Q3: valid source TLVs, explicit accepting policies/send grants, no interior route, different source/destination costs and immutable source bytes isolate link-cost provenance. Never-advertised routes correctly emit no withdrawal; already-advertised routes must emit an exact pure withdrawal. Withholding is Ze's chosen refusal when it cannot meet the nonzero increment, not an RFC mandate to withdraw. Q4: sourceAIGPLinkMetric reads the received peer's settings, aigpIncrement uses that configured cost only for the direct received next hop when no usable resolved cost exists, and applyFactsAIGP preserves unchanged next hops or adds the known nonzero amount. General and RS callers use the same received source identity. Both migrated tagged tests retain all old inputs, guards, exact metrics and withdrawals while adding a builder success check. This source audit ran no tests, builds, proof records, check or stamp. Main's subsequently inspected renewals are separate execution evidence; no additional live interoperability is claimed.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC7311DirectLinkCostNeverWrapsOrDisappears`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_direct_link_test.go#L20) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| negative | [`TestAIGPForwardedMetricsReachFinalWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L175) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| negative | [`TestRFC7311SourceLinkCostReachesRecipient`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_source_cost_test.go#L31) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| negative | [`TestRFC7311UnknownCostWithdrawsUntilMetricRecovers`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_source_cost_test.go#L142) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| positive | [`TestRFC7311DirectLinkCostNeverWrapsOrDisappears`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_direct_link_test.go#L19) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| positive | [`TestAIGPForwardedMetricsReachFinalWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L174) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| positive | [`TestRFC7311SourceLinkCostReachesRecipient`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_source_cost_test.go#L30) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| positive | [`TestRFC7311UnknownCostWithdrawsUntilMetricRecovers`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_source_cost_test.go#L143) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |

### [`RFC7311-3.4.3-7`](#rfc7311-3.4.3-7)

Any change due to (a) in any of these values MUST trigger a new AIGP computation for that route. (§3.4.3)

Audit verdict: enforced (the tests do what the requirement demands), stale-unit: internal/component/bgp/plugins/rib/rfc7311_selection_test.go::TestAIGPDistanceChangeReselectsRetainedRoutes, internal/component/bgp/plugins/rib/rfc7311_selection_test.go::TestAIGPDistanceChangeReselectsRetainedRoutes#2 moved. Re-judged 2026-10-31 by an independent auditor who authored none of the change. RFC text (rfc7311): "Any change due to (a) in any of these values MUST trigger a new AIGP computation for that route." be95c10053 replaced lowLiveSettings with acceptingLiveSettings for source and destination: the same settings plus FinalAccept import and export policies with no refs (filter_chain_test.go acceptingPolicies). An accepting policy neither refuses nor rewrites, so the AIGP recomputation the test observes on the wire still comes from the producer. Assertions unchanged. Verdict unchanged: enforced. Earlier judgements are in this file's git history. Maintenance rejudgment 2026-11-16 preserves the full current population: fourteen tagged covers, fourteen tests-map keys and fourteen units-map keys across seven top-level units, each retaining its base key and #2 key. The units read are TestAIGPDistanceChangeReselectsRetainedRoutes in rib/rfc7311_selection_test.go; TestAIGPReadvertisesFromReceivedGeneration in reactor/rfc7311_aigp_readvertise_test.go; TestRFC7311ExtendedNextHopCostRecovery and TestRFC7311RecursiveZeroCostRemainsAnnounced in reactor/rfc7311_cost_boundaries_test.go; TestRFC7311ReceivedRecursiveAIGPChangeReachesWire and TestRFC7311RecursiveAIGPChangeReachesWire in reactor/rfc7311_recursive_readvertise_test.go; and TestAIGPRecursiveDistanceUsesReceivedMetrics in sysrib/rfc7311_metric_test.go. The twelve existing reactor/sysrib discrimination observations retain their identities, producers and scopes. Their assertions still cover cache-independent received-generation wire replay (107 then 111); resolved recursive zero remaining an announcement, with initial-zero and positive-to-zero cases; negotiated 16/32-octet IPv6 next-hop recovery of IPv4 NLRI with exact withdrawal/MP_REACH/AIGP outcomes; automatically observed recursive saturation and finite-recovery wire transitions; established-session received AIGP-only replacement of immediate and deeper recursive BGP hops with an unchanged control route and retained original source attributes; and direct recursive-distance computations 330 then 530, unchanged recursion, explicit missing AIGP and saturation. The recursive-zero transition and extended-next-hop recovery cases explicitly call readvertiseAIGP, and the sysrib case explicitly invokes IGPMetric for the next computation; their scope remains those assertions. The automatic-trigger wire cases continue to use the production subscriber or established-session path. In the RIB selection fixture, aigpSelectionNextHopRoutes now installs actual connected terminal Loc-RIB routes and independently requires igpcost.Resolve to report them resolved; the original custom distance provider, received attributes, peer metadata and original winner/received-metric/unchanged-distance assertions remain. The additive recursive-aigp child uses actual igpcost.Resolve over its own Loc-RIB: received recursive AIGP 5 plus terminal cost 7 yields distance 12 and candidate total 22, beating the competing total 30. Changing only that recursive route's received AIGP to 30 yields distance 37 and candidate total 47, and the production runAIGPSelection subscriber must move the winner to the competing route without another candidate UPDATE or a manual selection call. Both peer RIB entries retain received metric 10. Reinserting the same recursive path reports no Loc-RIB change, preserves the winner and distances, and the final best-path check reports no change. The main session's job-ci-aigp-real-recursive-selection-3c324434.log reports PASS for this outer unit and its recursive-aigp child; this is scoped test-run evidence only. Main-thread native 7e9becde passed the clean unit then observed the recursive child fail only its B-winner wait at line 218 after normal-return suppression of all subscriber notifications; initial A selection remained intact. Native 491c612e passed clean then observed the original terminal unchanged-result assertion fail at line 142 after the sameBest branch falsely reported a change. Both records now use explicit hand-written semantic provenance; neither is a panic or interruption. Only the two RIB tuples were replaced; all twelve other record objects remain unchanged. This scoped renewal makes no full-suite-green claim.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestAIGPDistanceChangeReselectsRetainedRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc7311_selection_test.go#L103) | unit/verify | mutant, unit-changed (the tagged unit's behavior changed since the red was observed) |
| negative | [`TestAIGPReadvertisesFromReceivedGeneration`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_aigp_readvertise_test.go#L173) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| negative | [`TestRFC7311ExtendedNextHopCostRecovery`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_cost_boundaries_test.go#L103) | unit/verify | revert, verified |
| negative | [`TestRFC7311RecursiveZeroCostRemainsAnnounced`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_cost_boundaries_test.go#L27) | unit/verify | revert, verified |
| negative | [`TestRFC7311ReceivedRecursiveAIGPChangeReachesWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_recursive_readvertise_test.go#L88) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| negative | [`TestRFC7311RecursiveAIGPChangeReachesWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_recursive_readvertise_test.go#L24) | unit/verify | revert, verified |
| negative | [`TestAIGPRecursiveDistanceUsesReceivedMetrics`](https://github.com/ze-software/ze/blob/main/internal/component/sysrib/rfc7311_metric_test.go#L17) | unit/verify | revert, verified |
| positive | [`TestAIGPDistanceChangeReselectsRetainedRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc7311_selection_test.go#L102) | unit/verify | mutant, unit-changed (the tagged unit's behavior changed since the red was observed) |
| positive | [`TestAIGPReadvertisesFromReceivedGeneration`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_aigp_readvertise_test.go#L172) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| positive | [`TestRFC7311ExtendedNextHopCostRecovery`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_cost_boundaries_test.go#L102) | unit/verify | revert, verified |
| positive | [`TestRFC7311RecursiveZeroCostRemainsAnnounced`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_cost_boundaries_test.go#L26) | unit/verify | revert, verified |
| positive | [`TestRFC7311ReceivedRecursiveAIGPChangeReachesWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_recursive_readvertise_test.go#L87) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| positive | [`TestRFC7311RecursiveAIGPChangeReachesWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_recursive_readvertise_test.go#L23) | unit/verify | revert, verified |
| positive | [`TestAIGPRecursiveDistanceUsesReceivedMetrics`](https://github.com/ze-software/ze/blob/main/internal/component/sysrib/rfc7311_metric_test.go#L16) | unit/verify | revert, verified |

### [`RFC7311-4.1-1`](#rfc7311-4.1-1)

Assuming that the BGP decision process invokes the tie-breaking procedures, the procedures in this section MUST be executed BEFORE any of the tie-breaking procedures described in [BGP], Section 9.1.2.2 are executed. (§4.1)

Audit verdict: enforced (the tests do what the requirement demands), stale-unit: internal/component/bgp/plugins/rib/rfc7311_selection_test.go::TestAIGPSelectsStoredRoutesBeforeASPath, internal/component/bgp/plugins/rib/rfc7311_selection_test.go::TestAIGPSelectsStoredRoutesBeforeASPath#2 moved. Forbidden: running a Section 9.1.2.2 tie-breaker before the AIGP step. TestAIGPSelectsStoredRoutesBeforeASPath gives peer B the lower AIGP sum (20 plus 10) and the longer AS_PATH (3 versus 1); require.Equal(nhB, winner.NextHop) goes red if AS_PATH length, the first 9.1.2.2 tie-breaker, runs first. Negatives assert LOCAL_PREF (Section 9.1.1, not a tie-breaker) still wins before AIGP and that a missing AIGP is not metric zero. Maintenance rejudgment 2026-11-16 preserves both tagged covers of this same top-level unit, its base and #2 test keys, and its base and #2 unit-map keys. The full body and its compareBeforeMED/compareAIGP/checkRouteBestChange producers were read. Actual connected terminal routes now make each next hop independently resolvable in the Loc-RIB before candidate admission, while the original custom provider still returns costs 100 for A and 10 for B. The original positive case retains received metrics 5 and 20, equal LOCAL_PREF 100, and AS_PATH lengths 1 and 3: the required winner remains B with received AIGP 20 in both its best-change event and collected replay, rather than accumulated metric 30. The first negative replaces A with LOCAL_PREF 101 and no AIGP and requires A to win with AIGPPresent false; the second returns A to LOCAL_PREF 100 without AIGP and requires B to win, so absence cannot act as metric zero or survive to the shorter-AS_PATH criterion. The producer still compares LOCAL_PREF before the AIGP step and AIGP before AS_PATH. The fixture prerequisite adds reachability while preserving all of those original assertions and the full requirement scope. The additive recursive-aigp child belongs to the separate distance-reselection unit and creates no additional tag key for this ordering row. Main-thread native 4319577b passed clean then observed the first expected B-winner assertion fail at line 83 when numeric AIGP ranking was bypassed while presence ordering remained. Native aaca26ef passed clean, retained the numeric and LOCAL_PREF phases, then observed the named missing-AIGP winner assertion fail at line 99 when presence confinement was bypassed. Both failures returned normally; both records retain explicit hand-written semantic provenance. No full-suite-green claim is made.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestAIGPSelectsStoredRoutesBeforeASPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc7311_selection_test.go#L54) | unit/verify | mutant, unit-changed (the tagged unit's behavior changed since the red was observed) |
| positive | [`TestAIGPSelectsStoredRoutesBeforeASPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc7311_selection_test.go#L53) | unit/verify | mutant, unit-changed (the tagged unit's behavior changed since the red was observed) |

## Requirement list review

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | rfc2119 |
| Source | rfc/full/rfc7311.txt |
| Source fingerprint | cfedbb96c63b8a55 |
| Record | rfc/extraction/rfc7311.json |
| Mapped sentences | 20 |
| Declined as scope | 2 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | reviewed | not stated |
| `1` | not stated | 0 | reviewed | not stated |
| `2` | not stated | 0 | reviewed | not stated |
| `3` | not stated | 1 | reviewed | not stated |
| `3.1` | not stated | 0 | reviewed | not stated |
| `3.2` | not stated | 4 | reviewed | not stated |
| `3.3` | not stated | 5 | reviewed | not stated |
| `3.4` | not stated | 0 | reviewed | not stated |
| `3.4.1` | not stated | 4 | reviewed | not stated |
| `3.4.2` | not stated | 0 | reviewed | not stated |
| `3.4.3` | not stated | 7 | reviewed | not stated |
| `4` | not stated | 0 | reviewed | not stated |
| `4.1` | not stated | 1 | reviewed | not stated |
| `4.2` | not stated | 0 | reviewed | not stated |
| `5` | not stated | 0 | reviewed | not stated |
| `6` | not stated | 0 | reviewed | not stated |
| `7` | not stated | 0 | reviewed | not stated |
| `8` | not stated | 0 | reviewed | not stated |
| `9` | not stated | 0 | reviewed | not stated |
| `9.1` | not stated | 0 | reviewed | not stated |
| `9.2` | not stated | 0 | reviewed | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `3.2:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The sentence opens "That is," and restates the preceding MUST of the same paragraph, quoting [BGP] Section 5; site 3.2:1 already maps that obligation. | That is, it "MUST be quietly ignored and not passed along to other BGP peers" (see [BGP], Section 5). |
| `3.3:5` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The sentence opens "That is," and restates the preceding MUST of the same paragraph, quoting [BGP] Section 5; site 3.3:4 already maps that obligation. | That is, it "MUST be quietly ignored and not passed along to other BGP peers" (see [BGP], Section 5). |

## Superseded

No document obsoletes RFC 7311, so its obligations are stated where they were written.
