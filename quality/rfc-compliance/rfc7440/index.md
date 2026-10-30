# RFC 7440 - TFTP Windowsize Option

Partial. Every requirement this repository extracted from RFC 7440, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 55.6% | 5 of 9 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 9 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 9 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 9 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Proven by a recorded break | 100.0% | 16 of 16 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 9 | of 15 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 9 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 9 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 9 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 9 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |
| Linux cannot do this | 0.0% | 0 of 9 gated MUSTs | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and remains unmet; it stays in every denominator and earns no proof credit |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 44.4% | 4 of 9 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

The 9 shares marked as a part above are the whole of the 9 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| Partial proof; remaining gap | ok | green at zero, RED above it: a tested clause cannot prove the whole requirement |
| No test at all | bad | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green shares above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Linux cannot do this | neutral | neutral at every value: an evidenced platform limit is neither a proof nor a scope exemption |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Partial |
| Enrolment | Enrolled |
| Requirements | 15 |
| Gated MUST-level | 9 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 4 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 16 |
| Tagged units | 16 |
| Recorded audit verdicts | 6 |
| Discrimination records | 16 |
| Summary | `rfc/short/rfc7440.md` |
| Requirement shard | `rfc/requirements/rfc7440.md` |
| RFC text | `rfc/full/rfc7440.txt` |

## Enrolment

Enrolled: TFTP Windowsize Option: nine MUST-level requirements. 3-1 (all RRQ/WRQ fields except the opcode are NUL-terminated ASCII strings) is met with positive+negative tags on the server's RRQ parser (internal/plugins/tftpserver/handler.go parseRRQ rejects a field lacking its NUL). Four are met by Ze's TFTP client, the data receiver of a read and the tftp scheme of ze init --from (internal/core/fetch/tftp.go, internal/core/tftp/oack.go grantWindowsize), each with both polarities: 3-2 (an OACK windowsize outside 1..65535 is refused with ERROR 8) and 3-4 (the client uses the OACK windowsize or sends ERROR 8) via TestRFC7440ClientWindowsize, 4-2 (the receiver ACKs the last block of each window) via TestRFC7440ClientWindowsize, and 4-3 (a window of one is RFC 1350 lockstep) via TestRFC7440ClientWindowOneIsLockstep; the SHOULD 4-6 (a sequence error draws the ACK of the last block received in order) via TestRFC7440ClientSequenceError. The four sender-side MUSTs (3-3 acknowledged windowsize, 4-1 windowed send, 4-4 timeout window-start, 4-5 sequence-error rollback) stay {not-applicable}: ze's TFTP server implements base RFC 1350 with the RFC 2347/2348/2349 blksize and tsize options only; it records the windowsize option name but discards the value and never negotiates it, falling back to lockstep.

## What the public ledger says

**Status:** Partial

**What the ledger says is covered:**

