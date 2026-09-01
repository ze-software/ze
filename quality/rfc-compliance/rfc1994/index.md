# RFC 1994 - PPP Challenge Handshake Authentication Protocol (CHAP)

Partial. Every requirement this repository extracted from RFC 1994, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 29.4% | 5 of 17 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 52.9% | 9 of 17 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 17 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 0.0% | 0 of 24 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 17 | of 27 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 17 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 17.6% | 3 of 17 binding obligations | no test carries the requirement id, whether or not a gap states why |

The 4 shares marked as a part above are the whole of the 17 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| No test at all | bad | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Partial |
| Enrolment | Enrolled |
| Requirements | 27 |
| Gated MUST-level | 17 |
| Obligations that bind Ze | 17 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 3 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 24 |
| Tagged units | 24 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc1994.md` |
| Requirement shard | `rfc/requirements/rfc1994.md` |
| RFC text | `rfc/full/rfc1994.txt` |

## Enrolment

Enrolled: PPP CHAP-MD5 (RFC 1994): authenticator (LNS) + peer (PPPoE client); 5 MET (auth-protocol advertise, Success/Failure per comparison, match->Success, mismatch->Failure, peer Response) + 9 single-polarity positive (Challenge Code 1, changing Identifier/Value, echoed Identifier, repeated-Response tolerance, other-phase discard, Message-independence, interop) + 3 gap (no Challenge retransmit, no repeated-Response replay, no 1-octet secret minimum)

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

CHAP-MD5 authenticator (LNS) and peer (PPPoE client): Challenge/Response/Success/Failure codec, per-call 16-octet random Challenge, changing Identifier, MD5(id\|\|secret\|\|challenge) validation via local user table and RADIUS CHAP-Password, LCP Auth-Protocol negotiation.

**What the ledger says remains**

Three MUST gaps in [`rfc/short/rfc1994.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc1994.md): [`RFC1994-4.1-2`](#rfc1994-4.1-2) -- one Challenge is sent then the session fails closed on timeout (no retransmission); [`RFC1994-4.1-9`](#rfc1994-4.1-9) -- a repeated Response with the current Challenge Identifier is silently dropped rather than re-answered with the prior reply Code (session_run.go); [`RFC1994-2.3-1`](#rfc1994-2.3-1) -- the CHAP secret has no 1-octet minimum, so an empty password is accepted.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 5 | one part of the gated population |
| Annotated instead of tested | 12 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **17** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (5):** [`RFC1994-1-1`](#rfc1994-1-1), [`RFC1994-4.1-3`](#rfc1994-4.1-3), [`RFC1994-4.2-1`](#rfc1994-4.2-1), [`RFC1994-4.2-2`](#rfc1994-4.2-2), [`RFC1994-4.1-4`](#rfc1994-4.1-4)

**Annotated instead of tested (12):** [`RFC1994-4.1-1`](#rfc1994-4.1-1), [`RFC1994-4.1-2`](#rfc1994-4.1-2), [`RFC1994-4.1-5`](#rfc1994-4.1-5), [`RFC1994-4.1-6`](#rfc1994-4.1-6), [`RFC1994-4.1-7`](#rfc1994-4.1-7), [`RFC1994-4.2-3`](#rfc1994-4.2-3), [`RFC1994-4.1-8`](#rfc1994-4.1-8), [`RFC1994-4.1-9`](#rfc1994-4.1-9), [`RFC1994-4.1-10`](#rfc1994-4.1-10), [`RFC1994-2.3-1`](#rfc1994-2.3-1), [`RFC1994-4.2-4`](#rfc1994-4.2-4), [`RFC1994-1.1-1`](#rfc1994-1.1-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC1994-1-1` | If authentication is desired, specify Authentication-Protocol Configuration Option during Link Establishment phase (Section 1) | MUST | 1 | **positive:** `unit/verify` [`TestLocalCONFREQAdvertisesAuthMethod`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_dispatch_test.go#L684). **negative:** `unit/verify` [`TestAuthProtoRejectClearsMethod`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_dispatch_test.go#L155) |
| `RFC1994-4.1-1` | Authenticator must transmit a Challenge packet (Code=1) (Section 4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestCHAPResponseEmitsEvent`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/chap_test.go#L385). **negative:** no negative test. **{single-polarity}:** runCHAPAuthPhase always frames and writes a CHAP Challenge with Code=1 as its first wire act, with no must-not-challenge branch (internal/component/l2tp/ppp/chap.go:259-261, :144-146) |
| `RFC1994-4.1-2` | Additional Challenge packets must be sent until valid Response received or retry counter expires (Section 4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze sends exactly one Challenge and, on Response timeout, fails the session closed rather than retransmitting the same Identifier/Value (internal/component/l2tp/ppp/chap.go:257-263 single send; internal/component/l2tp/ppp/auth.go:328-337 timeout calls s.fail with no retransmit) |
| `RFC1994-4.1-3` | Authenticator must send Success or Failure based on Response comparison (Section 4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestCHAPResponseEmitsEvent`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/chap_test.go#L386). **negative:** `unit/verify` [`TestCHAPRejectWritesFailure`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/chap_test.go#L521) |
| `RFC1994-4.2-1` | If Response Value equals expected value, must transmit Success (Code=3) (Section 4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestCHAPResponseEmitsEvent`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/chap_test.go#L387). **positive:** `unit/verify` [`TestLocalAuthCHAPMD5Accept`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authlocal/auth_test.go#L47). **negative:** `unit/verify` [`TestCHAPRejectWritesFailure`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/chap_test.go#L522). **negative:** `unit/verify` [`TestLocalAuthCHAPMD5Reject`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authlocal/auth_test.go#L82) |
| `RFC1994-4.2-2` | If Response Value does not equal expected value, must transmit Failure (Code=4) (Section 4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestCHAPRejectWritesFailure`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/chap_test.go#L523). **positive:** `unit/verify` [`TestLocalAuthCHAPMD5Reject`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authlocal/auth_test.go#L84). **negative:** `unit/verify` [`TestLocalAuthCHAPMD5Accept`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authlocal/auth_test.go#L50) |
| `RFC1994-4.1-4` | Peer must transmit Response (Code=2) whenever Challenge is received (Section 4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestBuildCHAPResponse`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/session_test.go#L22). **negative:** `unit/verify` [`TestBuildCHAPResponseMalformed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/session_test.go#L60) |
| `RFC1994-4.1-5` | Identifier must be changed each time a Challenge is sent (Section 4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestCHAPIdentifierMonotonic`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/chap_test.go#L835). **positive:** `unit/verify` [`TestCHAPIdentifierWraps`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/chap_test.go#L880). **negative:** no negative test. **{single-polarity}:** each runCHAPAuthPhase increments the per-session chapIdentifier before sending, so every new Challenge carries a distinct Identifier, and ze never retransmits a Challenge to form a reuse negative (internal/component/l2tp/ppp/chap.go:254-255) |
| `RFC1994-4.1-6` | Response Identifier must be copied from the Challenge Identifier (Section 4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestBuildCHAPResponse`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/session_test.go#L24). **negative:** no negative test. **{single-polarity}:** the peer copies the received Challenge's Identifier byte into the Response header, asserted directly with no rejecting counterpart (internal/component/l2tp/pppoeclient/session.go:400) |
| `RFC1994-4.1-7` | Challenge Value must be changed each time a Challenge is sent (Section 4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestCHAPChallengeRandom`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/chap_test.go#L947). **negative:** no negative test. **{single-polarity}:** runCHAPAuthPhase draws a fresh 16-octet value from crypto/rand for every Challenge (internal/component/l2tp/ppp/chap.go:219-225, :248-249) |
| `RFC1994-4.2-3` | Success/Failure Identifier must be copied from the Response Identifier (Section 4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestCHAPResponseEmitsEvent`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/chap_test.go#L388). **negative:** no negative test. **{single-polarity}:** waitCHAPResponse only returns a Response whose Identifier equals the outstanding Challenge Identifier, and runCHAPAuthPhase writes Success/Failure with that same Identifier (internal/component/l2tp/ppp/chap.go:296-298, auth.go:318-323) |
| `RFC1994-4.1-8` | Authenticator must allow repeated Response packets during Network-Layer Protocol phase (Section 4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestCHAPRepeatedResponseAfterSuccessKeepsSessionUp`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/chap_reauth_test.go#L407). **negative:** no negative test. **{single-polarity}:** a CHAP Response arriving in the main loop after auth completes hits the frame-dispatch default and is dropped without terminating the session, so repeated Responses are tolerated (internal/component/l2tp/ppp/session_run.go:681-683) |
| `RFC1994-4.1-9` | Response with current Challenge Identifier must return same reply Code as previously (Section 4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze caches no per-Challenge reply Code and does not re-send the prior Success/Failure for a repeated Response; it silently drops it (internal/component/l2tp/ppp/session_run.go:681-683; no reply-Code cache in runCHAPAuthPhase) |
| `RFC1994-4.1-10` | Response packets received during any other phase must be silently discarded (Section 4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestCHAPIdentifierMismatchSilentDiscard`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/chap_reauth_test.go#L151). **negative:** no negative test. **{single-polarity}:** outside an active auth-wait a CHAP Response is silently dropped by the frame-dispatch default, and during a wait a Response whose Identifier does not match is silently discarded and the wait continues (internal/component/l2tp/ppp/session_run.go:681-683, auth.go:318-322) |
| `RFC1994-2.3-1` | Length of the secret must be at least 1 octet (Section 2.3) | MUST | 2.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the CHAP shared secret (authlocal user password) has no minimum-length constraint, so an empty password is accepted and fed to the MD5 hash without rejection (internal/component/l2tp/plugins/authlocal/auth.go:94-99; empty password stored in register.go) |
| `RFC1994-4.2-4` | Message field must not affect operation of the protocol (Section 4.2) | MUST NOT | 4.2 | **positive:** `unit/verify` [`TestCHAPSuccessMessageDoesNotAffectOutcome`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1994_chap_message_test.go#L39). **negative:** no negative test. **{single-polarity}:** the peer branches only on the Success/Failure Code (3 succeed, 4 fail) and never reads or acts on the Message field (internal/component/l2tp/pppoeclient/session.go:270-274) |
| `RFC1994-1.1-1` | Implementation not including an option must be prepared to interoperate with one that does (Section 1.1) | MUST | 1.1 | **positive:** `unit/verify` [`TestNegotiatePeerAuthProtoAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/lcp_options_test.go#L211). **positive:** `unit/verify` [`TestNegotiatePeerAuthProtoRejected`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/lcp_options_test.go#L194). **negative:** no negative test. **{single-polarity}:** ze negotiates the Auth-Protocol option in both directions -- it accepts a peer-proposed option and handles the peer's Configure-Nak/Reject of it -- so it interoperates whether or not the option is used (internal/component/l2tp/ppp/lcp_options.go:174-183, auth.go:38-68) |
| `RFC1994-2-1` | Connection should be terminated on authentication failure (Section 2, Section 4.2) | SHOULD | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC1994-4.1-11` | Peer should expect Challenge packets during Authentication and Network-Layer Protocol phases (Section 4.1) | SHOULD | 4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC1994-2.3-2` | Secret should be at least as large and unguessable as a well-chosen password (Section 2.3) | SHOULD | 2.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC1994-2.3-3` | Each challenge value should be unique, exhibit global and temporal uniqueness (Section 2.3) | SHOULD | 2.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC1994-2.3-4` | Each challenge value should be unpredictable (Section 2.3) | SHOULD | 2.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC1994-1.2-1` | Provide capability of logging silently discarded packets and record in statistics counter (Section 1.2) | SHOULD | 1.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC1994-x-1` | Secret should not be the same in both directions (Security Considerations) | SHOULD NOT | x | **positive:** no positive test. **negative:** no negative test |
| `RFC1994-4.1-12` | Challenge may be sent at any time during Network-Layer Protocol phase (Section 4.1) | MAY | 4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC1994-4.1-13` | Name may contain ASCII strings or ASN.1 identifiers (Section 4.1) | MAY | 4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC1994-4.1-14` | Success/Failure Message may differ between replies for the same Identifier (Section 4.1) | MAY | 4.1 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC1994-4.1-2`](#rfc1994-4.1-2) Additional Challenge packets must be sent until valid Response received or retry counter expires (Section 4.1) | {gap}, no test | ze sends exactly one Challenge and, on Response timeout, fails the session closed rather than retransmitting the same Identifier/Value (internal/component/l2tp/ppp/chap.go:257-263 single send; internal/component/l2tp/ppp/auth.go:328-337 timeout calls s.fail with no retransmit) |
| [`RFC1994-4.1-9`](#rfc1994-4.1-9) Response with current Challenge Identifier must return same reply Code as previously (Section 4.1) | {gap}, no test | ze caches no per-Challenge reply Code and does not re-send the prior Success/Failure for a repeated Response; it silently drops it (internal/component/l2tp/ppp/session_run.go:681-683; no reply-Code cache in runCHAPAuthPhase) |
| [`RFC1994-2.3-1`](#rfc1994-2.3-1) Length of the secret must be at least 1 octet (Section 2.3) | {gap}, no test | the CHAP shared secret (authlocal user password) has no minimum-length constraint, so an empty password is accepted and fed to the MD5 hash without rejection (internal/component/l2tp/plugins/authlocal/auth.go:94-99; empty password stored in register.go) |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC1994-1-1`](#rfc1994-1-1)

If authentication is desired, specify Authentication-Protocol Configuration Option during Link Establishment phase (Section 1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestAuthProtoRejectClearsMethod`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_dispatch_test.go#L155) | unit/verify | unproven |
| positive | [`TestLocalCONFREQAdvertisesAuthMethod`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_dispatch_test.go#L684) | unit/verify | unproven |

### [`RFC1994-4.1-1`](#rfc1994-4.1-1)

Authenticator must transmit a Challenge packet (Code=1) (Section 4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestCHAPResponseEmitsEvent`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/chap_test.go#L385) | unit/verify | unproven |

### [`RFC1994-4.1-2`](#rfc1994-4.1-2)

Additional Challenge packets must be sent until valid Response received or retry counter expires (Section 4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1994-4.1-2, so no unit is bound to it.

### [`RFC1994-4.1-3`](#rfc1994-4.1-3)

Authenticator must send Success or Failure based on Response comparison (Section 4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestCHAPRejectWritesFailure`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/chap_test.go#L521) | unit/verify | unproven |
| positive | [`TestCHAPResponseEmitsEvent`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/chap_test.go#L386) | unit/verify | unproven |

### [`RFC1994-4.2-1`](#rfc1994-4.2-1)

If Response Value equals expected value, must transmit Success (Code=3) (Section 4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLocalAuthCHAPMD5Reject`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authlocal/auth_test.go#L82) | unit/verify | unproven |
| negative | [`TestCHAPRejectWritesFailure`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/chap_test.go#L522) | unit/verify | unproven |
| positive | [`TestLocalAuthCHAPMD5Accept`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authlocal/auth_test.go#L47) | unit/verify | unproven |
| positive | [`TestCHAPResponseEmitsEvent`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/chap_test.go#L387) | unit/verify | unproven |

### [`RFC1994-4.2-2`](#rfc1994-4.2-2)

If Response Value does not equal expected value, must transmit Failure (Code=4) (Section 4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLocalAuthCHAPMD5Accept`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authlocal/auth_test.go#L50) | unit/verify | unproven |
| positive | [`TestLocalAuthCHAPMD5Reject`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authlocal/auth_test.go#L84) | unit/verify | unproven |
| positive | [`TestCHAPRejectWritesFailure`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/chap_test.go#L523) | unit/verify | unproven |

### [`RFC1994-4.1-4`](#rfc1994-4.1-4)

Peer must transmit Response (Code=2) whenever Challenge is received (Section 4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestBuildCHAPResponseMalformed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/session_test.go#L60) | unit/verify | unproven |
| positive | [`TestBuildCHAPResponse`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/session_test.go#L22) | unit/verify | unproven |

### [`RFC1994-4.1-5`](#rfc1994-4.1-5)

Identifier must be changed each time a Challenge is sent (Section 4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestCHAPIdentifierMonotonic`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/chap_test.go#L835) | unit/verify | unproven |
| positive | [`TestCHAPIdentifierWraps`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/chap_test.go#L880) | unit/verify | unproven |

### [`RFC1994-4.1-6`](#rfc1994-4.1-6)

Response Identifier must be copied from the Challenge Identifier (Section 4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestBuildCHAPResponse`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/session_test.go#L24) | unit/verify | unproven |

### [`RFC1994-4.1-7`](#rfc1994-4.1-7)

Challenge Value must be changed each time a Challenge is sent (Section 4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestCHAPChallengeRandom`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/chap_test.go#L947) | unit/verify | unproven |

### [`RFC1994-4.2-3`](#rfc1994-4.2-3)

Success/Failure Identifier must be copied from the Response Identifier (Section 4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestCHAPResponseEmitsEvent`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/chap_test.go#L388) | unit/verify | unproven |

### [`RFC1994-4.1-8`](#rfc1994-4.1-8)

Authenticator must allow repeated Response packets during Network-Layer Protocol phase (Section 4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestCHAPRepeatedResponseAfterSuccessKeepsSessionUp`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/chap_reauth_test.go#L407) | unit/verify | unproven |

### [`RFC1994-4.1-9`](#rfc1994-4.1-9)

Response with current Challenge Identifier must return same reply Code as previously (Section 4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1994-4.1-9, so no unit is bound to it.

### [`RFC1994-4.1-10`](#rfc1994-4.1-10)

Response packets received during any other phase must be silently discarded (Section 4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestCHAPIdentifierMismatchSilentDiscard`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/chap_reauth_test.go#L151) | unit/verify | unproven |

### [`RFC1994-2.3-1`](#rfc1994-2.3-1)

Length of the secret must be at least 1 octet (Section 2.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1994-2.3-1, so no unit is bound to it.

### [`RFC1994-4.2-4`](#rfc1994-4.2-4)

Message field must not affect operation of the protocol (Section 4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestCHAPSuccessMessageDoesNotAffectOutcome`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1994_chap_message_test.go#L39) | unit/verify | unproven |

### [`RFC1994-1.1-1`](#rfc1994-1.1-1)

Implementation not including an option must be prepared to interoperate with one that does (Section 1.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestNegotiatePeerAuthProtoAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/lcp_options_test.go#L211) | unit/verify | unproven |
| positive | [`TestNegotiatePeerAuthProtoRejected`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/lcp_options_test.go#L194) | unit/verify | unproven |

## Extraction sign-off

No extraction sign-off exists for RFC 1994, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 1994, so its obligations are stated where they were written.
