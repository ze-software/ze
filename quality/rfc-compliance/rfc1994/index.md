# RFC 1994 - PPP Challenge Handshake Authentication Protocol (CHAP)

PPP CHAP (MD5). Every requirement listed for RFC 1994, the tests linked to it, and their recorded reviews. This summary is included in the automatic compliance check, ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 5 | of 16 mandatory requirements | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 9 | of 16 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0 | of 16 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0 | of 16 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Tests shown to detect a fault | 25 | 25 of 41 tests linked to requirements, 0 without a fault test and 9 with outdated fault tests | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests counts as proof only after this check has demonstrated that they can detect a fault |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Declared gap or no test | 2 | of 16 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 16 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 0 | of 16 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 0 | of 16 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze proven: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 16 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 16 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no proof credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level) | 16 | of 27 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements included in the automatic checks: the compliance check (./le rfc check) holds Ze to each of them. This is the total being assessed; the counts above show the recorded status of those requirements. |

The 9 counts marked as a part above are the whole of the 16 mandatory requirements: they add up to 16. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got. An outlined card with no colored edge, under Totals, is a total the other cards are counted against, not a measure.

| Card | Tone here | Why that color |
|---|---|---|
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the count under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| Partial proof; remaining gap | ok | green at zero, RED above it: a tested clause cannot prove the whole requirement |
| Declared gap or no test | bad | green at zero, RED above it: a binding obligation that nothing shows Ze meets is owed, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green counts above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Linux cannot do this | neutral | neutral at every value: an evidenced platform limit is neither a proof nor a scope exemption |
| Tests shown to detect a fault | ok | green at every value: these tests have been shown to detect a deliberate fault. This card counts individual tests separately, so its total cannot be added to the requirement counts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a mandatory requirement is unreviewed, green when every one is judged sound and current |
| Mandatory requirements (MUST level) | outlined | outlined, with no colored edge: it is the total the other cards are counted against, not a measure, and a larger one is neither good news nor bad |

## At a glance

| Field | Value |
|---|---|
| Public row | PPP CHAP (MD5) |
| Automatic checks | Included |
| Requirements | 27 |
| Mandatory requirements (MUST level) | 16 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 2 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 41 |
| Tests linked to requirements | 41 |
| Recorded audit verdicts | 15 |
| Fault test records | 34 |
| Summary | `rfc/short/rfc1994.md` |
| Requirement table | `rfc/requirements/rfc1994.md` |
| RFC text | `rfc/full/rfc1994.txt` |

## Automatic checks

Included in automatic checks: PPP CHAP-MD5 (RFC 1994): authenticator (LNS) + peer (PPPoE client); 5 MET (auth-protocol advertise, Success/Failure per comparison, match->Success, mismatch->Failure, peer Response) + 9 single-polarity positive (Challenge Code 1, changing Identifier/Value, echoed Identifier, repeated-Response tolerance, other-phase discard, Message-independence, interop) + 3 gap (no Challenge retransmit, no repeated-Response replay, no 1-octet secret minimum)

## Published support status

**Area:** PPP CHAP (MD5)

**Proof:** 6 of 16 MUST proven

**Evidence:** unit 6

**Unmet:** RFC1994-2.3-1, RFC1994-4.1-9


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 5 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 11 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 0 | one part of the mandatory requirements |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level)** | **16** | every mandatory requirement appears in exactly one group above |

