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
| No test at all | 0.0% | 0 of 22 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 14.8% | 8 of 54 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 22 | of 26 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 22 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 22 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 22 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 22 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

The 7 shares marked as a part above are the whole of the 22 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
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
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 54 |
| Tagged units | 54 |
| Recorded audit verdicts | 0 |
| Discrimination records | 8 |
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


**What the ledger says remains:**

Automatic origination policies are not enabled. Origination accepts explicitly configured metrics and remains subject to the domain and next-hop restrictions in Section 3.4.1.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 22 | one part of the gated population |
| Annotated instead of tested | 0 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **22** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (22):** [`RFC7311-3-1`](#rfc7311-3-1), [`RFC7311-3-2`](#rfc7311-3-2), [`RFC7311-3-3`](#rfc7311-3-3), [`RFC7311-3.2-4`](#rfc7311-3.2-4), [`RFC7311-3.2-1`](#rfc7311-3.2-1), [`RFC7311-3.2-5`](#rfc7311-3.2-5), [`RFC7311-3.2-6`](#rfc7311-3.2-6), [`RFC7311-3.3-1`](#rfc7311-3.3-1), [`RFC7311-3.3-2`](#rfc7311-3.3-2), [`RFC7311-3.3-3`](#rfc7311-3.3-3), [`RFC7311-3.3-4`](#rfc7311-3.3-4), [`RFC7311-3.4.1-1`](#rfc7311-3.4.1-1), [`RFC7311-3.4.1-2`](#rfc7311-3.4.1-2), [`RFC7311-3.4.1-3`](#rfc7311-3.4.1-3), [`RFC7311-3.4.3-1`](#rfc7311-3.4.3-1), [`RFC7311-3.4.3-2`](#rfc7311-3.4.3-2), [`RFC7311-3.4.3-3`](#rfc7311-3.4.3-3), [`RFC7311-3.4.3-4`](#rfc7311-3.4.3-4), [`RFC7311-3.4.3-5`](#rfc7311-3.4.3-5), [`RFC7311-3.4.3-6`](#rfc7311-3.4.3-6), [`RFC7311-3.4.3-7`](#rfc7311-3.4.3-7), [`RFC7311-4.1-1`](#rfc7311-4.1-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC7311-3-1` | AIGP TLV type 1 MUST have length 11 (3 header + 8 metric) (§3) | MUST | 3 | **positive:** `unit/verify` [`TestParseAIGP`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/aigp_test.go#L20). **negative:** `unit/verify` [`TestParseAIGPMalformedMetricWrongLength`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/aigp_test.go#L101) |
| `RFC7311-3-2` | Total attribute length must be consistent with contained TLVs (§3) | MUST | 3 | **positive:** `unit/verify` [`TestParseAIGPMultipleTLVs`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/aigp_test.go#L52). **negative:** `unit/verify` [`TestParseAIGPMalformedTruncatedValue`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/aigp_test.go#L89) |
| `RFC7311-3-3` | Unknown TLV types MUST be preserved and not discarded (§3) | MUST | 3 | **positive:** `unit/verify` [`TestParseAIGPMultipleTLVs`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/aigp_test.go#L57). **negative:** `unit/verify` [`TestAIGPWriteToMultipleTLVs`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/aigp_test.go#L142) |
| `RFC7311-3.2-4` | A received path attribute carrying the AIGP codepoint with the transitive bit set is a malformed AIGP attribute and MUST be discarded (§3.2) | MUST | 3.2 | **positive:** `unit/verify` [`TestRFC7311AIGPNonTransitiveKeptOnReceive`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_aigp_receive_test.go#L103). **positive:** `unit/verify` [`TestRFC7606AIGPNonTransitiveIsKept`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_aigp_test.go#L104). **negative:** `unit/verify` [`TestRFC7311AIGPTransitiveDiscardedOnReceive`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_aigp_receive_test.go#L60). **negative:** `unit/verify` [`TestRFC7606AIGPTransitiveIsDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_aigp_test.go#L68) |
| `RFC7311-3.2-1` | A BGP speaker MUST NOT add the AIGP attribute to any route whose path leads outside the AIGP administrative domain to which the BGP speaker belongs (§3.4.1) | MUST NOT | 3.4.1 | **positive:** `unit/verify` [`TestAIGPConfiguredOriginationRejectsOutsideDomain`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L225). **negative:** `unit/verify` [`TestAIGPConfiguredOriginationRejectsOutsideDomain`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L226) |
| `RFC7311-3.2-5` | When receiving a BGP Update message containing a malformed AIGP attribute, the attribute MUST be treated exactly as if it were an unrecognized non-transitive attribute; that is, it "MUST be quietly ignored and not passed along to other BGP peers" (§3.2) | MUST | 3.2 | **positive:** `unit/verify` [`TestAIGPReceiveValidatesEveryTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L82). **negative:** `unit/verify` [`TestAIGPReceiveValidatesEveryTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L83) |
| `RFC7311-3.2-2` | If an AIGP attribute is received and its first AIGP TLV contains the maximum value 0xffffffffffffffff, the attribute SHOULD be considered to be malformed and SHOULD be discarded (§3.2) | SHOULD | 3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7311-3.2-6` | Note that an AIGP attribute MUST NOT be considered to be malformed because it contains more than one TLV of a given type or because it contains TLVs of unknown types. (§3.2) | MUST NOT | 3.2 | **positive:** `unit/verify` [`TestRFC7311DuplicateAndUnknownTLVsAreWellFormed`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/aigp_rfc7311_test.go#L50). **negative:** `unit/verify` [`TestRFC7311MalformedVerdictIsForLengthOnly`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/aigp_rfc7311_test.go#L80) |
| `RFC7311-3.3-1` | An implementation that supports the AIGP attribute MUST support a per-session configuration item, AIGP_SESSION, that indicates whether the attribute is enabled or disabled for use on that session (§3.3) | MUST | 3.3 | **positive:** `unit/verify` [`TestAIGPSessionReceiveBoundary`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L44). **negative:** `unit/verify` [`TestAIGPSessionReceiveBoundary`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L45) |
| `RFC7311-3.3-2` | - For all other External BGP (EBGP) sessions, the default value of AIGP_SESSION MUST be "disabled". (§3.3) | MUST | 3.3 | **positive:** `unit/verify` [`TestAIGPSessionReceiveBoundary`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L46). **negative:** `unit/verify` [`TestAIGPSessionReceiveBoundary`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L47) |
| `RFC7311-3.3-3` | The AIGP attribute MUST NOT be sent on any BGP session for which AIGP_SESSION is disabled (§3.3) | MUST NOT | 3.3 | **positive:** `unit/verify` [`TestAIGPFinalWriterBoundary`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L111). **negative:** `unit/verify` [`TestAIGPFinalWriterBoundary`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L112) |
| `RFC7311-3.3-4` | If an AIGP attribute is received on a BGP session for which AIGP_SESSION is disabled, the attribute MUST be treated exactly as if it were an unrecognized non-transitive attribute (§3.3) | MUST | 3.3 | **positive:** `unit/verify` [`TestAIGPSessionReceiveBoundary`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L48). **negative:** `unit/verify` [`TestAIGPSessionReceiveBoundary`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L49) |
| `RFC7311-3.4.1-1` | An implementation that supports the AIGP attribute MUST support a configuration item, AIGP_ORIGINATE, that enables or disables its creation and attachment to routes (§3.4.1) | MUST | 3.4.1 | **positive:** `unit/verify` [`TestAIGPOriginationControlsReachWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L134). **negative:** `unit/verify` [`TestAIGPOriginationControlsReachWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L135) |
| `RFC7311-3.4.1-2` | The default value of AIGP_ORIGINATE MUST be "disabled" (§3.4.1) | MUST | 3.4.1 | **positive:** `unit/verify` [`TestAIGPOriginationControlsReachWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L136). **negative:** `unit/verify` [`TestAIGPOriginationControlsReachWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L137) |
| `RFC7311-3.4.1-3` | A BGP speaker R MUST NOT add the AIGP attribute to any route for which R does not set itself as the next hop (§3.4.1) | MUST NOT | 3.4.1 | **positive:** `unit/verify` [`TestAIGPOriginationControlsReachWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L138). **negative:** `unit/verify` [`TestAIGPOriginationControlsReachWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L139) |
| `RFC7311-3.4.3-1` | If R1 does not change the next hop of the route, then R1 MUST NOT change the AIGP attribute value of the route (§3.4.3) | MUST NOT | 3.4.3 | **positive:** `unit/verify` [`TestAIGPForwardedMetricsReachFinalWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L164). **negative:** `unit/verify` [`TestAIGPForwardedMetricsReachFinalWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L165) |
| `RFC7311-3.4.3-2` | In all the computations of §3.4.3, the AIGP value MUST be capped at its maximum unsigned value 0xffffffffffffffff (§3.4.3) | MUST | 3.4.3 | **positive:** `unit/verify` [`TestAIGPForwardedMetricsReachFinalWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L166). **negative:** `unit/verify` [`TestAIGPForwardedMetricsReachFinalWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L167) |
| `RFC7311-3.4.3-3` | Increasing the AIGP value MUST NOT cause the value to wrap around (§3.4.3) | MUST NOT | 3.4.3 | **positive:** `unit/verify` [`TestAIGPForwardedMetricsReachFinalWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L168). **negative:** `unit/verify` [`TestAIGPForwardedMetricsReachFinalWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L169) |
| `RFC7311-3.4.3-4` | If R1's route to R2 is either (a) an IGP-learned route or (b) a static route that does not require recursive next hop resolution, then R1 MUST increase the value of the AIGP TLV by adding to A the distance from R1 to R2. (§3.4.3) | MUST | 3.4.3 | **positive:** `unit/verify` [`TestAIGPForwardedMetricsReachFinalWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L170). **positive:** `unit/verify` [`TestAIGPRecursiveDistanceUsesReceivedMetrics`](https://github.com/ze-software/ze/blob/main/internal/component/sysrib/rfc7311_metric_test.go#L12). **positive:** `unit/verify` [`TestAIGPRecursiveStaticDistance`](https://github.com/ze-software/ze/blob/main/internal/component/sysrib/rfc7311_metric_test.go#L61). **negative:** `unit/verify` [`TestAIGPForwardedMetricsReachFinalWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L171). **negative:** `unit/verify` [`TestAIGPRecursiveDistanceUsesReceivedMetrics`](https://github.com/ze-software/ze/blob/main/internal/component/sysrib/rfc7311_metric_test.go#L13). **negative:** `unit/verify` [`TestAIGPRecursiveStaticDistance`](https://github.com/ze-software/ze/blob/main/internal/component/sysrib/rfc7311_metric_test.go#L62) |
| `RFC7311-3.4.3-5` | A MUST be increased by a non-zero amount (§3.4.3) | MUST | 3.4.3 | **positive:** `unit/verify` [`TestAIGPForwardedMetricsReachFinalWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L172). **negative:** `unit/verify` [`TestAIGPForwardedMetricsReachFinalWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L173) |
| `RFC7311-3.4.3-6` | Then, when R1 changes the next hop of a route from R2 to R1, the AIGP TLV value MUST be increased by a non-zero amount. (§3.4.3) | MUST | 3.4.3 | **positive:** `unit/verify` [`TestAIGPForwardedMetricsReachFinalWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L174). **negative:** `unit/verify` [`TestAIGPForwardedMetricsReachFinalWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L175) |
| `RFC7311-3.4.3-7` | Any change due to (a) in any of these values MUST trigger a new AIGP computation for that route. (§3.4.3) | MUST | 3.4.3 | **positive:** `unit/verify` [`TestAIGPDistanceChangeReselectsRetainedRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc7311_selection_test.go#L82). **positive:** `unit/verify` [`TestAIGPReadvertisesFromReceivedGeneration`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/aigp_readvertise_test.go#L169). **positive:** `unit/verify` [`TestAIGPRecursiveDistanceUsesReceivedMetrics`](https://github.com/ze-software/ze/blob/main/internal/component/sysrib/rfc7311_metric_test.go#L14). **negative:** `unit/verify` [`TestAIGPDistanceChangeReselectsRetainedRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc7311_selection_test.go#L83). **negative:** `unit/verify` [`TestAIGPReadvertisesFromReceivedGeneration`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/aigp_readvertise_test.go#L170). **negative:** `unit/verify` [`TestAIGPRecursiveDistanceUsesReceivedMetrics`](https://github.com/ze-software/ze/blob/main/internal/component/sysrib/rfc7311_metric_test.go#L15) |
| `RFC7311-4.1-1` | Assuming that the BGP decision process invokes the tie-breaking procedures, the procedures of §4.1 MUST be executed BEFORE any of the tie-breaking procedures described in [BGP], Section 9.1.2.2 are executed (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestAIGPSelectsStoredRoutesBeforeASPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc7311_selection_test.go#L34). **negative:** `unit/verify` [`TestAIGPSelectsStoredRoutesBeforeASPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc7311_selection_test.go#L35) |
| `RFC7311-3.3-5` | For IBGP sessions, and for EBGP sessions between members of the same BGP Confederation, the default value of AIGP_SESSION SHOULD be "enabled" (§3.3) | SHOULD | 3.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7311-3.1-1` | "This document only considers the use of the AIGP attribute in networks where each router uses tunneling of some sort to deliver a packet to its BGP next hop.  Use of the AIGP attribute in other scenarios is outside the scope of this document" (§3.1) | MAY | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7311-3.4.1-4` | The AIGP attribute may be added only to a static route not leading outside the AIGP administrative domain that is redistributed into BGP, an IGP route redistributed into BGP, an IBGP-learned route whose AS_PATH is empty, or an EBGP-learned route whose AS_PATH contains only ASes in the same AIGP administrative domain (§3.4.1) | MAY | 3.4.1 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

RFC 7311 declares no gap, and every gated MUST it carries has a test bound to it.

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC7311-3-1`](#rfc7311-3-1)

AIGP TLV type 1 MUST have length 11 (3 header + 8 metric) (§3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestParseAIGPMalformedMetricWrongLength`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/aigp_test.go#L101) | unit/verify | unproven |
| positive | [`TestParseAIGP`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/aigp_test.go#L20) | unit/verify | unproven |

### [`RFC7311-3-2`](#rfc7311-3-2)

Total attribute length must be consistent with contained TLVs (§3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestParseAIGPMalformedTruncatedValue`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/aigp_test.go#L89) | unit/verify | unproven |
| positive | [`TestParseAIGPMultipleTLVs`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/aigp_test.go#L52) | unit/verify | unproven |

### [`RFC7311-3-3`](#rfc7311-3-3)

Unknown TLV types MUST be preserved and not discarded (§3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestAIGPWriteToMultipleTLVs`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/aigp_test.go#L142) | unit/verify | unproven |
| positive | [`TestParseAIGPMultipleTLVs`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/aigp_test.go#L57) | unit/verify | unproven |

### [`RFC7311-3.2-4`](#rfc7311-3.2-4)

A received path attribute carrying the AIGP codepoint with the transitive bit set is a malformed AIGP attribute and MUST be discarded (§3.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7606AIGPTransitiveIsDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_aigp_test.go#L68) | unit/verify | revert, verified |
| negative | [`TestRFC7311AIGPTransitiveDiscardedOnReceive`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_aigp_receive_test.go#L60) | unit/verify | revert, verified |
| positive | [`TestRFC7606AIGPNonTransitiveIsKept`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_aigp_test.go#L104) | unit/verify | revert, verified |
| positive | [`TestRFC7311AIGPNonTransitiveKeptOnReceive`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_aigp_receive_test.go#L103) | unit/verify | revert, verified |

### [`RFC7311-3.2-1`](#rfc7311-3.2-1)

A BGP speaker MUST NOT add the AIGP attribute to any route whose path leads outside the AIGP administrative domain to which the BGP speaker belongs (§3.4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestAIGPConfiguredOriginationRejectsOutsideDomain`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L226) | unit/verify | unproven |
| positive | [`TestAIGPConfiguredOriginationRejectsOutsideDomain`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L225) | unit/verify | unproven |

### [`RFC7311-3.2-5`](#rfc7311-3.2-5)

When receiving a BGP Update message containing a malformed AIGP attribute, the attribute MUST be treated exactly as if it were an unrecognized non-transitive attribute; that is, it "MUST be quietly ignored and not passed along to other BGP peers" (§3.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestAIGPReceiveValidatesEveryTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L83) | unit/verify | unproven |
| positive | [`TestAIGPReceiveValidatesEveryTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L82) | unit/verify | unproven |

### [`RFC7311-3.2-6`](#rfc7311-3.2-6)

Note that an AIGP attribute MUST NOT be considered to be malformed because it contains more than one TLV of a given type or because it contains TLVs of unknown types. (§3.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7311MalformedVerdictIsForLengthOnly`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/aigp_rfc7311_test.go#L80) | unit/verify | revert, verified |
| positive | [`TestRFC7311DuplicateAndUnknownTLVsAreWellFormed`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/aigp_rfc7311_test.go#L50) | unit/verify | revert, verified |

### [`RFC7311-3.3-1`](#rfc7311-3.3-1)

An implementation that supports the AIGP attribute MUST support a per-session configuration item, AIGP_SESSION, that indicates whether the attribute is enabled or disabled for use on that session (§3.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestAIGPSessionReceiveBoundary`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L45) | unit/verify | unproven |
| positive | [`TestAIGPSessionReceiveBoundary`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L44) | unit/verify | unproven |

### [`RFC7311-3.3-2`](#rfc7311-3.3-2)

- For all other External BGP (EBGP) sessions, the default value of AIGP_SESSION MUST be "disabled". (§3.3)

Audit verdict: not audited: no reader has judged these tests

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

In all the computations of §3.4.3, the AIGP value MUST be capped at its maximum unsigned value 0xffffffffffffffff (§3.4.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestAIGPForwardedMetricsReachFinalWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L167) | unit/verify | unproven |
| positive | [`TestAIGPForwardedMetricsReachFinalWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L166) | unit/verify | unproven |

### [`RFC7311-3.4.3-3`](#rfc7311-3.4.3-3)

Increasing the AIGP value MUST NOT cause the value to wrap around (§3.4.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestAIGPForwardedMetricsReachFinalWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L169) | unit/verify | unproven |
| positive | [`TestAIGPForwardedMetricsReachFinalWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L168) | unit/verify | unproven |

### [`RFC7311-3.4.3-4`](#rfc7311-3.4.3-4)

If R1's route to R2 is either (a) an IGP-learned route or (b) a static route that does not require recursive next hop resolution, then R1 MUST increase the value of the AIGP TLV by adding to A the distance from R1 to R2. (§3.4.3)

Audit verdict: not audited: no reader has judged these tests

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

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestAIGPForwardedMetricsReachFinalWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L175) | unit/verify | unproven |
| positive | [`TestAIGPForwardedMetricsReachFinalWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7311_processing_test.go#L174) | unit/verify | unproven |

### [`RFC7311-3.4.3-7`](#rfc7311-3.4.3-7)

Any change due to (a) in any of these values MUST trigger a new AIGP computation for that route. (§3.4.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestAIGPDistanceChangeReselectsRetainedRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc7311_selection_test.go#L83) | unit/verify | unproven |
| negative | [`TestAIGPReadvertisesFromReceivedGeneration`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/aigp_readvertise_test.go#L170) | unit/verify | unproven |
| negative | [`TestAIGPRecursiveDistanceUsesReceivedMetrics`](https://github.com/ze-software/ze/blob/main/internal/component/sysrib/rfc7311_metric_test.go#L15) | unit/verify | unproven |
| positive | [`TestAIGPDistanceChangeReselectsRetainedRoutes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc7311_selection_test.go#L82) | unit/verify | unproven |
| positive | [`TestAIGPReadvertisesFromReceivedGeneration`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/aigp_readvertise_test.go#L169) | unit/verify | unproven |
| positive | [`TestAIGPRecursiveDistanceUsesReceivedMetrics`](https://github.com/ze-software/ze/blob/main/internal/component/sysrib/rfc7311_metric_test.go#L14) | unit/verify | unproven |

### [`RFC7311-4.1-1`](#rfc7311-4.1-1)

Assuming that the BGP decision process invokes the tie-breaking procedures, the procedures of §4.1 MUST be executed BEFORE any of the tie-breaking procedures described in [BGP], Section 9.1.2.2 are executed (§4.1)

Audit verdict: not audited: no reader has judged these tests

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
