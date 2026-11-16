# RFC 5998 - An Extension for EAP-Only Authentication in IKEv2

IKEv2 EAP. Every requirement listed for RFC 5998, the tests linked to it, and their recorded reviews. This summary is included in the automatic compliance check, ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0 | of 6 mandatory requirements | a positive test checks that Ze does what the requirement demands and a negative one checks that it refuses what the requirement forbids |
| One polarity plus reason | 1 | of 6 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason supplies the complete test coverage required |
| One polarity, unexcused | 0 | of 6 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial test coverage | 0 | of 6 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unverified, with zero whole-requirement credit |
| Tests shown to detect a fault | 1 | 1 of 1 tests linked to requirements | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests verifies a requirement only after both tests have been shown to detect a fault |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Declared gap or no test | 5 | of 6 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 6 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 0 | of 6 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 0 | of 6 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze verified: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 6 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 6 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no test verification credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level) | 6 | of 7 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements included in the automatic checks: the compliance check (./le rfc check) holds Ze to each of them. This is the total being assessed; the counts above show the recorded status of those requirements. |

The 9 counts marked as a part above are the whole of the 6 mandatory requirements: they add up to 6. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got. An outlined card with no colored edge, under Totals, is a total the other cards are counted against, not a measure.

| Card | Tone here | Why that color |
|---|---|---|
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the count under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: one test direction is missing without a recorded reason |
| Partial test coverage | ok | green at zero, RED above it: testing one clause does not verify the whole requirement |
| Declared gap or no test | bad | green at zero, RED above it: a binding obligation that nothing shows Ze meets is owed, whether or not a reason is stated |
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
| Public row | IKEv2 EAP |
| Automatic checks | Included |
| Requirements | 7 |
| Mandatory requirements (MUST level) | 6 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 5 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 1 |
| Tests linked to requirements | 1 |
| Recorded audit verdicts | 0 |
| Fault test records | 1 |
| Summary | `rfc/short/rfc5998.md` |
| Requirement table | `rfc/requirements/rfc5998.md` |
| RFC text | `rfc/full/rfc5998.txt` |

## Automatic checks

Included in automatic checks: EAP-only authentication in IKEv2: six MUST-level rows, every one classified. Ze implements no part of the extension: no EAP_ONLY_AUTHENTICATION (16417) notify is defined in internal/component/ike/wire/payload_notify.go, the initiator never sends it and the responder ignores it as an unrecognised status notify. Five rows are {gap}, because the vpn-concentrator profile requires this RFC. The resumption prohibition (RFC5998-3-5) holds because ze never sends the notify, and owes a tagged test. The AAA proxy trust rule binds the operator's AAA deployment.

## Published support status

**Area:** IKEv2 EAP

**Test verification:** 1 of 6 MUST proven

**Evidence:** unit 1

**Unmet:** RFC5998-3-1, RFC5998-3-2, RFC5998-3-3, RFC5998-3-4, RFC5998-4-1


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 0 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 6 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 0 | one part of the mandatory requirements |
| Partial test coverage (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level)** | **6** | every mandatory requirement appears in exactly one group above |

