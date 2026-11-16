# RFC 2348 - TFTP Blocksize Option

No entry in the public support list. Every requirement listed for RFC 2348, the tests linked to it, and their recorded reviews. This summary is included in the automatic compliance check, ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 5 | of 5 mandatory requirements | a positive test checks that Ze does what the requirement demands and a negative one checks that it refuses what the requirement forbids |
| One polarity plus reason | 0 | of 5 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason supplies the complete test coverage required |
| One polarity, unexcused | 0 | of 5 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial test coverage | 0 | of 5 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unverified, with zero whole-requirement credit |
| Declared gap or no test | 0 | of 5 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |
| Tests shown to detect a fault | 20 | 20 of 20 tests linked to requirements | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests verifies a requirement only after both tests have been shown to detect a fault |
| Audit verdicts | 5 | of 5 mandatory requirements reviewed | 0 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 5 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 0 | of 5 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 0 | of 5 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze verified: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 5 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 5 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no test verification credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level) | 5 | of 5 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements included in the automatic checks: the compliance check (./le rfc check) holds Ze to each of them. This is the total being assessed; the counts above show the recorded status of those requirements. |

The 9 counts marked as a part above are the whole of the 5 mandatory requirements: they add up to 5. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

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
| Public row | No entry in the public support list |
| Automatic checks | Included |
| Requirements | 5 |
| Mandatory requirements (MUST level) | 5 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 20 |
| Tests linked to requirements | 20 |
| Recorded audit verdicts | 5 |
| Fault test records | 20 |
| Summary | `rfc/short/rfc2348.md` |
| Requirement table | `rfc/requirements/rfc2348.md` |
| RFC text | `rfc/full/rfc2348.txt` |

## Automatic checks

Included in automatic checks: TFTP Blocksize Option: five MUST-level requirements, all met and tested. RFC2348-x-1 (acknowledged blksize <= requested) both polarities via TestRFC2348BlksizeAckNotAboveRequest: a request within Ze cap (1200) is acked as 1200, a request above the 1468 cap (60000) is acked as 1468 which never exceeds the request (producer handleRRQ min(opts.blksize, blksizeEthernet), handler.go). RFC2348-x-3 (valid range 8..65464) both polarities via TestRFC2348BlksizeRangeEnforced: 512 is acked, while 5 and 70000 are ignored and absent from the OACK (producer parseRRQ n >= blksizeMin && n <= blksizeMax, handler.go). RFC2348-x-4 (a short block ends the transfer) both polarities: TestTFTPReadLargeFile (1500 bytes over 512 ends after a 476-byte short block) and TestTFTPReadExact512 (a full 512-byte block does NOT end, block 2 follows). RFC2348-x-5 (exact multiple -> extra zero-length block) both polarities: TestTFTPReadExact512 (512-byte file -> 512 block + a 0-byte end block) and TestTFTPReadLargeFile (1500 is not a multiple, so no extra zero block). RFC2348-x-2 (the client MUST use the OACK size or send ERROR 8) is met by Ze's TFTP client, the tftp scheme of ze init --from (internal/core/tftp/oack.go grantBlksize, internal/core/fetch/tftp.go): TestRFC2348ClientBlocksize uses a granted 1468 or 8 and answers 1469, 2000 or 7 with ERROR 8. The client also ends on a block shorter than the negotiated size (RFC2348-x-4, TestRFC2348ClientFinalBlockByNegotiatedSize). No SHOULD/MAY requirements are gated.

## Published support status

No entry in the public support list, so its summary declares `| Support | - |` and docs/features/rfc-status.md carries no row for RFC 2348.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 5 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 0 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 0 | one part of the mandatory requirements |
| Partial test coverage (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level)** | **5** | every mandatory requirement appears in exactly one group above |

