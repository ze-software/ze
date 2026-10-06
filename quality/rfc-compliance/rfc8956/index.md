# RFC 8956 - Dissemination of Flow Specification Rules for IPv6

Partial. Every requirement this repository extracted from RFC 8956, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 77.8% | 7 of 9 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 22.2% | 2 of 9 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 9 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 9 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| No test at all | 0.0% | 0 of 9 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 16.7% | 3 of 18 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 9 | of 14 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 9 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 9 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 9 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 9 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

The 8 shares marked as a part above are the whole of the 9 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 14 |
| Gated MUST-level | 9 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 18 |
| Tagged units | 18 |
| Recorded audit verdicts | 7 |
| Discrimination records | 3 |
| Summary | `rfc/short/rfc8956.md` |
| Requirement shard | `rfc/requirements/rfc8956.md` |
| RFC text | `rfc/full/rfc8956.txt` |

## Enrolment

Enrolled: Dissemination of Flow Specification Rules for IPv6; native AFI 2 FlowSpec and VPN FlowSpec encoding, decoding, policy and retained-route validation are implemented in Ze's BGP path.

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

- AFI 2 / SAFI 133 and 134 capability negotiation and native NLRI handling
- offset-aware prefix patterns with checked bounds and padding normalization
- family-specific fragment reserved-bit masking and single-octet encoding
- destination-offset-zero validation against the matching IPv6 unicast or VPN RIB. Source and regression carriers are present
- integration validation remains outstanding.


**What the ledger says remains**

