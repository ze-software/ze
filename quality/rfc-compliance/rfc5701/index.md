# RFC 5701 - IPv6 Address Specific BGP Extended Community Attribute

IPv6 extended communities. Every requirement this repository extracted from RFC 5701, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 3 | of 4 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0 | of 4 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0 | of 4 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0 | of 4 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Proven by a recorded break | 0 | 0 of 6 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 4 | of 6 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the counts beside it are what says how Ze stands |
| Out of scope | 0 | of 4 gated MUSTs | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as a Ze gap. Scope, not coverage: it is counted in the gated whole the cards on this page add up to |
| Not applicable | 0 | of 4 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as a Ze gap. It is counted in the gated whole the cards add up to |
| Met below Ze | 0 | of 4 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze proven: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 4 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as a Ze gap. It is counted in the gated whole the cards add up to |
| Linux cannot do this | 0 | of 4 gated MUSTs | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as a Ze gap. It is counted in the gated whole the cards add up to and earns no proof credit |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 1 | of 4 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

The 9 counts marked as a part above are the whole of the 4 gated MUSTs: they add up to 4. Proven by a recorded break counts TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the count under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| Partial proof; remaining gap | ok | green at zero, RED above it: a tested clause cannot prove the whole requirement |
| No test at all | bad | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green counts above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Linux cannot do this | neutral | neutral at every value: an evidenced platform limit is neither a proof nor a scope exemption |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. It counts TAGGED UNITS, not obligations, so this count is not one of the parts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public row | IPv6 extended communities |
| Enrolment | Enrolled |
| Requirements | 6 |
| Gated MUST-level | 4 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 1 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 6 |
| Tagged units | 6 |
| Recorded audit verdicts | 3 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc5701.md` |
| Requirement shard | `rfc/requirements/rfc5701.md` |
| RFC text | `rfc/full/rfc5701.txt` |

## Enrolment

Enrolled: IPv6 Address Specific BGP Extended Community Attribute (code 25): four MUST-level requirements over the codec (internal/core/bgp/attribute/community.go). RFC5701-2-1 (optional transitive, O=1 T=1) has both polarities in TestRFC5701IPv6ExtCommunityFlags: Flags() (community.go:483) returns exactly FlagOptional\|FlagTransitive, never the non-transitive 0x80 or a well-known/partial form. RFC5701-2-2 (each community exactly 20 octets) has both polarities in TestRFC5701IPv6ExtCommunityTwentyOctets: the [20]byte type, Len()=20*count and WriteTo lay each community on a 20-octet stride, and the second community starts at offset 20 not the 8-octet RFC 4360 stride. RFC5701-2-3 (length a multiple of 20) has both polarities in TestRFC5701IPv6ExtCommunityLengthMultipleOf20: a 40-octet buffer parses to 2 communities while 19/21/8/39/41 are rejected with ErrInvalidLength (community.go:515). RFC5701-4-1 (RFC 4271/7606 malformed optional-transitive handling) is {gap}: code 25 has no RFC 7606 structural validator (attrValidators[25] unset), so a malformed code-25 attribute passes structural validation rather than being treat-as-withdrawn; the lazy parser rejects a bad length only on access. Disclosed in the docs/features/rfc-status.md RFC 5701 row (Partial); same code-25 omission tracked under RFC 7606. The 2-4 SHOULD (non-transitive not propagated to external peers) and 2-5 MAY are not gated.

## What the public ledger says

**Area:** IPv6 extended communities

**Proof:** 0 of 4 MUST proven

**Evidence:** -

**Unmet:** RFC5701-4-1


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 3 | one part of the gated population |
| Annotated (including scoped evidence) | 1 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **4** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (3):** [`RFC5701-2-1`](#rfc5701-2-1), [`RFC5701-2-2`](#rfc5701-2-2), [`RFC5701-2-3`](#rfc5701-2-3)

**Annotated (including scoped evidence) (1):** [`RFC5701-4-1`](#rfc5701-4-1)

## Figures

- **MUST bar**: Ze correctness gap 4
- **Overview bar**: SHOULD not decided 2, Ze correctness gap 4

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Not walked | 0 |
| Mandatory register | not complete |
| MUST rows | 4 |
| MUST rows kept (the MUST bar) | 4 |
| Ze proven | 0 |
| OS verified | 0 |
| OS testing gap | 0 |
| Ze correctness gap | 4 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 2 |
| SHOULD proven | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD not ever | 0 |
| SHOULD not yet | 0 |
| SHOULD not decided | 2 |

The MUST bar counts the MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT rows of the walked RFCs that the stack owes: Ze proven means a recorded test break proves the row; a row a layer under Ze performs is OS verified when a test checks that layer on the state Ze installs, an OS testing gap until one does. The overview bar adds the MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT rows and the rows not owed, which do not bind Ze. An RFC that is not walked has no valid extraction sign-off, so its rows count in no figure. Every count is read from the RFC data files when the page loads.

Source: [data/rfc/rfc5701.json](../../../data/rfc/rfc5701.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC5701-2-1` | The IPv6 Address Specific Extended Community Attribute is a transitive, optional BGP attribute [BGP-4]. (§2) | MUST | 2 | **positive:** `unit/verify` [`TestRFC5701IPv6ExtCommunityFlags`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc5701_ipv6_extcommunity_test.go#L14). **negative:** `unit/verify` [`TestRFC5701IPv6ExtCommunityFlags`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc5701_ipv6_extcommunity_test.go#L19) |
| `RFC5701-2-2` | Each IPv6 Address Specific extended community is encoded as a 20-octet quantity (§2) | MUST | 2 | **positive:** `unit/verify` [`TestRFC5701IPv6ExtCommunityTwentyOctets`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc5701_ipv6_extcommunity_test.go#L46). **negative:** `unit/verify` [`TestRFC5701IPv6ExtCommunityTwentyOctets`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc5701_ipv6_extcommunity_test.go#L50) |
| `RFC5701-2-3` | The attribute consists of a set of "IPv6 Address Specific extended communities". All routes with the IPv6 Address Specific Extended Community attribute belong to the communities listed in the attribute. Just like all other BGP Extended Communities, the IPv6 Address Specific Extended Community supports multiple sub-types. Each IPv6 Address Specific extended community is encoded as a 20-octet quantity (§2) | MUST | 2 | **positive:** `unit/verify` [`TestRFC5701IPv6ExtCommunityLengthMultipleOf20`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc5701_ipv6_extcommunity_test.go#L80). **negative:** `unit/verify` [`TestRFC5701IPv6ExtCommunityLengthMultipleOf20`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc5701_ipv6_extcommunity_test.go#L83) |
| `RFC5701-4-1` | There is a potential serious issue if a malformed, optional, transitive attribute is received. This issue and the steps to avoid it are discussed in [OPT_TRANS]. (§4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze does not apply RFC 7606 malformed-attribute handling (treat-as-withdraw) to a malformed IPv6 Address Specific Extended Community (attribute code 25). The RFC 7606 structural validation pass has no per-attribute validator for code 25 (internal/component/bgp/message/rfc7606.go attrValidators[25] is unset), so a code-25 attribute whose length is not a multiple of 20 passes structural validation instead of being treat-as-withdrawn. The lazy value parser ParseIPv6ExtendedCommunities (internal/core/bgp/attribute/community.go:515) does reject a bad length with ErrInvalidLength, but only when the attribute is later accessed, which is not the RFC 4271/7606-mandated optional-transitive treat-as-withdraw at ingest. This is the same code-25 omission already disclosed under RFC 7606 (§7.15). Disclosed in docs/features/rfc-status.md |
| `RFC5701-2-4` | The first high-order octet indicates whether a particular sub-type of this community is transitive across Autonomous Systems (ASes) (0x00), or not (0x40). (§2) | SHOULD | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5701-2-5` | The organization that has been assigned the IPv6 address in the Global Administrator field can encode any information in this field. (§2) | MAY | 2 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC5701-4-1`](#rfc5701-4-1) There is a potential serious issue if a malformed, optional, transitive attribute is received. This issue and the steps to avoid it are discussed in [OPT_TRANS]. (§4) | {gap}, no test | Ze does not apply RFC 7606 malformed-attribute handling (treat-as-withdraw) to a malformed IPv6 Address Specific Extended Community (attribute code 25). The RFC 7606 structural validation pass has no per-attribute validator for code 25 (internal/component/bgp/message/rfc7606.go attrValidators[25] is unset), so a code-25 attribute whose length is not a multiple of 20 passes structural validation instead of being treat-as-withdrawn. The lazy value parser ParseIPv6ExtendedCommunities (internal/core/bgp/attribute/community.go:515) does reject a bad length with ErrInvalidLength, but only when the attribute is later accessed, which is not the RFC 4271/7606-mandated optional-transitive treat-as-withdraw at ingest. This is the same code-25 omission already disclosed under RFC 7606 (§7.15). Disclosed in docs/features/rfc-status.md |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC5701-2-1`](#rfc5701-2-1)

The IPv6 Address Specific Extended Community Attribute is a transitive, optional BGP attribute [BGP-4]. (§2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: flagging code 25 as anything but optional transitive (0x80 non-transitive, 0x40 well-known, partial on origination). TestRFC5701IPv6ExtCommunityFlags asserts IsOptional, IsTransitive, f != FlagOptional, !IsPartial and f == FlagOptional|FlagTransitive, each red on the corresponding wrong flag byte from Flags() (community.go). Both polarities.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5701IPv6ExtCommunityFlags`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc5701_ipv6_extcommunity_test.go#L19) | unit/verify | unproven |
| positive | [`TestRFC5701IPv6ExtCommunityFlags`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc5701_ipv6_extcommunity_test.go#L14) | unit/verify | unproven |

### [`RFC5701-2-2`](#rfc5701-2-2)

Each IPv6 Address Specific extended community is encoded as a 20-octet quantity (§2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: encoding a community on any stride but 20 octets (e.g. the 8-octet RFC 4360 stride). TestRFC5701IPv6ExtCommunityTwentyOctets asserts Len() == 40 and WriteTo n == 40 for two communities, and buf[20] == 0xBB with buf[8], buf[16] != 0xBB, red on any other per-community size. Both polarities.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5701IPv6ExtCommunityTwentyOctets`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc5701_ipv6_extcommunity_test.go#L50) | unit/verify | unproven |
| positive | [`TestRFC5701IPv6ExtCommunityTwentyOctets`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc5701_ipv6_extcommunity_test.go#L46) | unit/verify | unproven |

### [`RFC5701-2-3`](#rfc5701-2-3)

The attribute consists of a set of "IPv6 Address Specific extended communities". All routes with the IPv6 Address Specific Extended Community attribute belong to the communities listed in the attribute. Just like all other BGP Extended Communities, the IPv6 Address Specific Extended Community supports multiple sub-types. Each IPv6 Address Specific extended community is encoded as a 20-octet quantity (§2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. The span states the attribute is a set of 20-octet communities, so a value whose length is not a multiple of 20 is not such a set. Forbidden: parsing a non-multiple length as communities (truncating). TestRFC5701IPv6ExtCommunityLengthMultipleOf20 asserts ParseIPv6ExtendedCommunities accepts 40 octets as exactly 2, and returns ErrInvalidLength for 19, 21, 8, 39 and 41, red on truncation or acceptance. Both polarities. The zero-length and treat-as-withdraw handling is RFC 7606 Section 7.15, row RFC7606-7.15-1.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5701IPv6ExtCommunityLengthMultipleOf20`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc5701_ipv6_extcommunity_test.go#L83) | unit/verify | unproven |
| positive | [`TestRFC5701IPv6ExtCommunityLengthMultipleOf20`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc5701_ipv6_extcommunity_test.go#L80) | unit/verify | unproven |

### [`RFC5701-4-1`](#rfc5701-4-1)

There is a potential serious issue if a malformed, optional, transitive attribute is received. This issue and the steps to avoid it are discussed in [OPT_TRANS]. (§4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5701-4-1, so no unit is bound to it.

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | prose |
| Source | rfc/full/rfc5701.txt |
| Source fingerprint | a247b10290a694aa |
| Record | rfc/extraction/rfc5701.json |
| Mapped sentences | 0 |
| Declined as scope | 1 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 1 | walked | not stated |
| `1` | not stated | 0 | walked | not stated |
| `2` | not stated | 0 | walked | not stated |
| `3` | not stated | 0 | walked | not stated |
| `4` | not stated | 0 | walked | not stated |
| `5` | not stated | 0 | walked | not stated |
| `6` | not stated | 0 | walked | not stated |
| `6.1` | not stated | 0 | walked | not stated |
| `6.2` | not stated | 0 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `front:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Boilerplate: the sentence is part of the IETF Trust Copyright Notice and constrains how code extracted from the document is licensed, not the behavior of a BGP speaker. | Code Components extracted from this document must include Simplified BSD License text as described in Section 4.e of the Trust Legal Provisions and are provided without warranty as described in the BSD License. |

## Superseded

No document obsoletes RFC 5701, so its obligations are stated where they were written.
