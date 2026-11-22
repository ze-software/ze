# RFC 8203 - BGP Administrative Shutdown Communication

Administrative Shutdown Communication. Every requirement listed for RFC 8203, the tests linked to it, and their recorded reviews. This summary is included in the automatic compliance check, ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 4 | of 5 mandatory requirements | a positive test checks that Ze does what the requirement demands and a negative one checks that it refuses what the requirement forbids |
| One polarity plus reason | 1 | of 5 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason supplies the complete test coverage required |
| One polarity, unexcused | 0 | of 5 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial test coverage | 0 | of 5 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unverified, with zero whole-requirement credit |
| Declared gap or no test | 0 | of 5 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |
| Tests shown to detect a fault | 9 | 9 of 15 tests linked to requirements | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests verifies a requirement only after both tests have been shown to detect a fault |
| Audit verdicts | 5 | of 5 mandatory requirements reviewed | 0 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

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
| Declared gap or no test | ok | green at zero, RED above it: a binding obligation that nothing shows Ze meets is owed, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green counts above are what says how much Ze's own tests verify |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Linux cannot do this | neutral | neutral at every value: an evidenced platform limit is neither test verification nor a scope exemption |
| Tests shown to detect a fault | ok | green at every value: these tests have been shown to detect a deliberate fault. This card counts individual tests separately, so its total cannot be added to the requirement counts above |
| Audit verdicts | ok | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a mandatory requirement is unreviewed, green when every one is judged sound and current |
| Mandatory requirements (MUST level) | outlined | outlined, with no colored edge: it is the total the other cards are counted against, not a measure, and a larger one is neither good news nor bad |

## At a glance

| Field | Value |
|---|---|
| Public row | Administrative Shutdown Communication |
| Automatic checks | Included |
| Requirements | 9 |
| Mandatory requirements (MUST level) | 5 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 15 |
| Tests linked to requirements | 15 |
| Recorded audit verdicts | 5 |
| Fault test records | 9 |
| Summary | `rfc/short/rfc8203.md` |
| Requirement table | `rfc/requirements/rfc8203.md` |
| RFC text | `rfc/full/rfc8203.txt` |

## Automatic checks

Included in automatic checks: Administrative Shutdown Communication: five MUST-level requirements over the Cease NOTIFICATION shutdown message, all bound to the message codec (internal/component/bgp/message/notification.go). RFC8203-2-1 (subcode 2/4) is proven by ShutdownMessage extracting for Admin Shutdown/Reset and returning empty for any other subcode; RFC8203-2-3 (UTF-8) and RFC8203-2-4 (MUST NOT interpret invalid UTF-8) by a valid message round-tripping while an invalid sequence is rejected; RFC8203-6-1 (shortest form) by an overlong 0xC0 0xAF encoding being rejected where the canonical form is accepted (Go utf8.Valid enforces shortest form). RFC8203-2-2 (length 0-128) is {single-polarity: positive}: BuildShutdownData caps the sender at 128, while the receiver deliberately follows RFC 9003 (which obsoletes RFC 8203 and raised the cap to 255), so there is no over-128 rejection on receive to assert.

## Published support status

**Area:** Administrative Shutdown Communication

**Test verification:** 5 of 5 MUST proven

**Evidence:** unit 5

**Unmet:** -


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 4 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 1 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 0 | one part of the mandatory requirements |
| Partial test coverage (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level)** | **5** | every mandatory requirement appears in exactly one group above |

