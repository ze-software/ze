# RFC 1334 - PPP Authentication Protocols

PAP. Every requirement this repository extracted from RFC 1334, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 13 | of 13 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0 | of 13 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0 | of 13 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0 | of 13 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| No test at all | 0 | of 13 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 34 | 34 of 34 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 13 | of 16 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the counts beside it are what says how Ze stands |
| Out of scope | 0 | of 13 gated MUSTs | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as a Ze gap. Scope, not coverage: it is counted in the gated whole the cards on this page add up to |
| Not applicable | 0 | of 13 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as a Ze gap. It is counted in the gated whole the cards add up to |
| Met below Ze | 0 | of 13 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze proven: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 13 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as a Ze gap. It is counted in the gated whole the cards add up to |
| Linux cannot do this | 0 | of 13 gated MUSTs | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as a Ze gap. It is counted in the gated whole the cards add up to and earns no proof credit |

The 9 counts marked as a part above are the whole of the 13 gated MUSTs: they add up to 13. Proven by a recorded break counts TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the count under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| Partial proof; remaining gap | ok | green at zero, RED above it: a tested clause cannot prove the whole requirement |
| No test at all | ok | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green counts above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Linux cannot do this | neutral | neutral at every value: an evidenced platform limit is neither a proof nor a scope exemption |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. It counts TAGGED UNITS, not obligations, so this count is not one of the parts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public row | PAP |
| Enrolment | Enrolled |
| Requirements | 16 |
| Gated MUST-level | 13 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 34 |
| Tagged units | 34 |
| Recorded audit verdicts | 10 |
| Discrimination records | 34 |
| Summary | `rfc/short/rfc1334.md` |
| Requirement shard | `rfc/requirements/rfc1334.md` |
| RFC text | `rfc/full/rfc1334.txt` |

## Enrolment

Enrolled: PAP and LCP authentication are implemented by the shared PPP authenticator and PPPoE client. PAP Authenticate-Request retries, changing Identifiers, matching reply validation and post-authentication reanswers are implemented with regression tests, and each of their tags carries a discrimination record in `rfc/discrimination/rfc1334/` (2026-10-09).

## What the public ledger says

**Area:** PAP

**Proof:** 13 of 13 MUST proven

**Evidence:** unit 13

**Unmet:** -


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

## Figures

- **MUST bar**: Ze proven 13
- **Overview bar**: Ze MUST proven 13, SHOULD not decided 3

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Not walked | 0 |
| Mandatory register | complete |
| MUST rows | 13 |
| MUST rows kept (the MUST bar) | 13 |
| Ze proven | 13 |
| OS verified | 0 |
| OS testing gap | 0 |
| Ze correctness gap | 0 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 3 |
| SHOULD proven | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD not ever | 0 |
| SHOULD not yet | 0 |
| SHOULD not decided | 3 |

The MUST bar counts the MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT rows of the walked RFCs that the stack owes: Ze proven means a recorded test break proves the row; a row a layer under Ze performs is OS verified when a test checks that layer on the state Ze installs, an OS testing gap until one does. The overview bar adds the MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT rows and the rows not owed, which do not bind Ze. An RFC that is not walked has no valid extraction sign-off, so its rows count in no figure. Every count is read from the RFC data files when the page loads.

