# RFC 4360 - BGP Extended Communities Attribute

Extended communities. Every requirement listed for RFC 4360, the tests linked to it, and their recorded reviews. This summary is included in the automatic compliance check, ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 3 | of 5 mandatory requirements | a positive test checks that Ze does what the requirement demands and a negative one checks that it refuses what the requirement forbids |
| One polarity plus reason | 0 | of 5 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason supplies the complete test coverage required |
| One polarity, unexcused | 0 | of 5 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial test coverage | 0 | of 5 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unverified, with zero whole-requirement credit |
| Tests shown to detect a fault | 6 | 6 of 10 tests linked to requirements | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests verifies a requirement only after both tests have been shown to detect a fault |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Declared gap or no test | 2 | of 5 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 5 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 0 | of 5 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 0 | of 5 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze verified: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 5 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 5 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no test verification credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level) | 5 | of 9 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements included in the automatic checks: the compliance check (./le rfc check) holds Ze to each of them. This is the total being assessed; the counts above show the recorded status of those requirements. |

The 9 counts marked as a part above are the whole of the 5 mandatory requirements: they add up to 5. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

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
| Public row | Extended communities |
| Automatic checks | Included |
| Requirements | 9 |
| Mandatory requirements (MUST level) | 5 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 2 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 10 |
| Tests linked to requirements | 10 |
| Recorded audit verdicts | 2 |
| Fault test records | 6 |
| Summary | `rfc/short/rfc4360.md` |
| Requirement table | `rfc/requirements/rfc4360.md` |
| RFC text | `rfc/full/rfc4360.txt` |

## Automatic checks

Included in automatic checks: BGP Extended Communities: five MUST-level requirements. Three are tested with both polarities: RFC4360-2-1 (equal only when all 8 octets match) via ExtendedCommunity [8]byte equality, RFC4360-x-1 (length a multiple of 8) via ParseExtendedCommunities (wired at wire.go:412), and RFC4360-6-1 (decision process does not use Extended Communities to modify best-path selection, internal/component/bgp/plugins/rib/bestpath.go::comparePair). Two are gaps: RFC4360-7-1 and 7-2, IANA type-allocation rules no sentence scopes to a role, which no test checks against ze's type table. A non-supporting speaker's pass-through is RFC 4271's rule (RFC4271-5-3), so no RFC 4360 row carries it (rfc/corrections/rfc4360.md). The 6-2 SHOULD, 6-3 SHOULD NOT and 6-4/6-5 MAYs are not gated.

## Published support status

**Area:** Extended communities

**Test verification:** 3 of 5 MUST proven

**Evidence:** unit 3

**Unmet:** RFC4360-7-1, RFC4360-7-2


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 3 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 2 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 0 | one part of the mandatory requirements |
| Partial test coverage (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level)** | **5** | every mandatory requirement appears in exactly one group above |

