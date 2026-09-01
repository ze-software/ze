# RFC 2869 - RADIUS Extensions

Supported for subscriber access. Every requirement this repository extracted from RFC 2869, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 88.9% | 8 of 9 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 11.1% | 1 of 9 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 9 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 0.0% | 0 of 9 binding obligations | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 0.0% | 0 of 17 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 11 | of 14 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 2 | of 11 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

The 4 shares marked as a part above are the whole of the 9 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Public status | Supported for subscriber access |
| Enrolment | Enrolled |
| Requirements | 14 |
| Gated MUST-level | 11 |
| Obligations that bind Ze | 9 |
| Not applicable, so out of scope | 2 |
| Declared gaps | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 17 |
| Tagged units | 17 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc2869.md` |
| Requirement shard | `rfc/requirements/rfc2869.md` |
| RFC text | `rfc/full/rfc2869.txt` |

## Enrolment

Enrolled: RADIUS Extensions (NAS/accounting-client obligations): five MUST-level requirements. x-2 (Gigawords present only for Stop/Interim) and x-3 (Gigawords present only when the counter is non-zero) are met with positive+negative tags on the accounting builder tests (internal/component/l2tp/plugins/authradius/acct.go). x-5 (NAS derives Gigawords from the actual 64-bit byte count) is {single-polarity: positive} bound to TestSplitGigawords. x-1 (server handles missing Gigawords) and x-4 (reconstruct total from received Gigawords) are {not-applicable}: ze runs only the RADIUS accounting client (NAS) role and has no accounting-server receive path (internal/component/radius/client.go binds only to receive responses to its own requests).

## What the public ledger says

**Status:** Supported for subscriber access

**What the ledger says is covered:**

Gigaword counters and selected accounting extensions.

**What the ledger says remains:**

Scoped to subscriber access.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 8 | one part of the gated population |
| Annotated instead of tested | 3 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **11** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (8):** [`RFC2869-1.1-1`](#rfc2869-1.1-1), [`RFC2869-1.1-2`](#rfc2869-1.1-2), [`RFC2869-2.1-1`](#rfc2869-2.1-1), [`RFC2869-2.1-2`](#rfc2869-2.1-2), [`RFC2869-5.19-1`](#rfc2869-5.19-1), [`RFC2869-x-2`](#rfc2869-x-2), [`RFC2869-x-3`](#rfc2869-x-3), [`RFC2869-5.14-1`](#rfc2869-5.14-1)

**Annotated instead of tested (3):** [`RFC2869-x-1`](#rfc2869-x-1), [`RFC2869-x-4`](#rfc2869-x-4), [`RFC2869-x-5`](#rfc2869-x-5)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC2869-1.1-1` | A NAS that does not implement a given service MUST NOT implement the RADIUS attributes for that service (§1.1) | MUST NOT | 1.1 - Specification of Requirements | **positive:** `unit/verify` [`TestRFC2869DictionaryCoversTheServicesZeOffers`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2869_unoffered_service_attributes_test.go#L97). **negative:** `unit/verify` [`TestRFC2869DictionaryDeclaresNoAttributeForAnUnofferedService`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2869_unoffered_service_attributes_test.go#L126) |
| `RFC2869-1.1-2` | A NAS MUST treat a RADIUS access-request requesting an unavailable service as an access-reject instead (§1.1) | MUST | 1.1 - Specification of Requirements | **positive:** `unit/verify` [`TestRFC2869AccessRequestNamesTheServiceZeOffers`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc2869_access_request_service_test.go#L57). **negative:** `unit/verify` [`TestRFC2869AccessRequestNeverRequestsAnUnavailableService`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc2869_access_request_service_test.go#L99) |
| `RFC2869-2.1-1` | A NAS MUST ensure that only a single generation of an interim Accounting message for a given session is present in the retransmission queue at any given time (§2.1) | MUST | 2.1 - RADIUS support for Interim Accounting Updates | **positive:** `unit/verify` [`TestRFC2869InterimLoopSendsOnItsInterval`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc2869_interim_generation_test.go#L89). **negative:** `unit/verify` [`TestRFC2869InterimLoopKeepsOneGenerationOutstanding`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc2869_interim_generation_test.go#L117) |
| `RFC2869-2.1-2` | A locally configured interim interval value on the NAS MUST override the value found in an Access-Accept (§2.1) | MUST | 2.1 - RADIUS support for Interim Accounting Updates | **positive:** `unit/verify` [`TestRFC2869LocalAcctIntervalOverridesAccessAccept`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc2869_interim_interval_precedence_test.go#L102). **negative:** `unit/verify` [`TestRFC2869AbsentAcctIntervalLeavesTheAccessAcceptInCharge`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc2869_interim_interval_precedence_test.go#L126) |
| `RFC2869-5.19-1` | An Access-Request that contains a User-Password, a CHAP-Password, an ARAP-Password or one or more EAP-Message attributes MUST NOT contain more than one type of those four attributes (§5.19 Note 1) | MUST NOT | 5.19 - Table of Attributes heading and its opening paragraph | **positive:** `unit/verify` [`TestRFC2869AccessRequestCarriesTheCredentialOfItsMethod`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc2869_access_request_service_test.go#L137). **negative:** `unit/verify` [`TestRFC2869AccessRequestCarriesOneKindOfCredential`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc2869_access_request_service_test.go#L169) |
| `RFC2869-x-1` | Accounting servers MUST handle the absence of Gigaword attributes for backward compatibility with RFC 2866-only implementations (Implementation Constraints) | MUST | x | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze runs only the RADIUS accounting client (NAS) role; it has no accounting-server receive path (internal/component/radius/client.go binds only to receive responses to its own requests), so it never handles missing Gigawords on receipt |
| `RFC2869-x-2` | Gigaword attributes MUST only be present in Accounting-Request records where Acct-Status-Type is Stop or Interim-Update (Presence Rules) | MUST | x | **positive:** `unit/verify` [`TestBuildAcctPacketGigawords`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/acct_test.go#L293). **negative:** `unit/verify` [`TestRFC2869GigawordsAbsentOnStart`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc2869_accounting_test.go#L21) |
| `RFC2869-x-3` | Gigaword attributes MUST only be included when the counter value is non-zero (i.e., 32-bit octet counter has wrapped at least once) (Presence Rules) | MUST | x | **positive:** `unit/verify` [`TestBuildAcctPacketGigawords`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/acct_test.go#L295). **negative:** `unit/verify` [`TestBuildAcctPacketWithCounters`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/acct_test.go#L257) |
| `RFC2869-x-4` | Total byte count MUST be reconstructed as (Gigawords * 2^32) + Octets (Gigaword Accounting) | MUST | x | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze runs only the RADIUS accounting client (NAS) role; it has no accounting-server receive path (internal/component/radius/client.go binds only to receive responses to its own requests), so it never reconstructs a byte total from received Gigaword attributes |
| `RFC2869-x-5` | NAS MUST compute Gigawords from the actual byte count, not independently track wrap events (Implementation Constraints) | MUST | x | **positive:** `unit/verify` [`TestSplitGigawords`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/acct_test.go#L227). **negative:** no negative test. **{single-polarity}:** splitGigawords derives Gigawords directly from the 64-bit byte counter (internal/component/l2tp/plugins/authradius/acct.go:37-39) with no separate wrap counter; there is no reject path so no negative case exists |
| `RFC2869-5.14-1` | A RADIUS client receiving an Access-Accept, Access-Reject or Access-Challenge with a Message-Authenticator attribute present MUST calculate the correct value of the Message-Authenticator and silently discard the packet if it does not match the value sent (§5.14) | MUST | 5.14 - Message-Authenticator | **positive:** `unit/verify` [`TestRFC2869AccessAcceptWithValidMessageAuthenticatorIsAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2869_message_authenticator_test.go#L153). **negative:** `unit/verify` [`TestRFC2869AccessAcceptWithWrongMessageAuthenticatorIsDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2869_message_authenticator_test.go#L185) |
| `RFC2869-5.17-1` | Either NAS-Port or NAS-Port-Id SHOULD be present in an Access-Request packet, if the NAS differentiates among its ports (§5.17) | SHOULD | 5.17 - NAS-Port-Id | **positive:** no positive test. **negative:** no negative test |
| `RFC2869-x-6` | State attribute (type 24) MAY be maintained between Access-Challenge and Access-Request (Other Attributes) | MAY | x | **positive:** no positive test. **negative:** no negative test |
| `RFC2869-x-7` | Acct-Interim-Interval attribute (type 85) MAY be used to configure seconds between Interim-Updates (Other Attributes) | MAY | x | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC2869-x-1`](#rfc2869-x-1) Accounting servers MUST handle the absence of Gigaword attributes for backward compatibility with RFC 2866-only implementations (Implementation Constraints) | no test | no test carries this requirement id; annotated {not-applicable}: ze runs only the RADIUS accounting client (NAS) role; it has no accounting-server receive path (internal/component/radius/client.go binds only to receive responses to its own requests), so it never handles missing Gigawords on receipt |
| [`RFC2869-x-4`](#rfc2869-x-4) Total byte count MUST be reconstructed as (Gigawords * 2^32) + Octets (Gigaword Accounting) | no test | no test carries this requirement id; annotated {not-applicable}: ze runs only the RADIUS accounting client (NAS) role; it has no accounting-server receive path (internal/component/radius/client.go binds only to receive responses to its own requests), so it never reconstructs a byte total from received Gigaword attributes |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC2869-1.1-1`](#rfc2869-1.1-1)

A NAS that does not implement a given service MUST NOT implement the RADIUS attributes for that service (§1.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2869DictionaryDeclaresNoAttributeForAnUnofferedService`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2869_unoffered_service_attributes_test.go#L126) | unit/verify | unproven |
| positive | [`TestRFC2869DictionaryCoversTheServicesZeOffers`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2869_unoffered_service_attributes_test.go#L97) | unit/verify | unproven |

### [`RFC2869-1.1-2`](#rfc2869-1.1-2)

A NAS MUST treat a RADIUS access-request requesting an unavailable service as an access-reject instead (§1.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2869AccessRequestNeverRequestsAnUnavailableService`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc2869_access_request_service_test.go#L99) | unit/verify | unproven |
| positive | [`TestRFC2869AccessRequestNamesTheServiceZeOffers`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc2869_access_request_service_test.go#L57) | unit/verify | unproven |

### [`RFC2869-2.1-1`](#rfc2869-2.1-1)

A NAS MUST ensure that only a single generation of an interim Accounting message for a given session is present in the retransmission queue at any given time (§2.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2869InterimLoopKeepsOneGenerationOutstanding`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc2869_interim_generation_test.go#L117) | unit/verify | unproven |
| positive | [`TestRFC2869InterimLoopSendsOnItsInterval`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc2869_interim_generation_test.go#L89) | unit/verify | unproven |

### [`RFC2869-2.1-2`](#rfc2869-2.1-2)

A locally configured interim interval value on the NAS MUST override the value found in an Access-Accept (§2.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2869AbsentAcctIntervalLeavesTheAccessAcceptInCharge`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc2869_interim_interval_precedence_test.go#L126) | unit/verify | unproven |
| positive | [`TestRFC2869LocalAcctIntervalOverridesAccessAccept`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc2869_interim_interval_precedence_test.go#L102) | unit/verify | unproven |

### [`RFC2869-5.19-1`](#rfc2869-5.19-1)

An Access-Request that contains a User-Password, a CHAP-Password, an ARAP-Password or one or more EAP-Message attributes MUST NOT contain more than one type of those four attributes (§5.19 Note 1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2869AccessRequestCarriesOneKindOfCredential`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc2869_access_request_service_test.go#L169) | unit/verify | unproven |
| positive | [`TestRFC2869AccessRequestCarriesTheCredentialOfItsMethod`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc2869_access_request_service_test.go#L137) | unit/verify | unproven |

### [`RFC2869-x-1`](#rfc2869-x-1)

Accounting servers MUST handle the absence of Gigaword attributes for backward compatibility with RFC 2866-only implementations (Implementation Constraints)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2869-x-1, so no unit is bound to it.

### [`RFC2869-x-2`](#rfc2869-x-2)

Gigaword attributes MUST only be present in Accounting-Request records where Acct-Status-Type is Stop or Interim-Update (Presence Rules)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2869GigawordsAbsentOnStart`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc2869_accounting_test.go#L21) | unit/verify | unproven |
| positive | [`TestBuildAcctPacketGigawords`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/acct_test.go#L293) | unit/verify | unproven |

### [`RFC2869-x-3`](#rfc2869-x-3)

Gigaword attributes MUST only be included when the counter value is non-zero (i.e., 32-bit octet counter has wrapped at least once) (Presence Rules)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestBuildAcctPacketWithCounters`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/acct_test.go#L257) | unit/verify | unproven |
| positive | [`TestBuildAcctPacketGigawords`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/acct_test.go#L295) | unit/verify | unproven |

### [`RFC2869-x-4`](#rfc2869-x-4)

Total byte count MUST be reconstructed as (Gigawords * 2^32) + Octets (Gigaword Accounting)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2869-x-4, so no unit is bound to it.

### [`RFC2869-x-5`](#rfc2869-x-5)

NAS MUST compute Gigawords from the actual byte count, not independently track wrap events (Implementation Constraints)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestSplitGigawords`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/acct_test.go#L227) | unit/verify | unproven |

### [`RFC2869-5.14-1`](#rfc2869-5.14-1)

A RADIUS client receiving an Access-Accept, Access-Reject or Access-Challenge with a Message-Authenticator attribute present MUST calculate the correct value of the Message-Authenticator and silently discard the packet if it does not match the value sent (§5.14)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2869AccessAcceptWithWrongMessageAuthenticatorIsDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2869_message_authenticator_test.go#L185) | unit/verify | unproven |
| positive | [`TestRFC2869AccessAcceptWithValidMessageAuthenticatorIsAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2869_message_authenticator_test.go#L153) | unit/verify | unproven |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | ze-work agent, spec-rfcgate-6 phase 2, rfc2869 |
| Signed off | 2026-08-31 |
| Register | rfc2119 |
| Source | rfc/full/rfc2869.txt |
| Source fingerprint | 5b468f2205956613 |
| Record | rfc/extraction/rfc2869.json |
| Mapped sentences | 6 |
| Declined as scope | 38 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | skipped (front-matter) | Title block, Status of this Memo, copyright notice, Abstract and table of contents. The Abstract states what the memo adds to RADIUS and binds no speaker. |
| `1` | Introduction | 0 | walked | Introduction. Says what RFC 2865 and RFC 2866 describe, that the new attributes are experimental, and that attributes are Type-Length-Value triples. No obligation. |
| `1.1` | Specification of Requirements | 3 | walked | Specification of Requirements. The RFC 2119 key-words paragraph, the compliant / unconditionally compliant / conditionally compliant definitions, and one normative paragraph that scopes the document's other requirements to the services a NAS offers. The three sites below come from that last paragraph. |
| `1.2` | Terminology | 0 | walked | Terminology. Defines service, session and 'silently discard'. The 'silently discard' entry carries two SHOULD clauses about logging the error and counting the event; neither is gated and neither is declared in rfc/short/rfc2869.md. |
| `2` | Operation | 0 | walked | Operation. One sentence: operation is identical to RFC 2865 and RFC 2866. No obligation of its own. |
| `2.1` | RADIUS support for Interim Accounting Updates | 2 | walked | RADIUS support for Interim Accounting Updates. Two capitalised MUSTs, both binding the NAS, both classified below. |
| `2.2` | RADIUS support for Apple Remote Access Protocol | 1 | walked | RADIUS support for Apple Remote Access Protocol. Describes ARAP's two-way DES exchange and the ARAP attribute set. Ze implements no ARAP: internal/component/radius/dict.go declares no attribute in the 70-74 or 84 range, and internal/component/l2tp/ppp offers PAP, CHAP and MS-CHAPv2 only. |
| `2.3` | RADIUS Support for Extensible Authentication Protocol | 0 | walked | RADIUS Support for Extensible Authentication Protocol. Introduces EAP-Message and Message-Authenticator. No capitalised keyword. |
| `2.3.1` | Protocol Overview | 12 | walked | Protocol Overview. Twelve capitalised MUSTs describing an EAP conversation carried over RADIUS. Ze carries no EAP over RADIUS: internal/component/radius/dict.go declares no EAP-Message attribute (type 79), and internal/component/l2tp/ppp has no EAP module, so LCP never negotiates EAP. Ze's IKEv2 EAP (internal/component/ike/eap) terminates EAP locally and imports no RADIUS package. |
| `2.3.2` | Retransmission | 0 | walked | Retransmission. Says the NAS retransmits between peer and NAS and the RADIUS client retransmits between client and server, and that Session-Timeout in an Access-Challenge bounds the wait for an EAP-Response. Indicative prose, no capitalised keyword. |
| `2.3.3` | Fragmentation | 0 | walked | Fragmentation. Says Framed-MTU may be included in an Access-Request carrying an EAP-Message. Permissive, no capitalised keyword. |
| `2.3.4` | Examples | 0 | walked | Examples. Two message-sequence diagrams of an OTP authentication. Illustration only. |
| `2.3.5` | Alternative uses | 1 | walked | Alternative uses. One capitalised MUST binding a RADIUS server that proxies RADIUS-encapsulated EAP to a backend security server. |
| `3` | Packet Format | 0 | walked | Packet Format. One sentence: identical to RFC 2865 and RFC 2866. |
| `4` | Packet Types | 0 | walked | Packet Types. Identical to RFC 2865 and RFC 2866, and points at the Table of Attributes. |
| `5` | Attributes | 3 | walked | Attributes. The attribute type list and the five data types. The section states that 'A summary of the attribute format is the same as in RFC 2865 [1] but is included here for ease of reference', so its three capitalised MUSTs restate RFC 2865 Section 5. |
| `5.1` | Acct-Input-Gigawords | 0 | walked | Acct-Input-Gigawords. Defines type 52 and says the attribute 'indicates how many times the Acct-Input-Octets counter has wrapped around 2^32 over the course of this service being provided, and can only be present in Accounting-Request records where the Acct-Status-Type is set to Stop or Interim-Update'. Indicative prose with no capitalised keyword, which is why the site scan sees nothing here. The five gated Gigawords rows in rfc/short/rfc2869.md were all read from this sentence and its identically worded twin in Section 5.2, so they are declared unsourced here. |
| `5.2` | Acct-Output-Gigawords | 0 | walked | Acct-Output-Gigawords. Defines type 53 in wording identical to Section 5.1 with Acct-Output-Octets substituted. The five gated rows it shares with Section 5.1 are declared unsourced on Section 5.1 rather than twice. |
| `5.3` | Event-Timestamp | 0 | walked | Event-Timestamp. Defines type 55 as the time the event occurred. No obligation. |
| `5.4` | ARAP-Password | 0 | walked | ARAP-Password. Defines type 70. Ze implements no ARAP. |
| `5.5` | ARAP-Features | 0 | walked | ARAP-Features. Defines type 71. Ze implements no ARAP. |
| `5.6` | ARAP-Zone-Access | 0 | walked | ARAP-Zone-Access. Defines type 72. Ze implements no ARAP. |
| `5.7` | ARAP-Security | 0 | walked | ARAP-Security. Defines type 73. Ze implements no ARAP. |
| `5.8` | ARAP-Security-Data | 0 | walked | ARAP-Security-Data. Defines type 74. Ze implements no ARAP. |
| `5.9` | Password-Retry | 0 | walked | Password-Retry. Defines type 75, how many attempts a user is allowed. Ze declares no such attribute constant. |
| `5.10` | Prompt | 0 | walked | Prompt. Defines type 76, whether the NAS echoes the user's response. Ze declares no such attribute constant. |
| `5.11` | Connect-Info | 0 | walked | Connect-Info. Defines type 77, the connect speed the NAS reports. Ze declares no such attribute constant. |
| `5.12` | Configuration-Token | 0 | walked | Configuration-Token. Defines type 78, for a RADIUS proxy. Ze is no RADIUS proxy. |
| `5.13` | EAP-Message | 6 | walked | EAP-Message. Defines type 79 and six capitalised MUSTs over its handling. Ze declares no EAP-Message attribute constant and sends and receives none. |
| `5.14` | Message-Authenticator | 3 | walked | Message-Authenticator. Defines type 80, its two HMAC-MD5 formulas, and three capitalised MUSTs. The third binds a RADIUS client, which is the role Ze plays (internal/component/radius/client.go), and is captured as RFC2869-5.14-1. |
| `5.15` | ARAP-Challenge-Response | 0 | walked | ARAP-Challenge-Response. Defines type 84. Ze implements no ARAP. |
| `5.16` | Acct-Interim-Interval | 1 | walked | Acct-Interim-Interval. Defines type 85 and one capitalised MUST NOT over the value the sender puts in it. The section states the attribute 'can only appear in the Access-Accept message', so the obligation binds the RADIUS server. |
| `5.17` | NAS-Port-Id | 0 | walked | NAS-Port-Id. Defines type 87 as UTF-8 text of length 3 or more naming the physical port. Its one obligation is a SHOULD read from indicative prose, which the site scan does not surface and which rfc/short/rfc2869.md declares as RFC2869-5.17-1. |
| `5.18` | Framed-Pool | 0 | walked | Framed-Pool. Defines type 88, the address pool name. No obligation. |
| `5.19` | Table of Attributes heading and its opening paragraph | 0 | walked | Table of Attributes heading and its opening paragraph. The table body, Note 1 and the notation legend fall outside the numbered-section split and are carried by the derived section '0'. |
| `0` | not stated | 3 | walked | The tail of Section 5.19: the packet-versus-attribute table, Note 1, and the legend defining 0, 0+, 0-1 and 1. The section splitter cannot attribute this block to a numbered heading, so it derives as section '0'. |
| `6` | IANA Considerations | 0 | skipped (iana) | IANA Considerations. Registers the packet type codes, attribute types and attribute values from the RADIUS name spaces per BCP 26. Binds IANA, not a speaker. |
| `7` | Security Considerations | 0 | walked | Security Considerations. One sentence: the attributes other than Message-Authenticator and EAP-Message add nothing beyond RFC 2865. |
| `7.1` | Message-Authenticator Security | 0 | walked | Message-Authenticator Security. Explains why an Access-Request without a User-Password should carry a Message-Authenticator. Lowercase 'should', no capitalised keyword. |
| `7.2` | EAP Security | 0 | walked | EAP Security. Lists the five threats the subsections address. No obligation. |
| `7.2.1` | Separation of EAP server and PPP authenticator | 0 | walked | Separation of EAP server and PPP authenticator. Describes key transport between the EAP server and the PPP authenticator. No capitalised keyword. |
| `7.2.2` | Connection hijacking | 1 | walked | Connection hijacking. One capitalised MUST requiring every EAP/RADIUS packet to be authenticated with the Message-Authenticator attribute. |
| `7.2.3` | Man in the middle attacks | 0 | walked | Man in the middle attacks. States that a compromised RADIUS proxy can modify EAP packets. No countermeasure is directed at a speaker. |
| `7.2.4` | Multiple databases | 0 | walked | Multiple databases. Recommends consolidating the RADIUS and backend security databases. Advice to a deployer, no capitalised keyword. |
| `7.2.5` | Negotiation attacks | 8 | walked | Negotiation attacks. Eight capitalised keywords across the authenticating peer, an EAP-capable NAS, and the RADIUS server or proxy. |
| `8` | not stated | 0 | skipped (references) | References: RFC 2865, RFC 2866, RFC 2284, RFC 2119, RFC 1700, RFC 2868, RFC 2867, RFC 2279, RFC 2104, RFC 2434 and one informative citation. |
| `9` | Acknowledgements | 0 | skipped (acknowledgements) | Acknowledgements. |
| `10` | Chair's Address | 0 | skipped (front-matter) | Chair's Address. A postal and mail address block, matching how rfc/extraction/rfc3765.json skips its Author's Address section. |
| `11` | Authors' Addresses | 0 | skipped (front-matter) | Authors' Addresses. Address blocks only. |
| `12` | Full Copyright Statement | 0 | skipped (front-matter) | Full Copyright Statement. The ISOC boilerplate and its disclaimer. |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `1.1:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The sentence opens 'For example' and names ARAP as an instance of the rule site 1.1:1 states. It adds no obligation site 1.1:1 does not already carry. | For example, a NAS that is unable to offer ARAP service MUST NOT implement the RADIUS attributes for ARAP. |
| `2.2:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the RADIUS server. The sentence sits inside the list of attributes the server returns in an ARAP Access-Accept, and the actor is the party that computes ARAP-Challenge-Response by DES-encrypting the dial-in client's challenge with the user's password. Ze is a RADIUS client only (internal/component/radius/client.go opens a UDP socket to send requests and match replies) and implements no ARAP. | If the user's password is greater than 8 octets in length, an Access-Reject MUST be sent instead. |
| `2.3.1:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds a NAS that has negotiated EAP within PPP LCP. Ze never plays that role: internal/component/l2tp/ppp carries pap.go, chap.go and mschapv2.go and no EAP module, so LCP never offers EAP, and internal/component/radius/dict.go declares no EAP-Message attribute (type 79). Ze's IKEv2 EAP (internal/component/ike/eap) is a separate protocol terminated locally and imports no RADIUS package. | Once EAP has been negotiated, the NAS MUST send an EAP-Request/Identity message to the authenticating peer, unless identity is determined via some other means such as Called-Station-Id or Calling-Station-Id. |
| `2.3.1:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds a NAS that has negotiated EAP within PPP LCP. Ze never plays that role: internal/component/l2tp/ppp carries pap.go, chap.go and mschapv2.go and no EAP module, so LCP never offers EAP, and internal/component/radius/dict.go declares no EAP-Message attribute (type 79). Ze's IKEv2 EAP (internal/component/ike/eap) is a separate protocol terminated locally and imports no RADIUS package. | In order to permit non-EAP aware RADIUS proxies to forward the Access-Request packet, if the NAS sends the EAP-Request/Identity, the NAS MUST copy the contents of the EAP-Response/Identity into the User-Name attribute and MUST include the EAP-Response/Identity in the User-Name attribute in every subsequent Access-Request. |
| `2.3.1:3` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds a NAS that has negotiated EAP within PPP LCP. Ze never plays that role: internal/component/l2tp/ppp carries pap.go, chap.go and mschapv2.go and no EAP module, so LCP never offers EAP, and internal/component/radius/dict.go declares no EAP-Message attribute (type 79). Ze's IKEv2 EAP (internal/component/ike/eap) is a separate protocol terminated locally and imports no RADIUS package. | NAS-Port or NAS-Port-Id SHOULD be included in the attributes issued by the NAS in the Access-Request packet, and either NAS-Identifier or NAS-IP- Address MUST be included. |
| `2.3.1:4` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the RADIUS server. Ze runs no RADIUS server: internal/component/radius/client.go binds a socket only to receive replies to its own requests, and the one listener Ze does run (internal/component/l2tp/plugins/authradius/coa.go) accepts CoA-Request and Disconnect-Request under RFC 5176, never an Access-Request. | In order to permit forwarding of the Access-Reply by EAP-unaware proxies, if a User-Name attribute was included in an Access-Request, the RADIUS Server MUST include the User-Name attribute in subsequent Access-Accept packets. |
| `2.3.1:5` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds a NAS that has negotiated EAP within PPP LCP. Ze never plays that role: internal/component/l2tp/ppp carries pap.go, chap.go and mschapv2.go and no EAP module, so LCP never offers EAP, and internal/component/radius/dict.go declares no EAP-Message attribute (type 79). Ze's IKEv2 EAP (internal/component/ike/eap) is a separate protocol terminated locally and imports no RADIUS package. | If identity is determined via another means such as Called-Station-Id or Calling-Station-Id, the NAS MUST include these identifying attributes in every Access-Request. |
| `2.3.1:6` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the RADIUS server. Ze runs no RADIUS server: internal/component/radius/client.go binds a socket only to receive replies to its own requests, and the one listener Ze does run (internal/component/l2tp/plugins/authradius/coa.go) accepts CoA-Request and Disconnect-Request under RFC 5176, never an Access-Request. | If the RADIUS server supports EAP, it MUST respond with an Access- Challenge packet containing an EAP-Message attribute. |
| `2.3.1:7` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the RADIUS server. Ze runs no RADIUS server: internal/component/radius/client.go binds a socket only to receive replies to its own requests, and the one listener Ze does run (internal/component/l2tp/plugins/authradius/coa.go) accepts CoA-Request and Disconnect-Request under RFC 5176, never an Access-Request. | If the RADIUS server does not support EAP, it MUST respond with an Access-Reject. |
| `2.3.1:8` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds a NAS that has negotiated EAP within PPP LCP. Ze never plays that role: internal/component/l2tp/ppp carries pap.go, chap.go and mschapv2.go and no EAP module, so LCP never offers EAP, and internal/component/radius/dict.go declares no EAP-Message attribute (type 79). Ze's IKEv2 EAP (internal/component/ike/eap) is a separate protocol terminated locally and imports no RADIUS package. | Reception of a RADIUS Access-Reject packet, with or without an EAP- Message attribute encapsulating EAP-Failure, MUST result in the NAS issuing an LCP Terminate Request to the authenticating peer. |
| `2.3.1:9` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the RADIUS server. Ze runs no RADIUS server: internal/component/radius/client.go binds a socket only to receive replies to its own requests, and the one listener Ze does run (internal/component/l2tp/plugins/authradius/coa.go) accepts CoA-Request and Disconnect-Request under RFC 5176, never an Access-Request. | The RADIUS Access-Accept/EAP-Message/EAP-Success packet MUST contain all of the expected attributes which are currently returned in an Access-Accept packet. |
| `2.3.1:10` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the RADIUS server. Ze runs no RADIUS server: internal/component/radius/client.go binds a socket only to receive replies to its own requests, and the one listener Ze does run (internal/component/l2tp/plugins/authradius/coa.go) accepts CoA-Request and Disconnect-Request under RFC 5176, never an Access-Request. | If the domain is determined based on the user's identity, the local RADIUS Server MUST respond with a RADIUS Access-Challenge/EAP-Identity packet. |
| `2.3.1:11` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the RADIUS server. Ze runs no RADIUS server: internal/component/radius/client.go binds a socket only to receive replies to its own requests, and the one listener Ze does run (internal/component/l2tp/plugins/authradius/coa.go) accepts CoA-Request and Disconnect-Request under RFC 5176, never an Access-Request. | The response from the authenticating peer MUST be proxied to the final authentication server. |
| `2.3.1:12` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds a NAS that has negotiated EAP within PPP LCP. Ze never plays that role: internal/component/l2tp/ppp carries pap.go, chap.go and mschapv2.go and no EAP module, so LCP never offers EAP, and internal/component/radius/dict.go declares no EAP-Message attribute (type 79). Ze's IKEv2 EAP (internal/component/ike/eap) is a separate protocol terminated locally and imports no RADIUS package. | On receiving an Access-Reject, the NAS MUST send an LCP Terminate Request to the authenticating peer, and disconnect. |
| `2.3.5:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the RADIUS server. Ze runs no RADIUS server: internal/component/radius/client.go binds a socket only to receive replies to its own requests, and the one listener Ze does run (internal/component/l2tp/plugins/authradius/coa.go) accepts CoA-Request and Disconnect-Request under RFC 5176, never an Access-Request. | This means that the RADIUS server MUST add these attributes prior to sending an Access-Accept/EAP-Success message to the NAS. |
| `5:1` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | RFC 2865 Section 5, 'servers and clients MUST be able to deal with embedded nulls'. Section 5 states 'A summary of the attribute format is the same as in RFC 2865 [1] but is included here for ease of reference', and the sentence appears verbatim in RFC 2865 Section 5. The obligation belongs to RFC 2865, which carries its own summary at rfc/short/rfc2865.md. | Servers and servers and clients MUST be able to deal with embedded nulls. |
| `5:2` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | RFC 2865 Section 5, 'Text of length zero (0) MUST NOT be sent; omit the entire attribute instead'. Section 5 states 'A summary of the attribute format is the same as in RFC 2865 [1] but is included here for ease of reference', and the sentence appears verbatim in RFC 2865 Section 5. The obligation belongs to RFC 2865, which carries its own summary at rfc/short/rfc2865.md. | Text of length zero (0) MUST NOT be sent; omit the entire attribute instead. string 1-253 octets containing binary data (values 0 through 255 decimal, inclusive). |
| `5:3` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | RFC 2865 Section 5, 'Strings of length zero (0) MUST NOT be sent; omit the entire attribute instead'. Section 5 states 'A summary of the attribute format is the same as in RFC 2865 [1] but is included here for ease of reference', and the sentence appears verbatim in RFC 2865 Section 5. The obligation belongs to RFC 2865, which carries its own summary at rfc/short/rfc2865.md. | Strings of length zero (0) MUST NOT be sent; omit the entire attribute instead. |
| `5.13:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds a RADIUS speaker that puts EAP-Message attributes in a packet. internal/component/radius/dict.go declares no EAP-Message attribute (type 79) and no encoder emits one, so Ze never builds a packet the ordering rule can govern. | If multiple EAP- Messages are contained within an Access-Request or Access- Challenge packet, they MUST be in order and they MUST be consecutive attributes in the Access-Request or Access-Challenge packet. |
| `5.13:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the sender of a packet carrying an EAP-Message attribute. Ze sends none: internal/component/radius/dict.go declares no type 79. | Therefore the Message-Authenticator attribute MUST be used to protect all Access-Request, Access-Challenge, Access-Accept, and Access-Reject packets containing an EAP-Message attribute. |
| `5.13:3` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the RADIUS server. Ze runs no RADIUS server: internal/component/radius/client.go binds a socket only to receive replies to its own requests, and the one listener Ze does run (internal/component/l2tp/plugins/authradius/coa.go) accepts CoA-Request and Disconnect-Request under RFC 5176, never an Access-Request. | A RADIUS Server supporting EAP-Message MUST calculate the correct value of the Message-Authenticator and silently discard the packet if it does not match the value sent. |
| `5.13:4` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the RADIUS server. Ze runs no RADIUS server: internal/component/radius/client.go binds a socket only to receive replies to its own requests, and the one listener Ze does run (internal/component/l2tp/plugins/authradius/coa.go) accepts CoA-Request and Disconnect-Request under RFC 5176, never an Access-Request. | A RADIUS Server not supporting EAP-Message MUST return an Access- Reject if it receives an Access-Request containing an EAP-Message attribute. |
| `5.13:5` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the RADIUS server. Ze runs no RADIUS server: internal/component/radius/client.go binds a socket only to receive replies to its own requests, and the one listener Ze does run (internal/component/l2tp/plugins/authradius/coa.go) accepts CoA-Request and Disconnect-Request under RFC 5176, never an Access-Request. | A RADIUS Server receiving an EAP-Message attribute that it does not understand MUST return an Access-Reject. |
| `5.13:6` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds 'A NAS supporting EAP-Message'. Ze supports none: internal/component/radius/dict.go declares no type 79, so no Access-Challenge, Access-Accept or Access-Reject Ze reads can carry the attribute this rule conditions on. The unconditional client-side rule in the same section, site 5.14:3, is the one that binds Ze, and it is mapped. | A NAS supporting EAP-Message MUST calculate the correct value of the Message-Authenticator and silently discard the packet if it does not match the value sent. |
| `5.14:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the sender of a packet that includes an EAP-Message attribute. Ze includes none in the Access-Request it builds (internal/component/l2tp/plugins/authradius/handler.go, buildAuthAttrs) and declares no type 79 in internal/component/radius/dict.go, so the condition never holds for a packet Ze sends. Ze does add a Message-Authenticator when reading one: the receive-side obligation is site 5.14:3. | It MUST be used in any Access-Request, Access-Accept, Access-Reject or Access- Challenge that includes an EAP-Message attribute. |
| `5.14:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the RADIUS server. Ze runs no RADIUS server: internal/component/radius/client.go binds a socket only to receive replies to its own requests, and the one listener Ze does run (internal/component/l2tp/plugins/authradius/coa.go) accepts CoA-Request and Disconnect-Request under RFC 5176, never an Access-Request. | A RADIUS Server receiving an Access-Request with a Message- Authenticator Attribute present MUST calculate the correct value of the Message-Authenticator and silently discard the packet if it does not match the value sent. |
| `5.16:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the RADIUS server. Section 5.16 states the attribute 'can only appear in the Access-Accept message', and Section 5.19's table gives Acct-Interim-Interval 0-1 instances in an Accept and 0 in a Request, so the value the MUST NOT constrains is the one the server writes. Ze never sends the attribute; it reads it (internal/component/l2tp/plugins/authradius/extract.go) and clamps a received value into [60, 3600] (clampAcctInterval, acct.go), which is a defence against a non-conformant server rather than the obligation this sentence states. | The value MUST NOT be smaller than 60. |
| `0:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the sender of any packet type carrying an EAP-Message attribute. Ze sends no EAP-Message: internal/component/radius/dict.go declares no type 79. | If any packet type contains an EAP-Message attribute it MUST also contain a Message-Authenticator. |
| `0:3` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | The legend of the Section 5.19 table, quoted from the four lines that define what the cells 0, 0+, 0-1 and 1 mean. The keywords describe the notation, not a speaker's behaviour; the obligations the table expresses are carried by the individual attribute sections and by Note 1. | 0 This attribute MUST NOT be present 0+ Zero or more instances of this attribute MAY be present. 0-1 Zero or one instance of this attribute MAY be present. 1 Exactly one instance of this attribute MUST be present. |
| `7.2.2:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds a speaker of EAP over RADIUS: the sentence's subject is 'all EAP/RADIUS packets'. Ze exchanges none, because internal/component/radius/dict.go declares no EAP-Message attribute and internal/component/l2tp/ppp never negotiates EAP. | In order to provide for authentication of all packets in the EAP exchange, all EAP/RADIUS packets MUST be authenticated using the Message-Authenticator attribute, as described previously. |
| `7.2.5:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the authenticating peer, the dial-in client at the far end of the PPP link. Ze is the NAS and LNS side (internal/component/l2tp), never the dial-in client. | Should the NAS not be able to negotiate EAP, or should the EAP-Request sent by the NAS be of a different EAP type than what is expected, the authenticating peer MUST disconnect. |
| `7.2.5:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the authenticating peer, the dial-in client at the far end of the PPP link. Ze is the NAS and LNS side (internal/component/l2tp), never the dial-in client. | An authenticating peer expecting EAP to be negotiated for a session MUST NOT negotiate CHAP or PAP. |
| `7.2.5:3` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds a NAS some of whose users are required to authenticate with EAP. Ze offers no EAP at all (internal/component/l2tp/ppp has no EAP module), so it has no such users. The first 'MUST' in the sentence is indicative ('if any users of the NAS MUST do EAP') and states the condition rather than an obligation. | In such cases, if any users of the NAS MUST do EAP, then the NAS MUST attempt to negotiate EAP for every call. |
| `7.2.5:4` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the RADIUS server. Ze runs no RADIUS server: internal/component/radius/client.go binds a socket only to receive replies to its own requests, and the one listener Ze does run (internal/component/l2tp/plugins/authradius/coa.go) accepts CoA-Request and Disconnect-Request under RFC 5176, never an Access-Request. | However, if CHAP has been negotiated but EAP is required, the RADIUS server MUST respond with an Access-Reject, rather than an Access- Challenge/EAP-Message/EAP-Request packet. |
| `7.2.5:5` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the authenticating peer, the dial-in client at the far end of the PPP link. Ze is the NAS and LNS side (internal/component/l2tp), never the dial-in client. | The authenticating peer MUST refuse to renegotiate authentication, even if the renegotiation is from CHAP to EAP. |
| `7.2.5:6` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the RADIUS server. Ze runs no RADIUS server: internal/component/radius/client.go binds a socket only to receive replies to its own requests, and the one listener Ze does run (internal/component/l2tp/plugins/authradius/coa.go) accepts CoA-Request and Disconnect-Request under RFC 5176, never an Access-Request. | If EAP is negotiated but is not supported by the RADIUS proxy or server, then the server or proxy MUST respond with an Access-Reject. |
| `7.2.5:7` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds a NAS that has negotiated EAP within PPP LCP. Ze never plays that role: internal/component/l2tp/ppp carries pap.go, chap.go and mschapv2.go and no EAP module, so LCP never offers EAP, and internal/component/radius/dict.go declares no EAP-Message attribute (type 79). Ze's IKEv2 EAP (internal/component/ike/eap) is a separate protocol terminated locally and imports no RADIUS package. | In these cases, the NAS MUST send an LCP-Terminate and disconnect the user. |
| `7.2.5:8` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the authenticating peer, the dial-in client at the far end of the PPP link. Ze is the NAS and LNS side (internal/component/l2tp), never the dial-in client. | An EAP-capable authenticating peer MUST refuse to renegotiate the authentication protocol if EAP had initially been negotiated. |

## Superseded

No document obsoletes RFC 2869, so its obligations are stated where they were written.
