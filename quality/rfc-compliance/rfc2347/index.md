# RFC 2347 - TFTP Option Extension

TFTP option negotiation. Every requirement listed for RFC 2347, the tests linked to it, and their recorded reviews. This summary is included in the automatic compliance check, ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 3 | of 3 mandatory requirements | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0 | of 3 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0 | of 3 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0 | of 3 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Declared gap or no test | 0 | of 3 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |
| Tests shown to detect a fault | 14 | 14 of 14 tests linked to requirements | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests counts as proof only after this check has demonstrated that they can detect a fault |
| Audit verdicts | 4 | of 3 mandatory requirements reviewed | 0 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 3 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 0 | of 3 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 0 | of 3 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze proven: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 3 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 3 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no proof credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level) | 3 | of 5 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements included in the automatic checks: the compliance check (./le rfc check) holds Ze to each of them. This is the total being assessed; the counts above show the recorded status of those requirements. |

The 9 counts marked as a part above are the whole of the 3 mandatory requirements: they add up to 3. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got. An outlined card with no colored edge, under Totals, is a total the other cards are counted against, not a measure.

| Card | Tone here | Why that color |
|---|---|---|
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the count under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| Partial proof; remaining gap | ok | green at zero, RED above it: a tested clause cannot prove the whole requirement |
| Declared gap or no test | ok | green at zero, RED above it: a binding obligation that nothing shows Ze meets is owed, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green counts above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Linux cannot do this | neutral | neutral at every value: an evidenced platform limit is neither a proof nor a scope exemption |
| Tests shown to detect a fault | ok | green at every value: these tests have been shown to detect a deliberate fault. This card counts individual tests separately, so its total cannot be added to the requirement counts above |
| Audit verdicts | ok | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a mandatory requirement is unreviewed, green when every one is judged sound and current |
| Mandatory requirements (MUST level) | outlined | outlined, with no colored edge: it is the total the other cards are counted against, not a measure, and a larger one is neither good news nor bad |

## At a glance

| Field | Value |
|---|---|
| Public row | TFTP option negotiation |
| Automatic checks | Included |
| Requirements | 5 |
| Mandatory requirements (MUST level) | 3 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 14 |
| Tests linked to requirements | 14 |
| Recorded audit verdicts | 4 |
| Fault test records | 14 |
| Summary | `rfc/short/rfc2347.md` |
| Requirement table | `rfc/requirements/rfc2347.md` |
| RFC text | `rfc/full/rfc2347.txt` |

## Automatic checks

Included in automatic checks: TFTP Option Extension: three MUST-level rows, every one met and tested in both polarities. The two server-side rows run loopback OACK round-trips in rfc2347_option_negotiation_test.go and rfc2347_2348_transfer_test.go (producer: parseRRQ, handleRRQ and sendOACKAndWait, internal/plugins/tftpserver/handler.go): RFC2347-x-1 (the server MUST NOT include in the OACK any option not requested by the client) via TestRFC2347ServerOACKOnlyRequestedOptions; RFC2347-x-3 (an option not acknowledged is ignored as if never requested) via TestRFC2347ServerIgnoresUnacknowledgedOption on the server and TestRFC2347ClientUsesOnlyAcknowledgedOptions on the client. RFC2347-x-4 (the client MUST use the acknowledged options and MUST NOT use the others) is met by Ze's TFTP client, the tftp scheme of ze init --from (internal/core/fetch/tftp.go, internal/core/tftp/oack.go ParseOACK): TestRFC2347ClientUsesOnlyAcknowledgedOptions shows an OACK granting blksize alone gives 1024-octet blocks in lockstep, and a DATA answer that acknowledges nothing gives 512-octet blocks, not the 1468 requested. RFC2347-x-2, which quoted the first half of the x-4 sentence, was retired into x-4 on 2026-09-30 (rfc/corrections/rfc2347.md). The SHOULD x-6 (ERROR 8 for an unrequested option in an OACK) is met by the client, TestRFC2347ClientRejectsUnrequestedOption.

## Published support status

**Area:** TFTP option negotiation

**Proof:** 3 of 3 MUST proven

**Evidence:** unit 3

**Unmet:** -


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 3 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 0 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 0 | one part of the mandatory requirements |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level)** | **3** | every mandatory requirement appears in exactly one group above |