**Positive and negative tests (3):** [`RFC4360-6-1`](#rfc4360-6-1), [`RFC4360-2-1`](#rfc4360-2-1), [`RFC4360-x-1`](#rfc4360-x-1)

**Annotated (including scoped evidence) (2):** [`RFC4360-7-1`](#rfc4360-7-1), [`RFC4360-7-2`](#rfc4360-7-2)

## Figures

- **MUST bar**: Ze verified 3, Not built 2
- **Overview bar**: Ze MUST verified 3, Not assessed 4, Not built 2

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | not complete |
| MUST rows | 5 |
| MUST rows kept (the MUST bar) | 5 |
| Ze verified | 3 |
| OS verified | 0 |
| Tests awaiting verification | 0 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 2 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 4 |
| SHOULD verified | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD tests awaiting verification | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 4 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze verified means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Tests awaiting verification counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

Source: [data/rfc/rfc4360.json](../../../data/rfc/rfc4360.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC4360-6-1` | The Extended Community attribute MUST NOT be used to modify the BGP best path selection algorithm in a way that leads to forwarding loops. (§6) | MUST NOT | 6 - Operations | **positive:** `unit/verify` [`TestRFC4360ExtendedCommunitiesDoNotAlterBestPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4360_bestpath_test.go#L31). **negative:** `unit/verify` [`TestRFC4360ExtendedCommunitiesDoNotAlterBestPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4360_bestpath_test.go#L35) |
| `RFC4360-7-1` | The value allocated for a regular Type MUST NOT be reused as the value of the high-order octet when allocating an extended Type. (§7) | MUST NOT | 7 - IANA Considerations | **positive:** no positive test. **negative:** no negative test. **{gap}:** an IANA allocation rule with no implementer role the RFC names; ze allocates no Type value, and its codec (internal/core/bgp/attribute/community.go) reads only codes IANA assigned; no test pins ze's type table against the registry, so nothing proves ze treats no regular Type as an extended Type's high-order octet |
| `RFC4360-7-2` | The value of the high-order octet allocated for an extended Type MUST NOT be reused when allocating a regular Type (§7) | MUST NOT | 7 - IANA Considerations | **positive:** no positive test. **negative:** no negative test. **{gap}:** the converse IANA allocation rule of RFC4360-7-1, with no implementer role the RFC names; ze allocates no Type value, and no test pins the high-order octets of the extended Types ze decodes (internal/core/bgp/attribute/community.go) against the registry's regular Types |
| `RFC4360-2-1` | Two extended communities are declared equal only when all 8 octets of the community are equal. (§2) | MUST | 2 - BGP Extended Communities Attribute | **positive:** `unit/verify` [`TestExtendedCommunityEquality`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/community_test.go#L300). **positive:** `unit/verify` [`TestRFC4360ExtendedCommunitiesEqualOnlyOnAllEightOctets`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4360_equality_test.go#L22). **negative:** `unit/verify` [`TestExtendedCommunityEquality`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/community_test.go#L302). **negative:** `unit/verify` [`TestRFC4360ExtendedCommunitiesEqualOnlyOnAllEightOctets`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4360_equality_test.go#L23) |
| `RFC4360-x-1` | The attribute consists of a set of "extended communities". All routes with the Extended Communities attribute belong to the communities listed in the attribute. Each Extended Community is encoded as an 8-octet quantity (§2) | MUST | 2 - BGP Extended Communities Attribute | **positive:** `unit/verify` [`TestExtendedCommunitiesParse`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/community_test.go#L280). **positive:** `unit/verify` [`TestRFC4360ExtendedCommunitiesAreWholeEightOctetQuantities`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc4360_length_test.go#L20). **negative:** `unit/verify` [`TestExtendedCommunitiesParseRejectsBadLength`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/community_test.go#L290). **negative:** `unit/verify` [`TestRFC4360ExtendedCommunitiesAreWholeEightOctetQuantities`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc4360_length_test.go#L21) |
| `RFC4360-6-2` | If a route has a non-transitivity extended community, then before advertising the route across the Autonomous System boundary the community SHOULD be removed from the route. (§6) | SHOULD | 6 - Operations | **positive:** no positive test. **negative:** no negative test |
| `RFC4360-6-3` | However, the community SHOULD NOT be removed when advertising the route across the BGP Confederation boundary. (§6) | SHOULD NOT | 6 - Operations | **positive:** no positive test. **negative:** no negative test |
| `RFC4360-6-4` | A BGP speaker receiving a route that doesn't have the Extended Communities attribute MAY append this attribute to the route when propagating it to its peers. (§6) | MAY | 6 - Operations | **positive:** no positive test. **negative:** no negative test |
| `RFC4360-6-5` | A BGP speaker receiving a route with the Extended Communities attribute MAY modify this attribute according to the local policy. (§6) | MAY | 6 - Operations | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC4360-7-1`](#rfc4360-7-1) The value allocated for a regular Type MUST NOT be reused as the value of the high-order octet when allocating an extended Type. (§7) | {gap}, no test | an IANA allocation rule with no implementer role the RFC names; ze allocates no Type value, and its codec (internal/core/bgp/attribute/community.go) reads only codes IANA assigned; no test pins ze's type table against the registry, so nothing proves ze treats no regular Type as an extended Type's high-order octet |
| [`RFC4360-7-2`](#rfc4360-7-2) The value of the high-order octet allocated for an extended Type MUST NOT be reused when allocating a regular Type (§7) | {gap}, no test | the converse IANA allocation rule of RFC4360-7-1, with no implementer role the RFC names; ze allocates no Type value, and no test pins the high-order octets of the extended Types ze decodes (internal/core/bgp/attribute/community.go) against the registry's regular Types |

## Test status

A test reads not verified when no record shows it failing after a deliberate fault. Until then, the test has not been shown to detect a fault in the required behavior.

### [`RFC4360-6-1`](#rfc4360-6-1)

The Extended Community attribute MUST NOT be used to modify the BGP best path selection algorithm in a way that leads to forwarding loops. (§6)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC4360ExtendedCommunitiesDoNotAlterBestPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4360_bestpath_test.go#L35) | unit/verify | revert, verified |
| positive | [`TestRFC4360ExtendedCommunitiesDoNotAlterBestPath`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4360_bestpath_test.go#L31) | unit/verify | revert, verified |

### [`RFC4360-7-1`](#rfc4360-7-1)

The value allocated for a regular Type MUST NOT be reused as the value of the high-order octet when allocating an extended Type. (§7)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4360-7-1, so there is no linked test.

### [`RFC4360-7-2`](#rfc4360-7-2)

The value of the high-order octet allocated for an extended Type MUST NOT be reused when allocating a regular Type (§7)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4360-7-2, so there is no linked test.

### [`RFC4360-2-1`](#rfc4360-2-1)

Two extended communities are declared equal only when all 8 octets of the community are equal. (§2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge 2026-09-30. Rib TestRFC4360ExtendedCommunitiesEqualOnlyOnAllEightOctets drives extCommunitiesEqual (rib_replay.go), the comparator that decides whether a replayed route's communities changed: + an octet-equal copy is equal; - flipping any one of the 8 octets makes it unequal. ExtendedCommunity is [8]byte, so equality elsewhere is whole-value by type; filter plugins match by policy, not equality, and are not driven.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC4360ExtendedCommunitiesEqualOnlyOnAllEightOctets`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4360_equality_test.go#L23) | unit/verify | revert, verified |
| negative | [`TestExtendedCommunityEquality`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/community_test.go#L302) | unit/verify | not verified |
| positive | [`TestRFC4360ExtendedCommunitiesEqualOnlyOnAllEightOctets`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4360_equality_test.go#L22) | unit/verify | revert, verified |
| positive | [`TestExtendedCommunityEquality`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/community_test.go#L300) | unit/verify | not verified |

### [`RFC4360-x-1`](#rfc4360-x-1)

The attribute consists of a set of "extended communities". All routes with the Extended Communities attribute belong to the communities listed in the attribute. Each Extended Community is encoded as an 8-octet quantity (§2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge 2026-09-30. TestRFC4360ExtendedCommunitiesAreWholeEightOctetQuantities over ParseExtendedCommunities, the registered parser: + 8 and 16 octets yield one and two communities holding the exact octets in order; - 12 and 20 octets are refused with ErrInvalidLength, not truncated.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestExtendedCommunitiesParseRejectsBadLength`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/community_test.go#L290) | unit/verify | not verified |
| negative | [`TestRFC4360ExtendedCommunitiesAreWholeEightOctetQuantities`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc4360_length_test.go#L21) | unit/verify | revert, verified |
| positive | [`TestExtendedCommunitiesParse`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/community_test.go#L280) | unit/verify | not verified |
| positive | [`TestRFC4360ExtendedCommunitiesAreWholeEightOctetQuantities`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/rfc4360_length_test.go#L20) | unit/verify | revert, verified |

## Requirement list review

| Field | Value |
|---|---|
| Reviewer | ze-work agent, spec-rfcgate-6 phase 5, rfc4360 |
| Signed off | 2026-08-31 |
| Register | prose |
| Source | rfc/full/rfc4360.txt |
| Source fingerprint | 759f302ace542b9f |
| Record | rfc/extraction/rfc4360.json |
| Mapped sentences | 3 |
| Declined as scope | 3 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | skipped (front-matter) | Title block, Status of This Memo, Copyright Notice and Abstract. The Abstract restates section 1: the attribute labels information carried in BGP-4 and the labels can control its distribution. No sentence directs a speaker. |
| `1` | Introduction | 0 | reviewed | Introduction. Indicative prose: the two enhancements over RFC 1997 (an extended range and a Type field), and what structure buys a policy writer. No directive. |
| `1.1` | Specification of Requirements | 0 | reviewed | Specification of Requirements. The RFC 2119 key-words paragraph. It tells a reader how to read the other sections and binds no speaker, which is why the derivation excludes it from the site inventory. Unlike RFC 7313 section 2 it does not add the 'only when in all upper case' sentence, so this walk reads a lowercase modal on its own merits rather than as a key word. |
| `2` | BGP Extended Communities Attribute | 0 | reviewed | BGP Extended Communities Attribute. The document's wire-format section, written entirely in the indicative, so the site scan sees nothing in it and two declared MUST-level rows are read from here. It states the attribute is transitive optional with Type Code 16 (the pass-through a non-supporting speaker owes that class is RFC 4271's rule, row RFC4271-5-3, so the RFC4360 row that claimed it was retired 2026-09-27), that each extended community is an 8-octet quantity, that the Type Field is 1 octet for a Regular type and 2 for an Extended type, that the high-order octet carries the I (IANA authority) and T (transitive) bits with T=0 transitive and T=1 non-transitive across ASes, and that 'Two extended communities are declared equal only when all 8 octets of the community are equal.' The two unsourced ids below are the 8-octet and the equality obligations. The remaining sentences are value definitions carried by the Wire Formats and Type Classification tables of rfc/short/rfc4360.md, and the closing 'The two members in the tuple <Type, Value> should be enumerated to specify any community value' is a lowercase modal describing how a value is written down, not a directive to a speaker. |
| `3` | Defined BGP Extended Community Types | 0 | reviewed | Defined BGP Extended Community Types. One paragraph naming what 3.1 to 3.3 define: templates identified by the high-order octet, with the low-order octet as sub-type. No directive. |
| `3.1` | Two-Octet AS Specific Extended Community | 0 | reviewed | Two-Octet AS Specific Extended Community. Value assignment: high-order octet 0x00 or 0x40, a 2-octet Global Administrator holding an IANA-assigned AS number and a 4-octet Local Administrator. Its one modal, 'The format and meaning of the value encoded in this sub-field should be defined by the sub-type of the community', is lowercase and binds whoever defines a future sub-type, not a speaker encoding one. The layout is carried by the Two-Octet AS Specific table of rfc/short/rfc4360.md. |
| `3.2` | IPv4 Address Specific Extended Community | 0 | reviewed | IPv4 Address Specific Extended Community. Value assignment: high-order octet 0x01 or 0x41, a 4-octet Global Administrator holding a registry-assigned IPv4 address and a 2-octet Local Administrator. Its lowercase 'should be defined by the sub-type' sentence binds a future sub-type definition, as in 3.1. The layout is carried by the IPv4 Address Specific table of rfc/short/rfc4360.md. |
| `3.3` | Opaque Extended Community | 0 | reviewed | Opaque Extended Community. Value assignment: high-order octet 0x03 or 0x43 and a 6-octet opaque Value. States that the sub-type defining the Value Field is to be assigned by IANA, which is an allocation fact rather than a directive to a speaker. The layout is carried by the Opaque Extended Community table of rfc/short/rfc4360.md. |
| `4` | Route Target Community | 0 | reviewed | Route Target Community. Value assignment: an extended type whose high-order octet is 0x00, 0x01 or 0x02 and whose low-order octet is 0x02, transitive across the AS boundary, with the Local Administrator drawn from the numbering space of the organization holding the AS number or IP address in the Global Administrator sub-field. Every sentence is indicative, and the use is deferred to RFC 4364. Carried by the Sub-Types table of rfc/short/rfc4360.md. |
| `5` | Route Origin Community | 0 | reviewed | Route Origin Community. The same shape as section 4 with low-order octet 0x03: an extended type, transitive across the AS boundary, Local Administrator drawn from the holder's numbering space, use deferred to RFC 4364. Indicative throughout. Carried by the Sub-Types table of rfc/short/rfc4360.md. |
| `6` | Operations | 1 | reviewed | Operations. The only section that directs a BGP speaker. Its one MUST-level site is 6:1, mapped below to RFC4360-6-1. Its four remaining directives are the two MAYs (append the attribute to a route that lacks it, modify the attribute per local policy) and the SHOULD and SHOULD NOT that bracket a non-transitive community at an AS boundary against a Confederation boundary; those are the four unsourced ids below, advisory and never gated. Two further sentences the site scan does not see are not obligations: the aggregation paragraph states a lowercase-'should' DEFAULT that the same paragraph says 'could be overridden via local configuration', and the closing paragraph is indicative division of labour, saying a route may carry both attributes and that the RFC 1997 one is handled per RFC 1997. |
| `7` | IANA Considerations | 3 | reviewed | IANA Considerations. Walked rather than skipped because two of its three sites are MUST-level sentences the summary declares as rows (RFC4360-7-1 and RFC4360-7-2), so an `iana` skip would hide two gated ids behind a skip kind. Both are mapped below, and both bind the registry: rfc/short/rfc4360.md records that as their {not-applicable} annotation, which is the summary's judgement and not this walk's. The rest of the section creates the 'BGP Extended Communities Type' registry and the three per-class registries, fixes the Standards Action, Early IANA Allocation, First Come First Served and Experimental ranges, and assigns 0x0002, 0x0003, 0x0102 and 0x0103. Those ranges and values are carried by the Type Classification and IANA Registered Type Values tables of rfc/short/rfc4360.md. |
| `8` | Security Considerations | 1 | reviewed | Security Considerations. States that the extension has security implications similar to RFC 1997 and changes no underlying security issue, then places one condition on the operator (site 8:1), and puts the mechanism providing that trust relationship out of scope. No countermeasure is directed at a speaker. |
| `9` | Acknowledgements | 0 | skipped (acknowledgements) | Acknowledgements. |
| `10` | not stated | 0 | skipped (references) | Normative References: RFC 4271, RFC 1997, RFC 2119, RFC 2434, RFC 4020. |
| `11` | Informative References: RFC 4364 | 1 | skipped (references) | Informative References: RFC 4364. The derived section span runs to the end of the document, so it also holds Authors' Addresses, the Full Copyright Statement, the Intellectual Property boilerplate and the RFC Editor funding note. Site 11:1 is one sentence of that IPR boilerplate and is excluded below; nothing in the span states an obligation on a BGP speaker. |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `7:3` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the party filing a future Type value assignment request with IANA, and IANA which records the answer: the sentence directs what such a REQUEST must specify, namely whether the value is for a transitive or a non-transitive Extended Community. Ze consumes allocated type codes and files no assignment request, so it never plays the requester role. The lowercase 'must' is why the site scan sees it only under the prose register. The role is a registry act by a person, so no producer could perform it. Ze CONSUMES the resulting assignment: the extended community codec reads the type octet it is given (`internal/core/bgp/attribute`). | Future requests for assignment of a Type value must specify whether the Type value is intended for a transitive or a non- transitive Extended Community. |
| `8:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the network operator: the sentence conditions an operator who RELIES on information carried in BGP on having a transitive trust relationship back to the source, and the next sentence puts the mechanism providing that relationship beyond the scope of the document. It directs no encoding, decoding or propagation behavior a BGP speaker performs, and there is nothing for a daemon to implement. The role is the AS operator, so no producer could act as it. Ze CONSUMES the operator's decision: the capability is negotiated per session in the reactor (`internal/component/bgp/reactor`), which advertises what it is configured to and decides no AS-wide policy. | Specifically, an operator who is relying on the information carried in BGP must have a transitive trust relationship back to the source of the information. |
| `11:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Non-normative use: this is the IETF's standard Intellectual Property boilerplate, which the derived section span carries into section 11 after the Informative References. The sentence invites interested parties to disclose patent rights to the IETF, and its 'may be required to implement this standard' describes what a patent might cover rather than requiring anything of an implementation. | The IETF invites any interested party to bring to its attention any copyrights, patents or patent applications, or other proprietary rights that may cover technology that may be required to implement this standard. |

## Superseded

No document obsoletes RFC 4360, so its obligations are stated where they were written.
