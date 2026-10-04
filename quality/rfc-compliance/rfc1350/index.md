# RFC 1350 - The TFTP Protocol (Revision 2)

Supported. Every requirement this repository extracted from RFC 1350, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 40.0% | 4 of 10 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 40.0% | 4 of 10 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 10 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 10 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| No test at all | 0.0% | 0 of 10 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 81.0% | 17 of 21 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 10 | of 14 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 2 | of 10 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 20.0% | 2 of 10 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 10 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 10 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

The 8 shares marked as a part above are the whole of the 10 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Public status | Supported |
| Enrolment | Enrolled |
| Requirements | 14 |
| Gated MUST-level | 10 |
| Not applicable, so out of scope | 2 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 21 |
| Tagged units | 21 |
| Recorded audit verdicts | 8 |
| Discrimination records | 17 |
| Summary | `rfc/short/rfc1350.md` |
| Requirement shard | `rfc/requirements/rfc1350.md` |
| RFC text | `rfc/full/rfc1350.txt` |

## Enrolment

Enrolled: The TFTP Protocol (revision 2), base TFTP read-only server: ten MUST-level requirements. Eight are met -- 2-1 (512-octet blocks with a short block ending the transfer), 4-1 (block numbers start at 1 and increment), 4-3 (a fresh transfer TID distinct from port 69), and 5-3 (an exact-multiple file ends with a zero-length DATA block) are {single-polarity: positive} since ze is the DATA sender with no reject path; 2-2 (lockstep, advance only after ACK), 2-3 (a duplicate ACK triggers no DATA, the Sorcerer's Apprentice fix in sendAndWaitACK), 6-1 (retransmit the last DATA until it is acknowledged) and 7-1 (the ACK timeout ends the transfer of a gone peer) carry positive+negative tags. 5-1 (netascii translation on received data) and 4-2 (WRQ acknowledged with an ACK for block 0) are {not-applicable}: ze is a read-only server that rejects WRQ and accepts only octet mode.

## What the public ledger says

**Status:** Supported

**What the ledger says is covered:**

Read-only TFTP server for PXE bootloader delivery.

**What the ledger says remains:**

None: RFC1350-5-2, which binds a host that receives an octet file and returns it, is retired because Ze's server never receives a file (it refuses WRQ).

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 4 | one part of the gated population |
| Annotated (including scoped evidence) | 6 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **10** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (4):** [`RFC1350-2-2`](#rfc1350-2-2), [`RFC1350-6-1`](#rfc1350-6-1), [`RFC1350-7-1`](#rfc1350-7-1), [`RFC1350-2-3`](#rfc1350-2-3)

**Annotated (including scoped evidence) (6):** [`RFC1350-2-1`](#rfc1350-2-1), [`RFC1350-5-1`](#rfc1350-5-1), [`RFC1350-4-1`](#rfc1350-4-1), [`RFC1350-4-2`](#rfc1350-4-2), [`RFC1350-4-3`](#rfc1350-4-3), [`RFC1350-5-3`](#rfc1350-5-3)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC1350-2-1` | the file is sent in fixed length blocks of 512 bytes (§2) | MUST | 2 - Overview of the Protocol | **positive:** `unit/verify` [`TestTFTPReadExact512`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L466). **positive:** `unit/verify` [`TestTFTPReadLargeFile`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L387). **negative:** no negative test. **{single-polarity}:** ze is the DATA sender and always frames output at the block size, ending on a short or zero block (handler.go:360, 373, 379), so no client input can make it emit a non-conforming block and there is no reject path for a negative |
| `RFC1350-2-2` | Each data packet contains one block of data, and must be acknowledged by an acknowledgment packet before the next packet can be sent. (Section 2) | MUST | 2 - Overview of the Protocol | **positive:** `unit/verify` [`TestRFC1350LockstepWaitsForTheACK`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc1350_transfer_test.go#L153). **positive:** `unit/verify` [`TestTFTPReadLargeFile`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L390). **negative:** `unit/verify` [`TestRFC1350LockstepWaitsForTheACK`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc1350_transfer_test.go#L155). **negative:** `unit/verify` [`TestTFTPConcurrentLimit`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L721) |
| `RFC1350-6-1` | The host sending the last DATA must retransmit it until the packet is acknowledged or the sending host times out. (Section 6) | MUST | 6 - Normal Termination | **positive:** `unit/verify` [`TestRFC1350LastDATARetransmittedUntilAcknowledged`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc1350_transfer_test.go#L271). **positive:** `unit/verify` [`TestTFTPRetransmitOnTimeout`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L752). **negative:** `unit/verify` [`TestRFC1350LastDATARetransmittedUntilAcknowledged`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc1350_transfer_test.go#L273) |
| `RFC1350-5-1` | A host which receives netascii mode data must translate the data to its own format. (Section 5) | MUST | 5 - TFTP Packets | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze is a read-only TFTP server that rejects WRQ (internal/plugins/tftpserver/handler.go:227) and accepts only octet mode (handler.go:248), so it never receives file data to translate between netascii and its local format |
| `RFC1350-4-1` | block numbers are consecutive and begin with one (§4) | MUST | 4 - Initial Connection Protocol | **positive:** `unit/verify` [`TestTFTPReadLargeFile`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L393). **positive:** `unit/verify` [`TestTFTPReadRequest`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L332). **negative:** no negative test. **{single-polarity}:** the server assigns block numbers itself, starting at 1 and incrementing (handler.go:362, 382), so no input can make it emit a non-1-based or non-consecutive number and there is no reject path for a negative |
| `RFC1350-4-2` | Since the positive response to a write request is an acknowledgment packet, in this special case the block number will be zero. (§4) | MUST | 4 - Initial Connection Protocol | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze serves reads only and rejects WRQ with an ERROR (internal/plugins/tftpserver/handler.go:227-230), so there is no WRQ-to-ACK-block-0 write-initiation path |
| `RFC1350-4-3` | each end of the connection chooses a TID for itself, to be used for the duration of that connection (§4) | MUST | 4 - Initial Connection Protocol | **positive:** `unit/verify` [`TestListenTFTPLoopbackRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc1350_socket_integration_linux_test.go#L144). **positive:** `unit/verify` [`TestRFC1350ServerChoosesOneTIDForTheTransfer`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc1350_transfer_test.go#L202). **negative:** no negative test. **{single-polarity}:** the server always allocates a fresh transfer socket via net.DialUDP per RRQ (handler.go:287), so no configuration or input reuses port 69 and there is no stale-TID negative case |
| `RFC1350-7-1` | Timeouts must also be used to detect errors. (Section 7) | MUST | 7 - Premature Termination | **positive:** `unit/verify` [`TestRFC1350TimeoutEndsTheTransferOfAGonePeer`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc1350_transfer_test.go#L302). **positive:** `unit/verify` [`TestTFTPRetransmitOnTimeout`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L755). **negative:** `unit/verify` [`TestRFC1350TimeoutThenACKContinuesTheTransfer`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc1350_transfer_test.go#L338) |
| `RFC1350-2-3` | All packets other than duplicate ACK's and those used for termination are acknowledged unless a timeout occurs [4]. Sending a DATA packet is an acknowledgment for the first ACK packet of the previous DATA packet. (§5) | MUST | 5 - TFTP Packets | **positive:** `unit/verify` [`TestRFC1350DuplicateACKTriggersNoDATA`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc1350_transfer_test.go#L178). **negative:** `unit/verify` [`TestRFC1350DuplicateACKTriggersNoDATA`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc1350_transfer_test.go#L180) |
| `RFC1350-5-3` | The data field is from zero to 512 bytes long. If it is 512 bytes long, the block is not the last block of data; if it is from zero to 511 bytes long, it signals the end of the transfer. (§5) | MUST | 5 - TFTP Packets | **positive:** `unit/verify` [`TestRFC1350DataFieldLengthEndsTheTransfer`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc1350_transfer_test.go#L231). **positive:** `unit/verify` [`TestTFTPReadEmptyFile`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L532). **positive:** `unit/verify` [`TestTFTPReadExact512`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L469). **negative:** no negative test. **{single-polarity}:** ze is the sender and always appends the zero-length terminator when the file length is an exact multiple of the block size (handler.go:364-383, 379), so no input can suppress or misplace it and there is no reject-path negative for this framing requirement |
| `RFC1350-4-4` | If a source TID does not match, the packet should be discarded as erroneously sent from somewhere else. An error packet should be sent to the source of the incorrect packet, while not disturbing the transfer. (§4) | SHOULD | 4 - Initial Connection Protocol | **positive:** no positive test. **negative:** no negative test |
| `RFC1350-4-5` | The TID's chosen for a connection should be randomly chosen (§4) | SHOULD | 4 - Initial Connection Protocol | **positive:** no positive test. **negative:** no negative test |
| `RFC1350-6-2` | dallying is encouraged. This means that the host sending the final ACK will wait for a while before terminating in order to retransmit the final ACK if it has been lost. (§6) | SHOULD | 6 - Normal Termination | **positive:** no positive test. **negative:** no negative test |
| `RFC1350-5-4` | The error message is intended for human consumption, and should be in netascii. (§5) | SHOULD | 5 - TFTP Packets | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC1350-5-1`](#rfc1350-5-1) A host which receives netascii mode data must translate the data to its own format. (Section 5) | no test | no test carries this requirement id; annotated {not-applicable}: ze is a read-only TFTP server that rejects WRQ (internal/plugins/tftpserver/handler.go:227) and accepts only octet mode (handler.go:248), so it never receives file data to translate between netascii and its local format |
| [`RFC1350-4-2`](#rfc1350-4-2) Since the positive response to a write request is an acknowledgment packet, in this special case the block number will be zero. (§4) | no test | no test carries this requirement id; annotated {not-applicable}: ze serves reads only and rejects WRQ with an ERROR (internal/plugins/tftpserver/handler.go:227-230), so there is no WRQ-to-ACK-block-0 write-initiation path |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC1350-2-1`](#rfc1350-2-1)

the file is sent in fixed length blocks of 512 bytes (§2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-30: the units changed only by dropping the retired RFC1350-5-2 tag comments; every assertion is unchanged. Forbidden: a non-final DATA block that is not 512 bytes. TestTFTPReadExact512 asserts block 1 of a 512-byte file carries exactly 512 bytes (n-4 != 512 fails) and that the transfer continues to block 2; TestTFTPReadLargeFile asserts three blocks for 1500 bytes. Single-polarity marker: ze is the sender and no input changes its framing.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestTFTPReadExact512`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L466) | unit/verify | unproven |
| positive | [`TestTFTPReadLargeFile`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L387) | unit/verify | unproven |

### [`RFC1350-2-2`](#rfc1350-2-2)

Each data packet contains one block of data, and must be acknowledged by an acknowledgment packet before the next packet can be sent. (Section 2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-30: the units changed only by dropping the retired RFC1350-5-2 tag comments; every assertion is unchanged. Forbidden: DATA block 2 before block 1 is ACKed. TestRFC1350LockstepWaitsForTheACK requires block 1 to be exactly octets 0..511, then expectSilence for 1 s with block 1 unacked (red on a server that streams ahead; the ACK timeout is 5 s so no retransmission confounds it), then after ACK 1 block 2 must be octets 512..1023. Both clauses (one block per packet, ACK before the next) are asserted in both polarities. The older handler_test units stay weak on their own.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestTFTPConcurrentLimit`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L721) | unit/verify | revert, verified |
| negative | [`TestRFC1350LockstepWaitsForTheACK`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc1350_transfer_test.go#L155) | unit/verify | revert, verified |
| positive | [`TestTFTPReadLargeFile`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L390) | unit/verify | revert, verified |
| positive | [`TestRFC1350LockstepWaitsForTheACK`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc1350_transfer_test.go#L153) | unit/verify | revert, verified |

### [`RFC1350-6-1`](#rfc1350-6-1)

The host sending the last DATA must retransmit it until the packet is acknowledged or the sending host times out. (Section 6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRFC1350LastDATARetransmittedUntilAcknowledged: the last DATA (block 2 of 1000 octets) left unacked must be retransmitted byte for byte within ackTimeout+2 s (red on no retransmission); after ACK 2, expectSilence for ackTimeout+2 s (red on retransmitting past the ACK, the 'until acknowledged' clause the old verdict found missing). The 'or times out' end is the sender's give-up, asserted by the RFC1350-7-1 unit on a single-block (last) DATA.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1350LastDATARetransmittedUntilAcknowledged`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc1350_transfer_test.go#L273) | unit/verify | revert, verified |
| positive | [`TestTFTPRetransmitOnTimeout`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L752) | unit/verify | revert, verified |
| positive | [`TestRFC1350LastDATARetransmittedUntilAcknowledged`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc1350_transfer_test.go#L271) | unit/verify | revert, verified |

### [`RFC1350-5-1`](#rfc1350-5-1)

A host which receives netascii mode data must translate the data to its own format. (Section 5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1350-5-1, so no unit is bound to it.

### [`RFC1350-4-1`](#rfc1350-4-1)

block numbers are consecutive and begin with one (§4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-30: the units changed only by dropping the retired RFC1350-5-2 tag comments; every assertion is unchanged. Forbidden: a first block other than 1, or a gap or repeat in the numbering. TestTFTPReadRequest asserts buf[2:4] == 1; TestTFTPReadLargeFile asserts gotBlock == block for 1,2,3; TestTFTPReadExact512 asserts block 2 follows block 1. Single-polarity marker: the server assigns the numbers.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestTFTPReadLargeFile`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L393) | unit/verify | unproven |
| positive | [`TestTFTPReadRequest`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L332) | unit/verify | unproven |

### [`RFC1350-4-2`](#rfc1350-4-2)

Since the positive response to a write request is an acknowledgment packet, in this special case the block number will be zero. (§4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1350-4-2, so no unit is bound to it.

### [`RFC1350-4-3`](#rfc1350-4-3)

each end of the connection chooses a TID for itself, to be used for the duration of that connection (§4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: answering from the request port, or changing TID mid-transfer. TestRFC1350ServerChoosesOneTIDForTheTransfer reads the source of each of three DATA blocks and fails if any equals the listener port or differs from block 1's port. Single-polarity marker holds: the server has no input-driven refusal path; a server reusing port 69 or re-dialing per block goes red on the positive.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestListenTFTPLoopbackRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc1350_socket_integration_linux_test.go#L144) | unit/verify | revert, verified |
| positive | [`TestRFC1350ServerChoosesOneTIDForTheTransfer`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc1350_transfer_test.go#L202) | unit/verify | revert, verified |

### [`RFC1350-7-1`](#rfc1350-7-1)

Timeouts must also be used to detect errors. (Section 7)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Positive TestRFC1350TimeoutEndsTheTransferOfAGonePeer: with no ACK, block 1 must arrive exactly maxRetransmit+1 times then silence past a full ackTimeout+2 s (red on retransmitting forever or on no timeout), and a one-slot server must then serve a new RRQ (red if the failed transfer never releases its slot). Negative TestRFC1350TimeoutThenACKContinuesTheTransfer: an ACK after one timeout-driven retransmission continues to block 2 (red on treating the first timeout as fatal). The old single-polarity test-cost argument is gone from the row.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1350TimeoutThenACKContinuesTheTransfer`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc1350_transfer_test.go#L338) | unit/verify | revert, verified |
| positive | [`TestTFTPRetransmitOnTimeout`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L755) | unit/verify | revert, verified |
| positive | [`TestRFC1350TimeoutEndsTheTransferOfAGonePeer`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc1350_transfer_test.go#L302) | unit/verify | revert, verified |

### [`RFC1350-2-3`](#rfc1350-2-3)

All packets other than duplicate ACK's and those used for termination are acknowledged unless a timeout occurs [4]. Sending a DATA packet is an acknowledgment for the first ACK packet of the previous DATA packet. (§5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRFC1350DuplicateACKTriggersNoDATA: after ACK 1 and DATA 2, a second ACK 1 must produce no datagram for 1 s (the ACK timeout is 5 s, so a timeout retransmission cannot confound it); judge re-ran it against HEAD's handler.go by overlay and it fails 'DATA sent in answer to a duplicate ACK: got opcode 3 block 2', so it discriminates the Sorcerer's Apprentice bug. Positive: a fresh ACK 2 is answered by DATA 3, and after ACK 3 silence, so block 3 is sent once. Producer sendAndWaitACK now keeps reading to the same deadline on a non-matching packet (isACKFor).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1350DuplicateACKTriggersNoDATA`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc1350_transfer_test.go#L180) | unit/verify | revert, verified |
| positive | [`TestRFC1350DuplicateACKTriggersNoDATA`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc1350_transfer_test.go#L178) | unit/verify | revert, verified |

### [`RFC1350-5-3`](#rfc1350-5-3)

The data field is from zero to 512 bytes long. If it is 512 bytes long, the block is not the last block of data; if it is from zero to 511 bytes long, it signals the end of the transfer. (§5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRFC1350DataFieldLengthEndsTheTransfer reads into a 64 KiB buffer so an oversize field is seen whole (fails >512); for 1500/1024/0-octet files a 512 block must be followed by the next block (expectData fails otherwise), the transfer must end on the first 0..511 block with expectSilence for 1 s after its ACK (red on DATA after the end), and the reassembled bytes equal the file (red on a short non-final block, which would end the loop early). All three clauses asserted; single-polarity marker holds (Ze is the sender).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestTFTPReadEmptyFile`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L532) | unit/verify | revert, verified |
| positive | [`TestTFTPReadExact512`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L469) | unit/verify | revert, verified |
| positive | [`TestRFC1350DataFieldLengthEndsTheTransfer`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc1350_transfer_test.go#L231) | unit/verify | revert, verified |

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
