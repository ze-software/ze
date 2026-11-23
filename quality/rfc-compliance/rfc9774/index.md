# RFC 9774 - Deprecation of AS_SET and AS_CONFED_SET in BGP

Deprecation of AS_SET and AS_CONFED_SET. Every requirement listed for RFC 9774, the tests linked to it, and their recorded reviews. This summary is included in the automatic compliance check, ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 3 | of 3 mandatory requirements | a positive test checks that Ze does what the requirement demands and a negative one checks that it refuses what the requirement forbids |
| One polarity plus reason | 0 | of 3 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason supplies the complete test coverage required |
| One polarity, unexcused | 0 | of 3 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial test coverage | 0 | of 3 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unverified, with zero whole-requirement credit |
| Declared gap or no test | 0 | of 3 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |
| Tests shown to detect a fault | 12 | 12 of 12 tests linked to requirements | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests verifies a requirement only after both tests have been shown to detect a fault |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 3 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 0 | of 3 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 0 | of 3 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze verified: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 3 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 3 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no test verification credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level) | 3 | of 3 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements included in the automatic checks: the compliance check (./le rfc check) holds Ze to each of them. This is the total being assessed; the counts above show the recorded status of those requirements. |

The 9 counts marked as a part above are the whole of the 3 mandatory requirements: they add up to 3. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

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
| Public row | Deprecation of AS_SET and AS_CONFED_SET |
| Automatic checks | Included |
| Requirements | 3 |
| Mandatory requirements (MUST level) | 3 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 12 |
| Tests linked to requirements | 12 |
| Recorded audit verdicts | 2 |
| Fault test records | 12 |
| Summary | `rfc/short/rfc9774.md` |
| Requirement table | `rfc/requirements/rfc9774.md` |
| RFC text | `rfc/full/rfc9774.txt` |

## Automatic checks

Included in automatic checks: Owner decision 2026-10-18, "Implement RFC 9774", lifting the 2026-10-08 enrolment freeze for this RFC only. Two MUST-level requirements, both in Section 3, both met and proven in both polarities. RFC9774-3-2 (treat-as-withdraw on receipt of an AS_SET or AS_CONFED_SET in AS_PATH or AS4_PATH) is enforced by validateASPathAttr (internal/component/bgp/message/rfc7606.go) and validateAS4PathAttr (internal/component/bgp/message/rfc7607.go), which share ASPathSetSegment (internal/component/bgp/message/rfc9774.go). RFC9774-3-1 (never advertise an UPDATE containing either set) is enforced by Session.transmittedPathHoldsSet and Session.writeUpdateAsWithdrawal (internal/component/bgp/reactor/session_rfc9774.go), called from Session.writeUpdateBody, the one UPDATE writer. The two MUST-level sites of Sections 5.2 and 6.1 bind route aggregation, which Ze does not implement, and are excluded as feature-out-of-scope in rfc/extraction/rfc9774.json. Enrolled 2026-10-19.

## Published support status

**Area:** Deprecation of AS_SET and AS_CONFED_SET

**Test verification:** 3 of 3 MUST proven

**Evidence:** unit 3

**Unmet:** -


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 3 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 0 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 0 | one part of the mandatory requirements |
| Partial test coverage (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level)** | **3** | every mandatory requirement appears in exactly one group above |

