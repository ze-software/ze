# RFC 9319 - The Use of maxLength in the Resource Public Key Infrastructure (RPKI)

No entry in the public support list. Every requirement listed for RFC 9319, the tests linked to it, and their recorded reviews. This summary is excluded from automatic compliance checks.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0 | of 4 mandatory requirements | a positive test checks that Ze does what the requirement demands and a negative one checks that it refuses what the requirement forbids |
| One polarity plus reason | 0 | of 4 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason supplies the complete test coverage required |
| One polarity, unexcused | 0 | of 4 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial test coverage | 0 | of 4 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unverified, with zero whole-requirement credit |
| Tests shown to detect a fault | 0 | 0 of 0 tests linked to requirements | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests verifies a requirement only after both tests have been shown to detect a fault |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Declared gap or no test | 4 | of 4 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |

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
| Mandatory requirements (MUST level), not checked | 4 | of 11 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements listed in this summary. This RFC is excluded from automatic checks (foundation), so these requirements are not checked. The counts above show what the summary records. |

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
| Mandatory requirements (MUST level), not checked | outlined | outlined, with no colored edge: it is the total the other cards are counted against, not a measure, and a larger one is neither good news nor bad |

## At a glance

| Field | Value |
|---|---|
| Public row | No entry in the public support list |
| Automatic checks | Excluded (foundation) |
| Requirements | 11 |
| Mandatory requirements (MUST level), not checked | 4 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 4 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 0 |
| Tests linked to requirements | 0 |
| Recorded audit verdicts | 0 |
| Fault test records | 0 |
| Summary | `rfc/short/rfc9319.md` |
| Requirement table | `rfc/requirements/rfc9319.md` |
| RFC text | `rfc/full/rfc9319.txt` |

## Automatic checks

Excluded from automatic checks (foundation, the document defines, registers or describes, and obliges no implementer, so there is no implementation anywhere for a gate to hold): A Best Current Practice addressed to operators and ROA issuers rather than to a router.

## Published support status

No entry in the public support list, so its summary declares `| Support | - |` and docs/features/rfc-status.md carries no row for RFC 9319.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 0 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 4 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 0 | one part of the mandatory requirements |
| Partial test coverage (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level), not checked** | **4** | every mandatory requirement appears in exactly one group above |

