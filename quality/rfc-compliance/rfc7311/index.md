# RFC 7311 - The Accumulated IGP Metric Attribute for BGP

Partial. Every requirement this repository extracted from RFC 7311, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 100.0% | 22 of 22 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 22 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 22 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 22 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| No test at all | 0.0% | 0 of 22 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 40.3% | 31 of 77 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 22 | of 26 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 22 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 22 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 22 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 22 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

The 8 shares marked as a part above are the whole of the 22 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Partial |
| Enrolment | Enrolled |
| Requirements | 26 |
| Gated MUST-level | 22 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 77 |
| Tagged units | 77 |
| Recorded audit verdicts | 12 |
| Discrimination records | 31 |
| Summary | `rfc/short/rfc7311.md` |
| Requirement shard | `rfc/requirements/rfc7311.md` |
| RFC text | `rfc/full/rfc7311.txt` |

## Enrolment

Enrolled: Ze implements AIGP validation, per-session policy, configured origination, forwarding accumulation and best-path selection. Recursive metric changes reach retained advertisements and the selecting RIB, including its subprocess transport. Requirement identities follow the mappings in `rfc/extraction/rfc7311.json`; the legacy RFC7311-3.2-1 identity maps to the origination prohibition in Section 3.4.1, not a separate propagation rule in Section 3.2.

## What the public ledger says

**Status:** Partial

**What the ledger says is covered:**

- Attribute codec and presentation
- receive validation
- per-session enablement
- configured origination
- next-hop-self accumulation
- retained-generation re-advertisement
- native and forked best-path reselection.


**What the ledger says remains**

