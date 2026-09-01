# RFC 2347 - TFTP Option Extension

Supported. Every requirement this repository extracted from RFC 2347, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 100.0% | 2 of 2 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 2 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 2 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 0.0% | 0 of 2 binding obligations | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 0.0% | 0 of 4 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 4 | of 6 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 2 | of 4 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

The 4 shares marked as a part above are the whole of the 2 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Public status | Supported |
| Enrolment | Enrolled |
| Requirements | 6 |
| Gated MUST-level | 4 |
| Obligations that bind Ze | 2 |
| Not applicable, so out of scope | 2 |
| Declared gaps | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 4 |
| Tagged units | 4 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc2347.md` |
| Requirement shard | `rfc/requirements/rfc2347.md` |
| RFC text | `rfc/full/rfc2347.txt` |

## Enrolment

Enrolled: TFTP Option Extension: four MUST-level requirements. The two server-side requirements are tested with both polarities via loopback OACK round-trips in rfc2347_option_negotiation_test.go (producer: parseRRQ + sendOACKAndWait oackOpts assembly, internal/plugins/tftpserver/handler.go): RFC2347-x-1 (server MUST NOT include in the OACK any option not requested by the client) via TestRFC2347ServerOACKOnlyRequestedOptions (requesting only blksize yields an OACK with blksize but no tsize); RFC2347-x-3 (an option not acknowledged is ignored as if never requested) via TestRFC2347ServerIgnoresUnacknowledgedOption (a requested-but-unsupported windowsize is omitted from the OACK and the server falls back to lockstep, handler.go:276, while the supported blksize is still acknowledged). The two client-side requirements are {not-applicable}: RFC2347-x-2 (client MUST use acknowledged options) and RFC2347-x-4 (client MUST NOT use unacknowledged options) govern a TFTP CLIENT, and Ze ships only a TFTP server with no client. No SHOULD/MAY requirements are gated.

## What the public ledger says

**Status:** Supported

**What the ledger says is covered:**

Option negotiation for TFTP provisioning paths.

**What the ledger says remains:**

No tracked gap in current source anchors.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 2 | one part of the gated population |
| Annotated instead of tested | 2 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **4** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (2):** [`RFC2347-x-1`](#rfc2347-x-1), [`RFC2347-x-3`](#rfc2347-x-3)

**Annotated instead of tested (2):** [`RFC2347-x-2`](#rfc2347-x-2), [`RFC2347-x-4`](#rfc2347-x-4)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC2347-x-1` | Server MUST NOT include in the OACK any option which had not been specifically requested by the client (Negotiation Protocol) | MUST NOT | x | **positive:** `unit/verify` [`TestRFC2347ServerOACKOnlyRequestedOptions`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2347_option_negotiation_test.go#L78). **negative:** `unit/verify` [`TestRFC2347ServerOACKOnlyRequestedOptions`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2347_option_negotiation_test.go#L82) |
| `RFC2347-x-2` | If multiple options were requested, the client MUST use those options which were acknowledged by the server (Negotiation Protocol) | MUST | x | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** This requirement governs the TFTP CLIENT (it MUST use the options the server acknowledged). Ze ships only a TFTP SERVER (internal/plugins/tftpserver/handler.go) and has no TFTP client, so there is no client-side option-consumption code path to which this applies. |
| `RFC2347-x-3` | An option not acknowledged by the server must be ignored by the client and server as if it were never requested (Negotiation Protocol) | MUST | x | **positive:** `unit/verify` [`TestRFC2347ServerIgnoresUnacknowledgedOption`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2347_option_negotiation_test.go#L103). **negative:** `unit/verify` [`TestRFC2347ServerIgnoresUnacknowledgedOption`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2347_option_negotiation_test.go#L108) |
| `RFC2347-x-4` | If multiple options were requested, the client MUST NOT use those options which were not acknowledged by the server (Negotiation Protocol) | MUST NOT | x | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** This requirement governs the TFTP CLIENT (it MUST NOT use options the server did not acknowledge). Ze has no TFTP client (only the server in internal/plugins/tftpserver/handler.go), so no client-side code path could use an unacknowledged option. |
| `RFC2347-x-5` | Unrecognized options SHOULD be omitted from the OACK, not cause an ERROR packet (Negotiation Protocol) | SHOULD | x | **positive:** no positive test. **negative:** no negative test |
| `RFC2347-x-6` | If client receives an OACK containing an unrequested option, it SHOULD respond with ERROR code 8 and terminate (Negotiation Protocol) | SHOULD | x | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC2347-x-2`](#rfc2347-x-2) If multiple options were requested, the client MUST use those options which were acknowledged by the server (Negotiation Protocol) | no test | no test carries this requirement id; annotated {not-applicable}: This requirement governs the TFTP CLIENT (it MUST use the options the server acknowledged). Ze ships only a TFTP SERVER (internal/plugins/tftpserver/handler.go) and has no TFTP client, so there is no client-side option-consumption code path to which this applies. |
| [`RFC2347-x-4`](#rfc2347-x-4) If multiple options were requested, the client MUST NOT use those options which were not acknowledged by the server (Negotiation Protocol) | no test | no test carries this requirement id; annotated {not-applicable}: This requirement governs the TFTP CLIENT (it MUST NOT use options the server did not acknowledge). Ze has no TFTP client (only the server in internal/plugins/tftpserver/handler.go), so no client-side code path could use an unacknowledged option. |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC2347-x-1`](#rfc2347-x-1)

Server MUST NOT include in the OACK any option which had not been specifically requested by the client (Negotiation Protocol)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2347ServerOACKOnlyRequestedOptions`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2347_option_negotiation_test.go#L82) | unit/verify | unproven |
| positive | [`TestRFC2347ServerOACKOnlyRequestedOptions`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2347_option_negotiation_test.go#L78) | unit/verify | unproven |

### [`RFC2347-x-2`](#rfc2347-x-2)

If multiple options were requested, the client MUST use those options which were acknowledged by the server (Negotiation Protocol)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2347-x-2, so no unit is bound to it.

### [`RFC2347-x-3`](#rfc2347-x-3)

An option not acknowledged by the server must be ignored by the client and server as if it were never requested (Negotiation Protocol)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2347ServerIgnoresUnacknowledgedOption`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2347_option_negotiation_test.go#L108) | unit/verify | unproven |
| positive | [`TestRFC2347ServerIgnoresUnacknowledgedOption`](https://github.com/ze-software/ze/blob/main/internal/plugins/tftpserver/rfc2347_option_negotiation_test.go#L103) | unit/verify | unproven |

### [`RFC2347-x-4`](#rfc2347-x-4)

If multiple options were requested, the client MUST NOT use those options which were not acknowledged by the server (Negotiation Protocol)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2347-x-4, so no unit is bound to it.

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
| `front` | The whole document | 4 | walked | The whole document. RFC 2347 carries no numbered section headings, so the derivation returns one section spanning the title block to the Full Copyright Statement, and a skip of it would hide every gated id behind a skip kind. Walked heading by heading. Status of this Memo, the Copyright Notice and the Abstract are indicative. The Introduction says the mechanism is a backward-compatible extension enforcing a request-respond-acknowledge sequence and names RFC 2348 and RFC 2349 as the options it was created to carry; it directs nobody. Packet Formats is the wire-format section, written entirely in the indicative: options are appended to the Read Request or Write Request as NUL-terminated name/value pairs, a new opcode 6 (OACK) acknowledges them, a new error code 8 terminates a transfer over option negotiation, the order of options is not significant, and 'The maximum size of a request packet is 512 octets.' Ze reads those bytes in parseRRQ and writes them in buildOACK (internal/plugins/tftpserver/handler.go), and rfc/short/rfc2347.md carries the two figures and the constants; no gated row is read from this material. Packet Formats also defers three fields to RFC 1350 in the indicative ('as defined in [1]' for opc, filename and mode), which cites the base document rather than restating an obligation of it, so no site here is a cross-document exclusion. Negotiation Protocol is the only normative section and holds three of the four sites, front:1 to front:3, all classified below; its remaining normative sentences are advisory and ungated -- options the server does not support 'should be omitted from the OACK; they should not cause an ERROR packet to be generated' (RFC2347-x-5), and a client receiving an unrequested option in an OACK 'should respond with an ERROR packet, with error code 8' (RFC2347-x-6). Its three-response tables for RRQ and WRQ, the fallback narrative for a server that does not implement options, and the two ways a client confirms an OACK are indicative. The Examples section is two packet traces. Security Considerations says the document adds no security to TFTP and no additional risk, so it states no countermeasure. The References, Authors' Addresses and the Full Copyright Statement bind no TFTP speaker; the copyright statement's one modal sentence is site front:4, excluded below. RFC2347-x-4 is declared unsourced here: it is the second half of site front:3, which the sentence splitter returns whole, so the obligation has no site locator of its own. |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `front:4` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Boilerplate the extractor did not strip: the RFC Editor's Full Copyright Statement, which the site scan reaches because the sentence carries 'may not be modified' and 'must be followed'. It binds whoever copies or translates the document, under the Internet Society's copyright terms, and says nothing about TFTP option negotiation or about any byte on the wire. rfc/short/rfc2347.md declares no requirement for it. | However, this document itself may not be modified in any way, such as by removing the copyright notice or references to the Internet Society or other Internet organizations, except as needed for the purpose of developing Internet standards in which case the procedures for copyrights defined in the Internet Standards process must be followed, or as required to translate it into languages other than English. |

## Superseded

No document obsoletes RFC 2347, so its obligations are stated where they were written.
