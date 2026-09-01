# RFC 2205 - Resource ReSerVation Protocol (RSVP) -- Version 1 Functional Specification

Experimental. Every requirement this repository extracted from RFC 2205, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 33.3% | 2 of 6 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 33.3% | 2 of 6 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 6 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 0.0% | 0 of 7 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 6 | of 10 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 6 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 33.3% | 2 of 6 binding obligations | no test carries the requirement id, whether or not a gap states why |

The 4 shares marked as a part above are the whole of the 6 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Public status | Experimental |
| Enrolment | Enrolled |
| Requirements | 10 |
| Gated MUST-level | 6 |
| Obligations that bind Ze | 6 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 2 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 10 |
| Tagged units | 7 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc2205.md` |
| Requirement shard | `rfc/requirements/rfc2205.md` |
| RFC text | `rfc/full/rfc2205.txt` |

## Enrolment

Enrolled: RSVP version 1 base protocol (codec shared by ze's RSVP-TE plugin): six MUST-level requirements. 3.1-1 (Version MUST be 1) is met with positive+negative tags (encode round-trip and bad-version decode reject, internal/plugins/rsvpte/wire.go). 3.1-2 (reserved octet 0 on send) and 3.1.2-1 (object length multiple of 4) are {single-polarity: positive} with send-side tests (wire.go and the object encoders). 3.10-1 (reject an unknown Class-Num of the form 0bbbbbbb) is met with positive+negative tags: DecodeMessage classifies by the high-order bit (classifyUnknownClass, wire.go) and engine.rejectUnknownObject answers a PATH with Error Code 13. 3.1-3 (verify checksum on receipt) and x-1 (IP Router Alert in PATH) are {gap}: ze's receive path (wire.go DecodeHeader) and raw-socket send (transport_linux.go) omit these. Disclosed in the docs/features/rfc-status.md RFC 2205 row.

## What the public ledger says

**Status:** Experimental

**What the ledger says is covered:**

- RSVP base common-header and object codec used by RSVP-TE: Version-1 header enforced on decode, reserved octet zeroed on send, every emitted object length a multiple of 4
- tests bound per requirement in [`rfc/requirements/rfc2205.md`](https://github.com/ze-software/ze/blob/main/rfc/requirements/rfc2205.md).


**What the ledger says remains:**

Two MUST gaps gated in [`rfc/short/rfc2205.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc2205.md): the receive path does not verify the RSVP checksum or drop bad-checksum messages (3.1); and PATH is sent without the IP Router Alert option.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 2 | one part of the gated population |
| Annotated instead of tested | 4 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **6** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (2):** [`RFC2205-3.1-1`](#rfc2205-3.1-1), [`RFC2205-3.10-1`](#rfc2205-3.10-1)

**Annotated instead of tested (4):** [`RFC2205-3.1-2`](#rfc2205-3.1-2), [`RFC2205-3.1.2-1`](#rfc2205-3.1.2-1), [`RFC2205-3.1-3`](#rfc2205-3.1-3), [`RFC2205-x-1`](#rfc2205-x-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC2205-3.1-1` | Version field MUST be 1 (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestRSVPHeaderRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_test.go#L405). **negative:** `unit/verify` [`TestRSVPDecodeHeaderBadVersion`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_test.go#L439) |
| `RFC2205-3.1-2` | Reserved field in common header MUST be zero (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestRSVPReservedByteZeroOnSend`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_test.go#L581). **negative:** no negative test. **{single-polarity}:** ze sets the reserved byte to 0 on send (internal/plugins/rsvpte/wire.go:177) and the RFC does not require receivers to reject a nonzero reserved field, so no negative case exists |
| `RFC2205-3.1.2-1` | Object lengths MUST be a multiple of 4 (§3.1.2) | MUST | 3.1.2 | **positive:** `unit/verify` [`TestRSVPObjectLengthMultipleOfFour`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_test.go#L602). **negative:** no negative test. **{single-polarity}:** every RSVP object encoder in internal/plugins/rsvpte/wire.go emits a length that is a multiple of 4; the receive path does not enforce %4, so the reject/negative polarity has no code path |
| `RFC2205-3.1-3` | Checksum MUST be verified on receipt; drop messages with bad checksum (§3.1) | MUST | 3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze computes the RFC 2205 checksum on send (internal/plugins/rsvpte/build.go:48 internetChecksum) but the receive path (internal/plugins/rsvpte/wire.go:190 DecodeHeader / DecodeMessage) does not verify it or drop bad-checksum messages |
| `RFC2205-x-1` | IP Router Alert option MUST be set in PATH messages (Transport) | MUST | x | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze sends PATH over a raw protocol-46 socket (internal/plugins/rsvpte/transport_linux.go:35-69) and never sets the IP Router Alert option |
| `RFC2205-3.10-1` | Unknown Class-Num of the form 0bbbbbbb: reject the entire message and return an "Unknown Object Class" error (§3.10) | MUST | 3.10 | **positive:** `unit/verify` [`TestDecodeUnknownObjectClass`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_test.go#L718). **negative:** `unit/verify` [`TestDecodeUnknownObjectClass`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_test.go#L725). **negative:** `unit/verify` [`TestEnginePathWithIgnorableObjectAccepted`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L935) |
| `RFC2205-3.7-1` | Refresh period SHOULD be jittered by +/- 50% of R to prevent synchronization (§3.7) | SHOULD | 3.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC2205-x-2` | Jitter: SHOULD randomize refresh timing (Soft-State Model) | SHOULD | x | **positive:** no positive test. **negative:** no negative test |
| `RFC2205-3.10-2` | Unknown Class-Num of the form 10bbbbbb: ignore the object, neither forwarding it nor sending an error message (§3.10) | MAY | 3.10 | **positive:** no positive test. **negative:** no negative test |
| `RFC2205-3.10-3` | Unknown Class-Num of the form 11bbbbbb: ignore the object but forward it unexamined and unmodified in every message resulting from this one (§3.10) | MAY | 3.10 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC2205-3.1-3`](#rfc2205-3.1-3) Checksum MUST be verified on receipt; drop messages with bad checksum (§3.1) | {gap}, no test | ze computes the RFC 2205 checksum on send (internal/plugins/rsvpte/build.go:48 internetChecksum) but the receive path (internal/plugins/rsvpte/wire.go:190 DecodeHeader / DecodeMessage) does not verify it or drop bad-checksum messages |
| [`RFC2205-x-1`](#rfc2205-x-1) IP Router Alert option MUST be set in PATH messages (Transport) | {gap}, no test | ze sends PATH over a raw protocol-46 socket (internal/plugins/rsvpte/transport_linux.go:35-69) and never sets the IP Router Alert option |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC2205-3.1-1`](#rfc2205-3.1-1)

Version field MUST be 1 (§3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRSVPDecodeHeaderBadVersion`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_test.go#L439) | unit/verify | unproven |
| positive | [`TestRSVPHeaderRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_test.go#L405) | unit/verify | unproven |

### [`RFC2205-3.1-2`](#rfc2205-3.1-2)

Reserved field in common header MUST be zero (§3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRSVPReservedByteZeroOnSend`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_test.go#L581) | unit/verify | unproven |

### [`RFC2205-3.1.2-1`](#rfc2205-3.1.2-1)

Object lengths MUST be a multiple of 4 (§3.1.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRSVPObjectLengthMultipleOfFour`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_test.go#L602) | unit/verify | unproven |

### [`RFC2205-3.1-3`](#rfc2205-3.1-3)

Checksum MUST be verified on receipt; drop messages with bad checksum (§3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-3.1-3, so no unit is bound to it.

### [`RFC2205-x-1`](#rfc2205-x-1)

IP Router Alert option MUST be set in PATH messages (Transport)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-x-1, so no unit is bound to it.

### [`RFC2205-3.10-1`](#rfc2205-3.10-1)

Unknown Class-Num of the form 0bbbbbbb: reject the entire message and return an "Unknown Object Class" error (§3.10)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestEnginePathWithIgnorableObjectAccepted`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L935) | unit/verify | unproven |
| negative | [`TestDecodeUnknownObjectClass`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_test.go#L725) | unit/verify | unproven |
| positive | [`TestDecodeUnknownObjectClass`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_test.go#L718) | unit/verify | unproven |

## Extraction sign-off

No extraction sign-off exists for RFC 2205, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 2205, so its obligations are stated where they were written.