- The TFTP client ([`internal/core/fetch/tftp.go`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/tftp.go), [`internal/core/tftp/oack.go`](https://github.com/ze-software/ze/blob/main/internal/core/tftp/oack.go)) negotiates and uses windowsize
- the TFTP server ([`internal/plugins/tftpserver/handler.go`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler.go)) serves in RFC 1350 lockstep and never acknowledges windowsize.


**What the ledger says remains:**

Four MUST gaps in proof on the server side ([`RFC7440-3-3`](#rfc7440-3-3), 4-1, 4-4, 4-5): the lockstep server meets each as a window of one, and no test asserts it.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 5 | one part of the gated population |
| Annotated (including scoped evidence) | 4 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **9** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (5):** [`RFC7440-3-1`](#rfc7440-3-1), [`RFC7440-3-2`](#rfc7440-3-2), [`RFC7440-3-4`](#rfc7440-3-4), [`RFC7440-4-2`](#rfc7440-4-2), [`RFC7440-4-3`](#rfc7440-4-3)

**Annotated (including scoped evidence) (4):** [`RFC7440-3-3`](#rfc7440-3-3), [`RFC7440-4-1`](#rfc7440-4-1), [`RFC7440-4-4`](#rfc7440-4-4), [`RFC7440-4-5`](#rfc7440-4-5)

## Attainable ceiling

Mandatory ceiling: 9 of 9 (100.0%). Optional ceiling: 6 of 6 (100.0%). Every RFC shown with measured rows can reach 100% under current classifications. Lowest mandatory ceiling: rfc7440 9 of 9 (100.0%). Lowest optional ceiling: rfc7440 6 of 6 (100.0%). Ceilings describe current classifications, not an immutable Linux maximum. Not-applicable and feature-declined classifications can be revisited. Every gap remains work Ze owes.

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC7440-3-1` | Note that all fields except "opc" MUST be ASCII strings followed by a single-byte NULL character. (§3) | MUST | 3 | **positive:** `unit/verify` [`TestRFC7440ClientRequestFieldsAreNULTerminatedASCII`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc7440_client_test.go#L153). **positive:** `unit/verify` [`TestRFC7440FieldsAreNULTerminatedASCIIStrings`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2347_2348_transfer_test.go#L259). **positive:** `unit/verify` [`TestTFTPParseRRQ`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L99). **negative:** `unit/verify` [`TestRFC7440ClientRequestFieldsAreNULTerminatedASCII`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc7440_client_test.go#L156). **negative:** `unit/verify` [`TestRFC7440FieldsAreNULTerminatedASCIIStrings`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2347_2348_transfer_test.go#L263). **negative:** `unit/verify` [`TestTFTPParseRRQInvalid`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L127) |
| `RFC7440-3-2` | The valid values range MUST be between 1 and 65535 blocks, inclusive. (§3) | MUST | 3 | **positive:** `unit/verify` [`TestRFC7440ClientWindowsize`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc7440_client_test.go#L20). **negative:** `unit/verify` [`TestRFC7440ClientWindowsize`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc7440_client_test.go#L29) |
| `RFC7440-3-3` | If the server is willing to accept the windowsize option, it sends an Option Acknowledgment (OACK) to the client. The specified value MUST be less than or equal to the value specified by the client. (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** gap in proof: the condition never holds, because internal/plugins/tftpserver/handler.go::sendOACKAndWait builds its OACK from blksize and tsize only and never acknowledges windowsize, so no acknowledged value can exceed the client's; no test asserts it |
| `RFC7440-3-4` | The client MUST then either use the size specified in the OACK or send an ERROR packet, with error code 8, to terminate the transfer. (§3) | MUST | 3 | **positive:** `unit/verify` [`TestRFC7440ClientWindowsize`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc7440_client_test.go#L18). **negative:** `unit/verify` [`TestRFC7440ClientWindowsize`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc7440_client_test.go#L27) |
| `RFC7440-4-1` | The DSND MUST cyclically send to the DRCV the agreed windowsize consecutive data blocks before normally stopping and waiting for the ACK of the transferred window. (§4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** gap in proof: with no windowsize agreed, internal/plugins/tftpserver/handler.go::serveFile sends one DATA block per ACK through sendAndWaitACK, the RFC 1350 lockstep that RFC7440-4-3 equates with a window of one; no test asserts the server side |
| `RFC7440-4-2` | The DRCV MUST send to the DSND the ACK of the last data block of the window in order to confirm a successful data block window reception. (§4) | MUST | 4 | **positive:** `unit/verify` [`TestRFC7440ClientWindowsize`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc7440_client_test.go#L22). **negative:** `unit/verify` [`TestRFC7440ClientWindowsize`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc7440_client_test.go#L25) |
| `RFC7440-4-3` | Traffic with windowsize = 1 MUST be equivalent to traffic specified by [RFC1350]. (§4) | MUST | 4 | **positive:** `unit/verify` [`TestRFC7440ClientWindowOneIsLockstep`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc7440_client_test.go#L80). **negative:** `unit/verify` [`TestRFC7440ClientWindowOneIsLockstep`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc7440_client_test.go#L84) |
| `RFC7440-4-4` | In the case of an expected ACK not timely reaching the DSND (timeout), the last received ACK SHALL set the beginning of the next windowsize data block window to be sent. (§4) | SHALL | 4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** gap in proof: on timeout internal/plugins/tftpserver/handler.go::sendAndWaitACK resends the one unacknowledged block, the block after the last received ACK, which is the window of one that ACK sets; no test asserts it |
| `RFC7440-4-5` | The notified DSND SHOULD send a new data block window whose beginning MUST be set based on the ACK received out of sequence. (§4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** gap in proof: internal/plugins/tftpserver/handler.go::sendAndWaitACK accepts only the ACK of the block just sent and otherwise resends that block, the window of one an out-of-sequence ACK sets; no test asserts it |
| `RFC7440-5-1` | Operators SHOULD test various values and SHOULD be conservative when selecting a windowsize value (§5) | SHOULD | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC7440-6-1` | Implementations SHOULD always set a maximum number of retries for datagram retransmissions (§6) | SHOULD | 6 | **positive:** no positive test. **negative:** no negative test |
| `RFC7440-6-2` | Implementations SHOULD always set a maximum number of retries for datagram retransmissions, imposing an appropriate threshold on error recovery attempts, after which a transfer SHOULD always be aborted to prevent pathological retransmission conditions. (§6) | SHOULD | 6 | **positive:** no positive test. **negative:** no negative test |
| `RFC7440-4-6` | In the case of a data block sequence error, the DRCV SHOULD notify the DSND by sending an ACK corresponding to the last data block correctly received. (§4) | SHOULD | 4 | **positive:** `unit/verify` [`TestRFC7440ClientSequenceError`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc7440_client_test.go#L117). **negative:** `unit/verify` [`TestRFC7440ClientSequenceError`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc7440_client_test.go#L120) |
| `RFC7440-6-3` | The rate at which TFTP UDP datagrams are sent SHOULD follow the CC guidelines in Section 3.1 of [RFC5405]. (§6) | SHOULD | 6 | **positive:** no positive test. **negative:** no negative test |
| `RFC7440-7-1` | TFTP file transfers are NOT RECOMMENDED where the inherent protocol limitations could raise insurmountable liability concerns (§7) | NOT RECOMMENDED | 7 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC7440-3-3`](#rfc7440-3-3) If the server is willing to accept the windowsize option, it sends an Option Acknowledgment (OACK) to the client. The specified value MUST be less than or equal to the value specified by the client. (§3) | {gap}, no test | gap in proof: the condition never holds, because internal/plugins/tftpserver/handler.go::sendOACKAndWait builds its OACK from blksize and tsize only and never acknowledges windowsize, so no acknowledged value can exceed the client's; no test asserts it |
| [`RFC7440-4-1`](#rfc7440-4-1) The DSND MUST cyclically send to the DRCV the agreed windowsize consecutive data blocks before normally stopping and waiting for the ACK of the transferred window. (§4) | {gap}, no test | gap in proof: with no windowsize agreed, internal/plugins/tftpserver/handler.go::serveFile sends one DATA block per ACK through sendAndWaitACK, the RFC 1350 lockstep that RFC7440-4-3 equates with a window of one; no test asserts the server side |
| [`RFC7440-4-4`](#rfc7440-4-4) In the case of an expected ACK not timely reaching the DSND (timeout), the last received ACK SHALL set the beginning of the next windowsize data block window to be sent. (§4) | {gap}, no test | gap in proof: on timeout internal/plugins/tftpserver/handler.go::sendAndWaitACK resends the one unacknowledged block, the block after the last received ACK, which is the window of one that ACK sets; no test asserts it |
| [`RFC7440-4-5`](#rfc7440-4-5) The notified DSND SHOULD send a new data block window whose beginning MUST be set based on the ACK received out of sequence. (§4) | {gap}, no test | gap in proof: internal/plugins/tftpserver/handler.go::sendAndWaitACK accepts only the ACK of the block just sent and otherwise resends that block, the window of one an out-of-sequence ACK sets; no test asserts it |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC7440-3-1`](#rfc7440-3-1)

Note that all fields except "opc" MUST be ASCII strings followed by a single-byte NULL character. (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Both roles are now tagged. Sender (Ze's client): TestRFC7440ClientRequestFieldsAreNULTerminatedASCII compares the whole RRQ byte for byte (f.bin, octet, blksize 1468, tsize 0, windowsize 4, each ASCII with one NUL), so a missing or doubled NUL is red; BuildRRQ must refuse a filename holding a NUL or a non-ASCII byte (a\x00b, caf\xc3\xa9). Receiver (server): TestTFTPParseRRQ/Invalid and TestRFC7440FieldsAreNULTerminatedASCIIStrings, rename-only diffs, still refuse an unterminated field.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7440ClientRequestFieldsAreNULTerminatedASCII`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc7440_client_test.go#L156) | unit/verify | revert, verified |
| negative | [`TestTFTPParseRRQInvalid`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L127) | unit/verify | revert, verified |
| negative | [`TestRFC7440FieldsAreNULTerminatedASCIIStrings`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2347_2348_transfer_test.go#L263) | unit/verify | revert, verified |
| positive | [`TestRFC7440ClientRequestFieldsAreNULTerminatedASCII`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc7440_client_test.go#L153) | unit/verify | revert, verified |
| positive | [`TestTFTPParseRRQ`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L99) | unit/verify | revert, verified |
| positive | [`TestRFC7440FieldsAreNULTerminatedASCIIStrings`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2347_2348_transfer_test.go#L259) | unit/verify | revert, verified |

### [`RFC7440-3-2`](#rfc7440-3-2)

The valid values range MUST be between 1 and 65535 blocks, inclusive. (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRFC7440ClientWindowsize: OACK windowsize 0 must draw ERROR with code exactly 8, silence and a failed fetch (red if the lower-range check in grantWindowsize is removed: 0 would be acked). windowsize 2 is accepted with ACK 0. The 65536 case is refused by the not-above-request check, not the range check, so it isolates nothing; the upper bound cannot be exceeded because the client requests 4 and refuses anything above it. Refusing an in-range value is allowed by 3-4, so the only violation is using an out-of-range value, which the 0 case catches.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7440ClientWindowsize`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc7440_client_test.go#L29) | unit/verify | revert, verified |
| positive | [`TestRFC7440ClientWindowsize`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc7440_client_test.go#L20) | unit/verify | revert, verified |

### [`RFC7440-3-3`](#rfc7440-3-3)

If the server is willing to accept the windowsize option, it sends an Option Acknowledgment (OACK) to the client. The specified value MUST be less than or equal to the value specified by the client. (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7440-3-3, so no unit is bound to it.

### [`RFC7440-3-4`](#rfc7440-3-4)

The client MUST then either use the size specified in the OACK or send an ERROR packet, with error code 8, to terminate the transfer. (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRFC7440ClientWindowsize: with windowsize 2 granted (4 requested), blocks 1 and 2 must draw ACK 2 within tftpTimeout/2 (500 ms against a 1 s timeout), so the retransmitted ACK no longer masks a client that keeps its own window. Observed red under the overlay mutant grantWindowsize setting granted.Windowsize = req.Windowsize. Negative: windowsize 5, above the request, draws ERROR 8 and a failed fetch.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7440ClientWindowsize`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc7440_client_test.go#L27) | unit/verify | revert, verified |
| positive | [`TestRFC7440ClientWindowsize`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc7440_client_test.go#L18) | unit/verify | revert, verified |

### [`RFC7440-4-1`](#rfc7440-4-1)

The DSND MUST cyclically send to the DRCV the agreed windowsize consecutive data blocks before normally stopping and waiting for the ACK of the transferred window. (§4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7440-4-1, so no unit is bound to it.

### [`RFC7440-4-2`](#rfc7440-4-2)

The DRCV MUST send to the DSND the ACK of the last data block of the window in order to confirm a successful data block window reception. (§4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRFC7440ClientWindowsize: the ACK of each window's last block (2, 4, and the short 5) must arrive within tftpTimeout/2, before any timeout retransmission could produce it, and the first packet after blocks 1 and 2 must be ACK 2, not ACK 1. Observed red under the overlay mutant that uses the requested window 4 against a granted 2 (no ACK at window end).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7440ClientWindowsize`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc7440_client_test.go#L25) | unit/verify | revert, verified |
| positive | [`TestRFC7440ClientWindowsize`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc7440_client_test.go#L22) | unit/verify | revert, verified |

### [`RFC7440-4-3`](#rfc7440-4-3)

Traffic with windowsize = 1 MUST be equivalent to traffic specified by [RFC1350]. (§4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRFC7440ClientWindowOneIsLockstep now uses the shared shortenTFTP (tftpTimeout 1 s, expectACK waits 500 ms). With windowsize 1, every DATA must draw its own ACK within tftpTimeout/2, before the next DATA is sent, so lockstep timing is now bounded. Block 3, sent while block 2 is expected, must draw nothing for 300 ms and must not be written, and the file must hold each block once. Observed red under the overlay mutant that ignores the granted windowsize: 'client sent 00 04 00 01, want nothing'.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7440ClientWindowOneIsLockstep`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc7440_client_test.go#L84) | unit/verify | revert, verified |
| positive | [`TestRFC7440ClientWindowOneIsLockstep`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc7440_client_test.go#L80) | unit/verify | revert, verified |

### [`RFC7440-4-4`](#rfc7440-4-4)

In the case of an expected ACK not timely reaching the DSND (timeout), the last received ACK SHALL set the beginning of the next windowsize data block window to be sent. (§4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7440-4-4, so no unit is bound to it.

### [`RFC7440-4-5`](#rfc7440-4-5)

The notified DSND SHOULD send a new data block window whose beginning MUST be set based on the ACK received out of sequence. (§4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7440-4-5, so no unit is bound to it.

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
