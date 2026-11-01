# RFC 7634 - ChaCha20, Poly1305, and Their Use in the Internet Key Exchange Protocol (IKE) and IPsec

ESP. Every requirement this repository extracted from RFC 7634, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 1 | of 2 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0 | of 2 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0 | of 2 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0 | of 2 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| No test at all | 0 | of 2 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 2 | 2 of 2 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 2 | of 5 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the counts beside it are what says how Ze stands |
| Out of scope | 0 | of 2 gated MUSTs | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as a Ze gap. Scope, not coverage: it is counted in the gated whole the cards on this page add up to |
| Not applicable | 0 | of 2 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as a Ze gap. It is counted in the gated whole the cards add up to |
| Met below Ze | 1 | of 2 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze proven: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 2 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as a Ze gap. It is counted in the gated whole the cards add up to |
| Linux cannot do this | 0 | of 2 gated MUSTs | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as a Ze gap. It is counted in the gated whole the cards add up to and earns no proof credit |

The 9 counts marked as a part above are the whole of the 2 gated MUSTs: they add up to 2. Proven by a recorded break counts TAGGED UNITS, a different population, so it is not one of them.

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
| Public row | ESP |
| Enrolment | Enrolled |
| Requirements | 5 |
| Gated MUST-level | 2 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 2 |
| Tagged units | 2 |
| Recorded audit verdicts | 0 |
| Discrimination records | 2 |
| Summary | `rfc/short/rfc7634.md` |
| Requirement shard | `rfc/requirements/rfc7634.md` |
| RFC text | `rfc/full/rfc7634.txt` |

## Enrolment

Enrolled: ChaCha20-Poly1305 for ESP and IKEv2: two MUST-level rows, both classified. IV uniqueness (RFC7634-2-1) is performed by the ESP datapath on the association ze installs and carries {lower-layer}. The AEAD proposal shape (RFC7634-4-1) is ze's own proposal code and owes a tagged test. The Section 3 padding rule binds the IKE SA Encrypted payload, and ze offers ChaCha20-Poly1305 for ESP only, which RFC 8247 grades SHOULD for IKEv2 (feature-out-of-scope in the extraction).

## What the public ledger says

**Area:** ESP

**Proof:** 1 of 2 MUST proven

**Evidence:** unit 1

**Unmet:** -


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 1 | one part of the gated population |
| Annotated (including scoped evidence) | 1 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **2** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (1):** [`RFC7634-4-1`](#rfc7634-4-1)

**Annotated (including scoped evidence) (1):** [`RFC7634-2-1`](#rfc7634-2-1)

## Figures

- **MUST bar**: Ze proven 1, OS testing gap 1
- **Overview bar**: Ze MUST proven 1, SHOULD not decided 3, OS testing gap 1

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Not walked | 0 |
| Mandatory register | not complete |
| MUST rows | 2 |
| MUST rows kept (the MUST bar) | 2 |
| Ze proven | 1 |
| OS verified | 0 |
| OS testing gap | 1 |
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

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC7634-2-1`](#rfc7634-2-1)

The IV MUST be unique for each invocation for a particular security association (SA) but does not need to be unpredictable. (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7634-2-1, so no unit is bound to it.

### [`RFC7634-4-1`](#rfc7634-4-1)

As with other AEAD algorithms, INTEG (type 3) transform substructures MUST NOT be specified, or just one INTEG transform MAY be included with value NONE (0). (§4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7634ChaChaProposalCarriesNoInteg`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/ike_auth_algorithms_test.go#L216) | unit/verify | revert, verified |
| positive | [`TestRFC7634ChaChaProposalCarriesNoInteg`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/ike_auth_algorithms_test.go#L187) | unit/verify | revert, verified |

## Extraction sign-off

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
| `1` | not stated | 0 | walked | not stated |
| `1.1` | not stated | 0 | walked | not stated |
| `2` | not stated | 1 | walked | not stated |
| `2.1` | not stated | 0 | walked | not stated |
| `3` | not stated | 1 | walked | not stated |
| `4` | not stated | 1 | walked | not stated |
| `5` | not stated | 0 | walked | not stated |
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
