# RFC 8955 - Dissemination of Flow Specification Rules

Partial. Every requirement this repository extracted from RFC 8955, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 69.0% | 20 of 29 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 20.7% | 6 of 29 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 29 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 0.0% | 0 of 29 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 59.4% | 63 of 106 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 29 | of 49 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 2 | of 29 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 6.9% | 2 of 29 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 29 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 29 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |
| Linux cannot do this | 0.0% | 0 of 29 gated MUSTs | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and remains unmet; it stays in every denominator and earns no proof credit |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Partial proof; remaining gap | 3.4% | 1 of 29 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Audit verdicts | 29 | of 29 gated MUSTs judged | 1 weak, wrong or unimplemented, 3 no longer current. Each is named below under its own requirement id |

The 9 shares marked as a part above are the whole of the 29 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| Partial proof; remaining gap | bad | green at zero, RED above it: a tested clause cannot prove the whole requirement |
| No test at all | ok | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green shares above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Linux cannot do this | neutral | neutral at every value: an evidenced platform limit is neither a proof nor a scope exemption |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | bad | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Partial |
| Enrolment | Enrolled |
| Requirements | 49 |
| Gated MUST-level | 29 |
| Not applicable, so out of scope | 2 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 106 |
| Tagged units | 106 |
| Recorded audit verdicts | 29 |
| Discrimination records | 63 |
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

