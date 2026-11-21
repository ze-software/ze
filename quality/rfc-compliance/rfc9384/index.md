# RFC 9384 - A BGP Cease NOTIFICATION Subcode for Bidirectional Forwarding Detection (BFD)

BFD-triggered BGP Cease. Every requirement listed for RFC 9384, the tests linked to it, and their recorded reviews. This summary is excluded from automatic compliance checks.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0 | of 0 mandatory requirements | a positive test checks that Ze does what the requirement demands and a negative one checks that it refuses what the requirement forbids |
| One polarity plus reason | 0 | of 0 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason supplies the complete test coverage required |
| One polarity, unexcused | 0 | of 0 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial test coverage | 0 | of 0 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unverified, with zero whole-requirement credit |
| Declared gap or no test | 0 | of 0 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |
| Tests shown to detect a fault | 4 | 4 of 4 tests linked to requirements | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests verifies a requirement only after both tests have been shown to detect a fault |
| Audit verdicts | 0 | of 0 mandatory requirements reviewed | 0 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 0 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 0 | of 0 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 0 | of 0 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze verified: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 0 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 0 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no test verification credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level), not checked | 0 | of 3 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements listed in this summary. This RFC is excluded from automatic checks (non-normative), so these requirements are not checked. The counts above show what the summary records. |

No card above is a part of a population, so there is nothing to add up.

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
| Audit verdicts | ok | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a mandatory requirement is unreviewed, green when every one is judged sound and current |
| Mandatory requirements (MUST level), not checked | outlined | outlined, with no colored edge: it is the total the other cards are counted against, not a measure, and a larger one is neither good news nor bad |

## At a glance

| Field | Value |
|---|---|
| Public row | BFD-triggered BGP Cease |
| Automatic checks | Excluded (non-normative) |
| Requirements | 3 |
| Mandatory requirements (MUST level), not checked | 0 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 4 |
| Tests linked to requirements | 4 |
| Recorded audit verdicts | 0 |
| Fault test records | 4 |
| Summary | `rfc/short/rfc9384.md` |
| Requirement table | `rfc/requirements/rfc9384.md` |
| RFC text | `rfc/full/rfc9384.txt` |

## Automatic checks

Excluded from automatic checks (non-normative, the document imposes no MUST-level obligation on an implementation, so there is nothing to gate): A BGP Cease NOTIFICATION Subcode for Bidirectional Forwarding Detection (BFD), IETF category Standards Track. The document carries the RFC 2119 / RFC 8174 key-words paragraph at Section 2, and a capitalised MUST / MUST NOT / SHALL / SHALL NOT / REQUIRED scan over rfc/full/rfc9384.txt hits those five words on one line only, line 101, which is the key-words sentence itself. That sentence tells a reader how to read the other sentences and states no obligation of its own. Outside it the text uses no MUST-level keyword anywhere. The summary written 2026-09-01 therefore captures three requirements and gates none: RFC9384-3-1 at Section 3, and RFC9384-4-1 and RFC9384-4-2 at Section 4, all three at SHOULD. Section 5 says the subcode "is purely informational and has no impact on the BGP Finite State Machine beyond that already documented by [RFC4271], Sections 6.6 and 6.7", so the document adds one registry value and three recommendations about using it. A zero-MUST document can reach the public ledger two ways, as this disposition or as a manual-walk extraction sign-off with a register-reason. This disposition is the route taken, because the sign-off at rfc/extraction/rfc9384.json declares the register the source derives, prose, and the second route would need it to declare the weaker manual-walk grade instead. That sign-off bounds the three-row checklist against the source text.

## Published support status

**Area:** BFD-triggered BGP Cease

**Test verification:** no MUST rows

**Evidence:** -

**Unmet:** -


## Coverage

RFC 9384 lists no mandatory requirements for these checks, so nothing is counted here.

## Figures

- **MUST bar**: no rows
- **Overview bar**: SHOULD verified 1, Not assessed 2

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | complete |
| MUST rows | 0 |
| MUST rows kept (the MUST bar) | 0 |
| Ze verified | 0 |
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
| SHOULD / MAY rows | 3 |
| SHOULD verified | 1 |
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

