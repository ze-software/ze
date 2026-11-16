# RFC 1071 - Computing the Internet Checksum

Internet checksum (OSPF, VRRP, ICMP probe, RSVP-TE). Every requirement listed for RFC 1071, the tests linked to it, and their recorded reviews. This summary is included in the automatic compliance check, ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 1 | of 6 mandatory requirements | a positive test checks that Ze does what the requirement demands and a negative one checks that it refuses what the requirement forbids |
| One polarity plus reason | 5 | of 6 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason supplies the complete test coverage required |
| One polarity, unexcused | 0 | of 6 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial test coverage | 0 | of 6 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unverified, with zero whole-requirement credit |
| Declared gap or no test | 0 | of 6 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |
| Tests shown to detect a fault | 11 | 11 of 13 tests linked to requirements | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests verifies a requirement only after both tests have been shown to detect a fault |
| Audit verdicts | 6 | of 6 mandatory requirements reviewed | 0 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 6 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 0 | of 6 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 0 | of 6 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze verified: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 6 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 6 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no test verification credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level) | 6 | of 9 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements included in the automatic checks: the compliance check (./le rfc check) holds Ze to each of them. This is the total being assessed; the counts above show the recorded status of those requirements. |

The 9 counts marked as a part above are the whole of the 6 mandatory requirements: they add up to 6. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got. An outlined card with no colored edge, under Totals, is a total the other cards are counted against, not a measure.

| Card | Tone here | Why that color |
|---|---|---|
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the count under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: one test direction is missing without a recorded reason |
| Partial test coverage | ok | green at zero, RED above it: testing one clause does not verify the whole requirement |
| Declared gap or no test | ok | green at zero, RED above it: a binding obligation that nothing shows Ze meets is owed, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green counts above are what says how much Ze's own tests verify |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Linux cannot do this | neutral | neutral at every value: an evidenced platform limit is neither test verification nor a scope exemption |
| Tests shown to detect a fault | ok | green at every value: these tests have been shown to detect a deliberate fault. This card counts individual tests separately, so its total cannot be added to the requirement counts above |
| Audit verdicts | ok | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a mandatory requirement is unreviewed, green when every one is judged sound and current |
| Mandatory requirements (MUST level) | outlined | outlined, with no colored edge: it is the total the other cards are counted against, not a measure, and a larger one is neither good news nor bad |

## At a glance

| Field | Value |
|---|---|
| Public row | Internet checksum (OSPF, VRRP, ICMP probe, RSVP-TE) |
| Automatic checks | Included |
| Requirements | 9 |
| Mandatory requirements (MUST level) | 6 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 13 |
| Tests linked to requirements | 13 |
| Recorded audit verdicts | 6 |
| Fault test records | 11 |
| Summary | `rfc/short/rfc1071.md` |
| Requirement table | `rfc/requirements/rfc1071.md` |
| RFC text | `rfc/full/rfc1071.txt` |

## Automatic checks

Included in automatic checks: Computing the Internet Checksum (ones-complement 16-bit): eight MUST-level requirements, all met across ze's four checksum implementations (OSPF header/LSA, VRRP, ICMP probe, RSVP-TE). 1-5 (verification: the sum including the checksum folds to 0xffff) and x-1 (OSPF excludes the 8-octet Authentication field) carry positive+negative tags. The arithmetic-shape MUSTs 1-1 (zero the field before computing, store the complement), 1-2 (ones-complement sum with end-around carry), 1-3 (store the bitwise complement of the sum), 1-4 (odd-length zero-pad in the sum only, transmitted length unchanged), 1-6 (fold carries until none remain), and x-2 (AuType2 checksum field is zero) are {single-polarity: positive}, each having no reject path. Tags on internal/plugins/ospf/packet, internal/plugins/ospf/types, internal/core/probe, internal/plugins/vrrp/packet tests.

## Published support status

**Area:** Internet checksum (OSPF, VRRP, ICMP probe, RSVP-TE)

**Test verification:** 6 of 6 MUST proven

**Evidence:** unit 6

**Unmet:** -


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 1 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 5 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 0 | one part of the mandatory requirements |
| Partial test coverage (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level)** | **6** | every mandatory requirement appears in exactly one group above |

