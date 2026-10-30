# RFC 4754 - IKE and IKEv2 Authentication Using the Elliptic Curve Digital Signature Algorithm (ECDSA)

Partial. Every requirement this repository extracted from RFC 4754, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 100.0% | 1 of 1 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 1 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 1 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 1 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| No test at all | 0.0% | 0 of 1 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 100.0% | 2 of 2 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 1 | of 1 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 1 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 1 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 1 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 1 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |
| Linux cannot do this | 0.0% | 0 of 1 gated MUSTs | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and remains unmet; it stays in every denominator and earns no proof credit |

The 9 shares marked as a part above are the whole of the 1 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Linux cannot do this | neutral | neutral at every value: an evidenced platform limit is neither a proof nor a scope exemption |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Partial |
| Enrolment | Enrolled |
| Requirements | 1 |
| Gated MUST-level | 1 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 2 |
| Tagged units | 2 |
| Recorded audit verdicts | 0 |
| Discrimination records | 2 |
| Summary | `rfc/short/rfc4754.md` |
| Requirement shard | `rfc/requirements/rfc4754.md` |
| RFC text | `rfc/full/rfc4754.txt` |

## Enrolment

Enrolled: ENROLLED by owner decision, 2026-10-30: Thomas brought RFC 4754 into scope, so it moved from the optional to the required tier of the vpn-endpoint and vpn-concentrator profiles. The extraction sign-off is rfc/extraction/rfc4754.json over rfc/full/rfc4754.txt. The single MUST-level requirement, RFC4754-7-1 (the Section 7 r-and-s signature encoding), is met on the receive side by `internal/component/ike/engine/auth_ecdsa.go::verifyECDSAAuth`, which `verifyRemoteAuth` reaches for AUTH methods 9, 10 and 11, and is proven by TestRFC4754SignatureIsRConcatS in both polarities (rfc/discrimination/rfc4754/RFC4754-7-1.json). Ze never signs with these methods; whether it should is not decided.

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

Ze verifies a peer's AUTH of method 9, 10 or 11: the Authentication Data must be r followed by s, each zero-padded to 32, 48 or 66 octets, the peer's certificate key must be on the method's curve, and the signature is checked with the method's hash (`internal/component/ike/engine/auth_ecdsa.go::verifyECDSAAuth`, reached from `auth.go::verifyRemoteAuth` on every received IKE_AUTH). Ze signs with ECDSA keys only through the RFC 7427 Digital Signature method (14), with a DER-encoded signature and an AlgorithmIdentifier (`auth.go::selectSignatureAlgorithm`, `signDigest`).

**What the ledger says remains**

Ze never signs with ECDSA-256 (9), ECDSA-384 (10) or ECDSA-521 (11). With no SIGNATURE_HASH_ALGORITHMS notify from the peer, `computeX509Auth` refuses rather than falling back to one of them, so an ECDSA-certificate tunnel to a peer without RFC 7427 support completes only where Ze does not sign (an EAP initiator verifying the responder). Signing with these methods is not decided.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 1 | one part of the gated population |
| Annotated (including scoped evidence) | 0 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **1** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (1):** [`RFC4754-7-1`](#rfc4754-7-1)

## Attainable ceiling

Mandatory ceiling: 1 of 1 (100.0%). Optional ceiling: no rows. Every RFC shown with measured rows can reach 100% under current classifications. Lowest mandatory ceiling: rfc4754 1 of 1 (100.0%). Ceilings describe current classifications, not an immutable Linux maximum. Not-applicable and feature-declined classifications can be revisited. Every gap remains work Ze owes.

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC4754-7-1` | When ECDSA-256, ECDSA-384, or ECDSA-521 is used as the digital signature in IKE or IKEv2, the signature payload SHALL contain an encoding of the computed signature consisting of the concatenation of a pair of integers r and s. (§7) | SHALL | 7 | **positive:** `unit/verify` [`TestRFC4754SignatureIsRConcatS`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4754_auth_test.go#L83). **negative:** `unit/verify` [`TestRFC4754SignatureIsRConcatS`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4754_auth_test.go#L84) |

## Gaps and untested MUSTs

RFC 4754 declares no gap, and every gated MUST it carries has a test bound to it.

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC4754-7-1`](#rfc4754-7-1)

When ECDSA-256, ECDSA-384, or ECDSA-521 is used as the digital signature in IKE or IKEv2, the signature payload SHALL contain an encoding of the computed signature consisting of the concatenation of a pair of integers r and s. (§7)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4754SignatureIsRConcatS`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4754_auth_test.go#L84) | unit/verify | mutant, verified |
| positive | [`TestRFC4754SignatureIsRConcatS`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4754_auth_test.go#L83) | unit/verify | mutant, verified |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-10-26 |
| Register | rfc2119 |
| Source | rfc/full/rfc4754.txt |
| Source fingerprint | fc7087d50dadf19f |
| Record | rfc/extraction/rfc4754.json |
| Mapped sentences | 1 |
| Declined as scope | 0 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | walked | not stated |
| `1` | not stated | 0 | walked | not stated |
| `2` | not stated | 0 | walked | not stated |
| `3` | not stated | 0 | walked | not stated |
| `4` | not stated | 0 | walked | not stated |
| `5` | not stated | 0 | walked | not stated |
| `6` | not stated | 0 | walked | not stated |
| `7` | not stated | 1 | walked | not stated |
| `8` | not stated | 0 | walked | not stated |
| `8.1` | not stated | 0 | walked | not stated |
| `8.2` | not stated | 0 | walked | not stated |
| `8.3` | not stated | 0 | walked | not stated |
| `9` | not stated | 0 | walked | not stated |
| `9.1` | not stated | 0 | walked | not stated |
| `9.2` | not stated | 0 | walked | not stated |

### Excluded sentences

The walk over RFC 4754 declined no sentence: every site it found is mapped to a requirement.

## Superseded

No document obsoletes RFC 4754, so its obligations are stated where they were written.
