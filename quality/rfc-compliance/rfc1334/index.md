# RFC 1334 - PPP Authentication Protocols

PAP. Every requirement listed for RFC 1334, the tests linked to it, and their recorded reviews. This summary is included in the automatic compliance check, ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 13 | of 13 mandatory requirements | a positive test checks that Ze does what the requirement demands and a negative one checks that it refuses what the requirement forbids |
| One polarity plus reason | 0 | of 13 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason supplies the complete test coverage required |
| One polarity, unexcused | 0 | of 13 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial test coverage | 0 | of 13 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unverified, with zero whole-requirement credit |
| Declared gap or no test | 0 | of 13 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |
| Tests shown to detect a fault | 34 | 34 of 34 tests linked to requirements | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests verifies a requirement only after both tests have been shown to detect a fault |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 13 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 0 | of 13 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 0 | of 13 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze verified: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 13 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 13 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no test verification credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level) | 13 | of 16 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements included in the automatic checks: the compliance check (./le rfc check) holds Ze to each of them. This is the total being assessed; the counts above show the recorded status of those requirements. |

The 9 counts marked as a part above are the whole of the 13 mandatory requirements: they add up to 13. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got. An outlined card with no colored edge, under Totals, is a total the other cards are counted against, not a measure.

| Card | Tone here | Why that color |
|---|---|---|
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the count under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: one test direction is missing without a recorded reason |
| Partial test coverage | ok | green at zero, RED above it: testing one clause does not verify the whole requirement |
| Declared gap or no test | ok | green at zero, RED above it: a binding obligation that nothing shows Ze meets is owed, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green counts above are what says how much Ze's own tests verify |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Linux cannot do this | neutral | neutral at every value: an evidenced platform limit is neither test verification nor a scope exemption |
| Tests shown to detect a fault | ok | green at every value: these tests have been shown to detect a deliberate fault. This card counts individual tests separately, so its total cannot be added to the requirement counts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a mandatory requirement is unreviewed, green when every one is judged sound and current |
| Mandatory requirements (MUST level) | outlined | outlined, with no colored edge: it is the total the other cards are counted against, not a measure, and a larger one is neither good news nor bad |

## At a glance

| Field | Value |
|---|---|
| Public row | PAP |
| Automatic checks | Included |
| Requirements | 16 |
| Mandatory requirements (MUST level) | 13 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 34 |
| Tests linked to requirements | 34 |
| Recorded audit verdicts | 10 |
| Fault test records | 34 |
| Summary | `rfc/short/rfc1334.md` |
| Requirement table | `rfc/requirements/rfc1334.md` |
| RFC text | `rfc/full/rfc1334.txt` |

## Automatic checks

Included in automatic checks: PAP and LCP authentication are implemented by the shared PPP authenticator and PPPoE client. PAP Authenticate-Request retries, changing Identifiers, matching reply validation and post-authentication reanswers are implemented with regression tests, and each of their tags carries a discrimination record in `rfc/discrimination/rfc1334/` (2026-10-09).

## Published support status

**Area:** PAP

**Test verification:** 13 of 13 MUST proven

**Evidence:** unit 13

**Unmet:** -


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 13 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 0 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 0 | one part of the mandatory requirements |
| Partial test coverage (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level)** | **13** | every mandatory requirement appears in exactly one group above |

