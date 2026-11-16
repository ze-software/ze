# RFC 9370 - Multiple Key Exchanges in the Internet Key Exchange Protocol Version 2 (IKEv2)

Multiple key exchanges (hybrid post-quantum). Every requirement listed for RFC 9370, the tests linked to it, and their recorded reviews. This summary is excluded from automatic compliance checks.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0 | of 23 mandatory requirements | a positive test checks that Ze does what the requirement demands and a negative one checks that it refuses what the requirement forbids |
| One polarity plus reason | 0 | of 23 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason supplies the complete test coverage required |
| One polarity, unexcused | 0 | of 23 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial test coverage | 0 | of 23 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unverified, with zero whole-requirement credit |
| Tests shown to detect a fault | 0 | 0 of 0 tests linked to requirements | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests verifies a requirement only after both tests have been shown to detect a fault |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Declared gap or no test | 23 | of 23 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 23 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 0 | of 23 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 0 | of 23 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze verified: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 23 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 23 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no test verification credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level), not checked | 23 | of 27 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements listed in this summary. This RFC is excluded from automatic checks (out-of-scope), so these requirements are not checked. The counts above show what the summary records. |

The 9 counts marked as a part above are the whole of the 23 mandatory requirements: they add up to 23. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

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
| Mandatory requirements (MUST level), not checked | outlined | outlined, with no colored edge: it is the total the other cards are counted against, not a measure, and a larger one is neither good news nor bad |

## At a glance

| Field | Value |
|---|---|
| Public row | Multiple key exchanges (hybrid post-quantum) |
| Automatic checks | Excluded (out-of-scope) |
| Requirements | 27 |
| Mandatory requirements (MUST level), not checked | 23 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 23 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 0 |
| Tests linked to requirements | 0 |
| Recorded audit verdicts | 0 |
| Fault test records | 0 |
| Summary | `rfc/short/rfc9370.md` |
| Requirement table | `rfc/requirements/rfc9370.md` |
| RFC text | `rfc/full/rfc9370.txt` |

## Automatic checks

Excluded from automatic checks (out-of-scope, the requirements ARE extracted and the owner decided not to offer the feature for now, so the absence is a scope decision rather than a conformance gap): OUT OF SCOPE as a document by owner decision, 2026-10-26: the owner rule committed in 1e7a9dbc34 (ai/META-DECISIONS.md, Goal and scope) counts an RFC a profile lists as optional only once Thomas brings it into scope, and this summary's `Optional use cases` row lists this RFC in the optional tier of the vpn-endpoint and vpn-concentrator profiles. The extraction is COMPLETE: the source text is at rfc/full/rfc9370.txt and the extraction sign-off is rfc/extraction/rfc9370.json. Ze implements none of it, and each MUST-level row keeps its `{gap}` note naming the absent producer: all 23 MUST-level requirements are classified, and all 23 are `{gap}`, because Ze's IKE engine implements neither the IKE_INTERMEDIATE exchange (RFC 9242) nor IKE_FOLLOWUP_KE, nor the ADDKE Transform Types 6 to 12. `internal/component/ike/wire/header.go` declares exchange types 34 to 37 only, and `internal/component/ike/wire/payload_notify.go` declares neither ADDITIONAL_KEY_EXCHANGE (16441) nor STATE_NOT_FOUND (47). One Appendix C site states the rationale for a discarded alternative design and is excluded in the extraction sign-off.

## Published support status

**Area:** Multiple key exchanges (hybrid post-quantum)

**Test verification:** 0 of 23 MUST proven

**Evidence:** -

**Unmet:** RFC9370-2.2.1-1, RFC9370-2.2.1-10, RFC9370-2.2.1-11, RFC9370-2.2.1-12, RFC9370-2.2.1-2, RFC9370-2.2.1-3, RFC9370-2.2.1-4, RFC9370-2.2.1-5, RFC9370-2.2.1-6, RFC9370-2.2.1-7, RFC9370-2.2.1-8, RFC9370-2.2.1-9, RFC9370-2.2.2-1, RFC9370-2.2.4-1, RFC9370-2.2.4-10, RFC9370-2.2.4-2, RFC9370-2.2.4-3, RFC9370-2.2.4-4, RFC9370-2.2.4-5, RFC9370-2.2.4-6, RFC9370-2.2.4-7, RFC9370-2.2.4-8, RFC9370-2.2.4-9


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 0 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 23 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 0 | one part of the mandatory requirements |
| Partial test coverage (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level), not checked** | **23** | every mandatory requirement appears in exactly one group above |