Linux firewall packet verification requires the privileged integration carrier. The global firewall bridge refuses unsupported predicates, including nonzero source offsets, fragments and Flow Labels, rather than installing a broader filter. VPN FlowSpec is validated and propagated but not installed in the global firewall. These disclosures are not a conformance or coverage-count claim.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 7 | one part of the gated population |
| Annotated (including scoped evidence) | 2 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **9** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (7):** [`RFC8956-3.1-1`](#rfc8956-3.1-1), [`RFC8956-3.1-2`](#rfc8956-3.1-2), [`RFC8956-3.6-1`](#rfc8956-3.6-1), [`RFC8956-3.6-2`](#rfc8956-3.6-2), [`RFC8956-3.6-3`](#rfc8956-3.6-3), [`RFC8956-3.1-3`](#rfc8956-3.1-3), [`RFC8956-5-1`](#rfc8956-5-1)

**Annotated (including scoped evidence) (2):** [`RFC8956-2-1`](#rfc8956-2-1), [`RFC8956-2-2`](#rfc8956-2-2)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC8956-2-1` | Implementations wishing to exchange IPv6 Flow Specifications MUST use BGP's Capability Advertisement facility to exchange the Multiprotocol Extension Capability Code (Code 1) (§2) | MUST | 2 | **positive:** `unit/verify` [`TestIPv6FlowSpecNegotiatesMultiprotocolCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/plugin_test.go#L856). **negative:** no negative test. **{single-polarity}:** the flowspec plugin unconditionally maps its declared ipv6/flow decode family to a Multiprotocol capability during OPEN; there is no wrong input the negotiation path rejects, so no negative case exists |
| `RFC8956-2-2` | The (AFI, SAFI) pair carried in the Multiprotocol Extension Capability MUST be (AFI=2, SAFI=133) for IPv6 Flow Specification rules and (AFI=2, SAFI=134) for L3VPN (§2) | MUST | 2 | **positive:** `unit/verify` [`TestFlowSpecIPv6Basic`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L439). **positive:** `unit/verify` [`TestFlowSpecVPNFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L654). **negative:** no negative test. **{single-polarity}:** the (AFI 2, SAFI 133/134) assignment is a family-registration constant, not an input guard; the code accepts no alternative value it could reject, so only the positive assignment is assertable |
| `RFC8956-3.1-1` | Padding bits MUST be 0 on encoding (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestIPv6FlowPrefixPatternAndPadding`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/rfc8956_prefix_test.go#L11). **negative:** `unit/verify` [`TestIPv6FlowPrefixPatternAndPadding`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/rfc8956_prefix_test.go#L12) |
| `RFC8956-3.1-2` | Padding bits MUST be 0 on encoding and MUST be ignored on decoding. (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestIPv6FlowPrefixPatternAndPadding`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/rfc8956_prefix_test.go#L13). **negative:** `unit/verify` [`TestIPv6FlowPrefixPatternAndPadding`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/rfc8956_prefix_test.go#L14) |
| `RFC8956-3.6-1` | The Type 12 component bitmask MUST be encoded as a single octet bitmask (bitmask_op len=00). (§3.6) | MUST | 3.6 | **positive:** `unit/verify` [`TestRFC8956FragmentSingleOctet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/rfc8956_fragment_test.go#L10). **negative:** `unit/verify` [`TestRFC8956FragmentSingleOctet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/rfc8956_fragment_test.go#L11) |
| `RFC8956-3.6-2` | 0: MUST be set to 0 on NLRI encoding (§3.6) | MUST | 3.6 | **positive:** `unit/verify` [`TestRFC8956FragmentEncodingRespectsEnclosingFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/rfc8956_fragment_test.go#L45). **negative:** `unit/verify` [`TestRFC8956FragmentEncodingRespectsEnclosingFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/rfc8956_fragment_test.go#L46) |
| `RFC8956-3.6-3` | 0: MUST be set to 0 on NLRI encoding and MUST be ignored during decoding (§3.6) | MUST | 3.6 | **positive:** `unit/verify` [`TestRFC8956FragmentDecodeIgnoresReservedBits`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/rfc8956_fragment_test.go#L69). **negative:** `unit/verify` [`TestRFC8956FragmentDecodeIgnoresReservedBits`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/rfc8956_fragment_test.go#L70) |
| `RFC8956-3.1-3` | If length = 0 and offset = 0, this component matches every address; otherwise, length MUST be in the range offset < length < 129 or the component is malformed. (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestIPv6FlowPrefixRejectsMalformedBounds`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/rfc8956_prefix_test.go#L44). **positive:** `unit/verify` [`TestRFC8956ZeroLengthZeroOffsetMatchesEveryAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowspec-firewall/rfc8956_zero_prefix_test.go#L26). **negative:** `unit/verify` [`TestIPv6FlowPrefixRejectsMalformedBounds`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/rfc8956_prefix_test.go#L45) |
| `RFC8956-5-1` | item a) of the validation procedure should now read as follows: \| a) A destination prefix component with offset=0 is embedded in \| the Flow Specification (§5) | MUST | 5 | **positive:** `unit/verify` [`TestFlowSpecVPNValidationSeparatesAFIAndRD`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_flowspec_validation_test.go#L292). **negative:** `unit/verify` [`TestFlowSpecVPNValidationSeparatesAFIAndRD`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_flowspec_validation_test.go#L293) |
| `RFC8956-3.3-1` | Type 3 component values SHOULD be encoded as a single octet (numeric_op len=00). (§3.3) | SHOULD | 3.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC8956-3.4-1` | Type 7 component values SHOULD be encoded as a single octet (numeric_op len=00). (§3.4) | SHOULD | 3.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC8956-3.5-1` | Type 8 component values SHOULD be encoded as a single octet (numeric_op len=00). (§3.5) | SHOULD | 3.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC8956-3.7-1` | Type 13 component values SHOULD be encoded as 4-octet quantities (numeric_op len=10). (§3.7) | SHOULD | 3.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC8956-6.1-1` | If several local instances match this criteria, the choice between them is a local matter (for example, the instance with the lowest Route Distinguisher value can be elected). (§6.1) | MAY | 6.1 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

RFC 8956 declares no gap, and every gated MUST it carries has a test bound to it.

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC8956-2-1`](#rfc8956-2-1)

Implementations wishing to exchange IPv6 Flow Specifications MUST use BGP's Capability Advertisement facility to exchange the Multiprotocol Extension Capability Code (Code 1) (§2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestIPv6FlowSpecNegotiatesMultiprotocolCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/plugin_test.go#L856) | unit/verify | unproven |

### [`RFC8956-2-2`](#rfc8956-2-2)

The (AFI, SAFI) pair carried in the Multiprotocol Extension Capability MUST be (AFI=2, SAFI=133) for IPv6 Flow Specification rules and (AFI=2, SAFI=134) for L3VPN (§2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestFlowSpecIPv6Basic`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L439) | unit/verify | unproven |
| positive | [`TestFlowSpecVPNFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L654) | unit/verify | unproven |

### [`RFC8956-3.1-1`](#rfc8956-3.1-1)

Padding bits MUST be 0 on encoding (§3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Non-compliant: nonzero padding bits emitted. TestIPv6FlowPrefixPatternAndPadding re-encodes received patterns whose padding is 0xff and asserts fs.Bytes() and WriteTo equal the canonical zero-padded bytes (e.g. 0xff -> 0xfe for 72/65), and asserts a configured 2001:db8:ffff::/33 encodes its last octet as 0x80 with host bits cleared. Both polarities present.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestIPv6FlowPrefixPatternAndPadding`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/rfc8956_prefix_test.go#L12) | unit/verify | unproven |
| positive | [`TestIPv6FlowPrefixPatternAndPadding`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/rfc8956_prefix_test.go#L11) | unit/verify | unproven |

### [`RFC8956-3.1-2`](#rfc8956-3.1-2)

Padding bits MUST be 0 on encoding and MUST be ignored on decoding. (§3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. The quote carries both clauses. Encoding: the canonical-bytes assertions described for RFC8956-3.1-1 go red if padding is emitted nonzero. Decoding: inputs with padding 0xff must parse without error and comp.Prefix() must equal the prefix built from pattern bits only (::7f00:0:0:0/72, 103:8000::/17); a decoder that read padding into the match or rejected nonzero padding fails require.NoError or the Prefix assertion.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestIPv6FlowPrefixPatternAndPadding`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/rfc8956_prefix_test.go#L14) | unit/verify | unproven |
| positive | [`TestIPv6FlowPrefixPatternAndPadding`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/rfc8956_prefix_test.go#L13) | unit/verify | unproven |

### [`RFC8956-3.6-1`](#rfc8956-3.6-1)

The Type 12 component bitmask MUST be encoded as a single octet bitmask (bitmask_op len=00). (§3.6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Non-compliant: a fragment bitmask encoded wider than one octet. TestRFC8956FragmentSingleOctet encodes a value of ^uint64(0) through IPv6 FlowSpec and VPN producers and asserts the exact bytes {12, 0x81, 0x0e} (len=00, one octet) via WriteTo and Bytes(); the negative requires ParseFlowSpec/ParseFlowSpecVPN to error on a 0x91 (len=01) two-octet operand.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8956FragmentSingleOctet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/rfc8956_fragment_test.go#L11) | unit/verify | unproven |
| positive | [`TestRFC8956FragmentSingleOctet`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/rfc8956_fragment_test.go#L10) | unit/verify | unproven |

### [`RFC8956-3.6-2`](#rfc8956-3.6-2)

0: MUST be set to 0 on NLRI encoding (§3.6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Non-compliant: a reserved bit set in an encoded IPv6 fragment operand. TestRFC8956FragmentEncodingRespectsEnclosingFamily adds DF|IsF and asserts the IPv6 and IPv6-VPN encodings carry 2 (DF bit cleared) while IPv4 carries 3; IsF/FF/LF emit 2, 4, 8. High reserved bits are cleared in the 3.6-1 unit (0x0e from all-ones).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8956FragmentEncodingRespectsEnclosingFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/rfc8956_fragment_test.go#L46) | unit/verify | unproven |
| positive | [`TestRFC8956FragmentEncodingRespectsEnclosingFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/rfc8956_fragment_test.go#L45) | unit/verify | unproven |

### [`RFC8956-3.6-3`](#rfc8956-3.6-3)

0: MUST be set to 0 on NLRI encoding and MUST be ignored during decoding (§3.6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. The quote carries both clauses. Decoding: TestRFC8956FragmentDecodeIgnoresReservedBits parses operands 0xf3, 0xf5, 0xf9, 0xf1 (every reserved bit set) without error and asserts Matches() is IsF, FF, LF, 0; a decoder that kept or rejected reserved bits fails. Encoding: the re-encoded NLRI must equal want with reserved bits zeroed (2, 4, 8, 0).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8956FragmentDecodeIgnoresReservedBits`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/rfc8956_fragment_test.go#L70) | unit/verify | unproven |
| positive | [`TestRFC8956FragmentDecodeIgnoresReservedBits`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/rfc8956_fragment_test.go#L69) | unit/verify | unproven |

### [`RFC8956-3.1-3`](#rfc8956-3.1-3)

If length = 0 and offset = 0, this component matches every address; otherwise, length MUST be in the range offset < length < 129 or the component is malformed. (§3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-02 (BGP c31 judge). RFC 8956 Section 3.1: "If length = 0 and offset = 0, this component matches every address; otherwise, length MUST be in the range offset < length < 129 or the component is malformed." First clause: flowspec-firewall rfc8956_zero_prefix_test.go::TestRFC8956ZeroLengthZeroOffsetMatchesEveryAddress parses wire 03 01 00 00 and 03 02 00 00 through ParseFlowSpec and requires the firewall matches MatchDestinationAddress{::/0} and MatchSourceAddress{::/0} exactly (its address-containment loop asserts on the ::/0 constant and adds nothing; the require.Equal on the match is the proof). Second clause: HEAD TestIPv6FlowPrefixRejectsMalformedBounds refuses 0/1, 64/64, 63/64, 129/0 and accepts 80/64. Judge overlay: extractPrefix refusing a 0-bit prefix turns the new positive red; revert record on extractPrefix.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestIPv6FlowPrefixRejectsMalformedBounds`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/rfc8956_prefix_test.go#L45) | unit/verify | unproven |
| positive | [`TestIPv6FlowPrefixRejectsMalformedBounds`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/rfc8956_prefix_test.go#L44) | unit/verify | unproven |
| positive | [`TestRFC8956ZeroLengthZeroOffsetMatchesEveryAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowspec-firewall/rfc8956_zero_prefix_test.go#L26) | unit/verify | revert, verified |

### [`RFC8956-5-1`](#rfc8956-5-1)

item a) of the validation procedure should now read as follows: | a) A destination prefix component with offset=0 is embedded in | the Flow Specification (§5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent semantic rejudgment: RFC8956 Section 5: "A destination prefix component with offset=0 is embedded in the Flow Specification." Section 5 retains RFC8955 validation but replaces item a with this offset-zero amendment. Current TestFlowSpecVPNValidationSeparatesAFIAndRD uses a legal /64 offset32 four-byte pattern, the same RD/default authorizing route, and unchanged wire input. It requires retained receipt but false generation-7 eligibility, zero candidates and no additional selected event; the zero-offset rule succeeds. Removing only the Offset()==0 gate now fails that consumer oracle and clean controls pass. The pattern shape is legal under Section 3.1 length-minus-offset; rejection is authorization, not malformed encoding. Enforcement is for this amendment, not blanket inherited-rule or dataplane compliance. These are inspected scratch semantic executions, not canonical discrimination records. Current native record renewal/stamping remains parent-owned; no pending FRR carrier run, SRPM behavior, dataplane behavior, or full-RFC acceptance is inferred.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFlowSpecVPNValidationSeparatesAFIAndRD`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_flowspec_validation_test.go#L293) | unit/verify | revert, verified |
| positive | [`TestFlowSpecVPNValidationSeparatesAFIAndRD`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_flowspec_validation_test.go#L292) | unit/verify | revert, verified |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | prose |
| Source | rfc/full/rfc8956.txt |
| Source fingerprint | 68a638fb8b5a0b2b |
| Record | rfc/extraction/rfc8956.json |
| Mapped sentences | 6 |
| Declined as scope | 5 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 1 | walked | not stated |
| `1` | not stated | 1 | walked | not stated |
| `1.1` | not stated | 0 | walked | not stated |
| `2` | not stated | 2 | walked | not stated |
| `3` | not stated | 0 | walked | not stated |
| `3.1` | not stated | 4 | walked | not stated |
| `3.2` | not stated | 0 | walked | not stated |
| `3.3` | not stated | 0 | walked | not stated |
| `3.4` | not stated | 0 | walked | not stated |
| `3.5` | not stated | 0 | walked | not stated |
| `3.6` | not stated | 2 | walked | not stated |
| `3.7` | not stated | 0 | walked | not stated |
| `3.8` | not stated | 0 | walked | not stated |
| `3.8.1` | not stated | 0 | walked | not stated |
| `3.8.2` | not stated | 0 | walked | not stated |
| `4` | not stated | 0 | walked | not stated |
| `5` | not stated | 0 | walked | not stated |
| `6` | not stated | 0 | walked | not stated |
| `6.1` | not stated | 0 | walked | not stated |
| `7` | not stated | 1 | walked | not stated |
| `8` | not stated | 0 | walked | not stated |
| `8.1` | not stated | 0 | walked | not stated |
| `8.1.1` | not stated | 0 | walked | not stated |
| `8.1.2` | not stated | 0 | walked | not stated |
| `8.2` | not stated | 0 | walked | not stated |
| `9` | not stated | 0 | walked | not stated |
| `A` | not stated | 0 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `front:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | IETF Trust boilerplate on the Simplified BSD License for extracted Code Components; it binds republication of the document, not any protocol behaviour. | Code Components extracted from this document must include Simplified BSD License text as described in Section 4.e of the Trust Legal Provisions and are provided without warranty as described in the Simplified BSD License. |
| `1:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Scope statement in the Introduction. 'the delta changes required to support IPv6' is lowercase and describes what this document contains, not an obligation on an implementation. | It only defines the delta changes required to support IPv6, while all other definitions and operation mechanisms of "Dissemination of Flow Specification Rules" will remain in the main specification and will not be repeated here. |
| `3.1:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Rationale for the offset field: 'where one is required to skip over the first N bits' is lowercase and explains why the field exists. | The offset has been defined to allow for flexible matching to portions of an IPv6 address where one is required to skip over the first N bits of the address. |
| `3.1:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Field definition of 'padding' in the Section 3.1 encoding list: it states how many bits the field holds. The obligation on those bits is the next sentence, site 3.1:3. | padding: This contains the minimum number of bits required to pad the component to an octet boundary. |
| `7:1` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | The 'first fragment must include the upper-layer header' obligation is Section 4.5 of RFC 8200, addressed to the packet's source. RFC 8956 cites it only to explain why a Type 3 component cannot be enforced on malformed packets. | [RFC7112] describes the impact of oversized IPv6 header chains when trying to match on the transport header; Section 4.5 of [RFC8200] also requires that the first fragment must include the upper-layer header, but there could be wrongly formatted packets not respecting [RFC8200]. |

## Superseded

No document obsoletes RFC 8956, so its obligations are stated where they were written.
