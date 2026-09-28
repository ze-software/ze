# RFC 5880 - Bidirectional Forwarding Detection (BFD)

Partial. Every requirement this repository extracted from RFC 5880, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 79.6% | 78 of 98 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 7.1% | 7 of 98 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 98 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 20.0% | 38 of 190 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 98 | of 121 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 1 | of 98 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 1.0% | 1 of 98 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 98 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 98 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 12.2% | 12 of 98 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Audit verdicts | 84 | of 98 gated MUSTs judged | 50 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

The 7 shares marked as a part above are the whole of the 98 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
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
| Requirements | 121 |
| Gated MUST-level | 98 |
| Not applicable, so out of scope | 1 |
| Declared gaps | 12 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 190 |
| Tagged units | 190 |
| Recorded audit verdicts | 84 |
| Discrimination records | 38 |
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

Twelve MUST gaps, gated in [`rfc/short/rfc5880.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc5880.md): Demand mode is not driven -- bfd.DemandMode has no writer and the stored remote D bit is never read, so no Poll is raised on a D-bit or content change and periodic transmission is never suppressed ([`RFC5880-6.6-2`](#rfc5880-6.6-2), 6.6-3, 6.8.6-14, 6.8.7-7, 6.8.17-1); periodic transmission also continues when bfd.RemoteMinRxInterval is zero ([`RFC5880-6.8.7-6`](#rfc5880-6.8.7-6)); a reduced bfd.RequiredMinRxInterval enters the Detection Time at once instead of at Poll termination ([`RFC5880-6.8.3-4`](#rfc5880-6.8.3-4)); bfd.XmitAuthSeq starts at zero rather than a random value and bfd.AuthSeqKnown is never cleared after twice the Detection Time ([`RFC5880-6.8.1-11`](#rfc5880-6.8.1-11), 6.8.1-13); there is no forwarding-plane-reset hook, so diagnostic 4 has no producer ([`RFC5880-6.8.15-1`](#rfc5880-6.8.15-1)); and no congestion-control mechanism governs the transmit rate ([`RFC5880-7-1`](#rfc5880-7-1), 7-2). Final-packet rate limiting is absent by design and annotated not-applicable. IPv6 transport coverage is tracked with BFD.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 78 | one part of the gated population |
| Annotated instead of tested | 20 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **98** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (78):** [`RFC5880-4.1-1`](#rfc5880-4.1-1), [`RFC5880-4.1-2`](#rfc5880-4.1-2), [`RFC5880-6.3-1`](#rfc5880-6.3-1), [`RFC5880-6.8.1-3`](#rfc5880-6.8.1-3), [`RFC5880-6.8.1-4`](#rfc5880-6.8.1-4), [`RFC5880-6.8.1-5`](#rfc5880-6.8.1-5), [`RFC5880-6.8.1-6`](#rfc5880-6.8.1-6), [`RFC5880-6.8.1-7`](#rfc5880-6.8.1-7), [`RFC5880-6.8.1-8`](#rfc5880-6.8.1-8), [`RFC5880-6.8.1-9`](#rfc5880-6.8.1-9), [`RFC5880-6.8.1-10`](#rfc5880-6.8.1-10), [`RFC5880-6.8.1-12`](#rfc5880-6.8.1-12), [`RFC5880-6.8.1-14`](#rfc5880-6.8.1-14), [`RFC5880-6.1-1`](#rfc5880-6.1-1), [`RFC5880-6.1-2`](#rfc5880-6.1-2), [`RFC5880-6.1-3`](#rfc5880-6.1-3), [`RFC5880-6.8.3-1`](#rfc5880-6.8.3-1), [`RFC5880-6.8.3-2`](#rfc5880-6.8.3-2), [`RFC5880-6.8.3-3`](#rfc5880-6.8.3-3), [`RFC5880-6.8.3-5`](#rfc5880-6.8.3-5), [`RFC5880-6.8.3-6`](#rfc5880-6.8.3-6), [`RFC5880-6.5-1`](#rfc5880-6.5-1), [`RFC5880-6.5-2`](#rfc5880-6.5-2), [`RFC5880-6.6-1`](#rfc5880-6.6-1), [`RFC5880-6.7-1`](#rfc5880-6.7-1), [`RFC5880-4.2-1`](#rfc5880-4.2-1), [`RFC5880-6.7.2-1`](#rfc5880-6.7.2-1), [`RFC5880-6.7.2-8`](#rfc5880-6.7.2-8), [`RFC5880-6.7.3-1`](#rfc5880-6.7.3-1), [`RFC5880-6.7.3-2`](#rfc5880-6.7.3-2), [`RFC5880-6.7.3-4`](#rfc5880-6.7.3-4), [`RFC5880-6.7.3-13`](#rfc5880-6.7.3-13), [`RFC5880-6.7.4-1`](#rfc5880-6.7.4-1), [`RFC5880-6.7.4-2`](#rfc5880-6.7.4-2), [`RFC5880-6.7.4-4`](#rfc5880-6.7.4-4), [`RFC5880-6.7.4-5`](#rfc5880-6.7.4-5), [`RFC5880-6.7.4-7`](#rfc5880-6.7.4-7), [`RFC5880-6.8.6-1`](#rfc5880-6.8.6-1), [`RFC5880-6.8.6-2`](#rfc5880-6.8.6-2), [`RFC5880-6.8.6-3`](#rfc5880-6.8.6-3), [`RFC5880-6.8.6-4`](#rfc5880-6.8.6-4), [`RFC5880-6.8.6-5`](#rfc5880-6.8.6-5), [`RFC5880-6.8.6-6`](#rfc5880-6.8.6-6), [`RFC5880-6.8.6-7`](#rfc5880-6.8.6-7), [`RFC5880-6.8.6-8`](#rfc5880-6.8.6-8), [`RFC5880-6.8.6-9`](#rfc5880-6.8.6-9), [`RFC5880-6.8.6-10`](#rfc5880-6.8.6-10), [`RFC5880-6.8.6-11`](#rfc5880-6.8.6-11), [`RFC5880-6.8.6-12`](#rfc5880-6.8.6-12), [`RFC5880-6.8.6-13`](#rfc5880-6.8.6-13), [`RFC5880-6.8.6-15`](#rfc5880-6.8.6-15), [`RFC5880-6.8.6-16`](#rfc5880-6.8.6-16), [`RFC5880-6.8.6-18`](#rfc5880-6.8.6-18), [`RFC5880-6.8.7-1`](#rfc5880-6.8.7-1), [`RFC5880-6.8.7-2`](#rfc5880-6.8.7-2), [`RFC5880-6.8.7-3`](#rfc5880-6.8.7-3), [`RFC5880-6.8.7-4`](#rfc5880-6.8.7-4), [`RFC5880-6.8.7-5`](#rfc5880-6.8.7-5), [`RFC5880-6.8.4-1`](#rfc5880-6.8.4-1), [`RFC5880-6.8.5-1`](#rfc5880-6.8.5-1), [`RFC5880-6.8.16-1`](#rfc5880-6.8.16-1), [`RFC5880-6.8.8-1`](#rfc5880-6.8.8-1), [`RFC5880-6.8.8-2`](#rfc5880-6.8.8-2), [`RFC5880-6.8.9-1`](#rfc5880-6.8.9-1), [`RFC5880-6.8.9-2`](#rfc5880-6.8.9-2), [`RFC5880-6.8.9-3`](#rfc5880-6.8.9-3), [`RFC5880-9-2`](#rfc5880-9-2), [`RFC5880-6.7.3-7`](#rfc5880-6.7.3-7), [`RFC5880-6.7.2-3`](#rfc5880-6.7.2-3), [`RFC5880-6.7.3-8`](#rfc5880-6.7.3-8), [`RFC5880-6.7.2-4`](#rfc5880-6.7.2-4), [`RFC5880-6.7.2-5`](#rfc5880-6.7.2-5), [`RFC5880-6.7.2-6`](#rfc5880-6.7.2-6), [`RFC5880-6.7.2-7`](#rfc5880-6.7.2-7), [`RFC5880-6.7.3-9`](#rfc5880-6.7.3-9), [`RFC5880-6.7.3-10`](#rfc5880-6.7.3-10), [`RFC5880-6.7.3-11`](#rfc5880-6.7.3-11), [`RFC5880-6.7.3-12`](#rfc5880-6.7.3-12)

**Annotated instead of tested (20):** [`RFC5880-6.8.1-1`](#rfc5880-6.8.1-1), [`RFC5880-6.8.1-2`](#rfc5880-6.8.1-2), [`RFC5880-6.8.1-11`](#rfc5880-6.8.1-11), [`RFC5880-6.8.1-13`](#rfc5880-6.8.1-13), [`RFC5880-6.8.3-4`](#rfc5880-6.8.3-4), [`RFC5880-6.6-2`](#rfc5880-6.6-2), [`RFC5880-6.6-3`](#rfc5880-6.6-3), [`RFC5880-6.7.3-3`](#rfc5880-6.7.3-3), [`RFC5880-6.7.4-3`](#rfc5880-6.7.4-3), [`RFC5880-6.8.6-14`](#rfc5880-6.8.6-14), [`RFC5880-6.8.7-6`](#rfc5880-6.8.7-6), [`RFC5880-6.8.7-7`](#rfc5880-6.8.7-7), [`RFC5880-6.8.7-8`](#rfc5880-6.8.7-8), [`RFC5880-6.8.15-1`](#rfc5880-6.8.15-1), [`RFC5880-7-1`](#rfc5880-7-1), [`RFC5880-7-2`](#rfc5880-7-2), [`RFC5880-4.3-1`](#rfc5880-4.3-1), [`RFC5880-6.8.14-1`](#rfc5880-6.8.14-1), [`RFC5880-6.8.17-1`](#rfc5880-6.8.17-1), [`RFC5880-6.8.3-8`](#rfc5880-6.8.3-8)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC5880-4.1-1` | The contents of transmitted BFD Control packets MUST be set as follows: Version Set to the current version number (1). (§6.8.7) | MUST | 6.8.7 - Transmitting BFD Control Packets | **positive:** `unit/verify` [`TestRFC5880VersionOneAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/packet/rfc5880_test.go#L44). **negative:** `unit/verify` [`TestRFC5880VersionNotOneDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/packet/rfc5880_test.go#L64) |
| `RFC5880-4.1-2` | It MUST be zero on both transmit and receipt. (§4.1) | MUST | 4.1 - Generic BFD Control Packet Format | **positive:** `unit/verify` [`TestRFC5880MultipointZeroAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/packet/rfc5880_test.go#L80). **negative:** `unit/verify` [`TestRFC5880MultipointSetDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/packet/rfc5880_test.go#L96) |
| `RFC5880-6.3-1` | Each system MUST choose an opaque discriminator value that identifies each session, and which MUST be unique among all BFD sessions on the system. (§6.3) | MUST | 6.3 - Demultiplexing and the Discriminator Fields | **positive:** `unit/verify` [`TestRFC5880DiscriminatorsAreNonZeroAndUnique`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L62). **negative:** `unit/verify` [`TestRFC5880DiscriminatorAllocatorSkipsReservedAndTaken`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L90) |
| `RFC5880-6.8.1-1` | This variable MUST be initialized to Down. (§6.8.1) | MUST | 6.8.1 - State Variables | **positive:** `unit/verify` [`TestRFC5880InitStatesAreDown`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L73). **negative:** no negative test. **{single-polarity}:** Init assigns bfd.SessionState = Down unconditionally (internal/component/bfd/session/session.go:246) before any packet can be exchanged, so no non-conformant input exists to reject |
| `RFC5880-6.8.1-2` | This variable MUST be initialized to Down. (§6.8.1) | MUST | 6.8.1 - State Variables | **positive:** `unit/verify` [`TestRFC5880InitStatesAreDown`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L76). **negative:** no negative test. **{single-polarity}:** Init assigns bfd.RemoteSessionState = Down unconditionally (internal/component/bfd/session/session.go:247), so there is no non-conformant input to reject |
| `RFC5880-6.8.1-3` | It MUST be unique across all BFD sessions on this system, and nonzero. (§6.8.1) | MUST | 6.8.1 - State Variables | **positive:** `unit/verify` [`TestRFC5880DiscriminatorsAreNonZeroAndUnique`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L67). **negative:** `unit/verify` [`TestRFC5880DiscriminatorAllocatorSkipsReservedAndTaken`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L95) |
| `RFC5880-6.8.1-4` | This MUST be initialized to zero. (§6.8.1) | MUST | 6.8.1 - State Variables | **positive:** `unit/verify` [`TestRFC5880InitVariableDefaults`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L88). **negative:** `unit/verify` [`TestRFC5880InitVariablesAreNotConstants`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L113) |
| `RFC5880-6.8.1-5` | If a period of a Detection Time passes without the receipt of a valid, authenticated BFD packet from the remote system, this variable MUST be set to zero. (§6.8.1) | MUST | 6.8.1 - State Variables | **positive:** `unit/verify` [`TestRFC5880RemoteDiscrClearedAfterSilenceWhileDown`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L1215). **positive:** `unit/verify` [`TestRFC5880RemoteDiscrClearedOnDetectionExpiry`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L153). **negative:** `unit/verify` [`TestRFC5880RemoteDiscrKeptOnNeighborSignaledDown`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L180) |
| `RFC5880-6.8.1-6` | This MUST be initialized to zero (No Diagnostic). (§6.8.1) | MUST | 6.8.1 - State Variables | **positive:** `unit/verify` [`TestRFC5880InitVariableDefaults`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L90). **negative:** `unit/verify` [`TestRFC5880InitVariablesAreNotConstants`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L116) |
| `RFC5880-6.8.1-7` | This MUST be initialized to a value of at least one second (1,000,000 microseconds) according to the rules described in section 6.8.3. (§6.8.1, §6.8.3) | MUST | 6.8.1 - State Variables | **positive:** `unit/verify` [`TestRFC5880SlowStartFloorWhileNotUp`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L208). **negative:** `unit/verify` [`TestRFC5880SlowStartFloorLiftedWhenUp`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L228) |
| `RFC5880-6.8.1-8` | The last value of Required Min RX Interval received from the remote system in a BFD Control packet. This variable MUST be initialized to 1. (§6.8.1) | MUST | 6.8.1 - State Variables | **positive:** `unit/verify` [`TestRFC5880InitVariableDefaults`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L92). **negative:** `unit/verify` [`TestRFC5880InitVariablesAreNotConstants`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L118) |
| `RFC5880-6.8.1-9` | This variable MUST be initialized to zero. (§6.8.1) | MUST | 6.8.1 - State Variables | **positive:** `unit/verify` [`TestRFC5880InitVariableDefaults`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L94). **negative:** `unit/verify` [`TestRFC5880InitVariablesAreNotConstants`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L121) |
| `RFC5880-6.8.1-10` | This variable MUST be a nonzero integer, and is otherwise outside the scope of this specification. (§6.8.1) | MUST | 6.8.1 - State Variables | **positive:** `unit/verify` [`TestRFC5880DetectMultConfiguredValue`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L248). **negative:** `unit/verify` [`TestRFC5880DetectMultZeroRequestSubstituted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L259) |
| `RFC5880-6.8.1-11` | This variable MUST be initialized to a random 32-bit value. (§6.8.1) | MUST | 6.8.1 - State Variables | **positive:** no positive test. **negative:** no negative test. **{gap}:** SetAuth seeds bfd.XmitAuthSeq from the persister or leaves the Vars zero value (internal/component/bfd/session/auth.go:36) and Init never randomizes it (internal/component/bfd/session/session.go:245-259), so the initial transmit sequence is 0 rather than a random 32-bit value |
| `RFC5880-6.8.1-12` | This variable MUST be initialized to zero. (§6.8.1) | MUST | 6.8.1 - State Variables | **positive:** `unit/verify` [`TestRFC5880AuthSeqKnownStartsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L426). **negative:** `unit/verify` [`TestRFC5880AuthSeqKnownSetAfterFirstPacket`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L444) |
| `RFC5880-6.8.1-13` | This variable MUST be set to zero after no packets have been received on this session for at least twice the Detection Time. (§6.8.1) | MUST | 6.8.1 - State Variables | **positive:** no positive test. **negative:** no negative test. **{gap}:** bfd.AuthSeqKnown is SeqState.initialized, which only Advance sets (internal/component/bfd/auth/meticulous.go:63-66) and nothing ever clears; CheckDetection clears the detection deadline alone (internal/component/bfd/session/timers.go:82), so the flag survives twice the Detection Time of silence |
| `RFC5880-6.8.1-14` | Once session state is created, and at least one BFD Control packet is received from the remote end, it MUST be preserved for at least one Detection Time (see section 6.8.4) subsequent to the receipt of the last BFD Control packet, regardless of the session state. (§6.8.1) | MUST | 6.8.1 - State Variables | **positive:** `unit/verify` [`TestRFC5880StatePreservedForOneDetectionTime`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L283). **negative:** `unit/verify` [`TestRFC5880DetectionExpiryDownDiagOne`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L308) |
| `RFC5880-6.1-1` | A system taking the Active role MUST send BFD Control packets for a particular session, regardless of whether it has received any BFD packets for that session. (§6.1) | MUST | 6.1 - Overview | **positive:** `unit/verify` [`TestRFC5880ActiveRoleTransmitsWithoutReception`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L361). **negative:** `unit/verify` [`TestRFC5880PassiveRoleTransmitsAfterReception`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L410) |
| `RFC5880-6.1-2` | A system taking the Passive role MUST NOT begin sending BFD packets for a particular session until it has received a BFD packet for that session, and thus has learned the remote system's discriminator value. (§6.1) | MUST NOT | 6.1 - Overview | **positive:** `unit/verify` [`TestRFC5880PassiveRoleSilentUntilReception`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L386). **negative:** `unit/verify` [`TestRFC5880PassiveRoleTransmitsAfterReception`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L415) |
| `RFC5880-6.1-3` | At least one system MUST take the Active role (possibly both). (§6.1) | MUST | 6.1 - Overview | **positive:** `unit/verify` [`TestRFC5880ActiveRoleTransmitsWithoutReception`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L366). **negative:** `unit/verify` [`TestRFC5880PassiveRoleSilentUntilReception`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L394) |
| `RFC5880-6.8.3-1` | When bfd.SessionState is not Up, the system MUST set bfd.DesiredMinTxInterval to a value of not less than one second (1,000,000 microseconds). (§6.8.3) | MUST | 6.8.3 - Timer Manipulation | **positive:** `unit/verify` [`TestRFC5880SlowStartFloorRestoredOnAdminDown`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L1253). **positive:** `unit/verify` [`TestRFC5880SlowStartFloorWhileNotUp`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L213). **negative:** `unit/verify` [`TestRFC5880SlowStartFloorLiftedWhenUp`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L232) |
| `RFC5880-6.8.3-2` | If either bfd.DesiredMinTxInterval is changed or bfd.RequiredMinRxInterval is changed, a Poll Sequence MUST be initiated (see section 6.5). (§6.8.3) | MUST | 6.8.3 - Timer Manipulation | **positive:** `unit/verify` [`TestRFC5880PollInitiatedOnIntervalChange`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L436). **negative:** `unit/verify` [`TestRFC5880NoPollWhenIntervalsUnchanged`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L470) |
| `RFC5880-6.8.3-3` | If bfd.DesiredMinTxInterval is increased and bfd.SessionState is Up, the actual transmission interval used MUST NOT change until the Poll Sequence described above has terminated. (§6.8.3) | MUST NOT | 6.8.3 - Timer Manipulation | **positive:** `unit/verify` [`TestRFC5880RaisedDesiredMinTxHeldUntilPollTerminates`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L1458). **negative:** `unit/verify` [`TestRFC5880RaisedDesiredMinTxAppliedAtFinal`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L1517) |
| `RFC5880-6.8.3-4` | If bfd.RequiredMinRxInterval is reduced and bfd.SessionState is Up, the previous value of bfd.RequiredMinRxInterval MUST be used when calculating the Detection Time for the remote system until the Poll Sequence described above has terminated. (§6.8.3) | MUST | 6.8.3 - Timer Manipulation | **positive:** no positive test. **negative:** no negative test. **{gap}:** revertEchoSlowdownLocked reduces bfd.RequiredMinRxInterval while Up (internal/component/bfd/session/timers.go:249) and DetectionInterval immediately uses the reduced value (internal/component/bfd/session/timers.go:29); no previous value is retained until the Poll Sequence terminates |
| `RFC5880-6.8.3-5` | If the local system reduces its transmit interval due to bfd.RemoteMinRxInterval being reduced (the remote system has advertised a reduced value in Required Min RX Interval), and the remote system is not in Demand mode, the local system MUST honor the new interval immediately. (§6.8.3) | MUST | 6.8.3 - Timer Manipulation | **positive:** `unit/verify` [`TestRFC5880RemoteMinRxChangeReschedulesNextTx`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L1284). **positive:** `unit/verify` [`TestRFC5880RemoteMinRxReductionHonoredImmediately`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L497). **negative:** `unit/verify` [`TestRFC5880TransmitIntervalFlooredByLocalDesired`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L533). **negative:** `unit/verify` [`TestRFC5880UnchangedRemoteMinRxKeepsNextTx`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L1372) |
| `RFC5880-6.8.3-6` | Therefore, if multiple changes are made that require the use of a Poll Sequence, there are three choices: 1) they MUST be communicated in a single BFD Control packet (so the semantics of the Final reply are clear), or 2) sufficient time must have transpired since the Poll Sequence was completed to disambiguate the situation (at least a round trip time since the last Poll was transmitted) prior to the initiation of another Poll Sequence, or 3) an additional BFD Control packet with the Final (F) bit *clear* MUST be received after the Poll Sequence has completed prior to the initiation of another Poll Sequence (this option is not available when Demand mode is active). (§6.8.3) | MUST | 6.8.3 - Timer Manipulation | **positive:** `unit/verify` [`TestRFC5880EchoSlowdownPollsAfterSettledPoll`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L726). **positive:** `unit/verify` [`TestRFC5880PollInitiatedOnIntervalChange`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L441). **negative:** `unit/verify` [`TestRFC5880EchoSlowdownRevertWaitsForOutstandingPoll`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L755). **negative:** `unit/verify` [`TestRFC5880EchoSlowdownWaitsForOutstandingPoll`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L689) |
| `RFC5880-6.5-1` | A BFD Control packet MUST NOT have both the Poll (P) and Final (F) bits set. (§6.5) | MUST NOT | 6.5 - The Poll Sequence | **positive:** `unit/verify` [`TestRFC5880PollPacketHasNoFinalBit`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L567). **negative:** `unit/verify` [`TestRFC5880FinalReplyClearsPollBit`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L586) |
| `RFC5880-6.5-2` | If periodic BFD Control packets are already being sent (the remote system is not in Demand mode), the Poll Sequence MUST be performed by setting the Poll (P) bit on those scheduled periodic transmissions; additional packets MUST NOT be sent. (§6.5) | MUST | 6.5 - The Poll Sequence | **positive:** `unit/verify` [`TestRFC5880PollPacketHasNoFinalBit`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L571). **negative:** `unit/verify` [`TestRFC5880FinalReplyClearsPollBit`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L590) |
| `RFC5880-6.6-1` | A system MUST NOT set the Demand (D) bit unless bfd.DemandMode is 1, bfd.SessionState is Up, and bfd.RemoteSessionState is Up. (§6.8.7) | MUST NOT | 6.8.7 - Transmitting BFD Control Packets | **positive:** `unit/verify` [`TestRFC5880DemandBitSetWhenAllConditionsHold`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L652). **negative:** `unit/verify` [`TestRFC5880DemandBitClearWhenAnyConditionFails`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L666) |
| `RFC5880-6.6-2` | When the transmitted value of the Demand (D) bit is to be changed, the transmitting system MUST initiate a Poll Sequence in conjunction with changing the bit in order to ensure that both systems are aware of the change. (§6.6) | MUST | 6.6 - Demand mode | **positive:** no positive test. **negative:** no negative test. **{gap}:** bfd.DemandMode has no writer in production code -- it is declared at internal/component/bfd/session/session.go:58 and read only by canSetDemand (internal/component/bfd/session/fsm.go:247) -- so no D-bit change path exists and none initiates a Poll Sequence |
| `RFC5880-6.6-3` | If Demand mode is active on either or both systems, a Poll Sequence MUST be initiated whenever the contents of the next BFD Control packet to be sent would be different than the contents of the previous packet, with the exception of the Poll (P) and Final (F) bits. (§6.6) | MUST | 6.6 - Demand mode | **positive:** no positive test. **negative:** no negative test. **{gap}:** bfd.RemoteDemandMode is stored by Receive (internal/component/bfd/session/fsm.go:58) and read nowhere, and bfd.DemandMode has no writer (internal/component/bfd/session/session.go:58), so no code path starts a Poll when packet contents would change while Demand mode is active |
| `RFC5880-6.7-1` | Implementations supporting authentication MUST support both types of SHA1 authentication. (§6.7) | MUST | 6.7 - Authentication | **positive:** `unit/verify` [`TestRFC5880BothSHA1VariantsSupported`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L51). **negative:** `unit/verify` [`TestRFC5880UnsupportedAuthTypesRejected`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L70) |
| `RFC5880-4.2-1` | The password is a binary string, and MUST be from 1 to 16 bytes in length. (§4.2, §6.7.2) | MUST | 4.2 - Simple Password Authentication Section Format | **positive:** `unit/verify` [`TestRFC5880SimplePasswordManagementAcceptsASCIIStrings`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5880_test.go#L101). **positive:** `unit/verify` [`TestRFC5880SimplePasswordSectionHeader`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L635). **negative:** `unit/verify` [`auth/TestRFC5880SimplePasswordLengthOutOfRangeRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L790). **negative:** `unit/verify` [`bfd/TestRFC5880SimplePasswordLengthOutOfRangeRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5880_test.go#L128) |
| `RFC5880-6.7.2-1` | For interoperability, the management interface by which the password is configured MUST accept ASCII strings (§6.7.2) | MUST | 6.7.2 - Simple Password Authentication | **positive:** `unit/verify` [`TestRFC5880SimplePasswordManagementAcceptsASCIIStrings`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5880_test.go#L96). **negative:** `unit/verify` [`bfd/TestRFC5880SimplePasswordLengthOutOfRangeRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5880_test.go#L132) |
| `RFC5880-6.7.2-8` | The Auth Type field MUST be set to 1 (Simple Password). The Auth Len field MUST be set to the proper length (4 to 19 bytes). (§6.7.2) | MUST | 6.7.2 - Simple Password Authentication | **positive:** `unit/verify` [`TestRFC5880SimplePasswordSectionHeader`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L630). **negative:** `unit/verify` [`TestRFC5880SimplePasswordRejectsForeignSectionShape`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L656) |
| `RFC5880-6.7.3-1` | The Auth Type field MUST be set to 2 (Keyed MD5) or 3 (Meticulous Keyed MD5). The Auth Len field MUST be set to 24. (§6.7.3) | MUST | 6.7.3 - Keyed MD5 and Meticulous Keyed MD5 Authentication | **positive:** `unit/verify` [`TestRFC5880KeyedMD5SectionHeader`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L86). **negative:** `unit/verify` [`TestRFC5880KeyedMD5RejectsForeignSectionShape`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L113) |
| `RFC5880-6.7.3-2` | An MD5 digest MUST be calculated over the entire BFD Control packet. (§6.7.3) | MUST | 6.7.3 - Keyed MD5 and Meticulous Keyed MD5 Authentication | **positive:** `unit/verify` [`TestRFC5880DigestCoversWholePacket`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L182). **negative:** `unit/verify` [`TestRFC5880DigestRejectsMandatorySectionTamper`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L201) |
| `RFC5880-6.7.3-3` | replacing the secret key, which MUST NOT be carried in the packet (§6.7.3) | MUST NOT | 6.7.3 - Keyed MD5 and Meticulous Keyed MD5 Authentication | **positive:** `unit/verify` [`TestRFC5880SecretKeyNotCarriedInPacket`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L225). **negative:** no negative test. **{single-polarity}:** Sign overwrites the key scratch with the computed digest before the packet is handed to the transport (internal/component/bfd/auth/sha1.go:85-87), so no key-bearing packet is ever emitted for a receiver to reject |
| `RFC5880-6.7.3-4` | For Meticulous Keyed MD5, bfd.XmitAuthSeq MUST be incremented in a circular fashion (when treated as an unsigned 32-bit value). (§6.7.3) | MUST | 6.7.3 - Keyed MD5 and Meticulous Keyed MD5 Authentication | **positive:** `unit/verify` [`TestRFC5880MeticulousSequenceIncrementsPerPacket`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L287). **negative:** `unit/verify` [`TestRFC5880MeticulousRejectsUnincrementedSequence`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L582) |
| `RFC5880-6.7.3-13` | The authentication key value is a binary string of up to 16 bytes, and MUST be placed into the Auth Key/Digest field, padded with trailing zero bytes as necessary. (§6.7.3) | MUST | 6.7.3 - Keyed MD5 and Meticulous Keyed MD5 Authentication | **positive:** `unit/verify` [`TestKeyedSecretLongerThanSlotRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L931). **positive:** `unit/verify` [`TestParseAuthConfigKeyedSecretLength`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5881_test.go#L145). **positive:** `unit/verify` [`TestRFC5880ShortKeyZeroPaddedIntoDigestField`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L977). **negative:** `unit/verify` [`TestKeyedSecretLongerThanSlotRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L934). **negative:** `unit/verify` [`TestParseAuthConfigKeyedSecretLength`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5881_test.go#L148) |
| `RFC5880-6.7.4-1` | The Auth Type field MUST be set to 4 (Keyed SHA1) or 5 (Meticulous Keyed SHA1).  The Auth Len field MUST be set to 28. (§6.7.4) | MUST | 6.7.4 - Keyed SHA1 and Meticulous Keyed SHA1 Authentication | **positive:** `unit/verify` [`TestRFC5880KeyedSHA1SectionHeader`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L138). **negative:** `unit/verify` [`TestRFC5880KeyedSHA1RejectsForeignSectionShape`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L159) |
| `RFC5880-6.7.4-2` | A SHA1 hash MUST be calculated over the entire BFD control packet. (§6.7.4) | MUST | 6.7.4 - Keyed SHA1 and Meticulous Keyed SHA1 Authentication | **positive:** `unit/verify` [`TestRFC5880DigestCoversWholePacket`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L188). **negative:** `unit/verify` [`TestRFC5880DigestRejectsMandatorySectionTamper`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L206) |
| `RFC5880-6.7.4-3` | the secret key, which MUST NOT be carried in the packet (§6.7.4) | MUST NOT | 6.7.4 - Keyed SHA1 and Meticulous Keyed SHA1 Authentication | **positive:** `unit/verify` [`TestRFC5880SecretKeyNotCarriedInPacket`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L229). **negative:** no negative test. **{single-polarity}:** the SHA1 signer shares that producer with a 20-byte digest slot (internal/component/bfd/auth/sha1.go:85-87,193-195), so no key-bearing packet is emitted |
| `RFC5880-6.7.4-4` | For Meticulous Keyed SHA1, bfd.XmitAuthSeq MUST be incremented in a circular fashion (when treated as an unsigned 32-bit value). (§6.7.4) | MUST | 6.7.4 - Keyed SHA1 and Meticulous Keyed SHA1 Authentication | **positive:** `unit/verify` [`TestRFC5880MeticulousSequenceIncrementsPerPacket`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L293). **negative:** `unit/verify` [`TestRFC5880MeticulousRejectsUnincrementedSequence`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L587) |
| `RFC5880-6.7.4-5` | the management interface by which the key is configured MUST accept ASCII strings (§6.7.4) | MUST | 6.7.4 - Keyed SHA1 and Meticulous Keyed SHA1 Authentication | **positive:** `unit/verify` [`TestRFC5880KeyManagementAcceptsASCIIStrings`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5880_test.go#L31). **negative:** `unit/verify` [`TestRFC5880KeyManagementRejectsIncompleteConfig`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5880_test.go#L70) |
| `RFC5880-6.7.4-7` | The authentication key value is a binary string of up to 20 bytes, and MUST be placed into the Auth Key/Hash field, padding with trailing zero bytes as necessary. (§6.7.4) | MUST | 6.7.4 - Keyed SHA1 and Meticulous Keyed SHA1 Authentication | **positive:** `unit/verify` [`TestKeyedSecretLongerThanSlotRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L937). **positive:** `unit/verify` [`TestParseAuthConfigKeyedSecretLength`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5881_test.go#L150). **positive:** `unit/verify` [`TestRFC5880ShortKeyZeroPaddedIntoDigestField`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L983). **negative:** `unit/verify` [`TestKeyedSecretLongerThanSlotRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L940). **negative:** `unit/verify` [`TestParseAuthConfigKeyedSecretLength`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5881_test.go#L152) |
| `RFC5880-6.8.6-1` | If the version number is not correct (1), the packet MUST be discarded. (§6.8.6) | MUST | 6.8.6 - Reception of BFD Control Packets | **positive:** `unit/verify` [`TestRFC5880VersionOneAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/packet/rfc5880_test.go#L48). **negative:** `unit/verify` [`TestRFC5880VersionNotOneDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/packet/rfc5880_test.go#L66) |
| `RFC5880-6.8.6-2` | If the Length field is less than the minimum correct value (24 if the A bit is clear, or 26 if the A bit is set), the packet MUST be discarded. (§6.8.6) | MUST | 6.8.6 - Reception of BFD Control Packets | **positive:** `unit/verify` [`TestRFC5880LengthMinimumAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/packet/rfc5880_test.go#L108). **negative:** `unit/verify` [`TestRFC5880LengthBelowMinimumDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/packet/rfc5880_test.go#L128) |
| `RFC5880-6.8.6-3` | If the Length field is greater than the payload of the encapsulating protocol, the packet MUST be discarded. (§6.8.6) | MUST | 6.8.6 - Reception of BFD Control Packets | **positive:** `unit/verify` [`TestRFC5880LengthEqualsPayloadAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/packet/rfc5880_test.go#L148). **negative:** `unit/verify` [`TestRFC5880LengthOverPayloadDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/packet/rfc5880_test.go#L163) |
| `RFC5880-6.8.6-4` | If the Detect Mult field is zero, the packet MUST be discarded. (§6.8.6) | MUST | 6.8.6 - Reception of BFD Control Packets | **positive:** `unit/verify` [`TestRFC5880DetectMultNonZeroAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/packet/rfc5880_test.go#L174). **negative:** `unit/verify` [`TestRFC5880DetectMultZeroDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/packet/rfc5880_test.go#L190) |
| `RFC5880-6.8.6-5` | If the Multipoint (M) bit is nonzero, the packet MUST be discarded. (§6.8.6) | MUST | 6.8.6 - Reception of BFD Control Packets | **positive:** `unit/verify` [`TestRFC5880MultipointZeroAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/packet/rfc5880_test.go#L84). **negative:** `unit/verify` [`TestRFC5880MultipointSetDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/packet/rfc5880_test.go#L99) |
| `RFC5880-6.8.6-6` | If the My Discriminator field is zero, the packet MUST be discarded. (§6.8.6) | MUST | 6.8.6 - Reception of BFD Control Packets | **positive:** `unit/verify` [`TestRFC5880MyDiscriminatorNonZeroAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/packet/rfc5880_test.go#L200). **negative:** `unit/verify` [`TestRFC5880MyDiscriminatorZeroDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/packet/rfc5880_test.go#L214) |
| `RFC5880-6.8.6-7` | If the Your Discriminator field is nonzero, it MUST be used to select the session with which this BFD packet is associated.  If no session is found, the packet MUST be discarded. (§6.8.6) | MUST | 6.8.6 - Reception of BFD Control Packets | **positive:** `unit/verify` [`TestRFC5880YourDiscriminatorSelectsSession`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L124). **negative:** `unit/verify` [`TestRFC5880UnknownYourDiscriminatorDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L140) |
| `RFC5880-6.8.6-8` | If the Your Discriminator field is zero and the State field is not Down or AdminDown, the packet MUST be discarded. (§6.8.6) | MUST | 6.8.6 - Reception of BFD Control Packets | **positive:** `unit/verify` [`TestRFC5880ZeroYourDiscriminatorAcceptedWhenDown`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L727). **negative:** `unit/verify` [`TestRFC5880ZeroYourDiscriminatorDiscardedWhenLive`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L699) |
| `RFC5880-6.8.6-9` | If the A bit is set and no authentication is in use (bfd.AuthType is zero), the packet MUST be discarded. (§6.8.6) | MUST | 6.8.6 - Reception of BFD Control Packets | **positive:** `unit/verify` [`TestRFC5880UnauthenticatedSessionAcceptsUnauthenticatedPacket`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L765). **negative:** `unit/verify` [`TestRFC5880UnauthenticatedSessionDiscardsAuthenticatedPacket`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L780) |
| `RFC5880-6.8.6-10` | If the A bit is clear and authentication is in use (bfd.AuthType is nonzero), the packet MUST be discarded. (§6.8.6) | MUST | 6.8.6 - Reception of BFD Control Packets | **positive:** `unit/verify` [`TestRFC5880AuthenticatedSessionAcceptsAuthenticatedPacket`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L797). **negative:** `unit/verify` [`TestRFC5880AuthenticatedSessionDiscardsUnauthenticatedPacket`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L815) |
| `RFC5880-6.8.6-11` | If the A bit is set, the packet MUST be authenticated under the rules of section 6.7, based on the authentication type in use (bfd.AuthType). (§6.8.6) | MUST | 6.8.6 - Reception of BFD Control Packets | **positive:** `unit/verify` [`TestRFC5880AuthenticatedPacketVerifiedAndDelivered`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L241). **negative:** `unit/verify` [`TestRFC5880UnauthenticPacketDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L258) |
| `RFC5880-6.8.6-12` | If the Required Min Echo RX Interval field is zero, the transmission of Echo packets, if any, MUST cease. (§6.8.6) | MUST | 6.8.6 - Reception of BFD Control Packets | **positive:** `unit/verify` [`TestRFC5880EchoCeasesWhenPeerAdvertisesZero`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L831). **negative:** `unit/verify` [`TestRFC5880EchoEnabledWhenPeerAdvertisesNonZero`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L868) |
| `RFC5880-6.8.6-13` | If a Poll Sequence is being transmitted by the local system and the Final (F) bit in the received packet is set, the Poll Sequence MUST be terminated. (§6.8.6) | MUST | 6.8.6 - Reception of BFD Control Packets | **positive:** `unit/verify` [`TestRFC5880FinalTerminatesPoll`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L610). **negative:** `unit/verify` [`TestRFC5880NonFinalDoesNotTerminatePoll`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L630) |
| `RFC5880-6.8.6-14` | If bfd.RemoteDemandMode is 1, bfd.SessionState is Up, and bfd.RemoteSessionState is Up, Demand mode is active on the remote system and the local system MUST cease the periodic transmission of BFD Control packets (see section 6.8.7). (§6.8.6) | MUST | 6.8.6 - Reception of BFD Control Packets | **positive:** no positive test. **negative:** no negative test. **{gap}:** tick transmits whenever the periodic deadline has passed and never consults the remote Demand state (internal/component/bfd/engine/loop.go:192-201), so periodic Control packets continue after the peer sets D=1 |
| `RFC5880-6.8.6-15` | If bfd.RemoteDemandMode is 0, or bfd.SessionState is not Up, or bfd.RemoteSessionState is not Up, Demand mode is not active on the remote system and the local system MUST send periodic BFD Control packets (see section 6.8.7). (§6.8.6) | MUST | 6.8.6 - Reception of BFD Control Packets | **positive:** `unit/verify` [`TestRFC5880PeriodicTransmitWhenRemoteDemandInactive`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L368). **negative:** `unit/verify` [`TestRFC5880NoPeriodicTransmitWhileAdminDown`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L403) |
| `RFC5880-6.8.6-16` | If a BFD Control packet is received with the Poll (P) bit set to 1, the receiving system MUST transmit a BFD Control packet with the Poll (P) bit clear and the Final (F) bit set as soon as practicable (§6.8.7) | MUST | 6.8.7 - Transmitting BFD Control Packets | **positive:** `unit/verify` [`TestRFC5880PollAnsweredWithImmediateFinal`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L322). **negative:** `unit/verify` [`TestRFC5880NonPollProducesNoImmediateReply`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L352) |
| `RFC5880-6.8.6-18` | If the Your Discriminator field is zero, the session MUST be selected based on some combination of other fields, possibly including source addressing information, the My Discriminator field, and the interface over which the packet was received. (§6.8.6) | MUST | 6.8.6 - Reception of BFD Control Packets | **positive:** `unit/verify` [`TestFirstPacketMatchesWhatTheTransportSurfaces`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5881_test.go#L363). **positive:** `unit/verify` [`TestRFC5881FirstPacketMatchesByTuple`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5881_test.go#L135). **negative:** `unit/verify` [`TestRFC5881FirstPacketWrongSourceDropped`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5881_test.go#L158) |
| `RFC5880-6.8.7-1` | With the exceptions listed in the remainder of this section, a system MUST NOT transmit BFD Control packets at an interval less than the larger of bfd.DesiredMinTxInterval and bfd.RemoteMinRxInterval, less applied jitter (see below). (§6.8.7) | MUST NOT | 6.8.7 - Transmitting BFD Control Packets | **positive:** `unit/verify` [`TestRFC5880TransmitDeadlineUsesNegotiatedInterval`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L1034). **negative:** `unit/verify` [`TestRFC5880TransmitDeadlineClampsBadJitter`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L1064) |
| `RFC5880-6.8.7-2` | The periodic transmission of BFD Control packets MUST be jittered on a per-packet basis by up to 25%, that is, the interval MUST be reduced by a random value of 0 to 25% (§6.8.7) | MUST | 6.8.7 - Transmitting BFD Control Packets | **positive:** `unit/verify` [`TestRFC5880JitterIsAppliedPerPacket`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L422). **negative:** `unit/verify` [`TestRFC5880JitterStaysWithinBand`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L450) |
| `RFC5880-6.8.7-3` | If bfd.DetectMult is equal to 1, the interval between transmitted BFD Control packets MUST be no more than 90% of the negotiated transmission interval, and MUST be no less than 75% of the negotiated transmission interval. (§6.8.7) | MUST | 6.8.7 - Transmitting BFD Control Packets | **positive:** `unit/verify` [`TestRFC5880JitterDetectMultOneWindow`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L470). **negative:** `unit/verify` [`TestRFC5880JitterFloorOnlyForDetectMultOne`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L488) |
| `RFC5880-6.8.7-4` | The transmit interval MUST be recalculated whenever bfd.DesiredMinTxInterval changes, or whenever bfd.RemoteMinRxInterval changes, and is equal to the greater of those two values. (§6.8.7) | MUST | 6.8.7 - Transmitting BFD Control Packets | **positive:** `unit/verify` [`TestRFC5880DesiredMinTxChangeReschedulesNextTx`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L1337). **positive:** `unit/verify` [`TestRFC5880RemoteMinRxChangeReschedulesNextTx`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L1290). **positive:** `unit/verify` [`TestRFC5880RemoteMinRxReductionHonoredImmediately`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L504). **negative:** `unit/verify` [`TestRFC5880TransmitIntervalFlooredByLocalDesired`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L538) |
| `RFC5880-6.8.7-5` | A system MUST NOT transmit BFD Control packets if bfd.RemoteDiscr is zero and the system is taking the Passive role. (§6.8.7) | MUST NOT | 6.8.7 - Transmitting BFD Control Packets | **positive:** `unit/verify` [`TestRFC5880PassiveRoleSilentUntilReception`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L391). **positive:** `unit/verify` [`TestRFC5880PassiveSilentOnceRemoteDiscrCleared`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L1396). **negative:** `unit/verify` [`TestRFC5880ActiveRoleTransmitsWithoutReception`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L369). **negative:** `unit/verify` [`TestRFC5880PassiveTransmitsAgainOnceRemoteDiscrLearned`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L1436) |
| `RFC5880-6.8.7-6` | A system MUST NOT periodically transmit BFD Control packets if bfd.RemoteMinRxInterval is zero. (§6.8.7) | MUST NOT | 6.8.7 - Transmitting BFD Control Packets | **positive:** no positive test. **negative:** no negative test. **{gap}:** TransmitInterval substitutes the slow-start interval when the negotiated maximum is zero (internal/component/bfd/session/timers.go:45-47) and tick carries no bfd.RemoteMinRxInterval == 0 suppression (internal/component/bfd/engine/loop.go:192-201), so periodic transmission continues |
| `RFC5880-6.8.7-7` | A system MUST NOT periodically transmit BFD Control packets if Demand mode is active on the remote system (bfd.RemoteDemandMode is 1, bfd.SessionState is Up, and bfd.RemoteSessionState is Up) and a Poll Sequence is not being transmitted. (§6.8.7) | MUST NOT | 6.8.7 - Transmitting BFD Control Packets | **positive:** no positive test. **negative:** no negative test. **{gap}:** the same tick producer (internal/component/bfd/engine/loop.go:192-201) has no remote-Demand suppression, so periodic transmission continues while the remote Demand mode is active with no Poll in flight |
| `RFC5880-6.8.7-8` | If rate limiting is in effect, the advertised value of Desired Min TX Interval MUST be greater than or equal to the interval between transmitted packets imposed by the rate limiting function. (§6.8.7) | MUST | 6.8.7 - Transmitting BFD Control Packets | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze does not rate-limit Final packets -- handleInbound sends the Final unconditionally on every received Poll (internal/component/bfd/engine/loop.go:140-142) and no rate limiter exists on that path, so the conditional advertised-interval floor never applies |
| `RFC5880-6.8.4-1` | If Demand mode is not active, and a period of time equal to the Detection Time passes without receiving a BFD Control packet from the remote system, and bfd.SessionState is Init or Up, the session has gone down -- the local system MUST set bfd.SessionState to Down and bfd.LocalDiag to 1 (Control Detection Time Expired). (§6.8.4) | MUST | 6.8.4 - Calculating the Detection Time | **positive:** `unit/verify` [`TestRFC5880DetectionExpiryDownDiagOne`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L312). **negative:** `unit/verify` [`TestRFC5880DetectionExpiryIgnoredWhenDown`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L333) |
| `RFC5880-6.8.5-1` | When the Echo function is active and a sufficient number of Echo packets have not arrived as they should, the session has gone down -- the local system MUST set bfd.SessionState to Down and bfd.LocalDiag to 2 (Echo Function Failed). (§6.8.5) | MUST | 6.8.5 - Detecting Failures with the Echo Function | **positive:** `unit/verify` [`TestRFC5880EchoMissDetectedAndReported`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L962). **negative:** `unit/verify` [`TestRFC5880EchoReturnClearsMissAndFailIsScoped`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L998) |
| `RFC5880-6.8.15-1` | When the forwarding plane in the local system is reset for some reason, such that the remote system can no longer rely on the local forwarding state, the local system MUST set bfd.LocalDiag to 4 (Forwarding Plane Reset), and set bfd.SessionState to Down. (§6.8.15) | MUST | 6.8.15 - Forwarding Plane Reset | **positive:** no positive test. **negative:** no negative test. **{gap}:** packet.DiagForwardingPlaneReset (internal/component/bfd/packet/diag.go:21) has no producer, and the only external state-forcing entry points are AdminDown and AdminEnable (internal/component/bfd/session/fsm.go:180,192), which move the session to AdminDown or Down carrying the caller's diagnostic and are never called with code 4 |
| `RFC5880-6.8.16-1` | There may be circumstances where it is desirable to administratively enable or disable a BFD session.  When this is desired, the following procedure MUST be followed (§6.8.16) | MUST | 6.8.16 - Administrative Control | **positive:** `unit/verify` [`TestRFC5880AdministrativeDisableEnable`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L1090). **negative:** `unit/verify` [`TestRFC5880AdministrativeCallsAreGuarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L1125) |
| `RFC5880-6.8.8-1` | A received BFD Echo packet MUST be demultiplexed to the appropriate session for processing. (§6.8.8) | MUST | 6.8.8 - Reception of BFD Echo Packets | **positive:** `unit/verify` [`TestRFC5880EchoDemultiplexedToItsSession`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L589). **negative:** `unit/verify` [`TestRFC5880UnknownEchoDropped`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L626) |
| `RFC5880-6.8.8-2` | A means of detecting missing Echo packets MUST be implemented, which most likely involves processing of the Echo packets that are received. (§6.8.8) | MUST | 6.8.8 - Reception of BFD Echo Packets | **positive:** `unit/verify` [`TestRFC5880EchoMissDetectedAndReported`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L957). **negative:** `unit/verify` [`TestRFC5880EchoReturnClearsMissAndFailIsScoped`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L993) |
| `RFC5880-6.8.9-1` | BFD Echo packets MUST NOT be transmitted when bfd.SessionState is not Up. (§6.8.9) | MUST NOT | 6.8.9 - Transmission of BFD Echo Packets | **positive:** `unit/verify` [`TestRFC5880EchoTransmittedWhileUp`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L543). **negative:** `unit/verify` [`TestRFC5880NoEchoTransmittedWhenNotUp`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L564) |
| `RFC5880-6.8.9-2` | BFD Echo packets MUST NOT be transmitted unless the last BFD Control packet received from the remote system contains a nonzero value in Required Min Echo RX Interval. (§6.8.9) | MUST NOT | 6.8.9 - Transmission of BFD Echo Packets | **positive:** `unit/verify` [`TestRFC5880EchoEnabledWhenPeerAdvertisesNonZero`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L871). **negative:** `unit/verify` [`TestRFC5880EchoNotTransmittedWithoutPeerAdvertisement`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L892) |
| `RFC5880-6.8.9-3` | The interval between transmitted BFD Echo packets MUST NOT be less than the value advertised by the remote system in Required Min Echo RX Interval (§6.8.9) | MUST NOT | 6.8.9 - Transmission of BFD Echo Packets | **positive:** `unit/verify` [`TestRFC5880EchoIntervalHonorsPeerFloor`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L915). **positive:** `unit/verify` [`TestRFC5880EchoRaisedPeerFloorDelaysNextEcho`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L796). **negative:** `unit/verify` [`TestRFC5880EchoIntervalNotBelowLocalTarget`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L934). **negative:** `unit/verify` [`TestRFC5880EchoLoweredPeerFloorTakesEffect`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L828) |
| `RFC5880-7-1` | When BFD is used across multiple hops, a congestion control mechanism MUST be implemented (§7) | MUST | 7 - Operational Considerations | **positive:** no positive test. **negative:** no negative test. **{gap}:** the transmit rate is derived solely from TransmitInterval plus jitter (internal/component/bfd/engine/loop.go:197-200) with no congestion-feedback input, and no congestion-control producer exists anywhere under internal/component/bfd |
| `RFC5880-7-2` | When BFD is used across multiple hops, a congestion control mechanism MUST be implemented, and when congestion is detected, the BFD implementation MUST reduce the amount of traffic it generates. (§7) | MUST | 7 - Operational Considerations | **positive:** no positive test. **negative:** no negative test. **{gap}:** the same rate producer (internal/component/bfd/engine/loop.go:197-200) is the only authority over generated traffic and nothing reduces it in response to detected congestion |
| `RFC5880-4.3-1` | This byte MUST be set to zero on transmit (§4.3, §4.4) | MUST | 4.3 | **positive:** `unit/verify` [`TestRFC5880KeyedMD5SectionHeader`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L91). **negative:** no negative test. **{single-polarity}:** Sign hardcodes the Reserved byte to 0 (internal/component/bfd/auth/sha1.go:83) and no code path emits a non-zero value, so there is no non-conformant transmission to reject |
| `RFC5880-6.8.1-15` | It SHOULD be set to a random (but still unique) value to improve security. (§6.8.1) | SHOULD | 6.8.1 - State Variables | **positive:** no positive test. **negative:** no negative test |
| `RFC5880-6.4-1` | A system SHOULD otherwise advertise the lowest value of Required Min RX Interval and Required Min Echo RX Interval that it can under the circumstances, to give the other system more freedom in choosing its transmission rate. (§6.4) | SHOULD | 6.4 - The Echo Function and Asymmetry | **positive:** no positive test. **negative:** no negative test |
| `RFC5880-5-1` | Some form of authentication SHOULD be included, since Echo packets may be spoofed. (§5) | SHOULD | 5 - BFD Echo Packet Format | **positive:** no positive test. **negative:** no negative test |
| `RFC5880-6.8.3-7` | When the Echo function is active, a system SHOULD set bfd.RequiredMinRxInterval to a value of not less than one second (1,000,000 microseconds). (§6.8.3) | SHOULD | 6.8.3 - Timer Manipulation | **positive:** no positive test. **negative:** no negative test |
| `RFC5880-6.8.7-9` | A BFD Control packet SHOULD be transmitted during the interval between periodic Control packet transmissions when the contents of that packet would differ from that in the previously transmitted packet (other than the Poll and Final bits) in order to more rapidly communicate a change in state. (§6.8.7) | SHOULD | 6.8.7 - Transmitting BFD Control Packets | **positive:** no positive test. **negative:** no negative test |
| `RFC5880-6.7.1-1` | In order to avoid security risks, implementations using this method SHOULD only allow the authentication state to be changed at most once without some form of intervention (so that authentication cannot be turned on and off repeatedly simply based on the receipt of BFD Control packets from remote systems). (§6.7.1) | SHOULD | 6.7.1 - Enabling and Disabling Authentication | **positive:** no positive test. **negative:** no negative test |
| `RFC5880-6.7.1-2` | Unless it is desired to enable or disable authentication, an implementation SHOULD NOT allow the authentication state to change based on the receipt of BFD Control packets. (§6.7.1) | SHOULD NOT | 6.7.1 - Enabling and Disabling Authentication | **positive:** no positive test. **negative:** no negative test |
| `RFC5880-6.7.2-2` | the management interface by which the password is configured MUST accept ASCII strings, and SHOULD also allow for the configuration of any arbitrary binary string in hexadecimal form. (§6.7.2) | SHOULD | 6.7.2 - Simple Password Authentication | **positive:** no positive test. **negative:** no negative test |
| `RFC5880-6.7.3-5` | bfd.XmitAuthSeq SHOULD be incremented when the session state changes, or when the transmitted BFD Control packet carries different contents than the previously transmitted packet. (§6.7.3) | SHOULD | 6.7.3 - Keyed MD5 and Meticulous Keyed MD5 Authentication | **positive:** no positive test. **negative:** no negative test |
| `RFC5880-6.8.16-2` | BFD Control packets SHOULD be transmitted for at least a Detection Time after transitioning to AdminDown state in order to ensure that the remote system is aware of the state change. (§6.8.16) | SHOULD | 6.8.16 - Administrative Control | **positive:** no positive test. **negative:** no negative test |
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
| `RFC5880-6.8.16-3` | BFD Control packets MAY be transmitted indefinitely after transitioning to AdminDown state in order to maintain session state in each system (§6.8.16) | MAY | 6.8.16 - Administrative Control | **positive:** no positive test. **negative:** no negative test |
| `RFC5880-9-2` | When a BFD session is directly connected across a single link (physical, or a secure tunnel such as IPsec), the TTL or Hop Count MUST be set to the maximum on transmit, and checked to be equal to the maximum value on reception (and the packet dropped if this is not the case). (§9) | MUST | 9 - Security Considerations | **positive:** `unit/verify` [`TestRFC5880SingleHopTransmitTTLIsMaximum`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/transport/rfc5880_test.go#L43). **negative:** `unit/verify` [`TestRFC5880SingleHopReceiveTTLNotMaxDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L160) |
| `RFC5880-6.8.14-1` | If Demand mode is no longer active on the remote system, the local system MUST begin transmitting periodic BFD Control packets (§6.8.14) | MUST | 6.8.14 - Enabling or Disabling Demand Mode | **positive:** `unit/verify` [`TestRFC5880PeriodicTransmitWhenRemoteDemandInactive`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L372). **negative:** no negative test. **{single-polarity}:** tick transmits periodic Control packets without consulting the remote D bit (internal/component/bfd/engine/loop.go:186-201), so a remote that stops asserting Demand keeps receiving them and there is no suppressed state to leave |
| `RFC5880-6.8.17-1` | If Demand mode is active on the remote system (the local system is not transmitting periodic BFD Control packets), a Poll Sequence MUST be initiated to ensure that the diagnostic code is transmitted. (§6.8.17) | MUST | 6.8.17 - Concatenated Paths | **positive:** no positive test. **negative:** no negative test. **{gap}:** packet.DiagConcatPathDown (internal/component/bfd/packet/diag.go:23) has no producer and the remote Demand state stored at internal/component/bfd/session/fsm.go:58 is never read, so a concatenated-path failure neither sets the diagnostic nor initiates a Poll Sequence |
| `RFC5880-6.8.3-8` | In any case other than those explicitly called out above, timing parameter changes MUST be effected immediately (changing the transmission rate and/or the Detection Time). (§6.8.3) | MUST | 6.8.3 - Timer Manipulation | **positive:** `unit/verify` [`TestRFC5880RemoteMinRxReductionHonoredImmediately`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L506). **negative:** no negative test. **{single-polarity}:** TransmitInterval and DetectionInterval are recomputed from the live bfd.* variables on every call (internal/component/bfd/session/timers.go:24-49), so an unexcepted timing change is in force at the next evaluation and no held-back value exists to reject |
| `RFC5880-6.7.3-7` | the management interface by which the key is configured MUST accept ASCII strings (§6.7.3) | MUST | 6.7.3 - Keyed MD5 and Meticulous Keyed MD5 Authentication | **positive:** `unit/verify` [`TestRFC5880KeyManagementAcceptsASCIIStrings`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5880_test.go#L26). **negative:** `unit/verify` [`TestRFC5880KeyManagementRejectsIncompleteConfig`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5880_test.go#L64) |
| `RFC5880-6.7.2-3` | The currently selected password and Key ID for the session MUST be stored in the Authentication Section of each outgoing BFD Control packet. (§6.7.2) | MUST | 6.7.2 - Simple Password Authentication | **positive:** `unit/verify` [`TestRFC5880AuthKeyIDIsTheConfiguredKey`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L249). **positive:** `unit/verify` [`TestRFC5880SimplePasswordSectionCarriesPasswordAndKeyID`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L685). **negative:** `unit/verify` [`TestRFC5880AuthKeyIDMismatchDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L267) |
| `RFC5880-6.7.3-8` | The Sequence Number field MUST be set to bfd.XmitAuthSeq. (§6.7.3, §6.7.4) | MUST | 6.7.3 - Keyed MD5 and Meticulous Keyed MD5 Authentication | **positive:** `unit/verify` [`TestRFC5880AuthSequenceFieldIsXmitAuthSeq`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L1159). **negative:** `unit/verify` [`TestRFC5880AuthSequenceFieldFollowsAdvance`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L1182) |
| `RFC5880-6.7.2-4` | If the received BFD Control packet does not contain an Authentication Section, or the Auth Type is not 1 (Simple Password), then the received packet MUST be discarded. (§6.7.2, §6.7.3, §6.7.4) | MUST | 6.7.2 - Simple Password Authentication | **positive:** `unit/verify` [`TestRFC5880AuthTypeMatchAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L300). **negative:** `unit/verify` [`TestRFC5880AuthTypeMismatchDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L313) |
| `RFC5880-6.7.2-5` | If the Auth Key ID field does not match the ID of a configured password, the received packet MUST be discarded. (§6.7.2, §6.7.3, §6.7.4) | MUST | 6.7.2 - Simple Password Authentication | **positive:** `unit/verify` [`TestRFC5880AuthKeyIDMatchAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L288). **positive:** `unit/verify` [`TestRFC5880SimplePasswordMatchingKeyIDAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L775). **negative:** `unit/verify` [`TestRFC5880AuthKeyIDMismatchDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L270). **negative:** `unit/verify` [`TestRFC5880SimplePasswordWrongKeyIDDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L758) |
| `RFC5880-6.7.2-6` | If the Auth Len field is not equal to the length of the password selected by the key ID, plus three, the packet MUST be discarded. (§6.7.2, §6.7.3, §6.7.4) | MUST | 6.7.2 - Simple Password Authentication | **positive:** `unit/verify` [`TestRFC5880AuthLenExpectedAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L329). **negative:** `unit/verify` [`TestRFC5880AuthLenMismatchDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L346) |
| `RFC5880-6.7.2-7` | If the Password field does not match the password selected by the key ID, the packet MUST be discarded. (§6.7.2) | MUST | 6.7.2 - Simple Password Authentication | **positive:** `unit/verify` [`TestRFC5880SimplePasswordMatchAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L708). **negative:** `unit/verify` [`TestRFC5880SimplePasswordMismatchDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L727) |
| `RFC5880-6.7.3-9` | For Keyed MD5, if the sequence number lies outside of the range of bfd.RcvAuthSeq to bfd.RcvAuthSeq+(3*Detect Mult) inclusive (when treated as an unsigned 32-bit circular number space), the received packet MUST be discarded. (§6.7.3) | MUST | 6.7.3 - Keyed MD5 and Meticulous Keyed MD5 Authentication | **positive:** `unit/verify` [`TestRFC5880KeyedSequenceAtOrAboveFloorAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L539). **positive:** `unit/verify` [`TestRFC5880KeyedSequenceWindowWraps`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L862). **negative:** `unit/verify` [`TestRFC5880KeyedSequenceBelowFloorDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L560). **negative:** `unit/verify` [`TestRFC5880KeyedSequenceBeyondWindowDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L837) |
| `RFC5880-6.7.3-10` | For Meticulous Keyed MD5, if the sequence number lies outside of the range of bfd.RcvAuthSeq+1 to bfd.RcvAuthSeq+(3*Detect Mult) inclusive (when treated as an unsigned 32-bit circular number space) the received packet MUST be discarded. (§6.7.3) | MUST | 6.7.3 - Keyed MD5 and Meticulous Keyed MD5 Authentication | **positive:** `unit/verify` [`TestRFC5880MeticulousSequenceInsideWindowAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L883). **negative:** `unit/verify` [`TestRFC5880MeticulousSequenceOutsideWindowDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L906) |
| `RFC5880-6.7.3-11` | Otherwise (the digest does not match the Auth Key/Digest field), the received packet MUST be discarded. (§6.7.3, §6.7.4) | MUST | 6.7.3 - Keyed MD5 and Meticulous Keyed MD5 Authentication | **positive:** `unit/verify` [`TestRFC5880DigestMatchAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L380). **negative:** `unit/verify` [`TestRFC5880DigestMismatchDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L393) |
| `RFC5880-6.7.3-12` | Otherwise (bfd.AuthSeqKnown is 0), bfd.AuthSeqKnown MUST be set to 1, and bfd.RcvAuthSeq MUST be set to the value of the received Sequence Number field. (§6.7.3, §6.7.4) | MUST | 6.7.3 - Keyed MD5 and Meticulous Keyed MD5 Authentication | **positive:** `unit/verify` [`TestRFC5880FirstAuthenticatedPacketSeedsReplayFloor`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L455). **positive:** `unit/verify` [`TestRFC5880FirstPacketSeedsReplayFloorBeforeDigest`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L479). **negative:** `unit/verify` [`TestRFC5880KnownSequenceNotReseededByForgedPacket`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L513) |

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
| [`RFC5880-6.8.17-1`](#rfc5880-6.8.17-1) If Demand mode is active on the remote system (the local system is not transmitting periodic BFD Control packets), a Poll Sequence MUST be initiated to ensure that the diagnostic code is transmitted. (§6.8.17) | {gap}, no test | packet.DiagConcatPathDown (internal/component/bfd/packet/diag.go:23) has no producer and the remote Demand state stored at internal/component/bfd/session/fsm.go:58 is never read, so a concatenated-path failure neither sets the diagnostic nor initiates a Poll Sequence |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC5880-4.1-1`](#rfc5880-4.1-1)

The contents of transmitted BFD Control packets MUST be set as follows: Version Set to the current version number (1). (§6.8.7)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Row now quotes the 6.8.7 transmit rule. The positive encodes the test fixture rfc5880Good (packet/rfc5880_test.go), never session Build (session/fsm.go:214), so Build writing another version would stay green; the negative asserts the 6.8.6-1 receive discard, a neighbouring rule.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880VersionNotOneDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/packet/rfc5880_test.go#L64) | unit/verify | unproven |
| positive | [`TestRFC5880VersionOneAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/packet/rfc5880_test.go#L44) | unit/verify | unproven |

### [`RFC5880-4.1-2`](#rfc5880-4.1-2)

It MUST be zero on both transmit and receipt. (§4.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Receipt clause enforced (ParseControl rejects M=1). Transmit clause asserted only on the fixture rfc5880Good, not on session Build (fsm.go:222), so a Build that set M would stay green.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880MultipointSetDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/packet/rfc5880_test.go#L96) | unit/verify | unproven |
| positive | [`TestRFC5880MultipointZeroAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/packet/rfc5880_test.go#L80) | unit/verify | unproven |

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

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Forbidden: bfd.RemoteDiscr non-zero after Init. Red: TestRFC5880InitVariableDefaults Fatalf on RemoteDiscriminator()!=0. The negative tag shows Receive installs the peer's My Discriminator (the 6.8.6 rule), not a violation of this row; no {single-polarity} marker, so one polarity.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880InitVariablesAreNotConstants`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L113) | unit/verify | unproven |
| positive | [`TestRFC5880InitVariableDefaults`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L88) | unit/verify | unproven |

### [`RFC5880-6.8.1-5`](#rfc5880-6.8.1-5)

If a period of a Detection Time passes without the receipt of a valid, authenticated BFD packet from the remote system, this variable MUST be set to zero. (§6.8.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. SB-1 strict re-read 2026-09-27 (D-4 send-back): clause 'without the receipt of a valid, authenticated BFD packet': no tagged unit delivers an invalid or unauthenticated packet during the Detection Time and asserts bfd.RemoteDiscr is still cleared at its end, so a Receive that re-armed detection before its discard checks stays green; the note's appeal to the 6.8.6 discard rows is an argument, not an assertion in a unit tagged here. The expiry clauses stand: TestRFC5880RemoteDiscrClearedOnDetectionExpiry (Init/Up expiry) and TestRFC5880RemoteDiscrClearedAfterSilenceWhileDown (kept 1 ms short, 0 once a Detection Time passes), with TestRFC5880RemoteDiscrKeptOnNeighborSignaledDown against an over-eager clear. Blind reader BS-B: same finding.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880RemoteDiscrKeptOnNeighborSignaledDown`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L180) | unit/verify | unproven |
| positive | [`TestRFC5880RemoteDiscrClearedAfterSilenceWhileDown`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L1215) | unit/verify | revert, verified |
| positive | [`TestRFC5880RemoteDiscrClearedOnDetectionExpiry`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L153) | unit/verify | unproven |

### [`RFC5880-6.8.1-6`](#rfc5880-6.8.1-6)

This MUST be initialized to zero (No Diagnostic). (§6.8.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: bfd.LocalDiag initialised to anything but 0. TestRFC5880InitVariableDefaults fails on LocalDiag() != DiagNone straight after Init. Negative TestRFC5880InitVariablesAreNotConstants shows the value is an initial value, not a constant: a peer-signalled Down moves it to DiagNeighborSignaledDown.

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

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: bfd.RemoteMinRxInterval initialised to anything but 1. TestRFC5880InitVariableDefaults fails on RemoteMinRxInterval != 1 after Init. Negative TestRFC5880InitVariablesAreNotConstants shows the value is replaced by the advertised 700000, so the 1 is an initial value.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880InitVariablesAreNotConstants`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L118) | unit/verify | unproven |
| positive | [`TestRFC5880InitVariableDefaults`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L92) | unit/verify | unproven |

### [`RFC5880-6.8.1-9`](#rfc5880-6.8.1-9)

This variable MUST be initialized to zero. (§6.8.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: bfd.RemoteDemandMode initialised true. TestRFC5880InitVariableDefaults fails on RemoteDemandMode true after Init. Negative TestRFC5880InitVariablesAreNotConstants shows it follows a received D=1, so false is an initial value.

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

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Forbidden: bfd.AuthSeqKnown starting at 1. Red: TestRFC5880AuthSeqKnownStartsZero Fatalf when a zero-value SeqState reports Initialized. The negative tag shows Advance sets it (6.7.3-12), not a violation of this row; no {single-polarity} marker. The unit checks the SeqState zero value, not the state a session Init creates. Re-stamped 2026-09-27 (DF-BFD): the unit bodies changed only in fixtures (a nonzero Your Discriminator in peer Init/Up packets, keys that fit their slot, dropped line numbers); the judgment is unchanged. RA-BFD strict re-read 2026-09-27: weak stands for the reason above.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880AuthSeqKnownSetAfterFirstPacket`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L444) | unit/verify | unproven |
| positive | [`TestRFC5880AuthSeqKnownStartsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L426) | unit/verify | unproven |

### [`RFC5880-6.8.1-13`](#rfc5880-6.8.1-13)

This variable MUST be set to zero after no packets have been received on this session for at least twice the Detection Time. (§6.8.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5880-6.8.1-13, so no unit is bound to it.

### [`RFC5880-6.8.1-14`](#rfc5880-6.8.1-14)

Once session state is created, and at least one BFD Control packet is received from the remote end, it MUST be preserved for at least one Detection Time (see section 6.8.4) subsequent to the receipt of the last BFD Control packet, regardless of the session state. (§6.8.1)

Audit verdict: wrong (the tests assert something other than what the requirement demands), fresh. RA-BFD strict re-read 2026-09-27, lowered from weak. The row obliges keeping session state (the variables) for at least one Detection Time after the last received Control packet, regardless of session state. TestRFC5880StatePreservedForOneDetectionTime asserts CheckDetection does not fire and the FSM stays Up just before the deadline, and TestRFC5880DetectionExpiryDownDiagOne asserts Down at the deadline: both are detection timing, RFC5880-6.8.4-1, a neighbouring rule. Neither shows the Machine or engine entry survives a Down, a ReleaseSession or a teardown for a Detection Time, and the negative asserts a bound this row does not set (a system MAY preserve longer).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880DetectionExpiryDownDiagOne`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L308) | unit/verify | unproven |
| positive | [`TestRFC5880StatePreservedForOneDetectionTime`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L283) | unit/verify | unproven |

### [`RFC5880-6.1-1`](#rfc5880-6.1-1)

A system taking the Active role MUST send BFD Control packets for a particular session, regardless of whether it has received any BFD packets for that session. (§6.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Forbidden: an Active session that waits for reception before transmitting. TestRFC5880ActiveRoleTransmitsWithoutReception asserts only that Init arms NextTxDeadline with RemoteDiscr 0; the engine tick (engine/loop.go tick) that turns the deadline into sendLocked is not driven, so a transmit gate added there on RemoteDiscr stays green. The negative tag (Passive transmits after reception) proves 6.1-2, not a violation of this row, and the row has no {single-polarity} marker.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880PassiveRoleTransmitsAfterReception`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L410) | unit/verify | unproven |
| positive | [`TestRFC5880ActiveRoleTransmitsWithoutReception`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L361) | unit/verify | unproven |

### [`RFC5880-6.1-2`](#rfc5880-6.1-2)

A system taking the Passive role MUST NOT begin sending BFD packets for a particular session until it has received a BFD packet for that session, and thus has learned the remote system's discriminator value. (§6.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Forbidden: a Passive session transmitting before any reception. The units assert the NextTxDeadline field only (zero before Receive, non-zero after); the producer that actually withholds the packet is the zero-deadline skip in the engine tick (engine/loop.go tick, next.IsZero() continue), and no tagged unit drives it, so deleting that skip stays green. The before/after pair is otherwise a sound polarity.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880PassiveRoleTransmitsAfterReception`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L415) | unit/verify | unproven |
| positive | [`TestRFC5880PassiveRoleSilentUntilReception`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L386) | unit/verify | unproven |

### [`RFC5880-6.1-3`](#rfc5880-6.1-3)

At least one system MUST take the Active role (possibly both). (§6.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. The obligation binds the pair of systems. Positive asserts only that the default role is Active (m.role after newMachine); the negative shows Passive is selectable, which does not violate the row. Nothing refuses or flags Ze Passive facing a Passive peer, and the row has no {single-polarity} marker: Ze's default is proven, the pair obligation is not.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880PassiveRoleSilentUntilReception`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L394) | unit/verify | unproven |
| positive | [`TestRFC5880ActiveRoleTransmitsWithoutReception`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L366) | unit/verify | unproven |

### [`RFC5880-6.8.3-1`](#rfc5880-6.8.3-1)

When bfd.SessionState is not Up, the system MUST set bfd.DesiredMinTxInterval to a value of not less than one second (1,000,000 microseconds). (§6.8.3)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. RA-BFD2 strict re-read 2026-09-27. The two DF-BFD-6 edits to TestRFC5880SlowStartFloorRestoredOnAdminDown only add settlePoll and a precondition that the slow-down applied (prior unit reconstructed to its recorded sha 3cf2312badfafde9; no assertion removed). Asserted: initial Down (TestRFC5880SlowStartFloorWhileNotUp) and Up to AdminDown with and without the echo slow-down; negative TestRFC5880SlowStartFloorLiftedWhenUp pins 300 ms in Up. Not asserted: the floor on Up to Down (detection expiry or peer-signalled Down) and in Init, so an onStateChange that set the floor only for AdminDown leaves every tagged unit green. The code (onStateChange, any non-Up state) is correct.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880SlowStartFloorLiftedWhenUp`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L232) | unit/verify | unproven |
| positive | [`TestRFC5880SlowStartFloorRestoredOnAdminDown`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L1253) | unit/verify | revert, verified |
| positive | [`TestRFC5880SlowStartFloorWhileNotUp`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L213) | unit/verify | unproven |

### [`RFC5880-6.8.3-2`](#rfc5880-6.8.3-2)

If either bfd.DesiredMinTxInterval is changed or bfd.RequiredMinRxInterval is changed, a Poll Sequence MUST be initiated (see section 6.5). (§6.8.3)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. The quote has two clauses (DesiredMinTxInterval changed, OR RequiredMinRxInterval changed). TestRFC5880PollInitiatedOnIntervalChange changes BOTH at once at Up, and TestRFC5880NoPollWhenIntervalsUnchanged changes neither, so a guard that polled only on a DesiredMinTx change, or only when both changed (&& for ||), passes both units. No unit changes one interval alone. The Down-entry and AdminEnable resets of DesiredMinTxInterval to 1 s (fsm.go onStateChange, AdminEnable) raise no Poll and no unit covers them. Re-stamped 2026-09-27 (DF-BFD): the unit bodies changed only in fixtures (a nonzero Your Discriminator in peer Init/Up packets, keys that fit their slot, dropped line numbers); the judgment is unchanged. RA-BFD strict re-read 2026-09-27: weak stands for the reason above.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880NoPollWhenIntervalsUnchanged`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L470) | unit/verify | unproven |
| positive | [`TestRFC5880PollInitiatedOnIntervalChange`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L436) | unit/verify | unproven |

### [`RFC5880-6.8.3-3`](#rfc5880-6.8.3-3)

If bfd.DesiredMinTxInterval is increased and bfd.SessionState is Up, the actual transmission interval used MUST NOT change until the Poll Sequence described above has terminated. (§6.8.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RA-BFD3 first verdict 2026-09-27 (gap closed by DF-BFD-7: desiredMinTxInForceUs, holdDesiredMinTxLocked, releaseDesiredMinTxLocked in timers.go). The sentence has one condition pair (increase AND Up) and one prohibition (actual interval unchanged until the Poll terminates). Forbidden behaviour: TX at the raised interval while the Poll is outstanding. Both writers of an increase in Up are driven by the positive TestRFC5880RaisedDesiredMinTxHeldUntilPollTerminates: echo slow-down (300 ms to 1 s, RemoteMinRx 100 ms) requires TransmitInterval == 300 ms, NextTxDeadline == last TX + 300 ms after AdvanceTxWithJitter, and still +300 ms after a non-Final packet (the Receive reschedule path); Up transition with a 2 s configured interval requires TransmitInterval == 1 s (remote 300 ms) with the Poll outstanding. Each goes red if TransmitInterval reads the live bfd.DesiredMinTxInterval (1 s, 2 s). Negative TestRFC5880RaisedDesiredMinTxAppliedAtFinal: the terminating Final makes TransmitInterval 1 s and moves the pending deadline to last TX + 1 s, red on a hold that is never released or released without rescheduling; a session leaving Up drops the hold (TransmitInterval 1 s slow-start, not the held 300 ms). The only runtime writers of bfd.DesiredMinTxInterval in Up are ApplyEchoSlowdown and onStateChange, both covered; a decrease (revertEchoSlowdownLocked) is not held, as the sentence covers only an increase.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880RaisedDesiredMinTxAppliedAtFinal`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L1517) | unit/verify | revert, verified |
| positive | [`TestRFC5880RaisedDesiredMinTxHeldUntilPollTerminates`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L1458) | unit/verify | revert, verified |

### [`RFC5880-6.8.3-4`](#rfc5880-6.8.3-4)

If bfd.RequiredMinRxInterval is reduced and bfd.SessionState is Up, the previous value of bfd.RequiredMinRxInterval MUST be used when calculating the Detection Time for the remote system until the Poll Sequence described above has terminated. (§6.8.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5880-6.8.3-4, so no unit is bound to it.

### [`RFC5880-6.8.3-5`](#rfc5880-6.8.3-5)

If the local system reduces its transmit interval due to bfd.RemoteMinRxInterval being reduced (the remote system has advertised a reduced value in Required Min RX Interval), and the remote system is not in Demand mode, the local system MUST honor the new interval immediately. (§6.8.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RA-BFD2 strict re-read 2026-09-27 (stale on Receive: echo reschedule and pollSettling added after rescheduleTxLocked, which is still called on every accepted packet). Forbidden: keeping the old, longer wait after a reduction. TestRFC5880RemoteMinRxChangeReschedulesNextTx requires last TX + 400 ms after 900 to 400 ms, and a jittered 900 of 1.2 s rescaled to 300 of 400 ms, already due. TestRFC5880RemoteMinRxReductionHonoredImmediately pins TransmitInterval at 400 ms at once. Negatives: TestRFC5880UnchangedRemoteMinRxKeepsNextTx (no change, deadline kept) and TestRFC5880TransmitIntervalFlooredByLocalDesired (a reduction below the local desired value does not shorten TX). The Demand-mode condition only removes the obligation; Ze runs no Demand mode, so it always honours the reduction.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880TransmitIntervalFlooredByLocalDesired`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L533) | unit/verify | unproven |
| negative | [`TestRFC5880UnchangedRemoteMinRxKeepsNextTx`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L1372) | unit/verify | revert, verified |
| positive | [`TestRFC5880RemoteMinRxChangeReschedulesNextTx`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L1284) | unit/verify | revert, verified |
| positive | [`TestRFC5880RemoteMinRxReductionHonoredImmediately`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L497) | unit/verify | unproven |

### [`RFC5880-6.8.3-6`](#rfc5880-6.8.3-6)

Therefore, if multiple changes are made that require the use of a Poll Sequence, there are three choices: 1) they MUST be communicated in a single BFD Control packet (so the semantics of the Final reply are clear), or 2) sufficient time must have transpired since the Poll Sequence was completed to disambiguate the situation (at least a round trip time since the last Poll was transmitted) prior to the initiation of another Poll Sequence, or 3) an additional BFD Control packet with the Final (F) bit *clear* MUST be received after the Poll Sequence has completed prior to the initiation of another Poll Sequence (this option is not available when Demand mode is active). (§6.8.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RA-BFD2 strict re-read 2026-09-27, raised from weak: the RA-BFD defect is fixed at the producer. Every Poll initiator is choice 1 or choice 3: onStateChange changes both intervals under one PollOutstanding (and can only fire from Down/Init, where Down entry clears PollOutstanding and pollSettling); ApplyEchoSlowdown and revertEchoSlowdownLocked (while Up) return with the flag unchanged unless pollSequenceIdle, which Receive clears only on the Final that ends the Poll (pollSettling set) followed by a packet with F clear. Forbidden second Poll during an outstanding Poll: TestRFC5880EchoSlowdownWaitsForOutstandingPoll (tx and rx stay 10000 during the Up Poll) and TestRFC5880EchoSlowdownRevertWaitsForOutstandingPoll (stays 1000000). Forbidden Poll after the Final before an F-clear packet: both tests assert no Poll and the old intervals. Positive: TestRFC5880EchoSlowdownPollsAfterSettledPoll (P set, both intervals 1000000 in one packet) and TestRFC5880PollInitiatedOnIntervalChange (single packet). Choice 2 is an alternative Ze does not take; Demand mode is not driven, so choice 3 is always available.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880EchoSlowdownRevertWaitsForOutstandingPoll`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L755) | unit/verify | revert, verified |
| negative | [`TestRFC5880EchoSlowdownWaitsForOutstandingPoll`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L689) | unit/verify | revert, verified |
| positive | [`TestRFC5880EchoSlowdownPollsAfterSettledPoll`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L726) | unit/verify | revert, verified |
| positive | [`TestRFC5880PollInitiatedOnIntervalChange`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L441) | unit/verify | unproven |

### [`RFC5880-6.5-1`](#rfc5880-6.5-1)

A BFD Control packet MUST NOT have both the Poll (P) and Final (F) bits set. (§6.5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: P and F both set. Red: TestRFC5880PollPacketHasNoFinalBit Fatalf on c.Final from Build with PollOutstanding; TestRFC5880FinalReplyClearsPollBit Fatalf on f.Poll from BuildFinal with PollOutstanding true, the state where inheriting P would set both. Build and BuildFinal are the only packets sendLocked is handed (engine/loop.go).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880FinalReplyClearsPollBit`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L586) | unit/verify | unproven |
| positive | [`TestRFC5880PollPacketHasNoFinalBit`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L567) | unit/verify | unproven |

### [`RFC5880-6.5-2`](#rfc5880-6.5-2)

If periodic BFD Control packets are already being sent (the remote system is not in Demand mode), the Poll Sequence MUST be performed by setting the Poll (P) bit on those scheduled periodic transmissions; additional packets MUST NOT be sent. (§6.5)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. P is shown on Build output and absent when no Poll is outstanding. The second clause, additional packets MUST NOT be sent, is untested: nothing asserts that raising a Poll causes no extra transmission.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880FinalReplyClearsPollBit`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L590) | unit/verify | unproven |
| positive | [`TestRFC5880PollPacketHasNoFinalBit`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L571) | unit/verify | unproven |

### [`RFC5880-6.6-1`](#rfc5880-6.6-1)

A system MUST NOT set the Demand (D) bit unless bfd.DemandMode is 1, bfd.SessionState is Up, and bfd.RemoteSessionState is Up. (§6.8.7)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: D set while DemandMode, local Up or remote Up is false. Red: TestRFC5880DemandBitClearWhenAnyConditionFails Fatalf on Build().Demand with each condition dropped alone and with both states Down; positive TestRFC5880DemandBitSetWhenAllConditionsHold shows D is set when all hold, so the clear cases are not a constant false.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880DemandBitClearWhenAnyConditionFails`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L666) | unit/verify | unproven |
| positive | [`TestRFC5880DemandBitSetWhenAllConditionsHold`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L652) | unit/verify | unproven |

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

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Forbidden: dropping either SHA1 type. Red: TestRFC5880BothSHA1VariantsSupported Fatalf on a NewSigner/NewVerifier error, a wrong AuthType or a failed round trip for types 4 and 5. The negative tag (types 0, 6, 200 refused) does not violate this row, and the row has no {single-polarity} marker, so only one polarity exists.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880UnsupportedAuthTypesRejected`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L70) | unit/verify | unproven |
| positive | [`TestRFC5880BothSHA1VariantsSupported`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L51) | unit/verify | unproven |

### [`RFC5880-4.2-1`](#rfc5880-4.2-1)

The password is a binary string, and MUST be from 1 to 16 bytes in length. (§4.2, §6.7.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a 0-byte or over-16-byte password accepted. Red: auth TestRFC5880SimplePasswordLengthOutOfRangeRefused Fatalf unless NewSigner and NewVerifier return ErrKeyLengthInvalid for 0, 17, 32, 255 bytes; bfd TestRFC5880SimplePasswordLengthOutOfRangeRefused Fatalf when parseAuthConfig accepts 0, 17 or 32 bytes. Positive: 1, 8, 16 bytes encode (auth SectionHeader) and 1, 14, 16 bytes parse (SimplePasswordManagementAcceptsASCIIStrings), both range ends.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`auth/TestRFC5880SimplePasswordLengthOutOfRangeRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L790) | unit/verify | unproven |
| negative | [`bfd/TestRFC5880SimplePasswordLengthOutOfRangeRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5880_test.go#L128) | unit/verify | unproven |
| positive | [`TestRFC5880SimplePasswordSectionHeader`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L635) | unit/verify | unproven |
| positive | [`TestRFC5880SimplePasswordManagementAcceptsASCIIStrings`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5880_test.go#L101) | unit/verify | unproven |

### [`RFC5880-6.7.2-1`](#rfc5880-6.7.2-1)

For interoperability, the management interface by which the password is configured MUST accept ASCII strings (§6.7.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Forbidden: a management interface that refuses or re-encodes an ASCII password. TestRFC5880SimplePasswordManagementAcceptsASCIIStrings Fatalf unless parseAuthConfig keeps 1, 14 and 16-byte ASCII strings verbatim, but it calls parseAuthConfig directly: the YANG secret leaf and the config/CLI entry the operator uses are not driven. The negative tag is the 4.2-1 length refusal, not a violation of this row; no {single-polarity} marker.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`bfd/TestRFC5880SimplePasswordLengthOutOfRangeRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5880_test.go#L132) | unit/verify | unproven |
| positive | [`TestRFC5880SimplePasswordManagementAcceptsASCIIStrings`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5880_test.go#L96) | unit/verify | unproven |

### [`RFC5880-6.7.2-8`](#rfc5880-6.7.2-8)

The Auth Type field MUST be set to 1 (Simple Password). The Auth Len field MUST be set to the proper length (4 to 19 bytes). (§6.7.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Forbidden: transmitting Auth Type other than 1, or Auth Len other than password+3 within 4..19. Red: TestRFC5880SimplePasswordSectionHeader Fatalf on buf[off]!=1 or Auth Len != 3+octets for 1, 8, 16 bytes. The negative tag is the receive-side shape check in simpleVerifier, a neighbouring rule; a transmit rule without a {single-polarity} marker, so one polarity only.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880SimplePasswordRejectsForeignSectionShape`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L656) | unit/verify | unproven |
| positive | [`TestRFC5880SimplePasswordSectionHeader`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L630) | unit/verify | unproven |

### [`RFC5880-6.7.3-1`](#rfc5880-6.7.3-1)

The Auth Type field MUST be set to 2 (Keyed MD5) or 3 (Meticulous Keyed MD5). The Auth Len field MUST be set to 24. (§6.7.3)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Forbidden: transmitting an MD5 section with Auth Type other than 2/3 or Auth Len other than 24. Red: TestRFC5880KeyedMD5SectionHeader Fatalf on buf[off]!=at or buf[off+1]!=24 for both variants. The negative tag is the receive-side shape check (digestVerifier.Verify, auth/sha1.go:159-163), a neighbouring rule; no {single-polarity} marker, so one polarity only.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880KeyedMD5RejectsForeignSectionShape`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L113) | unit/verify | unproven |
| positive | [`TestRFC5880KeyedMD5SectionHeader`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L86) | unit/verify | unproven |

### [`RFC5880-6.7.3-2`](#rfc5880-6.7.3-2)

An MD5 digest MUST be calculated over the entire BFD Control packet. (§6.7.3)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Forbidden: an MD5 digest over less than the entire packet. TestRFC5880DigestRejectsMandatorySectionTamper flips only mandatory bytes 1, 4 and 20; no tagged unit tampers the auth header or the Sequence Number, so a Sign/Verify pair whose span stops at the mandatory section stays green (sign and verify share the span, so the positive round trip cannot fail). Bytes 21-23 are untouched too.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880DigestRejectsMandatorySectionTamper`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L201) | unit/verify | unproven |
| positive | [`TestRFC5880DigestCoversWholePacket`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L182) | unit/verify | unproven |

### [`RFC5880-6.7.3-3`](#rfc5880-6.7.3-3)

replacing the secret key, which MUST NOT be carried in the packet (§6.7.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. single-polarity positive: neither the padded nor the raw secret appears in the emitted bytes.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC5880SecretKeyNotCarriedInPacket`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L225) | unit/verify | unproven |

### [`RFC5880-6.7.3-4`](#rfc5880-6.7.3-4)

For Meticulous Keyed MD5, bfd.XmitAuthSeq MUST be incremented in a circular fashion (when treated as an unsigned 32-bit value). (§6.7.3)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Positive checks three consecutive increments only; the circular 32-bit wrap the quote states is never exercised, so a saturating XmitAuthSeq would stay green (AdvanceAuthSeq does wrap via uint32 ++). Negative asserts the receiver rejects a repeated sequence, a neighbouring receive rule. Re-stamped 2026-09-27 (DF-BFD): the unit bodies changed only in fixtures (a nonzero Your Discriminator in peer Init/Up packets, keys that fit their slot, dropped line numbers); the judgment is unchanged. RA-BFD strict re-read 2026-09-27: weak stands for the reason above.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880MeticulousRejectsUnincrementedSequence`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L582) | unit/verify | unproven |
| positive | [`TestRFC5880MeticulousSequenceIncrementsPerPacket`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L287) | unit/verify | unproven |

### [`RFC5880-6.7.3-13`](#rfc5880-6.7.3-13)

The authentication key value is a binary string of up to 16 bytes, and MUST be placed into the Auth Key/Digest field, padded with trailing zero bytes as necessary. (§6.7.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Positive: TestRFC5880ShortKeyZeroPaddedIntoDigestField computes MD5 independently over the signed packet with the Auth Key/Digest field replaced by the 11-byte key and five zero bytes and requires Sign's transmitted digest to equal it; TestKeyedSecretLongerThanSlotRefused (NewSigner, NewVerifier) and TestParseAuthConfigKeyedSecretLength (parseAuthConfig) accept a 16-byte key for both MD5 types. Negative: the same two refuse a 17-byte key, NewSigner and NewVerifier with ErrKeyLengthInvalid, so no key longer than the field is truncated into it.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestKeyedSecretLongerThanSlotRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L934) | unit/verify | revert, verified |
| negative | [`TestParseAuthConfigKeyedSecretLength`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5881_test.go#L148) | unit/verify | revert, verified |
| positive | [`TestKeyedSecretLongerThanSlotRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L931) | unit/verify | revert, verified |
| positive | [`TestRFC5880ShortKeyZeroPaddedIntoDigestField`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L977) | unit/verify | revert, verified |
| positive | [`TestParseAuthConfigKeyedSecretLength`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5881_test.go#L145) | unit/verify | revert, verified |

### [`RFC5880-6.7.4-1`](#rfc5880-6.7.4-1)

The Auth Type field MUST be set to 4 (Keyed SHA1) or 5 (Meticulous Keyed SHA1).  The Auth Len field MUST be set to 28. (§6.7.4)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Forbidden: transmitting a SHA1 section with Auth Type other than 4/5 or Auth Len other than 28. Red: TestRFC5880KeyedSHA1SectionHeader Fatalf on buf[off]!=at or buf[off+1]!=28 for both variants. The negative tag is the receive-side shape check (auth/sha1.go:159-163), a neighbouring rule; no {single-polarity} marker, so one polarity only.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880KeyedSHA1RejectsForeignSectionShape`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L159) | unit/verify | unproven |
| positive | [`TestRFC5880KeyedSHA1SectionHeader`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L138) | unit/verify | unproven |

### [`RFC5880-6.7.4-2`](#rfc5880-6.7.4-2)

A SHA1 hash MUST be calculated over the entire BFD control packet. (§6.7.4)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Forbidden: a SHA1 hash over less than the entire packet. TestRFC5880DigestRejectsMandatorySectionTamper flips only mandatory bytes 1, 4 and 20; no tagged unit tampers the auth header or the Sequence Number, so a Sign/Verify pair whose span stops at the mandatory section stays green (shared span, so the positive round trip cannot fail). Bytes 21-23 are untouched too.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880DigestRejectsMandatorySectionTamper`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L206) | unit/verify | unproven |
| positive | [`TestRFC5880DigestCoversWholePacket`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L188) | unit/verify | unproven |

### [`RFC5880-6.7.4-3`](#rfc5880-6.7.4-3)

the secret key, which MUST NOT be carried in the packet (§6.7.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRFC5880SecretKeyNotCarriedInPacket asserts neither the padded nor the raw secret appears in the emitted SHA1 packet (single-polarity).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC5880SecretKeyNotCarriedInPacket`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L229) | unit/verify | unproven |

### [`RFC5880-6.7.4-4`](#rfc5880-6.7.4-4)

For Meticulous Keyed SHA1, bfd.XmitAuthSeq MUST be incremented in a circular fashion (when treated as an unsigned 32-bit value). (§6.7.4)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Positive (engine TestRFC5880MeticulousSequenceIncrementsPerPacket) checks three consecutive +1 increments only; nothing drives bfd.XmitAuthSeq across 0xFFFFFFFF to prove the circular 32-bit wrap the row states. Negative (auth TestRFC5880MeticulousRejectsUnincrementedSequence) tests the receiver's strict-greater check, not the transmitter. Re-stamped 2026-09-27 (DF-BFD): the unit bodies changed only in fixtures (a nonzero Your Discriminator in peer Init/Up packets, keys that fit their slot, dropped line numbers); the judgment is unchanged. RA-BFD strict re-read 2026-09-27: weak stands for the reason above.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880MeticulousRejectsUnincrementedSequence`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L587) | unit/verify | unproven |
| positive | [`TestRFC5880MeticulousSequenceIncrementsPerPacket`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L293) | unit/verify | unproven |

### [`RFC5880-6.7.4-5`](#rfc5880-6.7.4-5)

the management interface by which the key is configured MUST accept ASCII strings (§6.7.4)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Positive TestRFC5880KeyManagementAcceptsASCIIStrings proves parseAuthConfig stores an ASCII secret verbatim for keyed-sha1 and meticulous-keyed-sha1, called directly rather than through the YANG secret leaf or config entry. Negative TestRFC5880KeyManagementRejectsIncompleteConfig rejects incomplete config, a neighbouring rule; no {single-polarity} marker. Re-stamped 2026-09-27 (DF-BFD): the unit bodies changed only in fixtures (a nonzero Your Discriminator in peer Init/Up packets, keys that fit their slot, dropped line numbers); the judgment is unchanged. RA-BFD strict re-read 2026-09-27: weak stands for the reason above.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880KeyManagementRejectsIncompleteConfig`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5880_test.go#L70) | unit/verify | unproven |
| positive | [`TestRFC5880KeyManagementAcceptsASCIIStrings`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5880_test.go#L31) | unit/verify | unproven |

### [`RFC5880-6.7.4-7`](#rfc5880-6.7.4-7)

The authentication key value is a binary string of up to 20 bytes, and MUST be placed into the Auth Key/Hash field, padding with trailing zero bytes as necessary. (§6.7.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Positive: TestRFC5880ShortKeyZeroPaddedIntoDigestField computes SHA1 independently over the signed packet with the Auth Key/Hash field replaced by the 11-byte key and nine zero bytes and requires Sign's transmitted hash to equal it; TestKeyedSecretLongerThanSlotRefused and TestParseAuthConfigKeyedSecretLength accept a 20-byte key for both SHA1 types. Negative: the same two refuse a 21-byte key.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestKeyedSecretLongerThanSlotRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L940) | unit/verify | revert, verified |
| negative | [`TestParseAuthConfigKeyedSecretLength`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5881_test.go#L152) | unit/verify | revert, verified |
| positive | [`TestKeyedSecretLongerThanSlotRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L937) | unit/verify | revert, verified |
| positive | [`TestRFC5880ShortKeyZeroPaddedIntoDigestField`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L983) | unit/verify | revert, verified |
| positive | [`TestParseAuthConfigKeyedSecretLength`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5881_test.go#L150) | unit/verify | revert, verified |

### [`RFC5880-6.8.6-1`](#rfc5880-6.8.6-1)

If the version number is not correct (1), the packet MUST be discarded. (§6.8.6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: accepting a Version other than 1. TestRFC5880VersionNotOneDiscarded fails unless ParseControl returns exactly ErrBadVersion for 0, 2, 3 and 7 (version is the first check, so no other rule trips); handleInbound returns on any ParseControl error. Positive TestRFC5880VersionOneAccepted: version 1 parses with no error.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880VersionNotOneDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/packet/rfc5880_test.go#L66) | unit/verify | unproven |
| positive | [`TestRFC5880VersionOneAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/packet/rfc5880_test.go#L48) | unit/verify | unproven |

### [`RFC5880-6.8.6-2`](#rfc5880-6.8.6-2)

If the Length field is less than the minimum correct value (24 if the A bit is clear, or 26 if the A bit is set), the packet MUST be discarded. (§6.8.6)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. A=0 is proven at the boundary: 24 accepted, 23 returns ErrLengthTooSmall. A=1 is not: the units accept 26 and reject 24, but never test 25, so a minLen of MandatoryLen+1 for A=1 (accepting a 25-byte authenticated packet, which the RFC requires be discarded) passes both units.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880LengthBelowMinimumDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/packet/rfc5880_test.go#L128) | unit/verify | unproven |
| positive | [`TestRFC5880LengthMinimumAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/packet/rfc5880_test.go#L108) | unit/verify | unproven |

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

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. SB-1 strict re-read 2026-09-27 (D-4 send-back): row widened to the next sentence of Section 6.8.6, 'If no session is found, the packet MUST be discarded.', which the backfill quote had dropped. Discard clause: TestRFC5880UnknownYourDiscriminatorDiscarded sends an unallocated Your Discriminator from the session's own peer tuple and Fatalf if RemoteDiscr moves, so both a delivery and a tuple fallback go red. Selection clause: TestRFC5880YourDiscriminatorSelectsSession runs a loop holding ONE session, so a loop that delivered any known discriminator to the tuple-matched session passes; no two-session case proves the packet reaches the session its Your Discriminator names (the gap RFC5880-6.8.8-1 is weak for).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880UnknownYourDiscriminatorDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L140) | unit/verify | unproven |
| positive | [`TestRFC5880YourDiscriminatorSelectsSession`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L124) | unit/verify | unproven |

### [`RFC5880-6.8.6-8`](#rfc5880-6.8.6-8)

If the Your Discriminator field is zero and the State field is not Down or AdminDown, the packet MUST be discarded. (§6.8.6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RA-BFD2 strict re-read 2026-09-27 (stale on Receive: additions sit after the discard test, which is still the first statement). Both State values the sentence names are driven: TestRFC5880ZeroYourDiscriminatorDiscardedWhenLive sends Your Discriminator 0 with State Init and Up to local Down, Init and Up, and requires ErrYourDiscriminatorReset, unchanged state and unchanged RemoteDiscr (a guard moved below the field updates goes red on RemoteDiscr). Positive TestRFC5880ZeroYourDiscriminatorAcceptedWhenDown accepts State Down and AdminDown in Down, AdminDown and Up. Local AdminDown is not in the negative loop; the sentence conditions only on the packet's State.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880ZeroYourDiscriminatorDiscardedWhenLive`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L699) | unit/verify | revert, verified |
| positive | [`TestRFC5880ZeroYourDiscriminatorAcceptedWhenDown`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L727) | unit/verify | revert, verified |

### [`RFC5880-6.8.6-9`](#rfc5880-6.8.6-9)

If the A bit is set and no authentication is in use (bfd.AuthType is zero), the packet MUST be discarded. (§6.8.6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: processing A=1 at an AuthType-0 session. TestRFC5880UnauthenticatedSessionDiscardsAuthenticatedPacket fails unless Receive returns ErrAuthMismatch and the state stays Down; the engine hands such a packet to Receive because HasAuth is false. Positive TestRFC5880UnauthenticatedSessionAcceptsUnauthenticatedPacket: A=0 accepted.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880UnauthenticatedSessionDiscardsAuthenticatedPacket`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L780) | unit/verify | unproven |
| positive | [`TestRFC5880UnauthenticatedSessionAcceptsUnauthenticatedPacket`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L765) | unit/verify | unproven |

### [`RFC5880-6.8.6-10`](#rfc5880-6.8.6-10)

If the A bit is clear and authentication is in use (bfd.AuthType is nonzero), the packet MUST be discarded. (§6.8.6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: processing A=0 at an authenticated session. TestRFC5880AuthenticatedSessionDiscardsUnauthenticatedPacket fails unless Receive returns ErrAuthMismatch with the state unchanged; the engine skips Verify for A=0 and relies on this check. Positive TestRFC5880AuthenticatedSessionAcceptsAuthenticatedPacket: A=1 accepted.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880AuthenticatedSessionDiscardsUnauthenticatedPacket`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L815) | unit/verify | unproven |
| positive | [`TestRFC5880AuthenticatedSessionAcceptsAuthenticatedPacket`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L797) | unit/verify | unproven |

### [`RFC5880-6.8.6-11`](#rfc5880-6.8.6-11)

If the A bit is set, the packet MUST be authenticated under the rules of section 6.7, based on the authentication type in use (bfd.AuthType). (§6.8.6)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. The drop of an unauthentic A=1 packet is proven: a tampered digest and a wrong-key signature leave RemoteDiscr 0, a correct signature sets it. But the quote requires authentication "based on the authentication type in use (bfd.AuthType)", and both units run Keyed SHA1 only: a verifier that ignored bfd.AuthType and always checked Keyed SHA1 passes. No unit exercises another AuthType or a type mismatch.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880UnauthenticPacketDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L258) | unit/verify | unproven |
| positive | [`TestRFC5880AuthenticatedPacketVerifiedAndDelivered`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L241) | unit/verify | unproven |

### [`RFC5880-6.8.6-12`](#rfc5880-6.8.6-12)

If the Required Min Echo RX Interval field is zero, the transmission of Echo packets, if any, MUST cease. (§6.8.6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RA-BFD strict re-read 2026-09-27. One clause: a received Required Min Echo RX of zero ends echo transmission. Forbidden: an echo sent after that packet. Red: TestRFC5880EchoCeasesWhenPeerAdvertisesZero Fatal when EchoEnabled stays true, when PrimeEcho leaves the already-armed deadline set, or when EchoInterval is nonzero. The engine sends only from an armed deadline after its own EchoEnabled gate (echoTickLocked, echo.go), and PrimeEcho clears the deadline when echo is off, so removing either gate alone sends nothing and removing PrimeEcho's check reddens this unit. Contrast: TestRFC5880EchoEnabledWhenPeerAdvertisesNonZero keeps echo enabled and armed on a nonzero value.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880EchoEnabledWhenPeerAdvertisesNonZero`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L868) | unit/verify | unproven |
| positive | [`TestRFC5880EchoCeasesWhenPeerAdvertisesZero`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L831) | unit/verify | unproven |

### [`RFC5880-6.8.6-13`](#rfc5880-6.8.6-13)

If a Poll Sequence is being transmitted by the local system and the Final (F) bit in the received packet is set, the Poll Sequence MUST be terminated. (§6.8.6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a Poll that survives F=1. TestRFC5880FinalTerminatesPoll fails if PollOutstanding stays set after F=1; TestRFC5880NonFinalDoesNotTerminatePoll fails if F=0 clears it, so a clear-on-any-packet mutation goes red.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880NonFinalDoesNotTerminatePoll`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L630) | unit/verify | unproven |
| positive | [`TestRFC5880FinalTerminatesPoll`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L610) | unit/verify | unproven |

### [`RFC5880-6.8.6-14`](#rfc5880-6.8.6-14)

If bfd.RemoteDemandMode is 1, bfd.SessionState is Up, and bfd.RemoteSessionState is Up, Demand mode is active on the remote system and the local system MUST cease the periodic transmission of BFD Control packets (see section 6.8.7). (§6.8.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5880-6.8.6-14, so no unit is bound to it.

### [`RFC5880-6.8.6-15`](#rfc5880-6.8.6-15)

If bfd.RemoteDemandMode is 0, or bfd.SessionState is not Up, or bfd.RemoteSessionState is not Up, Demand mode is not active on the remote system and the local system MUST send periodic BFD Control packets (see section 6.8.7). (§6.8.6)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. The quote has three disjuncts (RemoteDemandMode 0, SessionState not Up, RemoteSessionState not Up). TestRFC5880PeriodicTransmitWhenRemoteDemandInactive covers only D=0 with both ends Up/Init; no unit has D=1 with either state not Up, so code that stopped periodic TX whenever D=1 passes. The negative TestRFC5880NoPeriodicTransmitWhileAdminDown tests the AdminDown skip, a neighbouring rule.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880NoPeriodicTransmitWhileAdminDown`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L403) | unit/verify | unproven |
| positive | [`TestRFC5880PeriodicTransmitWhenRemoteDemandInactive`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L368) | unit/verify | unproven |

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

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. The quote bounds TX by the LARGER of DesiredMinTxInterval and RemoteMinRxInterval. TestRFC5880TransmitDeadlineUsesNegotiatedInterval sets 300 ms local and 700 ms remote, and TestRFC5880TransmitDeadlineClampsBadJitter sets both to 300 ms, so a TransmitInterval that returned RemoteMinRxInterval alone passes both. Both units also drive AdvanceTxWithJitter by hand; nothing asserts the gap between two packets tick() actually sends.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880TransmitDeadlineClampsBadJitter`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L1064) | unit/verify | unproven |
| positive | [`TestRFC5880TransmitDeadlineUsesNegotiatedInterval`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L1034) | unit/verify | unproven |

### [`RFC5880-6.8.7-2`](#rfc5880-6.8.7-2)

The periodic transmission of BFD Control packets MUST be jittered on a per-packet basis by up to 25%, that is, the interval MUST be reduced by a random value of 0 to 25% (§6.8.7)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. TestRFC5880JitterIsAppliedPerPacket and TestRFC5880JitterStaysWithinBand call applyJitter directly and prove the draw is per-call and inside [0,25%). Nothing asserts the periodic transmission is jittered: tick() (engine/loop.go) is never driven, so removing its applyJitter call (passing 0 to AdvanceTxWithJitter) leaves both green.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880JitterStaysWithinBand`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L450) | unit/verify | unproven |
| positive | [`TestRFC5880JitterIsAppliedPerPacket`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L422) | unit/verify | unproven |

### [`RFC5880-6.8.7-3`](#rfc5880-6.8.7-3)

If bfd.DetectMult is equal to 1, the interval between transmitted BFD Control packets MUST be no more than 90% of the negotiated transmission interval, and MUST be no less than 75% of the negotiated transmission interval. (§6.8.7)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. TestRFC5880JitterDetectMultOneWindow and TestRFC5880JitterFloorOnlyForDetectMultOne prove applyJitter(base, 1) lands in [75%,90%] and applyJitter(base, 3) does not. The quote bounds the interval between TRANSMITTED packets; no unit drives tick(), so tick passing the wrong multiplier or no jitter at all to AdvanceTxWithJitter leaves both green.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880JitterFloorOnlyForDetectMultOne`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L488) | unit/verify | unproven |
| positive | [`TestRFC5880JitterDetectMultOneWindow`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L470) | unit/verify | unproven |

### [`RFC5880-6.8.7-4`](#rfc5880-6.8.7-4)

The transmit interval MUST be recalculated whenever bfd.DesiredMinTxInterval changes, or whenever bfd.RemoteMinRxInterval changes, and is equal to the greater of those two values. (§6.8.7)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RA-BFD3 strict re-read 2026-09-27 after DF-BFD-7 moved settlePoll before the 1 s precondition in TestRFC5880DesiredMinTxChangeReschedulesNextTx (the old order asserted the Section 6.8.3 defect; no assertion removed). Clause 1, recalculated when bfd.DesiredMinTxInterval changes: forbidden is a pending TX still drawn from the old interval. Decrease: after the revert (1 s to 300 ms, RemoteMinRx 100 ms) the final Fatalf requires NextTxDeadline == last TX + 300 ms, red without the rescheduleTxLocked call in revertEchoSlowdownLocked (deadline stays at +1 s). Increase: the precondition Fatalf requires last TX + 1 s once the slow-down Poll has settled, red if TransmitInterval ignored the raised variable. Clause 2, recalculated when bfd.RemoteMinRxInterval changes: TestRFC5880RemoteMinRxChangeReschedulesNextTx requires the pending deadline at +400 ms after 900 to 400 and at +1.2 s after 400 to 1.2 s, red without the Receive reschedule. Clause 3, equal to the greater: 900 remote over 300 local (RemoteMinRxReductionHonoredImmediately), 1.2 s remote over 300 local, 300 local over 1 us remote (TransmitIntervalFlooredByLocalDesired, which is the negative: a remote value below the local target must not shorten TX, and RequiredMinRx alone must not move it). An increase of DesiredMinTx while Up is held until its Poll ends by Section 6.8.3, judged under RFC5880-6.8.3-3.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880TransmitIntervalFlooredByLocalDesired`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L538) | unit/verify | unproven |
| positive | [`TestRFC5880DesiredMinTxChangeReschedulesNextTx`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L1337) | unit/verify | revert, verified |
| positive | [`TestRFC5880RemoteMinRxChangeReschedulesNextTx`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L1290) | unit/verify | revert, verified |
| positive | [`TestRFC5880RemoteMinRxReductionHonoredImmediately`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L504) | unit/verify | unproven |

### [`RFC5880-6.8.7-5`](#rfc5880-6.8.7-5)

A system MUST NOT transmit BFD Control packets if bfd.RemoteDiscr is zero and the system is taking the Passive role. (§6.8.7)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. SB-1 strict re-read 2026-09-27 (D-4 send-back): forbidden: a Passive session with bfd.RemoteDiscr 0 transmitting. The tagged units assert only that NextTxDeadline is zero (Init, after a timeout from Up, after a Detection Time of silence while Down) and non-zero once the discriminator is learned. The producer that withholds the packet is the zero-deadline skip in the engine tick (engine/loop.go tick), which no tagged unit drives, so deleting that skip leaves every unit green: the same gap RFC5880-6.1-2 is weak for.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880ActiveRoleTransmitsWithoutReception`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L369) | unit/verify | unproven |
| negative | [`TestRFC5880PassiveTransmitsAgainOnceRemoteDiscrLearned`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L1436) | unit/verify | revert, verified |
| positive | [`TestRFC5880PassiveRoleSilentUntilReception`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L391) | unit/verify | unproven |
| positive | [`TestRFC5880PassiveSilentOnceRemoteDiscrCleared`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L1396) | unit/verify | revert, verified |

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

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. TestRFC5880DetectionExpiryDownDiagOne drives expiry only from Init; expiry from Up (the row names Init or Up) is not asserted to set Down and diag 1, so a CheckDetection scoped to Init alone passes. The "Demand mode is not active" condition is not asserted either: no unit shows expiry is suppressed or handled differently with Demand active. Negative (ignored when Down) is sound. Re-stamped 2026-09-27 (DF-BFD): the unit bodies changed only in fixtures (a nonzero Your Discriminator in peer Init/Up packets, keys that fit their slot, dropped line numbers); the judgment is unchanged. RA-BFD strict re-read 2026-09-27: weak stands for the reason above.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880DetectionExpiryIgnoredWhenDown`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L333) | unit/verify | unproven |
| positive | [`TestRFC5880DetectionExpiryDownDiagOne`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L312) | unit/verify | unproven |

### [`RFC5880-6.8.5-1`](#rfc5880-6.8.5-1)

When the Echo function is active and a sufficient number of Echo packets have not arrived as they should, the session has gone down -- the local system MUST set bfd.SessionState to Down and bfd.LocalDiag to 2 (Echo Function Failed). (§6.8.5)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. TestRFC5880EchoMissDetectedAndReported calls EchoFail by hand after EchoDetectionExpired; nothing asserts the engine (echo.go:60) calls EchoFail when echoes go missing, so removing that call leaves the test green. Re-stamped 2026-09-27 (DF-BFD): the unit bodies changed only in fixtures (a nonzero Your Discriminator in peer Init/Up packets, keys that fit their slot, dropped line numbers); the judgment is unchanged. RA-BFD strict re-read 2026-09-27: weak stands for the reason above.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880EchoReturnClearsMissAndFailIsScoped`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L998) | unit/verify | unproven |
| positive | [`TestRFC5880EchoMissDetectedAndReported`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L962) | unit/verify | unproven |

### [`RFC5880-6.8.15-1`](#rfc5880-6.8.15-1)

When the forwarding plane in the local system is reset for some reason, such that the remote system can no longer rely on the local forwarding state, the local system MUST set bfd.LocalDiag to 4 (Forwarding Plane Reset), and set bfd.SessionState to Down. (§6.8.15)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5880-6.8.15-1, so no unit is bound to it.

### [`RFC5880-6.8.16-1`](#rfc5880-6.8.16-1)

There may be circumstances where it is desirable to administratively enable or disable a BFD session.  When this is desired, the following procedure MUST be followed (§6.8.16)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. TestRFC5880AdministrativeDisableEnable asserts AdminDown state, diag and enable-to-Down, but not the procedure's 'Cease the transmission of BFD Echo packets' step on disable.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880AdministrativeCallsAreGuarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L1125) | unit/verify | unproven |
| positive | [`TestRFC5880AdministrativeDisableEnable`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L1090) | unit/verify | unproven |

### [`RFC5880-6.8.8-1`](#rfc5880-6.8.8-1)

A received BFD Echo packet MUST be demultiplexed to the appropriate session for processing. (§6.8.8)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. TestRFC5880EchoDemultiplexedToItsSession runs a loop holding one session and asserts OnEchoRx fired once; code delivering every echo to its only session also passes. No two-session case proves the echo reaches the APPROPRIATE session. Negative (no-session drop) is sound.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880UnknownEchoDropped`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L626) | unit/verify | unproven |
| positive | [`TestRFC5880EchoDemultiplexedToItsSession`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L589) | unit/verify | unproven |

### [`RFC5880-6.8.8-2`](#rfc5880-6.8.8-2)

A means of detecting missing Echo packets MUST be implemented, which most likely involves processing of the Echo packets that are received. (§6.8.8)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. RA-BFD strict re-read 2026-09-27, lowered from enforced. The units prove the detector in isolation: TestRFC5880EchoMissDetectedAndReported fails if EchoDetectionExpired stays false for an unreturned echo past the echo detection time, and TestRFC5880EchoReturnClearsMissAndFailIsScoped fails if a MatchEchoRx-cleared echo still counts. Both call RegisterEchoTx, MatchEchoRx and EchoDetectionExpired by hand. Nothing tagged shows the running engine consults the detector (echoTickLocked, echo.go) or feeds returning echoes into MatchEchoRx, so deleting either call leaves both units green: the same gap RFC5880-6.8.5-1 is weak for.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880EchoReturnClearsMissAndFailIsScoped`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L993) | unit/verify | unproven |
| positive | [`TestRFC5880EchoMissDetectedAndReported`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L957) | unit/verify | unproven |

### [`RFC5880-6.8.9-1`](#rfc5880-6.8.9-1)

BFD Echo packets MUST NOT be transmitted when bfd.SessionState is not Up. (§6.8.9)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. "Not Up" covers Down, Init and AdminDown. TestRFC5880NoEchoTransmittedWhenNotUp drives only AdminDown, so an echoTickLocked gate that skipped AdminDown alone passes it; no unit shows a Down or Init session sends no echo. Positive TestRFC5880EchoTransmittedWhileUp is sound.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880NoEchoTransmittedWhenNotUp`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L564) | unit/verify | unproven |
| positive | [`TestRFC5880EchoTransmittedWhileUp`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L543) | unit/verify | unproven |

### [`RFC5880-6.8.9-2`](#rfc5880-6.8.9-2)

BFD Echo packets MUST NOT be transmitted unless the last BFD Control packet received from the remote system contains a nonzero value in Required Min Echo RX Interval. (§6.8.9)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. RA-BFD strict re-read 2026-09-27, lowered from enforced. Forbidden: echo TX when the LAST received Control packet carried Required Min Echo RX zero. TestRFC5880EchoNotTransmittedWithoutPeerAdvertisement fails if EchoEnabled is true or PrimeEcho arms a deadline after a single zero packet, and TestRFC5880EchoEnabledWhenPeerAdvertisesNonZero arms on 75 ms; the engine (echoTickLocked) sends only from that deadline behind its own EchoEnabled gate. But neither tagged unit sends a zero after a nonzero, so a latch that kept echo on after any earlier nonzero value passes both: the "last" clause is proven only by TestRFC5880EchoCeasesWhenPeerAdvertisesZero, which carries no tag for this row.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880EchoNotTransmittedWithoutPeerAdvertisement`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L892) | unit/verify | unproven |
| positive | [`TestRFC5880EchoEnabledWhenPeerAdvertisesNonZero`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L871) | unit/verify | unproven |

### [`RFC5880-6.8.9-3`](#rfc5880-6.8.9-3)

The interval between transmitted BFD Echo packets MUST NOT be less than the value advertised by the remote system in Required Min Echo RX Interval (§6.8.9)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. RA-BFD2 strict re-read 2026-09-27. The RA-BFD code defect is fixed: Receive calls rescheduleEchoLocked on a changed RemoteMinEchoRxInterval, moving nextEchoAt to lastEchoAt + EchoInterval (TestRFC5880EchoRaisedPeerFloorDelaysNextEcho: no echo at 60 or 199 ms, one at 200 ms; TestRFC5880EchoLoweredPeerFloorTakesEffect). Still weak: the steady-state spacing, with no change of the peer value, is asserted only through the value EchoInterval returns. Observed: an overlay making AdvanceEcho schedule from DesiredMinEchoTxInterval alone (10 ms under the peer's 50 ms) leaves all four tagged units green (work/agents/RA-BFD2, job-ra-bfd2-adv log), because both engine tests reschedule from lastEchoAt before the next tick. Missing: an engine test that ticks between two echoes with no floor change and asserts no echo before the peer's value.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880EchoLoweredPeerFloorTakesEffect`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L828) | unit/verify | revert, verified |
| negative | [`TestRFC5880EchoIntervalNotBelowLocalTarget`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L934) | unit/verify | unproven |
| positive | [`TestRFC5880EchoRaisedPeerFloorDelaysNextEcho`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L796) | unit/verify | revert, verified |
| positive | [`TestRFC5880EchoIntervalHonorsPeerFloor`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L915) | unit/verify | unproven |

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

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. SB-1 strict re-read 2026-09-27 (D-4 send-back): the row cites Section 4.3 (Keyed MD5) and Section 4.4 (Keyed SHA1). The only tagged unit, TestRFC5880KeyedMD5SectionHeader, Fatalf on buf[off+3] != 0 for Auth Types 2 and 3 only; no tagged unit reads the Reserved byte of a SHA1 section, so the Section 4.4 half rests on the shared digestSigner.Sign line, which is an argument, not an assertion.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC5880KeyedMD5SectionHeader`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L91) | unit/verify | unproven |

### [`RFC5880-9-2`](#rfc5880-9-2)

When a BFD session is directly connected across a single link (physical, or a secure tunnel such as IPsec), the TTL or Hop Count MUST be set to the maximum on transmit, and checked to be equal to the maximum value on reception (and the packet dropped if this is not the case). (§9)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. transport TestRFC5880SingleHopTransmitTTLIsMaximum pins IP_TTL 255 on the IPv4 socket only (no IPv6 Hop Limit); engine TestRFC5880SingleHopReceiveTTLNotMaxDiscarded proves non-255 is dropped, but no tagged unit shows a TTL-255 packet accepted, so a gate that dropped every packet would still pass the receive half

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880SingleHopReceiveTTLNotMaxDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L160) | unit/verify | unproven |
| positive | [`TestRFC5880SingleHopTransmitTTLIsMaximum`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/transport/rfc5880_test.go#L43) | unit/verify | unproven |

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

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. single positive TestRFC5880RemoteMinRxReductionHonoredImmediately proves a remote Required Min RX reduction changes TransmitInterval at once; the quoted sentence also covers the Detection Time, and no tagged unit changes a parameter and asserts DetectionInterval moves immediately

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC5880RemoteMinRxReductionHonoredImmediately`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L506) | unit/verify | unproven |

### [`RFC5880-6.7.3-7`](#rfc5880-6.7.3-7)

the management interface by which the key is configured MUST accept ASCII strings (§6.7.3)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Positive TestRFC5880KeyManagementAcceptsASCIIStrings proves parseAuthConfig keeps an ASCII MD5 key verbatim, called directly rather than through the YANG secret leaf or the config entry the operator uses. Negative TestRFC5880KeyManagementRejectsIncompleteConfig rejects missing or unknown fields, a neighbouring rule; no {single-polarity} marker. Re-stamped 2026-09-27 (DF-BFD): the unit bodies changed only in fixtures (a nonzero Your Discriminator in peer Init/Up packets, keys that fit their slot, dropped line numbers); the judgment is unchanged. RA-BFD strict re-read 2026-09-27: weak stands for the reason above.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880KeyManagementRejectsIncompleteConfig`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5880_test.go#L64) | unit/verify | unproven |
| positive | [`TestRFC5880KeyManagementAcceptsASCIIStrings`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5880_test.go#L26) | unit/verify | unproven |

### [`RFC5880-6.7.2-3`](#rfc5880-6.7.2-3)

The currently selected password and Key ID for the session MUST be stored in the Authentication Section of each outgoing BFD Control packet. (§6.7.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. only TestRFC5880SimplePasswordSectionCarriesPasswordAndKeyID exercises the Simple Password signer the quoted sentence governs; TestRFC5880AuthKeyIDIsTheConfiguredKey and TestRFC5880AuthKeyIDMismatchDiscarded use keyed SHA1 (the section 6.7.4 Key ID obligation) and the negative is a reception discard, so no negative violates the transmit rule

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880AuthKeyIDMismatchDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L267) | unit/verify | unproven |
| positive | [`TestRFC5880AuthKeyIDIsTheConfiguredKey`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L249) | unit/verify | unproven |
| positive | [`TestRFC5880SimplePasswordSectionCarriesPasswordAndKeyID`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L685) | unit/verify | unproven |

### [`RFC5880-6.7.3-8`](#rfc5880-6.7.3-8)

The Sequence Number field MUST be set to bfd.XmitAuthSeq. (§6.7.3, §6.7.4)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. SB-1 strict re-read 2026-09-27 (D-4 send-back): row cite widened to (§6.7.3, §6.7.4): the same sentence is the Keyed SHA1 transmit rule, and the backfill quote had dropped the SHA1 half the paraphrase claimed. Forbidden: a Sequence Number field other than bfd.XmitAuthSeq. Red: TestRFC5880AuthSequenceFieldIsXmitAuthSeq Fatalf unless the field equals 0x11223344, and TestRFC5880AuthSequenceFieldFollowsAdvance Fatalf unless it equals the advanced variable. Both are positive checks of one assertion with two values; no violating input exists and the row carries no {single-polarity} marker. Units sign with keyed SHA1; MD5 shares digestSigner.Sign (auth/sha1.go:84).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880AuthSequenceFieldFollowsAdvance`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L1182) | unit/verify | unproven |
| positive | [`TestRFC5880AuthSequenceFieldIsXmitAuthSeq`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L1159) | unit/verify | unproven |

### [`RFC5880-6.7.2-4`](#rfc5880-6.7.2-4)

If the received BFD Control packet does not contain an Authentication Section, or the Auth Type is not 1 (Simple Password), then the received packet MUST be discarded. (§6.7.2, §6.7.3, §6.7.4)

Audit verdict: wrong (the tests assert something other than what the requirement demands), fresh. the row is Simple Password (Auth Type 1, simpleVerifier.Verify) but both tagged units TestRFC5880AuthTypeMatchAccepted and TestRFC5880AuthTypeMismatchDiscarded drive the keyed SHA1 digestVerifier; no tagged unit feeds simpleVerifier a missing Authentication Section or a non-1 Auth Type

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880AuthTypeMismatchDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L313) | unit/verify | unproven |
| positive | [`TestRFC5880AuthTypeMatchAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L300) | unit/verify | unproven |

### [`RFC5880-6.7.2-5`](#rfc5880-6.7.2-5)

If the Auth Key ID field does not match the ID of a configured password, the received packet MUST be discarded. (§6.7.2, §6.7.3, §6.7.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: accepting a Simple Password packet whose Auth Key ID is not the configured one. Red: TestRFC5880SimplePasswordWrongKeyIDDiscarded signs key 7 with the right password against a verifier for key 8 and Fatalf unless ErrPasswordMismatch (only the key id differs, so isolated); positive TestRFC5880SimplePasswordMatchingKeyIDAccepted accepts key 7. The SHA1 units AuthKeyIDMatchAccepted/MismatchDiscarded prove the same rule for digestVerifier (the 6.7.3/6.7.4 twin), not Simple Password.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880AuthKeyIDMismatchDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L270) | unit/verify | unproven |
| negative | [`TestRFC5880SimplePasswordWrongKeyIDDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L758) | unit/verify | unproven |
| positive | [`TestRFC5880AuthKeyIDMatchAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L288) | unit/verify | unproven |
| positive | [`TestRFC5880SimplePasswordMatchingKeyIDAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L775) | unit/verify | unproven |

### [`RFC5880-6.7.2-6`](#rfc5880-6.7.2-6)

If the Auth Len field is not equal to the length of the password selected by the key ID, plus three, the packet MUST be discarded. (§6.7.2, §6.7.3, §6.7.4)

Audit verdict: wrong (the tests assert something other than what the requirement demands), fresh. the row is the Simple Password rule Auth Len equals password length plus three, but TestRFC5880AuthLenExpectedAccepted and TestRFC5880AuthLenMismatchDiscarded check the keyed SHA1 fixed length 28 on digestVerifier; the Simple Password length check is exercised only under the RFC5880-6.7.2-8 tag

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880AuthLenMismatchDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L346) | unit/verify | unproven |
| positive | [`TestRFC5880AuthLenExpectedAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L329) | unit/verify | unproven |

### [`RFC5880-6.7.2-7`](#rfc5880-6.7.2-7)

If the Password field does not match the password selected by the key ID, the packet MUST be discarded. (§6.7.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: accepting a password other than the configured one. Red: TestRFC5880SimplePasswordMismatchDiscarded Fatalf unless ErrPasswordMismatch for a same-length one-byte difference (isolated from the Auth Len check), plus shorter and longer passwords; positive TestRFC5880SimplePasswordMatchAccepted round-trips 1, 8 and 16-byte passwords. One key per session, so 'selected by the key ID' is the configured pair.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880SimplePasswordMismatchDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L727) | unit/verify | unproven |
| positive | [`TestRFC5880SimplePasswordMatchAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L708) | unit/verify | unproven |

### [`RFC5880-6.7.3-9`](#rfc5880-6.7.3-9)

For Keyed MD5, if the sequence number lies outside of the range of bfd.RcvAuthSeq to bfd.RcvAuthSeq+(3*Detect Mult) inclusive (when treated as an unsigned 32-bit circular number space), the received packet MUST be discarded. (§6.7.3)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. RA-FIX re-read 2026-09-27. SeqState.Check (auth/meticulous.go) discards when the circular distance from bfd.RcvAuthSeq exceeds 3 * the packet's Detect Mult. The row is Keyed MD5. MD5 is exercised only by the 10-ahead negative (TestRFC5880KeyedSequenceBeyondWindowDiscarded); the inclusive-boundary positives (equal, +9, TestRFC5880KeyedSequenceAtOrAboveFloorAccepted), the wrap positive and the below-floor negative run Keyed SHA1 only. And every tagged unit signs with one Detect Mult (3), so the '3*Detect Mult' clause is never varied: a verifier with a hard-coded window of 9, or one that read the local rather than the received Detect Mult, stays green.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880KeyedSequenceBelowFloorDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L560) | unit/verify | revert, verified |
| negative | [`TestRFC5880KeyedSequenceBeyondWindowDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L837) | unit/verify | revert, verified |
| positive | [`TestRFC5880KeyedSequenceAtOrAboveFloorAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L539) | unit/verify | revert, verified |
| positive | [`TestRFC5880KeyedSequenceWindowWraps`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L862) | unit/verify | revert, verified |

### [`RFC5880-6.7.3-10`](#rfc5880-6.7.3-10)

For Meticulous Keyed MD5, if the sequence number lies outside of the range of bfd.RcvAuthSeq+1 to bfd.RcvAuthSeq+(3*Detect Mult) inclusive (when treated as an unsigned 32-bit circular number space) the received packet MUST be discarded. (§6.7.3)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. RA-FIX re-read 2026-09-27. Both Meticulous types are driven: TestRFC5880MeticulousSequenceInsideWindowAccepted accepts +1, +9 and the wrap 0xFFFFFFFF to 0; TestRFC5880MeticulousSequenceOutsideWindowDiscarded discards +10, equal and behind with the floor unmoved, so the lower bound RcvAuthSeq+1 is pinned. But every unit signs with Detect Mult 3, so the '3*Detect Mult' upper bound is never varied: a hard-coded window of 9, or one computed from the local Detect Mult, leaves both units green.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880MeticulousSequenceOutsideWindowDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L906) | unit/verify | revert, verified |
| positive | [`TestRFC5880MeticulousSequenceInsideWindowAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L883) | unit/verify | revert, verified |

### [`RFC5880-6.7.3-11`](#rfc5880-6.7.3-11)

Otherwise (the digest does not match the Auth Key/Digest field), the received packet MUST be discarded. (§6.7.3, §6.7.4)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. RA-BFD strict re-read 2026-09-27, lowered from enforced. The quote is Section 6.7.3's Keyed MD5 sentence (the Auth Key/Digest field). Both units drive only keyed SHA1: TestRFC5880DigestMismatchDiscarded requires ErrDigestMismatch for a flipped digest byte and a wrong-key packet, and that the floor stays at 40; TestRFC5880DigestMatchAccepted accepts the matching digest. The shared digestVerifier.Verify compare is exercised, but the MD5-specific parts (md5Sum, AuthLenKeyedMD5 through newMD5Verifier) are not: an md5Sum that returned a fixed digest would accept a forged MD5 packet with every tagged unit green.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880DigestMismatchDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L393) | unit/verify | revert, verified |
| positive | [`TestRFC5880DigestMatchAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L380) | unit/verify | unproven |

### [`RFC5880-6.7.3-12`](#rfc5880-6.7.3-12)

Otherwise (bfd.AuthSeqKnown is 0), bfd.AuthSeqKnown MUST be set to 1, and bfd.RcvAuthSeq MUST be set to the value of the received Sequence Number field. (§6.7.3, §6.7.4)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. SB-1 strict re-read 2026-09-27 (D-4 send-back): the quoted sentence is the Section 6.7.3 Keyed MD5 rule, and the row also cites Section 6.7.4. Every tagged unit (TestRFC5880FirstPacketSeedsReplayFloorBeforeDigest, TestRFC5880FirstAuthenticatedPacketSeedsReplayFloor, TestRFC5880KnownSequenceNotReseededByForgedPacket) configures Settings{Type: AuthTypeKeyedSHA1}; no MD5 verifier seeds bfd.AuthSeqKnown or bfd.RcvAuthSeq in a tagged unit, so the Section 6.7.3 half rests on digestVerifier being shared. For SHA1 both halves are proven: AuthSeqKnown 1 and RcvAuthSeq = received after the first packet (also before the digest compare, owner decision D-14), and no reseed once AuthSeqKnown is 1.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880KnownSequenceNotReseededByForgedPacket`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L513) | unit/verify | revert, verified |
| positive | [`TestRFC5880FirstAuthenticatedPacketSeedsReplayFloor`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L455) | unit/verify | revert, verified |
| positive | [`TestRFC5880FirstPacketSeedsReplayFloorBeforeDigest`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L479) | unit/verify | revert, verified |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | ze-work agent, spec-fixit-rfc-drain-quota-never-armed WP-1 |
| Signed off | 2026-08-31 |
| Register | rfc2119 |
| Source | rfc/full/rfc5880.txt |
| Source fingerprint | 9a3492d0917193af |
| Record | rfc/extraction/rfc5880.json |
| Mapped sentences | 99 |
| Declined as scope | 29 |
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
| `6.7.4` | Keyed SHA1 and Meticulous Keyed SHA1 Authentication | 17 | walked | Keyed SHA1 and Meticulous Keyed SHA1 Authentication. Seventeen sites that repeat the section 6.7.3 rules with SHA1 constants, so most are excluded as duplicates of the rows the summary declares once for both families. |
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
| `6.8.16` | Administrative Control | 1 | walked | Administrative Control. One site, the enable and disable procedure. The transmission after entering AdminDown is advisory. |
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
| `6.7.3:10` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The Auth Type reception discard for the MD5 family. RFC5880-6.7.2-4 declares it for all three families and site 6.7.2:6 maps it. | If the received BFD Control packet does not contain an Authentication Section, or the Auth Type is not correct (2 for Keyed MD5 or 3 for Meticulous Keyed MD5), then the received packet MUST be discarded. |
| `6.7.3:11` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The Auth Key ID reception discard for the MD5 family, declared once as RFC5880-6.7.2-5 and mapped by site 6.7.2:7. | If the Auth Key ID field does not match the ID of a configured authentication key, the received packet MUST be discarded. |
| `6.7.3:12` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The Auth Len reception discard for the MD5 family, declared once as RFC5880-6.7.2-6 and mapped by site 6.7.2:8. | If the Auth Len field is not equal to 24, the packet MUST be discarded. |
| `6.7.3:16` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The accepting face of the digest comparison. RFC5880-6.7.3-11 states the same comparison as a discard, and site 6.7.3:17 maps it. | If the MD5 digest of the entire BFD Control packet is equal to the received value of the Auth Key/Digest field, the received packet MUST be accepted. |
| `6.7.4:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The length half of the row site 6.7.4:1 maps. | The Auth Len field MUST be set to 28. |
| `6.7.4:3` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The Auth Key ID transmit rule for the SHA1 family, declared once as RFC5880-6.7.2-3 and mapped by site 6.7.3:3. | The Auth Key ID field MUST be set to the ID of the current authentication key. |
| `6.7.4:4` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The Sequence Number transmit rule for the SHA1 family, declared once as RFC5880-6.7.3-8 and mapped by site 6.7.3:4. | The Sequence Number field MUST be set to bfd.XmitAuthSeq. |
| `6.7.4:10` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The Auth Type reception discard for the SHA1 family, declared once as RFC5880-6.7.2-4 and mapped by site 6.7.2:6. | If the received BFD Control packet does not contain an Authentication Section, or the Auth Type is not correct (4 for Keyed SHA1 or 5 for Meticulous Keyed SHA1), then the received packet MUST be discarded. |
| `6.7.4:11` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The Auth Key ID reception discard for the SHA1 family, declared once as RFC5880-6.7.2-5 and mapped by site 6.7.2:7. | If the Auth Key ID field does not match the ID of a configured authentication key, the received packet MUST be discarded. |
| `6.7.4:12` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The Auth Len reception discard for the SHA1 family, declared once as RFC5880-6.7.2-6 and mapped by site 6.7.2:8. | If the Auth Len field is not equal to 28, the packet MUST be discarded. |
| `6.7.4:13` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The Keyed SHA1 sequence window discard. RFC5880-6.7.3-9 declares the rule for MD5 and SHA1 together, and site 6.7.3:13 maps it. | For Keyed SHA1, if the sequence number lies outside of the range of bfd.RcvAuthSeq to bfd.RcvAuthSeq+(3*Detect Mult) inclusive (when treated as an unsigned 32-bit circular number space), the received packet MUST be discarded. |
| `6.7.4:14` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The Meticulous Keyed SHA1 sequence window discard, declared once as RFC5880-6.7.3-10 and mapped by site 6.7.3:14. | For Meticulous Keyed SHA1, if the sequence number lies outside of the range of bfd.RcvAuthSeq+1 to bfd.RcvAuthSeq+(3*Detect Mult) inclusive (when treated as an unsigned 32-bit circular number space, the received packet MUST be discarded. |
| `6.7.4:15` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The first-packet sequence learning rule for the SHA1 family, declared once as RFC5880-6.7.3-12 and mapped by site 6.7.3:15. | Otherwise (bfd.AuthSeqKnown is 0), bfd.AuthSeqKnown MUST be set to 1, bfd.RcvAuthSeq MUST be set to the value of the received Sequence Number field, and the received packet MUST be accepted. |
| `6.7.4:16` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The accepting face of the SHA1 hash comparison, whose discarding face is RFC5880-6.7.3-11, mapped by site 6.7.3:17. | If the SHA1 hash of the entire BFD Control packet is equal to the received value of the Auth Key/Hash field, the received packet MUST be accepted. |
| `6.7.4:17` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The SHA1 hash mismatch discard. RFC5880-6.7.3-11 declares the rule for MD5 and SHA1 together, and site 6.7.3:17 maps it. | Otherwise (the hash does not match the Auth Key/Hash field), the received packet MUST be discarded. |
| `6.8.3:6` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The second sentence of the rule site 6.8.3:5 maps. It states what 'immediately' means when the new interval has already elapsed, and binds nothing further. | If this interval has already passed since the last transmission (because the new interval is significantly shorter), the local system MUST send the next periodic BFD Control packet as soon as practicable. |
| `6.8.4:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The Demand mode detection expiry. The action is the same obligation RFC5880-6.8.4-1 states, set Down and diagnostic 1; only the clock the Detection Time runs from differs, and section 6.6 and section 6.8.4 both state that clock. | If Demand mode is active, and a period of time equal to the Detection Time passes after the initiation of a Poll Sequence (the transmission of the first BFD Control packet with the Poll bit set), the session has gone down -- the local system MUST set bfd.SessionState to Down, and bfd.LocalDiag to 1 (Control Detection Time Expired). |
| `6.8.6:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The lead-in to the reception procedure. Its steps are declared as RFC5880-6.8.6-1 through RFC5880-6.8.6-15, in the order the summary lists them, and site 6.8.6:3 maps the first. | When a BFD Control packet is received, the following procedure MUST be followed, in the order specified. |
| `6.8.6:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The stop-on-discard clause of the same lead-in. Every step it governs is a discard rule already declared, the first of which site 6.8.6:3 maps. | If the packet is discarded according to these rules, processing of the packet MUST cease at that point. |
| `6.8.6:10` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The no-session discard, which RFC5880-6.8.6-7 states as the second half of its own text. Site 6.8.6:9 maps that row. | If no session is found, the packet MUST be discarded. |
| `6.8.7:10` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The Demand bit precondition, restated here in the transmission rules. RFC5880-6.6-1 cites section 6.6 and section 6.8.7, and site 6.6:1 maps it. | A system MUST NOT set the Demand (D) bit unless bfd.DemandMode is 1, bfd.SessionState is Up, and bfd.RemoteSessionState is Up. |
| `6.8.18:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The section 6.8.1 state preservation obligation, restated here at REQUIRED as the first of the two holddown mechanisms. Site 6.8.1:1 maps RFC5880-6.8.1-14. | First, a system is REQUIRED to maintain session state (including timing parameters), even when a session is down, until a Detection Time has passed without the receipt of any BFD Control packets. |

## Superseded

No document obsoletes RFC 5880, so its obligations are stated where they were written.