**Positive and negative tests (1):** [`RFC1071-1-5`](#rfc1071-1-5)

**Annotated (including scoped evidence) (5):** [`RFC1071-1-1`](#rfc1071-1-1), [`RFC1071-1-2`](#rfc1071-1-2), [`RFC1071-1-3`](#rfc1071-1-3), [`RFC1071-1-4`](#rfc1071-1-4), [`RFC1071-1-6`](#rfc1071-1-6)

## Figures

- **MUST bar**: Ze verified 6
- **Overview bar**: Ze MUST verified 6, Not assessed 3

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | complete |
| MUST rows | 6 |
| MUST rows kept (the MUST bar) | 6 |
| Ze verified | 6 |
| OS verified | 0 |
| Tests awaiting verification | 0 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 0 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 3 |
| SHOULD verified | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD tests awaiting verification | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 3 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze verified means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Tests awaiting verification counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

Source: [data/rfc/rfc1071.json](../../../data/rfc/rfc1071.json).

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

RFC 1071 lists no missing behavior, and every mandatory requirement included in automatic checks has a linked test.

## Test status

A test reads not verified when no record shows it failing after a deliberate fault. Until then, the test has not been shown to detect a fault in the required behavior.

### [`RFC1071-1-1`](#rfc1071-1-1)

To generate a checksum, the checksum field itself is cleared, the 16-bit 1's complement sum is computed over the octets concerned, and the 1's complement of this sum is placed in the checksum field. (§1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged c18. Sentence: clear the field, sum, place the complement. TestRFC1071PacketChecksumClearedSummedAndComplemented encodes a Hello through Packet.WriteTo with a stale header Checksum 0xBEEF into a 0xA5-prefilled buffer and asserts the field equals the test's own RFC 1071 checksum (independent 64-bit fold, field cleared, auth excluded, complemented), equals the same packet encoded from a zero Checksum, and that the covered packet then folds to 0xFFFF. An encoder that skipped the clear (header.go WriteTo h.Checksum = 0) would sum 0xBEEF and go red; one that skipped the complement or placed nothing goes red on the independent value. Record breaks PacketChecksum (the sum/complement producer, checksum.go); the clear and the placement live in Packet.WriteTo, which the record tool cannot name because header.go holds three WriteTo, so their discrimination is argued from the assertions, not recorded. Old TestOSPFPacketChecksum (round-trip only) kept as supplementary. {single-polarity: positive} holds: a generate rule has no reject path; corruption detection is RFC1071-1-5.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestOSPFPacketChecksum`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/checksum_test.go#L14) | unit/verify | revert, verified |
| positive | [`TestRFC1071PacketChecksumClearedSummedAndComplemented`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/rfc1071_checksum_test.go#L60) | unit/verify | revert, verified |

### [`RFC1071-1-2`](#rfc1071-1-2)

On a 2's complement machine, the 1's complement sum must be computed by means of an "end around carry", i.e., any overflows from the most significant bits are added into the least significant bits. (§1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a one's complement sum without end-around carry. Red on it: TestChecksumRFC1071 compares checksum16 with an independent straight-line reference (t.Fatalf on got != want) over inputs whose sums overflow 16 bits (0xDEAD+0xBEEF, the golden VRRP packets). Single-polarity marker on the row.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestChecksumRFC1071`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/packet/checksum_test.go#L30) | unit/verify | mutant, verified |

### [`RFC1071-1-3`](#rfc1071-1-3)

the 16-bit 1's complement sum is computed over the octets concerned, and the 1's complement of this sum is placed in the checksum field. (§1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged c18. Sub-span: compute the 16-bit one's complement sum and place its complement in the checksum field. Placement is now ze's: TestRFC1071PacketChecksumClearedSummedAndComplemented reads back the field Packet.WriteTo wrote and compares it with the complement of the test's independent sum (the 0xFFFF fold of the covered packet pins the complement). The exact vector 0x1411 in TestInternetChecksumRFC1071Vectors still pins the complement in internetChecksum. Record breaks PacketChecksum, observed red. Single-polarity positive valid (no reject path).

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestRFC1071PacketChecksumClearedSummedAndComplemented`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/rfc1071_checksum_test.go#L61) | unit/verify | revert, verified |
| positive | [`TestInternetChecksumRFC1071Vectors`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/types/checksum_test.go#L61) | unit/verify | revert, verified |

### [`RFC1071-1-4`](#rfc1071-1-4)

Using the notation [a,b] for the 16-bit integer a*256+b, where a and b are bytes, then the 16-bit 1's complement sum of these bytes is given by one of the following: [A,B] +' [C,D] +' ... +' [Y,Z] [1] [A,B] +' [C,D] +' ... +' [Z,0] [2] where +' indicates 1's complement addition. These cases correspond to an even or odd count of bytes, respectively. (§1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Both forms of the RFC 1071 Section 1 sentence are now pinned by exact vectors on both producers. Even count [1]: TestRFC1071ChecksumEvenLength (icmpChecksum) and TestInternetChecksumEvenLength (internetChecksum) answer 0x9753 for [12 34 56 78] and 0xfc96 for [12 34 56 78 9a bc]; a little-endian word, a padded even tail or a missing end-around carry (the 6-octet sum 0x10368) each goes red. Odd count [2] [Z,0]: TestInternetChecksumOddLength pins 0x97cb and TestRFC792ChecksumOddLength folds an 11-octet echo to 0xffff. The {single-polarity: positive} holds: the row defines the sum a generator forms and has no reject path.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestRFC792ChecksumOddLength`](https://github.com/ze-software/ze/blob/main/internal/core/probe/icmp_test.go#L254) | unit/verify | not verified |
| positive | [`TestRFC1071ChecksumEvenLength`](https://github.com/ze-software/ze/blob/main/internal/core/probe/rfc1071_even_test.go#L16) | unit/verify | revert, verified |
| positive | [`TestInternetChecksumOddLength`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/types/checksum_test.go#L97) | unit/verify | not verified |
| positive | [`TestInternetChecksumEvenLength`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/types/rfc_checksum_forms_test.go#L16) | unit/verify | revert, verified |

### [`RFC1071-1-5`](#rfc1071-1-5)

To check a checksum, the 1's complement sum is computed over the same set of octets, including the checksum field. If the result is all 1 bits (-0 in 1's complement arithmetic), the check succeeds. (§1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged c18 (was wrong: probe units checked a test helper). Sentence: to check, sum the same octets including the checksum field; all ones succeeds. Positive TestRFC1071VerifyAcceptsAllOnesSum places the TEST's independent checksum, proves in setup the covered fold including the field is 0xFFFF, and ze's VerifyPacketChecksum accepts. Negative TestRFC1071VerifyRefusesSumNotAllOnes, from a verifying base, changes only the checksum field (+1), one covered body octet, or zeroes the field; each is setup-checked as not folding to 0xFFFF and each is refused. The field-only cases catch a verifier that left the field out of the sum or compared against nothing. Both polarities recorded red under a VerifyPacketChecksum break. The probe RFC792 units lost their 1-5 tags (D-15/R1): probe has no verify function. Scope: OSPFv2 packet verify (InternetChecksumPairValid); other ze verifiers (vrrp) are not tagged here.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC1071VerifyRefusesSumNotAllOnes`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/rfc1071_checksum_test.go#L100) | unit/verify | revert, verified |
| positive | [`TestRFC1071VerifyAcceptsAllOnesSum`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/rfc1071_checksum_test.go#L82) | unit/verify | revert, verified |

### [`RFC1071-1-6`](#rfc1071-1-6)

When the sum has been computed, we "fold" the long sum into 16 bits by adding the 16-bit segments. Each 16-bit addition may produce new end-around carries that must be added. (§1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged c18. Sentence: fold the long sum by adding 16-bit segments; each addition may carry again and those carries must be added. TestRFC1071FoldRepeatsUntilNoCarry sums FFFF FFFF 0001 = 0x1FFFF, whose first fold 0x10000 carries again; internetChecksum and InternetChecksumPair must both answer exactly 0xFFFE (a single fold answers 0xFFFF), and internetChecksumValid accepts the data with 0xFFFE appended. Record breaks internetChecksum, observed red. TestInternetChecksumRFC1071Vectors prose narrowed to its one carry (D-15). Single-polarity positive valid: a fold has no reject path.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestInternetChecksumRFC1071Vectors`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/types/checksum_test.go#L62) | unit/verify | revert, verified |
| positive | [`TestRFC1071FoldRepeatsUntilNoCarry`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/types/rfc1071_fold_test.go#L14) | unit/verify | revert, verified |

## Requirement list review

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
| `front` | not stated | 0 | reviewed | not stated |
| `1` | not stated | 2 | reviewed | not stated |
| `3` | not stated | 0 | reviewed | not stated |
| `4` | not stated | 0 | reviewed | not stated |
| `4.1` | not stated | 0 | reviewed | not stated |
| `4.2` | not stated | 0 | reviewed | not stated |
| `4.3` | not stated | 0 | reviewed | not stated |
| `4.4` | not stated | 16 | reviewed | not stated |

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
