# RFC 1071 - Computing the Internet Checksum

Partial. Every requirement this repository extracted from RFC 1071, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 16.7% | 1 of 6 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 83.3% | 5 of 6 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 6 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 6 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| No test at all | 0.0% | 0 of 6 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 53.8% | 7 of 13 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |
| Audit verdicts | 6 | of 6 gated MUSTs judged | 0 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 6 | of 9 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 6 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 6 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 6 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 6 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

The 8 shares marked as a part above are the whole of the 6 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Audit verdicts | ok | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Partial |
| Enrolment | Enrolled |
| Requirements | 9 |
| Gated MUST-level | 6 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 13 |
| Tagged units | 13 |
| Recorded audit verdicts | 6 |
| Discrimination records | 7 |
| Summary | `rfc/short/rfc1071.md` |
| Requirement shard | `rfc/requirements/rfc1071.md` |
| RFC text | `rfc/full/rfc1071.txt` |

## Enrolment

Enrolled: Computing the Internet Checksum (ones-complement 16-bit): eight MUST-level requirements, all met across ze's four checksum implementations (OSPF header/LSA, VRRP, ICMP probe, RSVP-TE). 1-5 (verification: the sum including the checksum folds to 0xffff) and x-1 (OSPF excludes the 8-octet Authentication field) carry positive+negative tags. The arithmetic-shape MUSTs 1-1 (zero the field before computing, store the complement), 1-2 (ones-complement sum with end-around carry), 1-3 (store the bitwise complement of the sum), 1-4 (odd-length zero-pad in the sum only, transmitted length unchanged), 1-6 (fold carries until none remain), and x-2 (AuType2 checksum field is zero) are {single-polarity: positive}, each having no reject path. Tags on internal/plugins/ospf/packet, internal/plugins/ospf/types, internal/core/probe, internal/plugins/vrrp/packet tests.

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

Ze computes the ones-complement 16-bit checksum in its own Go for the OSPF packet header and LSAs (`PacketChecksum`, `FinalizeLSAChecksum`), VRRP (`FillChecksum`), the ICMP probe (`icmpChecksum`) and RSVP-TE, and verifies received OSPF and VRRP packets (`VerifyPacketChecksum`, `verifyChecksumSum`).

**What the ledger says remains**