**Positive and negative tests (3):** [`RFC2347-x-1`](#rfc2347-x-1), [`RFC2347-x-3`](#rfc2347-x-3), [`RFC2347-x-4`](#rfc2347-x-4)

## Figures

- **MUST bar**: Ze proven 3
- **Overview bar**: Ze MUST proven 3, SHOULD proven 1, Not assessed 1

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | complete |
| MUST rows | 3 |
| MUST rows kept (the MUST bar) | 3 |
| Ze proven | 3 |
| OS verified | 0 |
| Proof pending | 0 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 0 |
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
| SHOULD proof pending | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 1 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze proven means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Proof pending counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

Source: [data/rfc/rfc2347.json](../../../data/rfc/rfc2347.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC2347-x-1` | The server must not include in the OACK any option which had not been specifically requested by the client; that is, only the client may initiate option negotiation. (§Negotiation) | MUST NOT | Negotiation | **positive:** `unit/verify` [`TestRFC2347OnlyTheClientInitiatesNegotiation`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2347_2348_transfer_test.go#L94). **positive:** `unit/verify` [`TestRFC2347ServerOACKOnlyRequestedOptions`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2347_option_negotiation_test.go#L80). **negative:** `unit/verify` [`TestRFC2347OnlyTheClientInitiatesNegotiation`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2347_2348_transfer_test.go#L96). **negative:** `unit/verify` [`TestRFC2347ServerOACKOnlyRequestedOptions`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2347_option_negotiation_test.go#L84) |
| `RFC2347-x-3` | An option not acknowledged by the server must be ignored by the client and server as if it were never requested. (§Negotiation) | MUST | Negotiation | **positive:** `unit/verify` [`TestRFC2347ClientUsesOnlyAcknowledgedOptions`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc2347_client_test.go#L21). **positive:** `unit/verify` [`TestRFC2347ServerIgnoresUnacknowledgedOption`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2347_option_negotiation_test.go#L109). **positive:** `unit/verify` [`TestRFC2347UnacknowledgedOptionIsNotUsed`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2347_2348_transfer_test.go#L133). **negative:** `unit/verify` [`TestRFC2347ClientUsesOnlyAcknowledgedOptions`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc2347_client_test.go#L27). **negative:** `unit/verify` [`TestRFC2347ServerIgnoresUnacknowledgedOption`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2347_option_negotiation_test.go#L115). **negative:** `unit/verify` [`TestRFC2347UnacknowledgedOptionIsNotUsed`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2347_2348_transfer_test.go#L139) |
| `RFC2347-x-4` | If multiple options were requested, the client must use those options which were acknowledged by the server and must not use those options which were not acknowledged by the server. (§Negotiation) | MUST NOT | Negotiation | **positive:** `unit/verify` [`TestRFC2347ClientUsesOnlyAcknowledgedOptions`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc2347_client_test.go#L17). **negative:** `unit/verify` [`TestRFC2347ClientUsesOnlyAcknowledgedOptions`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc2347_client_test.go#L23) |
| `RFC2347-x-5` | Options which the server does not support should be omitted from the OACK; they should not cause an ERROR packet to be generated. (§Negotiation) | SHOULD | Negotiation | **positive:** no positive test. **negative:** no negative test |
| `RFC2347-x-6` | If the client receives an OACK containing an unrequested option, it should respond with an ERROR packet, with error code 8, and terminate the transfer. (§Negotiation) | SHOULD | Negotiation | **positive:** `unit/verify` [`TestRFC2347ClientRejectsUnrequestedOption`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc2347_client_test.go#L77). **negative:** `unit/verify` [`TestRFC2347ClientRejectsUnrequestedOption`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc2347_client_test.go#L80) |

## Gaps and untested MUSTs

RFC 2347 lists no missing behavior, and every mandatory requirement included in automatic checks has a linked test.

## Proof state

A test reads unproven when no record shows it failing after a deliberate fault. Until then, its claim to check the requirement remains unproven.

### [`RFC2347-x-1`](#rfc2347-x-1)

The server must not include in the OACK any option which had not been specifically requested by the client; that is, only the client may initiate option negotiation. (§Negotiation)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-31 after the weak finding was fixed in the tagged units: the tsize-only OACK of TestRFC2347OnlyTheClientInitiatesNegotiation must hold tsize=1500 and exactly one option, and the blksize-only OACK of TestRFC2347ServerOACKOnlyRequestedOptions must hold blksize and exactly one option. Any unrequested blksize, timeout, tsize or windowsize that sendOACKAndWait appended is now red, covering all four options the server can emit. The no-option and utimeout-only RRQs must draw DATA 1 with no OACK, so the server never starts negotiation.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2347OnlyTheClientInitiatesNegotiation`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2347_2348_transfer_test.go#L96) | unit/verify | revert, verified |
| negative | [`TestRFC2347ServerOACKOnlyRequestedOptions`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2347_option_negotiation_test.go#L84) | unit/verify | revert, verified |
| positive | [`TestRFC2347OnlyTheClientInitiatesNegotiation`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2347_2348_transfer_test.go#L94) | unit/verify | revert, verified |
| positive | [`TestRFC2347ServerOACKOnlyRequestedOptions`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2347_option_negotiation_test.go#L80) | unit/verify | revert, verified |

### [`RFC2347-x-3`](#rfc2347-x-3)

An option not acknowledged by the server must be ignored by the client and server as if it were never requested. (§Negotiation)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-31: windowsize is now supported, so the server half moved to an option it declines. TestRFC2347UnacknowledgedOptionIsNotUsed: windowsize 0 (outside RFC 7440's 1..65535) beside blksize 1024 is absent from the OACK and is not used (block 1 carries the acknowledged 1024, then 1 s silence before ACK 1, so no window is sent; grantWindowsize gives 1 for 0); blksize 7 or 65465 beside tsize is absent and block 1 carries the default 512. TestRFC2347ServerIgnoresUnacknowledgedOption: utimeout beside blksize draws an OACK without utimeout and with blksize. Negative: the acknowledged blksize 1024 is used. Client TestRFC2347ClientUsesOnlyAcknowledgedOptions unchanged. Changes approved by an independent ze-test-judge.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2347ClientUsesOnlyAcknowledgedOptions`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc2347_client_test.go#L27) | unit/verify | revert, verified |
| negative | [`TestRFC2347UnacknowledgedOptionIsNotUsed`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2347_2348_transfer_test.go#L139) | unit/verify | revert, verified |
| negative | [`TestRFC2347ServerIgnoresUnacknowledgedOption`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2347_option_negotiation_test.go#L115) | unit/verify | revert, verified |
| positive | [`TestRFC2347ClientUsesOnlyAcknowledgedOptions`](https://github.com/ze-software/ze/blob/main/internal/core/fetch/rfc2347_client_test.go#L21) | unit/verify | revert, verified |
| positive | [`TestRFC2347UnacknowledgedOptionIsNotUsed`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2347_2348_transfer_test.go#L133) | unit/verify | revert, verified |
| positive | [`TestRFC2347ServerIgnoresUnacknowledgedOption`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2347_option_negotiation_test.go#L109) | unit/verify | revert, verified |

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

## Requirement list review

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
| `front` | The whole document | 4 | reviewed | The whole document. RFC 2347 carries no numbered section headings, so the derivation returns one section spanning the title block to the Full Copyright Statement, and a skip of it would hide every gated id behind a skip kind. Walked heading by heading. Status of this Memo, the Copyright Notice and the Abstract are indicative. The Introduction says the mechanism is a backward-compatible extension enforcing a request-respond-acknowledge sequence and names RFC 2348 and RFC 2349 as the options it was created to carry; it directs nobody. Packet Formats is the wire-format section, written entirely in the indicative: options are appended to the Read Request or Write Request as NUL-terminated name/value pairs, a new opcode 6 (OACK) acknowledges them, a new error code 8 terminates a transfer over option negotiation, the order of options is not significant, and 'The maximum size of a request packet is 512 octets.' Ze reads those bytes in parseRRQ and writes them in buildOACK (internal/plugins/tftpserver/handler.go), and rfc/short/rfc2347.md carries the two figures and the constants; no gated row is read from this material. Packet Formats also defers three fields to RFC 1350 in the indicative ('as defined in [1]' for opc, filename and mode), which cites the base document rather than restating an obligation of it, so no site here is a cross-document exclusion. Negotiation Protocol is the only normative section and holds three of the four sites, front:1 to front:3, all classified below; its remaining normative sentences are advisory and ungated -- options the server does not support 'should be omitted from the OACK; they should not cause an ERROR packet to be generated' (RFC2347-x-5), and a client receiving an unrequested option in an OACK 'should respond with an ERROR packet, with error code 8' (RFC2347-x-6). Its three-response tables for RRQ and WRQ, the fallback narrative for a server that does not implement options, and the two ways a client confirms an OACK are indicative. The Examples section is two packet traces. Security Considerations says the document adds no security to TFTP and no additional risk, so it states no countermeasure. The References, Authors' Addresses and the Full Copyright Statement bind no TFTP speaker; the copyright statement's one modal sentence is site front:4, excluded below. |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `front:4` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Boilerplate the extractor did not strip: the RFC Editor's Full Copyright Statement, which the site scan reaches because the sentence carries 'may not be modified' and 'must be followed'. It binds whoever copies or translates the document, under the Internet Society's copyright terms, and says nothing about TFTP option negotiation or about any byte on the wire. rfc/short/rfc2347.md declares no requirement for it. | However, this document itself may not be modified in any way, such as by removing the copyright notice or references to the Internet Society or other Internet organizations, except as needed for the purpose of developing Internet standards in which case the procedures for copyrights defined in the Internet Standards process must be followed, or as required to translate it into languages other than English. |

## Superseded

No document obsoletes RFC 2347, so its obligations are stated where they were written.
