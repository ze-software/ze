# RFC 2347 - TFTP Option Extension

Supported. Every requirement this repository extracted from RFC 2347, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 100.0% | 3 of 3 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 3 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 3 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 3 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| No test at all | 0.0% | 0 of 3 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 100.0% | 14 of 14 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |
| Audit verdicts | 4 | of 3 gated MUSTs judged | 0 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 3 | of 5 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 3 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 3 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 3 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 3 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |
| Linux cannot do this | 0.0% | 0 of 3 gated MUSTs | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and remains unmet; it stays in every denominator and earns no proof credit |

The 9 shares marked as a part above are the whole of the 3 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Linux cannot do this | neutral | neutral at every value: an evidenced platform limit is neither a proof nor a scope exemption |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | ok | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Supported |
| Enrolment | Enrolled |
| Requirements | 5 |
| Gated MUST-level | 3 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 14 |
| Tagged units | 14 |
| Recorded audit verdicts | 4 |
| Discrimination records | 14 |
| Summary | `rfc/short/rfc2347.md` |
| Requirement shard | `rfc/requirements/rfc2347.md` |
| RFC text | `rfc/full/rfc2347.txt` |

## Enrolment

Enrolled: TFTP Option Extension: three MUST-level rows, every one met and tested in both polarities. The two server-side rows run loopback OACK round-trips in rfc2347_option_negotiation_test.go and rfc2347_2348_transfer_test.go (producer: parseRRQ, handleRRQ and sendOACKAndWait, internal/plugins/tftpserver/handler.go): RFC2347-x-1 (the server MUST NOT include in the OACK any option not requested by the client) via TestRFC2347ServerOACKOnlyRequestedOptions; RFC2347-x-3 (an option not acknowledged is ignored as if never requested) via TestRFC2347ServerIgnoresUnacknowledgedOption on the server and TestRFC2347ClientUsesOnlyAcknowledgedOptions on the client. RFC2347-x-4 (the client MUST use the acknowledged options and MUST NOT use the others) is met by Ze's TFTP client, the tftp scheme of ze init --from (internal/core/fetch/tftp.go, internal/core/tftp/oack.go ParseOACK): TestRFC2347ClientUsesOnlyAcknowledgedOptions shows an OACK granting blksize alone gives 1024-octet blocks in lockstep, and a DATA answer that acknowledges nothing gives 512-octet blocks, not the 1468 requested. RFC2347-x-2, which quoted the first half of the x-4 sentence, was retired into x-4 on 2026-09-30 (rfc/corrections/rfc2347.md). The SHOULD x-6 (ERROR 8 for an unrequested option in an OACK) is met by the client, TestRFC2347ClientRejectsUnrequestedOption.

## What the public ledger says

**Status:** Supported

**What the ledger says is covered:**

Option negotiation for TFTP provisioning paths.

**What the ledger says remains:**

