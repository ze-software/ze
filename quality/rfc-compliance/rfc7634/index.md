# RFC 7634 - ChaCha20, Poly1305, and Their Use in the Internet Key Exchange Protocol (IKE) and IPsec

ESP. Every requirement listed for RFC 7634, the tests linked to it, and their recorded reviews. This summary is included in the automatic compliance check, ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 1 | of 2 mandatory requirements | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0 | of 2 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0 | of 2 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0 | of 2 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Declared gap or no test | 0 | of 2 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |
| Tests shown to detect a fault | 2 | 2 of 2 tests linked to requirements | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests counts as proof only after this check has demonstrated that they can detect a fault |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 2 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 0 | of 2 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 1 | of 2 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze proven: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 2 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 2 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no proof credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level) | 2 | of 5 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements included in the automatic checks: the compliance check (./le rfc check) holds Ze to each of them. This is the total being assessed; the counts above show the recorded status of those requirements. |

The 9 counts marked as a part above are the whole of the 2 mandatory requirements: they add up to 2. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got. An outlined card with no colored edge, under Totals, is a total the other cards are counted against, not a measure.

| Card | Tone here | Why that color |
|---|---|---|
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the count under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| Partial proof; remaining gap | ok | green at zero, RED above it: a tested clause cannot prove the whole requirement |
| Declared gap or no test | ok | green at zero, RED above it: a binding obligation that nothing shows Ze meets is owed, whether or not a reason is stated |
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
| Public row | ESP |
| Automatic checks | Included |
| Requirements | 5 |
| Mandatory requirements (MUST level) | 2 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 2 |
| Tests linked to requirements | 2 |
| Recorded audit verdicts | 0 |
| Fault test records | 2 |
| Summary | `rfc/short/rfc7634.md` |
| Requirement table | `rfc/requirements/rfc7634.md` |
| RFC text | `rfc/full/rfc7634.txt` |

## Automatic checks

Included in automatic checks: ChaCha20-Poly1305 for ESP and IKEv2: two MUST-level rows, both classified. IV uniqueness (RFC7634-2-1) is performed by the ESP datapath on the association ze installs and carries {lower-layer}. The AEAD proposal shape (RFC7634-4-1) is ze's own proposal code and owes a tagged test. The Section 3 padding rule binds the IKE SA Encrypted payload, and ze offers ChaCha20-Poly1305 for ESP only, which RFC 8247 grades SHOULD for IKEv2 (feature-out-of-scope in the extraction).

## Published support status

**Area:** ESP

**Proof:** 1 of 2 MUST proven

**Evidence:** unit 1

**Unmet:** -


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 1 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 1 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 0 | one part of the mandatory requirements |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level)** | **2** | every mandatory requirement appears in exactly one group above |

