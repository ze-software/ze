# RFC 5303 - Three-Way Handshake for IS-IS Point-to-Point Adjacencies

Experimental. Every requirement this repository extracted from RFC 5303, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 58.8% | 10 of 17 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 17 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 17 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 17 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Proven by a recorded break | 81.8% | 27 of 33 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 17 | of 18 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 2 | of 17 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 11.8% | 2 of 17 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 17 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 17 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 29.4% | 5 of 17 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

The 8 shares marked as a part above are the whole of the 17 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Experimental |
| Enrolment | Enrolled |
| Requirements | 18 |
| Gated MUST-level | 17 |
| Not applicable, so out of scope | 2 |
| Declared gaps | 5 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 33 |
| Tagged units | 33 |
| Recorded audit verdicts | 10 |
| Discrimination records | 27 |
| Summary | `rfc/short/rfc5303.md` |
| Requirement shard | `rfc/requirements/rfc5303.md` |
| RFC text | `rfc/full/rfc5303.txt` |

## Enrolment

Enrolled: The point-to-point handshake has per-requirement tests and explicit gaps below. Invalid-state discard is implemented and exercised through the packet decoder and circuit receiver; enrolment does not claim full support. Earlier fixed MET/gap counts described summary annotations, not a verified implementation count.

## What the public ledger says

**Status:** Experimental

**What the ledger says is covered**

