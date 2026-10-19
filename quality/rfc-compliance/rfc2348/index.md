# RFC 2348 - TFTP Blocksize Option

No row in the public ledger. Every requirement this repository extracted from RFC 2348, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 80.0% | 4 of 5 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 5 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 5 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 5 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| No test at all | 0.0% | 0 of 5 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 85.7% | 12 of 14 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 5 | of 5 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 1 | of 5 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 20.0% | 1 of 5 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 5 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 5 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

The 8 shares marked as a part above are the whole of the 5 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Public status | No row in the public ledger |
| Enrolment | Enrolled |
| Requirements | 5 |
| Gated MUST-level | 5 |
| Not applicable, so out of scope | 1 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 14 |
| Tagged units | 14 |
| Recorded audit verdicts | 4 |
| Discrimination records | 12 |
| Summary | `rfc/short/rfc2348.md` |
| Requirement shard | `rfc/requirements/rfc2348.md` |
| RFC text | `rfc/full/rfc2348.txt` |

## Enrolment

Enrolled: TFTP Blocksize Option: five MUST-level requirements, four met and tested, one client-side {not-applicable}. RFC2348-x-1 (acknowledged blksize <= requested) both polarities via TestRFC2348BlksizeAckNotAboveRequest: a request within Ze cap (1200) is acked as 1200, a request above the 1468 cap (60000) is acked as 1468 which never exceeds the request (producer handleRRQ min(opts.blksize, blksizeEthernet), handler.go:271-272). RFC2348-x-3 (valid range 8..65464) both polarities via TestRFC2348BlksizeRangeEnforced: 512 is acked, while 5 and 70000 are ignored and absent from the OACK (producer parseRRQ n >= blksizeMin && n <= blksizeMax, handler.go:122-125). RFC2348-x-4 (a short block ends the transfer) both polarities: TestTFTPReadLargeFile (1500 bytes over 512 ends after a 476-byte short block) and TestTFTPReadExact512 (a full 512-byte block does NOT end, block 2 follows). RFC2348-x-5 (exact multiple -> extra zero-length block) both polarities: TestTFTPReadExact512 (512-byte file -> 512 block + a 0-byte end block) and TestTFTPReadLargeFile (1500 is not a multiple, so no extra zero block). RFC2348-x-2 (client MUST use the OACK size or send ERROR 8) is {not-applicable}: Ze ships only a TFTP server, no client. No SHOULD/MAY requirements are gated.

## What the public ledger says

