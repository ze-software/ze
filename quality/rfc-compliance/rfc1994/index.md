# RFC 1994 - PPP Challenge Handshake Authentication Protocol (CHAP)

Partial. Every requirement this repository extracted from RFC 1994, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 31.2% | 5 of 16 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 50.0% | 8 of 16 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 16 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 16 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Proven by a recorded break | 84.6% | 33 of 39 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 16 | of 27 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 16 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 16 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 16 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 16 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 18.8% | 3 of 16 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

The 8 shares marked as a part above are the whole of the 16 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| Partial proof; remaining gap | ok | green at zero, RED above it: a tested clause cannot prove the whole requirement |
| No test at all | bad | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green shares above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Partial |
| Enrolment | Enrolled |
| Requirements | 27 |
| Gated MUST-level | 16 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 3 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 39 |
| Tagged units | 39 |
| Recorded audit verdicts | 15 |
| Discrimination records | 33 |
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
| Annotated (including scoped evidence) | 11 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **16** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (5):** [`RFC1994-1-1`](#rfc1994-1-1), [`RFC1994-4.1-3`](#rfc1994-4.1-3), [`RFC1994-4.2-1`](#rfc1994-4.2-1), [`RFC1994-4.2-2`](#rfc1994-4.2-2), [`RFC1994-4.1-4`](#rfc1994-4.1-4)

**Annotated (including scoped evidence) (11):** [`RFC1994-4.1-1`](#rfc1994-4.1-1), [`RFC1994-4.1-2`](#rfc1994-4.1-2), [`RFC1994-4.1-5`](#rfc1994-4.1-5), [`RFC1994-4.1-6`](#rfc1994-4.1-6), [`RFC1994-4.1-7`](#rfc1994-4.1-7), [`RFC1994-4.2-3`](#rfc1994-4.2-3), [`RFC1994-4.1-8`](#rfc1994-4.1-8), [`RFC1994-4.1-9`](#rfc1994-4.1-9), [`RFC1994-4.1-10`](#rfc1994-4.1-10), [`RFC1994-2.3-1`](#rfc1994-2.3-1), [`RFC1994-4.2-4`](#rfc1994-4.2-4)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC1994-1-1` | If authentication of the link is desired, an implementation MUST specify the Authentication-Protocol Configuration Option during Link Establishment phase. (Section 1) | MUST | 1 | **positive:** `unit/verify` [`TestLocalCONFREQAdvertisesAuthMethod`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_dispatch_test.go#L690). **negative:** `unit/verify` [`TestAuthProtoRejectClearsMethod`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_dispatch_test.go#L155) |
| `RFC1994-4.1-1` | The authenticator MUST transmit a CHAP packet with the Code field set to 1 (Challenge). (Section 4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestCHAPResponseEmitsEvent`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1994_chap_test.go#L385). **negative:** no negative test. **{single-polarity}:** runCHAPAuthPhase always frames and writes a CHAP Challenge with Code=1 as its first wire act, with no must-not-challenge branch (internal/component/l2tp/ppp/chap.go:259-261, :144-146) |
| `RFC1994-4.1-2` | Additional Challenge packets MUST be sent until a valid Response packet is received, or an optional retry counter expires. (Section 4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze sends exactly one Challenge and, on Response timeout, fails the session closed rather than retransmitting the same Identifier/Value (internal/component/l2tp/ppp/chap.go:257-263 single send; internal/component/l2tp/ppp/auth.go:328-337 timeout calls s.fail with no retransmit) |
| `RFC1994-4.1-3` | Based on this comparison, the authenticator MUST send a Success or Failure packet (described below). (Section 4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestCHAPResponseEmitsEvent`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1994_chap_test.go#L386). **positive:** `unit/verify` [`TestRFC1994EqualResponseValueDrawsSuccessThroughLocalVerifier`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1994_local_verifier_test.go#L170). **negative:** `unit/verify` [`TestCHAPRejectWritesFailure`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1994_chap_test.go#L521). **negative:** `unit/verify` [`TestRFC1994DifferentResponseValueDrawsFailureThroughLocalVerifier`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1994_local_verifier_test.go#L187) |
| `RFC1994-4.2-1` | If the Value received in a Response is equal to the expected value, then the implementation MUST transmit a CHAP packet with the Code field set to 3 (Success). (Section 4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestCHAPResponseEmitsEvent`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1994_chap_test.go#L387). **positive:** `unit/verify` [`TestLocalAuthCHAPMD5Accept`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authlocal/rfc1994_auth_test.go#L46). **positive:** `unit/verify` [`TestRFC1994EqualResponseValueDrawsSuccessThroughLocalVerifier`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1994_local_verifier_test.go#L168). **negative:** `unit/verify` [`TestCHAPRejectWritesFailure`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1994_chap_test.go#L522). **negative:** `unit/verify` [`TestLocalAuthCHAPMD5Reject`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authlocal/rfc1994_auth_test.go#L81). **negative:** `unit/verify` [`TestRFC1994DifferentResponseValueDrawsFailureThroughLocalVerifier`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1994_local_verifier_test.go#L186) |
| `RFC1994-4.2-2` | If the Value received in a Response is not equal to the expected value, then the implementation MUST transmit a CHAP packet with the Code field set to 4 (Failure) (§4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestCHAPRejectWritesFailure`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1994_chap_test.go#L523). **positive:** `unit/verify` [`TestLocalAuthCHAPMD5Reject`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authlocal/rfc1994_auth_test.go#L83). **positive:** `unit/verify` [`TestRFC1994DifferentResponseValueDrawsFailureThroughLocalVerifier`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1994_local_verifier_test.go#L185). **negative:** `unit/verify` [`TestLocalAuthCHAPMD5Accept`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authlocal/rfc1994_auth_test.go#L49). **negative:** `unit/verify` [`TestRFC1994EqualResponseValueDrawsSuccessThroughLocalVerifier`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1994_local_verifier_test.go#L169) |
| `RFC1994-4.1-4` | Whenever a Challenge packet is received, the peer MUST transmit a CHAP packet with the Code field set to 2 (Response). (Section 4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestBuildCHAPResponse`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/session_test.go#L22). **positive:** `unit/verify` [`TestRFC1994IPCPWindowChallengeAnswered`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1994_network_phase_test.go#L192). **positive:** `unit/verify` [`TestRFC1994NetworkPhaseChallengeAnswered`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1994_network_phase_test.go#L57). **positive:** `unit/verify` [`TestRFC1994PeerTransmitsResponseForEveryChallenge`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1994_peer_rfc_test.go#L66). **negative:** `unit/verify` [`TestBuildCHAPResponseMalformed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/session_test.go#L60). **negative:** `unit/verify` [`TestRFC1994IPCPWindowNoResponseWithoutChallenge`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1994_network_phase_test.go#L232). **negative:** `unit/verify` [`TestRFC1994NetworkPhaseNoResponseWithoutChallenge`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1994_network_phase_test.go#L100). **negative:** `unit/verify` [`TestRFC1994PeerSendsNoResponseWithoutChallenge`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1994_peer_rfc_test.go#L95) |
| `RFC1994-4.1-5` | The Identifier field MUST be changed each time a Challenge is sent. (Section 4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestCHAPIdentifierMonotonic`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1994_chap_test.go#L835). **positive:** `unit/verify` [`TestCHAPIdentifierWraps`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1994_chap_test.go#L880). **negative:** no negative test. **{single-polarity}:** each runCHAPAuthPhase increments the per-session chapIdentifier before sending, so every new Challenge carries a distinct Identifier, and ze never retransmits a Challenge to form a reuse negative (internal/component/l2tp/ppp/chap.go:254-255) |
| `RFC1994-4.1-6` | The Response Identifier MUST be copied from the Identifier field of the Challenge which caused the Response. (Section 4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestBuildCHAPResponse`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/session_test.go#L24). **negative:** no negative test. **{single-polarity}:** the peer copies the received Challenge's Identifier byte into the Response header, asserted directly with no rejecting counterpart (internal/component/l2tp/pppoeclient/session.go:400) |
| `RFC1994-4.1-7` | The Challenge Value MUST be changed each time a Challenge is sent. (Section 4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestCHAPChallengeRandom`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1994_chap_test.go#L952). **negative:** no negative test. **{single-polarity}:** runCHAPAuthPhase draws a fresh 16-octet value from crypto/rand for every Challenge (internal/component/l2tp/ppp/chap.go:219-225, :248-249) |
| `RFC1994-4.2-3` | The Identifier field MUST be copied from the Identifier field of the Response which caused this reply. (Section 4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestCHAPResponseEmitsEvent`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1994_chap_test.go#L388). **negative:** no negative test. **{single-polarity}:** waitCHAPResponse only returns a Response whose Identifier equals the outstanding Challenge Identifier, and runCHAPAuthPhase writes Success/Failure with that same Identifier (internal/component/l2tp/ppp/chap.go:296-298, auth.go:318-323) |
| `RFC1994-4.1-8` | Implementation Notes: Because the Success might be lost, the authenticator MUST allow repeated Response packets during the Network-Layer Protocol phase after completing the Authentication phase. (Section 4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestCHAPRepeatedResponseAfterSuccessKeepsSessionUp`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1994_chap_reauth_test.go#L446). **negative:** no negative test. **{single-polarity}:** a CHAP Response arriving in the main loop after auth completes hits the frame-dispatch default and is dropped without terminating the session, so repeated Responses are tolerated (internal/component/l2tp/ppp/session_run.go:681-683) |
| `RFC1994-4.1-9` | To prevent discovery of alternative Names and Secrets, any Response packets received having the current Challenge Identifier MUST return the same reply Code previously returned for that specific Challenge (§4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze caches no per-Challenge reply Code and does not re-send the prior Success/Failure for a repeated Response; it silently drops it (internal/component/l2tp/ppp/session_run.go:681-683; no reply-Code cache in runCHAPAuthPhase) |
| `RFC1994-4.1-10` | Any Response packets received during any other phase MUST be silently discarded. (Section 4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestCHAPResponseOutsideAuthPhaseSilentlyDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1994_chap_reauth_test.go#L239). **negative:** no negative test. **{single-polarity}:** outside an active auth-wait a CHAP Response is silently dropped by the frame-dispatch default, and during a wait a Response whose Identifier does not match is silently discarded and the wait continues (internal/component/l2tp/ppp/session_run.go:681-683, auth.go:318-322) |
| `RFC1994-2.3-1` | The CHAP algorithm requires that the length of the secret MUST be at least 1 octet. (Section 2.3) | MUST | 2.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the CHAP shared secret (authlocal user password) has no minimum-length constraint, so an empty password is accepted and fed to the MD5 hash without rejection (internal/component/l2tp/plugins/authlocal/auth.go:94-99; empty password stored in register.go) |
| `RFC1994-4.2-4` | It is intended to be human readable, and MUST NOT affect operation of the protocol. (Section 4.2) | MUST NOT | 4.2 | **positive:** `unit/verify` [`TestCHAPSuccessMessageDoesNotAffectOutcome`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1994_chap_message_test.go#L39). **positive:** `unit/verify` [`TestRFC1994FailureMessageDoesNotAffectOutcome`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1994_peer_rfc_test.go#L118). **negative:** no negative test. **{single-polarity}:** the peer branches only on the Success/Failure Code (3 succeed, 4 fail) and never reads or acts on the Message field (internal/component/l2tp/pppoeclient/session.go::runClientAuth, the case 3, 4 branch) |
| `RFC1994-2-1` | If the values match, the authentication is acknowledged; otherwise the connection SHOULD be terminated. (§2) | SHOULD | 2 | **positive:** `unit/verify` [`TestRFC1994CHAPFailureTerminatesTheLink`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1994_failure_terminates_link_test.go#L92). **negative:** `unit/verify` [`TestRFC1994CHAPSuccessKeepsTheLink`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1994_failure_terminates_link_test.go#L129) |
| `RFC1994-4.2-5` | If the Value received in a Response is not equal to the expected value, then the implementation MUST transmit a CHAP packet with the Code field set to 4 (Failure), and SHOULD take action to terminate the link. (§4.2, the SHOULD clause: the authenticator takes action to terminate the link after a Failure; the MUST clause is RFC1994-4.2-2) | SHOULD | 4.2 | **positive:** `unit/verify` [`TestRFC1994CHAPFailureTerminatesTheLink`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1994_failure_terminates_link_test.go#L87). **negative:** `unit/verify` [`TestRFC1994CHAPSuccessKeepsTheLink`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1994_failure_terminates_link_test.go#L126) |
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

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-read after both units gained AuthFallbackOrder pinned to the configured method; the asserted behaviour is unchanged. TestLocalCONFREQAdvertisesAuthMethod asserts CHAP-MD5 configured puts Auth-Protocol 0xC223 algorithm 0x05 in the driver CONFREQ; TestAuthProtoRejectClearsMethod asserts the option is omitted once the peer Rejects it and the method becomes None. Both observed red under recorded breaks (authMethodToLCPOptions, adjustAuthOnNakOrReject).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestAuthProtoRejectClearsMethod`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_dispatch_test.go#L155) | unit/verify | revert, verified |
| positive | [`TestLocalCONFREQAdvertisesAuthMethod`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_dispatch_test.go#L690) | unit/verify | revert, verified |

### [`RFC1994-4.1-1`](#rfc1994-4.1-1)

The authenticator MUST transmit a CHAP packet with the Code field set to 1 (Challenge). (Section 4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: an authenticator that does not send a Challenge (Code 1). TestCHAPResponseEmitsEvent reads the first wire frame runCHAPAuthPhase writes and asserts protocol 0xC223 and Code CHAPCodeChallenge, red otherwise. Single-polarity marker: there is no must-not-challenge branch.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestCHAPResponseEmitsEvent`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1994_chap_test.go#L385) | unit/verify | unproven |

### [`RFC1994-4.1-2`](#rfc1994-4.1-2)

Additional Challenge packets MUST be sent until a valid Response packet is received, or an optional retry counter expires. (Section 4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1994-4.1-2, so no unit is bound to it.

### [`RFC1994-4.1-3`](#rfc1994-4.1-3)

Based on this comparison, the authenticator MUST send a Success or Failure packet (described below). (Section 4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge ppp4 2026-09-30. rfc1994_local_verifier_test.go (package ppp_test) drives a real LNS session over a net.Pipe: the peer computes its Response Value from a secret, the Driver's EventAuthRequest is answered by l2tp.GetAuthHandler(), the handler l2tp-auth-local registers in init (verifyCHAPMD5 against bob/secret), and the CHAP Code ze writes is read off the wire. Equal Value -> Success (+), differing Value -> Failure (-), so the comparison, not an injected decision, chooses the reply. Targeted breaks (verifyCHAPMD5 always accept / always reject) each redden the matching unit (ppp4-tb-1994-accept.log, ppp4-tb-1994-reject.log); records are runCHAPAuthPhase reverts. The chap_test.go decision-injected units stay as supplementary.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestCHAPRejectWritesFailure`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1994_chap_test.go#L521) | unit/verify | revert, verified |
| negative | [`TestRFC1994DifferentResponseValueDrawsFailureThroughLocalVerifier`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1994_local_verifier_test.go#L187) | unit/verify | revert, verified |
| positive | [`TestCHAPResponseEmitsEvent`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1994_chap_test.go#L386) | unit/verify | revert, verified |
| positive | [`TestRFC1994EqualResponseValueDrawsSuccessThroughLocalVerifier`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1994_local_verifier_test.go#L170) | unit/verify | revert, verified |

### [`RFC1994-4.2-1`](#rfc1994-4.2-1)

If the Value received in a Response is equal to the expected value, then the implementation MUST transmit a CHAP packet with the Code field set to 3 (Success). (Section 4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge ppp4 2026-09-30. TestRFC1994EqualResponseValueDrawsSuccessThroughLocalVerifier (+): MD5(Id, configured secret, Challenge) through the registered local verifier draws Code 3 on the wire. TestRFC1994DifferentResponseValueDrawsFailureThroughLocalVerifier (-): a differing Value never draws Code 3; red under the always-accept break (ppp4-tb-1994-accept.log). This closes the earlier two-half gap: one unit now carries the Response Value to the wire Code. Records: verifyCHAPMD5 revert.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLocalAuthCHAPMD5Reject`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authlocal/rfc1994_auth_test.go#L81) | unit/verify | revert, verified |
| negative | [`TestCHAPRejectWritesFailure`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1994_chap_test.go#L522) | unit/verify | revert, verified |
| negative | [`TestRFC1994DifferentResponseValueDrawsFailureThroughLocalVerifier`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1994_local_verifier_test.go#L186) | unit/verify | revert, verified |
| positive | [`TestLocalAuthCHAPMD5Accept`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authlocal/rfc1994_auth_test.go#L46) | unit/verify | revert, verified |
| positive | [`TestCHAPResponseEmitsEvent`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1994_chap_test.go#L387) | unit/verify | revert, verified |
| positive | [`TestRFC1994EqualResponseValueDrawsSuccessThroughLocalVerifier`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1994_local_verifier_test.go#L168) | unit/verify | revert, verified |

### [`RFC1994-4.2-2`](#rfc1994-4.2-2)

If the Value received in a Response is not equal to the expected value, then the implementation MUST transmit a CHAP packet with the Code field set to 4 (Failure) (§4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge ppp4 2026-09-30. TestRFC1994DifferentResponseValueDrawsFailureThroughLocalVerifier (+): a Value computed from another secret, judged by the registered l2tp-auth-local verifier, draws Code 4 on the wire. TestRFC1994EqualResponseValueDrawsSuccessThroughLocalVerifier (-): an equal Value never draws Code 4; red under the always-reject break (ppp4-tb-1994-reject.log). Records: verifyCHAPMD5 revert.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLocalAuthCHAPMD5Accept`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authlocal/rfc1994_auth_test.go#L49) | unit/verify | revert, verified |
| negative | [`TestRFC1994EqualResponseValueDrawsSuccessThroughLocalVerifier`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1994_local_verifier_test.go#L169) | unit/verify | revert, verified |
| positive | [`TestLocalAuthCHAPMD5Reject`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authlocal/rfc1994_auth_test.go#L83) | unit/verify | revert, verified |
| positive | [`TestCHAPRejectWritesFailure`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1994_chap_test.go#L523) | unit/verify | revert, verified |
| positive | [`TestRFC1994DifferentResponseValueDrawsFailureThroughLocalVerifier`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1994_local_verifier_test.go#L185) | unit/verify | revert, verified |

### [`RFC1994-4.1-4`](#rfc1994-4.1-4)

Whenever a Challenge packet is received, the peer MUST transmit a CHAP packet with the Code field set to 2 (Response). (Section 4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge ppp8 2026-09-30 (rejudge). The D-8 defect is fixed at the producer: networkPhaseCHAP.handle (network_phase.go) builds the Response through buildCHAPResponse (Code 2, the Challenge's Identifier, Value-Size 16, MD5(Identifier||secret||Challenge value), Name) and is called from negotiateIPCP (the Network-Layer phase opens with IPCP) and then keepaliveLoop, the same instance carrying a pending Response across; no window between runClientAuth and keepaliveLoop reads frames elsewhere. Every phase in which RFC 1994 Section 4.1 says the peer SHOULD expect a Challenge now has a positive: Authentication (TestRFC1994PeerTransmitsResponseForEveryChallenge), IPCP (TestRFC1994IPCPWindowChallengeAnswered), keepalive (TestRFC1994NetworkPhaseChallengeAnswered), each asserting exactly one Code 2 packet with the Identifier and the exact MD5 Value and Name; the two new positives went red naturally on the unfixed loops (ppp6-red, ppp7-red logs) and carry revert records. Negatives in each phase assert no CHAP packet is written for Code 2 and Code 5 packets, exact len==0 checks. session_test.go units stay supplementary (the malformed-Challenge negative proves a neighbouring discard rule). A Challenge during Link Establishment is correctly discarded under RFC 1661 and is not in scope of this row.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1994IPCPWindowNoResponseWithoutChallenge`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1994_network_phase_test.go#L232) | unit/verify | revert, verified |
| negative | [`TestRFC1994NetworkPhaseNoResponseWithoutChallenge`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1994_network_phase_test.go#L100) | unit/verify | revert, verified |
| negative | [`TestRFC1994PeerSendsNoResponseWithoutChallenge`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1994_peer_rfc_test.go#L95) | unit/verify | revert, verified |
| negative | [`TestBuildCHAPResponseMalformed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/session_test.go#L60) | unit/verify | revert, verified |
| positive | [`TestRFC1994IPCPWindowChallengeAnswered`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1994_network_phase_test.go#L192) | unit/verify | revert, verified |
| positive | [`TestRFC1994NetworkPhaseChallengeAnswered`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1994_network_phase_test.go#L57) | unit/verify | revert, verified |
| positive | [`TestRFC1994PeerTransmitsResponseForEveryChallenge`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1994_peer_rfc_test.go#L66) | unit/verify | revert, verified |
| positive | [`TestBuildCHAPResponse`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/session_test.go#L22) | unit/verify | revert, verified |

### [`RFC1994-4.1-5`](#rfc1994-4.1-5)

The Identifier field MUST be changed each time a Challenge is sent. (Section 4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a Challenge sent with the previous Challenge's Identifier. TestCHAPIdentifierMonotonic seeds the counter with the Identifier the first Challenge carried and asserts the next Challenge's differs (and is +1); TestCHAPIdentifierWraps asserts 0xFF moves to 0x00. A runCHAPAuthPhase that did not advance the counter turns both red. Single-polarity marker.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestCHAPIdentifierMonotonic`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1994_chap_test.go#L835) | unit/verify | unproven |
| positive | [`TestCHAPIdentifierWraps`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1994_chap_test.go#L880) | unit/verify | unproven |

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
| positive | [`TestCHAPChallengeRandom`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1994_chap_test.go#L952) | unit/verify | unproven |

### [`RFC1994-4.2-3`](#rfc1994-4.2-3)

The Identifier field MUST be copied from the Identifier field of the Response which caused this reply. (Section 4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a Success or Failure whose Identifier is not the Response's. TestCHAPResponseEmitsEvent sends a Response carrying the Challenge Identifier and asserts the Success frame's Identifier equals it, red on any other Identifier. Single-polarity marker: waitCHAPResponse admits only a Response whose Identifier matches the outstanding Challenge.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestCHAPResponseEmitsEvent`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1994_chap_test.go#L388) | unit/verify | unproven |

### [`RFC1994-4.1-8`](#rfc1994-4.1-8)

Implementation Notes: Because the Success might be lost, the authenticator MUST allow repeated Response packets during the Network-Layer Protocol phase after completing the Authentication phase. (Section 4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Single-polarity accepted. TestCHAPRepeatedResponseAfterSuccessKeepsSessionUp now repeats the Response that earned the Success (challenge Identifier from readCHAPChallengeAndRespond, same 0x33 Value) in the Network-Layer phase: red on EventSessionDown or any lifecycle event, and the session must still answer an LCP Echo-Request. Re-sending the prior reply Code is RFC1994-4.1-9, a recorded gap.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestCHAPRepeatedResponseAfterSuccessKeepsSessionUp`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1994_chap_reauth_test.go#L446) | unit/verify | revert, verified |

### [`RFC1994-4.1-9`](#rfc1994-4.1-9)

To prevent discovery of alternative Names and Secrets, any Response packets received having the current Challenge Identifier MUST return the same reply Code previously returned for that specific Challenge (§4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1994-4.1-9, so no unit is bound to it.

### [`RFC1994-4.1-10`](#rfc1994-4.1-10)

Any Response packets received during any other phase MUST be silently discarded. (Section 4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Single-polarity accepted. TestCHAPResponseOutsideAuthPhaseSilentlyDiscarded hands handleFrame a well-formed CHAP Response in Req-Sent, Ack-Sent (Link Establishment) and Closing (Termination): red on any frame written, any auth or session event, an end, or a state change. The auth-phase unit TestCHAPIdentifierMismatchSilentDiscard no longer claims this row. The annotation still cites the auth-wait mismatch path (auth.go) as a second example; that clause is not this row's.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestCHAPResponseOutsideAuthPhaseSilentlyDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1994_chap_reauth_test.go#L239) | unit/verify | revert, verified |

### [`RFC1994-2.3-1`](#rfc1994-2.3-1)

The CHAP algorithm requires that the length of the secret MUST be at least 1 octet. (Section 2.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1994-2.3-1, so no unit is bound to it.

### [`RFC1994-4.2-4`](#rfc1994-4.2-4)

It is intended to be human readable, and MUST NOT affect operation of the protocol. (Section 4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge ppp5 2026-09-30. The Message binds the receiver of Success/Failure, the pppoeclient peer (runClientAuth); the reasoning holds. TestRFC1994FailureMessageDoesNotAffectOutcome holds the Message fixed ('', 'authentication succeeded', non-ASCII) and varies only the Code: Failure returns the 'CHAP auth failed' error (not the closed-channel error an ignored Failure would give), Success returns nil, so a Message that turned a Failure into success, or a Success into failure, is red (recorded runClientAuth revert). TestCHAPSuccessMessageDoesNotAffectOutcome kept. Single-polarity positive per the row marker, whose line citation now names runClientAuth.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestCHAPSuccessMessageDoesNotAffectOutcome`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1994_chap_message_test.go#L39) | unit/verify | revert, verified |
| positive | [`TestRFC1994FailureMessageDoesNotAffectOutcome`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1994_peer_rfc_test.go#L118) | unit/verify | revert, verified |

### [`RFC1994-2-1`](#rfc1994-2-1)

If the values match, the authentication is acknowledged; otherwise the connection SHOULD be terminated. (§2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge 2026-09-30 (access cont 11, independent). Same units and producer as RFC1994-4.2-5: values do not match (verifier rejects) -> Failure then EventSessionDown to the transport and the auth phase ends false (connection terminated); values match -> Success (acknowledged), true, no EventSessionDown. Both assertions would fail on the non-compliant behaviour (no teardown on reject; teardown on accept). Records: + fail, - runCHAPAuthPhase, observed red. The hash comparison is exercised under RFC1994-4.2-2.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1994CHAPSuccessKeepsTheLink`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1994_failure_terminates_link_test.go#L129) | unit/verify | revert, verified |
| positive | [`TestRFC1994CHAPFailureTerminatesTheLink`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1994_failure_terminates_link_test.go#L92) | unit/verify | revert, verified |

### [`RFC1994-4.2-5`](#rfc1994-4.2-5)

If the Value received in a Response is not equal to the expected value, then the implementation MUST transmit a CHAP packet with the Code field set to 4 (Failure), and SHOULD take action to terminate the link. (§4.2, the SHOULD clause: the authenticator takes action to terminate the link after a Failure; the MUST clause is RFC1994-4.2-2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge 2026-09-30 (access cont 11, independent). Producer read: chap.go runCHAPAuthPhase on a reject writes the Failure, then s.fail sends EventSessionDown (Cause NAS-Error) to the transport and returns false, so no NCP phase: the SHOULD action is taken. + TestRFC1994CHAPFailureTerminatesTheLink: reply Code 4, false, EventSessionDown 55/66 'auth rejected' NAS-Error (Fatal if absent). - TestRFC1994CHAPSuccessKeepsTheLink: Code 3, true, no lifecycle event, so the teardown is owed to the Failure alone. The Value comparison is injected through authRespCh; the comparison itself is RFC1994-4.2-2's (real local verifier). Records: + break of session_run.go::fail, - break of runCHAPAuthPhase, both observed red (whole-body panic breaks, coarse but real). Row quotes the whole 4.2 sentence at SHOULD, MUST clause stays on 4.2-2; listed in 4.2 unsourced-ids because site 4.2:2 maps to 4.2-2.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1994CHAPSuccessKeepsTheLink`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1994_failure_terminates_link_test.go#L126) | unit/verify | revert, verified |
| positive | [`TestRFC1994CHAPFailureTerminatesTheLink`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1994_failure_terminates_link_test.go#L87) | unit/verify | revert, verified |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-29 |
| Register | rfc2119 |
| Source | rfc/full/rfc1994.txt |
| Source fingerprint | f76fecfc276d54fb |
| Record | rfc/extraction/rfc1994.json |
| Mapped sentences | 16 |
| Declined as scope | 3 |
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
| `1.1:3` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1.1 Specification of Requirements is the RFC 2119 keyword glossary; the sentence is part of the definition of MAY and states no CHAP behaviour. | An implementation which does not include this option MUST be prepared to interoperate with another implementation which does include the option. |

## Superseded

No document obsoletes RFC 1994, so its obligations are stated where they were written.
