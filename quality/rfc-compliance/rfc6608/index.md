# RFC 6608 - Subcodes for BGP Finite State Machine Error

Partial. Every requirement this repository extracted from RFC 6608, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0.0% | 0 of 3 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 3 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 3 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 0.0% | 0 of 0 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 3 | of 3 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 3 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 100.0% | 3 of 3 binding obligations | no test carries the requirement id, whether or not a gap states why |

The 4 shares marked as a part above are the whole of the 3 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Public status | Partial |
| Enrolment | Enrolled |
| Requirements | 3 |
| Gated MUST-level | 3 |
| Obligations that bind Ze | 3 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 3 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 0 |
| Tagged units | 0 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc6608.md` |
| Requirement shard | `rfc/requirements/rfc6608.md` |
| RFC text | `rfc/full/rfc6608.txt` |

## Enrolment

Enrolled: Subcodes for BGP Finite State Machine Error: three MUSTs requiring a code-5 FSM Error NOTIFICATION with subcode 1/2/3 (plus the 1-octet message type) on an unexpected message in OpenSent/OpenConfirm/Established. All three are {gap}: ze defines and decodes the subcodes (message/notification.go:110-113, format/decode.go:316-320) but never originates them -- the reactor dispatches received messages by type with no FSM-state guard (reactor/session_read.go:260-273) and no path emits these subcodes. Disclosed in the docs/features/rfc-status.md RFC 6608 row (Partial).

## What the public ledger says

**Status:** Partial

**What the ledger says is covered:**

The RFC 6608 FSM Error subcodes (1 OpenSent, 2 OpenConfirm, 3 Established) are defined and decoded for display of a received NOTIFICATION.

**What the ledger says remains:**

Three MUST gaps, gated in [`rfc/short/rfc6608.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc6608.md): ze does not originate a code-5 FSM Error NOTIFICATION with these subcodes on an unexpected message; the reactor dispatches received messages by type without an FSM-state guard.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 0 | one part of the gated population |
| Annotated instead of tested | 3 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **3** | every gated MUST falls in exactly one bucket above |

**Annotated instead of tested (3):** [`RFC6608-4-1`](#rfc6608-4-1), [`RFC6608-4-2`](#rfc6608-4-2), [`RFC6608-4-3`](#rfc6608-4-3)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC6608-4-1` | On receiving an unexpected message in OpenSent state, MUST send NOTIFICATION with Error Code 5 (FSM Error) and Subcode 1 ("Receive Unexpected Message in OpenSent State"), with Data field containing the 1-octet message type (Section 4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze defines and decodes the RFC 6608 subcodes (internal/component/bgp/message/notification.go:110-113, internal/component/bgp/format/decode.go:316-320) but never sends them -- the reactor dispatches received messages by type with no FSM-state guard (internal/component/bgp/reactor/session_read.go:260-273), and no code path emits a code-5 FSM Error NOTIFICATION with subcode 1 on an unexpected message in OpenSent. Disclosed in the docs/features/rfc-status.md RFC 6608 row |
| `RFC6608-4-2` | On receiving an unexpected message in OpenConfirm state, MUST send NOTIFICATION with Error Code 5 (FSM Error) and Subcode 2 ("Receive Unexpected Message in OpenConfirm State"), with Data field containing the 1-octet message type (Section 4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** same root cause as 4-1 -- ze decodes the RFC 6608 subcodes but no reactor path sends a code-5 FSM Error NOTIFICATION with subcode 2 on an unexpected message in OpenConfirm (session_read.go:260-273). Disclosed in the docs/features/rfc-status.md RFC 6608 row |
| `RFC6608-4-3` | On receiving an unexpected message in Established state, MUST send NOTIFICATION with Error Code 5 (FSM Error) and Subcode 3 ("Receive Unexpected Message in Established State"), with Data field containing the 1-octet message type (Section 4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** same root cause as 4-1 -- ze decodes the RFC 6608 subcodes but no reactor path sends a code-5 FSM Error NOTIFICATION with subcode 3 on an unexpected message in Established (session_read.go:260-273). Disclosed in the docs/features/rfc-status.md RFC 6608 row |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC6608-4-1`](#rfc6608-4-1) On receiving an unexpected message in OpenSent state, MUST send NOTIFICATION with Error Code 5 (FSM Error) and Subcode 1 ("Receive Unexpected Message in OpenSent State"), with Data field containing the 1-octet message type (Section 4) | {gap}, no test | ze defines and decodes the RFC 6608 subcodes (internal/component/bgp/message/notification.go:110-113, internal/component/bgp/format/decode.go:316-320) but never sends them -- the reactor dispatches received messages by type with no FSM-state guard (internal/component/bgp/reactor/session_read.go:260-273), and no code path emits a code-5 FSM Error NOTIFICATION with subcode 1 on an unexpected message in OpenSent. Disclosed in the docs/features/rfc-status.md RFC 6608 row |
| [`RFC6608-4-2`](#rfc6608-4-2) On receiving an unexpected message in OpenConfirm state, MUST send NOTIFICATION with Error Code 5 (FSM Error) and Subcode 2 ("Receive Unexpected Message in OpenConfirm State"), with Data field containing the 1-octet message type (Section 4) | {gap}, no test | same root cause as 4-1 -- ze decodes the RFC 6608 subcodes but no reactor path sends a code-5 FSM Error NOTIFICATION with subcode 2 on an unexpected message in OpenConfirm (session_read.go:260-273). Disclosed in the docs/features/rfc-status.md RFC 6608 row |
| [`RFC6608-4-3`](#rfc6608-4-3) On receiving an unexpected message in Established state, MUST send NOTIFICATION with Error Code 5 (FSM Error) and Subcode 3 ("Receive Unexpected Message in Established State"), with Data field containing the 1-octet message type (Section 4) | {gap}, no test | same root cause as 4-1 -- ze decodes the RFC 6608 subcodes but no reactor path sends a code-5 FSM Error NOTIFICATION with subcode 3 on an unexpected message in Established (session_read.go:260-273). Disclosed in the docs/features/rfc-status.md RFC 6608 row |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC6608-4-1`](#rfc6608-4-1)

On receiving an unexpected message in OpenSent state, MUST send NOTIFICATION with Error Code 5 (FSM Error) and Subcode 1 ("Receive Unexpected Message in OpenSent State"), with Data field containing the 1-octet message type (Section 4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6608-4-1, so no unit is bound to it.

### [`RFC6608-4-2`](#rfc6608-4-2)

On receiving an unexpected message in OpenConfirm state, MUST send NOTIFICATION with Error Code 5 (FSM Error) and Subcode 2 ("Receive Unexpected Message in OpenConfirm State"), with Data field containing the 1-octet message type (Section 4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6608-4-2, so no unit is bound to it.

### [`RFC6608-4-3`](#rfc6608-4-3)

On receiving an unexpected message in Established state, MUST send NOTIFICATION with Error Code 5 (FSM Error) and Subcode 3 ("Receive Unexpected Message in Established State"), with Data field containing the 1-octet message type (Section 4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6608-4-3, so no unit is bound to it.

## Extraction sign-off

No extraction sign-off exists for RFC 6608, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 6608, so its obligations are stated where they were written.
