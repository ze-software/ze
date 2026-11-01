# RFC 6608 - Subcodes for BGP Finite State Machine Error

BGP FSM Error subcodes. Every requirement this repository extracted from RFC 6608, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0 | of 3 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0 | of 3 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0 | of 3 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0 | of 3 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Proven by a recorded break | 0 | 0 of 0 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 3 | of 3 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the counts beside it are what says how Ze stands |
| Out of scope | 0 | of 3 gated MUSTs | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as a Ze gap. Scope, not coverage: it is counted in the gated whole the cards on this page add up to |
| Not applicable | 0 | of 3 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as a Ze gap. It is counted in the gated whole the cards add up to |
| Met below Ze | 0 | of 3 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze proven: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 3 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as a Ze gap. It is counted in the gated whole the cards add up to |
| Linux cannot do this | 0 | of 3 gated MUSTs | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as a Ze gap. It is counted in the gated whole the cards add up to and earns no proof credit |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 3 | of 3 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

The 9 counts marked as a part above are the whole of the 3 gated MUSTs: they add up to 3. Proven by a recorded break counts TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the count under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| Partial proof; remaining gap | ok | green at zero, RED above it: a tested clause cannot prove the whole requirement |
| No test at all | bad | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green counts above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Linux cannot do this | neutral | neutral at every value: an evidenced platform limit is neither a proof nor a scope exemption |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. It counts TAGGED UNITS, not obligations, so this count is not one of the parts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public row | BGP FSM Error subcodes |
| Enrolment | Enrolled |
| Requirements | 3 |
| Gated MUST-level | 3 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 3 |
| Declared gaps a test demonstrates | 0 |
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

**Area:** BGP FSM Error subcodes

**Proof:** 0 of 3 MUST proven

**Evidence:** -

**Unmet:** RFC6608-4-1, RFC6608-4-2, RFC6608-4-3


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 0 | one part of the gated population |
| Annotated (including scoped evidence) | 3 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **3** | every gated MUST falls in exactly one bucket above |

