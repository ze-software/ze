# RFC 7634 - ChaCha20, Poly1305, and Their Use in the Internet Key Exchange Protocol (IKE) and IPsec

Partial. Every requirement this repository extracted from RFC 7634, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 50.0% | 1 of 2 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 2 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 2 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 2 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| No test at all | 0.0% | 0 of 2 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 100.0% | 2 of 2 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 2 | of 5 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 2 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 2 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 50.0% | 1 of 2 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 2 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

The 8 shares marked as a part above are the whole of the 2 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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

**Status:** Partial

**What the ledger says is covered**

- ENCR_CHACHA20_POLY1305 for ESP Child SAs: 32-octet key plus 4-octet salt from KEYMAT, 8-octet IV, 16-octet tag
- per-packet processing by Linux XFRM. [`internal/component/ike/crypto/keys.go`](https://github.com/ze-software/ze/blob/main/internal/component/ike/crypto/keys.go)::DeriveChildSAKeys derives KEYMAT
- [`internal/le/interoplab/ipsec/vpnproof.go`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/ipsec/vpnproof.go)::checkESPKernel proves the negotiated cipher, installed rfc7539esp(chacha20,poly1305) transform and received traffic against strongSwan on native amd64/KVM and arm64/HVF. This kernel interop evidence adds no RFC-tagged requirement verdict.


**What the ledger says remains:**

ChaCha20-Poly1305 is not offered for the IKE SA (Section 3). Requirement test owed for [`RFC7634-4-1`](#rfc7634-4-1).

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

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC7634-2-1` | The IV MUST be unique for each invocation for a particular security association (SA) but does not need to be unpredictable. (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux XFRM rfc7539esp(chacha20,poly1305) and its IV generator; internal/component/ike/dataplane/xfrm_linux.go::xfrmStateFromParams installs only the key and salt, and the kernel chooses every IV |
| `RFC7634-4-1` | As with other AEAD algorithms, INTEG (type 3) transform substructures MUST NOT be specified, or just one INTEG transform MAY be included with value NONE (0). (§4) | MUST NOT | 4 | **positive:** `unit/verify` [`TestRFC7634ChaChaProposalCarriesNoInteg`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/ike_auth_algorithms_test.go#L187). **negative:** `unit/verify` [`TestRFC7634ChaChaProposalCarriesNoInteg`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/ike_auth_algorithms_test.go#L216) |
| `RFC7634-2-2` | The use of a counter or a linear feedback shift register (LFSR) is RECOMMENDED. (§2) | RECOMMENDED | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7634-3-1` | The sender SHOULD include no padding and set the Pad Length field to zero. (§3) | SHOULD | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7634-5-1` | Thus, implementations that keep keys and other secret material within some security boundary MAY export the Salt from the security boundary. (§5) | MAY | 5 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC7634-2-1`](#rfc7634-2-1) The IV MUST be unique for each invocation for a particular security association (SA) but does not need to be unpredictable. (§2) | no test | no test carries this requirement id; annotated {lower-layer}: Linux XFRM rfc7539esp(chacha20,poly1305) and its IV generator; internal/component/ike/dataplane/xfrm_linux.go::xfrmStateFromParams installs only the key and salt, and the kernel chooses every IV |

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
