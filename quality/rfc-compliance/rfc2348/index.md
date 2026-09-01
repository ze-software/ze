# RFC 2348 - TFTP Blocksize Option

No row in the public ledger. Every requirement this repository extracted from RFC 2348, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 100.0% | 4 of 4 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 4 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 4 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 0.0% | 0 of 4 binding obligations | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 0.0% | 0 of 8 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 5 | of 5 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 1 | of 5 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

The 4 shares marked as a part above are the whole of the 4 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 5 |
| Gated MUST-level | 5 |
| Obligations that bind Ze | 4 |
| Not applicable, so out of scope | 1 |
| Declared gaps | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 8 |
| Tagged units | 8 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
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
| Annotated instead of tested | 1 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **5** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (4):** [`RFC2348-x-1`](#rfc2348-x-1), [`RFC2348-x-3`](#rfc2348-x-3), [`RFC2348-x-4`](#rfc2348-x-4), [`RFC2348-x-5`](#rfc2348-x-5)

**Annotated instead of tested (1):** [`RFC2348-x-2`](#rfc2348-x-2)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC2348-x-1` | Server's acknowledged blksize MUST be less than or equal to the client's requested value (Blocksize Option Specification) | MUST | x | **positive:** `unit/verify` [`TestRFC2348BlksizeAckNotAboveRequest`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2348_blksize_test.go#L13). **negative:** `unit/verify` [`TestRFC2348BlksizeAckNotAboveRequest`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2348_blksize_test.go#L17) |
| `RFC2348-x-2` | Client MUST use the size specified in the OACK, or send ERROR code 8 to terminate (Blocksize Option Specification) | MUST | x | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** This requirement governs the TFTP CLIENT (it MUST use the OACK blocksize or send ERROR 8). Ze ships only a TFTP SERVER (internal/plugins/tftpserver/handler.go) with no TFTP client, so there is no client-side code path that consumes an OACK blocksize or emits ERROR 8. |
| `RFC2348-x-3` | Valid blksize range MUST be 8 to 65464 inclusive (Blocksize Option Specification) | MUST | x | **positive:** `unit/verify` [`TestRFC2348BlksizeRangeEnforced`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2348_blksize_test.go#L52). **negative:** `unit/verify` [`TestRFC2348BlksizeRangeEnforced`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2348_blksize_test.go#L55) |
| `RFC2348-x-4` | A data block shorter than the negotiated blksize signals end of transfer (Blocksize Option Specification) | MUST | x | **positive:** `unit/verify` [`TestTFTPReadLargeFile`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L383). **negative:** `unit/verify` [`TestTFTPReadExact512`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L468) |
| `RFC2348-x-5` | If the transfer size is an exact multiple of the blocksize, an extra zero-length data packet MUST be sent to end the transfer (Blocksize Option Specification) | MUST | x | **positive:** `unit/verify` [`TestTFTPReadExact512`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L464). **negative:** `unit/verify` [`TestTFTPReadLargeFile`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L387) |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC2348-x-2`](#rfc2348-x-2) Client MUST use the size specified in the OACK, or send ERROR code 8 to terminate (Blocksize Option Specification) | no test | no test carries this requirement id; annotated {not-applicable}: This requirement governs the TFTP CLIENT (it MUST use the OACK blocksize or send ERROR 8). Ze ships only a TFTP SERVER (internal/plugins/tftpserver/handler.go) with no TFTP client, so there is no client-side code path that consumes an OACK blocksize or emits ERROR 8. |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC2348-x-1`](#rfc2348-x-1)

Server's acknowledged blksize MUST be less than or equal to the client's requested value (Blocksize Option Specification)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2348BlksizeAckNotAboveRequest`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2348_blksize_test.go#L17) | unit/verify | unproven |
| positive | [`TestRFC2348BlksizeAckNotAboveRequest`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2348_blksize_test.go#L13) | unit/verify | unproven |

### [`RFC2348-x-2`](#rfc2348-x-2)

Client MUST use the size specified in the OACK, or send ERROR code 8 to terminate (Blocksize Option Specification)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2348-x-2, so no unit is bound to it.

### [`RFC2348-x-3`](#rfc2348-x-3)

Valid blksize range MUST be 8 to 65464 inclusive (Blocksize Option Specification)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2348BlksizeRangeEnforced`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2348_blksize_test.go#L55) | unit/verify | unproven |
| positive | [`TestRFC2348BlksizeRangeEnforced`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2348_blksize_test.go#L52) | unit/verify | unproven |

### [`RFC2348-x-4`](#rfc2348-x-4)

A data block shorter than the negotiated blksize signals end of transfer (Blocksize Option Specification)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestTFTPReadExact512`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L468) | unit/verify | unproven |
| positive | [`TestTFTPReadLargeFile`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L383) | unit/verify | unproven |

### [`RFC2348-x-5`](#rfc2348-x-5)

If the transfer size is an exact multiple of the blocksize, an extra zero-length data packet MUST be sent to end the transfer (Blocksize Option Specification)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestTFTPReadLargeFile`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L387) | unit/verify | unproven |
| positive | [`TestTFTPReadExact512`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L464) | unit/verify | unproven |

## Extraction sign-off

No extraction sign-off exists for RFC 2348, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 2348, so its obligations are stated where they were written.
