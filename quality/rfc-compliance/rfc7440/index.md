# RFC 7440 - TFTP Windowsize Option

No row in the public ledger. Every requirement this repository extracted from RFC 7440, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

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
| Gated MUSTs | 9 | of 15 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 8 | of 9 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

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
| Requirements | 15 |
| Gated MUST-level | 9 |
| Obligations that bind Ze | 1 |
| Not applicable, so out of scope | 8 |
| Declared gaps | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 2 |
| Tagged units | 2 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc7440.md` |
| Requirement shard | `rfc/requirements/rfc7440.md` |
| RFC text | `rfc/full/rfc7440.txt` |

## Enrolment

Enrolled: TFTP Windowsize Option: nine MUST-level requirements. 3-1 (all RRQ/WRQ fields except the opcode are NUL-terminated ASCII strings) is met with positive+negative tags on the RRQ parser (internal/plugins/tftpserver/handler.go:61 parseRRQ rejects a field lacking its NUL). The eight windowsize-option MUSTs (3-2 value range, 3-3 acknowledged windowsize, 3-4 client uses the OACK windowsize, 4-1 windowed send, 4-2 windowed ACK, 4-3 windowsize-1 equivalence, 4-4 timeout window-start, 4-5 sequence-error rollback) are {not-applicable}: ze's TFTP server implements base RFC 1350 with the RFC 2347/2348/2349 blksize and tsize options only; it records the windowsize option name but discards the value and never negotiates it, falling back to lockstep.

## What the public ledger says

No row in the public ledger, so its summary declares `| Support | - |` and docs/features/rfc-status.md carries no row for RFC 7440.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 1 | one part of the gated population |
| Annotated instead of tested | 8 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **9** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (1):** [`RFC7440-3-1`](#rfc7440-3-1)

**Annotated instead of tested (8):** [`RFC7440-3-2`](#rfc7440-3-2), [`RFC7440-3-3`](#rfc7440-3-3), [`RFC7440-3-4`](#rfc7440-3-4), [`RFC7440-4-1`](#rfc7440-4-1), [`RFC7440-4-2`](#rfc7440-4-2), [`RFC7440-4-3`](#rfc7440-4-3), [`RFC7440-4-4`](#rfc7440-4-4), [`RFC7440-4-5`](#rfc7440-4-5)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC7440-3-1` | All fields in RRQ/WRQ except "opc" MUST be ASCII strings followed by a single-byte NULL character (§3) | MUST | 3 | **positive:** `unit/verify` [`TestTFTPParseRRQ`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L98). **negative:** `unit/verify` [`TestTFTPParseRRQInvalid`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L126) |
| `RFC7440-3-2` | Valid windowsize values MUST be between 1 and 65535 blocks, inclusive (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze does not implement the RFC 7440 windowsize option; parseRRQ records the option name as a bool and discards the value (internal/plugins/tftpserver/handler.go:129-130) without range-validating it, and the option is never negotiated |
| `RFC7440-3-3` | Server's acknowledged windowsize MUST be less than or equal to the client's requested value (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze never acknowledges the windowsize option -- sendOACKAndWait (internal/plugins/tftpserver/handler.go:312-343) builds the OACK from blksize/tsize only, so there is no acknowledged windowsize to constrain |
| `RFC7440-3-4` | Client MUST use the windowsize specified in the OACK or send ERROR code 8 to terminate (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** this is a TFTP client obligation (use the OACK windowsize or send ERROR 8); ze is a TFTP server and does not implement the windowsize option |
| `RFC7440-4-1` | The data sender MUST cyclically send the agreed windowsize consecutive data blocks before stopping and waiting for ACK (§4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's TFTP server transfers in RFC 1350 lockstep -- one DATA block per ACK (serveFile/sendAndWaitACK internal/plugins/tftpserver/handler.go:346-413) -- and implements no windowed send; it logs a fallback to lockstep when a client requests windowsize (handler.go:276-277) |
| `RFC7440-4-2` | The data receiver MUST send ACK of the last data block of the window to confirm successful reception (§4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze acknowledges each single DATA block, not a window -- serveFile/sendAndWaitACK wait for the ACK of the one block just sent (internal/plugins/tftpserver/handler.go:346-413); with no negotiated windowsize there is no last-block-of-window ACK to send |
| `RFC7440-4-3` | Traffic with windowsize=1 MUST be equivalent to traffic specified by RFC 1350 (§4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's default RFC 1350 lockstep is windowsize-1-equivalent by construction, but the windowsize option itself is unimplemented -- parseRRQ discards the requested value (internal/plugins/tftpserver/handler.go:129-130) and the OACK never carries windowsize (handler.go:312-343), so there is no negotiated windowsize=1 to equate |
| `RFC7440-4-4` | On timeout, the beginning of the next window MUST be set based on the last received ACK (§4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze retransmits the single unacknowledged block on timeout (sendAndWaitACK internal/plugins/tftpserver/handler.go:386-413) and computes no window start; with no windowed send there is no next-window beginning to derive from the last ACK |
| `RFC7440-4-5` | On sequence error, the sender's new window beginning MUST be set based on the ACK received out of sequence (§4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze has no windowed transfer, so no out-of-sequence window recomputation exists -- sendAndWaitACK only accepts the ACK matching the block just sent and otherwise retransmits that one block (internal/plugins/tftpserver/handler.go:404-411) |
| `RFC7440-5-1` | Operators SHOULD test various windowsize values and SHOULD be conservative when selecting (§5) | SHOULD | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC7440-6-1` | Implementations SHOULD always set a maximum number of retries for datagram retransmissions (§6) | SHOULD | 6 | **positive:** no positive test. **negative:** no negative test |
| `RFC7440-6-2` | After max retries exceeded, a transfer SHOULD always be aborted (§6) | SHOULD | 6 | **positive:** no positive test. **negative:** no negative test |
| `RFC7440-4-6` | The data receiver SHOULD notify the sender of sequence errors by ACKing the last correctly received block (§4) | SHOULD | 4 | **positive:** no positive test. **negative:** no negative test |
| `RFC7440-6-3` | The rate of TFTP UDP datagrams SHOULD follow the congestion control guidelines in RFC 5405 Section 3.1 (§6) | SHOULD | 6 | **positive:** no positive test. **negative:** no negative test |
| `RFC7440-7-1` | TFTP file transfers are NOT RECOMMENDED where the inherent protocol limitations could raise insurmountable liability concerns (§7) | NOT RECOMMENDED | 7 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC7440-3-2`](#rfc7440-3-2) Valid windowsize values MUST be between 1 and 65535 blocks, inclusive (§3) | no test | no test carries this requirement id; annotated {not-applicable}: ze does not implement the RFC 7440 windowsize option; parseRRQ records the option name as a bool and discards the value (internal/plugins/tftpserver/handler.go:129-130) without range-validating it, and the option is never negotiated |
| [`RFC7440-3-3`](#rfc7440-3-3) Server's acknowledged windowsize MUST be less than or equal to the client's requested value (§3) | no test | no test carries this requirement id; annotated {not-applicable}: ze never acknowledges the windowsize option -- sendOACKAndWait (internal/plugins/tftpserver/handler.go:312-343) builds the OACK from blksize/tsize only, so there is no acknowledged windowsize to constrain |
| [`RFC7440-3-4`](#rfc7440-3-4) Client MUST use the windowsize specified in the OACK or send ERROR code 8 to terminate (§3) | no test | no test carries this requirement id; annotated {not-applicable}: this is a TFTP client obligation (use the OACK windowsize or send ERROR 8); ze is a TFTP server and does not implement the windowsize option |
| [`RFC7440-4-1`](#rfc7440-4-1) The data sender MUST cyclically send the agreed windowsize consecutive data blocks before stopping and waiting for ACK (§4) | no test | no test carries this requirement id; annotated {not-applicable}: ze's TFTP server transfers in RFC 1350 lockstep -- one DATA block per ACK (serveFile/sendAndWaitACK internal/plugins/tftpserver/handler.go:346-413) -- and implements no windowed send; it logs a fallback to lockstep when a client requests windowsize (handler.go:276-277) |
| [`RFC7440-4-2`](#rfc7440-4-2) The data receiver MUST send ACK of the last data block of the window to confirm successful reception (§4) | no test | no test carries this requirement id; annotated {not-applicable}: ze acknowledges each single DATA block, not a window -- serveFile/sendAndWaitACK wait for the ACK of the one block just sent (internal/plugins/tftpserver/handler.go:346-413); with no negotiated windowsize there is no last-block-of-window ACK to send |
| [`RFC7440-4-3`](#rfc7440-4-3) Traffic with windowsize=1 MUST be equivalent to traffic specified by RFC 1350 (§4) | no test | no test carries this requirement id; annotated {not-applicable}: ze's default RFC 1350 lockstep is windowsize-1-equivalent by construction, but the windowsize option itself is unimplemented -- parseRRQ discards the requested value (internal/plugins/tftpserver/handler.go:129-130) and the OACK never carries windowsize (handler.go:312-343), so there is no negotiated windowsize=1 to equate |
| [`RFC7440-4-4`](#rfc7440-4-4) On timeout, the beginning of the next window MUST be set based on the last received ACK (§4) | no test | no test carries this requirement id; annotated {not-applicable}: ze retransmits the single unacknowledged block on timeout (sendAndWaitACK internal/plugins/tftpserver/handler.go:386-413) and computes no window start; with no windowed send there is no next-window beginning to derive from the last ACK |
| [`RFC7440-4-5`](#rfc7440-4-5) On sequence error, the sender's new window beginning MUST be set based on the ACK received out of sequence (§4) | no test | no test carries this requirement id; annotated {not-applicable}: ze has no windowed transfer, so no out-of-sequence window recomputation exists -- sendAndWaitACK only accepts the ACK matching the block just sent and otherwise retransmits that one block (internal/plugins/tftpserver/handler.go:404-411) |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC7440-3-1`](#rfc7440-3-1)

All fields in RRQ/WRQ except "opc" MUST be ASCII strings followed by a single-byte NULL character (§3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestTFTPParseRRQInvalid`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L126) | unit/verify | unproven |
| positive | [`TestTFTPParseRRQ`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L98) | unit/verify | unproven |

### [`RFC7440-3-2`](#rfc7440-3-2)

Valid windowsize values MUST be between 1 and 65535 blocks, inclusive (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7440-3-2, so no unit is bound to it.

### [`RFC7440-3-3`](#rfc7440-3-3)

Server's acknowledged windowsize MUST be less than or equal to the client's requested value (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7440-3-3, so no unit is bound to it.

### [`RFC7440-3-4`](#rfc7440-3-4)

Client MUST use the windowsize specified in the OACK or send ERROR code 8 to terminate (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7440-3-4, so no unit is bound to it.

### [`RFC7440-4-1`](#rfc7440-4-1)

The data sender MUST cyclically send the agreed windowsize consecutive data blocks before stopping and waiting for ACK (§4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7440-4-1, so no unit is bound to it.

### [`RFC7440-4-2`](#rfc7440-4-2)

The data receiver MUST send ACK of the last data block of the window to confirm successful reception (§4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7440-4-2, so no unit is bound to it.

### [`RFC7440-4-3`](#rfc7440-4-3)

Traffic with windowsize=1 MUST be equivalent to traffic specified by RFC 1350 (§4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7440-4-3, so no unit is bound to it.

### [`RFC7440-4-4`](#rfc7440-4-4)

On timeout, the beginning of the next window MUST be set based on the last received ACK (§4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7440-4-4, so no unit is bound to it.

### [`RFC7440-4-5`](#rfc7440-4-5)

On sequence error, the sender's new window beginning MUST be set based on the ACK received out of sequence (§4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7440-4-5, so no unit is bound to it.

## Extraction sign-off

No extraction sign-off exists for RFC 7440, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 7440, so its obligations are stated where they were written.
