# RFC 8956 - Dissemination of Flow Specification Rules for IPv6

IPv6 FlowSpec. Every requirement listed for RFC 8956, the tests linked to it, and their recorded reviews. This summary is included in the automatic compliance check, ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 7 | of 9 mandatory requirements | a positive test checks that Ze does what the requirement demands and a negative one checks that it refuses what the requirement forbids |
| One polarity plus reason | 2 | of 9 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason supplies the complete test coverage required |
| One polarity, unexcused | 0 | of 9 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial test coverage | 0 | of 9 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unverified, with zero whole-requirement credit |
| Declared gap or no test | 0 | of 9 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |
| Tests shown to detect a fault | 13 | 13 of 18 tests linked to requirements | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests verifies a requirement only after both tests have been shown to detect a fault |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 9 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 0 | of 9 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 0 | of 9 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze verified: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 9 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 9 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no test verification credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level) | 9 | of 14 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements included in the automatic checks: the compliance check (./le rfc check) holds Ze to each of them. This is the total being assessed; the counts above show the recorded status of those requirements. |

The 9 counts marked as a part above are the whole of the 9 mandatory requirements: they add up to 9. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

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
| Public row | IPv6 FlowSpec |
| Automatic checks | Included |
| Requirements | 14 |
| Mandatory requirements (MUST level) | 9 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 18 |
| Tests linked to requirements | 18 |
| Recorded audit verdicts | 7 |
| Fault test records | 13 |
| Summary | `rfc/short/rfc8956.md` |
| Requirement table | `rfc/requirements/rfc8956.md` |
| RFC text | `rfc/full/rfc8956.txt` |

## Automatic checks

Included in automatic checks: Dissemination of Flow Specification Rules for IPv6; native AFI 2 FlowSpec and VPN FlowSpec encoding, decoding, policy and retained-route validation are implemented in Ze's BGP path.

## Published support status

**Area:** IPv6 FlowSpec

**Test verification:** 7 of 9 MUST proven

**Evidence:** unit 7

**Unmet:** -


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 7 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 2 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 0 | one part of the mandatory requirements |
| Partial test coverage (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level)** | **9** | every mandatory requirement appears in exactly one group above |

