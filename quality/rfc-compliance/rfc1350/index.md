# RFC 1350 - The TFTP Protocol (Revision 2)

Partial. Every requirement this repository extracted from RFC 1350, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 60.0% | 6 of 10 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 20.0% | 2 of 10 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 10 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 10 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Proven by a recorded break | 100.0% | 33 of 33 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |
| Audit verdicts | 10 | of 10 gated MUSTs judged | 0 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 10 | of 14 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 10 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 10 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 10 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 10 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |
| Linux cannot do this | 0.0% | 0 of 10 gated MUSTs | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and remains unmet; it stays in every denominator and earns no proof credit |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 20.0% | 2 of 10 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

The 9 shares marked as a part above are the whole of the 10 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Audit verdicts | ok | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Partial |
| Enrolment | Enrolled |
| Requirements | 14 |
| Gated MUST-level | 10 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 2 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 33 |
| Tagged units | 33 |
| Recorded audit verdicts | 10 |
| Discrimination records | 33 |
| Summary | `rfc/short/rfc1350.md` |
| Requirement shard | `rfc/requirements/rfc1350.md` |
| RFC text | `rfc/full/rfc1350.txt` |

## Enrolment

Enrolled: The TFTP Protocol (revision 2), base TFTP read-only server: ten MUST-level requirements. Eight are met -- 2-1 (512-octet blocks with a short block ending the transfer) and 4-1 (block numbers start at 1 and increment) are {single-polarity: positive} since ze is the DATA sender with no reject path; 4-3 (a fresh transfer TID distinct from port 69) and 5-3 (an exact-multiple file ends with a zero-length DATA block) are proven on the server's side positively and on the client's side both ways; 2-2 (lockstep, advance only after ACK), 2-3 (a duplicate ACK triggers no DATA, the Sorcerer's Apprentice fix in sendAndWaitACK), 6-1 (retransmit the last DATA until it is acknowledged) and 7-1 (the ACK timeout ends the transfer of a gone peer) carry positive+negative tags. 5-1 (netascii translation on received data) and 4-2 (WRQ acknowledged with an ACK for block 0) are gaps: ze is a read-only server that rejects WRQ and accepts only octet mode, and no owner decision declines either. Ze's TFTP client (the tftp scheme of ze init --from, internal/core/fetch/tftp.go) meets the SHOULDs 4-4 (DATA from another TID draws ERROR 5 and is not written, TestRFC1350ClientRefusesStrangerTID) and 6-2 (it dallies and acknowledges a resent final block, TestRFC1350ClientDallies). The client also carries the receiver's side of 4-3 (it keeps its own TID and sends nothing more to the request port, TestRFC1350ClientKeepsItsTID), 5-3 (a DATA of 0 to 511 octets ends the transfer and a 512-octet one does not, TestRFC1350ClientDataLengthEndsTheTransfer), 7-1 (a lost request is sent again after a timeout and a silent server is an error, TestRFC1350ClientTimeouts) and 2-3 (every DATA is acknowledged, an ERROR is not), so 4-3 and 5-3 carry both polarities.

## What the public ledger says

**Status:** Partial

**What the ledger says is covered:**

Read-only TFTP server for PXE bootloader delivery.

**What the ledger says remains**

Two MUST gaps in the requirement checklist: ze implements no netascii mode ([`RFC1350-5-1`](#rfc1350-5-1)) and serves no write request ([`RFC1350-4-2`](#rfc1350-4-2)), with no owner decision declining either. RFC1350-5-2, which binds a host that receives an octet file and returns it, is retired because Ze's server never receives a file (it refuses WRQ).

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 6 | one part of the gated population |
| Annotated (including scoped evidence) | 4 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **10** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (6):** [`RFC1350-2-2`](#rfc1350-2-2), [`RFC1350-6-1`](#rfc1350-6-1), [`RFC1350-4-3`](#rfc1350-4-3), [`RFC1350-7-1`](#rfc1350-7-1), [`RFC1350-2-3`](#rfc1350-2-3), [`RFC1350-5-3`](#rfc1350-5-3)

**Annotated (including scoped evidence) (4):** [`RFC1350-2-1`](#rfc1350-2-1), [`RFC1350-5-1`](#rfc1350-5-1), [`RFC1350-4-1`](#rfc1350-4-1), [`RFC1350-4-2`](#rfc1350-4-2)

## Attainable ceiling

Mandatory ceiling: 10 of 10 (100.0%). Optional ceiling: 4 of 4 (100.0%). Every RFC shown with measured rows can reach 100% under current classifications. Lowest mandatory ceiling: rfc1350 10 of 10 (100.0%). Lowest optional ceiling: rfc1350 4 of 4 (100.0%). Ceilings describe current classifications, not an immutable Linux maximum. Not-applicable and feature-declined classifications can be revisited. Every gap remains work Ze owes.

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC1350-2-1` | the file is sent in fixed length blocks of 512 bytes (§2) | MUST | 2 - Overview of the Protocol | **positive:** `unit/verify` [`TestTFTPReadExact512`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L467). **positive:** `unit/verify` [`TestTFTPReadLargeFile`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L388). **negative:** no negative test. **{single-polarity}:** ze is the DATA sender and always frames output at the block size, ending on a short or zero block (handler.go:360, 373, 379), so no client input can make it emit a non-conforming block and there is no reject path for a negative |
| `RFC1350-2-2` | Each data packet contains one block of data, and must be acknowledged by an acknowledgment packet before the next packet can be sent. (Section 2) | MUST | 2 - Overview of the Protocol | **positive:** `unit/verify` [`TestRFC1350LockstepWaitsForTheACK`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc1350_transfer_test.go#L155). **positive:** `unit/verify` [`TestTFTPReadLargeFile`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L391). **negative:** `unit/verify` [`TestRFC1350LockstepWaitsForTheACK`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc1350_transfer_test.go#L157). **negative:** `unit/verify` [`TestTFTPConcurrentLimit`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L722) |
| `RFC1350-6-1` | The host sending the last DATA must retransmit it until the packet is acknowledged or the sending host times out. (Section 6) | MUST | 6 - Normal Termination | **positive:** `unit/verify` [`TestRFC1350LastDATARetransmittedUntilAcknowledged`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc1350_transfer_test.go#L273). **positive:** `unit/verify` [`TestTFTPRetransmitOnTimeout`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L753). **negative:** `unit/verify` [`TestRFC1350LastDATARetransmittedUntilAcknowledged`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc1350_transfer_test.go#L275) |
| `RFC1350-5-1` | A host which receives netascii mode data must translate the data to its own format. (Section 5) | MUST | 5 - TFTP Packets | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze implements no netascii mode and no owner decision declines it: internal/plugins/tftpserver/handler.go refuses any mode but octet (only octet mode supported) and rejects every WRQ (write not supported), and the client internal/core/fetch/tftp.go::parseTFTPURL refuses any mode but octet, so neither side ever receives netascii data to translate |
| `RFC1350-4-1` | block numbers are consecutive and begin with one (§4) | MUST | 4 - Initial Connection Protocol | **positive:** `unit/verify` [`TestTFTPReadLargeFile`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L394). **positive:** `unit/verify` [`TestTFTPReadRequest`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L333). **negative:** no negative test. **{single-polarity}:** the server assigns block numbers itself, starting at 1 and incrementing (handler.go:362, 382), so no input can make it emit a non-1-based or non-consecutive number and there is no reject path for a negative |
| `RFC1350-4-2` | Since the positive response to a write request is an acknowledgment packet, in this special case the block number will be zero. (§4) | MUST | 4 - Initial Connection Protocol | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze serves no write request and no owner decision declines writes: internal/plugins/tftpserver/handler.go answers every WRQ with an Illegal Operation error (write not supported), so no WRQ is acknowledged with block 0 |
| `RFC1350-4-3` | each end of the connection chooses a TID for itself, to be used for the duration of that connection (§4) | MUST | 4 - Initial Connection Protocol | **positive:** `unit/verify` [`TestListenTFTPLoopbackRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc1350_socket_integration_linux_test.go#L153). **positive:** `unit/verify` [`TestRFC1350ClientKeepsItsTID`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc1350_client_test.go#L290). **positive:** `unit/verify` [`TestRFC1350ServerChoosesOneTIDForTheTransfer`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc1350_transfer_test.go#L204). **negative:** `unit/verify` [`TestRFC1350ClientKeepsItsTID`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc1350_client_test.go#L293) |
| `RFC1350-7-1` | Timeouts must also be used to detect errors. (Section 7) | MUST | 7 - Premature Termination | **positive:** `unit/verify` [`TestRFC1350ClientTimeouts`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc1350_client_test.go#L359). **positive:** `unit/verify` [`TestRFC1350TimeoutEndsTheTransferOfAGonePeer`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc1350_transfer_test.go#L304). **positive:** `unit/verify` [`TestTFTPRetransmitOnTimeout`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L756). **negative:** `unit/verify` [`TestRFC1350ClientTimeouts`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc1350_client_test.go#L362). **negative:** `unit/verify` [`TestRFC1350TimeoutThenACKContinuesTheTransfer`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc1350_transfer_test.go#L340) |
| `RFC1350-2-3` | All packets other than duplicate ACK's and those used for termination are acknowledged unless a timeout occurs [4]. Sending a DATA packet is an acknowledgment for the first ACK packet of the previous DATA packet. (§5) | MUST | 5 - TFTP Packets | **positive:** `unit/verify` [`TestRFC1350DuplicateACKTriggersNoDATA`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc1350_transfer_test.go#L180). **positive:** `unit/verify` [`TestRFC7440ClientWindowOneIsLockstep`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc7440_client_test.go#L82). **negative:** `unit/verify` [`TestRFC1350ClientServerError`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc1350_client_test.go#L226). **negative:** `unit/verify` [`TestRFC1350DuplicateACKTriggersNoDATA`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc1350_transfer_test.go#L182) |
| `RFC1350-5-3` | The data field is from zero to 512 bytes long. If it is 512 bytes long, the block is not the last block of data; if it is from zero to 511 bytes long, it signals the end of the transfer. (§5) | MUST | 5 - TFTP Packets | **positive:** `unit/verify` [`TestRFC1350ClientDataLengthEndsTheTransfer`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc1350_client_test.go#L322). **positive:** `unit/verify` [`TestRFC1350DataFieldLengthEndsTheTransfer`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc1350_transfer_test.go#L233). **positive:** `unit/verify` [`TestTFTPReadEmptyFile`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L533). **positive:** `unit/verify` [`TestTFTPReadExact512`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L470). **negative:** `unit/verify` [`TestRFC1350ClientDataLengthEndsTheTransfer`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc1350_client_test.go#L325) |
| `RFC1350-4-4` | If a source TID does not match, the packet should be discarded as erroneously sent from somewhere else. An error packet should be sent to the source of the incorrect packet, while not disturbing the transfer. (§4) | SHOULD | 4 - Initial Connection Protocol | **positive:** `unit/verify` [`TestRFC1350ClientRefusesStrangerTID`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc1350_client_test.go#L22). **negative:** `unit/verify` [`TestRFC1350ClientRefusesStrangerTID`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc1350_client_test.go#L25) |
| `RFC1350-4-5` | The TID's chosen for a connection should be randomly chosen (§4) | SHOULD | 4 - Initial Connection Protocol | **positive:** no positive test. **negative:** no negative test |
| `RFC1350-6-2` | dallying is encouraged. This means that the host sending the final ACK will wait for a while before terminating in order to retransmit the final ACK if it has been lost. (§6) | SHOULD | 6 - Normal Termination | **positive:** `unit/verify` [`TestRFC1350ClientDallies`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc1350_client_test.go#L74). **negative:** `unit/verify` [`TestRFC1350ClientDallies`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc1350_client_test.go#L77) |
| `RFC1350-5-4` | The error message is intended for human consumption, and should be in netascii. (§5) | SHOULD | 5 - TFTP Packets | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC1350-5-1`](#rfc1350-5-1) A host which receives netascii mode data must translate the data to its own format. (Section 5) | {gap}, no test | ze implements no netascii mode and no owner decision declines it: internal/plugins/tftpserver/handler.go refuses any mode but octet (only octet mode supported) and rejects every WRQ (write not supported), and the client internal/core/fetch/tftp.go::parseTFTPURL refuses any mode but octet, so neither side ever receives netascii data to translate |
| [`RFC1350-4-2`](#rfc1350-4-2) Since the positive response to a write request is an acknowledgment packet, in this special case the block number will be zero. (§4) | {gap}, no test | ze serves no write request and no owner decision declines writes: internal/plugins/tftpserver/handler.go answers every WRQ with an Illegal Operation error (write not supported), so no WRQ is acknowledged with block 0 |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC1350-2-1`](#rfc1350-2-1)

the file is sent in fixed length blocks of 512 bytes (§2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-30 by an independent auditor: the handler_test units changed only by identifier renames to internal/core/tftp (opX->tftp.OpX, errX->tftp.CodeX, defaultBlockSize->tftp.DefaultBlockSize), constant values identical, assertions unchanged. Sender-only row (Ze's client never sends DATA). TestTFTPReadExact512 requires block 1 of a 512-byte file to carry exactly 512 bytes and the transfer to continue; TestTFTPReadLargeFile requires three blocks for 1500 bytes. Single-polarity marker holds.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestTFTPReadExact512`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L467) | unit/verify | revert, verified |
| positive | [`TestTFTPReadLargeFile`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L388) | unit/verify | revert, verified |

### [`RFC1350-2-2`](#rfc1350-2-2)

Each data packet contains one block of data, and must be acknowledged by an acknowledgment packet before the next packet can be sent. (Section 2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-30 by an independent auditor: the handler_test units changed only by identifier renames to internal/core/tftp (opX->tftp.OpX, errX->tftp.CodeX, defaultBlockSize->tftp.DefaultBlockSize), constant values identical, assertions unchanged. The operative MUST is the sender waiting for the ACK: TestRFC1350LockstepWaitsForTheACK requires block 1 to be octets 0..511, silence for 1 s with block 1 unacked, then block 2 = octets 512..1023 after ACK 1. The receiver's duty to ACK every DATA is RFC1350-2-3's subject.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestTFTPConcurrentLimit`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L722) | unit/verify | mutant, verified |
| negative | [`TestRFC1350LockstepWaitsForTheACK`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc1350_transfer_test.go#L157) | unit/verify | revert, verified |
| positive | [`TestTFTPReadLargeFile`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L391) | unit/verify | revert, verified |
| positive | [`TestRFC1350LockstepWaitsForTheACK`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc1350_transfer_test.go#L155) | unit/verify | revert, verified |

### [`RFC1350-6-1`](#rfc1350-6-1)

The host sending the last DATA must retransmit it until the packet is acknowledged or the sending host times out. (Section 6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-30 by an independent auditor: the handler_test units changed only by identifier renames to internal/core/tftp (opX->tftp.OpX, errX->tftp.CodeX, defaultBlockSize->tftp.DefaultBlockSize), constant values identical, assertions unchanged. Sender-only row (the last DATA's sender; Ze's client never sends DATA). TestRFC1350LastDATARetransmittedUntilAcknowledged: the unacked last DATA is retransmitted byte for byte within ackTimeout+2 s, and after ACK 2 silence for ackTimeout+2 s.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1350LastDATARetransmittedUntilAcknowledged`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc1350_transfer_test.go#L275) | unit/verify | revert, verified |
| positive | [`TestTFTPRetransmitOnTimeout`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L753) | unit/verify | revert, verified |
| positive | [`TestRFC1350LastDATARetransmittedUntilAcknowledged`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc1350_transfer_test.go#L273) | unit/verify | revert, verified |

### [`RFC1350-5-1`](#rfc1350-5-1)

A host which receives netascii mode data must translate the data to its own format. (Section 5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1350-5-1, so no unit is bound to it.

### [`RFC1350-4-1`](#rfc1350-4-1)

block numbers are consecutive and begin with one (§4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-30 by an independent auditor: the handler_test units changed only by identifier renames to internal/core/tftp (opX->tftp.OpX, errX->tftp.CodeX, defaultBlockSize->tftp.DefaultBlockSize), constant values identical, assertions unchanged. The numbering is the sender's: TestTFTPReadRequest asserts block 1 first, TestTFTPReadLargeFile asserts 1,2,3. Single-polarity marker holds for the server.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestTFTPReadLargeFile`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L394) | unit/verify | revert, verified |
| positive | [`TestTFTPReadRequest`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L333) | unit/verify | revert, verified |

### [`RFC1350-4-2`](#rfc1350-4-2)

Since the positive response to a write request is an acknowledgment packet, in this special case the block number will be zero. (§4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1350-4-2, so no unit is bound to it.

### [`RFC1350-4-3`](#rfc1350-4-3)

each end of the connection chooses a TID for itself, to be used for the duration of that connection (§4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Both ends are now tagged. Client: tftpPeer.read fails the test on any client packet whose source address or port differs from the RRQ's, so every client test checks that the client keeps its TID. TestRFC1350ClientKeepsItsTID also requires that nothing more reaches the request port for 300 ms once the server answered from its own TID. Server: TestListenTFTPLoopbackRoundTrip and TestRFC1350ServerChoosesOneTIDForTheTransfer are unchanged in substance; handleRRQ and runTransfer changed only by identifier renames.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1350ClientKeepsItsTID`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc1350_client_test.go#L293) | unit/verify | revert, verified |
| positive | [`TestRFC1350ClientKeepsItsTID`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc1350_client_test.go#L290) | unit/verify | revert, verified |
| positive | [`TestListenTFTPLoopbackRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc1350_socket_integration_linux_test.go#L153) | unit/verify | mutant, verified |
| positive | [`TestRFC1350ServerChoosesOneTIDForTheTransfer`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc1350_transfer_test.go#L204) | unit/verify | revert, verified |

### [`RFC1350-7-1`](#rfc1350-7-1)

Timeouts must also be used to detect errors. (Section 7)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Client now tagged. TestRFC1350ClientTimeouts, tftpTimeout 200 ms: a lost first RRQ must be sent again after one timeout, and the transfer the second RRQ starts must complete with 'ok'. A server that never answers must fail the fetch with 'no answer' rather than wait forever (bounded by tftpRetransmitMax). Server half unchanged: TestRFC1350TimeoutEndsTheTransferOfAGonePeer and TestRFC1350TimeoutThenACKContinuesTheTransfer.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1350ClientTimeouts`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc1350_client_test.go#L362) | unit/verify | revert, verified |
| negative | [`TestRFC1350TimeoutThenACKContinuesTheTransfer`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc1350_transfer_test.go#L340) | unit/verify | revert, verified |
| positive | [`TestRFC1350ClientTimeouts`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc1350_client_test.go#L359) | unit/verify | revert, verified |
| positive | [`TestTFTPRetransmitOnTimeout`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L756) | unit/verify | revert, verified |
| positive | [`TestRFC1350TimeoutEndsTheTransferOfAGonePeer`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc1350_transfer_test.go#L304) | unit/verify | revert, verified |

### [`RFC1350-2-3`](#rfc1350-2-3)

All packets other than duplicate ACK's and those used for termination are acknowledged unless a timeout occurs [4]. Sending a DATA packet is an acknowledgment for the first ACK packet of the previous DATA packet. (§5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Receiver half now tagged. Positive: TestRFC7440ClientWindowOneIsLockstep, where every DATA must draw its own ACK within tftpTimeout/2, before the server sends the next one. Negative: TestRFC1350ClientServerError, where an ERROR (a termination packet) must draw nothing for 300 ms and the fetch must fail naming code 1 and its message. Sender half unchanged: TestRFC1350DuplicateACKTriggersNoDATA, where a duplicate ACK draws no DATA for 1 s and a fresh ACK draws the next block.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1350ClientServerError`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc1350_client_test.go#L226) | unit/verify | revert, verified |
| negative | [`TestRFC1350DuplicateACKTriggersNoDATA`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc1350_transfer_test.go#L182) | unit/verify | revert, verified |
| positive | [`TestRFC7440ClientWindowOneIsLockstep`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc7440_client_test.go#L82) | unit/verify | revert, verified |
| positive | [`TestRFC1350DuplicateACKTriggersNoDATA`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc1350_transfer_test.go#L180) | unit/verify | revert, verified |

### [`RFC1350-5-3`](#rfc1350-5-3)

The data field is from zero to 512 bytes long. If it is 512 bytes long, the block is not the last block of data; if it is from zero to 511 bytes long, it signals the end of the transfer. (§5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Receiver half now tagged with both polarities. TestRFC1350ClientDataLengthEndsTheTransfer, no option acknowledged: a 511-octet block, an empty block after 512, and an empty first block must each end the file. A block sent after the end draws nothing for 300 ms and is not written, so the file must be exact. A 512-octet block must draw its ACK and the next block. Sender half unchanged: TestRFC1350DataFieldLengthEndsTheTransfer and the renamed-only handler_test units.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1350ClientDataLengthEndsTheTransfer`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc1350_client_test.go#L325) | unit/verify | revert, verified |
| positive | [`TestRFC1350ClientDataLengthEndsTheTransfer`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc1350_client_test.go#L322) | unit/verify | revert, verified |
| positive | [`TestTFTPReadEmptyFile`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L533) | unit/verify | revert, verified |
| positive | [`TestTFTPReadExact512`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L470) | unit/verify | revert, verified |
| positive | [`TestRFC1350DataFieldLengthEndsTheTransfer`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc1350_transfer_test.go#L233) | unit/verify | revert, verified |

### [`RFC1350-4-4`](#rfc1350-4-4)

If a source TID does not match, the packet should be discarded as erroneously sent from somewhere else. An error packet should be sent to the source of the incorrect packet, while not disturbing the transfer. (§4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRFC1350ClientRefusesStrangerTID: mid-transfer, DATA block 2 'intruder' (8 octets, so it would be final) from another loopback port must draw ERROR code exactly 5 back to that port, and the transfer with the server's TID must continue to ACK 2 with the file holding only the server's 532 octets. A client that accepted the stranger ends on 'intruder' (red); one that only discarded it gets no ERROR (red). Only a different port is exercised, not a different address; fromPeer compares both.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1350ClientRefusesStrangerTID`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc1350_client_test.go#L25) | unit/verify | revert, verified |
| positive | [`TestRFC1350ClientRefusesStrangerTID`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc1350_client_test.go#L22) | unit/verify | revert, verified |

### [`RFC1350-6-2`](#rfc1350-6-2)

dallying is encouraged. This means that the host sending the final ACK will wait for a while before terminating in order to retransmit the final ACK if it has been lost. (§6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRFC1350ClientDallies now sets the new tftpDally (1 s) instead of tftpTimeout; the assertions are unchanged. After the final ACK 2, a resent final DATA 2 must draw ACK 2 again within tftpTimeout/2, so a client that exits at once is red. A resent older block 1 must draw nothing for 200 ms, so a dally that ACKs anything is red.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1350ClientDallies`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc1350_client_test.go#L77) | unit/verify | revert, verified |
| positive | [`TestRFC1350ClientDallies`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc1350_client_test.go#L74) | unit/verify | revert, verified |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | independent judge, spec-rfc-verdict-fix-services (session 869df689) |
| Signed off | 2026-09-30 |
| Register | prose |
| Source | rfc/full/rfc1350.txt |
| Source fingerprint | 37aca32d5dfaf1a8 |
| Record | rfc/extraction/rfc1350.json |
| Mapped sentences | 4 |
| Declined as scope | 6 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | skipped (front-matter) | Title block, Status of this Memo, Summary and Acknowlegements. The Summary restates the protocol in one paragraph and the Acknowlegements name the authors of the 1992 revision and the Sorcerer's Apprentice fix. Neither directs a TFTP speaker. |
| `1` | Purpose | 0 | walked | Purpose. Says what TFTP is, that it runs over UDP, that it lacks directory listing and user authentication, and that it passes 8-bit bytes. It names the three transfer modes: netascii, octet and mail. The one directive-shaped sentence is 'The mail mode is obsolete and should not be implemented or used', which is advisory and states no gated obligation; the mail-mode obligation itself is site 5:3, excluded below. Ze accepts only octet mode (parseRRQ callers reject any other mode in handleRRQ, internal/plugins/tftpserver/handler.go). No gated requirement of rfc/short/rfc1350.md is read from this section. |
| `2` | Overview of the Protocol | 1 | walked | Overview of the Protocol. Its one modal sentence is site 2:1, the lockstep obligation, mapped below. The rest is indicative and states two obligations rfc/short/rfc1350.md gates with no modal behind them. 'the connection is opened and the file is sent in fixed length blocks of 512 bytes' is where RFC1350-2-1 is read from, declared unsourced below. 'A data packet of less than 512 bytes signals termination of a transfer' is the framing rule RFC1350-5-3 turns into the exact-multiple terminator; the id names section 5, so it is declared there. The paragraph on error handling ('Most errors cause termination of the connection', 'TFTP recognizes only one error condition that does not cause termination, the source port of a received packet being incorrect') is indicative and its two obligations are stated normatively in sections 4 and 7. The duplicate-ACK exception this section's retransmission paragraph implies is stated in section 5 and RFC1350-2-3 is declared there. |
| `3` | Relation to other Protocols | 1 | walked | Relation to other Protocols. Describes the header stack (local medium, Internet, Datagram, TFTP) and says TFTP specifies no value in the Internet header while the Datagram source and destination ports carry the TIDs. Its one modal sentence is site 3:1, excluded below as a description of the datagram layer's port range. No gated requirement of rfc/short/rfc1350.md is read from this section. |
| `4` | Initial Connection Protocol | 0 | walked | Initial Connection Protocol. The section that carries the most gated obligations of the document and states every one of them in the indicative, so the modal scan sees none and derives zero sites here. 'Each data packet has associated with it a block number; block numbers are consecutive and begin with one' is RFC1350-4-1. 'Since the positive response to a write request is an acknowledgment packet, in this special case the block number will be zero' is RFC1350-4-2. 'In order to create a connection, each end of the connection chooses a TID for itself, to be used for the duration of that connection' is RFC1350-4-3. All three are declared unsourced below. The remaining normative material is advisory and ungated: the TIDs 'should be randomly chosen' (RFC1350-4-5) and a packet whose source TID does not match 'should be discarded' with an error packet sent to the wrong source 'while not disturbing the transfer' (RFC1350-4-4). The write-establishment example and the duplicated-request narrative that closes the section are worked examples and direct nobody. |
| `5` | TFTP Packets | 5 | walked | TFTP Packets. The wire-format section and the largest site cluster: five of the document's ten modal sentences sit here. Two are mapped (5:1 netascii translation, 5:2 octet round-trip identity) and three are excluded (5:3 mail mode, 5:4 the DEC-20 special-mode example, 5:5 the caution on defining new modes). Two further gated rows are read from indicative sentences of this section and declared unsourced below. RFC1350-5-3, the zero-length final DATA block, is read from 'The data field is from zero to 512 bytes long. If it is 512 bytes long, the block is not the last block of data; if it is from zero to 511 bytes long, it signals the end of the transfer.' RFC1350-2-3, the Sorcerer's Apprentice fix, is read from 'All packets other than duplicate ACK's and those used for termination are acknowledged unless a timeout occurs [4].' That sentence is the only place RFC 1350 states the duplicate-ACK exception the 1992 revision was written to add, and it carries no modal; the id names section 2 because rfc/short/rfc1350.md cites the overview and RFC 1123 Section 4.2, so the row is homed here on the section the sentence is in rather than on the section its id spells. The opcode table, the four packet figures, the case-insensitive mode string, the block-number and data-length rules of the DATA figure, and the ERROR packet's human-readable message (advisory, RFC1350-5-4) complete the section. |
| `6` | Normal Termination | 1 | walked | Normal Termination. Its one modal sentence is site 6:1, the last-DATA retransmission obligation, mapped below. The rest is the dallying paragraph, advisory and carried by RFC1350-6-2, plus the indicative statement that the end of a transfer is marked by a DATA packet of 0 to 511 bytes, which corroborates RFC1350-5-3 declared on section 5. |
| `7` | Premature Termination | 1 | walked | Premature Termination. Two sentences. The ERROR packet is 'only a courtesy since it will not be retransmitted or acknowledged', which is indicative, and 'Timeouts must also be used to detect errors', which is site 7:1, mapped below. |
| `I` | not stated | 1 | walked | Appendix, and everything the derivation folds under it: the header order figure, the four packet format figures, the read-establishment example, the error code table (values 0 to 7), the UDP header reproduced for convenience, the References, Security Considerations and the Author's Address. The figures and tables restate section 5's formats and section 4's establishment steps and direct nobody; the UDP header is reproduced with the RFC's own note that 'TFTP need not be implemented on top of the Internet User Datagram Protocol'. The one modal sentence is site I:1, in Security Considerations, excluded below as binding the administrator who grants rights to the server process. No gated requirement of rfc/short/rfc1350.md is read from this section. |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `3:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | A description of the datagram layer, not a directive to a TFTP speaker. The sentence derives a range from a property of the protocol underneath it: TIDs are handed to UDP to be used as ports, 'therefore they must be between 0 and 65,535'. The bound is the width of the UDP port field, so no TFTP implementation can violate it and none can be tested against it. Ze obtains its transfer TID from net.DialUDP in handleRRQ (internal/plugins/tftpserver/handler.go) and never chooses a port number itself. rfc/short/rfc1350.md declares no requirement for this sentence, and the TID obligation it does declare, RFC1350-4-3, is read from section 4 and declared unsourced there. | The transfer identifiers (TID's) used by TFTP are passed to the Datagram layer to be used as ports; therefore they must be between 0 and 65,535. |
| `5:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | binds a host that receives an octet file and then returns it, which Ze does not implement: the TFTP server refuses WRQ (serve in internal/plugins/tftpserver/handler.go) and Ze has no TFTP client, so it never receives a file. Row RFC1350-5-2 is retired (rfc/corrections/rfc1350.md) | If a host receives a octet file and then returns it, the returned file must be identical to the original. |
| `5:3` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the mail-mode role, which RFC 1350 retires in its own section 1: 'mail, netascii characters sent to a user rather than a file. (The mail mode is obsolete and should not be implemented or used.)' The sentence tells a host that offers mail mode that such a transfer begins with a WRQ and names a recipient in place of a file. Ze plays no mail-mode role at all: handleRRQ accepts only 'octet' and the server rejects WRQ outright in serve (internal/plugins/tftpserver/handler.go). rfc/short/rfc1350.md declares no requirement for mail mode. | Mail mode uses the name of a mail recipient in place of a file and must begin with a WRQ. |
| `5:4` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | A worked example, not a directive. The sentence sits inside the DEC-20 narrative the section opens with 'One might create a special mode for such a machine which read all the bits in a word, but in which the receiver stored the information in 8-bit format', and the RFC closes the narrative by saying 'No such machine or application specific modes have been specified in TFTP'. The 'must' describes what such a hypothetical mode would need in order to be useful, and binds no speaker of the protocol as specified. rfc/short/rfc1350.md declares no requirement for it. | When such a file is retrieved from the storage site, it must be restored to its original form to be useful, so the reverse mode must also be implemented. |
| `5:5` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | A caution attached to a permission, naming no observable behavior. The enclosing construction is 'It is also possible to define other modes for cooperating pairs of hosts, although this must be done with care', and the two sentences after it say 'There is no requirement that any other hosts implement these. There is no central authority that will define these modes or assign them names.' Care is not a wire behavior a test can assert or a decoder can violate, and the RFC itself says the sentence creates no requirement. rfc/short/rfc1350.md declares none for it. | It is also possible to define other modes for cooperating pairs of hosts, although this must be done with care. |
| `I:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the system administrator who grants file system rights to the TFTP server process, a role RFC 1350 names in the sentence itself: 'care must be taken in the rights granted to a TFTP server process'. The obligation is a deployment decision made outside the protocol and outside ze, and the sentence after it describes the common deployment rather than requiring one: 'TFTP is often installed with controls such that only files that have public read access are available via TFTP and writing files via TFTP is disallowed.' Ze confines every transfer to the configured root through resolvePath (internal/plugins/tftpserver/handler.go) and serves reads only, which is the posture the sentence recommends to that administrator, but the rights on the files themselves are not ze's to grant. rfc/short/rfc1350.md declares no requirement for it. | Since TFTP includes no login or access control mechanisms, care must be taken in the rights granted to a TFTP server process so as not to violate the security of the server hosts file system. |

## Superseded

No document obsoletes RFC 1350, so its obligations are stated where they were written.
