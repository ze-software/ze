# RFC 4754 - IKE and IKEv2 Authentication Using the Elliptic Curve Digital Signature Algorithm (ECDSA)

ECDSA authentication methods 9, 10 and 11. Every requirement this repository extracted from RFC 4754, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 1 | of 1 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0 | of 1 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0 | of 1 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0 | of 1 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| No test at all | 0 | of 1 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 4 | 4 of 4 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |
| Audit verdicts | 1 | of 1 gated MUSTs judged | 0 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 1 | of 1 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the counts beside it are what says how Ze stands |
| Out of scope | 0 | of 1 gated MUSTs | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as a Ze gap. Scope, not coverage: it is counted in the gated whole the cards on this page add up to |
| Not applicable | 0 | of 1 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as a Ze gap. It is counted in the gated whole the cards add up to |
| Met below Ze | 0 | of 1 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze proven: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 1 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as a Ze gap. It is counted in the gated whole the cards add up to |
| Linux cannot do this | 0 | of 1 gated MUSTs | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as a Ze gap. It is counted in the gated whole the cards add up to and earns no proof credit |

The 9 counts marked as a part above are the whole of the 1 gated MUSTs: they add up to 1. Proven by a recorded break counts TAGGED UNITS, a different population, so it is not one of them.

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
| Audit verdicts | ok | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public row | ECDSA authentication methods 9, 10 and 11 |
| Enrolment | Enrolled |
| Requirements | 1 |
| Gated MUST-level | 1 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 4 |
| Tagged units | 4 |
| Recorded audit verdicts | 1 |
| Discrimination records | 4 |
| Summary | `rfc/short/rfc4754.md` |
| Requirement shard | `rfc/requirements/rfc4754.md` |
| RFC text | `rfc/full/rfc4754.txt` |

## Enrolment

Enrolled: ENROLLED by owner decision, 2026-10-30: Thomas brought RFC 4754 into scope, so it moved from the optional to the required tier of the vpn-endpoint and vpn-concentrator profiles. The extraction sign-off is rfc/extraction/rfc4754.json over rfc/full/rfc4754.txt. The single MUST-level requirement, RFC4754-7-1 (the Section 7 r-and-s signature encoding), is met on the receive side by `internal/component/ike/engine/auth_ecdsa.go::verifyECDSAAuth`, which `verifyRemoteAuth` reaches for AUTH methods 9, 10 and 11, and is proven by TestRFC4754SignatureIsRConcatS in both polarities (rfc/discrimination/rfc4754/RFC4754-7-1.json). The send side is met by `auth_ecdsa.go::signECDSAAuth`, which `computeX509Auth` reaches through `auth_legacy.go::computeLegacySignatureAuth` when the peer sent no SIGNATURE_HASH_ALGORITHMS notify (Thomas approved signing with these methods on 2026-10-31): TestRFC8247LegacySignatureWithoutNotify and TestRFC4754SignerPadsRAndS prove the r-and-s encoding Ze emits, and the `x509-legacy-ecdsa-p256-signature`, `-p384-` and `-p521-` interop scenarios prove strongSwan 6.1.0 with RFC 7427 disabled verifies it.

## What the public ledger says

**Area:** ECDSA authentication methods 9, 10 and 11

**Proof:** 1 of 1 MUST proven

**Evidence:** unit 1

**Unmet:** -


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

## Figures

- **MUST bar**: Ze proven 1
- **Overview bar**: Ze MUST proven 1

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Not walked | 0 |
| Mandatory register | complete |
| MUST rows | 1 |
| MUST rows kept (the MUST bar) | 1 |
| Ze proven | 1 |
| OS verified | 0 |
| OS testing gap | 0 |
| Ze correctness gap | 0 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 0 |
| SHOULD proven | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD not ever | 0 |
| SHOULD not yet | 0 |
| SHOULD not decided | 0 |

The MUST bar counts the MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT rows of the walked RFCs that the stack owes: Ze proven means a recorded test break proves the row; a row a layer under Ze performs is OS verified when a test checks that layer on the state Ze installs, an OS testing gap until one does. The overview bar adds the MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT rows and the rows not owed, which do not bind Ze. An RFC that is not walked has no valid extraction sign-off, so its rows count in no figure. Every count is read from the RFC data files when the page loads.

Source: [data/rfc/rfc4754.json](../../../data/rfc/rfc4754.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC4754-7-1` | When ECDSA-256, ECDSA-384, or ECDSA-521 is used as the digital signature in IKE or IKEv2, the signature payload SHALL contain an encoding of the computed signature consisting of the concatenation of a pair of integers r and s. (§7) | SHALL | 7 | **positive:** `unit/verify` [`TestRFC4754SignatureIsRConcatS`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4754_auth_test.go#L83). **positive:** `unit/verify` [`TestRFC4754SignerPadsRAndS`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc8247_legacy_auth_test.go#L320). **positive:** `unit/verify` [`TestRFC8247LegacySignatureWithoutNotify`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc8247_legacy_auth_test.go#L73). **negative:** `unit/verify` [`TestRFC4754SignatureIsRConcatS`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4754_auth_test.go#L84) |

## Gaps and untested MUSTs

RFC 4754 declares no gap, and every gated MUST it carries has a test bound to it.

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC4754-7-1`](#rfc4754-7-1)

When ECDSA-256, ECDSA-384, or ECDSA-521 is used as the digital signature in IKE or IKEv2, the signature payload SHALL contain an encoding of the computed signature consisting of the concatenation of a pair of integers r and s. (§7)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged after TestRFC4754SignerPadsRAndS began forcing both short integers. RFC 4754 Section 7: the ECDSA-256/384/521 signature payload SHALL be r concatenated with s, 256/384/528 bits each, left-padded with zeros. Receive side: TestRFC4754SignatureIsRConcatS, through verifyRemoteAuth for methods 9, 10 and 11, accepts r||s with a padded short r and refuses s||r (errSignatureFailed) and the DER SEQUENCE (length); its records are line mutants of verifyECDSAAuth observed red. Send side: TestRFC8247LegacySignatureWithoutNotify drives computeX509Auth with no notify for each curve and requires method 9/10/11, exactly 64/96/132 octets, and ecdsa.Verify of the first half as r and the second as s with SHA-256/384/512, so DER output, a swap, a wrong hash or unpadded scalars go red. TestRFC4754SignerPadsRAndS loops computeX509Auth (up to 20000 tries) until it has seen both an r and an s below 2^248, requiring 64 octets on every signature, a leading zero octet on each short integer and a verifying r||s, and fails if either case was never seen, so a signer that leaves r or s unpadded goes red deterministically; the earlier s-padding gap is closed. Send-side tags are positive only; the negative polarity is the receive-side test.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4754SignatureIsRConcatS`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4754_auth_test.go#L84) | unit/verify | mutant, verified |
| positive | [`TestRFC4754SignatureIsRConcatS`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc4754_auth_test.go#L83) | unit/verify | mutant, verified |
| positive | [`TestRFC4754SignerPadsRAndS`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc8247_legacy_auth_test.go#L320) | unit/verify | revert, verified |
| positive | [`TestRFC8247LegacySignatureWithoutNotify`](https://github.com/ze-software/ze/blob/main/internal/component/ike/engine/rfc8247_legacy_auth_test.go#L73) | unit/verify | revert, verified |

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
