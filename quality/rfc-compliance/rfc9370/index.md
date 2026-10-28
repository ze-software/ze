# RFC 9370 - Multiple Key Exchanges in the Internet Key Exchange Protocol Version 2 (IKEv2)

Future. Every requirement this repository extracted from RFC 9370, the tests bound to it, and what a reader has verified about them. This summary is not enrolled.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0.0% | 0 of 23 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 23 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 23 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 23 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Proven by a recorded break | 0.0% | 0 of 0 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| MUSTs declared | 23 | of 27 this summary declares | MUST-level requirements this summary DECLARES. The gate holds none of them, because this RFC is not enrolled (out-of-scope), so every share below reads what the summary records rather than what the gate enforces |
| Out of scope | 0 | of 23 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 23 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 23 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 23 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 100.0% | 23 of 23 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

The 8 shares marked as a part above are the whole of the 23 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| MUSTs declared | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
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
| Public status | Future |
| Enrolment | Not enrolled (out-of-scope) |
| Requirements | 27 |
| Gated MUST-level | 23 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 23 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 0 |
| Tagged units | 0 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc9370.md` |
| Requirement shard | `rfc/requirements/rfc9370.md` |
| RFC text | `rfc/full/rfc9370.txt` |

## Enrolment

Not enrolled (out-of-scope, the requirements ARE extracted and the owner decided not to offer the feature for now, so the absence is a scope decision rather than a conformance gap): OUT OF SCOPE as a document by owner decision, 2026-10-26: the owner rule committed in 1e7a9dbc34 (ai/META-DECISIONS.md, Goal and scope) counts an RFC a profile lists as optional only once Thomas brings it into scope, and this summary's `Optional use cases` row lists this RFC in the optional tier of the vpn-endpoint and vpn-concentrator profiles. The extraction is COMPLETE: the source text is at rfc/full/rfc9370.txt and the extraction sign-off is rfc/extraction/rfc9370.json. Ze implements none of it, and each MUST-level row keeps its `{gap}` note naming the absent producer: all 23 MUST-level requirements are classified, and all 23 are `{gap}`, because Ze's IKE engine implements neither the IKE_INTERMEDIATE exchange (RFC 9242) nor IKE_FOLLOWUP_KE, nor the ADDKE Transform Types 6 to 12. `internal/component/ike/wire/header.go` declares exchange types 34 to 37 only, and `internal/component/ike/wire/payload_notify.go` declares neither ADDITIONAL_KEY_EXCHANGE (16441) nor STATE_NOT_FOUND (47). One Appendix C site states the rationale for a discarded alternative design and is excluded in the extraction sign-off.

## What the public ledger says

**Status:** Future

**What the ledger says is covered**

None: an IKE SA and each Child SA run one Transform Type 4 key exchange. A proposal carrying a Transform Type the engine does not use is refused as a whole proposal (`internal/component/ike/crypto/proposal.go::TransformTypeUnderstoodIKE`, `ikeProposalComplete`), which is how an ADDKE Transform Type is treated today, but no test asserts that against an ADDKE type.

**What the ledger says remains**

ADDKE Transform Types 6 to 12, INTERMEDIATE_EXCHANGE_SUPPORTED negotiation, IKE_INTERMEDIATE key exchange rounds, IKE_FOLLOWUP_KE (44), ADDITIONAL_KEY_EXCHANGE (16441), STATE_NOT_FOUND (47) and the multi-secret SKEYSEED update are absent. All 23 MUST-level requirements are annotated `{gap}` in [`rfc/short/rfc9370.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc9370.md). Out of scope by owner decision, 2026-10-26.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 0 | one part of the gated population |
| Annotated (including scoped evidence) | 23 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **23** | every gated MUST falls in exactly one bucket above |

