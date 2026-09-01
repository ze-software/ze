# RFC 2866 - RADIUS Accounting

Supported for subscriber access. Every requirement this repository extracted from RFC 2866, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 100.0% | 16 of 16 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 16 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 16 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 0.0% | 0 of 16 binding obligations | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 0.0% | 0 of 35 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 16 | of 18 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 16 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

The 4 shares marked as a part above are the whole of the 16 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 18 |
| Gated MUST-level | 16 |
| Obligations that bind Ze | 16 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 35 |
| Tagged units | 35 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc2866.md` |
| Requirement shard | `rfc/requirements/rfc2866.md` |
| RFC text | `rfc/full/rfc2866.txt` |

## Enrolment

Enrolled: ze is a RADIUS accounting client (NAS); the five gated MUST requirements the checklist held before 2026-08-31 are tested with positive+negative pairs. RFC2866-3-1 (MUST NOT tear down sessions on accounting failure): TestRFC2866AcctFailureKeepsSession positive (a failed Accounting-Start against an unreachable server leaves the session tracked), TestRFC2866SessionTeardownIndependentOfAccounting negative (teardown is driven only by the session-down event; producer authradius/acct.go:254-264 sendAcctPacket only logs on error, onSessionDown separate at acct.go:143-162). RFC2866-3-2 (authenticator = MD5(Code+ID+Length+16 zero octets+Attributes+Secret)): TestRFC2866AccountingRequestAuthFormula positive (equals an independent MD5 reference; producer radius/packet.go:252 AccountingRequestAuth, applied radius/client.go:127-132), TestRFC2866AccountingRequestAuthRejectsTampering negative (authenticator changes when secret/attribute/Code changes). RFC2866-3-3 (retransmit reuses Identifier): TestRFC2866AccountingRetransmitSameIdentifier positive (retransmit carries the same Identifier; producer radius/client.go:147-193 re-sends the pre-encoded buf), TestRFC2866AccountingDistinctRequestsDifferIdentifier negative. RFC2866-5-1 (Acct-Status-Type present Start/Stop/Interim): TestRFC2866AcctStatusTypePresent positive (each lifecycle event yields exactly one attribute valued 1/2/3; producer authradius/acct.go:196 buildAcctPacket), TestRFC2866AcctStatusTypeNeverOmitted negative. RFC2866-5.5-1 (Acct-Session-Id unique across the NAS): TestRFC2866AcctSessionIDUnique positive (1600 concurrent ids all distinct; producer authradius/acct.go:77-84 genSessionID monotonic counter under lock), TestRFC2866AcctSessionIDNoCollisionOnReusedKey negative. No SHOULD/MAY requirements are gated. The extraction walk of 2026-08-31 (rfc/extraction/rfc2866.json) added ten obligations the checklist never declared, each with a positive+negative pair: RFC2866-3-4 and RFC2866-3-5 (octets outside the Length are padding, a datagram shorter than its Length is discarded; producers radius/packet.go Decode and radius/client.go readLoop), RFC2866-4.1-2 (User-Password, CHAP-Password, Reply-Message and State never present), RFC2866-4.1-3 (either NAS-IP-Address or NAS-Identifier present; producer authradius/nasidentity.go appendNASIdentity, which fixed a config setting neither leaf), RFC2866-4.1-4 (a new request takes a new Identifier; producer radius/client.go SendToServers), RFC2866-4.2-1 (the Response Authenticator of an Accounting-Response; producer radius/client.go dispatchResponse), RFC2866-5-2 (embedded nulls survive an attribute value), RFC2866-5-3 (text of length zero omitted; producer authradius/acct.go buildAcctPacket, which fixed an empty User-Name reaching the wire), RFC2866-5.5-2 and RFC2866-5.5-3 (every record carries an Acct-Session-Id, and the records of one session carry the same one).

## What the public ledger says

**Status:** Supported for subscriber access

**What the ledger says is covered:**

Start, Stop, and Interim-Update accounting records.

**What the ledger says remains:**

Admin/operator RADIUS accounting is not wired; the admin backend is authentication-only.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 16 | one part of the gated population |
| Annotated instead of tested | 0 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **16** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (16):** [`RFC2866-3-1`](#rfc2866-3-1), [`RFC2866-3-2`](#rfc2866-3-2), [`RFC2866-5.5-1`](#rfc2866-5.5-1), [`RFC2866-4.1-1`](#rfc2866-4.1-1), [`RFC2866-5-1`](#rfc2866-5-1), [`RFC2866-3-3`](#rfc2866-3-3), [`RFC2866-3-4`](#rfc2866-3-4), [`RFC2866-3-5`](#rfc2866-3-5), [`RFC2866-4.1-2`](#rfc2866-4.1-2), [`RFC2866-4.1-3`](#rfc2866-4.1-3), [`RFC2866-4.1-4`](#rfc2866-4.1-4), [`RFC2866-4.2-1`](#rfc2866-4.2-1), [`RFC2866-5-2`](#rfc2866-5-2), [`RFC2866-5-3`](#rfc2866-5-3), [`RFC2866-5.5-2`](#rfc2866-5.5-2), [`RFC2866-5.5-3`](#rfc2866-5.5-3)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC2866-3-1` | Accounting failures MUST NOT tear down user sessions (§3) | MUST NOT | 3 - Packet Format | **positive:** `unit/verify` [`TestRFC2866AcctFailureKeepsSession`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc2866_accounting_test.go#L156). **negative:** `unit/verify` [`TestRFC2866SessionTeardownIndependentOfAccounting`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc2866_accounting_test.go#L192) |
| `RFC2866-3-2` | Accounting-Request authenticator MUST be computed as MD5(Code+ID+Length+16_zero_octets+Attributes+Secret) (§3) | MUST | 3 - Packet Format | **positive:** `unit/verify` [`TestRFC2866AccountingRequestAuthFormula`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2866_accounting_test.go#L84). **negative:** `unit/verify` [`TestRFC2866AccountingRequestAuthRejectsTampering`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2866_accounting_test.go#L127) |
| `RFC2866-5.5-1` | Acct-Session-Id MUST be unique across all active sessions on the NAS (§5.5) | MUST | 5.5 - Acct-Session-Id | **positive:** `unit/verify` [`TestRFC2866AcctSessionIDUnique`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc2866_accounting_test.go#L112). **negative:** `unit/verify` [`TestRFC2866AcctSessionIDNoCollisionOnReusedKey`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc2866_accounting_test.go#L144) |
| `RFC2866-4.1-1` | A Framed-IP-Address included in an Accounting-Request MUST contain the IP address of the user, and where the Access-Accept used a special value telling the NAS to assign or negotiate an address, MUST contain the address actually assigned or negotiated (§4.1) | MUST | 4.1 - Accounting-Request | **positive:** `unit/verify` [`TestAcctFramedIPAddressPresent`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/acct_address_test.go#L32). **positive:** `unit/verify` [`TestSessionEventDrivesAddressAndPortID`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/acct_address_test.go#L127). **negative:** `unit/verify` [`TestAcctFramedIPAddressIsSubscriberNotNAS`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/acct_address_test.go#L56). **negative:** `unit/verify` [`TestAcctFramedIPAddressOmittedWhenNotIPv4`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/acct_address_test.go#L79). **positive:** `functional/verify` [`radius-acct-wire.ci`](https://github.com/ze-software/ze/blob/main/test/l2tp/radius-acct-wire.ci#L24) |
| `RFC2866-5-1` | Acct-Status-Type attribute MUST be included in Accounting-Request to indicate Start (1), Stop (2), or Interim-Update (3) (§5) | MUST | 5 - Attributes | **positive:** `unit/verify` [`TestRFC2866AcctStatusTypePresent`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc2866_accounting_test.go#L57). **negative:** `unit/verify` [`TestRFC2866AcctStatusTypeNeverOmitted`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc2866_accounting_test.go#L88) |
| `RFC2866-3-3` | Same retransmit rules as RFC 2865: retransmitted request MUST use the same Identifier (§3) | MUST | 3 - Packet Format | **positive:** `unit/verify` [`TestRFC2866AccountingRetransmitSameIdentifier`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2866_accounting_test.go#L167). **negative:** `unit/verify` [`TestRFC2866AccountingDistinctRequestsDifferIdentifier`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2866_accounting_test.go#L210) |
| `RFC2866-3-4` | Octets outside the range of the Length field MUST be treated as padding and ignored on reception (§3) | MUST | 3 - Packet Format | **positive:** `unit/verify` [`TestRFC2866LengthPaddingIgnoredOnReception`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2866_packet_test.go#L112). **negative:** `unit/verify` [`TestRFC2866LengthPaddingBoundaryIsTheLengthField`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2866_packet_test.go#L133) |
| `RFC2866-3-5` | A packet shorter than its Length field indicates MUST be silently discarded (§3) | MUST | 3 - Packet Format | **positive:** `unit/verify` [`TestRFC2866ShortPacketSilentlyDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2866_packet_test.go#L164). **negative:** `unit/verify` [`TestRFC2866HonestLengthIsAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2866_packet_test.go#L181) |
| `RFC2866-4.1-2` | User-Password, CHAP-Password, Reply-Message and State MUST NOT be present in an Accounting-Request (§4.1) | MUST NOT | 4.1 - Accounting-Request | **positive:** `unit/verify` [`TestRFC2866AcctForbiddenAttributesAbsent`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc2866_request_contents_test.go#L37). **negative:** `unit/verify` [`TestRFC2866AcctForbiddenAttributesDoNotEmptyTheRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc2866_request_contents_test.go#L67) |
| `RFC2866-4.1-3` | Either NAS-IP-Address or NAS-Identifier MUST be present in an Accounting-Request (§4.1, restated at §5.13 Note 1) | MUST | 4.1 - Accounting-Request | **positive:** `unit/verify` [`TestRFC2866AcctNASIdentityAlwaysPresent`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc2866_request_contents_test.go#L90). **negative:** `unit/verify` [`TestRFC2866AcctNASIdentityFallbackIsNarrow`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc2866_request_contents_test.go#L126) |
| `RFC2866-4.1-4` | The Identifier MUST change whenever the content of the Attributes field changes, and whenever a valid reply has been received for a previous request (§4.1) | MUST | 4.1 - Accounting-Request | **positive:** `unit/verify` [`TestRFC2866IdentifierChangesForANewRequest`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2866_packet_test.go#L193). **negative:** `unit/verify` [`TestRFC2866IdentifierCounterCoversTheWholeSpace`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2866_packet_test.go#L234) |
| `RFC2866-4.2-1` | The Response Authenticator of an Accounting-Response MUST contain the correct response for the pending Accounting-Request (§4.2) | MUST | 4.2 - Accounting-Response | **positive:** `unit/verify` [`TestRFC2866AccountingResponseAuthenticatorAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2866_packet_test.go#L260). **negative:** `unit/verify` [`TestRFC2866AccountingResponseAuthenticatorForgeryDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2866_packet_test.go#L272) |
| `RFC2866-5-2` | Servers and clients MUST be able to deal with embedded nulls in an attribute value (§5) | MUST | 5 - Attributes | **positive:** `unit/verify` [`TestRFC2866EmbeddedNullsSurviveTheWire`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2866_packet_test.go#L288). **negative:** `unit/verify` [`TestRFC2866AllNullValueKeepsItsLength`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2866_packet_test.go#L313) |
| `RFC2866-5-3` | Text of length zero MUST NOT be sent; the entire attribute is omitted instead (§5) | MUST NOT | 5 - Attributes | **positive:** `unit/verify` [`TestRFC2866AcctZeroLengthTextOmitted`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc2866_request_contents_test.go#L148). **negative:** `unit/verify` [`TestRFC2866AcctNonEmptyTextIsSent`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc2866_request_contents_test.go#L168) |
| `RFC2866-5.5-2` | The start and stop records for a given session MUST have the same Acct-Session-Id (§5.5) | MUST | 5.5 - Acct-Session-Id | **positive:** `unit/verify` [`TestRFC2866AcctSessionIDSameAcrossRecords`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc2866_request_contents_test.go#L181). **negative:** `unit/verify` [`TestRFC2866AcctSessionIDDiffersBetweenSessions`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc2866_request_contents_test.go#L200) |
| `RFC2866-5.5-3` | An Accounting-Request packet MUST have an Acct-Session-Id (§5.5) | MUST | 5.5 - Acct-Session-Id | **positive:** `unit/verify` [`TestRFC2866AcctSessionIDPresentOnEveryRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc2866_request_contents_test.go#L216). **negative:** `unit/verify` [`TestRFC2866AcctSessionIDNeverEmpty`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc2866_request_contents_test.go#L236) |
| `RFC2866-x-1` | NAS SHOULD use exponential backoff between retransmits (per RFC 2865 §2.5) | SHOULD | x | **positive:** no positive test. **negative:** no negative test |
| `RFC2866-x-2` | Interim-Update interval MAY be locally configured (Implementation Constraints) | MAY | x | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

RFC 2866 declares no gap, and every gated MUST it carries has a test bound to it.

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC2866-3-1`](#rfc2866-3-1)

Accounting failures MUST NOT tear down user sessions (§3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2866SessionTeardownIndependentOfAccounting`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc2866_accounting_test.go#L192) | unit/verify | unproven |
| positive | [`TestRFC2866AcctFailureKeepsSession`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc2866_accounting_test.go#L156) | unit/verify | unproven |

### [`RFC2866-3-2`](#rfc2866-3-2)

Accounting-Request authenticator MUST be computed as MD5(Code+ID+Length+16_zero_octets+Attributes+Secret) (§3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2866AccountingRequestAuthRejectsTampering`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2866_accounting_test.go#L127) | unit/verify | unproven |
| positive | [`TestRFC2866AccountingRequestAuthFormula`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2866_accounting_test.go#L84) | unit/verify | unproven |

### [`RFC2866-5.5-1`](#rfc2866-5.5-1)

Acct-Session-Id MUST be unique across all active sessions on the NAS (§5.5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2866AcctSessionIDNoCollisionOnReusedKey`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc2866_accounting_test.go#L144) | unit/verify | unproven |
| positive | [`TestRFC2866AcctSessionIDUnique`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc2866_accounting_test.go#L112) | unit/verify | unproven |

### [`RFC2866-4.1-1`](#rfc2866-4.1-1)

A Framed-IP-Address included in an Accounting-Request MUST contain the IP address of the user, and where the Access-Accept used a special value telling the NAS to assign or negotiate an address, MUST contain the address actually assigned or negotiated (§4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestAcctFramedIPAddressIsSubscriberNotNAS`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/acct_address_test.go#L56) | unit/verify | unproven |
| negative | [`TestAcctFramedIPAddressOmittedWhenNotIPv4`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/acct_address_test.go#L79) | unit/verify | unproven |
| positive | [`TestAcctFramedIPAddressPresent`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/acct_address_test.go#L32) | unit/verify | unproven |
| positive | [`TestSessionEventDrivesAddressAndPortID`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/acct_address_test.go#L127) | unit/verify | unproven |
| positive | [`radius-acct-wire.ci`](https://github.com/ze-software/ze/blob/main/test/l2tp/radius-acct-wire.ci#L24) | functional/verify | unproven |

### [`RFC2866-5-1`](#rfc2866-5-1)

Acct-Status-Type attribute MUST be included in Accounting-Request to indicate Start (1), Stop (2), or Interim-Update (3) (§5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2866AcctStatusTypeNeverOmitted`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc2866_accounting_test.go#L88) | unit/verify | unproven |
| positive | [`TestRFC2866AcctStatusTypePresent`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc2866_accounting_test.go#L57) | unit/verify | unproven |

### [`RFC2866-3-3`](#rfc2866-3-3)

Same retransmit rules as RFC 2865: retransmitted request MUST use the same Identifier (§3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2866AccountingDistinctRequestsDifferIdentifier`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2866_accounting_test.go#L210) | unit/verify | unproven |
| positive | [`TestRFC2866AccountingRetransmitSameIdentifier`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2866_accounting_test.go#L167) | unit/verify | unproven |

### [`RFC2866-3-4`](#rfc2866-3-4)

Octets outside the range of the Length field MUST be treated as padding and ignored on reception (§3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2866LengthPaddingBoundaryIsTheLengthField`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2866_packet_test.go#L133) | unit/verify | unproven |
| positive | [`TestRFC2866LengthPaddingIgnoredOnReception`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2866_packet_test.go#L112) | unit/verify | unproven |

### [`RFC2866-3-5`](#rfc2866-3-5)

A packet shorter than its Length field indicates MUST be silently discarded (§3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2866HonestLengthIsAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2866_packet_test.go#L181) | unit/verify | unproven |
| positive | [`TestRFC2866ShortPacketSilentlyDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2866_packet_test.go#L164) | unit/verify | unproven |

### [`RFC2866-4.1-2`](#rfc2866-4.1-2)

User-Password, CHAP-Password, Reply-Message and State MUST NOT be present in an Accounting-Request (§4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2866AcctForbiddenAttributesDoNotEmptyTheRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc2866_request_contents_test.go#L67) | unit/verify | unproven |
| positive | [`TestRFC2866AcctForbiddenAttributesAbsent`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc2866_request_contents_test.go#L37) | unit/verify | unproven |

### [`RFC2866-4.1-3`](#rfc2866-4.1-3)

Either NAS-IP-Address or NAS-Identifier MUST be present in an Accounting-Request (§4.1, restated at §5.13 Note 1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2866AcctNASIdentityFallbackIsNarrow`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc2866_request_contents_test.go#L126) | unit/verify | unproven |
| positive | [`TestRFC2866AcctNASIdentityAlwaysPresent`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc2866_request_contents_test.go#L90) | unit/verify | unproven |

### [`RFC2866-4.1-4`](#rfc2866-4.1-4)

The Identifier MUST change whenever the content of the Attributes field changes, and whenever a valid reply has been received for a previous request (§4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2866IdentifierCounterCoversTheWholeSpace`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2866_packet_test.go#L234) | unit/verify | unproven |
| positive | [`TestRFC2866IdentifierChangesForANewRequest`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2866_packet_test.go#L193) | unit/verify | unproven |

### [`RFC2866-4.2-1`](#rfc2866-4.2-1)

The Response Authenticator of an Accounting-Response MUST contain the correct response for the pending Accounting-Request (§4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2866AccountingResponseAuthenticatorForgeryDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2866_packet_test.go#L272) | unit/verify | unproven |
| positive | [`TestRFC2866AccountingResponseAuthenticatorAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2866_packet_test.go#L260) | unit/verify | unproven |

### [`RFC2866-5-2`](#rfc2866-5-2)

Servers and clients MUST be able to deal with embedded nulls in an attribute value (§5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2866AllNullValueKeepsItsLength`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2866_packet_test.go#L313) | unit/verify | unproven |
| positive | [`TestRFC2866EmbeddedNullsSurviveTheWire`](https://github.com/ze-software/ze/blob/main/internal/component/radius/rfc2866_packet_test.go#L288) | unit/verify | unproven |

### [`RFC2866-5-3`](#rfc2866-5-3)

Text of length zero MUST NOT be sent; the entire attribute is omitted instead (§5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2866AcctNonEmptyTextIsSent`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc2866_request_contents_test.go#L168) | unit/verify | unproven |
| positive | [`TestRFC2866AcctZeroLengthTextOmitted`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc2866_request_contents_test.go#L148) | unit/verify | unproven |

### [`RFC2866-5.5-2`](#rfc2866-5.5-2)

The start and stop records for a given session MUST have the same Acct-Session-Id (§5.5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2866AcctSessionIDDiffersBetweenSessions`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc2866_request_contents_test.go#L200) | unit/verify | unproven |
| positive | [`TestRFC2866AcctSessionIDSameAcrossRecords`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc2866_request_contents_test.go#L181) | unit/verify | unproven |

### [`RFC2866-5.5-3`](#rfc2866-5.5-3)

An Accounting-Request packet MUST have an Acct-Session-Id (§5.5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2866AcctSessionIDNeverEmpty`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc2866_request_contents_test.go#L236) | unit/verify | unproven |
| positive | [`TestRFC2866AcctSessionIDPresentOnEveryRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc2866_request_contents_test.go#L216) | unit/verify | unproven |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | ze-work agent, spec-rfcgate-6, rfc2866 |
| Signed off | 2026-08-31 |
| Register | rfc2119 |
| Source | rfc/full/rfc2866.txt |
| Source fingerprint | 928686054877a829 |
| Record | rfc/extraction/rfc2866.json |
| Mapped sentences | 14 |
| Declined as scope | 8 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | skipped (front-matter) | Title block, Status of this Memo, copyright notice, Abstract, the Implementation Note on UDP port 1646 against 1813, and the table of contents. The Implementation Note records deployment history and binds no speaker. |
| `1` | Introduction | 0 | walked | Introduction. Names the problem RADIUS Accounting answers, and lists the key features: the client/server model, the shared secret that is never sent over the network, and the variable-length Attribute-Length-Value 3-tuples. No obligation. |
| `1.1` | Specification of Requirements | 0 | walked | Specification of Requirements. The RFC 2119 key-words paragraph, plus one sentence of its own: 'These key words mean the same thing whether capitalized or not.' That sentence is why this walk reads a lowercase modal as normative wherever it binds a speaker, and it is why site 2.1:1 counts although the source writes 'MUST not'. |
| `1.2` | Terminology | 0 | walked | Terminology. Defines service, session and 'silently discard'. The 'silently discard' entry carries two SHOULD clauses, on logging the error and on counting the event; neither is gated and neither is declared in rfc/short/rfc2866.md. |
| `2` | Operation | 1 | walked | Operation. Describes the Start and Stop exchange, recommends that the client retry with some form of backoff and can fall back to an alternate server, and states that retry and fallback algorithms are not specified in detail in this document. One capitalised site, binding the accounting server. RFC2866-3-1 is declared unsourced here: RFC 2866 nowhere states that an accounting failure must not tear a session down. The summary read it from this section's model, in which accounting is an exchange beside the session rather than a step of it, and ze meets it (sendAcctPacket logs a failed record and returns; onSessionDown is driven by the session-down event alone). |
| `2.1` | Proxy | 1 | walked | Proxy. Walks the four steps of a forwarding server and a remote server, and states one obligation binding the forwarding server. The rest is advice to whoever implements a proxy that takes responsibility for retransmissions. |
| `3` | Packet Format | 2 | walked | Packet Format. States the encapsulation, UDP port 1813, the field layout, and both authenticator computations in RFC 2866's own voice; it does not defer to RFC 2865. The two capitalised sites are the Length rules. The Request Authenticator formula is stated in indicative prose ('contains a one-way MD5 hash calculated over a stream of octets consisting of the Code + Identifier + Length + 16 zero octets + request attributes + shared secret'), so the site scan sees no keyword there and RFC2866-3-2 is declared unsourced. The Response Authenticator paragraph is indicative in the same way; RFC2866-4.2-1 is instead read from the capitalised sentence in Section 4.2. The section closes with one SHOULD on preserving the order of attributes of the same type. |
| `4` | Packet Types | 0 | walked | Packet Types. One sentence: the Code field in the first octet decides the packet type. |
| `4.1` | Accounting-Request | 7 | walked | Accounting-Request. The description, the packet diagram, and the field notes for Code, Identifier, Request Authenticator and Attributes. Seven capitalised sites: one binds the accounting server, and six bind the client that builds the record, which is the role ze plays. |
| `4.2` | Accounting-Response | 2 | walked | Accounting-Response. Two capitalised sites: the server transmits the response when it recorded the request, and the Response Authenticator holds the correct response for the pending Accounting-Request, which the client checks on reception. |
| `5` | Attributes | 4 | walked | Attributes. The attribute format, the type list for 40 to 51, the Length rule, and the five data types. RFC 2866 states this section in its own voice: unlike RFC 2869 Section 5, it carries no sentence saying the format is included from RFC 2865 for ease of reference, so its four capitalised sites are read as RFC 2866's own obligations rather than as citations. |
| `5.1` | Acct-Status-Type | 0 | walked | Acct-Status-Type. Defines type 40 and its values: Start, Stop, Interim-Update, Accounting-On, Accounting-Off, the tunnel range and Failed. One MAY, on marking the start and the end of accounting with Accounting-On and Accounting-Off; ze sends neither, and the option is the owner's to take (ai/rules/rfc-compliance.md). No capitalised MUST-level keyword. |
| `5.2` | Acct-Delay-Time | 0 | walked | Acct-Delay-Time. Defines type 41, and notes that changing it changes the Attributes field and so requires a new Identifier and Request Authenticator. ze sends no Acct-Delay-Time. |
| `5.3` | Acct-Input-Octets | 0 | walked | Acct-Input-Octets. Defines type 42 and says it can only be present where Acct-Status-Type is Stop. Indicative prose, no capitalised keyword. |
| `5.4` | Acct-Output-Octets | 0 | walked | Acct-Output-Octets. Defines type 43 in the wording of Section 5.3 with the direction reversed. Indicative prose, no capitalised keyword. |
| `5.5` | Acct-Session-Id | 3 | walked | Acct-Session-Id. Defines type 44, states three capitalised obligations, and adds two SHOULDs on UTF-8 encoding. RFC2866-5.5-1 is declared unsourced here: 'unique across all active sessions on the NAS' is the summary's rendering of the indicative opening sentence, 'This attribute is a unique Accounting ID to make it easy to match start and stop records in a log file', which carries no keyword for the site scan to see. genSessionID (authradius/acct.go) is its producer. |
| `5.6` | Acct-Authentic | 0 | walked | Acct-Authentic. Defines type 45, how the user was authenticated. Carries one MAY on including it and one SHOULD NOT: 'Users who are delivered service without being authenticated SHOULD NOT generate Accounting records.' Neither is gated and ze sends no Acct-Authentic. The SHOULD NOT is the RFC's own acknowledgement that an unauthenticated session reaches accounting, which is the session whose empty User-Name site 5:3 governs. |
| `5.7` | Acct-Session-Time | 0 | walked | Acct-Session-Time. Defines type 46. Indicative prose, no capitalised keyword. |
| `5.8` | Acct-Input-Packets | 0 | walked | Acct-Input-Packets. Defines type 47. Indicative prose, no capitalised keyword. |
| `5.9` | Acct-Output-Packets | 0 | walked | Acct-Output-Packets. Defines type 48. Indicative prose, no capitalised keyword. |
| `5.10` | Acct-Terminate-Cause | 0 | walked | Acct-Terminate-Cause. Defines type 49 and its eighteen values, each with a one-line meaning. ze sends no Acct-Terminate-Cause. No obligation. |
| `5.11` | Acct-Multi-Session-Id | 0 | walked | Acct-Multi-Session-Id. Defines type 50 for linking the sessions of one multilink bundle. 'It is strongly recommended that' the value is UTF-8, which is advice and not a keyword. ze sends no Acct-Multi-Session-Id. |
| `5.12` | Acct-Link-Count | 0 | walked | Acct-Link-Count. Defines type 51, carries one MAY on including it in an Accounting-Request that might have multiple links, and works an eight-record example. ze runs no multilink bundle and sends no Acct-Link-Count. |
| `5.13` | Table of Attributes | 2 | walked | Table of Attributes. The table itself, Note 1, and the legend that defines the 0, 0+, 0-1 and 1 symbols. The two capitalised sites are Note 1 and the legend. |
| `6` | IANA Considerations | 0 | skipped (iana) | IANA Considerations. Registers the packet type codes, attribute types and attribute values from the RADIUS name spaces of RFC 2865, under BCP 26. Binds IANA, not a speaker. |
| `7` | Security Considerations | 0 | walked | Security Considerations. One sentence: the security issues are discussed in the sections about the authenticator and the shared secret. No obligation of its own. |
| `8` | Change Log | 0 | skipped (appendix-non-normative) | Change Log. Section 1 calls it an appendix: it lists what changed against RFC 2139, in the past tense. Its 'must' in 'it must be used in the accounting-request for that session' is a report of the change site 5.5:3 carries, not a second obligation. |
| `9` | not stated | 0 | skipped (references) | References: RFC 2139, RFC 2865, RFC 2119, RFC 768, RFC 1321, RFC 1700, RFC 2279 and RFC 2434. |
| `10` | Acknowledgements | 0 | skipped (acknowledgements) | Acknowledgements. Credits Livingston Enterprises for the original RADIUS Accounting work. |
| `11` | Chair's Address | 0 | skipped (front-matter) | Chair's Address. A postal, phone and mail address block, skipped the way rfc/extraction/rfc2869.json skips the same section. |
| `12` | Author's Address | 0 | skipped (front-matter) | Author's Address. An address block only. |
| `13` | Full Copyright Statement | 0 | skipped (front-matter) | Full Copyright Statement. The ISOC boilerplate, its disclaimer, and the RFC Editor funding acknowledgement. |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `2:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the RADIUS accounting server, the party that records the packet. ze runs no accounting server: radius/client.go opens a UDP socket to send requests and match the replies to them, and the one listener ze does run (authradius/coa.go) accepts a CoA-Request and a Disconnect-Request under RFC 5176, never an Accounting-Request. | If the RADIUS accounting server is unable to successfully record the accounting packet it MUST NOT send an Accounting-Response acknowledgment to the client. |
| `2.1:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds a forwarding server: the party that, in the four steps above the sentence, logs an accounting-request, adds its own Proxy-State after any other, updates the Request Authenticator and forwards the request to a remote server. ze proxies no RADIUS. buildAcctPacket (authradius/acct.go) builds the records of ze's own subscriber sessions and adds neither Proxy-State nor Class, and SendToServers (radius/client.go) sends them to the servers the operator configured. | A forwarding server MUST not modify existing Proxy-State or Class attributes present in the packet. |
| `4.1:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the RADIUS accounting server, the party that records the packet. ze runs no accounting server: radius/client.go opens a UDP socket to send requests and match the replies to them, and the one listener ze does run (authradius/coa.go) accepts a CoA-Request and a Disconnect-Request under RFC 5176, never an Accounting-Request. | Upon receipt of an Accounting-Request, the server MUST transmit an Accounting-Response reply if it successfully records the accounting packet, and MUST NOT transmit any reply if it fails to record the accounting packet. |
| `4.2:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the RADIUS accounting server, the party that records the packet. ze runs no accounting server: radius/client.go opens a UDP socket to send requests and match the replies to them, and the one listener ze does run (authradius/coa.go) accepts a CoA-Request and a Disconnect-Request under RFC 5176, never an Accounting-Request. | If the Accounting- Request was recorded successfully then the RADIUS accounting server MUST transmit a packet with the Code field set to 5 (Accounting-Response). |
| `5:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | The sentence scopes itself to an attribute 'received in an Accounting-Request', and only the accounting server receives one. Binds the RADIUS accounting server, the party that records the packet. ze runs no accounting server: radius/client.go opens a UDP socket to send requests and match the replies to them, and the one listener ze does run (authradius/coa.go) accepts a CoA-Request and a Disconnect-Request under RFC 5176, never an Accounting-Request. Decode (radius/packet.go) does refuse an attribute whose Length is below 2 or runs past the packet, for every code it parses, so the rule holds wherever ze could reach it. | If an attribute is received in an Accounting-Request with an invalid Length, the entire request MUST be silently discarded. |
| `5:4` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The same obligation for the string data type that the sentence before it states for text; the two differ only in the type they name, and site 5:3 maps it. The producer is the same and no attribute ze builds is a zero-length string: an Accounting-Request from ze carries string-typed values only in NAS-Port-Id, appended only when the template resolved to text, and in the NAS identity, which appendNASIdentity never leaves empty. | Strings of length zero (0) MUST NOT be sent; omit the entire attribute instead. |
| `5.5:3` | `advisory-in-context` (never bound Ze): the sentence advises on applying a rule stated elsewhere and adds no obligation of its own | The enclosing construction is 'An Access-Request packet MAY have an Acct-Session-Id; if it does, then the NAS MUST use the same Acct-Session-Id in the Accounting-Request packets for that session.' The MUST is conditioned on the MAY, and ze declines the option: buildAuthAttrs (authradius/handler.go) puts no Acct-Session-Id in an Access-Request, and the id does not exist yet at that point, because onSessionIPAssigned generates it when IPCP assigns the address (authradius/acct.go). Whether ze should take the option is a MAY for the owner to answer (ai/rules/rfc-compliance.md), not a requirement this walk can close. | An Access-Request packet MAY have an Acct-Session-Id; if it does, then the NAS MUST use the same Acct-Session-Id in the Accounting- Request packets for that session. |
| `5.13:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Note 1 restates over the Table of Attributes the NAS identity rule Section 4.1 already states; site 4.1:3 maps it, and appendNASIdentity (authradius/nasidentity.go) is the one producer both sentences describe. | [Note 1] An Accounting-Request MUST contain either a NAS-IP-Address or a NAS-Identifier (or both). |

## Superseded

No document obsoletes RFC 2866, so its obligations are stated where they were written.
