# RFC 2348 - TFTP Blocksize Option

No row in the public ledger. Every requirement this repository extracted from RFC 2348, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 5 | of 5 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0 | of 5 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0 | of 5 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0 | of 5 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| No test at all | 0 | of 5 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 20 | 20 of 20 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |
| Audit verdicts | 5 | of 5 gated MUSTs judged | 0 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 5 | of 5 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the counts beside it are what says how Ze stands |
| Out of scope | 0 | of 5 gated MUSTs | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as a Ze gap. Scope, not coverage: it is counted in the gated whole the cards on this page add up to |
| Not applicable | 0 | of 5 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as a Ze gap. It is counted in the gated whole the cards add up to |
| Met below Ze | 0 | of 5 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze proven: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 5 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as a Ze gap. It is counted in the gated whole the cards add up to |
| Linux cannot do this | 0 | of 5 gated MUSTs | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as a Ze gap. It is counted in the gated whole the cards add up to and earns no proof credit |

The 9 counts marked as a part above are the whole of the 5 gated MUSTs: they add up to 5. Proven by a recorded break counts TAGGED UNITS, a different population, so it is not one of them.

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
| Public row | No row in the public ledger |
| Enrolment | Enrolled |
| Requirements | 5 |
| Gated MUST-level | 5 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 20 |
| Tagged units | 20 |
| Recorded audit verdicts | 5 |
| Discrimination records | 20 |
| Summary | `rfc/short/rfc2348.md` |
| Requirement shard | `rfc/requirements/rfc2348.md` |
| RFC text | `rfc/full/rfc2348.txt` |

## Enrolment

Enrolled: TFTP Blocksize Option: five MUST-level requirements, all met and tested. RFC2348-x-1 (acknowledged blksize <= requested) both polarities via TestRFC2348BlksizeAckNotAboveRequest: a request within Ze cap (1200) is acked as 1200, a request above the 1468 cap (60000) is acked as 1468 which never exceeds the request (producer handleRRQ min(opts.blksize, blksizeEthernet), handler.go). RFC2348-x-3 (valid range 8..65464) both polarities via TestRFC2348BlksizeRangeEnforced: 512 is acked, while 5 and 70000 are ignored and absent from the OACK (producer parseRRQ n >= blksizeMin && n <= blksizeMax, handler.go). RFC2348-x-4 (a short block ends the transfer) both polarities: TestTFTPReadLargeFile (1500 bytes over 512 ends after a 476-byte short block) and TestTFTPReadExact512 (a full 512-byte block does NOT end, block 2 follows). RFC2348-x-5 (exact multiple -> extra zero-length block) both polarities: TestTFTPReadExact512 (512-byte file -> 512 block + a 0-byte end block) and TestTFTPReadLargeFile (1500 is not a multiple, so no extra zero block). RFC2348-x-2 (the client MUST use the OACK size or send ERROR 8) is met by Ze's TFTP client, the tftp scheme of ze init --from (internal/core/tftp/oack.go grantBlksize, internal/core/fetch/tftp.go): TestRFC2348ClientBlocksize uses a granted 1468 or 8 and answers 1469, 2000 or 7 with ERROR 8. The client also ends on a block shorter than the negotiated size (RFC2348-x-4, TestRFC2348ClientFinalBlockByNegotiatedSize). No SHOULD/MAY requirements are gated.

## What the public ledger says