**Annotated (including scoped evidence) (3):** [`RFC6608-4-1`](#rfc6608-4-1), [`RFC6608-4-2`](#rfc6608-4-2), [`RFC6608-4-3`](#rfc6608-4-3)

## Figures

- **MUST bar**: Ze correctness gap 3
- **Overview bar**: Ze correctness gap 3

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Not walked | 0 |
| Mandatory register | not complete |
| MUST rows | 3 |
| MUST rows kept (the MUST bar) | 3 |
| Ze proven | 0 |
| OS verified | 0 |
| OS testing gap | 0 |
| Ze correctness gap | 3 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 0 |
| SHOULD proven | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD not ever | 0 |
| SHOULD not yet | 0 |
| SHOULD not decided | 0 |

The MUST bar counts the MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT rows of the walked RFCs that the stack owes: Ze proven means a recorded test break proves the row; a row a layer under Ze performs is OS verified when a test checks that layer on the state Ze installs, an OS testing gap until one does. The overview bar adds the MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT rows and the rows not owed, which do not bind Ze. An RFC that is not walked has no valid extraction sign-off, so its rows count in no figure. Every count is read from the RFC data files when the page loads.

Source: [data/rfc/rfc6608.json](../../../data/rfc/rfc6608.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC6608-4-1` | If a BGP speaker receives an unexpected message (e.g., KEEPALIVE/ UPDATE/ROUTE-REFRESH message) on a session in OpenSent state, it MUST send to the neighbor a NOTIFICATION message with the Error Code Finite State Machine Error and the Error Subcode "Receive Unexpected Message in OpenSent State". The Data field is a 1-octet, unsigned integer that indicates the type of the unexpected message. (Section 4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze defines and decodes the RFC 6608 subcodes (internal/component/bgp/message/notification.go:110-113, internal/component/bgp/format/decode.go:316-320) but never sends them -- the reactor dispatches received messages by type with no FSM-state guard (internal/component/bgp/reactor/session_read.go:260-273), and no code path emits a code-5 FSM Error NOTIFICATION with subcode 1 on an unexpected message in OpenSent. Disclosed in the docs/features/rfc-status.md RFC 6608 row |
| `RFC6608-4-2` | If a BGP speaker receives an unexpected message (e.g., OPEN/UPDATE/ ROUTE-REFRESH message) on a session in OpenConfirm state, it MUST send a NOTIFICATION message with the Error Code Finite State Machine Error and the Error Subcode "Receive Unexpected Message in OpenConfirm State" to the neighbor. The Data field is a 1-octet, unsigned integer that indicates the type of the unexpected message. (Section 4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** same root cause as 4-1 -- ze decodes the RFC 6608 subcodes but no reactor path sends a code-5 FSM Error NOTIFICATION with subcode 2 on an unexpected message in OpenConfirm (session_read.go:260-273). Disclosed in the docs/features/rfc-status.md RFC 6608 row |
| `RFC6608-4-3` | If a BGP speaker receives an unexpected message (e.g., OPEN message) on a session in Established State, it MUST send to the neighbor a NOTIFICATION message with the Error Code Finite State Machine Error and the Error Subcode "Receive Unexpected Message in Established State". The Data field is a 1-octet, unsigned integer that indicates the type of the unexpected message. (Section 4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** same root cause as 4-1 -- ze decodes the RFC 6608 subcodes but no reactor path sends a code-5 FSM Error NOTIFICATION with subcode 3 on an unexpected message in Established (session_read.go:260-273). Disclosed in the docs/features/rfc-status.md RFC 6608 row |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC6608-4-1`](#rfc6608-4-1) If a BGP speaker receives an unexpected message (e.g., KEEPALIVE/ UPDATE/ROUTE-REFRESH message) on a session in OpenSent state, it MUST send to the neighbor a NOTIFICATION message with the Error Code Finite State Machine Error and the Error Subcode "Receive Unexpected Message in OpenSent State". The Data field is a 1-octet, unsigned integer that indicates the type of the unexpected message. (Section 4) | {gap}, no test | ze defines and decodes the RFC 6608 subcodes (internal/component/bgp/message/notification.go:110-113, internal/component/bgp/format/decode.go:316-320) but never sends them -- the reactor dispatches received messages by type with no FSM-state guard (internal/component/bgp/reactor/session_read.go:260-273), and no code path emits a code-5 FSM Error NOTIFICATION with subcode 1 on an unexpected message in OpenSent. Disclosed in the docs/features/rfc-status.md RFC 6608 row |
| [`RFC6608-4-2`](#rfc6608-4-2) If a BGP speaker receives an unexpected message (e.g., OPEN/UPDATE/ ROUTE-REFRESH message) on a session in OpenConfirm state, it MUST send a NOTIFICATION message with the Error Code Finite State Machine Error and the Error Subcode "Receive Unexpected Message in OpenConfirm State" to the neighbor. The Data field is a 1-octet, unsigned integer that indicates the type of the unexpected message. (Section 4) | {gap}, no test | same root cause as 4-1 -- ze decodes the RFC 6608 subcodes but no reactor path sends a code-5 FSM Error NOTIFICATION with subcode 2 on an unexpected message in OpenConfirm (session_read.go:260-273). Disclosed in the docs/features/rfc-status.md RFC 6608 row |
| [`RFC6608-4-3`](#rfc6608-4-3) If a BGP speaker receives an unexpected message (e.g., OPEN message) on a session in Established State, it MUST send to the neighbor a NOTIFICATION message with the Error Code Finite State Machine Error and the Error Subcode "Receive Unexpected Message in Established State". The Data field is a 1-octet, unsigned integer that indicates the type of the unexpected message. (Section 4) | {gap}, no test | same root cause as 4-1 -- ze decodes the RFC 6608 subcodes but no reactor path sends a code-5 FSM Error NOTIFICATION with subcode 3 on an unexpected message in Established (session_read.go:260-273). Disclosed in the docs/features/rfc-status.md RFC 6608 row |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC6608-4-1`](#rfc6608-4-1)

If a BGP speaker receives an unexpected message (e.g., KEEPALIVE/ UPDATE/ROUTE-REFRESH message) on a session in OpenSent state, it MUST send to the neighbor a NOTIFICATION message with the Error Code Finite State Machine Error and the Error Subcode "Receive Unexpected Message in OpenSent State". The Data field is a 1-octet, unsigned integer that indicates the type of the unexpected message. (Section 4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6608-4-1, so no unit is bound to it.

### [`RFC6608-4-2`](#rfc6608-4-2)

If a BGP speaker receives an unexpected message (e.g., OPEN/UPDATE/ ROUTE-REFRESH message) on a session in OpenConfirm state, it MUST send a NOTIFICATION message with the Error Code Finite State Machine Error and the Error Subcode "Receive Unexpected Message in OpenConfirm State" to the neighbor. The Data field is a 1-octet, unsigned integer that indicates the type of the unexpected message. (Section 4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6608-4-2, so no unit is bound to it.

### [`RFC6608-4-3`](#rfc6608-4-3)

If a BGP speaker receives an unexpected message (e.g., OPEN message) on a session in Established State, it MUST send to the neighbor a NOTIFICATION message with the Error Code Finite State Machine Error and the Error Subcode "Receive Unexpected Message in Established State". The Data field is a 1-octet, unsigned integer that indicates the type of the unexpected message. (Section 4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC6608-4-3, so no unit is bound to it.

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | rfc2119 |
| Source | rfc/full/rfc6608.txt |
| Source fingerprint | a864ccc2fecb4951 |
| Record | rfc/extraction/rfc6608.json |
| Mapped sentences | 3 |
| Declined as scope | 0 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | walked | not stated |
| `1` | not stated | 0 | walked | not stated |
| `2` | not stated | 0 | walked | not stated |
| `3` | not stated | 0 | walked | not stated |
| `4` | not stated | 3 | walked | not stated |
| `5` | not stated | 0 | walked | not stated |
| `6` | not stated | 0 | walked | not stated |
| `7` | not stated | 0 | walked | not stated |
| `8` | not stated | 0 | walked | not stated |
| `9` | not stated | 0 | walked | not stated |
| `9.1` | not stated | 0 | walked | not stated |
| `9.2` | not stated | 0 | walked | not stated |

### Excluded sentences

The walk over RFC 6608 declined no sentence: every site it found is mapped to a requirement.

## Superseded

No document obsoletes RFC 6608, so its obligations are stated where they were written.
