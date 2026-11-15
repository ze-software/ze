# RFC 7012 - Information Model for IP Flow Information Export (IPFIX)

IPFIX Information Model. Every requirement listed for RFC 7012, the tests linked to it, and their recorded reviews. This summary is included in the automatic compliance check, ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0 | of 6 mandatory requirements | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 1 | of 6 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0 | of 6 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0 | of 6 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Tests shown to detect a fault | 2 | 2 of 2 tests linked to requirements | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests counts as proof only after this check has demonstrated that they can detect a fault |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Declared gap or no test | 5 | of 6 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 6 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 0 | of 6 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 0 | of 6 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze proven: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 6 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 6 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no proof credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level) | 6 | of 6 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements included in the automatic checks: the compliance check (./le rfc check) holds Ze to each of them. This is the total being assessed; the counts above show the recorded status of those requirements. |

The 9 counts marked as a part above are the whole of the 6 mandatory requirements: they add up to 6. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got. An outlined card with no colored edge, under Totals, is a total the other cards are counted against, not a measure.

| Card | Tone here | Why that color |
|---|---|---|
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the count under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| Partial proof; remaining gap | ok | green at zero, RED above it: a tested clause cannot prove the whole requirement |
| Declared gap or no test | bad | green at zero, RED above it: a binding obligation that nothing shows Ze meets is owed, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green counts above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Linux cannot do this | neutral | neutral at every value: an evidenced platform limit is neither a proof nor a scope exemption |
| Tests shown to detect a fault | ok | green at every value: these tests have been shown to detect a deliberate fault. This card counts individual tests separately, so its total cannot be added to the requirement counts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a mandatory requirement is unreviewed, green when every one is judged sound and current |
| Mandatory requirements (MUST level) | outlined | outlined, with no colored edge: it is the total the other cards are counted against, not a measure, and a larger one is neither good news nor bad |

## At a glance

| Field | Value |
|---|---|
| Public row | IPFIX Information Model |
| Automatic checks | Included |
| Requirements | 6 |
| Mandatory requirements (MUST level) | 6 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 5 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 2 |
| Tests linked to requirements | 2 |
| Recorded audit verdicts | 1 |
| Fault test records | 2 |
| Summary | `rfc/short/rfc7012.md` |
| Requirement table | `rfc/requirements/rfc7012.md` |
| RFC text | `rfc/full/rfc7012.txt` |

## Automatic checks

Included in automatic checks: IPFIX Information Model: exporter role. Six rows after the 2026-09-21 extraction walk: 3 gaps in proof (2.1-2, 2.1-3, 4-2, all enterprise-specific IEs ze never emits), 1 gap for IANA-registry authorship that fits neither not-applicable shape (2.1-1), 1 single-polarity positive (4-1 IE identifier 0 never used) and 1 gap in proof, a MUST NOT added by the walk (2.1-5, a value outside an IE's valid inclusive range must not be exported: no exported IE has a registry Range). The walk deleted six rows that stated RFC 7011 obligations and one SHOULD no sentence of RFC 7012 carries; most of this document's remaining MUSTs bind the author of an IE definition in the IANA registry and are excluded in rfc/extraction/rfc7012.json.

## Published support status

**Area:** IPFIX Information Model

**Proof:** 1 of 6 MUST proven

**Evidence:** unit 1

**Unmet:** RFC7012-2.1-1, RFC7012-2.1-2, RFC7012-2.1-3, RFC7012-2.1-5, RFC7012-4-2


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 0 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 6 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 0 | one part of the mandatory requirements |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level)** | **6** | every mandatory requirement appears in exactly one group above |