**Positive and negative tests (1):** [`RFC7634-4-1`](#rfc7634-4-1)

**Annotated (including scoped evidence) (1):** [`RFC7634-2-1`](#rfc7634-2-1)

## Figures

- **MUST bar**: Ze proven 1, OS testing gap 1
- **Overview bar**: Ze MUST proven 1, OS testing gap 1, Not assessed 3

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | not complete |
| MUST rows | 2 |
| MUST rows kept (the MUST bar) | 2 |
| Ze proven | 1 |
| OS verified | 0 |
| Proof pending | 0 |
| OS testing gap | 1 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 0 |
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
| SHOULD proof pending | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 3 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze proven means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Proof pending counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

Source: [data/rfc/rfc7634.json](../../../data/rfc/rfc7634.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC7634-2-1` | The IV MUST be unique for each invocation for a particular security association (SA) but does not need to be unpredictable. (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux XFRM rfc7539esp(chacha20,poly1305) and its seqiv IV generator, linux:CONFIG_CRYPTO_CHACHA20POLY1305; internal/component/ike/dataplane/xfrm_linux.go::xfrmStateFromParams installs only the key and salt, and the kernel chooses every IV; the generator derives each IV from the SA's sequence number, and the value Ze installs that keeps it unique is OSeqMayWrap=false, written on every state by the `state.OSeqMayWrap = false` guard in xfrmStateFromParams. Owed tag on that value: internal/component/ike/dataplane/rfc4301_sad_items_linux_test.go (TestRFC4301SADItemSequenceCounterOverflowIsNeverRollover) and rfc4106_gcm_install_linux_test.go (TestRFC4106GCMKeyCannotOutliveTheSequenceSpace) |
| `RFC7634-4-1` | As with other AEAD algorithms, INTEG (type 3) transform substructures MUST NOT be specified, or just one INTEG transform MAY be included with value NONE (0). (§4) | MUST NOT | 4 | **positive:** `unit/verify` [`TestRFC7634ChaChaProposalCarriesNoInteg`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/ike_auth_algorithms_test.go#L187). **negative:** `unit/verify` [`TestRFC7634ChaChaProposalCarriesNoInteg`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/ike_auth_algorithms_test.go#L216) |
| `RFC7634-2-2` | The use of a counter or a linear feedback shift register (LFSR) is RECOMMENDED. (§2) | RECOMMENDED | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7634-3-1` | The sender SHOULD include no padding and set the Pad Length field to zero. (§3) | SHOULD | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7634-5-1` | Thus, implementations that keep keys and other secret material within some security boundary MAY export the Salt from the security boundary. (§5) | MAY | 5 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC7634-2-1`](#rfc7634-2-1) The IV MUST be unique for each invocation for a particular security association (SA) but does not need to be unpredictable. (§2) | no test | no test carries this requirement id; annotated {lower-layer}: Linux XFRM rfc7539esp(chacha20,poly1305) and its seqiv IV generator, linux:CONFIG_CRYPTO_CHACHA20POLY1305; internal/component/ike/dataplane/xfrm_linux.go::xfrmStateFromParams installs only the key and salt, and the kernel chooses every IV; the generator derives each IV from the SA's sequence number, and the value Ze installs that keeps it unique is OSeqMayWrap=false, written on every state by the `state.OSeqMayWrap = false` guard in xfrmStateFromParams. Owed tag on that value: internal/component/ike/dataplane/rfc4301_sad_items_linux_test.go (TestRFC4301SADItemSequenceCounterOverflowIsNeverRollover) and rfc4106_gcm_install_linux_test.go (TestRFC4106GCMKeyCannotOutliveTheSequenceSpace) |

## Proof state

A test reads unproven when no record shows it failing after a deliberate fault. Until then, its claim to check the requirement remains unproven.

### [`RFC7634-2-1`](#rfc7634-2-1)

The IV MUST be unique for each invocation for a particular security association (SA) but does not need to be unpredictable. (§2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC7634-2-1, so there is no linked test.

### [`RFC7634-4-1`](#rfc7634-4-1)

As with other AEAD algorithms, INTEG (type 3) transform substructures MUST NOT be specified, or just one INTEG transform MAY be included with value NONE (0). (§4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7634ChaChaProposalCarriesNoInteg`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/ike_auth_algorithms_test.go#L216) | unit/verify | revert, verified |
| positive | [`TestRFC7634ChaChaProposalCarriesNoInteg`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/ike_auth_algorithms_test.go#L187) | unit/verify | revert, verified |

## Requirement list review

| Field | Value |
|---|---|
| Reviewer | claude (ze-rfc phase-1 agent, session 45f03892) |
| Signed off | 2026-10-26 |
| Register | rfc2119 |
| Source | rfc/full/rfc7634.txt |
| Source fingerprint | 8ca1bba99b0321c4 |
| Record | rfc/extraction/rfc7634.json |
| Mapped sentences | 2 |
| Declined as scope | 1 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | title, status and table of contents | 0 | skipped (front-matter) | title, status and table of contents |
| `1` | not stated | 0 | reviewed | not stated |
| `1.1` | not stated | 0 | reviewed | not stated |
| `2` | not stated | 1 | reviewed | not stated |
| `2.1` | not stated | 0 | reviewed | not stated |
| `3` | not stated | 1 | reviewed | not stated |
| `4` | not stated | 1 | reviewed | not stated |
| `5` | not stated | 0 | reviewed | not stated |
| `6` | IANA considerations | 0 | skipped (iana) | IANA considerations |
| `7` | references | 0 | skipped (references) | references |
| `7.1` | normative references | 0 | skipped (references) | normative references |
| `7.2` | informative references | 0 | skipped (references) | informative references |
| `A` | ESP example packet | 0 | skipped (appendix-non-normative) | ESP example packet |
| `B` | IKEv2 example exchange | 0 | skipped (appendix-non-normative) | IKEv2 example exchange |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `3:1` | `feature-out-of-scope` (never bound Ze): the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it | RFC 8247 Section 2.1 grades the cipher SHOULD for IKEv2: "there were not enough IKEv2 implementations supporting ENCR_CHACHA20_POLY1305 to be able to introduce it at the SHOULD+ level". The receiver padding rule binds the IKE SA Encrypted payload, and ze admits ChaCha20-Poly1305 for ESP only (internal/component/ike/crypto/transform.go espOnlyEncryptionRegistry; aeadTransforms gives it no IKE mode). The absent IKE SA use is disclosed on docs/features/rfc-status.md through the Support remaining row of rfc/short/rfc7634.md | The receiver MUST accept any length of padding. |

## Superseded

No document obsoletes RFC 7634, so its obligations are stated where they were written.