**Positive and negative tests (3):** [`RFC9774-3-1`](#rfc9774-3-1), [`RFC9774-3-2`](#rfc9774-3-2), [`RFC9774-5.2-1`](#rfc9774-5.2-1)

## Figures

- **MUST bar**: Ze verified 3
- **Overview bar**: Ze MUST verified 3

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | complete |
| MUST rows | 3 |
| MUST rows kept (the MUST bar) | 3 |
| Ze verified | 3 |
| OS verified | 0 |
| Tests awaiting verification | 0 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 0 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 0 |
| SHOULD verified | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD tests awaiting verification | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 0 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze verified means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Tests awaiting verification counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

Source: [data/rfc/rfc9774.json](../../../data/rfc/rfc9774.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC9774-3-1` | MUST NOT advertise BGP UPDATE messages containing AS_SETs or AS_CONFED_SETs (§3) | MUST NOT | 3 | **positive:** `unit/verify` [`TestRFC9774SendASSetWrittenAsWithdrawal`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9774_send_test.go#L49). **negative:** `unit/verify` [`TestRFC9774SendASSetWrittenAsWithdrawal`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9774_send_test.go#L52) |
| `RFC9774-3-2` | MUST use the "treat-as-withdraw" error handling behavior per [RFC7606] upon reception of BGP UPDATE messages containing AS_SETs or AS_CONFED_SETs in the AS_PATH or AS4_PATH [RFC6793] (§3) | MUST | 3 | **positive:** `unit/verify` [`TestRFC9774ReceiveAS4PathSetTreatAsWithdraw`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9774_receive_test.go#L131). **positive:** `unit/verify` [`TestRFC9774ReceiveASSetAcceptedByOperator`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9774_accept_test.go#L45). **positive:** `unit/verify` [`TestRFC9774ReceiveASSetTreatAsWithdraw`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9774_receive_test.go#L64). **positive:** `unit/verify` [`TestValidateAS4PathRejectsSetSegments`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc9774_test.go#L121). **positive:** `unit/verify` [`TestValidateASPathRejectsSetSegments`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc9774_test.go#L35). **negative:** `unit/verify` [`TestRFC9774ReceiveAS4PathSetTreatAsWithdraw`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9774_receive_test.go#L134). **negative:** `unit/verify` [`TestRFC9774ReceiveASSetTreatAsWithdraw`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9774_receive_test.go#L67). **negative:** `unit/verify` [`TestValidateASPathRejectsSetSegments`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc9774_test.go#L38) |
| `RFC9774-5.2-1` | To ensure a consistent BGP origin AS is announced for aggregate BGP routes for implementations of "brief" BGP aggregation, the implementation MUST be configured to truncate the AS_PATH after the right-most instance of the desired origin AS for the aggregate. (§5.2) | MUST | 5.2 - Consistent brief aggregation | **positive:** `unit/verify` [`TestRFC9774ConsistentBriefAggregationTruncatesAfterRightMostOriginAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_aggregate_test.go#L598). **negative:** `unit/verify` [`TestRFC9774ConsistentBriefAggregationTruncatesAfterRightMostOriginAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_aggregate_test.go#L599) |

## Gaps and untested MUSTs

RFC 9774 lists no missing behavior, and every mandatory requirement included in automatic checks has a linked test.

## Test status

A test reads not verified when no record shows it failing after a deliberate fault. Until then, the test has not been shown to detect a fault in the required behavior.

### [`RFC9774-3-1`](#rfc9774-3-1)

MUST NOT advertise BGP UPDATE messages containing AS_SETs or AS_CONFED_SETs (§3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC9774SendASSetWrittenAsWithdrawal`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9774_send_test.go#L52) | unit/verify | revert, verified |
| positive | [`TestRFC9774SendASSetWrittenAsWithdrawal`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9774_send_test.go#L49) | unit/verify | revert, verified |

### [`RFC9774-3-2`](#rfc9774-3-2)

MUST use the "treat-as-withdraw" error handling behavior per [RFC7606] upon reception of BGP UPDATE messages containing AS_SETs or AS_CONFED_SETs in the AS_PATH or AS4_PATH [RFC6793] (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-24 (independent auditor, authored none of 9c3697d0d1 or 439a413ffd). RFC 9774 Section 3: "Unless explicitly configured by a network operator to do otherwise (e.g., during a transition phase), BGP speakers:" ... "MUST use the "treat-as-withdraw" error handling behavior per [RFC7606] upon reception of BGP UPDATE messages containing AS_SETs or AS_CONFED_SETs in the AS_PATH or AS4_PATH [RFC6793]." The weak judgement of 926410a332 (AS4_PATH clause proven at positive polarity only, AS_SET only, 2-octet session only) is answered by new tags, not new code: TestRFC9774ReceiveAS4PathSetTreatAsWithdraw now carries positive and negative. Re-read its body: an UPDATE with a sequence-only AS_PATH and an AS4_PATH holding AS_SET or AS_CONFED_SET draws exact TreatAsWithdraw from Session.enforceRFC7606 with no error on a 2-octet session (rfc7607Session) and on a 4-octet eBGP session (firstASSession with ASN4), and the twin with a sequence segment in the set's place draws exact None on both, so the verdict is bound to the segment type alone and an over-withdraw of a clean AS4_PATH goes red. TestValidateAS4PathRejectsSetSegments (new positive) asserts for asn4 false and true exact TreatAsWithdraw, AttrCode 17 and the exact RFC 9774 Section 3 description for AS_SET, AS_CONFED_SET and a set beside AS 0, and its controls assert discard for AS 0 alone, nil for a clean path and nil from the set scan for a misframed path. The AS_PATH clause stays enforced by TestValidateASPathRejectsSetSegments and TestRFC9774ReceiveASSetTreatAsWithdraw (unchanged); TestRFC9774ReceiveASSetAcceptedByOperator proves the default through parsePeerFromTree and its false comment (AS4_PATH reaches its validator only on a 2-octet session) is corrected. Both earlier findings closed. Each claim states what its body checks. Discrimination records observed red: positive on both AS4_PATH units under rfc7607.go !acceptASSet forced false, negative under the rfc9774.go segment-type comparison inverted. The operator exception is an uncounted deviation recorded in rfc/short/rfc9774.md. Units ran green at HEAD.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestValidateASPathRejectsSetSegments`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc9774_test.go#L38) | unit/verify | revert, verified |
| negative | [`TestRFC9774ReceiveAS4PathSetTreatAsWithdraw`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9774_receive_test.go#L134) | unit/verify | mutant, verified |
| negative | [`TestRFC9774ReceiveASSetTreatAsWithdraw`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9774_receive_test.go#L67) | unit/verify | revert, verified |
| positive | [`TestValidateAS4PathRejectsSetSegments`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc9774_test.go#L121) | unit/verify | mutant, verified |
| positive | [`TestValidateASPathRejectsSetSegments`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc9774_test.go#L35) | unit/verify | revert, verified |
| positive | [`TestRFC9774ReceiveASSetAcceptedByOperator`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9774_accept_test.go#L45) | unit/verify | revert, verified |
| positive | [`TestRFC9774ReceiveAS4PathSetTreatAsWithdraw`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9774_receive_test.go#L131) | unit/verify | mutant, verified |
| positive | [`TestRFC9774ReceiveASSetTreatAsWithdraw`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9774_receive_test.go#L64) | unit/verify | revert, verified |

### [`RFC9774-5.2-1`](#rfc9774-5.2-1)

To ensure a consistent BGP origin AS is announced for aggregate BGP routes for implementations of "brief" BGP aggregation, the implementation MUST be configured to truncate the AS_PATH after the right-most instance of the desired origin AS for the aggregate. (§5.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged against RFC 9774 Section 5.2: "To ensure a consistent BGP origin AS is announced for aggregate BGP routes for implementations of \"brief\" BGP aggregation, the implementation MUST be configured to truncate the AS_PATH after the right-most instance of the desired origin AS for the aggregate." aggregateBriefASPath finds the right-most instance of rule.BriefOriginAS in the common leading AS_SEQUENCE (truncating all subsequent ASes and marking excluded for ATOMIC_AGGREGATE), supports speaker.LocalAS by emitting an empty AS_PATH that eBGP announcement prepends with the local AS, and refuses synthesis when the desired origin AS is absent. parseASPath4ForAggregate stops immediately on any non-AS_SEQUENCE segment. TestRFC9774ConsistentBriefAggregationTruncatesAfterRightMostOriginAS proves: without brief-origin-as, shared trailing 65030 is kept ([65010,65020,65010,65030]); with brief-origin-as=65010, wire AS_PATH on BestChange (fully decoded as AS_SEQUENCE only) and dispatched update-text command truncate after right-most 65010 to [65010,65020,65010] with length-0 ATOMIC_AGGREGATE and stay stable when p2 withdraws (Appendix B); with brief-origin-as=65000 (local AS), wire AS_PATH is empty with ATOMIC_AGGREGATE (paired with reactor eBGP local-AS prepend verification); and absent origin AS (65999) or origin AS behind AS_SET (65020) suppresses aggregation. Both positive and negative discrimination records in rfc/discrimination/rfc9774/RFC9774-5.2-1.json are current (producer-disable revert records).

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC9774ConsistentBriefAggregationTruncatesAfterRightMostOriginAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_aggregate_test.go#L599) | unit/verify | revert, verified |
| positive | [`TestRFC9774ConsistentBriefAggregationTruncatesAfterRightMostOriginAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc4271_aggregate_test.go#L598) | unit/verify | revert, verified |

## Requirement list review

| Field | Value |
|---|---|
| Reviewer | ze-implement agent, spec-bgp-rfc9774-as-set-deprecation |
| Signed off | 2026-10-19 |
| Register | rfc2119 |
| Source | rfc/full/rfc9774.txt |
| Source fingerprint | 1c7ab2a8ddf3a013 |
| Record | rfc/extraction/rfc9774.json |
| Mapped sentences | 3 |
| Declined as scope | 1 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | skipped (front-matter) | Title block, Status of This Memo, copyright notice, Abstract and Table of Contents. The Abstract states scope and binds nobody. |
| `1` | Introduction | 0 | reviewed | Introduction. Explains why AS_SET and AS_CONFED_SET complicate BGP security and states the document updates RFC 4271 and RFC 5065 and obsoletes RFC 6472. No RFC 2119 keyword; the obligations are stated in section 3. |
| `2` | Requirements Language boilerplate citing BCP 14 | 0 | reviewed | Requirements Language boilerplate citing BCP 14. It defines keywords and obliges nobody. |
| `3` | not stated | 2 | reviewed | The two obligations of the document, both mapped: 3:1 to RFC9774-3-1 (send) and 3:2 to RFC9774-3-2 (receive). The closing paragraph records which sections of RFC 4271 and RFC 5065 are updated and binds nothing further. |
| `4` | not stated | 0 | reviewed | Describes how RPKI-ROV, BGPsec and ASPA already treat a route with an AS_SET. Descriptive, no RFC 2119 keyword, and it obliges no BGP speaker beyond the documents it cites. |
| `5` | Describes brief aggregation | 0 | reviewed | Describes brief aggregation. Its one SHOULD is a quotation of RFC 4271 Section 5.1.6 and belongs to RFC 4271; the section adds no obligation of its own. |
| `5.1` | not stated | 0 | reviewed | Explains why brief aggregation yields an unpredictable origin AS for RPKI-ROV. Descriptive, no RFC 2119 keyword. |
| `5.2` | Consistent brief aggregation | 1 | reviewed | Consistent brief aggregation. Its MUST (5.2:1) maps to RFC9774-5.2-1, implemented in internal/component/bgp/plugins/rib/aggregate.go (aggregateBriefASPath, brief-origin-as in ze-bgp-conf.yang). |
| `6` | not stated | 0 | reviewed | Section preamble: operator advice on deployment and configuration. No keyword. |
| `6.1` | not stated | 1 | reviewed | Its MUST (6.1:1) binds network operators ('When aggregating prefixes, network operators MUST use consistent brief aggregation'), not the BGP speaker software. |
| `6.2` | not stated | 0 | reviewed | SHOULD NOT and SHOULD on announcing an aggregate to contributing ASes; recommendations on operator export policy. No MUST-level site. |
| `6.3` | not stated | 0 | reviewed | Reminds that RFC 4632 Section 5.1 Rule 2 binds aggregating implementations; it restates another document and adds no keyword. |
| `7` | Security Considerations | 0 | reviewed | Security Considerations. Notes the deprecation shrinks the attack surface. No keyword. |
| `8` | not stated | 0 | skipped (iana) | IANA Considerations: marks segment types 1 and 4 deprecated in the registry. A registry action, no speaker obligation. |
| `9` | References heading | 0 | skipped (references) | References heading. |
| `9.1` | Normative references | 0 | skipped (references) | Normative references. |
| `9.2` | Informative references | 0 | skipped (references) | Informative references. |
| `A` | not stated | 0 | skipped (appendix-non-normative) | Example filtering policy for not advertising an aggregate to contributing ASes. Illustrative, no keyword. |
| `B` | not stated | 0 | skipped (appendix-non-normative) | Examples of consistent brief aggregation and the ROAs they need. Illustrative, no keyword. |
| `B.1` | Worked example of Appendix B | 0 | skipped (appendix-non-normative) | Worked example of Appendix B. Illustrative. |
| `B.2` | Worked example of Appendix B | 0 | skipped (appendix-non-normative) | Worked example of Appendix B. Illustrative. |
| `B.3` | Worked example of Appendix B | 0 | skipped (appendix-non-normative) | Worked example of Appendix B. Illustrative. |
| `B.4` | Worked example of Appendix B | 0 | skipped (appendix-non-normative) | Worked example of Appendix B. Illustrative. |
| `C` | Discussion of forwarding loops and AS_SETs | 0 | skipped (appendix-non-normative) | Discussion of forwarding loops and AS_SETs. Explanatory, no keyword. |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `6.1:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds network operators configuring aggregation policy rather than BGP speaker protocol behavior; the speaker mechanism it relies on is RFC9774-5.2-1. | When aggregating prefixes, network operators MUST use consistent brief aggregation as described in Section 5.2. |

## Superseded

No document obsoletes RFC 9774, so its obligations are stated where they were written.