**Positive and negative tests (5):** [`RFC2348-x-1`](#rfc2348-x-1), [`RFC2348-x-2`](#rfc2348-x-2), [`RFC2348-x-3`](#rfc2348-x-3), [`RFC2348-x-4`](#rfc2348-x-4), [`RFC2348-x-5`](#rfc2348-x-5)

## Figures

- **MUST bar**: Ze verified 5
- **Overview bar**: Ze MUST verified 5

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | complete |
| MUST rows | 5 |
| MUST rows kept (the MUST bar) | 5 |
| Ze verified | 5 |
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
| SHOULD / MAY rows | 0 |
| SHOULD verified | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD tests awaiting verification | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 0 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze verified means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Tests awaiting verification counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

Source: [data/rfc/rfc2348.json](../../../data/rfc/rfc2348.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC2348-x-1` | The specified value must be less than or equal to the value specified by the client. (§Blocksize Option Specification) | MUST | Blocksize | **positive:** `unit/verify` [`TestRFC2348BlksizeAckNotAboveRequest`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2348_blksize_test.go#L15). **negative:** `unit/verify` [`TestRFC2348BlksizeAckNotAboveRequest`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2348_blksize_test.go#L19) |
| `RFC2348-x-2` | The client must then either use the size specified in the OACK, or send an ERROR packet, with error code 8, to terminate the transfer. (§Blocksize Option Specification) | MUST | Blocksize | **positive:** `unit/verify` [`TestRFC2348ClientBlocksize`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc2348_client_test.go#L19). **negative:** `unit/verify` [`TestRFC2348ClientBlocksize`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc2348_client_test.go#L22) |
| `RFC2348-x-3` | Valid values range between "8" and "65464" octets, inclusive. (§Blocksize Option Specification) | MUST | Blocksize | **positive:** `unit/verify` [`TestRFC2348BlksizeBoundsAreInclusive`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2347_2348_transfer_test.go#L181). **positive:** `unit/verify` [`TestRFC2348BlksizeRangeEnforced`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2348_blksize_test.go#L57). **positive:** `unit/verify` [`TestRFC2348ClientBlocksize`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc2348_client_test.go#L25). **negative:** `unit/verify` [`TestRFC2348BlksizeBoundsAreInclusive`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2347_2348_transfer_test.go#L184). **negative:** `unit/verify` [`TestRFC2348BlksizeRangeEnforced`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2348_blksize_test.go#L60). **negative:** `unit/verify` [`TestRFC2348ClientBlocksize`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc2348_client_test.go#L27) |
| `RFC2348-x-4` | The reception of a data packet with a data length less than the negotiated blocksize is the final packet. (§Blocksize Option Specification) | MUST | Blocksize | **positive:** `unit/verify` [`TestRFC2348ClientFinalBlockByNegotiatedSize`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc2348_client_test.go#L74). **positive:** `unit/verify` [`TestRFC2348ShortBlockIsTheFinalPacket`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2347_2348_transfer_test.go#L217). **positive:** `unit/verify` [`TestTFTPReadLargeFile`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L361). **negative:** `unit/verify` [`TestRFC2348ClientFinalBlockByNegotiatedSize`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc2348_client_test.go#L77). **negative:** `unit/verify` [`TestRFC2348ShortBlockIsTheFinalPacket`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2347_2348_transfer_test.go#L221). **negative:** `unit/verify` [`TestTFTPReadExact512`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L446) |
| `RFC2348-x-5` | If the amount of data to be transfered is an integral multiple of the blocksize, an extra data packet containing no data is sent to end the transfer. (§Blocksize Option Specification) | MUST | Blocksize | **positive:** `unit/verify` [`TestRFC2348ZeroLengthBlockEndsAnExactMultiple`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2347_2348_transfer_test.go#L241). **positive:** `unit/verify` [`TestTFTPReadExact512`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L441). **negative:** `unit/verify` [`TestRFC2348ZeroLengthBlockEndsAnExactMultiple`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2347_2348_transfer_test.go#L244). **negative:** `unit/verify` [`TestTFTPReadLargeFile`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L366) |

## Gaps and untested MUSTs

RFC 2348 lists no missing behavior, and every mandatory requirement included in automatic checks has a linked test.

## Test status

A test reads not verified when no record shows it failing after a deliberate fault. Until then, the test has not been shown to detect a fault in the required behavior.

### [`RFC2348-x-1`](#rfc2348-x-1)

The specified value must be less than or equal to the value specified by the client. (§Blocksize Option Specification)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-30 by an independent auditor: the unit changed only by identifier renames to internal/core/tftp (blksizeEthernet->tftp.BlksizeEthernet, opX->tftp.OpX), values identical; assertions unchanged. Server-side row: the 'above cap' subtest is red when the acked blksize exceeds the 60000 requested, and the within-cap subtest pins 1200 == requested 1200.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC2348BlksizeAckNotAboveRequest`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2348_blksize_test.go#L19) | unit/verify | revert, verified |
| positive | [`TestRFC2348BlksizeAckNotAboveRequest`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2348_blksize_test.go#L15) | unit/verify | revert, verified |

### [`RFC2348-x-2`](#rfc2348-x-2)

The client must then either use the size specified in the OACK, or send an ERROR packet, with error code 8, to terminate the transfer. (§Blocksize Option Specification)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRFC2348ClientBlocksize, unchanged apart from the added x-3 tags. Positive: an OACK blksize of 1468 or 8 must draw ACK 0 within tftpTimeout/2, a block of exactly that size is not final, and the file must be exact. Negative: 1469, 2000 and 7 must draw ERROR with code exactly 8 as the first packet, then silence, and the fetch must fail.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC2348ClientBlocksize`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc2348_client_test.go#L22) | unit/verify | revert, verified |
| positive | [`TestRFC2348ClientBlocksize`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc2348_client_test.go#L19) | unit/verify | revert, verified |

### [`RFC2348-x-3`](#rfc2348-x-3)

Valid values range between "8" and "65464" octets, inclusive. (§Blocksize Option Specification)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-31: TestRFC2348BlksizeRangeEnforced changed only in its claim comment (producer is now parseRRQ through optionInRange with BlksizeMin/BlksizeMax, which the code confirms); assertions identical. Server: it and TestRFC2348BlksizeBoundsAreInclusive pin 8 and 65464 accepted, 5, 7, 65465 and 70000 absent from the OACK beside an acknowledged tsize. Client TestRFC2348ClientBlocksize unchanged: granted 8 used, 7 draws ERROR 8.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC2348ClientBlocksize`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc2348_client_test.go#L27) | unit/verify | revert, verified |
| negative | [`TestRFC2348BlksizeBoundsAreInclusive`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2347_2348_transfer_test.go#L184) | unit/verify | revert, verified |
| negative | [`TestRFC2348BlksizeRangeEnforced`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2348_blksize_test.go#L60) | unit/verify | revert, verified |
| positive | [`TestRFC2348ClientBlocksize`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc2348_client_test.go#L25) | unit/verify | revert, verified |
| positive | [`TestRFC2348BlksizeBoundsAreInclusive`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2347_2348_transfer_test.go#L181) | unit/verify | revert, verified |
| positive | [`TestRFC2348BlksizeRangeEnforced`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2348_blksize_test.go#L57) | unit/verify | revert, verified |

### [`RFC2348-x-4`](#rfc2348-x-4)

The reception of a data packet with a data length less than the negotiated blocksize is the final packet. (§Blocksize Option Specification)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-31: TestTFTPReadLargeFile and TestTFTPReadExact512 changed only in claim comments (producer now windowSender.sendWindow, which returns final when n < blksize); assertions identical. Server TestRFC2348ShortBlockIsTheFinalPacket at negotiated blksize; client TestRFC2348ClientFinalBlockByNegotiatedSize unchanged (1024 not final, 1023 final, red against 512 or 1468).

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC2348ClientFinalBlockByNegotiatedSize`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc2348_client_test.go#L77) | unit/verify | revert, verified |
| negative | [`TestTFTPReadExact512`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L446) | unit/verify | revert, verified |
| negative | [`TestRFC2348ShortBlockIsTheFinalPacket`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2347_2348_transfer_test.go#L221) | unit/verify | revert, verified |
| positive | [`TestRFC2348ClientFinalBlockByNegotiatedSize`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc2348_client_test.go#L74) | unit/verify | revert, verified |
| positive | [`TestTFTPReadLargeFile`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L361) | unit/verify | revert, verified |
| positive | [`TestRFC2348ShortBlockIsTheFinalPacket`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2347_2348_transfer_test.go#L217) | unit/verify | revert, verified |

### [`RFC2348-x-5`](#rfc2348-x-5)

If the amount of data to be transfered is an integral multiple of the blocksize, an extra data packet containing no data is sent to end the transfer. (§Blocksize Option Specification)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-31: TestTFTPReadLargeFile and TestTFTPReadExact512 changed only in claim comments; assertions identical: 1500 bytes end on the short block with no extra empty one, 512 bytes end with a zero-length block 2 (readBlock gives an empty block past EOF). Sender-only row. TestRFC2348ZeroLengthBlockEndsAnExactMultiple at 1024: 2048 is [1024,1024,0], 1500 [1024,476], 512 [512], each followed by silence.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestTFTPReadLargeFile`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L366) | unit/verify | revert, verified |
| negative | [`TestRFC2348ZeroLengthBlockEndsAnExactMultiple`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2347_2348_transfer_test.go#L244) | unit/verify | revert, verified |
| positive | [`TestTFTPReadExact512`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L441) | unit/verify | revert, verified |
| positive | [`TestRFC2348ZeroLengthBlockEndsAnExactMultiple`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2347_2348_transfer_test.go#L241) | unit/verify | revert, verified |

## Requirement list review

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | prose |
| Source | rfc/full/rfc2348.txt |
| Source fingerprint | 5603bbee0abe1b05 |
| Record | rfc/extraction/rfc2348.json |
| Mapped sentences | 2 |
| Declined as scope | 2 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 4 | reviewed | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `front:3` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Description, not an obligation: the sentence sits in the Proof of Concept section explaining why measured transfer time falls as blocksize rises. 'the data transmitter must wait for an ACK' describes the lock-step behavior RFC 1350 already defines, and imposes nothing on an implementation of this option. | For example, by increasing the blocksize from 512 octets to 1024 octets, not only are the number of data packets halved, but the number of acknowledgement packets is also halved (along with the number of times the data transmitter must wait for an ACK). |
| `front:4` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Boilerplate: the sentence is part of the RFC's Full Copyright Statement and constrains redistribution of the document, not the behavior of a TFTP implementation. | However, this document itself may not be modified in any way, such as by removing the copyright notice or references to the Internet Society or other Internet organizations, except as needed for the purpose of developing Internet standards in which case the procedures for copyrights defined in the Internet Standards process must be followed, or as required to translate it into languages other than English. |

## Superseded

No document obsoletes RFC 2348, so its obligations are stated where they were written.
