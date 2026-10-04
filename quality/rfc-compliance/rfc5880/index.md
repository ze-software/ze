# RFC 5880 - Bidirectional Forwarding Detection (BFD)

Partial. Every requirement this repository extracted from RFC 5880, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 80.2% | 89 of 111 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 8.1% | 9 of 111 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 111 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 111 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Proven by a recorded break | 79.7% | 228 of 286 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 111 | of 137 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 1 | of 111 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.9% | 1 of 111 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 111 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 111 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 10.8% | 12 of 111 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Audit verdicts | 102 | of 111 gated MUSTs judged | 2 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

The 8 shares marked as a part above are the whole of the 111 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| Partial proof; remaining gap | ok | green at zero, RED above it: a tested clause cannot prove the whole requirement |
| No test at all | bad | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green shares above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | bad | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Partial |
| Enrolment | Enrolled |
| Requirements | 137 |
| Gated MUST-level | 111 |
| Not applicable, so out of scope | 1 |
| Declared gaps | 14 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 286 |
| Tagged units | 286 |
| Recorded audit verdicts | 102 |
| Discrimination records | 228 |
| Summary | `rfc/short/rfc5880.md` |
| Requirement shard | `rfc/requirements/rfc5880.md` |
| RFC text | `rfc/full/rfc5880.txt` |

## Enrolment

Enrolled: Bidirectional Forwarding Detection (BFD)

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