Preferred destination/source-port, ICMP-code and packet-length emission now have producer and recipient-wire proof; zero packet-rate discard, controls and withdrawal have runtime-kernel proof for local IPv4 input. These do not establish event-subscription delivery or broader firewall precedence, continuation, sampling, policing and marking. Other native codec, prefix-policy projection, unicast-authorized selection and revalidation changes still require their own integration validation. VPN FlowSpec is validated and propagated by the BGP RIB but is not installed by the global firewall bridge. The rule-a destination bypass remains absent. A FlowSpec route carrying the redirect-to-IP-next-hop (0x0800) community is advertised with the redirect address as its MP_REACH_NLRI next hop, against the Section 4 zero-length rule ([`RFC8955-4-3`](#rfc8955-4-3), `{partial}`): an owner-approved deviation of 2026-10-20/21 for draft-simpson-idr-flowspec-redirect-02 and ExaBGP compatibility, never counted conformant.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 20 | one part of the gated population |
| Annotated (including scoped evidence) | 9 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 1 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **29** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (20):** [`RFC8955-4-4`](#rfc8955-4-4), [`RFC8955-4.2-1`](#rfc8955-4.2-1), [`RFC8955-4.2-2`](#rfc8955-4.2-2), [`RFC8955-4.2.1.1-2`](#rfc8955-4.2.1.1-2), [`RFC8955-4.2.1.1-3`](#rfc8955-4.2.1.1-3), [`RFC8955-4.2.1.2-1`](#rfc8955-4.2.1.2-1), [`RFC8955-4.2.2.12-2`](#rfc8955-4.2.2.12-2), [`RFC8955-6-1`](#rfc8955-6-1), [`RFC8955-6-3`](#rfc8955-6-3), [`RFC8955-7.1-1`](#rfc8955-7.1-1), [`RFC8955-7.1-2`](#rfc8955-7.1-2), [`RFC8955-7.3-1`](#rfc8955-7.3-1), [`RFC8955-7.5-1`](#rfc8955-7.5-1), [`RFC8955-5.1-1`](#rfc8955-5.1-1), [`RFC8955-5.1-2`](#rfc8955-5.1-2), [`RFC8955-8-1`](#rfc8955-8-1), [`RFC8955-8-2`](#rfc8955-8-2), [`RFC8955-3-1`](#rfc8955-3-1), [`RFC8955-6-5`](#rfc8955-6-5), [`RFC8955-7.3-2`](#rfc8955-7.3-2)

**Annotated (including scoped evidence) (9):** [`RFC8955-4-1`](#rfc8955-4-1), [`RFC8955-4-2`](#rfc8955-4-2), [`RFC8955-4-3`](#rfc8955-4-3), [`RFC8955-4.2.1.1-1`](#rfc8955-4.2.1.1-1), [`RFC8955-4.2.2.9-1`](#rfc8955-4.2.2.9-1), [`RFC8955-4.2.2.11-1`](#rfc8955-4.2.2.11-1), [`RFC8955-4.2.2.12-1`](#rfc8955-4.2.2.12-1), [`RFC8955-12-1`](#rfc8955-12-1), [`RFC8955-12-2`](#rfc8955-12-2)

**Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) (1):** [`RFC8955-4-3`](#rfc8955-4-3)

## Attainable ceiling

Mandatory ceiling: 27 of 29 (93.1%); excluded: Not applicable (classification can be revisited): 2 of 29 (6.9%). Optional ceiling: 20 of 20 (100.0%). Lowest mandatory ceiling: rfc8955 27 of 29 (93.1%). Lowest optional ceiling: rfc8955 20 of 20 (100.0%). Ceilings describe current classifications, not an immutable Linux maximum. Not-applicable and feature-declined classifications can be revisited. Every gap remains work Ze owes.

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC8955-4-1` | Implementations wishing to exchange Flow Specification MUST use BGP's Capability Advertisement facility to exchange the Multiprotocol Extension Capability Code (Code 1) (§4) | MUST | 4 | **positive:** `unit/verify` [`TestIPv4FlowSpecNegotiatesMultiprotocolCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/plugin_test.go#L903). **negative:** no negative test. **{single-polarity}:** the flowspec plugin unconditionally maps each declared FlowSpec family to a Multiprotocol (Code 1) capability during OPEN, so there is no wrong input the negotiation path rejects (internal/component/bgp/plugins/nlri/flowspec/register.go: the init registration, and the family vars at register.go:22) |
| `RFC8955-4-2` | The (AFI, SAFI) pair carried in the Multiprotocol Extension Capability MUST be (AFI=1, SAFI=133) for IPv4 Flow Specification and (AFI=1, SAFI=134) for VPNv4 Flow Specification (§4) | MUST | 4 | **positive:** `unit/verify` [`TestFlowSpecVPNFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L648). **positive:** `unit/verify` [`TestIPv4FlowSpecNegotiatesMultiprotocolCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/plugin_test.go#L904). **negative:** no negative test. **{single-polarity}:** the (AFI 1, SAFI 133) and (AFI 1, SAFI 134) pairs are family-registration constants, not an input guard, so only the positive assignment is assertable (internal/component/bgp/plugins/nlri/flowspec/register.go:22-25) |
| `RFC8955-4-3` | Length of the Next-Hop Network Address MUST be set to 0 (§4) | MUST | 4 | **positive:** `unit/verify` [`TestFlowSpecForwardingOmitsNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/flowspec_forward_wire_test.go#L31). **positive:** `unit/verify` [`TestFlowSpecOriginationOmitsConfiguredNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/flowspec_origin_wire_test.go#L32). **positive:** `unit/verify` [`TestFlowSpecRouteServerOmitsNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/flowspec_rs_wire_test.go#L26). **positive:** `unit/verify` [`TestFlowSpecUpdateIgnoresConfiguredNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/flowspec_wire_contract_test.go#L15). **negative:** `unit/verify` [`TestFlowSpecForwardingKeepsLegacyNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8955_flowspec_forward_mixed_test.go#L16). **negative:** `unit/verify` [`TestFlowSpecForwardingOmitsNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/flowspec_forward_wire_test.go#L32). **negative:** `unit/verify` [`TestFlowSpecOriginationOmitsConfiguredNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/flowspec_origin_wire_test.go#L33). **negative:** `unit/verify` [`TestFlowSpecRouteServerOmitsNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/flowspec_rs_wire_test.go#L27). **negative:** `unit/verify` [`TestFlowSpecUpdateIgnoresConfiguredNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/flowspec_wire_contract_test.go#L16). **positive:** `functional/verify` [`flow-encode.ci`](https://github.com/ze-software/ze/blob/main/test/encode/flow-encode.ci#L15). **{partial}:** tested "MUST be set to 0"; gap "Length of the Next-Hop Network Address"; internal/component/bgp/message/update_build_plugin.go::MPReachCarriesNextHop is the boundary: Tested covers every FlowSpec advertisement that carries no redirect-to-IP-next-hop (0x0800) community, which every rail encodes with a zero-length next hop. The gap is a route carrying the 0x0800 community: MPReachCarriesNextHop returns true for it, so its MP_REACH_NLRI carries the redirect address as the next hop with a non-zero length, an owner-approved deviation of 2026-10-20/21 for draft-simpson-idr-flowspec-redirect-02 Section 3 and ExaBGP compatibility, never counted conformant. internal/component/bgp/reactor/peer_forward_facts.go::precomputeRedirectNextHop is the second producer of the same deviation, on the forward rails: a received route carrying the 0x0800 community and a 4- or 16-octet next hop leaves with that address kept or reset per destination (applyRedirectNextHop), and with length 0 only where the action does not leave with it. **Scoped evidence:** Tests and tag-claim records apply only to Tested; zero whole-requirement credit. partial (the tests enforce only the declared tested scope; the whole requirement remains unmet), fresh. Rejudged 2026-10-23 by an independent auditor (not the author) after 010e649057, which makes the final writer (session_ownership.go adjOutWrite.allow) send a synthesized withdrawal only to a destination holding an adj-out record for the route. RFC 4271 Section 3.1 allows a withdrawal only for "the IP prefix that expresses the destination for a previously advertised route". RFC 8955 Section 4: "When advertising Flow Specifications, the Length of the Next-Hop Network Address MUST be set to 0." Tested "MUST be set to 0"; Gap "Length of the Next-Hop Network Address"; Producer internal/component/bgp/message/update_build_plugin.go::MPReachCarriesNextHop returns true for the 0x0800 redirect-to-IP-next-hop community, and internal/component/bgp/reactor/peer_forward_facts.go::precomputeRedirectNextHop keeps or resets that address on the forward rails: the owner-approved deviation for draft-simpson-idr-flowspec-redirect-02 Section 3 and ExaBGP compatibility, never counted conformant; the whole unconditional MUST stays unmet with zero whole-requirement credit. The moved unit is TestFlowSpecForwardingKeepsLegacyNextHop (negative tag): its legacy-withdraw subcase now plants the unicast sibling at the destination (advertiseFirst) so the peer-own legacy next-hop gate's withdrawal is legitimate. Assertions unchanged: exactly two recipient UPDATEs; the FlowSpec section is the exact zero-length-next-hop MP_REACH (assertFlowForwardWire) although its received field names the recipient; the legacy section is the exact withdrawal under the gate or the unchanged NEXT_HOP and NLRI otherwise; received observer bytes are unmodified. The other carriers (TestFlowSpecUpdateIgnoresConfiguredNextHop, TestFlowSpecOriginationOmitsConfiguredNextHop, TestFlowSpecForwardingOmitsNextHop, TestFlowSpecRouteServerOmitsNextHop, test/encode/flow-encode.ci) are byte-identical and keep the 2026-10-22 judgement; flow-encode sequences 2/8/9 pin the disclosed redirect deviation and give no conformance credit. Ran: PASS on 2026-10-23. |
| `RFC8955-4-4` | Network Address of the Next-Hop field MUST be ignored (§4) | MUST | 4 | **positive:** `unit/verify` [`TestRFC8955NextHopIgnoredForFlowSpec`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_flowspec_validation_test.go#L512). **negative:** `unit/verify` [`TestRFC8955NextHopIgnoredForFlowSpec`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_flowspec_validation_test.go#L513) |
| `RFC8955-4.2-1` | Components MUST follow strict type ordering by increasing numerical order (§4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestFlowSpecComponentsAscendingOrder`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1229). **positive:** `unit/verify` [`TestFlowSpecJoinsRepeatedTypeIntoOneComponent`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1271). **negative:** `unit/verify` [`TestAddComponentRefusesASecondPrefix`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1388). **negative:** `unit/verify` [`TestParseFlowSpecRefusesRepeatedComponentType`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1308). **negative:** `unit/verify` [`TestParseFlowSpecVPNRefusesRepeatedComponentType`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1368) |
| `RFC8955-4.2-2` | If present, it MUST precede any component of higher numeric type value. (§4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestFlowSpecComponentsAscendingOrder`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1230). **negative:** `unit/verify` [`TestParseFlowSpecRefusesDescendingComponentType`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1336) |
| `RFC8955-4.2.1.1-1` | In the first operator octet of a sequence, it MUST be encoded as unset (§4.2.1.1) | MUST | 4.2.1.1 | **positive:** `unit/verify` [`TestRFC8955FirstOperatorAndBitEncodedUnset`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1541). **negative:** no negative test. **{single-polarity}:** parseNumericTermsText sets the AND bit only on a term that follows an '&' and refuses a token that starts with '&' when no term comes before it, so the first operator-value pair is always encoded with the AND bit clear and no input sets it (internal/component/bgp/plugins/nlri/flowspec/plugin_encode_text.go parseNumericTermsText, reached from config through config_builder.go parseFlowNumericMatches) |
| `RFC8955-4.2.1.1-2` | In the first operator octet of a sequence, it MUST be encoded as unset and MUST be treated as always unset on decoding. (§4.2.1.1) | MUST | 4.2.1.1 | **positive:** `unit/verify` [`TestRFC8955FirstOperatorAndBitTreatedUnsetOnDecode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1572). **positive:** `unit/verify` [`TestRFC8955TextLeadingAndEncodedUnset`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/rfc8955_text_encode_reserved_test.go#L36). **negative:** `unit/verify` [`TestRFC8955FirstOperatorAndBitTreatedUnsetOnDecode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1576) |
| `RFC8955-4.2.1.1-3` | 0: MUST be set to 0 on NLRI encoding and MUST be ignored during decoding (§4.2.1.1) | MUST | 4.2.1.1 | **positive:** `unit/verify` [`TestFlowSpecNumericReservedAndEightOctetOperand`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/flowspec/compare_rfc8955_test.go#L42). **negative:** `unit/verify` [`TestFlowSpecNumericReservedAndEightOctetOperand`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/flowspec/compare_rfc8955_test.go#L43) |
| `RFC8955-4.2.1.2-1` | 0 (all 0 bits): MUST be set to 0 on NLRI encoding and MUST be ignored during decoding (§4.2.1.2) | MUST | 4.2.1.2 | **positive:** `unit/verify` [`TestFlowSpecBitmaskOperatorReservedBitsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1449). **negative:** `unit/verify` [`TestRFC8955BitmaskOperatorReservedBitsIgnoredOnDecode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1610) |
| `RFC8955-4.2.2.9-1` | Type 9 component bitmasks MUST be encoded as 1- or 2-octet bitmask (bitmask_op len=00 or len=01). (§4.2.2.9) | MUST | 4.2.2.9 | **positive:** `unit/verify` [`TestRFC8955TCPFlagsBitmaskSingleOctet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1670). **negative:** no negative test. **{single-polarity}:** parseFlowBitmaskMatches hands each term to parseBitmaskTermsText, whose parseBitmaskOperand resolves flag names and numbers to 8-bit values, and numericComponent.Bytes() selects the 1-octet length code for any value <= 0xFF, so an emitted Type-9 bitmask is always 1 octet and no over-long bitmask can be produced to reject (internal/component/bgp/plugins/nlri/flowspec/config_builder.go parseFlowBitmaskMatches, plugin_encode_text.go parseBitmaskTermsText and parseBitmaskOperand, internal/core/bgp/nlri/flowspec/types_numeric.go) |
| `RFC8955-4.2.2.11-1` | Type 11 component values MUST be encoded as single octet (numeric_op len=00). (§4.2.2.11) | MUST | 4.2.2.11 | **positive:** `unit/verify` [`TestRFC8955DSCPValueSingleOctet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1698). **negative:** no negative test. **{single-polarity}:** parseFlowOctets parses DSCP values as uint8 and NewFlowDSCPComponent stores them, so numericComponent.Bytes() always selects the 1-octet length code and no multi-octet DSCP encoding exists to reject (internal/component/bgp/plugins/nlri/flowspec/config_builder.go:271-286, internal/core/bgp/nlri/flowspec/types_numeric.go:446-452) |
| `RFC8955-4.2.2.12-1` | The Type 12 component bitmask MUST be encoded as single octet bitmask (bitmask_op len=00). (§4.2.2.12) | MUST | 4.2.2.12 | **positive:** `unit/verify` [`TestRFC8955FragmentBitmaskSingleOctet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1726). **negative:** no negative test. **{single-polarity}:** fragment values come from the four low-nibble FlowFragmentFlag constants, so numericComponent.Bytes() always selects the 1-octet length code and no multi-octet fragment bitmask can be produced to reject (internal/core/bgp/nlri/flowspec/types.go:196-201, types_numeric.go:458-464) |
| `RFC8955-4.2.2.12-2` | 0: MUST be set to 0 on NLRI encoding and MUST be ignored during decoding (§4.2.2.12) | MUST | 4.2.2.12 | **positive:** `unit/verify` [`TestFlowSpecFragmentReservedHighNibbleZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1477). **positive:** `unit/verify` [`TestRFC8955TextFragmentIPv4ReservedBitsCleared`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/rfc8955_text_encode_reserved_test.go#L74). **negative:** `unit/verify` [`TestRFC8955FragmentReservedBitsIgnoredOnDecode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1643) |
| `RFC8955-6-1` | In the absence of explicit configuration, a Flow Specification NLRI MUST be validated such that it is considered feasible if and only if all of the conditions below are true: (§6, updated by RFC 9117 §4.1) | MUST | 6 | **positive:** `unit/verify` [`TestFlowSpecAuthorizationFromReceivedUpdates`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_flowspec_validation_test.go#L140). **positive:** `unit/verify` [`TestFlowSpecForeignMoreSpecificLosingPathInvalidatesRule`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_flowspec_validation_test.go#L428). **positive:** `unit/verify` [`TestFlowSpecVPNValidationSeparatesAFIAndRD`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_flowspec_validation_test.go#L304). **positive:** `unit/verify` [`TestRFC9117AuthorizationUsesLongestCoveringRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9117_best_match_test.go#L18). **negative:** `unit/verify` [`TestFlowSpecAuthorizationFromReceivedUpdates`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_flowspec_validation_test.go#L141). **negative:** `unit/verify` [`TestFlowSpecForeignMoreSpecificLosingPathInvalidatesRule`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_flowspec_validation_test.go#L429). **negative:** `unit/verify` [`TestFlowSpecVPNValidationSeparatesAFIAndRD`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_flowspec_validation_test.go#L305). **negative:** `unit/verify` [`TestRFC9117AuthorizationUsesLongestCoveringRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9117_best_match_test.go#L19) |
| `RFC8955-6-3` | Therefore, a revalidation of the Flow Specification NLRI MUST be performed whenever unicast routes change. (§6) | MUST | 6 | **positive:** `unit/verify` [`TestFlowSpecUnicastLifecycleRevalidatesRetainedRule`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_flowspec_validation_test.go#L219). **negative:** `unit/verify` [`TestFlowSpecUnicastLifecycleRevalidatesRetainedRule`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_flowspec_validation_test.go#L220) |
| `RFC8955-7.1-1` | On encoding, the traffic-rate MUST NOT be negative. (§7.1, §7.2) | MUST NOT | 7.1 | **positive:** `unit/verify` [`TestParseExtendedCommunitiesTrafficRatePackets`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/route/rfc8955_route_parse_test.go#L1239). **negative:** `unit/verify` [`TestParseExtendedCommunitiesTrafficRatePackets`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/route/rfc8955_route_parse_test.go#L1240) |
| `RFC8955-7.1-2` | On decoding, negative values MUST be treated as zero (discard all traffic). (§7.1, §7.2) | MUST | 7.1 | **positive:** `unit/verify` [`TestRFC8955TrafficRateNegativeDecodesAsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/cli/rfc8955_decode_test.go#L1466). **negative:** `unit/verify` [`TestRFC8955TrafficRateNegativeDecodesAsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/cli/rfc8955_decode_test.go#L1474) |
| `RFC8955-7.3-1` | These bits MUST be set to 0 on encoding and MUST be ignored during decoding. (§7.3) | MUST | 7.3 | **positive:** `unit/verify` [`TestRFC8955TrafficActionBitsDecoded`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc8955_extcomm_decoded_test.go#L243). **positive:** `unit/verify` [`TestRFC8955TrafficActionUnusedBitsIgnoredOnDecode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/cli/rfc8955_decode_test.go#L1511). **positive:** `unit/verify` [`TestRFC8955TrafficActionUnusedBitsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/routeattr_test.go#L209). **negative:** `unit/verify` [`TestRFC8955TrafficActionBitsDecoded`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc8955_extcomm_decoded_test.go#L257). **negative:** `unit/verify` [`TestRFC8955TrafficActionUnusedBitsIgnoredOnDecode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/cli/rfc8955_decode_test.go#L1518). **positive:** `functional/verify` [`community-attributes-json.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/community-attributes-json.ci#L16). **negative:** `functional/verify` [`community-attributes-json.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/community-attributes-json.ci#L17) |
| `RFC8955-7.5-1` | reserved (r): MUST be set to 0 on encoding and MUST be ignored during decoding (§7.5) | MUST | 7.5 | **positive:** `unit/verify` [`TestEncodeRouteEmitsZeroValuedActions`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/rfc8955_encode_test.go#L53). **positive:** `unit/verify` [`TestEncodeRouteMarkKeepsReservedBitsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/rfc8955_encode_test.go#L86). **positive:** `unit/verify` [`TestParseExtendedCommunityMarkDSCPBound`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/rfc8955_routeattr_flowspec_test.go#L34). **positive:** `unit/verify` [`TestRFC8955TrafficMarkingReservedBitsIgnored`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc8955_extcomm_decoded_test.go#L209). **negative:** `unit/verify` [`TestEncodeRouteMarkKeepsReservedBitsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/rfc8955_encode_test.go#L92). **negative:** `unit/verify` [`TestParseExtendedCommunityMarkDSCPBound`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/rfc8955_routeattr_flowspec_test.go#L56). **negative:** `unit/verify` [`TestRFC8955TrafficMarkingReservedBitsIgnored`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc8955_extcomm_decoded_test.go#L216). **positive:** `functional/verify` [`community-attributes-json.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/community-attributes-json.ci#L14). **negative:** `functional/verify` [`community-attributes-json.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/community-attributes-json.ci#L15) |
| `RFC8955-12-1` | Specifications relaxing the validation restrictions MUST contain security considerations that provide details on the required additional filtering. (§12) | MUST | 12 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze publishes no specification introducing a validation relaxation. The implemented RFC 9117 rules are standards-defined; flowSpecAuthorized in internal/component/bgp/plugins/rib/rib_flowspec_validation.go still requires a valid destination and covering unicast reachability, with no configurable destination bypass |
| `RFC8955-5.1-1` | This ordering function is such that it does not depend on the arrival order of the Flow Specification via BGP and thus is consistent in the network. (§5.1) | MUST | 5.1 | **positive:** `unit/verify` [`TestFlowSpecPrecedenceFromWire`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/flowspec/compare_rfc8955_test.go#L9). **negative:** `unit/verify` [`TestFlowSpecPrecedenceFromWire`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/flowspec/compare_rfc8955_test.go#L10) |
| `RFC8955-5.1-2` | The relative order of two Flow Specifications is determined by comparing their respective components. The algorithm starts by comparing the left-most components (lowest component type value) of the Flow Specifications. If the types differ, the Flow Specification with lowest numeric type value has higher precedence (and thus will match before) than the Flow Specification that doesn't contain that component type. If the component types are the same, then a type- specific comparison is performed (see below). If the types are equal, the algorithm continues with the next component. For IP prefix values (IP destination or source prefix), if one of the two prefixes to compare is a more specific prefix of the other, the more specific prefix has higher precedence. Otherwise, the one with the lowest IP value has higher precedence. For all other component types, unless otherwise specified, the comparison is performed by comparing the component data as a binary string using the memcmp() function as defined by [ISO_IEC_9899]. For strings with equal lengths, the lowest string (memcmp) has higher precedence. For strings of different lengths, the common prefix is compared. If the common prefix is not equal, the string with the lowest prefix has higher precedence. If the common prefix is equal, the longest string is considered to have higher precedence than the shorter one. (§5.1) | MUST | 5.1 | **positive:** `unit/verify` [`TestFlowSpecPrecedenceFromWire`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/flowspec/compare_rfc8955_test.go#L11). **positive:** `unit/verify` [`TestSelectedFlowSpecKernelPacketSemantics`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowspec-firewall/rfc8955_selected_integration_linux_test.go#L28). **negative:** `unit/verify` [`TestFlowSpecPrecedenceFromWire`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/flowspec/compare_rfc8955_test.go#L12). **negative:** `unit/verify` [`TestSelectedFlowSpecKernelPacketSemantics`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowspec-firewall/rfc8955_selected_integration_linux_test.go#L29) |
| `RFC8955-8-1` | The NLRI format for this address family consists of a fixed-length Route Distinguisher field (8 octets) followed by the Flow Specification NLRI value (Section 4.2). (§8) | MUST | 8 | **positive:** `unit/verify` [`TestRFC8955VPNNLRIStructure`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/flowspec/rfc8955_vpn_test.go#L27). **negative:** `unit/verify` [`TestRFC8955VPNNLRIStructure`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/flowspec/rfc8955_vpn_test.go#L28) |
| `RFC8955-8-2` | The NLRI length field shall include both the 8 octets of the Route Distinguisher as well as the subsequent Flow Specification NLRI value. (§8) | MUST | 8 | **positive:** `unit/verify` [`TestRFC8955VPNLengthCoversRD`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/flowspec/rfc8955_vpn_test.go#L56). **negative:** `unit/verify` [`TestRFC8955VPNLengthCoversRD`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/flowspec/rfc8955_vpn_test.go#L57) |
| `RFC8955-3-1` | Standard BGP policy mechanisms, such as UPDATE filtering by NLRI prefix as well as community matching, must apply to the Flow specification defined NLRI-type. (§3) | MUST | 3 | **positive:** `unit/verify` [`TestPrefixPolicyFiltersFlowSpecDestination`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/filter_prefix/rfc8955_flowspec_test.go#L12). **positive:** `unit/verify` [`TestRFC8955CommunityPolicyAcceptsAPermittedFlowSpecRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/filter_community_match/rfc8955_flowspec_test.go#L24). **negative:** `unit/verify` [`TestPrefixPolicyFiltersFlowSpecDestination`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/filter_prefix/rfc8955_flowspec_test.go#L13). **negative:** `unit/verify` [`TestRFC8955CommunityPolicyRejectsADeniedFlowSpecRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/filter_community_match/rfc8955_flowspec_test.go#L39) |
| `RFC8955-6-5` | Although the forwarding attributes of two routes for the same Flow Specification prefix may be the same, BGP is still required to perform its path selection algorithm in order to select the correct set of attributes to advertise. (§6) | MUST | 6 | **positive:** `unit/verify` [`TestRFC8955FlowSpecPathSelectionPicksOneSetOfAttributes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8955_flowspec_bestchange_test.go#L79). **negative:** `unit/verify` [`TestRFC8955FlowSpecLosingPathIsNeverPublished`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8955_flowspec_bestchange_test.go#L159) |
| `RFC8955-7.3-2` | The use of the Terminal Action (bit 47) may result in more than one Flow Specification matching a particular traffic flow. All the Traffic Filtering Actions from these Flow Specifications shall be collected and applied. (§7.3) | MUST | 7.3 | **positive:** `unit/verify` [`TestSelectedFlowSpecKernelPacketSemantics`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowspec-firewall/rfc8955_selected_integration_linux_test.go#L30). **negative:** `unit/verify` [`TestSelectedFlowSpecKernelPacketSemantics`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowspec-firewall/rfc8955_selected_integration_linux_test.go#L31) |
| `RFC8955-12-2` | For a network to utilize this relaxation, the BGP policies must support additional filtering since the origin AS field is empty. (§12) | MUST | 12 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** conditional on an absent feature. flowSpecDestination and flowSpecAuthorized in internal/component/bgp/plugins/rib/rib_flowspec_validation.go require a valid destination, including offset zero for IPv6, and no configuration bypasses that guard. The owner explicitly retained strict destination validation |
| `RFC8955-4.2.2.3-1` | Type 3 component values SHOULD be encoded as single octet (numeric_op len=00). (§4.2.2.3) | SHOULD | 4.2.2.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC8955-4.2.2.4-1` | Type 4 component values SHOULD be encoded as 1- or 2-octet quantities (numeric_op len=00 or len=01). (§4.2.2.4) | SHOULD | 4.2.2.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC8955-4.2.2.5-1` | Type 5 component values SHOULD be encoded as 1- or 2-octet quantities (numeric_op len=00 or len=01). (§4.2.2.5) | SHOULD | 4.2.2.5 | **positive:** `unit/verify` [`TestFlowSpecPreferredNumericEmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/rfc8955_width_test.go#L31). **positive:** `unit/verify` [`TestFlowSpecPreferredNumericForwarding`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8955_width_wire_test.go#L25). **negative:** no negative test. **{single-polarity}:** numericComponent.writeToAFI unconditionally chooses a width from the destination-port value, emitting one or two octets for values through 65535; this emission recommendation has no rejecting input branch, and legal wider reception is a separate compatibility control, not negative emission evidence (internal/core/bgp/nlri/flowspec/types_numeric.go). Choosing the shortest of the permitted widths is Ze policy. |
| `RFC8955-4.2.2.6-1` | Type 6 component values SHOULD be encoded as 1- or 2-octet quantities (numeric_op len=00 or len=01). (§4.2.2.6) | SHOULD | 4.2.2.6 | **positive:** `unit/verify` [`TestFlowSpecPreferredNumericEmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/rfc8955_width_test.go#L32). **positive:** `unit/verify` [`TestFlowSpecPreferredNumericForwarding`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8955_width_wire_test.go#L26). **negative:** no negative test. **{single-polarity}:** numericComponent.writeToAFI unconditionally chooses a width from the source-port value, emitting one or two octets for values through 65535; this emission recommendation has no rejecting input branch, and legal wider reception is a separate compatibility control, not negative emission evidence (internal/core/bgp/nlri/flowspec/types_numeric.go). Choosing the shortest of the permitted widths is Ze policy. |
| `RFC8955-4.2.2.7-1` | Type 7 component values SHOULD be encoded as single octet (numeric_op len=00). (§4.2.2.7) | SHOULD | 4.2.2.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC8955-4.2.2.8-1` | Type 8 component values SHOULD be encoded as single octet (numeric_op len=00). (§4.2.2.8) | SHOULD | 4.2.2.8 | **positive:** `unit/verify` [`TestFlowSpecPreferredNumericEmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/rfc8955_width_test.go#L33). **positive:** `unit/verify` [`TestFlowSpecPreferredNumericForwarding`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8955_width_wire_test.go#L27). **negative:** no negative test. **{single-polarity}:** numericComponent.writeToAFI unconditionally emits one octet for ICMP-code values through 255; this emission recommendation has no rejecting input branch, and legal wider reception is a separate compatibility control, not negative emission evidence (internal/core/bgp/nlri/flowspec/types_numeric.go). |
| `RFC8955-4.2.2.10-1` | Type 10 component values SHOULD be encoded as 1- or 2-octet quantities (numeric_op len=00 or len=01). (§4.2.2.10) | SHOULD | 4.2.2.10 | **positive:** `unit/verify` [`TestFlowSpecPreferredNumericEmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/rfc8955_width_test.go#L34). **positive:** `unit/verify` [`TestFlowSpecPreferredNumericForwarding`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8955_width_wire_test.go#L28). **negative:** no negative test. **{single-polarity}:** numericComponent.writeToAFI unconditionally chooses a width from the packet-length value, emitting one or two octets for values through 65535; this emission recommendation has no rejecting input branch, and legal wider reception is a separate compatibility control, not negative emission evidence (internal/core/bgp/nlri/flowspec/types_numeric.go). Choosing the shortest of the permitted widths is Ze policy. |
| `RFC8955-4.2.2.11-2` | The six least significant bits contain the DSCP value. All other bits SHOULD be treated as 0. (§4.2.2.11) | SHOULD | 4.2.2.11 | **positive:** no positive test. **negative:** no negative test |
| `RFC8955-7-1` | Multiple Traffic Filtering Actions defined in this document may be present for a single Flow Specification and SHOULD be applied to the traffic flow (for example, traffic-rate-bytes and rt-redirect can be applied to packets at the same time). (§7) | SHOULD | 7 | **positive:** no positive test. **negative:** no negative test |
| `RFC8955-7.1-3` | The first two octets carry the 2-octet id, which can be assigned from a 2-octet AS number. When a 4-octet AS number is locally present, the 2 least significant octets of such an AS number can be used. This value is purely informational and SHOULD NOT be interpreted by the implementation. (§7.1) | SHOULD NOT | 7.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8955-4.2-3` | All combinations of components within a single Flow Specification are allowed. However, some combinations cannot match any packets (e.g., "ICMP Type AND Port" will never match any packets) and thus SHOULD NOT be propagated by BGP. (§4.2) | SHOULD NOT | 4.2 | **positive:** `unit/verify` [`TestCheckMatchable`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/flowspec/rfc8955_unmatchable_test.go#L183). **positive:** `unit/verify` [`TestFlowSpecUnmatchableByValueReceivedRuleIsNotPropagated`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8955_rib_flowspec_unmatchable_values_test.go#L23). **positive:** `unit/verify` [`TestFlowSpecUnmatchableByValueRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/rfc8955_unmatchable_values_test.go#L27). **positive:** `unit/verify` [`TestFlowSpecUnmatchableReceivedRuleIsNotPropagated`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_flowspec_validation_test.go#L653). **positive:** `unit/verify` [`TestFlowSpecUnmatchableRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/rfc8955_unmatchable_test.go#L52). **negative:** `unit/verify` [`TestCheckMatchable`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/flowspec/rfc8955_unmatchable_test.go#L182). **negative:** `unit/verify` [`TestFlowSpecUnmatchableByValueReceivedRuleIsNotPropagated`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8955_rib_flowspec_unmatchable_values_test.go#L24). **negative:** `unit/verify` [`TestFlowSpecUnmatchableByValueRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/rfc8955_unmatchable_values_test.go#L26). **negative:** `unit/verify` [`TestFlowSpecUnmatchableReceivedRuleIsNotPropagated`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_flowspec_validation_test.go#L654). **negative:** `unit/verify` [`TestFlowSpecUnmatchableReceivedRuleWithdraw`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_flowspec_validation_test.go#L699). **negative:** `unit/verify` [`TestFlowSpecUnmatchableRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/rfc8955_unmatchable_test.go#L34). **negative:** `functional/verify` [`flowspec-unmatchable-refused.ci`](https://github.com/ze-software/ze/blob/main/test/parse/flowspec-unmatchable-refused.ci#L4) |
| `RFC8955-9-1` | While this is an implementation specific choice, implementations SHOULD provide: * A mechanism to log the packet header of filtered traffic. (§9) | SHOULD | 9 | **positive:** no positive test. **negative:** no negative test |
| `RFC8955-9-2` | While this is an implementation specific choice, implementations SHOULD provide: * A mechanism to log the packet header of filtered traffic. * A mechanism to count the number of matches for a given Flow Specification rule. (§9) | SHOULD | 9 | **positive:** no positive test. **negative:** no negative test |
| `RFC8955-7.7-1` | If a Flow Specification associated with interfering Traffic Filtering Actions is selected for packet forwarding, it is an implementation decision which of the interfering Traffic Filtering Actions are selected. Implementors of this specification SHOULD document the behavior of their implementation in such cases. (§7.7) | SHOULD | 7.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC8955-7-2` | Any additional definition of Traffic Filtering Actions SHOULD specify the action to take if those Traffic Filtering Actions interfere (also with existing Traffic Filtering Actions). (§7) | SHOULD | 7 | **positive:** no positive test. **negative:** no negative test |
| `RFC8955-7.1-4` | A traffic-rate of 0 should result on all traffic for the particular flow to be discarded. (§7.1) | SHOULD | 7.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8955-7.2-3` | A traffic-rate-packets of 0 should result in all traffic for the particular flow to be discarded. (§7.2) | SHOULD | 7.2 | **positive:** `unit/verify` [`TestSelectedFlowSpecZeroPacketRateKernel`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowspec-firewall/rfc8955_packet_zero_integration_linux_test.go#L30). **negative:** `unit/verify` [`TestSelectedFlowSpecZeroPacketRateKernel`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowspec-firewall/rfc8955_packet_zero_integration_linux_test.go#L31) |
| `RFC8955-7.6-1` | Implementations should provide mechanisms that map an arbitrary BGP community value (normal or extended) to Traffic Filtering Actions that require different mappings on different systems in the network. (§7.6) | SHOULD | 7.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC8955-6-4` | However, rule a MAY be relaxed by explicit configuration, permitting Flow Specifications that include no destination prefix component. If such is the case, rules b and c are moot and MUST be disregarded. (§6) | MAY | 6 | **positive:** no positive test. **negative:** no negative test |
| `RFC8955-4.2-4` | A given component type MAY (exactly once) be present in the Flow Specification. (§4.2) | MAY | 4.2 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC8955-4-3`](#rfc8955-4-3) Length of the Next-Hop Network Address MUST be set to 0 (§4) | {partial}, Partial proof; remaining gap | tested "MUST be set to 0"; gap "Length of the Next-Hop Network Address"; internal/component/bgp/message/update_build_plugin.go::MPReachCarriesNextHop is the boundary: Tested covers every FlowSpec advertisement that carries no redirect-to-IP-next-hop (0x0800) community, which every rail encodes with a zero-length next hop. The gap is a route carrying the 0x0800 community: MPReachCarriesNextHop returns true for it, so its MP_REACH_NLRI carries the redirect address as the next hop with a non-zero length, an owner-approved deviation of 2026-10-20/21 for draft-simpson-idr-flowspec-redirect-02 Section 3 and ExaBGP compatibility, never counted conformant. internal/component/bgp/reactor/peer_forward_facts.go::precomputeRedirectNextHop is the second producer of the same deviation, on the forward rails: a received route carrying the 0x0800 community and a 4- or 16-octet next hop leaves with that address kept or reset per destination (applyRedirectNextHop), and with length 0 only where the action does not leave with it |
| [`RFC8955-12-1`](#rfc8955-12-1) Specifications relaxing the validation restrictions MUST contain security considerations that provide details on the required additional filtering. (§12) | no test | no test carries this requirement id; annotated {not-applicable}: ze publishes no specification introducing a validation relaxation. The implemented RFC 9117 rules are standards-defined; flowSpecAuthorized in internal/component/bgp/plugins/rib/rib_flowspec_validation.go still requires a valid destination and covering unicast reachability, with no configurable destination bypass |
| [`RFC8955-12-2`](#rfc8955-12-2) For a network to utilize this relaxation, the BGP policies must support additional filtering since the origin AS field is empty. (§12) | no test | no test carries this requirement id; annotated {not-applicable}: conditional on an absent feature. flowSpecDestination and flowSpecAuthorized in internal/component/bgp/plugins/rib/rib_flowspec_validation.go require a valid destination, including offset zero for IPv6, and no configuration bypasses that guard. The owner explicitly retained strict destination validation |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC8955-4-1`](#rfc8955-4-1)

Implementations wishing to exchange Flow Specification MUST use BGP's Capability Advertisement facility to exchange the Multiprotocol Extension Capability Code (Code 1) (§4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestIPv4FlowSpecNegotiatesMultiprotocolCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/plugin_test.go#L903) | unit/verify | unproven |

### [`RFC8955-4-2`](#rfc8955-4-2)

The (AFI, SAFI) pair carried in the Multiprotocol Extension Capability MUST be (AFI=1, SAFI=133) for IPv4 Flow Specification and (AFI=1, SAFI=134) for VPNv4 Flow Specification (§4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestIPv4FlowSpecNegotiatesMultiprotocolCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/plugin_test.go#L904) | unit/verify | unproven |
| positive | [`TestFlowSpecVPNFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L648) | unit/verify | unproven |

### [`RFC8955-4-3`](#rfc8955-4-3)

Length of the Next-Hop Network Address MUST be set to 0 (§4)

Scoped tag-claim records only: tested "MUST be set to 0"; gap "Length of the Next-Hop Network Address"; internal/component/bgp/message/update_build_plugin.go::MPReachCarriesNextHop is the boundary: Tested covers every FlowSpec advertisement that carries no redirect-to-IP-next-hop (0x0800) community, which every rail encodes with a zero-length next hop. The gap is a route carrying the 0x0800 community: MPReachCarriesNextHop returns true for it, so its MP_REACH_NLRI carries the redirect address as the next hop with a non-zero length, an owner-approved deviation of 2026-10-20/21 for draft-simpson-idr-flowspec-redirect-02 Section 3 and ExaBGP compatibility, never counted conformant. internal/component/bgp/reactor/peer_forward_facts.go::precomputeRedirectNextHop is the second producer of the same deviation, on the forward rails: a received route carrying the 0x0800 community and a 4- or 16-octet next hop leaves with that address kept or reset per destination (applyRedirectNextHop), and with length 0 only where the action does not leave with it; zero whole-requirement credit.

Audit verdict: partial (the tests enforce only the declared tested scope; the whole requirement remains unmet), fresh. Rejudged 2026-10-23 by an independent auditor (not the author) after 010e649057, which makes the final writer (session_ownership.go adjOutWrite.allow) send a synthesized withdrawal only to a destination holding an adj-out record for the route. RFC 4271 Section 3.1 allows a withdrawal only for "the IP prefix that expresses the destination for a previously advertised route". RFC 8955 Section 4: "When advertising Flow Specifications, the Length of the Next-Hop Network Address MUST be set to 0." Tested "MUST be set to 0"; Gap "Length of the Next-Hop Network Address"; Producer internal/component/bgp/message/update_build_plugin.go::MPReachCarriesNextHop returns true for the 0x0800 redirect-to-IP-next-hop community, and internal/component/bgp/reactor/peer_forward_facts.go::precomputeRedirectNextHop keeps or resets that address on the forward rails: the owner-approved deviation for draft-simpson-idr-flowspec-redirect-02 Section 3 and ExaBGP compatibility, never counted conformant; the whole unconditional MUST stays unmet with zero whole-requirement credit. The moved unit is TestFlowSpecForwardingKeepsLegacyNextHop (negative tag): its legacy-withdraw subcase now plants the unicast sibling at the destination (advertiseFirst) so the peer-own legacy next-hop gate's withdrawal is legitimate. Assertions unchanged: exactly two recipient UPDATEs; the FlowSpec section is the exact zero-length-next-hop MP_REACH (assertFlowForwardWire) although its received field names the recipient; the legacy section is the exact withdrawal under the gate or the unchanged NEXT_HOP and NLRI otherwise; received observer bytes are unmodified. The other carriers (TestFlowSpecUpdateIgnoresConfiguredNextHop, TestFlowSpecOriginationOmitsConfiguredNextHop, TestFlowSpecForwardingOmitsNextHop, TestFlowSpecRouteServerOmitsNextHop, test/encode/flow-encode.ci) are byte-identical and keep the 2026-10-22 judgement; flow-encode sequences 2/8/9 pin the disclosed redirect deviation and give no conformance credit. Ran: PASS on 2026-10-23.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFlowSpecUpdateIgnoresConfiguredNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/flowspec_wire_contract_test.go#L16) | unit/verify | revert, verified |
| negative | [`TestFlowSpecForwardingOmitsNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/flowspec_forward_wire_test.go#L32) | unit/verify | revert, verified |
| negative | [`TestFlowSpecOriginationOmitsConfiguredNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/flowspec_origin_wire_test.go#L33) | unit/verify | revert, verified |
| negative | [`TestFlowSpecRouteServerOmitsNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/flowspec_rs_wire_test.go#L27) | unit/verify | revert, verified |
| negative | [`TestFlowSpecForwardingKeepsLegacyNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8955_flowspec_forward_mixed_test.go#L16) | unit/verify | revert, verified |
| positive | [`TestFlowSpecUpdateIgnoresConfiguredNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/flowspec_wire_contract_test.go#L15) | unit/verify | revert, verified |
| positive | [`TestFlowSpecForwardingOmitsNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/flowspec_forward_wire_test.go#L31) | unit/verify | revert, verified |
| positive | [`TestFlowSpecOriginationOmitsConfiguredNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/flowspec_origin_wire_test.go#L32) | unit/verify | revert, verified |
| positive | [`TestFlowSpecRouteServerOmitsNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/flowspec_rs_wire_test.go#L26) | unit/verify | revert, verified |
| positive | [`flow-encode.ci`](https://github.com/ze-software/ze/blob/main/test/encode/flow-encode.ci#L15) | functional/verify | revert, verified |

### [`RFC8955-4-4`](#rfc8955-4-4)

Network Address of the Next-Hop field MUST be ignored (§4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8955NextHopIgnoredForFlowSpec`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_flowspec_validation_test.go#L513) | unit/verify | revert, verified |
| positive | [`TestRFC8955NextHopIgnoredForFlowSpec`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_flowspec_validation_test.go#L512) | unit/verify | revert, verified |

### [`RFC8955-4.2-1`](#rfc8955-4.2-1)

Components MUST follow strict type ordering by increasing numerical order (§4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestAddComponentRefusesASecondPrefix`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1388) | unit/verify | unproven |
| negative | [`TestParseFlowSpecRefusesRepeatedComponentType`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1308) | unit/verify | unproven |
| negative | [`TestParseFlowSpecVPNRefusesRepeatedComponentType`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1368) | unit/verify | unproven |
| positive | [`TestFlowSpecComponentsAscendingOrder`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1229) | unit/verify | unproven |
| positive | [`TestFlowSpecJoinsRepeatedTypeIntoOneComponent`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1271) | unit/verify | unproven |

### [`RFC8955-4.2-2`](#rfc8955-4.2-2)

If present, it MUST precede any component of higher numeric type value. (§4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-16 after the codec moved to internal/core/bgp/nlri/flowspec; units changed only by package qualification. RFC 8955 Section 4.2: "If present, it MUST precede any component of higher numeric type value." Positive re-parses an out-of-order build and pins the exact emitted order 1,3,5,12 (core writeComponentsSorted); negative feeds 5-then-3 and requires ErrFlowSpecTypeOrder from core parseComponents, with the same components ascending accepted, so the refusal is isolated to order.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestParseFlowSpecRefusesDescendingComponentType`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1336) | unit/verify | unproven |
| positive | [`TestFlowSpecComponentsAscendingOrder`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1230) | unit/verify | unproven |

### [`RFC8955-4.2.1.1-1`](#rfc8955-4.2.1.1-1)

In the first operator octet of a sequence, it MUST be encoded as unset (§4.2.1.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. independent re-read: TestRFC8955FirstOperatorAndBitEncodedUnset drives config destination-port >8080&<8088 through buildFlowSpecComponents and the encoder, walks the emitted operator octets, and asserts the first carries 0x40 clear and the second 0x40 set; an encoder setting AND on the first octet fails it. The doc-comment edit in 6fac9359ff renamed the config parser only; the body is unchanged.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC8955FirstOperatorAndBitEncodedUnset`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1541) | unit/verify | unproven |

### [`RFC8955-4.2.1.1-2`](#rfc8955-4.2.1.1-2)

In the first operator octet of a sequence, it MUST be encoded as unset and MUST be treated as always unset on decoding. (§4.2.1.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent source rejudgment. RFC 8955 Section 4.2.1.1 (rfc/full/rfc8955.txt:354-356): "In the first operator octet of a sequence, it MUST be encoded as unset and MUST be treated as always unset on decoding." Read both tagged units. internal/component/bgp/plugins/nlri/flowspec/types_test.go::TestRFC8955FirstOperatorAndBitTreatedUnsetOnDecode parses 05 05 01 50 81 16 and the otherwise identical 05 05 41 50 81 16, requires both accepted, pins the clear form to [[=80],[=22]], and requires identical decoded destination-port terms for the set form. The only input difference is the first AND bit; this observes semantic OR grouping, not merely acceptance. The changed quoted sentence now correctly says MUST, not the obsolete SHOULD wording. internal/component/bgp/plugins/nlri/flowspec/rfc8955_text_encode_reserved_test.go::TestRFC8955TextLeadingAndEncodedUnset directly constructs a first FlowMatch with And=true and requires exact wire 03 09 80 02; it also rejects text &syn and requires syn &ack to encode 05 09 00 02 c0 10, so the position rule is not confused with disabling AND everywhere. Producers read: internal/core/bgp/nlri/flowspec/types_numeric.go::writeToAFI clears first AND using i>0, and parseNumericComponent stores first And=false using len(matches)>0; internal/core/bgp/nlri/flowspec/format.go::FormatNumericMatches groups parsed terms; plugin_decode.go flowSpecToJSON/componentToJSON reaches that formatter; plugin_encode_text.go::parseBitmaskTermsText rejects a leading join. Encoding and decoding clauses are both asserted. The decode observation does not independently discriminate removal of the parser guard because the formatter also interprets a leading term independently; that redundant guard break is not claimed as proof of semantic noncompliance. The existing native record for the encoding test halts writeToAFI and proves reach, not a specific AND-bit mutation. No fresh checks or native discrimination run by this auditor; parent owns that proof.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8955FirstOperatorAndBitTreatedUnsetOnDecode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1576) | unit/verify | unproven |
| positive | [`TestRFC8955TextLeadingAndEncodedUnset`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/rfc8955_text_encode_reserved_test.go#L36) | unit/verify | revert, verified |
| positive | [`TestRFC8955FirstOperatorAndBitTreatedUnsetOnDecode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1572) | unit/verify | unproven |

### [`RFC8955-4.2.1.1-3`](#rfc8955-4.2.1.1-3)

0: MUST be set to 0 on NLRI encoding and MUST be ignored during decoding (§4.2.1.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-16: the unit moved verbatim to core compare_rfc8955_test.go, with only the family identifier renamed. RFC 8955 Section 4.2.1.1: "0: MUST be set to 0 on NLRI encoding and MUST be ignored during decoding". Wire op 0xf9 carries the reserved bit 0x08 and a leading AND. The positive requires the exact decoded []FlowMatch{{Op: FlowOpEqual, Value: 1<<56|7}}, so the reserved bit is ignored by core parseNumericComponent's 0x07 mask. The negative requires re-encoding to the exact op 0xb1, so reserved and leading AND are cleared by writeToAFI, and a missing end-of-list bit is refused.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFlowSpecNumericReservedAndEightOctetOperand`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/flowspec/compare_rfc8955_test.go#L43) | unit/verify | revert, verified |
| positive | [`TestFlowSpecNumericReservedAndEightOctetOperand`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/flowspec/compare_rfc8955_test.go#L42) | unit/verify | revert, verified |

### [`RFC8955-4.2.1.2-1`](#rfc8955-4.2.1.2-1)

0 (all 0 bits): MUST be set to 0 on NLRI encoding and MUST be ignored during decoding (§4.2.1.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged again 2026-10-16 after a comment-only edit. The new VALIDATES claim that parseNumericComponent masks 0x0C is TRUE: core types_numeric.go computes compOp = op & 0x03 for FlowTCPFlags and FlowFragment. RFC 8955 Section 4.2.1.2: "0 (all 0 bits): MUST be set to 0 on NLRI encoding and MUST be ignored during" decoding. Encode: assert.Zero(op&0x0C) over every emitted TCP-flags operator (MATCH, NOT, AND+END) goes red if the length, AND, END, NOT or MATCH bits move into 0x0C. The inputs carry no reserved bits, so deleting the 0x03 mask in core writeToAFI alone stays green. That mask is defence in depth: no reachable producer sets 0x0C, because parseBitmaskValue (text) and parseFlowTCPFlagMatches (config) build only NOT/MATCH, and parseNumericComponent masks received ops to 0x03. Decode: op 0x8D must render exactly like 0x81 (=syn) through the plugin JSON path, with the clean render pinned first. The decode proof is at the rendered match, not at the firewall lowering.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8955BitmaskOperatorReservedBitsIgnoredOnDecode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1610) | unit/verify | unproven |
| positive | [`TestFlowSpecBitmaskOperatorReservedBitsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1449) | unit/verify | unproven |

### [`RFC8955-4.2.2.9-1`](#rfc8955-4.2.2.9-1)

Type 9 component bitmasks MUST be encoded as 1- or 2-octet bitmask (bitmask_op len=00 or len=01). (§4.2.2.9)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent rejudge 2026-10-21 after 74626ae4b4 (unit change is the VALIDATES comment only; the producer grammar changed). RFC 8955 Section 4.2.2.9: "Type 9 component bitmasks MUST be encoded as 1- or 2-octet bitmask (bitmask_op len=00 or len=01)." TestRFC8955TCPFlagsBitmaskSingleOctet drives the config producer buildFlowSpecComponents with tcp-flags "syn&!=ack" and "cwr"; under the new grammar that is three terms (syn, AND !=ack, cwr), required exactly, and every emitted bitmask must be non-empty and at most 2 octets. Each operand comes from parseBitmaskOperand as a uint8 (names or parseUint8, 0..255), and writeToAFI takes its width from operandWidth, so a mutation widening tcp-flags to len=10 or len=11 reddens the <=2 assertion. The single-polarity argument still holds: no encode path builds a Type-9 operand above 0xFFFF (text and config are uint8; the decoder refuses valueLen > 2 for FlowTCPFlags, so re-encoding a received value yields at most 2 octets). Finding: the row's {single-polarity} annotation cites parseFlowTCPFlagMatches and config_builder.go:415-447, which 74626ae4b4 deleted; the producer is now parseFlowBitmaskMatches -> parseBitmaskTermsText -> parseBitmaskOperand (plugin_encode_text.go). The argument is true, its citation is stale. Recorded break (writeToAFI panics) proves reach only; the width-specific red is established by reading.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC8955TCPFlagsBitmaskSingleOctet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1670) | unit/verify | revert, verified |

### [`RFC8955-4.2.2.11-1`](#rfc8955-4.2.2.11-1)

Type 11 component values MUST be encoded as single octet (numeric_op len=00). (§4.2.2.11)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-16: the unit changed only in a comment path. RFC 8955 Section 4.2.2.11: "Type 11 component values MUST be encoded as single octet (numeric_op len=00)." Single-polarity positive: DSCP 46, 0 and 63 through the config producer each encode as exactly one octet. Core parseNumericComponent also refuses a DSCP valueLen other than 1 on receipt.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC8955DSCPValueSingleOctet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1698) | unit/verify | unproven |

### [`RFC8955-4.2.2.12-1`](#rfc8955-4.2.2.12-1)

The Type 12 component bitmask MUST be encoded as single octet bitmask (bitmask_op len=00). (§4.2.2.12)

Audit verdict: enforced (the tests do what the requirement demands), fresh. independent re-read: TestRFC8955FragmentBitmaskSingleOctet drives config fragment with all four flags through buildFlowSpecComponents (now parseFlowBitmaskMatches/parseBitmaskTermsText), walks the Type 12 operator list and asserts each of the four values is exactly one octet; a len=01 encoding fails it. Doc comment renamed to the shared bitmask parser on 2026-10-21; the body is unchanged.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC8955FragmentBitmaskSingleOctet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1726) | unit/verify | unproven |

### [`RFC8955-4.2.2.12-2`](#rfc8955-4.2.2.12-2)

0: MUST be set to 0 on NLRI encoding and MUST be ignored during decoding (§4.2.2.12)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent rejudge 2026-10-21 after 74626ae4b4 (unit change in TestRFC8955TextFragmentIPv4ReservedBitsCleared is the VALIDATES comment only; the parser it drives changed). RFC 8955 Section 4.2.2.12, of the four high bits of the fragment bitmask operand: "0: MUST be set to 0 on NLRI encoding and MUST be ignored during decoding". Encode: "fragment 242" and "fragment 255" go through EncodeFlowSpecComponents -> parseFragmentComponentText -> parseBitmaskTermsText -> parseBitmaskOperand, whose parseUint8 accepts 0..255 unmasked, and NewFlowFragmentMatchComponent stores the value, so 0xf2 and 0xff reach core writeToAFI, where `m.Value &= fragmentMask(afi)` (0x0f for IPv4) is the only clearing step. The exact 03 0c 80 02 and 03 0c 80 0f go red (0xf2, 0xff) if that step is removed, and 0x0f also tells the IPv4 mask from the IPv6 0x0e. Config shares parseBitmaskTermsText and writeToAFI. Decode: TestRFC8955FragmentReservedBitsIgnoredOnDecode (unit unchanged) requires 0xF2 to render exactly as 0x02, [[=is-fragment]]; parseNumericComponent masks the value at parse with fragmentMask and formatBitmaskValue also drops undefined bits, so the behaviour is proven at the JSON surface through both guards. TestFlowSpecFragmentReservedHighNibbleZero stays tagged but alone cannot fail (named low-nibble constants only); the row does not depend on it. Recorded break (fragmentMask panics) proves reach; the mask-specific red is established by reading.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8955FragmentReservedBitsIgnoredOnDecode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1643) | unit/verify | unproven |
| positive | [`TestRFC8955TextFragmentIPv4ReservedBitsCleared`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/rfc8955_text_encode_reserved_test.go#L74) | unit/verify | revert, verified |
| positive | [`TestFlowSpecFragmentReservedHighNibbleZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1477) | unit/verify | unproven |

### [`RFC8955-6-1`](#rfc8955-6-1)

In the absence of explicit configuration, a Flow Specification NLRI MUST be validated such that it is considered feasible if and only if all of the conditions below are true: (§6, updated by RFC 9117 §4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent semantic rejudgment: RFC8955 Section 6: "In the absence of explicit configuration, a Flow Specification NLRI MUST be validated such that it is considered feasible if and only if all of the conditions below are true: a) A destination prefix component is embedded in the Flow Specification. b) The originator of the Flow Specification matches the originator of the best-match unicast route for the destination prefix embedded in the Flow Specification (this is the unicast route with the longest possible prefix length covering the destination prefix embedded in the Flow Specification). c) There are no "more-specific" unicast routes, when compared with the flow destination prefix, that have been received from a different neighboring AS than the best-match unicast route, which has been determined in rule b." The existing receive-driven authorization units retain unique IDs, real received attributes and exact eligibility/candidate/selected-event effects. Tests separate longest-covering choice (with RFC9117 amendments), different-AFI/SAFI/RD routes, same-domain foreign-more-specific routes including a losing path, and removal-based reauthorization. Ignoring the losing foreign more-specific now incorrectly authorizes the route and fails the exact forbidden-candidate/install oracle; dropping route-domain comparison fails isolation while the valid matching-domain controls survive. This closes the specifically pending condition-c and route-domain producer counterfactuals without claiming that two faults exhaust all per-clause or dataplane behavior. The preserved broader assertions, not these two mutations alone, support the row. These are inspected scratch semantic executions, not canonical discrimination records. Current native record renewal/stamping remains parent-owned; no pending FRR carrier run, SRPM behavior, dataplane behavior, or full-RFC acceptance is inferred.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9117AuthorizationUsesLongestCoveringRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9117_best_match_test.go#L19) | unit/verify | revert, verified |
| negative | [`TestFlowSpecAuthorizationFromReceivedUpdates`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_flowspec_validation_test.go#L141) | unit/verify | revert, verified |
| negative | [`TestFlowSpecForeignMoreSpecificLosingPathInvalidatesRule`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_flowspec_validation_test.go#L429) | unit/verify | revert, verified |
| negative | [`TestFlowSpecVPNValidationSeparatesAFIAndRD`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_flowspec_validation_test.go#L305) | unit/verify | revert, verified |
| positive | [`TestRFC9117AuthorizationUsesLongestCoveringRoute`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9117_best_match_test.go#L18) | unit/verify | revert, verified |
| positive | [`TestFlowSpecAuthorizationFromReceivedUpdates`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_flowspec_validation_test.go#L140) | unit/verify | revert, verified |
| positive | [`TestFlowSpecForeignMoreSpecificLosingPathInvalidatesRule`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_flowspec_validation_test.go#L428) | unit/verify | revert, verified |
| positive | [`TestFlowSpecVPNValidationSeparatesAFIAndRD`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_flowspec_validation_test.go#L304) | unit/verify | revert, verified |

### [`RFC8955-6-3`](#rfc8955-6-3)

Therefore, a revalidation of the Flow Specification NLRI MUST be performed whenever unicast routes change. (§6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. drives unicast add of a foreign more-specific, its withdraw, loss and return of the covering route, validation eligibility flips and session down, asserting a withdraw or reinstall event for the retained rule at each step without a new FlowSpec UPDATE

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFlowSpecUnicastLifecycleRevalidatesRetainedRule`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_flowspec_validation_test.go#L220) | unit/verify | unproven |
| positive | [`TestFlowSpecUnicastLifecycleRevalidatesRetainedRule`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_flowspec_validation_test.go#L219) | unit/verify | unproven |

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
| negative | [`TestRFC8955TrafficRateNegativeDecodesAsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/cli/rfc8955_decode_test.go#L1474) | unit/verify | unproven |
| positive | [`TestRFC8955TrafficRateNegativeDecodesAsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/cli/rfc8955_decode_test.go#L1466) | unit/verify | unproven |

### [`RFC8955-7.3-1`](#rfc8955-7.3-1)

These bits MUST be set to 0 on encoding and MUST be ignored during decoding. (§7.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. encode: config actions leave octets 2-6 zero and only 0x03 in the last octet; decode: every reserved bit set renders the same as clear for all four S/T combinations, and a pair differing only in S changes the render, so a renderer reading no bit fails; the .ci repeats it end to end to a plugin

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8955TrafficActionUnusedBitsIgnoredOnDecode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/cli/rfc8955_decode_test.go#L1518) | unit/verify | unproven |
| negative | [`TestRFC8955TrafficActionBitsDecoded`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc8955_extcomm_decoded_test.go#L257) | unit/verify | unproven |
| negative | [`community-attributes-json.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/community-attributes-json.ci#L17) | functional/verify | unproven |
| positive | [`TestRFC8955TrafficActionUnusedBitsIgnoredOnDecode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/cli/rfc8955_decode_test.go#L1511) | unit/verify | unproven |
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

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-16: the unit moved verbatim to core compare_rfc8955_test.go, with only the family identifier renamed. RFC 8955 Section 5.1: "This ordering function is such that it does not depend on the arrival order of the Flow Specification via BGP and thus is consistent in the network." Compare is required to be antisymmetric and reflexive (a<b, b>a, a==a) over type, prefix, operator-byte and width pairs. Core Compare reads only component content (no peer, time or action). The arrival-order sort itself is exercised by RFC5575-5.1-1's TestRFC5575PrecedenceIndependentOfArrival, which this row does not tag.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFlowSpecPrecedenceFromWire`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/flowspec/compare_rfc8955_test.go#L10) | unit/verify | revert, verified |
| positive | [`TestFlowSpecPrecedenceFromWire`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/flowspec/compare_rfc8955_test.go#L9) | unit/verify | revert, verified |

### [`RFC8955-5.1-2`](#rfc8955-5.1-2)

The relative order of two Flow Specifications is determined by comparing their respective components. The algorithm starts by comparing the left-most components (lowest component type value) of the Flow Specifications. If the types differ, the Flow Specification with lowest numeric type value has higher precedence (and thus will match before) than the Flow Specification that doesn't contain that component type. If the component types are the same, then a type- specific comparison is performed (see below). If the types are equal, the algorithm continues with the next component. For IP prefix values (IP destination or source prefix), if one of the two prefixes to compare is a more specific prefix of the other, the more specific prefix has higher precedence. Otherwise, the one with the lowest IP value has higher precedence. For all other component types, unless otherwise specified, the comparison is performed by comparing the component data as a binary string using the memcmp() function as defined by [ISO_IEC_9899]. For strings with equal lengths, the lowest string (memcmp) has higher precedence. For strings of different lengths, the common prefix is compared. If the common prefix is not equal, the string with the lowest prefix has higher precedence. If the common prefix is equal, the longest string is considered to have higher precedence than the shorter one. (§5.1)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/plugins/flowspec-firewall/rfc8955_selected_integration_linux_test.go::TestSelectedFlowSpecKernelPacketSemantics, internal/plugins/flowspec-firewall/rfc8955_selected_integration_linux_test.go::TestSelectedFlowSpecKernelPacketSemantics#2 moved. Re-judged 2026-10-16: the compare unit moved verbatim to core and the flowspec-firewall kernel unit is unchanged. RFC 8955 Section 5.1: "The relative order of two Flow Specifications is determined by comparing their respective components." TestFlowSpecPrecedenceFromWire pins, with forward and reverse signs and self equality, each of these: a lower type first, a present-versus-absent next component, a more-specific overlapping prefix first, a lower disjoint prefix first, equal-width memcmp, and received-width bytes over numeric value. TestSelectedFlowSpecKernelPacketSemantics adds Linux packet evidence that the more-specific terminal rule wins over a covering discard in either install order. That unit needs integration+linux privileges, and a skipped run is not runtime proof. The clause "If the common prefix is equal, the longest string is considered to have higher precedence" has no pair. It is unreachable for well-formed numeric or bitmask data: the end-of-list bit sits in the operator octet, so one valid component's data can never be a proper prefix of another's.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFlowSpecPrecedenceFromWire`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/flowspec/compare_rfc8955_test.go#L12) | unit/verify | revert, verified |
| negative | [`TestSelectedFlowSpecKernelPacketSemantics`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowspec-firewall/rfc8955_selected_integration_linux_test.go#L29) | unit/verify | revert, verified |
| positive | [`TestFlowSpecPrecedenceFromWire`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/flowspec/compare_rfc8955_test.go#L11) | unit/verify | revert, verified |
| positive | [`TestSelectedFlowSpecKernelPacketSemantics`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowspec-firewall/rfc8955_selected_integration_linux_test.go#L28) | unit/verify | revert, verified |

### [`RFC8955-8-1`](#rfc8955-8-1)

The NLRI format for this address family consists of a fixed-length Route Distinguisher field (8 octets) followed by the Flow Specification NLRI value (Section 4.2). (§8)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-16: the unit moved verbatim to core rfc8955_vpn_test.go, with only the family identifier renamed (test-local AFI 1/SAFI 134). RFC 8955 Section 8: "The NLRI format for this address family consists of a fixed-length Route Distinguisher field (8 octets) followed by the Flow Specification NLRI value (Section 4.2)." The positive decodes RD 100:100 from the 8 octets after the length and the destination component after them, and requires the exact wire back from WriteTo. The negative requires ErrFlowSpecTruncated from core ParseFlowSpecVPN for a length too short to hold the RD.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8955VPNNLRIStructure`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/flowspec/rfc8955_vpn_test.go#L28) | unit/verify | revert, verified |
| positive | [`TestRFC8955VPNNLRIStructure`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/flowspec/rfc8955_vpn_test.go#L27) | unit/verify | revert, verified |

### [`RFC8955-8-2`](#rfc8955-8-2)

The NLRI length field shall include both the 8 octets of the Route Distinguisher as well as the subsequent Flow Specification NLRI value. (§8)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-16: the unit moved verbatim to core, with only the family identifier renamed. RFC 8955 Section 8: "The NLRI length field shall include both the 8 octets of the Route Distinguisher as well as the subsequent Flow Specification NLRI value." The positive requires Len() and the WriteTo length octet to equal 8 plus the value length; its first assertion checks the fixture against itself, and the WriteTo assertion is the real check. The negative requires ErrFlowSpecTruncated for the same RD and value under a length that counts only the value.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8955VPNLengthCoversRD`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/flowspec/rfc8955_vpn_test.go#L57) | unit/verify | revert, verified |
| positive | [`TestRFC8955VPNLengthCoversRD`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/flowspec/rfc8955_vpn_test.go#L56) | unit/verify | revert, verified |

### [`RFC8955-3-1`](#rfc8955-3-1)

Standard BGP policy mechanisms, such as UPDATE filtering by NLRI prefix as well as community matching, must apply to the Flow specification defined NLRI-type. (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Both halves now tagged both ways. Community half: TestRFC8955CommunityPolicyAcceptsAPermittedFlowSpecRoute (ipv4/flow and ipv4/flow-vpn with 65001:100 accepted) and TestRFC8955CommunityPolicyRejectsADeniedFlowSpecRoute (denied community first-match and implicit deny reject a FlowSpec route); revert records on evaluateCommunities. Prefix half: filter_prefix flowspec_test.go pair (accept, reject, missing, mixed), no record. Both at the plugin entry point: no .ci drives a FlowSpec UPDATE through a configured community filter in the running daemon. Rejudged 2026-10-17 (independent reader) after the owner-approved 6019dd0675 change to TestPrefixPolicyFiltersFlowSpecDestination: the filter input prefix changed from nlri to mp, the section keyword the filter text now uses for MP_REACH families. The cases are unchanged: ipv4/flow and ipv4/flow-vpn destinations inside 10.0.0.0/8 ge 16 le 24 must be accepted, and /25, out-of-range, invalid and mixed destinations rejected, with no unicast rewrite. A parser that stopped reading the mp form would fail the accept cases, and a policy that stopped applying would fail the reject cases. Package re-run green 2026-10-17. RFC 8955 Section 3: "Standard BGP policy mechanisms, such as UPDATE filtering by NLRI prefix as well as community matching, must apply to the Flow specification defined NLRI-type." The community units are unchanged. Verdict stands.

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

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/plugins/flowspec-firewall/rfc8955_selected_integration_linux_test.go::TestSelectedFlowSpecKernelPacketSemantics, internal/plugins/flowspec-firewall/rfc8955_selected_integration_linux_test.go::TestSelectedFlowSpecKernelPacketSemantics#2 moved. RFC 8955 Section 7.3: "The use of the Terminal Action (bit 47) may result in more than one Flow Specification matching a particular traffic flow. All the Traffic Filtering Actions from these Flow Specifications shall be collected and applied." The same paragraph leaves interfering-action selection implementation-defined. TestSelectedFlowSpecKernelPacketSemantics now observes a received packet with the first rule's DSCP 10 AND exactly one increment of the later rule's sample counter; switching only the narrow rule to T-clear leaves the sample counter unchanged. Earlier controls prove T-set reaches a covering discard and that marking does not change the original DSCP used by later matching. rule_chains.go ruleChains/beforeMarkActions/markingTerms separate deferred marking and continuing action execution. Omitting the first action, the later action, or continuation fails independently, closing the earlier mark-plus-discard ambiguity. Requires a real privileged integration+linux execution; skips are not proof. Independent source rejudgment only; no test, mutation, build or gate was executed by this auditor. Native observed-red renewal is a separate parent step.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSelectedFlowSpecKernelPacketSemantics`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowspec-firewall/rfc8955_selected_integration_linux_test.go#L31) | unit/verify | revert, verified |
| positive | [`TestSelectedFlowSpecKernelPacketSemantics`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowspec-firewall/rfc8955_selected_integration_linux_test.go#L30) | unit/verify | revert, verified |

### [`RFC8955-12-2`](#rfc8955-12-2)

For a network to utilize this relaxation, the BGP policies must support additional filtering since the origin AS field is empty. (§12)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8955-12-2, so no unit is bound to it.

### [`RFC8955-4.2.2.5-1`](#rfc8955-4.2.2.5-1)

Type 5 component values SHOULD be encoded as 1- or 2-octet quantities (numeric_op len=00 or len=01). (§4.2.2.5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-16: the emission unit changed only by package qualification (constructors and types now from core flowspec), and the reactor forwarding unit is unchanged. RFC 8955 Section 4.2.2.5: "Type 5 component values SHOULD be encoded as 1- or 2-octet quantities (numeric_op len=00 or len=01)." TestFlowSpecPreferredNumericEmission requires exact literal operator and value bytes at 0/255/256/65535 from typed, generic, registered config (ConfigRouteParserByFamily) and registered text (EncodeNLRIByFamily) producers, and from the EncodeRoute route command's MP_REACH. TestFlowSpecPreferredNumericForwarding carries the recipient TCP observation. Single-polarity as annotated, and the shortest width is Ze policy. The legal wide-reception compatibility control, which this row does not tag, moved to core numeric_wide_reception_test.go::TestFlowSpecPreferredNumericWideReception. Second re-judgement 2026-10-16: TestFlowSpecPreferredNumericEmission's typed packet-length producer is now the test helper packetLengthComponent (plugin types_test.go), which builds FlowMatch{Op: FlowOpEqual, Value: v} through NewFlowNumericComponent(FlowPacketLength, ...). That is byte-for-byte the matches that the deleted NewFlowPacketLengthComponent built at HEAD (plugin types_numeric.go:457-463). The deleted constructor had no non-test caller at HEAD, so no production producer lost coverage. The typed and generic cases now drive the same constructor, and the registered config, text and forwarding producers carry the distinct evidence.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestFlowSpecPreferredNumericEmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/rfc8955_width_test.go#L31) | unit/verify | revert, verified |
| positive | [`TestFlowSpecPreferredNumericForwarding`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8955_width_wire_test.go#L25) | unit/verify | revert, verified |

### [`RFC8955-4.2.2.6-1`](#rfc8955-4.2.2.6-1)

Type 6 component values SHOULD be encoded as 1- or 2-octet quantities (numeric_op len=00 or len=01). (§4.2.2.6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-16: same units and same change as RFC8955-4.2.2.5-1. RFC 8955 Section 4.2.2.6: "Type 6 component values SHOULD be encoded as 1- or 2-octet quantities (numeric_op len=00 or len=01)." Source-port emission requires exact one- and two-octet operands at 0/255/256/65535 across the typed, generic, config, text and route-command producers, plus recipient forwarding. Single-polarity as annotated. Second re-judgement 2026-10-16: TestFlowSpecPreferredNumericEmission's typed packet-length producer is now the test helper packetLengthComponent (plugin types_test.go), which builds FlowMatch{Op: FlowOpEqual, Value: v} through NewFlowNumericComponent(FlowPacketLength, ...). That is byte-for-byte the matches that the deleted NewFlowPacketLengthComponent built at HEAD (plugin types_numeric.go:457-463). The deleted constructor had no non-test caller at HEAD, so no production producer lost coverage. The typed and generic cases now drive the same constructor, and the registered config, text and forwarding producers carry the distinct evidence.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestFlowSpecPreferredNumericEmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/rfc8955_width_test.go#L32) | unit/verify | revert, verified |
| positive | [`TestFlowSpecPreferredNumericForwarding`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8955_width_wire_test.go#L26) | unit/verify | revert, verified |

### [`RFC8955-4.2.2.8-1`](#rfc8955-4.2.2.8-1)

Type 8 component values SHOULD be encoded as single octet (numeric_op len=00). (§4.2.2.8)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-16: same units and same change as RFC8955-4.2.2.5-1. RFC 8955 Section 4.2.2.8: "Type 8 component values SHOULD be encoded as single octet (numeric_op len=00)." ICMP-code emission requires the exact single octet at 0 and 255 across the typed (NewFlowICMPCodeComponent, now exported from core), generic, config and text producers, plus recipient forwarding. The route-command grammar has no ICMP-code criterion. Single-polarity as annotated. Second re-judgement 2026-10-16: TestFlowSpecPreferredNumericEmission's typed packet-length producer is now the test helper packetLengthComponent (plugin types_test.go), which builds FlowMatch{Op: FlowOpEqual, Value: v} through NewFlowNumericComponent(FlowPacketLength, ...). That is byte-for-byte the matches that the deleted NewFlowPacketLengthComponent built at HEAD (plugin types_numeric.go:457-463). The deleted constructor had no non-test caller at HEAD, so no production producer lost coverage. The typed and generic cases now drive the same constructor, and the registered config, text and forwarding producers carry the distinct evidence.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestFlowSpecPreferredNumericEmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/rfc8955_width_test.go#L33) | unit/verify | revert, verified |
| positive | [`TestFlowSpecPreferredNumericForwarding`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8955_width_wire_test.go#L27) | unit/verify | revert, verified |

### [`RFC8955-4.2.2.10-1`](#rfc8955-4.2.2.10-1)

Type 10 component values SHOULD be encoded as 1- or 2-octet quantities (numeric_op len=00 or len=01). (§4.2.2.10)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-16: same units and same change as RFC8955-4.2.2.5-1. RFC 8955 Section 4.2.2.10: "Type 10 component values SHOULD be encoded as 1- or 2-octet quantities (numeric_op len=00 or len=01)." Packet-length emission requires exact one- and two-octet operands at 0/255/256/65535 across the typed, generic, config and text producers, plus recipient forwarding. Single-polarity as annotated, and the shortest width is local policy. Second re-judgement 2026-10-16: TestFlowSpecPreferredNumericEmission's typed packet-length producer is now the test helper packetLengthComponent (plugin types_test.go), which builds FlowMatch{Op: FlowOpEqual, Value: v} through NewFlowNumericComponent(FlowPacketLength, ...). That is byte-for-byte the matches that the deleted NewFlowPacketLengthComponent built at HEAD (plugin types_numeric.go:457-463). The deleted constructor had no non-test caller at HEAD, so no production producer lost coverage. The typed and generic cases now drive the same constructor, and the registered config, text and forwarding producers carry the distinct evidence.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestFlowSpecPreferredNumericEmission`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/rfc8955_width_test.go#L34) | unit/verify | revert, verified |
| positive | [`TestFlowSpecPreferredNumericForwarding`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8955_width_wire_test.go#L28) | unit/verify | revert, verified |

### [`RFC8955-4.2-3`](#rfc8955-4.2-3)

All combinations of components within a single Flow Specification are allowed. However, some combinations cannot match any packets (e.g., "ICMP Type AND Port" will never match any packets) and thus SHOULD NOT be propagated by BGP. (§4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-21 (independent auditor, authored none of 445b2e05f1 or 3bc47af4f2) after CheckMatchable gained the value checks in unmatchable_values.go. RFC 8955 Section 4.2: "A specific packet is considered to match the Flow Specification when it matches the intersection (AND) of all the components present in the Flow Specification." and "However, some combinations cannot match any packets (e.g., "ICMP Type AND Port" will never match any packets) and thus SHOULD NOT be propagated by BGP." The earlier weak finding is closed: checkPortValues now refuses port beside destination-port and source-port when port meets neither (Section 4.2.2.4: "This component matches if either the destination port OR the source port of an IP packet matches the value."); the reasoning is sound, since with each list non-empty a packet (d,s) exists exactly when port meets destination-port or source-port, and with one of the two absent the free port always satisfies port. Also new: one numeric list admitting no value its field holds (AND groups, values above port/ICMP/IPv4 packet-length/DSCP/flow-label/protocol widths), an IPv6 next-header admitting only RFC 8200 extension headers (RFC 8956 Section 3.3), contradictory TCP-flag terms, and a fragment list admitting none of the four packet states; the states match the Section 4.2.2.12 bit definitions (LF needs offset not 0, so FF AND LF and FF AND IsF admit nothing). Tests assert exact outcomes: TestCheckMatchable runs every row built, from the wire, as VPN and VPN from the wire with errors.Is equality, each refusal beside its nearest matchable control (port 80 / dst 443 / src 443 refused vs dst 80 or src 80 accepted vs src-port absent accepted; >1000 vs <=1000 with src 1000-or-1001 accepted; >100&<50 vs >100&<500 vs >100&<50-or-80; dscp 64 vs 63; next-header 44-or-43 vs 44-or-6; syn&!syn vs syn&!ack; FF&IsF vs FF-or-IsF); TestFlowSpecUnmatchableByValueRefused drives the config and update-text paths for the port, AND-group and TCP-flag classes; TestFlowSpecUnmatchableByValueReceivedRuleIsNotPropagated pins receive-side ineligibility, no install event and no candidate for the port, AND-group, DSCP, fragment and TCP-flag classes with eligible controls; the ICMP/protocol/fragment pair carriers and flowspec-unmatchable-refused.ci are unchanged in meaning. Polarity pairs are genuine and isolate the rule. Not judged a miss: lower bounds on packet-length against header sizes (the RFC names no such case, and its own "unable to locate the transport header" clause is implementation-dependent). Types-only callers (ExaBGP bridge, migration) pass nil matches and get the type checks only; the FlowSpec is judged again with values when built. The .ci revert record staled by the producer change was re-taken.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFlowSpecUnmatchableRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/rfc8955_unmatchable_test.go#L34) | unit/verify | revert, verified |
| negative | [`TestFlowSpecUnmatchableByValueRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/rfc8955_unmatchable_values_test.go#L26) | unit/verify | revert, verified |
| negative | [`TestFlowSpecUnmatchableByValueReceivedRuleIsNotPropagated`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8955_rib_flowspec_unmatchable_values_test.go#L24) | unit/verify | revert, verified |
| negative | [`TestFlowSpecUnmatchableReceivedRuleIsNotPropagated`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_flowspec_validation_test.go#L654) | unit/verify | revert, verified |
| negative | [`TestFlowSpecUnmatchableReceivedRuleWithdraw`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_flowspec_validation_test.go#L699) | unit/verify | revert, verified |
| negative | [`TestCheckMatchable`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/flowspec/rfc8955_unmatchable_test.go#L182) | unit/verify | revert, verified |
| negative | [`flowspec-unmatchable-refused.ci`](https://github.com/ze-software/ze/blob/main/test/parse/flowspec-unmatchable-refused.ci#L4) | functional/verify | revert, verified |
| positive | [`TestFlowSpecUnmatchableRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/rfc8955_unmatchable_test.go#L52) | unit/verify | revert, verified |
| positive | [`TestFlowSpecUnmatchableByValueRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/rfc8955_unmatchable_values_test.go#L27) | unit/verify | revert, verified |
| positive | [`TestFlowSpecUnmatchableByValueReceivedRuleIsNotPropagated`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc8955_rib_flowspec_unmatchable_values_test.go#L23) | unit/verify | revert, verified |
| positive | [`TestFlowSpecUnmatchableReceivedRuleIsNotPropagated`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_flowspec_validation_test.go#L653) | unit/verify | revert, verified |
| positive | [`TestCheckMatchable`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/flowspec/rfc8955_unmatchable_test.go#L183) | unit/verify | revert, verified |

### [`RFC8955-7.2-3`](#rfc8955-7.2-3)

A traffic-rate-packets of 0 should result in all traffic for the particular flow to be discarded. (§7.2)

Audit verdict: enforced (the tests do what the requirement demands), stale-unit: internal/plugins/flowspec-firewall/rfc8955_packet_zero_integration_linux_test.go::TestSelectedFlowSpecZeroPacketRateKernel, internal/plugins/flowspec-firewall/rfc8955_packet_zero_integration_linux_test.go::TestSelectedFlowSpecZeroPacketRateKernel#2 moved. Literal packet-rate community 0x800c zero reaches the selected callback and real nft backend, with four target drops, exact four-packet/116-byte counters, unaffected-destination and positive-rate controls; the repaired sequence reinstalls zero, sends target token 7 and control token 8, requires only 8 plus a timeout, then withdraws that active discard and requires token 6 delivery, so stale zero-rate enforcement is now distinguishable from removal. Independent source judgment: FlowWidthZeroWholeReview. Current restored numeric/forwarding race observation: job-flowspec-numeric-restored-after-mutations-c672abd1.log. Width and legal-wide reception semantic negatives: 267ffab7, b0675576, 4cb46ab9, d65876fe, 73bd5cbc, 85fdc2cb, 567fcfe2, adf991fd. Packet-zero negative: job-flowspec-semantic-packet-zero-kernel-9c4ece52.log; repaired non-skipped runtime-kernel positive: job-flowspec-zero-restored-guest-path-637874ca.log. Direct selected-callback entry is not subscription-delivery proof; local IPv4 input is not VPN or transit dataplane coverage. Re-judged 2026-10-13 after 2ce54b1f91: both discard-probe checks moved from err.(net.Error) to errors.AsType[net.Error](err); ReadFromUDP returns *net.OpError, which satisfies net.Error at the top of the chain, so each timeout assertion accepts and refuses exactly the errors it did before and the verdict is unchanged.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSelectedFlowSpecZeroPacketRateKernel`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowspec-firewall/rfc8955_packet_zero_integration_linux_test.go#L31) | unit/verify | mutant, verified |
| positive | [`TestSelectedFlowSpecZeroPacketRateKernel`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowspec-firewall/rfc8955_packet_zero_integration_linux_test.go#L30) | unit/verify | mutant, verified |

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