**Annotated (including scoped evidence) (23):** [`RFC9370-2.2.1-1`](#rfc9370-2.2.1-1), [`RFC9370-2.2.1-2`](#rfc9370-2.2.1-2), [`RFC9370-2.2.1-3`](#rfc9370-2.2.1-3), [`RFC9370-2.2.1-4`](#rfc9370-2.2.1-4), [`RFC9370-2.2.1-5`](#rfc9370-2.2.1-5), [`RFC9370-2.2.1-6`](#rfc9370-2.2.1-6), [`RFC9370-2.2.1-7`](#rfc9370-2.2.1-7), [`RFC9370-2.2.1-8`](#rfc9370-2.2.1-8), [`RFC9370-2.2.1-9`](#rfc9370-2.2.1-9), [`RFC9370-2.2.1-10`](#rfc9370-2.2.1-10), [`RFC9370-2.2.1-11`](#rfc9370-2.2.1-11), [`RFC9370-2.2.1-12`](#rfc9370-2.2.1-12), [`RFC9370-2.2.2-1`](#rfc9370-2.2.2-1), [`RFC9370-2.2.4-1`](#rfc9370-2.2.4-1), [`RFC9370-2.2.4-2`](#rfc9370-2.2.4-2), [`RFC9370-2.2.4-3`](#rfc9370-2.2.4-3), [`RFC9370-2.2.4-4`](#rfc9370-2.2.4-4), [`RFC9370-2.2.4-5`](#rfc9370-2.2.4-5), [`RFC9370-2.2.4-6`](#rfc9370-2.2.4-6), [`RFC9370-2.2.4-7`](#rfc9370-2.2.4-7), [`RFC9370-2.2.4-8`](#rfc9370-2.2.4-8), [`RFC9370-2.2.4-9`](#rfc9370-2.2.4-9), [`RFC9370-2.2.4-10`](#rfc9370-2.2.4-10)

## Figures

- **MUST bar**: Not built 23
- **Overview bar**: Not assessed 4, Not built 23

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | not complete |
| MUST rows | 23 |
| MUST rows kept (the MUST bar) | 23 |
| Ze verified | 0 |
| OS verified | 0 |
| Tests awaiting verification | 0 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 23 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 4 |
| SHOULD verified | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD tests awaiting verification | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 4 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze verified means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Tests awaiting verification counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

Source: [data/rfc/rfc9370.json](../../../data/rfc/rfc9370.json).

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

## Test status

A test reads not verified when no record shows it failing after a deliberate fault. Until then, the test has not been shown to detect a fault in the required behavior.

### [`RFC9370-2.2.1-1`](#rfc9370-2.2.1-1)

Additional key exchanges negotiated via newly defined transforms MUST take place in a series of IKE_INTERMEDIATE exchanges following the IKE_SA_INIT exchange, performed in an order of the values of their Transform Types. (§2.2.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC9370-2.2.1-1, so there is no linked test.

### [`RFC9370-2.2.1-2`](#rfc9370-2.2.1-2)

Each additional key exchange method MUST be fully completed before the next one is started. (§2.2.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC9370-2.2.1-2, so there is no linked test.

### [`RFC9370-2.2.1-3`](#rfc9370-2.2.1-3)

On the other hand, if the initiator includes any ADDKE Transform Type in the proposal, the responder MUST select one of the algorithms proposed using this type. (§2.2.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC9370-2.2.1-3, so there is no linked test.

### [`RFC9370-2.2.1-4`](#rfc9370-2.2.1-4)

However, for the ADDKE Transform Types, the responder's choice MUST NOT contain duplicated algorithms (those with an identical Transform ID and attributes), except for the Transform ID of NONE. (§2.2.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC9370-2.2.1-4, so there is no linked test.

### [`RFC9370-2.2.1-5`](#rfc9370-2.2.1-5)

If the responder is unable to select algorithms that are not duplicated for each proposed key exchange (either because the proposal contains too few choices or due to the local policy restrictions on using the proposed algorithms), then the responder MUST reject the message with an error notification of type NO_PROPOSAL_CHOSEN. (§2.2.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC9370-2.2.1-5, so there is no linked test.

### [`RFC9370-2.2.1-6`](#rfc9370-2.2.1-6)

If the responder's message contains one or more duplicated choices, the initiator should log the error and MUST treat the exchange as failed. (§2.2.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC9370-2.2.1-6, so there is no linked test.

### [`RFC9370-2.2.1-7`](#rfc9370-2.2.1-7)

The initiator MUST NOT initiate any IKE_INTERMEDIATE (or IKE_FOLLOWUP_KE) exchanges so that no new SA is created. (§2.2.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC9370-2.2.1-7, so there is no linked test.

### [`RFC9370-2.2.1-8`](#rfc9370-2.2.1-8)

If the responder selects NONE for some ADDKE Transform Types (provided they are proposed by the initiator), then any corresponding additional key exchanges MUST NOT take place. (§2.2.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC9370-2.2.1-8, so there is no linked test.

### [`RFC9370-2.2.1-9`](#rfc9370-2.2.1-9)

The responder MUST treat all of the omitted ADDKE transforms as if they were proposed with Transform ID NONE. (§2.2.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC9370-2.2.1-9, so there is no linked test.

### [`RFC9370-2.2.1-10`](#rfc9370-2.2.1-10)

If the initiator includes any ADDKE Transform Types into the SA payload in the IKE_SA_INIT exchange request message, then it MUST also negotiate the use of the IKE_INTERMEDIATE exchange, as described in [RFC9242] by including an INTERMEDIATE_EXCHANGE_SUPPORTED notification in the same message. (§2.2.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC9370-2.2.1-10, so there is no linked test.

### [`RFC9370-2.2.1-11`](#rfc9370-2.2.1-11)

If the responder agrees to use additional key exchanges while establishing an initial IKE SA, it MUST also return this notification in the IKE_SA_INIT response message, confirming that IKE_INTERMEDIATE exchange is supported and will be used for transferring additional key exchange data. (§2.2.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC9370-2.2.1-11, so there is no linked test.

### [`RFC9370-2.2.1-12`](#rfc9370-2.2.1-12)

If the IKE_INTERMEDIATE exchange is not negotiated, then the peers MUST treat any ADDKE Transform Types in the IKE_SA_INIT exchange messages as unknown transform types and skip the proposals they appear in. (§2.2.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC9370-2.2.1-12, so there is no linked test.

### [`RFC9370-2.2.2-1`](#rfc9370-2.2.2-1)

The former "Diffie-Hellman Group Num" (now called "Key Exchange Method") field in the KEi(n) and KEr(n) payloads MUST match the n-th negotiated additional key exchange. (§2.2.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC9370-2.2.2-1, so there is no linked test.

### [`RFC9370-2.2.4-1`](#rfc9370-2.2.4-1)

Each additional key exchange method MUST be fully completed before the next one is started. (§2.2.4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC9370-2.2.4-1, so there is no linked test.

### [`RFC9370-2.2.4-2`](#rfc9370-2.2.4-2)

The responder MUST include this notification in a CREATE_CHILD_SA or IKE_FOLLOWUP_KE response message in case the next IKE_FOLLOWUP_KE exchange is expected, filling it with some data that would allow linking the current exchange to the next one. (§2.2.4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC9370-2.2.4-2, so there is no linked test.

### [`RFC9370-2.2.4-3`](#rfc9370-2.2.4-3)

The initiator MUST send back this notification intact in the request message of the next IKE_FOLLOWUP_KE exchange. (§2.2.4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC9370-2.2.4-3, so there is no linked test.

### [`RFC9370-2.2.4-4`](#rfc9370-2.2.4-4)

The former "Diffie-Hellman Group Num" (now called "Key Exchange Method") field in the KEi(n) and KEr(n) payloads MUST match the n-th negotiated additional key exchange. (§2.2.4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC9370-2.2.4-4, so there is no linked test.

### [`RFC9370-2.2.4-5`](#rfc9370-2.2.4-5)

The responder MUST handle this situation gracefully and delete the associated state if it does not receive the next expected IKE_FOLLOWUP_KE request after some reasonable period of time. (§2.2.4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC9370-2.2.4-5, so there is no linked test.

### [`RFC9370-2.2.4-6`](#rfc9370-2.2.4-6)

If the responder receives an IKE_FOLLOWUP_KE message for which it does not have a key exchange state, it MUST send back a new error type notification called "STATE_NOT_FOUND". (§2.2.4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC9370-2.2.4-6, so there is no linked test.

### [`RFC9370-2.2.4-7`](#rfc9370-2.2.4-7)

If the initiator receives this notification in response to an IKE_FOLLOWUP_KE exchange performing an additional key exchange, it MUST cancel this exchange and MUST treat the whole series of exchanges started from the CREATE_CHILD_SA exchange as having failed. (§2.2.4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC9370-2.2.4-7, so there is no linked test.

### [`RFC9370-2.2.4-8`](#rfc9370-2.2.4-8)

If the initiator continues to receive STATE_NOT_FOUND notifications after several retries, it MUST treat this situation as a fatal error and delete the IKE SA by sending a DELETE payload. (§2.2.4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC9370-2.2.4-8, so there is no linked test.

### [`RFC9370-2.2.4-9`](#rfc9370-2.2.4-9)

For this reason, if additional key exchanges are negotiated in the CREATE_CHILD_SA exchange in which the smallest nonce is used, then, because there is nothing to delete yet, the initiator of this exchange just stops the rekeying process, and it MUST NOT initiate the IKE_FOLLOWUP_KE exchange. (§2.2.4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC9370-2.2.4-9, so there is no linked test.

### [`RFC9370-2.2.4-10`](#rfc9370-2.2.4-10)

In this case, a TEMPORARY_FAILURE notification MUST be sent in response to such a request. (§2.2.4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC9370-2.2.4-10, so there is no linked test.

## Requirement list review

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
| `front` | not stated | 0 | reviewed | not stated |
| `1` | not stated | 0 | reviewed | not stated |
| `1.1` | not stated | 0 | reviewed | not stated |
| `1.2` | not stated | 0 | reviewed | not stated |
| `1.3` | not stated | 0 | reviewed | not stated |
| `2` | not stated | 0 | reviewed | not stated |
| `2.1` | not stated | 0 | reviewed | not stated |
| `2.2` | not stated | 0 | reviewed | not stated |
| `2.2.1` | not stated | 12 | reviewed | not stated |
| `2.2.2` | not stated | 1 | reviewed | not stated |
| `2.2.3` | not stated | 0 | reviewed | not stated |
| `2.2.4` | not stated | 10 | reviewed | not stated |
| `2.2.5` | not stated | 0 | reviewed | not stated |
| `2.2.5.1` | not stated | 0 | reviewed | not stated |
| `3` | not stated | 0 | reviewed | not stated |
| `4` | not stated | 0 | reviewed | not stated |
| `5` | not stated | 0 | reviewed | not stated |
| `5.1` | not stated | 0 | reviewed | not stated |
| `5.2` | not stated | 0 | reviewed | not stated |
| `A` | not stated | 0 | reviewed | not stated |
| `A.1` | not stated | 0 | reviewed | not stated |
| `A.2` | not stated | 0 | reviewed | not stated |
| `A.3` | not stated | 0 | reviewed | not stated |
| `A.4` | not stated | 0 | reviewed | not stated |
| `B` | not stated | 0 | reviewed | not stated |
| `C` | not stated | 1 | reviewed | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `C:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Appendix C lists alternative designs the authors considered and discarded. This sentence closes the bullet 'Sending post-quantum proposals and policies in the KE payload only', and states what that discarded design would have needed to resist the downgrade attack it describes. The adopted design negotiates additional exchanges through ADDKE Transform Types in the SA payload (Section 2.2.1), so no implementer of this document is bound by the sentence. | Therefore, in order to fully prevent any downgrade attacks, using a KE payload alone is not sufficient, and the initiator MUST always indicate its preferred post-quantum proposals and policies in a notify payload in the subsequent IKE_SA_INIT messages following an N(INVALID_KE_PAYLOAD) response. |

## Superseded

No document obsoletes RFC 9370, so its obligations are stated where they were written.
