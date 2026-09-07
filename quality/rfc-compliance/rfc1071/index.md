# RFC 1071 - Computing the Internet Checksum

No row in the public ledger. Every requirement this repository extracted from RFC 1071, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 25.0% | 2 of 8 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 75.0% | 6 of 8 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 8 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 0.0% | 0 of 8 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 0.0% | 0 of 11 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 8 | of 11 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 8 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 8 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 8 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 8 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

The 7 shares marked as a part above are the whole of the 8 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| No test at all | ok | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green shares above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | No row in the public ledger |
| Enrolment | Enrolled |
| Requirements | 11 |
| Gated MUST-level | 8 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 11 |
| Tagged units | 11 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc1071.md` |
| Requirement shard | `rfc/requirements/rfc1071.md` |
| RFC text | `rfc/full/rfc1071.txt` |

## Enrolment

Enrolled: Computing the Internet Checksum (ones-complement 16-bit): eight MUST-level requirements, all met across ze's four checksum implementations (OSPF header/LSA, VRRP, ICMP probe, RSVP-TE). 1-5 (verification: the sum including the checksum folds to 0xffff) and x-1 (OSPF excludes the 8-octet Authentication field) carry positive+negative tags. The arithmetic-shape MUSTs 1-1 (zero the field before computing, store the complement), 1-2 (ones-complement sum with end-around carry), 1-3 (store the bitwise complement of the sum), 1-4 (odd-length zero-pad in the sum only, transmitted length unchanged), 1-6 (fold carries until none remain), and x-2 (AuType2 checksum field is zero) are {single-polarity: positive}, each having no reject path. Tags on internal/plugins/ospf/packet, internal/plugins/ospf/types, internal/core/probe, internal/plugins/vrrp/packet tests.

## What the public ledger says

No row in the public ledger, so its summary declares `| Support | - |` and docs/features/rfc-status.md carries no row for RFC 1071.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 2 | one part of the gated population |
| Annotated instead of tested | 6 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **8** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (2):** [`RFC1071-1-5`](#rfc1071-1-5), [`RFC1071-x-1`](#rfc1071-x-1)

**Annotated instead of tested (6):** [`RFC1071-1-1`](#rfc1071-1-1), [`RFC1071-1-2`](#rfc1071-1-2), [`RFC1071-1-3`](#rfc1071-1-3), [`RFC1071-1-4`](#rfc1071-1-4), [`RFC1071-1-6`](#rfc1071-1-6), [`RFC1071-x-2`](#rfc1071-x-2)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC1071-1-1` | Zero the checksum field before computing the one's-complement sum (§1). | MUST | 1 | **positive:** `unit/verify` [`TestOSPFPacketChecksum`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/checksum_test.go#L14). **negative:** no negative test. **{single-polarity}:** generate-side shape -- header.go:301 zeroes the Checksum field and header.go:322-323 stores the complemented PacketChecksum, pinned by the round-trip test; a generate rule has no reject path, corruption detection being requirement 1-5 |
| `RFC1071-1-2` | Sum 16-bit words with end-around carry (overflow folded into the low-order bit) (§1). | MUST | 1 | **positive:** `unit/verify` [`TestChecksumRFC1071`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/checksum_test.go#L30). **negative:** no negative test. **{single-polarity}:** pure accumulator -- vrrp/packet/checksum.go:19-38 onesComplementSum+fold is cross-checked against an independent straight-line RFC 1071 reference on even and odd inputs, which discriminates a missing end-around carry; a summation function has no reject path |
| `RFC1071-1-3` | Store the one's complement (bitwise NOT) of the 16-bit sum in the checksum field (§1). | MUST | 1 | **positive:** `unit/verify` [`TestInternetChecksumRFC1071Vectors`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/types/checksum_test.go#L61). **negative:** no negative test. **{single-polarity}:** generate-side shape -- internetChecksum returns the bitwise-NOT of the folded sum (ospf/types/checksum.go:102) and the exact vector 0x1411 fails if the complement is dropped; a generate rule has no reject path |
| `RFC1071-1-4` | Pad an odd-length region with one zero octet for the sum only; do not transmit the pad (§1). | MUST | 1 | **positive:** `unit/verify` [`TestInternetChecksumOddLength`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/types/checksum_test.go#L97). **positive:** `unit/verify` [`TestRFC792ChecksumOddLength`](https://github.com/ze-software/ze/blob/main/internal/core/probe/icmp_test.go#L231). **negative:** no negative test. **{single-polarity}:** generate-side shape -- internetSum pads an odd tail with one zero octet for the sum only (ospf/types/checksum.go:146-148) while the transmitted length stays odd, pinned by the odd-length vectors; a generate rule has no reject path |
| `RFC1071-1-5` | Verify by summing over all octets including the checksum field and checking for an all-ones (0xFFFF) result (§1). | MUST | 1 | **positive:** `unit/verify` [`TestRFC792ChecksumValid`](https://github.com/ze-software/ze/blob/main/internal/core/probe/icmp_test.go#L210). **negative:** `unit/verify` [`TestRFC792ChecksumRejectsCorruption`](https://github.com/ze-software/ze/blob/main/internal/core/probe/icmp_test.go#L220) |
| `RFC1071-1-6` | Fold a wide (32/64-bit) accumulator down to 16 bits, repeating until no high bits remain, before inverting (§1, §2). | MUST | 1 | **positive:** `unit/verify` [`TestInternetChecksumRFC1071Vectors`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/types/checksum_test.go#L62). **negative:** no negative test. **{single-polarity}:** pure arithmetic -- internetChecksum folds the wide accumulator until no high bits remain before inverting (ospf/types/checksum.go:99-101), exercised by a carry-producing vector; a fold has no reject path |
| `RFC1071-2-1` | Respect the even/odd octet assignment when splitting or reordering partial sums (commutative/associative property, §2). | SHOULD | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC1071-2-2` | Sum in native byte order and byte-swap only the final 16-bit result (byte-swap property, §2). | MAY | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC1071-2-3` | Update the checksum incrementally from a changed field rather than recomputing (§2; exact form per RFC 1624). | MAY | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC1071-x-1` | (OSPF) Compute over the entire OSPF packet **excluding** the 64-bit Authentication field, with the Checksum field zeroed during computation (RFC 2328 §A.3.1). | MUST | x | **positive:** `unit/verify` [`TestOSPFPacketChecksumExcludesAuth`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/checksum_test.go#L30). **negative:** `unit/verify` [`TestOSPFPacketChecksumExcludesAuth`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/checksum_test.go#L31) |
| `RFC1071-x-2` | (OSPF, AuType 2) Set the Checksum field to zero entirely; do not use it (RFC 2328 §A.3.1). | MUST | x | **positive:** `unit/verify` [`TestOSPFPacketChecksumZeroForAuType2`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/checksum_test.go#L55). **negative:** no negative test. **{single-polarity}:** generate-side convention -- WriteTo leaves the AuType2 Checksum field zero (ospf/packet/header.go:317-321) and VerifyPacketChecksum accepts the zero checksum (ospf/packet/checksum.go:28-30); only the field-is-zero positive is asserted |

## Gaps and untested MUSTs

RFC 1071 declares no gap, and every gated MUST it carries has a test bound to it.

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC1071-1-1`](#rfc1071-1-1)

Zero the checksum field before computing the one's-complement sum (§1).

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestOSPFPacketChecksum`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/checksum_test.go#L14) | unit/verify | unproven |

### [`RFC1071-1-2`](#rfc1071-1-2)

Sum 16-bit words with end-around carry (overflow folded into the low-order bit) (§1).

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestChecksumRFC1071`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/checksum_test.go#L30) | unit/verify | unproven |

### [`RFC1071-1-3`](#rfc1071-1-3)

Store the one's complement (bitwise NOT) of the 16-bit sum in the checksum field (§1).

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestInternetChecksumRFC1071Vectors`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/types/checksum_test.go#L61) | unit/verify | unproven |

### [`RFC1071-1-4`](#rfc1071-1-4)

Pad an odd-length region with one zero octet for the sum only; do not transmit the pad (§1).

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC792ChecksumOddLength`](https://github.com/ze-software/ze/blob/main/internal/core/probe/icmp_test.go#L231) | unit/verify | unproven |
| positive | [`TestInternetChecksumOddLength`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/types/checksum_test.go#L97) | unit/verify | unproven |

### [`RFC1071-1-5`](#rfc1071-1-5)

Verify by summing over all octets including the checksum field and checking for an all-ones (0xFFFF) result (§1).

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC792ChecksumRejectsCorruption`](https://github.com/ze-software/ze/blob/main/internal/core/probe/icmp_test.go#L220) | unit/verify | unproven |
| positive | [`TestRFC792ChecksumValid`](https://github.com/ze-software/ze/blob/main/internal/core/probe/icmp_test.go#L210) | unit/verify | unproven |

### [`RFC1071-1-6`](#rfc1071-1-6)

Fold a wide (32/64-bit) accumulator down to 16 bits, repeating until no high bits remain, before inverting (§1, §2).

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestInternetChecksumRFC1071Vectors`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/types/checksum_test.go#L62) | unit/verify | unproven |

### [`RFC1071-x-1`](#rfc1071-x-1)

(OSPF) Compute over the entire OSPF packet **excluding** the 64-bit Authentication field, with the Checksum field zeroed during computation (RFC 2328 §A.3.1).

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFPacketChecksumExcludesAuth`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/checksum_test.go#L31) | unit/verify | unproven |
| positive | [`TestOSPFPacketChecksumExcludesAuth`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/checksum_test.go#L30) | unit/verify | unproven |

### [`RFC1071-x-2`](#rfc1071-x-2)

(OSPF, AuType 2) Set the Checksum field to zero entirely; do not use it (RFC 2328 §A.3.1).

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestOSPFPacketChecksumZeroForAuType2`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/checksum_test.go#L55) | unit/verify | unproven |

## Extraction sign-off

No extraction sign-off exists for RFC 1071, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 1071, so its obligations are stated where they were written.
