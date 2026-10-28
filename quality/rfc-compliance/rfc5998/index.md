# RFC 5998 - An Extension for EAP-Only Authentication in IKEv2

Unsupported. Every requirement this repository extracted from RFC 5998, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0.0% | 0 of 6 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 16.7% | 1 of 6 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 6 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 6 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Proven by a recorded break | 100.0% | 1 of 1 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 6 | of 7 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 6 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 6 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 6 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 6 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |
| Linux cannot do this | 0.0% | 0 of 6 gated MUSTs | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and remains unmet; it stays in every denominator and earns no proof credit |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 83.3% | 5 of 6 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

The 9 shares marked as a part above are the whole of the 6 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Linux cannot do this | neutral | neutral at every value: an evidenced platform limit is neither a proof nor a scope exemption |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Unsupported |
| Enrolment | Enrolled |
| Requirements | 7 |
| Gated MUST-level | 6 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 5 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 1 |
| Tagged units | 1 |
| Recorded audit verdicts | 0 |
| Discrimination records | 1 |
| Summary | `rfc/short/rfc5998.md` |
| Requirement shard | `rfc/requirements/rfc5998.md` |
| RFC text | `rfc/full/rfc5998.txt` |

## Enrolment

Enrolled: EAP-only authentication in IKEv2: six MUST-level rows, every one classified. Ze implements no part of the extension: no EAP_ONLY_AUTHENTICATION (16417) notify is defined in internal/component/ike/wire/payload_notify.go, the initiator never sends it and the responder ignores it as an unrecognised status notify. Five rows are {gap}, because the vpn-concentrator profile requires this RFC. The resumption prohibition (RFC5998-3-5) holds because ze never sends the notify, and owes a tagged test. The AAA proxy trust rule binds the operator's AAA deployment.

## What the public ledger says

**Status:** Unsupported

**What the ledger says is covered:**

None: ze's EAP in IKEv2 always pairs EAP with a public-key AUTH payload from the responder (RFC 7296 Section 2.16).

**What the ledger says remains:**

The EAP_ONLY_AUTHENTICATION notify (16417), the safe-method check (Section 4) and the EAP-key-only AUTH computation are not implemented: [`RFC5998-3-1`](#rfc5998-3-1), [`RFC5998-3-2`](#rfc5998-3-2), [`RFC5998-3-3`](#rfc5998-3-3), [`RFC5998-3-4`](#rfc5998-3-4) and [`RFC5998-4-1`](#rfc5998-4-1) are gaps.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 0 | one part of the gated population |
| Annotated (including scoped evidence) | 6 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **6** | every gated MUST falls in exactly one bucket above |

**Annotated (including scoped evidence) (6):** [`RFC5998-3-1`](#rfc5998-3-1), [`RFC5998-3-2`](#rfc5998-3-2), [`RFC5998-3-3`](#rfc5998-3-3), [`RFC5998-3-4`](#rfc5998-3-4), [`RFC5998-3-5`](#rfc5998-3-5), [`RFC5998-4-1`](#rfc5998-4-1)

## Attainable ceiling

Mandatory ceiling: 6 of 6 (100.0%). Optional ceiling: 1 of 1 (100.0%). Every RFC shown with measured rows can reach 100% under current classifications. Lowest mandatory ceiling: rfc5998 6 of 6 (100.0%). Lowest optional ceiling: rfc5998 1 of 1 (100.0%). Ceilings describe current classifications, not an immutable Linux maximum. Not-applicable and feature-declined classifications can be revisited. Every gap remains work Ze owes.

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

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC5998-3-1`](#rfc5998-3-1)

In this case, the initiator MUST verify that payload and any associated certificates, as per [RFC4306]. (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5998-3-1, so no unit is bound to it.

### [`RFC5998-3-2`](#rfc5998-3-2)

When receiving message 4, the initiator MUST verify that the proposed EAP method is allowed by this specification, and MUST abort the protocol immediately otherwise. (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5998-3-2, so no unit is bound to it.

### [`RFC5998-3-3`](#rfc5998-3-3)

Both the initiator and responder MUST verify that the EAP method actually used provided mutual authentication and established a shared secret key. (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5998-3-3, so no unit is bound to it.

### [`RFC5998-3-4`](#rfc5998-3-4)

The AUTH payloads sent after EAP Success MUST use the EAP-generated key, and MUST NOT use SK_pi or SK_pr (see Section 2.15 of [RFC5996]). (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5998-3-4, so no unit is bound to it.

### [`RFC5998-3-5`](#rfc5998-3-5)

Therefore, during the IKE_AUTH exchange of the resumed session, this extension MUST NOT be sent by the initiator. (§3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC5998IKEAuthCarriesNoEAPOnlyNotify`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/ike_auth_algorithms_test.go#L253) | unit/verify | revert, verified |

### [`RFC5998-4-1`](#rfc5998-4-1)

EAP methods to be used with this extension MUST have the following properties: (§4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5998-4-1, so no unit is bound to it.

## Extraction sign-off

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
| `1` | not stated | 0 | walked | not stated |
| `1.1` | not stated | 0 | walked | not stated |
| `2` | not stated | 0 | walked | not stated |
| `3` | not stated | 5 | walked | not stated |
| `4` | not stated | 1 | walked | not stated |
| `5` | IANA considerations | 0 | skipped (iana) | IANA considerations |
| `6` | not stated | 0 | walked | not stated |
| `6.1` | not stated | 1 | walked | not stated |
| `6.2` | not stated | 1 | walked | not stated |
| `6.3` | not stated | 0 | walked | not stated |
| `6.4` | not stated | 0 | walked | not stated |
| `6.5` | not stated | 0 | walked | not stated |
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