Native next-hop-self source-cost propagation is not currently established end to end: its FRR scenario fails before the later recursive metric transitions. Automatic origination policies are not enabled. Origination accepts explicitly configured metrics and remains subject to the domain and next-hop restrictions in Section 3.4.1.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 22 | one part of the gated population |
| Annotated (including scoped evidence) | 0 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **22** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (22):** [`RFC7311-3-1`](#rfc7311-3-1), [`RFC7311-3-2`](#rfc7311-3-2), [`RFC7311-3-3`](#rfc7311-3-3), [`RFC7311-3.2-4`](#rfc7311-3.2-4), [`RFC7311-3.2-1`](#rfc7311-3.2-1), [`RFC7311-3.2-5`](#rfc7311-3.2-5), [`RFC7311-3.2-6`](#rfc7311-3.2-6), [`RFC7311-3.3-1`](#rfc7311-3.3-1), [`RFC7311-3.3-2`](#rfc7311-3.3-2), [`RFC7311-3.3-3`](#rfc7311-3.3-3), [`RFC7311-3.3-4`](#rfc7311-3.3-4), [`RFC7311-3.4.1-1`](#rfc7311-3.4.1-1), [`RFC7311-3.4.1-2`](#rfc7311-3.4.1-2), [`RFC7311-3.4.1-3`](#rfc7311-3.4.1-3), [`RFC7311-3.4.3-1`](#rfc7311-3.4.3-1), [`RFC7311-3.4.3-2`](#rfc7311-3.4.3-2), [`RFC7311-3.4.3-3`](#rfc7311-3.4.3-3), [`RFC7311-3.4.3-4`](#rfc7311-3.4.3-4), [`RFC7311-3.4.3-5`](#rfc7311-3.4.3-5), [`RFC7311-3.4.3-6`](#rfc7311-3.4.3-6), [`RFC7311-3.4.3-7`](#rfc7311-3.4.3-7), [`RFC7311-4.1-1`](#rfc7311-4.1-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC7311-3-1` | The AIGP TLV is encoded as follows: - Type: 1 - Length: 11 - Value: Accumulated IGP Metric. The value field of the AIGP TLV is always 8 octets long (§3) | MUST | 3 | **positive:** `unit/verify` [`TestParseAIGP`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc7311_aigp_test.go#L20). **positive:** `unit/verify` [`TestRFC7311AIGPTLVLengthIsEleven`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc7311_tlv_test.go#L20). **negative:** `unit/verify` [`TestParseAIGPMalformedMetricWrongLength`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc7311_aigp_test.go#L101). **negative:** `unit/verify` [`TestRFC7311AIGPTLVLengthIsEleven`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc7311_tlv_test.go#L21) |
| `RFC7311-3-2` | The value field of the AIGP attribute is defined here to be a set of elements encoded as "Type/Length/Value" (i.e., a set of TLVs). (§3) | MUST | 3 | **positive:** `unit/verify` [`TestParseAIGPMultipleTLVs`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc7311_aigp_test.go#L52). **positive:** `unit/verify` [`TestRFC7311AIGPValueIsWholeTLVSet`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc7311_tlv_test.go#L57). **negative:** `unit/verify` [`TestParseAIGPMalformedTruncatedValue`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc7311_aigp_test.go#L89). **negative:** `unit/verify` [`TestRFC7311AIGPValueIsWholeTLVSet`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc7311_tlv_test.go#L58) |
| `RFC7311-3-3` | Any other AIGP TLVs in the AIGP attribute MUST be passed along unchanged if the AIGP attribute is passed along. (§3) | MUST | 3 | **positive:** `unit/verify` [`TestParseAIGPMultipleTLVs`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc7311_aigp_test.go#L57). **positive:** `unit/verify` [`TestRFC7311OtherAIGPTLVsPassedAlongUnchanged`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_aigp_other_tlvs_test.go#L58). **positive:** `unit/verify` [`TestRFC7311OtherAIGPTLVsSurviveTheCodec`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc7311_tlv_test.go#L85). **negative:** `unit/verify` [`TestAIGPWriteToMultipleTLVs`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc7311_aigp_test.go#L142). **negative:** `unit/verify` [`TestRFC7311OtherAIGPTLVsPassedAlongUnchanged`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_aigp_other_tlvs_test.go#L59) |
| `RFC7311-3.2-4` | If a BGP path attribute is received that has the AIGP attribute codepoint but also has the transitive bit set, the attribute MUST be considered to be a malformed AIGP attribute and MUST be discarded as specified in this section. (§3.2) | MUST | 3.2 | **positive:** `unit/verify` [`TestRFC7311AIGPNonTransitiveKeptOnReceive`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_aigp_receive_test.go#L103). **positive:** `unit/verify` [`TestRFC7606AIGPNonTransitiveIsKept`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7311_aigp_test.go#L104). **negative:** `unit/verify` [`TestRFC7311AIGPTransitiveDiscardedOnReceive`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_aigp_receive_test.go#L60). **negative:** `unit/verify` [`TestRFC7606AIGPTransitiveIsDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7311_aigp_test.go#L68) |
| `RFC7311-3.2-1` | A BGP speaker MUST NOT add the AIGP attribute to any route whose path leads outside the AIGP administrative domain to which the BGP speaker belongs (§3.4.1) | MUST NOT | 3.4.1 | **positive:** `unit/verify` [`TestAIGPConfiguredOriginationRejectsOutsideDomain`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L241). **negative:** `unit/verify` [`TestAIGPConfiguredOriginationRejectsOutsideDomain`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L242) |
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
| `RFC7311-3.4.3-2` | In all the computations discussed in this section, the AIGP value MUST be capped at its maximum unsigned value 0xffffffffffffffff. (§3.4.3) | MUST | 3.4.3 | **positive:** `unit/verify` [`TestAIGPForwardedMetricsReachFinalWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L166). **positive:** `unit/verify` [`TestRFC7311DirectLinkCostNeverWrapsOrDisappears`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_direct_link_test.go#L20). **positive:** `unit/verify` [`TestRFC7311RecursiveAIGPChangeReachesWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_recursive_readvertise_test.go#L19). **negative:** `unit/verify` [`TestAIGPForwardedMetricsReachFinalWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L167). **negative:** `unit/verify` [`TestRFC7311DirectLinkCostNeverWrapsOrDisappears`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_direct_link_test.go#L21). **negative:** `unit/verify` [`TestRFC7311RecursiveAIGPChangeReachesWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_recursive_readvertise_test.go#L20) |
| `RFC7311-3.4.3-3` | Increasing the AIGP value MUST NOT cause the value to wrap around (§3.4.3) | MUST NOT | 3.4.3 | **positive:** `unit/verify` [`TestAIGPForwardedMetricsReachFinalWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L168). **negative:** `unit/verify` [`TestAIGPForwardedMetricsReachFinalWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L169) |
| `RFC7311-3.4.3-4` | If R1's route to R2 is either (a) an IGP-learned route or (b) a static route that does not require recursive next hop resolution, then R1 MUST increase the value of the AIGP TLV by adding to A the distance from R1 to R2. (§3.4.3) | MUST | 3.4.3 | **positive:** `unit/verify` [`TestAIGPForwardedMetricsReachFinalWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L170). **positive:** `unit/verify` [`TestAIGPRecursiveDistanceUsesReceivedMetrics`](https://github.com/ze-software/ze/blob/main/internal/component/sysrib/rfc7311_metric_test.go#L12). **positive:** `unit/verify` [`TestAIGPRecursiveStaticDistance`](https://github.com/ze-software/ze/blob/main/internal/component/sysrib/rfc7311_metric_test.go#L61). **negative:** `unit/verify` [`TestAIGPForwardedMetricsReachFinalWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L171). **negative:** `unit/verify` [`TestAIGPRecursiveDistanceUsesReceivedMetrics`](https://github.com/ze-software/ze/blob/main/internal/component/sysrib/rfc7311_metric_test.go#L13). **negative:** `unit/verify` [`TestAIGPRecursiveStaticDistance`](https://github.com/ze-software/ze/blob/main/internal/component/sysrib/rfc7311_metric_test.go#L62) |
| `RFC7311-3.4.3-5` | A MUST be increased by a non-zero amount (§3.4.3) | MUST | 3.4.3 | **positive:** `unit/verify` [`TestAIGPForwardedMetricsReachFinalWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L172). **negative:** `unit/verify` [`TestAIGPForwardedMetricsReachFinalWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L173) |
| `RFC7311-3.4.3-6` | Then, when R1 changes the next hop of a route from R2 to R1, the AIGP TLV value MUST be increased by a non-zero amount. (§3.4.3) | MUST | 3.4.3 | **positive:** `unit/verify` [`TestAIGPForwardedMetricsReachFinalWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L174). **positive:** `unit/verify` [`TestRFC7311DirectLinkCostNeverWrapsOrDisappears`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_direct_link_test.go#L18). **positive:** `unit/verify` [`TestRFC7311SourceLinkCostReachesRecipient`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_source_cost_test.go#L29). **positive:** `unit/verify` [`TestRFC7311UnknownCostWithdrawsUntilMetricRecovers`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_source_cost_test.go#L138). **negative:** `unit/verify` [`TestAIGPForwardedMetricsReachFinalWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L175). **negative:** `unit/verify` [`TestRFC7311DirectLinkCostNeverWrapsOrDisappears`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_direct_link_test.go#L19). **negative:** `unit/verify` [`TestRFC7311SourceLinkCostReachesRecipient`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_source_cost_test.go#L30). **negative:** `unit/verify` [`TestRFC7311UnknownCostWithdrawsUntilMetricRecovers`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_source_cost_test.go#L137) |
| `RFC7311-3.4.3-7` | Any change due to (a) in any of these values MUST trigger a new AIGP computation for that route. (§3.4.3) | MUST | 3.4.3 | **positive:** `unit/verify` [`TestAIGPDistanceChangeReselectsRetainedRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc7311_selection_test.go#L82). **positive:** `unit/verify` [`TestAIGPReadvertisesFromReceivedGeneration`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_aigp_readvertise_test.go#L169). **positive:** `unit/verify` [`TestAIGPRecursiveDistanceUsesReceivedMetrics`](https://github.com/ze-software/ze/blob/main/internal/component/sysrib/rfc7311_metric_test.go#L14). **positive:** `unit/verify` [`TestRFC7311ExtendedNextHopCostRecovery`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_cost_boundaries_test.go#L99). **positive:** `unit/verify` [`TestRFC7311RecursiveAIGPChangeReachesWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_recursive_readvertise_test.go#L17). **positive:** `unit/verify` [`TestRFC7311RecursiveZeroCostRemainsAnnounced`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_cost_boundaries_test.go#L26). **negative:** `unit/verify` [`TestAIGPDistanceChangeReselectsRetainedRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc7311_selection_test.go#L83). **negative:** `unit/verify` [`TestAIGPReadvertisesFromReceivedGeneration`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_aigp_readvertise_test.go#L170). **negative:** `unit/verify` [`TestAIGPRecursiveDistanceUsesReceivedMetrics`](https://github.com/ze-software/ze/blob/main/internal/component/sysrib/rfc7311_metric_test.go#L15). **negative:** `unit/verify` [`TestRFC7311ExtendedNextHopCostRecovery`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_cost_boundaries_test.go#L100). **negative:** `unit/verify` [`TestRFC7311RecursiveAIGPChangeReachesWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_recursive_readvertise_test.go#L18). **negative:** `unit/verify` [`TestRFC7311RecursiveZeroCostRemainsAnnounced`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_cost_boundaries_test.go#L27) |
| `RFC7311-4.1-1` | Assuming that the BGP decision process invokes the tie-breaking procedures, the procedures in this section MUST be executed BEFORE any of the tie-breaking procedures described in [BGP], Section 9.1.2.2 are executed. (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestAIGPSelectsStoredRoutesBeforeASPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc7311_selection_test.go#L34). **negative:** `unit/verify` [`TestAIGPSelectsStoredRoutesBeforeASPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc7311_selection_test.go#L35) |
| `RFC7311-3.3-5` | For Internal BGP (IBGP) sessions, and for External BGP (EBGP) sessions between members of the same BGP Confederation [BGP-CONFED], the default value of AIGP_SESSION SHOULD be "enabled". (§3.3) | SHOULD | 3.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7311-3.1-1` | This document only considers the use of the AIGP attribute in networks where each router uses tunneling of some sort to deliver a packet to its BGP next hop. Use of the AIGP attribute in other scenarios is outside the scope of this document. (§3.1) | MAY | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7311-3.4.1-4` | The AIGP attribute may be added only to routes that satisfy one of the following conditions: - The route is a static route, not leading outside the AIGP administrative domain, that is being redistributed into BGP; - The route is an IGP route that is being redistributed into BGP; - The route is an IBGP-learned route whose AS_PATH attribute is empty; or - The route is an EBGP-learned route whose AS_PATH contains only ASes that are in the same AIGP administrative domain as the BGP speaker. (§3.4.1) | MAY | 3.4.1 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

RFC 7311 declares no gap, and every gated MUST it carries has a test bound to it.

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC7311-3-1`](#rfc7311-3-1)

The AIGP TLV is encoded as follows: - Type: 1 - Length: 11 - Value: Accumulated IGP Metric. The value field of the AIGP TLV is always 8 octets long (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge 2026-09-30. TestRFC7311AIGPTLVLengthIsEleven: + WriteTo emits 01 000b and 8 value octets and ParseAIGP reads the metric back; - Type-1 TLVs of Length 12 and 10 are refused by ParseAIGP, the registered parser. Forbidden: accepting a Type-1 TLV whose Length is not 11. Records observed red (AIGPMetricOffset).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestParseAIGPMalformedMetricWrongLength`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc7311_aigp_test.go#L101) | unit/verify | unproven |
| negative | [`TestRFC7311AIGPTLVLengthIsEleven`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc7311_tlv_test.go#L21) | unit/verify | revert, verified |
| positive | [`TestParseAIGP`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc7311_aigp_test.go#L20) | unit/verify | unproven |
| positive | [`TestRFC7311AIGPTLVLengthIsEleven`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc7311_tlv_test.go#L20) | unit/verify | revert, verified |

### [`RFC7311-3-2`](#rfc7311-3-2)

The value field of the AIGP attribute is defined here to be a set of elements encoded as "Type/Length/Value" (i.e., a set of TLVs). (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge 2026-09-30. TestRFC7311AIGPValueIsWholeTLVSet: + a value of two whole TLVs parses into exactly those two; - one, two or three trailing octets that do not form a TLV are refused. Forbidden: accepting a value that is not a whole set of TLVs.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestParseAIGPMalformedTruncatedValue`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc7311_aigp_test.go#L89) | unit/verify | unproven |
| negative | [`TestRFC7311AIGPValueIsWholeTLVSet`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc7311_tlv_test.go#L58) | unit/verify | revert, verified |
| positive | [`TestParseAIGPMultipleTLVs`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc7311_aigp_test.go#L52) | unit/verify | unproven |
| positive | [`TestRFC7311AIGPValueIsWholeTLVSet`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc7311_tlv_test.go#L57) | unit/verify | revert, verified |

### [`RFC7311-3-3`](#rfc7311-3-3)

Any other AIGP TLVs in the AIGP attribute MUST be passed along unchanged if the AIGP attribute is passed along. (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge 2026-09-30. Reactor TestRFC7311OtherAIGPTLVsPassedAlongUnchanged drives a received [T1 100][T1 7][T9 abcd] through applyFactsAIGP, buildModifiedPayload and fwdBatchHandler to the socket, transparent and next-hop-self, under a policy replacement that drops the other TLVs (R1(b)); the octets after the first TLV reach the wire unchanged. Codec half TestRFC7311OtherAIGPTLVsSurviveTheCodec. Judge break: OpCopy of received[:11] on both rails turned both subtests red. Supersedes the earlier wrong verdict: the tags now prove the other-TLV passing, not first-TLV use.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7311OtherAIGPTLVsPassedAlongUnchanged`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_aigp_other_tlvs_test.go#L59) | unit/verify | revert, verified |
| negative | [`TestAIGPWriteToMultipleTLVs`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc7311_aigp_test.go#L142) | unit/verify | unproven |
| positive | [`TestRFC7311OtherAIGPTLVsPassedAlongUnchanged`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_aigp_other_tlvs_test.go#L58) | unit/verify | revert, verified |
| positive | [`TestParseAIGPMultipleTLVs`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc7311_aigp_test.go#L57) | unit/verify | unproven |
| positive | [`TestRFC7311OtherAIGPTLVsSurviveTheCodec`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc7311_tlv_test.go#L85) | unit/verify | revert, verified |

### [`RFC7311-3.2-4`](#rfc7311-3.2-4)

If a BGP path attribute is received that has the AIGP attribute codepoint but also has the transitive bit set, the attribute MUST be considered to be a malformed AIGP attribute and MUST be discarded as specified in this section. (§3.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: keeping or passing along an AIGP whose transitive bit is set. TestRFC7606AIGPTransitiveIsDiscarded asserts Action==AttributeDiscard on code 26 with DiscardReasonMalformedValue and require.False(found) for AIGP after ApplyAttrDiscard, with ORIGIN, AS_PATH, NEXT_HOP intact; TestRFC7311AIGPTransitiveDiscardedOnReceive asserts the same through enforceRFC7606 with no error (discard, not reset). Positives with flags 0x80 assert RFC7606ActionNone and the AIGP bytes unchanged, so a codepoint-only check goes red.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7606AIGPTransitiveIsDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7311_aigp_test.go#L68) | unit/verify | revert, verified |
| negative | [`TestRFC7311AIGPTransitiveDiscardedOnReceive`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_aigp_receive_test.go#L60) | unit/verify | revert, verified |
| positive | [`TestRFC7606AIGPNonTransitiveIsKept`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7311_aigp_test.go#L104) | unit/verify | revert, verified |
| positive | [`TestRFC7311AIGPNonTransitiveKeptOnReceive`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_aigp_receive_test.go#L103) | unit/verify | revert, verified |

### [`RFC7311-3.2-1`](#rfc7311-3.2-1)

A BGP speaker MUST NOT add the AIGP attribute to any route whose path leads outside the AIGP administrative domain to which the BGP speaker belongs (§3.4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestAIGPConfiguredOriginationRejectsOutsideDomain`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L242) | unit/verify | unproven |
| positive | [`TestAIGPConfiguredOriginationRejectsOutsideDomain`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L241) | unit/verify | unproven |

### [`RFC7311-3.2-5`](#rfc7311-3.2-5)

When receiving a BGP Update message containing a malformed AIGP attribute, the attribute MUST be treated exactly as if it were an unrecognized non-transitive attribute. That is, it "MUST be quietly ignored and not passed along to other BGP peers" (§3.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: acting on, or passing along, a malformed AIGP, or escalating beyond ignoring it. TestAIGPReceiveValidatesEveryTLV (IBGP, AIGP enabled) appends a truncated trailing TLV and asserts require.Equal(!malformed, present) on the payload leaving enforceRFC7606 (the bytes passed along), require.NoError (quietly, no session reset) and the NLRI kept (route not withdrawn). The well-formed half (unknown TLV plus duplicate metric) keeps the metric, so a blanket AIGP strip goes red.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestAIGPReceiveValidatesEveryTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L83) | unit/verify | unproven |
| positive | [`TestAIGPReceiveValidatesEveryTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L82) | unit/verify | unproven |

### [`RFC7311-3.2-6`](#rfc7311-3.2-6)

Note that an AIGP attribute MUST NOT be considered to be malformed because it contains more than one TLV of a given type or because it contains TLVs of unknown types. (§3.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: judging an AIGP malformed for a repeated TLV type or an unknown TLV type. TestRFC7311DuplicateAndUnknownTLVsAreWellFormed parses two Type-1 TLVs and a Type-200 TLV with require.NoError, so rejecting either shape goes red; ParseAIGP and the receive validator validateAIGPAttr share AIGPMetricOffset. TestRFC7311MalformedVerdictIsForLengthOnly shows the malformed verdict exists (ErrMalformedValue on a Type-1 TLV of Length 8) and is lifted when only the length is repaired.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7311MalformedVerdictIsForLengthOnly`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc7311_aigp_tlv_test.go#L80) | unit/verify | revert, verified |
| positive | [`TestRFC7311DuplicateAndUnknownTLVsAreWellFormed`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc7311_aigp_tlv_test.go#L50) | unit/verify | revert, verified |

### [`RFC7311-3.3-1`](#rfc7311-3.3-1)

An implementation that supports the AIGP attribute MUST support a per-session configuration item, AIGP_SESSION, that indicates whether the attribute is enabled or disabled for use on that session (§3.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestAIGPSessionReceiveBoundary`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L45) | unit/verify | unproven |
| positive | [`TestAIGPSessionReceiveBoundary`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L44) | unit/verify | unproven |

### [`RFC7311-3.3-2`](#rfc7311-3.3-2)

- For all other External BGP (EBGP) sessions, the default value of AIGP_SESSION MUST be "disabled". (§3.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: an EBGP (non-confederation) session defaulting AIGP_SESSION to enabled. TestAIGPSessionReceiveBoundary case external-default (peer AS 65002, local 65001, AIGPSession nil) asserts require.Equal(false, present) for AIGP after enforceRFC7606, which goes red on an enabled default; external-enabled shows the item is honoured when set; internal-default keeps AIGP, so the check is not a blanket strip.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestAIGPSessionReceiveBoundary`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L47) | unit/verify | unproven |
| positive | [`TestAIGPSessionReceiveBoundary`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L46) | unit/verify | unproven |

### [`RFC7311-3.3-3`](#rfc7311-3.3-3)

The AIGP attribute MUST NOT be sent on any BGP session for which AIGP_SESSION is disabled (§3.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestAIGPFinalWriterBoundary`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L112) | unit/verify | revert, verified |
| positive | [`TestAIGPFinalWriterBoundary`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L111) | unit/verify | revert, verified |

### [`RFC7311-3.3-4`](#rfc7311-3.3-4)

If an AIGP attribute is received on a BGP session for which AIGP_SESSION is disabled, the attribute MUST be treated exactly as if it were an unrecognized non-transitive attribute (§3.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestAIGPSessionReceiveBoundary`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L49) | unit/verify | unproven |
| positive | [`TestAIGPSessionReceiveBoundary`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L48) | unit/verify | unproven |

### [`RFC7311-3.4.1-1`](#rfc7311-3.4.1-1)

An implementation that supports the AIGP attribute MUST support a configuration item, AIGP_ORIGINATE, that enables or disables its creation and attachment to routes (§3.4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestAIGPOriginationControlsReachWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L135) | unit/verify | unproven |
| positive | [`TestAIGPOriginationControlsReachWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L134) | unit/verify | unproven |

### [`RFC7311-3.4.1-2`](#rfc7311-3.4.1-2)

The default value of AIGP_ORIGINATE MUST be "disabled" (§3.4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestAIGPOriginationControlsReachWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L137) | unit/verify | unproven |
| positive | [`TestAIGPOriginationControlsReachWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L136) | unit/verify | unproven |

### [`RFC7311-3.4.1-3`](#rfc7311-3.4.1-3)

A BGP speaker R MUST NOT add the AIGP attribute to any route for which R does not set itself as the next hop (§3.4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestAIGPOriginationControlsReachWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L139) | unit/verify | unproven |
| positive | [`TestAIGPOriginationControlsReachWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L138) | unit/verify | unproven |

### [`RFC7311-3.4.3-1`](#rfc7311-3.4.3-1)

If R1 does not change the next hop of the route, then R1 MUST NOT change the AIGP attribute value of the route (§3.4.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestAIGPForwardedMetricsReachFinalWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L165) | unit/verify | unproven |
| positive | [`TestAIGPForwardedMetricsReachFinalWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L164) | unit/verify | unproven |

### [`RFC7311-3.4.3-2`](#rfc7311-3.4.3-2)

In all the computations discussed in this section, the AIGP value MUST be capped at its maximum unsigned value 0xffffffffffffffff. (§3.4.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RFC7311 Section 3.4.3: "In all the computations discussed in this section, the AIGP value MUST be capped at its maximum unsigned value 0xffffffffffffffff. Increasing the AIGP value MUST NOT cause the value to wrap around." All six tagged covers (three units) read. TestAIGPForwardedMetricsReachFinalWire requires exact final wire 100+30=130 and (max-5)+30=max. TestRFC7311DirectLinkCostNeverWrapsOrDisappears requires 100+7=107 and (max-5)+7=max. TestRFC7311RecursiveAIGPChangeReachesWire uses actual recursive Loc-RIB changes: initial 100+200+7=307; recursive max-5 overflows terminal addition; max-50 remains finite after terminal 7 but overflows source 100 addition; intervening and final recursive 400 yield exact 507. Finite and overflow controls isolate arithmetic, not malformed-attribute errors. Producers igpcost.Add/Resolve saturate recursive and terminal accumulation; applyFactsAIGP/replayAIGP saturate source addition. Consumer assertions inspect final UPDATE AIGP and distinguish finite recovery from reusing previously sent maximum. No new in-scope defect found. Independent supplemental source rejudgment 2026-10-06. Full RFC and every current tagged unit read. Scoped git diff HEAD for processing, direct-link and cost-boundaries carrier files is empty: their stale units are preexisting judgment debt relative to HEAD, not current mechanical edits. All prior enforced verdicts retained; no upgrade_reason is needed. No tests, semantic mutants or named peers executed; live named-peer proof remains pending and producer-halt records are not semantic evidence.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7311DirectLinkCostNeverWrapsOrDisappears`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_direct_link_test.go#L21) | unit/verify | revert, verified |
| negative | [`TestAIGPForwardedMetricsReachFinalWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L167) | unit/verify | unproven |
| negative | [`TestRFC7311RecursiveAIGPChangeReachesWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_recursive_readvertise_test.go#L20) | unit/verify | revert, verified |
| positive | [`TestRFC7311DirectLinkCostNeverWrapsOrDisappears`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_direct_link_test.go#L20) | unit/verify | revert, verified |
| positive | [`TestAIGPForwardedMetricsReachFinalWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L166) | unit/verify | unproven |
| positive | [`TestRFC7311RecursiveAIGPChangeReachesWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_recursive_readvertise_test.go#L19) | unit/verify | revert, verified |

### [`RFC7311-3.4.3-3`](#rfc7311-3.4.3-3)

Increasing the AIGP value MUST NOT cause the value to wrap around (§3.4.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestAIGPForwardedMetricsReachFinalWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L169) | unit/verify | unproven |
| positive | [`TestAIGPForwardedMetricsReachFinalWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L168) | unit/verify | unproven |

### [`RFC7311-3.4.3-4`](#rfc7311-3.4.3-4)

If R1's route to R2 is either (a) an IGP-learned route or (b) a static route that does not require recursive next hop resolution, then R1 MUST increase the value of the AIGP TLV by adding to A the distance from R1 to R2. (§3.4.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RFC7311 Section 3.4.3: "Suppose R1 changes the next hop of the route from R2 to R1. If R1's route to R2 is either (a) an IGP-learned route or (b) a static route that does not require recursive next hop resolution, then R1 MUST increase the value of the AIGP TLV by adding to A the distance from R1 to R2. This distance is either the IGP-computed distance from R1 to R2 or some value determined by policy. However, A MUST be increased by a non-zero amount." All six covers (three units) read. TestAIGPForwardedMetricsReachFinalWire requires final wire 130 from received 100 and distance 30 after next-hop-self; unchanged next hop retains 100 despite policy replacement, with direction/NLRI and source-immutability assertions. TestAIGPRecursiveDistanceUsesReceivedMetrics distinguishes received 100+200 from MED-like 9000/8000 and adds terminal interior 30 exactly once, yielding 330 and then 530. TestAIGPRecursiveStaticDistance ignores recursive static preference 9000, resolves terminal 30, rejects an unresolved chain and requires nonrecursive static distance 25. sysrib.IGPMetric delegates to igpcost.Resolve; its non-BGP nonrecursive branch returns terminal Metric; applyFactsAIGP consumes that distance. Resolver and wire adder are complementary separate tests, not a live IGP topology. No new in-scope defect found. Independent supplemental source rejudgment 2026-10-06. Full RFC and every current tagged unit read. Scoped git diff HEAD for processing, direct-link and cost-boundaries carrier files is empty: their stale units are preexisting judgment debt relative to HEAD, not current mechanical edits. All prior enforced verdicts retained; no upgrade_reason is needed. No tests, semantic mutants or named peers executed; live named-peer proof remains pending and producer-halt records are not semantic evidence.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestAIGPForwardedMetricsReachFinalWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L171) | unit/verify | unproven |
| negative | [`TestAIGPRecursiveDistanceUsesReceivedMetrics`](https://github.com/ze-software/ze/blob/main/internal/component/sysrib/rfc7311_metric_test.go#L13) | unit/verify | unproven |
| negative | [`TestAIGPRecursiveStaticDistance`](https://github.com/ze-software/ze/blob/main/internal/component/sysrib/rfc7311_metric_test.go#L62) | unit/verify | unproven |
| positive | [`TestAIGPForwardedMetricsReachFinalWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L170) | unit/verify | unproven |
| positive | [`TestAIGPRecursiveDistanceUsesReceivedMetrics`](https://github.com/ze-software/ze/blob/main/internal/component/sysrib/rfc7311_metric_test.go#L12) | unit/verify | unproven |
| positive | [`TestAIGPRecursiveStaticDistance`](https://github.com/ze-software/ze/blob/main/internal/component/sysrib/rfc7311_metric_test.go#L61) | unit/verify | unproven |

### [`RFC7311-3.4.3-5`](#rfc7311-3.4.3-5)

A MUST be increased by a non-zero amount (§3.4.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestAIGPForwardedMetricsReachFinalWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L173) | unit/verify | unproven |
| positive | [`TestAIGPForwardedMetricsReachFinalWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L172) | unit/verify | unproven |

### [`RFC7311-3.4.3-6`](#rfc7311-3.4.3-6)

Then, when R1 changes the next hop of a route from R2 to R1, the AIGP TLV value MUST be increased by a non-zero amount. (§3.4.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RFC 7311 Section 3.4.3, in the context of EBGP neighbors with a direct link running no IGP: 'Then, when R1 changes the next hop of a route from R2 to R1, the AIGP TLV value MUST be increased by a non-zero amount.' Re-read all four tagged units under internal/component/bgp/reactor/: rfc7311_processing_test.go::TestAIGPForwardedMetricsReachFinalWire, rfc7311_direct_link_test.go::TestRFC7311DirectLinkCostNeverWrapsOrDisappears, rfc7311_source_cost_test.go::TestRFC7311SourceLinkCostReachesRecipient and TestRFC7311UnknownCostWithdrawsUntilMetricRecovers. The first two directly call the arithmetic producer but now distinguish an exact withdrawal from an attribute-free announcement. The source-cost test closes their prior integration gap: a received UPDATE enters notifyMessageReceiver and the cache, then general ForwardUpdate or the live RS cached fallback through ForwardUpdatesDirect, finally reaching the worker/session socket. No IGP distance is available; source costs 7 and 19 deliberately differ from destination cost 43. The recipient must contain the same NLRI, no withdrawal, rewritten local NEXT_HOP, exact AIGP TLV/flags and metrics 107 or 119, with received source bytes unchanged. Transparent controls preserve metric 100, including unavailable cost, and boundary cases saturate rather than wrap as required by the surrounding paragraph. Unknown/resolved-zero controls on both rails require the exact withdrawn prefix with no announcement or attributes, both initially and after a prior metric-107 advertisement; supplying distance 11 restores the retained received metric as 111, without a new source UPDATE. These are substantive source-to-recipient assertions, not direct helper success or mere AIGP absence. Producing symbols: forward_aigp.go::sourceAIGPLinkMetric, aigpIncrement, applyFactsAIGP; reactor_api_forward.go::forwardUpdateSection and ForwardUpdate; forward_rs.go::sourceUsesCachedForward and reactorForwardRS (AIGP selects cached forwarding). Native panic records prove applyFactsAIGP reachability only, not arithmetic, source provenance or withdrawal semantics. The optional metric-comparability SHOULD is not silently promoted to this MUST row. No new checks, semantic mutation execution or interop pass is claimed.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7311DirectLinkCostNeverWrapsOrDisappears`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_direct_link_test.go#L19) | unit/verify | revert, verified |
| negative | [`TestAIGPForwardedMetricsReachFinalWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L175) | unit/verify | unproven |
| negative | [`TestRFC7311SourceLinkCostReachesRecipient`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_source_cost_test.go#L30) | unit/verify | revert, verified |
| negative | [`TestRFC7311UnknownCostWithdrawsUntilMetricRecovers`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_source_cost_test.go#L137) | unit/verify | revert, verified |
| positive | [`TestRFC7311DirectLinkCostNeverWrapsOrDisappears`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_direct_link_test.go#L18) | unit/verify | revert, verified |
| positive | [`TestAIGPForwardedMetricsReachFinalWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L174) | unit/verify | unproven |
| positive | [`TestRFC7311SourceLinkCostReachesRecipient`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_source_cost_test.go#L29) | unit/verify | revert, verified |
| positive | [`TestRFC7311UnknownCostWithdrawsUntilMetricRecovers`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_source_cost_test.go#L138) | unit/verify | revert, verified |

### [`RFC7311-3.4.3-7`](#rfc7311-3.4.3-7)

Any change due to (a) in any of these values MUST trigger a new AIGP computation for that route. (§3.4.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RFC7311 Section 3.4.3: "The AIGP TLV value of a given route depends on (a) the AIGP TLV values of all the next hops that are recursively resolved during this procedure, and (b) the IGP distance to any next hop that is not recursively resolved. Any change due to (a) in any of these values MUST trigger a new AIGP computation for that route. Whether a change due to (b) triggers a new AIGP computation depends upon whether the change in IGP distance exceeds a configurable threshold." All twelve current covers (six units) read, including two cost-boundaries units missing from the old four-unit audit map. TestRFC7311RecursiveAIGPChangeReachesWire supplies automatic trigger proof: after one received UPDATE it changes actual recursive Loc-RIB AIGP while runAIGPAdvertisements runs, waits for worker completion and requires wire 307,max,507,max,507. TestAIGPReadvertisesFromReceivedGeneration checks cache eviction, exactly two frames and 107 to 111 after interior distance change. TestAIGPRecursiveDistanceUsesReceivedMetrics tests deeper recursive 330 to 530, unchanged 330, missing AIGP and saturation. TestAIGPDistanceChangeReselectsRetainedRoutes explicitly reselects and requires changed Loc-RIB winner while preserving received AIGP 10. TestRFC7311RecursiveZeroCostRemainsAnnounced manually replays recursive 7 to 0 and requires announced AIGP 100, not withdrawal. TestRFC7311ExtendedNextHopCostRecovery manually replays after distance 11 becomes available and requires exact restored IPv4 MP_REACH with IPv6 self next hop and AIGP 111 for both 16/32-byte received forms. Manual replay/reselection carriers are supplementary, not credited as automatic-trigger proof. Production Loc-RIB equality includes AIGP/presence; changes increment revision and notify subscribers; runAIGPAdvertisements scans and replayAIGP recomputes from retained received bytes, dispatching normal forwarding to fwdBatchHandler and successful-flush receipt update. The mandatory recursive clause is enforced; no configurable clause-(b) threshold feature is claimed. No new in-scope defect found. Independent supplemental source rejudgment 2026-10-06. Full RFC and every current tagged unit read. Scoped git diff HEAD for processing, direct-link and cost-boundaries carrier files is empty: their stale units are preexisting judgment debt relative to HEAD, not current mechanical edits. All prior enforced verdicts retained; no upgrade_reason is needed. No tests, semantic mutants or named peers executed; live named-peer proof remains pending and producer-halt records are not semantic evidence.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestAIGPDistanceChangeReselectsRetainedRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc7311_selection_test.go#L83) | unit/verify | unproven |
| negative | [`TestAIGPReadvertisesFromReceivedGeneration`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_aigp_readvertise_test.go#L170) | unit/verify | unproven |
| negative | [`TestRFC7311ExtendedNextHopCostRecovery`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_cost_boundaries_test.go#L100) | unit/verify | revert, verified |
| negative | [`TestRFC7311RecursiveZeroCostRemainsAnnounced`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_cost_boundaries_test.go#L27) | unit/verify | revert, verified |
| negative | [`TestRFC7311RecursiveAIGPChangeReachesWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_recursive_readvertise_test.go#L18) | unit/verify | revert, verified |
| negative | [`TestAIGPRecursiveDistanceUsesReceivedMetrics`](https://github.com/ze-software/ze/blob/main/internal/component/sysrib/rfc7311_metric_test.go#L15) | unit/verify | unproven |
| positive | [`TestAIGPDistanceChangeReselectsRetainedRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc7311_selection_test.go#L82) | unit/verify | unproven |
| positive | [`TestAIGPReadvertisesFromReceivedGeneration`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_aigp_readvertise_test.go#L169) | unit/verify | unproven |
| positive | [`TestRFC7311ExtendedNextHopCostRecovery`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_cost_boundaries_test.go#L99) | unit/verify | revert, verified |
| positive | [`TestRFC7311RecursiveZeroCostRemainsAnnounced`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_cost_boundaries_test.go#L26) | unit/verify | revert, verified |
| positive | [`TestRFC7311RecursiveAIGPChangeReachesWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_recursive_readvertise_test.go#L17) | unit/verify | revert, verified |
| positive | [`TestAIGPRecursiveDistanceUsesReceivedMetrics`](https://github.com/ze-software/ze/blob/main/internal/component/sysrib/rfc7311_metric_test.go#L14) | unit/verify | unproven |

### [`RFC7311-4.1-1`](#rfc7311-4.1-1)

Assuming that the BGP decision process invokes the tie-breaking procedures, the procedures in this section MUST be executed BEFORE any of the tie-breaking procedures described in [BGP], Section 9.1.2.2 are executed. (§4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: running a Section 9.1.2.2 tie-breaker before the AIGP step. TestAIGPSelectsStoredRoutesBeforeASPath gives peer B the lower AIGP sum (20 plus 10) and the longer AS_PATH (3 versus 1); require.Equal(nhB, winner.NextHop) goes red if AS_PATH length, the first 9.1.2.2 tie-breaker, runs first. Negatives assert LOCAL_PREF (Section 9.1.1, not a tie-breaker) still wins before AIGP and that a missing AIGP is not metric zero.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestAIGPSelectsStoredRoutesBeforeASPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc7311_selection_test.go#L35) | unit/verify | unproven |
| positive | [`TestAIGPSelectsStoredRoutesBeforeASPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc7311_selection_test.go#L34) | unit/verify | unproven |

## Extraction sign-off

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
| `front` | not stated | 0 | walked | not stated |
| `1` | not stated | 0 | walked | not stated |
| `2` | not stated | 0 | walked | not stated |
| `3` | not stated | 1 | walked | not stated |
| `3.1` | not stated | 0 | walked | not stated |
| `3.2` | not stated | 4 | walked | not stated |
| `3.3` | not stated | 5 | walked | not stated |
| `3.4` | not stated | 0 | walked | not stated |
| `3.4.1` | not stated | 4 | walked | not stated |
| `3.4.2` | not stated | 0 | walked | not stated |
| `3.4.3` | not stated | 7 | walked | not stated |
| `4` | not stated | 0 | walked | not stated |
| `4.1` | not stated | 1 | walked | not stated |
| `4.2` | not stated | 0 | walked | not stated |
| `5` | not stated | 0 | walked | not stated |
| `6` | not stated | 0 | walked | not stated |
| `7` | not stated | 0 | walked | not stated |
| `8` | not stated | 0 | walked | not stated |
| `9` | not stated | 0 | walked | not stated |
| `9.1` | not stated | 0 | walked | not stated |
| `9.2` | not stated | 0 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `3.2:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The sentence opens "That is," and restates the preceding MUST of the same paragraph, quoting [BGP] Section 5; site 3.2:1 already maps that obligation. | That is, it "MUST be quietly ignored and not passed along to other BGP peers" (see [BGP], Section 5). |
| `3.3:5` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The sentence opens "That is," and restates the preceding MUST of the same paragraph, quoting [BGP] Section 5; site 3.3:4 already maps that obligation. | That is, it "MUST be quietly ignored and not passed along to other BGP peers" (see [BGP], Section 5). |

## Superseded

No document obsoletes RFC 7311, so its obligations are stated where they were written.