Source: [data/rfc/rfc9384.json](../../../data/rfc/rfc9384.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC9384-3-1` | When a BGP connection is terminated due to a BFD session going into the Down state, the BGP speaker SHOULD send a NOTIFICATION message with the error code "Cease" and the error subcode "BFD Down" (§3) | SHOULD | 3 - BFD Cease NOTIFICATION Subcode | **positive:** no positive test. **negative:** no negative test |
| `RFC9384-4-1` | When there is a total loss of connectivity between two BGP speakers, it may not have been possible for the Cease NOTIFICATION message to have been sent. Even so, BGP speakers SHOULD provide this reason as part of their operational state. (§4) | SHOULD | 4 - Operational Considerations | **positive:** `unit/verify` [`TestBgpSummaryLastErrorSeparatesToldFromCouldNotTell`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/cmd/peer/rfc9384_last_error_unsent_test.go#L31). **positive:** `unit/verify` [`TestNotificationRefusedBySocketStillRecordsTheReason`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9384_peer_last_error_test.go#L59). **negative:** `unit/verify` [`TestBgpSummaryLastErrorSeparatesToldFromCouldNotTell`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/cmd/peer/rfc9384_last_error_unsent_test.go#L35). **negative:** `unit/verify` [`TestNotificationDeliveredIsNotRecordedAsUnsent`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9384_peer_last_error_test.go#L96) |
| `RFC9384-4-2` | When the procedures in [RFC8538] for sending a NOTIFICATION message with a "Cease" code and "Hard Reset" subcode are required, and the BGP connection is being terminated because BFD has gone into the Down state, the "BFD Down" subcode SHOULD be encapsulated in the Hard Reset's data portion of the NOTIFICATION message (§4) | SHOULD | 4 - Operational Considerations | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

RFC 9384 lists no missing behavior, and every mandatory requirement included in automatic checks has a linked test.

## Test status

A test reads not verified when no record shows it failing after a deliberate fault. Until then, the test has not been shown to detect a fault in the required behavior.

### [`RFC9384-4-1`](#rfc9384-4-1)

When there is a total loss of connectivity between two BGP speakers, it may not have been possible for the Cease NOTIFICATION message to have been sent. Even so, BGP speakers SHOULD provide this reason as part of their operational state. (§4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestBgpSummaryLastErrorSeparatesToldFromCouldNotTell`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/cmd/peer/rfc9384_last_error_unsent_test.go#L35) | unit/verify | revert, verified |
| negative | [`TestNotificationDeliveredIsNotRecordedAsUnsent`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9384_peer_last_error_test.go#L96) | unit/verify | revert, verified |
| positive | [`TestBgpSummaryLastErrorSeparatesToldFromCouldNotTell`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/cmd/peer/rfc9384_last_error_unsent_test.go#L31) | unit/verify | revert, verified |
| positive | [`TestNotificationRefusedBySocketStillRecordsTheReason`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9384_peer_last_error_test.go#L59) | unit/verify | revert, verified |

## Requirement list review

| Field | Value |
|---|---|
| Reviewer | rfcgate-6-supported-extraction-signoff |
| Signed off | 2026-09-01 |
| Register | prose |
| Source | rfc/full/rfc9384.txt |
| Source fingerprint | bb7702e0b362aaeb |
| Record | rfc/extraction/rfc9384.json |
| Mapped sentences | 1 |
| Declined as scope | 1 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 1 | skipped (front-matter) | Title block, Abstract, Status of This Memo, Copyright Notice and Table of Contents. The Abstract restates Section 1 and Section 3 and binds nobody on its own. |
| `1` | Introduction | 0 | reviewed | Introduction. It states why a BGP speaker uses BFD for faster failure detection and announces the subcode Section 3 allocates. No obligation of its own. |
| `2` | Requirements Language | 0 | reviewed | Requirements Language. The RFC 2119 / RFC 8174 key-words paragraph, which tells a reader how to read the other sentences. It is the document's only capitalised MUST-level keyword line and the site scan excludes it. |
| `3` | BFD Cease NOTIFICATION Subcode | 0 | reviewed | BFD Cease NOTIFICATION Subcode. Two sentences: the IANA value, and the one SHOULD this document exists for. The site scan reads must, shall and required and cannot see a SHOULD, so the obligation is recorded as unsourced. |
| `4` | Operational Considerations | 1 | reviewed | Operational Considerations. Three paragraphs carrying two SHOULDs. The Hard Reset sentence is the one site the scan sees, on its lowercase 'are required'; the operational-state SHOULD carries no scanned modal and is recorded as unsourced. |
| `5` | Security Considerations | 0 | reviewed | Security Considerations. It states that the subcode is purely informational and has no impact on the BGP Finite State Machine beyond RFC 4271 Sections 6.6 and 6.7. A property of the subcode, not an obligation. |
| `6` | IANA Considerations | 0 | skipped (iana) | IANA Considerations. It records that IANA has assigned the value 10 with the name 'BFD Down'. The value is captured as a constant in the summary. |
| `7` | References, the parent heading of 7.1 and 7.2 | 0 | skipped (references) | References, the parent heading of 7.1 and 7.2. It carries no text of its own. |
| `7.1` | Normative References | 0 | skipped (references) | Normative References. Citation entries for RFC 2119, RFC 4271, RFC 5880, RFC 5882, RFC 8174 and RFC 8538. |
| `7.2` | not stated | 0 | skipped (references) | Informative References, followed by the Acknowledgments and the Author's Address, which the derivation attributes to this section because neither carries a numbered heading. |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `front:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | The IETF Trust Legal Provisions copyright notice, which the site scan reaches on its lowercase 'must'. It binds a person who republishes code from the document, and it states no protocol behavior. The scan strips the RFC 2119 key-words paragraph and does not strip this one. | Code Components extracted from this document must include Revised BSD License text as described in Section 4.e of the Trust Legal Provisions and are provided without warranty as described in the Revised BSD License. |

## Superseded

No document obsoletes RFC 9384, so its obligations are stated where they were written.