**Positive and negative tests (13):** [`RFC1334-1-1`](#rfc1334-1-1), [`RFC1334-x-1`](#rfc1334-x-1), [`RFC1334-2.3-1`](#rfc1334-2.3-1), [`RFC1334-2.3-2`](#rfc1334-2.3-2), [`RFC1334-2.2.1-1`](#rfc1334-2.2.1-1), [`RFC1334-2.2.1-2`](#rfc1334-2.2.1-2), [`RFC1334-2.2.1-3`](#rfc1334-2.2.1-3), [`RFC1334-2.2.1-4`](#rfc1334-2.2.1-4), [`RFC1334-2.2.1-5`](#rfc1334-2.2.1-5), [`RFC1334-2.2.1-6`](#rfc1334-2.2.1-6), [`RFC1334-2.2-1`](#rfc1334-2.2-1), [`RFC1334-2.3-3`](#rfc1334-2.3-3), [`RFC1334-2.3-4`](#rfc1334-2.3-4)

## Figures

- **MUST bar**: Ze verified 13
- **Overview bar**: Ze MUST verified 13, Not assessed 3

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | complete |
| MUST rows | 13 |
| MUST rows kept (the MUST bar) | 13 |
| Ze verified | 13 |
| OS verified | 0 |
| Tests awaiting verification | 0 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 0 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 3 |
| SHOULD verified | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD tests awaiting verification | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 3 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze verified means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Tests awaiting verification counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

Source: [data/rfc/rfc1334.json](../../../data/rfc/rfc1334.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC1334-1-1` | If authentication of the link is desired, an implementation MUST specify the Authentication-Protocol Configuration Option during Link Establishment phase. (Section 1) | MUST | 1 | **positive:** `unit/verify` [`TestLocalCONFREQAdvertisesAuthMethod`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_dispatch_test.go#L679). **negative:** `unit/verify` [`TestAuthProtoRejectClearsMethod`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_dispatch_test.go#L148) |
| `RFC1334-x-1` | Any implementations which include a stronger authentication method (such as CHAP, described below) MUST offer to negotiate that method prior to PAP. (§2) | MUST | 2 | **positive:** `unit/verify` [`TestAuthAllowSelectionMatrix`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_policy_driver_test.go#L33). **positive:** `unit/verify` [`TestAuthOffersCHAPBeforePAP`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1334_clauses_test.go#L49). **positive:** `unit/verify` [`TestConfiguredPAPStillOffersCHAPFirst`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1334_clauses_test.go#L98). **positive:** `unit/verify` [`TestInitialAuthMethodCHAPBeforePAPWhateverOrder`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1334_auth_test.go#L273). **negative:** `unit/verify` [`TestAuthAllowSelectionMatrix`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_policy_driver_test.go#L34). **negative:** `unit/verify` [`TestAuthOffersCHAPBeforePAP`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1334_clauses_test.go#L50). **negative:** `unit/verify` [`TestConfiguredPAPStillOffersCHAPFirst`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1334_clauses_test.go#L99). **negative:** `unit/verify` [`TestSelectAuthFallback`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1334_auth_test.go#L122) |
| `RFC1334-2.3-1` | If the Peer-ID/Password pair received in an Authenticate-Request is both recognizable and acceptable, then the authenticator MUST transmit a PAP packet with the Code field set to 2 (Authenticate- Ack). (Section 2.2.2) | MUST | 2.2.2 | **positive:** `unit/verify` [`TestPAPRequestEmitsEvent`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1334_pap_test.go#L293). **negative:** `unit/verify` [`TestPAPRejectWritesNak`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1334_pap_test.go#L411) |
| `RFC1334-2.3-2` | If the Peer-ID/Password pair received in a Authenticate-Request is not recognizable or acceptable, then the authenticator MUST transmit a PAP packet with the Code field set to 3 (Authenticate- Nak) (§2.2.2) | MUST | 2.2.2 | **positive:** `unit/verify` [`TestPAPRejectWritesNak`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1334_pap_test.go#L407). **negative:** `unit/verify` [`TestPAPRequestEmitsEvent`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1334_pap_test.go#L297) |
| `RFC1334-2.2.1-1` | The link peer MUST transmit a PAP packet with the Code field set to 1 (Authenticate-Request) during the Authentication phase (Section 2.2.1) | MUST | 2.2.1 | **positive:** `unit/verify` [`TestClientOpensAuthPhaseWithPAPRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1334_pap_request_test.go#L52). **negative:** `unit/verify` [`TestClientOpensAuthPhaseWithPAPRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1334_pap_request_test.go#L53) |
| `RFC1334-2.2.1-2` | The Authenticate-Request packet MUST be repeated until a valid reply packet is received, or an optional retry counter expires (Section 2.2.1) | MUST | 2.2.1 | **positive:** `unit/verify` [`TestClientPAPRetriesUntilMatchingReply`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1334_pap_request_test.go#L124). **negative:** `unit/verify` [`TestClientPAPRetriesUntilMatchingReply`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1334_pap_request_test.go#L125) |
| `RFC1334-2.2.1-3` | Upon reception of an Authenticate-Request packet, some type of Authenticate reply (described below) MUST be returned. (Section 2.2.1) | MUST | 2.2.1 | **positive:** `unit/verify` [`TestPAPRequestInAuthPhaseIsAnswered`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_phase_rfc1334_test.go#L77). **negative:** `unit/verify` [`TestPAPRejectedRequestIsStillAnswered`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_phase_rfc1334_test.go#L95) |
| `RFC1334-2.2.1-4` | Implementation Note: Because the Authenticate-Ack might be lost, the authenticator MUST allow repeated Authenticate- Request packets after completing the Authentication phase. (Section 2.2.1) | MUST | 2.2.1 | **positive:** `unit/verify` [`TestPAPReanswersAfterAuthentication`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_phase_rfc1334_test.go#L147). **negative:** `unit/verify` [`TestPAPReanswerDiscardsMalformedRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_phase_rfc1334_test.go#L208) |
| `RFC1334-2.2.1-5` | Protocol phase MUST return the same reply Code returned when the Authentication phase completed (§2.2.1) | MUST | 2.2.1 | **positive:** `unit/verify` [`TestPAPReanswersAfterAuthentication`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_phase_rfc1334_test.go#L148). **negative:** `unit/verify` [`TestPAPReanswerPreservesDecision`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_phase_rfc1334_test.go#L181) |
| `RFC1334-2.2.1-6` | Any Authenticate-Request packets received during any other phase MUST be silently discarded (Section 2.2.1) | MUST | 2.2.1 | **positive:** `unit/verify` [`TestPAPRequestOutsideAuthPhaseIsSilentlyDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_phase_rfc1334_test.go#L112). **negative:** `unit/verify` [`TestPAPRequestInAuthPhaseIsAnswered`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_phase_rfc1334_test.go#L78) |
| `RFC1334-2.2-1` | The Identifier field MUST be changed each time an Authenticate-Request packet is issued. (Section 2.2.1) | MUST | 2.2.1 | **positive:** `unit/verify` [`TestClientPAPRetriesUntilMatchingReply`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1334_pap_request_test.go#L126). **negative:** `unit/verify` [`TestClientPAPRetriesUntilMatchingReply`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1334_pap_request_test.go#L127) |
| `RFC1334-2.3-3` | The Identifier field MUST be copied from the Identifier field of the Authenticate-Request which caused this reply. (Section 2.2.2) | MUST | 2.2.2 | **positive:** `unit/verify` [`TestPAPReanswersAfterAuthentication`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_phase_rfc1334_test.go#L149). **positive:** `unit/verify` [`TestPAPRejectWritesNak`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1334_pap_test.go#L414). **positive:** `unit/verify` [`TestPAPRequestEmitsEvent`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1334_pap_test.go#L300). **negative:** `unit/verify` [`TestPAPReanswersAfterAuthentication`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_phase_rfc1334_test.go#L150) |
| `RFC1334-2.3-4` | It is intended to be human readable, and MUST NOT affect operation of the protocol. (Section 2.3) | MUST NOT | 2.3 | **positive:** `unit/verify` [`TestPAPReplyMessageDoesNotAffectOutcome`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1334_pap_message_test.go#L55). **negative:** `unit/verify` [`TestPAPReplyMessageDoesNotAffectOutcome`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1334_pap_message_test.go#L56) |
| `RFC1334-2.3-5` | If the Peer-ID/Password pair received in a Authenticate-Request is not recognizable or acceptable, then the authenticator MUST transmit a PAP packet with the Code field set to 3 (Authenticate- Nak), and SHOULD take action to terminate the link. (§2.2.2) | SHOULD | 2.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC1334-2.3-6` | The Message field is zero or more octets, and its contents are implementation dependent. (§2.2.2) | MAY | 2.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC1334-2.2-2` | The peer is in control of the frequency and timing of the attempts. (§2) | MAY | 2 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

RFC 1334 lists no missing behavior, and every mandatory requirement included in automatic checks has a linked test.

## Test status

A test reads not verified when no record shows it failing after a deliberate fault. Until then, the test has not been shown to detect a fault in the required behavior.

### [`RFC1334-1-1`](#rfc1334-1-1)

If authentication of the link is desired, an implementation MUST specify the Authentication-Protocol Configuration Option during Link Establishment phase. (Section 1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent CURRENT-Core source rejudgment against RFC1334 Section1: authentication desired requires Authentication-Protocol during Link Establishment; RFC1994 repeats this PPP-wide obligation. Read TestLocalCONFREQAdvertisesAuthMethod in full plus authProtoReplyPeer/TestAuthProtoRejectClearsMethod: real Driver initial LCP Configure-Request carries exact PAP0xC023, CHAP0xC223/algorithm05 or MS-CHAPv2/algorithm81 according to explicit single-method configuration. Peer helper first checks actual protocol/request/offer, sends a matching-ID exact-byte Configure-Reject, and the next request must omit the option when optional authentication is no longer desired. Positive and negative reach distinct valid protocol conditions, not another malformed-packet guard. Read authMethodToLCPOptions, sendConfigureRequest and accepted-Reject auth adjustment; actual wire option is configuration-dependent. New owned fixture/control plumbing does not bypass negotiation or replace wire assertions. Fresh native records remain separate. Rejudged independently after full old/current tagged-unit and B private lint-proposal reads. The address-taking caller edits and immediate helper-local copies preserve every old input, wire assertion, state transition, timeout and cleanup join; the changed helpers still construct genuine Driver/request/Attempt capabilities. All tagged units were included, including inline tags. Producer behavior for this requirement is unchanged by the scoped lint packet. This pending source verdict claims neither a fresh compiler/runtime result nor a native discrimination observation; MAIN owns final source stamping and recorded cuts.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestAuthProtoRejectClearsMethod`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_dispatch_test.go#L148) | unit/verify | revert, verified |
| positive | [`TestLocalCONFREQAdvertisesAuthMethod`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_dispatch_test.go#L679) | unit/verify | revert, verified |

### [`RFC1334-x-1`](#rfc1334-x-1)

Any implementations which include a stronger authentication method (such as CHAP, described below) MUST offer to negotiate that method prior to PAP. (§2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Read RFC1334 Section2 including the full stronger-method-before-PAP obligation; the superseded annotation says the PAP obligation remains owed. Read every tagged unit: TestAuthAllowSelectionMatrix uses real Driver wire negotiation, default configured PAP first offering CHAP-MD5, then correlated PAP Nak fallback; its ordered [pap,ms-chap-v2,chap-md5] case requires MS-CHAPv2 then CHAP-MD5 despite repeated valid PAP Naks before finally allowing PAP and observing PAP at Opened. TestAuthOffersCHAPBeforePAP and TestConfiguredPAPStillOffersCHAPFirst pin first stronger offers and valid post-offer fallback; TestSelectAuthFallback rejects PAP under CHAP-only policy; TestInitialAuthMethodCHAPBeforePAPWhateverOrder covers stronger variants and default policy. Read ResolveAuthAllow, initialAuthMethod, selectAuthFallback and accepted Nak handling: all allowed untried stronger methods precede PAP. The real ordered driver matrix supplies distinct temporal observations; the shared-assertion tags in smaller helpers are not credited as independent pairs. Rejudged independently after full old/current tagged-unit and B private lint-proposal reads. The address-taking caller edits and immediate helper-local copies preserve every old input, wire assertion, state transition, timeout and cleanup join; the changed helpers still construct genuine Driver/request/Attempt capabilities. All tagged units were included, including inline tags. Producer behavior for this requirement is unchanged by the scoped lint packet. This pending source verdict claims neither a fresh compiler/runtime result nor a native discrimination observation; MAIN owns final source stamping and recorded cuts.

| Polarity | Test | Kind and tier | Test status |
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

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent CURRENT-Core source rejudgment against RFC1334 Section2.2.2 (stable requirement ID retains2.3): acceptable Peer-ID/password requires a PAP Authenticate-Ack Code2. Complete TestPAPRequestEmitsEvent sends actual PAP credentials over a pipe, inspects the genuine auth request, answers its private capability with acceptance, and requires ProtoPAP, Authenticate-Ack, echoed Identifier0x42, success event and successful handler return. Distinct TestPAPRejectWritesNak sends different valid credentials and an explicit rejection, requiring ProtoPAP and Nak, so unconditional Ack is refused. runPAPAuthPhase uses the actual decision only at the Ack/Nak writer choice; WritePAPAck encodes the declared Code2. No trailing malformed data or earlier unrelated error masks the branch. Scope is protocol response to provider decision; local credential verification is a separate boundary. Fresh discriminator execution is owed.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestPAPRejectWritesNak`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1334_pap_test.go#L411) | unit/verify | mutant, verified |
| positive | [`TestPAPRequestEmitsEvent`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1334_pap_test.go#L293) | unit/verify | revert, verified |

### [`RFC1334-2.3-2`](#rfc1334-2.3-2)

If the Peer-ID/Password pair received in a Authenticate-Request is not recognizable or acceptable, then the authenticator MUST transmit a PAP packet with the Code field set to 3 (Authenticate- Nak) (§2.2.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent CURRENT-Core source rejudgment against RFC1334 Section2.2.2: unacceptable Peer-ID/password requires PAP Authenticate-Nak Code3; the SHOULD termination clause is another requirement. Complete TestPAPRejectWritesNak sends a valid request and answers its actual auth capability false, requiring ProtoPAP, Nak Code, Identifier0x77, original message, failure event and false handler return. TestPAPRequestEmitsEvent is the conforming accepted contrast requiring PAP Ack, so unconditional Nak cannot pass. Read runPAPAuthPhase, WritePAPNak and writePAPReply: actual decision selects a PAP-framed Code3 reply before refusal lifecycle. These distinct accepted/rejected inputs isolate the comparison outcome rather than structural parsing. Current request-capability migration preserves every wire check; no runtime/record freshness is inferred here.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestPAPRequestEmitsEvent`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1334_pap_test.go#L297) | unit/verify | mutant, verified |
| positive | [`TestPAPRejectWritesNak`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1334_pap_test.go#L407) | unit/verify | revert, verified |

### [`RFC1334-2.2.1-1`](#rfc1334-2.2.1-1)

The link peer MUST transmit a PAP packet with the Code field set to 1 (Authenticate-Request) during the Authentication phase (Section 2.2.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestClientOpensAuthPhaseWithPAPRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1334_pap_request_test.go#L53) | unit/verify | revert, verified |
| positive | [`TestClientOpensAuthPhaseWithPAPRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1334_pap_request_test.go#L52) | unit/verify | revert, verified |

### [`RFC1334-2.2.1-2`](#rfc1334-2.2.1-2)

The Authenticate-Request packet MUST be repeated until a valid reply packet is received, or an optional retry counter expires (Section 2.2.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestClientPAPRetriesUntilMatchingReply`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1334_pap_request_test.go#L125) | unit/verify | revert, verified |
| positive | [`TestClientPAPRetriesUntilMatchingReply`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1334_pap_request_test.go#L124) | unit/verify | revert, verified |

### [`RFC1334-2.2.1-3`](#rfc1334-2.2.1-3)

Upon reception of an Authenticate-Request packet, some type of Authenticate reply (described below) MUST be returned. (Section 2.2.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: an Authenticate-Request in the Authentication phase left without a reply. TestPAPRequestInAuthPhaseIsAnswered asserts a PAP reply frame with Code Ack and Identifier 0x5a for an accepted request; TestPAPRejectedRequestIsStillAnswered asserts a PAP Nak with 0x5a for a rejected one, so silence on either decision goes red.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestPAPRejectedRequestIsStillAnswered`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_phase_rfc1334_test.go#L95) | unit/verify | mutant, verified |
| positive | [`TestPAPRequestInAuthPhaseIsAnswered`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_phase_rfc1334_test.go#L77) | unit/verify | mutant, verified |

### [`RFC1334-2.2.1-4`](#rfc1334-2.2.1-4)

Implementation Note: Because the Authenticate-Ack might be lost, the authenticator MUST allow repeated Authenticate- Request packets after completing the Authentication phase. (Section 2.2.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: dropping or refusing an Authenticate-Request repeated after the Authentication phase completed. TestPAPReanswersAfterAuthentication delivers two repeats through handleFrame in AckSent and Opened and asserts three replies (require len 3), each an Ack; the negative TestPAPReanswerDiscardsMalformedRequest confines the reanswer to valid requests.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestPAPReanswerDiscardsMalformedRequest`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_phase_rfc1334_test.go#L208) | unit/verify | revert, verified |
| positive | [`TestPAPReanswersAfterAuthentication`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_phase_rfc1334_test.go#L147) | unit/verify | revert, verified |

### [`RFC1334-2.2.1-5`](#rfc1334-2.2.1-5)

Protocol phase MUST return the same reply Code returned when the Authentication phase completed (§2.2.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a request after authentication answered with a Code other than the one the Authentication phase returned. TestPAPReanswerPreservesDecision runs accept and reject, changes the credentials of the repeated request, and asserts replies[1].Code == replies[0].Code for both Ack and Nak; TestPAPReanswersAfterAuthentication asserts the Ack Code on repeats through handleFrame. RFC text note: rfc1334.txt drops the line naming the Network-Layer Protocol phase, so the row quotes the surviving span.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestPAPReanswerPreservesDecision`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_phase_rfc1334_test.go#L181) | unit/verify | revert, verified |
| positive | [`TestPAPReanswersAfterAuthentication`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_phase_rfc1334_test.go#L148) | unit/verify | revert, verified |

### [`RFC1334-2.2.1-6`](#rfc1334-2.2.1-6)

Any Authenticate-Request packets received during any other phase MUST be silently discarded (Section 2.2.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestPAPRequestInAuthPhaseIsAnswered`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_phase_rfc1334_test.go#L78) | unit/verify | mutant, verified |
| positive | [`TestPAPRequestOutsideAuthPhaseIsSilentlyDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_phase_rfc1334_test.go#L112) | unit/verify | revert, verified |

### [`RFC1334-2.2-1`](#rfc1334-2.2-1)

The Identifier field MUST be changed each time an Authenticate-Request packet is issued. (Section 2.2.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Rejudge 2026-11-08: TestClientPAPRetriesUntilMatchingReply changed only in plumbing (frames through testFrameReader; runClientAuth now takes the shared Identifier counter, new(uint8)). runClientAuth increments that counter before the first request and before each retransmission (session.go). Assertions unchanged: the unanswered first request is reissued by the production timer with packets[0].Identifier != packets[1].Identifier and identical credentials; the negative half asserts no third request after the matching Ack.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestClientPAPRetriesUntilMatchingReply`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1334_pap_request_test.go#L127) | unit/verify | revert, verified |
| positive | [`TestClientPAPRetriesUntilMatchingReply`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1334_pap_request_test.go#L126) | unit/verify | revert, verified |

### [`RFC1334-2.3-3`](#rfc1334-2.3-3)

The Identifier field MUST be copied from the Identifier field of the Authenticate-Request which caused this reply. (Section 2.2.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent CURRENT-Core source rejudgment against RFC1334 Section2.2.2: each PAP reply copies the triggering Authenticate-Request Identifier. Complete TestPAPRequestEmitsEvent and TestPAPRejectWritesNak require first Ack0x42 and Nak0x77 respectively. TestPAPReanswersAfterAuthentication uses actual initial PAP acceptance then valid repeated requests0x5b/0x5c in AckSent and Opened, requiring exactly three ProtoPAP Ack replies with0x5a/0x5b/0x5c rather than cached initial identity. Read runPAPAuthPhase/writePAPReply/reanswerPAP: initial paths pass req.Identifier and reanswer passes the new payload Identifier, independently of cached Code. Both initial outcome branches and replacement request identity are observed; the repeat test carries both polarities but does not replace the independent first-reply contrasts. Current real request answers leave these assertions intact. Native recorder evidence remains owed.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestPAPReanswersAfterAuthentication`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_phase_rfc1334_test.go#L150) | unit/verify | revert, verified |
| positive | [`TestPAPReanswersAfterAuthentication`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/auth_phase_rfc1334_test.go#L149) | unit/verify | revert, verified |
| positive | [`TestPAPRejectWritesNak`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1334_pap_test.go#L414) | unit/verify | revert, verified |
| positive | [`TestPAPRequestEmitsEvent`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc1334_pap_test.go#L300) | unit/verify | revert, verified |

### [`RFC1334-2.3-4`](#rfc1334-2.3-4)

It is intended to be human readable, and MUST NOT affect operation of the protocol. (Section 2.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Rejudge 2026-11-08: TestPAPReplyMessageDoesNotAffectOutcome changed only in plumbing (testFrameReader, new(uint8) Identifier counter, which yields Identifier 1 matching the reply). Code held fixed per group, Message varied over '', 'welcome aboard', 'invalid credentials': every Ack succeeds, every Nak returns the 'PAP auth rejected' error, which an ignored Nak would not give. A Message-driven decision turns one group red.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestPAPReplyMessageDoesNotAffectOutcome`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1334_pap_message_test.go#L56) | unit/verify | revert, verified |
| positive | [`TestPAPReplyMessageDoesNotAffectOutcome`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1334_pap_message_test.go#L55) | unit/verify | revert, verified |

## Requirement list review

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
| `front` | not stated | 0 | reviewed | not stated |
| `1` | not stated | 1 | reviewed | not stated |
| `1.1` | not stated | 3 | reviewed | not stated |
| `1.2` | not stated | 0 | reviewed | not stated |
| `2` | not stated | 1 | reviewed | not stated |
| `2.1` | not stated | 0 | reviewed | not stated |
| `2.2` | not stated | 0 | reviewed | not stated |
| `2.2.1` | not stated | 7 | reviewed | not stated |
| `2.2.2` | not stated | 4 | reviewed | not stated |
| `3` | not stated | 1 | reviewed | not stated |
| `3.1` | not stated | 0 | reviewed | not stated |
| `3.2` | not stated | 0 | reviewed | not stated |
| `3.2.1` | not stated | 10 | reviewed | not stated |
| `3.2.2` | not stated | 4 | reviewed | not stated |

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
