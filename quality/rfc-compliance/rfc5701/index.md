# RFC 5701 - IPv6 Address Specific BGP Extended Community Attribute

Partial. Every requirement this repository extracted from RFC 5701, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 75.0% | 3 of 4 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 4 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 4 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 0.0% | 0 of 6 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 4 | of 6 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 4 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 25.0% | 1 of 4 binding obligations | no test carries the requirement id, whether or not a gap states why |

The 4 shares marked as a part above are the whole of the 4 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 6 |
| Gated MUST-level | 4 |
| Obligations that bind Ze | 4 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 1 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 6 |
| Tagged units | 6 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc5701.md` |
| Requirement shard | `rfc/requirements/rfc5701.md` |
| RFC text | `rfc/full/rfc5701.txt` |

## Enrolment

Enrolled: IPv6 Address Specific BGP Extended Community Attribute (code 25): four MUST-level requirements over the codec (internal/core/bgp/attribute/community.go). RFC5701-2-1 (optional transitive, O=1 T=1) has both polarities in TestRFC5701IPv6ExtCommunityFlags: Flags() (community.go:483) returns exactly FlagOptional\|FlagTransitive, never the non-transitive 0x80 or a well-known/partial form. RFC5701-2-2 (each community exactly 20 octets) has both polarities in TestRFC5701IPv6ExtCommunityTwentyOctets: the [20]byte type, Len()=20*count and WriteTo lay each community on a 20-octet stride, and the second community starts at offset 20 not the 8-octet RFC 4360 stride. RFC5701-2-3 (length a multiple of 20) has both polarities in TestRFC5701IPv6ExtCommunityLengthMultipleOf20: a 40-octet buffer parses to 2 communities while 19/21/8/39/41 are rejected with ErrInvalidLength (community.go:515). RFC5701-4-1 (RFC 4271/7606 malformed optional-transitive handling) is {gap}: code 25 has no RFC 7606 structural validator (attrValidators[25] unset), so a malformed code-25 attribute passes structural validation rather than being treat-as-withdrawn; the lazy parser rejects a bad length only on access. Disclosed in the docs/features/rfc-status.md RFC 5701 row (Partial); same code-25 omission tracked under RFC 7606. The 2-4 SHOULD (non-transitive not propagated to external peers) and 2-5 MAY are not gated.

## What the public ledger says

**Status:** Partial

**What the ledger says is covered:**

IPv6 Address Specific Extended Community (code 25) codec: optional-transitive flags, 20-octet per-community encoding, and length-multiple-of-20 parse validation. Tests bound per requirement in [`rfc/requirements/rfc5701.md`](https://github.com/ze-software/ze/blob/main/rfc/requirements/rfc5701.md).

**What the ledger says remains**

One MUST gap, gated in [`rfc/short/rfc5701.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc5701.md): a malformed code-25 attribute is not treat-as-withdrawn per RFC 7606 (the structural validation pass has no validator for code 25), the same code-25 omission already tracked under RFC 7606 §7.15. The lazy parser still rejects a non-multiple-of-20 length on access.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 3 | one part of the gated population |
| Annotated instead of tested | 1 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **4** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (3):** [`RFC5701-2-1`](#rfc5701-2-1), [`RFC5701-2-2`](#rfc5701-2-2), [`RFC5701-2-3`](#rfc5701-2-3)

**Annotated instead of tested (1):** [`RFC5701-4-1`](#rfc5701-4-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC5701-2-1` | Attribute is optional, transitive (O=1 T=1) per BGP-4 attribute handling (§2) | MUST | 2 | **positive:** `unit/verify` [`TestRFC5701IPv6ExtCommunityFlags`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc5701_ipv6_extcommunity_test.go#L14). **negative:** `unit/verify` [`TestRFC5701IPv6ExtCommunityFlags`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc5701_ipv6_extcommunity_test.go#L19) |
| `RFC5701-2-2` | Each community is encoded as exactly 20 octets (§2) | MUST | 2 | **positive:** `unit/verify` [`TestRFC5701IPv6ExtCommunityTwentyOctets`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc5701_ipv6_extcommunity_test.go#L46). **negative:** `unit/verify` [`TestRFC5701IPv6ExtCommunityTwentyOctets`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc5701_ipv6_extcommunity_test.go#L50) |
| `RFC5701-2-3` | Attribute length must be a multiple of 20 octets (§2, Validation) | MUST | 2 | **positive:** `unit/verify` [`TestRFC5701IPv6ExtCommunityLengthMultipleOf20`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc5701_ipv6_extcommunity_test.go#L80). **negative:** `unit/verify` [`TestRFC5701IPv6ExtCommunityLengthMultipleOf20`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc5701_ipv6_extcommunity_test.go#L83) |
| `RFC5701-4-1` | Follow RFC 4271 transitive attribute handling for malformed optional transitive attributes (§4, referencing OPT_TRANS/RFC 7606) | MUST | 4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze does not apply RFC 7606 malformed-attribute handling (treat-as-withdraw) to a malformed IPv6 Address Specific Extended Community (attribute code 25). The RFC 7606 structural validation pass has no per-attribute validator for code 25 (internal/component/bgp/message/rfc7606.go attrValidators[25] is unset), so a code-25 attribute whose length is not a multiple of 20 passes structural validation instead of being treat-as-withdrawn. The lazy value parser ParseIPv6ExtendedCommunities (internal/core/bgp/attribute/community.go:515) does reject a bad length with ErrInvalidLength, but only when the attribute is later accessed, which is not the RFC 4271/7606-mandated optional-transitive treat-as-withdraw at ingest. This is the same code-25 omission already disclosed under RFC 7606 (§7.15). Disclosed in docs/features/rfc-status.md |
| `RFC5701-2-4` | Non-transitive communities (Type=0x40) should not be propagated to external peers (§2) | SHOULD | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5701-2-5` | Organization assigned the IPv6 address can encode any information in Local Administrator field (§2) | MAY | 2 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC5701-4-1`](#rfc5701-4-1) Follow RFC 4271 transitive attribute handling for malformed optional transitive attributes (§4, referencing OPT_TRANS/RFC 7606) | {gap}, no test | Ze does not apply RFC 7606 malformed-attribute handling (treat-as-withdraw) to a malformed IPv6 Address Specific Extended Community (attribute code 25). The RFC 7606 structural validation pass has no per-attribute validator for code 25 (internal/component/bgp/message/rfc7606.go attrValidators[25] is unset), so a code-25 attribute whose length is not a multiple of 20 passes structural validation instead of being treat-as-withdrawn. The lazy value parser ParseIPv6ExtendedCommunities (internal/core/bgp/attribute/community.go:515) does reject a bad length with ErrInvalidLength, but only when the attribute is later accessed, which is not the RFC 4271/7606-mandated optional-transitive treat-as-withdraw at ingest. This is the same code-25 omission already disclosed under RFC 7606 (§7.15). Disclosed in docs/features/rfc-status.md |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC5701-2-1`](#rfc5701-2-1)

Attribute is optional, transitive (O=1 T=1) per BGP-4 attribute handling (§2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5701IPv6ExtCommunityFlags`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc5701_ipv6_extcommunity_test.go#L19) | unit/verify | unproven |
| positive | [`TestRFC5701IPv6ExtCommunityFlags`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc5701_ipv6_extcommunity_test.go#L14) | unit/verify | unproven |

### [`RFC5701-2-2`](#rfc5701-2-2)

Each community is encoded as exactly 20 octets (§2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5701IPv6ExtCommunityTwentyOctets`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc5701_ipv6_extcommunity_test.go#L50) | unit/verify | unproven |
| positive | [`TestRFC5701IPv6ExtCommunityTwentyOctets`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc5701_ipv6_extcommunity_test.go#L46) | unit/verify | unproven |

### [`RFC5701-2-3`](#rfc5701-2-3)

Attribute length must be a multiple of 20 octets (§2, Validation)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5701IPv6ExtCommunityLengthMultipleOf20`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc5701_ipv6_extcommunity_test.go#L83) | unit/verify | unproven |
| positive | [`TestRFC5701IPv6ExtCommunityLengthMultipleOf20`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc5701_ipv6_extcommunity_test.go#L80) | unit/verify | unproven |

### [`RFC5701-4-1`](#rfc5701-4-1)

Follow RFC 4271 transitive attribute handling for malformed optional transitive attributes (§4, referencing OPT_TRANS/RFC 7606)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5701-4-1, so no unit is bound to it.

## Extraction sign-off

No extraction sign-off exists for RFC 5701, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 5701, so its obligations are stated where they were written.
