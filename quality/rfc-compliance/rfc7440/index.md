# RFC 7440 - TFTP Windowsize Option

No row in the public ledger. Every requirement this repository extracted from RFC 7440, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 11.1% | 1 of 9 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 9 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 9 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 9 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| No test at all | 0.0% | 0 of 9 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 100.0% | 4 of 4 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 9 | of 15 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 8 | of 9 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 88.9% | 8 of 9 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 9 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 9 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

The 8 shares marked as a part above are the whole of the 9 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Public status | No row in the public ledger |
| Enrolment | Enrolled |
| Requirements | 15 |
| Gated MUST-level | 9 |
| Not applicable, so out of scope | 8 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 4 |
| Tagged units | 4 |
| Recorded audit verdicts | 1 |
| Discrimination records | 4 |
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
| Annotated (including scoped evidence) | 8 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **9** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (1):** [`RFC7440-3-1`](#rfc7440-3-1)

**Annotated (including scoped evidence) (8):** [`RFC7440-3-2`](#rfc7440-3-2), [`RFC7440-3-3`](#rfc7440-3-3), [`RFC7440-3-4`](#rfc7440-3-4), [`RFC7440-4-1`](#rfc7440-4-1), [`RFC7440-4-2`](#rfc7440-4-2), [`RFC7440-4-3`](#rfc7440-4-3), [`RFC7440-4-4`](#rfc7440-4-4), [`RFC7440-4-5`](#rfc7440-4-5)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC7440-3-1` | Note that all fields except "opc" MUST be ASCII strings followed by a single-byte NULL character. (§3) | MUST | 3 | **positive:** `unit/verify` [`TestRFC7440FieldsAreNULTerminatedASCIIStrings`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2347_2348_transfer_test.go#L257). **positive:** `unit/verify` [`TestTFTPParseRRQ`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L98). **negative:** `unit/verify` [`TestRFC7440FieldsAreNULTerminatedASCIIStrings`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2347_2348_transfer_test.go#L261). **negative:** `unit/verify` [`TestTFTPParseRRQInvalid`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L126) |
| `RFC7440-3-2` | The valid values range MUST be between 1 and 65535 blocks, inclusive. (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze does not implement the RFC 7440 windowsize option; parseRRQ records the option name as a bool and discards the value (internal/plugins/tftpserver/handler.go:129-130) without range-validating it, and the option is never negotiated |
| `RFC7440-3-3` | If the server is willing to accept the windowsize option, it sends an Option Acknowledgment (OACK) to the client. The specified value MUST be less than or equal to the value specified by the client. (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze never acknowledges the windowsize option -- sendOACKAndWait (internal/plugins/tftpserver/handler.go:312-343) builds the OACK from blksize/tsize only, so there is no acknowledged windowsize to constrain |
| `RFC7440-3-4` | The client MUST then either use the size specified in the OACK or send an ERROR packet, with error code 8, to terminate the transfer. (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** this is a TFTP client obligation (use the OACK windowsize or send ERROR 8); ze is a TFTP server and does not implement the windowsize option |
| `RFC7440-4-1` | The DSND MUST cyclically send to the DRCV the agreed windowsize consecutive data blocks before normally stopping and waiting for the ACK of the transferred window. (§4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's TFTP server transfers in RFC 1350 lockstep -- one DATA block per ACK (serveFile/sendAndWaitACK internal/plugins/tftpserver/handler.go:346-413) -- and implements no windowed send; it logs a fallback to lockstep when a client requests windowsize (handler.go:276-277) |
| `RFC7440-4-2` | The DRCV MUST send to the DSND the ACK of the last data block of the window in order to confirm a successful data block window reception. (§4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze acknowledges each single DATA block, not a window -- serveFile/sendAndWaitACK wait for the ACK of the one block just sent (internal/plugins/tftpserver/handler.go:346-413); with no negotiated windowsize there is no last-block-of-window ACK to send |
| `RFC7440-4-3` | Traffic with windowsize = 1 MUST be equivalent to traffic specified by [RFC1350]. (§4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's default RFC 1350 lockstep is windowsize-1-equivalent by construction, but the windowsize option itself is unimplemented -- parseRRQ discards the requested value (internal/plugins/tftpserver/handler.go:129-130) and the OACK never carries windowsize (handler.go:312-343), so there is no negotiated windowsize=1 to equate |
| `RFC7440-4-4` | In the case of an expected ACK not timely reaching the DSND (timeout), the last received ACK SHALL set the beginning of the next windowsize data block window to be sent. (§4) | SHALL | 4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze retransmits the single unacknowledged block on timeout (sendAndWaitACK internal/plugins/tftpserver/handler.go:386-413) and computes no window start; with no windowed send there is no next-window beginning to derive from the last ACK |
| `RFC7440-4-5` | The notified DSND SHOULD send a new data block window whose beginning MUST be set based on the ACK received out of sequence. (§4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze has no windowed transfer, so no out-of-sequence window recomputation exists -- sendAndWaitACK only accepts the ACK matching the block just sent and otherwise retransmits that one block (internal/plugins/tftpserver/handler.go:404-411) |
| `RFC7440-5-1` | Operators SHOULD test various values and SHOULD be conservative when selecting a windowsize value (§5) | SHOULD | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC7440-6-1` | Implementations SHOULD always set a maximum number of retries for datagram retransmissions (§6) | SHOULD | 6 | **positive:** no positive test. **negative:** no negative test |
| `RFC7440-6-2` | Implementations SHOULD always set a maximum number of retries for datagram retransmissions, imposing an appropriate threshold on error recovery attempts, after which a transfer SHOULD always be aborted to prevent pathological retransmission conditions. (§6) | SHOULD | 6 | **positive:** no positive test. **negative:** no negative test |
| `RFC7440-4-6` | In the case of a data block sequence error, the DRCV SHOULD notify the DSND by sending an ACK corresponding to the last data block correctly received. (§4) | SHOULD | 4 | **positive:** no positive test. **negative:** no negative test |
| `RFC7440-6-3` | The rate at which TFTP UDP datagrams are sent SHOULD follow the CC guidelines in Section 3.1 of [RFC5405]. (§6) | SHOULD | 6 | **positive:** no positive test. **negative:** no negative test |
| `RFC7440-7-1` | TFTP file transfers are NOT RECOMMENDED where the inherent protocol limitations could raise insurmountable liability concerns (§7) | NOT RECOMMENDED | 7 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC7440-3-2`](#rfc7440-3-2) The valid values range MUST be between 1 and 65535 blocks, inclusive. (§3) | no test | no test carries this requirement id; annotated {not-applicable}: ze does not implement the RFC 7440 windowsize option; parseRRQ records the option name as a bool and discards the value (internal/plugins/tftpserver/handler.go:129-130) without range-validating it, and the option is never negotiated |
| [`RFC7440-3-3`](#rfc7440-3-3) If the server is willing to accept the windowsize option, it sends an Option Acknowledgment (OACK) to the client. The specified value MUST be less than or equal to the value specified by the client. (§3) | no test | no test carries this requirement id; annotated {not-applicable}: ze never acknowledges the windowsize option -- sendOACKAndWait (internal/plugins/tftpserver/handler.go:312-343) builds the OACK from blksize/tsize only, so there is no acknowledged windowsize to constrain |
| [`RFC7440-3-4`](#rfc7440-3-4) The client MUST then either use the size specified in the OACK or send an ERROR packet, with error code 8, to terminate the transfer. (§3) | no test | no test carries this requirement id; annotated {not-applicable}: this is a TFTP client obligation (use the OACK windowsize or send ERROR 8); ze is a TFTP server and does not implement the windowsize option |
| [`RFC7440-4-1`](#rfc7440-4-1) The DSND MUST cyclically send to the DRCV the agreed windowsize consecutive data blocks before normally stopping and waiting for the ACK of the transferred window. (§4) | no test | no test carries this requirement id; annotated {not-applicable}: ze's TFTP server transfers in RFC 1350 lockstep -- one DATA block per ACK (serveFile/sendAndWaitACK internal/plugins/tftpserver/handler.go:346-413) -- and implements no windowed send; it logs a fallback to lockstep when a client requests windowsize (handler.go:276-277) |
| [`RFC7440-4-2`](#rfc7440-4-2) The DRCV MUST send to the DSND the ACK of the last data block of the window in order to confirm a successful data block window reception. (§4) | no test | no test carries this requirement id; annotated {not-applicable}: ze acknowledges each single DATA block, not a window -- serveFile/sendAndWaitACK wait for the ACK of the one block just sent (internal/plugins/tftpserver/handler.go:346-413); with no negotiated windowsize there is no last-block-of-window ACK to send |
| [`RFC7440-4-3`](#rfc7440-4-3) Traffic with windowsize = 1 MUST be equivalent to traffic specified by [RFC1350]. (§4) | no test | no test carries this requirement id; annotated {not-applicable}: ze's default RFC 1350 lockstep is windowsize-1-equivalent by construction, but the windowsize option itself is unimplemented -- parseRRQ discards the requested value (internal/plugins/tftpserver/handler.go:129-130) and the OACK never carries windowsize (handler.go:312-343), so there is no negotiated windowsize=1 to equate |
| [`RFC7440-4-4`](#rfc7440-4-4) In the case of an expected ACK not timely reaching the DSND (timeout), the last received ACK SHALL set the beginning of the next windowsize data block window to be sent. (§4) | no test | no test carries this requirement id; annotated {not-applicable}: ze retransmits the single unacknowledged block on timeout (sendAndWaitACK internal/plugins/tftpserver/handler.go:386-413) and computes no window start; with no windowed send there is no next-window beginning to derive from the last ACK |
| [`RFC7440-4-5`](#rfc7440-4-5) The notified DSND SHOULD send a new data block window whose beginning MUST be set based on the ACK received out of sequence. (§4) | no test | no test carries this requirement id; annotated {not-applicable}: ze has no windowed transfer, so no out-of-sequence window recomputation exists -- sendAndWaitACK only accepts the ACK matching the block just sent and otherwise retransmits that one block (internal/plugins/tftpserver/handler.go:404-411) |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC7440-3-1`](#rfc7440-3-1)

Note that all fields except "opc" MUST be ASCII strings followed by a single-byte NULL character. (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. The sentence formats the RRQ/WRQ, so its ASCII clause binds the request's sender; Ze only receives requests, and no receiver obligation to refuse non-ASCII follows from it. Receiver side, TestRFC7440FieldsAreNULTerminatedASCIIStrings: the Section 3 example parses to foobar/octet/windowsize; a windowsize whose #blocks lacks its NUL is not recognized, and a filename without its NUL is refused (red on a parser taking an unterminated field). Supplementary: every field of the OACK Ze sends is printable ASCII with one NUL (four fields).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestTFTPParseRRQInvalid`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L126) | unit/verify | revert, verified |
| negative | [`TestRFC7440FieldsAreNULTerminatedASCIIStrings`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2347_2348_transfer_test.go#L261) | unit/verify | revert, verified |
| positive | [`TestTFTPParseRRQ`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/handler_test.go#L98) | unit/verify | revert, verified |
| positive | [`TestRFC7440FieldsAreNULTerminatedASCIIStrings`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2347_2348_transfer_test.go#L257) | unit/verify | revert, verified |

### [`RFC7440-3-2`](#rfc7440-3-2)

The valid values range MUST be between 1 and 65535 blocks, inclusive. (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7440-3-2, so no unit is bound to it.

### [`RFC7440-3-3`](#rfc7440-3-3)

If the server is willing to accept the windowsize option, it sends an Option Acknowledgment (OACK) to the client. The specified value MUST be less than or equal to the value specified by the client. (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7440-3-3, so no unit is bound to it.

### [`RFC7440-3-4`](#rfc7440-3-4)

The client MUST then either use the size specified in the OACK or send an ERROR packet, with error code 8, to terminate the transfer. (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7440-3-4, so no unit is bound to it.

### [`RFC7440-4-1`](#rfc7440-4-1)

The DSND MUST cyclically send to the DRCV the agreed windowsize consecutive data blocks before normally stopping and waiting for the ACK of the transferred window. (§4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7440-4-1, so no unit is bound to it.

### [`RFC7440-4-2`](#rfc7440-4-2)

The DRCV MUST send to the DSND the ACK of the last data block of the window in order to confirm a successful data block window reception. (§4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7440-4-2, so no unit is bound to it.

### [`RFC7440-4-3`](#rfc7440-4-3)

Traffic with windowsize = 1 MUST be equivalent to traffic specified by [RFC1350]. (§4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7440-4-3, so no unit is bound to it.

### [`RFC7440-4-4`](#rfc7440-4-4)

In the case of an expected ACK not timely reaching the DSND (timeout), the last received ACK SHALL set the beginning of the next windowsize data block window to be sent. (§4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7440-4-4, so no unit is bound to it.

### [`RFC7440-4-5`](#rfc7440-4-5)

The notified DSND SHOULD send a new data block window whose beginning MUST be set based on the ACK received out of sequence. (§4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7440-4-5, so no unit is bound to it.

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
