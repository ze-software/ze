# RFC 5701 - IPv6 Address Specific BGP Extended Community Attribute

IPv6 extended communities. Every requirement listed for RFC 5701, the tests linked to it, and their recorded reviews. This summary is included in the automatic compliance check, ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 3 | of 4 mandatory requirements | a positive test checks that Ze does what the requirement demands and a negative one checks that it refuses what the requirement forbids |
| One polarity plus reason | 0 | of 4 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason supplies the complete test coverage required |
| One polarity, unexcused | 0 | of 4 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial test coverage | 0 | of 4 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unverified, with zero whole-requirement credit |
| Tests shown to detect a fault | 0 | 0 of 6 tests linked to requirements | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests verifies a requirement only after both tests have been shown to detect a fault |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Declared gap or no test | 1 | of 4 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 4 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 0 | of 4 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 0 | of 4 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze verified: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 4 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 4 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no test verification credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level) | 4 | of 6 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements included in the automatic checks: the compliance check (./le rfc check) holds Ze to each of them. This is the total being assessed; the counts above show the recorded status of those requirements. |

The 9 counts marked as a part above are the whole of the 4 mandatory requirements: they add up to 4. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got. An outlined card with no colored edge, under Totals, is a total the other cards are counted against, not a measure.

| Card | Tone here | Why that color |
|---|---|---|
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the count under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: one test direction is missing without a recorded reason |
| Partial test coverage | ok | green at zero, RED above it: testing one clause does not verify the whole requirement |
| Declared gap or no test | bad | green at zero, RED above it: a binding obligation that nothing shows Ze meets is owed, whether or not a reason is stated |
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
| Public row | IPv6 extended communities |
| Automatic checks | Included |
| Requirements | 6 |
| Mandatory requirements (MUST level) | 4 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 1 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 6 |
| Tests linked to requirements | 6 |
| Recorded audit verdicts | 3 |
| Fault test records | 0 |
| Summary | `rfc/short/rfc5701.md` |
| Requirement table | `rfc/requirements/rfc5701.md` |
| RFC text | `rfc/full/rfc5701.txt` |

## Automatic checks

Included in automatic checks: IPv6 Address Specific BGP Extended Community Attribute (code 25): four MUST-level requirements over the codec (internal/core/bgp/attribute/community.go). RFC5701-2-1 (optional transitive, O=1 T=1) has both polarities in TestRFC5701IPv6ExtCommunityFlags: Flags() (community.go:483) returns exactly FlagOptional\|FlagTransitive, never the non-transitive 0x80 or a well-known/partial form. RFC5701-2-2 (each community exactly 20 octets) has both polarities in TestRFC5701IPv6ExtCommunityTwentyOctets: the [20]byte type, Len()=20*count and WriteTo lay each community on a 20-octet stride, and the second community starts at offset 20 not the 8-octet RFC 4360 stride. RFC5701-2-3 (length a multiple of 20) has both polarities in TestRFC5701IPv6ExtCommunityLengthMultipleOf20: a 40-octet buffer parses to 2 communities while 19/21/8/39/41 are rejected with ErrInvalidLength (community.go:515). RFC5701-4-1 (RFC 4271/7606 malformed optional-transitive handling) is {gap}: code 25 has no RFC 7606 structural validator (attrValidators[25] unset), so a malformed code-25 attribute passes structural validation rather than being treat-as-withdrawn; the lazy parser rejects a bad length only on access. Disclosed in the docs/features/rfc-status.md RFC 5701 row (Partial); same code-25 omission tracked under RFC 7606. The 2-4 SHOULD (non-transitive not propagated to external peers) and 2-5 MAY are not gated.

## Published support status

**Area:** IPv6 extended communities

**Test verification:** 0 of 4 MUST proven

**Evidence:** -

**Unmet:** RFC5701-4-1


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 3 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 1 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 0 | one part of the mandatory requirements |
| Partial test coverage (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level)** | **4** | every mandatory requirement appears in exactly one group above |

