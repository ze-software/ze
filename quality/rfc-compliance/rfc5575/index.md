# RFC 5575 - Dissemination of Flow Specification Rules

Partial. Every requirement this repository extracted from RFC 5575, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 16.7% | 2 of 12 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 50.0% | 6 of 12 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 12 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 0.0% | 0 of 12 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 12 | of 16 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 12 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 33.3% | 4 of 12 binding obligations | no test carries the requirement id, whether or not a gap states why |

The 4 shares marked as a part above are the whole of the 12 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| No test at all | bad | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Partial |
| Enrolment | Enrolled |
| Requirements | 16 |
| Gated MUST-level | 12 |
| Obligations that bind Ze | 12 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 4 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 12 |
| Tagged units | 12 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc5575.md` |
| Requirement shard | `rfc/requirements/rfc5575.md` |
| RFC text | `rfc/full/rfc5575.txt` |

## Enrolment

Enrolled: BGP Flowspec (obsoleted by RFC 8955): 8 single-polarity positive (capability/family/component-ordering/reserved-bits) + 4 gap (Section 6 validation unimplemented, same root cause as RFC8956-5-1)

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

- IPv4 Flowspec NLRI encode/decode, MP (Code 1) capability negotiation for (AFI 1, SAFI 133/134), traffic-action extended communities, and lowering to the firewall
- component ordering and operator/fragment/traffic-marking reserved bits enforced on encode.


**What the ledger says remains**

Four Section 6 validation MUSTs unmet (same root cause as RFC8956-5-1): 6-1 no eBGP AS_PATH leftmost-neighbor enforcement; 6-2 no feasibility validation against the unicast RIB; 6-3 no flow-spec-vs-best-match originator comparison; 6-4 no more-specific/different-neighbor-AS check.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 2 | one part of the gated population |
| Annotated instead of tested | 10 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **12** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (2):** [`RFC5575-4-3`](#rfc5575-4-3), [`RFC5575-4-4`](#rfc5575-4-4)

**Annotated instead of tested (10):** [`RFC5575-4-1`](#rfc5575-4-1), [`RFC5575-4-2`](#rfc5575-4-2), [`RFC5575-6-1`](#rfc5575-6-1), [`RFC5575-6-2`](#rfc5575-6-2), [`RFC5575-6-3`](#rfc5575-6-3), [`RFC5575-6-4`](#rfc5575-6-4), [`RFC5575-4-5`](#rfc5575-4-5), [`RFC5575-4-6`](#rfc5575-4-6), [`RFC5575-4-7`](#rfc5575-4-7), [`RFC5575-7-1`](#rfc5575-7-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC5575-4-1` | Implementations wishing to exchange flow specification rules MUST use BGP Capability Advertisement to exchange the Multiprotocol Extension Capability Code (Code 1) (§4) | MUST | 4 | **positive:** `unit/verify` [`TestIPv4FlowSpecNegotiatesMultiprotocolCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/plugin_test.go#L845). **negative:** no negative test. **{single-polarity}:** the flowspec plugin unconditionally maps its declared ipv4/flow family to a Multiprotocol (Code 1) capability during OPEN, and there is no wrong input the negotiation path rejects (internal/component/bgp/plugins/nlri/flowspec/register.go:19-22, types.go:47) |
| `RFC5575-4-2` | The (AFI, SAFI) pair in the Multiprotocol Extension Capability MUST match the application using this NLRI-type (§4) | MUST | 4 | **positive:** `unit/verify` [`TestFlowSpecIPv4Basic`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L420). **positive:** `unit/verify` [`TestFlowSpecVPNFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L646). **negative:** no negative test. **{single-polarity}:** the (AFI 1, SAFI 133/134) assignment is a family-registration constant, not an input guard, so only the positive assignment is assertable (internal/component/bgp/plugins/nlri/flowspec/types.go:47-49, encode.go:79-95) |
| `RFC5575-4-3` | Flow specification component types MUST appear in strictly ascending numeric order (§4) | MUST | 4 | **positive:** `unit/verify` [`TestFlowSpecComponentsAscendingOrder`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1226). **positive:** `unit/verify` [`TestFlowSpecJoinsRepeatedTypeIntoOneComponent`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1269). **negative:** `unit/verify` [`TestParseFlowSpecRefusesRepeatedComponentType`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1306) |
| `RFC5575-4-4` | A present component MUST precede any component of higher numeric type value (§4) | MUST | 4 | **positive:** `unit/verify` [`TestFlowSpecComponentsAscendingOrder`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1227). **negative:** `unit/verify` [`TestParseFlowSpecRefusesDescendingComponentType`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1334) |
| `RFC5575-6-1` | BGP implementations MUST enforce that the AS_PATH attribute of a route received via eBGP contains the neighboring AS in the left-most position (§6) | MUST | 6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze runs no eBGP leftmost-neighbor-AS enforcement; the only AS_PATH ingress guard is RFC 4271 Section 9 loop detection and firstASInPath is used solely for MED neighbor comparison (internal/component/bgp/reactor/filter/loop_metrics.go:31, internal/component/bgp/plugins/rib/bestpath.go:544) |
| `RFC5575-6-2` | Flow specification MUST be validated against unicast routing (feasibility check) (§6) | MUST | 6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze decodes a received flowspec NLRI and lowers it to the firewall with no feasibility validation against the unicast RIB, the same absence disclosed for RFC8956-5-1 (internal/component/bgp/plugins/nlri/flowspec/types.go:351, internal/plugins/flowspec-firewall/translate.go:166) |
| `RFC5575-6-3` | Originator matching MUST be performed: the originator of the flow spec must match the originator of the best-match unicast route for the destination prefix (§6) | MUST | 6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no code compares a flowspec's originator against the best-match unicast route's originator; this is part of the absent Section 6 validation procedure (internal/component/bgp/plugins/nlri/flowspec/) |
| `RFC5575-6-4` | There must be no more-specific unicast routes from a different neighboring AS than the best-match route (§6) | MUST | 6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no code walks more-specific unicast routes to compare neighboring AS, because the Section 6 validation procedure is unimplemented (internal/component/bgp/plugins/nlri/flowspec/) |
| `RFC5575-4-5` | Reserved bits in numeric operator format (bit 4) must be 0 (§4, Numeric Operator) | MUST | 4 | **positive:** `unit/verify` [`TestFlowSpecNumericOperatorReservedBitZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1421). **negative:** no negative test. **{single-polarity}:** the numeric operator byte is built as lenCode<<4 OR'd with only the comparison bits, so reserved bit 4 is never set on encode and decode ignores reserved bits (internal/component/bgp/plugins/nlri/flowspec/types_numeric.go:247-257) |
| `RFC5575-4-6` | Reserved bits in bitmask operator format (bits 4-5) must be 0 (§4, Bitmask Operator) | MUST | 4 | **positive:** `unit/verify` [`TestFlowSpecBitmaskOperatorReservedBitsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1449). **negative:** no negative test. **{single-polarity}:** bitmask components encode through the same operator builder using only match/not plus the len field, so reserved bits 4-5 are never set on encode (internal/component/bgp/plugins/nlri/flowspec/types_numeric.go:247-257) |
| `RFC5575-4-7` | Reserved bits in Fragment bitmask (bits 0-3) must be zero (§4, Type 12) | MUST | 4 | **positive:** `unit/verify` [`TestFlowSpecFragmentReservedHighNibbleZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1478). **negative:** no negative test. **{single-polarity}:** the Fragment value is assembled from the four low-nibble flag constants, so the reserved high-nibble bits are never set on encode (internal/component/bgp/plugins/nlri/flowspec/types.go:201-206) |
| `RFC5575-7-1` | Reserved bytes in Traffic-Marking extended community (bytes 2-6) must be zero (§7) | MUST | 7 | **positive:** `unit/verify` [`TestFlowSpecTrafficMarkingReservedBytesZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1510). **negative:** no negative test. **{single-polarity}:** the traffic-marking community is emitted with literal zero reserved bytes and only the trailing DSCP octet varies (internal/component/bgp/plugins/nlri/flowspec/encode.go:124-127) |
| `RFC5575-3-1` | Standard BGP policy mechanisms (UPDATE filtering by NLRI prefix and community matching) SHOULD apply to flow specification NLRI (§3) | SHOULD | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC5575-9-1` | Implementations SHOULD provide a mechanism to log the packet header of filtered traffic (§9) | SHOULD | 9 | **positive:** no positive test. **negative:** no negative test |
| `RFC5575-9-2` | Implementations SHOULD provide a mechanism to count the number of matches for a given flow specification rule (§9) | SHOULD | 9 | **positive:** no positive test. **negative:** no negative test |
| `RFC5575-7-2` | User-defined community mappings MAY be used for platform-specific behaviors (§7) | MAY | 7 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC5575-6-1`](#rfc5575-6-1) BGP implementations MUST enforce that the AS_PATH attribute of a route received via eBGP contains the neighboring AS in the left-most position (§6) | {gap}, no test | ze runs no eBGP leftmost-neighbor-AS enforcement; the only AS_PATH ingress guard is RFC 4271 Section 9 loop detection and firstASInPath is used solely for MED neighbor comparison (internal/component/bgp/reactor/filter/loop_metrics.go:31, internal/component/bgp/plugins/rib/bestpath.go:544) |
| [`RFC5575-6-2`](#rfc5575-6-2) Flow specification MUST be validated against unicast routing (feasibility check) (§6) | {gap}, no test | ze decodes a received flowspec NLRI and lowers it to the firewall with no feasibility validation against the unicast RIB, the same absence disclosed for RFC8956-5-1 (internal/component/bgp/plugins/nlri/flowspec/types.go:351, internal/plugins/flowspec-firewall/translate.go:166) |
| [`RFC5575-6-3`](#rfc5575-6-3) Originator matching MUST be performed: the originator of the flow spec must match the originator of the best-match unicast route for the destination prefix (§6) | {gap}, no test | no code compares a flowspec's originator against the best-match unicast route's originator; this is part of the absent Section 6 validation procedure (internal/component/bgp/plugins/nlri/flowspec/) |
| [`RFC5575-6-4`](#rfc5575-6-4) There must be no more-specific unicast routes from a different neighboring AS than the best-match route (§6) | {gap}, no test | no code walks more-specific unicast routes to compare neighboring AS, because the Section 6 validation procedure is unimplemented (internal/component/bgp/plugins/nlri/flowspec/) |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC5575-4-1`](#rfc5575-4-1)

Implementations wishing to exchange flow specification rules MUST use BGP Capability Advertisement to exchange the Multiprotocol Extension Capability Code (Code 1) (§4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestIPv4FlowSpecNegotiatesMultiprotocolCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/plugin_test.go#L845) | unit/verify | unproven |

### [`RFC5575-4-2`](#rfc5575-4-2)

The (AFI, SAFI) pair in the Multiprotocol Extension Capability MUST match the application using this NLRI-type (§4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestFlowSpecIPv4Basic`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L420) | unit/verify | unproven |
| positive | [`TestFlowSpecVPNFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L646) | unit/verify | unproven |

### [`RFC5575-4-3`](#rfc5575-4-3)

Flow specification component types MUST appear in strictly ascending numeric order (§4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestParseFlowSpecRefusesRepeatedComponentType`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1306) | unit/verify | unproven |
| positive | [`TestFlowSpecComponentsAscendingOrder`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1226) | unit/verify | unproven |
| positive | [`TestFlowSpecJoinsRepeatedTypeIntoOneComponent`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1269) | unit/verify | unproven |

### [`RFC5575-4-4`](#rfc5575-4-4)

A present component MUST precede any component of higher numeric type value (§4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestParseFlowSpecRefusesDescendingComponentType`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1334) | unit/verify | unproven |
| positive | [`TestFlowSpecComponentsAscendingOrder`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1227) | unit/verify | unproven |

### [`RFC5575-6-1`](#rfc5575-6-1)

BGP implementations MUST enforce that the AS_PATH attribute of a route received via eBGP contains the neighboring AS in the left-most position (§6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5575-6-1, so no unit is bound to it.

### [`RFC5575-6-2`](#rfc5575-6-2)

Flow specification MUST be validated against unicast routing (feasibility check) (§6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5575-6-2, so no unit is bound to it.

### [`RFC5575-6-3`](#rfc5575-6-3)

Originator matching MUST be performed: the originator of the flow spec must match the originator of the best-match unicast route for the destination prefix (§6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5575-6-3, so no unit is bound to it.

### [`RFC5575-6-4`](#rfc5575-6-4)

There must be no more-specific unicast routes from a different neighboring AS than the best-match route (§6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5575-6-4, so no unit is bound to it.

### [`RFC5575-4-5`](#rfc5575-4-5)

Reserved bits in numeric operator format (bit 4) must be 0 (§4, Numeric Operator)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestFlowSpecNumericOperatorReservedBitZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1421) | unit/verify | unproven |

### [`RFC5575-4-6`](#rfc5575-4-6)

Reserved bits in bitmask operator format (bits 4-5) must be 0 (§4, Bitmask Operator)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestFlowSpecBitmaskOperatorReservedBitsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1449) | unit/verify | unproven |

### [`RFC5575-4-7`](#rfc5575-4-7)

Reserved bits in Fragment bitmask (bits 0-3) must be zero (§4, Type 12)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestFlowSpecFragmentReservedHighNibbleZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1478) | unit/verify | unproven |

### [`RFC5575-7-1`](#rfc5575-7-1)

Reserved bytes in Traffic-Marking extended community (bytes 2-6) must be zero (§7)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestFlowSpecTrafficMarkingReservedBytesZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1510) | unit/verify | unproven |

## Extraction sign-off

No extraction sign-off exists for RFC 5575, so no reviewer has walked its text sentence by sentence.

## Superseded

RFC 5575 is obsoleted by RFC 8955.

| Requirement | Disposition | Now stated at | Reason |
|---|---|---|---|
| [`RFC5575-4-1`](#rfc5575-4-1) Implementations wishing to exchange flow specification rules MUST use BGP Capability Advertisement to exchange the Multiprotocol Extension Capability Code (Code 1) (§4) | restated | RFC8955-4-1 | RFC 8955 Section 4 keeps the sentence, that implementations wishing to exchange Flow Specification MUST use BGP's Capability Advertisement facility to exchange the Multiprotocol Extension Capability Code (Code 1) |
| [`RFC5575-4-2`](#rfc5575-4-2) The (AFI, SAFI) pair in the Multiprotocol Extension Capability MUST match the application using this NLRI-type (§4) | restated | RFC8955-4-2 | RFC 8955 Section 4 makes the pair explicit rather than leaving it to the application: (AFI 1, SAFI 133) for IPv4 Flow Specification and (AFI 1, SAFI 134) for VPNv4 Flow Specification |
| [`RFC5575-4-3`](#rfc5575-4-3) Flow specification component types MUST appear in strictly ascending numeric order (§4) | restated | RFC8955-4.2-1 | the NLRI value encoding moved from Section 4 to Section 4.2, which keeps the strict type ordering by increasing numerical order |
| [`RFC5575-4-4`](#rfc5575-4-4) A present component MUST precede any component of higher numeric type value (§4) | restated | RFC8955-4.2-2 | RFC 8955 Section 4.2 keeps the rule that a component, if present, MUST precede any component of higher numeric type value |
| [`RFC5575-6-1`](#rfc5575-6-1) BGP implementations MUST enforce that the AS_PATH attribute of a route received via eBGP contains the neighboring AS in the left-most position (§6) | restated | RFC8955-6-2 | RFC 8955 Section 6 keeps the eBGP leftmost-AS enforcement in the same words, and keeps the reason, that the rule is optional in the BGP specification and necessary here for security |
| [`RFC5575-6-2`](#rfc5575-6-2) Flow specification MUST be validated against unicast routing (feasibility check) (§6) | restated | RFC8955-6-1 | RFC 8955 Section 6 keeps the feasibility rule and scopes it, adding that it applies in the absence of explicit configuration and that SAFI 133 validates against SAFI 1 while SAFI 134 validates against SAFI 128 |
| [`RFC5575-6-3`](#rfc5575-6-3) Originator matching MUST be performed: the originator of the flow spec must match the originator of the best-match unicast route for the destination prefix (§6) | restated | RFC8955-6-1 | originator matching is condition (b) of the RFC 8955 Section 6 list, in the same words, and RFC 8955 states the three conditions as one feasibility rule rather than three. Condition (b) is moot when condition (a) is relaxed by explicit configuration |
| [`RFC5575-6-4`](#rfc5575-6-4) There must be no more-specific unicast routes from a different neighboring AS than the best-match route (§6) | restated | RFC8955-6-1 | the more-specific-route rule is condition (c) of the RFC 8955 Section 6 list, in the same words, and RFC 8955 states the three conditions as one feasibility rule rather than three. Condition (c) is moot when condition (a) is relaxed by explicit configuration |
| [`RFC5575-4-5`](#rfc5575-4-5) Reserved bits in numeric operator format (bit 4) must be 0 (§4, Numeric Operator) | restated | RFC8955-4.2.1.1-3 | RFC 8955 Section 4.2.1.1 keeps the numeric operator reserved bit at 0 on encoding and adds the receive half, that it MUST be ignored during decoding |
| [`RFC5575-4-6`](#rfc5575-4-6) Reserved bits in bitmask operator format (bits 4-5) must be 0 (§4, Bitmask Operator) | restated | RFC8955-4.2.1.2-1 | RFC 8955 Section 4.2.1.2 keeps the bitmask operator reserved bits at 0 on encoding and adds the receive half, that they MUST be ignored during decoding |
| [`RFC5575-4-7`](#rfc5575-4-7) Reserved bits in Fragment bitmask (bits 0-3) must be zero (§4, Type 12) | restated | RFC8955-4.2.2.12-2 | RFC 8955 Section 4.2.2.12 keeps the Fragment bitmask reserved bits at 0 on encoding and adds the receive half, that they MUST be ignored during decoding |
| [`RFC5575-7-1`](#rfc5575-7-1) Reserved bytes in Traffic-Marking extended community (bytes 2-6) must be zero (§7) | restated | RFC8955-7.5-1 | the traffic-marking action moved from Section 7 to Section 7.5, which keeps the reserved bits at 0 on encoding and adds the receive half, that they MUST be ignored during decoding |
| [`RFC5575-3-1`](#rfc5575-3-1) Standard BGP policy mechanisms (UPDATE filtering by NLRI prefix and community matching) SHOULD apply to flow specification NLRI (§3) | unextracted | §3 | RFC 8955 Section 3 states the obligation with a lowercase keyword, that standard BGP policy mechanisms such as UPDATE filtering by NLRI prefix and community matching must apply to the Flow specification defined NLRI-type. rfc/short/rfc8955.md declares no row for it |
| [`RFC5575-9-1`](#rfc5575-9-1) Implementations SHOULD provide a mechanism to log the packet header of filtered traffic (§9) | restated | RFC8955-9-1 | RFC 8955 Section 9 keeps the SHOULD to provide a mechanism to log the packet header of filtered traffic |
| [`RFC5575-9-2`](#rfc5575-9-2) Implementations SHOULD provide a mechanism to count the number of matches for a given flow specification rule (§9) | restated | RFC8955-9-2 | RFC 8955 Section 9 keeps the SHOULD to provide a mechanism to count the number of matches for a given Flow Specification rule |
| [`RFC5575-7-2`](#rfc5575-7-2) User-defined community mappings MAY be used for platform-specific behaviors (§7) | unextracted | §7.6 | RFC 8955 gives the paragraph its own section, 7.6, and keeps it word for word, that a user-defined community value can be mapped to platform-specific or network-specific behavior via user configuration. rfc/short/rfc8955.md declares no row for it |
