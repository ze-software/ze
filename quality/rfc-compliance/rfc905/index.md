# RFC 905 - ISO Transport Protocol Specification (ISO DP 8073)

No row in the public ledger. Every requirement this repository extracted from RFC 905, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 100.0% | 6 of 6 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 6 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 6 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 0.0% | 0 of 6 binding obligations | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 0.0% | 0 of 19 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 9 | of 10 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 3 | of 9 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

The 4 shares marked as a part above are the whole of the 6 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| No test at all | ok | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | No row in the public ledger |
| Enrolment | Enrolled |
| Requirements | 10 |
| Gated MUST-level | 9 |
| Obligations that bind Ze | 6 |
| Not applicable, so out of scope | 3 |
| Declared gaps | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 19 |
| Tagged units | 19 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc905.md` |
| Requirement shard | `rfc/requirements/rfc905.md` |
| RFC text | `rfc/full/rfc905.txt` |

## Enrolment

Enrolled: ISO Transport Protocol: nine MUST-level requirements. The six that ze realizes are the Annex B Fletcher checksum (mod-255 arithmetic, two-pass generation, verify-both-sums-zero, OSPF LSA checksum excluding LS Age, encode+decode vectors) -- x-1,x-2,x-3,x-4,x-6,x-7 -- each bound with positive+negative tags on the IS-IS (internal/plugins/isis/packet/checksum.go) and OSPF (internal/plugins/ospf/types/checksum.go, ospf/packet/checksum.go) checksum tests. The three ISO Transport TPDU MUSTs (6.17.3-1 generation, 6.17.3-2 receive/verify-and-discard, 13.2.3.1-1 0xC3 checksum parameter) are {not-applicable}: ze has no ISO Transport TPDU sender/receiver and no 0xC3 transport-checksum-parameter code path.

## What the public ledger says

No row in the public ledger, so its summary declares `| Support | - |` and docs/features/rfc-status.md carries no row for RFC 905.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 6 | one part of the gated population |
| Annotated instead of tested | 3 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **9** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (6):** [`RFC905-x-1`](#rfc905-x-1), [`RFC905-x-2`](#rfc905-x-2), [`RFC905-x-3`](#rfc905-x-3), [`RFC905-x-4`](#rfc905-x-4), [`RFC905-x-6`](#rfc905-x-6), [`RFC905-x-7`](#rfc905-x-7)

**Annotated instead of tested (3):** [`RFC905-6.17.3-1`](#rfc905-6.17.3-1), [`RFC905-6.17.3-2`](#rfc905-6.17.3-2), [`RFC905-13.2.3.1-1`](#rfc905-13.2.3.1-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC905-6.17.3-1` | Sender sets the checksum so SUM(a[i]) == 0 and SUM(i*a[i]) == 0, both mod 255, over octets 1..L (Section 6.17.3) | MUST | 6.17.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze implements the RFC 905 Annex B Fletcher checksum (reused by IS-IS at internal/plugins/isis/packet/checksum.go and OSPF at internal/plugins/ospf/types/checksum.go) but has no ISO Transport Protocol TPDU generator that sets a transport checksum parameter (grep for TPDU/TP4/0xC3 across internal/ is empty) |
| `RFC905-6.17.3-2` | Receiver (when checksum agreed) discards a TPDU that does not satisfy both checksum formulas (Section 6.17.3) | MUST | 6.17.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze implements the RFC 905 Annex B Fletcher checksum (reused by IS-IS at internal/plugins/isis/packet/checksum.go and OSPF at internal/plugins/ospf/types/checksum.go) but has no ISO Transport Protocol TPDU receiver that verifies and discards a transport TPDU (grep for TPDU/TP4/0xC3 across internal/ is empty) |
| `RFC905-x-1` | Use modulo-255 (one's-complement) arithmetic, NOT modulo 256, for both running sums (Annex B.2) | MUST | x | **positive:** `unit/verify` [`TestISISChecksumFixedVector`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/checksum_test.go#L70). **positive:** `unit/verify` [`TestISISChecksumModulus`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/checksum_test.go#L132). **negative:** `unit/verify` [`TestISISChecksumDetectsCorruption`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/checksum_test.go#L95) |
| `RFC905-x-2` | Generation pass 1: zero the checksum field, then run C0 += octet, C1 += C0 over i = 1..L (Annex B.3.1-B.3.3) | MUST | x | **positive:** `unit/verify` [`TestISISChecksumVectors`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/checksum_test.go#L16). **negative:** `unit/verify` [`TestISISChecksumDetectsCorruption`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/checksum_test.go#L96) |
| `RFC905-x-3` | Generation pass 2: X = -C1 + (L-n)*C0 ; Y = C1 - (L-n+1)*C0 (mod 255), placed at octets n and n+1 (Annex B.3.4-B.3.5) | MUST | x | **positive:** `unit/verify` [`TestISISChecksumFixedVector`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/checksum_test.go#L69). **positive:** `unit/verify` [`TestISISChecksumVectors`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/checksum_test.go#L17). **negative:** `unit/verify` [`TestISISChecksumDetectsCorruption`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/checksum_test.go#L97) |
| `RFC905-x-4` | Verification: run C0 += octet, C1 += C0 over the whole TPDU INCLUDING the checksum octets; both C0 and C1 MUST be zero (Annex B.4) | MUST | x | **positive:** `unit/verify` [`TestISISChecksumVectors`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/checksum_test.go#L18). **negative:** `unit/verify` [`TestISISChecksumDetectsCorruption`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/checksum_test.go#L94). **negative:** `unit/verify` [`TestOSPFLSAChecksumExcludesAge`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/checksum_test.go#L100) |
| `RFC905-x-5` | Treat a computed octet of 0 as 255 so the field cannot collide with the reserved all-zero "no checksum" value (Annex B.2) | SHOULD | x | **positive:** no positive test. **negative:** no negative test |
| `RFC905-13.2.3.1-1` | Encode the checksum parameter as code 0xC3, length 2, value = the two octets X, Y (Section 13.2.3.1) | MUST | 13.2.3.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze implements the RFC 905 Annex B Fletcher checksum (reused by IS-IS at internal/plugins/isis/packet/checksum.go and OSPF at internal/plugins/ospf/types/checksum.go) but has no ISO Transport Protocol code path that encodes the 0xC3 transport checksum parameter (grep for TPDU/TP4/0xC3 across internal/ is empty) |
| `RFC905-x-6` | (OSPF reuse) Compute the LSA Fletcher-16 over the LSA starting at the Options field, EXCLUDING the 2-byte LS Age, with the LS Checksum field zeroed during the forward computation (RFC 2328 Section 12.1.7) | MUST | x | **positive:** `unit/verify` [`TestFletcherIgnoresLSAge`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/types/checksum_test.go#L46). **positive:** `unit/verify` [`TestFletcherRFC905Vectors`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/types/checksum_test.go#L26). **positive:** `unit/verify` [`TestOSPFLSAChecksum`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/checksum_test.go#L73). **negative:** `unit/verify` [`TestOSPFLSAChecksumExcludesAge`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/checksum_test.go#L99) |
| `RFC905-x-7` | (OSPF reuse) Test BOTH encode (X,Y placement) and decode (verify-to-zero) against vectors; a common bug is encode-correct / verify-wrong, which passes self-interop but fails cross-interop | MUST | x | **positive:** `unit/verify` [`TestFletcherRFC905Vectors`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/types/checksum_test.go#L27). **positive:** `unit/verify` [`TestISISChecksumVectors`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/checksum_test.go#L19). **positive:** `unit/verify` [`TestOSPFLSAChecksum`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/checksum_test.go#L74). **negative:** `unit/verify` [`TestOSPFLSAChecksumExcludesAge`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/checksum_test.go#L101) |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC905-6.17.3-1`](#rfc905-6.17.3-1) Sender sets the checksum so SUM(a[i]) == 0 and SUM(i*a[i]) == 0, both mod 255, over octets 1..L (Section 6.17.3) | no test | no test carries this requirement id; annotated {not-applicable}: ze implements the RFC 905 Annex B Fletcher checksum (reused by IS-IS at internal/plugins/isis/packet/checksum.go and OSPF at internal/plugins/ospf/types/checksum.go) but has no ISO Transport Protocol TPDU generator that sets a transport checksum parameter (grep for TPDU/TP4/0xC3 across internal/ is empty) |
| [`RFC905-6.17.3-2`](#rfc905-6.17.3-2) Receiver (when checksum agreed) discards a TPDU that does not satisfy both checksum formulas (Section 6.17.3) | no test | no test carries this requirement id; annotated {not-applicable}: ze implements the RFC 905 Annex B Fletcher checksum (reused by IS-IS at internal/plugins/isis/packet/checksum.go and OSPF at internal/plugins/ospf/types/checksum.go) but has no ISO Transport Protocol TPDU receiver that verifies and discards a transport TPDU (grep for TPDU/TP4/0xC3 across internal/ is empty) |
| [`RFC905-13.2.3.1-1`](#rfc905-13.2.3.1-1) Encode the checksum parameter as code 0xC3, length 2, value = the two octets X, Y (Section 13.2.3.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze implements the RFC 905 Annex B Fletcher checksum (reused by IS-IS at internal/plugins/isis/packet/checksum.go and OSPF at internal/plugins/ospf/types/checksum.go) but has no ISO Transport Protocol code path that encodes the 0xC3 transport checksum parameter (grep for TPDU/TP4/0xC3 across internal/ is empty) |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC905-6.17.3-1`](#rfc905-6.17.3-1)

Sender sets the checksum so SUM(a[i]) == 0 and SUM(i*a[i]) == 0, both mod 255, over octets 1..L (Section 6.17.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC905-6.17.3-1, so no unit is bound to it.

### [`RFC905-6.17.3-2`](#rfc905-6.17.3-2)

Receiver (when checksum agreed) discards a TPDU that does not satisfy both checksum formulas (Section 6.17.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC905-6.17.3-2, so no unit is bound to it.

### [`RFC905-x-1`](#rfc905-x-1)

Use modulo-255 (one's-complement) arithmetic, NOT modulo 256, for both running sums (Annex B.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestISISChecksumDetectsCorruption`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/checksum_test.go#L95) | unit/verify | unproven |
| positive | [`TestISISChecksumFixedVector`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/checksum_test.go#L70) | unit/verify | unproven |
| positive | [`TestISISChecksumModulus`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/checksum_test.go#L132) | unit/verify | unproven |

### [`RFC905-x-2`](#rfc905-x-2)

Generation pass 1: zero the checksum field, then run C0 += octet, C1 += C0 over i = 1..L (Annex B.3.1-B.3.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestISISChecksumDetectsCorruption`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/checksum_test.go#L96) | unit/verify | unproven |
| positive | [`TestISISChecksumVectors`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/checksum_test.go#L16) | unit/verify | unproven |

### [`RFC905-x-3`](#rfc905-x-3)

Generation pass 2: X = -C1 + (L-n)*C0 ; Y = C1 - (L-n+1)*C0 (mod 255), placed at octets n and n+1 (Annex B.3.4-B.3.5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestISISChecksumDetectsCorruption`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/checksum_test.go#L97) | unit/verify | unproven |
| positive | [`TestISISChecksumFixedVector`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/checksum_test.go#L69) | unit/verify | unproven |
| positive | [`TestISISChecksumVectors`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/checksum_test.go#L17) | unit/verify | unproven |

### [`RFC905-x-4`](#rfc905-x-4)

Verification: run C0 += octet, C1 += C0 over the whole TPDU INCLUDING the checksum octets; both C0 and C1 MUST be zero (Annex B.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestISISChecksumDetectsCorruption`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/checksum_test.go#L94) | unit/verify | unproven |
| negative | [`TestOSPFLSAChecksumExcludesAge`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/checksum_test.go#L100) | unit/verify | unproven |
| positive | [`TestISISChecksumVectors`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/checksum_test.go#L18) | unit/verify | unproven |

### [`RFC905-13.2.3.1-1`](#rfc905-13.2.3.1-1)

Encode the checksum parameter as code 0xC3, length 2, value = the two octets X, Y (Section 13.2.3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC905-13.2.3.1-1, so no unit is bound to it.

### [`RFC905-x-6`](#rfc905-x-6)

(OSPF reuse) Compute the LSA Fletcher-16 over the LSA starting at the Options field, EXCLUDING the 2-byte LS Age, with the LS Checksum field zeroed during the forward computation (RFC 2328 Section 12.1.7)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFLSAChecksumExcludesAge`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/checksum_test.go#L99) | unit/verify | unproven |
| positive | [`TestOSPFLSAChecksum`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/checksum_test.go#L73) | unit/verify | unproven |
| positive | [`TestFletcherIgnoresLSAge`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/types/checksum_test.go#L46) | unit/verify | unproven |
| positive | [`TestFletcherRFC905Vectors`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/types/checksum_test.go#L26) | unit/verify | unproven |

### [`RFC905-x-7`](#rfc905-x-7)

(OSPF reuse) Test BOTH encode (X,Y placement) and decode (verify-to-zero) against vectors; a common bug is encode-correct / verify-wrong, which passes self-interop but fails cross-interop

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFLSAChecksumExcludesAge`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/checksum_test.go#L101) | unit/verify | unproven |
| positive | [`TestISISChecksumVectors`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/packet/checksum_test.go#L19) | unit/verify | unproven |
| positive | [`TestOSPFLSAChecksum`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/checksum_test.go#L74) | unit/verify | unproven |
| positive | [`TestFletcherRFC905Vectors`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/types/checksum_test.go#L27) | unit/verify | unproven |

## Extraction sign-off

No extraction sign-off exists for RFC 905, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 905, so its obligations are stated where they were written.