**Annotated (including scoped evidence) (6):** [`RFC5998-3-1`](#rfc5998-3-1), [`RFC5998-3-2`](#rfc5998-3-2), [`RFC5998-3-3`](#rfc5998-3-3), [`RFC5998-3-4`](#rfc5998-3-4), [`RFC5998-3-5`](#rfc5998-3-5), [`RFC5998-4-1`](#rfc5998-4-1)

## Figures

- **MUST bar**: Ze verified 1, Not built 5
- **Overview bar**: Ze MUST verified 1, Not assessed 1, Not built 5

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | not complete |
| MUST rows | 6 |
| MUST rows kept (the MUST bar) | 6 |
| Ze verified | 1 |
| OS verified | 0 |
| Tests awaiting verification | 0 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 5 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 1 |
| SHOULD verified | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD tests awaiting verification | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 1 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze verified means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Tests awaiting verification counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

Source: [data/rfc/rfc5998.json](../../../data/rfc/rfc5998.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC5998-3-1` | In this case, the initiator MUST verify that payload and any associated certificates, as per [RFC4306]. (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze never sends EAP_ONLY_AUTHENTICATION, so the initiator path this sentence conditions does not exist; internal/component/ike/wire/payload_notify.go |
| `RFC5998-3-2` | When receiving message 4, the initiator MUST verify that the proposed EAP method is allowed by this specification, and MUST abort the protocol immediately otherwise. (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze holds no list of methods this specification allows and no EAP-only initiator path; internal/component/ike/engine/eap_auth.go |
| `RFC5998-3-3` | Both the initiator and responder MUST verify that the EAP method actually used provided mutual authentication and established a shared secret key. (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the EAP-only exchange is not implemented, so no such verification exists; internal/component/ike/engine/eap_auth.go |
| `RFC5998-3-4` | The AUTH payloads sent after EAP Success MUST use the EAP-generated key, and MUST NOT use SK_pi or SK_pr (see Section 2.15 of [RFC5996]). (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the EAP-only exchange is not implemented; internal/component/ike/engine/eap_auth.go |
| `RFC5998-3-5` | Therefore, during the IKE_AUTH exchange of the resumed session, this extension MUST NOT be sent by the initiator. (§3) | MUST NOT | 3 | **positive:** `unit/verify` [`TestRFC5998IKEAuthCarriesNoEAPOnlyNotify`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/ike_auth_algorithms_test.go#L253). **negative:** no negative test. **{single-polarity}:** ze implements neither RFC 5998 nor RFC 5723 resumption and never sends EAP_ONLY_AUTHENTICATION in any IKE_AUTH, so no case exists in which sending it would be correct |
| `RFC5998-4-1` | EAP methods to be used with this extension MUST have the following properties: (§4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze applies no safe-method filter, because the extension is not implemented; internal/component/ike/engine/eap_auth.go |
| `RFC5998-6.2-1` | These AAA proxies MUST be trusted for secure operation, and therefore SHOULD be avoided when possible (§6.2) | SHOULD | 6.2 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC5998-3-1`](#rfc5998-3-1) In this case, the initiator MUST verify that payload and any associated certificates, as per [RFC4306]. (§3) | {gap}, no test | ze never sends EAP_ONLY_AUTHENTICATION, so the initiator path this sentence conditions does not exist; internal/component/ike/wire/payload_notify.go |
| [`RFC5998-3-2`](#rfc5998-3-2) When receiving message 4, the initiator MUST verify that the proposed EAP method is allowed by this specification, and MUST abort the protocol immediately otherwise. (§3) | {gap}, no test | ze holds no list of methods this specification allows and no EAP-only initiator path; internal/component/ike/engine/eap_auth.go |
| [`RFC5998-3-3`](#rfc5998-3-3) Both the initiator and responder MUST verify that the EAP method actually used provided mutual authentication and established a shared secret key. (§3) | {gap}, no test | the EAP-only exchange is not implemented, so no such verification exists; internal/component/ike/engine/eap_auth.go |
| [`RFC5998-3-4`](#rfc5998-3-4) The AUTH payloads sent after EAP Success MUST use the EAP-generated key, and MUST NOT use SK_pi or SK_pr (see Section 2.15 of [RFC5996]). (§3) | {gap}, no test | the EAP-only exchange is not implemented; internal/component/ike/engine/eap_auth.go |
| [`RFC5998-4-1`](#rfc5998-4-1) EAP methods to be used with this extension MUST have the following properties: (§4) | {gap}, no test | ze applies no safe-method filter, because the extension is not implemented; internal/component/ike/engine/eap_auth.go |

## Test status

A test reads not verified when no record shows it failing after a deliberate fault. Until then, the test has not been shown to detect a fault in the required behavior.

### [`RFC5998-3-1`](#rfc5998-3-1)

In this case, the initiator MUST verify that payload and any associated certificates, as per [RFC4306]. (§3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC5998-3-1, so there is no linked test.

### [`RFC5998-3-2`](#rfc5998-3-2)

When receiving message 4, the initiator MUST verify that the proposed EAP method is allowed by this specification, and MUST abort the protocol immediately otherwise. (§3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC5998-3-2, so there is no linked test.

### [`RFC5998-3-3`](#rfc5998-3-3)

Both the initiator and responder MUST verify that the EAP method actually used provided mutual authentication and established a shared secret key. (§3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC5998-3-3, so there is no linked test.

### [`RFC5998-3-4`](#rfc5998-3-4)

The AUTH payloads sent after EAP Success MUST use the EAP-generated key, and MUST NOT use SK_pi or SK_pr (see Section 2.15 of [RFC5996]). (§3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC5998-3-4, so there is no linked test.

### [`RFC5998-3-5`](#rfc5998-3-5)

Therefore, during the IKE_AUTH exchange of the resumed session, this extension MUST NOT be sent by the initiator. (§3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestRFC5998IKEAuthCarriesNoEAPOnlyNotify`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/ike_auth_algorithms_test.go#L253) | unit/verify | revert, verified |

### [`RFC5998-4-1`](#rfc5998-4-1)

EAP methods to be used with this extension MUST have the following properties: (§4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC5998-4-1, so there is no linked test.

## Requirement list review

| Field | Value |
|---|---|
| Reviewer | claude (ze-rfc phase-1 agent, session 45f03892) |
| Signed off | 2026-10-26 |
| Register | rfc2119 |
| Source | rfc/full/rfc5998.txt |
| Source fingerprint | 372e7f2780fe73e7 |
| Record | rfc/extraction/rfc5998.json |
| Mapped sentences | 6 |
| Declined as scope | 2 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | title, status and table of contents | 0 | skipped (front-matter) | title, status and table of contents |
| `1` | not stated | 0 | reviewed | not stated |
| `1.1` | not stated | 0 | reviewed | not stated |
| `2` | not stated | 0 | reviewed | not stated |
| `3` | not stated | 5 | reviewed | not stated |
| `4` | not stated | 1 | reviewed | not stated |
| `5` | IANA considerations | 0 | skipped (iana) | IANA considerations |
| `6` | not stated | 0 | reviewed | not stated |
| `6.1` | not stated | 1 | reviewed | not stated |
| `6.2` | not stated | 1 | reviewed | not stated |
| `6.3` | not stated | 0 | reviewed | not stated |
| `6.4` | not stated | 0 | reviewed | not stated |
| `6.5` | not stated | 0 | reviewed | not stated |
| `7` | acknowledgments | 0 | skipped (acknowledgements) | acknowledgments |
| `8` | references | 0 | skipped (references) | references |
| `8.1` | normative references | 0 | skipped (references) | normative references |
| `8.2` | informative references | 0 | skipped (references) | informative references |
| `A` | alternative approaches the authors rejected | 0 | skipped (appendix-non-normative) | alternative approaches the authors rejected |
| `A.1` | rejected alternative | 0 | skipped (appendix-non-normative) | rejected alternative |
| `A.2` | rejected alternative | 0 | skipped (appendix-non-normative) | rejected alternative |
| `A.3` | rejected alternative | 0 | skipped (appendix-non-normative) | rejected alternative |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `6.1:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | restates the Section 3 rule that the EAP method used must provide mutual authentication and a shared secret key | Thus, EAP methods that do not provide mutual authentication or establish a shared secret key MUST NOT be used with the modifications presented in this document. |
| `6.2:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | AAA deployment operator: the sentence obliges whoever places AAA proxies between the IKEv2 gateway and the backend AAA server to trust them; no IKEv2 implementation, ze's included, chooses or vets the AAA path | These AAA proxies MUST be trusted for secure operation, and therefore SHOULD be avoided when possible; see Section 2.3.4 of [RFC4072] and Section 4.3.7 of [RFC3579] for more discussion. |

## Superseded

No document obsoletes RFC 5998, so its obligations are stated where they were written.