No row in the public ledger, so its summary declares `| Support | - |` and docs/features/rfc-status.md carries no row for RFC 2348.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 4 | one part of the gated population |
| Annotated (including scoped evidence) | 1 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **5** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (4):** [`RFC2348-x-1`](#rfc2348-x-1), [`RFC2348-x-3`](#rfc2348-x-3), [`RFC2348-x-4`](#rfc2348-x-4), [`RFC2348-x-5`](#rfc2348-x-5)

**Annotated (including scoped evidence) (1):** [`RFC2348-x-2`](#rfc2348-x-2)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC2348-x-1` | The specified value must be less than or equal to the value specified by the client. (§Blocksize Option Specification) | MUST | Blocksize | **positive:** `unit/verify` [`TestRFC2348BlksizeAckNotAboveRequest`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2348_blksize_test.go#L13). **negative:** `unit/verify` [`TestRFC2348BlksizeAckNotAboveRequest`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2348_blksize_test.go#L17) |
| `RFC2348-x-2` | The client must then either use the size specified in the OACK, or send an ERROR packet, with error code 8, to terminate the transfer. (§Blocksize Option Specification) | MUST | Blocksize | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** This requirement governs the TFTP CLIENT (it MUST use the OACK blocksize or send ERROR 8). Ze ships only a TFTP SERVER (internal/plugins/tftpserver/handler.go) with no TFTP client, so there is no client-side code path that consumes an OACK blocksize or emits ERROR 8. |
| `RFC2348-x-3` | Valid values range between "8" and "65464" octets, inclusive. (§Blocksize Option Specification) | MUST | Blocksize | **positive:** `unit/verify` [`TestRFC2348BlksizeBoundsAreInclusive`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2347_2348_transfer_test.go#L174). **positive:** `unit/verify` [`TestRFC2348BlksizeRangeEnforced`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2348_blksize_test.go#L52). **negative:** `unit/verify` [`TestRFC2348BlksizeBoundsAreInclusive`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2347_2348_transfer_test.go#L177). **negative:** `unit/verify` [`TestRFC2348BlksizeRangeEnforced`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2348_blksize_test.go#L55) |
| `RFC2348-x-4` | The reception of a data packet with a data length less than the negotiated blocksize is the final packet. (§Blocksize Option Specification) | MUST | Blocksize | **positive:** `unit/verify` [`TestRFC2348ShortBlockIsTheFinalPacket`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2347_2348_transfer_test.go#L210). **positive:** `unit/verify` [`TestTFTPReadLargeFile`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L380). **negative:** `unit/verify` [`TestRFC2348ShortBlockIsTheFinalPacket`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2347_2348_transfer_test.go#L214). **negative:** `unit/verify` [`TestTFTPReadExact512`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L463) |
| `RFC2348-x-5` | If the amount of data to be transfered is an integral multiple of the blocksize, an extra data packet containing no data is sent to end the transfer. (§Blocksize Option Specification) | MUST | Blocksize | **positive:** `unit/verify` [`TestRFC2348ZeroLengthBlockEndsAnExactMultiple`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2347_2348_transfer_test.go#L234). **positive:** `unit/verify` [`TestTFTPReadExact512`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L459). **negative:** `unit/verify` [`TestRFC2348ZeroLengthBlockEndsAnExactMultiple`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2347_2348_transfer_test.go#L237). **negative:** `unit/verify` [`TestTFTPReadLargeFile`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L384) |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC2348-x-2`](#rfc2348-x-2) The client must then either use the size specified in the OACK, or send an ERROR packet, with error code 8, to terminate the transfer. (§Blocksize Option Specification) | no test | no test carries this requirement id; annotated {not-applicable}: This requirement governs the TFTP CLIENT (it MUST use the OACK blocksize or send ERROR 8). Ze ships only a TFTP SERVER (internal/plugins/tftpserver/handler.go) with no TFTP client, so there is no client-side code path that consumes an OACK blocksize or emits ERROR 8. |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC2348-x-1`](#rfc2348-x-1)

The specified value must be less than or equal to the value specified by the client. (§Blocksize Option Specification)

Audit verdict: enforced (the tests do what the requirement demands), fresh. (a) OACK blksize above the client's request; (b) 'above cap' subtest t.Errorf when acked > 60000, and the within-cap subtest pins 1200 == requested 1200, so an ack that ignores a smaller request also goes red.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2348BlksizeAckNotAboveRequest`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2348_blksize_test.go#L17) | unit/verify | unproven |
| positive | [`TestRFC2348BlksizeAckNotAboveRequest`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2348_blksize_test.go#L13) | unit/verify | unproven |

### [`RFC2348-x-2`](#rfc2348-x-2)

The client must then either use the size specified in the OACK, or send an ERROR packet, with error code 8, to terminate the transfer. (§Blocksize Option Specification)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2348-x-2, so no unit is bound to it.

### [`RFC2348-x-3`](#rfc2348-x-3)

Valid values range between "8" and "65464" octets, inclusive. (§Blocksize Option Specification)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRFC2348BlksizeBoundsAreInclusive asserts both inclusive bounds and one past each: 8 is acked as 8 and block 1 carries 8 octets; 65464 is acked (as the 1468 cap, which x-1 allows), red on n<65464; 7 and 65465 beside tsize are absent from the OACK, red on n>=7 or n<=65465. Every off-by-one at either bound goes red.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2348BlksizeBoundsAreInclusive`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2347_2348_transfer_test.go#L177) | unit/verify | revert, verified |
| negative | [`TestRFC2348BlksizeRangeEnforced`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2348_blksize_test.go#L55) | unit/verify | revert, verified |
| positive | [`TestRFC2348BlksizeBoundsAreInclusive`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2347_2348_transfer_test.go#L174) | unit/verify | revert, verified |
| positive | [`TestRFC2348BlksizeRangeEnforced`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2348_blksize_test.go#L52) | unit/verify | revert, verified |

### [`RFC2348-x-4`](#rfc2348-x-4)

The reception of a data packet with a data length less than the negotiated blocksize is the final packet. (§Blocksize Option Specification)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-30: TestTFTPReadLargeFile changed only by losing the retired RFC1350-5-2 tag comment; every assertion is unchanged. TestRFC2348ShortBlockIsTheFinalPacket at a negotiated 1024: a 600-octet file must be exactly [600] with silence after its ACK (a server comparing against 512 sends a block 2 and goes red), and 2500 must be [1024,1024,452] then silence (red on DATA after the short block). Negative: full 1024 blocks are followed by the next block.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestTFTPReadExact512`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L463) | unit/verify | revert, verified |
| negative | [`TestRFC2348ShortBlockIsTheFinalPacket`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2347_2348_transfer_test.go#L214) | unit/verify | revert, verified |
| positive | [`TestTFTPReadLargeFile`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L380) | unit/verify | revert, verified |
| positive | [`TestRFC2348ShortBlockIsTheFinalPacket`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2347_2348_transfer_test.go#L210) | unit/verify | revert, verified |

### [`RFC2348-x-5`](#rfc2348-x-5)

If the amount of data to be transfered is an integral multiple of the blocksize, an extra data packet containing no data is sent to end the transfer. (§Blocksize Option Specification)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-30: TestTFTPReadLargeFile changed only by losing the retired RFC1350-5-2 tag comment; every assertion is unchanged. TestRFC2348ZeroLengthBlockEndsAnExactMultiple at a negotiated 1024: 2048 must be [1024,1024,0] (red with no empty terminator), and 1500 must be [1024,476] and 512 must be [512], each followed by 1 s of silence (red on an extra empty packet, including one computed against 512 instead of the negotiated size).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestTFTPReadLargeFile`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L384) | unit/verify | revert, verified |
| negative | [`TestRFC2348ZeroLengthBlockEndsAnExactMultiple`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2347_2348_transfer_test.go#L237) | unit/verify | revert, verified |
| positive | [`TestTFTPReadExact512`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L459) | unit/verify | revert, verified |
| positive | [`TestRFC2348ZeroLengthBlockEndsAnExactMultiple`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2347_2348_transfer_test.go#L234) | unit/verify | revert, verified |

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
