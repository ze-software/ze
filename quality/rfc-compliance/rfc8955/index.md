# RFC 8955 - Dissemination of Flow Specification Rules

Partial. Every requirement this repository extracted from RFC 8955, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 73.3% | 22 of 30 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 20.0% | 6 of 30 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 30 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 0.0% | 0 of 30 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 13.0% | 9 of 69 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 30 | of 45 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 2 | of 30 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 6.7% | 2 of 30 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 30 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 30 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

The 7 shares marked as a part above are the whole of the 30 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 45 |
| Gated MUST-level | 30 |
| Not applicable, so out of scope | 2 |
| Declared gaps | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 69 |
| Tagged units | 69 |
| Recorded audit verdicts | 0 |
| Discrimination records | 9 |
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
| Positive and negative tests | 22 | one part of the gated population |
| Annotated instead of tested | 8 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **30** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (22):** [`RFC8955-4-3`](#rfc8955-4-3), [`RFC8955-4-4`](#rfc8955-4-4), [`RFC8955-4.2-1`](#rfc8955-4.2-1), [`RFC8955-4.2-2`](#rfc8955-4.2-2), [`RFC8955-4.2.1.1-2`](#rfc8955-4.2.1.1-2), [`RFC8955-4.2.1.1-3`](#rfc8955-4.2.1.1-3), [`RFC8955-4.2.1.2-1`](#rfc8955-4.2.1.2-1), [`RFC8955-4.2.2.12-2`](#rfc8955-4.2.2.12-2), [`RFC8955-6-1`](#rfc8955-6-1), [`RFC8955-6-2`](#rfc8955-6-2), [`RFC8955-6-3`](#rfc8955-6-3), [`RFC8955-7.1-1`](#rfc8955-7.1-1), [`RFC8955-7.1-2`](#rfc8955-7.1-2), [`RFC8955-7.3-1`](#rfc8955-7.3-1), [`RFC8955-7.5-1`](#rfc8955-7.5-1), [`RFC8955-5.1-1`](#rfc8955-5.1-1), [`RFC8955-5.1-2`](#rfc8955-5.1-2), [`RFC8955-8-1`](#rfc8955-8-1), [`RFC8955-8-2`](#rfc8955-8-2), [`RFC8955-3-1`](#rfc8955-3-1), [`RFC8955-6-5`](#rfc8955-6-5), [`RFC8955-7.3-2`](#rfc8955-7.3-2)

**Annotated instead of tested (8):** [`RFC8955-4-1`](#rfc8955-4-1), [`RFC8955-4-2`](#rfc8955-4-2), [`RFC8955-4.2.1.1-1`](#rfc8955-4.2.1.1-1), [`RFC8955-4.2.2.9-1`](#rfc8955-4.2.2.9-1), [`RFC8955-4.2.2.11-1`](#rfc8955-4.2.2.11-1), [`RFC8955-4.2.2.12-1`](#rfc8955-4.2.2.12-1), [`RFC8955-12-1`](#rfc8955-12-1), [`RFC8955-12-2`](#rfc8955-12-2)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC8955-4-1` | Implementations wishing to exchange Flow Specification MUST use BGP's Capability Advertisement facility to exchange the Multiprotocol Extension Capability Code (Code 1) (§4) | MUST | 4 | **positive:** `unit/verify` [`TestIPv4FlowSpecNegotiatesMultiprotocolCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/plugin_test.go#L846). **negative:** no negative test. **{single-polarity}:** the flowspec plugin unconditionally maps each declared FlowSpec family to a Multiprotocol (Code 1) capability during OPEN, so there is no wrong input the negotiation path rejects (internal/component/bgp/plugins/nlri/flowspec/register.go, types.go:47) |
| `RFC8955-4-2` | The (AFI, SAFI) pair carried in the Multiprotocol Extension Capability MUST be (AFI=1, SAFI=133) for IPv4 Flow Specification and (AFI=1, SAFI=134) for VPNv4 Flow Specification (§4) | MUST | 4 | **positive:** `unit/verify` [`TestFlowSpecVPNFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L647). **positive:** `unit/verify` [`TestIPv4FlowSpecNegotiatesMultiprotocolCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/plugin_test.go#L847). **negative:** no negative test. **{single-polarity}:** the (AFI 1, SAFI 133) and (AFI 1, SAFI 134) pairs are family-registration constants, not an input guard, so only the positive assignment is assertable (internal/component/bgp/plugins/nlri/flowspec/types.go:47-50) |
| `RFC8955-4-3` | Length of the Next-Hop Network Address MUST be set to 0 (§4) | MUST | 4 | **positive:** `unit/verify` [`TestFlowSpecUpdateIgnoresConfiguredNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/flowspec_wire_contract_test.go#L14). **negative:** `unit/verify` [`TestFlowSpecUpdateIgnoresConfiguredNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/flowspec_wire_contract_test.go#L15). **positive:** `functional/verify` [`flow-encode.ci`](https://github.com/ze-software/ze/blob/main/test/encode/flow-encode.ci#L11) |
| `RFC8955-4-4` | Network Address of the Next-Hop field MUST be ignored (§4) | MUST | 4 | **positive:** `unit/verify` [`TestRFC8955NextHopIgnoredForFlowSpec`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_flowspec_validation_test.go#L477). **negative:** `unit/verify` [`TestRFC8955NextHopIgnoredForFlowSpec`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_flowspec_validation_test.go#L478) |
| `RFC8955-4.2-1` | Components MUST follow strict type ordering by increasing numerical order (§4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestFlowSpecComponentsAscendingOrder`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1228). **positive:** `unit/verify` [`TestFlowSpecJoinsRepeatedTypeIntoOneComponent`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1270). **negative:** `unit/verify` [`TestAddComponentRefusesASecondPrefix`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1387). **negative:** `unit/verify` [`TestParseFlowSpecRefusesRepeatedComponentType`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1307). **negative:** `unit/verify` [`TestParseFlowSpecVPNRefusesRepeatedComponentType`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1367) |
| `RFC8955-4.2-2` | If present, it MUST precede any component of higher numeric type value. (§4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestFlowSpecComponentsAscendingOrder`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1229). **negative:** `unit/verify` [`TestParseFlowSpecRefusesDescendingComponentType`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1335) |
| `RFC8955-4.2.1.1-1` | In the first operator octet of a sequence, the AND bit MUST be encoded as unset (§4.2.1.1) | MUST | 4.2.1.1 | **positive:** `unit/verify` [`TestRFC8955FirstOperatorAndBitEncodedUnset`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1542). **negative:** no negative test. **{single-polarity}:** parseFlowMatches derives the AND bit purely from the position inside a '&'-joined expression (isAnd := i > 0), so the first operator-value pair is always encoded with the AND bit clear and no input sets it (internal/component/bgp/plugins/nlri/flowspec/config_builder.go:220,:252) |
| `RFC8955-4.2.1.1-2` | First operator AND bit MUST be treated as always unset on decoding (§4.2.1.1) | MUST | 4.2.1.1 | **positive:** `unit/verify` [`TestRFC8955FirstOperatorAndBitTreatedUnsetOnDecode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1573). **negative:** `unit/verify` [`TestRFC8955FirstOperatorAndBitTreatedUnsetOnDecode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1577) |
| `RFC8955-4.2.1.1-3` | Numeric operator reserved bit (bit 4) MUST be set to 0 on NLRI encoding and MUST be ignored during decoding (§4.2.1.1) | MUST | 4.2.1.1 | **positive:** `unit/verify` [`TestFlowSpecNumericReservedAndEightOctetOperand`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/compare_rfc8955_test.go#L42). **negative:** `unit/verify` [`TestFlowSpecNumericReservedAndEightOctetOperand`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/compare_rfc8955_test.go#L43) |
| `RFC8955-4.2.1.2-1` | Bitmask operator reserved bits (bits 4-5) MUST be set to 0 on NLRI encoding and MUST be ignored during decoding (§4.2.1.2) | MUST | 4.2.1.2 | **positive:** `unit/verify` [`TestFlowSpecBitmaskOperatorReservedBitsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1450). **negative:** `unit/verify` [`TestRFC8955BitmaskOperatorReservedBitsIgnoredOnDecode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1611) |
| `RFC8955-4.2.2.9-1` | Type 9 component bitmasks MUST be encoded as 1- or 2-octet bitmask (bitmask_op len=00 or len=01). (§4.2.2.9) | MUST | 4.2.2.9 | **positive:** `unit/verify` [`TestRFC8955TCPFlagsBitmaskSingleOctet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1671). **negative:** no negative test. **{single-polarity}:** parseFlowTCPFlagMatches resolves flag names to 8-bit values and numericComponent.Bytes() selects the 1-octet length code for any value <= 0xFF, so an emitted Type-9 bitmask is always 1 octet and no over-long bitmask can be produced to reject (internal/component/bgp/plugins/nlri/flowspec/config_builder.go:414-446, types_numeric.go:47-55) |
| `RFC8955-4.2.2.11-1` | Type 11 component values MUST be encoded as single octet (numeric_op len=00). (§4.2.2.11) | MUST | 4.2.2.11 | **positive:** `unit/verify` [`TestRFC8955DSCPValueSingleOctet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1699). **negative:** no negative test. **{single-polarity}:** parseFlowOctets parses DSCP values as uint8 and NewFlowDSCPComponent stores them, so numericComponent.Bytes() always selects the 1-octet length code and no multi-octet DSCP encoding exists to reject (internal/component/bgp/plugins/nlri/flowspec/config_builder.go:261-273, types_numeric.go:473-479) |
| `RFC8955-4.2.2.12-1` | The Type 12 component bitmask MUST be encoded as single octet bitmask (bitmask_op len=00). (§4.2.2.12) | MUST | 4.2.2.12 | **positive:** `unit/verify` [`TestRFC8955FragmentBitmaskSingleOctet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1726). **negative:** no negative test. **{single-polarity}:** fragment values come from the four low-nibble FlowFragmentFlag constants, so numericComponent.Bytes() always selects the 1-octet length code and no multi-octet fragment bitmask can be produced to reject (internal/component/bgp/plugins/nlri/flowspec/types.go:201-206, types_numeric.go:481-491) |
| `RFC8955-4.2.2.12-2` | 0: MUST be set to 0 on NLRI encoding and MUST be ignored during decoding (§4.2.2.12) | MUST | 4.2.2.12 | **positive:** `unit/verify` [`TestFlowSpecFragmentReservedHighNibbleZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1479). **negative:** `unit/verify` [`TestRFC8955FragmentReservedBitsIgnoredOnDecode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1644) |
| `RFC8955-6-1` | Flow Specification NLRI MUST be validated such that it is considered feasible if and only if all validation conditions are true (§6, updated by RFC 9117 §4.1) | MUST | 6 | **positive:** `unit/verify` [`TestFlowSpecAuthorizationFromReceivedUpdates`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_flowspec_validation_test.go#L126). **negative:** `unit/verify` [`TestFlowSpecAuthorizationFromReceivedUpdates`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_flowspec_validation_test.go#L127) |
| `RFC8955-6-2` | The leftmost AS_SEQUENCE ASN of a Flow Specification route received via eBGP MUST match the leftmost AS_SEQUENCE ASN of the best-match unicast route for its destination prefix (§6, replaced by RFC 9117 §4.2) | MUST | 6 | **positive:** `unit/verify` [`TestFlowSpecAuthorizationFromReceivedUpdates`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_flowspec_validation_test.go#L128). **negative:** `unit/verify` [`TestFlowSpecAuthorizationFromReceivedUpdates`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_flowspec_validation_test.go#L129) |
| `RFC8955-6-3` | Therefore, a revalidation of the Flow Specification NLRI MUST be performed whenever unicast routes change. (§6) | MUST | 6 | **positive:** `unit/verify` [`TestFlowSpecUnicastLifecycleRevalidatesRetainedRule`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_flowspec_validation_test.go#L203). **negative:** `unit/verify` [`TestFlowSpecUnicastLifecycleRevalidatesRetainedRule`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_flowspec_validation_test.go#L204) |
| `RFC8955-7.1-1` | On encoding, the traffic-rate MUST NOT be negative. (§7.1, §7.2) | MUST NOT | 7.1 | **positive:** `unit/verify` [`TestParseExtendedCommunitiesTrafficRatePackets`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/route/route_parse_test.go#L1239). **negative:** `unit/verify` [`TestParseExtendedCommunitiesTrafficRatePackets`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/route/route_parse_test.go#L1240) |
| `RFC8955-7.1-2` | On decoding, negative values MUST be treated as zero (discard all traffic). (§7.1, §7.2) | MUST | 7.1 | **positive:** `unit/verify` [`TestRFC8955TrafficRateNegativeDecodesAsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/cli/decode_test.go#L1465). **negative:** `unit/verify` [`TestRFC8955TrafficRateNegativeDecodesAsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/cli/decode_test.go#L1473) |
| `RFC8955-7.3-1` | These bits MUST be set to 0 on encoding and MUST be ignored during decoding. (§7.3) | MUST | 7.3 | **positive:** `unit/verify` [`TestRFC8955TrafficActionBitsDecoded`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/extcomm_decoded_test.go#L243). **positive:** `unit/verify` [`TestRFC8955TrafficActionUnusedBitsIgnoredOnDecode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/cli/decode_test.go#L1510). **positive:** `unit/verify` [`TestRFC8955TrafficActionUnusedBitsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/routeattr_test.go#L209). **negative:** `unit/verify` [`TestRFC8955TrafficActionBitsDecoded`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/extcomm_decoded_test.go#L257). **negative:** `unit/verify` [`TestRFC8955TrafficActionUnusedBitsIgnoredOnDecode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/cli/decode_test.go#L1517). **positive:** `functional/verify` [`community-attributes-json.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/community-attributes-json.ci#L16). **negative:** `functional/verify` [`community-attributes-json.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/community-attributes-json.ci#L17) |
| `RFC8955-7.5-1` | reserved (r): MUST be set to 0 on encoding and MUST be ignored during decoding (§7.5) | MUST | 7.5 | **positive:** `unit/verify` [`TestEncodeRouteEmitsZeroValuedActions`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/encode_test.go#L53). **positive:** `unit/verify` [`TestEncodeRouteMarkKeepsReservedBitsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/encode_test.go#L86). **positive:** `unit/verify` [`TestParseExtendedCommunityMarkDSCPBound`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/routeattr_flowspec_test.go#L34). **positive:** `unit/verify` [`TestRFC8955TrafficMarkingReservedBitsIgnored`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/extcomm_decoded_test.go#L209). **negative:** `unit/verify` [`TestEncodeRouteMarkKeepsReservedBitsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/encode_test.go#L92). **negative:** `unit/verify` [`TestParseExtendedCommunityMarkDSCPBound`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/routeattr_flowspec_test.go#L56). **negative:** `unit/verify` [`TestRFC8955TrafficMarkingReservedBitsIgnored`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/extcomm_decoded_test.go#L216). **positive:** `functional/verify` [`community-attributes-json.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/community-attributes-json.ci#L14). **negative:** `functional/verify` [`community-attributes-json.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/community-attributes-json.ci#L15) |
| `RFC8955-12-1` | Specifications relaxing the validation restrictions MUST contain security considerations that provide details on the required additional filtering. (§12) | MUST | 12 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze publishes no specification introducing a validation relaxation. The implemented RFC 9117 rules are standards-defined; flowSpecAuthorized in internal/component/bgp/plugins/rib/rib_flowspec_validation.go still requires a valid destination and covering unicast reachability, with no configurable destination bypass |
| `RFC8955-5.1-1` | "This ordering function is such that it does not depend on the arrival order of the Flow Specification via BGP and thus is consistent in the network" (§5.1) | MUST | 5.1 | **positive:** `unit/verify` [`TestFlowSpecPrecedenceFromWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/compare_rfc8955_test.go#L9). **negative:** `unit/verify` [`TestFlowSpecPrecedenceFromWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/compare_rfc8955_test.go#L10) |
| `RFC8955-5.1-2` | The relative order of two Flow Specifications is determined by comparing their components from the left-most (lowest component type value): the Flow Specification with the lowest numeric type value has higher precedence; for IP destination or source prefix values the more specific prefix has higher precedence and otherwise the lowest IP value does; for all other component types the data is compared as a binary string with memcmp(), the lowest string wins at equal lengths, and at different lengths the common prefix decides with the longest string winning when that prefix is equal (§5.1) | MUST | 5.1 | **positive:** `unit/verify` [`TestFlowSpecPrecedenceFromWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/compare_rfc8955_test.go#L11). **positive:** `unit/verify` [`TestSelectedFlowSpecKernelPacketSemantics`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowspec-firewall/selected_integration_linux_test.go#L23). **negative:** `unit/verify` [`TestFlowSpecPrecedenceFromWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/compare_rfc8955_test.go#L12). **negative:** `unit/verify` [`TestSelectedFlowSpecKernelPacketSemantics`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowspec-firewall/selected_integration_linux_test.go#L24) |
| `RFC8955-8-1` | The VPNv4 Flow Specification NLRI "consists of a fixed-length Route Distinguisher field (8 octets) followed by the Flow Specification NLRI value (Section 4.2)" (§8) | MUST | 8 | **positive:** `unit/verify` [`TestRFC8955VPNNLRIStructure`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/vpn_rfc8955_test.go#L27). **negative:** `unit/verify` [`TestRFC8955VPNNLRIStructure`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/vpn_rfc8955_test.go#L28) |
| `RFC8955-8-2` | The NLRI length field shall include both the 8 octets of the Route Distinguisher as well as the subsequent Flow Specification NLRI value. (§8) | MUST | 8 | **positive:** `unit/verify` [`TestRFC8955VPNLengthCoversRD`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/vpn_rfc8955_test.go#L56). **negative:** `unit/verify` [`TestRFC8955VPNLengthCoversRD`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/vpn_rfc8955_test.go#L57) |
| `RFC8955-3-1` | Standard BGP policy mechanisms, such as UPDATE filtering by NLRI prefix as well as community matching, must apply to the Flow specification defined NLRI-type. (§3) | MUST | 3 | **positive:** `unit/verify` [`TestPrefixPolicyFiltersFlowSpecDestination`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/filter_prefix/flowspec_test.go#L12). **negative:** `unit/verify` [`TestPrefixPolicyFiltersFlowSpecDestination`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/filter_prefix/flowspec_test.go#L13) |
| `RFC8955-6-5` | Although the forwarding attributes of two routes for the same Flow Specification prefix may be the same, BGP is still required to perform its path selection algorithm in order to select the correct set of attributes to advertise. (§6) | MUST | 6 | **positive:** `unit/verify` [`TestRFC8955FlowSpecPathSelectionPicksOneSetOfAttributes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8955_flowspec_bestchange_test.go#L79). **negative:** `unit/verify` [`TestRFC8955FlowSpecLosingPathIsNeverPublished`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8955_flowspec_bestchange_test.go#L159) |
| `RFC8955-7.3-2` | Where the Terminal Action bit is set and the evaluation continues to the next Flow Specification, "all the Traffic Filtering Actions from these Flow Specifications shall be collected and applied" (§7.3) | MUST | 7.3 | **positive:** `unit/verify` [`TestSelectedFlowSpecKernelPacketSemantics`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowspec-firewall/selected_integration_linux_test.go#L25). **negative:** `unit/verify` [`TestSelectedFlowSpecKernelPacketSemantics`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowspec-firewall/selected_integration_linux_test.go#L26) |
| `RFC8955-12-2` | Where the rule-a validation relaxation is used, "for a network to utilize this relaxation, the BGP policies must support additional filtering since the origin AS field is empty" (§12) | MUST | 12 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** conditional on an absent feature. flowSpecDestination and flowSpecAuthorized in internal/component/bgp/plugins/rib/rib_flowspec_validation.go require a valid destination, including offset zero for IPv6, and no configuration bypasses that guard. The owner explicitly retained strict destination validation |
| `RFC8955-4.2.2.3-1` | Type 3 (IP Protocol) values SHOULD be encoded as single octet (§4.2.2.3) | SHOULD | 4.2.2.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC8955-4.2.2.4-1` | Type 4-6, 10 (Port, Dst Port, Src Port, Packet Length) values SHOULD be encoded as 1- or 2-octet quantities (§4.2.2.4-6, §4.2.2.10) | SHOULD | 4.2.2.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC8955-4.2.2.7-1` | Type 7-8 (ICMP Type, ICMP Code) values SHOULD be encoded as single octet (§4.2.2.7-8) | SHOULD | 4.2.2.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC8955-4.2.2.11-2` | DSCP extra bits SHOULD be treated as 0 (§4.2.2.11) | SHOULD | 4.2.2.11 | **positive:** no positive test. **negative:** no negative test |
| `RFC8955-7-1` | Multiple Traffic Filtering Actions present for a single Flow Specification SHOULD be applied to the traffic flow (§7) | SHOULD | 7 | **positive:** no positive test. **negative:** no negative test |
| `RFC8955-7.1-3` | The 2-octet AS id in traffic-rate-bytes/packets is purely informational and SHOULD NOT be interpreted by the implementation (§7.1) | SHOULD NOT | 7.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8955-4.2-3` | Impossible combinations (e.g., ICMP Type AND Port) SHOULD NOT be propagated by BGP (§4.2) | SHOULD NOT | 4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC8955-9-1` | Implementations SHOULD provide a mechanism to log the packet header of filtered traffic (§9) | SHOULD | 9 | **positive:** no positive test. **negative:** no negative test |
| `RFC8955-9-2` | Implementations SHOULD provide a mechanism to count the number of matches for a given Flow Specification rule (§9) | SHOULD | 9 | **positive:** no positive test. **negative:** no negative test |
| `RFC8955-7.7-1` | Implementors SHOULD document the behavior of their implementation for interfering Traffic Filtering Actions (§7.7) | SHOULD | 7.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC8955-7-2` | "Any additional definition of Traffic Filtering Actions SHOULD specify the action to take if those Traffic Filtering Actions interfere (also with existing Traffic Filtering Actions)" (§7) | SHOULD | 7 | **positive:** no positive test. **negative:** no negative test |
| `RFC8955-7.1-4` | "A traffic-rate of 0 should result on all traffic for the particular flow to be discarded", and "a traffic-rate-packets of 0 should result in all traffic for the particular flow to be discarded" (§7.1, §7.2) | SHOULD | 7.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8955-7.6-1` | "Implementations should provide mechanisms that map an arbitrary BGP community value (normal or extended) to Traffic Filtering Actions that require different mappings on different systems in the network" (§7.6) | SHOULD | 7.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC8955-6-4` | Rule a (destination prefix requirement) MAY be relaxed by explicit configuration; if so, rules b and c MUST be disregarded (§6) | MAY | 6 | **positive:** no positive test. **negative:** no negative test |
| `RFC8955-4.2-4` | A given component type MAY appear exactly once (§4.2) | MAY | 4.2 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC8955-12-1`](#rfc8955-12-1) Specifications relaxing the validation restrictions MUST contain security considerations that provide details on the required additional filtering. (§12) | no test | no test carries this requirement id; annotated {not-applicable}: ze publishes no specification introducing a validation relaxation. The implemented RFC 9117 rules are standards-defined; flowSpecAuthorized in internal/component/bgp/plugins/rib/rib_flowspec_validation.go still requires a valid destination and covering unicast reachability, with no configurable destination bypass |
| [`RFC8955-12-2`](#rfc8955-12-2) Where the rule-a validation relaxation is used, "for a network to utilize this relaxation, the BGP policies must support additional filtering since the origin AS field is empty" (§12) | no test | no test carries this requirement id; annotated {not-applicable}: conditional on an absent feature. flowSpecDestination and flowSpecAuthorized in internal/component/bgp/plugins/rib/rib_flowspec_validation.go require a valid destination, including offset zero for IPv6, and no configuration bypasses that guard. The owner explicitly retained strict destination validation |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC8955-4-1`](#rfc8955-4-1)

Implementations wishing to exchange Flow Specification MUST use BGP's Capability Advertisement facility to exchange the Multiprotocol Extension Capability Code (Code 1) (§4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestIPv4FlowSpecNegotiatesMultiprotocolCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/plugin_test.go#L846) | unit/verify | unproven |

### [`RFC8955-4-2`](#rfc8955-4-2)

The (AFI, SAFI) pair carried in the Multiprotocol Extension Capability MUST be (AFI=1, SAFI=133) for IPv4 Flow Specification and (AFI=1, SAFI=134) for VPNv4 Flow Specification (§4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestIPv4FlowSpecNegotiatesMultiprotocolCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/plugin_test.go#L847) | unit/verify | unproven |
| positive | [`TestFlowSpecVPNFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L647) | unit/verify | unproven |

### [`RFC8955-4-3`](#rfc8955-4-3)

Length of the Next-Hop Network Address MUST be set to 0 (§4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFlowSpecUpdateIgnoresConfiguredNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/flowspec_wire_contract_test.go#L15) | unit/verify | unproven |
| positive | [`TestFlowSpecUpdateIgnoresConfiguredNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/flowspec_wire_contract_test.go#L14) | unit/verify | unproven |
| positive | [`flow-encode.ci`](https://github.com/ze-software/ze/blob/main/test/encode/flow-encode.ci#L11) | functional/verify | revert, verified |

### [`RFC8955-4-4`](#rfc8955-4-4)

Network Address of the Next-Hop field MUST be ignored (§4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8955NextHopIgnoredForFlowSpec`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_flowspec_validation_test.go#L478) | unit/verify | revert, verified |
| positive | [`TestRFC8955NextHopIgnoredForFlowSpec`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_flowspec_validation_test.go#L477) | unit/verify | revert, verified |

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

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestParseFlowSpecRefusesDescendingComponentType`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1335) | unit/verify | unproven |
| positive | [`TestFlowSpecComponentsAscendingOrder`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1229) | unit/verify | unproven |

### [`RFC8955-4.2.1.1-1`](#rfc8955-4.2.1.1-1)

In the first operator octet of a sequence, the AND bit MUST be encoded as unset (§4.2.1.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC8955FirstOperatorAndBitEncodedUnset`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1542) | unit/verify | unproven |

### [`RFC8955-4.2.1.1-2`](#rfc8955-4.2.1.1-2)

First operator AND bit MUST be treated as always unset on decoding (§4.2.1.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8955FirstOperatorAndBitTreatedUnsetOnDecode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1577) | unit/verify | unproven |
| positive | [`TestRFC8955FirstOperatorAndBitTreatedUnsetOnDecode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1573) | unit/verify | unproven |

### [`RFC8955-4.2.1.1-3`](#rfc8955-4.2.1.1-3)

Numeric operator reserved bit (bit 4) MUST be set to 0 on NLRI encoding and MUST be ignored during decoding (§4.2.1.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFlowSpecNumericReservedAndEightOctetOperand`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/compare_rfc8955_test.go#L43) | unit/verify | unproven |
| positive | [`TestFlowSpecNumericReservedAndEightOctetOperand`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/compare_rfc8955_test.go#L42) | unit/verify | unproven |

### [`RFC8955-4.2.1.2-1`](#rfc8955-4.2.1.2-1)

Bitmask operator reserved bits (bits 4-5) MUST be set to 0 on NLRI encoding and MUST be ignored during decoding (§4.2.1.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8955BitmaskOperatorReservedBitsIgnoredOnDecode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1611) | unit/verify | unproven |
| positive | [`TestFlowSpecBitmaskOperatorReservedBitsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1450) | unit/verify | unproven |

### [`RFC8955-4.2.2.9-1`](#rfc8955-4.2.2.9-1)

Type 9 component bitmasks MUST be encoded as 1- or 2-octet bitmask (bitmask_op len=00 or len=01). (§4.2.2.9)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC8955TCPFlagsBitmaskSingleOctet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1671) | unit/verify | unproven |

### [`RFC8955-4.2.2.11-1`](#rfc8955-4.2.2.11-1)

Type 11 component values MUST be encoded as single octet (numeric_op len=00). (§4.2.2.11)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC8955DSCPValueSingleOctet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1699) | unit/verify | unproven |

### [`RFC8955-4.2.2.12-1`](#rfc8955-4.2.2.12-1)

The Type 12 component bitmask MUST be encoded as single octet bitmask (bitmask_op len=00). (§4.2.2.12)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC8955FragmentBitmaskSingleOctet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1726) | unit/verify | unproven |

### [`RFC8955-4.2.2.12-2`](#rfc8955-4.2.2.12-2)

0: MUST be set to 0 on NLRI encoding and MUST be ignored during decoding (§4.2.2.12)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8955FragmentReservedBitsIgnoredOnDecode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1644) | unit/verify | unproven |
| positive | [`TestFlowSpecFragmentReservedHighNibbleZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1479) | unit/verify | unproven |

### [`RFC8955-6-1`](#rfc8955-6-1)

Flow Specification NLRI MUST be validated such that it is considered feasible if and only if all validation conditions are true (§6, updated by RFC 9117 §4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFlowSpecAuthorizationFromReceivedUpdates`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_flowspec_validation_test.go#L127) | unit/verify | unproven |
| positive | [`TestFlowSpecAuthorizationFromReceivedUpdates`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_flowspec_validation_test.go#L126) | unit/verify | unproven |

### [`RFC8955-6-2`](#rfc8955-6-2)

The leftmost AS_SEQUENCE ASN of a Flow Specification route received via eBGP MUST match the leftmost AS_SEQUENCE ASN of the best-match unicast route for its destination prefix (§6, replaced by RFC 9117 §4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFlowSpecAuthorizationFromReceivedUpdates`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_flowspec_validation_test.go#L129) | unit/verify | unproven |
| positive | [`TestFlowSpecAuthorizationFromReceivedUpdates`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_flowspec_validation_test.go#L128) | unit/verify | unproven |

### [`RFC8955-6-3`](#rfc8955-6-3)

Therefore, a revalidation of the Flow Specification NLRI MUST be performed whenever unicast routes change. (§6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFlowSpecUnicastLifecycleRevalidatesRetainedRule`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_flowspec_validation_test.go#L204) | unit/verify | unproven |
| positive | [`TestFlowSpecUnicastLifecycleRevalidatesRetainedRule`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_flowspec_validation_test.go#L203) | unit/verify | unproven |

### [`RFC8955-7.1-1`](#rfc8955-7.1-1)

On encoding, the traffic-rate MUST NOT be negative. (§7.1, §7.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestParseExtendedCommunitiesTrafficRatePackets`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/route/route_parse_test.go#L1240) | unit/verify | unproven |
| positive | [`TestParseExtendedCommunitiesTrafficRatePackets`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/route/route_parse_test.go#L1239) | unit/verify | unproven |

### [`RFC8955-7.1-2`](#rfc8955-7.1-2)

On decoding, negative values MUST be treated as zero (discard all traffic). (§7.1, §7.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8955TrafficRateNegativeDecodesAsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/cli/decode_test.go#L1473) | unit/verify | unproven |
| positive | [`TestRFC8955TrafficRateNegativeDecodesAsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/cli/decode_test.go#L1465) | unit/verify | unproven |

### [`RFC8955-7.3-1`](#rfc8955-7.3-1)

These bits MUST be set to 0 on encoding and MUST be ignored during decoding. (§7.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8955TrafficActionUnusedBitsIgnoredOnDecode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/cli/decode_test.go#L1517) | unit/verify | unproven |
| negative | [`TestRFC8955TrafficActionBitsDecoded`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/extcomm_decoded_test.go#L257) | unit/verify | unproven |
| negative | [`community-attributes-json.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/community-attributes-json.ci#L17) | functional/verify | unproven |
| positive | [`TestRFC8955TrafficActionUnusedBitsIgnoredOnDecode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/cli/decode_test.go#L1510) | unit/verify | unproven |
| positive | [`TestRFC8955TrafficActionUnusedBitsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/routeattr_test.go#L209) | unit/verify | unproven |
| positive | [`TestRFC8955TrafficActionBitsDecoded`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/extcomm_decoded_test.go#L243) | unit/verify | unproven |
| positive | [`community-attributes-json.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/community-attributes-json.ci#L16) | functional/verify | unproven |

### [`RFC8955-7.5-1`](#rfc8955-7.5-1)

reserved (r): MUST be set to 0 on encoding and MUST be ignored during decoding (§7.5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestParseExtendedCommunityMarkDSCPBound`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/routeattr_flowspec_test.go#L56) | unit/verify | unproven |
| negative | [`TestEncodeRouteMarkKeepsReservedBitsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/encode_test.go#L92) | unit/verify | unproven |
| negative | [`TestRFC8955TrafficMarkingReservedBitsIgnored`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/extcomm_decoded_test.go#L216) | unit/verify | unproven |
| negative | [`community-attributes-json.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/community-attributes-json.ci#L15) | functional/verify | unproven |
| positive | [`TestParseExtendedCommunityMarkDSCPBound`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/routeattr_flowspec_test.go#L34) | unit/verify | unproven |
| positive | [`TestEncodeRouteEmitsZeroValuedActions`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/encode_test.go#L53) | unit/verify | unproven |
| positive | [`TestEncodeRouteMarkKeepsReservedBitsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/encode_test.go#L86) | unit/verify | unproven |
| positive | [`TestRFC8955TrafficMarkingReservedBitsIgnored`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/extcomm_decoded_test.go#L209) | unit/verify | unproven |
| positive | [`community-attributes-json.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/community-attributes-json.ci#L14) | functional/verify | unproven |

### [`RFC8955-12-1`](#rfc8955-12-1)

Specifications relaxing the validation restrictions MUST contain security considerations that provide details on the required additional filtering. (§12)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8955-12-1, so no unit is bound to it.

### [`RFC8955-5.1-1`](#rfc8955-5.1-1)

"This ordering function is such that it does not depend on the arrival order of the Flow Specification via BGP and thus is consistent in the network" (§5.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFlowSpecPrecedenceFromWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/compare_rfc8955_test.go#L10) | unit/verify | unproven |
| positive | [`TestFlowSpecPrecedenceFromWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/compare_rfc8955_test.go#L9) | unit/verify | unproven |

### [`RFC8955-5.1-2`](#rfc8955-5.1-2)

The relative order of two Flow Specifications is determined by comparing their components from the left-most (lowest component type value): the Flow Specification with the lowest numeric type value has higher precedence; for IP destination or source prefix values the more specific prefix has higher precedence and otherwise the lowest IP value does; for all other component types the data is compared as a binary string with memcmp(), the lowest string wins at equal lengths, and at different lengths the common prefix decides with the longest string winning when that prefix is equal (§5.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFlowSpecPrecedenceFromWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/compare_rfc8955_test.go#L12) | unit/verify | unproven |
| negative | [`TestSelectedFlowSpecKernelPacketSemantics`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowspec-firewall/selected_integration_linux_test.go#L24) | unit/verify | unproven |
| positive | [`TestFlowSpecPrecedenceFromWire`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/compare_rfc8955_test.go#L11) | unit/verify | unproven |
| positive | [`TestSelectedFlowSpecKernelPacketSemantics`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowspec-firewall/selected_integration_linux_test.go#L23) | unit/verify | unproven |

### [`RFC8955-8-1`](#rfc8955-8-1)

The VPNv4 Flow Specification NLRI "consists of a fixed-length Route Distinguisher field (8 octets) followed by the Flow Specification NLRI value (Section 4.2)" (§8)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8955VPNNLRIStructure`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/vpn_rfc8955_test.go#L28) | unit/verify | revert, verified |
| positive | [`TestRFC8955VPNNLRIStructure`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/vpn_rfc8955_test.go#L27) | unit/verify | revert, verified |

### [`RFC8955-8-2`](#rfc8955-8-2)

The NLRI length field shall include both the 8 octets of the Route Distinguisher as well as the subsequent Flow Specification NLRI value. (§8)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8955VPNLengthCoversRD`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/vpn_rfc8955_test.go#L57) | unit/verify | revert, verified |
| positive | [`TestRFC8955VPNLengthCoversRD`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/vpn_rfc8955_test.go#L56) | unit/verify | revert, verified |

### [`RFC8955-3-1`](#rfc8955-3-1)

Standard BGP policy mechanisms, such as UPDATE filtering by NLRI prefix as well as community matching, must apply to the Flow specification defined NLRI-type. (§3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestPrefixPolicyFiltersFlowSpecDestination`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/filter_prefix/flowspec_test.go#L13) | unit/verify | unproven |
| positive | [`TestPrefixPolicyFiltersFlowSpecDestination`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/filter_prefix/flowspec_test.go#L12) | unit/verify | unproven |

### [`RFC8955-6-5`](#rfc8955-6-5)

Although the forwarding attributes of two routes for the same Flow Specification prefix may be the same, BGP is still required to perform its path selection algorithm in order to select the correct set of attributes to advertise. (§6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8955FlowSpecLosingPathIsNeverPublished`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8955_flowspec_bestchange_test.go#L159) | unit/verify | revert, verified |
| positive | [`TestRFC8955FlowSpecPathSelectionPicksOneSetOfAttributes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8955_flowspec_bestchange_test.go#L79) | unit/verify | revert, verified |

### [`RFC8955-7.3-2`](#rfc8955-7.3-2)

Where the Terminal Action bit is set and the evaluation continues to the next Flow Specification, "all the Traffic Filtering Actions from these Flow Specifications shall be collected and applied" (§7.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSelectedFlowSpecKernelPacketSemantics`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowspec-firewall/selected_integration_linux_test.go#L26) | unit/verify | unproven |
| positive | [`TestSelectedFlowSpecKernelPacketSemantics`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowspec-firewall/selected_integration_linux_test.go#L25) | unit/verify | unproven |

### [`RFC8955-12-2`](#rfc8955-12-2)

Where the rule-a validation relaxation is used, "for a network to utilize this relaxation, the BGP policies must support additional filtering since the origin AS field is empty" (§12)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8955-12-2, so no unit is bound to it.

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | prose |
| Source | rfc/full/rfc8955.txt |
| Source fingerprint | ab2cc37046acae1d |
| Record | rfc/extraction/rfc8955.json |
| Mapped sentences | 26 |
| Declined as scope | 12 |
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
| `7.2:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates the Section 7.1 encoding obligation for traffic-rate-packets, which uses the same encoding; RFC8955-7.1-1 already carries it and cites both sections. | On encoding, the traffic-rate-packets MUST NOT be negative. |
| `7.2:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates the Section 7.1 decoding obligation for traffic-rate-packets, which uses the same encoding; RFC8955-7.1-2 already carries it and cites both sections. | On decoding, negative values MUST be treated as zero (discard all traffic). |
| `11.2:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | An IANA registration-policy table reproduced as text, not a sentence; it assigns code-point ranges to registration policies. | +==============+========================+ \| Type Values \| Policy \| +==============+========================+ \| 0 \| Reserved \| +--------------+------------------------+ \| [1 .. 127] \| Specification Required \| +--------------+------------------------+ \| [128 .. 254] \| Expert Review \| +--------------+------------------------+ \| 255 \| Reserved \| +--------------+------------------------+ |
| `11.2:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | The obligation binds the IANA designated experts who review Flow Spec Component Types registrations under Specification Required and Expert Review, not a BGP speaker: the sentence tells the experts to verify that a requesting specification reached the IDR Working Group. The producer is the IANA designated expert review process for the Flow Spec Component Types registry, which is a review process and not code; Ze operates no registry, requests no code point and holds no file that would act as this role if it did. | The experts must also verify that any specification produced in the IETF that requests one of these code points has been made available for review by the IDR Working Group and that any specification produced outside the IETF does not conflict with work that is active or already published within the IETF. |
| `11.2:3` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Rhetorical "it must be pointed out"; the sentence warns that new component types can break interoperability and places no obligation on an implementation. | It must be pointed out that introducing new component types may break interoperability with existing implementations of this protocol. |
| `12:3` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Security Considerations exhortation ("additional care must be taken") with no action named; the obligation it introduces is RFC8955-12-2. | Since the validation of Flow Specification (Section 6) depends on this, additional care must be taken. |
| `12:4` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Statement of fact about what systems can and cannot locate in a packet header; it places no obligation. | Systems may not be able to locate all header values required to identify a packet. |

## Superseded

No document obsoletes RFC 8955, so its obligations are stated where they were written.
