# RFC 8203 - BGP Administrative Shutdown Communication

Supported. Every requirement this repository extracted from RFC 8203, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 80.0% | 4 of 5 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 20.0% | 1 of 5 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 5 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 0.0% | 0 of 5 binding obligations | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 0.0% | 0 of 9 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 5 | of 9 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 5 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

The 4 shares marked as a part above are the whole of the 5 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| No test at all | ok | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Supported |
| Enrolment | Enrolled |
| Requirements | 9 |
| Gated MUST-level | 5 |
| Obligations that bind Ze | 5 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 9 |
| Tagged units | 9 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc8203.md` |
| Requirement shard | `rfc/requirements/rfc8203.md` |
| RFC text | `rfc/full/rfc8203.txt` |

## Enrolment

Enrolled: Administrative Shutdown Communication: five MUST-level requirements over the Cease NOTIFICATION shutdown message, all bound to the message codec (internal/component/bgp/message/notification.go). RFC8203-2-1 (subcode 2/4) is proven by ShutdownMessage extracting for Admin Shutdown/Reset and returning empty for any other subcode; RFC8203-2-3 (UTF-8) and RFC8203-2-4 (MUST NOT interpret invalid UTF-8) by a valid message round-tripping while an invalid sequence is rejected; RFC8203-6-1 (shortest form) by an overlong 0xC0 0xAF encoding being rejected where the canonical form is accepted (Go utf8.Valid enforces shortest form). RFC8203-2-2 (length 0-128) is {single-polarity: positive}: BuildShutdownData caps the sender at 128, while the receiver deliberately follows RFC 9003 (which obsoletes RFC 8203 and raised the cap to 255), so there is no over-128 rejection on receive to assert.

## What the public ledger says

**Status:** Supported

**What the ledger says is covered:**

UTF-8 shutdown message for Cease/Admin Shutdown and Cease/Admin Reset.

**What the ledger says remains:**

Sender keeps the conservative 128-byte RFC 8203 limit.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 4 | one part of the gated population |
| Annotated instead of tested | 1 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **5** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (4):** [`RFC8203-2-1`](#rfc8203-2-1), [`RFC8203-2-3`](#rfc8203-2-3), [`RFC8203-6-1`](#rfc8203-6-1), [`RFC8203-2-4`](#rfc8203-2-4)

**Annotated instead of tested (1):** [`RFC8203-2-2`](#rfc8203-2-2)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC8203-2-1` | Error Subcode value must be 2 ("Administrative Shutdown") or 4 ("Administrative Reset") (§2) | MUST | 2 - Shutdown Communication | **positive:** `unit/verify` [`TestRFC8203Subcode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/notification_test.go#L368). **negative:** `unit/verify` [`TestRFC8203Subcode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/notification_test.go#L370) |
| `RFC8203-2-2` | Length field must range from 0 to 128 inclusive (§2) | MUST | 2 - Shutdown Communication | **positive:** `unit/verify` [`TestRFC8203LengthCap`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/notification_test.go#L393). **negative:** no negative test. **{single-polarity}:** the sender enforces the 0-128 range -- BuildShutdownData truncates a longer message to 128 at a UTF-8 boundary (internal/component/bgp/message/notification.go:312) -- but the receiver deliberately follows RFC 9003, which obsoletes RFC 8203 and raised the cap to 255 (ShutdownMessage reads a 1-byte length up to 255, notification.go:268-284), so ze intentionally does not reject a 129-255 length on receive and there is no over-128-rejected behavior to assert |
| `RFC8203-2-3` | Shutdown Communication field must be encoded using UTF-8 (§2) | MUST | 2 - Shutdown Communication | **positive:** `unit/verify` [`TestRFC8203UTF8Valid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/notification_test.go#L405). **negative:** `unit/verify` [`TestRFC8203UTF8Invalid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/notification_test.go#L423) |
| `RFC8203-6-1` | UTF-8 "Shortest Form" encoding is required (§6) | MUST | 6 - Security Considerations | **positive:** `unit/verify` [`TestRFC8203ShortestFormUTF8`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/notification_test.go#L441). **negative:** `unit/verify` [`TestRFC8203ShortestFormUTF8`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/notification_test.go#L443) |
| `RFC8203-2-4` | Receiving BGP speaker must not interpret invalid UTF-8 sequences (§2) | MUST NOT | 2 - Shutdown Communication | **positive:** `unit/verify` [`TestRFC8203UTF8Invalid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/notification_test.go#L425). **negative:** `unit/verify` [`TestRFC8203UTF8Valid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/notification_test.go#L407) |
| `RFC8203-2-5` | Reporting of Shutdown Communication should include methods such as Syslog (§2) | SHOULD | 2 - Shutdown Communication | **positive:** no positive test. **negative:** no negative test |
| `RFC8203-4-1` | Log a message for the operator when an invalid Length value or invalid UTF-8 sequence is received (§4) | SHOULD | 4 - Error Handling | **positive:** no positive test. **negative:** no negative test |
| `RFC8203-2-6` | Sender may include a UTF-8 encoded string in the Cease NOTIFICATION (§2) | MAY | 2 - Shutdown Communication | **positive:** no positive test. **negative:** no negative test |
| `RFC8203-4-2` | Erroneous or malformed Shutdown Communication may be logged in hexdump format (§4) | MAY | 4 - Error Handling | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

RFC 8203 declares no gap, and every gated MUST it carries has a test bound to it.

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC8203-2-1`](#rfc8203-2-1)

Error Subcode value must be 2 ("Administrative Shutdown") or 4 ("Administrative Reset") (§2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8203Subcode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/notification_test.go#L370) | unit/verify | unproven |
| positive | [`TestRFC8203Subcode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/notification_test.go#L368) | unit/verify | unproven |

### [`RFC8203-2-2`](#rfc8203-2-2)

Length field must range from 0 to 128 inclusive (§2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC8203LengthCap`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/notification_test.go#L393) | unit/verify | unproven |

### [`RFC8203-2-3`](#rfc8203-2-3)

Shutdown Communication field must be encoded using UTF-8 (§2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8203UTF8Invalid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/notification_test.go#L423) | unit/verify | unproven |
| positive | [`TestRFC8203UTF8Valid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/notification_test.go#L405) | unit/verify | unproven |

### [`RFC8203-6-1`](#rfc8203-6-1)

UTF-8 "Shortest Form" encoding is required (§6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8203ShortestFormUTF8`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/notification_test.go#L443) | unit/verify | unproven |
| positive | [`TestRFC8203ShortestFormUTF8`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/notification_test.go#L441) | unit/verify | unproven |

### [`RFC8203-2-4`](#rfc8203-2-4)

Receiving BGP speaker must not interpret invalid UTF-8 sequences (§2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8203UTF8Valid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/notification_test.go#L407) | unit/verify | unproven |
| positive | [`TestRFC8203UTF8Invalid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/notification_test.go#L425) | unit/verify | unproven |

## Extraction sign-off

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
| `1` | Introduction | 0 | walked | Introduction. Indicative prose: correlating a session teardown with an offline notice is troublesome, and this document updates RFC 4486 by specifying a mechanism to carry a short freeform UTF-8 message in a Cease NOTIFICATION. It states what the document does, never what a speaker must do. |
| `1.1` | Requirements Language | 0 | walked | Requirements Language. The BCP 14 key-words paragraph, which also states that the key words bind only when they appear in all capitals. It tells a reader how to read the other sections and binds no speaker, which is why the derivation excludes it from the site inventory. |
| `2` | Shutdown Communication | 4 | walked | Shutdown Communication. The document's whole normative surface for the wire format: four capitalised MUST-level sites, mapped below to RFC8203-2-1 through RFC8203-2-4. Its remaining directives are the opening MAY (a speaker MAY include a UTF-8 string) and the closing SHOULD (reporting mechanisms SHOULD include methods such as Syslog); both are the unsourced ids below. Three sentences the site scan cannot see are descriptive, not obligations: the Figure 1 field diagram and its Error Code 6, the note that a multibyte Shutdown Communication holds fewer characters than the length value, and "This field is not NUL terminated", which describes the field rather than directing a receiver. Ze's receiver already reads exactly the length-prefixed extent and never scans for a terminator (Notification.ShutdownMessage, internal/component/bgp/message/notification.go). |
| `3` | Operational Considerations | 0 | walked | Operational Considerations. Encourages operators to describe the reason for the shutdown and works through the "[TICKET-1-1438367390] software upgrade" example. Advice to an operator writing a message, with no RFC 2119 keyword and no directive to an implementation. |
| `4` | Error Handling | 0 | walked | Error Handling. Two directives, neither MUST-level and neither visible to the site scan: an invalid Length value or an invalid UTF-8 sequence SHOULD be logged for the attention of the operator, and the malformed Shutdown Communication MAY be logged in a hexdump format. Both are the unsourced ids below. |
| `5` | IANA Considerations | 0 | skipped (iana) | IANA Considerations. Records that IANA references this document, in addition to RFC 4486, for subcodes 2 and 4 in the "Cease NOTIFICATION message subcodes" registry. Binds IANA, not a speaker. |
| `6` | Security Considerations | 1 | walked | Security Considerations. Carries one capitalised site, the REQUIRED UTF-8 "Shortest Form" encoding, mapped below to RFC8203-6-1. This is the ONLY place RFC 8203 states shortest form; its section 2 does not, which is a real difference from RFC 9003, where the same sentence appears in both sections. The rest is risk narration: the Unicode issues of UTR36, the syslog-injection risk the 128-octet limit is said to limit, forgery and snooping without an integrity or confidentiality transport, and the RFC 6973 section 6.1 data-minimization advice. Each is stated as advice to implementers and operators, with no keyword. |
| `7` | References | 0 | skipped (references) | References. Section header for the two reference lists below. |
| `7.1` | not stated | 0 | skipped (references) | Normative References: RFC 2119, RFC 3629, RFC 4271, RFC 4486, RFC 8174. |
| `7.2` | Informative References: RFC 4272, RFC 5424, RFC 6973, UTR36 | 0 | skipped (references) | Informative References: RFC 4272, RFC 5424, RFC 6973, UTR36. |

### Excluded sentences

The walk over RFC 8203 declined no sentence: every site it found is mapped to a requirement.

## Superseded

No document obsoletes RFC 8203, so its obligations are stated where they were written.
