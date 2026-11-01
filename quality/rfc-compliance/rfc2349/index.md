# RFC 2349 - TFTP Timeout Interval and Transfer Size Options

TFTP timeout and transfer size options. Every requirement this repository extracted from RFC 2349, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 3 | of 4 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0 | of 4 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0 | of 4 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0 | of 4 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Proven by a recorded break | 10 | 10 of 10 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |
| Audit verdicts | 4 | of 4 gated MUSTs judged | 0 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 4 | of 6 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the counts beside it are what says how Ze stands |
| Out of scope | 0 | of 4 gated MUSTs | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as a Ze gap. Scope, not coverage: it is counted in the gated whole the cards on this page add up to |
| Not applicable | 0 | of 4 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as a Ze gap. It is counted in the gated whole the cards add up to |
| Met below Ze | 0 | of 4 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze proven: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 4 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as a Ze gap. It is counted in the gated whole the cards add up to |
| Linux cannot do this | 0 | of 4 gated MUSTs | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as a Ze gap. It is counted in the gated whole the cards add up to and earns no proof credit |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 1 | of 4 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

The 9 counts marked as a part above are the whole of the 4 gated MUSTs: they add up to 4. Proven by a recorded break counts TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the count under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| Partial proof; remaining gap | ok | green at zero, RED above it: a tested clause cannot prove the whole requirement |
| No test at all | bad | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green counts above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Linux cannot do this | neutral | neutral at every value: an evidenced platform limit is neither a proof nor a scope exemption |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. It counts TAGGED UNITS, not obligations, so this count is not one of the parts above |
| Audit verdicts | ok | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public row | TFTP timeout and transfer size options |
| Enrolment | Enrolled |
| Requirements | 6 |
| Gated MUST-level | 4 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 1 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 10 |
| Tagged units | 10 |
| Recorded audit verdicts | 4 |
| Discrimination records | 10 |
| Summary | `rfc/short/rfc2349.md` |
| Requirement shard | `rfc/requirements/rfc2349.md` |
| RFC text | `rfc/full/rfc2349.txt` |

## Enrolment

Enrolled: TFTP Timeout Interval and Transfer Size Options: four MUST-level requirements. RFC2349-x-3 (tsize in RRQ: client value "0", server returns actual file size in the OACK) is met and tested with both polarities via loopback OACK round-trips in rfc2349_tsize_test.go (producer sendOACKAndWait os.Stat + oackOpts tsize, internal/plugins/tftpserver/handler.go): a 7-byte file returns tsize "7", a 20-byte file returns "20", and the server never echoes the client placeholder "0". RFC2349-x-1 (the acknowledged timeout matches the request) and RFC2349-x-2 (timeout range 1-255) are met by the server's timeout option and tested with both polarities in rfc2349_timeout_test.go (producers parseRRQ, optionInRange, handleRRQ and sendOACKAndWait, internal/plugins/tftpserver/handler.go): timeout 1 and 255 are echoed unchanged, timeout 1 becomes the retransmission interval, and 0, 256, abc, +3 and the empty value are not acknowledged and leave the configured interval; curl 8.23.0's timeout is echoed in TestTFTPCurlTimeoutInterop. The one gap left, with no owner decision declining the feature, is RFC2349-x-4 (tsize in WRQ echoes the client size): it governs write requests, which Ze (a read-only TFTP server) rejects with Illegal-Operation. On 2026-10-31 Thomas put TFTP uploads (write requests) in scope for Ze's TFTP server, so x-4 is owed work, owned by plan/post-release/features/spec-tftp-server-uploads.md together with the WRQ side of x-1, x-2 and x-6. No SHOULD/MAY requirements are gated. The client side of RFC2349-x-3 (the read request carries tsize "0" and the OACK size is taken) is tested by Ze's TFTP client, TestRFC2349ClientTsize (internal/core/tftp/request.go BuildRRQ, internal/core/tftp/oack.go grantTsize), and the MAY x-5 (a file too large for the client draws ERROR 3) by TestRFC2349ClientAbortsTooLargeFile.

## What the public ledger says

**Area:** TFTP timeout and transfer size options

**Proof:** 3 of 4 MUST proven

**Evidence:** unit 3

