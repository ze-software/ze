# RFC 1334 - PPP Authentication Protocols

Partial. Every requirement this repository extracted from RFC 1334, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 100.0% | 13 of 13 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 13 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 13 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 13 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| No test at all | 0.0% | 0 of 13 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 50.0% | 16 of 32 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 13 | of 16 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 13 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 13 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 13 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 13 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

The 8 shares marked as a part above are the whole of the 13 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| Partial proof; remaining gap | ok | green at zero, RED above it: a tested clause cannot prove the whole requirement |
| No test at all | ok | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
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
| Requirements | 16 |
| Gated MUST-level | 13 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 32 |
| Tagged units | 32 |
| Recorded audit verdicts | 10 |
| Discrimination records | 16 |
| Summary | `rfc/short/rfc1334.md` |
| Requirement shard | `rfc/requirements/rfc1334.md` |
| RFC text | `rfc/full/rfc1334.txt` |

## Enrolment

Enrolled: PAP and LCP authentication are implemented by the shared PPP authenticator and PPPoE client. PAP Authenticate-Request retries, changing Identifiers, matching reply validation and post-authentication reanswers have source changes and regression tests; the 2026-09-21 source batch has not run their verification or discrimination proofs.

## What the public ledger says

**Status:** Partial

**What the ledger says is covered:**

PAP authentication option through PPP auth handling.

**What the ledger says remains**