Point-to-point IIH three-way handshake: TLV 240 (Point-to-Point Three-Way Adjacency) origination and decode, the mandatory Adjacency Three-Way State and Extended Local Circuit ID fields, the Neighbor System ID echo, the Section 3.2 Adjacency Three-Way State Table that decides each received Hello (including the restart "Down" action that deletes a new adjacency receiving Up), and the legacy two-way fall-back for peers that omit the option. Tests bound per requirement in [`rfc/requirements/rfc5303.md`](https://github.com/ze-software/ze/blob/main/rfc/requirements/rfc5303.md).

**What the ledger says remains**

Gaps gated in [`rfc/short/rfc5303.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc5303.md): the Extended Local Circuit ID is derived as uint8(ifindex) so it is not unique beyond 256 interfaces ([`RFC5303-3.2-4`](#rfc5303-3.2-4)); the Neighbor Extended Local Circuit ID is echoed as 0 and never examined ([`RFC5303-3.2-6`](#rfc5303-3.2-6), [`RFC5303-3.2-8`](#rfc5303-3.2-8)); the loop-detection neighbor-mismatch discard is absent ([`RFC5303-3.2-9`](#rfc5303-3.2-9)); and the restart "Down" action deletes the adjacency without the "Neighbor restarted" adjacencyStateChange event ([`RFC5303-3.2-12`](#rfc5303-3.2-12)). Invalid-state discard now has valid-input and invalid-input controls; the earlier claim that this path was absent is obsolete.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 10 | one part of the gated population |
| Annotated (including scoped evidence) | 7 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **17** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (10):** [`RFC5303-3.1-1`](#rfc5303-3.1-1), [`RFC5303-3.1-4`](#rfc5303-3.1-4), [`RFC5303-3.2-1`](#rfc5303-3.2-1), [`RFC5303-3.2-2`](#rfc5303-3.2-2), [`RFC5303-3.2-3`](#rfc5303-3.2-3), [`RFC5303-3.2-5`](#rfc5303-3.2-5), [`RFC5303-3.2-7`](#rfc5303-3.2-7), [`RFC5303-3.2-10`](#rfc5303-3.2-10), [`RFC5303-3.2-11`](#rfc5303-3.2-11), [`RFC5303-3.2-13`](#rfc5303-3.2-13)

**Annotated (including scoped evidence) (7):** [`RFC5303-3.1-2`](#rfc5303-3.1-2), [`RFC5303-3.1-3`](#rfc5303-3.1-3), [`RFC5303-3.2-4`](#rfc5303-3.2-4), [`RFC5303-3.2-6`](#rfc5303-3.2-6), [`RFC5303-3.2-8`](#rfc5303-3.2-8), [`RFC5303-3.2-9`](#rfc5303-3.2-9), [`RFC5303-3.2-12`](#rfc5303-3.2-12)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC5303-3.1-1` | Any system that supports this mechanism SHALL include this option in its Point-to-Point IIH packets. (§3.1) | SHALL | 3.1 | **positive:** `unit/verify` [`TestISISP2PIIHCarriesThreeWayOption`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/rfc5303_threeway_test.go#L125). **negative:** `unit/verify` [`TestISISP2PIIHCarriesThreeWayOption`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/rfc5303_threeway_test.go#L129) |
| `RFC5303-3.1-2` | Any system that does not understand this option SHALL ignore it (§3.1) | SHALL | 3.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** this constrains a system that does NOT understand the option; Ze implements and processes it (packet/tlv_core.go DecodeP2PThreeWayTLV decodes it, circuit/runtime.go:143 consumes it), so the "does not understand" role never applies to Ze |
| `RFC5303-3.1-3` | Any system that does not understand this option SHALL ignore it, and (of course) SHALL NOT include it in its own IIH packets. (§3.1) | SHALL NOT | 3.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** this constrains a non-supporting system's emission; Ze supports the mechanism and always emits TLV 240 in its point-to-point IIH (circuit/hello.go:158 threeWayTLV, circuit/hello.go:204 buildP2PHello), so the "does not understand" role never applies to Ze |
| `RFC5303-3.1-4` | Any system that supports this mechanism MUST include the Adjacency Three-Way State field in this option. (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestISISP2PThreeWayStateFieldIncluded`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/rfc5303_threeway_test.go#L16). **negative:** `unit/verify` [`TestISISP2PThreeWayStateFieldPresentWithoutNeighbor`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/rfc5303_threeway_test.go#L49) |
| `RFC5303-3.2-1` | The current three-way state of the adjacency with its neighbor on the link (as defined in new section 8.2.4.1.1 introduced later in the document) SHALL be reported in the Adjacency Three-Way State field. (§3.2) | SHALL | 3.2 | **positive:** `unit/verify` [`TestISISThreeWayReportsCurrentState`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/rfc5303_threeway_test.go#L146). **positive:** `unit/verify` [`TestRFC5303SentIIHReportsCurrentThreeWayState`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/rfc5303_sent_iih_test.go#L82). **negative:** `unit/verify` [`TestISISThreeWayReportsCurrentState`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/rfc5303_threeway_test.go#L149). **negative:** `unit/verify` [`TestRFC5303SentIIHReportsCurrentThreeWayState`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/rfc5303_sent_iih_test.go#L85) |
| `RFC5303-3.2-2` | If no adjacency exists, the state SHALL be reported as Down. (§3.2) | SHALL | 3.2 | **positive:** `unit/verify` [`TestISISThreeWayDownWhenNoAdjacency`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/rfc5303_threeway_test.go#L175). **positive:** `unit/verify` [`TestRFC5303SentIIHReportsDownWithoutAdjacency`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/rfc5303_sent_iih_test.go#L105). **negative:** `unit/verify` [`TestISISThreeWayDownWhenNoAdjacency`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/rfc5303_threeway_test.go#L178). **negative:** `unit/verify` [`TestRFC5303SentIIHReportsDownWithoutAdjacency`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/rfc5303_sent_iih_test.go#L108) |
| `RFC5303-3.2-3` | The Extended Local Circuit ID field SHALL contain a value assigned by this IS when the circuit is created. (§3.2) | SHALL | 3.2 | **positive:** `unit/verify` [`TestISISThreeWayExtendedLocalCircuitID`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/rfc5303_threeway_test.go#L203). **negative:** `unit/verify` [`TestISISThreeWayExtendedLocalCircuitID`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/rfc5303_threeway_test.go#L206) |
| `RFC5303-3.2-4` | This value SHALL be unique among all the circuits of this Intermediate System. (§3.2) | SHALL | 3.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the Extended Local Circuit ID is derived as uint8(ifindex) (circuits.go:317), truncating the interface index to 8 bits; two circuits whose ifindexes collide modulo 256 emit the same value, so uniqueness among all circuits is not guaranteed and the 4-octet field never exceeds 255 |
| `RFC5303-3.2-5` | If the system ID and Extended Local Circuit ID of the neighboring system are known (in adjacency three-way state Initializing or Up), the neighbor's system ID SHALL be reported in the Neighbor System ID field (§3.2) | SHALL | 3.2 | **positive:** `unit/verify` [`TestISISThreeWayReportsNeighborSystemID`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/rfc5303_threeway_test.go#L226). **positive:** `unit/verify` [`TestRFC5303SentIIHEchoesKnownNeighbor`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/rfc5303_sent_iih_test.go#L130). **negative:** `unit/verify` [`TestISISThreeWayReportsNeighborSystemID`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/rfc5303_threeway_test.go#L229). **negative:** `unit/verify` [`TestRFC5303SentIIHEchoesKnownNeighbor`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/rfc5303_sent_iih_test.go#L133) |
| `RFC5303-3.2-6` | the neighbor's Extended Local Circuit ID SHALL be reported in the Neighbor Extended Local Circuit ID field. (§3.2) | SHALL | 3.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze does not track the neighbor's Extended Local Circuit ID; threeWayTLV echoes a constant 0 in the Neighbor Extended Local Circuit ID field (circuit/hello.go:168) and updateThreeWay never stores the neighbor's value (adjacency/fsm.go updateThreeWay), so the neighbor's actual extended circuit ID is never reported |
| `RFC5303-3.2-7` | If the option is present and contains invalid Adjacency Three-Way State, the PDU SHALL be discarded and no further action is taken. (§3.2) | SHALL | 3.2 | **positive:** `unit/verify` [`TestRFC5303InvalidStateWireDiscard`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/rfc5303_invalid_state_test.go#L16). **negative:** `unit/verify` [`TestRFC5303InvalidStateBeforeValidDuplicate`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/rfc5303_invalid_state_test.go#L73). **negative:** `unit/verify` [`TestRFC5303InvalidStateLeavesAdjacencyUntouched`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/adjacency/rfc5303_invalid_state_test.go#L14). **negative:** `unit/verify` [`TestRFC5303InvalidStateWireDiscard`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/rfc5303_invalid_state_test.go#L17) |
| `RFC5303-3.2-8` | If the option with a valid Adjacency Three-Way State is present, the Neighbor System ID and Neighbor Extended Local Circuit ID fields, if present, SHALL be examined. (§3.2) | SHALL | 3.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** updateThreeWay examines only the Neighbor System ID (adjacency/fsm.go updateThreeWay) to set neighborNamesOther; the Neighbor Extended Local Circuit ID is decoded (packet/tlv_core.go DecodeP2PThreeWayTLV, the p2pThreeWayLenFull branch) but the FSM never examines it, so the required examination of both fields is incomplete |
| `RFC5303-3.2-9` | If they are present, and the Neighbor System ID contained therein does not match the local system's ID, or the Neighbor Extended Local Circuit ID does not match the local system's extended circuit ID, the PDU SHALL be discarded and no further action is taken. (§3.2) | SHALL | 3.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** on a Neighbor System ID that does not match ours Ze sets the adjacency Initializing (adjacency/fsm.go threeWayTableAction) and still processes the PDU, arming the hold timer and recording the neighbor (adjacency/fsm.go ReceiveHello); it neither discards the PDU nor checks the Neighbor Extended Local Circuit ID, so the loop-detection discard is absent |
| `RFC5303-3.2-10` | In section 8.2.4.2 a and b, the action "Up" from state tables 5, 6, 7, and 8 may create a new adjacency but the three-way state of the adjacency SHALL be Down. (§3.2) | SHALL | 3.2 | **positive:** `unit/verify` [`TestISISThreeWayNewAdjacencyStateDown`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/rfc5303_threeway_test.go#L247). **positive:** `unit/verify` [`TestRFC5303NewAdjacencyReceivingUpDoesNotComeUp`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/adjacency/rfc5303_new_adjacency_red_test.go#L37). **negative:** `unit/verify` [`TestISISThreeWayNewAdjacencyStateDown`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/rfc5303_threeway_test.go#L251). **negative:** `unit/verify` [`TestRFC5303NewAdjacencyReceivingUpDoesNotComeUp`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/adjacency/rfc5303_new_adjacency_red_test.go#L40) |
| `RFC5303-3.2-11` | If the action taken from section 8.2.4.2 a or b is "Up" or "Accept", the IS SHALL perform the action indicated by the new adjacency three-way state table below, based on the current adjacency three-way state and the received Adjacency Three-Way State value from the option. (§3.2) | SHALL | 3.2 | **positive:** `unit/verify` [`TestISISThreeWayProceduresEngagedByOption`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/adjacency/rfc5303_fsm_test.go#L400). **positive:** `unit/verify` [`TestRFC5303ThreeWayStateTable`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/adjacency/rfc5303_new_adjacency_red_test.go#L94). **negative:** `unit/verify` [`TestRFC5303ThreeWayStateTable`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/adjacency/rfc5303_new_adjacency_red_test.go#L98) |
| `RFC5303-3.2-12` | If the new action is "Down", an adjacencyStateChange(Down) event is generated with the reason "Neighbor restarted" and the adjacency SHALL be deleted. (§3.2) | SHALL | 3.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the "Down" action deletes the adjacency (adjacency/fsm.go applyThreeWayAction marks it for the next Reap) but generates no adjacencyStateChange(Down) event with the reason "Neighbor restarted": Ze has no event for an adjacency that was never Up |
| `RFC5303-3.2-13` | If the new action is "Initialize", no event is generated and the adjacency three-way state SHALL be set to "Initializing". (§3.2) | SHALL | 3.2 | **positive:** `unit/verify` [`TestISISThreeWayInitializeAction`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/adjacency/rfc5303_fsm_test.go#L430). **positive:** `unit/verify` [`TestRFC5303InitializeActionOnEveryRow`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/adjacency/rfc5303_new_adjacency_red_test.go#L129). **negative:** `unit/verify` [`TestISISThreeWayInitializeAction`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/adjacency/rfc5303_fsm_test.go#L433). **negative:** `unit/verify` [`TestRFC5303InitializeActionOnEveryRow`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/adjacency/rfc5303_new_adjacency_red_test.go#L134) |
| `RFC5303-3.1-5` | The other fields in this option SHOULD be included as explained below in section 3.2. (§3.1) | SHOULD | 3.1 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC5303-3.1-2`](#rfc5303-3.1-2) Any system that does not understand this option SHALL ignore it (§3.1) | no test | no test carries this requirement id; annotated {not-applicable}: this constrains a system that does NOT understand the option; Ze implements and processes it (packet/tlv_core.go DecodeP2PThreeWayTLV decodes it, circuit/runtime.go:143 consumes it), so the "does not understand" role never applies to Ze |
| [`RFC5303-3.1-3`](#rfc5303-3.1-3) Any system that does not understand this option SHALL ignore it, and (of course) SHALL NOT include it in its own IIH packets. (§3.1) | no test | no test carries this requirement id; annotated {not-applicable}: this constrains a non-supporting system's emission; Ze supports the mechanism and always emits TLV 240 in its point-to-point IIH (circuit/hello.go:158 threeWayTLV, circuit/hello.go:204 buildP2PHello), so the "does not understand" role never applies to Ze |
| [`RFC5303-3.2-4`](#rfc5303-3.2-4) This value SHALL be unique among all the circuits of this Intermediate System. (§3.2) | {gap}, no test | the Extended Local Circuit ID is derived as uint8(ifindex) (circuits.go:317), truncating the interface index to 8 bits; two circuits whose ifindexes collide modulo 256 emit the same value, so uniqueness among all circuits is not guaranteed and the 4-octet field never exceeds 255 |
| [`RFC5303-3.2-6`](#rfc5303-3.2-6) the neighbor's Extended Local Circuit ID SHALL be reported in the Neighbor Extended Local Circuit ID field. (§3.2) | {gap}, no test | Ze does not track the neighbor's Extended Local Circuit ID; threeWayTLV echoes a constant 0 in the Neighbor Extended Local Circuit ID field (circuit/hello.go:168) and updateThreeWay never stores the neighbor's value (adjacency/fsm.go updateThreeWay), so the neighbor's actual extended circuit ID is never reported |
| [`RFC5303-3.2-8`](#rfc5303-3.2-8) If the option with a valid Adjacency Three-Way State is present, the Neighbor System ID and Neighbor Extended Local Circuit ID fields, if present, SHALL be examined. (§3.2) | {gap}, no test | updateThreeWay examines only the Neighbor System ID (adjacency/fsm.go updateThreeWay) to set neighborNamesOther; the Neighbor Extended Local Circuit ID is decoded (packet/tlv_core.go DecodeP2PThreeWayTLV, the p2pThreeWayLenFull branch) but the FSM never examines it, so the required examination of both fields is incomplete |
| [`RFC5303-3.2-9`](#rfc5303-3.2-9) If they are present, and the Neighbor System ID contained therein does not match the local system's ID, or the Neighbor Extended Local Circuit ID does not match the local system's extended circuit ID, the PDU SHALL be discarded and no further action is taken. (§3.2) | {gap}, no test | on a Neighbor System ID that does not match ours Ze sets the adjacency Initializing (adjacency/fsm.go threeWayTableAction) and still processes the PDU, arming the hold timer and recording the neighbor (adjacency/fsm.go ReceiveHello); it neither discards the PDU nor checks the Neighbor Extended Local Circuit ID, so the loop-detection discard is absent |
| [`RFC5303-3.2-12`](#rfc5303-3.2-12) If the new action is "Down", an adjacencyStateChange(Down) event is generated with the reason "Neighbor restarted" and the adjacency SHALL be deleted. (§3.2) | {gap}, no test | the "Down" action deletes the adjacency (adjacency/fsm.go applyThreeWayAction marks it for the next Reap) but generates no adjacencyStateChange(Down) event with the reason "Neighbor restarted": Ze has no event for an adjacency that was never Up |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC5303-3.1-1`](#rfc5303-3.1-1)

Any system that supports this mechanism SHALL include this option in its Point-to-Point IIH packets. (§3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a Point-to-Point IIH without TLV 240. internal/plugins/isis/circuit/rfc5303_threeway_test.go::TestISISP2PIIHCarriesThreeWayOption: the IIH buildP2PHello emits (the builder sendP2PHello always uses) must decode with TLV 240 (absent is Fatal); negative: a LAN IIH never carries it.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestISISP2PIIHCarriesThreeWayOption`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/rfc5303_threeway_test.go#L129) | unit/verify | unproven |
| positive | [`TestISISP2PIIHCarriesThreeWayOption`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/rfc5303_threeway_test.go#L125) | unit/verify | unproven |

### [`RFC5303-3.1-2`](#rfc5303-3.1-2)

Any system that does not understand this option SHALL ignore it (§3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5303-3.1-2, so no unit is bound to it.

### [`RFC5303-3.1-3`](#rfc5303-3.1-3)

Any system that does not understand this option SHALL ignore it, and (of course) SHALL NOT include it in its own IIH packets. (§3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5303-3.1-3, so no unit is bound to it.

### [`RFC5303-3.1-4`](#rfc5303-3.1-4)

Any system that supports this mechanism MUST include the Adjacency Three-Way State field in this option. (§3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a TLV 240 without the Adjacency Three-Way State field. internal/plugins/isis/circuit/rfc5303_threeway_test.go::TestISISP2PThreeWayStateFieldIncluded: for Up, Initializing and Down the emitted value is non-empty and decodes to the same state (Fatalf otherwise); negative internal/plugins/isis/circuit/rfc5303_threeway_test.go::TestISISP2PThreeWayStateFieldPresentWithoutNeighbor: the minimal no-neighbor form still carries it.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestISISP2PThreeWayStateFieldPresentWithoutNeighbor`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/rfc5303_threeway_test.go#L49) | unit/verify | unproven |
| positive | [`TestISISP2PThreeWayStateFieldIncluded`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/rfc5303_threeway_test.go#L16) | unit/verify | unproven |

### [`RFC5303-3.2-1`](#rfc5303-3.2-1)

The current three-way state of the adjacency with its neighbor on the link (as defined in new section 8.2.4.1.1 introduced later in the document) SHALL be reported in the Adjacency Three-Way State field. (§3.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a transmitted IIH whose Adjacency Three-Way State is not the adjacency's current state. circuit TestRFC5303SentIIHReportsCurrentThreeWayState decodes the TLV 240 of the P2P IIH SendHello hands the transport (sendP2PHello -> p2pThreeWayState -> buildP2PHello): Initializing after a peer Hello that does not echo us, Up after the echo. The two values differ, so a send path that ignored the computed state or sent a fixed value goes red; recorded +/- on runtime.go p2pThreeWayState. The older helper-level units remain tagged and are subsumed.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5303SentIIHReportsCurrentThreeWayState`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/rfc5303_sent_iih_test.go#L85) | unit/verify | revert, verified |
| negative | [`TestISISThreeWayReportsCurrentState`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/rfc5303_threeway_test.go#L149) | unit/verify | revert, verified |
| positive | [`TestRFC5303SentIIHReportsCurrentThreeWayState`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/rfc5303_sent_iih_test.go#L82) | unit/verify | revert, verified |
| positive | [`TestISISThreeWayReportsCurrentState`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/rfc5303_threeway_test.go#L146) | unit/verify | revert, verified |

### [`RFC5303-3.2-2`](#rfc5303-3.2-2)

If no adjacency exists, the state SHALL be reported as Down. (§3.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a state other than Down reported with no adjacency. circuit TestRFC5303SentIIHReportsDownWithoutAdjacency decodes the transmitted IIH's TLV 240: Down before any peer Hello and again after Teardown (positive), and not Down while an Up adjacency exists (negative), so a sender that always reported Down, or kept a stale state after teardown, goes red. Recorded +/- on p2pThreeWayState.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5303SentIIHReportsDownWithoutAdjacency`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/rfc5303_sent_iih_test.go#L108) | unit/verify | revert, verified |
| negative | [`TestISISThreeWayDownWhenNoAdjacency`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/rfc5303_threeway_test.go#L178) | unit/verify | revert, verified |
| positive | [`TestRFC5303SentIIHReportsDownWithoutAdjacency`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/rfc5303_sent_iih_test.go#L105) | unit/verify | revert, verified |
| positive | [`TestISISThreeWayDownWhenNoAdjacency`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/rfc5303_threeway_test.go#L175) | unit/verify | revert, verified |

### [`RFC5303-3.2-3`](#rfc5303-3.2-3)

The Extended Local Circuit ID field SHALL contain a value assigned by this IS when the circuit is created. (§3.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: an Extended Local Circuit ID field that is not the circuit's assigned value (absent, a shared constant). internal/plugins/isis/circuit/rfc5303_threeway_test.go::TestISISThreeWayExtendedLocalCircuitID: the IIH buildP2PHello emits carries HasCircuitID and LocalCircuitID == 7 (Fatalf otherwise); negative: a circuit assigned 42 emits a different value. The uint8(ifindex) uniqueness defect is RFC5303-3.2-4's {gap}.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestISISThreeWayExtendedLocalCircuitID`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/rfc5303_threeway_test.go#L206) | unit/verify | unproven |
| positive | [`TestISISThreeWayExtendedLocalCircuitID`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/rfc5303_threeway_test.go#L203) | unit/verify | unproven |

### [`RFC5303-3.2-4`](#rfc5303-3.2-4)

This value SHALL be unique among all the circuits of this Intermediate System. (§3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5303-3.2-4, so no unit is bound to it.

### [`RFC5303-3.2-5`](#rfc5303-3.2-5)

If the system ID and Extended Local Circuit ID of the neighboring system are known (in adjacency three-way state Initializing or Up), the neighbor's system ID SHALL be reported in the Neighbor System ID field (§3.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: omitting the neighbor's System ID from the transmitted IIH when the neighbor is known (Initializing or Up). circuit TestRFC5303SentIIHEchoesKnownNeighbor decodes the sent TLV 240: no Neighbor System ID before any neighbor is heard (negative), and the peer's System ID present in Initializing and again in Up (positive). The adjacency is driven by received Hellos through Circuit.Receive, closing the earlier gap (p2pThreeWayState's haveNeighbor result was unasserted). Recorded + on p2pThreeWayState and - on hello.go threeWayTLV. The row's clause is the System ID only; the Extended Local Circuit ID half of the sentence's condition is not asserted here.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5303SentIIHEchoesKnownNeighbor`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/rfc5303_sent_iih_test.go#L133) | unit/verify | revert, verified |
| negative | [`TestISISThreeWayReportsNeighborSystemID`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/rfc5303_threeway_test.go#L229) | unit/verify | revert, verified |
| positive | [`TestRFC5303SentIIHEchoesKnownNeighbor`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/rfc5303_sent_iih_test.go#L130) | unit/verify | revert, verified |
| positive | [`TestISISThreeWayReportsNeighborSystemID`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/rfc5303_threeway_test.go#L226) | unit/verify | revert, verified |

### [`RFC5303-3.2-6`](#rfc5303-3.2-6)

the neighbor's Extended Local Circuit ID SHALL be reported in the Neighbor Extended Local Circuit ID field. (§3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5303-3.2-6, so no unit is bound to it.

### [`RFC5303-3.2-7`](#rfc5303-3.2-7)

If the option is present and contains invalid Adjacency Three-Way State, the PDU SHALL be discarded and no further action is taken. (§3.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent first judgment against RFC 5303 Section 3.2: "If the option is present and contains invalid Adjacency Three-Way State, the PDU SHALL be discarded and no further action is taken." TestRFC5303InvalidStateWireDiscard now carries a positive tag on its existing valid-state control: Circuit.Receive decodes the valid peer PDU and must reach StateUp before the established-neighbor invalid-state probes. Its negative assertions cover TLV lengths 1, 5 and 15, invalid states 3 and 255, fresh and established neighbors, unchanged neighbor presence and whole-adjacency equality, and no SessionUp, SessionDown or ForwardingChanged flags. TestRFC5303InvalidStateLeavesAdjacencyUntouched additionally covers every unnamed state 3 through 255 against Down, Initializing and Up adjacencies with whole-object equality and rejection; TestRFC5303InvalidStateBeforeValidDuplicate prevents a later valid option from hiding an earlier invalid one. Circuit.Receive routes P2P Hellos to handleP2PHello, which rejects every invalid TLV 240 before neighbor creation; ReceiveHello independently rejects invalid raw HelloInput before identity, hold-timer or adjacency mutation. The positive input is genuinely conforming, not another error; the negative assertions would fail if invalid states were admitted or mutated adjacency state. Canonical records name these producers, including the newly recorded positive cover; their producer-body panic breaks establish execution dependence, not selective semantic mutation. The assertions and producer routing, read against the whole sentence, support this requirement only; neighbor-ID mismatch and restart-event gaps remain separate.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5303InvalidStateLeavesAdjacencyUntouched`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/adjacency/rfc5303_invalid_state_test.go#L14) | unit/verify | revert, verified |
| negative | [`TestRFC5303InvalidStateBeforeValidDuplicate`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/rfc5303_invalid_state_test.go#L73) | unit/verify | revert, verified |
| negative | [`TestRFC5303InvalidStateWireDiscard`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/rfc5303_invalid_state_test.go#L17) | unit/verify | revert, verified |
| positive | [`TestRFC5303InvalidStateWireDiscard`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/rfc5303_invalid_state_test.go#L16) | unit/verify | revert, verified |

### [`RFC5303-3.2-8`](#rfc5303-3.2-8)

If the option with a valid Adjacency Three-Way State is present, the Neighbor System ID and Neighbor Extended Local Circuit ID fields, if present, SHALL be examined. (§3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5303-3.2-8, so no unit is bound to it.

### [`RFC5303-3.2-9`](#rfc5303-3.2-9)

If they are present, and the Neighbor System ID contained therein does not match the local system's ID, or the Neighbor Extended Local Circuit ID does not match the local system's extended circuit ID, the PDU SHALL be discarded and no further action is taken. (§3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5303-3.2-9, so no unit is bound to it.

### [`RFC5303-3.2-10`](#rfc5303-3.2-10)

In section 8.2.4.2 a and b, the action "Up" from state tables 5, 6, 7, and 8 may create a new adjacency but the three-way state of the adjacency SHALL be Down. (§3.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-30 after the D-8 (R39) fix: ReceiveHello now applies the section 3.2 table to a P2P adjacency that has seen TLV 240, and a new adjacency is created in StateDown so its first Hello reads the Down row. adjacency TestRFC5303NewAdjacencyReceivingUpDoesNotComeUp: a fresh adjacency receiving Initializing comes Up with SessionUp (positive, the Down row's Up cell); a fresh adjacency receiving Up with our System ID echoed stays Down with no session event and deleteAt=now (negative: the old code went straight Up), while an Initializing adjacency given the same Hello comes Up, so the refusal is the Down row's. Recorded +/- on fsm.go threeWayTableAction. circuit TestISISThreeWayNewAdjacencyStateDown stays tagged and proves only the neighbouring reporting rule (a Down record is reported Down); it carries no weight here.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5303NewAdjacencyReceivingUpDoesNotComeUp`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/adjacency/rfc5303_new_adjacency_red_test.go#L40) | unit/verify | revert, verified |
| negative | [`TestISISThreeWayNewAdjacencyStateDown`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/rfc5303_threeway_test.go#L251) | unit/verify | revert, verified |
| positive | [`TestRFC5303NewAdjacencyReceivingUpDoesNotComeUp`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/adjacency/rfc5303_new_adjacency_red_test.go#L37) | unit/verify | revert, verified |
| positive | [`TestISISThreeWayNewAdjacencyStateDown`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/circuit/rfc5303_threeway_test.go#L247) | unit/verify | revert, verified |

### [`RFC5303-3.2-11`](#rfc5303-3.2-11)

If the action taken from section 8.2.4.2 a or b is "Up" or "Accept", the IS SHALL perform the action indicated by the new adjacency three-way state table below, based on the current adjacency three-way state and the received Adjacency Three-Way State value from the option. (§3.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-30 (independent judge) after TestISISThreeWayProceduresEngagedByOption moved here from the retired RFC5303-3.1-6 (D-15). adjacency TestRFC5303ThreeWayStateTable drives all nine cells of the section 3.2 table through ReceiveHello and requires state and session flags per cell (positive and negative, recorded on threeWayTableAction), checked against rfc/full/rfc5303.txt. The moved positive adds the Down/Up cell from a brand-new adjacency: action Down, state stays Down, no SessionUp/SessionDown, deleteAt set to now (record + revert threeWayTableAction). Its legacy no-TLV-240 half is untagged and asserts nothing for this row. The PDU-discard preconditions remain separate obligations: invalid-state discard (3.2-7) now has positive and negative tagged coverage; neighbor-mismatch discard (3.2-9) remains a {gap}.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5303ThreeWayStateTable`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/adjacency/rfc5303_new_adjacency_red_test.go#L98) | unit/verify | revert, verified |
| positive | [`TestISISThreeWayProceduresEngagedByOption`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/adjacency/rfc5303_fsm_test.go#L400) | unit/verify | revert, verified |
| positive | [`TestRFC5303ThreeWayStateTable`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/adjacency/rfc5303_new_adjacency_red_test.go#L94) | unit/verify | revert, verified |

### [`RFC5303-3.2-12`](#rfc5303-3.2-12)

If the new action is "Down", an adjacencyStateChange(Down) event is generated with the reason "Neighbor restarted" and the adjacency SHALL be deleted. (§3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5303-3.2-12, so no unit is bound to it.

### [`RFC5303-3.2-13`](#rfc5303-3.2-13)

If the new action is "Initialize", no event is generated and the adjacency three-way state SHALL be set to "Initializing". (§3.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-30. Forbidden: on an Initialize action, generating an adjacency-up event or not setting Initializing. adjacency TestRFC5303InitializeActionOnEveryRow drives received Down on all three rows (Down, Initializing, Up, each reached by Hellos): every row ends Initializing with no SessionUp; Down and Initializing rows raise no flag at all; the Up row raises only SessionDown, which R39 keeps as Ze's internal withdraw notice (fsm.go applyThreeWayAction quotes RFC 5303 sec 3.2 and ISO 10589 sec 9.4/7.5), not the adjacencyStateChange event the sentence forbids. Negative: Initializing row + received Initializing is the Up action, leaving Initializing with SessionUp. This closes the earlier gap (Initializing and Up rows unasserted). TestISISThreeWayInitializeAction remains tagged. Recorded +/- on applyThreeWayAction for both units.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestISISThreeWayInitializeAction`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/adjacency/rfc5303_fsm_test.go#L433) | unit/verify | revert, verified |
| negative | [`TestRFC5303InitializeActionOnEveryRow`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/adjacency/rfc5303_new_adjacency_red_test.go#L134) | unit/verify | revert, verified |
| positive | [`TestISISThreeWayInitializeAction`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/adjacency/rfc5303_fsm_test.go#L430) | unit/verify | revert, verified |
| positive | [`TestRFC5303InitializeActionOnEveryRow`](https://github.com/ze-software/ze/blob/main/internal/plugins/isis/adjacency/rfc5303_new_adjacency_red_test.go#L129) | unit/verify | revert, verified |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-30 |
| Register | prose |
| Source | rfc/full/rfc5303.txt |
| Source fingerprint | 126dfb230a6e85de |
| Record | rfc/extraction/rfc5303.json |
| Mapped sentences | 15 |
| Declined as scope | 3 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | walked | not stated |
| `1` | not stated | 0 | walked | not stated |
| `1.1` | not stated | 0 | walked | not stated |
| `2` | not stated | 0 | walked | not stated |
| `2.1` | not stated | 0 | walked | not stated |
| `2.2` | not stated | 0 | walked | not stated |
| `3` | not stated | 0 | walked | not stated |
| `3.1` | not stated | 4 | walked | not stated |
| `3.2` | not stated | 13 | walked | not stated |
| `4` | not stated | 0 | walked | not stated |
| `5` | not stated | 0 | walked | not stated |
| `6` | not stated | 0 | walked | not stated |
| `7` | not stated | 0 | walked | not stated |
| `8` | not stated | 0 | walked | not stated |
| `9` | not stated | 1 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `3.1:4` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | A framing sentence: it points at the Section 3.2 Elements of Procedure, and each of those procedures is an obligation of its own in rows RFC5303-3.2-1 to RFC5303-3.2-13. No behavior belongs to this sentence that those rows do not already state. Row RFC5303-3.1-6 retired 2026-09-30 (rfc/corrections/rfc5303.md, ruling R22 shape). | Any system that is able to process this option SHALL follow the procedures below. |
| `3.2:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The sending clause repeats the Section 3.1 obligation to include the option in every Point-to-Point IIH, which site 3.1:1 already maps to RFC5303-3.1-1. | The IS SHALL include the Point-to-Point Three-Way Adjacency option in the transmitted Point-to-Point IIH PDU. |
| `9:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | IETF IPR boilerplate from the Intellectual Property notice; it invites disclosure of patent rights and states no protocol obligation. | The IETF invites any interested party to bring to its attention any copyrights, patents or patent applications, or other proprietary rights that may cover technology that may be required to implement this standard. |

## Superseded

No document obsoletes RFC 5303, so its obligations are stated where they were written.
