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
| No test at all | 0.0% | 0 of 6 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 0.0% | 0 of 8 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 6 | of 9 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 6 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 6 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 6 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 6 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Audit verdicts | 6 | of 6 gated MUSTs judged | 5 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

The 7 shares marked as a part above are the whole of the 6 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Audit verdicts | bad | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

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
| Test tags | 8 |
| Tagged units | 8 |
| Recorded audit verdicts | 6 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc1071.md` |
| Requirement shard | `rfc/requirements/rfc1071.md` |
| RFC text | `rfc/full/rfc1071.txt` |

## Enrolment

Enrolled: Computing the Internet Checksum (ones-complement 16-bit): eight MUST-level requirements, all met across ze's four checksum implementations (OSPF header/LSA, VRRP, ICMP probe, RSVP-TE). 1-5 (verification: the sum including the checksum folds to 0xffff) and x-1 (OSPF excludes the 8-octet Authentication field) carry positive+negative tags. The arithmetic-shape MUSTs 1-1 (zero the field before computing, store the complement), 1-2 (ones-complement sum with end-around carry), 1-3 (store the bitwise complement of the sum), 1-4 (odd-length zero-pad in the sum only, transmitted length unchanged), 1-6 (fold carries until none remain), and x-2 (AuType2 checksum field is zero) are {single-polarity: positive}, each having no reject path. Tags on internal/plugins/ospf/packet, internal/plugins/ospf/types, internal/core/probe, internal/plugins/vrrp/packet tests.

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

Ze computes the ones-complement 16-bit checksum in its own Go for the OSPF packet header and LSAs (`PacketChecksum`, `FinalizeLSAChecksum`), VRRP (`FillChecksum`), the ICMP probe (`icmpChecksum`) and RSVP-TE, and verifies received OSPF and VRRP packets (`VerifyPacketChecksum`, `verifyChecksumSum`).

**What the ledger says remains:**

The verification rule ([`RFC1071-1-5`](#rfc1071-1-5)) is audited wrong: its tagged tests check the generated checksum with a test helper and drive no Ze verify function, so receive-side checksum verification is implemented but not proven under this document.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 1 | one part of the gated population |
| Annotated instead of tested | 5 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **6** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (1):** [`RFC1071-1-5`](#rfc1071-1-5)

**Annotated instead of tested (5):** [`RFC1071-1-1`](#rfc1071-1-1), [`RFC1071-1-2`](#rfc1071-1-2), [`RFC1071-1-3`](#rfc1071-1-3), [`RFC1071-1-4`](#rfc1071-1-4), [`RFC1071-1-6`](#rfc1071-1-6)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC1071-1-1` | To generate a checksum, the checksum field itself is cleared, the 16-bit 1's complement sum is computed over the octets concerned, and the 1's complement of this sum is placed in the checksum field. (§1) | MUST | 1 | **positive:** `unit/verify` [`TestOSPFPacketChecksum`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/checksum_test.go#L14). **negative:** no negative test. **{single-polarity}:** generate-side shape -- header.go:301 zeroes the Checksum field and header.go:322-323 stores the complemented PacketChecksum, pinned by the round-trip test; a generate rule has no reject path, corruption detection being requirement 1-5 |
| `RFC1071-1-2` | On a 2's complement machine, the 1's complement sum must be computed by means of an "end around carry", i.e., any overflows from the most significant bits are added into the least significant bits. (§1) | MUST | 1 | **positive:** `unit/verify` [`TestChecksumRFC1071`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/checksum_test.go#L30). **negative:** no negative test. **{single-polarity}:** pure accumulator -- vrrp/packet/checksum.go:19-38 onesComplementSum+fold is cross-checked against an independent straight-line RFC 1071 reference on even and odd inputs, which discriminates a missing end-around carry; a summation function has no reject path |
| `RFC1071-1-3` | the 16-bit 1's complement sum is computed over the octets concerned, and the 1's complement of this sum is placed in the checksum field. (§1) | MUST | 1 | **positive:** `unit/verify` [`TestInternetChecksumRFC1071Vectors`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/types/checksum_test.go#L61). **negative:** no negative test. **{single-polarity}:** generate-side shape -- internetChecksum returns the bitwise-NOT of the folded sum (ospf/types/checksum.go:102) and the exact vector 0x1411 fails if the complement is dropped; a generate rule has no reject path |
| `RFC1071-1-4` | Using the notation [a,b] for the 16-bit integer a*256+b, where a and b are bytes, then the 16-bit 1's complement sum of these bytes is given by one of the following: [A,B] +' [C,D] +' ... +' [Y,Z] [1] [A,B] +' [C,D] +' ... +' [Z,0] [2] where +' indicates 1's complement addition. These cases correspond to an even or odd count of bytes, respectively. (§1) | MUST | 1 | **positive:** `unit/verify` [`TestInternetChecksumOddLength`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/types/checksum_test.go#L97). **positive:** `unit/verify` [`TestRFC792ChecksumOddLength`](https://github.com/ze-software/ze/blob/main/internal/core/probe/icmp_test.go#L231). **negative:** no negative test. **{single-polarity}:** generate-side shape -- internetSum pads an odd tail with one zero octet for the sum only (ospf/types/checksum.go:146-148) while the transmitted length stays odd, pinned by the odd-length vectors; a generate rule has no reject path |
| `RFC1071-1-5` | To check a checksum, the 1's complement sum is computed over the same set of octets, including the checksum field. If the result is all 1 bits (-0 in 1's complement arithmetic), the check succeeds. (§1) | MUST | 1 | **positive:** `unit/verify` [`TestRFC792ChecksumValid`](https://github.com/ze-software/ze/blob/main/internal/core/probe/icmp_test.go#L210). **negative:** `unit/verify` [`TestRFC792ChecksumRejectsCorruption`](https://github.com/ze-software/ze/blob/main/internal/core/probe/icmp_test.go#L220) |
| `RFC1071-1-6` | When the sum has been computed, we "fold" the long sum into 16 bits by adding the 16-bit segments. Each 16-bit addition may produce new end-around carries that must be added. (§1) | MUST | 1 | **positive:** `unit/verify` [`TestInternetChecksumRFC1071Vectors`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/types/checksum_test.go#L62). **negative:** no negative test. **{single-polarity}:** pure arithmetic -- internetChecksum folds the wide accumulator until no high bits remain before inverting (ospf/types/checksum.go:99-101), exercised by a carry-producing vector; a fold has no reject path |
| `RFC1071-2-1` | As long as the even/odd assignment of bytes is respected, the sum can be done in any order, and it can be arbitrarily split into groups. (§1) | SHOULD | 1 | **positive:** no positive test. **negative:** no negative test |
| `RFC1071-2-2` | Furthermore, again the byte order does not matter; we could instead sum 32-bit words: [D,C,B,A]+'... or [B,A,D,C]+'... and then swap the bytes of the final 16-bit sum as necessary. (§1) | MAY | 1 | **positive:** no positive test. **negative:** no negative test |
| `RFC1071-2-3` | In these cases it is possible to update the checksum without scanning the message or datagram. To update the checksum, simply add the differences of the sixteen bit integers that have been changed. (§1) | MAY | 1 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

RFC 1071 declares no gap, and every gated MUST it carries has a test bound to it.

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC1071-1-1`](#rfc1071-1-1)

To generate a checksum, the checksum field itself is cleared, the 16-bit 1's complement sum is computed over the octets concerned, and the 1's complement of this sum is placed in the checksum field. (§1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Sentence: to generate, the checksum field is cleared, the 16-bit one's complement sum is computed, and its complement is placed in the field. TestOSPFPacketChecksum asserts only that the field is non-zero and that ze's own VerifyPacketChecksum accepts. The clearing clause is not provable there: the encoded header starts with a zero Checksum, so an encoder that skipped the clear passes. No independent expected value pins the complement.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestOSPFPacketChecksum`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/checksum_test.go#L14) | unit/verify | unproven |

### [`RFC1071-1-2`](#rfc1071-1-2)

On a 2's complement machine, the 1's complement sum must be computed by means of an "end around carry", i.e., any overflows from the most significant bits are added into the least significant bits. (§1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a one's complement sum without end-around carry. Red on it: TestChecksumRFC1071 compares checksum16 with an independent straight-line reference (t.Fatalf on got != want) over inputs whose sums overflow 16 bits (0xDEAD+0xBEEF, the golden VRRP packets). Single-polarity marker on the row.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestChecksumRFC1071`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/checksum_test.go#L30) | unit/verify | unproven |

### [`RFC1071-1-3`](#rfc1071-1-3)

the 16-bit 1's complement sum is computed over the octets concerned, and the 1's complement of this sum is placed in the checksum field. (§1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Sub-span: the 16-bit one's complement sum is computed and its complement is placed in the checksum field. The complement is pinned by the exact vector 0x1411 in TestInternetChecksumRFC1071Vectors. The placement clause is not ze's in that unit: the test itself writes the value into data[2:4]; the tagged unit calls internetChecksum only.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestInternetChecksumRFC1071Vectors`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/types/checksum_test.go#L61) | unit/verify | unproven |

### [`RFC1071-1-4`](#rfc1071-1-4)

Using the notation [a,b] for the 16-bit integer a*256+b, where a and b are bytes, then the 16-bit 1's complement sum of these bytes is given by one of the following: [A,B] +' [C,D] +' ... +' [Y,Z] [1] [A,B] +' [C,D] +' ... +' [Z,0] [2] where +' indicates 1's complement addition. These cases correspond to an even or odd count of bytes, respectively. (§1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Sentence gives the even-count form [1] and the odd-count form [Z,0] [2]. The odd clause is proven: exact vector 0x97cb in TestInternetChecksumOddLength and the 0xffff fold of an 11-octet echo in TestRFC792ChecksumOddLength both go red on a dropped or low-byte pad. No 1-4-tagged unit asserts the even-count form. The row's old 'do not transmit the pad' is not in RFC 1071 (it is RFC 793 Section 3.1).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC792ChecksumOddLength`](https://github.com/ze-software/ze/blob/main/internal/core/probe/icmp_test.go#L231) | unit/verify | unproven |
| positive | [`TestInternetChecksumOddLength`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/types/checksum_test.go#L97) | unit/verify | unproven |

### [`RFC1071-1-5`](#rfc1071-1-5)

To check a checksum, the 1's complement sum is computed over the same set of octets, including the checksum field. If the result is all 1 bits (-0 in 1's complement arithmetic), the check succeeds. (§1)

Audit verdict: wrong (the tests assert something other than what the requirement demands), fresh. Sentence: to check a checksum, sum over the same octets including the checksum field and succeed on all ones. Both tagged units (TestRFC792ChecksumValid, TestRFC792ChecksumRejectsCorruption) perform the check with the test helper checksumOnesFold, not with ze code. They prove ze's GENERATED checksum is valid (the generate rule, RFC1071-1-1/RFC792-Echo-3), and that the helper detects corruption. No ze verify function (ospf VerifyPacketChecksum, vrrp verifyChecksumSum) is driven under this tag.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC792ChecksumRejectsCorruption`](https://github.com/ze-software/ze/blob/main/internal/core/probe/icmp_test.go#L220) | unit/verify | unproven |
| positive | [`TestRFC792ChecksumValid`](https://github.com/ze-software/ze/blob/main/internal/core/probe/icmp_test.go#L210) | unit/verify | unproven |

### [`RFC1071-1-6`](#rfc1071-1-6)

When the sum has been computed, we "fold" the long sum into 16 bits by adding the 16-bit segments. Each 16-bit addition may produce new end-around carries that must be added. (§1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Sentence: fold the long sum into 16 bits; each 16-bit addition may produce new end-around carries that must be added. The vector in TestInternetChecksumRFC1071Vectors overflows once (0x1EBED folds to 0xEBEE with no second carry), so a single fold without repetition still returns 0x1411. The repeated-carry clause has no assertion.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestInternetChecksumRFC1071Vectors`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/types/checksum_test.go#L62) | unit/verify | unproven |

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