**Annotated (including scoped evidence) (6):** [`RFC7012-2.1-1`](#rfc7012-2.1-1), [`RFC7012-2.1-2`](#rfc7012-2.1-2), [`RFC7012-2.1-3`](#rfc7012-2.1-3), [`RFC7012-2.1-5`](#rfc7012-2.1-5), [`RFC7012-4-1`](#rfc7012-4-1), [`RFC7012-4-2`](#rfc7012-4-2)

## Figures

- **MUST bar**: Ze proven 1, Not built 5
- **Overview bar**: Ze MUST proven 1, Not built 5

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | not complete |
| MUST rows | 6 |
| MUST rows kept (the MUST bar) | 6 |
| Ze proven | 1 |
| OS verified | 0 |
| Proof pending | 0 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 5 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 0 |
| SHOULD proven | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD proof pending | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 0 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze proven means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Proof pending counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

Source: [data/rfc/rfc7012.json](../../../data/rfc/rfc7012.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC7012-2.1-1` | All Information Elements specified for the IPFIX protocol MUST have the following properties defined: name - A unique and meaningful name for the Information Element. elementId - A numeric identifier of the Information Element. If this identifier is used without an enterprise identifier (see [RFC7011] and the definition of enterpriseId listed below), then it is globally unique, and the list of allowed values is administered by IANA. It is used for compact identification of an Information Element when encoding Templates in the protocol. description - The semantics of this Information Element. Describes how this Information Element is derived from the Flow or other information available to the observer. Information Elements of dataType string or octetArray that have length constraints (fixed length, minimum and/or maximum length) MUST note these constraints in their descriptions. dataType - One of the types listed in Section 3.1 of this document or registered in the IANA "IPFIX Information Element Data Types" subregistry. The type space for attributes is constrained to facilitate implementation. The existing type space encompasses most primitive types used in modern programming languages, as well as some derived types (such as ipv4Address) that are common to this domain. status - The status of the specification of this Information Element. Allowed values are 'current' and 'deprecated'. (Section 2.1) | MUST | 2.1 - Information Element properties | **positive:** no positive test. **negative:** no negative test. **{gap}:** the sentence binds whoever specifies an Information Element, the IANA registry and the documents that define IEs, a role ze never fills, and its subject carries a qualifying phrase that fits neither not-applicable shape; ze defines no Information Element and exports only IANA-registered ones (internal/plugins/flowexport/ipfix/ie.go), so it has nothing of its own to test |
| `RFC7012-2.1-2` | Enterprise-specific Information Elements MUST have the following property defined: enterpriseId (Section 2.1) | MUST | 2.1 - Information Element properties | **positive:** no positive test. **negative:** no negative test. **{gap}:** gap in proof: internal/plugins/flowexport/ipfix/flow_template.go::BuildFlowTemplate meets it by writing only IANA field specifiers with the Enterprise bit clear, so no enterprise-specific Information Element lacks an enterpriseId; no test asserts it |
| `RFC7012-2.1-3` | If specifications of enterprise-specific Information Elements are made public and/or if enterprise-specific identifiers are used by the IPFIX protocol outside the enterprise, then the enterprise- specific identifier MUST be made globally unique by combining it with an enterprise identifier. (Section 2.1) | MUST | 2.1 - Information Element properties | **positive:** no positive test. **negative:** no negative test. **{gap}:** gap in proof: internal/plugins/flowexport/ipfix/flow_template.go::BuildFlowTemplate meets it by writing only IANA field specifiers with the Enterprise bit clear, so ze uses no enterprise-specific identifier that would need an enterprise number; no test asserts it |
| `RFC7012-2.1-5` | values for this Information Element outside the range are invalid and MUST NOT be exported. (Section 2.1) | MUST NOT | 2.1 - Information Element properties | **positive:** no positive test. **negative:** no negative test. **{gap}:** gap in proof: none of the Information Elements the exporter writes (internal/plugins/flowexport/ipfix/ie.go: elements 1, 2, 4, 7, 8, 10, 11, 12, 14, 16, 17, 27, 28, 85, 86 and 150 to 153) has a Range in the IANA IPFIX registry, so no out-of-range value can be exported; no test asserts it |
| `RFC7012-4-1` | The values of these identifiers are in the range of 1-32767. Within this range, Information Element identifier values in the sub-range of 1-127 are compatible with field types used by NetFlow version 9 [RFC3954] for historical reasons. In general, IANA will add newly registered Information Elements to the registry, assigning the lowest available Information Element identifier in the range of 128-32767. Enterprise-specific Information Element identifiers have the same range of 1-32767, but they are coupled with an additional enterprise identifier. For enterprise-specific Information Elements, Information Element identifier 0 is also reserved. (Section 4) | MUST NOT | 4 - Information Element Identifiers | **positive:** `unit/verify` [`TestIPFIXFlowTemplate`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/ipfix/rfc7012_flow_template_test.go#L9). **positive:** `unit/verify` [`TestRFC7012EmittedIEIdentifiersInRange`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/ipfix/integral_rfc7011_test.go#L104). **negative:** no negative test. **{single-polarity}:** ze's static templates reference only non-zero IANA IE IDs and it has no code path that constructs IE identifier 0 to drive a negative test |
| `RFC7012-4-2` | Enterprise-specific Information Element identifiers have the same range of 1-32767, but they are coupled with an additional enterprise identifier. (Section 4) | MUST | 4 - Information Element Identifiers | **positive:** no positive test. **negative:** no negative test. **{gap}:** gap in proof: internal/plugins/flowexport/ipfix/flow_template.go::BuildFlowTemplate writes no enterprise-specific identifier, so none falls outside 1-32767 or lacks its enterprise number; no test asserts it |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC7012-2.1-1`](#rfc7012-2.1-1) All Information Elements specified for the IPFIX protocol MUST have the following properties defined: name - A unique and meaningful name for the Information Element. elementId - A numeric identifier of the Information Element. If this identifier is used without an enterprise identifier (see [RFC7011] and the definition of enterpriseId listed below), then it is globally unique, and the list of allowed values is administered by IANA. It is used for compact identification of an Information Element when encoding Templates in the protocol. description - The semantics of this Information Element. Describes how this Information Element is derived from the Flow or other information available to the observer. Information Elements of dataType string or octetArray that have length constraints (fixed length, minimum and/or maximum length) MUST note these constraints in their descriptions. dataType - One of the types listed in Section 3.1 of this document or registered in the IANA "IPFIX Information Element Data Types" subregistry. The type space for attributes is constrained to facilitate implementation. The existing type space encompasses most primitive types used in modern programming languages, as well as some derived types (such as ipv4Address) that are common to this domain. status - The status of the specification of this Information Element. Allowed values are 'current' and 'deprecated'. (Section 2.1) | {gap}, no test | the sentence binds whoever specifies an Information Element, the IANA registry and the documents that define IEs, a role ze never fills, and its subject carries a qualifying phrase that fits neither not-applicable shape; ze defines no Information Element and exports only IANA-registered ones (internal/plugins/flowexport/ipfix/ie.go), so it has nothing of its own to test |
| [`RFC7012-2.1-2`](#rfc7012-2.1-2) Enterprise-specific Information Elements MUST have the following property defined: enterpriseId (Section 2.1) | {gap}, no test | gap in proof: internal/plugins/flowexport/ipfix/flow_template.go::BuildFlowTemplate meets it by writing only IANA field specifiers with the Enterprise bit clear, so no enterprise-specific Information Element lacks an enterpriseId; no test asserts it |
| [`RFC7012-2.1-3`](#rfc7012-2.1-3) If specifications of enterprise-specific Information Elements are made public and/or if enterprise-specific identifiers are used by the IPFIX protocol outside the enterprise, then the enterprise- specific identifier MUST be made globally unique by combining it with an enterprise identifier. (Section 2.1) | {gap}, no test | gap in proof: internal/plugins/flowexport/ipfix/flow_template.go::BuildFlowTemplate meets it by writing only IANA field specifiers with the Enterprise bit clear, so ze uses no enterprise-specific identifier that would need an enterprise number; no test asserts it |
| [`RFC7012-2.1-5`](#rfc7012-2.1-5) values for this Information Element outside the range are invalid and MUST NOT be exported. (Section 2.1) | {gap}, no test | gap in proof: none of the Information Elements the exporter writes (internal/plugins/flowexport/ipfix/ie.go: elements 1, 2, 4, 7, 8, 10, 11, 12, 14, 16, 17, 27, 28, 85, 86 and 150 to 153) has a Range in the IANA IPFIX registry, so no out-of-range value can be exported; no test asserts it |
| [`RFC7012-4-2`](#rfc7012-4-2) Enterprise-specific Information Element identifiers have the same range of 1-32767, but they are coupled with an additional enterprise identifier. (Section 4) | {gap}, no test | gap in proof: internal/plugins/flowexport/ipfix/flow_template.go::BuildFlowTemplate writes no enterprise-specific identifier, so none falls outside 1-32767 or lacks its enterprise number; no test asserts it |

## Proof state

A test reads unproven when no record shows it failing after a deliberate fault. Until then, its claim to check the requirement remains unproven.

### [`RFC7012-2.1-1`](#rfc7012-2.1-1)

All Information Elements specified for the IPFIX protocol MUST have the following properties defined: name - A unique and meaningful name for the Information Element. elementId - A numeric identifier of the Information Element. If this identifier is used without an enterprise identifier (see [RFC7011] and the definition of enterpriseId listed below), then it is globally unique, and the list of allowed values is administered by IANA. It is used for compact identification of an Information Element when encoding Templates in the protocol. description - The semantics of this Information Element. Describes how this Information Element is derived from the Flow or other information available to the observer. Information Elements of dataType string or octetArray that have length constraints (fixed length, minimum and/or maximum length) MUST note these constraints in their descriptions. dataType - One of the types listed in Section 3.1 of this document or registered in the IANA "IPFIX Information Element Data Types" subregistry. The type space for attributes is constrained to facilitate implementation. The existing type space encompasses most primitive types used in modern programming languages, as well as some derived types (such as ipv4Address) that are common to this domain. status - The status of the specification of this Information Element. Allowed values are 'current' and 'deprecated'. (Section 2.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC7012-2.1-1, so there is no linked test.

### [`RFC7012-2.1-2`](#rfc7012-2.1-2)

Enterprise-specific Information Elements MUST have the following property defined: enterpriseId (Section 2.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC7012-2.1-2, so there is no linked test.

### [`RFC7012-2.1-3`](#rfc7012-2.1-3)

If specifications of enterprise-specific Information Elements are made public and/or if enterprise-specific identifiers are used by the IPFIX protocol outside the enterprise, then the enterprise- specific identifier MUST be made globally unique by combining it with an enterprise identifier. (Section 2.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC7012-2.1-3, so there is no linked test.

### [`RFC7012-2.1-5`](#rfc7012-2.1-5)

values for this Information Element outside the range are invalid and MUST NOT be exported. (Section 2.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC7012-2.1-5, so there is no linked test.

### [`RFC7012-4-1`](#rfc7012-4-1)

The values of these identifiers are in the range of 1-32767. Within this range, Information Element identifier values in the sub-range of 1-127 are compatible with field types used by NetFlow version 9 [RFC3954] for historical reasons. In general, IANA will add newly registered Information Elements to the registry, assigning the lowest available Information Element identifier in the range of 128-32767. Enterprise-specific Information Element identifiers have the same range of 1-32767, but they are coupled with an additional enterprise identifier. For enterprise-specific Information Elements, Information Element identifier 0 is also reserved. (Section 4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Single-polarity row. TestRFC7012EmittedIEIdentifiersInRange walks every field specifier of all three emitted templates (counter, IPv4 flow, IPv6 flow) and requires the IE identifier word to be neither 0 nor above 32767, so a template referencing IE 0 in any field goes red (recorded).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC7012EmittedIEIdentifiersInRange`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/ipfix/integral_rfc7011_test.go#L104) | unit/verify | revert, verified |
| positive | [`TestIPFIXFlowTemplate`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/ipfix/rfc7012_flow_template_test.go#L9) | unit/verify | revert, verified |

### [`RFC7012-4-2`](#rfc7012-4-2)

Enterprise-specific Information Element identifiers have the same range of 1-32767, but they are coupled with an additional enterprise identifier. (Section 4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC7012-4-2, so there is no linked test.

## Requirement list review

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | rfc2119 |
| Source | rfc/full/rfc7012.txt |
| Source fingerprint | 484167319fbb3a09 |
| Record | rfc/extraction/rfc7012.json |
| Mapped sentences | 4 |
| Declined as scope | 8 |
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
| `2.1` | Information Element properties | 5 | reviewed | Information Element properties. Five MUST-level sites. Three map to declared ids: the required property set (2.1-1), the enterpriseId property of an enterprise-specific IE (2.1-2), and the globally unique enterprise-specific identifier (2.1-3). One binds the IE definition's description text and is excluded. One binds the EXPORTER and the checklist did not carry it, so the 2026-09-21 walk added RFC7012-2.1-5 for it: "values for this Information Element outside the range are invalid and MUST NOT be exported". The SHOULD in the same sentence, that a valid inclusive range be specified, binds the IE definition and carries no row. |
| `2.2` | not stated | 0 | reviewed | not stated |
| `2.3` | Naming Conventions for Information Elements | 3 | reviewed | Naming Conventions for Information Elements. Three MUST-level sites, all excluded as binding the author of a registry entry. Its two SHOULDs, that names be descriptive and that enterprise-specific names be prefixed with a vendor name, bind the same role and carry no row. |
| `3` | not stated | 0 | reviewed | not stated |
| `3.1` | not stated | 0 | reviewed | not stated |
| `3.1.1` | not stated | 0 | reviewed | not stated |
| `3.1.2` | not stated | 0 | reviewed | not stated |
| `3.1.3` | not stated | 0 | reviewed | not stated |
| `3.1.4` | not stated | 0 | reviewed | not stated |
| `3.1.5` | not stated | 0 | reviewed | not stated |
| `3.1.6` | not stated | 0 | reviewed | not stated |
| `3.1.7` | not stated | 0 | reviewed | not stated |
| `3.1.8` | not stated | 0 | reviewed | not stated |
| `3.1.9` | not stated | 0 | reviewed | not stated |
| `3.1.10` | not stated | 0 | reviewed | not stated |
| `3.1.11` | not stated | 0 | reviewed | not stated |
| `3.1.12` | not stated | 0 | reviewed | not stated |
| `3.1.13` | not stated | 0 | reviewed | not stated |
| `3.1.14` | not stated | 0 | reviewed | not stated |
| `3.1.15` | not stated | 0 | reviewed | not stated |
| `3.1.16` | not stated | 0 | reviewed | not stated |
| `3.1.17` | not stated | 0 | reviewed | not stated |
| `3.1.18` | not stated | 0 | reviewed | not stated |
| `3.1.19` | not stated | 0 | reviewed | not stated |
| `3.1.20` | not stated | 0 | reviewed | not stated |
| `3.1.21` | not stated | 0 | reviewed | not stated |
| `3.1.22` | not stated | 0 | reviewed | not stated |
| `3.1.23` | not stated | 0 | reviewed | not stated |
| `3.2` | not stated | 0 | reviewed | not stated |
| `3.2.1` | not stated | 0 | reviewed | not stated |
| `3.2.2` | not stated | 0 | reviewed | not stated |
| `3.2.3` | not stated | 0 | reviewed | not stated |
| `3.2.4` | identifier semantics | 1 | reviewed | identifier semantics. One site, excluded as binding the dataType chosen for a registry entry. |
| `3.2.5` | flags semantics | 1 | reviewed | flags semantics. One site, excluded as binding the dataType chosen for a registry entry. |
| `4` | Information Element Identifiers | 0 | reviewed | Information Element Identifiers. No 2119 keyword: the section states in the indicative that identifier values are in the range 1-32767, that identifier 0 is also reserved for enterprise-specific Information Elements, and that enterprise-specific identifiers share that range coupled with an enterprise identifier. Those are RFC7012-4-1 and RFC7012-4-2, declared unsourced here. |
| `5` | Information Elements | 0 | reviewed | Information Elements. No obligation: it records that [IANA-IPFIX] is now the normative reference and lists the RFC 5102 categories as a historical note. |
| `6` | IANA Considerations | 1 | reviewed | IANA Considerations. One site, excluded as binding IANA's allocation policy and the Standards Track document that would define a new abstract data type or semantic. |
| `7` | not stated | 0 | reviewed | not stated |
| `7.1` | not stated | 0 | reviewed | not stated |
| `7.2` | mplsTopLabelType subregistry | 1 | reviewed | mplsTopLabelType subregistry. One site, excluded as binding whoever specifies a new MPLS label type. |
| `7.3` | not stated | 0 | reviewed | not stated |
| `7.4` | not stated | 0 | reviewed | not stated |
| `8` | Security Considerations | 0 | reviewed | Security Considerations. No MUST-level site. Its two directives bind the Collector, not the Exporter role ze fills: Collectors MAY take advantage of the machine-readability of the information model, and Collectors SHOULD NOT poll the IANA registry directly at runtime. Neither carries a checklist row. |
| `9` | not stated | 0 | reviewed | not stated |
| `10` | not stated | 0 | reviewed | not stated |
| `10.1` | not stated | 0 | reviewed | not stated |
| `10.2` | not stated | 0 | reviewed | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `2.1:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | The obligation is on the DESCRIPTION written for an Information Element: a string or octetArray IE with length constraints must note them in its description. The description is a registry field, written when the IE is defined, and it never travels on the IPFIX wire. Producer: the author of an Information Element definition in the IANA IPFIX Information Elements registry; ze holds no code for this role. Ze consumes the registry instead: internal/plugins/flowexport/ipfix/ie.go names registered IEs by numeric elementId and defines no registry entry, and internal/plugins/flowexport/ipfix/flow_template.go builds every field specifier from those numbers. | Information Elements of dataType string or octetArray that have length constraints (fixed length, minimum and/or maximum length) MUST note these constraints in their descriptions. |
| `2.3:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Names of Information Elements must be unique within the IANA registry. Uniqueness is a property of the registry, enforced when an entry is added to it. Producer: the author of an Information Element definition in the IANA IPFIX Information Elements registry; ze holds no code for this role. Ze consumes the registry instead: internal/plugins/flowexport/ipfix/ie.go names registered IEs by numeric elementId and defines no registry entry, and internal/plugins/flowexport/ipfix/flow_template.go builds every field specifier from those numbers. | o Names of Information Elements MUST be unique within the "IPFIX Information Elements" registry [IANA-IPFIX]. |
| `2.3:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Names of Information Elements must start with lowercase letters. This is a naming convention binding whoever writes the name into the registry. Producer: the author of an Information Element definition in the IANA IPFIX Information Elements registry; ze holds no code for this role. Ze consumes the registry instead: internal/plugins/flowexport/ipfix/ie.go names registered IEs by numeric elementId and defines no registry entry, and internal/plugins/flowexport/ipfix/flow_template.go builds every field specifier from those numbers. | o Names of Information Elements MUST start with lowercase letters. |
| `2.3:3` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Composed names must capitalise the first letter of each component after the first. This is a naming convention binding whoever writes the name into the registry. Producer: the author of an Information Element definition in the IANA IPFIX Information Elements registry; ze holds no code for this role. Ze consumes the registry instead: internal/plugins/flowexport/ipfix/ie.go names registered IEs by numeric elementId and defines no registry entry, and internal/plugins/flowexport/ipfix/flow_template.go builds every field specifier from those numbers. | o Composed names MUST use capital letters for the first letter of each component (except for the first one). |
| `3.2.4:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | An Information Element with 'identifier' data type semantics must be declared with a signed or unsigned data type. The constraint is on the dataType field of the registry entry, chosen when the IE is defined. Producer: the author of an Information Element definition in the IANA IPFIX Information Elements registry; ze holds no code for this role. Ze consumes the registry instead: internal/plugins/flowexport/ipfix/ie.go names registered IEs by numeric elementId and defines no registry entry, and internal/plugins/flowexport/ipfix/flow_template.go builds every field specifier from those numbers. | Identifiers MUST be one of the signed or unsigned data types. |
| `3.2.5:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | An Information Element with 'flags' data type semantics must be declared with an unsigned data type. The constraint is on the dataType field of the registry entry, chosen when the IE is defined. Producer: the author of an Information Element definition in the IANA IPFIX Information Elements registry; ze holds no code for this role. Ze consumes the registry instead: internal/plugins/flowexport/ipfix/ie.go names registered IEs by numeric elementId and defines no registry entry, and internal/plugins/flowexport/ipfix/flow_template.go builds every field specifier from those numbers. | Flags MUST always be of an unsigned data type. |
| `6:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | "New abstract data types and semantics are subject to Standards Action [RFC5226] and MUST be defined in IETF Standards Track documents updating this document." The obligation is on the IANA allocation policy and on the document that defines a new type. Producer: IANA and the author of an IETF Standards Track document that extends this registry; ze holds no code for this role. Ze consumes the registry instead: internal/plugins/flowexport/ipfix/ie.go names registered IEs by numeric elementId and defines no registry entry, and internal/plugins/flowexport/ipfix/flow_template.go builds every field specifier from those numbers. | New abstract data types and semantics are subject to Standards Action [RFC5226] and MUST be defined in IETF Standards Track documents updating this document. |
| `7.2:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | "The specification of new MPLS label types MUST be published using a well-established and persistent publication medium." The obligation is on whoever specifies a new MPLS label type for the IPFIX mplsTopLabelType subregistry. Producer: IANA and the author of an IETF Standards Track document that extends this registry; ze holds no code for this role. Ze consumes the registry instead: internal/plugins/flowexport/ipfix/ie.go names registered IEs by numeric elementId and defines no registry entry, and internal/plugins/flowexport/ipfix/flow_template.go builds every field specifier from those numbers. | The specification of new MPLS label types MUST be published using a well-established and persistent publication medium. |

## Superseded

No document obsoletes RFC 7012, so its obligations are stated where they were written.