No tracked gap in current source anchors.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 3 | one part of the gated population |
| Annotated (including scoped evidence) | 0 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **3** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (3):** [`RFC2347-x-1`](#rfc2347-x-1), [`RFC2347-x-3`](#rfc2347-x-3), [`RFC2347-x-4`](#rfc2347-x-4)

## Attainable ceiling

Mandatory ceiling: 3 of 3 (100.0%). Optional ceiling: 2 of 2 (100.0%). Every RFC shown with measured rows can reach 100% under current classifications. Lowest mandatory ceiling: rfc2347 3 of 3 (100.0%). Lowest optional ceiling: rfc2347 2 of 2 (100.0%). Ceilings describe current classifications, not an immutable Linux maximum. Not-applicable and feature-declined classifications can be revisited. Every gap remains work Ze owes.

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC2347-x-1` | The server must not include in the OACK any option which had not been specifically requested by the client; that is, only the client may initiate option negotiation. (§Negotiation) | MUST NOT | Negotiation | **positive:** `unit/verify` [`TestRFC2347OnlyTheClientInitiatesNegotiation`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2347_2348_transfer_test.go#L94). **positive:** `unit/verify` [`TestRFC2347ServerOACKOnlyRequestedOptions`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2347_option_negotiation_test.go#L80). **negative:** `unit/verify` [`TestRFC2347OnlyTheClientInitiatesNegotiation`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2347_2348_transfer_test.go#L96). **negative:** `unit/verify` [`TestRFC2347ServerOACKOnlyRequestedOptions`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2347_option_negotiation_test.go#L84) |
| `RFC2347-x-3` | An option not acknowledged by the server must be ignored by the client and server as if it were never requested. (§Negotiation) | MUST | Negotiation | **positive:** `unit/verify` [`TestRFC2347ClientUsesOnlyAcknowledgedOptions`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc2347_client_test.go#L21). **positive:** `unit/verify` [`TestRFC2347ServerIgnoresUnacknowledgedOption`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2347_option_negotiation_test.go#L105). **positive:** `unit/verify` [`TestRFC2347UnacknowledgedOptionIsNotUsed`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2347_2348_transfer_test.go#L129). **negative:** `unit/verify` [`TestRFC2347ClientUsesOnlyAcknowledgedOptions`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc2347_client_test.go#L27). **negative:** `unit/verify` [`TestRFC2347ServerIgnoresUnacknowledgedOption`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2347_option_negotiation_test.go#L110). **negative:** `unit/verify` [`TestRFC2347UnacknowledgedOptionIsNotUsed`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2347_2348_transfer_test.go#L134) |
| `RFC2347-x-4` | If multiple options were requested, the client must use those options which were acknowledged by the server and must not use those options which were not acknowledged by the server. (§Negotiation) | MUST NOT | Negotiation | **positive:** `unit/verify` [`TestRFC2347ClientUsesOnlyAcknowledgedOptions`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc2347_client_test.go#L17). **negative:** `unit/verify` [`TestRFC2347ClientUsesOnlyAcknowledgedOptions`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc2347_client_test.go#L23) |
| `RFC2347-x-5` | Options which the server does not support should be omitted from the OACK; they should not cause an ERROR packet to be generated. (§Negotiation) | SHOULD | Negotiation | **positive:** no positive test. **negative:** no negative test |
| `RFC2347-x-6` | If the client receives an OACK containing an unrequested option, it should respond with an ERROR packet, with error code 8, and terminate the transfer. (§Negotiation) | SHOULD | Negotiation | **positive:** `unit/verify` [`TestRFC2347ClientRejectsUnrequestedOption`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc2347_client_test.go#L77). **negative:** `unit/verify` [`TestRFC2347ClientRejectsUnrequestedOption`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc2347_client_test.go#L80) |

## Gaps and untested MUSTs

RFC 2347 declares no gap, and every gated MUST it carries has a test bound to it.

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC2347-x-1`](#rfc2347-x-1)

The server must not include in the OACK any option which had not been specifically requested by the client; that is, only the client may initiate option negotiation. (§Negotiation)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRFC2347OnlyTheClientInitiatesNegotiation: a tsize-only RRQ gets an OACK with tsize=1500 and fails if blksize is present (the case the old verdict found missing); a plain RRQ and a windowsize-only RRQ must get DATA block 1 as the first reply (expectData fails on an OACK), so the server never initiates negotiation. With TestRFC2347ServerOACKOnlyRequestedOptions (blksize-only, no tsize) both emittable options are pinned absent when unrequested.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2347OnlyTheClientInitiatesNegotiation`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2347_2348_transfer_test.go#L96) | unit/verify | revert, verified |
| negative | [`TestRFC2347ServerOACKOnlyRequestedOptions`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2347_option_negotiation_test.go#L84) | unit/verify | revert, verified |
| positive | [`TestRFC2347OnlyTheClientInitiatesNegotiation`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2347_2348_transfer_test.go#L94) | unit/verify | revert, verified |
| positive | [`TestRFC2347ServerOACKOnlyRequestedOptions`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2347_option_negotiation_test.go#L80) | unit/verify | revert, verified |

### [`RFC2347-x-3`](#rfc2347-x-3)

An option not acknowledged by the server must be ignored by the client and server as if it were never requested. (§Negotiation)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged after the timeout masking was fixed: shortenTFTP sets tftpTimeout 1 s and expectACK waits tftpTimeout/2, so a timeout-driven retransmitted ACK can no longer stand in for the immediate one. Client half, TestRFC2347ClientUsesOnlyAcknowledgedOptions: the unacknowledged windowsize must be ignored (ACK 1 within 500 ms after a 1024 or 512 block; observed red under the overlay mutant Unnegotiated() Windowsize 4) and the unacknowledged blksize 1468 must not be used (512 and 1024 blocks are not final, file exact). Server half unchanged: TestRFC2347ServerIgnoresUnacknowledgedOption and TestRFC2347UnacknowledgedOptionIsNotUsed (rename-only diffs).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2347ClientUsesOnlyAcknowledgedOptions`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc2347_client_test.go#L27) | unit/verify | revert, verified |
| negative | [`TestRFC2347UnacknowledgedOptionIsNotUsed`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2347_2348_transfer_test.go#L134) | unit/verify | revert, verified |
| negative | [`TestRFC2347ServerIgnoresUnacknowledgedOption`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2347_option_negotiation_test.go#L110) | unit/verify | revert, verified |
| positive | [`TestRFC2347ClientUsesOnlyAcknowledgedOptions`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc2347_client_test.go#L21) | unit/verify | revert, verified |
| positive | [`TestRFC2347UnacknowledgedOptionIsNotUsed`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2347_2348_transfer_test.go#L129) | unit/verify | revert, verified |
| positive | [`TestRFC2347ServerIgnoresUnacknowledgedOption`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2347_option_negotiation_test.go#L105) | unit/verify | revert, verified |

### [`RFC2347-x-4`](#rfc2347-x-4)

If multiple options were requested, the client must use those options which were acknowledged by the server and must not use those options which were not acknowledged by the server. (§Negotiation)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Ze's fetch client fills the client role. TestRFC2347ClientUsesOnlyAcknowledgedOptions: the acknowledged blksize 1024 is used (a 1024 block is not final, file 1124 octets), the unacknowledged 1468 is not (512 blocks are not final, file 1034 octets), and the unacknowledged windowsize 4 is not used: each ACK must arrive within tftpTimeout/2. Observed red under the overlay mutant that keeps the requested windowsize (Unnegotiated() Windowsize 4). Residual: tsize is requested and unacknowledged with no assertion that it goes unused, but the client uses tsize only for the size cap, so nothing observable depends on it.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2347ClientUsesOnlyAcknowledgedOptions`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc2347_client_test.go#L23) | unit/verify | revert, verified |
| positive | [`TestRFC2347ClientUsesOnlyAcknowledgedOptions`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc2347_client_test.go#L17) | unit/verify | revert, verified |

### [`RFC2347-x-6`](#rfc2347-x-6)

If the client receives an OACK containing an unrequested option, it should respond with an ERROR packet, with error code 8, and terminate the transfer. (§Negotiation)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRFC2347ClientRejectsUnrequestedOption: an OACK carrying unrequested 'timeout' (and one naming blksize twice) must draw ERROR with code exactly 8 as the client's first packet, then silence, and the fetch must fail; a client that ACKs or sends another code goes red (grant() default branch, ParseOACK seenOptions). Negative: requested options in mixed case draw ACK 0 and the file 'abc', so the refusal is not blanket.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2347ClientRejectsUnrequestedOption`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc2347_client_test.go#L80) | unit/verify | revert, verified |
| positive | [`TestRFC2347ClientRejectsUnrequestedOption`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc2347_client_test.go#L77) | unit/verify | revert, verified |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | ze-work agent, spec-rfcgate-6, rfc2347 |
| Signed off | 2026-08-31 |
| Register | prose |
| Source | rfc/full/rfc2347.txt |
| Source fingerprint | 15ce2b209b116c40 |
| Record | rfc/extraction/rfc2347.json |
| Mapped sentences | 3 |
| Declined as scope | 1 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | The whole document | 4 | walked | The whole document. RFC 2347 carries no numbered section headings, so the derivation returns one section spanning the title block to the Full Copyright Statement, and a skip of it would hide every gated id behind a skip kind. Walked heading by heading. Status of this Memo, the Copyright Notice and the Abstract are indicative. The Introduction says the mechanism is a backward-compatible extension enforcing a request-respond-acknowledge sequence and names RFC 2348 and RFC 2349 as the options it was created to carry; it directs nobody. Packet Formats is the wire-format section, written entirely in the indicative: options are appended to the Read Request or Write Request as NUL-terminated name/value pairs, a new opcode 6 (OACK) acknowledges them, a new error code 8 terminates a transfer over option negotiation, the order of options is not significant, and 'The maximum size of a request packet is 512 octets.' Ze reads those bytes in parseRRQ and writes them in buildOACK (internal/plugins/tftpserver/handler.go), and rfc/short/rfc2347.md carries the two figures and the constants; no gated row is read from this material. Packet Formats also defers three fields to RFC 1350 in the indicative ('as defined in [1]' for opc, filename and mode), which cites the base document rather than restating an obligation of it, so no site here is a cross-document exclusion. Negotiation Protocol is the only normative section and holds three of the four sites, front:1 to front:3, all classified below; its remaining normative sentences are advisory and ungated -- options the server does not support 'should be omitted from the OACK; they should not cause an ERROR packet to be generated' (RFC2347-x-5), and a client receiving an unrequested option in an OACK 'should respond with an ERROR packet, with error code 8' (RFC2347-x-6). Its three-response tables for RRQ and WRQ, the fallback narrative for a server that does not implement options, and the two ways a client confirms an OACK are indicative. The Examples section is two packet traces. Security Considerations says the document adds no security to TFTP and no additional risk, so it states no countermeasure. The References, Authors' Addresses and the Full Copyright Statement bind no TFTP speaker; the copyright statement's one modal sentence is site front:4, excluded below. |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `front:4` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Boilerplate the extractor did not strip: the RFC Editor's Full Copyright Statement, which the site scan reaches because the sentence carries 'may not be modified' and 'must be followed'. It binds whoever copies or translates the document, under the Internet Society's copyright terms, and says nothing about TFTP option negotiation or about any byte on the wire. rfc/short/rfc2347.md declares no requirement for it. | However, this document itself may not be modified in any way, such as by removing the copyright notice or references to the Internet Society or other Internet organizations, except as needed for the purpose of developing Internet standards in which case the procedures for copyrights defined in the Internet Standards process must be followed, or as required to translate it into languages other than English. |

## Superseded

No document obsoletes RFC 2347, so its obligations are stated where they were written.