**Positive and negative tests (7):** [`RFC8956-3.1-1`](#rfc8956-3.1-1), [`RFC8956-3.1-2`](#rfc8956-3.1-2), [`RFC8956-3.6-1`](#rfc8956-3.6-1), [`RFC8956-3.6-2`](#rfc8956-3.6-2), [`RFC8956-3.6-3`](#rfc8956-3.6-3), [`RFC8956-3.1-3`](#rfc8956-3.1-3), [`RFC8956-5-1`](#rfc8956-5-1)

**Annotated (including scoped evidence) (2):** [`RFC8956-2-1`](#rfc8956-2-1), [`RFC8956-2-2`](#rfc8956-2-2)

## Figures

- **MUST bar**: Ze verified 7, Tests awaiting verification 2
- **Overview bar**: Ze MUST verified 7, Tests awaiting verification 2, Not assessed 5

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | not complete |
| MUST rows | 9 |
| MUST rows kept (the MUST bar) | 9 |
| Ze verified | 7 |
| OS verified | 0 |
| Tests awaiting verification | 2 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 0 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 5 |
| SHOULD verified | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD tests awaiting verification | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 5 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze verified means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Tests awaiting verification counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

Source: [data/rfc/rfc8956.json](../../../data/rfc/rfc8956.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC8956-2-1` | Implementations wishing to exchange IPv6 Flow Specifications MUST use BGP's Capability Advertisement facility to exchange the Multiprotocol Extension Capability Code (Code 1) (§2) | MUST | 2 | **positive:** `unit/verify` [`TestIPv6FlowSpecNegotiatesMultiprotocolCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/plugin_test.go#L865). **negative:** no negative test. **{single-polarity}:** the flowspec plugin unconditionally maps its declared ipv6/flow decode family to a Multiprotocol capability during OPEN; there is no wrong input the negotiation path rejects, so no negative case exists |
| `RFC8956-2-2` | The (AFI, SAFI) pair carried in the Multiprotocol Extension Capability MUST be (AFI=2, SAFI=133) for IPv6 Flow Specification rules and (AFI=2, SAFI=134) for L3VPN (§2) | MUST | 2 | **positive:** `unit/verify` [`TestFlowSpecIPv6Basic`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L440). **positive:** `unit/verify` [`TestFlowSpecVPNFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L655). **negative:** no negative test. **{single-polarity}:** the (AFI 2, SAFI 133/134) assignment is a family-registration constant, not an input guard; the code accepts no alternative value it could reject, so only the positive assignment is assertable |
| `RFC8956-3.1-1` | Padding bits MUST be 0 on encoding (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestIPv6FlowPrefixPatternAndPadding`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/flowspec/rfc8956_prefix_test.go#L11). **negative:** `unit/verify` [`TestIPv6FlowPrefixPatternAndPadding`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/flowspec/rfc8956_prefix_test.go#L12) |
| `RFC8956-3.1-2` | Padding bits MUST be 0 on encoding and MUST be ignored on decoding. (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestIPv6FlowPrefixPatternAndPadding`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/flowspec/rfc8956_prefix_test.go#L13). **negative:** `unit/verify` [`TestIPv6FlowPrefixPatternAndPadding`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/flowspec/rfc8956_prefix_test.go#L14) |
| `RFC8956-3.6-1` | The Type 12 component bitmask MUST be encoded as a single octet bitmask (bitmask_op len=00). (§3.6) | MUST | 3.6 | **positive:** `unit/verify` [`TestRFC8956FragmentSingleOctet`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/flowspec/rfc8956_fragment_test.go#L10). **negative:** `unit/verify` [`TestRFC8956FragmentSingleOctet`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/flowspec/rfc8956_fragment_test.go#L11) |
| `RFC8956-3.6-2` | 0: MUST be set to 0 on NLRI encoding (§3.6) | MUST | 3.6 | **positive:** `unit/verify` [`TestRFC8956FragmentEncodingRespectsEnclosingFamily`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/flowspec/rfc8956_fragment_test.go#L45). **negative:** `unit/verify` [`TestRFC8956FragmentEncodingRespectsEnclosingFamily`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/flowspec/rfc8956_fragment_test.go#L46) |
| `RFC8956-3.6-3` | 0: MUST be set to 0 on NLRI encoding and MUST be ignored during decoding (§3.6) | MUST | 3.6 | **positive:** `unit/verify` [`TestRFC8956FragmentDecodeIgnoresReservedBits`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/flowspec/rfc8956_fragment_test.go#L69). **negative:** `unit/verify` [`TestRFC8956FragmentDecodeIgnoresReservedBits`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/flowspec/rfc8956_fragment_test.go#L70) |
| `RFC8956-3.1-3` | If length = 0 and offset = 0, this component matches every address; otherwise, length MUST be in the range offset < length < 129 or the component is malformed. (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestIPv6FlowPrefixRejectsMalformedBounds`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/rfc8956_prefix_test.go#L12). **positive:** `unit/verify` [`TestRFC8956ZeroLengthZeroOffsetMatchesEveryAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowspec-firewall/rfc8956_zero_prefix_test.go#L26). **negative:** `unit/verify` [`TestIPv6FlowPrefixRejectsMalformedBounds`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/rfc8956_prefix_test.go#L13) |
| `RFC8956-5-1` | item a) of the validation procedure should now read as follows: \| a) A destination prefix component with offset=0 is embedded in \| the Flow Specification (§5) | MUST | 5 | **positive:** `unit/verify` [`TestFlowSpecVPNValidationSeparatesAFIAndRD`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_flowspec_validation_test.go#L306). **negative:** `unit/verify` [`TestFlowSpecVPNValidationSeparatesAFIAndRD`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_flowspec_validation_test.go#L307) |
| `RFC8956-3.3-1` | Type 3 component values SHOULD be encoded as a single octet (numeric_op len=00). (§3.3) | SHOULD | 3.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC8956-3.4-1` | Type 7 component values SHOULD be encoded as a single octet (numeric_op len=00). (§3.4) | SHOULD | 3.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC8956-3.5-1` | Type 8 component values SHOULD be encoded as a single octet (numeric_op len=00). (§3.5) | SHOULD | 3.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC8956-3.7-1` | Type 13 component values SHOULD be encoded as 4-octet quantities (numeric_op len=10). (§3.7) | SHOULD | 3.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC8956-6.1-1` | If several local instances match this criteria, the choice between them is a local matter (for example, the instance with the lowest Route Distinguisher value can be elected). (§6.1) | MAY | 6.1 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

RFC 8956 lists no missing behavior, and every mandatory requirement included in automatic checks has a linked test.

## Test status

A test reads not verified when no record shows it failing after a deliberate fault. Until then, the test has not been shown to detect a fault in the required behavior.

### [`RFC8956-2-1`](#rfc8956-2-1)

Implementations wishing to exchange IPv6 Flow Specifications MUST use BGP's Capability Advertisement facility to exchange the Multiprotocol Extension Capability Code (Code 1) (§2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestIPv6FlowSpecNegotiatesMultiprotocolCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/plugin_test.go#L865) | unit/verify | not verified |

### [`RFC8956-2-2`](#rfc8956-2-2)

The (AFI, SAFI) pair carried in the Multiprotocol Extension Capability MUST be (AFI=2, SAFI=133) for IPv6 Flow Specification rules and (AFI=2, SAFI=134) for L3VPN (§2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestFlowSpecIPv6Basic`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L440) | unit/verify | not verified |
| positive | [`TestFlowSpecVPNFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L655) | unit/verify | not verified |

### [`RFC8956-3.1-1`](#rfc8956-3.1-1)

Padding bits MUST be 0 on encoding (§3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-16: TestIPv6FlowPrefixPatternAndPadding moved verbatim to core internal/core/bgp/nlri/flowspec/rfc8956_prefix_test.go, with only the family identifier renamed (test-local AFI 2/SAFI 133). RFC 8956 Section 3.1: "Padding bits MUST be 0 on encoding and MUST be ignored on decoding." Encode clause: received patterns padded with 0xff must re-encode through both Bytes() and WriteTo to the canonical zero-padded bytes (0xff becomes 0xfe for 72/65, 0xff becomes 0xc0 for 17/7), and a configured 2001:db8:ffff::/33 must encode its last pattern octet as exactly 0x80.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestIPv6FlowPrefixPatternAndPadding`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/flowspec/rfc8956_prefix_test.go#L12) | unit/verify | revert, verified |
| positive | [`TestIPv6FlowPrefixPatternAndPadding`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/flowspec/rfc8956_prefix_test.go#L11) | unit/verify | revert, verified |

### [`RFC8956-3.1-2`](#rfc8956-3.1-2)

Padding bits MUST be 0 on encoding and MUST be ignored on decoding. (§3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-16: same unit as RFC8956-3.1-1, moved verbatim to core. The quote carries both clauses. Encoding is proven by the canonical-bytes assertions. Decoding: inputs with 0xff padding must parse without error, and comp.Prefix() must equal the pattern-only prefix (::7f00:0:0:0/72, 103:8000::/17) with the exact offset. A decoder that read padding into the match, or rejected nonzero padding, fails require.NoError or the Prefix assertion.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestIPv6FlowPrefixPatternAndPadding`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/flowspec/rfc8956_prefix_test.go#L14) | unit/verify | revert, verified |
| positive | [`TestIPv6FlowPrefixPatternAndPadding`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/flowspec/rfc8956_prefix_test.go#L13) | unit/verify | revert, verified |

### [`RFC8956-3.6-1`](#rfc8956-3.6-1)

The Type 12 component bitmask MUST be encoded as a single octet bitmask (bitmask_op len=00). (§3.6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-16: the unit moved to core rfc8956_fragment_test.go. It changed only by the family identifiers and the now-exported NewFlowFragmentMatchComponent. RFC 8956 Section 3.6: "The Type 12 component bitmask MUST be encoded as a single octet bitmask (bitmask_op len=00)." A value of ^uint64(0) through the IPv6 FlowSpec and IPv6 VPN producers must encode exactly {12, 0x81, 0x0e} (len=00, one octet) via WriteTo and Bytes(), so core writeToAFI applies fragmentMask before choosing the width. The negative requires ParseFlowSpec/ParseFlowSpecVPN to refuse a 0x91 (len=01) two-octet operand.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC8956FragmentSingleOctet`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/flowspec/rfc8956_fragment_test.go#L11) | unit/verify | revert, verified |
| positive | [`TestRFC8956FragmentSingleOctet`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/flowspec/rfc8956_fragment_test.go#L10) | unit/verify | revert, verified |

### [`RFC8956-3.6-2`](#rfc8956-3.6-2)

0: MUST be set to 0 on NLRI encoding (§3.6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-16: the unit moved verbatim to core, with only the family identifiers renamed. RFC 8956 Section 3.6: "0: MUST be set to 0 on NLRI encoding". One shared component carrying DF|IsF, FF and LF must encode IPv6 and IPv6-VPN as 2, 4, 8 (DF, reserved in Figure 1, cleared), while IPv4 and IPv4-VPN keep 3. Core fragmentMask(AFIIPv6)=0x0e is what clears it. The high reserved bits are covered by the 3.6-1 unit.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC8956FragmentEncodingRespectsEnclosingFamily`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/flowspec/rfc8956_fragment_test.go#L46) | unit/verify | revert, verified |
| positive | [`TestRFC8956FragmentEncodingRespectsEnclosingFamily`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/flowspec/rfc8956_fragment_test.go#L45) | unit/verify | revert, verified |

### [`RFC8956-3.6-3`](#rfc8956-3.6-3)

0: MUST be set to 0 on NLRI encoding and MUST be ignored during decoding (§3.6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-16: the unit moved verbatim to core, with only the family identifiers renamed. The quote carries both clauses. Decoding: operands 0xf3, 0xf5, 0xf9, 0xf1, each carrying every reserved bit, must parse without error for IPv6 and IPv6-VPN, and Matches() must equal exactly IsF, FF, LF, 0. Encoding: the re-encoded NLRI must equal the input with reserved bits zeroed (2, 4, 8, 0).

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC8956FragmentDecodeIgnoresReservedBits`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/flowspec/rfc8956_fragment_test.go#L70) | unit/verify | revert, verified |
| positive | [`TestRFC8956FragmentDecodeIgnoresReservedBits`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/flowspec/rfc8956_fragment_test.go#L69) | unit/verify | revert, verified |

### [`RFC8956-3.1-3`](#rfc8956-3.1-3)

If length = 0 and offset = 0, this component matches every address; otherwise, length MUST be in the range offset < length < 129 or the component is malformed. (§3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-16: both units changed only by package qualification. The plugin unit now calls core flowspec.ParseFlowSpec and flowspec.KwDestinationIPv6; the firewall unit names the IPv6 FlowSpec family as a family.Family literal. RFC 8956 Section 3.1: "If length = 0 and offset = 0, this component matches every address; otherwise, length MUST be in the range offset < length < 129 or the component is malformed." First clause: flowspec-firewall TestRFC8956ZeroLengthZeroOffsetMatchesEveryAddress parses 03 01 00 00 and 03 02 00 00 and requires the firewall matches to be exactly MatchDestinationAddress{::/0} and MatchSourceAddress{::/0}. Its containment loop checks the constant and adds nothing, so the require.Equal is the proof. Second clause: TestIPv6FlowPrefixRejectsMalformedBounds requires parse errors for 0/1, 64/64, 63/64, 129/0 and a truncated pattern, and config refusal for /64/64, /64/65 and /0/1. It also requires 80/64 to encode and compare equal to its received form.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestIPv6FlowPrefixRejectsMalformedBounds`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/rfc8956_prefix_test.go#L13) | unit/verify | not verified |
| positive | [`TestIPv6FlowPrefixRejectsMalformedBounds`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/rfc8956_prefix_test.go#L12) | unit/verify | not verified |
| positive | [`TestRFC8956ZeroLengthZeroOffsetMatchesEveryAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowspec-firewall/rfc8956_zero_prefix_test.go#L26) | unit/verify | revert, verified |

### [`RFC8956-5-1`](#rfc8956-5-1)

item a) of the validation procedure should now read as follows: | a) A destination prefix component with offset=0 is embedded in | the Flow Specification (§5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent semantic rejudgment: RFC8956 Section 5: "A destination prefix component with offset=0 is embedded in the Flow Specification." Section 5 retains RFC8955 validation but replaces item a with this offset-zero amendment. Current TestFlowSpecVPNValidationSeparatesAFIAndRD uses a legal /64 offset32 four-byte pattern, the same RD/default authorizing route, and unchanged wire input. It requires retained receipt but false generation-7 eligibility, zero candidates and no additional selected event; the zero-offset rule succeeds. Removing only the Offset()==0 gate now fails that consumer oracle and clean controls pass. The pattern shape is legal under Section 3.1 length-minus-offset; rejection is authorization, not malformed encoding. Enforcement is for this amendment, not blanket inherited-rule or dataplane compliance. These are inspected scratch semantic executions, not canonical discrimination records. Current native record renewal/stamping remains parent-owned; no pending FRR carrier run, SRPM behavior, dataplane behavior, or full-RFC acceptance is inferred.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestFlowSpecVPNValidationSeparatesAFIAndRD`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_flowspec_validation_test.go#L307) | unit/verify | revert, verified |
| positive | [`TestFlowSpecVPNValidationSeparatesAFIAndRD`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_flowspec_validation_test.go#L306) | unit/verify | revert, verified |

## Requirement list review

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
| `front` | not stated | 1 | reviewed | not stated |
| `1` | not stated | 1 | reviewed | not stated |
| `1.1` | not stated | 0 | reviewed | not stated |
| `2` | not stated | 2 | reviewed | not stated |
| `3` | not stated | 0 | reviewed | not stated |
| `3.1` | not stated | 4 | reviewed | not stated |
| `3.2` | not stated | 0 | reviewed | not stated |
| `3.3` | not stated | 0 | reviewed | not stated |
| `3.4` | not stated | 0 | reviewed | not stated |
| `3.5` | not stated | 0 | reviewed | not stated |
| `3.6` | not stated | 2 | reviewed | not stated |
| `3.7` | not stated | 0 | reviewed | not stated |
| `3.8` | not stated | 0 | reviewed | not stated |
| `3.8.1` | not stated | 0 | reviewed | not stated |
| `3.8.2` | not stated | 0 | reviewed | not stated |
| `4` | not stated | 0 | reviewed | not stated |
| `5` | not stated | 0 | reviewed | not stated |
| `6` | not stated | 0 | reviewed | not stated |
| `6.1` | not stated | 0 | reviewed | not stated |
| `7` | not stated | 1 | reviewed | not stated |
| `8` | not stated | 0 | reviewed | not stated |
| `8.1` | not stated | 0 | reviewed | not stated |
| `8.1.1` | not stated | 0 | reviewed | not stated |
| `8.1.2` | not stated | 0 | reviewed | not stated |
| `8.2` | not stated | 0 | reviewed | not stated |
| `9` | not stated | 0 | reviewed | not stated |
| `A` | not stated | 0 | reviewed | not stated |

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