**Positive and negative tests (4):** [`RFC8203-2-1`](#rfc8203-2-1), [`RFC8203-2-3`](#rfc8203-2-3), [`RFC8203-6-1`](#rfc8203-6-1), [`RFC8203-2-4`](#rfc8203-2-4)

**Annotated (including scoped evidence) (1):** [`RFC8203-2-2`](#rfc8203-2-2)

## Figures

- **MUST bar**: Ze verified 5
- **Overview bar**: Ze MUST verified 5, Not assessed 4

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | complete |
| MUST rows | 5 |
| MUST rows kept (the MUST bar) | 5 |
| Ze verified | 5 |
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

Source: [data/rfc/rfc8203.json](../../../data/rfc/rfc8203.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC8203-2-1` | Subcode: the Error Subcode value MUST be one of the following values: 2 ("Administrative Shutdown") or 4 ("Administrative Reset"). (§2) | MUST | 2 - Shutdown Communication | **positive:** `unit/verify` [`TestRFC8203ShutdownCommunicationOnlyUnderSubcode2Or4`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8203_shutdown_send_test.go#L21). **positive:** `unit/verify` [`TestRFC8203Subcode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/notification_test.go#L389). **negative:** `unit/verify` [`TestRFC8203ShutdownCommunicationOnlyUnderSubcode2Or4`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8203_shutdown_send_test.go#L23). **negative:** `unit/verify` [`TestRFC8203Subcode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/notification_test.go#L391) |
| `RFC8203-2-2` | The length value MUST range from 0 to 128 inclusive. (§2) | MUST | 2 - Shutdown Communication | **positive:** `unit/verify` [`TestRFC8203LengthCap`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/notification_test.go#L414). **negative:** no negative test. **{single-polarity}:** the sender enforces the 0-128 range -- BuildShutdownData truncates a longer message to 128 at a UTF-8 boundary (internal/component/bgp/message/notification.go:312) -- but the receiver deliberately follows RFC 9003, which obsoletes RFC 8203 and raised the cap to 255 (ShutdownMessage reads a 1-byte length up to 255, notification.go:268-284), so ze intentionally does not reject a 129-255 length on receive and there is no over-128-rejected behavior to assert |
| `RFC8203-2-3` | Shutdown Communication: to support international characters, the Shutdown Communication field MUST be encoded using UTF-8. (§2) | MUST | 2 - Shutdown Communication | **positive:** `unit/verify` [`TestRFC8203ShutdownCommunicationSentAsShortestFormUTF8`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8203_shutdown_send_test.go#L60). **positive:** `unit/verify` [`TestRFC8203UTF8Valid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/notification_test.go#L426). **negative:** `unit/verify` [`TestRFC8203ShutdownCommunicationSentAsShortestFormUTF8`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8203_shutdown_send_test.go#L62). **negative:** `unit/verify` [`TestRFC8203UTF8Invalid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/notification_test.go#L444) |
| `RFC8203-6-1` | UTF-8 "Shortest Form" encoding is REQUIRED to guard against the technical issues outlined in [UTR36]. (§6) | MUST | 6 - Security Considerations | **positive:** `unit/verify` [`TestRFC8203ShortestFormUTF8`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/notification_test.go#L462). **positive:** `unit/verify` [`TestRFC8203ShutdownCommunicationSentAsShortestFormUTF8`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8203_shutdown_send_test.go#L64). **negative:** `unit/verify` [`TestRFC8203ShortestFormUTF8`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/notification_test.go#L464). **negative:** `unit/verify` [`TestRFC8203ShutdownCommunicationSentAsShortestFormUTF8`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8203_shutdown_send_test.go#L66) |
| `RFC8203-2-4` | A receiving BGP speaker MUST NOT interpret invalid UTF-8 sequences. (§2) | MUST NOT | 2 - Shutdown Communication | **positive:** `unit/verify` [`TestRFC8203UTF8Invalid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/notification_test.go#L446). **negative:** `unit/verify` [`TestRFC8203UTF8Valid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/notification_test.go#L428) |
| `RFC8203-2-5` | Mechanisms concerning the reporting of information contained in the Shutdown Communication are implementation specific but SHOULD include methods such as Syslog [RFC5424]. (§2) | SHOULD | 2 - Shutdown Communication | **positive:** no positive test. **negative:** no negative test |
| `RFC8203-4-1` | If a Shutdown Communication with an invalid Length value, or an invalid UTF-8 sequence is received, a message indicating this event SHOULD be logged for the attention of the operator. (§4) | SHOULD | 4 - Error Handling | **positive:** no positive test. **negative:** no negative test |
| `RFC8203-2-6` | If a BGP speaker decides to terminate its session with a BGP neighbor, and it sends a NOTIFICATION message with the Error Code "Cease" and Error Subcode "Administrative Shutdown" or "Administrative Reset" [RFC4486], it MAY include an UTF-8 encoded string. (§2) | MAY | 2 - Shutdown Communication | **positive:** no positive test. **negative:** no negative test |
| `RFC8203-4-2` | An erroneous or malformed Shutdown Communication itself MAY be logged in a hexdump format. (§4) | MAY | 4 - Error Handling | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

RFC 8203 lists no missing behavior, and every mandatory requirement included in automatic checks has a linked test.

## Test status

A test reads not verified when no record shows it failing after a deliberate fault. Until then, the test has not been shown to detect a fault in the required behavior.

### [`RFC8203-2-1`](#rfc8203-2-1)

Subcode: the Error Subcode value MUST be one of the following values: 2 ("Administrative Shutdown") or 4 ("Administrative Reset"). (§2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Sender side: TestRFC8203ShutdownCommunicationOnlyUnderSubcode2Or4 requires BuildCeaseData to return length+message under subcodes 2 and 4 and nothing under 0,1,3,5..10. BuildCeaseData is the teardown's only Cease-data builder (session_connection.go, behaviour unchanged); other Cease senders (max-prefix, collision, policy) send their own non-shutdown data. Judge break (return the message for every subcode): red.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC8203Subcode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/notification_test.go#L391) | unit/verify | not verified |
| negative | [`TestRFC8203ShutdownCommunicationOnlyUnderSubcode2Or4`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8203_shutdown_send_test.go#L23) | unit/verify | revert, verified |
| positive | [`TestRFC8203Subcode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/notification_test.go#L389) | unit/verify | not verified |
| positive | [`TestRFC8203ShutdownCommunicationOnlyUnderSubcode2Or4`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8203_shutdown_send_test.go#L21) | unit/verify | revert, verified |

### [`RFC8203-2-2`](#rfc8203-2-2)

The length value MUST range from 0 to 128 inclusive. (§2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden behaviour: Ze emitting a Length above 128. TestRFC8203LengthCap feeds a 200-byte message to BuildShutdownData and require.Equal(byte(128), data[0]) goes red if the cap is lost. Receive-side 129-255 acceptance follows RFC 9003 and is covered by the row's {single-polarity: positive} marker.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestRFC8203LengthCap`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/notification_test.go#L414) | unit/verify | revert, verified |

### [`RFC8203-2-3`](#rfc8203-2-3)

Shutdown Communication: to support international characters, the Shutdown Communication field MUST be encoded using UTF-8. (§2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRFC8203ShutdownCommunicationSentAsShortestFormUTF8: valid 2/3/4-octet UTF-8 sent octet-exact; stray octets, overlong, surrogate and a 128-octet cut inside a character each yield a field with a consistent length octet, utf8.Valid and <=128 octets; overlong input sent as 'xy'.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC8203UTF8Invalid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/notification_test.go#L444) | unit/verify | not verified |
| negative | [`TestRFC8203ShutdownCommunicationSentAsShortestFormUTF8`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8203_shutdown_send_test.go#L62) | unit/verify | revert, verified |
| positive | [`TestRFC8203UTF8Valid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/notification_test.go#L426) | unit/verify | not verified |
| positive | [`TestRFC8203ShutdownCommunicationSentAsShortestFormUTF8`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8203_shutdown_send_test.go#L60) | unit/verify | revert, verified |

### [`RFC8203-6-1`](#rfc8203-6-1)

UTF-8 "Shortest Form" encoding is REQUIRED to guard against the technical issues outlined in [UTR36]. (§6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Same unit: U+00E9 shortest form sent unchanged; the overlong 0xC0 0xAF and surrogate 0xED 0xA0 0x80 never reach the field (utf8.Valid refuses both forms, and the overlong case is asserted exact). Receive side held by the HEAD units.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC8203ShortestFormUTF8`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/notification_test.go#L464) | unit/verify | not verified |
| negative | [`TestRFC8203ShutdownCommunicationSentAsShortestFormUTF8`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8203_shutdown_send_test.go#L66) | unit/verify | revert, verified |
| positive | [`TestRFC8203ShortestFormUTF8`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/notification_test.go#L462) | unit/verify | not verified |
| positive | [`TestRFC8203ShutdownCommunicationSentAsShortestFormUTF8`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc8203_shutdown_send_test.go#L64) | unit/verify | revert, verified |

### [`RFC8203-2-4`](#rfc8203-2-4)

A receiving BGP speaker MUST NOT interpret invalid UTF-8 sequences. (§2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden behaviour: the receiver interpreting an invalid UTF-8 sequence. TestRFC8203UTF8Invalid feeds 0xff 0xfe 0xfd and require.Error plus require.Empty(msg) go red if ShutdownMessage returns decoded text. The negative tag TestRFC8203UTF8Valid proves valid multibyte text is still interpreted (require.Equal "日本語").

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC8203UTF8Valid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/notification_test.go#L428) | unit/verify | revert, verified |
| positive | [`TestRFC8203UTF8Invalid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/notification_test.go#L446) | unit/verify | revert, verified |

## Requirement list review

| Field | Value |
|---|---|
| Reviewer | ze-work agent, spec-rfcgate-6, rfc8203 |
| Signed off | 2026-08-31 |
| Register | rfc2119 |
| Source | rfc/full/rfc8203.txt |
| Source fingerprint | f4dedca562743ae1 |
| Record | rfc/extraction/rfc8203.json |
| Mapped sentences | 5 |
| Declined as scope | 0 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | skipped (front-matter) | Title block, Status of This Memo, Copyright Notice, Abstract and Table of Contents. The Abstract restates the Introduction and directs no speaker. |
| `1` | Introduction | 0 | reviewed | Introduction. Indicative prose: correlating a session teardown with an offline notice is troublesome, and this document updates RFC 4486 by specifying a mechanism to carry a short freeform UTF-8 message in a Cease NOTIFICATION. It states what the document does, never what a speaker must do. |
| `1.1` | Requirements Language | 0 | reviewed | Requirements Language. The BCP 14 key-words paragraph, which also states that the key words bind only when they appear in all capitals. It tells a reader how to read the other sections and binds no speaker, which is why the derivation excludes it from the site inventory. |
| `2` | Shutdown Communication | 4 | reviewed | Shutdown Communication. The document's whole normative surface for the wire format: four capitalised MUST-level sites, mapped below to RFC8203-2-1 through RFC8203-2-4. Its remaining directives are the opening MAY (a speaker MAY include a UTF-8 string) and the closing SHOULD (reporting mechanisms SHOULD include methods such as Syslog); both are the unsourced ids below. Three sentences the site scan cannot see are descriptive, not obligations: the Figure 1 field diagram and its Error Code 6, the note that a multibyte Shutdown Communication holds fewer characters than the length value, and "This field is not NUL terminated", which describes the field rather than directing a receiver. Ze's receiver already reads exactly the length-prefixed extent and never scans for a terminator (Notification.ShutdownMessage, internal/component/bgp/message/notification.go). |
| `3` | Operational Considerations | 0 | reviewed | Operational Considerations. Encourages operators to describe the reason for the shutdown and works through the "[TICKET-1-1438367390] software upgrade" example. Advice to an operator writing a message, with no RFC 2119 keyword and no directive to an implementation. |
| `4` | Error Handling | 0 | reviewed | Error Handling. Two directives, neither MUST-level and neither visible to the site scan: an invalid Length value or an invalid UTF-8 sequence SHOULD be logged for the attention of the operator, and the malformed Shutdown Communication MAY be logged in a hexdump format. Both are the unsourced ids below. |
| `5` | IANA Considerations | 0 | skipped (iana) | IANA Considerations. Records that IANA references this document, in addition to RFC 4486, for subcodes 2 and 4 in the "Cease NOTIFICATION message subcodes" registry. Binds IANA, not a speaker. |
| `6` | Security Considerations | 1 | reviewed | Security Considerations. Carries one capitalised site, the REQUIRED UTF-8 "Shortest Form" encoding, mapped below to RFC8203-6-1. This is the ONLY place RFC 8203 states shortest form; its section 2 does not, which is a real difference from RFC 9003, where the same sentence appears in both sections. The rest is risk narration: the Unicode issues of UTR36, the syslog-injection risk the 128-octet limit is said to limit, forgery and snooping without an integrity or confidentiality transport, and the RFC 6973 section 6.1 data-minimization advice. Each is stated as advice to implementers and operators, with no keyword. |
| `7` | References | 0 | skipped (references) | References. Section header for the two reference lists below. |
| `7.1` | not stated | 0 | skipped (references) | Normative References: RFC 2119, RFC 3629, RFC 4271, RFC 4486, RFC 8174. |
| `7.2` | Informative References: RFC 4272, RFC 5424, RFC 6973, UTR36 | 0 | skipped (references) | Informative References: RFC 4272, RFC 5424, RFC 6973, UTR36. |

### Excluded sentences

The review of RFC 8203 excluded no sentences: every sentence identified as an obligation is linked to a requirement.

## Superseded

No document obsoletes RFC 8203, so its obligations are stated where they were written.
