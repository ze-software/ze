# RFC 9003 - Extended BGP Administrative Shutdown Communication

Supported. Every requirement this repository extracted from RFC 9003, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 100.0% | 4 of 4 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 4 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 4 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 0.0% | 0 of 4 binding obligations | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 0.0% | 0 of 8 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 4 | of 11 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 4 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

The 4 shares marked as a part above are the whole of the 4 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 11 |
| Gated MUST-level | 4 |
| Obligations that bind Ze | 4 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 8 |
| Tagged units | 8 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc9003.md` |
| Requirement shard | `rfc/requirements/rfc9003.md` |
| RFC text | `rfc/full/rfc9003.txt` |

## Enrolment

Enrolled: Extended BGP Administrative Shutdown Communication (obsoletes RFC 8203): four MUST-level requirements over the Cease NOTIFICATION shutdown-communication codec (internal/component/bgp/message/notification.go), all bound to the same tests as the already-enrolled RFC 8203 because the requirements are identical (RFC 9003 only raises the sender length cap to 255). RFC9003-2-1 (Shutdown Communication MUST be UTF-8) and RFC9003-2-4 (receiver MUST NOT interpret invalid UTF-8) share TestRFC8203UTF8Valid/TestRFC8203UTF8Invalid: a valid message round-trips while an invalid sequence yields an error and empty string (ShutdownMessage utf8.Valid guard, notification.go:280). RFC9003-2-2 (UTF-8 Shortest Form REQUIRED) shares TestRFC8203ShortestFormUTF8: canonical 0xC3 0xA9 accepted, overlong 0xC0 0xAF rejected (Go utf8.Valid enforces shortest form). RFC9003-2-3 (Error Subcode MUST be 2 Administrative Shutdown or 4 Administrative Reset) shares TestRFC8203Subcode: ShutdownMessage extracts for those two subcodes and returns empty for any other (notification.go:258). All four have both polarities.

## What the public ledger says

**Status:** Supported

**What the ledger says is covered:**

- Receiver accepts the updated UTF-8 format and 255-byte length field
- invalid UTF-8 is stripped on send.


**What the ledger says remains:**

Sender remains conservative at 128 bytes for interoperability.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 4 | one part of the gated population |
| Annotated instead of tested | 0 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **4** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (4):** [`RFC9003-2-1`](#rfc9003-2-1), [`RFC9003-2-2`](#rfc9003-2-2), [`RFC9003-2-3`](#rfc9003-2-3), [`RFC9003-2-4`](#rfc9003-2-4)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC9003-2-1` | Shutdown Communication field MUST be encoded using UTF-8 (S2) | MUST | 2 - Shutdown Communication | **positive:** `unit/verify` [`TestRFC8203UTF8Valid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/notification_test.go#L409). **negative:** `unit/verify` [`TestRFC8203UTF8Invalid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/notification_test.go#L427) |
| `RFC9003-2-2` | UTF-8 "Shortest Form" encoding is REQUIRED (S2, S6) | MUST | 2 - Shutdown Communication | **positive:** `unit/verify` [`TestRFC8203ShortestFormUTF8`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/notification_test.go#L446). **negative:** `unit/verify` [`TestRFC8203ShortestFormUTF8`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/notification_test.go#L448) |
| `RFC9003-2-3` | Error Subcode value MUST be one of: 2 (Administrative Shutdown) or 4 (Administrative Reset) (S2) | MUST | 2 - Shutdown Communication | **positive:** `unit/verify` [`TestRFC8203Subcode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/notification_test.go#L372). **negative:** `unit/verify` [`TestRFC8203Subcode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/notification_test.go#L375) |
| `RFC9003-2-4` | Receiving BGP speaker MUST NOT interpret invalid UTF-8 sequences (S2) | MUST NOT | 2 - Shutdown Communication | **positive:** `unit/verify` [`TestRFC8203UTF8Invalid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/notification_test.go#L429). **negative:** `unit/verify` [`TestRFC8203UTF8Valid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/notification_test.go#L411) |
| `RFC9003-2-5` | Reporting mechanisms SHOULD include methods such as syslog (S2) | SHOULD | 2 - Shutdown Communication | **positive:** no positive test. **negative:** no negative test |
| `RFC9003-4-1` | If invalid UTF-8 sequence received, a message indicating this event SHOULD be logged (S4) | SHOULD | 4 - Error Handling | **positive:** no positive test. **negative:** no negative test |
| `RFC9003-3-1` | If peer support unknown, Shutdown Communication SHOULD NOT be longer than 128 octets (S3) | SHOULD NOT | 3 - Operational Considerations | **positive:** no positive test. **negative:** no negative test |
| `RFC9003-2-6` | Sender MAY include a UTF-8-encoded string in the Cease NOTIFICATION (S2) | MAY | 2 - Shutdown Communication | **positive:** no positive test. **negative:** no negative test |
| `RFC9003-4-2` | Erroneous or malformed Shutdown Communication MAY be logged in hexdump format (S4) | MAY | 4 - Error Handling | **positive:** no positive test. **negative:** no negative test |
| `RFC9003-3-2` | If peer known to support this spec, Shutdown Communication up to 255 octets MAY be sent (S3) | MAY | 3 - Operational Considerations | **positive:** no positive test. **negative:** no negative test |
| `RFC9003-3-3` | If peer support unknown, a Shutdown Communication MAY be sent (S3) | MAY | 3 - Operational Considerations | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

RFC 9003 declares no gap, and every gated MUST it carries has a test bound to it.

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC9003-2-1`](#rfc9003-2-1)

Shutdown Communication field MUST be encoded using UTF-8 (S2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8203UTF8Invalid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/notification_test.go#L427) | unit/verify | unproven |
| positive | [`TestRFC8203UTF8Valid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/notification_test.go#L409) | unit/verify | unproven |

### [`RFC9003-2-2`](#rfc9003-2-2)

UTF-8 "Shortest Form" encoding is REQUIRED (S2, S6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8203ShortestFormUTF8`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/notification_test.go#L448) | unit/verify | unproven |
| positive | [`TestRFC8203ShortestFormUTF8`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/notification_test.go#L446) | unit/verify | unproven |

### [`RFC9003-2-3`](#rfc9003-2-3)

Error Subcode value MUST be one of: 2 (Administrative Shutdown) or 4 (Administrative Reset) (S2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8203Subcode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/notification_test.go#L375) | unit/verify | unproven |
| positive | [`TestRFC8203Subcode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/notification_test.go#L372) | unit/verify | unproven |

### [`RFC9003-2-4`](#rfc9003-2-4)

Receiving BGP speaker MUST NOT interpret invalid UTF-8 sequences (S2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC8203UTF8Valid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/notification_test.go#L411) | unit/verify | unproven |
| positive | [`TestRFC8203UTF8Invalid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/notification_test.go#L429) | unit/verify | unproven |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | ze-work agent, spec-rfcgate-6, rfc9003 |
| Signed off | 2026-08-31 |
| Register | rfc2119 |
| Source | rfc/full/rfc9003.txt |
| Source fingerprint | cf060177c8f68e0c |
| Record | rfc/extraction/rfc9003.json |
| Mapped sentences | 4 |
| Declined as scope | 1 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | skipped (front-matter) | Title block, Status of This Memo, Copyright Notice, Abstract and Table of Contents. The Abstract records that the document updates RFC 4486 and obsoletes RFC 8203 by defining a Shutdown Communication of up to 255 octets. It directs no speaker. |
| `1` | Introduction | 0 | walked | Introduction. Indicative prose, near-identical to RFC 8203 section 1, plus the sentence "This document obsoletes [RFC8203]; the specific differences and rationale are discussed in detail in Appendix A". Obsoleting is a relationship between documents, not an obligation on a speaker. |
| `1.1` | Requirements Language | 0 | walked | Requirements Language. The BCP 14 key-words paragraph, identical in force to RFC 8203 section 1.1 and excluded from the site inventory for the same reason. |
| `2` | Shutdown Communication | 4 | walked | Shutdown Communication. Four capitalised MUST-level sites, mapped below to RFC9003-2-3 (subcode), RFC9003-2-1 (UTF-8), RFC9003-2-4 (invalid UTF-8) and RFC9003-2-2 (shortest form). Each is stated in this document's own voice and cites no other RFC for its force, so each is owned here and not read across from RFC 8203. Two differences from RFC 8203 section 2 are load-bearing and both are REMOVALS or ADDITIONS, not restatements: RFC 8203's "The length value MUST range from 0 to 128 inclusive" is DELETED here, leaving RFC 9003 with no MUST-level length bound anywhere (what replaces it is the MAY/SHOULD NOT pair in section 3), and the UTF-8 "Shortest Form" sentence is ADDED here, where RFC 8203 stated it only in section 6. The remaining directives are the opening MAY and the closing syslog SHOULD, the unsourced ids below. "This field is not NUL terminated" is descriptive here as in RFC 8203. |
| `3` | Operational Considerations | 0 | walked | Operational Considerations. Carries the length policy RFC 8203 stated as a MUST in its section 2, restated here at a lower level and in a different section: "If it is known that the peer BGP speaker supports this specification, then a Shutdown Communication that is not longer than 255 octets MAY be sent. Otherwise, a Shutdown Communication MAY be sent, but it SHOULD NOT be longer than 128 octets." The three ids below carry it. The site scan sees no site because the sentence is MAY/SHOULD NOT rather than MUST-level, which is the substantive finding of this walk: the 128-octet bound is an obligation under RFC 8203 and advice under RFC 9003. The rest of the section is the ticket-reference example inherited from RFC 8203, plus indicative statements about how an RFC 8203 speaker and a speaker implementing neither document will treat an over-long message, and the observation that a receiver cannot acknowledge a Shutdown Communication. |
| `4` | Error Handling | 0 | walked | Error Handling. The SHOULD to log an invalid UTF-8 sequence for the attention of the operator and the MAY to log the malformed Shutdown Communication in hexdump format, both the unsourced ids below. Narrower than RFC 8203 section 4, which also named an invalid Length value as a logging trigger; RFC 9003 drops that clause, consistent with its section 2 dropping the length MUST. |
| `5` | IANA Considerations | 0 | skipped (iana) | IANA Considerations. Records that IANA has referenced this document, in addition to RFC 4486, at subcodes "Administrative Shutdown" and "Administrative Reset" in the "BGP Cease NOTIFICATION message subcodes" registry. Binds IANA. |
| `6` | Security Considerations | 1 | walked | Security Considerations. Its one capitalised site repeats the shortest-form sentence verbatim from section 2 and is excluded below as a duplicate of RFC9003-2-2, which site 2:4 maps. The remaining prose mirrors RFC 8203 section 6 with one change of substance: where RFC 8203 said the syslog-injection attack is limited because the Shutdown Communication "is limited to 128 octets in length", this document says "The 255-octet length limit ... may help limit" it. Advice, with no keyword. |
| `7` | References | 0 | skipped (references) | References. Section header for the two reference lists below. |
| `7.1` | not stated | 0 | skipped (references) | Normative References: RFC 2119, RFC 3629, RFC 4271, RFC 4486, RFC 8174. |
| `7.2` | not stated | 0 | skipped (references) | Informative References: RFC 4272, RFC 5424, RFC 6973, RFC 8203, UTR36. |
| `A` | Appendix A, Changes to RFC 8203 | 0 | skipped (appendix-non-normative) | Appendix A, Changes to RFC 8203. Records that the maximum permitted length was changed from 128 to 255, gives the operator feedback that motivated it (a 65-byte English phrase is 139 bytes in Russian), and states indicatively that an RFC 8203 speaker receiving a message longer than 128 octets will bring it to an operator's attention but otherwise process the NOTIFICATION as normal. A change log with no keyword and no directive. |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `6:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 6 repeats the section 2 sentence word for word, so it restates an obligation site 2:4 already maps to RFC9003-2-2 rather than adding one. Not cross-document: the sentence names UTR36, a Unicode technical report, and cites no RFC for its force. | UTF-8 "Shortest Form" encoding is REQUIRED to guard against the technical issues outlined in [UTR36]. |

## Superseded

No document obsoletes RFC 9003, so its obligations are stated where they were written.