Source: [data/rfc/rfc1334.json](../../../data/rfc/rfc1334.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC1334-1-1` | If authentication of the link is desired, an implementation MUST specify the Authentication-Protocol Configuration Option during Link Establishment phase. (Section 1) | MUST | 1 | **positive:** `unit/verify` [`TestLocalCONFREQAdvertisesAuthMethod`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_dispatch_test.go#L678). **negative:** `unit/verify` [`TestAuthProtoRejectClearsMethod`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_dispatch_test.go#L148) |
| `RFC1334-x-1` | Any implementations which include a stronger authentication method (such as CHAP, described below) MUST offer to negotiate that method prior to PAP. (§2) | MUST | 2 | **positive:** `unit/verify` [`TestAuthAllowSelectionMatrix`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_policy_driver_test.go#L33). **positive:** `unit/verify` [`TestAuthOffersCHAPBeforePAP`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1334_clauses_test.go#L49). **positive:** `unit/verify` [`TestConfiguredPAPStillOffersCHAPFirst`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1334_clauses_test.go#L98). **positive:** `unit/verify` [`TestInitialAuthMethodCHAPBeforePAPWhateverOrder`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1334_auth_test.go#L273). **negative:** `unit/verify` [`TestAuthAllowSelectionMatrix`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_policy_driver_test.go#L34). **negative:** `unit/verify` [`TestAuthOffersCHAPBeforePAP`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1334_clauses_test.go#L50). **negative:** `unit/verify` [`TestConfiguredPAPStillOffersCHAPFirst`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1334_clauses_test.go#L99). **negative:** `unit/verify` [`TestSelectAuthFallback`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1334_auth_test.go#L122) |
| `RFC1334-2.3-1` | If the Peer-ID/Password pair received in an Authenticate-Request is both recognizable and acceptable, then the authenticator MUST transmit a PAP packet with the Code field set to 2 (Authenticate- Ack). (Section 2.2.2) | MUST | 2.2.2 | **positive:** `unit/verify` [`TestPAPRequestEmitsEvent`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1334_pap_test.go#L293). **negative:** `unit/verify` [`TestPAPRejectWritesNak`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1334_pap_test.go#L411) |
| `RFC1334-2.3-2` | If the Peer-ID/Password pair received in a Authenticate-Request is not recognizable or acceptable, then the authenticator MUST transmit a PAP packet with the Code field set to 3 (Authenticate- Nak) (§2.2.2) | MUST | 2.2.2 | **positive:** `unit/verify` [`TestPAPRejectWritesNak`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1334_pap_test.go#L407). **negative:** `unit/verify` [`TestPAPRequestEmitsEvent`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1334_pap_test.go#L297) |
| `RFC1334-2.2.1-1` | The link peer MUST transmit a PAP packet with the Code field set to 1 (Authenticate-Request) during the Authentication phase (Section 2.2.1) | MUST | 2.2.1 | **positive:** `unit/verify` [`TestClientOpensAuthPhaseWithPAPRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1334_pap_request_test.go#L52). **negative:** `unit/verify` [`TestClientOpensAuthPhaseWithPAPRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1334_pap_request_test.go#L53) |
| `RFC1334-2.2.1-2` | The Authenticate-Request packet MUST be repeated until a valid reply packet is received, or an optional retry counter expires (Section 2.2.1) | MUST | 2.2.1 | **positive:** `unit/verify` [`TestClientPAPRetriesUntilMatchingReply`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1334_pap_request_test.go#L124). **negative:** `unit/verify` [`TestClientPAPRetriesUntilMatchingReply`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1334_pap_request_test.go#L125) |
| `RFC1334-2.2.1-3` | Upon reception of an Authenticate-Request packet, some type of Authenticate reply (described below) MUST be returned. (Section 2.2.1) | MUST | 2.2.1 | **positive:** `unit/verify` [`TestPAPRequestInAuthPhaseIsAnswered`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_phase_rfc1334_test.go#L74). **negative:** `unit/verify` [`TestPAPRejectedRequestIsStillAnswered`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_phase_rfc1334_test.go#L92) |
| `RFC1334-2.2.1-4` | Implementation Note: Because the Authenticate-Ack might be lost, the authenticator MUST allow repeated Authenticate- Request packets after completing the Authentication phase. (Section 2.2.1) | MUST | 2.2.1 | **positive:** `unit/verify` [`TestPAPReanswersAfterAuthentication`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_phase_rfc1334_test.go#L144). **negative:** `unit/verify` [`TestPAPReanswerDiscardsMalformedRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_phase_rfc1334_test.go#L205) |
| `RFC1334-2.2.1-5` | Protocol phase MUST return the same reply Code returned when the Authentication phase completed (§2.2.1) | MUST | 2.2.1 | **positive:** `unit/verify` [`TestPAPReanswersAfterAuthentication`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_phase_rfc1334_test.go#L145). **negative:** `unit/verify` [`TestPAPReanswerPreservesDecision`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_phase_rfc1334_test.go#L178) |
| `RFC1334-2.2.1-6` | Any Authenticate-Request packets received during any other phase MUST be silently discarded (Section 2.2.1) | MUST | 2.2.1 | **positive:** `unit/verify` [`TestPAPRequestOutsideAuthPhaseIsSilentlyDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_phase_rfc1334_test.go#L109). **negative:** `unit/verify` [`TestPAPRequestInAuthPhaseIsAnswered`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_phase_rfc1334_test.go#L75) |
| `RFC1334-2.2-1` | The Identifier field MUST be changed each time an Authenticate-Request packet is issued. (Section 2.2.1) | MUST | 2.2.1 | **positive:** `unit/verify` [`TestClientPAPRetriesUntilMatchingReply`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1334_pap_request_test.go#L126). **negative:** `unit/verify` [`TestClientPAPRetriesUntilMatchingReply`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1334_pap_request_test.go#L127) |
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
| positive | [`TestLocalCONFREQAdvertisesAuthMethod`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_dispatch_test.go#L678) | unit/verify | revert, verified |

### [`RFC1334-x-1`](#rfc1334-x-1)

Any implementations which include a stronger authentication method (such as CHAP, described below) MUST offer to negotiate that method prior to PAP. (§2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Freshly rejudged the existing recorded enforced verdict after reading the actual new RFC1334-x-1 tags on TestAuthAllowSelectionMatrix, their two current native discrimination records, the exact wire/event assertions and native observation output (artifact 173), all other current tagged units, current producers and the complete RFC 1334 text. Source, Section 2, page 4: Any implementations which include a stronger authentication method (such as CHAP, described below) MUST offer to negotiate that method prior to PAP. Q1 yes: tests enforce stronger permitted methods before PAP rather than prohibit permitted PAP after stronger methods were offered. TestInitialAuthMethodCHAPBeforePAPWhateverOrder covers PAP-derived default, explicit PAP-first ordering, either CHAP initial choice, configured-CHAP precedence and PAP-only/no-auth conditional controls. The deleted broad-default helper is intentionally obsolete; neither its helper nor its historical panic record is needed for this proof. Q2 yes: the matrix positive PAP default case observes wire CHAP-MD5 then PAP after a correlated PAP Nak, and final Configure-Ack publishes PAP at Opened. The current positive initialAuthMethod mutant skips initial CHAP; native output shows the clean matrix passing and PAP_default failing with first offer pap want chap-md5. The independent negative PAP written first case permits [pap, ms-chap-v2, chap-md5] and answers with two correlated PAP Naks: exact wire offers must be MS-CHAPv2, CHAP-MD5, PAP, followed by PAP at Opened. The recorded selectAuthFallback PAP-branch continue-to-break mutant makes only this matrix case fail at its first fallback, offer 1 = pap want chap-md5; native output also shows its clean baseline passing. This is the previously identified coverage escape, now actually tagged and discriminated. Q3 yes: canonical Nak options echo the latest actual request Identifier, offered methods are recorded on successful sends, wire decoding includes the CHAP algorithm, and PAP and both CHAP variants are explicitly permitted in the negative case. Its failure therefore isolates premature PAP before still-untried CHAP, not unlisted-policy rejection, malformed input or stale correlation. Positive and negative use distinct matrix inputs and negotiation states, not two hats on one first frame. Older CHAP-only exhaustion/PAP-excluded controls prove membership, and the two Driver tags share an assertion; neither is relied upon as the independent temporal negative. TestSelectAuthFallback additionally has the genuine allowed-PAP waits-for-untried-CHAP case. Q4 yes: the current tagged set covers initial selection and fallback, default and explicit policy, PAP written first, both stronger variants before PAP, repeated/unknown/outside suggestions, configured CHAP precedence, PAP-only/no-auth boundaries, actual Driver/spawnSession first-offer wiring and negotiated method publication at Opened. TestAuthOffersCHAPBeforePAP and TestConfiguredPAPStillOffersCHAPFirst retain the clause-wire/real-Driver checks. ResolveAuthAllow supplies the safe PAP default, initialAuthMethod chooses stronger first, and the validated Nak dispatcher consumes selectAuthFallback before the next Configure-Request, so no boundary silently drops the selection. The prior pending weak judgment was a real evidence-linkage gap, not a product defect; the new tagged matrix and observed semantic breaks close it. No product fix is needed. This is a pending judgment only: no stamp, check, build or test run was performed by this auditor.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestAuthAllowSelectionMatrix`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_policy_driver_test.go#L34) | unit/verify | mutant, verified |
| negative | [`TestSelectAuthFallback`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1334_auth_test.go#L122) | unit/verify | mutant, verified |
| negative | [`TestAuthOffersCHAPBeforePAP`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1334_clauses_test.go#L50) | unit/verify | mutant, verified |
| negative | [`TestConfiguredPAPStillOffersCHAPFirst`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1334_clauses_test.go#L99) | unit/verify | mutant, verified |
| positive | [`TestAuthAllowSelectionMatrix`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_policy_driver_test.go#L33) | unit/verify | mutant, verified |
| positive | [`TestInitialAuthMethodCHAPBeforePAPWhateverOrder`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1334_auth_test.go#L273) | unit/verify | mutant, verified |
| positive | [`TestAuthOffersCHAPBeforePAP`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1334_clauses_test.go#L49) | unit/verify | mutant, verified |
| positive | [`TestConfiguredPAPStillOffersCHAPFirst`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1334_clauses_test.go#L98) | unit/verify | mutant, verified |

### [`RFC1334-2.3-1`](#rfc1334-2.3-1)

If the Peer-ID/Password pair received in an Authenticate-Request is both recognizable and acceptable, then the authenticator MUST transmit a PAP packet with the Code field set to 2 (Authenticate- Ack). (Section 2.2.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RFC 1334 Section 2.2.2: "If the Peer-ID/Password pair received in an Authenticate-Request is both recognizable and acceptable, then the authenticator MUST transmit a PAP packet with the Code field set to 2 (Authenticate-Ack)." Rejudged after 83deb89129; the only tagged-unit change for this row is the added proto assertion in the negative and corrected pap.go line references. TestPAPRequestEmitsEvent delivers accept through the real runPAPAuthPhase over a pipe and asserts ProtoPAP, PAPAuthenticateAck and a true return. TestPAPRejectWritesNak asserts ProtoPAP and PAPAuthenticateNak on reject, so an unconditional Ack goes red; the recorded mutant inverts the branch at pap.go:258. Acceptance itself is the handler's decision, delivered on authRespCh; this row proves the reply to that decision. Caveat on proof: the tagged units compare the symbolic constant, and the literal Code 2 is pinned by the untagged TestPAPWriteAck (0x02); the positive record panics WritePAPAck rather than changing the Code. The sentence is in Section 2.2.2; RFC 1334 has no Section 2.3.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestPAPRejectWritesNak`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1334_pap_test.go#L411) | unit/verify | mutant, verified |
| positive | [`TestPAPRequestEmitsEvent`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1334_pap_test.go#L293) | unit/verify | revert, verified |

### [`RFC1334-2.3-2`](#rfc1334-2.3-2)

If the Peer-ID/Password pair received in a Authenticate-Request is not recognizable or acceptable, then the authenticator MUST transmit a PAP packet with the Code field set to 3 (Authenticate- Nak) (§2.2.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RFC 1334 Section 2.2.2: "If the Peer-ID/Password pair received in a Authenticate-Request is not recognizable or acceptable, then the authenticator MUST transmit a PAP packet with the Code field set to 3 (Authenticate-Nak), and SHOULD take action to terminate the link." The earlier weak verdict rested on the clause 'a PAP packet': TestPAPRejectWritesNak discarded the Protocol. 83deb89129 makes it read the Protocol and assert proto == ProtoPAP on the reject reply, so a Code 3 framed under another Protocol now goes red by reading. Code 3 is asserted by the same unit, and TestPAPRequestEmitsEvent asserts Ack on accept, so an unconditional Nak goes red; the recorded mutant inverts the branch at pap.go:258. Caveat on proof: no record shows the new assertion red. The positive record panics WritePAPNak, which fails before any assertion is reached, and pap.go:250 frames both replies under one WriteFrame(ProtoPAP). The tagged units compare the symbolic constant; the literal Code 3 is pinned by the untagged TestPAPWriteNak (0x03). The SHOULD half is RFC1334-2.3-5 and is not claimed here.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestPAPRequestEmitsEvent`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1334_pap_test.go#L297) | unit/verify | mutant, verified |
| positive | [`TestPAPRejectWritesNak`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1334_pap_test.go#L407) | unit/verify | revert, verified |

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
| negative | [`TestClientPAPRetriesUntilMatchingReply`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1334_pap_request_test.go#L125) | unit/verify | revert, verified |
| positive | [`TestClientPAPRetriesUntilMatchingReply`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1334_pap_request_test.go#L124) | unit/verify | revert, verified |

### [`RFC1334-2.2.1-3`](#rfc1334-2.2.1-3)

Upon reception of an Authenticate-Request packet, some type of Authenticate reply (described below) MUST be returned. (Section 2.2.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: an Authenticate-Request in the Authentication phase left without a reply. TestPAPRequestInAuthPhaseIsAnswered asserts a PAP reply frame with Code Ack and Identifier 0x5a for an accepted request; TestPAPRejectedRequestIsStillAnswered asserts a PAP Nak with 0x5a for a rejected one, so silence on either decision goes red.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestPAPRejectedRequestIsStillAnswered`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_phase_rfc1334_test.go#L92) | unit/verify | mutant, verified |
| positive | [`TestPAPRequestInAuthPhaseIsAnswered`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_phase_rfc1334_test.go#L74) | unit/verify | mutant, verified |

### [`RFC1334-2.2.1-4`](#rfc1334-2.2.1-4)

Implementation Note: Because the Authenticate-Ack might be lost, the authenticator MUST allow repeated Authenticate- Request packets after completing the Authentication phase. (Section 2.2.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: dropping or refusing an Authenticate-Request repeated after the Authentication phase completed. TestPAPReanswersAfterAuthentication delivers two repeats through handleFrame in AckSent and Opened and asserts three replies (require len 3), each an Ack; the negative TestPAPReanswerDiscardsMalformedRequest confines the reanswer to valid requests.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestPAPReanswerDiscardsMalformedRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_phase_rfc1334_test.go#L205) | unit/verify | revert, verified |
| positive | [`TestPAPReanswersAfterAuthentication`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_phase_rfc1334_test.go#L144) | unit/verify | revert, verified |

### [`RFC1334-2.2.1-5`](#rfc1334-2.2.1-5)

Protocol phase MUST return the same reply Code returned when the Authentication phase completed (§2.2.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a request after authentication answered with a Code other than the one the Authentication phase returned. TestPAPReanswerPreservesDecision runs accept and reject, changes the credentials of the repeated request, and asserts replies[1].Code == replies[0].Code for both Ack and Nak; TestPAPReanswersAfterAuthentication asserts the Ack Code on repeats through handleFrame. RFC text note: rfc1334.txt drops the line naming the Network-Layer Protocol phase, so the row quotes the surviving span.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestPAPReanswerPreservesDecision`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_phase_rfc1334_test.go#L178) | unit/verify | revert, verified |
| positive | [`TestPAPReanswersAfterAuthentication`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_phase_rfc1334_test.go#L145) | unit/verify | revert, verified |

### [`RFC1334-2.2.1-6`](#rfc1334-2.2.1-6)

Any Authenticate-Request packets received during any other phase MUST be silently discarded (Section 2.2.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestPAPRequestInAuthPhaseIsAnswered`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_phase_rfc1334_test.go#L75) | unit/verify | mutant, verified |
| positive | [`TestPAPRequestOutsideAuthPhaseIsSilentlyDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_phase_rfc1334_test.go#L109) | unit/verify | revert, verified |

### [`RFC1334-2.2-1`](#rfc1334-2.2-1)

The Identifier field MUST be changed each time an Authenticate-Request packet is issued. (Section 2.2.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Rejudge 2026-11-08: TestClientPAPRetriesUntilMatchingReply changed only in plumbing (frames through testFrameReader; runClientAuth now takes the shared Identifier counter, new(uint8)). runClientAuth increments that counter before the first request and before each retransmission (session.go). Assertions unchanged: the unanswered first request is reissued by the production timer with packets[0].Identifier != packets[1].Identifier and identical credentials; the negative half asserts no third request after the matching Ack.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestClientPAPRetriesUntilMatchingReply`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1334_pap_request_test.go#L127) | unit/verify | revert, verified |
| positive | [`TestClientPAPRetriesUntilMatchingReply`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1334_pap_request_test.go#L126) | unit/verify | revert, verified |

### [`RFC1334-2.3-3`](#rfc1334-2.3-3)

The Identifier field MUST be copied from the Identifier field of the Authenticate-Request which caused this reply. (Section 2.2.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RFC 1334 Section 2.2.2: "The Identifier field MUST be copied from the Identifier field of the Authenticate-Request which caused this reply." Rejudged after 83deb89129, which changed the two rfc1334_pap_test.go units and left TestPAPReanswersAfterAuthentication as it was. TestPAPRequestEmitsEvent asserts 0x42 on Ack and TestPAPRejectWritesNak asserts 0x77 on Nak, so a hardcoded value goes red on the first-reply path. TestPAPReanswersAfterAuthentication drives handleFrame in AckSent and Opened and asserts replies 0x5a, 0x5b, 0x5c, so a cached initial Identifier in reanswerPAP goes red by reading. That unit carries both tags on one assertion, and the distinct positives in the other two units keep the polarity pair real. It answers Ack only; reanswerPAP passes payload[1] to writePAPReply whatever the cached Code. Caveat on proof: all four records panic reanswerPAP or writePAPReply, so none observes a wrong Identifier, and the MUTATION comment's cached-Identifier break has no record. The doc comment references to pap.go:132, 138, 147, 259 and 262 now match the source.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestPAPReanswersAfterAuthentication`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_phase_rfc1334_test.go#L147) | unit/verify | revert, verified |
| positive | [`TestPAPReanswersAfterAuthentication`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_phase_rfc1334_test.go#L146) | unit/verify | revert, verified |
| positive | [`TestPAPRejectWritesNak`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1334_pap_test.go#L414) | unit/verify | revert, verified |
| positive | [`TestPAPRequestEmitsEvent`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1334_pap_test.go#L300) | unit/verify | revert, verified |

### [`RFC1334-2.3-4`](#rfc1334-2.3-4)

It is intended to be human readable, and MUST NOT affect operation of the protocol. (Section 2.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Rejudge 2026-11-08: TestPAPReplyMessageDoesNotAffectOutcome changed only in plumbing (testFrameReader, new(uint8) Identifier counter, which yields Identifier 1 matching the reply). Code held fixed per group, Message varied over '', 'welcome aboard', 'invalid credentials': every Ack succeeds, every Nak returns the 'PAP auth rejected' error, which an ignored Nak would not give. A Message-driven decision turns one group red.

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
| [`RFC1334-2.3-1`](#rfc1334-2.3-1) If the Peer-ID/Password pair received in an Authenticate-Request is both recognizable and acceptable, then the authenticator MUST transmit a PAP packet with the Code field set to 2 (Authenticate- Ack). (Section 2.2.2) | dropped | not stated | RFC 1994 defines no PAP packet. Its Section 4.2 obliges a CHAP packet with Code 3 (Success) when the received Response Value equals the expected value, which is the CHAP handshake rather than the PAP Authenticate-Ack. PAP stays defined by RFC 1334, as this summary's forward Meta row records |
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
