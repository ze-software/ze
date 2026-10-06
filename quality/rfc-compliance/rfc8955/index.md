# RFC 8955 - Dissemination of Flow Specification Rules

Partial. Every requirement this repository extracted from RFC 8955, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 72.4% | 21 of 29 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 20.7% | 6 of 29 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 29 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 29 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| No test at all | 0.0% | 0 of 29 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 36.6% | 30 of 82 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 29 | of 44 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 2 | of 29 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 6.9% | 2 of 29 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 29 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 29 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

The 8 shares marked as a part above are the whole of the 29 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 44 |
| Gated MUST-level | 29 |
| Not applicable, so out of scope | 2 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 82 |
| Tagged units | 82 |
| Recorded audit verdicts | 22 |
| Discrimination records | 30 |
| Summary | `rfc/short/rfc8955.md` |
| Requirement shard | `rfc/requirements/rfc8955.md` |
| RFC text | `rfc/full/rfc8955.txt` |

## Enrolment

Enrolled: Dissemination of Flow Specification Rules

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

- IPv4 FlowSpec and FlowSpec VPN NLRI encoding, decoding, route config, filters, and action communities: (AFI 1, SAFI 133/134) Multiprotocol capability negotiation, ascending component ordering, first-operator AND-bit handling on encode and decode, single-octet TCP-flags/DSCP/fragment encodings, bitmask and fragment reserved bits ignored on decode, traffic-rate encode rejection of negative rates and decode clamping to zero, traffic-action unused bits zero on encode and ignored on decode, traffic-marking reserved bits ignored on decode, and lowering to the firewall
- tests bound per requirement in [`rfc/requirements/rfc8955.md`](https://github.com/ze-software/ze/blob/main/rfc/requirements/rfc8955.md).


**What the ledger says remains**

The native codec, prefix-policy projection, unicast-authorized selection and revalidation, and selected-rule firewall path have source changes and regression carriers awaiting integration validation. No conformance result follows from these edits alone. Kernel precedence, continuation, sampling, policing, marking, and withdrawal require the Linux integration carrier with network-namespace and nftables privileges. VPN FlowSpec is validated and propagated by the BGP RIB but is not installed by the global firewall bridge. The rule-a destination bypass remains absent.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 21 | one part of the gated population |
| Annotated (including scoped evidence) | 8 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **29** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (21):** [`RFC8955-4-3`](#rfc8955-4-3), [`RFC8955-4-4`](#rfc8955-4-4), [`RFC8955-4.2-1`](#rfc8955-4.2-1), [`RFC8955-4.2-2`](#rfc8955-4.2-2), [`RFC8955-4.2.1.1-2`](#rfc8955-4.2.1.1-2), [`RFC8955-4.2.1.1-3`](#rfc8955-4.2.1.1-3), [`RFC8955-4.2.1.2-1`](#rfc8955-4.2.1.2-1), [`RFC8955-4.2.2.12-2`](#rfc8955-4.2.2.12-2), [`RFC8955-6-1`](#rfc8955-6-1), [`RFC8955-6-3`](#rfc8955-6-3), [`RFC8955-7.1-1`](#rfc8955-7.1-1), [`RFC8955-7.1-2`](#rfc8955-7.1-2), [`RFC8955-7.3-1`](#rfc8955-7.3-1), [`RFC8955-7.5-1`](#rfc8955-7.5-1), [`RFC8955-5.1-1`](#rfc8955-5.1-1), [`RFC8955-5.1-2`](#rfc8955-5.1-2), [`RFC8955-8-1`](#rfc8955-8-1), [`RFC8955-8-2`](#rfc8955-8-2), [`RFC8955-3-1`](#rfc8955-3-1), [`RFC8955-6-5`](#rfc8955-6-5), [`RFC8955-7.3-2`](#rfc8955-7.3-2)

**Annotated (including scoped evidence) (8):** [`RFC8955-4-1`](#rfc8955-4-1), [`RFC8955-4-2`](#rfc8955-4-2), [`RFC8955-4.2.1.1-1`](#rfc8955-4.2.1.1-1), [`RFC8955-4.2.2.9-1`](#rfc8955-4.2.2.9-1), [`RFC8955-4.2.2.11-1`](#rfc8955-4.2.2.11-1), [`RFC8955-4.2.2.12-1`](#rfc8955-4.2.2.12-1), [`RFC8955-12-1`](#rfc8955-12-1), [`RFC8955-12-2`](#rfc8955-12-2)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC8955-4-1` | Implementations wishing to exchange Flow Specification MUST use BGP's Capability Advertisement facility to exchange the Multiprotocol Extension Capability Code (Code 1) (§4) | MUST | 4 | **positive:** `unit/verify` [`TestIPv4FlowSpecNegotiatesMultiprotocolCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/plugin_test.go#L894). **negative:** no negative test. **{single-polarity}:** the flowspec plugin unconditionally maps each declared FlowSpec family to a Multiprotocol (Code 1) capability during OPEN, so there is no wrong input the negotiation path rejects (internal/component/bgp/plugins/nlri/flowspec/register.go, types.go:47) |
| `RFC8955-4-2` | The (AFI, SAFI) pair carried in the Multiprotocol Extension Capability MUST be (AFI=1, SAFI=133) for IPv4 Flow Specification and (AFI=1, SAFI=134) for VPNv4 Flow Specification (§4) | MUST | 4 | **positive:** `unit/verify` [`TestFlowSpecVPNFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L647). **positive:** `unit/verify` [`TestIPv4FlowSpecNegotiatesMultiprotocolCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/plugin_test.go#L895). **negative:** no negative test. **{single-polarity}:** the (AFI 1, SAFI 133) and (AFI 1, SAFI 134) pairs are family-registration constants, not an input guard, so only the positive assignment is assertable (internal/component/bgp/plugins/nlri/flowspec/types.go:47-50) |
| `RFC8955-4-3` | Length of the Next-Hop Network Address MUST be set to 0 (§4) | MUST | 4 | **positive:** `unit/verify` [`TestFlowSpecForwardingOmitsNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/flowspec_forward_wire_test.go#L31). **positive:** `unit/verify` [`TestFlowSpecOriginationOmitsConfiguredNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/flowspec_origin_wire_test.go#L32). **positive:** `unit/verify` [`TestFlowSpecRouteServerOmitsNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/flowspec_rs_wire_test.go#L25). **positive:** `unit/verify` [`TestFlowSpecUpdateIgnoresConfiguredNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/flowspec_wire_contract_test.go#L14). **negative:** `unit/verify` [`TestFlowSpecForwardingKeepsLegacyNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/flowspec_forward_mixed_test.go#L16). **negative:** `unit/verify` [`TestFlowSpecForwardingOmitsNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/flowspec_forward_wire_test.go#L32). **negative:** `unit/verify` [`TestFlowSpecOriginationOmitsConfiguredNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/flowspec_origin_wire_test.go#L33). **negative:** `unit/verify` [`TestFlowSpecRouteServerOmitsNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/flowspec_rs_wire_test.go#L26). **negative:** `unit/verify` [`TestFlowSpecUpdateIgnoresConfiguredNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/flowspec_wire_contract_test.go#L15). **positive:** `functional/verify` [`flow-encode.ci`](https://github.com/ze-software/ze/blob/main/test/encode/flow-encode.ci#L11) |
| `RFC8955-4-4` | Network Address of the Next-Hop field MUST be ignored (§4) | MUST | 4 | **positive:** `unit/verify` [`TestRFC8955NextHopIgnoredForFlowSpec`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_flowspec_validation_test.go#L498). **negative:** `unit/verify` [`TestRFC8955NextHopIgnoredForFlowSpec`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_flowspec_validation_test.go#L499) |
| `RFC8955-4.2-1` | Components MUST follow strict type ordering by increasing numerical order (§4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestFlowSpecComponentsAscendingOrder`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1228). **positive:** `unit/verify` [`TestFlowSpecJoinsRepeatedTypeIntoOneComponent`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1270). **negative:** `unit/verify` [`TestAddComponentRefusesASecondPrefix`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1387). **negative:** `unit/verify` [`TestParseFlowSpecRefusesRepeatedComponentType`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1307). **negative:** `unit/verify` [`TestParseFlowSpecVPNRefusesRepeatedComponentType`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1367) |
| `RFC8955-4.2-2` | If present, it MUST precede any component of higher numeric type value. (§4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestFlowSpecComponentsAscendingOrder`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1229). **negative:** `unit/verify` [`TestParseFlowSpecRefusesDescendingComponentType`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1335) |
| `RFC8955-4.2.1.1-1` | In the first operator octet of a sequence, it MUST be encoded as unset (§4.2.1.1) | MUST | 4.2.1.1 | **positive:** `unit/verify` [`TestRFC8955FirstOperatorAndBitEncodedUnset`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1539). **negative:** no negative test. **{single-polarity}:** parseFlowMatches derives the AND bit purely from the position inside a '&'-joined expression (isAnd := i > 0), so the first operator-value pair is always encoded with the AND bit clear and no input sets it (internal/component/bgp/plugins/nlri/flowspec/config_builder.go:220,:252) |
| `RFC8955-4.2.1.1-2` | In the first operator octet of a sequence, it MUST be encoded as unset and MUST be treated as always unset on decoding. (§4.2.1.1) | MUST | 4.2.1.1 | **positive:** `unit/verify` [`TestRFC8955FirstOperatorAndBitTreatedUnsetOnDecode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1570). **negative:** `unit/verify` [`TestRFC8955FirstOperatorAndBitTreatedUnsetOnDecode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1574) |
| `RFC8955-4.2.1.1-3` | 0: MUST be set to 0 on NLRI encoding and MUST be ignored during decoding (§4.2.1.1) | MUST | 4.2.1.1 | **positive:** `unit/verify` [`TestFlowSpecNumericReservedAndEightOctetOperand`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/compare_rfc8955_test.go#L42). **negative:** `unit/verify` [`TestFlowSpecNumericReservedAndEightOctetOperand`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/compare_rfc8955_test.go#L43) |
| `RFC8955-4.2.1.2-1` | 0 (all 0 bits): MUST be set to 0 on NLRI encoding and MUST be ignored during decoding (§4.2.1.2) | MUST | 4.2.1.2 | **positive:** `unit/verify` [`TestFlowSpecBitmaskOperatorReservedBitsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1448). **negative:** `unit/verify` [`TestRFC8955BitmaskOperatorReservedBitsIgnoredOnDecode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1608) |
| `RFC8955-4.2.2.9-1` | Type 9 component bitmasks MUST be encoded as 1- or 2-octet bitmask (bitmask_op len=00 or len=01). (§4.2.2.9) | MUST | 4.2.2.9 | **positive:** `unit/verify` [`TestRFC8955TCPFlagsBitmaskSingleOctet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1668). **negative:** no negative test. **{single-polarity}:** parseFlowTCPFlagMatches resolves flag names to 8-bit values and numericComponent.Bytes() selects the 1-octet length code for any value <= 0xFF, so an emitted Type-9 bitmask is always 1 octet and no over-long bitmask can be produced to reject (internal/component/bgp/plugins/nlri/flowspec/config_builder.go:414-446, types_numeric.go:47-55) |
| `RFC8955-4.2.2.11-1` | Type 11 component values MUST be encoded as single octet (numeric_op len=00). (§4.2.2.11) | MUST | 4.2.2.11 | **positive:** `unit/verify` [`TestRFC8955DSCPValueSingleOctet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1696). **negative:** no negative test. **{single-polarity}:** parseFlowOctets parses DSCP values as uint8 and NewFlowDSCPComponent stores them, so numericComponent.Bytes() always selects the 1-octet length code and no multi-octet DSCP encoding exists to reject (internal/component/bgp/plugins/nlri/flowspec/config_builder.go:261-273, types_numeric.go:473-479) |
| `RFC8955-4.2.2.12-1` | The Type 12 component bitmask MUST be encoded as single octet bitmask (bitmask_op len=00). (§4.2.2.12) | MUST | 4.2.2.12 | **positive:** `unit/verify` [`TestRFC8955FragmentBitmaskSingleOctet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1723). **negative:** no negative test. **{single-polarity}:** fragment values come from the four low-nibble FlowFragmentFlag constants, so numericComponent.Bytes() always selects the 1-octet length code and no multi-octet fragment bitmask can be produced to reject (internal/component/bgp/plugins/nlri/flowspec/types.go:201-206, types_numeric.go:481-491) |
| `RFC8955-4.2.2.12-2` | 0: MUST be set to 0 on NLRI encoding and MUST be ignored during decoding (§4.2.2.12) | MUST | 4.2.2.12 | **positive:** `unit/verify` [`TestFlowSpecFragmentReservedHighNibbleZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1476). **negative:** `unit/verify` [`TestRFC8955FragmentReservedBitsIgnoredOnDecode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1641) |
| `RFC8955-6-1` | In the absence of explicit configuration, a Flow Specification NLRI MUST be validated such that it is considered feasible if and only if all of the conditions below are true: (§6, updated by RFC 9117 §4.1) | MUST | 6 | **positive:** `unit/verify` [`TestFlowSpecAuthorizationFromReceivedUpdates`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_flowspec_validation_test.go#L126). **positive:** `unit/verify` [`TestFlowSpecForeignMoreSpecificLosingPathInvalidatesRule`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_flowspec_validation_test.go#L414). **positive:** `unit/verify` [`TestFlowSpecVPNValidationSeparatesAFIAndRD`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_flowspec_validation_test.go#L290). **positive:** `unit/verify` [`TestRFC9117AuthorizationUsesLongestCoveringRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9117_best_match_test.go#L18). **negative:** `unit/verify` [`TestFlowSpecAuthorizationFromReceivedUpdates`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_flowspec_validation_test.go#L127). **negative:** `unit/verify` [`TestFlowSpecForeignMoreSpecificLosingPathInvalidatesRule`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_flowspec_validation_test.go#L415). **negative:** `unit/verify` [`TestFlowSpecVPNValidationSeparatesAFIAndRD`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_flowspec_validation_test.go#L291). **negative:** `unit/verify` [`TestRFC9117AuthorizationUsesLongestCoveringRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9117_best_match_test.go#L19) |
| `RFC8955-6-3` | Therefore, a revalidation of the Flow Specification NLRI MUST be performed whenever unicast routes change. (§6) | MUST | 6 | **positive:** `unit/verify` [`TestFlowSpecUnicastLifecycleRevalidatesRetainedRule`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_flowspec_validation_test.go#L205). **negative:** `unit/verify` [`TestFlowSpecUnicastLifecycleRevalidatesRetainedRule`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_flowspec_validation_test.go#L206) |
| `RFC8955-7.1-1` | On encoding, the traffic-rate MUST NOT be negative. (§7.1, §7.2) | MUST NOT | 7.1 | **positive:** `unit/verify` [`TestParseExtendedCommunitiesTrafficRatePackets`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/route/rfc8955_route_parse_test.go#L1239). **negative:** `unit/verify` [`TestParseExtendedCommunitiesTrafficRatePackets`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/route/rfc8955_route_parse_test.go#L1240) |
| `RFC8955-7.1-2` | On decoding, negative values MUST be treated as zero (discard all traffic). (§7.1, §7.2) | MUST | 7.1 | **positive:** `unit/verify` [`TestRFC8955TrafficRateNegativeDecodesAsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/cli/rfc8955_decode_test.go#L1465). **negative:** `unit/verify` [`TestRFC8955TrafficRateNegativeDecodesAsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/cli/rfc8955_decode_test.go#L1473) |
| `RFC8955-7.3-1` | These bits MUST be set to 0 on encoding and MUST be ignored during decoding. (§7.3) | MUST | 7.3 | **positive:** `unit/verify` [`TestRFC8955TrafficActionBitsDecoded`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc8955_extcomm_decoded_test.go#L243). **positive:** `unit/verify` [`TestRFC8955TrafficActionUnusedBitsIgnoredOnDecode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/cli/rfc8955_decode_test.go#L1510). **positive:** `unit/verify` [`TestRFC8955TrafficActionUnusedBitsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/routeattr_test.go#L209). **negative:** `unit/verify` [`TestRFC8955TrafficActionBitsDecoded`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc8955_extcomm_decoded_test.go#L257). **negative:** `unit/verify` [`TestRFC8955TrafficActionUnusedBitsIgnoredOnDecode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/cli/rfc8955_decode_test.go#L1517). **positive:** `functional/verify` [`community-attributes-json.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/community-attributes-json.ci#L16). **negative:** `functional/verify` [`community-attributes-json.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/community-attributes-json.ci#L17) |
| `RFC8955-7.5-1` | reserved (r): MUST be set to 0 on encoding and MUST be ignored during decoding (§7.5) | MUST | 7.5 | **positive:** `unit/verify` [`TestEncodeRouteEmitsZeroValuedActions`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/rfc8955_encode_test.go#L53). **positive:** `unit/verify` [`TestEncodeRouteMarkKeepsReservedBitsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/rfc8955_encode_test.go#L86). **positive:** `unit/verify` [`TestParseExtendedCommunityMarkDSCPBound`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/rfc8955_routeattr_flowspec_test.go#L34). **positive:** `unit/verify` [`TestRFC8955TrafficMarkingReservedBitsIgnored`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc8955_extcomm_decoded_test.go#L209). **negative:** `unit/verify` [`TestEncodeRouteMarkKeepsReservedBitsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/rfc8955_encode_test.go#L92). **negative:** `unit/verify` [`TestParseExtendedCommunityMarkDSCPBound`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/rfc8955_routeattr_flowspec_test.go#L56). **negative:** `unit/verify` [`TestRFC8955TrafficMarkingReservedBitsIgnored`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc8955_extcomm_decoded_test.go#L216). **positive:** `functional/verify` [`community-attributes-json.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/community-attributes-json.ci#L14). **negative:** `functional/verify` [`community-attributes-json.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/community-attributes-json.ci#L15) |
| `RFC8955-12-1` | Specifications relaxing the validation restrictions MUST contain security considerations that provide details on the required additional filtering. (§12) | MUST | 12 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze publishes no specification introducing a validation relaxation. The implemented RFC 9117 rules are standards-defined; flowSpecAuthorized in internal/component/bgp/plugins/rib/rib_flowspec_validation.go still requires a valid destination and covering unicast reachability, with no configurable destination bypass |
| `RFC8955-5.1-1` | This ordering function is such that it does not depend on the arrival order of the Flow Specification via BGP and thus is consistent in the network. (§5.1) | MUST | 5.1 | **positive:** `unit/verify` [`TestFlowSpecPrecedenceFromWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/compare_rfc8955_test.go#L9). **negative:** `unit/verify` [`TestFlowSpecPrecedenceFromWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/compare_rfc8955_test.go#L10) |
| `RFC8955-5.1-2` | The relative order of two Flow Specifications is determined by comparing their respective components. The algorithm starts by comparing the left-most components (lowest component type value) of the Flow Specifications. If the types differ, the Flow Specification with lowest numeric type value has higher precedence (and thus will match before) than the Flow Specification that doesn't contain that component type. If the component types are the same, then a type- specific comparison is performed (see below). If the types are equal, the algorithm continues with the next component. For IP prefix values (IP destination or source prefix), if one of the two prefixes to compare is a more specific prefix of the other, the more specific prefix has higher precedence. Otherwise, the one with the lowest IP value has higher precedence. For all other component types, unless otherwise specified, the comparison is performed by comparing the component data as a binary string using the memcmp() function as defined by [ISO_IEC_9899]. For strings with equal lengths, the lowest string (memcmp) has higher precedence. For strings of different lengths, the common prefix is compared. If the common prefix is not equal, the string with the lowest prefix has higher precedence. If the common prefix is equal, the longest string is considered to have higher precedence than the shorter one. (§5.1) | MUST | 5.1 | **positive:** `unit/verify` [`TestFlowSpecPrecedenceFromWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/compare_rfc8955_test.go#L11). **positive:** `unit/verify` [`TestSelectedFlowSpecKernelPacketSemantics`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowspec-firewall/rfc8955_selected_integration_linux_test.go#L23). **negative:** `unit/verify` [`TestFlowSpecPrecedenceFromWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/compare_rfc8955_test.go#L12). **negative:** `unit/verify` [`TestSelectedFlowSpecKernelPacketSemantics`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowspec-firewall/rfc8955_selected_integration_linux_test.go#L24) |
| `RFC8955-8-1` | The NLRI format for this address family consists of a fixed-length Route Distinguisher field (8 octets) followed by the Flow Specification NLRI value (Section 4.2). (§8) | MUST | 8 | **positive:** `unit/verify` [`TestRFC8955VPNNLRIStructure`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/rfc8955_vpn_test.go#L27). **negative:** `unit/verify` [`TestRFC8955VPNNLRIStructure`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/rfc8955_vpn_test.go#L28) |
| `RFC8955-8-2` | The NLRI length field shall include both the 8 octets of the Route Distinguisher as well as the subsequent Flow Specification NLRI value. (§8) | MUST | 8 | **positive:** `unit/verify` [`TestRFC8955VPNLengthCoversRD`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/rfc8955_vpn_test.go#L56). **negative:** `unit/verify` [`TestRFC8955VPNLengthCoversRD`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/rfc8955_vpn_test.go#L57) |
| `RFC8955-3-1` | Standard BGP policy mechanisms, such as UPDATE filtering by NLRI prefix as well as community matching, must apply to the Flow specification defined NLRI-type. (§3) | MUST | 3 | **positive:** `unit/verify` [`TestPrefixPolicyFiltersFlowSpecDestination`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/filter_prefix/rfc8955_flowspec_test.go#L12). **positive:** `unit/verify` [`TestRFC8955CommunityPolicyAcceptsAPermittedFlowSpecRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/filter_community_match/rfc8955_flowspec_test.go#L24). **negative:** `unit/verify` [`TestPrefixPolicyFiltersFlowSpecDestination`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/filter_prefix/rfc8955_flowspec_test.go#L13). **negative:** `unit/verify` [`TestRFC8955CommunityPolicyRejectsADeniedFlowSpecRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/filter_community_match/rfc8955_flowspec_test.go#L39) |
| `RFC8955-6-5` | Although the forwarding attributes of two routes for the same Flow Specification prefix may be the same, BGP is still required to perform its path selection algorithm in order to select the correct set of attributes to advertise. (§6) | MUST | 6 | **positive:** `unit/verify` [`TestRFC8955FlowSpecPathSelectionPicksOneSetOfAttributes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8955_flowspec_bestchange_test.go#L79). **negative:** `unit/verify` [`TestRFC8955FlowSpecLosingPathIsNeverPublished`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8955_flowspec_bestchange_test.go#L159) |
| `RFC8955-7.3-2` | The use of the Terminal Action (bit 47) may result in more than one Flow Specification matching a particular traffic flow. All the Traffic Filtering Actions from these Flow Specifications shall be collected and applied. (§7.3) | MUST | 7.3 | **positive:** `unit/verify` [`TestSelectedFlowSpecKernelPacketSemantics`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowspec-firewall/rfc8955_selected_integration_linux_test.go#L25). **negative:** `unit/verify` [`TestSelectedFlowSpecKernelPacketSemantics`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowspec-firewall/rfc8955_selected_integration_linux_test.go#L26) |
| `RFC8955-12-2` | For a network to utilize this relaxation, the BGP policies must support additional filtering since the origin AS field is empty. (§12) | MUST | 12 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** conditional on an absent feature. flowSpecDestination and flowSpecAuthorized in internal/component/bgp/plugins/rib/rib_flowspec_validation.go require a valid destination, including offset zero for IPv6, and no configuration bypasses that guard. The owner explicitly retained strict destination validation |
| `RFC8955-4.2.2.3-1` | Type 3 component values SHOULD be encoded as single octet (numeric_op len=00). (§4.2.2.3) | SHOULD | 4.2.2.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC8955-4.2.2.4-1` | Type 4 component values SHOULD be encoded as 1- or 2-octet quantities (numeric_op len=00 or len=01). (§4.2.2.4) | SHOULD | 4.2.2.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC8955-4.2.2.7-1` | Type 7 component values SHOULD be encoded as single octet (numeric_op len=00). (§4.2.2.7) | SHOULD | 4.2.2.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC8955-4.2.2.11-2` | The six least significant bits contain the DSCP value. All other bits SHOULD be treated as 0. (§4.2.2.11) | SHOULD | 4.2.2.11 | **positive:** no positive test. **negative:** no negative test |
| `RFC8955-7-1` | Multiple Traffic Filtering Actions defined in this document may be present for a single Flow Specification and SHOULD be applied to the traffic flow (for example, traffic-rate-bytes and rt-redirect can be applied to packets at the same time). (§7) | SHOULD | 7 | **positive:** no positive test. **negative:** no negative test |
| `RFC8955-7.1-3` | The first two octets carry the 2-octet id, which can be assigned from a 2-octet AS number. When a 4-octet AS number is locally present, the 2 least significant octets of such an AS number can be used. This value is purely informational and SHOULD NOT be interpreted by the implementation. (§7.1) | SHOULD NOT | 7.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8955-4.2-3` | All combinations of components within a single Flow Specification are allowed. However, some combinations cannot match any packets (e.g., "ICMP Type AND Port" will never match any packets) and thus SHOULD NOT be propagated by BGP. (§4.2) | SHOULD NOT | 4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC8955-9-1` | While this is an implementation specific choice, implementations SHOULD provide: * A mechanism to log the packet header of filtered traffic. (§9) | SHOULD | 9 | **positive:** no positive test. **negative:** no negative test |
| `RFC8955-9-2` | While this is an implementation specific choice, implementations SHOULD provide: * A mechanism to log the packet header of filtered traffic. * A mechanism to count the number of matches for a given Flow Specification rule. (§9) | SHOULD | 9 | **positive:** no positive test. **negative:** no negative test |
| `RFC8955-7.7-1` | If a Flow Specification associated with interfering Traffic Filtering Actions is selected for packet forwarding, it is an implementation decision which of the interfering Traffic Filtering Actions are selected. Implementors of this specification SHOULD document the behavior of their implementation in such cases. (§7.7) | SHOULD | 7.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC8955-7-2` | Any additional definition of Traffic Filtering Actions SHOULD specify the action to take if those Traffic Filtering Actions interfere (also with existing Traffic Filtering Actions). (§7) | SHOULD | 7 | **positive:** no positive test. **negative:** no negative test |
| `RFC8955-7.1-4` | A traffic-rate of 0 should result on all traffic for the particular flow to be discarded. (§7.1, §7.2) | SHOULD | 7.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8955-7.6-1` | Implementations should provide mechanisms that map an arbitrary BGP community value (normal or extended) to Traffic Filtering Actions that require different mappings on different systems in the network. (§7.6) | SHOULD | 7.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC8955-6-4` | However, rule a MAY be relaxed by explicit configuration, permitting Flow Specifications that include no destination prefix component. If such is the case, rules b and c are moot and MUST be disregarded. (§6) | MAY | 6 | **positive:** no positive test. **negative:** no negative test |
| `RFC8955-4.2-4` | A given component type MAY (exactly once) be present in the Flow Specification. (§4.2) | MAY | 4.2 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC8955-12-1`](#rfc8955-12-1) Specifications relaxing the validation restrictions MUST contain security considerations that provide details on the required additional filtering. (§12) | no test | no test carries this requirement id; annotated {not-applicable}: ze publishes no specification introducing a validation relaxation. The implemented RFC 9117 rules are standards-defined; flowSpecAuthorized in internal/component/bgp/plugins/rib/rib_flowspec_validation.go still requires a valid destination and covering unicast reachability, with no configurable destination bypass |
| [`RFC8955-12-2`](#rfc8955-12-2) For a network to utilize this relaxation, the BGP policies must support additional filtering since the origin AS field is empty. (§12) | no test | no test carries this requirement id; annotated {not-applicable}: conditional on an absent feature. flowSpecDestination and flowSpecAuthorized in internal/component/bgp/plugins/rib/rib_flowspec_validation.go require a valid destination, including offset zero for IPv6, and no configuration bypasses that guard. The owner explicitly retained strict destination validation |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC8955-4-1`](#rfc8955-4-1)

Implementations wishing to exchange Flow Specification MUST use BGP's Capability Advertisement facility to exchange the Multiprotocol Extension Capability Code (Code 1) (§4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestIPv4FlowSpecNegotiatesMultiprotocolCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/plugin_test.go#L894) | unit/verify | unproven |

### [`RFC8955-4-2`](#rfc8955-4-2)

The (AFI, SAFI) pair carried in the Multiprotocol Extension Capability MUST be (AFI=1, SAFI=133) for IPv4 Flow Specification and (AFI=1, SAFI=134) for VPNv4 Flow Specification (§4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestIPv4FlowSpecNegotiatesMultiprotocolCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/plugin_test.go#L895) | unit/verify | unproven |
| positive | [`TestFlowSpecVPNFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L647) | unit/verify | unproven |

### [`RFC8955-4-3`](#rfc8955-4-3)

Length of the Next-Hop Network Address MUST be set to 0 (§4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFlowSpecUpdateIgnoresConfiguredNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/flowspec_wire_contract_test.go#L15) | unit/verify | unproven |
| negative | [`TestFlowSpecForwardingKeepsLegacyNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/flowspec_forward_mixed_test.go#L16) | unit/verify | revert, verified |
| negative | [`TestFlowSpecForwardingOmitsNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/flowspec_forward_wire_test.go#L32) | unit/verify | revert, verified |
| negative | [`TestFlowSpecOriginationOmitsConfiguredNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/flowspec_origin_wire_test.go#L33) | unit/verify | revert, verified |
| negative | [`TestFlowSpecRouteServerOmitsNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/flowspec_rs_wire_test.go#L26) | unit/verify | revert, verified |
| positive | [`TestFlowSpecUpdateIgnoresConfiguredNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/flowspec_wire_contract_test.go#L14) | unit/verify | unproven |
| positive | [`TestFlowSpecForwardingOmitsNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/flowspec_forward_wire_test.go#L31) | unit/verify | revert, verified |
| positive | [`TestFlowSpecOriginationOmitsConfiguredNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/flowspec_origin_wire_test.go#L32) | unit/verify | revert, verified |
| positive | [`TestFlowSpecRouteServerOmitsNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/flowspec_rs_wire_test.go#L25) | unit/verify | revert, verified |
| positive | [`flow-encode.ci`](https://github.com/ze-software/ze/blob/main/test/encode/flow-encode.ci#L11) | functional/verify | revert, verified |

### [`RFC8955-4-4`](#rfc8955-4-4)

Network Address of the Next-Hop field MUST be ignored (§4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8955NextHopIgnoredForFlowSpec`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_flowspec_validation_test.go#L499) | unit/verify | revert, verified |
| positive | [`TestRFC8955NextHopIgnoredForFlowSpec`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_flowspec_validation_test.go#L498) | unit/verify | revert, verified |

### [`RFC8955-4.2-1`](#rfc8955-4.2-1)

Components MUST follow strict type ordering by increasing numerical order (§4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestAddComponentRefusesASecondPrefix`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1387) | unit/verify | unproven |
| negative | [`TestParseFlowSpecRefusesRepeatedComponentType`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1307) | unit/verify | unproven |
| negative | [`TestParseFlowSpecVPNRefusesRepeatedComponentType`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1367) | unit/verify | unproven |
| positive | [`TestFlowSpecComponentsAscendingOrder`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1228) | unit/verify | unproven |
| positive | [`TestFlowSpecJoinsRepeatedTypeIntoOneComponent`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1270) | unit/verify | unproven |

### [`RFC8955-4.2-2`](#rfc8955-4.2-2)

If present, it MUST precede any component of higher numeric type value. (§4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. positive re-parses an out-of-order build and pins the exact emitted order 1,3,5,12; negative feeds a descending 5-then-3 NLRI and asserts ErrFlowSpecTypeOrder, with the same components ascending accepted, so the refusal is isolated to order

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestParseFlowSpecRefusesDescendingComponentType`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1335) | unit/verify | unproven |
| positive | [`TestFlowSpecComponentsAscendingOrder`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1229) | unit/verify | unproven |

### [`RFC8955-4.2.1.1-1`](#rfc8955-4.2.1.1-1)

In the first operator octet of a sequence, it MUST be encoded as unset (§4.2.1.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. single-polarity positive: a config range '>8080&<8088' encodes two ops and the test asserts the first carries AND clear (0x40) while the second carries it set, so an encoder setting AND on the first operator goes red

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC8955FirstOperatorAndBitEncodedUnset`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1539) | unit/verify | unproven |

### [`RFC8955-4.2.1.1-2`](#rfc8955-4.2.1.1-2)

In the first operator octet of a sequence, it MUST be encoded as unset and MUST be treated as always unset on decoding. (§4.2.1.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. decodes the same OR list with the first operator's AND bit clear and set and asserts the identical [[=80],[=22]] structure. The parser guard (types_numeric.go And: len(matches) > 0) is backed by consumer guards (plugin_decode.go andGroup, types_numeric.go i > 0), so breaking the parser guard alone stays green; the assertion is on the end-to-end decode, which is what the sentence obliges. Test comment misquotes the encode half as SHOULD

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8955FirstOperatorAndBitTreatedUnsetOnDecode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1574) | unit/verify | unproven |
| positive | [`TestRFC8955FirstOperatorAndBitTreatedUnsetOnDecode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1570) | unit/verify | unproven |

### [`RFC8955-4.2.1.1-3`](#rfc8955-4.2.1.1-3)

0: MUST be set to 0 on NLRI encoding and MUST be ignored during decoding (§4.2.1.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. wire op 0xf9 carries the reserved bit (0x08) and a leading AND; positive asserts the exact decoded FlowMatch{Op: FlowOpEqual} (reserved bit ignored), negative asserts the re-encoded op is 0xb1 (reserved and AND cleared on encoding)

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFlowSpecNumericReservedAndEightOctetOperand`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/compare_rfc8955_test.go#L43) | unit/verify | unproven |
| positive | [`TestFlowSpecNumericReservedAndEightOctetOperand`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/compare_rfc8955_test.go#L42) | unit/verify | unproven |

### [`RFC8955-4.2.1.2-1`](#rfc8955-4.2.1.2-1)

0 (all 0 bits): MUST be set to 0 on NLRI encoding and MUST be ignored during decoding (§4.2.1.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-read 2026-09-27 after J078 deleted the retired RFC5575-4-6 tag and reworded one comment inside TestFlowSpecBitmaskOperatorReservedBitsZero; no assertion changed. Encode: a bitmask operator with 0x0C set on the wire is forbidden; assert.Zero(op&0x0C) over every emitted TCP-flags operator (MATCH, NOT, AND/END) goes red on it. Decode: reading meaning into 0x0C is forbidden; TestRFC8955BitmaskOperatorReservedBitsIgnoredOnDecode decodes op 0x8D and assert.Equal against the 0x81 render goes red if the bits change the match, with the clean render pinned first. The decode proof is at the rendered match, not at the firewall lowering.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8955BitmaskOperatorReservedBitsIgnoredOnDecode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1608) | unit/verify | unproven |
| positive | [`TestFlowSpecBitmaskOperatorReservedBitsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1448) | unit/verify | unproven |

### [`RFC8955-4.2.2.9-1`](#rfc8955-4.2.2.9-1)

Type 9 component bitmasks MUST be encoded as 1- or 2-octet bitmask (bitmask_op len=00 or len=01). (§4.2.2.9)

Audit verdict: enforced (the tests do what the requirement demands), fresh. single-polarity positive: every emitted tcp-flags bitmask from a config with AND/NOT/OR terms is asserted non-empty and at most 2 octets

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC8955TCPFlagsBitmaskSingleOctet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1668) | unit/verify | unproven |

### [`RFC8955-4.2.2.11-1`](#rfc8955-4.2.2.11-1)

Type 11 component values MUST be encoded as single octet (numeric_op len=00). (§4.2.2.11)

Audit verdict: enforced (the tests do what the requirement demands), fresh. single-polarity positive: DSCP 46, 0 and 63 each encode as exactly one octet

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC8955DSCPValueSingleOctet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1696) | unit/verify | unproven |

### [`RFC8955-4.2.2.12-1`](#rfc8955-4.2.2.12-1)

The Type 12 component bitmask MUST be encoded as single octet bitmask (bitmask_op len=00). (§4.2.2.12)

Audit verdict: enforced (the tests do what the requirement demands), fresh. single-polarity positive: all four fragment flags encode as exactly one-octet bitmasks

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC8955FragmentBitmaskSingleOctet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1723) | unit/verify | unproven |

### [`RFC8955-4.2.2.12-2`](#rfc8955-4.2.2.12-2)

0: MUST be set to 0 on NLRI encoding and MUST be ignored during decoding (§4.2.2.12)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-read 2026-09-27 after J078 deleted the retired RFC5575-4-7 tag and reworded one comment inside TestFlowSpecFragmentReservedHighNibbleZero; no assertion changed. Encode: a fragment value with any 0xF0 bit set is forbidden; assert.Zero(v[0]&0xF0) over each emitted value goes red on it. Decode: reading meaning into the high nibble is forbidden; TestRFC8955FragmentReservedBitsIgnoredOnDecode decodes 0xF2 and assert.Equal against the 0x02 render goes red if it changes the match, with the clean render pinned first. The decode proof is at the rendered match, not at the firewall lowering.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8955FragmentReservedBitsIgnoredOnDecode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1641) | unit/verify | unproven |
| positive | [`TestFlowSpecFragmentReservedHighNibbleZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1476) | unit/verify | unproven |

### [`RFC8955-6-1`](#rfc8955-6-1)

In the absence of explicit configuration, a Flow Specification NLRI MUST be validated such that it is considered feasible if and only if all of the conditions below are true: (§6, updated by RFC 9117 §4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent semantic rejudgment: RFC8955 Section 6: "In the absence of explicit configuration, a Flow Specification NLRI MUST be validated such that it is considered feasible if and only if all of the conditions below are true: a) A destination prefix component is embedded in the Flow Specification. b) The originator of the Flow Specification matches the originator of the best-match unicast route for the destination prefix embedded in the Flow Specification (this is the unicast route with the longest possible prefix length covering the destination prefix embedded in the Flow Specification). c) There are no "more-specific" unicast routes, when compared with the flow destination prefix, that have been received from a different neighboring AS than the best-match unicast route, which has been determined in rule b." The existing receive-driven authorization units retain unique IDs, real received attributes and exact eligibility/candidate/selected-event effects. Tests separate longest-covering choice (with RFC9117 amendments), different-AFI/SAFI/RD routes, same-domain foreign-more-specific routes including a losing path, and removal-based reauthorization. Ignoring the losing foreign more-specific now incorrectly authorizes the route and fails the exact forbidden-candidate/install oracle; dropping route-domain comparison fails isolation while the valid matching-domain controls survive. This closes the specifically pending condition-c and route-domain producer counterfactuals without claiming that two faults exhaust all per-clause or dataplane behavior. The preserved broader assertions, not these two mutations alone, support the row. These are inspected scratch semantic executions, not canonical discrimination records. Current native record renewal/stamping remains parent-owned; no pending FRR carrier run, SRPM behavior, dataplane behavior, or full-RFC acceptance is inferred.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9117AuthorizationUsesLongestCoveringRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9117_best_match_test.go#L19) | unit/verify | revert, verified |
| negative | [`TestFlowSpecAuthorizationFromReceivedUpdates`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_flowspec_validation_test.go#L127) | unit/verify | revert, verified |
| negative | [`TestFlowSpecForeignMoreSpecificLosingPathInvalidatesRule`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_flowspec_validation_test.go#L415) | unit/verify | revert, verified |
| negative | [`TestFlowSpecVPNValidationSeparatesAFIAndRD`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_flowspec_validation_test.go#L291) | unit/verify | revert, verified |
| positive | [`TestRFC9117AuthorizationUsesLongestCoveringRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9117_best_match_test.go#L18) | unit/verify | revert, verified |
| positive | [`TestFlowSpecAuthorizationFromReceivedUpdates`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_flowspec_validation_test.go#L126) | unit/verify | revert, verified |
| positive | [`TestFlowSpecForeignMoreSpecificLosingPathInvalidatesRule`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_flowspec_validation_test.go#L414) | unit/verify | revert, verified |
| positive | [`TestFlowSpecVPNValidationSeparatesAFIAndRD`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_flowspec_validation_test.go#L290) | unit/verify | revert, verified |

### [`RFC8955-6-3`](#rfc8955-6-3)

Therefore, a revalidation of the Flow Specification NLRI MUST be performed whenever unicast routes change. (§6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. drives unicast add of a foreign more-specific, its withdraw, loss and return of the covering route, validation eligibility flips and session down, asserting a withdraw or reinstall event for the retained rule at each step without a new FlowSpec UPDATE

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFlowSpecUnicastLifecycleRevalidatesRetainedRule`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_flowspec_validation_test.go#L206) | unit/verify | unproven |
| positive | [`TestFlowSpecUnicastLifecycleRevalidatesRetainedRule`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_flowspec_validation_test.go#L205) | unit/verify | unproven |

### [`RFC8955-7.1-1`](#rfc8955-7.1-1)

On encoding, the traffic-rate MUST NOT be negative. (§7.1, §7.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. positive pins the exact IEEE 754 bits for byte and packet rates; negative asserts '-1 packets' is refused. The negative input is packets only, but both subtypes share the one guard in flowspec_encode.go (rate < 0). Test doc names parseFlowSpecTrafficRateBits in route_community.go, which does not exist

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestParseExtendedCommunitiesTrafficRatePackets`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/route/rfc8955_route_parse_test.go#L1240) | unit/verify | unproven |
| positive | [`TestParseExtendedCommunitiesTrafficRatePackets`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/route/rfc8955_route_parse_test.go#L1239) | unit/verify | unproven |

### [`RFC8955-7.1-2`](#rfc8955-7.1-2)

On decoding, negative values MUST be treated as zero (discard all traffic). (§7.1, §7.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. positive decodes 1000.0 to rate-limit:1000; negative decodes -1000.0 to rate-limit:0 for both subtype 0x06 and 0x0c

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8955TrafficRateNegativeDecodesAsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/cli/rfc8955_decode_test.go#L1473) | unit/verify | unproven |
| positive | [`TestRFC8955TrafficRateNegativeDecodesAsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/cli/rfc8955_decode_test.go#L1465) | unit/verify | unproven |

### [`RFC8955-7.3-1`](#rfc8955-7.3-1)

These bits MUST be set to 0 on encoding and MUST be ignored during decoding. (§7.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. encode: config actions leave octets 2-6 zero and only 0x03 in the last octet; decode: every reserved bit set renders the same as clear for all four S/T combinations, and a pair differing only in S changes the render, so a renderer reading no bit fails; the .ci repeats it end to end to a plugin

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8955TrafficActionUnusedBitsIgnoredOnDecode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/cli/rfc8955_decode_test.go#L1517) | unit/verify | unproven |
| negative | [`TestRFC8955TrafficActionBitsDecoded`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc8955_extcomm_decoded_test.go#L257) | unit/verify | unproven |
| negative | [`community-attributes-json.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/community-attributes-json.ci#L17) | functional/verify | unproven |
| positive | [`TestRFC8955TrafficActionUnusedBitsIgnoredOnDecode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/cli/rfc8955_decode_test.go#L1510) | unit/verify | unproven |
| positive | [`TestRFC8955TrafficActionUnusedBitsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/routeattr_test.go#L209) | unit/verify | unproven |
| positive | [`TestRFC8955TrafficActionBitsDecoded`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc8955_extcomm_decoded_test.go#L243) | unit/verify | unproven |
| positive | [`community-attributes-json.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/community-attributes-json.ci#L16) | functional/verify | unproven |

### [`RFC8955-7.5-1`](#rfc8955-7.5-1)

reserved (r): MUST be set to 0 on encoding and MUST be ignored during decoding (§7.5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. encode through config and EncodeRoute pins every reserved octet and the two reserved bits zero for DSCP 0, 46, 63, and refuses 64 and above; decode renders 0xEE as mark:46 and 0x40 as mark:0; the .ci carries the reserved-bit community to a plugin

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestParseExtendedCommunityMarkDSCPBound`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/rfc8955_routeattr_flowspec_test.go#L56) | unit/verify | unproven |
| negative | [`TestEncodeRouteMarkKeepsReservedBitsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/rfc8955_encode_test.go#L92) | unit/verify | unproven |
| negative | [`TestRFC8955TrafficMarkingReservedBitsIgnored`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc8955_extcomm_decoded_test.go#L216) | unit/verify | unproven |
| negative | [`community-attributes-json.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/community-attributes-json.ci#L15) | functional/verify | unproven |
| positive | [`TestParseExtendedCommunityMarkDSCPBound`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/rfc8955_routeattr_flowspec_test.go#L34) | unit/verify | unproven |
| positive | [`TestEncodeRouteEmitsZeroValuedActions`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/rfc8955_encode_test.go#L53) | unit/verify | unproven |
| positive | [`TestEncodeRouteMarkKeepsReservedBitsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/rfc8955_encode_test.go#L86) | unit/verify | unproven |
| positive | [`TestRFC8955TrafficMarkingReservedBitsIgnored`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc8955_extcomm_decoded_test.go#L209) | unit/verify | unproven |
| positive | [`community-attributes-json.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/community-attributes-json.ci#L14) | functional/verify | unproven |

### [`RFC8955-12-1`](#rfc8955-12-1)

Specifications relaxing the validation restrictions MUST contain security considerations that provide details on the required additional filtering. (§12)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8955-12-1, so no unit is bound to it.

### [`RFC8955-5.1-1`](#rfc8955-5.1-1)

This ordering function is such that it does not depend on the arrival order of the Flow Specification via BGP and thus is consistent in the network. (§5.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Compare is asserted antisymmetric (a<b, b>a, a==a) over type, prefix, operator-byte and width pairs, so the order is a function of the two NLRIs alone; the arrival-order claim in the negative tag rests on that, and the kernel arrival-order case sits under 5.1-2

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFlowSpecPrecedenceFromWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/compare_rfc8955_test.go#L10) | unit/verify | unproven |
| positive | [`TestFlowSpecPrecedenceFromWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/compare_rfc8955_test.go#L9) | unit/verify | unproven |

### [`RFC8955-5.1-2`](#rfc8955-5.1-2)

The relative order of two Flow Specifications is determined by comparing their respective components. The algorithm starts by comparing the left-most components (lowest component type value) of the Flow Specifications. If the types differ, the Flow Specification with lowest numeric type value has higher precedence (and thus will match before) than the Flow Specification that doesn't contain that component type. If the component types are the same, then a type- specific comparison is performed (see below). If the types are equal, the algorithm continues with the next component. For IP prefix values (IP destination or source prefix), if one of the two prefixes to compare is a more specific prefix of the other, the more specific prefix has higher precedence. Otherwise, the one with the lowest IP value has higher precedence. For all other component types, unless otherwise specified, the comparison is performed by comparing the component data as a binary string using the memcmp() function as defined by [ISO_IEC_9899]. For strings with equal lengths, the lowest string (memcmp) has higher precedence. For strings of different lengths, the common prefix is compared. If the common prefix is not equal, the string with the lowest prefix has higher precedence. If the common prefix is equal, the longest string is considered to have higher precedence than the shorter one. (§5.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RFC 8955 Section 5.1: "The relative order of two Flow Specifications is determined by comparing their respective components." Whole rule read through the concluding sentence: "If the common prefix is equal, the longest string is considered to have higher precedence than the shorter one." compare_rfc8955_test.go TestFlowSpecPrecedenceFromWire pins lower component type, absent/present next component after an equal first component, most-specific overlapping prefix, lower disjoint prefix, equal-width memcmp and received-width byte ordering rather than numeric canonicalization; every pair asserts forward/reverse signs and self equality. Compare walks increasing component type, uses prefix specificity/address ordering, then common-byte comparison and reverse length comparison. TestSelectedFlowSpecKernelPacketSemantics adds actual Linux packet evidence: the covering discard installed first blocks traffic, then the narrower terminal mark admits the same packet with DSCP 10; T-set again reaches the covering discard. These are separate conforming/control states, not two labels on one assertion. Build constraints require integration+linux with namespace/nft privileges; a skipped host/guest run is NOT runtime proof. Independent source rejudgment only; no test, mutation, build or gate was executed by this auditor. Native observed-red renewal is a separate parent step. Whole normative list (rfc/full/rfc8955.txt:840-863): "The relative order of two Flow Specifications is determined by comparing their respective components. The algorithm starts by comparing the left-most components (lowest component type value) of the Flow Specifications. If the types differ, the Flow Specification with lowest numeric type value has higher precedence (and thus will match before) than the Flow Specification that doesn't contain that component type. If the component types are the same, then a type-specific comparison is performed (see below). If the types are equal, the algorithm continues with the next component. For IP prefix values (IP destination or source prefix), if one of the two prefixes to compare is a more specific prefix of the other, the more specific prefix has higher precedence. Otherwise, the one with the lowest IP value has higher precedence. For all other component types, unless otherwise specified, the comparison is performed by comparing the component data as a binary string using the memcmp() function as defined by [ISO_IEC_9899]. For strings with equal lengths, the lowest string (memcmp) has higher precedence. For strings of different lengths, the common prefix is compared. If the common prefix is not equal, the string with the lowest prefix has higher precedence. If the common prefix is equal, the longest string is considered to have higher precedence than the shorter one."

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFlowSpecPrecedenceFromWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/compare_rfc8955_test.go#L12) | unit/verify | unproven |
| negative | [`TestSelectedFlowSpecKernelPacketSemantics`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowspec-firewall/rfc8955_selected_integration_linux_test.go#L24) | unit/verify | revert, verified |
| positive | [`TestFlowSpecPrecedenceFromWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/compare_rfc8955_test.go#L11) | unit/verify | unproven |
| positive | [`TestSelectedFlowSpecKernelPacketSemantics`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowspec-firewall/rfc8955_selected_integration_linux_test.go#L23) | unit/verify | revert, verified |

### [`RFC8955-8-1`](#rfc8955-8-1)

The NLRI format for this address family consists of a fixed-length Route Distinguisher field (8 octets) followed by the Flow Specification NLRI value (Section 4.2). (§8)

Audit verdict: enforced (the tests do what the requirement demands), fresh. positive decodes RD 100:100 from the 8 octets after the length and the destination component after them, and re-encodes the exact wire; negative refuses a length too short to hold the RD with ErrFlowSpecTruncated

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8955VPNNLRIStructure`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/rfc8955_vpn_test.go#L28) | unit/verify | revert, verified |
| positive | [`TestRFC8955VPNNLRIStructure`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/rfc8955_vpn_test.go#L27) | unit/verify | revert, verified |

### [`RFC8955-8-2`](#rfc8955-8-2)

The NLRI length field shall include both the 8 octets of the Route Distinguisher as well as the subsequent Flow Specification NLRI value. (§8)

Audit verdict: enforced (the tests do what the requirement demands), fresh. positive asserts Len and the encoded length octet count RD plus value (its first assertion checks the fixture against itself, the WriteTo one is the real check); negative refuses a length that counts only the value

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8955VPNLengthCoversRD`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/rfc8955_vpn_test.go#L57) | unit/verify | revert, verified |
| positive | [`TestRFC8955VPNLengthCoversRD`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/rfc8955_vpn_test.go#L56) | unit/verify | revert, verified |

### [`RFC8955-3-1`](#rfc8955-3-1)

Standard BGP policy mechanisms, such as UPDATE filtering by NLRI prefix as well as community matching, must apply to the Flow specification defined NLRI-type. (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Both halves now tagged both ways. Community half: TestRFC8955CommunityPolicyAcceptsAPermittedFlowSpecRoute (ipv4/flow and ipv4/flow-vpn with 65001:100 accepted) and TestRFC8955CommunityPolicyRejectsADeniedFlowSpecRoute (denied community first-match and implicit deny reject a FlowSpec route); revert records on evaluateCommunities. Prefix half: filter_prefix flowspec_test.go pair (accept, reject, missing, mixed), no record. Both at the plugin entry point: no .ci drives a FlowSpec UPDATE through a configured community filter in the running daemon.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8955CommunityPolicyRejectsADeniedFlowSpecRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/filter_community_match/rfc8955_flowspec_test.go#L39) | unit/verify | revert, verified |
| negative | [`TestPrefixPolicyFiltersFlowSpecDestination`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/filter_prefix/rfc8955_flowspec_test.go#L13) | unit/verify | unproven |
| positive | [`TestRFC8955CommunityPolicyAcceptsAPermittedFlowSpecRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/filter_community_match/rfc8955_flowspec_test.go#L24) | unit/verify | revert, verified |
| positive | [`TestPrefixPolicyFiltersFlowSpecDestination`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/filter_prefix/rfc8955_flowspec_test.go#L12) | unit/verify | unproven |

### [`RFC8955-6-5`](#rfc8955-6-5)

Although the forwarding attributes of two routes for the same Flow Specification prefix may be the same, BGP is still required to perform its path selection algorithm in order to select the correct set of attributes to advertise. (§6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. positive: two peers' paths for one FlowSpec NLRI publish the lower-MED path as best and hand best back to the survivor on withdraw; negative: a higher-MED second path publishes no best-change and the best keeps MED 100

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8955FlowSpecLosingPathIsNeverPublished`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8955_flowspec_bestchange_test.go#L159) | unit/verify | revert, verified |
| positive | [`TestRFC8955FlowSpecPathSelectionPicksOneSetOfAttributes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8955_flowspec_bestchange_test.go#L79) | unit/verify | revert, verified |

### [`RFC8955-7.3-2`](#rfc8955-7.3-2)

The use of the Terminal Action (bit 47) may result in more than one Flow Specification matching a particular traffic flow. All the Traffic Filtering Actions from these Flow Specifications shall be collected and applied. (§7.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RFC 8955 Section 7.3: "The use of the Terminal Action (bit 47) may result in more than one Flow Specification matching a particular traffic flow. All the Traffic Filtering Actions from these Flow Specifications shall be collected and applied." The same paragraph leaves interfering-action selection implementation-defined. TestSelectedFlowSpecKernelPacketSemantics now observes a received packet with the first rule's DSCP 10 AND exactly one increment of the later rule's sample counter; switching only the narrow rule to T-clear leaves the sample counter unchanged. Earlier controls prove T-set reaches a covering discard and that marking does not change the original DSCP used by later matching. rule_chains.go ruleChains/beforeMarkActions/markingTerms separate deferred marking and continuing action execution. Omitting the first action, the later action, or continuation fails independently, closing the earlier mark-plus-discard ambiguity. Requires a real privileged integration+linux execution; skips are not proof. Independent source rejudgment only; no test, mutation, build or gate was executed by this auditor. Native observed-red renewal is a separate parent step.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSelectedFlowSpecKernelPacketSemantics`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowspec-firewall/rfc8955_selected_integration_linux_test.go#L26) | unit/verify | revert, verified |
| positive | [`TestSelectedFlowSpecKernelPacketSemantics`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowspec-firewall/rfc8955_selected_integration_linux_test.go#L25) | unit/verify | revert, verified |

### [`RFC8955-12-2`](#rfc8955-12-2)

For a network to utilize this relaxation, the BGP policies must support additional filtering since the origin AS field is empty. (§12)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8955-12-2, so no unit is bound to it.

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-10-02 |
| Register | prose |
| Source | rfc/full/rfc8955.txt |
| Source fingerprint | ab2cc37046acae1d |
| Record | rfc/extraction/rfc8955.json |
| Mapped sentences | 25 |
| Declined as scope | 13 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 1 | walked | not stated |
| `1` | not stated | 3 | walked | not stated |
| `2` | not stated | 0 | walked | not stated |
| `3` | not stated | 1 | walked | not stated |
| `4` | not stated | 4 | walked | not stated |
| `4.1` | not stated | 0 | walked | not stated |
| `4.2` | not stated | 2 | walked | not stated |
| `4.2.1` | not stated | 0 | walked | not stated |
| `4.2.1.1` | not stated | 2 | walked | not stated |
| `4.2.1.2` | not stated | 1 | walked | not stated |
| `4.2.2` | not stated | 0 | walked | not stated |
| `4.2.2.1` | not stated | 0 | walked | not stated |
| `4.2.2.2` | not stated | 0 | walked | not stated |
| `4.2.2.3` | not stated | 0 | walked | not stated |
| `4.2.2.4` | not stated | 0 | walked | not stated |
| `4.2.2.5` | not stated | 0 | walked | not stated |
| `4.2.2.6` | not stated | 0 | walked | not stated |
| `4.2.2.7` | not stated | 0 | walked | not stated |
| `4.2.2.8` | not stated | 0 | walked | not stated |
| `4.2.2.9` | not stated | 1 | walked | not stated |
| `4.2.2.10` | not stated | 0 | walked | not stated |
| `4.2.2.11` | not stated | 1 | walked | not stated |
| `4.2.2.12` | not stated | 2 | walked | not stated |
| `4.3` | not stated | 0 | walked | not stated |
| `4.3.1` | not stated | 0 | walked | not stated |
| `4.3.2` | not stated | 0 | walked | not stated |
| `4.3.3` | not stated | 0 | walked | not stated |
| `5` | not stated | 0 | walked | not stated |
| `5.1` | not stated | 0 | walked | not stated |
| `6` | not stated | 5 | walked | not stated |
| `7` | not stated | 0 | walked | not stated |
| `7.1` | not stated | 2 | walked | not stated |
| `7.2` | not stated | 2 | walked | not stated |
| `7.3` | not stated | 2 | walked | not stated |
| `7.4` | not stated | 0 | walked | not stated |
| `7.5` | not stated | 1 | walked | not stated |
| `7.6` | not stated | 0 | walked | not stated |
| `7.7` | not stated | 0 | walked | not stated |
| `8` | not stated | 1 | walked | not stated |
| `9` | not stated | 0 | walked | not stated |
| `10` | not stated | 0 | walked | not stated |
| `11` | not stated | 0 | walked | not stated |
| `11.1` | not stated | 0 | walked | not stated |
| `11.2` | not stated | 3 | walked | not stated |
| `11.3` | not stated | 0 | walked | not stated |
| `12` | not stated | 4 | walked | not stated |
| `13` | not stated | 0 | walked | not stated |
| `13.1` | not stated | 0 | walked | not stated |
| `13.2` | not stated | 0 | walked | not stated |
| `A` | not stated | 0 | walked | not stated |
| `B` | not stated | 0 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `front:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | IETF Trust boilerplate on the Simplified BSD License for extracted Code Components; it states no protocol obligation. | Code Components extracted from this document must include Simplified BSD License text as described in Section 4.e of the Trust Legal Provisions and are provided without warranty as described in the Simplified BSD License. |
| `1:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Introduction prose describing what Section 7 of the document defines; "required" is lowercase and qualifies the Extended Communities the document specifies, not an obligation on an implementation. | Additionally, Section 7 of this document defines the required Traffic Filtering Actions BGP Extended Communities and mechanisms to use BGP for intra- and inter-provider distribution of traffic filtering rules in order to mitigate DoS and DDoS attacks. |
| `1:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Introduction prose listing possible applications of the extension; "required" is lowercase and describes the coordination those applications need. | Possible applications of that extension are: Automated inter-domain coordination of traffic filtering, such as what is required in order to mitigate DoS and DDoS attacks or traffic filtering in the context of a BGP/MPLS VPN service. |
| `1:3` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Introduction scope statement about which applications the specification addresses; "required" is lowercase and qualifies the protocol extensions the document defines. | This specification defines required protocol extensions to address most common applications of IPv4 unicast and VPNv4 unicast filtering. |
| `6:3` | `advisory-in-context` (never bound Ze): the sentence advises on applying a rule stated elsewhere and adds no obligation of its own | The capitalised keyword is the consequent of the MAY in the preceding sentence, which the splitter cut away: "However, rule a MAY be relaxed by explicit configuration, permitting Flow Specifications that include no destination prefix component. If such is the case, rules b and c are moot and MUST be disregarded." Row RFC8955-6-4 [MAY] carries the whole construction. | If such is the case, rules b and c are moot and MUST be disregarded. |
| `6:4` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | RFC 9117 Section 4.2, which updates RFC 8955, quotes this sentence and replaces it: 'this document also redefines the [RFC8955] AS_PATH validation procedure referenced above as follows: BGP Flow Specification implementations MUST enforce that the AS in the left-most position of the AS_PATH attribute of a Flow Specification route received via the External Border Gateway Protocol (eBGP) matches the AS in the left-most position of the AS_PATH attribute of the best-match unicast route'. The obligation in force is RFC 9117's, row RFC9117-4.2-1; RFC 9117 Section 7 makes the original rule optional again. Row RFC8955-6-2 was retired 2026-10-02 (rfc/corrections/rfc8955.md). | BGP implementations MUST also enforce that the AS_PATH attribute of a route received via the External Border Gateway Protocol (eBGP) contains the neighboring AS in the left-most position of the AS_PATH attribute. |
| `7.2:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates the Section 7.1 encoding obligation for traffic-rate-packets, which uses the same encoding; RFC8955-7.1-1 already carries it and cites both sections. | On encoding, the traffic-rate-packets MUST NOT be negative. |
| `7.2:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates the Section 7.1 decoding obligation for traffic-rate-packets, which uses the same encoding; RFC8955-7.1-2 already carries it and cites both sections. | On decoding, negative values MUST be treated as zero (discard all traffic). |
| `11.2:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | An IANA registration-policy table reproduced as text, not a sentence; it assigns code-point ranges to registration policies. | +==============+========================+ \| Type Values \| Policy \| +==============+========================+ \| 0 \| Reserved \| +--------------+------------------------+ \| [1 .. 127] \| Specification Required \| +--------------+------------------------+ \| [128 .. 254] \| Expert Review \| +--------------+------------------------+ \| 255 \| Reserved \| +--------------+------------------------+ |
| `11.2:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | The obligation binds the IANA designated experts who review Flow Spec Component Types registrations under Specification Required and Expert Review, not a BGP speaker: the sentence tells the experts to verify that a requesting specification reached the IDR Working Group. The producer is the IANA designated expert review process for the Flow Spec Component Types registry, which is a review process and not code; Ze operates no registry, requests no code point and holds no file that would act as this role if it did. | The experts must also verify that any specification produced in the IETF that requests one of these code points has been made available for review by the IDR Working Group and that any specification produced outside the IETF does not conflict with work that is active or already published within the IETF. |
| `11.2:3` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Rhetorical "it must be pointed out"; the sentence warns that new component types can break interoperability and places no obligation on an implementation. | It must be pointed out that introducing new component types may break interoperability with existing implementations of this protocol. |
| `12:3` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Security Considerations exhortation ("additional care must be taken") with no action named; the obligation it introduces is RFC8955-12-2. | Since the validation of Flow Specification (Section 6) depends on this, additional care must be taken. |
| `12:4` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Statement of fact about what systems can and cannot locate in a packet header; it places no obligation. | Systems may not be able to locate all header values required to identify a packet. |

## Superseded

No document obsoletes RFC 8955, so its obligations are stated where they were written.
