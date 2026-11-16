# RFC 2349 - TFTP Timeout Interval and Transfer Size Options

TFTP timeout and transfer size options. Every requirement listed for RFC 2349, the tests linked to it, and their recorded reviews. This summary is included in the automatic compliance check, ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 3 | of 4 mandatory requirements | a positive test checks that Ze does what the requirement demands and a negative one checks that it refuses what the requirement forbids |
| One polarity plus reason | 0 | of 4 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason supplies the complete test coverage required |
| One polarity, unexcused | 0 | of 4 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial test coverage | 0 | of 4 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unverified, with zero whole-requirement credit |
| Tests shown to detect a fault | 10 | 10 of 10 tests linked to requirements | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests verifies a requirement only after both tests have been shown to detect a fault |
| Audit verdicts | 4 | of 4 mandatory requirements reviewed | 0 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Declared gap or no test | 1 | of 4 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 4 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 0 | of 4 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 0 | of 4 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze verified: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 4 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 4 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no test verification credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level) | 4 | of 6 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements included in the automatic checks: the compliance check (./le rfc check) holds Ze to each of them. This is the total being assessed; the counts above show the recorded status of those requirements. |

The 9 counts marked as a part above are the whole of the 4 mandatory requirements: they add up to 4. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got. An outlined card with no colored edge, under Totals, is a total the other cards are counted against, not a measure.

| Card | Tone here | Why that color |
|---|---|---|
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the count under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: one test direction is missing without a recorded reason |
| Partial test coverage | ok | green at zero, RED above it: testing one clause does not verify the whole requirement |
| Declared gap or no test | bad | green at zero, RED above it: a binding obligation that nothing shows Ze meets is owed, whether or not a reason is stated |
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
| Public row | TFTP timeout and transfer size options |
| Automatic checks | Included |
| Requirements | 6 |
| Mandatory requirements (MUST level) | 4 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 1 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 10 |
| Tests linked to requirements | 10 |
| Recorded audit verdicts | 4 |
| Fault test records | 10 |
| Summary | `rfc/short/rfc2349.md` |
| Requirement table | `rfc/requirements/rfc2349.md` |
| RFC text | `rfc/full/rfc2349.txt` |

## Automatic checks

Included in automatic checks: TFTP Timeout Interval and Transfer Size Options: four MUST-level requirements. RFC2349-x-3 (tsize in RRQ: client value "0", server returns actual file size in the OACK) is met and tested with both polarities via loopback OACK round-trips in rfc2349_tsize_test.go (producer sendOACKAndWait os.Stat + oackOpts tsize, internal/plugins/tftpserver/handler.go): a 7-byte file returns tsize "7", a 20-byte file returns "20", and the server never echoes the client placeholder "0". RFC2349-x-1 (the acknowledged timeout matches the request) and RFC2349-x-2 (timeout range 1-255) are met by the server's timeout option and tested with both polarities in rfc2349_timeout_test.go (producers parseRRQ, optionInRange, handleRRQ and sendOACKAndWait, internal/plugins/tftpserver/handler.go): timeout 1 and 255 are echoed unchanged, timeout 1 becomes the retransmission interval, and 0, 256, abc, +3 and the empty value are not acknowledged and leave the configured interval; curl 8.23.0's timeout is echoed in TestTFTPCurlTimeoutInterop. The one gap left, with no owner decision declining the feature, is RFC2349-x-4 (tsize in WRQ echoes the client size): it governs write requests, which Ze (a read-only TFTP server) rejects with Illegal-Operation. On 2026-10-31 Thomas put TFTP uploads (write requests) in scope for Ze's TFTP server, so x-4 is owed work, owned by plan/post-release/features/spec-tftp-server-uploads.md together with the WRQ side of x-1, x-2 and x-6. No SHOULD/MAY requirements are gated. The client side of RFC2349-x-3 (the read request carries tsize "0" and the OACK size is taken) is tested by Ze's TFTP client, TestRFC2349ClientTsize (internal/core/tftp/request.go BuildRRQ, internal/core/tftp/oack.go grantTsize), and the MAY x-5 (a file too large for the client draws ERROR 3) by TestRFC2349ClientAbortsTooLargeFile.

## Published support status

**Area:** TFTP timeout and transfer size options

**Test verification:** 3 of 4 MUST proven

**Evidence:** unit 3

**Unmet:** RFC2349-x-4


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 3 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 1 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 0 | one part of the mandatory requirements |
| Partial test coverage (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level)** | **4** | every mandatory requirement appears in exactly one group above |

