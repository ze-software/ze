# RFC 8955 - Dissemination of Flow Specification Rules

Partial. Every requirement this repository extracted from RFC 8955, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 42.9% | 9 of 21 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 33.3% | 7 of 21 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 21 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 0.0% | 0 of 41 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 22 | of 34 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 1 | of 22 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 23.8% | 5 of 21 binding obligations | no test carries the requirement id, whether or not a gap states why |

The 4 shares marked as a part above are the whole of the 21 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 34 |
| Gated MUST-level | 22 |
| Obligations that bind Ze | 21 |
| Not applicable, so out of scope | 1 |
| Declared gaps | 5 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 41 |
| Tagged units | 41 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
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

Five MUST-level gaps, each annotated in [`rfc/short/rfc8955.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc8955.md): [`RFC8955-4-3`](#rfc8955-4-3) -- the MP_REACH next-hop length is taken from the configured next-hop instead of being forced to 0 for SAFI 133/134; [`RFC8955-4.2.1.1-3`](#rfc8955-4.2.1.1-3) -- the numeric-operator decoder keeps reserved bit 4, so an operator carrying it decodes as `=`; [`RFC8955-6-1`](#rfc8955-6-1) -- no feasibility validation of a FlowSpec against the unicast RIB; [`RFC8955-6-2`](#rfc8955-6-2) -- no eBGP leftmost-neighbor-AS enforcement; and [`RFC8955-6-3`](#rfc8955-6-3) -- no revalidation when unicast routes change.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 9 | one part of the gated population |
| Annotated instead of tested | 13 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **22** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (9):** [`RFC8955-4.2-1`](#rfc8955-4.2-1), [`RFC8955-4.2-2`](#rfc8955-4.2-2), [`RFC8955-4.2.1.1-2`](#rfc8955-4.2.1.1-2), [`RFC8955-4.2.1.2-1`](#rfc8955-4.2.1.2-1), [`RFC8955-4.2.2.12-2`](#rfc8955-4.2.2.12-2), [`RFC8955-7.1-1`](#rfc8955-7.1-1), [`RFC8955-7.1-2`](#rfc8955-7.1-2), [`RFC8955-7.3-1`](#rfc8955-7.3-1), [`RFC8955-7.5-1`](#rfc8955-7.5-1)

**Annotated instead of tested (13):** [`RFC8955-4-1`](#rfc8955-4-1), [`RFC8955-4-2`](#rfc8955-4-2), [`RFC8955-4-3`](#rfc8955-4-3), [`RFC8955-4-4`](#rfc8955-4-4), [`RFC8955-4.2.1.1-1`](#rfc8955-4.2.1.1-1), [`RFC8955-4.2.1.1-3`](#rfc8955-4.2.1.1-3), [`RFC8955-4.2.2.9-1`](#rfc8955-4.2.2.9-1), [`RFC8955-4.2.2.11-1`](#rfc8955-4.2.2.11-1), [`RFC8955-4.2.2.12-1`](#rfc8955-4.2.2.12-1), [`RFC8955-6-1`](#rfc8955-6-1), [`RFC8955-6-2`](#rfc8955-6-2), [`RFC8955-6-3`](#rfc8955-6-3), [`RFC8955-12-1`](#rfc8955-12-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC8955-4-1` | Implementations wishing to exchange Flow Specification MUST use BGP's Capability Advertisement facility to exchange the Multiprotocol Extension Capability Code (Code 1) (§4) | MUST | 4 | **positive:** `unit/verify` [`TestIPv4FlowSpecNegotiatesMultiprotocolCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/plugin_test.go#L846). **negative:** no negative test. **{single-polarity}:** the flowspec plugin unconditionally maps each declared FlowSpec family to a Multiprotocol (Code 1) capability during OPEN, so there is no wrong input the negotiation path rejects (internal/component/bgp/plugins/nlri/flowspec/register.go, types.go:47) |
| `RFC8955-4-2` | The (AFI, SAFI) pair carried in the Multiprotocol Extension Capability MUST be (AFI=1, SAFI=133) for IPv4 Flow Specification and (AFI=1, SAFI=134) for VPNv4 Flow Specification (§4) | MUST | 4 | **positive:** `unit/verify` [`TestFlowSpecVPNFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L647). **positive:** `unit/verify` [`TestIPv4FlowSpecNegotiatesMultiprotocolCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/plugin_test.go#L847). **negative:** no negative test. **{single-polarity}:** the (AFI 1, SAFI 133) and (AFI 1, SAFI 134) pairs are family-registration constants, not an input guard, so only the positive assignment is assertable (internal/component/bgp/plugins/nlri/flowspec/types.go:47-50) |
| `RFC8955-4-3` | Length of the Next-Hop Network Address MUST be set to 0 (§4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** buildMPReachPlugin writes the MP_REACH next-hop length from the configured next-hop with no SAFI 133/134 special case (internal/component/bgp/message/update_build_plugin.go:140,:147), and the FlowSpec config parser passes the operator's next-hop straight through (internal/component/bgp/plugins/nlri/flowspec/config.go:94 -> internal/component/bgp/reactor/peer_static_routes.go:22), so a FlowSpec route configured with a next-hop encodes a 4- or 16-octet length; buildMPReachFlowSpec does the same (internal/component/bgp/message/update_build_flowspec.go:148,:180) |
| `RFC8955-4-4` | Network Address of the Next-Hop field MUST be ignored (§4) | MUST | 4 | **positive:** `unit/verify` [`TestRFC8955NextHopIgnoredForFlowSpec`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowspec-firewall/bridge_test.go#L264). **negative:** no negative test. **{single-polarity}:** the FlowSpec receive path lowers an UPDATE to firewall Terms from the NLRI components and action communities alone and no code in internal/plugins/flowspec-firewall reads a next-hop, so the field is ignored and there is no next-hop value to reject (internal/plugins/flowspec-firewall/translate.go:38, engine.go:106) |
| `RFC8955-4.2-1` | Components MUST follow strict type ordering by increasing numerical order (§4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestFlowSpecComponentsAscendingOrder`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1228). **positive:** `unit/verify` [`TestFlowSpecJoinsRepeatedTypeIntoOneComponent`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1270). **negative:** `unit/verify` [`TestAddComponentRefusesASecondPrefix`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1387). **negative:** `unit/verify` [`TestParseFlowSpecRefusesRepeatedComponentType`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1307). **negative:** `unit/verify` [`TestParseFlowSpecVPNRefusesRepeatedComponentType`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1367) |
| `RFC8955-4.2-2` | If a component is present, it MUST precede any component of higher numeric type value (§4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestFlowSpecComponentsAscendingOrder`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1229). **negative:** `unit/verify` [`TestParseFlowSpecRefusesDescendingComponentType`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1335) |
| `RFC8955-4.2.1.1-1` | In the first operator octet of a sequence, the AND bit MUST be encoded as unset (§4.2.1.1) | MUST | 4.2.1.1 | **positive:** `unit/verify` [`TestRFC8955FirstOperatorAndBitEncodedUnset`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1542). **negative:** no negative test. **{single-polarity}:** parseFlowMatches derives the AND bit purely from the position inside a '&'-joined expression (isAnd := i > 0), so the first operator-value pair is always encoded with the AND bit clear and no input sets it (internal/component/bgp/plugins/nlri/flowspec/config_builder.go:220,:252) |
| `RFC8955-4.2.1.1-2` | First operator AND bit MUST be treated as always unset on decoding (§4.2.1.1) | MUST | 4.2.1.1 | **positive:** `unit/verify` [`TestRFC8955FirstOperatorAndBitTreatedUnsetOnDecode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1573). **negative:** `unit/verify` [`TestRFC8955FirstOperatorAndBitTreatedUnsetOnDecode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1577) |
| `RFC8955-4.2.1.1-3` | Numeric operator reserved bit (bit 4) MUST be set to 0 on NLRI encoding and MUST be ignored during decoding (§4.2.1.1) | MUST | 4.2.1.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** parseNumericComponent masks off only the end-of-list, AND and length bits (op &^ 0xF0), so reserved bit 4 (0x08) survives into FlowMatch.Op and formatWithOperator's switch then falls through to the '=' default -- a received '>' operator with the reserved bit set decodes as '=' instead of being ignored (internal/component/bgp/plugins/nlri/flowspec/types_numeric.go:322, plugin_decode.go:247-265) |
| `RFC8955-4.2.1.2-1` | Bitmask operator reserved bits (bits 4-5) MUST be set to 0 on NLRI encoding and MUST be ignored during decoding (§4.2.1.2) | MUST | 4.2.1.2 | **positive:** `unit/verify` [`TestFlowSpecBitmaskOperatorReservedBitsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1450). **negative:** `unit/verify` [`TestRFC8955BitmaskOperatorReservedBitsIgnoredOnDecode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1611) |
| `RFC8955-4.2.2.9-1` | Type 9 (TCP Flags) component bitmasks MUST be encoded as 1- or 2-octet bitmask (§4.2.2.9) | MUST | 4.2.2.9 | **positive:** `unit/verify` [`TestRFC8955TCPFlagsBitmaskSingleOctet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1671). **negative:** no negative test. **{single-polarity}:** parseFlowTCPFlagMatches resolves flag names to 8-bit values and numericComponent.Bytes() selects the 1-octet length code for any value <= 0xFF, so an emitted Type-9 bitmask is always 1 octet and no over-long bitmask can be produced to reject (internal/component/bgp/plugins/nlri/flowspec/config_builder.go:414-446, types_numeric.go:47-55) |
| `RFC8955-4.2.2.11-1` | Type 11 (DSCP) component values MUST be encoded as single octet (§4.2.2.11) | MUST | 4.2.2.11 | **positive:** `unit/verify` [`TestRFC8955DSCPValueSingleOctet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1699). **negative:** no negative test. **{single-polarity}:** parseFlowOctets parses DSCP values as uint8 and NewFlowDSCPComponent stores them, so numericComponent.Bytes() always selects the 1-octet length code and no multi-octet DSCP encoding exists to reject (internal/component/bgp/plugins/nlri/flowspec/config_builder.go:261-273, types_numeric.go:473-479) |
| `RFC8955-4.2.2.12-1` | Type 12 (Fragment) component bitmask MUST be encoded as single octet bitmask (§4.2.2.12) | MUST | 4.2.2.12 | **positive:** `unit/verify` [`TestRFC8955FragmentBitmaskSingleOctet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1726). **negative:** no negative test. **{single-polarity}:** fragment values come from the four low-nibble FlowFragmentFlag constants, so numericComponent.Bytes() always selects the 1-octet length code and no multi-octet fragment bitmask can be produced to reject (internal/component/bgp/plugins/nlri/flowspec/types.go:201-206, types_numeric.go:481-491) |
| `RFC8955-4.2.2.12-2` | Fragment bitmask reserved bits MUST be set to 0 on NLRI encoding and MUST be ignored during decoding (§4.2.2.12) | MUST | 4.2.2.12 | **positive:** `unit/verify` [`TestFlowSpecFragmentReservedHighNibbleZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1479). **negative:** `unit/verify` [`TestRFC8955FragmentReservedBitsIgnoredOnDecode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1644) |
| `RFC8955-6-1` | Flow Specification NLRI MUST be validated such that it is considered feasible if and only if all validation conditions are true (§6) | MUST | 6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze decodes a received FlowSpec NLRI and lowers it to the firewall with no feasibility validation against the unicast RIB; no code implements the Section 6 procedure (internal/component/bgp/plugins/nlri/flowspec/types.go:351, internal/plugins/flowspec-firewall/translate.go:38) |
| `RFC8955-6-2` | BGP implementations MUST enforce that AS_PATH attribute of a route received via eBGP contains the neighboring AS in the left-most position (§6) | MUST | 6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze runs no eBGP leftmost-neighbor-AS enforcement; the only AS_PATH ingress guard is RFC 4271 Section 9 loop detection and firstASInPath serves MED neighbor comparison only (internal/component/bgp/reactor/filter/loop_metrics.go:31, internal/component/bgp/plugins/rib/bestpath.go:544) |
| `RFC8955-6-3` | Revalidation of the Flow Specification NLRI MUST be performed whenever unicast routes change (§6) | MUST | 6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** with no Section 6 validation implemented there is nothing to revalidate; the flowspec-firewall bridge rebuilds rules only on FlowSpec UPDATE and peer-down events, never on a unicast route change (internal/plugins/flowspec-firewall/engine.go:70-79) |
| `RFC8955-7.1-1` | traffic-rate (bytes and packets) MUST NOT be negative on encoding (§7.1, §7.2) | MUST NOT | 7.1 | **positive:** `unit/verify` [`TestParseExtendedCommunitiesTrafficRatePackets`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/route/route_parse_test.go#L1234). **negative:** `unit/verify` [`TestParseExtendedCommunitiesTrafficRatePackets`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/route/route_parse_test.go#L1235) |
| `RFC8955-7.1-2` | Negative traffic-rate values on decoding MUST be treated as zero (discard all traffic) (§7.1, §7.2) | MUST | 7.1 | **positive:** `unit/verify` [`TestRFC8955TrafficRateNegativeDecodesAsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/cli/decode_test.go#L1465). **negative:** `unit/verify` [`TestRFC8955TrafficRateNegativeDecodesAsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/cli/decode_test.go#L1473) |
| `RFC8955-7.3-1` | traffic-action unused bits MUST be set to 0 on encoding and MUST be ignored during decoding (§7.3) | MUST | 7.3 | **positive:** `unit/verify` [`TestRFC8955TrafficActionBitsDecoded`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/extcomm_decoded_test.go#L228). **positive:** `unit/verify` [`TestRFC8955TrafficActionUnusedBitsIgnoredOnDecode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/cli/decode_test.go#L1510). **positive:** `unit/verify` [`TestRFC8955TrafficActionUnusedBitsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/routeattr_test.go#L207). **negative:** `unit/verify` [`TestRFC8955TrafficActionBitsDecoded`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/extcomm_decoded_test.go#L242). **negative:** `unit/verify` [`TestRFC8955TrafficActionUnusedBitsIgnoredOnDecode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/cli/decode_test.go#L1517). **positive:** `functional/verify` [`community-attributes-json.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/community-attributes-json.ci#L16). **negative:** `functional/verify` [`community-attributes-json.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/community-attributes-json.ci#L17) |
| `RFC8955-7.5-1` | traffic-marking reserved bits MUST be set to 0 on encoding and MUST be ignored during decoding (§7.5) | MUST | 7.5 | **positive:** `unit/verify` [`TestEncodeRouteEmitsZeroValuedActions`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/encode_test.go#L53). **positive:** `unit/verify` [`TestEncodeRouteMarkKeepsReservedBitsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/encode_test.go#L86). **positive:** `unit/verify` [`TestParseExtendedCommunityMarkDSCPBound`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/routeattr_flowspec_test.go#L34). **positive:** `unit/verify` [`TestRFC8955TrafficMarkingReservedBitsIgnored`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/extcomm_decoded_test.go#L194). **negative:** `unit/verify` [`TestEncodeRouteMarkKeepsReservedBitsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/encode_test.go#L92). **negative:** `unit/verify` [`TestParseExtendedCommunityMarkDSCPBound`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/routeattr_flowspec_test.go#L56). **negative:** `unit/verify` [`TestRFC8955TrafficMarkingReservedBitsIgnored`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/extcomm_decoded_test.go#L201). **positive:** `functional/verify` [`community-attributes-json.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/community-attributes-json.ci#L14). **negative:** `functional/verify` [`community-attributes-json.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/community-attributes-json.ci#L15) |
| `RFC8955-12-1` | Specifications relaxing the validation restrictions MUST contain security considerations with details on required additional filtering (§12) | MUST | 12 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** this obligation binds IETF documents that relax the Section 6 validation restrictions, not implementations; ze publishes no such specification and offers no validation-relaxation knob -- grep -rni 'relax' over internal/component/bgp/plugins/nlri/flowspec and internal/plugins/flowspec-firewall returns no producer, and no Section 6 validation exists to relax |
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
| `RFC8955-6-4` | Rule a (destination prefix requirement) MAY be relaxed by explicit configuration; if so, rules b and c MUST be disregarded (§6) | MAY | 6 | **positive:** no positive test. **negative:** no negative test |
| `RFC8955-4.2-4` | A given component type MAY appear exactly once (§4.2) | MAY | 4.2 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC8955-4-3`](#rfc8955-4-3) Length of the Next-Hop Network Address MUST be set to 0 (§4) | {gap}, no test | buildMPReachPlugin writes the MP_REACH next-hop length from the configured next-hop with no SAFI 133/134 special case (internal/component/bgp/message/update_build_plugin.go:140,:147), and the FlowSpec config parser passes the operator's next-hop straight through (internal/component/bgp/plugins/nlri/flowspec/config.go:94 -> internal/component/bgp/reactor/peer_static_routes.go:22), so a FlowSpec route configured with a next-hop encodes a 4- or 16-octet length; buildMPReachFlowSpec does the same (internal/component/bgp/message/update_build_flowspec.go:148,:180) |
| [`RFC8955-4.2.1.1-3`](#rfc8955-4.2.1.1-3) Numeric operator reserved bit (bit 4) MUST be set to 0 on NLRI encoding and MUST be ignored during decoding (§4.2.1.1) | {gap}, no test | parseNumericComponent masks off only the end-of-list, AND and length bits (op &^ 0xF0), so reserved bit 4 (0x08) survives into FlowMatch.Op and formatWithOperator's switch then falls through to the '=' default -- a received '>' operator with the reserved bit set decodes as '=' instead of being ignored (internal/component/bgp/plugins/nlri/flowspec/types_numeric.go:322, plugin_decode.go:247-265) |
| [`RFC8955-6-1`](#rfc8955-6-1) Flow Specification NLRI MUST be validated such that it is considered feasible if and only if all validation conditions are true (§6) | {gap}, no test | ze decodes a received FlowSpec NLRI and lowers it to the firewall with no feasibility validation against the unicast RIB; no code implements the Section 6 procedure (internal/component/bgp/plugins/nlri/flowspec/types.go:351, internal/plugins/flowspec-firewall/translate.go:38) |
| [`RFC8955-6-2`](#rfc8955-6-2) BGP implementations MUST enforce that AS_PATH attribute of a route received via eBGP contains the neighboring AS in the left-most position (§6) | {gap}, no test | ze runs no eBGP leftmost-neighbor-AS enforcement; the only AS_PATH ingress guard is RFC 4271 Section 9 loop detection and firstASInPath serves MED neighbor comparison only (internal/component/bgp/reactor/filter/loop_metrics.go:31, internal/component/bgp/plugins/rib/bestpath.go:544) |
| [`RFC8955-6-3`](#rfc8955-6-3) Revalidation of the Flow Specification NLRI MUST be performed whenever unicast routes change (§6) | {gap}, no test | with no Section 6 validation implemented there is nothing to revalidate; the flowspec-firewall bridge rebuilds rules only on FlowSpec UPDATE and peer-down events, never on a unicast route change (internal/plugins/flowspec-firewall/engine.go:70-79) |
| [`RFC8955-12-1`](#rfc8955-12-1) Specifications relaxing the validation restrictions MUST contain security considerations with details on required additional filtering (§12) | no test | no test carries this requirement id; annotated {not-applicable}: this obligation binds IETF documents that relax the Section 6 validation restrictions, not implementations; ze publishes no such specification and offers no validation-relaxation knob -- grep -rni 'relax' over internal/component/bgp/plugins/nlri/flowspec and internal/plugins/flowspec-firewall returns no producer, and no Section 6 validation exists to relax |

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

No test carries RFC8955-4-3, so no unit is bound to it.

### [`RFC8955-4-4`](#rfc8955-4-4)

Network Address of the Next-Hop field MUST be ignored (§4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC8955NextHopIgnoredForFlowSpec`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowspec-firewall/bridge_test.go#L264) | unit/verify | unproven |

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

If a component is present, it MUST precede any component of higher numeric type value (§4.2)

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

No test carries RFC8955-4.2.1.1-3, so no unit is bound to it.

### [`RFC8955-4.2.1.2-1`](#rfc8955-4.2.1.2-1)

Bitmask operator reserved bits (bits 4-5) MUST be set to 0 on NLRI encoding and MUST be ignored during decoding (§4.2.1.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8955BitmaskOperatorReservedBitsIgnoredOnDecode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1611) | unit/verify | unproven |
| positive | [`TestFlowSpecBitmaskOperatorReservedBitsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1450) | unit/verify | unproven |

### [`RFC8955-4.2.2.9-1`](#rfc8955-4.2.2.9-1)

Type 9 (TCP Flags) component bitmasks MUST be encoded as 1- or 2-octet bitmask (§4.2.2.9)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC8955TCPFlagsBitmaskSingleOctet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1671) | unit/verify | unproven |

### [`RFC8955-4.2.2.11-1`](#rfc8955-4.2.2.11-1)

Type 11 (DSCP) component values MUST be encoded as single octet (§4.2.2.11)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC8955DSCPValueSingleOctet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1699) | unit/verify | unproven |

### [`RFC8955-4.2.2.12-1`](#rfc8955-4.2.2.12-1)

Type 12 (Fragment) component bitmask MUST be encoded as single octet bitmask (§4.2.2.12)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC8955FragmentBitmaskSingleOctet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1726) | unit/verify | unproven |

### [`RFC8955-4.2.2.12-2`](#rfc8955-4.2.2.12-2)

Fragment bitmask reserved bits MUST be set to 0 on NLRI encoding and MUST be ignored during decoding (§4.2.2.12)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8955FragmentReservedBitsIgnoredOnDecode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1644) | unit/verify | unproven |
| positive | [`TestFlowSpecFragmentReservedHighNibbleZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1479) | unit/verify | unproven |

### [`RFC8955-6-1`](#rfc8955-6-1)

Flow Specification NLRI MUST be validated such that it is considered feasible if and only if all validation conditions are true (§6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8955-6-1, so no unit is bound to it.

### [`RFC8955-6-2`](#rfc8955-6-2)

BGP implementations MUST enforce that AS_PATH attribute of a route received via eBGP contains the neighboring AS in the left-most position (§6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8955-6-2, so no unit is bound to it.

### [`RFC8955-6-3`](#rfc8955-6-3)

Revalidation of the Flow Specification NLRI MUST be performed whenever unicast routes change (§6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8955-6-3, so no unit is bound to it.

### [`RFC8955-7.1-1`](#rfc8955-7.1-1)

traffic-rate (bytes and packets) MUST NOT be negative on encoding (§7.1, §7.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestParseExtendedCommunitiesTrafficRatePackets`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/route/route_parse_test.go#L1235) | unit/verify | unproven |
| positive | [`TestParseExtendedCommunitiesTrafficRatePackets`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/route/route_parse_test.go#L1234) | unit/verify | unproven |

### [`RFC8955-7.1-2`](#rfc8955-7.1-2)

Negative traffic-rate values on decoding MUST be treated as zero (discard all traffic) (§7.1, §7.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8955TrafficRateNegativeDecodesAsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/cli/decode_test.go#L1473) | unit/verify | unproven |
| positive | [`TestRFC8955TrafficRateNegativeDecodesAsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/cli/decode_test.go#L1465) | unit/verify | unproven |

### [`RFC8955-7.3-1`](#rfc8955-7.3-1)

traffic-action unused bits MUST be set to 0 on encoding and MUST be ignored during decoding (§7.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8955TrafficActionUnusedBitsIgnoredOnDecode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/cli/decode_test.go#L1517) | unit/verify | unproven |
| negative | [`TestRFC8955TrafficActionBitsDecoded`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/extcomm_decoded_test.go#L242) | unit/verify | unproven |
| negative | [`community-attributes-json.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/community-attributes-json.ci#L17) | functional/verify | unproven |
| positive | [`TestRFC8955TrafficActionUnusedBitsIgnoredOnDecode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/cli/decode_test.go#L1510) | unit/verify | unproven |
| positive | [`TestRFC8955TrafficActionUnusedBitsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/routeattr_test.go#L207) | unit/verify | unproven |
| positive | [`TestRFC8955TrafficActionBitsDecoded`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/extcomm_decoded_test.go#L228) | unit/verify | unproven |
| positive | [`community-attributes-json.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/community-attributes-json.ci#L16) | functional/verify | unproven |

### [`RFC8955-7.5-1`](#rfc8955-7.5-1)

traffic-marking reserved bits MUST be set to 0 on encoding and MUST be ignored during decoding (§7.5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestParseExtendedCommunityMarkDSCPBound`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/routeattr_flowspec_test.go#L56) | unit/verify | unproven |
| negative | [`TestEncodeRouteMarkKeepsReservedBitsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/encode_test.go#L92) | unit/verify | unproven |
| negative | [`TestRFC8955TrafficMarkingReservedBitsIgnored`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/extcomm_decoded_test.go#L201) | unit/verify | unproven |
| negative | [`community-attributes-json.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/community-attributes-json.ci#L15) | functional/verify | unproven |
| positive | [`TestParseExtendedCommunityMarkDSCPBound`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/routeattr_flowspec_test.go#L34) | unit/verify | unproven |
| positive | [`TestEncodeRouteEmitsZeroValuedActions`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/encode_test.go#L53) | unit/verify | unproven |
| positive | [`TestEncodeRouteMarkKeepsReservedBitsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/encode_test.go#L86) | unit/verify | unproven |
| positive | [`TestRFC8955TrafficMarkingReservedBitsIgnored`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/extcomm_decoded_test.go#L194) | unit/verify | unproven |
| positive | [`community-attributes-json.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/community-attributes-json.ci#L14) | functional/verify | unproven |

### [`RFC8955-12-1`](#rfc8955-12-1)

Specifications relaxing the validation restrictions MUST contain security considerations with details on required additional filtering (§12)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8955-12-1, so no unit is bound to it.

## Extraction sign-off

No extraction sign-off exists for RFC 8955, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 8955, so its obligations are stated where they were written.
