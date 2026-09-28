# RFC 1994 - PPP Challenge Handshake Authentication Protocol (CHAP)

Partial. Every requirement this repository extracted from RFC 1994, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 29.4% | 5 of 17 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 52.9% | 9 of 17 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 17 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 4.2% | 1 of 24 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 17 | of 27 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 17 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 17 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 17 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 17 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 17.6% | 3 of 17 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Audit verdicts | 14 | of 17 gated MUSTs judged | 8 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

The 7 shares marked as a part above are the whole of the 17 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| No test at all | bad | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green shares above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | bad | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Partial |
| Enrolment | Enrolled |
| Requirements | 27 |
| Gated MUST-level | 17 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 3 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 24 |
| Tagged units | 24 |
| Recorded audit verdicts | 14 |
| Discrimination records | 1 |
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
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **17** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (5):** [`RFC1994-1-1`](#rfc1994-1-1), [`RFC1994-4.1-3`](#rfc1994-4.1-3), [`RFC1994-4.2-1`](#rfc1994-4.2-1), [`RFC1994-4.2-2`](#rfc1994-4.2-2), [`RFC1994-4.1-4`](#rfc1994-4.1-4)

**Annotated instead of tested (12):** [`RFC1994-4.1-1`](#rfc1994-4.1-1), [`RFC1994-4.1-2`](#rfc1994-4.1-2), [`RFC1994-4.1-5`](#rfc1994-4.1-5), [`RFC1994-4.1-6`](#rfc1994-4.1-6), [`RFC1994-4.1-7`](#rfc1994-4.1-7), [`RFC1994-4.2-3`](#rfc1994-4.2-3), [`RFC1994-4.1-8`](#rfc1994-4.1-8), [`RFC1994-4.1-9`](#rfc1994-4.1-9), [`RFC1994-4.1-10`](#rfc1994-4.1-10), [`RFC1994-2.3-1`](#rfc1994-2.3-1), [`RFC1994-4.2-4`](#rfc1994-4.2-4), [`RFC1994-1.1-1`](#rfc1994-1.1-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC1994-1-1` | If authentication of the link is desired, an implementation MUST specify the Authentication-Protocol Configuration Option during Link Establishment phase. (Section 1) | MUST | 1 | **positive:** `unit/verify` [`TestLocalCONFREQAdvertisesAuthMethod`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_dispatch_test.go#L684). **negative:** `unit/verify` [`TestAuthProtoRejectClearsMethod`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_dispatch_test.go#L155) |
| `RFC1994-4.1-1` | The authenticator MUST transmit a CHAP packet with the Code field set to 1 (Challenge). (Section 4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestCHAPResponseEmitsEvent`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/chap_test.go#L385). **negative:** no negative test. **{single-polarity}:** runCHAPAuthPhase always frames and writes a CHAP Challenge with Code=1 as its first wire act, with no must-not-challenge branch (internal/component/l2tp/ppp/chap.go:259-261, :144-146) |
| `RFC1994-4.1-2` | Additional Challenge packets MUST be sent until a valid Response packet is received, or an optional retry counter expires. (Section 4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze sends exactly one Challenge and, on Response timeout, fails the session closed rather than retransmitting the same Identifier/Value (internal/component/l2tp/ppp/chap.go:257-263 single send; internal/component/l2tp/ppp/auth.go:328-337 timeout calls s.fail with no retransmit) |
| `RFC1994-4.1-3` | Based on this comparison, the authenticator MUST send a Success or Failure packet (described below). (Section 4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestCHAPResponseEmitsEvent`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/chap_test.go#L386). **negative:** `unit/verify` [`TestCHAPRejectWritesFailure`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/chap_test.go#L521) |
| `RFC1994-4.2-1` | If the Value received in a Response is equal to the expected value, then the implementation MUST transmit a CHAP packet with the Code field set to 3 (Success). (Section 4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestCHAPResponseEmitsEvent`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/chap_test.go#L387). **positive:** `unit/verify` [`TestLocalAuthCHAPMD5Accept`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authlocal/auth_test.go#L46). **negative:** `unit/verify` [`TestCHAPRejectWritesFailure`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/chap_test.go#L522). **negative:** `unit/verify` [`TestLocalAuthCHAPMD5Reject`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authlocal/auth_test.go#L81) |
| `RFC1994-4.2-2` | If the Value received in a Response is not equal to the expected value, then the implementation MUST transmit a CHAP packet with the Code field set to 4 (Failure) (§4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestCHAPRejectWritesFailure`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/chap_test.go#L523). **positive:** `unit/verify` [`TestLocalAuthCHAPMD5Reject`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authlocal/auth_test.go#L83). **negative:** `unit/verify` [`TestLocalAuthCHAPMD5Accept`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authlocal/auth_test.go#L49) |
| `RFC1994-4.1-4` | Whenever a Challenge packet is received, the peer MUST transmit a CHAP packet with the Code field set to 2 (Response). (Section 4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestBuildCHAPResponse`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/session_test.go#L22). **negative:** `unit/verify` [`TestBuildCHAPResponseMalformed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/session_test.go#L60) |
| `RFC1994-4.1-5` | The Identifier field MUST be changed each time a Challenge is sent. (Section 4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestCHAPIdentifierMonotonic`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/chap_test.go#L835). **positive:** `unit/verify` [`TestCHAPIdentifierWraps`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/chap_test.go#L880). **negative:** no negative test. **{single-polarity}:** each runCHAPAuthPhase increments the per-session chapIdentifier before sending, so every new Challenge carries a distinct Identifier, and ze never retransmits a Challenge to form a reuse negative (internal/component/l2tp/ppp/chap.go:254-255) |
| `RFC1994-4.1-6` | The Response Identifier MUST be copied from the Identifier field of the Challenge which caused the Response. (Section 4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestBuildCHAPResponse`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/session_test.go#L24). **negative:** no negative test. **{single-polarity}:** the peer copies the received Challenge's Identifier byte into the Response header, asserted directly with no rejecting counterpart (internal/component/l2tp/pppoeclient/session.go:400) |
| `RFC1994-4.1-7` | The Challenge Value MUST be changed each time a Challenge is sent. (Section 4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestCHAPChallengeRandom`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/chap_test.go#L952). **negative:** no negative test. **{single-polarity}:** runCHAPAuthPhase draws a fresh 16-octet value from crypto/rand for every Challenge (internal/component/l2tp/ppp/chap.go:219-225, :248-249) |
| `RFC1994-4.2-3` | The Identifier field MUST be copied from the Identifier field of the Response which caused this reply. (Section 4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestCHAPResponseEmitsEvent`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/chap_test.go#L388). **negative:** no negative test. **{single-polarity}:** waitCHAPResponse only returns a Response whose Identifier equals the outstanding Challenge Identifier, and runCHAPAuthPhase writes Success/Failure with that same Identifier (internal/component/l2tp/ppp/chap.go:296-298, auth.go:318-323) |
| `RFC1994-4.1-8` | Implementation Notes: Because the Success might be lost, the authenticator MUST allow repeated Response packets during the Network-Layer Protocol phase after completing the Authentication phase. (Section 4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestCHAPRepeatedResponseAfterSuccessKeepsSessionUp`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/chap_reauth_test.go#L407). **negative:** no negative test. **{single-polarity}:** a CHAP Response arriving in the main loop after auth completes hits the frame-dispatch default and is dropped without terminating the session, so repeated Responses are tolerated (internal/component/l2tp/ppp/session_run.go:681-683) |
| `RFC1994-4.1-9` | To prevent discovery of alternative Names and Secrets, any Response packets received having the current Challenge Identifier MUST return the same reply Code previously returned for that specific Challenge (§4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze caches no per-Challenge reply Code and does not re-send the prior Success/Failure for a repeated Response; it silently drops it (internal/component/l2tp/ppp/session_run.go:681-683; no reply-Code cache in runCHAPAuthPhase) |
| `RFC1994-4.1-10` | Any Response packets received during any other phase MUST be silently discarded. (Section 4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestCHAPIdentifierMismatchSilentDiscard`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/chap_reauth_test.go#L151). **negative:** no negative test. **{single-polarity}:** outside an active auth-wait a CHAP Response is silently dropped by the frame-dispatch default, and during a wait a Response whose Identifier does not match is silently discarded and the wait continues (internal/component/l2tp/ppp/session_run.go:681-683, auth.go:318-322) |
| `RFC1994-2.3-1` | The CHAP algorithm requires that the length of the secret MUST be at least 1 octet. (Section 2.3) | MUST | 2.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the CHAP shared secret (authlocal user password) has no minimum-length constraint, so an empty password is accepted and fed to the MD5 hash without rejection (internal/component/l2tp/plugins/authlocal/auth.go:94-99; empty password stored in register.go) |
| `RFC1994-4.2-4` | It is intended to be human readable, and MUST NOT affect operation of the protocol. (Section 4.2) | MUST NOT | 4.2 | **positive:** `unit/verify` [`TestCHAPSuccessMessageDoesNotAffectOutcome`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1994_chap_message_test.go#L39). **negative:** no negative test. **{single-polarity}:** the peer branches only on the Success/Failure Code (3 succeed, 4 fail) and never reads or acts on the Message field (internal/component/l2tp/pppoeclient/session.go:270-274) |
| `RFC1994-1.1-1` | An implementation which does not include this option MUST be prepared to interoperate with another implementation which does include the option. (Section 1.1) | MUST | 1.1 | **positive:** `unit/verify` [`TestNegotiatePeerAuthProtoAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/lcp_options_test.go#L211). **positive:** `unit/verify` [`TestNegotiatePeerAuthProtoRejected`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/lcp_options_test.go#L194). **negative:** no negative test. **{single-polarity}:** ze negotiates the Auth-Protocol option in both directions -- it accepts a peer-proposed option and handles the peer's Configure-Nak/Reject of it -- so it interoperates whether or not the option is used (internal/component/l2tp/ppp/lcp_options.go:174-183, auth.go:38-68) |
| `RFC1994-2-1` | If the values match, the authentication is acknowledged; otherwise the connection SHOULD be terminated. (§2) | SHOULD | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC1994-4.1-11` | The peer SHOULD expect Challenge packets during the Authentication phase and the Network-Layer Protocol phase. (§4.1) | SHOULD | 4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC1994-2.3-2` | The secret SHOULD be at least as large and unguessable as a well-chosen password. (§2.3) | SHOULD | 2.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC1994-2.3-3` | Each challenge value SHOULD be unique, since repetition of a challenge value in conjunction with the same secret would permit an attacker to reply with a previously intercepted response. Since it is expected that the same secret MAY be used to authenticate with servers in disparate geographic regions, the challenge SHOULD exhibit global and temporal uniqueness. (§2.3) | SHOULD | 2.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC1994-2.3-4` | Each challenge value SHOULD also be unpredictable, least an attacker trick a peer into responding to a predicted future challenge, and then use the response to masquerade as that peer to an authenticator. (§2.3) | SHOULD | 2.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC1994-1.2-1` | The implementation SHOULD provide the capability of logging the error, including the contents of the silently discarded packet, and SHOULD record the event in a statistics counter. (§1.2) | SHOULD | 1.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC1994-x-1` | The secret SHOULD NOT be the same in both directions. (§4.2) | SHOULD NOT | 4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC1994-4.1-12` | A Challenge packet MAY also be transmitted at any time during the Network-Layer Protocol phase to ensure that the connection has not been altered. (§4.1) | MAY | 4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC1994-4.1-13` | For example, it MAY contain ASCII character strings or globally unique identifiers in ASN.1 syntax. (§4.1) | MAY | 4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC1994-4.1-14` | reply Code previously returned for that specific Challenge (the message portion MAY be different) (§4.1) | MAY | 4.1 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC1994-4.1-2`](#rfc1994-4.1-2) Additional Challenge packets MUST be sent until a valid Response packet is received, or an optional retry counter expires. (Section 4.1) | {gap}, no test | ze sends exactly one Challenge and, on Response timeout, fails the session closed rather than retransmitting the same Identifier/Value (internal/component/l2tp/ppp/chap.go:257-263 single send; internal/component/l2tp/ppp/auth.go:328-337 timeout calls s.fail with no retransmit) |
| [`RFC1994-4.1-9`](#rfc1994-4.1-9) To prevent discovery of alternative Names and Secrets, any Response packets received having the current Challenge Identifier MUST return the same reply Code previously returned for that specific Challenge (§4.1) | {gap}, no test | ze caches no per-Challenge reply Code and does not re-send the prior Success/Failure for a repeated Response; it silently drops it (internal/component/l2tp/ppp/session_run.go:681-683; no reply-Code cache in runCHAPAuthPhase) |
| [`RFC1994-2.3-1`](#rfc1994-2.3-1) The CHAP algorithm requires that the length of the secret MUST be at least 1 octet. (Section 2.3) | {gap}, no test | the CHAP shared secret (authlocal user password) has no minimum-length constraint, so an empty password is accepted and fed to the MD5 hash without rejection (internal/component/l2tp/plugins/authlocal/auth.go:94-99; empty password stored in register.go) |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC1994-1-1`](#rfc1994-1-1)

If authentication of the link is desired, an implementation MUST specify the Authentication-Protocol Configuration Option during Link Establishment phase. (Section 1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a CONFREQ that omits the Authentication-Protocol option while authentication is desired. TestLocalCONFREQAdvertisesAuthMethod starts a session with CHAP-MD5 configured and asserts the CONFREQ carries Auth-Protocol 0xC223 algorithm 0x05 (red if absent); TestAuthProtoRejectClearsMethod asserts that once the peer Configure-Rejects the option and the method becomes None, the resent CONFREQ omits it, over the same producer, so emission follows the desire.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestAuthProtoRejectClearsMethod`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_dispatch_test.go#L155) | unit/verify | unproven |
| positive | [`TestLocalCONFREQAdvertisesAuthMethod`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_dispatch_test.go#L684) | unit/verify | unproven |

### [`RFC1994-4.1-1`](#rfc1994-4.1-1)

The authenticator MUST transmit a CHAP packet with the Code field set to 1 (Challenge). (Section 4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: an authenticator that does not send a Challenge (Code 1). TestCHAPResponseEmitsEvent reads the first wire frame runCHAPAuthPhase writes and asserts protocol 0xC223 and Code CHAPCodeChallenge, red otherwise. Single-polarity marker: there is no must-not-challenge branch.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestCHAPResponseEmitsEvent`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/chap_test.go#L385) | unit/verify | unproven |

### [`RFC1994-4.1-2`](#rfc1994-4.1-2)

Additional Challenge packets MUST be sent until a valid Response packet is received, or an optional retry counter expires. (Section 4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1994-4.1-2, so no unit is bound to it.

### [`RFC1994-4.1-3`](#rfc1994-4.1-3)

Based on this comparison, the authenticator MUST send a Success or Failure packet (described below). (Section 4.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Forbidden: answering a Response with something other than Success or Failure chosen BY THE COMPARISON. TestCHAPResponseEmitsEvent and TestCHAPRejectWritesFailure inject the decision on s.authRespCh (accept true/false) and assert Code 3/4; the comparison of the received Value with the expected one is not in either unit. Inverting the verifier-to-authRespCh handoff leaves both green, the same two-half gap as RFC1994-4.2-1/4.2-2.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestCHAPRejectWritesFailure`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/chap_test.go#L521) | unit/verify | unproven |
| positive | [`TestCHAPResponseEmitsEvent`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/chap_test.go#L386) | unit/verify | unproven |

### [`RFC1994-4.2-1`](#rfc1994-4.2-1)

If the Value received in a Response is equal to the expected value, then the implementation MUST transmit a CHAP packet with the Code field set to 3 (Success). (Section 4.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Proven in two halves: authlocal TestLocalAuthCHAPMD5Accept/Reject prove verifyCHAPMD5 compares the Value, and ppp TestCHAPResponseEmitsEvent/TestCHAPRejectWritesFailure prove an accept/reject decision yields Code 3/4. No tagged unit drives a Response Value through to the wire Code, so inverting the result-to-authRespCh handoff would leave every tag green.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLocalAuthCHAPMD5Reject`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authlocal/auth_test.go#L81) | unit/verify | unproven |
| negative | [`TestCHAPRejectWritesFailure`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/chap_test.go#L522) | unit/verify | unproven |
| positive | [`TestLocalAuthCHAPMD5Accept`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authlocal/auth_test.go#L46) | unit/verify | unproven |
| positive | [`TestCHAPResponseEmitsEvent`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/chap_test.go#L387) | unit/verify | unproven |

### [`RFC1994-4.2-2`](#rfc1994-4.2-2)

If the Value received in a Response is not equal to the expected value, then the implementation MUST transmit a CHAP packet with the Code field set to 4 (Failure) (§4.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Same two-half proof as RFC1994-4.2-1: TestLocalAuthCHAPMD5Reject proves a mismatching Value is rejected and TestCHAPRejectWritesFailure proves a reject decision writes Code 4, but no unit carries a mismatching Response Value to a wire Failure frame.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLocalAuthCHAPMD5Accept`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authlocal/auth_test.go#L49) | unit/verify | unproven |
| positive | [`TestLocalAuthCHAPMD5Reject`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authlocal/auth_test.go#L83) | unit/verify | unproven |
| positive | [`TestCHAPRejectWritesFailure`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/chap_test.go#L523) | unit/verify | unproven |

### [`RFC1994-4.1-4`](#rfc1994-4.1-4)

Whenever a Challenge packet is received, the peer MUST transmit a CHAP packet with the Code field set to 2 (Response). (Section 4.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. TestBuildCHAPResponse proves buildCHAPResponse frames Code 2, but no tagged unit proves the peer TRANSMITS a Response when a Challenge arrives (runClientAuth, session.go). The negative TestBuildCHAPResponseMalformed tests malformed-Challenge rejection, a neighbouring rule, not a violation of this one.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestBuildCHAPResponseMalformed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/session_test.go#L60) | unit/verify | unproven |
| positive | [`TestBuildCHAPResponse`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/session_test.go#L22) | unit/verify | unproven |

### [`RFC1994-4.1-5`](#rfc1994-4.1-5)

The Identifier field MUST be changed each time a Challenge is sent. (Section 4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a Challenge sent with the previous Challenge's Identifier. TestCHAPIdentifierMonotonic seeds the counter with the Identifier the first Challenge carried and asserts the next Challenge's differs (and is +1); TestCHAPIdentifierWraps asserts 0xFF moves to 0x00. A runCHAPAuthPhase that did not advance the counter turns both red. Single-polarity marker.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestCHAPIdentifierMonotonic`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/chap_test.go#L835) | unit/verify | unproven |
| positive | [`TestCHAPIdentifierWraps`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/chap_test.go#L880) | unit/verify | unproven |

### [`RFC1994-4.1-6`](#rfc1994-4.1-6)

The Response Identifier MUST be copied from the Identifier field of the Challenge which caused the Response. (Section 4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a Response whose Identifier is not the Challenge's. TestBuildCHAPResponse passes a Challenge with Identifier 0x42 and asserts resp[1] == 0x42, red on any other value; buildCHAPResponse is what session.go calls with the received Challenge. Single-polarity marker.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestBuildCHAPResponse`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/session_test.go#L24) | unit/verify | unproven |

### [`RFC1994-4.1-7`](#rfc1994-4.1-7)

The Challenge Value MUST be changed each time a Challenge is sent. (Section 4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a Challenge Value reused across Challenges. TestCHAPChallengeRandom runs runCHAPAuthPhase twice and asserts the two wire Values differ, red on a constant Value. Single-polarity marker.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestCHAPChallengeRandom`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/chap_test.go#L952) | unit/verify | unproven |

### [`RFC1994-4.2-3`](#rfc1994-4.2-3)

The Identifier field MUST be copied from the Identifier field of the Response which caused this reply. (Section 4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a Success or Failure whose Identifier is not the Response's. TestCHAPResponseEmitsEvent sends a Response carrying the Challenge Identifier and asserts the Success frame's Identifier equals it, red on any other Identifier. Single-polarity marker: waitCHAPResponse admits only a Response whose Identifier matches the outstanding Challenge.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestCHAPResponseEmitsEvent`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/chap_test.go#L388) | unit/verify | unproven |

### [`RFC1994-4.1-8`](#rfc1994-4.1-8)

Implementation Notes: Because the Success might be lost, the authenticator MUST allow repeated Response packets during the Network-Layer Protocol phase after completing the Authentication phase. (Section 4.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. TestCHAPRepeatedResponseAfterSuccessKeepsSessionUp sends a Response with Identifier 0x99 after Success, not a repeat of the Response that earned it (the current Challenge Identifier). It proves a stray Response is tolerated, not a genuinely repeated one.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestCHAPRepeatedResponseAfterSuccessKeepsSessionUp`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/chap_reauth_test.go#L407) | unit/verify | unproven |

### [`RFC1994-4.1-9`](#rfc1994-4.1-9)

To prevent discovery of alternative Names and Secrets, any Response packets received having the current Challenge Identifier MUST return the same reply Code previously returned for that specific Challenge (§4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1994-4.1-9, so no unit is bound to it.

### [`RFC1994-4.1-10`](#rfc1994-4.1-10)

Any Response packets received during any other phase MUST be silently discarded. (Section 4.1)

Audit verdict: wrong (the tests assert something other than what the requirement demands), fresh. The only tagged unit, TestCHAPIdentifierMismatchSilentDiscard, sends mismatched-Identifier Responses DURING the Authentication phase. The requirement is about Responses received in any OTHER phase (outside Authentication and the post-auth Network-Layer phase, e.g. Link Establishment). No unit sends a Response in another phase.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestCHAPIdentifierMismatchSilentDiscard`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/chap_reauth_test.go#L151) | unit/verify | unproven |

### [`RFC1994-2.3-1`](#rfc1994-2.3-1)

The CHAP algorithm requires that the length of the secret MUST be at least 1 octet. (Section 2.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1994-2.3-1, so no unit is bound to it.

### [`RFC1994-4.2-4`](#rfc1994-4.2-4)

It is intended to be human readable, and MUST NOT affect operation of the protocol. (Section 4.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. TestCHAPSuccessMessageDoesNotAffectOutcome proves two different Messages on a Success both complete authentication in runClientAuth. No case carries a Message on a Failure, so a Message that turned a Failure into success would pass.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestCHAPSuccessMessageDoesNotAffectOutcome`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1994_chap_message_test.go#L39) | unit/verify | revert, verified |

### [`RFC1994-1.1-1`](#rfc1994-1.1-1)

An implementation which does not include this option MUST be prepared to interoperate with another implementation which does include the option. (Section 1.1)

Audit verdict: wrong (the tests assert something other than what the requirement demands), fresh. The sentence is RFC 1994 Section 1.1's definition of MAY (quoted there under 'MAY'): an implementation lacking an optional feature must interoperate with one that has it. It states no CHAP obligation, so the row should be retired with its two tags. Both tagged units (TestNegotiatePeerAuthProtoAccepted, TestNegotiatePeerAuthProtoRejected) read 'option' as the LCP Auth-Protocol option and exercise NegotiatePeerOptions with a PAP option, which is not what the sentence obliges.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestNegotiatePeerAuthProtoAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/lcp_options_test.go#L211) | unit/verify | unproven |
| positive | [`TestNegotiatePeerAuthProtoRejected`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/lcp_options_test.go#L194) | unit/verify | unproven |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | rfc2119 |
| Source | rfc/full/rfc1994.txt |
| Source fingerprint | f76fecfc276d54fb |
| Record | rfc/extraction/rfc1994.json |
| Mapped sentences | 17 |
| Declined as scope | 2 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | walked | not stated |
| `1` | not stated | 1 | walked | not stated |
| `1.1` | not stated | 3 | walked | not stated |
| `1.2` | not stated | 0 | walked | not stated |
| `2` | not stated | 0 | walked | not stated |
| `2.1` | not stated | 0 | walked | not stated |
| `2.2` | not stated | 0 | walked | not stated |
| `2.3` | not stated | 1 | walked | not stated |
| `3` | not stated | 0 | walked | not stated |
| `4` | not stated | 0 | walked | not stated |
| `4.1` | not stated | 10 | walked | not stated |
| `4.2` | not stated | 4 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `1.1:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1.1 Specification of Requirements is the RFC 2119 keyword glossary; the sentence defines what MUST means and states no CHAP behaviour. | MUST This word, or the adjective "required", means that the definition is an absolute requirement of the specification. |
| `1.1:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1.1 Specification of Requirements is the RFC 2119 keyword glossary; the sentence defines what MUST NOT means and states no CHAP behaviour. | MUST NOT This phrase means that the definition is an absolute prohibition of the specification. |

## Superseded

No document obsoletes RFC 1994, so its obligations are stated where they were written.
