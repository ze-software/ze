# RFC 8097 - BGP Prefix Origin Validation State Extended Community

RPKI origin validation state extended community. Every requirement listed for RFC 8097, the tests linked to it, and their recorded reviews. This summary is included in the automatic compliance check, ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0 | of 5 mandatory requirements | a positive test checks that Ze does what the requirement demands and a negative one checks that it refuses what the requirement forbids |
| One polarity plus reason | 0 | of 5 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason supplies the complete test coverage required |
| One polarity, unexcused | 0 | of 5 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial test coverage | 0 | of 5 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unverified, with zero whole-requirement credit |
| Tests shown to detect a fault | 0 | 0 of 0 tests linked to requirements | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests verifies a requirement only after both tests have been shown to detect a fault |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Declared gap or no test | 5 | of 5 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |

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
| Mandatory requirements (MUST level) | 5 | of 10 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements included in the automatic checks: the compliance check (./le rfc check) holds Ze to each of them. This is the total being assessed; the counts above show the recorded status of those requirements. |

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
| Public row | RPKI origin validation state extended community |
| Automatic checks | Included |
| Requirements | 10 |
| Mandatory requirements (MUST level) | 5 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 5 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 0 |
| Tests linked to requirements | 0 |
| Recorded audit verdicts | 0 |
| Fault test records | 0 |
| Summary | `rfc/short/rfc8097.md` |
| Requirement table | `rfc/requirements/rfc8097.md` |
| RFC text | `rfc/full/rfc8097.txt` |

## Automatic checks

Included in automatic checks: BGP Prefix Origin Validation State Extended Community: five MUST-level requirements, all gaps. Ze performs local RFC 6811 origin validation (internal/component/bgp/plugins/rpki/validate.go) but does not implement the RFC 8097 origin validation state extended community: it has no encoder or decoder for the type-0x43 community and treats every extended community as an opaque 8-octet value (internal/core/bgp/attribute/community.go, ExtendedCommunity and ParseExtendedCommunities). An operator can still configure one in the 0x hex form with any Reserved value, and one received from an EBGP peer is readvertised unchanged rather than dropped (RFC8097-2-4). No owner decision declines the community, so each row is owed.

## Published support status

**Area:** RPKI origin validation state extended community

**Test verification:** 0 of 5 MUST proven

**Evidence:** -

**Unmet:** RFC8097-2-1, RFC8097-2-2, RFC8097-2-3, RFC8097-2-4, RFC8097-2-5


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 0 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 5 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 0 | one part of the mandatory requirements |
| Partial test coverage (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level)** | **5** | every mandatory requirement appears in exactly one group above |

