# RFC 1350 - The TFTP Protocol (Revision 2)

Supported. Every requirement this repository extracted from RFC 1350, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 11.1% | 1 of 9 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 77.8% | 7 of 9 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 9 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 0.0% | 0 of 13 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 11 | of 15 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 2 | of 11 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 11.1% | 1 of 9 binding obligations | no test carries the requirement id, whether or not a gap states why |

The 4 shares marked as a part above are the whole of the 9 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| No test at all | bad | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Supported |
| Enrolment | Enrolled |
| Requirements | 15 |
| Gated MUST-level | 11 |
| Obligations that bind Ze | 9 |
| Not applicable, so out of scope | 2 |
| Declared gaps | 1 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 13 |
| Tagged units | 13 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc1350.md` |
| Requirement shard | `rfc/requirements/rfc1350.md` |
| RFC text | `rfc/full/rfc1350.txt` |

## Enrolment

Enrolled: The TFTP Protocol (revision 2), base TFTP read-only server: eleven MUST-level requirements. Eight are met -- 2-1 (512-octet blocks with a short block ending the transfer), 6-1 (retransmit on timeout), 5-2 (octet mode transfers bytes unchanged), 4-1 (block numbers start at 1 and increment), 4-3 (a fresh transfer TID distinct from port 69), 7-1 (timeout then abort), and 5-3 (an exact-multiple file ends with a zero-length DATA block) are {single-polarity: positive} since ze is the DATA sender with no reject path; 2-2 (lockstep, advance only after ACK) carries positive+negative tags. 2-3 (silently ignore a duplicate or stale ACK per the Sorcerer's Apprentice fix) is {gap}: ze's sendAndWaitACK retransmits on any non-matching ACK. 5-1 (netascii translation on received data) and 4-2 (WRQ acknowledged with an ACK for block 0) are {not-applicable}: ze is a read-only server that rejects WRQ and accepts only octet mode.

## What the public ledger says

**Status:** Supported

**What the ledger says is covered:**

Read-only TFTP server for PXE bootloader delivery.

**What the ledger says remains:**

[`RFC1350-2-3`](#rfc1350-2-3) unmet (Sorcerer's Apprentice fix): sendAndWaitACK retransmits DATA on any non-matching ACK (handler.go) instead of silently ignoring a duplicate or stale ACK.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 1 | one part of the gated population |
| Annotated instead of tested | 10 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **11** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (1):** [`RFC1350-2-2`](#rfc1350-2-2)

**Annotated instead of tested (10):** [`RFC1350-2-1`](#rfc1350-2-1), [`RFC1350-6-1`](#rfc1350-6-1), [`RFC1350-5-1`](#rfc1350-5-1), [`RFC1350-5-2`](#rfc1350-5-2), [`RFC1350-4-1`](#rfc1350-4-1), [`RFC1350-4-2`](#rfc1350-4-2), [`RFC1350-4-3`](#rfc1350-4-3), [`RFC1350-7-1`](#rfc1350-7-1), [`RFC1350-2-3`](#rfc1350-2-3), [`RFC1350-5-3`](#rfc1350-5-3)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC1350-2-1` | File is sent in fixed length blocks of 512 bytes (Section 2) | MUST | 2 - Overview of the Protocol | **positive:** `unit/verify` [`TestTFTPReadExact512`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L471). **positive:** `unit/verify` [`TestTFTPReadLargeFile`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L390). **negative:** no negative test. **{single-polarity}:** ze is the DATA sender and always frames output at the block size, ending on a short or zero block (handler.go:360, 373, 379), so no client input can make it emit a non-conforming block and there is no reject path for a negative |
| `RFC1350-2-2` | Each data packet must be acknowledged before the next packet can be sent (Section 2) | MUST | 2 - Overview of the Protocol | **positive:** `unit/verify` [`TestTFTPReadLargeFile`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L393). **negative:** `unit/verify` [`TestTFTPConcurrentLimit`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L726) |
| `RFC1350-6-1` | Host sending the last DATA must retransmit it until acknowledged or timed out (Section 6) | MUST | 6 - Normal Termination | **positive:** `unit/verify` [`TestTFTPRetransmitOnTimeout`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L757). **negative:** no negative test. **{single-polarity}:** retransmission fires only on ackTimeout expiry in sendAndWaitACK (handler.go:392-400). Its only counterpart, ceasing to resend once a valid ACK arrives, is the lockstep advance already pinned by RFC1350-2-2, so no distinct negative remains |
| `RFC1350-5-1` | Host receiving netascii mode data must translate the data to its own format (Section 5) | MUST | 5 - TFTP Packets | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze is a read-only TFTP server that rejects WRQ (internal/plugins/tftpserver/handler.go:227) and accepts only octet mode (handler.go:248), so it never receives file data to translate between netascii and its local format |
| `RFC1350-5-2` | If a host receives an octet file and then returns it, the returned file must be identical to the original (Section 5) | MUST | 5 - TFTP Packets | **positive:** `unit/verify` [`TestTFTPReadLargeFile`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L398). **positive:** `unit/verify` [`TestTFTPReadRequest`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L335). **negative:** no negative test. **{single-polarity}:** octet is the only accepted mode (handler.go:248) and serveFile copies file bytes verbatim into DATA (handler.go:360, 373), so no octet code path transforms bytes and there is no altered-bytes negative to test |
| `RFC1350-4-1` | Block numbers are consecutive and begin with one (Section 4) | MUST | 4 - Initial Connection Protocol | **positive:** `unit/verify` [`TestTFTPReadLargeFile`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L396). **positive:** `unit/verify` [`TestTFTPReadRequest`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L332). **negative:** no negative test. **{single-polarity}:** the server assigns block numbers itself, starting at 1 and incrementing (handler.go:362, 382), so no input can make it emit a non-1-based or non-consecutive number and there is no reject path for a negative |
| `RFC1350-4-2` | Positive response to a write request is an ACK with block number zero (Section 4) | MUST | 4 - Initial Connection Protocol | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze serves reads only and rejects WRQ with an ERROR (internal/plugins/tftpserver/handler.go:227-230), so there is no WRQ-to-ACK-block-0 write-initiation path |
| `RFC1350-4-3` | Each end of the connection chooses a TID for itself, used for the duration of that connection (Section 4) | MUST | 4 - Initial Connection Protocol | **positive:** `unit/verify` [`TestListenTFTPLoopbackRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/socket_integration_linux_test.go#L144). **negative:** no negative test. **{single-polarity}:** the server always allocates a fresh transfer socket via net.DialUDP per RRQ (handler.go:287), so no configuration or input reuses port 69 and there is no stale-TID negative case |
| `RFC1350-7-1` | Timeouts must be used to detect errors (Section 7) | MUST | 7 - Premature Termination | **positive:** `unit/verify` [`TestTFTPRetransmitOnTimeout`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L760). **negative:** no negative test. **{single-polarity}:** the failure branch (abort after maxRetransmit ackTimeout expiries, handler.go:387 and 411, roughly 4x5s) needs about 20 seconds of real timeouts to reach, so a unit test exercises only the positive timeout-detection |
| `RFC1350-2-3` | Duplicate ACKs must be silently ignored; must not resend next DATA block (Sorcerer's Apprentice fix, Section 2 / RFC 1123 Section 4.2) | MUST | 2 - Overview of the Protocol | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze's sendAndWaitACK (internal/plugins/tftpserver/handler.go:386-413) retransmits the DATA block on any non-matching ACK (ackBlock != block, handler.go:407) rather than silently ignoring a duplicate or stale ACK, so the RFC 1350 Sorcerer's Apprentice Syndrome fix is not implemented |
| `RFC1350-5-3` | When file size is exact multiple of 512, a final DATA packet with zero bytes of data must be sent (Section 5, Section 6) | MUST | 5 - TFTP Packets | **positive:** `unit/verify` [`TestTFTPReadEmptyFile`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L537). **positive:** `unit/verify` [`TestTFTPReadExact512`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L474). **negative:** no negative test. **{single-polarity}:** ze is the sender and always appends the zero-length terminator when the file length is an exact multiple of the block size (handler.go:364-383, 379), so no input can suppress or misplace it and there is no reject-path negative for this framing requirement |
| `RFC1350-4-4` | If source TID does not match, packet should be discarded; an error packet should be sent to incorrect source while not disturbing the transfer (Section 4) | SHOULD | 4 - Initial Connection Protocol | **positive:** no positive test. **negative:** no negative test |
| `RFC1350-4-5` | TIDs chosen for a connection should be randomly chosen (Section 4) | SHOULD | 4 - Initial Connection Protocol | **positive:** no positive test. **negative:** no negative test |
| `RFC1350-6-2` | Host sending final ACK should dally (wait before terminating) to retransmit final ACK if lost (Section 6) | SHOULD | 6 - Normal Termination | **positive:** no positive test. **negative:** no negative test |
| `RFC1350-5-4` | Error message in ERROR packet should be in netascii (Section 5) | SHOULD | 5 - TFTP Packets | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC1350-5-1`](#rfc1350-5-1) Host receiving netascii mode data must translate the data to its own format (Section 5) | no test | no test carries this requirement id; annotated {not-applicable}: ze is a read-only TFTP server that rejects WRQ (internal/plugins/tftpserver/handler.go:227) and accepts only octet mode (handler.go:248), so it never receives file data to translate between netascii and its local format |
| [`RFC1350-4-2`](#rfc1350-4-2) Positive response to a write request is an ACK with block number zero (Section 4) | no test | no test carries this requirement id; annotated {not-applicable}: ze serves reads only and rejects WRQ with an ERROR (internal/plugins/tftpserver/handler.go:227-230), so there is no WRQ-to-ACK-block-0 write-initiation path |
| [`RFC1350-2-3`](#rfc1350-2-3) Duplicate ACKs must be silently ignored; must not resend next DATA block (Sorcerer's Apprentice fix, Section 2 / RFC 1123 Section 4.2) | {gap}, no test | ze's sendAndWaitACK (internal/plugins/tftpserver/handler.go:386-413) retransmits the DATA block on any non-matching ACK (ackBlock != block, handler.go:407) rather than silently ignoring a duplicate or stale ACK, so the RFC 1350 Sorcerer's Apprentice Syndrome fix is not implemented |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC1350-2-1`](#rfc1350-2-1)

File is sent in fixed length blocks of 512 bytes (Section 2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestTFTPReadExact512`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L471) | unit/verify | unproven |
| positive | [`TestTFTPReadLargeFile`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L390) | unit/verify | unproven |

### [`RFC1350-2-2`](#rfc1350-2-2)

Each data packet must be acknowledged before the next packet can be sent (Section 2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestTFTPConcurrentLimit`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L726) | unit/verify | unproven |
| positive | [`TestTFTPReadLargeFile`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L393) | unit/verify | unproven |

### [`RFC1350-6-1`](#rfc1350-6-1)

Host sending the last DATA must retransmit it until acknowledged or timed out (Section 6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestTFTPRetransmitOnTimeout`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L757) | unit/verify | unproven |

### [`RFC1350-5-1`](#rfc1350-5-1)

Host receiving netascii mode data must translate the data to its own format (Section 5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1350-5-1, so no unit is bound to it.

### [`RFC1350-5-2`](#rfc1350-5-2)

If a host receives an octet file and then returns it, the returned file must be identical to the original (Section 5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestTFTPReadLargeFile`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L398) | unit/verify | unproven |
| positive | [`TestTFTPReadRequest`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L335) | unit/verify | unproven |

### [`RFC1350-4-1`](#rfc1350-4-1)

Block numbers are consecutive and begin with one (Section 4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestTFTPReadLargeFile`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L396) | unit/verify | unproven |
| positive | [`TestTFTPReadRequest`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L332) | unit/verify | unproven |

### [`RFC1350-4-2`](#rfc1350-4-2)

Positive response to a write request is an ACK with block number zero (Section 4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1350-4-2, so no unit is bound to it.

### [`RFC1350-4-3`](#rfc1350-4-3)

Each end of the connection chooses a TID for itself, used for the duration of that connection (Section 4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestListenTFTPLoopbackRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/socket_integration_linux_test.go#L144) | unit/verify | unproven |

### [`RFC1350-7-1`](#rfc1350-7-1)

Timeouts must be used to detect errors (Section 7)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestTFTPRetransmitOnTimeout`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L760) | unit/verify | unproven |

### [`RFC1350-2-3`](#rfc1350-2-3)

Duplicate ACKs must be silently ignored; must not resend next DATA block (Sorcerer's Apprentice fix, Section 2 / RFC 1123 Section 4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1350-2-3, so no unit is bound to it.

### [`RFC1350-5-3`](#rfc1350-5-3)

When file size is exact multiple of 512, a final DATA packet with zero bytes of data must be sent (Section 5, Section 6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestTFTPReadEmptyFile`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L537) | unit/verify | unproven |
| positive | [`TestTFTPReadExact512`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L474) | unit/verify | unproven |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | ze-work agent, spec-rfcgate-6, rfc1350 |
| Signed off | 2026-08-31 |
| Register | prose |
| Source | rfc/full/rfc1350.txt |
| Source fingerprint | 37aca32d5dfaf1a8 |
| Record | rfc/extraction/rfc1350.json |
| Mapped sentences | 5 |
| Declined as scope | 5 |
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
| `5:3` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the mail-mode role, which RFC 1350 retires in its own section 1: 'mail, netascii characters sent to a user rather than a file. (The mail mode is obsolete and should not be implemented or used.)' The sentence tells a host that offers mail mode that such a transfer begins with a WRQ and names a recipient in place of a file. Ze plays no mail-mode role at all: handleRRQ accepts only 'octet' and the server rejects WRQ outright in serve (internal/plugins/tftpserver/handler.go). rfc/short/rfc1350.md declares no requirement for mail mode. | Mail mode uses the name of a mail recipient in place of a file and must begin with a WRQ. |
| `5:4` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | A worked example, not a directive. The sentence sits inside the DEC-20 narrative the section opens with 'One might create a special mode for such a machine which read all the bits in a word, but in which the receiver stored the information in 8-bit format', and the RFC closes the narrative by saying 'No such machine or application specific modes have been specified in TFTP'. The 'must' describes what such a hypothetical mode would need in order to be useful, and binds no speaker of the protocol as specified. rfc/short/rfc1350.md declares no requirement for it. | When such a file is retrieved from the storage site, it must be restored to its original form to be useful, so the reverse mode must also be implemented. |
| `5:5` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | A caution attached to a permission, naming no observable behavior. The enclosing construction is 'It is also possible to define other modes for cooperating pairs of hosts, although this must be done with care', and the two sentences after it say 'There is no requirement that any other hosts implement these. There is no central authority that will define these modes or assign them names.' Care is not a wire behavior a test can assert or a decoder can violate, and the RFC itself says the sentence creates no requirement. rfc/short/rfc1350.md declares none for it. | It is also possible to define other modes for cooperating pairs of hosts, although this must be done with care. |
| `I:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the system administrator who grants file system rights to the TFTP server process, a role RFC 1350 names in the sentence itself: 'care must be taken in the rights granted to a TFTP server process'. The obligation is a deployment decision made outside the protocol and outside ze, and the sentence after it describes the common deployment rather than requiring one: 'TFTP is often installed with controls such that only files that have public read access are available via TFTP and writing files via TFTP is disallowed.' Ze confines every transfer to the configured root through resolvePath (internal/plugins/tftpserver/handler.go) and serves reads only, which is the posture the sentence recommends to that administrator, but the rights on the files themselves are not ze's to grant. rfc/short/rfc1350.md declares no requirement for it. | Since TFTP includes no login or access control mechanisms, care must be taken in the rights granted to a TFTP server process so as not to violate the security of the server hosts file system. |

## Superseded

No document obsoletes RFC 1350, so its obligations are stated where they were written.