Verification remains for the retry and post-authentication reanswer changes in [`plan/immediate/spec-pppoe-client-pap-retry.md`](https://github.com/ze-software/ze/blob/main/plan/immediate/spec-pppoe-client-pap-retry.md) and [`plan/immediate/spec-ppp-pap-reanswer-after-auth.md`](https://github.com/ze-software/ze/blob/main/plan/immediate/spec-ppp-pap-reanswer-after-auth.md). The source batch adds tests for [`RFC1334-2.2.1-2`](#rfc1334-2.2.1-2), [`RFC1334-2.2.1-4`](#rfc1334-2.2.1-4), [`RFC1334-2.2.1-5`](#rfc1334-2.2.1-5), [`RFC1334-2.2-1`](#rfc1334-2.2-1) and [`RFC1334-2.3-3`](#rfc1334-2.3-3); no new test result or discrimination proof is claimed.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 13 | one part of the gated population |
| Annotated (including scoped evidence) | 0 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **13** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (13):** [`RFC1334-1-1`](#rfc1334-1-1), [`RFC1334-x-1`](#rfc1334-x-1), [`RFC1334-2.3-1`](#rfc1334-2.3-1), [`RFC1334-2.3-2`](#rfc1334-2.3-2), [`RFC1334-2.2.1-1`](#rfc1334-2.2.1-1), [`RFC1334-2.2.1-2`](#rfc1334-2.2.1-2), [`RFC1334-2.2.1-3`](#rfc1334-2.2.1-3), [`RFC1334-2.2.1-4`](#rfc1334-2.2.1-4), [`RFC1334-2.2.1-5`](#rfc1334-2.2.1-5), [`RFC1334-2.2.1-6`](#rfc1334-2.2.1-6), [`RFC1334-2.2-1`](#rfc1334-2.2-1), [`RFC1334-2.3-3`](#rfc1334-2.3-3), [`RFC1334-2.3-4`](#rfc1334-2.3-4)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC1334-1-1` | If authentication of the link is desired, an implementation MUST specify the Authentication-Protocol Configuration Option during Link Establishment phase. (Section 1) | MUST | 1 | **positive:** `unit/verify` [`TestLocalCONFREQAdvertisesAuthMethod`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_dispatch_test.go#L675). **negative:** `unit/verify` [`TestAuthProtoRejectClearsMethod`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_dispatch_test.go#L148) |
| `RFC1334-x-1` | Any implementations which include a stronger authentication method (such as CHAP, described below) MUST offer to negotiate that method prior to PAP. (§2) | MUST | 2 | **positive:** `unit/verify` [`TestAuthOffersCHAPBeforePAP`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1334_clauses_test.go#L49). **positive:** `unit/verify` [`TestConfiguredPAPStillOffersCHAPFirst`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1334_clauses_test.go#L97). **positive:** `unit/verify` [`TestDefaultAuthFallbackOrder`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1334_auth_test.go#L292). **negative:** `unit/verify` [`TestAuthOffersCHAPBeforePAP`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1334_clauses_test.go#L50). **negative:** `unit/verify` [`TestConfiguredPAPStillOffersCHAPFirst`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1334_clauses_test.go#L98). **negative:** `unit/verify` [`TestSelectAuthFallback`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1334_auth_test.go#L134) |
| `RFC1334-2.3-1` | If the Peer-ID/Password pair received in an Authenticate-Request is both recognizable and acceptable, then the authenticator MUST transmit a PAP packet with the Code field set to 2 (Authenticate- Ack). (Section 2.3) | MUST | 2.3 | **positive:** `unit/verify` [`TestPAPRequestEmitsEvent`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1334_pap_test.go#L293). **negative:** `unit/verify` [`TestPAPRejectWritesNak`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1334_pap_test.go#L411) |
| `RFC1334-2.3-2` | If the Peer-ID/Password pair received in a Authenticate-Request is not recognizable or acceptable, then the authenticator MUST transmit a PAP packet with the Code field set to 3 (Authenticate- Nak) (§2.2.2) | MUST | 2.2.2 | **positive:** `unit/verify` [`TestPAPRejectWritesNak`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1334_pap_test.go#L407). **negative:** `unit/verify` [`TestPAPRequestEmitsEvent`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1334_pap_test.go#L297) |
| `RFC1334-2.2.1-1` | The link peer MUST transmit a PAP packet with the Code field set to 1 (Authenticate-Request) during the Authentication phase (Section 2.2.1) | MUST | 2.2.1 | **positive:** `unit/verify` [`TestClientOpensAuthPhaseWithPAPRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1334_pap_request_test.go#L52). **negative:** `unit/verify` [`TestClientOpensAuthPhaseWithPAPRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1334_pap_request_test.go#L53) |
| `RFC1334-2.2.1-2` | The Authenticate-Request packet MUST be repeated until a valid reply packet is received, or an optional retry counter expires (Section 2.2.1) | MUST | 2.2.1 | **positive:** `unit/verify` [`TestClientPAPRetriesUntilMatchingReply`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1334_pap_request_test.go#L125). **negative:** `unit/verify` [`TestClientPAPRetriesUntilMatchingReply`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1334_pap_request_test.go#L126) |
| `RFC1334-2.2.1-3` | Upon reception of an Authenticate-Request packet, some type of Authenticate reply (described below) MUST be returned. (Section 2.2.1) | MUST | 2.2.1 | **positive:** `unit/verify` [`TestPAPRequestInAuthPhaseIsAnswered`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_phase_rfc1334_test.go#L74). **negative:** `unit/verify` [`TestPAPRejectedRequestIsStillAnswered`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_phase_rfc1334_test.go#L92) |
| `RFC1334-2.2.1-4` | Implementation Note: Because the Authenticate-Ack might be lost, the authenticator MUST allow repeated Authenticate- Request packets after completing the Authentication phase. (Section 2.2.1) | MUST | 2.2.1 | **positive:** `unit/verify` [`TestPAPReanswersAfterAuthentication`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_phase_rfc1334_test.go#L144). **negative:** `unit/verify` [`TestPAPReanswerDiscardsMalformedRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_phase_rfc1334_test.go#L205) |
| `RFC1334-2.2.1-5` | Protocol phase MUST return the same reply Code returned when the Authentication phase completed (§2.2.1) | MUST | 2.2.1 | **positive:** `unit/verify` [`TestPAPReanswersAfterAuthentication`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_phase_rfc1334_test.go#L145). **negative:** `unit/verify` [`TestPAPReanswerPreservesDecision`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_phase_rfc1334_test.go#L178) |
| `RFC1334-2.2.1-6` | Any Authenticate-Request packets received during any other phase MUST be silently discarded (Section 2.2.1) | MUST | 2.2.1 | **positive:** `unit/verify` [`TestPAPRequestOutsideAuthPhaseIsSilentlyDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_phase_rfc1334_test.go#L109). **negative:** `unit/verify` [`TestPAPRequestInAuthPhaseIsAnswered`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_phase_rfc1334_test.go#L75) |
| `RFC1334-2.2-1` | The Identifier field MUST be changed each time an Authenticate-Request packet is issued. (Section 2.2.1) | MUST | 2.2.1 | **positive:** `unit/verify` [`TestClientPAPRetriesUntilMatchingReply`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1334_pap_request_test.go#L127). **negative:** `unit/verify` [`TestClientPAPRetriesUntilMatchingReply`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1334_pap_request_test.go#L128) |
| `RFC1334-2.3-3` | The Identifier field MUST be copied from the Identifier field of the Authenticate-Request which caused this reply. (Section 2.2.2) | MUST | 2.2.2 | **positive:** `unit/verify` [`TestPAPReanswersAfterAuthentication`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_phase_rfc1334_test.go#L146). **positive:** `unit/verify` [`TestPAPRejectWritesNak`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1334_pap_test.go#L414). **positive:** `unit/verify` [`TestPAPRequestEmitsEvent`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1334_pap_test.go#L300). **negative:** `unit/verify` [`TestPAPReanswersAfterAuthentication`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_phase_rfc1334_test.go#L147) |
| `RFC1334-2.3-4` | It is intended to be human readable, and MUST NOT affect operation of the protocol. (Section 2.3) | MUST NOT | 2.3 | **positive:** `unit/verify` [`TestPAPReplyMessageDoesNotAffectOutcome`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1334_pap_message_test.go#L55). **negative:** `unit/verify` [`TestPAPReplyMessageDoesNotAffectOutcome`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1334_pap_message_test.go#L56) |
| `RFC1334-2.3-5` | If the Peer-ID/Password pair received in a Authenticate-Request is not recognizable or acceptable, then the authenticator MUST transmit a PAP packet with the Code field set to 3 (Authenticate- Nak), and SHOULD take action to terminate the link. (§2.2.2) | SHOULD | 2.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC1334-2.3-6` | The Message field is zero or more octets, and its contents are implementation dependent. (§2.2.2) | MAY | 2.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC1334-2.2-2` | The peer is in control of the frequency and timing of the attempts. (§2) | MAY | 2 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

RFC 1334 declares no gap, and every gated MUST it carries has a test bound to it.

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC1334-1-1`](#rfc1334-1-1)

If authentication of the link is desired, an implementation MUST specify the Authentication-Protocol Configuration Option during Link Establishment phase. (Section 1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-read after both units gained AuthFallbackOrder pinned to the configured method (the RFC 1334 Section 2 fix would otherwise put CHAP first); the asserted behaviour is unchanged. TestLocalCONFREQAdvertisesAuthMethod asserts the Auth-Protocol option and its value in the first driver CONFREQ; TestAuthProtoRejectClearsMethod asserts it is absent once the peer Rejects it and authentication is no longer desired. Both observed red under recorded breaks (authMethodToLCPOptions, adjustAuthOnNakOrReject).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestAuthProtoRejectClearsMethod`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_dispatch_test.go#L148) | unit/verify | revert, verified |
| positive | [`TestLocalCONFREQAdvertisesAuthMethod`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_dispatch_test.go#L675) | unit/verify | revert, verified |

### [`RFC1334-x-1`](#rfc1334-x-1)

Any implementations which include a stronger authentication method (such as CHAP, described below) MUST offer to negotiate that method prior to PAP. (§2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Both entry points now proven. TestConfiguredPAPStillOffersCHAPFirst starts a session through Driver/spawnSession with AuthMethod PAP and the default order: the first Configure-Request must offer CHAP 0xc223 (red on the old copy of StartSession.AuthMethod, observed before the fix and recorded against initialAuthMethod), PAP only after the peer's PAP Nak; the PAP-alone control keeps PAP first where no stronger method is included. TestAuthOffersCHAPBeforePAP drives the default path and the fallback after a PAP Nak. The negative (first Configure-Request never PAP while CHAP is in the order) reads the same first frame as the positive; a send-side MUST has no refusing input, so the pair is the correct shape. TestDefaultAuthFallbackOrder and TestSelectAuthFallback are supporting unit tags with no discrimination record.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSelectAuthFallback`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1334_auth_test.go#L134) | unit/verify | revert, verified |
| negative | [`TestAuthOffersCHAPBeforePAP`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1334_clauses_test.go#L50) | unit/verify | revert, verified |
| negative | [`TestConfiguredPAPStillOffersCHAPFirst`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1334_clauses_test.go#L98) | unit/verify | revert, verified |
| positive | [`TestDefaultAuthFallbackOrder`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1334_auth_test.go#L292) | unit/verify | revert, verified |
| positive | [`TestAuthOffersCHAPBeforePAP`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1334_clauses_test.go#L49) | unit/verify | revert, verified |
| positive | [`TestConfiguredPAPStillOffersCHAPFirst`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1334_clauses_test.go#L97) | unit/verify | revert, verified |

### [`RFC1334-2.3-1`](#rfc1334-2.3-1)

If the Peer-ID/Password pair received in an Authenticate-Request is both recognizable and acceptable, then the authenticator MUST transmit a PAP packet with the Code field set to 2 (Authenticate- Ack). (Section 2.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: an accepted Peer-ID/Password pair answered with anything but a PAP Code 2. TestPAPRequestEmitsEvent (accept=true) asserts reply proto ProtoPAP and payload[0]==PAPAuthenticateAck through runPAPAuthPhase; TestPAPRejectWritesNak asserts the reject path sends Nak, so an unconditional Ack goes red.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestPAPRejectWritesNak`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1334_pap_test.go#L411) | unit/verify | unproven |
| positive | [`TestPAPRequestEmitsEvent`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1334_pap_test.go#L293) | unit/verify | unproven |

### [`RFC1334-2.3-2`](#rfc1334-2.3-2)

If the Peer-ID/Password pair received in a Authenticate-Request is not recognizable or acceptable, then the authenticator MUST transmit a PAP packet with the Code field set to 3 (Authenticate- Nak) (§2.2.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a rejected pair answered with anything but a PAP Code 3. TestPAPRejectWritesNak (accept=false) asserts payload[0]==PAPAuthenticateNak; TestPAPRequestEmitsEvent asserts the accept path sends Ack, so an unconditional Nak goes red. The SHOULD-terminate half of the sentence is RFC1334-2.3-5 and is not claimed here.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestPAPRequestEmitsEvent`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1334_pap_test.go#L297) | unit/verify | unproven |
| positive | [`TestPAPRejectWritesNak`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1334_pap_test.go#L407) | unit/verify | unproven |

### [`RFC1334-2.2.1-1`](#rfc1334-2.2.1-1)

The link peer MUST transmit a PAP packet with the Code field set to 1 (Authenticate-Request) during the Authentication phase (Section 2.2.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestClientOpensAuthPhaseWithPAPRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1334_pap_request_test.go#L53) | unit/verify | revert, verified |
| positive | [`TestClientOpensAuthPhaseWithPAPRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1334_pap_request_test.go#L52) | unit/verify | revert, verified |

### [`RFC1334-2.2.1-2`](#rfc1334-2.2.1-2)

The Authenticate-Request packet MUST be repeated until a valid reply packet is received, or an optional retry counter expires (Section 2.2.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestClientPAPRetriesUntilMatchingReply`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1334_pap_request_test.go#L126) | unit/verify | unproven |
| positive | [`TestClientPAPRetriesUntilMatchingReply`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1334_pap_request_test.go#L125) | unit/verify | unproven |

### [`RFC1334-2.2.1-3`](#rfc1334-2.2.1-3)

Upon reception of an Authenticate-Request packet, some type of Authenticate reply (described below) MUST be returned. (Section 2.2.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: an Authenticate-Request in the Authentication phase left without a reply. TestPAPRequestInAuthPhaseIsAnswered asserts a PAP reply frame with Code Ack and Identifier 0x5a for an accepted request; TestPAPRejectedRequestIsStillAnswered asserts a PAP Nak with 0x5a for a rejected one, so silence on either decision goes red.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestPAPRejectedRequestIsStillAnswered`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_phase_rfc1334_test.go#L92) | unit/verify | revert, verified |
| positive | [`TestPAPRequestInAuthPhaseIsAnswered`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_phase_rfc1334_test.go#L74) | unit/verify | revert, verified |

### [`RFC1334-2.2.1-4`](#rfc1334-2.2.1-4)

Implementation Note: Because the Authenticate-Ack might be lost, the authenticator MUST allow repeated Authenticate- Request packets after completing the Authentication phase. (Section 2.2.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: dropping or refusing an Authenticate-Request repeated after the Authentication phase completed. TestPAPReanswersAfterAuthentication delivers two repeats through handleFrame in AckSent and Opened and asserts three replies (require len 3), each an Ack; the negative TestPAPReanswerDiscardsMalformedRequest confines the reanswer to valid requests.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestPAPReanswerDiscardsMalformedRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_phase_rfc1334_test.go#L205) | unit/verify | unproven |
| positive | [`TestPAPReanswersAfterAuthentication`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_phase_rfc1334_test.go#L144) | unit/verify | unproven |

### [`RFC1334-2.2.1-5`](#rfc1334-2.2.1-5)

Protocol phase MUST return the same reply Code returned when the Authentication phase completed (§2.2.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a request after authentication answered with a Code other than the one the Authentication phase returned. TestPAPReanswerPreservesDecision runs accept and reject, changes the credentials of the repeated request, and asserts replies[1].Code == replies[0].Code for both Ack and Nak; TestPAPReanswersAfterAuthentication asserts the Ack Code on repeats through handleFrame. RFC text note: rfc1334.txt drops the line naming the Network-Layer Protocol phase, so the row quotes the surviving span.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestPAPReanswerPreservesDecision`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_phase_rfc1334_test.go#L178) | unit/verify | unproven |
| positive | [`TestPAPReanswersAfterAuthentication`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_phase_rfc1334_test.go#L145) | unit/verify | unproven |

### [`RFC1334-2.2.1-6`](#rfc1334-2.2.1-6)

Any Authenticate-Request packets received during any other phase MUST be silently discarded (Section 2.2.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestPAPRequestInAuthPhaseIsAnswered`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_phase_rfc1334_test.go#L75) | unit/verify | revert, verified |
| positive | [`TestPAPRequestOutsideAuthPhaseIsSilentlyDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_phase_rfc1334_test.go#L109) | unit/verify | revert, verified |

### [`RFC1334-2.2-1`](#rfc1334-2.2-1)

The Identifier field MUST be changed each time an Authenticate-Request packet is issued. (Section 2.2.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a reissued Authenticate-Request carrying the previous Identifier. TestClientPAPRetriesUntilMatchingReply lets the first request go unanswered, advances the production retry timer, and asserts packets[0].Identifier != packets[1].Identifier with the credentials unchanged; the negative half asserts no third request after the matching Ack.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestClientPAPRetriesUntilMatchingReply`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1334_pap_request_test.go#L128) | unit/verify | unproven |
| positive | [`TestClientPAPRetriesUntilMatchingReply`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1334_pap_request_test.go#L127) | unit/verify | unproven |

### [`RFC1334-2.3-3`](#rfc1334-2.3-3)

The Identifier field MUST be copied from the Identifier field of the Authenticate-Request which caused this reply. (Section 2.2.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a reply Identifier not copied from the request that caused it. TestPAPRequestEmitsEvent and TestPAPRejectWritesNak assert 0x42 and 0x77 echoed on Ack and Nak; TestPAPReanswersAfterAuthentication asserts replies carry 0x5a, 0x5b, 0x5c in turn, so a hardcoded or stale Identifier goes red.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestPAPReanswersAfterAuthentication`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_phase_rfc1334_test.go#L147) | unit/verify | unproven |
| positive | [`TestPAPReanswersAfterAuthentication`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_phase_rfc1334_test.go#L146) | unit/verify | unproven |
| positive | [`TestPAPRejectWritesNak`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1334_pap_test.go#L414) | unit/verify | unproven |
| positive | [`TestPAPRequestEmitsEvent`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1334_pap_test.go#L300) | unit/verify | unproven |

### [`RFC1334-2.3-4`](#rfc1334-2.3-4)

It is intended to be human readable, and MUST NOT affect operation of the protocol. (Section 2.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge ppp5 2026-09-30. The Message binds the receiver of the Ack/Nak, the pppoeclient peer (runClientAuth); ppp's authenticator never receives it. TestPAPReplyMessageDoesNotAffectOutcome now holds the Code fixed per group and varies the Message over the same three values ('', 'welcome aboard', 'invalid credentials'): every Ack succeeds (+), every Nak returns the 'PAP auth rejected' error (-), which an ignored Nak (retry exhaustion) would not give. A Message-driven decision turns one group red. Records: runClientAuth revert, both polarities.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestPAPReplyMessageDoesNotAffectOutcome`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1334_pap_message_test.go#L56) | unit/verify | revert, verified |
| positive | [`TestPAPReplyMessageDoesNotAffectOutcome`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1334_pap_message_test.go#L55) | unit/verify | revert, verified |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | rfc2119 |
| Source | rfc/full/rfc1334.txt |
| Source fingerprint | 34483221567656af |
| Record | rfc/extraction/rfc1334.json |
| Mapped sentences | 13 |
| Declined as scope | 18 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | walked | not stated |
| `1` | not stated | 1 | walked | not stated |
| `1.1` | not stated | 3 | walked | not stated |
| `1.2` | not stated | 0 | walked | not stated |
| `2` | not stated | 1 | walked | not stated |
| `2.1` | not stated | 0 | walked | not stated |
| `2.2` | not stated | 0 | walked | not stated |
| `2.2.1` | not stated | 7 | walked | not stated |
| `2.2.2` | not stated | 4 | walked | not stated |
| `3` | not stated | 1 | walked | not stated |
| `3.1` | not stated | 0 | walked | not stated |
| `3.2` | not stated | 0 | walked | not stated |
| `3.2.1` | not stated | 10 | walked | not stated |
| `3.2.2` | not stated | 4 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `1.1:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1.1 'Specification Requirements' defines the keywords themselves; this sentence is part of the definition of MAY/'optional' and states no PAP or CHAP behaviour. | MUST This word, or the adjective "required", means that the definition is an absolute requirement of the specification. |
| `1.1:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1.1 'Specification Requirements' defines the keywords themselves; this sentence is part of the definition of MAY/'optional' and states no PAP or CHAP behaviour. | MUST NOT This phrase means that the definition is an absolute prohibition of the specification. |
| `1.1:3` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1.1 'Specification Requirements' defines the keywords themselves; this sentence is part of the definition of MAY/'optional' and states no PAP or CHAP behaviour. | An implementation which does not include this option MUST be prepared to interoperate with another implementation which does include the option. |
| `3:1` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | RFC 1994 carries 'Obsoletes: 1334' in its header and defines CHAP in full; the Meta row of this summary records the partial obsoletion ('RFC 1994 for CHAP only -- PAP remains defined here'). The CHAP obligation is owed under rfc/short/rfc1994.md, not here. | The CHAP algorithm requires that the length of the secret MUST be at least 1 octet. |
| `3.2.1:1` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | RFC 1994 carries 'Obsoletes: 1334' in its header and defines CHAP in full; the Meta row of this summary records the partial obsoletion ('RFC 1994 for CHAP only -- PAP remains defined here'). The CHAP obligation is owed under rfc/short/rfc1994.md, not here. | The authenticator MUST transmit a CHAP packet with the Code field set to 1 (Challenge). |
| `3.2.1:2` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | RFC 1994 carries 'Obsoletes: 1334' in its header and defines CHAP in full; the Meta row of this summary records the partial obsoletion ('RFC 1994 for CHAP only -- PAP remains defined here'). The CHAP obligation is owed under rfc/short/rfc1994.md, not here. | Additional Challenge packets MUST be sent until a valid Response packet is received, or an optional retry counter expires. |
| `3.2.1:3` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | RFC 1994 carries 'Obsoletes: 1334' in its header and defines CHAP in full; the Meta row of this summary records the partial obsoletion ('RFC 1994 for CHAP only -- PAP remains defined here'). The CHAP obligation is owed under rfc/short/rfc1994.md, not here. | Whenever a Challenge packet is received, the peer MUST transmit a CHAP packet with the Code field set to 2 (Response). |
| `3.2.1:4` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | RFC 1994 carries 'Obsoletes: 1334' in its header and defines CHAP in full; the Meta row of this summary records the partial obsoletion ('RFC 1994 for CHAP only -- PAP remains defined here'). The CHAP obligation is owed under rfc/short/rfc1994.md, not here. | Based on this comparison, the authenticator MUST send a Success or Failure packet (described below). |
| `3.2.1:5` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | RFC 1994 carries 'Obsoletes: 1334' in its header and defines CHAP in full; the Meta row of this summary records the partial obsoletion ('RFC 1994 for CHAP only -- PAP remains defined here'). The CHAP obligation is owed under rfc/short/rfc1994.md, not here. | Implementation Note: Because the Success might be lost, the authenticator MUST allow repeated Response packets after completing the Authentication phase. |
| `3.2.1:6` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | RFC 1994 carries 'Obsoletes: 1334' in its header and defines CHAP in full; the Meta row of this summary records the partial obsoletion ('RFC 1994 for CHAP only -- PAP remains defined here'). The CHAP obligation is owed under rfc/short/rfc1994.md, not here. | To prevent discovery of alternative Names and Secrets, any Response packets received having the current Challenge Identifier MUST return the same reply Code returned when the Authentication phase completed (the message portion MAY be different). |
| `3.2.1:7` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | RFC 1994 carries 'Obsoletes: 1334' in its header and defines CHAP in full; the Meta row of this summary records the partial obsoletion ('RFC 1994 for CHAP only -- PAP remains defined here'). The CHAP obligation is owed under rfc/short/rfc1994.md, not here. | Any Response packets received during any other phase MUST be silently discarded. |
| `3.2.1:8` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | RFC 1994 carries 'Obsoletes: 1334' in its header and defines CHAP in full; the Meta row of this summary records the partial obsoletion ('RFC 1994 for CHAP only -- PAP remains defined here'). The CHAP obligation is owed under rfc/short/rfc1994.md, not here. | The Identifier field MUST be changed each time a Challenge is sent. |
| `3.2.1:9` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | RFC 1994 carries 'Obsoletes: 1334' in its header and defines CHAP in full; the Meta row of this summary records the partial obsoletion ('RFC 1994 for CHAP only -- PAP remains defined here'). The CHAP obligation is owed under rfc/short/rfc1994.md, not here. | The Response Identifier MUST be copied from the Identifier field of the Challenge which caused the Response. |
| `3.2.1:10` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | RFC 1994 carries 'Obsoletes: 1334' in its header and defines CHAP in full; the Meta row of this summary records the partial obsoletion ('RFC 1994 for CHAP only -- PAP remains defined here'). The CHAP obligation is owed under rfc/short/rfc1994.md, not here. | The Challenge Value MUST be changed each time a Challenge is sent. |
| `3.2.2:1` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | RFC 1994 carries 'Obsoletes: 1334' in its header and defines CHAP in full; the Meta row of this summary records the partial obsoletion ('RFC 1994 for CHAP only -- PAP remains defined here'). The CHAP obligation is owed under rfc/short/rfc1994.md, not here. | If the Value received in a Response is equal to the expected value, then the implementation MUST transmit a CHAP packet with the Code field set to 3 (Success). |
| `3.2.2:2` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | RFC 1994 carries 'Obsoletes: 1334' in its header and defines CHAP in full; the Meta row of this summary records the partial obsoletion ('RFC 1994 for CHAP only -- PAP remains defined here'). The CHAP obligation is owed under rfc/short/rfc1994.md, not here. | If the Value received in a Response is not equal to the expected value, then the implementation MUST transmit a CHAP packet with the Code field set to 4 (Failure), and SHOULD take action to terminate the link. |
| `3.2.2:3` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | RFC 1994 carries 'Obsoletes: 1334' in its header and defines CHAP in full; the Meta row of this summary records the partial obsoletion ('RFC 1994 for CHAP only -- PAP remains defined here'). The CHAP obligation is owed under rfc/short/rfc1994.md, not here. | The Identifier field MUST be copied from the Identifier field of the Response which caused this reply. |
| `3.2.2:4` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | RFC 1994 carries 'Obsoletes: 1334' in its header and defines CHAP in full; the Meta row of this summary records the partial obsoletion ('RFC 1994 for CHAP only -- PAP remains defined here'). The CHAP obligation is owed under rfc/short/rfc1994.md, not here. | It is intended to be human readable, and MUST NOT affect operation of the protocol. |

## Superseded

RFC 1334 is obsoleted by RFC 1994.

| Requirement | Disposition | Now stated at | Reason |
|---|---|---|---|
| [`RFC1334-1-1`](#rfc1334-1-1) If authentication of the link is desired, an implementation MUST specify the Authentication-Protocol Configuration Option during Link Establishment phase. (Section 1) | restated | RFC1994-1-1 | RFC 1994 Section 1 repeats the sentence word for word, that an implementation MUST specify the Authentication-Protocol Configuration Option during Link Establishment phase if authentication of the link is desired. The obligation is PPP-wide and binds neither authentication protocol in particular |
| [`RFC1334-x-1`](#rfc1334-x-1) Any implementations which include a stronger authentication method (such as CHAP, described below) MUST offer to negotiate that method prior to PAP. (§2) | dropped | not stated | RFC 1994 defines CHAP alone and states no obligation to offer a stronger method before PAP. Its Security Considerations warn that authenticating one user name by several methods exposes the least secure of them, and recommend one method per user name, with no keyword. The rule is still owed for as long as PAP is offered |
| [`RFC1334-2.3-1`](#rfc1334-2.3-1) If the Peer-ID/Password pair received in an Authenticate-Request is both recognizable and acceptable, then the authenticator MUST transmit a PAP packet with the Code field set to 2 (Authenticate- Ack). (Section 2.3) | dropped | not stated | RFC 1994 defines no PAP packet. Its Section 4.2 obliges a CHAP packet with Code 3 (Success) when the received Response Value equals the expected value, which is the CHAP handshake rather than the PAP Authenticate-Ack. PAP stays defined by RFC 1334, as this summary's forward Meta row records |
| [`RFC1334-2.3-2`](#rfc1334-2.3-2) If the Peer-ID/Password pair received in a Authenticate-Request is not recognizable or acceptable, then the authenticator MUST transmit a PAP packet with the Code field set to 3 (Authenticate- Nak) (§2.2.2) | dropped | not stated | RFC 1994 Section 4.2 obliges a CHAP packet with Code 4 (Failure) when the Response Value does not match, and defines no PAP Authenticate-Nak. The PAP rule is still owed for as long as PAP peers are authenticated |
| [`RFC1334-2.2.1-1`](#rfc1334-2.2.1-1) The link peer MUST transmit a PAP packet with the Code field set to 1 (Authenticate-Request) during the Authentication phase (Section 2.2.1) | dropped | not stated | RFC 1994 defines CHAP alone and no PAP packet, so it states no obligation to transmit an Authenticate-Request; PAP stays defined here |
| [`RFC1334-2.2.1-2`](#rfc1334-2.2.1-2) The Authenticate-Request packet MUST be repeated until a valid reply packet is received, or an optional retry counter expires (Section 2.2.1) | dropped | not stated | RFC 1994 defines no PAP packet and no Authenticate-Request retry; its Section 4.1 retries only the CHAP Challenge; PAP stays defined here |
| [`RFC1334-2.2.1-3`](#rfc1334-2.2.1-3) Upon reception of an Authenticate-Request packet, some type of Authenticate reply (described below) MUST be returned. (Section 2.2.1) | dropped | not stated | RFC 1994 defines no PAP packet, so no reply to an Authenticate-Request; its Section 4.1 obliges only a Success or Failure to a CHAP Response; PAP stays defined here |
| [`RFC1334-2.2.1-4`](#rfc1334-2.2.1-4) Implementation Note: Because the Authenticate-Ack might be lost, the authenticator MUST allow repeated Authenticate- Request packets after completing the Authentication phase. (Section 2.2.1) | dropped | not stated | RFC 1994 defines no PAP Authenticate-Ack and states no obligation to answer repeated Authenticate-Requests; PAP stays defined here |
| [`RFC1334-2.2.1-5`](#rfc1334-2.2.1-5) Protocol phase MUST return the same reply Code returned when the Authentication phase completed (§2.2.1) | dropped | not stated | RFC 1994 states no obligation on a PAP packet received in the Network-Layer Protocol phase; PAP stays defined here |
| [`RFC1334-2.2.1-6`](#rfc1334-2.2.1-6) Any Authenticate-Request packets received during any other phase MUST be silently discarded (Section 2.2.1) | dropped | not stated | RFC 1994 states no obligation on a PAP packet received in another phase; its Section 4.1 discards only CHAP Response packets; PAP stays defined here |
| [`RFC1334-2.2-1`](#rfc1334-2.2-1) The Identifier field MUST be changed each time an Authenticate-Request packet is issued. (Section 2.2.1) | dropped | not stated | RFC 1994 states no PAP obligation. Its Section 4.1 requires the Identifier to change each time a Challenge is sent, which binds the CHAP Challenge and not a reissued PAP Authenticate-Request |
| [`RFC1334-2.3-3`](#rfc1334-2.3-3) The Identifier field MUST be copied from the Identifier field of the Authenticate-Request which caused this reply. (Section 2.2.2) | dropped | not stated | RFC 1994 Section 4.2 requires the Success or Failure Identifier to be copied from the Response which caused the reply, which binds CHAP. It states nothing about a PAP Authenticate-Ack or Authenticate-Nak |
| [`RFC1334-2.3-4`](#rfc1334-2.3-4) It is intended to be human readable, and MUST NOT affect operation of the protocol. (Section 2.3) | dropped | not stated | RFC 1994 Section 4.2 carries the same MUST NOT for the Message field of a CHAP Success or Failure packet. It states nothing about the PAP Ack and Nak Message field, which RFC 1334 still defines |
| [`RFC1334-2.3-5`](#rfc1334-2.3-5) If the Peer-ID/Password pair received in a Authenticate-Request is not recognizable or acceptable, then the authenticator MUST transmit a PAP packet with the Code field set to 3 (Authenticate- Nak), and SHOULD take action to terminate the link. (§2.2.2) | dropped | not stated | RFC 1994 Section 4.2 says a CHAP implementation SHOULD take action to terminate the link when it transmits a Failure. That binds the CHAP exchange, and RFC 1994 states nothing about a PAP Authenticate-Nak |
| [`RFC1334-2.3-6`](#rfc1334-2.3-6) The Message field is zero or more octets, and its contents are implementation dependent. (§2.2.2) | dropped | not stated | RFC 1994 Section 4.2 says the Message field of a CHAP Success or Failure is zero or more octets. It states nothing about the PAP Ack and Nak Message field |
| [`RFC1334-2.2-2`](#rfc1334-2.2-2) The peer is in control of the frequency and timing of the attempts. (§2) | dropped | not stated | RFC 1994 Section 4.1 obliges the authenticator to send further Challenges until a valid Response arrives or a retry counter expires, which is a CHAP obligation on the authenticator. RFC 1994 states nothing about a peer retrying a PAP Authenticate-Request |
