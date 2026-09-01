# RFC 8956 - Dissemination of Flow Specification Rules for IPv6

Partial. Every requirement this repository extracted from RFC 8956, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0.0% | 0 of 9 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 22.2% | 2 of 9 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 9 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 0.0% | 0 of 3 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 9 | of 14 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 9 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 77.8% | 7 of 9 binding obligations | no test carries the requirement id, whether or not a gap states why |

The 4 shares marked as a part above are the whole of the 9 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 14 |
| Gated MUST-level | 9 |
| Obligations that bind Ze | 9 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 7 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 3 |
| Tagged units | 3 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc8956.md` |
| Requirement shard | `rfc/requirements/rfc8956.md` |
| RFC text | `rfc/full/rfc8956.txt` |

## Enrolment

Enrolled: Dissemination of Flow Specification Rules for IPv6: nine MUST-level requirements. Two are met in internal/component/bgp/plugins/nlri/flowspec: 2-1 (IPv6 FlowSpec negotiates the (AFI 2, SAFI 133) Multiprotocol capability) via a new test, and 2-2 (IPv6 FlowSpec uses AFI 2 with SAFI 133, and SAFI 134 for VPN) with explicit AFI/SAFI assertions -- both {single-polarity: positive}. Seven are {gap}: ze encodes and decodes IPv6 FlowSpec NLRI structurally but lacks the conformance guards -- 3.1-1 and 3.1-2 (prefix padding not zeroed on encode and not masked on decode), 3.1-3 (offset/length not validated and the pattern encoded from the address start, correct only for offset 0), 3.6-1 (minimal-octet encoding incidental), 3.6-2 and 3.6-3 (the Fragment component uses the RFC 8955 IPv4 bitmask layout and does not handle the IPv6 reserved bits), and 5-1 (no validation-against-unicast procedure). Disclosed in the docs/features/rfc-status.md RFC 8956 row.

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

- IPv6 FlowSpec and FlowSpec VPN NLRI structural encode/decode (AFI 2, SAFI 133/134): prefix, numeric, bitmask and Flow Label components, round-trip, family registration, and (AFI 2, SAFI 133/134) Multiprotocol capability negotiation
- tests bound per requirement in [`rfc/requirements/rfc8956.md`](https://github.com/ze-software/ze/blob/main/rfc/requirements/rfc8956.md).


**What the ledger says remains**

Seven MUST-level gaps, each annotated in [`rfc/short/rfc8956.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc8956.md): [`RFC8956-3.1-1`](#rfc8956-3.1-1) -- the prefix encoder does not zero sub-byte padding beyond the prefix length; [`RFC8956-3.1-2`](#rfc8956-3.1-2) -- the decoder retains padding bits without masking; [`RFC8956-3.1-3`](#rfc8956-3.1-3) -- offset/length (offset < length <= 128) is not validated and the pattern is encoded from the address start, correct only for offset 0; [`RFC8956-3.6-1`](#rfc8956-3.6-1) -- minimal single-octet encoding is incidental, not enforced; [`RFC8956-3.6-2`](#rfc8956-3.6-2) -- the Fragment component uses the RFC 8955 IPv4 bitmask layout and does not zero the IPv6 reserved bits on transmit; [`RFC8956-3.6-3`](#rfc8956-3.6-3) -- the Fragment decoder reads the raw byte without masking the IPv6 reserved bits; and [`RFC8956-5-1`](#rfc8956-5-1) -- no Section 5 / RFC 8955 Section 6 flowspec validation-against-unicast procedure is implemented.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 0 | one part of the gated population |
| Annotated instead of tested | 9 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **9** | every gated MUST falls in exactly one bucket above |

**Annotated instead of tested (9):** [`RFC8956-2-1`](#rfc8956-2-1), [`RFC8956-2-2`](#rfc8956-2-2), [`RFC8956-3.1-1`](#rfc8956-3.1-1), [`RFC8956-3.1-2`](#rfc8956-3.1-2), [`RFC8956-3.6-1`](#rfc8956-3.6-1), [`RFC8956-3.6-2`](#rfc8956-3.6-2), [`RFC8956-3.6-3`](#rfc8956-3.6-3), [`RFC8956-3.1-3`](#rfc8956-3.1-3), [`RFC8956-5-1`](#rfc8956-5-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC8956-2-1` | Implementations wishing to exchange IPv6 Flow Specifications MUST use BGP's Capability Advertisement facility to exchange the Multiprotocol Extension Capability Code (Code 1) (§2) | MUST | 2 | **positive:** `unit/verify` [`TestIPv6FlowSpecNegotiatesMultiprotocolCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/plugin_test.go#L808). **negative:** no negative test. **{single-polarity}:** the flowspec plugin unconditionally maps its declared ipv6/flow decode family to a Multiprotocol capability during OPEN; there is no wrong input the negotiation path rejects, so no negative case exists |
| `RFC8956-2-2` | The (AFI, SAFI) pair carried in the Multiprotocol Extension Capability MUST be (AFI=2, SAFI=133) for IPv6 Flow Specification rules and (AFI=2, SAFI=134) for L3VPN (§2) | MUST | 2 | **positive:** `unit/verify` [`TestFlowSpecIPv6Basic`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L439). **positive:** `unit/verify` [`TestFlowSpecVPNFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L654). **negative:** no negative test. **{single-polarity}:** the (AFI 2, SAFI 133/134) assignment is a family-registration constant, not an input guard; the code accepts no alternative value it could reject, so only the positive assignment is assertable |
| `RFC8956-3.1-1` | Padding bits in IPv6 prefix components MUST be 0 on encoding (§3.1) | MUST | 3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze's IPv6 flowspec prefix encoder (internal/component/bgp/plugins/nlri/flowspec/types_prefix.go:103-127 WriteTo) copies the address bytes without zeroing the sub-byte padding beyond the prefix length; padding-zero relies on a pre-masked caller value and is not enforced by the producer |
| `RFC8956-3.1-2` | Padding bits in IPv6 prefix components MUST be ignored on decoding (§3.1) | MUST | 3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze's IPv6 flowspec prefix decoder (internal/component/bgp/plugins/nlri/flowspec/types_prefix.go:148-201 parsePrefixComponent) retains the pattern bytes without masking the padding beyond the prefix length, so it neither zeroes nor validates the trailing bits |
| `RFC8956-3.6-1` | Type 12 (Fragment) component bitmask MUST be encoded as a single octet bitmask (bitmask_op len=00) (§3.6) | MUST | 3.6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze's IPv6 flowspec numeric encoder (internal/component/bgp/plugins/nlri/flowspec/types_numeric.go:238-264) selects a single octet only incidentally for small values and does not enforce the minimal-length encoding as a producer rule |
| `RFC8956-3.6-2` | Fragment bitmask reserved bits (bits 0,1,2,3,7) MUST be set to 0 on NLRI encoding (§3.6) | MUST | 3.6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze encodes the Fragment component with the RFC 8955 IPv4 fragment bitmask layout (internal/component/bgp/plugins/nlri/flowspec/types.go:201-206) rather than the RFC 8956 IPv6 layout, and does not zero the IPv6 reserved bits on transmit |
| `RFC8956-3.6-3` | Fragment bitmask reserved bits MUST be ignored during decoding (§3.6) | MUST | 3.6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze's Fragment decoder (internal/component/bgp/plugins/nlri/flowspec/types_numeric.go:291-343) reads the raw byte without masking the IPv6 fragment reserved bits, using the RFC 8955 IPv4 layout |
| `RFC8956-3.1-3` | Destination prefix length/offset: length MUST be in the range offset < length < 129 unless length=0 and offset=0 (matching all); otherwise the component is malformed (§3.1) | MUST | 3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze's IPv6 flowspec prefix decoder (internal/component/bgp/plugins/nlri/flowspec/types_prefix.go:148-201) does not validate the offset/length relationship (offset < length <= 128) and feeds the length straight to netip.PrefixFrom, so a malformed offset/length is not rejected; additionally the pattern is encoded from the address start rather than from the offset, correct only for offset 0 |
| `RFC8956-5-1` | Destination prefix for validation: offset MUST be 0 to pass the validation procedure (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze implements no RFC 8956 Section 5 / RFC 8955 Section 6 flowspec validation-against-unicast procedure; a received flowspec NLRI is decoded and installed without the origin/best-match validation |
| `RFC8956-3.3-1` | Type 3 (Upper-Layer Protocol) values SHOULD be encoded as a single octet (numeric_op len=00) (§3.3) | SHOULD | 3.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC8956-3.4-1` | Type 7 (ICMPv6 Type) values SHOULD be encoded as a single octet (numeric_op len=00) (§3.4) | SHOULD | 3.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC8956-3.5-1` | Type 8 (ICMPv6 Code) values SHOULD be encoded as a single octet (numeric_op len=00) (§3.5) | SHOULD | 3.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC8956-3.7-1` | Type 13 (Flow Label) values SHOULD be encoded as 4-octet quantities (numeric_op len=10) (§3.7) | SHOULD | 3.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC8956-6.1-1` | When multiple VRFs match a redirect, local choice MAY determine which is used (§6.1) | MAY | 6.1 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC8956-3.1-1`](#rfc8956-3.1-1) Padding bits in IPv6 prefix components MUST be 0 on encoding (§3.1) | {gap}, no test | ze's IPv6 flowspec prefix encoder (internal/component/bgp/plugins/nlri/flowspec/types_prefix.go:103-127 WriteTo) copies the address bytes without zeroing the sub-byte padding beyond the prefix length; padding-zero relies on a pre-masked caller value and is not enforced by the producer |
| [`RFC8956-3.1-2`](#rfc8956-3.1-2) Padding bits in IPv6 prefix components MUST be ignored on decoding (§3.1) | {gap}, no test | ze's IPv6 flowspec prefix decoder (internal/component/bgp/plugins/nlri/flowspec/types_prefix.go:148-201 parsePrefixComponent) retains the pattern bytes without masking the padding beyond the prefix length, so it neither zeroes nor validates the trailing bits |
| [`RFC8956-3.6-1`](#rfc8956-3.6-1) Type 12 (Fragment) component bitmask MUST be encoded as a single octet bitmask (bitmask_op len=00) (§3.6) | {gap}, no test | ze's IPv6 flowspec numeric encoder (internal/component/bgp/plugins/nlri/flowspec/types_numeric.go:238-264) selects a single octet only incidentally for small values and does not enforce the minimal-length encoding as a producer rule |
| [`RFC8956-3.6-2`](#rfc8956-3.6-2) Fragment bitmask reserved bits (bits 0,1,2,3,7) MUST be set to 0 on NLRI encoding (§3.6) | {gap}, no test | ze encodes the Fragment component with the RFC 8955 IPv4 fragment bitmask layout (internal/component/bgp/plugins/nlri/flowspec/types.go:201-206) rather than the RFC 8956 IPv6 layout, and does not zero the IPv6 reserved bits on transmit |
| [`RFC8956-3.6-3`](#rfc8956-3.6-3) Fragment bitmask reserved bits MUST be ignored during decoding (§3.6) | {gap}, no test | ze's Fragment decoder (internal/component/bgp/plugins/nlri/flowspec/types_numeric.go:291-343) reads the raw byte without masking the IPv6 fragment reserved bits, using the RFC 8955 IPv4 layout |
| [`RFC8956-3.1-3`](#rfc8956-3.1-3) Destination prefix length/offset: length MUST be in the range offset < length < 129 unless length=0 and offset=0 (matching all); otherwise the component is malformed (§3.1) | {gap}, no test | ze's IPv6 flowspec prefix decoder (internal/component/bgp/plugins/nlri/flowspec/types_prefix.go:148-201) does not validate the offset/length relationship (offset < length <= 128) and feeds the length straight to netip.PrefixFrom, so a malformed offset/length is not rejected; additionally the pattern is encoded from the address start rather than from the offset, correct only for offset 0 |
| [`RFC8956-5-1`](#rfc8956-5-1) Destination prefix for validation: offset MUST be 0 to pass the validation procedure (§5) | {gap}, no test | ze implements no RFC 8956 Section 5 / RFC 8955 Section 6 flowspec validation-against-unicast procedure; a received flowspec NLRI is decoded and installed without the origin/best-match validation |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC8956-2-1`](#rfc8956-2-1)

Implementations wishing to exchange IPv6 Flow Specifications MUST use BGP's Capability Advertisement facility to exchange the Multiprotocol Extension Capability Code (Code 1) (§2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestIPv6FlowSpecNegotiatesMultiprotocolCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/plugin_test.go#L808) | unit/verify | unproven |

### [`RFC8956-2-2`](#rfc8956-2-2)

The (AFI, SAFI) pair carried in the Multiprotocol Extension Capability MUST be (AFI=2, SAFI=133) for IPv6 Flow Specification rules and (AFI=2, SAFI=134) for L3VPN (§2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestFlowSpecIPv6Basic`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L439) | unit/verify | unproven |
| positive | [`TestFlowSpecVPNFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L654) | unit/verify | unproven |

### [`RFC8956-3.1-1`](#rfc8956-3.1-1)

Padding bits in IPv6 prefix components MUST be 0 on encoding (§3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8956-3.1-1, so no unit is bound to it.

### [`RFC8956-3.1-2`](#rfc8956-3.1-2)

Padding bits in IPv6 prefix components MUST be ignored on decoding (§3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8956-3.1-2, so no unit is bound to it.

### [`RFC8956-3.6-1`](#rfc8956-3.6-1)

Type 12 (Fragment) component bitmask MUST be encoded as a single octet bitmask (bitmask_op len=00) (§3.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8956-3.6-1, so no unit is bound to it.

### [`RFC8956-3.6-2`](#rfc8956-3.6-2)

Fragment bitmask reserved bits (bits 0,1,2,3,7) MUST be set to 0 on NLRI encoding (§3.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8956-3.6-2, so no unit is bound to it.

### [`RFC8956-3.6-3`](#rfc8956-3.6-3)

Fragment bitmask reserved bits MUST be ignored during decoding (§3.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8956-3.6-3, so no unit is bound to it.

### [`RFC8956-3.1-3`](#rfc8956-3.1-3)

Destination prefix length/offset: length MUST be in the range offset < length < 129 unless length=0 and offset=0 (matching all); otherwise the component is malformed (§3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8956-3.1-3, so no unit is bound to it.

### [`RFC8956-5-1`](#rfc8956-5-1)

Destination prefix for validation: offset MUST be 0 to pass the validation procedure (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8956-5-1, so no unit is bound to it.

## Extraction sign-off

No extraction sign-off exists for RFC 8956, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 8956, so its obligations are stated where they were written.