**Annotated (including scoped evidence) (5):** [`RFC8097-2-1`](#rfc8097-2-1), [`RFC8097-2-2`](#rfc8097-2-2), [`RFC8097-2-3`](#rfc8097-2-3), [`RFC8097-2-4`](#rfc8097-2-4), [`RFC8097-2-5`](#rfc8097-2-5)

## Figures

- **MUST bar**: Not built 5
- **Overview bar**: Not assessed 5, Not built 5

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | not complete |
| MUST rows | 5 |
| MUST rows kept (the MUST bar) | 5 |
| Ze verified | 0 |
| OS verified | 0 |
| Tests awaiting verification | 0 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 5 |
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

Source: [data/rfc/rfc8097.json](../../../data/rfc/rfc8097.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC8097-2-1` | The Reserved field MUST be set to 0 (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze never builds the community itself, but an operator can configure the community in the 0x hex form with any value (internal/core/bgp/attribute/extcomm_hex.go::ParseExtendedCommunityHex interprets no type octets), so nothing refuses a non-zero Reserved field. ze implements no origin validation state extended community: internal/core/bgp/attribute/community.go::ParseExtendedCommunities keeps every extended community as eight opaque octets and nothing reads type 0x43 sub-type 0x00. No owner decision declines the community, so this is owed with it |
| `RFC8097-2-2` | The Reserved field MUST be set to 0 and ignored upon the receipt of this community. (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze reads no Reserved field of the community. ze implements no origin validation state extended community: internal/core/bgp/attribute/community.go::ParseExtendedCommunities keeps every extended community as eight opaque octets and nothing reads type 0x43 sub-type 0x00. No owner decision declines the community, so this is owed with it |
| `RFC8097-2-3` | However, if more than one instance is received, an implementation MUST disregard all instances other than the one with the numerically greatest validation state value. (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze compares no validation state across instances. ze implements no origin validation state extended community: internal/core/bgp/attribute/community.go::ParseExtendedCommunities keeps every extended community as eight opaque octets and nothing reads type 0x43 sub-type 0x00. No owner decision declines the community, so this is owed with it |
| `RFC8097-2-4` | By default, implementations MUST drop the origin validation state extended community if received from an External BGP (EBGP) peer, without processing it further. (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** a community received from an EBGP peer is kept and readvertised unchanged, to IBGP peers included (internal/core/bgp/attribute/community.go::ExtendedCommunities.WriteTo copies it back out), so the default drop does not happen. ze implements no origin validation state extended community: internal/core/bgp/attribute/community.go::ParseExtendedCommunities keeps every extended community as eight opaque octets and nothing reads type 0x43 sub-type 0x00. No owner decision declines the community, so this is owed with it |
| `RFC8097-2-5` | If the value received is greater than the largest specified value (2), the implementation MUST apply a strategy similar to attribute discard [RFC7606] by discarding the erroneous community and logging the error for further analysis. (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze reads no validation state value, so a value above 2 is neither discarded nor logged. ze implements no origin validation state extended community: internal/core/bgp/attribute/community.go::ParseExtendedCommunities keeps every extended community as eight opaque octets and nothing reads type 0x43 sub-type 0x00. No owner decision declines the community, so this is owed with it |
| `RFC8097-2-6` | If the router is configured to support the extensions defined in this document, it SHOULD attach the origin validation state extended community to BGP UPDATE messages sent to IBGP peers by mapping the computed validation state in the last octet of the extended community. (§2) | SHOULD | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC8097-2-7` | Similarly, a receiving BGP speaker, in the absence of validation state set based on local data, SHOULD derive a validation state from the last octet of the extended community, if present. (§2) | SHOULD | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC8097-2-8` | An implementation SHOULD NOT send more than one instance of the origin validation state extended community. (§2) | SHOULD NOT | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC8097-2-9` | Similarly, by default, an implementation SHOULD NOT send the community to EBGP peers. (§2) | SHOULD NOT | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC8097-2-10` | However, it SHOULD be possible to configure an implementation to send or accept the community when warranted. (§2) | SHOULD | 2 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC8097-2-1`](#rfc8097-2-1) The Reserved field MUST be set to 0 (§2) | {gap}, no test | ze never builds the community itself, but an operator can configure the community in the 0x hex form with any value (internal/core/bgp/attribute/extcomm_hex.go::ParseExtendedCommunityHex interprets no type octets), so nothing refuses a non-zero Reserved field. ze implements no origin validation state extended community: internal/core/bgp/attribute/community.go::ParseExtendedCommunities keeps every extended community as eight opaque octets and nothing reads type 0x43 sub-type 0x00. No owner decision declines the community, so this is owed with it |
| [`RFC8097-2-2`](#rfc8097-2-2) The Reserved field MUST be set to 0 and ignored upon the receipt of this community. (§2) | {gap}, no test | ze reads no Reserved field of the community. ze implements no origin validation state extended community: internal/core/bgp/attribute/community.go::ParseExtendedCommunities keeps every extended community as eight opaque octets and nothing reads type 0x43 sub-type 0x00. No owner decision declines the community, so this is owed with it |
| [`RFC8097-2-3`](#rfc8097-2-3) However, if more than one instance is received, an implementation MUST disregard all instances other than the one with the numerically greatest validation state value. (§2) | {gap}, no test | ze compares no validation state across instances. ze implements no origin validation state extended community: internal/core/bgp/attribute/community.go::ParseExtendedCommunities keeps every extended community as eight opaque octets and nothing reads type 0x43 sub-type 0x00. No owner decision declines the community, so this is owed with it |
| [`RFC8097-2-4`](#rfc8097-2-4) By default, implementations MUST drop the origin validation state extended community if received from an External BGP (EBGP) peer, without processing it further. (§2) | {gap}, no test | a community received from an EBGP peer is kept and readvertised unchanged, to IBGP peers included (internal/core/bgp/attribute/community.go::ExtendedCommunities.WriteTo copies it back out), so the default drop does not happen. ze implements no origin validation state extended community: internal/core/bgp/attribute/community.go::ParseExtendedCommunities keeps every extended community as eight opaque octets and nothing reads type 0x43 sub-type 0x00. No owner decision declines the community, so this is owed with it |
| [`RFC8097-2-5`](#rfc8097-2-5) If the value received is greater than the largest specified value (2), the implementation MUST apply a strategy similar to attribute discard [RFC7606] by discarding the erroneous community and logging the error for further analysis. (§2) | {gap}, no test | ze reads no validation state value, so a value above 2 is neither discarded nor logged. ze implements no origin validation state extended community: internal/core/bgp/attribute/community.go::ParseExtendedCommunities keeps every extended community as eight opaque octets and nothing reads type 0x43 sub-type 0x00. No owner decision declines the community, so this is owed with it |

## Test status

A test reads not verified when no record shows it failing after a deliberate fault. Until then, the test has not been shown to detect a fault in the required behavior.

### [`RFC8097-2-1`](#rfc8097-2-1)

The Reserved field MUST be set to 0 (§2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC8097-2-1, so there is no linked test.

### [`RFC8097-2-2`](#rfc8097-2-2)

The Reserved field MUST be set to 0 and ignored upon the receipt of this community. (§2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC8097-2-2, so there is no linked test.

### [`RFC8097-2-3`](#rfc8097-2-3)

However, if more than one instance is received, an implementation MUST disregard all instances other than the one with the numerically greatest validation state value. (§2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC8097-2-3, so there is no linked test.

### [`RFC8097-2-4`](#rfc8097-2-4)

By default, implementations MUST drop the origin validation state extended community if received from an External BGP (EBGP) peer, without processing it further. (§2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC8097-2-4, so there is no linked test.

### [`RFC8097-2-5`](#rfc8097-2-5)

If the value received is greater than the largest specified value (2), the implementation MUST apply a strategy similar to attribute discard [RFC7606] by discarding the erroneous community and logging the error for further analysis. (§2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC8097-2-5, so there is no linked test.

## Requirement list review

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | prose |
| Source | rfc/full/rfc8097.txt |
| Source fingerprint | 0e52119a0d75abc0 |
| Record | rfc/extraction/rfc8097.json |
| Mapped sentences | 4 |
| Declined as scope | 1 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 1 | reviewed | not stated |
| `1` | not stated | 0 | reviewed | not stated |
| `1.1` | not stated | 0 | reviewed | not stated |
| `2` | not stated | 4 | reviewed | not stated |
| `3` | not stated | 0 | reviewed | not stated |
| `4` | not stated | 0 | reviewed | not stated |
| `5` | not stated | 0 | reviewed | not stated |
| `6` | not stated | 0 | reviewed | not stated |
| `6.1` | not stated | 0 | reviewed | not stated |
| `6.2` | not stated | 0 | reviewed | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `front:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | IETF Trust copyright boilerplate in the Status of This Memo section: it binds the extraction of code components from the document, not a BGP speaker, and the keyword is lowercase 'must'. | Code Components extracted from this document must include Simplified BSD License text as described in Section 4.e of the Trust Legal Provisions and are provided without warranty as described in the Simplified BSD License. |

## Superseded

No document obsoletes RFC 8097, so its obligations are stated where they were written.