**Positive and negative tests (3):** [`RFC5701-2-1`](#rfc5701-2-1), [`RFC5701-2-2`](#rfc5701-2-2), [`RFC5701-2-3`](#rfc5701-2-3)

**Annotated (including scoped evidence) (1):** [`RFC5701-4-1`](#rfc5701-4-1)

## Figures

- **MUST bar**: Tests awaiting verification 3, Not built 1
- **Overview bar**: Tests awaiting verification 3, Not assessed 2, Not built 1

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | not complete |
| MUST rows | 4 |
| MUST rows kept (the MUST bar) | 4 |
| Ze verified | 0 |
| OS verified | 0 |
| Tests awaiting verification | 3 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 1 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 2 |
| SHOULD verified | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD tests awaiting verification | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 2 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze verified means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Tests awaiting verification counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

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

## Test status

A test reads not verified when no record shows it failing after a deliberate fault. Until then, the test has not been shown to detect a fault in the required behavior.

### [`RFC5701-2-1`](#rfc5701-2-1)

The IPv6 Address Specific Extended Community Attribute is a transitive, optional BGP attribute [BGP-4]. (§2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: flagging code 25 as anything but optional transitive (0x80 non-transitive, 0x40 well-known, partial on origination). TestRFC5701IPv6ExtCommunityFlags asserts IsOptional, IsTransitive, f != FlagOptional, !IsPartial and f == FlagOptional|FlagTransitive, each red on the corresponding wrong flag byte from Flags() (community.go). Both polarities.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC5701IPv6ExtCommunityFlags`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc5701_ipv6_extcommunity_test.go#L19) | unit/verify | not verified |
| positive | [`TestRFC5701IPv6ExtCommunityFlags`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc5701_ipv6_extcommunity_test.go#L14) | unit/verify | not verified |

### [`RFC5701-2-2`](#rfc5701-2-2)

Each IPv6 Address Specific extended community is encoded as a 20-octet quantity (§2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: encoding a community on any stride but 20 octets (e.g. the 8-octet RFC 4360 stride). TestRFC5701IPv6ExtCommunityTwentyOctets asserts Len() == 40 and WriteTo n == 40 for two communities, and buf[20] == 0xBB with buf[8], buf[16] != 0xBB, red on any other per-community size. Both polarities.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC5701IPv6ExtCommunityTwentyOctets`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc5701_ipv6_extcommunity_test.go#L50) | unit/verify | not verified |
| positive | [`TestRFC5701IPv6ExtCommunityTwentyOctets`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc5701_ipv6_extcommunity_test.go#L46) | unit/verify | not verified |

### [`RFC5701-2-3`](#rfc5701-2-3)

The attribute consists of a set of "IPv6 Address Specific extended communities". All routes with the IPv6 Address Specific Extended Community attribute belong to the communities listed in the attribute. Just like all other BGP Extended Communities, the IPv6 Address Specific Extended Community supports multiple sub-types. Each IPv6 Address Specific extended community is encoded as a 20-octet quantity (§2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. The span states the attribute is a set of 20-octet communities, so a value whose length is not a multiple of 20 is not such a set. Forbidden: parsing a non-multiple length as communities (truncating). TestRFC5701IPv6ExtCommunityLengthMultipleOf20 asserts ParseIPv6ExtendedCommunities accepts 40 octets as exactly 2, and returns ErrInvalidLength for 19, 21, 8, 39 and 41, red on truncation or acceptance. Both polarities. The zero-length and treat-as-withdraw handling is RFC 7606 Section 7.15, row RFC7606-7.15-1.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC5701IPv6ExtCommunityLengthMultipleOf20`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc5701_ipv6_extcommunity_test.go#L83) | unit/verify | not verified |
| positive | [`TestRFC5701IPv6ExtCommunityLengthMultipleOf20`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc5701_ipv6_extcommunity_test.go#L80) | unit/verify | not verified |

### [`RFC5701-4-1`](#rfc5701-4-1)

There is a potential serious issue if a malformed, optional, transitive attribute is received. This issue and the steps to avoid it are discussed in [OPT_TRANS]. (§4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC5701-4-1, so there is no linked test.

## Requirement list review

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
| `front` | not stated | 1 | reviewed | not stated |
| `1` | not stated | 0 | reviewed | not stated |
| `2` | not stated | 0 | reviewed | not stated |
| `3` | not stated | 0 | reviewed | not stated |
| `4` | not stated | 0 | reviewed | not stated |
| `5` | not stated | 0 | reviewed | not stated |
| `6` | not stated | 0 | reviewed | not stated |
| `6.1` | not stated | 0 | reviewed | not stated |
| `6.2` | not stated | 0 | reviewed | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `front:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Boilerplate: the sentence is part of the IETF Trust Copyright Notice and constrains how code extracted from the document is licensed, not the behavior of a BGP speaker. | Code Components extracted from this document must include Simplified BSD License text as described in Section 4.e of the Trust Legal Provisions and are provided without warranty as described in the BSD License. |

## Superseded

No document obsoletes RFC 5701, so its obligations are stated where they were written.