Every MUST row (1-1 to 1-6) is audited enforced. The verification rule ([`RFC1071-1-5`](#rfc1071-1-5)) is proven on the OSPFv2 packet verifier (`VerifyPacketChecksum`) alone: VRRP's `verifyChecksumSum` is implemented but carries no 1-5 test, and the ICMP probe verifies no received checksum.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 1 | one part of the gated population |
| Annotated (including scoped evidence) | 5 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **6** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (1):** [`RFC1071-1-5`](#rfc1071-1-5)

**Annotated (including scoped evidence) (5):** [`RFC1071-1-1`](#rfc1071-1-1), [`RFC1071-1-2`](#rfc1071-1-2), [`RFC1071-1-3`](#rfc1071-1-3), [`RFC1071-1-4`](#rfc1071-1-4), [`RFC1071-1-6`](#rfc1071-1-6)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC1071-1-1` | To generate a checksum, the checksum field itself is cleared, the 16-bit 1's complement sum is computed over the octets concerned, and the 1's complement of this sum is placed in the checksum field. (§1) | MUST | 1 | **positive:** `unit/verify` [`TestOSPFPacketChecksum`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/checksum_test.go#L14). **positive:** `unit/verify` [`TestRFC1071PacketChecksumClearedSummedAndComplemented`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/rfc1071_checksum_test.go#L60). **negative:** no negative test. **{single-polarity}:** generate-side shape -- header.go:301 zeroes the Checksum field and header.go:322-323 stores the complemented PacketChecksum, pinned by the round-trip test; a generate rule has no reject path, corruption detection being requirement 1-5 |
| `RFC1071-1-2` | On a 2's complement machine, the 1's complement sum must be computed by means of an "end around carry", i.e., any overflows from the most significant bits are added into the least significant bits. (§1) | MUST | 1 | **positive:** `unit/verify` [`TestChecksumRFC1071`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/checksum_test.go#L30). **negative:** no negative test. **{single-polarity}:** pure accumulator -- vrrp/packet/checksum.go:19-38 onesComplementSum+fold is cross-checked against an independent straight-line RFC 1071 reference on even and odd inputs, which discriminates a missing end-around carry; a summation function has no reject path |
| `RFC1071-1-3` | the 16-bit 1's complement sum is computed over the octets concerned, and the 1's complement of this sum is placed in the checksum field. (§1) | MUST | 1 | **positive:** `unit/verify` [`TestInternetChecksumRFC1071Vectors`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/types/checksum_test.go#L61). **positive:** `unit/verify` [`TestRFC1071PacketChecksumClearedSummedAndComplemented`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/rfc1071_checksum_test.go#L61). **negative:** no negative test. **{single-polarity}:** generate-side shape -- internetChecksum returns the bitwise-NOT of the folded sum (ospf/types/checksum.go:102) and the exact vector 0x1411 fails if the complement is dropped; a generate rule has no reject path |
| `RFC1071-1-4` | Using the notation [a,b] for the 16-bit integer a*256+b, where a and b are bytes, then the 16-bit 1's complement sum of these bytes is given by one of the following: [A,B] +' [C,D] +' ... +' [Y,Z] [1] [A,B] +' [C,D] +' ... +' [Z,0] [2] where +' indicates 1's complement addition. These cases correspond to an even or odd count of bytes, respectively. (§1) | MUST | 1 | **positive:** `unit/verify` [`TestInternetChecksumEvenLength`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/types/rfc_checksum_forms_test.go#L16). **positive:** `unit/verify` [`TestInternetChecksumOddLength`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/types/checksum_test.go#L97). **positive:** `unit/verify` [`TestRFC1071ChecksumEvenLength`](https://github.com/ze-software/ze/blob/main/internal/core/probe/rfc1071_even_test.go#L16). **positive:** `unit/verify` [`TestRFC792ChecksumOddLength`](https://github.com/ze-software/ze/blob/main/internal/core/probe/icmp_test.go#L254). **negative:** no negative test. **{single-polarity}:** generate-side shape -- internetSum pads an odd tail with one zero octet for the sum only (ospf/types/checksum.go:146-148) while the transmitted length stays odd, pinned by the odd-length vectors; a generate rule has no reject path |
| `RFC1071-1-5` | To check a checksum, the 1's complement sum is computed over the same set of octets, including the checksum field. If the result is all 1 bits (-0 in 1's complement arithmetic), the check succeeds. (§1) | MUST | 1 | **positive:** `unit/verify` [`TestRFC1071VerifyAcceptsAllOnesSum`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/rfc1071_checksum_test.go#L82). **negative:** `unit/verify` [`TestRFC1071VerifyRefusesSumNotAllOnes`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/rfc1071_checksum_test.go#L100) |
| `RFC1071-1-6` | When the sum has been computed, we "fold" the long sum into 16 bits by adding the 16-bit segments. Each 16-bit addition may produce new end-around carries that must be added. (§1) | MUST | 1 | **positive:** `unit/verify` [`TestInternetChecksumRFC1071Vectors`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/types/checksum_test.go#L62). **positive:** `unit/verify` [`TestRFC1071FoldRepeatsUntilNoCarry`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/types/rfc1071_fold_test.go#L14). **negative:** no negative test. **{single-polarity}:** pure arithmetic -- internetChecksum folds the wide accumulator until no high bits remain before inverting (ospf/types/checksum.go:99-101), exercised by a carry-producing vector; a fold has no reject path |
| `RFC1071-2-1` | As long as the even/odd assignment of bytes is respected, the sum can be done in any order, and it can be arbitrarily split into groups. (§1) | SHOULD | 1 | **positive:** no positive test. **negative:** no negative test |
| `RFC1071-2-2` | Furthermore, again the byte order does not matter; we could instead sum 32-bit words: [D,C,B,A]+'... or [B,A,D,C]+'... and then swap the bytes of the final 16-bit sum as necessary. (§1) | MAY | 1 | **positive:** no positive test. **negative:** no negative test |
| `RFC1071-2-3` | In these cases it is possible to update the checksum without scanning the message or datagram. To update the checksum, simply add the differences of the sixteen bit integers that have been changed. (§1) | MAY | 1 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

RFC 1071 declares no gap, and every gated MUST it carries has a test bound to it.

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC1071-1-1`](#rfc1071-1-1)

To generate a checksum, the checksum field itself is cleared, the 16-bit 1's complement sum is computed over the octets concerned, and the 1's complement of this sum is placed in the checksum field. (§1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged c18. Sentence: clear the field, sum, place the complement. TestRFC1071PacketChecksumClearedSummedAndComplemented encodes a Hello through Packet.WriteTo with a stale header Checksum 0xBEEF into a 0xA5-prefilled buffer and asserts the field equals the test's own RFC 1071 checksum (independent 64-bit fold, field cleared, auth excluded, complemented), equals the same packet encoded from a zero Checksum, and that the covered packet then folds to 0xFFFF. An encoder that skipped the clear (header.go WriteTo h.Checksum = 0) would sum 0xBEEF and go red; one that skipped the complement or placed nothing goes red on the independent value. Record breaks PacketChecksum (the sum/complement producer, checksum.go); the clear and the placement live in Packet.WriteTo, which the record tool cannot name because header.go holds three WriteTo, so their discrimination is argued from the assertions, not recorded. Old TestOSPFPacketChecksum (round-trip only) kept as supplementary. {single-polarity: positive} holds: a generate rule has no reject path; corruption detection is RFC1071-1-5.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestOSPFPacketChecksum`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/checksum_test.go#L14) | unit/verify | unproven |
| positive | [`TestRFC1071PacketChecksumClearedSummedAndComplemented`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/rfc1071_checksum_test.go#L60) | unit/verify | revert, verified |

### [`RFC1071-1-2`](#rfc1071-1-2)

On a 2's complement machine, the 1's complement sum must be computed by means of an "end around carry", i.e., any overflows from the most significant bits are added into the least significant bits. (§1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a one's complement sum without end-around carry. Red on it: TestChecksumRFC1071 compares checksum16 with an independent straight-line reference (t.Fatalf on got != want) over inputs whose sums overflow 16 bits (0xDEAD+0xBEEF, the golden VRRP packets). Single-polarity marker on the row.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestChecksumRFC1071`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/checksum_test.go#L30) | unit/verify | unproven |

### [`RFC1071-1-3`](#rfc1071-1-3)

the 16-bit 1's complement sum is computed over the octets concerned, and the 1's complement of this sum is placed in the checksum field. (§1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged c18. Sub-span: compute the 16-bit one's complement sum and place its complement in the checksum field. Placement is now ze's: TestRFC1071PacketChecksumClearedSummedAndComplemented reads back the field Packet.WriteTo wrote and compares it with the complement of the test's independent sum (the 0xFFFF fold of the covered packet pins the complement). The exact vector 0x1411 in TestInternetChecksumRFC1071Vectors still pins the complement in internetChecksum. Record breaks PacketChecksum, observed red. Single-polarity positive valid (no reject path).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC1071PacketChecksumClearedSummedAndComplemented`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/rfc1071_checksum_test.go#L61) | unit/verify | revert, verified |
| positive | [`TestInternetChecksumRFC1071Vectors`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/types/checksum_test.go#L61) | unit/verify | unproven |

### [`RFC1071-1-4`](#rfc1071-1-4)

Using the notation [a,b] for the 16-bit integer a*256+b, where a and b are bytes, then the 16-bit 1's complement sum of these bytes is given by one of the following: [A,B] +' [C,D] +' ... +' [Y,Z] [1] [A,B] +' [C,D] +' ... +' [Z,0] [2] where +' indicates 1's complement addition. These cases correspond to an even or odd count of bytes, respectively. (§1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Both forms of the RFC 1071 Section 1 sentence are now pinned by exact vectors on both producers. Even count [1]: TestRFC1071ChecksumEvenLength (icmpChecksum) and TestInternetChecksumEvenLength (internetChecksum) answer 0x9753 for [12 34 56 78] and 0xfc96 for [12 34 56 78 9a bc]; a little-endian word, a padded even tail or a missing end-around carry (the 6-octet sum 0x10368) each goes red. Odd count [2] [Z,0]: TestInternetChecksumOddLength pins 0x97cb and TestRFC792ChecksumOddLength folds an 11-octet echo to 0xffff. The {single-polarity: positive} holds: the row defines the sum a generator forms and has no reject path.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC792ChecksumOddLength`](https://github.com/ze-software/ze/blob/main/internal/core/probe/icmp_test.go#L254) | unit/verify | unproven |
| positive | [`TestRFC1071ChecksumEvenLength`](https://github.com/ze-software/ze/blob/main/internal/core/probe/rfc1071_even_test.go#L16) | unit/verify | revert, verified |
| positive | [`TestInternetChecksumOddLength`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/types/checksum_test.go#L97) | unit/verify | unproven |
| positive | [`TestInternetChecksumEvenLength`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/types/rfc_checksum_forms_test.go#L16) | unit/verify | revert, verified |

### [`RFC1071-1-5`](#rfc1071-1-5)

To check a checksum, the 1's complement sum is computed over the same set of octets, including the checksum field. If the result is all 1 bits (-0 in 1's complement arithmetic), the check succeeds. (§1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged c18 (was wrong: probe units checked a test helper). Sentence: to check, sum the same octets including the checksum field; all ones succeeds. Positive TestRFC1071VerifyAcceptsAllOnesSum places the TEST's independent checksum, proves in setup the covered fold including the field is 0xFFFF, and ze's VerifyPacketChecksum accepts. Negative TestRFC1071VerifyRefusesSumNotAllOnes, from a verifying base, changes only the checksum field (+1), one covered body octet, or zeroes the field; each is setup-checked as not folding to 0xFFFF and each is refused. The field-only cases catch a verifier that left the field out of the sum or compared against nothing. Both polarities recorded red under a VerifyPacketChecksum break. The probe RFC792 units lost their 1-5 tags (D-15/R1): probe has no verify function. Scope: OSPFv2 packet verify (InternetChecksumPairValid); other ze verifiers (vrrp) are not tagged here.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1071VerifyRefusesSumNotAllOnes`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/rfc1071_checksum_test.go#L100) | unit/verify | revert, verified |
| positive | [`TestRFC1071VerifyAcceptsAllOnesSum`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/rfc1071_checksum_test.go#L82) | unit/verify | revert, verified |

### [`RFC1071-1-6`](#rfc1071-1-6)

When the sum has been computed, we "fold" the long sum into 16 bits by adding the 16-bit segments. Each 16-bit addition may produce new end-around carries that must be added. (§1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged c18. Sentence: fold the long sum by adding 16-bit segments; each addition may carry again and those carries must be added. TestRFC1071FoldRepeatsUntilNoCarry sums FFFF FFFF 0001 = 0x1FFFF, whose first fold 0x10000 carries again; internetChecksum and InternetChecksumPair must both answer exactly 0xFFFE (a single fold answers 0xFFFF), and internetChecksumValid accepts the data with 0xFFFE appended. Record breaks internetChecksum, observed red. TestInternetChecksumRFC1071Vectors prose narrowed to its one carry (D-15). Single-polarity positive valid: a fold has no reject path.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestInternetChecksumRFC1071Vectors`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/types/checksum_test.go#L62) | unit/verify | unproven |
| positive | [`TestRFC1071FoldRepeatsUntilNoCarry`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/types/rfc1071_fold_test.go#L14) | unit/verify | revert, verified |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | prose |
| Source | rfc/full/rfc1071.txt |
| Source fingerprint | 14cc1f9826f8d215 |
| Record | rfc/extraction/rfc1071.json |
| Mapped sentences | 2 |
| Declined as scope | 16 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | walked | not stated |
| `1` | not stated | 2 | walked | not stated |
| `3` | not stated | 0 | walked | not stated |
| `4` | not stated | 0 | walked | not stated |
| `4.1` | not stated | 0 | walked | not stated |
| `4.2` | not stated | 0 | walked | not stated |
| `4.3` | not stated | 0 | walked | not stated |
| `4.4` | not stated | 16 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `4.4:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | A comment block inside the IBM 370 assembly listing of Section 4.4, stating a register-pairing constraint of that example program ("(RCARRY, RSUM) must be an even/odd register pair"). A code comment in a machine-specific sample, not an obligation on an implementation. | * Registers RADDR and RCOUNT contain the address and length of * the block to be checksummed. * * (RCARRY, RSUM) must be an even/odd register pair. * (RCOUNT, RMOD) must be an even/odd register pair. * CHECKSUM SR RSUM,RSUM Clear working registers. |
| `4.4:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Prose from the IEN-45 paper "Checksum Function Design" by Bill Plummer, which Section 1 says is reproduced here as an extended appendix ("Since IEN-45 has not been widely available, we include it as an extended appendix to this RFC"). The splitter absorbed it into Section 4.4. It is a 1978 design study of candidate checksum functions, not an obligation of this memo; the lowercase "must"/"required" here is a statement of the paper's own scope ("The focus in this paper is on checksum functions for protocols such as TCP"). | The focus in this paper is on checksum functions for protocols such as TCP where the required reliable delivery is achieved by retransmission. |
| `4.4:3` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Prose from the IEN-45 paper "Checksum Function Design" by Bill Plummer, which Section 1 says is reproduced here as an extended appendix ("Since IEN-45 has not been widely available, we include it as an extended appendix to this RFC"). The splitter absorbed it into Section 4.4. It is a 1978 design study of candidate checksum functions, not an obligation of this memo; the lowercase "must"/"required" here is the paper introducing its own property list P1-P7 for evaluating candidate schemes. | Albeit imprecise, another property which must be preserved in any future checksum scheme is: |
| `4.4:4` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Prose from the IEN-45 paper "Checksum Function Design" by Bill Plummer, which Section 1 says is reproduced here as an extended appendix ("Since IEN-45 has not been widely available, we include it as an extended appendix to this RFC"). The splitter absorbed it into Section 4.4. It is a 1978 design study of candidate checksum functions, not an obligation of this memo; the lowercase "must"/"required" here is a description of why the sample loops carry an extra instruction. | The "extra" instruction in the loops above are required to convert the two's complement ADD instruction(s) into a one's complement add by making the carries be end-around. |
| `4.4:5` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Prose from the IEN-45 paper "Checksum Function Design" by Bill Plummer, which Section 1 says is reproduced here as an extended appendix ("Since IEN-45 has not been widely available, we include it as an extended appendix to this RFC"). The splitter absorbed it into Section 4.4. It is a 1978 design study of candidate checksum functions, not an obligation of this memo; the lowercase "must"/"required" here is a mathematical precondition of a candidate residue-code scheme the paper evaluates and does not adopt. | K must be relatively prime to the base chosen to express the message. |
| `4.4:6` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Prose from the IEN-45 paper "Checksum Function Design" by Bill Plummer, which Section 1 says is reproduced here as an extended appendix ("Since IEN-45 has not been widely available, we include it as an extended appendix to this RFC"). The splitter absorbed it into Section 4.4. It is a 1978 design study of candidate checksum functions, not an obligation of this memo; the lowercase "must"/"required" here is a step in the paper's description of a candidate product-code scheme. | In order to form Ms, the sender must multiply the multiple precision "number" Mo by 2**16 - 1. |
| `4.4:7` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Prose from the IEN-45 paper "Checksum Function Design" by Bill Plummer, which Section 1 says is reproduced here as an extended appendix ("Since IEN-45 has not been widely available, we include it as an extended appendix to this RFC"). The splitter absorbed it into Section 4.4. It is a 1978 design study of candidate checksum functions, not an obligation of this memo; the lowercase "must"/"required" here is an explanation of why the candidate scheme uses one's complement arithmetic. | Since carries must propagate between digits, but it is only the current digit which is of interest, one's complement arithmetic is used. |
| `4.4:8` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Prose from the IEN-45 paper "Checksum Function Design" by Bill Plummer, which Section 1 says is reproduced here as an extended appendix ("Since IEN-45 has not been widely available, we include it as an extended appendix to this RFC"). The splitter absorbed it into Section 4.4. It is a 1978 design study of candidate checksum functions, not an obligation of this memo; the lowercase "must"/"required" here is a description of sender retransmission buffers under a candidate scheme. | In general the original copy, Mo, will have to be retained by the sender for retransmission purposes and therefore must remain readable. |
| `4.4:9` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Prose from the IEN-45 paper "Checksum Function Design" by Bill Plummer, which Section 1 says is reproduced here as an extended appendix ("Since IEN-45 has not been widely available, we include it as an extended appendix to this RFC"). The splitter absorbed it into Section 4.4. It is a 1978 design study of candidate checksum functions, not an obligation of this memo; the lowercase "must"/"required" here is an accounting of memory cycles in a PDP-11 sample loop. | Thus the MOV R0,(R3)+ is required which accounts for 2 of the 8 memory cycles per loop. |
| `4.4:10` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Prose from the IEN-45 paper "Checksum Function Design" by Bill Plummer, which Section 1 says is reproduced here as an extended appendix ("Since IEN-45 has not been widely available, we include it as an extended appendix to this RFC"). The splitter absorbed it into Section 4.4. It is a 1978 design study of candidate checksum functions, not an obligation of this memo; the lowercase "must"/"required" here is a refinement the paper proposes for one of its candidate schemes. | A slight refinement of the procedure is required in order to protect against an all-zero message passing to the destination. |
| `4.4:11` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Prose from the IEN-45 paper "Checksum Function Design" by Bill Plummer, which Section 1 says is reproduced here as an extended appendix ("Since IEN-45 has not been widely available, we include it as an extended appendix to this RFC"). The splitter absorbed it into Section 4.4. It is a 1978 design study of candidate checksum functions, not an obligation of this memo; the lowercase "must"/"required" here is the paper announcing its own evaluation of a candidate against its P1-P7 list. | The product code checksum must be evaluated in terms of the desired properties P1 - P7. |
| `4.4:12` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Prose from the IEN-45 paper "Checksum Function Design" by Bill Plummer, which Section 1 says is reproduced here as an extended appendix ("Since IEN-45 has not been widely available, we include it as an extended appendix to this RFC"). The splitter absorbed it into Section 4.4. It is a 1978 design study of candidate checksum functions, not an obligation of this memo; the lowercase "must"/"required" here is a criticism of a hardware checksum-processor option the paper rejects. | In general this is not a very good solution since such a processor must be constructed for every different host machine which uses TCP messages. |
| `4.4:13` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Prose from the IEN-45 paper "Checksum Function Design" by Bill Plummer, which Section 1 says is reproduced here as an extended appendix ("Since IEN-45 has not been widely available, we include it as an extended appendix to this RFC"). The splitter absorbed it into Section 4.4. It is a 1978 design study of candidate checksum functions, not an obligation of this memo; the lowercase "must"/"required" here is a consequence the paper draws for a frontend-processor architecture it discusses. | A separate, small protocol must be developed to cover this link. |
| `4.4:14` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Prose from the IEN-45 paper "Checksum Function Design" by Bill Plummer, which Section 1 says is reproduced here as an extended appendix ("Since IEN-45 has not been widely available, we include it as an extended appendix to this RFC"). The splitter absorbed it into Section 4.4. It is a 1978 design study of candidate checksum functions, not an obligation of this memo; the lowercase "must"/"required" here is an explanation of why a frontend offload reduces host cost. | The reason this scheme reduces the computing burden on the host is that all that is required in order to validate the message using the end-to-end checksum is to send it back to the frontend machine. |
| `4.4:15` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Prose from the IEN-45 paper "Checksum Function Design" by Bill Plummer, which Section 1 says is reproduced here as an extended appendix ("Since IEN-45 has not been widely available, we include it as an extended appendix to this RFC"). The splitter absorbed it into Section 4.4. It is a 1978 design study of candidate checksum functions, not an obligation of this memo; the lowercase "must"/"required" here is a PDP-10 memory-cycle measurement. | In the case of the PDP-10, this requires only 0.5 memory cycles per 16-bit byte of Internet message, and only a few processor cycles to setup the required transfers. |
| `4.4:16` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Prose from the IEN-45 paper "Checksum Function Design" by Bill Plummer, which Section 1 says is reproduced here as an extended appendix ("Since IEN-45 has not been widely available, we include it as an extended appendix to this RFC"). The splitter absorbed it into Section 4.4. It is a 1978 design study of candidate checksum functions, not an obligation of this memo; the lowercase "must"/"required" here is a drawback the paper attributes to one of its candidate schemes. | It suffers mainly in that messages must be (at least partially) decoded by intermediate gateways in order that they can be forwarded. |

## Superseded

No document obsoletes RFC 1071, so its obligations are stated where they were written.