No row in the public ledger, so its summary declares `| Support | - |` and docs/features/rfc-status.md carries no row for RFC 2348.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 5 | one part of the gated population |
| Annotated (including scoped evidence) | 0 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **5** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (5):** [`RFC2348-x-1`](#rfc2348-x-1), [`RFC2348-x-2`](#rfc2348-x-2), [`RFC2348-x-3`](#rfc2348-x-3), [`RFC2348-x-4`](#rfc2348-x-4), [`RFC2348-x-5`](#rfc2348-x-5)

## Figures

- **MUST bar**: Ze proven 5
- **Overview bar**: Ze MUST proven 5

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Not walked | 0 |
| Mandatory register | complete |
| MUST rows | 5 |
| MUST rows kept (the MUST bar) | 5 |
| Ze proven | 5 |
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

RFC 2348 declares no gap, and every gated MUST it carries has a test bound to it.

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC2348-x-1`](#rfc2348-x-1)

The specified value must be less than or equal to the value specified by the client. (§Blocksize Option Specification)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-30 by an independent auditor: the unit changed only by identifier renames to internal/core/tftp (blksizeEthernet->tftp.BlksizeEthernet, opX->tftp.OpX), values identical; assertions unchanged. Server-side row: the 'above cap' subtest is red when the acked blksize exceeds the 60000 requested, and the within-cap subtest pins 1200 == requested 1200.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2348BlksizeAckNotAboveRequest`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2348_blksize_test.go#L19) | unit/verify | revert, verified |
| positive | [`TestRFC2348BlksizeAckNotAboveRequest`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2348_blksize_test.go#L15) | unit/verify | revert, verified |

### [`RFC2348-x-2`](#rfc2348-x-2)

The client must then either use the size specified in the OACK, or send an ERROR packet, with error code 8, to terminate the transfer. (§Blocksize Option Specification)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRFC2348ClientBlocksize, unchanged apart from the added x-3 tags. Positive: an OACK blksize of 1468 or 8 must draw ACK 0 within tftpTimeout/2, a block of exactly that size is not final, and the file must be exact. Negative: 1469, 2000 and 7 must draw ERROR with code exactly 8 as the first packet, then silence, and the fetch must fail.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2348ClientBlocksize`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc2348_client_test.go#L22) | unit/verify | revert, verified |
| positive | [`TestRFC2348ClientBlocksize`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc2348_client_test.go#L19) | unit/verify | revert, verified |

### [`RFC2348-x-3`](#rfc2348-x-3)

Valid values range between "8" and "65464" octets, inclusive. (§Blocksize Option Specification)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-31: TestRFC2348BlksizeRangeEnforced changed only in its claim comment (producer is now parseRRQ through optionInRange with BlksizeMin/BlksizeMax, which the code confirms); assertions identical. Server: it and TestRFC2348BlksizeBoundsAreInclusive pin 8 and 65464 accepted, 5, 7, 65465 and 70000 absent from the OACK beside an acknowledged tsize. Client TestRFC2348ClientBlocksize unchanged: granted 8 used, 7 draws ERROR 8.

| Polarity | Test | Kind and tier | Proof state |
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

| Polarity | Test | Kind and tier | Proof state |
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

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestTFTPReadLargeFile`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L366) | unit/verify | revert, verified |
| negative | [`TestRFC2348ZeroLengthBlockEndsAnExactMultiple`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2347_2348_transfer_test.go#L244) | unit/verify | revert, verified |
| positive | [`TestTFTPReadExact512`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L441) | unit/verify | revert, verified |
| positive | [`TestRFC2348ZeroLengthBlockEndsAnExactMultiple`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2347_2348_transfer_test.go#L241) | unit/verify | revert, verified |

## Extraction sign-off

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
| `front` | not stated | 4 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `front:3` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Description, not an obligation: the sentence sits in the Proof of Concept section explaining why measured transfer time falls as blocksize rises. 'the data transmitter must wait for an ACK' describes the lock-step behavior RFC 1350 already defines, and imposes nothing on an implementation of this option. | For example, by increasing the blocksize from 512 octets to 1024 octets, not only are the number of data packets halved, but the number of acknowledgement packets is also halved (along with the number of times the data transmitter must wait for an ACK). |
| `front:4` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Boilerplate: the sentence is part of the RFC's Full Copyright Statement and constrains redistribution of the document, not the behavior of a TFTP implementation. | However, this document itself may not be modified in any way, such as by removing the copyright notice or references to the Internet Society or other Internet organizations, except as needed for the purpose of developing Internet standards in which case the procedures for copyrights defined in the Internet Standards process must be followed, or as required to translate it into languages other than English. |

## Superseded

No document obsoletes RFC 2348, so its obligations are stated where they were written.
