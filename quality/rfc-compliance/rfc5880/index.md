# RFC 5880 - Bidirectional Forwarding Detection (BFD)

Partial. Every requirement this repository extracted from RFC 5880, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 76.8% | 73 of 95 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 8.4% | 8 of 95 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 95 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 0.0% | 0 of 159 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 96 | of 119 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 1 | of 96 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 14.7% | 14 of 95 binding obligations | no test carries the requirement id, whether or not a gap states why |

The 4 shares marked as a part above are the whole of the 95 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 119 |
| Gated MUST-level | 96 |
| Obligations that bind Ze | 95 |
| Not applicable, so out of scope | 1 |
| Declared gaps | 14 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 159 |
| Tagged units | 159 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
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

Fourteen MUST gaps, gated in [`rfc/short/rfc5880.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc5880.md): Demand mode is not driven -- bfd.DemandMode has no writer and the stored remote D bit is never read, so no Poll is raised on a D-bit or content change and periodic transmission is never suppressed ([`RFC5880-6.6-2`](#rfc5880-6.6-2), 6.6-3, 6.8.6-14, 6.8.7-7, 6.8.17-1); periodic transmission also continues when bfd.RemoteMinRxInterval is zero ([`RFC5880-6.8.7-6`](#rfc5880-6.8.7-6)); the two timing changes the RFC excepts from immediate effect are applied immediately instead of at Poll termination ([`RFC5880-6.8.3-3`](#rfc5880-6.8.3-3), 6.8.3-4); bfd.XmitAuthSeq starts at zero rather than a random value and bfd.AuthSeqKnown is never cleared after twice the Detection Time ([`RFC5880-6.8.1-11`](#rfc5880-6.8.1-11), 6.8.1-13); the meticulous replay check accepts any strictly greater sequence rather than exactly RcvAuthSeq+1 ([`RFC5880-6.7.3-10`](#rfc5880-6.7.3-10)); there is no forwarding-plane-reset hook, so diagnostic 4 has no producer ([`RFC5880-6.8.15-1`](#rfc5880-6.8.15-1)); and no congestion-control mechanism governs the transmit rate ([`RFC5880-7-1`](#rfc5880-7-1), 7-2). Final-packet rate limiting is absent by design and annotated not-applicable. IPv6 transport coverage is tracked with BFD.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 73 | one part of the gated population |
| Annotated instead of tested | 23 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **96** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (73):** [`RFC5880-4.1-1`](#rfc5880-4.1-1), [`RFC5880-4.1-2`](#rfc5880-4.1-2), [`RFC5880-6.3-1`](#rfc5880-6.3-1), [`RFC5880-6.8.1-3`](#rfc5880-6.8.1-3), [`RFC5880-6.8.1-4`](#rfc5880-6.8.1-4), [`RFC5880-6.8.1-5`](#rfc5880-6.8.1-5), [`RFC5880-6.8.1-6`](#rfc5880-6.8.1-6), [`RFC5880-6.8.1-7`](#rfc5880-6.8.1-7), [`RFC5880-6.8.1-8`](#rfc5880-6.8.1-8), [`RFC5880-6.8.1-9`](#rfc5880-6.8.1-9), [`RFC5880-6.8.1-10`](#rfc5880-6.8.1-10), [`RFC5880-6.8.1-12`](#rfc5880-6.8.1-12), [`RFC5880-6.8.1-14`](#rfc5880-6.8.1-14), [`RFC5880-6.1-1`](#rfc5880-6.1-1), [`RFC5880-6.1-2`](#rfc5880-6.1-2), [`RFC5880-6.1-3`](#rfc5880-6.1-3), [`RFC5880-6.8.3-1`](#rfc5880-6.8.3-1), [`RFC5880-6.8.3-2`](#rfc5880-6.8.3-2), [`RFC5880-6.8.3-5`](#rfc5880-6.8.3-5), [`RFC5880-6.5-1`](#rfc5880-6.5-1), [`RFC5880-6.5-2`](#rfc5880-6.5-2), [`RFC5880-6.6-1`](#rfc5880-6.6-1), [`RFC5880-6.7-1`](#rfc5880-6.7-1), [`RFC5880-4.2-1`](#rfc5880-4.2-1), [`RFC5880-6.7.2-1`](#rfc5880-6.7.2-1), [`RFC5880-6.7.2-8`](#rfc5880-6.7.2-8), [`RFC5880-6.7.3-1`](#rfc5880-6.7.3-1), [`RFC5880-6.7.3-2`](#rfc5880-6.7.3-2), [`RFC5880-6.7.3-4`](#rfc5880-6.7.3-4), [`RFC5880-6.7.4-1`](#rfc5880-6.7.4-1), [`RFC5880-6.7.4-2`](#rfc5880-6.7.4-2), [`RFC5880-6.7.4-4`](#rfc5880-6.7.4-4), [`RFC5880-6.7.4-5`](#rfc5880-6.7.4-5), [`RFC5880-6.8.6-1`](#rfc5880-6.8.6-1), [`RFC5880-6.8.6-2`](#rfc5880-6.8.6-2), [`RFC5880-6.8.6-3`](#rfc5880-6.8.6-3), [`RFC5880-6.8.6-4`](#rfc5880-6.8.6-4), [`RFC5880-6.8.6-5`](#rfc5880-6.8.6-5), [`RFC5880-6.8.6-6`](#rfc5880-6.8.6-6), [`RFC5880-6.8.6-7`](#rfc5880-6.8.6-7), [`RFC5880-6.8.6-8`](#rfc5880-6.8.6-8), [`RFC5880-6.8.6-9`](#rfc5880-6.8.6-9), [`RFC5880-6.8.6-10`](#rfc5880-6.8.6-10), [`RFC5880-6.8.6-11`](#rfc5880-6.8.6-11), [`RFC5880-6.8.6-12`](#rfc5880-6.8.6-12), [`RFC5880-6.8.6-13`](#rfc5880-6.8.6-13), [`RFC5880-6.8.6-15`](#rfc5880-6.8.6-15), [`RFC5880-6.8.6-16`](#rfc5880-6.8.6-16), [`RFC5880-6.8.6-18`](#rfc5880-6.8.6-18), [`RFC5880-6.8.7-1`](#rfc5880-6.8.7-1), [`RFC5880-6.8.7-2`](#rfc5880-6.8.7-2), [`RFC5880-6.8.7-3`](#rfc5880-6.8.7-3), [`RFC5880-6.8.7-4`](#rfc5880-6.8.7-4), [`RFC5880-6.8.7-5`](#rfc5880-6.8.7-5), [`RFC5880-6.8.4-1`](#rfc5880-6.8.4-1), [`RFC5880-6.8.5-1`](#rfc5880-6.8.5-1), [`RFC5880-6.8.16-1`](#rfc5880-6.8.16-1), [`RFC5880-6.8.8-1`](#rfc5880-6.8.8-1), [`RFC5880-6.8.8-2`](#rfc5880-6.8.8-2), [`RFC5880-6.8.9-1`](#rfc5880-6.8.9-1), [`RFC5880-6.8.9-2`](#rfc5880-6.8.9-2), [`RFC5880-6.8.9-3`](#rfc5880-6.8.9-3), [`RFC5880-9-2`](#rfc5880-9-2), [`RFC5880-6.7.3-7`](#rfc5880-6.7.3-7), [`RFC5880-6.7.2-3`](#rfc5880-6.7.2-3), [`RFC5880-6.7.3-8`](#rfc5880-6.7.3-8), [`RFC5880-6.7.2-4`](#rfc5880-6.7.2-4), [`RFC5880-6.7.2-5`](#rfc5880-6.7.2-5), [`RFC5880-6.7.2-6`](#rfc5880-6.7.2-6), [`RFC5880-6.7.2-7`](#rfc5880-6.7.2-7), [`RFC5880-6.7.3-9`](#rfc5880-6.7.3-9), [`RFC5880-6.7.3-11`](#rfc5880-6.7.3-11), [`RFC5880-6.7.3-12`](#rfc5880-6.7.3-12)

**Annotated instead of tested (23):** [`RFC5880-6.8.1-1`](#rfc5880-6.8.1-1), [`RFC5880-6.8.1-2`](#rfc5880-6.8.1-2), [`RFC5880-6.8.1-11`](#rfc5880-6.8.1-11), [`RFC5880-6.8.1-13`](#rfc5880-6.8.1-13), [`RFC5880-6.8.3-3`](#rfc5880-6.8.3-3), [`RFC5880-6.8.3-4`](#rfc5880-6.8.3-4), [`RFC5880-6.8.3-6`](#rfc5880-6.8.3-6), [`RFC5880-6.6-2`](#rfc5880-6.6-2), [`RFC5880-6.6-3`](#rfc5880-6.6-3), [`RFC5880-6.7.3-3`](#rfc5880-6.7.3-3), [`RFC5880-6.7.4-3`](#rfc5880-6.7.4-3), [`RFC5880-6.8.6-14`](#rfc5880-6.8.6-14), [`RFC5880-6.8.7-6`](#rfc5880-6.8.7-6), [`RFC5880-6.8.7-7`](#rfc5880-6.8.7-7), [`RFC5880-6.8.7-8`](#rfc5880-6.8.7-8), [`RFC5880-6.8.15-1`](#rfc5880-6.8.15-1), [`RFC5880-7-1`](#rfc5880-7-1), [`RFC5880-7-2`](#rfc5880-7-2), [`RFC5880-4.3-1`](#rfc5880-4.3-1), [`RFC5880-6.8.14-1`](#rfc5880-6.8.14-1), [`RFC5880-6.8.17-1`](#rfc5880-6.8.17-1), [`RFC5880-6.8.3-8`](#rfc5880-6.8.3-8), [`RFC5880-6.7.3-10`](#rfc5880-6.7.3-10)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC5880-4.1-1` | Version field must be 1 (§4.1, §6.8.6) | MUST | 4.1 - Generic BFD Control Packet Format | **positive:** `unit/verify` [`TestRFC5880VersionOneAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/packet/rfc5880_test.go#L44). **negative:** `unit/verify` [`TestRFC5880VersionNotOneDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/packet/rfc5880_test.go#L64) |
| `RFC5880-4.1-2` | Multipoint (M) bit must be zero on both transmit and receipt (§4.1) | MUST | 4.1 - Generic BFD Control Packet Format | **positive:** `unit/verify` [`TestRFC5880MultipointZeroAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/packet/rfc5880_test.go#L80). **negative:** `unit/verify` [`TestRFC5880MultipointSetDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/packet/rfc5880_test.go#L96) |
| `RFC5880-6.3-1` | My Discriminator must be nonzero and unique across all BFD sessions on the system (§6.3) | MUST | 6.3 - Demultiplexing and the Discriminator Fields | **positive:** `unit/verify` [`TestRFC5880DiscriminatorsAreNonZeroAndUnique`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L62). **negative:** `unit/verify` [`TestRFC5880DiscriminatorAllocatorSkipsReservedAndTaken`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L90) |
| `RFC5880-6.8.1-1` | bfd.SessionState must be initialized to Down (§6.8.1) | MUST | 6.8.1 - State Variables | **positive:** `unit/verify` [`TestRFC5880InitStatesAreDown`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L73). **negative:** no negative test. **{single-polarity}:** Init assigns bfd.SessionState = Down unconditionally (internal/component/bfd/session/session.go:246) before any packet can be exchanged, so no non-conformant input exists to reject |
| `RFC5880-6.8.1-2` | bfd.RemoteSessionState must be initialized to Down (§6.8.1) | MUST | 6.8.1 - State Variables | **positive:** `unit/verify` [`TestRFC5880InitStatesAreDown`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L76). **negative:** no negative test. **{single-polarity}:** Init assigns bfd.RemoteSessionState = Down unconditionally (internal/component/bfd/session/session.go:247), so there is no non-conformant input to reject |
| `RFC5880-6.8.1-3` | bfd.LocalDiscr must be unique across all BFD sessions on the system, and nonzero (§6.8.1) | MUST | 6.8.1 - State Variables | **positive:** `unit/verify` [`TestRFC5880DiscriminatorsAreNonZeroAndUnique`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L67). **negative:** `unit/verify` [`TestRFC5880DiscriminatorAllocatorSkipsReservedAndTaken`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L95) |
| `RFC5880-6.8.1-4` | bfd.RemoteDiscr must be initialized to zero (§6.8.1) | MUST | 6.8.1 - State Variables | **positive:** `unit/verify` [`TestRFC5880InitVariableDefaults`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L88). **negative:** `unit/verify` [`TestRFC5880InitVariablesAreNotConstants`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L113) |
| `RFC5880-6.8.1-5` | bfd.RemoteDiscr must be set to zero if no valid packet received for one Detection Time (§6.8.1) | MUST | 6.8.1 - State Variables | **positive:** `unit/verify` [`TestRFC5880RemoteDiscrClearedOnDetectionExpiry`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L153). **negative:** `unit/verify` [`TestRFC5880RemoteDiscrKeptOnNeighborSignaledDown`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L180) |
| `RFC5880-6.8.1-6` | bfd.LocalDiag must be initialized to zero (§6.8.1) | MUST | 6.8.1 - State Variables | **positive:** `unit/verify` [`TestRFC5880InitVariableDefaults`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L90). **negative:** `unit/verify` [`TestRFC5880InitVariablesAreNotConstants`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L116) |
| `RFC5880-6.8.1-7` | bfd.DesiredMinTxInterval must be initialized to at least 1,000,000 microseconds (§6.8.1, §6.8.3) | MUST | 6.8.1 - State Variables | **positive:** `unit/verify` [`TestRFC5880SlowStartFloorWhileNotUp`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L208). **negative:** `unit/verify` [`TestRFC5880SlowStartFloorLiftedWhenUp`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L228) |
| `RFC5880-6.8.1-8` | bfd.RemoteMinRxInterval must be initialized to 1 (§6.8.1) | MUST | 6.8.1 - State Variables | **positive:** `unit/verify` [`TestRFC5880InitVariableDefaults`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L92). **negative:** `unit/verify` [`TestRFC5880InitVariablesAreNotConstants`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L118) |
| `RFC5880-6.8.1-9` | bfd.RemoteDemandMode must be initialized to zero (§6.8.1) | MUST | 6.8.1 - State Variables | **positive:** `unit/verify` [`TestRFC5880InitVariableDefaults`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L94). **negative:** `unit/verify` [`TestRFC5880InitVariablesAreNotConstants`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L121) |
| `RFC5880-6.8.1-10` | bfd.DetectMult must be a nonzero integer (§6.8.1) | MUST | 6.8.1 - State Variables | **positive:** `unit/verify` [`TestRFC5880DetectMultConfiguredValue`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L248). **negative:** `unit/verify` [`TestRFC5880DetectMultZeroRequestSubstituted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L259) |
| `RFC5880-6.8.1-11` | bfd.XmitAuthSeq must be initialized to a random 32-bit value (§6.8.1) | MUST | 6.8.1 - State Variables | **positive:** no positive test. **negative:** no negative test. **{gap}:** SetAuth seeds bfd.XmitAuthSeq from the persister or leaves the Vars zero value (internal/component/bfd/session/auth.go:36) and Init never randomizes it (internal/component/bfd/session/session.go:245-259), so the initial transmit sequence is 0 rather than a random 32-bit value |
| `RFC5880-6.8.1-12` | bfd.AuthSeqKnown must be initialized to zero (§6.8.1) | MUST | 6.8.1 - State Variables | **positive:** `unit/verify` [`TestRFC5880AuthSeqKnownStartsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L418). **negative:** `unit/verify` [`TestRFC5880AuthSeqKnownSetAfterFirstPacket`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L436) |
| `RFC5880-6.8.1-13` | bfd.AuthSeqKnown must be set to zero after no packets received for twice the Detection Time (§6.8.1) | MUST | 6.8.1 - State Variables | **positive:** no positive test. **negative:** no negative test. **{gap}:** bfd.AuthSeqKnown is SeqState.initialized, which only Advance sets (internal/component/bfd/auth/meticulous.go:63-66) and nothing ever clears; CheckDetection clears the detection deadline alone (internal/component/bfd/session/timers.go:82), so the flag survives twice the Detection Time of silence |
| `RFC5880-6.8.1-14` | Session state must be preserved for at least one Detection Time after last valid packet (§6.8.1) | MUST | 6.8.1 - State Variables | **positive:** `unit/verify` [`TestRFC5880StatePreservedForOneDetectionTime`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L283). **negative:** `unit/verify` [`TestRFC5880DetectionExpiryDownDiagOne`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L308) |
| `RFC5880-6.1-1` | Active role system must send BFD Control packets regardless of whether packets have been received (§6.1) | MUST | 6.1 - Overview | **positive:** `unit/verify` [`TestRFC5880ActiveRoleTransmitsWithoutReception`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L361). **negative:** `unit/verify` [`TestRFC5880PassiveRoleTransmitsAfterReception`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L410) |
| `RFC5880-6.1-2` | Passive role system must not begin sending BFD packets until it has received one (§6.1) | MUST NOT | 6.1 - Overview | **positive:** `unit/verify` [`TestRFC5880PassiveRoleSilentUntilReception`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L386). **negative:** `unit/verify` [`TestRFC5880PassiveRoleTransmitsAfterReception`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L415) |
| `RFC5880-6.1-3` | At least one system must take the Active role (§6.1) | MUST | 6.1 - Overview | **positive:** `unit/verify` [`TestRFC5880ActiveRoleTransmitsWithoutReception`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L366). **negative:** `unit/verify` [`TestRFC5880PassiveRoleSilentUntilReception`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L394) |
| `RFC5880-6.8.3-1` | When session state is not Up, bfd.DesiredMinTxInterval must be at least 1,000,000 microseconds (§6.8.3) | MUST | 6.8.3 - Timer Manipulation | **positive:** `unit/verify` [`TestRFC5880SlowStartFloorWhileNotUp`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L213). **negative:** `unit/verify` [`TestRFC5880SlowStartFloorLiftedWhenUp`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L232) |
| `RFC5880-6.8.3-2` | If bfd.DesiredMinTxInterval or bfd.RequiredMinRxInterval changes, a Poll Sequence must be initiated (§6.8.3) | MUST | 6.8.3 - Timer Manipulation | **positive:** `unit/verify` [`TestRFC5880PollInitiatedOnIntervalChange`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L436). **negative:** `unit/verify` [`TestRFC5880NoPollWhenIntervalsUnchanged`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L470) |
| `RFC5880-6.8.3-3` | If bfd.DesiredMinTxInterval is increased while Up, the actual TX interval must not change until the Poll Sequence terminates (§6.8.3) | MUST NOT | 6.8.3 - Timer Manipulation | **positive:** no positive test. **negative:** no negative test. **{gap}:** ApplyEchoSlowdown raises bfd.DesiredMinTxInterval while the session is Up (internal/component/bfd/session/timers.go:235) and TransmitInterval returns the raised value on the very next call (internal/component/bfd/session/timers.go:44), so the longer interval takes effect before the Poll Sequence terminates |
| `RFC5880-6.8.3-4` | If bfd.RequiredMinRxInterval is reduced while Up, the previous value must be used for detection time until the Poll terminates (§6.8.3) | MUST | 6.8.3 - Timer Manipulation | **positive:** no positive test. **negative:** no negative test. **{gap}:** revertEchoSlowdownLocked reduces bfd.RequiredMinRxInterval while Up (internal/component/bfd/session/timers.go:249) and DetectionInterval immediately uses the reduced value (internal/component/bfd/session/timers.go:29); no previous value is retained until the Poll Sequence terminates |
| `RFC5880-6.8.3-5` | If local system reduces TX interval due to bfd.RemoteMinRxInterval being reduced, it must honor the new interval immediately (§6.8.3) | MUST | 6.8.3 - Timer Manipulation | **positive:** `unit/verify` [`TestRFC5880RemoteMinRxReductionHonoredImmediately`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L497). **negative:** `unit/verify` [`TestRFC5880TransmitIntervalFlooredByLocalDesired`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L533) |
| `RFC5880-6.8.3-6` | Multiple parameter changes requiring Poll Sequence must be communicated in a single packet, or a round-trip must elapse, or an F=0 packet must be received before starting another Poll (§6.8.3) | MUST | 6.8.3 - Timer Manipulation | **positive:** `unit/verify` [`TestRFC5880PollInitiatedOnIntervalChange`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L441). **negative:** no negative test. **{single-polarity}:** onStateChange applies both interval changes and raises exactly one Poll (internal/component/bfd/session/fsm.go:139-144), so ze always takes the single-packet option and no overlapping second Poll Sequence exists to reject |
| `RFC5880-6.5-1` | A BFD Control packet must not have both Poll (P) and Final (F) bits set (§6.5) | MUST NOT | 6.5 - The Poll Sequence | **positive:** `unit/verify` [`TestRFC5880PollPacketHasNoFinalBit`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L567). **negative:** `unit/verify` [`TestRFC5880FinalReplyClearsPollBit`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L586) |
| `RFC5880-6.5-2` | If periodic Control packets are being sent, Poll Sequence must be performed by setting P bit on scheduled transmissions; additional packets must not be sent (§6.5) | MUST | 6.5 - The Poll Sequence | **positive:** `unit/verify` [`TestRFC5880PollPacketHasNoFinalBit`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L571). **negative:** `unit/verify` [`TestRFC5880FinalReplyClearsPollBit`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L590) |
| `RFC5880-6.6-1` | Demand (D) bit must not be set unless bfd.DemandMode is 1, bfd.SessionState is Up, and bfd.RemoteSessionState is Up (§6.6, §6.8.7) | MUST NOT | 6.6 - Demand mode | **positive:** `unit/verify` [`TestRFC5880DemandBitSetWhenAllConditionsHold`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L652). **negative:** `unit/verify` [`TestRFC5880DemandBitClearWhenAnyConditionFails`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L666) |
| `RFC5880-6.6-2` | When the D bit value is to be changed, a Poll Sequence must be initiated (§6.6) | MUST | 6.6 - Demand mode | **positive:** no positive test. **negative:** no negative test. **{gap}:** bfd.DemandMode has no writer in production code -- it is declared at internal/component/bfd/session/session.go:58 and read only by canSetDemand (internal/component/bfd/session/fsm.go:247) -- so no D-bit change path exists and none initiates a Poll Sequence |
| `RFC5880-6.6-3` | If Demand mode is active on either system, a Poll Sequence must be initiated whenever next packet contents would differ (except P/F bits) (§6.6) | MUST | 6.6 - Demand mode | **positive:** no positive test. **negative:** no negative test. **{gap}:** bfd.RemoteDemandMode is stored by Receive (internal/component/bfd/session/fsm.go:58) and read nowhere, and bfd.DemandMode has no writer (internal/component/bfd/session/session.go:58), so no code path starts a Poll when packet contents would change while Demand mode is active |
| `RFC5880-6.7-1` | Implementations supporting authentication must support both types of SHA1 authentication (§6.7) | MUST | 6.7 - Authentication | **positive:** `unit/verify` [`TestRFC5880BothSHA1VariantsSupported`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L48). **negative:** `unit/verify` [`TestRFC5880UnsupportedAuthTypesRejected`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L67) |
| `RFC5880-4.2-1` | Simple Password must be 1 to 16 bytes in length (§4.2, §6.7.2) | MUST | 4.2 - Simple Password Authentication Section Format | **positive:** `unit/verify` [`TestRFC5880SimplePasswordManagementAcceptsASCIIStrings`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5880_test.go#L101). **positive:** `unit/verify` [`TestRFC5880SimplePasswordSectionHeader`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L589). **negative:** `unit/verify` [`auth/TestRFC5880SimplePasswordLengthOutOfRangeRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L744). **negative:** `unit/verify` [`bfd/TestRFC5880SimplePasswordLengthOutOfRangeRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5880_test.go#L128) |
| `RFC5880-6.7.2-1` | Simple Password management interface must accept ASCII strings (§6.7.2) | MUST | 6.7.2 - Simple Password Authentication | **positive:** `unit/verify` [`TestRFC5880SimplePasswordManagementAcceptsASCIIStrings`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5880_test.go#L96). **negative:** `unit/verify` [`bfd/TestRFC5880SimplePasswordLengthOutOfRangeRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5880_test.go#L132) |
| `RFC5880-6.7.2-8` | Simple Password Auth Type must be set to 1, Auth Len must be set to the proper length of 4 to 19 bytes (§6.7.2) | MUST | 6.7.2 - Simple Password Authentication | **positive:** `unit/verify` [`TestRFC5880SimplePasswordSectionHeader`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L584). **negative:** `unit/verify` [`TestRFC5880SimplePasswordRejectsForeignSectionShape`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L610) |
| `RFC5880-6.7.3-1` | Keyed MD5 Auth Type must be set to 2 or 3, Auth Len must be 24 (§6.7.3) | MUST | 6.7.3 - Keyed MD5 and Meticulous Keyed MD5 Authentication | **positive:** `unit/verify` [`TestRFC5880KeyedMD5SectionHeader`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L83). **negative:** `unit/verify` [`TestRFC5880KeyedMD5RejectsForeignSectionShape`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L110) |
| `RFC5880-6.7.3-2` | Keyed MD5 Auth Key/Digest: MD5 digest must be calculated over entire BFD Control packet (§6.7.3) | MUST | 6.7.3 - Keyed MD5 and Meticulous Keyed MD5 Authentication | **positive:** `unit/verify` [`TestRFC5880DigestCoversWholePacket`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L179). **negative:** `unit/verify` [`TestRFC5880DigestRejectsMandatorySectionTamper`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L198) |
| `RFC5880-6.7.3-3` | Keyed MD5: secret key must not be carried in the packet (§6.7.3) | MUST NOT | 6.7.3 - Keyed MD5 and Meticulous Keyed MD5 Authentication | **positive:** `unit/verify` [`TestRFC5880SecretKeyNotCarriedInPacket`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L222). **negative:** no negative test. **{single-polarity}:** Sign overwrites the key scratch with the computed digest before the packet is handed to the transport (internal/component/bfd/auth/sha1.go:85-87), so no key-bearing packet is ever emitted for a receiver to reject |
| `RFC5880-6.7.3-4` | Meticulous Keyed MD5: bfd.XmitAuthSeq must be incremented for each packet (§6.7.3) | MUST | 6.7.3 - Keyed MD5 and Meticulous Keyed MD5 Authentication | **positive:** `unit/verify` [`TestRFC5880MeticulousSequenceIncrementsPerPacket`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L287). **negative:** `unit/verify` [`TestRFC5880MeticulousRejectsUnincrementedSequence`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L536) |
| `RFC5880-6.7.4-1` | Keyed SHA1 Auth Type must be set to 4 or 5, Auth Len must be 28 (§6.7.4) | MUST | 6.7.4 - Keyed SHA1 and Meticulous Keyed SHA1 Authentication | **positive:** `unit/verify` [`TestRFC5880KeyedSHA1SectionHeader`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L135). **negative:** `unit/verify` [`TestRFC5880KeyedSHA1RejectsForeignSectionShape`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L156) |
| `RFC5880-6.7.4-2` | SHA1 hash must be calculated over entire BFD Control packet (§6.7.4) | MUST | 6.7.4 - Keyed SHA1 and Meticulous Keyed SHA1 Authentication | **positive:** `unit/verify` [`TestRFC5880DigestCoversWholePacket`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L185). **negative:** `unit/verify` [`TestRFC5880DigestRejectsMandatorySectionTamper`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L203) |
| `RFC5880-6.7.4-3` | Keyed SHA1: secret key must not be carried in the packet (§6.7.4) | MUST NOT | 6.7.4 - Keyed SHA1 and Meticulous Keyed SHA1 Authentication | **positive:** `unit/verify` [`TestRFC5880SecretKeyNotCarriedInPacket`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L226). **negative:** no negative test. **{single-polarity}:** the SHA1 signer shares that producer with a 20-byte digest slot (internal/component/bfd/auth/sha1.go:85-87,193-195), so no key-bearing packet is emitted |
| `RFC5880-6.7.4-4` | Meticulous Keyed SHA1: bfd.XmitAuthSeq must be incremented for each packet (§6.7.4) | MUST | 6.7.4 - Keyed SHA1 and Meticulous Keyed SHA1 Authentication | **positive:** `unit/verify` [`TestRFC5880MeticulousSequenceIncrementsPerPacket`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L293). **negative:** `unit/verify` [`TestRFC5880MeticulousRejectsUnincrementedSequence`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L541) |
| `RFC5880-6.7.4-5` | SHA1 key management interface must accept ASCII strings (§6.7.4) | MUST | 6.7.4 - Keyed SHA1 and Meticulous Keyed SHA1 Authentication | **positive:** `unit/verify` [`TestRFC5880KeyManagementAcceptsASCIIStrings`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5880_test.go#L31). **negative:** `unit/verify` [`TestRFC5880KeyManagementRejectsIncompleteConfig`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5880_test.go#L70) |
| `RFC5880-6.8.6-1` | Reception: discard if Version != 1 (§6.8.6) | MUST | 6.8.6 - Reception of BFD Control Packets | **positive:** `unit/verify` [`TestRFC5880VersionOneAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/packet/rfc5880_test.go#L48). **negative:** `unit/verify` [`TestRFC5880VersionNotOneDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/packet/rfc5880_test.go#L66) |
| `RFC5880-6.8.6-2` | Reception: discard if Length < 24 (A=0) or < 26 (A=1) (§6.8.6) | MUST | 6.8.6 - Reception of BFD Control Packets | **positive:** `unit/verify` [`TestRFC5880LengthMinimumAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/packet/rfc5880_test.go#L108). **negative:** `unit/verify` [`TestRFC5880LengthBelowMinimumDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/packet/rfc5880_test.go#L128) |
| `RFC5880-6.8.6-3` | Reception: discard if Length > encapsulating payload (§6.8.6) | MUST | 6.8.6 - Reception of BFD Control Packets | **positive:** `unit/verify` [`TestRFC5880LengthEqualsPayloadAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/packet/rfc5880_test.go#L148). **negative:** `unit/verify` [`TestRFC5880LengthOverPayloadDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/packet/rfc5880_test.go#L163) |
| `RFC5880-6.8.6-4` | Reception: discard if Detect Mult == 0 (§6.8.6) | MUST | 6.8.6 - Reception of BFD Control Packets | **positive:** `unit/verify` [`TestRFC5880DetectMultNonZeroAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/packet/rfc5880_test.go#L174). **negative:** `unit/verify` [`TestRFC5880DetectMultZeroDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/packet/rfc5880_test.go#L190) |
| `RFC5880-6.8.6-5` | Reception: discard if M bit is nonzero (§6.8.6) | MUST | 6.8.6 - Reception of BFD Control Packets | **positive:** `unit/verify` [`TestRFC5880MultipointZeroAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/packet/rfc5880_test.go#L84). **negative:** `unit/verify` [`TestRFC5880MultipointSetDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/packet/rfc5880_test.go#L99) |
| `RFC5880-6.8.6-6` | Reception: discard if My Discriminator == 0 (§6.8.6) | MUST | 6.8.6 - Reception of BFD Control Packets | **positive:** `unit/verify` [`TestRFC5880MyDiscriminatorNonZeroAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/packet/rfc5880_test.go#L200). **negative:** `unit/verify` [`TestRFC5880MyDiscriminatorZeroDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/packet/rfc5880_test.go#L214) |
| `RFC5880-6.8.6-7` | Reception: if Your Discriminator nonzero, use it to select session; discard if no session found (§6.8.6) | MUST | 6.8.6 - Reception of BFD Control Packets | **positive:** `unit/verify` [`TestRFC5880YourDiscriminatorSelectsSession`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L124). **negative:** `unit/verify` [`TestRFC5880UnknownYourDiscriminatorDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L140) |
| `RFC5880-6.8.6-8` | Reception: if Your Discriminator zero and State is not Down or AdminDown, discard (§6.8.6) | MUST | 6.8.6 - Reception of BFD Control Packets | **positive:** `unit/verify` [`TestRFC5880ZeroYourDiscriminatorAcceptedWhenDown`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L724). **negative:** `unit/verify` [`TestRFC5880ZeroYourDiscriminatorDiscardedWhenLive`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L699) |
| `RFC5880-6.8.6-9` | Reception: if A=1 and bfd.AuthType is zero, discard (§6.8.6) | MUST | 6.8.6 - Reception of BFD Control Packets | **positive:** `unit/verify` [`TestRFC5880UnauthenticatedSessionAcceptsUnauthenticatedPacket`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L746). **negative:** `unit/verify` [`TestRFC5880UnauthenticatedSessionDiscardsAuthenticatedPacket`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L761) |
| `RFC5880-6.8.6-10` | Reception: if A=0 and bfd.AuthType is nonzero, discard (§6.8.6) | MUST | 6.8.6 - Reception of BFD Control Packets | **positive:** `unit/verify` [`TestRFC5880AuthenticatedSessionAcceptsAuthenticatedPacket`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L778). **negative:** `unit/verify` [`TestRFC5880AuthenticatedSessionDiscardsUnauthenticatedPacket`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L796) |
| `RFC5880-6.8.6-11` | Reception: if A=1, authenticate per §6.7 (§6.8.6) | MUST | 6.8.6 - Reception of BFD Control Packets | **positive:** `unit/verify` [`TestRFC5880AuthenticatedPacketVerifiedAndDelivered`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L241). **negative:** `unit/verify` [`TestRFC5880UnauthenticPacketDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L258) |
| `RFC5880-6.8.6-12` | Reception: if Required Min Echo RX Interval == 0, cease Echo transmission (§6.8.6) | MUST | 6.8.6 - Reception of BFD Control Packets | **positive:** `unit/verify` [`TestRFC5880EchoCeasesWhenPeerAdvertisesZero`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L812). **negative:** `unit/verify` [`TestRFC5880EchoEnabledWhenPeerAdvertisesNonZero`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L849) |
| `RFC5880-6.8.6-13` | Reception: if Poll in flight and F=1 received, terminate the Poll Sequence (§6.8.6) | MUST | 6.8.6 - Reception of BFD Control Packets | **positive:** `unit/verify` [`TestRFC5880FinalTerminatesPoll`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L610). **negative:** `unit/verify` [`TestRFC5880NonFinalDoesNotTerminatePoll`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L630) |
| `RFC5880-6.8.6-14` | Reception: if remote Demand mode active (D=1, both Up), cease periodic Control packet transmission (§6.8.6) | MUST | 6.8.6 - Reception of BFD Control Packets | **positive:** no positive test. **negative:** no negative test. **{gap}:** tick transmits whenever the periodic deadline has passed and never consults the remote Demand state (internal/component/bfd/engine/loop.go:192-201), so periodic Control packets continue after the peer sets D=1 |
| `RFC5880-6.8.6-15` | Reception: if remote Demand mode not active, send periodic Control packets (§6.8.6) | MUST | 6.8.6 - Reception of BFD Control Packets | **positive:** `unit/verify` [`TestRFC5880PeriodicTransmitWhenRemoteDemandInactive`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L368). **negative:** `unit/verify` [`TestRFC5880NoPeriodicTransmitWhileAdminDown`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L403) |
| `RFC5880-6.8.6-16` | Reception: if P=1, send Final packet immediately (§6.8.6, §6.8.7) | MUST | 6.8.6 - Reception of BFD Control Packets | **positive:** `unit/verify` [`TestRFC5880PollAnsweredWithImmediateFinal`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L322). **negative:** `unit/verify` [`TestRFC5880NonPollProducesNoImmediateReply`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L352) |
| `RFC5880-6.8.6-18` | Reception: if Your Discriminator zero, select the session on a combination of other fields, which can include source addressing information, My Discriminator, and the ingress interface (§6.8.6) | MUST | 6.8.6 - Reception of BFD Control Packets | **positive:** `unit/verify` [`TestRFC5881FirstPacketMatchesByTuple`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5881_test.go#L105). **negative:** `unit/verify` [`TestRFC5881FirstPacketWrongSourceDropped`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5881_test.go#L128) |
| `RFC5880-6.8.7-1` | Transmission: must not transmit at interval less than max(bfd.DesiredMinTxInterval, bfd.RemoteMinRxInterval) less jitter (§6.8.7) | MUST NOT | 6.8.7 - Transmitting BFD Control Packets | **positive:** `unit/verify` [`TestRFC5880TransmitDeadlineUsesNegotiatedInterval`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L1015). **negative:** `unit/verify` [`TestRFC5880TransmitDeadlineClampsBadJitter`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L1045) |
| `RFC5880-6.8.7-2` | Transmission: periodic TX must be jittered by 0-25% per packet (§6.8.7) | MUST | 6.8.7 - Transmitting BFD Control Packets | **positive:** `unit/verify` [`TestRFC5880JitterIsAppliedPerPacket`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L422). **negative:** `unit/verify` [`TestRFC5880JitterStaysWithinBand`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L450) |
| `RFC5880-6.8.7-3` | Transmission: if bfd.DetectMult == 1, interval must be 75-90% of negotiated interval (§6.8.7) | MUST | 6.8.7 - Transmitting BFD Control Packets | **positive:** `unit/verify` [`TestRFC5880JitterDetectMultOneWindow`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L470). **negative:** `unit/verify` [`TestRFC5880JitterFloorOnlyForDetectMultOne`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L488) |
| `RFC5880-6.8.7-4` | Transmission: TX interval must be recalculated whenever bfd.DesiredMinTxInterval or bfd.RemoteMinRxInterval changes (§6.8.7) | MUST | 6.8.7 - Transmitting BFD Control Packets | **positive:** `unit/verify` [`TestRFC5880RemoteMinRxReductionHonoredImmediately`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L504). **negative:** `unit/verify` [`TestRFC5880TransmitIntervalFlooredByLocalDesired`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L538) |
| `RFC5880-6.8.7-5` | Transmission: must not transmit if bfd.RemoteDiscr is zero and system is Passive (§6.8.7) | MUST NOT | 6.8.7 - Transmitting BFD Control Packets | **positive:** `unit/verify` [`TestRFC5880PassiveRoleSilentUntilReception`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L391). **negative:** `unit/verify` [`TestRFC5880ActiveRoleTransmitsWithoutReception`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L369) |
| `RFC5880-6.8.7-6` | Transmission: must not periodically transmit if bfd.RemoteMinRxInterval is zero (§6.8.7) | MUST NOT | 6.8.7 - Transmitting BFD Control Packets | **positive:** no positive test. **negative:** no negative test. **{gap}:** TransmitInterval substitutes the slow-start interval when the negotiated maximum is zero (internal/component/bfd/session/timers.go:45-47) and tick carries no bfd.RemoteMinRxInterval == 0 suppression (internal/component/bfd/engine/loop.go:192-201), so periodic transmission continues |
| `RFC5880-6.8.7-7` | Transmission: must not periodically transmit if remote Demand mode active and no Poll in flight (§6.8.7) | MUST NOT | 6.8.7 - Transmitting BFD Control Packets | **positive:** no positive test. **negative:** no negative test. **{gap}:** the same tick producer (internal/component/bfd/engine/loop.go:192-201) has no remote-Demand suppression, so periodic transmission continues while the remote Demand mode is active with no Poll in flight |
| `RFC5880-6.8.7-8` | If rate limiting Final packets, advertised Desired Min TX Interval must be >= rate-limit interval (§6.8.7) | MUST | 6.8.7 - Transmitting BFD Control Packets | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze does not rate-limit Final packets -- handleInbound sends the Final unconditionally on every received Poll (internal/component/bfd/engine/loop.go:140-142) and no rate limiter exists on that path, so the conditional advertised-interval floor never applies |
| `RFC5880-6.8.4-1` | Detection time expired while Init or Up: set bfd.SessionState to Down, bfd.LocalDiag to 1 (§6.8.4) | MUST | 6.8.4 - Calculating the Detection Time | **positive:** `unit/verify` [`TestRFC5880DetectionExpiryDownDiagOne`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L312). **negative:** `unit/verify` [`TestRFC5880DetectionExpiryIgnoredWhenDown`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L333) |
| `RFC5880-6.8.5-1` | Echo function failure: set bfd.SessionState to Down, bfd.LocalDiag to 2 (§6.8.5) | MUST | 6.8.5 - Detecting Failures with the Echo Function | **positive:** `unit/verify` [`TestRFC5880EchoMissDetectedAndReported`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L943). **negative:** `unit/verify` [`TestRFC5880EchoReturnClearsMissAndFailIsScoped`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L979) |
| `RFC5880-6.8.15-1` | Forwarding plane reset: set bfd.LocalDiag to 4, bfd.SessionState to Down (§6.8.15) | MUST | 6.8.15 - Forwarding Plane Reset | **positive:** no positive test. **negative:** no negative test. **{gap}:** packet.DiagForwardingPlaneReset (internal/component/bfd/packet/diag.go:21) has no producer, and the only external state-forcing entry points are AdminDown and AdminEnable (internal/component/bfd/session/fsm.go:180,192), which move the session to AdminDown or Down carrying the caller's diagnostic and are never called with code 4 |
| `RFC5880-6.8.16-1` | Administrative control: follow the enable/disable procedure (§6.8.16) | MUST | 6.8.16 - Administrative Control | **positive:** `unit/verify` [`TestRFC5880AdministrativeDisableEnable`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L1071). **negative:** `unit/verify` [`TestRFC5880AdministrativeCallsAreGuarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L1106) |
| `RFC5880-6.8.8-1` | Echo packets must be demultiplexed to the appropriate session (§6.8.8) | MUST | 6.8.8 - Reception of BFD Echo Packets | **positive:** `unit/verify` [`TestRFC5880EchoDemultiplexedToItsSession`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L589). **negative:** `unit/verify` [`TestRFC5880UnknownEchoDropped`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L626) |
| `RFC5880-6.8.8-2` | A means of detecting missing Echo packets must be implemented (§6.8.8) | MUST | 6.8.8 - Reception of BFD Echo Packets | **positive:** `unit/verify` [`TestRFC5880EchoMissDetectedAndReported`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L938). **negative:** `unit/verify` [`TestRFC5880EchoReturnClearsMissAndFailIsScoped`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L974) |
| `RFC5880-6.8.9-1` | Echo packets must not be transmitted when bfd.SessionState is not Up (§6.8.9) | MUST NOT | 6.8.9 - Transmission of BFD Echo Packets | **positive:** `unit/verify` [`TestRFC5880EchoTransmittedWhileUp`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L543). **negative:** `unit/verify` [`TestRFC5880NoEchoTransmittedWhenNotUp`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L564) |
| `RFC5880-6.8.9-2` | Echo packets must not be transmitted unless remote Required Min Echo RX Interval is nonzero (§6.8.9) | MUST NOT | 6.8.9 - Transmission of BFD Echo Packets | **positive:** `unit/verify` [`TestRFC5880EchoEnabledWhenPeerAdvertisesNonZero`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L852). **negative:** `unit/verify` [`TestRFC5880EchoNotTransmittedWithoutPeerAdvertisement`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L873) |
| `RFC5880-6.8.9-3` | Echo packet TX interval must not be less than remote Required Min Echo RX Interval (§6.8.9) | MUST NOT | 6.8.9 - Transmission of BFD Echo Packets | **positive:** `unit/verify` [`TestRFC5880EchoIntervalHonorsPeerFloor`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L896). **negative:** `unit/verify` [`TestRFC5880EchoIntervalNotBelowLocalTarget`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L915) |
| `RFC5880-7-1` | Multihop: a congestion control mechanism must be implemented (§7) | MUST | 7 - Operational Considerations | **positive:** no positive test. **negative:** no negative test. **{gap}:** the transmit rate is derived solely from TransmitInterval plus jitter (internal/component/bfd/engine/loop.go:197-200) with no congestion-feedback input, and no congestion-control producer exists anywhere under internal/component/bfd |
| `RFC5880-7-2` | Multihop: when congestion detected, BFD must reduce traffic generated (§7) | MUST | 7 - Operational Considerations | **positive:** no positive test. **negative:** no negative test. **{gap}:** the same rate producer (internal/component/bfd/engine/loop.go:197-200) is the only authority over generated traffic and nothing reduces it in response to detected congestion |
| `RFC5880-4.3-1` | Reserved byte in MD5/SHA1 auth section must be set to zero on transmit (§4.3, §4.4) | MUST | 4.3 | **positive:** `unit/verify` [`TestRFC5880KeyedMD5SectionHeader`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L88). **negative:** no negative test. **{single-polarity}:** Sign hardcodes the Reserved byte to 0 (internal/component/bfd/auth/sha1.go:83) and no code path emits a non-zero value, so there is no non-conformant transmission to reject |
| `RFC5880-6.8.1-15` | bfd.LocalDiscr should be set to a random value to improve security (§6.8.1) | SHOULD | 6.8.1 - State Variables | **positive:** no positive test. **negative:** no negative test |
| `RFC5880-6.4-1` | Advertise the lowest values of Required Min RX Interval and Required Min Echo RX Interval possible (§6.4) | SHOULD | 6.4 - The Echo Function and Asymmetry | **positive:** no positive test. **negative:** no negative test |
| `RFC5880-5-1` | Echo packets should include some form of authentication (§5) | SHOULD | 5 - BFD Echo Packet Format | **positive:** no positive test. **negative:** no negative test |
| `RFC5880-6.8.3-7` | When Echo function is active, set bfd.RequiredMinRxInterval to at least 1,000,000 microseconds (§6.8.3) | SHOULD | 6.8.3 - Timer Manipulation | **positive:** no positive test. **negative:** no negative test |
| `RFC5880-6.8.7-9` | A non-periodic Control packet should be sent when content would change (§6.8.7) | SHOULD | 6.8.7 - Transmitting BFD Control Packets | **positive:** no positive test. **negative:** no negative test |
| `RFC5880-6.7.1-1` | Authentication state change should only be allowed at most once without intervention (§6.7.1) | SHOULD | 6.7.1 - Enabling and Disabling Authentication | **positive:** no positive test. **negative:** no negative test |
| `RFC5880-6.7.1-2` | Authentication state should not change based on receipt of BFD Control packets (§6.7.1) | SHOULD NOT | 6.7.1 - Enabling and Disabling Authentication | **positive:** no positive test. **negative:** no negative test |
| `RFC5880-6.7.2-2` | Simple Password and key management should also allow hexadecimal binary string configuration (§6.7.2, §6.7.3, §6.7.4) | SHOULD | 6.7.2 - Simple Password Authentication | **positive:** no positive test. **negative:** no negative test |
| `RFC5880-6.7.3-5` | Keyed MD5/SHA1: bfd.XmitAuthSeq should be incremented on state change or content change (§6.7.3, §6.7.4) | SHOULD | 6.7.3 - Keyed MD5 and Meticulous Keyed MD5 Authentication | **positive:** no positive test. **negative:** no negative test |
| `RFC5880-6.8.16-2` | BFD Control packets should be transmitted for at least one Detection Time after transitioning to AdminDown (§6.8.16) | SHOULD | 6.8.16 - Administrative Control | **positive:** no positive test. **negative:** no negative test |
| `RFC5880-7-3` | Multihop: congestion control algorithm should be used even across single hops (§7) | SHOULD | 7 - Operational Considerations | **positive:** no positive test. **negative:** no negative test |
| `RFC5880-9-1` | Multihop: if run across multiple hops or insecure tunnel, Authentication Section should be utilized (§9) | SHOULD | 9 - Security Considerations | **positive:** no positive test. **negative:** no negative test |
| `RFC5880-6.8.18-1` | If no BFD Control packets received for a Detection Time, remote system should reset bfd.RemoteMinRxInterval to 1 (§6.8.18) | SHOULD | 6.8.18 - Holding Down Sessions | **positive:** no positive test. **negative:** no negative test |
| `RFC5880-6.2-1` | A session may be kept administratively down (§6.2) | MAY | 6.2 - BFD State Machine | **positive:** no positive test. **negative:** no negative test |
| `RFC5880-6.3-2` | A system may change its discriminator during a session (§6.3) | MAY | 6.3 - Demultiplexing and the Discriminator Fields | **positive:** no positive test. **negative:** no negative test |
| `RFC5880-6.8.6-17` | When Your Discriminator is zero, a new session may be created or the packet may be discarded (§6.8.6) | MAY | 6.8.6 - Reception of BFD Control Packets | **positive:** no positive test. **negative:** no negative test |
| `RFC5880-6.8.7-10` | Final packets may be rate-limited (§6.8.7) | MAY | 6.8.7 - Transmitting BFD Control Packets | **positive:** no positive test. **negative:** no negative test |
| `RFC5880-6.6-4` | Demand mode may be enabled or disabled at any time (§6.6) | MAY | 6.6 - Demand mode | **positive:** no positive test. **negative:** no negative test |
| `RFC5880-6.8.13-1` | Echo function may be enabled or disabled at any time (§6.8.13) | MAY | 6.8.13 - Enabling or Disabling The Echo Function | **positive:** no positive test. **negative:** no negative test |
| `RFC5880-6.7.3-6` | Keyed MD5: bfd.XmitAuthSeq may be incremented in a circular fashion (§6.7.3) | MAY | 6.7.3 - Keyed MD5 and Meticulous Keyed MD5 Authentication | **positive:** no positive test. **negative:** no negative test |
| `RFC5880-6.7.4-6` | Keyed SHA1: bfd.XmitAuthSeq may be incremented in a circular fashion (§6.7.4) | MAY | 6.7.4 - Keyed SHA1 and Meticulous Keyed SHA1 Authentication | **positive:** no positive test. **negative:** no negative test |
| `RFC5880-6.8.1-16` | Session state may be preserved longer than one Detection Time (§6.8.1) | MAY | 6.8.1 - State Variables | **positive:** no positive test. **negative:** no negative test |
| `RFC5880-6.8.16-3` | BFD Control packets may be transmitted indefinitely after transitioning to AdminDown (§6.8.16) | MAY | 6.8.16 - Administrative Control | **positive:** no positive test. **negative:** no negative test |
| `RFC5880-9-2` | For single-hop sessions, TTL or Hop Count MUST be set to maximum on transmit and checked equal to maximum on receipt (§9) | MUST | 9 - Security Considerations | **positive:** `unit/verify` [`TestRFC5880SingleHopTransmitTTLIsMaximum`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/transport/rfc5880_test.go#L43). **negative:** `unit/verify` [`TestRFC5880SingleHopReceiveTTLNotMaxDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L160) |
| `RFC5880-6.8.14-1` | If Demand mode is no longer active on the remote system, the local system MUST begin transmitting periodic BFD Control packets (§6.8.14) | MUST | 6.8.14 - Enabling or Disabling Demand Mode | **positive:** `unit/verify` [`TestRFC5880PeriodicTransmitWhenRemoteDemandInactive`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L372). **negative:** no negative test. **{single-polarity}:** tick transmits periodic Control packets without consulting the remote D bit (internal/component/bfd/engine/loop.go:186-201), so a remote that stops asserting Demand keeps receiving them and there is no suppressed state to leave |
| `RFC5880-6.8.17-1` | If concatenated path failure diagnostic must be communicated and remote Demand mode is active, a Poll Sequence MUST be initiated (§6.8.17) | MUST | 6.8.17 - Concatenated Paths | **positive:** no positive test. **negative:** no negative test. **{gap}:** packet.DiagConcatPathDown (internal/component/bfd/packet/diag.go:23) has no producer and the remote Demand state stored at internal/component/bfd/session/fsm.go:58 is never read, so a concatenated-path failure neither sets the diagnostic nor initiates a Poll Sequence |
| `RFC5880-6.8.3-8` | Timing parameter changes not explicitly excepted MUST be effected immediately (§6.8.3) | MUST | 6.8.3 - Timer Manipulation | **positive:** `unit/verify` [`TestRFC5880RemoteMinRxReductionHonoredImmediately`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L506). **negative:** no negative test. **{single-polarity}:** TransmitInterval and DetectionInterval are recomputed from the live bfd.* variables on every call (internal/component/bfd/session/timers.go:24-49), so an unexcepted timing change is in force at the next evaluation and no held-back value exists to reject |
| `RFC5880-6.7.3-7` | MD5 key management interface MUST accept ASCII strings (§6.7.3) | MUST | 6.7.3 - Keyed MD5 and Meticulous Keyed MD5 Authentication | **positive:** `unit/verify` [`TestRFC5880KeyManagementAcceptsASCIIStrings`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5880_test.go#L26). **negative:** `unit/verify` [`TestRFC5880KeyManagementRejectsIncompleteConfig`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5880_test.go#L64) |
| `RFC5880-6.7.2-3` | Simple Password, MD5, and SHA1 Auth Key ID field MUST be set to the ID of the current authentication key (§6.7.2, §6.7.3, §6.7.4) | MUST | 6.7.2 - Simple Password Authentication | **positive:** `unit/verify` [`TestRFC5880AuthKeyIDIsTheConfiguredKey`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L246). **positive:** `unit/verify` [`TestRFC5880SimplePasswordSectionCarriesPasswordAndKeyID`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L639). **negative:** `unit/verify` [`TestRFC5880AuthKeyIDMismatchDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L264) |
| `RFC5880-6.7.3-8` | Simple Password, MD5, and SHA1 Sequence Number field MUST be set to bfd.XmitAuthSeq (§6.7.3, §6.7.4) | MUST | 6.7.3 - Keyed MD5 and Meticulous Keyed MD5 Authentication | **positive:** `unit/verify` [`TestRFC5880AuthSequenceFieldIsXmitAuthSeq`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L1140). **negative:** `unit/verify` [`TestRFC5880AuthSequenceFieldFollowsAdvance`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L1163) |
| `RFC5880-6.7.2-4` | Reception: if Auth Type does not match bfd.AuthType, packet MUST be discarded (§6.7.2, §6.7.3, §6.7.4) | MUST | 6.7.2 - Simple Password Authentication | **positive:** `unit/verify` [`TestRFC5880AuthTypeMatchAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L297). **negative:** `unit/verify` [`TestRFC5880AuthTypeMismatchDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L310) |
| `RFC5880-6.7.2-5` | Reception: if Auth Key ID does not match any configured authentication key, packet MUST be discarded (§6.7.2, §6.7.3, §6.7.4) | MUST | 6.7.2 - Simple Password Authentication | **positive:** `unit/verify` [`TestRFC5880AuthKeyIDMatchAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L285). **positive:** `unit/verify` [`TestRFC5880SimplePasswordMatchingKeyIDAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L729). **negative:** `unit/verify` [`TestRFC5880AuthKeyIDMismatchDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L267). **negative:** `unit/verify` [`TestRFC5880SimplePasswordWrongKeyIDDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L712) |
| `RFC5880-6.7.2-6` | Reception: if Auth Len does not match expected length, packet MUST be discarded (§6.7.2, §6.7.3, §6.7.4) | MUST | 6.7.2 - Simple Password Authentication | **positive:** `unit/verify` [`TestRFC5880AuthLenExpectedAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L326). **negative:** `unit/verify` [`TestRFC5880AuthLenMismatchDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L343) |
| `RFC5880-6.7.2-7` | Reception: if password does not match configured password for Simple Password, packet MUST be discarded (§6.7.2) | MUST | 6.7.2 - Simple Password Authentication | **positive:** `unit/verify` [`TestRFC5880SimplePasswordMatchAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L662). **negative:** `unit/verify` [`TestRFC5880SimplePasswordMismatchDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L681) |
| `RFC5880-6.7.3-9` | Reception: for Keyed MD5/SHA1, if Sequence Number is less than bfd.RcvAuthSeq (accounting for wrap), packet MUST be discarded (§6.7.3, §6.7.4) | MUST | 6.7.3 - Keyed MD5 and Meticulous Keyed MD5 Authentication | **positive:** `unit/verify` [`TestRFC5880KeyedSequenceAtOrAboveFloorAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L494). **negative:** `unit/verify` [`TestRFC5880KeyedSequenceBelowFloorDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L514) |
| `RFC5880-6.7.3-10` | Reception: for Meticulous Keyed MD5/SHA1, if Sequence Number is not exactly bfd.RcvAuthSeq+1 (accounting for wrap), packet MUST be discarded (§6.7.3, §6.7.4) | MUST | 6.7.3 - Keyed MD5 and Meticulous Keyed MD5 Authentication | **positive:** no positive test. **negative:** no negative test. **{gap}:** SeqState.Check accepts any sequence strictly greater than bfd.RcvAuthSeq for the meticulous variants (internal/component/bfd/auth/meticulous.go:47-51), so a packet whose sequence jumps past RcvAuthSeq+1 is accepted rather than discarded |
| `RFC5880-6.7.3-11` | Reception: if digest/hash does not match computed value, packet MUST be discarded (§6.7.3, §6.7.4) | MUST | 6.7.3 - Keyed MD5 and Meticulous Keyed MD5 Authentication | **positive:** `unit/verify` [`TestRFC5880DigestMatchAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L377). **negative:** `unit/verify` [`TestRFC5880DigestMismatchDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L390) |
| `RFC5880-6.7.3-12` | Reception: if bfd.AuthSeqKnown is 0, it MUST be set to 1 and bfd.RcvAuthSeq MUST be set to received Sequence Number (§6.7.3, §6.7.4) | MUST | 6.7.3 - Keyed MD5 and Meticulous Keyed MD5 Authentication | **positive:** `unit/verify` [`TestRFC5880FirstAuthenticatedPacketSeedsReplayFloor`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L447). **negative:** `unit/verify` [`TestRFC5880ForgedFirstPacketDoesNotSeedReplayFloor`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L471) |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC5880-6.8.1-11`](#rfc5880-6.8.1-11) bfd.XmitAuthSeq must be initialized to a random 32-bit value (§6.8.1) | {gap}, no test | SetAuth seeds bfd.XmitAuthSeq from the persister or leaves the Vars zero value (internal/component/bfd/session/auth.go:36) and Init never randomizes it (internal/component/bfd/session/session.go:245-259), so the initial transmit sequence is 0 rather than a random 32-bit value |
| [`RFC5880-6.8.1-13`](#rfc5880-6.8.1-13) bfd.AuthSeqKnown must be set to zero after no packets received for twice the Detection Time (§6.8.1) | {gap}, no test | bfd.AuthSeqKnown is SeqState.initialized, which only Advance sets (internal/component/bfd/auth/meticulous.go:63-66) and nothing ever clears; CheckDetection clears the detection deadline alone (internal/component/bfd/session/timers.go:82), so the flag survives twice the Detection Time of silence |
| [`RFC5880-6.8.3-3`](#rfc5880-6.8.3-3) If bfd.DesiredMinTxInterval is increased while Up, the actual TX interval must not change until the Poll Sequence terminates (§6.8.3) | {gap}, no test | ApplyEchoSlowdown raises bfd.DesiredMinTxInterval while the session is Up (internal/component/bfd/session/timers.go:235) and TransmitInterval returns the raised value on the very next call (internal/component/bfd/session/timers.go:44), so the longer interval takes effect before the Poll Sequence terminates |
| [`RFC5880-6.8.3-4`](#rfc5880-6.8.3-4) If bfd.RequiredMinRxInterval is reduced while Up, the previous value must be used for detection time until the Poll terminates (§6.8.3) | {gap}, no test | revertEchoSlowdownLocked reduces bfd.RequiredMinRxInterval while Up (internal/component/bfd/session/timers.go:249) and DetectionInterval immediately uses the reduced value (internal/component/bfd/session/timers.go:29); no previous value is retained until the Poll Sequence terminates |
| [`RFC5880-6.6-2`](#rfc5880-6.6-2) When the D bit value is to be changed, a Poll Sequence must be initiated (§6.6) | {gap}, no test | bfd.DemandMode has no writer in production code -- it is declared at internal/component/bfd/session/session.go:58 and read only by canSetDemand (internal/component/bfd/session/fsm.go:247) -- so no D-bit change path exists and none initiates a Poll Sequence |
| [`RFC5880-6.6-3`](#rfc5880-6.6-3) If Demand mode is active on either system, a Poll Sequence must be initiated whenever next packet contents would differ (except P/F bits) (§6.6) | {gap}, no test | bfd.RemoteDemandMode is stored by Receive (internal/component/bfd/session/fsm.go:58) and read nowhere, and bfd.DemandMode has no writer (internal/component/bfd/session/session.go:58), so no code path starts a Poll when packet contents would change while Demand mode is active |
| [`RFC5880-6.8.6-14`](#rfc5880-6.8.6-14) Reception: if remote Demand mode active (D=1, both Up), cease periodic Control packet transmission (§6.8.6) | {gap}, no test | tick transmits whenever the periodic deadline has passed and never consults the remote Demand state (internal/component/bfd/engine/loop.go:192-201), so periodic Control packets continue after the peer sets D=1 |
| [`RFC5880-6.8.7-6`](#rfc5880-6.8.7-6) Transmission: must not periodically transmit if bfd.RemoteMinRxInterval is zero (§6.8.7) | {gap}, no test | TransmitInterval substitutes the slow-start interval when the negotiated maximum is zero (internal/component/bfd/session/timers.go:45-47) and tick carries no bfd.RemoteMinRxInterval == 0 suppression (internal/component/bfd/engine/loop.go:192-201), so periodic transmission continues |
| [`RFC5880-6.8.7-7`](#rfc5880-6.8.7-7) Transmission: must not periodically transmit if remote Demand mode active and no Poll in flight (§6.8.7) | {gap}, no test | the same tick producer (internal/component/bfd/engine/loop.go:192-201) has no remote-Demand suppression, so periodic transmission continues while the remote Demand mode is active with no Poll in flight |
| [`RFC5880-6.8.7-8`](#rfc5880-6.8.7-8) If rate limiting Final packets, advertised Desired Min TX Interval must be >= rate-limit interval (§6.8.7) | no test | no test carries this requirement id; annotated {not-applicable}: ze does not rate-limit Final packets -- handleInbound sends the Final unconditionally on every received Poll (internal/component/bfd/engine/loop.go:140-142) and no rate limiter exists on that path, so the conditional advertised-interval floor never applies |
| [`RFC5880-6.8.15-1`](#rfc5880-6.8.15-1) Forwarding plane reset: set bfd.LocalDiag to 4, bfd.SessionState to Down (§6.8.15) | {gap}, no test | packet.DiagForwardingPlaneReset (internal/component/bfd/packet/diag.go:21) has no producer, and the only external state-forcing entry points are AdminDown and AdminEnable (internal/component/bfd/session/fsm.go:180,192), which move the session to AdminDown or Down carrying the caller's diagnostic and are never called with code 4 |
| [`RFC5880-7-1`](#rfc5880-7-1) Multihop: a congestion control mechanism must be implemented (§7) | {gap}, no test | the transmit rate is derived solely from TransmitInterval plus jitter (internal/component/bfd/engine/loop.go:197-200) with no congestion-feedback input, and no congestion-control producer exists anywhere under internal/component/bfd |
| [`RFC5880-7-2`](#rfc5880-7-2) Multihop: when congestion detected, BFD must reduce traffic generated (§7) | {gap}, no test | the same rate producer (internal/component/bfd/engine/loop.go:197-200) is the only authority over generated traffic and nothing reduces it in response to detected congestion |
| [`RFC5880-6.8.17-1`](#rfc5880-6.8.17-1) If concatenated path failure diagnostic must be communicated and remote Demand mode is active, a Poll Sequence MUST be initiated (§6.8.17) | {gap}, no test | packet.DiagConcatPathDown (internal/component/bfd/packet/diag.go:23) has no producer and the remote Demand state stored at internal/component/bfd/session/fsm.go:58 is never read, so a concatenated-path failure neither sets the diagnostic nor initiates a Poll Sequence |
| [`RFC5880-6.7.3-10`](#rfc5880-6.7.3-10) Reception: for Meticulous Keyed MD5/SHA1, if Sequence Number is not exactly bfd.RcvAuthSeq+1 (accounting for wrap), packet MUST be discarded (§6.7.3, §6.7.4) | {gap}, no test | SeqState.Check accepts any sequence strictly greater than bfd.RcvAuthSeq for the meticulous variants (internal/component/bfd/auth/meticulous.go:47-51), so a packet whose sequence jumps past RcvAuthSeq+1 is accepted rather than discarded |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC5880-4.1-1`](#rfc5880-4.1-1)

Version field must be 1 (§4.1, §6.8.6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880VersionNotOneDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/packet/rfc5880_test.go#L64) | unit/verify | unproven |
| positive | [`TestRFC5880VersionOneAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/packet/rfc5880_test.go#L44) | unit/verify | unproven |

### [`RFC5880-4.1-2`](#rfc5880-4.1-2)

Multipoint (M) bit must be zero on both transmit and receipt (§4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880MultipointSetDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/packet/rfc5880_test.go#L96) | unit/verify | unproven |
| positive | [`TestRFC5880MultipointZeroAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/packet/rfc5880_test.go#L80) | unit/verify | unproven |

### [`RFC5880-6.3-1`](#rfc5880-6.3-1)

My Discriminator must be nonzero and unique across all BFD sessions on the system (§6.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880DiscriminatorAllocatorSkipsReservedAndTaken`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L90) | unit/verify | unproven |
| positive | [`TestRFC5880DiscriminatorsAreNonZeroAndUnique`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L62) | unit/verify | unproven |

### [`RFC5880-6.8.1-1`](#rfc5880-6.8.1-1)

bfd.SessionState must be initialized to Down (§6.8.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC5880InitStatesAreDown`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L73) | unit/verify | unproven |

### [`RFC5880-6.8.1-2`](#rfc5880-6.8.1-2)

bfd.RemoteSessionState must be initialized to Down (§6.8.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC5880InitStatesAreDown`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L76) | unit/verify | unproven |

### [`RFC5880-6.8.1-3`](#rfc5880-6.8.1-3)

bfd.LocalDiscr must be unique across all BFD sessions on the system, and nonzero (§6.8.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880DiscriminatorAllocatorSkipsReservedAndTaken`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L95) | unit/verify | unproven |
| positive | [`TestRFC5880DiscriminatorsAreNonZeroAndUnique`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L67) | unit/verify | unproven |

### [`RFC5880-6.8.1-4`](#rfc5880-6.8.1-4)

bfd.RemoteDiscr must be initialized to zero (§6.8.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880InitVariablesAreNotConstants`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L113) | unit/verify | unproven |
| positive | [`TestRFC5880InitVariableDefaults`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L88) | unit/verify | unproven |

### [`RFC5880-6.8.1-5`](#rfc5880-6.8.1-5)

bfd.RemoteDiscr must be set to zero if no valid packet received for one Detection Time (§6.8.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880RemoteDiscrKeptOnNeighborSignaledDown`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L180) | unit/verify | unproven |
| positive | [`TestRFC5880RemoteDiscrClearedOnDetectionExpiry`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L153) | unit/verify | unproven |

### [`RFC5880-6.8.1-6`](#rfc5880-6.8.1-6)

bfd.LocalDiag must be initialized to zero (§6.8.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880InitVariablesAreNotConstants`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L116) | unit/verify | unproven |
| positive | [`TestRFC5880InitVariableDefaults`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L90) | unit/verify | unproven |

### [`RFC5880-6.8.1-7`](#rfc5880-6.8.1-7)

bfd.DesiredMinTxInterval must be initialized to at least 1,000,000 microseconds (§6.8.1, §6.8.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880SlowStartFloorLiftedWhenUp`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L228) | unit/verify | unproven |
| positive | [`TestRFC5880SlowStartFloorWhileNotUp`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L208) | unit/verify | unproven |

### [`RFC5880-6.8.1-8`](#rfc5880-6.8.1-8)

bfd.RemoteMinRxInterval must be initialized to 1 (§6.8.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880InitVariablesAreNotConstants`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L118) | unit/verify | unproven |
| positive | [`TestRFC5880InitVariableDefaults`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L92) | unit/verify | unproven |

### [`RFC5880-6.8.1-9`](#rfc5880-6.8.1-9)

bfd.RemoteDemandMode must be initialized to zero (§6.8.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880InitVariablesAreNotConstants`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L121) | unit/verify | unproven |
| positive | [`TestRFC5880InitVariableDefaults`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L94) | unit/verify | unproven |

### [`RFC5880-6.8.1-10`](#rfc5880-6.8.1-10)

bfd.DetectMult must be a nonzero integer (§6.8.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880DetectMultZeroRequestSubstituted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L259) | unit/verify | unproven |
| positive | [`TestRFC5880DetectMultConfiguredValue`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L248) | unit/verify | unproven |

### [`RFC5880-6.8.1-11`](#rfc5880-6.8.1-11)

bfd.XmitAuthSeq must be initialized to a random 32-bit value (§6.8.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5880-6.8.1-11, so no unit is bound to it.

### [`RFC5880-6.8.1-12`](#rfc5880-6.8.1-12)

bfd.AuthSeqKnown must be initialized to zero (§6.8.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880AuthSeqKnownSetAfterFirstPacket`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L436) | unit/verify | unproven |
| positive | [`TestRFC5880AuthSeqKnownStartsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L418) | unit/verify | unproven |

### [`RFC5880-6.8.1-13`](#rfc5880-6.8.1-13)

bfd.AuthSeqKnown must be set to zero after no packets received for twice the Detection Time (§6.8.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5880-6.8.1-13, so no unit is bound to it.

### [`RFC5880-6.8.1-14`](#rfc5880-6.8.1-14)

Session state must be preserved for at least one Detection Time after last valid packet (§6.8.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880DetectionExpiryDownDiagOne`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L308) | unit/verify | unproven |
| positive | [`TestRFC5880StatePreservedForOneDetectionTime`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L283) | unit/verify | unproven |

### [`RFC5880-6.1-1`](#rfc5880-6.1-1)

Active role system must send BFD Control packets regardless of whether packets have been received (§6.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880PassiveRoleTransmitsAfterReception`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L410) | unit/verify | unproven |
| positive | [`TestRFC5880ActiveRoleTransmitsWithoutReception`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L361) | unit/verify | unproven |

### [`RFC5880-6.1-2`](#rfc5880-6.1-2)

Passive role system must not begin sending BFD packets until it has received one (§6.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880PassiveRoleTransmitsAfterReception`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L415) | unit/verify | unproven |
| positive | [`TestRFC5880PassiveRoleSilentUntilReception`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L386) | unit/verify | unproven |

### [`RFC5880-6.1-3`](#rfc5880-6.1-3)

At least one system must take the Active role (§6.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880PassiveRoleSilentUntilReception`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L394) | unit/verify | unproven |
| positive | [`TestRFC5880ActiveRoleTransmitsWithoutReception`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L366) | unit/verify | unproven |

### [`RFC5880-6.8.3-1`](#rfc5880-6.8.3-1)

When session state is not Up, bfd.DesiredMinTxInterval must be at least 1,000,000 microseconds (§6.8.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880SlowStartFloorLiftedWhenUp`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L232) | unit/verify | unproven |
| positive | [`TestRFC5880SlowStartFloorWhileNotUp`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L213) | unit/verify | unproven |

### [`RFC5880-6.8.3-2`](#rfc5880-6.8.3-2)

If bfd.DesiredMinTxInterval or bfd.RequiredMinRxInterval changes, a Poll Sequence must be initiated (§6.8.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880NoPollWhenIntervalsUnchanged`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L470) | unit/verify | unproven |
| positive | [`TestRFC5880PollInitiatedOnIntervalChange`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L436) | unit/verify | unproven |

### [`RFC5880-6.8.3-3`](#rfc5880-6.8.3-3)

If bfd.DesiredMinTxInterval is increased while Up, the actual TX interval must not change until the Poll Sequence terminates (§6.8.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5880-6.8.3-3, so no unit is bound to it.

### [`RFC5880-6.8.3-4`](#rfc5880-6.8.3-4)

If bfd.RequiredMinRxInterval is reduced while Up, the previous value must be used for detection time until the Poll terminates (§6.8.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5880-6.8.3-4, so no unit is bound to it.

### [`RFC5880-6.8.3-5`](#rfc5880-6.8.3-5)

If local system reduces TX interval due to bfd.RemoteMinRxInterval being reduced, it must honor the new interval immediately (§6.8.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880TransmitIntervalFlooredByLocalDesired`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L533) | unit/verify | unproven |
| positive | [`TestRFC5880RemoteMinRxReductionHonoredImmediately`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L497) | unit/verify | unproven |

### [`RFC5880-6.8.3-6`](#rfc5880-6.8.3-6)

Multiple parameter changes requiring Poll Sequence must be communicated in a single packet, or a round-trip must elapse, or an F=0 packet must be received before starting another Poll (§6.8.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC5880PollInitiatedOnIntervalChange`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L441) | unit/verify | unproven |

### [`RFC5880-6.5-1`](#rfc5880-6.5-1)

A BFD Control packet must not have both Poll (P) and Final (F) bits set (§6.5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880FinalReplyClearsPollBit`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L586) | unit/verify | unproven |
| positive | [`TestRFC5880PollPacketHasNoFinalBit`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L567) | unit/verify | unproven |

### [`RFC5880-6.5-2`](#rfc5880-6.5-2)

If periodic Control packets are being sent, Poll Sequence must be performed by setting P bit on scheduled transmissions; additional packets must not be sent (§6.5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880FinalReplyClearsPollBit`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L590) | unit/verify | unproven |
| positive | [`TestRFC5880PollPacketHasNoFinalBit`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L571) | unit/verify | unproven |

### [`RFC5880-6.6-1`](#rfc5880-6.6-1)

Demand (D) bit must not be set unless bfd.DemandMode is 1, bfd.SessionState is Up, and bfd.RemoteSessionState is Up (§6.6, §6.8.7)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880DemandBitClearWhenAnyConditionFails`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L666) | unit/verify | unproven |
| positive | [`TestRFC5880DemandBitSetWhenAllConditionsHold`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L652) | unit/verify | unproven |

### [`RFC5880-6.6-2`](#rfc5880-6.6-2)

When the D bit value is to be changed, a Poll Sequence must be initiated (§6.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5880-6.6-2, so no unit is bound to it.

### [`RFC5880-6.6-3`](#rfc5880-6.6-3)

If Demand mode is active on either system, a Poll Sequence must be initiated whenever next packet contents would differ (except P/F bits) (§6.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5880-6.6-3, so no unit is bound to it.

### [`RFC5880-6.7-1`](#rfc5880-6.7-1)

Implementations supporting authentication must support both types of SHA1 authentication (§6.7)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880UnsupportedAuthTypesRejected`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L67) | unit/verify | unproven |
| positive | [`TestRFC5880BothSHA1VariantsSupported`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L48) | unit/verify | unproven |

### [`RFC5880-4.2-1`](#rfc5880-4.2-1)

Simple Password must be 1 to 16 bytes in length (§4.2, §6.7.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`auth/TestRFC5880SimplePasswordLengthOutOfRangeRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L744) | unit/verify | unproven |
| negative | [`bfd/TestRFC5880SimplePasswordLengthOutOfRangeRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5880_test.go#L128) | unit/verify | unproven |
| positive | [`TestRFC5880SimplePasswordSectionHeader`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L589) | unit/verify | unproven |
| positive | [`TestRFC5880SimplePasswordManagementAcceptsASCIIStrings`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5880_test.go#L101) | unit/verify | unproven |

### [`RFC5880-6.7.2-1`](#rfc5880-6.7.2-1)

Simple Password management interface must accept ASCII strings (§6.7.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`bfd/TestRFC5880SimplePasswordLengthOutOfRangeRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5880_test.go#L132) | unit/verify | unproven |
| positive | [`TestRFC5880SimplePasswordManagementAcceptsASCIIStrings`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5880_test.go#L96) | unit/verify | unproven |

### [`RFC5880-6.7.2-8`](#rfc5880-6.7.2-8)

Simple Password Auth Type must be set to 1, Auth Len must be set to the proper length of 4 to 19 bytes (§6.7.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880SimplePasswordRejectsForeignSectionShape`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L610) | unit/verify | unproven |
| positive | [`TestRFC5880SimplePasswordSectionHeader`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L584) | unit/verify | unproven |

### [`RFC5880-6.7.3-1`](#rfc5880-6.7.3-1)

Keyed MD5 Auth Type must be set to 2 or 3, Auth Len must be 24 (§6.7.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880KeyedMD5RejectsForeignSectionShape`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L110) | unit/verify | unproven |
| positive | [`TestRFC5880KeyedMD5SectionHeader`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L83) | unit/verify | unproven |

### [`RFC5880-6.7.3-2`](#rfc5880-6.7.3-2)

Keyed MD5 Auth Key/Digest: MD5 digest must be calculated over entire BFD Control packet (§6.7.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880DigestRejectsMandatorySectionTamper`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L198) | unit/verify | unproven |
| positive | [`TestRFC5880DigestCoversWholePacket`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L179) | unit/verify | unproven |

### [`RFC5880-6.7.3-3`](#rfc5880-6.7.3-3)

Keyed MD5: secret key must not be carried in the packet (§6.7.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC5880SecretKeyNotCarriedInPacket`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L222) | unit/verify | unproven |

### [`RFC5880-6.7.3-4`](#rfc5880-6.7.3-4)

Meticulous Keyed MD5: bfd.XmitAuthSeq must be incremented for each packet (§6.7.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880MeticulousRejectsUnincrementedSequence`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L536) | unit/verify | unproven |
| positive | [`TestRFC5880MeticulousSequenceIncrementsPerPacket`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L287) | unit/verify | unproven |

### [`RFC5880-6.7.4-1`](#rfc5880-6.7.4-1)

Keyed SHA1 Auth Type must be set to 4 or 5, Auth Len must be 28 (§6.7.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880KeyedSHA1RejectsForeignSectionShape`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L156) | unit/verify | unproven |
| positive | [`TestRFC5880KeyedSHA1SectionHeader`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L135) | unit/verify | unproven |

### [`RFC5880-6.7.4-2`](#rfc5880-6.7.4-2)

SHA1 hash must be calculated over entire BFD Control packet (§6.7.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880DigestRejectsMandatorySectionTamper`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L203) | unit/verify | unproven |
| positive | [`TestRFC5880DigestCoversWholePacket`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L185) | unit/verify | unproven |

### [`RFC5880-6.7.4-3`](#rfc5880-6.7.4-3)

Keyed SHA1: secret key must not be carried in the packet (§6.7.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC5880SecretKeyNotCarriedInPacket`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L226) | unit/verify | unproven |

### [`RFC5880-6.7.4-4`](#rfc5880-6.7.4-4)

Meticulous Keyed SHA1: bfd.XmitAuthSeq must be incremented for each packet (§6.7.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880MeticulousRejectsUnincrementedSequence`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L541) | unit/verify | unproven |
| positive | [`TestRFC5880MeticulousSequenceIncrementsPerPacket`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L293) | unit/verify | unproven |

### [`RFC5880-6.7.4-5`](#rfc5880-6.7.4-5)

SHA1 key management interface must accept ASCII strings (§6.7.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880KeyManagementRejectsIncompleteConfig`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5880_test.go#L70) | unit/verify | unproven |
| positive | [`TestRFC5880KeyManagementAcceptsASCIIStrings`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5880_test.go#L31) | unit/verify | unproven |

### [`RFC5880-6.8.6-1`](#rfc5880-6.8.6-1)

Reception: discard if Version != 1 (§6.8.6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880VersionNotOneDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/packet/rfc5880_test.go#L66) | unit/verify | unproven |
| positive | [`TestRFC5880VersionOneAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/packet/rfc5880_test.go#L48) | unit/verify | unproven |

### [`RFC5880-6.8.6-2`](#rfc5880-6.8.6-2)

Reception: discard if Length < 24 (A=0) or < 26 (A=1) (§6.8.6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880LengthBelowMinimumDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/packet/rfc5880_test.go#L128) | unit/verify | unproven |
| positive | [`TestRFC5880LengthMinimumAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/packet/rfc5880_test.go#L108) | unit/verify | unproven |

### [`RFC5880-6.8.6-3`](#rfc5880-6.8.6-3)

Reception: discard if Length > encapsulating payload (§6.8.6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880LengthOverPayloadDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/packet/rfc5880_test.go#L163) | unit/verify | unproven |
| positive | [`TestRFC5880LengthEqualsPayloadAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/packet/rfc5880_test.go#L148) | unit/verify | unproven |

### [`RFC5880-6.8.6-4`](#rfc5880-6.8.6-4)

Reception: discard if Detect Mult == 0 (§6.8.6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880DetectMultZeroDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/packet/rfc5880_test.go#L190) | unit/verify | unproven |
| positive | [`TestRFC5880DetectMultNonZeroAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/packet/rfc5880_test.go#L174) | unit/verify | unproven |

### [`RFC5880-6.8.6-5`](#rfc5880-6.8.6-5)

Reception: discard if M bit is nonzero (§6.8.6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880MultipointSetDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/packet/rfc5880_test.go#L99) | unit/verify | unproven |
| positive | [`TestRFC5880MultipointZeroAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/packet/rfc5880_test.go#L84) | unit/verify | unproven |

### [`RFC5880-6.8.6-6`](#rfc5880-6.8.6-6)

Reception: discard if My Discriminator == 0 (§6.8.6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880MyDiscriminatorZeroDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/packet/rfc5880_test.go#L214) | unit/verify | unproven |
| positive | [`TestRFC5880MyDiscriminatorNonZeroAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/packet/rfc5880_test.go#L200) | unit/verify | unproven |

### [`RFC5880-6.8.6-7`](#rfc5880-6.8.6-7)

Reception: if Your Discriminator nonzero, use it to select session; discard if no session found (§6.8.6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880UnknownYourDiscriminatorDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L140) | unit/verify | unproven |
| positive | [`TestRFC5880YourDiscriminatorSelectsSession`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L124) | unit/verify | unproven |

### [`RFC5880-6.8.6-8`](#rfc5880-6.8.6-8)

Reception: if Your Discriminator zero and State is not Down or AdminDown, discard (§6.8.6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880ZeroYourDiscriminatorDiscardedWhenLive`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L699) | unit/verify | unproven |
| positive | [`TestRFC5880ZeroYourDiscriminatorAcceptedWhenDown`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L724) | unit/verify | unproven |

### [`RFC5880-6.8.6-9`](#rfc5880-6.8.6-9)

Reception: if A=1 and bfd.AuthType is zero, discard (§6.8.6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880UnauthenticatedSessionDiscardsAuthenticatedPacket`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L761) | unit/verify | unproven |
| positive | [`TestRFC5880UnauthenticatedSessionAcceptsUnauthenticatedPacket`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L746) | unit/verify | unproven |

### [`RFC5880-6.8.6-10`](#rfc5880-6.8.6-10)

Reception: if A=0 and bfd.AuthType is nonzero, discard (§6.8.6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880AuthenticatedSessionDiscardsUnauthenticatedPacket`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L796) | unit/verify | unproven |
| positive | [`TestRFC5880AuthenticatedSessionAcceptsAuthenticatedPacket`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L778) | unit/verify | unproven |

### [`RFC5880-6.8.6-11`](#rfc5880-6.8.6-11)

Reception: if A=1, authenticate per §6.7 (§6.8.6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880UnauthenticPacketDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L258) | unit/verify | unproven |
| positive | [`TestRFC5880AuthenticatedPacketVerifiedAndDelivered`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L241) | unit/verify | unproven |

### [`RFC5880-6.8.6-12`](#rfc5880-6.8.6-12)

Reception: if Required Min Echo RX Interval == 0, cease Echo transmission (§6.8.6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880EchoEnabledWhenPeerAdvertisesNonZero`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L849) | unit/verify | unproven |
| positive | [`TestRFC5880EchoCeasesWhenPeerAdvertisesZero`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L812) | unit/verify | unproven |

### [`RFC5880-6.8.6-13`](#rfc5880-6.8.6-13)

Reception: if Poll in flight and F=1 received, terminate the Poll Sequence (§6.8.6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880NonFinalDoesNotTerminatePoll`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L630) | unit/verify | unproven |
| positive | [`TestRFC5880FinalTerminatesPoll`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L610) | unit/verify | unproven |

### [`RFC5880-6.8.6-14`](#rfc5880-6.8.6-14)

Reception: if remote Demand mode active (D=1, both Up), cease periodic Control packet transmission (§6.8.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5880-6.8.6-14, so no unit is bound to it.

### [`RFC5880-6.8.6-15`](#rfc5880-6.8.6-15)

Reception: if remote Demand mode not active, send periodic Control packets (§6.8.6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880NoPeriodicTransmitWhileAdminDown`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L403) | unit/verify | unproven |
| positive | [`TestRFC5880PeriodicTransmitWhenRemoteDemandInactive`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L368) | unit/verify | unproven |

### [`RFC5880-6.8.6-16`](#rfc5880-6.8.6-16)

Reception: if P=1, send Final packet immediately (§6.8.6, §6.8.7)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880NonPollProducesNoImmediateReply`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L352) | unit/verify | unproven |
| positive | [`TestRFC5880PollAnsweredWithImmediateFinal`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L322) | unit/verify | unproven |

### [`RFC5880-6.8.6-18`](#rfc5880-6.8.6-18)

Reception: if Your Discriminator zero, select the session on a combination of other fields, which can include source addressing information, My Discriminator, and the ingress interface (§6.8.6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5881FirstPacketWrongSourceDropped`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5881_test.go#L128) | unit/verify | unproven |
| positive | [`TestRFC5881FirstPacketMatchesByTuple`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5881_test.go#L105) | unit/verify | unproven |

### [`RFC5880-6.8.7-1`](#rfc5880-6.8.7-1)

Transmission: must not transmit at interval less than max(bfd.DesiredMinTxInterval, bfd.RemoteMinRxInterval) less jitter (§6.8.7)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880TransmitDeadlineClampsBadJitter`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L1045) | unit/verify | unproven |
| positive | [`TestRFC5880TransmitDeadlineUsesNegotiatedInterval`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L1015) | unit/verify | unproven |

### [`RFC5880-6.8.7-2`](#rfc5880-6.8.7-2)

Transmission: periodic TX must be jittered by 0-25% per packet (§6.8.7)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880JitterStaysWithinBand`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L450) | unit/verify | unproven |
| positive | [`TestRFC5880JitterIsAppliedPerPacket`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L422) | unit/verify | unproven |

### [`RFC5880-6.8.7-3`](#rfc5880-6.8.7-3)

Transmission: if bfd.DetectMult == 1, interval must be 75-90% of negotiated interval (§6.8.7)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880JitterFloorOnlyForDetectMultOne`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L488) | unit/verify | unproven |
| positive | [`TestRFC5880JitterDetectMultOneWindow`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L470) | unit/verify | unproven |

### [`RFC5880-6.8.7-4`](#rfc5880-6.8.7-4)

Transmission: TX interval must be recalculated whenever bfd.DesiredMinTxInterval or bfd.RemoteMinRxInterval changes (§6.8.7)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880TransmitIntervalFlooredByLocalDesired`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L538) | unit/verify | unproven |
| positive | [`TestRFC5880RemoteMinRxReductionHonoredImmediately`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L504) | unit/verify | unproven |

### [`RFC5880-6.8.7-5`](#rfc5880-6.8.7-5)

Transmission: must not transmit if bfd.RemoteDiscr is zero and system is Passive (§6.8.7)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880ActiveRoleTransmitsWithoutReception`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L369) | unit/verify | unproven |
| positive | [`TestRFC5880PassiveRoleSilentUntilReception`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L391) | unit/verify | unproven |

### [`RFC5880-6.8.7-6`](#rfc5880-6.8.7-6)

Transmission: must not periodically transmit if bfd.RemoteMinRxInterval is zero (§6.8.7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5880-6.8.7-6, so no unit is bound to it.

### [`RFC5880-6.8.7-7`](#rfc5880-6.8.7-7)

Transmission: must not periodically transmit if remote Demand mode active and no Poll in flight (§6.8.7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5880-6.8.7-7, so no unit is bound to it.

### [`RFC5880-6.8.7-8`](#rfc5880-6.8.7-8)

If rate limiting Final packets, advertised Desired Min TX Interval must be >= rate-limit interval (§6.8.7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5880-6.8.7-8, so no unit is bound to it.

### [`RFC5880-6.8.4-1`](#rfc5880-6.8.4-1)

Detection time expired while Init or Up: set bfd.SessionState to Down, bfd.LocalDiag to 1 (§6.8.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880DetectionExpiryIgnoredWhenDown`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L333) | unit/verify | unproven |
| positive | [`TestRFC5880DetectionExpiryDownDiagOne`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L312) | unit/verify | unproven |

### [`RFC5880-6.8.5-1`](#rfc5880-6.8.5-1)

Echo function failure: set bfd.SessionState to Down, bfd.LocalDiag to 2 (§6.8.5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880EchoReturnClearsMissAndFailIsScoped`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L979) | unit/verify | unproven |
| positive | [`TestRFC5880EchoMissDetectedAndReported`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L943) | unit/verify | unproven |

### [`RFC5880-6.8.15-1`](#rfc5880-6.8.15-1)

Forwarding plane reset: set bfd.LocalDiag to 4, bfd.SessionState to Down (§6.8.15)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5880-6.8.15-1, so no unit is bound to it.

### [`RFC5880-6.8.16-1`](#rfc5880-6.8.16-1)

Administrative control: follow the enable/disable procedure (§6.8.16)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880AdministrativeCallsAreGuarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L1106) | unit/verify | unproven |
| positive | [`TestRFC5880AdministrativeDisableEnable`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L1071) | unit/verify | unproven |

### [`RFC5880-6.8.8-1`](#rfc5880-6.8.8-1)

Echo packets must be demultiplexed to the appropriate session (§6.8.8)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880UnknownEchoDropped`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L626) | unit/verify | unproven |
| positive | [`TestRFC5880EchoDemultiplexedToItsSession`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L589) | unit/verify | unproven |

### [`RFC5880-6.8.8-2`](#rfc5880-6.8.8-2)

A means of detecting missing Echo packets must be implemented (§6.8.8)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880EchoReturnClearsMissAndFailIsScoped`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L974) | unit/verify | unproven |
| positive | [`TestRFC5880EchoMissDetectedAndReported`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L938) | unit/verify | unproven |

### [`RFC5880-6.8.9-1`](#rfc5880-6.8.9-1)

Echo packets must not be transmitted when bfd.SessionState is not Up (§6.8.9)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880NoEchoTransmittedWhenNotUp`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L564) | unit/verify | unproven |
| positive | [`TestRFC5880EchoTransmittedWhileUp`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/engine/rfc5880_test.go#L543) | unit/verify | unproven |

### [`RFC5880-6.8.9-2`](#rfc5880-6.8.9-2)

Echo packets must not be transmitted unless remote Required Min Echo RX Interval is nonzero (§6.8.9)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880EchoNotTransmittedWithoutPeerAdvertisement`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L873) | unit/verify | unproven |
| positive | [`TestRFC5880EchoEnabledWhenPeerAdvertisesNonZero`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L852) | unit/verify | unproven |

### [`RFC5880-6.8.9-3`](#rfc5880-6.8.9-3)

Echo packet TX interval must not be less than remote Required Min Echo RX Interval (§6.8.9)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880EchoIntervalNotBelowLocalTarget`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L915) | unit/verify | unproven |
| positive | [`TestRFC5880EchoIntervalHonorsPeerFloor`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L896) | unit/verify | unproven |

### [`RFC5880-7-1`](#rfc5880-7-1)

Multihop: a congestion control mechanism must be implemented (§7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5880-7-1, so no unit is bound to it.

### [`RFC5880-7-2`](#rfc5880-7-2)

Multihop: when congestion detected, BFD must reduce traffic generated (§7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5880-7-2, so no unit is bound to it.

### [`RFC5880-4.3-1`](#rfc5880-4.3-1)

Reserved byte in MD5/SHA1 auth section must be set to zero on transmit (§4.3, §4.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC5880KeyedMD5SectionHeader`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L88) | unit/verify | unproven |

### [`RFC5880-9-2`](#rfc5880-9-2)

For single-hop sessions, TTL or Hop Count MUST be set to maximum on transmit and checked equal to maximum on receipt (§9)

Audit verdict: not audited: no reader has judged these tests

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

If concatenated path failure diagnostic must be communicated and remote Demand mode is active, a Poll Sequence MUST be initiated (§6.8.17)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5880-6.8.17-1, so no unit is bound to it.

### [`RFC5880-6.8.3-8`](#rfc5880-6.8.3-8)

Timing parameter changes not explicitly excepted MUST be effected immediately (§6.8.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC5880RemoteMinRxReductionHonoredImmediately`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L506) | unit/verify | unproven |

### [`RFC5880-6.7.3-7`](#rfc5880-6.7.3-7)

MD5 key management interface MUST accept ASCII strings (§6.7.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880KeyManagementRejectsIncompleteConfig`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5880_test.go#L64) | unit/verify | unproven |
| positive | [`TestRFC5880KeyManagementAcceptsASCIIStrings`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/rfc5880_test.go#L26) | unit/verify | unproven |

### [`RFC5880-6.7.2-3`](#rfc5880-6.7.2-3)

Simple Password, MD5, and SHA1 Auth Key ID field MUST be set to the ID of the current authentication key (§6.7.2, §6.7.3, §6.7.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880AuthKeyIDMismatchDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L264) | unit/verify | unproven |
| positive | [`TestRFC5880AuthKeyIDIsTheConfiguredKey`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L246) | unit/verify | unproven |
| positive | [`TestRFC5880SimplePasswordSectionCarriesPasswordAndKeyID`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L639) | unit/verify | unproven |

### [`RFC5880-6.7.3-8`](#rfc5880-6.7.3-8)

Simple Password, MD5, and SHA1 Sequence Number field MUST be set to bfd.XmitAuthSeq (§6.7.3, §6.7.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880AuthSequenceFieldFollowsAdvance`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L1163) | unit/verify | unproven |
| positive | [`TestRFC5880AuthSequenceFieldIsXmitAuthSeq`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/session/rfc5880_test.go#L1140) | unit/verify | unproven |

### [`RFC5880-6.7.2-4`](#rfc5880-6.7.2-4)

Reception: if Auth Type does not match bfd.AuthType, packet MUST be discarded (§6.7.2, §6.7.3, §6.7.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880AuthTypeMismatchDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L310) | unit/verify | unproven |
| positive | [`TestRFC5880AuthTypeMatchAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L297) | unit/verify | unproven |

### [`RFC5880-6.7.2-5`](#rfc5880-6.7.2-5)

Reception: if Auth Key ID does not match any configured authentication key, packet MUST be discarded (§6.7.2, §6.7.3, §6.7.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880AuthKeyIDMismatchDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L267) | unit/verify | unproven |
| negative | [`TestRFC5880SimplePasswordWrongKeyIDDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L712) | unit/verify | unproven |
| positive | [`TestRFC5880AuthKeyIDMatchAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L285) | unit/verify | unproven |
| positive | [`TestRFC5880SimplePasswordMatchingKeyIDAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L729) | unit/verify | unproven |

### [`RFC5880-6.7.2-6`](#rfc5880-6.7.2-6)

Reception: if Auth Len does not match expected length, packet MUST be discarded (§6.7.2, §6.7.3, §6.7.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880AuthLenMismatchDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L343) | unit/verify | unproven |
| positive | [`TestRFC5880AuthLenExpectedAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L326) | unit/verify | unproven |

### [`RFC5880-6.7.2-7`](#rfc5880-6.7.2-7)

Reception: if password does not match configured password for Simple Password, packet MUST be discarded (§6.7.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880SimplePasswordMismatchDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L681) | unit/verify | unproven |
| positive | [`TestRFC5880SimplePasswordMatchAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L662) | unit/verify | unproven |

### [`RFC5880-6.7.3-9`](#rfc5880-6.7.3-9)

Reception: for Keyed MD5/SHA1, if Sequence Number is less than bfd.RcvAuthSeq (accounting for wrap), packet MUST be discarded (§6.7.3, §6.7.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880KeyedSequenceBelowFloorDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L514) | unit/verify | unproven |
| positive | [`TestRFC5880KeyedSequenceAtOrAboveFloorAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L494) | unit/verify | unproven |

### [`RFC5880-6.7.3-10`](#rfc5880-6.7.3-10)

Reception: for Meticulous Keyed MD5/SHA1, if Sequence Number is not exactly bfd.RcvAuthSeq+1 (accounting for wrap), packet MUST be discarded (§6.7.3, §6.7.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5880-6.7.3-10, so no unit is bound to it.

### [`RFC5880-6.7.3-11`](#rfc5880-6.7.3-11)

Reception: if digest/hash does not match computed value, packet MUST be discarded (§6.7.3, §6.7.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880DigestMismatchDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L390) | unit/verify | unproven |
| positive | [`TestRFC5880DigestMatchAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L377) | unit/verify | unproven |

### [`RFC5880-6.7.3-12`](#rfc5880-6.7.3-12)

Reception: if bfd.AuthSeqKnown is 0, it MUST be set to 1 and bfd.RcvAuthSeq MUST be set to received Sequence Number (§6.7.3, §6.7.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5880ForgedFirstPacketDoesNotSeedReplayFloor`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L471) | unit/verify | unproven |
| positive | [`TestRFC5880FirstAuthenticatedPacketSeedsReplayFloor`](https://github.com/ze-software/ze/blob/main/internal/component/bfd/auth/rfc5880_test.go#L447) | unit/verify | unproven |

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
