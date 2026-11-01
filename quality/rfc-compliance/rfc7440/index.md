# RFC 7440 - TFTP Windowsize Option

TFTP windowsize. Every requirement this repository extracted from RFC 7440, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 9 | of 9 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0 | of 9 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0 | of 9 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0 | of 9 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| No test at all | 0 | of 9 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 26 | 26 of 26 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |
| Audit verdicts | 10 | of 9 gated MUSTs judged | 0 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 9 | of 15 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the counts beside it are what says how Ze stands |
| Out of scope | 0 | of 9 gated MUSTs | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as a Ze gap. Scope, not coverage: it is counted in the gated whole the cards on this page add up to |
| Not applicable | 0 | of 9 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as a Ze gap. It is counted in the gated whole the cards add up to |
| Met below Ze | 0 | of 9 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze proven: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 9 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as a Ze gap. It is counted in the gated whole the cards add up to |
| Linux cannot do this | 0 | of 9 gated MUSTs | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as a Ze gap. It is counted in the gated whole the cards add up to and earns no proof credit |

The 9 counts marked as a part above are the whole of the 9 gated MUSTs: they add up to 9. Proven by a recorded break counts TAGGED UNITS, a different population, so it is not one of them.

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
| Public row | TFTP windowsize |
| Enrolment | Enrolled |
| Requirements | 15 |
| Gated MUST-level | 9 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 26 |
| Tagged units | 26 |
| Recorded audit verdicts | 10 |
| Discrimination records | 26 |
| Summary | `rfc/short/rfc7440.md` |
| Requirement shard | `rfc/requirements/rfc7440.md` |
| RFC text | `rfc/full/rfc7440.txt` |

## Enrolment

Enrolled: TFTP Windowsize Option: nine MUST-level requirements. 3-1 (all RRQ/WRQ fields except the opcode are NUL-terminated ASCII strings) is met with positive+negative tags on the server's RRQ parser (internal/plugins/tftpserver/handler.go parseRRQ rejects a field lacking its NUL). Four are met by Ze's TFTP client, the data receiver of a read and the tftp scheme of ze init --from (internal/core/fetch/tftp.go, internal/core/tftp/oack.go grantWindowsize), each with both polarities: 3-2 (an OACK windowsize outside 1..65535 is refused with ERROR 8) and 3-4 (the client uses the OACK windowsize or sends ERROR 8) via TestRFC7440ClientWindowsize, 4-2 (the receiver ACKs the last block of each window) via TestRFC7440ClientWindowsize, and 4-3 (a window of one is RFC 1350 lockstep) via TestRFC7440ClientWindowOneIsLockstep; the SHOULD 4-6 (a sequence error draws the ACK of the last block received in order) via TestRFC7440ClientSequenceError. The four sender-side MUSTs are met by Ze's TFTP server, the data sender of a read (internal/plugins/tftpserver/handler.go grantWindowsize and sendOACKAndWait, internal/plugins/tftpserver/window_send.go serveFile and windowSender), each with both polarities in rfc7440_window_send_test.go: 3-3 (the acknowledged windowsize is at most the request and the configured max-windowsize) via TestRFC7440ServerAcknowledgesWindowsize, 4-1 (the agreed window, then a wait for its ACK) via TestRFC7440ServerSendsWindow, 4-4 (a timeout restarts the window after the last ACK) via TestRFC7440ServerTimeoutRestartsWindow, and 4-5 (an ACK inside the window starts the next one after it) via TestRFC7440ServerInWindowACKRestartsWindow; the server side of 4-3 (a window of one is lockstep) via TestRFC7440ServerWindowOneIsLockstep. The tftp-hpa 7.2 client downloads from the windowed server in TestTFTPHPAWindowsizeInterop.

## What the public ledger says

**Area:** TFTP windowsize

**Proof:** 9 of 9 MUST proven

**Evidence:** unit 9