**Annotated (including scoped evidence) (23):** [`RFC9370-2.2.1-1`](#rfc9370-2.2.1-1), [`RFC9370-2.2.1-2`](#rfc9370-2.2.1-2), [`RFC9370-2.2.1-3`](#rfc9370-2.2.1-3), [`RFC9370-2.2.1-4`](#rfc9370-2.2.1-4), [`RFC9370-2.2.1-5`](#rfc9370-2.2.1-5), [`RFC9370-2.2.1-6`](#rfc9370-2.2.1-6), [`RFC9370-2.2.1-7`](#rfc9370-2.2.1-7), [`RFC9370-2.2.1-8`](#rfc9370-2.2.1-8), [`RFC9370-2.2.1-9`](#rfc9370-2.2.1-9), [`RFC9370-2.2.1-10`](#rfc9370-2.2.1-10), [`RFC9370-2.2.1-11`](#rfc9370-2.2.1-11), [`RFC9370-2.2.1-12`](#rfc9370-2.2.1-12), [`RFC9370-2.2.2-1`](#rfc9370-2.2.2-1), [`RFC9370-2.2.4-1`](#rfc9370-2.2.4-1), [`RFC9370-2.2.4-2`](#rfc9370-2.2.4-2), [`RFC9370-2.2.4-3`](#rfc9370-2.2.4-3), [`RFC9370-2.2.4-4`](#rfc9370-2.2.4-4), [`RFC9370-2.2.4-5`](#rfc9370-2.2.4-5), [`RFC9370-2.2.4-6`](#rfc9370-2.2.4-6), [`RFC9370-2.2.4-7`](#rfc9370-2.2.4-7), [`RFC9370-2.2.4-8`](#rfc9370-2.2.4-8), [`RFC9370-2.2.4-9`](#rfc9370-2.2.4-9), [`RFC9370-2.2.4-10`](#rfc9370-2.2.4-10)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC9370-2.2.1-1` | Additional key exchanges negotiated via newly defined transforms MUST take place in a series of IKE_INTERMEDIATE exchanges following the IKE_SA_INIT exchange, performed in an order of the values of their Transform Types. (§2.2.1) | MUST | 2.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze has no IKE_INTERMEDIATE exchange: internal/component/ike/wire/header.go declares exchange types 34 to 37 only, so no additional key exchange ever runs; internal/component/ike/wire/header.go |
| `RFC9370-2.2.1-2` | Each additional key exchange method MUST be fully completed before the next one is started. (§2.2.1) | MUST | 2.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no additional key exchange exists to order, because Ze implements no IKE_INTERMEDIATE round; internal/component/ike/wire/header.go |
| `RFC9370-2.2.1-3` | On the other hand, if the initiator includes any ADDKE Transform Type in the proposal, the responder MUST select one of the algorithms proposed using this type. (§2.2.1) | MUST | 2.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the Ze responder selects no ADDKE algorithm; internal/component/ike/crypto/proposal.go::TransformTypeUnderstoodIKE reads Transform Types 6 to 12 as not understood and the proposal is refused whole; internal/component/ike/crypto/proposal.go |
| `RFC9370-2.2.1-4` | However, for the ADDKE Transform Types, the responder's choice MUST NOT contain duplicated algorithms (those with an identical Transform ID and attributes), except for the Transform ID of NONE. (§2.2.1) | MUST NOT | 2.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze makes no ADDKE choice at all; internal/component/ike/crypto/proposal.go::TransformTypeUnderstoodIKE; internal/component/ike/crypto/proposal.go |
| `RFC9370-2.2.1-5` | If the responder is unable to select algorithms that are not duplicated for each proposed key exchange (either because the proposal contains too few choices or due to the local policy restrictions on using the proposed algorithms), then the responder MUST reject the message with an error notification of type NO_PROPOSAL_CHOSEN. (§2.2.1) | MUST | 2.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no ADDKE selection exists in which this failure could arise; internal/component/ike/crypto/proposal.go::negotiateIKE; internal/component/ike/crypto/proposal.go |
| `RFC9370-2.2.1-6` | If the responder's message contains one or more duplicated choices, the initiator should log the error and MUST treat the exchange as failed. (§2.2.1) | MUST | 2.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the Ze initiator never proposes ADDKE transforms, so it has no check for duplicated ADDKE choices in a response; internal/component/ike/crypto/proposal.go::VerifyAcceptedIKE; internal/component/ike/crypto/proposal.go |
| `RFC9370-2.2.1-7` | The initiator MUST NOT initiate any IKE_INTERMEDIATE (or IKE_FOLLOWUP_KE) exchanges so that no new SA is created. (§2.2.1) | MUST NOT | 2.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze implements neither exchange type, so the duplicated-choice failure path this sentence governs does not exist; internal/component/ike/wire/header.go |
| `RFC9370-2.2.1-8` | If the responder selects NONE for some ADDKE Transform Types (provided they are proposed by the initiator), then any corresponding additional key exchanges MUST NOT take place. (§2.2.1) | MUST NOT | 2.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze never proposes or selects an ADDKE Transform Type; internal/component/ike/crypto/proposal.go::TransformTypeUnderstoodIKE; internal/component/ike/crypto/proposal.go |
| `RFC9370-2.2.1-9` | The responder MUST treat all of the omitted ADDKE transforms as if they were proposed with Transform ID NONE. (§2.2.1) | MUST | 2.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze has no ADDKE negotiation state in which an omitted type could be read as NONE; internal/component/ike/crypto/proposal.go::negotiateIKE; internal/component/ike/crypto/proposal.go |
| `RFC9370-2.2.1-10` | If the initiator includes any ADDKE Transform Types into the SA payload in the IKE_SA_INIT exchange request message, then it MUST also negotiate the use of the IKE_INTERMEDIATE exchange, as described in [RFC9242] by including an INTERMEDIATE_EXCHANGE_SUPPORTED notification in the same message. (§2.2.1) | MUST | 2.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze sends neither ADDKE transforms nor INTERMEDIATE_EXCHANGE_SUPPORTED; internal/component/ike/wire/payload_notify.go declares no such Notify Message Type; internal/component/ike/wire/payload_notify.go |
| `RFC9370-2.2.1-11` | If the responder agrees to use additional key exchanges while establishing an initial IKE SA, it MUST also return this notification in the IKE_SA_INIT response message, confirming that IKE_INTERMEDIATE exchange is supported and will be used for transferring additional key exchange data. (§2.2.1) | MUST | 2.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the Ze responder never agrees to additional key exchanges and returns no INTERMEDIATE_EXCHANGE_SUPPORTED; internal/component/ike/wire/payload_notify.go |
| `RFC9370-2.2.1-12` | If the IKE_INTERMEDIATE exchange is not negotiated, then the peers MUST treat any ADDKE Transform Types in the IKE_SA_INIT exchange messages as unknown transform types and skip the proposals they appear in. (§2.2.1) | MUST | 2.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** unproven rather than known divergent. Ze never negotiates IKE_INTERMEDIATE, and internal/component/ike/crypto/proposal.go::TransformTypeUnderstoodIKE reads Transform Types 6 to 12 as not understood, so ikeProposalComplete refuses each proposal carrying one; no test tags this requirement with an ADDKE Transform Type, so it stays a gap until one does; internal/component/ike/crypto/proposal.go |
| `RFC9370-2.2.2-1` | The former "Diffie-Hellman Group Num" (now called "Key Exchange Method") field in the KEi(n) and KEr(n) payloads MUST match the n-th negotiated additional key exchange. (§2.2.2) | MUST | 2.2.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze sends and reads no KEi(n) or KEr(n) payload, because no IKE_INTERMEDIATE round exists; internal/component/ike/wire/header.go |
| `RFC9370-2.2.4-1` | Each additional key exchange method MUST be fully completed before the next one is started. (§2.2.4) | MUST | 2.2.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze has no IKE_FOLLOWUP_KE exchange, so no additional key exchange follows CREATE_CHILD_SA; internal/component/ike/wire/header.go |
| `RFC9370-2.2.4-2` | The responder MUST include this notification in a CREATE_CHILD_SA or IKE_FOLLOWUP_KE response message in case the next IKE_FOLLOWUP_KE exchange is expected, filling it with some data that would allow linking the current exchange to the next one. (§2.2.4) | MUST | 2.2.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze never sends ADDITIONAL_KEY_EXCHANGE (16441); internal/component/ike/wire/payload_notify.go declares no such Notify Message Type; internal/component/ike/wire/payload_notify.go |
| `RFC9370-2.2.4-3` | The initiator MUST send back this notification intact in the request message of the next IKE_FOLLOWUP_KE exchange. (§2.2.4) | MUST | 2.2.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze neither receives nor echoes ADDITIONAL_KEY_EXCHANGE; internal/component/ike/wire/payload_notify.go |
| `RFC9370-2.2.4-4` | The former "Diffie-Hellman Group Num" (now called "Key Exchange Method") field in the KEi(n) and KEr(n) payloads MUST match the n-th negotiated additional key exchange. (§2.2.4) | MUST | 2.2.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze sends and reads no KEi(n) or KEr(n) payload, because no IKE_FOLLOWUP_KE exchange exists; internal/component/ike/wire/header.go |
| `RFC9370-2.2.4-5` | The responder MUST handle this situation gracefully and delete the associated state if it does not receive the next expected IKE_FOLLOWUP_KE request after some reasonable period of time. (§2.2.4) | MUST | 2.2.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze holds no pending IKE_FOLLOWUP_KE state to expire; internal/component/ike/wire/header.go |
| `RFC9370-2.2.4-6` | If the responder receives an IKE_FOLLOWUP_KE message for which it does not have a key exchange state, it MUST send back a new error type notification called "STATE_NOT_FOUND". (§2.2.4) | MUST | 2.2.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze does not recognize exchange type 44 and declares no STATE_NOT_FOUND (47) notification; internal/component/ike/wire/payload_notify.go |
| `RFC9370-2.2.4-7` | If the initiator receives this notification in response to an IKE_FOLLOWUP_KE exchange performing an additional key exchange, it MUST cancel this exchange and MUST treat the whole series of exchanges started from the CREATE_CHILD_SA exchange as having failed. (§2.2.4) | MUST | 2.2.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze starts no IKE_FOLLOWUP_KE exchange and handles no STATE_NOT_FOUND; internal/component/ike/wire/payload_notify.go |
| `RFC9370-2.2.4-8` | If the initiator continues to receive STATE_NOT_FOUND notifications after several retries, it MUST treat this situation as a fatal error and delete the IKE SA by sending a DELETE payload. (§2.2.4) | MUST | 2.2.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze handles no STATE_NOT_FOUND and has no IKE_FOLLOWUP_KE retry; internal/component/ike/wire/payload_notify.go |
| `RFC9370-2.2.4-9` | For this reason, if additional key exchanges are negotiated in the CREATE_CHILD_SA exchange in which the smallest nonce is used, then, because there is nothing to delete yet, the initiator of this exchange just stops the rekeying process, and it MUST NOT initiate the IKE_FOLLOWUP_KE exchange. (§2.2.4) | MUST NOT | 2.2.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze negotiates no additional key exchange in CREATE_CHILD_SA, so the simultaneous-rekey rule for IKE_FOLLOWUP_KE has no path; internal/component/ike/wire/header.go |
| `RFC9370-2.2.4-10` | In this case, a TEMPORARY_FAILURE notification MUST be sent in response to such a request. (§2.2.4) | MUST | 2.2.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the sentence answers a CREATE_CHILD_SA rekey request that arrives while this peer's own IKE_FOLLOWUP_KE series is in progress; Ze never runs such a series, so the TEMPORARY_FAILURE answer for that collision is not built; internal/component/ike/wire/header.go |
| `RFC9370-2.2.1-13` | A Transform ID NONE MAY be added to those transform types that contain key exchange methods which the initiator believes are optional according to its local policy. (§2.2.1) | MAY | 2.2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9370-2.2.1-14` | If this happens in the CREATE_CHILD_SA exchange, then the initiator MAY delete the IKE SA over which the invalid message was received by sending a Delete payload. (§2.2.1) | MAY | 2.2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9370-2.2.1-15` | The initiator MAY propose ADDKE Transform Types that are not consecutive, for example, proposing ADDKE2 and ADDKE5 Transform Types only. (§2.2.1) | MAY | 2.2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9370-2.2.4-11` | After receiving this notification, the initiator MAY start a new CREATE_CHILD_SA exchange, which may eventually be followed by the IKE_FOLLOWUP_KE exchanges, to retry the failed attempt. (§2.2.4) | MAY | 2.2.4 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC9370-2.2.1-1`](#rfc9370-2.2.1-1) Additional key exchanges negotiated via newly defined transforms MUST take place in a series of IKE_INTERMEDIATE exchanges following the IKE_SA_INIT exchange, performed in an order of the values of their Transform Types. (§2.2.1) | {gap}, no test | Ze has no IKE_INTERMEDIATE exchange: internal/component/ike/wire/header.go declares exchange types 34 to 37 only, so no additional key exchange ever runs; internal/component/ike/wire/header.go |
| [`RFC9370-2.2.1-2`](#rfc9370-2.2.1-2) Each additional key exchange method MUST be fully completed before the next one is started. (§2.2.1) | {gap}, no test | no additional key exchange exists to order, because Ze implements no IKE_INTERMEDIATE round; internal/component/ike/wire/header.go |
| [`RFC9370-2.2.1-3`](#rfc9370-2.2.1-3) On the other hand, if the initiator includes any ADDKE Transform Type in the proposal, the responder MUST select one of the algorithms proposed using this type. (§2.2.1) | {gap}, no test | the Ze responder selects no ADDKE algorithm; internal/component/ike/crypto/proposal.go::TransformTypeUnderstoodIKE reads Transform Types 6 to 12 as not understood and the proposal is refused whole; internal/component/ike/crypto/proposal.go |
| [`RFC9370-2.2.1-4`](#rfc9370-2.2.1-4) However, for the ADDKE Transform Types, the responder's choice MUST NOT contain duplicated algorithms (those with an identical Transform ID and attributes), except for the Transform ID of NONE. (§2.2.1) | {gap}, no test | Ze makes no ADDKE choice at all; internal/component/ike/crypto/proposal.go::TransformTypeUnderstoodIKE; internal/component/ike/crypto/proposal.go |
| [`RFC9370-2.2.1-5`](#rfc9370-2.2.1-5) If the responder is unable to select algorithms that are not duplicated for each proposed key exchange (either because the proposal contains too few choices or due to the local policy restrictions on using the proposed algorithms), then the responder MUST reject the message with an error notification of type NO_PROPOSAL_CHOSEN. (§2.2.1) | {gap}, no test | no ADDKE selection exists in which this failure could arise; internal/component/ike/crypto/proposal.go::negotiateIKE; internal/component/ike/crypto/proposal.go |
| [`RFC9370-2.2.1-6`](#rfc9370-2.2.1-6) If the responder's message contains one or more duplicated choices, the initiator should log the error and MUST treat the exchange as failed. (§2.2.1) | {gap}, no test | the Ze initiator never proposes ADDKE transforms, so it has no check for duplicated ADDKE choices in a response; internal/component/ike/crypto/proposal.go::VerifyAcceptedIKE; internal/component/ike/crypto/proposal.go |
| [`RFC9370-2.2.1-7`](#rfc9370-2.2.1-7) The initiator MUST NOT initiate any IKE_INTERMEDIATE (or IKE_FOLLOWUP_KE) exchanges so that no new SA is created. (§2.2.1) | {gap}, no test | Ze implements neither exchange type, so the duplicated-choice failure path this sentence governs does not exist; internal/component/ike/wire/header.go |
| [`RFC9370-2.2.1-8`](#rfc9370-2.2.1-8) If the responder selects NONE for some ADDKE Transform Types (provided they are proposed by the initiator), then any corresponding additional key exchanges MUST NOT take place. (§2.2.1) | {gap}, no test | Ze never proposes or selects an ADDKE Transform Type; internal/component/ike/crypto/proposal.go::TransformTypeUnderstoodIKE; internal/component/ike/crypto/proposal.go |
| [`RFC9370-2.2.1-9`](#rfc9370-2.2.1-9) The responder MUST treat all of the omitted ADDKE transforms as if they were proposed with Transform ID NONE. (§2.2.1) | {gap}, no test | Ze has no ADDKE negotiation state in which an omitted type could be read as NONE; internal/component/ike/crypto/proposal.go::negotiateIKE; internal/component/ike/crypto/proposal.go |
| [`RFC9370-2.2.1-10`](#rfc9370-2.2.1-10) If the initiator includes any ADDKE Transform Types into the SA payload in the IKE_SA_INIT exchange request message, then it MUST also negotiate the use of the IKE_INTERMEDIATE exchange, as described in [RFC9242] by including an INTERMEDIATE_EXCHANGE_SUPPORTED notification in the same message. (§2.2.1) | {gap}, no test | Ze sends neither ADDKE transforms nor INTERMEDIATE_EXCHANGE_SUPPORTED; internal/component/ike/wire/payload_notify.go declares no such Notify Message Type; internal/component/ike/wire/payload_notify.go |
| [`RFC9370-2.2.1-11`](#rfc9370-2.2.1-11) If the responder agrees to use additional key exchanges while establishing an initial IKE SA, it MUST also return this notification in the IKE_SA_INIT response message, confirming that IKE_INTERMEDIATE exchange is supported and will be used for transferring additional key exchange data. (§2.2.1) | {gap}, no test | the Ze responder never agrees to additional key exchanges and returns no INTERMEDIATE_EXCHANGE_SUPPORTED; internal/component/ike/wire/payload_notify.go |
| [`RFC9370-2.2.1-12`](#rfc9370-2.2.1-12) If the IKE_INTERMEDIATE exchange is not negotiated, then the peers MUST treat any ADDKE Transform Types in the IKE_SA_INIT exchange messages as unknown transform types and skip the proposals they appear in. (§2.2.1) | {gap}, no test | unproven rather than known divergent. Ze never negotiates IKE_INTERMEDIATE, and internal/component/ike/crypto/proposal.go::TransformTypeUnderstoodIKE reads Transform Types 6 to 12 as not understood, so ikeProposalComplete refuses each proposal carrying one; no test tags this requirement with an ADDKE Transform Type, so it stays a gap until one does; internal/component/ike/crypto/proposal.go |
| [`RFC9370-2.2.2-1`](#rfc9370-2.2.2-1) The former "Diffie-Hellman Group Num" (now called "Key Exchange Method") field in the KEi(n) and KEr(n) payloads MUST match the n-th negotiated additional key exchange. (§2.2.2) | {gap}, no test | Ze sends and reads no KEi(n) or KEr(n) payload, because no IKE_INTERMEDIATE round exists; internal/component/ike/wire/header.go |
| [`RFC9370-2.2.4-1`](#rfc9370-2.2.4-1) Each additional key exchange method MUST be fully completed before the next one is started. (§2.2.4) | {gap}, no test | Ze has no IKE_FOLLOWUP_KE exchange, so no additional key exchange follows CREATE_CHILD_SA; internal/component/ike/wire/header.go |
| [`RFC9370-2.2.4-2`](#rfc9370-2.2.4-2) The responder MUST include this notification in a CREATE_CHILD_SA or IKE_FOLLOWUP_KE response message in case the next IKE_FOLLOWUP_KE exchange is expected, filling it with some data that would allow linking the current exchange to the next one. (§2.2.4) | {gap}, no test | Ze never sends ADDITIONAL_KEY_EXCHANGE (16441); internal/component/ike/wire/payload_notify.go declares no such Notify Message Type; internal/component/ike/wire/payload_notify.go |
| [`RFC9370-2.2.4-3`](#rfc9370-2.2.4-3) The initiator MUST send back this notification intact in the request message of the next IKE_FOLLOWUP_KE exchange. (§2.2.4) | {gap}, no test | Ze neither receives nor echoes ADDITIONAL_KEY_EXCHANGE; internal/component/ike/wire/payload_notify.go |
| [`RFC9370-2.2.4-4`](#rfc9370-2.2.4-4) The former "Diffie-Hellman Group Num" (now called "Key Exchange Method") field in the KEi(n) and KEr(n) payloads MUST match the n-th negotiated additional key exchange. (§2.2.4) | {gap}, no test | Ze sends and reads no KEi(n) or KEr(n) payload, because no IKE_FOLLOWUP_KE exchange exists; internal/component/ike/wire/header.go |
| [`RFC9370-2.2.4-5`](#rfc9370-2.2.4-5) The responder MUST handle this situation gracefully and delete the associated state if it does not receive the next expected IKE_FOLLOWUP_KE request after some reasonable period of time. (§2.2.4) | {gap}, no test | Ze holds no pending IKE_FOLLOWUP_KE state to expire; internal/component/ike/wire/header.go |
| [`RFC9370-2.2.4-6`](#rfc9370-2.2.4-6) If the responder receives an IKE_FOLLOWUP_KE message for which it does not have a key exchange state, it MUST send back a new error type notification called "STATE_NOT_FOUND". (§2.2.4) | {gap}, no test | Ze does not recognize exchange type 44 and declares no STATE_NOT_FOUND (47) notification; internal/component/ike/wire/payload_notify.go |
| [`RFC9370-2.2.4-7`](#rfc9370-2.2.4-7) If the initiator receives this notification in response to an IKE_FOLLOWUP_KE exchange performing an additional key exchange, it MUST cancel this exchange and MUST treat the whole series of exchanges started from the CREATE_CHILD_SA exchange as having failed. (§2.2.4) | {gap}, no test | Ze starts no IKE_FOLLOWUP_KE exchange and handles no STATE_NOT_FOUND; internal/component/ike/wire/payload_notify.go |
| [`RFC9370-2.2.4-8`](#rfc9370-2.2.4-8) If the initiator continues to receive STATE_NOT_FOUND notifications after several retries, it MUST treat this situation as a fatal error and delete the IKE SA by sending a DELETE payload. (§2.2.4) | {gap}, no test | Ze handles no STATE_NOT_FOUND and has no IKE_FOLLOWUP_KE retry; internal/component/ike/wire/payload_notify.go |
| [`RFC9370-2.2.4-9`](#rfc9370-2.2.4-9) For this reason, if additional key exchanges are negotiated in the CREATE_CHILD_SA exchange in which the smallest nonce is used, then, because there is nothing to delete yet, the initiator of this exchange just stops the rekeying process, and it MUST NOT initiate the IKE_FOLLOWUP_KE exchange. (§2.2.4) | {gap}, no test | Ze negotiates no additional key exchange in CREATE_CHILD_SA, so the simultaneous-rekey rule for IKE_FOLLOWUP_KE has no path; internal/component/ike/wire/header.go |
| [`RFC9370-2.2.4-10`](#rfc9370-2.2.4-10) In this case, a TEMPORARY_FAILURE notification MUST be sent in response to such a request. (§2.2.4) | {gap}, no test | the sentence answers a CREATE_CHILD_SA rekey request that arrives while this peer's own IKE_FOLLOWUP_KE series is in progress; Ze never runs such a series, so the TEMPORARY_FAILURE answer for that collision is not built; internal/component/ike/wire/header.go |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC9370-2.2.1-1`](#rfc9370-2.2.1-1)

Additional key exchanges negotiated via newly defined transforms MUST take place in a series of IKE_INTERMEDIATE exchanges following the IKE_SA_INIT exchange, performed in an order of the values of their Transform Types. (§2.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9370-2.2.1-1, so no unit is bound to it.

### [`RFC9370-2.2.1-2`](#rfc9370-2.2.1-2)

Each additional key exchange method MUST be fully completed before the next one is started. (§2.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9370-2.2.1-2, so no unit is bound to it.

### [`RFC9370-2.2.1-3`](#rfc9370-2.2.1-3)

On the other hand, if the initiator includes any ADDKE Transform Type in the proposal, the responder MUST select one of the algorithms proposed using this type. (§2.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9370-2.2.1-3, so no unit is bound to it.

### [`RFC9370-2.2.1-4`](#rfc9370-2.2.1-4)

However, for the ADDKE Transform Types, the responder's choice MUST NOT contain duplicated algorithms (those with an identical Transform ID and attributes), except for the Transform ID of NONE. (§2.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9370-2.2.1-4, so no unit is bound to it.

### [`RFC9370-2.2.1-5`](#rfc9370-2.2.1-5)

If the responder is unable to select algorithms that are not duplicated for each proposed key exchange (either because the proposal contains too few choices or due to the local policy restrictions on using the proposed algorithms), then the responder MUST reject the message with an error notification of type NO_PROPOSAL_CHOSEN. (§2.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9370-2.2.1-5, so no unit is bound to it.

### [`RFC9370-2.2.1-6`](#rfc9370-2.2.1-6)

If the responder's message contains one or more duplicated choices, the initiator should log the error and MUST treat the exchange as failed. (§2.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9370-2.2.1-6, so no unit is bound to it.

### [`RFC9370-2.2.1-7`](#rfc9370-2.2.1-7)

The initiator MUST NOT initiate any IKE_INTERMEDIATE (or IKE_FOLLOWUP_KE) exchanges so that no new SA is created. (§2.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9370-2.2.1-7, so no unit is bound to it.

### [`RFC9370-2.2.1-8`](#rfc9370-2.2.1-8)

If the responder selects NONE for some ADDKE Transform Types (provided they are proposed by the initiator), then any corresponding additional key exchanges MUST NOT take place. (§2.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9370-2.2.1-8, so no unit is bound to it.

### [`RFC9370-2.2.1-9`](#rfc9370-2.2.1-9)

The responder MUST treat all of the omitted ADDKE transforms as if they were proposed with Transform ID NONE. (§2.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9370-2.2.1-9, so no unit is bound to it.

### [`RFC9370-2.2.1-10`](#rfc9370-2.2.1-10)

If the initiator includes any ADDKE Transform Types into the SA payload in the IKE_SA_INIT exchange request message, then it MUST also negotiate the use of the IKE_INTERMEDIATE exchange, as described in [RFC9242] by including an INTERMEDIATE_EXCHANGE_SUPPORTED notification in the same message. (§2.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9370-2.2.1-10, so no unit is bound to it.

### [`RFC9370-2.2.1-11`](#rfc9370-2.2.1-11)

If the responder agrees to use additional key exchanges while establishing an initial IKE SA, it MUST also return this notification in the IKE_SA_INIT response message, confirming that IKE_INTERMEDIATE exchange is supported and will be used for transferring additional key exchange data. (§2.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9370-2.2.1-11, so no unit is bound to it.

### [`RFC9370-2.2.1-12`](#rfc9370-2.2.1-12)

If the IKE_INTERMEDIATE exchange is not negotiated, then the peers MUST treat any ADDKE Transform Types in the IKE_SA_INIT exchange messages as unknown transform types and skip the proposals they appear in. (§2.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9370-2.2.1-12, so no unit is bound to it.

### [`RFC9370-2.2.2-1`](#rfc9370-2.2.2-1)

The former "Diffie-Hellman Group Num" (now called "Key Exchange Method") field in the KEi(n) and KEr(n) payloads MUST match the n-th negotiated additional key exchange. (§2.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9370-2.2.2-1, so no unit is bound to it.

### [`RFC9370-2.2.4-1`](#rfc9370-2.2.4-1)

Each additional key exchange method MUST be fully completed before the next one is started. (§2.2.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9370-2.2.4-1, so no unit is bound to it.

### [`RFC9370-2.2.4-2`](#rfc9370-2.2.4-2)

The responder MUST include this notification in a CREATE_CHILD_SA or IKE_FOLLOWUP_KE response message in case the next IKE_FOLLOWUP_KE exchange is expected, filling it with some data that would allow linking the current exchange to the next one. (§2.2.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9370-2.2.4-2, so no unit is bound to it.

### [`RFC9370-2.2.4-3`](#rfc9370-2.2.4-3)

The initiator MUST send back this notification intact in the request message of the next IKE_FOLLOWUP_KE exchange. (§2.2.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9370-2.2.4-3, so no unit is bound to it.

### [`RFC9370-2.2.4-4`](#rfc9370-2.2.4-4)

The former "Diffie-Hellman Group Num" (now called "Key Exchange Method") field in the KEi(n) and KEr(n) payloads MUST match the n-th negotiated additional key exchange. (§2.2.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9370-2.2.4-4, so no unit is bound to it.

### [`RFC9370-2.2.4-5`](#rfc9370-2.2.4-5)

The responder MUST handle this situation gracefully and delete the associated state if it does not receive the next expected IKE_FOLLOWUP_KE request after some reasonable period of time. (§2.2.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9370-2.2.4-5, so no unit is bound to it.

### [`RFC9370-2.2.4-6`](#rfc9370-2.2.4-6)

If the responder receives an IKE_FOLLOWUP_KE message for which it does not have a key exchange state, it MUST send back a new error type notification called "STATE_NOT_FOUND". (§2.2.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9370-2.2.4-6, so no unit is bound to it.

### [`RFC9370-2.2.4-7`](#rfc9370-2.2.4-7)

If the initiator receives this notification in response to an IKE_FOLLOWUP_KE exchange performing an additional key exchange, it MUST cancel this exchange and MUST treat the whole series of exchanges started from the CREATE_CHILD_SA exchange as having failed. (§2.2.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9370-2.2.4-7, so no unit is bound to it.

### [`RFC9370-2.2.4-8`](#rfc9370-2.2.4-8)

If the initiator continues to receive STATE_NOT_FOUND notifications after several retries, it MUST treat this situation as a fatal error and delete the IKE SA by sending a DELETE payload. (§2.2.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9370-2.2.4-8, so no unit is bound to it.

### [`RFC9370-2.2.4-9`](#rfc9370-2.2.4-9)

For this reason, if additional key exchanges are negotiated in the CREATE_CHILD_SA exchange in which the smallest nonce is used, then, because there is nothing to delete yet, the initiator of this exchange just stops the rekeying process, and it MUST NOT initiate the IKE_FOLLOWUP_KE exchange. (§2.2.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9370-2.2.4-9, so no unit is bound to it.

### [`RFC9370-2.2.4-10`](#rfc9370-2.2.4-10)

In this case, a TEMPORARY_FAILURE notification MUST be sent in response to such a request. (§2.2.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9370-2.2.4-10, so no unit is bound to it.

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-10-26 |
| Register | rfc2119 |
| Source | rfc/full/rfc9370.txt |
| Source fingerprint | 81af94e0d6bb79b6 |
| Record | rfc/extraction/rfc9370.json |
| Mapped sentences | 23 |
| Declined as scope | 1 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | walked | not stated |
| `1` | not stated | 0 | walked | not stated |
| `1.1` | not stated | 0 | walked | not stated |
| `1.2` | not stated | 0 | walked | not stated |
| `1.3` | not stated | 0 | walked | not stated |
| `2` | not stated | 0 | walked | not stated |
| `2.1` | not stated | 0 | walked | not stated |
| `2.2` | not stated | 0 | walked | not stated |
| `2.2.1` | not stated | 12 | walked | not stated |
| `2.2.2` | not stated | 1 | walked | not stated |
| `2.2.3` | not stated | 0 | walked | not stated |
| `2.2.4` | not stated | 10 | walked | not stated |
| `2.2.5` | not stated | 0 | walked | not stated |
| `2.2.5.1` | not stated | 0 | walked | not stated |
| `3` | not stated | 0 | walked | not stated |
| `4` | not stated | 0 | walked | not stated |
| `5` | not stated | 0 | walked | not stated |
| `5.1` | not stated | 0 | walked | not stated |
| `5.2` | not stated | 0 | walked | not stated |
| `A` | not stated | 0 | walked | not stated |
| `A.1` | not stated | 0 | walked | not stated |
| `A.2` | not stated | 0 | walked | not stated |
| `A.3` | not stated | 0 | walked | not stated |
| `A.4` | not stated | 0 | walked | not stated |
| `B` | not stated | 0 | walked | not stated |
| `C` | not stated | 1 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `C:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Appendix C lists alternative designs the authors considered and discarded. This sentence closes the bullet 'Sending post-quantum proposals and policies in the KE payload only', and states what that discarded design would have needed to resist the downgrade attack it describes. The adopted design negotiates additional exchanges through ADDKE Transform Types in the SA payload (Section 2.2.1), so no implementer of this document is bound by the sentence. | Therefore, in order to fully prevent any downgrade attacks, using a KE payload alone is not sufficient, and the initiator MUST always indicate its preferred post-quantum proposals and policies in a notify payload in the subsequent IKE_SA_INIT messages following an N(INVALID_KE_PAYLOAD) response. |

## Superseded

No document obsoletes RFC 9370, so its obligations are stated where they were written.