**Annotated (including scoped evidence) (4):** [`RFC9319-5-1`](#rfc9319-5-1), [`RFC9319-5-2`](#rfc9319-5-2), [`RFC9319-5-3`](#rfc9319-5-3), [`RFC9319-6-1`](#rfc9319-6-1)

## Figures

- **MUST bar**: Not built 4
- **Overview bar**: Not assessed 7, Not built 4

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | not complete |
| MUST rows | 4 |
| MUST rows kept (the MUST bar) | 4 |
| Ze verified | 0 |
| OS verified | 0 |
| Tests awaiting verification | 0 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 4 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 7 |
| SHOULD verified | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD tests awaiting verification | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 7 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze verified means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Tests awaiting verification counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

Source: [data/rfc/rfc9319.json](../../../data/rfc/rfc9319.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC9319-5-1` | Operators that implement these recommendations and have existing ROAs published in the RPKI system MUST perform a review of such objects, especially where they make use of the maxLength attribute, to ensure that the set of included prefixes is "minimal" with respect to the current BGP origination and routing policies. (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** binds operators who publish ROAs, a role ze never fills: ze is an RPKI Relying Party that consumes Validated ROA Payloads over RTR (internal/component/bgp/plugins/rpki/roa_cache.go) and publishes no ROA. The subject carries a relative clause and Section 5 holds no scoping sentence, so neither not-applicable shape takes the row; it stays a gap, uncounted under foundation |
| `RFC9319-5-2` | Published ROAs MUST be replaced as necessary (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** binds the ROA issuer, a role ze never fills: ze keeps only a read-only VRP cache filled over RTR (internal/component/bgp/plugins/rpki/roa_cache.go) and publishes no ROA. The passive subject names no party and Section 5 holds no scoping sentence, so neither not-applicable shape takes the row; it stays a gap, uncounted under foundation |
| `RFC9319-5-3` | Such an exercise MUST be repeated whenever the operator makes changes to either policy. (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** binds the operator who reviews its own ROAs, a role ze never fills: ze issues no ROA and only re-validates received routes when the VRP cache changes (internal/component/bgp/plugins/rpki/origin_tracker.go, under RFC 6811). The subject names an exercise, not a party, and Section 5 holds no scoping sentence, so neither not-applicable shape takes the row; it stays a gap, uncounted under foundation |
| `RFC9319-6-1` | * Operators providing a means for operators of neighboring autonomous systems to advertise RTDR routes via BGP MUST NOT make the creation of non-minimal ROAs a pre-requisite for its use. (§6) | MUST NOT | 6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** binds operators that offer neighbouring networks a way to advertise RTDR routes, a role ze never fills: ze sells no such service and gates no feature on any ROA (internal/component/bgp/plugins/rpki/validate.go validates only its own received routes). The subject carries a qualifying phrase and Section 6 holds no scoping sentence, so neither not-applicable shape takes the row; it stays a gap, uncounted under foundation |
| `RFC9319-5-4` | Operators SHOULD use minimal ROAs whenever possible. A minimal ROA contains only those IP prefixes that are actually originated by an AS in BGP and no other IP prefixes. (§5) | SHOULD | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC9319-5-5` | In general, operators SHOULD avoid using the maxLength attribute in their ROAs, since its inclusion will usually make the ROA non- minimal. (§5) | SHOULD | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC9319-5.1-1` | In this case, the ROA SHOULD only include: (1) the set of IP prefixes that are always originated in BGP, and (2) the set of IP prefixes that are sometimes, but not always, originated in BGP. (§5.1) | SHOULD | 5.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9319-5.1-2` | The ROA SHOULD NOT include any IP prefixes that the operator knows will not be originated in BGP. (§5.1) | SHOULD NOT | 5.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9319-5.1-3` | In general, the ROA SHOULD NOT make use of the maxLength attribute unless doing so has no impact on the set of included prefixes. (§5.1) | SHOULD NOT | 5.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9319-6-2` | Operators SHOULD NOT create non-minimal ROAs (by either creating additional ROAs or using the maxLength attribute) for the purpose of advertising RTDR routes (§6) | SHOULD NOT | 6 | **positive:** no positive test. **negative:** no negative test |
| `RFC9319-7-1` | This document recommends that designers and/or providers of such user interfaces SHOULD provide warnings to draw the user's attention to the risks of creating non-minimal ROAs in general and using the maxLength attribute in particular. (§7) | SHOULD | 7 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC9319-5-1`](#rfc9319-5-1) Operators that implement these recommendations and have existing ROAs published in the RPKI system MUST perform a review of such objects, especially where they make use of the maxLength attribute, to ensure that the set of included prefixes is "minimal" with respect to the current BGP origination and routing policies. (§5) | {gap}, no test | binds operators who publish ROAs, a role ze never fills: ze is an RPKI Relying Party that consumes Validated ROA Payloads over RTR (internal/component/bgp/plugins/rpki/roa_cache.go) and publishes no ROA. The subject carries a relative clause and Section 5 holds no scoping sentence, so neither not-applicable shape takes the row; it stays a gap, uncounted under foundation |
| [`RFC9319-5-2`](#rfc9319-5-2) Published ROAs MUST be replaced as necessary (§5) | {gap}, no test | binds the ROA issuer, a role ze never fills: ze keeps only a read-only VRP cache filled over RTR (internal/component/bgp/plugins/rpki/roa_cache.go) and publishes no ROA. The passive subject names no party and Section 5 holds no scoping sentence, so neither not-applicable shape takes the row; it stays a gap, uncounted under foundation |
| [`RFC9319-5-3`](#rfc9319-5-3) Such an exercise MUST be repeated whenever the operator makes changes to either policy. (§5) | {gap}, no test | binds the operator who reviews its own ROAs, a role ze never fills: ze issues no ROA and only re-validates received routes when the VRP cache changes (internal/component/bgp/plugins/rpki/origin_tracker.go, under RFC 6811). The subject names an exercise, not a party, and Section 5 holds no scoping sentence, so neither not-applicable shape takes the row; it stays a gap, uncounted under foundation |
| [`RFC9319-6-1`](#rfc9319-6-1) * Operators providing a means for operators of neighboring autonomous systems to advertise RTDR routes via BGP MUST NOT make the creation of non-minimal ROAs a pre-requisite for its use. (§6) | {gap}, no test | binds operators that offer neighbouring networks a way to advertise RTDR routes, a role ze never fills: ze sells no such service and gates no feature on any ROA (internal/component/bgp/plugins/rpki/validate.go validates only its own received routes). The subject carries a qualifying phrase and Section 6 holds no scoping sentence, so neither not-applicable shape takes the row; it stays a gap, uncounted under foundation |

## Test status

A test reads not verified when no record shows it failing after a deliberate fault. Until then, the test has not been shown to detect a fault in the required behavior.

### [`RFC9319-5-1`](#rfc9319-5-1)

Operators that implement these recommendations and have existing ROAs published in the RPKI system MUST perform a review of such objects, especially where they make use of the maxLength attribute, to ensure that the set of included prefixes is "minimal" with respect to the current BGP origination and routing policies. (§5)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC9319-5-1, so there is no linked test.

### [`RFC9319-5-2`](#rfc9319-5-2)

Published ROAs MUST be replaced as necessary (§5)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC9319-5-2, so there is no linked test.

### [`RFC9319-5-3`](#rfc9319-5-3)

Such an exercise MUST be repeated whenever the operator makes changes to either policy. (§5)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC9319-5-3, so there is no linked test.

### [`RFC9319-6-1`](#rfc9319-6-1)

* Operators providing a means for operators of neighboring autonomous systems to advertise RTDR routes via BGP MUST NOT make the creation of non-minimal ROAs a pre-requisite for its use. (§6)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC9319-6-1, so there is no linked test.

## Requirement list review

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | rfc2119 |
| Source | rfc/full/rfc9319.txt |
| Source fingerprint | 0b377f370cf81004 |
| Record | rfc/extraction/rfc9319.json |
| Mapped sentences | 4 |
| Declined as scope | 0 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | reviewed | not stated |
| `1` | not stated | 0 | reviewed | not stated |
| `1.1` | not stated | 0 | reviewed | not stated |
| `1.2` | not stated | 0 | reviewed | not stated |
| `2` | not stated | 0 | reviewed | not stated |
| `3` | not stated | 0 | reviewed | not stated |
| `4` | not stated | 0 | reviewed | not stated |
| `5` | not stated | 3 | reviewed | not stated |
| `5.1` | not stated | 0 | reviewed | not stated |
| `5.2` | not stated | 0 | reviewed | not stated |
| `6` | not stated | 1 | reviewed | not stated |
| `7` | not stated | 0 | reviewed | not stated |
| `8` | not stated | 0 | reviewed | not stated |
| `9` | not stated | 0 | reviewed | not stated |
| `10` | not stated | 0 | reviewed | not stated |
| `11` | not stated | 0 | reviewed | not stated |
| `11.1` | not stated | 0 | reviewed | not stated |
| `11.2` | not stated | 0 | reviewed | not stated |

### Excluded sentences

The review of RFC 9319 excluded no sentences: every sentence identified as an obligation is linked to a requirement.

## Superseded

No document obsoletes RFC 9319, so its obligations are stated where they were written.