- Control packet codec and the Section 6.8.6 structural reception checks, the Section 6.8.6 transition table, Section 6.8.1 state variables, Active/Passive roles, slow start and the Poll/Final sequence, the D-bit guard, detection and echo timers with Section 6.8.7 jitter, Simple Password and Keyed / Meticulous Keyed MD5/SHA1 authentication, echo scheduling and demultiplexing, the single-hop TTL gate, metrics and show commands
- MUST-level requirements bound per requirement in [`rfc/requirements/rfc5880.md`](https://github.com/ze-software/ze/blob/main/rfc/requirements/rfc5880.md).


**What the ledger says remains**

Fourteen MUST and SHOULD gaps, gated in [`rfc/short/rfc5880.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc5880.md), twelve of them at MUST level: Demand mode is not driven -- bfd.DemandMode has no writer and the stored remote D bit is never read, so no Poll is raised on a D-bit or content change and periodic transmission is never suppressed ([`RFC5880-6.6-2`](#rfc5880-6.6-2), 6.6-3, 6.8.6-14, 6.8.7-7, 6.8.17-1); periodic transmission also continues when bfd.RemoteMinRxInterval is zero ([`RFC5880-6.8.7-6`](#rfc5880-6.8.7-6)); a reduced bfd.RequiredMinRxInterval enters the Detection Time at once instead of at Poll termination ([`RFC5880-6.8.3-4`](#rfc5880-6.8.3-4)); bfd.XmitAuthSeq starts at zero rather than a random value and bfd.AuthSeqKnown is never cleared after twice the Detection Time ([`RFC5880-6.8.1-11`](#rfc5880-6.8.1-11), 6.8.1-13); there is no forwarding-plane-reset hook, so diagnostic 4 has no producer ([`RFC5880-6.8.15-1`](#rfc5880-6.8.15-1)); and no congestion-control mechanism governs the transmit rate ([`RFC5880-7-1`](#rfc5880-7-1), 7-2).

- **The two SHOULD gaps:** the key management accepts no hexadecimal form for a Keyed MD5 or Keyed SHA1 key ([`RFC5880-6.7.3-14`](#rfc5880-6.7.3-14), 6.7.4-8). Final-packet rate limiting is absent by design and annotated not-applicable. IPv6 transport coverage is tracked with BFD.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 89 | one part of the gated population |
| Annotated (including scoped evidence) | 22 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **111** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (89):** [`RFC5880-4.1-1`](#rfc5880-4.1-1), [`RFC5880-4.1-2`](#rfc5880-4.1-2), [`RFC5880-6.3-1`](#rfc5880-6.3-1), [`RFC5880-6.8.1-3`](#rfc5880-6.8.1-3), [`RFC5880-6.8.1-4`](#rfc5880-6.8.1-4), [`RFC5880-6.8.1-5`](#rfc5880-6.8.1-5), [`RFC5880-6.8.1-6`](#rfc5880-6.8.1-6), [`RFC5880-6.8.1-7`](#rfc5880-6.8.1-7), [`RFC5880-6.8.1-8`](#rfc5880-6.8.1-8), [`RFC5880-6.8.1-9`](#rfc5880-6.8.1-9), [`RFC5880-6.8.1-10`](#rfc5880-6.8.1-10), [`RFC5880-6.8.1-12`](#rfc5880-6.8.1-12), [`RFC5880-6.8.1-14`](#rfc5880-6.8.1-14), [`RFC5880-6.1-1`](#rfc5880-6.1-1), [`RFC5880-6.1-2`](#rfc5880-6.1-2), [`RFC5880-6.8.3-1`](#rfc5880-6.8.3-1), [`RFC5880-6.8.3-2`](#rfc5880-6.8.3-2), [`RFC5880-6.8.3-3`](#rfc5880-6.8.3-3), [`RFC5880-6.8.3-5`](#rfc5880-6.8.3-5), [`RFC5880-6.8.3-6`](#rfc5880-6.8.3-6), [`RFC5880-6.5-1`](#rfc5880-6.5-1), [`RFC5880-6.5-2`](#rfc5880-6.5-2), [`RFC5880-6.6-1`](#rfc5880-6.6-1), [`RFC5880-6.7-1`](#rfc5880-6.7-1), [`RFC5880-4.2-1`](#rfc5880-4.2-1), [`RFC5880-6.7.2-1`](#rfc5880-6.7.2-1), [`RFC5880-6.7.2-8`](#rfc5880-6.7.2-8), [`RFC5880-6.7.3-1`](#rfc5880-6.7.3-1), [`RFC5880-6.7.3-2`](#rfc5880-6.7.3-2), [`RFC5880-6.7.3-4`](#rfc5880-6.7.3-4), [`RFC5880-6.7.3-13`](#rfc5880-6.7.3-13), [`RFC5880-6.7.4-1`](#rfc5880-6.7.4-1), [`RFC5880-6.7.4-2`](#rfc5880-6.7.4-2), [`RFC5880-6.7.4-4`](#rfc5880-6.7.4-4), [`RFC5880-6.7.4-5`](#rfc5880-6.7.4-5), [`RFC5880-6.7.4-7`](#rfc5880-6.7.4-7), [`RFC5880-6.7.3-15`](#rfc5880-6.7.3-15), [`RFC5880-6.7.4-10`](#rfc5880-6.7.4-10), [`RFC5880-6.7.4-11`](#rfc5880-6.7.4-11), [`RFC5880-6.8.6-1`](#rfc5880-6.8.6-1), [`RFC5880-6.8.6-2`](#rfc5880-6.8.6-2), [`RFC5880-6.8.6-3`](#rfc5880-6.8.6-3), [`RFC5880-6.8.6-4`](#rfc5880-6.8.6-4), [`RFC5880-6.8.6-5`](#rfc5880-6.8.6-5), [`RFC5880-6.8.6-6`](#rfc5880-6.8.6-6), [`RFC5880-6.8.6-7`](#rfc5880-6.8.6-7), [`RFC5880-6.8.6-8`](#rfc5880-6.8.6-8), [`RFC5880-6.8.6-9`](#rfc5880-6.8.6-9), [`RFC5880-6.8.6-10`](#rfc5880-6.8.6-10), [`RFC5880-6.8.6-11`](#rfc5880-6.8.6-11), [`RFC5880-6.8.6-12`](#rfc5880-6.8.6-12), [`RFC5880-6.8.6-13`](#rfc5880-6.8.6-13), [`RFC5880-6.8.6-16`](#rfc5880-6.8.6-16), [`RFC5880-6.8.6-18`](#rfc5880-6.8.6-18), [`RFC5880-6.8.7-1`](#rfc5880-6.8.7-1), [`RFC5880-6.8.7-2`](#rfc5880-6.8.7-2), [`RFC5880-6.8.7-3`](#rfc5880-6.8.7-3), [`RFC5880-6.8.7-4`](#rfc5880-6.8.7-4), [`RFC5880-6.8.7-5`](#rfc5880-6.8.7-5), [`RFC5880-6.8.4-1`](#rfc5880-6.8.4-1), [`RFC5880-6.8.5-1`](#rfc5880-6.8.5-1), [`RFC5880-6.8.16-4`](#rfc5880-6.8.16-4), [`RFC5880-6.8.16-5`](#rfc5880-6.8.16-5), [`RFC5880-6.8.8-1`](#rfc5880-6.8.8-1), [`RFC5880-6.8.8-2`](#rfc5880-6.8.8-2), [`RFC5880-6.8.9-1`](#rfc5880-6.8.9-1), [`RFC5880-6.8.9-2`](#rfc5880-6.8.9-2), [`RFC5880-6.8.9-3`](#rfc5880-6.8.9-3), [`RFC5880-9-2`](#rfc5880-9-2), [`RFC5880-6.7.3-7`](#rfc5880-6.7.3-7), [`RFC5880-6.7.2-3`](#rfc5880-6.7.2-3), [`RFC5880-6.7.3-8`](#rfc5880-6.7.3-8), [`RFC5880-6.7.2-4`](#rfc5880-6.7.2-4), [`RFC5880-6.7.2-5`](#rfc5880-6.7.2-5), [`RFC5880-6.7.2-6`](#rfc5880-6.7.2-6), [`RFC5880-6.7.3-16`](#rfc5880-6.7.3-16), [`RFC5880-6.7.3-17`](#rfc5880-6.7.3-17), [`RFC5880-6.7.3-18`](#rfc5880-6.7.3-18), [`RFC5880-6.7.4-14`](#rfc5880-6.7.4-14), [`RFC5880-6.7.4-15`](#rfc5880-6.7.4-15), [`RFC5880-6.7.4-16`](#rfc5880-6.7.4-16), [`RFC5880-6.7.2-7`](#rfc5880-6.7.2-7), [`RFC5880-6.7.3-9`](#rfc5880-6.7.3-9), [`RFC5880-6.7.3-10`](#rfc5880-6.7.3-10), [`RFC5880-6.7.3-11`](#rfc5880-6.7.3-11), [`RFC5880-6.7.4-12`](#rfc5880-6.7.4-12), [`RFC5880-6.7.4-13`](#rfc5880-6.7.4-13), [`RFC5880-6.7.4-17`](#rfc5880-6.7.4-17), [`RFC5880-6.7.3-12`](#rfc5880-6.7.3-12)

**Annotated (including scoped evidence) (22):** [`RFC5880-6.8.1-1`](#rfc5880-6.8.1-1), [`RFC5880-6.8.1-2`](#rfc5880-6.8.1-2), [`RFC5880-6.8.1-11`](#rfc5880-6.8.1-11), [`RFC5880-6.8.1-13`](#rfc5880-6.8.1-13), [`RFC5880-6.1-3`](#rfc5880-6.1-3), [`RFC5880-6.8.3-4`](#rfc5880-6.8.3-4), [`RFC5880-6.6-2`](#rfc5880-6.6-2), [`RFC5880-6.6-3`](#rfc5880-6.6-3), [`RFC5880-6.7.3-3`](#rfc5880-6.7.3-3), [`RFC5880-6.7.4-3`](#rfc5880-6.7.4-3), [`RFC5880-6.8.6-14`](#rfc5880-6.8.6-14), [`RFC5880-6.8.6-15`](#rfc5880-6.8.6-15), [`RFC5880-6.8.7-6`](#rfc5880-6.8.7-6), [`RFC5880-6.8.7-7`](#rfc5880-6.8.7-7), [`RFC5880-6.8.7-8`](#rfc5880-6.8.7-8), [`RFC5880-6.8.15-1`](#rfc5880-6.8.15-1), [`RFC5880-7-1`](#rfc5880-7-1), [`RFC5880-7-2`](#rfc5880-7-2), [`RFC5880-4.3-1`](#rfc5880-4.3-1), [`RFC5880-6.8.14-1`](#rfc5880-6.8.14-1), [`RFC5880-6.8.17-1`](#rfc5880-6.8.17-1), [`RFC5880-6.8.3-8`](#rfc5880-6.8.3-8)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC5880-4.1-1` | The contents of transmitted BFD Control packets MUST be set as follows: Version Set to the current version number (1). (§6.8.7) | MUST | 6.8.7 - Transmitting BFD Control Packets | **positive:** `unit/verify` [`TestRFC5880TransmittedControlCarriesVersionOne`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_transmit_header_test.go#L38). **positive:** `unit/verify` [`TestRFC5880VersionOneAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/packet/rfc5880_test.go#L44). **negative:** `unit/verify` [`TestRFC5880TransmittedVersionNeverOverwrittenByDiag`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_transmit_header_test.go#L52). **negative:** `unit/verify` [`TestRFC5880VersionNotOneDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/packet/rfc5880_test.go#L64) |
| `RFC5880-4.1-2` | It MUST be zero on both transmit and receipt. (§4.1) | MUST | 4.1 - Generic BFD Control Packet Format | **positive:** `unit/verify` [`TestRFC5880MultipointZeroAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/packet/rfc5880_test.go#L80). **positive:** `unit/verify` [`TestRFC5880TransmittedControlMultipointZero`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_transmit_header_test.go#L73). **negative:** `unit/verify` [`TestRFC5880MultipointSetDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/packet/rfc5880_test.go#L96) |
| `RFC5880-6.3-1` | Each system MUST choose an opaque discriminator value that identifies each session, and which MUST be unique among all BFD sessions on the system. (§6.3) | MUST | 6.3 - Demultiplexing and the Discriminator Fields | **positive:** `unit/verify` [`TestRFC5880DiscriminatorsAreNonZeroAndUnique`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L62). **negative:** `unit/verify` [`TestRFC5880DiscriminatorAllocatorSkipsReservedAndTaken`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L90) |
| `RFC5880-6.8.1-1` | This variable MUST be initialized to Down. (§6.8.1) | MUST | 6.8.1 - State Variables | **positive:** `unit/verify` [`TestRFC5880InitStatesAreDown`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L73). **negative:** no negative test. **{single-polarity}:** Init assigns bfd.SessionState = Down unconditionally (internal/component/bfd/session/session.go:246) before any packet can be exchanged, so no non-conformant input exists to reject |
| `RFC5880-6.8.1-2` | This variable MUST be initialized to Down. (§6.8.1) | MUST | 6.8.1 - State Variables | **positive:** `unit/verify` [`TestRFC5880InitStatesAreDown`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L76). **negative:** no negative test. **{single-polarity}:** Init assigns bfd.RemoteSessionState = Down unconditionally (internal/component/bfd/session/session.go:247), so there is no non-conformant input to reject |
| `RFC5880-6.8.1-3` | It MUST be unique across all BFD sessions on this system, and nonzero. (§6.8.1) | MUST | 6.8.1 - State Variables | **positive:** `unit/verify` [`TestRFC5880DiscriminatorsAreNonZeroAndUnique`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L67). **negative:** `unit/verify` [`TestRFC5880DiscriminatorAllocatorSkipsReservedAndTaken`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L95) |
| `RFC5880-6.8.1-4` | This MUST be initialized to zero. (§6.8.1) | MUST | 6.8.1 - State Variables | **positive:** `unit/verify` [`TestRFC5880InitVariableDefaults`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L88). **negative:** `unit/verify` [`TestRFC5880InitClearsLearnedRemoteDiscr`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_session_clauses_test.go#L375) |
| `RFC5880-6.8.1-5` | If a period of a Detection Time passes without the receipt of a valid, authenticated BFD packet from the remote system, this variable MUST be set to zero. (§6.8.1) | MUST | 6.8.1 - State Variables | **positive:** `unit/verify` [`TestRFC5880RemoteDiscrClearedAfterSilenceWhileDown`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L1258). **positive:** `unit/verify` [`TestRFC5880RemoteDiscrClearedDespiteInvalidPackets`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_session_clauses_test.go#L59). **positive:** `unit/verify` [`TestRFC5880RemoteDiscrClearedOnDetectionExpiry`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L153). **negative:** `unit/verify` [`TestRFC5880RemoteDiscrKeptOnNeighborSignaledDown`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L180) |
| `RFC5880-6.8.1-6` | This MUST be initialized to zero (No Diagnostic). (§6.8.1) | MUST | 6.8.1 - State Variables | **positive:** `unit/verify` [`TestRFC5880InitVariableDefaults`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L90). **negative:** `unit/verify` [`TestRFC5880InitVariablesAreNotConstants`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L116) |
| `RFC5880-6.8.1-7` | This MUST be initialized to a value of at least one second (1,000,000 microseconds) according to the rules described in section 6.8.3. (§6.8.1, §6.8.3) | MUST | 6.8.1 - State Variables | **positive:** `unit/verify` [`TestRFC5880SlowStartFloorWhileNotUp`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L208). **negative:** `unit/verify` [`TestRFC5880SlowStartFloorLiftedWhenUp`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L228) |
| `RFC5880-6.8.1-8` | The last value of Required Min RX Interval received from the remote system in a BFD Control packet. This variable MUST be initialized to 1. (§6.8.1) | MUST | 6.8.1 - State Variables | **positive:** `unit/verify` [`TestRFC5880InitVariableDefaults`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L92). **negative:** `unit/verify` [`TestRFC5880InitVariablesAreNotConstants`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L118) |
| `RFC5880-6.8.1-9` | This variable MUST be initialized to zero. (§6.8.1) | MUST | 6.8.1 - State Variables | **positive:** `unit/verify` [`TestRFC5880InitVariableDefaults`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L94). **negative:** `unit/verify` [`TestRFC5880InitVariablesAreNotConstants`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L121) |
| `RFC5880-6.8.1-10` | This variable MUST be a nonzero integer, and is otherwise outside the scope of this specification. (§6.8.1) | MUST | 6.8.1 - State Variables | **positive:** `unit/verify` [`TestRFC5880DetectMultConfiguredValue`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L248). **negative:** `unit/verify` [`TestRFC5880DetectMultZeroRequestSubstituted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L259) |
| `RFC5880-6.8.1-11` | This variable MUST be initialized to a random 32-bit value. (§6.8.1) | MUST | 6.8.1 - State Variables | **positive:** no positive test. **negative:** no negative test. **{gap}:** SetAuth seeds bfd.XmitAuthSeq from the persister or leaves the Vars zero value (internal/component/bfd/session/auth.go:36) and Init never randomizes it (internal/component/bfd/session/session.go:245-259), so the initial transmit sequence is 0 rather than a random 32-bit value |
| `RFC5880-6.8.1-12` | This variable MUST be initialized to zero. (§6.8.1) | MUST | 6.8.1 - State Variables | **positive:** `unit/verify` [`TestRFC5880AuthSeqKnownStartsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L427). **positive:** `unit/verify` [`TestRFC5880NewSessionAuthSeqKnownZero`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_auth_seq_test.go#L75). **negative:** `unit/verify` [`TestRFC5880AuthSeqKnownOneDiscardsFirstPacket`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_auth_seq_test.go#L100). **negative:** `unit/verify` [`TestRFC5880AuthSeqKnownSetAfterFirstPacket`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L445) |
| `RFC5880-6.8.1-13` | This variable MUST be set to zero after no packets have been received on this session for at least twice the Detection Time. (§6.8.1) | MUST | 6.8.1 - State Variables | **positive:** no positive test. **negative:** no negative test. **{gap}:** bfd.AuthSeqKnown is SeqState.initialized, which only Advance sets (internal/component/bfd/auth/meticulous.go:63-66) and nothing ever clears; CheckDetection clears the detection deadline alone (internal/component/bfd/session/timers.go:82), so the flag survives twice the Detection Time of silence |
| `RFC5880-6.8.1-14` | Once session state is created, and at least one BFD Control packet is received from the remote end, it MUST be preserved for at least one Detection Time (see section 6.8.4) subsequent to the receipt of the last BFD Control packet, regardless of the session state. (§6.8.1) | MUST | 6.8.1 - State Variables | **positive:** `unit/verify` [`TestRFC5880ReleasedSessionPreservedForOneDetectionTime`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_release_test.go#L30). **positive:** `unit/verify` [`TestRFC5880StatePreservedForOneDetectionTime`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L283). **negative:** `unit/verify` [`TestRFC5880ReleasedSessionAskedAgainKeepsRemoteState`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_release_test.go#L111) |
| `RFC5880-6.1-1` | A system taking the Active role MUST send BFD Control packets for a particular session, regardless of whether it has received any BFD packets for that session. (§6.1) | MUST | 6.1 - Overview | **positive:** `unit/verify` [`TestRFC5880ActiveRoleTransmitsWithoutReception`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L358). **positive:** `unit/verify` [`TestRFC5880ActiveSessionSendsBeforeAnyReception`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_role_transmit_test.go#L52). **negative:** `unit/verify` [`TestRFC5880ActiveSessionSendsAfterThePeerFallsSilent`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_role_transmit_test.go#L63). **negative:** `unit/verify` [`TestRFC5880PassiveRoleSilentUntilReception`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L391). **negative:** `unit/verify` [`TestRFC5880PassiveRoleTransmitsAfterReception`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L409) |
| `RFC5880-6.1-2` | A system taking the Passive role MUST NOT begin sending BFD packets for a particular session until it has received a BFD packet for that session, and thus has learned the remote system's discriminator value. (§6.1) | MUST NOT | 6.1 - Overview | **positive:** `unit/verify` [`TestRFC5880PassiveRoleSilentUntilReception`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L383). **positive:** `unit/verify` [`TestRFC5880PassiveSessionSilentThroughTicksUntilReception`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_role_transmit_test.go#L77). **negative:** `unit/verify` [`TestRFC5880PassiveRoleTransmitsAfterReception`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L414). **negative:** `unit/verify` [`TestRFC5880PassiveSessionSendsOnceThePeerSpoke`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_role_transmit_test.go#L88) |
| `RFC5880-6.1-3` | At least one system MUST take the Active role (possibly both). (§6.1) | MUST | 6.1 - Overview | **positive:** `unit/verify` [`TestRFC5880ActiveRoleTransmitsWithoutReception`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L363). **negative:** no negative test. **{single-polarity}:** the obligation binds the pair of systems and Ze cannot see the peer role, so no refusal path exists; the HEAD negative proved RFC5880-6.1-1 and stays there, owner ruling 2026-09-30 |
| `RFC5880-6.8.3-1` | When bfd.SessionState is not Up, the system MUST set bfd.DesiredMinTxInterval to a value of not less than one second (1,000,000 microseconds). (§6.8.3) | MUST | 6.8.3 - Timer Manipulation | **positive:** `unit/verify` [`TestRFC5880SlowStartFloorInEveryStateButUp`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_session_clauses_test.go#L103). **positive:** `unit/verify` [`TestRFC5880SlowStartFloorRestoredOnAdminDown`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L1296). **positive:** `unit/verify` [`TestRFC5880SlowStartFloorWhileNotUp`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L213). **negative:** `unit/verify` [`TestRFC5880SlowStartFloorLiftedWhenUp`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L232) |
| `RFC5880-6.8.3-2` | If either bfd.DesiredMinTxInterval is changed or bfd.RequiredMinRxInterval is changed, a Poll Sequence MUST be initiated (see section 6.5). (§6.8.3) | MUST | 6.8.3 - Timer Manipulation | **positive:** `unit/verify` [`TestRFC5880PollInitiatedForEachIntervalAlone`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_session_clauses_test.go#L158). **positive:** `unit/verify` [`TestRFC5880PollInitiatedOnIntervalChange`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L435). **positive:** `unit/verify` [`TestRFC5880PollInitiatedWhenDownEntryRaisesDesiredMinTx`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_session_clauses_test.go#L355). **negative:** `unit/verify` [`TestRFC5880NoPollWhenIntervalsUnchanged`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L469) |
| `RFC5880-6.8.3-3` | If bfd.DesiredMinTxInterval is increased and bfd.SessionState is Up, the actual transmission interval used MUST NOT change until the Poll Sequence described above has terminated. (§6.8.3) | MUST NOT | 6.8.3 - Timer Manipulation | **positive:** `unit/verify` [`TestRFC5880RaisedDesiredMinTxHeldUntilPollTerminates`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L1501). **negative:** `unit/verify` [`TestRFC5880RaisedDesiredMinTxAppliedAtFinal`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L1560) |
| `RFC5880-6.8.3-4` | If bfd.RequiredMinRxInterval is reduced and bfd.SessionState is Up, the previous value of bfd.RequiredMinRxInterval MUST be used when calculating the Detection Time for the remote system until the Poll Sequence described above has terminated. (§6.8.3) | MUST | 6.8.3 - Timer Manipulation | **positive:** no positive test. **negative:** no negative test. **{gap}:** revertEchoSlowdownLocked reduces bfd.RequiredMinRxInterval while Up (internal/component/bfd/session/timers.go:249) and DetectionInterval immediately uses the reduced value (internal/component/bfd/session/timers.go:29); no previous value is retained until the Poll Sequence terminates |
| `RFC5880-6.8.3-5` | If the local system reduces its transmit interval due to bfd.RemoteMinRxInterval being reduced (the remote system has advertised a reduced value in Required Min RX Interval), and the remote system is not in Demand mode, the local system MUST honor the new interval immediately. (§6.8.3) | MUST | 6.8.3 - Timer Manipulation | **positive:** `unit/verify` [`TestRFC5880RemoteMinRxChangeReschedulesNextTx`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L1327). **positive:** `unit/verify` [`TestRFC5880RemoteMinRxReductionHonoredImmediately`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L496). **negative:** `unit/verify` [`TestRFC5880TransmitIntervalFlooredByLocalDesired`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L532). **negative:** `unit/verify` [`TestRFC5880UnchangedRemoteMinRxKeepsNextTx`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L1415) |
| `RFC5880-6.8.3-6` | Therefore, if multiple changes are made that require the use of a Poll Sequence, there are three choices: 1) they MUST be communicated in a single BFD Control packet (so the semantics of the Final reply are clear), or 2) sufficient time must have transpired since the Poll Sequence was completed to disambiguate the situation (at least a round trip time since the last Poll was transmitted) prior to the initiation of another Poll Sequence, or 3) an additional BFD Control packet with the Final (F) bit *clear* MUST be received after the Poll Sequence has completed prior to the initiation of another Poll Sequence (this option is not available when Demand mode is active). (§6.8.3) | MUST | 6.8.3 - Timer Manipulation | **positive:** `unit/verify` [`TestRFC5880EchoSlowdownPollsAfterSettledPoll`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L759). **positive:** `unit/verify` [`TestRFC5880PollInitiatedOnIntervalChange`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L440). **negative:** `unit/verify` [`TestRFC5880EchoSlowdownRevertWaitsForOutstandingPoll`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L788). **negative:** `unit/verify` [`TestRFC5880EchoSlowdownWaitsForOutstandingPoll`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L722) |
| `RFC5880-6.5-1` | A BFD Control packet MUST NOT have both the Poll (P) and Final (F) bits set. (§6.5) | MUST NOT | 6.5 - The Poll Sequence | **positive:** `unit/verify` [`TestRFC5880PollPacketHasNoFinalBit`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L566). **negative:** `unit/verify` [`TestRFC5880FinalReplyClearsPollBit`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L585) |
| `RFC5880-6.5-2` | If periodic BFD Control packets are already being sent (the remote system is not in Demand mode), the Poll Sequence MUST be performed by setting the Poll (P) bit on those scheduled periodic transmissions; additional packets MUST NOT be sent. (§6.5) | MUST | 6.5 - The Poll Sequence | **positive:** `unit/verify` [`TestRFC5880PollPacketHasNoFinalBit`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L570). **positive:** `unit/verify` [`TestRFC5880PollRidesTheScheduledPacket`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_session_clauses_test.go#L326). **negative:** `unit/verify` [`TestRFC5880FinalReplyClearsPollBit`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L589) |
| `RFC5880-6.6-1` | A system MUST NOT set the Demand (D) bit unless bfd.DemandMode is 1, bfd.SessionState is Up, and bfd.RemoteSessionState is Up. (§6.8.7) | MUST NOT | 6.8.7 - Transmitting BFD Control Packets | **positive:** `unit/verify` [`TestRFC5880DemandBitSetWhenAllConditionsHold`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L651). **negative:** `unit/verify` [`TestRFC5880DemandBitClearWhenAnyConditionFails`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L665) |
| `RFC5880-6.6-2` | When the transmitted value of the Demand (D) bit is to be changed, the transmitting system MUST initiate a Poll Sequence in conjunction with changing the bit in order to ensure that both systems are aware of the change. (§6.6) | MUST | 6.6 - Demand mode | **positive:** no positive test. **negative:** no negative test. **{gap}:** bfd.DemandMode has no writer in production code -- it is declared at internal/component/bfd/session/session.go:58 and read only by canSetDemand (internal/component/bfd/session/fsm.go:247) -- so no D-bit change path exists and none initiates a Poll Sequence |
| `RFC5880-6.6-3` | If Demand mode is active on either or both systems, a Poll Sequence MUST be initiated whenever the contents of the next BFD Control packet to be sent would be different than the contents of the previous packet, with the exception of the Poll (P) and Final (F) bits. (§6.6) | MUST | 6.6 - Demand mode | **positive:** no positive test. **negative:** no negative test. **{gap}:** bfd.RemoteDemandMode is stored by Receive (internal/component/bfd/session/fsm.go:58) and read nowhere, and bfd.DemandMode has no writer (internal/component/bfd/session/session.go:58), so no code path starts a Poll when packet contents would change while Demand mode is active |
| `RFC5880-6.7-1` | Implementations supporting authentication MUST support both types of SHA1 authentication. (§6.7) | MUST | 6.7 - Authentication | **positive:** `unit/verify` [`TestRFC5880BothSHA1VariantsSupported`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L51). **negative:** `unit/verify` [`TestRFC5880SHA1TypesEnforcedDistinctly`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_keyed_test.go#L140). **negative:** `unit/verify` [`TestRFC5880UnsupportedAuthTypesRejected`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L70) |
| `RFC5880-4.2-1` | The password is a binary string, and MUST be from 1 to 16 bytes in length. (§4.2, §6.7.2) | MUST | 4.2 - Simple Password Authentication Section Format | **positive:** `unit/verify` [`TestRFC5880SimplePasswordManagementAcceptsASCIIStrings`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5880_test.go#L99). **positive:** `unit/verify` [`TestRFC5880SimplePasswordSectionHeader`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L636). **negative:** `unit/verify` [`auth/TestRFC5880SimplePasswordLengthOutOfRangeRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L791). **negative:** `unit/verify` [`bfd/TestRFC5880SimplePasswordLengthOutOfRangeRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5880_test.go#L126) |
| `RFC5880-6.7.2-1` | For interoperability, the management interface by which the password is configured MUST accept ASCII strings (§6.7.2) | MUST | 6.7.2 - Simple Password Authentication | **positive:** `unit/verify` [`TestRFC5880KeyManagementConfigEntryAcceptsASCIIStrings`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5880_key_management_config_test.go#L88). **positive:** `unit/verify` [`TestRFC5880SimplePasswordManagementAcceptsASCIIStrings`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5880_test.go#L94). **negative:** `unit/verify` [`TestRFC5880KeyManagementConfigEntryKeepsEveryASCIICharacter`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5880_key_management_config_test.go#L110) |
| `RFC5880-6.7.2-8` | The Auth Type field MUST be set to 1 (Simple Password). The Auth Len field MUST be set to the proper length (4 to 19 bytes). (§6.7.2) | MUST | 6.7.2 - Simple Password Authentication | **positive:** `unit/verify` [`TestRFC5880SimplePasswordSectionHeader`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L631). **negative:** `unit/verify` [`TestRFC5880SimplePasswordRejectsForeignSectionShape`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L657). **negative:** `unit/verify` [`TestRFC5880SimplePasswordSectionOutsideProperLengthNotBuilt`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_keyed_test.go#L281) |
| `RFC5880-6.7.3-1` | The Auth Type field MUST be set to 2 (Keyed MD5) or 3 (Meticulous Keyed MD5). The Auth Len field MUST be set to 24. (§6.7.3) | MUST | 6.7.3 - Keyed MD5 and Meticulous Keyed MD5 Authentication | **positive:** `unit/verify` [`TestRFC5880KeyedMD5SectionHeader`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L86). **negative:** `unit/verify` [`TestRFC5880KeyedMD5OtherAlgorithmFieldsDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_section_fields_test.go#L96). **negative:** `unit/verify` [`TestRFC5880KeyedMD5OversizedKeyNotSigned`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_keyed_test.go#L295). **negative:** `unit/verify` [`TestRFC5880KeyedMD5RejectsForeignSectionShape`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L113) |
| `RFC5880-6.7.3-2` | An MD5 digest MUST be calculated over the entire BFD Control packet. (§6.7.3) | MUST | 6.7.3 - Keyed MD5 and Meticulous Keyed MD5 Authentication | **positive:** `unit/verify` [`TestRFC5880DigestCoversWholePacket`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L182). **positive:** `unit/verify` [`TestRFC5880DigestSpansEntirePacket`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_keyed_test.go#L511). **negative:** `unit/verify` [`TestRFC5880DigestOverPartialPacketDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_keyed_test.go#L541). **negative:** `unit/verify` [`TestRFC5880DigestRejectsMandatorySectionTamper`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L201) |
| `RFC5880-6.7.3-3` | replacing the secret key, which MUST NOT be carried in the packet (§6.7.3) | MUST NOT | 6.7.3 - Keyed MD5 and Meticulous Keyed MD5 Authentication | **positive:** `unit/verify` [`TestRFC5880SecretKeyNotCarriedInPacket`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L225). **negative:** no negative test. **{single-polarity}:** Sign overwrites the key scratch with the computed digest before the packet is handed to the transport (internal/component/bfd/auth/sha1.go:85-87), so no key-bearing packet is ever emitted for a receiver to reject |
| `RFC5880-6.7.3-4` | For Meticulous Keyed MD5, bfd.XmitAuthSeq MUST be incremented in a circular fashion (when treated as an unsigned 32-bit value). (§6.7.3) | MUST | 6.7.3 - Keyed MD5 and Meticulous Keyed MD5 Authentication | **positive:** `unit/verify` [`TestRFC5880MeticulousSequenceIncrementsPerPacket`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L287). **positive:** `unit/verify` [`TestRFC5880MeticulousXmitAuthSeqWrapsAt32Bits`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_auth_seq_test.go#L48). **negative:** `unit/verify` [`TestRFC5880MeticulousNonCircularRepeatAtWrapDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_auth_wrap_test.go#L28). **negative:** `unit/verify` [`TestRFC5880MeticulousRejectsUnincrementedSequence`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L583) |
| `RFC5880-6.7.3-13` | The authentication key value is a binary string of up to 16 bytes, and MUST be placed into the Auth Key/Digest field, padded with trailing zero bytes as necessary. (§6.7.3) | MUST | 6.7.3 - Keyed MD5 and Meticulous Keyed MD5 Authentication | **positive:** `unit/verify` [`TestKeyedSecretLongerThanSlotRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L932). **positive:** `unit/verify` [`TestParseAuthConfigKeyedSecretLength`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5881_test.go#L136). **positive:** `unit/verify` [`TestRFC5880ShortKeyZeroPaddedIntoDigestField`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L978). **negative:** `unit/verify` [`TestKeyedSecretLongerThanSlotRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L935). **negative:** `unit/verify` [`TestParseAuthConfigKeyedSecretLength`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5881_test.go#L139) |
| `RFC5880-6.7.4-1` | The Auth Type field MUST be set to 4 (Keyed SHA1) or 5 (Meticulous Keyed SHA1).  The Auth Len field MUST be set to 28. (§6.7.4) | MUST | 6.7.4 - Keyed SHA1 and Meticulous Keyed SHA1 Authentication | **positive:** `unit/verify` [`TestRFC5880KeyedSHA1SectionHeader`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L138). **negative:** `unit/verify` [`TestRFC5880KeyedSHA1OtherAlgorithmFieldsDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_section_fields_test.go#L107). **negative:** `unit/verify` [`TestRFC5880KeyedSHA1OversizedKeyNotSigned`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_keyed_test.go#L310). **negative:** `unit/verify` [`TestRFC5880KeyedSHA1RejectsForeignSectionShape`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L159) |
| `RFC5880-6.7.4-2` | A SHA1 hash MUST be calculated over the entire BFD control packet. (§6.7.4) | MUST | 6.7.4 - Keyed SHA1 and Meticulous Keyed SHA1 Authentication | **positive:** `unit/verify` [`TestRFC5880DigestCoversWholePacket`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L188). **positive:** `unit/verify` [`TestRFC5880DigestSpansEntirePacket`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_keyed_test.go#L515). **negative:** `unit/verify` [`TestRFC5880DigestOverPartialPacketDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_keyed_test.go#L547). **negative:** `unit/verify` [`TestRFC5880DigestRejectsMandatorySectionTamper`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L206) |
| `RFC5880-6.7.4-3` | the secret key, which MUST NOT be carried in the packet (§6.7.4) | MUST NOT | 6.7.4 - Keyed SHA1 and Meticulous Keyed SHA1 Authentication | **positive:** `unit/verify` [`TestRFC5880SecretKeyNotCarriedInPacket`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L229). **negative:** no negative test. **{single-polarity}:** the SHA1 signer shares that producer with a 20-byte digest slot (internal/component/bfd/auth/sha1.go:85-87,193-195), so no key-bearing packet is emitted |
| `RFC5880-6.7.4-4` | For Meticulous Keyed SHA1, bfd.XmitAuthSeq MUST be incremented in a circular fashion (when treated as an unsigned 32-bit value). (§6.7.4) | MUST | 6.7.4 - Keyed SHA1 and Meticulous Keyed SHA1 Authentication | **positive:** `unit/verify` [`TestRFC5880MeticulousSequenceIncrementsPerPacket`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L293). **positive:** `unit/verify` [`TestRFC5880MeticulousXmitAuthSeqWrapsAt32Bits`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_auth_seq_test.go#L52). **negative:** `unit/verify` [`TestRFC5880MeticulousNonCircularRepeatAtWrapDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_auth_wrap_test.go#L33). **negative:** `unit/verify` [`TestRFC5880MeticulousRejectsUnincrementedSequence`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L588) |
| `RFC5880-6.7.4-5` | the management interface by which the key is configured MUST accept ASCII strings (§6.7.4) | MUST | 6.7.4 - Keyed SHA1 and Meticulous Keyed SHA1 Authentication | **positive:** `unit/verify` [`TestRFC5880KeyManagementAcceptsASCIIStrings`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5880_test.go#L31). **positive:** `unit/verify` [`TestRFC5880KeyManagementConfigEntryAcceptsASCIIStrings`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5880_key_management_config_test.go#L95). **negative:** `unit/verify` [`TestRFC5880KeyManagementConfigEntryKeepsEveryASCIICharacter`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5880_key_management_config_test.go#L119) |
| `RFC5880-6.7.4-7` | The authentication key value is a binary string of up to 20 bytes, and MUST be placed into the Auth Key/Hash field, padding with trailing zero bytes as necessary. (§6.7.4) | MUST | 6.7.4 - Keyed SHA1 and Meticulous Keyed SHA1 Authentication | **positive:** `unit/verify` [`TestKeyedSecretLongerThanSlotRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L938). **positive:** `unit/verify` [`TestParseAuthConfigKeyedSecretLength`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5881_test.go#L141). **positive:** `unit/verify` [`TestRFC5880ShortKeyZeroPaddedIntoDigestField`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L984). **negative:** `unit/verify` [`TestKeyedSecretLongerThanSlotRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L941). **negative:** `unit/verify` [`TestParseAuthConfigKeyedSecretLength`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5881_test.go#L143) |
| `RFC5880-6.7.3-15` | The Auth Key ID field MUST be set to the ID of the current authentication key. (§6.7.3) | MUST | 6.7.3 - Keyed MD5 and Meticulous Keyed MD5 Authentication | **positive:** `unit/verify` [`TestRFC5880MD5AuthKeyIDIsTheCurrentKey`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_md5_split_test.go#L28). **negative:** `unit/verify` [`TestRFC5880MD5AuthKeyIDOverwritesStaleBuffer`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_md5_split_test.go#L48) |
| `RFC5880-6.7.4-10` | The Auth Key ID field MUST be set to the ID of the current authentication key. (§6.7.4) | MUST | 6.7.4 - Keyed SHA1 and Meticulous Keyed SHA1 Authentication | **positive:** `unit/verify` [`TestRFC5880AuthKeyIDIsTheConfiguredKey`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L249). **negative:** `unit/verify` [`TestRFC5880AuthKeyIDMismatchDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L267) |
| `RFC5880-6.7.4-11` | The Sequence Number field MUST be set to bfd.XmitAuthSeq. (§6.7.4) | MUST | 6.7.4 - Keyed SHA1 and Meticulous Keyed SHA1 Authentication | **positive:** `unit/verify` [`TestRFC5880AuthSequenceFieldIsXmitAuthSeq`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L1200). **negative:** `unit/verify` [`TestRFC5880AuthSequenceFieldFollowsAdvance`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L1224) |
| `RFC5880-6.8.6-1` | If the version number is not correct (1), the packet MUST be discarded. (§6.8.6) | MUST | 6.8.6 - Reception of BFD Control Packets | **positive:** `unit/verify` [`TestRFC5880VersionOneAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/packet/rfc5880_test.go#L48). **negative:** `unit/verify` [`TestRFC5880VersionNotOneDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/packet/rfc5880_test.go#L66) |
| `RFC5880-6.8.6-2` | If the Length field is less than the minimum correct value (24 if the A bit is clear, or 26 if the A bit is set), the packet MUST be discarded. (§6.8.6) | MUST | 6.8.6 - Reception of BFD Control Packets | **positive:** `unit/verify` [`TestRFC5880LengthMinimumAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/packet/rfc5880_test.go#L108). **negative:** `unit/verify` [`TestRFC5880AuthenticatedLengthOneBelowMinimumDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/packet/rfc5880_length_boundary_test.go#L8). **negative:** `unit/verify` [`TestRFC5880LengthBelowMinimumDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/packet/rfc5880_test.go#L128) |
| `RFC5880-6.8.6-3` | If the Length field is greater than the payload of the encapsulating protocol, the packet MUST be discarded. (§6.8.6) | MUST | 6.8.6 - Reception of BFD Control Packets | **positive:** `unit/verify` [`TestRFC5880LengthEqualsPayloadAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/packet/rfc5880_test.go#L148). **negative:** `unit/verify` [`TestRFC5880LengthOverPayloadDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/packet/rfc5880_test.go#L163) |
| `RFC5880-6.8.6-4` | If the Detect Mult field is zero, the packet MUST be discarded. (§6.8.6) | MUST | 6.8.6 - Reception of BFD Control Packets | **positive:** `unit/verify` [`TestRFC5880DetectMultNonZeroAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/packet/rfc5880_test.go#L174). **negative:** `unit/verify` [`TestRFC5880DetectMultZeroDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/packet/rfc5880_test.go#L190) |
| `RFC5880-6.8.6-5` | If the Multipoint (M) bit is nonzero, the packet MUST be discarded. (§6.8.6) | MUST | 6.8.6 - Reception of BFD Control Packets | **positive:** `unit/verify` [`TestRFC5880MultipointZeroAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/packet/rfc5880_test.go#L84). **negative:** `unit/verify` [`TestRFC5880MultipointSetDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/packet/rfc5880_test.go#L99) |
| `RFC5880-6.8.6-6` | If the My Discriminator field is zero, the packet MUST be discarded. (§6.8.6) | MUST | 6.8.6 - Reception of BFD Control Packets | **positive:** `unit/verify` [`TestRFC5880MyDiscriminatorNonZeroAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/packet/rfc5880_test.go#L200). **negative:** `unit/verify` [`TestRFC5880MyDiscriminatorZeroDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/packet/rfc5880_test.go#L214) |
| `RFC5880-6.8.6-7` | If the Your Discriminator field is nonzero, it MUST be used to select the session with which this BFD packet is associated.  If no session is found, the packet MUST be discarded. (§6.8.6) | MUST | 6.8.6 - Reception of BFD Control Packets | **positive:** `unit/verify` [`TestRFC5880YourDiscriminatorSelectsAmongSessions`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_discriminator_select_test.go#L9). **positive:** `unit/verify` [`TestRFC5880YourDiscriminatorSelectsSession`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L124). **negative:** `unit/verify` [`TestRFC5880UnknownYourDiscriminatorDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L140) |
| `RFC5880-6.8.6-8` | If the Your Discriminator field is zero and the State field is not Down or AdminDown, the packet MUST be discarded. (§6.8.6) | MUST | 6.8.6 - Reception of BFD Control Packets | **positive:** `unit/verify` [`TestRFC5880ZeroYourDiscriminatorAcceptedWhenDown`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L726). **negative:** `unit/verify` [`TestRFC5880ZeroYourDiscriminatorDiscardedWhenLive`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L698) |
| `RFC5880-6.8.6-9` | If the A bit is set and no authentication is in use (bfd.AuthType is zero), the packet MUST be discarded. (§6.8.6) | MUST | 6.8.6 - Reception of BFD Control Packets | **positive:** `unit/verify` [`TestRFC5880UnauthenticatedSessionAcceptsUnauthenticatedPacket`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L764). **negative:** `unit/verify` [`TestRFC5880UnauthenticatedSessionDiscardsAuthenticatedPacket`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L779) |
| `RFC5880-6.8.6-10` | If the A bit is clear and authentication is in use (bfd.AuthType is nonzero), the packet MUST be discarded. (§6.8.6) | MUST | 6.8.6 - Reception of BFD Control Packets | **positive:** `unit/verify` [`TestRFC5880AuthenticatedSessionAcceptsAuthenticatedPacket`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L796). **negative:** `unit/verify` [`TestRFC5880AuthenticatedSessionDiscardsUnauthenticatedPacket`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L814) |
| `RFC5880-6.8.6-11` | If the A bit is set, the packet MUST be authenticated under the rules of section 6.7, based on the authentication type in use (bfd.AuthType). (§6.8.6) | MUST | 6.8.6 - Reception of BFD Control Packets | **positive:** `unit/verify` [`TestRFC5880AuthenticatedPacketVerifiedAndDelivered`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L241). **positive:** `unit/verify` [`TestRFC5880AuthenticatedUnderEachSessionAuthType`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_auth_type_test.go#L45). **negative:** `unit/verify` [`TestRFC5880AuthenticatedPacketOfAnotherAuthTypeDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_auth_type_test.go#L67). **negative:** `unit/verify` [`TestRFC5880UnauthenticPacketDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L258) |
| `RFC5880-6.8.6-12` | If the Required Min Echo RX Interval field is zero, the transmission of Echo packets, if any, MUST cease. (§6.8.6) | MUST | 6.8.6 - Reception of BFD Control Packets | **positive:** `unit/verify` [`TestRFC5880EchoCeasesWhenPeerAdvertisesZero`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L830). **negative:** `unit/verify` [`TestRFC5880EchoEnabledWhenPeerAdvertisesNonZero`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L867) |
| `RFC5880-6.8.6-13` | If a Poll Sequence is being transmitted by the local system and the Final (F) bit in the received packet is set, the Poll Sequence MUST be terminated. (§6.8.6) | MUST | 6.8.6 - Reception of BFD Control Packets | **positive:** `unit/verify` [`TestRFC5880FinalTerminatesPoll`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L609). **negative:** `unit/verify` [`TestRFC5880NonFinalDoesNotTerminatePoll`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L629) |
| `RFC5880-6.8.6-14` | If bfd.RemoteDemandMode is 1, bfd.SessionState is Up, and bfd.RemoteSessionState is Up, Demand mode is active on the remote system and the local system MUST cease the periodic transmission of BFD Control packets (see section 6.8.7). (§6.8.6) | MUST | 6.8.6 - Reception of BFD Control Packets | **positive:** no positive test. **negative:** no negative test. **{gap}:** tick transmits whenever the periodic deadline has passed and never consults the remote Demand state (internal/component/bfd/engine/loop.go:192-201), so periodic Control packets continue after the peer sets D=1 |
| `RFC5880-6.8.6-15` | If bfd.RemoteDemandMode is 0, or bfd.SessionState is not Up, or bfd.RemoteSessionState is not Up, Demand mode is not active on the remote system and the local system MUST send periodic BFD Control packets (see section 6.8.7). (§6.8.6) | MUST | 6.8.6 - Reception of BFD Control Packets | **positive:** `unit/verify` [`TestRFC5880PeriodicTransmitDemandClearBothUp`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_demand_disjunct_test.go#L73). **positive:** `unit/verify` [`TestRFC5880PeriodicTransmitDemandSetLocalNotUpRemoteUp`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_demand_disjunct_test.go#L90). **positive:** `unit/verify` [`TestRFC5880PeriodicTransmitDemandSetLocalUpRemoteNotUp`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_demand_disjunct_test.go#L107). **positive:** `unit/verify` [`TestRFC5880PeriodicTransmitWhenRemoteDemandInactive`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L368). **positive:** `unit/verify` [`TestRFC5880PeriodicTransmitWhileDemandBitSetAndNotBothUp`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_demand_periodic_test.go#L9). **negative:** no negative test. **{single-polarity}:** no genuine negative exists, the held AdminDown negative proved the RFC5880-6.8.16-3 window and stays there, owner ruling 2026-09-30 |
| `RFC5880-6.8.6-16` | If a BFD Control packet is received with the Poll (P) bit set to 1, the receiving system MUST transmit a BFD Control packet with the Poll (P) bit clear and the Final (F) bit set as soon as practicable (§6.8.7) | MUST | 6.8.7 - Transmitting BFD Control Packets | **positive:** `unit/verify` [`TestRFC5880PollAnsweredWithImmediateFinal`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L322). **negative:** `unit/verify` [`TestRFC5880NonPollProducesNoImmediateReply`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L352) |
| `RFC5880-6.8.6-18` | If the Your Discriminator field is zero, the session MUST be selected based on some combination of other fields, possibly including source addressing information, the My Discriminator field, and the interface over which the packet was received. (§6.8.6) | MUST | 6.8.6 - Reception of BFD Control Packets | **positive:** `unit/verify` [`TestFirstPacketMatchesWhatTheTransportSurfaces`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5881_test.go#L363). **positive:** `unit/verify` [`TestRFC5881FirstPacketMatchesByTuple`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5881_test.go#L135). **negative:** `unit/verify` [`TestRFC5881FirstPacketWrongSourceDropped`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5881_test.go#L158) |
| `RFC5880-6.8.7-1` | With the exceptions listed in the remainder of this section, a system MUST NOT transmit BFD Control packets at an interval less than the larger of bfd.DesiredMinTxInterval and bfd.RemoteMinRxInterval, less applied jitter (see below). (§6.8.7) | MUST NOT | 6.8.7 - Transmitting BFD Control Packets | **positive:** `unit/verify` [`TestRFC5880TickSpacesControlPacketsByLargerInterval`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_engine_clauses_test.go#L37). **positive:** `unit/verify` [`TestRFC5880TransmitDeadlineUsesLargerOfBothIntervals`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_session_clauses_test.go#L272). **positive:** `unit/verify` [`TestRFC5880TransmitDeadlineUsesNegotiatedInterval`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L1033). **negative:** `unit/verify` [`TestRFC5880TransmitDeadlineClampsBadJitter`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L1063) |
| `RFC5880-6.8.7-2` | The periodic transmission of BFD Control packets MUST be jittered on a per-packet basis by up to 25%, that is, the interval MUST be reduced by a random value of 0 to 25% (§6.8.7) | MUST | 6.8.7 - Transmitting BFD Control Packets | **positive:** `unit/verify` [`TestRFC5880JitterIsAppliedPerPacket`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L455). **positive:** `unit/verify` [`TestRFC5880PeriodicTransmitJitteredPerPacket`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_jitter_tick_test.go#L64). **negative:** `unit/verify` [`TestRFC5880JitterStaysWithinBand`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L483) |
| `RFC5880-6.8.7-3` | If bfd.DetectMult is equal to 1, the interval between transmitted BFD Control packets MUST be no more than 90% of the negotiated transmission interval, and MUST be no less than 75% of the negotiated transmission interval. (§6.8.7) | MUST | 6.8.7 - Transmitting BFD Control Packets | **positive:** `unit/verify` [`TestRFC5880JitterDetectMultOneWindow`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L503). **positive:** `unit/verify` [`TestRFC5880PeriodicTransmitDetectMultOneWindow`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_jitter_tick_test.go#L100). **negative:** `unit/verify` [`TestRFC5880JitterFloorOnlyForDetectMultOne`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L521) |
| `RFC5880-6.8.7-4` | The transmit interval MUST be recalculated whenever bfd.DesiredMinTxInterval changes, or whenever bfd.RemoteMinRxInterval changes, and is equal to the greater of those two values. (§6.8.7) | MUST | 6.8.7 - Transmitting BFD Control Packets | **positive:** `unit/verify` [`TestRFC5880DesiredMinTxChangeReschedulesNextTx`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L1380). **positive:** `unit/verify` [`TestRFC5880RemoteMinRxChangeReschedulesNextTx`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L1333). **positive:** `unit/verify` [`TestRFC5880RemoteMinRxReductionHonoredImmediately`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L503). **negative:** `unit/verify` [`TestRFC5880TransmitIntervalFlooredByLocalDesired`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L537) |
| `RFC5880-6.8.7-5` | A system MUST NOT transmit BFD Control packets if bfd.RemoteDiscr is zero and the system is taking the Passive role. (§6.8.7) | MUST NOT | 6.8.7 - Transmitting BFD Control Packets | **positive:** `unit/verify` [`TestRFC5880PassiveRoleSilentUntilReception`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L388). **positive:** `unit/verify` [`TestRFC5880PassiveSilentOnceRemoteDiscrCleared`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L1439). **positive:** `unit/verify` [`TestRFC5880PassiveTickSendsNothingWithoutRemoteDiscr`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_engine_clauses_test.go#L82). **negative:** `unit/verify` [`TestRFC5880ActiveRoleTransmitsWithoutReception`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L366). **negative:** `unit/verify` [`TestRFC5880PassiveTransmitsAgainOnceRemoteDiscrLearned`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L1479) |
| `RFC5880-6.8.7-6` | A system MUST NOT periodically transmit BFD Control packets if bfd.RemoteMinRxInterval is zero. (§6.8.7) | MUST NOT | 6.8.7 - Transmitting BFD Control Packets | **positive:** no positive test. **negative:** no negative test. **{gap}:** TransmitInterval substitutes the slow-start interval when the negotiated maximum is zero (internal/component/bfd/session/timers.go:45-47) and tick carries no bfd.RemoteMinRxInterval == 0 suppression (internal/component/bfd/engine/loop.go:192-201), so periodic transmission continues |
| `RFC5880-6.8.7-7` | A system MUST NOT periodically transmit BFD Control packets if Demand mode is active on the remote system (bfd.RemoteDemandMode is 1, bfd.SessionState is Up, and bfd.RemoteSessionState is Up) and a Poll Sequence is not being transmitted. (§6.8.7) | MUST NOT | 6.8.7 - Transmitting BFD Control Packets | **positive:** no positive test. **negative:** no negative test. **{gap}:** the same tick producer (internal/component/bfd/engine/loop.go:192-201) has no remote-Demand suppression, so periodic transmission continues while the remote Demand mode is active with no Poll in flight |
| `RFC5880-6.8.7-8` | If rate limiting is in effect, the advertised value of Desired Min TX Interval MUST be greater than or equal to the interval between transmitted packets imposed by the rate limiting function. (§6.8.7) | MUST | 6.8.7 - Transmitting BFD Control Packets | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze does not rate-limit Final packets -- handleInbound sends the Final unconditionally on every received Poll (internal/component/bfd/engine/loop.go:140-142) and no rate limiter exists on that path, so the conditional advertised-interval floor never applies |
| `RFC5880-6.8.4-1` | If Demand mode is not active, and a period of time equal to the Detection Time passes without receiving a BFD Control packet from the remote system, and bfd.SessionState is Init or Up, the session has gone down -- the local system MUST set bfd.SessionState to Down and bfd.LocalDiag to 1 (Control Detection Time Expired). (§6.8.4) | MUST | 6.8.4 - Calculating the Detection Time | **positive:** `unit/verify` [`TestRFC5880DetectionExpiryDownDiagOne`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L308). **positive:** `unit/verify` [`TestRFC5880DetectionExpiryFromUpDownDiagOne`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_session_clauses_test.go#L235). **negative:** `unit/verify` [`TestRFC5880DetectionExpiryIgnoredWhenDown`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L330) |
| `RFC5880-6.8.5-1` | When the Echo function is active and a sufficient number of Echo packets have not arrived as they should, the session has gone down -- the local system MUST set bfd.SessionState to Down and bfd.LocalDiag to 2 (Echo Function Failed). (§6.8.5) | MUST | 6.8.5 - Detecting Failures with the Echo Function | **positive:** `unit/verify` [`TestRFC5880EchoMissDetectedAndReported`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L961). **positive:** `unit/verify` [`TestRFC5880EngineFailsSessionOnMissingEchoes`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_engine_clauses_test.go#L114). **negative:** `unit/verify` [`TestRFC5880EchoReturnClearsMissAndFailIsScoped`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L997) |
| `RFC5880-6.8.15-1` | When the forwarding plane in the local system is reset for some reason, such that the remote system can no longer rely on the local forwarding state, the local system MUST set bfd.LocalDiag to 4 (Forwarding Plane Reset), and set bfd.SessionState to Down. (§6.8.15) | MUST | 6.8.15 - Forwarding Plane Reset | **positive:** no positive test. **negative:** no negative test. **{gap}:** packet.DiagForwardingPlaneReset (internal/component/bfd/packet/diag.go:21) has no producer, and the only external state-forcing entry points are AdminDown and AdminEnable (internal/component/bfd/session/fsm.go:180,192), which move the session to AdminDown or Down carrying the caller's diagnostic and are never called with code 4 |
| `RFC5880-6.8.16-4` | If enabling session Set bfd.SessionState to Down (§6.8.16) | MUST | 6.8.16 - Administrative Control | **positive:** `unit/verify` [`TestRFC5880AdministrativeDisableEnable`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L1094). **negative:** `unit/verify` [`TestRFC5880EnableLandsOnDownWhilePeerSaysUpOrInit`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L1159) |
| `RFC5880-6.8.16-5` | Else Set bfd.SessionState to AdminDown Set bfd.LocalDiag to an appropriate value Cease the transmission of BFD Echo packets (§6.8.16) | MUST | 6.8.16 - Administrative Control | **positive:** `unit/verify` [`TestRFC5880AdminDisableCeasesEchoes`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_engine_clauses_test.go#L191). **positive:** `unit/verify` [`TestRFC5880AdministrativeDisableEnable`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L1089). **negative:** `unit/verify` [`TestRFC5880AdminDisableIgnoresPeerEchoRequest`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_engine_clauses_test.go#L235) |
| `RFC5880-6.8.8-1` | A received BFD Echo packet MUST be demultiplexed to the appropriate session for processing. (§6.8.8) | MUST | 6.8.8 - Reception of BFD Echo Packets | **positive:** `unit/verify` [`TestRFC5880EchoDemultiplexedAmongSessions`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_echo_session_test.go#L122). **positive:** `unit/verify` [`TestRFC5880EchoDemultiplexedToItsSession`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L622). **negative:** `unit/verify` [`TestRFC5880UnknownEchoDropped`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L659) |
| `RFC5880-6.8.8-2` | A means of detecting missing Echo packets MUST be implemented, which most likely involves processing of the Echo packets that are received. (§6.8.8) | MUST | 6.8.8 - Reception of BFD Echo Packets | **positive:** `unit/verify` [`TestRFC5880EchoMissDetectedAndReported`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L956). **positive:** `unit/verify` [`TestRFC5880EngineFailsSessionOnMissingEchoes`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_engine_clauses_test.go#L119). **negative:** `unit/verify` [`TestRFC5880EchoReturnClearsMissAndFailIsScoped`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L992). **negative:** `unit/verify` [`TestRFC5880EngineReturnedEchoesKeepSessionUp`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_engine_clauses_test.go#L140) |
| `RFC5880-6.8.9-1` | BFD Echo packets MUST NOT be transmitted when bfd.SessionState is not Up. (§6.8.9) | MUST NOT | 6.8.9 - Transmission of BFD Echo Packets | **positive:** `unit/verify` [`TestRFC5880EchoTransmittedWhileUp`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L576). **negative:** `unit/verify` [`TestRFC5880NoEchoTransmittedInInitOrDown`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_echo_session_test.go#L69). **negative:** `unit/verify` [`TestRFC5880NoEchoTransmittedWhenNotUp`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L597) |
| `RFC5880-6.8.9-2` | BFD Echo packets MUST NOT be transmitted unless the last BFD Control packet received from the remote system contains a nonzero value in Required Min Echo RX Interval. (§6.8.9) | MUST NOT | 6.8.9 - Transmission of BFD Echo Packets | **positive:** `unit/verify` [`TestRFC5880EchoEnabledWhenPeerAdvertisesNonZero`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L870). **negative:** `unit/verify` [`TestRFC5880EchoFollowsLastAdvertisement`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_session_clauses_test.go#L298). **negative:** `unit/verify` [`TestRFC5880EchoNotTransmittedWithoutPeerAdvertisement`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L891) |
| `RFC5880-6.8.9-3` | The interval between transmitted BFD Echo packets MUST NOT be less than the value advertised by the remote system in Required Min Echo RX Interval (§6.8.9) | MUST NOT | 6.8.9 - Transmission of BFD Echo Packets | **positive:** `unit/verify` [`TestRFC5880EchoIntervalHonorsPeerFloor`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L914). **positive:** `unit/verify` [`TestRFC5880EchoRaisedPeerFloorDelaysNextEcho`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L829). **positive:** `unit/verify` [`TestRFC5880EchoSteadySpacingHonorsPeerFloor`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_engine_clauses_test.go#L163). **negative:** `unit/verify` [`TestRFC5880EchoIntervalNotBelowLocalTarget`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L933). **negative:** `unit/verify` [`TestRFC5880EchoLoweredPeerFloorTakesEffect`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L861) |
| `RFC5880-7-1` | When BFD is used across multiple hops, a congestion control mechanism MUST be implemented (§7) | MUST | 7 - Operational Considerations | **positive:** no positive test. **negative:** no negative test. **{gap}:** the transmit rate is derived solely from TransmitInterval plus jitter (internal/component/bfd/engine/loop.go:197-200) with no congestion-feedback input, and no congestion-control producer exists anywhere under internal/component/bfd |
| `RFC5880-7-2` | When BFD is used across multiple hops, a congestion control mechanism MUST be implemented, and when congestion is detected, the BFD implementation MUST reduce the amount of traffic it generates. (§7) | MUST | 7 - Operational Considerations | **positive:** no positive test. **negative:** no negative test. **{gap}:** the same rate producer (internal/component/bfd/engine/loop.go:197-200) is the only authority over generated traffic and nothing reduces it in response to detected congestion |
| `RFC5880-4.3-1` | This byte MUST be set to zero on transmit (§4.3, §4.4) | MUST | 4.3 | **positive:** `unit/verify` [`TestRFC5880KeyedMD5SectionHeader`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L91). **positive:** `unit/verify` [`TestRFC5880ReservedByteZeroedOnTransmit`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_keyed_test.go#L121). **negative:** no negative test. **{single-polarity}:** Sign hardcodes the Reserved byte to 0 (internal/component/bfd/auth/sha1.go:83) and no code path emits a non-zero value, so there is no non-conformant transmission to reject |
| `RFC5880-6.8.1-15` | It SHOULD be set to a random (but still unique) value to improve security. (§6.8.1) | SHOULD | 6.8.1 - State Variables | **positive:** no positive test. **negative:** no negative test |
| `RFC5880-6.4-1` | A system SHOULD otherwise advertise the lowest value of Required Min RX Interval and Required Min Echo RX Interval that it can under the circumstances, to give the other system more freedom in choosing its transmission rate. (§6.4) | SHOULD | 6.4 - The Echo Function and Asymmetry | **positive:** no positive test. **negative:** no negative test |
| `RFC5880-5-1` | Some form of authentication SHOULD be included, since Echo packets may be spoofed. (§5) | SHOULD | 5 - BFD Echo Packet Format | **positive:** no positive test. **negative:** no negative test |
| `RFC5880-6.8.3-7` | When the Echo function is active, a system SHOULD set bfd.RequiredMinRxInterval to a value of not less than one second (1,000,000 microseconds). (§6.8.3) | SHOULD | 6.8.3 - Timer Manipulation | **positive:** no positive test. **negative:** no negative test |
| `RFC5880-6.8.7-9` | A BFD Control packet SHOULD be transmitted during the interval between periodic Control packet transmissions when the contents of that packet would differ from that in the previously transmitted packet (other than the Poll and Final bits) in order to more rapidly communicate a change in state. (§6.8.7) | SHOULD | 6.8.7 - Transmitting BFD Control Packets | **positive:** no positive test. **negative:** no negative test |
| `RFC5880-6.7.1-1` | In order to avoid security risks, implementations using this method SHOULD only allow the authentication state to be changed at most once without some form of intervention (so that authentication cannot be turned on and off repeatedly simply based on the receipt of BFD Control packets from remote systems). (§6.7.1) | SHOULD | 6.7.1 - Enabling and Disabling Authentication | **positive:** no positive test. **negative:** no negative test |
| `RFC5880-6.7.1-2` | Unless it is desired to enable or disable authentication, an implementation SHOULD NOT allow the authentication state to change based on the receipt of BFD Control packets. (§6.7.1) | SHOULD NOT | 6.7.1 - Enabling and Disabling Authentication | **positive:** no positive test. **negative:** no negative test |
| `RFC5880-6.7.2-2` | the management interface by which the password is configured MUST accept ASCII strings, and SHOULD also allow for the configuration of any arbitrary binary string in hexadecimal form. (§6.7.2) | SHOULD | 6.7.2 - Simple Password Authentication | **positive:** no positive test. **negative:** no negative test |
| `RFC5880-6.7.3-5` | bfd.XmitAuthSeq SHOULD be incremented when the session state changes, or when the transmitted BFD Control packet carries different contents than the previously transmitted packet. (§6.7.3) | SHOULD | 6.7.3 - Keyed MD5 and Meticulous Keyed MD5 Authentication | **positive:** no positive test. **negative:** no negative test |
| `RFC5880-6.7.4-9` | bfd.XmitAuthSeq SHOULD be incremented when the session state changes, or when the transmitted BFD Control packet carries different contents than the previously transmitted packet. (§6.7.4) | SHOULD | 6.7.4 - Keyed SHA1 and Meticulous Keyed SHA1 Authentication | **positive:** `unit/verify` [`TestRFC5880XmitAuthSeqAdvancesOnStateAndContentChange`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_xmit_auth_seq_test.go#L144). **negative:** `unit/verify` [`TestRFC5880XmitAuthSeqAdvancesForAnOutOfTimerFinal`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_xmit_auth_seq_test.go#L194) |
| `RFC5880-6.7.3-14` | SHOULD also allow for the configuration of any arbitrary binary string in hexadecimal form. (§6.7.3) | SHOULD | 6.7.3 - Keyed MD5 and Meticulous Keyed MD5 Authentication | **positive:** no positive test. **negative:** no negative test. **{gap}:** parseAuthConfig reads the secret leaf as a string and uses its bytes as the key (internal/component/bfd/config.go:299); no hexadecimal form is parsed, so a binary Keyed MD5 key cannot be configured |
| `RFC5880-6.7.4-8` | SHOULD also allow for the configuration of any arbitrary binary string in hexadecimal form. (§6.7.4) | SHOULD | 6.7.4 - Keyed SHA1 and Meticulous Keyed SHA1 Authentication | **positive:** no positive test. **negative:** no negative test. **{gap}:** parseAuthConfig reads the secret leaf as a string and uses its bytes as the key (internal/component/bfd/config.go:299); no hexadecimal form is parsed, so a binary Keyed SHA1 key cannot be configured |
| `RFC5880-6.8.16-2` | BFD Control packets SHOULD be transmitted for at least a Detection Time after transitioning to AdminDown state in order to ensure that the remote system is aware of the state change. (§6.8.16) | SHOULD | 6.8.16 - Administrative Control | **positive:** `unit/verify` [`TestRFC5880AdminDownControlTransmittedForADetectionTime`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_admindown_transmit_test.go#L55). **negative:** no negative test. **{single-polarity}:** the obligation is to keep sending, and no input or peer behaviour exists for Ze to refuse: stopping once the window has passed is what RFC5880-6.8.16-3 leaves optional ("MAY be transmitted indefinitely"), so silence after it is no violation a negative could catch. Loop.tick (internal/component/bfd/engine/loop.go) sends AdminDown Control packets until Machine.AdminDownTransmitEnd, set by Machine.AdminDown to three times the longer of the local and the remote Detection Time (owner ruling 2026-09-30, see RFC5880-6.8.16-3) |
| `RFC5880-7-3` | As such, a congestion control algorithm SHOULD be used even across single hops in order to avoid the possibility of catastrophic system collapse, as such failures have been seen repeatedly in other periodic Hello-based protocols. (§7) | SHOULD | 7 - Operational Considerations | **positive:** no positive test. **negative:** no negative test |
| `RFC5880-9-1` | If BFD is run across multiple hops or an insecure tunnel (such as Generic Routing Encapsulation (GRE)), the Authentication Section SHOULD be utilized. (§9) | SHOULD | 9 - Security Considerations | **positive:** no positive test. **negative:** no negative test |
| `RFC5880-6.8.18-1` | If the remote system does not receive any BFD Control packets for a Detection Time, it SHOULD reset bfd.RemoteMinRxInterval to its initial value of 1 (per section 6.8.1, since it is no longer required to maintain previous session state) and then can transmit at its own rate. (§6.8.18) | SHOULD | 6.8.18 - Holding Down Sessions | **positive:** no positive test. **negative:** no negative test |
| `RFC5880-6.2-1` | A session MAY be kept administratively down by entering the AdminDown state and sending an explanatory diagnostic code in the Diagnostic field. (§6.1) | MAY | 6.1 - Overview | **positive:** no positive test. **negative:** no negative test |
| `RFC5880-6.3-2` | it is permissible for a system to change its discriminator during a session without affecting the session state (§6.3) | MAY | 6.3 - Demultiplexing and the Discriminator Fields | **positive:** no positive test. **negative:** no negative test |
| `RFC5880-6.8.6-17` | If a matching session is not found, a new session MAY be created, or the packet MAY be discarded. (§6.8.6) | MAY | 6.8.6 - Reception of BFD Control Packets | **positive:** no positive test. **negative:** no negative test |
| `RFC5880-6.8.7-10` | A system MAY limit the rate at which such packets are transmitted. (§6.8.7) | MAY | 6.8.7 - Transmitting BFD Control Packets | **positive:** no positive test. **negative:** no negative test |
| `RFC5880-6.6-4` | Demand mode MAY be enabled or disabled at any time, independently in each direction, by setting or clearing the Demand (D) bit in the BFD Control packet, without affecting the BFD session state. (§6.6) | MAY | 6.6 - Demand mode | **positive:** no positive test. **negative:** no negative test |
| `RFC5880-6.8.13-1` | If it is desired to start or stop the transmission of BFD Echo packets, this MAY be done at any time (subject to the transmission requirements detailed in section 6.8.9). If it is desired to enable or disable the looping back of received BFD Echo packets, this MAY be done at any time by changing the value of Required Min Echo RX Interval to zero or nonzero in outgoing BFD Control packets. (§6.8.13) | MAY | 6.8.13 - Enabling or Disabling The Echo Function | **positive:** no positive test. **negative:** no negative test |
| `RFC5880-6.7.3-6` | For Keyed MD5, bfd.XmitAuthSeq MAY be incremented in a circular fashion (when treated as an unsigned 32-bit value). (§6.7.3) | MAY | 6.7.3 - Keyed MD5 and Meticulous Keyed MD5 Authentication | **positive:** no positive test. **negative:** no negative test |
| `RFC5880-6.7.4-6` | For Keyed SHA1, bfd.XmitAuthSeq MAY be incremented in a circular fashion (when treated as an unsigned 32-bit value). (§6.7.4) | MAY | 6.7.4 - Keyed SHA1 and Meticulous Keyed SHA1 Authentication | **positive:** no positive test. **negative:** no negative test |
| `RFC5880-6.8.1-16` | A system MAY preserve session state longer than this. (§6.8.1) | MAY | 6.8.1 - State Variables | **positive:** no positive test. **negative:** no negative test |
| `RFC5880-6.8.16-3` | BFD Control packets MAY be transmitted indefinitely after transitioning to AdminDown state in order to maintain session state in each system (§6.8.16) | MAY | 6.8.16 - Administrative Control | **positive:** no positive test. **negative:** `unit/verify` [`TestRFC5880NoPeriodicTransmitWhileAdminDown`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L403). **{single-polarity}:** the MAY leaves the length of AdminDown transmission to the implementer, and Ze's decision (owner ruling 2026-09-30) is three Detection Times, three times the one RFC5880-6.8.16-2 sets as its floor, so the change survives a run of lost packets, and then silence: Ze never transmits indefinitely, so a positive would have nothing to exercise. Machine.AdminDown (internal/component/bfd/session/fsm.go) sets Machine.AdminDownTransmitEnd to the transition plus adminDownTransmitDetectionTimes (3) Detection Times, and Loop.tick (internal/component/bfd/engine/loop.go) stops there. TestRFC5880NoPeriodicTransmitWhileAdminDown pins the choice: packets after one and two Detection Times, none at three nor an hour later |
| `RFC5880-9-2` | When a BFD session is directly connected across a single link (physical, or a secure tunnel such as IPsec), the TTL or Hop Count MUST be set to the maximum on transmit, and checked to be equal to the maximum value on reception (and the packet dropped if this is not the case). (§9) | MUST | 9 - Security Considerations | **positive:** `unit/verify` [`TestRFC5880SingleHopReceiveTTLMaxAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_ttl_accept_test.go#L5). **positive:** `unit/verify` [`TestRFC5880SingleHopTransmitHopLimitIsMaximumIPv6`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/transport/rfc5880_hop_limit_linux_test.go#L14). **positive:** `unit/verify` [`TestRFC5880SingleHopTransmitTTLIsMaximum`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/transport/rfc5880_test.go#L43). **negative:** `unit/verify` [`TestRFC5880SingleHopReceiveTTLNotMaxDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L160) |
| `RFC5880-6.8.14-1` | If Demand mode is no longer active on the remote system, the local system MUST begin transmitting periodic BFD Control packets (§6.8.14) | MUST | 6.8.14 - Enabling or Disabling Demand Mode | **positive:** `unit/verify` [`TestRFC5880PeriodicTransmitWhenRemoteDemandInactive`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L372). **negative:** no negative test. **{single-polarity}:** tick transmits periodic Control packets without consulting the remote D bit (internal/component/bfd/engine/loop.go:186-201), so a remote that stops asserting Demand keeps receiving them and there is no suppressed state to leave |
| `RFC5880-6.8.17-1` | If Demand mode is active on the remote system (the local system is not transmitting periodic BFD Control packets), a Poll Sequence MUST be initiated to ensure that the diagnostic code is transmitted. (§6.8.17) | MUST | 6.8.17 - Concatenated Paths | **positive:** no positive test. **negative:** no negative test. **{gap}:** packet.DiagConcatPathDown (internal/component/bfd/packet/diag.go:23) has no producer and the remote Demand state stored at internal/component/bfd/session/fsm.go:58 is never read, so a concatenated-path failure neither sets the diagnostic nor initiates a Poll Sequence |
| `RFC5880-6.8.3-8` | In any case other than those explicitly called out above, timing parameter changes MUST be effected immediately (changing the transmission rate and/or the Detection Time). (§6.8.3) | MUST | 6.8.3 - Timer Manipulation | **positive:** `unit/verify` [`TestRFC5880RemoteMinRxReductionHonoredImmediately`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L505). **positive:** `unit/verify` [`TestRFC5880RemoteTimingChangeMovesDetectionTimeAtOnce`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_session_clauses_test.go#L193). **negative:** no negative test. **{single-polarity}:** TransmitInterval and DetectionInterval are recomputed from the live bfd.* variables on every call (internal/component/bfd/session/timers.go:24-49), so an unexcepted timing change is in force at the next evaluation and no held-back value exists to reject |
| `RFC5880-6.7.3-7` | the management interface by which the key is configured MUST accept ASCII strings (§6.7.3) | MUST | 6.7.3 - Keyed MD5 and Meticulous Keyed MD5 Authentication | **positive:** `unit/verify` [`TestRFC5880KeyManagementAcceptsASCIIStrings`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5880_test.go#L26). **positive:** `unit/verify` [`TestRFC5880KeyManagementConfigEntryAcceptsASCIIStrings`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5880_key_management_config_test.go#L92). **negative:** `unit/verify` [`TestRFC5880KeyManagementConfigEntryKeepsEveryASCIICharacter`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5880_key_management_config_test.go#L117) |
| `RFC5880-6.7.2-3` | The currently selected password and Key ID for the session MUST be stored in the Authentication Section of each outgoing BFD Control packet. (§6.7.2) | MUST | 6.7.2 - Simple Password Authentication | **positive:** `unit/verify` [`TestRFC5880SimplePasswordSectionCarriesPasswordAndKeyID`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L686). **negative:** `unit/verify` [`TestRFC5880SimplePasswordOtherThanSelectedPairDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_section_fields_test.go#L63). **negative:** `unit/verify` [`TestRFC5880SimplePasswordSignerNeedsPassword`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_keyed_test.go#L182) |
| `RFC5880-6.7.3-8` | The Sequence Number field MUST be set to bfd.XmitAuthSeq. (§6.7.3) | MUST | 6.7.3 - Keyed MD5 and Meticulous Keyed MD5 Authentication | **positive:** `unit/verify` [`TestRFC5880MD5SequenceFieldIsXmitAuthSeq`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_md5_seq_field_test.go#L15). **negative:** `unit/verify` [`TestRFC5880SequenceFieldIsXmitAuthSeqAcrossTheCounterRange`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_auth_seq_field_test.go#L17) |
| `RFC5880-6.7.2-4` | If the received BFD Control packet does not contain an Authentication Section, or the Auth Type is not 1 (Simple Password), then the received packet MUST be discarded. (§6.7.2) | MUST | 6.7.2 - Simple Password Authentication | **positive:** `unit/verify` [`TestRFC5880SimplePasswordSectionOfTypeOneAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_keyed_test.go#L194). **negative:** `unit/verify` [`TestRFC5880SimplePasswordMissingSectionOrOtherTypeDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_keyed_test.go#L211) |
| `RFC5880-6.7.2-5` | If the Auth Key ID field does not match the ID of a configured password, the received packet MUST be discarded. (§6.7.2) | MUST | 6.7.2 - Simple Password Authentication | **positive:** `unit/verify` [`TestRFC5880SimplePasswordMatchingKeyIDAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L776). **negative:** `unit/verify` [`TestRFC5880SimplePasswordWrongKeyIDDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L759) |
| `RFC5880-6.7.2-6` | If the Auth Len field is not equal to the length of the password selected by the key ID, plus three, the packet MUST be discarded. (§6.7.2) | MUST | 6.7.2 - Simple Password Authentication | **positive:** `unit/verify` [`TestRFC5880SimplePasswordAuthLenPlusThreeAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_keyed_test.go#L244). **negative:** `unit/verify` [`TestRFC5880SimplePasswordAuthLenNotPlusThreeDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_keyed_test.go#L261) |
| `RFC5880-6.7.3-16` | If the received BFD Control packet does not contain an Authentication Section, or the Auth Type is not correct (2 for Keyed MD5 or 3 for Meticulous Keyed MD5), then the received packet MUST be discarded. (§6.7.3) | MUST | 6.7.3 - Keyed MD5 and Meticulous Keyed MD5 Authentication | **positive:** `unit/verify` [`TestRFC5880MD5AuthTypeCorrectAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_md5_split_test.go#L76). **negative:** `unit/verify` [`TestRFC5880KeyedMissingSectionOrWrongTypeDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_md5_split_test.go#L93) |
| `RFC5880-6.7.3-17` | If the Auth Key ID field does not match the ID of a configured authentication key, the received packet MUST be discarded. (§6.7.3) | MUST | 6.7.3 - Keyed MD5 and Meticulous Keyed MD5 Authentication | **positive:** `unit/verify` [`TestRFC5880MD5ConfiguredKeyIDAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_md5_split_test.go#L157). **negative:** `unit/verify` [`TestRFC5880MD5UnconfiguredKeyIDDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_md5_split_test.go#L171) |
| `RFC5880-6.7.3-18` | If the Auth Len field is not equal to 24, the packet MUST be discarded. (§6.7.3) | MUST | 6.7.3 - Keyed MD5 and Meticulous Keyed MD5 Authentication | **positive:** `unit/verify` [`TestRFC5880MD5AuthLen24Accepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_md5_split_test.go#L193). **negative:** `unit/verify` [`TestRFC5880MD5AuthLenNot24Discarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_md5_split_test.go#L213) |
| `RFC5880-6.7.4-14` | If the received BFD Control packet does not contain an Authentication Section, or the Auth Type is not correct (4 for Keyed SHA1 or 5 for Meticulous Keyed SHA1), then the received packet MUST be discarded. (§6.7.4) | MUST | 6.7.4 - Keyed SHA1 and Meticulous Keyed SHA1 Authentication | **positive:** `unit/verify` [`TestRFC5880AuthTypeMatchAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L301). **negative:** `unit/verify` [`TestRFC5880AuthTypeMismatchDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L314). **negative:** `unit/verify` [`TestRFC5880KeyedMissingSectionOrWrongTypeDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_md5_split_test.go#L98) |
| `RFC5880-6.7.4-15` | If the Auth Key ID field does not match the ID of a configured authentication key, the received packet MUST be discarded. (§6.7.4) | MUST | 6.7.4 - Keyed SHA1 and Meticulous Keyed SHA1 Authentication | **positive:** `unit/verify` [`TestRFC5880AuthKeyIDMatchAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L289). **negative:** `unit/verify` [`TestRFC5880AuthKeyIDMismatchDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L270) |
| `RFC5880-6.7.4-16` | If the Auth Len field is not equal to 28, the packet MUST be discarded. (§6.7.4) | MUST | 6.7.4 - Keyed SHA1 and Meticulous Keyed SHA1 Authentication | **positive:** `unit/verify` [`TestRFC5880AuthLenExpectedAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L330). **positive:** `unit/verify` [`TestRFC5880SHA1AuthLen28HandBuiltAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_sha1_authlen_test.go#L38). **negative:** `unit/verify` [`TestRFC5880AuthLenMismatchDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L347). **negative:** `unit/verify` [`TestRFC5880SHA1AuthLenByteDiscardedWithAuthenticDigest`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_sha1_authlen_test.go#L53) |
| `RFC5880-6.7.2-7` | If the Password field does not match the password selected by the key ID, the packet MUST be discarded. (§6.7.2) | MUST | 6.7.2 - Simple Password Authentication | **positive:** `unit/verify` [`TestRFC5880SimplePasswordMatchAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L709). **negative:** `unit/verify` [`TestRFC5880SimplePasswordMismatchDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L728) |
| `RFC5880-6.7.3-9` | For Keyed MD5, if the sequence number lies outside of the range of bfd.RcvAuthSeq to bfd.RcvAuthSeq+(3*Detect Mult) inclusive (when treated as an unsigned 32-bit circular number space), the received packet MUST be discarded. (§6.7.3) | MUST | 6.7.3 - Keyed MD5 and Meticulous Keyed MD5 Authentication | **positive:** `unit/verify` [`TestRFC5880KeyedWindowFollowsReceivedDetectMult`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_keyed_test.go#L325). **negative:** `unit/verify` [`TestRFC5880KeyedOutsideWindowDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_keyed_test.go#L366). **negative:** `unit/verify` [`TestRFC5880KeyedSequenceBeyondWindowDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L838). **negative:** `unit/verify` [`TestRFC5880SequenceFieldOtherThanXmitAuthSeqDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_auth_wrap_test.go#L67) |
| `RFC5880-6.7.3-10` | For Meticulous Keyed MD5, if the sequence number lies outside of the range of bfd.RcvAuthSeq+1 to bfd.RcvAuthSeq+(3*Detect Mult) inclusive (when treated as an unsigned 32-bit circular number space) the received packet MUST be discarded. (§6.7.3) | MUST | 6.7.3 - Keyed MD5 and Meticulous Keyed MD5 Authentication | **positive:** `unit/verify` [`TestRFC5880MeticulousSequenceInsideWindowAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L884). **positive:** `unit/verify` [`TestRFC5880MeticulousWindowFollowsReceivedDetectMult`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_keyed_test.go#L408). **negative:** `unit/verify` [`TestRFC5880MeticulousOutsideWindowDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_keyed_test.go#L442). **negative:** `unit/verify` [`TestRFC5880MeticulousSequenceOutsideWindowDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L907). **negative:** `unit/verify` [`TestRFC5880SequenceFieldOtherThanXmitAuthSeqDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_auth_wrap_test.go#L72) |
| `RFC5880-6.7.3-11` | Otherwise (the digest does not match the Auth Key/Digest field), the received packet MUST be discarded. (§6.7.3) | MUST | 6.7.3 - Keyed MD5 and Meticulous Keyed MD5 Authentication | **positive:** `unit/verify` [`TestRFC5880KeyedMD5IndependentDigestAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_keyed_test.go#L462). **negative:** `unit/verify` [`TestRFC5880KeyedMD5DigestMismatchDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_keyed_test.go#L480) |
| `RFC5880-6.7.4-12` | For Keyed SHA1, if the sequence number lies outside of the range of bfd.RcvAuthSeq to bfd.RcvAuthSeq+(3*Detect Mult) inclusive (when treated as an unsigned 32-bit circular number space), the received packet MUST be discarded. (§6.7.4) | MUST | 6.7.4 - Keyed SHA1 and Meticulous Keyed SHA1 Authentication | **positive:** `unit/verify` [`TestRFC5880KeyedSequenceAtOrAboveFloorAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L540). **positive:** `unit/verify` [`TestRFC5880KeyedSequenceWindowWraps`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L863). **positive:** `unit/verify` [`TestRFC5880KeyedWindowFollowsReceivedDetectMult`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_keyed_test.go#L328). **negative:** `unit/verify` [`TestRFC5880KeyedOutsideWindowDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_keyed_test.go#L368). **negative:** `unit/verify` [`TestRFC5880KeyedSequenceBelowFloorDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L561) |
| `RFC5880-6.7.4-13` | For Meticulous Keyed SHA1, if the sequence number lies outside of the range of bfd.RcvAuthSeq+1 to bfd.RcvAuthSeq+(3*Detect Mult) inclusive (when treated as an unsigned 32-bit circular number space, the received packet MUST be discarded. (§6.7.4) | MUST | 6.7.4 - Keyed SHA1 and Meticulous Keyed SHA1 Authentication | **positive:** `unit/verify` [`TestRFC5880MeticulousWindowFollowsReceivedDetectMult`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_keyed_test.go#L410). **negative:** `unit/verify` [`TestRFC5880MeticulousOutsideWindowDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_keyed_test.go#L444) |
| `RFC5880-6.7.4-17` | Otherwise (the hash does not match the Auth Key/Hash field), the received packet MUST be discarded. (§6.7.4) | MUST | 6.7.4 - Keyed SHA1 and Meticulous Keyed SHA1 Authentication | **positive:** `unit/verify` [`TestRFC5880DigestMatchAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L381). **negative:** `unit/verify` [`TestRFC5880DigestMismatchDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L394) |
| `RFC5880-6.7.3-12` | Otherwise (bfd.AuthSeqKnown is 0), bfd.AuthSeqKnown MUST be set to 1, and bfd.RcvAuthSeq MUST be set to the value of the received Sequence Number field. (§6.7.3, §6.7.4) | MUST | 6.7.3 - Keyed MD5 and Meticulous Keyed MD5 Authentication | **positive:** `unit/verify` [`TestRFC5880FirstAuthenticatedPacketSeedsReplayFloor`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L456). **positive:** `unit/verify` [`TestRFC5880FirstPacketSeedsReplayFloorBeforeDigest`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L480). **positive:** `unit/verify` [`TestRFC5880KeyedMD5FirstPacketSeedsReplayFloor`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_keyed_test.go#L580). **negative:** `unit/verify` [`TestRFC5880KeyedMD5KnownFloorNotReseeded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_keyed_test.go#L613). **negative:** `unit/verify` [`TestRFC5880KnownSequenceNotReseededByForgedPacket`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L514) |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC5880-6.8.1-11`](#rfc5880-6.8.1-11) This variable MUST be initialized to a random 32-bit value. (§6.8.1) | {gap}, no test | SetAuth seeds bfd.XmitAuthSeq from the persister or leaves the Vars zero value (internal/component/bfd/session/auth.go:36) and Init never randomizes it (internal/component/bfd/session/session.go:245-259), so the initial transmit sequence is 0 rather than a random 32-bit value |
| [`RFC5880-6.8.1-13`](#rfc5880-6.8.1-13) This variable MUST be set to zero after no packets have been received on this session for at least twice the Detection Time. (§6.8.1) | {gap}, no test | bfd.AuthSeqKnown is SeqState.initialized, which only Advance sets (internal/component/bfd/auth/meticulous.go:63-66) and nothing ever clears; CheckDetection clears the detection deadline alone (internal/component/bfd/session/timers.go:82), so the flag survives twice the Detection Time of silence |
| [`RFC5880-6.8.3-4`](#rfc5880-6.8.3-4) If bfd.RequiredMinRxInterval is reduced and bfd.SessionState is Up, the previous value of bfd.RequiredMinRxInterval MUST be used when calculating the Detection Time for the remote system until the Poll Sequence described above has terminated. (§6.8.3) | {gap}, no test | revertEchoSlowdownLocked reduces bfd.RequiredMinRxInterval while Up (internal/component/bfd/session/timers.go:249) and DetectionInterval immediately uses the reduced value (internal/component/bfd/session/timers.go:29); no previous value is retained until the Poll Sequence terminates |
| [`RFC5880-6.6-2`](#rfc5880-6.6-2) When the transmitted value of the Demand (D) bit is to be changed, the transmitting system MUST initiate a Poll Sequence in conjunction with changing the bit in order to ensure that both systems are aware of the change. (§6.6) | {gap}, no test | bfd.DemandMode has no writer in production code -- it is declared at internal/component/bfd/session/session.go:58 and read only by canSetDemand (internal/component/bfd/session/fsm.go:247) -- so no D-bit change path exists and none initiates a Poll Sequence |
| [`RFC5880-6.6-3`](#rfc5880-6.6-3) If Demand mode is active on either or both systems, a Poll Sequence MUST be initiated whenever the contents of the next BFD Control packet to be sent would be different than the contents of the previous packet, with the exception of the Poll (P) and Final (F) bits. (§6.6) | {gap}, no test | bfd.RemoteDemandMode is stored by Receive (internal/component/bfd/session/fsm.go:58) and read nowhere, and bfd.DemandMode has no writer (internal/component/bfd/session/session.go:58), so no code path starts a Poll when packet contents would change while Demand mode is active |
| [`RFC5880-6.8.6-14`](#rfc5880-6.8.6-14) If bfd.RemoteDemandMode is 1, bfd.SessionState is Up, and bfd.RemoteSessionState is Up, Demand mode is active on the remote system and the local system MUST cease the periodic transmission of BFD Control packets (see section 6.8.7). (§6.8.6) | {gap}, no test | tick transmits whenever the periodic deadline has passed and never consults the remote Demand state (internal/component/bfd/engine/loop.go:192-201), so periodic Control packets continue after the peer sets D=1 |
| [`RFC5880-6.8.7-6`](#rfc5880-6.8.7-6) A system MUST NOT periodically transmit BFD Control packets if bfd.RemoteMinRxInterval is zero. (§6.8.7) | {gap}, no test | TransmitInterval substitutes the slow-start interval when the negotiated maximum is zero (internal/component/bfd/session/timers.go:45-47) and tick carries no bfd.RemoteMinRxInterval == 0 suppression (internal/component/bfd/engine/loop.go:192-201), so periodic transmission continues |
| [`RFC5880-6.8.7-7`](#rfc5880-6.8.7-7) A system MUST NOT periodically transmit BFD Control packets if Demand mode is active on the remote system (bfd.RemoteDemandMode is 1, bfd.SessionState is Up, and bfd.RemoteSessionState is Up) and a Poll Sequence is not being transmitted. (§6.8.7) | {gap}, no test | the same tick producer (internal/component/bfd/engine/loop.go:192-201) has no remote-Demand suppression, so periodic transmission continues while the remote Demand mode is active with no Poll in flight |
| [`RFC5880-6.8.7-8`](#rfc5880-6.8.7-8) If rate limiting is in effect, the advertised value of Desired Min TX Interval MUST be greater than or equal to the interval between transmitted packets imposed by the rate limiting function. (§6.8.7) | no test | no test carries this requirement id; annotated {not-applicable}: ze does not rate-limit Final packets -- handleInbound sends the Final unconditionally on every received Poll (internal/component/bfd/engine/loop.go:140-142) and no rate limiter exists on that path, so the conditional advertised-interval floor never applies |
| [`RFC5880-6.8.15-1`](#rfc5880-6.8.15-1) When the forwarding plane in the local system is reset for some reason, such that the remote system can no longer rely on the local forwarding state, the local system MUST set bfd.LocalDiag to 4 (Forwarding Plane Reset), and set bfd.SessionState to Down. (§6.8.15) | {gap}, no test | packet.DiagForwardingPlaneReset (internal/component/bfd/packet/diag.go:21) has no producer, and the only external state-forcing entry points are AdminDown and AdminEnable (internal/component/bfd/session/fsm.go:180,192), which move the session to AdminDown or Down carrying the caller's diagnostic and are never called with code 4 |
| [`RFC5880-7-1`](#rfc5880-7-1) When BFD is used across multiple hops, a congestion control mechanism MUST be implemented (§7) | {gap}, no test | the transmit rate is derived solely from TransmitInterval plus jitter (internal/component/bfd/engine/loop.go:197-200) with no congestion-feedback input, and no congestion-control producer exists anywhere under internal/component/bfd |
| [`RFC5880-7-2`](#rfc5880-7-2) When BFD is used across multiple hops, a congestion control mechanism MUST be implemented, and when congestion is detected, the BFD implementation MUST reduce the amount of traffic it generates. (§7) | {gap}, no test | the same rate producer (internal/component/bfd/engine/loop.go:197-200) is the only authority over generated traffic and nothing reduces it in response to detected congestion |
| [`RFC5880-6.7.3-14`](#rfc5880-6.7.3-14) SHOULD also allow for the configuration of any arbitrary binary string in hexadecimal form. (§6.7.3) | {gap} | parseAuthConfig reads the secret leaf as a string and uses its bytes as the key (internal/component/bfd/config.go:299); no hexadecimal form is parsed, so a binary Keyed MD5 key cannot be configured |
| [`RFC5880-6.7.4-8`](#rfc5880-6.7.4-8) SHOULD also allow for the configuration of any arbitrary binary string in hexadecimal form. (§6.7.4) | {gap} | parseAuthConfig reads the secret leaf as a string and uses its bytes as the key (internal/component/bfd/config.go:299); no hexadecimal form is parsed, so a binary Keyed SHA1 key cannot be configured |
| [`RFC5880-6.8.17-1`](#rfc5880-6.8.17-1) If Demand mode is active on the remote system (the local system is not transmitting periodic BFD Control packets), a Poll Sequence MUST be initiated to ensure that the diagnostic code is transmitted. (§6.8.17) | {gap}, no test | packet.DiagConcatPathDown (internal/component/bfd/packet/diag.go:23) has no producer and the remote Demand state stored at internal/component/bfd/session/fsm.go:58 is never read, so a concatenated-path failure neither sets the diagnostic nor initiates a Poll Sequence |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC5880-4.1-1`](#rfc5880-4.1-1)

The contents of transmitted BFD Control packets MUST be set as follows: Version Set to the current version number (1). (§6.8.7)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge 2026-09-30 (rejudge, subnet). Row quotes §6.8.7 "Version Set to the current version number (1)" (dated correction explains the kept id). + session TestRFC5880TransmittedControlCarriesVersionOne encodes Build in Down/Init/Up/AdminDown and BuildFinal through WriteTo and asserts octet0>>5 == 1; - session TestRFC5880TransmittedVersionNeverOverwrittenByDiag (Diag 0xFF cannot write into the version bits, packet still parses). Records: + revert session/fsm.go::Build, - revert packet/control.go::WriteTo, both observed red; author overlay dropping the Diag mask reds only the -. Residual: the older packet-level negative TestRFC5880VersionNotOneDiscarded asserts the receive discard, which is RFC5880-6.8.6-1, not this transmit row; it stays tagged but carries no weight here.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880VersionNotOneDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/packet/rfc5880_test.go#L64) | unit/verify | revert, verified |
| negative | [`TestRFC5880TransmittedVersionNeverOverwrittenByDiag`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_transmit_header_test.go#L52) | unit/verify | revert, verified |
| positive | [`TestRFC5880VersionOneAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/packet/rfc5880_test.go#L44) | unit/verify | revert, verified |
| positive | [`TestRFC5880TransmittedControlCarriesVersionOne`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_transmit_header_test.go#L38) | unit/verify | revert, verified |

### [`RFC5880-4.1-2`](#rfc5880-4.1-2)

It MUST be zero on both transmit and receipt. (§4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge 2026-09-30 (rejudge, subnet). §4.1 "It MUST be zero on both transmit and receipt." Transmit clause: + session TestRFC5880TransmittedControlMultipointZero asserts the M bit clear on the session's own Build/BuildFinal wire in every transmitting state (record: revert session/fsm.go::Build, observed red); a property of every transmitted packet, single polarity is valid. Receipt clause: - packet TestRFC5880MultipointSetDiscarded (ParseControl refuses M=1) and + TestRFC5880MultipointZeroAccepted.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880MultipointSetDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/packet/rfc5880_test.go#L96) | unit/verify | revert, verified |
| positive | [`TestRFC5880MultipointZeroAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/packet/rfc5880_test.go#L80) | unit/verify | revert, verified |
| positive | [`TestRFC5880TransmittedControlMultipointZero`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_transmit_header_test.go#L73) | unit/verify | revert, verified |

### [`RFC5880-6.3-1`](#rfc5880-6.3-1)

Each system MUST choose an opaque discriminator value that identifies each session, and which MUST be unique among all BFD sessions on the system. (§6.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: discriminator 0 or a value held by another session. Red: TestRFC5880DiscriminatorsAreNonZeroAndUnique Fatalf on d==0 or a repeat across 32 EnsureSession calls; TestRFC5880DiscriminatorAllocatorSkipsReservedAndTaken parks nextDiscr on 0 with 1 and 2 taken and Fatalf if 0, 1 or 2 is returned. 'Opaque' carries no testable behaviour.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880DiscriminatorAllocatorSkipsReservedAndTaken`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L90) | unit/verify | unproven |
| positive | [`TestRFC5880DiscriminatorsAreNonZeroAndUnique`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L62) | unit/verify | unproven |

### [`RFC5880-6.8.1-1`](#rfc5880-6.8.1-1)

This variable MUST be initialized to Down. (§6.8.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. single-polarity positive: Init state is Down.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC5880InitStatesAreDown`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L73) | unit/verify | unproven |

### [`RFC5880-6.8.1-2`](#rfc5880-6.8.1-2)

This variable MUST be initialized to Down. (§6.8.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. single-polarity positive: RemoteSessionState is Down after Init.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC5880InitStatesAreDown`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L76) | unit/verify | unproven |

### [`RFC5880-6.8.1-3`](#rfc5880-6.8.1-3)

It MUST be unique across all BFD sessions on this system, and nonzero. (§6.8.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. same allocator tests as 6.3-1: nonzero and unique LocalDiscr, skip of 0 and taken values.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880DiscriminatorAllocatorSkipsReservedAndTaken`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L95) | unit/verify | unproven |
| positive | [`TestRFC5880DiscriminatorsAreNonZeroAndUnique`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L67) | unit/verify | unproven |

### [`RFC5880-6.8.1-4`](#rfc5880-6.8.1-4)

This MUST be initialized to zero. (§6.8.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge rejudge 2026-09-30 (bfd engine 2). Positive TestRFC5880InitVariableDefaults fails on RemoteDiscriminator()!=0 after Init. Genuine negative (R1 route b) TestRFC5880InitClearsLearnedRemoteDiscr: a Machine that learned the peer discriminator 1 is Init-ed again and must hold 0. Targeted break Init literal RemoteDiscr: 0 -> m.vars.RemoteDiscr (carry the learned value) observed red on the negative (the positive stays green under it, since a fresh Machine holds 0 anyway). The old negative tag (Receive installing My Discriminator, a 6.8.6 step) was removed with approval.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880InitClearsLearnedRemoteDiscr`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_session_clauses_test.go#L375) | unit/verify | revert, verified |
| positive | [`TestRFC5880InitVariableDefaults`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L88) | unit/verify | revert, verified |

### [`RFC5880-6.8.1-5`](#rfc5880-6.8.1-5)

If a period of a Detection Time passes without the receipt of a valid, authenticated BFD packet from the remote system, this variable MUST be set to zero. (§6.8.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge rejudge 2026-09-30 (bfd session+engine continuation). Adds TestRFC5880RemoteDiscrClearedDespiteInvalidPackets: on an authenticated Up session an A-bit-clear packet (ErrAuthMismatch) and a Your-Discriminator-0 Up packet (ErrYourDiscriminatorReset) inside the Detection Time leave CheckDetection to clear bfd.RemoteDiscr at its end, so the 'valid, authenticated' clause is asserted; mutant fsm.go discard guard observed red. Expiry clauses and the neighbor-signaled-Down negative stand as before. Digest failures are refused in the engine before Receive (auth rows).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880RemoteDiscrKeptOnNeighborSignaledDown`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L180) | unit/verify | revert, verified |
| positive | [`TestRFC5880RemoteDiscrClearedDespiteInvalidPackets`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_session_clauses_test.go#L59) | unit/verify | mutant, verified |
| positive | [`TestRFC5880RemoteDiscrClearedAfterSilenceWhileDown`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L1258) | unit/verify | revert, verified |
| positive | [`TestRFC5880RemoteDiscrClearedOnDetectionExpiry`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L153) | unit/verify | revert, verified |

### [`RFC5880-6.8.1-6`](#rfc5880-6.8.1-6)

This MUST be initialized to zero (No Diagnostic). (§6.8.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge rejudge 2026-09-30 (bfd engine 2). Unit re-read after TestRFC5880InitVariablesAreNotConstants lost its 6.8.1-4 tag (doc comment only, body unchanged). Judgement unchanged: Forbidden: bfd.LocalDiag initialised to anything but 0. TestRFC5880InitVariableDefaults fails on LocalDiag() != DiagNone straight after Init. Negative TestRFC5880InitVariablesAreNotConstants shows the value is an initial value, not a constant: a peer-signalled Down moves it to DiagNeighborSignaledDown.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880InitVariablesAreNotConstants`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L116) | unit/verify | unproven |
| positive | [`TestRFC5880InitVariableDefaults`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L90) | unit/verify | unproven |

### [`RFC5880-6.8.1-7`](#rfc5880-6.8.1-7)

This MUST be initialized to a value of at least one second (1,000,000 microseconds) according to the rules described in section 6.8.3. (§6.8.1, §6.8.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RA-BFD strict re-read 2026-09-27. One clause: bfd.DesiredMinTxInterval initialised to at least 1,000,000 us. Forbidden: Init storing a value below 1 s (here the configured 300 ms). Red: TestRFC5880SlowStartFloorWhileNotUp Fatalf when DesiredMinTxIntervalUs() < 1000000 straight after Init (Init, session.go, assigns SlowStartIntervalUs), and when TransmitInterval is below 1 s while Down. Contrast: TestRFC5880SlowStartFloorLiftedWhenUp fails unless the same accessor reads the configured 300000 at Up, so the positive cannot pass on a constant.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880SlowStartFloorLiftedWhenUp`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L228) | unit/verify | unproven |
| positive | [`TestRFC5880SlowStartFloorWhileNotUp`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L208) | unit/verify | unproven |

### [`RFC5880-6.8.1-8`](#rfc5880-6.8.1-8)

The last value of Required Min RX Interval received from the remote system in a BFD Control packet. This variable MUST be initialized to 1. (§6.8.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge rejudge 2026-09-30 (bfd engine 2). Unit re-read after TestRFC5880InitVariablesAreNotConstants lost its 6.8.1-4 tag (doc comment only, body unchanged). Judgement unchanged: Forbidden: bfd.RemoteMinRxInterval initialised to anything but 1. TestRFC5880InitVariableDefaults fails on RemoteMinRxInterval != 1 after Init. Negative TestRFC5880InitVariablesAreNotConstants shows the value is replaced by the advertised 700000, so the 1 is an initial value.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880InitVariablesAreNotConstants`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L118) | unit/verify | unproven |
| positive | [`TestRFC5880InitVariableDefaults`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L92) | unit/verify | unproven |

### [`RFC5880-6.8.1-9`](#rfc5880-6.8.1-9)

This variable MUST be initialized to zero. (§6.8.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge rejudge 2026-09-30 (bfd engine 2). Unit re-read after TestRFC5880InitVariablesAreNotConstants lost its 6.8.1-4 tag (doc comment only, body unchanged). Judgement unchanged: Forbidden: bfd.RemoteDemandMode initialised true. TestRFC5880InitVariableDefaults fails on RemoteDemandMode true after Init. Negative TestRFC5880InitVariablesAreNotConstants shows it follows a received D=1, so false is an initial value.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880InitVariablesAreNotConstants`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L121) | unit/verify | unproven |
| positive | [`TestRFC5880InitVariableDefaults`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L94) | unit/verify | unproven |

### [`RFC5880-6.8.1-10`](#rfc5880-6.8.1-10)

This variable MUST be a nonzero integer, and is otherwise outside the scope of this specification. (§6.8.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: bfd.DetectMult zero. Red: TestRFC5880DetectMultZeroRequestSubstituted Inits with DetectMult 0 and Fatalf if DetectMult()==0 or != DefaultDetectMult; positive keeps the configured 3. Init (session/session.go:256) is the only writer of vars.DetectMult.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880DetectMultZeroRequestSubstituted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L259) | unit/verify | unproven |
| positive | [`TestRFC5880DetectMultConfiguredValue`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L248) | unit/verify | unproven |

### [`RFC5880-6.8.1-11`](#rfc5880-6.8.1-11)

This variable MUST be initialized to a random 32-bit value. (§6.8.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5880-6.8.1-11, so no unit is bound to it.

### [`RFC5880-6.8.1-12`](#rfc5880-6.8.1-12)

This variable MUST be initialized to zero. (§6.8.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge 2026-09-28. Positive TestRFC5880NewSessionAuthSeqKnownZero: a new Machine with SetAuth has AuthSeqKnown 0 and accepts a first packet at 0x80000000, which Check would refuse from any initialized floor of 0. Negative TestRFC5880AuthSeqKnownOneDiscardsFirstPacket builds the violating initial state (AuthSeqKnown 1) and shows the same packet is refused, so the observable outcome depends on the zero start; an initialization has no refusal path, and the violating state is the non-conforming input.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880AuthSeqKnownSetAfterFirstPacket`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L445) | unit/verify | revert, verified |
| negative | [`TestRFC5880AuthSeqKnownOneDiscardsFirstPacket`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_auth_seq_test.go#L100) | unit/verify | revert, verified |
| positive | [`TestRFC5880AuthSeqKnownStartsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L427) | unit/verify | revert, verified |
| positive | [`TestRFC5880NewSessionAuthSeqKnownZero`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_auth_seq_test.go#L75) | unit/verify | revert, verified |

### [`RFC5880-6.8.1-13`](#rfc5880-6.8.1-13)

This variable MUST be set to zero after no packets have been received on this session for at least twice the Detection Time. (§6.8.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5880-6.8.1-13, so no unit is bound to it.

### [`RFC5880-6.8.1-14`](#rfc5880-6.8.1-14)

Once session state is created, and at least one BFD Control packet is received from the remote end, it MUST be preserved for at least one Detection Time (see section 6.8.4) subsequent to the receipt of the last BFD Control packet, regardless of the session state. (§6.8.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge rejudge 2026-09-30 (bfd AdminDown). Positive TestRFC5880ReleasedSessionPreservedForOneDetectionTime edited under D-15: the peer keeps sending past AdminDownTransmitEnd so the final removal is the one 6.8.1 sets, entry present one ns before the last deadline and gone at it; retention can now only be longer than 6.8.1 (retireReleasedLocked also waits for the 6.8.16 window), never shorter. Session-level positive TestRFC5880StatePreservedForOneDetectionTime and negative TestRFC5880ReleasedSessionAskedAgainKeepsRemoteState unchanged (revive keeps Machine, discriminator, RemoteDiscr, RemoteMinRx, LastReceived). Records: revert retireReleasedLocked (+) and reviveReleasedLocked (-) observed red.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880ReleasedSessionAskedAgainKeepsRemoteState`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_release_test.go#L111) | unit/verify | revert, verified |
| positive | [`TestRFC5880ReleasedSessionPreservedForOneDetectionTime`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_release_test.go#L30) | unit/verify | revert, verified |
| positive | [`TestRFC5880StatePreservedForOneDetectionTime`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L283) | unit/verify | revert, verified |

### [`RFC5880-6.1-1`](#rfc5880-6.1-1)

A system taking the Active role MUST send BFD Control packets for a particular session, regardless of whether it has received any BFD packets for that session. (§6.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge rejudge 2026-10-02 (tag move under the 2026-09-30 owner ruling, OWNER RULING 6(b)); judged by an agent that authored neither test. Stale only because the RFC5880-6.1-3 negative tag comment was removed from session TestRFC5880PassiveRoleSilentUntilReception; body unchanged. Proof unchanged: + engine TestRFC5880ActiveSessionSendsBeforeAnyReception; - engine TestRFC5880ActiveSessionSendsAfterThePeerFallsSilent, both recorded revert loop.go::tick observed red. The session-level negative stays supplementary.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880ActiveSessionSendsAfterThePeerFallsSilent`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_role_transmit_test.go#L63) | unit/verify | revert, verified |
| negative | [`TestRFC5880PassiveRoleSilentUntilReception`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L391) | unit/verify | revert, verified |
| negative | [`TestRFC5880PassiveRoleTransmitsAfterReception`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L409) | unit/verify | revert, verified |
| positive | [`TestRFC5880ActiveSessionSendsBeforeAnyReception`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_role_transmit_test.go#L52) | unit/verify | revert, verified |
| positive | [`TestRFC5880ActiveRoleTransmitsWithoutReception`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L358) | unit/verify | revert, verified |

### [`RFC5880-6.1-2`](#rfc5880-6.1-2)

A system taking the Passive role MUST NOT begin sending BFD packets for a particular session until it has received a BFD packet for that session, and thus has learned the remote system's discriminator value. (§6.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge rejudge 2026-10-02 (tag move under the 2026-09-30 owner ruling, OWNER RULING 6(b)); judged by an agent that authored neither test. Stale only because the RFC5880-6.1-3 negative tag comment was removed from session TestRFC5880PassiveRoleSilentUntilReception; body unchanged. + engine TestRFC5880PassiveSessionSilentThroughTicksUntilReception; - engine TestRFC5880PassiveSessionSendsOnceThePeerSpoke; both recorded revert loop.go::tick observed red.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880PassiveSessionSendsOnceThePeerSpoke`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_role_transmit_test.go#L88) | unit/verify | revert, verified |
| negative | [`TestRFC5880PassiveRoleTransmitsAfterReception`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L414) | unit/verify | revert, verified |
| positive | [`TestRFC5880PassiveSessionSilentThroughTicksUntilReception`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_role_transmit_test.go#L77) | unit/verify | revert, verified |
| positive | [`TestRFC5880PassiveRoleSilentUntilReception`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L383) | unit/verify | revert, verified |

### [`RFC5880-6.1-3`](#rfc5880-6.1-3)

At least one system MUST take the Active role (possibly both). (§6.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge rejudge 2026-10-02 (tag move under the 2026-09-30 owner ruling, OWNER RULING 6(b)); judged by an agent that authored neither test. Owner ruling 2026-09-30 ('we can not see the peer'): no refusal path exists, so the HEAD negative (TestRFC5880PassiveRoleSilentUntilReception, which proved RFC5880-6.1-1 and 6.1-2, where it stays) was removed and the row carries {single-polarity: positive}. + session TestRFC5880ActiveRoleTransmitsWithoutReception: a request with Passive unset gives RoleActive and arms the TX deadline with RemoteDiscr 0; record revert session.go::Init observed red. Operator default reaches that request (ze-bfd-conf.yang passive default false; single-hop Passive refused per RFC 5881 Section 3). The pair obligation beyond Ze's own role is not observable by Ze.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC5880ActiveRoleTransmitsWithoutReception`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L363) | unit/verify | revert, verified |

### [`RFC5880-6.8.3-1`](#rfc5880-6.8.3-1)

When bfd.SessionState is not Up, the system MUST set bfd.DesiredMinTxInterval to a value of not less than one second (1,000,000 microseconds). (§6.8.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge rejudge 2026-09-30 (bfd session+engine continuation). TestRFC5880SlowStartFloorInEveryStateButUp adds the floor (live variable and Build field) in Init, on Up->Down by detection expiry, by peer Down and by peer AdminDown, with 300 ms in Up between, closing the gap the earlier note named; local AdminDown stays covered by FloorRestoredOnAdminDown, negative LiftedWhenUp. Mutant onStateChange state guard and whole-body revert both observed red after the Poll fix.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880SlowStartFloorLiftedWhenUp`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L232) | unit/verify | revert, verified |
| positive | [`TestRFC5880SlowStartFloorInEveryStateButUp`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_session_clauses_test.go#L103) | unit/verify | mutant, verified |
| positive | [`TestRFC5880SlowStartFloorRestoredOnAdminDown`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L1296) | unit/verify | revert, verified |
| positive | [`TestRFC5880SlowStartFloorWhileNotUp`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L213) | unit/verify | revert, verified |

### [`RFC5880-6.8.3-2`](#rfc5880-6.8.3-2)

If either bfd.DesiredMinTxInterval is changed or bfd.RequiredMinRxInterval is changed, a Poll Sequence MUST be initiated (see section 6.5). (§6.8.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge rejudge 2026-09-30 (bfd session+engine continuation). Each clause alone: TestRFC5880PollInitiatedForEachIntervalAlone (TX alone on the move to Up, RX alone by the echo slow-down) and the Down-entry case now fixed in onStateChange: TestRFC5880PollInitiatedWhenDownEntryRaisesDesiredMinTx sets PollOutstanding when leaving Up raises bfd.DesiredMinTxInterval to 1 s. The Poll is set after the Down-entry reset, so it survives; it rides periodic packets only (6.5-2), a Passive session with no RemoteDiscr sends none, and Build/BuildFinal never set P and F together. Negative NoPollWhenIntervalsUnchanged. Mutants || -> && and the new != -> == observed red. AdminEnable leaves the floor unchanged, so no Poll is owed there.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880NoPollWhenIntervalsUnchanged`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L469) | unit/verify | revert, verified |
| positive | [`TestRFC5880PollInitiatedForEachIntervalAlone`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_session_clauses_test.go#L158) | unit/verify | mutant, verified |
| positive | [`TestRFC5880PollInitiatedWhenDownEntryRaisesDesiredMinTx`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_session_clauses_test.go#L355) | unit/verify | mutant, verified |
| positive | [`TestRFC5880PollInitiatedOnIntervalChange`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L435) | unit/verify | revert, verified |

### [`RFC5880-6.8.3-3`](#rfc5880-6.8.3-3)

If bfd.DesiredMinTxInterval is increased and bfd.SessionState is Up, the actual transmission interval used MUST NOT change until the Poll Sequence described above has terminated. (§6.8.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RA-BFD3 first verdict 2026-09-27 (gap closed by DF-BFD-7: desiredMinTxInForceUs, holdDesiredMinTxLocked, releaseDesiredMinTxLocked in timers.go). The sentence has one condition pair (increase AND Up) and one prohibition (actual interval unchanged until the Poll terminates). Forbidden behaviour: TX at the raised interval while the Poll is outstanding. Both writers of an increase in Up are driven by the positive TestRFC5880RaisedDesiredMinTxHeldUntilPollTerminates: echo slow-down (300 ms to 1 s, RemoteMinRx 100 ms) requires TransmitInterval == 300 ms, NextTxDeadline == last TX + 300 ms after AdvanceTxWithJitter, and still +300 ms after a non-Final packet (the Receive reschedule path); Up transition with a 2 s configured interval requires TransmitInterval == 1 s (remote 300 ms) with the Poll outstanding. Each goes red if TransmitInterval reads the live bfd.DesiredMinTxInterval (1 s, 2 s). Negative TestRFC5880RaisedDesiredMinTxAppliedAtFinal: the terminating Final makes TransmitInterval 1 s and moves the pending deadline to last TX + 1 s, red on a hold that is never released or released without rescheduling; a session leaving Up drops the hold (TransmitInterval 1 s slow-start, not the held 300 ms). The only runtime writers of bfd.DesiredMinTxInterval in Up are ApplyEchoSlowdown and onStateChange, both covered; a decrease (revertEchoSlowdownLocked) is not held, as the sentence covers only an increase.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880RaisedDesiredMinTxAppliedAtFinal`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L1560) | unit/verify | revert, verified |
| positive | [`TestRFC5880RaisedDesiredMinTxHeldUntilPollTerminates`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L1501) | unit/verify | revert, verified |

### [`RFC5880-6.8.3-4`](#rfc5880-6.8.3-4)

If bfd.RequiredMinRxInterval is reduced and bfd.SessionState is Up, the previous value of bfd.RequiredMinRxInterval MUST be used when calculating the Detection Time for the remote system until the Poll Sequence described above has terminated. (§6.8.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5880-6.8.3-4, so no unit is bound to it.

### [`RFC5880-6.8.3-5`](#rfc5880-6.8.3-5)

If the local system reduces its transmit interval due to bfd.RemoteMinRxInterval being reduced (the remote system has advertised a reduced value in Required Min RX Interval), and the remote system is not in Demand mode, the local system MUST honor the new interval immediately. (§6.8.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RA-BFD2 strict re-read 2026-09-27 (stale on Receive: echo reschedule and pollSettling added after rescheduleTxLocked, which is still called on every accepted packet). Forbidden: keeping the old, longer wait after a reduction. TestRFC5880RemoteMinRxChangeReschedulesNextTx requires last TX + 400 ms after 900 to 400 ms, and a jittered 900 of 1.2 s rescaled to 300 of 400 ms, already due. TestRFC5880RemoteMinRxReductionHonoredImmediately pins TransmitInterval at 400 ms at once. Negatives: TestRFC5880UnchangedRemoteMinRxKeepsNextTx (no change, deadline kept) and TestRFC5880TransmitIntervalFlooredByLocalDesired (a reduction below the local desired value does not shorten TX). The Demand-mode condition only removes the obligation; Ze runs no Demand mode, so it always honours the reduction.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880TransmitIntervalFlooredByLocalDesired`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L532) | unit/verify | unproven |
| negative | [`TestRFC5880UnchangedRemoteMinRxKeepsNextTx`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L1415) | unit/verify | revert, verified |
| positive | [`TestRFC5880RemoteMinRxChangeReschedulesNextTx`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L1327) | unit/verify | revert, verified |
| positive | [`TestRFC5880RemoteMinRxReductionHonoredImmediately`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L496) | unit/verify | unproven |

### [`RFC5880-6.8.3-6`](#rfc5880-6.8.3-6)

Therefore, if multiple changes are made that require the use of a Poll Sequence, there are three choices: 1) they MUST be communicated in a single BFD Control packet (so the semantics of the Final reply are clear), or 2) sufficient time must have transpired since the Poll Sequence was completed to disambiguate the situation (at least a round trip time since the last Poll was transmitted) prior to the initiation of another Poll Sequence, or 3) an additional BFD Control packet with the Final (F) bit *clear* MUST be received after the Poll Sequence has completed prior to the initiation of another Poll Sequence (this option is not available when Demand mode is active). (§6.8.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RA-BFD2 strict re-read 2026-09-27, raised from weak: the RA-BFD defect is fixed at the producer. Every Poll initiator is choice 1 or choice 3: onStateChange changes both intervals under one PollOutstanding (and can only fire from Down/Init, where Down entry clears PollOutstanding and pollSettling); ApplyEchoSlowdown and revertEchoSlowdownLocked (while Up) return with the flag unchanged unless pollSequenceIdle, which Receive clears only on the Final that ends the Poll (pollSettling set) followed by a packet with F clear. Forbidden second Poll during an outstanding Poll: TestRFC5880EchoSlowdownWaitsForOutstandingPoll (tx and rx stay 10000 during the Up Poll) and TestRFC5880EchoSlowdownRevertWaitsForOutstandingPoll (stays 1000000). Forbidden Poll after the Final before an F-clear packet: both tests assert no Poll and the old intervals. Positive: TestRFC5880EchoSlowdownPollsAfterSettledPoll (P set, both intervals 1000000 in one packet) and TestRFC5880PollInitiatedOnIntervalChange (single packet). Choice 2 is an alternative Ze does not take; Demand mode is not driven, so choice 3 is always available.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880EchoSlowdownRevertWaitsForOutstandingPoll`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L788) | unit/verify | revert, verified |
| negative | [`TestRFC5880EchoSlowdownWaitsForOutstandingPoll`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L722) | unit/verify | revert, verified |
| positive | [`TestRFC5880EchoSlowdownPollsAfterSettledPoll`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L759) | unit/verify | revert, verified |
| positive | [`TestRFC5880PollInitiatedOnIntervalChange`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L440) | unit/verify | unproven |

### [`RFC5880-6.5-1`](#rfc5880-6.5-1)

A BFD Control packet MUST NOT have both the Poll (P) and Final (F) bits set. (§6.5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: P and F both set. Red: TestRFC5880PollPacketHasNoFinalBit Fatalf on c.Final from Build with PollOutstanding; TestRFC5880FinalReplyClearsPollBit Fatalf on f.Poll from BuildFinal with PollOutstanding true, the state where inheriting P would set both. Build and BuildFinal are the only packets sendLocked is handed (engine/loop.go).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880FinalReplyClearsPollBit`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L585) | unit/verify | unproven |
| positive | [`TestRFC5880PollPacketHasNoFinalBit`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L566) | unit/verify | unproven |

### [`RFC5880-6.5-2`](#rfc5880-6.5-2)

If periodic BFD Control packets are already being sent (the remote system is not in Demand mode), the Poll Sequence MUST be performed by setting the Poll (P) bit on those scheduled periodic transmissions; additional packets MUST NOT be sent. (§6.5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge rejudge 2026-09-30 (bfd session+engine continuation). TestRFC5880PollRidesTheScheduledPacket: a Poll raised in Up leaves NextTxDeadline unchanged and the packet built at that deadline carries P; the engine has no send path keyed on PollOutstanding (only BuildFinal answers a received Poll), so no additional packet exists. Mutant ApplyEchoSlowdown pollSequenceIdle guard observed red. P-bit negative unchanged.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880FinalReplyClearsPollBit`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L589) | unit/verify | revert, verified |
| positive | [`TestRFC5880PollRidesTheScheduledPacket`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_session_clauses_test.go#L326) | unit/verify | mutant, verified |
| positive | [`TestRFC5880PollPacketHasNoFinalBit`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L570) | unit/verify | revert, verified |

### [`RFC5880-6.6-1`](#rfc5880-6.6-1)

A system MUST NOT set the Demand (D) bit unless bfd.DemandMode is 1, bfd.SessionState is Up, and bfd.RemoteSessionState is Up. (§6.8.7)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: D set while DemandMode, local Up or remote Up is false. Red: TestRFC5880DemandBitClearWhenAnyConditionFails Fatalf on Build().Demand with each condition dropped alone and with both states Down; positive TestRFC5880DemandBitSetWhenAllConditionsHold shows D is set when all hold, so the clear cases are not a constant false.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880DemandBitClearWhenAnyConditionFails`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L665) | unit/verify | unproven |
| positive | [`TestRFC5880DemandBitSetWhenAllConditionsHold`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L651) | unit/verify | unproven |

### [`RFC5880-6.6-2`](#rfc5880-6.6-2)

When the transmitted value of the Demand (D) bit is to be changed, the transmitting system MUST initiate a Poll Sequence in conjunction with changing the bit in order to ensure that both systems are aware of the change. (§6.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5880-6.6-2, so no unit is bound to it.

### [`RFC5880-6.6-3`](#rfc5880-6.6-3)

If Demand mode is active on either or both systems, a Poll Sequence MUST be initiated whenever the contents of the next BFD Control packet to be sent would be different than the contents of the previous packet, with the exception of the Poll (P) and Final (F) bits. (§6.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5880-6.6-3, so no unit is bound to it.

### [`RFC5880-6.7-1`](#rfc5880-6.7-1)

Implementations supporting authentication MUST support both types of SHA1 authentication. (§6.7)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge 2026-09-28. Positive: TestRFC5880BothSHA1VariantsSupported round-trips types 4 and 5. Negative: TestRFC5880SHA1TypesEnforcedDistinctly shows each SHA1 type is really implemented: a flipped hash is refused for each, each refuses the other's section, and a repeated sequence is accepted by 4 and refused by 5, so a stub or an aliased Meticulous type goes red. The older negative (types 0/6/200 refused) stays tagged but does not violate this row.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880SHA1TypesEnforcedDistinctly`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_keyed_test.go#L140) | unit/verify | revert, verified |
| negative | [`TestRFC5880UnsupportedAuthTypesRejected`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L70) | unit/verify | revert, verified |
| positive | [`TestRFC5880BothSHA1VariantsSupported`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L51) | unit/verify | revert, verified |

### [`RFC5880-4.2-1`](#rfc5880-4.2-1)

The password is a binary string, and MUST be from 1 to 16 bytes in length. (§4.2, §6.7.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge 2026-09-30 re-stamp: the bfd TestRFC5880SimplePasswordLengthOutOfRangeRefused unit lost only its 6.7.2-1 tag (D-15); body unchanged. Forbidden: a 0-byte or over-16-byte password accepted. Red: auth TestRFC5880SimplePasswordLengthOutOfRangeRefused Fatalf unless NewSigner and NewVerifier return ErrKeyLengthInvalid for 0, 17, 32, 255 bytes; bfd TestRFC5880SimplePasswordLengthOutOfRangeRefused Fatalf when parseAuthConfig accepts 0, 17 or 32 bytes. Positive: 1, 8, 16 bytes encode (auth SectionHeader) and 1, 14, 16 bytes parse (SimplePasswordManagementAcceptsASCIIStrings), both range ends.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`auth/TestRFC5880SimplePasswordLengthOutOfRangeRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L791) | unit/verify | unproven |
| negative | [`bfd/TestRFC5880SimplePasswordLengthOutOfRangeRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5880_test.go#L126) | unit/verify | unproven |
| positive | [`TestRFC5880SimplePasswordSectionHeader`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L636) | unit/verify | unproven |
| positive | [`TestRFC5880SimplePasswordManagementAcceptsASCIIStrings`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5880_test.go#L99) | unit/verify | unproven |

### [`RFC5880-6.7.2-1`](#rfc5880-6.7.2-1)

For interoperability, the management interface by which the password is configured MUST accept ASCII strings (§6.7.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge 2026-09-30. Now driven through the operator's entry: config text parsed against the bfd YANG, the plugin config section, parseSections, resolveProfile, asserting the session request's Auth.Secret byte for byte. Positive: a plain ASCII password for simple-password. Negative (R1b, inputs forced toward the violation): every ASCII byte 0x01-0x7F including quote, backslash, ; { } # and whitespace, plus leading/trailing spaces, in 16-byte chunks. Judge's own break (parseAuthConfig strips one trailing space) reds the negative in all 5 types while the positive stays green; revert records observed red on both. Old direct parseAuthConfig positive kept; the 4.2-1 length refusal no longer carries this row (D-15).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880KeyManagementConfigEntryKeepsEveryASCIICharacter`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5880_key_management_config_test.go#L110) | unit/verify | revert, verified |
| positive | [`TestRFC5880KeyManagementConfigEntryAcceptsASCIIStrings`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5880_key_management_config_test.go#L88) | unit/verify | revert, verified |
| positive | [`TestRFC5880SimplePasswordManagementAcceptsASCIIStrings`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5880_test.go#L94) | unit/verify | revert, verified |

### [`RFC5880-6.7.2-8`](#rfc5880-6.7.2-8)

The Auth Type field MUST be set to 1 (Simple Password). The Auth Len field MUST be set to the proper length (4 to 19 bytes). (§6.7.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge 2026-09-28. Positive TestRFC5880SimplePasswordSectionHeader pins Auth Type 1 and Auth Len password+3 within 4..19. Auth Type clause negative: TestRFC5880SimplePasswordRejectsForeignSectionShape, a receiver refusing a section built against the transmit rule, isolated (no digest). Auth Len clause negative: TestRFC5880SimplePasswordSectionOutsideProperLengthNotBuilt, NewSigner refuses the 0 and 17-byte passwords that would carry Auth Len 3 and 20.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880SimplePasswordSectionOutsideProperLengthNotBuilt`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_keyed_test.go#L281) | unit/verify | revert, verified |
| negative | [`TestRFC5880SimplePasswordRejectsForeignSectionShape`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L657) | unit/verify | revert, verified |
| positive | [`TestRFC5880SimplePasswordSectionHeader`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L631) | unit/verify | revert, verified |

### [`RFC5880-6.7.3-1`](#rfc5880-6.7.3-1)

The Auth Type field MUST be set to 2 (Keyed MD5) or 3 (Meticulous Keyed MD5). The Auth Len field MUST be set to 24. (§6.7.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge 2026-09-29. Both clauses. Positive TestRFC5880KeyedMD5SectionHeader asserts Auth Type 2/3 and Auth Len 24 on the signed wire (observed red with digestSigner.Sign broken). Negative TestRFC5880KeyedMD5OtherAlgorithmFieldsDiscarded, types 2 and 3, hand-builds packets with Auth Type 4/5 or Auth Len 28 and computes the MD5 digest AFTER the alteration, so the digest compare would accept them and only the field checks in digestVerifier.Verify refuse (read at the producer: Control Length is left correct, so the Length check cannot fire first); exact ErrDigestMismatch, fresh SeqState per packet, the unaltered packet accepted. The Auth Type half has an observed-red mutant record; the Auth Len half is isolated by construction but its mutants timed out, so it has no record. The older confounded tags (RejectsForeignSectionShape, OversizedKeyNotSigned) are not relied on.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880KeyedMD5OversizedKeyNotSigned`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_keyed_test.go#L295) | unit/verify | revert, verified |
| negative | [`TestRFC5880KeyedMD5OtherAlgorithmFieldsDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_section_fields_test.go#L96) | unit/verify | mutant, verified |
| negative | [`TestRFC5880KeyedMD5RejectsForeignSectionShape`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L113) | unit/verify | revert, verified |
| positive | [`TestRFC5880KeyedMD5SectionHeader`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L86) | unit/verify | revert, verified |

### [`RFC5880-6.7.3-2`](#rfc5880-6.7.3-2)

An MD5 digest MUST be calculated over the entire BFD Control packet. (§6.7.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge 2026-09-28. Positive TestRFC5880DigestSpansEntirePacket compares Sign's digest with crypto/md5 computed independently over bytes 0..Length with the padded key, and accepts a hand-built packet. Negative TestRFC5880DigestOverPartialPacketDiscarded refuses digests over 24, off+4, off+8 and Length-1 bytes and every single-byte flip of the mandatory section, Reserved byte and Sequence Number (fresh SeqState, so D-14 seeding precedes and the window cannot refuse first). Types 2 and 3.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880DigestOverPartialPacketDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_keyed_test.go#L541) | unit/verify | revert, verified |
| negative | [`TestRFC5880DigestRejectsMandatorySectionTamper`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L201) | unit/verify | revert, verified |
| positive | [`TestRFC5880DigestSpansEntirePacket`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_keyed_test.go#L511) | unit/verify | revert, verified |
| positive | [`TestRFC5880DigestCoversWholePacket`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L182) | unit/verify | revert, verified |

### [`RFC5880-6.7.3-3`](#rfc5880-6.7.3-3)

replacing the secret key, which MUST NOT be carried in the packet (§6.7.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. single-polarity positive: neither the padded nor the raw secret appears in the emitted bytes.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC5880SecretKeyNotCarriedInPacket`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L225) | unit/verify | unproven |

### [`RFC5880-6.7.3-4`](#rfc5880-6.7.3-4)

For Meticulous Keyed MD5, bfd.XmitAuthSeq MUST be incremented in a circular fashion (when treated as an unsigned 32-bit value). (§6.7.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge 2026-09-29. Both clauses. Positive TestRFC5880MeticulousXmitAuthSeqWrapsAt32Bits puts 0xFFFFFFFE, 0xFFFFFFFF, 0, 1 on the wire through Machine.Sign/AdvanceAuthSeq (observed red with AdvanceAuthSeq broken), so a saturating or non-incrementing XmitAuthSeq goes red. Negative TestRFC5880MeticulousNonCircularRepeatAtWrapDiscarded (type 3) signs 0xFFFFFFFF twice through Machine.Sign, the exact output of a non-circular transmitter, and the peer refuses the repeat with ErrSequenceOutsideWindow, then accepts the circular successor 0 in the same state (observed red with the meticulous.go equality check mutated). The held incremented-clause negative stays.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880MeticulousRejectsUnincrementedSequence`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L583) | unit/verify | revert, verified |
| negative | [`TestRFC5880MeticulousNonCircularRepeatAtWrapDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_auth_wrap_test.go#L28) | unit/verify | mutant, verified |
| positive | [`TestRFC5880MeticulousSequenceIncrementsPerPacket`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L287) | unit/verify | revert, verified |
| positive | [`TestRFC5880MeticulousXmitAuthSeqWrapsAt32Bits`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_auth_seq_test.go#L48) | unit/verify | revert, verified |

### [`RFC5880-6.7.3-13`](#rfc5880-6.7.3-13)

The authentication key value is a binary string of up to 16 bytes, and MUST be placed into the Auth Key/Digest field, padded with trailing zero bytes as necessary. (§6.7.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Positive: TestRFC5880ShortKeyZeroPaddedIntoDigestField computes MD5 independently over the signed packet with the Auth Key/Digest field replaced by the 11-byte key and five zero bytes and requires Sign's transmitted digest to equal it; TestKeyedSecretLongerThanSlotRefused (NewSigner, NewVerifier) and TestParseAuthConfigKeyedSecretLength (parseAuthConfig) accept a 16-byte key for both MD5 types. Negative: the same two refuse a 17-byte key, NewSigner and NewVerifier with ErrKeyLengthInvalid, so no key longer than the field is truncated into it.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestKeyedSecretLongerThanSlotRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L935) | unit/verify | revert, verified |
| negative | [`TestParseAuthConfigKeyedSecretLength`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5881_test.go#L139) | unit/verify | revert, verified |
| positive | [`TestKeyedSecretLongerThanSlotRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L932) | unit/verify | revert, verified |
| positive | [`TestRFC5880ShortKeyZeroPaddedIntoDigestField`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L978) | unit/verify | revert, verified |
| positive | [`TestParseAuthConfigKeyedSecretLength`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5881_test.go#L136) | unit/verify | revert, verified |

### [`RFC5880-6.7.4-1`](#rfc5880-6.7.4-1)

The Auth Type field MUST be set to 4 (Keyed SHA1) or 5 (Meticulous Keyed SHA1).  The Auth Len field MUST be set to 28. (§6.7.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge 2026-09-29. Both clauses. Positive TestRFC5880KeyedSHA1SectionHeader asserts Auth Type 4/5 and Auth Len 28 (observed red with digestSigner.Sign broken). Negative TestRFC5880KeyedSHA1OtherAlgorithmFieldsDiscarded, types 4 and 5, hashes Auth Type 2/3 or Auth Len 24 in after the alteration, exact ErrDigestMismatch, unaltered accepted; isolated as for 6.7.3-1 (Control Length correct, digest valid). Auth Type half recorded by mutant; Auth Len half isolated by construction, no record. Older confounded tags not relied on.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880KeyedSHA1OversizedKeyNotSigned`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_keyed_test.go#L310) | unit/verify | revert, verified |
| negative | [`TestRFC5880KeyedSHA1OtherAlgorithmFieldsDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_section_fields_test.go#L107) | unit/verify | mutant, verified |
| negative | [`TestRFC5880KeyedSHA1RejectsForeignSectionShape`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L159) | unit/verify | revert, verified |
| positive | [`TestRFC5880KeyedSHA1SectionHeader`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L138) | unit/verify | revert, verified |

### [`RFC5880-6.7.4-2`](#rfc5880-6.7.4-2)

A SHA1 hash MUST be calculated over the entire BFD control packet. (§6.7.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge 2026-09-28. Same units as RFC5880-6.7.3-2 for types 4 and 5: Sign's hash equals crypto/sha1 over the entire packet computed independently, and hashes over 24, off+4, off+8, Length-1 bytes plus every single-byte flip of the mandatory section, Reserved byte and Sequence Number are refused with ErrDigestMismatch.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880DigestOverPartialPacketDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_keyed_test.go#L547) | unit/verify | revert, verified |
| negative | [`TestRFC5880DigestRejectsMandatorySectionTamper`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L206) | unit/verify | revert, verified |
| positive | [`TestRFC5880DigestSpansEntirePacket`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_keyed_test.go#L515) | unit/verify | revert, verified |
| positive | [`TestRFC5880DigestCoversWholePacket`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L188) | unit/verify | revert, verified |

### [`RFC5880-6.7.4-3`](#rfc5880-6.7.4-3)

the secret key, which MUST NOT be carried in the packet (§6.7.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRFC5880SecretKeyNotCarriedInPacket asserts neither the padded nor the raw secret appears in the emitted SHA1 packet (single-polarity).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC5880SecretKeyNotCarriedInPacket`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L229) | unit/verify | unproven |

### [`RFC5880-6.7.4-4`](#rfc5880-6.7.4-4)

For Meticulous Keyed SHA1, bfd.XmitAuthSeq MUST be incremented in a circular fashion (when treated as an unsigned 32-bit value). (§6.7.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge 2026-09-29. Same units as 6.7.3-4 over type 5: the positive wrap test runs type 5 through Machine.Sign, and the negative loop runs type 5 after type 3 through the same Verify/Check path, repeat refused with ErrSequenceOutsideWindow and 0 accepted. The recorded red fired on the type-3 iteration; the type-5 iteration asserts the same outcome over the SHA1 verifier.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880MeticulousRejectsUnincrementedSequence`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L588) | unit/verify | revert, verified |
| negative | [`TestRFC5880MeticulousNonCircularRepeatAtWrapDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_auth_wrap_test.go#L33) | unit/verify | mutant, verified |
| positive | [`TestRFC5880MeticulousSequenceIncrementsPerPacket`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L293) | unit/verify | revert, verified |
| positive | [`TestRFC5880MeticulousXmitAuthSeqWrapsAt32Bits`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_auth_seq_test.go#L52) | unit/verify | revert, verified |

### [`RFC5880-6.7.4-5`](#rfc5880-6.7.4-5)

the management interface by which the key is configured MUST accept ASCII strings (§6.7.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge 2026-09-30. Same units as 6.7.3-7 over keyed-sha1 and meticulous-keyed-sha1 (keys up to 20 bytes, the sweep chunked at 20); judge's trailing-space-strip break reds both SHA1 negative subtests. Revert records observed red. Incomplete-block negative untagged (D-15).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880KeyManagementConfigEntryKeepsEveryASCIICharacter`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5880_key_management_config_test.go#L119) | unit/verify | revert, verified |
| positive | [`TestRFC5880KeyManagementConfigEntryAcceptsASCIIStrings`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5880_key_management_config_test.go#L95) | unit/verify | revert, verified |
| positive | [`TestRFC5880KeyManagementAcceptsASCIIStrings`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5880_test.go#L31) | unit/verify | revert, verified |

### [`RFC5880-6.7.4-7`](#rfc5880-6.7.4-7)

The authentication key value is a binary string of up to 20 bytes, and MUST be placed into the Auth Key/Hash field, padding with trailing zero bytes as necessary. (§6.7.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Positive: TestRFC5880ShortKeyZeroPaddedIntoDigestField computes SHA1 independently over the signed packet with the Auth Key/Hash field replaced by the 11-byte key and nine zero bytes and requires Sign's transmitted hash to equal it; TestKeyedSecretLongerThanSlotRefused and TestParseAuthConfigKeyedSecretLength accept a 20-byte key for both SHA1 types. Negative: the same two refuse a 21-byte key.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestKeyedSecretLongerThanSlotRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L941) | unit/verify | revert, verified |
| negative | [`TestParseAuthConfigKeyedSecretLength`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5881_test.go#L143) | unit/verify | revert, verified |
| positive | [`TestKeyedSecretLongerThanSlotRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L938) | unit/verify | revert, verified |
| positive | [`TestRFC5880ShortKeyZeroPaddedIntoDigestField`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L984) | unit/verify | revert, verified |
| positive | [`TestParseAuthConfigKeyedSecretLength`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5881_test.go#L141) | unit/verify | revert, verified |

### [`RFC5880-6.7.3-15`](#rfc5880-6.7.3-15)

The Auth Key ID field MUST be set to the ID of the current authentication key. (§6.7.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge 2026-09-30 (phase 8). Positive TestRFC5880MD5AuthKeyIDIsTheCurrentKey: both MD5 types, key ids 0/1/200/255 read back at MandatoryLen+2 and verified. Negative (R1b, forced input) TestRFC5880MD5AuthKeyIDOverwritesStaleBuffer: section pre-filled 0xFF, Sign must leave key 7. Judge break digestSigner.Sign writing buf[off+2] unchanged reds both (and the positive at ids 1..255).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880MD5AuthKeyIDOverwritesStaleBuffer`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_md5_split_test.go#L48) | unit/verify | revert, verified |
| positive | [`TestRFC5880MD5AuthKeyIDIsTheCurrentKey`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_md5_split_test.go#L28) | unit/verify | revert, verified |

### [`RFC5880-6.7.4-10`](#rfc5880-6.7.4-10)

The Auth Key ID field MUST be set to the ID of the current authentication key. (§6.7.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge 2026-09-30 (phase 8). Positive TestRFC5880AuthKeyIDIsTheConfiguredKey (Keyed SHA1, ids 0/1/200/255 on the wire); judge break of the key-id write in digestSigner.Sign reds it. Negative TestRFC5880AuthKeyIDMismatchDiscarded is the receive-side handling of a peer PDU with a key id other than the configured one (owner ruling 2 reading of a generate row); judge break removing the key-id check in digestVerifier.Verify reds it through its replay-floor assertion, so it is isolated from the digest.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880AuthKeyIDMismatchDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L267) | unit/verify | revert, verified |
| positive | [`TestRFC5880AuthKeyIDIsTheConfiguredKey`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L249) | unit/verify | revert, verified |

### [`RFC5880-6.7.4-11`](#rfc5880-6.7.4-11)

The Sequence Number field MUST be set to bfd.XmitAuthSeq. (§6.7.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge 2026-09-30 (phase 8). Keyed SHA1 via session Machine.Sign: positive TestRFC5880AuthSequenceFieldIsXmitAuthSeq pins 0x11223344 in the field; negative (forced input) TestRFC5880AuthSequenceFieldFollowsAdvance requires the field to follow AdvanceAuthSeq to first+1, so a constant or ignored seq fails; revert records observed red. Limit: neither unit drives the variable above 0x7FFFFFFF; the full-range unit TestRFC5880SequenceFieldIsXmitAuthSeqAcrossTheCounterRange loops the SHA1 types too but is tagged on 6.7.3-8 only (shared digestSigner.Sign producer).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880AuthSequenceFieldFollowsAdvance`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L1224) | unit/verify | revert, verified |
| positive | [`TestRFC5880AuthSequenceFieldIsXmitAuthSeq`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L1200) | unit/verify | revert, verified |

### [`RFC5880-6.8.6-1`](#rfc5880-6.8.6-1)

If the version number is not correct (1), the packet MUST be discarded. (§6.8.6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: accepting a Version other than 1. TestRFC5880VersionNotOneDiscarded fails unless ParseControl returns exactly ErrBadVersion for 0, 2, 3 and 7 (version is the first check, so no other rule trips); handleInbound returns on any ParseControl error. Positive TestRFC5880VersionOneAccepted: version 1 parses with no error.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880VersionNotOneDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/packet/rfc5880_test.go#L66) | unit/verify | unproven |
| positive | [`TestRFC5880VersionOneAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/packet/rfc5880_test.go#L48) | unit/verify | unproven |

### [`RFC5880-6.8.6-2`](#rfc5880-6.8.6-2)

If the Length field is less than the minimum correct value (24 if the A bit is clear, or 26 if the A bit is set), the packet MUST be discarded. (§6.8.6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge 2026-09-30 (rejudge, 6-2 + rest). Section 6.8.6: Length below 24 (A=0) or 26 (A=1) MUST be discarded. Both floors now proven from both sides at ParseControl: + TestRFC5880LengthMinimumAccepted (24 A=0, 26 A=1); - TestRFC5880LengthBelowMinimumDiscarded (23, 24 A=1) and NEW TestRFC5880AuthenticatedLengthOneBelowMinimumDiscarded (A=1 Length 25 -> ErrLengthTooSmall), which closes the 25-byte gap: a targeted overlay setting the A=1 floor to MandatoryLen+1 reds only the new unit (job-ov6). Record: revert control.go::ParseControl observed red.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880AuthenticatedLengthOneBelowMinimumDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/packet/rfc5880_length_boundary_test.go#L8) | unit/verify | revert, verified |
| negative | [`TestRFC5880LengthBelowMinimumDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/packet/rfc5880_test.go#L128) | unit/verify | revert, verified |
| positive | [`TestRFC5880LengthMinimumAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/packet/rfc5880_test.go#L108) | unit/verify | revert, verified |

### [`RFC5880-6.8.6-3`](#rfc5880-6.8.6-3)

If the Length field is greater than the payload of the encapsulating protocol, the packet MUST be discarded. (§6.8.6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: accepting Length > payload. TestRFC5880LengthOverPayloadDiscarded fails unless ParseControl returns ErrLengthOverBuffer for Length = payload + 8; TestRFC5880LengthEqualsPayloadAccepted fails if Length == payload is rejected, which catches a >= mutation. handleInbound returns on the error.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880LengthOverPayloadDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/packet/rfc5880_test.go#L163) | unit/verify | unproven |
| positive | [`TestRFC5880LengthEqualsPayloadAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/packet/rfc5880_test.go#L148) | unit/verify | unproven |

### [`RFC5880-6.8.6-4`](#rfc5880-6.8.6-4)

If the Detect Mult field is zero, the packet MUST be discarded. (§6.8.6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: accepting Detect Mult 0. TestRFC5880DetectMultZeroDiscarded fails unless ParseControl returns ErrZeroDetectMult; TestRFC5880DetectMultNonZeroAccepted fails if 1, 3 or 255 is rejected or mis-decoded. handleInbound returns on the error.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880DetectMultZeroDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/packet/rfc5880_test.go#L190) | unit/verify | unproven |
| positive | [`TestRFC5880DetectMultNonZeroAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/packet/rfc5880_test.go#L174) | unit/verify | unproven |

### [`RFC5880-6.8.6-5`](#rfc5880-6.8.6-5)

If the Multipoint (M) bit is nonzero, the packet MUST be discarded. (§6.8.6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: accepting M=1. TestRFC5880MultipointSetDiscarded fails unless ParseControl returns ErrMultipointSet; TestRFC5880MultipointZeroAccepted fails if an M=0 packet is rejected. handleInbound returns on the error.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880MultipointSetDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/packet/rfc5880_test.go#L99) | unit/verify | unproven |
| positive | [`TestRFC5880MultipointZeroAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/packet/rfc5880_test.go#L84) | unit/verify | unproven |

### [`RFC5880-6.8.6-6`](#rfc5880-6.8.6-6)

If the My Discriminator field is zero, the packet MUST be discarded. (§6.8.6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: accepting My Discriminator 0. TestRFC5880MyDiscriminatorZeroDiscarded fails unless ParseControl returns ErrZeroMyDisc; TestRFC5880MyDiscriminatorNonZeroAccepted fails if a nonzero value is rejected or mis-decoded. handleInbound returns on the error.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880MyDiscriminatorZeroDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/packet/rfc5880_test.go#L214) | unit/verify | unproven |
| positive | [`TestRFC5880MyDiscriminatorNonZeroAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/packet/rfc5880_test.go#L200) | unit/verify | unproven |

### [`RFC5880-6.8.6-7`](#rfc5880-6.8.6-7)

If the Your Discriminator field is nonzero, it MUST be used to select the session with which this BFD packet is associated.  If no session is found, the packet MUST be discarded. (§6.8.6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge 2026-09-30 (rejudge, 6-2 + rest). Selection clause: NEW TestRFC5880YourDiscriminatorSelectsAmongSessions holds two sessions to one peer on eth0/eth1; a packet arriving on eth0 naming eth1's discriminator reaches eth1's session only (first RemoteDiscr stays 0) and the mirror reaches the first, so selection by arrival tuple goes red (overlay selecting by byKey when YD != 0 reds it and the discard negative, job-ov8). Discard clause: - TestRFC5880UnknownYourDiscriminatorDiscarded unchanged and sound. Old one-session + kept as supplementary. Record: revert loop.go::handleInbound observed red.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880UnknownYourDiscriminatorDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L140) | unit/verify | revert, verified |
| positive | [`TestRFC5880YourDiscriminatorSelectsAmongSessions`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_discriminator_select_test.go#L9) | unit/verify | revert, verified |
| positive | [`TestRFC5880YourDiscriminatorSelectsSession`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L124) | unit/verify | revert, verified |

### [`RFC5880-6.8.6-8`](#rfc5880-6.8.6-8)

If the Your Discriminator field is zero and the State field is not Down or AdminDown, the packet MUST be discarded. (§6.8.6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RA-BFD2 strict re-read 2026-09-27 (stale on Receive: additions sit after the discard test, which is still the first statement). Both State values the sentence names are driven: TestRFC5880ZeroYourDiscriminatorDiscardedWhenLive sends Your Discriminator 0 with State Init and Up to local Down, Init and Up, and requires ErrYourDiscriminatorReset, unchanged state and unchanged RemoteDiscr (a guard moved below the field updates goes red on RemoteDiscr). Positive TestRFC5880ZeroYourDiscriminatorAcceptedWhenDown accepts State Down and AdminDown in Down, AdminDown and Up. Local AdminDown is not in the negative loop; the sentence conditions only on the packet's State.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880ZeroYourDiscriminatorDiscardedWhenLive`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L698) | unit/verify | revert, verified |
| positive | [`TestRFC5880ZeroYourDiscriminatorAcceptedWhenDown`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L726) | unit/verify | revert, verified |

### [`RFC5880-6.8.6-9`](#rfc5880-6.8.6-9)

If the A bit is set and no authentication is in use (bfd.AuthType is zero), the packet MUST be discarded. (§6.8.6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: processing A=1 at an AuthType-0 session. TestRFC5880UnauthenticatedSessionDiscardsAuthenticatedPacket fails unless Receive returns ErrAuthMismatch and the state stays Down; the engine hands such a packet to Receive because HasAuth is false. Positive TestRFC5880UnauthenticatedSessionAcceptsUnauthenticatedPacket: A=0 accepted.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880UnauthenticatedSessionDiscardsAuthenticatedPacket`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L779) | unit/verify | unproven |
| positive | [`TestRFC5880UnauthenticatedSessionAcceptsUnauthenticatedPacket`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L764) | unit/verify | unproven |

### [`RFC5880-6.8.6-10`](#rfc5880-6.8.6-10)

If the A bit is clear and authentication is in use (bfd.AuthType is nonzero), the packet MUST be discarded. (§6.8.6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: processing A=0 at an authenticated session. TestRFC5880AuthenticatedSessionDiscardsUnauthenticatedPacket fails unless Receive returns ErrAuthMismatch with the state unchanged; the engine skips Verify for A=0 and relies on this check. Positive TestRFC5880AuthenticatedSessionAcceptsAuthenticatedPacket: A=1 accepted.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880AuthenticatedSessionDiscardsUnauthenticatedPacket`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L814) | unit/verify | unproven |
| positive | [`TestRFC5880AuthenticatedSessionAcceptsAuthenticatedPacket`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L796) | unit/verify | unproven |

### [`RFC5880-6.8.6-11`](#rfc5880-6.8.6-11)

If the A bit is set, the packet MUST be authenticated under the rules of section 6.7, based on the authentication type in use (bfd.AuthType). (§6.8.6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge 2026-09-30. Positive TestRFC5880AuthenticatedUnderEachSessionAuthType: for each of the 5 Auth Types a session configured with it delivers a packet signed under it through handleInbound (RemoteDiscr learned), so a verifier fixed on one type fails. Negative TestRFC5880AuthenticatedPacketOfAnotherAuthTypeDiscarded: every configured type x every other type, same key id and secret, correctly signed, is discarded (RemoteDiscr 0); the Keyed-vs-Meticulous pairs of one hash isolate the Auth Type byte (same length and digest). Judge's own break (sha1.go Verify accepts any nonzero Auth Type) reds exactly those 4 pairs while the positive and both old Keyed SHA1 units stay green. Old tamper/wrong-key units kept.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880AuthenticatedPacketOfAnotherAuthTypeDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_auth_type_test.go#L67) | unit/verify | revert, verified |
| negative | [`TestRFC5880UnauthenticPacketDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L258) | unit/verify | revert, verified |
| positive | [`TestRFC5880AuthenticatedUnderEachSessionAuthType`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_auth_type_test.go#L45) | unit/verify | revert, verified |
| positive | [`TestRFC5880AuthenticatedPacketVerifiedAndDelivered`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L241) | unit/verify | revert, verified |

### [`RFC5880-6.8.6-12`](#rfc5880-6.8.6-12)

If the Required Min Echo RX Interval field is zero, the transmission of Echo packets, if any, MUST cease. (§6.8.6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RA-BFD strict re-read 2026-09-27. One clause: a received Required Min Echo RX of zero ends echo transmission. Forbidden: an echo sent after that packet. Red: TestRFC5880EchoCeasesWhenPeerAdvertisesZero Fatal when EchoEnabled stays true, when PrimeEcho leaves the already-armed deadline set, or when EchoInterval is nonzero. The engine sends only from an armed deadline after its own EchoEnabled gate (echoTickLocked, echo.go), and PrimeEcho clears the deadline when echo is off, so removing either gate alone sends nothing and removing PrimeEcho's check reddens this unit. Contrast: TestRFC5880EchoEnabledWhenPeerAdvertisesNonZero keeps echo enabled and armed on a nonzero value.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880EchoEnabledWhenPeerAdvertisesNonZero`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L867) | unit/verify | unproven |
| positive | [`TestRFC5880EchoCeasesWhenPeerAdvertisesZero`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L830) | unit/verify | unproven |

### [`RFC5880-6.8.6-13`](#rfc5880-6.8.6-13)

If a Poll Sequence is being transmitted by the local system and the Final (F) bit in the received packet is set, the Poll Sequence MUST be terminated. (§6.8.6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a Poll that survives F=1. TestRFC5880FinalTerminatesPoll fails if PollOutstanding stays set after F=1; TestRFC5880NonFinalDoesNotTerminatePoll fails if F=0 clears it, so a clear-on-any-packet mutation goes red.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880NonFinalDoesNotTerminatePoll`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L629) | unit/verify | unproven |
| positive | [`TestRFC5880FinalTerminatesPoll`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L609) | unit/verify | unproven |

### [`RFC5880-6.8.6-14`](#rfc5880-6.8.6-14)

If bfd.RemoteDemandMode is 1, bfd.SessionState is Up, and bfd.RemoteSessionState is Up, Demand mode is active on the remote system and the local system MUST cease the periodic transmission of BFD Control packets (see section 6.8.7). (§6.8.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5880-6.8.6-14, so no unit is bound to it.

### [`RFC5880-6.8.6-15`](#rfc5880-6.8.6-15)

If bfd.RemoteDemandMode is 0, or bfd.SessionState is not Up, or bfd.RemoteSessionState is not Up, Demand mode is not active on the remote system and the local system MUST send periodic BFD Control packets (see section 6.8.7). (§6.8.6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge rejudge 2026-10-02 (tag move under the 2026-09-30 owner ruling, OWNER RULING 6(b)); judged by an agent that authored neither test. Owner ruling 2026-09-30: no genuine negative exists, so the HEAD negative on TestRFC5880NoPeriodicTransmitWhileAdminDown (which proves the RFC5880-6.8.16-3 window, where it stays) was removed and the row carries {single-polarity: positive}. Judged on the positives: engine/rfc5880_demand_disjunct_test.go, one positive per disjunct with the other two false (ClearBothUp, SetLocalNotUpRemoteUp, SetLocalUpRemoteNotUp), each with a revert record on loop.go::tick observed red; rfc5880_demand_periodic_test.go and TestRFC5880PeriodicTransmitWhenRemoteDemandInactive supplementary. Ze never reads bfd.RemoteDemandMode, so it never enters the suppression the complement permits.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC5880PeriodicTransmitDemandClearBothUp`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_demand_disjunct_test.go#L73) | unit/verify | revert, verified |
| positive | [`TestRFC5880PeriodicTransmitDemandSetLocalNotUpRemoteUp`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_demand_disjunct_test.go#L90) | unit/verify | revert, verified |
| positive | [`TestRFC5880PeriodicTransmitDemandSetLocalUpRemoteNotUp`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_demand_disjunct_test.go#L107) | unit/verify | revert, verified |
| positive | [`TestRFC5880PeriodicTransmitWhileDemandBitSetAndNotBothUp`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_demand_periodic_test.go#L9) | unit/verify | revert, verified |
| positive | [`TestRFC5880PeriodicTransmitWhenRemoteDemandInactive`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L368) | unit/verify | revert, verified |

### [`RFC5880-6.8.6-16`](#rfc5880-6.8.6-16)

If a BFD Control packet is received with the Poll (P) bit set to 1, the receiving system MUST transmit a BFD Control packet with the Poll (P) bit clear and the Final (F) bit set as soon as practicable (§6.8.7)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: no immediate F=1/P=0 reply to P=1. TestRFC5880PollAnsweredWithImmediateFinal fails if handleInbound sends nothing, or the reply lacks F or sets P; TestRFC5880NonPollProducesNoImmediateReply fails if a P=0 packet triggers a reply.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880NonPollProducesNoImmediateReply`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L352) | unit/verify | unproven |
| positive | [`TestRFC5880PollAnsweredWithImmediateFinal`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L322) | unit/verify | unproven |

### [`RFC5880-6.8.6-18`](#rfc5880-6.8.6-18)

If the Your Discriminator field is zero, the session MUST be selected based on some combination of other fields, possibly including source addressing information, the My Discriminator field, and the interface over which the packet was received. (§6.8.6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: selecting a zero-discriminator packet on nothing, or on fields the receiver cannot observe. TestRFC5881FirstPacketMatchesByTuple fails if the matching tuple is not delivered; TestRFC5881FirstPacketWrongSourceDropped fails if a wrong source is delivered; TestFirstPacketMatchesWhatTheTransportSurfaces fails if another interface or the wildcard local address selects the session.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5881FirstPacketWrongSourceDropped`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5881_test.go#L158) | unit/verify | unproven |
| positive | [`TestFirstPacketMatchesWhatTheTransportSurfaces`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5881_test.go#L363) | unit/verify | revert, verified |
| positive | [`TestRFC5881FirstPacketMatchesByTuple`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5881_test.go#L135) | unit/verify | unproven |

### [`RFC5880-6.8.7-1`](#rfc5880-6.8.7-1)

With the exceptions listed in the remainder of this section, a system MUST NOT transmit BFD Control packets at an interval less than the larger of bfd.DesiredMinTxInterval and bfd.RemoteMinRxInterval, less applied jitter (see below). (§6.8.7)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge rejudge 2026-09-30 (bfd engine 2). Engine TestRFC5880TickSpacesControlPacketsByLargerInterval ticks every 1 ms for 20 s with the peer Required Min RX 2 s above the 1 s floor and asserts every sent gap lies in [1.5 s, 2 s + 1 ms]. Targeted breaks observed red: TransmitInterval using DesiredMinTx alone (765 ms gap), tick ignoring the deadline (1 ms gap). Session units prove the larger-of in both orders and the jitter bound.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880TransmitDeadlineClampsBadJitter`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L1063) | unit/verify | revert, verified |
| positive | [`TestRFC5880TickSpacesControlPacketsByLargerInterval`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_engine_clauses_test.go#L37) | unit/verify | revert, verified |
| positive | [`TestRFC5880TransmitDeadlineUsesLargerOfBothIntervals`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_session_clauses_test.go#L272) | unit/verify | mutant, verified |
| positive | [`TestRFC5880TransmitDeadlineUsesNegotiatedInterval`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L1033) | unit/verify | revert, verified |

### [`RFC5880-6.8.7-2`](#rfc5880-6.8.7-2)

The periodic transmission of BFD Control packets MUST be jittered on a per-packet basis by up to 25%, that is, the interval MUST be reduced by a random value of 0 to 25% (§6.8.7)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge 2026-09-30 (rejudge, 6-2 + rest). The audit gap (tick never driven) is closed: NEW TestRFC5880PeriodicTransmitJitteredPerPacket drives Loop.tick through a stepped clock and a stamping transport for 200 sends at DetectMult 3; every gap between transmitted packets lies in (75%,100%] of TransmitInterval, >= 50 distinct gaps, and one reduced by more than 5%, so tick passing no reduction or a reused one goes red (overlay 'reduction * 0' reds both tick tests and no old one, job-ov10a; record revert loop.go::tick observed red). applyJitter + and band - kept.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880JitterStaysWithinBand`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L483) | unit/verify | revert, verified |
| positive | [`TestRFC5880PeriodicTransmitJitteredPerPacket`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_jitter_tick_test.go#L64) | unit/verify | revert, verified |
| positive | [`TestRFC5880JitterIsAppliedPerPacket`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L455) | unit/verify | revert, verified |

### [`RFC5880-6.8.7-3`](#rfc5880-6.8.7-3)

If bfd.DetectMult is equal to 1, the interval between transmitted BFD Control packets MUST be no more than 90% of the negotiated transmission interval, and MUST be no less than 75% of the negotiated transmission interval. (§6.8.7)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge 2026-09-30 (rejudge, 6-2 + rest). NEW TestRFC5880PeriodicTransmitDetectMultOneWindow drives tick for 200 sends at DetectMult 1 and bounds every transmitted interval to [75%,90%] of TransmitInterval; overlay 'applyJitter(base, 3)' in tick reds ONLY this unit (job-ov10b); record revert loop.go::tick observed red. applyJitter-level + (DetectMultOneWindow) and - (floor only for DetectMult 1) kept.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880JitterFloorOnlyForDetectMultOne`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L521) | unit/verify | revert, verified |
| positive | [`TestRFC5880PeriodicTransmitDetectMultOneWindow`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_jitter_tick_test.go#L100) | unit/verify | revert, verified |
| positive | [`TestRFC5880JitterDetectMultOneWindow`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L503) | unit/verify | revert, verified |

### [`RFC5880-6.8.7-4`](#rfc5880-6.8.7-4)

The transmit interval MUST be recalculated whenever bfd.DesiredMinTxInterval changes, or whenever bfd.RemoteMinRxInterval changes, and is equal to the greater of those two values. (§6.8.7)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RA-BFD3 strict re-read 2026-09-27 after DF-BFD-7 moved settlePoll before the 1 s precondition in TestRFC5880DesiredMinTxChangeReschedulesNextTx (the old order asserted the Section 6.8.3 defect; no assertion removed). Clause 1, recalculated when bfd.DesiredMinTxInterval changes: forbidden is a pending TX still drawn from the old interval. Decrease: after the revert (1 s to 300 ms, RemoteMinRx 100 ms) the final Fatalf requires NextTxDeadline == last TX + 300 ms, red without the rescheduleTxLocked call in revertEchoSlowdownLocked (deadline stays at +1 s). Increase: the precondition Fatalf requires last TX + 1 s once the slow-down Poll has settled, red if TransmitInterval ignored the raised variable. Clause 2, recalculated when bfd.RemoteMinRxInterval changes: TestRFC5880RemoteMinRxChangeReschedulesNextTx requires the pending deadline at +400 ms after 900 to 400 and at +1.2 s after 400 to 1.2 s, red without the Receive reschedule. Clause 3, equal to the greater: 900 remote over 300 local (RemoteMinRxReductionHonoredImmediately), 1.2 s remote over 300 local, 300 local over 1 us remote (TransmitIntervalFlooredByLocalDesired, which is the negative: a remote value below the local target must not shorten TX, and RequiredMinRx alone must not move it). An increase of DesiredMinTx while Up is held until its Poll ends by Section 6.8.3, judged under RFC5880-6.8.3-3.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880TransmitIntervalFlooredByLocalDesired`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L537) | unit/verify | unproven |
| positive | [`TestRFC5880DesiredMinTxChangeReschedulesNextTx`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L1380) | unit/verify | revert, verified |
| positive | [`TestRFC5880RemoteMinRxChangeReschedulesNextTx`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L1333) | unit/verify | revert, verified |
| positive | [`TestRFC5880RemoteMinRxReductionHonoredImmediately`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L503) | unit/verify | unproven |

### [`RFC5880-6.8.7-5`](#rfc5880-6.8.7-5)

A system MUST NOT transmit BFD Control packets if bfd.RemoteDiscr is zero and the system is taking the Passive role. (§6.8.7)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge rejudge 2026-10-02 (tag move under the 2026-09-30 owner ruling, OWNER RULING 6(b)); judged by an agent that authored neither test. Stale only because the RFC5880-6.1-3 negative tag comment was removed from session TestRFC5880PassiveRoleSilentUntilReception; body unchanged. Engine TestRFC5880PassiveTickSendsNothingWithoutRemoteDiscr recorded revert loop.go::tick; session TestRFC5880PassiveSilentOnceRemoteDiscrCleared (+) and TestRFC5880PassiveTransmitsAgainOnceRemoteDiscrLearned (-) recorded revert timers.go::transmitPermitted, all observed red.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880ActiveRoleTransmitsWithoutReception`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L366) | unit/verify | revert, verified |
| negative | [`TestRFC5880PassiveTransmitsAgainOnceRemoteDiscrLearned`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L1479) | unit/verify | revert, verified |
| positive | [`TestRFC5880PassiveTickSendsNothingWithoutRemoteDiscr`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_engine_clauses_test.go#L82) | unit/verify | revert, verified |
| positive | [`TestRFC5880PassiveRoleSilentUntilReception`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L388) | unit/verify | revert, verified |
| positive | [`TestRFC5880PassiveSilentOnceRemoteDiscrCleared`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L1439) | unit/verify | revert, verified |

### [`RFC5880-6.8.7-6`](#rfc5880-6.8.7-6)

A system MUST NOT periodically transmit BFD Control packets if bfd.RemoteMinRxInterval is zero. (§6.8.7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5880-6.8.7-6, so no unit is bound to it.

### [`RFC5880-6.8.7-7`](#rfc5880-6.8.7-7)

A system MUST NOT periodically transmit BFD Control packets if Demand mode is active on the remote system (bfd.RemoteDemandMode is 1, bfd.SessionState is Up, and bfd.RemoteSessionState is Up) and a Poll Sequence is not being transmitted. (§6.8.7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5880-6.8.7-7, so no unit is bound to it.

### [`RFC5880-6.8.7-8`](#rfc5880-6.8.7-8)

If rate limiting is in effect, the advertised value of Desired Min TX Interval MUST be greater than or equal to the interval between transmitted packets imposed by the rate limiting function. (§6.8.7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5880-6.8.7-8, so no unit is bound to it.

### [`RFC5880-6.8.4-1`](#rfc5880-6.8.4-1)

If Demand mode is not active, and a period of time equal to the Detection Time passes without receiving a BFD Control packet from the remote system, and bfd.SessionState is Init or Up, the session has gone down -- the local system MUST set bfd.SessionState to Down and bfd.LocalDiag to 1 (Control Detection Time Expired). (§6.8.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge rejudge 2026-09-30 (bfd engine 2). TestRFC5880DetectionExpiryDownDiagOne lost its 6.8.1-14 negative tag (comment only, body unchanged); it still proves expiry from Init sets Down and diag 1. Judgement unchanged: Judge rejudge 2026-09-30 (bfd session+engine continuation). TestRFC5880DetectionExpiryFromUpDownDiagOne adds expiry from Up (D bit clear and set in the last packet): 1 ns early no change, at the Detection Time Down, diag 1, one notify; Init expiry and the ignored-when-Down negative stand. Ze never sets bfd.DemandMode, so the 'Demand mode is not active' condition always holds; local Demand mode is an absent feature, not a branch to test. Mutant CheckDetection state guard observed red.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880DetectionExpiryIgnoredWhenDown`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L330) | unit/verify | revert, verified |
| positive | [`TestRFC5880DetectionExpiryFromUpDownDiagOne`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_session_clauses_test.go#L235) | unit/verify | mutant, verified |
| positive | [`TestRFC5880DetectionExpiryDownDiagOne`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L308) | unit/verify | revert, verified |

### [`RFC5880-6.8.5-1`](#rfc5880-6.8.5-1)

When the Echo function is active and a sufficient number of Echo packets have not arrived as they should, the session has gone down -- the local system MUST set bfd.SessionState to Down and bfd.LocalDiag to 2 (Echo Function Failed). (§6.8.5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge rejudge 2026-09-30 (bfd engine 2). Engine TestRFC5880EngineFailsSessionOnMissingEchoes: echoTickLocked sends an echo, none returns, and the tick after the echo detection time leaves Down with diag 2; the test never calls EchoFail. The targeted break the earlier note named, removing the EchoFail call in echoTickLocked, observed red (state stays Up). Session +/- units unchanged.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880EchoReturnClearsMissAndFailIsScoped`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L997) | unit/verify | revert, verified |
| positive | [`TestRFC5880EngineFailsSessionOnMissingEchoes`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_engine_clauses_test.go#L114) | unit/verify | revert, verified |
| positive | [`TestRFC5880EchoMissDetectedAndReported`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L961) | unit/verify | revert, verified |

### [`RFC5880-6.8.15-1`](#rfc5880-6.8.15-1)

When the forwarding plane in the local system is reset for some reason, such that the remote system can no longer rely on the local forwarding state, the local system MUST set bfd.LocalDiag to 4 (Forwarding Plane Reset), and set bfd.SessionState to Down. (§6.8.15)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5880-6.8.15-1, so no unit is bound to it.

### [`RFC5880-6.8.16-4`](#rfc5880-6.8.16-4)

If enabling session Set bfd.SessionState to Down (§6.8.16)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge 2026-09-30 (rejudge, netns+pinning). RFC 5880 §6.8.16 "If enabling session Set bfd.SessionState to Down". Positive TestRFC5880AdministrativeDisableEnable asserts exact Down after AdminEnable on an AdminDown session. Negative TestRFC5880EnableLandsOnDownWhilePeerSaysUpOrInit (R1 b) pushes the inputs toward the violation: Up session disabled, peer keeps sending Up then Init so bfd.RemoteSessionState is Up/Init (asserted as precondition), AdminEnable still lands on Down and Build() announces Down; an overlay adopting RemoteSessionState reds it (author e4-ov7). Old TestRFC5880AdministrativeCallsAreGuarded untagged (asserted a Ze guard, not this row). Revert records on fsm.go::AdminEnable observed red for both.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880EnableLandsOnDownWhilePeerSaysUpOrInit`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L1159) | unit/verify | revert, verified |
| positive | [`TestRFC5880AdministrativeDisableEnable`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L1094) | unit/verify | revert, verified |

### [`RFC5880-6.8.16-5`](#rfc5880-6.8.16-5)

Else Set bfd.SessionState to AdminDown Set bfd.LocalDiag to an appropriate value Cease the transmission of BFD Echo packets (§6.8.16)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge 2026-09-30. Row quotes the three disable steps of §6.8.16 verbatim. State and diag: session TestRFC5880AdministrativeDisableEnable asserts AdminDown and LocalDiag 7 after AdminDown from Up; engine TestRFC5880AdminDisableCeasesEchoes asserts the same through handle.Shutdown. Cease Echo: CeasesEchoes returns the first echo so detection stays clear, then 100 ticks at 1 ms (twice the 50 ms Required Min Echo RX) send nothing; the negative TestRFC5880AdminDisableIgnoresPeerEchoRequest forces the input toward the violation (peer keeps sending Up with a 50 ms echo request every 10 ms) and asserts no echo and AdminDown/diag 7. Judge re-observed: overlay break echo.go echoTickLocked gate `!= StateUp` -> `== StateDown` reddens both engine units at 50 ms. Revert records on echoTickLocked and AdminDown verify.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880AdminDisableIgnoresPeerEchoRequest`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_engine_clauses_test.go#L235) | unit/verify | revert, verified |
| positive | [`TestRFC5880AdminDisableCeasesEchoes`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_engine_clauses_test.go#L191) | unit/verify | revert, verified |
| positive | [`TestRFC5880AdministrativeDisableEnable`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L1089) | unit/verify | revert, verified |

### [`RFC5880-6.8.8-1`](#rfc5880-6.8.8-1)

A received BFD Echo packet MUST be demultiplexed to the appropriate session for processing. (§6.8.8)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge 2026-09-30 (rejudge, 6-2 + rest). NEW TestRFC5880EchoDemultiplexedAmongSessions: two Up echo sessions in one loop; a returning echo naming session 2 records its RTT on session 2 and leaves session 1 at zero, then one naming session 1 records on session 1, so delivery to 'the only/first session' goes red. Record revert echo.go::handleEchoInbound observed red. - TestRFC5880UnknownEchoDropped sound; old one-session + kept.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880UnknownEchoDropped`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L659) | unit/verify | revert, verified |
| positive | [`TestRFC5880EchoDemultiplexedAmongSessions`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_echo_session_test.go#L122) | unit/verify | revert, verified |
| positive | [`TestRFC5880EchoDemultiplexedToItsSession`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L622) | unit/verify | revert, verified |

### [`RFC5880-6.8.8-2`](#rfc5880-6.8.8-2)

A means of detecting missing Echo packets MUST be implemented, which most likely involves processing of the Echo packets that are received. (§6.8.8)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge rejudge 2026-09-30 (bfd engine 2). Positive: engine TestRFC5880EngineFailsSessionOnMissingEchoes (the engine consults EchoDetectionExpired on its tick; removing the EchoFail branch observed red). Negative: engine TestRFC5880EngineReturnedEchoesKeepSessionUp returns every echo through handleEchoInbound and stays Up for three echo detection times; targeted break recordEchoRTTLocked not calling MatchEchoRx observed red (Down, diag 2). Session detector units unchanged.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880EngineReturnedEchoesKeepSessionUp`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_engine_clauses_test.go#L140) | unit/verify | revert, verified |
| negative | [`TestRFC5880EchoReturnClearsMissAndFailIsScoped`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L992) | unit/verify | revert, verified |
| positive | [`TestRFC5880EngineFailsSessionOnMissingEchoes`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_engine_clauses_test.go#L119) | unit/verify | revert, verified |
| positive | [`TestRFC5880EchoMissDetectedAndReported`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L956) | unit/verify | revert, verified |

### [`RFC5880-6.8.9-1`](#rfc5880-6.8.9-1)

BFD Echo packets MUST NOT be transmitted when bfd.SessionState is not Up. (§6.8.9)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge 2026-09-30 (rejudge, 6-2 + rest). 'Not Up' now covered beyond AdminDown: NEW TestRFC5880NoEchoTransmittedInInitOrDown drives an Init session and a Down-after-Up session with echo still negotiated through handleInbound; an echo scheduler pass sends nothing and arms no schedule, while an Up session in the same loop does send (control). Overlay gating echo on '== AdminDown' reds both subtests (job-ov7); record revert echo.go::echoTickLocked observed red. + TestRFC5880EchoTransmittedWhileUp and AdminDown - kept.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880NoEchoTransmittedInInitOrDown`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_echo_session_test.go#L69) | unit/verify | revert, verified |
| negative | [`TestRFC5880NoEchoTransmittedWhenNotUp`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L597) | unit/verify | revert, verified |
| positive | [`TestRFC5880EchoTransmittedWhileUp`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L576) | unit/verify | revert, verified |

### [`RFC5880-6.8.9-2`](#rfc5880-6.8.9-2)

BFD Echo packets MUST NOT be transmitted unless the last BFD Control packet received from the remote system contains a nonzero value in Required Min Echo RX Interval. (§6.8.9)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge rejudge 2026-09-30 (bfd session+engine continuation). TestRFC5880EchoFollowsLastAdvertisement sends 50 ms, 0, 50 ms and asserts EchoEnabled and the PrimeEcho deadline follow the LAST packet each time, closing the latch gap the earlier note named; mutant EchoEnabled != -> == observed red. The engine echoTickLocked sends only for an Up session with EchoEnabled and a primed deadline.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880EchoFollowsLastAdvertisement`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_session_clauses_test.go#L298) | unit/verify | mutant, verified |
| negative | [`TestRFC5880EchoNotTransmittedWithoutPeerAdvertisement`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L891) | unit/verify | revert, verified |
| positive | [`TestRFC5880EchoEnabledWhenPeerAdvertisesNonZero`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L870) | unit/verify | revert, verified |

### [`RFC5880-6.8.9-3`](#rfc5880-6.8.9-3)

The interval between transmitted BFD Echo packets MUST NOT be less than the value advertised by the remote system in Required Min Echo RX Interval (§6.8.9)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge rejudge 2026-09-30 (bfd engine 2). Engine TestRFC5880EchoSteadySpacingHonorsPeerFloor: local 10 ms, peer 50 ms, no floor change; ticks at 1..49 ms send nothing, 50 ms sends. The targeted break the earlier note named, AdvanceEcho scheduling from DesiredMinEchoTxInterval alone, observed red (echo at 10 ms). Earlier engine and session units unchanged.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880EchoLoweredPeerFloorTakesEffect`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L861) | unit/verify | revert, verified |
| negative | [`TestRFC5880EchoIntervalNotBelowLocalTarget`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L933) | unit/verify | revert, verified |
| positive | [`TestRFC5880EchoSteadySpacingHonorsPeerFloor`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_engine_clauses_test.go#L163) | unit/verify | revert, verified |
| positive | [`TestRFC5880EchoRaisedPeerFloorDelaysNextEcho`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L829) | unit/verify | revert, verified |
| positive | [`TestRFC5880EchoIntervalHonorsPeerFloor`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L914) | unit/verify | revert, verified |

### [`RFC5880-7-1`](#rfc5880-7-1)

When BFD is used across multiple hops, a congestion control mechanism MUST be implemented (§7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5880-7-1, so no unit is bound to it.

### [`RFC5880-7-2`](#rfc5880-7-2)

When BFD is used across multiple hops, a congestion control mechanism MUST be implemented, and when congestion is detected, the BFD implementation MUST reduce the amount of traffic it generates. (§7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5880-7-2, so no unit is bound to it.

### [`RFC5880-4.3-1`](#rfc5880-4.3-1)

This byte MUST be set to zero on transmit (§4.3, §4.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge 2026-09-28 (bfd/auth, independent of the author). TestRFC5880ReservedByteZeroedOnTransmit signs into a buffer pre-filled with 0xFF for all four keyed types, so the zero at off+3 is written by digestSigner.Sign (buf[off+3] = 0), covering both cited sections 4.3 and 4.4. The {single-polarity: positive} annotation holds: Sign writes a constant 0 and no path emits another value.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC5880ReservedByteZeroedOnTransmit`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_keyed_test.go#L121) | unit/verify | revert, verified |
| positive | [`TestRFC5880KeyedMD5SectionHeader`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L91) | unit/verify | revert, verified |

### [`RFC5880-6.7.4-9`](#rfc5880-6.7.4-9)

bfd.XmitAuthSeq SHOULD be incremented when the session state changes, or when the transmitted BFD Control packet carries different contents than the previously transmitted packet. (§6.7.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge 2026-09-30 (re-judge, independent of the author). Row is the Keyed SHA1 §6.7.4 SHOULD, quoted verbatim (rfc5880.txt 1409). engine/rfc5880_xmit_auth_seq_test.go drives a Keyed SHA1 session through the real transmit path (Loop.tick periodic, Loop.handleInbound Final) and parses the Sequence Number of every transmitted packet. + AdvancesOnStateAndContentChange: state clause (Down->Init->Up, preconditions pin the transmitted states, each seq differs) and contents clause (Final with F set, Up, then periodic with F clear, Up: seq differs). - AdvancesForAnOutOfTimerFinal (R1b: forces the producer input toward the violation): the Final sent from handleInbound at the same instant as the previous periodic packet carries a new seq, and every differing consecutive pair carries distinct seqs. Records revert AdvanceAuthSeq (+) and sendLocked (-), observed red; author's targeted overlays (advance only after periodic / only on timer) each red only their own unit, so the two units are not one assertion wearing two hats. MD5 twin 6.7.3-5 not covered here.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880XmitAuthSeqAdvancesForAnOutOfTimerFinal`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_xmit_auth_seq_test.go#L194) | unit/verify | revert, verified |
| positive | [`TestRFC5880XmitAuthSeqAdvancesOnStateAndContentChange`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_xmit_auth_seq_test.go#L144) | unit/verify | revert, verified |

### [`RFC5880-6.7.3-14`](#rfc5880-6.7.3-14)

SHOULD also allow for the configuration of any arbitrary binary string in hexadecimal form. (§6.7.3)

Audit verdict: unimplemented (no code path enforces the requirement), fresh. Judge 2026-09-30 (phase 8). Confirmed at the producer: parseAuthConfig reads the secret leaf with stringField and stores []byte(secret) (config.go); the YANG leaf secret is type string with no hex form, so a binary Keyed MD5 key cannot be configured. The {gap} annotation is accurate; disclosed in Support remaining.

No test carries RFC5880-6.7.3-14, so no unit is bound to it.

### [`RFC5880-6.7.4-8`](#rfc5880-6.7.4-8)

SHOULD also allow for the configuration of any arbitrary binary string in hexadecimal form. (§6.7.4)

Audit verdict: unimplemented (no code path enforces the requirement), fresh. Judge 2026-09-30 (phase 8). Confirmed at the producer: parseAuthConfig stores []byte(secret) of the string leaf; no hexadecimal form is parsed, so a binary Keyed SHA1 key cannot be configured. The {gap} annotation is accurate; disclosed in Support remaining.

No test carries RFC5880-6.7.4-8, so no unit is bound to it.

### [`RFC5880-6.8.16-2`](#rfc5880-6.8.16-2)

BFD Control packets SHOULD be transmitted for at least a Detection Time after transitioning to AdminDown state in order to ensure that the remote system is aware of the state change. (§6.8.16)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge rejudge 2026-09-30 under OWNER RULING 3 (Ze sends AdminDown Control packets for 3x the Detection Time). Producer: session/fsm.go const adminDownTransmitDetectionTimes = 3, Machine.AdminDown sets adminDownTxEnd = now + 3 x max(local DetectionInterval, DetectMult x TransmitInterval read after the 1 s non-Up floor); Loop.tick sends until then; retireReleasedLocked keeps a released entry until AdminDownTransmitEnd, so the release path stays sending for the whole window. + TestRFC5880AdminDownControlTransmittedForADetectionTime (shutdown and release, peer silent, 30 s of ticks): first packet at the transition, every packet State AdminDown Diag 7, no gap over the announced interval, packets after 1x and 2x the Detection Time, last no earlier than 3x minus one interval, Detection Time computed from the packets themselves. Stopping before one Detection Time (the SHOULD's floor) reddens it. Judge overlays: factor 1 and 2 red both subtests ('none sent after N Detection Times'); window from the local Detection Time only reds both (remote half discriminated); factor 4 leaves it green (correct: sending longer satisfies the SHOULD). Record revert loop.go::tick re-recorded, observed red. {single-polarity: positive} still accepted: the obligation is to keep sending, silence after the window is 6.8.16-3's MAY.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC5880AdminDownControlTransmittedForADetectionTime`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_admindown_transmit_test.go#L55) | unit/verify | revert, verified |

### [`RFC5880-6.8.16-3`](#rfc5880-6.8.16-3)

BFD Control packets MAY be transmitted indefinitely after transitioning to AdminDown state in order to maintain session state in each system (§6.8.16)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge rejudge 2026-10-02 (tag move under the 2026-09-30 owner ruling, OWNER RULING 6(b)); judged by an agent that authored neither test. Stale only because the RFC5880-6.8.6-15 negative tag comment was removed from engine TestRFC5880NoPeriodicTransmitWhileAdminDown; body unchanged. Judged under OWNER RULING 3 on that test: sends at the transition, at start+1x+1ns and start+2x+1ns, nothing at exactly 3x nor an hour later; producer session/fsm.go adminDownTransmitDetectionTimes = 3 and Loop.tick; record revert loop.go::tick observed red. {single-polarity: negative} as before.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880NoPeriodicTransmitWhileAdminDown`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L403) | unit/verify | revert, verified |

### [`RFC5880-9-2`](#rfc5880-9-2)

When a BFD session is directly connected across a single link (physical, or a secure tunnel such as IPsec), the TTL or Hop Count MUST be set to the maximum on transmit, and checked to be equal to the maximum value on reception (and the packet dropped if this is not the case). (§9)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge 2026-09-30 (rejudge, 6-2 + rest). Transmit: IPv4 IP_TTL 255 (TestRFC5880SingleHopTransmitTTLIsMaximum) and NEW IPv6 IPV6_UNICAST_HOPS 255 on a [::1] single-hop socket (TestRFC5880SingleHopTransmitHopLimitIsMaximumIPv6; record revert udp_linux.go::applySocketOptionsV6 observed red). Receive: NEW + TestRFC5880SingleHopReceiveTTLMaxAccepted (TTL 255 delivered, RemoteDiscr set; record revert loop.go::passesTTLGate observed red) beside - TestRFC5880SingleHopReceiveTTLNotMaxDiscarded, so a gate dropping every packet now goes red. Both audit gaps closed.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880SingleHopReceiveTTLNotMaxDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L160) | unit/verify | revert, verified |
| positive | [`TestRFC5880SingleHopReceiveTTLMaxAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_ttl_accept_test.go#L5) | unit/verify | revert, verified |
| positive | [`TestRFC5880SingleHopTransmitHopLimitIsMaximumIPv6`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/transport/rfc5880_hop_limit_linux_test.go#L14) | unit/verify | revert, verified |
| positive | [`TestRFC5880SingleHopTransmitTTLIsMaximum`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/transport/rfc5880_test.go#L43) | unit/verify | revert, verified |

### [`RFC5880-6.8.14-1`](#rfc5880-6.8.14-1)

If Demand mode is no longer active on the remote system, the local system MUST begin transmitting periodic BFD Control packets (§6.8.14)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC5880PeriodicTransmitWhenRemoteDemandInactive`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L372) | unit/verify | unproven |

### [`RFC5880-6.8.17-1`](#rfc5880-6.8.17-1)

If Demand mode is active on the remote system (the local system is not transmitting periodic BFD Control packets), a Poll Sequence MUST be initiated to ensure that the diagnostic code is transmitted. (§6.8.17)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5880-6.8.17-1, so no unit is bound to it.

### [`RFC5880-6.8.3-8`](#rfc5880-6.8.3-8)

In any case other than those explicitly called out above, timing parameter changes MUST be effected immediately (changing the transmission rate and/or the Detection Time). (§6.8.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge rejudge 2026-09-30 (bfd session+engine continuation). Row is {single-polarity: positive}. TestRFC5880RemoteTimingChangeMovesDetectionTimeAtOnce adds the Detection Time half: a received Desired Min TX 600 ms moves it 900 ms -> 1.8 s at once (no expiry at 901 ms, expiry at 1.8 s) and Detect Mult 5 moves it to 1.5 s; the TX half stays on RemoteMinRxReductionHonoredImmediately. Mutant DetectionInterval mult guard observed red.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC5880RemoteTimingChangeMovesDetectionTimeAtOnce`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_session_clauses_test.go#L193) | unit/verify | mutant, verified |
| positive | [`TestRFC5880RemoteMinRxReductionHonoredImmediately`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L505) | unit/verify | revert, verified |

### [`RFC5880-6.7.3-7`](#rfc5880-6.7.3-7)

the management interface by which the key is configured MUST accept ASCII strings (§6.7.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge 2026-09-30. Same config-text -> YANG -> plugin section -> resolveProfile units as 6.7.2-1, keyed-md5 and meticulous-keyed-md5 subtests (keys up to 16 bytes): positive a plain ASCII key, negative (R1b) the 0x01-0x7F sweep with leading/trailing spaces; judge's trailing-space-strip break reds both MD5 negative subtests, positive green. Revert records observed red. The incomplete-block refusal is untagged now (Ze's own rule, D-15).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880KeyManagementConfigEntryKeepsEveryASCIICharacter`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5880_key_management_config_test.go#L117) | unit/verify | revert, verified |
| positive | [`TestRFC5880KeyManagementConfigEntryAcceptsASCIIStrings`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5880_key_management_config_test.go#L92) | unit/verify | revert, verified |
| positive | [`TestRFC5880KeyManagementAcceptsASCIIStrings`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5880_test.go#L26) | unit/verify | revert, verified |

### [`RFC5880-6.7.2-3`](#rfc5880-6.7.2-3)

The currently selected password and Key ID for the session MUST be stored in the Authentication Section of each outgoing BFD Control packet. (§6.7.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge rejudge 2026-09-30 (phase 8: the Keyed SHA1 key-id units moved to RFC5880-6.7.4-10, row cites 6.7.2 only). Positive TestRFC5880SimplePasswordSectionCarriesPasswordAndKeyID asserts the selected password and Key ID on the signed wire. Negatives TestRFC5880SimplePasswordOtherThanSelectedPairDiscarded (key id 4/6/255 with the right password, password changed in first/middle/last byte, ErrPasswordMismatch each, selected pair accepted) and TestRFC5880SimplePasswordSignerNeedsPassword (nil/empty password refused at NewSigner). Units unchanged; judgement unchanged.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880SimplePasswordSignerNeedsPassword`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_keyed_test.go#L182) | unit/verify | revert, verified |
| negative | [`TestRFC5880SimplePasswordOtherThanSelectedPairDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_section_fields_test.go#L63) | unit/verify | mutant, verified |
| positive | [`TestRFC5880SimplePasswordSectionCarriesPasswordAndKeyID`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L686) | unit/verify | revert, verified |

### [`RFC5880-6.7.3-8`](#rfc5880-6.7.3-8)

The Sequence Number field MUST be set to bfd.XmitAuthSeq. (§6.7.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge rejudge 2026-09-30 (phase 8: row cites 6.7.3 only; the Keyed SHA1 session units moved to RFC5880-6.7.4-11). Positive TestRFC5880MD5SequenceFieldIsXmitAuthSeq (new): both MD5 types through Machine.Sign, field = 0x11223344 and then the advanced value. Negative (R1b, forced input) TestRFC5880SequenceFieldIsXmitAuthSeqAcrossTheCounterRange: XmitAuthSeq forced to 0, 1, 0x7FFFFFFF, 0x80000000, 0xFFFFFFFE, 0xFFFFFFFF and across the wrap, field equal after every Sign, all four keyed types; an earlier judge break (seq masked to 31 bits) reds it.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880SequenceFieldIsXmitAuthSeqAcrossTheCounterRange`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_auth_seq_field_test.go#L17) | unit/verify | revert, verified |
| positive | [`TestRFC5880MD5SequenceFieldIsXmitAuthSeq`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_md5_seq_field_test.go#L15) | unit/verify | revert, verified |

### [`RFC5880-6.7.2-4`](#rfc5880-6.7.2-4)

If the received BFD Control packet does not contain an Authentication Section, or the Auth Type is not 1 (Simple Password), then the received packet MUST be discarded. (§6.7.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge rejudge 2026-09-30 (phase 8: row cites 6.7.2 only; the SHA1 type units moved to RFC5880-6.7.4-14, the MD5 clause is RFC5880-6.7.3-16). Positive accepts Auth Type 1 sections for 1/8/16-byte passwords; negative TestRFC5880SimplePasswordMissingSectionOrOtherTypeDiscarded refuses an A-clear Length-24 packet (ErrShortAuthBody) and types 0,2,3,4,5,6,255 with every other field valid, isolated because Simple Password has no digest. Stack-level no-section discard is Receive's A-bit gate (RFC5880-6.8.6-10).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880SimplePasswordMissingSectionOrOtherTypeDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_keyed_test.go#L211) | unit/verify | revert, verified |
| positive | [`TestRFC5880SimplePasswordSectionOfTypeOneAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_keyed_test.go#L194) | unit/verify | revert, verified |

### [`RFC5880-6.7.2-5`](#rfc5880-6.7.2-5)

If the Auth Key ID field does not match the ID of a configured password, the received packet MUST be discarded. (§6.7.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge rejudge 2026-09-30 (phase 8: row cites 6.7.2 only; the SHA1 key-id units moved to RFC5880-6.7.4-15). Negative TestRFC5880SimplePasswordWrongKeyIDDiscarded signs key 7 with the right password against a verifier for key 8 and requires ErrPasswordMismatch (only the key id differs, isolated); positive TestRFC5880SimplePasswordMatchingKeyIDAccepted accepts key 7.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880SimplePasswordWrongKeyIDDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L759) | unit/verify | unproven |
| positive | [`TestRFC5880SimplePasswordMatchingKeyIDAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L776) | unit/verify | unproven |

### [`RFC5880-6.7.2-6`](#rfc5880-6.7.2-6)

If the Auth Len field is not equal to the length of the password selected by the key ID, plus three, the packet MUST be discarded. (§6.7.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge rejudge 2026-09-30 (phase 8: row cites 6.7.2 only; the SHA1 Auth Len units moved to RFC5880-6.7.4-16). Positive accepts Auth Len = password+3 for 1/8/16 bytes; negative changes only the Auth Len byte (0, n+2, n+4, 255) with Control Length, type, key id and password valid and requires ErrPasswordMismatch; removing the byte check in simpleVerifier.Verify accepts them.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880SimplePasswordAuthLenNotPlusThreeDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_keyed_test.go#L261) | unit/verify | revert, verified |
| positive | [`TestRFC5880SimplePasswordAuthLenPlusThreeAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_keyed_test.go#L244) | unit/verify | revert, verified |

### [`RFC5880-6.7.3-16`](#rfc5880-6.7.3-16)

If the received BFD Control packet does not contain an Authentication Section, or the Auth Type is not correct (2 for Keyed MD5 or 3 for Meticulous Keyed MD5), then the received packet MUST be discarded. (§6.7.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge 2026-09-30 (phase 8). Positive TestRFC5880MD5AuthTypeCorrectAccepted (types 2 and 3). Negative TestRFC5880KeyedMissingSectionOrWrongTypeDiscarded: A-clear Length-24 packet -> ErrShortAuthBody, and types 0,1,2..6,255 relabelled on an authentic packet -> discard with the replay floor unset. Judge break removing the Auth Type check in digestVerifier.Verify reds it (the floor assertion isolates it from the digest). The no-section clause at stack level is enforced by Receive's A-bit gate (session/fsm.go, proven under RFC5880-6.8.6-10, loop.go skips Verify for A=0); this unit proves the verifier refuses it too.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880KeyedMissingSectionOrWrongTypeDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_md5_split_test.go#L93) | unit/verify | revert, verified |
| positive | [`TestRFC5880MD5AuthTypeCorrectAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_md5_split_test.go#L76) | unit/verify | revert, verified |

### [`RFC5880-6.7.3-17`](#rfc5880-6.7.3-17)

If the Auth Key ID field does not match the ID of a configured authentication key, the received packet MUST be discarded. (§6.7.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge 2026-09-30 (phase 8). Positive TestRFC5880MD5ConfiguredKeyIDAccepted; negative TestRFC5880MD5UnconfiguredKeyIDDiscarded (ids 2/4/255 vs 3, both MD5 types, floor unset). Judge break removing the key-id check in digestVerifier.Verify reds the negative (floor seeded), so it is isolated from the digest.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880MD5UnconfiguredKeyIDDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_md5_split_test.go#L171) | unit/verify | revert, verified |
| positive | [`TestRFC5880MD5ConfiguredKeyIDAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_md5_split_test.go#L157) | unit/verify | revert, verified |

### [`RFC5880-6.7.3-18`](#rfc5880-6.7.3-18)

If the Auth Len field is not equal to 24, the packet MUST be discarded. (§6.7.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge 2026-09-30 (phase 8). Positive TestRFC5880MD5AuthLen24Accepted (byte 24, Length 48); negative TestRFC5880MD5AuthLenNot24Discarded (byte 0/23/25/28 with floor unset, Length 47 ErrShortAuthBody, 49 refused). Judge break removing the Auth Len byte check in digestVerifier.Verify reds the negative through the floor assertion.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880MD5AuthLenNot24Discarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_md5_split_test.go#L213) | unit/verify | revert, verified |
| positive | [`TestRFC5880MD5AuthLen24Accepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_md5_split_test.go#L193) | unit/verify | revert, verified |

### [`RFC5880-6.7.4-14`](#rfc5880-6.7.4-14)

If the received BFD Control packet does not contain an Authentication Section, or the Auth Type is not correct (4 for Keyed SHA1 or 5 for Meticulous Keyed SHA1), then the received packet MUST be discarded. (§6.7.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge 2026-09-30 (phase 8). Positive TestRFC5880AuthTypeMatchAccepted (type 5). Negative TestRFC5880KeyedMissingSectionOrWrongTypeDiscarded covers types 4 and 5 (no section; types 0,1,2,3,6,255 and the other SHA1 variant) with the floor unset, and the judge break removing the Auth Type check reds it. The moved negative TestRFC5880AuthTypeMismatchDiscarded stayed GREEN under that break: it asserts only err != nil and the relabelled byte also breaks the hash, so it is digest-confounded and proves nothing alone. No-section clause at stack level: RFC5880-6.8.6-10.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880KeyedMissingSectionOrWrongTypeDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_md5_split_test.go#L98) | unit/verify | revert, verified |
| negative | [`TestRFC5880AuthTypeMismatchDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L314) | unit/verify | revert, verified |
| positive | [`TestRFC5880AuthTypeMatchAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L301) | unit/verify | revert, verified |

### [`RFC5880-6.7.4-15`](#rfc5880-6.7.4-15)

If the Auth Key ID field does not match the ID of a configured authentication key, the received packet MUST be discarded. (§6.7.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge 2026-09-30 (phase 8). Positive TestRFC5880AuthKeyIDMatchAccepted (key 3); negative TestRFC5880AuthKeyIDMismatchDiscarded (key 4 vs 3, ErrDigestMismatch, floor unset). Judge break removing the key-id check reds the negative through the floor assertion. Ze configures one key per session, so 'a configured key' is that key.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880AuthKeyIDMismatchDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L270) | unit/verify | revert, verified |
| positive | [`TestRFC5880AuthKeyIDMatchAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L289) | unit/verify | revert, verified |

### [`RFC5880-6.7.4-16`](#rfc5880-6.7.4-16)

If the Auth Len field is not equal to 28, the packet MUST be discarded. (§6.7.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge rejudge 2026-09-30 (last five). Earlier weak finding closed: new negative auth/rfc5880_sha1_authlen_test.go::TestRFC5880SHA1AuthLenByteDiscardedWithAuthenticDigest hashes the digest over the forged Auth Len byte (0/20/24/27/29/255) for Keyed SHA1 and Meticulous Keyed SHA1, keeps Control Length and the other section fields authentic, and asserts ErrDigestMismatch plus bfd.RcvAuthSeq unseeded, so only the Auth Len check can refuse it. Judge break (int(data[off+1]) != v.bodyLen -> if false && ..., sha1.go Verify, go overlay): new negative RED ('type 4: Auth Len 0 with an authentic digest: got <nil>'), new positive and the old pair green. Positive TestRFC5880SHA1AuthLen28HandBuiltAccepted shows the same builder at 28 is accepted (isolation). Old TestRFC5880AuthLenMismatchDiscarded keeps its tag but does not discriminate the Auth Len byte (digest refuses it); supplementary only. Records: Verify revert, observed red, all four units.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880SHA1AuthLenByteDiscardedWithAuthenticDigest`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_sha1_authlen_test.go#L53) | unit/verify | revert, verified |
| negative | [`TestRFC5880AuthLenMismatchDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L347) | unit/verify | revert, verified |
| positive | [`TestRFC5880SHA1AuthLen28HandBuiltAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_sha1_authlen_test.go#L38) | unit/verify | revert, verified |
| positive | [`TestRFC5880AuthLenExpectedAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L330) | unit/verify | revert, verified |

### [`RFC5880-6.7.2-7`](#rfc5880-6.7.2-7)

If the Password field does not match the password selected by the key ID, the packet MUST be discarded. (§6.7.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: accepting a password other than the configured one. Red: TestRFC5880SimplePasswordMismatchDiscarded Fatalf unless ErrPasswordMismatch for a same-length one-byte difference (isolated from the Auth Len check), plus shorter and longer passwords; positive TestRFC5880SimplePasswordMatchAccepted round-trips 1, 8 and 16-byte passwords. One key per session, so 'selected by the key ID' is the configured pair.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880SimplePasswordMismatchDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L728) | unit/verify | unproven |
| positive | [`TestRFC5880SimplePasswordMatchAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L709) | unit/verify | unproven |

### [`RFC5880-6.7.3-9`](#rfc5880-6.7.3-9)

For Keyed MD5, if the sequence number lies outside of the range of bfd.RcvAuthSeq to bfd.RcvAuthSeq+(3*Detect Mult) inclusive (when treated as an unsigned 32-bit circular number space), the received packet MUST be discarded. (§6.7.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge rejudge 2026-09-30 (last five). Earlier finding closed: the three Keyed-SHA1-only units (KeyedSequenceAtOrAboveFloorAccepted, KeyedSequenceBelowFloorDiscarded, KeyedSequenceWindowWraps) moved to RFC5880-6.7.4-12. Keyed MD5 is proven by TestRFC5880KeyedWindowFollowsReceivedDetectMult (+, equal and +3*DM at DM 1/5/255 and across the wrap) and TestRFC5880KeyedOutsideWindowDiscarded (-, -1, +3*DM+1, +2^31, wrap, floor unmoved), both looping MD5, plus TestRFC5880KeyedSequenceBeyondWindowDiscarded (MD5 and SHA1) and session TestRFC5880SequenceFieldOtherThanXmitAuthSeqDiscarded. Both inclusive bounds pinned for MD5; meticulous.go Check revert records observed red. Three orphan records of the moved claims show stale under discriminate id; harmless, no gate refusal.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880KeyedOutsideWindowDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_keyed_test.go#L366) | unit/verify | revert, verified |
| negative | [`TestRFC5880KeyedSequenceBeyondWindowDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L838) | unit/verify | revert, verified |
| negative | [`TestRFC5880SequenceFieldOtherThanXmitAuthSeqDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_auth_wrap_test.go#L67) | unit/verify | revert, verified |
| positive | [`TestRFC5880KeyedWindowFollowsReceivedDetectMult`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_keyed_test.go#L325) | unit/verify | revert, verified |

### [`RFC5880-6.7.3-10`](#rfc5880-6.7.3-10)

For Meticulous Keyed MD5, if the sequence number lies outside of the range of bfd.RcvAuthSeq+1 to bfd.RcvAuthSeq+(3*Detect Mult) inclusive (when treated as an unsigned 32-bit circular number space) the received packet MUST be discarded. (§6.7.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge rejudge 2026-09-30 (phase 8: keyed_test prose narrowed to Meticulous MD5, the SHA1 half is RFC5880-6.7.4-13). Positive units accept RcvAuthSeq+1 and +3*DM at DM 1/5/255 and both wraps; negatives refuse equal, -1, +3*DM+1 and behind-across-wrap with the floor unmoved; every tagged unit loops Meticulous Keyed MD5 (with SHA1 alongside). meticulous.go Check revert observed red.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880MeticulousOutsideWindowDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_keyed_test.go#L442) | unit/verify | revert, verified |
| negative | [`TestRFC5880MeticulousSequenceOutsideWindowDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L907) | unit/verify | revert, verified |
| negative | [`TestRFC5880SequenceFieldOtherThanXmitAuthSeqDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_auth_wrap_test.go#L72) | unit/verify | revert, verified |
| positive | [`TestRFC5880MeticulousWindowFollowsReceivedDetectMult`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_keyed_test.go#L408) | unit/verify | revert, verified |
| positive | [`TestRFC5880MeticulousSequenceInsideWindowAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L884) | unit/verify | revert, verified |

### [`RFC5880-6.7.3-11`](#rfc5880-6.7.3-11)

Otherwise (the digest does not match the Auth Key/Digest field), the received packet MUST be discarded. (§6.7.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge rejudge 2026-09-30 (phase 8: row cites 6.7.3 only; the SHA1 digest units moved to RFC5880-6.7.4-17). Positive TestRFC5880KeyedMD5IndependentDigestAccepted accepts a packet hashed with crypto/md5 outside the product (types 2, 3); negative TestRFC5880KeyedMD5DigestMismatchDiscarded refuses a flipped digest and a digest under another key with ErrDigestMismatch and the floor stays 40. Judge break making the constant-time compare never fail reds the negative.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880KeyedMD5DigestMismatchDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_keyed_test.go#L480) | unit/verify | revert, verified |
| positive | [`TestRFC5880KeyedMD5IndependentDigestAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_keyed_test.go#L462) | unit/verify | revert, verified |

### [`RFC5880-6.7.4-12`](#rfc5880-6.7.4-12)

For Keyed SHA1, if the sequence number lies outside of the range of bfd.RcvAuthSeq to bfd.RcvAuthSeq+(3*Detect Mult) inclusive (when treated as an unsigned 32-bit circular number space), the received packet MUST be discarded. (§6.7.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge rejudge 2026-09-30 (last five). Tags added by D-15 move on three Keyed SHA1 units in auth/rfc5880_test.go: + TestRFC5880KeyedSequenceAtOrAboveFloorAccepted (100,100,101,110 accepted, floor 110), - TestRFC5880KeyedSequenceBelowFloorDiscarded (399 below floor 400 -> ErrSequenceOutsideWindow, floor unmoved), + TestRFC5880KeyedSequenceWindowWraps (circular successor accepted); each fixture is AuthTypeKeyedSHA1 only, prose matches. Earlier proof kept: keyed_test WindowFollowsReceivedDetectMult / OutsideWindowDiscarded on SHA1 (both bounds, DM 1/5/255, wrap). meticulous.go Check revert records observed red for every unit.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880KeyedOutsideWindowDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_keyed_test.go#L368) | unit/verify | revert, verified |
| negative | [`TestRFC5880KeyedSequenceBelowFloorDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L561) | unit/verify | revert, verified |
| positive | [`TestRFC5880KeyedWindowFollowsReceivedDetectMult`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_keyed_test.go#L328) | unit/verify | revert, verified |
| positive | [`TestRFC5880KeyedSequenceAtOrAboveFloorAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L540) | unit/verify | revert, verified |
| positive | [`TestRFC5880KeyedSequenceWindowWraps`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L863) | unit/verify | revert, verified |

### [`RFC5880-6.7.4-13`](#rfc5880-6.7.4-13)

For Meticulous Keyed SHA1, if the sequence number lies outside of the range of bfd.RcvAuthSeq+1 to bfd.RcvAuthSeq+(3*Detect Mult) inclusive (when treated as an unsigned 32-bit circular number space, the received packet MUST be discarded. (§6.7.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge 2026-09-30 (phase 8). Positive TestRFC5880MeticulousWindowFollowsReceivedDetectMult (+1 and +3*DM at DM 1/5/255, both wraps) and negative TestRFC5880MeticulousOutsideWindowDiscarded (equal, -1, +3*DM+1, behind-across-wrap, floor unmoved), both looping Meticulous Keyed SHA1. The row keeps the RFC's own unbalanced parenthesis (verbatim). meticulous.go Check revert observed red.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880MeticulousOutsideWindowDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_keyed_test.go#L444) | unit/verify | revert, verified |
| positive | [`TestRFC5880MeticulousWindowFollowsReceivedDetectMult`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_keyed_test.go#L410) | unit/verify | revert, verified |

### [`RFC5880-6.7.4-17`](#rfc5880-6.7.4-17)

Otherwise (the hash does not match the Auth Key/Hash field), the received packet MUST be discarded. (§6.7.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge 2026-09-30 (phase 8). Positive TestRFC5880DigestMatchAccepted (Keyed SHA1 signed packet accepted). Negative TestRFC5880DigestMismatchDiscarded: flipped hash and other-secret hash refused with the floor left at the known value. Judge break making the constant-time compare never fail reds the negative. The positive uses the product signer, not an independent SHA1 (the MD5 row has a crypto/md5 hand-built packet); acceptable because the negative pins the compare.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880DigestMismatchDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L394) | unit/verify | revert, verified |
| positive | [`TestRFC5880DigestMatchAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L381) | unit/verify | revert, verified |

### [`RFC5880-6.7.3-12`](#rfc5880-6.7.3-12)

Otherwise (bfd.AuthSeqKnown is 0), bfd.AuthSeqKnown MUST be set to 1, and bfd.RcvAuthSeq MUST be set to the value of the received Sequence Number field. (§6.7.3, §6.7.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judge 2026-09-28. MD5: TestRFC5880KeyedMD5FirstPacketSeedsReplayFloor sets AuthSeqKnown and RcvAuthSeq from an authentic and a forged first packet (seeding precedes the digest, owner decision D-14, kept); TestRFC5880KeyedMD5KnownFloorNotReseeded shows the branch runs only while AuthSeqKnown is 0 (a forged in-window and a +2^31 packet leave the floor at 100). SHA1 (cited 6.7.4) keeps the older seeding units. RFC5880-6.8.1-13 stays a gap.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880KeyedMD5KnownFloorNotReseeded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_keyed_test.go#L613) | unit/verify | revert, verified |
| negative | [`TestRFC5880KnownSequenceNotReseededByForgedPacket`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L514) | unit/verify | revert, verified |
| positive | [`TestRFC5880KeyedMD5FirstPacketSeedsReplayFloor`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_keyed_test.go#L580) | unit/verify | revert, verified |
| positive | [`TestRFC5880FirstAuthenticatedPacketSeedsReplayFloor`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L456) | unit/verify | revert, verified |
| positive | [`TestRFC5880FirstPacketSeedsReplayFloorBeforeDigest`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L480) | unit/verify | revert, verified |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | ze-work agent, spec-fixit-rfc-drain-quota-never-armed WP-1 |
| Signed off | 2026-08-31 |
| Register | rfc2119 |
| Source | rfc/full/rfc5880.txt |
| Source fingerprint | 9a3492d0917193af |
| Record | rfc/extraction/rfc5880.json |
| Mapped sentences | 110 |
| Declined as scope | 18 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | skipped (front-matter) | Title block, Abstract, Status of This Memo, copyright notice and table of contents. The Abstract restates section 1 and states no obligation. |
| `1` | Introduction | 0 | walked | Introduction. States the goal of the protocol and says that application-dependent mechanisms live in companion documents. No obligation. |
| `1.1` | not stated | 0 | walked | Conventions Used in This Document: the RFC 2119 key-words paragraph. It tells a reader how to read the other sections and binds no speaker, which is why the derivation excludes it from the site inventory. |
| `2` | Design | 0 | walked | Design. Describes where BFD sits relative to the forwarding plane and what it runs over. Written in the indicative throughout and states no obligation. |
| `3` | Protocol Overview | 0 | walked | Protocol Overview. Describes the Hello exchange and the rate negotiation in the indicative. No obligation. |
| `3.1` | Addressing and Session Establishment | 0 | walked | Addressing and Session Establishment. States that the application chooses the addresses and that BFD has no discovery mechanism. No obligation. |
| `3.2` | Operating Modes | 0 | walked | Operating Modes. Describes Asynchronous mode, Demand mode and the Echo function, and the trade-offs between them. Their obligations are stated in section 6 and captured there. |
| `4` | BFD Control Packet Format | 0 | walked | BFD Control Packet Format. Section heading only; the formats are in the subsections below. |
| `4.1` | Generic BFD Control Packet Format | 1 | walked | Generic BFD Control Packet Format. The field descriptions are written in the indicative apart from the Multipoint bit, which is the one site. |
| `4.2` | Simple Password Authentication Section Format | 2 | walked | Simple Password Authentication Section Format. Two sites: the password length bound and the pointer to the section 6.7.2 encoding rules. |
| `4.3` | not stated | 2 | walked | Keyed MD5 and Meticulous Keyed MD5 Authentication Section Format. Two sites: the Reserved byte and the pointer to the section 6.7.3 key encoding rules. |
| `4.4` | not stated | 2 | walked | Keyed SHA1 and Meticulous Keyed SHA1 Authentication Section Format. Two sites: the Reserved byte and the pointer to the section 6.7.4 key encoding rules. |
| `5` | BFD Echo Packet Format | 0 | walked | BFD Echo Packet Format. The payload is a local matter, so the section states one advisory obligation and no MUST-level one. |
| `6` | Elements of Procedure | 0 | walked | Elements of Procedure. Introductory text that names section 6.8.1 as the home of the bfd.Xx state variables and warns against enforcing more than the section states. No obligation. |
| `6.1` | Overview | 3 | walked | Overview. The three role obligations are the sites; the rest of the section describes the session lifecycle in the indicative and is specified normatively in section 6.8. |
| `6.2` | BFD State Machine | 0 | walked | BFD State Machine. The transitions are described in the indicative and are specified normatively in section 6.8.6 and section 6.8.16. The one advisory obligation is the right to hold a session down. |
| `6.3` | Demultiplexing and the Discriminator Fields | 1 | walked | Demultiplexing and the Discriminator Fields. One site, the discriminator choice rule. The permission to change a discriminator mid-session is advisory. |
| `6.4` | The Echo Function and Asymmetry | 0 | walked | The Echo Function and Asymmetry. Describes independent Echo directions in the indicative and states one advisory obligation about advertised intervals. |
| `6.5` | The Poll Sequence | 2 | walked | The Poll Sequence. Two sites: the P and F exclusion, and the rule that a Poll rides on the scheduled periodic packets. |
| `6.6` | Demand mode | 3 | walked | Demand mode. Three sites, all about the Demand bit and the Poll Sequences a Demand mode change requires. The permission to enable or disable Demand mode at any time is advisory. |
| `6.7` | Authentication | 1 | walked | Authentication. Describes the generic authentication section, then states the one obligation of the section: an implementation that supports authentication supports both SHA1 types. |
| `6.7.1` | Enabling and Disabling Authentication | 0 | walked | Enabling and Disabling Authentication. The section says the mechanism is out of scope and then states two advisory obligations about changing the authentication state on received packets. |
| `6.7.2` | Simple Password Authentication | 10 | walked | Simple Password Authentication. Ten sites: three transmission field rules, the password bound, the management interface rule, four reception discards and the closing acceptance. The hexadecimal configuration clause is advisory. |
| `6.7.3` | Keyed MD5 and Meticulous Keyed MD5 Authentication | 17 | walked | Keyed MD5 and Meticulous Keyed MD5 Authentication. Seventeen sites over the transmission and reception rules. The sequence increment guidance for Keyed MD5 is advisory, and the circular increment is a permission. |
| `6.7.4` | Keyed SHA1 and Meticulous Keyed SHA1 Authentication | 17 | walked | Keyed SHA1 and Meticulous Keyed SHA1 Authentication. Seventeen sites that repeat the section 6.7.3 rules with SHA1 constants. Each SHA1 obligation has its own section 6.7.4 row; the Auth Len transmit rule, the first-packet seeding rule and the accepting face of the hash comparison are excluded as duplicates of the rows that carry them. The hexadecimal key clause and the sequence increment guidance are advisory, and the circular increment for Keyed SHA1 is a permission. |
| `6.8` | Functional Specifics | 0 | walked | Functional Specifics. Introductory text defining what 'the Echo function active' and 'Demand mode active' mean for the subsections below. No obligation. |
| `6.8.1` | State Variables | 14 | walked | State Variables. Fourteen sites: the session state preservation rule and the initial value or constraint of each bfd.Xx variable. The random local discriminator and the longer preservation are advisory. |
| `6.8.2` | Timer Negotiation | 0 | walked | Timer Negotiation. Describes the continuous negotiation in the indicative and defers the detail to section 6.8.7. No obligation. |
| `6.8.3` | Timer Manipulation | 8 | walked | Timer Manipulation. Eight sites over the Poll Sequence requirement, the two holds while a Poll runs, the one-second floor, the immediate honoring rule and the disambiguation choices. The Echo receive interval floor is advisory. |
| `6.8.4` | Calculating the Detection Time | 2 | walked | Calculating the Detection Time. Two sites, the Asynchronous and Demand mode expiries. The calculations themselves are written in the indicative. |
| `6.8.5` | Detecting Failures with the Echo Function | 1 | walked | Detecting Failures with the Echo Function. One site, the transition on Echo failure. The detection method is declared out of scope. |
| `6.8.6` | Reception of BFD Control Packets | 19 | walked | Reception of BFD Control Packets. Nineteen sites: the ordered procedure lead-in, its stop-on-discard clause, and the discard, demultiplexing, authentication and Demand mode steps. The state machine block in the middle of the section is written in the indicative. |
| `6.8.7` | Transmitting BFD Control Packets | 11 | walked | Transmitting BFD Control Packets. Eleven sites over the interval floor, jitter, the transmission bars, the Final response, the Demand bit precondition and the field table lead-in. The change-driven extra packet and the Final rate limit are advisory. |
| `6.8.8` | Reception of BFD Echo Packets | 2 | walked | Reception of BFD Echo Packets. Two sites: demultiplexing and the obligation to detect loss. |
| `6.8.9` | Transmission of BFD Echo Packets | 3 | walked | Transmission of BFD Echo Packets. Three sites, all bars on when and how fast Echo packets leave. |
| `6.8.10` | Min Rx Interval Change | 0 | walked | Min Rx Interval Change. States that bfd.RequiredMinRxInterval can change at any time and points at section 6.8.3 for the rules. No obligation of its own. |
| `6.8.11` | Min Tx Interval Change | 0 | walked | Min Tx Interval Change. States that bfd.DesiredMinTxInterval can change at any time and points at section 6.8.3. No obligation of its own. |
| `6.8.12` | Detect Multiplier Change | 0 | walked | Detect Multiplier Change. States that bfd.DetectMult can change to any nonzero value without a Poll Sequence and points at section 6.6. No obligation of its own. |
| `6.8.13` | Enabling or Disabling The Echo Function | 0 | walked | Enabling or Disabling The Echo Function. Two permissions, both advisory, and the summary declares them as one row. |
| `6.8.14` | Enabling or Disabling Demand Mode | 1 | walked | Enabling or Disabling Demand Mode. One site, the obligation to resume periodic transmission when the remote system leaves Demand mode. |
| `6.8.15` | Forwarding Plane Reset | 1 | walked | Forwarding Plane Reset. One site, the diagnostic and state transition on a local forwarding plane reset. |
| `6.8.16` | Administrative Control | 1 | walked | Administrative Control. One site, the enable and disable procedure, whose enable step and disable steps are two rows. The transmission after entering AdminDown is advisory. |
| `6.8.17` | Concatenated Paths | 1 | walked | Concatenated Paths. One site, the Poll Sequence that carries a concatenated path diagnostic to a remote system in Demand mode. Setting the diagnostic itself is a permission. |
| `6.8.18` | Holding Down Sessions | 1 | walked | Holding Down Sessions. One site, the REQUIRED state maintenance, which restates the section 6.8.1 preservation rule. The reset of bfd.RemoteMinRxInterval after a Detection Time of silence is advisory. |
| `7` | Operational Considerations | 1 | walked | Operational Considerations. One site carrying two obligations: implement congestion control across multiple hops, and reduce generated traffic when congestion is detected. The site maps the first; the second is declared here because no separate sentence sources it. The single-hop congestion algorithm is advisory. |
| `8` | IANA Considerations | 0 | skipped (iana) | IANA Considerations. Defines the BFD Diagnostic Codes and BFD Authentication Types registries and their initial values. Binds IANA, not a speaker. |
| `9` | Security Considerations | 1 | walked | Security Considerations. One site, the GTSM rule for a directly connected session. The use of the Authentication Section across multiple hops is advisory, and the rest of the section compares the authentication types without directing a speaker. |
| `10` | References heading | 0 | skipped (references) | References heading. |
| `10.1` | Normative References: RFC 5082, RFC 2119, RFC 1321, RFC 3174 | 0 | skipped (references) | Normative References: RFC 5082, RFC 2119, RFC 1321, RFC 3174. |
| `10.2` | Informative References: RFC 2104, RFC 5226, RFC 2328 | 0 | skipped (references) | Informative References: RFC 2104, RFC 5226, RFC 2328. |
| `A` | Appendix A, Backward Compatibility | 0 | skipped (appendix-non-normative) | Appendix A, Backward Compatibility. The heading declares itself non-normative and the text describes a suggested version 0 fallback that no speaker is bound to. |
| `B` | Appendix B, Contributors | 0 | skipped (acknowledgements) | Appendix B, Contributors. |
| `C` | Appendix C, Acknowledgments | 0 | skipped (acknowledgements) | Appendix C, Acknowledgments. |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `4.2:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | A pointer sentence: the obligation it names lives in section 6.7.2, where site 6.7.2:5 maps it as RFC5880-6.7.2-1. | The password MUST be encoded and configured according to section 6.7.2. |
| `4.3:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | A pointer sentence: the key encoding and configuration obligation lives in section 6.7.3, where site 6.7.3:6 maps it as RFC5880-6.7.3-7. | The shared key MUST be encoded and configured to section 6.7.3. |
| `4.4:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | A pointer sentence: the key encoding and configuration obligation lives in section 6.7.4, where site 6.7.4:6 maps it as RFC5880-6.7.4-5. | The shared key MUST be encoded and configured to section 6.7.4. |
| `6.7.2:3` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The length half of the row site 6.7.2:2 maps. | The Auth Len field MUST be set to the proper length (4 to 19 bytes). |
| `6.7.2:4` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The section 4.2 password length bound, restated in the transmission paragraph. Site 4.2:1 maps RFC5880-4.2-1. | The password is a binary string, and MUST be 1 to 16 bytes in length. |
| `6.7.2:10` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The closing complement of the four Simple Password discard rules. It adds no test of its own: a packet is accepted when no discard condition held, and the last of those conditions is mapped by site 6.7.2:9. | Otherwise, the packet MUST be accepted. |
| `6.7.3:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The length half of the row site 6.7.3:1 maps. | The Auth Len field MUST be set to 24. |
| `6.7.3:16` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The accepting face of the digest comparison. RFC5880-6.7.3-11 states the same comparison as a discard, and site 6.7.3:17 maps it. | If the MD5 digest of the entire BFD Control packet is equal to the received value of the Auth Key/Digest field, the received packet MUST be accepted. |
| `6.7.4:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The length half of the row site 6.7.4:1 maps. | The Auth Len field MUST be set to 28. |
| `6.7.4:15` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The first-packet sequence learning rule for the SHA1 family, declared once as RFC5880-6.7.3-12 and mapped by site 6.7.3:15. | Otherwise (bfd.AuthSeqKnown is 0), bfd.AuthSeqKnown MUST be set to 1, bfd.RcvAuthSeq MUST be set to the value of the received Sequence Number field, and the received packet MUST be accepted. |
| `6.7.4:16` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The accepting face of the SHA1 hash comparison, whose discarding face is RFC5880-6.7.4-17, mapped by site 6.7.4:17. | If the SHA1 hash of the entire BFD Control packet is equal to the received value of the Auth Key/Hash field, the received packet MUST be accepted. |
| `6.8.3:6` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The second sentence of the rule site 6.8.3:5 maps. It states what 'immediately' means when the new interval has already elapsed, and binds nothing further. | If this interval has already passed since the last transmission (because the new interval is significantly shorter), the local system MUST send the next periodic BFD Control packet as soon as practicable. |
| `6.8.4:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The Demand mode detection expiry. The action is the same obligation RFC5880-6.8.4-1 states, set Down and diagnostic 1; only the clock the Detection Time runs from differs, and section 6.6 and section 6.8.4 both state that clock. | If Demand mode is active, and a period of time equal to the Detection Time passes after the initiation of a Poll Sequence (the transmission of the first BFD Control packet with the Poll bit set), the session has gone down -- the local system MUST set bfd.SessionState to Down, and bfd.LocalDiag to 1 (Control Detection Time Expired). |
| `6.8.6:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The lead-in to the reception procedure. Its steps are declared as RFC5880-6.8.6-1 through RFC5880-6.8.6-15, in the order the summary lists them, and site 6.8.6:3 maps the first. | When a BFD Control packet is received, the following procedure MUST be followed, in the order specified. |
| `6.8.6:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The stop-on-discard clause of the same lead-in. Every step it governs is a discard rule already declared, the first of which site 6.8.6:3 maps. | If the packet is discarded according to these rules, processing of the packet MUST cease at that point. |
| `6.8.6:10` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The no-session discard, which RFC5880-6.8.6-7 states as the second half of its own text. Site 6.8.6:9 maps that row. | If no session is found, the packet MUST be discarded. |
| `6.8.7:10` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The Demand bit precondition, restated here in the transmission rules. RFC5880-6.6-1 cites section 6.6 and section 6.8.7, and site 6.6:1 maps it. | A system MUST NOT set the Demand (D) bit unless bfd.DemandMode is 1, bfd.SessionState is Up, and bfd.RemoteSessionState is Up. |
| `6.8.18:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The section 6.8.1 state preservation obligation, restated here at REQUIRED as the first of the two holddown mechanisms. Site 6.8.1:1 maps RFC5880-6.8.1-14. | First, a system is REQUIRED to maintain session state (including timing parameters), even when a session is down, until a Detection Time has passed without the receipt of any BFD Control packets. |

## Superseded

No document obsoletes RFC 5880, so its obligations are stated where they were written.