**Unmet:** RFC2349-x-4


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 3 | one part of the gated population |
| Annotated (including scoped evidence) | 1 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **4** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (3):** [`RFC2349-x-1`](#rfc2349-x-1), [`RFC2349-x-2`](#rfc2349-x-2), [`RFC2349-x-3`](#rfc2349-x-3)

**Annotated (including scoped evidence) (1):** [`RFC2349-x-4`](#rfc2349-x-4)

## Figures

- **MUST bar**: Ze proven 3, Ze correctness gap 1
- **Overview bar**: Ze MUST proven 3, SHOULD proven 1, SHOULD not decided 1, Ze correctness gap 1

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Not walked | 0 |
| Mandatory register | not complete |
| MUST rows | 4 |
| MUST rows kept (the MUST bar) | 4 |
| Ze proven | 3 |
| OS verified | 0 |
| OS testing gap | 0 |
| Ze correctness gap | 1 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 2 |
| SHOULD proven | 1 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD not ever | 0 |
| SHOULD not yet | 0 |
| SHOULD not decided | 1 |

The MUST bar counts the MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT rows of the walked RFCs that the stack owes: Ze proven means a recorded test break proves the row; a row a layer under Ze performs is OS verified when a test checks that layer on the state Ze installs, an OS testing gap until one does. The overview bar adds the MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT rows and the rows not owed, which do not bind Ze. An RFC that is not walked has no valid extraction sign-off, so its rows count in no figure. Every count is read from the RFC data files when the page loads.

Source: [data/rfc/rfc2349.json](../../../data/rfc/rfc2349.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC2349-x-1` | If the server is willing to accept the timeout option, it sends an Option Acknowledgment (OACK) to the client. The specified timeout value must match the value specified by the client. (§Timeout Interval Option Specification) | MUST | Timeout | **positive:** `unit/verify` [`TestRFC2349ServerAcknowledgesTimeout`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2349_timeout_test.go#L16). **negative:** `unit/verify` [`TestRFC2349ServerAcknowledgesTimeout`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2349_timeout_test.go#L20) |
| `RFC2349-x-2` | The number of seconds to wait before retransmitting, specified in ASCII. Valid values range between "1" and "255" seconds, inclusive. (§Timeout Interval Option Specification) | MUST | Timeout | **positive:** `unit/verify` [`TestRFC2349ServerDeclinesTimeoutOutOfRange`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2349_timeout_test.go#L55). **negative:** `unit/verify` [`TestRFC2349ServerDeclinesTimeoutOutOfRange`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2349_timeout_test.go#L57) |
| `RFC2349-x-3` | In Read Request packets, a size of "0" is specified in the request and the size of the file, in octets, is returned in the OACK. (§Transfer Size Option Specification) | MUST | Transfer | **positive:** `unit/verify` [`TestRFC2349ClientTsize`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc2349_client_test.go#L17). **positive:** `unit/verify` [`TestRFC2349TsizeRRQReturnsActualSize`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2349_tsize_test.go#L32). **negative:** `unit/verify` [`TestRFC2349ClientTsize`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc2349_client_test.go#L20). **negative:** `unit/verify` [`TestRFC2349TsizeRRQReturnsActualSize`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2349_tsize_test.go#L37) |
| `RFC2349-x-4` | In Write Request packets, the size of the file, in octets, is specified in the request and echoed back in the OACK. (§Transfer Size Option Specification) | MUST | Transfer | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze serves no write request and no owner decision declines writes: internal/plugins/tftpserver/handler.go answers every WRQ with an Illegal Operation error (write not supported, TestTFTPWriteRejected), so no WRQ tsize is echoed in an OACK |
| `RFC2349-x-5` | If the file is too large for the client to handle, it may abort the transfer with an Error packet (error code 3). (§Transfer Size Option Specification) | MAY | Transfer | **positive:** `unit/verify` [`TestRFC2349ClientAbortsTooLargeFile`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc2349_client_test.go#L61). **negative:** `unit/verify` [`TestRFC2349ClientAbortsTooLargeFile`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc2349_client_test.go#L63) |
| `RFC2349-x-6` | If the file is too large for the server to handle, it may abort the transfer with an Error packet (error code 3). (§Transfer Size Option Specification) | MAY | Transfer | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC2349-x-4`](#rfc2349-x-4) In Write Request packets, the size of the file, in octets, is specified in the request and echoed back in the OACK. (§Transfer Size Option Specification) | {gap}, no test | ze serves no write request and no owner decision declines writes: internal/plugins/tftpserver/handler.go answers every WRQ with an Illegal Operation error (write not supported, TestTFTPWriteRejected), so no WRQ tsize is echoed in an OACK |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC2349-x-1`](#rfc2349-x-1)

If the server is willing to accept the timeout option, it sends an Option Acknowledgment (OACK) to the client. The specified timeout value must match the value specified by the client. (§Timeout Interval Option Specification)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RFC 2349 Timeout Interval: 'If the server is willing to accept the timeout option, it sends an Option Acknowledgment (OACK) to the client. The specified timeout value must match the value specified by the client.' TestRFC2349ServerAcknowledgesTimeout: timeout 1 and 255 must come back unchanged in the OACK (red on any counter-proposal or clamp, e.g. a cap below 255), and with timeout 1 an unacked DATA 1 must be resent within 3 s yet not before 0.8 s, against a 5 s default, so acknowledging a value and then using another is red. Producer: sendOACKAndWait echoes t.opts.timeout; handleRRQ sets the interval from it.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2349ServerAcknowledgesTimeout`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2349_timeout_test.go#L20) | unit/verify | revert, verified |
| positive | [`TestRFC2349ServerAcknowledgesTimeout`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2349_timeout_test.go#L16) | unit/verify | revert, verified |

### [`RFC2349-x-2`](#rfc2349-x-2)

The number of seconds to wait before retransmitting, specified in ASCII. Valid values range between "1" and "255" seconds, inclusive. (§Timeout Interval Option Specification)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RFC 2349: '#secs ... specified in ASCII. Valid values range between "1" and "255" seconds, inclusive.' TestRFC2349ServerDeclinesTimeoutOutOfRange: the bounds 1 and 255 are acknowledged beside tsize; 0, 256, abc, +3 and the empty value each draw an OACK with tsize 100 and no timeout (isolated: the OACK still exists, so the option alone was declined); declined 0 is not used as the interval (no resend within 2 s), and timeout 0 alone draws DATA 1 with no OACK. Off-by-one at either bound or accepting a signed or non-decimal value is red. Producer parseRRQ via optionInRange(TimeoutMin, TimeoutMax).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2349ServerDeclinesTimeoutOutOfRange`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2349_timeout_test.go#L57) | unit/verify | revert, verified |
| positive | [`TestRFC2349ServerDeclinesTimeoutOutOfRange`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2349_timeout_test.go#L55) | unit/verify | revert, verified |

### [`RFC2349-x-3`](#rfc2349-x-3)

In Read Request packets, a size of "0" is specified in the request and the size of the file, in octets, is returned in the OACK. (§Transfer Size Option Specification)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-31: TestRFC2349TsizeRRQReturnsActualSize changed only in its claim comment (producer named as sendOACKAndWait, where os.Stat and the tsize entry now live, which the code confirms); assertions identical: tsize 7 and 20 for 7- and 20-octet files, red on an echo of "0" or a constant. Client TestRFC2349ClientTsize unchanged (whole RRQ byte for byte with tsize 0; '12k' refused with ERROR 8).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2349ClientTsize`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc2349_client_test.go#L20) | unit/verify | revert, verified |
| negative | [`TestRFC2349TsizeRRQReturnsActualSize`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2349_tsize_test.go#L37) | unit/verify | revert, verified |
| positive | [`TestRFC2349ClientTsize`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc2349_client_test.go#L17) | unit/verify | revert, verified |
| positive | [`TestRFC2349TsizeRRQReturnsActualSize`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2349_tsize_test.go#L32) | unit/verify | revert, verified |

### [`RFC2349-x-4`](#rfc2349-x-4)

In Write Request packets, the size of the file, in octets, is specified in the request and echoed back in the OACK. (§Transfer Size Option Specification)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2349-x-4, so no unit is bound to it.

### [`RFC2349-x-5`](#rfc2349-x-5)

If the file is too large for the client to handle, it may abort the transfer with an Error packet (error code 3). (§Transfer Size Option Specification)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRFC2349ClientAbortsTooLargeFile: an OACK tsize of fileSizeMax+1 must draw ERROR with code exactly 3 before any ACK, then silence, and a failed fetch (red on ACK 0 or another code); exactly fileSizeMax draws ACK 0 and completes, so the bound is not off by one.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2349ClientAbortsTooLargeFile`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc2349_client_test.go#L63) | unit/verify | revert, verified |
| positive | [`TestRFC2349ClientAbortsTooLargeFile`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc2349_client_test.go#L61) | unit/verify | revert, verified |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | prose |
| Source | rfc/full/rfc2349.txt |
| Source fingerprint | 3a54afc4baf69390 |
| Record | rfc/extraction/rfc2349.json |
| Mapped sentences | 1 |
| Declined as scope | 1 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 2 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `front:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Boilerplate: the sentence is part of the RFC's Full Copyright Statement and constrains redistribution of the document, not the behavior of a TFTP implementation. | However, this document itself may not be modified in any way, such as by removing the copyright notice or references to the Internet Society or other Internet organizations, except as needed for the purpose of developing Internet standards in which case the procedures for copyrights defined in the Internet Standards process must be followed, or as required to translate it into languages other than English. |

## Superseded

No document obsoletes RFC 2349, so its obligations are stated where they were written.