**Positive and negative tests (5):** [`RFC1994-1-1`](#rfc1994-1-1), [`RFC1994-4.1-3`](#rfc1994-4.1-3), [`RFC1994-4.2-1`](#rfc1994-4.2-1), [`RFC1994-4.2-2`](#rfc1994-4.2-2), [`RFC1994-4.1-4`](#rfc1994-4.1-4)

**Annotated (including scoped evidence) (11):** [`RFC1994-4.1-1`](#rfc1994-4.1-1), [`RFC1994-4.1-2`](#rfc1994-4.1-2), [`RFC1994-4.1-5`](#rfc1994-4.1-5), [`RFC1994-4.1-6`](#rfc1994-4.1-6), [`RFC1994-4.1-7`](#rfc1994-4.1-7), [`RFC1994-4.2-3`](#rfc1994-4.2-3), [`RFC1994-4.1-8`](#rfc1994-4.1-8), [`RFC1994-4.1-9`](#rfc1994-4.1-9), [`RFC1994-4.1-10`](#rfc1994-4.1-10), [`RFC1994-2.3-1`](#rfc1994-2.3-1), [`RFC1994-4.2-4`](#rfc1994-4.2-4)

## Figures

- **MUST bar**: Ze proven 6, Proof pending 8, Not built 2
- **Overview bar**: Ze MUST proven 6, SHOULD proven 2, Proof pending 8, Not assessed 9, Not built 2

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | not complete |
| MUST rows | 16 |
| MUST rows kept (the MUST bar) | 16 |
| Ze proven | 6 |
| OS verified | 0 |
| Proof pending | 8 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 2 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 11 |
| SHOULD proven | 2 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD proof pending | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 9 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze proven means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Proof pending counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

Source: [data/rfc/rfc1994.json](../../../data/rfc/rfc1994.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC1994-1-1` | If authentication of the link is desired, an implementation MUST specify the Authentication-Protocol Configuration Option during Link Establishment phase. (Section 1) | MUST | 1 | **positive:** `unit/verify` [`TestLocalCONFREQAdvertisesAuthMethod`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_dispatch_test.go#L694). **negative:** `unit/verify` [`TestAuthProtoRejectClearsMethod`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_dispatch_test.go#L155) |
| `RFC1994-4.1-1` | The authenticator MUST transmit a CHAP packet with the Code field set to 1 (Challenge). (Section 4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestCHAPResponseEmitsEvent`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1994_chap_test.go#L385). **negative:** no negative test. **{single-polarity}:** runCHAPAuthPhase always frames and writes a CHAP Challenge with Code=1 as its first wire act, through waitCHAPLike's first call of sendCHAPChallenge, with no must-not-challenge branch (internal/component/l2tp/ppp/chap.go:242-244, :144-146) |
| `RFC1994-4.1-2` | Additional Challenge packets MUST be sent until a valid Response packet is received, or an optional retry counter expires. (Section 4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestCHAPChallengeRetransmittedUntilResponse`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1994_chap_retransmit_test.go#L98). **negative:** no negative test. **{single-polarity}:** waitCHAPLike sends a new Challenge every restart interval until a matching Response or the chapChallengeMax retry counter, so there is no must-not-retransmit branch to negate (internal/component/l2tp/ppp/auth.go:554-572) |
| `RFC1994-4.1-3` | Based on this comparison, the authenticator MUST send a Success or Failure packet (described below). (Section 4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestCHAPResponseEmitsEvent`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1994_chap_test.go#L386). **positive:** `unit/verify` [`TestRFC1994EqualResponseValueDrawsSuccessThroughLocalVerifier`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1994_local_verifier_test.go#L168). **negative:** `unit/verify` [`TestCHAPRejectWritesFailure`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1994_chap_test.go#L521). **negative:** `unit/verify` [`TestRFC1994DifferentResponseValueDrawsFailureThroughLocalVerifier`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1994_local_verifier_test.go#L185) |
| `RFC1994-4.2-1` | If the Value received in a Response is equal to the expected value, then the implementation MUST transmit a CHAP packet with the Code field set to 3 (Success). (Section 4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestCHAPResponseEmitsEvent`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1994_chap_test.go#L387). **positive:** `unit/verify` [`TestLocalAuthCHAPMD5Accept`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authlocal/rfc1994_auth_test.go#L46). **positive:** `unit/verify` [`TestRFC1994EqualResponseValueDrawsSuccessThroughLocalVerifier`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1994_local_verifier_test.go#L166). **negative:** `unit/verify` [`TestCHAPRejectWritesFailure`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1994_chap_test.go#L522). **negative:** `unit/verify` [`TestLocalAuthCHAPMD5Reject`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authlocal/rfc1994_auth_test.go#L81). **negative:** `unit/verify` [`TestRFC1994DifferentResponseValueDrawsFailureThroughLocalVerifier`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1994_local_verifier_test.go#L184) |
| `RFC1994-4.2-2` | If the Value received in a Response is not equal to the expected value, then the implementation MUST transmit a CHAP packet with the Code field set to 4 (Failure) (§4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestCHAPRejectWritesFailure`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1994_chap_test.go#L523). **positive:** `unit/verify` [`TestLocalAuthCHAPMD5Reject`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authlocal/rfc1994_auth_test.go#L83). **positive:** `unit/verify` [`TestRFC1994DifferentResponseValueDrawsFailureThroughLocalVerifier`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1994_local_verifier_test.go#L183). **negative:** `unit/verify` [`TestLocalAuthCHAPMD5Accept`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authlocal/rfc1994_auth_test.go#L49). **negative:** `unit/verify` [`TestRFC1994EqualResponseValueDrawsSuccessThroughLocalVerifier`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1994_local_verifier_test.go#L167) |
| `RFC1994-4.1-4` | Whenever a Challenge packet is received, the peer MUST transmit a CHAP packet with the Code field set to 2 (Response). (Section 4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestBuildCHAPResponse`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/session_test.go#L22). **positive:** `unit/verify` [`TestRFC1994IPCPWindowChallengeAnswered`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1994_network_phase_test.go#L195). **positive:** `unit/verify` [`TestRFC1994NetworkPhaseChallengeAnswered`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1994_network_phase_test.go#L59). **positive:** `unit/verify` [`TestRFC1994PeerTransmitsResponseForEveryChallenge`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1994_peer_rfc_test.go#L66). **negative:** `unit/verify` [`TestBuildCHAPResponseMalformed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/session_test.go#L60). **negative:** `unit/verify` [`TestRFC1994IPCPWindowNoResponseWithoutChallenge`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1994_network_phase_test.go#L235). **negative:** `unit/verify` [`TestRFC1994NetworkPhaseNoResponseWithoutChallenge`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1994_network_phase_test.go#L102). **negative:** `unit/verify` [`TestRFC1994PeerSendsNoResponseWithoutChallenge`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1994_peer_rfc_test.go#L95) |
| `RFC1994-4.1-5` | The Identifier field MUST be changed each time a Challenge is sent. (Section 4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestCHAPIdentifierMonotonic`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1994_chap_test.go#L842). **positive:** `unit/verify` [`TestCHAPIdentifierWraps`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1994_chap_test.go#L887). **negative:** no negative test. **{single-polarity}:** sendCHAPChallenge increments the per-session chapIdentifier for every Challenge it sends, retransmissions included, so no send path reuses an Identifier to form a negative (internal/component/l2tp/ppp/chap.go:240-243) |
| `RFC1994-4.1-6` | The Response Identifier MUST be copied from the Identifier field of the Challenge which caused the Response. (Section 4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestBuildCHAPResponse`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/session_test.go#L24). **negative:** no negative test. **{single-polarity}:** the peer copies the received Challenge's Identifier byte into the Response header, asserted directly with no rejecting counterpart (internal/component/l2tp/pppoeclient/session.go:400) |
| `RFC1994-4.1-7` | The Challenge Value MUST be changed each time a Challenge is sent. (Section 4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestCHAPChallengeRandom`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1994_chap_test.go#L959). **negative:** no negative test. **{single-polarity}:** sendCHAPChallenge draws a fresh 16-octet value from crypto/rand for every Challenge, retransmissions included (internal/component/l2tp/ppp/chap.go:219-225, :235) |
| `RFC1994-4.2-3` | The Identifier field MUST be copied from the Identifier field of the Response which caused this reply. (Section 4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestCHAPRejectWritesFailure`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1994_chap_test.go#L524). **positive:** `unit/verify` [`TestCHAPResponseEmitsEvent`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1994_chap_test.go#L388). **negative:** no negative test. **{single-polarity}:** waitCHAPResponse only returns a Response whose Identifier equals the outstanding Challenge Identifier, and runCHAPAuthPhase writes Success/Failure with that same Identifier (internal/component/l2tp/ppp/chap.go:296-298, auth.go:318-323) |
| `RFC1994-4.1-8` | Implementation Notes: Because the Success might be lost, the authenticator MUST allow repeated Response packets during the Network-Layer Protocol phase after completing the Authentication phase. (Section 4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestCHAPRepeatedResponseAfterSuccessKeepsSessionUp`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1994_chap_reauth_test.go#L452). **negative:** no negative test. **{single-polarity}:** a CHAP Response arriving in the main loop after auth completes hits the frame-dispatch default and is dropped without terminating the session, so repeated Responses are tolerated (internal/component/l2tp/ppp/session_run.go:681-683) |
| `RFC1994-4.1-9` | To prevent discovery of alternative Names and Secrets, any Response packets received having the current Challenge Identifier MUST return the same reply Code previously returned for that specific Challenge (§4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze caches no per-Challenge reply Code and does not re-send the prior Success/Failure for a repeated Response; it silently drops it (internal/component/l2tp/ppp/session_run.go:681-683; no reply-Code cache in runCHAPAuthPhase) |
| `RFC1994-4.1-10` | Any Response packets received during any other phase MUST be silently discarded. (Section 4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestCHAPResponseOutsideAuthPhaseSilentlyDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1994_chap_reauth_test.go#L247). **negative:** no negative test. **{single-polarity}:** outside an active auth-wait a CHAP Response is silently dropped by the frame-dispatch default, and during a wait a Response whose Identifier does not match is silently discarded and the wait continues (internal/component/l2tp/ppp/session_run.go:681-683, auth.go:318-322) |
| `RFC1994-2.3-1` | The CHAP algorithm requires that the length of the secret MUST be at least 1 octet. (Section 2.3) | MUST | 2.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the CHAP shared secret (authlocal user password) has no minimum-length constraint, so an empty password is accepted and fed to the MD5 hash without rejection (internal/component/l2tp/plugins/authlocal/auth.go:94-99; empty password stored in register.go) |
| `RFC1994-4.2-4` | It is intended to be human readable, and MUST NOT affect operation of the protocol. (Section 4.2) | MUST NOT | 4.2 | **positive:** `unit/verify` [`TestCHAPSuccessMessageDoesNotAffectOutcome`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1994_chap_message_test.go#L39). **positive:** `unit/verify` [`TestRFC1994FailureMessageDoesNotAffectOutcome`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1994_peer_rfc_test.go#L118). **negative:** no negative test. **{single-polarity}:** the peer branches only on the Success/Failure Code (3 succeed, 4 fail) and never reads or acts on the Message field (internal/component/l2tp/pppoeclient/session.go::runClientAuth, the case 3, 4 branch) |
| `RFC1994-2-1` | If the values match, the authentication is acknowledged; otherwise the connection SHOULD be terminated. (§2) | SHOULD | 2 | **positive:** `unit/verify` [`TestRFC1994CHAPFailureTerminatesTheLink`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1994_failure_terminates_link_test.go#L98). **negative:** `unit/verify` [`TestRFC1994CHAPSuccessKeepsTheLink`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1994_failure_terminates_link_test.go#L135) |
| `RFC1994-4.2-5` | If the Value received in a Response is not equal to the expected value, then the implementation MUST transmit a CHAP packet with the Code field set to 4 (Failure), and SHOULD take action to terminate the link. (§4.2, the SHOULD clause: the authenticator takes action to terminate the link after a Failure; the MUST clause is RFC1994-4.2-2) | SHOULD | 4.2 | **positive:** `unit/verify` [`TestRFC1994CHAPFailureTerminatesTheLink`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1994_failure_terminates_link_test.go#L93). **negative:** `unit/verify` [`TestRFC1994CHAPSuccessKeepsTheLink`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1994_failure_terminates_link_test.go#L132) |
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
| [`RFC1994-4.1-9`](#rfc1994-4.1-9) To prevent discovery of alternative Names and Secrets, any Response packets received having the current Challenge Identifier MUST return the same reply Code previously returned for that specific Challenge (§4.1) | {gap}, no test | ze caches no per-Challenge reply Code and does not re-send the prior Success/Failure for a repeated Response; it silently drops it (internal/component/l2tp/ppp/session_run.go:681-683; no reply-Code cache in runCHAPAuthPhase) |
| [`RFC1994-2.3-1`](#rfc1994-2.3-1) The CHAP algorithm requires that the length of the secret MUST be at least 1 octet. (Section 2.3) | {gap}, no test | the CHAP shared secret (authlocal user password) has no minimum-length constraint, so an empty password is accepted and fed to the MD5 hash without rejection (internal/component/l2tp/plugins/authlocal/auth.go:94-99; empty password stored in register.go) |

## Proof state

A test reads unproven when no record shows it failing after a deliberate fault. Until then, its claim to check the requirement remains unproven.

### [`RFC1994-1-1`](#rfc1994-1-1)

If authentication of the link is desired, an implementation MUST specify the Authentication-Protocol Configuration Option during Link Establishment phase. (Section 1)

Audit verdict: enforced (the tests do what the requirement demands), stale-unit: internal/component/l2tp/ppp/auth_dispatch_test.go::TestAuthProtoRejectClearsMethod, internal/component/l2tp/ppp/auth_dispatch_test.go::TestLocalCONFREQAdvertisesAuthMethod moved. Independent rejudgment 2026-11-15 against RFC 1994 Section 1: "If authentication of the link is desired, an implementation MUST specify the Authentication-Protocol Configuration Option during Link Establishment phase." TestLocalCONFREQAdvertisesAuthMethod starts the actual Driver, supplies valid LCP settings and an explicit method-only fallback order, then requires its first LCP Configure-Request to contain CHAP Authentication-Protocol 0xc223 and algorithm 0x05 for CHAP-MD5 (and checks PAP/MS-CHAPv2 controls). The distinct TestAuthProtoRejectClearsMethod verifies an initial optional PAP offer, sends its exact option back in a correlated Configure-Reject and requires a new Configure-Request without Authentication-Protocol once the method becomes None; the Section 1 obligation is PPP-wide rather than CHAP-specific. Both inputs are structurally valid and reach the intended advertisement/clear producers. Re-read spawnSession, authMethodToLCPOptions, BuildLocalConfigRequest, sendConfigureRequest and adjustAuthOnNakOrReject. Added validated LCP configuration changes only admission, not either wire assertion. Current native records were retaken by the main thread; producer-disabled panic observations are not described as new semantic coverage.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestAuthProtoRejectClearsMethod`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_dispatch_test.go#L155) | unit/verify | revert, unit-changed (the tagged unit's behavior changed since the red was observed) |
| positive | [`TestLocalCONFREQAdvertisesAuthMethod`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_dispatch_test.go#L694) | unit/verify | revert, unit-changed (the tagged unit's behavior changed since the red was observed) |

### [`RFC1994-4.1-1`](#rfc1994-4.1-1)

The authenticator MUST transmit a CHAP packet with the Code field set to 1 (Challenge). (Section 4.1)

Audit verdict: enforced (the tests do what the requirement demands), stale-unit: internal/component/l2tp/ppp/rfc1994_chap_test.go::TestCHAPResponseEmitsEvent moved. Forbidden: an authenticator that does not send a Challenge (Code 1). TestCHAPResponseEmitsEvent reads the first wire frame runCHAPAuthPhase writes and asserts protocol 0xC223 and Code CHAPCodeChallenge, red otherwise. Single-polarity marker: there is no must-not-challenge branch.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestCHAPResponseEmitsEvent`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1994_chap_test.go#L385) | unit/verify | unproven |

### [`RFC1994-4.1-2`](#rfc1994-4.1-2)

Additional Challenge packets MUST be sent until a valid Response packet is received, or an optional retry counter expires. (Section 4.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestCHAPChallengeRetransmittedUntilResponse`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1994_chap_retransmit_test.go#L98) | unit/verify | mutant, unit-changed (the tagged unit's behavior changed since the red was observed) |

### [`RFC1994-4.1-3`](#rfc1994-4.1-3)

Based on this comparison, the authenticator MUST send a Success or Failure packet (described below). (Section 4.1)

Audit verdict: enforced (the tests do what the requirement demands), stale-unit: internal/component/l2tp/ppp/rfc1994_chap_test.go::TestCHAPRejectWritesFailure, internal/component/l2tp/ppp/rfc1994_chap_test.go::TestCHAPResponseEmitsEvent moved. Judge ppp4 2026-09-30. rfc1994_local_verifier_test.go (package ppp_test) drives a real LNS session over a net.Pipe: the peer computes its Response Value from a secret, the Driver's EventAuthRequest is answered by l2tp.GetAuthHandler(), the handler l2tp-auth-local registers in init (verifyCHAPMD5 against bob/secret), and the CHAP Code ze writes is read off the wire. Equal Value -> Success (+), differing Value -> Failure (-), so the comparison, not an injected decision, chooses the reply. Targeted breaks (verifyCHAPMD5 always accept / always reject) each redden the matching unit (ppp4-tb-1994-accept.log, ppp4-tb-1994-reject.log); records are runCHAPAuthPhase reverts. The chap_test.go decision-injected units stay as supplementary.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestCHAPRejectWritesFailure`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1994_chap_test.go#L521) | unit/verify | mutant, unit-changed (the tagged unit's behavior changed since the red was observed) |
| negative | [`TestRFC1994DifferentResponseValueDrawsFailureThroughLocalVerifier`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1994_local_verifier_test.go#L185) | unit/verify | mutant, verified |
| positive | [`TestCHAPResponseEmitsEvent`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1994_chap_test.go#L386) | unit/verify | mutant, unit-changed (the tagged unit's behavior changed since the red was observed) |
| positive | [`TestRFC1994EqualResponseValueDrawsSuccessThroughLocalVerifier`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1994_local_verifier_test.go#L168) | unit/verify | mutant, verified |

### [`RFC1994-4.2-1`](#rfc1994-4.2-1)

If the Value received in a Response is equal to the expected value, then the implementation MUST transmit a CHAP packet with the Code field set to 3 (Success). (Section 4.2)

Audit verdict: enforced (the tests do what the requirement demands), stale-unit: internal/component/l2tp/ppp/rfc1994_chap_test.go::TestCHAPRejectWritesFailure, internal/component/l2tp/ppp/rfc1994_chap_test.go::TestCHAPResponseEmitsEvent moved. Judge ppp4 2026-09-30. TestRFC1994EqualResponseValueDrawsSuccessThroughLocalVerifier (+): MD5(Id, configured secret, Challenge) through the registered local verifier draws Code 3 on the wire. TestRFC1994DifferentResponseValueDrawsFailureThroughLocalVerifier (-): a differing Value never draws Code 3; red under the always-accept break (ppp4-tb-1994-accept.log). This closes the earlier two-half gap: one unit now carries the Response Value to the wire Code. Records: verifyCHAPMD5 revert.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLocalAuthCHAPMD5Reject`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authlocal/rfc1994_auth_test.go#L81) | unit/verify | revert, verified |
| negative | [`TestCHAPRejectWritesFailure`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1994_chap_test.go#L522) | unit/verify | mutant, unit-changed (the tagged unit's behavior changed since the red was observed) |
| negative | [`TestRFC1994DifferentResponseValueDrawsFailureThroughLocalVerifier`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1994_local_verifier_test.go#L184) | unit/verify | revert, verified |
| positive | [`TestLocalAuthCHAPMD5Accept`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authlocal/rfc1994_auth_test.go#L46) | unit/verify | revert, verified |
| positive | [`TestCHAPResponseEmitsEvent`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1994_chap_test.go#L387) | unit/verify | mutant, unit-changed (the tagged unit's behavior changed since the red was observed) |
| positive | [`TestRFC1994EqualResponseValueDrawsSuccessThroughLocalVerifier`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1994_local_verifier_test.go#L166) | unit/verify | revert, verified |

### [`RFC1994-4.2-2`](#rfc1994-4.2-2)

If the Value received in a Response is not equal to the expected value, then the implementation MUST transmit a CHAP packet with the Code field set to 4 (Failure) (§4.2)

Audit verdict: enforced (the tests do what the requirement demands), stale-unit: internal/component/l2tp/ppp/rfc1994_chap_test.go::TestCHAPRejectWritesFailure moved. Judge ppp4 2026-09-30. TestRFC1994DifferentResponseValueDrawsFailureThroughLocalVerifier (+): a Value computed from another secret, judged by the registered l2tp-auth-local verifier, draws Code 4 on the wire. TestRFC1994EqualResponseValueDrawsSuccessThroughLocalVerifier (-): an equal Value never draws Code 4; red under the always-reject break (ppp4-tb-1994-reject.log). Records: verifyCHAPMD5 revert.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLocalAuthCHAPMD5Accept`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authlocal/rfc1994_auth_test.go#L49) | unit/verify | revert, verified |
| negative | [`TestRFC1994EqualResponseValueDrawsSuccessThroughLocalVerifier`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1994_local_verifier_test.go#L167) | unit/verify | revert, verified |
| positive | [`TestLocalAuthCHAPMD5Reject`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authlocal/rfc1994_auth_test.go#L83) | unit/verify | revert, verified |
| positive | [`TestCHAPRejectWritesFailure`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1994_chap_test.go#L523) | unit/verify | mutant, unit-changed (the tagged unit's behavior changed since the red was observed) |
| positive | [`TestRFC1994DifferentResponseValueDrawsFailureThroughLocalVerifier`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1994_local_verifier_test.go#L183) | unit/verify | revert, verified |

### [`RFC1994-4.1-4`](#rfc1994-4.1-4)

Whenever a Challenge packet is received, the peer MUST transmit a CHAP packet with the Code field set to 2 (Response). (Section 4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Rejudge 2026-11-08: TestRFC1994PeerTransmitsResponseForEveryChallenge and TestRFC1994PeerSendsNoResponseWithoutChallenge changed only by passing t to runCHAPClient, which now feeds frames through testFrameReader and the shared Identifier counter. Positive: two Challenges (0x42, 0x43) draw exactly two CHAP packets, Code 2, each with its Challenge's Identifier. Negative: Code 2 and Code 5 packets draw no CHAP packet (len == 0). The IPCP-window and network-phase positives of the earlier verdict are unchanged.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1994IPCPWindowNoResponseWithoutChallenge`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1994_network_phase_test.go#L235) | unit/verify | revert, verified |
| negative | [`TestRFC1994NetworkPhaseNoResponseWithoutChallenge`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1994_network_phase_test.go#L102) | unit/verify | revert, verified |
| negative | [`TestRFC1994PeerSendsNoResponseWithoutChallenge`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1994_peer_rfc_test.go#L95) | unit/verify | revert, verified |
| negative | [`TestBuildCHAPResponseMalformed`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/session_test.go#L60) | unit/verify | revert, verified |
| positive | [`TestRFC1994IPCPWindowChallengeAnswered`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1994_network_phase_test.go#L195) | unit/verify | revert, verified |
| positive | [`TestRFC1994NetworkPhaseChallengeAnswered`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1994_network_phase_test.go#L59) | unit/verify | revert, verified |
| positive | [`TestRFC1994PeerTransmitsResponseForEveryChallenge`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1994_peer_rfc_test.go#L66) | unit/verify | revert, verified |
| positive | [`TestBuildCHAPResponse`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/session_test.go#L22) | unit/verify | revert, verified |

### [`RFC1994-4.1-5`](#rfc1994-4.1-5)

The Identifier field MUST be changed each time a Challenge is sent. (Section 4.1)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/l2tp/ppp/rfc1994_chap_test.go::TestCHAPIdentifierMonotonic, internal/component/l2tp/ppp/rfc1994_chap_test.go::TestCHAPIdentifierWraps moved. Forbidden: a Challenge sent with the previous Challenge's Identifier. TestCHAPIdentifierMonotonic seeds the counter with the Identifier the first Challenge carried and asserts the next Challenge's differs (and is +1); TestCHAPIdentifierWraps asserts 0xFF moves to 0x00. A runCHAPAuthPhase that did not advance the counter turns both red. Single-polarity marker.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestCHAPIdentifierMonotonic`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1994_chap_test.go#L842) | unit/verify | unproven |
| positive | [`TestCHAPIdentifierWraps`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1994_chap_test.go#L887) | unit/verify | unproven |

### [`RFC1994-4.1-6`](#rfc1994-4.1-6)

The Response Identifier MUST be copied from the Identifier field of the Challenge which caused the Response. (Section 4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a Response whose Identifier is not the Challenge's. TestBuildCHAPResponse passes a Challenge with Identifier 0x42 and asserts resp[1] == 0x42, red on any other value; buildCHAPResponse is what session.go calls with the received Challenge. Single-polarity marker.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestBuildCHAPResponse`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/session_test.go#L24) | unit/verify | unproven |

### [`RFC1994-4.1-7`](#rfc1994-4.1-7)

The Challenge Value MUST be changed each time a Challenge is sent. (Section 4.1)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/l2tp/ppp/rfc1994_chap_test.go::TestCHAPChallengeRandom moved. Forbidden: a Challenge Value reused across Challenges. TestCHAPChallengeRandom runs runCHAPAuthPhase twice and asserts the two wire Values differ, red on a constant Value. Single-polarity marker.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestCHAPChallengeRandom`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1994_chap_test.go#L959) | unit/verify | unproven |

### [`RFC1994-4.2-3`](#rfc1994-4.2-3)

The Identifier field MUST be copied from the Identifier field of the Response which caused this reply. (Section 4.2)

Audit verdict: enforced (the tests do what the requirement demands), stale-unit: internal/component/l2tp/ppp/rfc1994_chap_test.go::TestCHAPRejectWritesFailure, internal/component/l2tp/ppp/rfc1994_chap_test.go::TestCHAPResponseEmitsEvent moved. Forbidden: a Success or Failure whose Identifier is not the Response's. TestCHAPResponseEmitsEvent sends a Response carrying the Challenge Identifier and asserts the Success frame's Identifier equals it, red on any other Identifier. Single-polarity marker: waitCHAPResponse admits only a Response whose Identifier matches the outstanding Challenge.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestCHAPRejectWritesFailure`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1994_chap_test.go#L524) | unit/verify | unproven |
| positive | [`TestCHAPResponseEmitsEvent`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1994_chap_test.go#L388) | unit/verify | unproven |

### [`RFC1994-4.1-8`](#rfc1994-4.1-8)

Implementation Notes: Because the Success might be lost, the authenticator MUST allow repeated Response packets during the Network-Layer Protocol phase after completing the Authentication phase. (Section 4.1)

Audit verdict: enforced (the tests do what the requirement demands), stale-unit: internal/component/l2tp/ppp/rfc1994_chap_reauth_test.go::TestCHAPRepeatedResponseAfterSuccessKeepsSessionUp moved. Independent rejudgment 2026-11-15 against RFC 1994 Section 4.1: "Because the Success might be lost, the authenticator MUST allow repeated Response packets during the Network-Layer Protocol phase after completing the Authentication phase." TestCHAPRepeatedResponseAfterSuccessKeepsSessionUp completes real LCP and accepted CHAP authentication, observes CHAP Success and the success event, then repeats the identical Response Identifier and 0x33 digest produced by readCHAPChallengeAndRespond after the session reached the network phase. It rejects every lifecycle event during the repeat and requires a subsequent LCP Echo-Reply, so a terminated or stalled session fails the scoped tolerance claim. The direct handler owns no misleading malformed-packet precondition; the auth backend deliberately accepts the stand-in digest. Read handleFrame/supportsProtocol/rejectUnsupportedProtocol: configured CHAP is tolerated without a Protocol-Reject or teardown outside the auth waiter. The single-positive annotation is justified for this repeated-input tolerance. Resending the previous CHAP reply Code is the separate RFC1994-4.1-9 obligation and remains an explicitly recorded gap; this verdict does not claim that behavior. The new valid LCP fixture leaves these assertions unchanged. No additional semantic coverage is inferred from a recorded producer-disabled panic.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestCHAPRepeatedResponseAfterSuccessKeepsSessionUp`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1994_chap_reauth_test.go#L452) | unit/verify | mutant, unit-changed (the tagged unit's behavior changed since the red was observed) |

### [`RFC1994-4.1-9`](#rfc1994-4.1-9)

To prevent discovery of alternative Names and Secrets, any Response packets received having the current Challenge Identifier MUST return the same reply Code previously returned for that specific Challenge (§4.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC1994-4.1-9, so there is no linked test.

### [`RFC1994-4.1-10`](#rfc1994-4.1-10)

Any Response packets received during any other phase MUST be silently discarded. (Section 4.1)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/l2tp/ppp/rfc1994_chap_reauth_test.go::TestCHAPResponseOutsideAuthPhaseSilentlyDiscarded moved. Single-polarity accepted. TestCHAPResponseOutsideAuthPhaseSilentlyDiscarded hands handleFrame a well-formed CHAP Response in Req-Sent, Ack-Sent (Link Establishment) and Closing (Termination): red on any frame written, any auth or session event, an end, or a state change. The auth-phase unit TestCHAPIdentifierMismatchSilentDiscard no longer claims this row. The annotation still cites the auth-wait mismatch path (auth.go) as a second example; that clause is not this row's.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestCHAPResponseOutsideAuthPhaseSilentlyDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1994_chap_reauth_test.go#L247) | unit/verify | mutant, verified |

### [`RFC1994-2.3-1`](#rfc1994-2.3-1)

The CHAP algorithm requires that the length of the secret MUST be at least 1 octet. (Section 2.3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC1994-2.3-1, so there is no linked test.

### [`RFC1994-4.2-4`](#rfc1994-4.2-4)

It is intended to be human readable, and MUST NOT affect operation of the protocol. (Section 4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Rejudge 2026-11-08: TestRFC1994FailureMessageDoesNotAffectOutcome (runCHAPClient now takes t) and TestCHAPSuccessMessageDoesNotAffectOutcome (testFrameReader, new(uint8)) changed only in plumbing. Message held fixed ('', 'authentication succeeded', non-ASCII) and Code varied: Failure returns the 'CHAP auth failed' error, Success returns nil, so a Message-driven outcome is red. Single-polarity positive per the row marker; runClientAuth still branches only on Code.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestCHAPSuccessMessageDoesNotAffectOutcome`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1994_chap_message_test.go#L39) | unit/verify | revert, verified |
| positive | [`TestRFC1994FailureMessageDoesNotAffectOutcome`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1994_peer_rfc_test.go#L118) | unit/verify | revert, verified |

### [`RFC1994-2-1`](#rfc1994-2-1)

If the values match, the authentication is acknowledged; otherwise the connection SHOULD be terminated. (§2)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/l2tp/ppp/rfc1994_failure_terminates_link_test.go::TestRFC1994CHAPFailureTerminatesTheLink, internal/component/l2tp/ppp/rfc1994_failure_terminates_link_test.go::TestRFC1994CHAPSuccessKeepsTheLink moved. Judge 2026-09-30 (access cont 11, independent). Same units and producer as RFC1994-4.2-5: values do not match (verifier rejects) -> Failure then EventSessionDown to the transport and the auth phase ends false (connection terminated); values match -> Success (acknowledged), true, no EventSessionDown. Both assertions would fail on the non-compliant behaviour (no teardown on reject; teardown on accept). Records: + fail, - runCHAPAuthPhase, observed red. The hash comparison is exercised under RFC1994-4.2-2.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1994CHAPSuccessKeepsTheLink`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1994_failure_terminates_link_test.go#L135) | unit/verify | mutant, verified |
| positive | [`TestRFC1994CHAPFailureTerminatesTheLink`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1994_failure_terminates_link_test.go#L98) | unit/verify | revert, verified |

### [`RFC1994-4.2-5`](#rfc1994-4.2-5)

If the Value received in a Response is not equal to the expected value, then the implementation MUST transmit a CHAP packet with the Code field set to 4 (Failure), and SHOULD take action to terminate the link. (§4.2, the SHOULD clause: the authenticator takes action to terminate the link after a Failure; the MUST clause is RFC1994-4.2-2)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/l2tp/ppp/rfc1994_failure_terminates_link_test.go::TestRFC1994CHAPFailureTerminatesTheLink, internal/component/l2tp/ppp/rfc1994_failure_terminates_link_test.go::TestRFC1994CHAPSuccessKeepsTheLink moved. Judge 2026-09-30 (access cont 11, independent). Producer read: chap.go runCHAPAuthPhase on a reject writes the Failure, then s.fail sends EventSessionDown (Cause NAS-Error) to the transport and returns false, so no NCP phase: the SHOULD action is taken. + TestRFC1994CHAPFailureTerminatesTheLink: reply Code 4, false, EventSessionDown 55/66 'auth rejected' NAS-Error (Fatal if absent). - TestRFC1994CHAPSuccessKeepsTheLink: Code 3, true, no lifecycle event, so the teardown is owed to the Failure alone. The Value comparison is injected through authRespCh; the comparison itself is RFC1994-4.2-2's (real local verifier). Records: + break of session_run.go::fail, - break of runCHAPAuthPhase, both observed red (whole-body panic breaks, coarse but real). Row quotes the whole 4.2 sentence at SHOULD, MUST clause stays on 4.2-2; listed in 4.2 unsourced-ids because site 4.2:2 maps to 4.2-2.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1994CHAPSuccessKeepsTheLink`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1994_failure_terminates_link_test.go#L132) | unit/verify | mutant, verified |
| positive | [`TestRFC1994CHAPFailureTerminatesTheLink`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1994_failure_terminates_link_test.go#L93) | unit/verify | revert, verified |

## Requirement list review

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
| `front` | not stated | 0 | reviewed | not stated |
| `1` | not stated | 1 | reviewed | not stated |
| `1.1` | not stated | 3 | reviewed | not stated |
| `1.2` | not stated | 0 | reviewed | not stated |
| `2` | not stated | 0 | reviewed | not stated |
| `2.1` | not stated | 0 | reviewed | not stated |
| `2.2` | not stated | 0 | reviewed | not stated |
| `2.3` | not stated | 1 | reviewed | not stated |
| `3` | not stated | 0 | reviewed | not stated |
| `4` | not stated | 0 | reviewed | not stated |
| `4.1` | not stated | 10 | reviewed | not stated |
| `4.2` | not stated | 4 | reviewed | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `1.1:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1.1 Specification of Requirements is the RFC 2119 keyword glossary; the sentence defines what MUST means and states no CHAP behaviour. | MUST This word, or the adjective "required", means that the definition is an absolute requirement of the specification. |
| `1.1:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1.1 Specification of Requirements is the RFC 2119 keyword glossary; the sentence defines what MUST NOT means and states no CHAP behaviour. | MUST NOT This phrase means that the definition is an absolute prohibition of the specification. |
| `1.1:3` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1.1 Specification of Requirements is the RFC 2119 keyword glossary; the sentence is part of the definition of MAY and states no CHAP behaviour. | An implementation which does not include this option MUST be prepared to interoperate with another implementation which does include the option. |

## Superseded

No document obsoletes RFC 1994, so its obligations are stated where they were written.