**Positive and negative tests (3):** [`RFC2349-x-1`](#rfc2349-x-1), [`RFC2349-x-2`](#rfc2349-x-2), [`RFC2349-x-3`](#rfc2349-x-3)

**Annotated (including scoped evidence) (1):** [`RFC2349-x-4`](#rfc2349-x-4)

## Figures

- **MUST bar**: Ze verified 3, Not built 1
- **Overview bar**: Ze MUST verified 3, SHOULD verified 1, Not assessed 1, Not built 1

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | not complete |
| MUST rows | 4 |
| MUST rows kept (the MUST bar) | 4 |
| Ze verified | 3 |
| OS verified | 0 |
| Tests awaiting verification | 0 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 1 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 2 |
| SHOULD verified | 1 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD tests awaiting verification | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 1 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze verified means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Tests awaiting verification counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

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

## Test status

A test reads not verified when no record shows it failing after a deliberate fault. Until then, the test has not been shown to detect a fault in the required behavior.

### [`RFC2349-x-1`](#rfc2349-x-1)

If the server is willing to accept the timeout option, it sends an Option Acknowledgment (OACK) to the client. The specified timeout value must match the value specified by the client. (§Timeout Interval Option Specification)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RFC 2349 Timeout Interval: 'If the server is willing to accept the timeout option, it sends an Option Acknowledgment (OACK) to the client. The specified timeout value must match the value specified by the client.' TestRFC2349ServerAcknowledgesTimeout: timeout 1 and 255 must come back unchanged in the OACK (red on any counter-proposal or clamp, e.g. a cap below 255), and with timeout 1 an unacked DATA 1 must be resent within 3 s yet not before 0.8 s, against a 5 s default, so acknowledging a value and then using another is red. Producer: sendOACKAndWait echoes t.opts.timeout; handleRRQ sets the interval from it.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC2349ServerAcknowledgesTimeout`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2349_timeout_test.go#L20) | unit/verify | revert, verified |
| positive | [`TestRFC2349ServerAcknowledgesTimeout`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2349_timeout_test.go#L16) | unit/verify | revert, verified |

### [`RFC2349-x-2`](#rfc2349-x-2)

The number of seconds to wait before retransmitting, specified in ASCII. Valid values range between "1" and "255" seconds, inclusive. (§Timeout Interval Option Specification)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RFC 2349: '#secs ... specified in ASCII. Valid values range between "1" and "255" seconds, inclusive.' TestRFC2349ServerDeclinesTimeoutOutOfRange: the bounds 1 and 255 are acknowledged beside tsize; 0, 256, abc, +3 and the empty value each draw an OACK with tsize 100 and no timeout (isolated: the OACK still exists, so the option alone was declined); declined 0 is not used as the interval (no resend within 2 s), and timeout 0 alone draws DATA 1 with no OACK. Off-by-one at either bound or accepting a signed or non-decimal value is red. Producer parseRRQ via optionInRange(TimeoutMin, TimeoutMax).

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC2349ServerDeclinesTimeoutOutOfRange`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2349_timeout_test.go#L57) | unit/verify | revert, verified |
| positive | [`TestRFC2349ServerDeclinesTimeoutOutOfRange`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2349_timeout_test.go#L55) | unit/verify | revert, verified |

### [`RFC2349-x-3`](#rfc2349-x-3)

In Read Request packets, a size of "0" is specified in the request and the size of the file, in octets, is returned in the OACK. (§Transfer Size Option Specification)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-31: TestRFC2349TsizeRRQReturnsActualSize changed only in its claim comment (producer named as sendOACKAndWait, where os.Stat and the tsize entry now live, which the code confirms); assertions identical: tsize 7 and 20 for 7- and 20-octet files, red on an echo of "0" or a constant. Client TestRFC2349ClientTsize unchanged (whole RRQ byte for byte with tsize 0; '12k' refused with ERROR 8).

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC2349ClientTsize`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc2349_client_test.go#L20) | unit/verify | revert, verified |
| negative | [`TestRFC2349TsizeRRQReturnsActualSize`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2349_tsize_test.go#L37) | unit/verify | revert, verified |
| positive | [`TestRFC2349ClientTsize`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc2349_client_test.go#L17) | unit/verify | revert, verified |
| positive | [`TestRFC2349TsizeRRQReturnsActualSize`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2349_tsize_test.go#L32) | unit/verify | revert, verified |

### [`RFC2349-x-4`](#rfc2349-x-4)

In Write Request packets, the size of the file, in octets, is specified in the request and echoed back in the OACK. (§Transfer Size Option Specification)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC2349-x-4, so there is no linked test.

### [`RFC2349-x-5`](#rfc2349-x-5)

If the file is too large for the client to handle, it may abort the transfer with an Error packet (error code 3). (§Transfer Size Option Specification)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRFC2349ClientAbortsTooLargeFile: an OACK tsize of fileSizeMax+1 must draw ERROR with code exactly 3 before any ACK, then silence, and a failed fetch (red on ACK 0 or another code); exactly fileSizeMax draws ACK 0 and completes, so the bound is not off by one.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC2349ClientAbortsTooLargeFile`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc2349_client_test.go#L63) | unit/verify | revert, verified |
| positive | [`TestRFC2349ClientAbortsTooLargeFile`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc2349_client_test.go#L61) | unit/verify | revert, verified |

## Requirement list review

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
| `front` | not stated | 2 | reviewed | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `front:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Boilerplate: the sentence is part of the RFC's Full Copyright Statement and constrains redistribution of the document, not the behavior of a TFTP implementation. | However, this document itself may not be modified in any way, such as by removing the copyright notice or references to the Internet Society or other Internet organizations, except as needed for the purpose of developing Internet standards in which case the procedures for copyrights defined in the Internet Standards process must be followed, or as required to translate it into languages other than English. |

## Superseded

No document obsoletes RFC 2349, so its obligations are stated where they were written.