**Unmet:** -


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 9 | one part of the gated population |
| Annotated (including scoped evidence) | 0 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **9** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (9):** [`RFC7440-3-1`](#rfc7440-3-1), [`RFC7440-3-2`](#rfc7440-3-2), [`RFC7440-3-3`](#rfc7440-3-3), [`RFC7440-3-4`](#rfc7440-3-4), [`RFC7440-4-1`](#rfc7440-4-1), [`RFC7440-4-2`](#rfc7440-4-2), [`RFC7440-4-3`](#rfc7440-4-3), [`RFC7440-4-4`](#rfc7440-4-4), [`RFC7440-4-5`](#rfc7440-4-5)

## Figures

- **MUST bar**: Ze proven 9
- **Overview bar**: Ze MUST proven 9, SHOULD proven 1, SHOULD not decided 5

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Not walked | 0 |
| Mandatory register | complete |
| MUST rows | 9 |
| MUST rows kept (the MUST bar) | 9 |
| Ze proven | 9 |
| OS verified | 0 |
| OS testing gap | 0 |
| Ze correctness gap | 0 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 6 |
| SHOULD proven | 1 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD not ever | 0 |
| SHOULD not yet | 0 |
| SHOULD not decided | 5 |

The MUST bar counts the MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT rows of the walked RFCs that the stack owes: Ze proven means a recorded test break proves the row; a row a layer under Ze performs is OS verified when a test checks that layer on the state Ze installs, an OS testing gap until one does. The overview bar adds the MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT rows and the rows not owed, which do not bind Ze. An RFC that is not walked has no valid extraction sign-off, so its rows count in no figure. Every count is read from the RFC data files when the page loads.

Source: [data/rfc/rfc7440.json](../../../data/rfc/rfc7440.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC7440-3-1` | Note that all fields except "opc" MUST be ASCII strings followed by a single-byte NULL character. (§3) | MUST | 3 | **positive:** `unit/verify` [`TestRFC7440ClientRequestFieldsAreNULTerminatedASCII`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc7440_client_test.go#L153). **positive:** `unit/verify` [`TestRFC7440FieldsAreNULTerminatedASCIIStrings`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2347_2348_transfer_test.go#L264). **positive:** `unit/verify` [`TestTFTPParseRRQ`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L99). **negative:** `unit/verify` [`TestRFC7440ClientRequestFieldsAreNULTerminatedASCII`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc7440_client_test.go#L156). **negative:** `unit/verify` [`TestRFC7440FieldsAreNULTerminatedASCIIStrings`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2347_2348_transfer_test.go#L268). **negative:** `unit/verify` [`TestTFTPParseRRQInvalid`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L127) |
| `RFC7440-3-2` | The valid values range MUST be between 1 and 65535 blocks, inclusive. (§3) | MUST | 3 | **positive:** `unit/verify` [`TestRFC7440ClientWindowsize`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc7440_client_test.go#L20). **negative:** `unit/verify` [`TestRFC7440ClientWindowsize`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc7440_client_test.go#L29) |
| `RFC7440-3-3` | If the server is willing to accept the windowsize option, it sends an Option Acknowledgment (OACK) to the client. The specified value MUST be less than or equal to the value specified by the client. (§3) | MUST | 3 | **positive:** `unit/verify` [`TestRFC7440ServerAcknowledgesWindowsize`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc7440_window_send_test.go#L60). **negative:** `unit/verify` [`TestRFC7440ServerAcknowledgesWindowsize`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc7440_window_send_test.go#L63) |
| `RFC7440-3-4` | The client MUST then either use the size specified in the OACK or send an ERROR packet, with error code 8, to terminate the transfer. (§3) | MUST | 3 | **positive:** `unit/verify` [`TestRFC7440ClientWindowsize`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc7440_client_test.go#L18). **negative:** `unit/verify` [`TestRFC7440ClientWindowsize`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc7440_client_test.go#L27) |
| `RFC7440-4-1` | The DSND MUST cyclically send to the DRCV the agreed windowsize consecutive data blocks before normally stopping and waiting for the ACK of the transferred window. (§4) | MUST | 4 | **positive:** `unit/verify` [`TestRFC7440ServerSendsWindow`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc7440_window_send_test.go#L105). **negative:** `unit/verify` [`TestRFC7440ServerSendsWindow`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc7440_window_send_test.go#L108) |
| `RFC7440-4-2` | The DRCV MUST send to the DSND the ACK of the last data block of the window in order to confirm a successful data block window reception. (§4) | MUST | 4 | **positive:** `unit/verify` [`TestRFC7440ClientWindowsize`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc7440_client_test.go#L22). **negative:** `unit/verify` [`TestRFC7440ClientWindowsize`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc7440_client_test.go#L25) |
| `RFC7440-4-3` | Traffic with windowsize = 1 MUST be equivalent to traffic specified by [RFC1350]. (§4) | MUST | 4 | **positive:** `unit/verify` [`TestRFC7440ClientWindowOneIsLockstep`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc7440_client_test.go#L80). **positive:** `unit/verify` [`TestRFC7440ServerWindowOneIsLockstep`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc7440_window_send_test.go#L137). **negative:** `unit/verify` [`TestRFC7440ClientWindowOneIsLockstep`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc7440_client_test.go#L84). **negative:** `unit/verify` [`TestRFC7440ServerWindowOneIsLockstep`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc7440_window_send_test.go#L140) |
| `RFC7440-4-4` | In the case of an expected ACK not timely reaching the DSND (timeout), the last received ACK SHALL set the beginning of the next windowsize data block window to be sent. (§4) | SHALL | 4 | **positive:** `unit/verify` [`TestRFC7440ServerTimeoutRestartsWindow`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc7440_window_send_test.go#L171). **negative:** `unit/verify` [`TestRFC7440ServerTimeoutRestartsWindow`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc7440_window_send_test.go#L175) |
| `RFC7440-4-5` | The notified DSND SHOULD send a new data block window whose beginning MUST be set based on the ACK received out of sequence. (§4) | MUST | 4 | **positive:** `unit/verify` [`TestRFC7440ServerInWindowACKRestartsWindow`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc7440_window_send_test.go#L213). **negative:** `unit/verify` [`TestRFC7440ServerInWindowACKRestartsWindow`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc7440_window_send_test.go#L216) |
| `RFC7440-5-1` | Operators SHOULD test various values and SHOULD be conservative when selecting a windowsize value (§5) | SHOULD | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC7440-6-1` | Implementations SHOULD always set a maximum number of retries for datagram retransmissions (§6) | SHOULD | 6 | **positive:** no positive test. **negative:** no negative test |
| `RFC7440-6-2` | Implementations SHOULD always set a maximum number of retries for datagram retransmissions, imposing an appropriate threshold on error recovery attempts, after which a transfer SHOULD always be aborted to prevent pathological retransmission conditions. (§6) | SHOULD | 6 | **positive:** no positive test. **negative:** no negative test |
| `RFC7440-4-6` | In the case of a data block sequence error, the DRCV SHOULD notify the DSND by sending an ACK corresponding to the last data block correctly received. (§4) | SHOULD | 4 | **positive:** `unit/verify` [`TestRFC7440ClientSequenceError`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc7440_client_test.go#L117). **negative:** `unit/verify` [`TestRFC7440ClientSequenceError`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc7440_client_test.go#L120) |
| `RFC7440-6-3` | The rate at which TFTP UDP datagrams are sent SHOULD follow the CC guidelines in Section 3.1 of [RFC5405]. (§6) | SHOULD | 6 | **positive:** no positive test. **negative:** no negative test |
| `RFC7440-7-1` | TFTP file transfers are NOT RECOMMENDED where the inherent protocol limitations could raise insurmountable liability concerns (§7) | NOT RECOMMENDED | 7 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

RFC 7440 declares no gap, and every gated MUST it carries has a test bound to it.

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC7440-3-1`](#rfc7440-3-1)

Note that all fields except "opc" MUST be ASCII strings followed by a single-byte NULL character. (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-31: TestRFC7440FieldsAreNULTerminatedASCIIStrings now requests blksize, tsize and windowsize 16 and requires the OACK to be exactly six non-empty printable-ASCII fields each ending in one NUL (blksize 1024, tsize 700, windowsize 16): a missing or doubled NUL, a non-ASCII octet, or an extra or missing field is red. Its request half is unchanged: the Section 3 example parses into foobar, octet, windowsize; an unterminated #blocks leaves windowsize unrecognised (parseRRQ breaks on valEnd < 0); a filename with no NUL is refused. TestTFTPParseRRQ/Invalid changed only in claim comments. Client TestRFC7440ClientRequestFieldsAreNULTerminatedASCII unchanged.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7440ClientRequestFieldsAreNULTerminatedASCII`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc7440_client_test.go#L156) | unit/verify | revert, verified |
| negative | [`TestTFTPParseRRQInvalid`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L127) | unit/verify | revert, verified |
| negative | [`TestRFC7440FieldsAreNULTerminatedASCIIStrings`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2347_2348_transfer_test.go#L268) | unit/verify | revert, verified |
| positive | [`TestRFC7440ClientRequestFieldsAreNULTerminatedASCII`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc7440_client_test.go#L153) | unit/verify | revert, verified |
| positive | [`TestTFTPParseRRQ`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L99) | unit/verify | revert, verified |
| positive | [`TestRFC7440FieldsAreNULTerminatedASCIIStrings`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2347_2348_transfer_test.go#L264) | unit/verify | revert, verified |

### [`RFC7440-3-2`](#rfc7440-3-2)

The valid values range MUST be between 1 and 65535 blocks, inclusive. (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRFC7440ClientWindowsize: OACK windowsize 0 must draw ERROR with code exactly 8, silence and a failed fetch (red if the lower-range check in grantWindowsize is removed: 0 would be acked). windowsize 2 is accepted with ACK 0. The 65536 case is refused by the not-above-request check, not the range check, so it isolates nothing; the upper bound cannot be exceeded because the client requests 4 and refuses anything above it. Refusing an in-range value is allowed by 3-4, so the only violation is using an out-of-range value, which the 0 case catches.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7440ClientWindowsize`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc7440_client_test.go#L29) | unit/verify | revert, verified |
| positive | [`TestRFC7440ClientWindowsize`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc7440_client_test.go#L20) | unit/verify | revert, verified |

### [`RFC7440-3-3`](#rfc7440-3-3)

If the server is willing to accept the windowsize option, it sends an Option Acknowledgment (OACK) to the client. The specified value MUST be less than or equal to the value specified by the client. (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RFC 7440 Section 3: 'The specified value MUST be less than or equal to the value specified by the client.' TestRFC7440ServerAcknowledgesWindowsize asserts the exact grant: 8 gives 8 and 64 gives 64 (red if the server grants its maximum regardless), 1000 and 65535 give 64 (red if it echoes the request above its maximum), plus granted <= asked on every case; 0, 65536 and abc are not acknowledged beside blksize. Producer grantWindowsize min(requested, maximum) and sendOACKAndWait.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7440ServerAcknowledgesWindowsize`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc7440_window_send_test.go#L63) | unit/verify | revert, verified |
| positive | [`TestRFC7440ServerAcknowledgesWindowsize`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc7440_window_send_test.go#L60) | unit/verify | revert, verified |

### [`RFC7440-3-4`](#rfc7440-3-4)

The client MUST then either use the size specified in the OACK or send an ERROR packet, with error code 8, to terminate the transfer. (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRFC7440ClientWindowsize: with windowsize 2 granted (4 requested), blocks 1 and 2 must draw ACK 2 within tftpTimeout/2 (500 ms against a 1 s timeout), so the retransmitted ACK no longer masks a client that keeps its own window. Observed red under the overlay mutant grantWindowsize setting granted.Windowsize = req.Windowsize. Negative: windowsize 5, above the request, draws ERROR 8 and a failed fetch.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7440ClientWindowsize`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc7440_client_test.go#L27) | unit/verify | revert, verified |
| positive | [`TestRFC7440ClientWindowsize`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc7440_client_test.go#L18) | unit/verify | revert, verified |

### [`RFC7440-4-1`](#rfc7440-4-1)

The DSND MUST cyclically send to the DRCV the agreed windowsize consecutive data blocks before normally stopping and waiting for the ACK of the transferred window. (§4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RFC 7440 Section 4: 'The DSND MUST cyclically send to the DRCV the agreed windowsize consecutive data blocks before normally stopping and waiting for the ACK of the transferred window.' TestRFC7440ServerSendsWindow, windowsize 4 over a 9-block file: blocks 1-4 in order, 1 s silence before ACK 4, blocks 5-8, silence before ACK 8, final short block 9, silence after its ACK, bytes equal the file. A sender that streams past the window, sends fewer than four, or keeps lockstep is red. Producer windowSender.sendWindow then awaitACK.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7440ServerSendsWindow`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc7440_window_send_test.go#L108) | unit/verify | revert, verified |
| positive | [`TestRFC7440ServerSendsWindow`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc7440_window_send_test.go#L105) | unit/verify | revert, verified |

### [`RFC7440-4-2`](#rfc7440-4-2)

The DRCV MUST send to the DSND the ACK of the last data block of the window in order to confirm a successful data block window reception. (§4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRFC7440ClientWindowsize: the ACK of each window's last block (2, 4, and the short 5) must arrive within tftpTimeout/2, before any timeout retransmission could produce it, and the first packet after blocks 1 and 2 must be ACK 2, not ACK 1. Observed red under the overlay mutant that uses the requested window 4 against a granted 2 (no ACK at window end).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7440ClientWindowsize`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc7440_client_test.go#L25) | unit/verify | revert, verified |
| positive | [`TestRFC7440ClientWindowsize`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc7440_client_test.go#L22) | unit/verify | revert, verified |

### [`RFC7440-4-3`](#rfc7440-4-3)

Traffic with windowsize = 1 MUST be equivalent to traffic specified by [RFC1350]. (§4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-31: the server now sends windows and gained TestRFC7440ServerWindowOneIsLockstep: windowsize 1 alone draws DATA 1 with no OACK, then silence for 1 s before ACK 1, then block 2; beside blksize 1024, windowsize 1 is left out of the OACK and the transfer is still lockstep. A sender that streams a second block before the ACK is red. grantWindowsize returns 1 for a request of 1, so the transfer takes the same serveFile path as an RFC 1350 request. Client TestRFC7440ClientWindowOneIsLockstep unchanged (each DATA draws its ACK within tftpTimeout/2; an out-of-order block draws nothing).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7440ClientWindowOneIsLockstep`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc7440_client_test.go#L84) | unit/verify | revert, verified |
| negative | [`TestRFC7440ServerWindowOneIsLockstep`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc7440_window_send_test.go#L140) | unit/verify | revert, verified |
| positive | [`TestRFC7440ClientWindowOneIsLockstep`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc7440_client_test.go#L80) | unit/verify | revert, verified |
| positive | [`TestRFC7440ServerWindowOneIsLockstep`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc7440_window_send_test.go#L137) | unit/verify | revert, verified |

### [`RFC7440-4-4`](#rfc7440-4-4)

In the case of an expected ACK not timely reaching the DSND (timeout), the last received ACK SHALL set the beginning of the next windowsize data block window to be sent. (§4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RFC 7440 Section 4: 'In the case of an expected ACK not timely reaching the DSND (timeout), the last received ACK SHALL set the beginning of the next windowsize data block window to be sent.' TestRFC7440ServerTimeoutRestartsWindow, windowsize 4 and timeout 1: after ACK 4, the unacked window 5-8 must come again as 5-8 with identical bytes (red on a restart from 1 or a skip to 9), then ACK 8 draws 9-12; a never-acknowledged window is sent four times then silence for 2 s. Producer serveFile keeps first on errNoACK.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7440ServerTimeoutRestartsWindow`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc7440_window_send_test.go#L175) | unit/verify | revert, verified |
| positive | [`TestRFC7440ServerTimeoutRestartsWindow`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc7440_window_send_test.go#L171) | unit/verify | revert, verified |

### [`RFC7440-4-5`](#rfc7440-4-5)

The notified DSND SHOULD send a new data block window whose beginning MUST be set based on the ACK received out of sequence. (§4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RFC 7440 Section 4: 'The notified DSND SHOULD send a new data block window whose beginning MUST be set based on the ACK received out of sequence.' TestRFC7440ServerInWindowACKRestartsWindow, windowsize 4: blocks 1-4 sent, ACK 2 must draw exactly 3-6 before the 5 s timeout would (red on a restart from 1 or 5, or on waiting for the timeout); a repeated ACK 2 (the block before the window, a duplicate under RFC 1350 Section 5) and ACK 20 (never sent) draw no DATA; ACK 6 then draws 7-10. Producer serveFile first = acked + 1 with awaitACK's windowPosition.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7440ServerInWindowACKRestartsWindow`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc7440_window_send_test.go#L216) | unit/verify | revert, verified |
| positive | [`TestRFC7440ServerInWindowACKRestartsWindow`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc7440_window_send_test.go#L213) | unit/verify | revert, verified |

### [`RFC7440-4-6`](#rfc7440-4-6)

In the case of a data block sequence error, the DRCV SHOULD notify the DSND by sending an ACK corresponding to the last data block correctly received. (§4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRFC7440ClientSequenceError: after block 1 and then block 3 (block 2 lost) in a window of four, ACK 1 must arrive within tftpTimeout/2, so only the sequence-error notification can produce it, not the timeout retransmission. Negative: block 4, still past the gap, draws no second ACK for 300 ms. Then the restarted window completes the file. Observed red under an overlay mutant that drops the gap ACK in (*tftpTransfer).data, re-made against the current tftp.go.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7440ClientSequenceError`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc7440_client_test.go#L120) | unit/verify | revert, verified |
| positive | [`TestRFC7440ClientSequenceError`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc7440_client_test.go#L117) | unit/verify | revert, verified |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | rfc2119 |
| Source | rfc/full/rfc7440.txt |
| Source fingerprint | a7ede16e34257a1a |
| Record | rfc/extraction/rfc7440.json |
| Mapped sentences | 9 |
| Declined as scope | 0 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | walked | not stated |
| `1` | not stated | 0 | walked | not stated |
| `2` | not stated | 0 | walked | not stated |
| `3` | not stated | 4 | walked | not stated |
| `4` | not stated | 5 | walked | not stated |
| `5` | not stated | 0 | walked | not stated |
| `6` | not stated | 0 | walked | not stated |
| `7` | not stated | 0 | walked | not stated |
| `8` | not stated | 0 | walked | not stated |
| `8.1` | not stated | 0 | walked | not stated |

### Excluded sentences

The walk over RFC 7440 declined no sentence: every site it found is mapped to a requirement.

## Superseded

No document obsoletes RFC 7440, so its obligations are stated where they were written.
