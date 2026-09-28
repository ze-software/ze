# RFC 5575 - Dissemination of Flow Specification Rules

Partial. Every requirement this repository extracted from RFC 5575, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 62.5% | 5 of 8 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 37.5% | 3 of 8 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 8 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 0.0% | 0 of 8 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 0.0% | 0 of 15 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 8 | of 16 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 8 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 8 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 8 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 8 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Audit verdicts | 8 | of 8 gated MUSTs judged | 4 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

The 7 shares marked as a part above are the whole of the 8 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Audit verdicts | bad | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Partial |
| Enrolment | Enrolled |
| Requirements | 16 |
| Gated MUST-level | 8 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 15 |
| Tagged units | 15 |
| Recorded audit verdicts | 8 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc5575.md` |
| Requirement shard | `rfc/requirements/rfc5575.md` |
| RFC text | `rfc/full/rfc5575.txt` |

## Enrolment

Enrolled: BGP Flowspec (obsoleted by RFC 8955): 8 single-polarity positive (capability/family/component-ordering/reserved-bits) + 4 rollup onto the RFC 8955 Section 6 validation rows that restate them

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

- IPv4 Flowspec NLRI encode/decode, MP (Code 1) capability negotiation for (AFI 1, SAFI 133/134), traffic-action extended communities, and lowering to the firewall
- component ordering and operator/fragment/traffic-marking reserved bits enforced on encode.


**What the ledger says remains**

RFC 8955 and RFC 9117 supply the current validation rules. The native codec, retained-route feasibility and revalidation, first-AS comparison against covering unicast reachability, originator/local-controller authorization, and more-specific-AS guard have source changes and regression carriers awaiting integration validation. The global firewall installs selected SAFI 133 rules only; VPN FlowSpec remains a BGP propagation feature.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 5 | one part of the gated population |
| Annotated instead of tested | 3 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 4 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **8** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (5):** [`RFC5575-4-3`](#rfc5575-4-3), [`RFC5575-4-4`](#rfc5575-4-4), [`RFC5575-4-8`](#rfc5575-4-8), [`RFC5575-5.1-1`](#rfc5575-5.1-1), [`RFC5575-8-1`](#rfc5575-8-1)

**Annotated instead of tested (3):** [`RFC5575-4-1`](#rfc5575-4-1), [`RFC5575-4-2`](#rfc5575-4-2), [`RFC5575-7-1`](#rfc5575-7-1)

**Derived from other rows (4):** [`RFC5575-6-1`](#rfc5575-6-1), [`RFC5575-6-2`](#rfc5575-6-2), [`RFC5575-6-3`](#rfc5575-6-3), [`RFC5575-6-4`](#rfc5575-6-4)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC5575-4-1` | Implementations wishing to exchange flow specification rules MUST use BGP's Capability Advertisement facility to exchange the Multiprotocol Extension Capability Code (Code 1) as defined in RFC 4760 [RFC4760]. (§4) | MUST | 4 | **positive:** `unit/verify` [`TestIPv4FlowSpecNegotiatesMultiprotocolCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/plugin_test.go#L845). **negative:** no negative test. **{single-polarity}:** the flowspec plugin unconditionally maps its declared ipv4/flow family to a Multiprotocol (Code 1) capability during OPEN, and there is no wrong input the negotiation path rejects (internal/component/bgp/plugins/nlri/flowspec/register.go:19-22, types.go:47) |
| `RFC5575-4-2` | The (AFI, SAFI) pair carried in the Multiprotocol Extension Capability MUST be the same as the one used to identify a particular application that uses this NLRI-type. (§4) | MUST | 4 | **positive:** `unit/verify` [`TestFlowSpecIPv4Basic`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L420). **positive:** `unit/verify` [`TestFlowSpecVPNFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L646). **negative:** no negative test. **{single-polarity}:** the (AFI 1, SAFI 133/134) assignment is a family-registration constant, not an input guard, so only the positive assignment is assertable (internal/component/bgp/plugins/nlri/flowspec/types.go:47-49, encode.go:79-95) |
| `RFC5575-4-3` | Flow specification components must follow strict type ordering. (§4) | MUST | 4 | **positive:** `unit/verify` [`TestFlowSpecComponentsAscendingOrder`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1226). **positive:** `unit/verify` [`TestFlowSpecJoinsRepeatedTypeIntoOneComponent`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1269). **negative:** `unit/verify` [`TestParseFlowSpecRefusesRepeatedComponentType`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1306) |
| `RFC5575-4-4` | A given component type may or may not be present in the specification, but if present, it MUST precede any component of higher numeric type value. (§4) | MUST | 4 | **positive:** `unit/verify` [`TestFlowSpecComponentsAscendingOrder`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1227). **negative:** `unit/verify` [`TestParseFlowSpecRefusesDescendingComponentType`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1334) |
| `RFC5575-6-1` | BGP implementations MUST also enforce that the AS_PATH attribute of a route received via the External Border Gateway Protocol (eBGP) contains the neighboring AS in the left-most position of the AS_PATH attribute. (§6) | MUST | 6 | **positive:** no positive test. **negative:** no negative test. **{rollup}:** RFC8955-6-2; RFC 5575 is obsoleted and this sentence is restated by RFC 8955 Section 6, later replaced by RFC 9117 Section 4.2, so the row holds exactly when RFC8955-6-2 does. **derived:** met |
| `RFC5575-6-2` | A flow specification NLRI must be validated such that it is considered feasible if and only if: a) The originator of the flow specification matches the originator of the best-match unicast route for the destination prefix embedded in the flow specification. b) There are no more specific unicast routes, when compared with the flow destination prefix, that have been received from a different neighboring AS than the best-match unicast route, which has been determined in step a). (§6) | MUST | 6 | **positive:** no positive test. **negative:** no negative test. **{rollup}:** RFC8955-6-1; RFC 5575 is obsoleted and RFC 8955 Section 6 restates the feasibility check as its validation procedure, so the row holds exactly when RFC8955-6-1 does. **derived:** met |
| `RFC5575-6-3` | A flow specification NLRI must be validated such that it is considered feasible if and only if: a) The originator of the flow specification matches the originator of the best-match unicast route for the destination prefix embedded in the flow specification. (§6) | MUST | 6 | **positive:** no positive test. **negative:** no negative test. **{rollup}:** RFC8955-6-1; RFC 5575 is obsoleted and originator matching is condition (b) of the RFC 8955 Section 6 procedure, so the row holds exactly when RFC8955-6-1 does. **derived:** met |
| `RFC5575-6-4` | A flow specification NLRI must be validated such that it is considered feasible if and only if: a) The originator of the flow specification matches the originator of the best-match unicast route for the destination prefix embedded in the flow specification. b) There are no more specific unicast routes, when compared with the flow destination prefix, that have been received from a different neighboring AS than the best-match unicast route, which has been determined in step a). (§6) | MUST | 6 | **positive:** no positive test. **negative:** no negative test. **{rollup}:** RFC8955-6-1; RFC 5575 is obsoleted and the more-specific-route rule is condition (c) of the RFC 8955 Section 6 procedure, so the row holds exactly when RFC8955-6-1 does. **derived:** met |
| `RFC5575-7-1` | This extended community is encoded as a sequence of 5 zero bytes followed by the DSCP value encoded in the 6 least significant bits of 6th byte. (§7) | MUST | 7 | **positive:** `unit/verify` [`TestFlowSpecTrafficMarkingReservedBytesZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1507). **negative:** no negative test. **{single-polarity}:** the traffic-marking community is emitted with literal zero reserved bytes and only the trailing DSCP octet varies (internal/component/bgp/plugins/nlri/flowspec/encode.go:124-127) |
| `RFC5575-4-8` | Whenever the corresponding application does not require Next-Hop information, this shall be encoded as a 0-octet length Next Hop in the MP_REACH_NLRI attribute and ignored on receipt. (§4) | MUST | 4 | **positive:** `unit/verify` [`TestFlowSpecUpdateIgnoresConfiguredNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/flowspec_wire_contract_test.go#L16). **negative:** `unit/verify` [`TestFlowSpecUpdateIgnoresConfiguredNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/flowspec_wire_contract_test.go#L17) |
| `RFC5575-5.1-1` | This ordering function must be such that it must not depend on the arrival order of the flow specification's rules and must be constant in the network. (§5.1) | MUST | 5.1 | **positive:** `unit/verify` [`TestFlowSpecPrecedenceFromWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/compare_rfc8955_test.go#L13). **negative:** `unit/verify` [`TestFlowSpecPrecedenceFromWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/compare_rfc8955_test.go#L14) |
| `RFC5575-8-1` | The NLRI length field shall include both the 8 bytes of the Route Distinguisher as well as the subsequent flow specification. (§8) | MUST | 8 | **positive:** `unit/verify` [`TestFlowSpecVPNUpdateCountsRouteDistinguisher`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/flowspec_wire_contract_test.go#L35). **negative:** `unit/verify` [`TestFlowSpecVPNUpdateCountsRouteDistinguisher`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/flowspec_wire_contract_test.go#L36) |
| `RFC5575-3-1` | Standard BGP policy mechanisms, such as UPDATE filtering by NLRI prefix and community matching, SHOULD apply to the newly defined NLRI-type. (§3) | SHOULD | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC5575-9-1` | While this is an implementation-specific choice, implementations SHOULD provide: o A mechanism to log the packet header of filtered traffic. (§9) | SHOULD | 9 | **positive:** no positive test. **negative:** no negative test |
| `RFC5575-9-2` | While this is an implementation-specific choice, implementations SHOULD provide: o A mechanism to log the packet header of filtered traffic. o A mechanism to count the number of matches for a given flow specification rule. (§9) | SHOULD | 9 | **positive:** no positive test. **negative:** no negative test |
| `RFC5575-7-2` | Rather than attempting to define it here, this can be accomplished by mapping a user-defined community value to platform-/network-specific behavior via user configuration. (§7) | MAY | 7 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC5575-6-1`](#rfc5575-6-1) BGP implementations MUST also enforce that the AS_PATH attribute of a route received via the External Border Gateway Protocol (eBGP) contains the neighboring AS in the left-most position of the AS_PATH attribute. (§6) | no test | no test carries this requirement id; annotated {rollup}: RFC8955-6-2; RFC 5575 is obsoleted and this sentence is restated by RFC 8955 Section 6, later replaced by RFC 9117 Section 4.2, so the row holds exactly when RFC8955-6-2 does |
| [`RFC5575-6-2`](#rfc5575-6-2) A flow specification NLRI must be validated such that it is considered feasible if and only if: a) The originator of the flow specification matches the originator of the best-match unicast route for the destination prefix embedded in the flow specification. b) There are no more specific unicast routes, when compared with the flow destination prefix, that have been received from a different neighboring AS than the best-match unicast route, which has been determined in step a). (§6) | no test | no test carries this requirement id; annotated {rollup}: RFC8955-6-1; RFC 5575 is obsoleted and RFC 8955 Section 6 restates the feasibility check as its validation procedure, so the row holds exactly when RFC8955-6-1 does |
| [`RFC5575-6-3`](#rfc5575-6-3) A flow specification NLRI must be validated such that it is considered feasible if and only if: a) The originator of the flow specification matches the originator of the best-match unicast route for the destination prefix embedded in the flow specification. (§6) | no test | no test carries this requirement id; annotated {rollup}: RFC8955-6-1; RFC 5575 is obsoleted and originator matching is condition (b) of the RFC 8955 Section 6 procedure, so the row holds exactly when RFC8955-6-1 does |
| [`RFC5575-6-4`](#rfc5575-6-4) A flow specification NLRI must be validated such that it is considered feasible if and only if: a) The originator of the flow specification matches the originator of the best-match unicast route for the destination prefix embedded in the flow specification. b) There are no more specific unicast routes, when compared with the flow destination prefix, that have been received from a different neighboring AS than the best-match unicast route, which has been determined in step a). (§6) | no test | no test carries this requirement id; annotated {rollup}: RFC8955-6-1; RFC 5575 is obsoleted and the more-specific-route rule is condition (c) of the RFC 8955 Section 6 procedure, so the row holds exactly when RFC8955-6-1 does |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC5575-4-1`](#rfc5575-4-1)

Implementations wishing to exchange flow specification rules MUST use BGP's Capability Advertisement facility to exchange the Multiprotocol Extension Capability Code (Code 1) as defined in RFC 4760 [RFC4760]. (§4)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Forbidden: exchanging flow specification rules without the Multiprotocol (Code 1) capability in OPEN. TestIPv4FlowSpecNegotiatesMultiprotocolCapability only checks the plugin registers ipv4/flow and then builds a capability.Multiprotocol in the test itself, so mp.Code()==1 is tautological; no assertion reads a real OPEN, and nothing goes red if sendOpen omits the capability or flow routes go to a peer that did not negotiate it.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestIPv4FlowSpecNegotiatesMultiprotocolCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/plugin_test.go#L845) | unit/verify | unproven |

### [`RFC5575-4-2`](#rfc5575-4-2)

The (AFI, SAFI) pair carried in the Multiprotocol Extension Capability MUST be the same as the one used to identify a particular application that uses this NLRI-type. (§4)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Forbidden: an MP capability (AFI, SAFI) pair that differs from the pair the application's NLRI is sent under. TestFlowSpecIPv4Basic and TestFlowSpecVPNFamily assert family constants (1,133) and (1,134) only; no assertion compares the capability pair with the MP_REACH AFI/SAFI the encoder emits (encode.go chooses afi separately), so an encoder writing another SAFI stays green.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestFlowSpecIPv4Basic`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L420) | unit/verify | unproven |
| positive | [`TestFlowSpecVPNFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L646) | unit/verify | unproven |

### [`RFC5575-4-3`](#rfc5575-4-3)

Flow specification components must follow strict type ordering. (§4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: components out of strict type order (descending or repeated). TestFlowSpecComponentsAscendingOrder (positive) adds out of order and asserts wire order 1,3,5,12, red on insertion-order emission; TestFlowSpecJoinsRepeatedTypeIntoOneComponent asserts one Type 4 component; TestParseFlowSpecRefusesRepeatedComponentType require.ErrorIs ErrFlowSpecDuplicateType goes red if a repeat is accepted.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestParseFlowSpecRefusesRepeatedComponentType`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1306) | unit/verify | unproven |
| positive | [`TestFlowSpecComponentsAscendingOrder`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1226) | unit/verify | unproven |
| positive | [`TestFlowSpecJoinsRepeatedTypeIntoOneComponent`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1269) | unit/verify | unproven |

### [`RFC5575-4-4`](#rfc5575-4-4)

A given component type may or may not be present in the specification, but if present, it MUST precede any component of higher numeric type value. (§4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a present component after a component of higher type. Positive asserts strictly ascending emitted types (assert.Less) red on any inversion; TestParseFlowSpecRefusesDescendingComponentType require.ErrorIs ErrFlowSpecTypeOrder goes red if 5-before-3 is accepted, with the ascending control accepted.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestParseFlowSpecRefusesDescendingComponentType`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1334) | unit/verify | unproven |
| positive | [`TestFlowSpecComponentsAscendingOrder`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1227) | unit/verify | unproven |

### [`RFC5575-6-1`](#rfc5575-6-1)

BGP implementations MUST also enforce that the AS_PATH attribute of a route received via the External Border Gateway Protocol (eBGP) contains the neighboring AS in the left-most position of the AS_PATH attribute. (§6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5575-6-1, so no unit is bound to it.

### [`RFC5575-6-2`](#rfc5575-6-2)

A flow specification NLRI must be validated such that it is considered feasible if and only if: a) The originator of the flow specification matches the originator of the best-match unicast route for the destination prefix embedded in the flow specification. b) There are no more specific unicast routes, when compared with the flow destination prefix, that have been received from a different neighboring AS than the best-match unicast route, which has been determined in step a). (§6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5575-6-2, so no unit is bound to it.

### [`RFC5575-6-3`](#rfc5575-6-3)

A flow specification NLRI must be validated such that it is considered feasible if and only if: a) The originator of the flow specification matches the originator of the best-match unicast route for the destination prefix embedded in the flow specification. (§6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5575-6-3, so no unit is bound to it.

### [`RFC5575-6-4`](#rfc5575-6-4)

A flow specification NLRI must be validated such that it is considered feasible if and only if: a) The originator of the flow specification matches the originator of the best-match unicast route for the destination prefix embedded in the flow specification. b) There are no more specific unicast routes, when compared with the flow destination prefix, that have been received from a different neighboring AS than the best-match unicast route, which has been determined in step a). (§6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5575-6-4, so no unit is bound to it.

### [`RFC5575-7-1`](#rfc5575-7-1)

This extended community is encoded as a sequence of 5 zero bytes followed by the DSCP value encoded in the 6 least significant bits of 6th byte. (§7)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: non-zero bytes in the five octets before the DSCP, or the DSCP elsewhere. TestFlowSpecTrafficMarkingReservedBytesZero asserts ec[2..6]==0 and ec[7]==dscp with type 0x80/0x09; red on either. {single-polarity} marker present.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestFlowSpecTrafficMarkingReservedBytesZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1507) | unit/verify | unproven |

### [`RFC5575-4-8`](#rfc5575-4-8)

Whenever the corresponding application does not require Next-Hop information, this shall be encoded as a 0-octet length Next Hop in the MP_REACH_NLRI attribute and ignored on receipt. (§4)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Two clauses. Encode: TestFlowSpecUpdateIgnoresConfiguredNextHop asserts reach[3]==0 and no valid next hop even with a configured IPv4/IPv6 address, red on a non-empty next hop. Receipt: no tagged unit feeds a received FlowSpec MP_REACH with a non-zero next hop and asserts it is ignored, so a receiver that uses or refuses it stays green.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFlowSpecUpdateIgnoresConfiguredNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/flowspec_wire_contract_test.go#L17) | unit/verify | unproven |
| positive | [`TestFlowSpecUpdateIgnoresConfiguredNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/flowspec_wire_contract_test.go#L16) | unit/verify | unproven |

### [`RFC5575-5.1-1`](#rfc5575-5.1-1)

This ordering function must be such that it must not depend on the arrival order of the flow specification's rules and must be constant in the network. (§5.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Forbidden: an ordering that depends on arrival order. TestFlowSpecPrecedenceFromWire always parses the expected-first rule first, so an arrival-order tie-break still yields Compare(a,b)<0 and Compare(b,a)>0; the negative tag says reversing arrival order cannot reverse precedence, but no case parses the pair in the reverse order or feeds the ordering from two arrival sequences.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFlowSpecPrecedenceFromWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/compare_rfc8955_test.go#L14) | unit/verify | unproven |
| positive | [`TestFlowSpecPrecedenceFromWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/compare_rfc8955_test.go#L13) | unit/verify | unproven |

### [`RFC5575-8-1`](#rfc5575-8-1)

The NLRI length field shall include both the 8 bytes of the Route Distinguisher as well as the subsequent flow specification. (§8)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a VPN NLRI length that omits the RD octets. TestFlowSpecVPNUpdateCountsRouteDistinguisher asserts the emitted length octet 13 (8 RD + 5), red on 5; the negative subtracts 8 and require.Error goes red if the parser accepts it.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFlowSpecVPNUpdateCountsRouteDistinguisher`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/flowspec_wire_contract_test.go#L36) | unit/verify | unproven |
| positive | [`TestFlowSpecVPNUpdateCountsRouteDistinguisher`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/flowspec_wire_contract_test.go#L35) | unit/verify | unproven |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | prose |
| Source | rfc/full/rfc5575.txt |
| Source fingerprint | f57a08b1d5469245 |
| Record | rfc/extraction/rfc5575.json |
| Mapped sentences | 9 |
| Declined as scope | 8 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 1 | walked | not stated |
| `1` | not stated | 3 | walked | not stated |
| `2` | not stated | 0 | walked | not stated |
| `3` | not stated | 0 | walked | not stated |
| `4` | not stated | 5 | walked | not stated |
| `5` | not stated | 0 | walked | not stated |
| `5.1` | not stated | 1 | walked | not stated |
| `6` | not stated | 3 | walked | not stated |
| `7` | not stated | 0 | walked | not stated |
| `8` | not stated | 1 | walked | not stated |
| `9` | not stated | 0 | walked | not stated |
| `10` | not stated | 0 | walked | not stated |
| `11` | not stated | 3 | walked | not stated |
| `12` | not stated | 0 | walked | not stated |
| `13` | not stated | 0 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `front:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Abstract prose describing what the document defines; the lowercase 'required' is part of the phrase 'such as what is required in order to mitigate (distributed) denial-of-service attacks', not an obligation on an implementation. | Additionally, it defines two applications of that encoding format: one that can be used to automate inter-domain coordination of traffic filtering, such as what is required in order to mitigate (distributed) denial-of-service attacks, and a second application to provide traffic filtering in the context of a BGP/MPLS VPN service. |
| `1:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Introduction prose stating the document's own scope ('we define the required mechanisms'); no obligation on an implementation. | Additionally, we define the required mechanisms to utilize this definition to the problem of immediate concern to the authors: intra- and inter-provider distribution of traffic filtering rules to filter (distributed) denial-of-service (DoS) attacks. |
| `1:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Introduction sentence heading a descriptive list of technology components; 'required to address the class of problems' describes the problem space, not an implementation obligation. | The key technology components required to address the class of problems targeted by this document are: |
| `1:3` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Introduction prose stating the document's own scope ('defines required protocol extensions'); the obligations themselves are in Sections 4, 6 and 7. | This specification defines required protocol extensions to address most common applications of IPv4 unicast and VPNv4 unicast filtering. |
| `6:1` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | The obligation to run path selection is RFC 4271's, which the same paragraph cites: 'The first step of the BGP Route Selection procedure (Section 9.1.2 of [RFC4271])'. This sentence only states that flow specification routes are not exempt from it. | Although the forwarding attributes of two routes for the same flow specification prefix may be the same, BGP is still required to perform its path selection algorithm in order to select the correct set of attributes to advertise. |
| `11:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | IANA Considerations: the sentence directs IANA in its administration of the 'Flow Spec Component Types' registry ('Types must be assigned and interpreted uniquely'). The producer is IANA, and Ze holds no code for that role and could hold none: it consumes code points and operates no code-point registry. | Types must be assigned and interpreted uniquely. |
| `11:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | An IANA allocation-policy table (Invalid value / Defined by this specification / Specification Required / First Come First Served), not a sentence stating an obligation on a BGP speaker. | +--------------+-------------------------------+ \| Range \| Policy \| +--------------+-------------------------------+ \| 0 \| Invalid value \| \| [1 .. 12] \| Defined by this specification \| \| [13 .. 127] \| Specification Required \| \| [128 .. 255] \| First Come First Served \| +--------------+-------------------------------+ |
| `11:3` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | IANA Considerations: the sentence states what a registering document must say, so the producer is the author of a future specification registering a flow component type under the 'Specification Required' policy. Ze holds no code for that role: it registers no new component type and publishes no such specification. | The specification of a particular "flow component type" must clearly identify what the criteria used to match packets forwarded by the router is. |

## Superseded

RFC 5575 is obsoleted by RFC 8955.

| Requirement | Disposition | Now stated at | Reason |
|---|---|---|---|
| [`RFC5575-4-1`](#rfc5575-4-1) Implementations wishing to exchange flow specification rules MUST use BGP's Capability Advertisement facility to exchange the Multiprotocol Extension Capability Code (Code 1) as defined in RFC 4760 [RFC4760]. (§4) | restated | RFC8955-4-1 | RFC 8955 Section 4 keeps the sentence, that implementations wishing to exchange Flow Specification MUST use BGP's Capability Advertisement facility to exchange the Multiprotocol Extension Capability Code (Code 1) |
| [`RFC5575-4-2`](#rfc5575-4-2) The (AFI, SAFI) pair carried in the Multiprotocol Extension Capability MUST be the same as the one used to identify a particular application that uses this NLRI-type. (§4) | restated | RFC8955-4-2 | RFC 8955 Section 4 makes the pair explicit rather than leaving it to the application: (AFI 1, SAFI 133) for IPv4 Flow Specification and (AFI 1, SAFI 134) for VPNv4 Flow Specification |
| [`RFC5575-4-3`](#rfc5575-4-3) Flow specification components must follow strict type ordering. (§4) | restated | RFC8955-4.2-1 | the NLRI value encoding moved from Section 4 to Section 4.2, which keeps the strict type ordering by increasing numerical order |
| [`RFC5575-4-4`](#rfc5575-4-4) A given component type may or may not be present in the specification, but if present, it MUST precede any component of higher numeric type value. (§4) | restated | RFC8955-4.2-2 | RFC 8955 Section 4.2 keeps the rule that a component, if present, MUST precede any component of higher numeric type value |
| [`RFC5575-6-1`](#rfc5575-6-1) BGP implementations MUST also enforce that the AS_PATH attribute of a route received via the External Border Gateway Protocol (eBGP) contains the neighboring AS in the left-most position of the AS_PATH attribute. (§6) | restated | RFC8955-6-2 | RFC 8955 Section 6 restates this rule, and RFC 9117 Section 4.2 subsequently replaces it with comparison against the first AS of the best-match unicast route |
| [`RFC5575-6-2`](#rfc5575-6-2) A flow specification NLRI must be validated such that it is considered feasible if and only if: a) The originator of the flow specification matches the originator of the best-match unicast route for the destination prefix embedded in the flow specification. b) There are no more specific unicast routes, when compared with the flow destination prefix, that have been received from a different neighboring AS than the best-match unicast route, which has been determined in step a). (§6) | restated | RFC8955-6-1 | RFC 8955 Section 6 keeps the feasibility rule and scopes it, adding that it applies in the absence of explicit configuration and that SAFI 133 validates against SAFI 1 while SAFI 134 validates against SAFI 128 |
| [`RFC5575-6-3`](#rfc5575-6-3) A flow specification NLRI must be validated such that it is considered feasible if and only if: a) The originator of the flow specification matches the originator of the best-match unicast route for the destination prefix embedded in the flow specification. (§6) | restated | RFC8955-6-1 | originator matching is condition (b) of RFC 8955 Section 6; RFC 9117 Section 4.1 adds the local-domain controller alternative |
| [`RFC5575-6-4`](#rfc5575-6-4) A flow specification NLRI must be validated such that it is considered feasible if and only if: a) The originator of the flow specification matches the originator of the best-match unicast route for the destination prefix embedded in the flow specification. b) There are no more specific unicast routes, when compared with the flow destination prefix, that have been received from a different neighboring AS than the best-match unicast route, which has been determined in step a). (§6) | restated | RFC8955-6-1 | the more-specific-route rule is condition (c) of RFC 8955 Section 6 |
| [`RFC5575-7-1`](#rfc5575-7-1) This extended community is encoded as a sequence of 5 zero bytes followed by the DSCP value encoded in the 6 least significant bits of 6th byte. (§7) | restated | RFC8955-7.5-1 | the traffic-marking action moved from Section 7 to Section 7.5, which keeps the reserved bits at 0 on encoding and adds the receive half, that they MUST be ignored during decoding |
| [`RFC5575-4-8`](#rfc5575-4-8) Whenever the corresponding application does not require Next-Hop information, this shall be encoded as a 0-octet length Next Hop in the MP_REACH_NLRI attribute and ignored on receipt. (§4) | restated | RFC8955-4-3 | RFC 8955 §4 splits the sentence into "Length of the Next-Hop Network Address MUST be set to 0" (RFC8955-4-3) and "Network Address of the Next-Hop field MUST be ignored" (RFC8955-4-4) |
| [`RFC5575-5.1-1`](#rfc5575-5.1-1) This ordering function must be such that it must not depend on the arrival order of the flow specification's rules and must be constant in the network. (§5.1) | restated | RFC8955-5.1-1 | RFC 8955 §5.1 keeps the ordering function that "does not depend on the arrival order of the Flow Specification via BGP" |
| [`RFC5575-8-1`](#rfc5575-8-1) The NLRI length field shall include both the 8 bytes of the Route Distinguisher as well as the subsequent flow specification. (§8) | restated | RFC8955-8-2 | RFC 8955 §8 keeps "The NLRI length field shall include both the 8 octets of the Route Distinguisher as well as the subsequent Flow Specification NLRI value" |
| [`RFC5575-3-1`](#rfc5575-3-1) Standard BGP policy mechanisms, such as UPDATE filtering by NLRI prefix and community matching, SHOULD apply to the newly defined NLRI-type. (§3) | unextracted | §3 | RFC 8955 Section 3 states the obligation with a lowercase keyword, that standard BGP policy mechanisms such as UPDATE filtering by NLRI prefix and community matching must apply to the Flow specification defined NLRI-type. rfc/short/rfc8955.md declares no row for it |
| [`RFC5575-9-1`](#rfc5575-9-1) While this is an implementation-specific choice, implementations SHOULD provide: o A mechanism to log the packet header of filtered traffic. (§9) | restated | RFC8955-9-1 | RFC 8955 Section 9 keeps the SHOULD to provide a mechanism to log the packet header of filtered traffic |
| [`RFC5575-9-2`](#rfc5575-9-2) While this is an implementation-specific choice, implementations SHOULD provide: o A mechanism to log the packet header of filtered traffic. o A mechanism to count the number of matches for a given flow specification rule. (§9) | restated | RFC8955-9-2 | RFC 8955 Section 9 keeps the SHOULD to provide a mechanism to count the number of matches for a given Flow Specification rule |
| [`RFC5575-7-2`](#rfc5575-7-2) Rather than attempting to define it here, this can be accomplished by mapping a user-defined community value to platform-/network-specific behavior via user configuration. (§7) | unextracted | §7.6 | RFC 8955 gives the paragraph its own section, 7.6, and keeps it word for word, that a user-defined community value can be mapped to platform-specific or network-specific behavior via user configuration. rfc/short/rfc8955.md declares no row for it |
