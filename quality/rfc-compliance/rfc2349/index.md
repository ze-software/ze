# RFC 2349 - TFTP Timeout Interval and Transfer Size Options

No row in the public ledger. Every requirement this repository extracted from RFC 2349, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 100.0% | 1 of 1 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 1 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 1 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 0.0% | 0 of 1 binding obligations | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 0.0% | 0 of 2 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 4 | of 6 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 3 | of 4 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

The 4 shares marked as a part above are the whole of the 1 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 6 |
| Gated MUST-level | 4 |
| Obligations that bind Ze | 1 |
| Not applicable, so out of scope | 3 |
| Declared gaps | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 2 |
| Tagged units | 2 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc2349.md` |
| Requirement shard | `rfc/requirements/rfc2349.md` |
| RFC text | `rfc/full/rfc2349.txt` |

## Enrolment

Enrolled: TFTP Timeout Interval and Transfer Size Options: four MUST-level requirements. RFC2349-x-3 (tsize in RRQ: client value "0", server returns actual file size in the OACK) is met and tested with both polarities via loopback OACK round-trips in rfc2349_tsize_test.go (producer handleRRQ os.Stat + oackOpts tsize, internal/plugins/tftpserver/handler.go:319-329): a 7-byte file returns tsize "7", a 20-byte file returns "20", and the server never echoes the client placeholder "0". The other three are {not-applicable}: RFC2349-x-1 (timeout acknowledged value matches request) and RFC2349-x-2 (timeout range 1-255) govern the timeout option, which Ze does not implement (parseRRQ recognizes only blksize/tsize/windowsize and ignores timeout per RFC 2347); RFC2349-x-4 (tsize in WRQ echoes the client size) governs write requests, which Ze (a read-only TFTP server) rejects with Illegal-Operation. No SHOULD/MAY requirements are gated.

## What the public ledger says

No row in the public ledger, so its summary declares `| Support | - |` and docs/features/rfc-status.md carries no row for RFC 2349.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 1 | one part of the gated population |
| Annotated instead of tested | 3 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **4** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (1):** [`RFC2349-x-3`](#rfc2349-x-3)

**Annotated instead of tested (3):** [`RFC2349-x-1`](#rfc2349-x-1), [`RFC2349-x-2`](#rfc2349-x-2), [`RFC2349-x-4`](#rfc2349-x-4)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC2349-x-1` | Timeout: server's acknowledged value MUST match the client's requested value exactly (Timeout Interval Option Specification) | MUST | x | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** Ze does not implement the RFC 2349 timeout option. Its RRQ parser recognizes only blksize, tsize, and windowsize (internal/plugins/tftpserver/handler.go:121-131 parseRRQ); a requested timeout option falls through and is ignored per RFC 2347 (an unacknowledged option is treated as never requested, gated as RFC2347-x-3). Ze never places timeout in an OACK, so it has no code path that could acknowledge a mismatched timeout value. |
| `RFC2349-x-2` | Timeout valid range MUST be 1 to 255 seconds inclusive (Timeout Interval Option Specification) | MUST | x | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** Ze does not implement the timeout option (parseRRQ recognizes only blksize/tsize/windowsize, internal/plugins/tftpserver/handler.go:121-131), so it has no timeout value to range-check against 1-255 seconds. |
| `RFC2349-x-3` | Tsize in RRQ: client's value MUST be "0"; server returns actual file size in OACK (Transfer Size Option Specification) | MUST | x | **positive:** `unit/verify` [`TestRFC2349TsizeRRQReturnsActualSize`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2349_tsize_test.go#L32). **negative:** `unit/verify` [`TestRFC2349TsizeRRQReturnsActualSize`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2349_tsize_test.go#L37) |
| `RFC2349-x-4` | Tsize in WRQ: server's OACK value MUST echo the client's specified file size (Transfer Size Option Specification) | MUST | x | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** Ze is a read-only TFTP server: it rejects every write request (WRQ) with an Illegal-Operation error (internal/plugins/tftpserver/handler.go:226-227 "write not supported"; TestTFTPWriteRejected). With no WRQ transfer ever accepted, Ze has no code path that would echo a client-specified file size in a WRQ OACK. |
| `RFC2349-x-5` | If the file is too large for the client (RRQ), it MAY abort with ERROR code 3 (Transfer Size Option Specification) | MAY | x | **positive:** no positive test. **negative:** no negative test |
| `RFC2349-x-6` | If the file is too large for the server (WRQ), it MAY abort with ERROR code 3 (Transfer Size Option Specification) | MAY | x | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC2349-x-1`](#rfc2349-x-1) Timeout: server's acknowledged value MUST match the client's requested value exactly (Timeout Interval Option Specification) | no test | no test carries this requirement id; annotated {not-applicable}: Ze does not implement the RFC 2349 timeout option. Its RRQ parser recognizes only blksize, tsize, and windowsize (internal/plugins/tftpserver/handler.go:121-131 parseRRQ); a requested timeout option falls through and is ignored per RFC 2347 (an unacknowledged option is treated as never requested, gated as RFC2347-x-3). Ze never places timeout in an OACK, so it has no code path that could acknowledge a mismatched timeout value. |
| [`RFC2349-x-2`](#rfc2349-x-2) Timeout valid range MUST be 1 to 255 seconds inclusive (Timeout Interval Option Specification) | no test | no test carries this requirement id; annotated {not-applicable}: Ze does not implement the timeout option (parseRRQ recognizes only blksize/tsize/windowsize, internal/plugins/tftpserver/handler.go:121-131), so it has no timeout value to range-check against 1-255 seconds. |
| [`RFC2349-x-4`](#rfc2349-x-4) Tsize in WRQ: server's OACK value MUST echo the client's specified file size (Transfer Size Option Specification) | no test | no test carries this requirement id; annotated {not-applicable}: Ze is a read-only TFTP server: it rejects every write request (WRQ) with an Illegal-Operation error (internal/plugins/tftpserver/handler.go:226-227 "write not supported"; TestTFTPWriteRejected). With no WRQ transfer ever accepted, Ze has no code path that would echo a client-specified file size in a WRQ OACK. |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC2349-x-1`](#rfc2349-x-1)

Timeout: server's acknowledged value MUST match the client's requested value exactly (Timeout Interval Option Specification)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2349-x-1, so no unit is bound to it.

### [`RFC2349-x-2`](#rfc2349-x-2)

Timeout valid range MUST be 1 to 255 seconds inclusive (Timeout Interval Option Specification)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2349-x-2, so no unit is bound to it.

### [`RFC2349-x-3`](#rfc2349-x-3)

Tsize in RRQ: client's value MUST be "0"; server returns actual file size in OACK (Transfer Size Option Specification)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2349TsizeRRQReturnsActualSize`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2349_tsize_test.go#L37) | unit/verify | unproven |
| positive | [`TestRFC2349TsizeRRQReturnsActualSize`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2349_tsize_test.go#L32) | unit/verify | unproven |

### [`RFC2349-x-4`](#rfc2349-x-4)

Tsize in WRQ: server's OACK value MUST echo the client's specified file size (Transfer Size Option Specification)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2349-x-4, so no unit is bound to it.

## Extraction sign-off

No extraction sign-off exists for RFC 2349, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 2349, so its obligations are stated where they were written.
